package limits

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/uuigww/home-proxy/internal/store"
	"github.com/uuigww/home-proxy/internal/xray"
)

// ----------------------------------------------------------------------------
// Test doubles
// ----------------------------------------------------------------------------

// fakeNotifier records every Event it receives. Safe for concurrent use.
type fakeNotifier struct {
	mu     sync.Mutex
	events []Event
}

func (f *fakeNotifier) Notify(_ context.Context, ev Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, ev)
	return nil
}

func (f *fakeNotifier) Snapshot() []Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Event, len(f.events))
	copy(out, f.events)
	return out
}

func (f *fakeNotifier) HasID(id string) bool {
	for _, e := range f.Snapshot() {
		if e.ID == id {
			return true
		}
	}
	return false
}

// fakeXray implements xray.Client with scripted per-email stats and
// toggleable ping behaviour.
type fakeXray struct {
	mu          sync.Mutex
	stats       map[string]statPair
	pingErr     error
	removedV    []string
	removedS    []string
	removeCalls int
}

type statPair struct{ up, down int64 }

func newFakeXray() *fakeXray { return &fakeXray{stats: map[string]statPair{}} }

func (f *fakeXray) setStats(email string, up, down int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stats[email] = statPair{up: up, down: down}
}

func (f *fakeXray) setPingErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pingErr = err
}

func (f *fakeXray) AddVLESSUser(_ context.Context, _, _ string) error { return nil }
func (f *fakeXray) RemoveVLESSUser(_ context.Context, email string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removedV = append(f.removedV, email)
	f.removeCalls++
	return nil
}
func (f *fakeXray) AddSOCKSUser(_ context.Context, _, _, _ string) error { return nil }
func (f *fakeXray) RemoveSOCKSUser(_ context.Context, email string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removedS = append(f.removedS, email)
	f.removeCalls++
	return nil
}
func (f *fakeXray) GetUserStats(_ context.Context, email string) (int64, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.stats[email]
	if !ok {
		return 0, 0, xray.ErrUserNotFound
	}
	return s.up, s.down, nil
}
func (f *fakeXray) Reset(_ context.Context) error { return nil }
func (f *fakeXray) Ping(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.pingErr
}

// silentLogger discards all slog output in tests.
func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newStoreT opens a fresh on-disk SQLite store for the test.
func newStoreT(t *testing.T) *store.Store {
	t.Helper()
	path := t.TempDir() + "/home-proxy.db"
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// ptr is the same helper store's internal tests use, duplicated here
// because it is unexported there.
func ptr(s string) *string { return &s }

// ----------------------------------------------------------------------------
// Tests
// ----------------------------------------------------------------------------

func TestPollTraffic_EightyFiresOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	s := newStoreT(t)
	x := newFakeXray()
	n := &fakeNotifier{}
	w := New(s, x, n, Config{}, silentLogger())

	u := &store.User{
		Name:       "alice",
		VLESSUUID:  ptr("uuid"),
		Enabled:    true,
		LimitBytes: 1000,
	}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// First poll: seed prevStats with 0.
	w.pollTraffic(ctx)
	if len(n.Snapshot()) != 0 {
		t.Fatalf("no events expected before any delta, got %+v", n.Snapshot())
	}

	// Climb to 850 bytes (> 80%, < 100%).
	x.setStats("alice", 400, 450)
	w.pollTraffic(ctx)
	if !n.HasID("user.quota.80") {
		t.Fatalf("expected user.quota.80, got %+v", n.Snapshot())
	}

	// Climb further, still under 100%. Flag should suppress the second fire.
	x.setStats("alice", 450, 450) // total 900
	w.pollTraffic(ctx)
	count := 0
	for _, e := range n.Snapshot() {
		if e.ID == "user.quota.80" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 user.quota.80, got %d", count)
	}
}

