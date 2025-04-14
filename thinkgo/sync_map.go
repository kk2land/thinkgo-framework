package thinkgo

import "sync"

type SyncMap struct {
	m      sync.Map
	mu     sync.Mutex
	create func(name string) (interface{}, error)
}

func NewInstanceMap(f func(name string) (interface{}, error)) *SyncMap {
	return &SyncMap{
		create: f,
	}
}

func (m *SyncMap) LoadOrCreate(name string) (interface{}, error) {
	var obj interface{}
	var ok bool
	if obj, ok = m.m.Load(name); ok {
		return obj, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	//double check
	if obj, ok = m.m.Load(name); ok {
		return obj, nil
	}
	var err error
	if obj, err = m.create(name); err != nil {
		return nil, err
	}
	m.m.Store(name, obj)
	return obj, nil
}

func (m *SyncMap) Delete(name string) {
	m.m.Delete(name)
}

func (m *SyncMap) Clear(f func(name string, inst interface{})) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var names []string
	m.m.Range(func(key, value interface{}) bool {
		name := key.(string)
		f(name, value)
		names = append(names, name)
		return true
	})
	for _, name := range names {
		m.m.Delete(name)
	}
}
