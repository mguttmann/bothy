# M8-06: Binding of imported session references

Status: ready | Depends on: M2-22, M2-34, M8-04 | Milestone: M8

## Goal

After this ticket an imported thread can continue the agent session it had before the migration. `bothy import discord --apply --sessions <file>` reads JSON lines `{"discord_thread", "agent", "external_session_ref"}` with the optional `cost_baseline_micros` and `currency`, drops (and counts) every line whose agent is not the planned operator of the thread's channel, and sends the reference with the thread item; the server writes one earlier `agent_sessions` row per thread (`is_current = false`, `started = true`, `ended_at` = import time, `end_reason = import`, the cost baseline) exactly once, also when a later run brings the file for an already imported thread, and never after the thread has had a current session. The binding at the first trigger, the compare-and-swap, the fail-open line and "never twice" were built by M2-12; this ticket proves them on rows written by the import path, end to end. For costs, the bound session now carries its baseline to the adapter (`AgentSession.cost_baseline_micros` and `currency`), and the Claude Code runner, which reports per run the difference of cumulative session values, starts from that baseline instead of zero, so the first run after a binding does not book the old session's amount again (RK-75, 22 9 step 7). The report fills its column "Sessions" (taken over, discarded). Why: threads must keep their agent and history after the switch-over (22 9, R-IM-10, REUSE L15 and L16).

## Read first

- `docs/spec/22-import-migration.md`: 5.1.1 (operator of imported threads), 9 (steps 1 to 7), 10.1 (`--sessions`), 11 (column "Sessions"), 16 (row "Binding once"), 17 (R-IM-08, R-IM-10).
- `docs/spec/03-data-model.md` (core contract, read only): `agent_sessions` (all columns, `end_reason` meanings, the partial indexes, `cost_baseline_micros`, `currency`, `cumulative_cost_micros`), `messages.system_code` `agent_session.resume_failed`, R-DM-42.
- `docs/spec/09-agents.md`: 4.4 (invocation payload with the session), 4.10 (usage), 9 (sessions; 9.5 import binding and fail open), 18.7 (runner costs).
- `docs/spec/10-backend-policy.md` 12 (cost booking, estimates).
- `docs/REUSE.md`: L15, L16; `docs/RISKS.md`: RK-75.
- `docs/api/openapi.yaml`: `AgentSession`, `agent.invocation` payload in `docs/api/asyncapi.yaml`, `ImportThreadData` (M8-11), `ImportItem`.
- `docs/DECISIONS.md`: D-IM-06, D-K-I5, D-K-I6, D-W-104 (R-DM-42), D-W-100, D-W-120, D-W-124; rows `D-M2-12-<n>`, `D-M2-22-<n>`, `D-M8-04-<n>`, `D-M8-11-<n>`; section 10 format.
- Briefs (merged names win, D-W-100): `M2-12-agent-sessions.md` (Out of scope on the overlap with this ticket, Design notes 9 and 14, `agentstest.InsertImportedSession`, `TestImportBindingOnce`, AG-18), `M2-22-costs.md` (note 6: the baseline is not subtracted there and is owned here), `M2-34-claude-code-runner-core.md` (requirement 21 and the cost difference against the last stored values), `M8-04-import-tool.md`, `M8-11-import-content-items.md`.
- Code on the base branch: `services/server/internal/agents`, `internal/importer`, `internal/runs`, `internal/billing`, `services/server/cmd/bothy/importtool`, `plugins/claude-code-runner`, `sdk/`.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Session references in tests are invented strings.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`, `actions.yaml`) is not edited by this ticket. Everything this ticket adds is English.

Product requirements implemented here:

11. One earlier row per accepted line: `is_current = false`, `started = true`, `ended_at` = import time, `end_reason = import`, `cost_baseline_micros` and `currency` from the line (22 9 step 1, 03).
12. A line whose agent differs from the thread's `operator_agent_id` (the planned operator) is dropped and counted (22 9 step 1).
13. Exactly once: a repeated run or a second file never creates a second row for the same thread, agent and reference; a thread that has had a current session since the import gets no new import row (22 9 step 6).
14. The binding itself stays as M2-12 built it: first permitted trigger, compare-and-swap, fail open with exactly one system line, never after a reset (22 9 steps 2 to 6, I2).
15. Costs: the session's baseline reaches the adapter; a runtime that reports cumulative session values books per run only the part above the last known state, which after a binding starts at the baseline (22 9 step 7, 09 18.7, RK-75).
16. References are opaque and never logged; audit details carry IDs and codes only (SEC-AUD-5).

## Scope

All paths are repository paths. Every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`, every new Python file with `# SPDX-License-Identifier: Apache-2.0`.

