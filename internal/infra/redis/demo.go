package redis

import (
	"github.com/bpcoder16/pixiu/infra/redisx"
	goredis "github.com/redis/go-redis/v9"
)

// DemoName 是 demo Redis 的实例名，供初始化时声明默认实例。
const DemoName = "demo"

// Client 返回共享的默认 Redis 客户端，命令执行时传入本次操作的 context。
func Client() *goredis.Client {
	return redisx.Default().Client()
}
