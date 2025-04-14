package thinkgo

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"reflect"
	"time"
)

type redisCmdable interface {
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Unlink(ctx context.Context, keys ...string) *redis.IntCmd

	Exists(ctx context.Context, keys ...string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	ExpireAt(ctx context.Context, key string, tm time.Time) *redis.BoolCmd

	Persist(ctx context.Context, key string) *redis.BoolCmd
	PExpire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	PExpireAt(ctx context.Context, key string, tm time.Time) *redis.BoolCmd
	PTTL(ctx context.Context, key string) *redis.DurationCmd

	Rename(ctx context.Context, key, newkey string) *redis.StatusCmd
	RenameNX(ctx context.Context, key, newkey string) *redis.BoolCmd

	Sort(ctx context.Context, key string, sort *redis.Sort) *redis.StringSliceCmd
	SortStore(ctx context.Context, key, store string, sort *redis.Sort) *redis.IntCmd
	SortInterfaces(ctx context.Context, key string, sort *redis.Sort) *redis.SliceCmd
	Touch(ctx context.Context, keys ...string) *redis.IntCmd
	TTL(ctx context.Context, key string) *redis.DurationCmd
	Type(ctx context.Context, key string) *redis.StatusCmd
	Append(ctx context.Context, key, value string) *redis.IntCmd
	Decr(ctx context.Context, key string) *redis.IntCmd
	DecrBy(ctx context.Context, key string, decrement int64) *redis.IntCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	GetRange(ctx context.Context, key string, start, end int64) *redis.StringCmd
	GetSet(ctx context.Context, key string, value interface{}) *redis.StringCmd
	Incr(ctx context.Context, key string) *redis.IntCmd
	IncrBy(ctx context.Context, key string, value int64) *redis.IntCmd
	IncrByFloat(ctx context.Context, key string, value float64) *redis.FloatCmd
	MGet(ctx context.Context, keys ...string) *redis.SliceCmd
	MSet(ctx context.Context, values ...interface{}) *redis.StatusCmd
	MSetNX(ctx context.Context, values ...interface{}) *redis.BoolCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	SetArgs(ctx context.Context, key string, value interface{}, a redis.SetArgs) *redis.StatusCmd
	SetEX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
	SetXX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
	SetRange(ctx context.Context, key string, offset int64, value string) *redis.IntCmd
	StrLen(ctx context.Context, key string) *redis.IntCmd

	GetBit(ctx context.Context, key string, offset int64) *redis.IntCmd
	SetBit(ctx context.Context, key string, offset int64, value int) *redis.IntCmd
	BitCount(ctx context.Context, key string, bitCount *redis.BitCount) *redis.IntCmd
	BitOpAnd(ctx context.Context, destKey string, keys ...string) *redis.IntCmd
	BitOpOr(ctx context.Context, destKey string, keys ...string) *redis.IntCmd
	BitOpXor(ctx context.Context, destKey string, keys ...string) *redis.IntCmd
	BitOpNot(ctx context.Context, destKey string, key string) *redis.IntCmd
	BitPos(ctx context.Context, key string, bit int64, pos ...int64) *redis.IntCmd
	BitField(ctx context.Context, key string, args ...interface{}) *redis.IntSliceCmd

	HDel(ctx context.Context, key string, fields ...string) *redis.IntCmd
	HExists(ctx context.Context, key, field string) *redis.BoolCmd
	HGet(ctx context.Context, key, field string) *redis.StringCmd
	HGetAll(ctx context.Context, key string) *redis.StringStringMapCmd
	HIncrBy(ctx context.Context, key, field string, incr int64) *redis.IntCmd
	HIncrByFloat(ctx context.Context, key, field string, incr float64) *redis.FloatCmd
	HKeys(ctx context.Context, key string) *redis.StringSliceCmd
	HLen(ctx context.Context, key string) *redis.IntCmd
	HMGet(ctx context.Context, key string, fields ...string) *redis.SliceCmd
	HSet(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	HMSet(ctx context.Context, key string, values ...interface{}) *redis.BoolCmd
	HSetNX(ctx context.Context, key, field string, value interface{}) *redis.BoolCmd
	HVals(ctx context.Context, key string) *redis.StringSliceCmd

	BLPop(ctx context.Context, timeout time.Duration, keys ...string) *redis.StringSliceCmd
	BRPop(ctx context.Context, timeout time.Duration, keys ...string) *redis.StringSliceCmd
	BRPopLPush(ctx context.Context, source, destination string, timeout time.Duration) *redis.StringCmd
	LIndex(ctx context.Context, key string, index int64) *redis.StringCmd
	LInsert(ctx context.Context, key, op string, pivot, value interface{}) *redis.IntCmd
	LInsertBefore(ctx context.Context, key string, pivot, value interface{}) *redis.IntCmd
	LInsertAfter(ctx context.Context, key string, pivot, value interface{}) *redis.IntCmd
	LLen(ctx context.Context, key string) *redis.IntCmd
	LPop(ctx context.Context, key string) *redis.StringCmd
	LPos(ctx context.Context, key string, value string, args redis.LPosArgs) *redis.IntCmd
	LPosCount(ctx context.Context, key string, value string, count int64, args redis.LPosArgs) *redis.IntSliceCmd
	LPush(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	LPushX(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	LRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd
	LRem(ctx context.Context, key string, count int64, value interface{}) *redis.IntCmd
	LSet(ctx context.Context, key string, index int64, value interface{}) *redis.StatusCmd
	LTrim(ctx context.Context, key string, start, stop int64) *redis.StatusCmd
	RPop(ctx context.Context, key string) *redis.StringCmd
	RPopLPush(ctx context.Context, source, destination string) *redis.StringCmd
	RPush(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	RPushX(ctx context.Context, key string, values ...interface{}) *redis.IntCmd

	SAdd(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	SCard(ctx context.Context, key string) *redis.IntCmd
	SDiff(ctx context.Context, keys ...string) *redis.StringSliceCmd
	SDiffStore(ctx context.Context, destination string, keys ...string) *redis.IntCmd
	SInter(ctx context.Context, keys ...string) *redis.StringSliceCmd
	SInterStore(ctx context.Context, destination string, keys ...string) *redis.IntCmd
	SIsMember(ctx context.Context, key string, member interface{}) *redis.BoolCmd
	SMembers(ctx context.Context, key string) *redis.StringSliceCmd
	SMembersMap(ctx context.Context, key string) *redis.StringStructMapCmd
	SMove(ctx context.Context, source, destination string, member interface{}) *redis.BoolCmd
	SPop(ctx context.Context, key string) *redis.StringCmd
	SPopN(ctx context.Context, key string, count int64) *redis.StringSliceCmd
	SRandMember(ctx context.Context, key string) *redis.StringCmd
	SRandMemberN(ctx context.Context, key string, count int64) *redis.StringSliceCmd
	SRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	SUnion(ctx context.Context, keys ...string) *redis.StringSliceCmd
	SUnionStore(ctx context.Context, destination string, keys ...string) *redis.IntCmd

	BZPopMax(ctx context.Context, timeout time.Duration, keys ...string) *redis.ZWithKeyCmd
	BZPopMin(ctx context.Context, timeout time.Duration, keys ...string) *redis.ZWithKeyCmd
	ZAdd(ctx context.Context, key string, members ...*redis.Z) *redis.IntCmd
	ZAddNX(ctx context.Context, key string, members ...*redis.Z) *redis.IntCmd
	ZAddXX(ctx context.Context, key string, members ...*redis.Z) *redis.IntCmd
	ZAddCh(ctx context.Context, key string, members ...*redis.Z) *redis.IntCmd
	ZAddNXCh(ctx context.Context, key string, members ...*redis.Z) *redis.IntCmd
	ZAddXXCh(ctx context.Context, key string, members ...*redis.Z) *redis.IntCmd
	ZIncr(ctx context.Context, key string, member *redis.Z) *redis.FloatCmd
	ZIncrNX(ctx context.Context, key string, member *redis.Z) *redis.FloatCmd
	ZIncrXX(ctx context.Context, key string, member *redis.Z) *redis.FloatCmd
	ZCard(ctx context.Context, key string) *redis.IntCmd
	ZCount(ctx context.Context, key, min, max string) *redis.IntCmd
	ZLexCount(ctx context.Context, key, min, max string) *redis.IntCmd
	ZIncrBy(ctx context.Context, key string, increment float64, member string) *redis.FloatCmd
	ZInterStore(ctx context.Context, destination string, store *redis.ZStore) *redis.IntCmd
	ZPopMax(ctx context.Context, key string, count ...int64) *redis.ZSliceCmd
	ZPopMin(ctx context.Context, key string, count ...int64) *redis.ZSliceCmd
	ZRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd
	ZRangeWithScores(ctx context.Context, key string, start, stop int64) *redis.ZSliceCmd
	ZRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.StringSliceCmd
	ZRangeByLex(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.StringSliceCmd
	ZRangeByScoreWithScores(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.ZSliceCmd
	ZRank(ctx context.Context, key, member string) *redis.IntCmd
	ZRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	ZRemRangeByRank(ctx context.Context, key string, start, stop int64) *redis.IntCmd
	ZRemRangeByScore(ctx context.Context, key, min, max string) *redis.IntCmd
	ZRemRangeByLex(ctx context.Context, key, min, max string) *redis.IntCmd
	ZRevRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd
	ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) *redis.ZSliceCmd
	ZRevRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.StringSliceCmd
	ZRevRangeByLex(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.StringSliceCmd
	ZRevRangeByScoreWithScores(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.ZSliceCmd
	ZRevRank(ctx context.Context, key, member string) *redis.IntCmd
	ZScore(ctx context.Context, key, member string) *redis.FloatCmd
	ZUnionStore(ctx context.Context, dest string, store *redis.ZStore) *redis.IntCmd

	PFAdd(ctx context.Context, key string, els ...interface{}) *redis.IntCmd
	PFCount(ctx context.Context, keys ...string) *redis.IntCmd
	PFMerge(ctx context.Context, dest string, keys ...string) *redis.StatusCmd
}

type redisPrefixCmdable struct {
	cmdable redis.Cmdable
	prefix  string
}

func (r *redisPrefixCmdable) k(key string) string {
	return r.prefix + key
}

func (r *redisPrefixCmdable) ks(keys []string) []string {
	for i := 0; i < len(keys); i++ {
		keys[i] = r.k(keys[i])
	}
	return keys
}

func (r *redisPrefixCmdable) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	return r.cmdable.Del(ctx, r.ks(keys)...)
}

func (r *redisPrefixCmdable) Unlink(ctx context.Context, keys ...string) *redis.IntCmd {
	return r.cmdable.Unlink(ctx, r.ks(keys)...)
}

func (r *redisPrefixCmdable) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	return r.cmdable.Exists(ctx, r.ks(keys)...)
}

func (r *redisPrefixCmdable) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	return r.cmdable.Expire(ctx, r.k(key), expiration)
}

func (r *redisPrefixCmdable) ExpireAt(ctx context.Context, key string, tm time.Time) *redis.BoolCmd {
	return r.cmdable.ExpireAt(ctx, r.k(key), tm)
}

func (r *redisPrefixCmdable) Persist(ctx context.Context, key string) *redis.BoolCmd {
	return r.cmdable.Persist(ctx, r.k(key))
}

func (r *redisPrefixCmdable) PExpire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	return r.cmdable.PExpire(ctx, r.k(key), expiration)
}

func (r *redisPrefixCmdable) PExpireAt(ctx context.Context, key string, tm time.Time) *redis.BoolCmd {
	return r.cmdable.PExpireAt(ctx, r.k(key), tm)
}

func (r *redisPrefixCmdable) PTTL(ctx context.Context, key string) *redis.DurationCmd {
	return r.cmdable.PTTL(ctx, r.k(key))
}

func (r *redisPrefixCmdable) Rename(ctx context.Context, key, newkey string) *redis.StatusCmd {
	return r.cmdable.Rename(ctx, r.k(key), r.k(newkey))
}

func (r *redisPrefixCmdable) RenameNX(ctx context.Context, key, newkey string) *redis.BoolCmd {
	return r.cmdable.RenameNX(ctx, r.k(key), r.k(newkey))
}

func (r *redisPrefixCmdable) Sort(ctx context.Context, key string, sort *redis.Sort) *redis.StringSliceCmd {
	return r.cmdable.Sort(ctx, r.k(key), sort)
}

func (r *redisPrefixCmdable) SortStore(ctx context.Context, key, store string, sort *redis.Sort) *redis.IntCmd {
	return r.cmdable.SortStore(ctx, r.k(key), store, sort)
}

func (r *redisPrefixCmdable) SortInterfaces(ctx context.Context, key string, sort *redis.Sort) *redis.SliceCmd {
	return r.cmdable.SortInterfaces(ctx, r.k(key), sort)
}

func (r *redisPrefixCmdable) Touch(ctx context.Context, keys ...string) *redis.IntCmd {
	return r.cmdable.Touch(ctx, r.ks(keys)...)
}

func (r *redisPrefixCmdable) TTL(ctx context.Context, key string) *redis.DurationCmd {
	return r.cmdable.TTL(ctx, r.k(key))
}

func (r *redisPrefixCmdable) Type(ctx context.Context, key string) *redis.StatusCmd {
	return r.cmdable.Type(ctx, r.k(key))
}

func (r *redisPrefixCmdable) Append(ctx context.Context, key, value string) *redis.IntCmd {
	return r.cmdable.Append(ctx, r.k(key), value)
}

func (r *redisPrefixCmdable) Decr(ctx context.Context, key string) *redis.IntCmd {
	return r.cmdable.Decr(ctx, r.k(key))
}

func (r *redisPrefixCmdable) DecrBy(ctx context.Context, key string, decrement int64) *redis.IntCmd {
	return r.cmdable.DecrBy(ctx, r.k(key), decrement)
}

func (r *redisPrefixCmdable) Get(ctx context.Context, key string) *redis.StringCmd {
	return r.cmdable.Get(ctx, r.k(key))
}

func (r *redisPrefixCmdable) GetRange(ctx context.Context, key string, start, end int64) *redis.StringCmd {
	return r.cmdable.GetRange(ctx, r.k(key), start, end)
}

func (r *redisPrefixCmdable) GetSet(ctx context.Context, key string, value interface{}) *redis.StringCmd {
	return r.cmdable.GetSet(ctx, r.k(key), value)
}

func (r *redisPrefixCmdable) Incr(ctx context.Context, key string) *redis.IntCmd {
	return r.cmdable.Incr(ctx, r.k(key))
}

func (r *redisPrefixCmdable) IncrBy(ctx context.Context, key string, value int64) *redis.IntCmd {
	return r.cmdable.IncrBy(ctx, r.k(key), value)
}

func (r *redisPrefixCmdable) IncrByFloat(ctx context.Context, key string, value float64) *redis.FloatCmd {
	return r.cmdable.IncrByFloat(ctx, r.k(key), value)
}

func (r *redisPrefixCmdable) MGet(ctx context.Context, keys ...string) *redis.SliceCmd {
	return r.cmdable.MGet(ctx, r.ks(keys)...)
}

func (r *redisPrefixCmdable) MSet(ctx context.Context, values ...interface{}) *redis.StatusCmd {
	if len(values) == 1 {
		if m, ok := values[0].(map[string]interface{}); !ok {
			cmd := redis.NewStatusCmd(ctx)
			cmd.SetErr(fmt.Errorf("mset参数错误-params[0](%s)不是map[string]interface{}", reflect.TypeOf(values[0]).String()))
			return cmd
		} else {
			vals := make([]interface{}, 2*len(m))
			i := 0
			for k, v := range m {
				vals[i] = r.k(k)
				i++
				vals[i] = v
				i++
			}
			return r.cmdable.MSet(ctx, vals...)
		}
	} else {
		if len(values)%2 != 0 {
			cmd := redis.NewStatusCmd(ctx)
			cmd.SetErr(fmt.Errorf("mset参数错误-values个数不是偶数(%d)", len(values)))
			return cmd
		}
		for i := 0; i < len(values); i += 2 {
			if s, ok := values[i].(string); !ok {
				cmd := redis.NewStatusCmd(ctx)
				cmd.SetErr(fmt.Errorf("mset参数错误-params[%d]不是string(%s)", i, reflect.TypeOf(values[i]).String()))
				return cmd
			} else {
				values[i] = r.k(s)
			}
		}
		return r.cmdable.MSet(ctx, values...)
	}
}

func (r *redisPrefixCmdable) MSetNX(ctx context.Context, values ...interface{}) *redis.BoolCmd {
	if len(values) == 1 {
		if m, ok := values[0].(map[string]interface{}); !ok {
			cmd := redis.NewBoolCmd(ctx)
			cmd.SetErr(fmt.Errorf("mset参数错误-params[0](%s)不是map[string]interface{}", reflect.TypeOf(values[0]).String()))
			return cmd
		} else {
			vals := make([]interface{}, 2*len(m))
			i := 0
			for k, v := range m {
				vals[i] = r.k(k)
				i++
				vals[i] = v
				i++
			}
			return r.cmdable.MSetNX(ctx, vals...)
		}
	} else {
		if len(values)%2 != 0 {
			cmd := redis.NewBoolCmd(ctx)
			cmd.SetErr(fmt.Errorf("mset参数错误-values个数不是偶数(%d)", len(values)))
			return cmd
		}
		for i := 0; i < len(values); i += 2 {
			if s, ok := values[i].(string); !ok {
				cmd := redis.NewBoolCmd(ctx)
				cmd.SetErr(fmt.Errorf("mset参数错误-params[%d]不是string(%s)", i, reflect.TypeOf(values[i]).String()))
				return cmd
			} else {
				values[i] = r.k(s)
			}
		}
		return r.cmdable.MSetNX(ctx, values...)
	}
}

func (r *redisPrefixCmdable) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	return r.cmdable.Set(ctx, r.k(key), value, expiration)
}

func (r *redisPrefixCmdable) SetArgs(ctx context.Context, key string, value interface{}, a redis.SetArgs) *redis.StatusCmd {
	return r.cmdable.SetArgs(ctx, r.k(key), value, a)
}

func (r *redisPrefixCmdable) SetEX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	return r.cmdable.SetEX(ctx, r.k(key), value, expiration)
}

func (r *redisPrefixCmdable) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd {
	return r.cmdable.SetNX(ctx, r.k(key), value, expiration)
}

func (r *redisPrefixCmdable) SetXX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd {
	return r.cmdable.SetXX(ctx, r.k(key), value, expiration)
}

func (r *redisPrefixCmdable) SetRange(ctx context.Context, key string, offset int64, value string) *redis.IntCmd {
	return r.cmdable.SetRange(ctx, r.k(key), offset, value)
}

func (r *redisPrefixCmdable) StrLen(ctx context.Context, key string) *redis.IntCmd {
	return r.cmdable.StrLen(ctx, r.k(key))
}

func (r *redisPrefixCmdable) GetBit(ctx context.Context, key string, offset int64) *redis.IntCmd {
	return r.cmdable.GetBit(ctx, r.k(key), offset)
}

func (r *redisPrefixCmdable) SetBit(ctx context.Context, key string, offset int64, value int) *redis.IntCmd {
	return r.cmdable.SetBit(ctx, r.k(key), offset, value)
}

func (r *redisPrefixCmdable) BitCount(ctx context.Context, key string, bitCount *redis.BitCount) *redis.IntCmd {
	return r.cmdable.BitCount(ctx, r.k(key), bitCount)
}

func (r *redisPrefixCmdable) BitOpAnd(ctx context.Context, destKey string, keys ...string) *redis.IntCmd {
	return r.cmdable.BitOpAnd(ctx, r.k(destKey), r.ks(keys)...)
}

func (r *redisPrefixCmdable) BitOpOr(ctx context.Context, destKey string, keys ...string) *redis.IntCmd {
	return r.cmdable.BitOpOr(ctx, r.k(destKey), r.ks(keys)...)
}

func (r *redisPrefixCmdable) BitOpXor(ctx context.Context, destKey string, keys ...string) *redis.IntCmd {
	return r.cmdable.BitOpXor(ctx, r.k(destKey), r.ks(keys)...)
}

func (r *redisPrefixCmdable) BitOpNot(ctx context.Context, destKey string, key string) *redis.IntCmd {
	return r.cmdable.BitOpNot(ctx, r.k(destKey), r.k(key))
}

func (r *redisPrefixCmdable) BitPos(ctx context.Context, key string, bit int64, pos ...int64) *redis.IntCmd {
	return r.cmdable.BitPos(ctx, r.k(key), bit, pos...)
}

func (r *redisPrefixCmdable) BitField(ctx context.Context, key string, args ...interface{}) *redis.IntSliceCmd {
	return r.cmdable.BitField(ctx, r.k(key), args...)
}

func (r *redisPrefixCmdable) HDel(ctx context.Context, key string, fields ...string) *redis.IntCmd {
	return r.cmdable.HDel(ctx, r.k(key), fields...)
}

func (r *redisPrefixCmdable) HExists(ctx context.Context, key, field string) *redis.BoolCmd {
	return r.cmdable.HExists(ctx, r.k(key), field)
}

func (r *redisPrefixCmdable) HGet(ctx context.Context, key, field string) *redis.StringCmd {
	return r.cmdable.HGet(ctx, r.k(key), field)
}

func (r *redisPrefixCmdable) HGetAll(ctx context.Context, key string) *redis.StringStringMapCmd {
	return r.cmdable.HGetAll(ctx, r.k(key))
}

func (r *redisPrefixCmdable) HIncrBy(ctx context.Context, key, field string, incr int64) *redis.IntCmd {
	return r.cmdable.HIncrBy(ctx, r.k(key), field, incr)
}

func (r *redisPrefixCmdable) HIncrByFloat(ctx context.Context, key, field string, incr float64) *redis.FloatCmd {
	return r.cmdable.HIncrByFloat(ctx, r.k(key), field, incr)
}

func (r *redisPrefixCmdable) HKeys(ctx context.Context, key string) *redis.StringSliceCmd {
	return r.cmdable.HKeys(ctx, r.k(key))
}

func (r *redisPrefixCmdable) HLen(ctx context.Context, key string) *redis.IntCmd {
	return r.cmdable.HLen(ctx, r.k(key))
}

func (r *redisPrefixCmdable) HMGet(ctx context.Context, key string, fields ...string) *redis.SliceCmd {
	return r.cmdable.HMGet(ctx, r.k(key), fields...)
}

func (r *redisPrefixCmdable) HSet(ctx context.Context, key string, values ...interface{}) *redis.IntCmd {
	return r.cmdable.HSet(ctx, r.k(key), values...)
}

func (r *redisPrefixCmdable) HMSet(ctx context.Context, key string, values ...interface{}) *redis.BoolCmd {
	return r.cmdable.HMSet(ctx, r.k(key), values...)
}

func (r *redisPrefixCmdable) HSetNX(ctx context.Context, key, field string, value interface{}) *redis.BoolCmd {
	return r.cmdable.HSetNX(ctx, r.k(key), field, value)
}

func (r *redisPrefixCmdable) HVals(ctx context.Context, key string) *redis.StringSliceCmd {
	return r.cmdable.HVals(ctx, r.k(key))
}

func (r *redisPrefixCmdable) BLPop(ctx context.Context, timeout time.Duration, keys ...string) *redis.StringSliceCmd {
	return r.cmdable.BLPop(ctx, timeout, r.ks(keys)...)
}

func (r *redisPrefixCmdable) BRPop(ctx context.Context, timeout time.Duration, keys ...string) *redis.StringSliceCmd {
	return r.cmdable.BRPop(ctx, timeout, r.ks(keys)...)
}

func (r *redisPrefixCmdable) BRPopLPush(ctx context.Context, source, destination string, timeout time.Duration) *redis.StringCmd {
	return r.cmdable.BRPopLPush(ctx, r.k(source), r.k(destination), timeout)
}

func (r *redisPrefixCmdable) LIndex(ctx context.Context, key string, index int64) *redis.StringCmd {
	return r.cmdable.LIndex(ctx, r.k(key), index)
}

func (r *redisPrefixCmdable) LInsert(ctx context.Context, key, op string, pivot, value interface{}) *redis.IntCmd {
	return r.cmdable.LInsert(ctx, r.k(key), op, pivot, value)
}

func (r *redisPrefixCmdable) LInsertBefore(ctx context.Context, key string, pivot, value interface{}) *redis.IntCmd {
	return r.cmdable.LInsertBefore(ctx, r.k(key), pivot, value)
}

func (r *redisPrefixCmdable) LInsertAfter(ctx context.Context, key string, pivot, value interface{}) *redis.IntCmd {
	return r.cmdable.LInsertAfter(ctx, r.k(key), pivot, value)
}

func (r *redisPrefixCmdable) LLen(ctx context.Context, key string) *redis.IntCmd {
	return r.cmdable.LLen(ctx, r.k(key))
}

func (r *redisPrefixCmdable) LPop(ctx context.Context, key string) *redis.StringCmd {
	return r.cmdable.LPop(ctx, r.k(key))
}

func (r *redisPrefixCmdable) LPos(ctx context.Context, key string, value string, args redis.LPosArgs) *redis.IntCmd {
	return r.cmdable.LPos(ctx, r.k(key), value, args)
}

func (r *redisPrefixCmdable) LPosCount(ctx context.Context, key string, value string, count int64, args redis.LPosArgs) *redis.IntSliceCmd {
	return r.cmdable.LPosCount(ctx, r.k(key), value, count, args)
}

func (r *redisPrefixCmdable) LPush(ctx context.Context, key string, values ...interface{}) *redis.IntCmd {
	return r.cmdable.LPush(ctx, r.k(key), values...)
}

func (r *redisPrefixCmdable) LPushX(ctx context.Context, key string, values ...interface{}) *redis.IntCmd {
	return r.cmdable.LPushX(ctx, r.k(key), values...)
}

func (r *redisPrefixCmdable) LRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd {
	return r.cmdable.LRange(ctx, r.k(key), start, stop)
}

func (r *redisPrefixCmdable) LRem(ctx context.Context, key string, count int64, value interface{}) *redis.IntCmd {
	return r.cmdable.LRem(ctx, r.k(key), count, value)
}

func (r *redisPrefixCmdable) LSet(ctx context.Context, key string, index int64, value interface{}) *redis.StatusCmd {
	return r.cmdable.LSet(ctx, r.k(key), index, value)
}

func (r *redisPrefixCmdable) LTrim(ctx context.Context, key string, start, stop int64) *redis.StatusCmd {
	return r.cmdable.LTrim(ctx, r.k(key), start, stop)
}

func (r *redisPrefixCmdable) RPop(ctx context.Context, key string) *redis.StringCmd {
	return r.cmdable.RPop(ctx, r.k(key))
}

func (r *redisPrefixCmdable) RPopLPush(ctx context.Context, source, destination string) *redis.StringCmd {
	return r.cmdable.RPopLPush(ctx, r.k(source), r.k(destination))
}

func (r *redisPrefixCmdable) RPush(ctx context.Context, key string, values ...interface{}) *redis.IntCmd {
	return r.cmdable.RPush(ctx, r.k(key), values...)
}

func (r *redisPrefixCmdable) RPushX(ctx context.Context, key string, values ...interface{}) *redis.IntCmd {
	return r.cmdable.RPushX(ctx, r.k(key), values...)
}

func (r *redisPrefixCmdable) SAdd(ctx context.Context, key string, members ...interface{}) *redis.IntCmd {
	return r.cmdable.SAdd(ctx, r.k(key), members...)
}

func (r *redisPrefixCmdable) SCard(ctx context.Context, key string) *redis.IntCmd {
	return r.cmdable.SCard(ctx, r.k(key))
}

func (r *redisPrefixCmdable) SDiff(ctx context.Context, keys ...string) *redis.StringSliceCmd {
	return r.cmdable.SDiff(ctx, r.ks(keys)...)
}

func (r *redisPrefixCmdable) SDiffStore(ctx context.Context, destination string, keys ...string) *redis.IntCmd {
	return r.cmdable.SDiffStore(ctx, r.k(destination), r.ks(keys)...)
}

func (r *redisPrefixCmdable) SInter(ctx context.Context, keys ...string) *redis.StringSliceCmd {
	return r.cmdable.SInter(ctx, r.ks(keys)...)
}

func (r *redisPrefixCmdable) SInterStore(ctx context.Context, destination string, keys ...string) *redis.IntCmd {
	return r.cmdable.SInterStore(ctx, r.k(destination), r.ks(keys)...)
}

func (r *redisPrefixCmdable) SIsMember(ctx context.Context, key string, member interface{}) *redis.BoolCmd {
	return r.cmdable.SIsMember(ctx, r.k(key), member)
}

func (r *redisPrefixCmdable) SMembers(ctx context.Context, key string) *redis.StringSliceCmd {
	return r.cmdable.SMembers(ctx, r.k(key))
}

func (r *redisPrefixCmdable) SMembersMap(ctx context.Context, key string) *redis.StringStructMapCmd {
	return r.cmdable.SMembersMap(ctx, r.k(key))
}

func (r *redisPrefixCmdable) SMove(ctx context.Context, source, destination string, member interface{}) *redis.BoolCmd {
	return r.cmdable.SMove(ctx, r.k(source), r.k(destination), member)
}

func (r *redisPrefixCmdable) SPop(ctx context.Context, key string) *redis.StringCmd {
	return r.cmdable.SPop(ctx, r.k(key))
}

func (r *redisPrefixCmdable) SPopN(ctx context.Context, key string, count int64) *redis.StringSliceCmd {
	return r.cmdable.SPopN(ctx, r.k(key), count)
}

func (r *redisPrefixCmdable) SRandMember(ctx context.Context, key string) *redis.StringCmd {
	return r.cmdable.SRandMember(ctx, r.k(key))
}

func (r *redisPrefixCmdable) SRandMemberN(ctx context.Context, key string, count int64) *redis.StringSliceCmd {
	return r.cmdable.SRandMemberN(ctx, r.k(key), count)
}

func (r *redisPrefixCmdable) SRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd {
	return r.cmdable.SRem(ctx, r.k(key), members...)
}

func (r *redisPrefixCmdable) SUnion(ctx context.Context, keys ...string) *redis.StringSliceCmd {
	return r.cmdable.SUnion(ctx, r.ks(keys)...)
}

func (r *redisPrefixCmdable) SUnionStore(ctx context.Context, destination string, keys ...string) *redis.IntCmd {
	return r.cmdable.SUnionStore(ctx, r.k(destination), r.ks(keys)...)
}

func (r *redisPrefixCmdable) BZPopMax(ctx context.Context, timeout time.Duration, keys ...string) *redis.ZWithKeyCmd {
	return r.cmdable.BZPopMax(ctx, timeout, r.ks(keys)...)
}

func (r *redisPrefixCmdable) BZPopMin(ctx context.Context, timeout time.Duration, keys ...string) *redis.ZWithKeyCmd {
	return r.cmdable.BZPopMin(ctx, timeout, r.ks(keys)...)
}

func (r *redisPrefixCmdable) ZAdd(ctx context.Context, key string, members ...*redis.Z) *redis.IntCmd {
	return r.cmdable.ZAdd(ctx, r.k(key), members...)
}

func (r *redisPrefixCmdable) ZAddNX(ctx context.Context, key string, members ...*redis.Z) *redis.IntCmd {
	return r.cmdable.ZAddNX(ctx, r.k(key), members...)
}

func (r *redisPrefixCmdable) ZAddXX(ctx context.Context, key string, members ...*redis.Z) *redis.IntCmd {
	return r.cmdable.ZAddXX(ctx, r.k(key), members...)
}

func (r *redisPrefixCmdable) ZAddCh(ctx context.Context, key string, members ...*redis.Z) *redis.IntCmd {
	return r.cmdable.ZAddCh(ctx, r.k(key), members...)
}

func (r *redisPrefixCmdable) ZAddNXCh(ctx context.Context, key string, members ...*redis.Z) *redis.IntCmd {
	return r.cmdable.ZAddNXCh(ctx, r.k(key), members...)
}

func (r *redisPrefixCmdable) ZAddXXCh(ctx context.Context, key string, members ...*redis.Z) *redis.IntCmd {
	return r.cmdable.ZAddXXCh(ctx, r.k(key), members...)
}

func (r *redisPrefixCmdable) ZIncr(ctx context.Context, key string, member *redis.Z) *redis.FloatCmd {
	return r.cmdable.ZIncr(ctx, r.k(key), member)
}

func (r *redisPrefixCmdable) ZIncrNX(ctx context.Context, key string, member *redis.Z) *redis.FloatCmd {
	return r.cmdable.ZIncrNX(ctx, r.k(key), member)
}

func (r *redisPrefixCmdable) ZIncrXX(ctx context.Context, key string, member *redis.Z) *redis.FloatCmd {
	return r.cmdable.ZIncrXX(ctx, r.k(key), member)
}

func (r *redisPrefixCmdable) ZCard(ctx context.Context, key string) *redis.IntCmd {
	return r.cmdable.ZCard(ctx, r.k(key))
}

func (r *redisPrefixCmdable) ZCount(ctx context.Context, key, min, max string) *redis.IntCmd {
	return r.cmdable.ZCount(ctx, r.k(key), min, max)
}

func (r *redisPrefixCmdable) ZLexCount(ctx context.Context, key, min, max string) *redis.IntCmd {
	return r.cmdable.ZLexCount(ctx, r.k(key), min, max)
}

func (r *redisPrefixCmdable) ZIncrBy(ctx context.Context, key string, increment float64, member string) *redis.FloatCmd {
	return r.cmdable.ZIncrBy(ctx, r.k(key), increment, member)
}

func (r *redisPrefixCmdable) ZInterStore(ctx context.Context, destination string, store *redis.ZStore) *redis.IntCmd {
	return r.cmdable.ZInterStore(ctx, r.k(destination), store)
}

func (r *redisPrefixCmdable) ZPopMax(ctx context.Context, key string, count ...int64) *redis.ZSliceCmd {
	return r.cmdable.ZPopMax(ctx, r.k(key), count...)
}

func (r *redisPrefixCmdable) ZPopMin(ctx context.Context, key string, count ...int64) *redis.ZSliceCmd {
	return r.cmdable.ZPopMin(ctx, r.k(key), count...)
}

func (r *redisPrefixCmdable) ZRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd {
	return r.cmdable.ZRange(ctx, r.k(key), start, stop)
}

func (r *redisPrefixCmdable) ZRangeWithScores(ctx context.Context, key string, start, stop int64) *redis.ZSliceCmd {
	return r.cmdable.ZRangeWithScores(ctx, r.k(key), start, stop)
}

func (r *redisPrefixCmdable) ZRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.StringSliceCmd {
	return r.cmdable.ZRangeByScore(ctx, r.k(key), opt)
}

func (r *redisPrefixCmdable) ZRangeByLex(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.StringSliceCmd {
	return r.cmdable.ZRangeByLex(ctx, r.k(key), opt)
}

func (r *redisPrefixCmdable) ZRangeByScoreWithScores(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.ZSliceCmd {
	return r.cmdable.ZRangeByScoreWithScores(ctx, r.k(key), opt)
}

func (r *redisPrefixCmdable) ZRank(ctx context.Context, key, member string) *redis.IntCmd {
	return r.cmdable.ZRank(ctx, r.k(key), member)
}

func (r *redisPrefixCmdable) ZRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd {
	return r.cmdable.ZRem(ctx, r.k(key), members...)
}

func (r *redisPrefixCmdable) ZRemRangeByRank(ctx context.Context, key string, start, stop int64) *redis.IntCmd {
	return r.cmdable.ZRemRangeByRank(ctx, r.k(key), start, stop)
}

func (r *redisPrefixCmdable) ZRemRangeByScore(ctx context.Context, key, min, max string) *redis.IntCmd {
	return r.cmdable.ZRemRangeByScore(ctx, r.k(key), min, max)
}

func (r *redisPrefixCmdable) ZRemRangeByLex(ctx context.Context, key, min, max string) *redis.IntCmd {
	return r.cmdable.ZRemRangeByLex(ctx, r.k(key), min, max)
}

func (r *redisPrefixCmdable) ZRevRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd {
	return r.cmdable.ZRevRange(ctx, r.k(key), start, stop)
}

func (r *redisPrefixCmdable) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) *redis.ZSliceCmd {
	return r.cmdable.ZRevRangeWithScores(ctx, r.k(key), start, stop)
}

func (r *redisPrefixCmdable) ZRevRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.StringSliceCmd {
	return r.cmdable.ZRevRangeByScore(ctx, r.k(key), opt)
}

func (r *redisPrefixCmdable) ZRevRangeByLex(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.StringSliceCmd {
	return r.cmdable.ZRevRangeByLex(ctx, r.k(key), opt)
}

func (r *redisPrefixCmdable) ZRevRangeByScoreWithScores(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.ZSliceCmd {
	return r.cmdable.ZRevRangeByScoreWithScores(ctx, r.k(key), opt)
}

func (r *redisPrefixCmdable) ZRevRank(ctx context.Context, key, member string) *redis.IntCmd {
	return r.cmdable.ZRevRank(ctx, r.k(key), member)
}

func (r *redisPrefixCmdable) ZScore(ctx context.Context, key, member string) *redis.FloatCmd {
	return r.cmdable.ZScore(ctx, r.k(key), member)
}

func (r *redisPrefixCmdable) ZUnionStore(ctx context.Context, dest string, store *redis.ZStore) *redis.IntCmd {
	store.Keys = r.ks(store.Keys)
	return r.cmdable.ZUnionStore(ctx, r.k(dest), store)
}

func (r *redisPrefixCmdable) PFAdd(ctx context.Context, key string, els ...interface{}) *redis.IntCmd {
	return r.cmdable.PFAdd(ctx, r.k(key), els...)
}

func (r *redisPrefixCmdable) PFCount(ctx context.Context, keys ...string) *redis.IntCmd {
	return r.cmdable.PFCount(ctx, r.ks(keys)...)
}

func (r *redisPrefixCmdable) PFMerge(ctx context.Context, dest string, keys ...string) *redis.StatusCmd {
	return r.cmdable.PFMerge(ctx, r.k(dest), r.ks(keys)...)
}

type RedisClient struct {
	redisCmdable
	client    *redis.Client
	name      string
	hasPrefix bool
	prefix    string
}

func (r *RedisClient) Name() string {
	return r.name
}

func (r *RedisClient) Addr() string {
	return r.client.Options().Addr
}

func (r *RedisClient) Prefix(key string) string {
	if r.hasPrefix {
		return r.prefix + key
	} else {
		return key
	}
}

func (r *RedisClient) Prefixes(keys []string) []string {
	if r.hasPrefix {
		for i := 0; i < len(keys); i++ {
			keys[i] = r.prefix + keys[i]
		}
	}
	return keys
}

func (r *RedisClient) Wait(backoff BackoffPolicy) bool {
	for backoff.Next() {
		if err := r.Raw().Ping(context.Background()).Err(); err == nil {
			return true
		} else if backoff.End() {
			Logger.Errorf("[RedisClient][Wait] ping fail,err=%s", err)
			break
		}
		time.Sleep(backoff.Get())
	}
	return false
}

func (r *RedisClient) Exec(backoff BackoffPolicy, f func() error) error {
	cb := func() (err error) {
		defer func() {
			if err1 := recover(); err1 != nil {
				err = Recover2Error(err1)
			}
		}()
		return f()
	}
	var err error
	for backoff.Next() {
		if err = cb(); err == nil {
			return nil
		} else if !RedisErrRetry(err) {
			return err
		} else if backoff.End() {
			return err
		}
		time.Sleep(backoff.Get())
	}
	return nil
}

func (r *RedisClient) Raw() *redis.Client {
	return r.client
}

func (r *RedisClient) close() error {
	return r.client.Close()
}

func newRedisClient(name string, client *redis.Client, prefix string) *RedisClient {
	var cmdable redisCmdable
	if len(prefix) > 0 {
		cmdable = &redisPrefixCmdable{
			cmdable: client,
			prefix:  prefix,
		}
	} else {
		cmdable = client
	}
	return &RedisClient{
		redisCmdable: cmdable,
		client:       client,
		name:         name,
		hasPrefix:    len(prefix) > 0,
		prefix:       prefix,
	}
}
