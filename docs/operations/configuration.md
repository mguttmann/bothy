# Konfigurationsreferenz

Status: Entwurf M0. Sammlung aller benannten Konfigurationswerte mit Umgebungsname, Typ,
Standard und Bedeutung. Regeln: [16-betrieb.md](../spec/16-betrieb.md) 5 und ADR 0011.

Quelle ist das Schema im Code (Ruling L4): Jeder Schlüssel, den der Server liest, steht mit Art,
Standard, Grenzen und Kapitel in `services/server/internal/wire/configkeys_*.go`, exportiert nach
`services/server/architecture/config-schema.json`. Die Tabelle "Konfigurationswerte dieses
Kapitels" des genannten Kapitels nennt denselben Standard und die Bedeutung; Werte, die der Server
noch nicht liest, und Werte anderer Programme stehen nur dort. Abschnitt 4 entsteht aus Schema und
Kapiteltabellen (`tools/configdoc/configdoc.py`), und `make config-doc-check` hält beide gleich
(Abschnitt 10).

## 1. Quellen

| Quelle | Inhalt | Ändern | Wirkung |
|---|---|---|---|
| Prozess-Konfiguration | alle Schlüssel aus Abschnitt 4 mit Ort `Server` und Stand `aktiv` | Umgebung (`BOTHY_*`), YAML-Datei (`--config` oder `BOTHY_CONFIG_FILE`) oder bei `server.role` das Flag `--role` | beim Start; Geheimnisse nach `secrets.cache_ttl` |
| Instanz-Einstellungen | Abschnitt 6, Tabelle `instance_settings` | Admin-Konsole, `instance.settings.update`, `instance.branding.update` (Owner, Step-up, Audit) | sofort, Ereignis `instance.updated` |
| Feature-Flags | Abschnitt 7, Tabelle `feature_flags` | Admin-Konsole, `feature_flag.update` (Owner, Step-up, Audit) | sofort, Ereignis `instance.updated` |
| OpenTelemetry | Abschnitt 8, Variablen `OTEL_*` | Umgebung | beim Start |
| Compose | Abschnitt 9, Datei `infra/compose/.env` | Datei | bei `docker compose up` |

Vorrang der Prozess-Konfiguration: Kommandozeile (nur `--role`, `--config`) vor Umgebung vor Datei
vor Standard. Ein Schlüssel steht in genau einer Quelle. Abschnitt 4 führt auch Schlüssel, deren
Nutzer erst in einer späteren Etappe entsteht, mit dem Stand `geplant (<Etappe>)`: Diese
Server-Version kennt sie nicht und lehnt sie als unbekannt ab (Abschnitt 3, Punkt 1). Ins Schema
kommt ein Schlüssel mit dem Ticket, das seinen ersten Nutzer baut
([02-architektur.md](../spec/02-architektur.md) 13, Punkt 5).

## 2. Namensregel und Formate

Jede Art hat genau einen Parser; Standardwert, Umgebung, Datei und Flag gehen durch denselben.
Fehlermeldungen nennen nie den geprüften Text.

| Thema | Regel | Beispiel |
|---|---|---|
| Schlüssel | mindestens zwei Segmente, durch `.` getrennt, jedes Segment `[a-z][a-z0-9]*(_[a-z0-9]+)*`; die Endung `_ref` haben genau die Schlüssel der Art `secret_ref` | `sync.event_retention` |
| Umgebungsname | `BOTHY_` plus Schlüssel in Großbuchstaben, Punkt wird `__` | `sync.event_retention` -> `BOTHY_SYNC__EVENT_RETENTION` |
| Datei | YAML mit genau einem Dokument, Schlüssel verschachtelt nach Punkten. Skalare gelten als geschriebener Text und gehen an den Parser der Art, so ändert die Typisierung von YAML keinen Wert (`yes` bleibt `yes`, `1.10` bleibt `1.10`, `0x10` ist keine Zahl). Doppelte und gepunktete Schlüssel, Anker, Aliase, Merge-Schlüssel (`<<`) und ausdrückliche Tags (`!!str`) sind Fehler, ebenso eine Liste oder ein Mapping, wo ein Wert steht, und ein Skalar oder `null`, wo ein Abschnitt steht. Ein Schlüssel, dessen Text keine Schlüsselform hat, erscheint in der Meldung nur mit seiner Zeile, weil ein Tippfehler einen Wert zum Teil des Schlüssels machen kann (`{role:wert}` ohne Leerzeichen nach dem Doppelpunkt) | `sync: {event_retention: 30d}` |
| `string` | beliebiger Text, auch leer | `bothy` |
| `enum` | genau einer der Werte, die das Kapitel nennt; Groß- und Kleinschreibung zählt | `worker` |
| `bool` | `true` oder `false` | `true` |
| `int` | ganze Zahl im Zehnersystem im Bereich von int64, optional mit führendem `-`; kein `+`, kein Trennzeichen, kein Exponent, keine Hexadezimalzahl | `1048576` |
| `float` | endliche Dezimalzahl: `-` optional, Ziffern mit optionalem Nachkommateil oder nur Nachkommateil (`.5`), optional Exponent `e` oder `E` mit optionalem Vorzeichen; kein `+` vorn, kein `inf`, kein `NaN`, keine Hexadezimalzahl | `0.05` |
| Dauer (`duration`) | genau `(0\|[1-9][0-9]*)(ms\|s\|m\|h\|d)`: eine Zahl ohne Vorzeichen, Bruch oder führende Null und genau eine Einheit, `d` = 24 h. Keine Zusammensetzung wie `1h30m`; eine Dauer über der größten Dauer von Go (`time.Duration`, int64-Nanosekunden) ist ein Fehler, die größte ganze Zahl hängt darum von der Einheit ab (`106751d`, `2562047h`) | `250ms`, `30d` |
| Liste (`list(string)`) | Umgebung und Standardwert: ein JSON-Array von Strings, wenn der Text mit `[` beginnt, sonst durch Komma getrennt mit entferntem Leerraum um jedes Element; leerer Text ist die leere Liste; ein leeres Element ist ein Fehler. Datei: YAML-Liste von Skalaren oder ein Skalar in der Form der Umgebung | `a.example.com, b.example.com`, `["x,y"]` |
| Präfix-Liste (`list(cidr)`) | wie `list(string)`; jedes Element ist ein IPv4- oder IPv6-Präfix in CIDR-Form ohne gesetzte Host-Bits (`192.0.2.10/24` ist ein Fehler) | `BOTHY_SERVER__TRUSTED_PROXIES=192.0.2.10/32,198.51.100.0/24` |
| Map (`map(string)`) | Environment and default: a JSON object whose values are strings; keys are not empty and occur once; an array, a number, `null` or a nested object as a value is an error. File: a YAML mapping of plain scalars below the key, or a scalar in the environment form. The canonical text is the JSON object with sorted keys (M3-13, D-M3-13-4) | `BOTHY_SEARCH__LOCALE_MAP={"de":"german","en":"english"}` |
| `listen address` | `host:port`; Host leer (`:port` heißt alle Schnittstellen) oder IP-Literal, IPv6 in Klammern, kein DNS-Name; Port dezimal von 0 bis 65535 ohne führende Null | `127.0.0.1:8080`, `[::1]:8081`, `:8080` |
| `cron` | fünf durch Leerraum getrennte Felder in UTC: Minute 0 bis 59, Stunde 0 bis 23, Tag 1 bis 31, Monat 1 bis 12, Wochentag 0 bis 6 (0 = Sonntag). Ein Feld ist eine Komma-Liste aus `*`, `*/n`, `a`, `a-b` und `a-b/n` mit `a <= b` und Schritt `n` von 1 bis zum Höchstwert des Felds; Zahlen ohne führende Null, keine Namen, keine Kürzel wie `@daily` | `0 2 * * 0`, `*/15 0-6 * * 1-5` |
| `semver` | SemVer 2.0.0 ohne führendes `v`, mit optionaler Vorabversion und Build-Angabe | `1.4.0`, `1.4.0-rc.1` |
| `secret_ref` | `<schema>:<verweis>` ([16-betrieb.md](../spec/16-betrieb.md) 5.2): `file:` mit absolutem Pfad in sauberer Form (kein `.` oder `..`, kein doppelter oder abschließender `/`, kein NUL-Zeichen), `env:` mit einem Variablennamen aus `[A-Z_][A-Z0-9_]*`, der weder `BOTHY_CONFIG_FILE` ist noch die Form einer Schlüssel-Variable hat (`BOTHY_A__B`); üblich ist `BOTHY_SECRET_<NAME>`. `azkv` und `awssm` meldet diese Server-Version als nicht unterstützt; jedes andere Schema, ein fehlendes Schema und leerer Text sind Fehler. Ein Standardwert darf `<schlüssel>` eines String-Schlüssels enthalten; beim Laden steht dort dessen wirksamer Wert. In gesetzten Werten wird nur `<secrets.dir>` ersetzt | `file:/run/bothy/secrets/db_app_password`, `file:<secrets.dir>/db_app_password` |
| `url` | Schema `http` oder `https`, Host nicht leer, kein Benutzer-Teil, kein Fragment (`#`), Port, falls angegeben, dezimal von 1 bis 65535 ohne führende Null | `https://chat.example.com` |
| Egress-Ziel (`egress.allowed_hosts`) | Liste wie `list(string)`, jedes Element `host[:port][;allow_private]` (gelesen von `egress.ParseRule`); Host als DNS-Name aus Buchstaben, Ziffern, `-` und `_` (Großbuchstaben werden klein, kein Punkt am Ende, das letzte Label ist keine Zahl wie `1` oder `0x7f`) oder IP-Literal (IPv6 in Klammern, wenn ein Port folgt; keine Zone; eine IPv4-gemappte IPv6-Adresse gilt als IPv4-Adresse); kein Platzhalter, kein Schema, kein Pfad, kein Benutzer-Teil; Port 1 bis 65535 ohne führende Null, ohne Port gilt nur der Standard-Port des Schemas; einziger Schalter `allow_private`; zwei Elemente mit gleichem Host und Port sind ein Ladefehler. Beginnt der Wert in der Umgebung mit `[` (IPv6-Literal mit Port als erstes Element), muss er ein JSON-Array sein | `api.example.com:443`, `[2001:db8::10]:8443;allow_private`, `BOTHY_EGRESS__ALLOWED_HOSTS=["[2001:db8::10]:8443","api.example.com"]` |

Schlüssel enthalten nie zwei Unterstriche hintereinander; darum ist die Namensregel umkehrbar.

## 3. Prüfung

1. Unbekannte `BOTHY_*`-Variablen und unbekannte Schlüssel in der Datei brechen den Start ab.
   Ausgenommen sind `BOTHY_CONFIG_FILE` und Variablen, die ein gültiger Verweis `env:` im
   wirksamen Wert eines Schlüssels `*_ref` nennt ([16-betrieb.md](../spec/16-betrieb.md) 5.1).
   Variablen ohne Präfix `BOTHY_` bleiben unbeachtet. Steht eine Variable zweimal in der
   Umgebung des Prozesses, sieht der Server nur den ersten Eintrag (so übergibt die Go-Laufzeit die
   Umgebung); den zweiten prüft er nicht. `BOTHY_CONFIG_FILE` gilt nur, wenn `--config` fehlt;
   leer heißt keine Datei. `--config` mit leerem Pfad ist ein Aufruffehler.
2. Typ- und Bereichsfehler brechen den Start ab, auch für einen Wert, den eine höhere Quelle
   überschreibt. Ein Pflicht-Schlüssel braucht einen nicht leeren Wert aus Datei, Umgebung oder
   Flag. Danach laufen die Querprüfungen aus [02-architektur.md](../spec/02-architektur.md) 13,
   Punkt 4, sobald ihre Schlüssel fehlerfrei geladen sind; ein Schlüssel einer verworfenen Datei
   oder unter einem verworfenen Abschnitt der Datei ist es nicht.
3. Alle Probleme kommen zusammen, nach Schlüssel sortiert, je Zeile
   `config: <schlüssel>: <meldung> (source: <quelle>)`, bei unbekannten Namen
   `config: unknown variable BOTHY_X (source: environment)` bzw.
   `config: unknown key "a.b" (source: file <pfad>)`; ein Datei-Schlüssel ohne Schlüsselform
   steht nur mit seiner Zeile da (`config: unknown key at line 3 below "server" (source: file
   <pfad>)`). Keine Meldung nennt einen Wert (ein Geheimnis kann im falschen Schlüssel stehen).
   Pfade und Variablennamen mit ungültigem UTF-8, nicht druckbaren Zeichen, Leerraum am Rand oder
   einem Anführungszeichen am Anfang stehen in Anführungszeichen mit Go-Escapes, damit jedes
   Problem eine Zeile bleibt.
4. `bothy config check [--secrets] [--config DATEI] [--role ROLLE]` prüft ohne Start:
   `configuration ok` und Exitcode 0, sonst jedes Problem auf der Standardfehlerausgabe und
   Exitcode 1; ein Aufruffehler ergibt Exitcode 2. Je Schlüssel `*_ref`, dessen wirksamer Verweis
   `env:` nutzt, schreibt es mit und ohne `--secrets` die Warnung
   `warning: <schlüssel> uses an env: reference; prefer file: (16-betrieb.md 5.2)` auf die
   Standardfehlerausgabe; der Exitcode bleibt davon unberührt. Mit `--secrets` löst es
   zusätzlich jeden Wert eines Schlüssels `*_ref` über den Secret-Provider auf und schreibt je
   Schlüssel `secret <schlüssel>: ok` oder `secret <schlüssel>: <fehlercode>` (`invalid_ref`,
   `unsupported_scheme`, `not_found`, `insecure_file`, `read_failed`), nie einen Wert; die
   Fehlermeldung mit Verweis und Regel steht auf der Standardfehlerausgabe. `configuration ok`
   folgt nur, wenn jeder Verweis aufgelöst ist.
5. `bothy config print [--redacted] [--config DATEI] [--role ROLLE]` zeigt in der ersten Zeile
   `# configuration file: <pfad>` oder `# configuration file: none`, dann je Schlüssel nach Namen
   sortiert `<schlüssel> = <wert>  # <quelle>` mit der Quelle `default`, `file <pfad>`,
   `env BOTHY_X` oder `flag --role`. Geheimnisse erscheinen nur als Verweis; aufgelöst wird
   nichts. Die Ausgabe ist immer redigiert, `--redacted` sagt das nur aus, `--redacted=false` ist
   ein Aufruffehler. Werte stehen in kanonischer Form, die sich wieder einlesen lässt: Dauer in
   der größten ganzen Einheit (`120s` als `2m`), Listen durch `, ` getrennt oder als JSON-Array,
   wenn ein Element ein Komma oder Leerraum am Rand hat oder das erste mit `[` beginnt,
   IP-Adressen in Normalform. Text mit ungültigem UTF-8, nicht druckbaren Zeichen, Leerraum am
   Rand oder einem Anführungszeichen am Anfang steht in Anführungszeichen mit Go-Escapes.

## 4. Benannte Werte aller Kapitel

Erzeugt von `tools/configdoc/configdoc.py` aus dem Schema des Servers und den Tabellen
"Konfigurationswerte dieses Kapitels" (Ruling L4); `make config-reference` schreibt den Block neu,
`make config-doc-check` prüft ihn (Abschnitt 10). Nicht von Hand ändern. Die Spalten:

- Umgebung: Variable nach der Namensregel aus Abschnitt 2, nur bei Ort `Server`.
- Ort: `Server` ist die Prozess-Konfiguration aus Abschnitt 1. Die anderen Orte `App`, `Adapter`,
  `Hermes-Adapter`, `Runner`, `Host-Dienst`, `SDK`, `Abgleich-App` und Werkzeug `bothy import`
  sind Werte dieser Programme; `Lasttest` sind Schwellen von `make perf`, keine
  Laufzeit-Konfiguration ([02-architektur.md](../spec/02-architektur.md) 22). Ein anderer Ort in
  einer Kapiteltabelle ist ein Befund (Abschnitt 10).
- Typ: die Art aus dem Schema mit der Grammatik aus Abschnitt 2, bei `enum` mit den erlaubten
  Werten; leer, solange der Schlüssel nicht im Schema steht.
- Standard und Bedeutung: aus der Kapiteltabelle; bei Stand `aktiv` ist der Standard der des
  Schemas.
- Stand: `aktiv` heißt, diese Server-Version liest den Schlüssel. `geplant (<Etappe>)` heißt, ein
  Ticket dieser Etappe ([PLAN](../PLAN.md)) baut seinen Nutzer; bis dahin lehnt diese
  Server-Version den Schlüssel als unbekannt ab (Abschnitt 3, Punkt 1). Die Etappe ist eine
  Auskunft des Plans, kein Vertrag. Leer bei Werten anderer Programme.

<!-- config-reference:start -->
### 01 Produkt

Quelle: [01-produkt.md](../spec/01-produkt.md), Abschnitt 14.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `ui.message_group_window` |  | App |  | `5m` | Nachrichten desselben Autors innerhalb dieser Zeit ohne eigenen Kopf |  |
| `ui.message_collapse_lines` |  | App |  | `24` | ab so vielen Zeilen wird eine Nachricht eingeklappt |  |
| `ui.livebox_visible_steps` |  | App |  | `6` | sichtbare Schritte in der laufenden Live-Box |  |
| `ui.composer_max_lines` |  | App |  | `12` | so weit wächst die Eingabe |  |
| `ui.autocomplete_debounce` |  | App |  | `150ms` | Wartezeit vor `command.autocomplete` |  |
| `ui.quick_switch_max_results` |  | App |  | `20` | Treffer im Schnellwechsel |  |
| `ui.desktop_min_width_four_columns` |  | App |  | `1280px` | darunter klappt die Sitzungs-Spalte ein |  |
| `ui.desktop_min_width_three_columns` |  | App |  | `960px` | darunter wird die Beitragsliste zum Stapel |  |
| `a11y.live_announce_min_interval` |  | App |  | `10s` | höchstens so oft sagt die Live-Box einen Wechsel an |  |
| `client.share.recent_targets` |  | App |  | `8` | zuletzt genutzte Ziele im Teilen-Blatt |  |

### 02 Architektur

