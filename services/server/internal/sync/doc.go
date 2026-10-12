// SPDX-License-Identifier: Apache-2.0

// Package sync is the port of the sync module: the event log (append, tail, catch-up, redaction,
// partitions), the snapshot, idempotency keys, read states, drafts, personal settings and the
// cursors of the worker consumers. Other modules import only this package
// (docs/spec/02-architecture.md, "Layers within a module").
package sync
