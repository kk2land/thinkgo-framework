package thinkgo

import "time"

// MinutesTicker，每分钟的tick

type MinutesTicker struct {
	*time.Ticker
	minutes int
	first   bool
}

func NewMinutesTicker(minutes int) *MinutesTicker {
	return &MinutesTicker{
		Ticker:  time.NewTicker(minutesTickerDuration(time.Now(), minutes)),
		minutes: minutes,
		first:   true,
	}
}

func (c *MinutesTicker) Correct() {
	c.CorrectByTime(time.Now())
}

func (c *MinutesTicker) CorrectByTime(t time.Time) {
	if c.first {
		c.Reset(time.Duration(c.minutes) * time.Minute)
		c.first = false
	} else {
		sec := t.Second()
		if sec > 3 {
			c.Reset(minutesTickerDuration(t, c.minutes))
		}
	}
}

func minutesTickerDuration(now time.Time, minutes int) time.Duration {
	d := (minutes*60 - 60*(now.Minute()%minutes)) - now.Second()
	return time.Second * time.Duration(d)
}
