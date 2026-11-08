// Package migrations embeds the Goose SQL migrations, so the binary carries them.
// Migrations are forward-only and additive (ADR 0008): add a new file, never edit one.
package migrations

import "embed"

// FS holds every migration file in this directory.
//
//go:embed *.sql
var FS embed.FS
