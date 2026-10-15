---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team A (architecture), Team G (platform services), Team B (API)
informed: Team F (operations)
---

# Worker consumers of the event log with a cursor instead of job inserts in the write path

## Context and Problem Statement

Push and outgoing webhooks are consequences of durable events that go outside. They must
be triggered exactly once after a commit, even if a process crashes or several
`worker` processes run. The event log is commit-ordered and gapless (ADR 0009). Where are the
jobs for these consequences created: in the writing transaction of every producer or in a
consumer that reads the log?

## Decision Drivers

- Every consequence is created exactly once, even on a crash and with several `worker` processes.
- Producers do not know their consumers; a new consumer changes no producer.
- The serialized commit phase at the counter `event_seq` stays short
  ([02-architecture.md](../spec/02-architecture.md), section 11.7).
- No external calls in transactions (R-AR-07).
- No additional service in operations (B3, ADR 0009).
- Permissions apply at delivery time (B7).

## Considered Options

- A consumer per name with a durable cursor in `event_consumer_cursors`; a round locks the
  row, reads from the cursor, creates jobs and advances the cursor in the same transaction
- Insert jobs in the writing transaction of every producer (River `InsertTx`)
- Processing directly from `NOTIFY` in memory
- A separate cursor per target (per webhook subscription, per push registration), without a
  central consumer
- Logical Decoding or an external broker

## Decision Outcome

Chosen option: "Consumers with a cursor in `event_consumer_cursors`", because cursor and jobs
commit in one transaction and the log is the outbox anyway. Flow according to
[02-architecture.md](../spec/02-architecture.md), section 11.5, table in
[03-data-model.md](../spec/03-data-model.md), section 4.10 (R-DM-27), protocol in
[04-sync-protocol.md](../spec/04-sync-protocol.md), section 7.6:

1. For each consumer name (`push`, `webhooks`) there is one row with `last_seq`. A new
   consumer is an additive migration of the allowed names.
2. A round in the `worker`: open a transaction, lock the row with `FOR UPDATE SKIP LOCKED` (if it
   is locked, the round ends immediately), read up to `consumer.batch_size` events from `last_seq`
   in `seq` order, determine the receivers per event and create River jobs with `InsertTx`,
   advance `last_seq`, commit.
3. Wake-up happens through the same `NOTIFY event_log` as the log tail, and additionally every
   `consumer.poll_interval`.
4. Before sending, every job checks `push.deliver` or `webhook_out.deliver` again with the
   current permissions ([05-actions.md](../spec/05-actions.md), R-AZ-11).
5. Outgoing webhooks additionally have the unique key `(subscription_id, event_seq)` in
   `webhook_deliveries` and per subscription `last_delivered_seq` for order and gap detection
   ([11-extensions.md](../spec/11-extensions.md), sections 6 and 7).
6. After a restore, `bothy restore finalize` sets every cursor to the new head
   (ADR 0059).

The framework is built with the first consumer (webhooks, build ticket M2-01), not before
([PLAN.md](../PLAN.md), P-13).

### Consequences

- Good, because the write path knows no consequence; push, webhooks and later consumers change no
  producer.
- Good, because jobs are created exactly once: cursor and jobs commit together, and a second
  `worker` skips the locked row [Q1, verified].
- Good, because the producer's transaction does not get longer; the work for consequences lies
  outside the serialized commit phase.
- Good, because permissions apply at delivery time: a revocation between commit and sending
  suppresses the delivery.
- Bad, because a consumer name is only ever processed by one `worker` at a time; the
  throughput per consumer depends on the round size. If it is not enough, a new
  decision is needed (for example partitioning by space).
- Bad, because consequences have a delay until the next round; `perf.push_handoff_p95`
  holds it.
