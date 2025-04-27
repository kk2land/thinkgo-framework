package thinkgo

import (
	"github.com/gorilla/websocket"
	"sync"
	"time"
)

var wsEventMessageSep = []byte{','}

func wsEventLogger(logger FieldLogger, key string) FieldLogger {
	return logger.With("key", key)
}

func wsEventContextChannel(size int, defaultSize int) chan interface{} {
	if size == 0 {
		size = defaultSize
	}
	if size > 0 {
		return make(chan interface{}, size)
	} else {
		return make(chan interface{})
	}
}

type wsEventContextInConnStart struct {
	params map[string]interface{}
	conn   *websocket.Conn
	connId uint64
}

type wsEventContextInMessage struct {
	msg    *WsEventMessage
	connId uint64
}

type wsEventContextInPong struct {
	connId uint64
}

type wsEventContextInConnClose struct {
	connId uint64
}

type wsEventContextInStop struct {
	wait *sync.WaitGroup
}

type wsEventContext struct {
	key          string
	paramsString string
	router       *wsEventRouter
	routerParams interface{}
	inC          chan interface{} //websocket协程读取的channel
	sendC        chan interface{} //websocket-send协程读取的channel
	params       map[string]interface{}
	values       map[string]interface{}
	atime        time.Time
	logger       FieldLogger
}

func (c *wsEventContext) Key() string {
	return c.key
}

func (c *wsEventContext) ParamsString() string {
	return c.paramsString
}

func (c *wsEventContext) Logger() FieldLogger {
	return c.logger
}

func (c *wsEventContext) CtxSafeValueSet(k string, v interface{}) {
	c.values[k] = v
}

func (c *wsEventContext) CtxSafeValueGet(k string) (interface{}, bool) {
	val, ok := c.values[k]
	return val, ok
}

func (c *wsEventContext) CtxSafeParamGet(k string) (interface{}, bool) {
	val, ok := c.params[k]
	return val, ok
}

func (c *wsEventContext) Send(msg *WsEventMessage) {
	if err := SafeSendChannel[interface{}](c.sendC, msg); err != nil {
		c.logger.Errorf("[wsEventContext][Send] msg=%s,panic=%s", msg, err)
		c.router.callWriteFailHandler(c.key, msg, err)
	}
}

func (c *wsEventContext) SendForKeys(msg *WsEventMessage, keys ...string) {
	c.router.SendForKeys(msg, keys...)
}

func (c *wsEventContext) SendForParams(msg *WsEventMessage, paramsString string) {
	c.router.SendForParams(msg, paramsString)
}

func (c *wsEventContext) SendForAll(msg *WsEventMessage) {
	c.router.SendForAll(msg)
}

func (c *wsEventContext) SendForParamsExcludeKeys(msg *WsEventMessage, paramsString string, excludeKeys map[string]bool) {
	c.router.SendForParamsExcludeKeys(msg, paramsString, excludeKeys)
}

func (c *wsEventContext) SendForAllExcludeKeys(msg *WsEventMessage, excludeKeys map[string]bool) {
	c.router.SendForAllExcludeKeys(msg, excludeKeys)
}

func (c *wsEventContext) Close() {
	_ = SafeSendChannel[interface{}](c.inC, &wsEventContextInStop{})
}

func newWsEventContext(
	key string,
	paramsString string,
	router *wsEventRouter,
	routerParams interface{},
	logger FieldLogger,
) *wsEventContext {
	return &wsEventContext{
		key:          key,
		paramsString: paramsString,
		router:       router,
		routerParams: routerParams,
		inC:          wsEventContextChannel(router.readChannelSize, WsEventReadChannelSizeDefault),
		sendC:        wsEventContextChannel(router.writeChannelSize, WsEventWriteChannelSizeDefault),
		params:       nil,
		values:       make(map[string]interface{}),
		logger:       logger,
		atime:        time.Now(),
	}
}

