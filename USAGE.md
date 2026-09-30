# thinkgo-framework Usage

## 目的

本文面向在其他业务项目中工作的 AI Coding Agent / 开发者。

当项目依赖 `github.com/kk2land/thinkgo-framework`，并且代码里出现 `github.com/kk2land/thinkgo-framework/thinkgo` 时，优先参考本文判断框架已经提供了哪些能力，避免重复封装 HTTP、日志、Redis、数据库、锁、缓存或并发工具。

> Agent 应先阅读 [`AGENTS.md`](./AGENTS.md) 获取开发决策规则，再按需使用本文查询具体能力和 API。文档与源码不一致时，以当前源码为准。

## 一句话总结

`thinkgo-framework` 是一个围绕全局配置、生命周期、日志、HTTP/Gin、gRPC、WebSocket、数据库、Redis、锁、缓存和并发工具构建的轻量运行时框架。它不是 DI 容器，也不是完整业务脚手架；主要把常见服务端基础设施沉淀在 `thinkgo` 包里供业务直接复用。

## 快速识别

- `go.mod` 依赖 `github.com/kk2land/thinkgo-framework`
- 业务代码通常导入 `github.com/kk2land/thinkgo-framework/thinkgo`
- 标准运行目录包含 `app/config`、`app/runtime`、`app/bin`
- module 模式还包含 `app/{module}/config`、`app/{module}/runtime`、`app/{module}/bin`
- module 构建可通过 `-ldflags "-X github.com/kk2land/thinkgo-framework/thinkgo.ModuleName=xxx"` 注入模块名

## 运行时模型

### 1. Import 即完成环境装配

对应：`thinkgo/base.go`、`thinkgo/config.go`

导入 `thinkgo` 时 `init()` 会立即执行：

- 推导 `RootPath`、`AppPath`、`RuntimePath`
- 如果 `_TK_SYSTEM_ENV` 指向有效文件，先加载该 env 文件
- 再加载项目根目录 `.env`
- 根据 `_TK_AppStatus` 确定环境并加载 TOML 配置
- 初始化全局 `Config`、`Logger`
- 注册框架 start/shutdown hook
- 初始化 Redis 运行时

因此业务项目导入 `thinkgo` 时默认要求真实的 `app/`、`config/`、`runtime/` 目录存在；测试、一次性脚本和临时工具需要特别注意。可通过 `_TK_RootPath` 显式指定项目根目录。

常用环境变量：

- `_TK_RootPath`：项目根目录
- `_TK_AppStatus`：运行环境；非 `prod*` 环境默认开启 debug
- `_TK_AppDebug`：显式覆盖 debug，`1` 为开启
- `_TK_LogConsole`：`1` 时日志输出到控制台
- `_TK_Command`：覆盖 `CommandName`
- `_TK_SYSTEM_ENV`：额外系统级 env 文件，在项目 `.env` 前加载

### 2. 全局运行时变量

对应：`thinkgo/base.go`

常见变量：`ModuleName`、`InModule`、`RootPath`、`AppPath`、`RuntimePath`、`CommandName`、`AppName`、`AppStatus`、`AppDebug`、`Hostname`、`Pid`、`Config`、`Logger`。

### 3. 配置加载规则

对应：`thinkgo/config.go`、`app.toml`

配置格式为 TOML。文件按以下顺序叠加：

1. `app/config/app.toml`
2. `app/config/app_{_TK_AppStatus}.toml`
3. `app/{module}/config/app.toml`
4. `app/{module}/config/app_{_TK_AppStatus}.toml`

第 3、4 项只在 module 模式加载。后加载内容覆盖/补充前面的配置。

内置模板宏：`{{ModuleName}}`、`{{AppPath}}`、`{{AppStatus}}`、`{{CommandName}}`、`{{Hostname}}`；其他 `{{...}}` 按环境变量读取。

路径辅助：

