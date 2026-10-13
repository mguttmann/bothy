// SPDX-License-Identifier: Apache-2.0

// Package app holds the use cases of the instance module behind its ports: the sequence
// counters (instance.Counters, with the phase guard of docs/spec/02-architecture.md
// "Lock order") and the instance row (instance.State). The database is reached only through
// the outgoing port Repository, which adapters/postgres implements.
//
// Example (internal/wire):
//
//	repo := postgres.NewRepository(pool)
//	counters := app.NewCounters(repo)       // only for the phase hook constructors
//	state := app.NewState(repo, uow)
package app