Quelle: [02-architektur.md](../spec/02-architektur.md), Abschnitt 22.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `cluster.mode` | `BOTHY_CLUSTER__MODE` | Server | enum(single, multi) | `single` | `single` oder `multi` (mehrere api-Knoten) | aktiv |
| `http.read_header_timeout` | `BOTHY_HTTP__READ_HEADER_TIMEOUT` | Server | duration | `5s` | Frist für Kopfzeilen | aktiv |
| `http.request_timeout` | `BOTHY_HTTP__REQUEST_TIMEOUT` | Server | duration | `15s` | Kontext-Frist normaler Anfragen | aktiv |
| `http.transfer_timeout` | `BOTHY_HTTP__TRANSFER_TIMEOUT` | Server | duration | `10m` | Frist für Upload und Download | aktiv |
| `http.idle_timeout` | `BOTHY_HTTP__IDLE_TIMEOUT` | Server | duration | `120s` | Leerlauf von Keep-Alive-Verbindungen | aktiv |
| `http.max_header_bytes` | `BOTHY_HTTP__MAX_HEADER_BYTES` | Server | int | `65536` | größte Kopfzeilen | aktiv |
| `http.max_json_body_bytes` | `BOTHY_HTTP__MAX_JSON_BODY_BYTES` | Server | int | `1048576` | größter JSON-Körper; echt größer als `message.max_content_bytes` (13, Punkt 4) | aktiv |
| `http.max_json_depth` | `BOTHY_HTTP__MAX_JSON_DEPTH` | Server | int | `32` | größte Schachtelungstiefe eines JSON-Körpers (SEC-IN-2); ein Test hält, dass jedes Anfrage-Beispiel in `openapi.yaml` darunter bleibt | aktiv |
| `http.unavailable_retry_after` | `BOTHY_HTTP__UNAVAILABLE_RETRY_AFTER` | Server | duration | `5s` | `Retry-After` jeder Antwort `503 unavailable` (in ganzen Sekunden) | aktiv |
| `database.acquire_timeout` | `BOTHY_DATABASE__ACQUIRE_TIMEOUT` | Server | duration | `2s` | Warten auf eine Pool-Verbindung | aktiv |
| `database.statement_timeout` | `BOTHY_DATABASE__STATEMENT_TIMEOUT` | Server | duration | `5s` | Frist je SQL-Anweisung | aktiv |
| `database.lock_timeout` | `BOTHY_DATABASE__LOCK_TIMEOUT` | Server | duration | `2s` | Frist für Sperren | aktiv |
| `database.idle_in_tx_timeout` | `BOTHY_DATABASE__IDLE_IN_TX_TIMEOUT` | Server | duration | `10s` | offene Transaktion ohne Arbeit | aktiv |
| `database.tx_retry_max` | `BOTHY_DATABASE__TX_RETRY_MAX` | Server | int | `2` | Wiederholungen des Unit of Work bei `40P01` und `55P03` | aktiv |
| `database.listen_reconnect_backoff_cap` | `BOTHY_DATABASE__LISTEN_RECONNECT_BACKOFF_CAP` | Server | duration | `10s` | größter Abstand beim Neuaufbau von `LISTEN` | aktiv |
| `startup.dependency_wait` | `BOTHY_STARTUP__DEPENDENCY_WAIT` | Server | duration | `60s` | Warten auf die Datenbank beim Start | aktiv |
| `sync.log_poll_interval` | `BOTHY_SYNC__LOG_POLL_INTERVAL` | Server | duration | `1s` | Poll des Log-Tails ohne `NOTIFY` | aktiv |
| `sync.tail_batch_size` | `BOTHY_SYNC__TAIL_BATCH_SIZE` | Server | int | `1000` | Ereignisse je Lesevorgang des Log-Tails | aktiv |
| `sync.partition_maintenance_interval` | `BOTHY_SYNC__PARTITION_MAINTENANCE_INTERVAL` | Server | duration | `10m` | Abstand des Jobs `sync.partition_maintenance` | aktiv |
| `sync.event_log_partitions_ahead` | `BOTHY_SYNC__EVENT_LOG_PARTITIONS_AHEAD` | Server | int | `3` | Partitionen von `event_log`, die vor dem Kopf bereitstehen; mit `sync.event_log_partition_size` reichen sie bei `perf.event_commit_min_per_s` länger als zwei Abstände `sync.partition_maintenance_interval` | aktiv |
| `gateway.write_timeout` | `BOTHY_GATEWAY__WRITE_TIMEOUT` | Server | duration | `10s` | Frist für einen WebSocket-Schreibvorgang | aktiv |
| `consumer.batch_size` | `BOTHY_CONSUMER__BATCH_SIZE` | Server | int | `500` | Ereignisse je Runde eines Worker-Verbrauchers | aktiv |
| `consumer.poll_interval` | `BOTHY_CONSUMER__POLL_INTERVAL` | Server | duration | `1s` | Poll eines Worker-Verbrauchers ohne `NOTIFY` | aktiv |
| `consumer.pending_alert_after` | `BOTHY_CONSUMER__PENDING_ALERT_AFTER` | Server | duration | `15m` | age of the oldest unprocessed event of a worker consumer that raises the alarm `BothyEventConsumerLag`; strictly below `sync.event_retention` (13, item 4) | aktiv |
| `sync.idempotency_cleanup_interval` | `BOTHY_SYNC__IDEMPOTENCY_CLEANUP_INTERVAL` | Server | duration | `1h` | Abstand des Jobs `sync.idempotency_cleanup` | aktiv |
| `sync.idempotency_cleanup_batch_size` | `BOTHY_SYNC__IDEMPOTENCY_CLEANUP_BATCH_SIZE` | Server | int | `1000` | Zeilen je Löschstapel von `sync.idempotency_cleanup` | aktiv |
| `ratelimit.max_tracked_keys` | `BOTHY_RATELIMIT__MAX_TRACKED_KEYS` | Server | int | `100000` | höchstens so viele Rate-Limit-Eimer hält ein Prozess; darüber werden volle, dann älteste Eimer entfernt | aktiv |
| `ratelimit.sweep_interval` | `BOTHY_RATELIMIT__SWEEP_INTERVAL` | Server | duration | `1m` | Abstand, in dem volle Eimer entfernt werden (ein voller Eimer entscheidet wie ein fehlender) | aktiv |
| `jobs.schema` | `BOTHY_JOBS__SCHEMA` | Server | string | `jobs` | PostgreSQL-Schema der River-Tabellen | aktiv |
| `jobs.queue.default.max_workers` | `BOTHY_JOBS__QUEUE__DEFAULT__MAX_WORKERS` | Server | int | `8` | Parallelität `default` | aktiv |
| `jobs.queue.files.max_workers` | `BOTHY_JOBS__QUEUE__FILES__MAX_WORKERS` | Server | int | `2` | Parallelität `files` | aktiv |
| `jobs.queue.delivery.max_workers` | `BOTHY_JOBS__QUEUE__DELIVERY__MAX_WORKERS` | Server | int | `16` | Parallelität `delivery` | aktiv |
| `jobs.queue.maintenance.max_workers` | `BOTHY_JOBS__QUEUE__MAINTENANCE__MAX_WORKERS` | Server | int | `2` | Parallelität `maintenance` | aktiv |
| `jobs.queue.privacy.max_workers` | `BOTHY_JOBS__QUEUE__PRIVACY__MAX_WORKERS` | Server | int | `1` | Parallelität `privacy` | aktiv |
| `shutdown.drain_delay` | `BOTHY_SHUTDOWN__DRAIN_DELAY` | Server | duration | `5s` | Warten nach `ready = false` | aktiv |
| `shutdown.http_grace` | `BOTHY_SHUTDOWN__HTTP_GRACE` | Server | duration | `20s` | Frist für laufende HTTP-Anfragen | aktiv |
| `shutdown.jobs_soft_timeout` | `BOTHY_SHUTDOWN__JOBS_SOFT_TIMEOUT` | Server | duration | `20s` | weicher Stopp von River | aktiv |
| `shutdown.telemetry_flush_timeout` | `BOTHY_SHUTDOWN__TELEMETRY_FLUSH_TIMEOUT` | Server | duration | `5s` | Telemetrie leeren | aktiv |
| `shutdown.total_timeout` | `BOTHY_SHUTDOWN__TOTAL_TIMEOUT` | Server | duration | `60s` | gesamte Frist bis zum Ende; mindestens die Summe der Schritt-Budgets (15.3) | aktiv |
| `perf.reference_profile` |  | Lasttest |  | `4 vCPU, 8 GiB RAM, SSD, PostgreSQL 18 lokal mit fsync und synchronous_commit an, Rolle all` | Umgebung der Lasttests; Modi der Messung in ADR 0063 |  |
| `perf.sustained_messages_per_s` |  | Lasttest |  | `200` | Dauerlast für `perf.message_create_p99` |  |
| `perf.message_create_p99` |  | Lasttest |  | `150ms` | Ziel |  |
| `perf.event_commit_min_per_s` |  | Lasttest |  | `1000` | Ziel (Mindestwert) |  |
| `perf.fanout_local_p99` |  | Lasttest |  | `200ms` | Ziel |  |
| `perf.fanout_cross_node_p99` |  | Lasttest |  | `500ms` | Ziel |  |
| `perf.invocation_delivery_p99` |  | Lasttest |  | `500ms` | Ziel |  |
| `perf.run_patch_fanout_p99` |  | Lasttest |  | `500ms` | Ziel |  |
| `perf.catchup_page_p99` |  | Lasttest |  | `250ms` | Ziel |  |
| `perf.authorize_cached_p99` |  | Lasttest |  | `100us` | Ziel |  |
| `perf.ws_connections_per_node` |  | Lasttest |  | `5000` | Ziel (Mindestwert) |  |
| `perf.ws_idle_conn_mem_max` |  | Lasttest |  | `64KiB` | Ziel (Höchstwert) |  |
| `perf.idle_rss_max` |  | Lasttest |  | `256MiB` | Ziel (Höchstwert) |  |
| `perf.startup_ready_max` |  | Lasttest |  | `5s` | Ziel (Höchstwert) |  |
| `perf.push_handoff_p95` |  | Lasttest |  | `2s` | Ziel |  |

### 03 Datenmodell (PostgreSQL)

Quelle: [03-datenmodell.md](../spec/03-datenmodell.md), Abschnitt 9.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `principal.handle_max_chars` | `BOTHY_PRINCIPAL__HANDLE_MAX_CHARS` | Server | int | `32` | Länge eines Handles | aktiv |
| `principal.display_name_max_chars` | `BOTHY_PRINCIPAL__DISPLAY_NAME_MAX_CHARS` | Server | int | `64` | Länge eines Anzeigenamens in Unicode-Zeichen | aktiv |
| `branding.product_name_max_chars` | `BOTHY_BRANDING__PRODUCT_NAME_MAX_CHARS` | Server | int | `64` | Länge des Produktnamens im Branding (`instance_settings`, `branding.product_name`) in Unicode-Zeichen | aktiv |
| `group.name_max_chars` | `BOTHY_GROUP__NAME_MAX_CHARS` | Server | int | `100` | Länge eines Gruppennamens in Unicode-Zeichen | aktiv |
| `group.description_max_chars` | `BOTHY_GROUP__DESCRIPTION_MAX_CHARS` | Server | int | `500` | Länge einer Gruppenbeschreibung in Unicode-Zeichen | aktiv |
| `host.name_max_chars` | `BOTHY_HOST__NAME_MAX_CHARS` | Server | int | `100` | Länge eines Rechnernamens in Unicode-Zeichen | aktiv |
| `host.os_user_max_chars` | `BOTHY_HOST__OS_USER_MAX_CHARS` | Server | int | `64` | Länge des Namens eines Sitzungsbenutzers in `host_session_users.os_user` | aktiv |
| `space.name_max_chars` | `BOTHY_SPACE__NAME_MAX_CHARS` | Server | int | `100` | Länge eines Bereichsnamens in Unicode-Zeichen | aktiv |
| `channel.name_max_chars` | `BOTHY_CHANNEL__NAME_MAX_CHARS` | Server | int | `100` | Länge eines Kanalnamens in Unicode-Zeichen | aktiv |
| `channel.topic_max_chars` | `BOTHY_CHANNEL__TOPIC_MAX_CHARS` | Server | int | `1024` | Länge eines Kanal-Themas in Unicode-Zeichen | aktiv |
| `forum_tag.name_max_chars` | `BOTHY_FORUM_TAG__NAME_MAX_CHARS` | Server | int | `50` | Länge eines Tag-Namens in Unicode-Zeichen | aktiv |
| `identity.last_active_granularity` | `BOTHY_IDENTITY__LAST_ACTIVE_GRANULARITY` | Server | duration | `5m` | höchstens so oft wird `last_active_at` bzw. `last_used_at` geschrieben | aktiv |
| `auth.session_max_age` | `BOTHY_AUTH__SESSION_MAX_AGE` | Server | duration | `30d` | absolutes Ende einer Anmeldesitzung | aktiv |
| `auth.refresh_token_ttl` | `BOTHY_AUTH__REFRESH_TOKEN_TTL` | Server | duration | `14d` | Lebensdauer eines Refresh-Tokens | aktiv |
| `auth.step_up_max_age` | `BOTHY_AUTH__STEP_UP_MAX_AGE` | Server | duration | `5m` | so lange gilt eine Step-up-Bestätigung als frisch | aktiv |
| `auth.refresh_reuse_grace` | `BOTHY_AUTH__REFRESH_REUSE_GRACE` | Server | duration | `30s` | Nachsicht für die unmittelbar vorige Refresh-Generation mit demselben DPoP-Schlüssel | aktiv |
| `agent.heartbeat_persist_interval` | `BOTHY_AGENT__HEARTBEAT_PERSIST_INTERVAL` | Server | duration | `60s` | höchstens so oft werden Heartbeat-Felder gespeichert | aktiv |
| `channel.history_backfill_limit_default` | `BOTHY_CHANNEL__HISTORY_BACKFILL_LIMIT_DEFAULT` | Server | int | `20` | Standard für `history_backfill_limit` | aktiv |
| `channel.history_backfill_limit_max` | `BOTHY_CHANNEL__HISTORY_BACKFILL_LIMIT_MAX` | Server | int | `200` | Obergrenze für `history_backfill_limit` | aktiv |
| `channel.prompt_max_bytes` | `BOTHY_CHANNEL__PROMPT_MAX_BYTES` | Server | int | `32768` | Größe eines Kanal-Prompts | aktiv |
| `thread.title_max_chars` | `BOTHY_THREAD__TITLE_MAX_CHARS` | Server | int | `200` | Länge eines Thread-Titels | aktiv |
| `thread.max_tags` | `BOTHY_THREAD__MAX_TAGS` | Server | int | `5` | Forum-Tags je Thread | aktiv |
| `message.max_content_bytes` | `BOTHY_MESSAGE__MAX_CONTENT_BYTES` | Server | int | `262144` | Größe einer Nachricht (lange Antworten ohne Teilen) | aktiv |
| `message.max_attachments` | `BOTHY_MESSAGE__MAX_ATTACHMENTS` | Server | int | `10` | Anhänge je Nachricht | aktiv |
| `message.max_mentions` | `BOTHY_MESSAGE__MAX_MENTIONS` | Server | int | `50` | verschiedene Erwähnungs-Handles je Nachricht (Inhaltsformat, `messages`) | aktiv |
| `reaction.emoji_max_bytes` | `BOTHY_REACTION__EMOJI_MAX_BYTES` | Server | int | `64` | größte Länge eines Reaktions-Emoji in Bytes (ein Graphem-Cluster hat sonst keine Grenze) | aktiv |
| `reaction.max_distinct_per_message` | `BOTHY_REACTION__MAX_DISTINCT_PER_MESSAGE` | Server | int | `20` | verschiedene Emoji je Nachricht; begrenzt `Message.reactions` und die Nutzlast der Ereignisse | aktiv |
| `pin.max_per_thread` | `BOTHY_PIN__MAX_PER_THREAD` | Server | int | `50` | Anheftungen je Thread; `listPins` ist nicht seitenweise | aktiv |
| `template.name_max_chars` | `BOTHY_TEMPLATE__NAME_MAX_CHARS` | Server | int | `100` | Länge eines Vorlagennamens in Unicode-Zeichen | aktiv |
| `template.max_per_scope` | `BOTHY_TEMPLATE__MAX_PER_SCOPE` | Server | int | `100` | Vorlagen je Person bzw. je Kanal; `listTemplates` ist nicht seitenweise | aktiv |
| `file.max_upload_bytes` | `BOTHY_FILE__MAX_UPLOAD_BYTES` | Server | int | `104857600` | Größe einer Datei | aktiv |
| `run.excerpt_max_chars` | `BOTHY_RUN__EXCERPT_MAX_CHARS` | Server |  | `500` | Länge von `last_text_excerpt` | geplant (M2) |
| `run.step_detail_max_bytes` | `BOTHY_RUN__STEP_DETAIL_MAX_BYTES` | Server | int | `16384` | Größe von `run_steps.detail` | aktiv |
| `run.max_steps_persisted` | `BOTHY_RUN__MAX_STEPS_PERSISTED` | Server |  | `2000` | gespeicherte Schritte je Lauf | geplant (M2) |
| `run.stall_after` | `BOTHY_RUN__STALL_AFTER` | Server |  | `10m` | ohne Fortschritt: Stillstandshinweis | geplant (M2) |
| `component.default_ttl` | `BOTHY_COMPONENT__DEFAULT_TTL` | Server |  | `24h` | Ablauf einer Komponente ohne eigene Angabe | geplant (M2) |
| `push.max_consecutive_failures` | `BOTHY_PUSH__MAX_CONSECUTIVE_FAILURES` | Server |  | `5` | danach wird eine Push-Registrierung deaktiviert | geplant (M3) |
| `webhook.allowed_hosts` | `BOTHY_WEBHOOK__ALLOWED_HOSTS` | Server | list(string) | leer | erlaubte Ziel-Hosts für ausgehende Webhooks | aktiv |
| `webhook.max_consecutive_failures` | `BOTHY_WEBHOOK__MAX_CONSECUTIVE_FAILURES` | Server | int | `20` | danach wird ein Abo deaktiviert | aktiv |
| `webhook.retry_backoff_base` | `BOTHY_WEBHOOK__RETRY_BACKOFF_BASE` | Server | duration | `10s` | Basis des Backoffs | aktiv |
| `webhook.retry_backoff_cap` | `BOTHY_WEBHOOK__RETRY_BACKOFF_CAP` | Server | duration | `1h` | größter Abstand | aktiv |
| `sync.event_log_partition_size` | `BOTHY_SYNC__EVENT_LOG_PARTITION_SIZE` | Server | int | `1000000` | Sequenznummern je Partition | aktiv |

### 04 Ereignis- und Sync-Protokoll

