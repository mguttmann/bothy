# Modul `sync`

Status: built in part (M1-16, M1-20, M2-01, M1-58, M1-61, M1-63): the event log with the commit-ordered sequence, the phase hook of `sync`, the log tail, the partitions of `event_log`, the one envelope of gateway, REST and webhooks, the idempotency keys behind `kernel/idempotency.Store` with the job `sync.idempotency_cleanup`, and the frame of the worker consumers with `event_consumer_cursors` (no production consumer yet: M2-26 registers `webhooks`, M3-19 `push`); the read states, drafts and personal settings with their twelve REST operations and the subscription `messaging.MessagePosted` (M1-61). The redaction of a deletion (M1-58) implements `eventlog.Redactor`. Catch-up and snapshot (M1-63): the ports `CatchUp` and `Snapshot` with `GET /api/v1/events` and `GET /api/v1/sync/snapshot`, which the gateway reuses (M1-64, M1-65).
Normativ ist [docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Ereignis-Log (Anhängen, Tail, Nachholen, Schwärzen, Partitionen), Momentaufnahme, Idempotenz-Schlüssel, Gelesen-Stand, Entwürfe, persönliche Einstellungen, Cursor der Worker-Verbraucher.

## Code

| Package | Content |
|---|---|
| `internal/sync` | port: `Event` (a stored row, zero values for NULL), `DeliveryFields`, `Envelope` with `EnvelopeActor`, `EnvelopeScope`, `EnvelopeResource` and `Event.Envelope`, `Sink`, `Tail`, `ErrGap`; worker consumers: `ConsumerName` (`ConsumerPush`, `ConsumerWebhooks`, `ConsumerNames`, `Valid`), `ConsumerHandler`, `ConsumerLossHandler`, `ConsumerState`, `Consumers`, `LogReader`; personal data (M1-61, `personal.go`): `Register` (the four personal events), `ReadStates`, `Drafts`, `Settings` with the DTOs `ReadState`, `AdvanceReadState`, `MarkUnread`, `Precondition`, `Draft`, `PutDraft`, `DraftPutResult`, `UserSetting`, `SidebarState`; catch-up and snapshot (M1-63, `catchup.go`): `CatchUp` with `CheckInput`, `CheckResult`, `ResyncReason`, `PageInput`, `PageResult`, `EnvelopeJSON`, and `Snapshot` with `SnapshotView` and its parts `SnapshotPrincipal`, `SnapshotSpace`, `SnapshotChannel`, `UnreadThread`, `RunSummary`, `Money`, `AgentPresence`, `DraftRef`, `EvaluatedFlag` |
| `domain/` | event validation (`ValidateEvent`, `CheckPayload`), partition arithmetic (`PartitionLower`, `PartitionName`), the gap check of the tail (`CheckContiguous`, `GapError`) and the round plan of a worker consumer (`PlanRound`, `RoundPlan`, `ErrConsumerGap`); personal data (M1-61): `CheckReadSeq`, `Advance`, `MarkUnread` (`readstate.go`), `DraftInput` with `Empty`, `ValidateDraft`, `OverwroteNewer`, `TruncateContent` (`draft.go`) and the key registry `SettingSpec`, `Settings`, `KnownSetting`, `ValidateSetting` (`settings.go`); catch-up (M1-63): `ResyncReason` with `Reason` and `NextCursor` (`catchup.go`) |
| `app/` | `Appender` (implements `kernel/eventlog.Appender`), `PhaseHook` (phase `sync`), `PartitionMaintenance`, `IdempotencyStore` (implements `kernel/idempotency.Store`), `IdempotencyCleanup`, `ConsumerService` (implements `sync.Consumers`: `Register`, `Ensure`, `Round`, `States`) and `ConsumerRunner`; outgoing ports `Writer`, `Partitions`, `Correlator`, `KeyRows`, `ConsumerCursorStore`, `ConsumerLogReader`, `Waker`; personal data (M1-61): `Personal` (implements `sync.ReadStates`; `Personal.Drafts` and `Personal.Settings` the other two ports), `PersonalConfig`, `PersonalDeps`, outgoing ports `PersonalStore`, `DraftRows`, `ThreadSource`, `ChannelSource`, `MentionCounter`, and `Subscribers` with `OnMessagePosted`; catch-up and snapshot (M1-63): `CatchUpService` (`NewCatchUp`, `CatchUpConfig`, `CatchUpDeps`) and `SnapshotService` (`NewSnapshot`, `SnapshotDeps`), outgoing ports `CatchUpLog` with `VisibleQuery`, `HeadSource`, `EpochSource`, `SnapshotStructure`, `ThreadCounters`, `SnapshotStore`, `AgentLister`, `PrincipalDirectory`, `FlagEvaluator` and `RunSummarySource` |
| `adapters/postgres/` | `Repository` (writer, partitions, head, minimum, reads, the cursor store and log reader of the worker consumers) and `IdempotencyRepository` (`KeyRows`) over sqlc queries in `queries/`, generated code in `sqlcgen/` (entry in `services/server/sqlc.yaml`); `Tail` and `Waker`, each with its own `LISTEN` connection over one shared listening loop (`listener`); `PersonalRepository` (`PersonalStore`) over `queries/read_states.sql`, `drafts.sql` and `user_settings.sql` (M1-61), also `SnapshotStore` (M1-63); the page query of catch-up `queries/catchup.sql` (`Repository.VisibleEvents`, M1-63) |
| `adapters/httpapi/` | `Personal`, the handlers of `listMyReadStates`, `putReadState`, `markThreadUnread`, `putThreadDraft`, `deleteThreadDraft`, `putChannelDraft`, `deleteChannelDraft`, `listMyDrafts`, `listMySettings`, `putMySetting`, `deleteMySetting` and `getMySidebar` (M1-61); `Sync`, the handlers of `listEvents` and `getSnapshot`, and `SnapshotREST` (M1-63) |
| `adapters/noruns/` | `Source`, the M1 binding of `app.RunSummarySource`: no running run, because the table `runs` exists only from M2-05, which binds the runs port instead (M1-63) |
| `adapters/otelmetrics/` | the lag gauges of the worker consumers (`RegisterConsumerGauges`); `New`, the implementation of `app.Metrics` (`bothy.event.appended`, `bothy.db.event_seq.lock_wait`, M1-68) |
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
than 131072 digits before or 16383 after the point (the limits of numeric), the
character U+0000 or an unpaired surrogate escape (jsonb refuses both). The reads hand the sinks
the compact form of the stored jsonb, which is never larger than the appended payload. A producer
with a larger object builds a reduced payload and sets `Truncated` (04 payload rule 2). Events
without ID get a UUIDv7; the correlation of the append context is attached: the trace ID of a
valid span as `correlation_id` (without a span the correlation ID of the request in
`kernel/reqmeta`), and the W3C `traceparent` as `trace_context` only for a sampled span.

The phase hook of `sync` (02 "Ablauf" step 3.2) does nothing for a unit without events, so such a
unit takes neither the counter lock nor `NOTIFY` (02 "Kosten der Serialisierung"). Otherwise it
advances `event_seq` by n through `instance.Counters` (the lock holds until the commit), reads
`permission_version` after the lock, and inserts the rows in append order with
`seq = last - n + 1 .. last` together with `pg_notify('event_log', last)` in one statement
(`InsertEvents`, a data-modifying CTE), which keeps the lock one round trip shorter.
`occurred_at` is the column default `now()`, the transaction time. An insert that no partition
takes (SQLSTATE 23514) is kind `unavailable`; the error line names the first seq, never a
payload.

### Metrics of the phase hook (M1-68)

The phase hook measures the statement that locks and advances the `event_seq` row
(`bothy.db.event_seq.lock_wait`) and, once the unit of work committed (`txn.AfterCommit`), counts
its events per durable type (`bothy.event.appended`); a rolled-back unit counts nothing. It records
through the port `app.Metrics` (`PhaseHook.WithMetrics`), implemented by `adapters/otelmetrics.New`.

### Log tail

One `Tail` per api process (roles `api` and `all`, start step 8 before the public listener;
02 "Log-Tail je api-Prozess"): `Run` sets `local_head` to `max(seq)`, holds its own connection
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

The port `Consumers` (02 "Ports, Ereignisse und Quellen je Modul", "Worker-Cursor") runs the
consumers of the event log in the worker (02 "Worker-Verbraucher", ADR 0051). They are not the
agent consumers of the module `agents` (`agent_consumer_cursors`, M2-07), which that port also
calls `Consumers`: worker consumers are internal, have no API, consuming is no action and writes
no audit entry.

A handler that also implements `ConsumerLossHandler` (M2-26, the consumer `webhooks`) learns the
range retention removed before the round read it: `HandleLost(from, to)` runs once in the round's
transaction before the first `HandleEvent`, and its error rolls the round back like a handler
error. `LogReader` (implemented by `adapters/postgres.Repository`) reads one event by `seq`
(`EventBySeq`, found false once retention removed it) and the smallest retained `seq` for the
delivery jobs. `DeliveryFields.SyncEpoch` sets the member `sync_epoch` of the envelope, which only
webhook deliveries carry.

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
  the attribute `bothy.consumer.name` (`adapters/otelmetrics`, declared in the metric registry of
  `platform/otel/metrics` since M1-68); the alarm `BothyEventConsumerLag` fires at `consumer.pending_alert_after`.
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
`bothy_app` has `SELECT, INSERT` on the table, the column right `UPDATE (payload, redacted_at)`
of the redaction (migration `00037_sync_event_log_redaction.sql`, M1-58) and no right on the
partitions, which it reaches only through `event_log`.

### Redaction (M1-58)

`adapters/postgres.Redactor` implements the kernel port `eventlog.Redactor` (`wire.Sync.Redactor`
gives it to messaging): inside the caller's unit of work it locks the stored events of one
resource (`resource_kind`, `resource_id`; index `event_log_resource_idx`) `FOR UPDATE` in `seq`
order, calls the `RedactFunc` of the caller per event with type, schema version and payload, and
writes the results in one `UPDATE ... FROM unnest` that sets `payload` and `redacted_at` only if
it was unset. A result that is not a JSON object or an error of the function fails the unit. The
envelope maps `redacted_at IS NOT NULL` to `"redacted": true` (`TestEnvelope_RedactedFlag`).
Events still buffered in the unit of work are not stored yet and are not redacted; messaging
appends `message.deleted` after the redaction.

Partitions change only through two `SECURITY DEFINER` functions of `bothy_owner` with
`search_path = public, pg_temp`, `EXECUTE` revoked from `PUBLIC` and granted to `bothy_app`, which
take only numbers and times and build every name with `format('%I')` (16 "Rollen", R-OP-17):

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
  "Partition Maintenance"). `CREATE TABLE ... PARTITION OF` would take `ACCESS EXCLUSIVE`
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
with `RunOnStart` (02 "Job-Katalog"), and `bothy migrate up` runs the same use case once after
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
come from `kernel/clock` through the request of the stage (03 Grundsatz 2).

