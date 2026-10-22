# Package `platform/httpserver`

Status: built (listeners M1-11, chain M1-18, transport protection and HSTS M1-21, the webhook token slot and the per-operation body limit M2-27). Normative are
[02-architecture.md](../../../../../docs/spec/02-architecture.md) "Request path",
"Error kinds" and "Deadlines", [06-api.md](../../../../../docs/spec/06-api.md) "Basic rules",
"Errors", "Concurrency" and "Pagination",
[19-security.md](../../../../../docs/spec/19-security.md) SEC-TLS, SEC-HDR, SEC-CORS, SEC-IN and SEC-LOG,
[16-operations.md](../../../../../docs/spec/16-operations.md) "Two listeners" and ADRs 0015 and 0053; the
chapters win on any difference. Rulings: [DECISIONS.md](../../../../../docs/DECISIONS.md) M1-18 and M1-21.

## Interface

- `Listen(ctx, addr)`, `Config`, `NewServer(cfg, handler, log)`, `Internal(registry)`: the
  listeners of M1-11. `NewServer` has no `ReadTimeout` or `WriteTimeout`; the per-route deadlines
  of `StageRoute` apply instead. `InternalHandler(mux, sec)` serves the internal mux with
  SEC-HDR-1, the API headers and problem bodies for unknown paths and methods.
- `ListenPublic(ctx, TransportConfig, SecretGetter, log)`, `PeerFilter`, `CheckTransport`,
  `TLSEnabled`: the transport protection of the public listener (M1-21, see below).
- `SecurityHeadersFor(publicURL, hstsMaxAge)` and `SecurityHeaders.Apply`: SEC-HDR-1 with HSTS.
- `NewChain(ChainConfig, Deps)`: the chain with every stage of this ticket. `ChainConfig` binds
  `http.max_json_body_bytes`, `http.max_json_depth`, `http.request_timeout`,
  `http.transfer_timeout`, `http.unavailable_retry_after`, `file.max_upload_bytes`,
  `api.cors_allowed_origins`, `server.trusted_proxies`, `server.public_url` and
  `http.hsts_max_age`; `Deps` holds the
  logger, the meter provider, the `actor.Authenticator` and the `Validator`.
- `Chain.Wrap(op, handler)` per route, `Chain.Public(mux)` for the listener, `Chain.Preflight` for
  `OPTIONS <path>` when CORS origins are configured, `Chain.Use(stage, mw)` for the slots,
  `Chain.SetCredentialsPrecheck` for M1-19 (installed by `ratelimit.Install`; its settle function
  gets the result of the `Authenticator`, and `401 unauthenticated` for an actor with
  `CredentialInactive`, M1-44), `Chain.SetFailedCredentialsHook`, `Chain.Order()` for the documented order. `Operation.RateLimitClass` is
  the class `StageRateLimit` counts. The route class of transfers (`http.transfer_timeout` for the
  context and the write deadline) holds for a multipart or octet-stream request body and, since
  M1-50, for `Operation.Download`, a success that streams a body that is not JSON (the export of
  the audit log).
- `WriteProblem(w, r, err)`, `ValidationErrorHandler`, `MalformedRequestHandler`: the error
  writers; `internal/wire` passes them to `routes.Build`.
- `NewSchemaValidator(spec, names)` and `SchemaValidator.Validate(name, body)`: a JSON value
  against one named schema of the contract with the field codes of the request validator; the
  gateway checks the body of every client frame with it (M1-64).
- Console cookie (M6-01): `StageAuthenticate` reads `__Host-console_session` into
  `actor.Credentials.ConsoleCookie`; a request the cookie authenticated then passes ConsoleGuard
  before every later stage: methods other than `GET`, `HEAD` and `OPTIONS` need
  `X-Console-Request: 1` (`HeaderConsoleRequest`) and an `Origin` equal to the origin of
  `server.public_url` (`SameOrigin`), else `403 forbidden` with reason `csrf_check_failed`
  (`ReasonCSRFCheckFailed`), logged with the reason only. Requests with an `Authorization` header
  are not checked. `httpservertest.Actors.SetCookie` and `LogCapture.Contains` serve the tests.
  The action `console.csp_report` (`ActionCSPReport`) is not checked: a browser sends a violation
  report with the cookie but never with the CSRF header, and the report changes nothing (M6-03).
