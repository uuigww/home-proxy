package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/uuigww/home-proxy/internal/i18n"
	"github.com/uuigww/home-proxy/internal/store"
)

// Severity tiers a notification can belong to.
type Severity int

const (
	// SeverityCritical is always delivered ignoring quiet hours and prefs.
	SeverityCritical Severity = iota + 1
	// SeverityImportant is delivered unless the admin disabled it.
	SeverityImportant
	// SeverityInfo is an audit-trail event (user CRUD, admin actions).
	SeverityInfo
	// SeveritySecurity is a trust / tamper signal.
	SeveritySecurity
	// SeverityScheduled is the periodic digests.
	SeverityScheduled
)

// Button is the sub-set of the Telegram inline button we need for notifier
// messages; callers can attach a URL or a callback_data payload.
type Button struct {
	Text string
	URL  string
	Data string
}

// Event is a single notification to dispatch.
//
// Params are passed directly to fmt.Sprintf against the i18n template
// (i18n key "notif.<ID>"). ActorTGID is the Telegram user id of whoever
// caused the event; for Info events honouring NotifyInfoOthersOnly this
// admin is skipped.
type Event struct {
	ID        string
	Severity  Severity
	Params    []any
	Buttons   [][]Button
	ActorTGID int64
}

// tgSender is the subset of the Telegram client the notifier uses. Kept
// narrow so tests can provide a stub without mocking the whole client.
type tgSender interface {
	SendMessage(ctx context.Context, p *bot.SendMessageParams) (*models.Message, error)
	EditMessageText(ctx context.Context, p *bot.EditMessageTextParams) (*models.Message, error)
}

// Notifier pushes Events to admins, honouring their preferences.
//
// Concurrency: all exported methods are safe for concurrent use. Internal
// state (last-sent and quiet outbox) is guarded by a single mutex; volumes
// are small (at most a few hundred events per day in steady state) so this
// is simpler than sharding by admin.
type Notifier struct {
	send     tgSender
	store    *store.Store
	i18n     *i18n.Bundle
	admins   []int64
	defLang  string
	log      *slog.Logger
	now      func() time.Time
	interval map[string]time.Duration

	mu     sync.Mutex
	last   map[notifyKey]*lastSent // key -> last message info (for coalescing)
	outbox map[int64][]Event       // admin -> events queued for quiet-hours
}

// notifyKey identifies a coalescing slot: same event for same admin.
type notifyKey struct {
	adminID int64
	eventID string
}

// lastSent records enough context to update-in-place when coalescing.
type lastSent struct {
	chatID    int64
	messageID int
	at        time.Time
	count     int
	text      string
}

// NewNotifier builds a Notifier from the daemon collaborators.
//
// When tg is nil (tests) messages are merely logged and the notifier
// becomes a pure state machine.
func NewNotifier(
	tg *bot.Bot,
	s *store.Store,
	bundle *i18n.Bundle,
	admins []int64,
	defLang string,
	log *slog.Logger,
) *Notifier {
	if log == nil {
		log = slog.Default()
	}
	n := &Notifier{
		store:    s,
		i18n:     bundle,
		admins:   admins,
		defLang:  defLang,
		log:      log,
		now:      time.Now,
		interval: defaultIntervals(),
		last:     map[notifyKey]*lastSent{},
		outbox:   map[int64][]Event{},
	}
	if tg != nil {
		n.send = tgSenderAdapter{tg: tg}
	}
	return n
}

// newNotifierForTest builds a Notifier around an arbitrary tgSender. Used
// only by tests — hence the lowercase name.
func newNotifierForTest(
	send tgSender,
	s *store.Store,
	bundle *i18n.Bundle,
	admins []int64,
	defLang string,
) *Notifier {
	n := NewNotifier(nil, s, bundle, admins, defLang, slog.Default())
	n.send = send
	return n
}

// tgSenderAdapter adapts *bot.Bot to the tgSender interface; this keeps the
// main type cleanly separated from the SDK for testing.
type tgSenderAdapter struct{ tg *bot.Bot }

func (a tgSenderAdapter) SendMessage(ctx context.Context, p *bot.SendMessageParams) (*models.Message, error) {
	return a.tg.SendMessage(ctx, p)
}
func (a tgSenderAdapter) EditMessageText(ctx context.Context, p *bot.EditMessageTextParams) (*models.Message, error) {
	return a.tg.EditMessageText(ctx, p)
}

// defaultIntervals returns the coalesce window per event_id, mirroring
// docs/notifications.md.
func defaultIntervals() map[string]time.Duration {
	return map[string]time.Duration{
		"xray.api.unreachable":        5 * time.Minute,
		"xray.process.restart_failed": 10 * time.Minute,
		"warp.outbound.down":          15 * time.Minute,
		"xray.config.build_failed":    2 * time.Minute,
		"host.disk_low":               6 * time.Hour,
		"reality.key.age_warn":        7 * 24 * time.Hour,
		"geo.data.stale":              24 * time.Hour,
		"user.created":                time.Second,
		"user.deleted":                time.Second,
		"user.enabled":                time.Second,
		"user.disabled":               time.Second,
		"user.limit_changed":          time.Second,
		"user.protocols_changed":      time.Second,
	}
}

