package mysql

import (
	"context"

	"github.com/bpcoder16/pixiu/infra/mysqlx"
	"gorm.io/gorm"
)

// DemoName 是 demo 逻辑库的实例名，供初始化时声明默认实例。
const DemoName = "demo"

// MasterDB 返回带 ctx 的默认主库会话；bootstrap 将 demo 注册为默认实例。
func MasterDB(ctx context.Context) *gorm.DB {
	return mysqlx.Default().MasterDB(ctx)
}

// SlaveDB 返回带 ctx 的默认从库会话；未配置从库时使用主库。
func SlaveDB(ctx context.Context) *gorm.DB {
	return mysqlx.Default().SlaveDB(ctx)
}
