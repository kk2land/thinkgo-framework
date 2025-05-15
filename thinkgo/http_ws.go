package thinkgo

import "C"
import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

const (
	HttpWsConnCloseFromNormal     = iota // HttpWsConn正常退出
	HttpWsConnCloseFromConnectErr        // HttpWsConn因为onConnect错误导致退出
	HttpWsConnCloseFromWriteErr          // HttpWsConn写错误导致退出
	HttpWsConnCloseFromReadErr           // HttpWsConn读错误导致退出
	HttpWsConnCloseFromMessageErr        // HttpWsConn的onMessage错误导致退出
	HttpWsConnCloseFromPingErr           // HttpWsConn发送ping错误导致退出
	HttpWsConnCloseFromNoPong            // HttpWsConn接收消息超时导致退出(包括pong消息)
	HttpWsConnCloseFromKeyReplace        // HttpWsConn因为相同key被顶掉导致退出
)

// HttpWsHandler HttpWsRouter需要的一些处理配置
type HttpWsHandler interface {
	// WriteChannelSize 写channel的容量
	WriteChannelSize() int
	// OnConnect 在http请求协程中调用
	OnConnect(c *gin.Context, wsConn *HttpWsConn) error
	// OnClose 确保只在write协程中调用
	OnClose(wsConn *HttpWsConn, from int, err error, writeNoNetErr func([]byte))
	// Marshal 序列化消息
	Marshal(msg interface{}) ([]byte, error)
	// Unmarshal 反序列化消息
	Unmarshal(b []byte) (interface{}, error)
}

var httpWsConnIdCounter atomic.Uint64
var httpWsConnGroupIdCounter atomic.Uint64
var httpWsConnPingInterval = 15 * time.Second
var httpWsConnReadDeadline = httpWsConnPingInterval * 2

type httpWsConnCloseData struct {
	from int
	err  error
}

type httpWsConnControlGroupData struct {
	op    int //1=append; 2=remove
	group *HttpWsConnGroup
}

type httpWsConnControlKeyData struct {
	key string
}

// HttpWsConn 封装的websocket connection
type HttpWsConn struct {
	logger         FieldLogger
	router         *HttpWsRouter
	conn           *websocket.Conn
	connId         uint64
	ctxLock        sync.RWMutex
	ctx            map[string]interface{}
	writeLock      sync.RWMutex
	writeClosed    atomic.Bool
	writeChannel   chan []byte
	controlChannel chan interface{}
	closeChannel   chan *httpWsConnCloseData
}

func newHttpWsConn(logger FieldLogger, router *HttpWsRouter, c *gin.Context, conn *websocket.Conn) *HttpWsConn {
	connId := httpWsConnIdCounter.Add(1)
	wsConn := &HttpWsConn{
		logger:         logger.With("wsConn", connId),
		router:         router,
		conn:           conn,
		connId:         connId,
		ctx:            make(map[string]interface{}),
		writeChannel:   make(chan []byte, router.handler.WriteChannelSize()),
		controlChannel: make(chan interface{}),
		closeChannel:   make(chan *httpWsConnCloseData, 1),
	}
	wsConn.ctx["RemoteAddr"] = c.Request.RemoteAddr
	return wsConn
}

func (m *HttpWsConn) Logger() FieldLogger {
	return m.logger
}

func (m *HttpWsConn) Router() *HttpWsRouter {
	return m.router
}

// ConnId 获取wsConn内存全局id
func (m *HttpWsConn) ConnId() uint64 {
	return m.connId
}

// CtxLoad 读取业务key/value
func (m *HttpWsConn) CtxLoad(key string) (val interface{}, ok bool) {
	m.ctxLock.RLock()
	m.ctxLock.RUnlock()
	val, ok = m.ctx[key]
	return
}

// CtxStore 写入业务key/value
func (m *HttpWsConn) CtxStore(key string, val interface{}) {
	m.ctxLock.Lock()
	m.ctxLock.Unlock()
	m.ctx[key] = val
}

