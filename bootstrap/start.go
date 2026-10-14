package bootstrap

import (
	"errors"

	"pixiu-demo/internal/route"

	"github.com/bpcoder16/pixiu/biz/bootstrap"
	"github.com/bpcoder16/pixiu/infra/apprun"
	"github.com/bpcoder16/pixiu/infra/env"
	"github.com/bpcoder16/pixiu/infra/ginx"
	"github.com/gin-gonic/gin"
)

// Start 读取 gin.yaml 和 http.yaml，按运行模式注册文档，启动 HTTP 并接管 SIGINT/SIGTERM。
// 启动阶段串行设置 Gin 全局模式，再构造路由；运行期间不切换模式。
// RunSignals 内部创建后台 context，管理停机通知及信号订阅。
// main 记录运行或关闭错误，在管理器返回后回收共享客户端和日志，然后正常返回。
// HTTP 关闭超时后不额外等待 handler,直接进入资源清理。
// 总关闭超时返回 ErrShutdownTimeout，main 仍尽力清理资源。
// 此处不重复登记资源关闭函数。
func Start() error {
	manager, err := bootstrap.NewAppManager()
	if err != nil {
		return err
	}
	httpService, httpServiceErr := newHTTPService()
	if httpServiceErr != nil {
		return httpServiceErr
	}
	if registerErr := manager.Register(httpService); registerErr != nil {
		return errors.Join(registerErr, httpService.Shutdown())
	}
	return manager.RunSignals()
}

// newHTTPService 在启动阶段设置 Gin 模式，读取配置并构造挂载项目路由的 HTTP 服务。
// 调用前须初始化 env；服务由调用方注册，监听在 Run 时才启动。
func newHTTPService() (apprun.Service, error) {
	gin.SetMode(env.RunMode())
	handler, err := bootstrap.NewGinEngine(
		ginx.OpenAPIModule{Name: "app", Title: "App"},
		ginx.OpenAPIModule{Name: "admin", Title: "Admin"},
	)
	if err != nil {
		return apprun.Service{}, err
	}
	route.Register(handler)
	return bootstrap.NewHTTPService(handler)
}
