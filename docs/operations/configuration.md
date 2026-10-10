# Konfigurationsreferenz

Status: Entwurf M0. Sammlung aller benannten Konfigurationswerte mit Umgebungsname, Typ,
Standard und Bedeutung. Regeln: [16-betrieb.md](../spec/16-betrieb.md) 5 und ADR 0011.

Maßgeblich für einen Standardwert ist die Tabelle "Konfigurationswerte dieses Kapitels" im
genannten Kapitel (Ruling L4). Abschnitt 4 wird daraus erzeugt (`tools/docs/config_reference.py`,
Check in `make docs`). Ab M1 ist das Konfigurationsschema des Servers die Quelle; dann entsteht
Abschnitt 4 aus dem Schema, und der Check vergleicht es mit den Kapiteltabellen.

## 1. Quellen

| Quelle | Inhalt | Ändern | Wirkung |
|---|---|---|---|
| Prozess-Konfiguration | alle Schlüssel aus Abschnitt 4 mit Ort `Server` | Umgebung (`BOTHY_*`) oder YAML-Datei (`BOTHY_CONFIG_FILE`) | beim Start; Geheimnisse nach `secrets.cache_ttl` |
| Instanz-Einstellungen | Abschnitt 6, Tabelle `instance_settings` | Admin-Konsole, `instance.settings.update`, `instance.branding.update` (Owner, Step-up, Audit) | sofort, Ereignis `instance.updated` |
| Feature-Flags | Abschnitt 7, Tabelle `feature_flags` | Admin-Konsole, `feature_flag.update` (Owner, Step-up, Audit) | sofort, Ereignis `instance.updated` |
| OpenTelemetry | Abschnitt 8, Variablen `OTEL_*` | Umgebung | beim Start |
| Compose | Abschnitt 9, Datei `infra/compose/.env` | Datei | bei `docker compose up` |

Vorrang der Prozess-Konfiguration: Kommandozeile (nur `--role`, `--config`) vor Umgebung vor Datei
vor Standard. Ein Schlüssel steht in genau einer Quelle. Abschnitt 4 führt auch Schlüssel, deren
Nutzer erst in einer späteren Etappe entsteht; das Konfigurationsschema des Servers kennt einen
Schlüssel ab dem Ticket, das seinen ersten Nutzer baut ([02-architektur.md](../spec/02-architektur.md)
13, Punkt 5).

## 2. Namensregel und Formate

| Thema | Regel | Beispiel |
|---|---|---|
| Umgebungsname | `BOTHY_` plus Schlüssel in Großbuchstaben, Punkt wird `__` | `sync.event_retention` -> `BOTHY_SYNC__EVENT_RETENTION` |
| Datei | YAML, Schlüssel verschachtelt nach Punkten | `sync: {event_retention: 30d}` |
| Dauer (`duration`) | Zahl plus Einheit `ms`, `s`, `m`, `h`, `d` (`d` = 24 h) | `250ms`, `30d` |
| Liste (`list(...)`) | Umgebung: durch Komma getrennt; Datei: YAML-Liste | `BOTHY_SERVER__TRUSTED_PROXIES=192.0.2.10/32,198.51.100.0/24` |
| `cron` | fünf Felder, Zeitzone UTC | `0 2 * * 0` |
| `secret_ref` | `<schema>:<pfad>`, Schemata in [16-betrieb.md](../spec/16-betrieb.md) 5.2 | `file:/run/bothy/secrets/db_app_password` |
| `listen address` | `host:port`; `:port` heißt alle Schnittstellen | `127.0.0.1:8080` |
| `semver` | SemVer ohne `v` | `1.4.0` |
| Egress-Ziel (`egress.allowed_hosts`) | `host[:port][;allow_private]`; Host als DNS-Name (klein, ohne Punkt am Ende) oder IP-Literal (IPv6 in Klammern, wenn ein Port folgt); kein Platzhalter, kein Schema, kein Pfad; Port 1 bis 65535; einziger Schalter `allow_private` | `api.example.com:443`, `[2001:db8::10]:8443;allow_private` |

Schlüssel enthalten nie zwei Unterstriche hintereinander; darum ist die Namensregel umkehrbar.

## 3. Prüfung

1. Unbekannte `BOTHY_*`-Variablen und unbekannte Schlüssel in der Datei brechen den Start ab.
   Ausgenommen sind `BOTHY_CONFIG_FILE` und Variablen, die ein wirksamer Verweis `env:` nennt
   ([16-betrieb.md](../spec/16-betrieb.md) 5.1).
2. Typ- und Bereichsfehler brechen den Start ab; die Meldung nennt Schlüssel und Quelle, nie einen
   Wert (ein Geheimnis kann im falschen Schlüssel stehen).
3. `bothy config check` prüft ohne Start. `bothy config print --redacted` zeigt die wirksamen
   Werte und ihre Quelle, Geheimnisse nur als Verweis.

## 4. Benannte Werte aller Kapitel

Erzeugt von `tools/docs/config_reference.py` aus den Tabellen "Konfigurationswerte dieses
Kapitels" (Ruling L4); `make config-reference` schreibt den Block neu, `make docs` prüft, dass er
aktuell ist und kein Name doppelt vorkommt. Nicht von Hand ändern. Spalte "Ort": `Server` ist die
Prozess-Konfiguration aus Abschnitt 1 mit Umgebungsnamen nach Abschnitt 2; andere Orte (App,
Adapter, Runner, Werkzeug, Abgleich-App) sind Werte dieser Programme. Typ und Wertebereich stehen
im jeweiligen Kapitel; ab M1 kommen sie aus dem Konfigurationsschema des Servers.

<!-- config-reference:start -->
### 01 Produkt

Quelle: [01-produkt.md](../spec/01-produkt.md), Abschnitt 14.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `ui.message_group_window` |  | App | `5m` | Nachrichten desselben Autors innerhalb dieser Zeit ohne eigenen Kopf |
| `ui.message_collapse_lines` |  | App | `24` | ab so vielen Zeilen wird eine Nachricht eingeklappt |
| `ui.livebox_visible_steps` |  | App | `6` | sichtbare Schritte in der laufenden Live-Box |
| `ui.composer_max_lines` |  | App | `12` | so weit wächst die Eingabe |
| `ui.autocomplete_debounce` |  | App | `150ms` | Wartezeit vor `command.autocomplete` |
| `ui.quick_switch_max_results` |  | App | `20` | Treffer im Schnellwechsel |
| `ui.desktop_min_width_four_columns` |  | App | `1280px` | darunter klappt die Sitzungs-Spalte ein |
| `ui.desktop_min_width_three_columns` |  | App | `960px` | darunter wird die Beitragsliste zum Stapel |
| `a11y.live_announce_min_interval` |  | App | `10s` | höchstens so oft sagt die Live-Box einen Wechsel an |
| `client.share.recent_targets` |  | App | `8` | zuletzt genutzte Ziele im Teilen-Blatt |

### 02 Architektur

Quelle: [02-architektur.md](../spec/02-architektur.md), Abschnitt 22.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `cluster.mode` | `BOTHY_CLUSTER__MODE` | Server | `single` | `single` oder `multi` (mehrere api-Knoten) |
| `http.read_header_timeout` | `BOTHY_HTTP__READ_HEADER_TIMEOUT` | Server | `5s` | Frist für Kopfzeilen |
| `http.request_timeout` | `BOTHY_HTTP__REQUEST_TIMEOUT` | Server | `15s` | Kontext-Frist normaler Anfragen |
| `http.transfer_timeout` | `BOTHY_HTTP__TRANSFER_TIMEOUT` | Server | `10m` | Frist für Upload und Download |
| `http.idle_timeout` | `BOTHY_HTTP__IDLE_TIMEOUT` | Server | `120s` | Leerlauf von Keep-Alive-Verbindungen |
| `http.max_header_bytes` | `BOTHY_HTTP__MAX_HEADER_BYTES` | Server | `65536` | größte Kopfzeilen |
| `http.max_json_body_bytes` | `BOTHY_HTTP__MAX_JSON_BODY_BYTES` | Server | `1048576` | größter JSON-Körper; echt größer als `message.max_content_bytes` (13, Punkt 4) |
| `http.max_json_depth` | `BOTHY_HTTP__MAX_JSON_DEPTH` | Server | `32` | größte Schachtelungstiefe eines JSON-Körpers (SEC-IN-2); ein Test hält, dass jedes Anfrage-Beispiel in `openapi.yaml` darunter bleibt |
| `http.unavailable_retry_after` | `BOTHY_HTTP__UNAVAILABLE_RETRY_AFTER` | Server | `5s` | `Retry-After` jeder Antwort `503 unavailable` (in ganzen Sekunden) |
| `database.acquire_timeout` | `BOTHY_DATABASE__ACQUIRE_TIMEOUT` | Server | `2s` | Warten auf eine Pool-Verbindung |
| `database.statement_timeout` | `BOTHY_DATABASE__STATEMENT_TIMEOUT` | Server | `5s` | Frist je SQL-Anweisung |
| `database.lock_timeout` | `BOTHY_DATABASE__LOCK_TIMEOUT` | Server | `2s` | Frist für Sperren |
| `database.idle_in_tx_timeout` | `BOTHY_DATABASE__IDLE_IN_TX_TIMEOUT` | Server | `10s` | offene Transaktion ohne Arbeit |
| `database.tx_retry_max` | `BOTHY_DATABASE__TX_RETRY_MAX` | Server | `2` | Wiederholungen des Unit of Work bei `40P01` und `55P03` |
| `database.listen_reconnect_backoff_cap` | `BOTHY_DATABASE__LISTEN_RECONNECT_BACKOFF_CAP` | Server | `10s` | größter Abstand beim Neuaufbau von `LISTEN` |
| `startup.dependency_wait` | `BOTHY_STARTUP__DEPENDENCY_WAIT` | Server | `60s` | Warten auf die Datenbank beim Start |
| `sync.log_poll_interval` | `BOTHY_SYNC__LOG_POLL_INTERVAL` | Server | `1s` | Poll des Log-Tails ohne `NOTIFY` |
| `sync.tail_batch_size` | `BOTHY_SYNC__TAIL_BATCH_SIZE` | Server | `1000` | Ereignisse je Lesevorgang des Log-Tails |
| `sync.partition_maintenance_interval` | `BOTHY_SYNC__PARTITION_MAINTENANCE_INTERVAL` | Server | `10m` | Abstand des Jobs `sync.partition_maintenance` |
| `sync.event_log_partitions_ahead` | `BOTHY_SYNC__EVENT_LOG_PARTITIONS_AHEAD` | Server | `3` | Partitionen von `event_log`, die vor dem Kopf bereitstehen; mit `sync.event_log_partition_size` reichen sie bei `perf.event_commit_min_per_s` länger als zwei Abstände `sync.partition_maintenance_interval` |
| `gateway.write_timeout` | `BOTHY_GATEWAY__WRITE_TIMEOUT` | Server | `10s` | Frist für einen WebSocket-Schreibvorgang |
| `consumer.batch_size` | `BOTHY_CONSUMER__BATCH_SIZE` | Server | `500` | Ereignisse je Runde eines Worker-Verbrauchers |
| `consumer.poll_interval` | `BOTHY_CONSUMER__POLL_INTERVAL` | Server | `1s` | Poll eines Worker-Verbrauchers ohne `NOTIFY` |
| `sync.idempotency_cleanup_interval` | `BOTHY_SYNC__IDEMPOTENCY_CLEANUP_INTERVAL` | Server | `1h` | Abstand des Jobs `sync.idempotency_cleanup` |
| `sync.idempotency_cleanup_batch_size` | `BOTHY_SYNC__IDEMPOTENCY_CLEANUP_BATCH_SIZE` | Server | `1000` | Zeilen je Löschstapel von `sync.idempotency_cleanup` |
| `ratelimit.max_tracked_keys` | `BOTHY_RATELIMIT__MAX_TRACKED_KEYS` | Server | `100000` | höchstens so viele Rate-Limit-Eimer hält ein Prozess; darüber werden volle, dann älteste Eimer entfernt |
| `ratelimit.sweep_interval` | `BOTHY_RATELIMIT__SWEEP_INTERVAL` | Server | `1m` | Abstand, in dem volle Eimer entfernt werden (ein voller Eimer entscheidet wie ein fehlender) |
| `jobs.schema` | `BOTHY_JOBS__SCHEMA` | Server | `jobs` | PostgreSQL-Schema der River-Tabellen |
| `jobs.queue.default.max_workers` | `BOTHY_JOBS__QUEUE__DEFAULT__MAX_WORKERS` | Server | `8` | Parallelität `default` |
| `jobs.queue.files.max_workers` | `BOTHY_JOBS__QUEUE__FILES__MAX_WORKERS` | Server | `2` | Parallelität `files` |
| `jobs.queue.delivery.max_workers` | `BOTHY_JOBS__QUEUE__DELIVERY__MAX_WORKERS` | Server | `16` | Parallelität `delivery` |
| `jobs.queue.maintenance.max_workers` | `BOTHY_JOBS__QUEUE__MAINTENANCE__MAX_WORKERS` | Server | `2` | Parallelität `maintenance` |
| `jobs.queue.privacy.max_workers` | `BOTHY_JOBS__QUEUE__PRIVACY__MAX_WORKERS` | Server | `1` | Parallelität `privacy` |
| `shutdown.drain_delay` | `BOTHY_SHUTDOWN__DRAIN_DELAY` | Server | `5s` | Warten nach `ready = false` |
| `shutdown.http_grace` | `BOTHY_SHUTDOWN__HTTP_GRACE` | Server | `20s` | Frist für laufende HTTP-Anfragen |
| `shutdown.jobs_soft_timeout` | `BOTHY_SHUTDOWN__JOBS_SOFT_TIMEOUT` | Server | `20s` | weicher Stopp von River |
| `shutdown.telemetry_flush_timeout` | `BOTHY_SHUTDOWN__TELEMETRY_FLUSH_TIMEOUT` | Server | `5s` | Telemetrie leeren |
| `shutdown.total_timeout` | `BOTHY_SHUTDOWN__TOTAL_TIMEOUT` | Server | `60s` | gesamte Frist bis zum Ende; mindestens die Summe der Schritt-Budgets (15.3) |
| `perf.reference_profile` |  | Lasttest | `4 vCPU, 8 GiB RAM, SSD, PostgreSQL 18 lokal, Rolle all` | Umgebung der Lasttests |
| `perf.sustained_messages_per_s` |  | Lasttest | `200` | Dauerlast für `perf.message_create_p99` |
| `perf.message_create_p99` |  | Lasttest | `150ms` | Ziel |
| `perf.event_commit_min_per_s` |  | Lasttest | `1000` | Ziel (Mindestwert) |
| `perf.fanout_local_p99` |  | Lasttest | `200ms` | Ziel |
| `perf.fanout_cross_node_p99` |  | Lasttest | `500ms` | Ziel |
| `perf.invocation_delivery_p99` |  | Lasttest | `500ms` | Ziel |
| `perf.run_patch_fanout_p99` |  | Lasttest | `500ms` | Ziel |
| `perf.catchup_page_p99` |  | Lasttest | `250ms` | Ziel |
| `perf.authorize_cached_p99` |  | Lasttest | `100us` | Ziel |
| `perf.ws_connections_per_node` |  | Lasttest | `5000` | Ziel (Mindestwert) |
| `perf.ws_idle_conn_mem_max` |  | Lasttest | `64KiB` | Ziel (Höchstwert) |
| `perf.idle_rss_max` |  | Lasttest | `256MiB` | Ziel (Höchstwert) |
| `perf.startup_ready_max` |  | Lasttest | `5s` | Ziel (Höchstwert) |
| `perf.push_handoff_p95` |  | Lasttest | `2s` | Ziel |

