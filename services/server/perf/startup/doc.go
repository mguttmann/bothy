// SPDX-License-Identifier: Apache-2.0

// Package startup holds the load test of perf.startup_ready_max and perf.idle_rss_max (TestPerf_StartupReadyAndIdleRSS)
// (ADR 0063, docs/spec/02-architektur.md "Leistungsziele"). The test runs only with the build tag
// perf, through make perf, never in make check (plan P-07); perf/README.md describes it.
package startup
