package database

import "github.com/bpcoder16/pixiu/infra/schemamigrate"

// Tables 返回本项目拥有并允许自动迁移的物理表清单。
// 新业务 Model 在此登记一次，启用迁移时由 Pixiu 调用 GORM AutoMigrate。
func Tables() []schemamigrate.Table {
	// 示例尚未登记业务表；新增 Model 时在此集中登记。
	return nil
}
