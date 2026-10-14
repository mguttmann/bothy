# M7-06: Access, export, account deletion and rectification

Status: ready | Depends on: M7-05 | Milestone: M7

## Goal

After this ticket the privacy requests of 20 5 work end to end. A person files an export, deletion or rectification request for themselves (`createGdprRequest`, step-up), or the owner files it for them; the export of one's own data is approved by the system unless `privacy.export_requires_approval` is set, a deletion always needs the owner's decision (`decideGdprRequest`); the jobs `privacy.export` and `privacy.erase` run the approved request with one participant per module (`kernel/personaldata.Participant`), so every module exports and erases only its own tables. The export package is the ZIP of 20 5.2 (manifest with SHA-256 per file, only own data, no foreign messages, no secrets, no hashes), stored as a file with `purpose = 'gdpr_export'`, downloadable with step-up through `downloadGdprExport` for `privacy.export_ttl` and deleted after it (DP-T03, DP-T04). An account deletion runs the seven steps of 20 4.4 in order, with `privacy.account_deletion_content` deciding between `keep_pseudonymized` and `delete`; afterwards no clear data stays in `principals` and `users`, and the audit chain still verifies (DP-T02). A rectification request documents the owner's correction (20 5.3). The statutory deadline `due_at` (receipt plus one calendar month) is set on creation; before it expires, the owners get the personal durable event `gdpr.request_due` (R-AZ-35), which the consumer `push` of M3-19 maps to a hint of kind `admin` (13 2, D-W-108). The job `retention.privacy_job` removes old requests per `retention.privacy_job`. Why: rights of access, portability and erasure (Art. 15, 17, 20 GDPR) are obligations of the instance operator; without these flows they cannot be met.

## Read first

- `docs/spec/20-privacy.md`: 2 (personal data per table), 3, 4.4 (all seven steps), 4.5 (audit and pseudonymization), 5.1 to 5.3, 10 (DP-T02, DP-T03, DP-T04), 11, 13 (`privacy.*`, `retention.privacy_job`).
- `docs/spec/03-data-model.md` (core contract, read only): `gdpr_requests` (states, partial unique index, `(state, due_at)`), `principals` and `users` (pseudonymization fields), `files` (`purpose = 'gdpr_export'`), principle 7.
- `docs/spec/05-actions.md` and `docs/spec/actions.yaml` (core contract, read only): `gdpr.request.create`, `gdpr.request.read`, `gdpr.request.decide`, `gdpr.export.download`, event `gdpr.request_updated`, the personal event `gdpr.request_due` (R-AZ-35), rule on `file` with `purpose = 'gdpr_export'`.
- `docs/spec/13-push-notifications.md` 2 (kind `admin`: content only kind and target ID), 12; `docs/spec/08-identity.md` 11 (`user.disable`), step-up rules.
- `docs/spec/06-api.md` (privacy operations, `410 gone`), `docs/api/openapi.yaml` (`listGdprRequests`, `createGdprRequest`, `getGdprRequest`, `decideGdprRequest`, `downloadGdprExport`, `GdprRequestDueEvent`), `docs/api/asyncapi.yaml` (`event.gdpr.request_due`).
- `docs/spec/02-architecture.md` 12.3 (jobs `privacy.export`, `privacy.erase`), 4.5 (module `privacy`).
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 with R-AZ-35 (D-W-125), D-W-108 (the reminder through the consumer `push`); section 10 (format of `D-M7-06-<n>` rows).
- Briefs (merged names win, D-W-100): `M7-05-retention-per-class.md` (participant pattern, `kernel/retention`), `M3-19-notify-jobs.md` (classification of events into hints; "Hints of kind `admin` for privacy request deadlines: M7-06 ..."), `M3-05-file-download.md` (download tickets), `M1-35-identity-step-up.md`, `M1-61-sync-personal-data.md`, `M1-53-chat-channels.md`, `M1-60-messaging-mentions-templates.md`, `M2-05-runs-state-machine.md` (each names `personaldata.Participant` for M7), `M1-50-audit-verify.md`.
- Code on the base branch: `services/server/internal/privacy`, `internal/kernel/personaldata`, the modules with personal tables, `internal/notify` (consumer `push`), `internal/files`.

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
9. Periods only through the named keys of 20 13; the statutory deadline is not a configuration value (R-DP-05); code comments and docs name the key, never the value.
10. Specification first: the export format and any change to an operation go into `docs/api/` and `docs/spec/06-api.md` first. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`, `actions.yaml`) is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125).

Product requirements implemented here:

12. Only the data subject's own data and metadata are exported (Art. 15(4)); channel and thread names only where the person may still read, otherwise IDs; no secrets, hashes or keys; every file of the package is listed with its SHA-256 in `manifest.json` (DP-T03).
13. The package expires after `privacy.export_ttl`: the file and its object are deleted, `downloadGdprExport` answers `410 gone` (DP-T04).
14. The deletion runs the steps of 20 4.4 in order, each step idempotent and recorded in `progress`, so a crashed job resumes; the last owner cannot be deleted; pseudonymization leaves IDs, so the audit chain verifies unchanged afterwards (DP-T02).
15. One request per subject and kind may be open (partial unique index of 03); a second one gets `409` with the existing request named.
16. `gdpr.request_due` goes once per open request to every active owner when `due_at - privacy.request_reminder_lead` has passed; its hint carries only the kind and the request ID (13 2).
17. Every module with personal data implements `personaldata.Participant`; a coverage test fails for a table of 20 2 with personal data and no participant.

## Scope

Server module (paths under `services/server/` unless they start with `docs/`; every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`):

