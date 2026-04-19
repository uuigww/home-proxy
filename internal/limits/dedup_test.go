package limits

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDedup_FirstFireReturnsTrue(t *testing.T) {
	d := NewDedup()
	if !d.ShouldFire("k", time.Minute) {
		t.Fatal("first call should fire")
	}
}

func TestDedup_SuppressesWithinInterval(t *testing.T) {
	now := time.Unix(0, 0)
	d := newDedupWithClock(func() time.Time { return now })
	if !d.ShouldFire("k", time.Minute) {
		t.Fatal("first call should fire")
	}
	// Advance less than the interval.
	now = now.Add(30 * time.Second)
	if d.ShouldFire("k", time.Minute) {
		t.Fatal("second call within interval should be suppressed")
	}
}

func TestDedup_FiresAfterInterval(t *testing.T) {
	now := time.Unix(0, 0)
	d := newDedupWithClock(func() time.Time { return now })
	if !d.ShouldFire("k", time.Minute) {
		t.Fatal("first call should fire")
	}
	now = now.Add(2 * time.Minute)
	if !d.ShouldFire("k", time.Minute) {
		t.Fatal("call after interval should fire again")
	}
}

func TestDedup_ZeroIntervalAlwaysFires(t *testing.T) {
	d := NewDedup()
	for i := 0; i < 10; i++ {
		if !d.ShouldFire("k", 0) {
			t.Fatalf("iter %d should fire with zero interval", i)
		}
	}
}

func TestDedup_ResetClearsKey(t *testing.T) {
	now := time.Unix(0, 0)
	d := newDedupWithClock(func() time.Time { return now })
	_ = d.ShouldFire("k", time.Minute)
	d.Reset("k")
	if !d.ShouldFire("k", time.Minute) {
		t.Fatal("after Reset the key should fire again immediately")
	}
}

func TestDedup_ConcurrentOnlyOneWins(t *testing.T) {
	d := NewDedup()
	const goroutines = 32
	var wg sync.WaitGroup
	var wins int64
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if d.ShouldFire("race", time.Hour) {
				atomic.AddInt64(&wins, 1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if wins != 1 {
		t.Fatalf("expected exactly one goroutine to win, got %d", wins)
	}
}
