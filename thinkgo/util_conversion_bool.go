package thinkgo

import (
	"bytes"
	"fmt"
)

// ConversionBool 将string(true/false)转成bool的序列化/反序列化，支持json/toml
type ConversionBool bool

func ParseConversionBool(s string) ConversionBool {
	return s == "true"
}

func ToConversionBool(b bool) ConversionBool {
	return ConversionBool(b)
}

func (m ConversionBool) Bool() bool {
	return bool(m)
}

func (m ConversionBool) String() string {
	return fmt.Sprintf("%t", m)
}

func (m *ConversionBool) UnmarshalText(text []byte) error {
	*m = ParseConversionBool(string(text))
	return nil
}

func (m *ConversionBool) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, ConversionJsonNull) {
		return nil
	}
	if b[0] == '"' || b[0] == '\'' {
		*m = ParseConversionBool(string(b[1 : len(b)-1]))
	} else {
		*m = ParseConversionBool(string(b))
	}
	return nil
}

func (m ConversionBool) MarshalJSON() ([]byte, error) {
	return []byte("\"" + m.String() + "\""), nil
}