### 03 Datenmodell (PostgreSQL)

Quelle: [03-datenmodell.md](../spec/03-datenmodell.md), Abschnitt 9.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `principal.handle_max_chars` | `BOTHY_PRINCIPAL__HANDLE_MAX_CHARS` | Server | `32` | Länge eines Handles |
| `principal.display_name_max_chars` | `BOTHY_PRINCIPAL__DISPLAY_NAME_MAX_CHARS` | Server | `64` | Länge eines Anzeigenamens in Unicode-Zeichen |
| `identity.last_active_granularity` | `BOTHY_IDENTITY__LAST_ACTIVE_GRANULARITY` | Server | `5m` | höchstens so oft wird `last_active_at` bzw. `last_used_at` geschrieben |
| `auth.session_max_age` | `BOTHY_AUTH__SESSION_MAX_AGE` | Server | `30d` | absolutes Ende einer Anmeldesitzung |
| `auth.refresh_token_ttl` | `BOTHY_AUTH__REFRESH_TOKEN_TTL` | Server | `14d` | Lebensdauer eines Refresh-Tokens |
| `auth.step_up_max_age` | `BOTHY_AUTH__STEP_UP_MAX_AGE` | Server | `5m` | so lange gilt eine Step-up-Bestätigung als frisch |
| `auth.refresh_reuse_grace` | `BOTHY_AUTH__REFRESH_REUSE_GRACE` | Server | `30s` | Nachsicht für die unmittelbar vorige Refresh-Generation mit demselben DPoP-Schlüssel |
| `agent.heartbeat_persist_interval` | `BOTHY_AGENT__HEARTBEAT_PERSIST_INTERVAL` | Server | `60s` | höchstens so oft werden Heartbeat-Felder gespeichert |
| `channel.history_backfill_limit_default` | `BOTHY_CHANNEL__HISTORY_BACKFILL_LIMIT_DEFAULT` | Server | `20` | Standard für `history_backfill_limit` |
| `channel.history_backfill_limit_max` | `BOTHY_CHANNEL__HISTORY_BACKFILL_LIMIT_MAX` | Server | `200` | Obergrenze für `history_backfill_limit` |
| `channel.prompt_max_bytes` | `BOTHY_CHANNEL__PROMPT_MAX_BYTES` | Server | `32768` | Größe eines Kanal-Prompts |
| `thread.title_max_chars` | `BOTHY_THREAD__TITLE_MAX_CHARS` | Server | `200` | Länge eines Thread-Titels |
| `message.max_content_bytes` | `BOTHY_MESSAGE__MAX_CONTENT_BYTES` | Server | `262144` | Größe einer Nachricht (lange Antworten ohne Teilen) |
| `message.max_attachments` | `BOTHY_MESSAGE__MAX_ATTACHMENTS` | Server | `10` | Anhänge je Nachricht |
| `file.max_upload_bytes` | `BOTHY_FILE__MAX_UPLOAD_BYTES` | Server | `104857600` | Größe einer Datei |
| `run.excerpt_max_chars` | `BOTHY_RUN__EXCERPT_MAX_CHARS` | Server | `500` | Länge von `last_text_excerpt` |
| `run.step_detail_max_bytes` | `BOTHY_RUN__STEP_DETAIL_MAX_BYTES` | Server | `16384` | Größe von `run_steps.detail` |
| `run.max_steps_persisted` | `BOTHY_RUN__MAX_STEPS_PERSISTED` | Server | `2000` | gespeicherte Schritte je Lauf |
| `run.stall_after` | `BOTHY_RUN__STALL_AFTER` | Server | `10m` | ohne Fortschritt: Stillstandshinweis |
| `component.default_ttl` | `BOTHY_COMPONENT__DEFAULT_TTL` | Server | `24h` | Ablauf einer Komponente ohne eigene Angabe |
| `push.max_consecutive_failures` | `BOTHY_PUSH__MAX_CONSECUTIVE_FAILURES` | Server | `5` | danach wird eine Push-Registrierung deaktiviert |
| `webhook.allowed_hosts` | `BOTHY_WEBHOOK__ALLOWED_HOSTS` | Server | leer | erlaubte Ziel-Hosts für ausgehende Webhooks |
| `webhook.max_consecutive_failures` | `BOTHY_WEBHOOK__MAX_CONSECUTIVE_FAILURES` | Server | `20` | danach wird ein Abo deaktiviert |
| `webhook.retry_backoff_base` | `BOTHY_WEBHOOK__RETRY_BACKOFF_BASE` | Server | `10s` | Basis des Backoffs |
| `webhook.retry_backoff_cap` | `BOTHY_WEBHOOK__RETRY_BACKOFF_CAP` | Server | `1h` | größter Abstand |
| `sync.event_log_partition_size` | `BOTHY_SYNC__EVENT_LOG_PARTITION_SIZE` | Server | `1000000` | Sequenznummern je Partition |

### 04 Ereignis- und Sync-Protokoll

Quelle: [04-sync-protokoll.md](../spec/04-sync-protokoll.md), Abschnitt 18.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `sync.event_retention` | `BOTHY_SYNC__EVENT_RETENTION` | Server | `30d` | so lange bleiben dauerhafte Ereignisse nachholbar |
| `sync.event_payload_max_bytes` | `BOTHY_SYNC__EVENT_PAYLOAD_MAX_BYTES` | Server | `65536` | größte Nutzlast eines Ereignisses |
| `sync.ws_max_frame_bytes` | `BOTHY_SYNC__WS_MAX_FRAME_BYTES` | Server | `1048576` | größter WebSocket-Rahmen |
| `sync.identify_timeout` | `BOTHY_SYNC__IDENTIFY_TIMEOUT` | Server | `10s` | Zeit zwischen `hello` und `identify` |
| `sync.heartbeat_interval` | `BOTHY_SYNC__HEARTBEAT_INTERVAL` | Server | `30s` | Abstand der Heartbeats (in `hello` gemeldet) |
| `sync.heartbeat_miss_limit` | `BOTHY_SYNC__HEARTBEAT_MISS_LIMIT` | Server | `2` | fehlende Heartbeats bis zum Schließen |
| `sync.resume_window` | `BOTHY_SYNC__RESUME_WINDOW` | Server | `5m` | so lange ist `resume` nach Trennung möglich |
| `sync.reauth_lead_time` | `BOTHY_SYNC__REAUTH_LEAD_TIME` | Server | `60s` | Vorlauf von `reauth_required` vor Token-Ablauf |
| `sync.reconnect_backoff_base` | `BOTHY_SYNC__RECONNECT_BACKOFF_BASE` | Server | `1s` | Basis des Backoffs beim Wiederverbinden |
| `sync.reconnect_backoff_cap` | `BOTHY_SYNC__RECONNECT_BACKOFF_CAP` | Server | `60s` | größter Abstand beim Wiederverbinden |
| `sync.catchup_page_size` | `BOTHY_SYNC__CATCHUP_PAGE_SIZE` | Server | `500` | Ereignisse je Nachhol-Seite |
| `sync.catchup_max_events` | `BOTHY_SYNC__CATCHUP_MAX_EVENTS` | Server | `50000` | mehr Rückstand ergibt `resync_required` |
| `sync.cursor_advance_every` | `BOTHY_SYNC__CURSOR_ADVANCE_EVERY` | Server | `1000` | nach so vielen übersprungenen Ereignissen ein `cursor`-Rahmen |
| `sync.outbound_queue_max` | `BOTHY_SYNC__OUTBOUND_QUEUE_MAX` | Server | `10000` | Rahmen im Ausgang einer Verbindung bis `4014` |
| `sync.idempotency_key_ttl` | `BOTHY_SYNC__IDEMPOTENCY_KEY_TTL` | Server | `24h` | Aufbewahrung von `Idempotency-Key` |
| `sync.max_focused_threads` | `BOTHY_SYNC__MAX_FOCUSED_THREADS` | Server | `20` | Threads im `focus` je Verbindung |
| `sync.run_patch_coalesce_interval` | `BOTHY_SYNC__RUN_PATCH_COALESCE_INTERVAL` | Server | `250ms` | Sammelfenster für `run.updated` |
| `sync.typing_min_interval` | `BOTHY_SYNC__TYPING_MIN_INTERVAL` | Server | `5s` | höchstens so oft `typing.start` |
| `sync.typing_ttl` | `BOTHY_SYNC__TYPING_TTL` | Server | `8s` | Tipp-Anzeige ohne Erneuerung |
| `client.cache_max_age` | `BOTHY_CLIENT__CACHE_MAX_AGE` | Server | `30d` | ungenutzte Cache-Einträge |
| `client.outbox_retry_base` | `BOTHY_CLIENT__OUTBOX_RETRY_BASE` | Server | `2s` | Basis des Backoffs im Ausgangsfach |
| `client.outbox_retry_cap` | `BOTHY_CLIENT__OUTBOX_RETRY_CAP` | Server | `5m` | größter Abstand im Ausgangsfach |
| `agent.invocation_token_ttl` | `BOTHY_AGENT__INVOCATION_TOKEN_TTL` | Server | `60s` | Gültigkeit einer Aufruf-Marke |
| `agent.ack_timeout` | `BOTHY_AGENT__ACK_TIMEOUT` | Server | `30s` | ohne `ack`: erneute Zustellung |
| `agent.ack_persist_interval` | `BOTHY_AGENT__ACK_PERSIST_INTERVAL` | Server | `1s` | spätestens so oft wird `acked_seq` gespeichert |
| `agent.max_inflight_events` | `BOTHY_AGENT__MAX_INFLIGHT_EVENTS` | Server | `100` | unbestätigte Ereignisse je Verbraucher |
| `agent.run_update_min_interval` | `BOTHY_AGENT__RUN_UPDATE_MIN_INTERVAL` | Server | `500ms` | Adapter-Drossel für `run.update` |
| `agent.presence_timeout` | `BOTHY_AGENT__PRESENCE_TIMEOUT` | Server | `90s` | ohne Heartbeat gilt der Agent als `offline` |
| `agent.cancel_grace` | `BOTHY_AGENT__CANCEL_GRACE` | Server | `30s` | Zeit für den Adapter, einen Abbruch zu melden |

### 05 Aktionskatalog der Autorisierung

