package aliyunes

import (
	"context"
	"fmt"

	"pixiu-demo/internal/infra/aliyunes"

	"github.com/bpcoder16/pixiu/infra/aliyun/esx"
)

// SynonymsDictService 查询默认阿里云 ES 实例的同义词词典。
type SynonymsDictService struct{}

// NewSynonymsDictService 创建词典服务，共享客户端由启动阶段统一管理。
func NewSynonymsDictService() *SynonymsDictService {
	return &SynonymsDictService{}
}

// List 使用当前操作的 Context 查询完整同义词文件清单，保留元数据和下载链接。
func (s *SynonymsDictService) List(ctx context.Context) (esx.ListSynonymsDictsResult, error) {
	result, err := aliyunes.Client().ListSynonymsDicts(ctx)
	if err != nil {
		return result, fmt.Errorf("list Aliyun ES synonyms dictionaries: %w", err)
	}
	return result, nil
}
