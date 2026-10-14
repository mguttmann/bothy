# M5-05: Android push service without the Flutter engine

Status: ready | Depends on: M5-03, M5-04 | Milestone: M5

## Goal

After this ticket the Android app receives FCM data messages that carry only IDs and shows them at once, then with text (13 5.2, 6.2; 15 10.2; ADR 0056 item 3): a `FirebaseMessagingService` in Kotlin, without a Flutter engine, posts a notification with the localized placeholder immediately into the channel of the kind (the mapping of M5-03), tagged with the thread ID so a later one replaces it, and starts an expedited WorkManager job that fetches the preview from `GET /api/v1/push/preview` with the preview token and a DPoP proof of the `dpop` device key (the Kotlin core of M5-01) within `client.push.fetch_timeout` and replaces the same notification with "<channel> · <thread title>" and "<sender>: <excerpt>" as the person's `notify.preview_mode` allows; without network or after the timeout the placeholder stays; on `404` "Message not available"; with `notify.preview_mode = none` nothing is fetched. Quiet notifications post silently. The message `k = "sync"` removes the notifications of the cleared threads and updates the badge (13 8). The service passes the FCM token to Dart through Pigeon; Dart registers it with `registerPush` (provider `fcm`) after the notification permission is granted (runtime permission from API 33), and the Kotlin side stores the preview token in its own Keystore-encrypted store. Tapping opens the thread at the message. Without a Firebase project in the build configuration the app builds and runs without push (15 10.2). An instrumented test drives the service with a fake FCM payload against a fake server, and without network the placeholder stays. Why: push carries only IDs (R-Push), and starting a Flutter engine in the background would cost the few seconds FCM gives (15 10.2).

## Read first

