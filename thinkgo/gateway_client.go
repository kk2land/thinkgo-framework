package thinkgo

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type GatewayClientId struct {
	LocalIP      net.IP
	LocalPort    uint16
	ConnectionId uint32
}

func GatewayClientIdToAddress(clientId string) (*GatewayClientId, error) {
	b, err := hex.DecodeString(clientId)
	if err != nil {
		return nil, err
	}
	localIP := binary.BigEndian.Uint32(b[0:4])
	localPort := binary.BigEndian.Uint16(b[4:6])
	connectionID := binary.BigEndian.Uint32(b[6:10])

	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, localIP)
	return &GatewayClientId{
		LocalIP:      ip,
		LocalPort:    localPort,
		ConnectionId: connectionID,
	}, nil
}

var gatewayRegisterEventWorkerConnect = []byte("{\"event\":\"worker_connect\",\"secret_key\":\"\"}\n")

type gatewayRegisterEventBroadcastAddresses struct {
	Event     string   `json:"event"`
	Addresses []string `json:"addresses"`
}

const (
	gatewayProtocolCmdSendToOne   = 5
	gatewayProtocolCmdSendToAll   = 6
	gatewayProtocolCmdSendToUID   = 14
	gatewayProtocolCmdSendToGroup = 22
)

type gatewayProtocol struct {
	Cmd          uint8
	LocalIP      net.IP
	LocalPort    uint16
	ClientIP     net.IP
	ClientPort   uint16
	ConnectionId uint32
	Flag         uint8
	GatewayPort  uint16
	ExtData      interface{}
	Body         []byte
}

func (m *gatewayProtocol) encode() []byte {
	var extData []byte
	if m.ExtData != nil {
		extData, _ = json.Marshal(m.ExtData)
	}
	size := 28 + len(extData) + len(m.Body)
	buf := make([]byte, 0, size)
	//NCNnNnNCnN
	buf = binary.BigEndian.AppendUint32(buf, uint32(size))
	buf = append(buf, m.Cmd)
	if len(m.LocalIP) > 0 {
		buf = append(buf, m.LocalIP.To4()...)
	} else {
		buf = binary.BigEndian.AppendUint32(buf, 0)
	}
	buf = binary.BigEndian.AppendUint16(buf, m.LocalPort)
	if len(m.ClientIP) > 0 {
		buf = append(buf, m.ClientIP.To4()...)
	} else {
		buf = binary.BigEndian.AppendUint32(buf, 0)
	}
	buf = binary.BigEndian.AppendUint16(buf, m.ClientPort)
	buf = binary.BigEndian.AppendUint32(buf, m.ConnectionId)
	buf = append(buf, m.Flag|1)
	buf = binary.BigEndian.AppendUint16(buf, m.GatewayPort)
	buf = binary.BigEndian.AppendUint32(buf, uint32(len(extData)))
	buf = append(buf, extData...)
	buf = append(buf, m.Body...)
	return buf
}

// GatewayClient 连接php的WorkerMan的gateway，向其发送消息
type GatewayClient struct {
	concurrency         int
	registerAddress     string
	timeout             time.Duration
	gatewayAddressesRef atomic.Pointer[gatewayRegisterEventBroadcastAddresses]
	lock                sync.RWMutex
	closed              atomic.Bool
	done                chan Void
	gatewayChannel      chan *gatewayProtocol
}

func NewGatewayClient(concurrency int, registerAddress string) *GatewayClient {
	return &GatewayClient{
		concurrency:     concurrency,
		registerAddress: registerAddress,
		timeout:         5 * time.Second,
		done:            make(chan Void),
		gatewayChannel:  make(chan *gatewayProtocol, 20),
	}
}

func (m *GatewayClient) withRLock(f func(closed bool)) bool {
	if m.closed.Load() {
		f(true)
		return true
	} else {
		m.lock.RLock()
		defer m.lock.RUnlock()
		closed := m.closed.Load()
		f(closed)
		return closed
	}
}

func (m *GatewayClient) getAllGatewayAddressesFromRegister() error {
	conn, err := net.DialTimeout("tcp", m.registerAddress, m.timeout)
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close()
	}()

	_, err = conn.Write(gatewayRegisterEventWorkerConnect)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(conn)
	var b []byte
	if b, err = reader.ReadBytes('\n'); err != nil {
		return err
	}
	Logger.Debugf("GatewayClient::getAllGatewayAddressesFromRegister-%s", b)

	var event *gatewayRegisterEventBroadcastAddresses
	if err = json.Unmarshal(b, &event); err != nil {
		return err
	} else if event.Event == "broadcast_addresses" {
		m.gatewayAddressesRef.Store(event)
	}
	return nil
}

