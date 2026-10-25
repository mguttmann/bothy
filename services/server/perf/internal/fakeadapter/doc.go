// SPDX-License-Identifier: Apache-2.0

// Package fakeadapter is the fake adapter of the load tests of the agent path (brief M2-36
// design note 3, docs/spec/09-agents.md section 16.1 "Fake adapter (load)"): a Go client without
// SDK that follows the adapter contract for one agent. It holds one gateway connection as the
// consumer agentpath.ConsumerName, acknowledges cumulatively without ever filling the window of
// agent.max_inflight_events, checks every invocation token with the reference verifier of
// runs/adapters/runstest, reads the run context, starts the run with the delivered token and
// either finishes it (mode invocation) or patches it through run.update throttled to
// agent.run_update_min_interval (mode patch), and sends heartbeats with active_runs. It has no
// runtime and no model and writes no messages.
//
// The receive time of a frame is taken by the transport as soon as the frame was read, before the
// adapter decodes anything, so the measured interval of perf.invocation_delivery_p99 ends there.
// Every deviation from the contract (an error or close frame, a rejected token, a redelivered or
// second invocation, an unexpected HTTP status) is recorded and fails the report.
//
// The adapter logic works over the small interfaces of transport.go, so the unit tests use fakes;
// DialGateway and HarnessAPI bind them to the gateway test client wstest and the harness client.
//
// Example:
//
//	a, err := fakeadapter.New(cfg, fakeadapter.HarnessAPI{C: agent}, verifier)
//	err = a.Start(ctx, func(ctx context.Context) (fakeadapter.Conn, error) {
//		return fakeadapter.DialGateway(ctx, t, agent, publicURL, agentpath.ConsumerName)
//	})
//	defer a.Stop()
//	rep := a.Report()
package fakeadapter
