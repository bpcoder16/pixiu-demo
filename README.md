# pixiu-demo

基于 [Pixiu](https://github.com/bpcoder16/pixiu)、Gin 和 GORM 的 Go HTTP 服务基础骨架，用于演示项目分层、统一响应、基础设施装配、日志和资源生命周期。

当前提供两个公开 Ping 样例、OSS 文件上传、ES 同义词词库验证与搜索分析器重载、阿里云 ES 同义词词典查询与更新、实例详情查询接口、MySQL 主从会话入口、Redis、Elasticsearch、阿里云 OSS、阿里云 ES、NATS 默认实例及消息收发示例，以及 App/Admin 分模块的 OpenAPI 文档。项目没有身份认证或令牌处理；接入实际业务时自行实现相应规则。

应用入口使用 Pixiu 的 `biz/appconfig`，直接将配置传给 `bootstrap.MustBaseInit`；env/log 文件结构保持不变。HTTP 和 Cron 通过 `bootstrap.Start` 一起运行，示例任务每 10 秒记录一次心跳日志。

启动入口将 NATS、HTTP、Cron 的构造函数传给 bootstrap.Start，统一构造、登记并接管 SIGINT/SIGTERM。失败时清理已构造服务；运行期统一停收，共享默认 30 秒并行排空，超时取消剩余任务。随后 main 回收资源栈，整体默认 20 秒。

## 开发环境与启动

[`go.mod`](go.mod) 声明 Go 1.27.2，通过本地 `replace` 使用相邻目录中的 Pixiu；构建时使用该目录的源码：

```text
workspace/
├── pixiu-demo/
└── pixiu/
```

首次使用时，在同一父目录克隆两个仓库，再进入 pixiu-demo 准备依赖和配置：

```bash
git clone https://github.com/bpcoder16/pixiu.git
git clone https://github.com/bpcoder16/pixiu-demo.git
cd pixiu-demo
go mod download
mkdir -p conf
cp -n conf.example/*.yaml conf/
```

已有相邻工作目录时，从项目根目录执行 `go mod download` 及后续命令即可。

启动前准备相应的 MySQL、Redis、Elasticsearch 和 NATS 服务，并在 `conf/` 中填写实际连接参数；阿里云 OSS 和阿里云 ES 需配置 Region、Bucket 或 InstanceID 及访问凭据。`conf/` 被 Git 忽略，运行配置不应提交到仓库。

| 文件 | 用途 |
| --- | --- |
| [`app.yaml`](conf.example/app.yaml) | 应用名、运行模式、时区、日志目录和轮转 |
| [`gin.yaml`](conf.example/gin.yaml) | 请求日志、可信代理和详情采集；模板默认开启请求/响应详情 |
| [`http.yaml`](conf.example/http.yaml) | 监听地址、请求超时和关闭期限，模板监听 `:8888` |
| [`cron.yaml`](conf.example/cron.yaml) | 心跳任务、执行间隔、任务超时及 Cron 关闭期限 |
| [`mysql.demo.yaml`](conf.example/mysql.demo.yaml) | 默认 MySQL 主库、可选从库及连接池 |
| [`migrate.demo.yaml`](conf.example/migrate.demo.yaml) | Model 自动迁移开关和整体期限；模板已开启，当前表清单为空 |
| [`redis.demo.yaml`](conf.example/redis.demo.yaml) | 默认 Redis 连接、重试和连接池 |
| [`elasticsearch.demo.yaml`](conf.example/elasticsearch.demo.yaml) | 默认 ES 数据面实例；版本须与实际服务端一致 |
| [`aliyunossx.demo.yaml`](conf.example/aliyunossx.demo.yaml) | 默认阿里云 OSS 实例，固定 Region、Bucket 和访问凭据 |
| [`aliyunesx.demo.yaml`](conf.example/aliyunesx.demo.yaml) | 默认阿里云 ES 管理 API 实例，固定 Region、InstanceID 和访问凭据；慢调用阈值 200ms |
| [`nats.demo.yaml`](conf.example/nats.demo.yaml) | 默认 NATS 连接、可选环境前缀及排空期限 |
| [`nats-register.demo.yaml`](conf.example/nats-register.demo.yaml) | Stream、JetStream/Core Consumer 声明及 Handler 绑定 |

基础设施实例名统一为 `demo`，由 `bootstrap.MustInit` 注册；配置文件中无需再次指定实例名或默认标识。启动会初始化全部六类基础设施，即使只访问 Ping，也需完成初始化。阿里云 OSS 和阿里云 ES 初始化只校验配置并构造客户端，不联网验活；实际访问时再验证凭据、权限和资源可用性。

实例声明完成后，`MustBaseInit` 先应用环境时区，再依次初始化日志 → 阿里云 OSS → 阿里云 ES → MySQL → Redis → Elasticsearch → NATS。随后 `MustInit` 执行 MySQL 迁移，成功返回后再由 `Start` 构造并启动 NATS 消费、HTTP、Cron；启动不发布示例消息。跨组件的声明顺序不改变上述初始化顺序；同一组件按声明顺序初始化、逆序关闭，依赖实例须先声明。

模板使用 `debug` 模式，启动前须生成接口文档：

```bash
go generate ./openapi
go run .
```

入口固定加载 `./conf/app.yaml`，运行时工作目录须包含 `conf/`。`release` 模式关闭文档路由，不要求生成文档；`debug`、`test` 模式在文档未生成时拒绝启动。

## NATS 消息示例

[`nats-register.demo.yaml`](conf.example/nats-register.demo.yaml) 集中声明 Stream 和消费者；[`registry.go`](internal/natshandler/registry.go) 与消费 Handler 同包，集中维护处理方法及稳定名称；`NewRegistries(handler)` 每次创建独立的 JetStream/Core 注册表，不获取客户端或启动消费。消费入口在 [`natshandler`](internal/natshandler/handler.go)，业务流程在 [`nats.Service`](internal/service/nats/service.go)，实际发送在 [`natspublisher`](internal/natspublisher/publish.go)。[`bootstrap/nats.go`](bootstrap/nats.go) 在依赖就绪后构造 Service、Handler，并调用注册表工厂完成绑定；配置读取与启动消费由 bootstrap 装配。消费和发布复用已初始化的 demo 客户端。服务端须开启 JetStream，例如本地执行 `nats-server -js`，再按前文配置其余基础设施并运行应用；客户端的 `jetStream` 配置不会开启服务端功能。

可选环境前缀 `namespace` 只在 `nats.demo.yaml` 配置，省略、`null` 或空字符串时不启用，模板默认为空；非空值只允许英文字母、数字、`_`、`-`，初始化后固定。注册配置及发布、Request 均填写原始名称和主题，由 Client 统一添加前缀。例如 `test1` 将 `pixiu.demo.tasks` 映射为 `test1.pixiu.demo.tasks`，Stream、Durable、Core 消费声明名称及非空 queue 添加 `test1_`。Handler 引用名称不变，回调主题保留前缀；修改前缀不迁移或删除旧 Stream/Durable。下表列出未添加前缀的原始名称。

| 通道 | 启动声明 | 消费方式 |
| --- | --- | --- |
| `pixiu.demo.tasks` | 文件存储、LimitsPolicy Stream `PIXIU_DEMO_TASKS`，Durable Consumer `demo-worker` | 每个 Client 1 个本地 worker；多个应用实例共享该 Durable，竞争处理任务 |
| `pixiu.demo.broadcast` | Core 订阅，Queue 留空 | 每个在线应用实例都收到一份 |
| `pixiu.demo.queue` | Core 订阅，Queue 为 `pixiu-demo-workers`，每个 Client 创建 4 个同组订阅 | 同组每条消息只交给一个订阅，Handler 可并发执行 |

启动注册：MustInit 初始化共享客户端并执行 MySQL 迁移；Start 的工厂装配消费链路，调用 `bootstrap.NewNATSConsumerService(nats.DemoName, jetStreamHandlers, coreHandlers)`，内部由 NewNATSConsumer 读取配置、绑定 Handler，返回服务和构造错误。Run 使用可由 Stop 取消的注册 context，依次注册 Stream、JetStream 和 Core；Stop 不等待远端注册完成。各注册入口每 Client 只调用一次，注册失败由管理器清理已启动部分，远端资源保留。Start 并发 Run，登记顺序不保证就绪；Handler 依赖须提前准备。

注册文件是必需文件，缺失或未知字段会阻止启动。`streamDefaults.replicas` 始终必填且为正整数，同时作为逐项副本数的上限，即使没有 Stream 或全部 Stream 都有覆盖值；单个 Stream 的 `replicas` 省略、`null` 或解码后不大于 0 时继承全局值，正值不得超过全局值，解码及数字转换沿用 Pixiu 的 `configx`。示例全局值为 1，适用于单节点；多副本须具备相应服务端条件。

注册模板中的可选调参项以注释展示，按需取消注释；省略或设为 `null` 时沿用 natsx Default：

| 位置 | 可选字段与默认值 |
| --- | --- |
| `streams[]` | `retention: "limits"`、`maxAge: "2160h"`、`duplicates: "2m"` |
| `jetStreamConsumers[]` | `ackWait: "60s"`、`maxDeliver: 5`、`maxAckPending: 1000`、`retryDelay: "0s"` |
| `jetStreamConsumers[].consume` | `workers: 1`、`maxMessages: 0` |

`retention` 可选 `limits`、`interest`、`workqueue`。`maxAge=0` 表示不限制年龄，`duplicates=0` 由服务端确定，不代表关闭去重；时长范围由服务端校验，`maxAge` 大于 0 时去重窗口不得超过它。时长使用 Go 单位，不支持 `d`。`ackWait` 必须大于 0；`maxDeliver`、`maxAckPending` 的范围由服务端校验。`workers=0` 按 1 处理，`maxMessages=0` 为有效 workers 的两倍，不含处理中的消息；workers、maxMessages、retryDelay 不得为负。其余参数沿用 Default，不支持 `enabled`、热更新或完整 SDK 配置。

Core 的 `queue` 非空时，`subscribers` 必填且必须大于 1，控制当前 Client 创建的同组订阅数，示例设为 4，Handler 须支持并发；`queue` 为空时广播，忽略此数量且始终只有一个订阅。

Handler 引用必须与登记名称完全一致，区分大小写；包级函数自动使用原始函数名，方法值和闭包用 `MustRegister(handler, name)` 显式命名。Durable 与 Handler 名称独立，函数重命名不应改变 Durable。只启动文件列出的消费者；新增使用已有 Handler 的消费者只需增加 YAML 项，新增处理逻辑再补登记。删除配置项后重启仅停用本进程对应消费，远端 Stream/Durable 和进度保留，不影响其他进程。三份列表可为空，但全局副本数仍必填。

从旧代码声明迁移时，Consumer 的 AckWait 从 30s 改为 60s、MaxDeliver 从 3 改为 5、本地 Workers 从 2 改为 1、RetryDelay 从 1s 改为 0。过滤主题使用 Default 的 FilterSubjects，服务端须支持 NATS Server 2.10 或以上。已有远端配置不一致时须在部署前显式对齐；启动不会自动更新或删除已有资源。后续升级 Pixiu 若 Default 改变，同样要检查已有资源兼容性。LimitsPolicy 下 ACK 不删除消息，默认仍受 5 GiB 容量、90 天保留期限和满额拒绝新消息的限制。

ACK 与失败重投：`natshandler.Handler.HandleTask` 接收 `context.Context` 和 `jetstream.Msg`，将原始文本转换为 `message.DemoMessage` 后调用 Service，并返回业务 error；Service 示例只记录 `DemoNATSJetStream`。

- 返回 nil 由组件 ACK；返回 error 或 panic 时，默认 `retryDelay=0` 保持未确认，等待服务端按 `AckWait=60s` 重投；配置正值 retryDelay 时发送延迟 NAK。默认 `MaxDeliver=5` 包含首次在内最多投递 5 次。
- 失败耗尽时记录日志，不 ACK/Term/主动 NAK，不自动删除或转入死信队列。handler 不自行 ACK/NAK/Term，须完成本条业务后再返回，支持并发且保证业务幂等。
- `ackWait` 应覆盖预取等待和处理时间，demo 默认为 60s；组件不自动 `InProgress`。可在 YAML 调整 `ackWait`、`maxDeliver`、`retryDelay`，不开放 `BackOff`。
- Core handler 接收 `context.Context` 和 `*nats.Msg`、返回 error，分别记录 `DemoNATSCoreBroadcast`、`DemoNATSCoreQueue`；error 或 panic 只记日志、不重投。需要请求响应时显式调用 `msg.Respond`。

同步与异步发布：启动后调用 `POST /api/app/nats/publish`，由 `httphandler.AppNATSPublish` 使用请求 context 调用 `nats.Service.PublishExamples`，向 Core 广播、Queue 和 JetStream 同步、异步各发送一条固定示例消息：

```bash
curl -sS 'http://127.0.0.1:8888/api/app/nats/publish' \
  -H 'Content-Type: application/json' \
  -d '{"messageIDPrefix":"demo-batch-1"}'
```

成功返回 `{"code":0,"msg":"成功"}`；缺少或空白前缀返回业务码 `10001`，发布失败返回 `400`，均使用 HTTP 200。Core 的 `Publish` 成功只表示客户端受理；`PublishJetStream` 等待 PubAck，`PublishJetStreamAsync` 先检查提交 error，再通过 `future.Ok()` / `future.Err()` 获取最终结果。接口成功不代表 handler 已完成；示例确认日志为 `DemoNATSPublish`。失败可能已有部分消息发出或消费，不回滚、不自动重试。

两种 JetStream 发布都要求非空 messageID，使用请求的 `messageIDPrefix` 分别加 `:sync`、`:async`；同一批重试复用前缀，新批次更换。去重只在同一 Stream 的服务端窗口内生效，不能替代业务幂等；重试接口会再次发送 Core 消息。异步 ctx 只用于提交前检查、日志和追踪，取消等待不撤销已提交消息；`PublishAsyncTimeout` 在 [`nats.demo.yaml`](conf.example/nats.demo.yaml) 中配置，默认 5 秒，与消费者 AckWait 无关。超时不保证消息未写入，重试仍复用原 ID。异步方法复制 data/Header，返回后可复用输入，不要修改 `future.Msg()`。

在 `log/pixiu-demo/biz/info.log` 查看上述 `DemoNATS*` 记录。启动多个连接同一 NATS 的应用实例（HTTP 需使用不同端口），调用测试接口可观察广播与 Queue 的差异。

退出排空：HTTP、Cron、NATS 统一停止接收新工作，共享 30 秒等待已接纳任务完成。NATS StopConsumers 停止新增接收，SDK pending 继续排空；期间消息 context 有效，Named/Default、发布、请求及 ACK 可继续使用。30 秒后 Cancel 未完成任务并回收资源，NATS 可强制关闭连接，剩余业务可能失败。资源栈默认整体 20 秒，逆序回收、日志最后关闭；到期立即交回 main 退出。业务不自行关闭共享客户端或在回调内等待自身关闭，CloseAll 开始后的命名查询仍 panic。

所有 NATS 关闭阶段共用 `CloseTimeout`（默认 20 秒），`DrainTimeout`（默认 10 秒）只用于底层连接 drain，两者不相加。排空完成不等于全部发布成功，Close 不汇总单条发布错误，仍须检查 future。整体关闭超时时，Close 返回 `natsx.ErrCloseTimeout`，剩余 future 以 `jetstream.ErrJetStreamPublisherClosed` 结束；这不保证消息未写入，重试仍复用原 messageID。强制关闭不能终止不响应 context 的业务函数。

## 定时任务

[`internal/cron/heartbeat.go`](internal/cron/heartbeat.go) 的 `Heartbeat` 直接接收调度器传入的 context，记录 `DemoCronHeartbeat`，沿用本轮独立的 `job_name`、`logId` 及耗时作用域，不访问数据库或外部服务。任务在 [`internal/cron/registry.go`](internal/cron/registry.go) 中通过 Pixiu 的 `infra/cronx` 包提供的 `cronx.MustNewHandlerRegistry(Heartbeat)` 批量声明，默认使用原始函数名 `Heartbeat`，保留大小写。[`bootstrap/start.go`](bootstrap/start.go) 调用 `bootstrap.NewCronService(cron.Registry.Handlers()...)` 完成调度器构造与服务适配，由 [`cron.yaml`](conf.example/cron.yaml) 决定是否启用及执行间隔；首次等待 10 秒，不在声明或构造时执行。

每个任务必须在 `every`、`cron`、`randomEvery` 中恰好选择一种调度方式。模板默认使用 `every: "10s"`，并提供另外两种方式的注释示例；切换时先注释原调度字段，再取消所选配置段的完整注释，不能同时启用。`cron: "0 10 0 * * *"` 表示每天 00:10:00 执行，使用 `app.yaml` 的 `env.timeLocation`；表达式必须为秒起始的六字段，不接受内嵌时区或描述符。`every` 和 `randomEvery.min` 最短均为 100ms，`randomEvery.max` 必须大于 min；随机示例的 min 为 15m、max 为 20m，间隔范围为 `[min, max)`。固定及随机间隔首次等待，不立即执行。

已有本地配置目录时需补齐新文件：

```bash
cp -n conf.example/cron.yaml conf/
```

`cron.yaml` 是必需文件，缺失、非法配置或引用未注册任务名会阻止整个应用启动。配置按任务名精确匹配，区分大小写；自动命名的 `Heartbeat` 必须配置为 `name: Heartbeat`，旧配置中的 `heartbeat` 需同步修改。注释该任务的完整配置项或改为 `jobs: []` 后重启即可停用，保留配置文件。新增包级任务时在 `internal/cron/` 实现 Handler，在 `Registry` 的批量声明中增加函数名，再在文件的 jobs 中启用，无需修改 `Start` 或为每个任务编写 `init()`。注册器实例独立，`Handlers()` 返回清单副本；nil、非法名称或完全相同的名称在登记时 panic，仅大小写不同的名称视为不同任务。闭包、方法值和泛型函数通过 `cronx.HandlerRegistry.MustRegister(handler, name)` 显式命名；函数重命名会影响自动名称的配置绑定和分布式锁标识，需要兼容时显式固定原名称。

模板默认不启用分布式锁，构造 Cron 不要求已注册 Redis。同一个 `Scheduler` 实例内，同名任务在上一轮 Handler 实际返回前不会重入，后续重叠触发直接跳过、不排队；即使任务已超时，也要等待 Handler 返回后才释放执行位置。不同任务可以并行，该防重入约束不跨进程或调度器实例。

需要跨实例互斥时，取消整个 `lock` 段的注释，复用 `MustInit` 中已声明并初始化的 `demo` Redis（连接配置为 `redis.demo.yaml`），锁前缀为 `pixiu-demo:cron:`，TTL 为 30 秒、每 10 秒续租。多实例须连接相同 Redis、DB，并使用相同前缀和稳定任务名；不同应用或环境使用不同前缀。启用该段时，Redis 实例必须在构造 Cron 前初始化。

模板的 HTTP、Cron 没有独立 shutdownTimeout/drainTimeout，统一由管理器提供 30 秒。心跳任务自身 timeout 仍为 2 秒；启用锁的任务必须有正 timeout。Cancel 只协作取消，不强杀业务或提前解锁。任务结果仍由 CronJobResult 等日志记录。

## 日志与标准输出

使用模板中的 `appName: pixiu-demo` 和 `dir: ./log` 时，日志位于应用启动目录下：

| 路径 | 内容 |
| --- | --- |
| `log/pixiu-demo/biz/debug.log` | 默认 Logger 的 Debug 日志 |
| `log/pixiu-demo/biz/info.log` | 默认 Logger 的 Info 日志 |
| `log/pixiu-demo/biz/wf.log` | 默认 Logger 的 Warn、Error、Fatal 日志 |
| `log/pixiu-demo/biz/http.*.log` | HTTP 命名 Logger 的分级日志，访问日志写入 `http.info.log` |
| `log/pixiu-demo/biz/cron.*.log` | Cron 命名 Logger 的分级日志，心跳和成功结果写入 `cron.info.log` |
| `log/pixiu-demo/std/stdout.log` | stdout 原始输出 |
| `log/pixiu-demo/std/stderr.log` | stderr 原始输出 |

修改应用名或日志目录后，路径随配置变化。业务日志按模板使用小时轮转，每个分流文件的保留上限为 48 个实际文件（包含当前时段），整点清理完成前可能暂时超限；debug/test 输出 Debug 及以上，release 输出 Info 及以上。

模板的 `log.names` 包含 `http` 和 `cron`；已有 `conf/app.yaml` 时可在该列表加入 `cron` 以启用独立任务日志文件，未声明时 Cron 日志回退到默认 Logger。

`log.hookStd` 默认开启，模板也设置为 true，stdout/stderr 会重定向到上述 `std/` 文件，持续至进程退出。这两个原始输出文件不轮转。该能力仅支持 Linux/macOS；其他平台或需要保留终端标准输出时，在 `conf/app.yaml` 的 `log` 段设置 `hookStd: false`，业务日志仍写入 `biz/` 文件。

`gin.yaml` 模板将 `logRequestInfo`、`logResponseInfo` 都设为 true，完整记录请求和响应详情；文档请求由底层库自动跳过访问日志和详情采集。按需关闭对应开关可停止请求或响应详情采集。

## 接口与文档

| 方法 | 路径 | 行为 |
| --- | --- | --- |
| GET | `/api/app/ping` | 可选参数 `name`，默认 `world`，最多 32 个字符 |
| POST | `/api/app/nats/publish` | JSON 必填 `messageIDPrefix`，发布一组 Core 和 JetStream 示例消息并等待 PubAck |
| POST | `/api/app/aliyun/oss/upload` | multipart 必填 `file`、`key`，上传单个文件 |
| POST | `/api/app/elasticsearch/synonyms-validate` | JSON 必填 `synonymsPath`，创建并清理临时索引以验证节点词库 |
| POST | `/api/app/elasticsearch/synonyms-reload` | JSON 必填 `index`，重载单个实际索引的搜索分析器并清理 request cache |
| GET | `/api/app/aliyun/es/synonyms-dicts-list` | 无需参数，查询默认阿里云 ES 实例的全部同义词文件 |
| POST | `/api/app/aliyun/es/synonyms-dicts-update` | JSON 必填 `clientToken`、非空 `dicts`，提交完整目标清单 |
| GET | `/api/app/aliyun/es/describe-instance` | 无需参数，查询默认阿里云 ES 实例详情 |
| GET | `/api/admin/ping` | 返回固定问候 `hello admin` |

```bash
curl -sS 'http://127.0.0.1:8888/api/app/ping?name=demo'
```

```json
{"code":0,"msg":"成功","data":{"message":"hello demo"}}
```

上传文件时使用已初始化的默认 OSS 实例，目标 Bucket 来自 `conf/aliyunossx.demo.yaml`：

```bash
curl -sS 'http://127.0.0.1:8888/api/app/aliyun/oss/upload' \
  -F 'file=@/path/to/demo.txt' \
  -F 'key=uploads/demo.txt'
```

`file` 只接收一个文件，本 demo 不额外设置固定的文件或请求正文大小上限。`key` 不能为空或全为空白，按原值传给 `ossx.PutObject`；同名对象遵循 OSS 的覆盖和版本控制规则。文件超过 1 MiB 时使用临时文件，上传结束后关闭并清理；向 OSS 传递文件的 Content-Type，缺失时使用 `application/octet-stream`。

成功响应的 `data` 包含 `key`、`etag`、`requestId`、`versionId`；未开启 Bucket 版本控制时 `versionId` 可为空。参数不合法时返回业务码 `10001`，OSS 上传失败时返回 `400`。请求取消或超时会传递到 OSS 调用；上传失败不自动重试，超时不保证远端未写入。

验证同义词词库时，使用 `conf/elasticsearch.demo.yaml` 配置的默认 ES 数据面客户端，调用 Pixiu 的 `biz/elasticsearchservice` 包中 `(*Service).ValidateSynonyms` 方法：

```bash
curl -sS 'http://127.0.0.1:8888/api/app/elasticsearch/synonyms-validate' \
  -H 'Content-Type: application/json' \
  --data-binary '{"synonymsPath":"analysis/synonyms.txt"}'
```

`synonymsPath` 是 ES 节点上的 Solr 格式词库路径，不是 Go 进程本机文件或下载链接；不能为空白或含空字符，按原值传递。默认 ES 客户端须具备创建、删除临时索引的权限。验证使用固定的 `standard → lowercase → synonym_graph` 分析链，严格加载词库，只有创建及分片均确认才算通过；确认创建后以独立的 5 秒期限清理索引，方法等待清理结束后返回。

```json
{"code":0,"msg":"成功","data":{"index":"pixiu-synonym-validation-0123456789abcdef0123456789abcdef","valid":true,"stage":"complete","cleanup":"succeeded","mayExist":false}}
```

`valid` 仅证明固定分析链在执行节点可加载词库，不证明全节点分发一致或业务召回效果。参数不合法返回业务码 `10001`，创建或清理失败均返回 `400`，省略 `data`，不暴露底层错误详情。失败日志保留 `index`、`valid`、`stage`、`cleanup`、`mayExist` 和错误概要；`mayExist=true` 表示可能遗留索引，需按日志中的 `index` 人工核查。即使 `valid=true`，清理失败仍返回服务错误。

请求取消或超时传递到创建操作；确认创建后的清理不受请求取消影响。创建结果不确定时不会自动删除，不自动重试，避免误删或掩盖不确定结果。

重载同义词搜索分析器时，使用同一个默认 ES 数据面客户端，调用 Pixiu 的 `infra/elasticsearchx` 包中 `(*Client).ReloadSearchAnalyzers` 方法：

```bash
curl -sS 'http://127.0.0.1:8888/api/app/elasticsearch/synonyms-reload' \
  -H 'Content-Type: application/json' \
  --data-binary '{"index":"products"}'
```

`index` 须为单个实际索引名，符合 ES 小写、最长 255 字节等命名约束；拒绝通配符、逗号列表、`_all`、路径和日期数学表达式，不解析别名或数据流。调用前须将同义词文件更新到所有数据节点（包括未持有该索引分片的节点），`synonym` 或 `synonym_graph` 过滤器设置 `updateable: true`，且仅用于搜索分析器。启用安全功能时客户端须有目标索引的 `manage` 权限；本接口不负责上传或分发词库。

```json
{"code":0,"msg":"成功","data":{"shards":{"total":1,"successful":1,"failed":0},"reloadDetails":[{"index":"products","reloadedAnalyzers":["search_synonyms"],"reloadedNodeIds":["node-a"]}],"cacheShards":{"total":2,"successful":2,"failed":0}}}
```

先重载搜索分析器，再仅清理同一索引的 request cache，以避免继续命中旧分析器产生的结果缓存。两次请求共用本次请求的 Context，取消或超时均传递到下游，不自动重试或回滚。`shards` 为重载按节点统计的执行计数，`cacheShards` 为缓存清理的分片计数，两者无需相等。`code=0` 仅表示两步请求均成功；空 `reloadDetails` 或空 `reloadedAnalyzers` 不代表同义词已更新，调用方须检查结果中是否包含预期分析器。

索引名参数错误返回业务码 `10001`；重载或清缓存失败均返回 `400`，省略 `data`，不暴露底层错误详情。失败日志保留 `index`、`reloadShards`、`reloadDetails`、`cacheShards` 与错误概要；清缓存失败时，重载可能已经生效。`cacheShards` 为 null 只表示未取得有效清理响应，不保证清理请求未发送。

查询同义词词典时使用默认阿里云 ES 管理客户端，目标实例来自 `conf/aliyunesx.demo.yaml` 的 `instanceID`：

```bash
curl -sS 'http://127.0.0.1:8888/api/app/aliyun/es/synonyms-dicts-list'
```

```json
{"code":0,"msg":"成功","data":{"requestId":"es-request-id","dicts":[{"name":"synonyms.txt","fileSize":1024,"sourceType":"OSS","type":"SYNONYMS","downloadUrl":"https://example.com/synonyms.txt"}]}}
```

词典顺序和字段按管理 API 返回，空清单返回 `dicts: []`；管理端清单不表示节点已分发或加载词典。查询失败返回业务码 `400`，省略 `data`，不暴露 SDK 错误详情；请求取消或超时传递到下游，不自动重试。

`downloadUrl` 按原值返回，可能包含短期签名。模板默认记录完整响应正文，使用该接口时建议在 `conf/gin.yaml` 中设置 `logResponseInfo: false`，避免签名链接进入访问日志。

更新同义词词典时，`dicts` 必须包含全部目标文件，遗漏已有文件会删除它，空清单不可提交。可先查询现有清单，再以 `sourceType: ORIGIN` 保留已有文件，以 `sourceType: OSS` 从指定 `bucket`、`key` 新增或替换文件。下面样例假设目标清单仅有 `keep.txt` 和 `current.txt`：

```bash
curl -sS 'http://127.0.0.1:8888/api/app/aliyun/es/synonyms-dicts-update' \
  -H 'Content-Type: application/json' \
  --data-binary '{
    "clientToken": "release-1",
    "dicts": [
      {"name": "keep.txt", "sourceType": "ORIGIN"},
      {"name": "current.txt", "sourceType": "OSS", "bucket": "dict-bucket", "key": "release/词库.txt"}
    ]
  }'
```

`clientToken` 不能为空白，最多 64 个 ASCII 字符；同一次更新重试须复用它，不同更新须更换。文件名限英文字母、数字、下划线及 `.txt` 后缀，总长最多 30 个字符，且不能重复；OSS 来源要求非空白 `bucket` 和非空 `key`，按原值传给管理 API。

```json
{"code":0,"msg":"成功","data":{"requestId":"es-request-id","dicts":[{"name":"keep.txt","fileSize":10,"sourceType":"ORIGIN","type":"SYNONYMS"},{"name":"current.txt","fileSize":20,"sourceType":"OSS","type":"SYNONYMS"}]}}
```

更新成功仅表示管理 API 成功，不保证词典已分发、索引已加载或实例不会重启。参数不合法返回业务码 `10001`，下游失败返回 `400`，不暴露 SDK 错误详情；请求取消或超时传递到下游，不自动重试，超时不保证远端未受理。

本样例直接提交完整清单，不提供更新并发控制，也不会自动读取或合并旧清单、比较版本。实际业务调用时由调用方协调同一实例的更新，避免完整清单相互覆盖。

查询默认阿里云 ES 实例详情：

```bash
curl -sS 'http://127.0.0.1:8888/api/app/aliyun/es/describe-instance'
```

```json
{"code":0,"msg":"成功","data":{"requestId":"es-request-id","instanceId":"es-instance-id","status":"active","version":"8.17"}}
```

目标实例同样来自 `conf/aliyunesx.demo.yaml` 的 `instanceID`；返回管理 API 的实例 ID、原始状态和 ES 版本，`active` 不表示词典已分发或加载。查询失败返回业务码 `400`，省略 `data`，不暴露 SDK 错误详情；请求取消或超时传递到下游，不自动重试。

业务响应统一使用 HTTP 200，客户端应检查 `code`：成功为 `0`，参数错误为 `10001`，通用服务错误为 `400`。错误响应省略 `data`；未知错误码的文案为“未知错误”。未登记路径保持 HTTP 404，已登记路径的方法不匹配返回 HTTP 405。

文档格式为 OpenAPI 2.0（Swagger 2.0）。`/docs` 展示 App 文档，顶部可切换 Admin；也可访问 `/docs/app`、`/docs/admin` 及 `/openapi/app.json`、`/openapi/admin.json`。静态资源位于 `/docs/assets/`；旧 `/elements` 及其子路径返回 404，不提供重定向。App 文档包含 Ping、NATS 消息发布、OSS 上传、ES 同义词词库验证与搜索分析器重载，以及阿里云 ES 同义词词典查询与更新、实例详情查询；Admin 文档包含 Admin Ping。

新增接口时，在 Handler 注释中使用 `@x-module-app true` 或 `@x-module-admin true` 指定文档归属，生成命令通过 `--parseExtension` 筛选接口；未标记的接口不会收录。`@Tags` 只控制模块内的业务分组，不决定顶部模块导航。生成规则见 [`openapi/placeholder.go`](openapi/placeholder.go)，顶部模块名称和标题由 [`bootstrap/start.go`](bootstrap/start.go) 中的 `OpenAPIModule` 声明。

修改接口后同步 Handler 注释和请求/响应结构，再执行 `go generate ./openapi`。生成物被 Git 忽略，编译时通过 `main.go` 的文档包导入注册。

## 项目结构与开发约定

```text
main.go                    配置加载与资源收尾
bootstrap/                 基础设施注册、HTTP/Cron 装配及信号停机
conf.example/              可共享的配置模板
errorcode/                 统一业务码和文案
internal/route/            路由登记
internal/httphandler/      HTTP 参数校验与统一响应
internal/natshandler/      NATS 消息适配、Service 调用及注册清单
internal/natspublisher/    NATS 消息编码、实际发布与确认
internal/cron/             定时任务 Handler，当前为心跳示例
internal/service/aliyunoss/ 阿里云 OSS 上传业务
internal/service/aliyunes/ 阿里云 ES 词典管理及实例查询业务
internal/service/elasticsearch/ ES 数据面同义词词库验证与搜索分析器重载
internal/service/nats/  NATS 示例发布编排及消费业务
internal/model/            HTTP 请求/响应及 message 消息数据结构
internal/repository/       数据库查询及 GORM 默认主从会话入口
internal/database/         项目拥有的 Model 迁移清单
internal/infra/            共享基础设施客户端入口
openapi/                   文档元信息、生成命令及包入口
package/bin/               部署控制脚本及 supervise
test/                      控制脚本隔离测试
```

Handler 只负责参数解析、校验和根据 Service 返回数据构造响应；具体业务放入 `internal/service/`，按业务领域分包，通过构造函数创建服务并接收标准 `context.Context`。OSS 上传由 `internal/service/aliyunoss` 负责打开和关闭文件、设置 Content-Type 并调用默认 OSS 客户端；multipart 临时文件仍由 Handler 随请求结束清理。阿里云 ES 词典管理及实例查询由 `internal/service/aliyunes` 调用默认管理客户端，Handler 将结果转换为接口响应。

HTTP 和 NATS 是并列的业务入口：`httphandler → Service`、`natshandler → Service`。NATS 发布遵循 `Service → natspublisher → infra/nats`，数据库访问继续走 `Service → Repository`；`natspublisher` 与数据库 Repository 同属外部访问层。消息结构独立放在 `internal/model/message/`，各层按需引用；Service 不接收 Gin 或 NATS SDK 消息。示例沿用原始文本正文，不改为 JSON。

`nats.Service.PublishExamples` 决定发布顺序、消息内容和 messageID；`natspublisher.Publisher` 负责主题映射、正文编码、发送及同步/异步确认，不决定业务流程。`natshandler.Handler` 将 SDK 消息转换为业务数据并调用同一个领域的 Service，等待业务完成后返回，不自行 ACK/NAK/Term。

`bootstrap.newNATSRegistries` 在共享客户端和数据库迁移就绪后构造 Service 和 Handler，再调用 `natshandler.NewRegistries(handler)` 创建独立注册表。处理方法清单集中在 `natshandler/registry.go`，方法值使用现有的 `HandleTask`、`HandleBroadcast`、`HandleQueue` 名称，配置引用不变。新增处理方法时同步维护该清单和 YAML，bootstrap 无需逐项绑定。`infra/nats` 仅保留客户端入口和实例名，不反向导入 Handler 或 Service，避免循环依赖；Publisher 继续通过该入口取得默认客户端。当前示例消费链路持有启动时创建的 Service 和 Publisher，Publisher 保存客户端引用；也可在回调中按调用创建 Service。NATS 消费排空期间仍可查询默认客户端并继续发布，CloseAll 开始后查询仍 panic。HTTP/Cron 在共享资源关闭前已经退出。多实例场景可用 `NewService(natspublisher.NewPublisher(client))` 显式指定依赖；所有消费服务排空期间共享连接仍可用，但不保证消息交给已停止的消费者。客户端回收阶段不再执行业务或跨实例调用。

Service 按职责、依赖、状态和生命周期划分。同一领域内职责相关、依赖相同且没有独立状态或生命周期差异的操作，默认归入同一个 Service 结构体，方法可按功能分布在多个 `.go` 文件中；文件数量和接口数量不决定结构体数量。只有出现独立业务流程、明显不同的依赖或状态、不同的初始化或关闭职责时，再拆分多个 Service；共用一个客户端本身不足以决定合并。

`internal/service/aliyunes` 统一使用 `aliyunes.Service` 和 `aliyunes.NewService()`。`service.go` 定义结构体与构造函数，`synonyms.go` 实现 `(*Service).ListSynonymsDicts(ctx)` 和 `(*Service).UpdateSynonymsDicts(ctx, req)`，`instance.go` 实现 `(*Service).DescribeInstance(ctx)`。方法名明确操作对象；请求的 Context 通过方法参数传入，不保存在 Service 中，共享客户端仍由启动阶段管理。

`internal/service/elasticsearch` 使用默认 ES 数据面客户端调用 Pixiu 的 `elasticsearchservice` 验证同义词词库，并通过 `elasticsearchx.ReloadSearchAnalyzers` 重载搜索分析器及清理 request cache；Handler 负责参数校验、失败状态日志和响应转换。

数据库查询放入 `internal/repository/`，Repository 只能由 Service 调用，Handler 不直接访问数据库。沿调用链传递当前操作的 Context；每次查询通过 `DefaultSessions().MasterDB(ctx)` 或 `SlaveDB(ctx)` 选择会话，不缓存请求会话。未配置从库时由 Pixiu 回退到主库。

本项目是业务项目，不创建或维护用于包级使用说明的 `doc.go`；新增或重构业务包时，必要说明写入 README 和代码注释。Pixiu 作为可复用基础库，仍须为已实现的包提供 `doc.go` 用途说明和调用样例，该要求不延伸到本项目。

共享客户端由启动阶段初始化并统一关闭，业务代码不自行关闭连接池或修改客户端配置。NATS 消费者在所有依赖就绪后注册订阅，发布服务可提前构造复用，也可按调用创建。

模块 Logger 必须在 `bootstrap.MustInit` 成功返回后创建和保存，不要在包级变量或 `init()` 中调用并缓存 `logit.Default()`、`logit.With(...)`、`logit.Named(...)`。初始化前默认日志保持静默，提前取得的 Logger 不会随后续初始化自动切换。当前业务代码在 Handler 或 Cron 任务执行时调用日志便捷函数，没有提前缓存模块 Logger；包级 `cron.Registry` 只登记任务声明，不执行任务或取得 Logger。

应用收到信号后统一停收并共享 30 秒排空，超时取消剩余任务；main 随后始终回收资源栈，按 NATS → Elasticsearch → Redis → MySQL → 阿里云 ES → OSS → 日志顺序执行。`lifecycle.Stack` 零值整体预算 20 秒；需要覆盖时用 `resources, err := lifecycle.New(15 * time.Second)`，向初始化传 resources。全部回调共享该期限，不逐资源重置。到期不再启动后续关闭，也不保证日志已刷盘，main 随即退出。收尾 defer 早于初始化，初始化 panic 仍先有界回收再传播。

[`bootstrap.MustInit`](bootstrap/bootstrap.go) 在共享实例初始化完成后调用 `RunMySQLMigration`，严格读取 `conf/migrate.demo.yaml`，该文件必须存在。模板已设置 `enabled: true`、`timeout: 5m`；当前 [`internal/database.Tables()`](internal/database/models.go) 清单为空，不执行表结构迁移。添加项目业务 Model 后，下次启动会检查并迁移登记的表；需要关闭时，将运行配置中的 `enabled` 设为 false。

自动迁移只处理显式登记的项目业务表，没有跨实例迁移锁，部署时避免多实例同时迁移。迁移失败会中止启动并回收资源；DDL 失败或超时可能已部分生效，不能保证整批回滚。

## 构建、测试与打包

```bash
go test ./...
go test -race ./...
go build -o pixiu-demo .
bash test/control_test.sh
gofmt -l bootstrap internal errorcode main.go openapi_test.go openapi
```

联合验证相邻的 Pixiu 库时，还需执行其统一入口：

```bash
make -C ../pixiu test
make -C ../pixiu test-race
```

Pixiu 的每个入口均包含常规测试和带 `natsintegration` 标签的 NATS 配置注册集成测试两轮；单独执行其 `go test ./...` 不包含第二轮。Demo 的 NATS 测试仍由本仓库的 `go test ./...` 和 `go test -race ./...` 覆盖，无需额外标签。

Go 测试覆盖公开 Ping、OSS 上传、ES 同义词词库验证及清理失败、阿里云 ES 同义词词典查询与更新、实例详情查询、参数边界、请求取消、HTTP 404/405、请求日志作用域、文档核心契约和 HTTP 装配冒烟；另验证默认模板构造 Cron 不要求已注册 Redis，而 Cron 配置错误或显式启用锁时引用的 Redis 未初始化会阻止启动，以及 Unix 子进程中通过真实 Start 同时提供 HTTP 和执行心跳、SIGTERM 按序关闭后不再调度。混合启动测试不启用分布式锁以隔离 Redis；OSS、ES 数据面和阿里云 ES 测试使用本地 HTTP 模拟服务，无需启动外部基础设施。控制脚本测试使用模拟进程、端口和 supervise。测试和构建成功不代替真实基础设施连接验证。

NATS 装配测试位于 `bootstrap/nats_test.go`，调用实际的消费链路装配和注册表工厂，为两个客户端分别创建独立注册表，复用 Pixiu 相同版本的 `nats-server/v2`，在隔离测试进程内启动临时 JetStream 服务，读取共享注册模板并绑定实际 Handler，覆盖无整体期限的配置注册、Default 参数、两实例共享 Durable、Core 广播与 Queue、同步／异步 PubAck、消费 ACK、资源冲突及客户端收尾；接口测试覆盖参数校验、发布确认、同批次去重与统一错误响应。不读取本地 `conf/`，无需预先安装或启动 NATS 服务。

```bash
make                           # 构建并打包
make GENERATE_SWAGGER=1         # 先生成文档再构建和打包
```

`BINARY_NAME` 默认值为 `pixiu-demo`，可通过 `make BINARY_NAME=my-service` 覆盖。Makefile 的 `prepare` 会通过 `go env -w` 持久设置 `GO111MODULE=on` 和 `GOPROXY=https://goproxy.cn,direct`；如果已管理自己的 Go 环境，可直接使用上面的 Go 命令。

打包结果位于被忽略的 `output/`，包括 `bin/`、`conf.example/`、`log/` 和 `status/`。部署时在 `output/conf/` 准备运行配置，以 `output/` 为工作目录运行 `./bin/pixiu-demo`。

附带的 supervise 二进制适用于 Linux x86_64、glibc 环境；`control` 需要 Bash、`ps`、`awk` 和 `ss`。在该环境可执行 `./bin/control start|stop|restart|status`，监听端口变化时通过 `PORT_MAIN` 覆盖默认 `8888`。`drain`、`undrain` 只管理状态文件，当前服务不读取该标记。
