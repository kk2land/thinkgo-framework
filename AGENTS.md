# ThinkGo Framework — Agent Guide

本文是 AI Coding Agent 在使用 `thinkgo-framework` 开发或修改业务项目时的入口文档。

> 完整能力说明与 API 入口见 [`USAGE.md`](./USAGE.md)。在实现基础设施能力前，先阅读本文，再按需查阅 `USAGE.md` 和对应源码。

## 1. Agent 的首要原则

在新增代码前，先判断 ThinkGo 是否已经提供对应能力。

优先复用框架已有实现，不要在业务项目中重复封装：

- 配置与环境加载
- 生命周期和进程管理
- 日志与请求级日志
- HTTP / Gin 服务
- gRPC 服务
- WebSocket 连接管理
- DB / xorm 实例管理
- Redis / go-redis 实例管理
- 本地锁与分布式锁
- KeyStore 缓存
- Leader 选举
- SQL 批量构建
- 并发 Map、LRU、队列、Ticker、Once
- Backoff / 重试
- 常用 HTTP、IO、IP、转换工具

如果框架已有能力满足需求，应优先使用框架能力，而不是新增平行抽象或第三方依赖。

## 2. 框架定位

`thinkgo-framework` 是 Go 服务端项目的轻量运行时框架。

主要包：

```go
import "github.com/kk2land/thinkgo-framework/thinkgo"
```

它主要负责基础设施的统一装配和公共能力复用。

它不是：

- DI 容器
- 完整业务脚手架平台
- 新的 ORM
- 新的 Redis 协议层

数据库能力主要建立在 `xorm` 上；Redis 能力主要建立在 `go-redis/v8` 上。ThinkGo 负责实例创建、配置、日志、重试、前缀和生命周期等运行时能力。

## 3. 重要运行时特征

### Import 即初始化

导入 `thinkgo` 包时，框架 `init()` 会立即执行，包括：

- 推导项目路径
- 加载 `.env`
- 加载 TOML 配置
- 初始化全局 `Config`
- 初始化全局 `Logger`
- 初始化相关运行时设施
- 注册 start / shutdown hook

因此不要随意在脱离标准项目目录的临时程序或测试中导入 `thinkgo`。

必要时可通过 `_TK_RootPath` 指定项目根目录。

### 常用全局运行时变量

重点关注：

- `ModuleName`
- `InModule`
- `RootPath`
- `AppPath`
- `RuntimePath`
- `CommandName`
- `AppName`
- `AppStatus`
- `AppDebug`
- `Config`
- `Logger`

### 标准目录

业务项目通常具有：

```text
app/
  config/
  runtime/
    log/
    pid/
  bin/
```

module 模式下通常为：

```text
app/{module}/
  config/
  runtime/
  bin/
```

## 4. 能力选择速查

| 需求 | 优先检查 |
| --- | --- |
| 配置 / 环境变量 | `config.go` |
| 生命周期 / 命令 / PID | `base.go` |
| 日志 | `logger.go` |
| HTTP / Gin | `http_server.go` |
| WebSocket | `http_ws.go` |
| gRPC | `grpc_server.go` |
| 数据库 | `database.go` |
| Redis | `redis.go`, `redis_client.go` |
| 本地 / Redis / MySQL 锁 | `key_lock.go` |
| 本地 + Backend 缓存 | `key_store.go`, `key_store_backend.go` |
| Leader 选举 | `leader.go` |
| GatewayWorker 客户端 | `gateway_client.go` |
| 批量 Insert / Upsert | `insert_builder.go`, `insert_on_duplicate_builder.go` |
| 并发 Map / LRU | `concurrent_map.go`, `concurrent_map_lru.go` |
| 串行任务队列 | `util_go_queue.go` |
| Backoff | `backoff.go` |
| 动态 JSON | `jmap.go` |
| 类型转换 | `conv_*.go` |
| 通用工具 | `util*.go` |

详细能力、入口函数和注意事项见 [`USAGE.md`](./USAGE.md)。

## 5. Agent 开发流程

处理使用 ThinkGo 的业务项目时，按以下顺序工作：

