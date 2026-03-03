package thinkgo

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"hash/crc32"
	"math"
	"math/big"
	"math/rand"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"
)

// TimeFormatYmd 时间格式 = Y-items-d
var TimeFormatYmd = "2006-01-02"

// TimeFormatYmdHis 时间格式 = Y-items-d H:i:s
var TimeFormatYmdHis = "2006-01-02 15:04:05"

// BytesBuffer1024 共用的内存池
var BytesBuffer1024 = NewBytesBuffer(1024)

// RandSourceDefault 共用的随机数对象池
var RandSourceDefault = NewRandSource()

// ConversionJsonNull json的null
var ConversionJsonNull = []byte{'n', 'u', 'l', 'l'}

// VoidValue 空值
var VoidValue Void

// float 0.5
var Float0_5 = big.NewFloat(0.5)

// Void 无意义类型，一般给channel使用
type Void struct{}

// BytesBuffer 内存池
type BytesBuffer struct {
	sync.Pool
}

func NewBytesBuffer(initialSize int) *BytesBuffer {
	return &BytesBuffer{
		sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, initialSize))
			},
		},
	}
}

func (b *BytesBuffer) Get() *bytes.Buffer {
	return b.Pool.Get().(*bytes.Buffer)
}

func (b *BytesBuffer) Put(buf *bytes.Buffer) {
	buf.Reset()
	b.Pool.Put(buf)
}

// StringBuffer 字符串池
type StringBuffer struct {
	sync.Pool
}

// RandSource 随机数对象池
type RandSource struct {
	sync.Pool
}

func NewRandSource() *RandSource {
	return &RandSource{
		sync.Pool{
			New: func() interface{} {
				return rand.NewSource(time.Now().UnixNano())
			},
		},
	}
}

func (r *RandSource) Get() rand.Source {
	rd := r.Pool.Get().(rand.Source)
	return rd
}

func (r *RandSource) Put(rd rand.Source) {
	rd.Seed(time.Now().UnixNano())
	r.Pool.Put(rd)
}

// SafeSendChannel 安全send channel，不会panic，而是返回error
func SafeSendChannel[T any](C chan T, obj T) (err error) {
	defer func() {
		if err1 := recover(); err1 != nil {
			err = Recover2Error(err1)
		}
	}()
	C <- obj
	return nil
}

// Recover2Error recover中获取error
func Recover2Error(rec interface{}) (err error) {
	switch v := rec.(type) {
	case string:
		err = errors.New(v)
	case error:
		err = v
	default:
		err = errors.New("unknown panic")
	}
	return
}

type timeoutError interface {
	Timeout() bool
}

// ErrIsTimeout 判断err是timeout
func ErrIsTimeout(err error) bool {
	if e, ok := err.(timeoutError); ok {
		return e.Timeout()
	} else {
		return false
	}
}

// ErrIsBrokenPipe 判断err是连接中断
func ErrIsBrokenPipe(err interface{}) bool {
	var brokenPipe bool
	if ne, ok := err.(*net.OpError); ok {
		s := strings.ToLower(ne.Error())
		if strings.Contains(s, "broken pipe") ||
			strings.Contains(s, "connection refused") ||
			strings.Contains(s, "connection reset by peer") {
			brokenPipe = true
		}
	}
	return brokenPipe
}

// LogErrAndPanic 写错误日志，并且panic
func LogErrAndPanic(format string, v ...interface{}) {
	Logger.Errorf(format, v...)
	panic(fmt.Errorf(format, v...))
}

// Md5
func Md5(s string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(s)))
}

// Crc32
func Crc32(s string) uint32 {
	return crc32.ChecksumIEEE([]byte(s))
}

const (
	letterBytes  = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	letterIdBits = 6
	letterIdMask = 1<<letterIdBits - 1
	letterIdMax  = 63 / letterIdBits
)

