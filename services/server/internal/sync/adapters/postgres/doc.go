// SPDX-License-Identifier: Apache-2.0

// Package postgres is the PostgreSQL adapter of the sync module for the event log (M1-16): the
// Repository implements the outgoing ports Writer and Partitions of sync/app over the sqlc
// queries in queries/ (batch insert with pg_notify in one statement, head and minimum, the two SECURITY DEFINER
// partition functions), and Tail implements sync.Tail with its own LISTEN connection
// (docs/spec/02-architektur.md "Log-Tail je api-Prozess").
package postgres
