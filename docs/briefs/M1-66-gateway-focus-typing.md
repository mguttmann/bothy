# M1-66: Gateway focus and typing

Status: ready | Depends on: M1-65 | Etappe: M1

## Goal

After this ticket a client tells the gateway with the op `focus` which threads are on screen, and
the gateway sends focus-bound ephemeral events only to connections that focus the thread and may
read it. People see each other typing: `typing.start` produces the ephemeral event
`typing.started` for the focused readers of the thread, throttled and rate limited, ending by
itself after `sync.typing_ttl`. The gateway also becomes the process's `EphemeralPublisher`, the
single path for ephemeral events that `runs`, `agents` and `notify` use from M2 and M3 on.

## Read first

- `docs/PLAN.md`: section "Arbeitsweise je Ticket"; section "M1 Fundament", rows M1-64 to M1-68;
  rows M2-10 and M3-23 (later users of focus and of the publisher).
- `docs/spec/04-sync-protokoll.md`: "Zwei Arten", "Umschlag (Envelope)", "Ops" (row `focus` with
  `idle`, rows `typing.start` / `typing.stop`), "Lebenszyklus" step 7 (`resume` restores focus),
  "Was der Client bei Entzug bekommt und tut" item 2 (focus ends), "Zustellgarantie" item 4,
  "Reihenfolge", "Vom Server zu den Clients" item 2 (`sync.max_focused_threads`), "Tippen und
  Agenten-Präsenz" item 1 and item 3, "Konfigurationswerte dieses Kapitels".
