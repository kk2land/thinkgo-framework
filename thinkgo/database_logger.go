package thinkgo

import (
	"xorm.io/xorm/log"
)

// 将框架logger兼容xorm的logger

type dbLogger struct {
	FieldLogger
	showSQL bool
}

func (l *dbLogger) Level() log.LogLevel {
	if l.IsDebug() {
		return log.LOG_DEBUG
	}
	return log.LOG_INFO
}

func (l *dbLogger) SetLevel(level log.LogLevel) {
	//nothing
}

func (l *dbLogger) ShowSQL(show ...bool) {
	if len(show) == 0 {
		l.showSQL = true
		return
	}
	l.showSQL = show[0]
}

func (l *dbLogger) IsShowSQL() bool {
	return l.showSQL
}
