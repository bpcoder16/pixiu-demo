package bootstrap

import (
	"pixiu-demo/internal/cronhandler"
	"pixiu-demo/internal/infra/nats"
	"pixiu-demo/internal/natshandler"
	"pixiu-demo/internal/route"
	"pixiu-demo/internal/wshandler"

	"github.com/bpcoder16/pixiu/biz/bootstrap"
	"github.com/bpcoder16/pixiu/infra/apprun"
	"github.com/bpcoder16/pixiu/infra/ginx"
)

// Start 构造 NATS、独立端口的 WebSocket、Gin HTTP 和 Cron，统一接管 SIGINT/SIGTERM。
// 构造时串行设置 Gin 全局模式，运行期间不切换模式。
// 收到信号统一停收，最多共享 30 秒等待实际排空；超时取消剩余任务。
// main 始终回收共享资源，默认总回收期限 20 秒，日志最后关闭。
func Start() error {
	return bootstrap.Start(
		// NATS 消费：读取 nats-register.demo.yaml，绑定 JetStream/Core 处理器，运行时注册订阅。
		func() (apprun.Service, error) {
			return bootstrap.NewNATSConsumerService(
				nats.DemoName,
				natshandler.JetStreamRegistry,
				natshandler.CoreRegistry,
			)
		},
		// WebSocket：读取 websocket.app.yaml，独立监听并装配消息服务与就绪探针。
		func() (apprun.Service, error) {
			return bootstrap.NewWebSocketService(
				"app",
				wshandler.NewServer,
			)
		},
		// Gin HTTP：读取 gin.yaml 和 http.yaml，登记业务路由，声明 App/Admin OpenAPI 模块。
		func() (apprun.Service, error) {
			return bootstrap.NewGinHTTPService(
				route.Register,
				ginx.OpenAPIModule{Name: "app", Title: "App"},
				ginx.OpenAPIModule{Name: "admin", Title: "Admin"},
			)
		},
		// Cron：读取 cron.yaml，按配置启用注册表中的任务处理器。
		func() (apprun.Service, error) {
			return bootstrap.NewCronService(cronhandler.Registry.Handlers()...)
		},
	)
}
