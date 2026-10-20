package cron

import "github.com/bpcoder16/pixiu/biz/bootstrap"

// Registry 集中声明可由 cron.yaml 启用的任务，默认名称保留函数名的大小写，配置须完全一致。
var Registry = bootstrap.MustNewCronRegistry(
	Heartbeat,
)
