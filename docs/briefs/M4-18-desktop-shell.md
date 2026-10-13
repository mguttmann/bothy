# M4-18: Desktop app shell

Status: ready | Depends on: M4-07, M4-11, M4-17 | Milestone: M4

## Goal

After this ticket `apps/client` is a desktop app for macOS and Windows that a person can install from a local build, connect to an instance, log in with, get confirmed on, and log out of: the platform folders `macos/` and `windows/`, the composition of the packages (device keys of M4-10 and M4-11 behind the signer ports of `auth`, secrets and the database key in secure storage, the encrypted `local_store`, the `SyncEngine` of M4-17), Riverpod providers that read only from the database and the engine (15 4), the router with the paths of 15 11 and deep links from outside, the column layout of D1 and D2 with draggable widths stored as `layout.column_widths` and the collapse widths of 01 14, and the onboarding screens E1 to E7 of 01 5.6 as cards in an empty window with the branding of the instance. Logout and revocation delete every local datum of the account (R-SY-14). The columns stay empty frames that M4-19 to M4-25 fill. Why: every desktop feature ticket needs a running app with login, a local database and a connected engine; and the onboarding screens carry the security rules of 08 (no content before login, no exception button on a bad certificate, codes read out character by character, R-PR-14).

## Read first

- `docs/PLAN.md`: section 3; section 10 (Goal, Exit criteria items 2 and 4, Test strategy, Risks; rows M4-18 to M4-25, M4-30, M4-31, M4-34).
- `docs/spec/01-product.md`: 4 "Information architecture", 5.1 D1 and D2 (columns, widths, collapse), 5.6 "Onboarding and sign-in" (rules for all screens, E1 to E7 with their texts and results), 6.6 and 6.7 ("Not found or no access"), 8 (accessibility rules for codes, focus, text scaling), 9, 11 (`layout.column_widths`, `appearance.*`), 12 "Branding per instance", 13 (R-PR-01, R-PR-14), 14 (`ui.desktop_min_width_four_columns`, `ui.desktop_min_width_three_columns`, `a11y.live_announce_min_interval`).
- `docs/spec/15-client-architecture.md`: 1, 2 (macOS 12, Windows 10 2004), 3 (apps), 3.1 (rule 4: platform code only in `device_key` and `apps/client`), 4 "State", 5, 6.3 (key in secure storage; `4012` deletes database, file cache, keys and tokens), 8 (login in the system browser, loopback redirect on desktop), 9 (secure storage per platform), 11 "Deep links and navigation", 15 (`4007` shows "Update required"), 18 (switches from the server), 20 (rows "Widget", "Golden", "Integration").
- `docs/spec/08-identity.md`: 2.2, 2.4, 2.5 (login methods and what E2 offers), 3.2 (enrollment code input rules), 5.1 to 5.4 (registration, approval with the verification code, pairing, no key change), 7.4 and 8 (sessions), 9 (app lock: only the screen here), 10 (lost device), 14.1 and 14.2 (flows).
- `docs/spec/04-sync-protocol.md` 8.5 and 13 (what the client deletes and keeps); `docs/spec/16-operations.md` 3.1 (setup wizard for E1); `docs/spec/18-release.md` (minimum client versions for E1).
- `docs/api/openapi.yaml`: `getDiscovery` (`Discovery`, `AuthProviderInfo`, `PublicBranding`, `ClientVersionRequirement`, `setup_required`), the login and device operations named in `docs/briefs/M4-09-auth-dpop.md`, `getInstance` (`InstanceInfo`, `InstanceLimits`), `getMe`, `getBrandingLogo`.
- `docs/design/components.md` (cards, buttons, inputs, banners) and `docs/design/README.md` (previews of the onboarding screens if present).
- ADRs `0027`, `0029`, `0044-working-name-and-branding-as-configuration.md` (build branding), `0056` (no Flutter engine in extensions; not used on desktop).
- `docs/DECISIONS.md`: D-W-100, D-W-114, D-W-115, D-W-116 (this ticket depends on M4-11 for the device keys; `auth.device_pending_ttl` in `AppConfig`); section 10.
- Briefs: `docs/briefs/M4-17-fake-server.md` (`SyncEngine`, `FakeServer`, `ScenarioClient`, test-only plain HTTP), `docs/briefs/M4-09-auth-dpop.md` (`LoginService`, `AuthSession`, ports, `AuthEvent`), `docs/briefs/M4-10-device-key-apple.md` and `docs/briefs/M4-11-device-key-windows.md` (`DeviceKeyClient`, errors), `docs/briefs/M4-12-local-store.md` (`openLocalStore`, `DatabaseKeySource`, `wipe`), `docs/briefs/M4-07-core-components.md` (components, view models), `docs/briefs/M4-04-design-system-theme.md` (`DsThemeScope`, `resolveDsContext`, `dsGoldenMatrix`), `docs/briefs/M4-06-l10n.md` (ARB rules, `problemText`, `resolveAppLocale`), `docs/briefs/M4-01-workspace-arch.md` (`platform-tests.json`, CI jobs).

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
9. Limits only through named keys: `ui.desktop_min_width_four_columns`, `ui.desktop_min_width_three_columns`, `a11y.live_announce_min_interval` and `auth.device_pending_ttl` (D-W-116) from `AppConfig`; the pairing time from `expires_at` of `beginPairing`; no literal duration or width in code.
10. Specification first: 01 5.6 and 15 11 do not say how a desktop build gets its URL scheme and app name before `branding.yaml` exists (ADR 0044, distribution in M7); this ticket follows Design note 7 and states it in 15 11.

