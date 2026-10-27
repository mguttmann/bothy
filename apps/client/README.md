# apps/client

Status: desktop app shell (M4-18), sidebar and post list (M4-19), thread view (M4-20), notification levels and channel export (M4-33): macOS and Windows builds, the composition root, the router,
the onboarding screens E1 to E7 and the empty column frames of the main view, which M4-19 to
M4-25 fill. Settings (M4-25): the sections of 01 5.1 D8 under `/settings/<section>`, with the
device and session views of `admin_ui`. Administration (M4-26): the `AdminShell` of `admin_ui`
at `/admin`, `/admin/<section>` and `/admin/<section>/<id>` (`lib/admin/admin_route.dart`),
over the REST client of the signed-in session; grants (M4-27) at `/admin/grants/<target key>`
with the grant wizard, "Who has access?" and "View as person"; hosts (M4-37) at
`/admin/hosts` and `/admin/hosts/<id>` for owners; agents (M4-28) at `/admin/agents` and
`/admin/agents/<id>`, whose page gets the agent's presence from the engine's `EphemeralStore`;
the permission hints and the access that expires soon (M6-07) at `/admin/hints` and
`/admin/expiring`. iPhone (M5-02): the iOS runner and the phone layout of the same screens (see
"iPhone"). Android (M5-03): the Android runner with the same phone screens and Android controls
(see "Android").

Flutter app for macOS, Windows, the iPhone and Android. One code base for interface and
logic; native code only where the system requires it.

## Purpose

- Spaces, channels, forums and threads; agent sessions with live box, follow-ups, components
  and slash commands ([01-product.md](../../docs/spec/01-product.md)).
- Offline reading and writing with an outbox; catch-up according to
  [04-sync-protocol.md](../../docs/spec/04-sync-protocol.md).
- Push without content, the app fetches the text itself; sharing from other apps; voice
  messages.
- Administration on the desktop (from `packages/admin_ui`), emergency actions on the phone.

## Dependencies

Allowed members (docs/spec/15-client-architecture.md, 3.1; `tools/arch-check/rules.yaml`):
`admin_ui`, `sync_engine`, `design_system`, `device_key`, `terminal_ui` and every member they
reach; `make arch` checks the manifest and every import. Outside the workspace: `flutter_riverpod`
and `riverpod_annotation` (state, with `riverpod_generator`), `go_router`, `flutter_secure_storage`,
`url_launcher`, `path_provider`, `timezone` (IANA time zones of the profile), `dio`,
`sentry_flutter` (crash reports, M4-31), `crypto` (the hashed instance tag of a crash report, the
SHA-256 of uploads and downloads), and for files and voice (M4-22, docs/DECISIONS.md D-M4-22-4)
`record`, `audioplayers`, `video_player`, `file_selector`, `desktop_drop` and `pasteboard`. The network status needs no plugin
(docs/DECISIONS.md D-M4-18-8).

## Layout

| Path | Content |
|---|---|
| `lib/main.dart` | starts the app with the ports of the platform |
| `lib/app/bootstrap.dart` | `AppPorts` and `InstanceServices`: the composition root per server address |
| `lib/app/app.dart`, `router.dart`, `deep_links.dart` | the app widget, the paths of 15 11, links from outside |
| `lib/app/account_wiper.dart` | the one wipe of every sign-out cause (R-SY-14) |
| `lib/app/discovery.dart`, `target_resolver.dart` | E1's discovery, "Not found or no access" |
| `lib/app/providers/` | Riverpod providers (generated `*.g.dart`): ports, session, engine, settings, targets |
| `lib/app/layout/` | the desktop shell, the four columns and their stored widths |
| `lib/sidebar/` | the sidebar column (M4-19): head, space tree, channel rows, foot, personal order, `SidebarNavigation` for M4-24 |
| `lib/forum/` | the post list of a forum (M4-19): head with filters, rows, Done section, "New post" dialog, paging |
| `lib/thread/` | the history column (M4-20): header, message list with paging, gaps and jumps, message tiles and actions, outbox rows, dialogs (versions, conflict, delete, reactions, title, move), the D2 layout |
| `lib/composer/` | the composer (M4-20): send path, draft sync, mention suggestions, templates |
| `lib/session_column/` | the frame of the session column with the agent part (M4-21) and the sections "Pinned" (M4-20) and "Files" (M4-22) |
| `lib/files/` | files (M4-22): the upload executor and the attaching of its files, the part plan and hashing, the composer's attachments (button, drag and drop, paste), attachment tiles and their states, image previews, downloads with `Repr-Digest`, the file cache, ticket players, the file ports and providers |
| `lib/voice/` | voice messages (M4-22): the recorder port over `record`, the recording bar, the player with speeds and the transcript |
| `lib/sync/` | the app's outbox executors: `putMySidebarSpace` (`sidebar_ops.dart`), `createThread` (`thread_ops.dart`), the thread view's writes (`message_ops_app.dart`), the notification levels and quiet hours (`notify_ops.dart`, M4-33), the interactions, commands and session actions of the agent interface (`agent_ops.dart`, M4-21); the transcript feed of the offline search (`search_index_feed.dart`, M4-23) |
| `lib/search/` | search D7 (M4-23): the notation of `from:`, `in:`, `has:file`, `before:` and `after:`, the controller that merges server and local hits, the offline search over the FTS5 index, the segments of the server, the view with its filter column |
| `lib/switcher/` | the quick switcher D6 (M4-23): local sources and the prefix search of thread titles, the recently visited targets of this device, the overlay |
| `lib/mentions/` | the view of the own mentions (M4-23) over `listMyMentions` |
| `lib/agents/` | the agent interface (M4-21): the pinned live box, the collapsed run line, the mappings of runs, trigger states and components, cards and their interactions, follow-ups, the agent's typing line, slash commands (built-ins, suggestions, options, autocomplete, invocation, loader) and the REST reads |
| `lib/running/` | "Running now" D10 (M4-23): the rows of the active runs, the filters, Stop and the cost sum; the control room cards and the run graph D14 (M4-39); their phone layouts M11 and M12 (M5-14) |
| `lib/onboarding/` | E1 to E7, the controller of the flow, codes and remaining times |
| `lib/admin/` | the route of the administration: `AdminRoute`, `adminRepositoryProvider`, the mapping of `AdminLocation` to `/admin/...` |
| `lib/settings/` | Settings (01 5.1 D8): the sections, the personal settings of 01 11 with their defaults, the profile and the devices section hosting `admin_ui` |
| `lib/notifications_settings/` | D9 (M4-33): the page inside Settings > Notifications, the tree of levels with inheritance, the quiet hours editor, the effective level of 13 3.1 for display, the providers of the notification settings |
| `lib/terminal/` | the terminal (M4-36): the group "Hosts" of the sidebar, the view of a host with its tabs, the ports of `terminal_ui` (WebSocket transport with DPoP `identify`, pins in `host_key_pins`, host events of the engine, the REST adapter), the table "In the terminal" and the providers |
| `lib/menus/` | the menus of a space, a channel and a thread (M4-33): level, "Mute until", "Export history" |
| `lib/export/` | the channel export (M4-33): range dialog, save dialog, the stream into the file with progress and cancel |
| `lib/notifications/` | local notifications (M4-30): the content of a toast, the router from `notification.created` to a toast or nothing, the presenter over `flutter_local_notifications` and their providers |
| `lib/desktop/` | the desktop window (M4-30): close to the notification area on Windows, Quit, the window part of `idle`, the tray icon with Open and Quit, "Start at login" |
| `lib/telemetry/` | app metrics and crash reports (M4-31): the collector, the measurement points, the sender, the scrubbed crash path and their providers |
| `lib/platform/` | secure storage, device key signers, loopback redirect, browser, lifecycle, the phone's privacy cover and background mapping (`lifecycle.dart`), the iOS adapters of login, passkeys and file protection (`ios_platform.dart`), the Android adapters of login and passkeys (`android_platform.dart`) on the shared browser session (`browser_session.dart`), the notification channels (`notification_channels.dart`), the terminal's platform guard (`terminal_guard.dart`: `FLAG_SECURE` on Android, the screen capture of iOS, the hardware keyboard; M5-12), network, frames |
| `lib/viz/` | "Live" (M6-16): `LivePage` (D12 with TV mode on the desktop, M10 under Me on the phone) over `LiveScreen` of `admin_ui`, and `EngineLiveFrameSource`, the `LiveFrameSource` of `bothy_viz` over the `stats.subscribe` subscription of the sync engine |
| `lib/phone/` | the phone (M5-02): the shell with tabs and one stack per tab (`phone_shell.dart`), the tab and stack rules of the paths (`phone_navigation.dart`), the pages (`phone_pages.dart`), the privacy cover; `terminal/` the terminal of the phone (M5-12): M9 `HostsScreen` (`/hosts` under Me), M8 `TerminalSessionScreen` (`/hosts/<host>`) and `PhoneTerminalCoordinator`, which detaches the sessions in the background and on the app lock and attaches them again after the lock |
| `lib/src/generated/` | the Pigeon channels of the iOS and Android runners, of push on iOS, macOS and Android and of sharing, generated from `pigeons/ios_platform_api.dart`, `pigeons/android_platform_api.dart`, `pigeons/push_apple.dart`, `pigeons/push_android.dart`, `pigeons/share_apple.dart` and `pigeons/share_android.dart` |
| `macos/`, `windows/`, `ios/`, `android/` | the runners (`flutter create`), URL scheme registration and the deep link channel; `ios/Runner/IosPlatform.swift` holds the iOS glue (`ASWebAuthenticationSession`, passkeys, file protection), `ios/Runner/PushApple.swift` the push glue that the macOS runner compiles too |
| `ios/ClientKit/` | the Swift package the native extensions share (M5-04): shared keychain, App Group state, DPoP, preview fetch, the Notification Service Extension's logic, with its tests |
| `ios/NotificationService/`, `macos/NotificationService/` | the Notification Service Extensions (M5-04), each a subclass of ClientKit's without code of its own |
| `ios/Localizations/` | the native push strings, generated from the ARB files by `make apple-strings` |
| `lib/platform/push_apple.dart`, `lib/notifications/apple_push_provider.dart` | push on iOS and macOS (M5-04): permission, registration, the preview mode for the extension, taps, display counters |
| `lib/share/` | sharing from other apps (M5-06): the child session and recent targets for the Share Extension (`share_apple.dart`), the takeover of its inbox (`share_takeover.dart`), the one send path into the outbox (`share_outbox.dart`), the Android receiver glue (`share_android.dart`), the target sheet and the route `/share` |
| `ios/ShareExtension/`, `macos/ShareExtension/` | the Share Extensions (M5-06), each a subclass of `ClientKitShareViewController` of ClientKit's product `ClientKitShare`; `ios/Runner/ShareApple.swift` is their glue in the runners |
| `ios/RunnerUITests/` | the UI test target of the `patrol` device tests (M5-06) |
| `tool/branding_defines.dart` | build branding to dart defines and native settings |
| `Makefile` | the release builds of the desktop app and their trials (M7-12, section "Release builds") |
| `test/` | widget, golden and flow tests (`make test`); the phone's in `test/phone/` and `test/goldens/phone/` |
| `integration_test/` | the app against the fake server of M4-17 on a desktop and, `integration_test/phone/`, on an iOS simulator (`make apps-platform`); `integration_test/share/` the `patrol` device tests of sharing (`tool/run_share_device_tests.sh`) |
| `e2e_test/` | the desktop E2E suite against the real server of the Compose stack (`make e2e-desktop`, M4-34): `desktop_e2e_test.dart`, the flows in `flows/`, the app, network, keys, actors and world of a run in `support/`; `terminal_e2e_test.dart` against the terminal's client stack (M4-38) with `support/terminal_stack.dart`; `share/id_t16_test.dart` is ID-T16 on the iOS simulator (`tools/e2e/id_t16.sh`, M5-06); `phone/` holds the `patrol` suites of the phone apps (`make e2e-mobile`, M5-11; [e2e_test/phone/README.md](e2e_test/phone/README.md)) |
| `perf_test/` | the performance level of the desktop targets `client.perf.*` (`make perf-desktop`, M4-34) and its evaluation `perf_summary.dart`; the terminal targets in `terminal_perf_test.dart` with their evaluation `terminal_perf.dart` (M4-38); `phone/` the measurement of the phone targets on a physical device (`tool/mobile_perf.sh`, `make perf`, M5-11; [perf_test/phone/README.md](perf_test/phone/README.md)) |
| `test_driver/` | the drivers of `flutter drive`: `e2e_driver.dart` (the suite on a device), `perf_driver.dart` (writes `build/perf/summary.json`, fails on a missed target) |
| `tool/e2e_suite.sh` | runs the suite or the performance level with the files of `infra/compose/e2e.sh`, or their terminal parts with the connection file of `tools/ci/terminal_stack.py` |
| `terminal_stack_test/` | the terminal's adapters against the terminal stack of M2-48 with a host container (`make terminal-stack`, M4-36); `phone/` the phone run of M8 on an iOS simulator or an Android emulator (M5-12, `TERMINAL_STACK_DEVICE`); outside `integration_test/`, so no app build contains it |

