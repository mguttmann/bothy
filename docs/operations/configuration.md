# Konfigurationsreferenz

Status: Entwurf M0. Sammlung aller benannten Konfigurationswerte mit Umgebungsname, Typ,
Standard und Bedeutung. Regeln: [16-betrieb.md](../spec/16-betrieb.md) 5 und ADR 0011.

Maßgeblich für einen Standardwert ist bis M1 die Tabelle "Konfigurationswerte dieses Kapitels"
im genannten Kapitel. Diese Datei ist ihre Sammlung. Ab M1 entsteht diese Datei aus dem
Konfigurationsschema des Servers, und ein Check vergleicht das Schema mit den Kapiteltabellen
(Bau-Ticket "Konfigurationsschema und Referenz-Generator"). Bis dahin gilt: Wer einen Wert in
einem Kapitel ändert, ändert ihn hier in derselben Änderung.

## 1. Quellen

| Quelle | Inhalt | Ändern | Wirkung |
|---|---|---|---|
| Prozess-Konfiguration | alle Schlüssel aus Abschnitt 4 | Umgebung (`BOTHY_*`) oder YAML-Datei (`BOTHY_CONFIG_FILE`) | beim Start; Geheimnisse nach `secrets.cache_ttl` |
| Instanz-Einstellungen | Abschnitt 6, Tabelle `instance_settings` | Admin-Konsole, `instance.settings.update`, `instance.branding.update` (Owner, Step-up, Audit) | sofort, Ereignis `instance.updated` |
| Feature-Flags | Abschnitt 7, Tabelle `feature_flags` | Admin-Konsole, `feature_flag.update` (Owner, Step-up, Audit) | sofort, Ereignis `instance.updated` |
| OpenTelemetry | Abschnitt 8, Variablen `OTEL_*` | Umgebung | beim Start |
| Compose | Abschnitt 9, Datei `infra/compose/.env` | Datei | bei `docker compose up` |

Vorrang der Prozess-Konfiguration: Umgebung vor Datei vor Standard. Ein Schlüssel steht in genau
einer Quelle.

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

Schlüssel enthalten nie zwei Unterstriche hintereinander; darum ist die Namensregel umkehrbar.

## 3. Prüfung

1. Unbekannte `BOTHY_*`-Variablen und unbekannte Schlüssel in der Datei brechen den Start ab.
2. Typ- und Bereichsfehler brechen den Start ab; die Meldung nennt Schlüssel und Quelle, nie den
   Wert eines Geheimnisses.
3. `bothy config check` prüft ohne Start. `bothy config print --redacted` zeigt die wirksamen
   Werte und ihre Quelle, Geheimnisse nur als Verweis.

## 4. Prozess-Konfiguration

### Betrieb (16)

Quelle der Standardwerte: [16-betrieb.md](../spec/16-betrieb.md).

