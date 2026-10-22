# apps/console

Web console for administration, statistics and the terminal, built with Flutter Web (ADR 0028,
ADR 0070) and served by the server under `/console/` on the origin of the API. No chat in the
browser: the console has no route for spaces, channels, threads or messages, and a thread shows
its ID to open in the app.

Status (M6-02): the shell with the navigation of
[01-product.md](../../docs/spec/01-product.md) 5.5 (every page a placeholder until its ticket
fills it), the login E2 and E7 of 01 5.6 in the console form, the step-up directly after the
login, and the version check. Rulings: `D-M6-02-0` to `D-M6-02-9` in `docs/DECISIONS.md`.

## Layout

| Path | Content |
|---|---|
| `lib/main.dart`, `lib/run.dart` | start: semantics on, path URLs, the app |
| `lib/app.dart`, `lib/routes.dart` | theme from `design_system`, texts from `l10n`, the route table `ConsolePage` and its router |
| `lib/session/` | `ConsoleController`: start, session from `getConsoleSession`, login, step-up, logout, E7 |
| `lib/login/` | E2, the step-up screen, E7, the OIDC flow keys and the WebAuthn JSON conversion |
| `lib/http/` | `createConsoleApiClient`: cookie of the browser, `X-Console-Request: 1` on state-changing requests, no `Authorization`, no `DPoP` |
| `lib/platform/` | the browser ports (`sessionStorage`, location, `navigator.credentials`); `browser_web.dart` in the web build, an inert stub elsewhere |
| `lib/shell/` | the navigation, the placeholder pages and the thread link |
| `lib/version/` | the check of `version.json` against `Discovery.spec_version` |
| `web/` | `index.html` and the loader configuration `flutter_bootstrap.js` (no service worker) |
| `tool/version_json.sh` | writes `build/web/version.json` after the build |

Screens: [01-product.md](../../docs/spec/01-product.md), sections 5.2 and 5.5. The screens
come from `packages/admin_ui` and are the same as in the desktop app.

## Session

The console holds no token. It logs in with the console form of the login operations
(`client: console`), and the server sets the cookie `__Host-console_session`, which the browser
sends with every request of the same origin (docs/spec/08-identity.md 7.5). Before any
administration page the session needs a step-up: passkey with user verification, else a new OIDC
login, else password with TOTP. A `401` during a session leads to E7 with the reason, a
`step_up_required` back to the step-up.

## Dependencies

Members (docs/spec/15-client-architecture.md 3.1; `tools/arch-check/rules.yaml`): `api_client`,
`auth` (only `PkcePair`), `design_system` and `l10n`, all reachable through `admin_ui`. Never
`sync_engine` or `local_store`, also not through another member (rule 3); `make arch` fails
otherwise. External: `dio`, `web` (browser interop), `built_value`, `built_collection`.

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
  login flows with a fake server, version check and the goldens of E2, the step-up and E7.
- `flutter test --platform chrome test/web`: the browser adapters in a real browser.
- From the repository root: `make lint test coverage arch COMPONENTS=apps`.

## Platforms

Current versions of Chromium browsers, Firefox and Safari (15 2), JavaScript build.
