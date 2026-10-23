# Module `agents`

Status: agent records, backend policy, credential holder and tool class built (M1-51); consumer cursors with fencing, the op `ack` and the REST consumer operations built (M2-07); presence and heartbeat built (M2-20); the read of persisted heartbeats for the stall check built (M2-09); agent sessions per thread built (M2-12); dev environments follow with the tickets of the milestone.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Agents, backend policy and tool class (configured), reported state, consumer cursors with fencing, command register, agent sessions, dev environments (milestone).

## Code

- `internal/agents` (`agents.go`): the port. `Agents` (M1-51) with `List`, `Get`, `Create`,
  `Update`, `Disable`, `Enable`, `SetBillingMode`, `SetBillingModeAll`, `SetCredentialHolder` and
  `SetToolClass`; the DTOs `Agent` (its field `Presence` is a `PresenceStatus`), `AgentPage`, `Page`, `Precondition` (the parsed
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
  `wire.NewAgentPolicySource` into the Authorizer and the rights hooks) and the
  `authz.AgentDirectory` of the permission hints (`AgentIDs`, every agent ascending, M6-07).
- Migration `migrations/00022_agents.sql`: the table `agents` as 03 section 4.4 states it, with the
  trigger `agents_kind_check` (an agents row only for a principal of kind `agent`, SQLSTATE
  `23514`). `bothy_app` may read, insert and update, not delete.

- Restore cursors (M7-04): the port `RestoreCursors` (`restore.go`) with `MoveCursorsTo(ctx,
  headSeq)`, implemented by `app.NewRestoreCursors` over the outgoing port `CursorMoveStore`
  (`adapters/postgres`, query `MoveAllConsumers`): it sets `agent_consumer_cursors.acked_seq` of every row to the
  head of the restored log inside the caller's unit of work and refuses to run outside one.
  `bothy restore finalize` calls it through `internal/wire` (`RestoreCursors`) in the transaction
  that sets the new sync epoch (docs/spec/03-data-model.md 6.7 item 1).

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
  `rl.agent_runtime`, no audit of allows): an owner who is a person gets `403 forbidden` with reason
  `subject_kind_not_allowed`, any other person or app `404` (visibility first). `Take` creates the
  row at the given head or writes the holder; `Save` and `Restart` are fenced by the holder and
  answer `ErrConsumerReplaced` without a row; `State` never creates a row (`404` for a consumer
  that never connected); `Poll` delegates to the feed and answers `410 resync_required` with the
  reason after a move to the head; `Ack` checks against the head of the serving process, writes
  `GREATEST` without holder condition (idempotency-exempt) and tells the feed.
- Table `agent_consumer_cursors` (migration `00045_agent_consumer_cursors.sql`, retention class
  `security`, `ON DELETE CASCADE` with the agent); queries `TakeConsumer`, `SaveConsumerAck`,
  `RestartConsumer`, `AckConsumer`, `GetConsumer` in `adapters/postgres/queries/consumer_cursors.sql`,
  run on the pool or the ambient transaction (single-row statements, no event).
- `adapters/httpapi/consumers.go`: `getConsumerState`, `pollAgentEvents`, `ackAgentEvents`.
  The poll reads through `ConsumerFeed`, the delivery engine of the gateway, so every
  invocation-bearing event of a page passes `runs.Delivery` there with the same rules as on the
  WebSocket (M2-08): an allowed one carries a fresh `invocation_token`, a refused one is not in
  `events` and `next_cursor` covers it. `agents` imports nothing of `runs`, so no source port
  `DeliverySource` is needed (D-M2-08-13).
- Configuration keys: `agent.ack_timeout`, `agent.ack_persist_interval`,
  `agent.max_inflight_events` (bound by the gateway engine).
- Rulings: docs/DECISIONS.md M2-07.

## Sessions (M2-12)

