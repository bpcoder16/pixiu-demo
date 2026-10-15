package handler

import (
	"strings"

	"pixiu-demo/errorcode"
	"pixiu-demo/internal/model/request"
	"pixiu-demo/internal/model/response"
	"pixiu-demo/internal/service/elasticsearch"

	"github.com/bpcoder16/pixiu/logit"
	"github.com/gin-gonic/gin"
)

// AppElasticsearchValidateSynonyms 使用默认 ES 数据面客户端创建并清理临时索引，验证节点词库。
//
// @Summary      验证 ES 同义词词库
// @Description  创建临时索引，以 standard → lowercase → synonym_graph 严格加载 Solr 词库；确认创建后独立限时 5 秒清理。仅证明固定分析链在执行节点可加载，不保证集群分发或业务召回效果；创建或清理失败均返回服务错误，验证及清理状态记录在日志中
// @x-module-app true
// @Tags         Elasticsearch
// @Accept       json
// @Produce      json
// @Param        request body request.AppElasticsearchValidateSynonymsRequest true "synonymsPath 必须是 ES 节点上的词库路径，不是本机文件或下载链接；不能为空白或含空字符，按原值传递"
// @Success      200 {object} Response{data=response.AppElasticsearchValidateSynonymsResponse} "code=0 时验证及清理均成功并返回 data；code=10001 表示参数错误，code=400 表示创建或清理失败，失败时省略 data"
// @Router       /api/app/elasticsearch/synonyms-validate [post]
func AppElasticsearchValidateSynonyms(ctx *gin.Context) {
	var req request.AppElasticsearchValidateSynonymsRequest
	if err := paramsValidator(ctx, &req); err != nil {
		return
	}
	if strings.TrimSpace(req.SynonymsPath) == "" || strings.ContainsRune(req.SynonymsPath, '\x00') {
		returnErrJson(ctx, errorcode.ErrParams, "synonymsPath 不能为空白或含空字符")
		return
	}
	result, err := elasticsearch.NewService().ValidateSynonyms(ctx.Request.Context(), req.SynonymsPath)
	if err != nil {
		// 失败响应省略 data，将可能遗留的索引及清理状态保留在请求日志中供核查。
		logit.Error(ctx.Request.Context(), "验证 ES 同义词失败",
			logit.Str("index", result.Index),
			logit.Bool("valid", result.Valid),
			logit.Str("stage", string(result.Stage)),
			logit.Str("cleanup", string(result.Cleanup)),
			logit.Bool("mayExist", result.MayExist),
			logit.Err(err),
		)
		returnErrJson(ctx, errorcode.ErrServiceException)
		return
	}
	returnSuccessJson(ctx, response.AppElasticsearchValidateSynonymsResponse{
		Index:    result.Index,
		Valid:    result.Valid,
		Stage:    string(result.Stage),
		Cleanup:  string(result.Cleanup),
		MayExist: result.MayExist,
	})
}