Quelle: [04-sync-protokoll.md](../spec/04-sync-protokoll.md), Abschnitt 18.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `sync.event_retention` | `BOTHY_SYNC__EVENT_RETENTION` | Server | duration | `30d` | so lange bleiben dauerhafte Ereignisse nachholbar | aktiv |
| `sync.event_payload_max_bytes` | `BOTHY_SYNC__EVENT_PAYLOAD_MAX_BYTES` | Server | int | `65536` | größte Nutzlast eines Ereignisses | aktiv |
| `sync.ws_max_frame_bytes` | `BOTHY_SYNC__WS_MAX_FRAME_BYTES` | Server | int | `1048576` | größter WebSocket-Rahmen | aktiv |
| `sync.identify_timeout` | `BOTHY_SYNC__IDENTIFY_TIMEOUT` | Server | duration | `10s` | Zeit zwischen `hello` und `identify` | aktiv |
| `sync.heartbeat_interval` | `BOTHY_SYNC__HEARTBEAT_INTERVAL` | Server | duration | `30s` | Abstand der Heartbeats (in `hello` gemeldet) | aktiv |
| `sync.heartbeat_miss_limit` | `BOTHY_SYNC__HEARTBEAT_MISS_LIMIT` | Server | int | `2` | fehlende Heartbeats bis zum Schließen | aktiv |
| `sync.resume_window` | `BOTHY_SYNC__RESUME_WINDOW` | Server | duration | `5m` | so lange ist `resume` nach Trennung möglich | aktiv |
| `sync.reauth_lead_time` | `BOTHY_SYNC__REAUTH_LEAD_TIME` | Server | duration | `60s` | Vorlauf von `reauth_required` vor Token-Ablauf | aktiv |
| `sync.reconnect_backoff_base` | `BOTHY_SYNC__RECONNECT_BACKOFF_BASE` | Server |  | `1s` | Basis des Backoffs beim Wiederverbinden | geplant (M2) |
| `sync.reconnect_backoff_cap` | `BOTHY_SYNC__RECONNECT_BACKOFF_CAP` | Server |  | `60s` | größter Abstand beim Wiederverbinden | geplant (M2) |
| `sync.catchup_page_size` | `BOTHY_SYNC__CATCHUP_PAGE_SIZE` | Server | int | `500` | Ereignisse je Nachhol-Seite | aktiv |
| `sync.catchup_max_events` | `BOTHY_SYNC__CATCHUP_MAX_EVENTS` | Server | int | `50000` | mehr Rückstand ergibt `resync_required` | aktiv |
| `sync.cursor_advance_every` | `BOTHY_SYNC__CURSOR_ADVANCE_EVERY` | Server | int | `1000` | nach so vielen übersprungenen Ereignissen ein `cursor`-Rahmen | aktiv |
| `sync.outbound_queue_max` | `BOTHY_SYNC__OUTBOUND_QUEUE_MAX` | Server | int | `10000` | Rahmen im Ausgang einer Verbindung bis `4014` | aktiv |
| `sync.idempotency_key_ttl` | `BOTHY_SYNC__IDEMPOTENCY_KEY_TTL` | Server | duration | `24h` | Aufbewahrung von `Idempotency-Key` | aktiv |
| `sync.max_focused_threads` | `BOTHY_SYNC__MAX_FOCUSED_THREADS` | Server | int | `20` | Threads im `focus` je Verbindung | aktiv |
| `sync.run_patch_coalesce_interval` | `BOTHY_SYNC__RUN_PATCH_COALESCE_INTERVAL` | Server |  | `250ms` | Sammelfenster für `run.updated` | geplant (M2) |
| `sync.typing_min_interval` | `BOTHY_SYNC__TYPING_MIN_INTERVAL` | Server | duration | `5s` | höchstens so oft `typing.start` | aktiv |
| `sync.typing_ttl` | `BOTHY_SYNC__TYPING_TTL` | Server | duration | `8s` | Tipp-Anzeige ohne Erneuerung | aktiv |
| `client.cache_max_age` | `BOTHY_CLIENT__CACHE_MAX_AGE` | Server | duration | `30d` | ungenutzte Cache-Einträge | aktiv |
| `client.outbox_retry_base` | `BOTHY_CLIENT__OUTBOX_RETRY_BASE` | Server |  | `2s` | Basis des Backoffs im Ausgangsfach | geplant (M4) |
| `client.outbox_retry_cap` | `BOTHY_CLIENT__OUTBOX_RETRY_CAP` | Server |  | `5m` | größter Abstand im Ausgangsfach | geplant (M4) |
| `agent.invocation_token_ttl` | `BOTHY_AGENT__INVOCATION_TOKEN_TTL` | Server | duration | `60s` | Gültigkeit einer Aufruf-Marke | aktiv |
| `agent.ack_timeout` | `BOTHY_AGENT__ACK_TIMEOUT` | Server | duration | `30s` | ohne `ack`: erneute Zustellung | aktiv |
| `agent.ack_persist_interval` | `BOTHY_AGENT__ACK_PERSIST_INTERVAL` | Server | duration | `1s` | spätestens so oft wird `acked_seq` gespeichert | aktiv |
| `agent.max_inflight_events` | `BOTHY_AGENT__MAX_INFLIGHT_EVENTS` | Server | int | `100` | unbestätigte Ereignisse je Verbraucher | aktiv |
| `agent.run_update_min_interval` | `BOTHY_AGENT__RUN_UPDATE_MIN_INTERVAL` | Server |  | `500ms` | Adapter-Drossel für `run.update` | geplant (M2) |
| `agent.presence_timeout` | `BOTHY_AGENT__PRESENCE_TIMEOUT` | Server | duration | `90s` | ohne Heartbeat gilt der Agent als `offline` | aktiv |
| `agent.cancel_grace` | `BOTHY_AGENT__CANCEL_GRACE` | Server |  | `30s` | Zeit für den Adapter, einen Abbruch zu melden | geplant (M2) |

### 05 Aktionskatalog der Autorisierung

Quelle: [05-aktionen.md](../spec/05-aktionen.md), Abschnitt 10.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `audit.deny_aggregate_window` | `BOTHY_AUDIT__DENY_AGGREGATE_WINDOW` | Server | duration | `60s` | Zusammenfassungsfenster für gleiche Ablehnungen | aktiv |
| `audit.deny_aggregate_max_keys` | `BOTHY_AUDIT__DENY_AGGREGATE_MAX_KEYS` | Server | int | `10000` | höchstens so viele offene Zusammenfassungen je Prozess; begrenzt den Speicher bei Fluten ohne Anmeldung | aktiv |
| `status.public_enabled` | `BOTHY_STATUS__PUBLIC_ENABLED` | Server |  | `false` | Statusseite ohne Anmeldung | geplant (M7) |
| `ratelimit.rl.public_auth.per_minute` | `BOTHY_RATELIMIT__RL__PUBLIC_AUTH__PER_MINUTE` | Server | int | `30` | je Client-IP | aktiv |
| `ratelimit.rl.public_auth.burst` | `BOTHY_RATELIMIT__RL__PUBLIC_AUTH__BURST` | Server | int | `10` | je Client-IP | aktiv |
| `ratelimit.rl.gateway_connect.per_minute` | `BOTHY_RATELIMIT__RL__GATEWAY_CONNECT__PER_MINUTE` | Server | int | `12` | je Gerät oder Token | aktiv |
| `ratelimit.rl.gateway_connect.burst` | `BOTHY_RATELIMIT__RL__GATEWAY_CONNECT__BURST` | Server | int | `4` | je Gerät oder Token | aktiv |
| `ratelimit.rl.read.per_minute` | `BOTHY_RATELIMIT__RL__READ__PER_MINUTE` | Server | int | `600` | je Prinzipal | aktiv |
| `ratelimit.rl.read.burst` | `BOTHY_RATELIMIT__RL__READ__BURST` | Server | int | `100` | je Prinzipal | aktiv |
| `ratelimit.rl.write.per_minute` | `BOTHY_RATELIMIT__RL__WRITE__PER_MINUTE` | Server | int | `120` | je Prinzipal | aktiv |
| `ratelimit.rl.write.burst` | `BOTHY_RATELIMIT__RL__WRITE__BURST` | Server | int | `30` | je Prinzipal | aktiv |
| `ratelimit.rl.message.per_minute` | `BOTHY_RATELIMIT__RL__MESSAGE__PER_MINUTE` | Server | int | `60` | je Prinzipal | aktiv |
| `ratelimit.rl.message.burst` | `BOTHY_RATELIMIT__RL__MESSAGE__BURST` | Server | int | `20` | je Prinzipal | aktiv |
| `ratelimit.rl.trigger.per_minute` | `BOTHY_RATELIMIT__RL__TRIGGER__PER_MINUTE` | Server | int | `30` | je Prinzipal | aktiv |
| `ratelimit.rl.trigger.burst` | `BOTHY_RATELIMIT__RL__TRIGGER__BURST` | Server | int | `10` | je Prinzipal | aktiv |
| `ratelimit.rl.interaction.per_minute` | `BOTHY_RATELIMIT__RL__INTERACTION__PER_MINUTE` | Server | int | `60` | je Prinzipal | aktiv |
| `ratelimit.rl.interaction.burst` | `BOTHY_RATELIMIT__RL__INTERACTION__BURST` | Server | int | `20` | je Prinzipal | aktiv |
| `ratelimit.rl.upload.per_minute` | `BOTHY_RATELIMIT__RL__UPLOAD__PER_MINUTE` | Server | int | `30` | je Prinzipal | aktiv |
| `ratelimit.rl.upload.burst` | `BOTHY_RATELIMIT__RL__UPLOAD__BURST` | Server | int | `10` | je Prinzipal | aktiv |
| `ratelimit.rl.search.per_minute` | `BOTHY_RATELIMIT__RL__SEARCH__PER_MINUTE` | Server | int | `60` | je Prinzipal | aktiv |
| `ratelimit.rl.search.burst` | `BOTHY_RATELIMIT__RL__SEARCH__BURST` | Server | int | `20` | je Prinzipal | aktiv |
| `ratelimit.rl.typing.per_minute` | `BOTHY_RATELIMIT__RL__TYPING__PER_MINUTE` | Server | int | `30` | je Verbindung | aktiv |
| `ratelimit.rl.typing.burst` | `BOTHY_RATELIMIT__RL__TYPING__BURST` | Server | int | `5` | je Verbindung | aktiv |
| `ratelimit.rl.admin.per_minute` | `BOTHY_RATELIMIT__RL__ADMIN__PER_MINUTE` | Server | int | `60` | je Prinzipal | aktiv |
| `ratelimit.rl.admin.burst` | `BOTHY_RATELIMIT__RL__ADMIN__BURST` | Server | int | `20` | je Prinzipal | aktiv |
| `ratelimit.rl.export.per_minute` | `BOTHY_RATELIMIT__RL__EXPORT__PER_MINUTE` | Server | int | `2` | je Prinzipal | aktiv |
| `ratelimit.rl.export.burst` | `BOTHY_RATELIMIT__RL__EXPORT__BURST` | Server | int | `2` | je Prinzipal | aktiv |
| `ratelimit.rl.agent_runtime.per_minute` | `BOTHY_RATELIMIT__RL__AGENT_RUNTIME__PER_MINUTE` | Server | int | `1200` | je Agent | aktiv |
| `ratelimit.rl.agent_runtime.burst` | `BOTHY_RATELIMIT__RL__AGENT_RUNTIME__BURST` | Server | int | `200` | je Agent | aktiv |
| `ratelimit.rl.webhook_in.per_minute` | `BOTHY_RATELIMIT__RL__WEBHOOK_IN__PER_MINUTE` | Server | int | `60` | je Eingangs-Webhook | aktiv |
| `ratelimit.rl.webhook_in.burst` | `BOTHY_RATELIMIT__RL__WEBHOOK_IN__BURST` | Server | int | `20` | je Eingangs-Webhook | aktiv |
| `ratelimit.rl.import.per_minute` | `BOTHY_RATELIMIT__RL__IMPORT__PER_MINUTE` | Server | int | `120` | je Owner für `import.apply` | aktiv |
| `ratelimit.rl.import.burst` | `BOTHY_RATELIMIT__RL__IMPORT__BURST` | Server | int | `30` | je Owner für `import.apply` | aktiv |
| `ratelimit.rl.terminal.per_minute` | `BOTHY_RATELIMIT__RL__TERMINAL__PER_MINUTE` | Server | int | `20` | je Mensch | aktiv |
| `ratelimit.rl.terminal.burst` | `BOTHY_RATELIMIT__RL__TERMINAL__BURST` | Server | int | `5` | je Mensch | aktiv |
| `ratelimit.rl.host.per_minute` | `BOTHY_RATELIMIT__RL__HOST__PER_MINUTE` | Server | int | `120` | je Rechner | aktiv |
| `ratelimit.rl.host.burst` | `BOTHY_RATELIMIT__RL__HOST__BURST` | Server | int | `30` | je Rechner | aktiv |

### 06 Öffentliche API

Quelle: [06-api.md](../spec/06-api.md), Abschnitt 20.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `api.page_size_default` | `BOTHY_API__PAGE_SIZE_DEFAULT` | Server | int | `50` | Seitengröße ohne `limit` | aktiv |
| `api.page_size_max` | `BOTHY_API__PAGE_SIZE_MAX` | Server | int | `200` | größere `limit` werden gekappt | aktiv |
| `api.cors_allowed_origins` | `BOTHY_API__CORS_ALLOWED_ORIGINS` | Server | list(string) | leer | zusätzliche Origins für CORS | aktiv |
| `api.deprecation_min_notice` | `BOTHY_API__DEPRECATION_MIN_NOTICE` | Server |  | `180d` | Mindestabstand von `Deprecation` zu `Sunset` | geplant (M1) |
| `api.major_version_overlap` | `BOTHY_API__MAJOR_VERSION_OVERLAP` | Server |  | `365d` | so lange bleibt eine alte Hauptversion nach Erscheinen der neuen | geplant (M7) |
| `api.stats_max_buckets` | `BOTHY_API__STATS_MAX_BUCKETS` | Server |  | `1000` | Zeit-Buckets je Statistik-Abfrage; mehr ergibt `400` | geplant (M6) |
| `message.list_content_max_bytes` | `BOTHY_MESSAGE__LIST_CONTENT_MAX_BYTES` | Server | int | `16384` | größter `content_md` je Nachricht in Listen; darüber gekürzt mit `content_truncated` (Abschnitt 6) | aktiv |
| `status.history_window` | `BOTHY_STATUS__HISTORY_WINDOW` | Server |  | `7d` | so lange zeigt die Statusseite gelöste Störungen | geplant (M7) |

### 07 Rechte und Autorisierung

Quelle: [07-rechte.md](../spec/07-rechte.md), Abschnitt 14.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `authz.expiry_sweep_interval` | `BOTHY_AUTHZ__EXPIRY_SWEEP_INTERVAL` | Server | duration | `1m` | Abstand des Jobs, der Abläufe meldet und Erinnerungen schreibt | aktiv |
| `authz.expiry_reminder_lead` | `BOTHY_AUTHZ__EXPIRY_REMINDER_LEAD` | Server | duration | `48h` | so lange vor Ablauf kommt die Erinnerung (Studie C: zwei Tage) | aktiv |
| `authz.expiry_sweep_batch_max` | `BOTHY_AUTHZ__EXPIRY_SWEEP_BATCH_MAX` | Server | int | `500` | Zeilen je Transaktion der Jobs `authz.expiry_sweep` und `authz.expiry_reminder` | aktiv |
| `authz.view_as_max_duration` | `BOTHY_AUTHZ__VIEW_AS_MAX_DURATION` | Server | duration | `15m` | Höchstdauer von "Als Person ansehen" | aktiv |
| `authz.snapshot_cache_max_entries` | `BOTHY_AUTHZ__SNAPSHOT_CACHE_MAX_ENTRIES` | Server | int | `10000` | Rechte-Stände im Speicher je Prozess (LRU) | aktiv |
| `authz.not_found_timing_tolerance` | `BOTHY_AUTHZ__NOT_FOUND_TIMING_TOLERANCE` | Server |  | `20ms` | erlaubte Abweichung des Medians in AZ-P4 | geplant (M1) |

### 08 Identität, Geräte und Sitzungen

