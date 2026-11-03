# Android iOS FE WebSocket PB 联调指南

适用于 pixiu-demo 当前 WebSocket 协议，协议快照日期为 2026-11-03。Android、iOS 和 FE 共用本文的两份 proto，分别生成或加载各自语言的类型。连接使用 WebSocket，数据使用二进制 Protobuf；本文中的业务示例为设置当前页面、接收状态响应和接收服务端事件。

发送时先编码业务正文，再把所得 bytes 放进公共 `Envelope.data`，最后编码整个 Envelope。接收时先解码 Envelope，检查 `kind` 和 `data/error`，再按业务 `type` 解码正文。

## 一 联调资料与连接参数

联调包包含以下文件。两份 proto 均无 `import`，无需额外的业务协议依赖；两者也无需互相 import，因为公共信封通过 `bytes data` 承载业务消息。

| 文件 | 用途 |
| --- | --- |
| `realtime.proto` | 公共信封 `Envelope`、消息种类 `Kind`、业务错误 `Error` |
| `client.proto` | 业务正文 `SetPageRequest`、`StateReply`、`Connected` |
| `PB-CLIENT-INTEGRATION.md` | 连接、编解码、路由、收发示例和验收步骤 |

| 项目 | 当前 demo 约定 |
| --- | --- |
| 本机连接地址 | `ws://127.0.0.1:8889/ws/app` |
| 真机或远程连接 | 将主机和端口替换为后端提供的可达地址；`127.0.0.1` 指客户端所在设备自身 |
| TLS 入口 | 后端提供 HTTPS 入口时使用对应的 `wss://` 地址 |
| 握手子协议 | `pixiu.realtime.v1.protobuf` |
| 数据消息类型 | 双向均使用 WebSocket Binary |
| 服务就绪检查 | 对应 HTTP 地址的 `GET /ready/ws` 返回 204 |
| 当前 demo 身份 | 所有连接使用 `demo-user` / `demo-device`，不验证登录凭据 |

握手请求必须带上以下子协议，升级响应应选中同一个值：

```http
Sec-WebSocket-Protocol: pixiu.realtime.v1.protobuf
```

本轮 PB 联调只提供上述 PB 子协议。如果同时提供 `pixiu.realtime.v1.json`，服务端优先选择 JSON，随后必须用 Text 消息，不能继续发送 PB。连接建立后不可切换编码。

FE 从不同主机、协议或端口的页面连接时，需要后端通过 `Transport.AllowedOrigins` 放行页面的完整 Origin，例如 `http://localhost:5173`。默认按同源校验，因此直接从另一端口连接可能被拒绝。HTTPS 页面连接使用 `wss://`，避免被浏览器作为混合内容拦截。

上述身份仅用于当前 demo。接入实际登录后，后端需要另行给出凭据传递及失效处理约定。

## 二 两份完整 proto

以下代码块可以分别保存为对应文件。`go_package` 仅指定 Go 代码的包路径，各客户端保留原值即可。仓库中的 `internal/` 目录名不限制 Android、iOS 或 FE 使用协议文件。

### 公共信封 realtime.proto

```proto
syntax = "proto3";

package pixiu.realtime.v1;

option go_package = "github.com/bpcoder16/pixiu/biz/realtime/protocol";

// Kind 区分客户端请求、服务端响应和服务端主动事件。
enum Kind {
  KIND_UNSPECIFIED = 0;
  REQUEST = 1;
  RESPONSE = 2;
  EVENT = 3;
}

// Error 仅承载业务显式允许公开的错误，不包含内部错误文本。
message Error {
  string code = 1;
  string message = 2;
}

// Envelope 是 PB 公共信封；JSON 的 data 直接使用业务 ProtoJSON 值。
message Envelope {
  Kind kind = 1;
  string request_id = 2;
  // scene/action 只用于上行请求路由，下行不设置。
  string scene = 3;
  string action = 4;
  oneof body {
    bytes data = 5;
    Error error = 6;
  }
  // type 只用于下行消息，由业务显式指定，响应、错误响应和事件均必填。
  string type = 7;
}
```

### 业务正文 client.proto

