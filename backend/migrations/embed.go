// Package migrations embeds Beacon's SQL schema migrations into the binary so
// the API can apply them without shipping loose files. Migrations are plain,
// hand-written SQL (never auto-generated) named NNNN_description.up.sql and
// NNNN_description.down.sql. They are applied in ascending numeric order.
package migrations

import "embed"

// FS holds every migration file.
//
//go:embed *.sql
var FS embed.FS
