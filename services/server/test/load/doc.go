// SPDX-License-Identifier: Apache-2.0

// Package load is the capacity load test of docs/spec/16-operations.md section 13 (brief M7-18):
// a seeded world of people in group spaces, an open-loop sender of messages through the public
// API and gateway connections that measure the fan-out, run as a ramp of steps against a
// running server.
//
// Like test/smoke it is a black box of the server: besides the standard library it imports only
// the generated api/rest, github.com/coder/websocket and the DPoP helpers of test/smoke
// (TestBlackBoxImports). The command cmd/bothy-load drives the full stack installed from the
// images (tools/load/capacity_run.py, make load-capacity); the capacity gate of make perf
// (perf/capacity) shares the scenario parameters of params.go and the goodput of SummarizeSends.
// tests/load/README.md describes the scenario, the parameters and the result file.
//
// Every threshold comes from the caller (Targets), read from the tables of the specification;
// the package holds no threshold as a literal.
package load
