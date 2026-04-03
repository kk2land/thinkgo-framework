package thinkgo

import (
	"sync"
	"sync/atomic"
)

// GoQueue 将一些不重要需要上锁的操作改成使用go + channel的方式
type GoQueue struct {
	ch      chan interface{}
	state   int32
	waitRef atomic.Pointer[sync.WaitGroup]
	handler func(obj interface{}) //保证单协程中运行
}

func NewGoQueue(size int, handler func(obj interface{})) *GoQueue {
	//wait := &sync.WaitGroup{}
	//wait.Add(1)
	return &GoQueue{
		ch:    make(chan interface{}, size),
		state: 0,
		//wait:    wait,
		handler: handler,
	}
}

func (g *GoQueue) Start() {
	if !atomic.CompareAndSwapInt32(&g.state, 0, 1) {
		//already flushStart or close
		return
	}
	go func() {
	loop:
		for {
			select {
			case obj := <-g.ch:
				if obj == nil {
					break loop
				}
				g.doHandle(obj)
			}
		}
		if wait := g.waitRef.Swap(nil); wait != nil {
			wait.Done()
		}
	}()
}

func (g *GoQueue) TrySend(obj interface{}) {
	if atomic.LoadInt32(&g.state) != 2 {
		select {
		case g.ch <- obj:
		default:
		}
	}
}

func (g *GoQueue) Send(obj interface{}) {
	if atomic.LoadInt32(&g.state) != 2 {
		defer func() {
			_ = recover()
		}()
		g.ch <- obj
	}
}

func (g *GoQueue) Close() {
	if atomic.CompareAndSwapInt32(&g.state, 1, 2) {
		close(g.ch)
	}
}

func (g *GoQueue) CloseAndWait(wait *sync.WaitGroup) {
	if atomic.CompareAndSwapInt32(&g.state, 1, 2) {
		if wait != nil {
			wait.Add(1)
			g.waitRef.Store(wait)
		}
		close(g.ch)
	}
}

func (g *GoQueue) doHandle(obj interface{}) {
	defer func() {
		if err := recover(); err != nil {
			Logger.Errorf("[GoQueue] doHandle fail - %s", err)
		}
	}()
	g.handler(obj)
}
