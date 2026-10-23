// SPDX-License-Identifier: Apache-2.0

// Package pushhandoff holds the load test of perf.push_handoff_p95, "commit to handoff to the
// push service (fake)" (TestPerf_PushHandoffP95; docs/spec/02-architecture.md "Performance
// targets", docs/spec/13-push-notifications.md section 9, ADR 0051 Confirmation, brief M3-24).
// The test runs only with the build tag perf, through make perf, never in make check (plan P-07);
// perf/README.md describes it. Its untagged parts live in perf/internal/pushhandoff and
// perf/internal/pushworld.
package pushhandoff
