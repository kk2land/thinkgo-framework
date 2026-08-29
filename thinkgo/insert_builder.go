package thinkgo

import (
	"strings"
)

type InsertBuilderInterface interface {
	// InitArgs 初始化总共有几行数据
	InitArgs(capacity int)
	// AppendArgs 添加数据，rowIdx表示第几行数据，从0开始
	AppendArgs(rowIdx int, args ...interface{})
	AppendBean(rowIdx int, db *DBInstance, bean interface{}) error
	TakeArgs() (ret []interface{})
	SetTable(t string)
	GetTable() string
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
	table    string
	colsMap  map[string]int
	colsStr  string
}

func NewInsertBuilder(
	mode int, //0=普通insert; 1=insert ignore; 2=replace(mysql);
	driver string,
	table string,
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
	var sql1 = op + " into " // + table + "(" + colsStr + ") values"
	//第二部分
	var sql2 string
	var colsMap = make(map[string]int)
	{
		var tmp strings.Builder
		tmp.Grow(2 + len(cols)*2 - 1)
		tmp.WriteByte('(')
		for i, col := range cols {
			colsMap[col] = i
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
		cols:    len(cols),
		sql1:    sql1,
		sql2:    sql2,
		sql3:    sql3,
		table:   table,
		colsStr: "(" + colsStr + ") values",
		colsMap: colsMap,
	}
}

func (m *InsertBuilder) InitArgs(capacity int) {
	m.capacity = capacity
	//构建sql
	m.sql = &strings.Builder{}
	// len("insert into xxx(...) values") + capacity * len("(?,?,...)") + (capacity - 1) * len(",")
	m.sql.Grow(len(m.sql1) + len(m.table) + len(m.colsStr) + len(m.sql2)*capacity + len(m.sql3) + capacity - 1)
	m.sql.WriteString(m.sql1)
	m.sql.WriteString(m.table)
	m.sql.WriteString(m.colsStr)
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

func (m *InsertBuilder) AppendBean(rowIdx int, db *DBInstance, bean interface{}) error {
	table, err := db.TableInfo(m.table)
	if err != nil {
		return err
	}
	args := make([]interface{}, len(m.colsMap))
	for _, col := range table.Columns() {
		idx, ok := m.colsMap[col.Name]
		if !ok {
			continue
		}

		fieldValuePtr, err := col.ValueOf(bean)
		if err != nil {
			return err
		}
		fieldValue := *fieldValuePtr

		arg, err := db.Value2Interface(col, fieldValue)
		if err != nil {
			return err
		}
		args[idx] = arg
	}
	m.AppendArgs(rowIdx, args...)
	return nil
}

// TakeArgs 支持数量没有达到capacity，也可以正确获取数据
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

// SetTable 重新设置表明，必须在InitArgs调用
func (m *InsertBuilder) SetTable(t string) {
	m.table = t
}

func (m *InsertBuilder) GetTable() string {
	return m.table
}
