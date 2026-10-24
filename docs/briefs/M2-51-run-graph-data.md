# M2-51: Run graph data and stall detection

Status: ready | Depends on: M2-35 | Milestone: M2

## Goal

After this ticket the run document carries everything the run graph and the control room need (09 5.6, [design note](../design/run-visualization.md)), and the stall notice "no movement since ..." is a server fact. The Claude Code runner links every task step (`subagent`, `workflow`, `background_job`) to the step of the tool call that started it (`tool_use_id`), maps a workflow task to kind `workflow` once the recording of the pinned version names its `task_type`, and copies the usage of a task into the step `detail` member `usage`; it turns `tool_progress` into a patch of `/metrics/progress_events` at most once per `runner.progress_patch_interval`. The server keeps one summary step per parent when it merges old steps (09 5.3), stores `runs.last_progress_at` on every start and stored patch, and `runs.stall_check` sets `stall_warned_at` for a `running` run without progress for `run.stall_after`, with a version step and a `run.updated`; `stall_warned_at` is a member of the API document `Run` and of `RunSummary`. Why: the owner wants to see what a run and the agents it started do right now (2026-10-24); a tree needs stable parent references from the only party that knows them, a merge of old steps must not flatten it, and "stalled" must not be guessed by a client clock (D-RV-3 to D-RV-6, D-RV-9).

## Read first

- `docs/design/run-visualization.md` (all), `docs/decisions.d/run-visualization.md` (`D-RV-1` to `D-RV-12`).
- `docs/PLAN.md`: section 3 "Way of working per ticket"; section 8 "M2 Agent platform, extensions and terminal" and the rows M2-09, M2-10, M2-34, M2-35 (what exists) and M2-51 (this ticket); section 10 row M4-39 (who builds on it).
- Dependencies: the header names M2-35. Through it this ticket builds on M2-34, M2-29, M2-12, M2-10, M2-09, M2-03 and everything below them (all implied, so not in the header).
- `docs/spec/09-agents.md`: 5.3 (path table, rules of `patchRun`, the summary step and its addendum of 2026-10-24), 5.4, 5.5 (the row "No progress" and the paragraph of 2026-10-24), 5.6 (all), 18.4 (rows `system` `task_*` and `tool_progress`, the step title paragraph), 19 (R-AG-26, R-AG-27), 21 (`runner.progress_patch_interval`, `run.stall_after` through 03).
- `docs/spec/03-data-model.md` (core contract, read only): `runs` (`last_progress_at`, `stall_warned_at`, `metrics` with `progress_events`), `run_steps` (`parent_step_id`, `detail`, the summary rule), R-DM-53; 9 (`run.max_steps_persisted`, `run.stall_after`).
- `docs/spec/04-sync-protocol.md` (core contract, read only): 11.1 (member `stall_warned_at`), 11.3 items 3 to 7, R-SY-27.
- `docs/spec/02-architecture.md`: 12.3 row `runs.stall_check`.
- `docs/api/openapi.yaml`: schemas `Run`, `RunSummary`, `RunStep`, `RunMetrics`, `RunPatch`, `JsonPatchOperation`; operations `getRun`, `listRunSteps`, `listActiveRuns`, `patchRun`.
- `docs/decisions.d/M2-09.md` (D-M2-09-19 to D-M2-09-26: `runs.stall_check`), `docs/decisions.d/M2-10.md` (D-M2-10-2 clearing `stall_warned_at`, D-M2-10-5 normalized ops, D-M2-10-7 the summary step), `docs/decisions.d/M2-34.md` and `M2-35.md` where present.
- Briefs: `docs/briefs/M2-09-run-lifecycle.md` (note 10, `runs.stall_check`), `M2-10-run-document-live-box.md` (patch use case, summary step, normalized ops), `M2-34-claude-code-runner-core.md` (stream reader, live box mapping), `M2-35-claude-code-runner-paths.md` (Fake-Claude scenarios); merged code wins on names.
- Code on the base branch: `services/server/internal/runs` (patch, summary, `runs.stall_check`, `listActiveRuns`), `services/server/migrations` (latest `runs` migrations), `plugins/claude-code-runner/src/claude_code_runner/` (`streamreader.py`, `livebox.py`, `usage.py`) and its `tests/fixtures`, `sdk/python` (generated models).

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
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) already holds this ticket's changes and is not edited by it.

Product requirements implemented here:

11. A task step whose `task_*` message carries a `tool_use_id` of a tool call step of the same run has that step as `parent_step_id`; otherwise it has no parent. A tool call keeps its parent from `parent_tool_use_id` (09 18.4, D-RV-3).
12. A task whose `task_type` is the workflow type recorded for the pinned version is a step of kind `workflow`; without a recorded value no step is `workflow` (09 18.4, D-RV-4).
13. The usage of a task (`total_tokens`, `tool_uses`, `duration_ms`), where the stream reports it, becomes `detail.usage` (`tokens`, `tool_uses`, `duration_ms`); the runner never sets `cost_micros` from an estimate (09 5.6, D-RV-5).
14. `tool_progress` leads to `replace /metrics/progress_events` only when the last stored patch of the run is older than `runner.progress_patch_interval` (09 18.4).
15. Above `run.max_steps_persisted` a removed step is counted into the summary step of its own parent; a new summary step carries that parent; running steps and parents are never removed; old summary steps stay as they are (09 5.3, R-DM-53).
16. `runs.last_progress_at` is set to the server time by every successful `run.start` and every stored patch; `runs.stall_check` marks a `running` run whose `last_progress_at` is older than `run.stall_after` and whose `stall_warned_at` is empty, with one version step and `run.updated` `add /stall_warned_at`; the next stored patch carries `remove /stall_warned_at`; a run in `waiting_input` is never marked (09 5.5, 04 11.3 item 7).
17. `Run` and `RunSummary` carry `stall_warned_at`; `RunMetrics` carries `progress_events`; `listActiveRuns` returns `stall_warned_at` of every active run.
18. No new action, event, op or receive rule (D-RV-10, R-SY-27).

## Scope

Server (`services/server/`):

