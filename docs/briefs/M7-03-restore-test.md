# M7-03: Automatic restore test

Status: ready | Depends on: M7-02 | Milestone: M7

## Goal

After this ticket a backup counts only once it has been restored (R-OP-10): per `backup.restore_test.schedule`, `bothy-backupd` restores the latest backup into the volume `backup-scratch`, starts a PostgreSQL there on a Unix socket only (no TCP), runs the seven checks of 16 8.3 each with its own result, stops the database and empties the scratch directory. The checks: `pgbackrest verify` and the checksums of the restore; schema version and row counts against the manifest; the event log without gaps per partition (`count(*) = max(seq) - min(seq) + 1`) and `sequence_counters` at least every assigned number; the audit chain from the anchor to the end with the same verification as `audit.verify`; a sample of `backup.restore_test.sample_threads` threads with gapless `thread_seq`; a sample of `backup.restore_test.sample_files` files fetched from the object backup with SHA-256 against `files.sha256`; the duration against `dr.rto_target`. The result goes into `backup_runs` (kind `restore_test`, `checks` per `restore_test_checks.v1`) and into the metrics `bothy.backup.restore_test.*`. The test never writes to the production database. A manipulated copy (a gap in the event log, a wrong audit hash, a changed file) fails, each with the right check; a clean copy passes. Why: exit criterion 1 of M7 and threat T-BK-01 require proof that a backup is complete and untampered before it is needed.

## Read first

- `docs/spec/16-operations.md`: 8.1, 8.3 (all seven checks), 9 (`dr.rpo_target`, `dr.rto_target`), 14 (`backup.restore_test.*`, `dr.*`), 15 (R-OP-10).
- `docs/spec/03-data-model.md` (core contract, read only): `backup_runs` (`checks`, `restore_test_checks.v1`), `event_log` and its partitions, `sequence_counters`, `audit_events` (chain fields, anchor entry), `threads` and `messages` (`thread_seq`), `files` (`sha256`).
- `docs/spec/19-security.md`: SEC-AUD-1 and SEC-AUD-2 (what `audit.verify` checks), 12 (backups); `docs/security/threat-model.md` (T-BK-01).
- `docs/adr/0058-backup-with-pgbackrest-object-backup-and-restore-test.md` (item 6, "Confirmation"); `docs/operations/backup-restore.md` 4 (procedure steps 1 to 7) and 5.
- `docs/observability/signals.md` (rows `bothy.backup.restore_test.*`), `docs/observability/alerting.md` (`BothyRestoreTestStale` and the failure rule: one failure `ticket`, two in a row or an audit chain error `page`).
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125); section 10 (format of `D-M7-03-<n>` rows).
- Briefs (merged names win, D-W-100): `M7-02-backup-service.md` (use cases in `status`, executor port, manifest), `M7-01-postgres-image.md` (image test suite), `M1-50-audit-verify.md` (the chain verification and its result codes), `M1-16-event-log.md` (partitions), `M1-56-messaging-send.md` (`thread_seq`).
- Code on the base branch: `services/server/internal/status`, `services/server/internal/audit`, `services/server/cmd/bothy-backupd`, `services/server/architecture/modules.yaml`.

This ticket touches no table, no action and no event; it adds a check schema in `status/domain`.

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
9. Sample sizes, schedules and targets only through the named keys `backup.restore_test.*` and `dr.rto_target`; code comments and docs name the key, never the value.
10. Specification first: a change to behaviour or configuration updates `docs/spec/16-operations.md` and `docs/operations/backup-restore.md` in the same change. The core contract is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125): local targets and throwaway containers only.

Product requirements implemented here:

12. The test reads only the backup and the object backup and writes only to `backup-scratch`, `backup_runs` and the metrics; it opens no connection to the production database except the `backup_runs` writes through the role of M7-02.
13. Each of the seven checks has its own result (`passed`, `failed`, `skipped` with a reason) and duration in `checks`; a failed check never stops the remaining checks except where a later check needs the restored database (then `skipped` with the reason `restore_failed`).
14. The audit check is the verification of `audit.verify`, reused through the port of `audit`, not a second implementation (owner rule 2).
15. Samples are drawn at random per run from all threads and from all files not deleted (`deleted_at IS NULL`); the sample sizes are the configured values or all rows when fewer exist.
16. The scratch PostgreSQL listens only on a Unix socket in the scratch directory (`listen_addresses = ''`) and is stopped and removed even when a check panics.

## Scope

Server module (paths under `services/server/` unless they start with `docs/`):

