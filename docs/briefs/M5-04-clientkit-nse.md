# M5-04: ClientKit and Notification Service Extension (ADR 0056)

Status: ready | Depends on: M3-22, M4-31, M5-02 | Milestone: M5

## Goal

After this ticket iPhone and Mac receive pushes without content and show the text when the server is reachable (13 5.1, 6.1, 6.4; ADR 0031, ADR 0056). `ClientKit`, a Swift package in `apps/client`, holds what the native extensions share: access to the keychain group shared between the app and its extensions, DPoP proofs signed with the `dpop` device key of `DeviceKeyCore` (M4-10) with nonce adoption, the preview fetch (`GET /api/v1/push/preview` with the preview token, `getPushPreview`), and the small shared container state (preview mode, counters). A Notification Service Extension for iOS and for macOS, written in Swift without the Flutter engine, takes each push with `mutable-content`, reads the IDs from `b`, and within `client.push.fetch_timeout` replaces title and text with the names and excerpt the server returns for the person's `notify.preview_mode`; when the server is not reachable or the time runs out the localized placeholder stays; on `404` it shows "Message not available"; with `notify.preview_mode = none` it fetches nothing. It never refreshes a token and never touches a refresh family: it uses only the preview token of the push registration (13 5.1, 6.4; 08 7.4). The app requests the notification permission, registers the APNs token with `registerPush` (provider `apns`, environment, app ID), stores the returned preview token and registration ID in the shared keychain group, re-registers when the token changes, opens the thread at the message when a notification is tapped, and forwards the extension's display counters as client metrics. A build check proves that the extension targets link no Flutter framework. Why: the push carries only IDs (R-Push), and only a native extension can fetch the text within the system's limits (R-CL-06, R-CL-07).

## Read first

- `docs/adr/0056-native-extensions-without-the-flutter-engine.md` (all of it; "Confirmation" names this ticket).
- `docs/spec/13-push-notifications.md`: 1 (goals), 2 (kinds), 3.3 (`notify.preview_mode`), 3.4 (quiet hours: `interruption-level` `passive`), 5.1 (APNs, NSE, no token renewal in the NSE, device key, suppressing without the filtering entitlement: show "Message not available"), 6.1 (payload: `aps`, `b` with `v`, `nid`, `k`, `i`, `t`, `m`; `loc-key` per kind; `thread-id`; `badge`), 6.4 (preview endpoint and token, the field table per mode, `404`), 8 (badge-only push for sync), 9 (t2 is the preview fetch), 10, 13 (`push.register`: `provider`, `token`, `environment`, `app_id`; a known token is taken over), 14 (rows "Payload allow-list", "Revocation between event and sending", "`auth.logout`, `device.revoke`, `user.disable`").
- `docs/spec/15-client-architecture.md`: 1 principle 4, 3 (`ios/`, `macos/` hold Runner, NotificationService, ShareExtension, ClientKit), 8 (refresh rules: the NSE never refreshes, R-CL-07), 9 (row iOS and macOS: the DPoP key in the access group with the extensions, `AfterFirstUnlockThisDeviceOnly`; tokens for the extensions in the shared access group), 10, 10.1 (the sequence diagram), 11 (deep links), 17 (client metrics `client.push_display.duration`, `client.push.placeholder`), 21 (R-CL-06, R-CL-07), 22 (`client.push.fetch_timeout`).
- `docs/spec/08-identity.md`: 6.1, 7.3 (DPoP: `htu`, `ath`, nonce), 7.4 (extension sessions; the NSE has no session).
- `docs/spec/01-product.md`: 10 (10.1 levels, 10.2 content, 10.3 row iOS and macOS).
- `docs/api/openapi.yaml`: `registerPush`, `unregisterPush`, `getPushPreview`, schemas `PushRegistrationCreate`, `PushRegistration`, `PushPreview`; `submitClientMetrics`.
- `docs/observability/signals.md` 4.6a (`client.push_display.duration`, `client.push.placeholder`).
- `docs/DECISIONS.md`: D-PN-01, D-W-100 and its extension to M5 and M6 (D-W-120), D-W-115; section 10 (format of `D-M5-04-<n>` rows) and the rows of M3-22, M4-10, M5-02.
- Briefs (merged names win, D-W-100): `M3-22-push-registration.md` (preview token binding to registration, session and device key; `404` rules; the response per mode), `M3-20-apns-adapter.md` (the exact APNs payload), `M4-10-device-key-apple.md` (`DeviceKeyCore`, the shared access group passed at registration), `M4-09-auth-dpop.md` (the Dart DPoP profile that `ClientKit` must match byte for byte in form), `M5-02-iphone-layout.md` (iOS runner, build configuration, router), the brief of M4-31 (client metrics in the app).
- Code on the base branch: `apps/client/ios`, `apps/client/macos`, `packages/device_key/darwin/DeviceKeyCore`, `packages/auth`, `apps/client/lib`.

