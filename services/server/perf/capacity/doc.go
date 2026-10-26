// SPDX-License-Identifier: Apache-2.0

// Package capacity holds the capacity gate of make perf (brief M7-18, TestPerf_CapacitySteadyThroughput):
// a short steady run at perf.capacity_steady_messages_per_s in the scenario of the capacity load
// test (test/load, tests/load/README.md) that fails below perf.capacity_min_messages_per_s
// (docs/spec/02-architecture.md section 22, ADR 0063). The test runs only with the build tag perf,
// through make perf, never in make check (plan P-07); perf/README.md describes it.
package capacity
