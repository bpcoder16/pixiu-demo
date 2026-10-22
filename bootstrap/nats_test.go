package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"pixiu-demo/internal/natshandler"
	natsservice "pixiu-demo/internal/service/nats"

	"github.com/bpcoder16/pixiu/biz/bootstrap"
	"github.com/bpcoder16/pixiu/infra/apprun"
	"github.com/bpcoder16/pixiu/infra/env"
	"github.com/bpcoder16/pixiu/infra/natsx"
	"github.com/bpcoder16/pixiu/logit"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go/jetstream"
)

func TestExamplesPublishAndConsumeAcrossInstances(t *testing.T) {
	if !initExampleProcess(t) {
		return
	}
	// 两个真实客户端同时运行示例，验证广播、Queue 和共享 Durable 的装配。
	records := make(chan exampleLog, 32)
	logger := logit.MustNew(
		logit.OptEncoder(logit.DefaultJSONEncoder),
		logit.OptWriter(logit.NewWriter(exampleLogWriter(func(p []byte) (int, error) {
			var record exampleLog
			if err := json.Unmarshal(p, &record); err != nil {
				return 0, err
			}
			if strings.HasPrefix(record.Message, "DemoNATS") {
				records <- record
			}
			return len(p), nil
		}))),
	)
	previous := logit.Default()
	logit.SetDefault(logger)
	t.Cleanup(func() {
		logit.SetDefault(previous)
		_ = logit.Close(logger)
	})
	s := startExampleServer(t)
	url := s.ClientURL()
	first := newExampleClient(t, url, "first", true)
	second := newExampleClient(t, url, "second", false)
	var services []apprun.Service
	ran := make(chan error, 2)
	for _, name := range []string{"first", "second"} {
		writeExampleRegistration(t, name)
		service, err := bootstrap.NewNATSConsumerService(name, natshandler.JetStreamRegistry, natshandler.CoreRegistry)
		if err != nil {
			t.Fatal(err)
		}
		services = append(services, service)
		t.Cleanup(func() {
			if err := stopAndWait(service.Stop, service.Wait, service.Cancel); err != nil {
				t.Error(err)
			}
		})
		go func() { ran <- service.Run() }()
	}

	ctx, cancel := context.WithTimeout(logit.WithContextLogID(context.Background()), 10*time.Second)
	defer cancel()
	// Run 异步注册；服务端收到全部 Core 订阅后才发布，避免广播在启动期间丢失。
	for subject, want := range map[string]int{"pixiu.demo.broadcast": 2, "pixiu.demo.queue": 8} {
		for {
			subs, err := s.Subsz(&server.SubszOptions{
				Subscriptions: true,
				Test:          subject,
			})
			if err != nil {
				t.Fatal(err)
			}
			if subs.Total == want {
				break
			}
			select {
			case err := <-ran:
				t.Fatalf("消费者提前退出: %v", err)
			case <-ctx.Done():
				t.Fatalf("主题 %s 的订阅未就绪: %v", subject, ctx.Err())
			case <-time.After(time.Millisecond):
			}
		}
	}
	if err := natsservice.NewService().PublishExamples(ctx); err != nil {
		t.Fatal(err)
	}
	stream, err := first.JetStream().Stream(ctx, "PIXIU_DEMO_TASKS")
	if err != nil {
		t.Fatal(err)
	}
	if got := stream.CachedInfo().Config.MaxBytes; got != -1 {
		t.Fatalf("Demo Stream 应沿用默认不限量配置，实际为 %d 字节", got)
	}
	consumer, err := first.JetStream().Consumer(ctx, "PIXIU_DEMO_TASKS", "demo-worker")
	if err != nil {
		t.Fatal(err)
	}
	if cfg := consumer.CachedInfo().Config; cfg.AckWait != 60*time.Second || cfg.MaxDeliver != 5 || cfg.MaxAckPending != 1000 {
		t.Fatalf("配置迁移后未使用 Consumer Default: %+v", cfg)
	}
	// 等待业务完成后的 ACK，避免把取得 PubAck 误当成消费完成。
	for {
		info, err := consumer.Info(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if info.AckFloor.Consumer == 2 && info.NumAckPending == 0 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("示例消息未完成处理和 ACK")
		case <-time.After(10 * time.Millisecond):
		}
	}
	for _, service := range services {
		if err := stopAndWait(service.Stop, service.Wait, service.Cancel); err != nil {
			t.Fatal(err)
		}
	}
	for range services {
		select {
		case err := <-ran:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal("消费者未退出", ctx.Err())
		}
	}
	for _, client := range []*natsx.Client{first, second} {
		if err := client.Close(); err != nil {
			t.Fatal(err)
		}
	}

	counts := make(map[string]int)
	modes := make(map[string]int)
	bodies := make(map[string]int)
	traceID, _ := logit.LogIDFromContext(ctx)
	for len(records) > 0 {
		record := <-records
		counts[record.Message]++
		if record.Data != "" {
			bodies[record.Message+":"+record.Data]++
		}
		if record.LogID != traceID {
			t.Fatalf("发布与消费未沿用追踪 ID: %+v", record)
		}
		if record.Message == "DemoNATSPublish" {
			modes[record.Mode]++
			if record.Stream != "PIXIU_DEMO_TASKS" || record.Sequence == 0 {
				t.Fatalf("未记录最终 PubAck: %+v", record)
			}
		}
	}
	for name, want := range map[string]int{
		"DemoNATSJetStream":     2,
		"DemoNATSCoreBroadcast": 2,
		"DemoNATSCoreQueue":     1,
		"DemoNATSPublish":       2,
	} {
		if counts[name] != want {
			t.Errorf("%s 消息数 = %d，预期 %d", name, counts[name], want)
		}
	}
	for body, want := range map[string]int{
		"DemoNATSJetStream:sync example":          1,
		"DemoNATSJetStream:async example":         1,
		"DemoNATSCoreBroadcast:broadcast example": 2,
		"DemoNATSCoreQueue:queue example":         1,
	} {
		if bodies[body] != want {
			t.Errorf("消息正文 %q 的数量 = %d，预期 %d", body, bodies[body], want)
		}
	}
	if modes["sync"] != 1 || modes["async"] != 1 {
		t.Fatalf("未覆盖同步与异步确认: %v", modes)
	}
}

