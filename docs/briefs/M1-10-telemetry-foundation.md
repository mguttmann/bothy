# M1-10: Logging and telemetry foundation

Status: ready | Depends on: M1-09 | Etappe: M1

## Goal

After this ticket the server has one telemetry setup in `internal/platform/otel`: structured
`log/slog` logs that carry only catalog attributes, never headers outside an allowlist, never raw
paths or query strings; OpenTelemetry traces, metrics and logs exported over OTLP to the
Collector, asynchronously, dropping and counting (`bothy.telemetry.dropped`) instead of blocking;
a server-side sampling decision; HMAC pseudonyms for people; and one package that owns every
`gen_ai.*` key. Every later ticket instruments through this package, so content and secrets stay
out of telemetry by construction (B11, R-Telemetrie, ADR 0034).

## Read first

- `docs/spec/17-observability.md`: "Grundregeln", "Signale", "Datenschutz", "Apps" (sampling
  decision), "Tests, die diese Regeln halten", "Konfigurationswerte dieses Kapitels".
- `docs/observability/signals.md`: "Namen und Einheiten", "Kardinalität", "Resource-Attribute",
  "Sicherung, Audit, Telemetrie" (`bothy.telemetry.dropped`), "Weitergabe", "Spans",
  "Attribute in Spans und Logs", "GenAI-Konventionen (gekapselt)", "Logs".
- `docs/observability/privacy.md`: "Was nie in Telemetrie steht", "Zwei Linien", "Pseudonyme".
- `docs/spec/20-datenschutz.md`: "Telemetrie" and "Konfigurationswerte dieses Kapitels"
  (`privacy.telemetry_pseudonym_rotation`).
- `docs/spec/19-sicherheit.md`: "Logs ohne Geheimnisse und ohne Inhalte" (SEC-LOG-1 to SEC-LOG-6).
- `docs/spec/02-architektur.md`: "Kette im api-Knoten" (step 2), "Code-Layout", "Befehle des
  Binarys" (`bothy telemetry pseudonym`), "Ausfall einer Abhängigkeit" (row OTel Collector),
  "Shutdown" (step "Telemetrie leeren").
- `docs/operations/configuration.md`: "OpenTelemetry-Variablen".
- `infra/otel/collector.yaml` (`redaction/allowlist.allowed_keys`, `transform/scrub`).
- `docs/adr/0034-opentelemetry-ueber-collector-lokaler-standard-stack.md`.
- `docs/PLAN.md`: "Arbeitsweise je Ticket", M1 row M1-10, contradiction W8.
- Code you build on: `internal/platform/secrets` (M1-09), `internal/platform/config` and
  `cmd/bothy` (M1-07), `internal/kernel/clock` (M1-05), `.golangci.yml` (M1-05).
- No API operation, data model table or action id is touched by this ticket.

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
6. Test first: write the failing test, show it red, then make it green, then clean up.
7. No en dash or em dash characters in any file you write.
8. White label: no company, person, host or domain names except `example.com`, people Alex, Sam,
   Robin; no paths under a user home directory.
9. Configuration only through named keys listed in `docs/operations/configuration.md`.
10. 17 "Grundregeln" 1 to 5, 8, 9: OpenTelemetry only; export only to a Collector; no content,
    prompts, file names, search terms, tokens or secrets; asynchronous with a bounded queue that
    drops and counts `bothy.telemetry.dropped`, an export error never changes a request result;
    `gen_ai.*` keys in exactly one package with a snapshot test; people only as HMAC pseudonym,
    never on metrics; metric attributes only from closed sets.
11. SEC-LOG-1 to SEC-LOG-3: `log/slog`; headers logged only through an allowlist (`Authorization`,
    `DPoP`, `Cookie`, `Set-Cookie`, `X-Forwarded-*` and proxy identity headers never appear);
    paths only as route patterns, never raw; query strings never.
