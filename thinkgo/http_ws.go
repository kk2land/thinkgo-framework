package thinkgo

import "C"
import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	HttpWsConnCloseFromNormal     = iota // HttpWsConn正常退出
	HttpWsConnCloseFromWriteErr          // HttpWsConn写错误导致退出
	HttpWsConnCloseFromReadErr           // HttpWsConn读错误导致退出
	HttpWsConnCloseFromMessageErr        // HttpWsConn的onMessage错误导致退出
	HttpWsConnCloseFromPingErr           // HttpWsConn发送ping错误导致退出
	HttpWsConnCloseFromNoPong            // HttpWsConn接收消息超时导致退出(包括pong消息)
	HttpWsConnCloseFromKeyReplace        // HttpWsConn因为相同key被顶掉导致退出
	HttpWsConnCloseFromClient            // HttpWsConn因为客户端关闭导致退出
)

const httpWsConnPingInterval = 15 * time.Second
const httpWsConnReadDeadline = httpWsConnPingInterval * 2

type HttpWsConnWritePing func(conn *websocket.Conn) error

func HttpWsJsonMarshal(msg interface{}) ([]byte, error) {
	return json.Marshal(msg)
}

var httpWsConnIdCounter atomic.Uint64
var httpWsConnGroupIdCounter atomic.Uint64

type httpWsConnCloseData struct {
	from int
	err  error
}

type httpWsConnControlGroupData[T any] struct {
	op    int //1=append; 2=remove
	group *HttpWsConnGroup[T]
}

type httpWsConnControlKeyData struct {
	key string
}

func httpWsConnOnMessage[T any](wsConn *HttpWsConn[T], msg T) error {
	return nil
}

func httpWsConnOnClose[T any](wsConn *HttpWsConn[T], from int, err error, writeNoNetErr func([]byte)) {
}

func httpWsConnWritePing(conn *websocket.Conn) error {
	return conn.WriteMessage(websocket.PingMessage, nil)
}

// HttpWsConn 封装的websocket connection
type HttpWsConn[T any] struct {
	logger          FieldLogger
	router          *HttpWsRouter[T]
	conn            *websocket.Conn
	connId          uint64
	ctxLock         sync.RWMutex
	ctx             map[string]interface{}
	writeLock       sync.RWMutex
	writeClosed     atomic.Bool
	writeChannel    chan []byte
	controlChannel  chan interface{}
	closeChannel    chan *httpWsConnCloseData
	onMessage       func(wsConn *HttpWsConn[T], msg T) error
	onClose         func(wsConn *HttpWsConn[T], from int, err error, writeNoNetErr func([]byte))
	writePing       HttpWsConnWritePing
	writePingCustom bool
	pingInterval    time.Duration
	readDeadline    time.Duration
}

func newHttpWsConn[T any](logger FieldLogger, router *HttpWsRouter[T], c *gin.Context, conn *websocket.Conn) *HttpWsConn[T] {
	connId := httpWsConnIdCounter.Add(1)
	wsConn := &HttpWsConn[T]{
		logger:         logger.With("wsConn", connId),
		router:         router,
		conn:           conn,
		connId:         connId,
		ctx:            make(map[string]interface{}),
		writeChannel:   make(chan []byte, router.writeChannelSize),
		controlChannel: make(chan interface{}),
		closeChannel:   make(chan *httpWsConnCloseData, 1),
		onMessage:      httpWsConnOnMessage[T],
		onClose:        httpWsConnOnClose[T],
		writePing:      httpWsConnWritePing,
		pingInterval:   httpWsConnPingInterval,
		readDeadline:   httpWsConnReadDeadline,
	}

	_, clientPort, _ := net.SplitHostPort(c.Request.RemoteAddr)
	wsConn.ctx["RemoteAddr"] = c.ClientIP() + ":" + clientPort
	return wsConn
}

func (m *HttpWsConn[T]) Logger() FieldLogger {
	return m.logger
}

func (m *HttpWsConn[T]) Router() *HttpWsRouter[T] {
	return m.router
}

// ConnId 获取wsConn内存全局id
func (m *HttpWsConn[T]) ConnId() uint64 {
	return m.connId
}

// OnMessage 设置消息回调函数，必须在Start之前调用
func (m *HttpWsConn[T]) OnMessage(f func(wsConn *HttpWsConn[T], msg T) error) {
	m.onMessage = f
}

