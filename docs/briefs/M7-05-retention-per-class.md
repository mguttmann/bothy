# M7-05: Retention per class

Status: ready | Depends on: M1-39, M1-40, M1-59, M2-15, M2-22, M2-26, M2-46, M3-08, M3-15, M3-22 | Milestone: M7

## Goal

After this ticket no data outlives its retention class (20 4.1, R-DP-01). The worker runs one periodic job per class, named `retention.<class>` as in the job catalogue of 02, for the classes `security` (`retention.security_after_end` after expiry, revocation or end), `content` (`retention.content`, default `none`: no automatic deletion; with a period, contents are removed as tombstones exactly like a deletion of 20 4.2), `transient` (`retention.webhook_deliveries` and the short-lived flow tables), `audit` (truncation through the role `bothy_maint` with an anchor entry at the end of `retention.audit`, and destruction of the IP keys of `audit_ip_keys` after `privacy.audit_ip_key_retention`, 20 4.5) and `cost` (`retention.cost`). Each owning module deletes only its own tables, through a participant port, never by SQL on another module's tables. `bothy config check` and the start log warn when `retention.content = none` and `retention.content_none_reason` is empty, and the start log names the effective value otherwise (DP-T13). A coverage test reads every `Retention:` line of 03 and fails when a table of a class with a job has no participant. Why: storage limitation (Art. 5(1)(e) GDPR) and the codetermination defaults of 20 9 hold only when the periods are enforced, tested with a controlled clock (DP-T05) and provable for the IP pseudonyms (DP-T06).

## Read first

- `docs/spec/20-privacy.md`: 2 (classes per table), 4.1 (all classes, mechanisms, the warning of `retention.content = none`), 4.2 (deleting a message: the content job uses the same path), 4.3, 4.5 (audit, IP keys), 10 (DP-T05, DP-T06, DP-T13), 11 (R-DP-01, R-DP-03), 13 (all `retention.*` and `privacy.audit_ip_key_*`).
- `docs/spec/03-data-model.md` (core contract, read only): the `Retention:` line of every table in section 4, `audit_events` (anchor entry, `bothy_maint`), `audit_ip_keys` (destruction rule and check), 6.5 (deleting a message).
- `docs/spec/02-architecture.md`: 12.3 (job catalogue rows `retention.<class>`, queue `maintenance`), 4.5 (ports per module); `docs/spec/16-operations.md` 7.1 (`bothy_maint`); `docs/spec/19-security.md` SEC-AUD-1 and SEC-AUD-2 (verification starts at the anchor).
- `docs/operations/configuration.md` (rows `retention.*`, all `planned (M7)` except `retention.stats_minutes`).
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125); section 10 (format of `D-M7-05-<n>` rows).
- Briefs (merged names win, D-W-100): `M1-14-background-jobs.md` (periodic jobs, queue `maintenance`), `M1-24-audit-chain.md` (item on `bothy_maint` and the IP key table; "Truncation with anchor ... M7"), `M1-50-audit-verify.md` (verification from the anchor), `M1-16-event-log.md` (class `event_log` is already enforced there), `M1-20-idempotency-keys.md` (`sync.idempotency_cleanup` already covers `idempotency_keys`), `M3-08-file-cleanup.md` (files and upload sessions), `M2-26-webhooks-outgoing.md` (`webhook_deliveries`), `M2-22-costs.md` ("The retention job `retention.cost` (M7-05)"), `M2-46-terminal-revocation-limits.md` (`terminal_sessions` via `retention.security`), `M6-13-stats-measure.md` (class `stats` has its own job), `M1-07-config-schema-loader.md` (`bothy config check`).
- Code on the base branch: `services/server/internal/kernel`, `internal/platform/jobs`, `internal/wire`, the modules owning the tables of the classes above.