`IdempotencyStore` is the kernel port `idempotency.Store` the Idempotency-Key stage of
`platform/httpserver` uses (02 "Idempotenz im Unit of Work"):

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

### Read states, drafts and settings (M1-61)

Migration `00042_sync_personal_data.sql` creates `read_states`, `drafts` and `user_settings` as
03 section 4.10 (`bothy_app`: `SELECT`, `INSERT`, `UPDATE` on `read_states`, also `DELETE` on
the other two). Every query names the person (`user_id`). Every operation authorizes its action
of the catalogue on a resource owned by the caller; agents and apps get
`403 subject_kind_not_allowed` from `authorize` (docs/DECISIONS.md M1-61/3), an unknown and an
invisible thread or channel the same `404`.

- **Read state.** `putReadState` (`read_state.update` on the thread): `0 <= last_read_seq <=
  threads.last_message_seq` (else `400` on `/last_read_seq`, `out_of_range`) read under
  `FOR SHARE` on the thread (`chat.Threads.InfoForShare`), then one upsert with GREATEST; no change
  answers the current state (a thread without a row: position 0, version 0) without event.
  `markThreadUnread` (`read_state.mark_unread`): `If-Match` required (`428`), the version of the
  locked row (0 without one) or `412 version_conflict` with `current`, a value at most the
  current one (else `400`), equal answers without event; the answer has the `ETag`.
  `listMyReadStates` (`settings.read` on the caller): the rows of a channel in the visible set of
  `thread.read`, each checked with `thread.read` in filter mode; a foreign and an unknown channel
  give the same empty list. `mention_count` comes from `messaging.Mentions.CountUnread` above
  `last_read_seq` when the answer or event is built and is never stored (R-DM-32, M1-61/5); a new
  mention writes nothing.