| Schlüssel | Umgebung | Typ | Standard | Bedeutung |
|---|---|---|---|---|
| `instance.environment` | `BOTHY_INSTANCE__ENVIRONMENT` | enum: dev, staging, prod | `prod` | `dev`, `staging` oder `prod` (6.1) |
| `server.role` | `BOTHY_SERVER__ROLE` | enum: api, worker, all | `all` | `api`, `worker` oder `all` |
| `server.listen_addr` | `BOTHY_SERVER__LISTEN_ADDR` | listen address | `127.0.0.1:8080` | öffentlicher Listener (REST, Gateway) |
| `server.internal_listen_addr` | `BOTHY_SERVER__INTERNAL_LISTEN_ADDR` | listen address | `127.0.0.1:8081` | Health, Readiness, Eingangs-Webhooks aus dem Stack |
| `server.public_url` | `BOTHY_SERVER__PUBLIC_URL` | url | keiner, Pflicht | Basis-URL, unter der Apps den Server erreichen |
| `server.trusted_proxies` | `BOTHY_SERVER__TRUSTED_PROXIES` | list(cidr) | leer | CIDR-Liste, deren `X-Forwarded-For` gilt |
| `server.shutdown_grace` | `BOTHY_SERVER__SHUTDOWN_GRACE` | duration | `30s` | Zeit zum Leeren beim Beenden; Verbindungen schließen mit `4013` |
| `server.maintenance_mode` | `BOTHY_SERVER__MAINTENANCE_MODE` | enum: off, read_only | `off` | `off` oder `read_only` (Umzug, Upgrade) |
| `secrets.dir` | `BOTHY_SECRETS__DIR` | path | `/run/bothy/secrets` | Basis der Standardverweise `file:` |
| `secrets.cache_ttl` | `BOTHY_SECRETS__CACHE_TTL` | duration | `5m` | so lange gilt ein gelesenes Geheimnis |
| `database.host` | `BOTHY_DATABASE__HOST` | host | `localhost` | Host der Datenbank |
| `database.port` | `BOTHY_DATABASE__PORT` | int | `5432` | Port der Datenbank im Netz des Servers |
| `database.name` | `BOTHY_DATABASE__NAME` | string | `bothy` | Datenbankname |
| `database.user` | `BOTHY_DATABASE__USER` | string | `bothy_app` | Rolle des Servers |
| `database.password_ref` | `BOTHY_DATABASE__PASSWORD_REF` | secret_ref | `file:<secrets.dir>/db_app_password` | Verweis auf das Passwort |
| `database.sslmode` | `BOTHY_DATABASE__SSLMODE` | enum: disable, require, verify-ca, verify-full | `verify-full` | TLS zur Datenbank; im internen Compose-Netz `disable` |
| `database.max_conns` | `BOTHY_DATABASE__MAX_CONNS` | int | `20` | Größe des Verbindungspools je Prozess (Startwert bis zum Lasttest) |
| `database.statement_timeout` | `BOTHY_DATABASE__STATEMENT_TIMEOUT` | duration | `30s` | Abbruch einzelner Abfragen |
| `migrate.require_recent_backup` | `BOTHY_MIGRATE__REQUIRE_RECENT_BACKUP` | bool | `true` | `migrate` verlangt eine frische Sicherung |
| `migrate.recent_backup_max_age` | `BOTHY_MIGRATE__RECENT_BACKUP_MAX_AGE` | duration | `24h` | so alt darf diese Sicherung sein |
| `migrate.lock_timeout` | `BOTHY_MIGRATE__LOCK_TIMEOUT` | duration | `5s` | `lock_timeout` je Migrationsschritt |
| `backup.pg.full_schedule` | `BOTHY_BACKUP__PG__FULL_SCHEDULE` | cron | `0 2 * * 0` | Plan der Vollsicherung (cron) |
| `backup.pg.diff_schedule` | `BOTHY_BACKUP__PG__DIFF_SCHEDULE` | cron | `0 2 * * 1-6` | Plan der Differenzsicherung (cron) |
| `backup.pg.archive_timeout` | `BOTHY_BACKUP__PG__ARCHIVE_TIMEOUT` | duration | `60s` | spätestens dann wird ein WAL-Segment archiviert |
| `backup.pg.retention_full` | `BOTHY_BACKUP__PG__RETENTION_FULL` | int | `4` | behaltene Vollsicherungen in repo2 |
| `backup.pg.local_retention_full` | `BOTHY_BACKUP__PG__LOCAL_RETENTION_FULL` | int | `1` | behaltene Vollsicherungen in repo1 |
| `backup.logical.schedule` | `BOTHY_BACKUP__LOGICAL__SCHEDULE` | cron | `0 3 * * *` | Plan von `pg_dump` (cron) |
| `backup.logical.retention` | `BOTHY_BACKUP__LOGICAL__RETENTION` | duration | `30d` | Aufbewahrung der logischen Sicherung |
| `backup.objects.schedule` | `BOTHY_BACKUP__OBJECTS__SCHEDULE` | cron | `30 3 * * *` | Plan der Objektsicherung (cron) |
| `backup.objects.retention` | `BOTHY_BACKUP__OBJECTS__RETENTION` | duration | `90d` | Aufbewahrung der Objekt-Snapshots |
| `backup.offsite.target` | `BOTHY_BACKUP__OFFSITE__TARGET` | string | leer | Off-site-Ziel; leer heißt nur lokal (Warnung in Konsole und Status) |
| `backup.max_age` | `BOTHY_BACKUP__MAX_AGE` | duration | `26h` | ohne erfolgreiche Sicherung länger als das: Alarm |
| `backup.restore_test.schedule` | `BOTHY_BACKUP__RESTORE_TEST__SCHEDULE` | cron | `0 5 * * 6` | Plan des Wiederherstellungstests (cron) |
| `backup.restore_test.sample_files` | `BOTHY_BACKUP__RESTORE_TEST__SAMPLE_FILES` | int | `200` | geprüfte Dateien je Test |
| `backup.restore_test.sample_threads` | `BOTHY_BACKUP__RESTORE_TEST__SAMPLE_THREADS` | int | `200` | geprüfte Threads je Test |
| `backup.restore_test.max_age` | `BOTHY_BACKUP__RESTORE_TEST__MAX_AGE` | duration | `8d` | ohne erfolgreichen Test länger als das: Alarm |
| `dr.rpo_target` | `BOTHY_DR__RPO_TARGET` | duration | `5m` | Ziel für den größten Datenverlust |
| `dr.rto_target` | `BOTHY_DR__RTO_TARGET` | duration | `4h` | Ziel für die Dauer der Wiederherstellung |
| `status.check_interval` | `BOTHY_STATUS__CHECK_INTERVAL` | duration | `30s` | Abstand der automatischen Prüfungen |
| `status.degrade_after` | `BOTHY_STATUS__DEGRADE_AFTER` | duration | `2m` | so lange Fehler bis `degraded` |
| `status.incident_after` | `BOTHY_STATUS__INCIDENT_AFTER` | duration | `5m` | so lange `major_outage` bis zur automatischen Störung |
| `status.resolve_after` | `BOTHY_STATUS__RESOLVE_AFTER` | duration | `10m` | so lange gesund bis zur automatischen Auflösung |
| `status.error_ratio_degraded` | `BOTHY_STATUS__ERROR_RATIO_DEGRADED` | float | `0.05` | Fehleranteil, ab dem eine Prüfung scheitert |
| `status.error_ratio_outage` | `BOTHY_STATUS__ERROR_RATIO_OUTAGE` | float | `0.5` | Fehleranteil, ab dem `major_outage` gilt |
| `status.fanout_lag_degraded` | `BOTHY_STATUS__FANOUT_LAG_DEGRADED` | duration | `5s` | Fan-out-Verzug, ab dem `sync.fanout` scheitert |

