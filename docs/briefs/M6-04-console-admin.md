# M6-04: Administration in the console

Status: ready | Depends on: M4-28, M4-29, M6-03 | Milestone: M6

## Goal

After this ticket the console offers the administration screens A1 to A6 of 01 5.2 from `packages/admin_ui`, the same screens as on the desktop (R-CL-01): A1 People (list, create, the queue of waiting devices with approve and reject), Devices, Groups and grants with A2 (grant wizard with the plain-language preview and the I1 check before submitting), A3 "Who has access?", A4 "View as person" on the stored view of the console session (M6-01: banner with name and remaining time on every page, read-only, end), A5 Agents with the backend switch, costs and tokens, and A6 Audit with filters, the chain check and the CSV export for owners. Every changing action shows its confirmation and then the step-up with a passkey (or the console's other step-up methods where 05 allows them), the console's HTTP layer sends the CSRF header, and nothing in the console bypasses the server's permission decisions. `patrol` web tests run the flows in a browser against the Compose stack inside `make console-e2e`, under the enforced CSP of M6-03. Why: owners must be able to administer the instance without the app (B5, ADR 0028), with the same packages and the same checks as the desktop.

## Read first

- `docs/spec/01-product.md`: 5.2 (A1 to A6 and the sentence on confirmation with step-up), 5.5 (console navigation), 8, 9.
- `docs/spec/07-permissions.md`: 2.6 (delegation limits), 9.1 ("Who has access?"), 9.2 ("View as person"; the stored view of M6-01), 9.3 (plain-text preview, the golden example), 9.5 (revoking takes effect at once; filterable audit), SR5, SR8.
- `docs/spec/10-backend-policy.md` (I1 and the guided switch "switch to organization first", BR-07).
- `docs/spec/05-actions.md` (core contract, read only): the actions behind A1 to A6, their step-up and audit columns; `access.view_as`, `access.view_as.end`.
- `docs/spec/08-identity.md` 5.2 (device approval with the typed verification code), 6.2 (console step-up methods), 12 (API tokens shown once).
- `docs/adr/0028-*.md` (no chat in the browser; route test), `docs/spec/15-client-architecture.md` 3.1 rule 3, 20 (row E2E: `patrol` for the console).
- `docs/DECISIONS.md`: D-W-100 and its extension to M5 and M6 (D-W-120); section 10 (format of `D-M6-04-<n>` rows) and the rows of M4-26 to M4-29, M6-01 to M6-03.
- Briefs (merged names win, D-W-100): the briefs of M4-26 (people, groups, devices), M4-27 (grant wizard, Who has access, View as person), M4-28 (agents, backend switch, costs, tokens), M4-29 (audit view), `M6-01-console-session.md` (stored view, step-up methods), `M6-02-console-app.md` (shell, routes, HTTP layer, passkey adapter), `M6-03-console-csp.md` (`console-e2e`, the headless runner).
- Code on the base branch: `packages/admin_ui`, `apps/console`, `tools/console-e2e`.

This ticket touches no API operation, no table, no action, no event and no configuration key.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names. Interface texts come from `packages/l10n`.
9. No configuration key is added.
10. Specification first: a change to behaviour updates `docs/spec/` in the same change (01 5.2 A4 for the console, Design note 3). The core contract is not edited by this ticket.

Product requirements implemented here:

11. A1 to A6 in the console from `admin_ui`, the same screens and controllers as on the desktop; layouts at full width for the browser (01 5.5).
12. Every changing action shows its confirmation with the plain-language effect and then the step-up (01 5.2; 07 9.3, 9.5); revoking says that it takes effect at once.
13. "View as person" uses the stored view of the console session: banner with name and remaining time on every page, read-only, an end button, audit at start and end on the server (07 9.2).
14. No content routes in the console (ADR 0028): view-as shows structure and access, never message content (Design note 3).
15. Tokens and codes that the server shows once (API tokens, enrollment codes) appear once with a copy action and are not kept in browser storage.
16. The flows run as `patrol` web tests under the enforced CSP (SEC-T05 holds for the new screens).

## Scope

Create:

- `apps/console/lib/admin/`: the console routes and page frames for A1 to A6 that host the `admin_ui` screens, the view-as banner frame, the step-up adapter that runs the passkey (or the console's other methods) when an action answers `401 step_up_required` and repeats with the same `Idempotency-Key` (Design notes 1 and 2).
- `apps/console/integration_test/admin/`: `patrol` web tests of the flows (Design note 4).

Change:

- `packages/admin_ui`: layouts that adapt to the browser width where the desktop layout assumed a sidebar of the app; the view-as entry point that calls `viewAsUser` and the banner state from `getConsoleSession`; no new controllers unless a desktop controller cannot run without the app (then split it with tests that the desktop passes).
- `tools/console-e2e`: run the `patrol` web suites inside `make console-e2e` with the CSP enforced (Design note 4).
- `packages/l10n` ARB files; `apps/console/README.md`; `docs/spec/01-product.md` 5.2 A4 (console: structure and access only); root `CHANGELOG.md` under `[Unreleased]`; `docs/DECISIONS.md` section 10 rows `D-M6-04-1` to `D-M6-04-4`.

## Out of scope

- A7 statistics (M6-06), permission hints and expiry list (M6-07), apps and webhooks (M6-08), instance, branding, providers and flags (M6-09), A8 hosts and terminal (M6-18).
- Import (M8).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M6-04-console-admin`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(console): ...`, `feat(admin-ui): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules); the stack publishes its ports only on `127.0.0.1`.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets; tokens shown once in a test are compared and discarded, never logged.
- Docker, the pinned Flutter and the pinned browsers must be available for `make console-e2e`; the ticket must run it once on the build host or stop with `Verdict: BLOCKED <tool> missing`.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never relax the CSP.
- No permission logic in the client (15 1 principle 1).
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Hosting `admin_ui`** (R-CL-01; Ruling for docs/DECISIONS.md). The console routes of M6-02 (`people`, `groups`, `devices`, `agents`, `audit`) host the screens; the console provides the repositories with its cookie-based `api_client` and the step-up adapter; `admin_ui` stays free of console-specific code except width-adaptive layouts.

2. **Step-up in the browser** (08 6.2; 01 5.2). The confirmation dialog of `admin_ui` comes first; on confirm the request goes out; `401 step_up_required` starts the console step-up: the passkey through WebAuthn with `userVerification = required` when the person has one, otherwise the other methods `getConsoleSession` lists; after success the cookie is rotated by the server (M6-01) and the request repeats once with the same `Idempotency-Key`. A cancel sends nothing.

3. **View as person in the console** (07 9.2; ADR 0028; Ruling for docs/DECISIONS.md). Starting from A3 or A1 calls `viewAsUser`; the response gives the visible spaces and channels; the console shows them as a structure list and the "Not visible" list, and keeps the banner on every page until `endViewAs` or the end time from `getConsoleSession`. The console does not open channel content, because it has no content routes (ADR 0028); the desktop app keeps showing content read-only (M4-27). Update 01 5.2 A4 with this sentence for the console.

4. **`patrol` web tests** (15 20; SEC-T05). Flows on the Compose stack with a seeded instance: A1 create a person and approve a waiting device (the device registered by the test through the API, approval with the typed verification code); A2 create a grant with the preview text (compare with the golden example of 07 9.3 where the seed matches it) and an I1 rejection explained; A3 open "Who has access?" for Support; A4 start view as Sam, see the banner and the structure, end it; A5 switch an agent's billing mode through the guided flow and create a token that is shown once; A6 filter the audit by action and run the chain check, and export CSV as the owner. Each flow runs with the virtual authenticator for the step-up and with the CSP violation collector of M6-03 active; any violation fails the suite.

5. **Rulings for `docs/DECISIONS.md`** (`D-M6-04-<n>`): hosting `admin_ui` in the console (note 1); the step-up order in the browser (note 2); view as person without content in the console (note 3); the `patrol` web suites inside `console-e2e` (note 4).

## Tests (write them first)

- Widget and golden tests of the console pages with the `admin_ui` screens at browser widths (light and dark, text scaling 1 and 2, de and en); the view-as banner on every page while active.
- `apps/console/integration_test/admin/*_test.dart`: the `patrol` web flows of Design note 4, red first before the routes exist.
- The route test of M6-02 still finds no content route.

Commands that must pass, from the repository root:

- `cd packages/admin_ui && flutter analyze --fatal-infos && flutter test --coverage`
- `cd apps/console && flutter analyze --fatal-infos && flutter test --coverage`
- `make console-e2e` on the build host
- `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] `patrol` web tests of the flows A1 to A6 are green under the enforced CSP.
- [ ] Every changing action shows its confirmation and the step-up; revoking says it takes effect at once.
- [ ] View as person shows the banner on every page, is read-only, ends by the button or the time, and shows no content in the console.
- [ ] 01 5.2 A4, README, ARB files, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public Dart symbol; comments explain why.
- `apps/console/README.md` updated in English.
- Rulings of Design note 5 recorded in `docs/DECISIONS.md` section 10.
- Root `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check` and of `make console-e2e`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
