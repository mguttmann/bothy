# M2-10: Run document, patches and live box

Status: ready | Depends on: M2-09 | Milestone: M2

## Goal

After this ticket the agent of an active run changes its live box only through RFC 6902 patches
(`add`, `replace`, `remove`) on the allowed paths of `run_document.v1`, over REST (`patchRun`)
and over the gateway op `run.update`, with `base_version` as compare-and-swap. A stored patch
raises `runs.version`, writes no durable event and takes neither `event_seq` nor `NOTIFY`; it
reaches clients only as the ephemeral event `run.updated`, coalesced per run and connection for
`sync.run_patch_coalesce_interval`, and only on connections that focus the run's thread and may
read the run. Focusing a thread with an active run sends the whole document once, and a
connection whose outbound queue is full gets the whole document instead of the dropped patches.
This is the live box of 01 "Live box" and ADR 0025: the event log carries only state changes, so
the number of durable events per run does not depend on how often the agent patches.

## Read first

- `docs/PLAN.md`: section "Way of working per ticket"; section "8. M2 Agent platform, extensions and terminal" (Goal, Scope, Out of scope, Exit criteria points 1 and 6,
  Test strategy, Red team, Risks, block "agents and extensions"); rows M2-05, M2-07, M2-08,
  M2-09, M2-10, M2-11, M2-14, M2-22, M2-29, M2-31, M2-36 and M7-17; M1 rows M1-64 to M1-66 and
  M1-68.
- `docs/spec/09-agents.md`: "Run document (`run_document.v1`)" (5.3, the path table),
  "Live box content" (5.4), "Stall, interruption, orphaned runs" (5.5), "Adapter
  operations" (4.7, rows "Change live box", "Report step", "Acknowledge follow-up"), "Stop"
  (point 4), "Reliability" (row "Server restart"), "Test doubles" and "Required scenarios"
  (AG-12, AG-14), "Live box from the stream" (18.4), "Configuration values of this chapter".
- `docs/spec/04-sync-protocol.md`: "Two kinds", "Envelope", "Frames on the WebSocket", "Ops" (rows `focus` and `run.update`), "Lifecycle" step 7, "Flow" (points 3
  and 6), "Delivery guarantee" (points 3 and 4), "Ordering", "Run document" (11.1), "From the adapter to the server" (11.2), "From the server to the clients" (11.3), "Follow-ups" (15.4 point 3), "Cancellation"
  (15.5), "Decisions of this chapter" (R-SY-05, R-SY-06), "Configuration values of this chapter".
- `docs/spec/03-data-model.md`: tables `runs` (`version`, `status_line`, `last_text_excerpt`,
  `model`, `plan`, `metrics`, `cost_micros`, `currency`, `cost_is_estimate`, `stall_warned_at`,
  the partial index of active runs), `run_steps`, `run_followups`; "Lock order";
  "Notifying the gateway processes".
- `docs/spec/05-actions.md`: rows `run.update` and `run.read` (4.10) and `gateway.focus` (4.9);
  "Rate-limit classes" (`rl.agent_runtime`, key agent); "Columns Owner, Step-up, Audit, Permissions"
  (every denial is audited); "Decision"; "Binding rules" rule 2; "Durable event types and receive action" (table of ephemeral events, row `run.updated`);
  "WebSocket ops and action" (row `run.update`). `docs/spec/actions.yaml` (M1-02): entries
  `run.update` (`audit: never`) and `run.updated` (`resource_version: true`).
- `docs/spec/02-architecture.md`: "Ports, events and sources per module" (rows `runs` and
  `gateway`), "Allowed dependencies" (`gateway` may use `runs`; `runs` may use `chat` and
  `agents`), "Cross-cutting ports" (`kernel/eventlog` `EphemeralPublisher`, `kernel/authorize`),
  "Event types and WebSocket ops per module" (row `runs`), "Layers within a module" (rule 5,
  metrics), "Transactions and Unit of Work", "Lock order", "Gateway", "Hub and delivery",
  "Ephemeral events" (row `run.updated`), "Cost of serialization".
- `docs/spec/06-api.md`: "Errors" (points 3, 4 and 7), "Idempotency" (point 3, reason
  `agent_runtime`), "Concurrency" (point 4), endpoint table "Agent runtime".
- `docs/spec/07-permissions.md`: "SR9 Acting-for binding (confused deputy)" (patches on the own run are
  an exception), "Delivery filter", "Immediate effect on revocation" (point 3).
- `docs/spec/19-security.md`: SEC-IN-1 to SEC-IN-4, SEC-IN-7, SEC-LOG-4.
- Context only: `docs/spec/01-product.md` "Live box" (6.2), `docs/spec/15-client-architecture.md`
  "Live box" (7.4, the client rule), `docs/spec/10-backend-policy.md` "Display" (12.2, amounts
  only for `organization`).
- `docs/api/openapi.yaml`: operations `patchRun` (`x-authz-action: run.update`,
  `x-rate-limit-class: rl.agent_runtime`, `x-idempotency-exempt: agent_runtime`) and `getRun`;
  schemas `RunPatch`, `RunPatchResult`, `JsonPatchOperation`, `JsonValue`, `Run`, `RunStep`,
  `RunFollowup`, `RunMetrics`, `RunApiRetry`, `RunPlan`, `Money`, `RunUpdatedPayload`,
  `RunUpdatedEvent`, `GatewayRunUpdate`, `GatewayFocus`, `GatewayError`, `EventEnvelope`,
  `Problem` (`errors`, `current`), `FieldError`; responses `BadRequest`, `Forbidden`, `NotFound`,
  `Conflict`.
- `docs/api/asyncapi.yaml`: operations `receive.run.update` and `receive.focus`; messages
  `op.run.update`, `op.focus` and `event.run.updated` (`x-event-kind: ephemeral`,
  `x-receive-action: run.read`).
- ADRs 0025 (run as its own object for the live box), 0015 (`422` only for
  `idempotency_key_reuse`), 0017, 0052, 0053, 0008, 0026 (the SDK throttles and rebases).
- `docs/DECISIONS.md`: D-W-20 (closed list of problem codes), D-W-100; section 10 for the rows
  of this ticket.
- `docs/security/threat-model.md`: "Gateway (WebSocket)" (row "Focus on foreign threads"),
  "Agent adapters and runtime"; register rows T-AZ-01, T-AZ-03, T-IN-01, T-DS-01.
