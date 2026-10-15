package route

import (
	"pixiu-demo/internal/handler"

	"github.com/gin-gonic/gin"
)

func registerAppRoutesV2(appGroup *gin.RouterGroup) {
	appGroup.GET("/ping", handler.AppPing)

	elasticsearchGroup := appGroup.Group("/elasticsearch")
	{
		elasticsearchGroup.POST("/synonyms-validate", handler.AppElasticsearchValidateSynonyms)
		elasticsearchGroup.POST("/synonyms-reload", handler.AppElasticsearchReloadSynonyms)
	}

	aliyunGroup := appGroup.Group("/aliyun")
	{
		aliyunGroup.POST("/oss/upload", handler.AppOSSUpload)

		aliyunGroup.GET("/es/synonyms-dicts-list", handler.AppESSynonymsDictsList)
		aliyunGroup.POST("/es/synonyms-dicts-update", handler.AppESSynonymsDictsUpdate)
		aliyunGroup.GET("/es/describe-instance", handler.AppESDescribeInstance)
	}
}
