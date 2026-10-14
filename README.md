# pixiu-demo

基于 [Pixiu](https://github.com/bpcoder16/pixiu)、Gin 和 GORM 的 Go HTTP 服务基础骨架，用于演示项目分层、统一响应、基础设施装配、日志和资源生命周期。

当前提供两个公开 Ping 样例、MySQL 主从会话入口、Redis、Elasticsearch、阿里云 OSS、阿里云 ES、NATS 默认实例，以及 App/Admin 分模块的 OpenAPI 文档。项目没有身份认证、令牌处理或云服务业务接口；接入实际业务时自行实现相应规则。

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
| [`mysql.demo.yaml`](conf.example/mysql.demo.yaml) | 默认 MySQL 主库、可选从库及连接池 |
| [`migrate.demo.yaml`](conf.example/migrate.demo.yaml) | Model 自动迁移开关和整体期限；模板已开启，当前表清单为空 |
| [`redis.demo.yaml`](conf.example/redis.demo.yaml) | 默认 Redis 连接、重试和连接池 |
| [`elasticsearch.demo.yaml`](conf.example/elasticsearch.demo.yaml) | 默认 ES 数据面实例；版本须与实际服务端一致 |
| [`aliyunossx.demo.yaml`](conf.example/aliyunossx.demo.yaml) | 默认阿里云 OSS 实例，固定 Region、Bucket 和访问凭据 |
| [`aliyunesx.demo.yaml`](conf.example/aliyunesx.demo.yaml) | 默认阿里云 ES 管理 API 实例，固定 Region、InstanceID 和访问凭据 |
| [`nats.demo.yaml`](conf.example/nats.demo.yaml) | 默认 NATS 连接及排空期限 |

基础设施实例名统一为 `demo`，由 `bootstrap.MustInit` 注册；配置文件中无需再次指定实例名或默认标识。启动会初始化全部六类基础设施，即使只访问 Ping，也需完成初始化。阿里云 OSS 和阿里云 ES 初始化只校验配置并构造客户端，不联网验活；实际访问时再验证凭据、权限和资源可用性。

模板使用 `debug` 模式，启动前须生成接口文档：

```bash
go generate ./openapi
go run .
```

入口固定加载 `./conf/app.yaml`，运行时工作目录须包含 `conf/`。`release` 模式关闭文档路由，不要求生成文档；`debug`、`test` 模式在文档未生成时拒绝启动。

## 日志与标准输出

使用模板中的 `appName: pixiu-demo` 和 `dir: ./log` 时，日志位于应用启动目录下：

| 路径 | 内容 |
| --- | --- |
| `log/pixiu-demo/biz/debug.log` | 默认 Logger 的 Debug 日志 |
| `log/pixiu-demo/biz/info.log` | 默认 Logger 的 Info 日志 |
| `log/pixiu-demo/biz/wf.log` | 默认 Logger 的 Warn、Error、Fatal 日志 |
| `log/pixiu-demo/biz/http.*.log` | HTTP 命名 Logger 的分级日志，访问日志写入 `http.info.log` |
| `log/pixiu-demo/std/stdout.log` | stdout 原始输出 |
| `log/pixiu-demo/std/stderr.log` | stderr 原始输出 |

修改应用名或日志目录后，路径随配置变化。业务日志按模板使用小时轮转，每个分流文件的保留上限为 48 个实际文件（包含当前时段），整点清理完成前可能暂时超限；debug/test 输出 Debug 及以上，release 输出 Info 及以上。

`log.hookStd` 默认开启，模板也设置为 true，stdout/stderr 会重定向到上述 `std/` 文件，持续至进程退出。这两个原始输出文件不轮转。该能力仅支持 Linux/macOS；其他平台或需要保留终端标准输出时，在 `conf/app.yaml` 的 `log` 段设置 `hookStd: false`，业务日志仍写入 `biz/` 文件。

`gin.yaml` 模板将 `logRequestInfo`、`logResponseInfo` 都设为 true，完整记录请求和响应详情。访问 Elements 时，建议将 `logResponseInfo` 设为 false，避免把完整文档 UI 静态资源写入日志；按需关闭 `logRequestInfo` 可停止请求详情采集。

## 接口与文档

| 方法 | 路径 | 行为 |
| --- | --- | --- |
| GET | `/api/app/ping` | 可选参数 `name`，默认 `world`，最多 32 个字符 |
| GET | `/api/admin/ping` | 返回固定问候 `hello admin` |

```bash
curl -sS 'http://127.0.0.1:8888/api/app/ping?name=demo'
```

```json
{"code":0,"msg":"成功","data":{"message":"hello demo"}}
```

业务响应统一使用 HTTP 200，客户端应检查 `code`：成功为 `0`，参数错误为 `10001`，通用服务错误为 `400`。错误响应省略 `data`；未知错误码的文案为“未知错误”。未登记路径保持 HTTP 404，已登记路径的方法不匹配返回 HTTP 405。

文档格式为 OpenAPI 2.0（Swagger 2.0）。`/elements` 展示 App 文档，顶部可切换 Admin；也可访问 `/elements/app`、`/elements/admin` 及 `/openapi/app.json`、`/openapi/admin.json`。两个模块均只包含各自的公开 Ping。

新增接口时，在 Handler 注释中使用 `@x-module-app true` 或 `@x-module-admin true` 指定文档归属，生成命令通过 `--parseExtension` 筛选接口；未标记的接口不会收录。`@Tags` 只控制模块内的业务分组，不决定顶部模块导航。生成规则见 [`openapi/placeholder.go`](openapi/placeholder.go)，顶部模块名称和标题由 [`bootstrap/start.go`](bootstrap/start.go) 中的 `OpenAPIModule` 声明。

修改接口后同步 Handler 注释和请求/响应结构，再执行 `go generate ./openapi`。生成物被 Git 忽略，编译时通过 `main.go` 的文档包导入注册。

## 项目结构与开发约定

```text
main.go                    配置加载与资源收尾
bootstrap/                 基础设施注册、HTTP 启动及信号停机
conf.example/              可共享的配置模板
errorcode/                 统一业务码和文案
internal/route/            路由登记
internal/handler/          参数绑定与统一响应
internal/model/            请求和响应结构
internal/repository/       GORM 默认主从会话入口
internal/database/         项目拥有的 Model 迁移清单
internal/infra/            MySQL、Redis、Elasticsearch、阿里云 OSS/ES、NATS 入口
openapi/                   文档元信息、生成命令及包入口
package/bin/               部署控制脚本及 supervise
test/                      控制脚本隔离测试
```

业务逻辑可按需新增到 `internal/service/`，数据库访问放入 `internal/repository/`。沿调用链传递当前操作的 Context；每次查询通过 `DefaultSessions().MasterDB(ctx)` 或 `SlaveDB(ctx)` 选择会话，不缓存请求会话。未配置从库时由 Pixiu 回退到主库。

共享客户端由启动阶段初始化并统一关闭，业务代码不自行关闭连接池或修改客户端配置。NATS 消费者应在启动时取得并持有客户端，在所有依赖就绪后注册订阅。

应用退出时先停止 HTTP 入口，再排空 NATS，随后关闭其他基础设施，最后关闭日志。初始化 panic 由 `main` 提前登记的资源栈收尾，并继续传播。

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

Go 测试覆盖公开 Ping、参数边界、HTTP 404/405、请求日志作用域、文档核心契约和 HTTP 装配冒烟，无需启动外部基础设施。控制脚本测试使用模拟进程、端口和 supervise。测试和构建成功不代替真实基础设施连接验证。

```bash
make                           # 构建并打包
make GENERATE_SWAGGER=1         # 先生成文档再构建和打包
```

`BINARY_NAME` 默认值为 `pixiu-demo`，可通过 `make BINARY_NAME=my-service` 覆盖。Makefile 的 `prepare` 会通过 `go env -w` 持久设置 `GO111MODULE=on` 和 `GOPROXY=https://goproxy.cn,direct`；如果已管理自己的 Go 环境，可直接使用上面的 Go 命令。

打包结果位于被忽略的 `output/`，包括 `bin/`、`conf.example/`、`log/` 和 `status/`。部署时在 `output/conf/` 准备运行配置，以 `output/` 为工作目录运行 `./bin/pixiu-demo`。

附带的 supervise 二进制适用于 Linux x86_64、glibc 环境；`control` 需要 Bash、`ps`、`awk` 和 `ss`。在该环境可执行 `./bin/control start|stop|restart|status`，监听端口变化时通过 `PORT_MAIN` 覆盖默认 `8888`。`drain`、`undrain` 只管理状态文件，当前服务不读取该标记。
