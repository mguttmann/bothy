# API contracts

The files in this folder are normative (spec-first, ADR 0013). Server stubs, SDKs and the
Dart package of the apps are generated from them. Conventions:
[docs/spec/06-api.md](../spec/06-api.md).

| File | Content |
|---|---|
| [openapi.yaml](openapi.yaml) | REST API under `/api/v1` (OpenAPI 3.1), webhook events (`webhooks:`), all schemas, including those of the gateway |
| [asyncapi.yaml](asyncapi.yaml) | Gateway WebSocket (AsyncAPI 3.0): ops, events, close codes; bodies via `$ref` to openapi.yaml |
| [webhooks.md](webhooks.md) | Delivery, signature, retry and verification of webhooks; incoming webhooks |
| [redocly.yaml](redocly.yaml) | Lint rules for both files, including the requirement of `x-authz-action` and the comparison with the action catalogue |

## Check

Run all commands from the root folder of the repo. Redocly CLI 2 needs Node 22.12 or later
(https://redocly.com/docs/cli/, retrieved 2026-10-10 [verified]).

```sh
# both APIs from redocly.yaml (gate G1, docs/spec/06-api.md section 15)
npx -y @redocly/cli@2.62.1 lint --config docs/api/redocly.yaml

# only one file, with the same rules
npx -y @redocly/cli@2.62.1 lint docs/api/openapi.yaml --config docs/api/redocly.yaml

# second opinion on AsyncAPI; CI=true turns off the usage statistics of the AsyncAPI CLI
CI=true npx -y @asyncapi/cli@6.2.0 validate docs/api/asyncapi.yaml
```

Expected: Redocly without errors and warnings; the AsyncAPI CLI reports only the note that
AsyncAPI 3.1.0 exists. Without `--config`, Redocly uses only its default rules; the rules from
05-actions.md then do not apply.

The versions are pinned; Renovate raises them (ADR 0041). Whoever changes a rule in redocly.yaml
checks with a deliberately wrong copy of the spec whether the rule fires.

## Render

```sh
# REST reference as one HTML file
npx -y @redocly/cli@2.62.1 build-docs docs/api/openapi.yaml -o build/api/rest.html

# gateway reference as HTML
CI=true npx -y @asyncapi/cli@6.2.0 generate fromTemplate docs/api/asyncapi.yaml \
  @asyncapi/html-template@3 -o build/api/gateway --force-write --no-interactive
```

`build/` is not checked in. The docs site (ADR 0045) includes both outputs.

## Change

1. Change the spec first, then the code. Whether a change is additive or breaking is decided by
   06-api.md, section 13.
2. New operation: `operationId` in lowerCamelCase, `tags`, `x-authz-action` from 05-actions.md,
   `x-rate-limit-class` of the action, problem responses from `components.responses`, an example in
   the media type. Every POST and PATCH operation gets the parameter `IdempotencyKey` or
   `x-idempotency-exempt` with a reason code from 06-api.md, section 7. A new problem code goes
   first into `x-problem-codes` at the schema `Problem` and into the description of the response
   component whose `x-status` is its status. If no suitable action exists, it goes first into
   [docs/spec/actions.yaml](../spec/actions.yaml); `make catalog` generates the tables in
   05-actions.md and the lists in redocly.yaml from it. In the same commit the operation gets an
   entry under `later` in `services/server/api/operations.yaml` (enable list, PLAN P-04); it moves
   to `enabled` with the ticket that implements its handler, because only enabled operations are
   generated and registered.
3. New event: type and receive action first in [docs/spec/actions.yaml](../spec/actions.yaml),
   then `make catalog`; payload schema `<Name>Payload`,
   event schema `<Name>Event` as `allOf` from `EventEnvelope`, an entry in `Event` (durable),
   a message in asyncapi.yaml, if needed an entry under `webhooks:`.
4. No `null`, no type arrays, no `anyOf`, `oneOf` only with a discriminator. Repeated
   enum sets as a named schema.
5. Lint, regenerate the code, run the tests. `make api-tables` renews the generated tables in
   06-api.md and webhooks.md (markers `BEGIN GENERATED`); `make api-checks` checks in the gate
   that they and all copies of the contracts fit their sources (G2, G3,
   [tools/api-checks](../../tools/api-checks/README.md)).