```proto
syntax = "proto3";

package pixiudemo.ws.v1;

option go_package = "pixiu-demo/internal/wspb";

// 同一份业务定义同时用于 ProtoJSON 和二进制 PB。
message SetPageRequest {
  string page = 1;
}

message StateReply {
  string user_id = 1;
  string device_id = 2;
  string page = 3;
  uint64 revision = 4;
}

message Connected {
  string connection_id = 1;
}
```

## 三 信封与业务类型对应关系

### 信封字段

| 字段 | 客户端请求 | 服务端响应 | 服务端事件 |
| --- | --- | --- | --- |
| `kind` | `REQUEST = 1` | `RESPONSE = 2` | `EVENT = 3` |
| `request_id` | 必填，由客户端生成 | 回显原请求值 | 不携带 |
| `scene` | 必填，用于路由 | 不携带 | 不携带 |
| `action` | 必填，用于路由 | 不携带 | 不携带 |
| `type` | 不设置 | 必填，标识业务类型 | 必填，标识业务类型 |
| `data` | 必须选择该 oneof 分支 | 成功正文，与 error 二选一 | 事件正文，与 error 二选一 |
| `error` | 禁止携带 | 公开业务错误 | 公开业务错误 |

`request_id`、`scene`、`action` 和下行 `type` 均为 1–128 字节，允许 ASCII 字母、数字、点、下划线、短横线，区分大小写。客户端使用 `r1`、`r2` 或 UUID 等值关联请求；同一连接中等待响应的请求应使用不同 ID。服务端只回显 ID，不基于 ID 实现业务去重。

生成代码中的字段可能采用 `requestId`、`userId`、`deviceId` 等语言风格命名，以生成物为准；线上 PB 根据字段编号编码。

`data/error` 是 oneof，必须且只能选择一个。空 bytes 也可以表示已选择的合法 `data` 分支，不能用 `data.length > 0` 判断是否有正文。客户端应检查 oneof 分支或字段存在性。请求缺少 `data`、携带 `error`、缺少必填标识或 `kind` 错误会被视为无效信封。

### 业务路由与下行类型

| 方向 | 路由或 type | 正文类型 | 说明 |
| --- | --- | --- | --- |
| 请求 | `scene=client`，`action=setPage` | `pixiudemo.ws.v1.SetPageRequest` | 更新当前连接的页面状态 |
| 响应 | `type=client.state` | `pixiudemo.ws.v1.StateReply` | 返回更新后的状态，回显 requestId |
| 响应 | `type=request.error` | `Envelope.error` | 请求失败，无需解码业务 data |
| 事件 | `type=session.connected` | `pixiudemo.ws.v1.Connected` | 建连后服务端主动发送连接 ID |
| 事件 | `type=client.state` | `pixiudemo.ws.v1.StateReply` | 后端投递状态事件时使用；不关联请求 |

`type` 是双方约定的业务字符串，不是 protobuf 消息全名。相同 `client.state` 既可能是响应，也可能是主动事件：先用 `kind` 区分是否需要关联请求，再根据 `type` 选择正文类型。下行不通过 `scene/action` 分发。

## 四 请求响应与推送示例

以下 `text` 代码块是解码后的结构示意，省略字段代表未设置。实际网络消息是完整 Envelope 的 PB bytes，不能把这些结构直接作为字符串发送，也不能把业务正文改成 JSON 或 base64 后放入 data。

### 设置当前页面

业务正文：

```text
SetPageRequest {
  page: "home"
}
```

编码正文后构造请求信封：

```text
Envelope {
  kind: REQUEST                 // 数值 1
  request_id: "r1"
  scene: "client"
  action: "setPage"
  data: PBEncode(SetPageRequest{page: "home"})
}
```

编码层次：

```text
SetPageRequest
    → PBEncode → 业务 bytes
    → Envelope.data
    → PBEncode → 信封 bytes
    → 一条 WebSocket Binary 消息
```

对应的参考十六进制数据如下，空格仅用于阅读。发送时转换成字节，不能发送十六进制文本。

