package thinkgo

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"sync"
	"time"
)

/*

websocket支持，https://github.com/gorilla/websocket

流程说明：
1，http协程读取消息之后，通过channel发送给websocket协程
2，websocket-send协程，负责往客户端发送消息

// 使用get方式注册websocket处理
WsEventRouterRegister(path string, *WsEventRouterOption) WsEventRouter

*/

const (
	// WsEventErrTypeOpenFail WsEventRouterOption.ErrorHandler的errorType，OpenHandler返回的错误
	WsEventErrTypeOpenFail = 1
	// WsEventErrTypeStartFail CtxConnStartHandler panic的错误
	WsEventErrTypeStartFail = 2
	// WsEventErrTypeNotfound event对应的handler找不到
	WsEventErrTypeNotfound = 3
	// WsEventErrTypeEventPanic event处理的handler panic的错误
	WsEventErrTypeEventPanic = 4
)

// WsEventErrNoConn 写消息到客户端是，没有链接alive
var WsEventErrNoConn = errors.New("no conn")

// WsEventConnIdleTimeout 客户端超过多少秒没有消息过来，则关闭客户端连接
var WsEventConnIdleTimeout = 300 * time.Second

// WsEventReadChannelSizeDefault 接收客户端消息的channel默认size
var WsEventReadChannelSizeDefault = 1

// WsEventWriteChannelSizeDefault 发送客户端消息的channel默认size
var WsEventWriteChannelSizeDefault = 10

// WsEventMessage event消息
type WsEventMessage struct {
	Event   string
	Message []byte
}

func NewWsEventMessage(e string, m []byte) *WsEventMessage {
	return &WsEventMessage{Event: e, Message: m}
}

func (m *WsEventMessage) String() string {
	buf := BytesBuffer1024.Get()
	defer BytesBuffer1024.Put(buf)
	wsEventMessagePack(m, buf)
	return buf.String()
}

type WsEventRouterOption struct {
	/*
		OpenHandler http协程中调用，websocket链接打开的handler，返回值说明：
			key = 链接的标识，方便后续主动推送消息给客户端
			params = WsEventContext的CtxSafeParamGet读取的map
			err = 失败则断开链接
	*/
	OpenHandler func(c *gin.Context) (key string, params map[string]interface{}, err error)
	/*
		ErrorHandler http协程/websocket协程中调用，错误处理，返回值不为nil，则会发送给客户端
	*/
	ErrorHandler func(errorType int, err error) *WsEventMessage
	/*
		WriteFailHandler 任意协程中调用，处理写websocket-send协程的channel失败
	*/
	WriteFailHandler func(key string, msg *WsEventMessage, err error)
	/*
		CtxConnStartHandler websocket协程中调用，websocket链接打开的handler，参数说明：
			restart = 非首次链接
	*/
	CtxConnStartHandler func(ctx WsEventContext, connId uint64, restart bool)
	/*
		CtxConnCloseHandler websocket协程中调用，websocket链接关闭
	*/
	CtxConnCloseHandler func(ctx WsEventContext, connId uint64)
	/*
		CtxStopHandler websocket协程中调用，websocket链接stop(客户端超过CtxKeepalive时间还没有连接处触发)
	*/
	CtxStopHandler   func(ctx WsEventContext)
	ReadChannelSize  int
	WriteChannelSize int
	CtxKeepalive     time.Duration //客户端断开链接后，WsEventContext保留的秒数，默认60s
}

type WsEventHandler func(ctx WsEventContext, msg *WsEventMessage) *WsEventMessage

// WsEventContext 以下方法协程安全
type WsEventContext interface {
	Key() string
	ParamsString() string //path中动态的参数拼接起来的字符串
	Logger() FieldLogger
	Send(msg *WsEventMessage)
	SendForKeys(msg *WsEventMessage, keys ...string)
	SendForParams(msg *WsEventMessage, paramsString string)
	SendForAll(msg *WsEventMessage)
	SendForParamsExcludeKeys(msg *WsEventMessage, paramsString string, excludeKeys map[string]bool)
	SendForAllExcludeKeys(msg *WsEventMessage, excludeKeys map[string]bool)
	Close()
	// CtxSafeValueSet 以下方法只在：CtxConnStartHandler/CtxConnCloseHandler/CtxStopHandler中协程安全
	CtxSafeValueSet(k string, v interface{})
	CtxSafeValueGet(k string) (interface{}, bool)
	CtxSafeParamGet(k string) (interface{}, bool)
}

type WsEventRouter interface {
	// EventRegister 注册事件处理handler
	EventRegister(event string, handler WsEventHandler)
	// SendForKeys 给对应的key集合发送消息
	SendForKeys(msg *WsEventMessage, keys ...string)
	// SendForParams 基于path中动态参数拼接字符串发送消息
	SendForParams(msg *WsEventMessage, paramsString string)
	// SendForAll 发送给全部客户端
	SendForAll(msg *WsEventMessage)
	// SendForParamsExcludeKeys 基于path中动态参数拼接字符串发送消息，同时排除一些key
	SendForParamsExcludeKeys(msg *WsEventMessage, paramsString string, excludeKeys map[string]bool)
	// SendForAllExcludeKeys 发送给全部客户端，同时排除一些key
	SendForAllExcludeKeys(msg *WsEventMessage, excludeKeys map[string]bool)
	// 关闭当前的router
	Close(shouldWait bool)
}

var wsEventRouterMap = make(map[string]*wsEventRouter)

func WsEventRouterRegister(path string, opt *WsEventRouterOption) WsEventRouter {
	var router *wsEventRouter
	var ok bool
	if router, ok = wsEventRouterMap[path]; ok {
		panic(fmt.Errorf("router(%s)已经注册", path))
	}
	if opt.OpenHandler == nil {
		panic(fmt.Errorf("[WsEventRouterRegister]opt.OpenHandler未设置,path=%s", path))
	}
	router = newWsEventRouter(path, opt)
	wsEventRouterMap[path] = router
	HttpRouter().GET(path, router.httpHandle)
	return router
}

func WsEventRouterGet(path string) (WsEventRouter, bool) {
	if router, ok := wsEventRouterMap[path]; ok {
		return router, true
	} else {
		return nil, false
	}
}

func WsEventParamsString(params ...string) string {
	buf := BytesBuffer1024.Get()
	defer BytesBuffer1024.Put(buf)
	for _, s := range params {
		buf.WriteByte('/')
		buf.WriteString(s)
	}
	return buf.String()
}

func WsEventKey(k string, paramsString string) string {
	if len(paramsString) == 0 {
		return k
	}
	return paramsString + "^" + k
}

// wsEventClose 进程关闭时调用
func wsEventClose(wait *sync.WaitGroup) {
	wait.Add(1)
	go func() {
		wait1 := sync.WaitGroup{}
		wait1.Add(len(wsEventRouterMap))
		for k, v := range wsEventRouterMap {
			Logger.Infof("[wsEventClose] 开始关闭,path=%s", k)
			go func(r *wsEventRouter) {
				r.Close(true)
				wait1.Done()
			}(v)
		}
		wait1.Wait()
		Logger.Infof("[wsEventClose] 全部router已经关闭")
		wait.Done()
	}()
}
