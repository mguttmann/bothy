# Changelog

All notable changes to the TypeScript SDK (`sdk/typescript`, npm package `bothy-sdk`) are
documented in this file. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this component adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). Its version is the
[`VERSION`](VERSION) file next to this one; release tags are `sdk-typescript/v<version>`
(docs/spec/18-release.md). Nothing is published yet (OQ-17).

## [Unreleased]

### Added

- Generated types `src/generated/openapi.ts` from `docs/api/openapi.yaml` (openapi-typescript
  7.13.0, standard settings).
- `make generate` and `make generate-check` (gate G4: drift check, `tsc --noEmit` with
  `strict`).
- `createApiClient`, an openapi-fetch 0.17 client with `toleranceMiddleware`, which removes
  `null` members of JSON responses (gate G5).
- `decodeEvent`, `knownEvents`, `isKnownEvent` and `KNOWN_EVENT_TYPES`, checked against the
  generated `Event` union at compile time; events of unknown type are classified by `type` only.
- Node.js 24 or later.