- Migration `runs_last_progress_at` (next free number): column `runs.last_progress_at timestamptz NULL`, no backfill (an active run without a value is marked only after its next start or patch), `make migrations-lock`.
- `internal/runs`: `last_progress_at` in the start and patch writes (the same `UPDATE` that raises `version` and clears `stall_warned_at`); normalized op `remove /stall_warned_at` when a patch clears it; summary step per parent in the summary use case; the stall marking in the `runs.stall_check` handler (one unit of work per marked run, compare-and-swap on `version`); `stall_warned_at` in the document and summary encoders; `progress_events` accepted as a metric.
- `docs/api/openapi.yaml`: `Run.stall_warned_at`, `RunSummary.stall_warned_at` (both `Timestamp`, optional), `RunMetrics.progress_events` (integer, optional), the description of `RunStep.detail` naming `usage`; regenerate every generated client and server part (each component's `generate` target; `make codegen-check` and `make api-clients` prove it).
- `docs/spec/02-architecture.md` 12.3 row `runs.stall_check`: add the stall marking.

Runner (`plugins/claude-code-runner/`):

- Stream mapping in `streamreader.py` and `livebox.py` (names of the merged code win): the map from tool use ID to step ID also serves task messages; kind `workflow`; `detail.usage`; the progress patch with `runner.progress_patch_interval` in `config.py` and `config.example.toml`.
- A recording of the pinned Claude Code version (Design note 1) under `tests/fixtures/recording/` and Fake-Claude fixtures for every case of the Tests.
- README (stream mapping table) and `changelog.d/M2-51.md`.

Fragments: `docs/decisions.d/M2-51.md` with rows `D-M2-51-<n>`; `changelog.d/M2-51.md` at the root for the server and `plugins/claude-code-runner/changelog.d/M2-51.md`; regenerate `docs/operations/configuration.md` section 4 if the configuration key table changed (`make config-reference`).

## Out of scope

- Drawing the graph and the control room (M4-39, M5-14); the derivation function of 09 5.6 lives in `bothy_viz` (M4-39).
- A Hermes signal for subagents (the pinned Hermes version reports none, 09 17.3).
- Costs per node from an estimate, a graph across runs, a replay slider (design note section 5).
- Any change to permissions, actions, events, gateway ops or the focus limit.
- Changes to the core contract (already made by the design change).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M2-51-run-graph-data`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(runs): ...`, `feat(claude-code-runner): ...`, `docs(spec): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the README, fragment and generated files named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; database tests use one database per test from `pgtest`; time comes from the injected clock, never from a fixed sleep.
- No real Claude Code session, no login and no model call in tests: the runtime is Fake-Claude. The pinned CLI runs only for the recording of Design note 1, behind the explicit switch of M2-35, without login and without a model call; if a recording needs either, do not record and say so under "What is not proven".
- Never read or print secrets. Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M2 closed, in the manner of D-W-100. Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Recording first.** Before the mapping code, read the Agent SDK reference cited in 09 18.4 and, behind the switch, record the stream of the pinned version for a turn that starts a subagent, a background command and, if the version offers one without a login, a workflow. Keep in the fixture only the message types, subtypes and field names (no content). The mapping uses `tool_use_id`, `usage` and the workflow `task_type` only as far as the recording shows them; update the [derived] note of 09 18.4 with what was verified and rule it.
2. **Parent of a task.** The runner already keeps the step ID per `tool_use` ID. On `task_started` it looks up `tool_use_id`; a hit gives `parent_step_id`, a miss gives none. The value is set in the step `add` only (the server never replaces a parent). A task step added before its tool call step is impossible (the tool call comes first in the stream); if the recording shows otherwise, the task stays at the root and the report names it.
3. **Progress patch.** The runner remembers the time of its last stored patch per run (injected clock). On `tool_progress` it increments its count and sends `replace /metrics/progress_events` only when that time is older than `runner.progress_patch_interval`; the patch goes through the throttled SDK path like every other patch.
4. **Summary per parent.** In the summary use case of M2-10, group the candidates by `parent_step_id` (NULL as one group). The removal order stays the lowest ordinal of the run; each removed step goes into the summary step of its group, created on first use with that `parent_step_id` and the lowest removed ordinal of the group. Normalized ops as in M2-10 per summary step.
5. **Stall marking.** In the `runs.stall_check` round, select `running` runs with `last_progress_at < now - run.stall_after` and empty `stall_warned_at`; per run one unit of work that re-reads and locks the row, sets `stall_warned_at`, raises `version` and publishes the ephemeral `run.updated` with `base_version`, `version` and `add /stall_warned_at`. A run whose row changed in between is skipped (compare-and-swap on `version`).
6. **Rulings for `docs/decisions.d/M2-51.md`** (`D-M2-51-<n>`): what the recording verified; no backfill of `last_progress_at`; grouping of summary steps; the stall marking unit of work.

## Tests (write them first)

Server (`services/server/internal/runs`, with `pgtest` and an injected clock):

- `TestSummary_PerParent`: a run with nested finished leaves under two parents and at the root crosses `run.max_steps_persisted`; each parent gets its own summary step with that `parent_step_id`; running steps and parents stay; normalized ops as M2-10 describes.
- `TestSummary_OldRootSummaryStays`: a run with a stored root summary step keeps it and adds per parent summaries for later merges.
- `TestLastProgress_SetOnStartAndPatch`: `run.start` and a stored patch set `last_progress_at` to the clock time; a refused patch does not.
- `TestStallCheck_MarksRunningWithoutProgress`: after `run.stall_after` without progress one `stall_warned_at`, one version step, one `run.updated` with `add /stall_warned_at`; a second round changes nothing.
- `TestStallCheck_SkipsWaitingInput` and `TestStallCheck_PatchBetweenReadAndLockWins`.
- `TestPatch_ClearsStallWithRemoveOp`: the next stored patch clears the value and its normalized ops carry `remove /stall_warned_at`.
- `TestListActiveRuns_StallWarnedAt` and a contract test that `getRun` returns `stall_warned_at` and `metrics.progress_events`.

Runner (`plugins/claude-code-runner/tests`, Fake-Claude):

- `test_task_parent_from_tool_use_id`: a subagent started by an `Agent` tool call, a tool inside it (with `parent_tool_use_id`) and a background command: the expected chain of `parent_step_id`.
- `test_task_without_tool_use_id_at_root`.
- `test_workflow_task_kind` (with the recorded type; skipped with a reason only if the recording could not name it, and then the report says so).
- `test_task_usage_in_detail`: `usage` becomes `detail.usage`; no `cost_micros`.
- `test_tool_progress_patch_interval`: many `tool_progress` messages within the interval give one patch; after the interval one more.

Run the added tests by name with repetitions as the team rules require (Go `-race -count=20`, pytest repeated), through the machine's test slot.

Commands that must pass, from the repository root:

- `cd plugins/claude-code-runner && uv run --locked pytest`
- `make fmt-check`, `make lint`, `make typecheck`, `make test`, `make coverage`
- `make api-lint`, `make api-checks`, `make codegen-check`, `make api-clients`, `make migrations-lint`
- `make docs`, `make fragments-check`
- `make check`

## Acceptance criteria

- [ ] Task steps carry the parent of their starting tool call where the stream gives `tool_use_id`; tasks without it stay at the root; tools inside a subagent keep their parent from `parent_tool_use_id`.
- [ ] `workflow` and `detail.usage` are mapped exactly as far as the recording of the pinned version shows; 09 18.4 states what was verified.
- [ ] Summary steps are kept per parent; nothing running and no parent is removed.
- [ ] `last_progress_at`, the stall marking with its `run.updated` and the `remove` on clearing work as 09 5.5 says; `waiting_input` is never marked; `tool_progress` keeps a long tool unmarked.
- [ ] `Run.stall_warned_at`, `RunSummary.stall_warned_at` and `RunMetrics.progress_events` are in `openapi.yaml` and every generated client.
- [ ] Fragments written; `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 09 5.3, 5.5, 5.6 or 18.4.
- README of `plugins/claude-code-runner` (stream mapping) and of `internal/runs` updated.
- Rulings in `docs/decisions.d/M2-51.md`; changelog fragments as in Scope.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (for example a field the recording could not show).
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