- `GetAppConfigPath(file)`：获取公共 `app/config` 下文件；非 module 模式与当前 config 目录相同
- `GetConfigPath(file)`：获取当前应用的 config 目录，module 模式指向 `app/{module}/config`

DB / Redis 均支持按 `CommandName` 使用 `cmd_options` 覆盖部分运行参数。

### 4. 生命周期与进程管理

对应：`thinkgo/base.go`

核心入口：`AddStartHook()`、`AddShutdownHook()`、`RemoveShutdownHook()`、`CallStartHooks()`、`CallShutdownHooks()`、`WritePidFile()`。

`StartHttpServer()` 与 `ListenShutdownSignals()` 会自动触发 start/shutdown hooks，并在退出路径关闭 DB、Redis。PID 写入 `app/runtime/pid/{AppName}.pid` 或 module 对应的 `RuntimePath/pid`。

注意：start hooks 触发后再调用 `AddStartHook()` 会立即执行该 hook；shutdown 已完成后再 `AddShutdownHook()` 会 panic。

### 5. 命令模式

对应：`thinkgo/base.go`、`hook_pull.sh`

- `CommandRegister(name, func())`
- `CommandRun()`

典型模式是主二进制负责命令分发，wrapper 通过 `_TK_Command` 指定逻辑命令名。看到 `src/cmd` 或 `src/{module}/cmd` 时优先检查这一机制。

## 框架提供的能力

### 1. 配置与环境管理

已提供 TOML、多环境覆盖、module 叠加、env、模板宏、配置路径辅助，以及 DB/Redis 的命令维度参数覆盖。不要在业务层另建平行配置系统。

### 2. 日志与运维告警

对应：`thinkgo/logger.go`、`thinkgo/base.go`

日志基于 `logrus` 封装，核心接口为 `FieldLogger`。提供全局 `Logger`、字段日志、请求级 HTTP/gRPC logger、debug 联动和文件日志。

`OpsAlarm()`、`OpsAlarmDirect()`、`OpsAlarmWithGroup()` 通过 syslog 发送运维告警；普通 `OpsAlarm` 在框架启动后走内部 `GoQueue` 异步发送，direct 模式直接发送。

### 3. HTTP 服务

对应：`thinkgo/http_server.go`

核心入口：`HttpEngine()`、`HttpRouter()`、`HttpRouterWithPath()`、`HttpStartServer()`、`HttpStartServerWithConfig()`、`HttpSetErrorHandler()`、`HttpPort()`。

能力包括 Gin Engine、请求级日志、request ID、panic recovery、统一错误处理、路由组、`endless` 平滑重启，以及 `WithGrpc=true` 时同端口 h2c + gRPC。

底层还提供 `StartHttpServer(port, handler, options)`，可在不直接使用 Gin 启动入口的场景复用框架生命周期和平滑重启能力。

### 4. gRPC 服务

对应：`thinkgo/grpc_server.go`

核心入口：`GrpcOnBeforeCreate()`、`GrpcOnAfterCreate()`、`GrpcCreateServer()`、`GrpcLogger()`。

提供 unary/stream panic recovery、request ID 与日志关联、HTTP 同端口 h2c，以及 server 创建前后的扩展 hook。业务 protobuf service 通常通过 `GrpcOnAfterCreate` 注册。

### 5. WebSocket

对应：`thinkgo/http_server.go`、`thinkgo/http_ws.go`

核心：`HttpWebsocketUpgrade()`、`NewHttpWsRouter[T]()`、`HttpWsConn[T]`、`HttpWsConnGroup[T]`。

提供泛型消息序列化/反序列化、单连接写队列、ping/pong、读超时、key 绑定单连接及旧连接替换、分组广播、连接上下文和 close reason。`HttpWebsocketUpgrade()` 默认 `CheckOrigin` 返回 true，外网业务应自行评估 Origin/鉴权要求。

### 6. 数据库 / xorm

