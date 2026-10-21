# Changelog

All notable changes to the TypeScript SDK (`sdk/typescript`, npm package `bothy-sdk`) are
documented in this file. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this component adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). Its version is the
[`VERSION`](VERSION) file next to this one; release tags are `sdk-typescript/v<version>`
(docs/spec/18-release.md). Nothing is published yet (OQ-17).

## [Unreleased]

### Added

- `KNOWN_EVENT_TYPES` lists the personal event `gdpr.request_due` (R-AZ-35).
- Hand layer of docs/spec/11-erweiterungen.md 11.2 (M2-30) with the behaviour of the Python SDK:
  `client` (token or DPoP with nonce retry, `sdk.http_timeout`, retries of idempotent requests,
  deprecation warnings, `bindRun` for `Invocation-Token`), `auth`, `retry`, `errors`, `config`,
  `pages`, `gateway`, `invocation` (token check and inbox), `runs` (`Adapter`, `RunHandle`,
  `RunPatcher`, `renderContext`), `components`, `commands`, `outbox`, `heartbeat`.
- `webhooks`: Standard Webhooks verification of `v1a` and `v1` over the raw body within
  `webhook.receiver_tolerance`, also for `interactions_url`, proven on every case of
  `docs/api/webhook-test-vectors.json`.
- `testing`: FakeServer with faults and restart, `signWebhook`, the shared scenario functions and
  doubles; live runs of the scenarios against the real server in `test/live/`.
- Entry points `.`, `./core` (browser-safe) and `./testing`; `npm run build` writes `dist/`.
- typedoc 0.28.17: `npm run lint` fails on an undocumented export, `npm run docs` writes the
  reference to `build/reference/`.
- Dependencies `jose` and `ws`, dev dependencies `typedoc` and `@types/ws`.
- Generated types `src/generated/openapi.ts` from `docs/api/openapi.yaml` (openapi-typescript
  7.13.0, standard settings).
- `make generate` and `make generate-check` (gate G4: drift check, `tsc --noEmit` with
  `strict`).
- `createApiClient`, an openapi-fetch 0.17 client with `toleranceMiddleware`, which removes
  `null` members of JSON responses (gate G5).
- `decodeEvent`, `knownEvents`, `isKnownEvent` and `KNOWN_EVENT_TYPES`, checked against the
  generated `Event` union at compile time; events of unknown type are classified by `type` only.
- Node.js 24 or later.