// Notify dispatches ev to every admin per docs/notifications.md.
func (n *Notifier) Notify(ctx context.Context, ev Event) error {
	if n == nil {
		return fmt.Errorf("notifier: nil receiver")
	}
	if ev.ID == "" {
		return fmt.Errorf("notifier: event id required")
	}
	var firstErr error
	for _, adminID := range n.admins {
		if err := n.deliver(ctx, adminID, ev); err != nil {
			n.log.Warn("notifier: deliver", "err", err, "admin", adminID, "event", ev.ID)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// deliver is the per-admin branch of Notify.
func (n *Notifier) deliver(ctx context.Context, adminID int64, ev Event) error {
	prefs, err := n.prefsFor(ctx, adminID)
	if err != nil {
		return err
	}

	// Filter by severity + preferences.
	if !n.severityEnabled(ev.Severity, prefs) {
		return nil
	}
	if ev.Severity == SeverityInfo && prefs.NotifyInfoOthersOnly && ev.ActorTGID == adminID {
		return nil
	}

	// Quiet hours: critical always bypasses.
	if ev.Severity != SeverityCritical && n.inQuiet(prefs, n.now()) {
		n.mu.Lock()
		n.outbox[adminID] = append(n.outbox[adminID], ev)
		n.mu.Unlock()
		return nil
	}

	// Coalesce: if same event re-fires inside its interval, update the last
	// message with a counter suffix instead of sending a new one.
	text := n.render(prefs.Lang, ev)
	key := notifyKey{adminID: adminID, eventID: ev.ID}
	n.mu.Lock()
	if prev, ok := n.last[key]; ok {
		if window, ok := n.interval[ev.ID]; ok && n.now().Sub(prev.at) <= window {
			prev.count++
			newText := text + n.i18n.T(prefs.Lang, "batch.coalesced", prev.count)
			prev.text = newText
			n.mu.Unlock()
			if n.send != nil && prev.messageID != 0 {
				_, err := n.send.EditMessageText(ctx, &bot.EditMessageTextParams{
					ChatID:    prev.chatID,
					MessageID: prev.messageID,
					Text:      newText,
					ParseMode: models.ParseModeHTML,
				})
				if err != nil {
					n.log.Warn("notifier: edit coalesced", "err", err, "event", ev.ID)
				}
			}
			return nil
		}
	}
	n.mu.Unlock()

	if n.send == nil {
		// No transport (tests): still record for coalescing.
		n.mu.Lock()
		n.last[key] = &lastSent{chatID: adminID, messageID: 0, at: n.now(), count: 1, text: text}
		n.mu.Unlock()
		return nil
	}

	msg, err := n.send.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      adminID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: buttonsToMarkup(ev.Buttons),
	})
	if err != nil {
		return err
	}
	if msg != nil {
		n.mu.Lock()
		n.last[key] = &lastSent{chatID: adminID, messageID: msg.ID, at: n.now(), count: 1, text: text}
		n.mu.Unlock()
	}
	return nil
}

// severityEnabled reports whether the admin opted into this severity class.
// Critical is always enabled regardless of the flag for safety.
func (n *Notifier) severityEnabled(s Severity, p store.AdminPrefs) bool {
	switch s {
	case SeverityCritical:
		return true
	case SeverityImportant:
		return p.NotifyImportant
	case SeverityInfo:
		return p.NotifyInfo
	case SeveritySecurity:
		return p.NotifySecurity
	case SeverityScheduled:
		return p.NotifyDaily
	}
	return false
}

// prefsFor reads and memoises AdminPrefs, falling back to defaults when the
// admin has no row yet.
func (n *Notifier) prefsFor(ctx context.Context, adminID int64) (store.AdminPrefs, error) {
	if n.store == nil {
		p := store.NewDefaultAdminPrefs(adminID, n.defLang)
		return p, nil
	}
	p, err := n.store.GetAdminPrefs(ctx, adminID)
	if err == nil {
		return p, nil
	}
	if errors.Is(err, store.ErrNotFound) {
		return store.NewDefaultAdminPrefs(adminID, n.defLang), nil
	}
	return store.AdminPrefs{}, err
}

// inQuiet reports whether now is inside the admin's quiet window. Supports
// wrap-around (e.g. 23→07).
func (n *Notifier) inQuiet(p store.AdminPrefs, now time.Time) bool {
	from := p.QuietFromHour
	to := p.QuietToHour
	if from == to {
		return false
	}
	h := now.Hour()
	if from < to {
		return h >= from && h < to
	}
	return h >= from || h < to
}

// render applies i18n template to the event; Buttons are ignored here —
// they are attached separately on send.
func (n *Notifier) render(lang string, ev Event) string {
	if n.i18n == nil {
		return ev.ID
	}
	return n.i18n.T(lang, "notif."+ev.ID, ev.Params...)
}

// buttonsToMarkup converts the generic Button type to Telegram inline
// keyboard. Returns nil when no buttons were supplied (Telegram expects the
// field to be omitted, not empty).
func buttonsToMarkup(rows [][]Button) *models.InlineKeyboardMarkup {
	if len(rows) == 0 {
		return nil
	}
	out := make([][]models.InlineKeyboardButton, 0, len(rows))
	for _, row := range rows {
		r := make([]models.InlineKeyboardButton, 0, len(row))
		for _, b := range row {
			if b.URL != "" {
				r = append(r, models.InlineKeyboardButton{Text: b.Text, URL: b.URL})
			} else {
				r = append(r, models.InlineKeyboardButton{Text: b.Text, CallbackData: b.Data})
			}
		}
		out = append(out, r)
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: out}
}

