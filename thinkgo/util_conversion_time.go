package thinkgo

import (
	"bytes"
	"time"
)

// ConversionTime json/xorm将time.Time和string之间转换
type ConversionTime struct {
	time.Time
}

func (c *ConversionTime) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, ConversionJsonNull) {
		return nil
	}
	if b[0] == '"' {
		if len(b) < 2 {
			return nil
		}
		b = b[1 : len(b)-1]
	}
	return c.FromDB(b)
}

func (c ConversionTime) MarshalJSON() ([]byte, error) {
	s := c.Time.Format(TimeFormatYmdHis)
	return []byte("\"" + s + "\""), nil
}

func (c *ConversionTime) FromDB(b []byte) error {
	if t, err := time.Parse(TimeFormatYmdHis, string(b)); err != nil {
		return err
	} else {
		c.Time = t
		return nil
	}
}

func (c ConversionTime) ToDB() ([]byte, error) {
	s := c.Time.Format(TimeFormatYmdHis)
	return []byte(s), nil
}
