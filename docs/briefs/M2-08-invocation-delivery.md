# M2-08: Delivery with I2, guard rules and a fresh token

Status: ready | Depends on: M2-06, M2-07 | Milestone: M2

## Goal

A message of a human that triggers an agent (09 'Decision per message') is accepted in its own transaction: run `queued`, live-box anchor, `trigger_state = received` and the personal event `agent.invocation` without a token. Immediately before every hand-over of an invocation-bearing event (`agent.invocation`, `agent.followup`, `interaction.created`, `command.invoked`) to an agent, over the gateway or the REST poll, the port `runs.Delivery` checks `agent.invoke` for the triggering human with the permissions of that moment, including I2 (personal subscription only for its holder), SR2 (tool class `host` only for owners) and SR4 (only humans trigger). A denial delivers nothing, fails a queued run with a reason, sets `trigger_state = failed` and writes one audit entry; an allow issues a fresh EdDSA invocation token for exactly this delivery, records its `jti` where the run may start with it, and moves `trigger_state` from `received` to `queued` on the first successful delivery. A run nobody starts within `run.queued_max_age` fails with `agent_unavailable`. This is invariant I2 "before every invocation" (10, PLAN 18.2), R-SY-07 (a fresh token per delivery) and the server half of ADR 0023: no adapter ever receives an invocation that the current permissions would refuse.

## Read first

- `docs/PLAN.md`: section 8 "M2 Agent platform, extensions and terminal" (Goal, Scope, Out of scope, Exit criteria 1 to 3 and 6, Test strategy, Red team, Risks), the rows M2-02 to M2-07, M2-09, M2-11 to M2-17, M2-19, M2-20, M2-25, M2-26, M2-29, M2-31, M2-36, M3-07, V2-02 and M7-17 (who builds what next to this ticket), section 18.2 row "I2 before every invocation", "Way of working per ticket".
- Dependencies: M2-06 (token issuer) and M2-07 (agent consumers) are in the header; implied through them and therefore not in the header: M2-05 (runs, state machine, `trigger_state` mapping), M2-04 (trigger decision function), M2-02 (channel behaviour), M1-65 (gateway hub), M1-56 (`MessagePosted`), M1-54 (operators), M1-51 (agent records), M1-32 (signing keys), M1-28 (`authorize`), M1-24 (audit), M1-16 (event log), M1-14 (jobs), M1-13 (unit of work). M1-68 (metric declaration pattern) is in the closure as well, through M2-02. Not in the closure but used if merged: M2-03 (project binding, for AZ-M19).
- `docs/spec/09-agents.md`: 1 "Principles" (points 3 and 4), 2.4 "Tool class", 3.1 "Operator", 3.2 "Origin (`messages.origin`)", 3.4 "Decision per message", 3.5 "Reasons for rejection at acceptance", 3.6 "Guest agents", 3.7 "Auto-thread from a channel message", 4.2 "Transports", 4.3 "Connection and cursor", 4.4 "Events to the adapter", 4.5 "Invocation token" (claims, "Check in the adapter", "Keys and rotation"), 4.5.1 "Fresh token for a waiting run" (point 4 names the same checks as before a delivery), 5.1 "State machine", 5.2 "Reasons (`runs.state_reason`)", 5.4 "Live box content" (anchor), 5.5 "Stall, interruption, orphaned runs", 6 "Trigger state (`trigger_state`)", 7 "Follow-ups during a run", 15 "Reliability", 16.2 "Required scenarios" (AG-05, AG-07, AG-09, AG-27), 19 (R-AG-09, R-AG-23, R-AG-24), 21 "Configuration values of this chapter".
- `docs/spec/04-sync-protocol.md`: 2.2 "Envelope" (delivery fields `invocation_token`, `redelivered`; payload rules 1, 3 and 4), 7.1 "Cursor", 7.4 "REST variant", 8.1 "Basic rule", 8.3 "What the client receives and does on access removal" (point 5), 9 "Delivery guarantee" (point 3: `agent.invocation`, `agent.followup` and `command.invoked` carry no `resource.version`), 15.1 to 15.4 and 15.6, 16.5 "Agent run with follow-up", 17 (R-SY-07, R-SY-08, R-SY-10, R-SY-11, R-SY-17, R-SY-19).
- `docs/spec/05-actions.md`: 1 rules 1, 4, 6 (bullet "I2 is part of `agent.invoke`") and 8; 2.3 "Effective level" (point 1: owner permissions only in a session, with the exception of R-AZ-29); 3 "Rate-limit classes"; 4.10 rows `agent.invoke` and `run.token.renew`; 5 "Agent invocation `agent.invoke`" (points 1 to 8, the paragraph "Check without a request", claims, "Consequences on denial"); 6 rows `agent.invocation`, `agent.followup`, `interaction.created`, `command.invoked`, `run.state_changed`, `message.updated`; 8 (R-AZ-04, R-AZ-14, R-AZ-18, R-AZ-19, R-AZ-20, R-AZ-29); 9 test 6.
- `docs/spec/07-permissions.md`: 2.7 "Owner", 2.9 "Agents as principals", 2.11 "Tool class of an agent", 3 "Principal context", 4.1 "Decisions and reasons", 4.2 "Flow" (guards, filter mode, audit paragraph), 5 "SR1 Backend policy (I1, I2)", "SR2 Tool class", "SR3 Dev isolation" (V2), "SR4 Only humans trigger", "SR5 Administration only by humans with confirmation" (the exception of R-AZ-29), "SR7 Disabled means no permissions", 10 "Examples" (the test world), 11.1 rows AZ-M19, AZ-M20, AZ-M30, AZ-M51; 11.2 AZ-P10, AZ-P13.
- `docs/spec/10-backend-policy.md`: 3 "Invariants" (I2, I3), 4 "Overview of check points", 6 "Dispatch check (I2)", 9.2 "What happens when switching" (point 4), 14 "Audit trail" (row "I2 rejection"), 15 "Tests" (BR-04).
- `docs/spec/03-data-model.md` (core contract, read only): `runs` (`invocation_jti`, `renewed_from_jti`, `billing_mode_expected`, `state_reason`, `version`, `queued_at`, the partial unique index of R-DM-14, the transitions), `messages` (`origin`, `kind = 'run'`, `trigger_state`, `trigger_state_reason`, `silent`, checks), `threads` (`parent_message_id`, `operator_agent_id`), `agents` (`billing_mode`, `credential_holder_id`, `tool_class`, `reported_tool_class`), `event_log` (`audience_principal_id`, `receive_action`, `correlation_id`).
- `docs/spec/02-architecture.md`: 4.1 point 4 ("Source port", "In-tx event"), 4.3 "Ports, events and sources per module" (`runs`: `Delivery`), 4.4 "Allowed dependencies", 4.6 (`agent.invoke` belongs to `runs`), 4.7 (the note that `message.updated` for `trigger_state` is produced by `messaging`), 10 "Gateway" (last paragraph), 11.4 "Hub and delivery", 12.3 "Job catalogue".
- `docs/api/openapi.yaml`: operations `createMessage`, `createThread`, `pollAgentEvents`, `getJwks`; schemas `AgentInvocationPayload`, `AgentInvocationEvent`, `AgentFollowupPayload`, `AgentFollowupEvent`, `InteractionCreatedPayload`, `CommandInvokedPayload`, `RunCancelRequestedPayload` (reason `user_disabled`), `RunStateChangedPayload`, `RunSummary`, `Run`, `EventEnvelope` (`invocation_token`, `redelivered`), `EventPage` (`next_cursor`), `Message`, `TriggerState`, `PrincipalRef`.
- `docs/api/asyncapi.yaml`: messages `event.agent.invocation`, `event.agent.followup`, `event.interaction.created`, `event.command.invoked` (`x-receive-action: agent.events.consume`, `x-audience: self`).
- ADRs 0017, 0021, 0022, 0023, 0024, 0025, 0052, 0053, 0066, 0067.
- `docs/DECISIONS.md`: D-L22, D-L23, D-L24, D-AG-16, D-W-15, D-W-16, D-W-44, D-W-100; section 10 (format of the rows you add).
- `docs/OPEN-QUESTIONS.md`: OQ-01 (default: strict I1 without owner exception; the I2 tests assume it).
- `docs/security/threat-model.md`: T-AI-02, T-AI-07, T-AI-08, T-TK-05, T-AZ-03, T-AZ-04. `docs/RISKS.md`: RK-09, RK-13.
- `docs/observability/signals.md`: `bothy.agent.invocation.denied`, `bothy.run.start.latency`, `bothy.run.finished`, span `agent.dispatch`, 5.1 point 5 (fan-out traces link the request).
- `docs/operations/configuration.md`: `agent.invocation_token_ttl`, `agent.ack_timeout`, `agent.max_inflight_events`, `run.queued_max_age`, `auth.clock_leeway`, `sync.event_payload_max_bytes`.
- M1 briefs: M1-13 (unit of work, `txn.Subscribe`, `txn.Publish`, nested publish), M1-14 (`jobs.Inserter.InsertTx` with `InsertOptions.ScheduledAt`, `jobs.Register`), M1-16 (`eventlog.Event`, `eventlog.Appender`, `sync.DeliveryFields`), M1-24 (`auditing.Recorder.Record`, `auditing.Entry`, `audit_details.v1`), M1-25 (`authorize.Registry.Event`), M1-28 (Design notes "Public contract", `InvokeFacts`, "Guards" for `agent.invoke`, `Filter(ctx)`, `WithPermissionVersion`, `instance.Versions`, owner only with a session, snapshot cache key), M1-32 (`Keys.Signer` for purpose `invocation`), M1-51 (`AgentPolicySource`, `AgentPolicy`), M1-55 (`Threads.Create`, `Info`, `Get`, thread row lock in `AppendToThread`), M1-56 (`MessagePosted`), M1-64 and M1-65 (gateway test client, hub, `authorize.Delivery(ctx)`, which is a different thing from the port `runs.Delivery`), M1-10 (`internal/platform/otel`), M1-68 (metric declaration pattern).
- Code (merged names win, D-W-100): `services/server/architecture/modules.yaml` on `main` (`runs` may use `instance`, `chat`, `agents`, `messaging`, `billing`; `agents` may use `identity`, `authz`, `chat`; `gateway` may use `chat`, `agents`, `runs`, `sync`, `stats`), `internal/runs` (M2-04, M2-05, M2-06), `internal/agents` and `internal/gateway` (M2-07), `internal/messaging`, `internal/chat`, `internal/authz`, `internal/kernel/authorize`, `internal/kernel/actor`, `internal/kernel/auditing`, `internal/kernel/eventlog`, `internal/sync` (envelope), `internal/platform/jobs`, `internal/wire`.