12. signals.md "Logs": log messages are constant English text enforced by a linter; attributes
    only from "Attribute in Spans und Logs"; errors as `error.type`, never a foreign error message.
13. signals.md "Weitergabe" and 17 R-OB-05: W3C Trace Context; on the public listener the server
    starts its own trace and records an incoming `traceparent` only as a span link (`otelhttp`
    option for public endpoints), so trace id and sampling come from the server; only with
    `telemetry.traces.trust_remote_parent` the incoming context becomes the parent (its trace id
    and sampled flag apply). Every request has a valid trace id, also with
    `telemetry.enabled = false`, and it is the `correlation_id`.
14. privacy.md "Pseudonyme" and 20 "Telemetrie": `bothy.principal.pseudo_id` with a period key
    derived by HMAC from `telemetry.pseudonym_key_ref` per `privacy.telemetry_pseudonym_rotation`
    (the exact derivation is in privacy.md); `bothy telemetry pseudonym <principal_id> [--at]` lets
    the owner map an id to its pseudonym.
15. W8: the package is `internal/platform/otel` (02 wins over `platform/telemetry`).

## Scope

All Go paths are relative to `services/server/`. The package name is `otel`; import the upstream
API with an alias (for example `otelapi "go.opentelemetry.io/otel"`).

### `internal/platform/otel`

- `type Config struct` filled in `internal/wire` from: `telemetry.enabled`,
  `telemetry.traces.sampler_ratio` (0 to 1), `telemetry.traces.trust_remote_parent`,
  `telemetry.logs.exporter` (`otlp`, `stdout`, `both`), `telemetry.logs.level` (`debug`, `info`,
  `warn`, `error`), `telemetry.pseudonym_key_ref`, `privacy.telemetry_pseudonym_rotation`,
  `server.role`, `instance.environment`. Register each key in the config schema if M1-07 did not
  (types and ranges as listed, defaults from the chapter tables).
- `func Setup(ctx context.Context, cfg Config, deps Deps) (*Telemetry, error)` with
  `Deps{ Secrets *secrets.Provider; Clock clock.Clock; Stdout io.Writer; ServiceVersion string }`.
  Installs global tracer, meter and logger providers and the W3C `propagation.TraceContext`
  propagator (no baggage). `(*Telemetry).Logger() *slog.Logger`,
  `(*Telemetry).Pseudonymizer() *Pseudonymizer`, `(*Telemetry).Shutdown(ctx) error` (flush and
  close all exporters within the context deadline; the caller passes
  `shutdown.telemetry_flush_timeout`).
- `func NewCommandLogger(w io.Writer, level slog.Level) *slog.Logger`: JSON to `w`, same
  attribute allowlist, no OTLP. Used by every short-lived command and before configuration exists.
- `func Middleware(next http.Handler) http.Handler`: outer HTTP middleware (otelhttp server span,
  access log, route holder). `func RecordRoute(r *http.Request, pattern string)`: called by the
  innermost layer after routing (M1-18 wires both into the chain; this ticket tests them with a
  `http.ServeMux`).
- `func CorrelationID(ctx context.Context) string`: the trace id of the current span as UUID
  text (16 bytes, signals.md "Weitergabe" item 3); empty only without a span.
- `type Pseudonymizer` with `Pseudonym(ctx context.Context, principalID string, at time.Time) (string, error)`
  and `Attr(ctx, principalID) (attribute.KeyValue, error)` for key `bothy.principal.pseudo_id`.
- Dropped-telemetry accounting (D3) and the exporter construction (D2).

### `internal/platform/otel/attr`

