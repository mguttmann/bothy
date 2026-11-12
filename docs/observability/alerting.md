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
`external-heartbeat` [verified]. As built (M7-20): `make ops-check` holds the three routes in the
gate (section 5 item 3), and `make alert-e2e` proves the delivery end to end (section 5 item 5).
The three receivers read their URLs only from secret files (`url_file`), never from the
configuration.

## 2. Alert channel in the instance

As built (M7-20). The guide with the commands is
[docs/operations/alerting-setup.md](../operations/alerting-setup.md).

1. The owner creates, in a session with step-up (console or desktop app; an API token cannot step
   up, [08-identity.md](../spec/08-identity.md) 12), a space and in it the text channel
   "Operations" that only owners and their deputies read (no grant for anyone else), installs an
   app with the scope `messages:write` (`app.install`), grants it `write` on the channel and
   creates an incoming webhook of that app in the channel
   ([05-actions.md](../spec/05-actions.md) `channel.create`, `webhook_in.create`). No tool
   creates these: they need the step-up of a person (07 SR5).
2. Alertmanager sends its own unchanged JSON payload. The server recognises the format per
   request from the body (format `alertmanager`, [webhooks.md](../api/webhooks.md) section 8),
   so the URL that `createIncomingWebhook` returns serves both `native` and `alertmanager`; no
   format is chosen when the webhook is created. Each notification becomes one message with
   alert name, severity, summary, runbook link and the state firing or resolved. Alertmanager in
   the stack uses the same path `/api/v1/hooks/{webhook_id}/{token}` on
   `server.internal_listen_addr` (point 3).
3. The URL of the webhook is a secret. `infra/compose/alert-setup.sh channel` reads it from
   standard input (hidden on a terminal), keeps the path `/api/v1/hooks/<id>/<token>`, replaces
   the origin by the internal listener `http://api:8081` (option `--internal-base`), writes
   `infra/compose/secrets/alertmanager_bothy_webhook_url` with mode `0600` and prints only
   `<created|updated|kept> <name> sha256:<12 hex>`, the fingerprint form of `secrets-init`
   ([16-operations.md](../spec/16-operations.md) 5.3); a second run with the same URL changes nothing.
   `alert-setup.sh external` and `alert-setup.sh heartbeat` write the URLs of the external
   on-call channel and of the heartbeat service unchanged. `secrets-init` copies the three files
   into the subdirectory `alertmanager` of the volume `secrets` (files `0444`, as for every
   third-party service), which Alertmanager mounts read-only at `/run/bothy/secrets`. Alertmanager
   reaches the server only through the internal listener; there is no public path.
4. Alertmanager v0.34.1 starts with a missing `url_file` and fails only when it sends (checked
   2026-10-20 with the image `prom/alertmanager:v0.34.1`), which would drop every alert in
   silence. So it starts through the guard `infra/observability/alertmanager/start.sh`: while one
   `url_file` of the configuration is missing, unreadable or empty, it exits with a message that
   names the files (never a value) and the commands that fix it.
5. Messages from incoming webhooks never trigger agents (`origin = app`).

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
| `BothyBackupTelemetryMissing` | page | no `target_info` from `bothy-backupd` for 10 minutes | |
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
| `BothyBackupRunFailed` | ticket | a backup run failed in the last 24 hours, per kind of `backup_runs` (`pg_full`, `pg_diff`, `logical`, `objects`) | |
| `BothyWalArchiveStale` | page | no WAL segment for 15 minutes | above `backup.pg.archive_timeout` |
| `BothyRestoreTestStale` | ticket | no restore test for 8 days | `backup.restore_test.max_age` |
| `BothyRestoreTestFailed` | ticket | a check of the automatic restore test other than the audit chain failed in the last 24 hours | |
| `BothyRestoreTestAuditChainFailed` | page | the check of the audit chain of the automatic restore test failed in the last 24 hours | |
| `BothyAuditChainBroken` | page | check of the hash chain fails | |
| `BothyVolumeFull` | page | under 10 percent free | |
| `BothyDatabaseConnectionsHigh` | ticket | over 80 percent of `max_connections` | |
| `BothyDatabaseDeadlocks` | ticket | deadlock reported | Lock order [03-data-model.md](../spec/03-data-model.md) 6.3 |

`BothyBackupStale`, `BothyWalArchiveStale` and `BothyRestoreTestStale` compare a time that only
`bothy-backupd` reports ([signals.md](signals.md) 4.6). They fail closed: a time that was not
reported at all for as long as the threshold (26 hours per backup kind, 15 minutes, 8 days)
counts like one that is too old, so a backup kind or a restore test that never succeeded and a
value that is no longer reported raise the alert instead of keeping it quiet.
`BothyBackupTelemetryMissing` reports within minutes that `bothy-backupd` sends nothing; it is
the only alert for the volumes in that case, because `BothyVolumeFull` has no values then.

## 4. Runbook anchors

Every rule names a `runbook_url` that points to an anchor of this file (alert name in lowercase).
The first steps are in 3.1; for 3.2:

### watchdog
No action needed. If the heartbeat is missing at the external service: check Prometheus and Alertmanager.

### bothyservertelemetrymissing
`docker compose ps`; is `api` running? If yes: check the collector and `OTEL_EXPORTER_OTLP_ENDPOINT`.