对应：`thinkgo/database.go`、`thinkgo/database_logger.go`、`thinkgo/database_values.go`

核心：`DBDefault()`、`DBDefaultOrPanic()`、`DB(name)`、`DBOrPanic(name)`、`DBCloseAll()`；`DBInstance` 嵌入 `xorm.EngineInterface`。

提供多实例、默认实例、单库/读写集群、连接池配置、日志、`ExecWithBackoff()`、`DBErrRetry()`、`DBErrDuplicate()`。

`Value2Interface()` 提供 xorm 字段到数据库值的转换辅助，处理 `convert.Conversion`、时间、`sql.NullFloat64`、`big.Float`、`driver.Valuer`、JSON text/blob、slice/map 等。除非正在处理底层字段转换/批量构建，不要把它当普通业务 ORM API 使用。

### 7. Redis / go-redis

对应：`thinkgo/redis.go`、`thinkgo/redis_client.go`、`thinkgo/redis_client_extra.go`

核心：`RedisDefault()`、`RedisDefaultOrPanic()`、`Redis(name)`、`RedisOrPanic(name)`、`RedisCloseAll()`、`RedisClient`。

提供多实例、key prefix、命令维度 timeout/pool/retry 配置、`ExecWithBackoff()`、`Wait()`、错误判断、`Raw()`，以及 `SetExf()`、`IncrEx()`、`IncrByEx()` 等 Lua 辅助命令。

`RedisClient` 已代理大量常用 go-redis 命令。使用 `Raw()` 时框架不会替你自动补业务代码自行构造的 key，必须理解 `Prefix()` / `Prefixes()` 约定。

### 8. 分布式锁 / 本地锁

对应：`thinkgo/key_lock.go`

提供 `KeyLockMem*`、`KeyLockRedis*`、`KeyLockMySQL*` 三类 key 锁，以及阻塞、try、timeout 和 `KeyLockErrLockedFail()` 判断。已有语义能满足时不要自行用 `SETNX` 等重新实现。

### 9. KeyStore 业务缓存

对应：`thinkgo/key_store.go`、`thinkgo/key_store_backend.go`

核心：`NewKeyStore()`、`NewKeyStoreWithCache()`、`KeyStoreBackend`、`KeyStoreBackendRedis`、`NewKeyStoreBackendAsync()`。

提供本地 fastcache、TTL、backend 回源、同步/异步 backend 写入、Redis backend、`Load()` / `LoadOrCreate()` / `Store()` 和更新锁。注意 `LoadOrCreate()` 的创建结果只进入本地缓存，不会自动写回 backend。

### 10. Leader 选举

对应：`thinkgo/leader.go`

`Leader` / `LeaderRedis` / `NewLeaderRedis()` 提供 Redis leader 竞争、续租、状态回调、上下线和框架生命周期接入，适合定时任务单实例执行等场景。

### 11. GatewayClient

对应：`thinkgo/gateway_client.go`

这是 PHP WorkerMan Gateway/GatewayWorker 客户端，不是通用 API Gateway SDK。支持 register 地址发现 gateway、长连接、协议发送、`SendToGroup()`、`SendToUid()` 和周期性刷新节点。

### 12. SQL 批量构建器

对应：`thinkgo/insert_builder.go`、`thinkgo/insert_on_duplicate_builder.go`

提供 `InsertBuilder`、`InsertOnDuplicateBuilder`，覆盖 MySQL/PostgreSQL 批量插入、MySQL insert ignore/replace、PostgreSQL conflict do nothing 和 upsert。

### 13. 并发与缓存基础工具

对应：`concurrent_map.go`、`concurrent_map_lru.go`、`sync_map.go`、`util_go_queue.go`、`timeout_once.go`、`minutes_ticker.go`、`util_send_close.go`、`lru/lru.go`

包括：

