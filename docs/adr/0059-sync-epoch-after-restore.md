---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team F (operations), Team A (architecture), Team B (API)
informed: Team E (apps), Team D (agent platform)
---

# Sync epoch after restore

## Context and Problem Statement

Clients, agent adapters and worker consumers catch up with "everything from number X" (ADR 0009). A
restore from the backup resets the global sequence to the state of the backup.
After the first writes, the server assigns numbers that clients already know from the lost
timeline. A client with cursor 1000 then asks for "everything after 1000" and silently skips new
events with smaller or equal numbers; its cache also contains data that no longer exists on the
server. The check `cursor > head_seq` (R-SY-04) detects this only until the new timeline overtakes
the cursor. How do all receivers detect a new timeline reliably?

## Decision Drivers

- No silent skipping of events, even after the new timeline has overtaken old cursors.
- After a restore, clients discard their server state but keep their
  outbox (04, section 13).
- A relocation without data loss must not trigger a reload of all clients (16, section 11).
- Server-side cursors (agents, webhooks, workers) catch up nothing from the old timeline.
- One mechanism for gateway, REST and webhooks (ADR 0052).

## Considered Options

- Epoch `instance.sync_epoch` (UUIDv7), new after every restore; clients send it with
  the cursor; a mismatch results in `resync_required` with reason `epoch_changed`
- Only the check `cursor > head_seq` (R-SY-04)
- Let the numbers jump after the restore (counter far above any value ever assigned)
- The server remembers the cursor of every device and forces a reload per device

## Decision Outcome

Chosen option: "Epoch `instance.sync_epoch`", because it detects a new timeline independently of
the numbers and changes only on a real restore. Normative in
[03-data-model.md](../spec/03-data-model.md), section 6.7 (R-DM-26),
[04-sync-protocol.md](../spec/04-sync-protocol.md), sections 6.3 and 7.3 to 7.5 (R-SY-18), and
[16-operations.md](../spec/16-operations.md), section 8.4 (R-OP-11).

1. Setup creates the first epoch. After every restore, `bothy restore finalize` sets in one
   transaction: a new epoch, `agent_consumer_cursors.acked_seq`,
   `webhook_subscriptions.last_delivered_seq` and `event_consumer_cursors.last_seq` to the new
   `head_seq`, an audit entry.
2. `hello`, `ready`, `resumed`, the snapshot and `GET /api/v1/events` name the epoch. The
   client stores it with the cursor and sends both on `identify`, `resume` and in the
   REST catch-up.
3. If the epoch does not match, the server responds `resync_required` with reason `epoch_changed`,
   even if `cursor <= head_seq`. The client discards the server state in the cache, keeps the
   outbox, loads the snapshot and resets cursor and epoch.
4. Outgoing webhooks carry the epoch as the delivery field `sync_epoch` in the envelope (additive
   per 04, section 6.1). Reason: webhook receivers order by `seq` ([webhooks.md](../api/webhooks.md),
   section 3) and would otherwise discard smaller numbers of the new timeline as old. A
   receiver that sees a new epoch resets its state.
5. R-SY-04 remains as a second net for clients without a stored epoch.
6. A relocation without data loss keeps the epoch; clients continue with their cursor.

### Consequences

- Good, because no client silently skips events after a restore or keeps showing deleted
  data.
- Good, because the outbox is kept: messages that a client has not yet had confirmed, it sends
  again, and they are created again in the new timeline.
- Good, because agents and webhooks catch up nothing from the old timeline; the cursors are at the
  new head.
- Bad, because every restore makes all clients reload (snapshot, threads on
  demand); that costs load right after the restore.
- Bad, because the epoch only changes when `bothy restore finalize` runs. If an instance operator
  starts `api` before this step, only R-SY-04 applies until the cursor is overtaken. The runbook
  ([backup-restore.md](../operations/backup-restore.md), section 5) puts the step before the
  start; a technical lock is still missing (More Information).
- Bad, because messages that a client already had confirmed and removed from the outbox are
  lost with the old timeline (limit of the RPO, backup-restore.md
  section 7).

### Confirmation

- Build ticket M7-04 in [PLAN.md](../PLAN.md): a client with a cursor from the old timeline gets
  `resync_required` with `epoch_changed`, even if `head_seq` has overtaken the cursor; a relocation
  without a restore does not change the epoch.
- Test: after `bothy restore finalize`, all agent, webhook and worker cursors are at
  `head_seq`, and an audit entry exists; if the command aborts, nothing is changed.
- Test: a webhook delivery after the restore carries the new epoch.
- Restore test (ADR 0058) and portability test (16, section 11.1): the manifest names
  `sync_epoch`.
- Test `TestListEventsResync` (M1-63, `services/server/internal/sync/adapters/httpapi`), case
  `epoch_changed`: `GET /api/v1/events` with an `epoch` other than `instance.sync_epoch` answers
  `410 resync_required` with `reason = epoch_changed`, also when the cursor is above `head_seq`.

## Pros and Cons of the Options

### Epoch

- Good, because it is a single number per timeline, independent of numbers and provider.
- Bad, because clients must store and send another field.

### R-SY-04 only

- Good, because it needs no new field.
- Bad, because the check only works as long as the new timeline has not overtaken the cursor;
  after that the client skips silently.

### Let the numbers jump

- Good, because new numbers would always be larger than old cursors.
- Bad, because after a loss the server does not know the highest number ever assigned; it lay
  in the lost timeline.
- Bad, because clients would still keep data from the lost timeline in the cache; a
  reload is needed anyway.

### Cursor per device in the server

- Good, because the server could trigger reloads selectively.
- Bad, because the server would have to keep a durable state per device for this, which is itself
  outdated after a restore.

## More Information

- Open: a start lock that detects an unfinished restore. A candidate is the
  comparison of `pg_control_system().system_identifier` and `pg_control_checkpoint().timeline_id`
  with values that `bothy restore finalize` stores (functions:
  https://www.postgresql.org/docs/18/functions-info.html, retrieved 2026-10-10 [verified]). Whether
  managed services show these values to the application role and how a failover affects them is
  not checked [assumed]; build ticket M7-04 clarifies it.
- Related: ADR 0009 (event log), ADR 0051 (worker consumers), ADR 0052 (envelope),
  ADR 0058 (backup).