// CtxDelete 删除业务key/value
func (m *HttpWsConn) CtxDelete(key string) {
	m.ctxLock.Lock()
	m.ctxLock.Unlock()
	delete(m.ctx, key)
}

// CtxLock 业务key/value上写锁操作
func (m *HttpWsConn) CtxLock(f func(ctx map[string]interface{})) {
	m.ctxLock.Lock()
	m.ctxLock.Unlock()
	f(m.ctx)
}

// CtxRLock 业务key/value上读锁操作
func (m *HttpWsConn) CtxRLock(f func(ctx map[string]interface{})) {
	m.ctxLock.RLock()
	m.ctxLock.RUnlock()
	f(m.ctx)
}

// Write 发送消息
func (m *HttpWsConn) Write(msg any) error {
	if b, err := m.router.Marshal(msg); err != nil {
		return err
	} else {
		m.WriteBytes(b)
		return nil
	}
}

func (m *HttpWsConn) writeControl(data interface{}) bool {
	if m.writeClosed.Load() {
		return false
	}
	m.writeLock.RLock()
	defer m.writeLock.RUnlock()
	if !m.writeClosed.Load() {
		m.controlChannel <- data
		return true
	} else {
		return false
	}
}

// WriteBytes 发送消息
func (m *HttpWsConn) WriteBytes(b []byte) {
	if m.writeClosed.Load() {
		return
	}
	m.writeLock.RLock()
	defer m.writeLock.RUnlock()
	if !m.writeClosed.Load() {
		m.writeChannel <- b
	}
}

// TryWriteBytes 尝试写入
func (m *HttpWsConn) TryWriteBytes(b []byte) {
	select {
	case m.writeChannel <- b:
	}
}

// Close 关闭wsConn
func (m *HttpWsConn) Close() {
	if m.writeClosed.CompareAndSwap(false, true) {
		close(m.closeChannel)
	}
}

func (m *HttpWsConn) RemoteAddr() string {
	remoteAddr, _ := m.CtxLoad("RemoteAddr")
	return remoteAddr.(string)
}

func (m *HttpWsConn) Key() string {
	if key, ok := m.CtxLoad("Key"); ok {
		return key.(string)
	} else {
		return ""
	}
}

func (m *HttpWsConn) String() string {
	return fmt.Sprintf("wsConn=%d,remote_addr=%s", m.connId, m.RemoteAddr())
}

func (m *HttpWsConn) start() {
	go func() {
		tick := time.NewTicker(httpWsConnPingInterval)
		defer tick.Stop()

		writeNoNetErr := func(b []byte) {
			_ = m.conn.WriteMessage(websocket.TextMessage, b)
		}
		var key string
		var groups = make(map[uint64]*HttpWsConnGroup)
	loop:
		for {
			select {
			case data := <-m.closeChannel:
				if data == nil {
					m.logger.Infof("正常退出")
					m.router.handler.OnClose(m, HttpWsConnCloseFromNormal, nil, writeNoNetErr)
				} else {
					m.logger.Warnf("异常退出-%d,%v", data.from, data.err)
					switch data.from {
					// 以下close类型，在关闭前可以发送数据到客户端
					case HttpWsConnCloseFromConnectErr,
						HttpWsConnCloseFromMessageErr,
						HttpWsConnCloseFromKeyReplace:
						m.router.handler.OnClose(m, data.from, data.err, writeNoNetErr)
					default:
						m.router.handler.OnClose(m, data.from, data.err, nil)
					}
				}
				break loop

			case data := <-m.controlChannel:
				switch d := data.(type) {
				case *httpWsConnControlGroupData:
					if d.op == 1 {
						groups[d.group.groupId] = d.group
					} else {
						delete(groups, d.group.groupId)
					}
				case *httpWsConnControlKeyData:
					key = d.key
					m.CtxStore("Key", key)
				}

			case b := <-m.writeChannel:
				if err := m.conn.WriteMessage(websocket.TextMessage, b); err != nil {
					m.logger.Errorf("write fail-%v,%s", err, b)
					m.router.handler.OnClose(m, HttpWsConnCloseFromWriteErr, err, nil)
					break loop
				}

			case <-tick.C:
				if err := m.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					m.logger.Errorf("ping fail-%v", err)
					m.router.handler.OnClose(m, HttpWsConnCloseFromPingErr, err, nil)
					break loop
				}
			}
		}
		m.logger.Infof("开始退出write协程,key=%s", key)
		//启动一个协程来消耗writeChannel/controlChannel
		go func() {
			for {
				select {
				case d := <-m.writeChannel:
					if d == nil {
						return
					}
				case d := <-m.controlChannel:
					if d == nil {
						return
					}
				}
			}
		}()
		//上写锁来设置write已经关闭
		m.writeLock.Lock()
		m.writeClosed.Store(true)
		close(m.writeChannel)
		close(m.controlChannel)
		m.writeLock.Unlock()
		//将conn从group中移除
		for _, group := range groups {
			group.Delete(m)
		}
		//将wsConn从router中移除
		m.router.closeConn(key, m)
		_ = m.conn.Close()
		m.logger.Infof("结束退出write协程,key=%s", key)
	}()
}