- **Drafts.** `draft.write` on the thread or on the channel (only a forum, else `400`), so
  read-only and archived channels save drafts. Form only: `content_md` at most
  `message.max_content_bytes` bytes (`413 payload_too_large` like sending) and without U+0000,
  `file_ids` UUIDv7, without duplicates and at most `message.max_attachments`, a reply target of
  the same thread (`messaging.ReplyTargets`, `not_in_thread`), none on a channel draft. A write is
  one upsert on the partial unique index, so concurrent first writes do not fail; a returned
  version 1 is new, a larger one replaced version - 1, and `overwrote_newer = existed AND
  (base_version absent OR base_version < version - 1)`. An empty draft (whitespace only, no
  files) is deleted like `DELETE` and answers the empty final state with the last version plus
  one. `listMyDrafts` (`draft.read`): the drafts in the visible channels, each checked with
  `thread.read` or `channel.read` in filter mode; drafts of lost channels stay stored.
- **clear_draft.** `Subscribers.OnMessagePosted` runs in the message transaction: with
  `ClearDraft` it deletes the author's draft of the thread, or for the first message of a new
  thread the channel draft of the forum (M1-61/4), and appends `draft.deleted`; a rollback keeps
  the draft. It writes no read state.
- **Settings.** The registry of `domain/settings.go` holds exactly the keys of 01 section 11 and
  13 section 3.3 (`TestSettingRegistryMatchesSpec` reads both tables). Unknown key `404`, invalid
  value `400` on `/value`; the stored value (jsonb equality) answers without event and version;
  delete appends `settings.updated` with the last value, version + 1 and `deleted`, and a later PUT
  starts at version 1 (M1-61/1). Defaults are not stored. `getMySidebar` (`settings.read`) returns
  `chat.Structure.ListSidebarStates`.
