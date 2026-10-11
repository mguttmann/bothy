# M1-18: HTTP middleware chain and problem details (ADR 0015, 0053)

Status: ready | Depends on: M1-11, M1-17 | Etappe: M1

## Goal

After this ticket every request on the public listener runs through one ordered chain: panic recovery, correlation, span and access log, security headers, routing, per-route limits and deadlines, CORS, authentication through `kernel/actor`, a strict JSON guard, preconditions, schema validation, and the proof that the handler called `authorize` with the route's action. Every error leaves the server as `application/problem+json` (RFC 9457, ADR 0015) with a code from one closed list, and no JSON response contains `null` (ADR 0053). Later tickets only add their stage (rate limits, maintenance mode, idempotency) and their handlers; the chain, its order and its error contract are fixed here.

## Read first

- `docs/PLAN.md`: section "M1 Fundament" (rows M1-17 to M1-21, M1-25, M1-28, M1-33, M1-34, M1-46, M1-64, M1-71), "Voraussetzungen aus dem Kern-Vertrag" (K2, K4), "Bekannte Widersprüche" (W1 with its name table, W7), "Entscheidungen dieses Plans" (P-08 to P-12).
- ADRs: `docs/adr/0015-fehlerformat-rfc-9457-idempotenz-paginierung.md`, `docs/adr/0017-eine-autorisierungsfunktion-404-statt-403-registrierungszwang.md`, `docs/adr/0013-spec-first-openapi-31-und-asyncapi-30-normativ.md`, `docs/adr/0000-vorlage-madr-4.md`; ADR 0053 (exists; add this ticket's tests to its "Confirmation").
- `docs/spec/02-architektur.md`: "Request-Pfad" (all three subsections: the chain order, the proof in the response writer, the rule on failed credentials), "Querschnitts-Ports (`internal/kernel`)" (`kernel/reqmeta`), "Fehlerarten" (closed public codes of 06, internal causes), "Fristen", "Wiederholungen", "Panics", "Konfigurationswerte dieses Kapitels" (`http.max_json_body_bytes`, `http.max_json_depth`, `http.unavailable_retry_after`).
- `docs/spec/06-api.md`: "Grundregeln" (rows Kein `null`, Ganzzahlen, Unbekanntes, Medientypen, Größe, Caching, Korrelation, CORS, Client-IP, Sprache), "Authentifizierung je Prinzipal-Art", "Autorisierung in der API", "Fehler" (with the generated problem table), "Nebenläufigkeit", "Vertragstests und Gate" (G7, G8), "Entscheidungen dieses Kapitels" (R-API-03, R-API-04, R-API-07, R-API-08), "Konfigurationswerte dieses Kapitels".
- `docs/spec/05-aktionen.md`: "Verbindliche Regeln" (rules 2, 5 and 9), "Entscheidung".
- `docs/spec/07-rechte.md`: "Prinzipal-Kontext", "Die Funktion `authorize`" ("Entscheidungen und Gründe", "Ablauf").
- `docs/spec/04-sync-protokoll.md`: "Umschlag (Envelope)". `docs/spec/03-datenmodell.md`: "Grundsätze" 10 and 13, column `event_log.correlation_id`.
- `docs/spec/17-observability.md` "Signale"; `docs/observability/signals.md` ("Kardinalität", metric table "REST und Gateway", "Traces" / "Weitergabe"); `docs/observability/alerting.md`; `infra/observability/prometheus/rules/bothy-alerts.yaml`.
- `docs/spec/19-sicherheit.md`: "HTTP-Sicherheitskopfzeilen" (SEC-HDR-1, SEC-HDR-2), "CORS und WebSocket-Herkunft" (SEC-CORS-1, SEC-CORS-2), "Eingabevalidierung im Server" (SEC-IN-1 to SEC-IN-3), "Darstellung in den Clients" (SEC-OUT-5), "Logs ohne Geheimnisse und ohne Inhalte" (SEC-LOG-2, SEC-LOG-3, SEC-LOG-5), "Tests" (SEC-T03, SEC-T04, SEC-T06).
- `docs/spec/08-identitaet.md`: "Schutz der Anmeldung" (client IP rule), test ID-T25. `docs/spec/16-betrieb.md`: "Zwei Listener", "Rand und Identität".
- `docs/security/threat-model.md`: rows T-IN-01, T-AZ-01, T-AZ-02, T-DS-01.
- `docs/api/openapi.yaml`: `components.responses`, `components.headers`, schemas `Problem`, `FieldError`, `Uuid`, `EventEnvelope`, `JsonObject`.
- Code from M1-05, M1-07, M1-10, M1-11 and M1-17: `internal/kernel/{errs,actor,authorize,ids}`, `internal/platform/{config,otel,httpserver,lifecycle}`, `api/routes`, `api/routes/gen`, `internal/wire`.
- `docs/quality/gates.md`, `CONTRIBUTING.md`.

## Fixed requirements that apply

1. No MVP. New from scratch and directly in the final state; stages are only the order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface with a docstring/doc
   comment (purpose, parameters, return value, errors, example where useful); comments explain the
   why, not the what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in MkDocs).
