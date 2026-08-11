// Package migrations embeds the SQL migration files into the compiled
// binary, so cmd/migrate (and, for tests, the test binary) doesn't need the
// source tree present at runtime — just the binary itself.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