- **Events.** Durable, audience and actor the person, receive action `self`, schema version 1:
  `read_state.updated` (resource the thread without version, scope space, channel, thread,
  `ReadStatePayload`), `draft.updated` (resource the draft with its version, scope with the thread
  of a thread draft, `DraftPayload`; a payload not below `sync.event_payload_max_bytes` gets the
  longest prefix of `content_md` that fits and `truncated`), `draft.deleted` (resource the draft
  without version as the catalogue records, M1-61/2, `DraftDeletedPayload`), `settings.updated`
  (resource the user without version, empty scope, `SettingsPayload`). Payloads carry
  `updated_by_device_id` of the writing device and no JSON null.

Thread moves (M1-62): `Subscribers.OnThreadMoved` is the in-transaction subscriber of
`chat.ThreadMoved` (after the one of messaging); it runs `MoveThreadReadStates` and
`MoveThreadDrafts` (`queries/threads_move.sql`) over the rows of every person for the thread,
without a version and without an event, through the port `app.ThreadMoveRows` of
`PersonalRepository`. A concurrent `putReadState` or `putThreadDraft` holds the thread `FOR
SHARE` (`Threads.InfoForShare`), so it either committed before the move's update or writes the
new channel (`TestMoveThreadConcurrentReadState` in `test/integration/move_test.go`).