Product requirements implemented here:

11. Before login only name, logo and accent from `getDiscovery` (`branding`); no content of the instance (01 5.6).
12. E1: "Connect" calls `getDiscovery`; results "No instance at this address", "Certificate invalid" (no continue, no exception button), "Instance not set up yet" on `setup_required`, "App too old" below `min_client_versions`; the app remembers the address (01 5.6, R-PR-14).
13. E2 offers exactly the methods `getDiscovery` names (OIDC providers, proxy header, local login name with passkey where offered and password with one-time code if `local_password_enabled`, always "I have an enrollment code" and "Pair with another device"); without a method "No login is set up for this instance".
14. Codes in groups, mono font, copyable, read character by character by screen readers; input ignores case, spaces and hyphens; remaining times as text announced at most once per `a11y.live_announce_min_interval` (01 5.6).
15. E4 shows the verification code of the waiting device and polls; confirmation leads to the main view, rejection or expiry to E7; "Cancel" logs out and deletes the keys (01 5.6, 08 5.2).
16. E5 shows the pairing code with its remaining time and "New code"; after the approval on another device the new device is active (08 5.3).
17. Logout, revocation (`4012`, `session.revoked`, `401 session_revoked`) and a rejected device delete the local database, the file cache, the keys and the tokens, and show E7 with the reason and "The data of this device has been deleted" (R-SY-14, 01 5.6).
18. OIDC runs in the system browser with a loopback redirect on `127.0.0.1` with a free port, never in a web view (15 8, RFC 8252).
19. The router has one path per target (15 11); a link to something invisible shows "Not found or no access" (01 6.7).
20. Widgets read only from providers over the local database and the engine (R-CL-03); the app shows `branding.product_name`, never a literal name.

## Scope

Create or fill in `apps/client/`:

- Platform folders from `flutter create --platforms=macos,windows --org com.example` in `apps/client` (only these two platforms), with the macOS entitlements for the keychain access group and outgoing network, the macOS URL type and the Windows protocol registration for the deep link scheme of Design note 7; `platform-tests.json` with `macos` and `windows` entries that run `flutter test integration_test -d <platform>` (Design note 9).
- `lib/main.dart` and `integration_test/support/test_app.dart` (the test entry that allows plain HTTP for the fake server, M4-17 Design note 8; it lives in a test folder, so no release build contains it), `lib/app/` (`app.dart`, `bootstrap.dart`: composition root of the app, Design note 1), `lib/app/providers/` (Riverpod providers with code generation: engine, auth, store, instance, settings, connection state), `lib/app/router.dart` (Design note 4), `lib/app/deep_links.dart`, `lib/app/layout/` (`desktop_shell.dart`, `column_layout.dart`, `column_widths.dart`, Design note 5), `lib/onboarding/` (one file per screen E1 to E7 plus `onboarding_controller.dart`, `code_text.dart` for grouped codes, Design note 6), `lib/platform/` (`secure_store.dart` on `flutter_secure_storage`, `device_key_signer.dart` adapters to the `auth` ports, `loopback_redirect.dart`, `browser_launcher.dart` on `url_launcher`, `network_status.dart`, `app_lifecycle.dart`, `frame_ticker.dart`), `lib/app/branding.dart` (build branding, Design note 7).
- `branding.example.yaml` at `apps/client/` (the example of ADR 0044 with `example.com` values) and `tool/branding_defines.dart` that turns it into `--dart-define-from-file` input (Design note 7).
- Tests: `test/onboarding/` (widget tests), `test/goldens/onboarding/` (golden files), `test/app/` (router, column widths, providers), `integration_test/` (Tests).
- Fake server additions in `packages/sync_engine/lib/src/testing/` through the handler registry and world controls of M4-17, only where a flow of this ticket needs one that M4-17 lacks.
- ARB keys in `packages/l10n/lib/l10n/app_*.arb` for every text of E1 to E7, the column frames and "Not found or no access" (where M4-06 has not added them yet).

