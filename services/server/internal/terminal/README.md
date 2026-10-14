# Module `terminal`

Status: host records (M2-39). The normative source is [docs/spec/24-terminal.md](../../../../docs/spec/24-terminal.md); where they differ, the chapter applies.

## Purpose

Hosts and pairing, declaration of the LLM login (I6), session users per person (ruling T12), terminal sessions, terminal session tokens, control connections of the host services, relay of the streams.

## Code

- `internal/terminal` (`hosts.go`): port `Hosts` with `List`, `Get`, `Create`, `Update`, `SetState` and the DTOs (`HostView` field by field like the schema `Host`, `CreateHost`, `UpdateHost`, `SetHostState`, `PairingCodeIssued` with the pairing code as a redacting `identity.Token`); `Register` registers `host.updated`.
- `domain/`: `Transition` is the one place that knows the life cycle of 24, section 3.1 (M2-41 and M2-43 call it for their transitions); `CloseReasons` and `CloseDetails` are the closed lists of 24, section 10.3; `NewHostName`, `NewOSUser` and `CheckReason` check the texts (SEC-IN-4); `PlanAssignments` plans assignment and end of session users after T12 (exactly a reported name, uniqueness per host without case over all rows, at most one valid assignment per person, every `400` before any `409`); `reported.go` reads the reported columns.
- `app/`: the five use cases (`Service`), `EndSessions` (the one way of every end by the server), `HostSource` (implementation of `authz.HostSource`), the view `view.go`, the outgoing ports `Store`, `PairingCodes`, `People`, `Presence` and `Cursors`; `NoConnections` is the presence without control connections.
- `adapters/postgres/`: the sqlc queries in `queries/`, the generated code in `sqlcgen/` and `Repository`.
- `adapters/httpapi/`: the handlers of `listHosts`, `createHost`, `getHost`, `updateHost` and `setHostState`.
- Test code under `services/server/test/`: `terminaltest` creates hosts in every state with reported profiles, session users and check results as well as assignments and sessions directly in SQL (in production M2-41, M2-43 and M2-44 write these); `terminalworld` is the HTTP test world of M2-39.

## Port (root package `internal/terminal`)

- Inbound: `Hosts` (M2-39). `Sessions` and `Relay` come with M2-44.
- Emits in-tx events: none.
- Subscribes to in-tx events: from M2-46 kernel `authorize.AccessChanged`, `identity.DeviceRevoked`, `identity.SessionEnded` (end sessions).
- Implements source ports and cross-cutting ports: `authz.HostSource` (state, assignee, LLM declaration, server host of the host), bound in `internal/wire` for the authorizer, the rights hook and grants.

## Dependencies

May use the ports of these modules: `instance`, `identity`, `authz`, `agents`.

Reason: signer for terminal session tokens (purpose `terminal`); people, devices, owners, consumption of the step-up, pairing codes (`identity.Tokens.NewHostPairingCode`); grants on hosts and I6; readiness of the dev environment (milestone).

Additionally for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`hosts`, `host_session_users`, `terminal_sessions`.

Only this module writes and reads these tables via SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`host.read`, `host.manage`, `host.pair`, `host.connect`, `terminal.open`, `terminal.attach`, `terminal.stream`, `terminal.read_own`, `terminal.close`, `terminal.terminate`.

## Events and WebSocket ops

- Durable: `host.updated` (M2-39), `terminal_session.changed` (M2-44).
- Ephemeral: `host.presence_changed` (M2-43).
- WebSocket ops (client to server): none on the gateway; separate connections with their own ops (05, section 7).

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `terminal.sweep` | periodic | `maintenance` |

## Built in M2-39

- Reading a host for owners, the assignee and people with an effective grant terminal; administration
  only by owners with step-up, mandatory reason, audit and rights change. Agents, apps and
  personal tokens get `403 forbidden` with `subject_kind_not_allowed`.
- The event of a host goes to everyone who may read it on every change, never with a key or the
  pairing code.
- `EndSessions` writes the end of every session as a system action into the audit. The remaining
  actions stay deferred until M2-40 to M2-44.

## Extension points

- M2-40: guard rules SR10, SR11 with I6 and SR12, the I6 check at `grant.create` on a host and at `host.manage`, end of the sessions when the assignee changes.
- M2-41: pairing (`issueHostPairingCode`, `pairHost`, `confirmHost`) through `domain.Transition` and `app.PairingCodes`.
- M2-43: control connection; replaces the binding `NoConnections` of `app.Presence` with its register and writes the columns `reported_*` and `last_seen_at`.
- M2-44: sessions and relay; adds `terminal_session.changed`, the close code `4100` and `session.close` to the host service to `EndSessions`.
- M2-46: revocation, limits and metrics (`bothy.terminal.sessions.closed`), job `terminal.sweep`.

## Notes for the build

- Terminal content is never on the server, in telemetry, in the audit, in search, in push or in
  the context of an agent; the content runs end-to-end encrypted between device and host, the
  server is relay and authorization point (24-terminal.md, sections 1 and 5.1).
- The streams run over separate connections with their own ops, not over the gateway (05-actions.md,
  section 7); relay and host connections live only in the adapter `wsrelay`.
- The role `bothy_app` reads, inserts and updates the three tables and never deletes; an assignment
  stays after its end, so its name stays taken on the host.
- The name of a host is never in the audit, a log or a problem response (24, section 10.4; `TestHostCanary_SECT10`).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [24-terminal.md](../../../../docs/spec/24-terminal.md), [07-permissions.md](../../../../docs/spec/07-permissions.md), [ADR 0068](../../../../docs/adr/0068-terminal-through-a-host-service-with-noise-stream-and-invariant-i6.md)