Quelle: [08-identitaet.md](../spec/08-identitaet.md), Abschnitt 18.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `auth.access_token_ttl` | `BOTHY_AUTH__ACCESS_TOKEN_TTL` | Server | duration | `10m` | Lebensdauer eines Zugriffstokens | aktiv |
| `auth.dpop_nonce_ttl` | `BOTHY_AUTH__DPOP_NONCE_TTL` | Server | duration | `5m` | Gültigkeit eines DPoP-Nonce (Kopfzeile `DPoP-Nonce` und `hello` des Gateways) | aktiv |
| `auth.token_pepper_ref` | `BOTHY_AUTH__TOKEN_PEPPER_REF` | Server | secret_ref | `file:<secrets.dir>/token_pepper` | Verweis auf den Server-Pepper für die HMAC-Hashes aller Tokens und Codes (03, Grundsatz 9) | aktiv |
| `auth.clock_leeway` | `BOTHY_AUTH__CLOCK_LEEWAY` | Server | duration | `30s` | erlaubte Uhrabweichung bei `exp`, `nbf`, `iat` | aktiv |
| `auth.dpop_proof_max_age` | `BOTHY_AUTH__DPOP_PROOF_MAX_AGE` | Server | duration | `60s` | Höchstalter eines DPoP-Beweises (`iat`) | aktiv |
| `auth.login_flow_ttl` | `BOTHY_AUTH__LOGIN_FLOW_TTL` | Server | duration | `10m` | Gültigkeit eines Anmeldevorgangs | aktiv |
| `auth.server_code_ttl` | `BOTHY_AUTH__SERVER_CODE_TTL` | Server | duration | `60s` | Gültigkeit des einmaligen Server-Codes | aktiv |
| `auth.oidc_jwks_cache_ttl` | `BOTHY_AUTH__OIDC_JWKS_CACHE_TTL` | Server | duration | `1h` | Cache der Anbieter-Metadaten (Discovery) und -Schlüssel (JWKS) | aktiv |
| `auth.oidc_client_assertion_ttl` | `BOTHY_AUTH__OIDC_CLIENT_ASSERTION_TTL` | Server | duration | `2m` | Gültigkeit einer Client-Assertion bei `private_key_jwt` | aktiv |
| `auth.group_claim_max_age` | `BOTHY_AUTH__GROUP_CLAIM_MAX_AGE` | Server | duration | `30d` | Ablauf von Mitgliedschaften aus Gruppen-Claims nach der letzten Anmeldung | aktiv |
| `auth.idp_recheck_interval` | `BOTHY_AUTH__IDP_RECHECK_INTERVAL` | Server | duration | `30d` | spätestens dann neue Anmeldung beim Anbieter | aktiv |
| `auth.trusted_header_bind_requests` | `BOTHY_AUTH__TRUSTED_HEADER_BIND_REQUESTS` | Server | bool | `true` | jede Anfrage muss denselben Header-Login tragen | aktiv |
| `auth.enrollment_code_ttl` | `BOTHY_AUTH__ENROLLMENT_CODE_TTL` | Server | duration | `72h` | Gültigkeit eines Einladungs-Codes | aktiv |
| `auth.enrollment_code_max_attempts` | `BOTHY_AUTH__ENROLLMENT_CODE_MAX_ATTEMPTS` | Server | int | `5` | Fehlversuche je Einladungs-Code | aktiv |
| `auth.device_pending_ttl` | `BOTHY_AUTH__DEVICE_PENDING_TTL` | Server | duration | `24h` | danach wird ein wartendes Gerät widerrufen | aktiv |
| `auth.device_verification_code_digits` | `BOTHY_AUTH__DEVICE_VERIFICATION_CODE_DIGITS` | Server | int | `6` | Länge des Bestätigungs-Codes | aktiv |
| `auth.pairing_code_ttl` | `BOTHY_AUTH__PAIRING_CODE_TTL` | Server | duration | `5m` | Gültigkeit eines Kopplungs-Codes | aktiv |
| `auth.pairing_code_chars` | `BOTHY_AUTH__PAIRING_CODE_CHARS` | Server | int | `8` | Länge des Kopplungs-Codes (Crockford-Base32, 40 Bit) | aktiv |
| `auth.device_inactive_revoke_after` | `BOTHY_AUTH__DEVICE_INACTIVE_REVOKE_AFTER` | Server | duration | `180d` | Gerät ohne Kontakt wird widerrufen | aktiv |
| `auth.cleanup_interval` | `BOTHY_AUTH__CLEANUP_INTERVAL` | Server | duration | `1h` | Abstand des Jobs `identity.auth_cleanup` (abgelaufene Herausforderungen, Anmeldevorgänge und Kopplungen löschen, wartende und ungenutzte Geräte widerrufen) | aktiv |
| `auth.step_up_challenge_ttl` | `BOTHY_AUTH__STEP_UP_CHALLENGE_TTL` | Server | duration | `2m` | Gültigkeit einer Step-up-Herausforderung | aktiv |
| `auth.step_up_fail_limit` | `BOTHY_AUTH__STEP_UP_FAIL_LIMIT` | Server | int | `5` | Fehlversuche beim Step-up je Fenster | aktiv |
| `auth.require_key_attestation` | `BOTHY_AUTH__REQUIRE_KEY_ATTESTATION` | Server |  | `false` | Plattform-Attestierung der Geräteschlüssel verlangen | geplant (V3) |
| `auth.console_session_max_age` | `BOTHY_AUTH__CONSOLE_SESSION_MAX_AGE` | Server | duration | `12h` | absolute Dauer einer Konsolen-Sitzung | aktiv |
| `auth.console_session_idle_timeout` | `BOTHY_AUTH__CONSOLE_SESSION_IDLE_TIMEOUT` | Server | duration | `30m` | Leerlauf bis zum Ende der Konsolen-Sitzung | aktiv |
| `auth.local_password_enabled` | `BOTHY_AUTH__LOCAL_PASSWORD_ENABLED` | Server | bool | `true` | Passwort mit TOTP als Rückfall für lokale Konten | aktiv |
| `auth.totp_key_ref` | `BOTHY_AUTH__TOTP_KEY_REF` | Server | secret_ref | `file:<secrets.dir>/totp_key` | Verweis auf den Schlüssel (32 Byte), mit dem TOTP-Geheimnisse verschlüsselt werden | aktiv |
| `auth.webauthn_rp_id` | `BOTHY_AUTH__WEBAUTHN_RP_ID` | Server | string | leer | RP-ID der Passkeys; leer heißt Host von `server.public_url` | aktiv |
| `auth.webauthn_origins` | `BOTHY_AUTH__WEBAUTHN_ORIGINS` | Server | list(string) | leer | erlaubte Origins der Passkeys; leer heißt Origin von `server.public_url`; Einträge dürfen auch App-Origins wie `android:apk-key-hash:...` sein | aktiv |
| `auth.password_min_length` | `BOTHY_AUTH__PASSWORD_MIN_LENGTH` | Server | int | `12` | Mindestlänge eines Passworts | aktiv |
| `auth.argon2id_memory_kib` | `BOTHY_AUTH__ARGON2ID_MEMORY_KIB` | Server | int | `65536` | Argon2id-Speicher (Untergrenze 19456) | aktiv |
| `auth.argon2id_iterations` | `BOTHY_AUTH__ARGON2ID_ITERATIONS` | Server | int | `3` | Argon2id-Durchläufe (Untergrenze 2) | aktiv |
| `auth.argon2id_parallelism` | `BOTHY_AUTH__ARGON2ID_PARALLELISM` | Server | int | `1` | Argon2id-Parallelität | aktiv |
| `auth.login_fail_limit` | `BOTHY_AUTH__LOGIN_FAIL_LIMIT` | Server | int | `10` | Fehlversuche je Konto und Fenster | aktiv |
| `auth.login_fail_window` | `BOTHY_AUTH__LOGIN_FAIL_WINDOW` | Server | duration | `15m` | Fenster für Fehlversuche | aktiv |
| `auth.login_lock_duration` | `BOTHY_AUTH__LOGIN_LOCK_DURATION` | Server | duration | `15m` | Sperre des Passwortwegs | aktiv |
| `auth.dpop_required_for` | `BOTHY_AUTH__DPOP_REQUIRED_FOR` | Server | list(string) | `["agent"]` | Token-Arten, die DPoP-gebunden sein müssen | aktiv |
| `auth.api_token_max_ttl` | `BOTHY_AUTH__API_TOKEN_MAX_TTL` | Server | duration | `365d` | Höchstdauer eines Agenten- oder App-Tokens | aktiv |
| `auth.personal_token_max_ttl` | `BOTHY_AUTH__PERSONAL_TOKEN_MAX_TTL` | Server | duration | `90d` | Höchstdauer eines persönlichen Tokens | aktiv |
| `client.app_lock_after` | `BOTHY_CLIENT__APP_LOCK_AFTER` | Server |  | `5m` | ohne Benutzung bis zur App-Sperre | geplant (M5) |
| `security.app_lock_required` | `BOTHY_SECURITY__APP_LOCK_REQUIRED` | Server |  | `false` | App-Sperre instanzweit verlangen | geplant (M5) |

### 09 Agenten

Quelle: [09-agenten.md](../spec/09-agenten.md), Abschnitt 21.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `agent.context_max_bytes` | `BOTHY_AGENT__CONTEXT_MAX_BYTES` | Server |  | `262144` | Summe der Inhalte im Kontext ohne auslösende Nachricht | geplant (M2) |
| `agent.context_message_max_bytes` | `BOTHY_AGENT__CONTEXT_MESSAGE_MAX_BYTES` | Server |  | `32768` | Inhalt je Kontext-Nachricht, darüber gekürzt | geplant (M2) |
| `agent.jwks_max_age` | `BOTHY_AGENT__JWKS_MAX_AGE` | Server | duration | `1h` | Cache-Dauer der JWKS; Vorlauf eines `next`-Schlüssels | aktiv |
| `signing_key.invocation_rotation_interval` | `BOTHY_SIGNING_KEY__INVOCATION_ROTATION_INTERVAL` | Server | duration | `30d` | planmäßiger Wechsel des Markenschlüssels | aktiv |
| `run.queued_max_age` | `BOTHY_RUN__QUEUED_MAX_AGE` | Server | duration | `48h` | so alt darf ein nicht gestarteter Lauf werden | aktiv |
| `run.orphan_timeout` | `BOTHY_RUN__ORPHAN_TIMEOUT` | Server |  | `15m` | ohne Heartbeat bei aktivem Lauf: `stale` | geplant (M2) |
| `interaction.ack_timeout` | `BOTHY_INTERACTION__ACK_TIMEOUT` | Server |  | `30s` | danach zeigt die Karte "nicht bestätigt" | geplant (M2) |
| `command.autocomplete_timeout` | `BOTHY_COMMAND__AUTOCOMPLETE_TIMEOUT` | Server |  | `2500ms` | Wartezeit auf Vorschläge | geplant (M2) |
| `command.autocomplete_max_choices` | `BOTHY_COMMAND__AUTOCOMPLETE_MAX_CHOICES` | Server |  | `25` | Vorschläge je Antwort | geplant (M2) |
| `command.max_per_principal` | `BOTHY_COMMAND__MAX_PER_PRINCIPAL` | Server | int | `100` | Befehle je Agent oder App | aktiv |
| `command.description_max_chars` | `BOTHY_COMMAND__DESCRIPTION_MAX_CHARS` | Server | int | `100` | Länge einer Beschreibung | aktiv |
| `component.max_per_message` | `BOTHY_COMPONENT__MAX_PER_MESSAGE` | Server |  | `25` | Komponenten je Nachricht | geplant (M2) |
| `component.custom_id_max_chars` | `BOTHY_COMPONENT__CUSTOM_ID_MAX_CHARS` | Server |  | `100` | Länge von `custom_id` | geplant (M2) |
| `component.form_max_fields` | `BOTHY_COMPONENT__FORM_MAX_FIELDS` | Server |  | `10` | Felder je Formular | geplant (M2) |
| `project.binding_text_max_chars` | `BOTHY_PROJECT__BINDING_TEXT_MAX_CHARS` | Server | int | `512` | longest `workspace_ref`, `repository_url`, `default_branch`, `runner_config.model` and tool name of a project binding in Unicode characters; 512 characters of at most 4 bytes in UTF-8, also after `lower()`, stay below the PostgreSQL B-tree entry limit of the unique index on `lower(workspace_ref)`, so an over-long reference is a field error, never an internal error (maximum `600`) | aktiv |
| `thread.placeholder_titles` | `BOTHY_THREAD__PLACEHOLDER_TITLES` | Server | list(string) | `["Neuer Chat", "New chat", "Chat", "Ohne Titel", "Untitled", "Test"]` | Platzhalter für die Titel-Erkennung; der leere Titel gilt immer als Platzhalter (12 Punkt 2), ein Listeneintrag ist nie leer | aktiv |
| `adapter.invocation_token_leeway` |  | Adapter |  | `10s` | Uhren-Toleranz bei `exp` und `iat`; höchstens `auth.clock_leeway` des Servers (4.5) |  |
| `adapter.jwks_refresh_min_interval` |  | Adapter |  | `60s` | höchstens so oft JWKS neu laden |  |
| `hermes_adapter.platform_key` |  | Hermes-Adapter |  | `bothy` | Plattformname in Hermes; nach Inbetriebnahme nie ändern |  |
| `hermes_adapter.consumer` |  | Hermes-Adapter |  | `default` | Verbrauchername am Gateway |  |
| `runner.claude_bin` |  | Runner |  | `claude` | Programm, gesucht im bereinigten `PATH` |  |
| `runner.projects_root` |  | Runner |  | kein Standard, Pflicht | Wurzel aller Projektordner |  |
| `runner.state_dir` |  | Runner |  | kein Standard, Pflicht | Zustand, Mitschnitte, Eingangsdateien |  |
| `runner.followup_grace` |  | Runner |  | `30s` | stdin bleibt nach `result` offen für Nachträge |  |
| `runner.stop_grace` |  | Runner |  | `10s` | nach SIGINT bis SIGTERM |  |
| `runner.kill_grace` |  | Runner |  | `5s` | nach SIGTERM bis SIGKILL |  |
| `runner.session_lock_wait` |  | Runner |  | `10m` | Wartezeit auf eine belegte Sitzung |  |
| `runner.max_parallel_runs` |  | Runner |  | `4` | gleichzeitige Turns |  |
| `runner.stream_log_retention` |  | Runner |  | `14d` | Aufbewahrung der Mitschnitte |  |
| `runner.attach_allowed_roots` |  | Runner |  | Projektordner, Eingangsordner, Runner-Temp | erlaubte Wurzeln für Antwort-Anhänge |  |
| `runner.attach_denied_patterns` |  | Runner |  | Liste in 18.10 | gesperrte Muster für Antwort-Anhänge |  |

### 10 Backend-Richtlinie

Quelle: [10-backend-richtlinie.md](../spec/10-backend-richtlinie.md), Abschnitt 17.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `billing.show_personal_estimates` | `BOTHY_BILLING__SHOW_PERSONAL_ESTIMATES` | Server |  | `false` | Beträge auch im Modus `personal` anzeigen, als Information | geplant (M2) |
| `billing.mismatch_alert_after` | `BOTHY_BILLING__MISMATCH_ALERT_AFTER` | Server | duration | `5m` | so lange darf Ist von Soll abweichen, bevor ein Alarm ausgeht | aktiv |
| `reconciler.min_request_interval` |  | Abgleich-App |  | `60s` | Abstand der Anfragen an die Cost-API |  |
| `reconciler.lookback` |  | Abgleich-App |  | `3d` | so viele Tage rückwärts wird abgeglichen |  |
| `reconciler.workspace_map` |  | Abgleich-App |  | leer | Zuordnung Arbeitsbereich zu Agent |  |

### 11 Erweiterungen: Apps, Webhooks, SDK

Quelle: [11-erweiterungen.md](../spec/11-erweiterungen.md), Abschnitt 16.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `app.manifest_max_bytes` | `BOTHY_APP__MANIFEST_MAX_BYTES` | Server | int | `65536` | Größe eines Manifests | aktiv |
| `app.interaction_timeout` | `BOTHY_APP__INTERACTION_TIMEOUT` | Server |  | `3s` | Frist für synchrone Antworten über `interactions_url` | geplant (M2) |
| `webhook.delivery_timeout` | `BOTHY_WEBHOOK__DELIVERY_TIMEOUT` | Server | duration | `15s` | Zeitgrenze einer Zustellung | aktiv |
| `webhook.max_pending_per_subscription` | `BOTHY_WEBHOOK__MAX_PENDING_PER_SUBSCRIPTION` | Server | int | `10000` | offene Zustellungen je Abo, darüber `paused` | aktiv |
| `webhook.retry_max_age` | `BOTHY_WEBHOOK__RETRY_MAX_AGE` | Server | duration | `72h` | after that a delivery becomes `dead` ([webhooks.md](../api/webhooks.md), section 4) | aktiv |
| `webhook.secret_rotation_overlap` | `BOTHY_WEBHOOK__SECRET_ROTATION_OVERLAP` | Server | duration | `24h` | how long the server signs with the old and the new HMAC generation | aktiv |
| `webhook.hmac_master_key_ref` | `BOTHY_WEBHOOK__HMAC_MASTER_KEY_REF` | Server | secret_ref | `file:<secrets.dir>/webhook_hmac_master_key` | reference to the master key from which the HMAC secrets of the subscriptions are derived ([webhooks.md](../api/webhooks.md), section 2.1) | aktiv |
| `webhook_in.max_body_bytes` | `BOTHY_WEBHOOK_IN__MAX_BODY_BYTES` | Server |  | `1048576` | Größe eines eingehenden Körpers | geplant (M2) |
| `webhook_in.formats_enabled` | `BOTHY_WEBHOOK_IN__FORMATS_ENABLED` | Server |  | `native, alertmanager` | eingeschaltete Eingangsformate | geplant (M2) |
| `sdk.http_timeout` |  | SDK |  | `30s` | Zeitgrenze einer REST-Anfrage |  |
| `sdk.retry_max_attempts` |  | SDK |  | `5` | Versuche je idempotenter Operation |  |
| `sdk.retry_backoff_base` |  | SDK |  | `500ms` | Basis des Backoffs |  |
| `sdk.retry_backoff_cap` |  | SDK |  | `30s` | größter Abstand |  |

### 12 Dateien

