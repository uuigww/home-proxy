package store

import (
	"context"
	"fmt"
	"time"
)

// TopUser is one entry from GetTopUsersByUsage.
type TopUser struct {
	UserID     int64
	Name       string
	TotalBytes int64
}

// RecordUsageDay adds (up, down) to the counters for (userID, day). The day
// is truncated to UTC midnight before lookup/upsert.
func (s *Store) RecordUsageDay(ctx context.Context, userID int64, day time.Time, up, down int64) error {
	if up < 0 || down < 0 {
		return fmt.Errorf("record usage: negative bytes up=%d down=%d", up, down)
	}
	d := day.UTC().Truncate(24 * time.Hour)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO usage_history (user_id, day, uplink_bytes, downlink_bytes)
VALUES (?, ?, ?, ?)
ON CONFLICT(user_id, day) DO UPDATE SET
    uplink_bytes = uplink_bytes + excluded.uplink_bytes,
    downlink_bytes = downlink_bytes + excluded.downlink_bytes`,
		userID, d, up, down)
	if err != nil {
		return fmt.Errorf("record usage user %d day %s: %w", userID, d.Format("2006-01-02"), err)
	}
	return nil
}

// GetUsageSince returns all usage_history rows for userID with day >= since,
// ordered by day ASC.
func (s *Store) GetUsageSince(ctx context.Context, userID int64, since time.Time) ([]UsageDay, error) {
	d := since.UTC().Truncate(24 * time.Hour)
	rows, err := s.db.QueryContext(ctx, `
SELECT user_id, day, uplink_bytes, downlink_bytes
FROM usage_history
WHERE user_id = ? AND day >= ?
ORDER BY day ASC`, userID, d)
	if err != nil {
		return nil, fmt.Errorf("usage since user %d: %w", userID, err)
	}
	defer rows.Close()
	var out []UsageDay
	for rows.Next() {
		var u UsageDay
		if err := rows.Scan(&u.UserID, &u.Day, &u.UplinkBytes, &u.DownlinkBytes); err != nil {
			return nil, fmt.Errorf("scan usage row: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate usage rows: %w", err)
	}
	return out, nil
}

// GetTopUsersByUsage returns up to n users ranked by total bytes transferred
// on or after since. Joins users to return the display name. Users with no
// recorded usage appear with TotalBytes=0 at the bottom of the ordering.
func (s *Store) GetTopUsersByUsage(ctx context.Context, n int, since time.Time) ([]TopUser, error) {
	if n <= 0 {
		return nil, nil
	}
	d := since.UTC().Truncate(24 * time.Hour)
	rows, err := s.db.QueryContext(ctx, `
SELECT u.id, u.name,
       COALESCE(SUM(h.uplink_bytes + h.downlink_bytes), 0) AS total
FROM users u
LEFT JOIN usage_history h ON h.user_id = u.id AND h.day >= ?
GROUP BY u.id, u.name
ORDER BY total DESC, u.name ASC
LIMIT ?`, d, n)
	if err != nil {
		return nil, fmt.Errorf("top users: %w", err)
	}
	defer rows.Close()
	var out []TopUser
	for rows.Next() {
		var t TopUser
		if err := rows.Scan(&t.UserID, &t.Name, &t.TotalBytes); err != nil {
			return nil, fmt.Errorf("scan top row: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate top rows: %w", err)
	}
	return out, nil
}
