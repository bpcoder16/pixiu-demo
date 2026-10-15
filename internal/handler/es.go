package handler

import (
	"pixiu-demo/errorcode"
	"pixiu-demo/internal/model/response"
	"pixiu-demo/internal/service/aliyunes"

	"github.com/gin-gonic/gin"
)

// AppESSynonymsDictsList 查询默认阿里云 ES 实例的全部同义词文件。
//
// @Summary      查询 ES 同义词词典
// @Description  返回默认实例的完整同义词文件清单及下载链接；管理端清单不表示节点已加载词典
// @x-module-app true
// @Tags         阿里云 ES
// @Produce      json
// @Success      200 {object} Response{data=response.AppESSynonymsDictsListResponse} "code=0 时返回 data；code=400 表示查询失败，失败时省略 data"
// @Router       /api/app/aliyun/es/synonyms-dicts-list [get]
func AppESSynonymsDictsList(ctx *gin.Context) {
	result, err := aliyunes.NewSynonymsDictService().List(ctx.Request.Context())
	if err != nil {
		// SDK 错误可能携带敏感详情，仅向调用方返回统一服务错误。
		returnErrJson(ctx, errorcode.ErrServiceException)
		return
	}
	data := response.AppESSynonymsDictsListResponse{
		RequestID: result.RequestID,
		Dicts:     make([]response.ESSynonymsDictInfo, 0, len(result.Dicts)),
	}
	for _, dict := range result.Dicts {
		data.Dicts = append(data.Dicts, response.ESSynonymsDictInfo{
			Name:        dict.Name,
			FileSize:    dict.FileSize,
			SourceType:  dict.SourceType,
			Type:        dict.Type,
			DownloadURL: dict.DownloadURL,
		})
	}
	returnSuccessJson(ctx, data)
}