Quelle: [05-aktionen.md](../spec/05-aktionen.md), Abschnitt 10.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `audit.deny_aggregate_window` | `BOTHY_AUDIT__DENY_AGGREGATE_WINDOW` | Server | `60s` | Zusammenfassungsfenster für gleiche Ablehnungen |
| `audit.deny_aggregate_max_keys` | `BOTHY_AUDIT__DENY_AGGREGATE_MAX_KEYS` | Server | `10000` | höchstens so viele offene Zusammenfassungen je Prozess; begrenzt den Speicher bei Fluten ohne Anmeldung |
| `status.public_enabled` | `BOTHY_STATUS__PUBLIC_ENABLED` | Server | `false` | Statusseite ohne Anmeldung |
| `ratelimit.rl.public_auth.per_minute` | `BOTHY_RATELIMIT__RL__PUBLIC_AUTH__PER_MINUTE` | Server | `30` | je Client-IP |
| `ratelimit.rl.public_auth.burst` | `BOTHY_RATELIMIT__RL__PUBLIC_AUTH__BURST` | Server | `10` | je Client-IP |
| `ratelimit.rl.gateway_connect.per_minute` | `BOTHY_RATELIMIT__RL__GATEWAY_CONNECT__PER_MINUTE` | Server | `12` | je Gerät oder Token |
| `ratelimit.rl.gateway_connect.burst` | `BOTHY_RATELIMIT__RL__GATEWAY_CONNECT__BURST` | Server | `4` | je Gerät oder Token |
| `ratelimit.rl.read.per_minute` | `BOTHY_RATELIMIT__RL__READ__PER_MINUTE` | Server | `600` | je Prinzipal |
| `ratelimit.rl.read.burst` | `BOTHY_RATELIMIT__RL__READ__BURST` | Server | `100` | je Prinzipal |
| `ratelimit.rl.write.per_minute` | `BOTHY_RATELIMIT__RL__WRITE__PER_MINUTE` | Server | `120` | je Prinzipal |
| `ratelimit.rl.write.burst` | `BOTHY_RATELIMIT__RL__WRITE__BURST` | Server | `30` | je Prinzipal |
| `ratelimit.rl.message.per_minute` | `BOTHY_RATELIMIT__RL__MESSAGE__PER_MINUTE` | Server | `60` | je Prinzipal |
| `ratelimit.rl.message.burst` | `BOTHY_RATELIMIT__RL__MESSAGE__BURST` | Server | `20` | je Prinzipal |
| `ratelimit.rl.trigger.per_minute` | `BOTHY_RATELIMIT__RL__TRIGGER__PER_MINUTE` | Server | `30` | je Prinzipal |
| `ratelimit.rl.trigger.burst` | `BOTHY_RATELIMIT__RL__TRIGGER__BURST` | Server | `10` | je Prinzipal |
| `ratelimit.rl.interaction.per_minute` | `BOTHY_RATELIMIT__RL__INTERACTION__PER_MINUTE` | Server | `60` | je Prinzipal |
| `ratelimit.rl.interaction.burst` | `BOTHY_RATELIMIT__RL__INTERACTION__BURST` | Server | `20` | je Prinzipal |
| `ratelimit.rl.upload.per_minute` | `BOTHY_RATELIMIT__RL__UPLOAD__PER_MINUTE` | Server | `30` | je Prinzipal |
| `ratelimit.rl.upload.burst` | `BOTHY_RATELIMIT__RL__UPLOAD__BURST` | Server | `10` | je Prinzipal |
| `ratelimit.rl.search.per_minute` | `BOTHY_RATELIMIT__RL__SEARCH__PER_MINUTE` | Server | `60` | je Prinzipal |
| `ratelimit.rl.search.burst` | `BOTHY_RATELIMIT__RL__SEARCH__BURST` | Server | `20` | je Prinzipal |
| `ratelimit.rl.typing.per_minute` | `BOTHY_RATELIMIT__RL__TYPING__PER_MINUTE` | Server | `30` | je Verbindung |
| `ratelimit.rl.typing.burst` | `BOTHY_RATELIMIT__RL__TYPING__BURST` | Server | `5` | je Verbindung |
| `ratelimit.rl.admin.per_minute` | `BOTHY_RATELIMIT__RL__ADMIN__PER_MINUTE` | Server | `60` | je Prinzipal |
| `ratelimit.rl.admin.burst` | `BOTHY_RATELIMIT__RL__ADMIN__BURST` | Server | `20` | je Prinzipal |
| `ratelimit.rl.export.per_minute` | `BOTHY_RATELIMIT__RL__EXPORT__PER_MINUTE` | Server | `2` | je Prinzipal |
| `ratelimit.rl.export.burst` | `BOTHY_RATELIMIT__RL__EXPORT__BURST` | Server | `2` | je Prinzipal |
| `ratelimit.rl.agent_runtime.per_minute` | `BOTHY_RATELIMIT__RL__AGENT_RUNTIME__PER_MINUTE` | Server | `1200` | je Agent |
| `ratelimit.rl.agent_runtime.burst` | `BOTHY_RATELIMIT__RL__AGENT_RUNTIME__BURST` | Server | `200` | je Agent |
| `ratelimit.rl.webhook_in.per_minute` | `BOTHY_RATELIMIT__RL__WEBHOOK_IN__PER_MINUTE` | Server | `60` | je Eingangs-Webhook |
| `ratelimit.rl.webhook_in.burst` | `BOTHY_RATELIMIT__RL__WEBHOOK_IN__BURST` | Server | `20` | je Eingangs-Webhook |
| `ratelimit.rl.import.per_minute` | `BOTHY_RATELIMIT__RL__IMPORT__PER_MINUTE` | Server | `120` | je Owner für `import.apply` |
| `ratelimit.rl.import.burst` | `BOTHY_RATELIMIT__RL__IMPORT__BURST` | Server | `30` | je Owner für `import.apply` |

### 06 Öffentliche API

Quelle: [06-api.md](../spec/06-api.md), Abschnitt 20.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `api.page_size_default` | `BOTHY_API__PAGE_SIZE_DEFAULT` | Server | `50` | Seitengröße ohne `limit` |
| `api.page_size_max` | `BOTHY_API__PAGE_SIZE_MAX` | Server | `200` | größere `limit` werden gekappt |
| `api.cors_allowed_origins` | `BOTHY_API__CORS_ALLOWED_ORIGINS` | Server | leer | zusätzliche Origins für CORS |
| `api.deprecation_min_notice` | `BOTHY_API__DEPRECATION_MIN_NOTICE` | Server | `180d` | Mindestabstand von `Deprecation` zu `Sunset` |
| `api.major_version_overlap` | `BOTHY_API__MAJOR_VERSION_OVERLAP` | Server | `365d` | so lange bleibt eine alte Hauptversion nach Erscheinen der neuen |
| `api.stats_max_buckets` | `BOTHY_API__STATS_MAX_BUCKETS` | Server | `1000` | Zeit-Buckets je Statistik-Abfrage; mehr ergibt `400` |
| `status.history_window` | `BOTHY_STATUS__HISTORY_WINDOW` | Server | `7d` | so lange zeigt die Statusseite gelöste Störungen |

### 07 Rechte und Autorisierung

Quelle: [07-rechte.md](../spec/07-rechte.md), Abschnitt 14.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `authz.expiry_sweep_interval` | `BOTHY_AUTHZ__EXPIRY_SWEEP_INTERVAL` | Server | `1m` | Abstand des Jobs, der Abläufe meldet und Erinnerungen schreibt |
| `authz.expiry_reminder_lead` | `BOTHY_AUTHZ__EXPIRY_REMINDER_LEAD` | Server | `48h` | so lange vor Ablauf kommt die Erinnerung (Studie C: zwei Tage) |
| `authz.expiry_sweep_batch_max` | `BOTHY_AUTHZ__EXPIRY_SWEEP_BATCH_MAX` | Server | `500` | Zeilen je Transaktion der Jobs `authz.expiry_sweep` und `authz.expiry_reminder` |
| `authz.view_as_max_duration` | `BOTHY_AUTHZ__VIEW_AS_MAX_DURATION` | Server | `15m` | Höchstdauer von "Als Person ansehen" |
| `authz.snapshot_cache_max_entries` | `BOTHY_AUTHZ__SNAPSHOT_CACHE_MAX_ENTRIES` | Server | `10000` | Rechte-Stände im Speicher je Prozess (LRU) |
| `authz.not_found_timing_tolerance` | `BOTHY_AUTHZ__NOT_FOUND_TIMING_TOLERANCE` | Server | `20ms` | erlaubte Abweichung des Medians in AZ-P4 |

### 08 Identität, Geräte und Sitzungen

Quelle: [08-identitaet.md](../spec/08-identitaet.md), Abschnitt 18.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `auth.access_token_ttl` | `BOTHY_AUTH__ACCESS_TOKEN_TTL` | Server | `10m` | Lebensdauer eines Zugriffstokens |
| `auth.dpop_nonce_ttl` | `BOTHY_AUTH__DPOP_NONCE_TTL` | Server | `5m` | Gültigkeit eines DPoP-Nonce (Kopfzeile `DPoP-Nonce` und `hello` des Gateways) |
| `auth.token_pepper_ref` | `BOTHY_AUTH__TOKEN_PEPPER_REF` | Server | `file:<secrets.dir>/token_pepper` | Verweis auf den Server-Pepper für die HMAC-Hashes aller Tokens und Codes (03, Grundsatz 9) |
| `auth.clock_leeway` | `BOTHY_AUTH__CLOCK_LEEWAY` | Server | `30s` | erlaubte Uhrabweichung bei `exp`, `nbf`, `iat` |
| `auth.dpop_proof_max_age` | `BOTHY_AUTH__DPOP_PROOF_MAX_AGE` | Server | `60s` | Höchstalter eines DPoP-Beweises (`iat`) |
| `auth.login_flow_ttl` | `BOTHY_AUTH__LOGIN_FLOW_TTL` | Server | `10m` | Gültigkeit eines Anmeldevorgangs |
| `auth.server_code_ttl` | `BOTHY_AUTH__SERVER_CODE_TTL` | Server | `60s` | Gültigkeit des einmaligen Server-Codes |
| `auth.oidc_jwks_cache_ttl` | `BOTHY_AUTH__OIDC_JWKS_CACHE_TTL` | Server | `1h` | Cache der Anbieter-Schlüssel |
| `auth.group_claim_max_age` | `BOTHY_AUTH__GROUP_CLAIM_MAX_AGE` | Server | `30d` | Ablauf von Mitgliedschaften aus Gruppen-Claims nach der letzten Anmeldung |
| `auth.idp_recheck_interval` | `BOTHY_AUTH__IDP_RECHECK_INTERVAL` | Server | `30d` | spätestens dann neue Anmeldung beim Anbieter |
| `auth.trusted_header_bind_requests` | `BOTHY_AUTH__TRUSTED_HEADER_BIND_REQUESTS` | Server | `true` | jede Anfrage muss denselben Header-Login tragen |
| `auth.enrollment_code_ttl` | `BOTHY_AUTH__ENROLLMENT_CODE_TTL` | Server | `72h` | Gültigkeit eines Einladungs-Codes |
| `auth.enrollment_code_max_attempts` | `BOTHY_AUTH__ENROLLMENT_CODE_MAX_ATTEMPTS` | Server | `5` | Fehlversuche je Einladungs-Code |
| `auth.device_pending_ttl` | `BOTHY_AUTH__DEVICE_PENDING_TTL` | Server | `24h` | danach wird ein wartendes Gerät widerrufen |
| `auth.device_verification_code_digits` | `BOTHY_AUTH__DEVICE_VERIFICATION_CODE_DIGITS` | Server | `6` | Länge des Bestätigungs-Codes |
| `auth.pairing_code_ttl` | `BOTHY_AUTH__PAIRING_CODE_TTL` | Server | `5m` | Gültigkeit eines Kopplungs-Codes |
| `auth.pairing_code_chars` | `BOTHY_AUTH__PAIRING_CODE_CHARS` | Server | `8` | Länge des Kopplungs-Codes (Crockford-Base32, 40 Bit) |
| `auth.device_inactive_revoke_after` | `BOTHY_AUTH__DEVICE_INACTIVE_REVOKE_AFTER` | Server | `180d` | Gerät ohne Kontakt wird widerrufen |
| `auth.step_up_challenge_ttl` | `BOTHY_AUTH__STEP_UP_CHALLENGE_TTL` | Server | `2m` | Gültigkeit einer Step-up-Herausforderung |
| `auth.step_up_fail_limit` | `BOTHY_AUTH__STEP_UP_FAIL_LIMIT` | Server | `5` | Fehlversuche beim Step-up je Fenster |
| `auth.require_key_attestation` | `BOTHY_AUTH__REQUIRE_KEY_ATTESTATION` | Server | `false` | Plattform-Attestierung der Geräteschlüssel verlangen |
| `auth.console_session_max_age` | `BOTHY_AUTH__CONSOLE_SESSION_MAX_AGE` | Server | `12h` | absolute Dauer einer Konsolen-Sitzung |
| `auth.console_session_idle_timeout` | `BOTHY_AUTH__CONSOLE_SESSION_IDLE_TIMEOUT` | Server | `30m` | Leerlauf bis zum Ende der Konsolen-Sitzung |
| `auth.local_password_enabled` | `BOTHY_AUTH__LOCAL_PASSWORD_ENABLED` | Server | `true` | Passwort mit TOTP als Rückfall für lokale Konten |
| `auth.password_min_length` | `BOTHY_AUTH__PASSWORD_MIN_LENGTH` | Server | `12` | Mindestlänge eines Passworts |
| `auth.argon2id_memory_kib` | `BOTHY_AUTH__ARGON2ID_MEMORY_KIB` | Server | `65536` | Argon2id-Speicher (Untergrenze 19456) |
| `auth.argon2id_iterations` | `BOTHY_AUTH__ARGON2ID_ITERATIONS` | Server | `3` | Argon2id-Durchläufe (Untergrenze 2) |
| `auth.argon2id_parallelism` | `BOTHY_AUTH__ARGON2ID_PARALLELISM` | Server | `1` | Argon2id-Parallelität |
| `auth.login_fail_limit` | `BOTHY_AUTH__LOGIN_FAIL_LIMIT` | Server | `10` | Fehlversuche je Konto und Fenster |
| `auth.login_fail_window` | `BOTHY_AUTH__LOGIN_FAIL_WINDOW` | Server | `15m` | Fenster für Fehlversuche |
| `auth.login_lock_duration` | `BOTHY_AUTH__LOGIN_LOCK_DURATION` | Server | `15m` | Sperre des Passwortwegs |
| `auth.dpop_required_for` | `BOTHY_AUTH__DPOP_REQUIRED_FOR` | Server | `["agent"]` | Token-Arten, die DPoP-gebunden sein müssen |
| `auth.api_token_max_ttl` | `BOTHY_AUTH__API_TOKEN_MAX_TTL` | Server | `365d` | Höchstdauer eines Agenten- oder App-Tokens |
| `auth.personal_token_max_ttl` | `BOTHY_AUTH__PERSONAL_TOKEN_MAX_TTL` | Server | `90d` | Höchstdauer eines persönlichen Tokens |
| `client.app_lock_after` | `BOTHY_CLIENT__APP_LOCK_AFTER` | Server | `5m` | ohne Benutzung bis zur App-Sperre |
| `security.app_lock_required` | `BOTHY_SECURITY__APP_LOCK_REQUIRED` | Server | `false` | App-Sperre instanzweit verlangen |

