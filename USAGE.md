# thinkgo-framework Usage

## 目的

本文面向在其他业务项目中工作的 AI Coding Agent / 开发者。

当项目依赖 `github.com/kk2land/thinkgo-framework`，并且代码里出现 `github.com/kk2land/thinkgo-framework/thinkgo` 时，优先参考本文判断框架已经提供了哪些能力，避免重复封装 HTTP、日志、Redis、数据库、锁、缓存或并发工具。

> Agent 应先阅读 [`AGENTS.md`](./AGENTS.md) 获取开发决策规则，再按需使用本文查询具体能力和 API。

## 一句话总结

`thinkgo-framework` 是一个围绕全局配置、生命周期、日志、HTTP/Gin、gRPC、WebSocket、数据库、Redis、锁、缓存和并发工具构建的轻量运行时框架。它不是一个 DI 容器，也不是一个完整的业务脚手架平台；更多是把常见服务端基础设施沉淀在 `thinkgo` 包里，业务项目直接复用。

## 快速识别

- `go.mod` 通常会依赖 `github.com/kk2land/thinkgo-framework`
- 业务代码常见导入包是 `github.com/kk2land/thinkgo-framework/thinkgo`
- 项目通常会有 `app/config`、`app/runtime`、`app/bin`
- 如果项目启用了 module 模式，还会有 `app/{module}/config`、`app/{module}/runtime`、`app/{module}/bin`
- 编译脚本可能通过 `-ldflags "-X github.com/kk2land/thinkgo-framework/thinkgo.ModuleName=xxx"` 注入模块名

## 运行时模型

### 1. 包初始化即完成环境装配

对应文件：`thinkgo/base.go`、`thinkgo/config.go`

只要导入 `thinkgo`，框架的 `init()` 就会立即执行，完成这些动作：

- 推导 `RootPath`、`AppPath`、`RuntimePath`
- 读取项目根目录下的 `.env`
- 读取并合并配置文件
- 初始化全局 `Config`
- 初始化全局 `Logger`
- 初始化 Redis 日志适配器
- 注册默认的 start/shutdown hook

这意味着：

- 业务项目一旦导入 `thinkgo`，默认就依赖真实的 `app/` 目录结构和配置文件
- 如果在测试、一次性脚本、临时工具中直接导入 `thinkgo`，但目录结构不完整，`init()` 期间就可能 panic
- 可以通过环境变量 `_TK_RootPath` 指定根目录，避免按可执行文件路径倒推目录

### 2. 全局运行时变量

对应文件：`thinkgo/base.go`

业务项目里经常会直接使用这些全局变量：

- `ModuleName`：编译时注入的模块名
- `InModule`：是否启用 module 模式
- `RootPath`：项目根目录
- `AppPath`：当前应用目录，通常是 `app` 或 `app/{module}`
- `RuntimePath`：运行时目录，通常是 `app/runtime` 或 `app/{module}/runtime`
- `CommandName`：当前命令名，来自可执行文件名或 `_TK_Command`
- `AppName`：配置中的应用名
- `AppStatus`：运行环境标识，来自 `_TK_AppStatus`
- `AppDebug`：调试模式
- `Config`：当前应用配置
- `Logger`：全局日志对象

### 3. 配置加载规则

对应文件：`thinkgo/config.go`、`app.toml`

配置格式是 TOML，支持 `.env` 和模板宏。

配置文件加载顺序：

1. `app/config/app.toml`
2. `app/config/app_{_TK_AppStatus}.toml`
3. `app/{module}/config/app.toml`
4. `app/{module}/config/app_{_TK_AppStatus}.toml`

只有 module 模式下才会加载第 3、4 项。

支持的内置宏：

- `{{ModuleName}}`
- `{{AppPath}}`
- `{{AppStatus}}`
- `{{CommandName}}`
- `{{Hostname}}`

其他 `{{...}}` 会按环境变量读取。

### 4. 生命周期与进程管理

对应文件：`thinkgo/base.go`

