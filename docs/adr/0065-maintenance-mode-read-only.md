---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team F (operations), Team B (API), Team D (agent platform)
informed: Team E (apps), Team C (identity)
---

# Maintenance mode read_only

## Context and Problem Statement

A move to another target and a major version upgrade of PostgreSQL copy the state of an
instance ([16-operations.md](../spec/16-operations.md), sections 10 and 11;
[cloud-migration.md](../operations/cloud-migration.md)). During the final backup the
old instance must not accept anything that would be missing on the target. People should still be
able to read, and running agent runs should end cleanly. The core contract specifies the layer and
the error ([05-actions.md](../spec/05-actions.md), rule 1.9: `503 maintenance` with `Retry-After`,
reading and catch-up continue). What is still open is the exact meaning: which action counts as
writing, which writes remain allowed, and what prevents an overlooked path from writing
after all?

## Decision Drivers

- After the switch, no state with a business effect is created, except for the defined
  exceptions; otherwise it is missing on the target.
- Reading, catch-up and file retrieval continue (05, rule 1.9).
- Running runs may end, new ones are not created ([PLAN.md](../PLAN.md), build ticket M7-08).
- Clients recognize the state and keep their outbox (`Retry-After`).
- Enforce instead of assume: an overlooked write path fails visibly.
- Every action is classified unambiguously. The column "Permission" from 05 is not enough for this:
  `settings.read` and `settings.update` both have the permission `self`.

## Considered Options

- Configuration value `server.maintenance_mode = read_only`; classification of every action in the
  catalogue, fixed exceptions for finishing runs, token refresh without rotation and deliveries;
  read-only transactions as a second line
- Stop the server entirely
- Set only the database to read-only (`default_transaction_read_only`)
- A switch in the database that the owner flips through the API
- Maintenance per module (for example only messages blocked)

## Decision Outcome

Chosen option: "`server.maintenance_mode = read_only` with classification in the catalogue, fixed
exceptions and read-only transactions", because only it allows reading, lets runs end and catches
every other write attempt in two places.

1. **Switch.** `server.maintenance_mode` (`off` or `read_only`) is configuration of the instance
   operator and takes effect at the start of `api` and `worker` (R-AR-15). Reason against a switch
   in the database: flipping it would itself be a write, the switch would have to exempt itself,
   and maintenance is a step of the instance operator around a copy, not a product switch.
2. **Classification.** Every action in the catalogue (ADR 0049) carries `effect: read` or
   `effect: write`; `write` means: the action can change durable state. This replaces the derivation
   from the column "Permission" in 05, rule 1.9.
3. **Layer.** After the rate limit and before `authorize` (05, section 2.5), the layer answers every
   action with `effect: write` that has no exception with `503` and problem code `maintenance`
   together with `Retry-After`. It writes no audit entry for this (that would be a write and not a
   permission decision); a metric counts the rejections.
4. **Exceptions.**
    - Finishing running runs: for an agent with a run in state `running`,
      `run.update`, `run.finish`, `agent.heartbeat` and `agent.events.consume` (`ack`) remain
      allowed, plus `message.create` and `file.upload` in the thread of this run (the answer).
      `run.cancel` remains allowed for people so that the owner can end hanging runs; the job
      `runs.cancel_timeout` keeps running. `run.start` is blocked: runs in `queued` and
      `waiting_input` wait and survive the copy because their state is in the database.
    - Token refresh: `auth.token.refresh` issues a new access token without rotating the refresh
      token and without writing. Reason: otherwise reading ends after `auth.access_token_ttl`;
      a rotation after the final backup would be missing on the target and would log the person
      out there. Refresh tokens are bound to the device key; without rotation the sender binding
      remains, which RFC 9700 names as an alternative to rotation
      ([08-identity.md](../spec/08-identity.md), section 7.4).
    - Security reactions: the revocation of a refresh family on detected reuse runs
      as always, and the emergency actions `auth.step_up`, `session.revoke`, `device.revoke`,
      `token.revoke` and `user.disable` remain allowed. Reason: a stolen device must be revocable
      in the maintenance window too. If such an action falls after the final backup,
      the owner repeats it on the target.
    - Deliveries: in the `worker`, the consumers `push` and `webhooks` and their jobs keep running
      (ADR 0051), as does `runs.cancel_timeout`. All other queues and periodic jobs
      pause.
    - `gateway.connect` remains allowed; it may keep writing the fencing column of the agent
      consumers in mode `multi`, because that column has no meaning on the target.