## Build and run

From `apps/client`, with the Flutter version of the root `pubspec.yaml`:

```sh
dart run tool/branding_defines.dart          # build/branding/dart_defines.json and native settings
flutter run -d macos --dart-define-from-file=build/branding/dart_defines.json
flutter build windows --dart-define-from-file=build/branding/dart_defines.json
```

The build branding comes from `branding.example.yaml` (neutral values under `example.com`);
`--input <file>` takes the `branding.yaml` of an instance instead (ADR 0044; the release builds
take the bundle identifier from `distribution.yaml`, section "Release builds"). Without the defines a build uses the values of `branding.example.yaml`.
The app shows `branding.product_name` of the instance after E1, never a name of its own.

A release build on macOS needs a team signature: the release entitlements carry the keychain
access group of `flutter_secure_storage` (docs/spec/15-client-architecture.md 9). Debug builds run
without one.

### Release builds

`Makefile` holds the release builds of the desktop and phone apps with the publisher values of
`DISTRIBUTION_FILE` (default: the template `../distribution/distribution.yaml`; the values and
their keys are described in [apps/distribution](../distribution/README.md)) and the build
branding of `BRANDING_FILE` (default `branding.example.yaml`; the bundle identifier, the Android
application ID and the update path come from the distribution). They run the steps of
`tools/distribution` (briefs M7-12 and M7-13, ADR 0037):

| Target | Host | Result in `OUT` |
|---|---|---|
| `make release-apple VERSION=<x.y.z> OUT=<dir> [APPCAST=<file>]` | macOS with Xcode | `<App>-<x.y.z>.zip` (Developer ID signature, hardened runtime, notarized and stapled, Sparkle.framework of the pinned release inside) and `appcast.xml` with its entry in the channel beta, both signed with the EdDSA key |
| `make release-windows VERSION=<x.y.z> OUT=<dir>` | Windows with the Windows SDK | `<package>_<x.y.z>.0_x64.msix` (signed, with the package identity, the toast activator and the URL scheme) and `beta.appinstaller` |
| `make trial-apple` | macOS with Xcode, `sudo` | the whole macOS chain with a test publisher and the Sparkle update test (`build/trial-apple/trial-apple.json`) |
| `make trial-windows` | Windows as administrator | the whole Windows chain with a test publisher and the App Installer update test (`build/trial-windows/trial-windows.json`) |
| `make release-ios VERSION=<x.y.z> OUT=<dir>` | macOS with Xcode | `<bundle id>-<x.y.z>.ipa` (Apple Distribution signature, App Store profiles of the app and its extensions), uploaded to App Store Connect and added to the TestFlight group beta |
| `make release-android VERSION=<x.y.z> OUT=<dir>` | Android SDK, Java 17 | `<application id>-<x.y.z>.aab` signed with the upload key and checked, released on the internal Play track |
| `make trial-ios` | macOS with Xcode | the unsigned archive of a test publisher with its identifiers, versions and entitlements checked (`build/trial-ios/trial-ios.json`) |
| `make trial-android` | Android SDK, Java 17 | the App Bundle signed with a throwaway upload key and checked with jarsigner, bundletool and apksigner (`build/trial-android/trial-android.json`) |
| `make release-build` | any | nothing: `release.yml` calls it for every component; the client is built by the jobs `build-apple`, `build-windows` and `build-android` |

The release targets need the signing material in the environment (the secrets of
`release-apple`, `release-windows` and `release-android`, docs/operations/release.md 4) and stop
with their names before the build when one is missing; they also stop on a placeholder of the
template. The trials need none: they create a self-signed identity in a temporary keychain or
certificate store for the run, a throwaway EdDSA key or upload key and an update host on
`127.0.0.1`, and remove them at the end; `trial-ios` signs nothing. CI runs them in the jobs
`desktop trial (macOS)`, `desktop trial (Windows)`, `mobile trial (iOS)` and `mobile trial
(Android)` when a change touches the release steps, and the mobile trials also when it touches
the files of the phone builds (`MOBILE_TRIAL_FILES` of `tools/ci/components.py`: this Makefile,
the Gradle build of the app, the Xcode project, the branding tool and template, `release.yml`;
docs/quality/gates.md). A release build on
Android without the upload key properties (`uploadStoreFile` and its passwords and alias) is
signed with the debug key, so `flutter build apk --release` works on a developer machine.

### Minimum version and update path

The app reads `client.min_version.<platform>` and `client.recommended_version.<platform>` from
`getDiscovery` (docs/spec/18-release.md 9; `decideClientVersion` in `packages/domain`, SemVer
precedence) at the start, on E1 and before every connection attempt (`VersionWatch` in
`lib/app/update/`, the connect gate of the sync engine). Below the minimum it opens no gateway
connection, closes the instance without deleting anything and shows E1 "App too old"; below the
recommended version the main view shows a notice, dismissed once per version. The action "Get the
update" opens the update path of the build (`UpdatePath`, the define `BRANDING_UPDATE_URL`, the
branding key `update_url` that the release steps write from `distribution.yaml`): the TestFlight
or store page, Sparkle's update check on macOS (channel `bothy/updater`), `stable.appinstaller`
on Windows. "Update required" after `4007` offers it too.

