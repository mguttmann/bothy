# Audit log and telemetry

Status: draft M0. Distinction according to B11. Audit log: [03-data-model.md](../spec/03-data-model.md)
4.12, actions with audit: [05-actions.md](../spec/05-actions.md). Security chapter 19 and
ADR 0021 define the content and the checking of the audit log.

## 1. Comparison

| Attribute | Audit log (`audit_events`) | Telemetry (traces, metrics, logs) |
|---|---|---|
| Purpose | Evidence: who decided or changed what | Operations: is it running, where does it get stuck |
| Content | Action, decision, reason, resource, actor (plain ID), device, session | Durations, counters, error types, IDs of objects, pseudonyms of people |
| Completeness | Every event of the mandatory list; denials aggregated per window | Sampled, may be discarded under backpressure |
| Writing | In the same transaction as the change; if the audit fails, the change fails | Asynchronous, fail-open |
| Integrity | Hash chain, only `INSERT` and `SELECT` for the application | None |
| Location | PostgreSQL of the instance | Collector, then a backend of choice |
| Access | Owner with step-up (`audit.read`, `audit.verify`) | Whoever runs the backend |
| Retention | Own class `audit` (chapter 20) | Short, per backend ([privacy.md](privacy.md) 5) |
| Backup | With the database, chain checked in the restore test | None |
| Export | Only by the owner via `exportAuditEvents`; the chain is checked with `verifyAuditChain` (M1-50) | Per overlay |

## 2. Rules

1. No audit entry is created from telemetry, no telemetry from the audit log. The code writes
   both at the same place, separately.
2. Link only through `correlation_id`: it is the trace ID of the request
   ([signals.md](signals.md) 5.1). An auditor can jump from the audit entry to the trace as long as
   the trace is still retained and recorded.
3. Denials by `authorize` appear in both: in the audit log as evidence (aggregated by
   `audit.deny_aggregate_window`), in telemetry as the counter `bothy.authz.decisions` without an
   actor.
4. The collector never receives the content of `audit_events.details`.
5. The audit log contains plain IDs of people, telemetry only pseudonyms. Whoever runs telemetry
   therefore cannot trace who did something; that is what the audit log is for.
6. The check of the hash chain runs periodically in the `worker` (`bothy.audit.verify.*`) and in
   every restore test; a failure is an alert `page`.

## 3. Example: revoke a grant

| Location | What is created |
|---|---|
| `audit_events` | `action = grant.revoke`, `decision = allow`, actor, device, session, `resource_id` of the grant, `correlation_id` |
| `event_log` | `access.changed` to the affected people, if needed `run.cancel_requested`, same `correlation_id` |
| Trace | Span `POST /api/v1/grants/{id}/revoke` with `bothy.action = grant.revoke`, `bothy.decision = allow`, pseudonym of the actor |
| Metric | `bothy.authz.decisions{bothy.action="grant.revoke", bothy.decision="allow"}` plus one |
