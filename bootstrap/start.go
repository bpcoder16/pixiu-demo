package bootstrap

import (
	"pixiu-demo/internal/cronhandler"
	"pixiu-demo/internal/infra/nats"
	"pixiu-demo/internal/natshandler"
	"pixiu-demo/internal/route"

	"github.com/bpcoder16/pixiu/biz/bootstrap"
	"github.com/bpcoder16/pixiu/infra/apprun"
	"github.com/bpcoder16/pixiu/infra/ginx"
)

// Start 构造 NATS 消费组件、HTTP、Cron，统一运行并接管 SIGINT/SIGTERM。
// 构造时串行设置 Gin 全局模式，运行期间不切换模式。
// 收到信号统一停收 HTTP、Cron 和 NATS，最多共享 30 秒等待实际排空。
// 排空超时取消剩余任务；main 始终回收共享资源，默认总回收期限 20 秒，日志最后关闭。
func Start() error {
	return bootstrap.Start(
		// NewNATSConsumerService
		func() (apprun.Service, error) {
			return bootstrap.NewNATSConsumerService(
				nats.DemoName,
				natshandler.JetStreamRegistry,
				natshandler.CoreRegistry,
			)
		},
		// GinHTTPService
		func() (apprun.Service, error) {
			return bootstrap.NewGinHTTPService(
				route.Register,
				ginx.OpenAPIModule{Name: "app", Title: "App"},
				ginx.OpenAPIModule{Name: "admin", Title: "Admin"},
			)
		},
		// NewCronService
		func() (apprun.Service, error) {
			return bootstrap.NewCronService(cronhandler.Registry.Handlers()...)
		},
	)
}