// OnClose 设置链接断开回调函数，必须在Start之前调用
func (m *HttpWsConn[T]) OnClose(f func(wsConn *HttpWsConn[T], from int, err error, writeNoNetErr func([]byte))) {
	m.onClose = f
}

// WritePing 设置自定义发送ping函数，必须在Start之前调用
func (m *HttpWsConn[T]) WritePing(f HttpWsConnWritePing) {
	m.writePing = f
	m.writePingCustom = true
}

// SetPingInterval 设置发送ping的时间间隔，必须在Start之前调用
func (m *HttpWsConn[T]) SetPingInterval(interval time.Duration) {
	m.pingInterval = interval
}

// SetReadDeadline 设置读取消息超时时间，必须在Start之前调用
func (m *HttpWsConn[T]) SetReadDeadline(deadline time.Duration) {
	m.readDeadline = deadline
}

// CtxLoad 读取业务key/value
func (m *HttpWsConn[T]) CtxLoad(key string) (val interface{}, ok bool) {
	m.ctxLock.RLock()
	m.ctxLock.RUnlock()
	val, ok = m.ctx[key]
	return
}

// CtxStore 写入业务key/value
func (m *HttpWsConn[T]) CtxStore(key string, val interface{}) {
	m.ctxLock.Lock()
	m.ctxLock.Unlock()
	m.ctx[key] = val
}

// CtxDelete 删除业务key/value
func (m *HttpWsConn[T]) CtxDelete(key string) {
	m.ctxLock.Lock()
	m.ctxLock.Unlock()
	delete(m.ctx, key)
}

// CtxLock 业务key/value上写锁操作
func (m *HttpWsConn[T]) CtxLock(f func(ctx map[string]interface{})) {
	m.ctxLock.Lock()
	m.ctxLock.Unlock()
	f(m.ctx)
}

// CtxRLock 业务key/value上读锁操作
func (m *HttpWsConn[T]) CtxRLock(f func(ctx map[string]interface{})) {
	m.ctxLock.RLock()
	m.ctxLock.RUnlock()
	f(m.ctx)
}

// Write 发送消息
func (m *HttpWsConn[T]) Write(msg any) error {
	if b, err := m.router.Marshal(msg); err != nil {
		return err
	} else {
		m.WriteBytes(b)
		return nil
	}
}

