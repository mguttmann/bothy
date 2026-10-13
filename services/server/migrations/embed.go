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

// MinServerVersion is the oldest server version allowed to run against the schema these
// migrations produce (docs/spec/18-release.md "Components" rule 6). bothy migrate up writes it
// to instance.min_server_version; a server below it keeps running but never becomes ready
// (readiness check schema_requires_newer_server). By rule M1 of docs/operations/upgrade.md it is
// the previous release, so a rollback by one version works; a release that breaks on purpose
// sets its own version; before the first release it equals the current version
// (services/server/VERSION). Whoever changes the schema checks at release time whether it must
// rise. TestMinServerVersionNotAboveOwnVersion keeps it valid SemVer and not above the build.
const MinServerVersion = "0.0.0"
