# sdk/typescript

Status: generated types, runtime client and tolerance layer (M2-23); the hand layer of
docs/spec/11-extensions.md 11.2 with the webhook verifier, the FakeServer, the shared scenarios
against the FakeServer and the real server, the typedoc reference and the build output `dist/`
(M2-30). The package is private and not published (OQ-17).

## Purpose

TypeScript SDK for apps, integrations and agent adapters in JavaScript environments. It has the
modules and the behaviour of the Python SDK of M2-29 (`sdk/python`); class names are the Python
class names, function names the camelCase form of the Python names (docs/decisions.d/M2-30.md D-M2-30-1).

## Layout

| Path | Content |
|---|---|
| `src/generated/openapi.ts` | Generated types (`paths`, `components`, `operations`, `webhooks`) of `docs/api/openapi.yaml`, written by openapi-typescript 7.13.0. Never edited by hand. |
| `src/tolerance.ts` | Tolerance layer (gate G5). |
| `src/index.ts` | Node.js entry `bothy-sdk`: everything except `testing`. |
| `src/core.ts` | Browser-safe entry `bothy-sdk/core`. |
| `src/testing/index.ts` | Entry `bothy-sdk/testing`: FakeServer, `signWebhook`, scenarios and doubles. |
| `src/<module>/` | One folder per hand-layer module with an `index.ts` (table below). |
| `test/` | Unit tests (Vitest) against the FakeServer or a fake `fetch`; G4 and G5 tests with the fixtures shared with the Python SDK in [`../testdata/`](../testdata/tolerance/README.md). |
| `test/live/` | The shared scenarios against the real server (`services/server/test/sdkserver`). |

npm package `bothy-sdk` with `"private": true` (working name, docs/spec/02-architecture.md
"Places that carry the product name").

## Entry points and Node line

ESM only; `engines.node = ">=24"` (on 2026-10-12 Node.js 24 is the only line in active LTS).
`npm run build` runs `tsc -p tsconfig.build.json` and writes ESM, declarations and source maps to
`dist/` (ignored by git); `package.json` maps `.`, `./core` and `./testing` into it.

- `bothy-sdk`: Node.js only, because it handles tokens and keys.
- `bothy-sdk/core`: generated types, tolerance layer, errors, settings, `components`,
  `renderContext`, `uuidv7` and `pages`; it imports no `node:` module and none of `auth`,
  `invocation`, `gateway`, `webhooks` and `outbox` (`test/packaging.test.ts`).
- `bothy-sdk/testing`: tests on Node.js; the only importer of `ws`.

## Modules

