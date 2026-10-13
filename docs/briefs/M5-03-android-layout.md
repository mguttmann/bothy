# M5-03: Android layout, navigation and notification channels

Status: ready | Depends on: M5-01, M5-02 | Milestone: M5

## Goal

After this ticket the client app runs on Android (API 26 and later) with the same phone screens as the iPhone (M5-02) and the Android controls of 01 5.4: system back (to the post list, out of a sheet, never ending a terminal session, M5-12), the top app bar, Material page routes and pickers, and sharing entry points left for M5-06. The Android runner is created (`flutter create --platforms=android`) with the build configuration from `branding.yaml` (application ID, URL scheme, App Links host if a public domain is configured) and neutral `example.com` defaults; the app registers the four notification channels of 01 10.3 ("Mentions", "Questions", "All messages", "Runs") with localized names at the first start, so they are visible and changeable in the system settings before the first push arrives (M5-05 posts into them). Login works through Custom Tabs and the app redirect (RFC 8252, 15 8), passkeys through the Credential Manager only when an App Links domain is configured, DPoP and step-up through the Android device key of M5-01, secrets through `flutter_secure_storage`, and the local database lives in the private app storage excluded from backups (15 6.3). In the background the gateway is suspended and resumed in the foreground (15 7.2), and the recent apps screenshot shows the privacy cover (`FLAG_SECURE` while the cover is up is not enough; Design note 5). Integration tests run on an emulator against the fake server of M4-17. Why: M5 needs an Android app with the same flows as the iPhone (P-20), built on the shared phone layouts instead of a second set of screens.

## Read first

- `docs/PLAN.md`: section 3; section 11 (Goal, Scope, Exit criteria, Test strategy, Red team), rows M5-02, M5-03, M5-05, M5-06, M5-12.
- `docs/spec/01-product.md`: 5.3 (the screens the Android app shares), 5.4 "Android" (system back, top app bar, notification channels, sharing through the system sheet; system back detaches a terminal session), 5.6 (E1 to E7), 8, 10.3 (row Android: the four channels, changeable in the system settings, grouping per thread), 11, 12.
- `docs/spec/15-client-architecture.md`: 2 (Android API 26), 6.3 (database in the private app storage, excluded from backups), 7.2 (`Suspended`), 8 (OIDC with Custom Tabs, App Links, passkeys only with a public domain), 9 (row Android), 10.2 (channels Mentions, Questions, All messages, Runs; instances without a Firebase project get no push on Android), 11 (deep links, App Links), 20.
- `docs/spec/13-push-notifications.md`: 2 (kinds of notifications), 3.4 (quiet hours: silent notifications), 6.2 (FCM payload keys, `k`, `quiet`), 8 (badge and `k = sync`).
- `docs/spec/08-identity.md`: 2.5, 6.1 (row Android), 14.1, 18 (`auth.webauthn_origins` with `android:apk-key-hash:` entries).
- ADRs: 0027, 0029, 0044, 0056 (item 4: Android sharing needs no extension).
- `docs/DECISIONS.md`: D-W-100 and its extension to M5 and M6 (D-W-120), D-W-115; section 10 (format of `D-M5-03-<n>` rows) and the rows `D-M5-01-<n>`, `D-M5-02-<n>`.
- Briefs (merged names win, D-W-100): `M5-02-iphone-layout.md` (phone shell, per-tab navigators, onboarding in phone form, privacy cover, lifecycle mapping), `M5-01-device-key-android.md` (Kotlin core module, Pigeon), `M4-09-auth-dpop.md` (`BrowserLauncher`, `RedirectReceiver`, `PasskeyAuthenticator`, `StepUpPrompt`), the brief of M4-17 (fake server), the brief of M4-18 (build configuration, router), `M4-01-workspace-arch.md` (platform tests and CI jobs).
- Code on the base branch: `apps/client` (lib with `phone/`, ios, macos, windows, integration_test), `packages/device_key/android`, `tools/ci/apps_platform.py` (system `android` of M5-01).

This ticket touches no API operation, no table, no action, no event and no server configuration key.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts, also the channel names, come from `packages/l10n` (passed to Kotlin through Pigeon), never as literals; build identifiers come from the build configuration with neutral `example.com` defaults.
9. Limits and intervals only through named configuration values (`InstanceLimits` from the server, `AppConfig` for app values, D-W-115); code comments name the key, never the number.
10. Specification first: a change to behaviour or configuration updates `docs/spec/` in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. The phone screens of M5-02 with Android controls: system back pops the current stack, closes a sheet, and on the root of a tab leaves the app; top app bar; Material routes and pickers (01 5.4).
12. Four notification channels "Mentions", "Questions", "All messages", "Runs" with localized names and descriptions, created at the first start and on every start when missing, visible in the system settings (01 10.3, 15 10.2).
13. Login with Custom Tabs and the app redirect (custom scheme or App Link); passkeys only with a configured App Links domain; no WebView (15 8).
14. The local database lives in the private app storage and is excluded from backups and device transfer (15 6.3); tokens in `flutter_secure_storage`; device keys from M5-01.
15. In the background the app suspends the gateway; the recent apps screen shows no content (01 5.6 E6, R-PR-12 for the terminal).
16. Touch targets pass `androidTapTargetGuideline`; text scaling up to 200 percent without loss.

