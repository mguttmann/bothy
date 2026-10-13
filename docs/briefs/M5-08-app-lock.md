# M5-08: App lock

Status: ready | Depends on: M5-03 | Milestone: M5

## Goal

After this ticket the phone apps lock (08 9, 01 5.6 E6): when the person has turned the app lock on, or the owner requires it instance-wide with `security.app_lock_required`, the app shows E6 at every start and after `client.app_lock_after` without use, hides all content (also in the app switcher and the recent apps screen), and unlocks only through the system's biometrics or device credential (iOS `LocalAuthentication`, Android `BiometricPrompt` with the device credential); after a failure the lock stays with "Try again", "Log out" leads to E7 and deletes the local data. The server delivers the two values the app needs, because both have place `Server` in the configuration reference: `getInstance` gains the member `app_lock` with `required` and `after_ms` (spec first in `openapi.yaml`, served by the instance module from the process configuration). When the lock is required, the setting cannot be turned off, and a device without a device credential cannot use the app until one is set. The lock publishes a `locked` event that the terminal view of M5-12 uses to detach and that the quick reattach rule follows ("only after the app lock", PLAN 11 red team). The local database stays encrypted with the key from secure storage, which is readable only after the device was unlocked (15 9: `WhenUnlockedThisDeviceOnly` on iOS); a test proves that a copy of the database file is unreadable without that key. Why: a lost or borrowed unlocked phone must not show chats, administration or terminals to the next person (T-DV-01); the server cannot check the lock, so the app must enforce it (08 9).

## Read first

- `docs/spec/08-identity.md`: 9 "App lock" (all of it), 10 (lost device; the lock is the residual protection), 18 (`client.app_lock_after`, `security.app_lock_required`).
- `docs/spec/01-product.md`: 5.6 E6 (screen, "Try again", "Log out" to E7, the lock detaches terminal sessions), E7, 5.3 M8 (reattach after the window with Face ID or fingerprint), 11 (personal settings; the app lock switch is not a synced key, Design note 4).
- `docs/spec/15-client-architecture.md`: 6.3 (database key in secure storage, protection class `complete`), 9 (rows iOS and Android: DB key `WhenUnlockedThisDeviceOnly`), 23.2 item 7 (detach in the background), 21 (R-CL-08).
- `docs/operations/configuration.md`: rows `client.app_lock_after` and `security.app_lock_required` (place `Server`, status `planned (M5)`).
- `docs/api/openapi.yaml`: `getInstance`, schemas `InstanceInfo`, `InstanceLimits`.
- `docs/security/threat-model.md`: T-DV-01 and the M5 red team items in `docs/PLAN.md` section 11 ("bypassing the app lock").
- `docs/DECISIONS.md`: D-W-100 and its extension to M5 and M6 (D-W-120), D-W-115; section 10 (format of `D-M5-08-<n>` rows).
- Briefs (merged names win, D-W-100): `M1-46-instance-settings.md` (`getInstance` and the instance module), `M5-02-iphone-layout.md` (E6 screen, privacy cover, lifecycle mapping), `M5-03-android-layout.md` (recent apps protection), the brief of M4-12 (database key and encryption), the brief of M4-18 (logout deletes local data, R-SY-14).
- Code on the base branch: `services/server/internal/instance`, `apps/client/lib`, `packages/local_store`.

This ticket touches no table, no action and no event.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts come from `packages/l10n`.
9. Limits only through named configuration keys: `client.app_lock_after` and `security.app_lock_required` from the server's process configuration, delivered by `getInstance`; code comments name the key, never the number.
10. Specification first: the new member of `InstanceInfo` goes into `docs/api/openapi.yaml` first; the configuration reference rows change from `planned (M5)` to active with this ticket as the first user. The core contract is not edited by this ticket.

Product requirements implemented here:

11. The lock applies at every cold start and after `client.app_lock_after` without use, when the person turned it on or `security.app_lock_required` is true (08 9).
12. While locked, no content is visible: the E6 screen replaces the whole app, the app switcher and recent apps show no content, notifications tapped while locked open E6 first and then the target (01 5.6 E6).
13. Unlock only through the system authentication (biometrics or device credential); a failure keeps the lock; "Log out" ends the session, deletes local data and keys, and shows E7.
14. With `security.app_lock_required` the switch is on and disabled; a device without a device credential shows a notice and no content until a credential is set.
15. Locking publishes an event that detaches open terminal sessions (they keep running on the host, 01 5.6 E6); the lock controller itself never reattaches; the terminal view of M5-12 decides that after `unlocked`.
16. The local database is readable only with the key from secure storage; the key is stored `ThisDeviceOnly` and, on iOS, `WhenUnlocked` (15 9).