### 09 Agenten

Quelle: [09-agenten.md](../spec/09-agenten.md), Abschnitt 21.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `agent.context_max_bytes` | `BOTHY_AGENT__CONTEXT_MAX_BYTES` | Server | `262144` | Summe der Inhalte im Kontext ohne auslösende Nachricht |
| `agent.context_message_max_bytes` | `BOTHY_AGENT__CONTEXT_MESSAGE_MAX_BYTES` | Server | `32768` | Inhalt je Kontext-Nachricht, darüber gekürzt |
| `agent.jwks_max_age` | `BOTHY_AGENT__JWKS_MAX_AGE` | Server | `1h` | Cache-Dauer der JWKS; Vorlauf eines `next`-Schlüssels |
| `signing_key.invocation_rotation_interval` | `BOTHY_SIGNING_KEY__INVOCATION_ROTATION_INTERVAL` | Server | `30d` | planmäßiger Wechsel des Markenschlüssels |
| `run.queued_max_age` | `BOTHY_RUN__QUEUED_MAX_AGE` | Server | `48h` | so alt darf ein nicht gestarteter Lauf werden |
| `run.orphan_timeout` | `BOTHY_RUN__ORPHAN_TIMEOUT` | Server | `15m` | ohne Heartbeat bei aktivem Lauf: `stale` |
| `interaction.ack_timeout` | `BOTHY_INTERACTION__ACK_TIMEOUT` | Server | `30s` | danach zeigt die Karte "nicht bestätigt" |
| `command.autocomplete_timeout` | `BOTHY_COMMAND__AUTOCOMPLETE_TIMEOUT` | Server | `2500ms` | Wartezeit auf Vorschläge |
| `command.autocomplete_max_choices` | `BOTHY_COMMAND__AUTOCOMPLETE_MAX_CHOICES` | Server | `25` | Vorschläge je Antwort |
| `command.max_per_principal` | `BOTHY_COMMAND__MAX_PER_PRINCIPAL` | Server | `100` | Befehle je Agent oder App |
| `command.description_max_chars` | `BOTHY_COMMAND__DESCRIPTION_MAX_CHARS` | Server | `100` | Länge einer Beschreibung |
| `component.max_per_message` | `BOTHY_COMPONENT__MAX_PER_MESSAGE` | Server | `25` | Komponenten je Nachricht |
| `component.custom_id_max_chars` | `BOTHY_COMPONENT__CUSTOM_ID_MAX_CHARS` | Server | `100` | Länge von `custom_id` |
| `component.form_max_fields` | `BOTHY_COMPONENT__FORM_MAX_FIELDS` | Server | `10` | Felder je Formular |
| `thread.placeholder_titles` | `BOTHY_THREAD__PLACEHOLDER_TITLES` | Server | `["", "Neuer Chat", "New chat", "Chat", "Ohne Titel", "Untitled", "Test"]` | Platzhalter für die Titel-Erkennung |
| `adapter.invocation_token_leeway` |  | Adapter | `10s` | Uhren-Toleranz bei `exp` und `iat`; höchstens `auth.clock_leeway` des Servers (4.5) |
| `adapter.jwks_refresh_min_interval` |  | Adapter | `60s` | höchstens so oft JWKS neu laden |
| `hermes_adapter.platform_key` |  | Hermes-Adapter | `bothy` | Plattformname in Hermes; nach Inbetriebnahme nie ändern |
| `hermes_adapter.consumer` |  | Hermes-Adapter | `default` | Verbrauchername am Gateway |
| `runner.claude_bin` |  | Runner | `claude` | Programm, gesucht im bereinigten `PATH` |
| `runner.projects_root` |  | Runner | kein Standard, Pflicht | Wurzel aller Projektordner |
| `runner.state_dir` |  | Runner | kein Standard, Pflicht | Zustand, Mitschnitte, Eingangsdateien |
| `runner.followup_grace` |  | Runner | `30s` | stdin bleibt nach `result` offen für Nachträge |
| `runner.stop_grace` |  | Runner | `10s` | nach SIGINT bis SIGTERM |
| `runner.kill_grace` |  | Runner | `5s` | nach SIGTERM bis SIGKILL |
| `runner.session_lock_wait` |  | Runner | `10m` | Wartezeit auf eine belegte Sitzung |
| `runner.max_parallel_runs` |  | Runner | `4` | gleichzeitige Turns |
| `runner.stream_log_retention` |  | Runner | `14d` | Aufbewahrung der Mitschnitte |
| `runner.attach_allowed_roots` |  | Runner | Projektordner, Eingangsordner, Runner-Temp | erlaubte Wurzeln für Antwort-Anhänge |
| `runner.attach_denied_patterns` |  | Runner | Liste in 18.10 | gesperrte Muster für Antwort-Anhänge |

### 10 Backend-Richtlinie

Quelle: [10-backend-richtlinie.md](../spec/10-backend-richtlinie.md), Abschnitt 17.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `billing.show_personal_estimates` | `BOTHY_BILLING__SHOW_PERSONAL_ESTIMATES` | Server | `false` | Beträge auch im Modus `personal` anzeigen, als Information |
| `billing.mismatch_alert_after` | `BOTHY_BILLING__MISMATCH_ALERT_AFTER` | Server | `5m` | so lange darf Ist von Soll abweichen, bevor ein Alarm ausgeht |
| `reconciler.min_request_interval` |  | Abgleich-App | `60s` | Abstand der Anfragen an die Cost-API |
| `reconciler.lookback` |  | Abgleich-App | `3d` | so viele Tage rückwärts wird abgeglichen |
| `reconciler.workspace_map` |  | Abgleich-App | leer | Zuordnung Arbeitsbereich zu Agent |

### 11 Erweiterungen: Apps, Webhooks, SDK

Quelle: [11-erweiterungen.md](../spec/11-erweiterungen.md), Abschnitt 16.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `app.manifest_max_bytes` | `BOTHY_APP__MANIFEST_MAX_BYTES` | Server | `65536` | Größe eines Manifests |
| `app.interaction_timeout` | `BOTHY_APP__INTERACTION_TIMEOUT` | Server | `3s` | Frist für synchrone Antworten über `interactions_url` |
| `webhook.delivery_timeout` | `BOTHY_WEBHOOK__DELIVERY_TIMEOUT` | Server | `15s` | Zeitgrenze einer Zustellung |
| `webhook.max_pending_per_subscription` | `BOTHY_WEBHOOK__MAX_PENDING_PER_SUBSCRIPTION` | Server | `10000` | offene Zustellungen je Abo, darüber `paused` |
| `webhook_in.max_body_bytes` | `BOTHY_WEBHOOK_IN__MAX_BODY_BYTES` | Server | `1048576` | Größe eines eingehenden Körpers |
| `webhook_in.formats_enabled` | `BOTHY_WEBHOOK_IN__FORMATS_ENABLED` | Server | `native, alertmanager` | eingeschaltete Eingangsformate |
| `sdk.http_timeout` |  | SDK | `30s` | Zeitgrenze einer REST-Anfrage |
| `sdk.retry_max_attempts` |  | SDK | `5` | Versuche je idempotenter Operation |
| `sdk.retry_backoff_base` |  | SDK | `500ms` | Basis des Backoffs |
| `sdk.retry_backoff_cap` |  | SDK | `30s` | größter Abstand |

### 12 Dateien

