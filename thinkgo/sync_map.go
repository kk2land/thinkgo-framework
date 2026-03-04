package thinkgo

import (
	"sync"
)

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

func (m *SyncMap[T]) Load(key string) (val T, ok bool) {
	if val1, ok1 := m.items.Load(key); ok1 {
		return val1.(T), ok1
	}
	return
}

func (m *SyncMap[T]) LoadOrCreate(key string) (val T, err error) {
	var val1 interface{}
	var ok1 bool
	if val1, ok1 = m.items.Load(key); ok1 {
		return val1.(T), nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	//double check
	if val1, ok1 = m.items.Load(key); ok1 {
		return val1.(T), nil
	}
	var err1 error
	if val1, err1 = m.create(key); err1 != nil {
		err = err1
		return
	}
	m.items.Store(key, val1)
	return val1.(T), nil
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

	m.items.Range(func(key, val any) bool {
		key1 := key.(string)
		f(key1, val.(T))
		m.items.Delete(key)
		return true
	})
}
