package thinkgo

import (
	"context"
	"github.com/go-redis/redis/v8"
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

// RedisIntCmd redis返回int的command
type RedisIntCmd struct {
	*redis.Cmd
	val int64
}

func (cmd *RedisIntCmd) Result() (int64, error) {
	return cmd.val, cmd.Err()
}

// SetExf 带过期时间设置key和value，但是如果key存在，则不会更新过期时间
func (r *RedisClient) SetExf(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.Cmd {
	return r.client.Eval(ctx, redisSetExf, []string{r.Prefix(key)}, value, int(expiration.Seconds()))
}

// IncrEx 带过时间incr某个key，首次设置值时才会设置过期时间
func (r *RedisClient) IncrEx(ctx context.Context, key string, expiration time.Duration) *RedisIntCmd {
	return r.IncrByEx(ctx, key, 1, expiration)
}

// IncrByEx 带过时间incr某个key，首次设置值时才会设置过期时间
func (r *RedisClient) IncrByEx(ctx context.Context, key string, val int64, expiration time.Duration) *RedisIntCmd {
	cmd := r.client.Eval(ctx, redisIncrByEx, []string{r.Prefix(key)}, val, int(expiration.Seconds()))
	var res int64 = 0
	if i, err := cmd.Result(); err == nil {
		res = i.(int64)
	}
	return &RedisIntCmd{Cmd: cmd, val: res}
}
