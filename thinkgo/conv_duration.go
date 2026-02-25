package thinkgo

import (
	"bytes"
	"regexp"
	"time"
)

// ConvDuration 序列化/反序列化time.Duration，支持json/toml/xorm；null->false;
//
//	反序列化支持的格式 = "10(默认秒)、10ms(毫秒)、10s(秒)、3m(分)、3h(时)";
type ConvDuration time.Duration

func ParseConvDuration(s string) (ConvDuration, error) {
	if ok, _ := regexp.MatchString("^[0-9]+$", s); ok {
		s = s + "s"
	}
	if d, err := time.ParseDuration(s); err != nil {
		return 0, err
	} else {
		return ConvDuration(d), nil
	}
}

func ToConversionDuration(t time.Duration) ConvDuration {
	return ConvDuration(t)
}

func (m ConvDuration) Duration() time.Duration {
	return time.Duration(m)
}

func (m ConvDuration) String() string {
	return m.Duration().String()
}

func (m *ConvDuration) UnmarshalText(text []byte) error {
	s := string(text)
	if d, err := ParseConvDuration(s); err != nil {
		return err
	} else {
		*m = d
		return nil
	}
}

func (m *ConvDuration) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, ConversionJsonNull) {
		return nil
	}
	return m.FromDB(JsonBytesTrimQuote(b))
}

func (m ConvDuration) MarshalJSON() ([]byte, error) {
	buf := &bytes.Buffer{}
	buf.WriteByte('"')
	buf.WriteString(m.String())
	buf.WriteByte('"')
	return buf.Bytes(), nil
}

func (m *ConvDuration) FromDB(b []byte) error {
	if d, err := ParseConvDuration(string(b)); err != nil {
		return err
	} else {
		*m = d
		return nil
	}
}

func (m *ConvDuration) ToDB() ([]byte, error) {
	if m != nil {
		return []byte(m.String()), nil
	} else {
		return []byte{}, nil
	}
}