// RandString 随机生成字符串
func RandString(n int) string {
	src := RandSourceDefault.Get()
	defer RandSourceDefault.Put(src)

	b := make([]byte, n)
	// A rand.Int63() generates 63 random bits, enough for letterIdMax letters!
	for i, cache, remain := n-1, src.Int63(), letterIdMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdMax
		}
		if idx := int(cache & letterIdMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdBits
		remain--
	}
	return *(*string)(unsafe.Pointer(&b))
}

// UnsafeStrToBytes 不安全的方式从string获取bytes，要确保string不会被释放
func UnsafeStrToBytes(s string) []byte {
	x := (*[2]uintptr)(unsafe.Pointer(&s))
	h := [3]uintptr{x[0], x[1], x[1]}
	return *(*[]byte)(unsafe.Pointer(&h))
}

// UnsafeBytesToStr 不安全方式从bytes获取string，要确保bytes不会被释放
func UnsafeBytesToStr(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// NginxHash nginx的upstream指定的hash一致的算法
// https://nginx.org/en/docs/http/ngx_http_upstream_module.html#hash
// server的配置: weight=1(默认); max_conns=0(默认,不限制); max_fails=0(非默认,不限制); 没有down;
func NginxHash(key string, num int) uint32 {
	return ((Crc32(key) >> 16) & 0x7fff) % uint32(num)
}

// SafeGo 安全启动协程，panic时会告警，并且shouldPanic控制是否会panic
// shouldPanic，发生panic时，是否终止终止进程
func SafeGo(shouldPanic bool, f func()) {
	SafeGo1(shouldPanic, true, f)
}

// SafeGo1 安全启动协程，panic时会告警，并且shouldPanic控制是否会panic
// shouldPanic，发生panic时，是否终止终止进程
// skipChannelClosed是否跳过"send on closed channel"错误
func SafeGo1(shouldPanic, skipChannelClosed bool, f func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("%v\n", r)
				stack := Stack(3, 5)
				if skipChannelClosed {
					//跳过：send一个已经被关闭的channel的错误
					if err, ok := r.(error); ok && err.Error() == "send on closed channel" {
						Logger.Error(err.Error())
						Logger.Errorf("stack=%s", stack)
						return
					}
				}
				var hint string
				if shouldPanic {
					hint = "退出"
				} else {
					hint = "错误"
				}
				OpsAlarmWithGroup(Config.OpsAlarm, shouldPanic, "%s 协程panic%s-%v", AppName, hint, r)
				Logger.Errorf("stack=%s", stack)
				if shouldPanic {
					panic(r)
				}
			}
		}()
		f()
	}()
}

// SortMap 对map的key进行排序，最后返回排序号的value slice
func SortMap[K comparable, V any](m map[K]V, less func(keys []K, i, j int) bool) []V {
	keys := make([]K, len(m))
	keysSize := 0
	for key, _ := range m {
		keys[keysSize] = key
		keysSize++
	}
	sort.Slice(keys, func(i, j int) bool {
		return less(keys, i, j)
	})
	ret := make([]V, len(m))
	for i, key := range keys {
		ret[i] = m[key]
	}
	return ret
}

func JsonBytesTrimQuote(b []byte) []byte {
	if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
		b = b[1 : len(b)-1]
	}
	return b
}

// TruncateBigFloat 对*big.Float按照精度四舍五入 scale = math.Pow10(precision)
func TruncateBigFloat(f *big.Float, scale *big.Float) *big.Float {
	val := new(big.Float).Mul(f, scale)
	// 四舍五入：tmp + 0.5，然后取整
	val.Add(val, Float0_5)
	intPart, _ := val.Int(nil) // 去掉小数部分
	// 转回浮点数：intPart / scale
	val.SetInt(intPart)
	return val.Quo(val, scale)
}

// TruncateFloat64 对float64按照精度四舍五入
func TruncateFloat64(f float64, precision int) float64 {
	factor := math.Pow(10, float64(precision))
	return math.Round(f*factor) / factor
}