- Console CSP (M6-03, `csp.go`): `ContentSecurityPolicy(nonce)` builds the policy of
  `docs/spec/19-security.md` 5 with `TrustedTypesPolicies` (the names SEC-T05 observes), `NewNonce`
  returns 128 random bits in base64, and `ConsoleAssets` sets the policy with a fresh nonce, COOP,
  COEP, CORP, `X-Frame-Options`, `Permissions-Policy` and `Reporting-Endpoints` (`CSPReportPath`) on
  every response of the console mount. `OpenConsoleAssets` reads `index.html` once and refuses one
  without `NoncePlaceholder`; each answer of index.html carries the nonce of its CSP, no ETag.
- Body kind `BodyReport` (M6-03): `application/reports+json` or `application/csp-report`, read
  whole under `http.max_json_body_bytes` and the JSON guard, available as `RawJSONBody`; the
  request validator checks no schema for it.
- `Operation.AuthorizedInBand`: the one route that authenticates and authorizes on the connection
  it opens (`GET /api/v1/gateway`, M1-64); `Chain.Wrap` runs no per-route stage for it, only the
  outer steps 1 to 4, and puts the operation into the context.
- `Operation.BodyLimit` and `Operation.BodyLimitKey` (M2-27): a `BodyLimit` above zero replaces
  `http.max_json_body_bytes` (and `file.max_upload_bytes`) as the body limit of the operation, and
  the `413 payload_too_large` names `BodyLimitKey` in its `detail` (the two post operations of the
  incoming webhooks: `webhook_in.max_body_bytes`, set by `wire.WebhookBodyLimit`). For such an
  operation a declared `Content-Length` above the limit is refused at the start of `StageBody`
  instead of `StageRoute`, so the stages before it (rate limit, webhook token) answer first (11
  8.4: token before size); the body is still never read beyond the limit.
- `Operation.AnonymousCredentials` (M2-27): the credential of the operation is not an identity
  credential (the token of an incoming webhook). `StageAuthenticate` runs neither the credentials
  precheck nor the `Authenticator` nor ConsoleGuard for it and puts the anonymous actor into the
  context, whatever `Authorization` holds; the stage at `StageWebhookToken` checks the credential
  and counts its failures itself (`ratelimit.Limiter.FailedCredentialGuard`).
- `NewValidator(spec, operationIDs)`, `CheckJSON`, `SanitizeJSON`, `ClientIP`, `CheckOrigin`,
  `FormatETag`, `ParseIfMatch`, `EncodeCursor`, `DecodeCursor`, `OperationFrom`.
- `NewIdempotencyStage(cfg, store, uow, clock)`, `IdempotencyConfig`, `Fingerprint`,
  `ReplayETag`: the Idempotency-Key stage; `internal/wire` installs it (`InstallIdempotency`).

## Stages

