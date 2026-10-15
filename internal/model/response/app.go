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

type AppElasticsearchValidateSynonymsResponse struct {
	Index string `json:"index" example:"pixiu-synonym-validation-0123456789abcdef0123456789abcdef"`
	// Valid 仅表示固定分析链在执行节点可加载词库，不保证集群分发或业务召回效果。
	Valid   bool   `json:"valid" example:"true"`
	Stage   string `json:"stage" enums:"prepare,create,complete" example:"complete"`
	Cleanup string `json:"cleanup" enums:"not_attempted,succeeded,failed" example:"succeeded"`
	// MayExist 表示本次调用可能遗留索引，不是存在性查询。
	MayExist bool `json:"mayExist" example:"false"`
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

type AppESDescribeInstanceResponse struct {
	RequestID  string `json:"requestId" example:"es-request-id"`
	InstanceID string `json:"instanceId" example:"es-instance-id"`
	// Status 保留管理 API 的原始状态，active 不代表词典已分发或加载。
	Status  string `json:"status" example:"active"`
	Version string `json:"version" example:"8.17"`
}

type AppESSynonymsDictsUpdateResponse struct {
	RequestID string                     `json:"requestId" example:"es-request-id"`
	Dicts     []ESSynonymsDictUpdateInfo `json:"dicts"`
}

// ESSynonymsDictUpdateInfo 是管理 API 返回的更新元数据，不表示词典已加载。
type ESSynonymsDictUpdateInfo struct {
	Name       string `json:"name" example:"synonyms.txt"`
	FileSize   int64  `json:"fileSize" example:"1024"`
	SourceType string `json:"sourceType" example:"OSS"`
	Type       string `json:"type" example:"SYNONYMS"`
}
