package route

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"pixiu-demo/errorcode"

	"github.com/bpcoder16/pixiu/infra/natsx"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go/jetstream"
)

func TestAppNATSPublish(t *testing.T) {
	// 默认实例注册表关闭后不可复用，沿用路由测试的独立进程隔离。
	const marker = "PIXIU_DEMO_NATS_TEST"
	if os.Getenv(marker) != t.Name() {
		cmd := exec.Command(os.Args[0], "-test.run=^TestAppNATSPublish$")
		cmd.Env = append(os.Environ(), marker+"="+t.Name())
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("NATS 独立进程测试失败: %v\n%s", err, output)
		}
		return
	}
	s, err := server.NewServer(&server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		JetStream: true,
		StoreDir:  t.TempDir(),
		NoLog:     true,
		NoSigs:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		s.Shutdown()
		s.WaitForShutdown()
	})
	s.Start()
	if !s.ReadyForConnections(5 * time.Second) {
		t.Fatal("测试 NATS 未就绪")
	}
	client, err := natsx.NewDefault(natsx.Config{
		Name: "demo",
		URLs: []string{s.ClientURL()},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := natsx.CloseAll(); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// 保留消息以检查 HTTP 返回前同步、异步发布均已确认，以及相同批次的去重。
	stream, err := client.JetStream().CreateStream(ctx, jetstream.StreamConfig{
		Name:     "PIXIU_DEMO_TASKS",
		Subjects: []string{"pixiu.demo.tasks"},
		Storage:  jetstream.MemoryStorage,
	})
	if err != nil {
		t.Fatal(err)
	}
	router := newAPITestRouter(t)
	for _, tc := range []struct {
		name string
		body string
		code int
		msgs uint64
	}{
		{name: "缺少批次", body: `{}`, code: errorcode.ErrParams},
		{name: "空白批次", body: `{"messageIDPrefix":" \t"}`, code: errorcode.ErrParams},
		{name: "发布四种示例", body: `{"messageIDPrefix":"batch-1"}`, msgs: 2},
		{name: "重试相同批次", body: `{"messageIDPrefix":"batch-1"}`, msgs: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/app/nats/publish", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			var result struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatalf("响应不是 JSON: %v, body=%s", err, w.Body.String())
			}
			if w.Code != http.StatusOK || result.Code != tc.code {
				t.Fatalf("发布结果错误: status=%d body=%s", w.Code, w.Body.String())
			}
			info, err := stream.Info(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if info.State.Msgs != tc.msgs {
				t.Fatalf("消息数 = %d，预期 %d", info.State.Msgs, tc.msgs)
			}
		})
	}
	// 关闭连接后由接口返回统一服务错误，不泄露下游细节。
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/app/nats/publish", strings.NewReader(`{"messageIDPrefix":"batch-2"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Body.String() != `{"code":400,"msg":"服务异常"}` {
		t.Fatalf("发布失败响应错误: status=%d body=%s", w.Code, w.Body.String())
	}
}