func (m *GatewayClient) goRegister() {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()

	var fails = 0
loop:
	for {
		select {
		case <-m.done:
			break loop
		case <-tick.C:
			if err := m.getAllGatewayAddressesFromRegister(); err != nil {
				fails++
				if fails%20 == 1 {
					OpsAlarm("GatewayClient通过Register(%s)获取地址错误,err=%v,次数=%d",
						m.registerAddress, err, fails)
				}
			} else {
				fails = 0
			}
		}
	}
}

func (m *GatewayClient) goGateway() {
	conns := make(map[string]net.Conn)
	failCounts := make(map[string]int)
	timeout := 5 * time.Second

	addFailCount := func(address string, err error) {
		count := failCounts[address]
		count++
		failCounts[address] = count
		if count%20 == 5 {
			OpsAlarm("GatewayClient连接gateway进程失败,err=%v,次数=%d", err, count)
		}
	}

loop:
	for {
		select {
		case <-m.done:
			break loop

		case d := <-m.gatewayChannel:
			addresses := m.gatewayAddressesRef.Load()
			if addresses == nil {
				if conns != nil {
					for _, conn := range conns {
						_ = conn.Close()
					}
					conns = make(map[string]net.Conn)
				}
				continue
			} else {
				deletes := make(map[string]bool)
				for k, _ := range conns {
					deletes[k] = true
				}
				for _, v := range addresses.Addresses {
					if _, ok := conns[v]; !ok {
						if conn, err := net.DialTimeout("tcp", v, timeout); err != nil {
							addFailCount(v, err)
						} else {
							conns[v] = conn
							delete(failCounts, v)
						}
					} else {
						delete(deletes, v)
					}
				}
				for k, _ := range deletes {
					conn := conns[k]
					_ = conn.Close()
					delete(conns, k)
				}
			}

			b := d.encode()
			if len(d.LocalIP) > 0 {
				k := d.LocalIP.String()
				Logger.Debugf("GatewayClient::goGateway,send-%s", k)
				if conn, ok := conns[k]; ok {
					if _, err := conn.Write(b); err != nil {
						addFailCount(k, err)
						_ = conn.Close()
						delete(conns, k)
					}
				} else {
					Logger.Warnf("GatewayClient::goGateway,找不到gateway节点-%s", k)
				}
			} else {
				var deletes []string
				for k, conn := range conns {
					Logger.Debugf("GatewayClient::goGateway,send-%s", k)
					_ = conn.SetWriteDeadline(time.Now().Add(m.timeout))
					if _, err := conn.Write(b); err != nil {
						addFailCount(k, err)
						deletes = append(deletes, k)
					}
				}
				for _, k := range deletes {
					_ = conns[k].Close()
					delete(conns, k)
				}
			}
		}
	}
}

func (m *GatewayClient) goStart() {
	if err := m.getAllGatewayAddressesFromRegister(); err != nil {
		Logger.Errorf("GatewayClient通过Register(%s)获取地址错误,err=%v", m.registerAddress, err)
	}
	SafeGo(true, func() {
		m.goRegister()
	})
	for i := 0; i < m.concurrency; i++ {
		SafeGo(true, func() {
			m.goGateway()
		})
	}
}

func (m *GatewayClient) Start() {
	go m.goStart()
	AddShutdownHook(func(wait *sync.WaitGroup) {
		m.Close()
	})
}

func (m *GatewayClient) Close() {
	if m.closed.CompareAndSwap(false, true) {
		m.lock.Lock()
		defer m.lock.Unlock()
		close(m.done)
	}
}

func (m *GatewayClient) SendToGroup(message []byte, group ...string) {
	m.withRLock(func(closed bool) {
		if closed {
			return
		}
		m.gatewayChannel <- &gatewayProtocol{
			Cmd:  gatewayProtocolCmdSendToGroup,
			Body: message,
			ExtData: map[string]interface{}{
				"group":   group,
				"exclude": nil,
			},
		}
	})
}

func (m *GatewayClient) SendToUid(message []byte, uid ...string) {
	m.withRLock(func(closed bool) {
		if closed {
			return
		}
		m.gatewayChannel <- &gatewayProtocol{
			Cmd:     gatewayProtocolCmdSendToUID,
			Body:    message,
			ExtData: uid,
		}
	})
}