### Observability (17)

Quelle der Standardwerte: [17-observability.md](../spec/17-observability.md).

| Schlüssel | Umgebung | Typ | Standard | Bedeutung |
|---|---|---|---|---|
| `telemetry.enabled` | `BOTHY_TELEMETRY__ENABLED` | bool | `true` | Telemetrie an oder aus |
| `telemetry.traces.sampler_ratio` | `BOTHY_TELEMETRY__TRACES__SAMPLER_RATIO` | float | `1.0` | Anteil der Traces, die das SDK aufzeichnet; der Collector sampelt danach nach |
| `telemetry.traces.trust_remote_sampling` | `BOTHY_TELEMETRY__TRACES__TRUST_REMOTE_SAMPLING` | bool | `false` | `true`: das Sampling-Flag eines eingehenden `traceparent` gilt |
| `telemetry.logs.exporter` | `BOTHY_TELEMETRY__LOGS__EXPORTER` | enum: otlp, stdout, both | `both` | `otlp`, `stdout` oder `both` |
| `telemetry.logs.level` | `BOTHY_TELEMETRY__LOGS__LEVEL` | enum: debug, info, warn, error | `info` | kleinste Stufe, die geschrieben wird |
| `telemetry.pseudonym_key_ref` | `BOTHY_TELEMETRY__PSEUDONYM_KEY_REF` | secret_ref | `file:<secrets.dir>/telemetry_pseudonym_key` | Schlüssel für HMAC-Pseudonyme; mindestens 32 Byte |
| `telemetry.dashboard_url` | `BOTHY_TELEMETRY__DASHBOARD_URL` | url | leer | Link aus der Admin-Konsole zum Telemetrie-Backend |
| `stats.cache_ttl` | `BOTHY_STATS__CACHE_TTL` | duration | `5m` | so lange gilt ein berechnetes Statistik-Ergebnis |
| `stats.min_group_size` | `BOTHY_STATS__MIN_GROUP_SIZE` | int | `5` | kleinere Zähler zu Menschen werden als "weniger als" angezeigt |
| `stats.max_range` | `BOTHY_STATS__MAX_RANGE` | duration | `400d` | größter abfragbarer Zeitraum |

### Release und Clients (18)

Quelle der Standardwerte: [18-release.md](../spec/18-release.md).

| Schlüssel | Umgebung | Typ | Standard | Bedeutung |
|---|---|---|---|---|
| `client.min_version.ios` | `BOTHY_CLIENT__MIN_VERSION__IOS` | semver | leer | kleinste zugelassene App-Version auf iOS; leer heißt keine Grenze |
| `client.min_version.android` | `BOTHY_CLIENT__MIN_VERSION__ANDROID` | semver | leer | dasselbe für Android |
| `client.min_version.macos` | `BOTHY_CLIENT__MIN_VERSION__MACOS` | semver | leer | dasselbe für macOS |
| `client.min_version.windows` | `BOTHY_CLIENT__MIN_VERSION__WINDOWS` | semver | leer | dasselbe für Windows |
| `client.recommended_version.ios` | `BOTHY_CLIENT__RECOMMENDED_VERSION__IOS` | semver | leer | darunter zeigt die App einen Update-Hinweis |
| `client.recommended_version.android` | `BOTHY_CLIENT__RECOMMENDED_VERSION__ANDROID` | semver | leer | dasselbe für Android |
| `client.recommended_version.macos` | `BOTHY_CLIENT__RECOMMENDED_VERSION__MACOS` | semver | leer | dasselbe für macOS |
| `client.recommended_version.windows` | `BOTHY_CLIENT__RECOMMENDED_VERSION__WINDOWS` | semver | leer | dasselbe für Windows |

