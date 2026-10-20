# Modul `agents`

Status: agent records, backend policy, credential holder and tool class built (M1-51); consumer cursors with fencing, the op `ack` and the REST consumer operations built (M2-07); presence and heartbeat (M2-20), commands, agent sessions and dev environments follow with the tickets of the etappe.
Normativ ist [docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Agenten, Backend-Richtlinie und Werkzeugklasse (Soll), gemeldeter Ist-Zustand, Verbraucher-Cursor mit Fencing, Befehlsregister, Agenten-Sitzungen, Dev-Umgebungen (Etappe).

## Code

- `internal/agents` (`agents.go`): the port. `Agents` (M1-51) with `List`, `Get`, `Create`,
  `Update`, `Disable`, `Enable`, `SetBillingMode`, `SetBillingModeAll`, `SetCredentialHolder` and
  `SetToolClass`; the DTOs `Agent`, `Presence`, `AgentPage`, `Page`, `Precondition` (the parsed
  If-Match) and one command per body; the in-transaction event `AgentDisabled{AgentID}`;
  `Register` declares the durable event `agent.updated`.
- `domain/`: the record `Agent` of 03 section 4.4 (every change bumps `Version`), the backend
  policy `Policy` with `To` (10 section 9: organization always and without holder, personal only
  with a holder, the same target is no change), `ToolClass` with the order `chat < scoped < host`
  and `EffectiveToolClass` (the higher of target and reported class, 07 section 2.11), the schema
  `agent_operator_config.v1` (`ParseOperatorConfig`, `OperatorConfigFromStore`, `Allows`; 09
  section 2.2), the presence view and the checks of `adapter_kind`, description and reason.
- `app/`: `Service`, one use case per operation (load, `authorize` with the action of 05 section
  4.10, unit of work, domain, repository, `agent.updated`, audit), and the outgoing ports `Store`,
  `Cursors` and `TokenRevoker`. `Config` binds `api.page_size_default` and `api.page_size_max`
  (`wire.AgentsConfig`).
- `adapters/postgres/`: `queries/agents.sql`, the sqlc output in `sqlcgen/` (entry in
  `services/server/sqlc.yaml`) and `Repository` (implements `app.Store`). Locks and writes run
  only inside a unit of work. Error mapping: the kind trigger and the checks and foreign keys of
  `agents` become `validation_failed` with the field; no error carries SQL text, constraint names
  or values; a stored JSON value outside its schema is an internal error.
- `adapters/httpapi/`: `Agents`, the handlers of `listAgents`, `createAgent`, `getAgent`,
  `updateAgent`, `disableAgent`, `enableAgent`, `setAgentBillingMode`, `setAllAgentsBillingMode`,
  `setCredentialHolder` and `setAgentToolClass`; they set `ETag` from `version` where the
  contract declares it, and `Cursors` (the list cursors of `platform/httpserver`).
- `adapters/authzsource/`: `Source`, the `authz.AgentPolicySource` of the process (wired by
  `wire.NewAgentPolicySource` into the Authorizer and the rights hooks).
- Migration `migrations/00022_agents.sql`: the table `agents` as 03 section 4.4 states it, with the
  trigger `agents_kind_check` (an agents row only for a principal of kind `agent`, SQLSTATE
  `23514`). `bothy_app` may read, insert and update, not delete.

- Restore cursors (M7-04): the port `RestoreCursors` (`restore.go`) with `MoveCursorsTo(ctx,
  headSeq)`, implemented by `app.NewRestoreCursors` over the outgoing port `CursorMoveStore`
  (`adapters/postgres`, query `MoveAllConsumers`): it sets `agent_consumer_cursors.acked_seq` of every row to the
  head of the restored log inside the caller's unit of work and refuses to run outside one.
  `bothy restore finalize` calls it through `internal/wire` (`RestoreCursors`) in the transaction
  that sets the new sync epoch (docs/spec/03-datenmodell.md 6.7 item 1).

## Consumers (M2-07)

- Port: `Consumers` (`Take`, `AcceptAck`, `Save`, `Restart`; the gateway reaches
  `agent_consumer_cursors` only through it), `ConsumerAPI` (`State`, `Poll`, `Ack` of the three
  REST operations), the declared source port `ConsumerFeed` (`Poll`, `Acked`, `Head`), which the
  delivery engine of the gateway implements, `ConsumerKey`, `ConsumerState`, `ConsumerView`,
  `FeedPage` and `ErrConsumerReplaced` (kind conflict). `Register` declares the op `ack`.
- `domain/consumer.go`: `DefaultConsumer`, `ValidConsumerName` (the pattern of `ConsumerPath`),
  `CheckAck` (below 0 or above the head is `validation_failed` on `/seq`, at or below the
  acknowledgement no change).
- `app/consumers.go`: `ConsumerService` implements `Consumers` and `ConsumerAPI`. Every call of an
  actor authorizes `agent.events.consume` on the agent itself (agent-self, class
  `rl.agent_runtime`, no audit of allows): an owner who is a person gets `403
  subject_kind_not_allowed`, any other person or app `404` (visibility first). `Take` creates the
  row at the given head or writes the holder; `Save` and `Restart` are fenced by the holder and
  answer `ErrConsumerReplaced` without a row; `State` never creates a row (`404` for a consumer
  that never connected); `Poll` delegates to the feed and answers `410 resync_required` with the
  reason after a move to the head; `Ack` checks against the head of the serving process, writes
  `GREATEST` without holder condition (idempotency-exempt) and tells the feed.
- Table `agent_consumer_cursors` (migration `00046_agent_consumer_cursors.sql`, retention class
  `security`, `ON DELETE CASCADE` with the agent); queries `TakeConsumer`, `SaveConsumerAck`,
  `RestartConsumer`, `AckConsumer`, `GetConsumer` in `adapters/postgres/queries/consumer_cursors.sql`,
  run on the pool or the ambient transaction (single-row statements, no event).
- `adapters/httpapi/consumers.go`: `getConsumerState`, `pollAgentEvents`, `ackAgentEvents`.
- Configuration keys: `agent.ack_timeout`, `agent.ack_persist_interval`,
  `agent.max_inflight_events` (bound by the gateway engine).
- Rulings: docs/DECISIONS.md M2-07.

## The use cases (M1-51)

| Operation | Action | Rights change | Without effect |
|---|---|---|---|
| `listAgents` | `agent.read` per item (filter mode) | no | |
| `getAgent` | `agent.read` | no | |
| `createAgent` | `agent.create` on the instance | yes (the new agent) | |
| `updateAgent` | `agent.update` | no | no field changes |
| `disableAgent` | `agent.disable` | yes | already disabled |
| `enableAgent` | `agent.enable` | yes (I1 for the agent) | already active |
| `setAgentBillingMode` | `agent.billing_mode.set` | yes (I1) | same mode and holder |
| `setAllAgentsBillingMode` | `agent.billing_mode.set_all` on the instance | yes (I1 for all) | no agent differs |
| `setCredentialHolder` | `agent.credential_holder.set` | yes (I1) | same holder |
| `setAgentToolClass` | `agent.tool_class.set` | yes | same class |

- Every mutation is owner only with a fresh step-up (05 section 4.10); a personal token of an
  owner has no owner role (SR5), so the agent is invisible to it. Visibility follows 07 section
  4.3: the owner, the agent itself and a principal with `read` on a channel the agent serves;
  missing and invisible agents give the same `404`.
- The changes with "Rechte" = ja run in two units like the owner changes of identity: a first unit
  loads, authorizes, checks reason, If-Match and the body and records a call without effect
  (`no_change: true`, no version, no event, no rights version, 06 section 2); a second unit
  declared with `txn.DeclareRightsChange` locks the record, checks again and writes. The rights
  hook of authz raises `permission_version`, verifies I1 over every active personal agent and on
  a violation rolls everything back with `409 i1_violation` (problem body with `conflicts`); its
  deny entry is written in its own transaction (07 section 4.4).
- `createAgent` creates the principal (kind `agent`, status `active`) through
  `identity.PrincipalWriter` with the handle and display name rules of M1-26 and the record with
  `organization`, `chat`, `unknown`, version 1. A handle taken by any principal is `409 conflict`
  on `/handle`.
- `disableAgent` disables the principal (`disabled_at`), revokes every active API token of the
  agent through identity (`RevokeAllForPrincipal`), publishes `AgentDisabled` in the transaction
  and bumps the version. `enableAgent` does not restore tokens.
- `setAllAgentsBillingMode` locks the rows in ascending id (with `agent_ids` the listed ones,
  without them every personal agent, disabled ones included), switches every agent whose policy
  differs and writes one audit entry with `agent_ids`, `from_billing_modes`, `held_agent_ids`,
  `from_credential_holder_ids`, `to_billing_mode`, `to_credential_holder_id` and the reason.
- Audit details are ids and codes (10 section 14): old and new mode, old and new holder, old and
  new class, the changed fields, the number of revoked tokens and the reason; never names or
  values of the configuration.
- `agent.updated` (payload `AgentPayload` with the full agent, resource `agent` with its version,
  empty scope, receive action `agent.read`) follows every mutation including the creation, so the
  badge (`billing_mode`) reaches every client and the agent itself.
- `AgentPolicySource`: `PersonalAgents` lists the personal agents whose principal is active (a
  disabled agent cannot be triggered, so `agent.enable` checks it again); `AgentPolicy` reports
  mode, holder, target and reported tool class, authz reckons with the higher class;
  `DevEnvironmentReady` is false until V2.
- No credential, key or subscription value is stored or logged: `operator_config` has no key
  that could carry one (B10, I5).

Rulings: docs/DECISIONS.md M1-51.

### Tests

`domain/domain_test.go` (operator config, policy transitions, tool class, presence),
`app/service_test.go` (the use cases on the fake-source harness of authz, AZ-M22 with Assistent
serving Research and Notizen), `adapters/postgres/repository_test.go`,
`adapters/authzsource/source_test.go` (`TestAgentPolicySourceContract` against the authz fake and
this source), `adapters/httpapi/*_test.go` over PostgreSQL 18 with the full chain and response
validation against openapi.yaml (BR-01, BR-03, BR-08, AZ-M22, the owner, step-up and audit table,
If-Match, tokens, visibility, badge) and `migrations/agents_test.go`.

## Port (Wurzelpaket `internal/agents`)

- Eingehend: `Agents` (Verwaltung, Richtlinie, Werkzeugklasse; M1-51), `Presence`, `Consumers` (Cursor, Bestätigung, Übernahme mit Fencing), `Commands` (Register), `Sessions`, `DevEnvironments` (Etappe).
- Gibt In-Tx-Ereignisse aus: `AgentDisabled`.
- Abonniert In-Tx-Ereignisse: keine.
- Implementiert Quellen-Ports und Querschnitts-Ports: `authz.AgentPolicySource` (Richtlinie, Werkzeugklasse, Bereitschaft der Dev-Umgebung; M1-51), kernel `personaldata.Participant`.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `identity`, `authz`, `chat`.

Grund: Agent-Prinzipal anlegen, Zugangsdaten-Inhaber ist Mensch; implementiert `AgentPolicySource`; Thread für Sitzungen, Geltung von Befehlen.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`agents`, `agent_consumer_cursors`, `commands`, `agent_sessions`, `dev_environments`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`agent.read`, `agent.create`, `agent.update`, `agent.disable`, `agent.enable`, `agent.billing_mode.set`, `agent.billing_mode.set_all`, `agent.credential_holder.set`, `agent.tool_class.set`, `agent.heartbeat`, `agent.events.consume`, `agent.triggerers.read`, `agent_session.read`, `agent_session.reset`, `agent_session.bind`, `agent_session.report`, `command.read`, `command.register`, `command.remove`, `dev_environment.read`, `dev_environment.manage`, `dev_environment.report`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `agent_session.changed`, `command.changed`, `agent.updated`.
- Kurzlebig: `agent.presence_changed`.
- WebSocket-Ops (Client an Server): `ack`.

## Hinweise für den Bau

- Heartbeat-Zustand lebt im Speicher und wird höchstens je `agent.heartbeat_persist_interval` gespeichert (03, 4.4).
- `acked_seq` wird spätestens je `agent.ack_persist_interval` gespeichert (04, 15.2); bei mehreren Knoten mit Fencing (02, Abschnitt 16).
- Umbinden einer Agenten-Sitzung ist ein Compare-and-swap auf `agent_sessions.version`.

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [09-agenten.md](../../../../docs/spec/09-agenten.md), [10-backend-richtlinie.md](../../../../docs/spec/10-backend-richtlinie.md)
