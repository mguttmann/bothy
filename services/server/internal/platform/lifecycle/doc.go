// SPDX-License-Identifier: Apache-2.0

// Package lifecycle runs the start and the graceful shutdown of a server process
// (docs/spec/02-architecture.md "Start and graceful shutdown", R-AR-16, R-AR-17).
//
// A Runner starts its components in registration order. When the start fails, a component
// reports a fatal runtime error (Fail), the context ends or the process receives SIGTERM or
// SIGINT, the Runner runs the stop steps registered so far in the fixed order of their Phase.
// Each step gets the smaller of its own budget and what is left of shutdown.total_timeout;
// when the total is spent, the remaining steps are skipped. A second signal ends the process at
// once with ExitForced.
//
// Roles differ only in the steps they register: the phase order yields the sequences of 02
// "Shutdown" for api, worker and all. Later tickets add their components and steps through
// Add and OnStop in internal/wire (README.md).
//
// Example:
//
//	r := lifecycle.New(lifecycle.Options{TotalTimeout: 60 * time.Second, Logger: log})
//	r.Add(pool)
//	r.OnStop(lifecycle.StopStep{Name: "pool", Phase: lifecycle.PhaseDatabase, Run: closePool})
//	os.Exit(r.Run(ctx))
package lifecycle
