# Module `gateway`

Status: connection and session built (M1-64). Delivery of events, revocation of open connections
and the shutdown close come with M1-65, focus and typing with M1-66, frame conformance with M1-67.
Normative are [docs/spec/04-sync-protocol.md](../../../../docs/spec/04-sync-protocol.md) section 6
and [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); on any difference the
chapters win. Rulings: [DECISIONS.md](../../../../docs/DECISIONS.md) M1-64.

## Purpose

WebSocket endpoint, gateway sessions, hub, focus, typing, relaying ephemeral events and agent
connections.

## Code

- `internal/gateway` (port): `Register` declares the client ops the gateway accepts (AZ-P2).
- `domain/`: the close codes of 04 section 6.4 with their contract names
  (`TestCloseCodesMatchAsyncAPI` holds them against `x-close-codes`), the outcome of a close for
  the span, the session states `awaiting_identify`, `ready`, `detached`, `closed` with their
  transitions, and the deadlines of heartbeat and reauth.
- `app/`: `Gateway` with `Open` (hello), `Handle`, `Tick` (deadlines), `Closed`; the handlers of
  `identify`, `resume`, `heartbeat` and `reauth`; the op registry `Dispatcher`
  (`Register(op, action, handler)`), which closes with `4000` and raises the alarm when a successful
  handler did not call `authorize` with its action; the session store `Sessions` with its
  sweeper; the ports `Conn`, `FrameDecoder`, `NonceSource`, `ConnectLimiter`, `EpochSource`,
  `ProofAlarm` and `Delivery`.
- `adapters/ws/`: the HTTP handler of `GET /api/v1/gateway` with `github.com/coder/websocket`
  (the only package of the server that imports it), one reader, one writer and one deadline
  goroutine per connection, the frame codec on the generated `Gateway*` types of `api/rest`, the
  span `gateway.connect`.
- `adapters/wstest/`: a Go client for tests of this and later tickets (`Dial`, `Hello`,
  `Identify`, `Resume`, `Send`, `Next`, `ExpectClose`, a heartbeat loop with a random first delay).

## Connection

1. Upgrade: a present `Origin` other than the origin of `server.public_url` answers `403
   forbidden` (SEC-CORS-3), and so does a request with the console cookie
   `__Host-console_session` and no `Origin` (M6-01); nothing else is checked. Compression is off.
2. `hello` with `heartbeat_interval_ms`, `resume_window_ms`, `protocol_versions`, the current DPoP
   nonce, `server_time` and `sync_epoch`.
3. `identify` or `resume` within `sync.identify_timeout`: token and proof are checked by the
   Authenticator of identity with method `GET` and the HTTPS form of the gateway URL as `htu`;
   then `rl.gateway_connect` per device or token, then `authorize(gateway.connect, instance)`.
4. `ready` with the snapshot of `sync` (delivery is registered before the head is read), or
   `resumed`; with a cursor, catch-up decides between delivery from the cursor and
   `resync_required` (the connection stays live from the head).
5. Heartbeats, `reauth_required` and `reauth`; the close codes below end the connection.

Frames are JSON text frames `{op, d}` of at most `sync.ws_max_frame_bytes`, checked with the JSON
guard of `platform/httpserver`, the schema of the op and the strict decoder.

## Ops

Enabled (section `gateway_ops` of `api/operations.yaml`): `identify`, `resume`, `heartbeat`,
`reauth`, each with the action `gateway.connect`. Every other client op of the channel is unknown
(`4001`) until its ticket enables it.

## Close codes owned

`4001` to `4009`, `4011`, `4016` to `4018`; used: `4000` (database or internal error, panic,
handler without authorization check), `4012` (revoked at identify or heartbeat), `4014` (full
outbound queue, missed write deadline).

## Sessions

In process memory only (R-AR-11); a resume on another process gets `4006`. One open connection
per device: a new `identify` or `resume` closes the older one with `4017`. A detached session is
resumable for `sync.resume_window`; the sweeper runs every `sync.resume_window`.

## Configuration

`server.public_url`, `sync.identify_timeout`, `sync.heartbeat_interval`,
`sync.heartbeat_miss_limit`, `sync.resume_window`, `sync.reauth_lead_time`,
`sync.ws_max_frame_bytes`, `sync.outbound_queue_max`, `gateway.write_timeout`,
`http.max_json_depth`, `ratelimit.rl.gateway_connect.per_minute`,
`ratelimit.rl.gateway_connect.burst`; the DPoP windows `auth.dpop_nonce_ttl` and
`auth.clock_leeway` apply through identity.

## Dependencies

May use the ports of these modules: `chat`, `agents`, `runs`, `sync`, `stats`.

Reason: thread to channel for focus and typing; consumer cursor and presence; run patches and delivery preparation; tail, catch-up, snapshot; live frame for `stats.subscribe`.

This ticket uses `sync` (`Snapshot`, `CatchUp`); the others serve focus, typing, agent presence and
live statistics of later tickets. Every module may use `internal/kernel`; `internal/platform`
(`httpserver` for the JSON guard, the schema validator and the problem writer) and `api/` only in
`adapters/`. Vendors of the adapters: `github.com/coder/websocket`, OpenTelemetry. `internal/wire`
binds identity's Authenticator and DPoP nonce, the Authorizer, the limiter of the HTTP chain, the
instance row and a delivery that delivers nothing until M1-65. Checked by go-arch-lint (ADR 0008).

## Tables

`gateway_presence` (03-data-model.md, section 4.16, R-DM-51; built by PLAN M7-17).

Only this module writes and reads this table via SQL (03-data-model.md, principle 12); for everything else it reads through the ports of other modules.

## Actions (05-actions.md)

`gateway.connect`, `gateway.focus`, `typing.start`.

`gateway.connect` is built by this ticket; `gateway.focus` and `typing.start` follow with M1-66.

## Events and WebSocket ops

- Durable: none.
- Ephemeral: `typing.started`.
- WebSocket ops (client to server): `identify`, `resume`, `heartbeat`, `reauth`, `focus`, `typing.start`, `typing.stop`.

## Enabled ops

This ticket enables `identify`, `resume`, `heartbeat` and `reauth`; `focus`, `typing.start`,
`typing.stop` and `typing.started` follow with M1-66.

## Not proven here

- The heartbeat and expiry tests run with windows of seconds; their margins under heavy load are
  not measured.
- A missed `gateway.write_timeout` tears the connection down without the close frame
  (DECISIONS M1-64/8); only the queue overflow is tested end to end.
