package httphandler

import (
	"strings"

	"pixiu-demo/errorcode"
	"pixiu-demo/internal/model/request"
	"pixiu-demo/internal/model/response"
	"pixiu-demo/internal/service/aliyunoss"

	"github.com/gin-gonic/gin"
)

// AppOSSUpload 将单个表单文件上传到默认 OSS 实例固定的 Bucket。
//
// @Summary      上传 OSS 文件
// @Description  上传单个文件；key 指定对象名，同名对象遵循 OSS 覆盖及版本规则
// @x-module-app true
// @Tags         文件上传
// @Accept       mpfd
// @Produce      json
// @Param        file formData file true "上传文件"
// @Param        key formData string true "Bucket 内的对象名，例如 uploads/demo.txt"
// @Success      200 {object} Response{data=response.AppOSSUploadResponse} "code=0 时返回 data；code=10001 表示参数错误，code=400 表示上传失败，失败时省略 data"
// @Router       /api/app/aliyun/oss/upload [post]
func AppOSSUpload(ctx *gin.Context) {
	defer func() {
		if form := ctx.Request.MultipartForm; form != nil {
			_ = form.RemoveAll()
		}
	}()
	// 1 MiB 只控制 multipart 文件的内存阈值，超过后落盘，不限制文件大小。
	if err := ctx.Request.ParseMultipartForm(1 << 20); err != nil {
		returnErrJson(ctx, errorcode.ErrParams, "上传表单无效，请使用 multipart/form-data")
		return
	}

	var req request.AppOSSUploadRequest
	if err := paramsValidator(ctx, &req); err != nil {
		return
	}
	if strings.TrimSpace(req.Key) == "" {
		returnErrJson(ctx, errorcode.ErrParams, "key 不能为空白")
		return
	}
	if len(ctx.Request.MultipartForm.File["file"]) != 1 {
		returnErrJson(ctx, errorcode.ErrParams, "file 只能上传一个文件")
		return
	}
	result, err := aliyunoss.NewUploadService().Upload(ctx.Request.Context(), req.Key, req.File)
	if err != nil {
		// SDK 错误可能携带签名或响应正文，仅向调用方返回统一服务错误。
		returnErrJson(ctx, errorcode.ErrServiceException)
		return
	}
	returnSuccessJson(ctx, response.AppOSSUploadResponse{
		Key:       req.Key,
		ETag:      result.ETag,
		RequestID: result.RequestID,
		VersionID: result.VersionID,
	})
}
