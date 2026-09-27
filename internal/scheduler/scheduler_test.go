package scheduler

import (
	"context"
	"errors"
	"io"
	"log"
	"sync/atomic"
	"testing"
	"time"
)

func TestImmediateTicksSerialAndCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ticks := make(chan time.Time, 3)
	var calls, active, max atomic.Int64
	done := make(chan struct{})
	go func() {
		RunTicks(ctx, ticks, func(context.Context) error {
			n := active.Add(1)
			if n > max.Load() {
				max.Store(n)
			}
			calls.Add(1)
			time.Sleep(5 * time.Millisecond)
			active.Add(-1)
			if calls.Load() == 1 {
				return errors.New("once")
			}
			return nil
		}, log.New(io.Discard, "", 0))
		close(done)
	}()
	deadline := time.After(time.Second)
	for calls.Load() < 1 {
		select {
		case <-deadline:
			t.Fatal("no immediate run")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	ticks <- time.Now()
	ticks <- time.Now()
	for calls.Load() < 3 {
		select {
		case <-deadline:
			t.Fatal("ticks not run")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()
	ticks <- time.Now()
	<-done
	if max.Load() != 1 {
		t.Fatal("overlap")
	}
	before := calls.Load()
	time.Sleep(10 * time.Millisecond)
	if calls.Load() != before {
		t.Fatal("run after cancel")
	}
}
