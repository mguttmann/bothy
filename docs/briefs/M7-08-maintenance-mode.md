# M7-08: Maintenance mode `read_only` (ADR 0065)

Status: ready | Depends on: M7-09 | Milestone: M7

## Goal

After this ticket an owner can freeze an instance for a relocation or an upgrade without stopping it: with `server.maintenance_mode = read_only` (read at the start of `api` and `worker`), every action with `effect: write` in the catalogue that has no exception gets `503` with the problem code `maintenance` and `Retry-After`, people keep reading, searching and downloading, and running runs can finish (ADR 0065, 05 rule 1.9, 16 11 step 3). The layer sits after the rate limit and before `authorize` (05 2.5, 02 "Request path" row 9); it writes no audit entry and counts rejections in a metric. A second line catches what the layer misses: the Unit of Work opens every transaction `READ ONLY` except for the exceptions, PostgreSQL rejects writes with SQLSTATE `25006`, and the server answers `503 maintenance`; side writes such as `last_used_at` are dropped. The exceptions of ADR 0065 item 4 hold: finishing running runs (`run.update`, `run.finish`, `agent.heartbeat`, `agent.events.consume` and `message.create` and `file.upload` in the run's thread for its agent; `run.cancel` for people), `auth.token.refresh` returns a new access token without rotating the refresh token and without a write, the emergency actions (`auth.step_up`, `session.revoke`, `device.revoke`, `token.revoke`, `user.disable`) and the revocation of a family on refresh token reuse work, `gateway.connect` works, and in the `worker` only the consumers `push` and `webhooks`, the queue `delivery` and the job `runs.cancel_timeout` run. The status page shows the maintenance, derived from the configuration without a write. Why: a relocation needs a final backup that equals the state at the end of the window (16 11), and an overlooked write path must fail loudly instead of creating data that would be missing on the target.

## Read first

- `docs/adr/0065-maintenance-mode-read-only.md` (all, including "Confirmation").
- `docs/spec/05-actions.md` (core contract, read only): rule 1.9, 2.5 (order of the layers), 2.7 (`effect`), the rows of the exception actions; `docs/spec/actions.yaml` (`effect` per action).
- `docs/spec/06-api.md` (problem code `maintenance`, `Retry-After`), `docs/api/openapi.yaml` (problem code row, the `503` responses).
- `docs/spec/02-architecture.md`: "Request path" (row 9), 11.5, 12.2 (the paragraph on maintenance mode in the `worker`), 12.3; `docs/spec/08-identity.md` 7.4 (refresh without rotation in maintenance mode); `docs/spec/16-operations.md` 11 (relocation), 12 (status page), 14 (`server.maintenance_mode`).
- `docs/operations/cloud-migration.md`, `docs/operations/upgrade.md`.
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125); section 10 (format of `D-M7-08-<n>` rows).
- Briefs (merged names win, D-W-100): `M1-18-http-chain.md` (stages of the chain), `M1-19-rate-limits.md`, `M1-13-unit-of-work.md` (transactions, phase hooks), `M1-34-identity-sessions.md` (refresh rotation and reuse), `M1-45-identity-revocation.md` (emergency actions), `M1-14-background-jobs.md` and `M2-01-worker-event-consumers.md` (queues and consumers), `M2-09-run-lifecycle.md` (run states, `runs.cancel_timeout`), `M1-64-gateway-connection.md` (`4013` on restart), `M7-09-status-page.md` (component state and the status read model).
- Code on the base branch: `services/server/internal/platform/httpserver`, `internal/platform/pg`, `internal/kernel/txn`, `internal/identity`, `internal/runs`, `internal/platform/jobs`, `internal/status`, `internal/wire`.

This ticket touches no table, no action and no event.

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
9. The `Retry-After` value and every other limit only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the value.
10. Specification first: a change to behaviour, API or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`, `actions.yaml`) is not edited by this ticket; the classification `effect` is read from the catalogue as it is. Everything this ticket adds is English.
11. No cost (D-W-125).

Product requirements implemented here:

12. Every action with `effect: write` and without an exception of ADR 0065 item 4 gives `503 maintenance` with `Retry-After`; no action with `effect: read` does (05 2.7, table test over the whole catalogue).
13. The layer writes no audit entry and no other row; the metric of the rejections carries the action ID only.
14. The exceptions for runs depend on the run state at request time (`running`), checked through the port of `runs`; `run.start` is blocked; runs in `queued` and `waiting_input` wait.
15. In the mode every transaction is `READ ONLY` except the exceptions; an unclassified write fails at PostgreSQL and becomes `503 maintenance`; the database is unchanged.
16. `auth.token.refresh` in the mode returns a new access token and the same refresh token; the rows of the session and the refresh family are byte-identical before and after.
17. In the `worker` only the consumers `push` and `webhooks`, the queue `delivery` and `runs.cancel_timeout` run; all other queues and periodic jobs do not start.
18. The status page reports the maintenance from the configuration (component state `maintenance` in the read model, without writing `status_components`).

## Scope

Server module (paths under `services/server/` unless they start with `docs/`; every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`):

