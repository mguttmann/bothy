# Package `platform/otel`

Status: built (M1-10). Normative: [17-observability.md](../../../../../docs/spec/17-observability.md)
"Grundregeln", "Signale", "Datenschutz", [signals.md](../../../../../docs/observability/signals.md),
[privacy.md](../../../../../docs/observability/privacy.md),
[19-sicherheit.md](../../../../../docs/spec/19-sicherheit.md) SEC-LOG-1 to SEC-LOG-6 and
[ADR 0034](../../../../../docs/adr/0034-opentelemetry-ueber-collector-lokaler-standard-stack.md);
on a difference the documents win. Rulings of the build: [DECISIONS.md](../../../../../docs/DECISIONS.md)
section M1-10.

## Purpose

The one telemetry setup of the server. Every package logs, traces and measures through it, so
content, secrets, raw paths, query strings and headers outside an allowlist stay out of telemetry
by construction, telemetry never blocks or fails a request, and every item that never reaches the
collector is counted.

## Code

| File | Content |
|---|---|
| `config.go` | `Config`, bound from the configuration in `internal/wire` |
| `setup.go` | `Deps`, `Setup`, `Telemetry`, providers, sampler, OpenTelemetry error handler |
| `env.go` | the `OTEL_*` variables the package interprets itself |
| `resource.go` | resource of the process |
| `queue.go` | in-flight budget and front queue of one signal |
| `spans.go` | span processor, attribute filter and counting span exporter |
| `txnspan.go` | `SubscriberSpan`, the span `txn.subscribe <module>` of an in-transaction subscriber (kernel/txn tracer, M1-13) |
| `logexport.go` | log processor and counting log exporter |
| `metrics.go` | `bothy.telemetry.dropped`, the view of `http.server.request.duration`, counting metric exporter |
| `logs.go` | slog handler chain, `NewCommandLogger`, `ParseLevel`, `CorrelationID`, `ServerErrorLog` |
| `http.go` | `Middleware`, `RecordRoute`, access log |
| `pseudonym.go` | `Pseudonymizer`, `NewPseudonymizer` |
| `attr/` | attribute keys and `attr.Allowed` |
| [`genai/`](genai/README.md) | the `gen_ai.*` keys |

## Setup

```go
tel, err := otel.Setup(ctx, cfg, otel.Deps{
	Secrets:        provider,            // resolves telemetry.pseudonym_key_ref
	Clock:          clock.System(),
	Stdout:         os.Stdout,
	ServiceVersion: buildinfo.Version(),
	Env:            os.Environ(),        // read once in cmd/bothy
})
if err != nil { /* start error: exit 1 */ }
defer tel.Shutdown(flushCtx)             // context with shutdown.telemetry_flush_timeout
logger := tel.Logger()
handler := otel.Middleware(mux)          // after Setup
```

- `cfg` comes from `wire.TelemetryConfig(values)`: `telemetry.enabled`,
  `telemetry.traces.sampler_ratio`, `telemetry.traces.trust_remote_parent`,
  `telemetry.logs.exporter`, `telemetry.logs.level`, `telemetry.pseudonym_key_ref`,
  `privacy.telemetry_pseudonym_rotation`, `server.role`, `instance.environment`.
- `Setup` installs the global tracer, meter and logger providers, the W3C `TraceContext`
  propagator (no baggage) and an error handler. It fails on an unusable `Config` or `Deps`
  (`ErrConfig`), an `OTEL_*` variable it cannot use (`ErrInvalidEnv`), a missing pseudonym key or
  one under 32 bytes (`ErrKeyTooShort`) and errors of the exporter constructors.
- `Deps.Env` is an addition to the brief: no package outside `cmd/bothy` reads the environment
  (forbidigo), and `Setup` reads `OTEL_EXPORTER_OTLP_PROTOCOL`, the per-signal
  `OTEL_EXPORTER_OTLP_{TRACES,METRICS,LOGS}_PROTOCOL` (`grpc` default, `http/protobuf`),
  `OTEL_{TRACES,METRICS,LOGS}_EXPORTER` (`otlp` or `none`) and `OTEL_BSP_MAX_QUEUE_SIZE`,
  `OTEL_BLRP_MAX_QUEUE_SIZE` (default 2048, the SDK default) from it. Other values are a start
  error naming the variable. The OTLP exporters read endpoint, headers, timeouts and TLS, and
  package `resource` reads `OTEL_RESOURCE_ATTRIBUTES`, from the process environment themselves.
