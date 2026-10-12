# Module `sync`

Status: built in part (M1-16, M1-20, M2-01): the event log with the commit-ordered sequence, the phase hook of `sync`, the log tail, the partitions of `event_log`, the one envelope of gateway, REST and webhooks, the idempotency keys behind `kernel/idempotency.Store` with the job `sync.idempotency_cleanup`, and the frame of the worker consumers with `event_consumer_cursors` (no production consumer yet: M2-26 registers `webhooks`, M3-19 `push`). Catch-up, snapshot, redaction, read states, drafts and settings come with M1-63, M1-58 and the tickets that need them.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); where they differ, the chapter applies.

## Purpose

Event log (append, tail, catch-up, redaction, partitions), snapshot, idempotency keys, read state, drafts, personal settings, cursors of the worker consumers.

## Code

| Package | Content |
|---|---|
| `internal/sync` | port: `Event` (a stored row, zero values for NULL), `DeliveryFields`, `Envelope` with `EnvelopeActor`, `EnvelopeScope`, `EnvelopeResource` and `Event.Envelope`, `Sink`, `Tail`, `ErrGap`; worker consumers: `ConsumerName` (`ConsumerPush`, `ConsumerWebhooks`, `ConsumerNames`, `Valid`), `ConsumerHandler`, `ConsumerState`, `Consumers` |
| `domain/` | event validation (`ValidateEvent`, `CheckPayload`), partition arithmetic (`PartitionLower`, `PartitionName`), the gap check of the tail (`CheckContiguous`, `GapError`) and the round plan of a worker consumer (`PlanRound`, `RoundPlan`, `ErrConsumerGap`) |
| `app/` | `Appender` (implements `kernel/eventlog.Appender`), `PhaseHook` (phase `sync`), `PartitionMaintenance`, `IdempotencyStore` (implements `kernel/idempotency.Store`), `IdempotencyCleanup`, `ConsumerService` (implements `sync.Consumers`: `Register`, `Ensure`, `Round`, `States`) and `ConsumerRunner`; outgoing ports `Writer`, `Partitions`, `Correlator`, `KeyRows`, `ConsumerCursorStore`, `ConsumerLogReader`, `Waker` |
| `adapters/postgres/` | `Repository` (writer, partitions, head, minimum, reads, the cursor store and log reader of the worker consumers) and `IdempotencyRepository` (`KeyRows`) over sqlc queries in `queries/`, generated code in `sqlcgen/` (entry in `services/server/sqlc.yaml`); `Tail` and `Waker`, each with its own `LISTEN` connection over one shared listening loop (`listener`) |
| `adapters/otelmetrics/` | the lag gauges of the worker consumers (`RegisterConsumerGauges`) |
| `adapters/telemetry/` | `Correlator`: `correlation_id` and `trace_context` from the trace context of the append |
| `adapters/jobs/` | jobs `sync.partition_maintenance` and `sync.idempotency_cleanup`: arguments, handlers, periodic registrations |

### Writing durable events

