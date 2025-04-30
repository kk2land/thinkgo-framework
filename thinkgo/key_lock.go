package thinkgo

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

// 对某个key来上锁，KeyLockMem=基于内存map；KeyLockRedis=基于redis；

var keyLockErrPrefix = "keyLock失败"

type KeyLockHandler func() interface{}

func KeyLockErrLockedFail(err interface{}) bool {
	if apiError, ok := err.(*ApiError); ok && strings.HasPrefix(apiError.message, keyLockErrPrefix) {
		return true
	}
	return false
}

// KeyLockMem start
var keyLockMemMap = CMapNew[interface{}]()

type keyLockMemItem struct {
	ch      chan struct{}
	counter int32
}

func keyLockMemItemNew(k string) interface{} {
	ch := make(chan struct{}, 1)
	ch <- struct{}{}
	return &keyLockMemItem{ch: ch, counter: 1}
}

// KeyLockMem 在内存中对某个key进行上锁，上锁成功则调用函数f，上锁失败则panic，返回函数f的返回值
func KeyLockMem(key string, f KeyLockHandler) interface{} {
	return KeyLockMemTimeout(key, 10*time.Second, f)
}

// KeyLockMemTry 在内存中对某个key进行尝试上锁，返回函数f的返回值，locked=true则表上锁成功；false则表示上锁失败；
func KeyLockMemTry(key string, f KeyLockHandler) (obj interface{}, locked bool) {
	defer func() {
		if err := recover(); err != nil {
			if KeyLockErrLockedFail(recover()) {
				return
			}
			panic(err)
		}
	}()
	return KeyLockMemTimeout(key, 0, f), true
}

// KeyLockMemTimeout 在内存中对某个key进行上锁，上锁成功则调用函数f，失败则panic，带超时功能
func KeyLockMemTimeout(key string, timeout time.Duration, f KeyLockHandler) interface{} {
	var item *keyLockMemItem
	shard := keyLockMemMap.GetShard(key)
	shard.LoadOrCreateCb(key, keyLockMemItemNew, func(val interface{}, loaded bool) {
		item = val.(*keyLockMemItem)
		if loaded {
			atomic.AddInt32(&item.counter, 1)
		}
	})
	t := time.NewTimer(timeout)
	var locked = false
	defer func() {
		if atomic.AddInt32(&item.counter, -1) == 0 {
			shard.Lock()
			if atomic.LoadInt32(&item.counter) == 0 {
				delete(shard.Items(), key)
			} else if locked {
				item.ch <- struct{}{}
			}
			shard.Unlock()
		} else if locked {
			item.ch <- struct{}{}
		}
		t.Stop()
	}()

	select {
	case <-item.ch:
		locked = true
		ret := f()
		return ret
	case <-t.C:
		break
	}
	panic(NewApiError1f("请求失败", "%s,key=%s", keyLockErrPrefix, key))
}

// KeyLockRedis start
var keyLockRedisTokenPrefix = fmt.Sprintf("%s-%d-", Hostname, Pid)
var keyLockRedisTTL = 25 * time.Second
var keyLockRedisWait = 5 * time.Second
var keyLockRedisRelease = `
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
end
return 0
`

// KeyLockRedisDefault 获取基于默认redis对某个key进行上锁
func KeyLockRedisDefault() *KeyLockRedis {
	return &KeyLockRedis{
		Client: RedisDefaultOrPanic(),
	}
}

// KeyLockRedis 基于redis对某个key进行上锁
type KeyLockRedis struct {
	Client *RedisClient
}

// Lock 上锁成功调用函数f，不成功则panic
func (l *KeyLockRedis) Lock(key string, f KeyLockHandler) interface{} {
	return l.LockTimeout(key, 10*time.Second, f)
}

// LockTry 上锁成功调用函数f，不成功则locked=false
func (l *KeyLockRedis) LockTry(key string, f KeyLockHandler) (obj interface{}, locked bool) {
	defer func() {
		if err := recover(); err != nil {
			if KeyLockErrLockedFail(recover()) {
				return
			}
			panic(err)
		}
	}()
	return l.LockTimeout(key, 0, f), true
}

// LockTimeout 上锁成功调用函数f，不成功则panic，带超时功能
func (l *KeyLockRedis) LockTimeout(key string, timeout time.Duration, f KeyLockHandler) interface{} {
	token := fmt.Sprintf("%s%d", keyLockRedisTokenPrefix, time.Now().UnixNano()/1000000)
	key = "redislock|" + key
	ctx := context.Background()
	r := l.Client

	for {
		ok, err := r.SetNX(ctx, key, token, keyLockRedisTTL).Result()
		if err != nil {
			panic(err)
		}
		if ok {
			defer func() {
				err := r.Raw().Eval(ctx, keyLockRedisRelease, []string{r.Prefix(key)}, token).Err()
				if err != nil {
					panic(err)
				}
			}()
			return f()
		}
		if timeout <= 0 {
			break
		}
		time.Sleep(keyLockRedisWait)
		timeout -= keyLockRedisWait
	}
	p := r.Raw().Pipeline()
	res1 := p.TTL(ctx, key)
	res2 := p.Get(ctx, key)
	if _, err := p.Exec(context.Background()); err != nil {
		panic(err)
	} else {
		var outMsg string
		if ttl, err := res1.Result(); err == nil {
			outMsg = fmt.Sprintf("请求失败,请%d秒后重试", ttl.Seconds())
		} else {
			outMsg = "请求失败"
		}
		panic(NewApiError1f(outMsg, "%s,key=%s,left=%s,val=%s", keyLockErrPrefix, key, res1.Val(), res2.Val()))
	}
}

// KeyLockMySQL start
var keyLockMySQLPrefix = "tkgokl-" + AppName + "-"

// KeyLockMySQLDefault 获取基于默认数据库配置的
func KeyLockMySQLDefault() *KeyLockMySQL {
	return &KeyLockMySQL{
		Client: DBDefaultOrPanic(),
	}
}

// KeyLockMySQL 基于mysql的对某个key进行上锁
type KeyLockMySQL struct {
	Client *DBInstance
}

func (l *KeyLockMySQL) Lock(key string, f KeyLockHandler) interface{} {
	return l.LockTimeout(key, 10*time.Second, f)
}

func (l *KeyLockMySQL) LockTry(key string, f KeyLockHandler) (obj interface{}, locked bool) {
	defer func() {
		if err := recover(); err != nil {
			if KeyLockErrLockedFail(err) {
				return
			}
			panic(err)
		}
	}()
	return l.LockTimeout(key, 0, f), true
}

// LockTimeout 参数timeout会int(timeout.Seconds())，向下取整1秒
func (l *KeyLockMySQL) LockTimeout(key string, timeout time.Duration, f KeyLockHandler) interface{} {
	sess := l.Client.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		panic(err)
	}
	defer sess.Commit()
	key = keyLockMySQLPrefix + key
	res, err := sess.QueryString("select GET_LOCK(?, ?) as l", key, int(timeout.Seconds()))
	if err != nil {
		panic(err)
	}
	if res[0]["l"] == "1" {
		defer func() {
			if _, err = sess.Exec("select RELEASE_LOCK(?)", key); err != nil {
				panic(err)
			}
		}()
		return f()
	}
	panic(NewApiError1f("请求失败", "%s,key=%s", keyLockErrPrefix, key))
}
