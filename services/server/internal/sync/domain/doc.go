// SPDX-License-Identifier: Apache-2.0

// Package domain holds the pure rules of the sync module: the validation of a durable event
// before it is buffered (docs/spec/04-sync-protocol.md "Envelope" and its payload
// rules, docs/spec/03-data-model.md table event_log), the arithmetic of the event_log partitions
// and the gap check of the log tail. It imports only kernel-core packages.
package domain
