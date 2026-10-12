# M2-05: Runs, state machine and trigger state

Status: ready | Depends on: M2-04 | Milestone: M2

## Goal

A human message that the trigger decision of M2-04 turns into an invocation now creates a run: in the transaction of the message the server writes the run in `queued`, the live-box anchor message and `trigger_state = received` on the triggering message, or, on a rejection, no run and `trigger_state = failed` with the reason of 09 "Reasons for rejection at acceptance". Every later state change of a run goes through one compare-and-swap primitive on `state` and `version` that rejects every transition 03 `runs` does not allow, validates the reason against 09 "Reasons (`runs.state_reason`)" (an unknown adapter reason becomes `adapter_error`), appends `run.state_changed` and moves `trigger_state` as 09 "Trigger state (`trigger_state`)" maps it. At most one active run exists per thread (R-DM-14), and of two concurrent starts exactly one wins. People read runs, steps and "Running now" over `listThreadRuns`, `getRun`, `listRunSteps` and `listActiveRuns`, filtered by `run.read`, and the snapshot names the running runs through the port that M1-63 wired empty. This is the run object of ADR 0025 on which delivery (M2-08), start, finish and cancel (M2-09), patches (M2-10) and follow-ups (M2-11) build.

## Read first

- `docs/spec/09-agents.md`: 3 "Operator and trigger rules" with 3.1 "Operator", 3.2 "Origin (`messages.origin`)", 3.4 "Decision per message" (point 7 is the acceptance this ticket builds), 3.5 "Reasons for rejection at acceptance", 3.6 "Guest agents", 3.7 "Auto-thread from a channel message"; 5 "Run" with 5.1 "State machine", 5.2 "Reasons (`runs.state_reason`)" (last sentence: unknown adapter reasons), 5.3 "Run document (`run_document.v1`)", 5.4 "Live box content" (anchor message); 6 "Trigger state (`trigger_state`)"; 16.2 "Required scenarios" (AG-03 to AG-06, AG-09 for orientation); 19 R-AG-03, R-AG-05, R-AG-09.
- `docs/spec/03-data-model.md`: `runs` (all columns, checks, the partial unique index, the transition list after the indexes), `run_steps`, `run_followups` (only to know it comes with M2-11), `messages` (`kind = 'run'`, `run_id` with the deferred FK, `trigger_state`, `trigger_state_reason`, `trigger_state_at`, `silent`, `version`, the checks), `threads` (`operator_agent_id`, `parent_message_id`), `agent_sessions` (module `agents`, M2-12), 1 "Principles" points 1, 10, 11, 12, 13; 6.3 "Lock order"; 8 R-DM-12, R-DM-14, R-DM-21.
- `docs/spec/04-sync-protocol.md`: 2.2 "Envelope"; 6.3 "Lifecycle" step 4 (the snapshot names running runs); 9 "Delivery guarantee" point 3; 10 "Ordering" (per thread strictly by `thread_seq`); 11 "Live box streaming" (11.1, 11.3 point 1: every state change is a durable `run.state_changed`); 15.3 "Invocation and run start" steps 1, 2 and 6; 15.4 "Follow-ups" point 1; 16.5 "Agent run with follow-up" (the `201` already carries `trigger_state received`).
- `docs/spec/05-actions.md`: 4.10 "Agents, commands, runs, interactions" (rows `agent.invoke`, `run.read`, `run.overview.read`); 5 "Agent invocation `agent.invoke`" (points 1 to 8, "Consequences on `allow`", "Consequences on denial"); 6 "Durable event types and receive action" (rows `run.state_changed`, `message.updated`, `thread.created`).
- `docs/spec/02-architecture.md`: 4.3 "Ports, events and sources per module" (rows `runs`, `messaging` with `SetTriggerState` and anchor, `sync`, `authz` with the reason `user_disabled`); 4.4 "Allowed dependencies"; 4.7 "Event types and WebSocket ops per module" (last paragraph: `message.updated` for `trigger_state` comes from `messaging`); 5 "Layers within a module" (rule 5: metrics port); 8.1 "Flow", 8.2 "Lock order", 8.3 "In-Tx events" (creating a run is a consequence that must be atomic with its cause); 9.2 "404 instead of 403"; 9.3 "Proof of the check" (lists prove the route action through `Scope`).
- `docs/spec/07-permissions.md`: 4.1 "Decisions and reasons" (deny reasons `level_insufficient`, `agent_not_reachable`, `billing_mode_personal_not_holder`, `tool_class_owner_only`, `project_not_ready`, `dev_environment_not_ready`, `run_active_other_agent`); 4.2 "Flow" (filter mode); 4.3 "Visibility"; 7.4 "Query pattern" rules 1, 3 and 6; 10 "Examples" and 11.1 "Test matrix" (test world, AZ-M30); 11.2 AZ-P4, AZ-P5.
- `docs/spec/06-api.md`: 5 "Errors" (codes `not_found`, `cursor_invalid`, `validation_failed`, `invalid_state_transition`, `already_started`); 6 "Pagination" points 1 to 4; 8 "Concurrency" (ETag); 16 "Endpoints", table "Runs".
- `docs/spec/10-backend-policy.md`: 12.2 "Display" (amounts per billing mode belong to M2-22; this ticket returns stored values).
- `docs/api/openapi.yaml`: operations `listThreadRuns`, `getRun`, `listRunSteps`, `listActiveRuns`, `createMessage`, `getSnapshot`, `listEvents`; schemas `Run`, `RunSummary`, `RunSummaryPage`, `RunStep`, `RunStepPage`, `RunState`, `RunFollowup`, `RunMetrics`, `RunPlan`, `Money`, `TriggerState`, `Message`, `MessagePayload`, `RunStateChangedEvent`, `RunStateChangedPayload`, `MessageUpdatedEvent`, `Snapshot`, `Problem` (`x-problem-codes`); parameters `ThreadIdPath`, `RunIdPath`, `Cursor`, `Limit`.
- `docs/api/asyncapi.yaml`: channel `gateway`, message `event.run.state_changed`. `docs/api/webhooks.md` is not changed by this ticket.
- ADR 0025 (run as its own object; "Confirmation" names the state machine test), ADR 0017 (one authorization function, 404 instead of 403), ADR 0009 (commit-ordered event log), ADR 0053 (no JSON `null`), ADR 0007 (migrations), ADR 0050 (module register), ADR 0052 (one envelope).
- `docs/observability/signals.md` 4.3 "Agents and runs": `bothy.run.active`, `bothy.run.start.latency`, `bothy.run.duration`, `bothy.run.finished`; `bothy.authz.errors` with `bothy.reason = filter_mismatch` (07 7.4 rule 3). `docs/observability/alerting.md` is not changed.
- `docs/operations/configuration.md`: `api.page_size_default`, `api.page_size_max`, `run.step_detail_max_bytes`, `authz.not_found_timing_tolerance`.
- `docs/security/threat-model.md`: T-AZ-01 (route without check), T-AZ-02 (leak over counters, lists or timing), T-AZ-04 (who triggered: `runs.triggered_by`), 5.10 "Agent adapters and runtime". `docs/RISKS.md`: RK-18 (a run executes twice; the start is a compare-and-swap). `docs/OPEN-QUESTIONS.md`: no open question blocks this ticket; OQ-22 (default: spec-first according to ADR 0013) applies to the generated handlers.
- `docs/DECISIONS.md`: D-G-280 (`active_runs` empty until M2-05), D-W-100 (M2 briefs written before M1 closed), section 10 format of `D-M2-05-<n>`.
- `docs/PLAN.md`: section 3 "Way of working per ticket"; section 8 "M2 Agent platform, extensions and terminal" (Goal, Scope, Exit criteria point 1, Test strategy, Red team, Risks); rows M2-02, M2-03, M2-04, M2-05, M2-06, M2-08, M2-09, M2-10, M2-11, M2-12, M2-15, M2-16, M2-22 and M1-63.
- Dependencies: the header names M2-04 only. Implied through it: M2-02 (channel behaviour), M1-56 (`MessagePosted`, `createMessage`), M1-54 (operators), M1-55 (`chat.Threads`), M1-51 (agents and `AgentPolicySource`), M1-28 (`authorize`, `Scope`, filter mode, `agent.invoke` guard), M1-24 (audit recorder), M1-18 (cursor codec, problem details), M1-17 (`api/operations.yaml`), M1-16 (`eventlog.Appender`), M1-13 (unit of work, `txn.Publish`, `txn.Subscribe`), M1-12 (`pgtest`), M1-10 (meter provider). Needed but not implied: M1-63 (`RunSummarySource` and its `noruns` binding), see note 0.
- M1 briefs whose interfaces this ticket uses: `M1-13-unit-of-work.md`, `M1-16-event-log.md`, `M1-18-http-chain.md`, `M1-28-authz-core.md`, `M1-51-agents-records.md`, `M1-55-chat-threads.md`, `M1-56-messaging-send.md` (D1, D3, D6, D7 and the deferred FK `messages.run_id`), `M1-57-messaging-history.md` (per-item filter pattern), `M1-61-sync-personal-data.md` (subscriber order), `M1-63-sync-catch-up.md` (note 9 `active_runs`), `M1-68-core-metrics.md` and `M1-70-leak-tests.md` (only if merged on the base: metrics registry and the SQL visibility lint).
- Code (read the merged state; `git ls-tree -r --name-only <base> services/server/internal/<module>`): `internal/runs` (port, `domain` from M2-04), `internal/messaging`, `internal/chat`, `internal/agents`, `internal/sync` (`RunSummarySource`, `adapters/noruns`), `internal/kernel/authorize`, `internal/kernel/txn`, `internal/kernel/eventlog`, `internal/kernel/errs` (`CodeInvalidStateTransition`, `CodeAlreadyStarted`), `internal/platform/httpserver` (cursor codec), `internal/platform/pg/pgtest`, `internal/wire` (txn registrations golden file, `configkeys_*.go`), `architecture/modules.yaml`, `api/operations.yaml`, `sqlc.yaml`.

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