1. 确认业务项目是否依赖 `github.com/kk2land/thinkgo-framework`。
2. 检查项目的 `app/config/*.toml`、`.env`、启动入口和构建脚本。
3. 阅读本文件和 `USAGE.md` 中与需求对应的章节。
4. 搜索 `thinkgo/` 中已有类型和函数，确认真实 API 和行为。
5. 搜索业务项目已有用法，优先保持现有项目风格。
6. 只有框架和业务项目均不存在对应能力时，再考虑新增抽象或第三方依赖。
7. 修改后检查初始化、副作用、生命周期、并发和 shutdown 行为。

不要仅根据函数名猜测行为；涉及关键逻辑时直接阅读实现。

## 6. 常见决策规则

### HTTP

优先使用框架提供的 `HttpEngine()`、`HttpRouter()`、`HttpRouterWithPath()`、统一错误处理和请求日志机制。

不要无必要重新创建独立 Gin Engine 或另一套 recovery / request logger。

### DB

优先通过 `DBDefault()` / `DB(name)` 获取实例。

查询和 ORM 语义按 xorm 使用；实例装配、日志、连接池和重试优先沿用 ThinkGo。

### Redis

优先通过 `RedisDefault()` / `Redis(name)` 获取 `RedisClient`。

不要为了几个 Redis 命令重新封装一层 client。使用 `Raw()` 时注意框架 key prefix 规则。

### 锁

先根据范围选择：

- 单进程：`KeyLockMem*`
- 跨进程且已有 Redis：`KeyLockRedis*`
- 依赖 MySQL 锁：`KeyLockMySQL*`

不要用普通 Redis `SETNX` 在业务代码中重新实现已有锁语义。

### 缓存

简单本地并发缓存优先检查 `CMapLRU` / `lru.Cache`。

需要“本地缓存 + 后端回源/写入”时优先检查 `KeyStore`。

### 后台 goroutine

需要 panic 防护时优先检查 `SafeGo()` / `SafeGo1()`；需要串行消费时优先检查 `GoQueue`。

需要跟随服务退出的后台任务，应检查并接入框架 shutdown 生命周期，而不是创建无法退出的 goroutine。

## 7. Agent 禁止的默认做法

除非需求明确或现有框架确实无法满足，否则不要：

- 再创建一套配置加载器
- 再创建全局 logger
- 再封装一套 DB / Redis manager
- 再实现 Redis 分布式锁
- 再实现 WebSocket connection manager
- 为已有轻量能力随意引入新第三方依赖
- 绕过框架生命周期启动永久 goroutine
- 假设导入 `thinkgo` 没有副作用
- 在未阅读源码时改变框架关键运行时语义

## 8. 框架当前边界

当前框架没有内建提供完整的：

- 依赖注入容器
- 配置中心 / 配置热更新
- 消息队列客户端抽象
- Metrics / distributed tracing
- Database migration
- 认证鉴权体系
- 统一 DTO validation 层

遇到这些需求时，应先检查业务项目是否已有实现，再决定扩展位置。

## 9. 修改框架本身时

如果任务是修改 `thinkgo-framework`，而不是使用框架开发业务功能：

- 优先保持已有 API 向后兼容。
- 修改全局初始化逻辑时评估所有 import `thinkgo` 的项目影响。
- 修改配置加载顺序、默认值、路径规则时视为高影响变更。
- 修改 DB / Redis wrapper 时避免破坏底层 xorm / go-redis 的既有使用方式。
- 新增基础设施能力前判断它是否属于框架层，而不是特定业务逻辑。
- 新能力应同步更新 `USAGE.md`，必要时更新本文件的能力速查表。

## 10. 文档优先级

Agent 获取框架信息时按以下顺序：

1. `AGENTS.md`：开发决策、约束和能力导航。
2. `USAGE.md`：完整能力说明、API 入口和注意事项。
3. `thinkgo/*.go`：真实实现，以源码为最终依据。
4. `app.toml`：配置示例。
5. `readme.md`：项目背景和基础说明。

当文档与源码不一致时，以当前源码行为为准，并考虑同步修正文档。