The macOS updater (`AppUpdater` in `macos/Runner/AppDelegate.swift`) starts only when the build
carries Sparkle and a feed, which only `release-apple` puts into the bundle; it follows the
default (stable) channel, and a beta tester adds `beta` to the user default `UpdateChannels`
(for the sandboxed app: `defaults write ~/Library/Containers/<bundle id>/Data/Library/Preferences/<bundle id> UpdateChannels -array beta`).
On Windows, App Installer checks the `.appinstaller` file the app was installed from at a start of
the app once `windows.hours_between_update_checks` hours have passed since its last check (at every
start with 0).

Crash reports need a target in the build configuration (docs/spec/15-client-architecture.md 16):
`--dart-define=CRASH_DSN=<dsn>` and optionally `--dart-define=CRASH_ENVIRONMENT=<name>`. Without
`CRASH_DSN` the crash path never starts and Settings hides `privacy.crash_reports`.

## Test

- `flutter analyze --fatal-infos && flutter test --coverage` in this folder, or
  `make lint test coverage arch COMPONENTS=apps` from the root.
- Goldens: `flutter test test/goldens --update-goldens` writes `test/goldens/*/ci/` (Ahem, compared
  everywhere) and on macOS `test/goldens/*/macos/`.
- Providers: after changing a file with `@riverpod`, run `dart run build_runner build` and
  `dart format lib`; `make codegen-check` fails on a stale `*.g.dart`.
