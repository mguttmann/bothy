# M1-64: Gateway connection and session

Status: ready | Depends on: M1-19, M1-21, M1-63 | Etappe: M1

## Goal

The WebSocket endpoint `/api/v1/gateway` accepts connections, checks the `Origin`, sends `hello` with a server DPoP nonce, authenticates in-band with `identify` (access token plus DPoP proof bound to that nonce, or an API token), answers `ready` with the snapshot, keeps the session alive with heartbeats, renews access tokens with `reauth`, resumes sessions within `sync.resume_window` and closes with the codes of 04. It is the connection layer on which delivery and revocation (M1-65), focus and typing (M1-66) and frame conformance (M1-67) build.

## Read first

- `docs/spec/04-sync-protokoll.md`: 'Rahmen auf dem WebSocket'; 'WebSocket-Gateway' with 'Endpunkt und Version', 'Ops', 'Lebenszyklus', 'Schließ-Codes', 'Neu verbinden'; 'resync_required'; 'Sync-Epoche'; 'Widerruf von Sitzung, Gerät oder Prinzipal'; sequence diagram 'Wiederverbinden mit Nachholen'; decisions R-SY-01, R-SY-14; configuration table.
- `docs/spec/05-aktionen.md`: `gateway.connect` ('Persönliche Daten, Benachrichtigung, Sync'), 'WebSocket-Ops und Aktion', 'Rate-Limit-Klassen' (`rl.gateway_connect`), 'Entscheidung' (pending devices, `4016`), 'Scopes für Apps'.
- `docs/spec/02-architektur.md`: 'Gateway', 'Schichten im Modul' rule 4 (only `gateway/adapters` imports `github.com/coder/websocket`), 'Erlaubte Abhängigkeiten' (`gateway`), 'Querschnitts-Ports' (`actor.Authenticator`, `Authorizer`), 'Kette im api-Knoten', 'Nachweis der Prüfung', 'Fristen' (`gateway.write_timeout`), 'Panics', R-AR-11.
- `docs/spec/06-api.md`: 'Echtzeit-Gateway', 'WebSocket-Ops und Ereignisse', 'DPoP', 'Rate-Limits' point 3, 'Fehler' point 6.
- `docs/spec/08-identitaet.md`: 'Zugriffstoken', 'DPoP', 'API-Tokens für Agenten, Apps und Werkzeuge', 'Bestätigung' (pending devices); configuration `auth.dpop_nonce_ttl`, `auth.clock_leeway`, `auth.dpop_proof_max_age`.
- `docs/spec/07-rechte.md`: 'Prinzipal-Kontext', 'Entscheidungen und Gründe'.
- `docs/spec/19-sicherheit.md`: 'CORS und WebSocket-Herkunft' (SEC-CORS-3), input rules SEC-IN-1 to SEC-IN-3, tests SEC-T04 and SEC-T10.
- `docs/spec/16-betrieb.md`: `server.public_url`.
- `docs/api/asyncapi.yaml`: channel `gateway`, `x-close-codes`, messages `op.*`. `docs/api/openapi.yaml`: schemas `GatewayHello`, `GatewayIdentify`, `GatewayClientInfo`, `GatewayReady`, `GatewayResume`, `GatewayResumed`, `GatewayHeartbeat`, `GatewayHeartbeatAck`, `GatewayReauth`, `GatewayReauthRequired`, `GatewayResyncRequired`, `GatewayError`, `Snapshot`.
- `docs/observability/signals.md` (gateway rows, span `gateway.connect`); `docs/security/threat-model.md` 'Gateway (WebSocket)'.
- ADRs 0013, 0017, 0020 and, if present, 0052, 0053, 0061.
- `docs/PLAN.md`: 'Arbeitsweise je Ticket', M1 rows M1-33, M1-34, M1-44, M1-63, M1-65 to M1-67, M2-07, M2-20.
- Code: `internal/kernel/actor`, `internal/kernel/authorize`, `internal/identity` (DPoP verifier and nonce source of M1-33, authenticator of M1-34, API tokens of M1-44, test helpers), `internal/platform/ratelimit` (M1-19), `internal/platform/httpserver` (M1-18, M1-21), `internal/sync` ports `Snapshot` and `CatchUp` (M1-63), `api/rest` (generated `Gateway*` types), `api/routes`, `architecture/modules.yaml`.