4. Documentation in the repo: README per module/service, ADR for every architecture decision,
   CHANGELOG per release, keep the documentation site current. A change without matching
   documentation is not done.
5. Quality gates (SonarQube, linters, type checking, security scanners, test coverage) block the
   merge.
6. Test first: write the failing test, show it red, then make it green. No en or em dashes in any text. White label everywhere. Configuration values only through named keys from `docs/operations/configuration.md`.
7. 02 "Request-Pfad" (B7, R-AR-14): every route runs through the chain; a successful response (2xx, 3xx) without an `authorize` check of the route's action is replaced by `500 internal_error` (cause `authz_not_checked` only in log and metric, 02 "Fehlerarten") and increments `bothy.authz.errors` with `bothy.reason = authz_not_checked` (signals.md, alarm `BothyAuthzError`).
8. 06 "Fehler" and ADR 0015: RFC 9457 body with `type` `/api/v1/problems/{code}`, `title`, `status`, `code`, `correlation_id`, optional `detail` and `instance`; extensions `errors[]`, `current`, `reason`, `hint`; `detail` never contains content, secrets or internal paths (SEC-OUT-5).
9. 06 "Grundregeln" and ADR 0053 (core contract K2): the server never sends JSON `null`; integers stay below 2^53.
10. 06 "Nebenläufigkeit" (K4): strong `ETag` with the version; a required `If-Match` that is missing gives `428 precondition_required`, a mismatch `412 version_conflict` with `current`.
11. 19 SEC-HDR-1 and SEC-HDR-2 on every API and error response (SEC-T03). HSTS belongs to M1-21.
12. 19 SEC-CORS-1 and SEC-CORS-2 for REST (SEC-T04, REST part). The WebSocket origin check belongs to M1-64.
13. 19 SEC-IN-1 (schema validation before the handler), SEC-IN-2 (duplicate keys, invalid UTF-8 and too deep nesting rejected), SEC-IN-3 (size limits before reading).
14. 06 "Grundregeln" row Client-IP and 08 "Schutz der Anmeldung": forwarding headers count only from peers in `server.trusted_proxies` (ID-T25, header part).
15. 17 "Signale", R-OB-05 and R-OB-07: `correlation_id` is the W3C trace ID of the server span; an incoming `traceparent` is only a span link unless `telemetry.traces.trust_remote_parent` (M1-10 configures `otelhttp` that way), so a client cannot choose its correlation ID; the response header `Correlation-Id` (schema `CorrelationId`) carries the same value.
16. W7 resolved as the plan recommends: 19 wins. Writing request schemas get `additionalProperties: false`; unknown request members give `400 validation_failed`; R-API-04 keeps applying to responses and events.

## Scope

Paths are relative to `services/server/` unless they start with `docs/` or `infra/`.