Create:

- `services/server/cmd/bothy/importtool/sessions.go`: parse and validate the `--sessions` file (Design note 1); hand accepted references to the plan of M8-04; report counts.
- `services/server/internal/agents` port `ImportedSessions.CreateEarlier(ctx, EarlierSession) (Outcome, error)` (note 2), implemented in `app` with a sqlc query; the production twin of `agentstest.InsertImportedSession`, which now calls it.
- Tests named under "Tests".

Change:

- `docs/api/openapi.yaml`: `ImportThreadData` gains the optional member `earlier_session{agent_id, external_session_ref, cost_baseline_micros?, currency?}`; `ImportItem` gains the optional member `session` (`taken_over`, `already_present`, `dropped_late`); `AgentSession` gains the read-only members `cost_baseline_micros` and `currency` (present when the baseline is above zero); `make api-lint`, `make api-checks`, `make codegen-check`, `make api-clients` (the generated SDK clients follow).
- `services/server/internal/importer/app/content.go` of M8-11: apply `earlier_session` of a thread item (note 3).
- `services/server/internal/agents`: include the baseline in `AgentSession` responses and in the session object of the invocation payload.
- `plugins/claude-code-runner`: the cost difference starts from the session's `cost_baseline_micros` (amount only) when the runner has no stored state for a session it was asked to bind (note 4).
- `services/server/cmd/bothy/importtool/report.go` (column "Sessions"), `README.md`.
- Docs: `docs/spec/22-import-migration.md` 9 steps 1 and 7 (thread item member, late file rule, the baseline in the session object), `docs/spec/09-agents.md` 4.10 and 18.7 (the rule of requirement 15 and the token limitation of note 4), READMEs of `agents`, `importer`, the runner; `services/server/CHANGELOG.md`, the runner's CHANGELOG; `docs/DECISIONS.md` section 10 rows `D-M8-06-1` to `D-M8-06-5`.

This ticket adds no table, no migration, no action, no event type, no configuration key and no top-level make target.

## Out of scope

