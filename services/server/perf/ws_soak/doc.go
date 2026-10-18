// SPDX-License-Identifier: Apache-2.0

// Package wssoak holds the load test of perf.ws_connections_per_node and perf.ws_idle_conn_mem_max (TestPerf_WSConnectionsPerNode)
// (ADR 0063, docs/spec/02-architektur.md "Leistungsziele"). The test runs only with the build tag
// perf, through make perf, never in make check (plan P-07); perf/README.md describes it.
package wssoak
