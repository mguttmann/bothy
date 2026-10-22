# Alerts

Status: draft M0. Rules: [infra/observability/prometheus/rules/](../../infra/observability/prometheus/rules/)
(SLO rules generated, `bothy-alerts.yaml` by hand). Routes:
[infra/observability/alertmanager/alertmanager.yaml](../../infra/observability/alertmanager/alertmanager.yaml).

## 1. Levels and routes

| Level | Meaning | Route |
|---|---|---|
| `page` | People feel it now or soon; act immediately | External channel (`external-oncall`) and alert channel of the instance (`bothy-ops`) |
| `ticket` | Act on one of the next working days | Alert channel of the instance |
| `heartbeat` | Watchdog; always fires | External heartbeat service |

Why two routes: A failure of the server must not swallow its own alert. The
watchdog reports a failure of Prometheus or Alertmanager: if its heartbeat stops arriving at the
external service, that service reports it. The instance operator chooses this service.

Routing checked locally with `amtool config routes test` (Alertmanager v0.34.1) on 2026-10-10: `page`
goes to `external-oncall` and `bothy-ops`, `ticket` to `bothy-ops`, `Watchdog` to
`external-heartbeat` [verified].

## 2. Alert channel in the instance

1. The owner creates a channel "Operations" (only owner and deputy with `read`) and in it an
   incoming webhook ([05-actions.md](../spec/05-actions.md) `webhook_in.create`).
2. Alertmanager sends its own unchanged JSON payload. The server recognises the format per
   request from the body (format `alertmanager`, [webhooks.md](../api/webhooks.md) section 8),
   so the URL that `createIncomingWebhook` returns serves both `native` and `alertmanager`; no
   format is chosen when the webhook is created. Each notification becomes one message with
   alert name, severity, summary, runbook link and the state firing or resolved. Alertmanager in
   the stack uses the same path `/api/v1/hooks/{webhook_id}/{token}` on
   `server.internal_listen_addr` (point 3).
3. The URL of the webhook is a secret and is stored in
   `secrets/alertmanager/alertmanager_bothy_webhook_url`. Alertmanager reaches the server through
   the internal listener (`http://api:8081`).
4. Messages from incoming webhooks never trigger agents (`origin = app`).

## 3. Alerts

### 3.1 SLO alerts (generated)

| Alert | SLO | Levels | First steps |
|---|---|---|---|
| `BothyApiAvailabilityBudgetBurn` | `bothy-api/requests-availability` | page, ticket | Dashboard Server: routes with 5xx; logs `ERROR`; database reachable? |
| `BothyDeliveryLatencyBudgetBurn` | `bothy-sync/delivery-latency` | page, ticket | Dashboard Delivery: fan-out lag, waiting at the sequence counter, CPU |
| `BothyReconnectBudgetBurn` | `bothy-sync/reconnect` | page, ticket | Edge (proxy, Tailscale) and certificate; login (OIDC reachable?) |
| `BothyPushAcceptanceBudgetBurn` | `bothy-push/provider-acceptance` | page, ticket | Push dashboard: result per provider; key expired? Status page of the provider |
| `BothyRunStartBudgetBurn` | `bothy-agents/run-start` | page, ticket | Agents dashboard: heartbeat, consumer backlog, rejections, mode mismatches |
| `BothyTurnDurationBudgetBurn` | `bothy-agents/turn-duration` | ticket | Turn duration per agent; model or provider slow? No intervention on the server needed |

### 3.2 Further alerts (`bothy-alerts.yaml`)

Thresholds are starting values. Where a threshold corresponds to a configuration value, the table
names it; the rule file must follow it. `make ops-check` holds this (check `alert-thresholds`):
if the column says `` `<name>` ``, the threshold of the last comparison `> <number>` in seconds
(a plain number; comments do not count) equals the default of the value in the [configuration reference](../operations/configuration.md);
if it says ``above `<name>` ``, it is strictly greater. The alerts of the table are exactly the
alerts of `bothy-alerts.yaml`.

