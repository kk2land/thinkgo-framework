package thinkgo

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var wsEventConnCounter uint64 = 0
var wsEventRouterCloseErr = errors.New("router close")
var wsEventContextKeepaliveDefault = 60 * time.Second

func wsEventEventHandlerPing(ctx WsEventContext, msg *WsEventMessage) *WsEventMessage {
	return NewWsEventMessage("pong", nil)
}

func wsEventMessageUnpack(data []byte) *WsEventMessage {
	arr := bytes.SplitN(data, wsEventMessageSep, 2)
	event := string(arr[0])
	var msg []byte = nil
	if len(arr) == 2 {
		msg = arr[1]
	}
	return NewWsEventMessage(event, msg)
}

func wsEventMessagePack(msg *WsEventMessage, buf *bytes.Buffer) {
	buf.WriteString(msg.Event)
	if msg.Message != nil {
		buf.WriteByte(',')
		buf.Write(msg.Message)
	}
}

func wsEventMessageWrite(conn *websocket.Conn, msg *WsEventMessage) error {
	buf := BytesBuffer1024.Get()
	defer BytesBuffer1024.Put(buf)
	wsEventMessagePack(msg, buf)
	return conn.WriteMessage(websocket.TextMessage, buf.Bytes())
}

type wsEventRouter struct {
	openHandler         func(c *gin.Context) (string, map[string]interface{}, error)
	errorHandler        func(errorType int, err error) *WsEventMessage
	writeFailHandler    func(key string, msg *WsEventMessage, err error)
	ctxConnStartHandler func(ctx WsEventContext, connId uint64, restart bool)
	CtxConnCloseHandler func(ctx WsEventContext, connId uint64)
	ctxStopHandler      func(ctx WsEventContext)
	readChannelSize     int
	writeChannelSize    int
	ctxKeepalive        time.Duration
	eventHandlers       map[string]WsEventHandler

	ctxMap  wsEventRouterCtxMap
	ctxWait sync.WaitGroup
	closed  int32
	done    chan struct{}
}

func newWsEventRouter(path string, opt *WsEventRouterOption) *wsEventRouter {
	var ctxKeepalive time.Duration
	if opt.CtxKeepalive > 0 {
		ctxKeepalive = opt.CtxKeepalive
	} else {
		ctxKeepalive = wsEventContextKeepaliveDefault
	}
	hasParams := strings.Contains(path, ":") || strings.Contains(path, "*")
	router := &wsEventRouter{
		openHandler:         opt.OpenHandler,
		errorHandler:        opt.ErrorHandler,
		writeFailHandler:    opt.WriteFailHandler,
		ctxConnStartHandler: opt.CtxConnStartHandler,
		CtxConnCloseHandler: opt.CtxConnCloseHandler,
		ctxStopHandler:      opt.CtxStopHandler,
		readChannelSize:     opt.ReadChannelSize,
		writeChannelSize:    opt.WriteChannelSize,
		ctxKeepalive:        ctxKeepalive,
		eventHandlers:       make(map[string]WsEventHandler),
		closed:              0,
		done:                make(chan struct{}),
	}
	if hasParams {
		router.ctxMap = &wsEventRouterCtxMapParams{router: router, cMap: CMapNew[*CMap[*wsEventContext]]()}
	} else {
		router.ctxMap = &wsEventRouterCtxMapDefault{router: router, cMap: CMapNew[*wsEventContext]()}
	}
	router.eventHandlers["ping"] = wsEventEventHandlerPing
	return router
}

func (r *wsEventRouter) EventRegister(event string, handler WsEventHandler) {
	if _, ok := r.eventHandlers[event]; ok {
		panic(fmt.Errorf("event(%s)已经注册过", event))
	}
	r.eventHandlers[event] = handler
}

