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
	"sync/atomic"
	"testing"

	"github.com/bpcoder16/pixiu/infra/aliyun/esx"
)

func TestAppAliyunESSynonymsDictsList(t *testing.T) {
	// 默认 ES 注册表的关闭不可逆，使用独立进程隔离真实客户端测试。
	const marker = "PIXIU_DEMO_ES_TEST"
	if os.Getenv(marker) != t.Name() {
		cmd := exec.Command(os.Args[0], "-test.run=^TestAppAliyunESSynonymsDictsList$")
		cmd.Env = append(os.Environ(), marker+"="+t.Name())
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("ES 独立进程测试失败: %v\n%s", err, output)
		}
		return
	}

	const dicts = `[{"name":"current.txt","fileSize":10,"sourceType":"OSS","type":"SYNONYMS","downloadUrl":"https://example.test/current.txt?Signature=test-signature&Expires=123"},{"name":"other.txt","fileSize":20,"sourceType":"ORIGIN","type":"SYNONYMS","downloadUrl":""}]`
	var calls atomic.Int64
	var mode atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/openapi/instances/es-demo/dicts" || r.URL.RawQuery != "analyzerType=SYNONYMS" {
			t.Errorf("词典查询目标错误: %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		switch mode.Load() {
		case 1:
			_, _ = io.WriteString(w, `{"RequestId":"empty-request","Result":[]}`)
		case 2:
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"RequestId":"failed-request","Code":"AccessDenied","Message":"private-es-error"}`)
		default:
			_, _ = io.WriteString(w, `{"RequestId":"list-request","Result":`+dicts+`}`)
		}
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

	for _, tc := range []struct {
		name   string
		mode   int32
		cancel bool
		calls  int64
		want   string
	}{
		{
			name:  "完整词典元数据和下载链接",
			calls: 1,
			want:  `{"code":0,"msg":"成功","data":{"requestId":"list-request","dicts":` + dicts + `}}`,
		},
		{
			name:  "空列表保持数组",
			mode:  1,
			calls: 1,
			want:  `{"code":0,"msg":"成功","data":{"requestId":"empty-request","dicts":[]}}`,
		},
		{
			name:  "ES 失败不泄露底层错误且不重试",
			mode:  2,
			calls: 1,
			want:  `{"code":400,"msg":"服务异常"}`,
		},
		{
			name:   "请求取消不发起下游调用",
			cancel: true,
			want:   `{"code":400,"msg":"服务异常"}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode.Store(tc.mode)
			before := calls.Load()
			req := httptest.NewRequest(http.MethodGet, "/api/app/aliyun/es/synonyms-dicts-list", nil)
			if tc.cancel {
				ctx, cancel := context.WithCancel(req.Context())
				cancel()
				req = req.WithContext(ctx)
			}
			writer := httptest.NewRecorder()
			router.ServeHTTP(writer, req)
			if writer.Code != http.StatusOK {
				t.Fatalf("响应状态错误: %d %s", writer.Code, writer.Body.String())
			}
			var got, want any
			if err := json.Unmarshal(writer.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.want), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("响应协议错误: got=%s, want=%s", writer.Body.String(), tc.want)
			}
			if calls.Load()-before != tc.calls {
				t.Fatalf("下游调用次数错误: got=%d, want=%d", calls.Load()-before, tc.calls)
			}
		})
	}
}
