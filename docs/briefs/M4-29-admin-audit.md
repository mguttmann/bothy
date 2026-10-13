# M4-29: Administration: audit view

Status: ready | Depends on: M4-26 | Milestone: M4

## Goal

After this ticket the administration in `packages/admin_ui` has A6 of 01 5.2: the audit log as a list (time, who, action, result, resource, reason) filterable by action, person, result, resource and time range with cursor paging, an entry's details (correlation ID, the deny reason, the details the server returns), the check of the hash chain (`verifyAuditChain`) with its result visible ("Chain checked up to no. n: unchanged", or the first broken entry and the warnings), and the export of a time range as the server's NDJSON stream with hashes (verifiable offline) and, for spreadsheets, a CSV converted locally from the same stream. Reading, checking and exporting are owner actions with step-up as 05 marks them. Why: the audit log is the record of every permission decision and administration action (ADR 0021); an owner must be able to see and check it without a database.

## Read first

- `docs/PLAN.md`: section 3; section 10 rows M4-26, M4-29; section 12 row M6-04.
- `docs/spec/01-product.md` 5.2 A6 "Audit".
- `docs/spec/03-data-model.md` `audit_events` (fields, `decision`, `deny_reason`, hash chain) and the audit section of 07 9.5.
- `docs/spec/05-actions.md` (`audit.read`, `audit.verify` with owner and step-up) and `docs/spec/actions.yaml`.
- `docs/spec/19-security.md` (audit requirements) and `docs/adr/0021-hash-chained-audit-log-separate-from-telemetry.md`.
- `docs/api/openapi.yaml`: `listAuditEvents` (filters `actor_id`, `action`, `decision`, `resource_kind`, `resource_id`, time range, cursor), `exportAuditEvents` (`application/x-ndjson`, `from`, `to`), `verifyAuditChain` (`AuditVerifyRequest`, `AuditVerifyResult`), schema `AuditEvent`.
- `docs/DECISIONS.md`: D-W-100, D-W-114; section 10.
- Briefs: `docs/briefs/M4-26-admin-people.md` (shell, repository, step-up test); for the server side `docs/briefs/M1-24-audit-chain.md`, `docs/briefs/M1-50-audit-verify.md`.

This ticket touches no API operation, no table of chapter 03, no action, no event and no configuration key.

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
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them, and the ARB test of M4-06 rejects them in interface texts. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts come from `packages/l10n`, never as literals in widgets.
9. No new configuration key; page sizes come from the server's defaults.
10. Specification first: 01 A6 names "Export CSV" while `exportAuditEvents` delivers NDJSON with hashes; this ticket offers both (Design note 3) and adjusts the line of 01 A6 in the same change.

Product requirements implemented here:

11. Filter by action, person, result (allowed, denied), resource and time range; cursor paging; times in the person's time zone (01 A6).
12. The chain check shows its result: checked range, `ok` or the first broken entry, the checkpoints compared and the warnings (ADR 0021).
13. Export of a time range: the server's NDJSON unchanged (hash links kept, a cut stream is detectable) and a CSV converted locally from it.
14. Reading, checking and exporting are owner actions with step-up (05); the screen shows the server's `403` otherwise.
15. Audit entries are never edited or deleted from the interface.

## Scope

Create in `packages/admin_ui/lib/src/audit/`:

- `audit_list.dart` (virtualized, filters, paging), `audit_filters.dart` (action from the catalogue names the server returns or a free field, person picker, result, resource, time range), `audit_entry_detail.dart`, `chain_check.dart` (`verifyAuditChain` with the result panel), `audit_export.dart` (Design note 3).
- Registration of the Audit entry in the admin shell.
- Fake server handlers through the registry of M4-17 for the three operations, with a chain that can be made broken at a given entry.
- Tests under `packages/admin_ui/test/audit/`, goldens (Tests).
- ARB keys for every new text in `packages/l10n`.

Change:

- `docs/spec/01-product.md` 5.2 A6 (the export line: NDJSON with hashes and a CSV converted on the device).
- `packages/admin_ui/README.md`.
- `docs/DECISIONS.md` section 10: rows `D-M4-29-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- The history link from "Who has access?" (M6-04) and the permission hints list (M6-07).
- Alerts on a broken chain (operations, M7).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-29-admin-audit`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(client): ...`, `feat(sync-engine): ...`, `build(gates): ...`, `docs(spec): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a Dart package (`publish_to: none` in every `pubspec.yaml`).
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and make no network call; fakes replace the server unless a section below names a container. Temporary directories only via `mktemp -d` (Dart `Directory.systemTemp.createTemp`, pytest `tmp_path`), removed at the end. Stop only processes you started, by PID.
- Never read or print secrets. No network calls except package downloads for pinned tools and pub packages and what the gates of `make check` do themselves.
- The Flutter SDK at the version pinned in the root `pubspec.yaml` must be on `PATH` (and Go and Docker where a section names them); without them the gates fail by design (R-QA-02), and the ticket stops with `Verdict: BLOCKED <tool> missing` instead of skipping.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- The product name appears only at the places of 02 "Code layout" ("Places that carry the product name"); the app shows `branding.product_name` from the instance, never a literal.
- No permission logic in the client (15 1 principle 1): controls follow server responses, snapshots and events, never an own derivation from grants.
- If the core contract (chapters 00, 03, 04, 05) or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M4 by D-W-114). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **List and filters.** Filters map one to one to the query parameters of `listAuditEvents`; the action filter offers the action IDs of the loaded entries plus a free field (the catalogue is not shipped to the client as a list of its own). Entries render the action ID with its translated label where `l10n` has one, otherwise the ID; the resource as kind and the name the server returns, or the ID.

2. **Chain check.** "Check chain" calls `verifyAuditChain` without a range (the server checks from the last checkpoint) or with the range of the current filter; the panel shows "Chain checked up to no. <checked_to_seq>: unchanged" when `ok`, otherwise the first broken entry, the checkpoints compared and the warnings, with the danger style.

3. **Export.** "Export" asks for a file path (save dialog) and streams `exportAuditEvents` for the time range of the filter to that file unchanged. "Export as CSV" streams the same response and writes one CSV row per line with the columns time, actor, action, decision, resource kind, resource ID, deny reason, correlation ID, `seq`, `hash`, `prev_hash` (RFC 4180 quoting; a cell that starts with `=`, `+`, `-` or `@` gets a leading apostrophe against formula injection). Nothing is kept in the app after the write. Ruling for docs/DECISIONS.md.

4. **Rulings for docs/DECISIONS.md** (`D-M4-29-<n>`): the action filter source (note 1); the chain check range (note 2); NDJSON and CSV export with the injection guard (note 3).

## Tests (write them first)

Widget tests (fake repository, fake signer):

- `audit_list_test.dart`: each filter sets its parameter; paging; times in the person's time zone; entry details.
- `chain_check_test.dart`: an intact chain shows "unchanged" with the checked number; a broken one shows the first broken entry and the warnings.
- `audit_export_test.dart`: the NDJSON file equals the server's bytes; the CSV has the columns, quotes correctly and guards cells that start with `=`, `+`, `-` or `@`.
- `audit_step_up_test.dart`: reading, checking and exporting go through the step-up dialog as `actions.yaml` marks `audit.read` and `audit.verify`.
- Goldens of the list with filters and the chain result in both states; light and dark, two densities, text scaling 1 and 2, `de` and `en`.

Commands that must pass, from the repository root:

- `cd packages/admin_ui && flutter analyze --fatal-infos && flutter test --coverage`
- `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Widget tests are green; the result of the chain check is visible in both states.
- [ ] Filters, paging and entry details as A6 shows; owner actions with step-up.
- [ ] Export as NDJSON unchanged and as a CSV with the injection guard; 01 A6 updated.
- [ ] Rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 01, 03 or ADR 0021.
- `packages/admin_ui/README.md` (English) as in Scope.
- 01 A6 updated; rulings of Design note 4 in `docs/DECISIONS.md` section 10.
- Root `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. The filters the API on the base branch offers compared with A6, and the export sizes of the test fixtures.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