## Fixed requirements that apply

Owner's fixed build rules (apply verbatim):

1. No MVP. New from scratch and directly in the final state; stages are only the order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface with a docstring/doc
   comment (purpose, parameters, return value, errors, example where useful); comments explain the
   why, not the what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in MkDocs).
4. Documentation in the repo: README per module/service, ADR for every architecture decision,
   CHANGELOG per release, keep the documentation site current. A change without matching
   documentation is not done.
5. Quality gates (SonarQube, linters, type checking, security scanners, test coverage) block the
   merge.

Rules for every brief:

6. Test first: write the failing test, run it and keep the red output for the report, then make it green, then clean up.
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, permission or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) is not edited by this ticket.

Product requirements implemented here:

11. Immediately before every delivery of `agent.invocation`, `agent.followup`, `interaction.created` or `command.invoked` to an agent, the server runs `authorize(triggering human, agent.invoke, ...)` with the permissions of that moment, including I2 (05 5 point 4), SR2 (05 5 point 5: the higher of `agents.tool_class` and `agents.reported_tool_class`) and SR4 (05 5 point 1) (04 15.3 step 2, 15.4 point 2; 07 SR1, SR2, SR4; 10 6; 02 10).
12. A denial before a delivery sends nothing (fail closed); a queued run becomes `failed` with a reason, the triggering message `trigger_state = failed` with `trigger_state_reason`, and one audit entry is written (04 15.3 step 2; 05 5 "Consequences on denial"; 10 4 and 14 row "I2 rejection"; AG-07, AZ-M51, BR-04).
13. Every delivery that passes, a redelivery included, carries a freshly issued invocation token (JWS, `alg = EdDSA`, `kid`, `typ = agent-invocation+jwt`, claims of 05 5) valid for `agent.invocation_token_ttl`, signed with the active key of purpose `invocation`, as the delivery field `invocation_token`; it is never stored in the event log (04 2.2, R-SY-07, R-SY-08, R-SY-17, ADR 0023, ADR 0052).
14. `runs.invocation_jti` is replaced by a delivery of `agent.invocation` and, while the run waits in `waiting_input`, by `agent.followup` and `interaction.created`; deliveries while `running` do not replace it; every replacement sets `runs.renewed_from_jti = NULL`; every delivery sets `runs.billing_mode_expected` to the current configured mode (03 `runs`, 04 15.3 steps 2 and 5, 09 5.1).
15. `trigger_state` moves from `received` to `queued` with the first successful delivery of `agent.invocation` (04 15.3 step 2, 09 6, R-AG-09).
16. Acceptance: a message of a human that triggers an agent creates in its own transaction the run `queued`, the live-box anchor (`kind = 'run'`), `trigger_state = received` and `agent.invocation` addressed to the agent without a token; with `auto_thread` in a main thread the run lives in a new thread `T'` with `parent_message_id = M`; a denial keeps the message without a run and sets `trigger_state = failed` with its reason (09 3.4, 3.5, 3.7, 5.4; 04 15.3 step 1; 05 5).
17. Only messages with `origin = 'human'` trigger; agents, apps, imports and system messages never do (05 5 point 1, 07 SR4, 09 3.2; AZ-M30).
18. A run that is still `queued` after `run.queued_max_age` fails with `agent_unavailable` and is never delivered afterwards (09 5.1, 5.2, 5.5, D-AG-16; AG-27).
19. Tool class per the core contract: SR2 compares the higher of `agents.tool_class` and `agents.reported_tool_class` (order `chat` < `scoped` < `host`); the token carries the configured `agents.tool_class` in the claim `tool_class` (05 5 point 5 and claims; 07 2.11; 09 2.4; AZ-M19, AZ-M20).
20. The four events go only to the agent they address (`audience`), with receive action `agent.events.consume` and without `resource.version`; payloads carry no message content, prompt, skills or project binding (05 6, 04 2.2 rules 1 and 4, 04 9 point 3, 09 4.4).
21. The denial counter `bothy.agent.invocation.denied` and the span `agent.dispatch` (signals.md).

