package thinkgo

import (
	"time"
)

// BackoffPolicy 重试的帮助类
//
//	for backoff.Next() {
//		err := ...
//		if err != nil && backoff.End() {
//			break
//		}
//		time.Sleep(backoff.Get())
//	}
type BackoffPolicy interface {
	Next() bool //是否还有next
	End() bool  //当前是否处于end
	Get() time.Duration
}

type backoffPolicyDefault struct {
	delay time.Duration
	total int
	index int
}

func BackoffPolicyDefault(delay time.Duration, total int) BackoffPolicy {
	return &backoffPolicyDefault{
		delay: delay,
		total: total,
		index: 0,
	}
}

func (b *backoffPolicyDefault) Next() bool {
	if b.index < b.total {
		b.index++
		return true
	}
	return false
}

func (b *backoffPolicyDefault) End() bool {
	return b.index == b.total
}

func (b *backoffPolicyDefault) Get() time.Duration {
	return b.delay
}

type backoffPolicyForever struct {
	delay time.Duration
}

func BackoffPolicyForever(delay time.Duration) BackoffPolicy {
	return &backoffPolicyForever{
		delay: delay,
	}
}

func (b *backoffPolicyForever) Next() bool {
	return true
}

func (b *backoffPolicyForever) End() bool {
	return false
}

func (b *backoffPolicyForever) Get() time.Duration {
	return b.delay
}
