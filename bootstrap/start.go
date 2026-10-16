package bootstrap

import (
	"pixiu-demo/internal/cron"
	"pixiu-demo/internal/route"

	"github.com/bpcoder16/pixiu/biz/bootstrap"
	"github.com/bpcoder16/pixiu/infra/apprun"
	"github.com/bpcoder16/pixiu/infra/ginx"
)

// Start 读取 gin.yaml、http.yaml 和 cron.yaml，统一启动 HTTP、Cron 并接管 SIGINT/SIGTERM。
// 启动阶段串行设置 Gin 全局模式，再构造路由；运行期间不切换模式。
// 先构造 HTTP 再构造 Cron，正常关闭时先排空 Cron 再关闭 HTTP，之后由 main 回收共享资源。
// RunSignals 内部创建后台 context，管理停机通知及信号订阅。
// main 记录运行或关闭错误，在管理器返回后回收共享客户端和日志，然后正常返回。
// HTTP 关闭超时后不额外等待 handler,直接进入资源清理。
// 总关闭超时返回 ErrShutdownTimeout，main 仍尽力清理资源。
// 此处不重复登记资源关闭函数。
func Start() error {
	return bootstrap.Start(
		// GinHTTPService
		func() (apprun.Service, error) {
			return bootstrap.NewGinHTTPService(
				route.Register,
				ginx.OpenAPIModule{Name: "app", Title: "App"},
				ginx.OpenAPIModule{Name: "admin", Title: "Admin"},
			)
		},
		// CronScheduler
		func() (apprun.Service, error) {
			scheduler, err := bootstrap.NewCronScheduler(cron.Registry.Handlers()...)
			if err != nil {
				return apprun.Service{}, err
			}
			return bootstrap.CronService(scheduler), nil
		},
	)
}