Quelle: [12-dateien.md](../spec/12-dateien.md), Abschnitt 18.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `file.storage.driver` | `BOTHY_FILE__STORAGE__DRIVER` | Server | `fs` | `fs` oder `s3` |
| `file.fs.root` | `BOTHY_FILE__FS__ROOT` | Server | `/var/lib/bothy/files` | Wurzel des Dateisystem-Treibers (Pfad im Container) |
| `file.fs.min_free_bytes` | `BOTHY_FILE__FS__MIN_FREE_BYTES` | Server | `2147483648` | freier Platz, unter dem Uploads mit `507` abgelehnt werden |
| `file.s3.endpoint` | `BOTHY_FILE__S3__ENDPOINT` | Server | leer | S3-Endpunkt |
| `file.s3.region` | `BOTHY_FILE__S3__REGION` | Server | `us-east-1` | Region für die Signatur |
| `file.s3.bucket` | `BOTHY_FILE__S3__BUCKET` | Server | leer | Bucket |
| `file.s3.prefix` | `BOTHY_FILE__S3__PREFIX` | Server | leer | Präfix vor allen Schlüsseln |
| `file.s3.use_path_style` | `BOTHY_FILE__S3__USE_PATH_STYLE` | Server | `true` | Pfad-Stil statt virtueller Hosts |
| `file.s3.checksum_mode` | `BOTHY_FILE__S3__CHECKSUM_MODE` | Server | `when_required` | SDK-Prüfsummen für Anfrage und Antwort |
| `file.s3.access_key_ref` | `BOTHY_FILE__S3__ACCESS_KEY_REF` | Server | `file:<secrets.dir>/s3_access_key` | Verweise auf den Secret-Provider; nur mit `file.storage.driver = s3` gelesen |
| `file.s3.secret_key_ref` | `BOTHY_FILE__S3__SECRET_KEY_REF` | Server | `file:<secrets.dir>/s3_secret_key` | Verweise auf den Secret-Provider; nur mit `file.storage.driver = s3` gelesen |
| `file.s3.presigned_upload` | `BOTHY_FILE__S3__PRESIGNED_UPLOAD` | Server | `false` | Modus `presigned` erlauben |
| `file.s3.presign_ttl` | `BOTHY_FILE__S3__PRESIGN_TTL` | Server | `15m` | Gültigkeit einer presigned URL |
| `file.upload_part_bytes` | `BOTHY_FILE__UPLOAD_PART_BYTES` | Server | `8388608` | Teilgröße |
| `file.upload_session_ttl` | `BOTHY_FILE__UPLOAD_SESSION_TTL` | Server | `24h` | Lebensdauer einer offenen Session |
| `file.upload_max_open_sessions` | `BOTHY_FILE__UPLOAD_MAX_OPEN_SESSIONS` | Server | `20` | offene Sessions je Prinzipal |
| `file.quota_bytes_per_principal` | `BOTHY_FILE__QUOTA_BYTES_PER_PRINCIPAL` | Server | `0` | Kontingent je Prinzipal, 0 = aus |
| `file.poster_max_bytes` | `BOTHY_FILE__POSTER_MAX_BYTES` | Server | `524288` | Größe des Video-Vorschaubilds vom Client |
| `file.sniff_bytes` | `BOTHY_FILE__SNIFF_BYTES` | Server | `1048576` | Bytes für die Typ-Erkennung |
| `file.denied_mime_types` | `BOTHY_FILE__DENIED_MIME_TYPES` | Server | leer | gesperrte Typen |
| `file.name_max_bytes` | `BOTHY_FILE__NAME_MAX_BYTES` | Server | `255` | Länge eines Dateinamens in UTF-8-Bytes |
| `file.scan.mode` | `BOTHY_FILE__SCAN__MODE` | Server | `required` | `required` oder `off` |
| `file.scan.clamd_address` | `BOTHY_FILE__SCAN__CLAMD_ADDRESS` | Server | `tcp://clamav:3310` | clamd im internen Netz; alternativ `unix:///...` |
| `file.scan.chunk_bytes` | `BOTHY_FILE__SCAN__CHUNK_BYTES` | Server | `65536` | Stückgröße für `INSTREAM` |
| `file.scan.timeout` | `BOTHY_FILE__SCAN__TIMEOUT` | Server | `120s` | Zeitlimit je Scan |
| `file.scan.max_concurrent` | `BOTHY_FILE__SCAN__MAX_CONCURRENT` | Server | `4` | Scans je Worker-Prozess |
| `file.scan.retry_backoff_cap` | `BOTHY_FILE__SCAN__RETRY_BACKOFF_CAP` | Server | `5m` | größter Abstand zwischen Scan-Versuchen |
| `file.scan.health_interval` | `BOTHY_FILE__SCAN__HEALTH_INTERVAL` | Server | `30s` | `PING`-Abstand |
| `file.scan.pending_alert_after` | `BOTHY_FILE__SCAN__PENDING_ALERT_AFTER` | Server | `10m` | Alter der ältesten `pending`-Datei bis zum Alarm |
| `file.scan.clamd_stream_max_bytes` | `BOTHY_FILE__SCAN__CLAMD_STREAM_MAX_BYTES` | Server | `104857600` | Spiegel von clamd `StreamMaxLength` |
| `file.invocation_hold_max` | `BOTHY_FILE__INVOCATION_HOLD_MAX` | Server | `120s` | längstes Warten des Agenten-Aufrufs auf Anhänge |
| `file.image.libvips_enabled` | `BOTHY_FILE__IMAGE__LIBVIPS_ENABLED` | Server | `false` | libvips-Build nutzen |
| `file.image.max_pixels` | `BOTHY_FILE__IMAGE__MAX_PIXELS` | Server | `50000000` | größte dekodierte Pixelzahl |
| `file.image.jpeg_quality` | `BOTHY_FILE__IMAGE__JPEG_QUALITY` | Server | `90` | Qualität beim Umwandeln unbewegter WebP-Bilder ohne Alpha in JPEG |
| `file.image.processing_timeout` | `BOTHY_FILE__IMAGE__PROCESSING_TIMEOUT` | Server | `60s` | Zeitlimit eines Bild-Jobs |
| `file.image.max_concurrent` | `BOTHY_FILE__IMAGE__MAX_CONCURRENT` | Server | `2` | Bild-Jobs je Worker-Prozess |
| `file.thumbnail.small_px` | `BOTHY_FILE__THUMBNAIL__SMALL_PX` | Server | `320` | längste Kante `small` |
| `file.thumbnail.medium_px` | `BOTHY_FILE__THUMBNAIL__MEDIUM_PX` | Server | `1024` | längste Kante `medium` |
| `file.thumbnail.large_px` | `BOTHY_FILE__THUMBNAIL__LARGE_PX` | Server | `2048` | längste Kante `large` |
| `file.thumbnail.jpeg_quality` | `BOTHY_FILE__THUMBNAIL__JPEG_QUALITY` | Server | `82` | Qualität der Vorschaubilder |
| `file.ticket_ttl` | `BOTHY_FILE__TICKET_TTL` | Server | `5m` | Gültigkeit eines Abruf-Tickets |
| `file.ticket_key_ref` | `BOTHY_FILE__TICKET_KEY_REF` | Server | `file:<secrets.dir>/file_ticket_key` | Verweis auf den Ticket-Schlüssel im Secret-Provider |
| `file.unattached_ttl` | `BOTHY_FILE__UNATTACHED_TTL` | Server | `24h` | Frist für nie angehängte Dateien |
| `file.failed_row_ttl` | `BOTHY_FILE__FAILED_ROW_TTL` | Server | `7d` | Frist für Zeilen fehlgeschlagener oder blockierter Dateien |
| `file.gc_interval` | `BOTHY_FILE__GC_INTERVAL` | Server | `10m` | Abstand des Aufräum-Jobs |
| `voice.max_duration` | `BOTHY_VOICE__MAX_DURATION` | Server | `15m` | längste Sprachnachricht |
| `voice.transcription_mode` | `BOTHY_VOICE__TRANSCRIPTION_MODE` | Server | `agent` | `off`, `agent` oder `service` |
| `voice.transcription_url` | `BOTHY_VOICE__TRANSCRIPTION_URL` | Server | leer | Endpunkt des Transkriptions-Dienstes |
| `voice.transcription_api_key_ref` | `BOTHY_VOICE__TRANSCRIPTION_API_KEY_REF` | Server | leer | Verweis auf den Schlüssel |
| `voice.transcription_timeout` | `BOTHY_VOICE__TRANSCRIPTION_TIMEOUT` | Server | `120s` | Zeitlimit einer Transkription |
| `voice.transcript_hold_max` | `BOTHY_VOICE__TRANSCRIPT_HOLD_MAX` | Server | `60s` | längstes Warten des Agenten-Aufrufs auf das Transkript |

### 13 Push und Benachrichtigungen

Quelle: [13-push-benachrichtigungen.md](../spec/13-push-benachrichtigungen.md), Abschnitt 17.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `push.ttl` | `BOTHY_PUSH__TTL` | Server | `24h` | Lebensdauer beim Anbieter (`apns-expiration`, `android.ttl`) |
| `push.max_age` | `BOTHY_PUSH__MAX_AGE` | Server | `1h` | älter wird ein noch nicht gesendeter Hinweis verworfen |
| `push.retry_base` | `BOTHY_PUSH__RETRY_BASE` | Server | `2s` | Basis des Backoffs |
| `push.retry_cap` | `BOTHY_PUSH__RETRY_CAP` | Server | `5m` | größter Backoff-Abstand |
| `push.max_concurrent_requests` | `BOTHY_PUSH__MAX_CONCURRENT_REQUESTS` | Server | `32` | gleichzeitige Anfragen an Anbieter je Prozess |
| `push.apns.team_id` | `BOTHY_PUSH__APNS__TEAM_ID` | Server | leer | Apple-Team-ID |
| `push.apns.key_id` | `BOTHY_PUSH__APNS__KEY_ID` | Server | leer | Key-ID des `.p8`-Schlüssels |
| `push.apns.private_key_ref` | `BOTHY_PUSH__APNS__PRIVATE_KEY_REF` | Server | leer | Verweis auf den Schlüssel im Secret-Provider |
| `push.apns.topics` | `BOTHY_PUSH__APNS__TOPICS` | Server | leer | erlaubte Bundle-IDs |
| `push.apns.jwt_refresh_interval` | `BOTHY_PUSH__APNS__JWT_REFRESH_INTERVAL` | Server | `40m` | Erneuerung des Anbieter-JWT, muss zwischen 20 und 60 Minuten liegen |
| `push.apns.retry_5xx_delay` | `BOTHY_PUSH__APNS__RETRY_5XX_DELAY` | Server | `15m` | Wartezeit nach `500` oder `503` |
| `push.apns.time_sensitive_enabled` | `BOTHY_PUSH__APNS__TIME_SENSITIVE_ENABLED` | Server | `false` | `time-sensitive` für `input_needed` |
| `push.fcm.project_id` | `BOTHY_PUSH__FCM__PROJECT_ID` | Server | leer | Firebase-Projekt |
| `push.fcm.service_account_ref` | `BOTHY_PUSH__FCM__SERVICE_ACCOUNT_REF` | Server | leer | Verweis auf den Dienstkonto-Schlüssel |
| `notify.user_default_level` | `BOTHY_NOTIFY__USER_DEFAULT_LEVEL` | Server | `mentions` | Stufe ohne jede Einstellung |
| `notify.default_preview_mode` | `BOTHY_NOTIFY__DEFAULT_PREVIEW_MODE` | Server | `full` | Standard für `notify.preview_mode` |
| `notify.default_input_needed_breaks_quiet_hours` | `BOTHY_NOTIFY__DEFAULT_INPUT_NEEDED_BREAKS_QUIET_HOURS` | Server | `false` | Standard für den gleichnamigen Schalter |
| `notify.default_admin_enabled` | `BOTHY_NOTIFY__DEFAULT_ADMIN_ENABLED` | Server | `true` | Standard für `notify.admin_enabled` |
| `notify.coalesce_window` | `BOTHY_NOTIFY__COALESCE_WINDOW` | Server | `60s` | Bündelfenster je Empfänger und Thread |
| `notify.mobile_delay_when_desktop_active` | `BOTHY_NOTIFY__MOBILE_DELAY_WHEN_DESKTOP_ACTIVE` | Server | `60s` | Verzögerung mobiler Pushes bei aktivem Desktop |
| `notify.preview_max_chars` | `BOTHY_NOTIFY__PREVIEW_MAX_CHARS` | Server | `240` | Länge des Auszugs in der Vorschau |
| `notify.badge_sync_min_interval` | `BOTHY_NOTIFY__BADGE_SYNC_MIN_INTERVAL` | Server | `60s` | kleinster Abstand der Badge-Abgleiche je Person |

### 14 Suche

Quelle: [14-suche.md](../spec/14-suche.md), Abschnitt 15.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `search.languages` | `BOTHY_SEARCH__LANGUAGES` | Server | `["german","english"]` | Konfigurationen für den Anfrage-Aufbau (plus immer `simple`) |
| `search.locale_map` | `BOTHY_SEARCH__LOCALE_MAP` | Server | `{"de":"german","en":"english"}` | Sprachkennung zu Textsuche-Konfiguration |
| `search.query_max_chars` | `BOTHY_SEARCH__QUERY_MAX_CHARS` | Server | `256` | Länge der Eingabe |
| `search.max_candidates` | `BOTHY_SEARCH__MAX_CANDIDATES` | Server | `2000` | Kandidaten vor dem Ranking |
| `search.rank_normalization` | `BOTHY_SEARCH__RANK_NORMALIZATION` | Server | `1` | Normalisierung für `ts_rank` |
| `search.recency_scale` | `BOTHY_SEARCH__RECENCY_SCALE` | Server | `30d` | Alter, bei dem der Wert auf die Hälfte fällt |
| `search.page_size_default` | `BOTHY_SEARCH__PAGE_SIZE_DEFAULT` | Server | `20` | Treffer je Seite |
| `search.page_size_max` | `BOTHY_SEARCH__PAGE_SIZE_MAX` | Server | `50` | höchstens Treffer je Seite |
| `search.headline_input_max_chars` | `BOTHY_SEARCH__HEADLINE_INPUT_MAX_CHARS` | Server | `20000` | Länge des Textes für `ts_headline` |
| `search.headline_max_fragments` | `BOTHY_SEARCH__HEADLINE_MAX_FRAGMENTS` | Server | `2` | Ausschnitte je Treffer |
| `search.headline_max_words` | `BOTHY_SEARCH__HEADLINE_MAX_WORDS` | Server | `30` | Wörter je Ausschnitt, höchstens |
| `search.headline_min_words` | `BOTHY_SEARCH__HEADLINE_MIN_WORDS` | Server | `10` | Wörter je Ausschnitt, mindestens |
| `search.prefix_min_chars` | `BOTHY_SEARCH__PREFIX_MIN_CHARS` | Server | `2` | kürzeste Wortlänge in der Präfix-Suche |
| `search.statement_timeout` | `BOTHY_SEARCH__STATEMENT_TIMEOUT` | Server | `3s` | Zeitlimit einer Suchabfrage |
| `search.p95_target` | `BOTHY_SEARCH__P95_TARGET` | Server | `500ms` | Ziel p95 im Lasttest |
| `search.prefix_p95_target` | `BOTHY_SEARCH__PREFIX_P95_TARGET` | Server | `100ms` | Ziel p95 der Präfix-Suche |
| `search.benchmark_messages` | `BOTHY_SEARCH__BENCHMARK_MESSAGES` | Server | `2000000` | Korpusgröße im Lasttest |
| `search.extract.max_input_bytes` | `BOTHY_SEARCH__EXTRACT__MAX_INPUT_BYTES` | Server | `52428800` | größte Datei für die Extraktion |
| `search.extract.max_text_bytes` | `BOTHY_SEARCH__EXTRACT__MAX_TEXT_BYTES` | Server | `1048576` | größter gespeicherter Text |
| `search.extract.max_uncompressed_bytes` | `BOTHY_SEARCH__EXTRACT__MAX_UNCOMPRESSED_BYTES` | Server | `209715200` | größte entpackte Menge bei Archiven |
| `search.extract.max_archive_entries` | `BOTHY_SEARCH__EXTRACT__MAX_ARCHIVE_ENTRIES` | Server | `10000` | Einträge je Archiv |
| `search.extract.timeout` | `BOTHY_SEARCH__EXTRACT__TIMEOUT` | Server | `60s` | Zeitlimit je Extraktion |
| `search.extract.tika_url` | `BOTHY_SEARCH__EXTRACT__TIKA_URL` | Server | leer | Tika-Server; leer = aus |

