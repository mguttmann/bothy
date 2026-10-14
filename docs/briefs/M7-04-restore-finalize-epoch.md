# M7-04: `bothy restore finalize` and sync epoch (ADR 0059)

Status: ready | Depends on: M2-07, M2-26, M7-02 | Milestone: M7

## Goal

After this ticket a restore can no longer silently mix two timelines. `bothy restore finalize` sets, in one transaction, a new `instance.sync_epoch` (UUIDv7), all `agent_consumer_cursors.acked_seq`, `webhook_subscriptions.last_delivered_seq` and `event_consumer_cursors.last_seq` to the new `head_seq`, writes one audit entry, and stores the identity of the restored database (`pg_control_system().system_identifier` and `pg_control_checkpoint().timeline_id`) in `instance.pg_system_identifier` and `instance.pg_timeline_id` (03 6.7, R-DM-26, R-DM-50, ADR 0059 item 1). The epoch in `hello`, `ready`, `resumed`, the snapshot and `GET /api/v1/events` and its comparison have existed since M1-63 and M1-64; this ticket proves them end to end after a real restore: a client with a cursor from the old timeline gets `resync_required` with `epoch_changed`, even when `head_seq` has overtaken its cursor. Outgoing webhooks carry `sync_epoch` in their envelope (04 2.2, ADR 0059 item 4). A start after a restore without `restore finalize` is detected by the changed database identity and does not become ready (`/readyz` reports `restore_not_finalized`), and a relocation without a restore does not change the epoch. Why: without the epoch the new timeline overtakes old cursors after the first writes and clients skip events silently; without the start lock an operator who forgets the step gets exactly that.

## Read first

- `docs/spec/03-data-model.md` (core contract, read only): `instance` (4.1, including the columns `pg_system_identifier` and `pg_timeline_id` and R-DM-50), 6.7, `agent_consumer_cursors`, `webhook_subscriptions`, `event_consumer_cursors`, `audit_events`.
- `docs/spec/04-sync-protocol.md` (core contract, read only): 2.2 (envelope, field `sync_epoch` of webhook deliveries), 6.3, 7.3 to 7.5 (R-SY-04, R-SY-18).
- `docs/spec/16-operations.md`: 8.4, 9, 11 (relocation keeps the epoch), 14; `docs/spec/02-architecture.md` 3.3 (command `bothy restore finalize`), 11 (readiness), 15.1.
- `docs/adr/0059-sync-epoch-after-restore.md` (all, including "More Information": the start lock); `docs/operations/backup-restore.md` 5.2 to 5.3, `docs/operations/cloud-migration.md`, `docs/operations/disaster-recovery.md`.
- `docs/api/webhooks.md` (envelope), `docs/api/openapi.yaml` (`GatewayHello`, `listEvents` parameter `epoch`, webhook envelope schema).
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 with R-DM-50 (D-W-125); section 10 (format of `D-M7-04-<n>` rows).
- Briefs (merged names win, D-W-100): `M1-63-sync-catch-up.md` (`CatchUp.Check`, `ResyncReason`), `M1-64-gateway-connection.md` (epoch in `hello`, `resume`), `M1-65-gateway-delivery.md`, `M1-11-lifecycle-health.md` (readiness reasons), `M1-12-postgres-migrations.md` (`migrate up` creates the `instance` row), `M2-01-worker-event-consumers.md`, `M2-07-agent-consumers.md`, `M2-26-webhooks-outgoing.md`, `M7-01-postgres-image.md` and `M7-02-backup-service.md` (PITR in the image test).
- Code on the base branch: `services/server/internal/instance`, `internal/sync`, `internal/gateway`, `internal/agents`, `internal/apps` (webhooks), `cmd/bothy`, `internal/platform/health`.

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
9. Limits and switches only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the value.
10. Specification first: a change to behaviour, API, configuration or a readiness reason updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`, `actions.yaml`) is not edited by this ticket; the two `instance` columns are built exactly as 03 has them. Everything this ticket adds is English.
11. No cost (D-W-125): local containers only.

Product requirements implemented here:

12. `restore finalize` is atomic: epoch, the three cursor kinds, the audit entry and the stored database identity change in one transaction or not at all (ADR 0059 confirmation).
13. The epoch changes only through `restore finalize`; `migrate up` sets the stored identity only when it is empty (first installation) and never changes `sync_epoch` (03 4.1). A relocation without data loss (16 11) also ends recovery on a new timeline; `bothy restore finalize --relocation <manifest>` then stores the new identity and keeps epoch and cursors, but only when `head_seq`, the audit head and `sync_epoch` of the restored database equal those of the given manifest of the final backup (Design note 3).
14. A mismatch between the stored and the current database identity keeps every server role not ready with the reason `restore_not_finalized`, unless `database.restore_guard` is `off` (Design note 3); `restore finalize` itself and `migrate` still run.
15. Outgoing webhook deliveries carry the current `sync_epoch`; after a restore the first delivery carries the new one.
16. `restore finalize` needs no running `api` or `worker` and refuses to run while the database reports another server process connected with the role `bothy_app` (so a running stack cannot see half a change).

## Scope

Server module (paths under `services/server/` unless they start with `docs/`; every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`):