Quelle: [12-dateien.md](../spec/12-dateien.md), Abschnitt 18.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `file.storage.driver` | `BOTHY_FILE__STORAGE__DRIVER` | Server | enum(fs, s3) | `fs` | `fs` oder `s3` | aktiv |
| `file.fs.root` | `BOTHY_FILE__FS__ROOT` | Server | string | `/var/lib/bothy/files` | Wurzel des Dateisystem-Treibers (Pfad im Container) | aktiv |
| `file.fs.min_free_bytes` | `BOTHY_FILE__FS__MIN_FREE_BYTES` | Server | int | `2147483648` | freier Platz, unter dem Uploads mit `503 unavailable` (`reason` `storage_full`) abgelehnt werden | aktiv |
| `file.s3.endpoint` | `BOTHY_FILE__S3__ENDPOINT` | Server | string | leer | S3-Endpunkt | aktiv |
| `file.s3.region` | `BOTHY_FILE__S3__REGION` | Server | string | `us-east-1` | Region für die Signatur | aktiv |
| `file.s3.bucket` | `BOTHY_FILE__S3__BUCKET` | Server | string | leer | Bucket | aktiv |
| `file.s3.prefix` | `BOTHY_FILE__S3__PREFIX` | Server | string | leer | Präfix vor allen Schlüsseln | aktiv |
| `file.s3.use_path_style` | `BOTHY_FILE__S3__USE_PATH_STYLE` | Server | bool | `true` | Pfad-Stil statt virtueller Hosts | aktiv |
| `file.s3.checksum_mode` | `BOTHY_FILE__S3__CHECKSUM_MODE` | Server | enum(when_required, when_supported) | `when_required` | SDK-Prüfsummen für Anfrage und Antwort: `when_required` oder `when_supported` (nur mit `https`-Endpunkt) | aktiv |
| `file.s3.access_key_ref` | `BOTHY_FILE__S3__ACCESS_KEY_REF` | Server | secret_ref | `file:<secrets.dir>/s3_access_key` | Verweise auf den Secret-Provider; nur mit `file.storage.driver = s3` gelesen | aktiv |
| `file.s3.secret_key_ref` | `BOTHY_FILE__S3__SECRET_KEY_REF` | Server | secret_ref | `file:<secrets.dir>/s3_secret_key` | Verweise auf den Secret-Provider; nur mit `file.storage.driver = s3` gelesen | aktiv |
| `file.s3.presigned_upload` | `BOTHY_FILE__S3__PRESIGNED_UPLOAD` | Server | bool | `false` | Modus `presigned` erlauben | aktiv |
| `file.s3.presign_ttl` | `BOTHY_FILE__S3__PRESIGN_TTL` | Server |  | `15m` | Gültigkeit einer presigned URL | geplant (M3) |
| `file.upload_part_bytes` | `BOTHY_FILE__UPLOAD_PART_BYTES` | Server | int | `8388608` | Teilgröße | aktiv |
| `file.upload_session_ttl` | `BOTHY_FILE__UPLOAD_SESSION_TTL` | Server | duration | `24h` | Lebensdauer einer offenen Session | aktiv |
| `file.upload_max_open_sessions` | `BOTHY_FILE__UPLOAD_MAX_OPEN_SESSIONS` | Server | int | `20` | offene Sessions je Prinzipal | aktiv |
| `file.quota_bytes_per_principal` | `BOTHY_FILE__QUOTA_BYTES_PER_PRINCIPAL` | Server | int | `0` | Kontingent je Prinzipal, 0 = aus | aktiv |
| `file.poster_max_bytes` | `BOTHY_FILE__POSTER_MAX_BYTES` | Server |  | `524288` | Größe des Video-Vorschaubilds vom Client | geplant (M3) |
| `file.sniff_bytes` | `BOTHY_FILE__SNIFF_BYTES` | Server | int | `1048576` | Bytes für die Typ-Erkennung | aktiv |
| `file.denied_mime_types` | `BOTHY_FILE__DENIED_MIME_TYPES` | Server | list(string) | leer | gesperrte Typen | aktiv |
| `file.name_max_bytes` | `BOTHY_FILE__NAME_MAX_BYTES` | Server | int | `255` | Länge eines Dateinamens in UTF-8-Bytes | aktiv |
| `file.scan.mode` | `BOTHY_FILE__SCAN__MODE` | Server | enum(required, off) | `required` | `required` oder `off` | aktiv |
| `file.scan.clamd_address` | `BOTHY_FILE__SCAN__CLAMD_ADDRESS` | Server | string | `unix:///run/clamav/clamd.sock` | Unix socket of clamd, `unix://` plus an absolute path; no other transport (19 SEC-TLS-5) | aktiv |
| `file.scan.chunk_bytes` | `BOTHY_FILE__SCAN__CHUNK_BYTES` | Server | int | `65536` | Stückgröße für `INSTREAM` | aktiv |
| `file.scan.timeout` | `BOTHY_FILE__SCAN__TIMEOUT` | Server | duration | `120s` | Zeitlimit je Scan | aktiv |
| `file.scan.max_concurrent` | `BOTHY_FILE__SCAN__MAX_CONCURRENT` | Server | int | `4` | Scans je Worker-Prozess | aktiv |
| `file.scan.retry_backoff_cap` | `BOTHY_FILE__SCAN__RETRY_BACKOFF_CAP` | Server | duration | `5m` | größter Abstand zwischen Scan-Versuchen | aktiv |
| `file.scan.health_interval` | `BOTHY_FILE__SCAN__HEALTH_INTERVAL` | Server | duration | `30s` | `PING`-Abstand | aktiv |
| `file.scan.pending_alert_after` | `BOTHY_FILE__SCAN__PENDING_ALERT_AFTER` | Server |  | `10m` | Alter der ältesten `pending`-Datei bis zum Alarm | geplant (M3) |
| `file.scan.clamd_stream_max_bytes` | `BOTHY_FILE__SCAN__CLAMD_STREAM_MAX_BYTES` | Server | int | `104857600` | Spiegel von clamd `StreamMaxLength` | aktiv |
| `file.invocation_hold_max` | `BOTHY_FILE__INVOCATION_HOLD_MAX` | Server |  | `120s` | längstes Warten des Agenten-Aufrufs auf Anhänge | geplant (M3) |
| `file.image.libvips_enabled` | `BOTHY_FILE__IMAGE__LIBVIPS_ENABLED` | Server |  | `false` | libvips-Build nutzen | geplant (M3) |
| `file.image.max_pixels` | `BOTHY_FILE__IMAGE__MAX_PIXELS` | Server | int | `50000000` | größte dekodierte Pixelzahl | aktiv |
| `file.image.jpeg_quality` | `BOTHY_FILE__IMAGE__JPEG_QUALITY` | Server | int | `90` | Qualität beim Umwandeln unbewegter WebP-Bilder ohne Alpha in JPEG | aktiv |
| `file.image.processing_timeout` | `BOTHY_FILE__IMAGE__PROCESSING_TIMEOUT` | Server | duration | `60s` | Zeitlimit eines Bild-Jobs | aktiv |
| `file.image.max_concurrent` | `BOTHY_FILE__IMAGE__MAX_CONCURRENT` | Server | int | `2` | Bild-Jobs je Worker-Prozess | aktiv |
| `file.thumbnail.small_px` | `BOTHY_FILE__THUMBNAIL__SMALL_PX` | Server | int | `320` | längste Kante `small` | aktiv |
| `file.thumbnail.medium_px` | `BOTHY_FILE__THUMBNAIL__MEDIUM_PX` | Server | int | `1024` | längste Kante `medium` | aktiv |
| `file.thumbnail.large_px` | `BOTHY_FILE__THUMBNAIL__LARGE_PX` | Server | int | `2048` | längste Kante `large` | aktiv |
| `file.thumbnail.jpeg_quality` | `BOTHY_FILE__THUMBNAIL__JPEG_QUALITY` | Server | int | `82` | Qualität der Vorschaubilder | aktiv |
| `file.ticket_ttl` | `BOTHY_FILE__TICKET_TTL` | Server | duration | `5m` | Gültigkeit eines Abruf-Tickets | aktiv |
| `file.ticket_key_ref` | `BOTHY_FILE__TICKET_KEY_REF` | Server | secret_ref | `file:<secrets.dir>/file_ticket_key` | Verweis auf den Ticket-Schlüssel im Secret-Provider | aktiv |
| `file.not_ready_retry_after` | `BOTHY_FILE__NOT_READY_RETRY_AFTER` | Server | duration | `5s` | `Retry-After` of the `409 file_not_ready` responses (in whole seconds) | aktiv |
| `file.unattached_ttl` | `BOTHY_FILE__UNATTACHED_TTL` | Server |  | `24h` | Frist für nie angehängte Dateien | geplant (M3) |
| `file.failed_row_ttl` | `BOTHY_FILE__FAILED_ROW_TTL` | Server |  | `7d` | Frist für Zeilen fehlgeschlagener oder blockierter Dateien | geplant (M3) |
| `file.gc_interval` | `BOTHY_FILE__GC_INTERVAL` | Server |  | `10m` | Abstand des Aufräum-Jobs | geplant (M3) |
| `voice.max_duration` | `BOTHY_VOICE__MAX_DURATION` | Server | duration | `15m` | längste Sprachnachricht | aktiv |
| `voice.transcription_mode` | `BOTHY_VOICE__TRANSCRIPTION_MODE` | Server |  | `agent` | `off`, `agent` oder `service` | geplant (M3) |
| `voice.transcription_url` | `BOTHY_VOICE__TRANSCRIPTION_URL` | Server |  | leer | Endpunkt des Transkriptions-Dienstes | geplant (M3) |
| `voice.transcription_api_key_ref` | `BOTHY_VOICE__TRANSCRIPTION_API_KEY_REF` | Server |  | leer | Verweis auf den Schlüssel | geplant (M3) |
| `voice.transcription_timeout` | `BOTHY_VOICE__TRANSCRIPTION_TIMEOUT` | Server |  | `120s` | Zeitlimit einer Transkription | geplant (M3) |
| `voice.transcript_hold_max` | `BOTHY_VOICE__TRANSCRIPT_HOLD_MAX` | Server |  | `60s` | längstes Warten des Agenten-Aufrufs auf das Transkript | geplant (M3) |

### 13 Push und Benachrichtigungen

Quelle: [13-push-benachrichtigungen.md](../spec/13-push-benachrichtigungen.md), Abschnitt 17.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `push.ttl` | `BOTHY_PUSH__TTL` | Server |  | `24h` | Lebensdauer beim Anbieter (`apns-expiration`, `android.ttl`) | geplant (M3) |
| `push.max_age` | `BOTHY_PUSH__MAX_AGE` | Server |  | `1h` | älter wird ein noch nicht gesendeter Hinweis verworfen | geplant (M3) |
| `push.retry_base` | `BOTHY_PUSH__RETRY_BASE` | Server |  | `2s` | Basis des Backoffs | geplant (M3) |
| `push.retry_cap` | `BOTHY_PUSH__RETRY_CAP` | Server |  | `5m` | größter Backoff-Abstand | geplant (M3) |
| `push.max_concurrent_requests` | `BOTHY_PUSH__MAX_CONCURRENT_REQUESTS` | Server |  | `32` | gleichzeitige Anfragen an Anbieter je Prozess | geplant (M3) |
| `push.apns.team_id` | `BOTHY_PUSH__APNS__TEAM_ID` | Server |  | leer | Apple-Team-ID | geplant (M3) |
| `push.apns.key_id` | `BOTHY_PUSH__APNS__KEY_ID` | Server |  | leer | Key-ID des `.p8`-Schlüssels | geplant (M3) |
| `push.apns.private_key_ref` | `BOTHY_PUSH__APNS__PRIVATE_KEY_REF` | Server |  | leer | Verweis auf den Schlüssel im Secret-Provider | geplant (M3) |
| `push.apns.topics` | `BOTHY_PUSH__APNS__TOPICS` | Server |  | leer | erlaubte Bundle-IDs | geplant (M3) |
| `push.apns.jwt_refresh_interval` | `BOTHY_PUSH__APNS__JWT_REFRESH_INTERVAL` | Server |  | `40m` | Erneuerung des Anbieter-JWT, muss zwischen 20 und 60 Minuten liegen | geplant (M3) |
| `push.apns.retry_5xx_delay` | `BOTHY_PUSH__APNS__RETRY_5XX_DELAY` | Server |  | `15m` | Wartezeit nach `500` oder `503` | geplant (M3) |
| `push.apns.time_sensitive_enabled` | `BOTHY_PUSH__APNS__TIME_SENSITIVE_ENABLED` | Server |  | `false` | `time-sensitive` für `input_needed` | geplant (M3) |
| `push.fcm.project_id` | `BOTHY_PUSH__FCM__PROJECT_ID` | Server |  | leer | Firebase-Projekt | geplant (M3) |
| `push.fcm.service_account_ref` | `BOTHY_PUSH__FCM__SERVICE_ACCOUNT_REF` | Server |  | leer | Verweis auf den Dienstkonto-Schlüssel | geplant (M3) |
| `notify.user_default_level` | `BOTHY_NOTIFY__USER_DEFAULT_LEVEL` | Server | enum(all, mentions, mute) | `mentions` | Stufe ohne jede Einstellung | aktiv |
| `notify.default_preview_mode` | `BOTHY_NOTIFY__DEFAULT_PREVIEW_MODE` | Server |  | `full` | Standard für `notify.preview_mode` | geplant (M3) |
| `notify.default_input_needed_breaks_quiet_hours` | `BOTHY_NOTIFY__DEFAULT_INPUT_NEEDED_BREAKS_QUIET_HOURS` | Server |  | `false` | Standard für den gleichnamigen Schalter | geplant (M3) |
| `notify.default_admin_enabled` | `BOTHY_NOTIFY__DEFAULT_ADMIN_ENABLED` | Server |  | `true` | Standard für `notify.admin_enabled` | geplant (M3) |
| `notify.coalesce_window` | `BOTHY_NOTIFY__COALESCE_WINDOW` | Server |  | `60s` | Bündelfenster je Empfänger und Thread | geplant (M3) |
| `notify.mobile_delay_when_desktop_active` | `BOTHY_NOTIFY__MOBILE_DELAY_WHEN_DESKTOP_ACTIVE` | Server |  | `60s` | Verzögerung mobiler Pushes bei aktivem Desktop | geplant (M3) |
| `notify.preview_max_chars` | `BOTHY_NOTIFY__PREVIEW_MAX_CHARS` | Server |  | `240` | Länge des Auszugs in der Vorschau | geplant (M3) |
| `notify.badge_sync_min_interval` | `BOTHY_NOTIFY__BADGE_SYNC_MIN_INTERVAL` | Server |  | `60s` | kleinster Abstand der Badge-Abgleiche je Person | geplant (M3) |

### 14 Suche

Quelle: [14-suche.md](../spec/14-suche.md), Abschnitt 15.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `search.languages` | `BOTHY_SEARCH__LANGUAGES` | Server | list(string) | `["german","english"]` | Konfigurationen für den Anfrage-Aufbau (plus immer `simple`) | aktiv |
| `search.locale_map` | `BOTHY_SEARCH__LOCALE_MAP` | Server | map(string) | `{"de":"german","en":"english"}` | Sprachkennung zu Textsuche-Konfiguration | aktiv |
| `search.query_max_chars` | `BOTHY_SEARCH__QUERY_MAX_CHARS` | Server |  | `256` | Länge der Eingabe | geplant (M3) |
| `search.max_candidates` | `BOTHY_SEARCH__MAX_CANDIDATES` | Server |  | `2000` | Kandidaten vor dem Ranking | geplant (M3) |
| `search.rank_normalization` | `BOTHY_SEARCH__RANK_NORMALIZATION` | Server |  | `1` | Normalisierung für `ts_rank` | geplant (M3) |
| `search.recency_scale` | `BOTHY_SEARCH__RECENCY_SCALE` | Server |  | `30d` | Alter, bei dem der Wert auf die Hälfte fällt | geplant (M3) |
| `search.page_size_default` | `BOTHY_SEARCH__PAGE_SIZE_DEFAULT` | Server |  | `20` | Treffer je Seite | geplant (M3) |
| `search.page_size_max` | `BOTHY_SEARCH__PAGE_SIZE_MAX` | Server |  | `50` | höchstens Treffer je Seite | geplant (M3) |
| `search.headline_input_max_chars` | `BOTHY_SEARCH__HEADLINE_INPUT_MAX_CHARS` | Server |  | `20000` | Länge des Textes für `ts_headline` | geplant (M3) |
| `search.headline_max_fragments` | `BOTHY_SEARCH__HEADLINE_MAX_FRAGMENTS` | Server |  | `2` | Ausschnitte je Treffer | geplant (M3) |
| `search.headline_max_words` | `BOTHY_SEARCH__HEADLINE_MAX_WORDS` | Server |  | `30` | Wörter je Ausschnitt, höchstens | geplant (M3) |
| `search.headline_min_words` | `BOTHY_SEARCH__HEADLINE_MIN_WORDS` | Server |  | `10` | Wörter je Ausschnitt, mindestens | geplant (M3) |
| `search.prefix_min_chars` | `BOTHY_SEARCH__PREFIX_MIN_CHARS` | Server |  | `2` | kürzeste Wortlänge in der Präfix-Suche | geplant (M3) |
| `search.statement_timeout` | `BOTHY_SEARCH__STATEMENT_TIMEOUT` | Server |  | `3s` | Zeitlimit einer Suchabfrage | geplant (M3) |
| `search.p95_target` | `BOTHY_SEARCH__P95_TARGET` | Server |  | `500ms` | Ziel p95 im Lasttest | geplant (M3) |
| `search.prefix_p95_target` | `BOTHY_SEARCH__PREFIX_P95_TARGET` | Server |  | `100ms` | Ziel p95 der Präfix-Suche | geplant (M3) |
| `search.benchmark_messages` | `BOTHY_SEARCH__BENCHMARK_MESSAGES` | Server |  | `2000000` | Korpusgröße im Lasttest | geplant (M3) |
| `search.extract.max_input_bytes` | `BOTHY_SEARCH__EXTRACT__MAX_INPUT_BYTES` | Server |  | `52428800` | größte Datei für die Extraktion | geplant (M3) |
| `search.extract.max_text_bytes` | `BOTHY_SEARCH__EXTRACT__MAX_TEXT_BYTES` | Server |  | `1048576` | größter gespeicherter Text | geplant (M3) |
| `search.extract.max_uncompressed_bytes` | `BOTHY_SEARCH__EXTRACT__MAX_UNCOMPRESSED_BYTES` | Server |  | `209715200` | größte entpackte Menge bei Archiven | geplant (M3) |
| `search.extract.max_archive_entries` | `BOTHY_SEARCH__EXTRACT__MAX_ARCHIVE_ENTRIES` | Server |  | `10000` | Einträge je Archiv | geplant (M3) |
| `search.extract.timeout` | `BOTHY_SEARCH__EXTRACT__TIMEOUT` | Server |  | `60s` | Zeitlimit je Extraktion | geplant (M3) |
| `search.extract.tika_url` | `BOTHY_SEARCH__EXTRACT__TIKA_URL` | Server |  | leer | Tika-Server; leer = aus | geplant (V3) |

### 15 Client-Architektur

Quelle: [15-client-architektur.md](../spec/15-client-architektur.md), Abschnitt 22.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `client.cache_max_bytes` |  | App |  | `1GiB` | Obergrenze für lokale Datenbank plus Datei-Cache; darüber werden die ältesten Einträge entfernt |  |
| `client.sync.parse_offload_bytes` |  | App |  | `262144` | ab dieser Seitengröße wird im Hintergrund-Isolate dekodiert |  |
| `client.outbox_stuck_after` |  | App |  | `2m` | ab dann zeigt ein wartender Eintrag "Senden verzögert" |  |
| `client.push.fetch_timeout` |  | App |  | `20s` | Zeit, die NSE bzw. Android-Dienst für das Holen des Texts nutzen |  |
| `client.voice.bitrate` |  | App |  | `64kbps` | Bitrate der AAC-Aufnahme |  |
| `client.metrics.flush_interval` |  | App |  | `5m` | höchstens so oft sendet der Client Kennzahlen |  |
| `client.perf.cold_start_p95` |  | App |  | `2500ms` | Ziel Kaltstart |  |
| `client.perf.thread_open_p95` |  | App |  | `300ms` | Ziel Thread öffnen aus dem Cache |  |
| `client.perf.send_local_echo_p95` |  | App |  | `50ms` | Ziel lokale Anzeige beim Senden |  |
| `client.perf.patch_to_frame_p95` |  | App |  | `100ms` | Ziel Live-Box-Patch bis Frame |  |
| `client.perf.jank_ratio_max` |  | App |  | `0.01` | höchster Anteil Frames über Budget |  |
| `client.perf.catchup_events_per_s_min` |  | App |  | `2000` | Mindestrate beim Nachholen |  |
| `client.perf.memory_mobile_max` |  | App |  | `300MiB` | Speicher mobil |  |
| `client.perf.memory_desktop_max` |  | App |  | `500MiB` | Speicher Desktop |  |
| `client.perf.terminal_frame_to_paint_p95` |  | App |  | `16ms` | Ziel Terminal-Rahmen bis Frame |  |
| `client.perf.terminal_throughput_min` |  | App |  | `1MiB/s` | Mindestrate der Terminal-Ansicht bei Dauerausgabe |  |
| `client.terminal.scrollback_lines` |  | App |  | `10000` | Zeilen im Rückblick des Renderers im Arbeitsspeicher; der Puffer für das Wiederanhängen liegt im Host-Dienst (`host_service.scrollback_bytes`) |  |
| `client.terminal.font_size_default` |  | App |  | `14` | Schriftgröße der Terminal-Ansicht in logischen Pixeln, bis die Person sie ändert |  |

### 16 Betrieb