Rulings: docs/DECISIONS.md M1-61. Tests: `domain/*_test.go`, `app/subscribers_test.go` and
`app/personal_test.go`, `adapters/postgres/personal_test.go`,
`adapters/httpapi/personal_test.go` (PostgreSQL 18, the HTTP chain with response validation
through `test/messagingworld` with `NewPersonal`) and
`migrations/sync_personal_data_test.go`.

### Catch-up and snapshot (M1-63)

- **Head.** `head_seq` is the committed value of `event_seq`, read without a lock through
  `instance.Versions.EventSeq`, never `max(seq)`: after retention the log can be empty while the
  counter is not. A commit is visible before its counter lock is released, so every `seq <=
  head_seq` is visible (04 section 3.2). `min_retained_seq` is `min(seq)`, `head_seq + 1` for an
  empty log; `sync_epoch` comes from `instance.State` (docs/DECISIONS.md M1-63/1).
- **Check.** `CatchUp.Check` authorizes `events.catch_up` on the instance (an app needs
  `events:subscribe`) and decides in this order, first match wins: an `epoch` that differs from
  `sync_epoch` gives `epoch_changed`; a cursor above the head `cursor_ahead`; a cursor below
  `min_retained_seq - 1` `retention`; more than `sync.catchup_max_events` behind
  `too_far_behind` (`domain.ResyncReason`, M1-63/2). Without `epoch` there is no epoch check.
- **Page.** `CatchUp.Page` takes the visible sets of `Authorizer.Scope` (`channel.read`), runs the
  predicate of 04 section 7.2 point 1 with `seq > after AND seq <= head ORDER BY seq LIMIT limit`
  (`queries/catchup.sql`), then per row: a personal event only to its audience, any other only
  when `authorize(actor, receive_action, authorize.ResourceFromEvent(...))` allows it in filter
  mode (no denial audit); authz decides an event whose stored kind the receive action does not take on the object it takes (M1-63/12). It reads no content table per event. `next_cursor` is the `seq` of the
  last row of a full page, otherwise the head; rows dropped by the audience rule or `authorize`
  count as checked (R-SY-03). The envelope is `Event.Envelope`; `trace_context` only for agents.
  A limit of 0 or above `sync.catchup_page_size` is that key.
- **`listEvents`.** `Check`, then `410 resync_required` with `reason`, or one page up to the head
  of `Check`: `200 EventPage {events, next_cursor, head_seq, sync_epoch}`, `next_cursor` also on
  an empty page.
- **Snapshot.** `Snapshot.Build` authorizes `sync.snapshot.read` (a pending device gets `403
  device_pending`), reads `head_seq` and `sync_epoch` first and every other part after them (READ
  COMMITTED), so every change up to `head_seq` is contained and later ones arrive by catch-up:
  `permission_version` from `Authorizer.Scope`; `principal` and `is_owner` from
  `identity.Directory`; `spaces` and `channels` through the chat listings of `listSpaces` and
  `listSpaceChannels` (visible, not archived); `sidebar`, `settings` and `drafts` through the
  personal data ports of M1-61; `unread_threads` from `chat.Threads.Counters` of the listed
  channels and the person's `read_states` there, combined in Go: unread =
  `last_message_seq - GREATEST(last_read_seq, unread_floor_seq)`, the mentions from
  `messaging.Mentions.CountUnread` above the same position, each listed thread checked with
  `thread.read` in filter mode (M1-63/4); `active_runs` from `RunSummarySource` (in M1
  `adapters/noruns`, empty); `agents` from the listing behind `listAgents`, each `state: unknown`
  without `reported_billing_mode` until heartbeats exist (M2-20, M1-63/3); `features` from
  `instance.Flags`. Agents and apps get empty `sidebar`, `unread_threads`, `settings` and
  `drafts`. The snapshot carries no thread content.