- `docs/spec/05-aktionen.md`: "Rate-Limit-Klassen" (`rl.read` keyed by principal, `rl.typing`
  keyed by connection), action rows `gateway.focus` and `typing.start`, "Verbindliche Regeln"
  rule 8 (read-only channels block typing), "Kurzlebige Ereignisse" (Zusatzbedingung "Thread im
  Fokus"), "WebSocket-Ops und Aktion".
- `docs/spec/02-architektur.md`: "Querschnitts-Ports" (`kernel/eventlog.EphemeralPublisher`),
  "Ports, Ereignisse und Quellen je Modul" (row `gateway`), "Erlaubte Abhängigkeiten" (`gateway`
  may use `chat`), "Kurzlebige Ereignisse".
- `docs/spec/13-push-benachrichtigungen.md`: "Aktiv am Gerät" (meaning of `idle`).
- `docs/spec/01-produkt.md`: feature row "Tippen-Anzeige (Menschen)".
- `docs/api/asyncapi.yaml`: `receive.focus` (`x-rate-limit-class: rl.read`), `receive.typing.start`,
  `receive.typing.stop`, message `event.typing.started`. `docs/api/openapi.yaml` schemas
  `GatewayFocus`, `GatewayTyping`, `TypingStartedEvent`, `TypingStartedPayload`, `EventEnvelope`,
  `GatewayError`.
- ADR `docs/adr/0017-*` and 0053 (no JSON null).
- `docs/security/threat-model.md` "Gateway (WebSocket)" (row "Fokus auf fremde Threads").
- Code you build on: `internal/gateway` (M1-64, M1-65: hub, queue, phases, test client),
  `internal/kernel/eventlog`, `internal/kernel/authorize` (`Delivery(ctx)` from M1-65),
  `internal/platform/ratelimit` (M1-19), the `chat` port (`Threads` info, M1-55).
- `docs/quality/gates.md`, `CONTRIBUTING.md`, `services/server/internal/gateway/README.md`.

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
6. Test first: every behaviour change has a test that was red before the change (record the red
   run in the report), then green, then clean up.
7. No en dash or em dash (U+2013, U+2014) in any file you write.
8. White label: only the examples Alex, Sam, Robin, `example.com` and the areas Support, Marketing,
   Engineering, Research; no paths under `/Users`.
9. Limits and intervals only through named keys from `docs/operations/configuration.md`.
10. `focus {thread_ids, idle}` replaces the previous set; each thread is checked with
    `authorize(principal, gateway.focus, thread)`; unreadable threads are ignored silently; at
    most `sync.max_focused_threads` threads (04 "Ops", "Vom Server zu den Clients"; 05 row
    `gateway.focus`).
11. `typing.start {thread_id}` is checked with action `typing.start` and rate class `rl.typing`;
    `typing.stop` uses the same action and class (05 "WebSocket-Ops und Aktion").
12. The server fans out `typing.start` at most once per `sync.typing_min_interval` and sends
    `typing.started {thread_id, principal_id, expires_at}` only to connections that focus the
    thread and may read it; without renewal the display ends after `sync.typing_ttl` (04 "Tippen
    und Agenten-Präsenz" item 1).
13. Agents send no typing of their own; a running run shows the agent's typing (04 "Tippen und
    Agenten-Präsenz" item 1; ruling L9; 01 "Tippen-Anzeige (Menschen)"). 05 gives `typing.start`
    the right "human und write", so `authorize` denies agents and apps.
14. Ephemeral events have no `seq` and no `permission_version`, are delivered at most once and are
    lost during a pause (04 "Zwei Arten", "Zustellgarantie" item 4, "Lebenszyklus" step 7).
15. When `access.changed` removes a target, the focus on threads of that target ends (04 "Was der
    Client bei Entzug bekommt und tut" item 2).
16. `resume` restores the focus (04 "Lebenszyklus" step 7).
17. A failing op answers `error` with a problem code and keeps the connection; `4008` is only for
    `rl.gateway_connect` (04 "Ops", "Schließ-Codes"). `focus` over `sync.max_focused_threads`
    answers `validation_failed` and keeps the previous set (04 11.3 item 2). `typing.stop` sends
    nothing and resets the throttle; `typing.started` never reaches the typist's own connections
    (04 12 item 1). Ephemeral frames are dropped when the queue is full (never `4014`) and never
    sent before `catchup.complete` (04 7.2 item 6).

## Scope

- `internal/kernel/eventlog/`: the type `EphemeralEvent` and the interface `EphemeralPublisher`
  (if M1-05 or M1-16 declared them, keep their shape and extend only what is missing):

```go
// EphemeralEvent is a short-lived event (04 "Zwei Arten"): no seq, never stored, at most once.
type EphemeralEvent struct {
    Type          string      // ephemeral type from docs/spec/actions.yaml
    SchemaVersion int
    Actor         *ActorRef   // absent for system events
    Scope         Scope       // space, channel, thread ids
    Resource      ResourceRef // kind, id, optional version
    Audience      ids.ID      // set only for personal ephemeral types
    Payload       any         // encoded with the no-null encoder
}
// EphemeralPublisher hands an ephemeral event to the gateway of this process. It never waits on
// a connection. Errors only report programming faults (unregistered type, payload over
// sync.event_payload_max_bytes).
type EphemeralPublisher interface {
    PublishEphemeral(ctx context.Context, ev EphemeralEvent) error
}
```

- `internal/gateway/domain/`: `FocusSet` (dedupe, limit check, `idle` flag), `TypingThrottle`
  (last fan-out per connection and thread, clock-driven), the set of focus-bound ephemeral types
  `typing.started` and `run.updated` with a doc comment citing 05 "Kurzlebige Ereignisse".
- `internal/gateway/app/`: op handlers `focus.go` and `typing.go`; `ephemeral.go` implementing
  `EphemeralPublisher` on top of the hub; the hub gains the index thread -> focused connections
  and focus pruning.
- `internal/gateway/adapters/ws/`: register the ops `focus`, `typing.start`, `typing.stop` in the
  op register (M1-64/M1-25) with action and rate class as in 05 and asyncapi.yaml, and move them
  from `gateway_ops.later` to `gateway_ops.enabled` in `services/server/api/operations.yaml`
  (P-04).
- `internal/wire/`: provide the gateway publisher as the process's
  `kernel/eventlog.EphemeralPublisher`.
- Docs: `services/server/internal/gateway/README.md` (German, like the existing file),
  `services/server/CHANGELOG.md` under "Unreleased", `docs/DECISIONS.md` (only decisions beyond the specification).

## Out of scope

- `run.updated` patches, coalescing and the full document on focus (M2-10); agent presence
  (M2-20); `notification.created` and the use of `idle` for push (M3-23);
  `command.autocomplete_requested` (M2). This ticket only provides the publisher they will use.
- Cross-node distribution through `NOTIFY ephemeral` for `cluster.mode = multi` (M7-17). Design the
  publisher so M7-17 can add forwarding as a decorator without changing callers.
- Frame conformance (M1-67); metrics (M1-68, see the note on `bothy.delivery.duration` below).
- Any durable event, catch-up or revocation logic beyond focus pruning (M1-65).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-66-gateway-focus-typing`.
- Conventional Commits with the module as scope (`feat(gateway): ...`) and DCO sign-off.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except the docs and changelog entries named here.
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp`, removed at the
  end. Tests bind `127.0.0.1` and port 0 only; PostgreSQL through `pgtest`.
- Never read or print secrets. No network calls except package downloads for pinned tools.

## Design notes

Op `focus` (order of steps as in 05 "Entscheidung": rate limit before `authorize`)

1. Rate limit `rl.read` keyed by principal (class from asyncapi `receive.focus`). Over the limit:
   frame `error {op: "focus", code: "rate_limited"}`, connection stays.
2. Decode `GatewayFocus`; malformed body follows M1-64's decode rule (`4002`).
3. Deduplicate `thread_ids`. More than `sync.max_focused_threads` distinct ids: frame
   `error {op: "focus", code: "validation_failed", detail: "sync.max_focused_threads"}` and the
   previous set stays (04 11.3 item 2).
4. For each id: load the thread through the `chat` port, then
   `authorize(principal, gateway.focus, thread)` in the normal request context (denials are
   audited per R-AZ-04 by authz; no error frame). Unknown and unreadable ids are both dropped
   silently, with the same code path (02 "404 statt 403").
5. Replace the connection's set atomically in the hub index; store `idle`. No other code reads
   `idle` in M1 (M3-23 does); do not add an accessor without a user (P-13).
6. `focus` is accepted in phase `catching_up` too; ephemeral frames still wait for `live`.

Ops `typing.start` and `typing.stop`

1. Only people may send them: `authorize` denies agents and apps with `forbidden`
   (`subject_kind_not_allowed`, 05 right "human und write"); they get `error {op, code:
   "forbidden"}` and the frame has no effect (requirement 13). If the requirement step of M1-28
   lacks the `human` part of a combined right, add it there with a test (no rights logic in the
   gateway).
2. Rate limit `rl.typing` keyed by connection (`ratelimit.rl.typing.*`); over the limit:
   `error {op, code: "rate_limited"}`.
3. Decode `GatewayTyping`; load the thread; `authorize(principal, typing.start, thread)`. A denial
   answers `error {op, code}` with the decision's code (`not_found`, `forbidden`,
   `channel_read_only`); 05 rule 8 makes read-only channels deny typing for everyone.
4. `typing.start`: if the last fan-out for this connection and thread is younger than
   `sync.typing_min_interval`, do nothing (no frame, no error). Otherwise record the time and
   publish `typing.started` with payload `{thread_id, principal_id, expires_at = now +
   sync.typing_ttl}` (RFC 3339 UTC with milliseconds), envelope `id` (UUIDv7), `schema_version`
   from the event register, `occurred_at`, `actor {id, kind: user}`, `scope {space_id,
   channel_id, thread_id}`, `resource {kind: thread, id: thread_id}`; no `seq`, no
   `permission_version`.
5. `typing.stop`: clears the throttle entry for this connection and thread so the next
   `typing.start` fans out at once; it sends no event, because 05 and asyncapi.yaml define no
   "typing stopped" type. Clients end the display at `expires_at` or on the next message of that
   person.
6. Recipients of `typing.started`: connections in phase `live` that focus the thread, minus every
   connection of the typing principal (04 12 item 1), each admitted with `thread.read` in
   `authorize.Delivery(ctx)` (05 "Kurzlebige Ereignisse").

Publisher and hub

- Candidates of an ephemeral event: focus-bound type -> thread index of `scope.thread_id`;
  `Audience` set -> connections of that principal; otherwise the scope rule of M1-65. Admission:
  the same `Admit` path as durable events, receive action from the event register.
- Encode once per event; frame `{"op":"dispatch","d":<envelope>}`.
- Ephemeral frames are never delivered to a connection before its `catchup.complete` and are
  never buffered (at most once; 04 "Ablauf" item 3 forbids mixing).
- If enqueuing an ephemeral frame would exceed `sync.outbound_queue_max`, drop that frame; never
  close with `4014` for an ephemeral frame (04 7.2 item 6). Durable frames keep M1-65's rule.
- `PublishEphemeral` never blocks; it validates the type against the register and the payload
  size against `sync.event_payload_max_bytes`.
- Focus pruning: on P's `access.changed` (inside M1-65's revocation step, before dispatch),
  re-check every focused thread of P's connections with `gateway.focus` in
  `authorize.Delivery(ctx)` and drop the unreadable ones; on `thread.moved`, `channel.archived`
  and `space.archived` re-check the connections that focus an affected thread.
- `resume`: the focus set lives in the in-memory gateway session; when M1-64's `resume` restores
  the session, re-check each thread as in pruning and keep the readable ones.

Metrics: none in this ticket. M1-68 depends on this ticket and records `bothy.delivery.duration`
for the durable and the ephemeral path.

Configuration keys used: `sync.max_focused_threads`, `sync.typing_min_interval`,
`sync.typing_ttl`, `sync.outbound_queue_max`, `sync.event_payload_max_bytes`,
`ratelimit.rl.typing.per_minute`, `ratelimit.rl.typing.burst`, `ratelimit.rl.read.per_minute`,
`ratelimit.rl.read.burst`.

No rulings of its own: the behaviour of requirements 10 to 17 is in 04 ("Ops", "Ablauf",
"Vom Server zu den Clients", "Tippen und Agenten-Präsenz") and 05 (`typing.start`). Record in
`docs/DECISIONS.md` only what the build had to decide beyond them.

## Tests (write them first)

Unit tests (fake `Authorizer`, fake `chat` port, test clock, fake rate limiter):

- `TestFocusSet_DedupeAndLimit`, `TestFocus_OverLimitRejectedKeepsPrevious`.
- `TestFocus_UnreadableAndUnknownIgnoredSilently`: no error frame, only readable ids kept.
- `TestFocus_ReplacesPreviousSet`, `TestFocus_IdleStored`, `TestFocus_RateLimitedErrorFrame`.
- `TestTyping_ThrottlePerConnectionAndThread`: second start inside `sync.typing_min_interval` no
  fan-out; after the interval fan-out; after `typing.stop` immediate fan-out.
- `TestTyping_RateLimitedErrorFrame` (class `rl.typing`, key connection).
- `TestTyping_OnlyPeople`: agent connection gets `error forbidden`, nothing published.
- `TestTyping_DeniedCodes`: table unreadable thread -> `not_found`, `read` grant -> `forbidden`,
  read-only channel -> `channel_read_only`.
- `TestTypingStarted_OnlyFocusedReaders`: focused reader receives; reader without focus does not;
  focused connection without read right does not; the typist's own second connection does not.
- `TestTypingStarted_EnvelopeAndExpiry`: no `seq`, no `permission_version`, `expires_at =
  occurred_at + sync.typing_ttl`.
- `TestEphemeral_NotDeliveredDuringCatchUp`, `TestEphemeral_DroppedWhenQueueFullNoClose`,
  `TestEphemeral_PublishNeverBlocks`, `TestEphemeral_UnregisteredTypeRejected`.
- `TestFocus_PrunedOnAccessChanged`, `TestFocus_RecheckedOnThreadMoved`,
  `TestFocus_RestoredOnResume`.

Integration test against PostgreSQL 18 with real modules and M1-64's test client:

- `TestGateway_TypingEndToEnd`: Alex and Sam have `write` on Support, Robin has no grant; Sam and
  Robin focus the same thread id (Robin's focus is dropped); Alex sends `typing.start`; Sam
  receives `typing.started` with the right scope and `expires_at`; Robin and Alex's own second
  device receive nothing.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/gateway/... ./internal/kernel/... ./internal/wire/...
make test
make check
```

## Acceptance criteria

- [ ] `focus` keeps at most `sync.max_focused_threads` threads, ignores unreadable and unknown ids
      silently, replaces the previous set and stores `idle`.
- [ ] `typing.start` fans out at most once per `sync.typing_min_interval` per connection and
      thread and is limited by `rl.typing`.
- [ ] `typing.started` reaches only live connections with focus on the thread and read right,
      never the typist's own connections, and carries `expires_at = occurred_at +
      sync.typing_ttl`.
- [ ] Agents and apps cannot produce typing.
- [ ] Focus ends for targets removed by `access.changed` and survives `resume` where still
      readable.
- [ ] The gateway is the process's `EphemeralPublisher`; ephemeral frames never close a
      connection and never interleave with catch-up.
- [ ] `make check` green.

## Definition of done

- Code and tests as above; doc comment on every public symbol (purpose, parameters, return value,
  errors); comments explain why.
- Gateway README updated (ops, config keys, ephemeral rules); rulings in `docs/DECISIONS.md`;
  `services/server/CHANGELOG.md` entry under "Unreleased". Write an ADR only if you take an
  architecture decision beyond this brief.
- `make check` green.
- Independent review (`make review` plus a code review over the diff) and the red team gate
  (`docs/quality/red-team-gate.md`) over the diff, verdict recorded in the report.
- Merge locally into the base branch only if review and red team are green and `make check` is
  green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then
commits, files with line counts, test results with counts (including the red runs), what is not
proven, and follow-ups that could not be done in scope with the reason. Then write
`{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