func (r *wsEventRouter) SendForKeys(msg *WsEventMessage, keys ...string) {
	r.ctxMap.RangeByKeys(func(key string, ctx *wsEventContext) {
		if err := SafeSendChannel(ctx.sendC, msg); err != nil {
			Logger.Errorf("[wsEventContext][SendForKeys] tokey=%s,msg=%s,panic=%s", ctx.key, msg, err)
			r.callWriteFailHandler(ctx.key, msg, err)
		}
	}, keys...)
}

func (r *wsEventRouter) SendForParams(msg *WsEventMessage, paramsString string) {
	r.ctxMap.Range(func(key string, ctx *wsEventContext) {
		if err := SafeSendChannel(ctx.sendC, msg); err != nil {
			Logger.Errorf("[wsEventContext][SendForParams] tokey=%s,msg=%s,panic=%s", ctx.key, msg, err)
			r.callWriteFailHandler(ctx.key, msg, err)
		}
	}, paramsString)
}

func (r *wsEventRouter) SendForAll(msg *WsEventMessage) {
	r.ctxMap.Range(func(key string, ctx *wsEventContext) {
		if err := SafeSendChannel(ctx.sendC, msg); err != nil {
			Logger.Errorf("[wsEventContext][SendForAll] tokey=%s,msg=%s,panic=%s", ctx.key, msg, err)
			r.callWriteFailHandler(ctx.key, msg, err)
		}
	}, "")
}

func (r *wsEventRouter) SendForParamsExcludeKeys(msg *WsEventMessage, paramsString string, excludeKeys map[string]bool) {
	r.ctxMap.Range(func(key string, ctx *wsEventContext) {
		if _, ok := excludeKeys[key]; ok {
			return
		}
		if err := SafeSendChannel(ctx.sendC, msg); err != nil {
			Logger.Errorf("[wsEventContext][SendForParams] tokey=%s,msg=%s,panic=%s", ctx.key, msg, err)
			r.callWriteFailHandler(ctx.key, msg, err)
		}
	}, paramsString)
}

func (r *wsEventRouter) SendForAllExcludeKeys(msg *WsEventMessage, excludeKeys map[string]bool) {
	r.ctxMap.Range(func(key string, ctx *wsEventContext) {
		if _, ok := excludeKeys[key]; ok {
			return
		}
		if err := SafeSendChannel(ctx.sendC, msg); err != nil {
			Logger.Errorf("[wsEventContext][SendForAll] tokey=%s,msg=%s,panic=%s", ctx.key, msg, err)
			r.callWriteFailHandler(ctx.key, msg, err)
		}
	}, "")
}

func (r *wsEventRouter) Close(shouldWait bool) {
	if !atomic.CompareAndSwapInt32(&r.closed, 0, 1) {
		return
	}
	close(r.done)
	if shouldWait {
		r.ctxWait.Wait()
	}
}

func (r *wsEventRouter) httpHandle(c *gin.Context) {
	var err error
	var conn *websocket.Conn
	var key string
	var params map[string]interface{}

	up := &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	logger := HttpLogger(c)
	conn, err = up.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Errorf("[wsEventRouter][httpHandle] upgrade fail - %s", err)
		return
	}
	connId := atomic.AddUint64(&wsEventConnCounter, 1)
	defer func() {
		if err := recover(); err != nil {
			if err == wsEventRouterCloseErr {
				logger.Info("router closed")
			} else {
				stack := Stack(3, 5)
				logger.Errorf("[wsEventRouter][httpHandle] connId=%d,panic=%s\n%s", connId, err, stack)
			}
		}
		_ = conn.Close()
	}()
	key, params, err = r.callOpenHandler(c, connId)
	if err != nil {
		if msg := r.callErrorHandler(WsEventErrTypeOpenFail, err); msg != nil {
			if err = wsEventMessageWrite(conn, msg); err != nil {
				logger.Errorf("[wsEventRouter][httpHandle] wsEventMessageWrite fail,connId=%d,msg=%s,err=%s", connId, msg, err)
			}
		}
		return
	}
	ctx := r.ctxMap.loadCtx(key, logger, c)
	ctx.logger.Debug("[wsEventRouter][httpHandle] ctx run")
	send := func(inData interface{}) {
		if err := SafeSendChannel(ctx.inC, inData); err != nil {
			panic(wsEventRouterCloseErr)
		}
	}
	conn.SetPongHandler(func(appData string) error {
		send(&wsEventContextInPong{connId: connId})
		return nil
	})
	send(&wsEventContextInConnStart{params: params, conn: conn, connId: connId})
	for {
		_, b, err := conn.ReadMessage()
		if err != nil {
			ctx.logger.Errorf("[wsEventRouter][httpHandle] ReadMessage fail,connId=%d,err=%s", connId, err)
			_ = conn.Close()
			send(&wsEventContextInConnClose{connId: connId})
			break
		}
		ctx.logger.Debugf("[wsEventRouter][httpHandle] ReadMessage=%s", b)
		send(&wsEventContextInMessage{msg: wsEventMessageUnpack(b), connId: connId})
	}
}

