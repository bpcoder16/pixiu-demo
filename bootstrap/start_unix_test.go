//go:build unix

package bootstrap

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/bpcoder16/pixiu/infra/ginx"
	"github.com/bpcoder16/pixiu/logit"
)

type cronLogWriter func([]byte) (int, error)

func (w cronLogWriter) Write(p []byte) (int, error) { return w(p) }

func TestNATSHTTPAndCronStartup(t *testing.T) {
	// 信号仅发送给 isolatedHTTP 创建的测试子进程，避免影响测试主进程。
	isolatedHTTP(t, func() {
		addr := initMixedStartConfig(t)
		natsClient := newExampleClient(t, startExampleServer(t), "demo", true)
		writeExampleRegistration(t, "demo")
		// 混合启动不启用分布式锁以隔离外部 Redis；显式启用锁时的 Redis 前置约束另有覆盖。
		writeStartConfig(t, "cron.yaml", "jobs:\n  - name: Heartbeat\n    every: 100ms\n    timeout: 2s\n")
		heartbeat := make(chan map[string]any, 1)
		var runs atomic.Int32
		var shutdowns []string
		var shutdownMu sync.Mutex
		logger := logit.MustNew(
			logit.OptEncoder(logit.DefaultJSONEncoder),
			logit.OptWriter(logit.NewWriter(cronLogWriter(func(p []byte) (int, error) {
				var record map[string]any
				if err := json.Unmarshal(p, &record); err != nil {
					t.Errorf("无法解析服务日志: %v", err)
					return len(p), nil
				}
				switch record["msg"] {
				case "DemoCronHeartbeat":
					runs.Add(1)
					select {
					case heartbeat <- record:
					default:
					}
				case "AppServiceStopped":
					shutdownMu.Lock()
					shutdowns = append(shutdowns, record["service"].(string))
					shutdownMu.Unlock()
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
		done := make(chan error, 1)
		go func() { done <- Start() }()
		client := &http.Client{Timeout: time.Second}
		defer client.CloseIdleConnections()
		deadline := time.Now().Add(5 * time.Second)
		ready := false
		for time.Now().Before(deadline) {
			response, err := client.Get("http://" + addr + "/api/app/ping")
			if err == nil {
				_, readErr := io.Copy(io.Discard, response.Body)
				_ = response.Body.Close()
				if readErr != nil || response.StatusCode != http.StatusOK || response.Header.Get(ginx.TraceHeader) == "" {
					t.Fatalf("HTTP 未使用项目路由及 Gin 中间件: status=%d, err=%v", response.StatusCode, readErr)
				}
				ready = true
				break
			}
			select {
			case err := <-done:
				t.Fatalf("应用提前退出: %v", err)
			default:
			}
			time.Sleep(5 * time.Millisecond)
		}
		if !ready {
			t.Fatal("HTTP 未就绪")
		}
		// HTTP 已监听意味着 RunSignals 已安装信号处理，此后失败也须停止三个服务。
		stop := sync.OnceFunc(func() {
			process, err := os.FindProcess(os.Getpid())
			if err != nil {
				t.Fatal(err)
			}
			if err := process.Signal(syscall.SIGTERM); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("混合服务关闭失败: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("HTTP 与 Cron 未完成关闭")
			}
		})
		defer stop()
		select {
		case record := <-heartbeat:
			if record["job_name"] != "Heartbeat" || record["logId"] == nil {
				t.Fatalf("心跳日志缺少任务作用域字段: %v", record)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("未执行示例心跳任务")
		}
		stop()
		slices.Sort(shutdowns)
		if !slices.Equal(shutdowns, []string{"cron", "http", "nats:demo"}) {
			t.Fatalf("完成停收的服务 = %v", shutdowns)
		}
		if natsClient.Conn().IsClosed() {
			t.Fatal("Start 应只停止消费，保留 NATS 连接供资源栈关闭")
		}
		completed := runs.Load()
		time.Sleep(250 * time.Millisecond)
		if runs.Load() != completed {
			t.Fatal("关闭完成后仍触发 Cron 任务")
		}
		if response, err := client.Get("http://" + addr + "/api/app/ping"); err == nil {
			_ = response.Body.Close()
			t.Fatal("关闭完成后 HTTP 仍接受请求")
		}
	})
}
