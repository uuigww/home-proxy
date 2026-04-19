package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// GetSession returns the bot's single-message state for the given admin.
func (s *Store) GetSession(ctx context.Context, tgID int64) (Session, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT tg_id, chat_id, message_id, screen, wizard_json, updated_at
FROM sessions WHERE tg_id = ?`, tgID)
	var sess Session
	err := row.Scan(&sess.TGID, &sess.ChatID, &sess.MessageID,
		&sess.Screen, &sess.WizardJSON, &sess.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrNotFound
		}
		return Session{}, fmt.Errorf("get session %d: %w", tgID, err)
	}
	return sess, nil
}

// UpsertSession inserts or replaces the session row for sess.TGID.
// An empty WizardJSON is normalised to "{}" before writing.
func (s *Store) UpsertSession(ctx context.Context, sess Session) error {
	if sess.TGID == 0 {
		return fmt.Errorf("upsert session: missing tg_id")
	}
	if sess.WizardJSON == "" {
		sess.WizardJSON = "{}"
	}
	sess.UpdatedAt = time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO sessions (tg_id, chat_id, message_id, screen, wizard_json, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(tg_id) DO UPDATE SET
    chat_id = excluded.chat_id,
    message_id = excluded.message_id,
    screen = excluded.screen,
    wizard_json = excluded.wizard_json,
    updated_at = excluded.updated_at`,
		sess.TGID, sess.ChatID, sess.MessageID,
		sess.Screen, sess.WizardJSON, sess.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert session %d: %w", sess.TGID, err)
	}
	return nil
}

// DeleteSession removes the session row for tgID. Missing rows are not an
// error.
func (s *Store) DeleteSession(ctx context.Context, tgID int64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE tg_id = ?`, tgID); err != nil {
		return fmt.Errorf("delete session %d: %w", tgID, err)
	}
	return nil
}
