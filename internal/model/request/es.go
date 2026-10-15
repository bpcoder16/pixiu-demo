package request

type AppElasticsearchValidateSynonymsRequest struct {
	// SynonymsPath 是 ES 节点上的 Solr 格式词库路径，不是本机文件或下载链接。
	SynonymsPath string `json:"synonymsPath" binding:"required" example:"analysis/synonyms.txt"`
}

type AppESSynonymsDictsUpdateRequest struct {
	// ClientToken 同一次更新的重试须复用，不同更新须更换，最多 64 个 ASCII 字符。
	ClientToken string `json:"clientToken" binding:"required,max=64,ascii" example:"release-1"`
	// Dicts 是非空的完整目标清单，遗漏已有文件会删除它。
	Dicts []ESSynonymsDictInput `json:"dicts" binding:"required,min=1,dive"`
}

type ESSynonymsDictInput struct {
	// Name 是 ES 词典文件名，限英文字母、数字、下划线及 .txt 后缀，总长不超过 30。
	Name string `json:"name" binding:"required,max=30" example:"synonyms.txt"`
	// SourceType 为 ORIGIN 时保留已有文件，为 OSS 时从 Bucket、Key 导入。
	SourceType string `json:"sourceType" binding:"required,oneof=ORIGIN OSS" enums:"ORIGIN,OSS" example:"OSS"`
	// Bucket 和 Key 仅用于 OSS 来源，按原值传给管理 API。
	Bucket string `json:"bucket" example:"dict-bucket"`
	Key    string `json:"key" example:"release/synonyms.txt"`
}
