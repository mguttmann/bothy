# M7-07: Deletion journal and restore

Status: ready | Depends on: M7-04, M7-06 | Milestone: M7

## Goal

After this ticket a restore can no longer bring back deleted data (R-DP-04, SEC-BAK-3, T-BK-02). Every deletion of a message, a file or an account writes a journal entry `{resource_kind, resource_id, deleted_at, reason}` without content (20 4.6). The journal lives outside the database: a consumer of the event log in the `worker` writes the entries continuously into the object storage under their own prefix, where the object backup of M7-02 copies them off-site; entries are kept for `privacy.deletion_journal_retention`, which must be at least as long as the longest backup period, or the server does not start (DP-T08). `bothy restore finalize` applies the journal before the instance accepts requests again: every entry whose resource exists again in the restored state is deleted again with the same use case (message deletion of 20 4.2, file deletion of 20 4.3, the erase steps of 20 4.4 with the recorded content mode), and the permission version rises so that all caches rebuild (SEC-BAK-3). The manifest of every backup run names the journal position, and the runbook's step for deletion requests completed after the target time is covered by the same journal (`backup-restore.md` 7 item 2). SEC-T15, DP-T07 and DP-T08 are green. Why: a restore that undoes a GDPR erasure or brings back a deleted message is a privacy incident; the journal is the only record that survives the restore.

## Read first

- `docs/spec/20-privacy.md`: 4.2, 4.3, 4.4, 4.6 (all of it), 10 (DP-T07, DP-T08), 11 (R-DP-04), 13 (`privacy.deletion_journal_retention`).
- `docs/spec/19-security.md`: SEC-BAK-3, SEC-T15; `docs/security/threat-model.md` (T-BK-02).
- `docs/spec/03-data-model.md` (core contract, read only): 6.3 (permission version), 6.5 (deleting a message), 6.7, `event_consumer_cursors`; `docs/spec/04-sync-protocol.md` 7.6 (consumers in the worker); `docs/spec/actions.yaml` events `message.deleted`, `file.updated` (redacted form after a deletion, R-DM-45), `user.updated`.
- `docs/spec/16-operations.md` 8.1 (backup periods), 8.2, 8.4; `docs/operations/backup-restore.md` 3, 5.2 (step 6), 7 (item 2); `docs/spec/12-files.md` 4.4 (object key schema).
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125); section 10 (format of `D-M7-07-<n>` rows).
- Briefs (merged names win, D-W-100): `M7-04-restore-finalize-epoch.md` (`FinalizeRestore`), `M7-06-privacy-requests.md` (erase steps, participants), `M7-02-backup-service.md` (manifest, object backup), `M2-01-worker-event-consumers.md` (consumer cursor pattern), `M3-01-object-storage-fs.md` and `M3-02-object-storage-s3.md` (storage port), `M3-08-file-cleanup.md`.
- Code on the base branch: `services/server/internal/privacy`, `internal/instance` (`FinalizeRestore`), `internal/files` (storage port), `internal/messaging`, `internal/authz` (permission version), `cmd/bothy`.

This ticket adds no action and no event. It adds the migrations of the table `deletion_journal_state` (03 4.12, R-DM-52) and of the check of `event_consumer_cursors.name` with `privacy.journal`, both as 03 already defines them (R-DM-27, D-W-125).

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
9. Periods only through named keys; code comments and docs name the key, never the value.
10. Specification first: a change to behaviour or configuration updates `docs/spec/20-privacy.md`, `docs/spec/16-operations.md` and the runbooks in the same change. The core contract is not edited by this ticket; the migrations follow 03 as it stands. Everything this ticket adds is English.
11. No cost (D-W-125).

Product requirements implemented here:

12. A journal entry carries only `resource_kind` (`message`, `file`, `account`), the resource ID, `deleted_at`, `reason` (a code, for account deletions with the content mode) and the `seq` of the event it came from; never content, names or emails.
13. Entries are written after commit by the worker consumer `privacy.journal` from the durable events, in `seq` order, as immutable objects (never overwritten); the consumer cursor advances only after the object is stored. A retry that finds an object with the same key and the same content counts as stored, so a crash between storing and moving the cursor does not stop the consumer.
14. `bothy restore finalize` applies every journal entry whose resource exists again, before it sets the new epoch, in the same command; application is idempotent, uses the normal deletion use cases (so tombstones, redaction and purges follow), and raises the permission version once (SEC-BAK-3).
15. The start of `api` and `worker` and `bothy config check` fail when `privacy.deletion_journal_retention` is shorter than the longest of `backup.logical.retention`, `backup.objects.retention` and the pgBackRest retention expressed as a period (DP-T08; the rule for converting full backup counts to a period is a ruling).
16. Journal objects older than `privacy.deletion_journal_retention` are removed by the job `retention.privacy_job` of M7-06 or a job of the same class, from `deletion_journal_state.low_number` upward, which then moves `low_number`; it removes an object only when its number is also below the `deletion_journal.next_number` of the manifest of every backup that is still kept (03 4.12), so a restore of any kept backup finds its first object; nothing else deletes them.

## Scope

