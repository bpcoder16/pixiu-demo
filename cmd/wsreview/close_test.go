package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pixiu-demo/internal/wspb"

	"github.com/bpcoder16/pixiu/biz/realtime"
	"github.com/bpcoder16/pixiu/infra/websocketx"
	"github.com/gorilla/websocket"
)

func TestReviewClientNormalClose(t *testing.T) {
	for _, encoding := range []string{"json", "pb"} {
		for _, page := range []string{"home", ""} {
			t.Run(encoding+"/page="+page, func(t *testing.T) {
				closed := make(chan websocketx.Event, 1)
				server, err := realtime.NewServer(realtime.Config{
					Transport: websocketx.Config{
						Observe: func(_ context.Context, event websocketx.Event) {
							if event.Kind == websocketx.EventClosed {
								closed <- event
							}
						},
					},
					Authenticate: func(context.Context, *http.Request) (realtime.ClientInfo, error) {
						return realtime.ClientInfo{}, nil
					},
					OnMessage: func(ctx context.Context, session *realtime.Session, message *realtime.Message) error {
						var request wspb.SetPageRequest
						if err := message.Decode(&request); err != nil {
							return err
						}
						if request.GetPage() == "" {
							return session.ReplyError(ctx, message, "INVALID_ARGUMENT", "page 不能为空")
						}
						return session.Reply(ctx, message, &wspb.StateReply{Page: request.GetPage()})
					},
				})
				if err != nil {
					t.Fatal(err)
				}
				runDone := make(chan error, 1)
				go func() { runDone <- server.Run() }()
				t.Cleanup(func() {
					_ = server.Stop()
					ctx, cancel := context.WithTimeout(context.Background(), time.Second)
					defer cancel()
					if err := server.Wait(ctx); err != nil {
						server.Cancel()
						t.Error(err)
					}
					select {
					case <-runDone:
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
				defer h.Close()
				err = run("ws"+strings.TrimPrefix(h.URL, "http"), encoding, page, io.Discard)
				if page == "" {
					if err == nil || !strings.Contains(err.Error(), "INVALID_ARGUMENT") {
						t.Fatalf("关闭握手覆盖了业务错误: %v", err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
				select {
				case event := <-closed:
					if event.Code != websocket.CloseNormalClosure || event.Err != nil {
						t.Fatalf("完成业务响应后应正常关闭: code=%d err=%v", event.Code, event.Err)
					}
				case <-time.After(time.Second):
					t.Fatal("连接未收尾")
				}
			})
		}
	}
}

// 对端接收了 Close 但不回复，客户端必须限时退出；业务错误仍应保留。
func TestReviewClientCloseTimeout(t *testing.T) {
	for _, businessError := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "business-error"}[businessError], func(t *testing.T) {
			release := make(chan struct{})
			receivedClose := make(chan int, 1)
			h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upgrader := websocket.Upgrader{Subprotocols: []string{realtime.SubprotocolJSON}}
				conn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()
				conn.SetCloseHandler(func(code int, _ string) error {
					receivedClose <- code
					return nil // 故意不发送 Close 回复。
				})
				_ = conn.SetReadDeadline(time.Now().Add(4 * time.Second))
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
				response := `{"kind":"response","requestId":"r1","scene":"client","action":"setPage","data":{"page":"home"}}`
				if businessError {
					response = `{"kind":"response","requestId":"r1","scene":"client","action":"setPage","error":{"code":"INVALID_ARGUMENT","message":"page 不能为空"}}`
				}
				if err := conn.WriteMessage(websocket.TextMessage, []byte(response)); err != nil {
					return
				}
				_, _, _ = conn.ReadMessage()
				<-release
			}))
			defer h.Close()
			defer close(release)
			result := make(chan error, 1)
			go func() {
				result <- run("ws"+strings.TrimPrefix(h.URL, "http"), "json", "home", io.Discard)
			}()
			select {
			case err := <-result:
				var networkError net.Error
				if !errors.As(err, &networkError) || !networkError.Timeout() {
					t.Fatalf("未报告关闭握手超时: %v", err)
				}
				if businessError && !strings.Contains(err.Error(), "INVALID_ARGUMENT") {
					t.Fatalf("关闭超时覆盖了业务错误: %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("关闭握手没有限时退出")
			}
			select {
			case code := <-receivedClose:
				if code != websocket.CloseNormalClosure {
					t.Fatalf("发出的关闭码 = %d", code)
				}
			default:
				t.Fatal("客户端未发送 Close")
			}
		})
	}
}
