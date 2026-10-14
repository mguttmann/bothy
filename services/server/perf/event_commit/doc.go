// SPDX-License-Identifier: Apache-2.0

// Package eventcommit holds the load test of perf.event_commit_min_per_s (ADR 0063,
// docs/spec/02-architecture.md "Performance targets"): TestEventCommitThroughput in event_commit_test.go
// runs only with the build tag perf, through make perf, never in make check (plan P-07).
package eventcommit
