package handler

import (
	"pixiu-demo/internal/model/request"
	"pixiu-demo/internal/model/response"

	"github.com/gin-gonic/gin"
)

// AppPing 骨架示例接口。
//
// 接入真实业务时可参考此文件的模式：使用 model/request、model/response 结构、paramsValidator 参数绑定、
// 统一 Response 返回；业务逻辑放入 internal/service，数据访问放入 internal/repository。
//
// @Summary      Ping
// @Description  骨架示例接口：返回 hello 与传入的 name，用于验证统一响应格式
// @x-module-app true
// @Tags         样例接口
// @Produce      json
// @Param        params query request.AppPingRequest false "查询参数"
// @Success      200 {object} Response{data=response.AppPingResponse} "code=0 时返回 data；code=10001 表示参数错误，失败时省略 data"
// @Router       /api/app/ping [get]
func AppPing(ctx *gin.Context) {
	var req request.AppPingRequest
	if err := paramsValidator(ctx, &req); err != nil {
		return
	}
	name := req.Name
	if name == "" {
		name = "world"
	}
	returnSuccessJson(ctx, response.AppPingResponse{Message: "hello " + name})
}
