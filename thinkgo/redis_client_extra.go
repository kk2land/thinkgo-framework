package thinkgo

import (
	"context"
	"github.com/go-redis/redis/v8"
	"strings"
	"sync/atomic"
	"time"
)

var redisSetExf = `
local value = redis.call("ttl", KEYS[1])
local ex = tonumber(ARGV[2])
if value > 0 then
	ex = value
end
return redis.call("setex", KEYS[1], ex, ARGV[1])
`
var redisIncrByEx = `
local add_val = tonumber(ARGV[1])
local ret = redis.call("incrby", KEYS[1], add_val)
if ret == add_val then
    redis.call("expire", KEYS[1], ARGV[2])
end
return ret
`

type RedisIntCmd struct {
	*redis.Cmd
	val int64
}

type RedisEval struct {
	Script string
	sha    atomic.Value
	done   uint32
}

func (eval *RedisEval) Load(r *RedisClient) {
	if !atomic.CompareAndSwapUint32(&eval.done, 0, 1) {
		return
	}
	go func() {
		str, err := r.Raw().ScriptLoad(context.Background(), eval.Script).Result()
		if err == nil {
			eval.sha.Store(str)
		}
		atomic.StoreUint32(&eval.done, 0)
	}()
}

func (cmd *RedisIntCmd) Result() (int64, error) {
	return cmd.val, cmd.Err()
}

func (r *RedisClient) SetExf(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.Cmd {
	return r.client.Eval(ctx, redisSetExf, []string{r.Prefix(key)}, value, int(expiration.Seconds()))
}

func (r *RedisClient) IncrEx(ctx context.Context, key string, expiration time.Duration) *RedisIntCmd {
	return r.IncrByEx(ctx, key, 1, expiration)
}

func (r *RedisClient) IncrByEx(ctx context.Context, key string, val int64, expiration time.Duration) *RedisIntCmd {
	cmd := r.client.Eval(ctx, redisIncrByEx, []string{r.Prefix(key)}, val, int(expiration.Seconds()))
	var res int64 = 0
	if i, err := cmd.Result(); err == nil {
		res = i.(int64)
	}
	return &RedisIntCmd{Cmd: cmd, val: res}
}

func (r *RedisClient) Eval1(ctx context.Context, eval *RedisEval, keys []string, args ...interface{}) *redis.Cmd {
	if sha := eval.sha.Load(); sha != nil {
		cmd := r.Raw().EvalSha(ctx, sha.(string), keys, args...)
		if err := cmd.Err(); err == nil {
			return cmd
		} else if !strings.HasPrefix(err.Error(), "NOSCRIPT No matching script") {
			return cmd
		}
	}
	eval.Load(r)
	return r.Raw().Eval(ctx, eval.Script, keys, args...)
}
