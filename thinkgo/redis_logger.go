package thinkgo

import (
	"context"
)

type redisLogger struct {
	FieldLogger
}

func (l *redisLogger) Printf(ctx context.Context, format string, v ...interface{}) {
	l.Debugf(format, v...)
}
