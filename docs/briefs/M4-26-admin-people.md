# M4-26: Administration: people, groups and devices

Status: ready | Depends on: M4-25 | Milestone: M4

## Goal

After this ticket the administration of 01 5.2 exists in `packages/admin_ui` with its shell (the navigation of A1: People, Groups, Grants, Devices, Agents, Apps, Audit, Statistics, Instance, Hosts, where the later tickets fill their entries) and the screens for people, groups and devices: the people list with search and filters, creating a person (no self-registration, no invitation links; an enrollment code shown once), a person's detail (owner flag, credentials, identities, passkeys, devices, sessions; disable and enable; new enrollment code for recovery), owners (add and revoke with the last-owner rule), groups (create, rename, delete, members with optional expiry), and the queue of waiting devices with the origin of the login (approve with the verification code, reject). Every changing action shows a confirmation and then the step-up of the step-up device key exactly where 05 marks the action with step-up (`docs/spec/actions.yaml`), through the step-up dialog of M4-25. The desktop app opens the shell at `/admin`. Why: administration is where permissions change; a screen that skips the step-up or invents its own permission logic would undermine SR5 and principle 1 of 15 (B7).

## Read first

- `docs/PLAN.md`: section 3; section 10 rows M4-25 to M4-29, M4-37; section 12 rows M6-04, M6-07 (the console reuses these screens).
- `docs/spec/01-product.md`: 3.7 "Administration, operations, integration", 5.2 "Administration" (the rule above A1 and A1 "People"), 6.7 (destructive actions confirm once; step-up asks the device), 13 (R-PR-07).
- `docs/spec/08-identity.md`: 3 (enrollment codes: shown once, `en_` format, purposes), 4 (owners, last owner), 5.2 (approval by an owner with the verification code), 5.4, 10, 11 and 12 (disable and enable a person, what ends), 6 (step-up).
- `docs/spec/07-permissions.md`: 2.1 (subjects, groups), 2.7 (owner), 5 SR5 (administration only by humans with confirmation), SR7 (disabled means no permissions), SR8 (last owner), 9.5.
- `docs/spec/05-actions.md` 2.4 (columns "Owner" and "Step-up") and `docs/spec/actions.yaml` (the `step_up` value of every action the screens call).
- `docs/api/openapi.yaml`: `listUsers`, `createUser`, `getUser`, `updateUser`, `disableUser`, `enableUser`, `createEnrollmentCode` (`EnrollmentCode`, `one_time_secret`), `getUserCredentials`, `unlinkIdentity`, `revokePasskey`, `listUserDevices`, `listUserSessions` (`Session` with `device_id` and `auth_method`), `listOwners`, `addOwner`, `revokeOwner`, `listGroups`, `createGroup`, `getGroup`, `updateGroup`, `deleteGroup`, `listGroupMembers`, `addGroupMember`, `removeGroupMember`, `approveDevice`, `revokeDevice`, `revokeSession`, `getMe` (`is_owner`); problem codes `last_owner` (SR8, `409`) and `i1_violation` (shown, resolved in M4-28).
- `docs/DECISIONS.md`: D-W-100, D-W-114; section 10.
- Briefs: `docs/briefs/M4-25-settings-devices.md` (`admin_ui` layout, `SelfRepository` pattern, `StepUpDialog`), `docs/briefs/M4-09-auth-dpop.md` (`StepUpInterceptor`), `docs/briefs/M4-07-core-components.md`, `docs/briefs/M4-17-fake-server.md` (handler registry); for the server side `docs/briefs/M1-31-identity-people.md`, `docs/briefs/M1-37-identity-bootstrap.md`, `docs/briefs/M1-45-identity-revocation.md`, `docs/briefs/M1-48-authz-api.md`.

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
9. No permission logic: a section, a button or a field appears according to the server's answers (`getMe.is_owner`, a `403` or `404` of the list call, the capabilities a response names), never from an own reading of grants (15 1).
10. The step-up is the server's decision (`401 step_up_required`); the screen never skips it and never adds one the server does not ask for.

Product requirements implemented here:

11. Every changing administration action with step-up shows a confirmation and then the step-up with the device key, before the change is submitted (01 5.2, SR5).
12. Creating a person gives an enrollment code that is shown once, copyable, read character by character; a repeated request does not show it again (`409` with `one_time_secret`) (08 3).
13. Disabling a person ends their sessions and shows that in the confirmation; enabling may hit I1 (`409 i1_violation` explained, resolved by M4-28) (08 11, 10 5.4).
14. The last owner cannot be revoked; the screen shows the server's reason (SR8).
15. The device queue shows platform, name, time and origin of the login. The API has no instance-wide list of waiting devices and no origin field on `Device`; the queue is therefore built from `listUsers`, `listUserDevices` filtered to `state = pending`, and the origin from `auth_method` of the session of that device in `listUserSessions` (`Session` carries `device_id` and `auth_method`); the shell's count comes from the same data. The `Device` schema has no hardware or attestation field either; 01 A1 shows "Key: hardware, attested", so the queue leaves that line out. The report names both gaps with the recommendation of a list operation for waiting devices (03 `devices` already names its index); approving needs the verification code from the waiting device; rejecting revokes it (08 5.2).
16. Group members can have an expiry; removing and adding members, renaming and deleting groups (07 2.1).
17. The administration is a package shared with the console; it uses REST only (15 3.1).

## Scope

Create in `packages/admin_ui/lib/src/`:

- `shell/admin_shell.dart` (navigation of A1 with the entries of 01 5.2; an entry without a screen yet shows nothing and is not listed; the "waiting devices" count), `shell/admin_routes.dart` (route names the apps map to `/admin/...`).
- `people/` (`people_list.dart`, `person_detail.dart`, `create_person_dialog.dart`, `enrollment_code_view.dart`, `credentials_section.dart`, `owners_section.dart`), `groups/` (`groups_list.dart`, `group_detail.dart`, `member_editor.dart` with expiry), `devices/` (`waiting_devices.dart`, `approve_device_dialog.dart`), `data/admin_repository.dart` (REST), `common/confirm_change.dart` (one confirmation with what changes, then the call that the step-up interceptor completes), `common/problem_banner.dart` (problem codes through `problemText`).
- Tests under `packages/admin_ui/test/admin/` and goldens (Tests).
- `apps/client/lib/admin/admin_route.dart` hosting the shell at `/admin`.
- Fake server handlers through the registry of M4-17 for the operations above, demanding step-up for exactly the actions whose `step_up` in `actions.yaml` is `true` (Design note 3).
- ARB keys for every new text in `packages/l10n`.

Change:

- `packages/admin_ui/README.md` (shell, screens, how the console embeds them), `apps/client/README.md`.
- `docs/DECISIONS.md` section 10: rows `D-M4-26-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Grants, "Who has access?" and "View as person" (M4-27); agents, backend switch and costs (M4-28); audit (M4-29); hosts (M4-37).
- Apps, statistics and instance settings (M6-08, M6-06, M6-09).
- Identity providers and login methods of the instance (M6-09).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-26-admin-people`; one writer per worktree.
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

1. **Repository and screens.** `AdminRepository` wraps the generated API with typed results and `ApiProblem`; screens are widgets that take the repository and a `StepUpService` through constructor parameters (the console passes its own, M6-04). Lists page with the API cursor; search and filters are request parameters where the API has them, otherwise local filters of the loaded page with a visible note.

2. **Confirmation then step-up.** `ConfirmChange` shows one dialog with what the change does in plain words (for example "Sam is disabled. All sessions of Sam end now.") and "Confirm"; then the call runs through the `StepUpInterceptor` of M4-09, which opens the `StepUpDialog` of M4-25 when the server answers `401 step_up_required`, and repeats the call with the same `Idempotency-Key`. Destructive actions (revoke, delete, disable, reject a device) use the danger style of `DsBanner` and ask once (01 6.7).

3. **Step-up per 05 in tests.** Widget tests in `packages/admin_ui` use an in-package fake of `AdminRepository` and a fake `StepUpService`, because `admin_ui` may not import `sync_engine/testing.dart` (15 3.1, the arch check of M4-01). The step-up enumeration runs as an integration test of the desktop app, `apps/client/integration_test/admin_step_up_test.dart`, against `FakeServer`: the fake server reads `docs/spec/actions.yaml` (walk up to the repository root) and answers `401 step_up_required` for an operation whose `x-authz-action` has `step_up: true` until the client completes the step-up. A test enumerates every operation the screens of this ticket call, looks up its action and `step_up`, and asserts that the screen goes through the step-up dialog exactly for those (and never for `false`). This holds "every action requires step-up according to 05" without a copy of the table.

4. **Enrollment codes.** Shown once in `CodeText` of M4-18's pattern (move it to `admin_ui` or `design_system` if the app's version cannot be imported from a package; packages never import from `apps/`, 15 3.1 rule 2), copyable, with the expiry; leaving the view discards it from memory.

5. **Rulings for docs/DECISIONS.md** (`D-M4-26-<n>`): repository and injected step-up (note 1); confirmation before the step-up (note 2); the step-up enumeration test (note 3); where the code view lives (note 4).

## Tests (write them first)

Widget tests in `packages/admin_ui/test/admin/` (in-package fake `AdminRepository`, fake `StepUpService`):

- `people_test.dart`: list, search, create with a one-time code view, disable with the sessions sentence, enable with `i1_violation` explained, credentials and devices of a person, owners with the last-owner rejection.
- `groups_test.dart`: create, rename, delete, members with expiry.
- `waiting_devices_test.dart`: the queue is assembled from `listUsers`, `listUserDevices` (pending only) and `listUserSessions` (origin from `auth_method`); approve with the code; reject.
- `apps/client/integration_test/admin_step_up_test.dart` (Design note 3, against `FakeServer`): every changing operation of these screens goes through the step-up dialog exactly when `actions.yaml` says `step_up: true` (and `fresh`; `conditional` as 05 2.4 defines it).
- `no_permission_logic_test.dart`: a `403` on a list hides the section with the server's reason; no widget reads grants.
- Goldens of the shell, people list, person detail, groups and the device queue; light and dark, two densities, text scaling 1 and 2, `de` and `en`.

Commands that must pass, from the repository root:

- `cd packages/admin_ui && flutter analyze --fatal-infos && flutter test --coverage`
- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS and on Windows: `make apps-platform` (runs the integration test); elsewhere it prints its SKIP line
- `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Widget tests of A1 are green; every changing action requires step-up exactly as 05 marks it.
- [ ] People, owners, groups with member expiry and the device queue work against the fake repository and, in the integration test, against the fake server, with the server's problem texts.
- [ ] Enrollment codes are shown once; no permission logic in the screens.
- [ ] `admin_ui` uses REST only (`make arch`); rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 01, 05, 07 or 08.
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
4. The list of operations with their action and `step_up` value as the enumeration test saw them.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
