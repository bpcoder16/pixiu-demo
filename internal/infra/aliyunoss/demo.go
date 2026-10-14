package aliyunoss

import "github.com/bpcoder16/pixiu/infra/aliyun/ossx"

// DemoName 是 demo 阿里云 OSS 的实例名，供初始化时声明默认实例。
const DemoName = "demo"

// Client 返回共享的默认 OSS 客户端，每次操作传入本次请求的 context。
// 使用方不自行关闭或修改客户端配置。
func Client() *ossx.Client {
	return ossx.Default()
}
