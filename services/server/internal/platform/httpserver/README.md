# Package `platform/httpserver`

Status: built (listeners M1-11, chain M1-18); M1-21 adds the plaintext peer check in `Listen` and
HSTS. Normative are [02-architektur.md](../../../../../docs/spec/02-architektur.md) "Request-Pfad",
"Fehlerarten" and "Fristen", [06-api.md](../../../../../docs/spec/06-api.md) "Grundregeln",
"Fehler", "Nebenläufigkeit" and "Paginierung",
[19-sicherheit.md](../../../../../docs/spec/19-sicherheit.md) SEC-HDR, SEC-CORS, SEC-IN and SEC-LOG,
[16-betrieb.md](../../../../../docs/spec/16-betrieb.md) "Zwei Listener" and ADRs 0015 and 0053; the
chapters win on any difference. Rulings: [DECISIONS.md](../../../../../docs/DECISIONS.md) M1-18.

## Interface

- `Listen(ctx, addr)`, `Config`, `NewServer(cfg, handler, log)`, `Internal(registry)`: the
  listeners of M1-11. `NewServer` has no `ReadTimeout` or `WriteTimeout`; the per-route deadlines
  of `StageRoute` apply instead.
- `NewChain(ChainConfig, Deps)`: the chain with every stage of this ticket. `ChainConfig` binds
  `http.max_json_body_bytes`, `http.max_json_depth`, `http.request_timeout`,
  `http.transfer_timeout`, `http.unavailable_retry_after`, `file.max_upload_bytes`,
  `api.cors_allowed_origins`, `server.trusted_proxies` and `server.public_url`; `Deps` holds the
  logger, the meter provider, the `actor.Authenticator` and the `Validator`.
- `Chain.Wrap(op, handler)` per route, `Chain.Public(mux)` for the listener, `Chain.Preflight` for
  `OPTIONS <path>` when CORS origins are configured, `Chain.Use(stage, mw)` for the slots,
  `Chain.SetFailedCredentialsHook` for M1-19, `Chain.Order()` for the documented order.
- `WriteProblem(w, r, err)`, `ValidationErrorHandler`, `MalformedRequestHandler`: the error
  writers; `internal/wire` passes them to `routes.Build`.
- `NewValidator(spec, operationIDs)`, `CheckJSON`, `SanitizeJSON`, `ClientIP`, `CheckOrigin`,
  `FormatETag`, `ParseIfMatch`, `EncodeCursor`, `DecodeCursor`, `OperationFrom`.

## Stages

| # | Stage | Does |
|---|---|---|
| 1 | recover | panic: `500 internal_error`, `panic recovered` in the log with the function; `http.ErrAbortHandler` re-raised |
| 2 | trace | `otel.Middleware` (span, access log, duration metric); correlation ID from the trace ID into `kernel/reqmeta` |
| 3 | headers | `X-Content-Type-Options`, `Referrer-Policy`, `Correlation-Id`; client IP into `kernel/reqmeta` |
| 4 | mux | routing; no pattern: `404 not_found`, wrong method: `405 method_not_allowed` with `Allow` |
| 5 | `StageRoute` | operation into the context, `Cache-Control: no-store`, CORP, body limit, deadline, response writer |
| 6 | `StageCORS` | none by default; `*` for `instance.discovery.read`; exact origins of `api.cors_allowed_origins` |
| 7 | `StageAuthenticate` | `actor.Authenticator` with method, htu from `server.public_url`, `Authorization`, `DPoP`, client IP |
| 8 | `StageRateLimit` | slot (M1-19) |
| 9 | `StageMaintenance` | slot (M7-08) |
| 10 | `StageIdempotency` | slot (M1-20) |
| 11 | `StageBody` | media type (`415`), JSON guard (`400 malformed_request`), required body |
| 12 | `StagePreconditions` | `IfMatchRequired` without `If-Match` or with `*`: `428` |
| 13 | `StageValidate` | parameters and JSON body against the contract (`400 validation_failed`) |
| 14 | `StageProof` | check log of `kernel/authorize` |
| 15 | handler | generated wrapper, strict handler, module adapter, use case with `authorize` |

`TestChainOrder_Golden` holds this order (`testdata/chain_order.golden`).

## How a ticket adds a stage

Build the middleware as `func(http.Handler) http.Handler`, read the operation with
`OperationFrom(r.Context())`, answer errors with `WriteProblem`, and install it once, before any
route is mounted, with `chain.Use(httpserver.StageRateLimit, mw)` in `internal/wire`. `Use`
panics when the stage is taken, is not a slot, or routes are already mounted. A stage that answers
a success itself (an idempotent replay) must record the check of the route's action first:
the proof applies to every success of a route.

## Error contract

- Status and code come from `kernel/errs`; titles from `x-problem-codes` of `openapi.yaml`.
- `*http.MaxBytesError` is `413 payload_too_large` with `detail` naming the key; an error wrapping
  `context.DeadlineExceeded` of the request is `503 unavailable` (`bothy.reason =
  deadline_exceeded` in the log); anything else is `500 internal_error` with only its Go type in
  the log.
- `503 unavailable` and `maintenance` always carry `Retry-After` (`http.unavailable_retry_after`).
- Error headers of `errs.Error.WithHeader` (`WWW-Authenticate`, `DPoP-Nonce`, `Allow`) are applied.
- `instance` is the escaped path with `{token}` segments kept as `{token}`; unmatched requests have
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
| `chain.go` | `Stage`, `Chain`, `Deps`, outer steps, unmatched handler |
| `route.go` | `StageRoute`, response writer, proof |
| `stages.go` | CORS, authentication, body, preconditions, validation stages |
| `problem.go` | `WriteProblem` and the error mapping |
| `validate.go` | `Validator` (libopenapi-validator) and the FieldError mapping |
| `jsonguard.go`, `sanitize.go` | JSON guard (SEC-IN-2), null and integer scan (ADR 0053) |
| `clientip.go`, `etag.go`, `cursor.go`, `config.go`, `operation.go` | helpers and types |
| `httpservertest/` | the test harness and its contract |

## Dependencies

`platform/health`, `platform/otel`, `kernel/actor`, `kernel/authorize`, kernel core; vendor groups
`otel` and `libopenapi` (module register).