## Scope

All paths under `services/server/` unless they start with `docs/`. No migration: the tables `runs` (M2-05), `messages` (M1-56), `agents` and `agent_consumer_cursors` (M1-51, M2-07) exist. No operation is enabled in `api/operations.yaml` by this ticket (`pollAgentEvents`, `getConsumerState`, `ackAgentEvents` come with M2-07). No new top-level Makefile target.

Create:

- `internal/runs/delivery.go` (port): the interface `Delivery` and its types (signature in Design note 1), `InvocationBearing(eventType string) bool`.
- `internal/runs/domain/delivery.go`: pure rules: the delivery matrix (event type x run state, Design note 2), the reason mapping (Design note 6), the claim assembly from facts (Design note 7), the queued-age test.
- `internal/runs/app/delivery.go`: the use case `Prepare` and the table of trigger resolvers per event type (Design note 5) with the entries for `agent.invocation` (trigger kind `message`) and `agent.followup`.
- `internal/runs/app/acceptance.go`: the subscriber `OnMessagePosted` (Design note 10). If the merged code of M2-05 already contains an acceptance subscriber, a run creation function or the anchor call, extend those instead of adding second ones and list it in the report.
- `internal/runs/app/queued_expiry.go`: the job `runs.queued_expiry` (Design note 11).
- `internal/runs/adapters/postgres/queries/delivery.sql`: lock of the run row, the update of `invocation_jti`, `renewed_from_jti`, `billing_mode_expected`, `version`; read of the active run of a thread for acceptance (if M2-05 has none).
- `internal/runs/app/metrics.go` (outbound port with a no-op default) and `internal/runs/adapters/otelmetrics/metrics.go` (Design note 15).
- Event payload builders in `internal/runs/domain` for `agent.invocation` and `agent.followup` (the latter is called by M2-11).

Change:

- `internal/kernel/actor`: the re-check marker on the principal type (Design note 4), with doc comment.
- `internal/kernel/authorize`: `InvokeFacts.NewThread bool` (Design note 5) with doc comment.
- `internal/authz`: evaluation of the re-check principal (Design note 4) and of `NewThread` in the `agent.invoke` guard; the snapshot cache key distinguishes the re-check principal; tests.
- `internal/messaging` port and app, each only if missing: `Messages.SetTriggerState` (02 4.3, writes `message.updated`), the anchor (`anchor` in 02 4.3: a message `kind = 'run'`), and the creation of the auto thread `T'` through `chat.Threads.Create` (messaging is the only caller of `Create`, M1-55 D1). If `createMessage` builds its `201` answer before the subscribers run, re-read the message so that the answer carries `trigger_state` (04 16.5).
- `internal/gateway`: the agent consumer send path of M2-07 calls `runs.Delivery.Prepare` before it sends an invocation-bearing event (Design note 9).
- The REST poll `pollAgentEvents` of M2-07: the same call. If its handler lives in a module that may not import `runs` (02 4.4: `agents` may not), that module declares the source port `DeliverySource` in its port and `runs` implements it in `internal/runs/adapters/<module>delivery` (02 4.1 point 4); `internal/wire` connects them.
- `internal/wire`: subscription `messaging.MessagePosted -> runs.OnMessagePosted` (after the existing subscribers), the job `runs.queued_expiry`, the `Delivery` port for the gateway and the poll, event registration of `agent.invocation` for module `runs` through `authorize.Registry.Event`; update the golden files.
- `internal/platform/config` schema: `run.queued_max_age` if absent (09 21); regenerate `docs/operations/configuration.md` with `make config-reference`.
- `docs/spec/02-architecture.md`: 4.3 (callers of `runs.Delivery`; the source port if you add one), 12.3 (row `runs.queued_expiry`).
- `docs/spec/09-agents.md`: 5.2 (the rows of Design note 6 with state `failed` at delivery, the new row `user_disabled`), 3.4 point 5 and 3.7 (Design note 10 c), 7 (Design note 3 f).
- `docs/observability/signals.md`: the value list of `bothy.reason` in the row `bothy.agent.invocation.denied`.
- `docs/architecture/seq-2-agent-run.mmd`: acceptance sets `trigger_state = received` and the delivery moves it to `queued` (04 15.3 steps 1 and 2; the diagram shows `queued` at acceptance today); regenerate the `.svg` with `make diagrams`.
- READMEs of `runs`, `authz`, `messaging`, `gateway` and of the module that holds the REST poll; `services/server/CHANGELOG.md`; `docs/DECISIONS.md` section 10 rows `D-M2-08-<n>` (Design note 16).

## Out of scope

