// SPDX-License-Identifier: Apache-2.0

// Package migrations holds the SQL migrations of the server, embedded into the binary
// (ADR 0007). Only `bothy migrate` applies them (R-AR-16); the server start compares the
// database with the highest version here (pg.RequiredSchemaVersion, pg.CheckSchema).
//
// Each file is NNNNN_<slug>.sql with a sequential five digit version and goose annotations;
// README.md in this directory states the numbering, grant and expand and contract rules.
//
// Example:
//
//	need, err := pg.RequiredSchemaVersion(migrations.FS)
package migrations

import "embed"

// FS contains every migration file (*.sql) of this directory at its root.
//
//go:embed *.sql
var FS embed.FS
