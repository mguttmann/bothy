// SPDX-License-Identifier: Apache-2.0

// Package agentpath holds everything of the load tests of the agent path (brief M2-36, targets
// perf.invocation_delivery_p99 and perf.run_patch_fanout_p99) that runs without a server: the
// named parameters of the load, the plan derived from the configuration defaults and the chapter
// tables, the invocation samples (commit of the triggering message to the receipt of
// agent.invocation at the fake adapter) and the patch version matcher (scheduled run.update to the
// first run.updated with that version at every focused viewer). It is untagged, so make test runs
// its unit tests and make check compiles, vets and lints it; the tagged tests perf/invocation and
// perf/run_patch only orchestrate.
//
// Example:
//
//	in, err := agentpath.Defaults()
//	plan, err := agentpath.NewPlan(in)
//	res := agentpath.InvocationSamples(triggers, invocations, offset, from, to)
//	v := agentpath.NewViewer(sentVersions, from, to)
//	v.Apply(agentpath.ViewerFrame{Received: t, Snapshot: true, Version: 3})
package agentpath
