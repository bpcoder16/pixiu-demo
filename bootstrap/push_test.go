package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bpcoder16/pixiu/biz/appconfig"
	wsinfra "pixiu-demo/internal/infra/websocket"
	"pixiu-demo/internal/natshandler"
	"pixiu-demo/internal/natspublisher"
	"pixiu-demo/internal/wshandler"
	"pixiu-demo/internal/wspb"

	pixiubootstrap "github.com/bpcoder16/pixiu/biz/bootstrap"
	"github.com/bpcoder16/pixiu/biz/realtime"
	"github.com/bpcoder16/pixiu/biz/realtime/protocol"
	"github.com/bpcoder16/pixiu/infra/apprun"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats-server/v2/server"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type client struct {
	c      *websocket.Conn
	binary bool
}

func connect(t *testing.T, url string, user, device, encoding string) client {
	t.Helper()
	d := websocket.Dialer{Subprotocols: []string{encoding}}
	c, _, err := d.Dial(url, http.Header{"X-User": {user}, "X-Device": {device}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return client{c: c, binary: encoding == realtime.SubprotocolProtobuf}
}
func (c client) request(t *testing.T) {
	t.Helper()
	kind := websocket.TextMessage
	wire := []byte(`{"kind":1,"requestId":"barrier","scene":"client","action":"setPage","data":{"page":"home"}}`)
	if c.binary {
		kind = websocket.BinaryMessage
		body, err := proto.Marshal(&wspb.SetPageRequest{Page: "home"})
		if err != nil {
			t.Fatal(err)
		}
		wire, err = proto.Marshal(&protocol.Envelope{Kind: protocol.Kind_REQUEST, RequestId: "barrier", Scene: "client", Action: "setPage", Body: &protocol.Envelope_Data{Data: body}})
		if err != nil {
			t.Fatal(err)
		}
	}
	_ = c.c.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := c.c.WriteMessage(kind, wire); err != nil {
		t.Fatal(err)
	}
}

type frame struct {
	kind                 protocol.Kind
	typ, requestID, code string
	body                 []byte
}

func (c client) read(t *testing.T) frame {
	t.Helper()
	_ = c.c.SetReadDeadline(time.Now().Add(3 * time.Second))
	kind, data, err := c.c.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if c.binary {
		var e protocol.Envelope
		if err := proto.Unmarshal(data, &e); err != nil || kind != websocket.BinaryMessage {
			t.Fatalf("PB 帧错误: %v", err)
		}
		return frame{e.Kind, e.Type, e.RequestId, e.GetError().GetCode(), e.GetData()}
	}
	var e struct {
		Kind                   protocol.Kind
		Type, RequestID        string
		Data                   json.RawMessage
		Error                  *realtime.Error
		ID, Message, ExpiresAt json.RawMessage
	}
	if err := json.Unmarshal(data, &e); err != nil || kind != websocket.TextMessage || e.ID != nil || e.Message != nil || e.ExpiresAt != nil {
		t.Fatalf("JSON 帧错误: %s, %v", data, err)
	}
	f := frame{kind: e.Kind, typ: e.Type, requestID: e.RequestID, body: e.Data}
	if e.Error != nil {
		f.code = e.Error.Code
	}
	return f
}
func (c client) barrier(t *testing.T) {
	t.Helper()
	c.request(t)
	f := c.read(t)
	if f.kind != protocol.Kind_RESPONSE || f.requestID != "barrier" || f.code != "" {
		t.Fatalf("应收到请求响应，出现越界或重复投递: %+v", f)
	}
}

const appPushTestSubject = "pixiu.demo.ws.app.push"

func waitSubscriptions(t *testing.T, bus *server.Server, subject string, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		subs, err := bus.Subsz(&server.SubszOptions{Subscriptions: true, Test: subject})
		if err != nil {
			t.Fatal(err)
		}
		if subs.Total == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("%s 未就绪，期待 %d 个订阅", subject, want)
}

// 子进程拥有独立的全局命名表，模拟两个 App 节点及一个不同主题的端。
func TestPushNodeProcess(t *testing.T) {
	dir := os.Getenv("PIXIU_WS_NODE_DIR")
	if dir == "" {
		t.Skip("由跨节点测试启动")
	}
	appconfig.MustLoadAppConfig(filepath.Join(dir, "app.yaml"))
	// 仅测试进程绕过会改写 time.Local 的基础初始化，实际声明到命名表的链路由 Pixiu 回归覆盖。
	newExampleClient(t, os.Getenv("PIXIU_WS_NODE_NATS"), "demo", true)
	app, err := realtime.NewServer(realtime.Config{
		Authenticate: func(_ context.Context, r *http.Request) (realtime.ClientInfo, error) {
			return realtime.ClientInfo{UserID: r.Header.Get("X-User"), DeviceID: r.Header.Get("X-Device")}, nil
		},
		RegisterHandlers: wshandler.RegisterApp,
		NewState:         func(realtime.ClientInfo) any { return &wshandler.AppClientState{} },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := realtime.Register(wsinfra.AppName, app); err != nil {
		t.Fatal(err)
	}
	ws, err := pixiubootstrap.NewWebSocketService(wsinfra.AppName)
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := pixiubootstrap.NewNATSConsumerService("demo", natshandler.JetStreamRegistry, natshandler.CoreRegistry)
	if err != nil {
		ws.Cancel()
		t.Fatal(err)
	}
	manager, err := pixiubootstrap.NewAppManager()
	if err != nil {
		t.Fatal(err)
	}
	for _, service := range []apprun.Service{ws, consumer} {
		if err := manager.Register(service); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() { _, _ = io.Copy(io.Discard, os.Stdin); cancel() }()
	if err := manager.Run(ctx); err != nil {
		t.Fatal(err)
	}
}

func pushNode(t *testing.T, bus *server.Server, name, subject string) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	dir := t.TempDir()
	registration, err := os.ReadFile("../conf.example/nats-register.demo.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"app.yaml":                "env:\n  appName: pixiu-demo-push-node\n  runMode: release\n  timeLocation: UTC\n  localIP: 127.0.0.1\n",
		"websocket.app.yaml":      "addr: " + addr + "\npath: /ws/app\n",
		"nats-register.demo.yaml": strings.ReplaceAll(string(registration), appPushTestSubject, subject),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestPushNodeProcess$")
	cmd.Env = append(os.Environ(), "PIXIU_WS_NODE_DIR="+dir, "PIXIU_WS_NODE_NATS="+bus.ClientURL())
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	stop := sync.OnceFunc(func() {
		_ = input.Close()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("节点 %s 退出失败: %v\n%s", name, err, output.String())
			}
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			<-done
			t.Errorf("节点 %s 未关闭\n%s", name, output.String())
		}
	})
	t.Cleanup(stop)
	client := &http.Client{Timeout: time.Second}
	defer client.CloseIdleConnections()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get("http://" + addr + "/ready/ws")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusNoContent {
				return "ws://" + addr + "/ws/app", stop
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	stop()
	t.Fatal("节点未就绪")
	return "", stop
}

func TestCrossNodePush(t *testing.T) {
	if !initExampleProcess(t) {
		return
	}
	bus := startExampleServer(t)
	newExampleClient(t, bus.ClientURL(), "publisher", true)
	a, stopA := pushNode(t, bus, "first", appPushTestSubject)
	b, _ := pushNode(t, bus, "second", appPushTestSubject)
	admin, _ := pushNode(t, bus, "admin", "pixiu.demo.ws.admin.push")
	waitSubscriptions(t, bus, appPushTestSubject, 2)
	waitSubscriptions(t, bus, "pixiu.demo.ws.admin.push", 1)
	clients := map[string]client{
		"phone":  connect(t, a, "u", "phone", realtime.SubprotocolJSON),
		"tablet": connect(t, b, "u", "tablet", realtime.SubprotocolProtobuf),
		"tab":    connect(t, a, "u", "tablet", realtime.SubprotocolJSON),
		"other":  connect(t, a, "other", "pc", realtime.SubprotocolJSON),
		"admin":  connect(t, admin, "u", "phone", realtime.SubprotocolJSON),
	}
	for _, c := range clients {
		c.barrier(t)
	}
	publisher := natspublisher.NewPublisher()
	state := &wspb.StateReply{Page: "news", Revision: 1 << 63}
	msg := realtime.PushMessage{Type: "client.state", Data: state}
	for _, tc := range []struct {
		name string
		kind realtime.PushKind
		id   string
		msg  realtime.PushMessage
		to   []string
	}{
		{"broadcast", realtime.PushBroadcast, "", msg, []string{"phone", "tablet", "tab", "other"}},
		{"user", realtime.PushUser, "u", msg, []string{"phone", "tablet", "tab"}},
		{"device", realtime.PushDevice, "tablet", msg, []string{"tablet", "tab"}},
		{"error", realtime.PushUser, "u", realtime.PushMessage{Type: "state.error", Error: &realtime.Error{Code: "STALE", Message: "请刷新"}}, []string{"phone", "tablet", "tab"}},
		{"offline", realtime.PushUser, "offline", msg, nil},
		{"expired", realtime.PushBroadcast, "", realtime.PushMessage{Type: "client.state", Data: state, ExpiresAt: time.Unix(1, 0)}, nil},
		{"unknown-body", realtime.PushBroadcast, "", realtime.PushMessage{Type: "unregistered", Data: state}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := publisher.PublishAppPush(t.Context(), realtime.PushCommand{Kind: tc.kind, ID: tc.id, Message: tc.msg}); err != nil {
				t.Fatal(err)
			}
			// 同一发布连接、同一 subject 上的广播作为 FIFO 屏障。每条 app 连接都必须读到且只读到预期事件。
			marker := realtime.PushCommand{Kind: realtime.PushBroadcast, Message: realtime.PushMessage{Type: "push.barrier", Error: &realtime.Error{Code: "MARK"}}}
			if err := publisher.PublishAppPush(t.Context(), marker); err != nil {
				t.Fatal(err)
			}
			for name, c := range clients {
				if name == "admin" {
					c.barrier(t)
					continue
				}
				if slices.Contains(tc.to, name) {
					f := c.read(t)
					if f.kind != protocol.Kind_EVENT || f.typ != tc.msg.Type || f.requestID != "" {
						t.Fatalf("%s 收到错误帧: %+v", name, f)
					}
					if tc.msg.Error != nil {
						if f.code != tc.msg.Error.Code || len(f.body) != 0 {
							t.Fatalf("%s 错误事件: %+v", name, f)
						}
					} else {
						got := new(wspb.StateReply)
						var err error
						if c.binary {
							err = proto.Unmarshal(f.body, got)
						} else {
							err = protojson.Unmarshal(f.body, got)
						}
						if f.code != "" || err != nil || !proto.Equal(got, state) {
							t.Fatalf("%s 错误正文: %v, %v", name, got, err)
						}
					}
				}
				if f := c.read(t); f.typ != "push.barrier" || f.code != "MARK" {
					t.Fatalf("%s 出现越界或重复投递: %+v", name, f)
				}
			}
		})
	}
	stopA()
	if err := publisher.PublishAppPush(t.Context(), realtime.PushCommand{Kind: realtime.PushBroadcast, Message: msg}); err != nil {
		t.Fatal(err)
	}
	if f := clients["tablet"].read(t); f.typ != msg.Type {
		t.Fatalf("停止 A 影响 B: %+v", f)
	}
}
