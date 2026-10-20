// SPDX-License-Identifier: Apache-2.0

// Package app holds the use cases of the runs module (M2-05): the acceptance of a triggering
// message (Acceptor, the in-transaction subscriber of messaging.MessagePosted), the lifecycle of a
// run (Open and Transition, the compare-and-swap of 03 runs on state and version that the later
// tickets M2-08, M2-09, M2-11, M2-15 and M2-16 call), the reads of runs.Runs and the source of the
// gauge bothy.run.active. It depends only on the ports of chat, agents and messaging and on the
// kernel; the tables are reached through Repository (adapters/postgres).
//
// Example:
//
//	svc, err := app.New(cfg, app.Deps{Repo: repo, UoW: uow, Authorizer: az, Events: events, Messages: msgs,
//		Threads: threads, Channels: threads, Agents: agentFacts, Cursors: cursors})
package app
