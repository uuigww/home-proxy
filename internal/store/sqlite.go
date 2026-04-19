package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/uuigww/home-proxy/internal/store/migrations"
)

// ErrNotFound is returned by lookup methods when no row matches the query.
var ErrNotFound = errors.New("store: not found")

// Store is the SQLite-backed persistence layer used by the daemon.
//
// All methods are safe for concurrent use; the underlying *sql.DB maintains
// its own connection pool and SQLite runs in WAL mode.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database at path and applies every
// embedded migration that has not been recorded yet.
//
// The pragmas journal_mode=WAL, foreign_keys=ON and busy_timeout=5000 are set
// immediately after opening. Callers must eventually call Close.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite %q: %w", path, err)
	}

	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("apply %q: %w", p, err)
		}
	}

	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close closes the underlying *sql.DB.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Ping verifies the database handle is alive.
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// DB exposes the underlying handle for advanced callers (integrity checks,
// backups). Prefer the typed methods on Store whenever possible.
func (s *Store) DB() *sql.DB { return s.db }

// migrate ensures schema_version exists, then applies every embedded *.sql
// file whose numeric prefix is greater than the recorded version, in order.
func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_version (
    version    INTEGER PRIMARY KEY,
    applied_at DATETIME NOT NULL
);`); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}

	var current int
	row := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_version`)
	if err := row.Scan(&current); err != nil {
		return fmt.Errorf("read schema_version: %w", err)
	}

	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		ver, err := migrationVersion(name)
		if err != nil {
			return fmt.Errorf("parse migration %q: %w", name, err)
		}
		if ver <= current {
			continue
		}
		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read migration %q: %w", name, err)
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx for %q: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %q: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_version(version, applied_at) VALUES (?, ?)`,
			ver, time.Now().UTC()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %q: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %q: %w", name, err)
		}
		current = ver
	}
	return nil
}

// migrationVersion extracts the leading numeric prefix from a migration
// filename such as "001_initial.sql" -> 1.
func migrationVersion(name string) (int, error) {
	end := 0
	for end < len(name) && name[end] >= '0' && name[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, fmt.Errorf("filename %q has no numeric prefix", name)
	}
	var v int
	if _, err := fmt.Sscanf(name[:end], "%d", &v); err != nil {
		return 0, fmt.Errorf("parse prefix %q: %w", name[:end], err)
	}
	return v, nil
}

// scanRow is the subset of *sql.Row / *sql.Rows we depend on for scanning.
type scanRow interface {
	Scan(dest ...any) error
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