- **Configuration.** `sync.catchup_page_size`, `sync.catchup_max_events` (04).

Rulings: docs/DECISIONS.md M1-63. Tests: `domain/catchup_test.go`, `app/catchup_test.go`,
`adapters/noruns`, `adapters/httpapi/sync_test.go` and `snapshot_test.go` (PostgreSQL 18, the HTTP
chain with response validation through `test/messagingworld.NewSync`, including AZ-P5 for the
snapshot), `internal/authz/app/eventresource_test.go` (`authorize.ResourceFromEvent`).

### Load test

`perf/event_commit` (`make perf`, build tag `perf`) measures `perf.event_commit_min_per_s` on a
database from `pgtest.Durable()` (`fsync` and `synchronous_commit` on, ADR 0063 point 2) and fails
below the value of 02; the threshold comes from the table of 02 through
`perf/internal/report`.

## Stored settings of many people (M3-18)

- Port: `Settings.ValuesFor(ctx, userIDs, keys)` returns the stored values of the keys for many people in one query (`SettingValues`), without defaults and without `authorize`: `notify` reads `notify.default_level` of every candidate of a thread (M3-18), and M3-19 reads its switches the same way.

## Port (Wurzelpaket `internal/sync`)

- Eingehend: `Tail`, `CatchUp`, `Snapshot`, `Consumers` (Worker-Cursor), `ReadStates`, `Drafts`, `Settings`.
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: `messaging.MessagePosted` (Entwurf leeren), `chat.ThreadMoved` (Kopien von `channel_id`).
- Implementiert Quellen-Ports und Querschnitts-Ports: kernel `eventlog.Appender`, kernel `eventlog.Redactor`, kernel `idempotency.Store`, kernel `personaldata.Participant`.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `instance`, `identity`, `chat`, `agents`, `messaging`, `runs`.

Grund: Zähler `event_seq`; Prinzipal der Momentaufnahme (Handle, Anzeigename); Momentaufnahme aus Struktur, Präsenz, laufenden Läufen; Abo auf Nachrichten.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`event_log`, `event_consumer_cursors`, `idempotency_keys`, `read_states`, `drafts`, `user_settings`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`read_state.update`, `read_state.mark_unread`, `draft.read`, `draft.write`, `settings.read`, `settings.update`, `sync.snapshot.read`, `events.catch_up`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `read_state.updated`, `draft.updated`, `draft.deleted`, `settings.updated`.
- Kurzlebig: keine.
- WebSocket-Ops (Client an Server): keine.

## Hintergrundjobs

| Job | Auslöser | Warteschlange |
|---|---|---|
| `sync.partition_maintenance` | periodisch, Partitionen anlegen und nach `sync.event_retention` abhängen | `maintenance` |
| `sync.idempotency_cleanup` | periodisch | `maintenance` |

## Hinweise für den Bau

- Implementiert `kernel/eventlog.Appender`: Nummern aus `event_seq` in Phase 3.2 der Unit of Work, danach `pg_notify` nur mit der Nummer (02, 8.1).
- Der Log-Tail nutzt eine eigene, nicht gepoolte Verbindung für `LISTEN` (02, 11.2).
- Die Momentaufnahme enthält keine Thread-Inhalte (04, 6.3).

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [04-sync-protokoll.md](../../../../docs/spec/04-sync-protokoll.md), [15-client-architektur.md](../../../../docs/spec/15-client-architektur.md)
