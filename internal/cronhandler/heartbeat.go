package cronhandler

import (
	"context"

	"github.com/bpcoder16/pixiu/logit"
)

// Heartbeat 演示直接使用调度器传入的任务 context，沿用独立日志作用域和 logId。
func Heartbeat(ctx context.Context) error {
	logit.Info(ctx, "DemoCronHeartbeat")
	return nil
}
