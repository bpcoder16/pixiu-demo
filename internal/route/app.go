package route

import (
	"pixiu-demo/internal/handler"

	"github.com/gin-gonic/gin"
)

func registerAppRoutesV2(appGroup *gin.RouterGroup) {
	appGroup.GET("/ping", handler.AppPing)

	aliyunGroup := appGroup.Group("/aliyun")
	{
		aliyunGroup.POST("/oss/upload", handler.AppOSSUpload)

		aliyunGroup.GET("/es/synonyms-dicts-list", handler.AppESSynonymsDictsList)
		aliyunGroup.GET("/es/describe-instance", handler.AppESDescribeInstance)
	}
}
