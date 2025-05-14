package thinkgo

import (
	"sync"
)

// CMap 并发map，比golang的sync.Map操作更加细致

// CMapShardsCountDefault 默认的CMap槽位
var CMapShardsCountDefault = 32 //runtime.NumCPU() * 2

// CMapShard CMap的槽
type CMapShard[K comparable, V any] struct {
	lock  sync.RWMutex
	idx   int
	items map[K]V
}

// Index 获取当前槽在CMap的下标
func (s *CMapShard[K, V]) Index() int {
	return s.idx
}

// Store 直接存当前槽的元素
func (s *CMapShard[K, V]) Store(key K, value V) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.items[key] = value
}

// Count 获取当前槽的元素个数
func (s *CMapShard[K, V]) Count() int {
	s.lock.RLock()
	cnt := len(s.items)
	s.lock.RUnlock()
	return cnt
}

// Load 读取当前槽的元素
func (s *CMapShard[K, V]) Load(key K) (V, bool) {
	s.lock.RLock()
	val, ok := s.items[key]
	s.lock.RUnlock()
	return val, ok
}

// LoadOrStore 读取当前槽的元素，当元素不存在，则存储；返回值loaded=true则表示是读取；loaded=false则表示是存储；
func (s *CMapShard[K, V]) LoadOrStore(key K, value V) (val V, loaded bool) {
	s.lock.RLock()
	val, ok := s.items[key]
	s.lock.RUnlock()
	if ok {
		return val, ok
	}
	s.lock.Lock()
	val, ok = s.items[key]
	if !ok {
		s.items[key] = value
		val = value
	}
	s.lock.Unlock()
	return val, ok
}

// LoadOrCreate 跟LoadOrStore一样，直接改成传一个函数来创建值
func (s *CMapShard[K, V]) LoadOrCreate(key K, create func(key K) V) (val V, loaded bool) {
	//先上读锁，判断是否存在，存在则直接返回
	s.lock.RLock()
	val, ok := s.items[key]
	s.lock.RUnlock()
	if ok {
		return val, ok
	}
	//然后上写锁，判断是否存在，不存在则创建
	s.lock.Lock()
	val, ok = s.items[key]
	if !ok {
		val = create(key)
		s.items[key] = val
	}
	s.lock.Unlock()
	return val, ok
}

// LoadOrCreateCb 跟LoadOrCreate，直接不返回而是改成回调，并且cb函数会在锁环境中调用
func (s *CMapShard[K, V]) LoadOrCreateF(key K, create func(key K) V, read func(val V, loaded bool)) {
	//先上读锁，判断是否存在，存在则回调
	s.lock.RLock()
	val, ok := s.items[key]
	if ok {
		read(val, ok)
		s.lock.RUnlock()
		return
	}
	s.lock.RUnlock()
	//然后上写锁，判断是否存在，不存在则创建
	s.lock.Lock()
	val, ok = s.items[key]
	if !ok {
		val = create(key)
		s.items[key] = val
	}
	read(val, ok)
	s.lock.Unlock()
}

// Delete 删除当前槽元素
func (s *CMapShard[K, V]) Delete(key K) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.items, key)
}

func (s *CMapShard[K, V]) Lock(f func(items map[K]V)) {
	s.lock.Lock()
	defer s.lock.Unlock()
	f(s.items)
}

func (s *CMapShard[K, V]) RLock(f func(items map[K]V)) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	f(s.items)
}

// CMap 并发map，比sync.Map操作更加细
type CMap[K comparable, V any] struct {
	shards      []*CMapShard[K, V]
	hash        func(key K) uint32
	shardsCount uint32
}

func NewCMap[K comparable, V any](count int, hash func(key K) uint32) *CMap[K, V] {
	shards := make([]*CMapShard[K, V], count)
	for i := 0; i < count; i++ {
		shards[i] = &CMapShard[K, V]{idx: i, items: make(map[K]V)}
	}
	return &CMap[K, V]{
		shards:      shards,
		shardsCount: uint32(count),
		hash:        hash,
	}
}

func NewCMapString[T any]() *CMap[string, T] {
	return NewCMap[string, T](CMapShardsCountDefault, cMapFnv32)
}

func NewCMapInt64[T any]() *CMap[int64, T] {
	return NewCMap[int64, T](CMapShardsCountDefault, func(key int64) uint32 { return uint32(key) })
}

func NewCMapUint64[T any]() *CMap[uint64, T] {
	return NewCMap[uint64, T](CMapShardsCountDefault, func(key uint64) uint32 { return uint32(key) })
}

// GetShard 通过key获取对应的槽
func (m *CMap[K, V]) GetShard(key K) *CMapShard[K, V] {
	return m.shards[m.hash(key)%m.shardsCount]
}

// GetShardByIdx 通过idx获取对应的槽
func (m *CMap[K, V]) GetShardByIdx(idx int) *CMapShard[K, V] {
	return m.shards[idx]
}

func (m *CMap[K, V]) Store(key K, value V) {
	m.GetShard(key).Store(key, value)
}

func (m *CMap[K, V]) Load(key K) (V, bool) {
	return m.GetShard(key).Load(key)
}

func (m *CMap[K, V]) LoadOrStore(key K, value V) (val V, loaded bool) {
	return m.GetShard(key).LoadOrStore(key, value)
}

func (m *CMap[K, V]) LoadOrCreate(key K, create func(key K) V) (val V, loaded bool) {
	return m.GetShard(key).LoadOrCreate(key, create)
}

func (m *CMap[K, V]) Delete(key K) {
	m.GetShard(key).Delete(key)
}

// Range 遍历元素，如果函数f返回false，则中断遍历
func (m *CMap[K, V]) Range(f func(k K, v V) bool) {
	for _, shard := range m.shards {
		shard.RLock(func(items map[K]V) {
			for k, v := range items {
				if !f(k, v) {
					return
				}
			}
		})
	}
}

// Reset 清空全部的槽，并且返回每个槽的元素
func (m *CMap[K, V]) Reset() []map[K]V {
	ret := make([]map[K]V, m.shardsCount)
	for i, shard := range m.shards {
		shard.Lock(func(items map[K]V) {
			ret[i] = items
			shard.items = make(map[K]V)
		})
	}
	return ret
}

func cMapFnv32(s string) uint32 {
	hash := uint32(2166136261)
	const prime32 = uint32(16777619)
	keyLength := len(s)
	for i := 0; i < keyLength; i++ {
		hash *= prime32
		hash ^= uint32(s[i])
	}
	return hash
}
