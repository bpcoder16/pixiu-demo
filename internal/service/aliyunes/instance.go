package aliyunes

import (
	"context"
	"fmt"

	"pixiu-demo/internal/infra/aliyunes"

	"github.com/bpcoder16/pixiu/infra/aliyun/esx"
)

// DescribeInstance 使用当前操作的 Context 查询实例详情，保留服务端原始元数据。
func (s *Service) DescribeInstance(ctx context.Context) (esx.DescribeInstanceResult, error) {
	result, err := aliyunes.Client().DescribeInstance(ctx)
	if err != nil {
		return result, fmt.Errorf("describe Aliyun ES instance: %w", err)
	}
	return result, nil
}
