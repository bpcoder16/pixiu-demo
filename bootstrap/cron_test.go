package bootstrap

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bpcoder16/pixiu/infra/env"
)

func TestCronConfigurationStopsStartup(t *testing.T) {
	for _, name := range []string{"missing", "unknown-handler", "unregistered-redis"} {
		t.Run(name, func(t *testing.T) {
			isolatedHTTP(t, func() {
				addr := initMixedStartConfig(t)
				if name == "unknown-handler" {
					writeStartConfig(t, "cron.yaml", "jobs:\n  - name: unknown\n    every: 1s\n")
				}
				if name == "unregistered-redis" {
					template, err := os.ReadFile("../conf.example/cron.yaml")
					if err != nil {
						t.Fatal(err)
					}
					writeStartConfig(t, "cron.yaml", string(template))
				}
				done := make(chan error, 1)
				go func() { done <- Start() }()
				select {
				case err := <-done:
					if name == "missing" && !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("缺失 cron.yaml 应保留文件错误: %v", err)
					}
					if name == "unknown-handler" && (err == nil || !strings.Contains(err.Error(), "unknown Cron handler")) {
						t.Fatalf("未拒绝未知 Cron 任务: %v", err)
					}
					if name == "unregistered-redis" && (err == nil || !strings.Contains(err.Error(), `Redis "demo"`)) {
						t.Fatalf("模板未要求已初始化的 demo Redis: %v", err)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("Cron 配置错误未阻止应用启动")
				}
				listener, err := net.Listen("tcp", addr)
				if err != nil {
					t.Fatalf("Cron 构造失败后 HTTP 地址仍被占用: %v", err)
				}
				_ = listener.Close()
			})
		})
	}
}

func initMixedStartConfig(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	if err := env.Init(env.Config{
		AppName:       "pixiu-demo-cron-test",
		RunMode:       env.RunModeRelease,
		TimeLocation:  "UTC",
		ConfigDirPath: t.TempDir(),
		LocalIP:       "127.0.0.1",
	}); err != nil {
		t.Fatal(err)
	}
	writeStartConfig(t, "gin.yaml", "disableAccessLog: true\n")
	writeStartConfig(t, "http.yaml", fmt.Sprintf("addr: %q\nshutdownTimeout: 1s\n", addr))
	return addr
}

func writeStartConfig(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(env.ConfigDirPath(), name), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
