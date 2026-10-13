# M4-37: Administration: hosts, pairing and grant `terminal`

Status: ready | Depends on: M4-27, M2-41 | Milestone: M4

## Goal

After this ticket the administration in `packages/admin_ui` has screen A8 of 01 5.2 for owners: the hosts list and a host page with state, assignee, platform and host service version, connection, fingerprint and key generation, LLM login, the server host question, check results, profiles, grants and session users; creating a host with name, optional assignee and reason; the pairing in three steps (a one-time pairing code with the copyable command for the host and its remaining time; typing the first characters of the fingerprint that `bothy-host pair` shows there, with `fingerprint_mismatch` and the remaining attempts on a wrong entry; the mandatory questions on the LLM login (I6) and the server host; the session user of each person chosen only from the names the host reported); assigning and ending session users with the explanation of `session_user_taken`; the check results with every named session user or former session user and the person of its assignment, also an ended one (rule for names: HD6, HD9, HD10, HD11); disable, re-pair and retire; and the grant wizard of M4-27 for the target host (capability `terminal`, no level) whose preview says "not effective" for a person who is not the assignee (SR10) or has no session user (SR12). Every change asks for a reason and step-up. Why: pairing and session users are where the terminal's protections are set up (24 2.2, 3.2, 7); a screen that hides an I6 violation or offers a name the host did not report would let an owner create a terminal that the server must then refuse or, worse, one that shares an account between people.

## Read first