## Fixed requirements that apply

Owner's fixed build rules (apply verbatim):

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or doc comment (purpose, parameters, return value, errors, example where useful); comments explain why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc, Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision, a CHANGELOG entry per release, docs site kept current. A change without matching documentation is not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the merge.

Rules for every brief:

6. Test first: write the failing test, run it and keep the red output for the report, then make it green, then clean up.
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossar.md`, `03-datenmodell.md`, `04-sync-protokoll.md`, `05-aktionen.md`) is not edited by this ticket.

Product requirements implemented here:

11. One endpoint `wss://<host>/api/v1/gateway`; JSON text frames `{op, d}` of at most `sync.ws_max_frame_bytes` (04 'Endpunkt und Version', 'Rahmen auf dem WebSocket').
12. The upgrade checks only the origin; a foreign `Origin` gives `403` before the upgrade (19 SEC-CORS-3, 04 'Lebenszyklus' step 1).
13. `hello` immediately with `heartbeat_interval_ms`, `resume_window_ms`, `protocol_versions`, `dpop_nonce`, `server_time`, `sync_epoch` (04, schema `GatewayHello`).
14. Authentication only in `identify`, `resume` and `reauth`, with the DPoP proof bound to the nonce from `hello` (R-SY-01, 08 'DPoP'); then `authorize(principal, gateway.connect, instance)`; a pending device gets `4016`.
15. `ready` with session id, principal, device, rights version, head and snapshot; `resume` within `sync.resume_window`, otherwise `4006`; heartbeats with `4009` after `sync.heartbeat_miss_limit` missed; `reauth_required` `sync.reauth_lead_time` before token expiry and `4011` without a valid `reauth` (04 'Lebenszyklus').
16. All close codes of 04 'Schließ-Codes'; this ticket owns `4001` to `4009`, `4011` and `4016`, and uses `4000`, `4012` and `4014` where named below.
17. Rate-limit class `rl.gateway_connect`; when exhausted the gateway closes with `4008` (06 'Rate-Limits' point 3).
18. Gateway sessions live only in process memory; a resume on another node gives `4006` (R-AR-11).
19. Every op goes through `authorize` with its action from 05 'WebSocket-Ops und Aktion' (B7).

## Scope

All paths under `services/server/` unless noted.

Create:

- `internal/gateway/domain/closecodes.go`: one constant per code of 04 'Schließ-Codes' with the name from `x-close-codes` and a doc comment with the client reaction; `internal/gateway/domain/session.go`: session states `awaiting_identify`, `ready`, `detached` (disconnected, resumable) and `closed` with the allowed transitions.
- `internal/gateway/app/`: `sessions.go` (in-memory store per process with resume window and sweeper), `dispatcher.go` (op registry `Register(op string, action authorize.Action, handler OpHandler)`, unknown op handling, proof that each handler called `authorize` with its declared action), `identify.go`, `resume.go`, `heartbeat.go`, `reauth.go`, `delivery.go` (interface `Delivery { Attach(ctx, *Session, fromSeq int64) error; Detach(*Session) }`), and the outgoing ports the use cases need (authenticator, authorizer, nonce source, rate limiter, `sync.Snapshot`, `sync.CatchUp`, instance epoch, clock).
- `internal/gateway/adapters/ws/`: the HTTP handler for `GET /api/v1/gateway` with `github.com/coder/websocket`, the reader and writer goroutines, the frame codec on the generated `Gateway*` types of `api/rest` with the strict JSON decoder of M1-18.
- `internal/gateway/adapters/wstest/`: a Go test client for this and later tickets: `Dial(ctx, baseURL string, opts ...Option) (*Client, error)`, `Hello() GatewayHello`, `Identify(ctx, IdentifyParams) (GatewayReady, error)`, `Resume(...)`, `Send(op string, d any) error`, `Next(ctx) (op string, d json.RawMessage, err error)`, `ExpectClose(ctx) (code int, reason string, err error)`, and an automatic heartbeat loop whose first heartbeat waits a uniformly random share of `heartbeat_interval_ms` (04 'Lebenszyklus' step 5).
- `internal/gateway/README.md` (status, ops, close codes owned, sessions, configuration, dependencies).

