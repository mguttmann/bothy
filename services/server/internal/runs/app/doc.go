// SPDX-License-Identifier: Apache-2.0

// Package app holds the use cases of the runs module (M2-05): the acceptance of a triggering
// message (Acceptor, the in-transaction subscriber of messaging.MessagePosted), the lifecycle of a
// run (Open and Transition, the compare-and-swap of 03 runs on state and version), the delivery
// with its invocation tokens, follow-ups, cards and interactions, commands, wake-ups, team runs,
// the reads of runs.Runs and the source of the gauge bothy.run.active. It depends on the ports of
// chat, agents, messaging, files, billing and instance and on the kernel; the tables are reached
// through Repository and the other store ports (adapters/postgres). The lock order is the one of
// the README section "Lock order".
//
// Example:
//
//	svc, err := app.New(cfg, app.Deps{Repo: repo, UoW: uow, Authorizer: az, Events: events, Messages: msgs,
//		Threads: threads, Channels: threads, Agents: agentFacts, Cursors: cursors})
package app