- Port (`sessions.go`): `Sessions` with six methods. For people and adapters through
  `adapters/httpapi` (`AgentSessions`): `List` (`listAgentSessions`, `agent_session.read`, every
  row of the thread newest first by `created_at`, then `id`, unpaginated), `Reset`
  (`resetAgentSession`, `agent_session.reset`), `Bind` (`bindAgentSession`, `agent_session.bind`)
  and `Report` (`reportAgentSession`, `agent_session.report`, agent-self). For `runs`, in the
  caller's unit of work and without authorization: `ForInvocation(ctx, InvocationSessionQuery)`
  in the transaction that appends `agent.invocation` of an operator run (returns the session row
  or none; `runs/domain` decides `session_mode` from it), and `AttachOnStart(ctx,
  AttachOnStartCmd)` at `queued -> running` of `run.start` (returns the ID of the attached row, zero for none). `CheckStartModel(model)` validates the `model` of `startRun` and `resumeRun` before the transition stores it (D-M2-12-20).
  DTO `Session` (every column `AgentSession` shows plus `CostBaselineMicros`), `SessionJSON` for
  the payload of `agent_session.changed`, the end reasons, `bind_failed` and the conflict reasons
  as constants.
- Declared source `RunSource` with `ActiveRun(ctx, threadID)`: the run of the thread in `queued`,
  `running` or `waiting_input`, its row locked `FOR UPDATE`. `runs` implements it in its postgres
  adapter and `internal/wire` wires it; `agents` never imports `runs`.
- In-Tx event `SessionAttached{ThreadID, RunID, AgentID, SessionID, ReplacedSessionID,
  BindFailed}`: a report created the run's row, or `bind_failed` replaced it. The subscriber
  `runs.OnSessionAttached` sets `runs.agent_session_id` and, with `BindFailed`, writes the system
  line `agent_session.resume_failed` with the run's agent as author through
  `messaging.Messages.AppendSystemLine` (it never reopens a closed thread, R-DM-42).
- Rules: a thread has at most one current row and an external session (`agent_id`,
  `external_session_ref`) is current in at most one thread (the two partial unique indexes of
  03; every insert uses `ON CONFLICT DO NOTHING` and maps the outcome, so no unique violation reaches
  a client as `500`). End reasons `reset` (reset), `rebind` (bind, take-over, `bind_failed`), `import`
  (import rows, written by the importer) and `agent_changed` (the next invocation after an
  operator change). Reset without a current row is 409 `conflict` with `reason` `no_current_session`; a reset or bind
  during a run leaves the run on its row. Bind is a compare-and-swap over
  `expected_current_session_id` (`session_changed`) and needs `take_over` for a reference current
  elsewhere (`session_bound_elsewhere`); authz checks that thread as the second target. Report is a
  compare-and-swap over `version` with `If-Match` (absent: the run has no row yet and the report
  creates it); `started` only moves forward; `bind_failed` needs a started row
  (`session_not_bound`).
- Import binding: at the first trigger of a thread that has no row other than import rows, the
  newest import row of the operator is copied into a current row with `started = true`, its
  model, cost baseline and currency, audited `agent_session.bind` with `cause` `import`; of two
  concurrent first triggers exactly one binds. Fail open: when the runtime no longer knows the
  session, the report with `bind_failed: session_missing` ends the bound row with `rebind`,
  starts a new row (created already ended when a reset or bind ended the bound row during the
  run) and leads to exactly one system line; the run continues.
- Events and audit: `agent_session.changed` for every committed change (ended rows first in
  ascending `id`, then the new row; resource `thread` without version, receive action
  `thread.read`). One allow audit entry per bind (explicit or import) with IDs and codes only,
  never the reference; reset, read and report write none.
- `domain/session.go`: the end transition, the report rules, the import-binding candidate, the
  cost baseline and the checks of `external_session_ref` (`agent.session_ref_max_bytes`,
  `textcheck.DisplayText`) and `model`, all pure.
- Data owned: the table `agent_sessions` (migration `00064_agents_agent_sessions.sql`, also the FK
  of `runs.agent_session_id`, validated by `00070`); queries in
  `adapters/postgres/queries/agent_sessions.sql`, the repository in `adapters/postgres/sessions.go`.
  Locks: run row (through `RunSource` or in `runs`) before session rows, session rows in ascending
  `id` (03 6.3).
- Configuration key: `agent.session_ref_max_bytes`.
- Test helper `adapters/agentstest`: `InsertImportedSession` writes an import row as 22 section 9
  step 1 describes, until the importer of M8 exists; only tests import it.
- Rulings: docs/decisions.d/M2-12.md.

## Presence and heartbeat (M2-20)