Change:

- `apps/client/pubspec.yaml` (members `sync_engine`, `auth`, `local_store`, `domain`, `api_client`, `device_key`, `design_system`, `l10n`, `admin_ui` only if used; `flutter_riverpod`, `riverpod_annotation`, `go_router`, `flutter_secure_storage`, `url_launcher`, `connectivity_plus` or the network status source Design note 1 chooses; dev: `riverpod_generator`, `build_runner`, `integration_test`, `alchemist`), `apps/client/README.md` (how to build and run on macOS and Windows, the test entry, branding defines).
- Root `Makefile` `codegen-check`: the Riverpod generation of `apps/client` compared like the other generated Dart code (M4-01).
- `docs/spec/15-client-architecture.md` 11 (desktop deep link registration and the build branding of Design note 7), 9 (where the app keeps the database key and tokens on macOS and Windows).
- `docs/DECISIONS.md` section 10: rows `D-M4-18-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Sidebar content, post list, thread view, session column content (M4-19 to M4-21); search and quick switcher (M4-23); shortcuts beyond Enter and Esc on the onboarding cards (M4-24); settings, devices and sessions (M4-25), including confirming another device with its code (the active-device side of E4 and E5).
- The app lock logic (M5-08): this ticket builds the E6 screen with its states and goldens only.
- iOS and Android folders (M5), web (console, M6).
- Notification area, local notifications, MSIX packaging (M4-30, M7); auto-update and signing (M7); `branding.yaml` processing for icons, bundle IDs and feeds (M7).
- Client metrics and crash reports (M4-31).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-18-desktop-shell`; one writer per worktree.
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
- Integration tests run on a desktop (`flutter test integration_test -d macos` or `-d windows`); on a host without a desktop session `make apps-platform` prints its SKIP line and the `apps-macos` and `apps-windows` CI jobs are the evidence. The report names where they ran.
- Tests never touch the login keychain or the credential store of the build host outside a test-only service name that the test deletes in teardown; the integration test entry uses an in-memory secret store and an in-memory key source unless a test is about the real store.
- The fake server of M4-17 is the only server in tests; no test calls a real instance.

## Design notes

1. **Composition root.** `bootstrap.dart` builds, per server address: the `DeviceKeyClient` (M4-10, M4-11) with an adapter per purpose that implements `DeviceKeySigner` of M4-09 (scope = the server's `instance_id` from discovery; `alg` `ES256`; JWK and thumbprint from `DeviceKeyInfo`); the `SecretStore` on `flutter_secure_storage` (macOS: Data Protection keychain, `synchronizable: false`, accessibility `first_unlock_this_device`; Windows: the package's default) with keys prefixed by the instance ID; the `DatabaseKeySource` that creates 32 random bytes with `Random.secure()` at the first start and stores them in the secret store; `openLocalStore` with a file under the application support folder (`path_provider`), one file per instance ID; the `AuthSession` and `LoginService`; and the `SyncEngine` of M4-17 with the platform ports of `lib/platform/`. Riverpod overrides replace every port in tests (15 4). Network status: use `connectivity_plus` (publisher fluttercommunity.dev, BSD-3-Clause) only if `make licenses` and `make osv` stay green; otherwise derive "network changed" from failed connects and `AppLifecycle` resume, and record which.

2. **Sign-out and wipe.** One `AccountWiper.wipe(reason)` used by logout, E4 "Cancel", a rejected or expired device, `4012`, `session.revoked` and `401 session_revoked`: stop the engine, `LocalStore.wipe()`, delete the file cache folder of the instance, delete both device keys (`DeviceKeyClient.delete(scope)`), delete the secrets of the instance (tokens, database key, device ID), keep only the server address (01 5.6: the app remembers the address), then show E7 with the reason text (`devices.revoke_reason` or the session reason mapped through `l10n`, never an internal name). A test asserts that after a wipe no file of the instance remains and the secret store has no key of the instance.