### Datenmodell (03)

Quelle der Standardwerte: [03-datenmodell.md](../spec/03-datenmodell.md).

| Schlüssel | Umgebung | Typ | Standard | Bedeutung |
|---|---|---|---|---|
| `principal.handle_max_chars` | `BOTHY_PRINCIPAL__HANDLE_MAX_CHARS` | int | `32` | Länge eines Handles |
| `identity.last_active_granularity` | `BOTHY_IDENTITY__LAST_ACTIVE_GRANULARITY` | duration | `5m` | höchstens so oft wird `last_active_at` bzw. `last_used_at` geschrieben |
| `auth.session_max_age` | `BOTHY_AUTH__SESSION_MAX_AGE` | duration | `30d` | absolutes Ende einer Anmeldesitzung |
| `auth.refresh_token_ttl` | `BOTHY_AUTH__REFRESH_TOKEN_TTL` | duration | `14d` | Lebensdauer eines Refresh-Tokens |
| `auth.step_up_max_age` | `BOTHY_AUTH__STEP_UP_MAX_AGE` | duration | `5m` | so lange gilt eine Passkey-Bestätigung als frisch |
| `agent.heartbeat_persist_interval` | `BOTHY_AGENT__HEARTBEAT_PERSIST_INTERVAL` | duration | `60s` | höchstens so oft werden Heartbeat-Felder gespeichert |
| `channel.history_backfill_limit_default` | `BOTHY_CHANNEL__HISTORY_BACKFILL_LIMIT_DEFAULT` | int | `20` | Standard für `history_backfill_limit` |
| `channel.history_backfill_limit_max` | `BOTHY_CHANNEL__HISTORY_BACKFILL_LIMIT_MAX` | int | `200` | Obergrenze für `history_backfill_limit` |
| `channel.prompt_max_bytes` | `BOTHY_CHANNEL__PROMPT_MAX_BYTES` | int (Byte) | `32768` | Größe eines Kanal-Prompts |
| `thread.title_max_chars` | `BOTHY_THREAD__TITLE_MAX_CHARS` | int | `200` | Länge eines Thread-Titels |
| `message.max_content_bytes` | `BOTHY_MESSAGE__MAX_CONTENT_BYTES` | int (Byte) | `262144` | Größe einer Nachricht (lange Antworten ohne Teilen) |
| `message.max_attachments` | `BOTHY_MESSAGE__MAX_ATTACHMENTS` | int | `10` | Anhänge je Nachricht |
| `file.max_upload_bytes` | `BOTHY_FILE__MAX_UPLOAD_BYTES` | int (Byte) | `104857600` | Größe einer Datei |
| `run.excerpt_max_chars` | `BOTHY_RUN__EXCERPT_MAX_CHARS` | int | `500` | Länge von `last_text_excerpt` |
| `run.step_detail_max_bytes` | `BOTHY_RUN__STEP_DETAIL_MAX_BYTES` | int (Byte) | `16384` | Größe von `run_steps.detail` |
| `run.max_steps_persisted` | `BOTHY_RUN__MAX_STEPS_PERSISTED` | int | `2000` | gespeicherte Schritte je Lauf |
| `run.stall_after` | `BOTHY_RUN__STALL_AFTER` | duration | `10m` | ohne Fortschritt: Stillstandshinweis |
| `component.default_ttl` | `BOTHY_COMPONENT__DEFAULT_TTL` | duration | `24h` | Ablauf einer Komponente ohne eigene Angabe |
| `push.max_consecutive_failures` | `BOTHY_PUSH__MAX_CONSECUTIVE_FAILURES` | int | `5` | danach wird eine Push-Registrierung deaktiviert |
| `webhook.allowed_hosts` | `BOTHY_WEBHOOK__ALLOWED_HOSTS` | list(host) | leer | erlaubte Ziel-Hosts für ausgehende Webhooks |
| `webhook.max_consecutive_failures` | `BOTHY_WEBHOOK__MAX_CONSECUTIVE_FAILURES` | int | `20` | danach wird ein Abo deaktiviert |
| `webhook.retry_backoff_base` | `BOTHY_WEBHOOK__RETRY_BACKOFF_BASE` | duration | `10s` | Basis des Backoffs |
| `webhook.retry_backoff_cap` | `BOTHY_WEBHOOK__RETRY_BACKOFF_CAP` | duration | `1h` | größter Abstand |
| `sync.event_log_partition_size` | `BOTHY_SYNC__EVENT_LOG_PARTITION_SIZE` | int | `1000000` | Sequenznummern je Partition |

