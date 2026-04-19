package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// GetRealityKeys returns the singleton reality_keys row, or ErrNotFound.
func (s *Store) GetRealityKeys(ctx context.Context) (RealityKeys, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT private_key, public_key, short_id, dest, server_name, created_at
FROM reality_keys WHERE id = 1`)
	var r RealityKeys
	err := row.Scan(&r.PrivateKey, &r.PublicKey, &r.ShortID, &r.Dest, &r.ServerName, &r.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RealityKeys{}, ErrNotFound
		}
		return RealityKeys{}, fmt.Errorf("get reality keys: %w", err)
	}
	return r, nil
}

// SaveRealityKeys overwrites (or inserts) the singleton reality_keys row.
// CreatedAt is set to now if zero.
func (s *Store) SaveRealityKeys(ctx context.Context, r RealityKeys) error {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO reality_keys (id, private_key, public_key, short_id, dest, server_name, created_at)
VALUES (1, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    private_key = excluded.private_key,
    public_key = excluded.public_key,
    short_id = excluded.short_id,
    dest = excluded.dest,
    server_name = excluded.server_name,
    created_at = excluded.created_at`,
		r.PrivateKey, r.PublicKey, r.ShortID, r.Dest, r.ServerName, r.CreatedAt)
	if err != nil {
		return fmt.Errorf("save reality keys: %w", err)
	}
	return nil
}

// GetWarpPeer returns the singleton warp_peer row, or ErrNotFound.
func (s *Store) GetWarpPeer(ctx context.Context) (WarpPeer, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT private_key, peer_public_key, ipv4, ipv6, endpoint, mtu, updated_at
FROM warp_peer WHERE id = 1`)
	var p WarpPeer
	err := row.Scan(&p.PrivateKey, &p.PeerPublicKey, &p.IPv4, &p.IPv6, &p.Endpoint, &p.MTU, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WarpPeer{}, ErrNotFound
		}
		return WarpPeer{}, fmt.Errorf("get warp peer: %w", err)
	}
	return p, nil
}

// SaveWarpPeer overwrites (or inserts) the singleton warp_peer row.
// UpdatedAt is refreshed to now; a zero MTU defaults to 1280.
func (s *Store) SaveWarpPeer(ctx context.Context, p WarpPeer) error {
	p.UpdatedAt = time.Now().UTC()
	if p.MTU == 0 {
		p.MTU = 1280
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO warp_peer (id, private_key, peer_public_key, ipv4, ipv6, endpoint, mtu, updated_at)
VALUES (1, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    private_key = excluded.private_key,
    peer_public_key = excluded.peer_public_key,
    ipv4 = excluded.ipv4,
    ipv6 = excluded.ipv6,
    endpoint = excluded.endpoint,
    mtu = excluded.mtu,
    updated_at = excluded.updated_at`,
		p.PrivateKey, p.PeerPublicKey, p.IPv4, p.IPv6, p.Endpoint, p.MTU, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("save warp peer: %w", err)
	}
	return nil
}