Server module (paths under `services/server/` unless they start with `docs/`; every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`):

- `internal/privacy`: `domain` (entry, object key `deletion-journal/<number zero-padded>` with a dense number from `deletion_journal_state`, Design note 2, the retention comparison), `app` (consumer `privacy.journal`, `ApplyJournal` for restore), `adapters` (storage of entries through the object storage port of `files`, reading them back in order).
- `internal/instance`: `FinalizeRestore` calls `privacy.ApplyJournal` before the epoch step; on an error nothing of the epoch step runs and the command exits `1`.
- `internal/authz` (through its port): one increase of the permission version after the journal is applied.
- `cmd/bothy`: `restore finalize` prints the counts of applied entries per kind (no IDs); `bothy config check` and the start check of DP-T08.
- `status` (manifest of M7-02): the reference `deletion_journal.next_number` (the number after the newest stored object) next to the references `pgbackrest.label` and `restic.snapshot_id` of `backup-restore.md` 3; 03 names the required content fields of `backup_manifest.v1` and stays unchanged.
- `migrations`: the table `deletion_journal_state` with its one row and the cursor check with `privacy.journal` (03 4.12, 4.10), with `make migrations-lint` and `make migrations-lock` green.
- Configuration: `privacy.deletion_journal_retention` becomes active.
- Docs: `docs/spec/20-privacy.md` 4.6 (where and how), `docs/spec/16-operations.md` 8.2 and 8.4, `docs/operations/backup-restore.md` 3, 5.2 and 7, `docs/operations/disaster-recovery.md`, `docs/security/threat-model.md` (T-BK-02 test reference), READMEs, `services/server/CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-07-<n>`.

## Out of scope

- Ordinary deletions after the target time that the instance operator wants to keep deleted beyond messages, files and accounts (20 lists exactly these three kinds).
- Data outside the server (20 4.7).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-07-deletion-journal`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(privacy): ...`, `feat(instance): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). Tests bind only `127.0.0.1` and port 0; PostgreSQL and SeaweedFS only as throwaway containers on random ports (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. Logs carry no resource ID of a deleted message or account.
- Time only through `kernel/clock`.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Source of entries** (20 4.6; Ruling for docs/DECISIONS.md). The consumer reads `message.deleted`, `file.updated` whose state is deleted, and `user.updated` whose status is `deleted`; the account entry's `reason` carries the content mode the erase job used (read from the request through the `privacy` tables, which the consumer owns). The cleanup of finished requests (`retention.privacy_job`, M7-06) keeps a request until the `privacy.journal` cursor has passed the `user.updated` event of its account deletion, so a lagging consumer still finds the mode; a test delays the consumer while the cleanup runs. Writing from the log after commit means a crash between commit and storage loses nothing: the cursor did not move. The window between commit and storage is bounded by the consumer lag and named in the runbook.

2. **Storage** (12 4.1, 4.4; R-DM-52; Ruling). Objects under a prefix that the file key schema never produces, with keys of lowercase letters, digits, `/` and `-` only (`domain.ValidateKey` of `files`). The object storage port has no listing and no conditional write (12 4.1, core S3), so entries carry dense numbers from `deletion_journal_state`: in its round the consumer writes the object `deletion-journal/<next_number zero-padded>` after a `Head` that finds no object (where one exists, a `Get` compares the content: the same content counts as stored, a different one is an error), then moves `next_number` and its cursor in one transaction; the row lock of `event_consumer_cursors` keeps it to one writer. `ApplyJournal` reads from the restored `next_number` (every lower number is a deletion already contained in the restored state, and DP-T08 keeps that range) up to the first missing number and, in the transaction that sets the new sync epoch (M7-04), stores the number after the last object found, so the consumer never writes an existing key after a restore. The object backup of M7-02 copies the objects like files.

3. **Apply on restore** (SEC-BAK-3). For each entry in number order (sequence numbers can repeat across timelines after a restore): message exists and is not deleted: run the deletion use case with the system actor; file: the same with the file deletion; account: if the principal is not `deleted`, run the erase steps of M7-06 with the recorded mode. Entries whose resource does not exist or is already deleted are skipped. The command reports counts only.

4. **Retention comparison** (DP-T08; Ruling). The pgBackRest retention is a count of full backups; convert it to a period with the full backup schedule (count times the interval of `backup.pg.full_schedule`, rounded up to whole days) and compare the longest of the three periods.

5. **Rulings for `docs/DECISIONS.md`** (`D-M7-07-<n>`): event sources (note 1), object prefix, dense numbering and the write check (note 2), apply order (note 3), period conversion (note 4), the manifest reference.

## Tests (write them first)

Unit:

- `privacy/domain`: `TestEntryHasNoContent` (the JSON of an entry has exactly the five fields), `TestRetentionComparison` (DP-T08 cases with each of the three backup periods the longest).

Integration (`pgtest` and the storage test containers):

- `TestJournalFromEvents`: deleting a message, a file and an account writes one entry each, in `seq` order; a storage failure leaves the cursor and retries; an entry that was stored before a crash and before the cursor moved is found again and counts as stored; the migration accepts the cursor row `privacy.journal`.
- `TestSECT15_RestoreKeepsDeleted` (SEC-T15, DP-T07): seed, back up the database state (a `pg_dump` snapshot is enough here), delete a message, a file and an account (both content modes in two runs), restore the snapshot, run `restore finalize`: the message, the file and the account are deleted again, the permission version rose, and the epoch is new.
- `TestApplyIsIdempotent`: running the apply twice changes nothing the second time.
- `TestNumberAfterRestore`: after `restore finalize` the next entry gets the number after the last object found, and no existing object is written again; after the retention removed the oldest objects, `ApplyJournal` still applies every entry from the restored `next_number` on, and the retention keeps every object at or above the `next_number` of the oldest kept backup even when it is older than the retention.
- `TestDPT08_StartAbort`: a too short `privacy.deletion_journal_retention` aborts the start and fails `bothy config check`.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/privacy/... ./internal/instance/... ./internal/messaging/... ./internal/files/... ./cmd/... ./internal/wire/...`
- `make migrations-lint`, `make migrations-lock`, `make arch`, `make register-check`, `make config-doc-check`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] SEC-T15, DP-T07 and DP-T08 are green.
- [ ] Every deletion of a message, file or account writes a content-free journal entry outside the database, after commit and in order.
- [ ] `restore finalize` applies the journal before the epoch step and raises the permission version.
- [ ] 20, 16, the runbooks, the threat model, READMEs, the CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `privacy` and `instance` updated in English.
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
4. The bound of the window between commit and journal storage as measured in the test.
5. What is not proven (a full PITR with the journal on a production-sized instance).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
