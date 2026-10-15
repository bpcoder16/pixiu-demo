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

type AppESSynonymsDictsListResponse struct {
	RequestID string               `json:"requestId" example:"es-request-id"`
	Dicts     []ESSynonymsDictInfo `json:"dicts"`
}

type ESSynonymsDictInfo struct {
	Name       string `json:"name" example:"synonyms.txt"`
	FileSize   int64  `json:"fileSize" example:"1024"`
	SourceType string `json:"sourceType" example:"OSS"`
	Type       string `json:"type" example:"SYNONYMS"`
	// DownloadURL 可能含短期签名，应避免将响应正文写入日志。
	DownloadURL string `json:"downloadUrl" example:"https://example.com/synonyms.txt"`
}
