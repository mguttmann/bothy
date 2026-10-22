# Package `platform/ratelimit`

Status: built (M1-19). Normative are [05-actions.md](../../../../../docs/spec/05-actions.md)
"Rate-limit classes" and "Configuration values of this chapter",
[06-api.md](../../../../../docs/spec/06-api.md) "Rate limits",
[02-architecture.md](../../../../../docs/spec/02-architecture.md) "Chain in the api node",
"Scaling path" and R-AR-12, and [08-identity.md](../../../../../docs/spec/08-identity.md)
"Login protection" (ID-T25); the chapters win on any difference. Rulings:
[DECISIONS.md](../../../../../docs/DECISIONS.md) M1-19.

## Interface

- `New(Config, clock.Clock, metric.MeterProvider)` (nil provider: the global one; the counter
  `bothy.ratelimit.rejected` comes from the declaration `metrics.RateLimitRejected` of the metric
  registry, M1-68): the limiter. `Config` holds the limits of every class
  (`ratelimit.<class>.per_minute`, `.burst`), `ratelimit.max_tracked_keys` and
  `ratelimit.sweep_interval`; `internal/wire` binds it (`RateLimitConfig`). Every class of 05 must
  have limits between 1 and `MaxLimit`.
- `Limiter.Allow(class, key)` takes a token, `Limiter.Peek(class, key)` only looks,
  `Limiter.Refund(class, key)` gives a reserved token back (at most up to the burst),
  `Limiter.Reject(class)` counts a refusal made on a `Peek`, `Limiter.Len()`, `Limiter.Sweep()`,
  `Limiter.Run(ctx)` (the sweeper; `internal/wire` starts it and stops it in `PhaseHTTP`).
- `Classes()`, `KindOf(class)`, `GatewayClass(class)`: the class table of 05.
- Keys: `KeyForIP`, `KeyForPrincipal`, `KeyForAgent`, `KeyForHost`, `KeyForDevice`,
  `KeyForToken`, `KeyForConnection`, `KeyForWebhook`, `DeviceOrTokenKey(actor)` and
  `RequestKey(r, class)` for REST requests.
- `HeaderValues(class, decision)`: the values of `RateLimit-Policy` and `RateLimit`.
- `Install(chain, limiter)`: `Middleware` as `httpserver.StageRateLimit`, plus the
  failed-credentials precheck of `StageAuthenticate`.
- `Limiter.FailedCredentialGuard(r)` (M2-27): the failed-credentials rule for a credential check
  outside `StageAuthenticate` (the token check of the incoming webhooks at
  `httpserver.StageWebhookToken`); see "Failed credentials".

The gateway uses the same API: M1-64 calls `Allow("rl.gateway_connect", DeviceOrTokenKey(actor))`
at `identify` and `resume` once the credentials are verified, M1-66 calls
`Allow("rl.typing", KeyForConnection(id))`.

## Classes and keys

| Class | Key | Key value |
|---|---|---|
| `rl.public_auth` | client IP | `ip:<address>`, IPv6 by its /64 prefix, IPv4-mapped IPv6 unmapped |
| `rl.gateway_connect` | device or token | `d:<device id>`, else `t:<API token id>` (gateway only) |
| `rl.read`, `rl.write`, `rl.message`, `rl.trigger`, `rl.interaction`, `rl.upload`, `rl.search`, `rl.admin`, `rl.export`, `rl.import`, `rl.terminal` | principal | `p:<actor id>` |
| `rl.agent_runtime` | agent | `a:<actor id>` |
| `rl.host` | host | `h:<actor id>` |
| `rl.typing` | connection | `c:<connection id>` (gateway only) |
| `rl.webhook_in` | incoming webhook | `w:<path value webhook_id>` |

A principal, agent or host class with the anonymous actor, or with an actor whose API token the
authenticator found revoked or expired (`CredentialInactive`, M1-44), counts by the client IP
instead (IPv6 by /64): unauthenticated calls and dead tokens never share one bucket and never
borrow a principal's. The client IP
is the one `platform/httpserver` resolved into `kernel/reqmeta`: forwarding headers count only from
peers in `server.trusted_proxies` (ID-T25). The internal map key is `<class>|<key>`, so classes
never share buckets. `TestKeyKinds_MatchChapter05` holds the hand-written class table against the
table of 05.

## Headers and rounding

`RateLimit-Policy: "<class>";q=<per_minute>;w=60` and
`RateLimit: "<class>";r=<whole tokens>;t=<seconds until full>` (draft-ietf-httpapi-ratelimit-headers-11,
provisional, R-API-11) go on every response of a limited operation from `StageRateLimit` on. `r`
rounds down, `t` and `Retry-After` round up, `Retry-After` is at least 1. An exhausted bucket gives
`429 rate_limited` with `Retry-After`; the handler never runs, so the answer is the same for every
resource ID. Tokens are fixed point (1/60e9 of a token, refilled by `per_minute` units per
nanosecond), so every value is an exact integer.

## Failed credentials

Before the credentials of a request are verified, the precheck reserves one token of
`rl.public_auth` of its client IP; while that bucket is empty, requests with credentials from the IP
get `429` and nothing is verified. The token stays taken when the credentials fail with `401`
(except `token_expired` and `use_dpop_nonce`) or name a revoked or expired API token (the stage
settles such an actor as `401 unauthenticated`, M1-44) and goes back otherwise: valid credentials, the two
normal flows and errors of other kinds (for example a `503` of the identity store) cost nothing.
Because the token is reserved before the verification, concurrent requests of one client IP cannot
all pass on the same token: at most `burst` verifications run per refill. The price: more than
`burst` requests with credentials of one client IP in flight at the same instant get `429` even
when their credentials are valid.

The precheck of `StageAuthenticate` is built on `Limiter.FailedCredentialGuard(r)` (M2-27), which
other credential checks call directly: it reserves one token of `rl.public_auth` of the client IP
of `r` before the check. While the bucket is empty it returns `429 rate_limited` (with
`Retry-After` and the `RateLimit-Policy` and `RateLimit` headers of `rl.public_auth`) and no settle
function, and the caller must not check the credential. Otherwise the caller calls
`settle(failed)` exactly once: `true` keeps the token (the credential failed), `false` gives it
back. The token check of the incoming webhooks (`apps/adapters/httpapi.TokenStage`) uses it, so a
wrong webhook token costs the same token of the client IP as a wrong API token; operations with
`httpserver.Operation.AnonymousCredentials` skip the precheck of `StageAuthenticate`, so one
request never pays twice.

## Memory

The buckets live in a fixed number of shards (constant `maxShards`; map plus mutex, shard by a hash of the internal key with a random seed per limiter), at most
`ratelimit.max_tracked_keys` in total. `Sweep` (every `ratelimit.sweep_interval`) removes full
buckets, which decide exactly like missing ones. An insert into a full shard removes the full
buckets of a small random sample (constant `evictSample`), else the least recently used bucket of the sample; the limiter
never denies because the table is full.

Every REST key has a bounded length: an IP address or prefix, an ID, or a `webhook_id`, which must
be an ID (any other value is `404` before a bucket exists). The gateway passes its own connection
IDs. Estimate: run `go test -run '^$' -bench BenchmarkLimiter_BytesPerKey ./internal/platform/ratelimit/`
and multiply its `bytes/key` (principal keys, map overhead included) by
`ratelimit.max_tracked_keys`.

## Several nodes

Limits live in the process (R-AR-12). With n api nodes behind a load balancer the effective limit
is up to n times the configured value (02 "Scaling path").
