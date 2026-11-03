package wshandler

import (
	"context"
	"net/http"
	"time"

	"pixiu-demo/internal/wspb"

	"github.com/bpcoder16/pixiu/biz/realtime"
	"github.com/bpcoder16/pixiu/biz/realtime/protocol"
	"github.com/bpcoder16/pixiu/logit"
)

// TerminalInfo 放在 ClientInfo.Value 中，连接建立后保持不变。
type TerminalInfo struct {
	Platform   string
	AppVersion string
}

// NewServer 仅构造服务，由 bootstrap 统一运行及关闭。
func NewServer() (*realtime.Server, error) {
	return realtime.NewServer(realtime.Config{
		// Transport 可省略，以下为默认值；需要定制时取消注释并调整。
		// Transport: websocketx.Config{
		// 	MaxConnections: 10_000,           // 最大连接数，包含握手和收尾中的连接。
		// 	ReadLimit:      64 << 10,         // 单条入站消息上限 64 KiB，包含信封。
		// 	MaxSendBytes:   64 << 10,         // 单条出站消息上限 64 KiB，包含信封。
		// 	MessageTimeout: 30 * time.Second, // 初始化及消息处理期限，包含消息排队时间。
		// 	OnCloseTimeout: 5 * time.Second,  // 关闭清理 context 的期限，回调须响应取消。
		// },
		Authenticate: demoAuthenticate,
		NewState: func(realtime.ClientInfo) any {
			return &ClientState{}
		},
		OnOpen:           onOpen,
		RegisterHandlers: Register,
		OnClose:          onClose,
	})
}

// demoAuthenticate 仅演示身份注入，所有连接使用相同的模拟身份，没有验证登录凭据。
// 实际业务应在这里验证请求凭据，再返回可信的用户、设备及端信息；不要保存请求或 token。
func demoAuthenticate(context.Context, *http.Request) (realtime.ClientInfo, error) {
	return realtime.ClientInfo{
		UserID:   "demo-user",
		DeviceID: "demo-device",
		Value: TerminalInfo{
			Platform:   "web",
			AppVersion: "1.0.0",
		},
	}, nil
}

func onOpen(ctx context.Context, session *realtime.Session) error {
	// 建连后主动发送当前连接事件，展示 Send；不涉及消息总线或按用户寻址。
	return session.Send(ctx, realtime.Outgoing{
		Kind: protocol.Kind_EVENT,
		Type: "session.connected",
		Data: &wspb.Connected{
			ConnectionId: session.ID(),
		},
		ExpiresAt: time.Now().Add(10 * time.Second),
	})
}

func onClose(ctx context.Context, session *realtime.Session) {
	// 清理 I/O 应使用这里的独立 ctx：最多 5 秒，且响应 Server.Cancel。
	// Session.Context 已因断连取消；本例只记录收尾，不额外占用外部资源。
	info := session.Info()
	terminal := info.Value.(TerminalInfo)
	logit.Info(ctx, "WebSocket 连接已收尾",
		logit.Str("connectionId", session.ID()),
		logit.Str("userId", info.UserID),
		logit.Str("platform", terminal.Platform),
	)
}
