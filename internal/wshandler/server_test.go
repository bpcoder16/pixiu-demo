package wshandler

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pixiu-demo/internal/wspb"

	"github.com/bpcoder16/pixiu/biz/realtime"
	"github.com/bpcoder16/pixiu/biz/realtime/protocol"
	"github.com/bpcoder16/pixiu/infra/websocketx"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

// 复用实际业务 Handler，验证成功回复和业务错误回复遭拒时，客户端都能感知关闭。
func TestReplyFailureClosesConnection(t *testing.T) {
	for _, subprotocol := range []string{realtime.SubprotocolJSON, realtime.SubprotocolProtobuf} {
		for _, action := range []string{"setPage", "unknown"} {
			for _, limit := range []string{"budget", "queue", "size"} {
				t.Run(subprotocol+"/"+action+"/"+limit, func(t *testing.T) {
					transport := websocketx.Config{}
					wantCode := websocket.CloseTryAgainLater
					wantErr := websocketx.ErrBudgetExceeded
					switch limit {
					case "budget":
						budget, err := websocketx.NewBudget(websocketx.BudgetConfig{SendBytes: 1})
						if err != nil {
							t.Fatal(err)
						}
						transport.Budget = budget
					case "queue":
						transport.SendQueueBytes = 1
						wantErr = websocketx.ErrQueueFull
					case "size":
						transport.MaxSendBytes = 1
						wantCode = websocket.CloseInternalServerErr
						wantErr = websocketx.ErrMessageTooLarge
					}
					handled := make(chan error, 1)
					closed := make(chan StateSnapshot, 1)
					server, err := realtime.NewServer(realtime.Config{
						Transport:    transport,
						Authenticate: demoAuthenticate,
						NewState: func(realtime.ClientInfo) any {
							return &ClientState{}
						},
						OnMessage: func(ctx context.Context, session *realtime.Session, message *realtime.Message) error {
							err := onMessage(ctx, session, message)
							handled <- err
							return err
						},
						OnClose: func(_ context.Context, session *realtime.Session) {
							state := session.State().(*ClientState)
							state.mu.Lock()
							defer state.mu.Unlock()
							closed <- StateSnapshot{
								Page:     state.page,
								Revision: state.revision,
							}
						},
					})
					if err != nil {
						t.Fatal(err)
					}
					run := make(chan error, 1)
					go func() { run <- server.Run() }()
					t.Cleanup(func() {
						server.Cancel()
						ctx, cancel := context.WithTimeout(context.Background(), time.Second)
						defer cancel()
						if err := server.Wait(ctx); errors.Is(err, context.DeadlineExceeded) {
							t.Error(err)
						}
						select {
						case <-run:
						case <-ctx.Done():
							t.Error("服务未退出")
						}
					})
					deadline := time.Now().Add(time.Second)
					for !server.Ready() {
						if time.Now().After(deadline) {
							t.Fatal("服务未就绪")
						}
						time.Sleep(time.Millisecond)
					}
					h := httptest.NewServer(server)
					t.Cleanup(h.Close)
					dialer := websocket.Dialer{
						Subprotocols:     []string{subprotocol},
						HandshakeTimeout: time.Second,
					}
					client, _, err := dialer.Dial("ws"+strings.TrimPrefix(h.URL, "http"), nil)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = client.Close() })
					frameType := websocket.TextMessage
					frame := []byte(`{"kind":"request","requestId":"r1","scene":"client","action":"` + action + `","data":{"page":"home"}}`)
					if subprotocol == realtime.SubprotocolProtobuf {
						frameType = websocket.BinaryMessage
						body, err := proto.Marshal(&wspb.SetPageRequest{Page: "home"})
						if err != nil {
							t.Fatal(err)
						}
						frame, err = proto.Marshal(&protocol.Envelope{
							Kind:      protocol.Kind_REQUEST,
							RequestId: "r1",
							Scene:     "client",
							Action:    action,
							Body:      &protocol.Envelope_Data{Data: body},
						})
						if err != nil {
							t.Fatal(err)
						}
					}
					if err := client.WriteMessage(frameType, frame); err != nil {
						t.Fatal(err)
					}
					select {
					case err := <-handled:
						if !errors.Is(err, wantErr) {
							t.Fatalf("发送错误丢失: %v，期望 %v", err, wantErr)
						}
					case <-time.After(time.Second):
						t.Fatal("请求未处理")
					}
					_ = client.SetReadDeadline(time.Now().Add(time.Second))
					if _, _, err := client.ReadMessage(); !websocket.IsCloseError(err, wantCode) {
						t.Fatalf("回复失败应关闭 %d，实际为 %v", wantCode, err)
					}
					select {
					case state := <-closed:
						want := StateSnapshot{}
						if action == "setPage" {
							// 发送失败不回滚已完成的业务，也不能自动重放请求。
							want = StateSnapshot{Page: "home", Revision: 1}
						}
						if state != want {
							t.Fatalf("关闭时业务状态 = %+v，期望 %+v", state, want)
						}
					case <-time.After(time.Second):
						t.Fatal("连接未完成业务收尾")
					}
				})
			}
		}
	}
}