Quelle: [16-betrieb.md](../spec/16-betrieb.md), Abschnitt 14.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `instance.environment` | `BOTHY_INSTANCE__ENVIRONMENT` | Server | enum(dev, staging, prod) | `prod` | `dev`, `staging` oder `prod` (6.1) | aktiv |
| `server.role` | `BOTHY_SERVER__ROLE` | Server | enum(api, worker, all) | `all` | `api`, `worker` oder `all` | aktiv |
| `server.listen_addr` | `BOTHY_SERVER__LISTEN_ADDR` | Server | listen address | `127.0.0.1:8080` | öffentlicher Listener (REST, Gateway) | aktiv |
| `server.internal_listen_addr` | `BOTHY_SERVER__INTERNAL_LISTEN_ADDR` | Server | listen address | `127.0.0.1:8081` | Health, Readiness, Eingangs-Webhooks aus dem Stack | aktiv |
| `server.public_url` | `BOTHY_SERVER__PUBLIC_URL` | Server | url | keiner, Pflicht | Origin, unter dem Apps den Server erreichen (Schema, Host, optional Port; ohne Pfad, Query und Fragment, sonst scheitert `bothy config check`); Grundlage von `aud`, DPoP-`htu` und WebAuthn ([08-identitaet.md](../spec/08-identitaet.md), Abschnitte 7.2 und 7.3) | aktiv |
| `server.trusted_proxies` | `BOTHY_SERVER__TRUSTED_PROXIES` | Server | list(cidr) | `127.0.0.1/32, ::1/128` | Gegenstellen, deren Weiterleitungs- und Identitäts-Kopfzeilen gelten und die Klartext senden dürfen ([19-sicherheit.md](../spec/19-sicherheit.md), SEC-TLS-1) | aktiv |
| `server.maintenance_mode` | `BOTHY_SERVER__MAINTENANCE_MODE` | Server |  | `off` | `off` oder `read_only` (Umzug, Upgrade; [ADR 0065](../adr/0065-wartungsmodus-read-only.md)) | geplant (M7) |
| `secrets.dir` | `BOTHY_SECRETS__DIR` | Server | string | `/run/bothy/secrets` | Basis der Standardverweise `file:`; absoluter Pfad in sauberer Form (kein `.` oder `..`, kein doppelter oder abschließender `/`), nicht `/`, ohne NUL-Zeichen und ohne `<`, sonst scheitert `bothy config check` | aktiv |
| `secrets.cache_ttl` | `BOTHY_SECRETS__CACHE_TTL` | Server | duration | `5m` | so lange gilt ein gelesenes Geheimnis; mindestens `1s` | aktiv |
| `secrets.signing_key_dir` | `BOTHY_SECRETS__SIGNING_KEY_DIR` | Server | string | `/var/lib/bothy/signing-keys` | beschreibbares Verzeichnis der privaten Signierschlüssel; im Besitz des Dienstbenutzers, Modus `0700` (5.2, [ADR 0066](../adr/0066-signierschluessel-im-beschreibbaren-schluesselspeicher.md)) | aktiv |
| `console.assets_dir` | `BOTHY_CONSOLE__ASSETS_DIR` | Server | string | leer | directory of the built web console (`flutter build web` of `apps/console`) that the api process serves under `/console/` ([02-architektur.md](../spec/02-architektur.md), 3.1); empty: no console, `404` under `/console/`; the image sets `/usr/share/bothy/console`; a set directory without `index.html` ends the start | aktiv |
| `console.csp_report_max_records` | `BOTHY_CONSOLE__CSP_REPORT_MAX_RECORDS` | Server | int | `20` | most log records one request of `submitCspReport` writes (1 to 1000): identical violations share one record, every violation is counted in `bothy.console.csp_violations`; the rate class `rl.public_auth` bounds the requests ([19-sicherheit.md](../spec/19-sicherheit.md) 5 rule 6, M6-03) | aktiv |
| `server.tls_cert_ref` | `BOTHY_SERVER__TLS_CERT_REF` | Server | secret_ref | leer | Verweis auf die Zertifikatskette (PEM, Blatt zuerst); leer: der Server beendet kein TLS (SEC-TLS-2) | aktiv |
| `server.tls_key_ref` | `BOTHY_SERVER__TLS_KEY_REF` | Server | secret_ref | leer | Verweis auf den privaten Schlüssel (PEM); nur zusammen mit `server.tls_cert_ref` | aktiv |
| `database.host` | `BOTHY_DATABASE__HOST` | Server | string | `localhost` | Host der Datenbank | aktiv |
| `database.port` | `BOTHY_DATABASE__PORT` | Server | int | `5432` | Port der Datenbank im Netz des Servers | aktiv |
| `database.name` | `BOTHY_DATABASE__NAME` | Server | string | `bothy` | Datenbankname | aktiv |
| `database.user` | `BOTHY_DATABASE__USER` | Server | string | `bothy_app` | Rolle des Servers | aktiv |
| `database.password_ref` | `BOTHY_DATABASE__PASSWORD_REF` | Server | secret_ref | `file:<secrets.dir>/db_app_password` | Verweis auf das Passwort | aktiv |
| `database.sslmode` | `BOTHY_DATABASE__SSLMODE` | Server | enum(disable, verify-full) | `verify-full` | TLS zur Datenbank; im internen Compose-Netz `disable` (SEC-TLS-5) | aktiv |
| `database.sslrootcert` | `BOTHY_DATABASE__SSLROOTCERT` | Server | string | leer | CA-Bündel für `verify-full`; leer nutzt die Wurzeln des Systems | aktiv |
| `database.max_conns` | `BOTHY_DATABASE__MAX_CONNS` | Server | int | `20` | Größe des Verbindungspools je Prozess (Startwert bis zum Lasttest) | aktiv |
| `database.restore_guard` | `BOTHY_DATABASE__RESTORE_GUARD` | Server | enum(on, off) | `on` | `on` or `off`: with `on` a server whose database identity differs from the stored one stays not ready (`restore_not_finalized`) until `bothy restore finalize`; `off` only where a failover loses no committed transaction (8.4, R-DM-50) | aktiv |
| `migrate.require_recent_backup` | `BOTHY_MIGRATE__REQUIRE_RECENT_BACKUP` | Server | bool | `true` | `migrate` verlangt eine frische Sicherung | aktiv |
| `migrate.recent_backup_max_age` | `BOTHY_MIGRATE__RECENT_BACKUP_MAX_AGE` | Server | duration | `24h` | so alt darf diese Sicherung sein | aktiv |
| `migrate.lock_timeout` | `BOTHY_MIGRATE__LOCK_TIMEOUT` | Server | duration | `5s` | `lock_timeout` je Migrationsschritt | aktiv |
| `migrate.bootstrap_user` | `BOTHY_MIGRATE__BOOTSTRAP_USER` | Server | string | `postgres` | Superuser-Anmeldung nur für `migrate up --bootstrap` | aktiv |
| `migrate.bootstrap_password_ref` | `BOTHY_MIGRATE__BOOTSTRAP_PASSWORD_REF` | Server | secret_ref | `file:<secrets.dir>/db_superuser_password` | dessen Passwort | aktiv |
| `migrate.owner_password_ref` | `BOTHY_MIGRATE__OWNER_PASSWORD_REF` | Server | secret_ref | `file:<secrets.dir>/db_owner_password` | Passwort, das `--bootstrap` für `bothy_owner` setzt | aktiv |
| `migrate.app_password_ref` | `BOTHY_MIGRATE__APP_PASSWORD_REF` | Server | secret_ref | `file:<secrets.dir>/db_app_password` | Passwort für `bothy_app` | aktiv |
| `migrate.maint_password_ref` | `BOTHY_MIGRATE__MAINT_PASSWORD_REF` | Server | secret_ref | `file:<secrets.dir>/db_maint_password` | Passwort für `bothy_maint` | aktiv |
| `migrate.monitor_password_ref` | `BOTHY_MIGRATE__MONITOR_PASSWORD_REF` | Server | secret_ref | `file:<secrets.dir>/db_monitor_password` | Passwort für `bothy_monitor` | aktiv |
| `migrate.backup_password_ref` | `BOTHY_MIGRATE__BACKUP_PASSWORD_REF` | Server | secret_ref | `file:<secrets.dir>/db_backup_password` | password for `bothy_backup` (M7-02) | aktiv |
| `backup.pg.full_schedule` | `BOTHY_BACKUP__PG__FULL_SCHEDULE` | Server | string | `0 2 * * 0` | Plan der Vollsicherung (cron) | aktiv |
| `backup.pg.diff_schedule` | `BOTHY_BACKUP__PG__DIFF_SCHEDULE` | Server | string | `0 2 * * 1-6` | Plan der Differenzsicherung (cron) | aktiv |
| `backup.pg.archive_timeout` | `BOTHY_BACKUP__PG__ARCHIVE_TIMEOUT` | Server |  | `60s` | spätestens dann wird ein WAL-Segment archiviert | geplant (M7) |
| `backup.pg.retention_full` | `BOTHY_BACKUP__PG__RETENTION_FULL` | Server | int | `4` | behaltene Vollsicherungen in repo2 | aktiv |
| `backup.pg.local_retention_full` | `BOTHY_BACKUP__PG__LOCAL_RETENTION_FULL` | Server | int | `1` | behaltene Vollsicherungen in repo1 | aktiv |
| `backup.logical.schedule` | `BOTHY_BACKUP__LOGICAL__SCHEDULE` | Server | string | `0 3 * * *` | Plan von `pg_dump` (cron) | aktiv |
| `backup.logical.retention` | `BOTHY_BACKUP__LOGICAL__RETENTION` | Server | duration | `30d` | Aufbewahrung der logischen Sicherung | aktiv |
| `backup.objects.schedule` | `BOTHY_BACKUP__OBJECTS__SCHEDULE` | Server | string | `30 3 * * *` | Plan der Objektsicherung (cron) | aktiv |
| `backup.objects.retention` | `BOTHY_BACKUP__OBJECTS__RETENTION` | Server | duration | `90d` | Aufbewahrung der Objekt-Snapshots | aktiv |
| `backup.offsite.target` | `BOTHY_BACKUP__OFFSITE__TARGET` | Server | string | leer | Off-site-Ziel; leer heißt nur lokal (Warnung in Konsole und Status) | aktiv |
| `backup.offsite.region` | `BOTHY_BACKUP__OFFSITE__REGION` | Server | string | `us-east-1` | region of the off-site bucket (signing of S3 requests) | aktiv |
| `backup.offsite.use_path_style` | `BOTHY_BACKUP__OFFSITE__USE_PATH_STYLE` | Server | bool | `true` | path-style addressing of the off-site bucket (`https://<host>/<bucket>`); `false` uses virtual hosts | aktiv |
| `backup.max_age` | `BOTHY_BACKUP__MAX_AGE` | Server |  | `26h` | ohne erfolgreiche Sicherung länger als das: Alarm | geplant (M7) |
| `backup.restore_test.schedule` | `BOTHY_BACKUP__RESTORE_TEST__SCHEDULE` | Server | string | `0 5 * * 6` | Plan des Wiederherstellungstests (cron) | aktiv |
| `backup.restore_test.sample_files` | `BOTHY_BACKUP__RESTORE_TEST__SAMPLE_FILES` | Server | int | `200` | geprüfte Dateien je Test | aktiv |
| `backup.restore_test.sample_threads` | `BOTHY_BACKUP__RESTORE_TEST__SAMPLE_THREADS` | Server | int | `200` | geprüfte Threads je Test | aktiv |
| `backup.restore_test.max_age` | `BOTHY_BACKUP__RESTORE_TEST__MAX_AGE` | Server |  | `8d` | ohne erfolgreichen Test länger als das: Alarm | geplant (M7) |
| `dr.rpo_target` | `BOTHY_DR__RPO_TARGET` | Server |  | `5m` | Ziel für den größten Datenverlust | geplant (M7) |
| `dr.rto_target` | `BOTHY_DR__RTO_TARGET` | Server | duration | `4h` | Ziel für die Dauer der Wiederherstellung | aktiv |
| `status.check_interval` | `BOTHY_STATUS__CHECK_INTERVAL` | Server |  | `30s` | Abstand der automatischen Prüfungen | geplant (M7) |
| `status.degrade_after` | `BOTHY_STATUS__DEGRADE_AFTER` | Server |  | `2m` | so lange Fehler bis `degraded` | geplant (M7) |
| `status.incident_after` | `BOTHY_STATUS__INCIDENT_AFTER` | Server |  | `5m` | so lange `major_outage` bis zur automatischen Störung | geplant (M7) |
| `status.resolve_after` | `BOTHY_STATUS__RESOLVE_AFTER` | Server |  | `10m` | so lange gesund bis zur automatischen Auflösung | geplant (M7) |
| `status.error_ratio_degraded` | `BOTHY_STATUS__ERROR_RATIO_DEGRADED` | Server |  | `0.05` | Fehleranteil, ab dem eine Prüfung scheitert | geplant (M7) |
| `status.error_ratio_outage` | `BOTHY_STATUS__ERROR_RATIO_OUTAGE` | Server |  | `0.5` | Fehleranteil, ab dem `major_outage` gilt | geplant (M7) |
| `status.fanout_lag_degraded` | `BOTHY_STATUS__FANOUT_LAG_DEGRADED` | Server |  | `5s` | Fan-out-Verzug, ab dem `sync.fanout` scheitert | geplant (M7) |
| `install.ready_timeout` |  | Installationsskript |  | `5m` | how long `install.sh` waits for `/readyz` after `docker compose up -d` (option `--ready-timeout`, 3.1); not a key of the server |  |

### 17 Observability

Quelle: [17-observability.md](../spec/17-observability.md), Abschnitt 11.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `telemetry.enabled` | `BOTHY_TELEMETRY__ENABLED` | Server | bool | `true` | Telemetrie an oder aus | aktiv |
| `telemetry.traces.sampler_ratio` | `BOTHY_TELEMETRY__TRACES__SAMPLER_RATIO` | Server | float | `1.0` | Anteil der Traces, die das SDK aufzeichnet; der Collector sampelt danach nach | aktiv |
| `telemetry.traces.trust_remote_parent` | `BOTHY_TELEMETRY__TRACES__TRUST_REMOTE_PARENT` | Server | bool | `false` | `true`: ein eingehender `traceparent` wird Eltern-Kontext (seine Trace-ID wird `correlation_id`, sein Sampling-Flag gilt); `false`: nur Span-Link, eigener Trace | aktiv |
| `telemetry.logs.exporter` | `BOTHY_TELEMETRY__LOGS__EXPORTER` | Server | enum(otlp, stdout, both) | `both` | `otlp`, `stdout` oder `both` | aktiv |
| `telemetry.logs.level` | `BOTHY_TELEMETRY__LOGS__LEVEL` | Server | enum(debug, info, warn, error) | `info` | kleinste Stufe, die geschrieben wird | aktiv |
| `telemetry.pseudonym_key_ref` | `BOTHY_TELEMETRY__PSEUDONYM_KEY_REF` | Server | secret_ref | `file:<secrets.dir>/telemetry_pseudonym_key` | Schlüssel für HMAC-Pseudonyme; mindestens 32 Byte | aktiv |
| `telemetry.dashboard_url` | `BOTHY_TELEMETRY__DASHBOARD_URL` | Server |  | leer | Link aus der Admin-Konsole zum Telemetrie-Backend | geplant (M6) |
| `stats.cache_ttl` | `BOTHY_STATS__CACHE_TTL` | Server |  | `5m` | so lange gilt ein berechnetes Statistik-Ergebnis | geplant (M6) |
| `stats.max_range` | `BOTHY_STATS__MAX_RANGE` | Server |  | `400d` | größter abfragbarer Zeitraum | geplant (M6) |

### 18 Release

Quelle: [18-release.md](../spec/18-release.md), Abschnitt 12.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `client.min_version.ios` | `BOTHY_CLIENT__MIN_VERSION__IOS` | Server | string | leer | kleinste zugelassene App-Version auf iOS; leer heißt keine Grenze | aktiv |
| `client.min_version.android` | `BOTHY_CLIENT__MIN_VERSION__ANDROID` | Server | string | leer | dasselbe für Android | aktiv |
| `client.min_version.macos` | `BOTHY_CLIENT__MIN_VERSION__MACOS` | Server | string | leer | dasselbe für macOS | aktiv |
| `client.min_version.windows` | `BOTHY_CLIENT__MIN_VERSION__WINDOWS` | Server | string | leer | dasselbe für Windows | aktiv |
| `client.recommended_version.ios` | `BOTHY_CLIENT__RECOMMENDED_VERSION__IOS` | Server | string | leer | darunter zeigt die App einen Update-Hinweis | aktiv |
| `client.recommended_version.android` | `BOTHY_CLIENT__RECOMMENDED_VERSION__ANDROID` | Server | string | leer | dasselbe für Android | aktiv |
| `client.recommended_version.macos` | `BOTHY_CLIENT__RECOMMENDED_VERSION__MACOS` | Server | string | leer | dasselbe für macOS | aktiv |
| `client.recommended_version.windows` | `BOTHY_CLIENT__RECOMMENDED_VERSION__WINDOWS` | Server | string | leer | dasselbe für Windows | aktiv |

### 19 Sicherheit

