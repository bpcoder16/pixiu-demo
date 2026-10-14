package route

import (
	"pixiu-demo/internal/handler"

	"github.com/gin-gonic/gin"
)

func registerAppRoutesV2(appGroup *gin.RouterGroup) {
	appGroup.GET("/ping", handler.AppPing)
}
