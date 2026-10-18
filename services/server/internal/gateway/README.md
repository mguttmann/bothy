# Module `gateway`

Status: connection and session built (M1-64); delivery of durable events, catch-up over the
WebSocket, revocation of open connections and the shutdown close built (M1-65); focus, typing and
the ephemeral publisher built (M1-66). Frame conformance comes with M1-67.
Normative are [docs/spec/04-sync-protokoll.md](../../../../docs/spec/04-sync-protokoll.md) section 6
and [docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); on any difference the
chapters win. Rulings: [DECISIONS.md](../../../../docs/DECISIONS.md) M1-64, M1-65 and M1-66.

## Purpose

WebSocket endpoint, gateway sessions, hub, focus, typing, relaying ephemeral events and agent
connections.

## Code

- `internal/gateway` (port): `Register` declares the client ops the gateway accepts (AZ-P2).
- `domain/`: the close codes of 04 section 6.4 with their contract names
  (`TestCloseCodesMatchAsyncAPI` holds them against `x-close-codes`), the outcome of a close for
  the span, the session states `awaiting_identify`, `ready`, `detached`, `closed` with their
  transitions, and the deadlines of heartbeat and reauth; the connection phases `catching_up`,
  `live` and `closing`; the bounded outbound queue with class (`dispatch` or `control`), seq, scope
  ids and `Purge` of removed targets (control frames are never purged); `FocusSet` (dedupe and
  the limit of a focus), the stored `Focus`, `TypingThrottle` and the register of the ephemeral
  types the gateway relays (`LookupEphemeral`: `typing.started` and `run.updated`, both bound to
  the focus, 05 "Ephemeral events").
- `app/`: `Gateway` with `Open` (hello), `Handle`, `Tick` (deadlines), `Closed`; the handlers of
  `identify`, `resume`, `heartbeat` and `reauth`; the op registry `Dispatcher`
  (`Register(op, action, handler)`), which closes with `4000` and raises the alarm when a successful
  handler did not call `authorize` with its action; the session store `Sessions` with its
  sweeper; the ports `Conn`, `FrameDecoder`, `NonceSource`, `ConnectLimiter`, `EpochSource`,
  `ProofAlarm`, `Delivery`, `OpLimiter` and `Threads`; `Hub` (the implementation of `Delivery`:
  indices, `Dispatch`, `CloseAll`), the catch-up runner (`catchup.go`), revocation
  (`revocation.go`) and shutdown (`shutdown.go`); the handlers of `focus` (`focus.go`),
  `typing.start` and `typing.stop` (`typing.go`), the thread index with focus pruning
  (`hubfocus.go`) and `Hub.PublishEphemeral`, the process's `kernel/eventlog.EphemeralPublisher`
  (`ephemeral.go`).
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

## Delivery (M1-65)

The log tail of every process with role `api` or `all` calls, per durable event in `seq` order and
in one goroutine, first `Authorizer.Observe`, then `Hub.Dispatch` (wire `DeliverySink`). A panic
there ends the process; the liveness check `event_tail` fails once that goroutine has ended.

- Candidates: the audience's connections for a personal event; for an event of a space or channel
  the space and channel indices, the instance-wide set and every connection whose visible sets are
  not known yet; every connection for an instance-level event. The set is a superset; the truth is
  `sync.Admit` in `authorize.Delivery` mode for each candidate (no denial audit).
  A check that fails with another error than `unauthenticated` closes that connection with `4000`
  (resumable), so no later frame moves the client's cursor past the event.
- Each event is encoded once per form and queued; `Dispatch` never waits on a connection. A full
  queue (`sync.outbound_queue_max`) or a write that misses `gateway.write_timeout` closes that
  connection with `4014`.
- Every `sync.cursor_advance_every` processed events a live connection whose last frame with a seq
  is older gets `cursor {seq}`.
