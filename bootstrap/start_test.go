package bootstrap

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	pixiubootstrap "github.com/bpcoder16/pixiu/biz/bootstrap"
	"github.com/bpcoder16/pixiu/biz/httpconfig"
	"github.com/bpcoder16/pixiu/infra/env"
	"github.com/bpcoder16/pixiu/infra/ginx"
	"github.com/gin-gonic/gin"
	"github.com/swaggo/swag"
)

func TestHTTPAssemblySmoke(t *testing.T) {
	isolatedHTTP(t, func() {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := listener.Addr().String()
		_ = listener.Close()
		configFile := filepath.Join(t.TempDir(), "http.yaml")
		if err := os.WriteFile(configFile, fmt.Appendf(nil, "addr: %q\nshutdownTimeout: 1s\n", addr), 0600); err != nil {
			t.Fatal(err)
		}
		appConfigFile := filepath.Join(filepath.Dir(configFile), "app.yaml")
		if err := os.WriteFile(appConfigFile, []byte("env:\n  appName: pixiu-demo-test\n  runMode: test\n  timeLocation: UTC\n  localIP: 127.0.0.1\n"), 0600); err != nil {
			t.Fatal(err)
		}
		httpconfig.MustLoadAppConfig(appConfigFile)
		if err := os.WriteFile(filepath.Join(filepath.Dir(configFile), "gin.yaml"), []byte("disableAccessLog: true\n"), 0600); err != nil {
			t.Fatal(err)
		}
		registerTestOpenAPIModules()
		manager, err := pixiubootstrap.NewAppManager()
		if err != nil {
			t.Fatal(err)
		}
		gin.SetMode(gin.ReleaseMode)
		service, err := newHTTPService()
		if err != nil {
			t.Fatal(err)
		}
		defer service.Shutdown()
		if gin.Mode() != env.RunMode() {
			t.Fatalf("Gin 模式未跟随应用配置: %s", gin.Mode())
		}
		if err := manager.Register(service); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- manager.Run(ctx) }()
		client := &http.Client{Timeout: time.Second}
		defer client.CloseIdleConnections()
		deadline := time.Now().Add(5 * time.Second)
		ready := false
		for time.Now().Before(deadline) {
			resp, err := client.Get("http://" + addr + "/api/app/ping")
			if err == nil {
				// 读完探活响应再复用连接，避免后续文档请求额外建连干扰短期限停机验证。
				_, readErr := io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				if readErr != nil {
					t.Fatal(readErr)
				}
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("API 端点状态: %d", resp.StatusCode)
				}
				if resp.Header.Get(ginx.TraceHeader) == "" {
					t.Fatal("HTTP 服务没有使用 ginx handler")
				}
				ready = true
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		if !ready {
			t.Fatal("服务未使用配置地址启动")
		}
		for _, path := range []string{"/openapi/app.json", "/openapi/admin.json", "/elements", "/elements/app", "/elements/admin"} {
			resp, err := client.Get("http://" + addr + path)
			if err != nil {
				t.Fatal(err)
			}
			_, readErr := io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				t.Fatal(readErr)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("文档端点 %s 未接入 HTTP 服务: %d", path, resp.StatusCode)
			}
		}
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("HTTP 未完成关闭")
		}
		if resp, err := client.Get("http://" + addr + "/api/app/ping"); err == nil {
			_ = resp.Body.Close()
			t.Fatal("关闭后仍接受请求")
		}
	})
}

func registerTestOpenAPIModules() {
	for _, name := range []string{"app", "admin"} {
		swag.Register(name, &swag.Spec{SwaggerTemplate: fmt.Sprintf(`{"swagger":"2.0","info":{"title":%q,"version":"1.0"},"paths":{}}`, name)})
	}
}

// env 只能初始化一次，使用独立进程隔离各测试的配置目录。
func isolatedHTTP(t *testing.T, run func()) {
	t.Helper()
	const marker = "PIXIU_DEMO_HTTP_TEST"
	if os.Getenv(marker) == t.Name() {
		run()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^"+regexp.QuoteMeta(t.Name())+"$")
	cmd.Env = append(os.Environ(), marker+"="+t.Name())
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("HTTP 独立进程测试失败: %v\n%s", err, output)
	}
}