Quelle: [19-sicherheit.md](../spec/19-sicherheit.md), Abschnitt 16.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `http.hsts_max_age` | `BOTHY_HTTP__HSTS_MAX_AGE` | Server | duration | `31536000s` | `max-age` für HSTS | aktiv |
| `egress.allowed_hosts` | `BOTHY_EGRESS__ALLOWED_HOSTS` | Server | list(string) | leer | zusätzliche erlaubte Ziele, Form `host[:port][;allow_private]` ([configuration.md](../operations/configuration.md), Abschnitt 2); doppelte Einträge sind ein Ladefehler | aktiv |
| `egress.timeout` | `BOTHY_EGRESS__TIMEOUT` | Server | duration | `15s` | Zeitgrenze einer ausgehenden Anfrage | aktiv |
| `egress.max_response_bytes` | `BOTHY_EGRESS__MAX_RESPONSE_BYTES` | Server | int | `1048576` | größte Antwort einer ausgehenden Anfrage | aktiv |
| `audit.checkpoint_interval` | `BOTHY_AUDIT__CHECKPOINT_INTERVAL` | Server | duration | `1h` | Abstand der signierten Prüfpunkte des Audit-Logs | aktiv |
| `audit.checkpoint_sink` | `BOTHY_AUDIT__CHECKPOINT_SINK` | Server | string | leer | Ablageziel der Prüfpunkte außerhalb der Datenbank: leer (Warnung, SEC-AUD-2), `file://<absoluter Pfad>` auf einem Speicher außerhalb des Datenbank-Volumes, or `s3://<bucket>[/<prefix>]` (M7-22), an S3-compatible object store with a retention lock, the recommended target for production: a bucket in the S3 bucket naming rules, an optional prefix of segments of `[a-z0-9-]` joined by single `/`. `bothy config check` refuses every other form; for `s3://` it and the server start read the Object Lock configuration of the bucket and fail unless Object Lock is enabled with a default retention in mode `COMPLIANCE` of at least `retention.audit` (`GOVERNANCE` fails) | aktiv |
| `audit.checkpoint_s3_endpoint` | `BOTHY_AUDIT__CHECKPOINT_S3_ENDPOINT` | Server | string | leer | endpoint of the object store of an `s3://` sink, an absolute `http` or `https` URL without path; its own endpoint, separate from `file.s3.endpoint` and the backup; `http` only with an `egress.allowed_hosts` entry with `allow_private` | aktiv |
| `audit.checkpoint_s3_region` | `BOTHY_AUDIT__CHECKPOINT_S3_REGION` | Server | string | `us-east-1` | region of the S3 signature of an `s3://` sink | aktiv |
| `audit.checkpoint_s3_use_path_style` | `BOTHY_AUDIT__CHECKPOINT_S3_USE_PATH_STYLE` | Server | bool | `true` | path-style requests of an `s3://` sink instead of `<bucket>.<endpoint host>` | aktiv |
| `audit.checkpoint_s3_access_key_ref` | `BOTHY_AUDIT__CHECKPOINT_S3_ACCESS_KEY_REF` | Server | secret_ref | `file:<secrets.dir>/audit_checkpoint_s3_access_key` | secret reference of the own access key ID of an `s3://` sink, read only with an `s3://` sink; supplied by the owner from a separate account ([backup-restore.md](../operations/backup-restore.md) 8) | aktiv |
| `audit.checkpoint_s3_secret_key_ref` | `BOTHY_AUDIT__CHECKPOINT_S3_SECRET_KEY_REF` | Server | secret_ref | `file:<secrets.dir>/audit_checkpoint_s3_secret_key` | secret reference of the own secret access key of an `s3://` sink, read only with an `s3://` sink | aktiv |
| `audit.reason_max_chars` | `BOTHY_AUDIT__REASON_MAX_CHARS` | Server | int | `500` | größte Länge eines Pflichtgrunds einer Verwaltungsaktion in Unicode-Zeichen (SEC-AUD-5) | aktiv |
| `signing_key.cache_ttl` | `BOTHY_SIGNING_KEY__CACHE_TTL` | Server | duration | `30s` | längste Zeit, die ein Prozess einen zurückgezogenen Schlüssel noch annimmt oder einen neuen nicht kennt | aktiv |
| `signing_key.rotation_check_interval` | `BOTHY_SIGNING_KEY__ROTATION_CHECK_INTERVAL` | Server | duration | `1m` | Abstand des Jobs `instance.key_rotation` | aktiv |
| `signing_key.access_token_rotation_interval` | `BOTHY_SIGNING_KEY__ACCESS_TOKEN_ROTATION_INTERVAL` | Server | duration | `30d` | planmäßiger Wechsel des Schlüssels für Zugriffstokens | aktiv |

### 20 Datenschutz

Quelle: [20-datenschutz.md](../spec/20-datenschutz.md), Abschnitt 13.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `retention.content` | `BOTHY_RETENTION__CONTENT` | Server |  | `none` | Frist für Inhalte; `none` heißt keine automatische Löschung (Konsole verlangt Bestätigung) | geplant (M7) |
| `retention.security_after_end` | `BOTHY_RETENTION__SECURITY_AFTER_END` | Server |  | `90d` | Sicherheitsdaten nach Ende, Ablauf oder Widerruf | geplant (M7) |
| `retention.audit` | `BOTHY_RETENTION__AUDIT` | Server | duration | `365d` | Audit-Log (Studie C schlug zwölf Monate vor) | aktiv |
| `retention.cost` | `BOTHY_RETENTION__COST` | Server |  | `395d` | Kosten und Preise | geplant (M7) |
| `retention.import_after_completion` | `BOTHY_RETENTION__IMPORT_AFTER_COMPLETION` | Server |  | `90d` | Import-Zustand nach Abschluss | geplant (M7) |
| `retention.privacy_job` | `BOTHY_RETENTION__PRIVACY_JOB` | Server |  | `1095d` | DSGVO-Aufträge (ohne Exportpakete) | geplant (M7) |
| `retention.ops` | `BOTHY_RETENTION__OPS` | Server |  | `395d` | Statusseite | geplant (M7) |
| `retention.stats_minutes` | `BOTHY_RETENTION__STATS_MINUTES` | Server |  | `90d` | Minuten-Aggregate der eigenen Messung (`stats_minutes`); mindestens der längste SLO-Zeitraum | geplant (M6) |
| `retention.webhook_deliveries` | `BOTHY_RETENTION__WEBHOOK_DELIVERIES` | Server |  | `30d` | Zustellversuche ausgehender Webhooks | geplant (M7) |
| `privacy.export_ttl` | `BOTHY_PRIVACY__EXPORT_TTL` | Server |  | `7d` | Bereitstellung eines Exportpakets | geplant (M7) |
| `privacy.export_requires_approval` | `BOTHY_PRIVACY__EXPORT_REQUIRES_APPROVAL` | Server |  | `false` | Export der eigenen Daten braucht Freigabe des Owners | geplant (M7) |
| `privacy.account_deletion_content` | `BOTHY_PRIVACY__ACCOUNT_DELETION_CONTENT` | Server |  | `keep_pseudonymized` | Inhalte gelöschter Konten: `keep_pseudonymized` oder `delete` | geplant (M7) |
| `privacy.object_delete_delay` | `BOTHY_PRIVACY__OBJECT_DELETE_DELAY` | Server |  | `24h` | höchste Zeit bis zum Entfernen gelöschter Objekte | geplant (M3) |
| `privacy.deletion_journal_retention` | `BOTHY_PRIVACY__DELETION_JOURNAL_RETENTION` | Server |  | `400d` | Aufbewahrung des Lösch-Journals; mindestens Sicherungsfrist | geplant (M7) |
| `privacy.audit_ip_key_period` | `BOTHY_PRIVACY__AUDIT_IP_KEY_PERIOD` | Server | duration | `30d` | Zeitraum je Schlüssel für IP-Pseudonyme | aktiv |
| `privacy.audit_ip_key_retention` | `BOTHY_PRIVACY__AUDIT_IP_KEY_RETENTION` | Server | duration | `90d` | danach werden Schlüssel vernichtet | aktiv |
| `privacy.audit_ip_kek_ref` | `BOTHY_PRIVACY__AUDIT_IP_KEK_REF` | Server | secret_ref | `file:<secrets.dir>/audit_ip_kek` | Verweis auf den Schlüssel, mit dem die IP-Schlüssel in `audit_ip_keys` verschlüsselt sind | aktiv |
| `privacy.telemetry_pseudonym_rotation` | `BOTHY_PRIVACY__TELEMETRY_PSEUDONYM_ROTATION` | Server | duration | `30d` | Wechsel des Schlüssels für pseudonyme IDs in Telemetrie | aktiv |
| `privacy.per_person_stats_enabled` | `BOTHY_PRIVACY__PER_PERSON_STATS_ENABLED` | Server |  | `false` | Statistik je Person | geplant (M6) |
| `privacy.stats_min_group_size` | `BOTHY_PRIVACY__STATS_MIN_GROUP_SIZE` | Server |  | `5` | kleinste Gruppe für Auswertungen nach Personen | geplant (M6) |
| `privacy.ai_notice_enabled` | `BOTHY_PRIVACY__AI_NOTICE_ENABLED` | Server |  | `true` | einmaliger KI-Hinweis je Kanal mit Agent | geplant (M7) |
| `privacy.notice_url` | `BOTHY_PRIVACY__NOTICE_URL` | Server |  | leer | Link zur Datenschutzinformation des Betreibers | geplant (M7) |

### 21 Mehrbenutzer-Dev-Umgebung und Abschottung

Quelle: [21-dev-isolation.md](../spec/21-dev-isolation.md), Abschnitt 12.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `dev.reverify_interval` | `BOTHY_DEV__REVERIFY_INTERVAL` | Server |  | `24h` | spätestens dann wird eine Umgebung neu geprüft | geplant (V2) |
| `dev.limits` | `BOTHY_DEV__LIMITS` | Server |  | `{"cpus": 4, "memory_mib": 8192, "disk_gib": 50, "pids": 2048}` | Grenzen je Umgebung | geplant (V2) |
| `dev.egress_allowlist` | `BOTHY_DEV__EGRESS_ALLOWLIST` | Server |  | leer | erlaubte Ziele (Hosts) für Umgebungen; ohne Eintrag kein Egress | geplant (V2) |

### 22 Import und Migration aus Discord

Quelle: [22-import-migration.md](../spec/22-import-migration.md), Abschnitt 19.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `import.discord.api_base` |  | Werkzeug `bothy import` |  | `https://discord.com/api/v10` | Discord-API-Basis [R6, belegt: v10 aktuell] |  |
| `import.discord.cdn_hosts` |  | Werkzeug `bothy import` |  | `cdn.discordapp.com`, `media.discordapp.net` | erlaubte Hosts für Anhänge |  |
| `import.discord.page_size` |  | Werkzeug `bothy import` |  | `100` | Nachrichten je Seite; Discord erlaubt höchstens 100 [R4, belegt] |  |
| `import.discord.max_429_retries` |  | Werkzeug `bothy import` |  | `5` | Wiederholungen nach `429` je Aufruf |  |
| `import.discord.max_rate_wait` |  | Werkzeug `bothy import` |  | `60s` | größte Wartezeit, bevor der Lauf abbricht |  |
| `import.discord.preflight_sample` |  | Werkzeug `bothy import` |  | `20` | Nachrichten für die Intent-Probe |  |
| `import.discord.closed_tags` |  | Werkzeug `bothy import` |  | leer | Forum-Tags, die einen Thread zusätzlich als erledigt markieren |  |
| `import.attachment_max_bytes` |  | Werkzeug `bothy import` |  | wie `file.max_upload_bytes` | größter übernommener Anhang |  |
| `import.http_timeout` |  | Werkzeug `bothy import` |  | `60s` | Zeitlimit je Aufruf an Discord und CDN; also the time limit of each call of the tool to Bothy |  |
| `import.login_poll_interval` |  | Werkzeug `bothy import` |  | `5s` | interval between polls of `completePairing` during `bothy import login`; the server limits public auth calls by `ratelimit.rl.public_auth.*` |  |
| `import.batch_max_items` |  | Werkzeug `bothy import` |  | `100` | Objekte je Stapel an den Server |  |
| `import.reply_quote_max_chars` |  | Werkzeug `bothy import` |  | `100` | Länge der Zitatzeile bei Antworten über Threads hinweg |  |
| `import.follow.poll_interval` |  | Werkzeug `bothy import` |  | `30s` | Abstand der Spiegel-Runden |  |
| `import.follow.settle_after` |  | Werkzeug `bothy import` |  | `60s` | Ruhezeit nach letzter Änderung |  |
| `import.follow.busy_reactions` |  | Werkzeug `bothy import` |  | `👀` | Reaktionen, die "Agent arbeitet noch" bedeuten |  |
| `import.follow.busy_lookback` |  | Werkzeug `bothy import` |  | `10` | Nachrichten vor der ersten neuen, die auf Markierung geprüft werden |  |
| `import.follow.max_backoff` |  | Werkzeug `bothy import` |  | `5m` | größter Abstand nach fehlgeschlagenen Runden |  |

### 24 Terminal

Quelle: [24-terminal.md](../spec/24-terminal.md), Abschnitt 17.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `terminal.pairing_code_ttl` | `BOTHY_TERMINAL__PAIRING_CODE_TTL` | Server | duration | `15m` | Gültigkeit eines Kopplungs-Codes für einen Rechner | aktiv |
| `terminal.fingerprint_confirm_chars` | `BOTHY_TERMINAL__FINGERPRINT_CONFIRM_CHARS` | Server | int | `8` | so viele Zeichen des Fingerabdrucks tippt der Owner bei der Bestätigung | aktiv |
| `terminal.fingerprint_max_attempts` | `BOTHY_TERMINAL__FINGERPRINT_MAX_ATTEMPTS` | Server | int | `3` | Fehlversuche bei der Bestätigung, danach Rückfall auf `pairing` | aktiv |
| `terminal.report_label_max_chars` | `BOTHY_TERMINAL__REPORT_LABEL_MAX_CHARS` | Server | int | `100` | längster Profilname, Anzeigename, Name eines Arbeitsordners und Betriebssystem-Version, die ein Rechner meldet (3.2) | aktiv |
| `terminal.report_max_entries` | `BOTHY_TERMINAL__REPORT_MAX_ENTRIES` | Server | int | `64` | höchstens so viele Einträge je gemeldeter Liste: Profile, Sitzungsbenutzer, Arbeitsordner eines Sitzungsbenutzers (3.2) | aktiv |
| `terminal.step_up_max_age` | `BOTHY_TERMINAL__STEP_UP_MAX_AGE` | Server | duration | `60s` | so jung muss der Step-up für `terminal.open` und `terminal.attach` sein | aktiv |
| `terminal.session_token_ttl` | `BOTHY_TERMINAL__SESSION_TOKEN_TTL` | Server | duration | `30s` | Gültigkeit einer Sitzungs-Marke | aktiv |
| `terminal.open_timeout` | `BOTHY_TERMINAL__OPEN_TIMEOUT` | Server | duration | `20s` | Frist, bis beide Seiten im Relais verbunden sind und der Handshake steht | aktiv |
| `terminal.detach_grace` | `BOTHY_TERMINAL__DETACH_GRACE` | Server | duration | `15m` | so lange bleibt eine getrennte Sitzung anhängbar | aktiv |
| `terminal.quick_reattach_window` | `BOTHY_TERMINAL__QUICK_REATTACH_WINDOW` | Server | duration | `60s` | Wiederanhängen vom selben Gerät ohne neuen Step-up innerhalb dieser Zeit nach der Trennung | aktiv |
| `terminal.idle_timeout` | `BOTHY_TERMINAL__IDLE_TIMEOUT` | Server | duration | `30m` | ohne Eingabe des Geräts endet die Sitzung | aktiv |
| `terminal.idle_grace` | `BOTHY_TERMINAL__IDLE_GRACE` | Server |  | `2m` | Zuschlag, nach dem der Server selbst wegen Leerlaufs beendet | geplant (M2) |
| `terminal.max_session_duration` | `BOTHY_TERMINAL__MAX_SESSION_DURATION` | Server |  | `12h` | Höchstdauer einer Sitzung | geplant (M2) |
| `terminal.max_sessions_per_person` | `BOTHY_TERMINAL__MAX_SESSIONS_PER_PERSON` | Server |  | `5` | offene Sitzungen je Mensch | geplant (M2) |
| `terminal.max_sessions_per_host` | `BOTHY_TERMINAL__MAX_SESSIONS_PER_HOST` | Server |  | `10` | offene Sitzungen je Rechner | geplant (M2) |
| `terminal.relay_buffer_max_bytes` | `BOTHY_TERMINAL__RELAY_BUFFER_MAX_BYTES` | Server | int | `262144` | Puffer des Relais je Richtung und Sitzung | aktiv |
| `terminal.host_heartbeat_interval` | `BOTHY_TERMINAL__HOST_HEARTBEAT_INTERVAL` | Server | duration | `20s` | Abstand der Heartbeats der Steuerverbindung (in `hello` gemeldet) | aktiv |
| `terminal.host_heartbeat_miss_limit` | `BOTHY_TERMINAL__HOST_HEARTBEAT_MISS_LIMIT` | Server | int | `3` | fehlende Heartbeats bis zum Schließen der Steuerverbindung | aktiv |
| `terminal.host_auth_timeout` | `BOTHY_TERMINAL__HOST_AUTH_TIMEOUT` | Server | duration | `10s` | Frist zwischen `hello` und `identify` der Steuer- und Datenverbindung | aktiv |
| `terminal.sweep_interval` | `BOTHY_TERMINAL__SWEEP_INTERVAL` | Server |  | `30s` | Abstand des Jobs `terminal.sweep` ([02-architektur.md](../spec/02-architektur.md), Abschnitt 12.3) | geplant (M2) |
| `terminal.host_duplicate_probe_timeout` | `BOTHY_TERMINAL__HOST_DUPLICATE_PROBE_TIMEOUT` | Server | duration | `5s` | so lange wartet der Server auf die Antwort der älteren Steuerverbindung (3.4) | aktiv |
| `host_service.state_dir` |  | Host-Dienst |  | `/var/lib/bothy-host` (Windows: `%ProgramData%\bothy-host`) | Zustandsordner mit den Schlüsseln des Rechners, den Bindungen der Sitzungsbenutzer an Personen und bei `organization` dem API-Schlüssel des Profils `claude`; also the pairing record and the last declaration from `ready`, both without secrets (D-M2-42-4); nie eine Abo-Anmeldung (7.4) |  |
| `host_service.session_users` |  | Host-Dienst |  | leer | Sitzungsbenutzer dieses Rechners, je Person einer; je Eintrag ein PTY-Helfer; nie der Dienstbenutzer; leer heißt: HD1 nicht bestanden; gelesen nur beim Start, geändert mit `bothy-host session-user add` und `remove`, die den Host-Dienst dafür neu starten (2.2) |  |
| `host_service.extra_forbidden_groups` |  | Host-Dienst |  | leer | weitere Gruppen, die HD2 zusätzlich zur festen Liste sperrt |  |
| `host_service.credential_check_interval` |  | Host-Dienst |  | `60s` | Abstand der Prüfungen HD6 und HD11 während des Betriebs (2.2, 7.4) |  |
| `host_service.entry_scan_timeout` |  | Host-Dienst |  | `10m` | Höchstdauer der Prüfung der Einträge von HD11 je Sitzungsbenutzer; danach bricht sein PTY-Helfer sie ab, und HD11 besteht für ihn nicht (`entry_scan_timeout`, 2.2) |  |
| `host_service.auth_status_timeout` |  | Host-Dienst |  | `30s` | bound of one call of `claude auth status` and of the login shell probe of HD6; a timeout gives `auth_status_unknown` (7.4, D-M2-42-3) |  |
| `host_service.profiles` |  | Host-Dienst |  | `[shell]` | angebotene Profile (11.1) |  |
| `host_service.claude_bin` |  | Host-Dienst |  | leer | absoluter Pfad des Coding-Agenten, den HD6 für `claude auth status` aufruft und den das Profil `claude` bei `organization` startet; gehört mit Ordnerkette, Interpreter und den Ordnern der Suchliste einem vertrauenswürdigen Besitzer (2.2 Punkt 6, HD10); unter Windows eine `.exe`; leer heißt: HD6 prüft nur die Anmeldeorte und warnt, bei `organization` gibt es kein Profil `claude`, die Durchsetzung trägt der eigene Sitzungsbenutzer (7.4); für HD6 und das Profil gilt der Wert auch als leer, wenn sich der Pfad nicht auflösen lässt oder die Suchliste den Interpreter eines Skripts nicht findet |  |
| `host_service.claude_workdirs` |  | Host-Dienst |  | leer | Arbeitsordner des Profils `claude` je Sitzungsbenutzer (Name aus `host_service.session_users` -> Liste von Ordnern mit Anzeigenamen; gemeldet werden nur die Anzeigenamen); jeder Ordner gehört genau diesem Sitzungsbenutzer (Ruling T13, HD7, HD11); ein Ordner bei zwei Sitzungsbenutzern fällt aus beiden Listen, gemeldet und für `wdi` benutzt wird die Liste danach; gelesen nur beim Start, eine Änderung wirkt nach `bothy-host restart`; ohne Eintrag kein Profil `claude` für diesen Sitzungsbenutzer, leer heißt kein Profil `claude` |  |
| `host_service.scrollback_bytes` |  | Host-Dienst |  | `1048576` | Rückblick-Puffer je Sitzung, nur im Speicher |  |
| `host_service.require_local_confirm` |  | Host-Dienst |  | `false` | jede PTY erst nach Zustimmung am Rechner (6.4) |  |
| `host_service.local_confirm_timeout` |  | Host-Dienst |  | `60s` | Frist für die Zustimmung am Rechner |  |
| `host_service.reconnect_backoff_base` |  | Host-Dienst |  | `1s` | first delay of the exponential backoff with full jitter before a new control connection (3.5) |  |
| `host_service.reconnect_backoff_cap` |  | Host-Dienst |  | `5m` | largest delay of that backoff; also the total time the login of a refused start keeps trying (3.5) |  |

