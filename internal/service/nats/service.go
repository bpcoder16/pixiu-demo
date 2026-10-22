package nats

import "pixiu-demo/internal/natspublisher"

// Service 编排 NATS 示例消息的发布与消费业务，不管理连接。
type Service struct {
	publisher *natspublisher.Publisher
}

// NewService 创建使用默认 Publisher 的服务，不保存请求 Context。
// 复用已初始化的共享客户端；消费排空期间可创建，CloseAll 开始后不再调用。
func NewService() *Service {
	return &Service{
		publisher: natspublisher.NewPublisher(),
	}
}
