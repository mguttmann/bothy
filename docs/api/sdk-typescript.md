# TypeScript SDK

The TypeScript SDK `sdk/typescript` (docs/spec/11-erweiterungen.md section 11, ADR 0026) for
agent adapters and apps on Node.js. Its [API reference](typescript-reference/index.html) is
generated from the TSDoc comments of the source with typedoc in every `make docs`; status, layout
and how to run the tests are in [sdk/typescript/README.md](../../sdk/typescript/README.md). The
Python SDK has the same modules and runs the same scenarios ([Python SDK](../sdk/python.md)).

## Two layers

The generated core `src/generated/openapi.ts` holds the types of every REST operation, schema
and webhook; openapi-typescript writes it from [openapi.yaml](openapi.yaml), and it is never
edited by hand. The hand layer on top of it calls REST through openapi-fetch with the tolerance
layer (unknown fields, enum values and event types are tolerated, a JSON `null` counts as absent)
and adds what every adapter and app needs. It declares no own type for an object the core
generates; a test holds that.

The generated core has no section in the API reference: [openapi.yaml](openapi.yaml) is its
documentation, and the site renders it as the pages "REST reference", "Schemas" and "Gateway
reference".

## Entry points

The package is ESM only and needs an active LTS line of Node.js (`engines` of its
`package.json`). `npm run build` writes the build output to `dist/`; `package.json` maps three
entry points into it:

| Import | Content | Runs in |
|---|---|---|
| `bothy-sdk` | everything except `testing`: the client with DPoP, retries and the run binding of `Invocation-Token`, the gateway, the invocation check and inbox, the agent loop, outbox, heartbeat, components, commands and the webhook verifier | Node.js only, because it handles tokens and keys |
| `bothy-sdk/core` | the browser-safe subset: generated types, tolerance layer, errors, component builders, `renderContext`, `uuidv7`, `pages` | Node.js and browsers; it imports no `node:` module and none of `auth`, `invocation`, `gateway`, `webhooks`, `outbox` (`test/packaging.test.ts`) |
| `bothy-sdk/testing` | the FakeServer, `signWebhook`, the shared scenario functions and their test doubles | tests on Node.js |

The package stays `private` and is not published; the name is a placeholder until the final
name is decided (OQ-17).

## Modules

| Module | Content |
|---|---|
| `config` | Named values of the SDK (`SdkSettings`, `DEFAULTS`, `SETTING_KEYS`); each default is the one of its key in the [configuration reference](../operations/configuration.md), held by a test. |
| `errors` | Typed errors after the problem format; an unknown problem code is classified by its status, and `404` means "not there or not visible". |
| `client` | `createApiClient` with token, DPoP proofs and nonce, per-attempt timeout, retries of idempotent operations, deprecation warnings and the run binding of `Invocation-Token`. |
| `auth` | DPoP keys from a file or a key store, proofs and the `jkt` thumbprint; `Secret` keeps tokens out of logs. |
| `retry` | Classification of retryable answers, the idempotency table and the backoff. |
| `pages` | Iterator over `next_cursor` pages. |
| `gateway` | Gateway consumer (`hello`, `identify`, `resume`, heartbeat, reconnect per close code) and the polling consumer. |
| `invocation` | Check of the invocation token with the JWKS cache, the `jti` store and the inbox. |
| `runs` | The agent loop `Adapter`, the run handle, the run patcher and `renderContext` with trust marks. |
| `components` | Builders of buttons, selects, forms, approval cards and questions within the component limits. |
| `commands` | Declared commands reconciled on connect, autocomplete with a guard against late answers. |
| `outbox` | Messages and writes stored before sending, resent with the same `client_msg_id` or `Idempotency-Key`. |
| `heartbeat` | The `agent_heartbeat.v1` body, which rejects fields that could carry secrets. |
| `webhooks` | Verification of outgoing webhooks and `interactions_url` requests (`v1a` and `v1`). |
| `testing` | FakeServer, `signWebhook`, scenario functions and doubles. |

## Webhook verification

An app verifies every delivery before it reads the body, as [webhooks.md](webhooks.md) section 3
describes: the signed content is `{webhook-id}.{webhook-timestamp}.{body}` over the raw body
bytes, `webhook-timestamp` must lie within `webhook.receiver_tolerance` of the own clock, and one
valid entry of `webhook-signature` suffices. `v1a` entries are checked against the Ed25519 keys
of `getWebhookKeys`, which `WebhookKeyCache` holds as a trust list and refreshes only after the
`max-age` of the response, never because of an incoming request. `v1` entries are checked against
the app's `whsec_` secrets in constant time. Requests to `interactions_url` are signed the same
way and are verified with the same function.

```ts
import {
  WebhookKeyCache,
  WebhookVerificationError,
  createApiClient,
  verifyWebhookRequest,
} from "bothy-sdk";

const cache = new WebhookKeyCache({
  client: createApiClient({ baseUrl: "https://chat.example.com/api/v1" }),
});

export async function handle(request: Request): Promise<Response> {
  try {
    const verified = await verifyWebhookRequest(request, {
      keys: await cache.keys(),
      secrets: process.env.WEBHOOK_SECRET ? [process.env.WEBHOOK_SECRET] : [],
    });
    // verified.id is the key of the app's own duplicate check (webhooks.md 3 point 4).
    console.log(verified.event.type, verified.syncEpoch);
    return new Response(null, { status: 204 });
  } catch (error) {
    if (error instanceof WebhookVerificationError)
      return new Response(null, { status: 401 });
    throw error;
  }
}
```

A failed verification raises `WebhookVerificationError` with one code of `missing_header`,
`invalid_timestamp`, `timestamp_out_of_tolerance`, `no_trusted_key` and `signature_mismatch`;
its message never contains the body, a signature or a secret. The SDK stores no `webhook-id`:
the app keeps the IDs it has seen for `webhook.retry_max_age` in its own storage.

## Testing with the FakeServer

`bothy-sdk/testing` starts an in-process FakeServer on `127.0.0.1` with port 0 for REST and the
gateway. It models the server rules the SDK depends on (agents and API tokens with DPoP, consumer
cursors with acknowledgement and redelivery, a fresh invocation token per delivery, the run
operations with their answers, patches with the path table and `base_version`) and injects
faults per operation (`stored_then_502`, `stored_late`, `rate_limited` and `disconnect`, the
names of the Python FakeServer) and restarts on the same state. `signWebhook` builds the `webhook-signature` value of a delivery for an app's own
tests.

## Live runs against the real server

The shared scenarios AG-08 (adapter side), AG-09, AG-12, AG-28 and AZ-P6 of
[sdk/testdata/scenarios](../../sdk/testdata/scenarios/README.md) run against the FakeServer and,
in `sdk/typescript/test/live/`, against the real server of this repository: the test setup builds
the `sdkserver` test binary of `services/server/test/sdkserver` and drives it over its stdin
protocol, as the Python SDK does. The live files are part of `npm test`, so `make test` and
`make check` run them; without Docker or the Go tool chain they fail and never skip.

## Building the reference

`npm --prefix sdk/typescript run docs` writes the HTML reference to
`sdk/typescript/build/reference/`; `make docs` runs it before MkDocs, and the hook
`tools/docs/mkdocs_sdk_reference.py` mounts it under `api/typescript-reference/`. `npm run lint`
runs ESLint and then `npm run docs:check`, which fails on every export without a TSDoc comment.
