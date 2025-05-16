package thinkgo

import (
	"bytes"
	"fmt"
	"math/big"
)

// ConversionDecimal 序列化/反序列化big.Float，支持json/toml/xorm
type ConversionDecimal big.Float

func ParseConversionDecimal(s string) (*ConversionDecimal, error) {
	f := new(big.Float)
	if _, ok := f.SetString(s); !ok {
		return nil, fmt.Errorf("ConversionDecimal invalid-%s", s)
	} else {
		return (*ConversionDecimal)(f), nil
	}
}

func ToConversionDecimal(f *big.Float) *ConversionDecimal {
	return (*ConversionDecimal)(f)
}

func (m *ConversionDecimal) Float() *big.Float {
	return (*big.Float)(m)
}

func (m *ConversionDecimal) String() string {
	return m.Float().Text('f', 10)
}

func (m *ConversionDecimal) UnmarshalText(text []byte) error {
	s := string(text)
	_, ok := m.Float().SetString(s)
	if !ok {
		return fmt.Errorf("无法解析ConversionDecimal-%s", s)
	} else {
		return nil
	}
}

func (m *ConversionDecimal) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, ConversionJsonNull) {
		return nil
	}
	if b[0] == '"' || b[0] == '\'' {
		return m.FromDB(b[1 : len(b)-1])
	}
	return fmt.Errorf("json解析非字符串-%s", string(b))
}

func (m ConversionDecimal) MarshalJSON() ([]byte, error) {
	buf := &bytes.Buffer{}
	buf.WriteByte('"')
	buf.WriteString(m.String())
	buf.WriteByte('"')
	return buf.Bytes(), nil
}

func (m *ConversionDecimal) FromDB(b []byte) error {
	f := (*big.Float)(m)
	if _, ok := f.SetString(string(b)); !ok {
		return fmt.Errorf("ConversionDecimal invalid-%s", b)
	} else {
		return nil
	}
}

func (m *ConversionDecimal) ToDB() ([]byte, error) {
	if m != nil {
		return []byte(m.String()), nil
	} else {
		return []byte{}, nil
	}
}
