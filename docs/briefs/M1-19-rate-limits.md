# M1-19: Rate limits per class

Status: ready | Depends on: M1-18 | Etappe: M1

## Goal

After this ticket token buckets per rate-limit class and key run in the chain stage `StageRateLimit`, before `authorize`. Every response of a limited REST operation carries `RateLimit-Policy` and `RateLimit`; an exhausted bucket answers `429 rate_limited` with `Retry-After`; memory stays bounded however many distinct keys arrive. The limiter is a reusable component: the gateway tickets (M1-64 for `rl.gateway_connect`, M1-66 for `rl.typing`) call the same API.

## Read first

- `docs/PLAN.md`: section "M1 Fundament" (rows M1-17 to M1-20, M1-37, M1-38, M1-64, M1-66, M1-68; exit criteria), "Entscheidungen dieses Plans" (P-07 to P-12).
- `docs/spec/05-aktionen.md`: "Rate-Limit-Klassen", "Entscheidung" (rate limits run in their own layer before `authorize`), "Konfigurationswerte dieses Kapitels".
- `docs/spec/06-api.md`: "Rate-Limits", "Grundregeln" (row Client-IP), "Fehler" (`rate_limited`), "Entscheidungen dieses Kapitels" (R-API-11), "Quellen" (Q5).
- `docs/spec/02-architektur.md`: "Request-Pfad", "Skalierungspfad" (limits per node), "Entscheidungen dieses Kapitels" (R-AR-12), "Konfigurationswerte dieses Kapitels".
- `docs/spec/08-identitaet.md`: "Schutz der Anmeldung", test ID-T25. `docs/spec/16-betrieb.md`: "Rand und Identität".
- `docs/spec/19-sicherheit.md`: SEC-AI-7, "Tests" (SEC-T03). `docs/security/threat-model.md`: row T-DS-01.
- `docs/observability/signals.md`: `bothy.ratelimit.rejected` and attribute `bothy.ratelimit.class`.
- `docs/spec/actions.yaml` (classes per action, M1-02).
- Code: `internal/platform/httpserver` and `httpservertest` (M1-18), `api/routes` (M1-17), `internal/kernel/{actor,reqmeta,clock,errs}`, `internal/platform/{config,otel,lifecycle}`, `internal/wire`.
- `docs/quality/gates.md`, `CONTRIBUTING.md`.

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or doc comment (purpose, parameters, return value, errors, example where useful); comments explain why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc, Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision, a CHANGELOG entry per release, docs site kept current. A change without matching documentation is not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the merge.
6. Test first: write the failing test, show it red, then make it green. No en or em dashes in any text. White label everywhere. Configuration values only through named keys from `docs/operations/configuration.md`.
7. 05 "Rate-Limit-Klassen" and 06 "Rate-Limits" point 1: one token bucket per class and key with `ratelimit.<class>.per_minute` and `ratelimit.<class>.burst` (for example `ratelimit.rl.read.per_minute`); the check runs before `authorize`.
8. 06 "Rate-Limits" point 2 (draft-ietf-httpapi-ratelimit-headers-11, provisional): `RateLimit-Policy: "<class>";q=<per_minute>;w=60` and `RateLimit: "<class>";r=<whole tokens>;t=<seconds until full>`.
9. 06 "Rate-Limits" point 3: an exhausted bucket gives `429 rate_limited` with `Retry-After` (RFC 9110).
10. Keys per class as in the 05 table column "Schlüssel": Client-IP, Prinzipal, Gerät bzw. Token, Verbindung, Agent, Eingangs-Webhook.
11. The client IP comes only from forwarding headers of peers in `server.trusted_proxies` (08 "Schutz der Anmeldung", ID-T25), resolved by M1-18 into `kernel/reqmeta`.
12. R-AR-12: limits live in the process; with several api nodes they apply per node.

## Scope

Paths are relative to `services/server/` unless they start with `docs/`.