11. The run states and transitions are exactly those of 03 `runs`: `queued -> running | failed | cancelled`, `running -> waiting_input | completed | failed | cancelled`, `waiting_input -> running | failed | cancelled`; every other pair (self loops and anything out of an end state included) is rejected with `409 invalid_state_transition` and changes nothing. Each transition is a compare-and-swap on `state` and `version` (03 `runs`, 09 5.1, ADR 0025, RK-18).
12. `failed` and `cancelled` carry a reason from 09 5.2; the other states carry none. An adapter reason that 09 5.2 does not list for the adapter is stored as `adapter_error`, with the original code kept in the details (09 5.2, last sentence).
13. `trigger_state` of the triggering message follows 09 6: `received` at acceptance, `working` at the start, `done` at `completed`, `failed` and `cancelled` with `trigger_state_reason` equal to the run reason; every change raises `messages.version` and writes `message.updated`, produced by `messaging` through `Messages.SetTriggerState` (03 `messages`, 02 4.7, 09 6).
14. At most one run in `queued`, `running` or `waiting_input` per thread, guarded by the partial unique index (03 `runs`, R-DM-14).
15. Acceptance (09 3.4 point 7, 04 15.3 step 1, 05 section 5 "Consequences"): in the transaction of the message, on `allow` of `agent.invoke`, a run in `queued`, the live-box anchor (`kind = 'run'`, author the agent, `origin = 'agent'`, empty content, no notification, 09 5.4) and `trigger_state = received`; with `auto_thread` in the main thread the run, the anchor and later answers live in a new thread with `parent_message_id` = the message (09 3.7). On a rejection the message stays, gets no run and gets `trigger_state = failed` with the reason (09 3.5). Rejections of `agent.invoke` are audited (05 4.10, Audit `denials`).
16. Every state change of a run, `queued` at creation included, appends the durable event `run.state_changed` (resource `run` with the new version, receive action `run.read`, payload `RunStateChangedPayload`) (04 11.3 point 1, 05 section 6, 04 9 point 3).
17. Reads: `listThreadRuns`, `getRun` and `listRunSteps` need `run.read`; `listActiveRuns` needs `run.overview.read` and lists only runs the caller may read with `run.read`; all use rate-limit class `rl.read` (05 4.10, `openapi.yaml`). A run or thread without read right gives the same `404` as an unknown ID (R-AZ-01, 02 9.2, AZ-P4).
18. Lists filter through the visible channels of `Authorizer.Scope` and then `authorize` per element; no totals (07 7.4 rules 1 and 3, 06 6 point 4, AZ-P5, T-AZ-02).
19. The snapshot lists the active runs the actor may read as `RunSummary` (04 6.3 step 4, `Snapshot.active_runs`).
20. No JSON `null` anywhere: an absent column value is an absent field (03 principle 13, ADR 0053).
21. Run metrics use exactly the names, units and attributes of signals.md 4.3.

## Scope

All paths under `services/server/` unless they start with `docs/`.

Create:

- `migrations/<next number>_runs_runs.sql` and `migrations/<next number + 1>_runs_run_steps.sql` (plan: migrations `runs`, `run_steps`), with the deliberate deviations of note 1, each commented in the file.
- `internal/runs/domain/state.go` (states, `CanTransition`, active and end states), `internal/runs/domain/reason.go` (the reason table of 09 5.2 plus `user_disabled`, setter per reason, `NormalizeAdapterReason`), `internal/runs/domain/triggerstate.go` (mapping of note 4), with tests. M2-04's decision function stays where M2-04 put it.
- Port additions in `internal/runs` (note 3): interface `Runs` with the read methods, DTOs `Run`, `RunSummary`, `RunStep`, pages and queries.
- `internal/runs/app/lifecycle.go` (`Open`, `Transition`), `internal/runs/app/accept.go` (`OnMessagePosted`), `internal/runs/app/read.go`, `internal/runs/app/metrics.go` (outbound metrics port with a no-op implementation), `internal/runs/app/ports.go` (repository and clock interfaces).
- `internal/runs/adapters/postgres/queries/runs.sql`, `internal/runs/adapters/postgres/queries/run_steps.sql`, the sqlc output and the repository.
- `internal/runs/adapters/httpapi/runs.go`: strict-server methods for `listThreadRuns`, `getRun`, `listRunSteps`, `listActiveRuns`.
- `internal/runs/adapters/otelmetrics/metrics.go` (note 11).
- `internal/sync/adapters/runsummary/`: implementation of M1-63's `RunSummarySource` over the `runs` port (note 9).
- `internal/wire/configkeys_runs.go` if `run.step_detail_max_bytes` is not yet registered (note 12).

Change:

- `internal/messaging` (port, app, adapters): `Messages.SetTriggerState` and `Messages.CreateRunAnchor`; `MessagePosted.Kind` documents `run`; the order "append `message.created`, then publish `MessagePosted`"; the `201` and replay bodies of `createMessage` built from the stored row after the unit of work (note 7).
- `internal/chat` port, additive and only if absent: `Threads.Locate(ctx, threadIDs []ids.ID) ([]ThreadInfo, error)` with one query `id = ANY($1)` (note 8).
- `internal/wire`: handlers; the subscription `messaging.MessagePosted -> runs.OnMessagePosted` after the existing subscribers; the binding of `RunSummarySource` to `internal/sync/adapters/runsummary`; the metrics adapter and the observable gauge in role `worker`; update `testdata/txn_registrations.golden`.
- Remove `internal/sync/adapters/noruns` and its tests once nothing references it.
- `api/operations.yaml`: move `listThreadRuns`, `getRun`, `listRunSteps`, `listActiveRuns` from `later` to `enabled`; rerun the route generator.
- `sqlc.yaml`: entry for `runs`. `architecture/modules.yaml`: the vendor groups the new `runs` adapters need in `adapter_vendors` (as the merged modules do for postgres and OpenTelemetry adapters); regenerate `.go-arch-lint.yml` with `go generate ./tools/archgen`.
- `docs/spec/07-permissions.md` 7.4 rule 1 (note 10) and `docs/spec/09-agents.md` 3.4 (note 6 step 4) and 5.2 (note 2), in English like the chapters.
- `docs/adr/0025-run-as-its-own-object-for-the-live-box.md`: add the test names of this ticket to "Confirmation" without changing the decision.
- If `tools/lint/sql_visibility.py` (M1-70) is on the base: its exception list and category set (note 10).
- READMEs of `runs`, `messaging`, `chat`, `sync`; `services/server/CHANGELOG.md`; `docs/DECISIONS.md` section 10 rows `D-M2-05-<n>`.

No new top-level Makefile target.

## Out of scope

