package limits

import (
	"context"
	"fmt"
	"time"

	"github.com/uuigww/home-proxy/internal/store"
)

// UserUsage is one row of the digest's top-N list.
type UserUsage struct {
	// Name is the user's display name (unique in the users table).
	Name string
	// Bytes is the summed uplink+downlink for the digest window.
	Bytes int64
}

// DailyDigest is the payload behind a digest.daily notification.
//
// All timestamps are UTC; the consumer renders them in the admin's
// local time zone when composing the Telegram message.
type DailyDigest struct {
	// Date is UTC midnight of the day the digest covers.
	Date time.Time
	// TotalUp is the sum of uplink bytes across all users on Date.
	TotalUp int64
	// TotalDown is the sum of downlink bytes across all users on Date.
	TotalDown int64
	// ActiveUsers counts enabled users at the time the digest ran.
	ActiveUsers int
	// DisabledUsers counts disabled users at the time the digest ran.
	DisabledUsers int
	// Top is the top-N users by total bytes for Date (up to 3).
	Top []UserUsage
	// Errors is the count of error-severity events logged today.
	// MVP: always 0 until the event-log store lands.
	Errors int
}

// digestStore is the subset of *store.Store the digest helpers need.
// Tests pass a fake implementing this interface.
type digestStore interface {
	ListUsers(ctx context.Context, includeDisabled bool) ([]store.User, error)
	GetUsageSince(ctx context.Context, userID int64, since time.Time) ([]store.UsageDay, error)
	GetTopUsersByUsage(ctx context.Context, n int, since time.Time) ([]store.TopUser, error)
}

// computeDailyDigest gathers totals, per-user breakdown and the top 3
// for today (UTC). The returned DailyDigest is ready to hand to
// (DailyDigest).ToParams for inclusion in a digest.daily Event.
func computeDailyDigest(ctx context.Context, s digestStore, now time.Time) (DailyDigest, error) {
	d := DailyDigest{Date: now.UTC().Truncate(24 * time.Hour)}

	users, err := s.ListUsers(ctx, true)
	if err != nil {
		return d, fmt.Errorf("digest: list users: %w", err)
	}
	for _, u := range users {
		if u.Enabled {
			d.ActiveUsers++
		} else {
			d.DisabledUsers++
		}
		rows, err := s.GetUsageSince(ctx, u.ID, d.Date)
		if err != nil {
			return d, fmt.Errorf("digest: usage %d: %w", u.ID, err)
		}
		for _, r := range rows {
			if !r.Day.UTC().Equal(d.Date) {
				continue
			}
			d.TotalUp += r.UplinkBytes
			d.TotalDown += r.DownlinkBytes
		}
	}

	top, err := s.GetTopUsersByUsage(ctx, 3, d.Date)
	if err != nil {
		return d, fmt.Errorf("digest: top users: %w", err)
	}
	for _, t := range top {
		if t.TotalBytes == 0 {
			continue
		}
		d.Top = append(d.Top, UserUsage{Name: t.Name, Bytes: t.TotalBytes})
	}
	return d, nil
}

// ToParams converts the digest into the Event.Params map shape the
// notifier's renderer consumes.
func (d DailyDigest) ToParams() map[string]any {
	top := make([]map[string]any, 0, len(d.Top))
	for _, u := range d.Top {
		top = append(top, map[string]any{
			"name":  u.Name,
			"bytes": u.Bytes,
		})
	}
	return map[string]any{
		"date":           d.Date.Format("2006-01-02"),
		"total_up":       d.TotalUp,
		"total_down":     d.TotalDown,
		"total":          d.TotalUp + d.TotalDown,
		"active_users":   d.ActiveUsers,
		"disabled_users": d.DisabledUsers,
		"top":            top,
		"errors":         d.Errors,
	}
}