| Alert | Level | Condition | Configuration value |
|---|---|---|---|
| `Watchdog` | heartbeat | always | |
| `BothyServerTelemetryMissing` | page | no `target_info` from the server for 10 minutes | |
| `BothyCollectorDown` | page | collector does not respond | |
| `BothyTelemetryExportFailing` | ticket | exporter of the collector fails | |
| `BothyTelemetryDropped` | ticket | server discards telemetry | |
| `BothyEventFanoutLag` | page | fan-out lag over 5 s | `status.fanout_lag_degraded` |
| `BothyEventConsumerLag` | page | the oldest unprocessed event of a worker consumer is older than `consumer.pending_alert_after` | `consumer.pending_alert_after` |
| `BothyAgentConsumerLag` | ticket | consumer backlog of over 1000 events | |
| `BothyAgentOffline` | ticket | no heartbeat for 5 minutes | above `agent.presence_timeout` |
| `BothyBillingModeMismatch` | page | reported mode differs from the configured mode, according to the heartbeat for longer than the threshold or at a run (I3) | `billing.mismatch_alert_after` |
| `BothyAuthzError` | page | permission check reports a programming error (`authz_not_checked`, `unknown_action`) or a difference between the SQL filter and `authorize` (`filter_mismatch`) | |
| `BothySecretRefreshFailing` | ticket | re-reading a secret fails | |
| `BothyMalwareScanFailing` | ticket | only errors, no result | |
| `BothyBackupStale` | page | no backup for 26 h | `backup.max_age` |
| `BothyWalArchiveStale` | page | no WAL segment for 15 minutes | above `backup.pg.archive_timeout` |
| `BothyRestoreTestStale` | ticket | no restore test for 8 days | `backup.restore_test.max_age` |
| `BothyAuditChainBroken` | page | check of the hash chain fails | |
| `BothyVolumeFull` | page | under 10 percent free | |
| `BothyDatabaseConnectionsHigh` | ticket | over 80 percent of `max_connections` | |
| `BothyDatabaseDeadlocks` | ticket | deadlock reported | Lock order [03-data-model.md](../spec/03-data-model.md) 6.3 |

## 4. Runbook anchors

Every rule names a `runbook_url` that points to an anchor of this file (alert name in lowercase).
The first steps are in 3.1; for 3.2:

### watchdog
No action needed. If the heartbeat is missing at the external service: check Prometheus and Alertmanager.

### bothyservertelemetrymissing
`docker compose ps`; is `api` running? If yes: check the collector and `OTEL_EXPORTER_OTLP_ENDPOINT`.

### bothycollectordown
`docker compose logs otel-collector`; often a missing secret of an overlay.

### bothytelemetryexportfailing
Target of the overlay reachable? Key valid? Until this is resolved, data of this backend is
lost; other overlays keep running.

### bothytelemetrydropped
Collector overloaded or not reachable; check `memory_limiter` and the exporters.

### bothyeventfanoutlag
Database load (dashboard Database), long transactions, `pg_stat_activity`. An `api` process
hangs: restart it.

### bothyeventconsumerlag
A worker consumer (`push`, `webhooks`; label `bothy_consumer_name`) stands still: pushes or
outgoing webhooks no longer go out. Is a process of the role `worker` or `all` running? Read the
lines `event consumer round failed` (with `error.type`) and `event consumer lost events to
retention`; is the database reachable? A handler that keeps failing stalls only its own consumer,
which catches up by itself after the fix. Never move the cursor in `event_consumer_cursors` by
hand: the events it jumps over are lost for that consumer for good. A consumer that stands longer
than `sync.event_retention` loses events ([02-architecture.md](../spec/02-architecture.md) 11.5).

### bothyagentconsumerlag
The adapter does not process or does not acknowledge. Adapter logs, `ack` behavior, connection.

### bothyagentoffline
Check the adapter process and its runtime; status of the agent's component.