- Re-building the binding, the fail-open line or the reset rule (M2-12; only reused and proven here).
- Booking changes in `billing` beyond what the runner reports (M2-22 books values as given).
- Cost baselines in other adapters: the Hermes adapter reports per-run values from its runtime; if its merged code computes differences of session sums, the same rule applies and the report lists it as a follow-up.
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M8-06-session-binding-import`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(agents): ...`, `feat(import): ...`, `fix(claude-code-runner): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Never contact Discord, a model provider or any real service: Discord is the fake of M8-01, the runner tests use the fake Claude of M2-34, real-server tests use the harness of M1-71 with `pgtest`. No real model call (PLAN 3 point 5). Without a Docker-compatible runtime the real-server tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `t.TempDir()` or `mktemp -d`, removed at the end.
- Never read or print secrets; external session references count as confidential and are never logged.
- Time only through `kernel/clock` (Go) and the injectable clock of the runner; tests use fake clocks.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M8 by D-W-124). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Sessions file** (22 9 step 1, 10.1). JSON lines, one object per line, members exactly `discord_thread` (digits), `agent` (handle), `external_session_ref` (non-empty, at most `agent.session_ref_max_bytes` bytes, the text rules M2-12 applies), optional `cost_baseline_micros` (integer at least 0) and `currency` (ISO 4217, required when the baseline is above 0). An unknown member, a bad value, a duplicate `discord_thread` or an unknown agent handle ends the run with exit `2` before any write, naming the line number, never the reference. A line whose thread is not part of the run (other channel, `--only-thread`) is ignored and counted. A line whose agent is not the `planned_operator` of the thread's channel in the stored mapping is dropped and counted as "discarded (agent does not match)" (22 11). `--sessions` with a dry run validates and counts without writing. `--sessions` with `--follow` is allowed (new threads of the mirror get their reference when their line exists).

2. **`CreateEarlier`** (03 `agent_sessions`; Ruling for docs/DECISIONS.md). In the batch transaction: when the thread has any row whose `end_reason` is not `import`, or is current, return `DroppedLate` and write nothing (22 9 step 6); when a row with the same thread, agent and reference and `end_reason = import` exists, return `AlreadyPresent`; else insert the row of requirement 11 (`ended_at` = the transaction's clock time, `model` empty, `cumulative_cost_micros = 0`) and return `TakenOver`. No audit entry and no event: the row is history, not a change of the current session (22 14 audits only "bindings"; the binding of M2-12 audits). The importer passes `agent_id`, which must equal `threads.operator_agent_id`, else `400 validation_failed` at `/items/<i>/data/earlier_session/agent_id` (the tool already dropped such lines).

3. **Thread item with `earlier_session`** (M8-11 note 3). For a new thread the row is created after the thread; for a posted thread the item still applies `earlier_session` through `CreateEarlier` (a later run with a sessions file reaches threads imported before), and the item's `session` member reports the outcome; the stats counters `server.sessions_taken_over`, `server.sessions_already_present`, `server.sessions_dropped_late`.

4. **Baseline to the adapter and the runner** (09 18.7, 4.10; RK-75; Ruling for docs/DECISIONS.md). `AgentSession` responses and the session object in `agent.invocation` carry `cost_baseline_micros` and `currency` when the baseline is above zero. The runner of M2-34 keeps the last cumulative values per session; when it receives a session with mode `bind` for which it has no stored values, it initializes the stored amount (`costUSD` in micros of the session currency) with `cost_baseline_micros` and the stored token counters with zero, because no token baseline exists; amounts of the first run after a binding are then only the part above the baseline. Tokens of that first run can include the old session's tokens, because 03 stores no token baseline: write this limitation into 09 18.7 and list it under "What is not proven" (a token baseline would be a core contract addition and is not needed for amounts, which are what RK-75 is about). A baseline in another currency than the runtime reports is ignored with a warning line in the runner log (no conversion).

5. **Rulings for `docs/DECISIONS.md`** (`D-M8-06-<n>`): the sessions file rules (note 1); `CreateEarlier` outcomes and the late rule (note 2); the thread item member and posted threads (note 3); the baseline in `AgentSession` and the runner start value, with the token limitation (note 4).

## Tests (write them first)

- `importtool/sessions_test.go`: one row per validation rule of note 1; dropped agent mismatch is counted; dry run writes nothing.
- `internal/agents` with `pgtest`: `TestCreateEarlier` (taken over, already present, dropped late after a reset row or a current row).
- `services/server/test/importtool/sessions_test.go` against the real server with the fake Discord and the agent test client of M2-12 (operator Lotse):
    - `TestImportedSessionBindsOnce` (plan acceptance, 22 16 row "Binding once"): the import writes the earlier row; two simultaneous first triggers by Alex in the imported thread give exactly one current row with the imported reference, one audit `agent_session.bind` with `cause` `import`, one `agent_session.changed`, and the invocation carries `session_mode = bind` and the baseline.
    - `TestImportedSessionMissingFailsOpen` (plan acceptance): the adapter reports `bind_failed: session_missing`; a new current row, exactly one system line `agent_session.resume_failed`, Alex's message reached the agent.
    - `TestResetNeverBindsAgain` (plan acceptance): after `resetAgentSession` the next trigger starts a new session; a second import run with the same file reports `dropped_late` and writes nothing.
    - `TestSecondRunAlreadyPresent`.
- `plugins/claude-code-runner` tests with the fake Claude: `test_bound_import_session_books_above_baseline` (plan acceptance: a first `result` with a cumulative cost above the baseline reports exactly the difference; a second run reports the difference to the first); `test_baseline_other_currency_ignored`.
- Secret test: no external reference in any log line of the tool, the server (debug level) or the runner.

Commands that must pass, from the repository root:

- `cd services/server && go test ./internal/agents/... ./internal/importer/... ./cmd/bothy/... ./test/importtool/... -race -count=1`
- the runner's test command as its README names it (for example `cd plugins/claude-code-runner && uv run --locked pytest`)
- `make api-lint`, `make api-checks`, `make codegen-check`, `make api-clients`, `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Two simultaneous first triggers bind an imported row exactly once.
- [ ] A missing old session results in a new session and exactly one system message.
- [ ] A later reset never binds again; a repeated import never adds a row.
- [ ] A cumulative cost report of the runner books only the part above the baseline.
- [ ] The report shows sessions taken over and discarded.
- [ ] OpenAPI, 22, 09, READMEs, CHANGELOGs and rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every exported Go symbol and public Python function; comments explain why.
- READMEs of `agents`, `importer`, the import tool and the runner updated in English.
- Rulings of Design note 5 recorded in `docs/DECISIONS.md` section 10.
- CHANGELOG entries under `[Unreleased]`.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (at least the token counts of the first run after a binding, note 4).
5. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
