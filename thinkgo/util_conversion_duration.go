package thinkgo

import (
	"bytes"
	"regexp"
	"time"
)

// ConversionDuration 序列化/反序列化time.Duration，支持json/toml/xorm；null->false;
//
//	反序列化支持的格式 = "10(默认秒)、10ms(毫秒)、10s(秒)、3m(分)、3h(时)";
type ConversionDuration time.Duration

func ParseConversionDuration(s string) (ConversionDuration, error) {
	if ok, _ := regexp.MatchString("^[0-9]+$", s); ok {
		s = s + "s"
	}
	if d, err := time.ParseDuration(s); err != nil {
		return 0, err
	} else {
		return ConversionDuration(d), nil
	}
}

func ToConversionDuration(t time.Duration) ConversionDuration {
	return ConversionDuration(t)
}

func (m ConversionDuration) Duration() time.Duration {
	return time.Duration(m)
}

func (m ConversionDuration) String() string {
	return m.Duration().String()
}

func (m *ConversionDuration) UnmarshalText(text []byte) error {
	s := string(text)
	if d, err := ParseConversionDuration(s); err != nil {
		return err
	} else {
		*m = d
		return nil
	}
}

func (m *ConversionDuration) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, ConversionJsonNull) {
		return nil
	}
	b = bytesTrimQuote(b)
	return m.FromDB(b)
}

func (m ConversionDuration) MarshalJSON() ([]byte, error) {
	buf := &bytes.Buffer{}
	buf.WriteByte('"')
	buf.WriteString(m.String())
	buf.WriteByte('"')
	return buf.Bytes(), nil
}

func (m *ConversionDuration) FromDB(b []byte) error {
	if d, err := ParseConversionDuration(string(b)); err != nil {
		return err
	} else {
		*m = d
		return nil
	}
}

func (m *ConversionDuration) ToDB() ([]byte, error) {
	if m != nil {
		return []byte(m.String()), nil
	} else {
		return []byte{}, nil
	}
}
