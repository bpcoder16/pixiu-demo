package handler

import (
	"pixiu-demo/internal/model/response"

	"github.com/gin-gonic/gin"
)

// AdminPing 仅演示管理端文档分组，不访问管理数据。
//
// @Summary      Ping
// @Description  公开的管理端文档样例，返回固定问候，不读写业务数据
// @x-module-admin true
// @Tags         样例接口
// @Produce      json
// @Success      200 {object} Response{data=response.AdminPingResponse} "code=0 时返回固定的 data"
// @Router       /api/admin/ping [get]
func AdminPing(ctx *gin.Context) {
	returnSuccessJson(ctx, response.AdminPingResponse{Message: "hello admin"})
}
