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

## 3. Resource-Attribute

| Attribut | Wert | Quelle |
|---|---|---|
| `service.name` | `bothy-server`, `bothy-backupd`, `bothy-adapter-<art>` | Prozess |
| `service.namespace` | `bothy` | Prozess |
| `service.version` | Version des Artefakts | Build |
| `service.instance.id` | eindeutig je Prozess (Container-Hostname plus Startzeit) | Prozess |
| `deployment.environment.name` | `dev`, `staging`, `prod` | `instance.environment` bzw. `OTEL_RESOURCE_ATTRIBUTES` |
| `bothy.server.role` | `api`, `worker`, `all` | `server.role` |

## 4. Metriken

### 4.1 REST und Gateway

| Name | Typ | Einheit | Attribute | Quelle | Prometheus |
|---|---|---|---|---|---|
| `http.server.request.duration` | Histogramm | `s` | `http.request.method`, `http.route`, `http.response.status_code`, `error.type` | otelhttp (Semantic Conventions HTTP, stabil) | `http_server_request_duration_seconds` |
| `bothy.authz.decisions` | Zähler | `{decision}` | `bothy.action`, `bothy.decision` | api | `bothy_authz_decisions_total` |
| `bothy.ratelimit.rejected` | Zähler | `{request}` | `bothy.ratelimit.class` | api | `bothy_ratelimit_rejected_total` |
| `bothy.gateway.connections.active` | UpDownCounter | `{connection}` | `bothy.principal.kind` | api | `bothy_gateway_connections_active` |
| `bothy.gateway.connect.duration` | Histogramm | `s` | `bothy.outcome` (`ok`, `client_error`, `server_error`), `bothy.principal.kind` | api: Upgrade bis `ready` oder `resumed` | `bothy_gateway_connect_duration_seconds` |
| `bothy.gateway.catchup.duration` | Histogramm | `s` | `bothy.outcome` (`complete`, `resync_required`) | api: `ready`/`resumed` bis `catchup.complete` | `bothy_gateway_catchup_duration_seconds` |
| `bothy.gateway.resync` | Zähler | `{connection}` | `bothy.reason` (`retention`, `too_far_behind`, `cursor_ahead`, `epoch_changed`) | api | `bothy_gateway_resync_total` |
| `bothy.gateway.close` | Zähler | `{connection}` | `bothy.close.code` | api | `bothy_gateway_close_total` |
| `bothy.gateway.outbound.dropped` | Zähler | `{frame}` | `bothy.reason` (`patch_replaced_by_snapshot`, `slow_consumer`) | api | `bothy_gateway_outbound_dropped_total` |

`client_error` umfasst die Schließ-Codes `4004`, `4007`, `4008` und Abbrüche des Clients vor
`identify`; alles andere ohne `ready` ist `server_error`.

### 4.2 Ereignisse und Zustellung

| Name | Typ | Einheit | Attribute | Quelle | Prometheus |
|---|---|---|---|---|---|
| `bothy.message.created` | Zähler | `{message}` | `bothy.message.origin` (`human`, `agent`, `app`, `import`, `system`), `bothy.message.kind` | api, worker | `bothy_message_created_total` |
| `bothy.event.appended` | Zähler | `{event}` | `bothy.event.type` | api, worker | `bothy_event_appended_total` |
| `bothy.db.event_seq.lock_wait` | Histogramm | `s` | keine | api, worker: Warten auf die Zähler-Zeile | `bothy_db_event_seq_lock_wait_seconds` |
| `bothy.event.fanout.lag` | Gauge | `s` | keine | api: Alter des zuletzt verteilten Ereignisses | `bothy_event_fanout_lag_seconds` |
| `bothy.delivery.duration` | Histogramm | `s` | `bothy.event.durability` (`durable`, `ephemeral`), `bothy.principal.kind` | api: `occurred_at` bis Schreiben des Rahmens, nur Live-Zustellung | `bothy_delivery_duration_seconds` |

### 4.3 Agenten und Läufe