- Catch-up (identify or resume with a cursor): the handshake registers the connection in phase
  `catching_up` before it reads `head_seq` H; pages of `sync.catchup_page_size` after the cursor
  up to H, read in filter mode at the current rights version (they can lie ahead of the log tail), `cursor` at the end of each page and after `sync.cursor_advance_every` skipped events,
  then `catchup.complete {head_seq: H}`, then the buffered live events with `seq > H`, admitted
  again with the newest rights; the flush waits for queue space like the pages, only a live
  buffer above `sync.outbound_queue_max` closes with `4014`. The runner leaves
  `domain.ControlReserve` entries of the queue free for control frames of the client's ops. A
  target that `access.changed` adds during the catch-up is no longer filtered. A failing page is retried with backoff up to
  `database.listen_reconnect_backoff_cap`. Without a cursor the connection is live from H at once.

## Revocation and shutdown

- `access.changed` of principal P, before it is dispatched: every connection of P is revalidated
  (identity `Revalidate` and `gateway.connect` in delivery mode), failures close with `4012`; P's
  index entries are recomputed; queued and buffered dispatches of the removed targets are dropped;
  then the event takes the normal path.
- `session.revoked` closes the connections of that session with `4012`; the other connections of
  the person receive the event.
- A rights change without a difference of visible targets writes no `access.changed` (for example
  `token.revoke`); the next event of the log carries the new rights version, the delivery check
  then rebuilds the snapshot, and a revoked credential answers `unauthenticated`, which closes the
  connection with `4012` before anything is delivered to it (DECISIONS M1-65). An own personal
  event is checked against the credential too (`sync.Admit`).
- Shutdown (stop phase `connections`, after the HTTP server and before the log tail): every
  connection closes with `4013 server_restart`, new `identify` and `resume` get `4013`, the
  writers flush within the rest of `shutdown.total_timeout`.

## Focus, typing and ephemeral events (M1-66)

- `focus {thread_ids, idle}`: `rl.read` keyed by the principal (`error rate_limited`), repeated
  ids count once, more than `sync.max_focused_threads` answers `error validation_failed` with
  the detail `sync.max_focused_threads` and keeps the previous set; then `gateway.focus` per
  thread in the request context (denials audited by authz). Unknown and unreadable threads both
  get `not_found` from authorize and are dropped without an error frame. The readable threads
  replace the focus of the connection in the thread index and of its gateway session, with
  `idle`; an empty set clears it. Accepted while the connection catches up.
- `typing.start {thread_id}` and `typing.stop {thread_id}`: `rl.typing` keyed by the connection,
  then `typing.start` on the thread (people with `write` only; read-only channels deny everyone);
  a denial answers `error` with the code of the decision (`not_found`, `forbidden`,
  `channel_read_only`). `typing.start` fans out at most once per `sync.typing_min_interval` per
  connection and thread and publishes `typing.started {thread_id, principal_id, expires_at}`
  with `expires_at = occurred_at + sync.typing_ttl` (UTC, milliseconds). `typing.stop` sends
  nothing; it clears the throttle so the next start fans out at once.
- `PublishEphemeral` checks the type against the register, the schema version and the payload
  (a JSON object without null, smaller than `sync.event_payload_max_bytes`), encodes the frame
  once without `seq` and `permission_version`, and offers it to the candidates: the connections
  that focus the thread of a focus-bound type, the audience's connections of a personal one,
  otherwise the scope rule of delivery; `typing.started` skips every connection of its actor.
  Each candidate must be live and pass `sync.Admit` with the type's receive action in delivery
  mode. Ephemeral frames are never buffered or sent before `catchup.complete`, a full queue drops
  them (never `4014`), and `access.changed` purges them like dispatches of the removed targets.
  It never waits on a connection. With `cluster.mode = multi` M7-17 adds the forwarding as a
  decorator of the publisher.
- Focus pruning: `access.changed` re-checks every focused thread of the principal's connections
  with `gateway.focus` in delivery mode; `thread.moved`, `channel.archived` and `space.archived`
  re-check the connections that focus an affected thread (the moved thread takes its new
  channel). `resume` restores the focus of the session after the same re-check.

