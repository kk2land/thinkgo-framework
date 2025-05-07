package thinkgo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// ConversionUInt32s json/xorm的uint32数组，格式：[0, 1, ...] or "0,1,..."
type ConversionUInt32s []uint32

func (c *ConversionUInt32s) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, ConversionJsonNull) {
		*c = nil
		return nil
	}
	switch b[0] {
	case '"':
		return c.FromDB(b[1 : len(b)-1])
	case '[':
		var arr []interface{}
		if err := json.Unmarshal(b, &arr); err != nil {
			return err
		}
		vals := make([]uint32, 0, len(arr))
		for i, v := range arr {
			if v1, ok := JInt(v); ok {
				if v1 < 0 {
					return fmt.Errorf("第%d个是负数-%v,%s", i, v, string(b))
				}
				vals = append(vals, uint32(v1))
			} else {
				return fmt.Errorf("第%d个无法转int-%v,%s", i, v, string(b))
			}
		}
		*c = vals
		return nil
	}
	return fmt.Errorf("json解析非字符串/数组-%s", string(b))
}

func (c ConversionUInt32s) MarshalJSON() ([]byte, error) {
	buf := &bytes.Buffer{}
	buf.WriteByte('"')
	for i, v := range c {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(strconv.FormatInt(int64(v), 10))
	}
	buf.WriteByte('"')
	return buf.Bytes(), nil
}

func (c *ConversionUInt32s) FromDB(b []byte) error {
	arr := bytes.Split(b, []byte{','})
	vals := make([]uint32, 0, len(arr))
	for i, v := range arr {
		if len(v) == 0 {
			continue
		}
		if v1, err := strconv.Atoi(string(v)); err != nil {
			return fmt.Errorf("第%d个无法转int-%s,%s", i, string(v), string(b))
		} else {
			if v1 < 0 {
				return fmt.Errorf("第%d个是负数-%s,%s", i, string(v), string(b))
			}
			vals = append(vals, uint32(v1))
		}
	}
	*c = vals
	return nil
}

func (c ConversionUInt32s) ToDB() ([]byte, error) {
	buf := &bytes.Buffer{}
	for i, v := range c {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(strconv.FormatInt(int64(v), 10))
	}
	return buf.Bytes(), nil
}