| Module | Content |
|---|---|
| `config` | `SdkSettings` and `DEFAULTS`, one member per configuration key the SDK uses; `test/config-defaults.test.ts` holds them equal to `docs/operations/configuration.md`. |
| `errors` | `ProblemError` and its subclasses after the problem format, `TransportError`, `TimeoutError`, `KNOWN_PROBLEM_CODES`; a `404` means "not there or not visible". |
| `client` | `createApiClient` with `Bearer` or DPoP, `DPoP-Nonce` and one nonce retry, `sdk.http_timeout`, retries of idempotent requests, one deprecation warning per operation, `bindRun` for `Invocation-Token` (07 SR9). |
| `auth` | `Secret`, `DpopKey`, `loadDpopKey`, `createDpopProof`, `dpopThumbprint`. |
| `retry` | `IDEMPOTENT_EXEMPT_OPERATIONS` (held equal to `openapi.yaml`), classification and backoff. |
| `gateway` | `Gateway` (hello, identify, resume, heartbeat, close codes, reconnect with full jitter), `GatewaySource` and `PollingSource` with the acknowledgement window `AckWindow`. `PollingSource` sends the heartbeat every `HeartbeatAck.interval_ms` and keeps its interval when an answer names none above zero. |
| `invocation` | `InvocationVerifier` (09 4.5 with its JWKS cache; a run from a model command compares the payload's `command_id`), `MemoryInboxStore` and `FileInboxStore` with the retention of "Adapter state" below. |
| `runs` | `Adapter` (09 4.9: acknowledgement after the inbox, start only after `200`), `RunHandle`, `RunPatcher`, `renderContext`. An end the server did not store is sent again ("The end of a run" below). |
| `components` | Builders, `approvalCard`, `questionCard`, `respond`, `update`. |
| `commands` | `reconcileCommands`, `guardedAutocomplete`; an autocomplete handler never calls a model. |
| `outbox` | `Outbox` with `MemoryOutboxStore` and `FileOutboxStore`; `sendMessage` takes `attachmentFileIds`. `sendLongMessage` keeps a long answer one message: the head as its text and the rest above `message_max_content_bytes` as one attachment `answer.txt` (`splitForLimit`, `uploadText`; M4-47). |
| `heartbeat` | `buildHeartbeat` and `HeartbeatSender`; `buildHeartbeat(state, { rateLimit: { state, resets_at } })` reports the provider's rate-limit state (M6-14; `resets_at` RFC 3339 with a time zone, never a quota, key or subscription name; a heartbeat without it clears it). |
| `webhooks` | `verifyWebhook`, `verifyWebhookRequest`, `WebhookKeyCache`, `parseWebhookPublicKey`, `parseWebhookSecret`. |
| `testing` | `FakeServer`, `signWebhook`, the scenario functions and doubles. |

## Webhook verification

```ts
import { WebhookKeyCache, createApiClient, verifyWebhookRequest } from "bothy-sdk";

const cache = new WebhookKeyCache({ client: createApiClient({ baseUrl: "https://example.com/api/v1" }) });
// The trust list is loaded outside the request path; a failed refresh keeps the last snapshot.
let keys = await cache.keys();
setInterval(() => {
  cache.keys().then((fresh) => (keys = fresh), () => undefined);
}, 60_000).unref();

export async function handle(request: Request): Promise<Response> {
  const verified = await verifyWebhookRequest(request, { keys });
  const event = verified.event;
  console.log(verified.id, event.kind === "known" ? event.event.type : event.type);
  return new Response(null, { status: 204 });
}
```

The verifier reads the raw body once, checks `webhook-timestamp` within
`webhook.receiver_tolerance`, tries every `v1a` entry against the trusted keys of
`getWebhookKeys` and every `v1` entry against the `whsec_` secrets in constant time, and throws
`WebhookVerificationError` otherwise. `WebhookKeyCache` refreshes only after the `max-age` of the
key list, never because of a request. The SDK stores no `webhook-id`: an app keeps the IDs it
has seen for `webhook.retry_max_age` in its own storage and answers a repeated one with `2xx`
without doing the work again (docs/api/webhooks.md 3 point 4).

## The end of a run

`RunHandle.finish` counts as done once the server answered. When `finishRun` gets no answer, or a
`5xx`, `401` or `429`, after the retries of the client, the server still holds the run as
running: `finish` throws, `finished` stays `false` and `finishPending` is `true`. The first end
asked for stays the run's end, so a later `finish` sends that end again, whatever it asks for.
The `Adapter` keeps such a run in `active_runs` of its heartbeat, so the server does not end it
as `interrupted` (09 5.5), and sends the end again with waits that double from
`sync.reconnect_backoff_base` up to `sync.reconnect_backoff_cap`, until the server stores or
refuses it. A run of an adapter that stops meanwhile is left to the server, like every run a
stopped adapter holds (09 4.9); the pending end is not written to disk.

## Adapter state

With `stateDir` the adapter keeps its inbox in `inbox/` (one file per accepted event with its
newest token, and `jti.json`) and its outbox in `outbox.json`. What leaves them:

- An inbox entry leaves a day after it was closed (`CLOSED_ENTRY_RETENTION_MS`, an option of the
  stores). `InboxStore.prune` drops it: the first prune that finds an entry closed starts its
  time, a redelivery of the event starts it again. An entry that is not closed stays.
- A `jti` row leaves with the same prune once its token cannot pass anymore (`exp` plus
  `adapter.invocation_token_leeway`).
- The adapter prunes at its start and every hour (`INBOX_PRUNE_INTERVAL_MS`). A new process does
  not know since when an entry is closed and counts from its own first prune.
- A sent outbox entry is deleted at once; a failed one stays until the caller deals with it
  (04 13.2 point 5: nothing is dropped silently).
- In memory the adapter keeps the token of a run only while it holds the run; a reply of an ended
  run is bound with the token of the inbox, or fails with `run_token_unavailable` once that
  entry has left.

## FakeServer and live runs

`FakeServer` (`bothy-sdk/testing`) listens on `127.0.0.1` port 0 for REST and the gateway and
models the server rules the SDK depends on, with the faults `stored_then_502`, `stored_late`,
`rate_limited` and `disconnect` and `restart()` on the same state. The scenario functions of
`src/testing/scenarios.ts` run in `test/scenarios.test.ts` against it and in
`test/live/scenarios.live.test.ts` against the real server: the Vitest project `live` builds
`services/server/test/sdkserver` once (`go test -c -tags sdkserver`) and drives it over its stdin
protocol. The live files are part of `npm test`; without the Go tool chain or a Docker-compatible
runtime they fail and never skip. `test/live/model-command.live.test.ts` runs a model command
against the real server: the adapter registers the command, Alex invokes it, and the run starts
and completes.

What the FakeServer plays beyond the run rules:

- `trigger(text, { command })` queues the run of a model command of Lotse
  (`trigger_kind = command`); its payload and tokens name the command.
- A follow-up (`deliverFollowup`) gets a row of the run, and a patch of
  `/followups/<message_id>/state` moves it forward only (`followupState`).
- `finishRun` with `costs` answers `400 validation_failed` (`/costs`, `not_enabled`) by default,
  the answer of a server without cost booking. `new FakeServer({ costs: true })` or
  `enableCosts()` accepts them as the server does since M2-22; `costs(runId)` returns them.
- `reportAgentSession` with the compare-and-swap over the session row (`sessionReports`),
  `getInstance` with `limits`, and the upload session of mode `direct` (`createUpload`,
  `uploadPart`, `completeUpload`, `abortUpload`; `uploadPartSize`, `upload(fileId)`), so
  `RunHandle.reportSession` and `sendLongMessage` run against it. `createMessage` refuses a
  content above `limits.message_max_content_bytes` and attaches completed uploads.

`test/fake-contract.test.ts` holds the FakeServer equal to `docs/api/openapi.yaml`: every route
is an operation of the contract with that method, path and `operationId`, and every answer of a
run over all routes has the form the contract gives for the operation and status (types,
required and unknown members, enums, patterns, bounds, UUIDs and timestamps). A new route fails
the test until a request of that file exercises it.

## Regenerate

After every change of `docs/api/openapi.yaml`:

```sh
make -C sdk/typescript generate        # rewrite src/generated/openapi.ts
make -C sdk/typescript generate-check  # gate G4: no drift, tsc --noEmit (strict)
```

`make api-clients` runs `generate-check` in every `make check`; `make bootstrap-ci` installs the
locked tools with `npm ci --ignore-scripts`.

## Tolerance (gate G5)

- Unknown field and unknown enum value: TypeScript has no runtime validation, so both arrive
  unchanged.
- Unknown event type: `knownEvents(page)` returns the known events in order and
  `decodeEvent(raw)` classifies one event by `type` alone (`UnknownEvent` stays acknowledgeable).
- JSON `null`: `toleranceMiddleware` removes object members whose value is `null` from
  `application/json` responses and every `application/*+json` type; the gateway drops them too.

## Reference and documentation gate

`npm run docs` writes the typedoc reference to `build/reference/` (ignored by git); `make docs`
runs it and mounts it under `api/typescript-reference/` of the documentation site.
`npm run lint` runs ESLint and then `npm run docs:check` (`typedoc --emit none` with
`notDocumented` as an error), so an export without a TSDoc comment fails `make lint`
(`tools/ci/tests/test_sdk_typescript_doc_gate.py`).

## Tool chain

- TypeScript 5.9.3 with `strict`, Prettier, ESLint 9 with typescript-eslint, Vitest with the v8
  coverage provider; typedoc 0.28.17. Runtime dependencies: `openapi-fetch`, `jose` and `ws`
  (exact versions in `package-lock.json`); webhooks and DPoP use `node:crypto`.
- Coverage of the hand-written code: `quality.coverage.typescript_min_percent`
  (`coverage.thresholds` in `vitest.config.ts`); generated code and `test/live/` are outside
  coverage, generated code also outside Prettier and ESLint, but inside `tsc`.