- `docs/RISKS.md`: RK-31 (patches without event log), RK-32 (many viewers of one live box).
- `docs/observability/signals.md`: 4.1 `bothy.gateway.outbound.dropped` (reason
  `patch_replaced_by_snapshot`), 4.2 `bothy.delivery.duration`, 4.3 `bothy.run.steps`.
- `docs/operations/configuration.md`: the keys of Design note 14.
- M1 briefs whose interfaces you use: M1-13 (unit of work, `database.tx_retry_max`), M1-17
  (`api/operations.yaml`, route generator), M1-18 (problem writer, `StageValidate`, the
  `FieldError` code vocabulary, no-null encoder), M1-19 (`rl.agent_runtime`), M1-26
  (`kernel/textcheck.DisplayText`), M1-28 (`AgentSelf`, visibility of kind `run`, filter mode,
  denial audit), M1-44 and M1-51 (agent tokens and records), M1-55 (`chat` `Threads` info),
  M1-64 (op dispatcher `Register(op, action, handler)`, `gateway_ops`, test client `wstest`),
  M1-65 (hub, bounded queue, `sync.Admit`, `Delivery(ctx)`), M1-66 (`focus`, focus pruning,
  `EphemeralPublisher`, the focus-bound type `run.updated`, the drop rule for ephemeral frames).
  Read M1-68 as well: its metric registry is in this ticket's dependency closure through M2-02
  (Design note 13).
- M2 briefs this ticket builds on: M2-09 (direct dependency: `startRun`, `finishRun`,
  `cancelRun`, job `runs.cancel_timeout`) and through it M2-05 (`runs`, `run_steps`, `getRun`,
  `listRunSteps`, the projection of the run document), M2-07 (agent connections on the gateway)
  and M2-08 (delivery that lets a run start). M1-66 is the second direct dependency; M1-64 and
  M1-65 come with it. Read the row of M2-11 to see what stays for it.
- Code: `internal/runs` (M2-05, M2-08, M2-09), `internal/gateway` (M1-64 to M1-66, M2-07),
  `internal/kernel/eventlog`, `internal/kernel/authorize`, `internal/kernel/txn`,
  `internal/kernel/textcheck`, `internal/kernel/errs`, `internal/wire`,
  `internal/platform/pg/pgtest`, `internal/gateway/adapters/wstest`, `api/operations.yaml`.

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

11. Patches use RFC 6902 `add`, `replace` and `remove` with JSON Pointers into `run_document.v1`,
    only on the paths and ops of 09 "Run document (`run_document.v1`)"; anything else gives
    `400 validation_failed` with `errors[].code` `patch_path_forbidden` and `pointer` on the op
    (09 5.3, 04 11.2, SEC-IN-7, AG-12).
12. `base_version` is a compare-and-swap on `runs.version`: a mismatch gives
    `409 base_version_conflict` with the current document in `current`; a stored patch raises
    `runs.version` and answers the new version as `RunPatchResult` (04 11.2; 06 "Errors" point 3,
    "Concurrency" point 4; AG-12).
13. Only the agent of the run patches, and only while the run is active: action `run.update`
    (`agent-self`, Audit `no`, rate-limit class `rl.agent_runtime`), no `Idempotency-Key`
    (`x-idempotency-exempt: agent_runtime`); patches after the end of a run are rejected (05 4.10;
    04 11.2 and 15.5; 09 "Stop" point 4; 06 "Idempotency" point 3; AG-14).
14. The gateway op `run.update` is equivalent to `patchRun`; a failing op answers `error` with the
    problem code and keeps the connection (04 "Ops"; asyncapi `receive.run.update`; 05 7).
15. A patch writes no durable event and takes neither `event_seq` nor `NOTIFY`; the number of
    durable events per run does not depend on the patch rate (04 2.1, R-SY-05; 02 "Ephemeral events", "Cost of serialization"; ADR 0025 Confirmation; RK-31).
16. Content changes reach clients only as the ephemeral `run.updated {run_id, base_version,
    version, ops}`, at most once, and only on connections that focus the thread and pass
    `run.read` at delivery (04 11.3 point 2; 05 6; 07 "Delivery filter"; T-AZ-01).
17. The server coalesces per run and connection for `sync.run_patch_coalesce_interval` into one
    frame from the oldest `base_version` to the newest `version`; a later `replace` on the same
    path replaces the earlier one (04 11.3 point 3; RK-32).
18. When the outbound queue of a connection is full, the gathered patches are dropped and the
    whole document goes out as `run.updated` with `snapshot`; an ephemeral frame never closes a
    connection (04 11.3 point 3 and 7.2 point 6).
19. Focusing a thread with an active run sends the whole document once (04 11.3 point 5; 15
    "Live box").
20. `last_text_excerpt` is cut to `run.excerpt_max_chars`, `run_steps.detail` to
    `run.step_detail_max_bytes`; above `run.max_steps_persisted` steps the server merges older
    finished steps into one status step; `ordinal` is assigned by the server, step ids come from
    the adapter as UUIDv7 (09 5.3 and 4.7; 03 `run_steps`).
21. Bodies and frames are decoded strictly and size limited, display texts are checked for
    control and bidi characters, and no patch content appears in logs, spans, metrics or error
    details (SEC-IN-1 to SEC-IN-4, SEC-LOG-4; 06 "Errors" point 3).
22. `run.updated` envelopes follow 04 "Envelope": no `seq`, no `permission_version`,
    `resource.version` is the run's version, no JSON `null` (ADR 0052, ADR 0053; actions.yaml
    `resource_version: true`).

## Scope

All paths under `services/server/` unless noted. Where M2-05, M2-08 or M2-09 already created a
file or type named here under another name, extend theirs (see Boundaries).

Create:

- `internal/runs/domain/patch.go`: JSON Pointer parsing (RFC 6901, `~0` and `~1`), the path
  table of Design note 2 as data, `ParseOps(ops []runs.PatchOp) ([]Op, []FieldError)` for the
  static checks of notes 2 and 3.
- `internal/runs/domain/patch_apply.go`: `Apply(state DocumentState, ops []Op, lim Limits)
  (Change, []Op, error)`: set semantics of note 4, the column changes, the step inserts and
  updates, the normalized ops.
- `internal/runs/domain/truncate.go`: cutting of `last_text_excerpt` and `detail` (note 5).
- `internal/runs/domain/steps_summary.go`: merging of older finished steps (note 6).
- `internal/runs/app/patch.go`: the use case `Patch` (note 1); `internal/runs/app/livebox.go`:
  the delivery reads of note 10; `internal/runs/app/followups.go`: the outgoing port
  `FollowupStates` and its empty default (note 7).