Exported constants for every key of signals.md "Attribute in Spans und Logs" and for every other
non-resource key of `infra/otel/collector.yaml` `redaction/allowlist.allowed_keys` except the
`gen_ai.*` keys (those live in `genai`): the HTTP, database, error and status keys such as
`http.request.method`, `http.route`, `http.response.status_code`, `error.type`,
`server.address`, `server.port`, `user_agent.original`, `db.system.name`, `db.namespace`,
`db.operation.name`, `db.collection.name`, `db.query.text`, `exception.type`.
`func Allowed(key string) bool` is true for these and for the `genai` constants. Signal
`bothy.telemetry.dropped` uses attribute `bothy.signal` (`traces`, `metrics`, `logs`), a
metric-only attribute; add it as a constant documented as metric-only.

### `internal/platform/otel/genai`

Constants for exactly the `gen_ai.*` keys of the collector allowlist (`gen_ai.operation.name`,
`gen_ai.provider.name`, `gen_ai.request.model`, `gen_ai.response.model`,
`gen_ai.response.finish_reasons`, `gen_ai.usage.input_tokens`, `gen_ai.usage.output_tokens`,
`gen_ai.usage.cache_read.input_tokens`, `gen_ai.usage.cache_write.input_tokens`,
`gen_ai.agent.id`, `gen_ai.agent.name`, `gen_ai.conversation.id`, `gen_ai.tool.name`),
`var Forbidden = []string{...}` with the never-sent keys of signals.md "GenAI-Konventionen"
item 4, and `const ConventionRef` naming the convention repository and its status as read on
2026-10-10 (no commit is recorded in signals.md; do not invent one). Golden file
`testdata/keys.golden`.

### `internal/platform/buildinfo` (18 "Komponenten" rule 5)

`func Version() string` returning a constant equal to the content of `services/server/VERSION`;
a unit test reads `../../../VERSION` and fails when they differ; no linker flags. This ticket
creates the package because it is its first user (`service.version`); M1-15 and M1-23 use it.

### `cmd/bothy`

- Every command builds its logger with `NewCommandLogger` (stderr, level from config when loaded,
  `info` before). `serve` (M1-11) will call `Setup`; until then nothing calls `Setup` outside tests.
- New command `bothy telemetry pseudonym <principal_id> [--at <RFC 3339 time>]`: loads config,
  resolves `telemetry.pseudonym_key_ref`, prints the pseudonym for the period that contains `--at`
  (default now) and the period start in UTC. Needs no database.

### Linter

`.golangci.yml`: enable `sloglint` with constant messages, attribute-only arguments, no raw keys
and context-aware calls (in golangci-lint 2.14.0 the options are, to be verified with
`golangci-lint config verify`: `static-msg: true`, `attr-only: true`, `no-raw-keys: true`,
`context: scope`). Keys then come from `otel/attr` constants.

### Modules register

`architecture/modules.yaml`: vendors `go.opentelemetry.io/otel/**`,
`go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`,
`go.opentelemetry.io/contrib/bridges/otelslog` for the platform component; regenerate
`.go-arch-lint.yml`.

## Out of scope

- Wiring the middleware into the request chain (M1-18), the actor and its pseudonym on spans
  (M1-18, M1-34).
- Calling `Setup` and `Shutdown` from `bothy serve` (M1-11).
- The catalog metrics of the server core (`bothy.authz.decisions` and others, M1-68), database
  spans (M1-12), the canary test across all outputs (M1-69).
- Collector, dashboards and alert files (M1-04 checks them).
- A key rotation reminder in the admin console (M6).

## Boundaries

- Work in a dedicated git worktree on branch `feat/m1-10-telemetry-foundation`.
- Conventional Commits with scope (`feat(otel): ...`) and DCO sign-off; squash before the merge.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except `services/server/CHANGELOG.md`,
  `docs/DECISIONS.md` and the docs named in "Definition of done".
- Follow the host rules supplied by the launcher. Stop only processes you started, by PID.
- Temporary directories only via `mktemp -d` or `t.TempDir()`, removed at the end.
- Never read or print real secrets. No network calls except package downloads for pinned modules
  and tools; tests talk only to listeners they open on `127.0.0.1` port 0.
- A missing core-contract piece means `Verdict: BLOCKED`, not a deviation.

