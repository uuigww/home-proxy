// Package migrations embeds the numbered SQL migration files applied by
// store.Open on first use. Files are discovered at init via go:embed and run
// in lexicographic order, tracked by the schema_version table.
package migrations

import "embed"

// FS holds every *.sql file in this directory, in lexicographic order.
//
//go:embed *.sql
var FS embed.FS
