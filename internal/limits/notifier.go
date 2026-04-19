package limits

import "context"

// Notifier is the subset of bot.Notifier the watcher depends on.
//
// The concrete implementation lives in internal/bot but is injected here
// through this interface so the watcher has no compile-time dependency
// on the bot package (prevents an import cycle and keeps tests trivial).
type Notifier interface {
	// Notify delivers ev to the admin channel. Implementations are
	// responsible for rate-limiting, quiet-hour batching, i18n
	// rendering and retry handling; the watcher's job is to fire-and-
	// forget.
	Notify(ctx context.Context, ev Event) error
}

// Event mirrors bot.Event but lives in this package so limits has no
// dep on bot. bot.Notifier satisfies limits.Notifier by accepting the
// same fields (the bot package converts limits.Event to its own type).
type Event struct {
	// ID is the stable event identifier from docs/notifications.md,
	// e.g. "user.quota.80", "xray.api.unreachable", "digest.daily".
	ID string
	// Severity controls delivery semantics (quiet-hour suppression,
	// toggles). See the Severity constants.
	Severity Severity
	// Params are the template parameters consumed by the renderer.
	// Values must be trivially JSON-encodable (strings, numbers,
	// bools, durations as strings).
	Params map[string]any
	// Buttons are optional inline keyboard rows for the rendered
	// message. Each inner slice is one row.
	Buttons [][]Button
	// ActorTGID is the Telegram user ID that triggered this event,
	// or 0 when the event is system-generated (quota, digest,
	// health). Used by the bot to filter "audit of my own action".
	ActorTGID int64
}

// Severity classifies an Event for delivery policy.
//
// The integer values mirror the five rows in docs/notifications.md;
// the enum starts at 1 so the zero value is an obvious bug.
type Severity int

// Severity levels, mirroring docs/notifications.md.
const (
	// Critical events are always delivered, ignoring per-admin
	// preferences and quiet hours.
	Critical Severity = iota + 1
	// Important events need admin attention today (quota hits, disk
	// low, reality key age).
	Important
	// Info events form the audit trail and are delivered to all
	// admins except the actor.
	Info
	// Security events cover access, tamper and trust signals.
	Security
	// Scheduled events are the periodic daily/weekly digests.
	Scheduled
)

// Button is one inline-keyboard button attached to a rendered Event.
//
// Label is the user-facing text; Data is the callback_data payload
// the bot handles when the button is pressed.
type Button struct {
	Label string
	Data  string
}
