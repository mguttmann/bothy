# apps/client

Status: desktop app shell (M4-18), sidebar and post list (M4-19), thread view (M4-20): macOS and Windows builds, the composition root, the router,
the onboarding screens E1 to E7 and the empty column frames of the main view, which M4-19 to
M4-25 fill. Settings (M4-25): the sections of 01 5.1 D8 under `/settings/<section>`, with the
device and session views of `admin_ui`. Administration (M4-26): the `AdminShell` of `admin_ui`
at `/admin`, `/admin/<section>` and `/admin/<section>/<id>` (`lib/admin/admin_route.dart`),
over the REST client of the signed-in session; grants (M4-27) at `/admin/grants/<target key>`
with the grant wizard, "Who has access?" and "View as person"; hosts (M4-37) at
`/admin/hosts` and `/admin/hosts/<id>` for owners. iOS and Android follow in M5.

Flutter app for macOS and Windows, later iOS and Android. One code base for interface and
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
`sentry_flutter` (crash reports, M4-31) and `crypto` (the hashed instance tag of a crash report). The network status needs no plugin
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
| `lib/session_column/` | the frame of the session column with the section "Pinned" (M4-20) |
| `lib/sync/` | the app's outbox executors: `putMySidebarSpace` (`sidebar_ops.dart`), `createThread` (`thread_ops.dart`), the thread view's writes (`message_ops_app.dart`) |
| `lib/onboarding/` | E1 to E7, the controller of the flow, codes and remaining times |
| `lib/admin/` | the route of the administration: `AdminRoute`, `adminRepositoryProvider`, the mapping of `AdminLocation` to `/admin/...` |
| `lib/settings/` | Settings (01 5.1 D8): the sections, the personal settings of 01 11 with their defaults, the profile and the devices section hosting `admin_ui` |
| `lib/telemetry/` | app metrics and crash reports (M4-31): the collector, the measurement points, the sender, the scrubbed crash path and their providers |
| `lib/platform/` | secure storage, device key signers, loopback redirect, browser, lifecycle, network, frames |
| `macos/`, `windows/` | the runners (`flutter create`), URL scheme registration and the deep link channel |
| `tool/branding_defines.dart` | build branding to dart defines and native settings |
| `test/` | widget, golden and flow tests (`make test`) |
| `integration_test/` | the app against the fake server of M4-17 on a desktop (`make apps-platform`) |

## Build and run

From `apps/client`, with the Flutter version of the root `pubspec.yaml`:

```sh
dart run tool/branding_defines.dart          # build/branding/dart_defines.json and native settings
flutter run -d macos --dart-define-from-file=build/branding/dart_defines.json
flutter build windows --dart-define-from-file=build/branding/dart_defines.json
```

The build branding comes from `branding.example.yaml` (neutral values under `example.com`);
`--input <file>` takes the `branding.yaml` of an instance instead (ADR 0044; the distribution of
M7 reads more of it). Without the defines a build uses the values of `branding.example.yaml`.
The app shows `branding.product_name` of the instance after E1, never a name of its own.

A release build on macOS needs a team signature: the release entitlements carry the keychain
access group of `flutter_secure_storage` (docs/spec/15-client-architecture.md 9). Debug builds run
without one.

Crash reports need a target in the build configuration (docs/spec/15-client-architektur.md 16):
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
the pinned messages; the agent session and the files follow (M4-21, M4-22).

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
Crash reports go through `sentry_flutter` only with a `CRASH_DSN` and `privacy.crash_reports` on;
`beforeSend` and `beforeBreadcrumb` remove every content, header, token and URL parameter
(D-M4-31-3). `test/telemetry/canary_test.dart` drives the collector, the sender and the Sentry
pipeline with a recording transport while a thread with canary content shows and asserts that no
canary value leaves the device; no test talks to a real Sentry service.
