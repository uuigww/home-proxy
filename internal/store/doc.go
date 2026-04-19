// Package store owns the SQLite-backed persistence layer for home-proxy.
//
// It exposes typed CRUD helpers for users, per-admin notification preferences,
// the single-message bot session state, rolling daily usage history and
// singleton blobs for Reality and Warp credentials. All schema changes are
// applied automatically on Open via the embedded migrations in
// internal/store/migrations, tracked by a schema_version row. The driver is
// modernc.org/sqlite (pure Go, no CGO) registered under the "sqlite" name.
package store
