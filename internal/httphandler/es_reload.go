package httphandler

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"pixiu-demo/errorcode"
	"pixiu-demo/internal/model/request"
	"pixiu-demo/internal/model/response"
	"pixiu-demo/internal/service/elasticsearch"

	"github.com/bpcoder16/pixiu/logit"
	"github.com/gin-gonic/gin"
)

// AppElasticsearchReloadSynonyms 重载默认 ES 实例中单个索引的搜索分析器并清理请求缓存。
//
// @Summary      重载 ES 同义词搜索分析器
// @Description  调用前须更新所有数据节点的词库，过滤器设置 updateable: true 且仅用于搜索分析器，客户端须有目标索引 manage 权限。顺序执行重载与 request cache 清理，共用请求 Context，不重试或回滚；成功仅表示两次请求成功，应检查 reloadDetails 是否包含预期分析器。清缓存失败时重载可能已生效，阶段结果保留在日志中
// @x-module-app true
// @Tags         Elasticsearch
// @Accept       json
// @Produce      json
// @Param        request body request.AppElasticsearchReloadSynonymsRequest true "单个实际索引名；不使用别名、数据流、通配符或逗号列表，不解析目标类型"
// @Success      200 {object} Response{data=response.AppElasticsearchReloadSynonymsResponse} "code=0 时返回重载详情与两阶段计数；code=10001 表示索引名参数错误，code=400 表示重载或清缓存失败，失败时省略 data"
// @Router       /api/app/elasticsearch/synonyms-reload [post]
func AppElasticsearchReloadSynonyms(ctx *gin.Context) {
	var req request.AppElasticsearchReloadSynonymsRequest
	if err := paramsValidator(ctx, &req); err != nil {
		return
	}
	if !validElasticsearchIndex(req.Index) {
		returnErrJson(ctx, errorcode.ErrParams, "index 须为合法的单个具体索引名")
		return
	}
	result, err := elasticsearch.NewService().ReloadSearchAnalyzers(ctx.Request.Context(), req.Index)
	if err != nil {
		// 两阶段不原子；响应省略 data，将已取得的阶段结果留在日志中供核查。
		logit.Error(ctx.Request.Context(), "重载 ES 同义词失败",
			logit.Str("index", req.Index),
			logit.Any("reloadShards", result.Shards),
			logit.Any("reloadDetails", result.ReloadDetails),
			logit.Any("cacheShards", result.CacheShards),
			logit.Err(err),
		)
		returnErrJson(ctx, errorcode.ErrServiceException)
		return
	}
	data := response.AppElasticsearchReloadSynonymsResponse{
		Shards: response.ElasticsearchShardCounts{
			Total:      result.Shards.Total,
			Successful: result.Shards.Successful,
			Failed:     result.Shards.Failed,
		},
		ReloadDetails: make([]response.ElasticsearchReloadDetail, 0, len(result.ReloadDetails)),
		CacheShards: response.ElasticsearchShardCounts{
			Total:      result.CacheShards.Total,
			Successful: result.CacheShards.Successful,
			Failed:     result.CacheShards.Failed,
		},
	}
	for _, detail := range result.ReloadDetails {
		data.ReloadDetails = append(data.ReloadDetails, response.ElasticsearchReloadDetail{
			Index:             detail.Index,
			ReloadedAnalyzers: detail.ReloadedAnalyzers,
			ReloadedNodeIDs:   detail.ReloadedNodeIDs,
		})
	}
	returnSuccessJson(ctx, data)
}

// 与 Pixiu 的具体索引名约束保持一致，将非法选择器作为参数错误拒绝；不查询或解析别名及数据流。
func validElasticsearchIndex(index string) bool {
	if index == "" || len(index) > 255 || !utf8.ValidString(index) || index != strings.ToLower(index) || index == "." || index == ".." {
		return false
	}
	if strings.ContainsAny(index, `\/*?"<>| ,#:`) || strings.ContainsRune("-_+", rune(index[0])) {
		return false
	}
	for _, r := range index {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}
