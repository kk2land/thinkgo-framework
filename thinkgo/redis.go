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

func RedisKeyNotExist(err error) bool {
	return errors.Is(err, redis.Nil)
}

func RedisNoErrOrKeyNotExist(err error) bool {
	return err == nil || errors.Is(err, redis.Nil)
}

func RedisErrRetry(err error) bool {
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

func RedisDefault() (*RedisClient, error) {
	redisDefaultOnce.Do(func() {
		redisDefault, _ = Redis("default")
	})
	if redisDefault != nil {
		return redisDefault, nil
	}
	return nil, errors.New("redis[default] not exists")
}

func RedisDefaultOrPanic() *RedisClient {
	if r, err := RedisDefault(); err != nil {
		panic(err)
	} else {
		return r
	}
}

func Redis(name string) (*RedisClient, error) {
	obj, err := redisInstanceMap.LoadOrCreate(name)
	if err != nil {
		return nil, err
	}
	return obj.(*RedisClient), nil
}

func RedisOrPanic(name string) *RedisClient {
	if r, err := Redis(name); err != nil {
		panic(err)
	} else {
		return r
	}
}

func RedisCloseAll() {
	redisInstanceMap.Clear(func(name string, inst interface{}) {
		if db, ok := inst.(*RedisClient); ok {
			if err := db.close(); err != nil {
				Logger.Errorf("[RedisCloseAll]close redis[%s] fail - %s", db.name, err.Error())
			}
		}
	})
}