1. `internal/platform/httpserver` (extend what M1-11 created): chain with named stages, problem writer and error mapping, response writer (buffering, null repair, 2^53 guard, proof enforcement), JSON guard, client IP resolution, security headers, CORS, preconditions and ETag helpers, request validation, unmatched handler (404, 405), deadlines and body limits, panic recovery. `doc.go` and `README.md`.
2. `internal/platform/httpserver/httpservertest` (new): a harness that builds the full chain around fake `routes.Operation` values and test handlers, with a fake authenticator, helpers that record authorize checks, a manual OpenTelemetry metric reader and a log capture. M1-19, M1-20 and every handler ticket use it.
3. `internal/kernel/authorize`: the check log (Design notes "Proof").
4. `internal/kernel/errs`: kinds, codes and statuses aligned with 06 "Fehler"; header carriage on errors (Design notes "Errors").
5. `internal/kernel/reqmeta` (new kernel package, stdlib only, 02 "Querschnitts-Ports"): correlation ID and client IP of the request in the context. Register it in `architecture/modules.yaml` with the kernel-core packages. If M1-10 already exposes a correlation helper with the same purpose, reuse it and record that.
6. `internal/kernel/actor`: the `AnonymousOnly` authenticator and, if the M1-05 port lacks it, a `Credentials` input (Design notes "Authentication").
7. `api/routes/gen` (M1-17): the W7 check "request schemas closed" with a negative fixture; generation of `api/rest/openapi.gen.yaml`, a byte-identical copy of `docs/api/openapi.yaml` that the validator embeds (`//go:embed`; covered by `make codegen-check`).
8. `internal/wire`: assemble the chain for roles `api` and `all`; pass the problem writers to `routes.Build` as error handlers.
9. Spec and contract work in the same change. The spec already says what this ticket builds: 02 "Kette im api-Knoten", "Nachweis der Prüfung", "Fehlerarten" and the keys `http.max_json_body_bytes`, `http.max_json_depth`, `http.unavailable_retry_after`; 06 "Grundregeln" rows Unbekanntes, Korrelation and Größe, R-API-04; schema `CorrelationId` in `openapi.yaml`; metric `bothy.authz.errors` in signals.md and the alarm `BothyAuthzError` with its runbook anchor. Left to do here:
    - `docs/api/openapi.yaml` (W7): `additionalProperties: false` on every object schema reachable from a request body, except `JsonObject` (free form by design); where a schema is composed with `allOf`, close it in a way the validator and the generators accept (for example `unevaluatedProperties: false` on the composing schema) and prove it with the generator check of Scope 7; descriptions of `FieldError.pointer` and `FieldError.code` list the forms and the vocabulary below; fix every example that becomes invalid. `make api-lint` stays green.
    - ADR 0053: add the property test of this ticket to its "Confirmation"; then `make adr-index`.
    - A `promtool test rules` case for `BothyAuthzError` if M1-04 set up rule tests.
    - `docs/operations/configuration.md` regenerated if the schema adds keys; `services/server/CHANGELOG.md` and `internal/platform/httpserver/README.md`; `docs/DECISIONS.md` only for decisions that the spec does not already state.