框架提供统一的启动和退出钩子：

- `AddStartHook(func())`
- `AddShutdownHook(func(*sync.WaitGroup))`
- `RemoveShutdownHook(id)`
- `CallStartHooks()`
- `CallShutdownHooks()`

框架还会处理这些运行时动作：

- 写入 PID 文件到 `app/runtime/pid/{AppName}.pid`
- 在服务退出时自动关闭 DB 和 Redis 连接
- `StartHttpServer()` 基于 `endless` 支持 HTTP 服务平滑重启
- `ListenShutdownSignals()` 支持非 HTTP 进程监听 `SIGINT` / `SIGTERM`

### 5. 命令模式

对应文件：`thinkgo/base.go`、`hook_pull.sh`

框架内置命令注册/执行机制：

- `CommandRegister(name, func())`
- `CommandRun()`

典型模式是：

- 主二进制处理命令分发
- `hook_pull.sh` 为每个命令生成 shell wrapper
- wrapper 通过 `_TK_Command` 指定逻辑命令名

如果你在业务项目里看到 `src/cmd` 或 `src/{module}/cmd`，通常说明项目在使用这一套命令机制。

## 框架提供的能力

### 1. 配置与环境管理

对应文件：`thinkgo/config.go`、`app.toml`

已提供的能力：

- TOML 配置加载
- `.env` 自动加载
- 按环境覆盖配置
- 按 module 叠加配置
- 配置宏替换
- `CommandName` 维度的 DB / Redis 参数覆盖

适合场景：

- 一个仓库里有多个命令、多个模块、多个环境
- 需要根据命令名调整连接池、超时、重试参数

### 2. 日志

对应文件：`thinkgo/logger.go`

日志系统是基于 `logrus` 的二次封装，核心接口是 `FieldLogger`。

已提供的能力：

- 全局 `Logger`
- `Logger.With(k, v, ...)` 形式的字段日志
- 调试级别与 `AppDebug` 联动
- 非 TTY 环境下自动按天切分日志文件
- 默认日志目录是 `app/runtime/log`
- 日志格式包含时间、应用名、PID、级别和字段
- `HttpLogger(c)` / `GrpcLogger(ctx)` 可拿到请求级日志对象

补充说明：

- 如果不是终端运行，且没有显式设置 `_TK_LogConsole=1`，日志会写文件而不是直接输出到控制台
- `OpsAlarm()` / `OpsAlarmDirect()` / `OpsAlarmWithGroup()` 可通过 syslog 发运维告警

### 3. HTTP 服务

对应文件：`thinkgo/http_server.go`

HTTP 层基于 Gin，核心入口：

- `HttpEngine()`
- `HttpRouter()`
- `HttpRouterWithPath("/prefix")`
- `HttpStartServer()`
- `HttpStartServerWithConfig(...)`
- `HttpSetErrorHandler(...)`
- `HttpPort()`

已提供的能力：

- 统一创建 Gin 引擎
- 请求级日志对象注入
- panic 恢复与统一错误处理
- 可按前缀拆分路由组
- 基于 `endless` 的 HTTP 服务启动与平滑重启
- `WithGrpc=true` 时复用同一端口承载 h2c + gRPC

行为特点：

- Debug 模式下会启用 Gin 自带请求日志
- 默认会给每个请求生成请求 ID
- 框架内置自己的 panic 恢复逻辑，不依赖 Gin 默认 `Recovery`

### 4. gRPC 服务

对应文件：`thinkgo/grpc_server.go`

gRPC 相关入口：

- `GrpcOnBeforeCreate(...)`
- `GrpcOnAfterCreate(...)`
- `GrpcCreateServer()`
- `GrpcLogger(ctx)`

已提供的能力：

- unary / stream 拦截器里的 panic 恢复
- 与 HTTP 共端口的 h2c 承载能力
- 请求 ID 注入与日志关联
- 在创建 gRPC server 前后注册自定义逻辑

需要注意：

