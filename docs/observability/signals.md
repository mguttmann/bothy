# Signal-Katalog

Status: Entwurf M0. Einzige Liste aller Telemetrie-Namen, Einheiten und Attribute. Regeln:
[17-observability.md](../spec/17-observability.md). Was hier fehlt, darf der Code nicht senden;
Allowlist des Collectors, Dashboards, Regeln und SLOs nutzen nur Namen von hier.

## 1. Namen und Einheiten

1. Eigene Namen beginnen mit `bothy.`; Namen aus den Semantic Conventions bleiben unverändert.
2. Namen in Kleinbuchstaben mit Punkten; die Einheit steht im Feld `unit` (UCUM), nie im Namen.
3. Zähler zählen Ereignisse (`{event}`, `{run}`), Dauern sind Sekunden (`s`), Größen Byte (`By`).
4. Prometheus übersetzt beim OTLP-Empfang: Punkte werden `_`, die Einheit wird angehängt (`s`
   wird `_seconds`, `By` wird `_bytes`), Zähler bekommen `_total`, Einheiten in geschweiften
   Klammern entfallen. `job` ist `<service.namespace>/<service.name>`, `instance` ist
   `service.instance.id` (https://prometheus.io/docs/guides/opentelemetry/, abgerufen 2026-10-10)
   [belegt]. Lokal geprüft mit Prometheus v3.15.0 und otelcol-contrib 0.162.0 am 2026-10-10:
   `bothy.delivery.duration` (Histogramm, `s`) wird `bothy_delivery_duration_seconds_bucket`,
   `bothy.message.created` (Zähler) wird `bothy_message_created_total`, ganzzahlige
   Bucket-Grenzen erscheinen ohne Nachkommastelle (`le="1"`, `le="5"`) [belegt].
5. Die Spalte "Prometheus" unten ist die Übersetzung; Dashboards und Regeln nutzen sie.

## 2. Kardinalität

1. Metrik-Attribute kommen nur aus geschlossenen Mengen im Code (Aufzählungen) oder aus Mengen,
   die der Betreiber begrenzt (Agenten-Handles, Push-Anbieter, Ereignistypen, Aktionen aus dem
   Katalog). Nie IDs, nie Eingaben von Menschen, nie Fehlertexte, nie ungetemplatete Pfade.
2. `http.route` ist immer das Muster (`/api/v1/threads/{id}/messages`). Anfragen ohne passende Route
   tragen `http.route = "unmatched"`.
3. IDs (Thread, Kanal, Nachricht, Lauf, Datei) stehen nur in Spans und Logs.
4. Ein Unit-Test zählt die möglichen Werte jedes Metrik-Attributs; ein neues offenes Attribut
   bricht den Test.
5. Instrumente aus Bibliotheken (otelhttp, otelpgx) laufen durch eine SDK-View mit einer
   Erlaubnisliste genau der Attribute dieses Katalogs. Grund: otelhttp setzt weitere Attribute,
   z. B. `server.address` aus der `Host`-Kopfzeile des Clients, also eine offene Menge.

## 3. Resource-Attribute

| Attribut | Wert | Quelle |
|---|---|---|
| `service.name` | `bothy-server`, `bothy-backupd`, `bothy-adapter-<art>` | Prozess |
| `service.namespace` | `bothy` | Prozess |
| `service.version` | Version des Artefakts | Build |
| `service.instance.id` | eindeutig je Prozess (Container-Hostname plus Startzeit) | Prozess |
| `deployment.environment.name` | `dev`, `staging`, `prod` | `instance.environment` bzw. `OTEL_RESOURCE_ATTRIBUTES` |
| `bothy.server.role` | `api`, `worker`, `all` | `server.role` |

Prometheus exposes these attributes of a process as the labels of the series `target_info`
(translated like the names in section 1, item 4); dashboards and alerts may use it to count
processes or to detect a missing one (M1-68).

## 4. Metriken

### 4.1 REST und Gateway

| Name | Typ | Einheit | Attribute | Quelle | Prometheus |
|---|---|---|---|---|---|
| `http.server.request.duration` | Histogramm | `s` | `http.request.method`, `http.route`, `http.response.status_code`, `error.type` | otelhttp (Semantic Conventions HTTP, stabil) | `http_server_request_duration_seconds` |
| `bothy.authz.decisions` | Zähler | `{decision}` | `bothy.action`, `bothy.decision` | api: Entscheidungen für Anfragen und Ops; Filter bei Zustellung und in Listen (07 Abschnitt 4.2) zählen nicht, sonst überdeckte die Zustellung jede andere Zahl | `bothy_authz_decisions_total` |
| `bothy.authz.errors` | Zähler | `{check}` | `bothy.reason` (`unknown_action`, `authz_not_checked`, `filter_mismatch`) | api: Programmfehler der Rechteprüfung ([02-architektur.md](../spec/02-architektur.md) 9.3, 07 Abschnitt 4.2); `filter_mismatch`: ein Element passierte den SQL-Filter einer Liste, aber nicht `authorize`, und wurde verworfen (07 Abschnitt 7.4, Regel 3) | `bothy_authz_errors_total` |
| `bothy.ratelimit.rejected` | Zähler | `{request}` | `bothy.ratelimit.class` | api | `bothy_ratelimit_rejected_total` |
| `bothy.console.csp_violations` | Zähler | `{report}` | `bothy.csp.directive`, `bothy.csp.disposition` | api: one per violation report that `submitCspReport` accepted ([19-sicherheit.md](../spec/19-sicherheit.md) 5 rule 6, M6-03); within `rl.public_auth` | `bothy_console_csp_violations_total` |
| `bothy.gateway.connections.active` | UpDownCounter | `{connection}` | `bothy.principal.kind` | api | `bothy_gateway_connections_active` |
| `bothy.gateway.connect.duration` | Histogramm | `s` | `bothy.outcome` (`ok`, `client_error`, `server_error`), `bothy.principal.kind` | api: Upgrade bis `ready` oder `resumed` | `bothy_gateway_connect_duration_seconds` |
| `bothy.gateway.catchup.duration` | Histogramm | `s` | `bothy.outcome` (`complete`, `resync_required`) | api: `ready`/`resumed` bis `catchup.complete` | `bothy_gateway_catchup_duration_seconds` |
| `bothy.gateway.resync` | Zähler | `{connection}` | `bothy.reason` (`retention`, `too_far_behind`, `cursor_ahead`, `epoch_changed`) | api | `bothy_gateway_resync_total` |
| `bothy.gateway.close` | Zähler | `{connection}` | `bothy.close.code` | api | `bothy_gateway_close_total` |
| `bothy.gateway.outbound.dropped` | Zähler | `{frame}` | `bothy.reason` (`patch_replaced_by_snapshot`, `slow_consumer`) | api | `bothy_gateway_outbound_dropped_total` |

`client_error` umfasst die Schließ-Codes `4001` bis `4008`, `4011`, `4012`, `4016` bis `4018` und
Abbrüche des Clients vor `ready`; alles andere ohne `ready` oder `resumed` ist `server_error` (z. B.
`4000`, `4013`). Grund: Wartende Geräte, widerrufene Sitzungen und Protokollfehler des Clients
dürfen das Fehlerbudget von `bothy-sync/reconnect` nicht verbrauchen.

### 4.2 Ereignisse und Zustellung

| Name | Typ | Einheit | Attribute | Quelle | Prometheus |
|---|---|---|---|---|---|
| `bothy.message.created` | Zähler | `{message}` | `bothy.message.origin` (`human`, `agent`, `app`, `import`, `system`), `bothy.message.kind` | api, worker | `bothy_message_created_total` |
| `bothy.event.appended` | Zähler | `{event}` | `bothy.event.type` | api, worker | `bothy_event_appended_total` |
| `bothy.db.event_seq.lock_wait` | Histogramm | `s` | keine | api, worker: Warten auf die Zähler-Zeile | `bothy_db_event_seq_lock_wait_seconds` |
| `bothy.event.fanout.lag` | Gauge | `s` | keine | api: Alter des ältesten Ereignisses, das der Log-Tail kennt und der Hub noch nicht verteilt hat (jetzt minus Zeitpunkt, zu dem es per `NOTIFY` oder Poll bekannt wurde); `0`, wenn alles verteilt ist. Grund: Das Alter des zuletzt verteilten Ereignisses wüchse auf einer ruhigen Instanz und bliebe bei einem hängenden Hub stehen | `bothy_event_fanout_lag_seconds` |
| `bothy.event.consumer.lag` | Gauge | `{event}` | `bothy.consumer.name` (`push`, `webhooks`) | api, worker: `head_seq - last_seq` per row of `event_consumer_cursors` (`head_seq` = the highest `seq` in the log) | `bothy_event_consumer_lag` |
| `bothy.event.consumer.pending_age` | Gauge | `s` | `bothy.consumer.name` | api, worker: now minus `occurred_at` of the oldest event with `seq > last_seq`; `0` when the consumer is at the head. Reason: `sync.event_retention` is a time window | `bothy_event_consumer_pending_age_seconds` |
| `bothy.delivery.duration` | Histogramm | `s` | `bothy.event.durability` (`durable`, `ephemeral`), `bothy.principal.kind` | api: `occurred_at` bis Schreiben des Rahmens, nur Live-Zustellung | `bothy_delivery_duration_seconds` |

### 4.3 Agenten und Läufe

| Name | Typ | Einheit | Attribute | Quelle | Prometheus |
|---|---|---|---|---|---|
| `bothy.run.active` | Gauge | `{run}` | `bothy.agent.handle`, `bothy.run.state` (`queued`, `running`, `waiting_input`) | worker: Abfrage je Intervall | `bothy_run_active` |
| `bothy.run.start.latency` | Histogramm | `s` | `bothy.agent.handle`, `bothy.outcome` (`started`, `failed`, `cancelled`), `bothy.billing_mode` (`personal`, `organization`) | api: `queued_at` bis Verlassen von `queued` | `bothy_run_start_latency_seconds` |
| `bothy.run.duration` | Histogramm | `s` | `bothy.agent.handle`, `bothy.run.state` (`completed`, `failed`, `cancelled`) | api: `started_at` bis Ende | `bothy_run_duration_seconds` |
| `bothy.run.finished` | Zähler | `{run}` | `bothy.agent.handle`, `bothy.run.state`, `bothy.reason` (`user_cancelled`, `access_revoked`, `cancel_timeout`, `agent_disabled`, `billing_mode_personal_not_holder`, `channel_read_only`, `billing_mode_mismatch`, `tool_class_owner_only`, `tool_class_mismatch`, `invalid_invocation_token`, `project_not_ready`, `agent_unavailable`, `interrupted`, `stale`, `runtime_error`, `adapter_error`, `runtime_not_authenticated`, `user_disabled`: the state_reason of 09 section 5.2, any other value records as other; absent for completed) | api | `bothy_run_finished_total` |
| `bothy.run.steps` | Zähler | `{step}` | `bothy.agent.handle`, `bothy.step.kind` | api | `bothy_run_steps_total` |
| `bothy.run.tokens` | Zähler | `{token}` | `bothy.agent.handle`, `bothy.token.type` (`input`, `output`, `cache_read`, `cache_write`), `gen_ai.provider.name`, `gen_ai.request.model` | api: aus `runs.metrics` am Laufende | `bothy_run_tokens_total` |
| `bothy.run.cost` | Zähler | `{currency_unit}` | `bothy.agent.handle`, `bothy.billing_mode`, `bothy.currency`, `bothy.cost.estimate` (`true`, `false`) | api: aus `cost_entries` | `bothy_run_cost_total` |
| `bothy.agent.consumer.lag` | Gauge | `{event}` | `bothy.agent.handle` | api: `head_seq - acked_seq`, Maximum über Verbraucher | `bothy_agent_consumer_lag` |
| `bothy.agent.heartbeat.age` | Gauge | `s` | `bothy.agent.handle` | api | `bothy_agent_heartbeat_age_seconds` |
| `bothy.agent.billing_mode.mismatch_age` | Gauge | `s` | `bothy.agent.handle` | api: seconds since the start of a mismatch between configured and reported mode according to the heartbeat (10, 8); 0 without a mismatch | `bothy_agent_billing_mode_mismatch_age_seconds` |
| `bothy.agent.invocation.denied` | Zähler | `{invocation}` | `bothy.agent.handle`, `bothy.reason` (`billing_mode_personal_not_holder`, `tool_class_owner_only`, `project_not_ready`, `channel_read_only`, `user_disabled`, `agent_disabled`, `access_revoked`) | api: Ablehnung vor Zustellung (I2) | `bothy_agent_invocation_denied_total` |
| `bothy.agent.billing_mode_mismatch` | Zähler | `{run}` | `bothy.agent.handle` | api: I3 | `bothy_agent_billing_mode_mismatch_total` |

Werkzeugnamen stehen nie an Metriken (offene Menge); die Statistik der Werkzeugaufrufe kommt aus
`run_steps` ([admin-statistics.md](admin-statistics.md)).

### 4.3a Terminal

Ohne IDs von Menschen, ohne Rechnernamen, ohne Inhalte ([24-terminal.md](../spec/24-terminal.md),
Abschnitt 10.4).

| Name | Typ | Einheit | Attribute | Quelle | Prometheus |
|---|---|---|---|---|---|
| `bothy.terminal.sessions.active` | UpDownCounter | `{session}` | `bothy.terminal.state` (`opening`, `active`, `detached`) | api | `bothy_terminal_sessions_active` |
| `bothy.terminal.sessions.closed` | Zähler | `{session}` | `bothy.reason` (Codes aus 24, Abschnitt 10.3) | api | `bothy_terminal_sessions_closed_total` |
| `bothy.terminal.relay.unpaired` | Zähler | `{connection}` | `bothy.terminal.side` (`client`, `host`) | api: eine Seite fand innerhalb von `terminal.open_timeout` keine Gegenseite (24, Abschnitt 9) | `bothy_terminal_relay_unpaired_total` |
| `bothy.terminal.hosts.connected` | UpDownCounter | `{host}` | keine | api | `bothy_terminal_hosts_connected` |

### 4.3b Eigene Messung und Live-Statistik

Betriebswerte der eigenen Messung ([25-visualisierung.md](../spec/25-visualisierung.md), Abschnitt
3); die gemessenen Werte selbst stehen in `stats_minutes`, nicht in der Telemetrie.

| Name | Typ | Einheit | Attribute | Quelle | Prometheus |
|---|---|---|---|---|---|
| `bothy.stats.flush.dropped` | Zähler | `{key}` | `bothy.reason` (`queue_full`, `db_error`) | api, worker: verworfene Zählerschlüssel der eigenen Messung | `bothy_stats_flush_dropped_total` |
| `bothy.stats.flush.duration` | Histogramm | `s` | `bothy.outcome` (`ok`, `error`) | api, worker: ein Flush nach `stats_minutes` | `bothy_stats_flush_duration_seconds` |
| `bothy.stats.live.subscribers` | UpDownCounter | `{subscription}` | keine | api: Abos von `stats.live` auf diesem Knoten | `bothy_stats_live_subscribers` |

### 4.4 Push, Dateien, Suche, Jobs

| Name | Typ | Einheit | Attribute | Quelle | Prometheus |
|---|---|---|---|---|---|
| `bothy.push.sent` | Zähler | `{notification}` | `bothy.push.provider`, `bothy.outcome` (`accepted`, `rejected_token`, `rejected_other`, `throttled`, `provider_error`) | worker: one per provider answer of the job `notify.push_send`, the outcome of its result class ([13-push-benachrichtigungen.md](../spec/13-push-benachrichtigungen.md) 7.3), recorded after the commit (M3-19) | `bothy_push_sent_total` |
| `bothy.push.send.duration` | Histogramm | `s` | `bothy.push.provider` | worker: one provider request of the job `notify.push_send`, outside any transaction (M3-19) | `bothy_push_send_duration_seconds` |
| `bothy.push.skipped` | Zähler | `{notification}` | `bothy.reason` (`no_access`, `preference`, `active_in_thread`, `no_registration`, `read`, `expired`, `no_provider`) | worker: one per recipient and hint the consumer `push` or a flush decides without a hint, and one per job `notify.push_send` that ends without sending; quiet hours make a hint quiet and never drop it (R-PN-05), so they are no reason (M3-19) | `bothy_push_skipped_total` |
| `bothy.push.fetch` | Zähler | `{notification}` | `bothy.push.provider` | api: Abruf durch Notification Service Extension bzw. Android-Handler | `bothy_push_fetch_total` |
| `bothy.push.registrations.active` | Gauge | `{registration}` | `bothy.push.provider` | worker: the rows of `push_registrations` with `disabled_at IS NULL` per provider, observed at each collection (M3-19) | `bothy_push_registrations_active` |
| `bothy.file.upload.size` | Histogramm | `By` | keine | api | `bothy_file_upload_size_bytes` |
| `bothy.file.scan` | Zähler | `{file}` | `bothy.outcome` (`clean`, `infected`, `error`, `skipped`) | worker | `bothy_file_scan_total` |
| `bothy.file.scan.duration` | Histogramm | `s` | `bothy.outcome` | worker | `bothy_file_scan_duration_seconds` |
| `bothy.file.scan.queue` | Gauge | `{file}` | keine | worker | `bothy_file_scan_queue` |
| `bothy.search.duration` | Histogramm | `s` | `bothy.outcome` | api | `bothy_search_duration_seconds` |
| `bothy.job.duration` | Histogramm | `s` | `bothy.job.kind`, `bothy.outcome` (`completed`, `failed`, `cancelled`, `snoozed`) | worker | `bothy_job_duration_seconds` |
| `bothy.job.queue` | Gauge | `{job}` | `bothy.job.kind` | worker | `bothy_job_queue` |
| `bothy.egress.blocked` | Zähler | `{request}` | `bothy.reason` (`url_not_allowed`, `scheme_not_allowed`, `host_not_allowed`, `address_blocked`, `redirect_not_allowed`) | api, worker: vom Egress-Client abgewiesene Verbindung ([19-sicherheit.md](../spec/19-sicherheit.md) SEC-NET-5) | `bothy_egress_blocked_total` |

### 4.5 Datenbank

| Name | Typ | Einheit | Attribute | Quelle | Prometheus |
|---|---|---|---|---|---|
| `db.client.operation.duration` | Histogramm | `s` | `db.operation.name`, `db.collection.name` | api, worker: the query tracer of `platform/pg`. otelpgx v0.13.0 (verified 2026-10-18) records a metric of this name with the raw first word of a statement and `pgx.operation.type`, and without `db.collection.name`; the view of the metric registry drops every otelpgx instrument (M1-68) | `db_client_operation_duration_seconds` |
| `bothy.db.pool.connections` | Gauge | `{connection}` | `bothy.pool.state` (`idle`, `acquired`, `max`) | api, worker | `bothy_db_pool_connections` |
| `postgresql.backends`, `postgresql.connection.max`, `postgresql.commits`, `postgresql.rollbacks`, `postgresql.db_size`, `postgresql.table.size`, `postgresql.deadlocks` | wie vom Empfänger | wie vom Empfänger | Resource `postgresql.database.name`, `postgresql.table.name` (als Labels übernommen) | Collector, Empfänger `postgresql` (https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.162.0/receiver/postgresqlreceiver/documentation.md, abgerufen 2026-10-10) [belegt] | `postgresql_backends`, `postgresql_connection_max`, `postgresql_commits_total`, `postgresql_rollbacks_total`, `postgresql_db_size_bytes`, `postgresql_table_size_bytes`, `postgresql_deadlocks_total` |

### 4.6 Sicherung, Audit, Telemetrie

| Name | Typ | Einheit | Attribute | Quelle | Prometheus |
|---|---|---|---|---|---|
| `bothy.backup.last_success.timestamp` | Gauge | `s` | `bothy.backup.kind` (`physical`, `logical`, `objects`) | bothy-backupd | `bothy_backup_last_success_timestamp_seconds` |
| `bothy.backup.wal.last_archived.timestamp` | Gauge | `s` | keine | bothy-backupd | `bothy_backup_wal_last_archived_timestamp_seconds` |
| `bothy.backup.duration` | Gauge | `s` | `bothy.backup.kind` | bothy-backupd (letzter Lauf) | `bothy_backup_duration_seconds` |
| `bothy.backup.size` | Gauge | `By` | `bothy.backup.kind` | bothy-backupd | `bothy_backup_size_bytes` |
| `bothy.backup.restore_test.last_success.timestamp` | Gauge | `s` | keine | bothy-backupd | `bothy_backup_restore_test_last_success_timestamp_seconds` |
| `bothy.backup.restore_test.duration` | Gauge | `s` | keine | bothy-backupd (letzter Lauf) | `bothy_backup_restore_test_duration_seconds` |
| `bothy.backup.restore_test.failures` | Zähler | `{check}` | `bothy.reason` (Name der Prüfung) | bothy-backupd | `bothy_backup_restore_test_failures_total` |
| `bothy.storage.volume.free` | Gauge | `By` | `bothy.volume` (`pgdata`, `files`, `pgbackrest-repo`, `backup-scratch`) | bothy-backupd | `bothy_storage_volume_free_bytes` |
| `bothy.storage.volume.size` | Gauge | `By` | `bothy.volume` | bothy-backupd | `bothy_storage_volume_size_bytes` |
| `bothy.audit.verify.failures` | Zähler | `{run}` | keine | worker: periodische Prüfung der Hash-Kette | `bothy_audit_verify_failures_total` |
| `bothy.audit.verify.last_success.timestamp` | Gauge | `s` | keine | worker | `bothy_audit_verify_last_success_timestamp_seconds` |
| `bothy.telemetry.dropped` | Zähler | `{item}` | `bothy.signal` (`traces`, `metrics`, `logs`) | alle Prozesse | `bothy_telemetry_dropped_total` |
| `bothy.secrets.refresh.failures` | Zähler | `{refresh}` | keine | alle Prozesse: erneutes Lesen eines Geheimnisses scheitert, der alte Wert bleibt ([02-architektur.md](../spec/02-architektur.md) 14.4) | `bothy_secrets_refresh_failures_total` |
| `bothy.client_metrics.discarded` | Counter | `{entry}` | `bothy.reason` (`unknown_name`, `kind_mismatch`, `invalid_bounds`, `invalid_value`) | api: entries of a `submitClientMetrics` batch the server discarded one by one (4.6a, [ADR 0057](../adr/0057-client-telemetrie-ueber-einen-server-endpunkt.md), M4-31) | `bothy_client_metrics_discarded_total` |

`bothy-backupd` sieht die Volumes `pgdata` und `files` nur lesend und misst dort nur den freien
Platz.

### 4.6a Kennzahlen der Apps

Die Apps senden diese Kennzahlen gebündelt an `POST /api/v1/client-metrics` (`ClientMetricsBatch`,
[15-client-architektur.md](../spec/15-client-architektur.md) Abschnitt 17, [ADR 0057](../adr/0057-client-telemetrie-ueber-einen-server-endpunkt.md)). Der Server
nimmt nur die Namen dieser Tabelle an und gibt sie unter `bothy.client.*` an den Collector weiter.
Einen Eintrag mit anderem Namen oder mit einem `kind`, das nicht zum Typ der Tabelle passt
(`histogram`, `counter`, `gauge`), verwirft er einzeln; der Rest des Stapels gilt, die Antwort
bleibt `202`. So verliert eine neuere App gegen einen älteren Server nicht den ganzen Stapel. Attribute aller Kennzahlen: `bothy.client.platform` (`ios`, `android`,
`macos`, `windows`, `web`), `bothy.client.density` (`comfortable`, `compact`, `touch`),
`bothy.client.theme` (`light`, `dark`) und `bothy.client.app_major` (Hauptversion der App, Werte
außerhalb `^[0-9]{1,2}$` werden `other`). Kein Prinzipal, kein Gerät. Histogramme kommen mit den
Grenzen "Dauern bis Sekunden" aus 4.7; andere Grenzen verwirft der Server.

| Name in der App | Typ | Einheit | Weitere Attribute | Server-Name und Prometheus |
|---|---|---|---|---|
| `client.cold_start.duration` | Histogramm | `s` | keine | `bothy.client.cold_start.duration`, `bothy_client_cold_start_duration_seconds` |
| `client.thread_open.duration` | Histogramm | `s` | keine | `bothy.client.thread_open.duration`, `bothy_client_thread_open_duration_seconds` |
| `client.send_local.duration` | Histogramm | `s` | keine | `bothy.client.send_local.duration`, `bothy_client_send_local_duration_seconds` |
| `client.frames` | Zähler | `{frame}` | keine | `bothy.client.frames`, `bothy_client_frames_total` |
| `client.frames.over_budget` | Zähler | `{frame}` | keine | `bothy.client.frames.over_budget`, `bothy_client_frames_over_budget_total` |
| `client.catchup.duration` | Histogramm | `s` | keine | `bothy.client.catchup.duration`, `bothy_client_catchup_duration_seconds` |
| `client.catchup.events` | Zähler | `{event}` | keine | `bothy.client.catchup.events`, `bothy_client_catchup_events_total` |
| `client.resync` | Zähler | `{connection}` | keine | `bothy.client.resync`, `bothy_client_resync_total` |
| `client.reconnects` | Zähler | `{connection}` | `bothy.close.code` (Codes aus 04 "Schließ-Codes", sonst `other`) | `bothy.client.reconnects`, `bothy_client_reconnects_total` |
| `client.outbox.depth` | Gauge | `{entry}` | keine | `bothy.client.outbox.depth`, `bothy_client_outbox_depth` |
| `client.push_display.duration` | Histogramm | `s` | keine | `bothy.client.push_display.duration`, `bothy_client_push_display_duration_seconds` |
| `client.push.placeholder` | Zähler | `{notification}` | keine | `bothy.client.push.placeholder`, `bothy_client_push_placeholder_total` |

### 4.7 Histogramm-Grenzen

| Histogramme | Grenzen |
|---|---|
| Dauern bis Sekunden (`http.server.request.duration`, `bothy.delivery.duration`, `bothy.gateway.connect.duration`, `bothy.run.start.latency`, `bothy.push.send.duration`, `bothy.db.event_seq.lock_wait`, `bothy.search.duration`, `bothy.stats.flush.duration`, `db.client.operation.duration`, and the app histograms of 4.6a: `bothy.client.cold_start.duration`, `bothy.client.thread_open.duration`, `bothy.client.send_local.duration`, `bothy.client.catchup.duration`, `bothy.client.push_display.duration`) | 0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10 |
| lange Dauern (`bothy.run.duration`, `bothy.gateway.catchup.duration`, `bothy.job.duration`, `bothy.file.scan.duration`) | 1, 2.5, 5, 10, 30, 60, 120, 300, 600, 1800, 3600, 7200 |
| Größen (`bothy.file.upload.size`) | 1024, 16384, 131072, 1048576, 8388608, 67108864, 536870912 |

Die erste Reihe ist die Empfehlung (`ExplicitBucketBoundaries`) der Semantic Conventions für
`http.server.request.duration`
(https://github.com/open-telemetry/semantic-conventions/blob/main/docs/http/http-metrics.md,
abgerufen 2026-10-10) [belegt]. Jede SLO-Schwelle ist eine dieser Grenzen (R-OB-06); ein Check
vergleicht `le` in den SLO-Dateien mit dieser Tabelle.

## 5. Traces

### 5.1 Weitergabe

1. W3C Trace Context (`traceparent`, `tracestate`) an jeder HTTP-Anfrage; Apps erzeugen ihn
   selbst, weil es kein Dart-SDK gibt ([17-observability.md](../spec/17-observability.md) 9).
2. Am öffentlichen Listener beginnt der Server für jede Anfrage einen eigenen Trace und hängt
   einen eingehenden `traceparent` nur als Span-Link an (otelhttp-Option für öffentliche
   Endpunkte). Trace-ID und Sampling kommen also vom Server. Nur mit
   `telemetry.traces.trust_remote_parent` wird ein eingehender Kontext Eltern-Kontext; dann gelten
   seine Trace-ID und sein Sampling-Flag, und ein Client wählt damit die `correlation_id` seiner
   Anfragen selbst.
3. `correlation_id` in `event_log` und `audit_events` ist die Trace-ID der auslösenden Anfrage
   (16 Byte, als UUID geschrieben, keine UUIDv7). Jede Anfrage hat eine gültige Trace-ID, auch mit
   `telemetry.enabled = false`: Der Server behält dann einen SDK-Tracer-Provider mit
   `NeverSample` und ohne Exporter. Jobs ohne Anfrage-Bezug haben keine.
4. Ereignisse an Adapter tragen `trace_context` (W3C `traceparent`) der auslösenden Anfrage, wenn
   diese aufgezeichnet wurde (Umschlagfeld `trace_context`, 04 Abschnitt 2.2). Der Adapter macht daraus den
   Eltern-Kontext seines `invoke_agent`-Spans und gibt ihn als Umgebungsvariable `TRACEPARENT` an
   die Laufzeit weiter. Claude Code liest `TRACEPARENT` in `-p` und im Agent SDK
   (https://code.claude.com/docs/en/monitoring-usage, abgerufen 2026-10-10) [belegt].
5. Der Fan-out ist kein Kind der Anfrage, sondern ein eigener Trace mit Link auf sie.

### 5.2 Spans

| Span | Art | Quelle | Wichtige Attribute |
|---|---|---|---|
| `<METHOD> <http.route>` | SERVER | otelhttp | HTTP-Konventionen, `bothy.principal.kind`, `bothy.principal.pseudo_id`, `bothy.action`, `bothy.decision` |
| `gateway.connect` | SERVER | api | `bothy.outcome`, `bothy.close.code`, `bothy.principal.kind` |
| `gateway.catchup` | INTERNAL | api | `bothy.outcome`, `bothy.event.seq` (Start) |
| `event.fanout` | INTERNAL | api, mit Links auf die Anfragen der Ereignisse | `bothy.event.seq` (erste Nummer), Anzahl |
| `agent.dispatch` | PRODUCER | api | `bothy.run.id`, `bothy.agent.handle`, `bothy.decision` |
| `push.send` | CLIENT | worker | `bothy.push.provider`, `bothy.outcome` |
| `webhook.deliver` | CLIENT | worker | `server.address` (host only), `http.response.status_code` (when a response arrived), `bothy.outcome` (`succeeded`, `throttled`, `gone`, `failed`, `blocked`; M2-26) |
| `file.scan` | CLIENT | worker | `bothy.file.id`, `bothy.outcome` |
| `job <bothy.job.kind>` | CONSUMER | worker (River) | `bothy.job.kind`, `bothy.outcome` (`completed`, `failed`, `cancelled`, `snoozed`) |
| `txn.subscribe <Abonnent>` | INTERNAL | api, worker: je In-Tx-Abonnent ([02-architektur.md](../spec/02-architektur.md) 8.3) | `bothy.outcome` (`ok`, `veto`) |
| Datenbank | CLIENT | otelpgx | `db.system.name`, `db.namespace`, `db.operation.name`, `db.query.text` (mit Platzhaltern, nie Parameter) |
| `invoke_agent <gen_ai.agent.name>` | CLIENT | Adapter (SDK) | `gen_ai.*` aus Abschnitt 6, `bothy.run.id` |
| `execute_tool <gen_ai.tool.name>` | INTERNAL | Adapter (SDK) | `gen_ai.tool.name`, `bothy.outcome` |

Ein Span lebt nie länger als eine Operation. Eine WebSocket-Verbindung ist kein Span.

### 5.3 Attribute in Spans und Logs

Diese Liste ist gleich der Allowlist in [infra/otel/collector.yaml](../../infra/otel/collector.yaml)
(Bothy-Teil); `make ops-check` hält beide gleich (Check `collector-allowlist`).

| Attribut | Werte | Hinweis |
|---|---|---|
| `bothy.action` | Aktions-ID aus [05-aktionen.md](../spec/05-aktionen.md) | |
| `bothy.agent.handle` | Handle des Agenten | vom Owner vergeben, kein Mensch |
| `bothy.agent.adapter_kind` | `agents.adapter_kind` | |
| `bothy.backup.kind` | `physical`, `logical`, `objects`, `restore_test` | |
| `bothy.billing_mode` | `personal`, `organization` | |
| `bothy.channel.id`, `bothy.space.id`, `bothy.thread.id`, `bothy.message.id`, `bothy.run.id`, `bothy.file.id` | UUID | nie an Metriken |
| `bothy.close.code` | Schließ-Code aus [04-sync-protokoll.md](../spec/04-sync-protokoll.md) 6.4 | |
| `bothy.consumer.name` | `push`, `webhooks` | name of the worker consumer ([03-datenmodell.md](../spec/03-datenmodell.md) `event_consumer_cursors`) |
| `bothy.correlation_id` | UUID | nur in Logs; in Spans ist es die Trace-ID |
| `bothy.csp.blocked_origin` | origin (scheme, host, port) of the blocked URL of a CSP violation report, or one of `inline`, `eval`, `wasm-eval`, `data`, `blob`, `trusted-types-policy`, `trusted-types-sink`, `other`; never a path, a query or the document URL | only in the log `csp violation reported`, never on metrics (M6-03) |
| `bothy.csp.directive` | effective directive of a CSP violation report of the console from the fixed list of the handler (`script-src-elem`, `style-src-elem`, `trusted-types`, ...), else `other` | log `csp violation reported` (info) of `submitCspReport` and the metric `bothy.console.csp_violations` (M6-03) |
| `bothy.csp.disposition` | `enforce`, `report`, `other` | like `bothy.csp.directive` |
| `bothy.decision` | Entscheidung aus [05-aktionen.md](../spec/05-aktionen.md) 2.5 | |
| `bothy.egress.purpose` | Zweck eines Egress-Clients aus dem Code (`oidc`, `webhook`, ...) | nur in Logs abgewiesener Verbindungen ([19-sicherheit.md](../spec/19-sicherheit.md) SEC-NET-5) |
| `bothy.event.durability` | `durable`, `ephemeral` | |
| `bothy.event.seq` | globale Sequenz | nie an Metriken |
| `bothy.event.type` | Ereignistyp aus [05-aktionen.md](../spec/05-aktionen.md) 6 | |
| `bothy.job.kind` | Job-Art aus dem Code | |
| `bothy.outcome` | feste Liste je Signal | |
| `bothy.principal.kind` | `user`, `agent`, `app`, `anonymous` | |
| `bothy.principal.pseudo_id` | HMAC-SHA-256 der Prinzipal-ID, gekürzt | [privacy.md](privacy.md) 3 |
| `bothy.push.provider` | `apns`, `fcm`, `unifiedpush`, `wns` | |
| `bothy.ratelimit.class` | Klasse aus [05-aktionen.md](../spec/05-aktionen.md) 3 | |
| `bothy.reason` | feste Liste je Signal | nie freier Text |
| `bothy.run.state` | Lauf-Zustand | |
| `bothy.server.role` | `api`, `worker`, `all` | Resource |
| `bothy.signing_key.purpose` | `access_token`, `invocation`, `webhook_out`, `audit_checkpoint` | nur in Logs der Schlüsselwechsel ([19-sicherheit.md](../spec/19-sicherheit.md) 10) |
| `bothy.signing_key.kid` | `kid` eines Server-Schlüssels (Fingerabdruck nach RFC 7638, öffentlich) | nur in Logs der Schlüsselwechsel, nie an Metriken |
| `bothy.signing_key.from_state`, `bothy.signing_key.to_state` | `none`, `next`, `active`, `retiring`, `retired` | nur in Logs der Schlüsselwechsel |
| `bothy.step.kind` | `tool_call`, `status`, `subagent`, `workflow`, `background_job` | |
| `bothy.terminal.side` | `client`, `host` | side of a connection in the relay ([24-terminal.md](../spec/24-terminal.md) section 9); on `bothy.terminal.relay.unpaired` and the log records of the relay (M2-44) |
| `bothy.version.own`, `bothy.version.required` | SemVer der eigenen Server-Version und `instance.min_server_version` | nur in der Startzeile der Schema-Prüfung ([02-architektur.md](../spec/02-architektur.md) 15.1, Schritt 5) |
| `bothy.webhook_subscription.id` | UUID of an outgoing webhook subscription; never on metrics | logs `webhook delivery failed` (info) and `webhook subscription state changed` (warn) of the apps module (M2-26), with `bothy.event.seq`, `bothy.event.type`, `bothy.reason` (the `last_error` code or the cause) and `server.address` |

## 6. GenAI-Konventionen (gekapselt)

1. Die GenAI-Konventionen liegen im Repository `open-telemetry/semantic-conventions-genai` und
   sind vollständig im Status "Development"; für Anthropic gilt `gen_ai.provider.name =
   "anthropic"` (https://github.com/open-telemetry/semantic-conventions-genai, abgerufen
   2026-10-10) [belegt].
2. Je Codebasis gibt es genau ein Paket mit allen `gen_ai.*`-Schlüsseln: im Server
   `internal/platform/otel/genai` (02, PLAN W8), in den SDKs ein Modul `telemetry.genai`. Der Rest
   des Codes nutzt nur dessen Konstanten. Das Paket nennt den Stand der Konvention: Repository,
   Status und Abrufdatum, dazu den Commit, sobald ihn ein Ticket mit Netzzugang gelesen hat. Der
   Commit ist bei der Planung nicht festgehalten worden [offen].
3. Ein Snapshot-Test hält die genutzten Schlüssel fest; eine neue Konvention ändert Paket und
   Snapshot in einer Änderung und Allowlist sowie Dashboards in derselben Änderung.
4. Nie gesendet: `gen_ai.input.messages`, `gen_ai.output.messages`, `gen_ai.system_instructions`,
   `gen_ai.tool.definitions`, Argumente und Ergebnisse von Werkzeugen. Der Collector löscht sie
   zusätzlich.

## 7. Logs

| Feld | Regel |
|---|---|
| Zeit, Stufe | aus dem Log-Record |
| Nachricht (`body`) | konstanter englischer Text aus dem Code, nie zusammengesetzt aus Eingaben; ein Linter erzwingt konstante Nachrichten |
| `trace_id`, `span_id` | aus dem Kontext |
| Attribute | nur aus 5.3 |
| Fehler | `error.type` (Klasse oder Code), nie die Fehlermeldung einer fremden Bibliothek mit Eingabewerten |

Der Server schreibt Logs je nach `telemetry.logs.exporter` als OTLP an den Collector, als JSON auf
stdout oder beides. Im Compose-Stack begrenzt der Log-Treiber stdout (`BOTHY_LOG_MAX_SIZE`,
`BOTHY_LOG_MAX_FILE`). Logs in Go sind im OTel-SDK "Release candidate", in Python "Development"
(https://opentelemetry.io/docs/languages/go/ und /python/, abgerufen 2026-10-10) [belegt]; darum
bleibt stdout als Rückfall.

## 8. Ende-zu-Ende-Beispiele

| Ablauf | Signale |
|---|---|
| Nachricht senden | Span `POST /api/v1/threads/{id}/messages` mit DB-Spans; `bothy.event.appended`, `bothy.db.event_seq.lock_wait`; Span `event.fanout` mit Link; `bothy.delivery.duration` je Empfänger |
| Agenten-Lauf | wie oben, dazu `agent.dispatch`, beim Adapter `invoke_agent` (Kind von `trace_context`), Laufzeit-Spans darunter; `bothy.run.start.latency`, `bothy.run.steps`, `bothy.run.duration`, `bothy.run.tokens`, `bothy.run.cost` |
| Push | Span `push.send` im Trace des Fan-outs; `bothy.push.sent`; später `bothy.push.fetch` beim Abruf durch die App |
| Wiederverbinden | Span `gateway.connect`, `gateway.catchup`; `bothy.gateway.connect.duration`, `bothy.gateway.catchup.duration`, ggf. `bothy.gateway.resync` |
