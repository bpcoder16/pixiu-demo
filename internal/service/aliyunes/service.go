package aliyunes

// Service 提供默认阿里云 ES 实例的词典管理及实例查询。
type Service struct{}

// NewService 创建服务，共享客户端由启动阶段统一管理。
func NewService() *Service {
	return &Service{}
}