func (m *HttpWsConn) close(from int, err error) {
	select {
	case m.closeChannel <- &httpWsConnCloseData{from, err}:
	}
}

// HttpWsConnGroup 创建一个wsConn的组，可以遍历组内全部的wsConn
type HttpWsConnGroup struct {
	groupId uint64
	conns   *CMap[uint64, *HttpWsConn]
}

func NewHttpWsConnGroup() *HttpWsConnGroup {
	return &HttpWsConnGroup{
		groupId: httpWsConnGroupIdCounter.Add(1),
		conns:   NewCMapUint64[*HttpWsConn](),
	}
}

func (m *HttpWsConnGroup) Append(wsConn *HttpWsConn) {
	m.conns.GetShard(wsConn.connId).Lock(func(items map[uint64]*HttpWsConn) {
		if wsConn.writeControl(&httpWsConnControlGroupData{1, m}) {
			items[wsConn.connId] = wsConn
		}
	})
}

func (m *HttpWsConnGroup) Delete(wsConn *HttpWsConn) {
	m.conns.GetShard(wsConn.connId).Lock(func(items map[uint64]*HttpWsConn) {
		delete(items, wsConn.connId)
		_ = wsConn.writeControl(&httpWsConnControlGroupData{2, m})
	})
}

func (m *HttpWsConnGroup) Range(f func(connId uint64, wsConn *HttpWsConn) bool) {
	m.conns.Range(f)
}

func (m *HttpWsConnGroup) Write(b []byte) {
	m.Range(func(connId uint64, wsConn *HttpWsConn) bool {
		wsConn.WriteBytes(b)
		return true
	})
}

func (m *HttpWsConnGroup) TryWrite(b []byte) {
	m.Range(func(connId uint64, wsConn *HttpWsConn) bool {
		wsConn.TryWriteBytes(b)
		return true
	})
}

// HttpWsRouter 在http的handler中升级成websocket的封装
type HttpWsRouter struct {
	handler  HttpWsHandler
	conns    *CMap[uint64, *HttpWsConn]
	keyConns *CMap[string, *HttpWsConn]
}

func NewHttpWsRouter(handler HttpWsHandler) *HttpWsRouter {
	router := &HttpWsRouter{
		handler:  handler,
		conns:    NewCMapUint64[*HttpWsConn](),
		keyConns: NewCMapString[*HttpWsConn](),
	}
	AddShutdownHook(func(wait *sync.WaitGroup) {
		router.conns.Range(func(k uint64, v *HttpWsConn) bool {
			v.Close()
			return true
		})
	})
	return router
}

func (m *HttpWsRouter) closeConn(key string, wsConn *HttpWsConn) {
	if key != "" {
		m.keyConns.GetShard(key).Lock(func(items map[string]*HttpWsConn) {
			if wsConn1, ok1 := items[key]; ok1 && wsConn1.connId == wsConn.connId {
				delete(items, key)
			}
		})
	}
	m.conns.Delete(wsConn.connId)
}