- `internal/runs/adapters/postgres/queries/run_patch.sql`: lock of the run row, update of the run
  columns with `version = version + 1`, highest ordinal, insert and member updates of
  `run_steps`, the summarization queries, active runs of a set of threads.
- `internal/runs/adapters/httpapi/patch.go`: the strict-server method for `patchRun`.
- `internal/gateway/domain/runpatch.go`: the merge rule of note 9 on a gateway-owned op type
  (`Op{Op, Path string; Value []byte}`), the per connection and run bookkeeping of the last sent
  version, the snapshot mark.
- `internal/gateway/app/runpatches.go`: the coalescer of note 9 (own goroutine, test clock).
- `internal/gateway/app/runupdate.go`: the op handler `run.update` (note 11).
- Integration tests under `test/integration/livebox/` (component `testsupport`, 02 "Code layout"),
  or next to the integration tests of M2-05 and M2-09 if they chose another place.

Change:

- `internal/runs` (port): DTOs and interfaces, with doc comments:

```go
// PatchOp is one RFC 6902 operation as sent by the adapter (09 5.3).
type PatchOp struct {
    Op    string          // "add", "replace" or "remove"
    Path  string          // JSON Pointer (RFC 6901) into run_document.v1
    Value json.RawMessage // empty for "remove"
}
// PatchRun is the input of Runs.Patch; REST and the gateway op build the same value.
type PatchRun struct {
    RunID       ids.ID
    BaseVersion int64
    Ops         []PatchOp
}
// PatchResult carries the new runs.version.
type PatchResult struct{ Version int64 }
// RunUpdated is the payload of the ephemeral event run.updated as published after commit.
// Ops are the normalized ops with stored values (Design note 4); Truncated means the ops were
// left out because they exceed sync.event_payload_max_bytes.
type RunUpdated struct {
    RunID, ThreadID      ids.ID
    BaseVersion, Version int64
    Ops                  []PatchOp
    Truncated            bool
}
// Runs gains (the other methods come from M2-05 and M2-09):
//   Patch(ctx context.Context, in PatchRun) (PatchResult, error)
// LiveBox serves the gateway's delivery reads; callers admit every recipient first (note 10).
type LiveBox interface {
    Documents(ctx context.Context, threadIDs []ids.ID) ([]Document, error) // active runs only
    Document(ctx context.Context, runID ids.ID) (Document, bool, error)    // false: not active
}
```

  `Document` stands for the run document DTO of M2-05 (the projection `getRun` answers); use
  its name.

- `internal/gateway/app/focus.go` and `internal/gateway/app/ephemeral.go` (M1-66): the snapshot on
  newly focused threads and after `resume` (note 10); `run.updated` goes into the coalescer
  instead of the immediate enqueue path.
- `internal/wire`: embed the `patchRun` handler; register the op `run.update` with action
  `run.update`; start and stop the coalescer with the process lifecycle; hand `runs.LiveBox` to
  the gateway; bind the empty `FollowupStates`; register missing configuration keys (note 14);
  update the golden files that list registrations.
- `api/operations.yaml`: enable `patchRun`; move `run.update` from `gateway_ops.later` to
  `gateway_ops.enabled`; rerun the route generator (P-04).
- `docs/api/openapi.yaml`: the request example of `patchRun` (today `add` on
  `/steps/<id>/status` without `value`, which 09 5.3 does not allow) becomes a `replace` with a
  `value`; the description of `patchRun` names `patch_path_forbidden`, `409 conflict` with
  `reason` `run_not_active` and the set semantics of note 4.
- `docs/spec/09-agents.md` 5.3 (not core contract): the API-visible rulings of notes 1 to 6 and
  11 (set semantics, `errors[].code` values, `run_not_active`, which end of the excerpt stays,
  the summary step).
- `docs/adr/0025-run-as-its-own-object-for-the-live-box.md`: if its Confirmation still says
  that a forbidden path gives `422`, correct that line to `400 validation_failed`
  (`patch_path_forbidden`) as ADR 0015, 06 "Errors" point 4 and 09 AG-12 say; record the
  correction in `docs/DECISIONS.md`.
- Metrics per note 13 (only with the registry of M1-68): `internal/runs/app/metrics.go`,
  `internal/runs/adapters/otelmetrics/metrics.go`, the gateway's metrics port.
- READMEs of `runs` and `gateway` (English, like the existing files); `services/server/CHANGELOG.md`
  under "Unreleased"; `docs/DECISIONS.md` section 10, rows `D-M2-10-<n>` (note 16).

No migration: `runs`, `run_steps` and their indexes come from M2-05, `run_followups` from M2-11.
No new top-level make target.

## Out of scope

- Tables `runs` and `run_steps`, `getRun`, `listRunSteps`, `listThreadRuns`, `listActiveRuns`, the
  projection of the run document and running runs in the snapshot (M2-05).
- Invocation tokens and delivery (M2-06, M2-08); agent consumers, `ack` and redelivery (M2-07).
- `run.start`, `run.token.renew`, `awaitRunInput`, `run.finish`, `run.cancel`, the jobs
  `runs.cancel_timeout` and `runs.stall_check`, and the stall detection itself (M2-09). This
  ticket only clears `stall_warned_at` on a stored patch (note 1f).
- Table `run_followups`, follow-up rows and states, the forward-only rule and
  `trigger_state = followup_delivered` (M2-11); this ticket provides the path, the value check
  and the port `FollowupStates`.
- Acting for the trigger (SR9, `acting_for`, M2-14); patches on the own run stay outside it.
- Cost booking, the price table and the rule "no amounts for `personal`" (M2-22, BR-11).
- Adapter side: throttling to `agent.run_update_min_interval`, rebase after `409`, step ids
  (M2-29, M2-30); the reference adapters (M2-33 to M2-35).
- The data-driven conformance scenario AG-12 with the fake agent (M2-31) and the `perf`
  measurement of `perf.run_patch_fanout_p99` (M2-36).
- Distribution across api nodes with `NOTIFY ephemeral` (M7-17); design the coalescer so that a
  "run changed to version V" signal from another node can mark connections for a snapshot.