- `Shutdown` flushes traces and logs first, so the drops they count reach the last metric
  export, and never waits beyond the context deadline. Later calls do nothing.
- `NewCommandLogger(w, level)` is the logger of short-lived commands and of the time before the
  configuration exists: JSON lines, the same allowlist, never OTLP. `cmd/bothy` uses it at
  `telemetry.logs.level`, `info` before the configuration is loaded.

## Signals

- Traces: OTLP, sampled by the server (see "Sampling"); span names `<METHOD> <http.route>`.
- Metrics: OTLP with cumulative temporality (the collector converts per backend). Of the
  `otelhttp` instruments only `http.server.request.duration` stays, with exactly
  `http.request.method`, `http.route`, `http.response.status_code`, `error.type` and the bucket
  boundaries of signals.md 4.7; the body size histograms are dropped and exemplars are off.
- Logs: the handler chain is level filter, allowlist filter, then the stdout JSON handler and/or
  the `otelslog` bridge per `telemetry.logs.exporter`. The stdout lines carry `trace_id`,
  `span_id` and `bothy.correlation_id`; the OTLP record carries trace and span id natively plus
  `bothy.correlation_id`.
- `telemetry.enabled = false` exports nothing over OTLP. Logs then go to stdout even with
  `telemetry.logs.exporter = otlp`, also when `OTEL_LOGS_EXPORTER=none`; logs are never silently
  lost. The tracer provider still issues trace ids, so `CorrelationID` exists for every request.
- `ServerErrorLog(logger)` is the `http.Server.ErrorLog`: every line becomes the message
  `http server error` with `error.type` `tls_handshake`, `accept`, `panic` or `other`; client
  addresses and foreign texts are discarded (SEC-LOG-5).
- Errors of the SDK and the exporters are logged as `telemetry export error` with `error.type`
  set to the Go type of the error, at most once per 10 s, to stdout only.

## Allowlists

1. Logs: the allowlist handler resolves `LogValuer` values, flattens groups (`slog.Group`,
   `WithGroup`) to dotted keys and keeps an attribute only when its flattened key is
   `attr.Allowed` and its value is a scalar (string, number, bool, duration, time or `[]string`).
   An error or struct value is dropped even under an allowed key: errors are logged as
   `error.type` with a class or code, never as text. The record itself is never dropped.
   Consequence: the group that `errs.Error.LogValue` writes (`code`, `kind`, ...) never reaches an
   output; log `error.type` with the public code instead.
2. Spans: the span exporter rebuilds every span with only `attr.Allowed` keys with scalar values
   on the span, its events and its links, and clears the status description and the trace states
   (free text; the trace state comes from the `tracestate` header of the client). This
   removes `url.path`, `url.query`, `client.address`, `network.peer.*` and whatever a library adds
   later. Server spans also lose `server.address` and `server.port`, which `otelhttp` takes from
   the Host header of the client. The pinned SDK has no supported span attribute filter, so the
   exporter uses `tracetest.SpanStubFromReadOnlySpan(...).Snapshot()`.
3. Log records that reach the logger provider another way are filtered the same way in the log
   processor.
4. HTTP: the access log (message `http request`) has only `http.request.method`, `http.route`
   (pattern or `unmatched`), `http.response.status_code`, `error.type` for 5xx and
   `user_agent.original`. `User-Agent` is the only header read for logging. A method outside the
   nine standard methods is `_OTHER` in logs and metrics and `HTTP` in span names, so a client
   cannot open new values.
5. The collector allowlist (`infra/otel/collector.yaml`) is the second line;
   `TestAttrKeysEqualCollectorAllowlist` keeps it equal to `attr` and `genai`.

## Drop accounting (D3)

Each of traces and logs has one in-flight budget of `OTEL_BSP_MAX_QUEUE_SIZE` or
`OTEL_BLRP_MAX_QUEUE_SIZE` items: an item takes a slot when it enters the front queue and returns
it after its export, failed or not. The front queue is a bounded channel; `OnEnd` and `OnEmit` do
a non-blocking send and count an item that gets no slot in `bothy.telemetry.dropped` with
`bothy.signal` `traces` or `logs`. One goroutine per signal forwards to the SDK batch processor
(spans: `BatchSpanProcessor` with `WithBlocking()`; logs: `BatchProcessor`, which has no blocking
option and drops its oldest record on overflow). The SDK queues are as large as the budget, so
they never overflow and never wait: the SDK never drops silently.