func TestRegisterRejectsStreamConflict(t *testing.T) {
	if !initExampleProcess(t) {
		return
	}
	client := newExampleClient(t, startExampleServer(t).ClientURL(), "conflict", true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.JetStream().CreateStream(ctx, jetstream.StreamConfig{
		Name:     "PIXIU_DEMO_TASKS",
		Subjects: []string{"existing.tasks"},
		Storage:  jetstream.FileStorage,
	}); err != nil {
		t.Fatal(err)
	}
	writeExampleRegistration(t, "conflict")
	service, err := bootstrap.NewNATSConsumerService("conflict", natshandler.JetStreamRegistry, natshandler.CoreRegistry)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stopAndWait(service.Stop, service.Wait, service.Cancel); err != nil {
			t.Error(err)
		}
	})
	if err := service.Run(); !errors.Is(err, jetstream.ErrStreamNameAlreadyInUse) {
		t.Fatalf("冲突资源应阻止继续注册，且保留 SDK 错误链: %v", err)
	}
	if _, err := client.JetStream().Consumer(ctx, "PIXIU_DEMO_TASKS", "demo-worker"); !errors.Is(err, jetstream.ErrConsumerNotFound) {
		t.Fatalf("Stream 冲突后仍创建了 Consumer: %v", err)
	}
}

func startExampleServer(t *testing.T) *server.Server {
	t.Helper()
	s, err := server.NewServer(&server.Options{
		Host:              "127.0.0.1",
		Port:              -1,
		JetStream:         true,
		JetStreamMaxStore: 2 << 30,
		StoreDir:          t.TempDir(),
		NoLog:             true,
		NoSigs:            true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		s.Shutdown()
		s.WaitForShutdown()
	})
	s.Start()
	if !s.ReadyForConnections(10 * time.Second) {
		t.Fatal("测试 NATS 未就绪")
	}
	return s
}

func newExampleClient(t *testing.T, url, name string, isDefault bool) *natsx.Client {
	t.Helper()
	newClient := natsx.NewNamed
	if isDefault {
		newClient = natsx.NewDefault
	}
	client, err := newClient(natsx.Config{
		Name: name,
		URLs: []string{url},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	return client
}

// 命名客户端和配置环境属于进程状态，按测试隔离以验证真实模板装配。
func initExampleProcess(t *testing.T) bool {
	t.Helper()
	const marker = "PIXIU_DEMO_NATS_EXAMPLE_TEST"
	if os.Getenv(marker) != t.Name() {
		cmd := exec.Command(os.Args[0], "-test.run=^"+regexp.QuoteMeta(t.Name())+"$")
		cmd.Env = append(os.Environ(), marker+"="+t.Name())
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("NATS 示例独立进程测试失败: %v\n%s", err, output)
		}
		return false
	}
	if err := env.Init(env.Config{
		AppName:       "pixiu-demo-nats-test",
		RunMode:       env.RunModeRelease,
		TimeLocation:  "UTC",
		ConfigDirPath: t.TempDir(),
		LocalIP:       "127.0.0.1",
	}); err != nil {
		t.Fatal(err)
	}
	return true
}

func writeExampleRegistration(t *testing.T, name string) {
	t.Helper()
	content, err := os.ReadFile("../conf.example/nats-register.demo.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.ConfigDirPath(), "nats-register."+name+".yaml"), content, 0o600); err != nil {
		t.Fatal(err)
	}
}

type exampleLog struct {
	Message  string `json:"msg"`
	Data     string `json:"data"`
	LogID    string `json:"logId"`
	Mode     string `json:"mode"`
	Stream   string `json:"stream"`
	Sequence uint64 `json:"sequence"`
}

type exampleLogWriter func([]byte) (int, error)

func (w exampleLogWriter) Write(p []byte) (int, error) {
	return w(p)
}
