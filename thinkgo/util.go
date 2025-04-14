package thinkgo

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"
	"unsafe"
)

var TimeFormatYmd = "2006-01-02"
var TimeFormatYmdHis = "2006-01-02 15:04:05"
var BytesBuffer1024 = NewBytesBuffer(1024)
var RandSourceDefault = NewRandSource()
var ConversionJsonNull = []byte{'n', 'u', 'l', 'l'}
var VoidValue Void

type Void struct{}

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

type StringBuffer struct {
	sync.Pool
}

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

func SendChannelInterface(C chan interface{}, obj interface{}) (err error) {
	defer func() {
		if err1 := recover(); err1 != nil {
			err = Recover2Error(err1)
		}
	}()
	C <- obj
	return nil
}

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

func ErrIsTimeout(err error) bool {
	if e, ok := err.(timeoutError); ok {
		return e.Timeout()
	} else {
		return false
	}
}

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

func LogErrAndPanic(format string, v ...interface{}) {
	Logger.Errorf(format, v...)
	panic(fmt.Errorf(format, v...))
}

func Md5(s string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(s)))
}

func Crc32(s string) uint32 {
	return crc32.ChecksumIEEE([]byte(s))
}

const (
	letterBytes  = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	letterIdBits = 6
	letterIdMask = 1<<letterIdBits - 1
	letterIdMax  = 63 / letterIdBits
)

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

func ClientIp(r *http.Request) string {
	if r == nil {
		return "127.0.0.1"
	}
	val := r.Header.Get("X-Forwarded-For")
	if len(val) > 0 {
		vals := strings.Split(val, ",")
		for _, v := range vals {
			v = strings.TrimSpace(v)
			if len(v) > 0 && v != "unknown" {
				return v
			}
		}
	} else {
		val = r.Header.Get("X-Real-IP")
		if len(val) > 0 {
			return val
		}
	}
	return r.RemoteAddr
}

func Values2JMap(values url.Values) JMap {
	data := make(JMap)
	for k, v := range values {
		if len(v) == 1 {
			data[k] = v[0]
		} else if len(v) > 1 {
			data[k] = v
		}
	}
	return data
}

func ParseForm(r *http.Request) (data JMap, err error) {
	ct := r.Header.Get("Content-Type")
	if ct == "application/json" {
		if r.Body == nil {
			err = errors.New("missing json body")
			return
		}
		var body []byte
		if body, err = ioutil.ReadAll(r.Body); err != nil {
			return
		} else {
			err = json.Unmarshal(body, &data)
		}
	} else if strings.HasPrefix(ct, "multipart/form-data") {
		err = r.ParseMultipartForm(HttpEngine().MaxMultipartMemory)
		if err != nil {
			return
		}
		data = Values2JMap(r.MultipartForm.Value)
	} else {
		var f = ct != "application/x-www-form-urlencoded"
		if f {
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		if err = r.ParseForm(); err != nil {
			return
		}
		data = Values2JMap(r.PostForm)
		if f {
			r.Header.Set("Content-Type", ct)
		}
	}
	return
}

func SetProcessName(name string) error {
	argv0str := (*reflect.StringHeader)(unsafe.Pointer(&os.Args[0]))
	argv0 := (*[1 << 30]byte)(unsafe.Pointer(argv0str.Data))[:argv0str.Len]
	n := copy(argv0, name)
	if n < len(argv0) {
		argv0[n] = 0
	}
	// Syscall PRCTL, not working on Darwin for me
	// bytes := append([]byte(name), 0)
	// ptr := unsafe.Pointer(&bytes[0])

	// if _, _, errno := syscall.RawSyscall6(syscall.SYS_PRCTL, syscall.PR_SET_NAME, uintptr(ptr), 0, 0, 0, 0); errno != 0 {
	// 	return syscall.Errno(errno)
	// }
	return nil
}

func UnsafeStrToBytes(s string) []byte {
	x := (*[2]uintptr)(unsafe.Pointer(&s))
	h := [3]uintptr{x[0], x[1], x[1]}
	return *(*[]byte)(unsafe.Pointer(&h))
}

func UnsafeBytesToStr(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func NginxHash(key string, num int) int {
	return int((Crc32(key)>>16)&0x7fff) % num
}