### 15 Client-Architektur

Quelle: [15-client-architektur.md](../spec/15-client-architektur.md), Abschnitt 22.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `client.cache_max_bytes` |  | App | `1GiB` | Obergrenze für lokale Datenbank plus Datei-Cache; darüber werden die ältesten Einträge entfernt |
| `client.sync.parse_offload_bytes` |  | App | `262144` | ab dieser Seitengröße wird im Hintergrund-Isolate dekodiert |
| `client.outbox_stuck_after` |  | App | `2m` | ab dann zeigt ein wartender Eintrag "Senden verzögert" |
| `client.push.fetch_timeout` |  | App | `20s` | Zeit, die NSE bzw. Android-Dienst für das Holen des Texts nutzen |
| `client.voice.bitrate` |  | App | `64kbps` | Bitrate der AAC-Aufnahme |
| `client.metrics.flush_interval` |  | App | `5m` | höchstens so oft sendet der Client Kennzahlen |
| `client.perf.cold_start_p95` |  | App | `2500ms` | Ziel Kaltstart |
| `client.perf.thread_open_p95` |  | App | `300ms` | Ziel Thread öffnen aus dem Cache |
| `client.perf.send_local_echo_p95` |  | App | `50ms` | Ziel lokale Anzeige beim Senden |
| `client.perf.patch_to_frame_p95` |  | App | `100ms` | Ziel Live-Box-Patch bis Frame |
| `client.perf.jank_ratio_max` |  | App | `0.01` | höchster Anteil Frames über Budget |
| `client.perf.catchup_events_per_s_min` |  | App | `2000` | Mindestrate beim Nachholen |
| `client.perf.memory_mobile_max` |  | App | `300MiB` | Speicher mobil |
| `client.perf.memory_desktop_max` |  | App | `500MiB` | Speicher Desktop |

### 16 Betrieb

Quelle: [16-betrieb.md](../spec/16-betrieb.md), Abschnitt 14.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `instance.environment` | `BOTHY_INSTANCE__ENVIRONMENT` | Server | `prod` | `dev`, `staging` oder `prod` (6.1) |
| `server.role` | `BOTHY_SERVER__ROLE` | Server | `all` | `api`, `worker` oder `all` |
| `server.listen_addr` | `BOTHY_SERVER__LISTEN_ADDR` | Server | `127.0.0.1:8080` | öffentlicher Listener (REST, Gateway) |
| `server.internal_listen_addr` | `BOTHY_SERVER__INTERNAL_LISTEN_ADDR` | Server | `127.0.0.1:8081` | Health, Readiness, Eingangs-Webhooks aus dem Stack |
| `server.public_url` | `BOTHY_SERVER__PUBLIC_URL` | Server | keiner, Pflicht | Basis-URL, unter der Apps den Server erreichen |
| `server.trusted_proxies` | `BOTHY_SERVER__TRUSTED_PROXIES` | Server | `127.0.0.1/32, ::1/128` | Gegenstellen, deren Weiterleitungs- und Identitäts-Kopfzeilen gelten und die Klartext senden dürfen ([19-sicherheit.md](../spec/19-sicherheit.md), SEC-TLS-1) |
| `server.maintenance_mode` | `BOTHY_SERVER__MAINTENANCE_MODE` | Server | `off` | `off` oder `read_only` (Umzug, Upgrade) |
| `secrets.dir` | `BOTHY_SECRETS__DIR` | Server | `/run/bothy/secrets` | Basis der Standardverweise `file:` |
| `secrets.cache_ttl` | `BOTHY_SECRETS__CACHE_TTL` | Server | `5m` | so lange gilt ein gelesenes Geheimnis |
| `secrets.signing_key_dir` | `BOTHY_SECRETS__SIGNING_KEY_DIR` | Server | `/var/lib/bothy/signing-keys` | beschreibbares Verzeichnis der privaten Signierschlüssel; im Besitz des Dienstbenutzers, Modus `0700` (5.2, ADR 0066) |
| `server.tls_cert_ref` | `BOTHY_SERVER__TLS_CERT_REF` | Server | leer | Verweis auf die Zertifikatskette (PEM, Blatt zuerst); leer: der Server beendet kein TLS (SEC-TLS-2) |
| `server.tls_key_ref` | `BOTHY_SERVER__TLS_KEY_REF` | Server | leer | Verweis auf den privaten Schlüssel (PEM); nur zusammen mit `server.tls_cert_ref` |
| `database.host` | `BOTHY_DATABASE__HOST` | Server | `localhost` | Host der Datenbank |
| `database.port` | `BOTHY_DATABASE__PORT` | Server | `5432` | Port der Datenbank im Netz des Servers |
| `database.name` | `BOTHY_DATABASE__NAME` | Server | `bothy` | Datenbankname |
| `database.user` | `BOTHY_DATABASE__USER` | Server | `bothy_app` | Rolle des Servers |
| `database.password_ref` | `BOTHY_DATABASE__PASSWORD_REF` | Server | `file:<secrets.dir>/db_app_password` | Verweis auf das Passwort |
| `database.sslmode` | `BOTHY_DATABASE__SSLMODE` | Server | `verify-full` | TLS zur Datenbank; im internen Compose-Netz `disable` (SEC-TLS-5) |
| `database.sslrootcert` | `BOTHY_DATABASE__SSLROOTCERT` | Server | leer | CA-Bündel für `verify-full`; leer nutzt die Wurzeln des Systems |
| `database.max_conns` | `BOTHY_DATABASE__MAX_CONNS` | Server | `20` | Größe des Verbindungspools je Prozess (Startwert bis zum Lasttest) |
| `migrate.require_recent_backup` | `BOTHY_MIGRATE__REQUIRE_RECENT_BACKUP` | Server | `true` | `migrate` verlangt eine frische Sicherung |
| `migrate.recent_backup_max_age` | `BOTHY_MIGRATE__RECENT_BACKUP_MAX_AGE` | Server | `24h` | so alt darf diese Sicherung sein |
| `migrate.lock_timeout` | `BOTHY_MIGRATE__LOCK_TIMEOUT` | Server | `5s` | `lock_timeout` je Migrationsschritt |
| `migrate.bootstrap_user` | `BOTHY_MIGRATE__BOOTSTRAP_USER` | Server | `postgres` | Superuser-Anmeldung nur für `migrate up --bootstrap` |
| `migrate.bootstrap_password_ref` | `BOTHY_MIGRATE__BOOTSTRAP_PASSWORD_REF` | Server | `file:<secrets.dir>/db_superuser_password` | dessen Passwort |
| `migrate.owner_password_ref` | `BOTHY_MIGRATE__OWNER_PASSWORD_REF` | Server | `file:<secrets.dir>/db_owner_password` | Passwort, das `--bootstrap` für `bothy_owner` setzt |
| `migrate.app_password_ref` | `BOTHY_MIGRATE__APP_PASSWORD_REF` | Server | `file:<secrets.dir>/db_app_password` | Passwort für `bothy_app` |
| `migrate.maint_password_ref` | `BOTHY_MIGRATE__MAINT_PASSWORD_REF` | Server | `file:<secrets.dir>/db_maint_password` | Passwort für `bothy_maint` |
| `migrate.monitor_password_ref` | `BOTHY_MIGRATE__MONITOR_PASSWORD_REF` | Server | `file:<secrets.dir>/db_monitor_password` | Passwort für `bothy_monitor` |
| `backup.pg.full_schedule` | `BOTHY_BACKUP__PG__FULL_SCHEDULE` | Server | `0 2 * * 0` | Plan der Vollsicherung (cron) |
| `backup.pg.diff_schedule` | `BOTHY_BACKUP__PG__DIFF_SCHEDULE` | Server | `0 2 * * 1-6` | Plan der Differenzsicherung (cron) |
| `backup.pg.archive_timeout` | `BOTHY_BACKUP__PG__ARCHIVE_TIMEOUT` | Server | `60s` | spätestens dann wird ein WAL-Segment archiviert |
| `backup.pg.retention_full` | `BOTHY_BACKUP__PG__RETENTION_FULL` | Server | `4` | behaltene Vollsicherungen in repo2 |
| `backup.pg.local_retention_full` | `BOTHY_BACKUP__PG__LOCAL_RETENTION_FULL` | Server | `1` | behaltene Vollsicherungen in repo1 |
| `backup.logical.schedule` | `BOTHY_BACKUP__LOGICAL__SCHEDULE` | Server | `0 3 * * *` | Plan von `pg_dump` (cron) |
| `backup.logical.retention` | `BOTHY_BACKUP__LOGICAL__RETENTION` | Server | `30d` | Aufbewahrung der logischen Sicherung |
| `backup.objects.schedule` | `BOTHY_BACKUP__OBJECTS__SCHEDULE` | Server | `30 3 * * *` | Plan der Objektsicherung (cron) |
| `backup.objects.retention` | `BOTHY_BACKUP__OBJECTS__RETENTION` | Server | `90d` | Aufbewahrung der Objekt-Snapshots |
| `backup.offsite.target` | `BOTHY_BACKUP__OFFSITE__TARGET` | Server | leer | Off-site-Ziel; leer heißt nur lokal (Warnung in Konsole und Status) |
| `backup.max_age` | `BOTHY_BACKUP__MAX_AGE` | Server | `26h` | ohne erfolgreiche Sicherung länger als das: Alarm |
| `backup.restore_test.schedule` | `BOTHY_BACKUP__RESTORE_TEST__SCHEDULE` | Server | `0 5 * * 6` | Plan des Wiederherstellungstests (cron) |
| `backup.restore_test.sample_files` | `BOTHY_BACKUP__RESTORE_TEST__SAMPLE_FILES` | Server | `200` | geprüfte Dateien je Test |
| `backup.restore_test.sample_threads` | `BOTHY_BACKUP__RESTORE_TEST__SAMPLE_THREADS` | Server | `200` | geprüfte Threads je Test |
| `backup.restore_test.max_age` | `BOTHY_BACKUP__RESTORE_TEST__MAX_AGE` | Server | `8d` | ohne erfolgreichen Test länger als das: Alarm |
| `dr.rpo_target` | `BOTHY_DR__RPO_TARGET` | Server | `5m` | Ziel für den größten Datenverlust |
| `dr.rto_target` | `BOTHY_DR__RTO_TARGET` | Server | `4h` | Ziel für die Dauer der Wiederherstellung |
| `status.check_interval` | `BOTHY_STATUS__CHECK_INTERVAL` | Server | `30s` | Abstand der automatischen Prüfungen |
| `status.degrade_after` | `BOTHY_STATUS__DEGRADE_AFTER` | Server | `2m` | so lange Fehler bis `degraded` |
| `status.incident_after` | `BOTHY_STATUS__INCIDENT_AFTER` | Server | `5m` | so lange `major_outage` bis zur automatischen Störung |
| `status.resolve_after` | `BOTHY_STATUS__RESOLVE_AFTER` | Server | `10m` | so lange gesund bis zur automatischen Auflösung |
| `status.error_ratio_degraded` | `BOTHY_STATUS__ERROR_RATIO_DEGRADED` | Server | `0.05` | Fehleranteil, ab dem eine Prüfung scheitert |
| `status.error_ratio_outage` | `BOTHY_STATUS__ERROR_RATIO_OUTAGE` | Server | `0.5` | Fehleranteil, ab dem `major_outage` gilt |
| `status.fanout_lag_degraded` | `BOTHY_STATUS__FANOUT_LAG_DEGRADED` | Server | `5s` | Fan-out-Verzug, ab dem `sync.fanout` scheitert |

### 17 Observability

Quelle: [17-observability.md](../spec/17-observability.md), Abschnitt 11.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `telemetry.enabled` | `BOTHY_TELEMETRY__ENABLED` | Server | `true` | Telemetrie an oder aus |
| `telemetry.traces.sampler_ratio` | `BOTHY_TELEMETRY__TRACES__SAMPLER_RATIO` | Server | `1.0` | Anteil der Traces, die das SDK aufzeichnet; der Collector sampelt danach nach |
| `telemetry.traces.trust_remote_parent` | `BOTHY_TELEMETRY__TRACES__TRUST_REMOTE_PARENT` | Server | `false` | `true`: ein eingehender `traceparent` wird Eltern-Kontext (seine Trace-ID wird `correlation_id`, sein Sampling-Flag gilt); `false`: nur Span-Link, eigener Trace |
| `telemetry.logs.exporter` | `BOTHY_TELEMETRY__LOGS__EXPORTER` | Server | `both` | `otlp`, `stdout` oder `both` |
| `telemetry.logs.level` | `BOTHY_TELEMETRY__LOGS__LEVEL` | Server | `info` | kleinste Stufe, die geschrieben wird |
| `telemetry.pseudonym_key_ref` | `BOTHY_TELEMETRY__PSEUDONYM_KEY_REF` | Server | `file:<secrets.dir>/telemetry_pseudonym_key` | Schlüssel für HMAC-Pseudonyme; mindestens 32 Byte |
| `telemetry.dashboard_url` | `BOTHY_TELEMETRY__DASHBOARD_URL` | Server | leer | Link aus der Admin-Konsole zum Telemetrie-Backend |
| `stats.cache_ttl` | `BOTHY_STATS__CACHE_TTL` | Server | `5m` | so lange gilt ein berechnetes Statistik-Ergebnis |
| `stats.max_range` | `BOTHY_STATS__MAX_RANGE` | Server | `400d` | größter abfragbarer Zeitraum |