10. Cursor codec for every list (06 "Paginierung" item 2, PLAN row M1-18) in `internal/platform/httpserver`: `EncodeCursor(list string, binding []byte, key any) (string, error)` and `DecodeCursor(list string, binding []byte, s string, key any) error`; opaque base64url without padding, versioned, carrying the list name, a SHA-256 fingerprint of the filter and sort binding (including the caller's principal ID) and the keyset values. A changed keyset value only moves the position inside the same bound and filtered list, so no signature is needed. A cursor of another list, another binding, another principal, a tampered or unknown version gives `400 cursor_invalid`. Every later list ticket uses it; no second codec.

## Out of scope

- Rate limiting (M1-19), idempotency (M1-20) and maintenance mode (M7-08): their stages exist here as empty slots.
- Transport protection, TLS limits and HSTS (M1-21).
- Real authentication (M1-33, M1-34, M1-44), `authorize` itself (M1-28), the closed catalogue and `unknown_action` (M1-25).
- Problem type documents (`getProblemType`, M1-46); this ticket fixes codes, statuses and titles only.
- Gateway: WebSocket origin check, gateway `error` frames, closing gateway client frame schemas under W7 (M1-64).
- Response validation in test builds (G8) and Schemathesis (G7): M1-71.
- File responses (SEC-HDR-4, M3) and console headers (SEC-HDR-3, M6).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-18-http-chain`.
- Conventional Commits with the component as scope (for example `feat(httpserver): ...`, `docs(api): ...`) and DCO sign-off (`git commit -s`). Squash into meaningful commits before the merge.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside Scope, except the docs and changelog entries named here.
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp`; remove them when done.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Tests bind only `127.0.0.1` and port 0 (P-10).
- Do not lower a threshold, disable a gate or delete a test.
- A gap in the core contract (chapters 00, 03, 04, 05) that this ticket needs ends the ticket with `Verdict: BLOCKED` (P-11).

## Design notes

### Chain order

Outer part, for every request on the public listener, before routing:

| # | Stage | Does |
|---|---|---|
| 1 | recover | catches panics: `500 internal_error`, stack in the log with the correlation ID; re-panics `http.ErrAbortHandler` (net/http convention) |
| 2 | trace and correlate | server span through M1-10 (`otelhttp`); correlation ID from the span; access log through the M1-10 logger (route pattern, status, duration, correlation ID; never query, raw path, client IP or credentials) |
| 3 | base headers and client IP | SEC-HDR-1 without HSTS, `Correlation-Id`; client IP into `reqmeta` |
| 4 | mux | routing; unmatched requests go to the unmatched handler |

Per route, wrapping each mounted `routes.Route.Handler`, so `r.Pattern` and the `routes.Operation` are known:

| # | Stage constant | Does | Owner |
|---|---|---|---|
| 5 | `StageRoute` | operation into the context, body limit, deadline, response writer | this ticket |
| 6 | `StageCORS` | SEC-CORS-1 and SEC-CORS-2, `api.cors_allowed_origins`, preflight | this ticket |
| 7 | `StageAuthenticate` | `actor.Authenticator`; anonymous without credentials | this ticket |
| 8 | `StageRateLimit` | slot | M1-19 |
| 9 | `StageMaintenance` | slot (05 rule 9) | M7-08 |
| 10 | `StageIdempotency` | slot | M1-20 |
| 11 | `StageBody` | media type, JSON guard | this ticket |
| 12 | `StagePreconditions` | required `If-Match` | this ticket |
| 13 | `StageValidate` | schema validation (SEC-IN-1, W7) | this ticket |
| 14 | `StageProof` | installs the check log | this ticket |
| 15 | generated wrapper, strict handler, module adapter, use case with `authorize` | | handler tickets |

API: `type Stage int` with the constants above; `type Chain`; `(*Chain).Use(stage Stage, mw func(http.Handler) http.Handler)` panics when a stage is set twice; empty stages pass through. A golden test holds the order, which is the order of 02 "Kette im api-Knoten". `StageAuthenticate` exposes a hook for failed credentials that M1-19 fills (02: failed credentials count against `rl.public_auth`).

### Errors

`kernel/errs` (M1-05) holds exactly the closed code list of 06 "Fehler"; internal causes such as `authz_not_checked` are `Cause` labels for log and metric, never a public code (02 "Fehlerarten"). Each code has a status and an English title. Title rule: the code with underscores replaced by spaces and the first letter upper case (`validation_failed` gives `Validation failed`, `i1_violation` gives `I1 violation`), which matches the examples in `openapi.yaml`. A test parses the generated problem block of 06 (`BEGIN GENERATED: problems`) and compares codes and statuses with `kernel/errs`.

Errors carry response headers where the contract needs them (`Retry-After`, `WWW-Authenticate`, `DPoP-Nonce`, `Allow`): add `errs.WithHeader(err, name, value)` or an equivalent; the problem writer applies them. `503 unavailable` always carries `Retry-After` with `http.unavailable_retry_after` in whole seconds. Add `errs.VersionConflict(current any)` and `errs.PreconditionRequired()`.

Problem writer `httpserver.WriteProblem(w http.ResponseWriter, r *http.Request, err error)`: body from the code; `correlation_id` from `reqmeta`; `instance` is the request path without query, except that every path parameter carrying a secret (today only `{token}` of `postIncomingWebhook`) appears as its template, e.g. the literal `{token}` (06 "Fehler" point 1, SEC-LOG-3); `detail` only where 06 asks for it (`payload_too_large` names the config key, for example `Body exceeds http.max_json_body_bytes.`); `errors[]` for `validation_failed`; `current` for `version_conflict`; `reason` and `hint` when the error carries them. Errors without a known kind become `500 internal_error`; their text goes only to the log. The same writer is the std-http `ErrorHandlerFunc` (parameter binding: `400 validation_failed`) and the strict `RequestErrorHandlerFunc` (`400 malformed_request`) and `ResponseErrorHandlerFunc` (mapping by kind) in `routes.Options`.

### Correlation ID

The correlation ID is the trace ID of the server span (16 bytes) written as a lowercase UUID string (8-4-4-4-12 hex). M1-10 guarantees a valid trace ID for every request, also with `telemetry.enabled = false` (17 "Signale"); if a unit test runs without that provider, generate 16 bytes from `crypto/rand` as a fallback. Store it with `reqmeta.WithCorrelationID`; every response carries `Correlation-Id`; a client-sent `Correlation-Id` header is ignored. An incoming `traceparent` is a span link, not the parent, unless `telemetry.traces.trust_remote_parent` (17, R-OB-05), so by default a client cannot choose the correlation ID. There is no separate request ID.

### Client IP

Input `server.trusted_proxies` (CIDR list). Start from the peer address of the connection. If the peer is not trusted, it is the client IP and forwarding headers are ignored. If it is trusted: use `Forwarded` (RFC 7239, `for=` values) when present, else `X-Forwarded-For`, never both; walk the address list from right to left and skip addresses inside `server.trusted_proxies`; the first address outside is the client IP; if all are trusted, the leftmost is the client. A malformed entry stops the walk, and the last trusted hop becomes the client IP (fail closed against spoofing). IPv4-mapped IPv6 addresses are unmapped. Store with `reqmeta.WithClientIP`. Logs never contain the client IP (SEC-LOG-5).

### Headers (SEC-T03)

Every response of the public listener: `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `Correlation-Id`, no `Server` header. API responses (JSON, problem, empty): `Cache-Control: no-store` (set before the handler; a file handler of M3 may override it), `Cross-Origin-Resource-Policy: same-origin`, and `Content-Type: application/json` or `application/problem+json` for bodies.

### CORS (SEC-T04, REST part)

No `Access-Control-*` header by default. Operations with action `instance.discovery.read` answer with `Access-Control-Allow-Origin: *` and never with `Access-Control-Allow-Credentials` (SEC-CORS-2). When `api.cors_allowed_origins` is not empty, a request whose `Origin` equals one entry exactly gets `Access-Control-Allow-Origin: <origin>`, `Vary: Origin` and `Access-Control-Expose-Headers: Correlation-Id, ETag, RateLimit, RateLimit-Policy, Retry-After, DPoP-Nonce, Idempotent-Replay`; `internal/wire` then mounts `OPTIONS <path>` for every enabled path, answering `204` with `Access-Control-Allow-Methods` (the enabled methods of that path), `Access-Control-Allow-Headers: Authorization, DPoP, Content-Type, Idempotency-Key, If-Match` and `Vary: Origin`. Other origins get no CORS header. Never credentials.

### Body, JSON guard, preconditions and validation

- Body limit in `StageRoute`: `http.MaxBytesReader` with `http.max_json_body_bytes` for `BodyJSON`, `file.max_upload_bytes` for `BodyMultipart` and `BodyOctetStream`. Exceeding gives `413 payload_too_large` with `detail` naming the key.
- Deadline in `StageRoute`: context deadline `http.request_timeout`; for multipart or octet-stream operations `http.transfer_timeout`, applied through `http.ResponseController` read and write deadlines (02 "Fristen"). An error that wraps `context.DeadlineExceeded` of the request context becomes `503 unavailable` with `Retry-After` (cause `deadline_exceeded` in the log).
- `StageBody`: a body whose `Content-Type` media type is not the operation's gives `415 unsupported_media_type`. For JSON: reject invalid UTF-8 (`utf8.Valid`), duplicate member names at any level, and nesting deeper than `http.max_json_depth`, each with `400 malformed_request`; then restore `r.Body` with the same bytes. Implement with `encoding/json` (`Decoder.Token`, `UseNumber`); no third-party JSON parser.
- `StagePreconditions`: an `IfMatchRequired` operation without `If-Match`, or with `*`, gives `428 precondition_required` (a wildcard would skip the version check the contract demands). Helpers: `FormatETag(version int64) string` returns the decimal in double quotes; `ParseIfMatch(h http.Header) IfMatch` returns `Present`, `Version` and `Valid`; a weak, list or non-decimal value is `Valid == false`, and the use case then answers `412 version_conflict` with `current` (strong comparison fails, RFC 9110).
- `StageValidate`: validate path, query and header parameters and the body against `api/rest/openapi.gen.yaml` before the generated wrapper runs. Library: `github.com/pb33f/libopenapi-validator` (MIT, OpenAPI 3.1, JSON Schema 2020-12). Write the probe tests `TestValidator_Probe_*` first; if it fails one, try `github.com/getkin/kin-openapi/openapi3filter`; if both fail, stop with `Verdict: BLOCKED` and the probe output. Record the choice in `docs/DECISIONS.md`. Errors become `400 validation_failed` with `errors[]`: `pointer` is a JSON Pointer into the body or `#path/<name>`, `#query/<name>`, `#header/<name>`; `code` is one of `required`, `unknown_member`, `invalid_type`, `invalid_format`, `invalid_enum`, `too_long`, `too_short`, `too_large`, `too_small`, `too_many_items`, `too_few_items`, `pattern_mismatch`, `invalid`; `detail` is a short English text without the submitted value.
- W7 spec edit and check: add `additionalProperties: false` to every object schema in the closure of request bodies except `JsonObject`. Extend the M1-17 generator with the check "request schemas closed" (fails naming the schema) and a negative fixture, so a new open request schema fails `make codegen-check`.

### No null and 2^53 (ADR 0053)

The response writer of `StageRoute` buffers bodies whose `Content-Type` is `application/json` or `application/problem+json` and streams other bodies unbuffered. Before sending, it scans the buffered JSON (`Decoder.Token`, `UseNumber`). An object member whose value is `null` is removed, and an error is logged with the route pattern and the JSON Pointer (never the value). A `null` array element, a top-level `null`, or an integer whose absolute value exceeds 2^53 - 1 replaces the response with `500 internal_error` and an error log. Clean bodies are sent byte for byte. The repair is a safety net only: the property test and the harness fail any handler test that produces the error log.

ADR 0053, if missing: context 06 R-API-03, 04 "Umschlag (Envelope)", 03 Grundsatz 13; options (a) an absent member means no value and `null` is never sent, (b) nullable members through OpenAPI 3.1 type arrays, (c) `null` only in PATCH bodies with merge-patch meaning; decision (a), because dart-dio has no null type and R-API-05 forbids type arrays; confirmation: Redocly rule `rule/no-type-arrays-or-null`, the null tests of this ticket, the envelope test of M1-16 and Schemathesis (G7).

### Proof of the authorize check

`kernel/authorize` gets a concurrency-safe check log in the request context: `WithCheckLog(ctx) context.Context`, `RecordCheck(ctx context.Context, c Check)` with `Check{Action string; Kind CheckKind; Allowed bool}` (`CheckKind` is `CheckAuthorize` or `CheckScope`), and `Checks(ctx) []Check`. Every `Authorizer` implementation (M1-28) must call `RecordCheck` once per `Authorize` and `Scope` call; say so in the doc comment of the interface. `StageProof` installs the log. The response writer, at the first `WriteHeader` or `Write`, checks: status below 400 and no check whose `Action` equals the route's action means the handler's headers and body are discarded, `500 internal_error` is sent (cause `authz_not_checked`), `bothy.authz.errors` with `bothy.reason = authz_not_checked` is incremented and an error is logged with the route pattern and the action. Responses with status 400 or higher pass unchanged (02 "Nachweis der Prüfung").

### Unmatched requests

The outer handler asks the mux for the handler (`ServeMux.Handler(r)`); if no pattern matched, it runs the mux's own handler against a probe writer to learn `404` or `405` and `Allow`, then writes `404 not_found` or `405 method_not_allowed` (with `Allow`) as a problem. Telemetry uses `http.route = unmatched`. Operations under `later` are therefore plain `404`, like unknown paths (P-04).

### Authentication

`StageAuthenticate` calls `actor.Authenticator`. It passes the method, the absolute request URI for the DPoP `htu` built from `server.public_url` plus the escaped path without query (never from the `Host` header), the values of `Authorization` and `DPoP`, and the client IP. Extend the M1-05 port with a `Credentials` struct if it lacks these, with doc comments, because identity (M1-34, M1-44) implements it later. A request without `Authorization` gets the anonymous actor; an error from the authenticator becomes its problem (the `401` codes of 06) with its headers. Until identity is wired, `internal/wire` uses `actor.AnonymousOnly`: anonymous without credentials, `401 unauthenticated` for any `Authorization` header (fail closed). The actor goes into the context through `kernel/actor`.

### Configuration

Use the names of `docs/operations/configuration.md` and add to the M1-07 schema what is missing: `http.max_json_body_bytes`, `http.max_json_depth` (a test proves every request example in `openapi.yaml` stays below it), `api.cors_allowed_origins`, `http.request_timeout`, `http.transfer_timeout`, `http.unavailable_retry_after`, `file.max_upload_bytes`, `server.trusted_proxies`, `server.public_url`. Defaults come from the chapter tables. Regenerate the reference (`make config-reference` or the M1-08 tool).

### Decisions to record in `docs/DECISIONS.md`

Validator library and probe result; FieldError code vocabulary; `If-Match: *` gives 428; null repair policy; title rule. (W7, the correlation ID, the chain order, the proof, `instance` with `{token}` and the new configuration names are already in 02, 06 and 17.)

## Tests (write them first)

Package `internal/platform/httpserver` (through `httpservertest`):

- `TestChainOrder_Golden`.
- `TestRecover_PanicGives500Problem`, `TestRecover_ErrAbortHandlerRepanics`.
- `TestCorrelationID_OnEveryResponseKind` (200, 204, 400, 404 unmatched, 405, 413, 500 from a panic): the header equals `correlation_id` in the body and the span trace ID; `TestCorrelationID_GeneratedWithoutTracer`; `TestCorrelationID_ClientHeaderIgnored`.
- `TestErrs_MatchesProblemTable` (06 block against `kernel/errs`), `TestProblem_ShapePerCode`, `TestProblem_NoInternalDetail` (a wrapped database error text never reaches the body), `TestProblem_InstanceRedactsToken`, `TestProblem_UnavailableHasRetryAfter`.
- `TestUnmatched_404Problem`, `TestUnmatched_405ProblemWithAllow`.
- `TestSECT03_HeadersPerResponseKind`.
- `TestSECT04_NoCORSHeadersByDefault`, `TestSECT04_DiscoveryWildcardWithoutCredentials`, `TestCORS_AllowedOriginEchoAndPreflight`, `TestCORS_OtherOriginNoHeaders`.
- `TestClientIP_UntrustedPeerIgnoresForwarding` (ID-T25, header part), `TestClientIP_TrustedChainRightToLeft`, `TestClientIP_ForwardedPreferredOverXFF`, `TestClientIP_MalformedStopsAtLastTrusted`, `TestClientIP_IPv6AndMapped`.
- `TestSECIN2_DuplicateKeysRejected` (top level and nested), `TestSECIN2_InvalidUTF8Rejected`, `TestSECIN2_DepthLimit`, `TestSECIN3_BodyLimit413NamesKey`, `TestBody_WrongMediaType415`, `FuzzJSONGuard` (seed corpus runs in `go test`).
- `TestW7_UnknownMemberRejected` (pointer to the member, code `unknown_member`), `TestValidate_ErrorShape` (one case per code family and per pointer form), `TestValidator_Probe_*` (features the spec uses: `additionalProperties: false`, `format: uuid`, enums, `minLength`, `maxItems`, `oneOf` with discriminator, header and path parameters).
- `TestPreconditions_MissingIfMatch428`, `TestPreconditions_Wildcard428`, `TestIfMatch_ParseAndFormat`, `TestVersionConflict_412WithCurrent`.
- `TestNoNull_PropertyRandomValues` (`testing/quick` over recursive maps, slices, pointers and nil slices: the output has no `null` token and equals the input without its null members), `FuzzSanitizeJSON`, `TestNoNull_NullInArrayGives500`, `TestInt53_AboveLimitGives500`, `TestNoNull_CleanBodyByteIdentical`.
- `TestProof_MissingCheckGives500AuthzNotChecked` (counter plus one through the manual metric reader), `TestProof_OtherActionGives500`, `TestProof_ScopeCounts`, `TestProof_ErrorsPassThrough`, `TestProof_ConcurrentRecords`.
- `TestDeadline_ExceededGives503`, `TestTransferDeadline_UsesTransferTimeout`.
- `TestAuth_AnonymousWithoutHeader`, `TestAuth_AnonymousOnlyRejectsAuthorization401`, `TestAuth_HtuFromPublicURL`.

Generator and config:

- `TestGenerate_RejectsOpenRequestSchema` (negative fixture), `TestRealSpec_RequestSchemasClosed`.
- `TestMaxJSONDepth_CoversAllRequestExamples`.

Commands that must pass: `cd services/server && go test -race ./internal/platform/httpserver/... ./internal/kernel/... ./api/... ./internal/wire/...`, `make api-lint`, `make codegen-check`, `make docs`, `make arch`, `make check`.

## Acceptance criteria

- [ ] A test route without `authorize` returns `500 internal_error` (log cause `authz_not_checked`) and increments `bothy.authz.errors` with `bothy.reason = authz_not_checked`.
- [ ] Every error kind maps to status and problem body per 06 "Fehler"; `kernel/errs` equals the 06 table; internal causes never appear in a response body.
- [ ] The property test finds no JSON `null` in responses; integers beyond 2^53 - 1 never leave the server.
- [ ] A missing `If-Match` on a required operation gives `428`, a mismatch `412` with `current`.
- [ ] Forwarding headers count only from trusted peers (ID-T25, header part).
- [ ] SEC-T03 holds for API and error responses; SEC-T04 holds for REST.
- [ ] Duplicate keys, invalid UTF-8 and too deep nesting give `400 malformed_request` (SEC-IN-2); unknown request members give `400 validation_failed` (W7).
- [ ] Request schemas in `openapi.yaml` are closed, the generator check holds it, `make api-lint` is green.
- [ ] ADR 0053 names this ticket's property test; `openapi.yaml` request schemas are closed (W7) and the generator check holds them; the configuration reference is regenerated.
- [ ] The cursor codec rejects a cursor of another list, binding or principal and any tampered byte with `400 cursor_invalid` (unit tests), and round-trips keyset values.
- [ ] `make check` is green.

## Definition of done

- Code and tests as above; `make check` green.
- Documentation: `internal/platform/httpserver/README.md` (stages, how a ticket adds a stage, error contract, harness usage); doc comment on every exported symbol (purpose, parameters, return value, errors, example where useful), comments explain why; ADR 0053 if it was missing (MADR 4, `make adr-index`); the spec edits listed in Scope; a `services/server/CHANGELOG.md` entry under Unreleased; regenerated configuration reference and API tables committed.
- Independent review over the diff (`make review` plus a code review) and the red team gate (`docs/quality/red-team-gate.md`) over the diff, with the verdict `JA` or `NEIN` recorded in the report. On `NEIN`: fix, one test per finding, recheck the delta.
- Merge locally into the base branch only if review and red team are green and `make check` passes; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then commits, files with line counts, test results with counts (including the red run of each new test), what is not proven, and follow-ups that could not be done in scope with the reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
