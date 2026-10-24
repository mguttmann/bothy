// SPDX-License-Identifier: Apache-2.0

// Package search holds the load test of the search (TestPerf_Search, brief M3-16): the p95 of
// GET /api/v1/search (type messages) against search.p95_target on a corpus of
// search.benchmark_messages messages, and the p95 of GET /api/v1/search/thread-titles against
// search.prefix_p95_target on the same world (docs/spec/14-search.md section 7, ADR 0032 and
// ADR 0063 Confirmation). The same run measures the rewrite of messages.search_vector per million
// messages, the share of the search indexes in the database and the time difference between a term
// only in channels the searcher cannot read and a term that occurs nowhere. The test runs only with
// the build tag perf, through make perf and make bench-search, never in make check (plan P-07);
// perf/README.md describes it. Its untagged parts live in perf/internal/searchbench and
// perf/internal/searchworld.
package search
