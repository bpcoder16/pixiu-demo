package nats

import (
	"context"
	"uuid"
)

// PublishExamples 编排一组示例消息，等待发布确认，不等待消费完成。
// 每次调用生成新的 UUID 批次前缀；重复调用会重新发布，部分失败不回滚。
func (s *Service) PublishExamples(ctx context.Context) error {
	messageIDPrefix := uuid.New().String()
	if err := s.publisher.PublishBroadcast(ctx, []byte("broadcast example")); err != nil {
		return err
	}
	if err := s.publisher.PublishQueue(ctx, []byte("queue example")); err != nil {
		return err
	}
	if err := s.publisher.PublishTask(ctx, []byte("sync example"), messageIDPrefix+":sync"); err != nil {
		return err
	}
	return s.publisher.PublishTaskAsync(ctx, []byte("async example"), messageIDPrefix+":async")
}