func (r *wsEventRouter) callOpenHandler(c *gin.Context, connId uint64) (key string, params map[string]interface{}, err error) {
	defer func() {
		if err1 := recover(); err1 != nil {
			err = Recover2Error(err1)
			stack := Stack(3, 5)
			Logger.Errorf("[wsEventRouter][callOpenHandler] connId=%d,panic=%s\n%s", connId, err, stack)
		}
	}()
	return r.openHandler(c)
}

func (r *wsEventRouter) callEventHandler(ctx *wsEventContext, inMsg *WsEventMessage) (outMsg *WsEventMessage) {
	defer func() {
		if err := recover(); err != nil {
			outMsg = r.callErrorHandler(WsEventErrTypeEventPanic, Recover2Error(err))
		}
	}()
	handler, ok := r.eventHandlers[inMsg.Event]
	if !ok {
		outMsg = r.callErrorHandler(WsEventErrTypeNotfound, fmt.Errorf(""))
		return
	}
	return handler(ctx, inMsg)
}

func (r *wsEventRouter) callErrorHandler(errorType int, err error) *WsEventMessage {
	if r.errorHandler == nil {
		return nil
	}
	return r.errorHandler(errorType, err)
}

func (r *wsEventRouter) callWriteFailHandler(key string, msg *WsEventMessage, err error) {
	if r.writeFailHandler == nil {
		return
	}
	defer func() {
		if err := recover(); err != nil {
			stack := Stack(3, 5)
			Logger.Errorf("[wsEventRouter][callWriteFailHandler] err=%s\n%s", err, stack)
		}
	}()
	r.writeFailHandler(key, msg, err)
}

func (r *wsEventRouter) callCtxConnStartHandler(ctx *wsEventContext, connId uint64, restart bool) (err error) {
	if r.ctxConnStartHandler == nil {
		return nil
	}
	defer func() {
		if err1 := recover(); err1 != nil {
			err = Recover2Error(err1)
			stack := Stack(3, 5)
			ctx.logger.Errorf("[wsEventRouter][callCtxConnStartHandler] connId=%d,panic=%s\n%s", connId, err, stack)
		}
	}()
	r.ctxConnStartHandler(ctx, connId, restart)
	return nil
}

func (r *wsEventRouter) callCtxConnCloseHandler(ctx *wsEventContext, connId uint64) {
	if r.CtxConnCloseHandler == nil {
		return
	}
	defer func() {
		if err := recover(); err != nil {
			stack := Stack(3, 5)
			ctx.logger.Errorf("[wsEventRouter][callCtxConnCloseHandler] connId=%d,panic=%s\n%s", connId, err, stack)
		}
	}()
	r.CtxConnCloseHandler(ctx, connId)
}

func (r *wsEventRouter) callCtxStopHandler(ctx *wsEventContext) {
	if r.ctxStopHandler == nil {
		return
	}
	defer func() {
		if err := recover(); err != nil {
			stack := Stack(3, 5)
			ctx.logger.Errorf("[wsEventRouter][callCtxStopHandler] panic=%s\n%s", err, stack)
		}
	}()
	r.ctxStopHandler(ctx)
}
