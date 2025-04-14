package thinkgo

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Leader，基于redis监听当前集群中当前进程是否是leader

type LeaderWatchHandler func(isLeader bool)

type Leader interface {
	Watch(f LeaderWatchHandler)
}

type LeaderRedis struct {
	name         string
	client       *RedisClient
	handlersMu   sync.Mutex
	handlers     []LeaderWatchHandler
	addStartHook bool
	done         chan Void
	state        int32
	stateCh      chan Void
}

func LeaderRedisNew(name string, client *RedisClient) *LeaderRedis {
	return &LeaderRedis{
		name:    name,
		client:  client,
		done:    make(chan Void),
		stateCh: make(chan Void, 1),
	}
}

func (l *LeaderRedis) Watch(f LeaderWatchHandler) {
	l.handlersMu.Lock()
	defer l.handlersMu.Unlock()
	if !l.addStartHook {
		l.addStartHook = true
		AddStartHook(l.start)
		AddShutdownHook(l.stop)
	}
	l.handlers = append(l.handlers, f)
}

func (l *LeaderRedis) start() {
	go l.goNotify()
	go l.goTick()
}

func (l *LeaderRedis) stop(wait *sync.WaitGroup) {
	close(l.done)
}

func (l *LeaderRedis) goNotify() {
	var lastState int32 = 0
	var callHandler = func(f LeaderWatchHandler, isLeader bool) {
		defer func() {
			if err := recover(); err != nil {
				stack := Stack(3, 5)
				Logger.Errorf("LeaderRedis(%s)调用LeaderWatchHandler panic=%s\n%s", l.name, err, stack)
			}
		}()
		f(isLeader)
	}
	for {
		select {
		case <-l.stateCh:
			state := atomic.LoadInt32(&l.state)
			if lastState == state {
				continue
			}
			l.handlersMu.Lock()
			for _, handler := range l.handlers {
				callHandler(handler, state == 1)
			}
			l.handlersMu.Unlock()
			lastState = state
		case <-l.done:
			break
		}
	}
}

func (l *LeaderRedis) goTick() {
	key := l.client.Prefix("tk-leader-" + l.name)
	token := fmt.Sprintf("%s-%d-%d", Hostname, Pid, time.Now().UnixNano()/1000000)
	ttl := 15 * time.Second
	call := func() {
		p := l.client.Raw().Pipeline()
		cmd1 := p.SetNX(context.Background(), key, token, ttl)
		cmd2 := p.Get(context.Background(), key)
		if _, err := p.Exec(context.Background()); err != nil {
			Logger.Errorf("LeaderRedis(%s)操作Redis错误,err=%s", l.name, err)
		} else {
			if cmd1.Val() || cmd2.Val() == token {
				atomic.StoreInt32(&l.state, 1)
			} else {
				atomic.StoreInt32(&l.state, 0)
			}
			select {
			case l.stateCh <- VoidValue:
			}
		}
	}
	call()

	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			call()
		case <-l.done:
			break
		}
	}
}
