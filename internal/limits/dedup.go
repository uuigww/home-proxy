package limits

import (
	"sync"
	"time"
)

// Dedup is an in-memory coalescing helper: it remembers the last time
// every string key "fired" and answers ShouldFire only when the caller
// has waited at least minInterval since the previous true answer for
// that key.
//
// It is safe for concurrent use.
type Dedup struct {
	mu   sync.Mutex
	last map[string]time.Time
	now  func() time.Time
}

// NewDedup returns a ready-to-use Dedup.
func NewDedup() *Dedup {
	return &Dedup{last: make(map[string]time.Time), now: time.Now}
}

// newDedupWithClock lets tests inject a clock. Kept unexported so the
// public API remains a single constructor.
func newDedupWithClock(now func() time.Time) *Dedup {
	return &Dedup{last: make(map[string]time.Time), now: now}
}

// ShouldFire reports whether key has not been fired within the last
// minInterval. When it returns true, it also records "now" as the most
// recent fire time for key, so the next call within the window returns
// false.
//
// minInterval <= 0 means "always fire and always update the
// timestamp".
func (d *Dedup) ShouldFire(key string, minInterval time.Duration) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := d.now()
	if minInterval > 0 {
		if prev, ok := d.last[key]; ok && now.Sub(prev) < minInterval {
			return false
		}
	}
	d.last[key] = now
	return true
}

// Reset forgets the last-fire time for key, so the next ShouldFire
// call returns true regardless of how recently key fired.
func (d *Dedup) Reset(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.last, key)
}
