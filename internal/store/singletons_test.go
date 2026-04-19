package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRealitySingleton(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	if _, err := s.GetRealityKeys(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound on empty store, got %v", err)
	}

	r := RealityKeys{
		PrivateKey: "priv", PublicKey: "pub", ShortID: "ab",
		Dest: "www.google.com:443", ServerName: "www.google.com",
	}
	if err := s.SaveRealityKeys(ctx, r); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.GetRealityKeys(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.PrivateKey != "priv" || got.PublicKey != "pub" || got.ShortID != "ab" {
		t.Fatalf("unexpected keys: %+v", got)
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("CreatedAt was not set")
	}

	r.PublicKey = "newpub"
	if err := s.SaveRealityKeys(ctx, r); err != nil {
		t.Fatalf("resave: %v", err)
	}
	got2, _ := s.GetRealityKeys(ctx)
	if got2.PublicKey != "newpub" {
		t.Fatalf("update not persisted: %+v", got2)
	}

	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM reality_keys`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 reality_keys row, got %d", n)
	}

	// CHECK(id=1) must reject id=2.
	_, err = s.DB().Exec(`INSERT INTO reality_keys (id, private_key, public_key, short_id, dest, server_name, created_at)
VALUES (2, '', '', '', '', '', ?)`, time.Now().UTC())
	if err == nil {
		t.Fatal("expected CHECK(id=1) to reject id=2 insert")
	}
	le := strings.ToLower(err.Error())
	if !strings.Contains(le, "check") && !strings.Contains(le, "constraint") {
		t.Fatalf("expected constraint error, got %v", err)
	}
}

func TestWarpSingleton(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	if _, err := s.GetWarpPeer(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound on empty store, got %v", err)
	}

	p := WarpPeer{
		PrivateKey: "pk", PeerPublicKey: "ppk",
		IPv4: "10.0.0.2", IPv6: "fd00::2", Endpoint: "engage.cloudflareclient.com:2408",
	}
	if err := s.SaveWarpPeer(ctx, p); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.GetWarpPeer(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.MTU != 1280 {
		t.Fatalf("expected MTU default 1280, got %d", got.MTU)
	}
	if got.PrivateKey != "pk" || got.Endpoint != p.Endpoint {
		t.Fatalf("unexpected warp peer: %+v", got)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt was not refreshed")
	}

	_, err = s.DB().Exec(`INSERT INTO warp_peer (id, private_key, peer_public_key, ipv4, ipv6, endpoint, mtu, updated_at)
VALUES (2, '', '', '', '', '', 1280, ?)`, time.Now().UTC())
	if err == nil {
		t.Fatal("expected CHECK(id=1) to reject id=2 insert")
	}
}
