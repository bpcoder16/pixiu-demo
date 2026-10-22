package natshandler

import "github.com/bpcoder16/pixiu/infra/natsx"

// JetStreamRegistry 集中声明 JetStream 消费函数，默认名称保留函数名的大小写，配置须完全一致。
var JetStreamRegistry = natsx.MustNewJetStreamHandlerRegistry(
	HandleTask,
)

// CoreRegistry 集中声明 Core 消费函数，默认名称保留函数名的大小写，配置须完全一致。
var CoreRegistry = natsx.MustNewCoreHandlerRegistry(
	HandleBroadcast,
	HandleQueue,
)