- `docs/PLAN.md`: section 3; section 10 rows M4-26, M4-27, M4-37, M4-38; section 8 rows M2-39 to M2-42.
- `docs/spec/01-product.md` 5.2 A8 "Hosts" (both wireframes and the text above them), 9 (terms Host, Host service, Pairing, Fingerprint).
- `docs/spec/24-terminal.md`: 2.2 (session users, `reported_session_users`, the table of checks HD1 to HD11 and the rule for names below it), 2.3 (server host), 3 "Hosts and lifecycle" (3.1 states, 3.2 pairing, 3.3 re-pairing), 4.3 (fingerprint), 7 "LLM login and invariant I6" (7.2, 7.3), 8 (permissions, 8.1 to 8.4), 13 (problem codes), 14 (TM-T09, TM-T19, TM-T20, TM-T21, TM-T24, TM-T28 as the server's view of what the screen shows), 17 (`terminal.fingerprint_confirm_chars`, `terminal.fingerprint_max_attempts`).
- `docs/spec/07-permissions.md`: 2.12 (target `host`, capability `terminal`), SR10, SR11, SR12, 9.3 (preview hint `terminal_not_effective`).
- `docs/api/openapi.yaml`: `listHosts`, `createHost` (`HostCreate`), `getHost` (`Host`), `issueHostPairingCode` (`HostPairingCode`), `confirmHost` (fingerprint prefix, LLM credentials, server host, session users), `updateHost` (assignee, `add_session_users`, `revoke_session_users`, reason), `setHostLlmCredentials`, `setHostState` (`HostStateSet`), `listTerminalSessions`, `listAuditEvents` (the audit of the host for ended assignments), `createGrant` and `previewGrant` for `target_kind = host`; schemas `HostDoctorCheck` (or the name on the base branch), `HostSessionUser`, `HostReportedSessionUser`; problem codes `fingerprint_mismatch`, `i6_violation`, `conflict` with `session_user_taken`, `validation_failed` with `session_user_not_on_host`, `terminal_denied`.
- `docs/DECISIONS.md`: D-W-100, D-W-114; section 10.
- Briefs: `docs/briefs/M4-27-admin-grants.md` (wizard and preview panel), `docs/briefs/M4-26-admin-people.md` (shell, repository, `ConfirmChange`, step-up test, code view); for the server side `docs/briefs/M2-39-terminal-hosts.md`, `docs/briefs/M2-40-terminal-authz.md`, `docs/briefs/M2-41-host-pairing.md`.

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
9. Values only from the server and named keys: the number of fingerprint characters (`terminal.fingerprint_confirm_chars`) and the remaining attempts come from the server's responses or the reference default with a note; the code expiry from `expires_at`; no literal in code.
10. No permission logic: "not effective" and every denial come from the server's preview and problem responses (07 9.3), never from an own reading of SR10 to SR12.
11. Owners only: the Hosts entry of the administration appears when `getMe.is_owner` is true, as M4-26 gates its shell (`listHosts` itself answers non-owners with the hosts they are assigned to or hold a grant on, so it is no gate).

Product requirements implemented here:

12. Create a host with name, optional assignee and reason; issue a one-time pairing code shown with the command `bothy-host pair --server <origin> --code <code>` (copyable) and its remaining time as text (01 A8).
13. Confirm by typing the first characters of the fingerprint; a wrong entry shows `fingerprint_mismatch` with the remaining attempts; after the last attempt the host returns to `pairing` and the screen says so (24 3.2, TM-T19).
14. Mandatory answers on the LLM login (none, personal with a holder, organization) and on the server host; an I6 violation is explained ("Personal: only this person may open a terminal here, not even other owners"), not just rejected (24 7.3).
15. Session users per person only from the names the host reported (`reported_session_users`); a name that is taken shows `session_user_taken` with an explanation; ending an assignment keeps the name taken on the host (24 2.2, T12).
16. Check results with every named session user or former session user and the person of its assignment, also an ended one (rule for names: HD6, HD9, HD10, HD11), also while the host is disconnected after a refused start (24 2.2).
17. Disable, re-pair (a new pairing code; the key changes and the generation increases), retire; every change with a reason and step-up (01 A8, 24 3).
18. Grant wizard for the target host: capability `terminal`, no level; the preview says "not effective" for a non-assignee (SR10) and "not effective: no session user" for a person without one (SR12).

## Scope

Create in `packages/admin_ui/lib/src/hosts/`:

- `hosts_list.dart`, `host_page.dart` (all fields of A8), `create_host_dialog.dart`, `pairing_wizard.dart` (the three steps of the second A8 wireframe), `fingerprint_confirm.dart`, `llm_login_section.dart` (with the I6 explanation), `server_host_section.dart`, `session_users_section.dart` (assign from reported names, end, taken names), `doctor_results.dart` (Design note 2), `host_state_actions.dart` (disable, re-pair, retire), `host_grants_section.dart` (grants on the host with the preview states).
- The target kind `host` in the grant wizard of M4-27 (`target_step.dart`, `level_step.dart`: no level, capabilities exactly `[terminal]`), with the preview text and the badge for the hint `terminal_not_effective` (Design note 4).
- Registration of the Hosts entry in the admin shell.
- Fake server handlers through the registry of M4-17 for the operations above, with fingerprint attempts, I6 checks, taken names and doctor results that name session users.
- Tests under `packages/admin_ui/test/hosts/`, goldens, an integration test under `apps/client/integration_test/` (Tests); ARB keys in `packages/l10n`.
- `docs/quality/accessibility-checklist.md`: the rows for screen A8 and the pairing wizard (01 8) with their automated evidence, in the section "Screen A8: hosts" of the existing page (skeleton of the brief package, D-W-116); leave the other sections as they are.

Change:

- `packages/admin_ui/README.md`, `apps/client/README.md`.
- `docs/DECISIONS.md` section 10: rows `D-M4-37-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- The terminal itself (M4-35, M4-36) and its E2E (M4-38).
- Installing and pairing on the host (`bothy-host`, M2-42) beyond showing the command.
- Host administration in the console (M6-04 reuses these screens).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-37-admin-hosts`; one writer per worktree.
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

1. **Pairing wizard.** Step 1: name, assignee, reason, `createHost`. Step 2: `issueHostPairingCode`; the code in a copyable code view with the command line built from the instance origin, the remaining time from `expires_at`; "New code" issues another. Step 3: the fingerprint prefix field (normalized: separators removed, case ignored), the LLM login choice (none, personal with a holder picker, organization), the server host question, and the session user rows (person picker, then a picker of the names in `reported_session_users` that are not yet taken; no free text field); "Confirm with passkey / device" sends `confirmHost` through the step-up interceptor. `fingerprint_mismatch` keeps the step with the remaining attempts the response names; after the last one the host page shows state `pairing` and a new code is needed.

2. **Check results.** Each `doctor` entry of the host shows the check ID (HD1 to HD11), passed or the code, and, where the server names a session user, that name with the person of its assignment: a valid assignment comes from `Host.session_users`; an ended one is not in `getHost` (it returns valid assignments only), so for a named session user without a valid assignment the page reads the audit of the host (`listAuditEvents` with `resource_id` of the host and the `host.manage` actions; their details carry the assignment changes, 24 10.1; owner action with step-up as 05 marks `audit.read`) and shows "formerly assigned to Sam"; when the audit holds no assignment for the name, it shows the name alone and the report names the gap (the API has no list of ended assignments). Entries are shown for a disconnected host as well. The screen never guesses names; it shows what the server returns.

3. **Session users.** Assign: `updateHost` with `add_session_users` (person and a reported name); `409 conflict` with `session_user_taken` explains that a name once given to a person stays bound on this host; `400` with `session_user_not_on_host` explains that the host must report the name first. End: `revoke_session_users`, with the confirmation that the name stays taken.

4. **I6 and grants.** The LLM login section and the grant wizard show `409 i6_violation` with its `hint` in plain words; for a host target the wizard shows the server's preview `text` (07 9.3), which states "not effective" and its cause (not the assignee, no session user, server host), together with a "not effective" badge for every hint `terminal_not_effective`; the client never tells the causes apart itself (`PreviewHint` has no reason member, and that would be permission logic).

5. **Rulings for docs/DECISIONS.md** (`D-M4-37-<n>`): names only from the reported list (note 1, 3); check results as returned (note 2); host target in the shared wizard (note 4).

## Tests (write them first)

Widget tests (fake repository, fake signer):

- `pairing_test.dart`: code with command and remaining time; a wrong fingerprint shows `fingerprint_mismatch` with the remaining attempts; the last attempt returns the host to `pairing`.
- `i6_test.dart`: an I6 violation is explained, not just rejected.
- `session_users_test.dart`: only reported, untaken names are offered; a taken name shows `session_user_taken` with an explanation; ending keeps the name taken.
- `doctor_results_test.dart`: an entry HD9 with the name of a current session user shows its person from `session_users`, and with the name of a former session user the person from the audit of the host (ended assignment); HD6, HD10 and HD11 entries with names; a disconnected host still shows them.
- `host_grants_test.dart`: the preview of a grant to a non-assignee shows the server's text saying "not effective" (SR10) with the badge; for a person with a grant but without a session user the server's text says "not effective: no session user" (SR12); the client adds no cause of its own.
- `apps/client/integration_test/admin_step_up_test.dart` (M4-26) extended to the host operations: every change goes through step-up exactly as `actions.yaml` marks it.
- Goldens of the host page, the pairing steps and the check results; light and dark, two densities, text scaling 1 and 2, `de` and `en`.

Integration test (`apps/client/integration_test/host_admin_test.dart`, against `FakeServer`): Alex creates a host, gets a code, the fake simulates `bothy-host pair` reporting `term-alex` and `term-sam` (neutral names; the product name stays out of test data, 02 "Places that carry the product name"), Alex types a wrong prefix once and then the right one, chooses organization and "no" for the server host, assigns Sam to `term-sam`, grants Sam `terminal` (the preview says "not effective" until Sam is the assignee), makes Sam the assignee, and the preview becomes effective.

Commands that must pass, from the repository root:

- `cd packages/admin_ui && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS and on Windows: `make apps-platform`; elsewhere it prints its SKIP line
- `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] A wrong fingerprint shows `fingerprint_mismatch` with the remaining attempts; an I6 violation is explained, not just rejected.
- [ ] The preview of a grant to a non-assignee says "not effective" (SR10); a person with a grant but without a session user shows "not effective: no session user" (SR12).
- [ ] A taken name shows `session_user_taken` with an explanation; an entry HD9 with the name of a former session user shows the person of its assignment, also an ended one.
- [ ] Every change requires step-up; widget and integration tests green; rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 24 or 07.
- `packages/admin_ui/README.md` (English) as in Scope.
- Rulings of Design note 5 in `docs/DECISIONS.md` section 10.
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
4. Where the integration test ran, and the shape of the doctor entries and session user fields as found on the base branch.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