## Scope

Server (paths under `services/server/` unless they start with `docs/`; every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`):

- `docs/api/openapi.yaml`: `InstanceInfo` gains `app_lock` (`AppLockPolicy {required: boolean, after_ms: integer int64}`, required in the response, description naming both keys); `make api-lint`, `make api-checks`, `make codegen-check`.
- `internal/instance`: `getInstance` fills `app_lock` from the configuration keys `client.app_lock_after` and `security.app_lock_required`, which enter the configuration schema with this ticket (`make config-reference`, `make config-doc-check`), with a handler test.

Client:

- `apps/client/lib/lock/`: `AppLockController` (policy from `getInstance` cached in the local database, the person's switch, last activity time from an injected clock, state `unlocked`, `locked`, `unavailable`), `AppLockPort` with implementations over the platform authentication (`local_auth` or Pigeon to `LAContext` and `BiometricPrompt`; choose by license and maintenance and record it), the `locked` event stream, and the E6 screen wiring (Design notes 1 to 5).
- `apps/client/lib/phone/settings/`: the switch in settings (Me, Security) with the disabled state under `required`.
- Tests (Tests).

Change:

- `tools/configdoc/planned.yaml`: remove the planned rules of `client.app_lock_after` and `security.app_lock_required`, which this ticket makes active (otherwise `make config-doc-check` reports them as stale); `services/server/internal/wire` the key file that registers both keys.
- `packages/local_store`: the cached `app_lock` policy with the instance row (Design note 1) and, if Design note 6 needs it, the migration of the stored key item, with the migration test of 15 6.2.
- `apps/client/lib/platform/lifecycle.dart` (M5-02): the cover and the lock decision on `inactive`, `paused` and `resumed`; activity tracking on pointer and keyboard events.
- Router: while locked every route resolves to E6; a deep link or notification tap is kept and opened after unlock.
- iOS `Info.plist`: `NSFaceIDUsageDescription` (localized).
- `packages/l10n` ARB files: E6 texts, settings texts, the "no device credential" notice.
- Docs: `docs/spec/08-identity.md` 9 (the delivery through `getInstance`, the device-local switch, the rule without a device credential), `docs/spec/15-client-architecture.md` 9 (the lock and the database key), `apps/client/README.md`, `services/server/internal/instance/README.md`, `services/server/CHANGELOG.md` and root `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M5-08-1` to `D-M5-08-6`.

## Out of scope

- The app lock on desktop (macOS, Windows): 08 9 names Windows Hello as well, but the plan places the lock in M5 for the phones; the desktop gets it with a later ticket that reuses `AppLockController` (record it as a follow-up).
- The terminal's reattach logic (M5-12 consumes the `locked` event).
- An instance setting for the policy in the console (the keys are process configuration; M6-09 shows them read-only if it lists process configuration at all).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M5-08-app-lock`; one writer per worktree.
- Conventional Commits with the module or component as scope (for example `feat(instance): ...`, `feat(client): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Server tests bind only `127.0.0.1` and port 0; PostgreSQL only through `pgtest`. Client tests use the fake server of M4-17; device tests enrol and match biometrics without a person: on the iOS simulator through `xcrun simctl spawn <udid> notifyutil -s com.apple.BiometricKit.enrollmentChanged 1` and `notifyutil -p com.apple.BiometricKit_Sim.fingerTouch.match` (or `.nomatch`), on the emulator through `adb -s <serial> emu finger touch <id>` after enrolling a fingerprint with the emulator's settings automation; record the commands that worked. Temporary directories only via `mktemp -d`, removed at the end. Stop only processes you started, by PID; simulators by UDID, emulators by serial.
- Never read or print secrets; never log the database key.
- The device tests need macOS with Xcode and an iOS simulator, and an Android emulator; where missing, `make apps-platform` prints its SKIP line; the report names where they ran.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Policy delivery** (08 9; Ruling for docs/DECISIONS.md). `getInstance` returns `app_lock: {required, after_ms}` for every authenticated caller; the app stores it with the instance row (15 6.1 `instance`) so the lock works offline and at a cold start before the network. Reason for an API member instead of `AppConfig` (D-W-115): the owner must be able to require the lock, so the value must come from the instance, not from the build.

2. **When to lock** (08 9; 01 5.6 E6). Lock at a cold start (before the first frame with content: the first route is E6 when the lock applies), and when the time since the last user activity reaches `client.app_lock_after` while the app is in the foreground or since it went to the background. Activity is any pointer or key event; a running upload or an incoming message is not activity. The clock is injected; the decision is a pure function tested without a device.

3. **Unlock** (08 9; Ruling for docs/DECISIONS.md). The system prompt with biometrics or the device credential (iOS `LAPolicy.deviceOwnerAuthentication`; Android `BIOMETRIC_STRONG or DEVICE_CREDENTIAL` from API 30, biometrics with the device credential fallback intent before it). Success unlocks; cancel or failure keeps E6 with "Try again". No counter of the app's own: the system limits attempts. "Log out" calls the logout of M4-18 (server logout best effort, local data, keys and tokens deleted, E7).

4. **The switch** (01 11; Ruling for docs/DECISIONS.md). The person's choice is a device-local preference stored in secure storage, not a synced `user_settings` key, because the lock protects this device and another device may lack biometrics. With `required` the switch shows on and disabled with the reason. Turning the lock on asks for authentication once (so a person cannot lock themselves out on a device without a credential). Without a device credential and with `required`, the app shows only a notice with the way to the system settings and "Log out".

5. **Hiding content and the event** (01 5.6 E6; 15 23.2 item 7). The privacy cover of M5-02 and the recent apps protection of M5-03 are always on while the lock is on. The controller emits `AppLockEvent.locked` before E6 appears and `unlocked` after success; M5-12 subscribes to detach terminal sessions and to require the system authentication before a quick reattach.

6. **Database key** (15 6.3, 9). Verify that the database key of M4-12 is stored with `ThisDeviceOnly` and on iOS `WhenUnlocked` (not `AfterFirstUnlock`, which the extensions' tokens use); if M4-12 chose another class, change it to these values with a migration of the stored item (read with the old attributes, write with the new) and a test.

7. **Rulings for `docs/DECISIONS.md`** (`D-M5-08-<n>`): policy in `getInstance` (note 1); lock triggers and activity (note 2); unlock and log out (note 3); device-local switch and the rule without a device credential (note 4); the lock event for the terminal (note 5); the database key accessibility (note 6).

## Tests (write them first)

Server (`pgtest`, full chain): `TestGetInstance_AppLock`: the default configuration gives `required = false` and `after_ms` from `client.app_lock_after`; with `security.app_lock_required = true` the response says so; the members are present for every authenticated caller and absent for an unauthenticated discovery call.

Client unit and widget (`apps/client/test/lock/`, fake clock and fake authentication port):

- `lock_decision_test.dart`: locked at a cold start when on; not locked when off and not required; locked after `client.app_lock_after` of inactivity in the foreground and in the background; activity resets the timer; an incoming message does not.
- `lock_screen_test.dart`: E6 replaces every route; no message text in the widget tree while locked; a deep link is opened after unlock; failure keeps E6; "Log out" deletes local data (the fake store is empty) and shows E7.
- `lock_switch_test.dart`: `required` shows the switch on and disabled; turning on asks for authentication; without a device credential and `required` only the notice and "Log out" are shown.
- `lock_event_test.dart`: `locked` is emitted before E6, `unlocked` after success.
- `database_key_test.dart` (PLAN row): a copy of the database file opened with a wrong or no key fails; the key item has the expected accessibility attributes (through the secure storage fake that records them).

Device (`patrol`, `apps/client/integration_test/phone/app_lock_test.dart`, iOS simulator with enrolled Face ID, Android emulator with an enrolled fingerprint, fake server):

- enable the lock; send the app to the background beyond the configured `client.app_lock_after` (set very low in the fake `getInstance`); return: E6 is shown and the app switcher snapshot contains no content (assert the cover is in the tree when the snapshot is taken); a failed match keeps the lock; a matching biometric unlocks.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/instance/... ./internal/wire/...`
- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS with Xcode and with an attached Android emulator: `make apps-platform`; otherwise the SKIP lines
- `make codegen-check`, `make api-lint`, `make api-checks`, `make config-doc-check`, `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Device test: the lock appears after `client.app_lock_after` and at start, hides content, and unlocks only through the system authentication.
- [ ] `security.app_lock_required` enforces the lock instance-wide through `getInstance`; the switch is disabled then.
- [ ] The local database is readable only with the key from secure storage (test with a copied file).
- [ ] The `locked` event exists for the terminal; "Log out" deletes local data and shows E7.
- [ ] `openapi.yaml`, 08 9, 15 9, the configuration reference, READMEs, CHANGELOGs and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public Dart and Go symbol; comments explain why.
- `apps/client/README.md` and the instance README updated in English.
- Rulings of Design note 7 recorded in `docs/DECISIONS.md` section 10.
- `services/server/CHANGELOG.md` and root `CHANGELOG.md` entries under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. Where the device tests ran; the chosen authentication package with version and license.
5. What is not proven (for example the app switcher image on a physical device).
6. Follow-ups that could not be done in this scope (the desktop lock), each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
