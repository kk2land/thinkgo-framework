package thinkgo

import "sync"

// SyncMap 比sync.Map多了其他方法的并发map，性能没有sync.Map好
type SyncMap[T any] struct {
	mu     sync.Mutex
	items  sync.Map
	create func(name string) (T, error)
}

func NewSyncMap[T any](create func(key string) (T, error)) *SyncMap[T] {
	return &SyncMap[T]{
		create: create,
	}
}

func (m *SyncMap[T]) LoadOrCreate(key string) (T, error) {
	var val interface{}
	var ok bool
	if val, ok = m.items.Load(key); ok {
		return val.(T), nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	//double check
	if val, ok = m.items.Load(key); ok {
		return val, nil
	}
	var err error
	if val, err = m.create(key); err != nil {
		return val, err
	}
	m.items.Store(key, val)
	return val, nil
}

func (m *SyncMap[T]) Delete(key string) {
	m.items.Delete(key)
}

func (m *SyncMap[T]) Range(f func(key string, val T) bool) {
	m.items.Range(func(key, val any) bool {
		return f(key.(string), val.(T))
	})
}

func (m *SyncMap[T]) Clear(f func(key string, val T)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var keys []string
	m.items.Range(func(key, val any) bool {
		key1 := key.(string)
		f(key1, val.(T))
		keys = append(keys, key1)
		return true
	})
	for _, key := range keys {
		m.items.Delete(key)
	}
}