### Sync-Protokoll (04)

Quelle der Standardwerte: [04-sync-protokoll.md](../spec/04-sync-protokoll.md).

| Schlüssel | Umgebung | Typ | Standard | Bedeutung |
|---|---|---|---|---|
| `sync.event_retention` | `BOTHY_SYNC__EVENT_RETENTION` | duration | `30d` | so lange bleiben dauerhafte Ereignisse nachholbar |
| `sync.event_payload_max_bytes` | `BOTHY_SYNC__EVENT_PAYLOAD_MAX_BYTES` | int (Byte) | `65536` | größte Nutzlast eines Ereignisses |
| `sync.ws_max_frame_bytes` | `BOTHY_SYNC__WS_MAX_FRAME_BYTES` | int (Byte) | `1048576` | größter WebSocket-Rahmen |
| `sync.identify_timeout` | `BOTHY_SYNC__IDENTIFY_TIMEOUT` | duration | `10s` | Zeit zwischen `hello` und `identify` |
| `sync.heartbeat_interval` | `BOTHY_SYNC__HEARTBEAT_INTERVAL` | duration | `30s` | Abstand der Heartbeats (in `hello` gemeldet) |
| `sync.heartbeat_miss_limit` | `BOTHY_SYNC__HEARTBEAT_MISS_LIMIT` | int | `2` | fehlende Heartbeats bis zum Schließen |
| `sync.resume_window` | `BOTHY_SYNC__RESUME_WINDOW` | duration | `5m` | so lange ist `resume` nach Trennung möglich |
| `sync.reauth_lead_time` | `BOTHY_SYNC__REAUTH_LEAD_TIME` | duration | `60s` | Vorlauf von `reauth_required` vor Token-Ablauf |
| `sync.reconnect_backoff_base` | `BOTHY_SYNC__RECONNECT_BACKOFF_BASE` | duration | `1s` | Basis des Backoffs beim Wiederverbinden |
| `sync.reconnect_backoff_cap` | `BOTHY_SYNC__RECONNECT_BACKOFF_CAP` | duration | `60s` | größter Abstand beim Wiederverbinden |
| `sync.catchup_page_size` | `BOTHY_SYNC__CATCHUP_PAGE_SIZE` | int | `500` | Ereignisse je Nachhol-Seite |
| `sync.catchup_max_events` | `BOTHY_SYNC__CATCHUP_MAX_EVENTS` | int | `50000` | mehr Rückstand ergibt `resync_required` |
| `sync.cursor_advance_every` | `BOTHY_SYNC__CURSOR_ADVANCE_EVERY` | int | `1000` | nach so vielen übersprungenen Ereignissen ein `cursor`-Rahmen |
| `sync.outbound_queue_max` | `BOTHY_SYNC__OUTBOUND_QUEUE_MAX` | int | `10000` | Rahmen im Ausgang einer Verbindung bis `4014` |
| `sync.idempotency_key_ttl` | `BOTHY_SYNC__IDEMPOTENCY_KEY_TTL` | duration | `24h` | Aufbewahrung von `Idempotency-Key` |
| `sync.max_focused_threads` | `BOTHY_SYNC__MAX_FOCUSED_THREADS` | int | `20` | Threads im `focus` je Verbindung |
| `sync.run_patch_coalesce_interval` | `BOTHY_SYNC__RUN_PATCH_COALESCE_INTERVAL` | duration | `250ms` | Sammelfenster für `run.updated` |
| `sync.typing_min_interval` | `BOTHY_SYNC__TYPING_MIN_INTERVAL` | duration | `5s` | höchstens so oft `typing.start` |
| `sync.typing_ttl` | `BOTHY_SYNC__TYPING_TTL` | duration | `8s` | Tipp-Anzeige ohne Erneuerung |
| `client.cache_max_age` | `BOTHY_CLIENT__CACHE_MAX_AGE` | duration | `30d` | ungenutzte Cache-Einträge |
| `client.outbox_retry_base` | `BOTHY_CLIENT__OUTBOX_RETRY_BASE` | duration | `2s` | Basis des Backoffs im Ausgangsfach |
| `client.outbox_retry_cap` | `BOTHY_CLIENT__OUTBOX_RETRY_CAP` | duration | `5m` | größter Abstand im Ausgangsfach |
| `agent.invocation_token_ttl` | `BOTHY_AGENT__INVOCATION_TOKEN_TTL` | duration | `60s` | Gültigkeit einer Aufruf-Marke |
| `agent.ack_timeout` | `BOTHY_AGENT__ACK_TIMEOUT` | duration | `30s` | ohne `ack`: erneute Zustellung |
| `agent.ack_persist_interval` | `BOTHY_AGENT__ACK_PERSIST_INTERVAL` | duration | `1s` | spätestens so oft wird `acked_seq` gespeichert |
| `agent.max_inflight_events` | `BOTHY_AGENT__MAX_INFLIGHT_EVENTS` | int | `100` | unbestätigte Ereignisse je Verbraucher |
| `agent.run_update_min_interval` | `BOTHY_AGENT__RUN_UPDATE_MIN_INTERVAL` | duration | `500ms` | Adapter-Drossel für `run.update` |
| `agent.presence_timeout` | `BOTHY_AGENT__PRESENCE_TIMEOUT` | duration | `90s` | ohne Heartbeat gilt der Agent als `offline` |
| `agent.cancel_grace` | `BOTHY_AGENT__CANCEL_GRACE` | duration | `30s` | Zeit für den Adapter, einen Abbruch zu melden |
| `auth.dpop_nonce_ttl` | `BOTHY_AUTH__DPOP_NONCE_TTL` | duration | `5m` | Gültigkeit des DPoP-Nonce aus `hello` |

