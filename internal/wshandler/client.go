package wshandler

import (
	"context"
	"sync"

	"pixiu-demo/internal/wspb"

	"github.com/bpcoder16/pixiu/biz/realtime"
)

// AppClientState 由 App 端的 NewState 为每条连接分别创建；重连不会继承旧状态。
// 同连接消息串行处理；锁让业务在其他 goroutine 访问状态时仍有明确的同步边界。
type AppClientState struct {
	mu       sync.Mutex
	page     string
	revision uint64
}

type AppStateSnapshot struct {
	Page     string
	Revision uint64
}

func (s *AppClientState) SetPage(page string) AppStateSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.page = page
	s.revision++
	return AppStateSnapshot{
		Page:     s.page,
		Revision: s.revision,
	}
}

func setPage(ctx context.Context, session *realtime.Session, message *realtime.Message) error {
	var request wspb.SetPageRequest
	if err := message.Decode(&request); err != nil {
		return session.ReplyError(ctx, message, "request.error", "INVALID_ARGUMENT", "请求正文格式错误")
	}
	if request.GetPage() == "" || len(request.GetPage()) > 128 {
		return session.ReplyError(ctx, message, "request.error", "INVALID_ARGUMENT", "page 长度必须为 1–128 字节")
	}
	state := session.State().(*AppClientState).SetPage(request.GetPage())
	info := session.Info()
	return session.Reply(ctx, message, "client.state", &wspb.StateReply{
		UserId:   info.UserID,
		DeviceId: info.DeviceID,
		Page:     state.Page,
		Revision: state.Revision,
	})
}
