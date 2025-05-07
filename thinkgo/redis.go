package thinkgo

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	"io"
	"strings"
	"sync"
)

var redisInstanceMap = NewInstanceMap(redisCreate)
var redisDefault *RedisClient
var redisDefaultOnce sync.Once

func initRedis() {
	redis.SetLogger(&redisLogger{FieldLogger: Logger})
}

// RedisErrorKeyNotExist 判断redis的error是否是key不存在
func RedisErrorKeyNotExist(err error) bool {
	return errors.Is(err, redis.Nil)
}

// RedisErrorNilOrKeyNotExist 判断redis的error是否是nil或者key不存在
func RedisErrorNilOrKeyNotExist(err error) bool {
	return err == nil || errors.Is(err, redis.Nil)
}

// RedisErrorRetry 判断redis的error是否是可以重试的
func RedisErrorRetry(err error) bool {
	switch {
	case err == io.EOF, errors.Is(err, io.ErrUnexpectedEOF):
		return true
	case err == nil, errors.Is(err, redis.Nil), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return false
	}
	if ErrIsTimeout(err) || ErrIsBrokenPipe(err) {
		return true
	}

	s := err.Error()
	if s == "ERR max number of clients reached" {
		return true
	}
	if strings.HasPrefix(s, "LOADING ") {
		return true
	}
	if strings.HasPrefix(s, "READONLY ") {
		return true
	}
	if strings.HasPrefix(s, "CLUSTERDOWN ") {
		return true
	}
	if strings.HasPrefix(s, "TRYAGAIN ") {
		return true
	}
	return false
}

func redisCreate(name string) (interface{}, error) {
	var config redisConfig
	var ok bool
	if config, ok = Config.Redis[name]; !ok {
		return nil, fmt.Errorf("config redis[%s] not exists", name)
	}
	var client *redis.Client
	//初始化redis
	opt := &redis.Options{
		Addr:         config.Addr,
		Password:     config.Password,
		DB:           config.DB,
		MinIdleConns: 0,
	}
	optConfig := config.getOptions()
	if optConfig.Timeout > 0 {
		opt.DialTimeout = optConfig.Timeout.ToDuration()
		opt.ReadTimeout = optConfig.Timeout.ToDuration()
	}
	if optConfig.PoolSize > 0 {
		opt.PoolSize = optConfig.PoolSize
	}
	if optConfig.MaxRetries != 0 {
		opt.MaxRetries = optConfig.MaxRetries
		if optConfig.MinRetryBackoff != 0 {
			opt.MinRetryBackoff = optConfig.MinRetryBackoff.ToDuration()
		}
		if optConfig.MaxRetryBackoff != 0 {
			opt.MaxRetryBackoff = optConfig.MaxRetryBackoff.ToDuration()
		}
	} else {
		opt.MaxRetries = 0
	}
	client = redis.NewClient(opt)
	//if err := client.Ping(context.Background()).Err(); err != nil {
	//	return nil, err
	//}
	db := newRedisClient(name, client, config.Prefix)
	return db, nil
}

// RedisDefault 获取默认的redis操作对象
func RedisDefault() (*RedisClient, error) {
	redisDefaultOnce.Do(func() {
		redisDefault, _ = Redis("default")
	})
	if redisDefault != nil {
		return redisDefault, nil
	}
	return nil, errors.New("redis[default] not exists")
}

// RedisDefaultOrPanic 获取默认的redis操作对象，否则panic
func RedisDefaultOrPanic() *RedisClient {
	if r, err := RedisDefault(); err != nil {
		panic(err)
	} else {
		return r
	}
}

// Redis 获取指定name的redis操作对象
func Redis(name string) (*RedisClient, error) {
	obj, err := redisInstanceMap.LoadOrCreate(name)
	if err != nil {
		return nil, err
	}
	return obj.(*RedisClient), nil
}

// RedisOrPanic 获取指定name的redis操作对象，否则panic
func RedisOrPanic(name string) *RedisClient {
	if r, err := Redis(name); err != nil {
		panic(err)
	} else {
		return r
	}
}

// RedisCloseAll 关闭全部redis操作对象
func RedisCloseAll() {
	redisInstanceMap.Clear(func(name string, inst interface{}) {
		if db, ok := inst.(*RedisClient); ok {
			if err := db.close(); err != nil {
				Logger.Errorf("[RedisCloseAll]close redis[%s] fail - %s", db.name, err.Error())
			}
		}
	})
}
