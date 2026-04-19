package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// GetAdminPrefs returns the stored preferences for tgID, or ErrNotFound.
func (s *Store) GetAdminPrefs(ctx context.Context, tgID int64) (AdminPrefs, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT tg_id, lang, notify_critical, notify_important, notify_info,
       notify_info_others_only, notify_security, notify_daily,
       notify_nonadmin_spam, digest_hour, quiet_from_hour, quiet_to_hour,
       updated_at
FROM admin_prefs WHERE tg_id = ?`, tgID)
	return scanAdminPrefs(row)
}

// UpsertAdminPrefs inserts or replaces the row for prefs.TGID. UpdatedAt is
// refreshed automatically.
func (s *Store) UpsertAdminPrefs(ctx context.Context, prefs AdminPrefs) error {
	if prefs.TGID == 0 {
		return fmt.Errorf("upsert admin prefs: missing tg_id")
	}
	prefs.UpdatedAt = time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO admin_prefs (
    tg_id, lang, notify_critical, notify_important, notify_info,
    notify_info_others_only, notify_security, notify_daily,
    notify_nonadmin_spam, digest_hour, quiet_from_hour, quiet_to_hour,
    updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(tg_id) DO UPDATE SET
    lang = excluded.lang,
    notify_critical = excluded.notify_critical,
    notify_important = excluded.notify_important,
    notify_info = excluded.notify_info,
    notify_info_others_only = excluded.notify_info_others_only,
    notify_security = excluded.notify_security,
    notify_daily = excluded.notify_daily,
    notify_nonadmin_spam = excluded.notify_nonadmin_spam,
    digest_hour = excluded.digest_hour,
    quiet_from_hour = excluded.quiet_from_hour,
    quiet_to_hour = excluded.quiet_to_hour,
    updated_at = excluded.updated_at`,
		prefs.TGID, prefs.Lang,
		boolToInt(prefs.NotifyCritical), boolToInt(prefs.NotifyImportant),
		boolToInt(prefs.NotifyInfo), boolToInt(prefs.NotifyInfoOthersOnly),
		boolToInt(prefs.NotifySecurity), boolToInt(prefs.NotifyDaily),
		boolToInt(prefs.NotifyNonadminSpam),
		prefs.DigestHour, prefs.QuietFromHour, prefs.QuietToHour,
		prefs.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert admin prefs %d: %w", prefs.TGID, err)
	}
	return nil
}

// ListAdminPrefs returns every admin_prefs row ordered by tg_id ASC.
func (s *Store) ListAdminPrefs(ctx context.Context) ([]AdminPrefs, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT tg_id, lang, notify_critical, notify_important, notify_info,
       notify_info_others_only, notify_security, notify_daily,
       notify_nonadmin_spam, digest_hour, quiet_from_hour, quiet_to_hour,
       updated_at
FROM admin_prefs ORDER BY tg_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list admin prefs: %w", err)
	}
	defer rows.Close()
	var out []AdminPrefs
	for rows.Next() {
		p, err := scanAdminPrefs(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin prefs: %w", err)
	}
	return out, nil
}

func scanAdminPrefs(r scanRow) (AdminPrefs, error) {
	var (
		p                                             AdminPrefs
		crit, imp, info, infoOthers, sec, daily, spam int64
	)
	err := r.Scan(
		&p.TGID, &p.Lang,
		&crit, &imp, &info, &infoOthers, &sec, &daily, &spam,
		&p.DigestHour, &p.QuietFromHour, &p.QuietToHour,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AdminPrefs{}, ErrNotFound
		}
		return AdminPrefs{}, fmt.Errorf("scan admin prefs: %w", err)
	}
	p.NotifyCritical = crit != 0
	p.NotifyImportant = imp != 0
	p.NotifyInfo = info != 0
	p.NotifyInfoOthersOnly = infoOthers != 0
	p.NotifySecurity = sec != 0
	p.NotifyDaily = daily != 0
	p.NotifyNonadminSpam = spam != 0
	return p, nil
}
