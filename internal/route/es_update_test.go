package route

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bpcoder16/pixiu/infra/aliyun/esx"
)

func TestAppAliyunESSynonymsDictsUpdate(t *testing.T) {
	// 默认 ES 注册表的关闭不可逆，使用独立进程隔离真实客户端测试。
	const marker = "PIXIU_DEMO_ES_UPDATE_TEST"
	if os.Getenv(marker) != t.Name() {
		cmd := exec.Command(os.Args[0], "-test.run=^TestAppAliyunESSynonymsDictsUpdate$")
		cmd.Env = append(os.Environ(), marker+"="+t.Name())
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("ES 词典更新独立进程测试失败: %v\n%s", err, output)
		}
		return
	}

	const inventory = `[{"name":"keep.txt","sourceType":"ORIGIN"},{"name":"current.txt","sourceType":"OSS","bucket":"dict-bucket","key":"release/词库.txt"}]`
	const validBody = `{"clientToken":"release-1","dicts":` + inventory + `}`
	const resultDicts = `[{"name":"keep.txt","fileSize":10,"sourceType":"ORIGIN","type":"SYNONYMS"},{"name":"current.txt","fileSize":20,"sourceType":"OSS","type":"SYNONYMS"}]`
	var calls atomic.Int64
	var mode atomic.Int32
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	releaseUpdate := sync.OnceFunc(func() { close(release) })
	t.Cleanup(releaseUpdate)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodPut || r.URL.Path != "/openapi/instances/es-demo/synonymsDict" || r.URL.Query().Get("clientToken") != "release-1" {
			t.Errorf("词典更新目标或幂等令牌错误: %s %s", r.Method, r.URL)
		}
		var got, want any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		_ = json.Unmarshal([]byte(`[{"name":"keep.txt","sourceType":"ORIGIN","type":"SYNONYMS"},{"name":"current.txt","sourceType":"OSS","type":"SYNONYMS","ossObject":{"bucketName":"dict-bucket","key":"release/词库.txt"}}]`), &want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("完整清单或 OSS 来源转换错误: got=%v, want=%v", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		switch mode.Load() {
		case 1:
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"RequestId":"failed-request","Code":"AccessDenied","Message":"private-update-error"}`)
			return
		case 2:
			entered <- struct{}{}
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		_, _ = io.WriteString(w, `{"RequestId":"update-request","Result":`+resultDicts+`}`)
	}))
	t.Cleanup(server.Close)
	if _, err := esx.NewDefault(esx.Config{
		Name:            "demo",
		Region:          "cn-hangzhou",
		InstanceID:      "es-demo",
		AccessKeyID:     "test-access-key",
		AccessKeySecret: "test-secret",
		Endpoint:        server.URL,
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := esx.CloseAll(); err != nil {
			t.Error(err)
		}
	})
	router := newAPITestRouter(t)
	serve := func(ctx context.Context, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/app/aliyun/es/synonyms-dicts-update", strings.NewReader(body)).WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")
		writer := httptest.NewRecorder()
		router.ServeHTTP(writer, req)
		return writer
	}
	checkResponse := func(t *testing.T, writer *httptest.ResponseRecorder, code int) {
		t.Helper()
		var got struct {
			Code int             `json:"code"`
			Msg  string          `json:"msg"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(writer.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if writer.Code != http.StatusOK || got.Code != code {
			t.Fatalf("响应协议错误: status=%d, body=%s", writer.Code, writer.Body.String())
		}
		if code != 0 && got.Data != nil {
			t.Fatalf("失败响应不应包含 data: %s", writer.Body.String())
		}
		if code == 400 && got.Msg != "服务异常" {
			t.Fatalf("下游错误不应泄露: %s", writer.Body.String())
		}
		if code == 0 {
			var data, want any
			if err := json.Unmarshal(got.Data, &data); err != nil {
				t.Fatal(err)
			}
			_ = json.Unmarshal([]byte(`{"requestId":"update-request","dicts":`+resultDicts+`}`), &want)
			if !reflect.DeepEqual(data, want) {
				t.Fatalf("更新元数据错误: %s", writer.Body.String())
			}
		}
	}

	for _, tc := range []struct {
		name   string
		body   string
		mode   int32
		cancel bool
		calls  int64
		code   int
	}{
		{name: "完整清单保留原文件并从 OSS 更新", body: validBody, calls: 1},
		{name: "空清单不可清空词典", body: `{"clientToken":"release-1","dicts":[]}`, code: 10001},
		{name: "重复文件不可提交", body: `{"clientToken":"release-1","dicts":[{"name":"same.txt","sourceType":"ORIGIN"},{"name":"same.txt","sourceType":"ORIGIN"}]}`, code: 10001},
		{name: "未知来源不可提交", body: `{"clientToken":"release-1","dicts":[{"name":"ok.txt","sourceType":"URL"}]}`, code: 10001},
		{name: "OSS 来源必须指定对象", body: `{"clientToken":"release-1","dicts":[{"name":"ok.txt","sourceType":"OSS","bucket":"dict-bucket"}]}`, code: 10001},
		{name: "空白 Bucket 不可提交", body: `{"clientToken":"release-1","dicts":[{"name":"ok.txt","sourceType":"OSS","bucket":" ","key":"ok.txt"}]}`, code: 10001},
		{name: "非法文件名不可提交", body: `{"clientToken":"release-1","dicts":[{"name":"../bad.txt","sourceType":"ORIGIN"}]}`, code: 10001},
		{name: "空白令牌不可提交", body: `{"clientToken":" ","dicts":` + inventory + `}`, code: 10001},
		{name: "非 ASCII 令牌不可提交", body: `{"clientToken":"发布","dicts":` + inventory + `}`, code: 10001},
		{name: "下游失败不重试且不泄露错误", body: validBody, mode: 1, calls: 1, code: 400},
		{name: "请求取消不发起下游调用", body: validBody, cancel: true, code: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode.Store(tc.mode)
			before := calls.Load()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			checkResponse(t, serve(ctx, tc.body), tc.code)
			if calls.Load()-before != tc.calls {
				t.Fatalf("下游调用次数错误: got=%d, want=%d", calls.Load()-before, tc.calls)
			}
		})
	}

	t.Run("共享默认实例的更新串行且等待可取消", func(t *testing.T) {
		mode.Store(2)
		defer releaseUpdate()
		before := calls.Load()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		done := make(chan *httptest.ResponseRecorder, 1)
		go func() { done <- serve(ctx, validBody) }()
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal("首次更新未进入下游")
		}
		waitingCtx, stopWaiting := context.WithTimeout(ctx, 100*time.Millisecond)
		defer stopWaiting()
		checkResponse(t, serve(waitingCtx, validBody), 400)
		if calls.Load()-before != 1 {
			t.Fatal("前一次更新完成前不应提交下一份完整清单")
		}
		releaseUpdate()
		select {
		case writer := <-done:
			checkResponse(t, writer, 0)
		case <-ctx.Done():
			t.Fatal("首次更新未结束")
		}
		mode.Store(0)
		checkResponse(t, serve(ctx, validBody), 0)
	})
}
