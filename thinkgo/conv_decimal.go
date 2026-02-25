package thinkgo

import (
	"fmt"
	"github.com/shopspring/decimal"
)

var Decimal0 = decimal.NewFromInt(0)

// ConvDecimal 序列化/反序列化big.Float，支持json/toml/xorm；
// 反序列化说明：
//
//	*ConvDecimal:
//		null -> nil
//		"" -> new(decimal.Decimal)
//		不存在 -> nil
//	ConvDecimal:
//		null -> new(decimal.Decimal)
//		"" -> new(decimal.Decimal)
//		不存在 -> new(decimal.Decimal)
type ConvDecimal struct {
	decimal.Decimal
}

func NewConvDecimal() *ConvDecimal {
	return &ConvDecimal{decimal.NewFromFloat(0)}
}

func ToConvDecimal1(d decimal.Decimal) *ConvDecimal {
	return &ConvDecimal{d}
}

// ToConvDecimal2 对d会进行复制操作
func ToConvDecimal2(d *decimal.Decimal) *ConvDecimal {
	if d == nil {
		return nil
	}
	return &ConvDecimal{d.Copy()}
}

func ParseConvDecimal(s string, d *ConvDecimal) error {
	if len(s) > 0 {
		if d1, err := decimal.NewFromString(s); err != nil {
			return fmt.Errorf("float invalid-%s", s)
		} else {
			d.Decimal = d1
			return nil
		}
	} else {
		d.Decimal = Decimal0
		return nil
	}
}

func (m *ConvDecimal) Set(d decimal.Decimal) {
	m.Decimal = d
}

func (m *ConvDecimal) Float64() float64 {
	f, _ := m.Decimal.Float64()
	return f
}

func (d *ConvDecimal) UnmarshalJSON(b []byte) error {
	b1 := JsonBytesTrimQuote(b)
	if len(b1) == 0 {
		d.Decimal = Decimal0
		return nil
	}
	return d.Decimal.UnmarshalJSON(b1)
}

func (m *ConvDecimal) UnmarshalText(text []byte) error {
	if err := ParseConvDecimal(string(text), m); err != nil {
		return err
	}
	return nil
}

func (m *ConvDecimal) FromDB(b []byte) error {
	return ParseConvDecimal(string(b), m)
}

func (m *ConvDecimal) ToDB() ([]byte, error) {
	if m != nil {
		return []byte(m.String()), nil
	} else {
		return nil, nil
	}
}
