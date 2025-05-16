package thinkgo

import (
	"bytes"
	"strconv"
)

// ConversionInt64 将string转成int64的序列化/反序列化，支持json/toml
type ConversionInt64 int64

func ParseConversionInt64(s string) (ConversionInt64, error) {
	i, err := strconv.ParseInt(s, 10, 64)
	return ConversionInt64(i), err
}

func ToConversionInt64(i int64) ConversionInt64 {
	return ConversionInt64(i)
}

func (m ConversionInt64) Int64() int64 {
	return int64(m)
}

func (m ConversionInt64) String() string {
	return strconv.FormatInt(m.Int64(), 10)
}

func (m *ConversionInt64) UnmarshalText(text []byte) error {
	i, err := ParseConversionInt64(string(text))
	if err != nil {
		return err
	}
	*m = i
	return nil
}

func (m *ConversionInt64) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, ConversionJsonNull) {
		return nil
	}
	if b[0] == '"' || b[0] == '\'' {
		if i, err := ParseConversionInt64(string(b[1 : len(b)-1])); err != nil {
			return err
		} else {
			*m = i
			return nil
		}
	} else {
		if i, err := ParseConversionInt64(string(b)); err != nil {
			return err
		} else {
			*m = i
			return nil
		}
	}
}

func (m ConversionInt64) MarshalJSON() ([]byte, error) {
	return []byte("\"" + m.String() + "\""), nil
}