1. `internal/platform/ratelimit` (new): limiter, buckets, shards, sweeper, key derivation, header formatting, HTTP middleware constructor; `doc.go` and `README.md`.
2. `internal/wire`: build the limiter from configuration, register it with `Chain.Use(httpserver.StageRateLimit, ...)` for roles `api` and `all`, start the sweeper under the lifecycle of M1-11 and stop it on shutdown, run the start check below.
3. Configuration schema (M1-07): the 30 keys `ratelimit.rl.<name>.per_minute` and `.burst` of 05 "Konfigurationswerte dieses Kapitels" (if not yet present) and two new keys `ratelimit.max_tracked_keys` and `ratelimit.sweep_interval` (Design notes).
4. Docs: rows for the two new keys in `docs/spec/02-architektur.md` "Konfigurationswerte dieses Kapitels"; in `docs/spec/06-api.md` "Rate-Limits" the rounding of `r`, `t` and `Retry-After`, the anonymous fallback to the client IP, and the per-node note; `docs/DECISIONS.md` entries; `docs/operations/configuration.md` regenerated; `services/server/CHANGELOG.md`.

## Out of scope

- Using the limiter in the gateway (`rl.gateway_connect` at connection setup in M1-64, `rl.typing` in M1-66). This ticket provides the API and the key rules.
- The per-code and per-account login protections of 08 "Schutz der Anmeldung" other than `rl.public_auth` (`auth.login_fail_limit` and similar): identity tickets M1-37 to M1-39.
- Upload session limits of 12 (M3), maintenance mode (M7-08), idempotency (M1-20).
- A shared limiter across nodes (02 "Skalierungspfad" keeps it per node).
- Metrics other than `bothy.ratelimit.rejected` (M1-68).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-19-rate-limits`.
- Conventional Commits with the component as scope (for example `feat(ratelimit): ...`) and DCO sign-off (`git commit -s`). Squash into meaningful commits before the merge.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside Scope, except the docs and changelog entries named here.
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp`; remove them when done.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Tests bind only `127.0.0.1` and port 0 (P-10).
- Do not lower a threshold, disable a gate or delete a test.
- A gap in the core contract (chapters 00, 03, 04, 05) that this ticket needs ends the ticket with `Verdict: BLOCKED` (P-11).

## Design notes

### Bucket arithmetic

For a class with `per_minute` P and `burst` B: rate R = P / 60 tokens per second, capacity B, a new bucket starts full. On each request at time `now` (from `kernel/clock`): tokens = min(B, tokens + elapsed seconds x R); if tokens >= 1, subtract 1 and allow, else deny. Header values after the decision: `r` = floor(tokens); `t` = ceil((B - tokens) / R) seconds, 0 when full; `q` = P; `w` = 60. On deny: `Retry-After` = ceil((1 - tokens) / R) seconds, at least 1. Store tokens as int64 fixed point (for example nanotokens) or float64; tests check exact header values with a fake clock.

### API (`internal/platform/ratelimit`)

```go
type Class string                  // for example "rl.read"
type KeyKind int                   // KeyClientIP, KeyPrincipal, KeyAgent, KeyDeviceOrToken, KeyConnection, KeyIncomingWebhook
type Limits struct{ PerMinute, Burst int }
type Config struct {
	Classes        map[Class]Limits    // from ratelimit.<class>.per_minute and .burst
	MaxTrackedKeys int                 // ratelimit.max_tracked_keys
	SweepInterval  time.Duration       // ratelimit.sweep_interval
}
type Decision struct {
	Allowed           bool
	Remaining         int // r
	ResetSeconds      int // t
	RetryAfterSeconds int // only when denied
	Limits            Limits
}
func New(cfg Config, clk clock.Clock, m metric.Meter) (*Limiter, error) // PerMinute >= 1 and Burst >= 1 for every class, MaxTrackedKeys >= 1
func (l *Limiter) Allow(class Class, key string) (Decision, error)     // error for an unknown class
func (l *Limiter) Len() int                                             // tracked buckets
func (l *Limiter) Sweep()                                               // removes full buckets
func (l *Limiter) Run(ctx context.Context)                              // sweeps every SweepInterval until ctx ends
func KindOf(class Class) (KeyKind, bool)
func HeaderValues(class Class, d Decision) (policy, state string)
func Middleware(l *Limiter) func(http.Handler) http.Handler             // for httpserver.StageRateLimit
```