- Run start and resume with token check, `run.token.renew`, `run.finish`, `run.cancel`, the jobs `runs.cancel_timeout` and `runs.stall_check`, the cancellation of runs on revocation (R-SY-11), on `agents.AgentDisabled` and on `chat.ChannelReadOnlySet`: M2-09. M2-09 keeps AG-07, AZ-M51 and BR-04 of this ticket green (Design note 3 g).
- Token issuer, claim schema, key purpose `invocation`, rotation, reference verifier, AG-22: M2-06.
- Consumer cursor, `ack`, in-flight limit, redelivery timer, `4015`, the REST poll and ack operations: M2-07.
- Follow-up creation at acceptance (`run_followups`, `steer` and `queue`, `followup_delivered`, deferred follow-ups): M2-11. Until M2-11, a message that would become a follow-up stays a normal message without `trigger_state` (Design note 10 e).
- Components and interactions, their producer and the trigger resolver for `interaction.created`: M2-15. Command execution, the resolver for `command.invoked` and model commands (`run_active`): M2-16. `command.invoked` and `interaction.created` for apps without `agent.invoke` (R-AZ-18): M2-25. Autocomplete tokens (`trigger_kind = autocomplete`): M2-17.
- `agent_session` in the payload and the end of the main-thread session after an operator change: M2-12. Run context: M2-13. SR9 and the token in requests: M2-14.
- Heartbeat with `reported_tool_class`, `reported_billing_mode` and project readiness: M2-20. Project binding: M2-03.
- Attachments that hold an invocation (`ReleaseInvocation`, `runs.invocation_hold_timeout`): M3-07; voice messages and `voice_unsupported`: M3.
- SR3, `dev_environment_not_ready`, `dev_environment_foreign_thread`, claim `dev_environment_id`: V2-02.
- Fencing of agent consumers over several nodes: M7-17.
- Webhook transport to agents (04 15.6, `webhook_subscriptions.agent_id`): no ticket owns it yet (reported as a plan gap); it must call `runs.Delivery` the same way.
- Load test `perf.invocation_delivery_p99`: M2-36. Conformance suite AG-01 to AG-30 with the fake agent: M2-31. Adapter side of the token (AZ-P6): M2-29, M2-33, M2-34.
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M2-08-invocation-delivery`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(runs): ...`, `feat(authz): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. Never log, print or persist an invocation token or a private key on the server side, not even in test output; tests compare tokens in memory. No network calls except package downloads for pinned tools and Go modules.
- No agent runtime, no model call and no third-party service in any test: the adapter side is the gateway test client of M1-64 with an agent token and the REST poll of M2-07.
- Never lower a threshold, disable a gate or delete a test.
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same holds for the merged code of M2-04 to M2-07: follow its names; where it lacks a function this brief expects from it (for example the run creation or `SetTriggerState`), add it in the module that owns it and report it.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11). The principal of the re-check (Design note 4) is in the core contract since D-W-104: 05 section 5 "Check without a request" with 05 2.3 point 1 and 04 15.3 step 2 (R-AZ-29); build it as written.

## Design notes

1. **Port `runs.Delivery`** (02 4.3 "delivery preparation with I2, guard rules and token"; the sequence diagram `docs/architecture/seq-2-agent-run.mmd` calls it `PrepareDelivery`):

   ```go
   // Delivery prepares the hand-over of one invocation-bearing event to one agent. Every
   // transport calls Prepare immediately before it sends such an event (04 15.3 step 2).
   type Delivery interface {
       Prepare(ctx context.Context, in DeliveryInput) (DeliveryOutcome, error)
   }
   type DeliveryInput struct {
       AgentID       ids.ID          // audience_principal_id of the event
       EventID       ids.ID
       Seq           int64
       Type          string          // agent.invocation, agent.followup, interaction.created, command.invoked
       ResourceKind  string
       ResourceID    ids.ID
       Payload       json.RawMessage
       CorrelationID ids.ID          // event_log.correlation_id; zero when absent
       Redelivered   bool
   }
   type DeliveryOutcome struct {
       Send            bool   // false: the event counts as checked and not admitted
       InvocationToken string // set exactly when Send is true
   }
   ```

   The port uses only `kernel` types and the standard library, because `sync` is above `runs` (02 4.4). `Prepare` returns an error only for failures (database, signing, unknown run, a run of another agent); policy results are always an outcome. `kernel/authorize.Delivery(ctx)` of M1-65 is a context marker of the hub and is not used here (note 3 d).

2. **Delivery matrix** (pure function in `runs/domain`, table test). "Check" means note 3 d to h.

   | Event type | `queued` | `running` | `waiting_input` | `completed`, `failed`, `cancelled` |
   |---|---|---|---|---|
   | `agent.invocation` | check; allow: token, record `jti`; deny: run `failed` | check; allow: token, `jti` not recorded; deny: run unchanged | as `running` | skip without check |
   | `agent.followup` | check; allow: token, `jti` not recorded; deny: follow-up refused | as `queued` | check; allow: token, record `jti`; deny: follow-up refused | skip without check |
   | `interaction.created` | row of `agent.followup` (04 15.3 step 5); resolver from M2-15 | | | |
   | `command.invoked` | check; `jti` never recorded (09 5.1 lists it under no start); resolver and the case without run from M2-16 | | | |

   Reasons: only a delivery that may start the run replaces `runs.invocation_jti` (09 5.1). `agent.invocation` records it only while `queued`: in `waiting_input` the original invocation must not resume the run without an answer of a human (D-W-44, R-AG-25); its later deliveries still carry a fresh token, and a start with it gets `409 already_started` or `superseded` (09 4.5, M2-09). Terminal runs need no check because nothing can start (Ruling for docs/DECISIONS.md).

3. **`Prepare`, one unit of work** (`txn.UnitOfWork.Do`, deadlocks retried by `database.tx_retry_max`):
   a. A type outside `InvocationBearing` is a caller error: internal error. An invocation-bearing type or trigger kind without a registered resolver (note 5) gives `Send = false` and an `Error` log naming the type, because a retry cannot help; nothing is sent (fail closed).
   b. Lock the run row (`SELECT ... FOR UPDATE`; run id from `ResourceID` for `agent.invocation` and `agent.followup`, from the resolver for the other two). A missing run or `runs.agent_id <> AgentID` is an internal error.
   c. Apply the matrix of note 2. For a `queued` run with `now >= queued_at + run.queued_max_age`: CAS `queued -> failed` with `agent_unavailable` through the transition of M2-05, `trigger_state = failed` with `agent_unavailable`, return `Send = false`. No check, no token, no audit (it is not an authorization decision; 07 4.1 has no such reason).
   d. Read the trigger facts through the resolver (note 5). Read the current permission version once through `instance.Versions` and pin it with `authorize.WithPermissionVersion`; decide in filter mode (`authorize.Filter(ctx)`, M1-28) with the re-check principal of note 4: `Authorize(ctx, recheck(triggerID), agent.invoke, Resource{Kind: thread, ID: run.thread_id, Invoke: &InvokeFacts{AgentID: run.agent_id, TriggerOrigin, MentionsAgent, ActiveRunAgentID: run.agent_id, NewThread}})`. Filter mode is used because this use case writes the one audit entry itself (note 8); a second, aggregated entry from the authz layer would count the same denial twice (Ruling for docs/DECISIONS.md).
   e. Denial, `agent.invocation` of a `queued` run: CAS `queued -> failed` with the mapped reason (note 6); `Messages.SetTriggerState(triggering message, failed, reason)` when the run has a triggering message; audit (note 8); counter (note 15); return `Send = false`.
   f. Denial, `agent.followup`: the follow-up message gets `trigger_state = failed` with the mapped reason; the run is unchanged (the run's own trigger was checked for its own invocation; another person's follow-up must not end it); M2-11 adds the state of the `run_followups` row; audit; counter; `Send = false` (Ruling for docs/DECISIONS.md; 09 7 gets the sentence).
   g. Denial of `agent.invocation` while `running` or `waiting_input`: nothing is sent, the run is unchanged, audit, counter. Ending such a run is the revocation path of M2-09 (R-SY-11). M2-09 must keep queued, undelivered runs on this path: the delivery check fails them (`failed`, `access_revoked`, AZ-M51) before the adapter handles `run.cancel_requested`.
   h. Allow: read the configured policy (`billing_mode`, `tool_class`) through the `agents` port, issue the token through the issuer of M2-06 with the claims of note 7, then store in the same transaction: `invocation_jti = jti` and `renewed_from_jti = NULL` when the matrix says "record", `billing_mode_expected` = current `agents.billing_mode`; raise `runs.version` by one only if one of these columns changed (03 `runs.version`); for `agent.invocation` of a `queued` run whose triggering message has `trigger_state = received`, `SetTriggerState(queued)` (note 12). Commit, then return `Send = true` with the token.

4. **Principal of the re-check** (core contract: 05 section 5 "Check without a request", 05 2.3 point 1 and 04 15.3 step 2, R-AZ-29; the code details below are a ruling for docs/DECISIONS.md). A delivery has no request, session, device or token of the triggering person; R-AZ-29 makes that person the principal of the check, with liveness from the principal status only and owner status from the current owner list, so a trigger that rests on owner permissions passes the re-check (AZ-M19, every `personal` agent, every `host` agent). Build: the re-check uses a principal of kind `user` with the person's ID, no session, device, token, `view_as` or `acting_for`, and a re-check marker in `kernel/actor` (for example a field `Recheck bool`). `authz` evaluates it like this: liveness only from the principal status (`principal_inactive`, `principal_pending`); owner status from the current owner list without a session; the marker is allowed only with the action `agent.invoke` and only with all the other fields empty, otherwise an internal error (R-AZ-29 also admits `command.invoke` and `interaction.create`, which M2-25 adds, and the SR9 permissions of the triggering person, which M2-14 computes the same way); the snapshot cache key includes the marker, so no token principal of the same person ever gets the owner snapshot. Reason: acceptance already applied R-AZ-14 to the real request; the re-check re-evaluates the same human with the permissions of the moment. Residual (named in R-AZ-29): an owner who triggered with a personal API token (acceptance without owner permissions, on grants) and loses those grants before the delivery is still allowed as owner at delivery; no permission of another person is used. M2-09 (`run.token.renew`), M2-11 (the follow-on run of deferred follow-ups) and M2-25 (the re-check of app commands and app interactions) reuse this principal, and M2-14 computes the triggering person's permissions in SR9 the same way, all under R-AZ-29.

5. **Trigger facts per event type** (resolver table in `runs/app`; one entry per type, for `agent.invocation` one per `runs.trigger_kind`):
    - `agent.invocation`, `trigger_kind = message` (this ticket): trigger = `runs.triggered_by`; triggering object = `runs.triggering_message_id`; `TriggerOrigin` and the mentions from that message, read now through the `messaging` port; place = current thread of the run (`chat.Threads`); `NewThread = true` when the thread's `parent_message_id` equals the triggering message (the auto thread of 09 3.7), so the guard applies the channel's main-thread mention rule "in the main thread and for new threads" (05 5 point 3; M1-28 would otherwise apply `thread_require_mention`).
    - `agent.followup` (this ticket): run = `ResourceID`; trigger = author of the follow-up message (`AgentFollowupPayload.message_id`); origin and mentions from that message; `NewThread = false`.
    - `agent.invocation` with `trigger_kind` `command` or `interaction`, `interaction.created`, `command.invoked`: M2-16 and M2-15 add their entries (05 5 point 6 for interactions). Until then such an event is never sent (note 3 a).
    - SR4 at delivery: guard 1 of M1-28 (principal kind `user`, `TriggerOrigin = human`) plus liveness of the re-check principal.

6. **Reason mapping at delivery** (pure function; the audit keeps the exact `authorize` reason, the run and the message get a code of 09 5.2; Ruling for docs/DECISIONS.md, and update 09 5.2 accordingly):

   | `authorize` reason | `runs.state_reason`, `trigger_state_reason`, counter value |
   |---|---|
   | `billing_mode_personal_not_holder` | `billing_mode_personal_not_holder` |
   | `tool_class_owner_only` | `tool_class_owner_only` |
   | `project_not_ready` | `project_not_ready` |
   | `channel_read_only`, `channel_archived` | `channel_read_only` |
   | `principal_inactive` | `user_disabled` (02 4.3; `RunCancelRequestedPayload.reason`) |
   | `agent_not_reachable` and the agent is not active (`agents` port) | `agent_disabled` |
   | every other reason (`no_grant`, `level_insufficient`, `agent_not_reachable`, `origin_not_human`, ...) | `access_revoked` (AZ-M51) |

   `dev_environment_not_ready` and `dev_environment_foreign_thread` cannot occur before V2-02, which adds their rows. In 09 5.2 add the state `failed` "on denial before delivery" to `access_revoked`, `agent_disabled` and `channel_read_only`, and add the row `user_disabled` (`cancelled` on revocation, `failed` at delivery).

7. **Claims of a delivery** (05 5, 09 4.5; the issuer of M2-06 signs):

   | Claim | Value |
   |---|---|
   | `iss`, `aud`, `sub` | `instance.instance_id`; `runs.agent_id`; the trigger of note 5 |
   | `jti`, `iat`, `exp` | new UUIDv7 per delivery; now; now plus `agent.invocation_token_ttl` |
   | `trigger_kind` | `runs.trigger_kind` for `agent.invocation`; `followup` for `agent.followup`; `interaction` for `interaction.created`; `command` for `command.invoked` |
   | `run_id` | the run of the event; absent only for `autocomplete` (M2-17) and for a `command.invoked` whose payload names no run (M2-16; 05 section 5, R-AZ-25) |
   | `message_id` or `interaction_id` or `command_id` | the triggering object of note 5 |
   | `thread_id`, `channel_id`, `space_id` | the run's thread and its current channel and space (07 4.2 resolves the place at decision time) |
   | `level` | `trigger`: an `allow` of `agent.invoke` implies effective `trigger`, the highest level (05 4.10, 07 2.3) |
   | `billing_mode`, `tool_class` | current `agents.billing_mode` and `agents.tool_class` (configured, never the reported class) |
   | `permission_version` | the pinned version of note 3 d |
   | `dev_environment_id` | absent until V2-02 |

8. **Audit** (10 14 row "I2 rejection", 07 4.2, 19 SEC-AUD-5). One `auditing.Recorder.Record` per denial in the transaction of note 3 (not `RecordDenial`: the transaction commits): action `agent.invoke`, decision `deny`, reason = the exact `authorize` reason, resource `thread` with the run's thread, attributes `run_id`, `agent_id`, `stage` (`delivery`), `event_type`, `trigger_kind`. Actor in the context: the re-check principal (no device, no session); correlation ID: the event's `correlation_id`. Nothing else, never a token or a `jti`. Allowed deliveries write no audit entry (05: Audit `denials`).

9. **Transports.** The gateway's agent consumer path (M2-07) calls `Prepare` for every event with `InvocationBearing(type)` addressed to the connected agent, from the consumer's own delivery goroutine, never from the hub goroutine (M1-65: `Dispatch` never blocks); on `Send` it builds the envelope with `sync.DeliveryFields{InvocationToken: ..., Redelivered: ...}`. The REST poll does the same per event of a page. A `Send = false` event counts as checked and is not in flight: the REST answer omits it and `next_cursor` covers it; the gateway treats it like a non-admitted event (04 7.1 `cursor` frame), and the adapter's later cumulative `ack` covers it. An event whose `Prepare` failed with an error is not sent but stays in flight, so the redelivery after `agent.ack_timeout` or the next connection prepares it again (Ruling for docs/DECISIONS.md). Every redelivery calls `Prepare` again and therefore carries a new token (R-SY-07). `run.cancel_requested` and `agent.config_changed` carry no token and do not pass `Prepare`.

10. **Acceptance** (`OnMessagePosted`, subscriber of `messaging.MessagePosted` in the message's unit of work):
    a. Only `Origin = human` and `Kind = text`; a DM channel never triggers (09 3.4 points 1 and 2; SR4). Voice is M3.
    b. Gather the facts for the decision function of M2-04 (09 3.4 points 3 to 6): thread kind, operator (fixed operator of a thread, effective operator of the channel for the main thread, 09 3.1), channel switches, mentioned agents (`MentionedIDs` of agent kind), auto thread, active run of the target. Permission facts come only from `authorize` (05 rule 1.1): call `Authorize` in filter mode with the request principal for the candidate `A` on the thread `T` where `M` was posted (`Invoke{AgentID: A, TriggerOrigin: human, MentionsAgent, ActiveRunAgentID: active run of the target, zero for a new thread}`) and feed its outcome into the decision function as M2-04 defines its inputs (follow the merged names).
    c. Auto thread (09 3.4 point 5, 3.7): the check of point 7 is evaluated for the target `T'` before `T'` is written, on `T` with the main-thread rule (05 5 point 3, "for new threads"); `T'` is created only after `allow`, in the same transaction, through `messaging` (parent `M`, empty title with `title_source = auto`, operator = effective operator of the channel). A denial creates no empty thread (Ruling for docs/DECISIONS.md; add the sentence to 09 3.4 point 5 and 3.7).
    d. Outcome mapping (09 3.5): `allow` without an active run of `A` in the target: invoke (f). `level_insufficient` (P lacks `trigger`): `trigger_state = failed` with `no_trigger_right` when `A` was mentioned, otherwise nothing (R-AG-05). `agent_not_reachable`: `agent_disabled` when `A` is not active, `agent_not_allowed_here` when `A` was mentioned, otherwise nothing. `run_active_other_agent`, `billing_mode_personal_not_holder`, `tool_class_owner_only`, `project_not_ready`: that code. Rejections of the decision function (`multiple_agents_mentioned`, `agent_not_allowed_here` for a guest without `allow_mentioned_agents`): that code. Every `trigger_state = failed` that comes from an `authorize` denial writes one audit entry as note 8 with `stage = acceptance` and `message_id` instead of `run_id`; silent cases and decision-function rejections write none (Ruling for docs/DECISIONS.md).
    e. `allow` with an active run of `A` in the target is a follow-up (09 3.4 point 6): M2-11. Until then the message gets no `trigger_state` and nothing else happens; a test pins this and M2-11 replaces it.
    f. Invoke, in this order in the transaction: (auto thread `T'`); the run through the creation function of M2-05 (`state = queued`, `trigger_kind = message`, `triggering_message_id = M`, `triggered_by = P`, `agent_id = A`, `thread_id` = target, `billing_mode_expected` = current configured mode, `invocation_jti` = a fresh UUIDv7 that no token carries until the first delivery replaces it, Ruling for docs/DECISIONS.md), which writes `run.state_changed`; the anchor in the target (`kind = 'run'`, author `A`, `origin = agent`, empty content, `run_id`, `silent = true` because the anchor never notifies, 09 5.4; it publishes `MessagePosted` with origin `agent`, which triggers nothing); `SetTriggerState(M, received)`; the job of note 11; last `agent.invocation` (note 14).
    g. A replay of `createMessage` with the same `client_msg_id` publishes no `MessagePosted` (M1-56) and therefore creates no second run.
    h. No `rl.trigger` is consumed: the request is `createMessage` with `rl.message` (05 3); reported as a spec gap.