## Design notes

D1. Logs.

- Handler chain: level filter (`telemetry.logs.level`) -> allowlist filter -> fan-out to the
  stdout JSON handler and/or the otelslog bridge, per `telemetry.logs.exporter`.
- The allowlist filter drops every attribute whose key is not `attr.Allowed`, including keys
  inside groups; it never drops the record. Every record with a span in its context gets
  `trace_id`, `span_id` and `bothy.correlation_id`.
- `telemetry.enabled = false` disables OTLP export of all signals; logs then go to stdout even
  when `telemetry.logs.exporter = otlp` (logs are never silently lost). Record in DECISIONS.md.
- Access log: one record per request, message `http request`, attributes
  `http.request.method`, `http.route` (pattern or `unmatched`), `http.response.status_code`,
  `error.type` when the status is 5xx, `user_agent.original`. Header allowlist is exactly
  `User-Agent -> user_agent.original`; no other header is ever read for logging.
- `http.Server.ErrorLog` messages contain client addresses (SEC-LOG-5); provide
  `func ServerErrorLog(l *slog.Logger) *log.Logger` that logs the constant message
  `http server error` without the original text (M1-11 uses it).

D2. Exporters and resource.

- Build exporters directly (not `autoexport`, because D3 wraps them): `otlptracegrpc` or
  `otlptracehttp`, `otlpmetricgrpc` or `otlpmetrichttp`, `otlploggrpc` or `otlploghttp`, chosen by
  `OTEL_EXPORTER_OTLP_PROTOCOL` and its per-signal variants (`grpc`, `http/protobuf`; default
  `grpc`). They read endpoint, headers and timeouts from the standard `OTEL_*` variables.
  `OTEL_TRACES_EXPORTER`, `OTEL_METRICS_EXPORTER`, `OTEL_LOGS_EXPORTER` set to `none` disable one
  signal.
- Resource: start from `resource.Environment()`, then set `service.name = bothy-server`,
  `service.namespace = bothy`, `service.version` (build version), `service.instance.id` (hostname
  plus process start time in Unix nanoseconds), `bothy.server.role` (`server.role`); these win
  over `OTEL_RESOURCE_ATTRIBUTES`. `deployment.environment.name` comes from
  `OTEL_RESOURCE_ATTRIBUTES` when set there, else from `instance.environment`.
- Metric temporality cumulative (Prometheus path); the collector converts per backend.

D3. Never block, always count.

