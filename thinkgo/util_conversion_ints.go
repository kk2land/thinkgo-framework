package thinkgo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// ConversionInts 格式：[int1, int2, ...] or "int1,int2,int3"的序列化/反序列化，支持json/xorm
type ConversionInts []int

func (c *ConversionInts) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, ConversionJsonNull) {
		*c = nil
		return nil
	}
	switch b[0] {
	case '"':
		return c.FromDB(bytes.Trim(b, "\""))
	case '\'':
		return c.FromDB(bytes.Trim(b, "'"))
	case '[':
		var arr []interface{}
		if err := json.Unmarshal(b, &arr); err != nil {
			return err
		}
		vals := make([]int, 0, len(arr))
		for i, v := range arr {
			if v1, ok := JInt(v); ok {
				vals = append(vals, int(v1))
			} else {
				return fmt.Errorf("第%d个无法转int-%v,%s", i, v, string(b))
			}
		}
		*c = vals
		return nil
	}
	return fmt.Errorf("json解析非字符串/数组-%s", string(b))
}

func (c ConversionInts) MarshalJSON() ([]byte, error) {
	buf := &bytes.Buffer{}
	buf.WriteByte('"')
	for i, v := range c {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(strconv.Itoa(v))
	}
	buf.WriteByte('"')
	return buf.Bytes(), nil
}

func (c *ConversionInts) FromDB(b []byte) error {
	arr := bytes.Split(b, []byte{','})
	vals := make([]int, 0, len(arr))
	for i, v := range arr {
		if len(v) == 0 {
			continue
		}
		if v1, err := strconv.Atoi(string(v)); err != nil {
			return fmt.Errorf("第%d个无法转int-%s,%s", i, string(v), string(b))
		} else {
			vals = append(vals, v1)
		}
	}
	*c = vals
	return nil
}

func (c ConversionInts) ToDB() ([]byte, error) {
	buf := &bytes.Buffer{}
	for i, v := range c {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(strconv.Itoa(v))
	}
	return buf.Bytes(), nil
}