5. **Second line.** In mode `read_only` the Unit of Work opens every transaction as `READ ONLY`,
   except for the exceptions from item 4. PostgreSQL then rejects `INSERT`, `UPDATE`, `DELETE` and
   DDL with SQLSTATE `25006` [Q1, verified]; the server answers `503 maintenance`.
   Side writes such as `last_used_at` of tokens and sessions are explicitly dropped in this
   mode.
6. **Display.** The status page and the console report the maintenance, derived from the
   configuration without a write. On `503 maintenance` apps show a notice instead of an error and
   resend their outbox after `Retry-After`.

The mode is not meant for a point-in-time restore: there the database is
replaced, and `api` and `worker` are stopped
([backup-restore.md](../operations/backup-restore.md), section 5.2).

### Consequences

- Good, because people can read, search and retrieve files during the move and runs can still
  deliver their answer.
- Good, because the state after waiting for the runs matches the state on the target; only
  deliveries can still change rows afterwards.
- Good, because an overlooked write path fails at PostgreSQL instead of silently creating data that
  would be missing on the target.
- Bad, because the catalogue gets another attribute and the exceptions for agents depend on the
  run state; for this the layer needs a query to the module `runs`.
- Bad, because switching on and off needs a restart; clients reconnect with `4013`.
- Bad, because deliveries after the final backup can happen a second time on the target
  (push twice, webhook with the same `webhook-id`); delivery is at least once
  anyway.
- Bad, because no new devices can be registered in the window and logging out only has a local effect.
- Bad, because emergency revocations after the final backup are missing on the target; the runbook
  of the move must remind the owner to repeat them there.
- Neutral, because 08 gets an exception from rotation that applies only in this mode.

### Confirmation

- Build ticket M7-08 in [PLAN.md](../PLAN.md): a message results in `503` with `Retry-After`; a
  running run can end; new runs are not created; the status page shows maintenance.
- Table test over the catalogue: every action has `effect`; every action with `effect: write` without
  an exception results in `503 maintenance` in the mode, no action with `effect: read` does.
- Test of the second line: a test handler with an unclassified write fails in the
  mode with `503`, and the database is unchanged.
- Test of the refresh: in the mode `auth.token.refresh` returns a new access token and the same
  refresh token; the rows of the session and the refresh family are the same before and after.
- Test in the `worker`: in the mode only delivery jobs and `runs.cancel_timeout` run.
- Test of the security reactions: in the mode `device.revoke` with step-up takes effect (the device's
  connection closes with `4012`), and reuse of a refresh token revokes the family.
- Rehearsal of the move ([cloud-migration.md](../operations/cloud-migration.md), section 3): the
  manifest of the final backup equals the state at the end of the window, apart from the
  delivery tables.

## Pros and Cons of the Options

### Configuration with classification, exceptions and a second line

- Good, because reading remains possible, runs end and two independent places prevent writes.
- Bad, because it is the most work of these options: catalogue attribute, exceptions, mode of the
  Unit of Work, token path.

### Stop the server entirely

- Good, because it is the simplest and safely without any write.
- Bad, because nobody can read, clients show "Server unreachable" instead of maintenance and
  running runs abort; their work and their costs are lost.

### Only the database in read-only mode

- Good, because PostgreSQL enforces it [Q1, verified].
- Bad, because errors occur only after work in the handler has begun and arrive without
  `Retry-After`; exceptions for runs would still need a classification per action. Therefore only
  as the second line.

### Switch in the database

- Good, because no restart is needed.
- Bad, because flipping it writes and the switch would have to exempt itself; an
  owner switch in the interface invites using it without the copy procedure.

### Maintenance per module

- Good, because it can be controlled more finely.
- Bad, because a copy needs the whole state; partial blocks leave write paths open and
  make the comparison of the manifests impossible.

## More Information

- Source (retrieved 2026-10-10): Q1 PostgreSQL 18, `SET TRANSACTION` (read mode) and error code
  `25006 read_only_sql_transaction`, https://www.postgresql.org/docs/18/sql-set-transaction.html and
  https://www.postgresql.org/docs/18/errcodes-appendix.html [verified].
- Problem code `maintenance`: [06-api.md](../spec/06-api.md), section 5.
- Related: ADR 0015 (error format), ADR 0020 (DPoP), ADR 0049 (action catalogue), ADR 0051
  (worker consumers), ADR 0058 (backup), ADR 0059 (sync epoch).