Use the metric API of M1-10 for `bothy.ratelimit.rejected` (counter, attribute `bothy.ratelimit.class`, values from the closed class list).

### Key per class

| Class | Kind | Key value |
|---|---|---|
| `rl.public_auth` | Client-IP | `reqmeta.ClientIP` |
| `rl.gateway_connect` | Gerät bzw. Token | device ID of the actor, else its API token ID, else the client IP (the gateway decides what it knows at connection setup) |
| `rl.read`, `rl.write`, `rl.message`, `rl.trigger`, `rl.interaction`, `rl.upload`, `rl.search`, `rl.admin`, `rl.export`, `rl.import` | Prinzipal | actor ID |
| `rl.agent_runtime` | Agent | actor ID (the agent) |
| `rl.typing` | Verbindung | gateway connection ID passed by the caller |
| `rl.webhook_in` | Eingangs-Webhook | path value `webhook_id` (`r.PathValue`) |

Principal-keyed classes with the anonymous actor use the client IP, so unauthenticated calls never share one bucket and never borrow a principal's bucket. Internal map key: `<class>` + `|` + `<kind prefix>:<value>`, so classes never share buckets. If `docs/spec/actions.yaml` (M1-02) carries the key kind per class, take the kind table from it (generated); otherwise keep a hand-written table and add `TestKeyKinds_MatchChapter05`, which parses the 05 table "Rate-Limit-Klassen" and compares class and key column.

Start check in `internal/wire`: every class used by an enabled operation (`routes.Enabled()`) and every class of the catalogue has limits in the configuration; an enabled REST operation with class `rl.typing` or `rl.gateway_connect` fails the start, because those classes belong to the gateway.

### Middleware

Reads the `routes.Operation` that M1-18 puts into the context in `StageRoute`, derives the key, calls `Allow`, and sets `RateLimit-Policy` and `RateLimit` before calling the next handler, so every response of the route carries them (success and error). On deny it writes `429 rate_limited` with `Retry-After` through the M1-18 problem writer, increments `bothy.ratelimit.rejected` and does not call the next handler. Because the stage runs before `authorize`, the answer to a limited request is the same for every resource ID (no existence leak). Rate-limit rejections are not audited (05 audits `authorize` decisions).

### Bounded memory

Buckets live in a fixed number of shards (map plus mutex each; shard chosen by FNV-1a of the internal key). A full bucket decides exactly like a missing bucket, so removing it changes no decision: `Sweep` removes every bucket that is full at sweep time, and `Run` calls it every `ratelimit.sweep_interval`. Hard cap: at most `ratelimit.max_tracked_keys` buckets in total, spread evenly over the shards (at least one per shard). Inserting into a full shard first removes full buckets found in a small sample of that shard (Go map iteration gives a random sample; the sample size is a code constant with a comment), and only if none is full evicts the sampled bucket with the oldest last use. An evicted key gets a fresh bucket on its next request; this happens only when more distinct keys are active within one refill window than the cap allows (a distributed flood). The limiter never blocks or denies because the table is full. Record this policy in `docs/DECISIONS.md`.

New configuration values in 02 "Konfigurationswerte dieses Kapitels": `ratelimit.max_tracked_keys` with default `100000` and `ratelimit.sweep_interval` with default `1m`. The README explains how to estimate memory from `BenchmarkLimiter_BytesPerKey` and `ratelimit.max_tracked_keys`.

### Several nodes

Limits are per process. The README and 06 "Rate-Limits" note that with n api nodes the effective limit is up to n times the configured value (02 "Skalierungspfad").

### Decisions to record in `docs/DECISIONS.md`

Rounding of `r`, `t` and `Retry-After`; anonymous fallback to the client IP; key choice for `rl.gateway_connect` and `rl.webhook_in`; eviction policy and the two new configuration values.

## Tests (write them first)

