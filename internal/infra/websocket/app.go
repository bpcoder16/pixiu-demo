package websocket

import "github.com/bpcoder16/pixiu/biz/realtime"

// AppName 是 App 端的实例名，与 websocket.app.yaml 及启动声明保持一致。
const AppName = "app"

// App 返回已初始化的 App WebSocket 实例。运行和关闭由 Service 管理。
func App() *realtime.Server {
	return realtime.Named(AppName)
}