Change:

- `architecture/modules.yaml`: allow `github.com/coder/websocket` for `gateway/adapters` only; regenerate `.go-arch-lint.yml`; `go.mod` and `go.sum` with the module pinned to a full release version (MIT-compatible ISC license, allowed in `quality/licenses/allowed.txt`).
- `internal/wire`: the gateway route on the public listener for roles `api` and `all`, the registration of the ops `identify`, `resume`, `heartbeat`, `reauth`, the sweeper in the lifecycle, and a `Delivery` binding that does nothing until M1-65 replaces it (doc comment names M1-65).
- Route registry (`internal/platform/httpserver` and the registration test of M1-25): register `GET /api/v1/gateway` with action `gateway.connect` and class `rl.gateway_connect`, marked as authorized in-band, so AZ-P1 counts it and the proof check of 02 'Nachweis der Prüfung' does not require a route-level `authorize` for the `101` response. Add that marker if the registry has none, with a test that only this route uses it.
- `internal/kernel/actor` and `internal/identity`, only if needed: if `actor.Authenticator` accepts only an `*http.Request`, add a transport-neutral method `AuthenticateCredentials(ctx, actor.Credentials{Scheme, Token, DPoPProof, Method, URL string; ExpectNonce bool}) (actor.Actor, error)` and implement the HTTP path on top of it, with tests in identity. Expose the current server DPoP nonce through the identity port (M1-33 source) if it is not exposed.
- `docs/api/openapi.yaml` (W7, SEC-IN-1; 06 "Grundregeln" row Unbekanntes): `additionalProperties: false` on every schema of a client-to-server frame (`GatewayIdentify` with `GatewayClientInfo`, `GatewayResume`, `GatewayHeartbeat`, `GatewayReauth`, `GatewayFocus`, `GatewayTyping`, `GatewayAck`, `GatewayRunUpdate`, `GatewayAutocompleteResult`), closed the same way M1-18 closes request schemas; extend the M1-18 generator check "request schemas closed" to these schemas; `make api-lint` green.
- `services/server/CHANGELOG.md`; `docs/DECISIONS.md`.

## Out of scope