- Migration: the columns `instance.pg_system_identifier` and `instance.pg_timeline_id` exactly as 03 4.1 (R-DM-50); `migrate up` fills them when empty.
- `internal/instance`: use case `FinalizeRestore` (new epoch, stored identity, audit entry through `kernel/auditing`), the identity query, the readiness check `restore_not_finalized`; it calls ports of `agents`, `apps` and `sync` to move their cursors in the same unit of work (Design note 2); 02 4.5 and `architecture/modules.yaml` get the dependencies if they are missing.
- `internal/agents`, `internal/apps`, `internal/sync`: one port method each that sets their cursors to a given `head_seq` inside the caller's transaction.
- `cmd/bothy`: subcommand `restore finalize [--relocation <manifest file>]` (exit codes as 02 3.3: `0` done, `1` error or refused, `2` usage) with its help text; the row in 02 3.3 names the option.
- Configuration: key `database.restore_guard` (`on` or `off`, default `on`) in 16 14 and the configuration reference.
- Webhook envelope: every delivery fills `sync_epoch` from the current epoch (M2-26 built the field; this ticket proves it after a restore and corrects the delivery where it does not); `docs/api/webhooks.md` and the envelope schema stay as they are.
- Docs: `docs/spec/16-operations.md` 8.4 (the start lock and the key), `docs/spec/02-architecture.md` 11 (readiness reason), `docs/operations/backup-restore.md` 5.2 step 6, `docs/operations/disaster-recovery.md`, `docs/operations/cloud-migration.md` (relocation keeps the epoch; managed failover and the key), `docs/adr/0059-*.md` "More Information" (the open point closed, with a link to the ruling), READMEs of the touched modules, `services/server/CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-04-<n>`.

## Out of scope

