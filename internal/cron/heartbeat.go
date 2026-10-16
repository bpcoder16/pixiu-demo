package cron

import (
	"context"

	"github.com/bpcoder16/pixiu/logit"
)

// HeartbeatName 是心跳任务的稳定名称，与 cron.yaml 中的 name 对应。
const HeartbeatName = "heartbeat"

// Heartbeat 演示直接使用调度器传入的任务 context，沿用独立日志作用域和 logId。
func Heartbeat(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	logit.Info(ctx, "DemoCronHeartbeat")
	return nil
}