- Hub, delivery of events, streaming catch-up with `cursor` frames and `catchup.complete`, revocation of open connections on `session.revoked` and `access.changed`, end-to-end slow consumer test, shutdown with `4013` (M1-65).
- `focus`, `typing.start`, `typing.stop` and `typing.started` (M1-66); frame conformance against the schemas and the op registry test (M1-67).
- Persistent agent consumer cursors, fencing and `4015` (M2-07); `ack`, `run.update`, `command.autocomplete_result` (M2); agent presence from `heartbeat.agent` (M2-20).
- Enforcing one connection per device (see note 15).
- Gateway metrics (M1-68); the canary test (M1-69); connection load and memory targets (M1-72).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-64-gateway-connection`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(gateway): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11). Do not invent close codes.

## Design notes

Upgrade and frames

1. `GET /api/v1/gateway` exists only on the public listener. It passes the chain steps panic, correlation, span and header limits (02 'Kette im api-Knoten' steps 1 to 3). If an `Origin` header is present and its scheme, host and port differ from the origin of `server.public_url`, answer `403` with problem code `forbidden` before the upgrade (SEC-CORS-3). Nothing else is checked at the upgrade. Accept with compression disabled (memory per idle connection, `perf.ws_idle_conn_mem_max`) and with `OriginPatterns` set to the host of `server.public_url`; never use the option that turns origin verification off.
2. Send `hello` right after the upgrade: `heartbeat_interval_ms` from `sync.heartbeat_interval`, `resume_window_ms` from `sync.resume_window`, `protocol_versions: [1]`, `dpop_nonce` = the current server nonce from identity (valid `auth.dpop_nonce_ttl`; the current and the previous nonce are accepted, 08 'DPoP'), `server_time`, `sync_epoch`.
3. Frames: text only. Read each message through a streaming reader limited to `sync.ws_max_frame_bytes` plus one byte; a larger frame closes with `4002` and reason `frame_too_large`. Binary frames, invalid JSON, duplicate keys, invalid UTF-8, a missing `op`, a `d` that does not match its schema and unknown members in `d` (Ruling W7) close with `4002`.
4. Before a successful `identify` or `resume`, any other known op closes with `4003`; an unknown op closes with `4001` at any time. Without `identify` or `resume` within `sync.identify_timeout` the server closes with `4003` (ruling: not identified in time).
5. Ops known to `asyncapi.yaml` but not registered yet (`focus`, `typing.start`, `typing.stop` until M1-66; `ack`, `run.update`, `command.autocomplete_result` until M2) are treated as unknown (`4001`). The dispatcher takes each op's action from the generated mapping in `api/routes`; after a handler returns, it checks that `authorize` was called with that action, otherwise it closes with `4000` and raises the alarm counter used by the HTTP proof check.

identify

6. Exactly one of `access_token` and `api_token`, otherwise `4002`. `protocol_version` not in `[1]` gives `4007`.
7. Authenticate with `actor.Authenticator`: token, DPoP proof, method `GET`, HTU `https://<host and port of server.public_url>/api/v1/gateway` (the HTTPS form even behind TLS termination, 04 'Lebenszyklus' step 3), nonce required. People must send `device_id` equal to the token's device claim. API tokens need a proof when the token has `dpop_jkt`.
8. Rate limit `rl.gateway_connect` through `platform/ratelimit`, keyed by the verified device id (people) or token id (agents and apps), checked after the credentials are verified so that nobody can drain another device's bucket. Exhausted: close `4008` with reason `retry_after=<seconds>`.
9. Then `authorize(actor, gateway.connect, instance)`.
10. Outcome to close code (ruling):

| Outcome | Close |
|---|---|
| malformed or unknown token, bad signature, wrong `iss` or `aud`, invalid DPoP proof (`htm`, `htu`, `ath`, `iat`, replayed `jti`, wrong or missing nonce), `device_id` mismatch | `4004` |
| access token expired beyond `auth.clock_leeway` | `4011` |
| `authorize` unauthenticated with `principal_inactive`, `session_revoked`, `device_inactive`, `token_revoked` or `app_not_installed` | `4012` |
| `authorize` forbidden with `device_pending` | `4016` |
| any other `forbidden` (for example `scope_missing` of an app) | `4004` |
| rate limit exhausted | `4008` |
| database unavailable while building `ready` | `4000` |

11. Success: create the session (UUIDv7 `gateway_session_id`, principal id and kind, device id, auth session id, token id, token expiry, `consumer` for agents and apps with default `default`, client info), build the snapshot with `sync.Snapshot.Build`, send `ready {gateway_session_id, principal (from the snapshot), device_id, permission_version, head_seq, snapshot, sync_epoch}`.
12. After `ready` or `resumed`: if the client sent a `cursor`, call `sync.CatchUp.Check(cursor, sync_epoch)`; on a reason send `resync_required {reason, head_seq, sync_epoch}` and do not attach delivery (the client reloads the snapshot and reconnects, 04 'resync_required'). Otherwise call `Delivery.Attach(session, cursor)`, or `Delivery.Attach(session, head_seq)` without a cursor.
13. A second `identify` or a `resume` on an identified connection closes with `4005`.

heartbeat, reauth, resume