- Gateway frame conformance G9 (M1-67); live-box views in the apps (M4, M5).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M2-10-run-document-live-box`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(runs): ...`, `feat(gateway): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same holds for the merged code of M2-05, M2-07, M2-08 and M2-09: follow their names and list each difference in the report.
- No agent runtime, model or adapter runs in any test: the adapter is the Go test client of M1-64 (`wstest`) or plain HTTP with an agent token. No real third-party service is called.
- A test-only dependency (note 4, the RFC 6902 reference applier) is pinned to a full release version in `go.mod` and must pass the license gate; nothing else is added to `go.mod`.

## Design notes

1. Use case `Runs.Patch` (REST `patchRun` and the op `run.update` call the same function), one
   unit of work, in this order:
   a. Load the run without a lock; an unknown id gives `404 not_found`. Build the descriptor of
      kind `run` on the run's thread with `OwnerID` = `runs.agent_id` and call
      `authorize(actor, run.update, descriptor)`. `authz` decides: no read permission on the thread
      gives `not_found` with exactly the body and headers of an unknown id; a visible run of
      another agent, or a person or an app as actor, gives `403 forbidden` (`not_self` or
      `subject_kind_not_allowed`, M1-28 `AgentSelf`). Denials are audited by `authz` like every
      denied request or op (05 "Columns Owner, Step-up, Audit, Permissions", R-AZ-04); an allowed
      patch writes no audit entry (Audit `no`, actions.yaml `audit: never`).
   b. Static checks without database access: every op through the path table (note 2) and the
      value rules (note 3). Failure: `400 validation_failed`; `errors[]` names every failing op
      found here, in op order.
   c. `SELECT ... FROM runs WHERE id = $1 FOR UPDATE`. A state other than `queued`, `running` or
      `waiting_input` gives `409 conflict` with `reason` `run_not_active`. Ruling for
      `docs/DECISIONS.md`: 04 15.5 and 09 "Stop" point 4 reject later patches without naming a
      code; the state comes before the version so that an adapter after `cancel_timeout` learns
      that the run ended, not only that it is behind.
   d. `base_version` different from `runs.version` gives `409 base_version_conflict` with
      `current` = the run document exactly as `getRun` answers it at this version (04 11.2; 06
      "Errors" point 3).
   e. Dynamic checks against the locked state (notes 4, 6, 7); the first failure gives
      `400 validation_failed` naming that op and rolls everything back.
   f. Write in the lock order of 03 "Lock order" (the `runs` row, then `run_steps` rows in
      ascending `id`): one `UPDATE runs SET <changed columns>, version = version + 1,
      stall_warned_at = NULL WHERE id = $1`, the inserts and member updates of `run_steps`, then
      the summarization (note 6). One stored patch is exactly one version step, whatever the
      number of ops (03 `runs.version`). Clearing `stall_warned_at` is a Ruling for
      `docs/DECISIONS.md`: a patch is movement, so the hint "no movement since ..." of 09 5.4
      must go.
   g. No `event_log` row, no `event_seq`, no `audit_seq`, no `NOTIFY`, no in-transaction event
      (02 "Cost of serialization", R-SY-05). The unit of work retries `40P01` and `55P03` up
      to `database.tx_retry_max`.
   h. Answer `200` with `RunPatchResult {version}`. Only after the unit of work returned without
      error, publish `run.updated` (note 8); a rolled back or retried attempt never publishes.
2. Path table (09 5.3). Everything not listed gives `patch_path_forbidden` with `pointer`
   `/ops/<i>`: `/state`, `/version`, `/billing_mode_expected`, `/billing_mode_used`, the times,
   `/summary`, the ids, the document root, paths inside `/cost`, `/plan` and
   `/metrics/last_api_retry`, unknown metric fields and any other step member.

| Path | Ops | Value | Stored in |
|---|---|---|---|
| `/status_line` | `add`, `replace`, `remove` | string, display text | `runs.status_line` |
| `/last_text_excerpt` | `add`, `replace` | string, cut (note 5) | `runs.last_text_excerpt` |
| `/model` | `replace` | string, display text | `runs.model` |
| `/metrics/<field>` for `context_pct`, `tokens_in`, `tokens_out`, `tokens_cache_read`, `tokens_cache_write`, `tokens_cache_write_1h`, `subagents`, `background_jobs`, `api_retries` | `add`, `replace` | number for `context_pct`, otherwise integer | `runs.metrics` |
| `/metrics/last_api_retry` | `add`, `replace`, `remove` | `RunApiRetry` | `runs.metrics` |
| `/cost` | `replace` | `Money` | `runs.cost_micros`, `runs.currency`, `runs.cost_is_estimate` |
| `/plan` | `add`, `replace`, `remove` | `RunPlan` | `runs.plan` |
| `/steps/<step_id>` | `add` | step without `ordinal` (note 3) | new `run_steps` row |
| `/steps/<step_id>/status`, `/title`, `/detail`, `/finished_at` | `replace` | enum of `RunStep.status`, display text, object cut (note 5), RFC 3339 time | `run_steps` column |
| `/followups/<message_id>/state` | `replace` | `delivered` or `absorbed` | `run_followups.state` (note 7) |

A `<step_id>` or `<message_id>` that is not a UUIDv7 (`kernel/ids`) gives `invalid_format` with
`pointer` `/ops/<i>/path`. `test`, `move` and `copy` already fail the schema (`invalid_enum`,
M1-18).

3. Values (SEC-IN-1, ADR 0053), `pointer` into the op's value (`/ops/<i>/value/...`), codes from
   the M1-18 vocabulary:
    - `value` is required for `add` and `replace` (`required`) and ignored for `remove` (RFC 6902
      section 4). A JSON `null` anywhere gives `invalid_type`.
    - Objects are closed: an unknown member gives `unknown_member`.
    - Integers lie between 0 and 2^53 - 1 (ADR 0053; `too_small`, `too_large`); `context_pct` lies
      between 0 and 100.
    - `Money`: `amount_micros` not negative; `currency` three upper-case letters
      (`pattern_mismatch`); `is_estimate` must be `true` (`invalid`). Ruling for
      `docs/DECISIONS.md`: every amount is an estimate until a reconciliation replaces it (10
      12.1); an adapter is no reconciliation.
    - `RunPlan.items[]`: `id`, `text`, `status` in `pending`, `in_progress`, `completed`; ids are
      unique within the plan (`invalid`). Ruling: clients key plan items by id.
    - Step value of `add`: `kind` and `name` required; `title` (default empty), `status` (default
      `running`, 03), `detail` (default `{}`), `parent_step_id`, `started_at` (default: time of
      the transaction) and `finished_at` optional; an `id` member must equal the path id
      (`invalid`); an `ordinal` member gives `patch_path_forbidden` at `/ops/<i>/value/ordinal`,
      because the server assigns it (09 4.7).
    - Display texts (`status_line`, `model`, step `name` and `title`, plan item `text`,
      `last_api_retry.error`) pass `kernel/textcheck.DisplayText` (SEC-IN-4); `errors[].code` is
      the rule it names (`invalid_utf8`, `control_character`, `bidi_control`). `last_text_excerpt`
      and `detail` are agent output like message content and stay as sent apart from cutting.
      Ruling for `docs/DECISIONS.md`.
    - No length key exists for these texts; `http.max_json_body_bytes` (REST) and
      `sync.ws_max_frame_bytes` (gateway) bound them (SEC-IN-3).
4. Set semantics and normalized ops. Ruling for `docs/DECISIONS.md` (spec gap: 09 5.3 allows only
   `replace` on `/model` and `/steps/<id>/finished_at`, but both start absent, `RunStart.model` is
   optional, and RFC 6902 section 4.3 requires the target of `replace` to exist):
    - A member exists when its value is stored: `status_line`, `last_text_excerpt`, `model` and a
      step's `finished_at` when not NULL; a metric field when present in `runs.metrics`; `plan`
      when `runs.plan` has `items`; `cost` always.
    - `add` and `replace` set the member, also when it is absent; `remove` of an absent member is
      a no-op.
    - `add` on `/steps/<id>` with an id that already exists gives `patch_target_exists` (the
      ordinal and start of a step never change; a repeated add after a lost answer meets
      `base_version_conflict` first). A member `replace` on an unknown step, or a follow-up the run
      does not have, gives `patch_target_missing`; so does a `parent_step_id` that names no step
      of this run and no step added earlier in the same patch.
    - The use case returns normalized ops: per effective change `add` when the member was absent
      at `base_version`, `replace` when present, `remove` when cleared; values are the stored
      values (after cutting; a step `add` carries the full `RunStep` with the server's `ordinal`
      and `started_at`); no-ops are dropped. Applying the normalized ops with RFC 6902 to the
      document at `base_version` gives exactly the document at the new version. Only these ops go
      to clients. Prove it in tests with an independent RFC 6902 applier: a maintained Go library for RFC
      6902 as a test-only dependency, pinned to a full release and under a license of
      `quality/licenses/allowed.txt`, or, if none passes the license gate, an applier in a test
      package written from RFC 6902 sections 4.1 to 4.3; record the choice.
5. Cutting. Ruling for `docs/DECISIONS.md` where 03 only says "shortened" and 09 only "truncated":
   `last_text_excerpt` keeps the last `run.excerpt_max_chars` runes, because drafts grow at the
   end (09 17.3 `send_draft`, 18.4). A `detail` whose canonical JSON exceeds
   `run.step_detail_max_bytes` gets its string leaves shortened, longest first, at a rune
   boundary, and a top-level member `truncated: true`; if it still does not fit, the stored value
   is `{"truncated": true}`. Equal input gives equal output.
6. Steps. `ordinal` is the run's highest ordinal plus one, taken under the run row lock (unique
   `(run_id, ordinal)`). Summarization (03 `run_steps`), Ruling for `docs/DECISIONS.md` on its
   shape: after the inserts of a patch, while the run has more than `run.max_steps_persisted`
   steps, take the finished step (`completed`, `failed`, `cancelled`) with the lowest ordinal that
   has no child step and is not the summary step, delete it and count it into the summary step:
   kind `status`, name `steps_summarized`, empty title, status `completed`, `detail`
   `{"summarized": {"count": <n>, "by_kind": {"<kind>": <n>}}}`, a server UUIDv7 and the lowest
   ordinal removed so far. Running steps and parents are never removed, because
   `parent_step_id` cascades on delete (03). Without a candidate the patch fails with
   `400 validation_failed`, `errors[].code` `limit_reached` at `/ops/<i>` of the first step
   `add`. The normalized ops contain `remove /steps/<removed_id>` per removed step and the `add`
   of the summary step or the `replace` of its `detail`.
7. Follow-up states. The path rule and the value check live here; the write goes through the
   outgoing port `FollowupStates` in `internal/runs/app` (read the states of given message ids of
   a run; set one state). Until M2-11 creates `run_followups`, `internal/wire` binds an
   implementation that knows no follow-ups, so every follow-up op gives `patch_target_missing`.
   M2-11 replaces it and adds the forward-only rule and `trigger_state = followup_delivered`
   (04 15.4 point 3; 09 4.7 row "Acknowledge follow-up").
8. Publication (`runs`): after the commit, `kernel/eventlog.EphemeralPublisher.PublishEphemeral`
   with type `run.updated`, `schema_version` from the event register, `occurred_at` = time of the
   transaction, actor `{id: <agent>, kind: agent}`, scope `{space_id, channel_id, thread_id}` of
   the run's thread (read through the `chat` port in the unit of work), resource `{kind: run, id,
   version: <new version>}`, payload `runs.RunUpdated`. No `seq`, no `permission_version`, no
   audience. If the encoded `RunUpdatedPayload` with ops exceeds `sync.event_payload_max_bytes`,
   publish it without ops and with `Truncated` set (04 2.2 rule 2). A publish error is logged
   with the run id and never reaches the adapter: the patch is stored, and the next patch or a
   `getRun` heals the box (ADR 0025, Consequences).
9. Coalescing (`gateway`; 04 11.3 point 3; 02 "Ephemeral events": the hub collects):
    - The hub hands `run.updated` from the publisher to the coalescer instead of the immediate
      enqueue path of M1-66; nothing changes for `typing.started`.
    - A change opens a window of `sync.run_patch_coalesce_interval` for its run. A change whose
      `BaseVersion` equals the chain's current version extends the chain: a later `replace` on a
      path removes an earlier `replace` or `add` on exactly that path (a removed `add` turns the
      later op into `add` at the later position); ops on other paths keep their order. A change
      whose base is above the chain's version (a version step without patch, for example a state
      transition of M2-09) discards the ops gathered so far and restarts the chain at its base.
      Ruling for `docs/DECISIONS.md`.
    - At the end of the window, for each connection in phase `live` that focuses the run's thread
      (`idle` included, 04 "Ops" row `focus`) and passes `run.read` through `sync.Admit` in
      `authorize.Delivery(ctx)` at that moment: if the last version sent to that connection for
      this run equals the chain's base, enqueue the frame `{run_id, base_version, version, ops}`;
      otherwise enqueue a snapshot (note 10). Encode each distinct frame once and share the bytes.
    - Queue full: if the frame does not fit under `sync.outbound_queue_max`, drop it, record the
      metric of note 13 and mark connection and run for a snapshot. The mark is retried at every
      later window end of that run and on a timer of `sync.run_patch_coalesce_interval` until it is
      sent, the connection no longer focuses the thread or the run is no longer active. Never close
      with `4014` for these frames (04 7.2 point 6).
    - A frame whose encoded payload exceeds `sync.event_payload_max_bytes`, or a change published
      with `Truncated`, is replaced by a snapshot; a snapshot that exceeds it goes out as
      `{run_id, version}` with envelope `truncated: true`, and the client loads `getRun` (04 2.2
      rule 2). Ruling for `docs/DECISIONS.md`.
    - Focus pruning of M1-66 (`access.changed`, `thread.moved`, archiving) drops the coalescer
      state of the pruned connection and thread (07 "Immediate effect on revocation" point 3). When a
      run leaves the active states, its state is dropped after the last frame.
    - The coalescer runs in its own goroutine; the log tail and `Hub.Dispatch` never wait on it,
      and it never waits on a connection (02 "Hub and delivery"). Snapshot reads happen in this
      goroutine, only after at least one recipient is admitted, one read per run and window end,
      shared by all connections that need it.
10. Whole document (04 11.3 point 5):
    - `focus`: after M1-66 replaced the set, ask `runs.LiveBox.Documents` for the active runs of
      the threads that are new in the set. For each run whose connection passes `run.read` in
      `Delivery(ctx)`, send exactly one `run.updated` with `snapshot` (the document as `getRun`
      answers), `version` and neither `base_version` nor `ops`, and record that version as last
      sent. A thread already in the previous set and a thread without active run get nothing.
      During catch-up keep a pending mark and send at `catchup.complete`, because M1-66 never
      buffers ephemeral frames.
    - `resume`: the restored focus counts as new, so each restored thread with an active run gets
      one snapshot after `catchup.complete`. Ruling for `docs/DECISIONS.md`: the ephemeral events
      of the pause are lost (04 "Lifecycle" step 7), and without a snapshot every client would
      load `getRun` on its first patch.
    - Snapshot, `current` of the conflict and `getRun` use one projection function (M2-05).
    - `LiveBox.Documents` and `LiveBox.Document` are delivery reads without their own `authorize`
      call: the focus handler has checked `gateway.focus` per thread and the gateway admits every
      recipient with `run.read` before sending, so they load an already checked resource (07 7.4
      rule 1, exception "loading a resource that has already been checked"). Name these queries in the runs
      README and on the exception list of the visible-channel lint of M1-70 (AZ-P12), where
      present.
11. Gateway op `run.update` (04 "Ops": agents only; asyncapi `receive.run.update`):
    - Register it in the dispatcher of M1-64 with action `run.update`; it is enabled through
      `gateway_ops.enabled`.
    - Rate limit `rl.agent_runtime` keyed by the agent before anything else; exhausted gives
      `error {op: "run.update", code: "rate_limited", ref}`.
    - Decode `GatewayRunUpdate` strictly (M1-64 decode rule, `4002`), then call `Runs.Patch`.
    - Success sends no frame: asyncapi defines no reply, and the adapter's next base is
      `base_version + 1`. A failure sends `error {op: "run.update", code, detail, ref}` with the
      codes of note 1 and keeps the connection. `detail` for `validation_failed` names pointer and
      code of the first error (for example `/ops/0: patch_path_forbidden`), for `conflict` the
      reason; never a value. `GatewayError` has no `current`, so after `base_version_conflict` the
      adapter loads `getRun` (09 "Reliability", row "Server restart"). Ruling for
      `docs/DECISIONS.md`; spec gap reported.
12. Permissions: `run.update` is `agent-self`. Patches on the own run are an exception of SR9; only the
    agent's permissions count (07 "SR9 Acting-for binding"), which M2-14 must keep. No permission version and
    no `access.changed`. The receive action of `run.updated` is `run.read` (05 6); an app needs
    `messages:read` (M1-28 scopes). The cost member is sent as stored; showing amounts only for
    `organization` is the rule of the clients and of M2-22 (10 12.2, BR-11).
13. Metrics, only through the registry of M1-68 (`internal/platform/otel/metrics`) and the
    outgoing ports of 02 "Layers within a module" rule 5: `bothy.gateway.outbound.dropped` with
    `bothy.reason = patch_replaced_by_snapshot` per dropped frame (signals.md 4.1), and
    `bothy.run.steps` with `bothy.agent.handle` and `bothy.step.kind` per step added by an
    adapter, summary steps excluded, after the commit (signals.md 4.3). M1-68 is in this
    ticket's dependency closure through M2-02: declare both instruments in its registry. No
    other instrument; never ids, values or texts as attributes.
14. Configuration keys used; register the missing ones in `internal/wire/configkeys_<module>.go`
    with the chapter constant, as M1-07 does, and regenerate the reference:
    `sync.run_patch_coalesce_interval`, `sync.max_focused_threads`, `sync.outbound_queue_max`,
    `sync.event_payload_max_bytes`, `sync.ws_max_frame_bytes`, `http.max_json_body_bytes`,
    `run.excerpt_max_chars`, `run.step_detail_max_bytes`, `run.max_steps_persisted`,
    `ratelimit.rl.agent_runtime.per_minute`, `ratelimit.rl.agent_runtime.burst`,
    `database.tx_retry_max`. `agent.run_update_min_interval` is the adapter's own throttle (04
    11.2); the server enforces only `rl.agent_runtime`. `run.stall_after` belongs to the stall
    check of M2-09.
15. Logs, spans and problem details never carry a patch value, a text or a step detail
    (SEC-LOG-4; 06 "Errors" point 3); log the run id, the op index and the error code only.
16. Rows for `docs/DECISIONS.md` section 10 (`D-M2-10-<n>`, one each with reason): order of
    checks and `409 conflict` with `run_not_active` (note 1); clearing `stall_warned_at` (1f); the
    `errors[].code` values `patch_path_forbidden` inside values, `patch_target_missing`,
    `patch_target_exists` and the text-check rule names (notes 2 to 4); `is_estimate` and unique
    plan item ids (note 3); set semantics and normalized ops (note 4); cutting (note 5); the
    summary step (note 6); the coalescing chain, the snapshot instead of a gap frame, the retried
    snapshot mark and the oversize rule (note 9); the snapshot after `resume` (note 10); no reply
    and the error frame of `run.update` (note 11); the correction of ADR 0025, if needed.

## Tests (write them first)

Test world (07 "Examples", white label): space Support with channels Allgemein and Eskalation and
the operator agent Lotse (`organization`), space Marketing with the operator agent Texter. Alex
has `trigger` on Support, Sam has `read` on Support/Allgemein, Robin has no grant on Support. Runs
reach `running` through the paths of M2-08 and M2-09 (or their test helpers); never insert rows
past the module. The agent side is plain HTTP with Lotse's agent token or the gateway test client
`wstest` of M1-64 identified as Lotse.

Unit, `internal/runs/domain`:

- `TestPathTable`: a table over every row of 09 5.3 and every op: allowed combinations pass;
  `/state` (AG-12), `/version`, `/billing_mode_expected`, `/billing_mode_used`, `/queued_at`,
  `/started_at`, `/finished_at`, `/duration_ms`, `/summary`, `/id`, the root, `/cost/currency`,
  `/plan/items/0`, `/metrics/unknown_field`, `/metrics/last_api_retry/attempt`,
  `/steps/<id>/ordinal`, `/steps/<id>/kind`, `add` on `/model`, `remove` on
  `/last_text_excerpt` and `replace` on `/steps/<id>` give `patch_path_forbidden` with pointer
  `/ops/<i>`.
- `TestParsePointer`: `~0` and `~1` escapes, empty and trailing segments, a malformed UUIDv7
  segment gives `invalid_format` at `/ops/<i>/path`.
- `TestValues`: per path the type, enum, bound and closed-object rules of note 3; JSON `null`
  anywhere gives `invalid_type`; `ordinal` in a step value gives `patch_path_forbidden` at
  `/ops/<i>/value/ordinal`; an `id` different from the path gives `invalid`; `is_estimate: false`
  gives `invalid`; duplicate plan item ids give `invalid`; a bidi control in `status_line` and a
  newline in a step `title` give `bidi_control` and `control_character`, the same characters in
  `last_text_excerpt` pass.
- `TestSetSemantics`: `replace` on absent `/model` and absent `finished_at` set the member and
  normalize to `add`; `remove` of an absent `status_line` is dropped; `add` on an existing step id
  gives `patch_target_exists`; a member `replace` on an unknown step and an unknown
  `parent_step_id` give `patch_target_missing`; a parent added earlier in the same patch is
  accepted.
- `TestNormalizedOpsApplyToBase`: property test over random valid patches: the normalized ops,
  applied with the independent RFC 6902 applier (note 4) to the document at `base_version`, give
  the document the change produces.
- `TestCutExcerptKeepsTail` (multi-byte runes, exactly `run.excerpt_max_chars` runes kept),
  `TestCutDetail` (valid JSON, at most `run.step_detail_max_bytes`, `truncated: true`,
  deterministic, fallback `{"truncated": true}`).
- `TestSummarizeSteps`: oldest finished leaf first; summary step created at the lowest removed
  ordinal and then updated; running steps and parents stay; no candidate gives `limit_reached`.
- `FuzzParseAndApply`: random op lists and values never panic and give either field errors or a
  change whose normalized ops satisfy `TestNormalizedOpsApplyToBase`.

Unit, `internal/runs/app` (fakes for the repository, `authorize`, the publisher and the clock):

- `TestPatchOrderOfChecks`: unknown run `not_found`; run of Texter patched by Lotse `forbidden`;
  a person as actor `forbidden`; a forbidden path `validation_failed` without taking the row lock;
  an ended run with a stale base `conflict` with `reason` `run_not_active`; a stale base
  `base_version_conflict` with `current`; a dynamic error rolls back every op of the patch.
- `TestPatchPublishesAfterCommitOnly`: rollback publishes nothing; commit publishes exactly one
  `RunUpdated` with base, new version and normalized ops; a unit of work retried after `40P01`
  publishes once.
- `TestPatchWritesNoEventNoAudit`: the fake appender, the audit recorder and the in-transaction
  bus see nothing on success; the version rises by exactly one for a patch with several ops.
- `TestPatchClearsStallWarning`, `TestPatchOversizeChangePublishedTruncated`.

Unit, `internal/gateway` (fake `LiveBox`, fake `Admit`, test clock, small `sync.outbound_queue_max`):

- `TestMergeOps`: a later `replace` replaces an earlier `replace`; `add` then `replace` on the same
  path stays one `add` with the later value; ops on other paths keep their order; property test:
  the merged ops equal the sequential application.
- `TestCoalescerOneFramePerWindow`: three patches inside `sync.run_patch_coalesce_interval` give
  one frame from the first base to the last version.
- `TestCoalescerGapRestartsChain`, `TestCoalescerJoinedConnectionGetsSnapshot`.
- `TestCoalescerQueueFullGivesSnapshot` (plan row): the frame is dropped, no `4014`, the mark is
  retried until the queue has room, then exactly one snapshot with the newest version is sent.
- `TestCoalescerAdmitsAtFlush`: `run.read` removed between patch and window end gives no frame.
- `TestCoalescerDropsStateOnPruneAndRunEnd`, `TestCoalescerNoFrameBeforeCatchupComplete`,
  `TestCoalescerOversizeFrame` (snapshot, then `truncated` frame with only `run_id` and `version`),
  `TestCoalescerNeverBlocksDispatch`.
- `TestFocusSendsDocumentOnce` (plan row): a newly focused thread with an active run gets exactly
  one snapshot; the same set again gets none; a thread without active run and an unreadable thread
  get none; during catch-up the snapshot waits for `catchup.complete`.
- `TestResumeSendsSnapshotForRestoredFocus`.
- `TestRunUpdateOp`: success sends no frame; `rate_limited`, `not_found`, `forbidden`,
  `validation_failed` (detail `/ops/0: patch_path_forbidden`), `base_version_conflict` and
  `conflict` give `error` frames with `ref` echoed and the connection stays open; a malformed `d`
  closes with `4002`.

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, HTTP through the
real chain, the gateway through `wstest`):

- AG-12 (09 "Required scenarios": "Patch on `/state` or with an old `base_version`" gives
  `400 validation_failed` (`patch_path_forbidden`) and `409 base_version_conflict` with the
  document): `TestAG12PatchStateRejected` over REST and over `run.update`: REST answers `400`
  with `errors[0]` `{pointer: /ops/0, code: patch_path_forbidden}`, the gateway answers an
  `error` frame with code `validation_failed`; `runs.version` and every column are unchanged.
  `TestAG12StaleBaseVersion`: REST answers `409 base_version_conflict` whose `current` equals the
  body of `getRun` at that moment; the gateway answers `error` with `base_version_conflict`.
  Record the red run of both before the implementation (M2 exit criterion 1, M2-31).
- `TestPatchRunStoresDocument`: `status_line`, `model`, metrics, `last_api_retry`, plan, cost, a
  step `add` (server `ordinal`), then `replace` of its `status` and `finished_at`; `getRun` shows
  every value, the version rises by one per patch, `RunPatchResult.version` matches.
- `TestPatchRunConcurrentSameBase`: two parallel patches with the same base: one `200`, one
  `409 base_version_conflict`, never both stored.
- `TestPatchRunNotOwnRun`: Texter on Lotse's run in Support, which Texter cannot read, gives
  `404` with the same status, body and headers as an unknown run id; a third agent with a `write`
  grant on Support (09 "Grants of an agent") gives `403`; Alex with his session gives `403`;
  each denial reaches the audit log.
- `TestPatchRunRejectedAfterCancelTimeout` (AG-14, the patch clause: "later patches
  rejected"): `cancelRun` by Alex, no answer from the adapter, the job `runs.cancel_timeout`
  after `agent.cancel_grace` on the test clock, then a patch with the current base gives
  `409 conflict` with `reason` `run_not_active`; the same over `run.update`.
- `TestDurableEventsIndependentOfPatchRate` (plan row, load test; ADR 0025 Confirmation): run A
  gets a few patches, run B many times as many in the same period over REST and the gateway
  (rate limit configured for the test, not disabled); afterwards the `event_log` rows of both runs
  have the same types and counts (only state changes), `sequence_counters` `event_seq` and
  `audit_seq` did not move during the patches, a `LISTEN event_log` connection received nothing
  during the patches, and the live-box anchor message has no new version and no revision.
- `TestRunUpdatedOnlyToFocusedReaders`: Alex focuses the thread and receives coalesced
  `run.updated` frames; Sam, who can read but does not focus, receives none; Robin's focus on the
  thread is dropped silently and he receives none; every frame has no `seq`, no
  `permission_version`, `resource {kind: run, id, version}` and no JSON `null`.
- `TestRunUpdatedCoalesced`: a burst of patches inside one window reaches Alex as one frame whose
  ops, applied to his snapshot, give `getRun`.
- `TestFocusFullDocumentOnce`: Alex focuses a thread with an active run and gets exactly one
  `run.updated` with `snapshot` equal to `getRun`; the next frame has `base_version` equal to the
  snapshot's version.
- `TestQueueFullGivesSnapshot`: Alex's client stops reading with a small
  `sync.outbound_queue_max`; patches continue; the connection is not closed; after Alex reads
  again he gets one snapshot with the newest version.
- `TestClientRuleConverges`: a reference client that applies 04 11.3 point 4 (apply on equal base,
  ignore older, `getRun` on a gap) over random patch streams with state transitions, queue drops
  and refocus ends with the document of `getRun`.
- `TestStepSummarizationBounded`: with a small `run.max_steps_persisted` in the test
  configuration, the run never holds more steps than the key allows, `listRunSteps` shows the
  summary step in ordinal order, and the frames carry the `remove` and summary ops.
- `TestPatchContentNeverLogged`: a marker string in an accepted `status_line`, in a rejected value
  and in a step `detail` appears in no log line, span attribute, metric or problem `detail`.
- `TestPatchRunSchemas`: responses and `run.updated` frames decode with the generated types of
  `api/rest` in strict mode (`RunPatchResult`, `Problem`, `RunUpdatedEvent`).

Commands that must pass, from the repository root:

```sh
cd services/server && go test -race -count=1 ./internal/runs/... ./internal/gateway/... ./internal/kernel/... ./internal/wire/... ./test/integration/...
make codegen-check
make arch
make api-lint
make api-checks
make register-check
make catalog-check
make config-reference && git diff --exit-code docs/operations/configuration.md
make config-doc-check
make docs
make licenses
make check
```

Run `FuzzParseAndApply` once locally with `go test -run=^$ -fuzz=FuzzParseAndApply
-fuzztime=<duration of your choice>` and report duration and result; commit any crashing input as
a seed after the fix.

## Acceptance criteria

- [ ] `patchRun` and the gateway op `run.update` are enabled, registered with action `run.update`
      and rate-limit class `rl.agent_runtime`, and call the same use case.
- [ ] AG-12: a patch on `/state` gives `400 validation_failed` with `patch_path_forbidden` and a
      stale `base_version` gives `409 base_version_conflict` with the document, over REST and the
      gateway; the red runs are in the report.
- [ ] Patches follow RFC 6902 on exactly the paths of 09 5.3 (`run_document.v1`); the normalized
      ops applied to the base document give the stored document.
- [ ] Only the agent of the run patches, only while the run is active; after `cancel_timeout`
      patches are rejected with `409 conflict` (`run_not_active`).
- [ ] The number of durable events per run does not depend on the patch rate (load test); patches
      take no `event_seq`, no `audit_seq` and send no `NOTIFY`.
- [ ] `run.updated` is ephemeral, reaches only connections that focus the thread and may read the
      run, and is coalesced per `sync.run_patch_coalesce_interval`.
- [ ] Focusing a thread with an active run sends the whole document once.
- [ ] A full outbound queue gives a `snapshot` instead of the dropped patches and never `4014`.
- [ ] `last_text_excerpt`, `detail` and the number of steps stay within their keys.
- [ ] No patch content in logs, spans, metrics or problem details.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `runs` and `gateway` updated (port `Runs.Patch` and `LiveBox`, the path table, the delivery reads of note 10, the coalescer, the op `run.update`, configuration keys, the follow-up port left for M2-11).
- `docs/spec/09-agents.md` 5.3 and `docs/api/openapi.yaml` (`patchRun`) carry the API-visible rulings; ADR 0025's Confirmation names `400 validation_failed`.
- Rulings of note 16 recorded in `docs/DECISIONS.md` (one row each with reason).
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- The red runs of AG-12 recorded in the report (M2 exit criterion 1).
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. The red team checks at least: patching another agent's run, patching an ended run, forbidden and deeply nested paths, oversized and `null` values, bidi controls in display texts, `run.updated` reaching an unfocused or unauthorized connection, and patch content in logs or error details (PLAN M2 "Red team"; T-AZ-01, T-IN-01, T-DS-01).
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