- Applying the deletion journal on restore (M7-07); the restore test (M7-03).
- The `moved_to` discovery of 16 11 (planned, question to team B; not built).
- Client behaviour on `epoch_changed` beyond what M4-14 already built.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-04-restore-finalize-epoch`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(instance): ...`, `feat(sync): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). Tests bind only `127.0.0.1` and port 0; PostgreSQL only as a throwaway container on a random port through `pgtest` or the image test (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Database identity** (ADR 0059 "More Information"; Ruling for docs/DECISIONS.md). Read `system_identifier` from `pg_control_system()` and `timeline_id` from `pg_control_checkpoint()`. Verify against the PostgreSQL 18 documentation which roles may execute them; if `bothy_app` may not, `migrate up --bootstrap` grants `EXECUTE` on exactly these two functions to `bothy_app` and the grant test of 16 7.1 rule 3 lists it. A PITR keeps the system identifier and raises the timeline; a logical restore into a fresh cluster changes the system identifier; both are mismatches.

2. **One transaction** (03 6.7 item 1). `FinalizeRestore` opens one unit of work, locks the `instance` row, reads `head_seq`, calls the cursor ports of `agents`, `apps` and `sync` with that value, writes the epoch and identity, appends the audit entry (system actor, action named in the ruling, no content). Any error rolls back everything; a test injects a failure in the last port and checks that nothing changed.

3. **Start lock and managed failover** (Ruling for docs/DECISIONS.md). On start, the readiness component compares the stored identity with the current one; a mismatch keeps `/readyz` at `restore_not_finalized` and logs once with the runbook step. A managed service with automatic failover raises the timeline without a restore. Only where the provider documents that a failover loses no committed transaction (synchronous replication) may the operator set `database.restore_guard = off` and rely on R-SY-04; a failover that can lose commits is handled like a restore (not ready until `restore finalize` starts a new epoch), because new writes could reuse sequence numbers a client already holds; 16 and `cloud-migration.md` say so. An empty stored identity (instance older than this ticket) is filled at the next `migrate up` and never blocks. A relocation (16 11 step 4) restores to the latest state, which also starts a new timeline: the runbook calls `restore finalize --relocation` with the manifest of the final backup; the command compares `head_seq`, the audit head (`seq`, `hash`) and `sync_epoch` of the restored database with the manifest and, only when all are equal, stores the new identity without touching epoch and cursors (audit entry with its own action name); any difference aborts with exit `1` and names the differing field, and the operator then runs `restore finalize` without the option.

4. **Webhooks** (04 2.2, ADR 0059 item 4). The epoch is read with `head_seq` per delivery batch; after `restore finalize` the webhook cursor is at the new head, so receivers see only the new timeline and its new epoch.

5. **Rulings for `docs/DECISIONS.md`** (`D-M7-04-<n>`): identity source and grants (note 1), the audit action name, the start lock, `--relocation` and `database.restore_guard` (note 3), the refusal while `bothy_app` sessions are connected.

## Tests (write them first)

Unit:

- `instance/domain`: `TestIdentityMismatch` (same, other timeline, other system identifier, empty stored value).

Integration (`pgtest`):

- `TestFinalizeRestore_Atomic`: after the command the epoch is new, all three cursor kinds equal `head_seq`, one audit entry exists; with an injected failure nothing changed.
- `TestFinalizeRestore_RefusesWithConnectedApp`: an open `bothy_app` session makes it exit `1` without a change.
- `TestEpochChanged_AfterOvertake`: a gateway client and a REST catch-up with a cursor of the old epoch get `resync_required` with `epoch_changed` after new writes made `head_seq` larger than the cursor.
- `TestRelocationKeepsEpoch`: after a relocation `restore finalize --relocation <manifest>` with a matching manifest keeps the epoch and the cursors, the server becomes ready and a client continues with its cursor without a resync; with a manifest whose `head_seq` differs the command exits `1` and changes nothing.
- `TestWebhookCarriesEpoch`: a delivery after `restore finalize` carries the new epoch.

Image test (extend `postgres-image-test`):

- `TestStartAfterPITRWithoutFinalize`: a PITR with the image of M7-01, then `bothy serve` against it stays not ready with `restore_not_finalized`; after `bothy restore finalize` it becomes ready.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/instance/... ./internal/sync/... ./internal/gateway/... ./internal/agents/... ./internal/apps/... ./cmd/... ./internal/wire/...`
- `make postgres-image postgres-image-test` on the build host
- `make migrations-lint`, `make migrations-lock`, `make codegen-check`, `make arch`, `make register-check`, `make config-doc-check`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] A client with a cursor of the old timeline gets `resync_required` with `epoch_changed`, even when `head_seq` has overtaken its cursor.
- [ ] `restore finalize` moves epoch, cursors, audit and stored identity atomically; a relocation without a restore keeps the epoch.
- [ ] Outgoing webhooks carry `sync_epoch`.
- [ ] A start after a restore without `restore finalize` does not become ready; with `database.restore_guard = off` it does.
- [ ] 16, 02, the runbooks, ADR 0059, the READMEs, the CHANGELOG and the rulings are updated.
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
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check` and of `make postgres-image-test`.
3. Review and red team verdicts and the findings fixed.
4. Which roles may execute the identity functions in PostgreSQL 18 and the source.
5. What is not proven (a managed service and its failover).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
