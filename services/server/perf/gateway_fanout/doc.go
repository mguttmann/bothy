// SPDX-License-Identifier: Apache-2.0

// Package gatewayfanout holds the load test of perf.fanout_local_p99 (TestPerf_FanoutLocalP99)
// (ADR 0063, docs/spec/02-architektur.md "Leistungsziele"). The test runs only with the build tag
// perf, through make perf, never in make check (plan P-07); perf/README.md describes it.
package gatewayfanout
