package thinkgo

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"github.com/VictoriaMetrics/fastcache"
	"sync"
	"time"
)

// KeyStoreDefaultCacheSize 默认的lru cache的大小
var KeyStoreDefaultCacheSize = 1 * 1024 * 1024
var keyStoreDefaultCacheOnce sync.Once
var keyStoreDefaultCache *fastcache.Cache
var keyStoreBigSize = 64 * 1024
var keyStoreBigKeyMark byte = 255

// KeyStoreMarshalJSON 将value进行序列化-json
func KeyStoreMarshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// KeyStoreMarshalMsgpack 将value进行序列化-msgpack
//func KeyStoreMarshalMsgpack(v interface{}) ([]byte, error) {
//	return msgpack.Marshal(v)
//}

// KeyStoreUnmarshalJMap 将value进行反序列化-json
func KeyStoreUnmarshalJMap(data []byte) (interface{}, error) {
	var v JMap
	err := json.Unmarshal(data, &v)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func keyStoreBigKey(key string) []byte {
	buf := bytes.NewBuffer(make([]byte, 0, 1+len(key)))
	buf.WriteByte(keyStoreBigKeyMark)
	buf.WriteString(key)
	return buf.Bytes()
}

// KeyStoreItem 存储到KeyStore中的元素
type KeyStoreItem struct {
	Key         string
	Value       interface{}
	BackendTTL  time.Duration //写入backend时的过期时间，默认0则不过期
	BackendSetF bool          //写入backend时，是否只在首次写入时，才设置过期时间
}

// KeyStoreMarshal 将value进行序列化的函数
type KeyStoreMarshal func(v interface{}) ([]byte, error)

// KeyStoreUnmarshal 将value进行反序列化的函数
type KeyStoreUnmarshal func(data []byte) (interface{}, error)

// KeyStore 业务缓存map，没有读取到则会从backend读取，同时set也会写入backend
type KeyStore struct {
	backend   KeyStoreBackend
	marshal   KeyStoreMarshal
	unmarshal KeyStoreUnmarshal
	ttl       time.Duration
	cache     *fastcache.Cache
}

func NewKeyStore(
	backend KeyStoreBackend,
	marshal KeyStoreMarshal,
	unmarshal KeyStoreUnmarshal,
	ttl time.Duration,
) *KeyStore {
	keyStoreDefaultCacheOnce.Do(func() {
		keyStoreDefaultCache = fastcache.New(KeyStoreDefaultCacheSize)
	})
	return &KeyStore{
		backend:   backend,
		marshal:   marshal,
		unmarshal: unmarshal,
		ttl:       ttl,
		cache:     keyStoreDefaultCache,
	}
}

func NewKeyStoreWithCache(
	backend KeyStoreBackend,
	marshal KeyStoreMarshal,
	unmarshal KeyStoreUnmarshal,
	ttl time.Duration,
	cacheSize int,
) *KeyStore {
	return &KeyStore{
		backend:   backend,
		marshal:   marshal,
		unmarshal: unmarshal,
		ttl:       ttl,
		cache:     fastcache.New(cacheSize),
	}
}

func (s *KeyStore) withTTL(b []byte) []byte {
	expire := time.Now().Add(s.ttl)
	buf := make([]byte, 4+len(b))
	binary.BigEndian.PutUint32(buf, uint32(expire.Unix()))
	copy(buf[4:], b)
	return buf
}

func (s *KeyStore) cacheGet(key string) (interface{}, error) {
	var b []byte
	var ok bool
	var big = false
	if b, ok = s.cache.HasGet(nil, []byte(key)); !ok {
		if b = s.cache.GetBig(nil, keyStoreBigKey(key)); b == nil {
			return nil, nil
		} else {
			big = true
		}
	}
	expire := binary.BigEndian.Uint32(b)
	if time.Now().After(time.Unix(int64(expire), 0)) {
		if !big {
			s.cache.Del([]byte(key))
		} else {
			s.cache.Del(keyStoreBigKey(key))
		}
		return nil, nil
	}
	return s.unmarshal(b[4:])
}

// Load 对于单个key来说，不保证事务，需要结合key_lock来使用
func (s *KeyStore) Load(key string) (interface{}, error) {
	var err error
	var val interface{}
	var b []byte

	val, err = s.cacheGet(key)
	if err != nil {
		return nil, err
	} else if val != nil {
		return val, nil
	}
	b, err = s.backend.Get(key)
	if err != nil {
		return nil, err
	}
	val, err = s.unmarshal(b)
	if err != nil {
		return nil, err
	}
	b = s.withTTL(b)
	if len(b) < keyStoreBigSize {
		s.cache.Set([]byte(key), b)
	} else {
		s.cache.SetBig(keyStoreBigKey(key), b)
	}
	return val, nil
}

// LoadOrCreate 对于单个key来说，不保证事务，如果是创建并不会写入backend中
func (s *KeyStore) LoadOrCreate(key string, f func(k string) interface{}) (value interface{}, loaded bool, err error) {
	val, err := s.Load(key)
	if err != nil {
		return nil, false, err
	} else if val != nil {
		return val, true, nil
	}
	val = f(key)
	b, err := s.marshal(val)
	if err != nil {
		return nil, false, err
	}
	b = s.withTTL(b)
	if len(b) < keyStoreBigSize {
		s.cache.Set([]byte(key), b)
	} else {
		s.cache.SetBig(keyStoreBigKey(key), b)
	}
	return val, false, nil
}

// Store 对于单个key来说，不保证事务
func (s *KeyStore) Store(item *KeyStoreItem) error {
	b, err := s.marshal(item.Value)
	if err != nil {
		return err
	}
	b = s.withTTL(b)
	if len(b) < keyStoreBigSize {
		if s.cache.Has(keyStoreBigKey(item.Key)) {
			s.cache.Del(keyStoreBigKey(item.Key))
		}
		s.cache.Set([]byte(item.Key), b)
	} else {
		if s.cache.Has([]byte(item.Key)) {
			s.cache.Del([]byte(item.Key))
		}
		s.cache.SetBig(keyStoreBigKey(item.Key), b)
	}
	return s.backend.Set(item.Key, b, item.BackendTTL, item.BackendSetF)
}

// Lock 对某个key直接上内存锁
func (s *KeyStore) Lock(key string, f func(value interface{}) bool) error {
	return s.LockTimeout(key, 10*time.Second, f)
}

// LockTimeout 对某个key直接上内存锁，支持超时
func (s *KeyStore) LockTimeout(key string, timeout time.Duration, f func(value interface{}) bool) error {
	ret := KeyLockMemTimeout(key, timeout, func() interface{} {
		val, err := s.Load(key)
		if err != nil {
			return err
		}
		if f(val) {
			return s.Store(&KeyStoreItem{
				Key:   key,
				Value: val,
			})
		} else {
			return nil
		}
	})
	if ret != nil {
		return ret.(error)
	} else {
		return nil
	}
}