- Bad, because a consumer that stands still for longer than `sync.event_retention` loses events
  as soon as their partition is detached. For webhook subscriptions the gap is detected at two
  points (M2-26): at the head of a subscription, when the event of its lowest pending delivery left
  the log, and in the round of the consumer `webhooks`, which reports the range retention removed
  before it read it (`sync.ConsumerLossHandler`); both disable the subscription with cause `gap`
  (11, section 6). For the consumer
  itself the gauges `bothy.event.consumer.lag` and `bothy.event.consumer.pending_age` and the
  alarm `BothyEventConsumerLag` at `consumer.pending_alert_after`, strictly below
  `sync.event_retention`, report the stall long before (M2-01, see More Information).

### Confirmation

- Test from build ticket M2-01: two `worker` processes in parallel, every event creates jobs
  exactly once; a crash in the middle of a round (abort before the commit) creates no duplicates;
  `NOTIFY` and poll both wake up. Built as `TestConsumersTwoWorkersExactlyOnce`,
  `TestConsumerCrashMidRoundNoDuplicates` (terminated backend, handler error, panic, cancelled
  context), `TestConsumerWakesOnNotify`, `TestConsumerWakesOnPoll`,
  `TestConsumerListenLossLosesNothing`, `TestConsumerRoundJobsAndCursorCommitTogether` and
  `TestConsumerRoundLockedEndsAtOnce` in `services/server/internal/sync/adapters/postgres`; the
  lag gauges by `TestConsumerLagGauges`, the alarm by the promtool cases of
  `BothyEventConsumerLag`.
- Test: a permission revocation between round and job prevents the sending. Built in M2-26 as
  `TestDeliveryRevokedRightIsDeadWithoutRequest` and `TestDeliveryScopeRemovedIsDead`; the loss of
  the consumer itself by `TestConsumerLossDisablesSubscriptions` and `TestRoundCallsHandleLost`.
- Load test `perf/push_handoff` with `perf.push_handoff_p95` (build ticket M3-24).
- Restore test: after `bothy restore finalize` every cursor is at `head_seq`
  (ADR 0059).

## Pros and Cons of the Options

### Consumers with a cursor

- Good, because only PostgreSQL and River are needed, which run anyway.
- Bad, because another row per consumer and a round framework must be built.

### Jobs in the producer's transaction

- Good, because River offers transactional inserts [Q2, verified] and no cursor is needed.
- Bad, because every producer must know every consequence; a new consumer changes many
  write paths, and a forgotten place silently loses deliveries.
- Bad, because determining the receivers (notification preferences, subscriptions) would run in
  the writing transaction and lengthen it.

### Processing from `NOTIFY`

- Good, because the delay is minimal.
- Bad, because a notification without a listening connection is lost; at-least-once
  cannot be reached this way. `NOTIFY` stays a wake-up signal without content (R-Stack).

### Cursor per target

- Good, because every target keeps its own order; webhook subscriptions use this in addition.
- Bad, because every target would have to read and filter the whole log; with many push
  registrations the read load multiplies.

### Logical Decoding or a broker

- Bad for the reasons in ADR 0009: replication slots hold on to WAL, a second system
  enters operations and backup.

## More Information

- Sources (retrieved 2026-10-10): Q1 PostgreSQL `SELECT ... FOR UPDATE SKIP LOCKED`,
  https://www.postgresql.org/docs/18/sql-select.html [verified]; Q2 River, transactional inserts,
  https://riverqueue.com/docs [verified].
- Settled by M2-01: the lag per consumer (`head_seq` minus `last_seq`) is the gauge
  `bothy.event.consumer.lag`, and because `sync.event_retention` is a time window the age of the
  oldest unprocessed event is the gauge `bothy.event.consumer.pending_age`
  ([signals.md](../observability/signals.md) 4.2), both exported by every role. The alarm
  `BothyEventConsumerLag` ([alerting.md](../observability/alerting.md) 3.2, severity `page`)
  fires at `consumer.pending_alert_after`, which the start check holds strictly below
  `sync.event_retention` ([DECISIONS.md](../DECISIONS.md) D-M2-01-5 and D-M2-01-6).
- Related: ADR 0009 (event log), ADR 0016 (webhooks), ADR 0031 (push), ADR 0052 (envelope).
