package thinkgo

import (
	"bytes"
	"fmt"
)

// ConvBool 将string(true/false)转成bool的序列化/反序列化，支持json/toml
type ConvBool bool

func ParseConvBool(s string) ConvBool {
	return s == "true"
}

func ToConvBool(b bool) ConvBool {
	return ConvBool(b)
}

func (m ConvBool) Bool() bool {
	return bool(m)
}

func (m ConvBool) String() string {
	return fmt.Sprintf("%t", m)
}

func (m *ConvBool) UnmarshalText(text []byte) error {
	*m = ParseConvBool(string(text))
	return nil
}

func (m *ConvBool) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, ConversionJsonNull) {
		return nil
	}
	b = JsonBytesTrimQuote(b)
	*m = ParseConvBool(string(b))
	return nil
}

func (m ConvBool) MarshalJSON() ([]byte, error) {
	return []byte("\"" + m.String() + "\""), nil
}
