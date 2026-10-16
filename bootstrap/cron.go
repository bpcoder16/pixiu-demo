package bootstrap

import (
	"pixiu-demo/internal/cron"

	"github.com/bpcoder16/pixiu/biz/bootstrap"
)

// CronHandlers 返回可由 cron.yaml 启用的任务清单，每次调用创建独立切片。
// 此处只绑定稳定名称和 Handler，是否启用及调度方式由配置决定。
func CronHandlers() []bootstrap.CronHandler {
	return []bootstrap.CronHandler{
		{
			Name:    cron.HeartbeatName,
			Handler: cron.Heartbeat,
		},
	}
}
