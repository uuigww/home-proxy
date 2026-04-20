package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CreateUser inserts a new user, populating CreatedAt/UpdatedAt and returning
// the assigned ID.
func (s *Store) CreateUser(ctx context.Context, u *User) error {
	if u == nil {
		return fmt.Errorf("create user: nil user")
	}
	now := time.Now().UTC()
	u.CreatedAt = now
	u.UpdatedAt = now
	res, err := s.db.ExecContext(ctx, `
INSERT INTO users (name, vless_uuid, socks_user, socks_pass, limit_bytes, used_bytes, enabled, mtproto_enabled, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.Name, u.VLESSUUID, u.SOCKSUser, u.SOCKSPass,
		u.LimitBytes, u.UsedBytes, boolToInt(u.Enabled), boolToInt(u.MTProtoEnabled),
		u.CreatedAt, u.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert user %q: %w", u.Name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	u.ID = id
	return nil
}

// GetUser loads a user by primary key.
func (s *Store) GetUser(ctx context.Context, id int64) (User, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, vless_uuid, socks_user, socks_pass, limit_bytes, used_bytes, enabled, mtproto_enabled, created_at, updated_at
FROM users WHERE id = ?`, id)
	return scanUser(row)
}

// GetUserByName loads a user by unique name.
func (s *Store) GetUserByName(ctx context.Context, name string) (User, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, vless_uuid, socks_user, socks_pass, limit_bytes, used_bytes, enabled, mtproto_enabled, created_at, updated_at
FROM users WHERE name = ?`, name)
	return scanUser(row)
}

// ListUsers returns every user; when includeDisabled is false, disabled rows
// are filtered out. Ordered by name ASC for stable UI listings.
func (s *Store) ListUsers(ctx context.Context, includeDisabled bool) ([]User, error) {
	q := `
SELECT id, name, vless_uuid, socks_user, socks_pass, limit_bytes, used_bytes, enabled, mtproto_enabled, created_at, updated_at
FROM users`
	if !includeDisabled {
		q += ` WHERE enabled = 1`
	}
	q += ` ORDER BY name ASC`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return out, nil
}

// UpdateUser overwrites the mutable fields of an existing user and bumps
// UpdatedAt. The ID must be set.
func (s *Store) UpdateUser(ctx context.Context, u *User) error {
	if u == nil || u.ID == 0 {
		return fmt.Errorf("update user: missing id")
	}
	u.UpdatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
UPDATE users SET
    name = ?, vless_uuid = ?, socks_user = ?, socks_pass = ?,
    limit_bytes = ?, used_bytes = ?, enabled = ?, mtproto_enabled = ?, updated_at = ?
WHERE id = ?`,
		u.Name, u.VLESSUUID, u.SOCKSUser, u.SOCKSPass,
		u.LimitBytes, u.UsedBytes, boolToInt(u.Enabled), boolToInt(u.MTProtoEnabled), u.UpdatedAt,
		u.ID)
	if err != nil {
		return fmt.Errorf("update user %d: %w", u.ID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUser removes a user by ID. Returns ErrNotFound if no row matched.
// usage_history rows cascade automatically via the foreign-key constraint.
func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete user %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetUserEnabled flips the enabled flag and bumps updated_at.
func (s *Store) SetUserEnabled(ctx context.Context, id int64, enabled bool) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET enabled = ?, updated_at = ? WHERE id = ?`,
		boolToInt(enabled), time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("set enabled user %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetUserMTProtoEnabled flips the MTProto UI flag and bumps updated_at.
//
// MTProto-enabled users see the shared tg://proxy link in the bot; mtg does
// not enforce this at the protocol level (single shared secret).
func (s *Store) SetUserMTProtoEnabled(ctx context.Context, id int64, on bool) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET mtproto_enabled = ?, updated_at = ? WHERE id = ?`,
		boolToInt(on), time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("set mtproto user %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// AddUserUsage atomically increments used_bytes by up+down. Either value may
// be zero. Negative values are rejected.
func (s *Store) AddUserUsage(ctx context.Context, id int64, up, down int64) error {
	if up < 0 || down < 0 {
		return fmt.Errorf("add usage: negative delta up=%d down=%d", up, down)
	}
	delta := up + down
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET used_bytes = used_bytes + ?, updated_at = ? WHERE id = ?`,
		delta, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("add usage user %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// scanUser decodes one row from a users SELECT. Works with *sql.Row and
// *sql.Rows via the shared Scan interface.
func scanUser(r scanRow) (User, error) {
	var (
		u       User
		enabled int64
		mtproto int64
		vlessU  sql.NullString
		socksU  sql.NullString
		socksP  sql.NullString
	)
	err := r.Scan(
		&u.ID, &u.Name, &vlessU, &socksU, &socksP,
		&u.LimitBytes, &u.UsedBytes, &enabled, &mtproto,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("scan user: %w", err)
	}
	if vlessU.Valid {
		v := vlessU.String
		u.VLESSUUID = &v
	}
	if socksU.Valid {
		v := socksU.String
		u.SOCKSUser = &v
	}
	if socksP.Valid {
		v := socksP.String
		u.SOCKSPass = &v
	}
	u.Enabled = enabled != 0
	u.MTProtoEnabled = mtproto != 0
	return u, nil
}