This ticket touches no table, no action, no event and no server configuration key.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Placeholder and notification texts come from `packages/l10n` (generated into the native string files, Design note 6), never as literals.
9. Limits and timeouts only through named configuration values (`client.push.fetch_timeout` from `AppConfig`, D-W-115); code comments name the key, never the number.
10. Specification first: a change to behaviour updates `docs/spec/` in the same change (for example 01 10.2, Design note 4). The core contract is not edited by this ticket.

Product requirements implemented here:

11. The NSE and `ClientKit` contain no Flutter framework and start no Flutter engine; a build check fails when an extension target links one (ADR 0056 "Confirmation").
12. The NSE never uses a refresh token and never starts a session; it authenticates only with the preview token and a DPoP proof of the `dpop` device key (13 5.1, 6.4; 08 7.4; R-CL-07).
13. With `notify.preview_mode = none` the NSE makes no request; with `full` or `names` it shows exactly the fields the response carries; on `404` "Message not available"; on no network or after `client.push.fetch_timeout` the placeholder stays (13 5.1, 6.4; 15 10.1).
14. The shared container and the shared keychain hold no message content: only the preview token, the registration ID, the server address, the preview mode and display counters; tokens and keys are `ThisDeviceOnly` and never synchronized (15 9).
15. Grouping per thread (`thread-id`), tapping opens the thread at the message (01 10.3; 15 11); the badge comes from the payload (13 8).
16. The app registers the push token after the person allowed notifications, re-registers on a new token, and unregisters on logout (the server also disables registrations of the session, 13 6.4); the preview token is stored only in the keychain.

## Scope

Create:

- `apps/client/ios/ClientKit/` as a Swift package used by the iOS and macOS targets (`Package.swift` with platforms iOS 15 and macOS 12; `Sources/ClientKit/`: `SharedKeychain`, `SharedState` (App Group container values), `DPoPProof` (ES256 over `DeviceKeyCore`), `NonceCache`, `PreviewClient`, `PreviewRenderer` (title and body from a response and the kind), `DisplayCounters`; `Tests/ClientKitTests/`), depending on `DeviceKeyCore` of `packages/device_key/darwin/DeviceKeyCore` by a local package reference (Design notes 1 to 3).
- `apps/client/ios/NotificationService/` (iOS NSE target) and `apps/client/macos/NotificationService/` (macOS NSE target), each a thin `UNNotificationServiceExtension` subclass over `ClientKit` (Design note 4).
- `apps/client/lib/platform/push_apple.dart`: permission request, token handling, `registerPush`, storage of the preview token and registration ID through a Pigeon call into the runner (never in Dart storage), re-registration, unregistration on logout, notification tap routing, forwarding of display counters to the client metrics of M4-31 (Design note 5).
- Pigeon interface `apps/client/pigeons/push_apple.dart` and its generated code; runner glue in `apps/client/ios/Runner/` and `apps/client/macos/Runner/`.
- Native localized strings for the placeholder keys `PUSH_TITLE`, `PUSH_BODY_MESSAGE`, `PUSH_BODY_MENTION`, `PUSH_BODY_DM`, `PUSH_BODY_RUN_DONE`, `PUSH_BODY_RUN_FAILED`, `PUSH_BODY_INPUT_NEEDED`, `PUSH_BODY_SECURITY`, `PUSH_BODY_ADMIN` and "Message not available", generated from the ARB files of `packages/l10n` by `tools/l10n/apple_strings.py` (or a Dart tool if M4-06 chose Dart for tools) with a check in `make codegen-check` (Design note 6).
- `tools/ci/check_extension_links.py` and its test (Design note 7).
- Tests (Tests).

