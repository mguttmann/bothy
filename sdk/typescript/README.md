# sdk/typescript

Status: generated types, runtime client and tolerance layer (M2-23). The rest of the hand layer
and the build output follow with M2-30. The package is private and not published (OQ-17).

## Purpose

TypeScript SDK for apps, integrations and agent adapters in JavaScript environments.

## Layout

| Path | Content |
|---|---|
| `src/generated/openapi.ts` | Generated types (`paths`, `components`, `operations`, `webhooks`) of `docs/api/openapi.yaml`, written by openapi-typescript 7.13.0. Never edited by hand. |
| `src/tolerance.ts` | Tolerance layer (gate G5). |
| `src/index.ts` | Entry: the generated types, the tolerance API and `createApiClient`. |
| `test/` | G4 and G5 tests (Vitest); the fixtures are shared with the Python SDK in [`../testdata/tolerance/`](../testdata/tolerance/README.md). |

npm package `bothy-sdk` with `"private": true` (working name, docs/spec/02-architecture.md
"Places that carry the product name").

## Regenerate

After every change of `docs/api/openapi.yaml`:

```sh
make -C sdk/typescript generate        # rewrite src/generated/openapi.ts
make -C sdk/typescript generate-check  # gate G4: no drift, tsc --noEmit (strict)
```

`make api-clients` runs `generate-check` in every `make check`; `make bootstrap-ci` installs the
locked tools with `npm ci --ignore-scripts`. The generator is pinned exactly in `devDependencies`
and locked in `package-lock.json`; Renovate updates it in the group "SDK code generators".

## Runtime client

`createApiClient(options)` creates an [openapi-fetch](https://openapi-ts.dev/openapi-fetch/)
client for `paths` and always registers `toleranceMiddleware`. Authentication, DPoP, timeouts and
retries come with M2-30.

## Tolerance (gate G5)

- Unknown field and unknown enum value: TypeScript has no runtime validation, so both arrive
  unchanged.
- Unknown event type: the page is passed through as received; `knownEvents(page)` returns the
  known events in order and `decodeEvent(raw)` classifies one event by `type` alone. Advance with
  `next_cursor`.
- JSON `null`: `toleranceMiddleware` removes object members whose value is `null` from
  `application/json` responses and every `application/*+json` type (Problem Details, JWKS).
- `KNOWN_EVENT_TYPES` is checked against the generated `Event` union in both directions at
  compile time, so `generate-check` fails until the list follows a spec change.

Not guaranteed: types do not narrow an unknown enum value (it is a string outside the literal
union), and `null` items of an array stay. Unknown gateway ops and close codes are handled by the
gateway module of M2-30.

Difference to the Python SDK: there the generated page already skips unknown events; here
`knownEvents` or `decodeEvent` skip them.

## Tool chain and runtimes

- `engines.node = ">=24"`: on 2026-10-12 Node.js 24 is the only line in active LTS.
- TypeScript 5.9.3 with `strict`, Prettier, ESLint 9 with typescript-eslint (type-checked
  recommended rules), Vitest with the v8 coverage provider (`coverage/lcov.info`). ESLint 10 and
  typescript-eslint from 8.56.1 on need minimatch 10.1 or later (BlueOak-1.0.0), which the license
  allowlist does not contain.
- Coverage of the hand-written code: `quality.coverage.typescript_min_percent`
  (`coverage.thresholds` in `vitest.config.ts`); generated code is outside coverage, Prettier and
  ESLint, but inside `tsc`.

## Next (M2-30)

Hand layer modules after docs/spec/11-extensions.md 11.2, the webhook signature verifier, the
typedoc reference and gate, and the build output (`dist/`).
