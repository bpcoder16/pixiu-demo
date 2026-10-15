// Package openapi 保留包入口，使未生成接口文档时仍可编译。
//
// 在项目根目录执行 go generate ./openapi，使用与 go.mod 一致的 swag v1.16.6。
// app.go、admin.go 分别声明文档元信息；Handler 的 @x-module-app true / @x-module-admin true 标记决定模块归属。
// 生成命令通过 --parseExtension module-app / module-admin 筛选带对应标记的接口；未标记的接口不会收录。
// 接口信息维护在 Handler 注释和请求/响应结构中，新增接口无需在本文件逐项登记或说明。
// @Tags 仅用于文档内的左侧业务分组（如“样例接口”“用户管理”），同名分组可以出现在不同模块中。
// 顶部导航由 bootstrap.NewGinEngine 传入的 OpenAPIModule.Name/Title 决定，与 @Tags 无关。
// 生成格式为 OpenAPI 2.0（Swagger 2.0），分别生成 app/admin 前缀的 docs.go、swagger.json 和 swagger.yaml。
// 生成物不纳入 Git；修改注释后、构建需要文档的部署版本前，应重新运行该命令。
// main 的 blank import 将生成文档编译并注册到程序；部署时不必携带 JSON/YAML 文件。
//
// Pixiu bootstrap.NewGinEngine 仅根据 env.RunMode() 自动注册文档：debug/test 模式提供以下入口，release 模式关闭：
//
//	/docs               默认展示 App，顶部可切换模块
//	/docs/app           App 文档
//	/docs/admin         Admin 文档
//	/openapi/app.json   App OpenAPI 2.0 文档
//	/openapi/admin.json Admin OpenAPI 2.0 文档
//
// 两个模块共用程序内嵌的 Elements 资源，各自读取独立文档；不再注册 /openapi/doc.json。
// Elements 使用 hash 导航，支持直接刷新接口详情页。
// debug/test 模式未生成文档时启动返回错误，release 模式不要求生成文档。
// 不读取独立文档配置。
// 同一个 HTTP 状态码只保留一条响应定义，业务失败码和 data 省略规则写入其描述，避免覆盖成功结构。
//
//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --dir .. --generalInfo openapi/app.go --output . --instanceName app --parseExtension module-app --parseInternal
//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --dir .. --generalInfo openapi/admin.go --output . --instanceName admin --parseExtension module-admin --parseInternal
package openapi