Unit tests with a fake clock, `internal/platform/ratelimit`:
- `TestBucket_StartsFull`, `TestBucket_BurstThenDeny` (exactly `burst` requests allowed at a frozen clock, then deny with `Retry-After` = ceil(60 / per_minute)), `TestBucket_Refill`, `TestBucket_NeverAboveBurst`.
- `TestHeaders_Format`: exact strings, for example policy `"rl.write";q=120;w=60` and state `"rl.write";r=29;t=1` after one request with defaults; `TestHeaders_RoundingFloorAndCeil`.
- `TestKeys_PerClass` (table over all 15 classes), `TestKeys_AnonymousUsesClientIP`, `TestKeys_ClassesIndependent` (draining `rl.write` leaves `rl.read` untouched), `TestKeys_WebhookPathValue`, `TestKeyKinds_MatchChapter05` (only with a hand-written table).
- `TestConfig_RejectsZeroValues`, `TestConfig_EveryCatalogueClassConfigured`, `TestAllow_UnknownClassErrors`.
- `TestMemory_BoundedUnderManyKeys` (three times `max_tracked_keys` distinct keys: `Len()` never exceeds the cap), `TestSweep_RemovesOnlyFullBuckets`, `TestEvict_PrefersFullThenOldest`, `TestRun_StopsOnCancel`.
- `TestConcurrent_NoOverAdmission` (`-race`; many goroutines on one key at a frozen clock: allowed equals `burst`).
- `TestMetric_RejectedCounter` (manual metric reader).
- `BenchmarkLimiter_Allow`, `BenchmarkLimiter_BytesPerKey`.

HTTP tests through `httpservertest` (M1-18):
- `TestHTTP_429ProblemWithRetryAfter`: problem body (`code` `rate_limited`, `correlation_id`), `Content-Type: application/problem+json`, `Retry-After`, the SEC-T03 headers.
- `TestHTTP_HeadersOnSuccessAndError`: both headers on 200 and on a 404 of the same route.
- `TestHTTP_RateLimitBeforeAuthorize`: a limited request never reaches the handler; the 429 is identical for a foreign and a missing resource ID.
- `TestIDT25_SpoofedForwardedForCountsPeer`: with `server.trusted_proxies` not containing the test peer, requests with changing `X-Forwarded-For` to an `rl.public_auth` operation share one bucket and get 429 after `burst`; with the peer trusted they get separate buckets.
- `TestStart_RejectsGatewayClassOnRESTOperation` (`internal/wire`, fixture operation with `rl.typing`).

Commands that must pass: `cd services/server && go test -race ./internal/platform/ratelimit/... ./internal/platform/httpserver/... ./internal/wire/...`, `make config-reference` (or the M1-08 generator) without a diff afterwards, `make docs`, `make arch`, `make check`.

## Acceptance criteria

- [ ] After `burst` requests the next one gets `429 rate_limited` with `Retry-After` as a problem.
- [ ] `RateLimit-Policy` and `RateLimit` match 06 "Rate-Limits" exactly on success and error responses of limited operations.
- [ ] Keys follow 05 "Rate-Limit-Klassen" (client IP, principal, device or token, connection, agent, incoming webhook); anonymous requests fall back to the client IP.
- [ ] Values come from `ratelimit.<class>.per_minute` and `.burst`; invalid values stop the start.
- [ ] A spoofed `X-Forwarded-For` from an untrusted peer counts against the peer (ID-T25).
- [ ] Memory stays bounded under many keys; full buckets disappear with the sweep.
- [ ] The check runs before `authorize`.
- [ ] `make check` is green.

## Definition of done

- Code and tests as above; `make check` green.
- Documentation: `internal/platform/ratelimit/README.md` (classes, keys, headers, memory estimate, per-node note); doc comment on every exported symbol (purpose, parameters, return value, errors, example where useful), comments explain why; the spec rows and `docs/DECISIONS.md` entries; regenerated configuration reference; a `services/server/CHANGELOG.md` entry under Unreleased. An ADR only if you take an architecture decision beyond R-AR-12.
- Independent review over the diff (`make review` plus a code review) and the red team gate (`docs/quality/red-team-gate.md`) over the diff, with the verdict `JA` or `NEIN` recorded in the report. On `NEIN`: fix, one test per finding, recheck the delta.
- Merge locally into the base branch only if review and red team are green and `make check` passes; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then commits, files with line counts, test results with counts (including the red run of each new test and the benchmark figures), what is not proven, and follow-ups that could not be done in scope with the reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
