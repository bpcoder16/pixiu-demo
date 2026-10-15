package handler

import (
	"errors"
	"regexp"
	"strings"

	"pixiu-demo/errorcode"
	"pixiu-demo/internal/model/request"
	"pixiu-demo/internal/model/response"
	"pixiu-demo/internal/service/aliyunes"

	"github.com/bpcoder16/pixiu/infra/aliyun/esx"
	"github.com/gin-gonic/gin"
)

var esSynonymsDictNamePattern = regexp.MustCompile(`^[A-Za-z0-9_]+\.txt$`)

// AppESSynonymsDictsUpdate 向默认阿里云 ES 实例提交完整同义词文件清单。
//
// @Summary      更新 ES 同义词词典
// @Description  提交非空完整目标清单，遗漏已有文件会删除；成功仅表示管理 API 成功，不保证分发或索引已加载
// @x-module-app true
// @Tags         阿里云 ES
// @Accept       json
// @Produce      json
// @Param        request body request.AppESSynonymsDictsUpdateRequest true "完整目标清单；ORIGIN 保留已有文件，OSS 通过 bucket、key 导入；同一次更新重试须复用 clientToken"
// @Success      200 {object} Response{data=response.AppESSynonymsDictsUpdateResponse} "code=0 时返回 data；code=10001 表示参数错误，code=400 表示更新失败，失败时省略 data"
// @Router       /api/app/aliyun/es/synonyms-dicts-update [post]
func AppESSynonymsDictsUpdate(ctx *gin.Context) {
	var req request.AppESSynonymsDictsUpdateRequest
	if err := paramsValidator(ctx, &req); err != nil {
		return
	}
	update, err := toESUpdateSynonymsDictsRequest(req)
	if err != nil {
		returnErrJson(ctx, errorcode.ErrParams, err.Error())
		return
	}
	result, err := aliyunes.NewService().UpdateSynonymsDicts(ctx.Request.Context(), update)
	if err != nil {
		// SDK 错误可能携带敏感详情，仅向调用方返回统一服务错误。
		returnErrJson(ctx, errorcode.ErrServiceException)
		return
	}
	data := response.AppESSynonymsDictsUpdateResponse{
		RequestID: result.RequestID,
		Dicts:     make([]response.ESSynonymsDictUpdateInfo, 0, len(result.Dicts)),
	}
	for _, dict := range result.Dicts {
		data.Dicts = append(data.Dicts, response.ESSynonymsDictUpdateInfo{
			Name:       dict.Name,
			FileSize:   dict.FileSize,
			SourceType: dict.SourceType,
			Type:       dict.Type,
		})
	}
	returnSuccessJson(ctx, data)
}

// toESUpdateSynonymsDictsRequest 补充绑定标签无法表达的校验，不读取或合并管理端清单。
func toESUpdateSynonymsDictsRequest(req request.AppESSynonymsDictsUpdateRequest) (esx.UpdateSynonymsDictsRequest, error) {
	update := esx.UpdateSynonymsDictsRequest{
		ClientToken: req.ClientToken,
		Dicts:       make([]esx.SynonymsDict, 0, len(req.Dicts)),
	}
	if strings.TrimSpace(req.ClientToken) == "" {
		return update, errors.New("clientToken 不能为空白")
	}
	seen := make(map[string]bool, len(req.Dicts))
	for _, dict := range req.Dicts {
		if !esSynonymsDictNamePattern.MatchString(dict.Name) {
			return update, errors.New("词典文件名须使用英文字母、数字或下划线，并以 .txt 结尾")
		}
		if seen[dict.Name] {
			return update, errors.New("词典文件名不能重复")
		}
		seen[dict.Name] = true
		switch dict.SourceType {
		case "ORIGIN":
			update.Dicts = append(update.Dicts, esx.KeepSynonymsDict(dict.Name))
		case "OSS":
			if strings.TrimSpace(dict.Bucket) == "" || dict.Key == "" {
				return update, errors.New("OSS 来源须指定非空白 bucket 和非空 key")
			}
			update.Dicts = append(update.Dicts, esx.SynonymsDictFromOSS(dict.Name, dict.Bucket, dict.Key))
		default:
			return update, errors.New("sourceType 须为 ORIGIN 或 OSS")
		}
	}
	return update, nil
}
