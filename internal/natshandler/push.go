package natshandler

import (
	"context"

	"pixiu-demo/internal/infra/websocket"
	"pixiu-demo/internal/wspb"

	"github.com/bpcoder16/pixiu/biz/realtime"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

// HandleAppPush 在处理消息时获取本进程的 App 实例，不接管其生命周期。
func HandleAppPush(ctx context.Context, msg *nats.Msg) error {
	cmd, err := realtime.UnmarshalPushCommand(msg.Data, newPushData)
	if err != nil {
		return err
	}
	server := websocket.App()
	switch cmd.Kind {
	case realtime.PushBroadcast:
		_, err = server.Broadcast(ctx, cmd.Message)
	case realtime.PushUser:
		_, err = server.PushToUser(ctx, cmd.ID, cmd.Message)
	case realtime.PushDevice:
		_, err = server.PushToDevice(ctx, cmd.ID, cmd.Message)
	default:
		return realtime.ErrInvalidMessage
	}
	return err
}

// type 是客户端的业务事件名，返回独立可写对象，供当前消息的 ProtoJSON 解码使用。
func newPushData(typ string) proto.Message {
	switch typ {
	case "client.state":
		return new(wspb.StateReply)
	default:
		return nil
	}
}
