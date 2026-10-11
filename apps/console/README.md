# apps/console

Status: planned (M0). No code yet.

Web console for administration and statistics, built with Flutter Web (ADR 0028). No chat in the
browser; links to threads open the app.

## Purpose

- People, groups, grants with a plain-language preview, "Who has access?", "View as person".
- Agents with backend policy and costs, apps and webhooks, import, instance and branding,
  status page.
- Audit log with a check of the hash chain, statistics.

Screens: [01-product.md](../../docs/spec/01-product.md), sections 5.2 and 5.5. The screens come
from `packages/admin_ui` and are the same as in the desktop app.

## Technology

- Packages: `design_system`, `l10n`, `domain`, `api_client`, `auth`, `admin_ui`, `device_key`
  (WebCrypto). Not: `sync_engine`, `local_store` (architecture test).
- Login with WebAuthn passkeys or OIDC; DPoP with a non-extractable WebCrypto key;
  step-up by passkey ([15-client-architecture.md](../../docs/spec/15-client-architecture.md),
  sections 8 and 9).
- JavaScript build; semantics enabled at startup (accessibility).
- Served by the server under its own path, same origin as the API (operations:
  [16-operations.md](../../docs/spec/16-operations.md)).

## Platforms

Current versions of Chromium browsers, Firefox and Safari.