- `CMap[K,V]`：分片并发 map
- `CMapLRU[T]`：TTL + LRU 分片缓存
- `SyncMap[T]`：支持 `LoadOrCreate` 的泛型 `sync.Map` 封装
- `GoQueue`：单 goroutine 串行消费队列
- `TimeoutOnce`：带等待超时的 once
- `MinutesTicker`：按自然分钟对齐 ticker
- `SendClose`：关闭后忽略 send 的并发控制
- `lru.Cache`：基础 LRU

### 14. 类型转换与动态数据

对应：`thinkgo/jmap.go`、`thinkgo/conv_*.go`

提供 `JMap`、`JString` / `JInt` / `JFloat` / `JBool`、`ConvDuration`、`ConvDecimal`、`ConvInt64`、`ConvBool`、`ConvInts`、`ConvUInt32s`，主要用于配置、动态 JSON 和 DB/JSON/TOML 转换。

### 15. Backoff、错误和通用工具

对应：`thinkgo/backoff.go`、`thinkgo/util*.go`

常用能力：

- `BackoffPolicyDefault()` / `BackoffPolicyForever()`
- `SafeGo()` / `SafeGo1()`：goroutine panic 捕获、日志/告警和可选继续 panic
- `Recover2Error()`：recover 值转 error
- `ErrIsTimeout()`：识别 `context.DeadlineExceeded` 和实现 `Timeout()` 的错误
- `ErrIsBrokenPipe()`：识别常见连接中断
- `LogErrAndPanic()`
- `SafeSendChannel()`
- `HttpGet()` / `HttpPost()` / `HttpPostForm()` / `HttpBodyWithTries()`
- `ParseForm()` / `ClientIp()`
- `ReadFile()` / `WriteFile()` / `IsDir()` / `IsFile()`
- `Stack()`
- `Md5()` / `Crc32()` / `RandString()` / `NginxHash()`
- `TruncateFloat64()` / `TruncateBigFloat()`
- `BytesBuffer` / `BytesBuffer1024`：可复用 bytes buffer pool
- `RandSource` / `RandSourceDefault`：随机源池
- `UnsafeStrToBytes()` / `UnsafeBytesToStr()`：零拷贝 unsafe 转换，仅在明确理解生命周期时使用
- `SortMap()`、`JsonBytesTrimQuote()` 等轻量辅助

Agent 不应为了这些已有的低层工具再引入平行 helper；但 unsafe 转换、哈希/随机等能力也不应在安全敏感场景中被误当作密码学 API。

## 工程脚手架与配套脚本

### `app_init.sh`

创建 `app/` 或 `app/{module}/` 下的 `config`、`bin`、`runtime/log`、`runtime/pid`，并输出 `.gitignore` 建议。

### `hook_pull.sh`

编译 `src/main/*.go` / `src/{module}/main/*.go`，module 构建时通过 ldflags 注入 `thinkgo.ModuleName`，并为 `src/cmd/*.go` / `src/{module}/cmd/*.go` 生成命令 wrapper。

### `doc.sh`

安装/启动本地 `godoc` 包文档服务。

## Agent / 开发者使用约定

1. 遇到 HTTP、日志、WebSocket、DB/Redis、锁、缓存、leader、并发工具等需求，先检查框架。
2. 优先检查业务入口、`app/config/*.toml`、module config、构建脚本和已有框架调用方式。
3. DB 查询语义主要参考 xorm，复杂 Redis 命令主要参考 go-redis；但实例、配置、日志、重试、prefix 和生命周期继续沿用 ThinkGo。
4. 牢记 import 即初始化。测试/脚本必须准备目录、配置和环境，或显式设置 `_TK_RootPath`。
5. 关键行为不要只根据函数名推断，直接查看对应源码。

## 框架边界

当前源码没有内建完整提供：依赖注入容器、配置中心/热更新、消息队列抽象、Metrics/distributed tracing、数据库 migration、认证鉴权体系、统一 DTO validation。

业务需要这些能力时，先检查业务项目已有实现，再判断是否应扩展框架。