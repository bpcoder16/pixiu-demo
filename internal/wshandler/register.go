package wshandler

import "github.com/bpcoder16/pixiu/biz/realtime"

// Register 集中登记 app 端的上行消息，由 realtime.NewServer 在启动阶段调用。
func Register(r *realtime.Router) error {
	return r.Handle("client", "setPage", setPage)
}