3. **Onboarding flow.** `OnboardingController` (a Riverpod `Notifier`) drives E1 to E7 from the `LoginService` outcomes of M4-09: E1 discovery result to E2; E2 method to the flow; `pending` to E4 with polling (`pollPendingDevice`) and the remaining time computed from the device's `created_at` plus `auth.device_pending_ttl` from `AppConfig` (D-W-116: the `Device` schema names no expiry; the value is display only, the server decides the expiry and the poll reports it); `signedIn` to the main view; `signedOut` to E7. E3: the code field, then the methods the instance offers for the redeemed code (passkey, password with one-time code including the QR step, link with a login service), redeemed codes make the device active at once. E5: `beginPairing` shows the code and the remaining time, "New code" starts a new pairing, the poll ends in the main view or, on expiry, back to E5 with the expiry text. E6: the screen with the unlock button, the failure state and "Log out" (wired to the wiper); no timer (M5-08). E1 certificate errors: a `HandshakeException` or `TlsException` from the discovery request shows "Certificate invalid" with no way to continue; there is no setting to trust a certificate. "App too old": compare the app version (from the build) with the `min_client_versions` entry of the platform (`macos` or `windows`) by semantic version.

4. **Router and deep links.** `go_router` with the paths of 15 11 (`/s/:spaceId`, `/c/:channelId`, `/t/:threadId?m=`, `/running`, `/mentions`, `/search?q=`, `/admin/...`, plus `/onboarding/...` for E1 to E7 and `/settings/...`); every content route requires a signed-in state and redirects to onboarding otherwise. A target that is not in the local database and that REST answers with `404` shows "Not found or no access" (01 6.7, the same text for "does not exist" and "no access"). Deep links from outside: `<scheme>://t/<thread_id>?m=<message_id>` (and `s`, `c`) arrive through the platform (macOS URL type through the app delegate, Windows protocol activation through the runner's command line forwarded over a method channel) and go to the router; a link that arrives before login is kept and opened after the login; a link to a thread that is not loaded waits until it is loaded locally or through REST, then jumps to the message (15 11). The route parser is a pure function with unit tests.

5. **Columns.** `ColumnLayout` with the columns `sidebar`, `post_list`, `history`, `session` (names of 01 11 `layout.column_widths`): drag handles between columns, widths saved through the personal setting `layout.column_widths` (an outbox write of `putMySetting` through the executor of M4-15) and read back from `user_settings`; below `ui.desktop_min_width_four_columns` the session column collapses, below `ui.desktop_min_width_three_columns` the post list becomes a stack; "Reset" restores the defaults. Each column is an empty frame with a test key that later tickets fill. The connection state of the engine appears as the offline banner of `DsBanner` (01 6.6) at the top of the content.

6. **Codes and remaining times.** `CodeText(String code, {int groupSize = 4})` (groups of four, as 01 5.6 and 24 4.3 show them) renders groups in `font.family.mono`, offers copy, and sets a semantics label that spells the code character by character (letters and digits separated so screen readers read each one, for example "4, 8, 1, 2, 0, 7"); code input fields accept paste and normalize like M4-09. Remaining times are a text ("Valid for 23 more minutes") from `expires_at` (pairing) or from `created_at` plus `auth.device_pending_ttl` (waiting device) and an injected clock; a `LiveAnnouncer` (M4-07) announces changes at most once per `a11y.live_announce_min_interval`.

7. **Build branding.** Until the distribution of M7 reads `branding.yaml`, the desktop build takes `app_name`, `url_scheme`, bundle identifier and default server from `--dart-define-from-file` generated by `tool/branding_defines.dart` from `apps/client/branding.example.yaml` (neutral values under `example.com`, a neutral working app name that is not the product name, scheme `chat-example`); the macOS `Info.plist` and the Windows runner read the same file through the Flutter build variables. The interface shows `branding.product_name` from the instance after discovery and the build app name only where the system needs one (window title before discovery). Ruling for docs/DECISIONS.md and a sentence in 15 11.

8. **Goldens.** E1 to E7 as cards (desktop form of 01 5.6) through `dsGoldenMatrix` in light and dark, text scaling 1 and 2, `de` and `en`, density `comfortable`; E1 in its states (empty, connecting, no instance, certificate invalid, not set up, app too old), E2 with three discovery fixtures (OIDC only; local with passkey and password; nothing), E3, E4, E5, E6, E7 (revoked by another device; disabled person without the login button).

9. **Integration tests.** `integration_test/` runs the app from `integration_test/support/test_app.dart` against `FakeServer` (M4-17) on loopback and implements `ScenarioClient` of M4-17 over the app's engine providers, so the five scenarios of 04 16 run in the app process. `platform-tests.json` lists `["flutter", "test", "integration_test", "-d", "macos"]` and the Windows equivalent; the CI jobs of M4-01 run them.

10. **Rulings for docs/DECISIONS.md** (`D-M4-18-<n>`): composition per server address and the secret key names (note 1); one wiper for every sign-out cause (note 2); certificate errors without an exception path (note 3); route set and the deep link wait (note 4); column names and the setting (note 5); code semantics (note 6); build branding before M7 (note 7); network status source (note 1).

## Tests (write them first)

Widget and golden tests (`flutter test`, fake `LoginService`, fake discovery, fake clock):

- `test/onboarding/e1_test.dart`: each discovery result leads to its text; a TLS error offers no continue; `setup_required` shows the setup pointer; an app version below `min_client_versions` shows "App too old"; the address is remembered.
- `test/onboarding/e2_test.dart`: the buttons equal the methods of the discovery fixture, nothing more; "No login is set up" without methods; Enter triggers the main action and Esc goes back.
- `test/onboarding/codes_test.dart`: a code is grouped in the mono font, copyable, and its semantics label spells it character by character; pasted input with spaces, hyphens and lower case is normalized.
- `test/onboarding/e4_e5_test.dart`: remaining time as text (E4 from `created_at` plus `auth.device_pending_ttl`, E5 from `expires_at`), announced at most once per interval; "Cancel" calls the wiper.
- `test/onboarding/e7_test.dart`: reason texts per revoke reason; no login button for a disabled person.
- `test/goldens/onboarding/*`: Design note 8.
- `test/app/router_test.dart`: route parsing of every path of 15 11 and of external links; an unknown target shows "Not found or no access"; a link before login is opened after it.
- `test/app/column_widths_test.dart`: widths are written to `layout.column_widths` and restored; collapse at the two widths from `AppConfig`.
- `test/app/wiper_test.dart`: after `wipe` no file of the instance and no secret of the instance remains; the server address stays.

Integration tests (`integration_test/`, against `FakeServer`, run on macOS and Windows):

- `login_pending_confirm_test.dart`: login with password and one-time code, the device is pending (E4 shows the code), the fake approves it from a second simulated device, the main view appears with the sidebar frame.
- `pending_rejected_expired_test.dart`: rejection (wrong code on the other device) and expiry lead to E7 and a wiped store.
- `pairing_test.dart`: E5 pairing with confirmation, with rejection and with expiry and "New code".
- `logout_wipes_test.dart`: logout deletes database, file cache, keys and tokens (R-SY-14); `4012` from the fake does the same.
- `scenarios_test.dart`: the five scenarios of M4-17 through the app's engine.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS and on Windows: `make apps-platform` (runs the integration tests); elsewhere it prints its SKIP line
- `make codegen-check`, `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make licenses`, `make osv`
- `make check`

## Acceptance criteria

- [ ] Integration test: login, waiting device, confirmation, and logout deletes local data (R-SY-14), green on macOS and Windows in the `apps-macos` and `apps-windows` jobs.
- [ ] Widget and golden tests of E1 to E7 in light and dark with text scaling 1 and 2; E1 with `setup_required`, an invalid certificate and the minimum version; E2 shows only the methods from `getDiscovery`; codes are read out character by character.
- [ ] E4 and E5 with confirmation, rejection and expiry against the fake server.
- [ ] Router with the paths of 15 11, deep links from outside on both platforms, "Not found or no access" for invisible targets.
- [ ] Column layout with stored widths and the collapse widths of 01 14; widgets read only from providers.
- [ ] 15 9 and 11 updated; rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 01, 08 or 15.
- `apps/client/README.md` (English) as in Scope.
- 15 updated; rulings of Design note 10 in `docs/DECISIONS.md` section 10.
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
4. Where the integration tests ran (macOS and Windows versions, CI or a named class of machine), the run time of the integration suite, the network status source chosen, and how a deep link was checked on each platform.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
