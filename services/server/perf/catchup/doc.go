// SPDX-License-Identifier: Apache-2.0

// Package catchup holds the load test of perf.catchup_page_p99 with a fractional and a large visible set (TestPerf_CatchupPageP99, TestPerf_CatchupPageP99LargeVisibleSet)
// (ADR 0063, docs/spec/02-architecture.md "Performance targets"). The test runs only with the build tag
// perf, through make perf, never in make check (plan P-07); perf/README.md describes it.
package catchup
