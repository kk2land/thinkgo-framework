package thinkgo

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type KeyStoreBackend interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte, ttl time.Duration, setf bool) error
}

// KeyStoreBackendRedis 基于redis的backend
type KeyStoreBackendRedis struct {
	Redis *RedisClient
}

func (b *KeyStoreBackendRedis) Get(key string) ([]byte, error) {
	s, err := b.Redis.Get(context.Background(), key).Result()
	if err != nil {
		return nil, err
	}
	return []byte(s), nil
}

func (b *KeyStoreBackendRedis) Set(key string, value []byte, ttl time.Duration, setf bool) error {
	ctx := context.Background()
	if setf && ttl > 0 {
		return b.Redis.SetExf(ctx, key, value, ttl).Err()
	}
	return b.Redis.Set(ctx, key, value, ttl).Err()
}

// KeyStoreBackendNone 空实现
type KeyStoreBackendNone struct {
}

func (b *KeyStoreBackendNone) Get(key string) ([]byte, error) {
	return nil, nil
}

func (b *KeyStoreBackendNone) Set(key string, value []byte, ttl time.Duration, setf bool) error {
	return nil
}

// keyStoreBackendAsync 封装backend，写时会异步写入backend
type keyStoreBackendAsyncItem struct {
	data []byte
	ttl  time.Duration
	setF bool
}

type keyStoreBackendAsync struct {
	KeyStoreBackend
	logger          FieldLogger
	changes         *CMap[interface{}]
	flushOnce       sync.Once
	flushPeriod     time.Duration
	flushMaxSize    uint64
	flushSetCounter uint64
	flushWriter     func(data []map[string]interface{})
	flushQueue      *GoQueue
}

func NewKeyStoreBackendAsync(
	name string,
	flushPeriod time.Duration,
	flushMaxSize int,
	backend KeyStoreBackend,
) KeyStoreBackend {
	b := &keyStoreBackendAsync{
		KeyStoreBackend: backend,
		logger:          Logger.With("keyStoreBackendAsync", name),
		changes:         CMapNew[interface{}](),
		flushPeriod:     flushPeriod,
		flushMaxSize:    uint64(flushMaxSize),
		flushSetCounter: 0,
	}
	switch v := backend.(type) {
	case *KeyStoreBackendRedis:
		b.flushWriter = func(data []map[string]interface{}) {
			b.writeRedis(data, v.Redis)
		}
	default:
		b.flushWriter = b.write
	}
	return b
}

func (b *keyStoreBackendAsync) Set(key string, value []byte, ttl time.Duration, setf bool) error {
	b.flushOnce.Do(b.flushStart)
	b.changes.Store(key, &keyStoreBackendAsyncItem{
		data: value,
		ttl:  ttl,
		setF: setf,
	})
	if atomic.AddUint64(&b.flushSetCounter, 1)%b.flushMaxSize == 0 {
		if data := b.changes.Reset(); len(data) > 0 {
			b.flushQueue.Send(data)
		}
	}
	return nil
}

func (b *keyStoreBackendAsync) flushStart() {
	AddShutdownHook(func(wait *sync.WaitGroup) {
		if data := b.changes.Reset(); len(data) > 0 {
			wait.Add(1)
			go func() {
				b.logger.Infof("shutdown write,len=%d", len(data))
				b.flushWriter(data)
				b.logger.Info("shutdown finish")
				wait.Done()
			}()
		}
	})
	b.flushQueue = NewGoQueue(1, func(obj interface{}) {
		b.flushWriter(obj.([]map[string]interface{}))
	})
	b.flushQueue.Start()
	go func() {
		tick := time.NewTicker(b.flushPeriod)
		for range tick.C {
			if data := b.changes.Reset(); len(data) > 0 {
				b.flushQueue.Send(data)
			}
		}
	}()
}

func (b *keyStoreBackendAsync) write(data []map[string]interface{}) {
	for _, v := range data {
		for k1, v2 := range v {
			item := v2.(*keyStoreBackendAsyncItem)
			err := b.KeyStoreBackend.Set(k1, item.data, item.ttl, item.setF)
			if err != nil {
				b.logger.Errorf("write backend failerr=%s", err)
			}
		}
	}
}

func (b *keyStoreBackendAsync) writeRedis(data []map[string]interface{}, r *RedisClient) {
	ctx := context.Background()
	p := r.Raw().Pipeline()
	for _, v := range data {
		for k1, v2 := range v {
			k1 = r.Prefix(k1)
			item := v2.(*keyStoreBackendAsyncItem)
			if item.setF && item.ttl > 0 {
				p.Eval(ctx, redisSetExf, []string{k1}, item.data, int(item.ttl.Seconds()))
			} else {
				p.Set(ctx, k1, item.data, item.ttl)
			}
		}
	}
	if cmders, err := p.Exec(ctx); err != nil {
		b.logger.Errorf("writeRedis pipeline fail,err=%s", err)
		for _, cmder := range cmders {
			if cmder.Err() != nil {
				b.logger.Errorf("writeRedis cmd fail,cmd=%s,err=%s", cmder.String(), cmder.Err())
			}
		}
	}
}
