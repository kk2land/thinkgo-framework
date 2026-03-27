package thinkgo

import (
	"fmt"
	"strings"
)

// InsertOnDuplicateBuilder 构建mysql的insert on duplicate key update的批量操作
type InsertOnDuplicateBuilder struct {
	cols     int
	sql1     string // "insert into xxx(...) values"
	sql2     string // "(?,?,...)"
	sql3     string // " on duplicate key update `col`=values(`col`)" or " on conflict(...) do update set `col`=excluded.`col`"
	capacity int
	sql      *strings.Builder
	args     []interface{}
	argsIdx  int
	Table    string
}

func NewInsertOnDuplicateBuilder(
	driver string,
	table string,
	cols []string,
	keyNum int, //前几列是key
) *InsertOnDuplicateBuilder {
	return NewInsertOnDuplicateBuilder1(driver, table, cols, keyNum, nil)
}

func NewInsertOnDuplicateBuilder1(
	driver string,
	table string,
	cols []string,
	keyNum int,                //前几列是key
	colExps map[string]string, //upsert的额外表达式，"col"=excluded."col"，如果是""空字符串，那就跳过该字段的update
) *InsertOnDuplicateBuilder {
	//第一部分
	var colsStr string
	switch driver {
	case "postgres":
		colsStr = "\"" + strings.Join(cols, "\",\"") + "\""
	default:
		colsStr = "`" + strings.Join(cols, "`,`") + "`"
	}

	sql1 := "insert into " + table + "(" + colsStr + ") values"
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
	if driver == "postgres" {
		//pgsql
		tmp1 := make([]string, 0, keyNum)
		tmp2 := make([]string, 0, len(cols)-keyNum)
		for i, col := range cols {
			if i < keyNum {
				tmp1 = append(tmp1, "\""+col+"\"")
			} else {
				if exp, ok := colExps[col]; ok {
					if exp != "" {
						tmp2 = append(tmp2, exp)
					}
				} else {
					tmp2 = append(tmp2, fmt.Sprintf("\"%s\"=excluded.\"%s\"", col, col))
				}
			}
		}
		sql3 = " on conflict(" + strings.Join(tmp1, ",") + ") do update set " + strings.Join(tmp2, ",")
	} else {
		//mysql
		tmp := make([]string, 0, len(cols)-keyNum)
		for i, col := range cols {
			if i < keyNum {
				continue
			}
			if exp, ok := colExps[col]; ok {
				if exp != "" {
					tmp = append(tmp, exp)
				}
			} else {
				tmp = append(tmp, "`"+col+"`=values(`"+col+"`)")
			}
		}
		sql3 = " on duplicate key update " + strings.Join(tmp, ",")
	}
	return &InsertOnDuplicateBuilder{
		cols:  len(cols),
		sql1:  sql1,
		sql2:  sql2,
		sql3:  sql3,
		Table: table,
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

func (m *InsertOnDuplicateBuilder) AppendArgs(rowIdx int, args ...interface{}) {
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

// TakeArgs 支持数量没有达到capacity，也可以正确获取数据
func (m *InsertOnDuplicateBuilder) TakeArgs() (ret []interface{}) {
	//第一个参数是sql
	m.sql.WriteString(m.sql3)
	m.args[0] = m.sql.String()
	m.sql = nil
	if m.argsIdx == len(m.args) {
		//表示args的空间都已经铺满
		ret = m.args
	} else {
		ret = m.args[0:m.argsIdx]
	}
	m.args = nil
	return ret
}

func (m *InsertOnDuplicateBuilder) GetTable() string {
	return m.Table
}
