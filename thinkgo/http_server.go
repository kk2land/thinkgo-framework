package thinkgo

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
)

// Http Server，基于Gin - https://github.com/gin-gonic/gin

const httpContextLogger = "tk-ctx-logger"

// HttpErrorHandler http的默认panic处理函数，需要对客户端返回进行处理
type HttpErrorHandler func(c *gin.Context, err interface{})

var (
	httpEngineOnce sync.Once
	httpEngine     *gin.Engine = nil
	httpRouters                = NewSyncMap[gin.IRouter](func(key string) (gin.IRouter, error) {
		if key == "" || key == "/" {
			return HttpEngine(), nil
		} else {
			return HttpEngine().Group(key), nil
		}
	})

	httpRootPath         atomic.Value
	httpPort             atomic.Int32
	httpRequestIdCounter atomic.Uint64
	httpErrorHandlerRef  atomic.Value
)

func httpRequestId() string {
	id := httpRequestIdCounter.Add(1)
	return strconv.Itoa(Pid) + "_" + strconv.FormatUint(id, 10)
}

func httpMiddlewareHandleError(c *gin.Context) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		stack := Stack(3, 5)
		if ErrIsBrokenPipe(r) {
			_ = c.Error(r.(error)) // nolint: errcheck
			c.Abort()
		} else {
			HttpLogger(c).Errorf("[HttpErrorHandler] err=%s\n%s", r, stack)
			if errorHandler := httpErrorHandlerRef.Load(); errorHandler != nil {
				errorHandler.(HttpErrorHandler)(c, r)
			} else {
				if !AppDebug {
					_ = c.AbortWithError(599, errors.New("internal error"))
				} else {
					msg := fmt.Sprintf("%v\n%s", r, stack)
					_ = c.AbortWithError(599, errors.New(msg))
				}
			}
		}
	}()
	c.Next()
}

func HttpSetErrorHandler(errorHandler HttpErrorHandler) {
	httpErrorHandlerRef.Store(errorHandler)
}

// HttpLogger 基于gin.Context获取当前请求的日志对象
func HttpLogger(c *gin.Context) FieldLogger {
	if logger, ok := c.Get(httpContextLogger); ok {
		return logger.(FieldLogger)
	} else {
		return Logger
	}
}

// HttpEngine 获取gin.Engine
func HttpEngine() *gin.Engine {
	httpEngineOnce.Do(func() {
		gin.DefaultWriter = Logger.Out()
		gin.DefaultErrorWriter = Logger.Out()
		if AppDebug {
			gin.SetMode(gin.DebugMode)
		} else {
			gin.SetMode(gin.ReleaseMode)
		}
		httpEngine = gin.New()
		var funcs []gin.HandlerFunc
		funcs = append(funcs, func(c *gin.Context) {
			c.Set(httpContextLogger, Logger.With(
				loggerFieldId, httpRequestId(), "ip", c.ClientIP(), "path", c.Request.URL.Path,
			))
		})
		if AppDebug {
			funcs = append(funcs, gin.Logger())
		}
		funcs = append(funcs, httpMiddlewareHandleError)
		httpEngine.Use(funcs...)
	})
	return httpEngine
}

// HttpRouter 获取默认配置路径的gin.IRouter
func HttpRouter() gin.IRouter {
	return HttpEngine()
}

// HttpRouterWithPath 获取指定前缀路径的gin.Router
func HttpRouterWithPath(relativePath string) gin.IRouter {
	r, _ := httpRouters.LoadOrCreate(relativePath)
	return r
}

// HttpStartServer 启动默认配置gin的http-server
func HttpStartServer() {
	HttpStartServerWithConfig(&Config.Http)
}

// HttpStartServerWithConfig 启动gin的http-server
func HttpStartServerWithConfig(httpConfig *HttpConfig) {
	port := httpConfig.Port
	if port <= 0 {
		envPort := os.Getenv(envKeyInternalHttpPort)
		if len(envPort) > 0 {
			var err error
			if port, err = strconv.Atoi(envPort); err != nil {
				LogErrAndPanic("[HttpStartServer]env[%s]不是合法port - %s", envKeyInternalHttpPort, envPort)
			}
		}
	}
	Logger.Infof("HttpStartServerWithConfig port=%d", httpConfig.Port)

	var handler http.Handler
	if httpConfig.WithGrpc {
		//todo 配置grpc
		handler = grpcHttpHandler(HttpEngine())
	} else {
		handler = HttpEngine()
	}
	StartHttpServer(
		port,
		h2c.NewHandler(handler, &http2.Server{}),
		func(server *http.Server, registerSignalHook HttpServerRegisterSignalHook) {
			if httpConfig.ReadTimeout != 0 {
				server.ReadTimeout = httpConfig.ReadTimeout.Duration()
			}
			if httpConfig.WriteTimeout != 0 {
				server.WriteTimeout = httpConfig.WriteTimeout.Duration()
			}
			if httpConfig.IdleTimeout != 0 {
				server.IdleTimeout = httpConfig.IdleTimeout.Duration()
			}
			if httpConfig.MaxHeaderBytes != 0 {
				server.MaxHeaderBytes = httpConfig.MaxHeaderBytes
			}
		},
	)
}

// HttpWebsocketUpgrade 处理websocket客户端连接
func HttpWebsocketUpgrade(c *gin.Context) (*websocket.Conn, error) {
	up := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	return up.Upgrade(c.Writer, c.Request, nil)
}

// HttpPort 获取当前http-server的端口
func HttpPort() int {
	return int(httpPort.Load())
}
