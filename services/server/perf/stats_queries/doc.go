// SPDX-License-Identifier: Apache-2.0

// Package statsqueries holds the load fixture of the statistics queries (M6-05,
// TestPerf_StatsQueries): ten channels, ten agents and one million messages spread over
// stats.max_range, with runs, tool calls and costs. It measures the answer time of each statistics
// operation and records the query plans of the largest reads; there is no target in chapter 02 yet,
// so it only records (docs/observability/admin-statistics.md section 5 item 3 decides from these
// numbers whether stats_daily is needed). The test runs only with the build tag perf, through make
// perf, never in make check (plan P-07); perf/README.md describes it.
package statsqueries
