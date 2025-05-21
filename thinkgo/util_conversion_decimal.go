package thinkgo

import (
	"bytes"
	"fmt"
	"math/big"
)

// ConversionDecimal 序列化/反序列化big.Float，支持json/toml/xorm；
// 反序列化说明：
//
//	*ConversionDecimal:
//		null -> nil
//		"" -> new(big.Float)
//		不存在 -> nil
//	ConversionDecimal:
//		null -> new(big.Float)
//		"" -> new(big.Float)
//		不存在 -> new(big.Float)
type ConversionDecimal big.Float

func ParseConversionDecimal(s string, d *ConversionDecimal) error {
	if len(s) > 0 {
		if _, ok := d.Float().SetString(s); !ok {
			return fmt.Errorf("ConversionDecimal invalid-%s", s)
		}
	}
	return nil
}

func ToConversionDecimal(f *big.Float) *ConversionDecimal {
	return (*ConversionDecimal)(f)
}

func (m *ConversionDecimal) Float() *big.Float {
	return (*big.Float)(m)
}

func (m *ConversionDecimal) String() string {
	return m.Float().Text('f', -1)
}

func (m *ConversionDecimal) UnmarshalText(text []byte) error {
	if err := ParseConversionDecimal(string(text), m); err != nil {
		return err
	}
	return nil
}

func (m *ConversionDecimal) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, ConversionJsonNull) {
		return nil
	}
	return m.FromDB(bytesTrimQuote(b))
}

func (m ConversionDecimal) MarshalJSON() ([]byte, error) {
	buf := &bytes.Buffer{}
	buf.WriteByte('"')
	buf.WriteString(m.String())
	buf.WriteByte('"')
	return buf.Bytes(), nil
}

func (m *ConversionDecimal) FromDB(b []byte) error {
	if err := ParseConversionDecimal(string(b), m); err != nil {
		return err
	}
	return nil
}

func (m *ConversionDecimal) ToDB() ([]byte, error) {
	if m != nil {
		return []byte(m.String()), nil
	} else {
		return []byte{}, nil
	}
}