11. **Job `runs.queued_expiry`** (09 5.5, D-AG-16; Ruling for docs/DECISIONS.md and row in 02 12.3: module `runs`, scheduled, `run.queued_max_age`, queue `default`, domain rule in 09). The run creation inserts it with `jobs.Inserter.InsertTx` and `ScheduledAt = queued_at + run.queued_max_age` in the creating transaction; arguments are the run ID only (02 12.1). The handler locks the run; if it is still `queued` and due: CAS to `failed` with `agent_unavailable` and `trigger_state = failed` with `agent_unavailable`; if it is `queued` but not yet due (the key was raised), it schedules itself again for the new due time; otherwise it does nothing. Note 3 c covers an adapter that comes back before the worker ran the job. Runs created later by M2-11, M2-15 and M2-16 use the same creation function and get the job.

12. **`trigger_state`** (09 6, R-AG-09): `received` at acceptance; `queued` set by `Prepare` in the transaction that records the first token for the run, because handing the event to the transport is the delivery the server can observe; a lost frame is delivered again after `agent.ack_timeout` (Ruling for docs/DECISIONS.md). Later deliveries do not touch it. `failed` with reason on every denial of note 3 e and f and on expiry. Every change goes through `Messages.SetTriggerState` and writes `message.updated` (02 4.7).

