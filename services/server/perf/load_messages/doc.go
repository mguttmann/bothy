// SPDX-License-Identifier: Apache-2.0

// Package loadmessages holds the load test of perf.message_create_p99 under perf.sustained_messages_per_s (TestPerf_MessageCreateP99)
// (ADR 0063, docs/spec/02-architektur.md "Leistungsziele"). The test runs only with the build tag
// perf, through make perf, never in make check (plan P-07); perf/README.md describes it.
package loadmessages
