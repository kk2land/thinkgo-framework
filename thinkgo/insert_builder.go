package thinkgo

import (
	"strings"
)

// InsertBuilder 构建mysql的insert ignore的批量操作
type InsertBuilder struct {
	cols     int
	sql1     string // "insert/replace [ignore] into xxx(...) values"
	sql2     string // "(?,?,...)"
	capacity int
	sql      *strings.Builder
	args     []interface{}
}

func NewInsertBuilder(
	mode int, //0=普通insert; 1=insert ignore; 2=replace;
	tableName string,
	cols []string,
) *InsertBuilder {
	//第一部分
	var op string
	switch mode {
	case 1:
		op = "insert ignore"
	case 2:
		op = "replace"
	default:
		op = "insert"
	}
	sql1 := op + " into " + tableName + "(`" + strings.Join(cols, "`,`") + "`) values"
	//第二部分
	var sql2 string
	{
		var tmp strings.Builder
		tmp.Grow(2 + len(cols)*2 - 1)
		tmp.WriteByte('(')
		for i, _ := range cols {
			tmp.WriteByte('?')
			if i < len(cols)-1 {
				tmp.WriteByte(',')
			}
		}
		tmp.WriteByte(')')
		sql2 = tmp.String()
	}
	return &InsertBuilder{
		cols: len(cols),
		sql1: sql1,
		sql2: sql2,
	}
}

func (m *InsertBuilder) InitArgs(capacity int) {
	m.capacity = capacity
	//构建sql
	m.sql = &strings.Builder{}
	// len("insert into xxx(...) values") + capacity * len("(?,?,...)") + (capacity - 1) * len(",")
	m.sql.Grow(len(m.sql1) + len(m.sql2)*capacity + capacity - 1)
	m.sql.WriteString(m.sql1)
	//构建参数
	m.args = make([]interface{}, capacity*m.cols+1)
}

func (m *InsertBuilder) AppendArgs(i int, args ...interface{}) {
	//构建sql
	m.sql.WriteString(m.sql2)
	if i < m.capacity-1 {
		m.sql.WriteByte(',')
	}
	//构建参数
	j := i*m.cols + 1
	for k, arg := range args {
		m.args[j+k] = arg
	}
}

func (m *InsertBuilder) TakeArgs() []interface{} {
	//第一个参数是sql
	m.args[0] = m.sql.String()
	m.sql = nil
	ret := m.args
	m.args = nil
	return ret
}
