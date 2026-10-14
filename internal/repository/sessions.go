package repository

import (
	"pixiu-demo/internal/infra/mysql"

	"github.com/bpcoder16/pixiu/infra/gormx"
)

// 只保存会话获取函数，查询时才访问已初始化的默认 MySQL。
var defaultSessions = gormx.NewSessions(mysql.MasterDB, mysql.SlaveDB)

// DefaultSessions 返回项目默认主、从库会话入口，供各 Repository 的无参构造函数使用。
// 连接池由 bootstrap 初始化并统一关闭；本函数不访问数据库。
func DefaultSessions() *gormx.Sessions {
	return defaultSessions
}