- 源码中定义了 `GrpcErrorHandler` 类型，但没有看到对应的公开 setter
- 业务项目通常应通过 `GrpcOnAfterCreate` 注册 protobuf service

### 5. WebSocket

对应文件：`thinkgo/http_server.go`、`thinkgo/http_ws.go`

WebSocket 不只是简单 upgrade，还额外提供了连接管理层。

核心入口：

- `HttpWebsocketUpgrade(c)`
- `NewHttpWsRouter[T](...)`
- `HttpWsConn[T]`
- `HttpWsConnGroup[T]`

已提供的能力：

- WebSocket upgrade
- 泛型消息路由器，支持自定义序列化 / 反序列化
- 单连接写队列
- ping / pong 保活
- 读超时管理
- 按 key 绑定单连接，并可顶掉旧连接
- 连接分组广播
- 连接级上下文存储
- close 原因枚举

适合场景：

- 游戏网关
- 长连接推送
- 用户维度的单活连接管理

补充说明：

- `HttpWebsocketUpgrade()` 的 `CheckOrigin` 默认直接返回 `true`

### 6. 数据库

对应文件：`thinkgo/database.go`、`thinkgo/database_logger.go`

数据库层本质上是 `xorm` 的运行时封装。

核心入口：

- `DBDefault()`
- `DBDefaultOrPanic()`
- `DB(name)`
- `DBOrPanic(name)`
- `DBCloseAll()`

核心类型：

- `DBInstance`，内部嵌入 `xorm.EngineInterface`

已提供的能力：

- 多数据库实例管理
- 默认实例 `default`
- 支持单库和读写集群
- 连接池参数按命令名覆盖
- 统一接入框架日志
- `ExecWithBackoff()` 封装重试执行
- `DBErrRetry()` 判断是否适合重试
- `DBErrDuplicate()` 判断是否唯一键冲突

对 Agent / 开发者的理解建议：

- 复杂数据库能力依然主要来自 `xorm`
- 框架提供的是实例创建、配置装配、日志和重试判断，不是新的 ORM

### 7. Redis

对应文件：`thinkgo/redis.go`、`thinkgo/redis_client.go`、`thinkgo/redis_client_extra.go`

Redis 层本质上是 `go-redis/v8` 的运行时封装。

核心入口：

- `RedisDefault()`
- `RedisDefaultOrPanic()`
- `Redis(name)`
- `RedisOrPanic(name)`
- `RedisCloseAll()`

核心类型：

- `RedisClient`

已提供的能力：

- 多 Redis 实例管理
- 默认实例 `default`
- key 前缀自动拼接
- 按命令名覆盖 timeout / pool size / retries
- `ExecWithBackoff()` 封装重试执行
- `Wait()` 等待 Redis 可用
- `RedisErrorRetry()` / `RedisErrorKeyNotExist()` 等错误判断
- `Raw()` 暴露原始 `*redis.Client`
- 额外 Lua 辅助命令 `SetExf()`、`IncrEx()`、`IncrByEx()`

对 Agent / 开发者的理解建议：

- `RedisClient` 已经代理了大部分常用 `Cmdable` 能力，很多时候不需要自己再包一层
- 如果直接用 `Raw()`，记得通过 `Prefix()` 或 `Prefixes()` 处理 key 前缀

### 8. 分布式锁 / 本地锁

对应文件：`thinkgo/key_lock.go`

已提供三种 key 级别锁：

- `KeyLockMem*`：进程内内存锁
- `KeyLockRedis*`：基于 Redis 的分布式锁
- `KeyLockMySQL*`：基于 MySQL `GET_LOCK` / `RELEASE_LOCK`

核心能力：

- 阻塞锁
- try lock
- timeout lock
- 锁失败错误判断 `KeyLockErrLockedFail(...)`

适合场景：

- 关键资源串行化
- 防重复提交
- 跨进程互斥

### 9. KeyStore 业务缓存

对应文件：`thinkgo/key_store.go`、`thinkgo/key_store_backend.go`