- `internal/status/domain`: the schema `restore_test_checks.v1` with the seven check names and its validation.
- `internal/status/app`: the use case `RestoreTest` (steps of `backup-restore.md` 4, check orchestration, total duration against `dr.rto_target` as check 7, the `backup_runs` row, metrics).
- `internal/status/adapters`: the scratch database adapter (start and stop PostgreSQL in `backup-scratch`, queries over the Unix socket), the restic sample fetch for files.
- `internal/audit`: a port function that verifies a chain over rows supplied by the caller (Design note 2); `architecture/modules.yaml` and 02 4.5 add `audit` to the dependencies of `status` with the reason.
- `cmd/bothy-backupd`: the schedule `backup.restore_test.schedule` in the scheduler of M7-02 and the subcommand `restore-test` (run now, exit `0` when all checks passed, `1` otherwise).
- Tests (below), including the manipulated copies in the image test suite of `postgres-image-test`.
- Docs: `docs/spec/16-operations.md` 8.3 where the build sharpens it, `docs/operations/backup-restore.md` 4, `services/server/internal/status/README.md`, `services/server/CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-03-<n>`.

## Out of scope

- Restoring production (`bothy-backupd restore`, PITR runbook steps) beyond the scratch area; `bothy restore finalize` and the epoch (M7-04); the deletion journal (M7-07).
- The restore test of the managed-database path (`backup-restore.md` 6 item 2).
- Alert routing (M7-20).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-03-restore-test`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(status): ...`, `feat(audit): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). Tests bind only `127.0.0.1` and port 0; containers only as throwaway containers on random ports (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets.
- Time only through `kernel/clock`; the duration check in tests uses a fake clock or a tiny `dr.rto_target` set in the test configuration, never a sleep.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Order of checks** (16 8.3). 1 restore with `pgbackrest restore --type=immediate` into the scratch directory and `pgbackrest verify`; start the scratch database; 2 schema version and row counts against the manifest of the restored backup (the manifest of the same `pgbackrest.label`); 3 event log per partition and `sequence_counters`; 4 audit chain; 5 thread sample; 6 file sample; 7 total duration. A failure of step 1 sets checks 2 to 6 to `skipped` with `restore_failed` and the run to `failed`.

2. **Audit verification through the port** (owner rule 2; Ruling for docs/DECISIONS.md). `audit` exposes a function that takes the instance's verification keys and an iterator over `audit_events` and `audit_checkpoints` rows in `seq` order and returns the first broken `seq` and the reason code, the same function `audit.chain_verify` uses. The status adapter reads the rows from the scratch database and passes them; no SQL on audit tables from `status` against production.

3. **Manipulated copies** (ADR 0058 confirmation). In the image test: take a backup of a seeded database, then for each case restore it, manipulate the scratch copy before the checks through a test hook of the adapter (delete one `event_log` row in the middle of a partition; change one `audit_events.hash`; replace the bytes of one object in the restic restore target), and assert exactly the expected check fails with its reason while the others pass. A clean copy passes all seven. Without the test hook in production builds (build tag or test-only constructor).

4. **Result and alerting** (`alerting.md`). `backup_runs.state = failed` when any check failed; the metric `bothy.backup.restore_test.failures` counts per failed check with `bothy.reason` set to the check name; `bothy.backup.restore_test.last_success.timestamp` only on success. The alert rules exist; this ticket only produces their inputs.

5. **Rulings for `docs/DECISIONS.md`** (`D-M7-03-<n>`): check order and skipping (note 1), the audit port (note 2), the sampling method, the test hook (note 3).

## Tests (write them first)

Unit:

- `status/domain`: `TestRestoreChecksSchema` (seven names, results, durations).
- `status/app`: `TestRestoreTestAllPass`, `TestRestoreFailureSkipsDependentChecks`, `TestDurationOverRTOFails` (fake clock), `TestSampleSizes` (fewer rows than the sample size takes all).
- `audit`: `TestVerifyRowsMatchesChainVerify` (the port function and the job agree on a valid chain and on one broken row).

Integration:

- `status/adapters` with `pgtest` as the source of a seeded database dump: event log gap and `sequence_counters` check, thread sample check.
- Image test in `postgres-image-test`: the clean copy passes, each manipulated copy fails with exactly its check (Design note 3); the scratch database has no TCP listener (`ss` or `/proc/net/tcp` shows none inside the container); the scratch directory is empty afterwards; `backup_runs` has the `restore_test` row with `checks`.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/status/... ./internal/audit/... ./cmd/...`
- `make postgres-image postgres-image-test` on the build host
- `make arch`, `make register-check`, `make signal-check`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] All seven checks of 16 8.3 run per schedule and by hand, each with its own result in `backup_runs.checks`.
- [ ] A gap in the event log, a wrong audit hash and a changed file each fail with the right check; a clean copy passes; the duration is measured against `dr.rto_target`.
- [ ] The test never touches the production database beyond `backup_runs`; the scratch database listens on a Unix socket only and is removed afterwards.
- [ ] The audit check reuses the verification of `audit`; 02 4.5 and `modules.yaml` name the new dependency.
- [ ] Runbook, 16, README, CHANGELOG and rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- README of `status` updated in English.
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
4. The measured duration of the restore test on the seeded data and the share of each check.
5. What is not proven (a restore of a production-sized database against `dr.rto_target`).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