13. **Concurrency.** Acceptance runs after `AppendToThread`, which holds the thread row lock (M1-55 D2), so two messages of one thread decide one after the other; the partial unique index of R-DM-14 is the last line. `Prepare` serializes on the run row: two consumers of one agent (two adapter processes) or a takeover with `4015` give two tokens with different `jti`; the later `Prepare` wins `invocation_jti` (AG-09). The expiry job and `Prepare` serialize on the same row lock; the CAS of M2-05 lets only one transition win. Signing happens inside the transaction but without network calls (02 8.5: the key store of M1-32 reads local files).

14. **Events.**
    - `agent.invocation`: `audience` = `A`, `receive_action` = `agent.events.consume`, `resource` `{kind: run, id}` without `version` (04 9 point 3), `scope` `{space_id, channel_id, thread_id}` of the target, `actor` = `P`, `correlation_id` of the request; payload `AgentInvocationPayload` with `run_id`, `thread_id`, `channel_id`, `space_id`, `trigger_kind = message`, `message_id`, `triggered_by` (the author's `PrincipalRef` as the `messaging` port returns it, because `runs` may not use `identity`), `billing_mode` (configured at acceptance), `settings_version` (channel); `agent_session` comes with M2-12. No content, no prompt (04 2.2 rules 1 and 4). The payload validates against the schema and contains no JSON `null` (ADR 0053).
    - `agent.followup`: the builder (`run_id`, `thread_id`, `message_id`; same envelope rules) is built here and called by M2-11.
    - The token is a delivery field only; the event log row never contains it (R-SY-17).

15. **Telemetry** (signals.md). Counter `bothy.agent.invocation.denied` with `bothy.agent.handle` and `bothy.reason` (values: the right column of note 6) per denial of note 3 e, f and g; span `agent.dispatch` (PRODUCER) per `Prepare` that runs the check, with `bothy.run.id`, `bothy.agent.handle`, `bothy.decision`, linked to the event's correlation (signals.md 5.1 point 5). `bothy.run.finished` and `bothy.run.start.latency` (outcome `failed`) are recorded by the transition of M2-05; if it does not record them, record them here for the denial and the expiry. Declare instruments in `internal/platform/otel/metrics` (M1-68, in the closure through M2-02), with name, unit and attributes exactly as signals.md.

16. **Rulings for `docs/DECISIONS.md`** (section 10, one row `D-M2-08-<n>` each, with reason and place): the code side of the re-check principal (note 4: marker, its checks, cache key; the rule itself is R-AZ-29); filter mode plus one own audit entry per denial (notes 3 d and 8); delivery matrix with "terminal runs are skipped" and "`agent.invocation` records its `jti` only while `queued`" (note 2); reason mapping (note 6); follow-up denial leaves the run unchanged (note 3 f); skip versus error in the transports (note 9); auto thread written only after `allow` (note 10 c); audit at acceptance only for `authorize` denials (note 10 d); unissued `invocation_jti` at creation (note 10 f); job `runs.queued_expiry` (note 11); `trigger_state = queued` set in the transaction of the first token (note 12); `runs.version` raised only when `Prepare` stores a change (note 3 h); the source port `DeliverySource` if you add it (Scope).

## Tests (write them first)

The test world is 07 "Examples": spaces Support, Marketing, Engineering, Research; agents Lotse (operator of Support, `organization`, `chat`), Texter (operator of Marketing, `organization`, `scoped`), Runner (operator of the forum Engineering/Projekt-A and of the text channel Engineering/Werkzeuge, `organization`, `host`), Assistent (operator of Research, `personal`, holder the Owner, `scoped`); one Owner; G-1 (Alex, `trigger` on Support), G-2 (Sam, `trigger` on Marketing). The test clock of `kernel/clock` drives time. The adapter side is the gateway test client of M1-64 with the agent's API token and a consumer name (M2-07), or the REST poll. Tokens are verified with the reference verifier of M2-06 against `getJwks`.

Unit:

- `internal/runs/domain`: `TestDeliveryMatrix` (every cell of note 2), `TestDeliveryReasonMapping` (every row of note 6, unknown reasons give `access_revoked`), `TestInvocationClaims` (per trigger kind: `sub`, triggering object, `level = trigger`, `tool_class` is the configured class even when the reported class is higher, `exp - iat` equals `agent.invocation_token_ttl`, no `dev_environment_id`), `TestQueuedAge` (exactly at `queued_at + run.queued_max_age` the run is too old), `TestInvocationPayload` (validates against `AgentInvocationPayload`, no `null`, no content).
- `internal/authz`: `TestRecheckPrincipal_OwnerWithoutSession` (owner status without session, AZ-M19 at the decision level), `TestRecheckPrincipal_DisabledPerson` (`unauthenticated`, `principal_inactive`), `TestRecheckPrincipal_OnlyAgentInvoke` (any other action is an internal error), `TestRecheckPrincipal_RejectsSessionOrToken`, `TestRecheckPrincipal_CacheKeySeparate` (a personal-token principal of the same owner never gets the owner snapshot), `TestAgentInvoke_NewThreadUsesMainRule` (`thread_require_mention` set, `require_mention` not set, no mention: allowed only with `NewThread`).

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, HTTP and gateway through the real chain):

