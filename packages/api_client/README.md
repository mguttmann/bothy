# api_client

REST client of the chat server for the apps and the console: the API generated from
`docs/api/openapi.yaml` (openapi-generator `dart-dio` with `built_value`, never changed by hand)
and a small hand-written layer that every client package uses (docs/spec/06-api.md 14).

Kind: pure Dart package, a member of the Dart workspace at the repository root
(`resolution: workspace`, `publish_to: none`).

## Dependencies

Allowed members (docs/spec/15-client-architektur.md, 3.1; `tools/arch-check/rules.yaml`): none: no arrow of the diagram leaves this package.
Every other member is out of reach; `make arch` fails on a dependency or an import of it.

Further rules checked by `make arch`:

- no import of `package:flutter/`, `dart:ui` (rule 1).
- no native source (rule 4).

Packages: `dio`, `built_value`, `built_collection`, `one_of`, `one_of_serializer` and
`http_parser`, which the generated code imports; the hand layer needs nothing more.

## Layout

| Path | Content |
|---|---|
| `lib/api_client.dart` | the public library: exports the generated API and the hand layer |
| `lib/src/generated/` | generated: the API classes (`api/`), the models (`model/`), the serializers (`*.g.dart` from `build_runner`) and the generator's library entry `generated_api.dart`; out of the doc lint, the style lint, the format check and coverage (docs/DECISIONS.md D-M4-01-4) |
| `lib/src/hand/` | the hand layer, see below; every public symbol has a doc comment |
| `test/` | tests of the hand layer and the tolerance test G5 (`test/fixtures/g5/`) |
| `tool/generate.sh` | the generation and the gate check, called by the `Makefile` |
| `.openapi-generator-ignore` | paths the generator must never write |

## Regenerate

After a change of `docs/api/openapi.yaml`, from the repository root (needs Docker and Flutter):

```sh
make -C packages/api_client generate
```

The generator runs from the image pinned by digest in the `Makefile`, without network, into a
temporary folder; only `lib/src/generated/` is replaced, then `build_runner` builds the
serializers and `dart format` formats them. Commit the result with the spec change.

`make -C packages/api_client generate-check` (called by `make api-clients`, gate G4) does the
same in a temporary copy, compares every generated file byte for byte with the checked-in state
(a missing, extra or changed file fails with its path), and runs `dart analyze`: an error in
generated code or any finding in hand-written code fails. It never writes into the working tree.
Temporary folders come from `$TMPDIR` (default `/tmp`) and are removed on exit; Docker must be
able to mount them (Colima mounts only the home folder by default, so set `TMPDIR` below it).

## Hand layer

- `createApiClient` builds the generated `ApiClient` for one server address: base URL plus
  `/api/v1`, JSON responses, and the interceptors in a fixed order: headers, the JSON problem
  body of a download (`BinaryProblemInterceptor`, M4-22), the `authInterceptors` slot, problem
  mapping, read retry.
- Headers: `traceparent` on every request from the caller's `TraceContext`, with a new span ID
  per request; `Idempotency-Key` on `POST`, `PATCH`, `PUT` and `DELETE` from the request option
  `idempotencyKeyExtra`, never invented; `ResponseMeta.of` exposes `Correlation-Id`.
- Read retry: only `GET` and `HEAD`, only after a network error, a timeout, `429` or `503`
  (not `503 setup_required`), waiting `Retry-After` or a full-jitter backoff from
  `ReadRetryPolicy.fromBackoff(base, cap)`, which the apps fill from `client.outbox_retry_base`
  and `client.outbox_retry_cap`. Writes are retried by the outbox, never here.
- Problems: every response error reaches the caller as a `DioException` carrying an
  `ApiProblem` (`ApiProblem.of(e)`) with `status`, `code` (or `http_<status>` without a problem
  body), `reason`, `hint`, `errors`, `current`, `correlationId` and `retryAfter`.
- Events: `EventsPageDecoder.decode` and `fetchEvents` read an event page element by element; an
  unknown event type becomes `UnknownEvent` instead of failing the page.
  `EventsPageDecoder.decodeEnvelope` reads the `d` member of a gateway `dispatch` frame the
  same way.

Request bodies carry no JSON `null`: the generated serializers leave absent optional members
out (`test/null_free_test.dart`).

## What M4-09 plugs in

The `auth` package passes its interceptors as `authInterceptors`: DPoP proofs, the nonce retry
of `use_dpop_nonce`, token refresh and step-up. They run after the headers (so a retry gets a
fresh `traceparent`) and see the raw error before the problem mapping. The generated auth
interceptors of openapi-generator are not installed.

Run from the repository root: `make lint test coverage arch api-clients COMPONENTS=apps`. The
API pages come from `make docs` ([Dart API reference](../../docs/reference/dart-api.md)).
