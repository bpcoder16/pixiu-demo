package nats

import "github.com/bpcoder16/pixiu/infra/natsx"

// DemoName 是 demo NATS 的实例名，供初始化时声明默认实例。
const DemoName = "demo"

// Client 返回共享的默认 NATS 客户端；消费排空期间仍可查询并发布。
// 未初始化或 CloseAll 完成后调用会 panic。
func Client() *natsx.Client {
	return natsx.Default()
}