- Writing the personal event `agent.invocation` at acceptance and delivering it, the checks before delivery (`agent.invoke`, I2, SR2, SR4 with current rights), the fresh invocation token, `trigger_state = queued` at the first delivery, AG-07, AG-27, AZ-M30 and AZ-M51 as owned tests: M2-08 (reason: ruling 1 of note 14).
- The invocation token with EdDSA and JWKS: M2-06. Agent consumers and acknowledgements: M2-07.
- `startRun`, `resumeRun`, `renewInvocationToken`, `awaitRunInput`, `finishRun`, `cancelRun`, the jobs `runs.cancel_timeout` and `runs.stall_check`, writing the columns `started_jti`, `renewed_from_jti`, `billing_mode_used`, `billing_evidence`, the cancel subscribers (`authorize.AccessChanged`, `agents.AgentDisabled`, `chat.ChannelReadOnlySet`) and the reason rule for a `cancelled` reported by the adapter: M2-09. They call `Transition` of this ticket.
- `patchRun`, `run.update`, `run.updated`, steps written by adapters, step compaction at `run.max_steps_persisted`, `bothy.run.steps`: M2-10.
- The table `run_followups`, the follow-up outcome of the acceptance, `followup_delivered`, `deferred` and the next run from deferred follow-ups: M2-11. Until then `Run.followups` is an empty object.
- `agent_sessions`, `runs.agent_session_id` and its FK: M2-12. `getRunContext` and `run.context.read`: M2-13. SR9 `acting_for` in `Authorize` and `Scope`: M2-14.
- `message_components`, `interactions`, the FK of `runs.triggering_interaction_id`, `trigger_kind = interaction`, the `messaging.MessageDeleted` subscriber and `messaging.ComponentReader`: M2-15.
- `commands`, the FK of `runs.triggering_command_id`, `trigger_kind = command`, `run_active`: M2-16. Auto title of the new thread: M2-18. `MessageCreate.run_id` for agent answers: M2-19. Heartbeat and `interrupted` from `active_runs`: M2-20.
- Cost bookings, `cost.read.thread`, hiding amounts in mode `personal` (BR-11): M2-22. Project binding readiness: M2-03 (until then `StructureSource` reports no binding).
- Attachments and the hold of an invocation (`ReleaseInvocation`, `files.FileFinalized`): M3-07; voice messages and `voice_unsupported`: M3.
- `kernel/personaldata.Participant` for runs: M7-06. FK of `runs.dev_environment_id` and SR3: V2-01.
- The conformance suite AG-01 to AG-30: M2-31; load tests of the agent path: M2-36; statistics over runs (`getRunStats`): M6.
- Gateway delivery code: unchanged; the hub of M1-65 delivers `run.state_changed` like every durable event.
- Changes to the core contract.

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M2-05-runs-state-machine`; one writer per worktree.
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
- No agent adapter, agent runtime, model or third-party service takes part in any test: runs are driven through the port and the HTTP chain only; real runtimes run only behind an explicit switch (09 16.1), and this ticket has none.
- Existing M1 tests whose worlds now trigger runs (note 13) are adapted to the specified behaviour, never deleted, skipped or weakened; each adaptation is listed in the report.

## Design notes

0. Start check: `make check` is green on the fresh worktree, else `Verdict: BLOCKED base branch red`. M2-04 is merged (its decision function exists in `internal/runs/domain`), else `Verdict: BLOCKED dependency M2-04 missing`. M1-63 is merged (`RunSummarySource` and `internal/sync/adapters/noruns` exist), else `Verdict: BLOCKED dependency M1-63 missing`: the plan row requires its port, but M1-63 is not in the dependency closure of this ticket.

1. Migrations.
    - `runs` exactly as 03: all columns with types, defaults and nullability; checks `state IN (...)`, `trigger_kind IN ('message','command','interaction')`, `billing_mode_expected IN ('personal','organization')`, end states have `finished_at`, at most one of the three `triggering_*` columns set and only the one matching `trigger_kind`; unique `invocation_jti`; partial unique index on `(thread_id) WHERE state IN ('queued','running','waiting_input')` (R-DM-14); indexes `(thread_id, queued_at DESC)`, `(state, agent_id) WHERE state IN ('queued','running','waiting_input')`, `(agent_id, finished_at DESC)`. FKs now: `thread_id`, `agent_id -> agents(principal_id)`, `triggering_message_id -> messages(id) ON DELETE SET NULL`, `triggered_by -> principals(id)`, with the `ON DELETE` of 03.
    - Deliberate deviations, commented: the FKs of `agent_session_id` (table of M2-12), `triggering_command_id` (M2-16), `triggering_interaction_id` (M2-15) and `dev_environment_id` (V2-01) are added by the ticket that creates the referenced table; the columns exist now.
    - Same file: `ALTER TABLE messages ADD CONSTRAINT messages_run_id_fkey FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE SET NULL DEFERRABLE INITIALLY DEFERRED` (03 marks it "deferred" like `threads.parent_message_id`; M1-56 left it to this ticket).
    - `run_steps` exactly as 03 (unique `(run_id, ordinal)`, partial index `(run_id) WHERE status = 'running'`, FKs with `ON DELETE CASCADE`).
    - Retention class `content` for both, as the module register and 03 state.

2. Domain (`internal/runs/domain`, pure, `kernel-core` only).
    - `type State string` with the six values of `RunState`; `Active()` for `queued`, `running`, `waiting_input`; `Ended()` for the three end states; `CanTransition(from, to State) bool` true exactly for the nine pairs of requirement 11.
    - Reason table, one row per code of 09 5.2 with its state (`failed` or `cancelled`) and its setter (server, adapter, or both), plus `user_disabled` (`cancelled`, server), which 02 4.3 names for runs cancelled by `user.disable` (ruling 7). `ValidateServerReason(to, code) error`: the code must be a server or both row of state `to`; anything else is an internal error (a programming error of the caller). `NormalizeAdapterReason(code string) (reason, original string)` for `failed`: a code whose row has the adapter as setter (`billing_mode_mismatch`, `tool_class_mismatch`, `invalid_invocation_token`, `runtime_error`, `adapter_error`, `runtime_not_authenticated`) is kept with an empty original; every other value, including a server-only code such as `stale` or `interrupted`, gives `adapter_error` and the original code (ruling 8). An empty code is `ErrReasonRequired` (M2-09 maps it to `400 validation_failed`, pointer `/reason`). A reason for `running`, `waiting_input` or `completed` is an internal error. Add the row `user_disabled` and the place of the original code (rulings 7 and 8) to 09 5.2 in English.
    - `OriginalReasonDetail(code string, maxBytes int) map[string]any`: `{"original_reason": code}` after `kernel/textcheck` (no control or bidi characters), cut at the longest valid UTF-8 prefix so the encoded detail fits `run.step_detail_max_bytes`; a code that `textcheck` rejects gives `{}`.
    - Trigger mapping (note 4) as a pure function.

3. Port `internal/runs` (DTOs, never domain types; names may follow the merged style):

```go
// Runs is the read side of the runs module for handlers and other modules.
type Runs interface {
    // Get returns the run document for the actor in ctx; an unknown or invisible run gives the
    // same not_found error.
    Get(ctx context.Context, runID ids.ID) (Run, error)
    // ListByThread pages the runs of one thread, newest first.
    ListByThread(ctx context.Context, q ThreadRunsQuery) (RunSummaryPage, error)
    // ListSteps pages the steps of one run by ordinal.
    ListSteps(ctx context.Context, q RunStepsQuery) (RunStepPage, error)
    // ListActive pages "Running now": runs in queued, running or waiting_input the actor may read.
    ListActive(ctx context.Context, q ActiveRunsQuery) (RunSummaryPage, error)
    // ActiveForActor lists all active runs the actor may read, for the snapshot.
    ActiveForActor(ctx context.Context) ([]RunSummary, error)
}
```

   `RunSummary` mirrors the schema `RunSummary`, `Run` the schema `Run` (`steps` keyed by step ID, `followups` keyed by message ID). `Open` and `Transition` (note 5) are exported from `internal/runs/app` for the later tickets of this module (M2-08, M2-09, M2-11, M2-15, M2-16), not from the port: no other module changes run states.

4. Trigger state mapping (09 6), applied when the run has `triggering_message_id`:

    | Event of the run | `trigger_state` | `trigger_state_reason` |
    |---|---|---|
    | created (`queued`, acceptance) | `received` | none |
    | `queued -> running`, `waiting_input -> running` | `working` | none |
    | `running -> waiting_input` | unchanged (`working` covers `waiting_input`) | |
    | `-> completed` | `done` | none |
    | `-> failed` | `failed` | the run's `state_reason` |
    | `-> cancelled` | `cancelled` | the run's `state_reason` |

   `queued` after the first delivery (M2-08) and `followup_delivered` (M2-11) are set by those tickets through the same `SetTriggerState`. Runs without a triggering message (command, interaction) change no `trigger_state`.

5. Lifecycle (`internal/runs/app/lifecycle.go`), always inside the caller's unit of work (`txn.InUnit`, else internal error).
    - `Open(ctx, OpenCmd{ThreadID, AgentID, TriggeredBy, TriggeringMessageID ids.ID; TriggerKind string; BillingModeExpected string}) (RunSummary, error)`:
        1. `INSERT INTO runs (...) VALUES (...) ON CONFLICT (thread_id) WHERE state IN ('queued','running','waiting_input') DO NOTHING RETURNING ...` with a new UUIDv7 `id`, `state = 'queued'`, `version = 1`, `invocation_jti` = a new UUIDv7 that no token carries (ruling 4; M2-08 replaces it at the first delivery), `billing_mode_expected` from the agent's current policy (ruling 5). No row returned: load the active run of the thread and return `ErrActiveRunExists{Active RunSummary}`; nothing else is written.
        2. `messaging.Messages.CreateRunAnchor` in the run's thread (note 7).
        3. Append `run.state_changed` (note 5 last bullet).
        4. `messaging.Messages.SetTriggerState(M, received)` when `TriggeringMessageID` is set.
    - `Transition(ctx, TransitionCmd{RunID ids.ID; From, To domain.State; Version int64; Reason string; SetBy domain.Setter}) (Transitioned, error)`:
        1. `CanTransition(From, To)` false: error with code `invalid_state_transition` (`409`), nothing written.
        2. Reason: server setter through `ValidateServerReason`; adapter setter with `To = failed` through `NormalizeAdapterReason`; any other combination is an internal error until M2-09 defines the adapter rule for `cancelled`.
        3. `UPDATE runs SET state = $to, state_reason = $reason (NULL unless failed or cancelled), version = version + 1, started_at = COALESCE(started_at, now()) when $to = running, finished_at = now() and duration_ms = milliseconds between started_at and finished_at (NULL when never started) when $to is an end state WHERE id = $id AND state = $from AND version = $version RETURNING ...`. No row: load the run; missing gives `not_found`; otherwise `ErrConcurrentChange{Current RunSummary}`, whose default mapping is `409 invalid_state_transition`; M2-09 maps a lost start race to `409 already_started` or to the replay of the same token. Later tickets add their columns to this same statement (M2-09: `started_jti`, `renewed_from_jti`, `billing_mode_used`, `billing_evidence`, `model`, `summary`, `metrics`).
        4. When an adapter reason was replaced: insert one `run_steps` row, server-generated UUIDv7, next `ordinal` of the run (under the row lock the `UPDATE` holds), `kind = 'status'`, `name = 'adapter_reason'`, `status = 'failed'`, `detail = OriginalReasonDetail(...)`, `finished_at = now()` (ruling 8). This goes through the one step-append function that M2-10 later extends with compaction.
        5. Append `run.state_changed`; then `SetTriggerState` per note 4.
        6. Return `Transitioned{Run RunSummary; From, To State; QueuedFor, Ran time.Duration; Started bool}`; the unit owner records metrics after the commit (note 11).
    - Locks: the acceptance runs inside the unit of `createMessage`, which holds the thread row of the message (M1-55 `AppendToThread`); `Open` then writes `runs`, the anchor appends to the target thread (the same row or a new one), and `SetTriggerState` updates `messages` last. `Transition` locks the `runs` row through its `UPDATE` before it touches `run_steps` and `messages`. Both orders take `runs` before `messages`; deadlocks are retried by the unit of work (`database.tx_retry_max`).
    - `run.state_changed`: `type` `run.state_changed`, `schema_version` 1, `actor` of the unit, `scope {space_id, channel_id, thread_id}` of the run's thread now (from `chat.Threads.Info`; a run follows its thread when the thread moves, M1-62), `resource {kind: run, id, version: runs.version}`, receive action `run.read`, no audience, payload `RunStateChangedPayload {run: RunSummary, triggering_message_id, trigger_state}` with the absent fields omitted. `RunSummary.step_count` counts `run_steps` of the run; `cost` is `Money{amount_micros: cost_micros, currency, is_estimate: cost_is_estimate}`.

6. Acceptance (`internal/runs/app/accept.go`, `OnMessagePosted(ctx, messaging.MessagePosted) error`), registered after the existing subscribers of `MessagePosted`. Business outcomes never veto: only infrastructure errors return an error (they roll back the message, which 02 8.3 intends).
    1. Return without writing unless `Origin = human` and `Kind = text` (system lines and anchors never trigger, 09 3.2; voice comes with M3). DM channels give no trigger (09 3.4 point 2).
    2. Gather the facts M2-04's input type asks for, through ports only: thread and channel kind, fixed operator of a non-main thread or effective operator of the channel (`chat`), the channel switches of 09 3.3 (the chat port of M2-02), mentioned agents among `MentionedIDs` (`agents` port), agent status, billing mode, credential holder and tool class (`agents` port), the active run of the target thread (this module's repository). Facts about rights come from `authorize` in filter mode (`authorize.Filter(ctx)`), never from a query of grants.
    3. Run M2-04's decision. Outcomes:
        - no trigger: return.
        - follow-up to the active run of the same agent: return without writing; M2-11 adds the follow-up here (ruling 12).
        - rejection `multiple_agents_mentioned`: `SetTriggerState(M, failed, multiple_agents_mentioned)`; no `authorize` call and no audit, because there is no single agent to check (ruling 12).
        - a candidate agent A (invocation, or any other rejection that names A): continue with step 4.
    4. Trigger right without mention (09 3.4 point 4, R-AG-05): when M does not mention A, call `authorize(P, agent.invoke, d)` in filter mode first; a denial with reason `level_insufficient` means P lacks `trigger`: return without writing and without audit (ruling 13). `d` = `{Kind: thread, ID: thread of M, Invoke: {AgentID: A, TriggerOrigin: human, MentionsAgent, ActiveRunAgentID: agent of the active run of the target, or zero}}`. For the auto thread the descriptor names the thread of M, not the new thread, so the mention rule of 09 3.4 point 3 and the guard of M1-28 judge the same thread, and the new thread is created only on `allow` (ruling 2). Write this order into 09 3.4 in English: the check of point 7 names the thread of M, and the thread of point 5 is created only after its `allow`.
    5. `authorize(P, agent.invoke, d)` in request mode: it is the truth and its recorder writes the denial audit (05 Audit `denials`). Map a denial to `trigger_state_reason`: `level_insufficient` gives `no_trigger_right`; `agent_not_reachable` gives `agent_disabled` when A's principal status is not `active`, else `agent_not_allowed_here`; `billing_mode_personal_not_holder`, `tool_class_owner_only`, `project_not_ready`, `dev_environment_not_ready` and `run_active_other_agent` stay as they are; any other reason is stored unchanged and logged at error level without IDs. Then `SetTriggerState(M, failed, reason)`; no run, no new thread.
    6. M2-04 and `authorize` must agree. When M2-04 rejected but `authorize` allows, fail closed: no run, `trigger_state = failed` with M2-04's reason, and a log entry `trigger_decision_mismatch` without IDs and contents (ruling 14). When M2-04 invoked but `authorize` denies, step 5 applies.
    7. On `allow`: with the auto thread, `chat.Threads.Create(ctx, CreateThreadCmd{ChannelID: C, Parent: &ParentMessage{MessageID: M, ThreadID: main thread}})` without title (09 3.7: `title = ''`, `title_source = 'auto'`, operator = effective operator of C); target = the new thread, else the thread of M. Then `Open` with `TriggerKind = message`, `TriggeringMessageID = M`, `TriggeredBy = P`. `ErrActiveRunExists` (another path opened a run in the meantime) re-runs steps 3 to 5 with that active run: the same agent gives the follow-up no-op, another agent gives `run_active_other_agent` through `authorize`.
    8. Event order inside the unit, fixed by test: `message.created` (M), then for the auto thread `thread.created`, then `message.created` (anchor), `run.state_changed` (`queued`), `message.updated` (M with `trigger_state = received`). Per thread the events follow `thread_seq` (04 10).

7. `messaging` changes.
    - Order: `createMessage` appends the `message.created` events of the unit (system line, then the message) before it publishes `MessagePosted` (system line first, then the message). Reason: a subscriber that appends a message to the same thread (the anchor) must not overtake the message in the log (04 10, ruling 3). Adapt M1-56 and M1-61 tests that pinned the old order to this order.
    - The `201` body and the replay body of `createMessage` are the stored row after the unit of work, through the single representation function of M1-56, so they carry `trigger_state`, `trigger_state_at` and the version after the acceptance (04 16.5).
    - `SetTriggerState(ctx, SetTriggerStateCmd{MessageID ids.ID; State string; Reason string}) error`: inside the ambient unit only; no `authorize` (the caller decided; doc comment says so). `State` is a `TriggerState` value; `Reason` only for `failed` and `cancelled` (03 check); the message must have `origin = 'human'` (03 check; otherwise internal error). Same state and reason as stored: no write, no event (ruling 17). Otherwise set the three columns (`trigger_state_at = now()`), raise `version` by one and append `message.updated` (resource `message` with the new version, receive action `message.read`, payload `MessagePayload` with the representation and truncation rule of M1-56 D7; a deleted message keeps its tombstone representation).
    - `CreateRunAnchor(ctx, RunAnchorCmd{ThreadID, RunID, AgentID ids.ID}) (AnchorRef, error)`: `AppendToThread` as M1-56 does for origin `agent`; insert `kind = 'run'`, `origin = 'agent'`, author the agent, `content_md = ''`, `silent = true` (09 5.4: the anchor notifies nobody), `run_id`, `client_msg_id` = a new server UUIDv7, `client_payload_sha256` with the M1-56 D3 encoding over the anchor's fields; no mentions; append `message.created`, then publish `MessagePosted` with `Kind = run`. The deferred FK lets the anchor and the run be written in either order; `Open` writes the run first.

8. Reads (`internal/runs/app/read.go`, handlers in `adapters/httpapi`).
    - Descriptor of a run: `{Kind: run, ID: run.id, ThreadID: run.thread_id, OwnerID: run.triggered_by}`; `authorize` resolves the thread's path at decision time (07 4.2).
    - `getRun`: load the run (missing: `404`), `authorize(actor, run.read, run)`; `not_found` returns the same error value as a missing run (02 9.2). `200` with `Run`: all steps of the run as an object keyed by step ID (order from `ordinal`), `followups` `{}` until M2-11, `metrics` as stored, `plan` only when it has items, `cost` as `Money`, optional columns absent when NULL; header `ETag: "<version>"` with the M1-18 helper.
    - `listRunSteps`: the same checks as `getRun`; keyset by `ordinal` ascending; cursor bound to the run ID.
    - `listThreadRuns`: `chat.Threads.Info(thread_id)` (missing: `404`); `Scope(actor, run.read)` (proves the route action, 02 9.3); a thread whose channel is not in `Scope.ChannelIDs` gives the same `404` as an unknown thread; query the runs of the thread with keyset `(queued_at DESC, id DESC)`; then `authorize(actor, run.read, run)` per item in filter mode.
    - `listActiveRuns`: `authorize(actor, run.overview.read, instance)`, then `Scope(actor, run.read)`; read active runs (optional `agent_id`, partial index) with keyset `(queued_at DESC, id DESC)` in batches of the page size; resolve their threads with `chat.Threads.Locate`; keep rows whose channel is in `Scope.ChannelIDs`; `authorize(actor, run.read, run)` per row in filter mode; repeat until the page is full or no rows remain. `next_cursor` is the keyset of the last examined row when the page is full and another row exists, absent otherwise (ruling 11). An unknown or invisible `agent_id` gives an empty page, never `404` (no probe).
    - Paging for all lists: `limit` defaults to `api.page_size_default` and is capped at `api.page_size_max` (06 6 point 3); cursors through the codec of `internal/platform/httpserver` (M1-18), bound to list name, filters and principal; a foreign cursor gives `400 cursor_invalid`. No totals.
    - A per-item denial after the channel filter drops the item, logs at error level with only action and resource kind, and increments `bothy.authz.errors` with `bothy.reason = filter_mismatch` (07 7.4 rule 3).
    - `step_count` for a page: one query `run_id = ANY($1) GROUP BY run_id`. `channel_id` of a summary: the thread's channel now.

9. Snapshot. `internal/sync/adapters/runsummary` implements `RunSummarySource` exactly as M1-63 declared it and calls `runs.Runs.ActiveForActor`, which runs the pipeline of `listActiveRuns` without paging (same set of states as `listActiveRuns`; `run.overview.read` is not checked because the snapshot's own action `sync.snapshot.read` was). `internal/wire` binds it instead of `noruns`; remove `noruns`; the sync README names the binding. The gateway `ready` frame gets the same list through M1-63's snapshot.

10. Visibility of thread-scoped tables (Ruling for docs/DECISIONS.md, ruling 9). 07 7.4 rule 1 asks every query over runs for `channel_id = ANY($visible)`, but 03 gives `runs` and `run_steps` no `channel_id` (and M1-62 relies on that when threads move). Rule used here: a read of `runs` or `run_steps` either (a) reads the rows of exactly one thread or run whose visibility was checked first (`getRun`, `listRunSteps`, `listThreadRuns`, the active run of a thread at acceptance), or (b) for the active-run lists reads the active runs, resolves their threads through the `chat` port, keeps only threads whose channel is in `Scope.ChannelIDs` and runs `authorize` per row. The system query of the gauge in note 11 has no principal. Write this into 07 7.4 rule 1 (English, like the chapter). If `tools/lint/sql_visibility.py` is on the base: list the queries of case (a) under category `after_authorize`, the gauge query under `system_job`, and add the category `thread_scope` for case (b) to the closed set, its documentation and its tests (a valid exception, an unknown category still fails); if it is not on the base, the 07 text is the record for M1-70.

11. Metrics. Declare the instruments of signals.md 4.3 with their units, attributes and buckets in `internal/runs/adapters/otelmetrics` over the meter provider of M1-10, or in the registry `internal/platform/otel/metrics` when M1-68 is on the base. The app port `Metrics` has `RunTransitioned(Transitioned)`: `bothy.run.start.latency` when leaving `queued` (`queued_at` to now, `bothy.outcome` `started`, `failed` or `cancelled`, `bothy.billing_mode` = `billing_mode_used` when set, else `billing_mode_expected`); `bothy.run.duration` on an end state of a started run (`bothy.run.state`); `bothy.run.finished` on every end state (`bothy.reason` from the reason table, `other` for any value outside it, not set for `completed`). The unit owner calls it after its unit of work committed, never inside (a retried unit would count twice). `bothy.run.active` is an observable gauge registered in role `worker`, counting active runs per agent handle and state with one grouped query. `bothy.agent.handle` comes from the `agents` port. Never IDs as attributes.

12. Configuration keys used: `api.page_size_default`, `api.page_size_max` (06), `run.step_detail_max_bytes` (03), `authz.not_found_timing_tolerance` (AZ-P4 test). Register `run.step_detail_max_bytes` in `internal/wire/configkeys_runs.go` (chapter 03) if absent, keep `make config-doc-check` green.

13. Effect on existing tests. From this ticket on, a human message in a thread of a channel with an operator (default switches of 09 3.3) or a mention of the operator in a main thread opens a run and an anchor message. M1 tests whose worlds have operators (for example Lotse on Support in the 07 11.1 world) may then see an extra message, another `thread_seq`, `trigger_state` or a new thread. Adapt their expectations to this specified behaviour or let their fixtures use a channel without operator where the test is about something else; list every adapted test in the report.

14. Rulings for `docs/DECISIONS.md` (one row `D-M2-05-<n>` each, with reason):
    1. The acceptance of 09 3.4 point 7 is built here; the personal event `agent.invocation` is written by M2-08 together with its delivery check, because a written but unchecked invocation could otherwise reach an agent connection before I2 is checked.
    2. With the auto thread, `agent.invoke` is checked on the thread of M and the new thread is created only on `allow`: no empty thread after a denial, and the same mention rule as 09 3.4 point 3.
    3. `message.created` of a message is appended before `MessagePosted` is published; the acceptance sets `received` through `SetTriggerState` (version plus one, `message.updated`), and the `201` carries the stored state.
    4. `runs.invocation_jti` of a new run is a fresh UUIDv7 that no token carries until the first delivery replaces it (03 requires the column, no token exists at acceptance).
    5. `billing_mode_expected` of a new run is the agent's billing mode at acceptance; each delivery sets it again (03).
    6. The anchor is `silent`, written in the target thread after the run row.
    7. The reason table is 09 5.2 plus `user_disabled` (`cancelled`, server) from 02 4.3; 09 5.2 gets the row.
    8. An adapter `failed` keeps its reason only when 09 5.2 lists the adapter as setter; otherwise `adapter_error`, and the original code is the detail of a server step `kind = status`, `name = adapter_reason`; 09 5.2 names this place.
    9. Thread-scoped tables without `channel_id` follow note 10; 07 7.4 rule 1 says so.
    10. `listThreadRuns` proves its route action through `Scope(run.read)`; an invisible thread gives the same `404` as an unknown one.
    11. `listActiveRuns` pages by `(queued_at DESC, id DESC)` and fills a page across batches; the cursor is the last examined row.
    12. Until M2-11 the follow-up outcome writes nothing; `multiple_agents_mentioned` writes no audit.
    13. A candidate without mention is first checked in filter mode: a person without `trigger` gets no `trigger_state` and no audit entry (R-AG-05).
    14. A disagreement between M2-04's decision and `authorize` fails closed.
    15. Run metrics are recorded after the commit by the unit owner; `bothy.run.active` is a worker gauge.
    16. The FKs of `runs` to `agent_sessions`, `commands`, `interactions` and `dev_environments` come with M2-12, M2-16, M2-15 and V2-01; `messages.run_id` gets its deferred FK here.
    17. `SetTriggerState` with the stored state and reason is a no-op.

## Tests (write them first)

Test world: 07 11.1 (spaces Support with channels Allgemein and Eskalation, Marketing, Engineering, Research; agents Lotse operating Support, Texter operating Marketing, Assistent operating Research in mode `personal` with the owner as holder; grants G-1 to G-6; the owner), extended for this ticket by: Robin with `write` (no `trigger`) on Support/Eskalation; Texter with `write` on Support/Eskalation; `allow_mentioned_agents = true` on Support/Eskalation. Messages are sent through `createMessage` over the real HTTP chain with people from the identity test helpers.

Unit, `internal/runs/domain`:

- `TestCanTransition_AllPairs`: all pairs of the six states; true exactly for the nine pairs of requirement 11, false for self loops and every pair out of an end state.
- `TestReasonTable`: every row of 09 5.2 plus `user_disabled` with state and setter; reasons exist only for `failed` and `cancelled`.
- `TestNormalizeAdapterReason`: the six adapter codes are kept; an unknown code and the server-only codes `stale`, `interrupted`, `agent_unavailable`, `billing_mode_personal_not_holder` give `adapter_error` with the original; an empty code gives `ErrReasonRequired`.
- `TestOriginalReasonDetail`: cut on a UTF-8 boundary within `run.step_detail_max_bytes`; a code with a bidi character gives `{}`.
- `TestTriggerStateMapping`: every row of note 4, checked against the table of 09 6.

Unit, `internal/runs/app` with fakes:

- `TestAccept_Outcomes` (table): origin `agent`, `app`, `import`, `system`, kind `run` and `system`, DM channel give no write; no candidate gives no write; `multiple_agents_mentioned` writes `failed` without any `authorize` call; follow-up writes nothing; `allow` calls `Open` with the right target; each mapping of note 6 step 5; `level_insufficient` without mention writes nothing and makes no request-mode call; a disagreement fails closed and logs `trigger_decision_mismatch`.
- `TestAccept_AutoThreadOnlyOnAllow`: a denial creates no thread; `allow` creates it before `Open`.
- `TestListActive_FilterMismatchDropped`: the channel filter keeps a row that `authorize` denies; the item is dropped, the log has no IDs, `bothy.authz.errors` with `filter_mismatch` rises by one.
- `TestListActive_PageFilledAcrossBatches`: invisible rows between visible ones; the page is full, `next_cursor` points after the last examined row, the next page starts there and repeats nothing.
- `TestTransition_MetricsAfterCommitOnly`: a vetoed unit records nothing.

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`):

