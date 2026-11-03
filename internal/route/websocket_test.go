package route_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pixiu-demo/internal/wshandler"
	"pixiu-demo/internal/wspb"

	"github.com/bpcoder16/pixiu/biz/realtime"
	"github.com/bpcoder16/pixiu/biz/realtime/protocol"
	"github.com/bpcoder16/pixiu/infra/ginx"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// 独立 HTTP 入口验证两种编码共用业务定义、错误不修改状态、连接间状态独立；bootstrap 测试覆盖实际端口装配。
func TestWebSocketRoundTrip(t *testing.T) {
	server, err := wshandler.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Run() }()
	t.Cleanup(func() {
		if err := server.Stop(); err != nil {
			t.Error(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Wait(ctx); err != nil {
			server.Cancel()
			t.Error(err)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-ctx.Done():
			t.Error("WebSocket Run 未退出")
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for !server.Ready() {
		if time.Now().After(deadline) {
			t.Fatal("WebSocket 未就绪")
		}
		time.Sleep(time.Millisecond)
	}
	httpServer := httptest.NewServer(server)
	t.Cleanup(httpServer.Close)
	address := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws/app"

	for _, subprotocol := range []string{realtime.SubprotocolJSON, realtime.SubprotocolProtobuf} {
		t.Run(subprotocol, func(t *testing.T) {
			first := dialDemoWS(t, address, subprotocol)
			second := dialDemoWS(t, address, subprotocol)
			var connected wspb.Connected
			event := first.read(&connected)
			if event.kind != protocol.Kind_EVENT || event.messageType != "session.connected" || connected.GetConnectionId() == "" {
				t.Fatalf("错误的连接事件: %+v, %v", event, &connected)
			}
			firstID := connected.GetConnectionId()
			second.read(&connected)
			if connected.GetConnectionId() == firstID {
				t.Fatal("连接 ID 不应复用")
			}

			assertPage := func(client *demoWSClient, page string, revision uint64) {
				t.Helper()
				client.send("client", "setPage", &wspb.SetPageRequest{Page: page})
				var reply wspb.StateReply
				message := client.read(&reply)
				if message.kind != protocol.Kind_RESPONSE || message.requestID != "r1" || message.messageType != "client.state" || message.errorCode != "" {
					t.Fatalf("错误的响应信封: %+v", message)
				}
				if reply.GetUserId() != "demo-user" || reply.GetDeviceId() != "demo-device" || reply.GetPage() != page || reply.GetRevision() != revision {
					t.Fatalf("错误的会话状态: %v", &reply)
				}
			}
			assertPage(first, "home", 1)
			for _, tc := range []struct {
				action string
				page   string
				code   string
			}{
				{"unknown", "ignored", "NOT_FOUND"},
				{"setPage", "", "INVALID_ARGUMENT"},
				{"setPage", strings.Repeat("x", 129), "INVALID_ARGUMENT"},
			} {
				first.send("client", tc.action, &wspb.SetPageRequest{Page: tc.page})
				message := first.read(nil)
				if message.errorCode != tc.code || message.kind != protocol.Kind_RESPONSE || message.requestID != "r1" || message.messageType != "request.error" {
					t.Fatalf("业务错误回复: %+v", message)
				}
			}
			// 信封合法、业务正文不符合 .proto，仍应显式回复并保留连接。
			badBody := []byte(`[]`)
			if subprotocol == realtime.SubprotocolProtobuf {
				badBody = []byte{0xff}
			}
			first.sendBody("client", "setPage", badBody)
			if message := first.read(nil); message.errorCode != "INVALID_ARGUMENT" || message.messageType != "request.error" {
				t.Fatalf("业务解码失败未回复: %+v", message)
			}
			assertPage(first, "settings", 2)
			assertPage(second, "profile", 1)
		})
	}
}

type demoWSClient struct {
	t      *testing.T
	conn   *websocket.Conn
	binary bool
}

type demoWSMessage struct {
	kind        protocol.Kind
	requestID   string
	messageType string
	errorCode   string
}

func dialDemoWS(t *testing.T, address, subprotocol string) *demoWSClient {
	t.Helper()
	dialer := websocket.Dialer{
		Subprotocols:     []string{subprotocol},
		HandshakeTimeout: 5 * time.Second,
	}
	conn, response, err := dialer.Dial(address, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if response.Header.Get(ginx.TraceHeader) != "" {
		t.Fatal("WebSocket 握手不应经过 Gin 请求作用域")
	}
	if conn.Subprotocol() != subprotocol {
		t.Fatalf("协商协议: %s", conn.Subprotocol())
	}
	conn.SetReadLimit(64 << 10)
	return &demoWSClient{t: t, conn: conn, binary: subprotocol == realtime.SubprotocolProtobuf}
}

func (c *demoWSClient) send(scene, action string, message proto.Message) {
	c.t.Helper()
	var body []byte
	var err error
	if c.binary {
		body, err = proto.Marshal(message)
	} else {
		body, err = protojson.Marshal(message)
	}
	if err != nil {
		c.t.Fatal(err)
	}
	c.sendBody(scene, action, body)
}

func (c *demoWSClient) sendBody(scene, action string, body []byte) {
	c.t.Helper()
	frameType := websocket.TextMessage
	var frame []byte
	var err error
	if c.binary {
		frameType = websocket.BinaryMessage
		frame, err = proto.Marshal(&protocol.Envelope{
			Kind:      protocol.Kind_REQUEST,
			RequestId: "r1",
			Scene:     scene,
			Action:    action,
			Body:      &protocol.Envelope_Data{Data: body},
		})
	} else {
		frame, err = json.Marshal(map[string]any{
			"kind":      protocol.Kind_REQUEST,
			"requestId": "r1",
			"scene":     scene,
			"action":    action,
			"data":      json.RawMessage(body),
		})
	}
	if err != nil {
		c.t.Fatal(err)
	}
	if err := c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		c.t.Fatal(err)
	}
	if err := c.conn.WriteMessage(frameType, frame); err != nil {
		c.t.Fatal(err)
	}
}

func (c *demoWSClient) read(dst proto.Message) demoWSMessage {
	c.t.Helper()
	if err := c.conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		c.t.Fatal(err)
	}
	frameType, frame, err := c.conn.ReadMessage()
	if err != nil {
		c.t.Fatal(err)
	}
	var result demoWSMessage
	var body []byte
	if c.binary {
		if frameType != websocket.BinaryMessage {
			c.t.Fatal("PB 必须使用 Binary 帧")
		}
		var envelope protocol.Envelope
		if err := proto.Unmarshal(frame, &envelope); err != nil {
			c.t.Fatal(err)
		}
		if envelope.GetScene() != "" || envelope.GetAction() != "" {
			c.t.Fatal("下行消息不应携带路由字段")
		}
		result = demoWSMessage{
			kind:        envelope.GetKind(),
			requestID:   envelope.GetRequestId(),
			messageType: envelope.GetType(),
			errorCode:   envelope.GetError().GetCode(),
		}
		body = envelope.GetData()
		if dst != nil {
			err = proto.Unmarshal(body, dst)
		}
	} else {
		if frameType != websocket.TextMessage {
			c.t.Fatal("JSON 必须使用 Text 帧")
		}
		var envelope struct {
			Kind      protocol.Kind   `json:"kind"`
			RequestID string          `json:"requestId"`
			Type      string          `json:"type"`
			Scene     json.RawMessage `json:"scene"`
			Action    json.RawMessage `json:"action"`
			Data      json.RawMessage `json:"data"`
			Error     *struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(frame, &envelope); err != nil {
			c.t.Fatal(err)
		}
		if envelope.Scene != nil || envelope.Action != nil {
			c.t.Fatal("下行消息不应携带路由字段")
		}
		result = demoWSMessage{
			kind:        envelope.Kind,
			requestID:   envelope.RequestID,
			messageType: envelope.Type,
		}
		if envelope.Error != nil {
			result.errorCode = envelope.Error.Code
		}
		body = envelope.Data
		if dst != nil {
			err = protojson.Unmarshal(body, dst)
		}
	}
	if err != nil {
		c.t.Fatal(err)
	}
	if result.errorCode != "" && len(body) != 0 {
		c.t.Fatal("错误响应不能同时携带 data")
	}
	return result
}
