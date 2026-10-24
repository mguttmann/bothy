# Signal catalogue

Status: draft M0. The single list of all telemetry names, units and attributes. Rules:
[17-observability.md](../spec/17-observability.md). What is missing here, the code must not send;
the collector allowlist, dashboards, rules and SLOs use only names from here.

## 1. Names and units

1. Own names begin with `bothy.`; names from the Semantic Conventions stay unchanged.
2. Names in lowercase with dots; the unit is in the `unit` field (UCUM), never in the name.
3. Counters count events (`{event}`, `{run}`), durations are seconds (`s`), sizes are bytes (`By`).
4. Prometheus translates on OTLP ingestion: dots become `_`, the unit is appended (`s`
   becomes `_seconds`, `By` becomes `_bytes`), counters get `_total`, units in curly
   braces are dropped. `job` is `<service.namespace>/<service.name>`, `instance` is
   `service.instance.id` (https://prometheus.io/docs/guides/opentelemetry/, retrieved 2026-10-10)
   [verified]. Checked locally with Prometheus v3.15.0 and otelcol-contrib 0.162.0 on 2026-10-10:
   `bothy.delivery.duration` (histogram, `s`) becomes `bothy_delivery_duration_seconds_bucket`,
   `bothy.message.created` (counter) becomes `bothy_message_created_total`, integer
   bucket boundaries appear without decimal places (`le="1"`, `le="5"`) [verified].
5. The "Prometheus" column below is the translation; dashboards and rules use it.

## 2. Cardinality

1. Metric attributes come only from closed sets in the code (enumerations) or from sets
   that the instance operator limits (agent handles, push providers, event types, actions from the
   catalogue). Never IDs, never input from people, never error texts, never untemplated paths.
2. `http.route` is always the pattern (`/api/v1/threads/{id}/messages`). Requests without a matching route
   carry `http.route = "unmatched"`.
3. IDs (thread, channel, message, run, file) appear only in spans and logs.
4. A unit test counts the possible values of every metric attribute; a new open attribute
   breaks the test.
5. Instruments from libraries (otelhttp, otelpgx) run through an SDK view with an
   allowlist of exactly the attributes of this catalogue. Reason: otelhttp sets further attributes,
   for example `server.address` from the client's `Host` header, which is an open set.

## 3. Resource attributes

| Attribute | Value | Source |
|---|---|---|
| `service.name` | `bothy-server`, `bothy-backupd`, `bothy-adapter-<kind>` | process |
| `service.namespace` | `bothy` | process |
| `service.version` | version of the artifact | build |
| `service.instance.id` | unique per process (container hostname plus start time) | process |
| `deployment.environment.name` | `dev`, `staging`, `prod` | `instance.environment` or `OTEL_RESOURCE_ATTRIBUTES` |
| `bothy.server.role` | `api`, `worker`, `all` | `server.role` |

Prometheus exposes these attributes of a process as the labels of the series `target_info`
(translated like the names in section 1, item 4); dashboards and alerts may use it to count
processes or to detect a missing one (M1-68).

## 4. Metrics

### 4.1 REST and gateway

| Name | Type | Unit | Attributes | Source | Prometheus |
|---|---|---|---|---|---|
| `http.server.request.duration` | Histogram | `s` | `http.request.method`, `http.route`, `http.response.status_code`, `error.type` | otelhttp (Semantic Conventions HTTP, stable) | `http_server_request_duration_seconds` |
| `bothy.authz.decisions` | Counter | `{decision}` | `bothy.action`, `bothy.decision` | api: decisions for requests and ops; filters at delivery and in lists (07 section 4.2) do not count, otherwise delivery would mask every other number | `bothy_authz_decisions_total` |
| `bothy.authz.errors` | Counter | `{check}` | `bothy.reason` (`unknown_action`, `authz_not_checked`, `filter_mismatch`) | api: program errors of the permission check ([02-architecture.md](../spec/02-architecture.md) 9.3, 07 section 4.2); `filter_mismatch`: an element passed the SQL filter of a list but not `authorize` and was discarded (07 section 7.4, rule 3) | `bothy_authz_errors_total` |
| `bothy.ratelimit.rejected` | Counter | `{request}` | `bothy.ratelimit.class` | api | `bothy_ratelimit_rejected_total` |
| `bothy.console.csp_violations` | Counter | `{report}` | `bothy.csp.directive`, `bothy.csp.disposition` | api: one per violation report that `submitCspReport` accepted ([19-security.md](../spec/19-security.md) 5 rule 6, M6-03); within `rl.public_auth` | `bothy_console_csp_violations_total` |
| `bothy.gateway.connections.active` | UpDownCounter | `{connection}` | `bothy.principal.kind` | api | `bothy_gateway_connections_active` |
| `bothy.gateway.connect.duration` | Histogram | `s` | `bothy.outcome` (`ok`, `client_error`, `server_error`), `bothy.principal.kind` | api: upgrade until `ready` or `resumed` | `bothy_gateway_connect_duration_seconds` |
| `bothy.gateway.catchup.duration` | Histogram | `s` | `bothy.outcome` (`complete`, `resync_required`) | api: `ready`/`resumed` until `catchup.complete` | `bothy_gateway_catchup_duration_seconds` |
| `bothy.gateway.resync` | Counter | `{connection}` | `bothy.reason` (`retention`, `too_far_behind`, `cursor_ahead`, `epoch_changed`) | api | `bothy_gateway_resync_total` |
| `bothy.gateway.close` | Counter | `{connection}` | `bothy.close.code` | api | `bothy_gateway_close_total` |
| `bothy.gateway.outbound.dropped` | Counter | `{frame}` | `bothy.reason` (`patch_replaced_by_snapshot`, `slow_consumer`) | api | `bothy_gateway_outbound_dropped_total` |
| `bothy.gateway.presence.write_failures` | Counter | `{write}` | none | api | `bothy_gateway_presence_write_failures_total` |

`client_error` covers the close codes `4001` to `4008`, `4011`, `4012`, `4016` to `4018` and
client aborts before `ready`; everything else without `ready` or `resumed` is `server_error` (for example
`4000`, `4013`). Reason: waiting devices, revoked sessions and protocol errors of the client
must not consume the error budget of `bothy-sync/reconnect`.

### 4.2 Events and delivery

| Name | Type | Unit | Attributes | Source | Prometheus |
|---|---|---|---|---|---|
| `bothy.message.created` | Counter | `{message}` | `bothy.message.origin` (`human`, `agent`, `app`, `import`, `system`), `bothy.message.kind` | api, worker | `bothy_message_created_total` |
| `bothy.event.appended` | Counter | `{event}` | `bothy.event.type` | api, worker | `bothy_event_appended_total` |
| `bothy.db.event_seq.lock_wait` | Histogram | `s` | none | api, worker: waiting for the counter row | `bothy_db_event_seq_lock_wait_seconds` |
| `bothy.event.fanout.lag` | Gauge | `s` | none | api: age of the oldest event that the log tail knows and the hub has not yet distributed (now minus the time at which it became known through `NOTIFY` or poll); `0` when everything is distributed. Reason: the age of the last distributed event would grow on a quiet instance and would stand still with a hung hub | `bothy_event_fanout_lag_seconds` |
| `bothy.event.consumer.lag` | Gauge | `{event}` | `bothy.consumer.name` (`push`, `webhooks`) | api, worker: `head_seq - last_seq` per row of `event_consumer_cursors` (`head_seq` = the highest `seq` in the log) | `bothy_event_consumer_lag` |
| `bothy.event.consumer.pending_age` | Gauge | `s` | `bothy.consumer.name` | api, worker: now minus `occurred_at` of the oldest event with `seq > last_seq`; `0` when the consumer is at the head. Reason: `sync.event_retention` is a time window | `bothy_event_consumer_pending_age_seconds` |
| `bothy.delivery.duration` | Histogram | `s` | `bothy.event.durability` (`durable`, `ephemeral`), `bothy.principal.kind` | api: `occurred_at` until the frame is written, live delivery only | `bothy_delivery_duration_seconds` |

### 4.3 Agents and runs

| Name | Type | Unit | Attributes | Source | Prometheus |
|---|---|---|---|---|---|
| `bothy.run.active` | Gauge | `{run}` | `bothy.agent.handle`, `bothy.run.state` (`queued`, `running`, `waiting_input`) | worker: query per interval | `bothy_run_active` |
| `bothy.run.start.latency` | Histogram | `s` | `bothy.agent.handle`, `bothy.outcome` (`started`, `failed`, `cancelled`), `bothy.billing_mode` (`personal`, `organization`) | api: `queued_at` until leaving `queued` | `bothy_run_start_latency_seconds` |
| `bothy.run.duration` | Histogram | `s` | `bothy.agent.handle`, `bothy.run.state` (`completed`, `failed`, `cancelled`) | api: `started_at` until the end | `bothy_run_duration_seconds` |
| `bothy.run.finished` | Counter | `{run}` | `bothy.agent.handle`, `bothy.run.state`, `bothy.reason` (`user_cancelled`, `access_revoked`, `cancel_timeout`, `agent_disabled`, `billing_mode_personal_not_holder`, `channel_read_only`, `billing_mode_mismatch`, `tool_class_owner_only`, `tool_class_mismatch`, `invalid_invocation_token`, `project_not_ready`, `agent_unavailable`, `interrupted`, `stale`, `runtime_error`, `adapter_error`, `runtime_not_authenticated`, `user_disabled`: the state_reason of 09 section 5.2, any other value records as other; absent for completed) | api | `bothy_run_finished_total` |
| `bothy.run.steps` | Counter | `{step}` | `bothy.agent.handle`, `bothy.step.kind` | api | `bothy_run_steps_total` |
| `bothy.run.tokens` | Counter | `{token}` | `bothy.agent.handle`, `bothy.token.type` (`input`, `output`, `cache_read`, `cache_write`), `gen_ai.provider.name`, `gen_ai.request.model` | api: from `runs.metrics` at the end of the run | `bothy_run_tokens_total` |
| `bothy.run.cost` | Counter | `{currency_unit}` | `bothy.agent.handle`, `bothy.billing_mode`, `bothy.currency`, `bothy.cost.estimate` (`true`, `false`) | api: from `cost_entries` | `bothy_run_cost_total` |
| `bothy.agent.consumer.lag` | Gauge | `{event}` | `bothy.agent.handle` | api: `head_seq - acked_seq`, maximum over consumers | `bothy_agent_consumer_lag` |
| `bothy.agent.heartbeat.age` | Gauge | `s` | `bothy.agent.handle` | api | `bothy_agent_heartbeat_age_seconds` |
| `bothy.agent.billing_mode.mismatch_age` | Gauge | `s` | `bothy.agent.handle` | api: seconds since the start of a mismatch between configured and reported mode according to the heartbeat (10, 8); 0 without a mismatch | `bothy_agent_billing_mode_mismatch_age_seconds` |
| `bothy.agent.invocation.denied` | Counter | `{invocation}` | `bothy.agent.handle`, `bothy.reason` (`billing_mode_personal_not_holder`, `tool_class_owner_only`, `project_not_ready`, `channel_read_only`, `user_disabled`, `agent_disabled`, `access_revoked`) | api: rejection before delivery (I2) | `bothy_agent_invocation_denied_total` |
| `bothy.agent.billing_mode_mismatch` | Counter | `{run}` | `bothy.agent.handle` | api: I3 | `bothy_agent_billing_mode_mismatch_total` |

Tool names never appear on metrics (open set); the statistics of the tool calls come from
`run_steps` ([admin-statistics.md](admin-statistics.md)).

### 4.3a Terminal

Without IDs of people, without host names, without contents ([24-terminal.md](../spec/24-terminal.md),
section 10.4).

| Name | Type | Unit | Attributes | Source | Prometheus |
|---|---|---|---|---|---|
| `bothy.terminal.sessions.active` | UpDownCounter | `{session}` | `bothy.terminal.state` (`opening`, `active`, `detached`) | api: the relay of each api process counts the sessions it holds with at least one connection (device stream or host data connection), in the state of the last committed `terminal_session.changed`, and stops counting at release; the sum over the api processes counts sessions with a connection on some node, a session without any connection (host offline, device gone) is not counted (M2-46, D-M2-46-13) | `bothy_terminal_sessions_active` |
| `bothy.terminal.sessions.closed` | Counter | `{session}` | `bothy.reason` (codes from 24, section 10.3) | api, worker: +1 per ended session after the commit of the end, in the process that committed it (a request, a subscriber, the relay or the job `terminal.sweep`); a rolled back end counts nothing (M2-46) | `bothy_terminal_sessions_closed_total` |
| `bothy.terminal.relay.unpaired` | Counter | `{connection}` | `bothy.terminal.side` (`client`, `host`) | api: one side found no counterpart within `terminal.open_timeout` (24, section 9) | `bothy_terminal_relay_unpaired_total` |
| `bothy.terminal.hosts.connected` | UpDownCounter | `{host}` | none | api: each api process counts its control connections that are usable for sessions (+1 when one becomes usable, -1 when it stops being usable); the sum over the api processes counts connected hosts (M2-46) | `bothy_terminal_hosts_connected` |

### 4.3b Own measurement and live statistics

Operating values of the own measurement ([25-visualization.md](../spec/25-visualization.md), section
3); the measured values themselves are in `stats_minutes`, not in the telemetry.

| Name | Type | Unit | Attributes | Source | Prometheus |
|---|---|---|---|---|---|
| `bothy.stats.flush.dropped` | Counter | `{key}` | `bothy.reason` (`queue_full`, `db_error`) | api, worker: discarded counter keys of the own measurement | `bothy_stats_flush_dropped_total` |
| `bothy.stats.flush.duration` | Histogram | `s` | `bothy.outcome` (`ok`, `error`) | api, worker: one flush to `stats_minutes` | `bothy_stats_flush_duration_seconds` |
| `bothy.stats.live.subscribers` | UpDownCounter | `{subscription}` | none | api: subscriptions to `stats.live` on this node | `bothy_stats_live_subscribers` |

### 4.4 Push, files, search, jobs

| Name | Type | Unit | Attributes | Source | Prometheus |
|---|---|---|---|---|---|
| `bothy.push.sent` | Counter | `{notification}` | `bothy.push.provider`, `bothy.outcome` (`accepted`, `rejected_token`, `rejected_other`, `throttled`, `provider_error`) | worker: one per provider answer of the job `notify.push_send`, the outcome of its result class ([13-push-notifications.md](../spec/13-push-notifications.md) 7.3), recorded after the commit (M3-19) | `bothy_push_sent_total` |
| `bothy.push.send.duration` | Histogram | `s` | `bothy.push.provider` | worker: one provider request of the job `notify.push_send`, outside any transaction (M3-19) | `bothy_push_send_duration_seconds` |
| `bothy.push.skipped` | Counter | `{notification}` | `bothy.reason` (`no_access`, `preference`, `active_in_thread`, `no_registration`, `read`, `expired`, `no_provider`) | worker: one per recipient and hint the consumer `push` or a flush decides without a hint, and one per job `notify.push_send` that ends without sending; quiet hours make a hint quiet and never drop it (R-PN-05), so they are no reason (M3-19) | `bothy_push_skipped_total` |
| `bothy.push.fetch` | Counter | `{notification}` | `bothy.push.provider` | api: fetch by the Notification Service Extension or the Android handler; one per `getPushPreview` request whose preview token identified an active registration, with the registration's provider, whatever the answer after that (`200`, `404`, or `401` from the session, proof or rights check); nothing for a malformed or unknown token, because no provider is known. The receipt t2 of [13-push-notifications.md](../spec/13-push-notifications.md) section 9 and the numerator of the confirmation rate; the server keeps no hint record, so it measures no t2 minus t0 latency (M3-22) | `bothy_push_fetch_total` |
| `bothy.push.registrations.active` | Gauge | `{registration}` | `bothy.push.provider` | worker: the rows of `push_registrations` with `disabled_at IS NULL` per provider, observed at each collection (M3-19) | `bothy_push_registrations_active` |
| `bothy.file.upload.size` | Histogram | `By` | none | api | `bothy_file_upload_size_bytes` |
| `bothy.file.scan` | Counter | `{file}` | `bothy.outcome` (`clean`, `infected`, `error`, `skipped`) | worker | `bothy_file_scan_total` |
| `bothy.file.scan.duration` | Histogram | `s` | `bothy.outcome` | worker | `bothy_file_scan_duration_seconds` |
| `bothy.file.scan.queue` | Gauge | `{file}` | none | worker | `bothy_file_scan_queue` |
| `bothy.search.duration` | Histogram | `s` | `bothy.outcome` | api | `bothy_search_duration_seconds` |
| `bothy.job.duration` | Histogram | `s` | `bothy.job.kind`, `bothy.outcome` (`completed`, `failed`, `cancelled`, `snoozed`) | worker | `bothy_job_duration_seconds` |
| `bothy.job.queue` | Gauge | `{job}` | `bothy.job.kind` | worker | `bothy_job_queue` |
| `bothy.egress.blocked` | Counter | `{request}` | `bothy.reason` (`url_not_allowed`, `scheme_not_allowed`, `host_not_allowed`, `address_blocked`, `redirect_not_allowed`) | api, worker: connection rejected by the egress client ([19-security.md](../spec/19-security.md) SEC-NET-5) | `bothy_egress_blocked_total` |

### 4.5 Database

| Name | Type | Unit | Attributes | Source | Prometheus |
|---|---|---|---|---|---|
| `db.client.operation.duration` | Histogram | `s` | `db.operation.name`, `db.collection.name` | api, worker: the query tracer of `platform/pg`. otelpgx v0.13.0 (verified 2026-10-18) records a metric of this name with the raw first word of a statement and `pgx.operation.type`, and without `db.collection.name`; the view of the metric registry drops every otelpgx instrument (M1-68) | `db_client_operation_duration_seconds` |
| `bothy.db.pool.connections` | Gauge | `{connection}` | `bothy.pool.state` (`idle`, `acquired`, `max`) | api, worker | `bothy_db_pool_connections` |
| `postgresql.backends`, `postgresql.connection.max`, `postgresql.commits`, `postgresql.rollbacks`, `postgresql.db_size`, `postgresql.table.size`, `postgresql.deadlocks` | as from the receiver | as from the receiver | resource `postgresql.database.name`, `postgresql.table.name` (taken over as labels) | collector, receiver `postgresql` (https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.162.0/receiver/postgresqlreceiver/documentation.md, retrieved 2026-10-10) [verified] | `postgresql_backends`, `postgresql_connection_max`, `postgresql_commits_total`, `postgresql_rollbacks_total`, `postgresql_db_size_bytes`, `postgresql_table_size_bytes`, `postgresql_deadlocks_total` |

### 4.6 Backup, audit, telemetry

| Name | Type | Unit | Attributes | Source | Prometheus |
|---|---|---|---|---|---|
| `bothy.backup.last_success.timestamp` | Gauge | `s` | `bothy.backup.kind` (`physical`, `logical`, `objects`) | bothy-backupd | `bothy_backup_last_success_timestamp_seconds` |
| `bothy.backup.wal.last_archived.timestamp` | Gauge | `s` | none | bothy-backupd | `bothy_backup_wal_last_archived_timestamp_seconds` |
| `bothy.backup.duration` | Gauge | `s` | `bothy.backup.kind` | bothy-backupd (last run) | `bothy_backup_duration_seconds` |
| `bothy.backup.size` | Gauge | `By` | `bothy.backup.kind` | bothy-backupd | `bothy_backup_size_bytes` |
| `bothy.backup.restore_test.last_success.timestamp` | Gauge | `s` | none | bothy-backupd | `bothy_backup_restore_test_last_success_timestamp_seconds` |
| `bothy.backup.restore_test.duration` | Gauge | `s` | none | bothy-backupd (last run) | `bothy_backup_restore_test_duration_seconds` |
| `bothy.backup.restore_test.failures` | Counter | `{check}` | `bothy.reason` (name of the check) | bothy-backupd | `bothy_backup_restore_test_failures_total` |
| `bothy.storage.volume.free` | Gauge | `By` | `bothy.volume` (`pgdata`, `files`, `pgbackrest-repo`, `backup-scratch`) | bothy-backupd | `bothy_storage_volume_free_bytes` |
| `bothy.storage.volume.size` | Gauge | `By` | `bothy.volume` | bothy-backupd | `bothy_storage_volume_size_bytes` |
| `bothy.audit.verify.failures` | Counter | `{run}` | none | worker: periodic check of the hash chain | `bothy_audit_verify_failures_total` |
| `bothy.audit.verify.last_success.timestamp` | Gauge | `s` | none | worker | `bothy_audit_verify_last_success_timestamp_seconds` |
| `bothy.telemetry.dropped` | Counter | `{item}` | `bothy.signal` (`traces`, `metrics`, `logs`) | all processes | `bothy_telemetry_dropped_total` |
| `bothy.secrets.refresh.failures` | Counter | `{refresh}` | none | all processes: re-reading a secret fails, the old value stays ([02-architecture.md](../spec/02-architecture.md) 14.4) | `bothy_secrets_refresh_failures_total` |
| `bothy.client_metrics.discarded` | Counter | `{entry}` | `bothy.reason` (`unknown_name`, `kind_mismatch`, `invalid_bounds`, `invalid_value`) | api: entries of a `submitClientMetrics` batch the server discarded one by one (4.6a, [ADR 0057](../adr/0057-client-telemetry-through-a-server-endpoint.md), M4-31) | `bothy_client_metrics_discarded_total` |

`bothy-backupd` sees the volumes `pgdata` and `files` read-only and measures only the free
space there.

### 4.6a App metrics

The apps send these metrics batched to `POST /api/v1/client-metrics` (`ClientMetricsBatch`,
[15-client-architecture.md](../spec/15-client-architecture.md) section 17, [ADR 0057](../adr/0057-client-telemetry-through-a-server-endpoint.md)). The server
accepts only the names of this table and passes them on to the collector under `bothy.client.*`.
It discards, one by one, each entry with another name or with a `kind` that does not match the type
of the table (`histogram`, `counter`, `gauge`); the rest of the batch is valid, the response
stays `202`. This way a newer app does not lose the whole batch against an older server. Attributes of all metrics: `bothy.client.platform` (`ios`, `android`,
`macos`, `windows`, `web`), `bothy.client.density` (`comfortable`, `compact`, `touch`),
`bothy.client.theme` (`light`, `dark`) and `bothy.client.app_major` (major version of the app, values
outside `^[0-9]{1,2}$` become `other`). No principal, no device. Histograms come with the
boundaries "Durations up to seconds" from 4.7; the server discards other boundaries.

| Name in the app | Type | Unit | Further attributes | Server name and Prometheus |
|---|---|---|---|---|
| `client.cold_start.duration` | Histogram | `s` | none | `bothy.client.cold_start.duration`, `bothy_client_cold_start_duration_seconds` |
| `client.thread_open.duration` | Histogram | `s` | none | `bothy.client.thread_open.duration`, `bothy_client_thread_open_duration_seconds` |
| `client.send_local.duration` | Histogram | `s` | none | `bothy.client.send_local.duration`, `bothy_client_send_local_duration_seconds` |
| `client.frames` | Counter | `{frame}` | none | `bothy.client.frames`, `bothy_client_frames_total` |
| `client.frames.over_budget` | Counter | `{frame}` | none | `bothy.client.frames.over_budget`, `bothy_client_frames_over_budget_total` |
| `client.catchup.duration` | Histogram | `s` | none | `bothy.client.catchup.duration`, `bothy_client_catchup_duration_seconds` |
| `client.catchup.events` | Counter | `{event}` | none | `bothy.client.catchup.events`, `bothy_client_catchup_events_total` |
| `client.resync` | Counter | `{connection}` | none | `bothy.client.resync`, `bothy_client_resync_total` |
| `client.reconnects` | Counter | `{connection}` | `bothy.close.code` (codes from 04 "Close codes", otherwise `other`) | `bothy.client.reconnects`, `bothy_client_reconnects_total` |
| `client.outbox.depth` | Gauge | `{entry}` | none | `bothy.client.outbox.depth`, `bothy_client_outbox_depth` |
| `client.push_display.duration` | Histogram | `s` | none | `bothy.client.push_display.duration`, `bothy_client_push_display_duration_seconds` |
| `client.push.placeholder` | Counter | `{notification}` | none | `bothy.client.push.placeholder`, `bothy_client_push_placeholder_total` |

### 4.7 Histogram boundaries

| Histograms | Boundaries |
|---|---|
| Durations up to seconds (`http.server.request.duration`, `bothy.delivery.duration`, `bothy.gateway.connect.duration`, `bothy.run.start.latency`, `bothy.push.send.duration`, `bothy.db.event_seq.lock_wait`, `bothy.search.duration`, `bothy.stats.flush.duration`, `db.client.operation.duration`, and the app histograms of 4.6a: `bothy.client.cold_start.duration`, `bothy.client.thread_open.duration`, `bothy.client.send_local.duration`, `bothy.client.catchup.duration`, `bothy.client.push_display.duration`) | 0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10 |
| long durations (`bothy.run.duration`, `bothy.gateway.catchup.duration`, `bothy.job.duration`, `bothy.file.scan.duration`) | 1, 2.5, 5, 10, 30, 60, 120, 300, 600, 1800, 3600, 7200 |
| sizes (`bothy.file.upload.size`) | 1024, 16384, 131072, 1048576, 8388608, 67108864, 536870912 |

The first row is the recommendation (`ExplicitBucketBoundaries`) of the Semantic Conventions for
`http.server.request.duration`
(https://github.com/open-telemetry/semantic-conventions/blob/main/docs/http/http-metrics.md,
retrieved 2026-10-10) [verified]. Every SLO threshold is one of these boundaries (R-OB-06); a check
compares `le` in the SLO files with this table.

## 5. Traces

### 5.1 Propagation

1. W3C Trace Context (`traceparent`, `tracestate`) on every HTTP request; apps create it
   themselves because there is no Dart SDK ([17-observability.md](../spec/17-observability.md) 9).
2. At the public listener the server starts its own trace for every request and attaches
   an incoming `traceparent` only as a span link (otelhttp option for public
   endpoints). Trace ID and sampling therefore come from the server. Only with
   `telemetry.traces.trust_remote_parent` does an incoming context become the parent context; then
   its trace ID and its sampling flag apply, and a client thereby chooses the `correlation_id` of its
   requests itself.
3. `correlation_id` in `event_log` and `audit_events` is the trace ID of the triggering request
   (16 bytes, written as a UUID, not a UUIDv7). Every request has a valid trace ID, even with
   `telemetry.enabled = false`: the server then keeps an SDK tracer provider with
   `NeverSample` and without an exporter. Jobs without a request context have none.
4. Events to adapters carry `trace_context` (W3C `traceparent`) of the triggering request if
   it was recorded (envelope field `trace_context`, 04 section 2.2). The adapter makes it the
   parent context of its `invoke_agent` span and passes it on to the runtime as the environment
   variable `TRACEPARENT`. Claude Code reads `TRACEPARENT` in `-p` and in the Agent SDK
   (https://code.claude.com/docs/en/monitoring-usage, retrieved 2026-10-10) [verified].
5. The fan-out is not a child of the request but its own trace with a link to it.

### 5.2 Spans

| Span | Kind | Source | Important attributes |
|---|---|---|---|
| `<METHOD> <http.route>` | SERVER | otelhttp | HTTP conventions, `bothy.principal.kind`, `bothy.principal.pseudo_id`, `bothy.action`, `bothy.decision` |
| `gateway.connect` | SERVER | api | `bothy.outcome`, `bothy.close.code`, `bothy.principal.kind` |
| `gateway.catchup` | INTERNAL | api | `bothy.outcome`, `bothy.event.seq` (start) |
| `event.fanout` | INTERNAL | api, with links to the requests of the events | `bothy.event.seq` (first number), count |
| `agent.dispatch` | PRODUCER | api | `bothy.run.id`, `bothy.agent.handle`, `bothy.decision` |
| `push.send` | CLIENT | worker | `bothy.push.provider`, `bothy.outcome` |
| `webhook.deliver` | CLIENT | worker | `server.address` (host only), `http.response.status_code` (when a response arrived), `bothy.outcome` (`succeeded`, `throttled`, `gone`, `failed`, `blocked`; M2-26) |
| `file.scan` | CLIENT | worker | `bothy.file.id`, `bothy.outcome` |
| `job <bothy.job.kind>` | CONSUMER | worker (River) | `bothy.job.kind`, `bothy.outcome` (`completed`, `failed`, `cancelled`, `snoozed`) |
| `txn.subscribe <Subscriber>` | INTERNAL | api, worker: per in-tx subscriber ([02-architecture.md](../spec/02-architecture.md) 8.3) | `bothy.outcome` (`ok`, `veto`) |
| Database | CLIENT | otelpgx | `db.system.name`, `db.namespace`, `db.operation.name`, `db.query.text` (with placeholders, never parameters) |
| `invoke_agent <gen_ai.agent.name>` | CLIENT | adapter (SDK) | `gen_ai.*` from section 6, `bothy.run.id` |
| `execute_tool <gen_ai.tool.name>` | INTERNAL | adapter (SDK) | `gen_ai.tool.name`, `bothy.outcome` |

A span never lives longer than one operation. A WebSocket connection is not a span.

### 5.3 Attributes in spans and logs

This list equals the allowlist in [infra/otel/collector.yaml](../../infra/otel/collector.yaml)
(Bothy part); `make ops-check` keeps both equal (check `collector-allowlist`).

| Attribute | Values | Note |
|---|---|---|
| `bothy.action` | action ID from [05-actions.md](../spec/05-actions.md) | |
| `bothy.agent.handle` | handle of the agent | assigned by the owner, not a person |
| `bothy.agent.adapter_kind` | `agents.adapter_kind` | |
| `bothy.backup.kind` | `physical`, `logical`, `objects`, `restore_test` | |
| `bothy.billing_mode` | `personal`, `organization` | |
| `bothy.channel.id`, `bothy.space.id`, `bothy.thread.id`, `bothy.message.id`, `bothy.run.id`, `bothy.file.id` | UUID | never on metrics |
| `bothy.close.code` | close code from [04-sync-protocol.md](../spec/04-sync-protocol.md) 6.4 | |
| `bothy.consumer.name` | `push`, `webhooks` | name of the worker consumer ([03-data-model.md](../spec/03-data-model.md) `event_consumer_cursors`) |
| `bothy.correlation_id` | UUID | only in logs; in spans it is the trace ID |
| `bothy.csp.blocked_origin` | origin (scheme, host, port) of the blocked URL of a CSP violation report, or one of `inline`, `eval`, `wasm-eval`, `data`, `blob`, `trusted-types-policy`, `trusted-types-sink`, `other`; never a path, a query or the document URL | only in the log `csp violation reported`, never on metrics (M6-03) |
| `bothy.csp.directive` | effective directive of a CSP violation report of the console from the fixed list of the handler (`script-src-elem`, `style-src-elem`, `trusted-types`, ...), else `other` | log `csp violation reported` (info) of `submitCspReport` and the metric `bothy.console.csp_violations` (M6-03) |
| `bothy.csp.disposition` | `enforce`, `report`, `other` | like `bothy.csp.directive` |
| `bothy.decision` | decision from [05-actions.md](../spec/05-actions.md) 2.5 | |
| `bothy.egress.purpose` | purpose of an egress client from the code (`oidc`, `webhook`, ...) | only in logs of rejected connections ([19-security.md](../spec/19-security.md) SEC-NET-5) |
| `bothy.event.durability` | `durable`, `ephemeral` | |
| `bothy.event.seq` | global sequence | never on metrics |
| `bothy.event.type` | event type from [05-actions.md](../spec/05-actions.md) 6 | |
| `bothy.job.kind` | job kind from the code | |
| `bothy.outcome` | fixed list per signal | |
| `bothy.principal.kind` | `user`, `agent`, `app`, `anonymous` | |
| `bothy.principal.pseudo_id` | HMAC-SHA-256 of the principal ID, truncated | [privacy.md](privacy.md) 3 |
| `bothy.push.provider` | `apns`, `fcm`, `unifiedpush`, `wns` | |
| `bothy.ratelimit.class` | class from [05-actions.md](../spec/05-actions.md) 3 | |
| `bothy.reason` | fixed list per signal | never free text |
| `bothy.run.state` | run state | |
| `bothy.server.role` | `api`, `worker`, `all` | resource |
| `bothy.signing_key.purpose` | `access_token`, `invocation`, `webhook_out`, `audit_checkpoint` | only in logs of key transitions ([19-security.md](../spec/19-security.md) 10) |
| `bothy.signing_key.kid` | `kid` of a server key (thumbprint per RFC 7638, public) | only in logs of key transitions, never on metrics |
| `bothy.signing_key.from_state`, `bothy.signing_key.to_state` | `none`, `next`, `active`, `retiring`, `retired` | only in logs of key transitions |
| `bothy.step.kind` | `tool_call`, `status`, `subagent`, `workflow`, `background_job` | |
| `bothy.terminal.side` | `client`, `host` | side of a connection in the relay ([24-terminal.md](../spec/24-terminal.md) section 9); on `bothy.terminal.relay.unpaired` and the log records of the relay (M2-44) |
| `bothy.version.own`, `bothy.version.required` | SemVer of the own server version and of `instance.min_server_version` | only in the start line of the schema check ([02-architecture.md](../spec/02-architecture.md) 15.1, step 5) |
| `bothy.webhook_subscription.id` | UUID of an outgoing webhook subscription; never on metrics | logs `webhook delivery failed` (info) and `webhook subscription state changed` (warn) of the apps module (M2-26), with `bothy.event.seq`, `bothy.event.type`, `bothy.reason` (the `last_error` code or the cause) and `server.address` |

## 6. GenAI conventions (encapsulated)

1. The GenAI conventions live in the repository `open-telemetry/semantic-conventions-genai` and
   are entirely in the status "Development"; for Anthropic, `gen_ai.provider.name =
   "anthropic"` applies (https://github.com/open-telemetry/semantic-conventions-genai, retrieved
   2026-10-10) [verified].
2. Per code base there is exactly one package with all `gen_ai.*` keys: in the server
   `internal/platform/otel/genai` (02, PLAN W8), in the SDKs a module `telemetry.genai`. The rest
   of the code uses only its constants. The package names the state of the convention: repository,
   status and retrieval date, plus the commit as soon as a ticket with network access has read it. The
   commit was not recorded during planning [open].
3. A snapshot test records the keys in use; a new convention changes package and
   snapshot in one change, and allowlist and dashboards in the same change.
4. Never sent: `gen_ai.input.messages`, `gen_ai.output.messages`, `gen_ai.system_instructions`,
   `gen_ai.tool.definitions`, arguments and results of tools. The collector also deletes
   them.

## 7. Logs

| Field | Rule |
|---|---|
| Time, level | from the log record |
| Message (`body`) | constant English text from the code, never composed from input; a linter enforces constant messages |
| `trace_id`, `span_id` | from the context |
| Attributes | only from 5.3 |
| Error | `error.type` (class or code), never the error message of a third-party library with input values |

Depending on `telemetry.logs.exporter`, the server writes logs as OTLP to the collector, as JSON to
stdout or both. In the Compose stack the log driver limits stdout (`BOTHY_LOG_MAX_SIZE`,
`BOTHY_LOG_MAX_FILE`). Logs in Go are "Release candidate" in the OTel SDK, in Python "Development"
(https://opentelemetry.io/docs/languages/go/ and /python/, retrieved 2026-10-10) [verified]; therefore
stdout stays as a fallback.

## 8. End-to-end examples

| Flow | Signals |
|---|---|
| Send a message | span `POST /api/v1/threads/{id}/messages` with DB spans; `bothy.event.appended`, `bothy.db.event_seq.lock_wait`; span `event.fanout` with link; `bothy.delivery.duration` per recipient |
| Agent run | as above, plus `agent.dispatch`, at the adapter `invoke_agent` (child of `trace_context`), runtime spans below it; `bothy.run.start.latency`, `bothy.run.steps`, `bothy.run.duration`, `bothy.run.tokens`, `bothy.run.cost` |
| Push | span `push.send` in the trace of the fan-out; `bothy.push.sent`; later `bothy.push.fetch` when the app fetches |
| Reconnect | span `gateway.connect`, `gateway.catchup`; `bothy.gateway.connect.duration`, `bothy.gateway.catchup.duration`, if needed `bothy.gateway.resync` |
