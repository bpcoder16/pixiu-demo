package route

import (
	"pixiu-demo/internal/httphandler"

	"github.com/gin-gonic/gin"
)

func registerAdminRoutesV2(adminGroup *gin.RouterGroup) {
	// 仅公开无数据访问的文档样例。
	adminGroup.GET("/ping", httphandler.AdminPing)
}
