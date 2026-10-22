package route

import (
	"pixiu-demo/internal/httphandler"

	"github.com/gin-gonic/gin"
)

func registerAppRoutesV2(appGroup *gin.RouterGroup) {
	appGroup.GET("/ping", httphandler.AppPing)
	appGroup.POST("/nats/publish", httphandler.AppNATSPublish)

	elasticsearchGroup := appGroup.Group("/elasticsearch")
	{
		elasticsearchGroup.POST("/synonyms-validate", httphandler.AppElasticsearchValidateSynonyms)
		elasticsearchGroup.POST("/synonyms-reload", httphandler.AppElasticsearchReloadSynonyms)
	}

	aliyunGroup := appGroup.Group("/aliyun")
	{
		aliyunGroup.POST("/oss/upload", httphandler.AppOSSUpload)

		aliyunGroup.GET("/es/synonyms-dicts-list", httphandler.AppESSynonymsDictsList)
		aliyunGroup.POST("/es/synonyms-dicts-update", httphandler.AppESSynonymsDictsUpdate)
		aliyunGroup.GET("/es/describe-instance", httphandler.AppESDescribeInstance)
	}
}
