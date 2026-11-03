package natshandler

import "github.com/bpcoder16/pixiu/infra/natsx"

// JetStreamRegistry 集中声明 JetStream 消费函数，名称与配置保持一致。
var JetStreamRegistry = natsx.MustNewJetStreamHandlerRegistry(
	HandleTask,
)

// CoreRegistry 只登记函数，不在包初始化阶段获取共享实例或启动消费。
var CoreRegistry = natsx.MustNewCoreHandlerRegistry(
	HandleBroadcast,
	HandleQueue,
	HandleAppPush,
)
