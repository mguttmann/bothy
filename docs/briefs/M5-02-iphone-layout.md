# M5-02: iPhone layout and navigation

Status: ready | Depends on: M4-21, M4-23 | Milestone: M5

## Goal

After this ticket the client app runs on the iPhone with the screens of 01 5.3 in phone form: the navigation stack (spaces, then channel or post list, then history; back by the edge swipe), the bottom tab bar (Spaces, Mentions, Running, Me), M1 Spaces, M2 Post list, M3 History with the compact live box whose stop button stays reachable and whose full view opens as a sheet, M4 large components at full width with forms as sheets and selections as the system picker, M7 Offline with the banner and the outbox states, and the onboarding screens E1 to E7 (01 5.6) in phone form, with E2 offering the one-time code autofill and E6 covering the content in the app switcher. The phone layouts are platform-neutral widgets in `apps/client/lib` that M5-03 reuses for Android; this ticket adds the iOS runner (`flutter create --platforms=ios`), its build configuration from the instance's `branding.yaml` with the neutral `example.com` default (ADR 0044), the iOS login adapters (OIDC with `ASWebAuthenticationSession` and the custom URL scheme, passkeys only with an associated domain), the protection class `NSFileProtectionComplete` for the local database, the suspended state of the sync engine in the background (15 7.2), and deep links `<scheme>://t/{thread_id}?m={message_id}` into the router. Text scales up to 200 percent without loss, touch targets follow `density.target-min` of the `touch` context. Widget and integration tests run against the fake server of M4-17. Why: M5 builds the mobile apps on the client foundation of M4 (P-20); every later M5 ticket (push, sharing, voice, app lock, emergency actions, terminal) attaches to this shell.

## Read first

- `docs/PLAN.md`: section 3; section 11 (Goal, Scope, Out of scope: iPad layout is V3, Exit criteria, Test strategy, Red team), rows M5-02 to M5-13 (what attaches to this shell); section 10 rows M4-18 to M4-25 (the desktop features this ticket lays out for the phone).
- `docs/spec/01-product.md`: 3 (feature list, rows marked for mobile), 4 (information architecture), 5.3 "iPhone" (M1 to M7; M8 and M9 come with M5-12, M10 with M6-16), 5.6 "Onboarding and sign-in" (rules for all onboarding screens, E1 to E7), 6 "Interaction rules" (6.1 to 6.7), 8 "Accessibility", 9 "Languages", 10.3 (iOS grouping and badge come with M5-04), 11 "Personal settings" (`appearance.density` fixed to `touch` on mobile, `composer.enter_sends` default `false` on mobile), 12 "Branding per instance" (build layer), 13 (R-PR-07, R-PR-14).
- `docs/spec/15-client-architecture.md`: 1, 2 (iOS 15), 3 and 3.1, 4, 6.3 (iOS protection class `complete`, the extensions do not need the database), 7.2 (state `Suspended`: no WebSocket in the background, close with `1000`, `resume` in the foreground), 8 (login: OIDC with PKCE in the system browser, redirect through a custom URL scheme or a claimed HTTPS link; passkeys in native apps only for instances with a public domain), 9 (row iOS), 11 (deep links), 19 (`client.perf.*` mobile targets; measured in M5-11), 20.
- `docs/spec/08-identity.md`: 2.5, 3.2 (code input rules), 5.2, 5.3, 6.2, 14.1 (OIDC flow of a native app), 14.2.
- `docs/design/components.md` (all components in the `touch` density), `docs/design/spacing-radii-elevation-motion.md` (`density.target-min`), `docs/design/README.md`.
- ADRs: 0027 (Flutter), 0029 (client state), 0030 (tokens), 0044 (branding: build layer, `branding.yaml`).
- `docs/DECISIONS.md`: D-W-100 and its extension to M5 and M6 (D-W-120), D-W-114, D-W-115 (`AppConfig`); section 10 (format of `D-M5-02-<n>` rows) and the rows of M4-18 to M4-21.
- Briefs (merged names win, D-W-100): the brief of M4-18 (router, deep links, login and device confirmation, E1 to E7 widgets and their goldens, build configuration for macOS and Windows), the briefs of M4-19 to M4-21 (sidebar, post list, thread view, input, live box, components, commands), the brief of M4-17 (fake server in `packages/sync_engine`, library `testing.dart`), `M4-09-auth-dpop.md` (`LoginService`, `BrowserLauncher`, `RedirectReceiver`, `PasskeyAuthenticator`, `StepUpPrompt`), `M4-10-device-key-apple.md` (iOS keys), the brief of M4-12 (local database and its encryption key), the brief of M4-13 (state `Suspended`), `M4-04-design-system-theme.md` (density `touch`), `M4-01-workspace-arch.md` (`platform-tests.json`, the platform CI jobs).
- Code on the base branch: `apps/client` (lib, macos, windows, integration_test), `packages/*`.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts come from `packages/l10n`, never as literals in widgets; the app shows `branding.product_name` from the instance, never a literal; build identifiers come from the build configuration with neutral `example.com` defaults.
9. Limits and intervals only through named configuration values: server-delivered limits from `getInstance` (`InstanceLimits`), app values from `AppConfig` of `packages/domain` (D-W-115); code comments name the key, never the number.
10. Specification first: a change to behaviour or configuration updates `docs/spec/` in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. Screens M1 to M7 of 01 5.3 and E1 to E7 of 01 5.6 in phone form, with the navigation stack, the edge swipe back and the tab bar; a link or a push opens the thread at the message (01 5.3; 15 11).
12. Touch targets at least `density.target-min` in the `touch` context; density is fixed to `touch` on the phone (01 11); text scaling up to 200 percent without loss of function, the layout wraps (01 8, SC 1.4.4).
13. Onboarding rules of 01 5.6: no instance content before login; codes in groups in the mono font, copyable and read character by character; code fields accept pasting and ignore case, spaces and hyphens; remaining times as text announced at most once per `a11y.live_announce_min_interval`; an invalid certificate gives no continue and no exception button (R-PR-14); E2 offers autofill of the system's one-time codes for the TOTP step.
14. E6 covers the content when the app is inactive or in the app switcher; the lock logic itself comes with M5-08, this ticket adds the cover and the E6 screen.
15. The live box stop is always reachable at the top right of its row; approval buttons are stacked at full width; no gesture triggers an approval (R-PR-04).
16. In the background there is no WebSocket: the app closes with `1000` and resumes in the foreground (15 7.2).
17. The local database file has protection class `NSFileProtectionComplete`; tokens and keys follow 15 9 row iOS; the extensions are not built here.
18. No permission logic in the client (15 1 principle 1); no WebView anywhere (15 8, SEC-VZ-1).

