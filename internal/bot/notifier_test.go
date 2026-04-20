package bot

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/uuigww/home-proxy/internal/i18n"
	"github.com/uuigww/home-proxy/internal/store"
)

// fakeSender captures every message so tests can assert on them.
type fakeSender struct {
	mu       sync.Mutex
	sent     []*bot.SendMessageParams
	edited   []*bot.EditMessageTextParams
	nextID   int64
	sendErr  error
	editErr  error
	editLast int
}

func (f *fakeSender) SendMessage(_ context.Context, p *bot.SendMessageParams) (*models.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	f.sent = append(f.sent, p)
	f.editLast = int(atomic.AddInt64(&f.nextID, 1))
	return &models.Message{ID: f.editLast}, nil
}
func (f *fakeSender) EditMessageText(_ context.Context, p *bot.EditMessageTextParams) (*models.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.editErr != nil {
		return nil, f.editErr
	}
	f.edited = append(f.edited, p)
	return &models.Message{ID: p.MessageID}, nil
}
func (f *fakeSender) sentCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}
func (f *fakeSender) editedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.edited)
}

func newStoreForTest(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func newBundle(t *testing.T) *i18n.Bundle {
	t.Helper()
	b, err := i18n.New()
	if err != nil {
		t.Fatalf("i18n: %v", err)
	}
	return b
}

func TestNotifierCriticalAlwaysDelivered(t *testing.T) {
	ctx := context.Background()
	st := newStoreForTest(t)
	// Mark critical OFF — it must still deliver.
	prefs := store.NewDefaultAdminPrefs(111, "en")
	prefs.NotifyCritical = false
	prefs.NotifyImportant = false
	prefs.NotifyInfo = false
	prefs.QuietFromHour = 0
	prefs.QuietToHour = 24 // always in quiet
	if err := st.UpsertAdminPrefs(ctx, prefs); err != nil {
		t.Fatalf("seed prefs: %v", err)
	}

	fs := &fakeSender{}
	n := newNotifierForTest(fs, st, newBundle(t), []int64{111}, "en")
	err := n.Notify(ctx, Event{ID: "xray.api.unreachable", Severity: SeverityCritical, Params: []any{"127.0.0.1", "30s"}})
	if err != nil {
		t.Fatalf("notify: %v", err)
	}
	if fs.sentCount() != 1 {
		t.Fatalf("critical should bypass everything, sent=%d", fs.sentCount())
	}
}

func TestNotifierInfoFilteredByPrefs(t *testing.T) {
	ctx := context.Background()
	st := newStoreForTest(t)
	prefs := store.NewDefaultAdminPrefs(222, "en")
	prefs.NotifyInfo = false
	prefs.QuietFromHour = 0 // disable quiet window so test is time-of-day independent
	prefs.QuietToHour = 0
	if err := st.UpsertAdminPrefs(ctx, prefs); err != nil {
		t.Fatalf("seed prefs: %v", err)
	}
	fs := &fakeSender{}
	n := newNotifierForTest(fs, st, newBundle(t), []int64{222}, "en")
	if err := n.Notify(ctx, Event{ID: "user.created", Severity: SeverityInfo, Params: []any{"alex", "alice", "VLESS", "50 GB"}}); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if fs.sentCount() != 0 {
		t.Fatalf("info should be filtered out, sent=%d", fs.sentCount())
	}
}

func TestNotifierInfoOthersOnlySkipsActor(t *testing.T) {
	ctx := context.Background()
	st := newStoreForTest(t)
	prefs := store.NewDefaultAdminPrefs(333, "en")
	prefs.NotifyInfoOthersOnly = true
	prefs.QuietFromHour = 0 // disable quiet window so test is time-of-day independent
	prefs.QuietToHour = 0
	if err := st.UpsertAdminPrefs(ctx, prefs); err != nil {
		t.Fatalf("seed: %v", err)
	}
	fs := &fakeSender{}
	n := newNotifierForTest(fs, st, newBundle(t), []int64{333}, "en")
	// Actor == same admin -> should NOT fire.
	_ = n.Notify(ctx, Event{ID: "user.enabled", Severity: SeverityInfo, ActorTGID: 333, Params: []any{"admin", "alice"}})
	if fs.sentCount() != 0 {
		t.Fatalf("expected skip for self-actor, sent=%d", fs.sentCount())
	}
	// Different actor -> should fire.
	_ = n.Notify(ctx, Event{ID: "user.enabled", Severity: SeverityInfo, ActorTGID: 999, Params: []any{"other", "alice"}})
	if fs.sentCount() != 1 {
		t.Fatalf("expected delivery for other actor, sent=%d", fs.sentCount())
	}
}

func TestNotifierQuietHoursBatch(t *testing.T) {
	ctx := context.Background()
	st := newStoreForTest(t)
	// Quiet window 0..24 — always quiet.
	prefs := store.NewDefaultAdminPrefs(444, "en")
	prefs.QuietFromHour = 0
	prefs.QuietToHour = 24
	if err := st.UpsertAdminPrefs(ctx, prefs); err != nil {
		t.Fatalf("seed: %v", err)
	}
	fs := &fakeSender{}
	n := newNotifierForTest(fs, st, newBundle(t), []int64{444}, "en")
	for i := 0; i < 3; i++ {
		_ = n.Notify(ctx, Event{ID: "user.created", Severity: SeverityInfo, Params: []any{"a", "b", "VLESS", "50 GB"}})
	}
	if fs.sentCount() != 0 {
		t.Fatalf("quiet hours should suppress sends, sent=%d", fs.sentCount())
	}
	// Force the admin's window to be "not quiet" by rewriting prefs, then
	// invoke flushQuiet manually.
	prefs.QuietFromHour = 0
	prefs.QuietToHour = 0
	_ = st.UpsertAdminPrefs(ctx, prefs)
	n.flushQuiet(ctx)
	if fs.sentCount() != 1 {
		t.Fatalf("flush should deliver one batched digest, sent=%d", fs.sentCount())
	}
}

func TestNotifierCoalesceSameEvent(t *testing.T) {
	ctx := context.Background()
	st := newStoreForTest(t)
	prefs := store.NewDefaultAdminPrefs(555, "en")
	prefs.QuietFromHour = 0
	prefs.QuietToHour = 0
	if err := st.UpsertAdminPrefs(ctx, prefs); err != nil {
		t.Fatalf("seed: %v", err)
	}
	fs := &fakeSender{}
	n := newNotifierForTest(fs, st, newBundle(t), []int64{555}, "en")
	// First fire: sends a new message.
	_ = n.Notify(ctx, Event{ID: "xray.api.unreachable", Severity: SeverityImportant, Params: []any{"127.0.0.1", "30s"}})
	// Second fire within interval: should edit, not send.
	_ = n.Notify(ctx, Event{ID: "xray.api.unreachable", Severity: SeverityImportant, Params: []any{"127.0.0.1", "60s"}})
	if fs.sentCount() != 1 {
		t.Fatalf("expected 1 send, got %d", fs.sentCount())
	}
	if fs.editedCount() != 1 {
		t.Fatalf("expected 1 edit for coalesce, got %d", fs.editedCount())
	}
}

func TestNotifierCoalesceExpires(t *testing.T) {
	ctx := context.Background()
	st := newStoreForTest(t)
	prefs := store.NewDefaultAdminPrefs(666, "en")
	prefs.QuietFromHour = 0
	prefs.QuietToHour = 0
	_ = st.UpsertAdminPrefs(ctx, prefs)
	fs := &fakeSender{}
	n := newNotifierForTest(fs, st, newBundle(t), []int64{666}, "en")

	// Freeze time so we can advance past the coalesce window.
	t0 := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	now := t0
	n.now = func() time.Time { return now }

	_ = n.Notify(ctx, Event{ID: "xray.api.unreachable", Severity: SeverityImportant, Params: []any{"127.0.0.1", "30s"}})
	now = t0.Add(10 * time.Minute) // > 5m window
	_ = n.Notify(ctx, Event{ID: "xray.api.unreachable", Severity: SeverityImportant, Params: []any{"127.0.0.1", "30s"}})
	if fs.sentCount() != 2 {
		t.Fatalf("expected 2 sends after window expiry, got %d", fs.sentCount())
	}
}