Change:

- Xcode projects of `apps/client/ios` and `apps/client/macos`: the NSE targets, the App Group and keychain access group entitlements (names from the build configuration of M5-02 and M4-18, neutral defaults under `com.example`), the `aps-environment` entitlement, `UIBackgroundModes` only as far as remote notifications need (no background fetch).
- `packages/device_key` (M4-10): only if `DeviceKeyCore` lacks a way to open the `dpop` key from an extension with the shared access group; then add it there with a test.
- `apps/client/platform-tests.json`: `swift test` of `ClientKit`, the NSE simulator test and the link check in the `macos` entry.
- `packages/l10n` ARB files: the placeholder texts in de and en.
- Docs: `apps/client/README.md` (push on Apple platforms, entitlements, shared container, how to run the NSE test), `docs/spec/15-client-architecture.md` 10.1 (what the shared container holds; the nonce rule; the counters), `docs/spec/01-product.md` 10.2 (with `none` the NSE shows the placeholder without a channel name, Design note 4), root `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M5-04-1` to `D-M5-04-8`.

## Out of scope

- The Share Extension (M5-06), which also uses `ClientKit`; this ticket builds only the parts the NSE needs, in a form the share extension can extend.
- Android push (M5-05); Windows notifications (M4-30).
- Notification actions (reply, mark read) and the `com.apple.developer.usernotifications.filtering` entitlement (a decision of the instance operator, 13 5.1).
- Device E2E against the Compose stack (M5-11).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M5-04-clientkit-nse`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(client-ios): ...`, `feat(clientkit): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never sign with a real identity, never use a real APNs key and never send a real push.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and make no network call; a local test server (`URLProtocol` stub in XCTest, the fake server of M4-17 in integration tests) replaces the server; pushes in the simulator are delivered with `xcrun simctl push` from a payload file. Temporary directories only via `mktemp -d`, removed at the end. Stop only processes you started, by PID; simulators by their UDID.
- Never read or print secrets; never log a preview token, an APNs token, a DPoP proof or a preview response. No network calls except package downloads for pinned tools and packages.
- The native tests need macOS with Xcode and an iOS simulator runtime; on another host `make apps-platform` prints its SKIP line and the `apps-macos` CI job is the evidence; the report names where they ran.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- No permission logic in the client (15 1 principle 1).
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **`ClientKit` layout** (ADR 0056 item 5; 15 9; Ruling for docs/DECISIONS.md). Foundation, Security and CryptoKit only, plus `DeviceKeyCore`. `SharedKeychain`: generic password items in the access group of the build configuration, `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly`, `kSecAttrSynchronizable = false`, keyed by server scope (the same scope hash as the device keys): `preview_token`, `registration_id`, `server_url`. `SharedState`: `UserDefaults(suiteName:)` of the App Group with only `preview_mode` (`full`, `names`, `none`), the instance ID, and the display counters; no content, no names, no IDs of threads or people. Every value written by the app, read by the NSE; the NSE writes only counters.

2. **DPoP in Swift** (08 7.3; M4-09 Design note 2; Ruling for docs/DECISIONS.md). `DPoPProof.make(method:url:accessToken:nonce:)` builds the same compact JWS as the Dart `DpopProofFactory`: header members `typ`, `alg`, `jwk` in this order, payload `jti` (UUIDv7 string), `htm`, `htu` (normalized: lowercase scheme and host, default port dropped, no query or fragment), `iat`, `ath` (base64url SHA-256 of the preview token), `nonce` when known; JSON without whitespace; ES256 `R || S` from `DeviceKeyCore`. A contract test computes a proof in Swift and in Dart for the same inputs with a fixed key, clock and `jti` and compares the signing input byte for byte (the signature differs because ECDSA is randomized; verify both with the public key). `NonceCache` keeps the last `DPoP-Nonce` per server in the App Group (a nonce is not secret and expires by itself, 08 7.3); on `401` with `use_dpop_nonce` the client repeats once with the new nonce.

3. **Preview fetch** (13 6.4; 15 10.1). `PreviewClient.fetch(nid:messageID:)` calls `GET /api/v1/push/preview?nid=&message_id=` on the server address from the keychain with `Authorization: DPoP <preview token>` and the proof, `URLSession` with an ephemeral configuration, no cookies, no cache, timeout `client.push.fetch_timeout` (value from the app configuration written into the App Group by the app at start, because the extension has no `AppConfig`; the key name stays the source). Results: `content(PushPreview)`, `notAvailable` (`404`), `failed` (any other status, a network error, a timeout). A response is decoded strictly against the members of 13 6.4; unknown members are ignored (G5 tolerance), missing required ones are `failed`.

4. **The NSE** (13 5.1, 6.1; ADR 0056 item 1; Ruling for docs/DECISIONS.md). `didReceive`: parse `b` (`v` must be `1`, otherwise leave the content unchanged); keep `thread-id`, badge and sound from `aps`; if `SharedState.preview_mode` is `none` or the keychain has no preview token for the instance `i`, deliver the placeholder at once and count `placeholder`; else fetch: `content` sets title to "<channel> · <thread title>" and body to "<sender>: <excerpt>" according to the members present (`names` mode: no excerpt; the format strings come from the localized strings), `notAvailable` sets the localized "Message not available", `failed` keeps the placeholder; count the duration from receipt to delivery and whether a placeholder stayed. `serviceExtensionTimeWillExpire` delivers the best content so far (the placeholder). The NSE never shows a channel name with `none`, because it cannot read the encrypted database (15 6.3) and a list of names in the shared container would be content outside it; update 01 10.2 accordingly. The image of `image_url` is not attached in this ticket (an attachment needs a second fetch within the budget; record it as a follow-up with the reason).

5. **App side** (13 13, 6.4; 15 10, 11; Ruling for docs/DECISIONS.md). Permission: requested after login at the first moment the person can understand it (after the first thread opened or from settings; never before login), options alert, sound, badge; without permission no registration. Token: from `didRegisterForRemoteNotificationsWithDeviceToken` through Pigeon; environment from the build (`aps-environment` entitlement: development or production); `app_id` the bundle ID. `registerPush` returns the registration and the preview token once; the runner stores both in the shared keychain immediately (Dart never persists them). On a new token register again (the server takes the old one over); on logout call `unregisterPush` best effort and delete the keychain items; on `4012` or revocation delete them as well. Tap: `userNotificationCenter(_:didReceive:)` passes `t` and `m` to the router (`/t/{t}?m={m}`), which waits until the thread is loaded (15 11). The app writes `preview_mode` to the shared container whenever `notify.preview_mode` changes in the synced settings and at start. At start the app reads and resets the display counters and submits them as `client.push_display.duration` and `client.push.placeholder` through the client metrics of M4-31 (attributes only platform and app version).

6. **Localized strings** (13 6.1; 01 9). The placeholder keys exist in the app's `Localizable.strings` (iOS and macOS) in de and en, generated from the ARB files so the texts have one source; the generator check compares the generated files with a fresh run. The NSE target includes the same strings.

7. **Link check** (ADR 0056 "Confirmation"; Ruling for docs/DECISIONS.md). `tools/ci/check_extension_links.py` builds or inspects the built NSE products (`otool -L` on the binary of each `.appex`) and fails when any of them links `Flutter.framework`, `FlutterMacOS.framework` or `App.framework`; it runs in the `macos` platform tests. A unit test feeds it recorded `otool` outputs with and without such a line.

8. **Rulings for `docs/DECISIONS.md`** (`D-M5-04-<n>`): `ClientKit` contents and the shared container (note 1); DPoP parity with Dart and the nonce cache (note 2); the timeout source for the extension (note 3); NSE content rules including `none` without names and no image yet (note 4); permission timing (note 5); preview token only in the keychain (note 5); generated native strings (note 6); the link check (note 7).

## Tests (write them first)

Swift, `ClientKit` (XCTest, `swift test`):

- `testSharedKeychainThisDeviceOnly` (attributes, no synchronizable flag; in-memory store and one real round trip skipped without the entitlement).
- `testProofMatchesDartProfile` (the contract vector of Design note 2; signature verifies with the public key).
- `testHtuNormalization`, `testAthIsHashOfPreviewToken`, `testNonceRetryOnce`.
- `testPreviewFullNamesNone` (stubbed responses per mode render the expected title and body; `none` never calls the stub), `testNotAvailableOn404`, `testPlaceholderOnTimeoutAndNetworkError` (the stub delays beyond the configured timeout), `testUnknownMembersIgnoredMissingRequiredFails`.
- `testNoContentInSharedState` (after a fetch, the App Group holds only the allowed keys; no excerpt, no name).

Swift, NSE on the simulator (XCUITest or patrol driven, in `platform-tests.json`):

- If `xcrun simctl push` does not run the extension on the simulator runtime at hand, run these four tests against an XCTest host that instantiates the extension's `NotificationService` and calls `didReceive` with the same payload; record which path ran.
- `testServerReachableShowsText`: the fake server answers the preview; `xcrun simctl push` delivers a payload of 13 6.1; the delivered notification shows "<channel> · <title>" and "<sender>: <excerpt>".
- `testServerUnreachableShowsPlaceholder`: the fake server is stopped; the placeholder stays.
- `testPreviewModeNoneFetchesNothing`: with `preview_mode = none` the fake server records no request.
- `testNseNeverRefreshes`: during all NSE tests the fake server records no call to the refresh, login or session endpoints.

Dart (`apps/client/test/`): `push_apple_test.dart` with a fake Pigeon host: registration only after permission, re-registration on a new token, unregistration and keychain deletion on logout, tap routing to `/t/{t}?m={m}`, `preview_mode` written on a settings change, counters submitted as client metrics.

Python: `tools/ci/tests/test_check_extension_links.py`; the generator check test for the native strings.

Commands that must pass, from the repository root:

- `cd apps/client/ios/ClientKit && swift test` (on macOS)
- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS with Xcode: `make apps-platform` (ClientKit tests, the NSE simulator tests, the link check); elsewhere it prints its SKIP line
- `make codegen-check`, `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] XCTest of `ClientKit` is green; the DPoP proof matches the Dart profile.
- [ ] Simulator test: text appears when the server is reachable, otherwise the placeholder; with `notify.preview_mode = none` nothing is fetched; the extension uses only the preview token and never refreshes (13 6.4).
- [ ] The NSE targets for iOS and macOS link no Flutter framework (link check green).
- [ ] The app registers, re-registers and unregisters the push token; the preview token lives only in the shared keychain; a tap opens the thread at the message.
- [ ] README, 15 10.1, 01 10.2, ARB files, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; documentation comments on every public Swift declaration and doc comments on every public Dart symbol; comments explain why.
- `apps/client/README.md` updated in English.
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
4. Where the native tests ran (macOS, Xcode, simulator runtime), the measured NSE duration with a reachable fake server.
5. What is not proven (real APNs delivery, a real device behind a VPN, macOS NSE on a quit app; M5-11 covers devices against the Compose stack).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