14. `heartbeat {last_seq}` (at least 0): answer `heartbeat_ack {server_time}` and check `authorize(actor, gateway.connect, instance)` from the cache; a revoked session, device, token or principal closes with `4012`. The server closes with `4009` when no heartbeat arrived for `sync.heartbeat_interval` times (`sync.heartbeat_miss_limit` + 1) since `ready`, `resumed` or the last heartbeat; the extra interval absorbs the client's random delay of the first heartbeat. The member `agent` is accepted and ignored until M2-20.
15. One connection per device (04 'Endpunkt und Version') is not enforced here: 04 has no close code for a replaced device connection (`4015` is for agent consumers only). Do not invent one; the gap is reported to the owner of the core contract.
16. People: send `reauth_required {expires_at, dpop_nonce}` `sync.reauth_lead_time` before the access token expires. `reauth {access_token, dpop}` is authenticated like `identify`; the new token must belong to the same principal, device and auth session, otherwise close `4004`. An invalid token or proof answers `error {op: reauth, code}` with the problem code and the connection stays. When the current token expires without a successful `reauth`, close `4011`. Agents and apps get no `reauth_required`; at the API token's `expires_at` the server closes with `4011`.
17. `resume {gateway_session_id, access_token or api_token, dpop, cursor, sync_epoch}` on a new connection after `hello`: an unknown session, a session detached longer than `sync.resume_window` and a session of another principal or device all close with `4006`, identically. Authenticate, rate limit and authorize as in notes 7 to 10. If the session's old connection is still open, close it with `4006` first. Send `resumed {gateway_session_id, head_seq, sync_epoch}` and continue as in note 12.
18. Sessions: a per-process map; a detached session keeps its id and data for `sync.resume_window`; a sweeper started and stopped by the lifecycle removes expired ones.

Connection mechanics

19. One reader goroutine handles ops in order. One writer goroutine drains a queue bounded by `sync.outbound_queue_max` frames; each write has the deadline `gateway.write_timeout`. Queue overflow or a write timeout closes with `4014` (the end-to-end slow consumer test is M1-65). A panic in a connection closes it with `4000` (02 'Panics').
20. Non-fatal op errors answer `error {op, code, detail, ref}` with problem codes of 06; fatal ones close. Close reasons are at most 123 bytes (RFC 6455) and contain only machine data, never tokens, proofs, nonces or content.
21. Telemetry: span `gateway.connect` with `bothy.outcome`, `bothy.close.code` and `bothy.principal.kind` (`docs/observability/signals.md`); instruments come with M1-68. Logs never contain tokens, proofs, nonces or the snapshot.
22. Agents and apps identify with API tokens in M1 and get `ready` like people; `consumer` must be non-empty and is kept on the session. Persistent cursors and `4015` come with M2-07.
23. Configuration keys used: `server.public_url`, `sync.identify_timeout`, `sync.heartbeat_interval`, `sync.heartbeat_miss_limit`, `sync.resume_window`, `sync.reauth_lead_time`, `sync.ws_max_frame_bytes`, `sync.outbound_queue_max`, `gateway.write_timeout`, `auth.dpop_nonce_ttl`, `auth.clock_leeway`, `ratelimit.rl.gateway_connect.per_minute`, `ratelimit.rl.gateway_connect.burst`. Tests may override durations with short values through the configuration loader.
24. Rulings for `docs/DECISIONS.md`: identify timeout closes `4003`; oversize frames close `4002`; the outcome table of note 10; rate limit at `identify` and `resume` instead of the upgrade (note 8); heartbeat deadline of note 14; reauth rules of note 16; resume rules of note 17; one connection per device not enforced until 04 names a code (note 15).

## Tests (write them first)

Unit:

- `internal/gateway/domain`: `TestCloseCodesMatchAsyncAPI` reads `docs/api/asyncapi.yaml` (`x-close-codes`) and compares every code and name with the constants.
- `internal/gateway/app` with fakes and a fake clock: `TestIdentifyOutcomeMapping` (one case per row of note 10), `TestHeartbeatDeadline`, `TestReauthSchedule`, `TestResumeWindow`, `TestDispatcherRequiresAuthorize`, `TestUnregisteredKnownOpIsUnknown`.

Integration (real server on `127.0.0.1:0`, PostgreSQL 18 via `pgtest`, people, devices, tokens and DPoP proofs from the identity test helpers, the `wstest` client):

