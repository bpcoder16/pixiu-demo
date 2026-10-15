package elasticsearch

import (
	"pixiu-demo/internal/infra/elasticsearch"

	"github.com/bpcoder16/pixiu/infra/elasticsearchx"
)

// Service 使用创建时选定的 ES 数据面客户端提供同义词验证。
type Service struct {
	client *elasticsearchx.Client
}

// NewService 创建服务，共享客户端由启动阶段统一管理。
func NewService() *Service {
	return &Service{
		client: elasticsearch.Client(),
	}
}