func (c *wsEventContext) goSend() {
	var conn *websocket.Conn = nil
	var connId uint64

	tick := time.NewTicker(WsEventConnIdleTimeout / 3)
	defer func() {
		if conn != nil {
			_ = conn.Close()
		}
		tick.Stop()
	}()
loop:
	for {
		select {
		case sendData := <-c.sendC:
			if sendData == nil {
				break loop
			}
			switch d := sendData.(type) {
			case *wsEventContextInConnStart:
				if conn != nil {
					_ = conn.Close()
				}
				conn = d.conn
				connId = d.connId
			case *WsEventMessage:
				var err error = nil
				if conn == nil {
					err = WsEventErrorNoConn
				} else if err = wsEventMessageWrite(conn, d); err != nil {
					_ = conn.Close()
					conn = nil
				}
				if err != nil {
					c.logger.Errorf("[wsEventContext][goSend] err=%s", err)
					c.router.callWriteFailHandler(c.key, d, err)
				}
			case *wsEventContextInConnClose:
				if conn != nil && connId == d.connId {
					_ = conn.Close()
					conn = nil
				}
			case *wsEventContextInStop:
				//websocket协程发送
				c.logger.Infof("[wsEventContext][goSend] ctxStop")
				if d.wait != nil {
					d.wait.Done()
				}
				c.router.ctxWait.Done()
				break loop
			}
		case t := <-tick.C:
			if conn != nil {
				c.logger.Debugf("[wsEventContext][goSend] connId=%d,ping", connId)
				_ = conn.WriteControl(websocket.PingMessage, nil, t.Add(time.Second))
			}
		}
	}
}

func (c *wsEventContext) goStart() {
	defer close(c.sendC)
	defer close(c.inC)

	go c.goSend()
	firstConn := true
	tick := time.NewTicker(c.router.ctxKeepalive / 2)
	defer tick.Stop()

	var connIdSuccess uint64 = 0
	var connAccessTime = time.Now()
	var connCloseTime = time.Now()
loop:
	for {
		select {
		case <-c.router.done: //WsEventRouter.Close是触发
			if c.router.ctxMap.deleteCtx(c, time.Now()) {
				c.logger.Infof("[wsEventContext][goStart] ctxStop by msg")
				c.router.callCtxStopHandler(c)
				c.sendC <- &wsEventContextInStop{}
				break loop
			}
		case inData := <-c.inC:
			switch d := inData.(type) {
			case *wsEventContextInConnStart:
				c.sendC <- d
				c.params = d.params
				restart := !firstConn
				if firstConn {
					firstConn = false
				}
				c.logger.Debugf("[wsEventContext][goStart] connStart,connId=%d,restart=%t", d.connId, restart)
				if err := c.router.callCtxConnStartHandler(c, d.connId, restart); err != nil {
					if outMsg := c.router.callErrorHandler(WsEventErrTypeStartFail, err); outMsg != nil {
						c.sendC <- outMsg
					}
					c.sendC <- &wsEventContextInConnClose{connId: d.connId}
					connIdSuccess = 0
				} else {
					c.logger.Debugf("[wsEventContext][goStart] connStart success,connId=%d", d.connId)
					connIdSuccess = d.connId
					connAccessTime = time.Now()
				}
			case *wsEventContextInMessage:
				if connIdSuccess == d.connId {
					connAccessTime = time.Now()
					if outMsg := c.router.callEventHandler(c, d.msg); outMsg != nil {
						c.sendC <- outMsg
					}
				} else {
					c.logger.Warnf("[wsEventContext][goStart] 跳过msg,msg=%s,msgConnId=%d,currentConnId=%d", d.msg, d.connId, connIdSuccess)
				}
			case *wsEventContextInPong:
				if connIdSuccess == d.connId {
					c.logger.Debugf("[wsEventContext][goStart] connId=%d,pong", d.connId)
					connAccessTime = time.Now()
				}
			case *wsEventContextInConnClose:
				c.logger.Debugf("[wsEventContext][goStart] connClose,connId=%d", d.connId)
				if d.connId == connIdSuccess {
					connIdSuccess = 0
					connCloseTime = time.Now()
				}
				c.sendC <- d
				c.router.callCtxConnCloseHandler(c, d.connId)
			case *wsEventContextInStop:
				if c.router.ctxMap.deleteCtx(c, time.Now()) {
					c.logger.Infof("[wsEventContext][goStart] ctxStop by msg")
					c.router.callCtxStopHandler(c)
					c.sendC <- d
					break loop
				}
			}
		case t := <-tick.C:
			if connIdSuccess == 0 {
				if t.Sub(connCloseTime) > c.router.ctxKeepalive && c.router.ctxMap.deleteCtx(c, t) {
					c.logger.Infof("[wsEventContext][goStart] ctxStop by time")
					c.router.callCtxStopHandler(c)
					c.sendC <- &wsEventContextInStop{}
					break loop
				}
			} else if t.Sub(connAccessTime) > WsEventConnIdleTimeout {
				c.logger.Infof(
					"[wsEventContext][goStart] connClose,connId=%d,accessTime=%s,idleTimeout=%f",
					connIdSuccess,
					connAccessTime.Format(TimeFormatYmdHis),
					WsEventConnIdleTimeout.Seconds(),
				)
				c.sendC <- &wsEventContextInConnClose{connId: connIdSuccess}
			}
		}
	}
}