### bothybackuptelemetrymissing
`docker compose ps backup`; is `bothy-backupd` running, and what does `docker compose logs backup`
say (a configuration that does not load ends it with exit code `2`)? If it runs: check the
collector and `OTEL_EXPORTER_OTLP_ENDPOINT` of the service `backup`. While this alert fires,
nothing watches the backups, the WAL archive and the volumes: `bothy-backupd` sends every value
that `BothyBackupStale`, `BothyWalArchiveStale`, `BothyRestoreTestStale` and `BothyVolumeFull`
read. The service is also the repository host that `postgres` archives its WAL to, so `pg_wal`
grows while it is down (see `bothywalarchivestale`).

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
A worker consumer (`push`, `webhooks`, `privacy.journal`; label `bothy_consumer_name`) stands
still: pushes or outgoing webhooks no longer go out, or deletions no longer reach the deletion
journal (a restore in that window would bring them back, [backup-restore.md](../operations/backup-restore.md) 7). Is a process of the role `worker` or `all` running? Read the
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
The alert also fires for a kind without a reported last success in the last 26 hours. Then its
runs have failed since the first one (the newest row of the kind in `backup_runs` holds the code
in `error`; with the driver `s3` the kind `objects` needs an off-site target and fails with
`s3_mirror_not_configured` without one), or `bothy-backupd` reports no value: either
`BothyBackupTelemetryMissing` fires too, or its log holds `read the last successful runs`, which
means that it could not read `backup_runs` yet; it tries again every minute. On a new
installation the alert fires until every kind has succeeded once
([install-compose.md](../operations/install-compose.md) 4).

### bothybackuprunfailed
The label names the kind of the run. Its newest row in `backup_runs` (`state = 'failed'`) holds
the code of the failed step in `error`, for example `pgbackrest_exit_82`; the log of the container
`backup` holds the output of the tool. After the fix, run the kind by hand
(`docker compose exec backup bothy-backupd backup <kind>`). The alert ends 24 hours after the
last failed run of the kind. A run that the stop of `bothy-backupd` canceled raises no alert, and
neither does a row closed as `interrupted` at its next start.

### bothywalarchivestale
pgBackRest `check` in the container `backup`; TLS between `postgres` and `backup`; space in `pg_wal`.
If `pg_wal` fills up, PostgreSQL stops (PANIC), and the data is preserved
(https://www.postgresql.org/docs/18/continuous-archiving.html, retrieved 2026-10-10) [verified].
The alert also fires when no time of a last archived segment was reported for 15 minutes:
PostgreSQL has archived none since its statistics were reset (`pg_stat_archiver`),
`bothy-backupd` cannot read them (its log holds `backup sample of the WAL archive failed`), or it
reports nothing (`BothyBackupTelemetryMissing`).

### bothyrestoreteststale
Look at the last test run in `backup_runs`; which check fails?
The alert also fires when no last successful test was reported for 8 days, for example because
no test has succeeded since the installation; run one by hand
(`docker compose exec backup bothy-backupd restore-test`).

### bothyrestoretestfailed
The label names the failed check. The newest `restore_test` row of `backup_runs` holds the result
and the reason code of every check in `checks`
([backup-restore.md](../operations/backup-restore.md) 4). After the fix, run the test by hand
(`docker compose exec backup bothy-backupd restore-test`).

### bothyrestoretestauditchainfailed
The audit chain of the restored copy does not verify; `checks` of the newest `restore_test` row
holds the reason code of the check `audit_chain`. If `BothyAuditChainBroken` fires as well, the
chain of the running database is broken: follow its steps. If it is quiet, the backup or the
repository is damaged: `docker compose exec backup pgbackrest --stanza=bothy verify`, then a new
full backup and the test again.

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
   case exactly at the threshold. An alert without a firing case fails the check. The three
   alerts on a time that `bothy-backupd` reports also fire in a case where the time was never
   reported and in one where it stops; the cases over the windows of 26 hours and 8 days are in
   [bothy-alerts-windows.test.yaml](../../infra/observability/prometheus/tests/bothy-alerts-windows.test.yaml),
   which evaluates every 15 minutes.
3. `amtool check-config` and the routes from section 1 with `amtool config routes test` (check
   `alertmanager`): `page` to `external-oncall` and `bothy-ops`, `ticket` to `bothy-ops`,
   `Watchdog` to `external-heartbeat`.
   Templates are parsed too: every `templates` entry must lie under `/etc/alertmanager/`, where
   Compose mounts `infra/observability/alertmanager/`, and match a file.
4. The thresholds from 3.2 against the configuration reference (check `alert-thresholds`). If a
   cell of the column "Configuration value" names a value of the reference, it must read exactly
   `` `<name>` `` or ``above `<name>` ``; any other form is a finding.
5. `make alert-e2e` ([tools/observability/alert_e2e.py](../../tools/observability/alert_e2e.py),
   not in `make check`, [gates.md](../quality/gates.md)) proves the path end to end against the
   Compose smoke stack of M1-73 with the profile `observability` and the test overlay
   `infra/compose/test/compose.alert-e2e.yaml`: Alertmanager refuses to start without its URL
   files; the owner sets up the channel "Operations", the app and the incoming webhook with
   step-up; `alert-setup.sh` writes the three URLs (the external ones point to a local stub);
   Prometheus loads the test rules `infra/compose/test/alert-e2e/rules.yaml` (one `page`, one
   `ticket` alert, only in the overlay) next to the real ones. The channel shows a firing message
   per test alert with name, level and summary and origin `app`, then a resolved message after
   the rules stop; the external stub gets the `page` alert and never the `ticket` alert; the
   heartbeat stub gets the Watchdog. Not proven: a real external on-call or heartbeat service.
