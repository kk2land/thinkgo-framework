package thinkgo

import (
	"bytes"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

//go get -u go.uber.org/zap

const loggerTimeFormatRFC3339 = "2006-01-02T15:04:05.000Z07:00"
const loggerTimeFormatDaily = "20060102"
const loggerFieldId = "id"

type fieldLoggerField interface {
	Key() string
	Write(b *bytes.Buffer)
}

func newFieldLoggerField(k, v interface{}) fieldLoggerField {
	key := k.(string)
	if v == nil {
		return fieldLoggerFieldK(key)
	}

	var str string
	switch val := v.(type) {
	case fmt.Stringer:
		str = val.String()
	case error:
		str = val.Error()
	case string:
		str = val
	case bool:
		if val {
			str = "true"
		} else {
			str = "false"
		}
	case int:
		str = strconv.FormatInt(int64(val), 10)
	case int8:
		str = strconv.FormatInt(int64(val), 10)
	case int16:
		str = strconv.FormatInt(int64(val), 10)
	case int32:
		str = strconv.FormatInt(int64(val), 10)
	case int64:
		str = strconv.FormatInt(val, 10)
	case uint:
		str = strconv.FormatUint(uint64(val), 10)
	case uint8:
		str = strconv.FormatUint(uint64(val), 10)
	case uint16:
		str = strconv.FormatUint(uint64(val), 10)
	case uint32:
		str = strconv.FormatUint(uint64(val), 10)
	case uint64:
		str = strconv.FormatUint(val, 10)
	default:
		str = fmt.Sprintf("%value", val)
	}
	return &fieldLoggerFieldKV{key: key, value: str}
}

type fieldLoggerFieldKV struct {
	key   string
	value string
}

func (kv *fieldLoggerFieldKV) Key() string {
	return kv.key
}

func (kv *fieldLoggerFieldKV) Write(b *bytes.Buffer) {
	b.WriteString(kv.key)
	b.WriteByte('=')
	b.WriteString(kv.value)
}

type fieldLoggerFieldK string

func (k fieldLoggerFieldK) Key() string {
	return string(k)
}

func (k fieldLoggerFieldK) Write(b *bytes.Buffer) {
	b.WriteString(string(k))
}

var loggerPid string

// FieldLogger 框架的日志对象
type FieldLogger interface {
	With(args ...interface{}) FieldLogger

	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Printf(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Panicf(format string, args ...interface{})

	Debug(args ...interface{})
	Info(args ...interface{})
	Print(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
	Fatal(args ...interface{})
	Panic(args ...interface{})

	Out() io.Writer
	IsDebug() bool
	SetLevel(level logrus.Level)
	SetOutput(output io.Writer)
}

func iniAppLogger(name string) FieldLogger {
	loggerPid = AppName + "," + strconv.Itoa(Pid)
	if len(name) == 0 {
		name = "app"
	}
	return NewLogger(name)
}

// NewLogger 基于name创建按天分隔写日志文件的日志对象
func NewLogger(name string) FieldLogger {
	logger := logrus.New()
	if AppDebug {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}
	logger.SetFormatter(&loggerLogrusFormat{})
	if !IsAtty && os.Getenv(envKeyLogConsole) != "1" {
		logger.SetNoLock()
		logger.SetOutput(newLoggerDailyWriter(name))
	} else {
		logger.SetOutput(os.Stdout)
	}
	log.SetOutput(logger.Out)
	return &loggerLogrus{Logger: logger}
}

// loggerLogrus 封装logrus
const loggerLogrusField = "tk"

type loggerLogrus struct {
	*logrus.Logger
}

// With 给日志添加前缀，并且返回新的日志对象
//
//	args参数的个数必须是偶数，最终会拼接成"key1=value1,key2=value2..."；
//	如果value1是nil，则最终会拼接成"key1,key2=value2"
func (l *loggerLogrus) With(args ...interface{}) FieldLogger {
	size := len(args)
	if size == 0 {
		return l
	} else if size%2 != 0 {
		l.Warnf("[loggerLogrus] with参数不是偶数个-%d", size)
		return l
	}
	var fields = make([]fieldLoggerField, 0, size/2)
	for i := 0; i < size; i += 2 {
		fields = append(fields, newFieldLoggerField(args[i], args[i+1]))
	}
	return &loggerLogrusEntry{l.WithField(loggerLogrusField, fields)}
}

func (l *loggerLogrus) Out() io.Writer {
	return l.Logger.Out
}

func (l *loggerLogrus) IsDebug() bool {
	return l.Level >= logrus.DebugLevel
}

func (l *loggerLogrus) SetLevel(level logrus.Level) {
	l.Logger.SetLevel(level)
}

func (l *loggerLogrus) SetOutput(output io.Writer) {
	l.Logger.SetOutput(output)
}

type loggerLogrusEntry struct {
	*logrus.Entry
}

func (l *loggerLogrusEntry) With(args ...interface{}) FieldLogger {
	size := len(args)
	if size == 0 {
		return l
	} else if size%2 != 0 {
		l.Warnf("[loggerLogrusEntry] with参数不是偶数个-%d", size)
		return l
	}

	var oldFields = l.Data[loggerLogrusField].([]fieldLoggerField)
	var newFields = make([]fieldLoggerField, 0, size/2)
	var newFieldSet = make(map[string]Void, size/2)
	for i := 0; i < size; i += 2 {
		k := args[i].(string)
		newFields = append(newFields, newFieldLoggerField(args[i], args[i+1]))
		newFieldSet[k] = VoidValue
	}
	var fields = make([]fieldLoggerField, 0, len(oldFields)+size/2)
	for _, field := range oldFields {
		if _, ok := newFieldSet[field.Key()]; !ok {
			fields = append(fields, field)
		}
	}
	fields = append(fields, newFields...)
	return &loggerLogrusEntry{l.Entry.WithField(loggerLogrusField, fields)}
}

func (l *loggerLogrusEntry) Out() io.Writer {
	return l.Entry.Logger.Out
}

func (l *loggerLogrusEntry) IsDebug() bool {
	return l.Entry.Level >= logrus.DebugLevel
}

func (l *loggerLogrusEntry) SetLevel(level logrus.Level) {
	//do nothing
}

func (l *loggerLogrusEntry) SetOutput(output io.Writer) {
	//do nothing
}

type loggerLogrusFormat struct{}

func (f *loggerLogrusFormat) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	b.WriteByte('[')
	b.WriteString(entry.Time.Format(loggerTimeFormatRFC3339))
	b.Write([]byte{']', '['})
	b.WriteString(loggerPid)
	b.Write([]byte{']', '['})
	b.WriteString(entry.Level.String())
	b.WriteByte(']')

	if obj, ok := entry.Data[loggerLogrusField]; ok {
		fields := obj.([]fieldLoggerField)
		fieldsEnd := len(fields) - 1
		for i, field := range fields {
			field.Write(b)
			if i != fieldsEnd {
				b.WriteByte(',')
			}
		}
	}
	b.WriteByte(' ')
	b.WriteString(entry.Message)
	if entry.HasCaller() {
		b.WriteString(fmt.Sprintf(" in \"%s@%s:%d\"", entry.Caller.Function, entry.Caller.File, entry.Caller.Line))
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}

// loggerDailyWriter 分日期写入writer
type loggerDailyWriter struct {
	pathFormat string
	currentFD  *os.File
	currentDay int32
	lock       sync.Mutex
}

func newLoggerDailyWriter(name string) *loggerDailyWriter {
	if !strings.HasSuffix(name, "_") {
		name = name + "_"
	}
	w := &loggerDailyWriter{
		pathFormat: filepath.Join(RuntimePath, "log", name+"%s.log"),
	}
	return w
}

func (w *loggerDailyWriter) open(now time.Time) error {
	day := int32(now.Day())
	if atomic.LoadInt32(&w.currentDay) == day {
		return nil
	}

	w.lock.Lock()
	defer w.lock.Unlock()
	if atomic.LoadInt32(&w.currentDay) == day {
		return nil
	}

	var err error
	if w.currentFD != nil {
		if err = w.currentFD.Close(); err != nil {
			w.currentFD = nil
			return err
		}
		w.currentFD = nil
	}
	path := fmt.Sprintf(w.pathFormat, now.Format(loggerTimeFormatDaily))
	var fd *os.File
	if fd, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0664); err != nil {
		return err
	}
	w.currentFD = fd
	atomic.StoreInt32(&w.currentDay, day)
	return nil
}

func (w *loggerDailyWriter) Write(p []byte) (n int, err error) {
	n = -1
	now := time.Now()
	if err = w.open(now); err != nil {
		return
	}
	return w.currentFD.Write(p)
}

func (w *loggerDailyWriter) Close() error {
	if w.currentFD != nil {
		return w.currentFD.Close()
	}
	return nil
}
