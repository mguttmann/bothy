# apps/console

Web console for administration, statistics and the terminal, built with Flutter Web (ADR 0028,
ADR 0070) and served by the server under `/console/` on the origin of the API. No chat in the
browser: the console has no route for spaces, channels, threads or messages, and a thread shows
its ID to open in the app.

## Pages

The shell of [01-product.md](../../docs/spec/01-product.md) 5.5 holds the navigation, the
header with the signed-in person and "Log out", and the page; below `consoleNarrowWidth` the
navigation is a drawer behind a menu button. The browser tab's title names the page. Before a
session the console shows the login E2 and E7 of 01 5.6 in the console form, the step-up directly
after the login and the version check.

- Administration: People, Groups and access grants, Devices, Agents and costs, and Audit host the
  screens A1 to A6 of `packages/admin_ui` (01 5.2), the same screens and controllers as the
  desktop app, at the full width of the browser (`AdminBody` with no maximum width). A detail adds
  its ID to the page's path (`/people/<id>`, `/groups/<id>`, `/agents/<id>`); the access grants
  are `/groups/grants`, and their target (a space or channel) stays out of the path, because the
  console has no content route (ADR 0028). The groups page has two more tabs (01 5.5 "Groups,
  grants"): "Permission hints" at `/groups/hints` and "Expiring soon" at `/groups/expiring`, the
  lists of 07 9.5 and 9.4 (`PermissionHintsView`, `ExpiringAccessView`); their filters stay in the
  widget state, so the paths name no space or channel. The agent card of a built-in agent adds
  "Edit", "Versions", "Change runner", "Disable" and "Continue setup"
  (`ConsoleAdmin.newAgentHost`), over the console's API only.
- Changing actions: every one shows the confirmation of `admin_ui` first; a
  `401 step_up_required` answer starts the step-up of the action in a dialog (passkey with user
  verification when the person has one, else password with one-time code, else the new OIDC
  login, which leaves the tab and does not repeat the change), and after it the request repeats
  once with the same `Idempotency-Key`. The challenge of the confirmation is created after the
  person answered the dialog, so a slow answer does not outlive it.
- View as person (from "Who has access?" or a person's detail): stores the view on the console
  session; a banner with name and remaining time stands on every page until "End view" or the end
  time, and `/people/view-as` shows the structure the person sees and the spaces they do not see,
  never content. When the server ends the view with a request after its end, the console reads its
  session again instead of asking for a step-up.
- Audit export: goes to a browser download when it has ended; an export that fails before the
  server sent a byte offers none. Codes and tokens the server shows once stay in the widget state
  only.
- Import (`/import`, web only, 01 3.7): the import sources of `admin_ui` (`ImportListPage`,
  `ImportSourcePage` at `/import/<id>`), every owner step of `import.manage` that the command-line
  tool cannot do, because a `cli` session never provides step-up (22 8): add a source from its
  YAML mapping file (the browser's file input, converted to JSON in the page, nothing stored),
  start, pause, cancel and complete it, and change the lead of a channel with the operator step
  that follows. The page reads the source again after each step, on its refresh control and when
  the tab becomes visible; it has no timer and no gateway connection.
- Hosts and terminal (`/hosts`, a host at `/hosts/<id>`): screen A8 of `admin_ui` (hosts, pairing
  wizard, session users) with the step-up of every change, and the page of a host offers "Open
  terminal" (see Terminal below).
- Live: the views of `LiveScreen` (see `lib/viz/` below). A run or chat opens a dialog with the
  thread ID and a copy action, because the console has no chat and discovery names no link into
  the apps.
- Operations (Grafana), owners only (`/grafana`, 25 9.2): only when `getInstance` names
  `grafana_embed_url` (the server sends it to owners only, with `console.grafana_embed_url` set),
  the navigation ends with that entry; the page states that it follows the permissions of
  Grafana, fills its content area with the dashboard in an iframe (`HtmlElementView` with
  `package:web`, `sandbox="allow-scripts allow-same-origin"`, `referrerpolicy="no-referrer"`) and
  opens it in a new tab on request. The server's CSP then names exactly the origin of the
  dashboard in `frame-src`; the console passes no identity to Grafana. Setup:
  [grafana-embedding.md](../../docs/operations/grafana-embedding.md).

## Content Security Policy

The console runs under the Content Security Policy of
[19-security.md](../../docs/spec/19-security.md) 5, which the server sets with a nonce per
response. `web/index.html` has no `<base>` element (`base-uri 'none'`) and carries the placeholder
`__BOTHY_CSP_NONCE__` in the nonce attribute of the bootstrap script tag, which the server
replaces; `web/flutter_bootstrap.js` derives the entry point, the assets, CanvasKit and the
fallback fonts from its own URL (all on the own origin) and hands the nonce to the loader;
`ConsolePlatformLocation` (`lib/run.dart`) gives the path URL strategy its base `/console/`.
`make console-e2e` (`tools/console-e2e`) proves the console under the policy in Chromium, Firefox
and WebKit (SEC-T05).

## Layout

| Path | Content |
|---|---|
| `lib/main.dart`, `lib/run.dart` | start: semantics on, path URLs, the app |
| `lib/app.dart`, `lib/routes.dart` | theme from `design_system`, texts from `l10n`, the route table `ConsolePage` and its router |
| `lib/admin/` | the places of the administration (`ConsoleRoute`), the page frame that hosts `AdminBody` (`ConsoleAdminPage`), view as person (`ConsoleAdmin`, `ConsoleViewAsPage`, `ConsoleViewAsBanner`), the step-up of an action (`ConsoleActionStepUp`, `ConsoleStepUpInterceptor`, `ConsoleStepUpDialog`) and the download target of the audit export |
| `integration_test/admin/` | the `patrol` web flows A1 to A10, the import (`import_test.dart`, M8-08), "Live" (`live_test.dart`, M6-16) and the status page (`status_test.dart`, M7-21) and their hand-written web bundle `admin_bundle.dart`, run by `make console-e2e` |
| `lib/terminal/` | "Hosts and terminal" (`ConsoleHostsPage`), the open terminals of the tab (`ConsoleTerminals`) and the ports of `terminal_ui` in the console: `ConsoleTerminalApi`, `ConsoleTerminalTransport`, `IndexedDbHostKeyPinStore`, `PasskeyStepUpPrompt`, `ConsoleTerminalPrompts`, over the browser seams of `ConsoleTerminalPorts` (`ports_web.dart` in the web build, an inert stub elsewhere) |
| `lib/session/` | `ConsoleController`: start, session from `getConsoleSession`, login, step-up, logout, E7 |
| `lib/login/` | E2 with the link to the public status page, the step-up screen, E7, the public status page `/console/public-status` (`PublicStatusPage`, `getPublicStatus` without login; "only after login" for the server's `404` unless `status.public_enabled`; M7-21), the OIDC flow keys and the WebAuthn JSON conversion |
| `lib/http/` | `createConsoleApiClient`: cookie of the browser, `X-Console-Request: 1` on state-changing requests, no `Authorization`, no `DPoP` |
| `lib/platform/` | the browser ports (`sessionStorage`, location, `navigator.credentials`, the Page Visibility of the tab); `browser_web.dart` in the web build, an inert stub elsewhere |
| `lib/shell/` | the shell with the navigation, the status view and the thread link |
| `lib/ops/` | "Operations (Grafana)" (M6-17): `ConsoleGrafanaPage` and `ConsoleGrafanaFrame`, the sandboxed iframe (`grafana_frame_web.dart` in the web build, an inert box elsewhere) |
| `lib/viz/` | "Live" (M6-16): `ConsoleLivePage` over `LiveScreen` of `admin_ui` with TV mode, and `PollingLiveFrameSource`, which polls `getStatsLive` every `viz.live_poll_interval`, stops while the tab is hidden (`PageVisibility`), ends on `feature_disabled`, `forbidden` or `not_found` and backs off up to `viz.stale_after` on other failures |
| `lib/version/` | the check of `version.json` against `Discovery.spec_version` |
| `web/` | `index.html` (no `<base>`, the nonce placeholder) and the loader configuration `flutter_bootstrap.js` (no service worker, every URL from its own, the nonce) |
| `tool/version_json.sh` | writes `build/web/version.json` after the build |

Screens: [01-product.md](../../docs/spec/01-product.md), sections 5.2 and 5.5. The screens
come from `packages/admin_ui` and are the same as in the desktop app. The page `/status` hosts the
status page of 16 12 with the owner forms (M7-21); the console has no sync engine, so it reads
`getStatus` again every `client.status_poll_interval` of `AppConfig` (`ConsoleAdmin.statusSource`)
and shows "Server unreachable" when a read gets no answer.

## Session

The console holds no token. It logs in with the console form of the login operations
(`client: console`), and the server sets the cookie `__Host-console_session`, which the browser
sends with every request of the same origin (docs/spec/08-identity.md 7.5). Before any
administration page the session needs a step-up: passkey with user verification, else a new OIDC
login, else password with TOTP. A `401` during a session leads to E7 with the reason; a
`step_up_required` of a signed-in console starts the step-up of the action (M6-04, above).

## Terminal

The terminal view is the one of the apps (`packages/terminal_ui`, M4-35) with the console's ports
(docs/spec/15-client-architecture.md 23.1, ADR 0070):

- Stream: the browser `WebSocket` (binary type `arraybuffer`) to the `stream_path` of the grant on
  the same origin (`wss` for an `https` page), allowed by `connect-src 'self'`. The browser sends
  the console cookie and the page's `Origin` with the upgrade; text messages until `identify`,
  then only binary. `identify` carries the protocol version and the device of the grant
  (`ConsoleTerminalApi` records `session.device_id` per `stream_path`, because the console's
  session answer has none), never an access token or a DPoP proof.
- Key: the ephemeral X25519 key of each handshake comes from `bothy_noise` with `cryptography`
  and lives only in the memory of the tab; the controller destroys it after message 2.
- Pins: per host the public key, key generation, fingerprint and first seen, in the IndexedDB
  database `bothy.terminal.pins` of the origin (one per browser profile). The console deletes the
  whole database before it shows E7: at the logout, at a `401` that ends the session (the
  clearing cookie) and at a start without a session.
- Pinning statement: below the dialog "Open terminal" and the key change warning the console
  states that in the browser the check only notices a key change that the server reports and does
  not protect against the server itself (24 5.1): the server delivers the console's code.
- Step-up: `terminal.open` and `terminal.attach` always confirm with the passkey and
  `userVerification: required`, never with password and TOTP or a new OIDC login. Without a
  passkey the console shows how to get one and opens nothing. The client decides no freshness:
  the server answers `401 step_up_required` with `reason: passkey_required`, also for an attach
  within `terminal.quick_reattach_window` (no quick reattach in the console), and the view asks
  again. The console's action step-up never answers these two requests.
- Host events: none. A8 and the view read the host through REST when they open and after each own
  action; there is no live update.
- Keyboard: the terminal takes Tab, Shift+Tab and F6 for the shell; Ctrl+Shift+F6 (Cmd+Shift+F6
  on Apple systems) moves the focus to the buttons above it, and the view names these keys.
- Sessions: the view knows only the session its tab opened; there is no session list yet, so a
  session a reload leaves behind stays detached on the host until it ends there (24 12, build
  state).

The controls of the view have stable keys for the `patrol` web test (`ConsoleTerminalKeys`:
`console.terminal.open`, `.pane`, `.detach`, `.reattach`, `.end`, `.close`).

## Dependencies

Members (docs/spec/15-client-architecture.md 3.1; `tools/arch-check/rules.yaml`): `admin_ui`
(the screens A1 to A6 and A8), `terminal_ui` (the terminal view and its ports), `api_client`, `auth` (`PkcePair`, `StepUpCancelled` and the purpose key
of a step-up), `design_system`, `l10n` and `domain` (`Uuid7Generator` for the
`Idempotency-Key`), all reachable through `admin_ui`. Never
`sync_engine` or `local_store`, also not through another member (rule 3); `make arch` fails
otherwise. External: `dio`, `web` (browser interop), `built_value`, `built_collection`; for tests only
`patrol` with `integration_test`.

## Build and serve

From `apps/console`:

```sh
flutter build web --release --base-href /console/ --no-web-resources-cdn
sh tool/version_json.sh ../../docs/api/openapi.yaml ../../services/server/VERSION build/web/version.json
```

Point `console.assets_dir` (`BOTHY_CONSOLE__ASSETS_DIR`) of the server at `build/web`; the
server image does both steps itself (`make image`). A build without the version step shows the
update notice, because its version cannot be compared.

## Test

- `flutter analyze --fatal-infos && flutter test --coverage`: routes, semantics, HTTP layer,
  login flows with a fake server, version check and the goldens of E2, the step-up and E7; the
  places, pages, view as person, the step-up adapter and dialog of the administration
  (`test/admin/`) and their goldens at a browser width; the terminal ports with a fake WebSocket
  and a fake IndexedDB, the passkey step-up, the pinning statement and the hosts page
  (`test/terminal/`); the live source with a fake clock and the page "Live" (`test/viz/`); the page "Operations
  (Grafana)" (`test/ops/`).
- `make console-e2e` (phase 2): the `patrol` web flows of `integration_test/admin/` in Chromium
  under the enforced CSP against the Compose stack (`tools/console-e2e/README.md`).
- `make console-web-test` (from the repository root; `flutter test --platform chrome test/web`
  here): the browser adapters, IndexedDB of the terminal pins and the attributes of the Grafana
  iframe in a real browser; CI runs it in the job `test (apps)`.
- From the repository root: `make lint test coverage arch COMPONENTS=apps`.

## Platforms

Current versions of Chromium browsers, Firefox and Safari (15 2), JavaScript build.

## Rulings

The decisions behind the console are recorded per ticket in `docs/decisions.d/`: `M6-02.md`
(shell, login, step-up, version check), `M6-03.md` (CSP), `M6-04.md` (administration, step-up of an action, view as person), `M6-17.md`
(Grafana), `M6-18.md` (hosts and terminal) and `M8-08.md` (import).