```text
业务正文，6 字节：
0a 04 68 6f 6d 65

完整请求信封，31 字节：
08 01 12 02 72 31 1a 06 63 6c 69 65 6e 74
22 07 73 65 74 50 61 67 65 2a 06 0a 04 68 6f 6d 65
```

该参考数据使用当前 Go 类型编码并完成解码校验。PB 编码字段顺序可以不同，跨语言校验应比较解码结果，无需强求编码字节完全相同。

服务端成功响应：

```text
Envelope {
  kind: RESPONSE               // 数值 2
  request_id: "r1"
  type: "client.state"
  data: PBEncode(StateReply{
    user_id: "demo-user"
    device_id: "demo-device"
    page: "home"
    revision: 1
  })
}
```

`page` 必须为 1–128 个 UTF-8 字节，中文字符可能占多个字节。每次有效设置会更新当前连接的 page，并将 revision 加一，即使 page 与上次相同。新连接从 revision 0 开始，首次成功设置返回 1；断线重连后不继承旧状态。不同连接即使使用相同用户或设备身份，状态仍各自独立。

### 请求失败

发送空 page 或超过 128 字节的 page，会收到：

```text
Envelope {
  kind: RESPONSE
  request_id: "r2"
  type: "request.error"
  error: Error{
    code: "INVALID_ARGUMENT"
    message: "page 长度必须为 1–128 字节"
  }
}
```

| 触发条件 | error.code | error.message |
| --- | --- | --- |
| 正文无法解码为请求类型 | `INVALID_ARGUMENT` | 请求正文格式错误 |
| page 不满足长度约束 | `INVALID_ARGUMENT` | page 长度必须为 1–128 字节 |
| 未注册的 scene/action | `NOT_FOUND` | 不支持的消息 |

这些业务错误通过正常 Binary 响应返回，状态不更新，连接可以继续使用。客户端应按 `error.code` 处理业务分支，message 用于提示。WebSocket 业务错误使用信封的 `error`，不套 HTTP 接口的 `code/msg/data` 包装。

### 建连事件

服务端在建连回调中主动发送：

```text
Envelope {
  kind: EVENT                  // 数值 3
  type: "session.connected"
  data: PBEncode(Connected{
    connection_id: "由服务端生成的本次连接ID"
  })
}
```

该事件不带 requestId，不需要客户端应答；demo 为其设置 10 秒投递有效期。connectionId 标识本次连接，重连会创建新的连接。

### 状态推送事件

后端主动投递状态事件时，客户端收到的格式为：

```text
Envelope {
  kind: EVENT
  type: "client.state"
  data: PBEncode(StateReply{
    page: "news"
    revision: 7
  })
}
```

此处字段值仅为事件示例，不表示已执行一次 setPage 请求，也不保证会在连接后自动发送。事件也可能选择 `error` 分支；无论 type 为何，都应先检查 data/error。服务端使用推送命令外层的 `kind`、`id` 选择目标，内层的 `expiresAt` 控制有效期。客户端只看到 WebSocket 事件的 `kind=3`、业务 `type` 和 `data` 或 `error`。

## 五 各端编解码接入

### 所有端共用的处理顺序

以下为语言无关伪代码，PBEncode/PBDecode 使用各端生成类型或运行库实现：

```text
发送：
  body = PBEncode(SetPageRequest(page = "home"))
  envelope = Envelope(
    kind = REQUEST,
    request_id = "r1",
    scene = "client",
    action = "setPage",
    body = data(body)
  )
  WebSocket.sendBinary(PBEncode(envelope))

接收：
  envelope = PBDecode(Envelope, binaryMessage)
  校验 kind 为 RESPONSE 或 EVENT
  如果是 RESPONSE，用 request_id 查找等待中的请求
  如果 body 是 error，处理 code/message 并结束该次处理
  如果 body 不是 data，记录协议异常并结束该次处理
  根据 type 选择 StateReply 或 Connected，解码 envelope.data
  如果 type 未识别，记录并跳过该事件或结束对应请求的等待
```

一条完整 WebSocket Binary 消息只承载一个 Envelope。使用普通 PB 编解码，不额外添加自定义消息头、总长度、分隔符或 gRPC 封装；不要调用会在消息外再增加长度前缀的 delimited 编码方法。WebSocket 分片由客户端库组装后再解码。

