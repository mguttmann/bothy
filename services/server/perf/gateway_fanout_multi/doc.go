// SPDX-License-Identifier: Apache-2.0

// Package gatewayfanoutmulti holds the load test of perf.fanout_cross_node_p99
// (TestPerf_FanoutCrossNodeP99, M7-17; ADR 0063, docs/spec/02-architecture.md "Performance
// targets"): two server processes with cluster.mode multi on one database behind an in-process
// edge, senders on the first, receivers on the second. The test runs only with the build tag perf,
// through make perf, never in make check (plan P-07); perf/README.md describes it.
package gatewayfanoutmulti
