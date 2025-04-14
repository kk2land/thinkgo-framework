package thinkgo

import (
	"sync"
)

// CMap 并发map，比golang的sync.Map操作更加细致

var CMapShardsCountDefault = 32 //runtime.NumCPU() * 2

type CMapShard[T any] struct {
	sync.RWMutex
	idx   int
	items map[string]T
}

func (s *CMapShard[T]) Index() int {
	return s.idx
}

func (s *CMapShard[T]) Items() map[string]T {
	return s.items
}

func (s *CMapShard[T]) Store(key string, value T) {
	s.Lock()
	s.items[key] = value
	s.Unlock()
}

func (s *CMapShard[T]) Count() int {
	s.RLock()
	cnt := len(s.items)
	s.RUnlock()
	return cnt
}

func (s *CMapShard[T]) Load(key string) (T, bool) {
	s.RLock()
	val, ok := s.items[key]
	s.RUnlock()
	return val, ok
}

func (s *CMapShard[T]) LoadOrStore(key string, value T) (val T, loaded bool) {
	s.RLock()
	val, ok := s.items[key]
	s.RUnlock()
	if ok {
		return val, ok
	}
	s.Lock()
	val, ok = s.items[key]
	if !ok {
		s.items[key] = value
		val = value
	}
	s.Unlock()
	return val, ok
}

func (s *CMapShard[T]) LoadOrCreate(key string, f func(k string) T) (val T, loaded bool) {
	s.RLock()
	val, ok := s.items[key]
	s.RUnlock()
	if ok {
		return val, ok
	}
	s.Lock()
	val, ok = s.items[key]
	if !ok {
		val = f(key)
		s.items[key] = val
	}
	s.Unlock()
	return val, ok
}

func (s *CMapShard[T]) LoadOrCreateCb(key string, f func(k string) T, cb func(val T, loaded bool)) {
	s.RLock()
	val, ok := s.items[key]
	if ok {
		cb(val, ok)
		s.RUnlock()
		return
	}
	s.RUnlock()
	//创建
	s.Lock()
	val, ok = s.items[key]
	if !ok {
		val = f(key)
		s.items[key] = val
	}
	cb(val, ok)
	s.Unlock()
}

func (s *CMapShard[T]) Delete(key string) {
	s.Lock()
	delete(s.items, key)
	s.Unlock()
}

func (s *CMapShard[T]) DeleteCb(key string, cb func(map[string]T)) {
	s.Lock()
	delete(s.items, key)
	cb(s.items)
	s.Unlock()
}

type CMap[T any] struct {
	shards      []*CMapShard[T]
	shardsCount uint32
}

func CMapNew[T any]() *CMap[T] {
	return CMapNew1[T](CMapShardsCountDefault)
}

func CMapNew1[T any](count int) *CMap[T] {
	shards := make([]*CMapShard[T], count)
	for i := 0; i < count; i++ {
		shards[i] = &CMapShard[T]{idx: i, items: make(map[string]T)}
	}
	return &CMap[T]{shards: shards, shardsCount: uint32(count)}
}

func (m *CMap[T]) GetShard(key string) *CMapShard[T] {
	return m.shards[cMapFnv32(key)%m.shardsCount]
}

func (m *CMap[T]) GetShardByIdx(idx int) *CMapShard[T] {
	return m.shards[idx]
}

func (m *CMap[T]) Store(key string, value T) {
	m.GetShard(key).Store(key, value)
}

func (m *CMap[T]) Load(key string) (T, bool) {
	return m.GetShard(key).Load(key)
}

func (m *CMap[T]) LoadOrStore(key string, value T) (val T, loaded bool) {
	return m.GetShard(key).LoadOrStore(key, value)
}

func (m *CMap[T]) LoadOrCreate(key string, f func(k string) T) (val T, loaded bool) {
	return m.GetShard(key).LoadOrCreate(key, f)
}

func (m *CMap[T]) Delete(key string) {
	m.GetShard(key).Delete(key)
}

func (m *CMap[T]) Range(f func(k string, v T) bool) {
	for _, shard := range m.shards {
		shard.RLock()
		for k, v := range shard.items {
			if !f(k, v) {
				shard.RUnlock()
				return
			}
		}
		shard.RUnlock()
	}
}

func (m *CMap[T]) Reset() []map[string]T {
	ret := make([]map[string]T, m.shardsCount)
	for i, shard := range m.shards {
		shard.Lock()
		ret[i] = shard.items
		shard.items = make(map[string]T)
		shard.Unlock()
	}
	return ret
}

func cMapFnv32(key string) uint32 {
	hash := uint32(2166136261)
	const prime32 = uint32(16777619)
	keyLength := len(key)
	for i := 0; i < keyLength; i++ {
		hash *= prime32
		hash ^= uint32(key[i])
	}
	return hash
}
