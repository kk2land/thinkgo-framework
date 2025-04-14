package thinkgo

import (
	"sync/atomic"
	"time"
)

// TimeoutOnce，带有超时的once

type TimeoutOnce struct {
	done  chan Void
	state int32
}

func TimeoutOnceNew() *TimeoutOnce {
	return &TimeoutOnce{
		done:  make(chan Void),
		state: 0,
	}
}

func (once *TimeoutOnce) Do(f func()) {
	if atomic.CompareAndSwapInt32(&once.state, 0, 1) {
		once.doSlow(f)
	}
	<-once.done
}

func (once *TimeoutOnce) Do1(timeout time.Duration, f func()) bool {
	if atomic.CompareAndSwapInt32(&once.state, 0, 1) {
		once.doSlow(f)
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-once.done:
		break
	case <-timer.C:
		return false
	}
	return true
}

func (once *TimeoutOnce) doSlow(f func()) {
	defer close(once.done)
	f()
}
