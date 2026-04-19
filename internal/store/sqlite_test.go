package store

import (
	"context"
	"path/filepath"
	"testing"
)

// newStore opens a fresh SQLite database inside t.TempDir() and registers a
// Cleanup hook that closes it at the end of the test.
func newStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "home-proxy.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// ptr is a tiny helper to take the address of a string literal in test
// structs (User.VLESSUUID et al. are *string).
func ptr(s string) *string { return &s }

func TestMigrationsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mig.db")

	s1, err := Open(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	var v1 int
	if err := s1.DB().QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&v1); err != nil {
		t.Fatalf("read schema_version: %v", err)
	}
	if v1 < 1 {
		t.Fatalf("expected schema_version >= 1, got %d", v1)
	}
	_ = s1.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer s2.Close()
	var v2 int
	if err := s2.DB().QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&v2); err != nil {
		t.Fatalf("read schema_version: %v", err)
	}
	if v2 != v1 {
		t.Fatalf("schema_version changed on re-open: %d -> %d", v1, v2)
	}

	var n int
	if err := s2.DB().QueryRow(`SELECT COUNT(*) FROM schema_version`).Scan(&n); err != nil {
		t.Fatalf("count schema_version: %v", err)
	}
	if n != v1 {
		t.Fatalf("expected %d schema_version rows, got %d", v1, n)
	}
}

func TestPing(t *testing.T) {
	s := newStore(t)
	if err := s.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestMigrationVersionHelper(t *testing.T) {
	cases := []struct {
		name string
		want int
		ok   bool
	}{
		{"001_initial.sql", 1, true},
		{"042_add_table.sql", 42, true},
		{"no_prefix.sql", 0, false},
	}
	for _, tc := range cases {
		got, err := migrationVersion(tc.name)
		if tc.ok && err != nil {
			t.Errorf("%s: unexpected error %v", tc.name, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
		if tc.ok && got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}
