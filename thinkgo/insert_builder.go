package thinkgo

import (
	"strings"
)

type InsertBuilderInterface interface {
	InitArgs(capacity int)
	AppendArgs(rowIdx int, args ...interface{})
	TakeArgs() (ret []interface{})
}

// InsertBuilder 构建mysql的insert ignore的批量操作
type InsertBuilder struct {
	cols     int
	sql1     string // "insert/replace [ignore] into xxx(...) values"
	sql2     string // "(?,?,...)"
	sql3     string // pgsql=" on conflict do nothing"
	capacity int
	sql      *strings.Builder
	args     []interface{}
	argsIdx  int
}

func NewInsertBuilder(
	mode int, //0=普通insert; 1=insert ignore; 2=replace(mysql);
	driver string,
	tableName string,
	cols []string,
) *InsertBuilder {
	//第一部分
	var op string
	switch mode {
	case 1:
		if driver == "postgres" {
			op = "insert"
		} else {
			op = "insert ignore"
		}
	case 2:
		op = "replace"
	default:
		op = "insert"
	}

	var colsStr string
	switch driver {
	case "postgres":
		colsStr = "\"" + strings.Join(cols, "\",\"") + "\""
	default:
		colsStr = "`" + strings.Join(cols, "`,`") + "`"
	}
	sql1 := op + " into " + tableName + "(" + colsStr + ") values"
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
	//第三部分
	var sql3 string
	if mode == 1 && driver == "postgres" {
		sql3 = " on conflict do nothing"
	}
	return &InsertBuilder{
		cols: len(cols),
		sql1: sql1,
		sql2: sql2,
		sql3: sql3,
	}
}

func (m *InsertBuilder) InitArgs(capacity int) {
	m.capacity = capacity
	//构建sql
	m.sql = &strings.Builder{}
	// len("insert into xxx(...) values") + capacity * len("(?,?,...)") + (capacity - 1) * len(",")
	m.sql.Grow(len(m.sql1) + len(m.sql2)*capacity + len(m.sql3) + capacity - 1)
	m.sql.WriteString(m.sql1)
	//构建参数
	m.args = make([]interface{}, capacity*m.cols+1)
}

func (m *InsertBuilder) AppendArgs(rowIdx int, args ...interface{}) {
	//构建sql
	if rowIdx > 0 {
		m.sql.WriteByte(',')
	}
	m.sql.WriteString(m.sql2)
	//构建参数
	j := rowIdx*m.cols + 1
	for k, arg := range args {
		m.args[j+k] = arg
	}
	m.argsIdx = j + m.cols
}

func (m *InsertBuilder) TakeArgs() (ret []interface{}) {
	//第一个参数是sql
	m.sql.WriteString(m.sql3)
	m.args[0] = m.sql.String()
	m.sql = nil
	if len(m.args) == m.argsIdx {
		//表示args的空间都已经铺满
		ret = m.args
	} else {
		ret = m.args[0:m.argsIdx]
	}
	m.args = nil
	return ret
}
