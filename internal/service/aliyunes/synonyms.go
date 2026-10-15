package aliyunes

import (
	"context"
	"fmt"

	"pixiu-demo/internal/infra/aliyunes"

	"github.com/bpcoder16/pixiu/infra/aliyun/esx"
)

// ListSynonymsDicts 使用当前操作的 Context 查询完整同义词文件清单，保留元数据和下载链接。
func (s *Service) ListSynonymsDicts(ctx context.Context) (esx.ListSynonymsDictsResult, error) {
	result, err := aliyunes.Client().ListSynonymsDicts(ctx)
	if err != nil {
		return result, fmt.Errorf("list Aliyun ES synonyms dictionaries: %w", err)
	}
	return result, nil
}