| Name | Typ | Einheit | Attribute | Quelle | Prometheus |
|---|---|---|---|---|---|
| `bothy.run.active` | Gauge | `{run}` | `bothy.agent.handle`, `bothy.run.state` (`queued`, `running`, `waiting_input`) | worker: Abfrage je Intervall | `bothy_run_active` |
| `bothy.run.start.latency` | Histogramm | `s` | `bothy.agent.handle`, `bothy.outcome` (`started`, `failed`, `cancelled`), `bothy.billing_mode` | api: `queued_at` bis Verlassen von `queued` | `bothy_run_start_latency_seconds` |
| `bothy.run.duration` | Histogramm | `s` | `bothy.agent.handle`, `bothy.run.state` (`completed`, `failed`, `cancelled`) | api: `started_at` bis Ende | `bothy_run_duration_seconds` |
| `bothy.run.finished` | Zähler | `{run}` | `bothy.agent.handle`, `bothy.run.state`, `bothy.reason` (`state_reason` aus fester Liste, sonst `other`) | api | `bothy_run_finished_total` |
| `bothy.run.steps` | Zähler | `{step}` | `bothy.agent.handle`, `bothy.step.kind` | api | `bothy_run_steps_total` |
| `bothy.run.tokens` | Zähler | `{token}` | `bothy.agent.handle`, `bothy.token.type` (`input`, `output`, `cache_read`, `cache_write`), `gen_ai.provider.name`, `gen_ai.request.model` | api: aus `runs.metrics` am Laufende | `bothy_run_tokens_total` |
| `bothy.run.cost` | Zähler | `{currency_unit}` | `bothy.agent.handle`, `bothy.billing_mode`, `bothy.currency`, `bothy.cost.estimate` (`true`, `false`) | api: aus `cost_entries` | `bothy_run_cost_total` |
| `bothy.agent.consumer.lag` | Gauge | `{event}` | `bothy.agent.handle` | api: `head_seq - acked_seq`, Maximum über Verbraucher | `bothy_agent_consumer_lag` |
| `bothy.agent.heartbeat.age` | Gauge | `s` | `bothy.agent.handle` | api | `bothy_agent_heartbeat_age_seconds` |
| `bothy.agent.invocation.denied` | Zähler | `{invocation}` | `bothy.agent.handle`, `bothy.reason` | api: Ablehnung vor Zustellung (I2) | `bothy_agent_invocation_denied_total` |
| `bothy.agent.billing_mode_mismatch` | Zähler | `{run}` | `bothy.agent.handle` | api: I3 | `bothy_agent_billing_mode_mismatch_total` |

Werkzeugnamen stehen nie an Metriken (offene Menge); die Statistik der Werkzeugaufrufe kommt aus
`run_steps` ([admin-statistics.md](admin-statistics.md)).

### 4.4 Push, Dateien, Suche, Jobs

| Name | Typ | Einheit | Attribute | Quelle | Prometheus |
|---|---|---|---|---|---|
| `bothy.push.sent` | Zähler | `{notification}` | `bothy.push.provider`, `bothy.outcome` (`accepted`, `rejected_token`, `rejected_other`, `throttled`, `provider_error`) | worker | `bothy_push_sent_total` |
| `bothy.push.send.duration` | Histogramm | `s` | `bothy.push.provider` | worker | `bothy_push_send_duration_seconds` |
| `bothy.push.skipped` | Zähler | `{notification}` | `bothy.reason` (`quiet_hours`, `preference`, `no_access`, `no_registration`) | worker | `bothy_push_skipped_total` |
| `bothy.push.fetch` | Zähler | `{notification}` | `bothy.push.provider` | api: Abruf durch Notification Service Extension bzw. Android-Handler | `bothy_push_fetch_total` |
| `bothy.push.registrations.active` | Gauge | `{registration}` | `bothy.push.provider` | worker | `bothy_push_registrations_active` |
| `bothy.file.upload.size` | Histogramm | `By` | keine | api | `bothy_file_upload_size_bytes` |
| `bothy.file.scan` | Zähler | `{file}` | `bothy.outcome` (`clean`, `infected`, `error`, `skipped`) | worker | `bothy_file_scan_total` |
| `bothy.file.scan.duration` | Histogramm | `s` | `bothy.outcome` | worker | `bothy_file_scan_duration_seconds` |
| `bothy.file.scan.queue` | Gauge | `{file}` | keine | worker | `bothy_file_scan_queue` |
| `bothy.search.duration` | Histogramm | `s` | `bothy.outcome` | api | `bothy_search_duration_seconds` |
| `bothy.job.duration` | Histogramm | `s` | `bothy.job.kind`, `bothy.outcome` | worker | `bothy_job_duration_seconds` |
| `bothy.job.queue` | Gauge | `{job}` | `bothy.job.kind` | worker | `bothy_job_queue` |

