package natspublisher

import (
	"pixiu-demo/internal/infra/nats"

	"github.com/bpcoder16/pixiu/infra/natsx"
)

// Publisher 负责 NATS 消息发送，借用并持有客户端，不初始化或关闭连接。
type Publisher struct {
	client *natsx.Client
}

// NewPublisher 在依赖就绪后取得默认客户端，并保存供后续发布使用。
// 可提前构造复用，也可按调用创建；发送入口关闭后发布返回 natsx.ErrClosed。
func NewPublisher() *Publisher {
	return &Publisher{
		client: nats.Client(),
	}
}