- `TestMigration_RunsAndRunSteps`: columns, defaults and checks as 03; a second active run in a thread fails on the partial unique index; a run in an end state without `finished_at` fails; two `triggering_*` columns fail; `messages.run_id` FK exists and is deferrable; the deferred FKs of note 1 are absent and commented.
- `TestOpen_AtMostOneActivePerThread` (R-DM-14): two units call `Open` for the same thread at the same time; exactly one run exists, the other gets `ErrActiveRunExists` with that run; after the first run reaches an end state a new `Open` succeeds.
- `TestTransition_EveryDisallowedPairRejected`: for each persisted state and each target outside the allowed pairs, `409 invalid_state_transition`, unchanged row and version, no event.
- `TestTransition_RaceOfTwoStartsOneWins`: two goroutines move the same `queued` run to `running` with the same expected version; exactly one succeeds, the other gets `ErrConcurrentChange` with state `running`; exactly one `run.state_changed` with `running` and one `message.updated` with `working`.
- `TestTransition_TriggerStatePerEndState`: `completed` gives `done`; `failed` with `stale` gives `failed` and reason `stale`; `cancelled` with `user_cancelled` gives `cancelled` and reason `user_cancelled`; each raises `messages.version` by one with one `message.updated`; `running -> waiting_input` changes no message.
- `TestTransition_UnknownAdapterReasonBecomesAdapterError`: adapter `failed` with `quota_exhausted` stores `adapter_error` and one status step `adapter_reason` whose detail holds `quota_exhausted`; adapter `runtime_error` is kept without a step; adapter `stale` becomes `adapter_error`.
- `TestTransition_EndStateTimes`: `finished_at` set on every end state, `duration_ms` only for started runs, `started_at` kept on `waiting_input -> running`.
- `TestAcceptance_ThreadMessageOpensRun`: Alex writes in a thread of Support/Allgemein without mention; one run `queued` for Lotse with `triggered_by` Alex and `triggering_message_id` M; anchor in the same thread with `kind = run`, author Lotse, `origin = agent`, empty content, `silent`, `run_id`; M has `trigger_state = received` and version 2; the `201` body shows both; events in the order of note 6 step 8 with consecutive `thread_seq` in the thread.
- `TestAcceptance_MainThreadAutoThread` (server part of AG-05): Alex mentions Lotse in the main thread of Support/Allgemein; a new thread with `parent_message_id` M, `title_source = auto`, operator Lotse; run and anchor in the new thread; `trigger_state` on M in the main thread.
- `TestAcceptance_Rejections`: two agents mentioned gives `multiple_agents_mentioned` and no audit entry; Texter mentioned in Support/Allgemein (no guests there) gives `agent_not_allowed_here`; Robin mentions Lotse in Support/Eskalation without `trigger` gives `no_trigger_right` with a deny audit entry; Robin without mention in a thread of Support/Eskalation gives no `trigger_state` and no audit entry; with an active Lotse run in a thread of Support/Eskalation, Alex mentioning Texter there gives `run_active_other_agent`; a disabled Lotse gives `agent_disabled`. No run, no new thread, no anchor in any case.
- `TestAcceptance_AgentMessageNeverTriggers`: Lotse writes a message mentioning Texter in a free channel; no run, no `trigger_state` (mirrors AZ-M30, owned by M2-08).
- `TestAcceptance_ConcurrentMessagesOneRun`: Alex and the owner write in the same thread of Support/Allgemein at the same time; one run exists; the other message has no `trigger_state` (follow-up, M2-11).
- `TestGetRun_Document`: all fields of `Run`, steps keyed by ID, `followups` `{}`, `ETag` equals the version, no JSON `null`.
- `TestListRunSteps_Order` and `TestListThreadRuns_NewestFirst`: keyset order, `limit` capped at `api.page_size_max`, foreign cursor `400 cursor_invalid`.
- `TestListActiveRuns_Visibility`: Alex sees the active runs of Support only; Sam sees those of Marketing and of Support/Allgemein (read through group Leser, G-4) but not those of Support/Eskalation; the owner sees all spaces and no DM; end states are absent; filter `agent_id` works; an unknown `agent_id` gives an empty page.
- `TestRunReads_NotFoundEquality` (AZ-P4 for `getRun`, `listRunSteps`, `listThreadRuns`): a foreign run or thread and an unknown ID give the same status, body and headers, and the medians of the response times differ by at most `authz.not_found_timing_tolerance`.
- `TestAZP5_RunsOverviewAndSnapshot`: property test over random worlds with active runs (seed logged, fixed number of worlds in `make test`, reuse the world generator of M1-63 if exported): no item of `listActiveRuns` and no entry of `Snapshot.active_runs` refers to a channel without read right.
- `TestSnapshot_ActiveRuns`: `getSnapshot` of Alex lists the active Support runs as `RunSummary`; an ended run disappears; `internal/sync/adapters/noruns` no longer exists.
- `TestRunStateChanged_CatchUpReceivers`: for a run in a thread of Support/Eskalation, `listEvents` gives `run.state_changed` to Alex and the owner and not to Sam; the envelope has `resource.version` equal to `runs.version`.
- `TestRunActiveGauge`: the worker gauge reports per agent handle and state.

