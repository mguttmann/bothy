# M2-09: Starting, finishing and cancelling runs, orphaned runs

Status: ready | Depends on: M2-08 | Milestone: M2

## Goal

An agent adapter starts a delivered run only with a valid, current invocation token whose configured billing mode equals the mode it actually uses and whose `tool_class` is not below the class it loaded; it resumes a waiting run only with the token of the answer, renews an expired token, puts a run into `waiting_input` and ends it. A person with `trigger` who started the run, or has `manage`, stops it; without the adapter's confirmation within `agent.cancel_grace` the server cancels it itself. A loss of `trigger` by the person who started a run, a disabled person and a disabled agent end the affected runs, and runs whose adapter restarted or vanished end as `interrupted` or `stale` without any automatic retry. After this ticket the operations `startRun`, `resumeRun`, `renewInvocationToken`, `awaitRunInput`, `finishRun` and `cancelRun` are live, the deferred authorization entries `run.start` and `run.token.renew` of M1-28 are real requirements, and the jobs `runs.cancel_timeout` and `runs.stall_check` run in the worker. This is the server side of I3 (10 "Invariants"), of R-SY-10, R-SY-11 and R-SY-19 (04) and of the scenarios AG-10, AG-11, AG-14 and AG-28 to AG-32 (09 "Required scenarios"); a run can never start twice and never runs on after its starter lost the permission to start it (R-AG-07, D-L23, RK-18).

## Read first