Exporter wrappers count every item of a failed export: spans, log records and metric data points
(`bothy.signal` `metrics`). The counter lives in the same meter provider and is cumulative, so the
count appears as soon as the collector is reachable again.

Not counted: items still queued (front queue, SDK queue or a running export) when `Shutdown`
reaches its deadline; `ForceFlush` at its deadline leaves them queued. A failed last metric export
at `Shutdown` is counted, but no later export carries that count.

## Sampling

- Default: `Middleware` uses the `otelhttp` public-endpoint option. An incoming `traceparent`
  becomes a span link, and the server span is a new root sampled by
  `TraceIDRatioBased(telemetry.traces.sampler_ratio)`.
- `telemetry.traces.trust_remote_parent = true`: the incoming context is the parent; its trace id
  applies and its sampled flag decides (`ParentBased`).
- Local parents follow the parent. With `telemetry.enabled = false` nothing is sampled, but every
  span has a valid trace id.
- `CorrelationID(ctx)` is the trace id of the current span as UUID text (8-4-4-4-12, lowercase);
  it is empty only without a span.

## Routes

`Middleware` puts a route holder into the context. The innermost layer calls
`RecordRoute(r, pattern)` after routing: it names the span `<METHOD> <route>`, sets `http.route`
on the span and on the `otelhttp` labeler (so the duration metric carries it) and stores the
route for the access log. A `ServeMux` directly below `Middleware` is read from `r.Pattern`
without `RecordRoute`. No route means `unmatched`. Only the part of the pattern from the first
slash is used, never the raw path.

The route `RecordRoute` named is also written back to the request `otelhttp` reads after the
handler, so an outer `ServeMux` that only knows a prefix (with `http.StripPrefix` below it) does
not put its own pattern on the duration metric.

A panic of the handler goes on unchanged to the recovery layer outside (02 "Kette im
api-Knoten" step 1) and is re-raised where it was caught, so the logged stack still shows where
it happened. Before that, the access log and the span record status 500 (or the status already
sent) with `error.type`. `otelhttp` records `http.server.request.duration` only when the handler
returns, so a panicking request is missing from that metric; the access log and the span have it.

## Pseudonyms

`bothy.principal.pseudo_id` (privacy.md "Pseudonyme"):

```text
p      = floor(unix_seconds(at) / seconds(privacy.telemetry_pseudonym_rotation)), UTC
K_p    = HMAC-SHA-256(master, "bothy/telemetry-pseudonym/v1/" + decimal(p))
pseudo = hex(HMAC-SHA-256(K_p, lowercase canonical UUID of the principal id))[:32]
```

The master key comes from `telemetry.pseudonym_key_ref` through the secrets provider, which also
handles its rotation; under 32 bytes is a start error. `Pseudonymizer.Attr(ctx, id)` gives the
attribute for now, `Pseudonym` and `PseudonymWithPeriod` for any instant.
`bothy telemetry pseudonym <principal_id> [--at TIME]` prints the pseudonym and the start of its
period for the owner, without a database. Pseudonyms are only for people; agents and apps appear
by handle. Metrics never carry a pseudonym.

## How later tickets instrument

- Spans: `otelapi.Tracer(<package path>).Start(ctx, name)` with a constant name and attributes
  from `attr` or `genai` only; unknown keys are removed on export.
- Metrics: `otelapi.Meter(<package path>)`; names and units from signals.md "Namen und
  Einheiten", attributes only from closed sets (signals.md "Kardinalität"), never an object id or
  a pseudonym.
- Logs: the logger from `Telemetry.Logger()` (or `NewCommandLogger`), a constant English message,
  `slog.Attr` values with `attr` constants and the context variant (`InfoContext`, `LogAttrs`);
  `sloglint` enforces this in `make lint`.
- A new key goes into signals.md "Attribute in Spans und Logs", the collector allowlist and
  `attr` (or `genai`) in one change; `TestAttrKeysEqualCollectorAllowlist` fails otherwise.

## Tests

`go test -race ./internal/platform/otel/...`. The tests use in-memory and fake exporters and a
manual metric reader through the unexported `setup` seam; `TestCollectorUnreachableDoesNotDelayRequests`
uses a `127.0.0.1:0` listener that accepts and never answers. Tests that install globals do not run
in parallel.

## Dependencies

`platform/secrets`; vendors `go.opentelemetry.io/otel/**`, `otelhttp`, `otelslog`
(`architecture/modules.yaml`).
