// SPDX-License-Identifier: Apache-2.0

// Package pushhandoff holds everything of the load test perf/push_handoff (target
// perf.push_handoff_p95, brief M3-24) that runs without a server: the named parameters of the
// load, the plan derived from the configuration defaults and chapter 02, the strict decoding of a
// Fake-APNs request into a hint record, the samples from the commit of a mention to the 200 of
// Fake-APNs, and the matcher with the failure conditions. It is untagged, so make test runs its
// unit tests and make check compiles, vets and lints it; the tagged test only orchestrates.
//
// Example:
//
//	in, err := pushhandoff.Defaults()
//	plan, err := pushhandoff.NewPlan(in)
//	msgs, err := plan.Schedule(plan.Total())
//	res := pushhandoff.Check(pushhandoff.Run{Sends: sends, Requests: fake.Requests(), ...})
package pushhandoff