## Scope

Create:

- `apps/client/android/` with `flutter create --platforms=android` (minimum SDK 26, target SDK the current stable), the build configuration hook for `branding.yaml` (application ID, label, URL scheme, App Links host with `autoVerify` only when configured) with neutral defaults, `allowBackup="false"` and `dataExtractionRules` excluding everything (Design note 1).
- `apps/client/android/app/src/main/kotlin/.../`: `MainActivity` and the Pigeon host glue for Custom Tabs (`BrowserLauncher`, redirect delivery to `RedirectReceiver`), the Credential Manager passkey adapter (`PasskeyAuthenticator`), the notification channel registrar, and the recent-apps protection hook (Design notes 2 to 5).
- `apps/client/pigeons/android_platform.dart` (or an extension of the Pigeon file of M5-02 if it is shared) and the generated code.
- `apps/client/lib/phone/android/`: the Android-specific adaptations of the phone shell (system back handling through `PopScope`, top app bar variant), selected by platform in the phone shell of M5-02.
- `apps/client/lib/platform/notification_channels.dart`: the channel list with IDs, importance and the kind mapping of Design note 4, handed to Kotlin at start.
- Tests (Tests).

Change:

- `apps/client/lib/phone/` (M5-02): platform adaptive pieces where the iOS and Android behaviour differs (back, pickers, page routes); no duplicated screens.
- `apps/client/platform-tests.json`: an `android` entry that runs `flutter test integration_test/phone` and the instrumented tests of the runner on the attached device (the system `android` of M5-01).
- `packages/l10n` ARB files: channel names and descriptions, Android texts.
- `apps/client/README.md` (Android build, configuration, emulator tests), `docs/spec/15-client-architecture.md` 10.2 (the kind to channel mapping), root `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M5-03-1` to `D-M5-03-6`.

## Out of scope

