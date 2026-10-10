# M1-65: Gateway hub, delivery, catch-up and revocation

Status: ready | Depends on: M1-45, M1-64 | Etappe: M1

## Goal

After this ticket every process with role `api` delivers each durable event of the event log to
exactly those gateway connections whose principal may receive it at delivery time, in `seq` order,
without ever blocking on a slow reader. A connection that identifies or resumes with a cursor
catches up page by page over the WebSocket and then switches to live delivery without gap,
duplicate or interleaving. Revoking a grant, session, device, principal or token takes effect on
open connections immediately, also across processes, and shutdown closes every connection with
`4013`. This is the delivery half of B8 and the guarantee "rights apply at every delivery" (04
"Ziele", items 1, 3 and 4).

## Read first

- `docs/PLAN.md`: section "Arbeitsweise je Ticket"; section "M1 Fundament" with the rows M1-63,
  M1-64, M1-65, M1-66, M1-67, M1-68, M1-70, M1-72 and the list "Bekannte Widersprüche, die M1
  auflöst".
- `docs/spec/04-sync-protokoll.md`: "Zwei Arten", "Umschlag (Envelope)", "Rahmen auf dem
  WebSocket", "Lebenszyklus", "Schließ-Codes", "Nachholen (Catch-up)" with "Cursor", "Ablauf",
  "`resync_required`" and "REST-Variante", "Sichtbarkeit bei Rechte-Änderungen" (all subsections),
  "Zustellgarantie", "Reihenfolge", the sequence diagrams "Nachricht senden, online",
  "Wiederverbinden mit Nachholen" and "Entzug während offener Verbindung", "Entscheidungen dieses
  Kapitels" (R-SY-02, R-SY-03, R-SY-04, R-SY-14, R-SY-15, R-SY-16), "Konfigurationswerte dieses
  Kapitels".
- `docs/spec/02-architektur.md`: "Querschnitts-Ports (`internal/kernel`)", "Erlaubte
  Abhängigkeiten", "Ereignistypen und WebSocket-Ops je Modul", "Schichten im Modul", "Gateway",
  "Ereignis-Pfad" ("Log-Tail je api-Prozess", "Rechte-Cache", "Hub und Zustellung"), "Ausfall
  einer Abhängigkeit", "Panics", "Health", "Shutdown", "Konfigurationswerte dieses Kapitels".
- `docs/spec/05-aktionen.md`: "Verbindliche Regeln" (rules 1, 2, 4, 7), "Entscheidung",
  "Dauerhafte Ereignistypen und Empfangs-Aktion", "WebSocket-Ops und Aktion". Action ids:
  `gateway.connect`, `events.catch_up`, `grant.revoke`, `session.revoke`, `device.revoke`,
  `user.disable`, `token.revoke`, `agent.disable`.
- `docs/spec/07-rechte.md`: "Ablauf" (pseudo-code of `authorize`), "Rechte-Stand (Snapshot)",
  "Gültigkeit des Caches", "Zustellfilter je Empfänger und sofortige Wirkung" (both
  subsections), "Pflicht-Tests" (AZ-P9), test matrix rows AZ-M40 and AZ-M41.
- `docs/spec/08-identitaet.md`: "Refresh-Tokens: Rotation, Familien, Wiederverwendung", "Verlorenes
  Gerät", "Person sperren".