- `docs/adr/0056-native-extensions-without-the-flutter-engine.md` (item 3, "Confirmation": this ticket).
- `docs/spec/13-push-notifications.md`: 2, 3.3 (`notify.preview_mode`), 3.4 (quiet hours), 5.2 (FCM data messages only, priority, no `collapse_key`, the app replaces notifications by a tag equal to the thread ID, overflow, time budget with WorkManager), 6.2 (payload keys `v`, `nid`, `k`, `i`, `t`, `m`, `badge`, `quiet`, and `k = "sync"` with `badge` and `cleared`), 6.4 (preview endpoint and token, fields per mode, `404`), 8 (badge and sync), 9 (t2), 13 (`push.register`), 14.
- `docs/spec/15-client-architecture.md`: 9 (row Android), 10, 10.2 (all of it), 11, 17 (`client.push_display.duration`, `client.push.placeholder`), 21 (R-CL-06), 22 (`client.push.fetch_timeout`).
- `docs/spec/08-identity.md`: 7.3 (DPoP), 7.4 (no refresh outside the app).
- `docs/spec/01-product.md`: 10 (10.2 content, 10.3 row Android).
- `docs/api/openapi.yaml`: `registerPush`, `unregisterPush`, `getPushPreview`, `PushRegistrationCreate`, `PushPreview`.
- Android documentation: `FirebaseMessagingService` (`onMessageReceived`, `onNewToken`), message priority and deprioritization (https://firebase.google.com/docs/cloud-messaging/android/message-priority), WorkManager expedited work (`setExpedited`, `OutOfQuotaPolicy`), `POST_NOTIFICATIONS` (API 33), `NotificationCompat` with tag and group.
- `docs/DECISIONS.md`: D-PN-01, D-W-100 and its extension to M5 and M6 (D-W-120), D-W-115; section 10 (format of `D-M5-05-<n>` rows) and the rows of M3-21, M3-22, M5-01, M5-03, M5-04.
- Briefs (merged names win, D-W-100): `M3-21-fcm-adapter.md` (the exact FCM payload), `M3-22-push-registration.md`, `M5-01-device-key-android.md` (Kotlin core module), `M5-03-android-layout.md` (channels and the kind mapping), `M5-04-clientkit-nse.md` (the content rules of the NSE, which this service mirrors; the DPoP contract vector; the native strings generator), the brief of M4-31 (client metrics).
- Code on the base branch: `apps/client/android`, `packages/device_key/android/devicekeycore`, `apps/client/lib/platform/`.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Placeholder texts come from `packages/l10n`, generated into Android string resources (Design note 5).
9. Limits and timeouts only through named configuration values (`client.push.fetch_timeout` from `AppConfig`, D-W-115); code comments name the key, never the number.
10. Specification first: a change to behaviour updates `docs/spec/` in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. No Flutter engine starts for a push; the service and the worker are Kotlin only (15 10.2, ADR 0056).
12. A notification appears for every message of the kinds of 13 2 at once with the placeholder, so FCM does not deprioritize the app (13 5.2); the same notification is replaced (tag = thread ID, 13 5.2).
13. The service authenticates only with the preview token and a DPoP proof of the `dpop` key; it never refreshes a token (13 6.4, 08 7.4).
14. With `notify.preview_mode = none` no request; `404` shows "Message not available"; no network or a timeout keeps the placeholder (13 5.2, 6.4).
15. `k = "sync"` removes notifications of the `cleared` threads and sets the badge (13 8); quiet notifications (`quiet = 1`) post silently.
16. The preview token is stored only in the Kotlin side's Keystore-encrypted store, never in Dart storage or logs; no content is stored outside the encrypted database.
17. Without a Firebase project in the build configuration there is no push on Android and no FCM dependency initialised (15 10.2).

## Scope

Create in `apps/client/android/`:

- `app/src/main/kotlin/.../push/PushMessagingService.kt` (no product name in code names, 02 6 "Places that carry the product name"), `PreviewWorker.kt` (expedited `CoroutineWorker`), `PreviewClient.kt` (OkHttp or `HttpURLConnection`, no cookies, no cache), `DPoPProof.kt` (same profile as M5-04 Design note 2, signed through the `devicekeycore` module of M5-01), `PushStore.kt` (Keystore-encrypted values: preview token, registration ID, server address, preview mode, counters), `NotificationPoster.kt` (channels, tag, group per thread, silent posting, replacement, `sync` handling) (Design notes 1 to 4).
- Pigeon interface `apps/client/pigeons/push_android.dart` and the generated code (token stream to Dart, store calls from Dart).
- `src/test` (JVM) and `src/androidTest` (instrumented) tests.

Create elsewhere:

- `apps/client/lib/platform/push_android.dart`: permission request (API 33 and later), token handling, `registerPush`, handing the preview token and registration to Kotlin, re-registration on `onNewToken`, unregistration on logout, tap routing, `preview_mode` mirroring, counters to client metrics (the same rules as `push_apple.dart` of M5-04 Design note 5; share a common Dart interface `PushPlatform` if it reduces duplication).

Change:

- Gradle files: the Firebase messaging dependency and the Google services plugin applied only when the build configuration provides a Firebase configuration file; `androidx.work` dependency; license gate green.
- `AndroidManifest.xml`: the service with `com.google.firebase.MESSAGING_EVENT`, `POST_NOTIFICATIONS`, no other new permission.
- The native strings generator of M5-04 (Design note 6 there): an Android output `values/strings.xml` and `values-de/strings.xml` for the placeholder keys and the content format strings, checked by `make codegen-check`.
- `apps/client/platform-tests.json` `android` entry: the instrumented push tests.
- Docs: `apps/client/README.md` (Android push, Firebase configuration in the build configuration, how to run the tests), `docs/spec/15-client-architecture.md` 10.2 (where the preview token lives on Android, the worker, the sync handling), root `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M5-05-1` to `D-M5-05-6`.

## Out of scope

- UnifiedPush (later, 13 5.4); notification actions; the iOS side (M5-04); sharing (M5-06).
- Device E2E against the Compose stack (M5-11).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M5-05-android-push`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(client-android): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never use a real Firebase project, never send a real FCM message.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 (the emulator reaches the host's fake server through `adb reverse tcp:<port> tcp:<port>`, so the app connects to its own `127.0.0.1`) and make no other network call; FCM messages are constructed in the test (`RemoteMessage.Builder`) and passed to the service directly. Temporary directories only via `mktemp -d`, removed at the end. Stop only processes you started, by PID or emulator serial.
- Never read or print secrets; never log a preview token, an FCM token, a DPoP proof or a preview response.
- A JDK and the Android SDK must be available; the instrumented tests run on an attached emulator or device (`make apps-platform`) and in the `apps-android` CI job; the report names where they ran.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- No permission logic in the client (15 1 principle 1).
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Receiving** (13 5.2, 6.2; Ruling for docs/DECISIONS.md). `onMessageReceived`: read `data`; `v` must be `"1"`, otherwise ignore; for `k = "sync"` go to Design note 3; else post the placeholder at once through `NotificationPoster` (channel from the kind mapping of M5-03, tag = `t`, group = `t`, silent when `quiet = "1"`, small icon from the design tokens' monochrome mark, content intent to `/t/{t}?m={m}`), then, unless `preview_mode` is `none` or no preview token is stored for instance `i`, enqueue a unique expedited `PreviewWorker` keyed by `nid` with `OutOfQuotaPolicy.RUN_AS_NON_EXPEDITED_WORK_REQUEST`. The service returns within the FCM budget; no network in `onMessageReceived`.

2. **Preview worker** (13 6.4; 15 10.2). The worker reads server address, preview token and nonce from `PushStore`, builds the DPoP proof with `devicekeycore`, fetches with timeout `client.push.fetch_timeout` (the value written by the app at start), retries once on `use_dpop_nonce`, and posts the replacement with the same tag only if its `nid` is still the newest one `PushStore` recorded for the thread `t` (workers keyed by `nid` can finish out of order) and the time since receipt is within `client.push.fetch_timeout` (a run deferred as non-expedited work keeps the placeholder): `content` as the NSE does (M5-04 Design note 4), `404` "Message not available", failure keeps the placeholder (no retry by WorkManager after the timeout, because a late text would replace a notification the person may have read). It records the display duration from receipt and whether the placeholder stayed in `PushStore` counters.

3. **Sync messages** (13 8). `k = "sync"`: cancel the notifications whose tag is one of `cleared` (comma-separated thread IDs); set the badge number on the summary notification where the launcher supports it (`setNumber`); post nothing new. A `sync` message never starts the worker.

4. **Token and store** (13 13; Ruling for docs/DECISIONS.md). `onNewToken` and `FirebaseMessaging.getInstance().token` feed the Pigeon token stream; Dart registers after permission (API 33 and later: `POST_NOTIFICATIONS` requested at the same moment rule as iOS, M5-04 Design note 5) and hands the returned preview token and registration ID to Kotlin, which stores them encrypted with a Keystore key (not the device keys; a dedicated AES-GCM key without user authentication). `PushStore` keeps no content and no names. Without a Firebase configuration in the build, `FirebaseApp` is never initialised, the service is disabled in the manifest merge, and the app shows in settings that push is unavailable on this instance.

5. **Strings** (13 6.1 keys reused for Android). The generator of M5-04 writes `strings.xml` per language for the placeholder keys, "Message not available" and the format strings "%1$s · %2$s" and "%1$s: %2$s" (positional, so translators can reorder).

6. **Rulings for `docs/DECISIONS.md`** (`D-M5-05-<n>`): immediate placeholder and unique worker per `nid` (note 1); no retry after the timeout (note 2); sync handling and badge (note 3); the push store key and contents (note 4); Firebase only with a build configuration (note 4); permission timing on API 33 (note 4).

## Tests (write them first)

Kotlin JVM (`src/test`): `DPoPProofTest` (the contract vector of M5-04 Design note 2, signing input equal; verification with the public key), `PayloadParsingTest` (every kind, `quiet`, unknown `v`, `sync` with three cleared IDs), `KindToChannelTest` (the mapping of M5-03), `PushStoreTest` (no content keys; round trip with an in-memory key).

Instrumented (`src/androidTest`, emulator or device, fake server on the host loopback):

- `placeholderPostedImmediately`: a constructed `RemoteMessage` passed to `onMessageReceived` posts a notification with the placeholder in the expected channel with tag `t` before any network call (the fake server has recorded nothing yet).
- `textReplacesPlaceholderWhenReachable`: the worker runs (WorkManager test driver), the fake server answers the preview, the notification with tag `t` shows the text.
- `placeholderStaysWithoutNetwork` (PLAN row): the fake server is stopped; after the timeout of the test configuration the placeholder stays; the counter records a placeholder.
- `notAvailableOn404`, `previewModeNoneFetchesNothing`, `quietPostsSilently`, `syncClearsThreads`.
- `neverRefreshes`: the fake server records no call to the refresh, login or session endpoints during all push tests.
- `noFirebaseConfigNoService`: a build variant without the Firebase configuration has the service disabled (merged manifest check).

Dart (`apps/client/test/`): `push_android_test.dart` with a fake Pigeon host: registration only after permission, token change re-registers, logout unregisters and clears the Kotlin store, tap routing, `preview_mode` mirroring, counters submitted.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- `cd apps/client && flutter build apk --debug` (with and without a Firebase configuration file from a test fixture)
- `cd apps/client/android && ./gradlew :app:testDebugUnitTest`
- with an attached emulator or device: `make apps-platform`; otherwise it prints its SKIP line
- `make codegen-check`, `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make licenses`
- `make check`

## Acceptance criteria

- [ ] Instrumented test with a fake FCM payload: the placeholder appears at once and is replaced by the text when the fake server is reachable; without network the placeholder stays.
- [ ] `none` fetches nothing; `404` shows "Message not available"; quiet posts silently; `sync` clears threads.
- [ ] No Flutter engine starts for a push; the service never refreshes a token; the preview token lives only in the Kotlin store.
- [ ] The app builds and runs without a Firebase configuration, without push.
- [ ] README, 15 10.2, the generated strings, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; KDoc on every public Kotlin declaration and doc comments on every public Dart symbol; comments explain why.
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
4. Where the instrumented tests ran, the measured time from `onMessageReceived` to the placeholder and to the text.
5. What is not proven (real FCM delivery and deprioritization, Doze on a physical device; M5-11 covers devices against the Compose stack).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