## Scope

Create:

- `apps/client/ios/` with `flutter create --platforms=ios` (iPhone only: `TARGETED_DEVICE_FAMILY = 1`, minimum iOS 15), the build configuration hook that reads `branding.yaml` (display name, bundle ID, URL scheme, associated domain if any, signing team placeholder) with neutral defaults under `example.com`, the same way M4-18 does for macOS (Design note 1).
- `apps/client/ios/Runner/`: Swift glue for the login adapters (`ASWebAuthenticationSession` for `BrowserLauncher` and `RedirectReceiver`; `ASAuthorizationPlatformPublicKeyCredentialProvider` for `PasskeyAuthenticator` when an associated domain is configured), exposed to Dart through a Pigeon interface in `apps/client/pigeons/` (Design note 3).
- `apps/client/lib/phone/`: the phone shell (tab bar, navigation stack, routes), the screens M1 to M7 as phone layouts composed of the components and controllers of M4-19 to M4-21 and of the cross-cutting views of M4-23 (Mentions, Running now, search), with the Me tab holding settings, devices and sessions (M4-25 screens where they exist, otherwise the person's profile and logout), the onboarding screens E1 to E7 in phone form (reusing the onboarding controllers of M4-18), the privacy cover of E6 (Design notes 2, 4, 5).
- `apps/client/lib/platform/lifecycle.dart`: the mapping of the app lifecycle to the sync engine states and to the privacy cover (Design note 6).
- Tests under `apps/client/test/phone/` and `apps/client/integration_test/phone/` (Tests).

Change:

- `apps/client/lib` router: phone and desktop layouts chosen by form factor (platform iOS or Android gives the phone layout; desktop platforms the column layout of M4-18), the same paths of 15 11; deep links from the custom scheme on iOS.
- `packages/local_store` (or the app's database opener of M4-12): on iOS set `NSFileProtectionComplete` on the database file and its `-wal` and `-shm` files at creation (Design note 6).
- `apps/client/platform-tests.json`: the iOS simulator run of the integration tests in the `macos` entry (Design note 7).
- `packages/l10n` ARB files: phone texts in de and en.
- `apps/client/README.md` (iOS build, configuration, tests), `docs/spec/15-client-architecture.md` 11 (iPhone only until V3), root `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M5-02-1` to `D-M5-02-7`.

## Out of scope

- Push, the Notification Service Extension and notification taps (M5-04); sharing (M5-06); voice, camera and photo (M5-07); the app lock logic and settings (M5-08); emergency actions (M5-09); terminal screens M8 and M9 (M5-12); the live views M10 (M6-16).
- Android (M5-03), iPad layout (V3), distribution through TestFlight (M7).
- Device E2E against the Compose stack and the mobile performance targets (M5-11).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M5-02-iphone-layout`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(client): ...`, `feat(client-ios): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never sign with a real identity or upload a build.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and make no network call; the fake server of M4-17 replaces the server. Temporary directories only via `mktemp -d` (Dart `Directory.systemTemp.createTemp`), removed at the end. Stop only processes you started, by PID; simulators you booted are shut down by their UDID.
- Never read or print secrets. No network calls except package downloads for pinned tools and pub packages and what the gates of `make check` do themselves.
- The Flutter SDK at the pinned version must be on `PATH`; the iOS parts need macOS with Xcode and an iOS simulator runtime. On another host `make apps-platform` prints its SKIP line and the `apps-macos` CI job is the evidence; the report names where the iOS tests ran.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate. A new pub package needs the license gate green; prefer the system APIs through the Pigeon glue over a plugin with a weak maintenance record.
- The product name appears only at the places of 02 "Code layout"; the app shows `branding.product_name` from the instance.
- No permission logic in the client (15 1 principle 1).
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **iOS runner and build configuration** (ADR 0044; 01 12; Ruling for docs/DECISIONS.md). `flutter create --platforms=ios --org com.example` inside `apps/client`; then: iPhone only (`TARGETED_DEVICE_FAMILY = 1`) until the iPad layout of V3; deployment target iOS 15; the Info.plist gets the URL scheme from the build configuration (`CFBundleURLTypes`), no usage description that this ticket does not need (camera, microphone and photos come with M5-07, notifications with M5-04); the associated domain entitlement only when `branding.yaml` names a domain. The build reads `branding.yaml` through the same script or `dart-define` mechanism M4-18 uses for macOS; without a file it uses the neutral defaults (`com.example.chat`, scheme `examplechat`). Keychain access group and App Group for the extensions are declared by M5-04 and M5-06, not here.

2. **Phone shell and navigation** (01 5.3, 4; 15 11). `lib/phone/` holds a `PhoneShell` with the bottom tab bar (Spaces, Mentions, Running, Me) and one navigator per tab, so each tab keeps its stack; `go_router` routes map the paths of 15 11 into the right tab and stack (a `/t/{id}?m=` link opens the Spaces tab with spaces, channel and thread on the stack, scrolled to the message). The edge swipe back is the platform default of the Cupertino page route on iOS; M5-03 uses the Material route and system back. Screens reuse the controllers and providers of M4-19 to M4-21 unchanged; only layouts differ. Widgets read only providers (ADR 0029).

3. **Login on iOS** (15 8; 08 14.1, 2.5; Ruling for docs/DECISIONS.md). `BrowserLauncher` and `RedirectReceiver` of M4-09: one `ASWebAuthenticationSession` with the callback scheme of the build configuration, `prefersEphemeralWebBrowserSession` off (so the system browser session of the identity provider can be reused, as RFC 8252 intends), presented from the key window; the redirect returns `login_code` to `LoginService`. `PasskeyAuthenticator`: `ASAuthorizationController` with the platform provider for the relying party ID of the instance, only when the build configuration names an associated domain equal to that relying party; otherwise E2 hides the passkey button (01 5.6 E2). No WebView. The Pigeon glue lives in `apps/client/ios/Runner` (rule 4 allows platform code there).

4. **Onboarding in phone form** (01 5.6; R-PR-14). E1 to E7 reuse the onboarding controllers of M4-18 and add phone layouts (full screen, keyboard-aware). E1: an invalid certificate shows the error without any way to continue. E2: the TOTP field has `AutofillHints.oneTimeCode` and `TextInputType.number`; methods only from `getDiscovery`. E3, E5: code fields normalize as 08 3.2; codes rendered in groups with `font.family.mono`, a copy action, and semantics that read character by character. E4: the verification code large in the center, the remaining time as text announced at most once per `a11y.live_announce_min_interval`. E7: the reason text and "The data of this device has been deleted". Goldens of each in light and dark, text scaling 1 and 2.

5. **Compact live box and components** (01 5.3 M3, M4; 6.2, 6.5). The live box row shows state, duration, progress and the stop button at the top right; tapping the row opens the full live box (component of M4-07) as a modal sheet; approval buttons are stacked at full width in the order of 09 11.5; forms open as sheets with the keyboard; selections use the system picker (`CupertinoPicker` on iOS through the adaptive component). No swipe or long press triggers an approval (R-PR-04).

6. **Lifecycle, background and file protection** (15 6.3, 7.2; Ruling for docs/DECISIONS.md). `AppLifecycleState.inactive` shows the privacy cover (a full-screen surface with the branding, no content) before the system takes the app switcher snapshot; `paused` asks the sync engine to suspend (close with `1000`, state `Suspended`); `resumed` removes the cover (unless the app lock of M5-08 is due) and reconnects with `resume`. The database opener sets `NSFileProtectionComplete` on the database file and its companions through a Pigeon call in the runner before the first open; a test on the simulator reads the attribute back.

7. **Tests on iOS** (M4-01 Design note 6). `apps/client/platform-tests.json` `macos` gains a run of `flutter test integration_test/phone -d <iOS simulator>` through a wrapper that picks the first available iPhone simulator (no hard-coded name), next to the desktop runs of M4-18. Widget and golden tests run in `make test` on every host.

8. **Rulings for `docs/DECISIONS.md`** (`D-M5-02-<n>`): iPhone only until V3 (note 1); build configuration keys for iOS (note 1); per-tab navigators and the deep link stack (note 2); `ASWebAuthenticationSession` without an ephemeral session (note 3); passkeys only with a matching associated domain (note 3); the privacy cover on `inactive` (note 6); file protection set before the first open (note 6).

## Tests (write them first)

Widget and golden (`apps/client/test/phone/`, `flutter test`):

- `phone_shell_test.dart`: tab bar with four tabs; each tab keeps its stack; a deep link `/t/{id}?m={mid}` builds the stack Spaces, channel, thread.
- `phone_screens_golden_test.dart`: M1, M2, M3 (running, waiting for input, collapsed), M4 (buttons, form sheet), M7 (offline banner, sending, failed with Retry, Edit, Discard), in light and dark, text scaling 1 and 2, de and en.
- `onboarding_phone_golden_test.dart`: E1 to E7 in light and dark, text scaling 1 and 2; E1 with `setup_required`, an invalid certificate and the minimum version; E2 with only the methods from a fake `getDiscovery`, with and without the passkey button.
- `onboarding_semantics_test.dart`: codes read character by character; remaining time announced at most once per `a11y.live_announce_min_interval` with a fake clock; the TOTP field carries `AutofillHints.oneTimeCode`.
- `text_scaling_test.dart`: every phone screen at text scaling 2 has no overflow (Flutter overflow errors fail the test) and every action stays reachable.
- `tap_target_test.dart`: `iOSTapTargetGuideline` and `labeledTapTargetGuideline` on every phone screen.
- `live_box_phone_test.dart`: the stop button is visible in every live box state; the sheet opens on tap; no gesture on an approval card sends an interaction.
- `privacy_cover_test.dart`: `inactive` shows the cover with no message text in the tree; `resumed` removes it.

Integration (`apps/client/integration_test/phone/`, iOS simulator, fake server of M4-17):

- `login_flow_test.dart`: E1 with the fake discovery, E2 with password and TOTP against the fake server, E4 waiting device confirmed by the fake server, main view; logout leads to E7 and deletes local data (R-SY-14).
- `navigation_flow_test.dart`: open a space, a forum post list, a thread; send a message; the edge swipe back returns; a deep link opens the thread at the message.
- `background_test.dart`: moving to the background closes the gateway with `1000` (fake server sees it) and the foreground resumes; the database file has `NSFileProtectionComplete`.
- `offline_test.dart`: the fake server drops the connection; the banner appears; a message waits with "sending" and is sent after reconnect.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS with Xcode: `make apps-platform` (runs the iOS integration tests on a simulator); elsewhere it prints its SKIP line
- `make codegen-check`, `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make licenses`
- `make check`

## Acceptance criteria

- [ ] Screens M1 to M7 and E1 to E7 exist in phone form with widget, golden and integration tests against the fake server; text scaling up to 200 percent passes; tap targets pass the guidelines.
- [ ] E6 covers the content in the app switcher; E2 offers one-time code autofill; an invalid certificate cannot be bypassed.
- [ ] The iOS runner builds with the neutral build configuration and with a `branding.yaml`; iPhone only; login works with `ASWebAuthenticationSession`, passkeys only with an associated domain.
- [ ] The app suspends the gateway in the background and resumes in the foreground; the database has `NSFileProtectionComplete`.
- [ ] README, 15 11, ARB files, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public Dart symbol and documentation comments on public Swift declarations; comments explain why.
- `apps/client/README.md` updated in English (iOS build, configuration, simulator tests).
- Rulings of Design note 8 recorded in `docs/DECISIONS.md` section 10.
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
4. Where the iOS tests ran (macOS and Xcode versions, simulator runtime and model).
5. What is not proven (for example the system browser login against a real identity provider, passkeys with a real associated domain).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