- `docs/spec/03-datenmodell.md`: table `event_log`; "Transaktionsregeln" ("Benachrichtigung der
  Gateway-Prozesse", "Sync-Epoche und Wiederherstellung").
- `docs/api/asyncapi.yaml`: operations `send.dispatch`, `send.cursor`, `send.catchup.complete`,
  `send.resync_required`; `x-close-codes`. `docs/api/openapi.yaml` schemas `EventEnvelope`,
  `GatewayCursor`, `GatewayCatchupComplete`, `GatewayResyncRequired`, `AccessChangedPayload`,
  `AccessTarget`, `SessionRevokedPayload`.
- ADRs: `docs/adr/0008-*`, `0009-*`, `0017-*`, and 0052 (one event envelope) and 0053 (no JSON
  null), written by M1-16.
- Diagrams: `docs/architecture/fluss-ereignis-pfad.svg`, `seq-3-wiederverbinden.svg`,
  `seq-4-freigabe-entziehen.svg`.
- `docs/security/threat-model.md`: "Gateway (WebSocket)", register rows T-AZ-03 and T-DS-01.
- Code you build on: `internal/gateway` (M1-64: endpoint, sessions, handshake, test client),
  `internal/sync` (M1-16: appender, `Tail`; M1-63: `CatchUp`, `Snapshot`, `listEvents`),
  `internal/authz` (M1-28: snapshot cache and `Authorize`; M1-29: `access.changed`),
  `internal/identity` (M1-34, M1-44, M1-45: sessions, tokens, revocation, `session.revoked`),
  `internal/kernel/*`, `internal/platform/lifecycle` and `internal/platform/health` (M1-11),
  `internal/platform/pg/pgtest` (M1-12), `internal/wire`.
- `docs/quality/gates.md`, `CONTRIBUTING.md`, `services/server/internal/gateway/README.md`.

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or
   doc comment (purpose, parameters, return value, errors, example where useful); comments explain
   why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision, a
   CHANGELOG entry per release, docs site kept current. A change without matching documentation is
   not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the
   merge.
6. Test first: every behaviour change has a test that was red before the change (record the red
   run in the report), then green, then clean up.
7. No en dash or em dash (U+2013, U+2014) in any file you write; the `text-rules` hook rejects
   them.
8. White label: no company, person, host or domain names except the examples Alex, Sam, Robin,
   `example.com` and the areas Support, Marketing, Engineering, Research; no paths under `/Users`.
9. Limits, windows and intervals only through named keys from `docs/operations/configuration.md`;
   no literal numbers in code or docs that a test does not hold.
10. Rights apply at delivery time. A gateway process updates the rights cache of a principal when
    its `access.changed` arrives in log order and checks every later event with the new state
    (04 "Grundregel", R-SY-02; 02 "Rechte-Cache").
11. Every `dispatch` passes `authorize(recipient, receive_action, resource)` first; personal events
    (`audience = self`) go only to the audience principal (05 "Dauerhafte Ereignistypen und
    Empfangs-Aktion"; 07 "Zustellfilter").
12. The hub never blocks on a connection; a connection whose queue exceeds
    `sync.outbound_queue_max` is closed with `4014` without delaying others (02 "Gateway", "Hub und
    Zustellung"; 04 "Ablauf" step 5).
13. Catch-up in pages of `sync.catchup_page_size`; `cursor` frames for skipped events at the latest
    after `sync.cursor_advance_every` and at the end of every page; live events buffered and sent
    after `catchup.complete {head_seq}` in `seq` order, never mixed (04 "Cursor", "Ablauf").
14. `resync_required {reason, head_seq, sync_epoch}` instead of events for the reasons `retention`,
    `too_far_behind` and `cursor_ahead` (04 "`resync_required`"). The epoch check (`epoch_changed`)
    runs once at `identify` and `resume` in M1-64 through `sync.CatchUp.Check` (M1-63), not here.
15. After `access.changed` nothing more from removed targets: buffered and queued frames of those
    targets are discarded (04 "Was der Client bei Entzug bekommt und tut", items 1 and 2).
16. Revocation of a session, device, principal or token closes the affected connections with
    `4012` (04 "Widerruf von Sitzung, Gerät oder Prinzipal"; 08 "Verlorenes Gerät", "Person
    sperren").
17. Shutdown of role `api` closes all connections with `4013 server_restart` (02 "Shutdown",
    step 3; R-AR-17).
18. AZ-P9: after the commit of a revocation in process A, nothing that depends on the revoked right
    is allowed or delivered by process B (07 "Pflicht-Tests").
19. A panic in the log tail or the hub ends the process with an error code; a panic on one
    connection closes it with `4000` (02 "Panics").

## Scope

Follow the package layout that M1-64 established in `internal/gateway`. Where M1-64 left no
place, use this layout (it satisfies 02 "Schichten im Modul"):

- `internal/gateway/domain/`: pure types and functions: connection phase (`catching_up`, `live`,
  `closing`), the bounded outbound queue with scope metadata and purge, the candidate rule of the
  hub, cursor bookkeeping. No I/O.
- `internal/gateway/app/`: `Hub` (indices, `Dispatch`), the per-connection catch-up runner
  (`catchup.go`), revocation handling (`revocation.go`), shutdown (`shutdown.go`). Imports only
  its own port and domain, `internal/kernel/*` and the port of `sync` (allowed edge in 02
  "Erlaubte Abhängigkeiten").
- `internal/gateway/adapters/ws/` (from M1-64): the writer goroutine drains the queue with
  deadline `gateway.write_timeout`; the handshake registers the connection in the hub before it
  reads `head_seq` (see "Connection phases").
- `internal/kernel/authorize/`: add `Observe` to the `Authorizer` interface if M1-28 did not, the
  type `LogEvent`, and the context marker `Delivery(ctx)`, which implies the filter mode
  `Filter(ctx)` of M1-28 (no denial audit, not counted).
- `internal/authz/`: implement `Observe` and the delivery mode of `Authorize` (`observe.go`,
  tests), including `thread.moved` (this ticket owns the cache side of moves; M1-62 builds the move
  itself, 04 "Grundregel").
- `internal/kernel/actor/` and `internal/identity/`: `Revalidate` (see "Revocation") if the
  `Authenticator` has no equivalent yet.
- `internal/sync/` port: expose what M1-63 built so the gateway can reuse it without copying:
  the resync check, a catch-up page with a fixed upper bound, and the per-event admission
  function (signatures below). Do not change REST behaviour of `listEvents`.
- `internal/wire/`: wire `Tail` -> `Authorizer.Observe` -> `Hub.Dispatch` in one goroutine per
  process, register the shutdown step and the liveness check.
- Docs: `services/server/internal/gateway/README.md` and `services/server/internal/authz/README.md`
  (German, like the existing files), `docs/spec/02-architektur.md` table "Querschnitts-Ports"
  (already names `Delivery` and `Revalidate`; change it only where the build proves it wrong),
  `docs/security/threat-model.md` row T-AZ-03 (add the new test names), `services/server/CHANGELOG.md`
  under "Unreleased", `docs/DECISIONS.md` (rulings listed below).

Interfaces (Go pseudo-code; keep existing names where earlier tickets already chose them):

```go
// internal/kernel/authorize
type LogEvent struct {
    Seq               int64
    Type              string      // event type as in docs/spec/actions.yaml
    PermissionVersion int64
    Audience          ids.ID      // zero unless audience = self
    Scope             Scope       // space, channel, thread ids; absent members are zero
    Resource          ResourceRef // kind, id, version
    AccessChanged     *AccessDiff // decoded payload of access.changed: added, removed, changed
}
type Authorizer interface {
    // existing: Authorize, Scope
    // Observe applies one durable event to the process-local rights cache. The log tail calls
    // it in seq order, before the hub sees the event.
    Observe(ctx context.Context, ev LogEvent)
}
// Delivery marks ctx as a delivery check (hub, gateway catch-up): cached state is validated
// against the Observe floor instead of reading sequence_counters, and it implies Filter(ctx) of
// M1-28: denials are filters, not requests, so they write no audit entry.
func Delivery(ctx context.Context) context.Context

// internal/kernel/actor (only if no equivalent exists)
// Revalidate re-checks an authenticated actor without a new proof: session not revoked,
// device active, principal active, API token not revoked. Access-token expiry stays with the
// reauth flow of M1-64 (4011). Returns an errs value of kind unauthenticated.
Revalidate(ctx context.Context, a Actor) error

// internal/sync port (names as M1-63 chose them; add what is missing)
Check(ctx, a actor.Actor, cursor int64) (head int64, resync *Resync, err error) // retention, too_far_behind, cursor_ahead
Page(ctx, a actor.Actor, after, upTo int64, limit int) (Page, error)          // admitted envelopes in seq order, LastChecked
Admit(ctx, a actor.Actor, ev Envelope) (bool, error)                        // receive action and audience; the single admission rule

// internal/gateway/app
func (h *Hub) Register(c *Conn)                         // before head_seq is read
func (h *Hub) Dispatch(ctx context.Context, ev sync.Envelope) // tail goroutine only; never blocks
func (h *Hub) CloseAll(code int, reason string)         // shutdown
```

## Out of scope

- Focus, typing, `EphemeralPublisher` and every ephemeral event (M1-66).
- Frame conformance harness G9 (M1-67); metrics (M1-68; do not add instruments here).
- Agent consumers: durable cursor, `ack`, redelivery, `4015`, `agent_consumer_cursors` (M2-07);
  invocation tokens and `runs.Delivery` (M2); `run.cancel_requested` handling (M2).
- The epoch check: it runs once at `identify` and `resume` in M1-64 through `sync.CatchUp.Check`
  (M1-63); the epoch only changes through a restore (M7-04).
- `cluster.mode = multi` and `NOTIFY ephemeral` (M7-17).
- Load tests and `make perf` targets (M1-72); leak tests (M1-70).
- Changes to the REST behaviour of `listEvents` or `getSnapshot` (M1-63).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-65-gateway-delivery`.
- Conventional Commits with the module as scope (for example `feat(gateway): ...`,
  `feat(authz): ...`) and DCO sign-off (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except the docs and changelog entries named here.
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp`, removed at the
  end. Tests bind `127.0.0.1` and port 0 only (P-10); PostgreSQL runs as a throwaway container
  through `pgtest` (P-09).
- Never read or print secrets. No network calls except package downloads for pinned tools.

## Design notes

Event path (one goroutine per process, 02 "Log-Tail je api-Prozess")

1. `sync.Tail` reads `event_log` in `seq` order. For each event: first
   `Authorizer.Observe(ev)`, then `Hub.Dispatch(ev)`. Nothing else consumes the tail in M1.
2. `Dispatch` must return without waiting on any connection: it only computes recipients,
   enqueues pre-encoded frames and schedules closes.
3. A panic in this goroutine is not recovered: the process exits with an error code (02
   "Panics"). Register a liveness check with `internal/platform/health` that fails once the tail
   or hub goroutine has exited, so `/healthz` reflects 02 "Health".

Rights cache in delivery mode (`internal/authz`)

- `Observe` on `access.changed` with audience P: set `floor[P] = ev.PermissionVersion` and drop the
  cached snapshot and revocation state of P.
- `Observe` on `channel.created`, `channel.updated` (carries `read_only`), `channel.archived`,
  `space.archived`, `thread.moved` (04 "Grundregel"): advance a process-wide structure generation;
  cached snapshots and resolved resource paths from an older generation are recomputed on next
  use. Reason: these events are rare, and a global generation is correct without knowing every
  affected principal. All other types: no-op.
- In `Delivery(ctx)` a cached snapshot of P is valid if its version is at least `floor[P]`, its
  generation is current and the clock is before `valid_until` (07 "Gültigkeit des Caches"). A
  recomputed snapshot reads current database state, which can only be newer than the log
  position; that is allowed (rights at delivery time).
- REST mode is unchanged: it keeps reading `permission_version` per request (R-RE-02).
- Delivery checks write no audit entry for denials (they are filters, 07 4.2; `Delivery(ctx)`
  implies `Filter(ctx)` of M1-28, which `listEvents` in M1-63 uses). Requests and ops keep R-AZ-04.
- A delivery decision `unauthenticated` for a connection means its credential was revoked: close
  that connection with `4012`.

Hub indices and candidates (02 "Hub und Zustellung")

- Indices: principal -> connections; space -> connections whose principal sees the space
  (`visible_space_ids` from `Authorizer.Scope`); channel -> connections (`visible_channel_ids`);
  instance-wide set (owner sessions, grants on the instance).
- Candidates of an event: audience set -> connections of that principal; else `space_id` set ->
  space index of that space, channel index of `channel_id`, instance-wide set; else (instance
  level, for example `instance.updated`, `user.updated`, `agent.updated`) -> all connections.
- The candidate set is only a performance filter and must be a superset of the allowed set; the
  truth is `sync.Admit` in `Delivery(ctx)` for each candidate. A new channel in a visible space is
  reached through the space index without a new rights version (02 "Ergänzungen am Kern-Vertrag"
  point 4).
- Recompute the index entries of P's connections from `Authorizer.Scope` (delivery mode) when
  P's `access.changed` is processed.
- Encode each event once (`{"op":"dispatch","d":<envelope>}` with the no-null encoder of M1-18)
  and enqueue the same bytes for every recipient. M1 has no delivery-only envelope fields.

Per-connection queue

- Bounded by `sync.outbound_queue_max` frames. Entries carry: frame bytes, class (`dispatch` or
  `control`), `seq` if any, scope ids and `occurred_at`.
- `Purge(removed)` drops queued `dispatch` entries whose channel or space is in the removed
  targets of an `access.changed`; control frames (`cursor`, `catchup.complete`, `error`,
  `heartbeat_ack`) are never purged.
- Overflow: mark the connection `closing`, drop its queue and close it with `4014` from the
  writer side. The writer writes each frame with deadline `gateway.write_timeout`; a write that
  misses the deadline also closes with `4014` (04 'Schließ-Codes', 02 'Gateway').

Connection phases and catch-up (04 "Ablauf")

1. Registration: the handshake calls `Delivery.Register` (M1-64) before it reads `head_seq` for
   `ready` or `resumed` (04 'Lebenszyklus' step 4); this ticket's implementation puts the
   connection into the hub in phase `catching_up`. Call the value H. Every event with `seq > H` the tail delivers from now on is either
   buffered or sent; every event with `seq <= H` is covered by the snapshot or the catch-up.
2. No cursor in `identify`: no catch-up frames; switch to `live` with base H immediately after
   `ready` (04 "Lebenszyklus" step 4: catch-up only when a cursor was sent).
3. Cursor c (from `identify` or `resume`): `sync.Check(c)`. On a resync reason send
   `resync_required {reason, head_seq: H, sync_epoch}` and switch to `live` with base H (the
   connection stays open, 04 '`resync_required`'). Otherwise read pages `after = c`, `upTo = H`,
   `limit = sync.catchup_page_size` and enqueue each admitted event as `dispatch`.
4. `cursor {seq}` frames: at the end of every page with the page's last checked `seq` (equal to H
   after the last page), and in addition whenever the number of skipped sequence numbers since
   the last frame that carried a `seq` reaches `sync.cursor_advance_every`.
5. End: `catchup.complete {head_seq: H}`; then, atomically with respect to `Dispatch`, run the
   live buffer through `Admit` in `seq` order (only `seq > H`), enqueue the admitted ones and
   switch to `live`. Admission at flush time applies the newest rights, so events of a target
   removed during the catch-up never go out.
6. The live buffer during catch-up is bounded by `sync.outbound_queue_max`; overflow closes with
   `4014`. The catch-up runner itself waits for queue space (it blocks only its own goroutine and
   stops when the connection closes).
7. A failing page read (database unavailable) is retried with backoff capped at
   `database.listen_reconnect_backoff_cap` while the connection is open; the gateway keeps
   connections and delivers nothing meanwhile (02 "Ausfall einer Abhängigkeit").

Live cursor frames (04 "Cursor", R-SY-03)

- The hub counts processed events. Every `sync.cursor_advance_every` processed events, each live
  connection whose last `seq`-carrying frame is older than the hub position gets
  `cursor {seq: hub position}`. Not being a candidate implies not visible, because the candidate
  set is a superset.

Revocation

- On P's `access.changed`, before it is dispatched: (a) `Revalidate` every connection of P and
  close failures with `4012`; (b) recompute P's index entries; (c) purge the queues and live
  buffers of P's connections for the `removed` targets; (d) dispatch `access.changed` through the
  normal path, in `seq` order (it must not overtake earlier frames, 04 "Zustellgarantie" item 2).
- On `session.revoked` (audience P, payload `session_id`): close every connection whose actor
  carries that session id with `4012`; it is delivered to P's other connections whose
  credentials are still valid.
- `user.disable`, `device.revoke`, `session.revoke`, `token.revoke` and `agent.disable` all raise
  the rights version and write `access.changed` (05 column "Rechte"), so the recheck in (a) covers
  every revocation even without a `session.revoked`.

Agents and apps in M1

- They are delivered like people, by their own grants and the operator rule. Without a cursor
  they are live from H. Durable consumer cursors, `ack` and redelivery are M2-07.

Shutdown (02 "Shutdown", role `api` and `all`)

- Register a lifecycle step after `http.Server.Shutdown` and before the tail stops:
  `Hub.CloseAll(4013, "server_restart")`, reject new `identify` and `resume` with `4013`, and wait
  for the writers to flush close frames within the remaining `shutdown.total_timeout` budget
  (each write still bounded by `gateway.write_timeout`). Saving agent cursors is M2-07.

Configuration keys used (defaults in their chapter tables): `sync.catchup_page_size`,
`sync.catchup_max_events`, `sync.event_retention`, `sync.cursor_advance_every`,
`sync.outbound_queue_max`, `sync.ws_max_frame_bytes`, `sync.tail_batch_size`,
`sync.log_poll_interval`, `gateway.write_timeout`, `database.listen_reconnect_backoff_cap`,
`authz.snapshot_cache_max_entries`, `shutdown.total_timeout`.

Ruling to record in `docs/DECISIONS.md` (with its reason): structure events invalidate all
cached snapshots through a generation counter. The other decisions of this ticket are in the
specification: the connection stays open after `resync_required` (04 7.3), a missed write
deadline closes with `4014` (04 6.4), delivery checks write no denial audit (07 4.2, R-AZ-04), and
AZ-P9 runs with two independent server instances in one test binary (07 "Pflicht-Tests").

If a change to 04 or 05 (core contract) turns out to be necessary, stop with
`Verdict: BLOCKED` (P-11). Changes to 02 named above are allowed.

## Tests (write them first)

Unit tests with fakes (fake `Authorizer`, fake `sync` port, a writer that blocks until released,
the `kernel/clock` test clock):

- `TestQueue_BoundedAndPurgeByScope`, `TestQueue_ControlFramesSurvivePurge`.
- `TestHub_DispatchNeverBlocksOnSlowConnection`: one writer blocked, `Dispatch` returns within a
  test deadline, the other connection receives every frame.
- `TestHub_SlowConsumerClosedWith4014`: set `sync.outbound_queue_max` low in the test config.
- `TestHub_PersonalEventsOnlyToAudience`.
- `TestHub_CandidatesSupersetOfAllowed`: property test in random worlds (principals, groups,
  grants, spaces, channels, events) with a seeded `math/rand/v2` generator; compare hub
  recipients with brute-force admission over all connections; print the seed on failure.
- `TestHub_UnauthenticatedDecisionCloses4012`.
- `TestHub_LiveCursorFrameAfterAdvanceEvery`.
- `TestCatchUp_PagesCursorFramesAndComplete`: about 2.5 pages, half of the events invisible.
- `TestCatchUp_LiveEventsBufferedThenFlushedInSeqOrder`: events committed while pages are read;
  assert the frame stream is `dispatch`/`cursor`..., `catchup.complete`, buffered events, and the
  `seq` of all `dispatch` frames strictly increases with no duplicate.
- `TestCatchUp_LiveBufferOverflowCloses4014`.
- `TestCatchUp_ResyncReasons`: table `retention`, `too_far_behind`, `cursor_ahead`; the connection
  stays open and receives later live events.
- `TestCatchUp_NoCursorNoCatchupFrames`.
- `TestAuthzObserve_AccessChangedRaisesFloor`, `TestAuthzObserve_StructureEventInvalidates`,
  `TestAuthzDelivery_DenialWritesNoAudit` (in `internal/authz`).

Integration tests against PostgreSQL 18 with all real modules, using the in-process server and
WebSocket test client from M1-64 and the step-up test authenticator of the identity tickets:

- `TestGateway_ReconnectWithCatchUp`: sequence diagram 04 "Wiederverbinden mit Nachholen".
- `TestGateway_RevocationDuringOpenConnection`: sequence diagram 04 "Entzug während offener
  Verbindung" without the agent part (M2): the Owner revokes Sam's grant on channel Support via
  `revokeGrant`; Sam receives `access.changed` with Support in `removed`; frames for Support queued
  before it (writer held) are not written; a message posted to Support afterwards does not reach
  Sam; Sam still receives events of Marketing.
- `TestGateway_RevocationCloses4012`: table over `revokeSession`, `revokeDevice`, `disableUser`,
  `revokeToken` (agent API token) and `disableAgent`; the affected connections close with `4012`;
  in the session case a second session of the same person stays open and receives
  `session.revoked`.
- `TestGateway_AZP9TwoInstances`: Sam connected to instance B; the Owner revokes on instance A;
  later events of the channel do not reach Sam on B, and a REST read on B returns `404`.
- `TestGateway_NewChannelInVisibleSpaceDelivered`.
- `TestGateway_EchoToAuthor`: `message.created` with `client_msg_id` reaches the author's own
  connection.
- `TestGateway_ShutdownClosesAllWith4013`: three clients receive `4013`; shutdown ends within
  `shutdown.total_timeout`.
- `TestHealth_HubStoppedFailsHealthz`.
- `BenchmarkHubDispatch` (many connections, one event): report the value; the gate is M1-72.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/gateway/... ./internal/authz/... ./internal/sync/... ./internal/identity/... ./internal/wire/...
make test
make check
```

## Acceptance criteria

- [ ] Every durable event reaches exactly the connections for which `Admit` allows it at delivery
      time; personal events only the audience (unit and property tests green).
- [ ] Catch-up sends pages with `cursor` frames, then `catchup.complete {head_seq}`, then the
      buffered live events in `seq` order; no gap, no duplicate, no interleaving.
- [ ] `resync_required` with `retention`, `too_far_behind` and `cursor_ahead` carries `head_seq` and
      `sync_epoch`; the connection stays live.
- [ ] A slow reader is closed with `4014` while other connections keep receiving; `Dispatch` never
      waits on a connection.
- [ ] After `access.changed` no frame of a removed target is written, including frames queued or
      buffered before it.
- [ ] Revoking a session, device, person, API token or agent closes exactly the affected
      connections with `4012`.
- [ ] AZ-P9 with two instances is green.
- [ ] Sequence diagrams 04 "Wiederverbinden mit Nachholen" and "Entzug während offener
      Verbindung" run as integration tests.
- [ ] Shutdown closes every connection with `4013` within `shutdown.total_timeout`; `/healthz`
      fails when tail or hub stop.
- [ ] Delivery denials write no audit entries; request denials still do.
- [ ] `make check` green.

## Definition of done

- Code and tests as above; doc comment on every public symbol (purpose, parameters, return value,
  errors); comments explain why.
- Module READMEs of `gateway` and `authz` updated (status, ports, actions, config keys, behaviour
  on revocation and shutdown); 02 table "Querschnitts-Ports" updated; threat model row T-AZ-03
  lists the new tests; rulings in `docs/DECISIONS.md`; `services/server/CHANGELOG.md` entry under
  "Unreleased". No new ADR is expected; write one if you take an architecture decision beyond
  this brief.
- `make check` green.
- Independent review (`make review` plus a code review over the diff) and the red team gate
  (`docs/quality/red-team-gate.md`) over the diff, verdict `JA` or `NEIN` recorded in the report.
  On `NEIN`: fix, add a test per finding, re-check the delta.
- Merge locally into the base branch only if review and red team are green and `make check` is
  green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then
commits, files with line counts, test results with counts (including the red runs before the
fixes), what is not proven, and follow-ups that could not be done in scope with the reason. Then
write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