A use case appends `eventlog.Event` values through `eventlog.Appender` in the phase `work` (or
`rights_check`, for `access.changed` of authz). The appender rejects with kind `internal`, before
anything is buffered: a call outside a unit of work or in another phase, a type that does not match
`^[a-z][a-z_]*(\.[a-z][a-z_]*)+$`, a schema version outside 1..32767, an empty resource kind or
receive action, `self` without an audience, an actor kind outside `user`, `agent`, `app`,
`system`, a principal for `system` or without a kind, a missing principal for `user`, `agent` and
`app`, and a payload that is not one JSON object in valid UTF-8 smaller than
`sync.event_payload_max_bytes`, or that contains a JSON `null` at any depth, a number with an
exponent (jsonb writes `1e100000` out in full, so the stored size would be unbounded) or with more
than 131072 digits before or 16383 after the point (the limits of numeric, [Numeric types](https://www.postgresql.org/docs/18/datatype-numeric.html),
retrieved 2026-10-13), the character U+0000 or an unpaired surrogate escape (jsonb refuses both,
[JSON types](https://www.postgresql.org/docs/18/datatype-json.html), retrieved 2026-10-13). The reads hand the sinks
the compact form of the stored jsonb, which is never larger than the appended payload. A producer
with a larger object builds a reduced payload and sets `Truncated` (04 payload rule 2). Events
without ID get a UUIDv7; the correlation of the append context is attached: the trace ID of a
valid span as `correlation_id` (without a span the correlation ID of the request in
`kernel/reqmeta`), and the W3C `traceparent` as `trace_context` only for a sampled span.

The phase hook of `sync` (02 "Flow" step 3.2) does nothing for a unit without events, so such a
unit takes neither the counter lock nor `NOTIFY` (02 "Cost of serialization"). Otherwise it
advances `event_seq` by n through `instance.Counters` (the lock holds until the commit), reads
`permission_version` after the lock, and inserts the rows in append order with
`seq = last - n + 1 .. last` together with `pg_notify('event_log', last)` in one statement
(`InsertEvents`, a data-modifying CTE), which keeps the lock one round trip shorter.
`occurred_at` is the column default `now()`, the transaction time. An insert that no partition
takes (SQLSTATE 23514) is kind `unavailable`; the error line names the first seq, never a
payload.

### Log tail

One `Tail` per api process (roles `api` and `all`, start step 8 before the public listener;
02 "Log tail per api process"): `Run` sets `local_head` to `max(seq)`, holds its own connection
outside the pool (`pg.Pool.DialDedicated`, application name `bothy-event-tail`) for
`LISTEN event_log`, and reads `seq > local_head ORDER BY seq LIMIT sync.tail_batch_size` on every
notification and every `sync.log_poll_interval` until a short batch. It checks that the numbers
continue `local_head + 1` without a hole (else `ErrGap`), hands each batch to the sinks in order
from its own goroutine and advances `local_head`. A lost `LISTEN` connection is reconnected with
exponential backoff from `sync.log_poll_interval` up to `database.listen_reconnect_backoff_cap`
while polling goes on; nothing is lost because `seq` is gapless. A read error is logged and
retried; a sink error or a gap ends `Run`, and the process exits with code 1 (02 "Panics"). The
sink list is empty today; M1-28 adds the rights cache, then M1-65 the hub.

### Worker consumers

The port `Consumers` (02 "Ports, events and sources per module", "Worker-Cursor") runs the
consumers of the event log in the worker (02 "Worker consumers", ADR 0051). They are not the
agent consumers of the module `agents` (`agent_consumer_cursors`, M2-07), which that port also
calls `Consumers`: worker consumers are internal, have no API, consuming is no action and writes
no audit entry.

- **Table.** Migration `00009_event_consumer_cursors.sql`: `event_consumer_cursors` as 03 4.10
  (`name` with `CHECK (name IN ('push','webhooks'))`, `last_seq` default `0`, `updated_at`,
  primary key `name`, no foreign key, retention class `config`); `bothy_app` has `SELECT`,
  `INSERT`, `UPDATE`, no `DELETE`. No row is seeded: `Ensure` inserts the row of each registered
  consumer at `max(seq)` (D-M2-01-1), so a consumer of a later release never replays old events.
- **Registration.** Only `internal/wire` calls `Register`, before the runner starts; an invalid
  name, a nil handler, a second registration and a registration after `Start` fail. The
  production list is empty until M2-26 and M3-19.
- **Round.** `ConsumerService.Round` is one unit of work: `SELECT last_seq ... FOR UPDATE SKIP
  LOCKED` (no row: `RoundLocked` at once), up to `consumer.batch_size` events after it in `seq`
  order (`RoundIdle` without), `domain.PlanRound`, the handler per event in phase `work` (it
  inserts River jobs with `InsertTx` in its own adapters), the refusal of durable events the
  handler appended (D-M2-01-4), the cursor update and the commit. Cursor and jobs commit together,
  so each event creates its jobs exactly once, with any number of workers.
- **Gaps and failures.** Events removed by retention (first number equals `min(seq)`) are skipped
  and logged once per round at error level (`event consumer lost events to retention`, with
  `bothy.consumer.name` and `bothy.event.seq`); any other hole is `ErrConsumerGap`. A gap, a
  handler error or panic and a database error roll back the whole round; no event is skipped and
  the consumer stalls (D-M2-01-2, D-M2-01-3).
- **Wake.** `ConsumerRunner` runs one goroutine per registered consumer; `Waker` holds one `LISTEN
  event_log` connection per process (application name `bothy-consumer-wake`, apart from the
  tail), each notification wakes every consumer through a channel of capacity one, and each
  consumer also polls every `consumer.poll_interval`. A full batch starts the next round at once.
  The first failed round after a success logs `event consumer round failed` (`error.type`), the
  next success `event consumer recovered`. Without a registered consumer the runner opens no
  connection.
- **Lifecycle.** Roles `worker` and `all`: after River, `Ensure`, then the runner (component
  `event_consumers`); stop step `event_consumers` in `lifecycle.PhaseConsumers` without own
  budget; an unexpected end of the runner ends the process with exit code 1.
- **Gauges.** Every role registers `bothy.event.consumer.lag` (`head_seq - last_seq`) and
  `bothy.event.consumer.pending_age` (now minus `occurred_at` of the oldest pending event) with
  the attribute `bothy.consumer.name` (`adapters/otelmetrics`, until the registry of M1-68 takes
  them over); the alarm `BothyEventConsumerLag` fires at `consumer.pending_alert_after`.
- **Reads without the visibility predicate.** `EventsAfter` (shared with the tail),
  `OldestEventAfter`, `ListConsumerCursors` (head) and `MinRetainedSeq` read every event without
  the visible-channel predicate, like the log tail: each consumer job checks `push.deliver` or
  `webhook_out.deliver` for its receiver before it sends (07 7.4 rule 1, R-AZ-11). They belong
  to the category `log_tail` of the visibility lint of M1-70 when it lands.

### Envelope

`Event.Envelope(DeliveryFields)` is the one envelope of gateway, REST catch-up and webhooks
(ADR 0052, OpenAPI `EventEnvelope`): `seq` and `permission_version` always (pointers, so 0 is
present); `actor` iff an actor kind is set and `actor.id` iff a principal is set; `scope` always
an object; `resource.version` only when set; `audience: "self"` iff an audience is set; `redacted`
and `truncated` only when true; `trace_context` only with `IncludeTraceContext` (deliveries to
agents); `invocation_token`, `redelivered` and `sync_epoch` only from `DeliveryFields`, never
stored (R-SY-17); `occurred_at` in RFC 3339, UTC. Every optional member has `omitzero`, so the
envelope never contains JSON `null` (ADR 0053, R-SY-16; `FuzzEnvelopeHasNoJSONNull`,
`TestEnvelope_FieldsMatchOpenAPI`).

### Table `event_log` and its partitions

Migration `00006_sync_event_log.sql` creates `event_log` as in 03: `PARTITION BY RANGE (seq)`,
primary key `(seq)`, unique `(id, seq)`, the checks of `actor_kind` (R-DM-29) and of `self`, the
indexes `(channel_id, seq)`, `(space_id, seq)`, `(audience_principal_id, seq)` (partial),
`(resource_kind, resource_id)` and BRIN `(occurred_at)`, no default partition and no foreign key.
`bothy_app` has `SELECT, INSERT` on the table only (redaction adds its column grant with M1-58)
and no right on the partitions, which it reaches only through `event_log`.

Partitions change only through two `SECURITY DEFINER` functions of `bothy_owner` with
`search_path = public, pg_temp`, `EXECUTE` revoked from `PUBLIC` and granted to `bothy_app`, which
take only numbers and times and build every name with `format('%I')` (16 "Roles", R-OP-17):

- `sync_event_log_ensure_partitions(p_size, p_ahead, p_head)` (size 1 to 10^12, ahead 0 to 1000,
  the bounds of the configuration keys) takes `SHARE UPDATE EXCLUSIVE` on
  `event_log` first, so concurrent runs serialise and the function is idempotent, reads the
  bounds from `pg_inherits` and `pg_get_expr(relpartbound, oid)`, and while the highest upper
  bound is at most `p_head + p_ahead * p_size` creates `[upper, upper + p_size)` (the first starts
  at 1) as a table `LIKE event_log INCLUDING DEFAULTS INCLUDING CONSTRAINTS` named
  `event_log_from_<lower bound, 19 digits>`, adds a `CHECK` matching the bounds, attaches it and
  drops the `CHECK`. `ATTACH PARTITION` takes only `SHARE UPDATE EXCLUSIVE` on the parent, so
  inserts continue, and the matching `CHECK` lets it skip the scan that validates the partition
  constraint ([ALTER TABLE](https://www.postgresql.org/docs/18/sql-altertable.html), "ATTACH
  PARTITION"; [Table Partitioning](https://www.postgresql.org/docs/18/ddl-partitioning.html),
  "Partition Maintenance"; both retrieved 2026-10-13). `CREATE TABLE ... PARTITION OF` would take `ACCESS EXCLUSIVE`
  on the parent instead. The `CHECK` constraints of the parent must exist on the new table for
  the attach, hence `INCLUDING CONSTRAINTS`.
- `sync_event_log_drop_expired(p_cutoff, p_head, p_lock_timeout)` sets `lock_timeout` for the
  transaction, clamps the head to `max(seq)` and the cutoff to `now()` (it trusts no caller beyond
  the table), and, in bound order, detaches with the plain `DETACH PARTITION` and drops each
  partition whose upper bound is at most `p_head` (so the partition holding the head is never
  touched) and whose newest `occurred_at` is older than `p_cutoff` or that is empty. It stops at
  the first partition it keeps, so the retained sequence has no hole even when `occurred_at` (the
  start of a transaction) does not grow with `seq`. Plain `DETACH PARTITION` takes
  `ACCESS EXCLUSIVE` on the parent, bounded by the lock timeout; `DETACH PARTITION ...
  CONCURRENTLY` cannot run inside a transaction block and therefore not in a function (both
  pages above, "DETACH PARTITION").

`max(seq)` is the head and equals the committed counter, because the head partition is never
dropped; `min(seq)` is `min_retained_seq` for M1-63 (`Repository.MinRetainedSeq`).

### Job `sync.partition_maintenance`

`PartitionMaintenance.Run` reads the head and runs `ensure(sync.event_log_partition_size,
sync.event_log_partitions_ahead, head)` in one unit of work, then `drop_expired(now -
sync.event_retention, head, database.lock_timeout)` in a second one; neither appends an event, and
the time comes from `kernel/clock`. A DETACH that cannot get its lock (a long reader such as a
backup) therefore never rolls back the partitions ahead: the run returns `ErrDropDeferred` for a
lock timeout (any other drop error unchanged), the job is retried later, and `bothy migrate up`
treats a deferred drop as success. The job runs on queue `maintenance` every `sync.partition_maintenance_interval`
with `RunOnStart` (02 "Job catalogue"), and `bothy migrate up` runs the same use case once after
River, goose and the instance row (upgrade.md rule M10), so a fresh or upgraded database has
partitions before `serve`. Test databases of `pgtest` have no partitions; tests create them with
the use case and their own size.

### Idempotency keys (M1-20)

Migration `00007_sync_idempotency_keys.sql` creates `idempotency_keys` exactly as in 03:
`principal_id`, `key`, `action`, `request_sha256`, `state` (`in_progress` or `completed`, default
`in_progress`), `response_status`, `response_body` (jsonb), `resource_id`, `created_at`,
`expires_at`; primary key `(principal_id, key)`, foreign key to `principals(id)` with
`ON DELETE CASCADE`, index `(expires_at)`, retention class `transient`. `bothy_app` has `SELECT,
INSERT, UPDATE, DELETE`. `created_at` and `expires_at = created_at + sync.idempotency_key_ttl`
come from `kernel/clock` through the request of the stage (03 principle 2).

`IdempotencyStore` is the kernel port `idempotency.Store` the Idempotency-Key stage of
`platform/httpserver` uses (02 "Idempotency in the Unit of Work"):

- `Claim` (ambient transaction): delete the row of the key if `expires_at <= now`, then
  `INSERT ... ON CONFLICT (principal_id, key) DO NOTHING` with `in_progress`; a taken key returns
  its committed row. A concurrent uncommitted insert of the same key makes the insert wait; a wait
  that ends with `database.lock_timeout` (SQLSTATE `55P03`) becomes `idempotency.ErrInProgress`
  in the adapter, without wrapping the database error, so the unit of work does not retry it.
- `Complete` (same transaction): `completed` with status, body and `resource_id`.
- `SaveFinal` (own unit of work, `txn.Detach`): delete an expired row of the key, then insert the
  completed row unless one exists.
- Reads hand out the compact form of the stored jsonb.

The job `sync.idempotency_cleanup` (`IdempotencyCleanup`, queue `maintenance`, periodic every
`sync.idempotency_cleanup_interval`, run only by `worker` and `all`, no arguments) deletes rows
with `expires_at <= now` in batches of `sync.idempotency_cleanup_batch_size`, oldest first, each
batch in its own unit of work with `FOR UPDATE SKIP LOCKED`, until a batch is short; on
cancellation it returns `ctx.Err()`. A repeated run is harmless.

Configuration: `sync.idempotency_key_ttl` (04), `sync.idempotency_cleanup_interval` and
`sync.idempotency_cleanup_batch_size` (02).

### Load test

`perf/event_commit` (`make perf`, build tag `perf`) measures `perf.event_commit_min_per_s` on a
database from `pgtest.Durable()` (`fsync` and `synchronous_commit` on, ADR 0063 point 2) and fails
below the value of 02; the threshold comes from the table of 02 through
`perf/internal/report`.

## Port (root package `internal/sync`)

- Inbound: `Tail`, `CatchUp`, `Snapshot`, `Consumers` (worker cursors), `ReadStates`, `Drafts`, `Settings`.
- Emits in-tx events: none.
- Subscribes to in-tx events: `messaging.MessagePosted` (clear the draft), `chat.ThreadMoved` (copies of `channel_id`).
- Implements source ports and cross-cutting ports: kernel `eventlog.Appender`, kernel `eventlog.Redactor`, kernel `idempotency.Store`, kernel `personaldata.Participant`.

## Dependencies

May use the ports of these modules: `instance`, `identity`, `chat`, `agents`, `messaging`, `runs`.

Reason: counter `event_seq`; principal of the snapshot (handle, display name); snapshot from structure, presence, active runs; subscription to messages.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`event_log`, `event_consumer_cursors`, `idempotency_keys`, `read_states`, `drafts`, `user_settings`.

Only this module writes and reads these tables via SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`read_state.update`, `read_state.mark_unread`, `draft.read`, `draft.write`, `settings.read`, `settings.update`, `sync.snapshot.read`, `events.catch_up`.

## Events and WebSocket ops

- Durable: `read_state.updated`, `draft.updated`, `draft.deleted`, `settings.updated`.
- Ephemeral: none.
- WebSocket ops (client to server): none.

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `sync.partition_maintenance` | periodic, create partitions and detach them after `sync.event_retention` | `maintenance` |
| `sync.idempotency_cleanup` | periodic | `maintenance` |

## Notes for the build

- Implements `kernel/eventlog.Appender`: numbers from `event_seq` in phase 3.2 of the Unit of Work, after that `pg_notify` only with the number (02, 8.1).
- The log tail uses its own, non-pooled connection for `LISTEN` (02, 11.2).
- The snapshot contains no thread contents (04, 6.3).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [04-sync-protocol.md](../../../../docs/spec/04-sync-protocol.md), [15-client-architecture.md](../../../../docs/spec/15-client-architecture.md)
