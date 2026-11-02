package migrations

import "embed"

// FS holds all SQL migration files in this directory.
// The //go:embed directive runs at compile time, so the migrations
// are baked into the binary and require no external files at runtime.
//
//go:embed *.sql
var FS embed.FS
