package elasticsearch

import "github.com/bpcoder16/pixiu/infra/elasticsearchx"

// DemoName 是 demo Elasticsearch 的实例名，供初始化时声明默认实例。
const DemoName = "demo"

// Client 返回共享的默认客户端，每次操作传入本次请求的 context。
func Client() *elasticsearchx.Client {
	return elasticsearchx.Default()
}
