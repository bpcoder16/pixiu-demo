package cron

import "github.com/bpcoder16/pixiu/biz/bootstrap"

// Registry 集中声明可由 cron.yaml 启用的任务，默认名称为函数名的小写形式。
var Registry = bootstrap.MustNewCronRegistry(
	Heartbeat,
)