- Port (`presence.go`): `Presence` with `Heartbeat(ctx, body)` (the JSON object of schema
  `Heartbeat`, the REST body of `sendHeartbeat` or the member `agent` of the gateway op
  `heartbeat`, so both transports share one decoder; the actor comes from the context) and
  `Statuses(ctx, agentIDs)` (tracked agents from memory, the others from their row; it does not
  authorize, callers pass only agents the actor may read). DTOs `PresenceStatus` (schema
  `AgentRuntimeStatus`; empty values mean "not reported") and `BillingEvidence`, the state
  constants (`unknown`, `online`, `degraded`, `offline`) and the warning codes
  (`billing_mode_mismatch`, `billing_mode_unsupported`, `billing_evidence_unknown`,
  `tool_class_mismatch`). `PresenceStatus` replaces the earlier struct `Presence`, whose name is
  now the port.
- `domain/heartbeat.go`: `DecodeHeartbeat`, the only mapping of the wire form `agent_heartbeat.v1`
  (09 section 4.8): closed top-level members and `details` keys, the forms of names, versions and
  codes, no duplicates, `protocol_version` a positive integer, `rate_limit` refused with `not_enabled`
  until M6-14; it checks the openapi patterns itself, so both transports refuse the same bodies.
  `ReportedChange(stored, next)` decides a change of the reported mode or class and builds the
  audit details (`billing_mode_old`, `billing_mode_new`, `tool_class_old`, `tool_class_new`,
  `key_type`, `method` from `next.Evidence`); `Report.StoredDetails` and `ReportFromStore` are the
  form of `agents.reported_details`.
- `domain/presence.go`: `StateAt` (offline at and after last heartbeat plus
  `agent.presence_timeout`), `StartRule`, `Assess` with the difference kinds (a) reported mode
  unequal, (b) configured mode not supported, (c) `key_type` unknown, and `Warnings`
  (`billing_mode_mismatch` only after `billing.mismatch_alert_after`, the others at once, nothing
  while `offline` or `unknown`).
- `app/presence.go`: `Tracker`, the process-wide presence in the roles `api` and `all`: one entry
  per agent with its lock, the last report, the state, a deadline timer, the last published pair
  and the start times of a difference. Timers run on the port `app.Timers` (`RealTimers` over
  `time.AfterFunc`, a fake in tests); the publisher (`kernel/eventlog.EphemeralPublisher`, the
  hub) and the readiness port of `chat` are bound late (`BindPublisher`, `BindReadiness`).
  `NewTracker` refuses `agent.presence_timeout` not above `sync.heartbeat_interval`.
- `app/heartbeat.go`: the use case `Heartbeat`: `authorize` `agent.heartbeat` on the actor's own
  agent (the owner gets `forbidden` `subject_kind_not_allowed`, any other person or app
  `not_found`, D-M2-20-0), decode (one
  `validation_failed` with every field error), under the entry lock read `GetPresenceRow` on every
  heartbeat (a configured value changed in another process counts at once), project readiness
  only when the list changed since this process applied it (`ApplyProjectReadiness` of `chat` in
  its own unit of work), then the change path, the entry update, the reassessment, the re-armed
  deadline and the event. The rate limit `rl.agent_runtime` runs before, in the chain or the
  gateway.
- State rules: `unknown` before the first heartbeat, `online` or `degraded` from the latest one,
  `offline` once the deadline (last heartbeat plus `agent.presence_timeout`, inclusive) fires
  without a newer heartbeat. Only principals of kind agent have presence.