| # | Stage | Does |
|---|---|---|
| 1 | recover | SEC-HDR-1 with HSTS, applied again before the header block goes out; panic: `500 internal_error`, `panic recovered` in the log with the function; `http.ErrAbortHandler` re-raised |
| 2 | trace | `otel.Middleware` (span, access log, duration metric); correlation ID from the trace ID into `kernel/reqmeta` |
| 3 | headers | `Correlation-Id`; client IP into `kernel/reqmeta` |
| 4 | mux | routing; no pattern: `404 not_found`, wrong method: `405 method_not_allowed` with `Allow` |
| 5 | `StageRoute` | operation into the context, `Cache-Control: no-store`, CORP, body limit (a declared `Content-Length` above it: `413`, except for an operation with `BodyLimit`, see step 12), deadline, response writer |
| 6 | `StageCORS` | none by default; `*` for `instance.discovery.read`; exact origins of `api.cors_allowed_origins` |
| 7 | `StageAuthenticate` | precheck for requests with credentials (M1-19: `429` while `rl.public_auth` of the client IP is empty), `actor.Authenticator` with method, htu from `server.public_url`, `Authorization`, `DPoP`, client IP; `reqmeta.Transport` (TCP peer from `RemoteAddr`, header, action) into the context for the trusted header driver (M1-43); failed-credentials hook; an operation with `AnonymousCredentials` skips precheck and `Authenticator` and stays anonymous (M2-27) |
| 8 | `StageRateLimit` | token bucket of the operation's class (`platform/ratelimit`, M1-19): `RateLimit-Policy`, `RateLimit`; exhausted: `429 rate_limited` with `Retry-After` |
| 9 | `StageWebhookToken` | slot (M2-27): the token check of the incoming webhooks (`apps/adapters/httpapi.TokenStage`, installed by `wire.InstallWebhookToken`), after `rl.webhook_in` and before any body stage |
| 10 | `StageMaintenance` | slot (M7-08) |
| 11 | `StageIdempotency` | `Idempotency-Key` in the unit of work (M1-20, below) |
| 12 | `StageBody` | declared `Content-Length` above `Operation.BodyLimit` (`413`, M2-27), media type (`415`), JSON guard (`400 malformed_request`), required body |
| 13 | `StagePreconditions` | `IfMatchRequired` without `If-Match` or with `*`: `428` |
| 14 | `StageValidate` | parameters and JSON body against the contract (`400 validation_failed`) |
| 15 | `StageProof` | check log of `kernel/authorize` |
| 16 | handler | generated wrapper, strict handler, module adapter, use case with `authorize` |

`TestChainOrder_Golden` holds this order (`testdata/chain_order.golden`).

## Idempotency-Key (M1-20)

