package response

type AppPingResponse struct {
	Message string `json:"message" example:"hello world"`
}

type AppOSSUploadResponse struct {
	Key       string `json:"key" example:"uploads/demo.txt"`
	ETag      string `json:"etag" example:"opaque-etag"`
	RequestID string `json:"requestId" example:"oss-request-id"`
	// 未开启 Bucket 版本控制时，VersionID 可为空。
	VersionID string `json:"versionId" example:"oss-version-id"`
}
