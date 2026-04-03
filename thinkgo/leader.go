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

// Leader 多个进程同时监听，抢占哪个是leader
type Leader interface {
	// Watch 注册回调函数，当leader有变化时，会触发回调
	Watch(f LeaderWatchHandler)
	// Online 重新参与leader选举
	Online()
	// Offline 退出leader选举
	Offline()
}

// LeaderRedis 基于redis实现的Leader
type LeaderRedis struct {
	name         string
	id           int
	client       *RedisClient
	handlersMu   sync.Mutex
	handlers     []LeaderWatchHandler
	addStartHook bool
	done         chan Void
	state        atomic.Bool
	stateCh      chan Void
	period       int         //多少秒更新一次redis
	offline      atomic.Bool //是否下线不参与leader选举
}

func NewLeaderRedis(name string, id int, client *RedisClient) *LeaderRedis {
	return &LeaderRedis{
		name:    name,
		id:      id,
		client:  client,
		done:    make(chan Void),
		stateCh: make(chan Void, 1),
		period:  5,
	}
}

func (l *LeaderRedis) SetPeriod(period int) {
	l.period = period
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

func (l *LeaderRedis) Online() {
	l.offline.Store(false)
}

func (l *LeaderRedis) Offline() {
	l.offline.Store(true)
}

func (l *LeaderRedis) start() {
	go l.goNotify()
	go l.goTick()
}

func (l *LeaderRedis) stop(wait *sync.WaitGroup) {
	close(l.done)
}

func (l *LeaderRedis) goNotify() {
	var lastState = false
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
			state := l.state.Load()
			if lastState == state {
				continue
			}
			l.handlersMu.Lock()
			for _, handler := range l.handlers {
				callHandler(handler, state)
			}
			l.handlersMu.Unlock()
			lastState = state
		case <-l.done:
			break
		}
	}
}

func (l *LeaderRedis) goTick() {
	ctx := context.Background()
	script := `
local key = KEYS[1]
local value = ARGV[1]
local ex = tonumber(ARGV[2])
local ret = redis.call("SET", key, value, "EX", ex, "NX")
if ret then
    return 1
else
    ret = redis.call("GET", key)
    if ret == value then
        return redis.call("EXPIRE", key, ex)
    else
        return 0
    end
end
`
	var scriptSha1 string
	if res := l.client.Raw().ScriptLoad(ctx, script); res.Err() != nil {
		OpsAlarm("LeaderRedis(%s)ScriptLoad失败-%s", l.name, res.Err())
		return
	} else {
		scriptSha1 = res.Val()
	}

	key := l.client.Prefix("tk-leader-" + l.name)
	token := fmt.Sprintf("%s-%d", Hostname, l.id)
	ttl := 3 * l.period
	call := func() {
		cmd := l.client.Raw().EvalSha(ctx, scriptSha1, []string{key}, token, ttl)
		if !RedisErrNilOrKeyNotExist(cmd.Err()) {
			OpsAlarm("LeaderRedis(%s)EvalSha错误=%s,sha1=%s", l.name, cmd.Err(), scriptSha1)
		} else if ret, err := cmd.Int(); err != nil {
			OpsAlarm("LeaderRedis(%s)EvalSha返回不是int=%s", l.name, cmd.Val())
		} else {
			l.state.Store(ret == 1)
			select {
			case l.stateCh <- VoidValue:
			default:
			}
		}
	}
	if !l.offline.Load() {
		call()
	}

	tick := time.NewTicker(time.Duration(l.period) * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if l.offline.Load() {
				//下线了，不参与leader选举
				l.state.Store(false)
				select {
				case l.stateCh <- VoidValue:
				default:
				}
			} else {
				call()
			}
		case <-l.done:
			break
		}
	}
}