- `internal/platform/httpserver`: the stage `StageMaintenance` after the rate limit and before `authorize`, with the exception predicate (Design note 1) and the problem response.
- `internal/platform/pg` and `internal/kernel/txn`: the read-only Unit of Work mode, the per-call exemption for the exception paths, the mapping of SQLSTATE `25006` to `maintenance`, dropping side writes.
- `internal/identity`: refresh without rotation and without writes in the mode; the emergency actions exempted.
- `internal/runs`: the query "is this agent's run in this thread running" through its port for the layer, exemptions of the run actions.
- `internal/platform/jobs` and `internal/wire`: queue and job filter in the mode.
- `internal/status`: the maintenance state in the status read model from the configuration (M7-09 builds the read model).
- Configuration: `server.maintenance_mode` becomes active; new key `server.maintenance_retry_after` (16 14 and the reference).
- Metric: `bothy.maintenance.rejected` (or the name the build records in `signals.md` and the registry of M1-68, `make signal-check`).
- Docs: `docs/spec/16-operations.md` 11 and 14, `docs/spec/02-architecture.md` (request path row 9 as built), `docs/operations/cloud-migration.md` and `upgrade.md` (the mode in the steps, repeating emergency revocations on the target), `docs/observability/signals.md`, `docs/operations/configuration.md` (new keys such as `server.maintenance_retry_after`), READMEs, `services/server/CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-08-<n>`.

## Out of scope

- The client notice and the outbox resend on `503 maintenance` (client behaviour of 15; already built where M4-15 handles `Retry-After`; a gap goes into the report).
- The `moved_to` discovery of 16 11 (planned, not built).
- Switching the mode without a restart (ADR 0065 item 1: start parameter).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-08-maintenance-mode`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(httpserver): ...`, `feat(identity): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). Tests bind only `127.0.0.1` and port 0; PostgreSQL only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Exception predicate** (ADR 0065 item 4; Ruling for docs/DECISIONS.md). One table in code, generated or checked against `actions.yaml`: the fixed exceptions (`auth.token.refresh`, `auth.step_up`, `session.revoke`, `device.revoke`, `token.revoke`, `user.disable`, `run.cancel`, `gateway.connect`) and the conditional ones for an agent with a running run (`run.update`, `run.finish`, `agent.heartbeat`, `agent.events.consume`, `message.create` and `file.upload` only in the thread of that run). A test fails when an exception names an action that is not in the catalogue or whose `effect` is `read`.

2. **Second line** (ADR 0065 item 5). The exemption is explicit per call (`txn.WithWriteInMaintenance` or the merged equivalent), used only by the exception paths and the worker consumers; everything else gets `SET TRANSACTION READ ONLY`. A test handler with an unclassified write in the mode fails with `503` and leaves the database unchanged.

3. **Refresh without rotation** (08 7.4). In the mode, refresh validates the token and the DPoP proof of the family as usual, issues an access token with the normal lifetime and returns the presented refresh token unchanged; reuse detection still revokes the family (an exception that writes).

4. **Worker** (02 12.2). The job runtime starts only the allowed queue, consumers and periodic job in the mode; the others are not registered, so nothing writes. A restart without the mode resumes them.

5. **Rulings for `docs/DECISIONS.md`** (`D-M7-08-<n>`): the exception table (note 1), the exemption API (note 2), refresh behaviour (note 3), the new key `server.maintenance_retry_after`, the metric name.

## Tests (write them first)

Unit:

- `httpserver`: `TestMaintenance_TableOverCatalogue` (every action of `actions.yaml`: `write` without exception gives `503 maintenance` with `Retry-After`, `read` passes, exceptions pass; the order after the rate limit and before `authorize`).
- `TestExceptionsAreCatalogueWrites` (Design note 1).

Integration (`pgtest`, real modules through `internal/wire`, the mode on):

- `TestMaintenance_MessageRejected` (`503`, `Retry-After`, no row, no audit entry, metric counted).
- `TestMaintenance_RunCanFinish`: an agent with a running run updates, posts its answer in the run's thread, uploads a file there and finishes; the same agent in another thread gets `503`; `run.start` gets `503`.
- `TestMaintenance_RefreshWithoutRotation` (rows identical before and after) and `TestMaintenance_ReuseRevokesFamily`.
- `TestMaintenance_DeviceRevokeWorks`: with step-up, the device's gateway connection closes with `4012`.
- `TestMaintenance_SecondLine`: an unclassified test write fails with `503`, database unchanged.
- `TestMaintenance_WorkerOnlyDeliveries`: only `push`, `webhooks`, `delivery` and `runs.cancel_timeout` run.
- `TestMaintenance_StatusShowsMaintenance`: `getStatus` reports the maintenance without a write.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/... ./cmd/...`
- `make arch`, `make catalog-check`, `make config-doc-check`, `make signal-check`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] A message gives `503` with `Retry-After`; a running run can finish; no new run is created; the status page shows maintenance.
- [ ] The table test over the catalogue: every `write` without an exception gives `503 maintenance`, no `read` does.
- [ ] Refresh returns the same refresh token without a write; emergency revocation works; in the `worker` only deliveries and `runs.cancel_timeout` run.
- [ ] An unclassified write attempt fails on the read-only transaction with `503`.
- [ ] 16, 02, the runbooks, `signals.md`, READMEs, the CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of the touched modules updated in English.
- Rulings of Design note 5 recorded in `docs/DECISIONS.md` section 10.
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
4. The exception table as built.
5. What is not proven (a full relocation rehearsal of `cloud-migration.md` 3).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
