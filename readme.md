# thinkgo-framework - 自研 Go 服务端框架

ThinkGo 是一个轻量 Go 服务端运行时框架，统一提供配置、生命周期、日志、HTTP/Gin、gRPC、WebSocket、DB/xorm、Redis、锁、缓存、Leader 选举和常用并发工具。

## 文档

- [`AGENTS.md`](./AGENTS.md)：AI Coding Agent 的首要入口；包含框架能力导航、开发决策规则、约束和文档维护规则
- [`USAGE.md`](./USAGE.md)：完整能力说明、主要 API、运行时行为、边界和注意事项
- [`app.toml`](./app.toml)：配置示例
- `thinkgo/*.go`：框架真实实现；文档与源码不一致时以当前源码为准

Agent 开发基于 ThinkGo 的项目时，阅读顺序建议为：`AGENTS.md` → `USAGE.md` → 对应 `thinkgo/*.go`。

## 安装

```bash
go get github.com/kk2land/thinkgo-framework
```

主要包：

```go
import "github.com/kk2land/thinkgo-framework/thinkgo"
```

## 部署

1. 将 `app_init.sh` 和 `hook_pull.sh` 复制到项目根目录。
2. 执行 `app_init.sh {module}` 生成目录结构；`{module}` 为空表示不使用 module 层级。
3. 使用 module 时，在构建流程中传入 module，并通过 ldflags 注入 `thinkgo.ModuleName`。

标准运行目录包含 `app/config`、`app/runtime`、`app/bin`；module 模式对应 `app/{module}/config`、`app/{module}/runtime`、`app/{module}/bin`。

## 配置

配置格式参考 `app.toml`。框架支持项目根目录 `.env`、`_TK_AppStatus` 环境配置覆盖和 module 配置叠加。

module 模式配置加载顺序：

1. `app/config/app.toml`
2. `app/config/app_{_TK_AppStatus}.toml`
3. `app/{module}/config/app.toml`
4. `app/{module}/config/app_{_TK_AppStatus}.toml`

完整环境变量、模板宏和配置行为见 [`USAGE.md`](./USAGE.md)。

## HTTP 示例

`handler_http.go`：

```go
var _ = thinkgo.HttpRouter().GET("/", func(c *gin.Context) {
    c.JSON(200, gin.H{
        "code": 0,
        "msg":  "succ",
        "data": map[string]interface{}{},
    })
})
```

入口中导入业务路由包后，通过 ThinkGo HTTP 启动能力运行服务。具体启动、错误处理、gRPC 共端口等能力见 `USAGE.md`。

## WebSocket

框架提供 `NewHttpWsRouter[T]`、`HttpWsConn[T]`、`HttpWsConnGroup[T]` 等泛型 WebSocket 路由与连接管理能力，包括消息编解码、写队列、ping/pong、连接绑定和分组广播。

不要以旧版非泛型示例作为 API 依据；以 [`USAGE.md`](./USAGE.md) 和当前 `thinkgo/http_ws.go` 为准。

## Redis

通过 `thinkgo.RedisDefault()` / `thinkgo.Redis(name)` 获取框架管理的 `RedisClient`。框架负责实例、配置、key prefix、重试和生命周期；详细用法见 `USAGE.md`。

## 数据库

数据库基于 xorm。通常通过：

```go
db := thinkgo.DBDefaultOrPanic()
```

获取默认实例后按 xorm 语义查询。例如：

```go
type MyStudent struct {
    Id         int64 `xorm:"pk autoincr"`
    Name       string
    CreateTime int64 `xorm:"ct"`
}

func (m *MyStudent) TableName() string {
    return "student"
}

student := &MyStudent{
    Name:       "A",
    CreateTime: time.Now().Unix(),
}
affected, err := db.Insert(student)

students := make([]*MyStudent, 0)
err = db.Find(&students)

one := new(MyStudent)
has, err := db.Where("id = ?", 1).Get(one)
```

ThinkGo 主要负责 xorm 实例装配、连接池、日志和重试判断，不替代 xorm 本身。