### 18 Release

Quelle: [18-release.md](../spec/18-release.md), Abschnitt 12.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `client.min_version.ios` | `BOTHY_CLIENT__MIN_VERSION__IOS` | Server | leer | kleinste zugelassene App-Version auf iOS; leer heißt keine Grenze |
| `client.min_version.android` | `BOTHY_CLIENT__MIN_VERSION__ANDROID` | Server | leer | dasselbe für Android |
| `client.min_version.macos` | `BOTHY_CLIENT__MIN_VERSION__MACOS` | Server | leer | dasselbe für macOS |
| `client.min_version.windows` | `BOTHY_CLIENT__MIN_VERSION__WINDOWS` | Server | leer | dasselbe für Windows |
| `client.recommended_version.ios` | `BOTHY_CLIENT__RECOMMENDED_VERSION__IOS` | Server | leer | darunter zeigt die App einen Update-Hinweis |
| `client.recommended_version.android` | `BOTHY_CLIENT__RECOMMENDED_VERSION__ANDROID` | Server | leer | dasselbe für Android |
| `client.recommended_version.macos` | `BOTHY_CLIENT__RECOMMENDED_VERSION__MACOS` | Server | leer | dasselbe für macOS |
| `client.recommended_version.windows` | `BOTHY_CLIENT__RECOMMENDED_VERSION__WINDOWS` | Server | leer | dasselbe für Windows |

### 19 Sicherheit

Quelle: [19-sicherheit.md](../spec/19-sicherheit.md), Abschnitt 16.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `http.hsts_max_age` | `BOTHY_HTTP__HSTS_MAX_AGE` | Server | `31536000s` | `max-age` für HSTS |
| `egress.allowed_hosts` | `BOTHY_EGRESS__ALLOWED_HOSTS` | Server | leer | zusätzliche erlaubte Ziele, Form `host[:port][;allow_private]` ([configuration.md](../operations/configuration.md), Abschnitt 2); doppelte Einträge sind ein Ladefehler |
| `egress.timeout` | `BOTHY_EGRESS__TIMEOUT` | Server | `15s` | Zeitgrenze einer ausgehenden Anfrage |
| `egress.max_response_bytes` | `BOTHY_EGRESS__MAX_RESPONSE_BYTES` | Server | `1048576` | größte Antwort einer ausgehenden Anfrage |
| `audit.checkpoint_interval` | `BOTHY_AUDIT__CHECKPOINT_INTERVAL` | Server | `1h` | Abstand der signierten Prüfpunkte des Audit-Logs |
| `audit.checkpoint_sink` | `BOTHY_AUDIT__CHECKPOINT_SINK` | Server | leer | Ablageziel der Prüfpunkte außerhalb der Datenbank (z. B. Objektspeicher-Präfix mit Aufbewahrungssperre); leer erzeugt eine Warnung |
| `signing_key.cache_ttl` | `BOTHY_SIGNING_KEY__CACHE_TTL` | Server | `30s` | längste Zeit, die ein Prozess einen zurückgezogenen Schlüssel noch annimmt oder einen neuen nicht kennt |
| `signing_key.rotation_check_interval` | `BOTHY_SIGNING_KEY__ROTATION_CHECK_INTERVAL` | Server | `1m` | Abstand des Jobs `instance.key_rotation` |
| `signing_key.access_token_rotation_interval` | `BOTHY_SIGNING_KEY__ACCESS_TOKEN_ROTATION_INTERVAL` | Server | `30d` | planmäßiger Wechsel des Schlüssels für Zugriffstokens |

### 20 Datenschutz

Quelle: [20-datenschutz.md](../spec/20-datenschutz.md), Abschnitt 13.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `retention.content` | `BOTHY_RETENTION__CONTENT` | Server | `none` | Frist für Inhalte; `none` heißt keine automatische Löschung (Konsole verlangt Bestätigung) |
| `retention.security_after_end` | `BOTHY_RETENTION__SECURITY_AFTER_END` | Server | `90d` | Sicherheitsdaten nach Ende, Ablauf oder Widerruf |
| `retention.audit` | `BOTHY_RETENTION__AUDIT` | Server | `365d` | Audit-Log (Studie C schlug zwölf Monate vor) |
| `retention.cost` | `BOTHY_RETENTION__COST` | Server | `395d` | Kosten und Preise |
| `retention.import_after_completion` | `BOTHY_RETENTION__IMPORT_AFTER_COMPLETION` | Server | `90d` | Import-Zustand nach Abschluss |
| `retention.privacy_job` | `BOTHY_RETENTION__PRIVACY_JOB` | Server | `1095d` | DSGVO-Aufträge (ohne Exportpakete) |
| `retention.ops` | `BOTHY_RETENTION__OPS` | Server | `395d` | Statusseite |
| `retention.webhook_deliveries` | `BOTHY_RETENTION__WEBHOOK_DELIVERIES` | Server | `30d` | Zustellversuche ausgehender Webhooks |
| `privacy.export_ttl` | `BOTHY_PRIVACY__EXPORT_TTL` | Server | `7d` | Bereitstellung eines Exportpakets |
| `privacy.export_requires_approval` | `BOTHY_PRIVACY__EXPORT_REQUIRES_APPROVAL` | Server | `false` | Export der eigenen Daten braucht Freigabe des Owners |
| `privacy.account_deletion_content` | `BOTHY_PRIVACY__ACCOUNT_DELETION_CONTENT` | Server | `keep_pseudonymized` | Inhalte gelöschter Konten: `keep_pseudonymized` oder `delete` |
| `privacy.object_delete_delay` | `BOTHY_PRIVACY__OBJECT_DELETE_DELAY` | Server | `24h` | höchste Zeit bis zum Entfernen gelöschter Objekte |
| `privacy.deletion_journal_retention` | `BOTHY_PRIVACY__DELETION_JOURNAL_RETENTION` | Server | `400d` | Aufbewahrung des Lösch-Journals; mindestens Sicherungsfrist |
| `privacy.audit_ip_key_period` | `BOTHY_PRIVACY__AUDIT_IP_KEY_PERIOD` | Server | `30d` | Zeitraum je Schlüssel für IP-Pseudonyme |
| `privacy.audit_ip_key_retention` | `BOTHY_PRIVACY__AUDIT_IP_KEY_RETENTION` | Server | `90d` | danach werden Schlüssel vernichtet |
| `privacy.audit_ip_kek_ref` | `BOTHY_PRIVACY__AUDIT_IP_KEK_REF` | Server | `file:<secrets.dir>/audit_ip_kek` | Verweis auf den Schlüssel, mit dem die IP-Schlüssel in `audit_ip_keys` verschlüsselt sind |
| `privacy.telemetry_pseudonym_rotation` | `BOTHY_PRIVACY__TELEMETRY_PSEUDONYM_ROTATION` | Server | `30d` | Wechsel des Schlüssels für pseudonyme IDs in Telemetrie |
| `privacy.per_person_stats_enabled` | `BOTHY_PRIVACY__PER_PERSON_STATS_ENABLED` | Server | `false` | Statistik je Person |
| `privacy.stats_min_group_size` | `BOTHY_PRIVACY__STATS_MIN_GROUP_SIZE` | Server | `5` | kleinste Gruppe für Auswertungen nach Personen |
| `privacy.ai_notice_enabled` | `BOTHY_PRIVACY__AI_NOTICE_ENABLED` | Server | `true` | einmaliger KI-Hinweis je Kanal mit Agent |
| `privacy.notice_url` | `BOTHY_PRIVACY__NOTICE_URL` | Server | leer | Link zur Datenschutzinformation des Betreibers |

### 21 Mehrbenutzer-Dev-Umgebung und Abschottung

Quelle: [21-dev-isolation.md](../spec/21-dev-isolation.md), Abschnitt 12.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `dev.reverify_interval` | `BOTHY_DEV__REVERIFY_INTERVAL` | Server | `24h` | spätestens dann wird eine Umgebung neu geprüft |
| `dev.limits` | `BOTHY_DEV__LIMITS` | Server | `{"cpus": 4, "memory_mib": 8192, "disk_gib": 50, "pids": 2048}` | Grenzen je Umgebung |
| `dev.egress_allowlist` | `BOTHY_DEV__EGRESS_ALLOWLIST` | Server | leer | erlaubte Ziele (Hosts) für Umgebungen; ohne Eintrag kein Egress |

### 22 Import und Migration aus Discord

Quelle: [22-import-migration.md](../spec/22-import-migration.md), Abschnitt 19.

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `import.discord.api_base` |  | Werkzeug `bothy import` | `https://discord.com/api/v10` | Discord-API-Basis [R6, belegt: v10 aktuell] |
| `import.discord.cdn_hosts` |  | Werkzeug `bothy import` | `cdn.discordapp.com`, `media.discordapp.net` | erlaubte Hosts für Anhänge |
| `import.discord.page_size` |  | Werkzeug `bothy import` | `100` | Nachrichten je Seite; Discord erlaubt höchstens 100 [R4, belegt] |
| `import.discord.max_429_retries` |  | Werkzeug `bothy import` | `5` | Wiederholungen nach `429` je Aufruf |
| `import.discord.max_rate_wait` |  | Werkzeug `bothy import` | `60s` | größte Wartezeit, bevor der Lauf abbricht |
| `import.discord.preflight_sample` |  | Werkzeug `bothy import` | `20` | Nachrichten für die Intent-Probe |
| `import.discord.closed_tags` |  | Werkzeug `bothy import` | leer | Forum-Tags, die einen Thread zusätzlich als erledigt markieren |
| `import.attachment_max_bytes` |  | Werkzeug `bothy import` | wie `file.max_upload_bytes` | größter übernommener Anhang |
| `import.http_timeout` |  | Werkzeug `bothy import` | `60s` | Zeitlimit je Aufruf an Discord und CDN |
| `import.batch_max_items` |  | Werkzeug `bothy import` | `100` | Objekte je Stapel an den Server |
| `import.reply_quote_max_chars` |  | Werkzeug `bothy import` | `100` | Länge der Zitatzeile bei Antworten über Threads hinweg |
| `import.follow.poll_interval` |  | Werkzeug `bothy import` | `30s` | Abstand der Spiegel-Runden |
| `import.follow.settle_after` |  | Werkzeug `bothy import` | `60s` | Ruhezeit nach letzter Änderung |
| `import.follow.busy_reactions` |  | Werkzeug `bothy import` | `👀` | Reaktionen, die "Agent arbeitet noch" bedeuten |
| `import.follow.busy_lookback` |  | Werkzeug `bothy import` | `10` | Nachrichten vor der ersten neuen, die auf Markierung geprüft werden |
| `import.follow.max_backoff` |  | Werkzeug `bothy import` | `5m` | größter Abstand nach fehlgeschlagenen Runden |
<!-- config-reference:end -->

## 5. Geheimnisse

Katalog der Dateien im Volume `secrets` ([16-betrieb.md](../spec/16-betrieb.md) 5.3).
"Erzeugt" legt `secrets-init` an, "geliefert" legt der Owner in `infra/compose/secrets/` ab.
Die Spalte "Dienste" nennt die Unterverzeichnisse (`subpath`), in die `secrets-init` die Datei
kopiert.

