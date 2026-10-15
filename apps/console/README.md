# apps/console

Status: skeleton (M4-01): `lib/main.dart` starts an empty `MaterialApp` without text, and
`test/app_test.dart` pumps it. M6-01 builds the console on it.

Web console for administration and statistics, built with Flutter Web (ADR 0028). No chat in
the browser; links to threads open the app.

## Purpose

- People, groups, grants with a plain-text preview, "Who has access?", "View as person".
- Agents with backend policy and costs, apps and webhooks, import, instance and branding,
  status page.
- Audit log with verification of the hash chain, statistics, terminal (ADR 0070).

Screens: [01-produkt.md](../../docs/spec/01-produkt.md), sections 5.2 and 5.5. The screens
come from `packages/admin_ui` and are the same as in the desktop app.

## Dependencies

Allowed members (docs/spec/15-client-architektur.md, 3.1; `tools/arch-check/rules.yaml`):
`admin_ui`, `device_key` (WebCrypto), `terminal_ui` and every member they reach. Never
`sync_engine` or `local_store`, also not through another member (rule 3); `make arch` fails
otherwise.

## Technology

- Login with WebAuthn passkeys or OIDC; DPoP with a non-extractable WebCrypto key; step-up by
  passkey ([15-client-architektur.md](../../docs/spec/15-client-architektur.md), sections 8
  and 9).
- JavaScript build; semantics enabled at start (accessibility).
- Served by the server under a path of its own, same origin as the API (operations:
  [16-betrieb.md](../../docs/spec/16-betrieb.md)).
- The `web/` folder comes with M6-01 and holds no native source (rule 4).

## Platforms

Current versions of Chromium browsers, Firefox and Safari.

## Build and test

From the repository root: `make lint test coverage arch COMPONENTS=apps`.
