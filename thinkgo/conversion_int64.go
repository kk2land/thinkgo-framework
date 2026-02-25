package thinkgo

import (
	"bytes"
	"strconv"
)

// ConvInt64 将string转成int64的序列化/反序列化，支持json/toml; null->0; ""->0;
type ConvInt64 int64

func ParseConvInt64(s string) (ConvInt64, error) {
	if len(s) > 0 {
		i, err := strconv.ParseInt(s, 10, 64)
		return ConvInt64(i), err
	}
	return 0, nil
}

func ToConvInt64(i int64) ConvInt64 {
	return ConvInt64(i)
}

func (m ConvInt64) Int64() int64 {
	return int64(m)
}

func (m ConvInt64) String() string {
	return strconv.FormatInt(m.Int64(), 10)
}

func (m *ConvInt64) UnmarshalText(text []byte) error {
	i, err := ParseConvInt64(string(text))
	if err != nil {
		return err
	}
	*m = i
	return nil
}

func (m *ConvInt64) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, ConversionJsonNull) {
		return nil
	}
	b = JsonBytesTrimQuote(b)
	if i, err := ParseConvInt64(string(b)); err != nil {
		return err
	} else {
		*m = i
		return nil
	}
}

func (m ConvInt64) MarshalJSON() ([]byte, error) {
	return []byte("\"" + m.String() + "\""), nil
}