- `docs/PLAN.md`: section 3 "Way of working per ticket"; section 8 "M2 Agent platform, extensions and terminal" (Goal, Scope, Out of scope, Exit criteria 1 and 2, Test strategy, Red team, Risks) and the rows M2-05 to M2-08 (what exists), M2-09 (this ticket), M2-10 to M2-16, M2-20 and M2-22 (who builds what next), M7-08 and M8-03.
- Dependencies: the header names M2-08. Through it this ticket also builds on M2-07, M2-06, M2-05, M2-04 and M2-02 and on the M1 tickets M1-13, M1-14, M1-16, M1-17, M1-19, M1-20, M1-24, M1-28, M1-29, M1-32, M1-33, M1-44, M1-45, M1-51, M1-53, M1-55, M1-56 and M1-65 (all implied, so not in the header).
- `docs/spec/09-agents.md`: 2.1 (a disabled agent's active runs end `cancelled`, `agent_disabled`), 2.4, 4.3 "Connection and cursor", 4.5 "Invocation token" (server check at start, the table of `reason` values, consumed token), 4.5.1 "Fresh token for a waiting run" (steps 1 to 5, `runs.renewed_from_jti`), 4.7 "Adapter operations" (rows `run.start`, `run.token.renew`, `run.finish`), 4.8 "Heartbeat and reported state" (`active_runs`), 4.9, 4.10, 5.1 "State machine", 5.2 "Reasons (`runs.state_reason`)", 5.5 "Stall, interruption, orphaned runs", 6 "Trigger state (`trigger_state`)", 7 (end of run, owned by M2-11), 8 "Stop", 15 "Reliability", 16.2 "Required scenarios" (AG-08, AG-10, AG-11, AG-14, AG-28 to AG-32), 19 (R-AG-07, R-AG-08, R-AG-17, R-AG-24, R-AG-25), 21 "Configuration values of this chapter".
- `docs/spec/04-sync-protocol.md`: 2.2 "Envelope" (personal events, `actor`, rule 3: no tokens in payloads), 8.2, 8.3 "What the client receives and does on access removal" point 5, 15.2, 15.3 "Invocation and run start" points 3 to 6, 15.5 "Cancellation", 17 (R-SY-07, R-SY-10, R-SY-11, R-SY-19).
- `docs/spec/03-data-model.md`: `runs` in 4.8 "Runs and interactions (`runs`)" (columns `state`, `state_reason`, `version`, `billing_mode_expected`, `billing_mode_used`, `billing_evidence`, `invocation_jti`, `started_jti`, `renewed_from_jti`, `metrics`, `summary`, `started_at`, `finished_at`, `duration_ms`; transitions; unique index on active runs, R-DM-14), `agents` (`last_heartbeat_at`, `reported_details`, the note on `agent.heartbeat_persist_interval`), `messages.trigger_state`, `audit_events.details` (only free text: the mandatory reason of an administrative action), 6.3 "Lock order".
- `docs/spec/05-actions.md`: rule 1.1, rule 1.6 (I3 at `run.start`), rule 1.8, 2.4 (column Audit `denials`), 2.5 "Decision", table 4.10 (rows `run.start`, `run.token.renew`, `run.finish`, `run.cancel`, `agent.invoke`), 5 "Agent invocation `agent.invoke`" (claims), 6 (rows `run.state_changed`, `run.cancel_requested`), R-AZ-04, R-AZ-07.
- `docs/spec/07-permissions.md`: 4.1 "Decisions and reasons" (`invocation_token_invalid`, `billing_mode_mismatch`, `tool_class_mismatch`, `not_self`), 4.2 "Flow" (the paragraph on deferred entries), 8.2 "Immediate effect on revocation" points 4, 5 and 8, 8.3, test rows AZ-M51, AZ-P1, AZ-P3, AZ-P6, AZ-P13.
- `docs/spec/10-backend-policy.md`: 3 "Invariants" (I2, I3), 4 "Overview of check points" (row `run.start`), 7 "Adapter check (I3)" (`key_type = unknown`), 8, 14 "Audit trail" (row I3 mismatch), 15 "Tests" (BR-06).
- `docs/spec/08-identity.md`: 11 "Disabling a person" point 7.
- `docs/spec/06-api.md`: 2 "Basic rules" (rows "No effect", "No `null`"), 5 "Errors" (points 3 and 7; `errors[].code` `not_enabled`), 7 "Idempotency" (point 3 `agent_runtime`, point 5), 16 "Endpoints" (groups "Agent runtime" and "Runs").
- `docs/spec/02-architecture.md`: 4.3 "Ports, events and sources per module" (row `runs`: subscriptions `authorize.AccessChanged` and `agents.AgentDisabled`; row `authz`: `user_disabled`), 4.4, 8.1 "Flow" (subscribers in phase 3.1), 8.2, 8.3, 12.1, 12.3 "Job catalogue" (rows `runs.cancel_timeout`, `runs.stall_check`), 12.4 "Rules for jobs".
- `docs/spec/19-security.md`: SEC-IN-4, SEC-LOG-2, SEC-AUD-5.
- `docs/api/openapi.yaml`: operations `startRun`, `resumeRun`, `renewInvocationToken`, `awaitRunInput`, `finishRun`, `cancelRun` (with `x-authz-action`, `x-rate-limit-class`, `x-idempotency-exempt`); schemas `RunStart`, `InvocationTokenRenew`, `InvocationTokenGrant`, `RunAwaitInput`, `RunFinish`, `CostReport`, `RunCancel`, `Run`, `RunSummary`, `RunState`, `RunMetrics`, `BillingEvidence`, `ToolClass`, `TriggerState`, `RunStateChangedEvent`, `RunStateChangedPayload`, `RunCancelRequestedEvent`, `RunCancelRequestedPayload`, `Problem` (`x-problem-codes`), header `Idempotent-Replay`, responses `Forbidden`, `Conflict`, `UnprocessableEntity`.
- `docs/api/asyncapi.yaml`: messages `event.run.state_changed`, `event.run.cancel_requested`.
- `docs/spec/actions.yaml`: entries `run.start` (right `agent-self and token valid`), `run.token.renew` (right `agent-self and token current`), `run.finish`, `run.cancel`.
- ADRs 0017, 0021, 0022, 0023, 0024, 0025, 0052, 0053, 0067.
- `docs/DECISIONS.md`: D-L23, D-L24, D-W-15, D-W-16, D-W-44, D-W-100, D-AG-06, D-AG-07, D-AG-16; section 10 for the format of new rows.
- `docs/security/threat-model.md`: T-TK-05 (reuse of an invocation token), T-AI-08 (misuse of a personal subscription). `docs/RISKS.md`: RK-09, RK-13, RK-18. No item of `docs/OPEN-QUESTIONS.md` concerns this ticket.
- `docs/observability/signals.md` 4.3 "Agents and runs": `bothy.run.start.latency`, `bothy.run.duration`, `bothy.run.finished`, `bothy.agent.billing_mode_mismatch`.
- `docs/operations/configuration.md`: `agent.invocation_token_ttl`, `auth.clock_leeway`, `agent.ack_timeout`, `agent.cancel_grace`, `agent.presence_timeout`, `agent.heartbeat_persist_interval`, `run.orphan_timeout`, `run.queued_max_age`, `audit.reason_max_chars`.
- Briefs: M1-13 (unit of work, `txn.Publish`, `txn.Subscribe`, golden registrations), M1-14 (`internal/platform/jobs`: `Args`, `Handler`, `PeriodicJob`, `Inserter.InsertTx` with `InsertOptions.ScheduledAt`, argument types), M1-16 (`eventlog.Event`, `eventlog.Appender`, `internal/sync/adapters/jobs` as the job adapter pattern), M1-17 (`api/operations.yaml`), M1-20 (`Idempotency-Key`), M1-24 (`auditing.Recorder`, `Entry`, `Attrs` value kinds), M1-28 (descriptor, requirement table, deferred entries, decision mapping), M1-29 (`authorize.AccessChanged`, `DiffVisible`), M1-32 (`instance.Keys.VerificationKey`), M1-33 (`kernel/jose`), M1-45 (`user.disable`), M1-51 (`agents.AgentDisabled`, the agents port), M1-55 (`chat.Threads.Info`), M1-56 (messages), M1-65.
- The briefs of the M2 tickets this one builds on: `docs/briefs/M2-05-runs-state-machine.md`, `M2-06-invocation-token.md`, `M2-07-agent-consumers.md`, `M2-08-invocation-delivery.md`, and the next ones that extend this ticket: `M2-10-run-document-live-box.md`, `M2-11-run-followups.md`, `M2-12-agent-sessions.md`.
- Code on the base branch: `internal/runs` (M2-05 state machine, reads and `run.state_changed`; M2-06 token issuer and reference verifier; M2-08 port `Delivery`), `internal/agents` (M1-51, M2-07), `internal/authz`, `internal/kernel/authorize`, `internal/kernel/auditing`, `internal/kernel/eventlog`, `internal/kernel/txn`, `internal/kernel/clock`, `internal/kernel/errs`, `internal/kernel/textcheck`, `internal/platform/jobs`, `internal/wire` (`configkeys_*.go`, `configchecks.go`, `configchapters.go`), `architecture/modules.yaml`, `api/operations.yaml`.

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

11. `startRun` (`queued -> running`) and `resumeRun` (`waiting_input -> running`) check the invocation token (signature, `kid`, `typ`, `alg`, `iss`, `aud`, run), its currency (`jti` equal to `runs.invocation_jti`, not consumed), its expiry, then the reported billing mode against the configured mode and `tool_class_used` against the `tool_class` claim; the transition is a compare-and-swap that stores `runs.started_jti` and sets `runs.renewed_from_jti` to NULL (09 4.5, 04 15.3 points 4 and 5, 03 `runs`, R-SY-10).
12. Every token error at a start or a renewal is `403 invocation_token_invalid` with `reason` `expired`, `superseded` or `invalid` (09 4.5, 06 "Errors", 07 4.1, D-W-15).
13. A repeated start with the same token answers `200` with `Idempotent-Replay: true` while the run is `running`; in `waiting_input` that token is consumed (`superseded`); a token of another delivery for a started run gets `409 already_started` (06 "Idempotency" point 5, R-SY-19, D-W-44).
14. `billing_mode_used` different from the configured mode gives `409 billing_mode_mismatch` and the run `failed`; `tool_class_used` above the claim gives `409 tool_class_mismatch` and the run `failed` (10 "Adapter check (I3)", BR-06, AG-11, AG-30, D-W-16).
15. `renewInvocationToken` runs steps 1 to 5 of 09 4.5.1 in this order and repeats the checks before a delivery with current permissions; on their failure the run becomes `failed` and the answer is `403 agent_invocation_denied` (09 4.5.1, 04 15.3 point 3, R-AG-25, AG-29, AG-32).
16. `awaitRunInput` moves `running -> waiting_input`; `finishRun` ends a run with `completed`, `failed` or `cancelled`; the same final state repeated answers `200` (09 4.7, 5.1, 05 `run.finish`).
17. `cancelRun` needs `trigger` and (triggering person of the run or `manage`), is audited (Audit `yes`), uses `rl.trigger` and `Idempotency-Key`, and writes `run.cancel_requested`; without confirmation within `agent.cancel_grace` the server sets `cancelled` with `cancel_timeout`, and later changes by the adapter are refused (09 8, 04 15.5, R-AZ-07, AG-14).
18. A loss of `trigger` by the starter cancels the starter's active runs in the same transaction through `run.cancel_requested` (`access_revoked`; `user_disabled` after `user.disable`; `channel_read_only` after an archive), for all agents (R-SY-11, 04 8.3 point 5, 07 8.2 point 4, 08 11 point 7, D-L23).
19. A disabled agent's active runs end `cancelled` with `agent_disabled` (09 2.1, 02 4.3).
20. A run missing from the adapter's `active_runs` longer than `agent.presence_timeout` fails with `interrupted`; a run without heartbeat for longer than `run.orphan_timeout` fails with `stale`; neither is ever retried automatically (09 4.8, 5.5, 15, R-AG-17, AG-10).
21. No JSON `null`; tokens never appear in events, logs, audit details, problem bodies or test output (ADR 0053, 04 2.2 rule 3, SEC-LOG-2, T-TK-05).
22. The deferred entries `run.start` and `run.token.renew` of M1-28 become real requirements and leave the golden list (07 4.2, AZ-P3).

## Scope

All paths under `services/server/` unless noted. Names of M2-05 to M2-08 below are the expected ones; where the merged code names them differently, follow the code (Boundaries).

Create:

- `internal/runs/domain/token_check.go`: pure functions `StartVerdict(c TokenClaims, r RunTokenState, op StartOp, now time.Time, leeway time.Duration) StartCheck` and `RenewVerdict(c TokenClaims, r RunTokenState) RenewCheck` with the outcomes of Design notes 3 and 4. `TokenClaims` is the parsed, signature-checked claim set; `RunTokenState{State, InvocationJTI, StartedJTI, RenewedFromJTI}`; `StartOp` is `start` or `resume`.
- `internal/runs/domain/finish.go`: pure `FinishDecision(current RunState, req FinishRequest) (FinishOutcome, error)` (Design note 6).
- `internal/runs/domain/cancel.go`: pure `CancelDecision(current RunState) CancelOutcome`, `CancelReasonFor(action authorize.Action) string` and `LostTriggerRuns(changes []authorize.PrincipalAccessChange, runs []ActiveRun) []ids.ID` (Design notes 7 and 8).
- `internal/runs/domain/orphans.go`: pure `OrphanVerdict(run ActiveRun, hb HeartbeatState, now time.Time, w OrphanWindows) OrphanOutcome` (Design note 10).
- `internal/runs/app/start.go` (`Start`, `Resume`), `renew.go` (`RenewToken`), `finish.go` (`AwaitInput`, `Finish`), `cancel.go` (`Cancel` and the in-module `requestCancel`), `subscribers.go` (`OnAccessChanged`, `OnAgentDisabled`), `timeouts.go` (cancel timeout use case), `orphans.go` (stall check use case). Outgoing app port `CancelTimeoutScheduler { Schedule(ctx context.Context, runID ids.ID, at time.Time) error }`.
- `internal/runs/adapters/jobs/`: args `CancelTimeout{RunID ids.ID}` with kind `runs.cancel_timeout` and `StallCheck{}` with kind `runs.stall_check`, their handlers (queue `maintenance`), and the scheduler that implements `CancelTimeoutScheduler` with `jobs.Inserter.InsertTx` and `InsertOptions.ScheduledAt`.
- `internal/runs/adapters/postgres/queries/run_lifecycle.sql`: lock of one run `FOR UPDATE`; the compare-and-swap updates of this ticket (through the M2-05 transition function where it exists); active runs by starter (`state IN ('queued','running','waiting_input') AND triggered_by = ANY($1) ORDER BY id FOR UPDATE`); active runs by agent; orphan candidates.
- Server-side token verification, only if M2-06 did not build it: `internal/runs/domain` (claims and checks) plus `internal/runs/app` (key lookup through `instance.Keys.VerificationKey(ctx, invocation, kid)`), parsing with `kernel/jose`; never a second JOSE library (ADR 0067). A fuzz target `FuzzParseInvocationToken` next to the parser.

Change:

- `internal/runs` port: on `Runs` add
  `Start(ctx context.Context, a actor.Principal, cmd StartRun) (StartResult, error)`,
  `Resume(ctx context.Context, a actor.Principal, cmd StartRun) (StartResult, error)`,
  `RenewToken(ctx context.Context, a actor.Principal, cmd RenewToken) (TokenGrant, error)`,
  `AwaitInput(ctx context.Context, a actor.Principal, cmd AwaitInput) (Run, error)`,
  `Finish(ctx context.Context, a actor.Principal, cmd FinishRun) (Run, error)`,
  `Cancel(ctx context.Context, a actor.Principal, cmd CancelRun) (RunSummary, error)`,
  with `StartRun{RunID ids.ID; Token string; BillingModeUsed, ToolClassUsed, Model string; BillingEvidence *BillingEvidence; ExternalSessionRef string}`, `StartResult{Run Run; Replay bool}`, `RenewToken{RunID ids.ID; Token string}`, `TokenGrant{Token string; ExpiresAt time.Time}`, `AwaitInput{RunID ids.ID; StatusLine string}`, `FinishRun{RunID ids.ID; State, Reason, Summary string; Metrics *RunMetrics; CostsPresent bool}`, `CancelRun{RunID ids.ID; Reason string}`. `actor.Principal` stands for the request principal type of `kernel/actor`.
- `internal/runs/adapters/httpapi`: strict-server methods of the six operations; `Idempotent-Replay: true` on a start replay.
- `internal/kernel/authorize`: `Resource.Token *TokenFacts` with `TokenFacts{Verdict TokenVerdict; BillingModeUsed, BillingModeExpected, BillingKeyType, ToolClassUsed, ToolClassClaim string}`, constants `TokenValid`, `TokenExpired`, `TokenSuperseded`, `TokenInvalid`, and `Decision.Detail string` (Design note 2).
- `internal/authz`: requirement entries of `run.start` and `run.token.renew` instead of `Deferred`; two rows of the "Permission" text mapping; atom `Token`; guard of `run.start`; row of the decision mapping for `invocation_token_invalid`; the golden file of deferred actions without both ids (Design note 2).
- `internal/agents` port: if no read of the persisted heartbeat exists, add `Agents.HeartbeatStates(ctx context.Context, agentIDs []ids.ID) ([]HeartbeatState, error)` with `HeartbeatState{AgentID ids.ID; LastHeartbeatAt time.Time; ActiveRunIDs []ids.ID}` (zero time when no heartbeat) reading `agents.last_heartbeat_at` and `agents.reported_details.active_runs`, with its sqlc query in `internal/agents/adapters/postgres`.
- `internal/wire`: subscriptions `authorize.AccessChanged -> runs.OnAccessChanged` and `agents.AgentDisabled -> runs.OnAgentDisabled` (method values of the `runs` app type, never closures); job registration for the roles `worker` and `all` (handler `runs.cancel_timeout`; periodic `runs.stall_check` with `Interval = agent.presence_timeout`, `RunOnStart = true`); config keys this ticket is the first consumer of, among `agent.invocation_token_ttl`, `auth.clock_leeway`, `agent.cancel_grace`, `agent.presence_timeout`, `agent.heartbeat_persist_interval`, `run.orphan_timeout`, `run.queued_max_age`, `audit.reason_max_chars` (defaults from their chapter tables, chapter constants in `configchapters.go`); the cross-check `heartbeat_persist_below_orphan_timeout` in `configchecks.go`; regenerate `internal/wire/testdata/txn_registrations.golden`.
- `api/operations.yaml`: move `startRun`, `resumeRun`, `renewInvocationToken`, `awaitRunInput`, `finishRun`, `cancelRun` from `later` to `enabled`; rerun the route generator (M1-17).
- `architecture/modules.yaml`: `run.token.renew` in the `actions` of `runs` if missing (02 4.6).
- `docs/api/openapi.yaml`: header `Idempotent-Replay` on the `200` of `startRun` and `resumeRun` (06 "Idempotency" point 5); value `channel_read_only` in `RunCancelRequestedPayload.reason` (04 8.3 point 5, 05 rule 1.8, 09 5.2). Then `make api-lint`, `make api-checks`, `make api-tables` and the server code generation.
- `docs/spec/09-agents.md` (English, like the chapter): 5.2 row `user_disabled` (`cancelled`, Server, Triggering person disabled, 08 11); 5.5 the evaluation by `runs.stall_check` (Design note 10); 8 point 2 the immediate stop of a `queued` run (Design note 7).
- `docs/spec/02-architecture.md` 12.3: row `runs.cancel_timeout` "scheduled, `agent.cancel_grace` after `run.cancel_requested`"; row `runs.stall_check` "periodic, `agent.presence_timeout`" with `interrupted`, `stale` and, if built here, `agent_unavailable`.
- `docs/operations/configuration.md`: regenerate with `make config-reference` if the schema text changes.
- READMEs of `runs`, `authz` and (if changed) `agents`; `services/server/CHANGELOG.md`; `docs/DECISIONS.md` section 10 rows `D-M2-09-<n>` for every Ruling below.
- Metrics (signals.md 4.3, closed attribute sets): `bothy.run.start.latency` when a run leaves `queued` (`started`, `failed`, `cancelled`), `bothy.run.duration` and `bothy.run.finished` at every end state (`bothy.reason` from the fixed list, else `other`), `bothy.agent.billing_mode_mismatch` at every I3 mismatch. Do not register an instrument twice if M2-05 already records it.

This ticket creates no migration and no new top-level make target.

## Out of scope

- State machine, `run.state_changed` writer, reads `listThreadRuns`, `getRun`, `listRunSteps`, `listActiveRuns`, the mapping of reasons to `trigger_state` and of unknown adapter reasons to `adapter_error` (M2-05).
- Claims, signing and rotation of the invocation token, AG-22 (M2-06); consumer cursors, `ack`, redelivery (M2-07).
- Checks before a delivery, issuing the token per delivery, `trigger_state = queued`, AG-07, AG-09, AG-27 at delivery time (M2-08). The periodic part of `agent_unavailable` belongs here only if M2-08 has none (Design note 10).
- Live-box patches, `patchRun`, gateway op `run.update` and the rejection of patches after the end; the patch part of AG-14 is asserted there (M2-10).
- Follow-ups `deferred` or `discarded` at the end of a run, `agent.followup` deliveries (M2-11); components `disabled` at the end, `interaction.created` deliveries (M2-15). Both hook into the end-of-run function of Design note 6.
- Agent sessions and binding `external_session_ref` at start (M2-12); run context `getRunContext` (M2-13); SR9 (M2-14); `/stop` (M2-16, reuses `requestCancel`); agent replies (M2-19).
- `sendHeartbeat`, presence, configured versus reported mode warnings, BR-05 (M2-20); the heartbeat rows this ticket reads come from there.
- Cost booking from `RunFinish.costs` into `cost_entries`, BR-09 (M2-22).
- `channel.read_only.set`, `chat.ChannelReadOnlySet` and its `runs` subscription (M8-03; it reuses `requestCancel` with `channel_read_only`).
- The stall warning `stall_warned_at` (09 5.5 "No progress"): the core contract has no time of the last `run.update` (spec gap in the report).
- Push "Answer ready" at the end of a run (M3-17, M3-19); maintenance mode exceptions (M7-08); fencing over several nodes (M7-17); the conformance suite with the fake agent (M2-31).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M2-09-run-lifecycle`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(runs): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same holds for M2-05 to M2-08: their merged names win over the expected names here; list each difference. If the base branch's `docs/spec/actions.yaml` lacks `run.token.renew`, the spec state is missing on the base branch: stop with `Verdict: BLOCKED <reason>`.
- No real agent runtime and no model call: the adapter in the tests is plain Go HTTP code with an agent API token; no third-party service.
- Time only through `kernel/clock`; tests move a fake clock and call job handlers directly, they never sleep for a window.
- Invocation tokens and API tokens never appear in logs, audit details, problem bodies, event payloads or test failure messages (print a hash prefix of the `jti` if needed).

## Design notes

1. **Descriptors.** Every operation loads the run first (unknown id gives the same `404` as an invisible run) and calls `authorize` with `Resource{Kind: run, ID, ThreadID: runs.thread_id, ...}`. For the agent-self actions `run.start`, `run.token.renew` and `run.finish` the field `OwnerID` is `runs.agent_id`; for `run.cancel` it is `runs.triggered_by` (05 "triggering person of the run"). Ruling for docs/DECISIONS.md: `OwnerID` of a run is the principal the action reserves it to. A run is visible to a principal with at least `read` on the thread's path (M1-28 visibility); the operator agent has it through the operator rule.

2. **Token facts in `authz`** (05 rule 1.1, 07 4.1 and 4.2: the "Permission" text "agent-self and token valid" is the requirement, and `invocation_token_invalid`, `billing_mode_mismatch` and `tool_class_mismatch` are decisions of `authorize`). `runs` verifies the token and computes the verdict (it owns keys access and the `jti` columns); `authz` decides on these facts, as M1-28 does with `InvokeFacts`.
   a. Requirement entries: `run.start`: `All(AgentSelf, Token(TokenValid))`; `run.token.renew`: `All(AgentSelf, Token(TokenValid, TokenExpired))`. Mapping rows: `agent-self and token valid` and `agent-self and token current` to these expressions. `Token(...)` fails with `forbidden` and reason `invocation_token_invalid`, and sets `Decision.Detail` to the verdict (`expired`, `superseded`, `invalid`). A missing `Resource.Token` is an internal error.
   b. Guard of `run.start` (07 4.2 step 9), only when `BillingModeUsed` is set: `BillingKeyType = unknown` or `BillingModeUsed != BillingModeExpected` gives `invariant_violation` `billing_mode_mismatch` (10 7: an unknown reported mode counts as unequal to every configured mode; RK-13); then a `ToolClassUsed` above `ToolClassClaim` in the order `chat` < `scoped` < `host` gives `invariant_violation` `tool_class_mismatch`.
   c. `Decision.Err()`: `forbidden` with `invocation_token_invalid` maps to problem code `invocation_token_invalid` with `reason` equal to `Detail`; the audit deny entry keeps `deny_reason = invocation_token_invalid` and records `token_verdict` (a code) in its details. Ruling for docs/DECISIONS.md: `Decision.Detail` carries the sub-reason of a decision whose problem `reason` differs from the audit reason.
   d. Denials are recorded by the authz layer (Audit `denials`, aggregated per `audit.deny_aggregate_window`, R-AZ-04). For `billing_mode_mismatch` the details carry `billing_mode_expected`, `billing_mode_used` and `billing_evidence` (`provider`, `key_type`, `method`, never values) as codes (10 14).
   e. Replace both entries and remove them from the deferred golden list in the same change; `VerifyRegistrations` then accepts the routes (AZ-P1).

3. **`startRun` and `resumeRun`** in one unit of work, run row `FOR UPDATE` first:
   a. Parse and verify the token: compact JWS; `alg` exactly `EdDSA`; `typ` exactly `agent-invocation+jwt`; `kid` known through `instance.Keys.VerificationKey(ctx, invocation, kid)` (states `next`, `active`, `retiring`); valid Ed25519 signature; `iss` equal to `instance.instance_id`; `aud` equal to the calling agent; `run_id` equal to the path; required claims present. Any failure is verdict `invalid` (09 4.5 table).
   b. `StartVerdict` for a token that passed 3a, first match wins; each branch names the verdict that goes to `authorize` and what follows an `allow`:
    - `jti = started_jti` and state `running`: replay; `TokenValid` without mode facts; answer as in 3f. The expiry is not checked, because the start already happened (06 "Idempotency" point 5, R-SY-19).
    - `jti = started_jti` and state `waiting_input`: `TokenSuperseded` (consumed, 09 4.5).
    - state neither `queued` nor `waiting_input`: `TokenValid` without mode facts; then `409 already_started`.
    - `jti != invocation_jti`: `TokenSuperseded`.
    - `now > exp + auth.clock_leeway`: `TokenExpired`. Ruling for docs/DECISIONS.md: the server's expiry tolerance is `auth.clock_leeway`, the upper bound of the adapter's tolerance (09 4.5 "Keys and rotation" point 2).
    - `startRun` on `waiting_input` or `resumeRun` on `queued`: `TokenValid` without mode facts; then `409 invalid_state_transition`. Ruling for docs/DECISIONS.md: each endpoint performs only its own transition (06 16 names `queued -> running` for `startRun` and `waiting_input -> running` for `resumeRun`).
    - otherwise `TokenValid` with the mode facts of 3c.
   c. Call `authorize(agent, run.start, descriptor with TokenFacts)`; the mode facts are `BillingModeUsed`, `BillingModeExpected` (`runs.billing_mode_expected`, equal to the `billing_mode` claim of a current token), `BillingKeyType` (`billing_evidence.key_type`, empty when absent), `ToolClassUsed` and `ToolClassClaim`. Resulting order of answers: `404` (unknown or invisible run), `403 forbidden` (`not_self`, `subject_kind_not_allowed`), `403 invocation_token_invalid` with `invalid`, the replay `200`, `409 already_started`, `403 invocation_token_invalid` with `superseded`, then with `expired`, `409 invalid_state_transition`, `409 billing_mode_mismatch`, `409 tool_class_mismatch`, `200`.
   d. On `billing_mode_mismatch` or `tool_class_mismatch` the run becomes `failed` with that `state_reason` in the same unit of work, which commits; the use case returns the `409` after the commit (return the outcome from the `Do` function, then the error). Count `bothy.agent.billing_mode_mismatch` for I3.
   e. `RunStart.external_session_ref`: `400 validation_failed` with `errors[].code` `not_enabled` and pointer `/external_session_ref` until M2-12 enables it (06 5 point 3; the session row it binds is M2-12's). Ruling for docs/DECISIONS.md. `RunStart.billing_evidence` is stored in `runs.billing_evidence`, `model` in `runs.model`. A schema-valid but empty `invocation_token` is verdict `invalid`; an absent one is `400 validation_failed` from the schema.
   f. On success through the M2-05 transition function: compare-and-swap on `state` and `version`; `started_jti = jti`, `renewed_from_jti = NULL`, `billing_mode_used`; `started_at` only on `queued -> running`; `version + 1`; `trigger_state = working` (unchanged on resume); `run.state_changed`. `invocation_jti` stays. Answer `200` with the `Run`; a replay answers the current `Run` with `Idempotent-Replay: true` and changes nothing.
   g. A token rejected as `invalid` changes nothing on the server; the adapter reports `failed` with `invalid_invocation_token` (09 5.2 names the adapter). Ruling for docs/DECISIONS.md: 04 15.3 point 4 "(run `failed`)" is the consequence through the adapter's `finishRun`.

4. **`renewInvocationToken`** in one unit of work with the run row `FOR UPDATE` (09 4.5.1):
   a. Step 1: parse and verify as 3a without the expiry; failure is `invalid`.
   b. `RenewVerdict` for a token that passed step 1: step 2, state neither `queued` nor `waiting_input` gives `TokenValid` and then `409 already_started` (checked before step 3, AG-32); step 3, `jti` equal to `invocation_jti` or to `renewed_from_jti`, and `started_jti IS DISTINCT FROM jti`, gives `TokenValid`, else `TokenSuperseded`. The expiry is never checked here, so `TokenExpired` does not occur.
   c. `authorize(agent, run.token.renew, descriptor with the verdict, no mode facts)`. Resulting order of answers: `404`, `403 forbidden`, `403 invocation_token_invalid` with `invalid`, `409 already_started`, `403 invocation_token_invalid` with `superseded`, `403 agent_invocation_denied`, `200`.
   d. Step 4: the checks of M2-08's `Delivery` before a delivery, for the starter (`sub`) with the triggering object of the presented token and current permissions (`agent.invoke`, I2, SR2, SR3). On failure the run becomes `failed` with the same `state_reason` the delivery check sets (07 AZ-M51 names `access_revoked`), the denial is audited as there, the unit commits, and the answer is `403 agent_invocation_denied` with `reason` equal to that `state_reason`. Ruling for docs/DECISIONS.md: the problem `reason` of a refused renewal is the run's new `state_reason`.
   e. Step 5: issue a new token through M2-06 with the same triggering object, `trigger_kind` and place, current `billing_mode`, `tool_class`, `level` and `permission_version`, a new `jti`, `exp = iat + agent.invocation_token_ttl`; set `invocation_jti` to it, `billing_mode_expected` to the current configured mode, and `renewed_from_jti` to the presented `jti` only when the presented token was the current one (a retry with `renewed_from_jti` keeps the value); change `version` exactly as M2-08's delivery does when it replaces `invocation_jti` (same repository function). Answer `200` `InvocationTokenGrant{invocation_token, expires_at}`. No event, no audit on success (Audit `denials`).

5. **Locks and concurrency.** One run row `FOR UPDATE` per operation; several runs in ascending `id` (03 6.3). Deadlocks are retried by the unit of work (`database.tx_retry_max`). Two starts with the same token give one transition and one replay; a start racing a cancel ends either `running` with a cancel request or `cancelled` with `409 already_started` for the start; a job and an adapter call on the same run serialize on the row.

6. **`awaitRunInput` and `finishRun`** (action `run.finish`, agent-self, Audit `no`):
   a. `awaitRunInput`: `running -> waiting_input` with `status_line` stored; on `waiting_input` `200` without change (06 2 "No effect"); from any other state `409 invalid_state_transition`. `trigger_state` stays `working`; `run.state_changed`.
   b. `finishRun` targets: `completed` from `running`; `failed` from `queued`, `running`, `waiting_input`; `cancelled` from `queued`, `running`, `waiting_input`. The same final state again answers `200` with the current `Run`, no new version, no event; another target on an ended run, or `completed` from `queued` or `waiting_input`, gives `409 invalid_state_transition`.
   c. Reasons (09 5.2): `failed` and `cancelled` need `reason`, else `400 validation_failed` (`errors[].code` `required`, pointer `/reason`). For `failed` the adapter reasons `billing_mode_mismatch`, `tool_class_mismatch`, `invalid_invocation_token`, `runtime_error`, `adapter_error`, `runtime_not_authenticated` are stored; every other code goes through M2-05's mapping to `adapter_error`. For `cancelled` only the reasons of a cancel request are accepted (`user_cancelled`, `access_revoked`, `agent_disabled`, `channel_read_only`, `user_disabled`), else `400 validation_failed` with pointer `/reason`. For `completed` a `reason` is not stored. Ruling for docs/DECISIONS.md: these three reason rules.
   d. `metrics`, when present, replaces `runs.metrics` (schema `RunMetrics`); `summary` goes to `runs.summary`. `costs` gives `400 validation_failed` with `errors[].code` `not_enabled` and pointer `/costs` until M2-22 books them into `cost_entries` (06 5 point 3). Ruling for docs/DECISIONS.md.
   e. `failed` with `billing_mode_mismatch` or `tool_class_mismatch` writes one audit entry in the same unit of work: action `run.finish`, decision `deny`, `deny_reason` that code, details `billing_mode_expected` and `tool_class` of the run as codes (10 14 row "I3 mismatch", reported by the adapter); count `bothy.agent.billing_mode_mismatch`. Ruling for docs/DECISIONS.md.
   f. Every end state goes through one end-of-run function in `internal/runs/app`: transition (`finished_at`, `duration_ms`, `trigger_state` `done`, `failed` or `cancelled` with `trigger_state_reason`), `run.state_changed`, metrics. M2-11 and M2-15 extend this function; leave a documented extension point, no empty stubs.

7. **`cancelRun`** (05 `run.cancel`: `trigger and (triggering person of the run or manage)`, `All(Level(trigger), Any(Self, Cap(manage)))` from M1-28; owners have both; an agent token fails with `forbidden`):
   a. Body `RunCancel.reason` is optional; when present it passes `textcheck.DisplayText` and has at most `audit.reason_max_chars` characters, else `400 validation_failed` with pointer `/reason`. It is stored nowhere: 03 `audit_events.details` allows free text only for the mandatory reason of an administrative action, and no table or event carries it. Ruling for docs/DECISIONS.md (spec gap in the report).
   b. `queued`: the run becomes `cancelled` with `user_cancelled` at once (03 allows `queued -> cancelled`; 09 5.1 names the person's `run.cancel` before the start), and `run.cancel_requested` is written as well, because the core contract keeps no delivery marker on a run; an adapter that never got the invocation ignores the request, one that holds it in its inbox drops it, and its later start gets `409 already_started`. Ruling for docs/DECISIONS.md; update 09 8 point 2.
   c. `running` or `waiting_input`: write `run.cancel_requested` and schedule `runs.cancel_timeout` at `now + agent.cancel_grace` with `InsertTx`; the state stays. A further cancel writes a further request and job (no pending marker in 03).
   d. Already `cancelled`: `202` with the current `RunSummary`, no event, audit with `no_change: true` (06 2 "No effect"); `completed` or `failed`: `409 invalid_state_transition`.
   e. Answer `202` with `RunSummary`. Audit allow entry in the transaction: action `run.cancel`, resource the run, details `state_before`, `immediate` (bool) and `cancel_reason` `user_cancelled`. `Idempotency-Key` is handled by the middleware of M1-20.
   f. `run.cancel_requested` (05 6): personal event to the agent (`AudiencePrincipalID = runs.agent_id`, receive action as in the catalogue), `scope {space_id, channel_id, thread_id}` of the thread now, `resource {kind: run, id}` without `version`, payload `RunCancelRequestedPayload {run_id, thread_id, reason, requested_by}`; `requested_by` only for `user_cancelled`. Ruling for docs/DECISIONS.md.
   g. `requestCancel(ctx, runID, reason, requestedBy)` in `internal/runs/app` is the one function behind 7b, 7c, 8 and 9; M2-16 and M8-03 reuse it.
   h. Ruling for docs/DECISIONS.md: `runs.cancel_timeout` is a job scheduled per request (`InsertTx`, `ScheduledAt = now + agent.cancel_grace`, args only the run id) instead of a periodic job, because 03 `runs` keeps no time of a cancel request; update 02 12.3. The handler sets a run that is still `queued`, `running` or `waiting_input` to `cancelled` with `cancel_timeout` (end-of-run function, no actor) and does nothing for an ended run; repeated execution is harmless (02 12.4 rule 1). After it, `startRun`, `resumeRun`, `awaitRunInput` and a `finishRun` with another state are refused (`409`).

8. **Revocation (R-SY-11)**: `OnAccessChanged(ctx, authorize.AccessChanged)` runs in phase 3.1 inside the revoking transaction (02 8.1), never calls `authorize` (the snapshot cache does not see uncommitted permissions, M1-29) and works only on the diff:
   a. For every principal in `Changes`, a channel is lost when it is in `Removed`, or in `Changed` with a new `level` below `trigger` (07 2.3). For `user.disable` every active run started by the person is affected.
   b. Load the active runs (`queued`, `running`, `waiting_input`) with `triggered_by` among these principals in ascending `id` `FOR UPDATE`, resolve each thread's current channel through `chat.Threads.Info` and keep the runs whose channel is lost.
   c. Reason by `AccessChanged.Action`: `user.disable` gives `user_disabled` (08 11 point 7, 02 4.3), `channel.archive` and `space.archive` give `channel_read_only` (09 5.2 "read-only or archived", 07 4.5), every other action gives `access_revoked`. Ruling for docs/DECISIONS.md.
   d. For every kept run: `run.cancel_requested` with that reason and `runs.cancel_timeout` scheduled (7c). The state stays, also for `queued`: undelivered calls fail at delivery (07 8.2 point 5) and a renewal fails at step 4 (AG-29), while a delivered token can no longer outlive `agent.cancel_grace`. Ruling for docs/DECISIONS.md.
   e. Add `user_disabled` to 09 5.2 and `channel_read_only` to `RunCancelRequestedPayload.reason` (Scope).

9. **Disabled agent**: `OnAgentDisabled(ctx, agents.AgentDisabled)` ends every active run of the agent at once with `cancelled` and `agent_disabled` through the end-of-run function and writes `run.cancel_requested` with `agent_disabled` (04 15.5); no timeout job, because the agent's tokens are revoked in the same transaction (09 2.1). Ruling for docs/DECISIONS.md.

10. **`runs.stall_check`** (periodic, `Interval = agent.presence_timeout`, the window of `interrupted`, instead of `run.stall_after` named in 02 12.3 today, because the stall warning is not built here (Out of scope); one unit of work per affected run, so no batch key is needed; no actor on its events). Ruling for docs/DECISIONS.md: the interval; update 02 12.3. It reads the persisted heartbeat of each agent with active runs through the `agents` port:
    a. `interrupted`: the run is `running` or `waiting_input`, the agent's last persisted heartbeat lacks it in `active_runs`, and that heartbeat is more than `agent.presence_timeout` after `runs.started_at`. Ruling for docs/DECISIONS.md: 09 4.8 compares "On every heartbeat", but only the last persisted heartbeat is visible to the worker (03 `agents`); update 09 5.5.
    b. `stale`: the run is `running` or `waiting_input`, the agent has no heartbeat or its last one is older than `run.orphan_timeout`, and `runs.started_at` is older than `run.orphan_timeout`.
    c. `agent_unavailable` for `queued` runs older than `run.queued_max_age` (09 5.5, D-AG-16): build it here only if M2-08 has no periodic check; never twice.
    d. Each outcome sets `failed` with its reason through the end-of-run function; no new run, no new `agent.invocation`, no retry (R-AG-17).
    e. Cross-check at load: `agent.heartbeat_persist_interval` below `run.orphan_timeout`, else start error naming both keys (the persisted heartbeat lags by up to the persist interval). Ruling for docs/DECISIONS.md.

11. **Permission version, events, audit.** No action here raises the permission version or writes `access.changed`. Events are written through `eventlog.Appender` only; `message.updated` for `trigger_state` comes from `messaging` (02 4.7). No event, log line, audit entry or problem body contains a token, a `jti` in clear or message content.

12. Rulings for docs/DECISIONS.md, one row each with reason: notes 1, 2a and 2c, 3b (tolerance, endpoint), 3e, 3g, 4d, 6c, 6d, 6e, 7a, 7b, 7f, 7h, 8c, 8d, 9, 10 (interval), 10a, 10e. Facts already in the spec (order of 4.5.1, replay, `superseded` for a consumed token, `key_type = unknown`) are not repeated there.

## Tests (write them first)

Unit:

- `internal/runs/domain`: `TestStartVerdict` with a row per branch of note 3b (replay after expiry, consumed in `waiting_input`, `already_started` for `running` and every end state, `superseded` by a newer delivery, `expired` exactly at `exp + auth.clock_leeway` is still valid and just after is expired, endpoint mismatch both ways, valid); `TestRenewVerdict` (current, renewed-from, consumed, replaced, `running` before a replaced `jti` gives `already_started`); `TestFinishDecision` (every transition of note 6b and every reason rule of 6c); `TestCancelDecision`; `TestCancelReasonFor` (`user.disable`, `channel.archive`, `space.archive`, `grant.revoke`, `grant.expire`, `group.member.remove`, `owner.remove`); `TestLostTriggerRuns` (removed channel, level changed to `read` and to `write`, capability-only change, change on another channel); `TestOrphanVerdict` (heartbeat exactly `agent.presence_timeout` after start is not yet `interrupted`, later is; run listed gives none; `stale` boundaries; no heartbeat at all; `queued` over `run.queued_max_age` if built here).
- `internal/runs/...`: `FuzzParseInvocationToken`: any input gives `invalid` or a parsed token, never a panic, never a valid verdict without a signature that verifies.
- `internal/authz`: `TestRunStartRequirement` (the run's agent with a valid token: allow; another agent: `forbidden` `not_self`; a human: `forbidden` `subject_kind_not_allowed`; invisible run: `not_found`; each of `expired`, `superseded`, `invalid`: `forbidden` `invocation_token_invalid` with `Detail`; configured and reported mode differ or `key_type = unknown`: `invariant_violation` `billing_mode_mismatch`; `host` against `scoped`: `tool_class_mismatch`; `chat` against `scoped`: allow); `TestRunTokenRenewRequirement` (valid and expired allow, the others deny); `TestDecisionErr_InvocationTokenInvalid` (problem code and `reason`, audit reason unchanged); `TestDeferredActionsGolden` (golden without `run.start` and `run.token.renew`); the source test of M1-28 for both new "Permission" texts.

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, HTTP through the real chain, agent tokens and people from the identity test helpers, fake clock; world: space Support with channel Allgemein whose operator is the agent Lotse (`organization`, tool class `scoped`), space Marketing with the agent Texter; Robin is the owner, Alex has `trigger` on Support, Sam's grants vary per test; a run is triggered by Alex and delivered through M2-08 and `pollAgentEvents`, so the adapter holds token T1):

- `TestStartRun_HappyPath`: `200` `Run` in `running`; `started_jti`, `started_at`, `billing_mode_used`, `billing_evidence`, `model` stored; `renewed_from_jti` NULL; `version` raised by one; `trigger_state = working`; one `run.state_changed`; `bothy.run.start.latency` with `started`.
- `TestStartRun_Replay`: the same T1 again gives `200` with `Idempotent-Replay: true`, also after the clock passed its `exp`; no second event; T1 after `awaitRunInput` gives `403 invocation_token_invalid` `superseded`.
- `TestStartRun_TokenInvalid_AG08`: empty token string (an absent field is `400 validation_failed` from the schema), garbled JWS, wrong signature, `alg` `none` and `HS256`, `typ` of an access token, unknown `kid`, `aud` of Texter, `run_id` of another run, other `iss`: each `403 invocation_token_invalid` with `reason: invalid` at `startRun` and at `renewInvocationToken`; the run stays `queued` and unchanged; one aggregated deny entry with `invocation_token_invalid` (AG-08 server part, AZ-P6, T-TK-05).
- `TestStartRun_Expired`: T1 after `agent.invocation_token_ttl` plus `auth.clock_leeway` gives `403 invocation_token_invalid` with `reason: expired`.
- `TestStartRun_BillingModeMismatch_BR06`: `billing_mode_used = personal` against `organization` gives `409 billing_mode_mismatch`; the run is `failed` with `billing_mode_mismatch`, `trigger_state = failed` with that reason; the deny entry carries configured mode, reported mode and evidence without values; the counter rises. Same with `billing_evidence.key_type = unknown` and a matching mode (AG-11 server part, BR-06).
- `TestStartRun_ToolClassMismatch_AG30`: `tool_class_used = host` against a token with `scoped` gives `409 tool_class_mismatch` and the run `failed` with `tool_class_mismatch`; `chat` is accepted (AG-30 server part).
- `TestStartRun_OtherDeliveryAlreadyStarted`: after a start with T2, T1 gives `409 already_started`.
- `TestResumeAndStartEndpoints`: `resumeRun` on `queued` and `startRun` on `waiting_input` with a current token give `409 invalid_state_transition`.
- `TestAG28_SlowStartRenewAndRestart`: Lotse acks T1; after `agent.ack_timeout` and `agent.invocation_token_ttl` no redelivery comes; `startRun(T1)` gives `403 invocation_token_invalid` with `reason: expired`; `renewInvocationToken(T1)` gives `200` T2; `startRun(T1)` gives `403 invocation_token_invalid` with `reason: superseded`; a restarted server instance on the same database with the consumer cursor before the event (as a restart within `agent.ack_persist_interval` leaves it) redelivers with T3; `startRun(T2)` gives `403 invocation_token_invalid` with `reason: superseded`; `startRun(T3)` gives `200`, again `200` replay; exactly one transition to `running`.
- `TestAG29_RenewRefusals`: a token replaced by a redelivery gives `403 invocation_token_invalid` with `reason: superseded`; renewal for a `running` run gives `409 already_started`; after Alex's grant is revoked, renewal gives `403 agent_invocation_denied` and the run is `failed` with the reason of M2-08's delivery check (`access_revoked`, AZ-M51), with its deny entry (AG-29 server part).
- `TestAG31_ConsumedStartToken`: start with S, `awaitRunInput`; `renewInvocationToken(S)` and `resumeRun(S)` both give `403 invocation_token_invalid` with `reason: superseded` and the run stays `waiting_input`; a start-allowing delivery prepared through M2-08's `Delivery` (trigger kind `interaction` or `followup`; if the merged port needs rows of M2-11 or M2-15, use the internal function it calls to issue the token and replace `invocation_jti`) yields A; `resumeRun(A)` gives `200` `running`, `started_at` unchanged.
- `TestAG32_LostRenewAnswer`: T1 expired; `renewInvocationToken(T1)` answer discarded (T2); retry with T1 gives `200` T3 and `renewed_from_jti` stays T1's `jti`; `startRun(T2)` gives `403 invocation_token_invalid` with `reason: superseded`; `startRun(T3)` gives `200`; one start; then `renewInvocationToken(T1)` gives `409 already_started`; after `awaitRunInput` it gives `403 invocation_token_invalid` with `reason: superseded`.
- `TestAwaitInput`: `running -> waiting_input` with `status_line`; again `200` without change; from `queued` `409 invalid_state_transition`.
- `TestFinishRun`: every transition and reason rule of note 6 over HTTP; a server-only reason such as `stale` from the adapter is stored as `adapter_error`; `costs` gives `400 validation_failed` with `errors[].code` `not_enabled`; `metrics` replaces; same final state again gives `200` without a new version or event; `bothy.run.finished` and `bothy.run.duration` recorded.
- `TestFinishRun_AdapterReportsMismatch_AG11`: `failed` with `billing_mode_mismatch` from `queued` ends the run, writes the deny entry of note 6e and counts the mismatch.
- `TestCancelRun_Queued`: `202` with state `cancelled`, `state_reason = user_cancelled`, `trigger_state = cancelled`; `run.cancel_requested` with `requested_by` Alex; a later `startRun(T1)` gives `409 already_started`; no timeout job.
- `TestCancelRun_Running_AG14`: `202` with state `running`; `run.cancel_requested` `user_cancelled`; one `runs.cancel_timeout` job scheduled at `agent.cancel_grace`; running the handler after the clock passed it gives `cancelled` with `cancel_timeout`; then `finishRun(completed)` and `awaitRunInput` give `409 invalid_state_transition`, `resumeRun` gives `409 already_started`, `finishRun(cancelled)` gives `200` without change (AG-14 server part).
- `TestCancelRun_AdapterConfirms`: `finishRun(cancelled, user_cancelled)` before the window gives `cancelled` `user_cancelled`; the later job changes nothing.
- `TestCancelRun_Rights`: Alex (starter) and Robin (owner) may; Sam with `trigger` but neither starter nor `manage` gets `403 forbidden`; Sam with `manage` may; Sam without `read` gets `404` like an unknown id; Lotse's token gets `403`; denials are audited, allows have one entry with the details of note 7e.
- `TestCancelRun_TerminalAndIdempotency`: on `cancelled` `202` without change and audit `no_change: true`; on `completed` `409 invalid_state_transition`; a replay with the same `Idempotency-Key` gives the stored answer and no second event; an invalid `reason` gives `400`.
- `TestRevocation_RSY11`: Alex has a `running` run in one thread of Allgemein and a delivered `queued` run in a second thread, and a run in Marketing through a separate grant; revoking her Support grant commits `run.cancel_requested` with `access_revoked` for both Support runs in the same transaction as her `access.changed` (same commit, same `correlation_id`), schedules two timeout jobs, leaves both states and the Marketing run untouched; Lotse's `finishRun(cancelled, access_revoked)` ends the first; the job ends the second with `cancel_timeout`. Variants: `disableUser` of Alex gives `user_disabled`; `archiveChannel` of Allgemein gives `channel_read_only`; a change that keeps her `trigger` (capability only) writes nothing.
- `TestAgentDisabled_CancelsRuns`: `disableAgent` of Lotse ends its active runs `cancelled` `agent_disabled` in the same transaction and writes `run.cancel_requested`.
- `TestStallCheck_Interrupted_AG10`: run started; the agents test helper persists a heartbeat without the run more than `agent.presence_timeout` after `started_at`; the handler sets `failed` `interrupted`, `trigger_state = failed`; the thread still has exactly one run and no new `agent.invocation` follows.
- `TestStallCheck_Stale`: no heartbeat for longer than `run.orphan_timeout` gives `failed` `stale` for `running` and `waiting_input`; a recent heartbeat listing the run changes nothing.
- `TestConcurrency`: parallel starts with the same token (one transition, one replay); parallel `cancelRun` and `startRun` on `queued` (never both `running` and `cancelled`); parallel job and `finishRun` (one end state).
- `TestEventsValidate`: every `run.state_changed` and `run.cancel_requested` row validates against `RunStateChangedEvent` and `RunCancelRequestedEvent`; no JSON `null`; no token or `jti` text in `event_log`, `audit_events` or captured logs.
- `TestWire_JobsAndSubscriptions`: `runs.cancel_timeout` on queue `maintenance`; `runs.stall_check` periodic with `agent.presence_timeout`, roles `worker` and `all` only; the golden registrations list both subscriptions after the existing ones.
- `TestConfig_HeartbeatPersistBelowOrphanTimeout`: a configuration with `agent.heartbeat_persist_interval` not below `run.orphan_timeout` aborts with a message naming both keys.
- `TestAZP1_RunRoutes`: the six operations are registered with their catalogue actions and classes, and the server starts.

Contract: responses of the six operations validate against `Run`, `RunSummary`, `InvocationTokenGrant` and `Problem` in the integration tests; problems carry `code` and, where named above, `reason`.

Red team focus of the milestone (PLAN 8 "Red team": forging or reusing the invocation token; faking the reported mode) is covered by `TestStartRun_TokenInvalid_AG08`, `TestStartRun_Replay`, `TestAG31_ConsumedStartToken`, `TestAG32_LostRenewAnswer` and `TestStartRun_BillingModeMismatch_BR06`; name them in the red team request.

Run `FuzzParseInvocationToken` once locally with `go test -run=^$ -fuzz=FuzzParseInvocationToken -fuzztime=<duration of your choice>` and report duration and result; commit any crashing input as a seed after the fix.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/runs/... ./internal/authz/... ./internal/agents/... ./internal/kernel/... ./internal/wire/... ./api/...`
- `make codegen-check`, `make arch`, `make api-lint`, `make api-checks`, `make api-tables`, `make register-check`
- `make config-reference` then `make config-doc-check`
- `make docs`
- `make check`

## Acceptance criteria

- [ ] `startRun`, `resumeRun`, `renewInvocationToken`, `awaitRunInput`, `finishRun` and `cancelRun` are enabled and registered; AZ-P1 passes; `run.start` and `run.token.renew` are real requirements and no longer in the deferred golden list.
- [ ] Every token error at start or renewal gives `403 invocation_token_invalid` with `reason` `expired`, `superseded` or `invalid` (AG-08 server part).
- [ ] A repeated start with the same token gives `200` with `Idempotent-Replay: true` while `running`; a consumed token gives `superseded` in `waiting_input`; another delivery's token gives `409 already_started`.
- [ ] BR-06 and AG-11 (server part): a different reported mode gives `409 billing_mode_mismatch` and a `failed` run with audit; AG-30 (server part): `tool_class_used` above `tool_class` gives `409 tool_class_mismatch` and a `failed` run.
- [ ] AG-28: slow start with expiry, renewal and redelivery after a restart starts the run exactly once.
- [ ] AG-29 (server part): renewal with a replaced token, of a running run and after the loss of `trigger` give `superseded`, `409 already_started` and `403 agent_invocation_denied` with a `failed` run.
- [ ] AG-31 and AG-32: a consumed start token resumes nothing; a lost renewal answer is recovered with `runs.renewed_from_jti`; the run starts once.
- [ ] AG-14: a stop without the adapter's answer ends `cancelled` with `cancel_timeout`, and later changes by the adapter are refused.
- [ ] R-SY-11: a loss of `trigger` writes `run.cancel_requested` in the revoking transaction (`access_revoked`, `user_disabled`, `channel_read_only`); a disabled agent's runs end `agent_disabled`.
- [ ] AG-10: a run missing from the adapter's `active_runs` ends `interrupted`, one without heartbeat ends `stale`, and none is retried.
- [ ] `run.cancel_requested` and `run.state_changed` validate against their schemas; no token in events, logs, audit or problems; no JSON `null`.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `runs`, `authz` and (if changed) `agents` updated (port methods, subscriptions, jobs, token facts, the end-of-run extension point, the cancel and orphan rules).
- Rulings of note 12 recorded in `docs/DECISIONS.md` section 10 (one row each with reason); the changes to 02, 09 and `openapi.yaml` of the Scope made in the same change.
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