This ticket touches no table definition, no action and no event.

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
9. Periods only through the named keys of 20 13; code comments and docs name the key, never the value.
10. Specification first: a change to behaviour or configuration updates `docs/spec/20-privacy.md`, `docs/spec/02-architecture.md` or `docs/operations/` in the same change. The core contract is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125).

Product requirements implemented here:

12. Only expired data is removed (DP-T05): each participant deletes rows whose end time (the column named per table in Design note 2) plus the period is before `clock.Now()`; nothing else.
13. `retention.content = none` deletes nothing; with a period, messages are deleted through the deletion use case of `messaging` (20 4.2: tombstone, redaction of event payloads, attachments through the purge of M3-08), never by a bare `DELETE`.
14. The audit log is truncated only through `bothy_maint` with an anchor entry; the chain still verifies from the anchor (`audit.verify`) after a trim; IP keys are destroyed by emptying `key_ciphertext` and setting `destroyed_at`, the row stays (03 `audit_ip_keys`), and an `ip_hmac` of a destroyed period can no longer be matched to any IP (DP-T06).
15. Jobs run in batches of a named size in short transactions on queue `maintenance` and report deleted rows per class as a metric without IDs.
16. The warning of 20 4.1 appears in `bothy config check` and in the start log only for `retention.content = none` with an empty reason (DP-T13).

## Scope

Server module (paths under `services/server/` unless they start with `docs/`; every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`):

- `internal/kernel/retention` (new kernel port, Design note 1): the participant interface, the class enumeration of 20 4.1, the registry; `architecture/modules.yaml` lists the port, 02 4.5 names it.
- Participants in the owning modules for their tables of the classes `security`, `content`, `transient`, `audit` and `cost` (the tables are listed in the coverage test; Design note 2), including `identity`, `instance` (retired signing keys), `agents`, `notify` (`push_registrations`), `terminal`, `messaging`, `chat`, `files`, `runs`, `apps` (`webhook_deliveries`), `billing`, `audit`, `search` (`file_text_extracts`) as their tables require.
- `internal/wire`: the jobs `retention.security`, `retention.content`, `retention.transient`, `retention.audit`, `retention.cost` on queue `maintenance` in the roles `worker` and `all`, each with an interval and batch size as named keys (`retention.job_interval`, `retention.batch_size`, added to 20 13 and the reference).
- Audit: grants for `bothy_maint` (truncation path of 03 `audit_events`, the anchor entry), the IP key destruction.
- Configuration: the keys of 20 13 that are `planned (M7)` become active; `bothy config check` and the start log for DP-T13.
- Docs: `docs/spec/20-privacy.md` 4.1 and 13 (new keys, mechanism notes), `docs/spec/02-architecture.md` 12.3 (job rows with intervals), READMEs of the touched modules (jobs table), `services/server/CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-05-<n>`.

## Out of scope

- Class `privacy_job` and the expiry of export packages (M7-06, which owns `gdpr_requests`); class `ops` (M7-09, which owns the status tables and `backup_runs`); class `import` (the import tickets of M8 own `import_sources` and `import_items`); class `stats` (M6-13); class `event_log` (M1-16) and `idempotency_keys` (M1-20), which already run; `dev_environments` (V2).
- Classes `config`, `account` and `personal_sync`, which have no time-based job (20 4.1).
- Account deletion and export (M7-06); the deletion journal (M7-07).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-05-retention-per-class`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(privacy): ...`, `feat(audit): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). Tests bind only `127.0.0.1` and port 0; PostgreSQL only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. Logs of this ticket carry no person, channel or message ID.
- Time only through `kernel/clock`; tests move a fake clock and call the jobs directly, they never sleep for an interval.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Participant port** (owner rule 2; Ruling for docs/DECISIONS.md). `kernel/retention` defines `Participant{Class() Class; Tables() []string; Purge(ctx, cutoff time.Time, batch int) (deleted int, more bool, err error)}`. A module registers its participants in `wire`; the job of a class calls each participant of the class until `more` is false or the job's time budget ends. `Tables()` exists for the coverage test only.