- Integration tests: `make apps-platform` runs `platform-tests.json` on macOS and Windows, one
  `flutter test integration_test/<file> -d macos` (or `-d windows`) per file, so a new file
  needs its line there (D-M4-18-19); elsewhere it prints its SKIP line.
  They start the app from `integration_test/support/test_app.dart`, the only entry that allows
  plain HTTP, with secrets in memory and software device keys, against `FakeServer` on
  `127.0.0.1`.
  Without a desktop device each file also runs alone on the test runner, for example
  `flutter test integration_test/forum_test.dart -d flutter-tester`.
  `integration_test/admin_step_up_test.dart` walks every changing action of the administration
  through its screens and checks the step-up dialog against `docs/spec/actions.yaml`;
  `test/admin/admin_step_up_test.dart` runs the same enumeration headless (grants and "View as
  person" included). `integration_test/grant_preview_test.dart` runs the example of 07 9.3 from
  the wizard to "Who has access?"; `test/admin/grant_preview_test.dart` runs it headless over
  REST. Both compare the preview with the example the fake reads from the specification at run
  time. `integration_test/host_admin_test.dart` runs screen A8 (M4-37): create a host, the fake
  plays `bothy-host pair`, a wrong and a right fingerprint prefix, the mandatory answers, a
  session user, a grant `terminal` that is not effective until the person is the assignee; the
  step-up enumeration includes the host operations and `listAuditEvents`.
  `integration_test/guided_flow_test.dart` runs the guided flow of 10 11 (M4-28): a personal
  agent operates a space, Alex grants Sam `trigger` there, takes "Switch to Organization and
  grant" with a reason and step-up, and the agent page warns until the fake's heartbeat reports
  organization; `test/admin/guided_flow_test.dart` runs the agent calls headless over REST,
  and the step-up enumeration includes the agent and token operations.

## iPhone

The iOS runner (`flutter create --platforms=ios`) is iPhone only until the iPad layout of V3
(iOS 15). Build and run on macOS with Xcode:

```sh
dart run tool/branding_defines.dart          # also ios/Runner/Configs/Branding.xcconfig and Branding.entitlements
flutter build ios --simulator --dart-define-from-file=build/branding/dart_defines.json
flutter run -d <iPhone simulator> --dart-define-from-file=build/branding/dart_defines.json
```

Configuration (docs/decisions.d/M5-02.md D-M5-02-2): `branding.example.yaml` or the
`branding.yaml` of an instance gives the display name, the bundle identifier, the URL scheme of
deep links and, optionally, `associated_domain` (passkeys; the build gets the `webcredentials`
entitlement and offers passkeys only to an instance at that domain or below it) and
`development_team` (signing; without it the build is unsigned, enough for the simulator). The
OIDC redirect is `<bundle_id>:/oidc/callback` through `ASWebAuthenticationSession`; the operator
lists it in `app_redirect_uris` of the provider (08 2.2).

The phone shows the same paths as the desktop in its own layout (D-M5-02-3): tabs Spaces,
Mentions, Running and Me, one stack per tab built from the path, the edge swipe back, links of the
URL scheme in Spaces with the whole stack. Density `touch` and `composer.enter_sends` `false` by
default. In the background the gateway closes with `1000` and the privacy cover hides the content;
the foreground removes it and resumes (D-M5-02-6). The database files get
`NSFileProtectionComplete` before the first open (D-M5-02-7). The Swift glue is
`ios/Runner/IosPlatform.swift` with the Pigeon interface `pigeons/ios_platform_api.dart`
(regenerate with `dart run pigeon --input pigeons/ios_platform_api.dart`, then
`dart format lib/src/generated`).

Tests: the widget and golden tests of `test/phone/` and `test/goldens/phone/` run on every host
(text scale 2 without overflow, the iOS tap target guideline, the privacy cover, the live box and
the cards, the onboarding rules). The integration tests of `integration_test/phone/` run alone in
the Flutter tester (`flutter test integration_test/phone/<file> -d flutter-tester`) and, through
`tool/run_ios_integration_tests.sh` in the `ios` entry of `platform-tests.json`, on the first available iPhone
simulator in one build (`integration_test/phone_suite_test.dart`); without Xcode the script
prints its SKIP line. The simulator keeps no protection class, so the attribute of the database
file is read back only on a device.

## App lock

The phones lock (08 9, 01 5.6 E6; brief M5-08; docs/decisions.d/M5-08.md): `lib/lock/` holds the
rules (`lock_decision.dart`, pure functions over the policy, the person's switch and an injected
clock), the system authentication (`AppLockPort`, `LocalAuthAppLockPort` over `local_auth`,
flutter.dev, BSD-3-Clause), the controller (`AppLockController` with the states `checking`,
`unlocked`, `locked`, `unavailable` and the events `locked` and `unlocked`), the provider
`appLockProvider` (null on the desktop and while signed out) and the screens (E6 and the notice
without a device credential). The policy is `app_lock` of `getInstance`, cached with the instance
row of `local_store` (`instance.app_lock`, schema version 5), so the lock works offline and at a
cold start; the person's switch (Settings, Security on the phone) is device-local in the
instance's secrets and goes with the wipe. A cold start with a stored session hides the content
behind the privacy cover until the lock decided and then shows E6 when the lock applies; after
`client.app_lock_after` without a pointer or key event, in front or since the move to the
background, the lock closes; the time counts on the wall clock and on a monotonic clock that
includes sleep (the runners' time since boot through Pigeon), the larger of both, so setting the
device's clock back does not keep the lock open (D-M5-08-8). While it is
closed every route resolves to E6 (`routeStep`, without a page transition), and a
deep link or a tapped notification is kept and opened after the unlock. `security.app_lock_required`
shows the switch on and disabled; with it, or with the person's switch on, a device without a
device credential shows only the notice and "Log out". Other parts of the app follow the lock's
`events`: the terminal view of M5-12 detaches on `locked` and decides itself after `unlocked`.

Tests: `test/lock/` (rules, E6 in the app, events, the switch, the database key and a full chain
against the fake server) and `integration_test/phone/app_lock_test.dart` in the phone suite. The
system prompt is a fake port there; a person's biometric match on a simulator or emulator is not
automated. iOS shows `NSFaceIDUsageDescription` (`ios/Runner/Info.plist`, localized in
`ios/Runner/<lang>.lproj/InfoPlist.strings`, generated from the ARB message `permissionFaceIdUsage`
like the media permissions below); Android's `LaunchTheme` has an AppCompat parent,
which the biometric prompt needs on Android 8. The desktop gets the lock with a later ticket.

## Android

The Android runner (`flutter create --platforms=android`) needs API 26 or later and builds with
the JDK 17 and the Android SDK (`ANDROID_HOME`):

```sh
dart run tool/branding_defines.dart          # also android/branding.properties
flutter build apk --debug --dart-define-from-file=build/branding/dart_defines.json
flutter run -d <device or emulator> --dart-define-from-file=build/branding/dart_defines.json
```

Configuration (docs/decisions.d/M5-03.md D-M5-03-1, D-M5-03-2): `android/app/build.gradle.kts`
reads `android/branding.properties`, which the tool writes from the branding: the application ID
(`application_id`, else `bundle_id`; lower-case letters and digits per segment, because it is also
the scheme of the OIDC redirect `<application ID>:/oidc/callback`, which the operator lists in
`app_redirect_uris`), the label, the URL scheme of deep links and the App Links host
(`associated_domain`). Only with a host the build declares verified App Links for `/s/`, `/c/`
and `/t/` on that host and offers passkeys, and only to an instance at exactly that host.

Passkeys (D-M5-03-3) need the Credential Manager's platform provider (API 34 and later; passkeys
use no Play services library), Digital Asset Links on the host
(`https://<host>/.well-known/assetlinks.json` with `delegate_permission/common.handle_all_urls` and
`delegate_permission/common.get_login_creds` for the application ID and the SHA-256 fingerprint
of the signing certificate) and the app's origin in `auth.webauthn_origins` of the instance
(08 18). The origin is `android:apk-key-hash:` followed by the same SHA-256 fingerprint in
unpadded base64url; with the fingerprint from `keytool -list -v -keystore <keystore>` (hex pairs
separated by colons):

```sh
echo '<fingerprint>' | tr -d ':' | xxd -r -p | base64 | tr '+/' '-_' | tr -d '='
```

Controls (01 5.4, D-M5-03-6): the screens are the phone's of M5-02; Android has Material pages,
the top app bar with the up arrow on deeper pages, and system back that pops the tab's stack,
closes a sheet and leaves the app on a tab's first page (`lib/phone/android/`). Login runs in a
Custom Tab, or the default browser without a Custom Tabs provider, never in a web view; links
from outside reach `LinkActivity`, which hands them to the running `MainActivity`. Secrets live
in `flutter_secure_storage` (AES-GCM, the key wrapped by the Android Keystore), the database in
the private app storage; no backup and no device transfer copies the app's data
(`android:allowBackup="false"`, `res/xml/data_extraction_rules.xml`, `res/xml/backup_rules.xml`).
At every start the app registers the notification channels Mentions, Questions, All messages and
Runs in the device language (D-M5-03-4); in the background the gateway closes with `1000`, the
privacy cover hides the content, and the recent apps screen shows none (D-M5-03-5). The Kotlin
glue lies in `android/app/src/main/kotlin/com/example/client/` behind the Pigeon interface
`pigeons/android_platform_api.dart` (regenerate with
`dart run pigeon --input pigeons/android_platform_api.dart`, then `dart format lib/src/generated`). The
Gradle dependencies of the app are locked in `android/gradle.lockfile` (D-M5-03-8), those of a
build with a Firebase configuration in `android/gradle-firebase.lockfile` (D-M5-05-5): after a
change of a dependency or of the Flutter version run
`./gradlew --write-locks :app:dependencies :app:assembleDebug :app:assembleDebugAndroidTest` in
`android/`, once without and once with `app/google-services.json` (copy
`android/fixtures/google-services.json` there and remove it afterwards). A release build runs R8; it is signed with the debug key until the distribution of
M7.

Tests (D-M5-03-7): the widget and golden tests of `test/phone/` (system back, the channels, the
adapters, the Android tap target guideline) and `test/goldens/phone/android_*` run on every host.
With exactly one device or emulator attached, `make apps-platform` runs
`tool/run_android_tests.sh`: the phone integration tests (`integration_test/phone_suite_test.dart`,
the same files as on the iPhone simulator, system back instead of the edge swipe) and the
instrumented tests of the runner (`android/app/src/androidTest`: channels in the system settings,
backup rules, recent apps protection). The CI jobs `apps-android-26` and `apps-android-36` run them
on emulators of the lowest and the current API level.
## Push on iOS and macOS

Pushes carry only IDs (13 6.1, R-Push). The Notification Service Extension of each Apple app
(`ios/NotificationService/`, `macos/NotificationService/`) is Swift without the Flutter engine
(ADR 0056): it subclasses `ClientKitNotificationService` of the Swift package `ios/ClientKit/`
and replaces the localized placeholder with "<channel> · <thread title>" and
"<sender>: <excerpt>" from `GET /api/v1/push/preview`, within `client.push.fetch_timeout`, with
the preview token of the push registration and a DPoP proof of the `dpop` device key; it never
refreshes a token. With `notify.preview_mode = none` it fetches nothing; on `404` it shows
"Message not available"; without network or when the time runs out the placeholder stays.

The app (`lib/platform/push_apple.dart`) asks for the notification permission when the person
first opens a thread after the login, registers the APNs token with `registerPush` (provider
`apns`, the environment of the build, the bundle identifier), and the runner stores the preview
token and the registration ID at once in the shared keychain (never in Dart). A new token
registers again; logout calls `unregisterPush` and every end of the account deletes the items.
A tapped notification opens `/t/<thread>?m=<message>`; the runner is the notification center's
delegate only for pushes and hands every other notification to the delegate it replaced, so the
local notifications of the desktop keep working (D-M5-04-9). At start the app takes the extension's
display counters into `client.push_display.duration` and `client.push.placeholder`.

Entitlements and the shared container (docs/decisions.d/M5-04.md D-M5-04-1): App Groups and
keychain sharing need a team signature, so only a signed build has push.

- iOS: with `development_team` in the branding, `tool/branding_defines.dart` writes
  `aps-environment` (`development`; the export for distribution re-signs with `production`, and
  the app registers the environment of its embedded profile, D-M5-04-10), the App Group
  `group.<bundle_id>` and the keychain groups `<team>.<bundle_id>` (the app's own, first) and
  `<team>.<bundle_id>.shared` into `ios/Runner/Branding.entitlements`, the two shared groups into
  `ios/NotificationService/Branding.entitlements`, and `CLIENTKIT_APP_GROUP` and
  `CLIENTKIT_KEYCHAIN_GROUP` into `Branding.xcconfig`. Without a team all are empty.
- macOS: a Release build has `com.apple.developer.aps-environment` (as on iOS), the App Group
  `<team>.<bundle_id>` and the keychain group `<team>.<bundle_id>.shared`; Debug and Profile
  are unsigned and have none.
- The app and the extension read the groups from the Info.plist keys `ClientKitAppGroup` and
  `DeviceKeyAccessGroup` (the latter also places the `dpop` device key in the shared group).
- The App Group holds only `notify.preview_mode`, the instance ID, `client.push.fetch_timeout`,
  the last `DPoP-Nonce` per server and the display counters; the keychain holds the preview
  token, the registration ID and the server address per instance, `AfterFirstUnlockThisDeviceOnly`
  and never synchronized. No message content leaves the encrypted database.

Tests: `sh ios/ClientKit/run_tests.sh` runs the Swift tests of ClientKit on any Mac (Xcode or the
command line tools), among them the extension tests: they run the extension's class in the test
process of Swift Testing and call `didReceive` with a payload of 13 6.1 against a recording fake server
(`testServerReachableShowsText`, `testServerUnreachableShowsPlaceholder`,
`testPreviewModeNoneFetchesNothing`, `testNseNeverRefreshes`). The DPoP contract vector
`ios/ClientKit/Tests/ClientKitTests/Fixtures/dpop_vector.json` is checked by the Swift tests and
by `test/platform/dpop_contract_test.dart`. After the iOS simulator and macOS builds of
`make apps-platform`, `tools/ci/check_extension_links.py` fails when an extension links a Flutter
framework or is missing from an app. The strings come from `make apple-strings`.

## Push on Android

Pushes are FCM data messages that carry only IDs (13 5.2, 6.2). The push service is Kotlin
without the Flutter engine (ADR 0056 item 3; `android/app/src/main/kotlin/com/example/client/push/`):
`PushMessagingService` hands the `data` of a message to `PushReceiver`, which posts the localized
placeholder at once into the channel of the kind (the mapping of M5-03), tagged and grouped with
the thread ID so a later push of the thread replaces it, silently for `quiet = 1`, and schedules
one expedited WorkManager job per `nid` (`PreviewWorker`). The worker fetches
`GET /api/v1/push/preview` with the preview token and a DPoP proof of the `dpop` device key
(`DeviceKeyCore` of M5-01) within `client.push.fetch_timeout` from receipt, only over `https`
without redirects, and replaces the same notification with "<channel> · <thread title>" and
"<sender>: <excerpt>"; "Message not available" on `404`; the placeholder stays without network,
after the timeout, with `notify.preview_mode = none` (then nothing is fetched) and when a newer
push of the thread arrived meanwhile. A `k = "sync"` message removes the notifications of the
`cleared` threads and sets the badge number on the others. The service never refreshes a token.

The app (`lib/platform/push_android.dart`, sharing `PushPlatform` of `push_platform.dart` with
iOS and macOS) asks for the notification permission (`POST_NOTIFICATIONS` from API 33) when the
person first opens a thread after the login, registers the FCM token with `registerPush`
(provider `fcm`, the package name), and the runner keeps the preview token and the registration
ID in its own store (`PushStore`: values sealed with an AES-GCM key of the Android Keystore,
never in Dart). A new token, also one FCM rotates while the app runs, registers again; logout
calls `unregisterPush`, and every end of the account deletes the store's entries and the shown
push notifications. A tap opens `/t/<thread>?m=<message>`. At start the app takes the service's
display counters into `client.push_display.duration` and `client.push.placeholder`.

Firebase configuration (D-M5-05-5, ADR 0072): push needs the instance operator's Firebase
project as `android/app/google-services.json` in the build (from the operator's build branding;
the file is ignored by git and never part of this repository). With it the build applies the
Google services plugin, links the Firebase messaging library (which brings Google Play services
libraries under Google's own terms) and enables the service; without it the app builds and runs
without push, links no Firebase code and the service stays disabled in the merged manifest. The
FCM token is fetched only when the app registers after the permission (automatic initialisation
is off), and Firebase analytics collection is off.

Tests: `./gradlew :app:testDebugUnitTest` in `android/` runs the JVM tests (the DPoP contract
vector of M5-04, the payload, the channel mapping, the store with an in-memory key, the
renderer). With one device or emulator, `tool/run_android_tests.sh` runs the instrumented push
tests (`android/app/src/androidTest/.../push/`) in the build without Firebase and again with the
fake configuration of `android/fixtures/google-services.json`, which adds the test of the FCM
service with a constructed `RemoteMessage`. They use a TLS fake server on the device's loopback
with a CA made for each run that only the tests trust, the real `dpop` key of a fresh instance,
the WorkManager test driver and a clock the test turns. `test/platform/push_android_test.dart`
tests the Dart side against a fake Pigeon host. The Pigeon interface is
`pigeons/push_android.dart` (`dart run pigeon --input pigeons/push_android.dart`); the strings
come from `make apple-strings`, which also writes `android/app/src/main/res/values*/strings.xml`.

Device E2E switches (brief M5-11; `tool/mobile_e2e_suite.sh` sets them as
`ORG_GRADLE_PROJECT_<name>`). They change only the debug build; release and profile builds check
before they are assembled that their merged manifest and classes carry nothing of them
(`./gradlew :app:checkReleaseE2eAbsent` in `android/`, also part of `check`).

- `e2ePush=true`: the push backend of `android/app/src/e2epush/` instead of the build's own,
  without Firebase: a random fake FCM token per install (kept in the app's private storage), so
  the app asks for the permission and registers provider `fcm`; and the receiver
  `com.example.client.push.E2ePushReceiver` (exported, but only the shell user and the system
  hold its permission `android.permission.DUMP`), which hands the `data` of a push to
  `PushReceiver` as the FCM service does:
  `adb shell am broadcast -a <application ID>.E2E_PUSH -n <application ID>/com.example.client.push.E2ePushReceiver --es data_b64 <standard base64 of the JSON object of data>`.
  Malformed input is dropped and logged without content. Together with
  `app/google-services.json` the configuration fails.
- `e2eCa=<absolute path of a PEM file>`: a network security config generated under
  `build/` that keeps the app's default (system CAs, no clear text) and trusts the CA in
  `debug-overrides`, so the native preview fetch reaches the stack's TLS front through
  `adb reverse`.

`./gradlew :app:testDebugUnitTest` also runs the JVM tests of the receiver's decoding and of the
token store, with or without the switches.

## Sharing from other apps

People share text, links, images and files from other apps into a thread, a channel's main
thread or a new post of a forum (01 5.3 M6, 5.4; 15 12; docs/decisions.d/M5-06.md).

- **iOS and macOS.** The Share Extension (`ios/ShareExtension/`, `macos/ShareExtension/`) is
  Swift and SwiftUI without the Flutter engine (ADR 0056): its `ShareViewController` subclasses
  `ClientKitShareViewController` of the package product `ClientKitShare` (`ios/ClientKit/`),
  which shows the sheet (preview, comment, the recent targets, a search over thread titles and
  channels, "Only targets with write permission."), copies the shared items into the App Group
  (photos as HEIC become JPEG in bands of rows, nothing is read whole into memory) and sends by
  itself through the share core of `ClientKit`: uploads streamed from disk, then the message or
  the forum post with the extension's own `client_msg_id`. It signs with the `dpop` device key and
  its own child session, which the app creates with `createExtensionSession` and keeps in the
  shared keychain, and which only the extension refreshes.
- **Inbox.** Without network or without a valid child session the share goes into
  `ShareInbox/<client_msg_id>/` of the App Group (files and `manifest.json`, never a token); the
  app (`lib/share/share_takeover.dart`) takes it over at start and on return to the foreground
  into `share_inbox` and its outbox with the same `client_msg_id`. Any other `4xx` is shown in
  the sheet and nothing is kept.
- **Android.** `ShareActivity` takes `ACTION_SEND` and `ACTION_SEND_MULTIPLE` for the concrete
  types of `ShareTypes.MIME_TYPES` (no wildcard), copies the content URIs into the app's cache and
  hands them to the running app, which shows the route `/share` with the same sheet in Flutter
  (`lib/share/share_sheet.dart`) and sends through the outbox.
- **Recent targets.** The app derives the last `client.share.recent_targets` targets from the
  person's own messages, keeps only writable channels, writes them to the App Group and deletes
  them at every end of the account; the display paths are the only names in the shared container.
- **Builds.** The App Group and the keychain group exist only in a signed build (D-M5-04-1); the
  branding tool writes `ios/ShareExtension/Branding.entitlements` like the extension of push. For
  the device tests, `CLIENTKIT_SIMULATOR_GROUPS=YES` in the environment of a build gives an
  unsigned simulator build the groups (`SimulatorGroups.entitlements`, D-M5-06-8).

Tests: `sh ios/ClientKit/run_tests.sh` runs the Swift tests of the share core and the sheet
(among them `testExtensionRefreshesOnlyChildFamily`, `testSingleFlightRefresh`,
`testUploadStreamsFromFile`, `testFailuresGoToInbox`, `test4xxShowsReasonAndKeepsNothing`,
`testInboxManifestHasNoTokens`); `flutter test test/share` the app side with its goldens. The
device tests use `patrol` (`dart pub global activate patrol_cli 4.8.0`):

```sh
sh tool/run_share_device_tests.sh ios       # a simulator: Photos -> Share Extension, offline inbox
sh tool/run_share_device_tests.sh android   # one device or emulator: the file app -> the sheet
tools/e2e/id_t16.sh                          # from the repository root: ID-T16 against the stack
```

The wrappers add the samples of `integration_test/share/assets/` and print a SKIP line without a
simulator or device; `make apps-platform` runs both. `tools/e2e/id_t16.sh` starts the stack of
the next section with the overlay `infra/compose/test/compose.id-t16.yaml` (short access
tokens), trusts its test CA in the simulator and runs `e2e_test/share/id_t16_test.dart`: the app
and the extension refresh at the same moment, both succeed, and the server records no reuse.

## E2E and performance against the Compose stack

Only an E2E against the real server proves that the app and the server agree (15 20; M4-34).
From the repository root, with `make image` built and a Docker engine running:

```sh
make e2e-desktop                              # the suite on this host's desktop system
E2E_DEVICE=flutter-tester make e2e-desktop    # the same suite headless, without a desktop session
make perf-desktop PERF_DEVICE_LABEL="<device>" PERF_REFERENCE=1   # profile build, 15 19 targets
```

`infra/compose/e2e.sh` starts the stack (`infra/compose/README.md`, section "Desktop E2E stack"),
prepares the world and runs `tool/e2e_suite.sh`, which hands the world (`e2e.json`) and the test
CA to the build as `--dart-define` values: a desktop app runs in its sandbox and cannot read the
files of the stack. The suite runs on a device through `flutter drive` with
`test_driver/e2e_driver.dart` (`flutter test` would run a file outside `integration_test/` on
flutter-tester whatever device it names) and headless through `flutter test`.

Each flow starts a fresh app with its own data folder, memory secrets and software device keys
(no test touches the keychain of the machine that runs it) and the discovery of a real build,
which talks HTTPS only. Every connection of the app goes through an HTTP CONNECT tunnel in the
test process (`support/e2e_network.dart`) that trusts only the test CA and that the offline flow
cuts and opens again. Sam and the owner act through the public API (`support/actors.dart`),
Alex's waiting second device is a second app without a view in the same test
(`E2eSecondApp`); a code of Alex is issued right before each sign-in, because issuing ends
every other unused code (08 3.2). Waits end when their condition holds, bounded by one patience.

The performance level measures in a profile build: cold start (a start on a filled data folder
until the sidebar shows the space), thread open from the cache, local echo (cold start, thread
open and echo end with the first drawn frame that shows the result), patch to frame (a
status line patch of `stream-agent` from its arrival until the end of the first frame that shows
it), the jank ratio while scrolling a thread of thousands of messages, the catch-up rate after a
reconnect with a backlog and the resident size with twenty threads in focus. p95 values need at
least twenty samples; `perf_summary.dart` compares them with `AppConfig` and the driver writes
`build/perf/summary.json` with the device and whether it is a reference device of 15 19.

### The phone apps (M5-11)

```sh
make e2e-mobile                                        # iOS simulator and Android emulator
MOBILE_E2E_PLATFORMS=android MOBILE_E2E_AVD=<avd> make e2e-mobile   # one platform, a given AVD
bash tool/mobile_perf.sh                               # the phone targets on an attached phone
```

`tools/e2e/mobile_e2e.py` creates a simulator and starts an emulator (or uses
`MOBILE_E2E_UDID` and `MOBILE_E2E_SERIAL`), starts the stack of `infra/compose/e2e.sh` with the
overlay `infra/compose/test/compose.e2e-mobile.yaml` (a test build of the server whose push goes
to fake APNs and FCM providers in the stack) and runs `tool/mobile_e2e_suite.sh` per platform,
while its relay `tools/e2e/push_relay.py` hands every push the fake providers captured to the
device (`xcrun simctl push`, or a broadcast to the debug-only receiver on Android, section "Push
on Android"). A platform whose tools are missing prints a SKIP line; the run fails when none ran.
The suites are described in `e2e_test/phone/README.md`. The measurement of the phone targets
`client.perf.*` needs a physical phone in a profile build; it counts only on a reference device
of 15 19 (the list is in `perf_test/phone/mobile_perf_summary.dart`, with the reason per entry)
and prints a SKIP line without a phone (`perf_test/phone/README.md`).

### The terminal parts (M4-38)

Both targets end with their terminal part, which runs on its own as well:

```sh
make e2e-desktop-terminal                              # pair, open, run, detach, reattach, end
E2E_DEVICE=flutter-tester make e2e-desktop-terminal    # the same headless
make perf-desktop-terminal PERF_DEVICE_LABEL="<device>"   # the two terminal targets of 15 19
```

The terminal needs a host service, which the Compose stack of M4-34 has not, so
`tools/ci/terminal_stack.py` starts the client stack of M4-36 (`services/server/test/terminal/
clientstack`: the stack of M2-48 with its TLS front, its test CA and one host container; Go and a
Docker engine with buildx) and hands its connection file to `tool/e2e_suite.sh terminal` or
`terminal-perf` as the define `TERMINAL_STACK`. The app keeps the origin
`https://chat.example.com:8443` and its tunnel dials the published port of the TLS front.

- E2E (`e2e_test/terminal_e2e_test.dart`, mode `e2e`): the host container is installed but not
  paired. Alex, an owner, creates the host in Administration, Hosts, with himself as assignee;
  the test hands the code of the wizard to the helper's control endpoint on `127.0.0.1` (bearer
  token of the connection file), which runs `bothy-host pair` in the host container of the
  stack's project and returns the printed fingerprint; Alex types its first characters, answers
  organization and "no" for the server host and assigns `term-alex`. Then he opens a shell from
  the group "Hosts" with the step-up, runs `echo` with a marker, detaches (the server keeps the
  session detached), reattaches (the scrollback replaces the screen and shows the marker again),
  runs `exit 3` (the tab shows the code) and ends the tab. A canary he typed must be in no row of
  the local database, no file of the data folder, no `print` or `debugPrint`, no client metric,
  and, scanned by the helper, in no container log, the database dump, the collector export or
  the host journal.
- Performance (`perf_test/terminal_perf_test.dart`, mode `perf`, profile build with
  `--dart-define=TERMINAL_PERF_HOOKS=true`): the host is paired and runs the generator
  `clientstack/termgen` as two `command` profiles. The timing hooks of `terminal_ui` report each
  decrypted `data` frame and each drawn frame after new output; the engine's frame timings give
  the end of the rasterization of that frame. Frame to paint is measured per numbered line of the
  rate profile (p95 over at least 1000 lines); the throughput per chunk of the burst profile
  (chunks at full speed, each after a line feed the test types once the chunk before is drawn,
  so at most one chunk is in flight and a reader that falls behind would be disconnected by the
  host's send queue). The values join `build/perf/summary.json` with the generator settings and
  the largest backlog of the renderer; a miss fails the target.

## Sidebar and post list

The sidebar (`lib/sidebar/`) and the post list (`lib/forum/`) read the local database through
Riverpod providers and never query it from a widget (R-CL-03). The person's order and collapse
state of the spaces is written locally at once and sent through the outbox as
`putMySidebarSpace`; another device's change arrives as `sidebar.updated` and wins by version
(04 14). Order keys come from `FractionalIndex` in `domain`, the rule the server validates.
Counters per channel come from the sync engine and the stored read states. A forum's post list
pages through `listChannelThreads`, writes new posts as `createThread` through the outbox, and
keeps the collapse state of its "Done (n)" section per device in `forum_view_states` (01 6.1;
docs/DECISIONS.md D-M4-19-1 to D-M4-19-7).

## Notification levels and channel export

The menu of a space or channel (right click, the menu key or Shift+F10 on its sidebar row), of a
post (on its row in the post list) and the "More" button of the thread header set the level All,
Mentions or Mute, "Mute until" (one hour, tomorrow 08:00 in the person's time zone, a chosen
time) or "Inherit", and name the inherited level with its source; a thread the server follows
shows "Following" (13 3.1, 3.2). Settings > Notifications holds D9: the personal switches
`notify.*` of 13 3.3, the quiet hours rules and the tree of levels. The effective level is
computed for display only; the server decides every notification. Writes change the local row
at once and go through the outbox; the lists of the server are loaded once per session because
the snapshot does not carry them. "Export history" appears only with the capability `export`;
the step-up is the server's demand, and the stream goes unchanged into the chosen file
(docs/DECISIONS.md D-M4-33-1 to D-M4-33-7).

## Thread view, composer and message actions

The history column (`lib/thread/`) reads the contiguous window of a thread from the local
database and puts the outbox entries of the thread after the last confirmed message; a gap shows a
loading row until the engine filled it (04 4), scrolling to the top loads older pages, and
`/t/<thread>?m=<message>` loads the range down to the message and highlights it. The visible
threads form the gateway focus, and the newest message in view marks the thread read through
the outbox. The composer (`lib/composer/`) sends with the limit of the instance, writes the draft
as one coalesced outbox entry, takes another device's draft only while the person does not type
(04 14) and offers mention suggestions from the stored people and agents and the templates. The
message actions (hover bar, right click, menu key or Shift+F10) and the header buttons are shown
by the level and capabilities the server reported for the channel, never by grants; every write
goes through the outbox (docs/DECISIONS.md D-M4-20-1 to D-M4-20-8). Rendering loads and runs
nothing from a message (SEC-T13). The session column (`lib/session_column/`) has its frame with
the pinned messages and the files of the thread; the agent session follows (M4-21).

## Files and voice messages

Attaching by the attachment button, drag and drop or paste checks the size against
`file_max_upload_bytes` and the count against `message_max_attachments` and then writes one
`upload` entry per file into the outbox, before the message (`lib/files/attach_controller.dart`).
The executor (`upload_executor.dart`) creates the session with the entry ID as
`Idempotency-Key`, sends the parts `getUpload` has not confirmed, at most `UploadOps.parallelParts` at a time and each
with `Content-Digest`, and completes the session; the message then attaches the files
(12 3.4, D-M4-22-1). The SHA-256 is computed in a background isolate while the file streams.
Attachments show the states of 12 2 (`attachment_mapping.dart`, `attachment_tile.dart`); images
show thumbnails from `downloadThumbnail` only, never from a URL in the content (SEC-OUT-2), and
`![alt](attachment:<file_id>)` in the content shows the thumbnail of the message's own file.
Downloads (`download_service.dart`) stream into the file cache (`file_cache.dart`,
`<instance folder>/files`) and keep only bytes that match `Repr-Digest`; audio and video play
through file tickets (`ticket_player.dart`). Voice messages (`lib/voice/`) record AAC-LC mono in
`.m4a` at `client.voice.bitrate` up to `voice_max_duration_ms`, play at the speeds of `voiceSpeeds` and show the
transcript when `file.updated` reports it. Every outside dependency is a port in
`lib/files/file_ports.dart` (`AppPorts.files`), so the tests use a fake recorder, fake players
and no dialogs. On macOS the runner declares `NSMicrophoneUsageDescription`, the audio input
entitlement and access to files the person picks; Windows needs nothing beyond the packages.

### Voice messages and photos on the phone

On the phone (brief M5-07) the microphone beside the input is held to record
(`lib/phone/compose/voice_button.dart`): a swipe left discards, a swipe up locks, releasing
sends; `PhoneVoiceRecorder` (`voice_recorder.dart`) runs the states, shows the notice
`client.voice.limit_notice_lead` before `voice_max_duration_ms` and stops a margin before the
limit, so the `mvhd` duration the server reads stays within it (D-M5-07-1). With an accessibility
service on, the microphone is a button: tap to record, tap to stop, then Send or Discard. The
background or a call stops a running recording and keeps it. The attachment button opens camera,
photo library or file; `media_picker.dart` turns a photo into a JPEG before the composer attaches
it. Recordings and photos lie in the app's temporary folder until the outbox moves them into
the outgoing folder.

Media permissions: iOS asks for the microphone and the camera at their first use with
`NSMicrophoneUsageDescription` and `NSCameraUsageDescription`; their texts per language are in
`ios/Runner/<lang>.lproj/InfoPlist.strings`, generated from the ARB messages
`permissionMicrophoneUsage`, `permissionCameraUsage` and `permissionPhotoLibraryUsage` with
`python tools/ci/info_plist_strings.py` (`make codegen-check` fails when they are stale). The
photo library uses the system picker, which asks for no permission; `Info.plist` still names
`NSPhotoLibraryUsageDescription` because App Store review requires it. Android declares
`RECORD_AUDIO` and `CAMERA` and asks at the first use. A denied permission shows the way to the
settings.

How the recorder is tested: the widget tests (`test/phone/compose/`) use the fake recorder and a
fake `RecordingClock`, so the limit, the notice and the interval of the announcements are checked
without waiting; `test/voice/mp4_duration_test.dart` checks the duration reader. The device test
`integration_test/phone/voice_test.dart` uses the fake recorder by default, since the audio
input of a hosted CI simulator or emulator is not dependable (a start there can hang). With
`CLIENT_VOICE_REAL_RECORDER=1` in the environment of `tool/run_ios_integration_tests.sh` (or
`--dart-define=CLIENT_VOICE_REAL_RECORDER=true`) it records with the real recorder on an iOS
simulator or an Android emulator when the microphone was granted before the run and a probe
recording works (the script grants it on the simulator); otherwise it prints a
`SKIP client voice-device` line and uses the fake recorder.

## Search, quick switcher, mentions and Running now

The cross-cutting views of 01 3.5 and 5.1 (M4-23) open from the sidebar head and the routes
`/search?q=`, `/mentions` and `/running` (15 11); the keyboard bindings follow in M4-24, which
calls `showQuickSwitcher` and the routes. Search D7 sends `search` with the filters of 14 4.4 as
parameters (the notation `from:`, `in:`, `has:file`, `before:` and `after:` is translated in the
app; the server never sees it), pages with the opaque cursor, restarts on `cursor_invalid` and
renders only the server's `segments`, never HTML. While the server answer is pending, and while
the app is offline, the local FTS5 index of `local_store` answers with the note "Messages stored
only on this device"; the server's hits replace them. The index holds the cached messages,
thread titles and, filled by `SearchIndexFeed`, transcripts; a removed target (04 8.3) and a
wipe take its rows with them. The quick switcher D6 lists spaces, channels, people, agents and
commands from the local database and thread titles by prefix, also from `searchThreadTitles`,
with the recently visited targets of this device first (local-only table `recent_targets`, never
synchronized) and at most `ui.quick_switch_max_results` rows. Mentions list `listMyMentions` and
jump to `/t/<thread>?m=<message>`. "Running now" D10 loads `listActiveRuns` once into the local
`runs` table and then follows `run.state_changed`; Stop shows only where the server reported the
level `trigger` and the person started the run or holds `manage`, and goes through the outbox;
the footer sums the organization's runs as an estimate and notes that personal runs are
information only (01 6.7; docs/DECISIONS.md D-M4-23-1 onwards).

## Control room and run graph

"Running now" shows the control room cards by default and the rows of M4-23 under "List" (01 D10;
M4-39). `ControlRoomCards` measures after every layout and scroll which cards are on screen and
hands them, top to bottom, to `ControlRoomFocus`: `EngineFocus.setBackgroundRuns` puts their
threads into the `focus` member `background_thread_ids` after the threads the app shows, within
`sync.max_focused_threads`, and never changes `idle`, which keeps following the window (04 6.2,
13 4.4). A card with a place draws the run document from the run store (current tool, open
subagents, workflows and jobs, stall notice); the others draw the summary with "live details
paused" and the stall mark of the last `listActiveRuns` load. The card and its button "Graph"
open the run graph at `/running/<run>?t=<thread>`; the button "Graph" of the live box opens the
same route with `from=thread`. `RunGraphScreen` focuses the run with its thread as a background
thread, derives the graph with `RunGraphModel.fromRun` of `bothy_viz` from the generated `Run`
on every applied patch and once a second, shows the final document from `getRun` after the end,
and offers Stop only where the server's answers allow `run.cancel`
(docs/decisions.d/M4-39.md).

On the phone (M5-14; 01 5.3 M11, M12) the tab Running shows the same view with the cards in one
column at full width, Graph and Stop stacked at full width, and the filters as a sheet
(`RunningFilterSheet`). A card opens the run graph on the Running stack (`PhonePageKind.runGraph`);
the button "Graph" of the live box sheet closes the sheet and opens it on top of the thread. The
screen then has the touch layout of `RunGraphView` (`touch: true`; the bars below the labels in
portrait and beside them in landscape, chosen by `runGraphLayoutFor` at `DsLayout.overlayWidth`)
and Stop at full width at the bottom; back is the page frame's, on Android the top app bar and
system back. `ControlRoomFocus` listens to the lifecycle port on the phone: in the background it
frees the cards' places at once and in the foreground it hands them in again, so the cards are
live only in the foreground (docs/decisions.d/M5-14.md).

## Live and statistics

The sidebar group "Live and statistics" (01 4, D12) opens "Live" at `/live` and "Statistics" A7
at `/admin/statistics`; the phone has "Live" under Me (M10). `LivePage` hosts `LiveScreen` of
`admin_ui` with the app's transport (docs/decisions.d/M6-16.md): `EngineLiveFrameSource`
subscribes the gateway connection with `stats.subscribe` for the scope and turns each `stats.live`
into a frame (nothing is stored); `ApiStatsQuerySource` and `ApiAdminStatsSource` run over the
client of the session (DPoP). TV mode (desktop only) covers the window, rotates every
`viz.tv_rotation_interval`, shows costs only with `viz.tv_show_costs` and ends on a key or a
click. "Open in Grafana" appears only when `getInstance` names `telemetry_dashboard_url` and opens
the system browser. On the phone the gateway closes in the background, so the server ends the
subscription and the views turn stale after `viz.stale_after`; `resume` restores it. A run card
opens `/t/{thread_id}`. The app contains no WebView and no iframe: `make arch` runs VZ-T16
(`tools/arch-check/no_webview.py`). Tests: `test/viz/`, `integration_test/viz/live_test.dart`
(desktop) and `integration_test/phone/viz_test.dart` (in the phone suite).

## Agent interface

The agent interface of 01 "Agents" (M4-21) lives in `lib/agents/`. While a run of the open thread
is active, `LiveBoxSlot` pins the live box over the history: the newest active run of the local
`runs` table is focused in the engine (its thread joins the gateway focus, so patches arrive
only for a visible thread) and its document from the `RunStore` is drawn at most once per frame;
the run time counts locally. Stop shows only where the server reported the level `trigger` and
the person started the run or holds `manage`, and goes through the outbox without a
confirmation. At the end the box empties and `RunLine` draws the collapsed line at the run's
anchor message (`kind = run`) from the summary of `run.state_changed`; expanding loads the
document once with `getRun`; after `interrupted` or `stale` it offers "Send again". The
triggering message shows its trigger state as an icon with a label (`cancelled` neutral). While
a run is active the composer reads "Follow-up to <agent>", and the agent's line "<agent> is
working" follows the run state (D-L9). Cards (`kind = card`) draw `DsMessageComponents` with the
header built only from server fields and the agent's text below the divider (SEC-OUT-4); one
activation is one `createInteraction` in the outbox. The slash suggestions offer the built-ins
of 09 10.1, which run their own operations, and the registered commands of the channel, which
`command_loader.dart` loads into the local table when the channel opens; option suggestions come
from `autocompleteCommand` after `ui.autocomplete_debounce`, and a complete command goes through
`invokeCommand`. The session column's agent part (`session_column/agent_section.dart`) shows the
agent card, the backend badge with the warning on a mismatch, model, session, project, context,
thread cost, runs and earlier sessions (docs/decisions.d/M4-21.md).

## Keyboard shortcuts and focus

The desktop app is usable by keyboard alone (01 7 and 8, M4-24). `lib/keyboard/bindings.dart`
holds the first table of 01 7 as data; `AppShortcuts` binds it above the columns of the shell
and the views offer their targets with `KeyboardActions` (Stop and the live box toggle only with
an active run, Done only in a thread, New post only in a forum). An action without a target does
nothing and leaves the key alone. Components register no action, so no shortcut can approve
anything (R-PR-04); Enter and Space on the focused button are the only keys that apply one. The
composer keeps Enter, Shift+Enter and arrow up; the history handles Esc (mark read) and "Esc then
Tab" (focus the input). F6 and Shift+F6 move between the visible columns (sidebar, list,
history, session), Tab moves in reading order inside a column, and the focus ring is always
drawn. `KeyboardScope.passThrough` marks the terminal region of M4-36, where only Cmd (macOS) or
Ctrl+Shift (Windows) shortcuts reach the app (R-PR-10). Cmd+/ or Ctrl+/ shows the list in the
language of the app; on macOS the menu bar offers the same actions. "Open hosts and terminal"
opens the first host the person sees, or the hint to choose one (M4-36). `test/keyboard/readme_table_test.dart` holds this table equal
to `bindings.dart`:

| Action | macOS | Windows |
|---|---|---|
| Quick switcher | Cmd+K | Ctrl+K |
| Search | Cmd+Shift+F | Ctrl+Shift+F |
| Search in thread | Cmd+F | Ctrl+F |
| New post or new chat in the current forum | Cmd+N | Ctrl+N |
| Focus the input | Esc then Tab, or Cmd+L | Esc then Tab, or Ctrl+L |
| Next or previous channel | Option+Arrow down / up | Alt+Arrow down / up |
| Next or previous unread channel | Option+Shift+Arrow down / up | Alt+Shift+Arrow down / up |
| Switch column (sidebar, list, history, session) | F6 / Shift+F6 | F6 / Shift+F6 |
| Mark thread as read | Esc | Esc |
| Mark as unread (from a message) | Option+Click | Alt+Click |
| Mark thread as done or reopen it | Cmd+Shift+E | Ctrl+Shift+E |
| Stop the run in the current thread | Cmd+. | Ctrl+. |
| Expand and collapse the live box | Cmd+Shift+L | Ctrl+Shift+L |
| Show and hide the session column | Cmd+Shift+I | Ctrl+Shift+I |
| Edit the last own message | Arrow up in the empty field | Arrow up in the empty field |
| Send / new line | Enter / Shift+Enter (reversible) | Enter / Shift+Enter (reversible) |
| Running now | Cmd+Shift+R | Ctrl+Shift+R |
| Mentions | Cmd+Shift+M | Ctrl+Shift+M |
| Settings | Cmd+, | Ctrl+, |
| List of shortcuts | Cmd+/ | Ctrl+/ |
| Open hosts and terminal | Cmd+Shift+T | Ctrl+Shift+T |

## Status page (M7-21)

`/status` (`lib/status/status_route.dart`) shows the status page of 16 12 with the
`StatusScreen` of `admin_ui`: on the desktop as its own screen, opened from the foot of the
sidebar ("Status page"), on the iPhone and on Android as a page of the Me tab. The
`appStatusSourceProvider` gives it `getStatus` through the session's client, the engine's
`status.updated` (`SyncEngine.statusUpdates`) as the changes and the engine's connection state as
the reachability (`serverUnreachableOf`: `offline` and `backoff` are unreachable, `live`,
`catchingUp` and `resyncing` reachable); while the server is unreachable the page shows "Server
unreachable" and hides every state (16 12 "Limit"). The component of an agent shows only when the
server returns it. The owner forms of `status.manage` are in the administration at
`/admin/status` (listed for owners), over the same source. Tests: `test/status/status_test.dart`
against the fake server.

## Terminal (M4-36)

Screen D11 of 01: the group "Hosts" below the spaces of the sidebar lists every host the server
returns (`listHosts`) with its connection (from `connected` and `host.presence_changed`),
"disabled" for a disabled host and the count of the person's own sessions that are not closed
(`listTerminalSessions`). A host opens `/hosts/<id>`: the view of `terminal_ui` on the right with
one tab per own session (a session the server lists becomes a detached tab with "Reattach"),
"New session" with the host's profiles, the dialog "Open terminal" (for `claude` only the working
folders of the person's own session user) and the step-up through the step-up device key. The
header counts down to `terminal.max_session_duration` of `AppConfig`. Hosts and sessions are not
stored; they are reloaded after the host events of the engine. Denials show the server's reason.

The ports of 15 23.1: `IoTerminalTransport` (its own WebSocket per stream, `identify` with the
access token and a DPoP proof), `LocalHostKeyPinStore` (`host_key_pins`, never cleaned, deleted
only with all local data), `EngineHostEvents` (`SyncEngine.hostEvents`) and
`ApiTerminalServerApi` over the session's REST client without the step-up interceptor
(`InstanceServices.terminalApi`). Terminal content stays in memory (R-CL-17); the search in the
scrollback reads it there and keeps nothing.

In the terminal every key goes to the shell; the shortcuts of the table "In the terminal" of 01 7
(`lib/terminal/terminal_shortcuts.dart`, held equal to the specification by
`test/terminal/terminal_shortcuts_spec_test.dart`) need Cmd on macOS and Ctrl+Shift on Windows:

| Action in the terminal | macOS | Windows |
|---|---|---|
| Copy / paste | Cmd+C / Cmd+V | Ctrl+Shift+C / Ctrl+Shift+V |
| Quick switcher | Cmd+K | Ctrl+Shift+K |
| New session on this host | Cmd+T | Ctrl+Shift+N |
| Next or previous session | Cmd+Shift+] / [ | Ctrl+Shift+Arrow right / left |
| Detach session (keeps running) | Cmd+Shift+D | Ctrl+Shift+D |
| Search in the scrollback | Cmd+F | Ctrl+Shift+F |
| Font larger / smaller | Cmd+Plus / Cmd+Minus | Ctrl+Shift+Plus / Ctrl+Shift+Minus |

Tests: `test/terminal/` (group, view, denials, shortcuts, adapters, accessibility),
`integration_test/terminal_fake_test.dart` (the fake server with an in-process host: opening,
detaching, reattaching with the scrollback, `4103`, `4100` with `access_revoked`, a changed key,
the canary), `make terminal-stack` (the real server and host service in containers) and the
terminal parts of `make e2e-desktop` and `make perf-desktop` (section "The terminal parts").

## Client metrics and crash reports

`lib/telemetry/` measures the closed list of docs/observability/signals.md 4.6a (`ClientMetric`,
held equal to the table by a test) and nothing else: no text, no name and no ID of a thread,
channel or person leaves the device (15 17, ADR 0057). The measurement points:

| Metric | Starts | Stops |
|---|---|---|
| `client.cold_start.duration` | the first line of `main` | the frame after the sidebar first shows spaces from the local database |
| `client.thread_open.duration` | the history column of a thread is created | the frame after it first shows messages |
| `client.send_local.duration` | Enter in the composer | the frame after the outbox write |
| `client.frames`, `client.frames.over_budget` | the frame timings of the engine | per frame; over budget above one refresh interval |
| `client.catchup.duration`, `client.catchup.events` | the connection enters catch-up | it is live; events are the `dispatch` frames in between |
| `client.resync` | a `Resynced` notice of the engine | |
| `client.reconnects` | a new gateway connection, with the close code of the previous one | |
| `client.outbox.depth` | every change of the outbox depth | |

Histograms use the bounds "Durations up to seconds" of signals.md 4.7; a value above the top
bound counts in the top bucket (`le` 10), because a bucket carries only its upper bound and the
server accepts only these bounds. The sender sends at most once per
`client.metrics.flush_interval` and when the app quits, only while `privacy.client_metrics` is on,
and stops after `403 feature_disabled` until the setting changes (docs/DECISIONS.md D-M4-31-2).
Crash reports go through `sentry_flutter` only with a `CRASH_DSN` and, after the login, the
person's stored `privacy.crash_reports` on (D-M4-31-5), and report Dart errors only: the
native crash handlers and the session pings are off, because they bypass the scrubber
(D-M4-31-6);
`beforeSend` and `beforeBreadcrumb` remove every content, header, token and URL parameter
(D-M4-31-3). `test/telemetry/canary_test.dart` drives the collector, the sender and the Sentry
pipeline with a recording transport while a thread with canary content shows and asserts that no
canary value leaves the device; no test talks to a real Sentry service.

## Notification area and local notifications

On Windows, closing the window hides it to the notification area (`window_manager`): the app keeps
running and keeps its gateway connection there. The tray icon (`tray_manager`, the runner's icon)
has the menu Open and Quit; a click on the icon opens the window. Quit closes the services of the
instance (sync engine and local database) and then ends the app, so the device has no gateway
connection any more and the server creates no notification for it (13 4.6 item 3). A second start
of the app shows the running window instead of starting a second instance. On macOS the window
keeps the system's behaviour; there is no tray icon (docs/DECISIONS.md D-M4-30-5).

Local notifications come only from the gateway event `notification.created` (R-PN-01), on
Windows and on macOS while the app is connected (13 5.3): title "<Channel> · <Thread title>",
text "<Author>: <excerpt>" from the preview fields, or the placeholder "New message" with the
channel name the local database knows (01 10.2). A quiet notification has no sound, each thread
has one slot that the next notification replaces, `security` and `admin` have one slot per kind,
and nothing shows for the thread that is open in the focused window. A click opens the thread at
the message through the router (15 11) and brings a hidden window back. A closed app gets no
notifications; Settings > Notifications says so and holds "Start at login", a device setting
kept by the system (the run key of the current user on Windows, the login item on macOS 13 and
later), off by default.

The MSIX of `make release-windows` gives the app package identity, with the toast activator
CLSID the app derives from its bundle identifier, so `cancel` and `getActiveNotifications` work in
an installed package. Limits without package identity (a build run from `flutter run` or the
build folder): toasts work with the app user model ID
of the build branding and a GUID derived from it, but `cancel` and `getActiveNotifications` do
nothing, so the app replaces a slot by showing the next notification under the same ID and logs
one line without content instead of removing one (docs/DECISIONS.md D-M4-30-2). The window state
decides `idle` in the focus: hidden, minimized or behind another window is idle (15 10,
D-M4-30-3).
