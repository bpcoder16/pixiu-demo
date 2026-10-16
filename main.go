package main

import (
	"log"

	"pixiu-demo/bootstrap"

	// 加载生成的 OpenAPI 文档注册逻辑。
	_ "pixiu-demo/openapi"

	"github.com/bpcoder16/pixiu/biz/appconfig"
	"github.com/bpcoder16/pixiu/lifecycle"
)

func main() {
	conf := appconfig.MustLoadAppConfig("./conf/app.yaml")

	var resources lifecycle.Stack
	// 提前登记收尾，确保初始化 panic 时也能释放已创建的资源。
	defer func() {
		if err := resources.Close(); err != nil {
			// 使用标准日志，避免依赖应用日志的关闭状态。
			log.Printf("close resources: %v", err)
		}
	}()

	bootstrap.MustInit(conf, &resources)
	if err := bootstrap.Start(); err != nil {
		log.Printf("server exit: %v", err)
	}
}