### Autorisierung und Rate-Limits (05)

Quelle der Standardwerte: [05-aktionen.md](../spec/05-aktionen.md).

| Schlüssel | Umgebung | Typ | Standard | Bedeutung |
|---|---|---|---|---|
| `audit.deny_aggregate_window` | `BOTHY_AUDIT__DENY_AGGREGATE_WINDOW` | duration | `60s` | Zusammenfassungsfenster für gleiche Ablehnungen |
| `status.public_enabled` | `BOTHY_STATUS__PUBLIC_ENABLED` | bool | `false` | Statusseite ohne Anmeldung |
| `ratelimit.rl.public_auth.per_minute` | `BOTHY_RATELIMIT__RL__PUBLIC_AUTH__PER_MINUTE` | int | `30` | je Client-IP: Token je Minute |
| `ratelimit.rl.public_auth.burst` | `BOTHY_RATELIMIT__RL__PUBLIC_AUTH__BURST` | int | `10` | je Client-IP: Bucket-Größe |
| `ratelimit.rl.gateway_connect.per_minute` | `BOTHY_RATELIMIT__RL__GATEWAY_CONNECT__PER_MINUTE` | int | `12` | je Gerät oder Token: Token je Minute |
| `ratelimit.rl.gateway_connect.burst` | `BOTHY_RATELIMIT__RL__GATEWAY_CONNECT__BURST` | int | `4` | je Gerät oder Token: Bucket-Größe |
| `ratelimit.rl.read.per_minute` | `BOTHY_RATELIMIT__RL__READ__PER_MINUTE` | int | `600` | je Prinzipal: Token je Minute |
| `ratelimit.rl.read.burst` | `BOTHY_RATELIMIT__RL__READ__BURST` | int | `100` | je Prinzipal: Bucket-Größe |
| `ratelimit.rl.write.per_minute` | `BOTHY_RATELIMIT__RL__WRITE__PER_MINUTE` | int | `120` | je Prinzipal: Token je Minute |
| `ratelimit.rl.write.burst` | `BOTHY_RATELIMIT__RL__WRITE__BURST` | int | `30` | je Prinzipal: Bucket-Größe |
| `ratelimit.rl.message.per_minute` | `BOTHY_RATELIMIT__RL__MESSAGE__PER_MINUTE` | int | `60` | je Prinzipal: Token je Minute |
| `ratelimit.rl.message.burst` | `BOTHY_RATELIMIT__RL__MESSAGE__BURST` | int | `20` | je Prinzipal: Bucket-Größe |
| `ratelimit.rl.trigger.per_minute` | `BOTHY_RATELIMIT__RL__TRIGGER__PER_MINUTE` | int | `30` | je Prinzipal: Token je Minute |
| `ratelimit.rl.trigger.burst` | `BOTHY_RATELIMIT__RL__TRIGGER__BURST` | int | `10` | je Prinzipal: Bucket-Größe |
| `ratelimit.rl.interaction.per_minute` | `BOTHY_RATELIMIT__RL__INTERACTION__PER_MINUTE` | int | `60` | je Prinzipal: Token je Minute |
| `ratelimit.rl.interaction.burst` | `BOTHY_RATELIMIT__RL__INTERACTION__BURST` | int | `20` | je Prinzipal: Bucket-Größe |
| `ratelimit.rl.upload.per_minute` | `BOTHY_RATELIMIT__RL__UPLOAD__PER_MINUTE` | int | `30` | je Prinzipal: Token je Minute |
| `ratelimit.rl.upload.burst` | `BOTHY_RATELIMIT__RL__UPLOAD__BURST` | int | `10` | je Prinzipal: Bucket-Größe |
| `ratelimit.rl.search.per_minute` | `BOTHY_RATELIMIT__RL__SEARCH__PER_MINUTE` | int | `60` | je Prinzipal: Token je Minute |
| `ratelimit.rl.search.burst` | `BOTHY_RATELIMIT__RL__SEARCH__BURST` | int | `20` | je Prinzipal: Bucket-Größe |
| `ratelimit.rl.typing.per_minute` | `BOTHY_RATELIMIT__RL__TYPING__PER_MINUTE` | int | `30` | je Verbindung: Token je Minute |
| `ratelimit.rl.typing.burst` | `BOTHY_RATELIMIT__RL__TYPING__BURST` | int | `5` | je Verbindung: Bucket-Größe |
| `ratelimit.rl.admin.per_minute` | `BOTHY_RATELIMIT__RL__ADMIN__PER_MINUTE` | int | `60` | je Prinzipal: Token je Minute |
| `ratelimit.rl.admin.burst` | `BOTHY_RATELIMIT__RL__ADMIN__BURST` | int | `20` | je Prinzipal: Bucket-Größe |
| `ratelimit.rl.export.per_minute` | `BOTHY_RATELIMIT__RL__EXPORT__PER_MINUTE` | int | `2` | je Prinzipal: Token je Minute |
| `ratelimit.rl.export.burst` | `BOTHY_RATELIMIT__RL__EXPORT__BURST` | int | `2` | je Prinzipal: Bucket-Größe |
| `ratelimit.rl.agent_runtime.per_minute` | `BOTHY_RATELIMIT__RL__AGENT_RUNTIME__PER_MINUTE` | int | `1200` | je Agent: Token je Minute |
| `ratelimit.rl.agent_runtime.burst` | `BOTHY_RATELIMIT__RL__AGENT_RUNTIME__BURST` | int | `200` | je Agent: Bucket-Größe |
| `ratelimit.rl.webhook_in.per_minute` | `BOTHY_RATELIMIT__RL__WEBHOOK_IN__PER_MINUTE` | int | `60` | je Eingangs-Webhook: Token je Minute |
| `ratelimit.rl.webhook_in.burst` | `BOTHY_RATELIMIT__RL__WEBHOOK_IN__BURST` | int | `20` | je Eingangs-Webhook: Bucket-Größe |