- `TestSECCORS3ForeignOriginRejected`: foreign `Origin` gives `403` before the upgrade; the instance origin and a missing `Origin` are accepted.
- `TestHelloFields`: values from the configuration, a nonce accepted by identify.
- `TestIdentifyPersonReady`: `ready` decodes into `GatewayReady` with unknown members rejected; `snapshot.head_seq` equals `ready.head_seq`.
- `TestIdentifyAgentToken`: a DPoP-bound agent token with proof gets `ready`; without proof `4004`.
- `TestClose4001UnknownOp`, `TestClose4002DecodeError` (bad JSON, duplicate key, unknown member, binary frame, frame above `sync.ws_max_frame_bytes`), `TestClose4003OpBeforeIdentify`, `TestClose4003IdentifyTimeout`, `TestClose4004BadProof` (wrong `htu`, wrong nonce, replayed `jti`, missing proof), `TestClose4005SecondIdentify`, `TestClose4006ResumeUnknown`, `TestClose4006ResumeAfterWindow`, `TestClose4007ProtocolVersion`, `TestClose4008RateLimited`, `TestClose4009HeartbeatTimeout`, `TestClose4011ExpiredWithoutReauth`, `TestClose4012RevokedAtIdentify`, `TestClose4016PendingDevice`.
- `TestResumeWithinWindow`: `resumed` with the same session id.
- `TestReauthKeepsConnection`: a refreshed token of the same session keeps the connection and moves the expiry; a token of another session closes `4004`; an invalid proof answers `error` and keeps the connection.
- `TestFirstHeartbeatJitterTolerated`: a first heartbeat shortly before twice `heartbeat_interval_ms` keeps the connection open; `TestTestClientJittersFirstHeartbeat`: the `wstest` client's first delay lies in the interval from zero to `heartbeat_interval_ms`.
- `TestResyncRequiredAfterReady`: an `identify` with a wrong `sync_epoch` gets `ready` followed by `resync_required` with `epoch_changed`.
- `TestOutboundQueueOverflowCloses4014`: with a writer that blocks, filling the queue closes with `4014`.
- `TestNoSecretsInCloseReasonsAndLogs`: tokens, proofs and nonces of the suite appear in no close reason and no captured log line.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/gateway/... ./internal/identity/... ./internal/kernel/... ./internal/platform/httpserver/... ./internal/wire/...`
- `make codegen-check`, `make arch`, `make register-check`, `make licenses`, `make osv`
- `make check`

## Acceptance criteria

- [ ] A foreign `Origin` gets `403` before the upgrade (SEC-CORS-3, part of SEC-T04).
- [ ] `hello` arrives first and carries a nonce that `identify` accepts; a proof with another nonce is rejected with `4004`.
- [ ] People and agents reach `ready` with a valid snapshot; a pending device gets `4016`.
- [ ] Every close code owned by this ticket (`4001` to `4009`, `4011`, `4016`) has a passing test; `4012` at identify and `4014` on overflow are tested.
- [ ] `resume` within `sync.resume_window` gives `resumed`, later `4006`.
- [ ] `reauth_required` comes `sync.reauth_lead_time` before expiry; without `reauth` the connection closes with `4011`.
- [ ] Frames above `sync.ws_max_frame_bytes` are rejected; unknown members are rejected.
- [ ] Only `gateway/adapters` imports `github.com/coder/websocket` (`make arch`).
- [ ] The gateway route is registered with `gateway.connect`; AZ-P1 passes.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- `internal/gateway/README.md` written; READMEs of `identity` and `platform/httpserver` updated if they changed.
- Rulings of note 24 recorded in `docs/DECISIONS.md` (one row each with reason). If the implementation adds a threat that `docs/security/threat-model.md` section 'Gateway (WebSocket)' does not list (for example connections held unauthenticated until `sync.identify_timeout`), add the row with its countermeasure.
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-vorlage-madr-4.md`, headings English, text German, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Merge locally into the base branch only if review, red team and `make check` are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (for example timing margins of the heartbeat tests).
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