- `internal/kernel/personaldata`: the `Participant` interface (export of one subject into a writer per file of 20 5.2, erase steps 2 to 6 of 20 4.4 for its tables, the list of its tables for the coverage test).
- `internal/privacy`: `domain` (request state machine of 03, deadline computation with calendar months in UTC, package layout), `app` (create, read, list, decide, the jobs `privacy.export` and `privacy.erase`, expiry of packages, the reminder job that appends `gdpr.request_due`, `retention.privacy_job` through the port of M7-05), `adapters` (PostgreSQL for `gdpr_requests`, the ZIP writer), the handlers of the five operations.
- Participants in every module with personal data of 20 2 (for example `identity`, `chat`, `messaging`, `sync`, `runs`, `files`, `agents`, `notify`, `terminal`, `billing`, `audit` for `audit.jsonl`).
- `internal/notify`: the classification of `gdpr.request_due` into a hint of kind `admin` with the request ID as target, next to `access.expiring` (M3-19).
- `docs/api/`: the export format as a versioned JSON Schema per file of 20 5.2 (for example `docs/api/gdpr-export/v1/*.schema.json`), referenced from 06; `make api-lint` and `make api-checks` cover it.
- Configuration: `privacy.export_ttl`, `privacy.export_requires_approval`, `privacy.account_deletion_content`, `retention.privacy_job` become active; new key `privacy.request_reminder_lead` (20 13 and the reference).
- Docs: `docs/spec/20-privacy.md` 5 and 13, `docs/spec/06-api.md` (the export schema), `docs/compliance/` templates only where they name the export, READMEs of `privacy` and the touched modules, `services/server/CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-06-<n>`.

## Out of scope

- The deletion journal and its use on restore (M7-07); the views of the requests in the apps and the console (no plan row; listed as a follow-up).
- Data outside the server (20 4.7: client caches, agent runtimes, providers) beyond the events the server already sends.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-06-privacy-requests`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(privacy): ...`, `feat(notify): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). Tests bind only `127.0.0.1` and port 0; PostgreSQL only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. Logs of this ticket carry no person ID, name, email or content.
- Time only through `kernel/clock`; tests move a fake clock.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Deadline** (20 5.1, R-DP-05). `due_at` = receipt plus one calendar month in UTC; a day that does not exist in the target month moves to the last day of that month (record the rule with its legal source in the ruling). The extension by up to two further months (Art. 12(3)) is an additive optional field of `GdprDecision` (for example `extend_months` with the values 1 or 2), added to `docs/api/openapi.yaml` and 06 first; it is accepted only before the original `due_at`, needs the `reason` that is communicated to the person, moves `due_at` once and is recorded in the audit entry of the decision.

2. **Export writing** (20 5.2). The job streams each participant into its file of the package in a temporary directory, computes SHA-256 per file, writes `manifest.json` last, zips, stores the ZIP through the files port with `purpose = 'gdpr_export'` and `result_expires_at = now + privacy.export_ttl`. `README.txt` in the language of the person (`de` or `en` from `users.locale`).

3. **Erase order and resume** (20 4.4). Steps 1 to 7 as in 20, each step a call of all participants in their own transaction, `progress` records the last finished step; the job retries from there. Step 5 with `delete` uses the message deletion use case (20 4.2) per message, so tombstones, redaction and attachment purge follow; with `keep_pseudonymized` contents stay. Step 7 writes the audit entry; the journal entry is M7-07.

4. **Reminder** (R-AZ-35, D-W-108). A periodic job on queue `maintenance` finds open requests with `due_at - privacy.request_reminder_lead <= now` and no reminder sent yet (recorded in `progress`, so it is sent once) and appends one `gdpr.request_due` per active owner in the same transaction.

5. **Rulings for `docs/DECISIONS.md`** (`D-M7-06-<n>`): the month rule (note 1), the package writing and language (note 2), erase steps and resume (note 3), the reminder lead and once-only rule (note 4), the participant coverage exceptions.

## Tests (write them first)

Unit:

- `privacy/domain`: `TestDueAtCalendarMonth` (31 January, 29 February, end of year), `TestStateMachine` (allowed transitions of 03).
- `privacy/app`: `TestExportAutoApproved` and `TestExportNeedsApproval`, `TestDeletionNeedsOwner`, `TestLastOwnerRejected`.

Integration (`pgtest`, real modules through `internal/wire`):

- `TestDPT03_Export`: the package validates against the JSON Schemas; it contains no message of another person, no secret, no hash; every checksum matches; a channel the person may no longer read appears only by ID.
- `TestDPT04_ExportExpiry`: after `privacy.export_ttl` the file is gone and the download gives `410 gone`.
- `TestDPT02_Deletion` for `keep_pseudonymized` and `delete`: every step of 20 4.4 verifiable, no clear data in `principals` and `users`, `audit.verify` passes; a crash after step 3 resumes at step 4.
- `TestReminder`: one `gdpr.request_due` per owner once, and the consumer `push` produces one `admin` hint per owner with the request ID and no text.
- `TestEveryPersonalTableHasAParticipant` (coverage over 20 2).
- Handler tests for the five operations: step-up, `self or owner`, `409` for a second open request.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/... ./cmd/...`
- `make codegen-check`, `make api-lint`, `make api-checks`, `make arch`, `make register-check`, `make config-doc-check`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] DP-T02, DP-T03 and DP-T04 are green; the audit chain verifies after pseudonymization.
- [ ] Every module with personal data has a participant; the coverage test holds it.
- [ ] Owners get `gdpr.request_due` once per open request before the deadline and its `admin` hint.
- [ ] The export format is a versioned schema in `docs/api/` and in 06.
- [ ] 20, 06, READMEs, the configuration reference, the CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- READMEs of `privacy` and the touched modules updated in English.
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
4. The table of participants: module, tables, export file, erase steps.
5. What is not proven.
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
