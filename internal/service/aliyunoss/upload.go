package aliyunoss

import (
	"context"
	"fmt"
	"mime/multipart"

	"pixiu-demo/internal/infra/aliyunoss"

	"github.com/bpcoder16/pixiu/infra/aliyun/ossx"
)

// UploadService 将上传文件保存到默认 OSS 实例固定的 Bucket。
type UploadService struct{}

// NewUploadService 创建上传服务，不初始化或关闭共享 OSS 客户端。
func NewUploadService() *UploadService {
	return &UploadService{}
}

// Upload 打开并关闭已校验的文件，原样传递 key，并沿用本次操作的 Context。
// multipart 临时文件由调用方在请求结束时清理。
func (s *UploadService) Upload(ctx context.Context, key string, fileHeader *multipart.FileHeader) (ossx.PutObjectResult, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return ossx.PutObjectResult{}, fmt.Errorf("open OSS upload file: %w", err)
	}
	defer file.Close()

	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	result, err := aliyunoss.Client().PutObject(ctx, ossx.PutObjectRequest{
		Key:         key,
		Body:        file,
		ContentType: contentType,
	})
	if err != nil {
		return result, fmt.Errorf("upload OSS object: %w", err)
	}
	return result, nil
}
