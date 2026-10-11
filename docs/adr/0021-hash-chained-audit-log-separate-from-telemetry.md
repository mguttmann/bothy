---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
---

# Hash-chained audit log, separate from telemetry

## Context and Problem Statement

Security-relevant decisions must be provable: permission changes, step-up, administrative
actions, denials, agent invocations with backend policy, confirmations of approval cards.
Telemetry serves operations, has other recipients and other retention periods, may drop data
(fail-open) and is often stored at external providers (Baseline B11). Evidence, by contrast,
must be complete and must make changes detectable.

Where and how is the audit log kept so that it is complete, verifiable and data-minimizing?

## Decision Drivers

- Completeness: no entry must be lost silently.
- Detectability of changes and deletions.
- Separate from telemetry: own retention period, own permissions, no external recipient needed.
- Data protection: no content, pseudonymization without breaking verifiability.
- Verifiable by the owner without special tools (`audit.verify`).

## Considered Options

1. Own table in PostgreSQL, append-only, hash chain over a canonical form of every row,
   signed checkpoints that are stored outside the database.
2. Audit as part of telemetry or the logs (OpenTelemetry logs in the backend of the instance
   operator).
3. Append-only table without a hash chain, protected only by database permissions.
4. External immutable target as the primary store (object storage with a retention lock or a
   transparency log).

## Decision Outcome

Chosen option: 1, because the entry is created in the same transaction as the change
(completeness), the chain makes every change to a row detectable, and the signed, externally
stored checkpoints also make a complete rewrite of the chain detectable, without the instance
depending on an external service.

Rulings:

- Table `audit_events` from [../spec/03-data-model.md](../spec/03-data-model.md): `seq` from the
  counter row `audit_seq` in the same transaction; `prev_hash` and `hash =
  SHA-256(prev_hash || canonical_json(row without hash))`.
- `canonical_json` is the JSON Canonicalization Scheme according to RFC 8785
  (https://www.rfc-editor.org/info/rfc8785, retrieved 2026-10-10 [verified]); with it, the server
  and the verification tool compute byte-identical results.
- Permissions: The application role may only `INSERT` and `SELECT`; `UPDATE` and `DELETE` are
  blocked by `REVOKE` and a trigger. Truncation at the end of the retention period only through a
  maintenance role with an anchor entry (03).
- Checkpoints: Every `audit.checkpoint_interval` the worker signs `{instance_id, seq, hash,
  occurred_at, previous_checkpoint_hash}` with a dedicated server key (purpose
  `audit_checkpoint`, 03 `server_signing_keys`) and stores it in the database (table
  `audit_checkpoints`, 03) and at the target `audit.checkpoint_sink`. The owner chooses a target
  outside the reach of the database administration (for example object storage with a retention
  lock in a different account).
- `audit.verify` recomputes the chain from the last anchor and compares it with the checkpoints
  from the target, not from the database.
- Content: only IDs, action, decision, reason code, resource, `details` according to the schema
  `audit_details.v1` ([../spec/19-security.md](../spec/19-security.md), SEC-AUD-5); never content,
  names, emails or secrets. The only free text is the mandatory reason of an administrative action
  (`reason`, for example for I4), because the action requires it and it is part of traceability.
  IP addresses only as an HMAC with a destroyable key per period
  ([../spec/20-privacy.md](../spec/20-privacy.md), section 4.5).
- Telemetry receives at most counters (for example denials per action), never audit rows.
- Identical denials of the same principal are aggregated per `audit.deny_aggregate_window`
  (R-AZ-04).

### Consequences

- Good, because no audit entry can be missing without the change itself being missing.
- Good, because pseudonymization (changing the principal row, destroying the IP key) changes no
  audit row and the chain stays valid.
- Good, because the instance works without an external service; the checkpoint target is
  replaceable.
- Bad, because the counter row `audit_seq` serializes all audited transactions at the end
  (like `event_seq`); a load test is needed.
- Bad, because whoever controls both the database and the checkpoint target can forge both.
  This remains a residual risk (T-DB-01 in the threat model).

### Confirmation

- Test: a change to an audit row with the maintenance role is detected by `audit.verify`.
- Test: a rewrite of the whole chain is detected by the comparison with the checkpoint.
- Test: the application role cannot `UPDATE` or `DELETE` (permission test against the migration).
- Test: truncation with an anchor, then `audit.verify` succeeds.
- Test: pseudonymization of an account, then `audit.verify` succeeds (20, DP-T02).
- Canary test: no content and no secrets in `details` (19, SEC-T10).

## Pros and Cons of the Options

### 1. Table with a hash chain and externally stored checkpoints

- Good, because transactionally complete and verifiable.
- Good, because it runs without an external service.
- Neutral, because an additional key and a storage target must be operated.
- Bad, because of serialization at the counter.

### 2. Audit in telemetry or logs

- Good, because the infrastructure exists anyway.
- Bad, because telemetry may and must drop data (fail-open), audit must not.
- Bad, because external telemetry providers would then hold evidence with personal data, and
  retention periods would not match.
- Bad, because changes are not detectable.

### 3. Table without a chain

- Good, because simplest.
- Bad, because any person with database permissions can change or delete entries unnoticed.

### 4. External immutable target as the primary store

- Good, because changes are technically impossible.
- Bad, because every audited transaction would depend on an external service (outage, latency)
  or entries would be written with a delay and thus not transactionally.
- Neutral: exactly right as the storage target of the checkpoints; that is part of option 1.

## More Information

- Data model: [../spec/03-data-model.md](../spec/03-data-model.md), `audit_events`.
- Actions that require an audit entry: [../spec/05-actions.md](../spec/05-actions.md), column "Audit".
- Controls and configuration values of the checkpoints: [../spec/19-security.md](../spec/19-security.md),
  section 12.
- Core contract: `server_signing_keys.purpose` with `audit_checkpoint` and the table
  `audit_checkpoints` are in [../spec/03-data-model.md](../spec/03-data-model.md),
  section 4.12.
