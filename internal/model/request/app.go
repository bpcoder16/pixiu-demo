package request

import "mime/multipart"

type AppPingRequest struct {
	// 打招呼名称，不传或为空时使用 world
	Name string `form:"name" binding:"omitempty,max=32" default:"world" example:"world"`
}

type AppOSSUploadRequest struct {
	// Key 是目标 Bucket 内的对象名，按原值传给 OSS。
	Key string `form:"key" binding:"required"`
	// File 是本次上传的唯一文件。
	File *multipart.FileHeader `form:"file" binding:"required"`
}