### Android

将两份 proto 纳入 Android 项目的 protobuf 生成流程，Kotlin 可以调用生成的 Java 类型。业务消息先序列化为 bytes，再通过信封的 data setter 设置 oneof 分支，最后以二进制发送。接收时先解析 Envelope，检查 body 的 case，再解析业务 bytes；不要仅依靠 error 的默认 getter 判断是否出错。Java 生成类型的序列化、解析及 oneof 用法见 [Protocol Buffers Java 官方文档](https://protobuf.dev/reference/java/java-generated/)。

### iOS

将两份 proto 交给 SwiftProtobuf 生成代码，并在项目中使用匹配的运行库。发送与接收均使用二进制序列化接口，检查生成的 body 枚举分支。`revision` 对应 `UInt64`。具体生成类型名和接口见 [SwiftProtobuf 官方说明](https://github.com/apple/swift-protobuf) 与 [SwiftProtobuf Message API](https://github.com/apple/swift-protobuf/blob/main/Documentation/API.md)。

Android 和 iOS 使用各自项目已有的 WebSocket 库，握手必须提供 PB 子协议，发送时选择二进制方法。

### FE 收发示例

下面示例采用 protobuf.js 完整版的运行时加载方式，以便直接读取两份 proto；如果项目已有生成代码方案，可沿用现有方案。protobuf.js 提供 `.proto` 加载、`encode` 和 `decode` 接口，见 [protobuf.js 官方说明](https://github.com/protobufjs/protobuf.js)。

示例假定前端工程已经安装并锁定 `protobufjs` 版本，将两份 proto 放到能通过页面同源 HTTP 访问的 `/proto/` 路径，并在支持 ES module 的工程入口调用此函数。后端 WebSocket 服务不会自动提供这些静态文件。

```javascript
import * as protobuf from "protobufjs";

export async function connectPB(url) {
  const root = await protobuf.load([
    "/proto/realtime.proto",
    "/proto/client.proto",
  ]);
  const Envelope = root.lookupType("pixiu.realtime.v1.Envelope");
  const SetPageRequest = root.lookupType("pixiudemo.ws.v1.SetPageRequest");
  const StateReply = root.lookupType("pixiudemo.ws.v1.StateReply");
  const Connected = root.lookupType("pixiudemo.ws.v1.Connected");
  const ws = new WebSocket(url, "pixiu.realtime.v1.protobuf");
  ws.binaryType = "arraybuffer";

  ws.onopen = () => {
    if (ws.protocol !== "pixiu.realtime.v1.protobuf") {
      ws.close();
      return;
    }
    const data = SetPageRequest.encode({ page: "home" }).finish();
    const wire = Envelope.encode({
      kind: 1,
      requestId: "r1",
      scene: "client",
      action: "setPage",
      data,
    }).finish();
    ws.send(wire);
  };

  ws.onmessage = (event) => {
    try {
      if (!(event.data instanceof ArrayBuffer)) {
        throw new Error("expected binary message");
      }
      const envelope = Envelope.decode(new Uint8Array(event.data));
      if (envelope.kind !== 2 && envelope.kind !== 3) {
        throw new Error("unexpected kind");
      }
      if (envelope.kind === 2 && !envelope.requestId) {
        throw new Error("missing requestId");
      }
      if (envelope.body === "error") {
        console.error(envelope.requestId, envelope.type, envelope.error);
        return;
      }
      if (envelope.body !== "data") {
        throw new Error("missing body");
      }
      const Body = envelope.type === "client.state" ? StateReply
        : envelope.type === "session.connected" ? Connected : null;
      if (!Body) {
        console.warn("unknown type", envelope.type);
        return;
      }
      const message = Body.decode(envelope.data);
      const value = Body.toObject(message, { longs: String, defaults: true });
      console.log(envelope.kind, envelope.requestId, envelope.type, value);
    } catch (error) {
      console.error("PB decode failed", error);
    }
  };

  ws.onclose = (event) => console.log("closed", event.code, event.reason);
  ws.onerror = (event) => console.error("websocket error", event);
  return ws;
}

// 替换为后端提供的可达地址。
// connectPB("ws://127.0.0.1:8889/ws/app").catch(console.error);
```

示例只发一个固定 ID 的请求，用于检查收发链路。实际页面需要管理不同 requestId 的等待项、超时、断连清理和未知类型处理。

protobuf.js 默认把 snake_case 字段转为 camelCase，因此代码使用 `requestId`。`revision` 是 uint64，FE 应使用支持完整 64 位整数的运行库配置，并转成十进制字符串展示或保存，避免转成 JavaScript Number 后丢失精度。示例使用 `longs: String` 输出；如果解码阶段已经使用不安全的 Number，事后转字符串不能恢复精度。

## 六 心跳关闭与排查

当前默认服务端每 20 秒发送 WebSocket Ping，收到 Pong 后续期，Pong 超时为 60 秒。使用原生控制帧，不需要封装业务 PB 心跳消息。Android 和 iOS 应确认客户端库正确处理 Ping/Pong；浏览器由运行时处理控制帧。

当前 realtime 默认入站和出站消息上限各为 64 KiB，包含完整信封。不要仅根据业务 data 大小判断是否超限；后端调整配置后以环境实际值为准。

| 现象 | 检查方向 |
| --- | --- |
| 升级前 HTTP 400 | 缺少 PB 子协议或未匹配到支持的子协议 |
| 升级前 HTTP 403 | 浏览器 Origin 未通过校验 |
| 升级前 HTTP 503 | 服务未就绪或正在关闭 |
| 关闭码 1003 | 协商 PB 后发送了 Text 消息 |
| 关闭码 1007 | 无效 PB 信封、缺少必填字段、错误 kind 或缺少 data 分支 |
| 关闭码 1009 | 入站消息超过限制 |
| 关闭码 1013 | 服务暂时繁忙，例如响应发送队列或预算不足 |
| 收到 request.error | 根据 error.code 修正业务请求；连接可继续使用 |
| 信封正常但正文解码失败 | 检查 type 对应关系、proto 版本及 data 是否经过错误的 JSON/base64 转换 |

握手错误通常还需结合后端日志定位；浏览器未必允许业务代码直接读取握手失败的 HTTP 状态。

连接断开后清理等待中的请求；重连创建新会话和新状态。重发请求是否安全由业务决定，requestId 不提供自动去重，也不能将“已发送”当成“已执行成功”。

## 七 联调验收步骤

1. 准备两份 proto，在各端完成代码生成或加载；本地先解码本文的 31 字节请求，确认 page 为 home、路由为 client/setPage。
2. 使用后端提供的地址连接，只声明 PB 子协议，确认升级成功且选中 `pixiu.realtime.v1.protobuf`。
3. 正常建连后接收 `kind=3`、`type=session.connected`，按 Connected 解码 connectionId。
4. 发送 `requestId=r1`、page 为 home 的请求，确认响应 `kind=2`、`type=client.state`，requestId 为 r1，page 为 home，首次 revision 为 1。
5. 在同一连接发送第二个有效请求，确认 revision 加一。再发送空 page，确认收到 INVALID_ARGUMENT；随后发送有效请求，确认连接仍可用且失败请求没有增加 revision。
6. 发送合法信封但使用未注册 action，确认收到 NOT_FOUND。不要用非法信封替代此步骤，否则连接会关闭。
7. 配合后端触发 client.state 主动推送，确认 `kind=3`、无 requestId，并使用 StateReply 解码；同时确认事件不会被误认为请求响应。
8. 重新建立连接，再次设置 page，确认新连接的首次 revision 为 1。

## 八 协议交付与维护

服务端业务 proto 来源为 `pixiu-demo/internal/wspb/client.proto`，公共信封来源为 `pixiu/biz/realtime/protocol/realtime.proto`。联调时后端统一提供匹配部署版本的两份文件，各端随项目锁定同一协议快照；不要从不同版本分别复制两份 proto。

新增字段使用新编号，既有字段编号和类型保持兼容；新增上行操作或下行 type 时，同步补充路由与正文映射。`type` 不会自动推导正文类型，新增消息需要各端明确登记。
