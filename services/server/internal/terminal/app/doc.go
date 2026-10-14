// SPDX-License-Identifier: Apache-2.0

// Package app holds the use cases of the terminal module (M2-39): the five operations of the API
// group Hosts (listHosts, createHost, getHost, updateHost, setHostState), the end of sessions in
// the transaction that causes it (EndSessions, the one function every later end goes through), the
// implementation of authz.HostSource and the view of a host. Each use case authorizes with
// host.read or host.manage (docs/spec/05-actions.md section 4.14) before it reads anything
// else, runs its writes in one unit of work of kernel/txn declared as a rights change with the
// audit entries and host.updated, and never writes a pairing code, a key or a host name into
// an event payload other than host.updated, the audit or a log. The outgoing ports are Store (the
// three tables), PairingCodes (identity), People (identity) and Presence (the control connections
// of M2-43).
//
// Example:
//
//	svc, err := app.New(cfg, app.Deps{Store: repo, PairingCodes: tokens, People: people, ...})
//	issued, err := svc.Create(ctx, owner, terminal.CreateHost{Name: "Build server", Reason: "New machine"})
package app
