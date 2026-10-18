// SPDX-License-Identifier: Apache-2.0

// Package authorizecached holds the load test of perf.authorize_cached_p99 from BenchmarkAuthorizeCached (TestPerf_AuthorizeCachedP99)
// (ADR 0063, docs/spec/02-architecture.md "Performance targets"). The test runs only with the build tag
// perf, through make perf, never in make check (plan P-07); perf/README.md describes it.
package authorizecached
