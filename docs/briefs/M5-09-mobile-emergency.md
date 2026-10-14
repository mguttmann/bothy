# M5-09: Emergency actions on mobile

Status: ready | Depends on: M4-27, M5-03 | Milestone: M5

## Goal

After this ticket an owner can do the four emergency actions of the administration on the phone (01 3.7 row "Emergency actions on the phone", R-PR-07): revoke a device (`device.revoke`), revoke a grant (`grant.revoke`), disable a person (`user.disable`) and stop a run (`run.cancel`). They live in `packages/admin_ui` as phone layouts of the existing administration controllers (M4-26, M4-27) and in the Me tab and the run views of the app: a compact "Emergency" area under Me lists people (search), their devices and their grants with one action each, and the stop button of the live box and of "Running now" stays where M5-02 put it. Who sees which action follows 05: revoking another person's device (`device.list`, `device.revoke`: self or owner) and disabling a person (`user.disable`: owner) appear only for owners; a space manager sees only revoking grants within the targets they manage (`grant.revoke`) and stopping runs where `run.cancel` allows it; everyone keeps revoking their own devices in the settings of M4-25. Every action that 05 marks with step-up asks for the step-up device key with biometrics or the device credential (08 6.2, 14.3) and repeats the request with the same `Idempotency-Key`; every confirmation says that revoking takes effect at once (07 9.5). The phone offers nothing beyond these four actions: creating grants, the audit view and statistics stay on desktop and console (R-PR-07). Integration tests run with step-up on the device against the fake server. Why: disabling and revoking must work on the go, while complex permission work on a small screen is error-prone (R-PR-07).

## Read first

- `docs/spec/01-product.md`: 3.7 (row "Emergency actions on the phone"), 4 (information architecture: "emergency actions on mobile"), 5.2 A1 (people, devices; the queue of waiting devices), A3 (grants with "Revoke"), 5.3 M3 and D10 (stop a run: `run.cancel` only where allowed), 13 (R-PR-07).
- `docs/spec/05-actions.md` (core contract, read only): rows `device.revoke`, `grant.revoke`, `user.disable`, `run.cancel` (step-up and audit columns; who may call each), 2.4 (step-up), `user.read`, `device.list`, `grant.read`.
- `docs/spec/07-permissions.md`: 2.6 (managers revoke only within their scope), 9.5 ("Revoking takes effect immediately and says so in the confirmation"), SR5 (administration only by humans with confirmation), SR8 (last owner).
- `docs/spec/08-identity.md`: 6.2, 10 (lost device), 11 (disabling a person), 14.3 (step-up flow).
- `docs/api/openapi.yaml`: `listUsers`, `getUser`, `listUserDevices`, `revokeDevice`, `disableUser`, `listGrants`, `revokeGrant`, `cancelRun`.
- `docs/DECISIONS.md`: D-W-100 and its extension to M5 and M6 (D-W-120); section 10 (format of `D-M5-09-<n>` rows) and the rows of M4-26, M4-27.
- Briefs (merged names win, D-W-100): the briefs of M4-26 and M4-27 (`admin_ui` controllers, the step-up flow in the UI, the plain-language confirmation), `M4-09-auth-dpop.md` (`StepUpService`, `StepUpInterceptor`, same `Idempotency-Key`), the brief of M4-21 (stop in the live box), `M5-02-iphone-layout.md` and `M5-03-android-layout.md` (phone shell, Me tab).
- Code on the base branch: `packages/admin_ui`, `apps/client/lib/phone`, `packages/auth`.

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
10. Specification first: a change to behaviour updates `docs/spec/` in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. Exactly the four actions on the phone: revoke a device, revoke a grant, disable a person, stop a run; no grant creation, no audit view, no statistics, no group editing (R-PR-07).
12. Each action that 05 marks with step-up runs through the step-up device key with system authentication and repeats the request with the same `Idempotency-Key`; a cancelled prompt sends nothing (08 6.2, 14.3; M4-09).
13. Each confirmation names the effect in plain language and says that it takes effect at once (07 9.5); disabling the last owner is explained when the server answers with the SR8 reason, not prevented by client logic.
14. Controls follow the server: the actions appear only where the server's responses allow them (for example the person list and the grants the caller can read; `403` shows its reason); no client derivation from grants (15 1 principle 1).
15. Touch targets, text scaling up to 200 percent and screen reader labels as on every phone screen.

## Scope

Create:

- `packages/admin_ui/lib/src/emergency/`: phone layouts `EmergencyHome`, `PersonEmergencyView` (devices with "Revoke", grants with "Revoke", "Disable person"), confirmation sheets with the plain-language text and the step-up, reusing the controllers and repositories of M4-26 and M4-27 (Design notes 1 and 2).
- `apps/client/lib/phone/me/emergency_entry.dart`: the entry under Me, shown when the caller is an owner or manages a space (from the server's self information and capabilities the app already has; Design note 3).
- Tests (Tests).

Change:

- `packages/admin_ui` exports and README (the emergency layouts and their rule); `packages/l10n` ARB files.
- `apps/client/lib/phone/` the Me tab; the live box and Running now keep the stop of M4-21 and M5-02 (verify that stop asks for nothing beyond what `run.cancel` requires).
- Docs: `docs/spec/01-product.md` 5.3 (a short wireframe "Emergency" in the style of M1 to M10 and one sentence on its entry), root `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M5-09-1` to `D-M5-09-4`.

## Out of scope

- Any other administration on the phone (R-PR-07); the console (M6).
- Server behaviour of the four actions (M1-45, M1-48, M2-09).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M5-09-mobile-emergency`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(admin-ui): ...`, `feat(client): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and use the fake server of M4-17 (extended with the four operations and the step-up challenge where it lacks them). Temporary directories only via `mktemp -d`, removed at the end. Stop only processes you started, by PID; simulators by UDID, emulators by serial.
- Never read or print secrets.
- The device tests need macOS with Xcode and an iOS simulator, and an Android emulator; where missing, `make apps-platform` prints its SKIP line; the report names where they ran.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- No permission logic in the client (15 1 principle 1).
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Reuse, not rebuild** (15 3 `admin_ui`; R-CL-01; Ruling for docs/DECISIONS.md). The emergency views are phone layouts over the controllers and repositories that M4-26 and M4-27 built for the desktop and the console; no second data layer. If a controller is coupled to a desktop widget, split the controller out in `admin_ui` with a test that the desktop still passes.

2. **Confirmation and step-up** (07 9.5; 08 14.3). Each action opens a sheet: what happens in plain language from `l10n` ("Sam's device 'Pixel' is revoked at once. It is logged out and deletes its local data at the next contact."), a reason field where the operation takes one, and the confirm button labelled "Confirm with Face ID" or the platform equivalent for the actions that 05 marks with step-up (`run.cancel` has none and gets a plain "Stop run"). Confirming sends the request; on `401 step_up_required` the `StepUpInterceptor` of M4-09 runs the device key step-up and repeats with the same `Idempotency-Key`; a cancel leaves everything unchanged and says so. The result shows the server's outcome; a `403` or `409` shows the reason (for example SR8 for the last owner).

3. **Entry** (01 4). The Me tab shows "Emergency" only for callers whose self information marks them as owner or who manage at least one space (the server data the app already holds; no derivation from grants); owners see all four actions, managers only grant revocation within their scope and stopping runs (05 rows; a `403` still shows its reason). The person search uses `listUsers` with the server's filter; a manager sees only what the server returns.

4. **Rulings for `docs/DECISIONS.md`** (`D-M5-09-<n>`): reuse of the admin controllers (note 1); the confirmation text and the step-up path (note 2); the entry rule (note 3); the exact list of actions on the phone (requirement 11).

## Tests (write them first)

Widget and golden (`packages/admin_ui/test/emergency/`, `apps/client/test/phone/me/`): the four actions with their sheets in light and dark, text scaling 1 and 2, de and en; labels and tap targets; no other administration entry on the phone (a test walks the Me tab and asserts the closed list of entries); the entry hidden for a person without owner status and without `manage`.

Integration (`apps/client/integration_test/phone/emergency_test.dart`, iOS simulator and Android emulator, fake server with step-up):

- revoke Sam's device: the fake server first answers `401 step_up_required`, the device key step-up runs with the simulated biometric match, the request repeats with the same `Idempotency-Key` and succeeds; a cancelled prompt sends no second request.
- revoke a grant, disable Sam, stop a run: each with the step-up where 05 requires it and the result shown; disabling the last owner shows the SR8 explanation from the server.

Commands that must pass, from the repository root:

- `cd packages/admin_ui && flutter analyze --fatal-infos && flutter test --coverage`
- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS with Xcode and with an attached Android emulator: `make apps-platform`; otherwise the SKIP lines
- `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Integration tests with step-up on the device pass for all four actions on iOS and Android.
- [ ] The phone offers exactly the four emergency actions; confirmations say that revoking takes effect at once.
- [ ] The admin controllers are reused; no permission logic in the client.
- [ ] 01 5.3, README, ARB files, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public Dart symbol; comments explain why.
- `packages/admin_ui/README.md` updated in English.
- Rulings of Design note 4 recorded in `docs/DECISIONS.md` section 10.
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
4. Where the device tests ran.
5. What is not proven.
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