Contract:

- `TestRunSchemas`: responses of the four operations validate against `RunSummaryPage`, `Run`, `RunStepPage` and `Problem`; `run.state_changed` validates against `RunStateChangedEvent`, `message.updated` against `MessageUpdatedEvent`; no JSON `null` in any of them.
- `TestTxnRegistrationsGolden` (M1-13) shows `subscribe messaging.MessagePosted -> runs ...` after the existing subscribers.
- AZ-P1: the route test of M1-17 and M1-28 passes with the four new routes.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/runs/... ./internal/messaging/... ./internal/chat/... ./internal/sync/... ./internal/authz/... ./internal/kernel/... ./internal/wire/... ./migrations/...`
- `make migrations-lint`, `make codegen-check`, `make arch`, `make register-check`, `make api-lint`, `make api-checks`, `make config-doc-check`, `make docs`
- `make check`

## Acceptance criteria

- [ ] `listThreadRuns`, `getRun`, `listRunSteps` and `listActiveRuns` are enabled and registered; AZ-P1 passes; an invisible run or thread gives the same `404` as an unknown one (AZ-P4).
- [ ] Migrations `runs` and `run_steps` match 03 except the documented deferred FKs; `messages.run_id` has its deferred FK.
- [ ] Every transition outside 03 `runs` is rejected with `409 invalid_state_transition` and changes nothing.
- [ ] A race of two starts on one run gives exactly one success, one `run.state_changed` and one `trigger_state` change.
- [ ] `trigger_state` per end state: `done`, `failed` with reason, `cancelled` with reason; `working` at the start; each change raises the message version and writes `message.updated`.
- [ ] An unknown adapter reason is stored as `adapter_error` with the original code in the detail of a status step.
- [ ] At most one active run per thread (R-DM-14), also under concurrent `Open`.
- [ ] A triggering human message opens a run, an anchor and `trigger_state = received` in its transaction, in the event order of note 6; rejections write `failed` with the 09 3.5 reason and no run; denials of `agent.invoke` are audited.
- [ ] The snapshot names the running runs the actor may read through `RunSummarySource`; the `noruns` binding is gone; AZ-P5 holds for "Running now" and the snapshot.
- [ ] Run metrics follow signals.md 4.3.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `runs`, `messaging`, `chat` and `sync` updated (ports, tables, events, subscriptions, the visibility rule of note 10, the `RunSummarySource` binding).
- Rulings of note 14 recorded in `docs/DECISIONS.md` (one row each with reason); 07 7.4 rule 1 and 09 3.4 and 5.2 updated as notes 10, 6 and 2 say.
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- ADR 0025 "Confirmation" names the tests of this ticket, then `make adr-index`. Another ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
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
