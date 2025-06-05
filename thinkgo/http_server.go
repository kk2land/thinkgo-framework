package thinkgo

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
)

// Http Server，基于Gin - https://github.com/gin-gonic/gin

const httpContextLogger = "tk-ctx-logger"

// var httpRouterOnce sync.Once
var httpEngine *gin.Engine = nil
var httpRouter gin.IRouter
var httpPort int
var httpRequestPrefix []byte
var httpRequestCounter uint64

// HttpServerInit 默认的httpServer初始化
func HttpServerInit() {
	HttpServerInitWithConfig(&Config.Http)
}

// HttpServerInitWithConfig httpServer初始化
func HttpServerInitWithConfig(httpConfig *HttpConfig) {
	gin.DefaultWriter = Logger.Out()
	gin.DefaultErrorWriter = Logger.Out()
	if AppDebug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	httpRequestPrefix = []byte(fmt.Sprintf("%d_", Pid))

	httpEngine = gin.New()
	var funcs []gin.HandlerFunc
	funcs = append(funcs, func(c *gin.Context) {
		c.Set(httpContextLogger, Logger.With(
			loggerFieldId, httpIncrReqId(),
			"ip", c.ClientIP(),
			"path", c.Request.URL.Path,
		))
	})
	if AppDebug {
		funcs = append(funcs, gin.Logger())
	}
	funcs = append(funcs, httpErrorHandler)
	httpEngine.Use(funcs...)

	rootPath := strings.TrimSpace(httpConfig.RootPath)
	if len(rootPath) == 0 || httpConfig.RootPath == "/" {
		httpRouter = httpEngine
	} else {
		httpRouter = httpEngine.Group(rootPath)
	}
	Logger.Infof("HttpServerInitWithConfig finished-%d", httpConfig.Port)
}

func httpIncrReqId() string {
	id := atomic.AddUint64(&httpRequestCounter, 1)
	return string(strconv.AppendUint(httpRequestPrefix, id, 10))
}

func httpErrorHandler(c *gin.Context) {
	defer func() {
		err := recover()
		if err == nil {
			return
		}
		logger := HttpLogger(c)
		brokenPipe := ErrIsBrokenPipe(err)
		msg := "internal error"
		stack := Stack(3, 5)

		var noStack = false
		var apiErr *ApiError = nil
		if apiError, ok := err.(*ApiError); ok && !AppDebug {
			noStack = apiError.noStack
		}
		if !noStack {
			logger.Errorf("[HttpErrorHandler] err=%s\n%s", err, stack)
		} else {
			logger.Errorf("[HttpErrorHandler] err=%s", err)
		}
		if AppDebug {
			msg = fmt.Sprintf("%s\n%s", err, stack)
		}
		if brokenPipe {
			_ = c.Error(err.(error)) // nolint: errcheck
			c.Abort()
		} else {
			var ret = "0"
			if !AppDebug && apiErr != nil {
				ret = fmt.Sprintf("%d", apiErr.code)
				msg = apiErr.apiMessage
			}
			c.AbortWithStatusJSON(http.StatusOK, gin.H{
				"ret":     ret,
				"msg":     msg,
				"content": gin.H{},
			})
		}
	}()
	c.Next()
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
	return httpEngine
}

// HttpRouter 获取gin.IRouter
func HttpRouter() gin.IRouter {
	HttpEngine()
	return httpRouter
}

// HttpStartServerDefault 启动默认配置gin的http-server
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
	var handler http.Handler
	if httpConfig.WithGrpc {
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
	return httpPort
}
