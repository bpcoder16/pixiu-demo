package aliyunes

import "github.com/bpcoder16/pixiu/infra/aliyun/esx"

// DemoName 是 demo 阿里云 ES 的实例名，供初始化时声明默认实例。
const DemoName = "demo"

// Client 返回共享的默认阿里云 ES 管理客户端，每次操作传入本次请求的 context。
// 使用方不自行关闭或修改客户端配置。
func Client() *esx.Client {
	return esx.Default()
}