- Persistence: a change of `reported_billing_mode` or `reported_tool_class` (also from or to "not
  reported", also the first report) runs one unit of work: `LockPresenceRow`, compare again,
  `WriteReportedChange` (mode and class at once with the whole reported state and
  `last_heartbeat_at` from the clock) and the audit entry `agent.heartbeat` `allow` in the same
  transaction; equal after the lock writes nothing. Only this path writes the two audited
  columns. Every other heartbeat marks the entry dirty; the flush loop (`app/presence_flush.go`)
  writes the dirty entries at most once per `agent.heartbeat_persist_interval`, ascending agent
  id, in one unit of work, with the monotone `PersistHeartbeat` (`last_heartbeat_at IS NULL OR
  last_heartbeat_at <= $hb`), so a fresher heartbeat stored by another process is never
  overwritten. No `agents.version`, no `agent.updated`, no `permission_version`, no
  `access.changed`, no `event_log` row, no `NOTIFY`.
- Start (`Tracker.Start`, a component of the start sequence): `ListLivePresence` loads the agents
  stored `online` or `degraded`; a missing `last_heartbeat_at` or one older than
  `agent.presence_timeout` plus `agent.heartbeat_persist_interval` is `offline` at once (stored by
  the first flush), the others keep their state until the start plus `agent.presence_timeout`.
  Agents whose principal is not active are loaded inactive (no metric series, their deadline
  runs). Agents not loaded get an entry with their first heartbeat.
- Shutdown (`Tracker.Stop`, stop step `agent_presence` in the phase of the connections after the
  gateway's step): stops the timers and the flush loop without publishing and flushes every dirty
  entry within the rest of `shutdown.total_timeout`; a restart is not an agent going offline.
- Events: the ephemeral `agent.presence_changed {agent_id, state, reported_billing_mode}` when
  state or reported mode differ from the last published pair (initially the stored values), empty
  scope, receive action `agent.read`, actor the agent for a heartbeat and none for the timer; the
  last reported mode stays while `offline`. A publish error is logged and never fails the
  heartbeat.
- Configured values changed: after the commit of `SetBillingMode`, `SetBillingModeAll`,
  `SetToolClass`, `Disable` and `Enable` the service calls `Tracker.Changed`, which reads
  `GetPresenceRows` for the tracked agents and reassesses; `Disable` makes the entry inactive (no
  series, the deadline keeps running), `Enable` active again. No event: `agent.updated` is
  already written. `getAgent` and `listAgents` show the tracker's presence.
- Metrics: `Tracker.Samples()` (memory only, never blocking) feeds the observable gauges of
  `adapters/otelmetrics` over the registry specs: `bothy.agent.heartbeat.age` per active tracked
  agent with a known heartbeat and `bothy.agent.billing_mode.mismatch_age` per active tracked
  agent (seconds since a difference of any kind began, 0 without one); the alarm
  `BothyBillingModeMismatch` reads the second.
- `adapters/postgres/queries/presence.sql`: `GetPresenceRow`, `LockPresenceRow` (`FOR UPDATE`),
  `GetPresenceRows`, `ListLivePresence`, `WriteReportedChange`, `PersistHeartbeat`; a transaction
  of the heartbeat locks at most one `agents` row and no row of another module (03 section 6.3).
- `adapters/httpapi/heartbeat.go`: `SendHeartbeat` (`POST /agents/me/heartbeat`) encodes the
  checked body again and hands it to `Presence.Heartbeat`; it answers `HeartbeatAck` with
  `interval_ms` = `sync.heartbeat_interval`.
- Configuration keys: `agent.presence_timeout`, `agent.heartbeat_persist_interval`,
  `billing.mismatch_alert_after`, `sync.heartbeat_interval` (`app.PresenceConfig`).
- Tests: `domain/heartbeat_test.go` and `domain/presence_test.go` (decoder, forms, change,
  boundary, start rule, assessment, fuzz seed corpus), `app/presence_test.go` (the tracker over
  fakes and the fake timers), `adapters/otelmetrics/presence_test.go` (the gauges with an
  in-process reader).
- Rulings: docs/DECISIONS.md M2-20.

## Run facts (M2-05)

- Port: `Facts` with `RunFacts(ctx, agentIDs)` and the DTO `RunFacts` (handle, status, billing
  mode, credential holder exactly for personal, target tool class) for the trigger decision and
  the run metrics of runs. It never authorizes and never opens a unit of work: it reads in the
  ambient transaction of ctx.
- `app/facts.go`: `Service` implements it (compile-time check `var _ agents.Facts`), with one
  read of the records (`Store.Agents`, query `GetAgents`, `principal_id = ANY` ascending, no
  lock) and one `identity.Directory.Principals` call; people, apps and unknown ids are absent.
- The effective capability `followups` (M2-11, 09 section 2.2, D-M2-11-12): `RunFacts` carries
  `FollowupsPermitted` (the owner's `operator_config.supports_followups`, nil when the key is
  absent, which allows; `OperatorConfig.Permission`) and `FollowupsReported` (the persisted
  `reported_details.capabilities.followups`, `domain.Reported.Followups`; "" when not reported).
  The heartbeat of M2-20 keeps the report current at least every
  `agent.heartbeat_persist_interval`; runs combines both. Test helper
  `agentstest.ReportCapabilities(tb, db, agentID, caps)` stores the reported capabilities.

## Persisted heartbeats (M2-09)

- Port (`heartbeats.go`): `Heartbeats` with `HeartbeatStates(ctx, agentIDs)`, the DTO
  `HeartbeatState {AgentID, LastHeartbeatAt, ActiveRunIDs, ActiveRunsReported}`. It reads only the
  stored columns `last_heartbeat_at` and `reported_details.active_runs`, ascending by id, unknown
  ids absent, in the ambient transaction of `ctx` when there is one; no authorization (the caller
  is the job `runs.stall_check` of `runs`). `ActiveRunsReported` is false when the stored heartbeat
  carried no `active_runs`, so no run counts as missing from it.
- `app/heartbeats.go`: `HeartbeatReader`, built with `NewHeartbeatReader(store PresenceStore)`
  over the postgres repository. It is separate from the `Tracker`, because the role `worker`
  starts no tracker and must see the same persisted state in every process (lagging the live
  heartbeat by up to `agent.heartbeat_persist_interval`). It reuses `GetPresenceRows` and the
  decoder of `reported_details` (`domain.ReportFromStore`); no new query.
- Test helper `adapters/agentstest`: `PersistHeartbeat(tb, db, agentID, at, activeRuns)` writes a
  heartbeat into the agents row as given (nil `activeRuns` stores none, an empty slice a reported
  empty list), for the integration tests of the stall check. Only tests import it;
  `TestAgentstest_NotInBinary` keeps it out of `cmd/bothy`.
- Tests: `app/heartbeats_test.go` (reader over the fake store), `adapters/agentstest/agentstest_test.go`
  (helper and reader against PostgreSQL through `pgtest`).

## The use cases (M1-51)

| Operation | Action | Rights change | No effect |
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
- The changes with "Rights change" = yes run in two units like the owner changes of identity: a first unit
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

### Switch to organization (M2-21)

`applySwitchToOrganization` (`app/switch.go`) is the one apply step of every switch to
`organization`: `SetBillingMode`, `SetBillingModeAll` and `SwitchToOrganization` write the same row
(holder NULL, `billing_mode_changed_at`, `billing_mode_changed_by`, version + 1) and the same
`agent.updated`; each caller keeps its own audit entry (one per agent for the single switch and the
guided flow, one with the list for `set_all`). `SwitchToOrganization` is the use case behind
`authz.AgentPolicySwitch` (`adapters/authzsource.Switch`): only authz calls it, after authorizing
every agent, inside the unit of work of the outer action; it locks the rows in ascending id, gives
an agent already on `organization` an entry with `no_change`, switches disabled agents too and
reads no If-Match. The reasons of `BillingModeSet`, `BillingModeSetAll` and `CredentialHolderSet`
have at most `audit.reason_max_chars` characters (`Config.ReasonMaxChars`). The tracker learns a
mode changed by the guided flow with the agent's next heartbeat; `billing_mode_matches` in
`getAgent` and `agent.updated` compares the reported mode with the desired mode of that read
(docs/DECISIONS.md D-M2-21-7).

### Tests

`domain/domain_test.go` (operator config, policy transitions, tool class, presence),
`app/facts_test.go` (`TestRunFacts`), `app/service_test.go` (the use cases on the fake-source harness of authz, AZ-M22 with Assistent
serving Research and Notizen), `adapters/postgres/repository_test.go`,
`adapters/authzsource/source_test.go` (`TestAgentPolicySourceContract` against the authz fake and
this source), `adapters/httpapi/*_test.go` over PostgreSQL 18 with the full chain and response
validation against openapi.yaml (BR-01, BR-03, BR-08, AZ-M22, the owner, step-up and audit table,
If-Match, tokens, visibility, badge) and `migrations/agents_test.go`.

## Slash command register (M2-16)

`Commands` (`commands.go`, implemented by `app/commands.go`, `CommandService`) is the register of
09 section 10 over the table `commands` (migration `00057`, queries `adapters/postgres/queries/commands.sql`).
The handlers of `registerCommand`, `updateCommand`, `removeCommand`, `getCommand` and
`listChannelCommands` are `adapters/httpapi/commands.go`.

- `domain/command.go` holds the pure rules: the built-in names of 09 10.1 (a test reads the
  chapter), the definition and option rules with their field codes (detail never repeats a
  value), scope coverage and the shadowing of `MostSpecific`.
- `Register`: rules, then `authorize(command.register)` with the command facts of the submitted
  definition (the target rule of `authz`), then the reserved names (`409` with
  `command_name_reserved`), the lock of the owner's `principals` row (`identity.PrincipalWriter.LockPrincipal`)
  and the count against `command.max_per_principal` (`400 validation_failed`, field code `limit_reached`), the insert (`409` without
  reason on a taken name) and in the same unit one audit entry and one `command.changed`.
- `Update`: rules, load, `command.read` on the stored command, `command.register` on the new
  definition, `If-Match`, reserved names, a no-op for an identical definition, otherwise a
  compare-and-swap on `version`. `Remove` sets `removed_at`; a removed command answers `204`
  without a write. The audit details carry the change, the scope, `control`, `required_level` and
  the version, never name, description or options (SEC-AUD-5).
- `command.changed` has the scope of the scope target (none, the space, or space and channel),
  the resource `command` with its new version and the receive action `command.read`.
- `ListForChannel` authorizes `command.read` on the channel, reads the live commands whose scope
  covers it, keeps the most specific per owner and name, asks `command.read` per owner in filter
  mode with the offered channel (09 10.3) and orders the channel's effective operator first.
  Built-in commands are not part of it (the client list, 09 10.1).
- `Resolve` gives the execution of `runs` the usable command without authorizing.
- The owner kind is read from `agents`: an owner with an agents row is an agent, any other an app
  (apps own commands from M2-25).

## Port (root package `internal/agents`)

- Inbound: `Agents` (administration, policy, tool class; M1-51), `Facts` (run facts of runs; M2-05), `Presence` (heartbeat and presence; M2-20), `Consumers` (cursor, ack, takeover with fencing), `Commands` (register), `Sessions`, `DevEnvironments` (milestone).
- Declares the source ports `ConsumerFeed` (M2-07) and `RunSource` (M2-12).
- Emits in-tx events: `AgentDisabled`, `SessionAttached` (M2-12).
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: `authz.AgentPolicySource` (policy, tool class, readiness of the dev environment; M1-51), kernel `personaldata.Participant`.

## Dependencies

May use the ports of these modules: `identity`, `authz`, `chat`.

Reason: create the agent principal, the credential holder is a person; implements `AgentPolicySource`; thread for sessions, where commands apply.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`agents`, `agent_consumer_cursors`, `commands`, `agent_sessions`, `dev_environments`.

Only this module writes and reads these tables by SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`agent.read`, `agent.create`, `agent.update`, `agent.disable`, `agent.enable`, `agent.billing_mode.set`, `agent.billing_mode.set_all`, `agent.credential_holder.set`, `agent.tool_class.set`, `agent.heartbeat`, `agent.events.consume`, `agent.triggerers.read`, `agent_session.read`, `agent_session.reset`, `agent_session.bind`, `agent_session.report`, `command.read`, `command.register`, `command.remove`, `dev_environment.read`, `dev_environment.manage`, `dev_environment.report`.

## Events and WebSocket ops

- Durable: `agent_session.changed`, `command.changed`, `agent.updated`.
- Ephemeral: `agent.presence_changed`.
- WebSocket ops (client to server): `ack`.

## Notes for the build

- `acked_seq` is stored at the latest after every `agent.ack_persist_interval` (04, 15.2); with several nodes, with fencing (02, section 16).
- Several api nodes (M7-17): the fencing is this module's guarded `Save` (`ErrConsumerReplaced`
  when the row holds another session); the gateway sends a takeover signal over
  `NOTIFY ephemeral`, and the other nodes call `Save` for their connections of that agent at once
  and close with `4015` on `ErrConsumerReplaced` (the gateway README, "Several api nodes"). Nothing
  in this module changed for it.
- Rebinding an agent session is a compare-and-swap over the current row (`expected_current_session_id`); `agent_session.report` is the compare-and-swap over `agent_sessions.version` (M2-12).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [09-agents.md](../../../../docs/spec/09-agents.md), [10-backend-policy.md](../../../../docs/spec/10-backend-policy.md)