### Kapitel anderer Teams

Diese Kapitel ergänzen ihre Tabellen beim Zusammenführen: 06 API, 07 Rechte, 08 Identität,
09 Agenten, 10 Backend-Richtlinie, 11 Erweiterungen, 12 Dateien, 13 Push, 14 Suche,
15 Client-Architektur, 19 Sicherheit, 20 Datenschutz, 21 Dev-Isolation, 22 Import, 23 Qualität.

Die Compose-Datei nutzt schon diese Schlüssel. Sie sind Vorschläge von Team F; das genannte
Kapitel bestätigt oder benennt sie um, und Compose folgt.

| Schlüssel | Umgebung | Typ | Vorschlag | Kapitel | Bedeutung |
|---|---|---|---|---|---|
| `storage.driver` | `BOTHY_STORAGE__DRIVER` | enum: fs, s3 | `fs` | 12 | Treiber des Objektspeichers (B4) |
| `storage.fs.root` | `BOTHY_STORAGE__FS__ROOT` | path | `/var/lib/bothy/files` | 12 | Wurzel des Dateisystem-Treibers |
| `storage.s3.endpoint` | `BOTHY_STORAGE__S3__ENDPOINT` | url | leer | 12 | S3-Endpunkt |
| `storage.s3.bucket` | `BOTHY_STORAGE__S3__BUCKET` | string | `bothy-files` | 12 | Bucket |
| `storage.s3.access_key_ref` | `BOTHY_STORAGE__S3__ACCESS_KEY_REF` | secret_ref | `file:<secrets.dir>/s3_access_key` | 12 | Zugangsschlüssel |
| `storage.s3.secret_key_ref` | `BOTHY_STORAGE__S3__SECRET_KEY_REF` | secret_ref | `file:<secrets.dir>/s3_secret_key` | 12 | geheimer Schlüssel |
| `files.scan.clamd_address` | `BOTHY_FILES__SCAN__CLAMD_ADDRESS` | string | leer (kein Scan) | 12 | `tcp://host:port` oder `unix:/pfad` |
| `auth.token_pepper_ref` | `BOTHY_AUTH__TOKEN_PEPPER_REF` | secret_ref | `file:<secrets.dir>/token_pepper` | 08 | Pepper für Token-Hashes ([03-datenmodell.md](../spec/03-datenmodell.md) 1.9) |
| `push.apns.key_ref` | `BOTHY_PUSH__APNS__KEY_REF` | secret_ref | `file:<secrets.dir>/apns_key.p8` | 13 | APNs-Signierschlüssel |
| `push.fcm.service_account_ref` | `BOTHY_PUSH__FCM__SERVICE_ACCOUNT_REF` | secret_ref | `file:<secrets.dir>/fcm_service_account.json` | 13 | FCM-Dienstkonto |

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
| `signing/<zweck>-<kid>.pem` | erzeugt | server | `server_signing_keys.private_key_ref` (Zwecke `access_token`, `invocation`, `webhook_out`) |
| `pgbackrest_cipher_pass` | erzeugt | backup | Verschlüsselung der pgBackRest-Repositorys |
| `pgbackrest_tls/` | erzeugt (eigene CA) | postgres, backup | gegenseitiges TLS zwischen Datenbank und Repository-Host |
| `restic_password` | erzeugt | backup | Verschlüsselung des restic-Repositorys |
| `s3_access_key`, `s3_secret_key` | erzeugt (Profil s3) oder geliefert (externer Dienst) | server, seaweedfs, backup | S3-Zugang |
| `grafana_admin_password` | erzeugt | grafana | Admin-Konto im lokalen Grafana |
| `apns_key.p8` | geliefert | server | Push iOS und macOS |
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

