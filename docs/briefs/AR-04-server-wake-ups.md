# AR-04: Server: wake-ups

Status: ready | Depends on: none (specification merged) | Milestone: AR

## Goal

After this ticket the server keeps wake-ups and starts work from them: `run_wakes` and `run_autonomy`, `trigger_kind = 'wake'`, the operations of 09 5.8.2 including `run.launch.check`, the heartbeat capabilities `turn_handover` and `wakes`, the event `run.wake_changed` to clients and adapter inboxes, the wake input `agent.wake_input` with its token, the job `runs.wake_due`, the fire checks on both paths, the caps of 5.8.6, the notes of 5.8.4 and the notification kind `wake_ended`, the channel switch `allow_autonomous_runs` and the reason `usage_limited`. It runs before any runner switches `runner.turn_handover` or `runner.wakes` on (16 10.1 item 5).

## Read first

- `docs/spec/09-agents.md` 3.2, 3.4, 4.5, 4.8, 5.1, 5.2 (and the paragraph on `usage_limited`), 5.5, 5.8 (all), 7, 18.19.6 (launch check), R-AG-17 (amended), R-AG-48 to R-AG-52, 21 (server keys of stage AR).
- `docs/spec/03-data-model.md` 4.5 `channels`, 4.8 `runs`, `run_wakes`, `run_autonomy`, R-DM-60; `docs/spec/07-permissions.md` 2.14, SR1, SR2, R-RE-13; `docs/spec/04-sync-protocol.md` 15.3, 15.4, 15.9, R-SY-30; `docs/spec/05-actions.md` R-AZ-37; `docs/spec/06-api.md` sections 5 to 7, R-API-30; `docs/spec/10-backend-policy.md` 10.4; `docs/spec/13-push-notifications.md` section 2 (`wake_ended`), R-PN-12; `docs/spec/17-observability.md` section 3, R-OB-11; `docs/spec/02-architecture.md` 12.3 (job catalogue).
- Code on the base branch: `services/server/internal/runs` (domain, app, adapters, `README.md`), `services/server/internal/agents` (heartbeat and capabilities), `services/server/internal/notify/domain` (`Kinds`), `services/server/internal/chat` (channel settings), `services/server/migrations`, `docs/api/openapi.yaml`, `docs/api/asyncapi.yaml`, `services/server/api/operations.yaml`, `docs/spec/actions.yaml`, `docs/observability/signals.md`.
- Design note `docs/design/autonomous-runs.md` (context and evidence; where it differs from the chapters, the chapters win); ADR 0077; `docs/decisions.d/AR-01.md`.

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
8. White label: examples, templates and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no internal script names, no paths under a home directory.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) changes in this ticket only where Scope names it: the catalogue rows of `docs/spec/actions.yaml` that R-AZ-37 assigns to this ticket and their generated copies (`make catalog`).
11. Additive only (09 18.19.1, ADR 0077): every new column is nullable or defaults to "as before", every key defaults to off, and an installation that sets none of the new keys behaves exactly as before.

Note on rule 1: "stages" are the milestones and the stages A1 and AR of the plan (PLAN P-27, P-29).

Product requirements implemented here:

12. Migrations (expand only) create `run_wakes` and `run_autonomy` exactly as 03 says, widen the check of `runs.trigger_kind` with `wake`, add `runs.triggering_wake_id` and `channels.allow_autonomous_runs` (default `false`); the migration tests hold them column by column.
13. The operations of 09 5.8.2 and the action `channel.autonomous_runs.set` with exactly the columns of 07 2.14, in `actions.yaml`, `openapi.yaml` and the enable list; each refusal is `409 conflict` with the reason of 09 5.8 or `400 validation_failed` (R-API-30), each with its own test.
14. Fire checks on both paths (wake run and wake input, 5.8.3), the wake input token and its claims, the return to `pending` after a skipped input, `pending` while another person's run is active; at most one active run per thread stays true.
15. `run.wake.check` and `run.launch.check` with every check of 5.8.3 and 18.19.6 (also archived or read-only channel); a refusal of `run.wake.check` ends the wake-up `refused` with audit entry, note and notification.
16. The caps of 5.8.6 with `run_autonomy` per thread and person: chain count reset only by that person's run or follow-up; idle count fed by `/metrics/autonomy_last` patches, each distinct input counted once; `autonomy_cap_reached` in the patch answer; the minimum delay; the pending cap; the deadline of `usage_limit` from `not_before`; `run.usage_wake_max`; idempotency of `run.wake.recover` and of `usage_limit` per outage.
17. The job `runs.wake_due` (in the job catalogue of 02 12.3) fires due `at` wake-ups and expires past deadlines; every end other than `fired` posts the `system` note; `wake_ended` notifications (13); `run.wake_changed` to clients and adapter inboxes.
18. The heartbeat accepts `turn_handover` and `wakes` in `details.capabilities` (closed list) and keeps them in `reported_details`; the reason `usage_limited` enters 09 5.2 and the reason table of the code (adapter-set, `failed`).
19. Rollback (09 18.19.1): an older server shows a run with `trigger_kind = 'wake'` as the test of the previous version proves, or the rollback note in the runs README says to let pending wake-ups fire or cancel them first; decide one with a test.

## Scope

All paths under `services/server/` unless they start with `docs/`.

Create:

- Migrations `<next number>_runs_wakes.sql` and `<next number>_runs_autonomy.sql` (and the `trigger_kind`, `triggering_wake_id`, `allow_autonomous_runs` steps), with migration tests.
- `internal/runs/app/wakes.go` (create, recover, list, check, fire, cancel, read, launch check, the job), `internal/runs/app/autonomy.go` (caps), their tests; integration tests under `test/integration/runs/`.
- `docs/decisions.d/AR-04.md`.

Change:

- `internal/runs` domain (trigger kind, reason table with `usage_limited`, wake input token claims), `internal/agents` (heartbeat capabilities), `internal/chat` (`allow_autonomous_runs`), `internal/notify/domain` (`wake_ended` in `Kinds`, the enum and 13 section 2 table), privacy participants of the new tables (20 section 2) and the retention coverage.
- `docs/api/openapi.yaml`, `docs/api/asyncapi.yaml`, `api/operations.yaml`, `docs/spec/actions.yaml`, `make catalog`, `make event-catalog`, `make contract-documents` and every generated copy of `make codegen-check`; `internal/wire` (the server keys of 09 21 for stage AR) and `make config-reference`; `tools/configdoc/planned.yaml` (remove the rules of the keys that entered the schema).
- `docs/spec/09-agents.md` 5.2 (move `usage_limited` into the table), `docs/spec/13-push-notifications.md` section 2 (`wake_ended` into the table), `docs/observability/signals.md` (the metrics of 17 section 3 with their code), `internal/runs/README.md`.

## Out of scope

- Anything in the runner (AR-03, AR-05 to AR-07) or the clients (AR-08).
- A dollar budget for autonomous work.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/AR-04-server-wake-ups`; one writer per worktree.
- Conventional Commits with scope and a DCO sign-off on every commit (`git commit -s`), no co-author trailers; squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG fragments, README and `docs/decisions.d/` entries named in this brief, each listed in the report.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL and other services run only as throwaway containers (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, `t.TempDir()` or `tmp_path`, removed at the end.
- Never read or print secrets. Tokens, DPoP keys, secret values and secret references resolved to values never appear in logs, test output, failure messages, audit entries or files outside the places this brief names.
- No real model call and nothing that costs money; a check of the pinned CLI runs only without a login against a local stand-in for the API on `127.0.0.1` (the method of `plugins/claude-code-runner/tests/real_cli/captured`).
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs beyond what Scope names, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before its dependencies were built, under the rule of D-W-100: where the merged code of a dependency names a package, type, function, file, operation or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from `docs/spec/09-agents.md` 5.8 or 18.19, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red`. Confirm that `docs/spec/09-agents.md` has 5.8 and `docs/spec/03-data-model.md` has `run_wakes`, else `Verdict: BLOCKED specification missing`.

2. **Fire in one transaction.** Lock the wake-up row and the thread's active run row; decide wake run, wake input or stay `pending`; apply the caps; write the new run or the delivery and the state change and append `run.wake_changed` in the same transaction.

3. **Wake input token.** The token is an invocation token with `trigger_kind = "wake"` and the claims of 09 5.8.3; it is delivered as `agent.wake_input` with the payload label, reason and result text; a skip reported by the adapter returns the wake-up to `pending` once per delivery.

4. **Counts.** `run_autonomy` rows are created on the first autonomous input; a run or follow-up of the person resets the row in the transaction that creates the run or follow-up.

5. **Rulings for `docs/decisions.d/AR-04.md`**: the rollback choice of requirement 19, the job interval, the payloads, every name that differs from 09 5.8.

## Tests (write them first)

- Go unit and integration (container runtime), `internal/runs`, `internal/agents`, `internal/chat`, `internal/notify`, `migrations`: one test per refusal of each operation; one per end state (`fired`, `cancelled`, `expired`, `refused`, `failed`) with note, audit and notification as 5.8.4 says; the server parts of AR-T08, AR-T09, AR-T13 and AR-T14; the launch check refusals of AR-T04; the deadline of `usage_limit`; `run.usage_wake_max`; heartbeat with and without the capabilities; the rollback behaviour.
- The contract tests that hold 09 5.2 and 13 section 2 against the code stay green with the new rows.

```sh
(cd services/server && go test -race -count=1 ./internal/runs/... ./internal/agents/... ./internal/chat/... ./internal/notify/... ./migrations/...)
make catalog && make catalog-check
make config-reference && make config-doc-check
make check
```

## Acceptance criteria

- [ ] Every operation, refusal and end state of 09 5.8 is tested; AR-T13 and AR-T14 server parts are green.
- [ ] Catalogue, API documents, configuration reference, 09 5.2, 13 section 2, `signals.md` and every generated copy are current.
- [ ] Existing runs, channels and heartbeats are unchanged by the migrations (an integration test with rows from before).
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- `internal/runs/README.md` describes wake-ups, the caps, the job and the rollback.
- An entry `changelog.d/AR-04.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/AR-04.md`, one row each with date, ruling and reason.
- An ADR only if you take an architecture decision beyond ADR 0077 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts. The red team checks at least: no wake-up fires or allows a host execution without the checks of that moment; the caps cannot be passed through wake inputs, notification turns or another person's message; every end other than `fired` is visible; at most one active run per thread; a refused check is audited.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team, `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (at least: the behaviour with a real runner (AR-05 to AR-07, AR-10)), and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
