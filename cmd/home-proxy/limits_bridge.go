package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/uuigww/home-proxy/internal/bot"
	"github.com/uuigww/home-proxy/internal/limits"
)

// limitsBridge adapts *bot.Notifier (positional Params []any) to
// limits.Notifier (keyed map[string]any as the watcher builds them).
//
// It owns the mapping between the watcher's map keys and each i18n template's
// %s slots. When a template changes its slot order, update the corresponding
// formatter here — this file is the single source of truth for that coupling.
type limitsBridge struct {
	inner *bot.Notifier
}

// newLimitsBridge returns a limits.Notifier backed by the bot's notifier.
func newLimitsBridge(n *bot.Notifier) *limitsBridge { return &limitsBridge{inner: n} }

// formatter converts a limits.Event's map params into the []any positional
// slice expected by the bot notifier (which renders via fmt.Sprintf against
// the template resolved from i18n key "notif.<event_id>").
type formatter func(p map[string]any) []any

// formatters maps event_id to its formatter. Events without an entry fall
// back to positional values in alphabetical key order, which keeps delivery
// best-effort when a new event is added before this table catches up.
var formatters = map[string]formatter{
	"user.quota.80": func(p map[string]any) []any {
		return []any{str(p["name"]), humanBytesAny(p["limit"])}
	},
	"user.quota.100": func(p map[string]any) []any {
		return []any{str(p["name"]), humanBytesAny(p["limit"])}
	},
	"xray.api.unreachable": func(p map[string]any) []any {
		return []any{intAny(p["attempts"]), str(p["err"])}
	},
	"host.disk_low": func(p map[string]any) []any {
		return []any{str(p["mount"]), humanBytesAny(p["free"])}
	},
	"reality.key.age_warn": func(p map[string]any) []any {
		return []any{intAny(p["age_days"])}
	},
	"geo.data.stale": func(p map[string]any) []any {
		return []any{intAny(p["age_days"])}
	},
	"warp.outbound.down":  func(p map[string]any) []any { return nil },
	"xray.config.drift":   func(p map[string]any) []any { return nil },

	"digest.daily": func(p map[string]any) []any {
		return []any{
			str(p["date"]),
			humanBytesAny(p["total"]),
			humanBytesAny(p["total_up"]),
			humanBytesAny(p["total_down"]),
			intAny(p["active_users"]),
			intAny(p["active_users"]) + intAny(p["disabled_users"]),
			intAny(p["errors"]),
		}
	},
	"digest.weekly": func(p map[string]any) []any {
		return []any{str(p["date"])}
	},
}

// Notify implements limits.Notifier.
func (b *limitsBridge) Notify(ctx context.Context, ev limits.Event) error {
	var args []any
	if f, ok := formatters[ev.ID]; ok {
		args = f(ev.Params)
	} else {
		args = fallbackArgs(ev.Params)
	}
	return b.inner.Notify(ctx, bot.Event{
		ID:        ev.ID,
		Severity:  toBotSeverity(ev.Severity),
		Params:    args,
		Buttons:   toBotButtons(ev.Buttons),
		ActorTGID: ev.ActorTGID,
	})
}

func toBotSeverity(s limits.Severity) bot.Severity {
	switch s {
	case limits.Critical:
		return bot.SeverityCritical
	case limits.Important:
		return bot.SeverityImportant
	case limits.Info:
		return bot.SeverityInfo
	case limits.Security:
		return bot.SeveritySecurity
	case limits.Scheduled:
		return bot.SeverityScheduled
	default:
		return bot.SeverityInfo
	}
}

func toBotButtons(in [][]limits.Button) [][]bot.Button {
	if len(in) == 0 {
		return nil
	}
	out := make([][]bot.Button, len(in))
	for i, row := range in {
		r := make([]bot.Button, len(row))
		for j, btn := range row {
			r[j] = bot.Button{Text: btn.Label, Data: btn.Data}
		}
		out[i] = r
	}
	return out
}

// fallbackArgs returns the values of p in alphabetical key order.
func fallbackArgs(p map[string]any) []any {
	if len(p) == 0 {
		return nil
	}
	keys := make([]string, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]any, 0, len(keys))
	for _, k := range keys {
		out = append(out, p[k])
	}
	return out
}

// humanBytesAny formats any int-like value as a 1024-based human size. Returns
// the raw stringified value when the type is unexpected.
func humanBytesAny(v any) string {
	var n int64
	switch t := v.(type) {
	case int64:
		n = t
	case int:
		n = int64(t)
	case int32:
		n = int64(t)
	case uint64:
		n = int64(t)
	default:
		return fmt.Sprintf("%v", v)
	}
	return humanBytesMain(n)
}

func humanBytesMain(n int64) string {
	const (
		kib = 1024
		mib = 1024 * kib
		gib = 1024 * mib
		tib = 1024 * gib
	)
	switch {
	case n < kib:
		return fmt.Sprintf("%d B", n)
	case n < mib:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(kib))
	case n < gib:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(mib))
	case n < tib:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(gib))
	default:
		return fmt.Sprintf("%.1f TB", float64(n)/float64(tib))
	}
}

func str(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func intAny(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case int32:
		return int(t)
	case uint64:
		return int(t)
	case float64:
		return int(t)
	default:
		return 0
	}
}
