package thinkgo

import (
	"strings"
)

// InsertOnDuplicateBuilder 构建mysql的insert on duplicate key update的批量操作
type InsertOnDuplicateBuilder struct {
	cols     int
	sql1     string // "insert into xxx(...) values"
	sql2     string // "(?,?,...)"
	sql3     string // " on duplicate key update `col`=values(`col`)"
	capacity int
	sql      *strings.Builder
	args     []interface{}
}

func NewInsertOnDuplicateBuilder(
	tableName string,
	cols []string,
	keyNum int, //前几列是key
) *InsertOnDuplicateBuilder {
	//第一部分
	sql1 := "insert into `" + tableName + "`(`" + strings.Join(cols, "`,`") + "`) values"
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
	{
		tmp := make([]string, len(cols)-keyNum)
		for i, col := range cols {
			if i < keyNum {
				continue
			}
			tmp[i-keyNum] = "`" + col + "`=values(`" + col + "`)"
		}
		sql3 = " on duplicate key update " + strings.Join(tmp, ",")
	}
	return &InsertOnDuplicateBuilder{
		cols: len(cols),
		sql1: sql1,
		sql2: sql2,
		sql3: sql3,
	}
}

func (m *InsertOnDuplicateBuilder) InitArgs(capacity int) {
	m.capacity = capacity
	//构建sql
	m.sql = &strings.Builder{}
	// len("insert into xxx(...) values") + capacity * len("(?,?,...)") + (capacity - 1) * len(",") + len(" on duplicate key update ...")
	m.sql.Grow(len(m.sql1) + len(m.sql2)*capacity + capacity - 1 + len(m.sql3))
	m.sql.WriteString(m.sql1)
	//构建参数
	m.args = make([]interface{}, capacity*m.cols+1)
}

func (m *InsertOnDuplicateBuilder) AppendArgs(i int, args ...interface{}) {
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

func (m *InsertOnDuplicateBuilder) TakeArgs() []interface{} {
	//第一个参数是sql
	m.sql.WriteString(m.sql3)
	m.args[0] = m.sql.String()
	m.sql = nil
	ret := m.args
	m.args = nil
	return ret
}
