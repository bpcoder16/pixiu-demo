package nats

import (
	"context"

	"github.com/bpcoder16/pixiu/logit"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// HandleTask 完成持久化任务的示例业务，返回后由消息组件确认或重投。
func (s *Service) HandleTask(ctx context.Context, msg jetstream.Msg) error {
	logit.Info(ctx, "DemoNATSJetStream", logit.Str("data", string(msg.Data())))
	return nil
}

// HandleBroadcast 处理每个在线实例收到的广播示例。
func (s *Service) HandleBroadcast(ctx context.Context, msg *nats.Msg) error {
	logit.Info(ctx, "DemoNATSCoreBroadcast", logit.Str("data", string(msg.Data)))
	return nil
}

// HandleQueue 处理同组竞争取得的队列示例。
func (s *Service) HandleQueue(ctx context.Context, msg *nats.Msg) error {
	logit.Info(ctx, "DemoNATSCoreQueue", logit.Str("data", string(msg.Data)))
	return nil
}
