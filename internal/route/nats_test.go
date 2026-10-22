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
	"uuid"

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
	// 保留消息以检查 HTTP 返回前同步、异步发布均已确认，以及每次调用生成的新批次。
	stream, err := client.JetStream().CreateStream(ctx, jetstream.StreamConfig{
		Name:     "PIXIU_DEMO_TASKS",
		Subjects: []string{"pixiu.demo.tasks"},
		Storage:  jetstream.MemoryStorage,
	})
	if err != nil {
		t.Fatal(err)
	}
	router := newAPITestRouter(t)
	var previousBatch string
	for _, tc := range []struct {
		name string
		body string
		msgs uint64
	}{
		{name: "无请求体发布", msgs: 2},
		{name: "空对象再次发布", body: `{}`, msgs: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/app/nats/publish", strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			var result struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatalf("响应不是 JSON: %v, body=%s", err, w.Body.String())
			}
			if w.Code != http.StatusOK || result.Code != 0 {
				t.Fatalf("发布结果错误: status=%d body=%s", w.Code, w.Body.String())
			}
			info, err := stream.Info(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if info.State.Msgs != tc.msgs {
				t.Fatalf("消息数 = %d，预期 %d", info.State.Msgs, tc.msgs)
			}
			var batch string
			for i, suffix := range []string{":sync", ":async"} {
				msg, err := stream.GetMsg(ctx, tc.msgs-1+uint64(i))
				if err != nil {
					t.Fatal(err)
				}
				messageID := msg.Header.Get(jetstream.MsgIDHeader)
				prefix, ok := strings.CutSuffix(messageID, suffix)
				id, err := uuid.Parse(prefix)
				if !ok || err != nil || id == uuid.Nil() || id.String() != prefix {
					t.Fatalf("消息 ID 应为 UUID 加 %s 后缀，实际为 %q", suffix, messageID)
				}
				if i == 0 {
					batch = prefix
				} else if prefix != batch {
					t.Fatalf("同一批同步、异步消息使用了不同前缀: %q, %q", batch, prefix)
				}
			}
			if batch == previousBatch {
				t.Fatalf("不同调用复用了批次前缀: %q", batch)
			}
			previousBatch = batch
		})
	}
	// 关闭连接后由接口返回统一服务错误，不泄露下游细节。
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/app/nats/publish", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Body.String() != `{"code":400,"msg":"服务异常"}` {
		t.Fatalf("发布失败响应错误: status=%d body=%s", w.Code, w.Body.String())
	}
}