## Ops

Enabled (section `gateway_ops` of `api/operations.yaml`): `identify`, `resume`, `heartbeat`,
`reauth` with the action `gateway.connect`; `focus` with `gateway.focus`; `typing.start` and
`typing.stop` with `typing.start`. Every other client op of the channel is unknown (`4001`) until
its ticket enables it.

## Close codes owned

`4001` to `4009`, `4011`, `4016` to `4018`; used: `4000` (database or internal error, panic,
handler without authorization check), `4012` (revoked at identify, heartbeat or delivery),
`4013` (shutdown), `4014` (full outbound queue or live buffer, missed write deadline).

## Sessions

In process memory only (R-AR-11); a resume on another process gets `4006`. One open connection
per device: a new `identify` or `resume` closes the older one with `4017`. A detached session is
resumable for `sync.resume_window`; the sweeper runs every `sync.resume_window`.

## Configuration

`server.public_url`, `sync.identify_timeout`, `sync.heartbeat_interval`,
`sync.heartbeat_miss_limit`, `sync.resume_window`, `sync.reauth_lead_time`,
`sync.ws_max_frame_bytes`, `sync.outbound_queue_max`, `gateway.write_timeout`,
`sync.catchup_page_size`, `sync.cursor_advance_every`, `sync.log_poll_interval`,
`database.listen_reconnect_backoff_cap`, `shutdown.total_timeout`, `http.max_json_depth`, `ratelimit.rl.gateway_connect.per_minute`,
`ratelimit.rl.gateway_connect.burst`, `sync.max_focused_threads`, `sync.typing_min_interval`,
`sync.typing_ttl`, `sync.event_payload_max_bytes`, `ratelimit.rl.read.per_minute`,
`ratelimit.rl.read.burst`, `ratelimit.rl.typing.per_minute`, `ratelimit.rl.typing.burst`; the
DPoP windows `auth.dpop_nonce_ttl` and `auth.clock_leeway` apply through identity.

## Abhängigkeiten

The section titles and lead-ins below are read by `make register-check` and stay in the form the
check expects.

Darf die Ports dieser Module nutzen: `chat`, `agents`, `runs`, `sync`, `stats`.

M1-64 and M1-65 use `sync` (`Snapshot`, `CatchUp`), M1-66 `chat` (`Threads.Info` for the place
of a focused or typed-in thread); the others serve agent presence and live statistics of later
tickets. Every module may use `internal/kernel`; `internal/platform`
(`httpserver` for the JSON guard, the schema validator and the problem writer) and `api/` only in
`adapters/`. Vendors of the adapters: `github.com/coder/websocket`, OpenTelemetry. `internal/wire`
binds identity's Authenticator and DPoP nonce, the Authorizer, the limiter of the HTTP chain, the
instance row, the threads of chat and the hub, which is also the process's `EphemeralPublisher`.

## Tabellen

Keine. The module reads through the ports of other modules (03-datenmodell.md, principle 12).

## Aktionen (05-aktionen.md)

`gateway.connect`, `gateway.focus`, `typing.start`.

`gateway.connect` is built by M1-64, `gateway.focus` and `typing.start` by M1-66.

## Ereignisse und WebSocket-Ops

- Dauerhaft: keine.
- Kurzlebig: `typing.started`.
- WebSocket-Ops (Client an Server): `identify`, `resume`, `heartbeat`, `reauth`, `focus`, `typing.start`, `typing.stop`.

## Enabled ops

M1-64 enables `identify`, `resume`, `heartbeat` and `reauth`; M1-66 enables `focus`,
`typing.start` and `typing.stop` and emits `typing.started`.

## Not proven here

- The heartbeat and expiry tests run with windows of seconds; their margins under heavy load are
  not measured.
- A missed `gateway.write_timeout` tears the connection down without the close frame
  (DECISIONS M1-64/8); only the queue overflow is tested end to end.