func TestPollTraffic_HundredPercentDisablesAndRemoves(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	s := newStoreT(t)
	x := newFakeXray()
	n := &fakeNotifier{}
	w := New(s, x, n, Config{}, silentLogger())

	u := &store.User{
		Name:       "bob",
		VLESSUUID:  ptr("uuid-b"),
		SOCKSUser:  ptr("bob"),
		SOCKSPass:  ptr("pass"),
		Enabled:    true,
		LimitBytes: 500,
	}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Seed.
	w.pollTraffic(ctx)
	// Jump straight past 100%.
	x.setStats("bob", 400, 200) // delta 600 -> used 600 > 500
	w.pollTraffic(ctx)

	if !n.HasID("user.quota.100") {
		t.Fatalf("expected user.quota.100, got %+v", n.Snapshot())
	}

	got, err := s.GetUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if got.Enabled {
		t.Fatalf("user should be disabled after quota.100")
	}

	if len(x.removedV) != 1 || x.removedV[0] != "bob" {
		t.Fatalf("expected VLESS remove for bob, got %v", x.removedV)
	}
	if len(x.removedS) != 1 || x.removedS[0] != "bob" {
		t.Fatalf("expected SOCKS remove for bob, got %v", x.removedS)
	}
}

func TestPollHealth_XrayUnreachableAfterThreeFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	s := newStoreT(t)
	x := newFakeXray()
	n := &fakeNotifier{}
	w := New(s, x, n, Config{}, silentLogger())

	// Stub diskFree so the health loop doesn't fire disk warnings
	// and confuse our assertions.
	origDiskFree := diskFree
	diskFree = func(string) (int64, error) { return int64(100) << 30, nil }
	t.Cleanup(func() { diskFree = origDiskFree })

	x.setPingErr(errors.New("connection refused"))

	w.pollHealth(ctx)
	if n.HasID("xray.api.unreachable") {
		t.Fatal("should not fire after 1 failure")
	}
	w.pollHealth(ctx)
	if n.HasID("xray.api.unreachable") {
		t.Fatal("should not fire after 2 failures")
	}
	w.pollHealth(ctx)
	if !n.HasID("xray.api.unreachable") {
		t.Fatalf("should fire after 3 failures, got %+v", n.Snapshot())
	}

	// Recovery: a successful ping clears the counter and the dedup
	// key so a new streak can fire again.
	x.setPingErr(nil)
	w.pollHealth(ctx)
	if w.xrayPingFails != 0 {
		t.Fatalf("expected ping failure counter reset, got %d", w.xrayPingFails)
	}
}

func TestPollTraffic_CounterResetDetected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	s := newStoreT(t)
	x := newFakeXray()
	n := &fakeNotifier{}
	w := New(s, x, n, Config{}, silentLogger())

	u := &store.User{Name: "carol", VLESSUUID: ptr("uuid-c"), Enabled: true, LimitBytes: 0}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}

	x.setStats("carol", 300, 300)
	w.pollTraffic(ctx) // delta 600 recorded

	// Simulate xray stats reset: counters drop back to small values.
	x.setStats("carol", 50, 50)
	w.pollTraffic(ctx) // delta should be 50+50=100, not negative

	got, err := s.GetUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	// 600 + 100 = 700; if reset-handling were broken we'd see 600 + 0
	// (if we bailed on negative) or a garbage number.
	if got.UsedBytes != 700 {
		t.Fatalf("expected used_bytes=700 after reset handling, got %d", got.UsedBytes)
	}
}

func TestDedup_UsedForHealthSuppressesRepeats(t *testing.T) {
	// Verify the watcher's own dedup is wired: two back-to-back
	// triple-failure cycles only produce one xray.api.unreachable
	// event inside the min interval (5m).
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	s := newStoreT(t)
	x := newFakeXray()
	n := &fakeNotifier{}
	w := New(s, x, n, Config{}, silentLogger())

	orig := diskFree
	diskFree = func(string) (int64, error) { return int64(100) << 30, nil }
	t.Cleanup(func() { diskFree = orig })

	x.setPingErr(errors.New("down"))
	for i := 0; i < 6; i++ {
		w.pollHealth(ctx)
	}

	var hits int
	for _, e := range n.Snapshot() {
		if e.ID == "xray.api.unreachable" {
			hits++
		}
	}
	if hits != 1 {
		t.Fatalf("expected 1 xray.api.unreachable within dedup window, got %d (%+v)", hits, n.Snapshot())
	}
}