### bothybillingmodemismatch
Check immediately: is an agent with `personal` running against an organization key or vice versa?
Configuration of the adapter's runtime; change nothing on the server before the cause is clear.
The heartbeat cause comes from the gauge of the heartbeat comparison (`bothy_agent_billing_mode_mismatch_age_seconds`): the agent card (`getAgent`) names the kind in `presence.warnings` (`billing_mode_mismatch`, `billing_mode_unsupported`, `billing_evidence_unknown`), and the audit entries `agent.heartbeat` show when the reported mode changed.

### bothyauthzerror
Treat as an error with a security impact: a route answered without its permission check
(the answer was replaced by `500`) or an unknown action was checked. Read the logs with the
`correlation_id` and the route pattern; do not roll out the affected version any further.

### bothysecretrefreshfailing
The server keeps using the old value. Which reference fails is shown by
`docker compose exec api bothy config check --secrets` (per key `ok` or the error code); the log
line names only the error code. Check the file in the secret provider: present, mode `0400`
or `0600`, owned by the service user (SEC-SEC-1). Until this is fixed, a rotation has no effect.

### bothymalwarescanfailing
`docker compose logs clamav`; signatures loaded? Enough memory? Until then, uploads stay in the
state `pending` and are not served.

### bothybackupstale
`backup_runs` in the admin console; off-site target reachable? Space in the repository?

### bothywalarchivestale
pgBackRest `check` in the container `backup`; TLS between `postgres` and `backup`; space in `pg_wal`.
If `pg_wal` fills up, PostgreSQL stops (PANIC), and the data is preserved
(https://www.postgresql.org/docs/18/continuous-archiving.html, retrieved 2026-10-10) [verified].

### bothyrestoreteststale
Look at the last test run in `backup_runs`; which check fails?

### bothyauditchainbroken
Treat as a security incident: who had write access to the database? Determine the last good point
of the chain, compare backups, inform the owner.

### bothyvolumefull
Which volume? `files`: retention, quotas; `pgdata`: event log partitions, bloat;
`pgbackrest-repo`: local retention.

### bothydatabaseconnectionshigh
`database.max_conns` per process times processes against `max_connections`; hanging connections.

### bothydatabasedeadlocks
Search the logs for `deadlock detected`; check the queries involved against the lock order.

## 5. Check

`make ops-check` checks rules and routes in the gate ([tools/opscheck](../../tools/opscheck/README.md)),
with `promtool` and `amtool` in the version of the images:

1. `promtool check rules` for all rule files (check `prometheus-rules`) and
   `promtool check config` with the loaded rules (check `prometheus-config`); every rule file
   must be loaded through `rule_files`.
2. `promtool test rules` with
   [bothy-alerts.test.yaml](../../infra/observability/prometheus/tests/bothy-alerts.test.yaml)
   (check `prometheus-rule-tests`): every alert of `bothy-alerts.yaml` fires in one case at the
   first evaluation at which it may, with labels and texts: once its `for` has run out, or, for
   an alert without `for`, as soon as its condition holds. One evaluation earlier (the test file
   sets `evaluation_interval: 1m`) it is still quiet
   (Watchdog fires from the first evaluation on). Every alert with a threshold stays quiet in one
   case exactly at the threshold. An alert without a firing case fails the check.
3. `amtool check-config` and the routes from section 1 with `amtool config routes test` (check
   `alertmanager`): `page` to `external-oncall` and `bothy-ops`, `ticket` to `bothy-ops`,
   `Watchdog` to `external-heartbeat`.
   Templates are parsed too: every `templates` entry must lie under `/etc/alertmanager/`, where
   Compose mounts `infra/observability/alertmanager/`, and match a file.
4. The thresholds from 3.2 against the configuration reference (check `alert-thresholds`). If a
   cell of the column "Configuration value" names a value of the reference, it must read exactly
   `` `<name>` `` or ``above `<name>` ``; any other form is a finding.
