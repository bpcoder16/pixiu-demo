package errorcode

const (
	Success = 0

	ErrServiceException = 400
)

// ErrParams 通用错误码使用 10000–10999；显式赋值，避免插入或合并常量时改变协议码值。
const (
	ErrParams = 10001
)

// CodeMsg 返回业务状态码对应的展示文案；未定义的业务码返回“未知错误”。
func CodeMsg(code int) (msg string) {
	// 所有码的文案集中在同一个 switch，使已登记的重复码值在编译时暴露。
	switch code {
	case Success:
		msg = "成功"
	case ErrServiceException:
		msg = "服务异常"
	case ErrParams:
		msg = "参数错误"
	default:
		msg = "未知错误"
	}
	return msg
}