`KeyStore` 是“本地缓存 + 后端存储”的统一抽象。

核心入口：

- `NewKeyStore(...)`
- `NewKeyStoreWithCache(...)`
- `KeyStoreBackend`
- `KeyStoreBackendRedis`
- `NewKeyStoreBackendAsync(...)`

已提供的能力：

- 本地 fastcache 缓存
- 缓存 TTL
- 从 backend 回源加载
- 写入时同步或异步落 backend
- Redis backend 支持
- `Load()` / `LoadOrCreate()` / `Store()`
- 基于内存锁的 `Lock()` / `LockTimeout()` 更新模式

需要注意：

- `LoadOrCreate()` 只会把创建结果放进本地缓存，不会自动写回 backend

### 10. Leader 选举

对应文件：`thinkgo/leader.go`

已提供基于 Redis 的 leader 选举器：

- `Leader`
- `LeaderRedis`
- `NewLeaderRedis(name, id, client)`

核心能力：

- 多实例竞争 leader
- 自动续租
- leader 状态变化回调
- 在线 / 离线切换
- 自动挂接到 start/shutdown hook

适合场景：

- 定时任务单实例执行
- 主从角色切换

### 11. GatewayClient

对应文件：`thinkgo/gateway_client.go`

这里的 `GatewayClient` 是给 PHP WorkerMan Gateway 使用的客户端，不是通用 API Gateway SDK。

已提供的能力：

- 连接 register 地址发现 gateway 节点
- 长驻连接多个 gateway 进程
- 按协议包发送消息
- `SendToGroup(...)`
- `SendToUid(...)`
- 可周期性拉取 gateway 列表

如果业务项目里有 WorkerMan / GatewayWorker 协作，这部分能力可以直接复用。

### 12. SQL 批量构建器

对应文件：`thinkgo/insert_builder.go`、`thinkgo/insert_on_duplicate_builder.go`

已提供两个批量 SQL 构建器：

- `InsertBuilder`
- `InsertOnDuplicateBuilder`

适合场景：

- MySQL / PostgreSQL 批量插入
- MySQL `insert ignore` / `replace`
- PostgreSQL `on conflict do nothing`
- MySQL / PostgreSQL upsert

### 13. 并发与缓存基础工具

对应文件：`thinkgo/concurrent_map.go`、`thinkgo/concurrent_map_lru.go`、`thinkgo/sync_map.go`、`thinkgo/util_go_queue.go`、`thinkgo/timeout_once.go`、`thinkgo/minutes_ticker.go`、`thinkgo/util_send_close.go`、`lru/lru.go`

已提供的能力：

- `CMap[K, V]`：分片并发 map
- `CMapLRU[T]`：带 TTL + LRU 的分片缓存
- `SyncMap[T]`：支持 `LoadOrCreate` 的 `sync.Map` 封装
- `GoQueue`：单协程消费队列
- `TimeoutOnce`：带超时等待的 once
- `MinutesTicker`：按自然分钟对齐的 ticker
- `SendClose`：帮助“关闭后忽略 send”的并发控制器
- `lru.Cache`：基础 LRU 容器

如果业务代码只是需要一个并发 map、一个轻量本地缓存或一个串行 worker，优先看看这些能力，不要先引入新依赖。

### 14. 类型转换与数据辅助

对应文件：`thinkgo/jmap.go`、`thinkgo/conv_*.go`

已提供的能力：

- `JMap`：动态 JSON map 工具
- `JString` / `JInt` / `JFloat` / `JBool`
- `ConvDuration`
- `ConvDecimal`
- `ConvInt64`
- `ConvBool`
- `ConvInts`
- `ConvUInt32s`

适合场景：

- 配置解析
- 动态 JSON 解析
- xorm / JSON / TOML 的统一转换

### 15. 通用工具函数

对应文件：`thinkgo/util.go`、`thinkgo/util_http.go`、`thinkgo/util_io.go`、`thinkgo/util_ip.go`、`thinkgo/util_stack.go`