func (m *HttpWsConn[T]) writeControl(data interface{}) bool {
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
func (m *HttpWsConn[T]) WriteBytes(b []byte) {
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
func (m *HttpWsConn[T]) TryWriteBytes(b []byte) {
	select {
	case m.writeChannel <- b:
	default:
	}
}

// Close 关闭wsConn
func (m *HttpWsConn[T]) Close() {
	if m.writeClosed.CompareAndSwap(false, true) {
		m.writeLock.Lock()
		close(m.closeChannel)
		m.writeLock.Unlock()
	}
}

func (m *HttpWsConn[T]) RemoteAddr() string {
	remoteAddr, _ := m.CtxLoad("RemoteAddr")
	return remoteAddr.(string)
}

// SetName 给wsConn设置名称，方便日志定位，不同于key，name不会排他性
func (m *HttpWsConn[T]) SetName(name string) {
	m.CtxStore("Name", name)
}

func (m *HttpWsConn[T]) Name() string {
	if tmp1, ok := m.CtxLoad("Name"); ok {
		return tmp1.(string)
	} else {
		return ""
	}
}

func (m *HttpWsConn[T]) Key() string {
	if key, ok := m.CtxLoad("Key"); ok {
		return key.(string)
	} else {
		return ""
	}
}

func (m *HttpWsConn[T]) String() string {
	return fmt.Sprintf("wsConn=%d,remote_addr=%s", m.connId, m.RemoteAddr())
}

func (m *HttpWsConn[T]) goWrite() {
	tick := time.NewTicker(m.pingInterval)
	defer tick.Stop()

	writeNoNetErr := func(b []byte) {
		_ = m.conn.WriteMessage(websocket.TextMessage, b)
	}
	var key string
	var groups = make(map[uint64]*HttpWsConnGroup[T])
	var suffix = func() string {
		if name := m.Name(); name != "" {
			return "key=" + key + ",name=" + name
		} else {
			return "key=" + key
		}
	}

loop:
	for {
		select {
		case data := <-m.closeChannel:
			if data == nil {
				m.logger.Debugf("正常退出-%s", suffix())
				m.onClose(m, HttpWsConnCloseFromNormal, nil, writeNoNetErr)
			} else {
				if data.from == HttpWsConnCloseFromClient {
					m.logger.Debugf("客户端退出-%s", suffix())
				} else {
					m.logger.Warnf("异常退出-%d,%v,%s", data.from, data.err, suffix())
				}
				switch data.from {
				// 以下close类型，在关闭前可以发送数据到客户端
				case HttpWsConnCloseFromMessageErr, HttpWsConnCloseFromKeyReplace:
					m.onClose(m, data.from, data.err, writeNoNetErr)
				default:
					m.onClose(m, data.from, data.err, nil)
				}
			}
			break loop

		case data := <-m.controlChannel:
			switch d := data.(type) {
			case *httpWsConnControlGroupData[T]:
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
			m.logger.Debugf("WriteMessage=%s", b)
			if err := m.conn.WriteMessage(websocket.TextMessage, b); err != nil {
				m.logger.Errorf("write fail-%v,%s,%s", err, b, suffix())
				m.onClose(m, HttpWsConnCloseFromWriteErr, err, nil)
				break loop
			}

		case <-tick.C:
			if err := m.writePing(m.conn); err != nil {
				m.logger.Errorf("ping fail-%v,%s", err, suffix())
				m.onClose(m, HttpWsConnCloseFromPingErr, err, nil)
				break loop
			}
		}
	}
	m.logger.Debugf("开始退出write协程-%s", suffix())
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
	m.logger.Debugf("结束退出write协程-%s", suffix())
}

func (m *HttpWsConn[T]) Start() error {
	var err error
	//设置读取超时时间
	if err = m.conn.SetReadDeadline(time.Now().Add(m.readDeadline)); err != nil {
		m.logger.Errorf("SetReadDeadline fail - %s", err)
		return err
	}
	//如果没有自定义writePing，则设置自动响应pong帧
	if !m.writePingCustom {
		m.conn.SetPongHandler(func(appData string) error {
			return m.conn.SetReadDeadline(time.Now().Add(m.readDeadline))
		})
	}

	//启动write协程
	go m.goWrite()

	m.logger.Debug("开始ReadMessage")
	var b []byte
	var msg T

	for {
		_, b, err = m.conn.ReadMessage()
		if err != nil {
			if err1, ok1 := err.(*websocket.CloseError); ok1 {
				m.close(HttpWsConnCloseFromClient, err)
				return err1
			}
			//m.logger.Errorf("ReadMessage fail,err=%s", err)
			if ErrIsTimeout(err) {
				m.close(HttpWsConnCloseFromNoPong, err)
			} else {
				m.close(HttpWsConnCloseFromReadErr, err)
			}
			return err
		}
		//收到消息就重置read deadline
		_ = m.conn.SetReadDeadline(time.Now().Add(m.readDeadline))
		m.logger.Debugf("ReadMessage=%s", b)
		if msg, err = m.router.Unmarshal(b); err != nil {
			m.logger.Warnf("Unmarshal error-%v,%s", err, b)
			b = nil
			continue
		}
		if err = m.onMessage(m, msg); err != nil {
			m.logger.Errorf("onMessage fail,err=%v,%s", err, b)
			m.close(HttpWsConnCloseFromMessageErr, err)
			return err
		}
	}
}

func (m *HttpWsConn[T]) close(from int, err error) {
	if m.writeClosed.Load() {
		return
	}

	m.writeLock.RLock()
	defer m.writeLock.RUnlock()

	if m.writeClosed.Load() {
		return
	}
	select {
	case m.closeChannel <- &httpWsConnCloseData{from, err}:
	default:
	}
}

// HttpWsConnGroup 创建一个wsConn的组，可以遍历组内全部的wsConn
type HttpWsConnGroup[T any] struct {
	groupId uint64
	conns   *CMap[uint64, *HttpWsConn[T]]
}

func NewHttpWsConnGroup[T any]() *HttpWsConnGroup[T] {
	return &HttpWsConnGroup[T]{
		groupId: httpWsConnGroupIdCounter.Add(1),
		conns:   NewCMapUint64[*HttpWsConn[T]](),
	}
}

func (m *HttpWsConnGroup[T]) Append(wsConn *HttpWsConn[T]) {
	m.conns.GetShard(wsConn.connId).Lock(func(items map[uint64]*HttpWsConn[T]) {
		if wsConn.writeControl(&httpWsConnControlGroupData[T]{1, m}) {
			items[wsConn.connId] = wsConn
		}
	})
}

func (m *HttpWsConnGroup[T]) Delete(wsConn *HttpWsConn[T]) {
	m.conns.GetShard(wsConn.connId).Lock(func(items map[uint64]*HttpWsConn[T]) {
		delete(items, wsConn.connId)
		_ = wsConn.writeControl(&httpWsConnControlGroupData[T]{2, m})
	})
}

func (m *HttpWsConnGroup[T]) Range(f func(connId uint64, wsConn *HttpWsConn[T]) bool) {
	m.conns.Range(f)
}

func (m *HttpWsConnGroup[T]) Write(b []byte) {
	m.Range(func(connId uint64, wsConn *HttpWsConn[T]) bool {
		wsConn.WriteBytes(b)
		return true
	})
}

func (m *HttpWsConnGroup[T]) TryWrite(b []byte) {
	m.Range(func(connId uint64, wsConn *HttpWsConn[T]) bool {
		wsConn.TryWriteBytes(b)
		return true
	})
}

// HttpWsRouter 在http的handler中升级成websocket的封装
type HttpWsRouter[T any] struct {
	writeChannelSize int
	marshal          func(msg interface{}) ([]byte, error)
	unmarshal        func(b []byte) (T, error)
	conns            *CMap[uint64, *HttpWsConn[T]]
	keyConns         *CMap[string, *HttpWsConn[T]]
}

// NewHttpWsRouter 先全局创建一个HttpWsRouter，然后使用它的Create()函数来获取HttpWsConn
func NewHttpWsRouter[T any](
	writeChannelSize int,
	marshal func(msg interface{}) ([]byte, error),
	unmarshal func(b []byte) (T, error),
) *HttpWsRouter[T] {
	router := &HttpWsRouter[T]{
		writeChannelSize: writeChannelSize,
		marshal:          marshal,
		unmarshal:        unmarshal,
		conns:            NewCMapUint64[*HttpWsConn[T]](),
		keyConns:         NewCMapString[*HttpWsConn[T]](),
	}
	AddShutdownHook(func(wait *sync.WaitGroup) {
		router.conns.Range(func(k uint64, v *HttpWsConn[T]) bool {
			v.Close()
			return true
		})
	})
	return router
}

func (m *HttpWsRouter[T]) closeConn(key string, wsConn *HttpWsConn[T]) {
	if key != "" {
		m.keyConns.GetShard(key).Lock(func(items map[string]*HttpWsConn[T]) {
			if wsConn1, ok1 := items[key]; ok1 && wsConn1.connId == wsConn.connId {
				delete(items, key)
			}
		})
	}
	m.conns.Delete(wsConn.connId)
}

func (m *HttpWsRouter[T]) Marshal(msg interface{}) ([]byte, error) {
	return m.marshal(msg)
}

func (m *HttpWsRouter[T]) Unmarshal(b []byte) (T, error) {
	return m.unmarshal(b)
}

// KeyConnBind 可以将wsConn绑定一个key，如果key相同，可以顶掉其他已经绑定的wsConn
func (m *HttpWsRouter[T]) KeyConnBind(key string, wsConn *HttpWsConn[T], replace bool) (swapped bool) {
	if key1 := wsConn.Key(); key1 != "" && key1 != key {
		//不支持对wsConn绑定不同的key
		return false
	}
	m.keyConns.GetShard(key).Lock(func(items map[string]*HttpWsConn[T]) {
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

func (m *HttpWsRouter[T]) KeyConnLoad(key string) (wsConn *HttpWsConn[T]) {
	wsConn, _ = m.keyConns.Load(key)
	return
}

func (m *HttpWsRouter[T]) KeyConnRange(f func(key string, wsConn *HttpWsConn[T]) bool) {
	m.keyConns.Range(f)
}

func (m *HttpWsRouter[T]) Create(c *gin.Context) (*HttpWsConn[T], error) {
	logger := HttpLogger(c)
	up := &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn, err := up.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Errorf("Upgrade fail - %s", err)
		return nil, err
	}
	wsConn := newHttpWsConn(logger, m, c, conn)
	m.conns.Store(wsConn.connId, wsConn)
	return wsConn, nil
}