- The FCM service, the placeholder notification and the preview fetch (M5-05); the runtime permission for notifications on API 33 and later (M5-05 asks for it with the push registration).
- The share activity (M5-06), voice and camera (M5-07), app lock (M5-08), emergency actions (M5-09), terminal (M5-12), live views (M6-16).
- Distribution (M7).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M5-03-android-layout`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(client): ...`, `feat(client-android): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never sign with a real key or upload a build.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 (the emulator reaches the host's fake server through `10.0.2.2` only as the emulator's alias of the host loopback; nothing binds to another interface) and make no other network call. Temporary directories only via `mktemp -d`, removed at the end. Stop only processes you started, by PID or by the emulator serial.
- Never read or print secrets. No network calls except package downloads for pinned tools, pub packages, Gradle dependencies and system images.
- The Flutter SDK at the pinned version, a JDK and the Android SDK must be available; the emulator tests run where an emulator or device is attached (`make apps-platform`) and in the `apps-android` CI job of M5-01; the report names where they ran. Without a JDK or Android SDK the Android build fails by design and the ticket stops with `Verdict: BLOCKED <tool> missing`.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- The product name appears only at the places of 02 "Code layout"; the app shows `branding.product_name` from the instance.
- No permission logic in the client (15 1 principle 1).
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Runner and build configuration** (ADR 0044; 15 6.3; Ruling for docs/DECISIONS.md). `flutter create --platforms=android --org com.example` inside `apps/client`; `minSdk 26`; the manifest gets the custom scheme intent filter for the redirect and deep links, and an App Links intent filter with `android:autoVerify="true"` only when `branding.yaml` names a host. `android:allowBackup="false"`, `android:fullBackupContent` and `dataExtractionRules` exclude all domains (the database, the secure storage and the shared preferences) so no backup or device transfer copies them (15 6.3: drift points out that backups can restore an old database). No permission is declared that this ticket does not use.

2. **Login** (15 8; 08 14.1, 18; Ruling for docs/DECISIONS.md). `BrowserLauncher`: Custom Tabs through `androidx.browser` (`CustomTabsIntent`), falling back to the default browser intent when no Custom Tabs provider exists; never a WebView. The redirect arrives at `MainActivity` (single top) and goes to `RedirectReceiver` through Pigeon. `PasskeyAuthenticator`: `androidx.credentials` `CredentialManager` with `GetPublicKeyCredentialOption`, only when the App Links host equals the relying party ID of the instance; the instance must list the app origin `android:apk-key-hash:<hash>` in `auth.webauthn_origins` (08 18); the README explains how an operator derives the hash. Otherwise E2 hides the passkey button.

3. **Device key and secrets.** The adapter from `device_key` (M5-01) to the signer port of `auth` is the one M4-18 wrote for the desktop; on Android the step-up prompt texts come from `l10n`. `flutter_secure_storage` with its Android options for the Keystore-backed encryption (15 9 row Android).

4. **Notification channels** (01 10.3; 13 2; Ruling for docs/DECISIONS.md). Channel IDs are stable codes (`mentions`, `questions`, `messages`, `runs`); names and descriptions come from `l10n` in the device language and are updated on each start (`createNotificationChannel` updates names, not importance). Importance: `mentions` and `questions` high, `messages` default, `runs` default. Kind to channel mapping, used by M5-05: `mention` and `dm` to `mentions`; `input_needed`, `security` and `admin` to `questions` (each asks the person to act); `message` to `messages`; `run_done` and `run_failed` to `runs`. Quiet notifications (`quiet = 1`, 13 3.4) are posted silently by M5-05 into the same channel with `setSilent(true)`, so the person's channel settings stay the only control. Write the mapping into 15 10.2.

5. **Background and recent apps** (15 7.2; 01 5.6 E6; Ruling for docs/DECISIONS.md). The lifecycle mapping of M5-02 applies (suspend on `paused`, resume on `resumed`). Android takes the recent apps thumbnail when the activity stops; to show no content there, set `FLAG_SECURE` on the window while the privacy cover is required (always, when the app lock of M5-08 is on or a terminal is open; the minimum of this ticket is: while the app is not in the foreground) and draw the cover on `inactive`; document that `FLAG_SECURE` also blocks screenshots and screen recording of the app while set, which 01 and the terminal red team (PLAN 11) want for the terminal. Choose the narrowest rule that hides content in the recent apps on API 26 and on the current API (on API 33 and later `setRecentsScreenshotEnabled(false)` exists; use it there and record the rule per API level).

6. **Rulings for `docs/DECISIONS.md`** (`D-M5-03-<n>`): backup exclusion (note 1); App Links only with a configured host (note 1); Custom Tabs and the Credential Manager (note 2); channel IDs, importance and the kind mapping (note 4); the recent apps protection per API level (note 5); M5-03 builds on the phone layouts of M5-02 instead of separate screens (Goal).

## Tests (write them first)

Widget (`apps/client/test/phone/`, `flutter test`, with `debugDefaultTargetPlatformOverride = TargetPlatform.android`):

- `android_back_test.dart`: system back pops the stack, closes a sheet, leaves the app at a tab root (the test checks `SystemNavigator.pop` through a fake).
- `android_goldens_test.dart`: M1, M2, M3 and E2 in the Android variant, light and dark, text scaling 1 and 2.
- `tap_target_android_test.dart`: `androidTapTargetGuideline` on every phone screen.
- `channels_test.dart`: the channel list has four entries with stable IDs and l10n names in de and en; the kind mapping covers every kind of 13 2.

Instrumented and integration (emulator or device, fake server of M4-17):

- `NotificationChannelsTest` (Kotlin, `androidTest`): after the app started once, `NotificationManager.getNotificationChannels()` returns the four channels with the expected IDs, names and importance (PLAN row: visible in the system settings).
- `BackupExclusionTest` (Kotlin): the merged manifest has `allowBackup = false` and the data extraction rules exclude every domain.
- `integration_test/phone/login_flow_test.dart` and `navigation_flow_test.dart` of M5-02 run on Android as well (same files; platform branches only where the controls differ).
- `recents_protection_test` (Kotlin or integration): moving to the background sets the protection of Design note 5 (window flag or recents screenshot setting) and the cover is in the widget tree.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- `cd apps/client && flutter build apk --debug` (the Android build compiles with the neutral configuration)
- with an attached emulator or device: `make apps-platform`; otherwise it prints its SKIP line
- `make codegen-check`, `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make licenses`
- `make check`

## Acceptance criteria

- [ ] The Android app shows the phone screens with Android controls; integration tests pass on the emulator against the fake server.
- [ ] The four notification channels are visible in the system settings with localized names; the kind mapping is documented in 15 10.2.
- [ ] Login uses Custom Tabs; passkeys only with an App Links host; no WebView.
- [ ] The database and secrets are excluded from backups; the recent apps screen shows no content.
- [ ] README, 15 10.2, ARB files, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public Dart symbol and KDoc on every public Kotlin declaration; comments explain why.
- `apps/client/README.md` updated in English.
- Rulings of Design note 6 recorded in `docs/DECISIONS.md` section 10.
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
4. Where the Android tests ran (emulator image and API levels, physical devices).
5. What is not proven (for example Custom Tabs against a real identity provider, passkeys with a verified App Link).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