已提供的能力：

- `SafeGo()` / `SafeGo1()` 安全启动 goroutine
- `BackoffPolicyDefault()` / `BackoffPolicyForever()`
- `HttpGet()` / `HttpPost()` / `HttpPostForm()` / `HttpBodyWithTries()`
- `ParseForm()` / `ClientIp()`
- `ReadFile()` / `WriteFile()` / `IsDir()` / `IsFile()`
- `Stack()` 获取格式化调用栈
- `Md5()` / `Crc32()` / `RandString()`
- `SafeSendChannel()`
- `TruncateFloat64()` / `TruncateBigFloat()`
- `NginxHash()`

这些函数偏“项目内公用基础设施”，不是标准库替代品，但很多业务项目会直接复用。

## 工程脚手架与配套脚本

### 1. 项目目录初始化

对应文件：`app_init.sh`

已提供的能力：

- 创建 `app/` 或 `app/{module}/` 目录
- 创建 `config`、`bin`、`runtime/log`、`runtime/pid`
- 输出建议加入的 `.gitignore` 片段

### 2. 构建脚本

对应文件：`hook_pull.sh`

已提供的能力：

- 编译 `src/main/*.go` 或 `src/{module}/main/*.go`
- 编译 module 时通过 `ldflags` 注入 `thinkgo.ModuleName`
- 为 `src/cmd/*.go` 或 `src/{module}/cmd/*.go` 生成命令 wrapper

### 3. 文档脚本

对应文件：`doc.sh`

已提供的能力：

- 自动安装 `godoc`
- 本地启动包文档服务

## 给 Agent / 开发者的使用约定

### 1. 先判断框架是否已经有现成能力

在业务项目中遇到以下需求时，优先检查框架而不是立即新造工具层：

- HTTP 路由和统一错误处理
- 请求级日志
- WebSocket 长连接管理
- DB/Redis 实例获取和重试
- 分布式锁 / 本地锁
- 本地缓存或带后端的业务缓存
- leader 选举
- 并发 map / GoQueue / ticker / once

### 2. 先看这些位置

如果某个业务项目使用了本框架，通常优先看这些文件或目录：

- `main.go` 或 `src/main/*.go`
- `src/cmd/*.go`
- `src/{module}/main/*.go`
- `src/{module}/cmd/*.go`
- `handler_http.go`
- `handler_ws.go`
- `app/config/*.toml`
- 构建脚本里是否设置了 `ModuleName` 或 `_TK_Command`

### 3. 记住它是“薄封装 + 运行时装配”

对这个框架要有两个判断：

- HTTP、生命周期、锁、缓存、并发工具这几块是框架自己提供的能力
- DB 和 Redis 主要还是 `xorm` 与 `go-redis` 的运行时封装

也就是说：

- 如果要改数据库查询语义，通常还是查 `xorm` 用法
- 如果要做复杂 Redis 操作，通常还是参考 `go-redis` 能力
- 但实例获取、配置装配、日志、重试和前缀处理，应该优先复用框架

### 4. 注意 import 即初始化

这是使用本框架时最容易忽略的一点：

- 不是等到 `main()` 里才初始化
- 而是导入 `thinkgo` 包时就会初始化路径、配置、日志、Redis

所以 Agent / 开发者修改业务项目时：

- 不要轻易把 `thinkgo` 引入到一个脱离项目目录结构的独立工具中
- 写测试或脚本时，先确认 `app/`、配置文件和环境变量是否齐全

## 框架边界

基于当前仓库源码，已明确提供的能力如上；同时，当前仓库中没有看到这些内建能力：

- 依赖注入容器
- 配置中心 / 配置热更新
- 消息队列客户端封装
- 指标监控 / tracing
- 数据库 migration 工具
- 认证鉴权体系
- 统一 DTO 校验层

如果业务项目里存在这些能力，通常是业务项目自己加的，不是这个框架原生提供的。
