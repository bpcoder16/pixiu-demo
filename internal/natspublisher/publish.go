package natspublisher

import (
	"context"
	"fmt"

	"github.com/bpcoder16/pixiu/logit"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	taskSubject      = "pixiu.demo.tasks"
	broadcastSubject = "pixiu.demo.broadcast"
	queueSubject     = "pixiu.demo.queue"
)

// PublishBroadcast 发送广播消息，成功仅表示客户端受理，不等待消费完成。
func (p *Publisher) PublishBroadcast(ctx context.Context, data []byte) error {
	if err := p.client.Publish(ctx, broadcastSubject, data); err != nil {
		return fmt.Errorf("publish broadcast example: %w", err)
	}
	return nil
}

// PublishQueue 发送队列消息，成功仅表示客户端受理，不提供去重保证。
func (p *Publisher) PublishQueue(ctx context.Context, data []byte) error {
	if err := p.client.Publish(ctx, queueSubject, data); err != nil {
		return fmt.Errorf("publish queue example: %w", err)
	}
	return nil
}

// PublishTask 同步发送持久化任务，等待服务端 PubAck，不等待业务消费。
func (p *Publisher) PublishTask(ctx context.Context, data []byte, messageID string) error {
	ack, err := p.client.PublishJetStream(ctx, taskSubject, data, messageID)
	if err != nil {
		return fmt.Errorf("publish sync example: %w", err)
	}
	logPublishAck(ctx, "sync", ack)
	return nil
}

// PublishTaskAsync 异步提交持久化任务，再等待最终 PubAck 或当前操作取消。
func (p *Publisher) PublishTaskAsync(ctx context.Context, data []byte, messageID string) error {
	future, err := p.client.PublishJetStreamAsync(ctx, taskSubject, data, messageID)
	if err != nil {
		return fmt.Errorf("submit async example: %w", err)
	}
	// 提交成功后可继续其他工作；最终确认通过 future 获取，无需每条消息创建 goroutine。
	select {
	case ack := <-future.Ok():
		logPublishAck(ctx, "async", ack)
		return nil
	case ackErr := <-future.Err():
		return fmt.Errorf("ack async example: %w", ackErr)
	case <-ctx.Done():
		// 停止等待不撤销已提交消息；退出时共享客户端仍会等待未完成的异步确认。
		return ctx.Err()
	}
}

func logPublishAck(ctx context.Context, mode string, ack *jetstream.PubAck) {
	logit.Info(ctx, "DemoNATSPublish",
		logit.Str("mode", mode),
		logit.Str("stream", ack.Stream),
		logit.Uint64("sequence", ack.Sequence),
		logit.Bool("duplicate", ack.Duplicate),
	)
}
