//go:build unix

package bootstrap

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/bpcoder16/pixiu/biz/realtime"
	"github.com/bpcoder16/pixiu/biz/realtime/protocol"
	"github.com/bpcoder16/pixiu/infra/env"
	"github.com/bpcoder16/pixiu/infra/ginx"
	"github.com/bpcoder16/pixiu/logit"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type cronLogWriter func([]byte) (int, error)

func (w cronLogWriter) Write(p []byte) (int, error) { return w(p) }

func TestNATSHTTPWebSocketAndCronStartup(t *testing.T) {
	// 信号仅发送给 isolatedHTTP 创建的测试子进程，避免影响测试主进程。
	isolatedHTTP(t, func() {
		addr := initMixedStartConfig(t)
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		wsAddr := listener.Addr().String()
		_ = listener.Close()
		writeStartConfig(t, "websocket.app.yaml", fmt.Sprintf("addr: %q\npath: /ws/app\n", wsAddr))
		natsClient := newExampleClient(t, startExampleServer(t).ClientURL(), "demo", true)
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
		gin.SetMode(gin.TestMode)
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
		// HTTP 已监听意味着 RunSignals 已安装信号处理，此后失败也须停止四个服务。
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
				t.Fatal("HTTP、WebSocket、NATS 与 Cron 未完成关闭")
			}
		})
		defer stop()
		// 当前混合启动使用 release，文档应保持关闭。
		for _, tc := range []struct {
			path   string
			status int
		}{
			{"/api/admin/ping", http.StatusOK},
			{"/ws/app", http.StatusNotFound},
			{"/ready/ws", http.StatusNotFound},
			{"/ws-demo", http.StatusNotFound},
			{"/openapi/app.json", http.StatusNotFound},
			{"/openapi/admin.json", http.StatusNotFound},
			{"/docs", http.StatusNotFound},
		} {
			response, err := client.Get("http://" + addr + tc.path)
			if err != nil {
				t.Fatal(err)
			}
			_, readErr := io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if readErr != nil || response.StatusCode != tc.status || response.Header.Get(ginx.TraceHeader) == "" {
				t.Fatalf("Gin API/文档未保留: path=%s, status=%d, err=%v", tc.path, response.StatusCode, readErr)
			}
		}
		// HTTP 与 WebSocket 并发 Run，分别检查就绪，再保留一条升级后的连接验证统一停机。
		deadline = time.Now().Add(5 * time.Second)
		for {
			response, err := client.Get("http://" + wsAddr + "/ready/ws")
			if err != nil {
				if time.Now().After(deadline) {
					t.Fatal(err)
				}
				time.Sleep(time.Millisecond)
				continue
			}
			_ = response.Body.Close()
			if response.Header.Get(ginx.TraceHeader) != "" {
				t.Fatal("WebSocket 探活不应经过 Gin 请求作用域")
			}
			if response.StatusCode == http.StatusNoContent {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("WebSocket 未就绪")
			}
			time.Sleep(time.Millisecond)
		}
		// WebSocket 独立端口不提供调试页或其他普通 HTTP 路由。
		for _, path := range []string{"/", "/ws-demo", "/api/app/ping"} {
			response, err := client.Get("http://" + wsAddr + path)
			if err != nil {
				t.Fatal(err)
			}
			_, readErr := io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if readErr != nil || response.StatusCode != http.StatusNotFound || response.Header.Get(ginx.TraceHeader) != "" {
				t.Fatalf("WebSocket 非服务路径应返回 404: path=%s status=%d err=%v", path, response.StatusCode, readErr)
			}
		}
		dialer := websocket.Dialer{
			Subprotocols:     []string{realtime.SubprotocolJSON},
			HandshakeTimeout: 5 * time.Second,
		}
		connection, handshake, err := dialer.Dial("ws://"+wsAddr+"/ws/app", http.Header{"Origin": {"http://" + wsAddr}})
		if err != nil {
			t.Fatal(err)
		}
		defer connection.Close()
		if handshake.Header.Get(ginx.TraceHeader) != "" {
			t.Fatal("WebSocket 握手不应经过 Gin 请求作用域")
		}
		if err := connection.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
		var welcome struct {
			Kind protocol.Kind `json:"kind"`
			Type string        `json:"type"`
		}
		if err := connection.ReadJSON(&welcome); err != nil || welcome.Kind != protocol.Kind_EVENT || welcome.Type != "session.connected" {
			t.Fatalf("Start 未接通 WebSocket: %+v, err=%v", welcome, err)
		}
		select {
		case record := <-heartbeat:
			if record["job_name"] != "Heartbeat" || record["logId"] == nil {
				t.Fatalf("心跳日志缺少任务作用域字段: %v", record)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("未执行示例心跳任务")
		}
		stop()
		if gin.Mode() != env.RunMode() {
			t.Fatalf("Gin 模式未跟随应用配置: %s", gin.Mode())
		}
		if err := connection.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, _, err := connection.ReadMessage(); !websocket.IsCloseError(err, websocket.CloseGoingAway) {
			t.Fatalf("升级后的 WebSocket 未随 Start 关闭: %v", err)
		}
		slices.Sort(shutdowns)
		if !slices.Equal(shutdowns, []string{"cron", "http", "nats:demo", "websocket:app"}) {
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
		if response, err := client.Get("http://" + wsAddr + "/ready/ws"); err == nil {
			_ = response.Body.Close()
			t.Fatal("关闭完成后 WebSocket 监听仍接受请求")
		}
		if response, err := client.Get("http://" + addr + "/api/app/ping"); err == nil {
			_ = response.Body.Close()
			t.Fatal("关闭完成后 HTTP 仍接受请求")
		}
	})
}