- `TestDelivery_HappyPathCarriesFreshToken` (04 16.5 up to the `ack`): Alex mentions Lotse in Support/Allgemein (`auto_thread` set); `201` with `trigger_state = received`; a new thread `T'` with `parent_message_id = M` holds the run and the anchor; Lotse's consumer receives `agent.invocation` with `invocation_token`; the token has `alg = EdDSA`, `typ = agent-invocation+jwt`, a `kid` of `getJwks`, `aud` Lotse, `sub` Alex, `jti` equal to `runs.invocation_jti`, `run_id`, `message_id` M, `thread_id` T', `channel_id`, `space_id`, `level = trigger`, `billing_mode = organization`, `tool_class = chat`, `trigger_kind = message`, `permission_version`; `M` then has `trigger_state = queued` and exactly one `message.updated` for it.
- `TestDelivery_EveryDeliveryFreshToken` (R-SY-07): no `ack`; after `agent.ack_timeout` the redelivery has `redelivered: true` and a token with a new `jti` that replaced `runs.invocation_jti`; `renewed_from_jti` is NULL; no second `message.updated`.
- `TestDelivery_TokenNeverStored`: the token string appears in no `event_log` row, no `audit_events` row and no captured log line.
- `TestDelivery_RESTPoll`: `pollAgentEvents` returns `agent.invocation` with `invocation_token`; a denied invocation is not in `events`, and `next_cursor` is at least its `seq`.
- `TestDelivery_AG07_AZM51_RevokedBeforeDelivery` (AG-07, AZ-M51, 05 test 6): Lotse offline; Alex mentions Lotse; the Owner revokes G-1 (`revokeGrant`); Lotse connects: no `agent.invocation` frame; the run is `failed` with `access_revoked`; `M` has `trigger_state = failed` with `access_revoked`; `run.state_changed` was written; exactly one audit entry `agent.invoke` `deny` with actor Alex, the `authorize` reason, resource the thread and `run_id`, `agent_id`, `stage = delivery`; `bothy.agent.invocation.denied` rose by one with `bothy.reason = access_revoked`.
- `TestDelivery_GrantExpiredBeforeDelivery` (AZ-P10 style): Alex's grant on Support has an expiry, the clock passes it, `authz.expiry_sweep` does not run: same outcome as the test above.
- `TestDelivery_BR04_SwitchBetweenAcceptanceAndDelivery` (BR-04): (a) Alex triggers Lotse; the Owner revokes G-1 and switches Lotse to `personal` with the Owner as holder (I1 holds); delivery refused, run `failed`. (b) I2 without I1: a raw SQL statement on the `pgtest` connection sets Lotse to `personal` with the Owner as holder while Alex keeps G-1 (a fixture that simulates an I1 defect); delivery refused with audit reason and run reason `billing_mode_personal_not_holder`. (c) The Owner triggers Assistent, then switches Assistent to `organization`: delivered, token `billing_mode = organization`, `runs.billing_mode_expected = organization` (10 9.2 point 4).
- `TestDelivery_AZM19_OwnerTriggersHostRunner` (AZ-M19): the Owner opens a post in Engineering/Projekt-A; acceptance allows; the delivery to Runner passes with `tool_class = host` although no request of the Owner exists at that moment (note 4). With M2-03 merged, Projekt-A has a ready binding (created with `putProjectBinding`; readiness set through the port that M2-03 offers for `project_bindings.ready`, follow the merged name); without it the test runs on Projekt-A without binding and the report names this under "What is not proven".
- `TestDelivery_AZM20_HostRunnerRefusedForNonOwner` (AZ-M20): Alex with a `trigger` grant on Engineering/Werkzeuge mentions Runner: `trigger_state = failed` with `tool_class_owner_only`, no run, no thread, one audit entry. Variants at delivery: Runner is `scoped` at acceptance and set to `host` (`setAgentToolClass`) before the delivery; Runner is `scoped` with `reported_tool_class = host`: both refused with `tool_class_owner_only`, run `failed`.
- `TestAcceptance_AZM30_AgentMessageNeverTriggers` (AZ-M30): with `free_response` set on Support/Allgemein, Lotse posts with its agent token a message that mentions Texter: no run, no `trigger_state`, no `agent.invocation`, no audit entry.
- `TestDelivery_SR4RecheckFailsClosed`: inside `internal/runs` tests, a run is created through the creation function for a message whose origin is `agent` (impossible through acceptance); `Prepare` refuses with audit reason `origin_not_human` and run reason `access_revoked`; a run whose trigger was disabled (`disableUser`) is refused with `user_disabled`.
- `TestDelivery_AG09_TwoAdapterProcesses` (AG-09, delivery part): (a) two consumers `a` and `b` of Lotse receive the same `agent.invocation` with different `jti`; `runs.invocation_jti` equals the `jti` of the later `Prepare`; two concurrent CAS starts through the transition of M2-05 give exactly one success. (b) A second `identify` of the same consumer closes the first with `4015`; the unacknowledged event comes again with a new token. (c) A second server instance in the same test binary after a restart redelivers the unacknowledged event with a new token; there is still one run and one `trigger_state` change. The full scenario with `startRun` runs in M2-09 and M2-31.
- `TestQueuedExpiry_AG27` (AG-27): Lotse offline; the clock passes `run.queued_max_age`; the job sets the run to `failed` with `agent_unavailable` and `trigger_state = failed` with `agent_unavailable`; when Lotse connects later nothing is delivered and no audit entry is written. `TestDelivery_QueuedTooOldWithoutJob`: the worker did not run the job; `Prepare` fails the run with `agent_unavailable` and sends nothing.
- `TestDelivery_AuditOnEveryDenial`: a table over the denials of note 3 e, f and g; each writes exactly one audit entry with the attributes of note 8 and nothing else; allowed deliveries and skips write none.
- `TestDelivery_InvocationAfterStartNotRecorded`: the run moved to `running` through M2-05; a redelivered `agent.invocation` is sent with a fresh token and `runs.invocation_jti` is unchanged; in `waiting_input` the same.
- `TestDelivery_FollowupJtiRules`: an `agent.followup` built with the builder of note 14 for a `running` run is sent with `trigger_kind = followup` and the follow-up's `message_id`, without changing `invocation_jti`; for a `waiting_input` run it replaces `invocation_jti` and clears `renewed_from_jti`; after the follow-up author lost `trigger` it is not sent, the follow-up message is `failed` with `access_revoked`, the run is unchanged, one audit entry.
- `TestDelivery_TerminalRunSkipped` and `TestDelivery_UnregisteredResolverFailsClosed` (`interaction.created` and `command.invoked` to an agent are not sent before M2-15 and M2-16).
- `TestAcceptance_AutoThreadAndDenial` (AG-05, integration part): allow creates `T'`, the run and the anchor in `T'` and `trigger_state` on `M`; a denied trigger in the same channel leaves `M` with `failed` and creates no thread.
- `TestAcceptance_Events`: the transaction ends with `agent.invocation`; audience Lotse, receive action `agent.events.consume`, resource `run` without `version`; payloads validate against `AgentInvocationPayload` and `RunStateChangedPayload`; the anchor has `kind = run`, `origin = agent`, `silent = true`.
- `TestAcceptance_DenialReasons`: `no_trigger_right` with mention and a silent message without mention (no audit entry), `agent_not_allowed_here`, `run_active_other_agent`, `billing_mode_personal_not_holder` (b-style fixture), `multiple_agents_mentioned` (no audit entry).
- `TestAcceptance_FollowupBranchUntilM211`: a second human message to Lotse in a thread with Lotse's active run creates no run and no `trigger_state`.
- `TestAcceptance_ConcurrentMessagesOneRun`: two messages to Lotse in one thread at the same time give exactly one run (R-DM-14).
- `TestAcceptance_ReplayCreatesNoSecondRun`.
- `TestDeliveryMetrics`: with an SDK `ManualReader`, `bothy.agent.invocation.denied` carries only the attributes of signals.md and only the values of note 6.
- `internal/wire`: golden files of subscriptions, jobs and event registrations updated.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/runs/... ./internal/authz/... ./internal/kernel/... ./internal/messaging/... ./internal/chat/... ./internal/agents/... ./internal/gateway/... ./internal/wire/...`
- `make codegen-check`, `make arch`, `make api-lint`, `make api-checks`, `make register-check`, `make catalog-check`, `make config-doc-check`, `make docs`
- `make check`

## Acceptance criteria

- [ ] `runs.Delivery.Prepare` runs before every delivery of `agent.invocation`, `agent.followup`, `interaction.created` and `command.invoked` to an agent, on the gateway and in the REST poll; types without a resolver are never sent.
- [ ] AG-07 and AZ-M51: a revocation or an expiry between acceptance and delivery gives run `failed` with `access_revoked`, no delivery, and an audit entry.
- [ ] BR-04: a revocation or a switch between acceptance and delivery gives run `failed` and no delivery; I2 refuses with `billing_mode_personal_not_holder` even when I1 was bypassed; a switch to `organization` is delivered with the new configured mode.
- [ ] AZ-M19: the Owner's trigger of the `host` Runner passes acceptance and delivery; AZ-M20: Alex is refused with `tool_class_owner_only` at acceptance and at delivery, also when only the reported class is `host`.
- [ ] AZ-M30: a message with `origin = agent` creates no run; the SR4 re-check refuses a non-human trigger at delivery.
- [ ] AG-09 (delivery part): two adapter processes and every redelivery get different tokens; only the newest `jti` is current; exactly one CAS start succeeds.
- [ ] AG-27: a run older than `run.queued_max_age` fails with `agent_unavailable`, by the job or at the next `Prepare`, and is never delivered.
- [ ] Every denial before a delivery writes exactly one audit entry with the attributes of Design note 8.
- [ ] `trigger_state` goes from `received` to `queued` with the first successful delivery of `agent.invocation`, exactly once.
- [ ] Every delivery carries a fresh token with the claims of Design note 7; the token is in no stored row and no log.
- [ ] Acceptance writes run, anchor, `trigger_state = received` and `agent.invocation` in the message's transaction; payloads validate against their schemas.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `runs`, `authz`, `messaging`, `gateway` and of the module that holds the REST poll updated (port `Delivery` and its callers, the re-check principal, acceptance and anchor, the delivery matrix, skip versus error, the job `runs.queued_expiry`).
- Rulings of Design note 16 recorded in `docs/DECISIONS.md` (one row each with reason).
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