| Datei | Herkunft | Dienste | Verwendung |
|---|---|---|---|
| `db_superuser_password` | erzeugt | postgres, migrate, backup | Superuser `postgres` |
| `db_owner_password` | erzeugt | migrate | Rolle `bothy_owner` |
| `db_app_password` | erzeugt | server, migrate | Rolle `bothy_app`, `database.password_ref` |
| `db_maint_password` | erzeugt | server, migrate | Rolle `bothy_maint` (Aufbewahrungsjob) |
| `db_monitor_password` | erzeugt | collector, migrate | Rolle `bothy_monitor`, Empfänger `postgresql` |
| `token_pepper` | erzeugt | server | `auth.token_pepper_ref` |
| `telemetry_pseudonym_key` | erzeugt, mindestens 32 Byte | server, collector | `telemetry.pseudonym_key_ref`, `hmac_key` der Allowlist |
| `audit_ip_kek` | erzeugt, 32 Byte | server | `privacy.audit_ip_kek_ref`: verschlüsselt die IP-Schlüssel in `audit_ip_keys` |
| `file_ticket_key` | erzeugt, 32 Byte | server | `file.ticket_key_ref`: HMAC der Abruf-Tickets ([12-dateien.md](../spec/12-dateien.md) 9.2) |
| `pgbackrest_cipher_pass` | erzeugt | backup | Verschlüsselung der pgBackRest-Repositorys |
| `pgbackrest_tls/` | erzeugt (eigene CA) | postgres, backup | gegenseitiges TLS zwischen Datenbank und Repository-Host |
| `restic_password` | erzeugt | backup | Verschlüsselung des restic-Repositorys |
| `s3_access_key`, `s3_secret_key` | erzeugt (Profil s3) oder geliefert (externer Dienst) | server, seaweedfs, backup | S3-Zugang, `file.s3.access_key_ref` und `file.s3.secret_key_ref` |
| `grafana_admin_password` | erzeugt | grafana | Admin-Konto im lokalen Grafana |
| `apns_key.p8` | geliefert | server | Push iOS und macOS, `push.apns.private_key_ref` |
| `fcm_service_account.json` | geliefert | server | Push Android |
| `oidc/<anbieter>_client_secret` | geliefert | server | OIDC (`identity_providers.config`) |
| `offsite/` | geliefert | backup | Zugang zum Off-site-Ziel (S3-Schlüssel oder SSH-Schlüssel) |
| `alertmanager_bothy_webhook_url` | geliefert | alertmanager | Eingangs-Webhook des Meldungskanals |
| `alertmanager_external_webhook_url` | geliefert | alertmanager | externer Bereitschaftskanal |
| `alertmanager_heartbeat_url` | geliefert | alertmanager | externer Herzschlag für den Watchdog |
| `newrelic_license_key` | geliefert | collector | Overlay `newrelic` |
| `grafana_otlp_authorization` | geliefert | collector | Overlay `grafana` (ganzer Kopfzeilenwert `Basic ...`) |
| `datadog_api_key` | geliefert | collector | Overlay `datadog` |
| `azure_client_secret` | geliefert | collector | Overlay `azure-monitor` (Service Principal) |
| `sentry_key` | geliefert | collector | Overlay `sentry` |

Nicht im Volume `secrets`: Die privaten Signierschlüssel (`<zweck>-<kid>.pem`, Zwecke
`access_token`, `invocation`, `webhook_out`, `audit_checkpoint`) erzeugt der Server selbst im
Volume `signing-keys` (`secrets.signing_key_dir`, ADR 0066); `server_signing_keys.private_key_ref`
zeigt dorthin. `bothy secrets export` sichert beide Orte.

Erzeuger späterer Etappen: `pgbackrest_tls/` (M7-01) und die erzeugte Form von `s3_access_key` und
`s3_secret_key` (Profil s3, M3). Bis dahin meldet `bothy secrets init` diese Einträge als `pending`;
`s3_*` gelten so lange als geliefert.

## 6. Instanz-Einstellungen

Gespeichert in `instance_settings` ([03-datenmodell.md](../spec/03-datenmodell.md) 4.1), geprüft
gegen das Schema des Schlüssels.

| Schlüssel | Typ | Standard | Bedeutung |
|---|---|---|---|
| `branding.product_name` | string | `Bothy` | angezeigter Produktname (ADR 0044) |
| `branding.accent_color` | Schlüssel der Kategorie-Farben | `spruce` | Akzentfarbe; nur Schlüssel aus der geprüften Palette (ADR 0044), kein freier Farbwert |
| `branding.logo_file_id` | uuid | leer | Logo als Datei |
| `locale.default` | BCP 47 | `de` | Standardsprache neuer Menschen |

Die Liste der Schlüssel führt [01-produkt.md](../spec/01-produkt.md), Abschnitt 11; persönliche
Einstellungen (`user_settings`, z. B. `notify.*`) stehen in [13-push-benachrichtigungen.md](../spec/13-push-benachrichtigungen.md),
Abschnitt 3.3.

## 7. Feature-Flags

Gespeichert in `feature_flags`; Regeln in [16-betrieb.md](../spec/16-betrieb.md) 6.2.

| Schlüssel | Art | Standard | Bedeutung | Quelle |
|---|---|---|---|---|
| `dm.enabled` | option | aus | Direktnachrichten zwischen Menschen | [05-aktionen.md](../spec/05-aktionen.md) `dm.create` |
| `voice.enabled` | option | aus | Transkription von Sprachnachrichten | [05-aktionen.md](../spec/05-aktionen.md) `file.transcribe` |
| `tokens.personal_enabled` | option | aus | persönliche API-Tokens | [05-aktionen.md](../spec/05-aktionen.md) `token.create` |

## 8. OpenTelemetry-Variablen

Der Server und `bothy-backupd` lesen die Standard-Variablen des OpenTelemetry-SDK unverändert.
Compose setzt diese:

| Variable | Wert in Compose | Bedeutung |
|---|---|---|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://otel-collector:4317` | Collector |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc` | Protokoll |
| `OTEL_METRIC_EXPORT_INTERVAL` | aus `BOTHY_OTEL_METRIC_EXPORT_INTERVAL_MS` | Export-Intervall der Metriken in ms |
| `OTEL_RESOURCE_ATTRIBUTES` | `deployment.environment.name=<Umgebung>` | Resource-Attribute |

Warteschlangen und Batch-Größen stellen die Standard-Variablen `OTEL_BSP_*` (Spans) und
`OTEL_BLRP_*` (Logs) ein. `service.name`, `service.namespace` und `bothy.server.role` setzt der
Server selbst; ein Wert in `OTEL_RESOURCE_ATTRIBUTES` überschreibt sie nicht.

## 9. Compose-Variablen

Datei `infra/compose/.env` nach Vorlage [`.env.example`](../../infra/compose/.env.example).
Diese Variablen sind keine Server-Konfiguration; Compose setzt daraus Werte der Dienste.

| Variable | Vorlage | Bedeutung |
|---|---|---|
| `COMPOSE_PROFILES` | `direct,scan` | Profile: genau eines von direct, tailscale; dazu scan, s3, observability |
| `BOTHY_REGISTRY` | `ghcr.io/OWNER` | Registry-Pfad der Bothy-Images in Kleinbuchstaben |
| `BOTHY_VERSION` | `0.0.0-placeholder` | Server-Version (SemVer ohne v); nie latest |
| `BOTHY_ENVIRONMENT` | `prod` | setzt `instance.environment` und `deployment.environment.name` |
| `BOTHY_PUBLIC_URL` | `https://chat.example.com` | setzt `server.public_url` |
| `BOTHY_HTTP_PUBLISH_PORT` | `8090` | Host-Port auf 127.0.0.1 für die API (Profil direct) |
| `BOTHY_TRUSTED_PROXIES` | `127.0.0.1/32,::1/128` | setzt `server.trusted_proxies` im Profil direct; dort die Gateway-Adresse des Compose-Netzes eintragen ([install-compose.md](install-compose.md) 5.1) |
| `BOTHY_OTLP_PUBLISH_PORT` | `4318` | Host-Port auf 127.0.0.1 für OTLP/HTTP von Adaptern auf demselben Host |
| `BOTHY_PROMETHEUS_PUBLISH_PORT` | `9090` | Host-Port auf 127.0.0.1 für Prometheus |
| `BOTHY_ALERTMANAGER_PUBLISH_PORT` | `9093` | Host-Port auf 127.0.0.1 für Alertmanager |
| `BOTHY_GRAFANA_PUBLISH_PORT` | `3300` | Host-Port auf 127.0.0.1 für Grafana |
| `TS_HOSTNAME` | `bothy` | Gerätename im Tailnet; erscheint im öffentlichen Zertifikats-Log, darum neutral wählen |
| `TS_AUTHKEY` | leer | Einmal-Schlüssel für die erste Tailscale-Anmeldung; danach leeren |
| `BOTHY_TAILSCALE_VERSION` | `v1.102.5` | Image-Version des Tailscale-Sidecars |
| `BOTHY_STORAGE_DRIVER` | `fs` | setzt `file.storage.driver`: fs oder s3 (Kapitel 12) |
| `BOTHY_S3_ENDPOINT` | `http://seaweedfs:8333` | setzt `file.s3.endpoint` bei Treiber s3 |
| `BOTHY_S3_BUCKET` | `bothy-files` | setzt `file.s3.bucket` bei Treiber s3 |
| `BOTHY_CLAMD_ADDRESS` | `tcp://clamav:3310` | setzt `file.scan.clamd_address` (Kapitel 12) |
| `BOTHY_PG_SHM_SIZE` | `256m` | Shared Memory des Datenbank-Containers |
| `BOTHY_PG_ARCHIVE_TIMEOUT` | `60s` | setzt `archive_timeout` in PostgreSQL; gleich `backup.pg.archive_timeout` |
| `BOTHY_BACKUP_OFFSITE_TARGET` | leer | setzt `backup.offsite.target` |
| `BOTHY_OTEL_EXPORTER` | `none` | erstes Collector-Overlay |
| `BOTHY_OTEL_EXPORTER_2` | `none` | zweites Collector-Overlay |
| `BOTHY_OTEL_METRIC_EXPORT_INTERVAL_MS` | `15000` | setzt `OTEL_METRIC_EXPORT_INTERVAL` der Server-Prozesse |
| `BOTHY_TRACE_SAMPLE_PERCENT` | `10` | Tail-Sampling: Anteil der übrigen Traces in Prozent |
| `BOTHY_TRACE_SLOW_THRESHOLD_MS` | `2000` | Tail-Sampling: ab dieser Dauer wird ein Trace immer behalten |
| `BOTHY_OTEL_MEMORY_LIMIT_PERCENT` | `80` | memory_limiter des Collectors |
| `NEW_RELIC_OTLP_ENDPOINT` | `https://otlp.eu01.nr-data.net` | Overlay newrelic: Endpunkt (EU oder US) |
| `GRAFANA_OTLP_ENDPOINT` | `https://otlp-gateway-REGION.grafana.net/otlp` | Overlay grafana: OTLP-Gateway |
| `GRAFANA_INSTANCE_ID` | leer | Overlay grafana: Instanz-ID (nur Information; der Kopfzeilenwert liegt in secrets) |
| `DD_SITE` | `datadoghq.eu` | Overlay datadog: Datadog-Site |
| `AZURE_TENANT_ID` | leer | Overlay azure-monitor: Tenant |
| `AZURE_CLIENT_ID` | leer | Overlay azure-monitor: Client-ID des Service Principals |
| `AZURE_MONITOR_TRACES_ENDPOINT` | leer | Overlay azure-monitor: OTLP-Endpunkt Traces (DCE/DCR) |
| `AZURE_MONITOR_METRICS_ENDPOINT` | leer | Overlay azure-monitor: OTLP-Endpunkt Metriken |
| `AZURE_MONITOR_LOGS_ENDPOINT` | leer | Overlay azure-monitor: OTLP-Endpunkt Logs |
| `SENTRY_OTLP_TRACES_ENDPOINT` | leer | Overlay sentry: OTLP-Endpunkt Traces |
| `SENTRY_OTLP_LOGS_ENDPOINT` | leer | Overlay sentry: OTLP-Endpunkt Logs |
| `BOTHY_PROMETHEUS_RETENTION` | `30d` | Aufbewahrung in Prometheus |
| `BOTHY_LOKI_RETENTION` | `14d` | Aufbewahrung in Loki |
| `BOTHY_TEMPO_RETENTION` | `168h` | Aufbewahrung in Tempo |
| `BOTHY_OTELCOL_VERSION` | `0.162.0` | Image-Version des Collectors (contrib) |
| `BOTHY_CLAMAV_VERSION` | `1.5.4` | Image-Version von ClamAV (clamav-debian, auch arm64) |
| `BOTHY_SEAWEEDFS_VERSION` | `4.48` | Image-Version von SeaweedFS |
| `BOTHY_PROMETHEUS_VERSION` | `v3.15.0` | Image-Version von Prometheus |
| `BOTHY_ALERTMANAGER_VERSION` | `v0.34.1` | Image-Version von Alertmanager |
| `BOTHY_LOKI_VERSION` | `3.7.8` | Image-Version von Loki |
| `BOTHY_TEMPO_VERSION` | `3.1.0` | Image-Version von Tempo |
| `BOTHY_GRAFANA_VERSION` | `13.2.3` | Image-Version von Grafana |
| `BOTHY_CLAMAV_MEMORY_LIMIT` | `4g` | Speichergrenze des ClamAV-Containers |
| `BOTHY_LOG_MAX_SIZE` | `20m` | Größe einer Container-Logdatei bis zur Rotation |
| `BOTHY_LOG_MAX_FILE` | `5` | behaltene Container-Logdateien je Dienst |

## 10. Pflege

1. Ein neuer Wert entsteht in der Tabelle "Konfigurationswerte dieses Kapitels" seines Kapitels;
   `make config-reference` übernimmt ihn in Abschnitt 4. Ein Name steht in genau einem Kapitel.
2. Ein Schlüssel wird nie umgedeutet. Entfernen ist eine Hauptversion des Servers
   ([18-release.md](../spec/18-release.md) 3); vorher steht er eine Minor-Version lang als
   veraltet in den Release Notes und erzeugt beim Start eine Warnung.
3. Prosa nennt nur den Namen, nie die Zahl.