### 4.5 Datenbank

| Name | Typ | Einheit | Attribute | Quelle | Prometheus |
|---|---|---|---|---|---|
| `db.client.operation.duration` | Histogramm | `s` | `db.operation.name`, `db.collection.name` | otelpgx (Namen der installierten Version im Ticket prüfen) | `db_client_operation_duration_seconds` |
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

`bothy-backupd` sieht die Volumes `pgdata` und `files` nur lesend und misst dort nur den freien
Platz.

### 4.7 Histogramm-Grenzen

| Histogramme | Grenzen |
|---|---|
| Dauern bis Sekunden (`http.server.request.duration`, `bothy.delivery.duration`, `bothy.gateway.connect.duration`, `bothy.run.start.latency`, `bothy.push.send.duration`, `bothy.db.event_seq.lock_wait`, `bothy.search.duration`, `db.client.operation.duration`) | 0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10 |
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
2. Der Server übernimmt die Trace-ID und entscheidet das Sampling selbst
   (`telemetry.traces.trust_remote_sampling`).
3. `correlation_id` in `event_log` und `audit_events` ist die Trace-ID der auslösenden Anfrage
   (16 Byte, als UUID geschrieben). Hat eine Anfrage keinen `traceparent`, erzeugt der Server eine
   Trace-ID; jede Anfrage hat also eine.
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
| `webhook.deliver` | CLIENT | worker | `server.address`, `http.response.status_code`, `bothy.outcome` |
| `file.scan` | CLIENT | worker | `bothy.file.id`, `bothy.outcome` |
| `job <bothy.job.kind>` | CONSUMER | worker (River) | `bothy.job.kind`, `bothy.outcome` |
| Datenbank | CLIENT | otelpgx | `db.system.name`, `db.namespace`, `db.operation.name`, `db.query.text` (mit Platzhaltern, nie Parameter) |
| `invoke_agent <gen_ai.agent.name>` | CLIENT | Adapter (SDK) | `gen_ai.*` aus Abschnitt 6, `bothy.run.id` |
| `execute_tool <gen_ai.tool.name>` | INTERNAL | Adapter (SDK) | `gen_ai.tool.name`, `bothy.outcome` |

Ein Span lebt nie länger als eine Operation. Eine WebSocket-Verbindung ist kein Span.

### 5.3 Attribute in Spans und Logs

Diese Liste ist gleich der Allowlist in [infra/otel/collector.yaml](../../infra/otel/collector.yaml)
(Bothy-Teil); ein Check hält beide gleich.

| Attribut | Werte | Hinweis |
|---|---|---|
| `bothy.action` | Aktions-ID aus [05-aktionen.md](../spec/05-aktionen.md) | |
| `bothy.agent.handle` | Handle des Agenten | vom Owner vergeben, kein Mensch |
| `bothy.agent.adapter_kind` | `agents.adapter_kind` | |
| `bothy.backup.kind` | `physical`, `logical`, `objects`, `restore_test` | |
| `bothy.billing_mode` | `personal`, `organization` | |
| `bothy.channel.id`, `bothy.space.id`, `bothy.thread.id`, `bothy.message.id`, `bothy.run.id`, `bothy.file.id` | UUID | nie an Metriken |
| `bothy.close.code` | Schließ-Code aus [04-sync-protokoll.md](../spec/04-sync-protokoll.md) 6.4 | |
| `bothy.correlation_id` | UUID | nur in Logs; in Spans ist es die Trace-ID |
| `bothy.decision` | Entscheidung aus [05-aktionen.md](../spec/05-aktionen.md) 2.5 | |
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
| `bothy.step.kind` | `tool_call`, `status`, `subagent`, `workflow`, `background_job` | |

## 6. GenAI-Konventionen (gekapselt)

1. Die GenAI-Konventionen liegen im Repository `open-telemetry/semantic-conventions-genai` und
   sind vollständig im Status "Development"; für Anthropic gilt `gen_ai.provider.name =
   "anthropic"` (https://github.com/open-telemetry/semantic-conventions-genai, abgerufen
   2026-10-10) [belegt].
2. Je Codebasis gibt es genau ein Paket mit allen `gen_ai.*`-Schlüsseln: im Server
   `internal/telemetry/genai`, in den SDKs ein Modul `telemetry.genai`. Der Rest des Codes nutzt
   nur dessen Konstanten. Das Paket nennt den Stand der Konvention (Commit des Repositorys).
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