- Spans: a front queue (bounded channel, size from `OTEL_BSP_MAX_QUEUE_SIZE`, falling back to
  the SDK's own default for that variable) in a custom `SpanProcessor`; `OnEnd` does a
  non-blocking send and on a full queue increments `bothy.telemetry.dropped{bothy.signal=traces}`.
  One goroutine forwards to an SDK `BatchSpanProcessor` created with `WithBlocking()`, so only
  that goroutine ever waits and the SDK itself never drops silently.
- Logs: same front queue in a custom `sdk/log` `Processor` in front of the SDK batch processor.
  If the pinned `sdk/log` batch processor can drop on overflow without a blocking option, size its
  queue to the front queue plus one export batch and state in the README which overflow is not
  counted.
- Exporter wrappers for all three signals count every item of a failed export as dropped (spans,
  log records, metric data points) with the matching `bothy.signal`.
- The counter lives in the same meter provider; it is cumulative and visible as soon as the
  collector is reachable again.
- `ForceFlush` and `Shutdown` respect the context deadline and never hang beyond it.

D4. Sampling and remote context. `Middleware` wraps `otelhttp` with the public-endpoint option
unless `telemetry.traces.trust_remote_parent` is true: an incoming `traceparent` becomes a span
link and the server span is a new root sampled by `TraceIDRatioBased(sampler_ratio)`. With
`trust_remote_parent` the incoming context is the parent: `ParentBased` with `AlwaysSample` for a
sampled remote parent and `NeverSample` for a not sampled one. Local parents follow the parent.
With `telemetry.enabled = false` the SDK tracer provider still exists with `NeverSample` and no
exporter, so trace ids and `CorrelationID` exist for every request (17 "Signale").

D5. HTTP spans and routes.

- `Middleware` wraps `otelhttp.NewHandler` with span name `<METHOD> <http.route>` once the route is
  known. `RecordRoute` sets on the current span the name and `http.route`, adds `http.route` to the
  `otelhttp` labeler (so `http.server.request.duration` carries it), and stores the pattern in a
  holder the outer middleware put into the context (a request copy below `otelhttp` cannot carry
  it back). No route recorded means `unmatched`.
- Source-side allowlist for spans: wrap the span exporter so every exported span, its events and
  links keep only `attr.Allowed` keys (this removes `url.path`, `url.query`, `client.address` and
  any future otelhttp attribute). Rebuild spans with `tracetest.SpanStubFromReadOnlySpan(...)` and
  `.Snapshot()` unless the pinned SDK offers a supported attribute filter; document the choice.

D6. Pseudonyms (as privacy.md "Pseudonyme" and 20 "Telemetrie" state them).

- Master key: `telemetry.pseudonym_key_ref` through the secrets provider; fewer than 32 bytes is a
  start error. The provider cache handles rotation of the master key.
- Period index `p = floor(unix_seconds(at) / seconds(privacy.telemetry_pseudonym_rotation))`, UTC.
- Period key `K_p = HMAC-SHA-256(master, "bothy/telemetry-pseudonym/v1/" + decimal(p))`.
- Pseudonym `hex(HMAC-SHA-256(K_p, lowercase canonical UUID text of the principal id))[:32]`.
- Effect: one secret to escrow (16 "Hinterlegung"), pseudonyms not linkable across periods for
  anyone without the master key (20), the owner can still map an id for any period with
  `bothy telemetry pseudonym --at`.
- Pseudonyms only for human principals; agents and apps appear by handle (privacy.md item 4).
  Metrics never carry the pseudonym; `attr` documents `bothy.principal.pseudo_id` as span and log
  only.

D7. W8: the package is `internal/platform/otel` (02); signals.md "GenAI-Konventionen" item 2
already names `internal/platform/otel/genai`.

## Tests (write them first)

- `TestCollectorUnreachableDoesNotDelayRequests`: OTLP endpoint is a `127.0.0.1:0` TCP listener
  that accepts and never answers; short export timeout through the standard variable; serve
  enough requests through `Middleware` to overflow the front queue; every request finishes within
  a bound defined in the test; after `ForceFlush` with a deadline the counter
  `bothy.telemetry.dropped{bothy.signal=traces}` read from a manual reader is greater than zero.
- `TestFailedExportCountsExactly`: fake exporter that fails; N spans and M log records; after flush
  the counter equals N and M per signal.
- `TestQueueFullDropsAndCounts`: fake exporter that blocks; queue size Q via the variable; end Q+K
  spans; at least K counted, `OnEnd` never blocks.
- `TestRemoteParentIsLinkByDefault`: with an incoming `traceparent` the server span has a new
  trace id, a link to the incoming span context, and its sampling follows the ratio (ratio 0:
  not sampled although the incoming flag is sampled).
- `TestTrustRemoteParent`: with `telemetry.traces.trust_remote_parent` the trace id equals the
  incoming one and the incoming sampled flag decides.
- `TestEveryRequestHasCorrelationID` (also with `telemetry.enabled = false`).
- `TestResourceAttributesNotOverridden`: `OTEL_RESOURCE_ATTRIBUTES=service.name=x,bothy.server.role=y,deployment.environment.name=staging`
  gives `bothy-server`, the configured role and `staging`.
- `TestLogsNeverContainSecretsHeadersOrRawPaths` (SEC-LOG-2, SEC-LOG-3): request to
  `/api/v1/hooks/{webhook_id}/{token}` with path values, query `?token=<canary>`, headers
  `Authorization`, `DPoP`, `Cookie`, `X-Forwarded-For` and a custom header carrying canaries; the
  stdout JSON, the exported log records and the exported spans contain no canary; `http.route` is
  the pattern; an unrouted request logs `unmatched`.
- `TestAllowlistFilterDropsUnknownAttributes` (logs and spans).
- `TestAttrKeysEqualCollectorAllowlist`: the set of non metric-only keys in `otel/attr` plus the
  `genai` constants equals the non-resource keys of `infra/otel/collector.yaml`
  `redaction/allowlist.allowed_keys` (resource keys are those of signals.md
  "Resource-Attribute" plus `telemetry.sdk.*`); a difference in either direction fails.
- `TestGenAIKeysSnapshot` (golden `testdata/keys.golden`, `-update` flag) and
  `TestGenAIKeysAllowedOrScrubbed`: each constant is in the collector allowlist, each `Forbidden`
  key is deleted by `transform/scrub` or absent from the allowlist.
- `TestPseudonymStableAndRotating`: same key, id and period give the same 32 hex characters;
  another master key gives another value; the next period gives another value; a 31-byte key is a
  start error.
- `TestTelemetryPseudonymCommand`: output for a known test key and id equals the library value.
- `TestServerErrorLogHasNoDetails`.
- `TestVersionMatchesVERSIONFile` in `internal/platform/buildinfo`.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/platform/otel/... ./cmd/bothy/...
make lint
make arch
make check
```

## Acceptance criteria

- [ ] An unreachable collector delays no request and increases `bothy.telemetry.dropped`.
- [ ] Failed exports and queue overflow are counted per signal.
- [ ] An incoming `traceparent` is only a span link unless `telemetry.traces.trust_remote_parent`;
      trace id and sampling come from the server by default.
- [ ] `Authorization`, `DPoP`, `Cookie`, query strings and raw paths never appear in logs or spans
      (SEC-LOG-2, SEC-LOG-3); routes appear as patterns.
- [ ] Pseudonyms are stable per key and period and change with key and period; keys under 32
      bytes fail the start.
- [ ] The `gen_ai` snapshot test exists and passes.
- [ ] `sloglint` enforces constant messages and attribute keys in `make lint`.
- [ ] `bothy telemetry pseudonym` works without a database.
- [ ] `make check` is green with module coverage at or above `quality.coverage.go_min_percent`.

## Definition of done

- Code and tests; doc comment on every exported symbol.
- READMEs: `internal/platform/otel/README.md` (setup, signals, allowlists, drop accounting and any
  overflow that is not counted, sampling, pseudonyms, how later tickets add spans, metrics and log
  keys), `internal/platform/otel/genai/README.md`, `internal/platform/buildinfo/README.md`.
- Docs: privacy.md, 20, 17 and signals.md already state D4, D6 and W8; change them only if the
  build proves a statement wrong. `docs/spec/02-architektur.md` "Befehle des Binarys" gets the
  `--at` flag.
- `docs/DECISIONS.md`: D1 (disabled telemetry keeps stdout logs), D3 (drop accounting).
- Configuration reference regenerated without diff.
- `services/server/CHANGELOG.md` entry under "Unreleased".
- No new ADR: ADR 0034 covers the architecture; the rulings go to DECISIONS.md.
- `make check` green; independent review and red team (`docs/quality/red-team-gate.md`) with
  verdict `JA` recorded; merge locally into the base branch only if both are green, then
  `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then
commits, files with line counts, test results with counts (including the red runs), what is not
proven (for example overflow inside the SDK log processor if it cannot be counted), follow-ups
that could not be done in scope with the reason. Then write `{{DONE_PATH}}` with
`exit=0 sha=<merge sha>` or `exit=1 <reason>`.
