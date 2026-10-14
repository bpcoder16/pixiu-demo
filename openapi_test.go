package main

import (
	"strings"
	"testing"

	"github.com/swaggo/swag"
)

func TestOpenAPIContract(t *testing.T) {
	// 保留公共约定和固定的 Ping 样例检查，新增接口无需逐项追加断言。
	for _, module := range []string{"app", "admin"} {
		t.Run(module, func(t *testing.T) {
			// 解析注释源，避免依赖未生成或过期的文档产物。
			parser := swag.New(swag.SetParseDependency(0), swag.SetParseExtension("module-"+module))
			if err := parser.ParseAPI(".", "openapi/"+module+".go", 100); err != nil {
				t.Fatal(err)
			}
			doc := parser.GetSwagger()
			responseName := "AppPingResponse"
			if module == "admin" {
				responseName = "AdminPingResponse"
			}
			for path := range doc.Paths.Paths {
				if !strings.HasPrefix(path, "/api/"+module+"/") {
					t.Fatalf("模块文档混入其他端的接口: %s", path)
				}
			}
			op := doc.Paths.Paths["/api/"+module+"/ping"].Get
			if op == nil {
				t.Fatal("缺少当前模块的 Ping")
			}
			if module == "app" {
				if len(op.Parameters) != 1 {
					t.Fatalf("App Ping 应有一个可选查询参数: %v", op.Parameters)
				}
				param := op.Parameters[0]
				if param.Name != "name" || param.In != "query" || param.Type != "string" || param.Required || param.MaxLength == nil || *param.MaxLength != 32 {
					t.Fatalf("name 参数契约错误: %+v", param)
				}
			} else if len(op.Parameters) != 0 {
				t.Fatal("Admin Ping 不应要求参数")
			}
			response := op.Responses.StatusCodeResponses[200]
			if response.Schema == nil {
				t.Fatal("Ping 应使用 HTTP 200 和统一响应")
			}
			found := false
			for _, schema := range response.Schema.AllOf {
				data := schema.Properties["data"]
				if data.Ref.String() == "#/definitions/response."+responseName {
					found = true
				}
			}
			if !found {
				t.Fatal("成功响应的 data 类型丢失")
			}
		})
	}
}
