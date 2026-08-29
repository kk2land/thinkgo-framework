package thinkgo

import (
	"fmt"
	"strings"
	"time"

	"xorm.io/xorm"
)

// InsertOnDuplicateBuilder 构建mysql的insert on duplicate key update的批量操作
type InsertOnDuplicateBuilder struct {
	cols     int
	sql2     string // "(?,?,...)"
	sql3     string // " on duplicate key update `col`=values(`col`)" or " on conflict(...) do update set `col`=excluded.`col`"
	capacity int
	sql      *strings.Builder
	args     []interface{}
	argsIdx  int
	table    string
	colsMap  map[string]int
	colsStr  string
}

// NewInsertOnDuplicateBuilder keyNum前几列是key
func NewInsertOnDuplicateBuilder(
	driver string,
	table string,
	cols []string,
	keyNum int,
) *InsertOnDuplicateBuilder {
	return NewInsertOnDuplicateBuilder1(driver, table, cols, keyNum, nil)
}

// NewInsertOnDuplicateBuilder1 keyNum前几列是key
func NewInsertOnDuplicateBuilder1(
	driver string,
	table string,
	cols []string,
	keyNum int,
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

	//var sql1 = "insert into " + table + "(" + colsStr + ") values"
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
	var colsMap = make(map[string]int, len(cols))
	if driver == "postgres" {
		//pgsql
		tmp1 := make([]string, 0, keyNum)
		tmp2 := make([]string, 0, len(cols)-keyNum)
		for i, col := range cols {
			colsMap[col] = i
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
			colsMap[col] = i
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
		cols:    len(cols),
		sql2:    sql2,
		sql3:    sql3,
		table:   table,
		colsMap: colsMap,
		colsStr: "(" + colsStr + ") values",
	}
}

func (m *InsertOnDuplicateBuilder) InitArgs(capacity int) {
	m.capacity = capacity
	//构建sql
	m.sql = &strings.Builder{}
	// len("insert into xxx(...) values") + capacity * len("(?,?,...)") + (capacity - 1) * len(",") + len(" on duplicate key update ...")
	//sql1 = "insert into " + table + "(" + colsStr + ") values"
	m.sql.Grow(12 + len(m.table) + len(m.colsStr) + len(m.sql2)*capacity + capacity - 1 + len(m.sql3))
	m.sql.WriteString("insert into ")
	m.sql.WriteString(m.table)
	m.sql.WriteString(m.colsStr)
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

func (m *InsertOnDuplicateBuilder) AppendBean(rowIdx int, db xorm.EngineInterface, bean interface{}) error {
	table, err := db.TableInfo(bean)
	if err != nil {
		return fmt.Errorf("TableInfo err:%v", err)
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

		arg, err := Value2Interface(db, col, fieldValue)
		if err != nil {
			return err
		}
		args[idx] = arg
	}
	m.AppendArgs(rowIdx, args...)
	return nil
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

// SetTable 重新设置表明，必须在InitArgs调用
func (m *InsertOnDuplicateBuilder) SetTable(t string) {
	m.table = t
}

func (m *InsertOnDuplicateBuilder) GetTable() string {
	return m.table
}

func (m *InsertOnDuplicateBuilder) Exec(db *DBInstance) error {
	args := m.TakeArgs()
	return db.ExecWithBackoff(
		BackoffPolicyDefault(300*time.Millisecond, 10),
		func(instance *DBInstance) error {
			_, err := instance.Exec(args...)
			return err
		},
	)
}