2. **End times per table** (20 4.1, 03). The class `security` counts from expiry, revocation or end (`expires_at`, `revoked_at`, `ended_at`, `closed_at`, `disabled_at` as each table has them; a row without any end time is not expired). Record the column per table in the README of the module and in the ruling. Tables of class `security` that a later state still needs (an active device, a valid key) never match because they have no end time.

3. **Coverage test** (class fix instead of instances). `TestEveryRetentionTableHasAParticipant` parses the `Retention:` lines of 03 section 4 and the class list of the jobs; every table of a class with a job built here must be in exactly one participant's `Tables()`; the allowed exceptions are an explicit list in the test with the owning later ticket (`gdpr_requests` M7-06, the four status tables and `backup_runs` M7-09, `import_sources` and `import_items` M8, `dev_environments` V2) and the tables an existing job of the job catalogue (02 12.3) already handles, each listed with that job (for example `event_log` with `sync.partition_maintenance` of M1-16, `idempotency_keys` with `sync.idempotency_cleanup` of M1-20, `auth_challenges`, `auth_login_flows` and `device_pairings` with `identity.auth_cleanup`, `stats_minutes` with `stats.retention` of M6-13, `file_upload_sessions` and `file_upload_parts` with `files.cleanup_pending` of M3-08). A listed job must exist in the job catalogue of 02, so the list cannot name a job that nobody runs. A new table of 03 with one of these classes and no participant fails the test.

4. **Audit trim** (03 `audit_events`, SEC-AUD-2). The job connects with `bothy_maint` only for the trim: it writes the anchor entry (the last removed `seq` and `hash`) through the audit port, then truncates the rows before it in the way 03 prescribes, in one transaction; `audit.verify` and the restore test of M7-03 start at the anchor. Checkpoints of class `audit` older than the anchor are removed with it.

5. **Rulings for `docs/DECISIONS.md`** (`D-M7-05-<n>`): the participant port (note 1), end time per table (note 2), the exception list of the coverage test (note 3), the audit trim with `bothy_maint` (note 4), the new keys `retention.job_interval` and `retention.batch_size`.

## Tests (write them first)

Unit:

- `kernel/retention`: `TestClassesEqual20` (class list equals 20 4.1).
- Per participant: `TestPurgeOnlyExpired` with a fake clock (rows just before, at and after the cutoff).

Integration (`pgtest`, real modules through `internal/wire`):

- `TestRetention_DPT05`: one scenario per class with a controlled clock; after the job only expired rows are gone (DP-T05).
- `TestRetention_ContentNoneDeletesNothing` and `TestRetention_ContentPeriodUsesDeletionPath` (tombstone, redacted event payloads, attachment purge job queued).
- `TestRetention_AuditTrimVerifies`: after the trim the anchor exists and `audit.verify` passes; a rewrite of a row after the anchor is still detected.
- `TestRetention_DPT06`: after key destruction an `ip_hmac` cannot be recomputed for any IP; the chain is valid; the key row stays with `destroyed_at`.
- `TestEveryRetentionTableHasAParticipant` (Design note 3).
- `TestConfigCheck_DPT13`: warning only for `none` with an empty reason; the start log names the effective value.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/... ./cmd/...`
- `make migrations-lint`, `make migrations-lock`, `make codegen-check`, `make arch`, `make register-check`, `make config-doc-check`, `make signal-check`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] DP-T05, DP-T06 and DP-T13 are green.
- [ ] One job per class `security`, `content`, `transient`, `audit`, `cost`; each module deletes only its own tables through its participant.
- [ ] The audit trim keeps the chain verifiable from the anchor; IP keys are destroyed as 03 requires.
- [ ] The coverage test fails for a table of a retention class without a participant and lists the later owners.
- [ ] 20, 02, READMEs, the configuration reference, the CHANGELOG and the rulings are updated.
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
4. The table of participants: class, table, end time column, owning module.
5. What is not proven.
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
