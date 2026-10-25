// SPDX-License-Identifier: Apache-2.0

// Package runpatch holds the load test of perf.run_patch_fanout_p99, "run.update of the adapter
// to run.updated at the focused client" (TestPerf_RunPatchFanoutP99;
// docs/spec/02-architecture.md "Performance targets", ADR 0063 Confirmation, brief M2-36). The
// test runs only with the build tag perf, through make perf, never in make check (plan P-07);
// perf/README.md describes it. Its untagged parts live in perf/internal/agentpath,
// perf/internal/fakeadapter and perf/internal/agentworld.
package runpatch
