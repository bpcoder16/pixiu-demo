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

func TestAppAliyunESDescribeInstance(t *testing.T) {
	// 默认 ES 注册表的关闭不可逆，使用独立进程隔离真实客户端测试。
	const marker = "PIXIU_DEMO_ES_INSTANCE_TEST"
	if os.Getenv(marker) != t.Name() {
		cmd := exec.Command(os.Args[0], "-test.run=^TestAppAliyunESDescribeInstance$")
		cmd.Env = append(os.Environ(), marker+"="+t.Name())
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("ES 实例查询独立进程测试失败: %v\n%s", err, output)
		}
		return
	}

	var calls atomic.Int64
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/openapi/instances/es-demo" || r.URL.RawQuery != "" {
			t.Errorf("实例查询目标错误: %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		if fail.Load() {
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"RequestId":"failed-request","Code":"AccessDenied","Message":"private-instance-error"}`)
			return
		}
		_, _ = io.WriteString(w, `{"RequestId":"describe-request","Result":{"instanceId":"es-from-api","status":"activating","esVersion":"8.17"}}`)
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
		fail   bool
		cancel bool
		calls  int64
		want   string
	}{
		{
			name:  "保留服务端实例元数据和原始状态",
			calls: 1,
			want:  `{"code":0,"msg":"成功","data":{"requestId":"describe-request","instanceId":"es-from-api","status":"activating","version":"8.17"}}`,
		},
		{
			name:  "ES 失败不泄露底层错误且不重试",
			fail:  true,
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
			fail.Store(tc.fail)
			before := calls.Load()
			req := httptest.NewRequest(http.MethodGet, "/api/app/aliyun/es/describe-instance", nil)
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
