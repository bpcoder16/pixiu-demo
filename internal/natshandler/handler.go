package natshandler

import (
	"context"

	natsservice "pixiu-demo/internal/service/nats"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// HandleTask 处理持久化消息；完成 Service 调用后返回，由 natsx 负责 ACK 或重投。
func HandleTask(ctx context.Context, msg jetstream.Msg) error {
	return natsservice.NewService().HandleTask(ctx, msg)
}

// HandleBroadcast 处理广播入口，失败由组件记录日志，不自动重投。
func HandleBroadcast(ctx context.Context, msg *nats.Msg) error {
	return natsservice.NewService().HandleBroadcast(ctx, msg)
}

// HandleQueue 处理队列入口，同组订阅可并发调用，失败不自动重投。
func HandleQueue(ctx context.Context, msg *nats.Msg) error {
	return natsservice.NewService().HandleQueue(ctx, msg)
}
