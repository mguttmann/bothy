// SPDX-License-Identifier: Apache-2.0

// Package domain holds the pure rules of the instance module: the names of the sequence counters
// and the phase in which each may be advanced or read (docs/spec/02-architektur.md "Sperrreihenfolge",
// 03 "Sperrreihenfolge"), and the comparison of server versions after SemVer 2.0.0 behind the
// start check of instance.min_server_version (docs/operations/upgrade.md "Schema-Prüfung beim
// Start"). It imports only kernel-core and no third-party library.
package domain
