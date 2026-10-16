package bootstrap

import (
	"fmt"

	"pixiu-demo/internal/database"
	"pixiu-demo/internal/infra/aliyunes"
	"pixiu-demo/internal/infra/aliyunoss"
	"pixiu-demo/internal/infra/elasticsearch"
	"pixiu-demo/internal/infra/mysql"
	"pixiu-demo/internal/infra/nats"
	"pixiu-demo/internal/infra/redis"

	"github.com/bpcoder16/pixiu/biz/appconfig"
	"github.com/bpcoder16/pixiu/biz/bootstrap"
	"github.com/bpcoder16/pixiu/lifecycle"
)

// MustInit 声明 MySQL、Redis、Elasticsearch、阿里云 OSS、阿里云 ES 和 NATS 实例，再执行通用和项目初始化。
// 两层共用由 main 创建和关闭的资源栈，初始化失败时 panic。
func MustInit(config *appconfig.AppConfig, resources *lifecycle.Stack) {
	bootstrap.MustRegisterMySQL(mysql.DemoName, true)
	bootstrap.MustRegisterRedis(redis.DemoName, true)
	bootstrap.MustRegisterElasticsearch(elasticsearch.DemoName, true)
	bootstrap.MustRegisterAliyunOSS(aliyunoss.DemoName, true)
	bootstrap.MustRegisterAliyunES(aliyunes.DemoName, true)
	bootstrap.MustRegisterNATS(nats.DemoName, true)
	bootstrap.MustBaseInit(config, resources)

	// 共享实例就绪后迁移登记的业务表；失败由 main 的资源栈收尾。
	if _, err := bootstrap.RunMySQLMigration(mysql.DemoName, database.Tables()); err != nil {
		panic(fmt.Errorf("bootstrap: migrate database: %w", err))
	}
}