## 6. Instanz-Einstellungen

Gespeichert in `instance_settings` ([03-datenmodell.md](../spec/03-datenmodell.md) 4.1), geprüft
gegen das Schema des Schlüssels.

| Schlüssel | Typ | Standard | Bedeutung |
|---|---|---|---|
| `branding.product_name` | string | `Bothy` | angezeigter Produktname (ADR 0044) |
| `branding.accent_color` | Farb-Token | Token des Design-Systems | Akzentfarbe |
| `branding.logo_file_id` | uuid | leer | Logo als Datei |
| `locale.default` | BCP 47 | `de` | Standardsprache neuer Menschen |

Weitere Einstellungen ergänzen Kapitel 01 und 15 (Team E).

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
| `BOTHY_OTLP_PUBLISH_PORT` | `4318` | Host-Port auf 127.0.0.1 für OTLP/HTTP von Adaptern auf demselben Host |
| `BOTHY_PROMETHEUS_PUBLISH_PORT` | `9090` | Host-Port auf 127.0.0.1 für Prometheus |
| `BOTHY_ALERTMANAGER_PUBLISH_PORT` | `9093` | Host-Port auf 127.0.0.1 für Alertmanager |
| `BOTHY_GRAFANA_PUBLISH_PORT` | `3300` | Host-Port auf 127.0.0.1 für Grafana |
| `TS_HOSTNAME` | `bothy` | Gerätename im Tailnet; erscheint im öffentlichen Zertifikats-Log, darum neutral wählen |
| `TS_AUTHKEY` | leer | Einmal-Schlüssel für die erste Tailscale-Anmeldung; danach leeren |
| `BOTHY_TAILSCALE_VERSION` | `v1.102.5` | Image-Version des Tailscale-Sidecars |
| `BOTHY_STORAGE_DRIVER` | `fs` | setzt den Dateitreiber: fs oder s3 (Kapitel 12) |
| `BOTHY_S3_ENDPOINT` | `http://seaweedfs:8333` | S3-Endpunkt bei Treiber s3 |
| `BOTHY_S3_BUCKET` | `bothy-files` | S3-Bucket bei Treiber s3 |
| `BOTHY_CLAMD_ADDRESS` | `tcp://clamav:3310` | Adresse von clamd (Kapitel 12) |
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

1. Ein neuer Wert entsteht in der Tabelle "Konfigurationswerte dieses Kapitels" seines Kapitels
   und in derselben Änderung hier.
2. Ein Schlüssel wird nie umgedeutet. Entfernen ist eine Hauptversion des Servers
   ([18-release.md](../spec/18-release.md) 3); vorher steht er eine Minor-Version lang als
   veraltet in den Release Notes und erzeugt beim Start eine Warnung.
3. Prosa nennt nur den Namen, nie die Zahl.
