package thinkgo

import (
	"bytes"
	"fmt"
	"math/big"
)

// ConversionDecimal 序列化/反序列化big.Float，支持json/toml/xorm；null->nil; ""->new(big.Float)
type ConversionDecimal big.Float

func ParseConversionDecimal(s string) (*ConversionDecimal, error) {
	f := new(big.Float)
	if len(s) > 0 {
		if _, ok := f.SetString(s); !ok {
			return nil, fmt.Errorf("ConversionDecimal invalid-%s", s)
		}
	}
	return (*ConversionDecimal)(f), nil
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
	f, err := ParseConversionDecimal(string(text))
	if err != nil {
		return err
	}
	m = f
	return nil
}

func (m *ConversionDecimal) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, ConversionJsonNull) {
		return nil
	}
	b = bytesTrimQuote(b)
	return m.FromDB(b)
}

func (m ConversionDecimal) MarshalJSON() ([]byte, error) {
	buf := &bytes.Buffer{}
	buf.WriteByte('"')
	buf.WriteString(m.String())
	buf.WriteByte('"')
	return buf.Bytes(), nil
}

func (m *ConversionDecimal) FromDB(b []byte) error {
	f, err := ParseConversionDecimal(string(b))
	if err != nil {
		return err
	}
	m = f
	return nil
}

func (m *ConversionDecimal) ToDB() ([]byte, error) {
	if m != nil {
		return []byte(m.String()), nil
	} else {
		return []byte{}, nil
	}
}
