package thinkgo

import (
	"git.hy545.cc/crypto/thinkgo-framework/lru"
	"math"
	"sync"
	"time"
)

type cMapLRUShardItem[T any] struct {
	v        T
	expireAt time.Time
}

func (i *cMapLRUShardItem[T]) expired() bool {
	return time.Now().After(i.expireAt)
}

type CMapLRUShard[T any] struct {
	lock  sync.Mutex
	idx   int
	cache *lru.Cache
	ttl   time.Duration
}

func (s *CMapLRUShard[T]) Index() int {
	return s.idx
}

func (s *CMapLRUShard[T]) Store(key string, value T) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.cache.Add(key, &cMapLRUShardItem[T]{v: value, expireAt: time.Now().Add(s.ttl)})
}

// Storef 如果存在则不更新ttl
func (s *CMapLRUShard[T]) Storef(key string, value T) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if val, ok := s.cache.Get(key); ok {
		val.(*cMapLRUShardItem[T]).v = value
	} else {
		s.cache.Add(key, &cMapLRUShardItem[T]{v: value, expireAt: time.Now().Add(s.ttl)})
	}
}

func (s *CMapLRUShard[T]) Load(key string) (T, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if val, ok := s.cache.Get(key); ok {
		item := val.(*cMapLRUShardItem[T])
		if !item.expired() {
			return item.v, true
		}
		s.cache.Remove(key)
	}
	var ret T
	return ret, false
}

func (s *CMapLRUShard[T]) LoadOrStore(key string, value T) (T, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	val, ok := s.cache.Get(key)
	if ok {
		item := val.(*cMapLRUShardItem[T])
		if !item.expired() {
			return item.v, true
		}
	}
	s.cache.Add(key, &cMapLRUShardItem[T]{v: value, expireAt: time.Now().Add(s.ttl)})
	return value, false
}

func (s *CMapLRUShard[T]) LoadOrCreate(key string, create func(k string) T) (T, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	obj, ok := s.cache.Get(key)
	if ok {
		item := obj.(*cMapLRUShardItem[T])
		if !item.expired() {
			return item.v, true
		}
	}
	val := create(key)
	s.cache.Add(key, &cMapLRUShardItem[T]{v: val, expireAt: time.Now().Add(s.ttl)})
	return val, false
}

func (s *CMapLRUShard[T]) Delete(key string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.cache.Remove(key)
}

func (s *CMapLRUShard[T]) Lock(f func(*lru.Cache)) {
	s.lock.Lock()
	defer s.lock.Unlock()
	f(s.cache)
}

// CMapLRU 带有LRU功能的并发map
type CMapLRU[T any] struct {
	shards      []*CMapLRUShard[T]
	shardsCount uint32
}

func NewCMapLRU[T any](size int, ttl time.Duration) *CMapLRU[T] {
	return NewCMapLRUWithCount[T](CMapShardsCountDefault, size, ttl)
}

func NewCMapLRUWithCount[T any](shardsCount int, size int, ttl time.Duration) *CMapLRU[T] {
	shards := make([]*CMapLRUShard[T], shardsCount)
	shardSize := int(math.Max(10.0, float64(size/shardsCount)))
	for i := 0; i < shardsCount; i++ {
		shards[i] = &CMapLRUShard[T]{
			idx:   i,
			cache: lru.New(shardSize),
			ttl:   ttl,
		}
	}
	return &CMapLRU[T]{shards: shards, shardsCount: uint32(shardsCount)}
}

func (m *CMapLRU[T]) GetShard(key string) *CMapLRUShard[T] {
	return m.shards[cMapFnv32(key)%m.shardsCount]
}

func (m *CMapLRU[T]) GetShardByIdx(idx int) *CMapLRUShard[T] {
	return m.shards[idx]
}

func (m *CMapLRU[T]) Store(key string, value T) {
	m.GetShard(key).Store(key, value)
}

func (m *CMapLRU[T]) Load(key string) (T, bool) {
	return m.GetShard(key).Load(key)
}

func (m *CMapLRU[T]) LoadOrStore(key string, value T) (val T, loaded bool) {
	return m.GetShard(key).LoadOrStore(key, value)
}

func (m *CMapLRU[T]) LoadOrCreate(key string, create func(k string) T) (val T, loaded bool) {
	return m.GetShard(key).LoadOrCreate(key, create)
}

func (m *CMapLRU[T]) Delete(key string) {
	m.GetShard(key).Delete(key)
}
