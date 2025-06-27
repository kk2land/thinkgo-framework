package thinkgo

import (
	"sync"
	"sync/atomic"
)

// SendClose 对于一些需要send channel的服务类，可以帮助其关闭后，忽略send channel
type SendClose struct {
	lock sync.RWMutex
	wait atomic.Pointer[sync.WaitGroup]
	flag atomic.Bool
}

func (m *SendClose) SendWithRLock(f func()) {
	if m.flag.Load() {
		return
	}
	m.lock.RLock()
	defer m.lock.RUnlock()

	if !m.flag.Load() {
		f()
	}
}

func (m *SendClose) Close(wait *sync.WaitGroup, f func()) {
	if m.flag.CompareAndSwap(false, true) {
		m.lock.Lock()
		defer m.lock.Unlock()
		if wait != nil {
			m.wait.Store(wait)
		}
		f()
	}
}

func (m *SendClose) Done() {
	if wait := m.wait.Load(); wait != nil {
		wait.Done()
	}
}