`NewIdempotencyStage` builds `StageIdempotency` over `kernel/idempotency.Store` (implemented by
the sync module) and `kernel/txn.UnitOfWork` (docs/spec/02-architecture.md "Idempotency in the Unit of
Work", 06 "Idempotency"). It acts only when the operation declares the parameter
(`Operation.IdempotencyKey`), the request carries the header and the actor is not anonymous
and not `CredentialInactive` (an API token the authenticator found revoked or expired, M1-44,
or a person's access token whose session, device or key is revoked, ended or inactive, M1-45:
such a request goes on to `authorize`, which refuses it, and never gets a stored answer);
otherwise the header has no effect. A key that is not one UUIDv7 is `400 validation_failed`
(`#header/Idempotency-Key`, `invalid_format`).

1. The body is read under the limit of `StageRoute` (over it: `413`, nothing stored) and the
   read deadline is lifted. `Fingerprint` is SHA-256 over the upper-case method, `\n`, the
   escaped path, `\n` and the RFC 8785 form of the body (`platform/jsoncanon`), the raw bytes
   when the body is not JSON or repeats a member, empty without a body.
2. The rest of the chain runs inside one unit of work, opened with `txn.AllowLateRightsChange()`
   so a keyed use case that changes rights declares its change in its nested unit (M1-31,
   resolves docs/DECISIONS.md M1-20/4); the use case joins it. `Store.Claim`
   claims the key; a committed row answers from the store without the handler, the validator or
   the proof: the stored status and body with `Idempotent-Replay: true`, `ETag` from `version`
   for the statuses in `Operation.ETagStatuses` (`ReplayETag`), `422 idempotency_key_reuse` for
   another fingerprint, `409 conflict` with `reason: one_time_secret` for a response whose handler
   called `idempotency.MarkOneTimeSecret`. A wait for an uncommitted first request that ends with
   `database.lock_timeout` is `409 request_in_progress`.
3. The handler writes into a buffer. `2xx`: `Store.Complete` and commit with the effect (a
   failed commit is the error of the unit of work, nothing stored). `400`, `403`, `404`, `409`
   except `request_in_progress`, `410`, `412`, `413`, `415`, `422` with a JSON body: rollback,
   then `Store.SaveFinal` in its own transaction. Every other status: rollback, nothing stored.
4. A deadlock or lock timeout that reaches `WriteProblem` inside an attempt ends the attempt with
   that error, so the unit of work retries it (`database.tx_retry_max`); each attempt gets the
   body again and an empty buffer.

The tests are `idempotency_test.go` (stage over an in-memory store) and
`internal/sync/adapters/postgres/idempotency_http_test.go` (the full chain on PostgreSQL 18).

## Setup gate (M1-37)

`SetupGate(state SetupState, routes RouteLookup)` keeps a fresh instance closed until the first
owner exists (08 "Initial installation"): only operations whose action is `instance.discovery.read`
pass; every other request, unknown paths and the gateway path included, gets `503
setup_required` without `Retry-After`. A failing `SetupState` is `503 unavailable`; the first
`true` is cached for the process. `Chain.UseGate` installs it in step 4 in front of the mux, so it
answers before authentication and rate limiting; `MuxLookup` resolves the action from the mounted
patterns with `(*http.ServeMux).Handler`, and `SetupStateFunc` adapts a function.
`internal/wire` connects it to the instance row (`InstallSetupGate`). Tests: `setupgate_test.go`
and `TestIDT06_RequestsBeforeSetupGet503` in `internal/wire`.

## Transport protection (M1-21)

| Mode | When | Public listener |
|---|---|---|
| plaintext | `server.tls_cert_ref` and `server.tls_key_ref` empty | `PeerFilter`: only peers on loopback (`127.0.0.0/8`, `::1`, also IPv4-mapped) or in `server.trusted_proxies` are served; every other connection is closed at accept time without a byte read or written, and accepting goes on (SEC-TLS-1) |
| TLS | both references set, `server.public_url` with `https` | TLS 1.2 as the floor, the cipher suites and curves of Go (a nil `CipherSuites` takes "a safe default list", [crypto/tls Config](https://pkg.go.dev/crypto/tls#Config), retrieved 2026-10-13), no client certificates, no peer filter, no plaintext redirect listener (SEC-TLS-2, R-SE-08) |

- One list: `TransportConfig.TrustedProxies` is the slice `ChainConfig.TrustedProxies` that the
  client IP uses, so who may send plaintext and whose forwarding headers count never differ.
- The first refusal of a listener logs at Warn with the hint to add the proxy to
  `server.trusted_proxies`, later ones at Debug; no line names the peer (SEC-LOG-5).
- Certificate material: the certificate reference holds the PEM chain with the leaf first, the key
  reference a PEM private key. `CheckTransport` (start step 2 in `internal/wire`) and
  `ListenPublic` refuse a missing or unreadable reference, a pair that does not match, a leaf that
  is not yet valid or expired and a leaf that does not cover the host of `server.public_url`; the
  errors name keys, never content. Every handshake asks the secret provider for both references
  (cached for `secrets.cache_ttl`); a new pair of versions is validated and used, a broken one
  keeps the previous certificate, logs `TLS certificate rotation failed, keeping the previous
  certificate` once and is tried again at most once per `rotationRetry` (a leaf not yet valid heals
  without a restart).
- The internal listener has no peer filter and stays plaintext (R-SE-09); it is never published.
- Not supported on purpose: PROXY protocol, h2c, Unix socket listeners for the public API.
- HSTS: `Strict-Transport-Security: max-age=<http.hsts_max_age in seconds>` on every response of
  both listeners while `server.public_url` uses `https`, without `includeSubDomains` and
  `preload`; absent with an `http` URL (SEC-TLS-3).

## How a ticket adds a stage

Build the middleware as `func(http.Handler) http.Handler`, read the operation with
`OperationFrom(r.Context())`, answer errors with `WriteProblem`, and install it once, before any
route is mounted, with `chain.Use(httpserver.StageRateLimit, mw)` in `internal/wire`. `Use`
panics when the stage is taken, is not a slot, or routes are already mounted. A stage that answers
a success itself (an idempotent replay) must record the check of the route's action first:
the proof applies to every success of a route.

## Error contract

- Status and code come from `kernel/errs`; titles from `x-problem-codes` of `openapi.yaml`.
- `*http.MaxBytesError` is `413 payload_too_large` with `detail` naming the key (`Operation.BodyLimitKey`
  when the operation sets `BodyLimit`, else `file.max_upload_bytes` or `http.max_json_body_bytes`); an error wrapping
  `context.DeadlineExceeded` of the request is `503 unavailable` (`bothy.reason =
  deadline_exceeded` in the log); anything else is `500 internal_error` with only its Go type in
  the log.
- `503 unavailable` and `maintenance` always carry `Retry-After` (`http.unavailable_retry_after`).
- Error headers of `errs.Error.WithHeader` (`WWW-Authenticate`, `DPoP-Nonce`, `Allow`) are applied.
- `instance` is the escaped path with `{token}` segments written as `%7Btoken%7D` (a URI reference has no braces); unmatched requests have
  none. Internal causes and wrapped texts never reach the body.
- Response writer: success without a check of the route's action is replaced by `500` and counts in
  `bothy.authz.errors`; JSON bodies lose null members (logged), null elements and integers beyond
  2^53 - 1 give `500`; clean bodies are byte for byte.

## Harness

```go
op := httpservertest.Op("getThing", http.MethodGet, "/api/v1/things/{thing_id}", "thing.read")
h := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{
	Op:      op,
	Handler: httpservertest.Authorized(op.Action, myHandler),
}}})
resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+id, ""))
_ = resp.Problem(t)                       // decoded problem body
_ = h.Counter("bothy.authz.errors")       // manual metric reader
_, _ = h.Logs.Find(httpserver.LogAuthzNotChecked)
_ = httpservertest.Spans().Ended()        // server spans
```

`Options.Contract` replaces the test contract (`httpservertest/contract.yaml`); every route's
operation ID must be in it. A handler that sends JSON `null` fails the test unless
`AllowNullRepair` is set.

## Code

| File | Content |
|---|---|
| `httpserver.go` | `Config`, `Listen`, `NewServer`, `Internal` |
| `transport.go` | `TransportConfig`, `ListenPublic`, `CheckTransport`, `PeerFilter` |
| `tlscert.go` | certificate source of the TLS mode: validation at start and rotation |
| `headers.go` | `SecurityHeaders` (SEC-HDR-1, HSTS), `InternalHandler`, unmatched handler |
| `chain.go` | `Stage`, `Chain`, `Deps`, outer steps |
| `route.go` | `StageRoute`, response writer, proof |
| `stages.go` | CORS, authentication, body, preconditions, validation stages |
| `problem.go` | `WriteProblem` and the error mapping |
| `idempotency.go` | `StageIdempotency`: fingerprint, unit of work, replay, buffered response |
| `setupgate.go` | `SetupGate`, `SetupState`, `SetupStateFunc`, `RouteLookup`, `MuxLookup` (M1-37) |
| `validate.go` | `Validator` (libopenapi-validator) and the FieldError mapping |
| `jsonguard.go`, `sanitize.go` | JSON guard (SEC-IN-2), null and integer scan (ADR 0053) |
| `clientip.go`, `etag.go`, `cursor.go`, `config.go`, `operation.go` | helpers and types |
| `httpservertest/` | the test harness and its contract |

## Dependencies

`platform/health`, `platform/otel`, `platform/secrets` (references and versions of the TLS
material), `platform/jsoncanon` (fingerprint), `kernel/actor`, `kernel/authorize`,
`kernel/idempotency`, `kernel/txn`, kernel core; vendor groups
`otel` and `libopenapi` (module register).
