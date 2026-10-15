package elasticsearch

import (
	"context"

	"github.com/bpcoder16/pixiu/biz/elasticsearchservice"
)

// ValidateSynonyms 沿用当前请求的 Context，并使用库默认的独立 5 秒清理期限。
func (s *Service) ValidateSynonyms(ctx context.Context, synonymsPath string) (elasticsearchservice.ValidationResult, error) {
	service, err := elasticsearchservice.New(s.client)
	if err != nil {
		return elasticsearchservice.ValidationResult{
			Stage:   elasticsearchservice.StagePrepare,
			Cleanup: elasticsearchservice.CleanupNotAttempted,
		}, err
	}
	return service.ValidateSynonyms(ctx, elasticsearchservice.ValidationOptions{
		SynonymsPath: synonymsPath,
	})
}
