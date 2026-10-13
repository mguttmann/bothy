# M4-25: Settings, devices and sessions on the desktop

Status: ready | Depends on: M4-18 | Milestone: M4

## Goal

After this ticket the desktop app has the settings of 01 5.1 D8: Appearance (theme, density, motion, Enter sends, column widths reset), Language (language and time zone of the profile), Keyboard (Enter sends, the list of shortcuts), Devices and sessions (own devices with platform, name, last activity, hardware binding, own sessions; revoke a device or end a session; confirm a waiting device by typing its verification code; add a device by its pairing code), Privacy (crash reports and performance data) and About (version and licenses), with a Notifications entry that M4-33 fills. Personal settings are synchronized across devices through `user_settings` (04 14), and every changing device action asks for step-up with the step-up device key (08 6). The device and session screens and the step-up dialog live in `packages/admin_ui`, because the console (M6) and the emergency actions on mobile (M5) reuse them. Why: confirming a waiting device and revoking a lost one are the security actions a person needs most (08 5, 10), and settings that differ between two devices of the same person look like a bug.

## Read first

- `docs/PLAN.md`: section 3; section 10 rows M4-18, M4-25, M4-26, M4-31, M4-33; section 11 (M5 emergency actions reuse `admin_ui`); section 12 row M6-01 (console uses `admin_ui`).
- `docs/spec/01-product.md`: 5.1 D8 "Settings", 5.6 E4 and E5 (the active-device side: "Settings, Devices and sessions, Confirm", "Add device"), 6.7 (destructive actions confirm once; step-up asks the device), 9 (language follows `users.locale`), 11 "Personal settings" (keys, values, defaults; the server rejects unknown keys), 12 (crash reports only with a configured target), 13 (R-PR-07).
- `docs/spec/08-identity.md`: 5.2 (approval with the verification code; a wrong code revokes the waiting device), 5.3 (pairing approval with step-up, `404` ends the step-up), 5.4 (management, revoke reasons), 6 (step-up: 6.1 to 6.3), 7 and 8 (sessions), 10 "Lost device".
- `docs/spec/04-sync-protocol.md` 14 (settings: last write wins per key, defaults not stored, `deleted: true`).
- `docs/spec/15-client-architecture.md` 3 (`admin_ui` used by desktop, console and mobile emergency actions), 3.1 (`admin_ui` depends on `design_system`, `auth`, `api_client`, `bothy_viz`; not on `sync_engine` or `local_store`), 8 (step-up), 16 (crash reports), 17 (client metrics switch).
- `docs/api/openapi.yaml`: `getMe`, `updateMyProfile` (`ProfileUpdate`), `listMySettings`, `putMySetting`, `deleteMySetting`, `listMyDevices`, `getDevice`, `approveDevice` (`DeviceApprove`), `approvePairing` (`PairingApprove`), `revokeDevice`, `listMySessions`, `revokeSession`, `createStepUpChallenge`, `completeStepUpWithDeviceKey`; problem codes `step_up_required` and its reasons.
- `docs/DECISIONS.md`: D-W-100, D-W-114; section 10.
- Briefs: `docs/briefs/M4-18-desktop-shell.md` (routes, providers, wiper, device key adapters), `docs/briefs/M4-09-auth-dpop.md` (`StepUpService`, `StepUpPrompt`, `StepUpInterceptor`), `docs/briefs/M4-04-design-system-theme.md` (`resolveDsContext`), `docs/briefs/M4-06-l10n.md` (`resolveAppLocale`), `docs/briefs/M4-15-sync-outbox.md` (setting executors); for the server side `docs/briefs/M1-38-identity-device-approval.md`, `docs/briefs/M1-45-identity-revocation.md`, `docs/briefs/M1-61-sync-personal-data.md`.

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
9. No new personal setting key: only the keys of 01 11 (and `notify.*` of 13 3.3 in M4-33); the server rejects others.
10. Step-up only through the `StepUpService` of M4-09; no own challenge handling.

Product requirements implemented here:

11. Settings of 01 11 with their values and defaults; defaults are not stored on the server; a changed value goes through the outbox as `putMySetting`, a reset as `deleteMySetting`; changes from another device apply per key (04 14).
12. `composer.enter_sends` without a stored value is `true` on desktop; a stored value applies on all devices (01 11).
13. `privacy.crash_reports` is offered only when the instance has a crash target; `privacy.client_metrics` default `true` (01 11, 15 16, 15 17).
14. Language and time zone are profile fields (`updateMyProfile`); the app language follows `users.locale` (01 9).
15. Devices: list with platform, name, last activity, "hardware-bound" or not, this device marked; revoke with one confirmation and step-up; a revoked own device ends in E7 on that device (08 10).
16. Confirm a waiting device: the person types the code shown on the waiting device, step-up, `approveDevice`; a wrong code revokes the waiting device and is shown as such (08 5.2).
17. Add a device: the pairing code from the new device, step-up, `approvePairing`; a code without an open pairing ends the step-up of this session and the next attempt asks again (08 5.3).
18. Sessions: list own sessions with device, method and last use; end one with step-up; ending the current one logs out and wipes (R-SY-14).
19. No permission logic: actions appear according to the server's answers, and a `403` shows its reason.

## Scope

Create:

- `packages/admin_ui/lib/src/self/` (`my_devices_view.dart`, `device_tile.dart`, `confirm_device_dialog.dart`, `add_device_dialog.dart`, `my_sessions_view.dart`, `revoke_confirm.dart`), `packages/admin_ui/lib/src/step_up/step_up_dialog.dart` (implements `StepUpPrompt` of M4-09: explains the action in the person's language before the system prompt), `packages/admin_ui/lib/src/data/self_repository.dart` (REST through `api_client`, no local database), exported from `lib/admin_ui.dart`.
- `apps/client/lib/settings/` (`settings_shell.dart` with the sections of D8, `appearance_section.dart`, `language_section.dart`, `keyboard_section.dart`, `devices_section.dart` hosting the `admin_ui` views, `privacy_section.dart`, `about_section.dart`, `notifications_entry.dart` as the link M4-33 fills, `settings_providers.dart` over `user_settings` with the defaults of 01 11).
- Fake server handlers through the registry of M4-17 for the operations above that it lacks, including a second simulated device of Alex.
- Tests under `packages/admin_ui/test/self/`, `apps/client/test/settings/`, goldens under both `test/goldens/`, integration tests under `apps/client/integration_test/` (Tests).
- ARB keys for every new text in `packages/l10n`.

Change:

- `packages/admin_ui/pubspec.yaml` (`api_client`, `auth`, `design_system`, `l10n`), `packages/admin_ui/README.md` (what the package offers to the desktop app, the console and the mobile emergency actions).
- `apps/client/lib/app/router.dart` (`/settings/<section>`), `apps/client/README.md`.
- `docs/DECISIONS.md` section 10: rows `D-M4-25-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Notification settings (D9) and the channel menu (M4-33).
- Administration of other people's devices (A1 in M4-26).
- Export of own data (Privacy "Export", GDPR flows in M7).
- Crash report and metric sending themselves (M4-31); this ticket only stores the switches.
- App lock settings (M5-08).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-25-settings-devices`; one writer per worktree.
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

1. **Settings model.** `SettingsRepository` (app side) reads `user_settings` from the local database through a provider and applies the defaults of 01 11 when a key is absent (`composer.enter_sends` by platform). Writes go to the outbox (`putMySetting`, `deleteMySetting` from M4-15); the local row changes at once and the echo is ignored by `updated_by_device_id` (04 14). Appearance changes rebuild the theme through `resolveDsContext` (M4-04) at once. "Column widths: Reset" deletes `layout.column_widths`.

2. **Profile.** Language and time zone through `updateMyProfile`; the language selector offers German and English; a change switches the app locale through `resolveAppLocale` (M4-06). Time zones from the IANA list the platform offers.

3. **Device actions.** `admin_ui` views take a `SelfRepository` (REST) and a `StepUpService`; every changing call (`approveDevice`, `approvePairing`, `revokeDevice`, `revokeSession`) runs through the `StepUpInterceptor` of M4-09, which shows `StepUpDialog` with the purpose text and then the OS prompt of the step-up key. Codes: the verification code field accepts digits only (`DeviceApprove.verification_code` pattern), the pairing code field normalizes like M4-09. A wrong verification code: the response `200` with `state = revoked` is shown as "The waiting device was rejected because the code did not match" (08 5.2). A pairing `404`: "No open pairing with this code" and the next attempt asks for step-up again (08 5.3). Revoking this device or ending the current session calls the wiper of M4-18.

4. **Privacy switches.** `privacy.crash_reports` is shown only when the instance or the build configuration names a crash target (the flag comes from M4-31 later; until then from the build defines of M4-18, absent means hidden); `privacy.client_metrics` is always shown.

5. **About.** App version from the build, instance version (`spec_version` of `getInstance`), licenses through Flutter's `LicenseRegistry` page.

6. **Rulings for docs/DECISIONS.md** (`D-M4-25-<n>`): settings defaults on the client (note 1); device and session screens in `admin_ui` with REST only (Scope, 15 3.1); step-up dialog in `admin_ui` (note 3); visibility of the crash switch (note 4).

## Tests (write them first)

Widget tests:

- `apps/client/test/settings/settings_test.dart`: defaults of 01 11 when keys are absent; a change writes one outbox entry and applies at once; reset deletes the key; theme, density and motion switch the theme.
- `packages/admin_ui/test/self/devices_test.dart`: list with the hardware label and this device marked; revoke asks once and then for step-up; confirm with a correct code activates, with a wrong code shows the rejection; pairing with `404` shows the text and asks for step-up again next time; actions hidden when the server's answer does not allow them.
- `packages/admin_ui/test/self/step_up_dialog_test.dart`: the dialog explains the action; cancel returns `StepUpCancelled`.
- Goldens of the settings sections and the device views, light and dark, two densities, text scaling 1 and 2, `de` and `en`.

Integration tests (`apps/client/integration_test/`, against `FakeServer` with two simulated devices of Alex):

- `settings_sync_test.dart`: theme and Enter sends changed on one device appear on the other (settings in sync across two simulated devices); a reset appears too.
- `devices_test.dart`: the second device logs in and waits; the first confirms it with the code and step-up; then revokes it, and the second device ends in E7 with a wiped store; ending the own session logs out.

Commands that must pass, from the repository root:

- `cd packages/admin_ui && flutter analyze --fatal-infos && flutter test --coverage`
- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS and on Windows: `make apps-platform`; elsewhere it prints its SKIP line
- `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Integration tests are green; settings stay in sync across two simulated devices.
- [ ] Devices and sessions: confirm with the verification code, add by pairing code, revoke and end with step-up, as 08 5 and 10 say.
- [ ] The settings of 01 11 with their defaults; language and time zone through the profile.
- [ ] `admin_ui` uses no `sync_engine` or `local_store` (`make arch`); rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 01, 04 or 08.
- `packages/admin_ui/README.md` and `apps/client/README.md` (English) as in Scope.
- Rulings of Design note 6 in `docs/DECISIONS.md` section 10.
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
4. Where the integration tests ran, and how the step-up prompt was exercised (fake signer in tests, real device key checked by hand or not).
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