### 25 Visualisierung

Quelle: [25-visualisierung.md](../spec/25-visualisierung.md), Abschnitt 15.

| Schlüssel | Umgebung | Ort | Typ | Standard | Bedeutung | Stand |
|---|---|---|---|---|---|---|
| `stats.live_enabled` | `BOTHY_STATS__LIVE_ENABLED` | Server |  | `true` | Live-Strom und `getStatsLive` an; aus: `403 feature_disabled` | geplant (M6) |
| `stats.live_interval` | `BOTHY_STATS__LIVE_INTERVAL` | Server |  | `1s` | Abstand der Live-Rahmen; zugleich Lebensdauer des Basis-Rahmens für `getStatsLive` | geplant (M6) |
| `stats.live_max_runs` | `BOTHY_STATS__LIVE_MAX_RUNS` | Server |  | `200` | höchstens so viele aktive Läufe je Rahmen | geplant (M6) |
| `stats.live_max_subscribers` | `BOTHY_STATS__LIVE_MAX_SUBSCRIBERS` | Server |  | `200` | Abos von `stats.live` je api-Knoten | geplant (M6) |
| `stats.live_sli_window` | `BOTHY_STATS__LIVE_SLI_WINDOW` | Server |  | `5m` | Zeitraum der SLI-Werte im Live-Rahmen | geplant (M6) |
| `stats.flush_interval` | `BOTHY_STATS__FLUSH_INTERVAL` | Server |  | `10s` | Abstand, in dem ein Prozess seine Zähler in `stats_minutes` schreibt | geplant (M6) |
| `stats.flush_queue_max` | `BOTHY_STATS__FLUSH_QUEUE_MAX` | Server |  | `20000` | Zählerschlüssel im Speicher je Prozess; darüber werden die ältesten verworfen | geplant (M6) |
| `stats.timeline_max_range` | `BOTHY_STATS__TIMELINE_MAX_RANGE` | Server |  | `24h` | größter Zeitraum einer Timeline-Abfrage | geplant (M6) |
| `stats.timeline_max_runs` | `BOTHY_STATS__TIMELINE_MAX_RUNS` | Server |  | `5000` | höchstens so viele Läufe in einer Timeline-Antwort | geplant (M6) |
| `stats.slo_warn_budget_ratio` | `BOTHY_STATS__SLO_WARN_BUDGET_RATIO` | Server |  | `0.25` | unter diesem Restbudget wird die Ampel gelb | geplant (M6) |
| `stats.slo_min_events` | `BOTHY_STATS__SLO_MIN_EVENTS` | Server |  | `100` | unter so vielen Ereignissen im SLO-Zeitraum ist die Ampel grau | geplant (M6) |
| `console.grafana_embed_url` | `BOTHY_CONSOLE__GRAFANA_EMBED_URL` | Server |  | leer | Dashboard-URL für die Einbettung in die Konsole; leer heißt keine Einbettung und kein `frame-src` | geplant (M6) |
| `viz.live_poll_interval` |  | App |  | `2s` | Abfrage von `getStatsLive` in der Web-Konsole |  |
| `viz.stale_after` |  | App |  | `5s` | ohne neuen Rahmen zeigt die Ansicht "veraltet" |  |
| `viz.timeline_speeds` |  | App |  | `60, 300, 900` | Abspielgeschwindigkeiten der Timeline (Sekunden Tageszeit je Sekunde) |  |
| `viz.tv_rotation_interval` |  | App |  | `30s` | Wechsel der Ansichten im TV-Modus |  |
| `viz.tv_show_costs` |  | App |  | `false` | Kosten im TV-Modus zeigen |  |
| `viz.perf.frame_build_p95` |  | App |  | `8ms` | Ziel: Aufbau eines Frames der Live-Topologie mit `stats.live_max_runs` Läufen |  |
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
| `db_backup_password` | erzeugt | migrate, backup | role `bothy_backup` of `bothy-backupd`, which writes only `backup_runs` (`migrate.backup_password_ref`; `database.password_ref` of `backup`) |
| `token_pepper` | erzeugt | server | `auth.token_pepper_ref` |
| `totp_key` | erzeugt, 32 Byte | server | `auth.totp_key_ref`: AES-256-GCM-Schlüssel der TOTP-Geheimnisse in `local_credentials` ([08-identitaet.md](../spec/08-identitaet.md) 2.5) |
| `telemetry_pseudonym_key` | erzeugt, mindestens 32 Byte | server, collector | `telemetry.pseudonym_key_ref`, `hmac_key` der Allowlist |
| `audit_ip_kek` | erzeugt, 32 Byte | server | `privacy.audit_ip_kek_ref`: verschlüsselt die IP-Schlüssel in `audit_ip_keys` |
| `file_ticket_key` | erzeugt, 32 Byte | server | `file.ticket_key_ref`: HMAC der Abruf-Tickets ([12-dateien.md](../spec/12-dateien.md) 9.2) |
| `webhook_hmac_master_key` | erzeugt, 32 Byte | server | `webhook.hmac_master_key_ref`: master key of the derived webhook secrets ([webhooks.md](../api/webhooks.md) 2.1) |
| `pgbackrest_cipher_pass` | erzeugt | backup | Verschlüsselung der pgBackRest-Repositorys |
| `pgbackrest_tls/` | erzeugt (eigene CA) | postgres, backup | gegenseitiges TLS zwischen Datenbank und Repository-Host: `ca.crt`, `postgres.crt` and `postgres.key`, `backup.crt` and `backup.key` (ECDSA P-256, valid for `stack.PGBackRestTLSValidity`; the CA key is not kept) |
| `restic_password` | erzeugt | backup | Verschlüsselung des restic-Repositorys |
| `s3_access_key`, `s3_secret_key` | erzeugt (Profil s3) oder geliefert (externer Dienst) | server, seaweedfs, backup | S3-Zugang, `file.s3.access_key_ref` und `file.s3.secret_key_ref` |
| `audit_checkpoint_s3_access_key`, `audit_checkpoint_s3_secret_key` | geliefert | server | own access key of the object store of the audit checkpoints, `audit.checkpoint_s3_access_key_ref` and `audit.checkpoint_s3_secret_key_ref`; read only with an `s3://` `audit.checkpoint_sink`, from a separate account ([backup-restore.md](backup-restore.md) 8) |
| `grafana_admin_password` | erzeugt | grafana | Admin-Konto im lokalen Grafana |
| `apns_key.p8` | geliefert | server | Push iOS und macOS, `push.apns.private_key_ref` |
| `fcm_service_account.json` | geliefert | server | Push Android |
| `oidc/<anbieter>_client_secret` | geliefert | server | OIDC mit `client_secret_basic` (`client_secret_ref` in `identity_providers.config`) |
| `oidc/<anbieter>_client_key.pem` | geliefert | server | OIDC mit `private_key_jwt`: privater Schlüssel der Client-Assertion, PKCS#8 (`client_key_ref`, [08-identitaet.md](../spec/08-identitaet.md) 2.6) |
| `offsite/` | geliefert | backup | key of the off-site S3 bucket (`backup.offsite.target`): `access_key_id`, `secret_access_key`, optional `ca.crt` for a private CA; may write and read, never remove a stored version ([backup-restore.md](backup-restore.md) 2) |
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
zeigt dorthin. `bothy secrets export` sichert beide Orte; den Schlüsselspeicher nimmt es mit M1-32
auf, das die Signierschlüssel erzeugt.

Erzeuger späterer Etappen: die erzeugte Form von
`s3_access_key` und `s3_secret_key` (Profil s3, M3). Bis dahin meldet `bothy secrets init` diese
Einträge als `pending`, solange weder Ablage noch Quelle sie enthält; `s3_*` gelten so lange als
geliefert. Erzeugt werden Passwörter als 32 Zufallsbyte in base64url ohne Auffüllung (43 Zeichen)
und Schlüssel (`token_pepper`, `totp_key`, `telemetry_pseudonym_key`, `audit_ip_kek`, `file_ticket_key`,
`webhook_hmac_master_key`) als
32 Zufallsbyte in Hex-Kleinschreibung (64 Zeichen); Nutzer, die rohe Schlüsselbytes brauchen,
dekodieren den Hex-Text.

`pgbackrest_tls/` (M7-01) is generated as a directory: a self-signed CA that may sign only leaf
certificates, and one certificate per pgBackRest host (`postgres`, `backup`) with the host name as
common name and DNS name and both TLS server and client use. The CA key exists only during the
run, so no service can issue further certificates; a rotation removes the directory from `_store`
and runs `secrets init` again, then restarts `postgres` and `backup`. Compose runs `secrets-init`
with `--owner postgres=999:999 --owner backup=999:999`, because pgBackRest reads keys only with
mode `0600` or stricter.

## 6. Instanz-Einstellungen

Gespeichert in `instance_settings` ([03-datenmodell.md](../spec/03-datenmodell.md) 4.1), geprüft
gegen das Schema des Schlüssels.

| Schlüssel | Typ | Standard | Bedeutung |
|---|---|---|---|
| `branding.product_name` | string | `Bothy` | angezeigter Produktname (ADR 0044); höchstens `branding.product_name_max_chars` Zeichen, ohne Steuer- und Bidi-Zeichen |
| `branding.accent_color` | Schlüssel der Kategorie-Farben | `spruce` | Akzentfarbe; nur Schlüssel aus der geprüften Palette (ADR 0044), kein freier Farbwert |
| `branding.logo_file_id` | uuid | leer | Logo als Datei |
| `locale.default` | BCP 47 | `de` | Standardsprache neuer Menschen |

Die Schlüssel des Brandings beschreibt [01-produkt.md](../spec/01-produkt.md), Abschnitt 12; persönliche
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
| `BOTHY_CLAMD_ADDRESS` | `unix:///run/clamav/clamd.sock` | setzt `file.scan.clamd_address` (Kapitel 12); the socket lies in the volume `clamav-socket` |
| `BOTHY_PG_SHM_SIZE` | `256m` | Shared Memory des Datenbank-Containers |
| `BOTHY_PG_ARCHIVE_TIMEOUT` | `60s` | setzt `archive_timeout` in PostgreSQL; gleich `backup.pg.archive_timeout` |
| `BOTHY_BACKUP_OFFSITE_TARGET` | leer | setzt `backup.offsite.target` (`s3:https://<host>[:<port>]/<bucket>[/<prefix>]`) and `BOTHY_PGBACKREST_REPO2` of `postgres` |
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
| `BOTHY_NGINX_VERSION` | `1.30.6-alpine` | image version of the example edge (nginx-unprivileged) of the overlay `compose.multi-node.yaml` ([install-compose.md](install-compose.md) 5.3) |
| `BOTHY_CLAMAV_MEMORY_LIMIT` | `4g` | Speichergrenze des ClamAV-Containers |
| `BOTHY_LOG_MAX_SIZE` | `20m` | Größe einer Container-Logdatei bis zur Rotation |
| `BOTHY_LOG_MAX_FILE` | `5` | behaltene Container-Logdateien je Dienst |

Variables of the image `bothy-postgres` (M7-01) that `infra/compose/compose.yaml` sets itself,
not through `.env`; only the entry point of the image reads them
([image README](../../services/server/images/bothy-postgres/README.md)):

| Variable | Service | Meaning |
|---|---|---|
| `BOTHY_PGBACKREST_REPO_HOST` | postgres | host name of the pgBackRest repository host (`backup`); required |
| `BOTHY_PG_ARCHIVE_TIMEOUT` | postgres | `archive_timeout` of PostgreSQL, from the row above; required |
| `BOTHY_PGBACKREST_PG_HOST` | backup | host name of the database host (`postgres`); required |
| `BOTHY_PGBACKREST_REPO2` | postgres | `on` when `BOTHY_BACKUP_OFFSITE_TARGET` is set, else empty: `archive-push` also writes to repo2 (M7-02) |

## 10. Pflege

1. Ein neuer Server-Schlüssel kommt in derselben Änderung ins Schema
   (`services/server/internal/wire/configkeys_*.go`, Kapitel im Feld `Chapter`) und in die
   Tabelle "Konfigurationswerte dieses Kapitels" seines Kapitels, mit demselben Standard. Danach
   in `services/server` `go generate ./tools/configschema` und im Repository
   `make config-reference`. Das schreibt Abschnitt 4 neu und meldet, was noch fehlt, etwa eine
   Regel in `tools/configdoc/planned.yaml`, die keinen geplanten Schlüssel mehr trifft; sie wird
   gelöscht. `make codegen-check` hält die exportierte Datei gleich dem Schema,
   `make config-doc-check` den Rest (Tabelle unten).
2. Ein Wert, den der Server noch nicht liest, steht nur in der Kapiteltabelle. Mit Ort `Server`
   braucht er eine Etappe in `tools/configdoc/planned.yaml`: eine Regel mit genauem Namen oder
   mit einem Präfix, das auf `.` endet; der genaue Name gilt vor einem Präfix, ein längeres
   Präfix vor einem kürzeren.
3. Ein Name steht in genau einem Kapitel. Ein durch W1 abgelöster Name
   (`tools/configdoc/retired.yaml`) kommt nicht wieder, weder als Name in Code noch als
   Umgebungsvariable.
4. Ein Schlüssel wird nie umgedeutet. Entfernen ist eine Hauptversion des Servers
   ([18-release.md](../spec/18-release.md) 3); vorher steht er eine Minor-Version lang als
   veraltet in den Release Notes und erzeugt beim Start eine Warnung.
5. Prosa nennt nur den Namen, nie die Zahl.

Regeln von `make config-doc-check` (Befund als `config-doc: <pfad>[:<zeile>]: <regel>: <text>`,
Exitcode 1). Lässt sich eine Eingabedatei nicht lesen oder ist sie fehlerhaft, oder fehlen die
Markierungen von Abschnitt 4, stehen sie doppelt oder in falscher Reihenfolge, endet die Prüfung
mit Exitcode 2. `make config-reference` schreibt Abschnitt 4 auch dann, wenn D1 bis D9 etwas
finden, meldet die Befunde und endet dann mit Exitcode 1. Das sind die Exitcodes von
`tools/configdoc/configdoc.py`; `make` selbst endet bei jedem Fehler eines Ziels mit Exitcode 2.

| Regel | Prüft |
|---|---|
| D1 | Kein Name steht in zwei Tabellenzeilen. |
| D2 | Jeder Schlüssel des Schemas hat eine Zeile in einer Kapiteltabelle. |
| D3 | Diese Zeile steht im Kapitel, das das Schema nennt. |
| D4 | Ihr Ort ist `Server`. |
| D5 | Ihr Standard ist der des Schemas: die Zelle ohne Backticks, `leer` für einen leeren Standard, `keiner, Pflicht` für einen Pflicht-Schlüssel. |
| D6 | Jeder geplante Server-Schlüssel hat eine Etappe in `planned.yaml`, und jede Regel dort gibt die Etappe mindestens eines geplanten Schlüssels. |
| D7 | Kein Name aus `retired.yaml` steht in Code (eingezäunter Code-Block oder Code-Span nach CommonMark mit GitHub-Tabellen, auch über Zeilenumbrüche eines Absatzes, Listenpunkts oder Zitats; in anderen Textdateien beenden nur Leerzeilen und Zäune einen Absatz) und keine seiner `BOTHY_`-Variablen irgendwo in einer getrackten Textdatei; ausgenommen sind `docs/PLAN.md`, `docs/DECISIONS.md`, der Brief von M1-08, `retired.yaml` und die Tests von `tools/configdoc`. Ein längerer Name, der einen abgelösten enthält, zählt nicht. Diese Datei wird in der Form geprüft, die `make config-reference` schreibt. Ein Test hält fest, dass D7 auf allen getrackten Markdown-Dateien jeden Namen in Code findet, den ein CommonMark-Parser findet. Mehr zu finden ist gewollt: Zellen jenseits der Kopfzeile, die ein Parser nach GitHub-Tabellen verwirft (etwa wenn ein senkrechter Strich im Code die Zeile teilt; auf der Docs-Site teilt er sie nicht), und Code-Spans in HTML-Blöcken, HTML-Kommentaren und eingerückten Code-Blöcken. Nicht erfasst sind Namen in Prosa ohne Backticks, in eingerückten Code-Blöcken außerhalb von Backticks und in verschachtelter YAML-Form (`a:` mit `b:` darunter). |
| D8 | Mindestens ein Compose-Dienst führt den Server aus (`entrypoint` und `command` beginnen mit `serve` oder mit dem Programm `bothy`, gefolgt von `serve`), und jeder solche Dienst hat ein `stop_grace_period` echt länger als `shutdown.total_timeout` ([02-architektur.md](../spec/02-architektur.md) 15.3): dem Wert, den der Dienst in `environment` für `BOTHY_SHUTDOWN__TOTAL_TIMEOUT` setzt, sonst dem Standard. Merge-Schlüssel (`<<: *api`) gelten; `env_file` und die Konfigurationsdatei liest die Prüfung nicht. |
| D9 | Keine zwei Namen haben dieselbe Umgebungsvariable, und die exportierte Schema-Datei nennt für jeden Schlüssel die Variable der Namensregel aus Abschnitt 2. |
| D10 | Abschnitt 4 ist aktuell. |
| `table` | Jedes Kapitel hat höchstens einen Abschnitt `## <Nummer>. Konfigurationswerte dieses Kapitels`; seine Tabelle hat die Kopfzeile `Name`, `Standard`, `Bedeutung` (mit `Ort` an zweiter Stelle) und eine Trennzeile, jede Zeile so viele Zellen wie die Kopfzeile (`\|` ist ein Strich in einer Zelle), gültige Namen, einen bekannten Ort, eine nicht leere Zelle "Standard" und so viele Standardwerte wie Namen. Zeilen in eingezäunten Code-Blöcken zählen nicht. |