func (m *HttpWsRouter) Marshal(msg interface{}) ([]byte, error) {
	return m.handler.Marshal(msg)
}

func (m *HttpWsRouter) Unmarshal(b []byte) (interface{}, error) {
	return m.handler.Unmarshal(b)
}

// KeyConnBind 可以将wsConn绑定一个key，
func (m *HttpWsRouter) KeyConnBind(key string, wsConn *HttpWsConn, replace bool) (swapped bool) {
	if key1 := wsConn.Key(); key1 != "" && key1 != key {
		//不支持对wsConn绑定不同的key
		return false
	}
	m.keyConns.GetShard(key).Lock(func(items map[string]*HttpWsConn) {
		if wsConn1, ok1 := items[key]; ok1 {
			//如果存在，并且不是同个wsConn
			if wsConn1.connId != wsConn.connId {
				if !replace {
					//存在，并且不替换，则直接return
					return
				}
				//旧的wsConn通知退出
				wsConn1.close(HttpWsConnCloseFromKeyReplace, fmt.Errorf("replace by %s", wsConn1.String()))
				//新的wsConn设置key
				if wsConn.writeControl(&httpWsConnControlKeyData{key}) {
					items[key] = wsConn
					swapped = true
				}
			}
		} else {
			//如果不存在，则给新的wsConn设置key
			if wsConn.writeControl(&httpWsConnControlKeyData{key}) {
				items[key] = wsConn
				swapped = true
			}
		}
	})
	wsConn.logger.Infof("绑定key-%s,%t", key, swapped)
	return
}

func (m *HttpWsRouter) KeyConnLoad(key string) (wsConn *HttpWsConn) {
	wsConn, _ = m.keyConns.Load(key)
	return
}

func (m *HttpWsRouter) KeyConnRange(f func(key string, wsConn *HttpWsConn) bool) {
	m.keyConns.Range(f)
}

func (m *HttpWsRouter) Handle(c *gin.Context, onMessage func(wsConn *HttpWsConn, msg interface{}) error) error {
	logger := HttpLogger(c)
	up := &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn, err := up.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Errorf("Upgrade fail - %s", err)
		return err
	}
	if err = conn.SetReadDeadline(time.Now().Add(httpWsConnReadDeadline)); err != nil {
		logger.Errorf("SetReadDeadline fail - %s", err)
		return err
	}
	conn.SetPongHandler(func(appData string) error {
		return conn.SetReadDeadline(time.Now().Add(httpWsConnReadDeadline))
	})

	wsConn := newHttpWsConn(logger, m, c, conn)
	m.conns.Store(wsConn.connId, wsConn)
	wsConn.start()
	if err = m.handler.OnConnect(c, wsConn); err != nil {
		wsConn.close(HttpWsConnCloseFromConnectErr, err)
		return err
	}

	wsConn.logger.Infof("开始ReadMessage")
	var b []byte
	var msg interface{}
	for {
		_, b, err = conn.ReadMessage()
		if err != nil {
			logger.Errorf("ReadMessage fail,err=%s", err)
			if ErrIsTimeout(err) {
				wsConn.close(HttpWsConnCloseFromNoPong, err)
			} else {
				wsConn.close(HttpWsConnCloseFromReadErr, err)
			}
			return err
		}
		//收到消息就重置read deadline
		_ = conn.SetReadDeadline(time.Now().Add(httpWsConnReadDeadline))
		logger.Debugf("conn=%d,ReadMessage=%s", wsConn.connId, b)
		if msg, err = m.handler.Unmarshal(b); err != nil {
			logger.Warnf("Unmarshal error-%v,%s", err, b)
			b = nil
			continue
		}
		if err = onMessage(wsConn, msg); err != nil {
			logger.Errorf("onMessage fail,err=%v,%s", err, b)
			wsConn.close(HttpWsConnCloseFromMessageErr, err)
			return err
		}
	}
}
