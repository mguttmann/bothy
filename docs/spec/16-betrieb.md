# 16 Betrieb

Status: Entwurf M0. Normativ für Deployment, Konfiguration, Geheimnisse, Sicherung,
Wiederherstellung, Upgrade, Umgebungen, Feature-Flags, Statusseite und Kapazität.
Begriffe: [00-glossar.md](00-glossar.md). Tabellen: [03-datenmodell.md](03-datenmodell.md).
Sync: [04-sync-protokoll.md](04-sync-protokoll.md). Aktionen: [05-aktionen.md](05-aktionen.md).
Telemetrie: [17-observability.md](17-observability.md). Releases: [18-release.md](18-release.md).
Anleitungen (Runbooks) stehen unter [docs/operations/](../operations/configuration.md).

Alle Grenzen, Fristen und Intervalle sind benannte Konfigurationswerte. Ihre Standardwerte stehen
in Abschnitt 14 und gesammelt in der [Konfigurationsreferenz](../operations/configuration.md).

## 1. Ziele

1. Ein Befehl startet eine vollständige Instanz auf einem Host mit Docker-kompatibler Laufzeit.
2. Mac, Linux-VM und Cloud nutzen dieselben Images. Unterschiede stehen nur in der Konfiguration.
3. Datenverlust bleibt unter `dr.rpo_target`, die Wiederherstellung unter `dr.rto_target`. Ein
   automatischer Wiederherstellungstest misst beides regelmäßig.
4. Ein Upgrade braucht keine Handarbeit an Daten. Der Rückweg um eine Version ist immer möglich.
5. Geheimnisse stehen nie in Images, Umgebungsvariablen, Logs, Telemetrie oder OpenTofu-State.

## 2. Bausteine einer Instanz

| Baustein | Image | Rolle |
|---|---|---|
| `api` | `bothy-server` | REST, Gateway, Fan-out (`bothy serve --role=api`, B1) |
| `worker` | `bothy-server` | Hintergrundjobs: Scan, Vorschaubilder, Push, Webhooks, Aufbewahrung, Statusprüfungen (`--role=worker`) |
| `migrate` | `bothy-server` | einmaliger Lauf vor `api` und `worker`: Rollen und Migrationen |
| `secrets-init` | `bothy-server` | einmaliger Lauf: Geheimnisse erzeugen und je Dienst ablegen (Abschnitt 5) |
| `postgres` | `bothy-postgres` | PostgreSQL 18 (B3) plus pgBackRest für die WAL-Archivierung |
| `backup` | `bothy-postgres` | `bothy-backupd`: pgBackRest-Repository, logische Sicherung, Objektsicherung, Wiederherstellungstest |
| `otel-collector` | OpenTelemetry Collector contrib | Telemetrie-Annahme, Datenschutz-Filter, Export (B11) |
| `clamav` | ClamAV | Virenscan der Uploads, eigener Prozess (ADR 0033, Team G) |
| `seaweedfs` | SeaweedFS | optionaler S3-Dienst für Einzelhosts (R-Objektspeicher) |
| `tailscale` | Tailscale | optionaler Rand mit `tailscale serve` (B9) |
| Observability-Stack | Prometheus, Alertmanager, Loki, Tempo, Grafana | optionaler lokaler Standard-Stack (ADR 0034) |

`bothy-postgres` ist das offizielle `postgres:18` plus pgBackRest und `bothy-backupd`. Ein
eigenes Image ist nötig, weil `archive_command` im Datenbank-Container läuft (Abschnitt 8.2).
Beide Bothy-Images entstehen in der Release-Kette aus [18-release.md](18-release.md).

Die Compose-Datei [infra/compose/compose.yaml](../../infra/compose/compose.yaml) ist die
Referenz dieser Bausteine. Profile schalten optionale Teile zu: `direct` oder `tailscale` (Rand),
`scan`, `s3`, `observability`.

![Compose-Stack](../operations/diagrams/deployment-compose.svg)

## 3. Deployment-Ziele

| Ziel | Laufzeit | Datenbank | Dateien | Rand | Geheimnisse |
|---|---|---|---|---|---|
| Mac | Docker Desktop oder Colima, Compose | Container `postgres` | Volume `files` | `direct` mit Reverse-Proxy oder `tailscale` | Treiber `file` |
| Linux-VM | Docker Engine, Compose | Container `postgres` | Volume oder S3-Dienst | Reverse-Proxy, `tailscale` oder privates Netz | `file` |
| Azure | VM mit Compose | Container oder Azure Database for PostgreSQL | Volume oder SeaweedFS | Application Gateway oder privates Netz | `file`, später `azkv` (Azure Key Vault) |
| AWS | EC2 mit Compose | Container oder Amazon RDS for PostgreSQL | Amazon S3 | Load Balancer oder privates Netz | `file`, später `awssm` (AWS Secrets Manager) |
| Hetzner | Cloud-Server mit Compose | Container | Volume oder Hetzner Object Storage | Reverse-Proxy oder `tailscale` | `file` |
| Kubernetes | später, Helm (`infra/helm`) | verwalteter Dienst | S3 | Ingress | Treiber des Clusters |

Regeln:

1. Der Server hat keine Mac-Abhängigkeit (B12). Der Mac ist nur ein Host für Container.
2. Auf Mac und Windows läuft Docker in einer VM. Ihr Speicher muss für alle Dienste reichen.
   ClamAV allein braucht laut Hersteller mindestens 3 GiB, besser 4 GiB
   (https://docs.clamav.net/manual/Installing/Docker.html, abgerufen 2026-10-10) [belegt].
3. Mit einem verwalteten PostgreSQL-Dienst übernimmt der Anbieter WAL-Archivierung und
   Point-in-Time-Recovery. `bothy-backupd` macht dann nur die logische Sicherung und die
   Objektsicherung (Abschnitt 8.1).
4. Azure Blob Storage bietet keine S3-API [vermutet, nicht geprüft]. Der S3-Treiber (B4) passt
   darum auf Azure nur mit SeaweedFS oder einem anderen S3-Dienst. Ein eigener Blob-Treiber ist
   eine Option für Kapitel 12.
5. Hetzner Object Storage ist S3-kompatibel und kennt Versionierung und Object Lock
   (https://docs.hetzner.com/storage/object-storage, abgerufen 2026-10-10) [belegt].
6. Ob die verwalteten Dienste von Azure und AWS PostgreSQL 18 anbieten, ist nicht geprüft
   [vermutet]. Das Modul-Ticket prüft es vor der Implementierung.
7. Mindestversionen für Compose: Docker Engine 26 (Volume-`subpath`) und Compose 2.26 (`configs`
   mit `content`, `depends_on.required`). Belege: Release Notes Docker Engine 26.0
   (https://docs.docker.com/engine/release-notes/26.0/) und Compose-Referenz
   (https://docs.docker.com/reference/compose-file/configs/), abgerufen 2026-10-10 [belegt].

OpenTofu-Module je Ziel beschreibt [infra/tofu/README.md](../../infra/tofu/README.md).

## 4. Netz

### 4.1 Grundregeln

1. Jeder veröffentlichte Port bindet an `127.0.0.1` des Hosts. TLS und öffentliche Erreichbarkeit
   liefert ein Rand (Reverse-Proxy, Load Balancer, `tailscale serve`), nie der Server selbst.
2. PostgreSQL hängt nur am internen Netz `internal` ohne Ausgang. Sein Port ist nie am Host
   veröffentlicht; so kollidiert er auch nie mit einem PostgreSQL des Hosts.
3. Ausgang brauchen nur `api` (OIDC-Anbieter), `worker` (Push-Dienste, Webhooks), `backup`
   (Off-site-Ziel), `otel-collector` (Telemetrie-Backend), `clamav` (Signaturen) und Alertmanager.
4. Der Server lädt keine fremden URLs (Baseline, SSRF). Ausgehende Webhooks gehen nur an Hosts
   aus `webhook.allowed_hosts` ([03-datenmodell.md](03-datenmodell.md), R-DM-17).

### 4.2 Zwei Listener

| Listener | Schlüssel | Inhalt |
|---|---|---|
| öffentlich | `server.listen_addr` | REST und Gateway |
| intern | `server.internal_listen_addr` | `/healthz`, `/readyz`, Eingangs-Webhooks von Diensten im Stack (Alertmanager) |

`/healthz` meldet nur, dass der Prozess lebt. `/readyz` meldet bereit, wenn die Datenbank
antwortet, das Schema zur Binärversion passt (Abschnitt 10) und der Objektspeicher antwortet.
Beide Antworten enthalten keine Details außer dem Namen der fehlgeschlagenen Prüfung.

### 4.3 Rand und Identität

| Rand | Anmeldung | Bedingung |
|---|---|---|
| Reverse-Proxy (Profil `direct`) | OIDC oder lokale Konten | `server.trusted_proxies` nennt den Proxy, damit die Client-IP für `rl.public_auth` stimmt |
| `tailscale serve` (Profil `tailscale`) | Identitätsanbieter `trusted_header` | Der öffentliche Listener lauscht nur auf `127.0.0.1` im Netz-Namespace des Sidecars |
| privates Netz der Cloud | OIDC | Load Balancer oder Application Gateway terminiert TLS |

Der Server prüft beim Start: Ist ein Anbieter `trusted_header` aktiv und `server.listen_addr`
keine Loopback-Adresse, meldet `/readyz` den Fehler `trusted_header_requires_loopback` und der
Anbieter bleibt aus (fail closed). Grund: Jeder, der den Listener direkt erreicht, könnte die
Kopfzeilen selbst setzen. Tailscale nennt das ausdrücklich
(https://tailscale.com/kb/1312/serve, abgerufen 2026-10-10) [belegt]. Details:
[tailscale.md](../operations/tailscale.md).

## 5. Konfiguration und Geheimnisse

### 5.1 Konfiguration (ADR 0011)

1. Prozess-Konfiguration kommt aus Umgebungsvariablen und einer optionalen YAML-Datei
   (`BOTHY_CONFIG_FILE`). Vorrang: Umgebung vor Datei vor Standardwert.
2. Namensregel: Schlüssel `a.b_c.d` heißt in der Umgebung `BOTHY_A__B_C__D` (Punkt wird `__`).
   Die Regel ist umkehrbar, weil Schlüssel nie zwei Unterstriche hintereinander enthalten.
3. Unbekannte Variablen mit Präfix `BOTHY_` und unbekannte Schlüssel in der Datei brechen den
   Start ab. Grund: Ein Tippfehler darf nie still eine Sicherheitsgrenze auf den Standard setzen.
4. Ungültige Werte (Typ, Bereich) brechen den Start ab. `bothy config check` prüft ohne Start,
   `bothy config print --redacted` zeigt die wirksame Konfiguration ohne Geheimnisse.
5. Prozess-Konfiguration wird nur beim Start gelesen. Ausnahme: Geheimnisse (5.2).
6. Instanz-Einstellungen (`instance_settings`, z. B. `branding.*`) und Feature-Flags
   (`feature_flags`) sind keine Prozess-Konfiguration. Sie ändern sich zur Laufzeit über die
   Admin-Konsole mit Step-up und Audit ([05-aktionen.md](05-aktionen.md) 4.2).
7. Standard-Variablen von OpenTelemetry (`OTEL_*`) gelten unverändert (Kapitel 17).

Die vollständige Liste aller Schlüssel mit Umgebungsname, Typ, Standard und Bedeutung steht in
[docs/operations/configuration.md](../operations/configuration.md).

### 5.2 Secret-Provider (ADR 0011)

Jedes Geheimnis ist ein Verweis `secret_ref` (Spalten `secret_ref`, `private_key_ref` in
[03-datenmodell.md](03-datenmodell.md), Schlüssel `*_ref` in der Konfiguration). Form:
`<schema>:<pfad>`.

| Schema | Beispiel | Treiber | Schreiben | Status |
|---|---|---|---|---|
| `file` | `file:/run/bothy/secrets/db_app_password` | Datei lesen | nein | M1 |
| `env` | `env:BOTHY_SECRET_DB_APP_PASSWORD` | Umgebungsvariable lesen | nein | M1, nur wo die Plattform nichts anderes kann |
| `azkv` | `azkv://vault-name/secret-name` | Azure Key Vault | ja | später |
| `awssm` | `awssm://eu-central-1/bothy/db_app_password` | AWS Secrets Manager | ja | später |

Schnittstelle des Treibers: `Get(ref) -> (Wert, Version)`, optional `Put(ref, Wert)` bei
schreibbaren Treibern. Der Server hält gelesene Werte höchstens `secrets.cache_ttl` und liest
danach neu; so wirkt eine Rotation ohne Neustart. Werte erscheinen nie in Logs, Telemetrie,
Fehlermeldungen oder Ereignissen. `env:` ist erlaubt, aber nicht empfohlen: Umgebungsvariablen
erben Kindprozesse und Absturzberichte.

Fehlt ein Verweis bei einem Schlüssel `*_ref`, gilt `file:<secrets.dir>/<name>` mit dem Namen aus
dem Katalog in der Konfigurationsreferenz (Abschnitt "Geheimnisse").

### 5.3 Geheimnisse im Compose-Stack

1. Der Owner legt gelieferte Geheimnisse (Push-Schlüssel, OIDC-Client-Secret, Zugang zum
   Off-site-Ziel, Telemetrie-Schlüssel, Webhook-URLs für Alarme) als Dateien in
   `infra/compose/secrets/` ab. Das Verzeichnis ist nie im Repository.
2. `secrets-init` erzeugt fehlende Geheimnisse (Datenbank-Passwörter, Token-Pepper,
   Pseudonym-Schlüssel der Telemetrie, Schlüssel der Sicherung, Signierschlüssel) und legt im
   Volume `secrets` je Dienst ein Unterverzeichnis an. Jeder Dienst bindet nur sein
   Unterverzeichnis ein (`subpath`), nur lesend. Bestehende Geheimnisse überschreibt es nie.
3. `secrets-init` gibt am Ende die Fingerabdrücke aus, nie die Werte, und erinnert an die
   Hinterlegung (5.4).

### 5.4 Hinterlegung (Escrow)

Ohne einige Geheimnisse ist eine Sicherung wertlos. Der Owner exportiert sie nach jeder Änderung
verschlüsselt (`bothy secrets export --age-recipient <öffentlicher Schlüssel>`) und bewahrt das
Bündel getrennt vom Server und getrennt vom Off-site-Ziel auf.

| Geheimnis | Folge bei Verlust |
|---|---|
| Token-Pepper | Alle API-Tokens, Refresh-Tokens und Eingangs-Webhook-Tokens werden ungültig (Hashes nicht mehr prüfbar) |
| Signierschlüssel | Zugriffstokens und Aufruf-Marken ungültig; alle Geräte melden sich neu an |
| Schlüssel der Sicherung (pgBackRest, restic) | Sicherungen nicht mehr lesbar |
| Push-Schlüssel (APNs, FCM) | Kein Push, bis der Owner neue Schlüssel beim Anbieter erzeugt |
| Pseudonym-Schlüssel der Telemetrie | Alte und neue Pseudonyme sind nicht mehr verknüpfbar (gewollte Folge einer Rotation) |

## 6. Umgebungen und Feature-Flags

### 6.1 Umgebungen

| Umgebung | Zweck | Daten | Push | Schutz |
|---|---|---|---|---|
| `dev` | Entwicklung, lokale Tests | nur synthetisch | APNs-Sandbox | Lockerungen nur hier erlaubt, z. B. HTTP ohne TLS auf Loopback |
| `staging` | Abnahme eines Release-Kandidaten, Portabilitätstest | synthetisch oder Import aus Testquellen | eigene Schlüssel | wie `prod`; Banner "Staging" in den Apps |
| `prod` | Betrieb | echte Daten | Produktion | alle Prüfungen scharf |

Regeln:

1. `instance.environment` ist der Standard `prod`. Lockerungen muss `dev` ausdrücklich setzen.
2. Zwischen Umgebungen wandert dasselbe Image per Digest, nie ein neu gebautes.
3. Produktionsdaten werden nie nach `dev` oder `staging` kopiert. Es gibt kein
   Anonymisierungswerkzeug; ein Datenabzug wäre ein Datenschutzvorfall (Kapitel 20).
4. Jede Umgebung hat eigene Geheimnisse, eigene Push-Schlüssel und eine eigene `instance_id`.

### 6.2 Feature-Flags

| Thema | Regel |
|---|---|
| Speicher | Tabelle `feature_flags` ([03-datenmodell.md](03-datenmodell.md) 4.1): globaler Wert plus `rollout` mit Personen und Gruppen, für die der Wert abweichend gilt |
| Registrierung | Jedes Flag ist im Code registriert: Schlüssel, Art (`release` oder `option`), Beschreibung, Standard, bei `release` die Version `remove_by` |
| Auswertung | `flags.Enabled(principal, key)` im Server; Wert = global, umgekehrt, wenn der Prinzipal oder eine seiner gültigen Gruppen in `rollout` steht. Unbekannte Schlüssel sind aus. Clients bekommen die für sie ausgewerteten Werte per REST; `instance.updated` nennt nur geänderte Schlüssel |
| Admin-Konsole | Liste mit Art, Beschreibung, globalem Wert, Rollout-Listen und letzter Änderung; Ändern mit `feature_flag.update` (Owner, Step-up, Audit) |
| Aufräumregel | Ein `release`-Flag verschwindet spätestens in Version `remove_by`. Ein Check im Gate schlägt fehl, wenn die Version des Servers `remove_by` erreicht und das Flag noch registriert ist. Die Migration, die den Code entfernt, löscht auch die Zeile |
| Grenze | Flags schalten Funktionen, nie Sicherheitsgrenzen, Grenzwerte oder Geheimnisse; dafür gibt es Konfiguration |

`option`-Flags sind dauerhafte Produktschalter, z. B. `dm.enabled`, `voice.enabled`,
`tokens.personal_enabled` ([05-aktionen.md](05-aktionen.md)). Anleitung:
[environments-and-flags.md](../operations/environments-and-flags.md).

## 7. Datenbank

### 7.1 Rollen

| Rolle | Rechte | Nutzer |
|---|---|---|
| `postgres` | Superuser | nur `migrate --bootstrap` und `bothy-backupd` |
| `bothy_owner` | Eigentümer des Schemas, DDL | `migrate` |
| `bothy_app` | DML; auf `audit_events` nur `INSERT` und `SELECT` | `api`, `worker` |
| `bothy_maint` | Kürzen des Audit-Logs am Fristende mit Anker-Eintrag | Aufbewahrungsjob ([03-datenmodell.md](03-datenmodell.md) 4.12) |
| `bothy_monitor` | `pg_monitor` | `otel-collector` |

`migrate --bootstrap` legt fehlende Rollen idempotent an und setzt ihre Passwörter aus den
Geheimnissen. Mit einem verwalteten Dienst legt das OpenTofu-Modul die Rollen an.

### 7.2 Einstellungen

| Einstellung | Wert | Grund |
|---|---|---|
| `wal_level` | `replica` | WAL-Archivierung |
| `archive_mode` | `on` | PITR |
| `archive_timeout` | `backup.pg.archive_timeout` | begrenzt den Datenverlust bei wenig Schreiblast |
| Isolationsstufe | `READ COMMITTED` | Zähler-Muster ([04-sync-protokoll.md](04-sync-protokoll.md) 3.2) |
| `statement_timeout` je Anwendungsverbindung | `database.statement_timeout` | keine hängenden Abfragen |

## 8. Sicherung und Wiederherstellung

### 8.1 Was gesichert wird

| Daten | Werkzeug | Häufigkeit | Aufbewahrung | Ziel |
|---|---|---|---|---|
| PostgreSQL physisch, WAL fortlaufend | pgBackRest | WAL je Segment, spätestens nach `backup.pg.archive_timeout`; Vollsicherung `backup.pg.full_schedule`, Differenz `backup.pg.diff_schedule` | `backup.pg.retention_full` Vollsicherungen | repo1 lokal, repo2 off-site |
| PostgreSQL logisch | `pg_dump -Fc` | `backup.logical.schedule` | `backup.logical.retention` | restic off-site |
| Objekte, Treiber `fs` | restic | `backup.objects.schedule` | `backup.objects.retention` | restic off-site |
| Objekte, Treiber `s3` | Versionierung und Replikation des Anbieters, sonst Spiegel ohne Löschen | wie oben | wie oben | zweiter Bucket |
| Geheimnisse | `bothy secrets export` (age) | nach jeder Änderung | bis zur Ablösung | offline beim Owner (5.4) |
| Konfiguration (`.env`, Overlays) | eigenes Repository des Betreibers | jede Änderung | unbegrenzt | Repository |

Begründungen:

1. pgBackRest (MIT) bietet Voll-, Differenz- und Inkrementalsicherung, Verschlüsselung des
   Repositorys, Prüfsummen je Datei mit Prüfung bei Restore und `verify`, mehrere Repositorys
   und Aufbewahrungsregeln (https://pgbackrest.org/, abgerufen 2026-10-10) [belegt]. Ein eigenes
   Skript mit `pg_basebackup` müsste all das nachbauen.
2. Der logische Abzug ist unabhängig von Hauptversion und Anbieter. Er ist das Werkzeug für den
   Umzug (Abschnitt 11) und die letzte Rückfallebene.
3. Dateien ändern sich nie (`object_key` eindeutig, keine Deduplizierung, R-DM-13). Eine
   inkrementelle, verschlüsselte Sicherung mit Snapshots passt; restic prüft Stichproben mit
   `check --read-data-subset` (https://restic.readthedocs.io/en/stable/045_working_with_repos.html,
   abgerufen 2026-10-10) [belegt].
4. Am 2026-10-04 hat pgBackRest 2.59.3 einen Fehler behoben, durch den schwache Schlüssel und
   Salts entstehen konnten (https://pgbackrest.org/release.html, abgerufen 2026-10-10) [belegt].
   `bothy-postgres` nutzt nur Versionen ab 2.59.3. Ob vorher erzeugte Repositorys neu
   verschlüsselt werden müssen, ist offen; für neue Instanzen stellt sich die Frage nicht.

### 8.2 Ablauf

1. `postgres` schiebt jedes WAL-Segment mit `archive-push` über das TLS-Protokoll von pgBackRest
   an den Repository-Host `backup`. Der Datenbank-Container braucht dafür keinen Ausgang und
   keine Zugangsdaten des Off-site-Ziels.
2. `bothy-backupd` startet Voll- und Differenzsicherungen, `pg_dump`, die Objektsicherung und
   `expire` nach Plan. Jeder Lauf schreibt ein Manifest: Zeilenzahl je Tabelle, `head_seq` des
   Ereignis-Logs, letzter `seq` und `hash` des Audit-Logs, Schema-Version, Server-Version, Zahl
   und Summe der Dateigrößen.
3. Off-site-Repositorys sind verschlüsselt und nur anhängend: Object Lock oder ein
   Append-only-Zugang. Löschen nach Ablauf der Frist darf nur ein getrenntes Konto. Grund: Wer den
   Server übernimmt, darf die Sicherungen nicht löschen können (Ransomware).
4. Jeder Lauf schreibt eine Zeile in `backup_runs` (Kern-Vertrag: Ergänzung beantragt) und meldet
   Kennzahlen (Kapitel 17). Die Admin-Konsole und die Statusseite lesen die Tabelle.

![Sicherung](../operations/diagrams/backup-flow.svg)

### 8.3 Automatischer Wiederherstellungstest

Nach Plan `backup.restore_test.schedule` stellt `bothy-backupd` die letzte Sicherung in
`backup-scratch` wieder her, startet dort eine Datenbank nur auf einem Unix-Socket und prüft:

1. `pgbackrest verify` und die Prüfsummen beim Restore.
2. Schema-Version und Zeilenzahlen gegen das Manifest.
3. Ereignis-Log lückenlos: je Partition `count(*) = max(seq) - min(seq) + 1`, und
   `sequence_counters` ist mindestens so groß wie jede vergebene Nummer.
4. Audit-Kette: dieselbe Prüfung wie `audit.verify` vom Anker bis zum Ende.
5. Stichprobe `backup.restore_test.sample_threads` Threads: `thread_seq` lückenlos.
6. Stichprobe `backup.restore_test.sample_files` Dateien: Objekt aus der Objektsicherung holen,
   SHA-256 gegen `files.sha256`.
7. Dauer des ganzen Tests gegen `dr.rto_target`.

Das Ergebnis geht in `backup_runs` (Art `restore_test`) und in die Kennzahlen. Fehlt ein
erfolgreicher Test länger als `backup.restore_test.max_age`, alarmiert das System. Anleitung:
[backup-restore.md](../operations/backup-restore.md).

### 8.4 Wiederherstellung und Sync (Kern-Vertrag: Ergänzung beantragt)

Eine Wiederherstellung setzt die globale Sequenz zurück. R-SY-04 erkennt das nur, solange
`cursor > head_seq` gilt. Nach neuen Schreibvorgängen überholt die neue Zeitlinie alte Cursor,
und ein Client würde fremde Ereignisse mit alten Nummern still überspringen.

Geplante Lösung: Die Tabelle `instance` bekommt `sync_epoch` (UUIDv7). `bothy restore finalize`
setzt sie nach jeder Wiederherstellung neu. `hello`, `ready` und `GET /v1/events` nennen die
Epoche; der Client speichert sie mit dem Cursor. Passt sie nicht, antwortet der Server
`resync_required` mit Grund `epoch_changed`. Derselbe Befehl setzt alle
`agent_consumer_cursors.acked_seq` und `webhook_subscriptions.last_delivered_seq` auf den neuen
`head_seq` und schreibt einen Audit-Eintrag. Ein Umzug ohne Datenverlust (Abschnitt 11) ändert
die Epoche nicht.

## 9. Disaster Recovery

Ziele: `dr.rpo_target` (Datenverlust) und `dr.rto_target` (Dauer bis zum Betrieb). Das Runbook
[disaster-recovery.md](../operations/disaster-recovery.md) behandelt diese Fälle:

| Fall | Weg |
|---|---|
| Daten versehentlich gelöscht oder beschädigt | PITR auf einen Zeitpunkt davor, neue Epoche |
| Host verloren | neuer Host, Secrets-Bündel, Restore aus repo2 und restic, neue Epoche |
| Off-site-Ziel verloren | neues Ziel, sofort Vollsicherung, Alarm bis zum ersten Test |
| Signierschlüssel kompromittiert | Rotation ([05-aktionen.md](05-aktionen.md) `signing_key.rotate`), alle Sitzungen widerrufen |
| Token-Pepper kompromittiert | neuer Pepper, alle API-Tokens neu ausgeben, alle Sitzungen widerrufen |
| Ransomware auf dem Host | Host als verloren behandeln; Off-site-Daten sind unveränderlich |

## 10. Upgrade

1. Jede Version bringt ihre Migrationen mit; `migrate` läuft vor dem Start von `api` und
   `worker`. Gibt es offene Migrationen auf einem bestehenden Schema und keine erfolgreiche
   Sicherung jünger als `migrate.recent_backup_max_age`, bricht `migrate` ab, solange
   `migrate.require_recent_backup` gilt. Eine leere Datenbank (Erstinstallation) ist ausgenommen.
2. Migrationen folgen Expand/Contract: Version N darf nur entfernen, was Version N-1 nicht mehr
   nutzt. Damit läuft der Code von N-1 gegen das Schema von N, und der Rückweg ist ein
   Image-Wechsel ohne Datenbank-Rückbau.
3. Down-Migrationen gibt es in Produktion nicht. Fehler korrigiert die nächste Version.
4. Der Server prüft beim Start das Schema: Ist es älter als nötig, oder verlangt es eine neuere
   Server-Version (`instance.min_server_version`, gesetzt von `migrate`; Kern-Vertrag: Ergänzung
   beantragt), meldet `/readyz` einen Fehler und der Server nimmt keinen Betrieb auf. Ein
   neueres, aber verträgliches Schema ist erlaubt; das ist der Rückweg aus Punkt 2.
5. Ein Major-Upgrade von PostgreSQL ist ein eigener Schritt mit Wartungsfenster.

Regeln im Detail und Ablauf: [upgrade.md](../operations/upgrade.md).

## 11. Umzug und Cloud-Portabilität

Ein Umzug ist eine Wiederherstellung auf einem anderen Ziel ohne Datenverlust:

1. Ziel bereitstellen (OpenTofu), gleiche Version, Secrets-Bündel einspielen.
2. Probe: letzte Sicherung zurückspielen, Prüfungen aus 8.3 grün.
3. Wartungsmodus `server.maintenance_mode = read_only` auf der alten Instanz. Schreibende Anfragen
   bekommen `503` mit Fehlercode `maintenance` und `Retry-After` (Kapitel 06 legt Code und Format
   fest). Clients halten ihr Ausgangsfach.
4. Abschluss-Sicherung, Restore auf dem Ziel, Manifest vergleichen.
5. DNS und Identität umstellen: Redirect-URIs bei OIDC, RP-ID der Passkeys (gleiche Domain behält
   die Passkeys), Tailscale-Gerät. Push-Schlüssel und App-Identität bleiben gleich, darum bleiben
   die Push-Registrierungen gültig.
6. Clients verbinden mit demselben Cursor neu; weil kein Ereignis verloren ging, bleibt die
   Epoche gleich und es gibt kein Neuladen.

Ändert sich der Hostname, brauchen die Apps die neue Adresse. Geplant: Die alte Instanz meldet im
Wartungsmodus über `instance.discovery.read` ein `moved_to`, signiert mit dem Instanzschlüssel;
der Client übernimmt es nur bei gleicher `instance_id` (Frage an Team B, siehe Bericht).
Anleitung: [cloud-migration.md](../operations/cloud-migration.md).

### 11.1 Portabilitätstest in CI

Ein Test belegt die Portabilität bei jedem Release-Kandidaten und nachts:

1. Frischer Linux-Runner (amd64 und arm64), nur Docker; Images per Digest aus der Registry.
2. `docker compose up` mit `direct,scan` und einmal mit `direct,scan,s3`.
3. Smoke-Tests über die öffentliche API: Owner per Server-Befehl, Bereich, Kanal, Thread,
   Nachricht, WebSocket-Empfang, Datei-Upload sauber und mit EICAR-Testdatei, Lauf eines
   Test-Adapters, Suche, Statusseite.
4. Sicherung auslösen, Wiederherstellungstest auslösen, alle Prüfungen grün.
5. Telemetrie über das Overlay `local` empfangen; Datenschutz-Kanarie (Kapitel 17) nicht gefunden.
6. Ergebnis als Status-Check am Release.

Der Test gehört zur Gate-Kette von Team H (ADR 0038, 0039); dieses Kapitel legt den Inhalt fest.

## 12. Statusseite

| Teil | Regel |
|---|---|
| Komponenten | `status_components`; bei der Einrichtung angelegt: `api` (Nachrichten und Sync), `push`, `files` (Dateien und Scan), `search`; je Agent eine Komponente mit `agent_id` |
| Automatischer Zustand | Der `worker` prüft alle `status.check_interval` die Quellen aus `health_source` und setzt den Zustand. Eine Prüfung scheitert, wenn sie keine Antwort bekommt oder ein Fehleranteil über `status.error_ratio_degraded` liegt; nach `status.degrade_after` mit scheiternden Prüfungen gilt `degraded`, über `status.error_ratio_outage` oder ohne jede Antwort `major_outage` |
| Störungen | Dauert `major_outage` länger als `status.incident_after`, legt der `worker` eine Störung an (`created_by` NULL). Nach Erholung und `status.resolve_after` setzt er sie auf `resolved`. Der Owner kann jederzeit Texte ergänzen (`status.manage`) |
| Agenten | Zustand aus `agents.reported_runtime_state` (Heartbeat). Menschen sehen nur Agenten mit `agent.read` |
| Sichtbarkeit | `status.read` für angemeldete Prinzipale; ohne Anmeldung nur mit `status.public_enabled` |
| Grenze | Die Statusseite läuft im Server. Fällt der ganze Server aus, zeigt die App "Server nicht erreichbar"; den Owner erreicht dann nur der externe Alarmweg (Kapitel 17, Watchdog) |

| `health_source` | Prüfung |
|---|---|
| `api.http` | Anteil 5xx seit der letzten Prüfung, aus allen `api`-Prozessen |
| `sync.fanout` | Fan-out-Verzug aller `api`-Prozesse unter `status.fanout_lag_degraded` |
| `push.apns`, `push.fcm` | Fehleranteil der Sendungen seit der letzten Prüfung, ohne ungültige Tokens |
| `files.storage` | Schreiben, Lesen, Löschen eines Prüfobjekts |
| `files.scan` | `PING` an clamd |
| `search.query` | Testabfrage gegen den Index |
| `agent.presence` | Zustand `online` des Agenten der Komponente |

Sicherung und Wiederherstellungstest sind keine Komponenten der Statusseite, weil diese alle
Menschen sehen. Ihr Zustand steht in der Admin-Konsole (`backup_runs`) und in den Alarmen.

## 13. Kapazität und Ressourcen

Feste Größen gibt es erst nach dem Lasttest (Ticket). Bis dahin gelten Startwerte aus der
Konfiguration und dieses Modell:

| Engpass | Kennzahl | Hebel |
|---|---|---|
| Serieller Abschnitt am Zähler `event_seq` ([04-sync-protokoll.md](04-sync-protokoll.md) 3.2) | `bothy.db.event_seq.lock_wait` | schnellerer Datenträger für WAL, kürzere Transaktionen |
| Verbindungen zur Datenbank | `postgresql.backends`, Pool-Kennzahlen | `database.max_conns`, mehrere `api`-Prozesse nur mit Pooler |
| WebSocket-Verbindungen | `bothy.gateway.connections.active` | weitere `api`-Prozesse; Fan-out liest jeder Prozess selbst |
| Fan-out-Verzug | `bothy.event.fanout.lag` | weniger Empfänger je Prozess |
| Speicher ClamAV | Container-Speicher | eigener Host für den Scan |

Plattenbedarf schätzt der Betreiber mit:

```
Datenbank   ≈ Nachrichten pro Tag × mittlere Größe × Tage
            + Ereignisse pro Tag × mittlere Nutzlast × sync.event_retention in Tagen
            + Indexe (ihren Anteil misst der Lasttest)
WAL-Archiv  ≈ WAL pro Tag × Tage bis zur ältesten behaltenen Vollsicherung
Dateien     ≈ Uploads pro Tag × mittlere Größe × Tage (plus Vorschaubilder)
Sicherungen ≈ Vollsicherungen × Datenbank + WAL-Archiv + restic-Snapshots (dedupliziert)
```

Die Kennzahl `bothy.storage.volume.free` warnt rechtzeitig (Alarm `BothyVolumeFull`).

## 14. Konfigurationswerte dieses Kapitels

| Name | Standard | Bedeutung |
|---|---|---|
| `instance.environment` | `prod` | `dev`, `staging` oder `prod` (6.1) |
| `server.role` | `all` | `api`, `worker` oder `all` |
| `server.listen_addr` | `127.0.0.1:8080` | öffentlicher Listener (REST, Gateway) |
| `server.internal_listen_addr` | `127.0.0.1:8081` | Health, Readiness, Eingangs-Webhooks aus dem Stack |
| `server.public_url` | keiner, Pflicht | Basis-URL, unter der Apps den Server erreichen |
| `server.trusted_proxies` | leer | CIDR-Liste, deren `X-Forwarded-For` gilt |
| `server.shutdown_grace` | `30s` | Zeit zum Leeren beim Beenden; Verbindungen schließen mit `4013` |
| `server.maintenance_mode` | `off` | `off` oder `read_only` (Umzug, Upgrade) |
| `secrets.dir` | `/run/bothy/secrets` | Basis der Standardverweise `file:` |
| `secrets.cache_ttl` | `5m` | so lange gilt ein gelesenes Geheimnis |
| `database.host` | `localhost` | Host der Datenbank |
| `database.port` | `5432` | Port der Datenbank im Netz des Servers |
| `database.name` | `bothy` | Datenbankname |
| `database.user` | `bothy_app` | Rolle des Servers |
| `database.password_ref` | `file:<secrets.dir>/db_app_password` | Verweis auf das Passwort |
| `database.sslmode` | `verify-full` | TLS zur Datenbank; im internen Compose-Netz `disable` |
| `database.max_conns` | `20` | Größe des Verbindungspools je Prozess (Startwert bis zum Lasttest) |
| `database.statement_timeout` | `30s` | Abbruch einzelner Abfragen |
| `migrate.require_recent_backup` | `true` | `migrate` verlangt eine frische Sicherung |
| `migrate.recent_backup_max_age` | `24h` | so alt darf diese Sicherung sein |
| `migrate.lock_timeout` | `5s` | `lock_timeout` je Migrationsschritt |
| `backup.pg.full_schedule` | `0 2 * * 0` | Plan der Vollsicherung (cron) |
| `backup.pg.diff_schedule` | `0 2 * * 1-6` | Plan der Differenzsicherung (cron) |
| `backup.pg.archive_timeout` | `60s` | spätestens dann wird ein WAL-Segment archiviert |
| `backup.pg.retention_full` | `4` | behaltene Vollsicherungen in repo2 |
| `backup.pg.local_retention_full` | `1` | behaltene Vollsicherungen in repo1 |
| `backup.logical.schedule` | `0 3 * * *` | Plan von `pg_dump` (cron) |
| `backup.logical.retention` | `30d` | Aufbewahrung der logischen Sicherung |
| `backup.objects.schedule` | `30 3 * * *` | Plan der Objektsicherung (cron) |
| `backup.objects.retention` | `90d` | Aufbewahrung der Objekt-Snapshots |
| `backup.offsite.target` | leer | Off-site-Ziel; leer heißt nur lokal (Warnung in Konsole und Status) |
| `backup.max_age` | `26h` | ohne erfolgreiche Sicherung länger als das: Alarm |
| `backup.restore_test.schedule` | `0 5 * * 6` | Plan des Wiederherstellungstests (cron) |
| `backup.restore_test.sample_files` | `200` | geprüfte Dateien je Test |
| `backup.restore_test.sample_threads` | `200` | geprüfte Threads je Test |
| `backup.restore_test.max_age` | `8d` | ohne erfolgreichen Test länger als das: Alarm |
| `dr.rpo_target` | `5m` | Ziel für den größten Datenverlust |
| `dr.rto_target` | `4h` | Ziel für die Dauer der Wiederherstellung |
| `status.check_interval` | `30s` | Abstand der automatischen Prüfungen |
| `status.degrade_after` | `2m` | so lange Fehler bis `degraded` |
| `status.incident_after` | `5m` | so lange `major_outage` bis zur automatischen Störung |
| `status.resolve_after` | `10m` | so lange gesund bis zur automatischen Auflösung |
| `status.error_ratio_degraded` | `0.05` | Fehleranteil, ab dem eine Prüfung scheitert |
| `status.error_ratio_outage` | `0.5` | Fehleranteil, ab dem `major_outage` gilt |
| `status.fanout_lag_degraded` | `5s` | Fan-out-Verzug, ab dem `sync.fanout` scheitert |

## 15. Entscheidungen dieses Kapitels

| Nr | Entscheidung | Grund |
|---|---|---|
| R-OP-01 | Zwei Listener: öffentlich und intern. | Health und interne Webhooks brauchen kein Header-Vertrauen und keinen öffentlichen Weg. |
| R-OP-02 | Header-Identität nur mit öffentlichem Listener auf Loopback; sonst bleibt der Anbieter aus. | Erzwingt die Bedingung aus B9 statt sie anzunehmen. |
| R-OP-03 | Tailscale als Sidecar mit geteiltem Netz-Namespace und Kernel-Netz (kein Userspace-Modus), statt `tailscale serve` auf dem Host; dazu Tailnet-Richtlinie nur Port 443. | Nur so lauscht der Server im Container nachweisbar nur auf Loopback; ob der Userspace-Modus an Loopback weiterreicht, ist nicht dokumentiert. |
| R-OP-04 | Unbekannte Konfigurationsschlüssel brechen den Start ab. | Ein Tippfehler darf nie still einen Standard setzen. |
| R-OP-05 | Geheimnisse als Dateien je Dienst (`subpath`), nie als Umgebungsvariablen. | Kindprozesse, Absturzberichte und `docker inspect` zeigen keine Werte; jeder Dienst sieht nur seine Geheimnisse. |
| R-OP-06 | pgBackRest für physische Sicherung und PITR, `pg_dump` für Portabilität, restic für Objekte. | Bewährte Werkzeuge mit Prüfsummen, Verschlüsselung und Aufbewahrung; kein Eigenbau. |
| R-OP-07 | Eigenes Image `bothy-postgres` (postgres:18 plus pgBackRest und `bothy-backupd`). | `archive_command` läuft im Datenbank-Container; ein Image für beide Rollen hält Versionen gleich. |
| R-OP-08 | Repository-Host `backup` per pgBackRest-TLS; der Datenbank-Container bleibt ohne Ausgang. | Off-site-Zugangsdaten liegen nur dort, wo sie gebraucht werden. |
| R-OP-09 | Off-site-Sicherungen nur anhängend; Löschen nur mit getrenntem Konto. | Ein übernommener Server darf seine Sicherungen nicht zerstören. |
| R-OP-10 | Automatischer Wiederherstellungstest mit Manifest-Vergleich, Sequenz- und Hash-Prüfung, Stichproben. | Eine Sicherung ist erst belegt, wenn sie zurückgespielt wurde. |
| R-OP-11 | Wiederherstellung setzt eine neue Sync-Epoche (Ergänzung beantragt). | Ohne Epoche überholt die neue Zeitlinie alte Cursor still. |
| R-OP-12 | Expand/Contract mit Rückweg um genau eine Version; keine Down-Migrationen in Produktion. | Rückweg ohne Datenbank-Rückbau; zwei Versionen zurück braucht eine Wiederherstellung. |
| R-OP-13 | `migrate` verlangt eine frische Sicherung. | Erzwingt den wichtigsten Schritt vor jeder Schemaänderung. |
| R-OP-14 | Feature-Flags mit Art `release` und `remove_by`, Gate-Check; Flags nie für Sicherheitsgrenzen. | Verhindert tote Flags und versteckte Sicherheitsschalter. |
| R-OP-15 | Statusseite im Server, automatische Zustände aus internen Prüfungen, nicht aus Prometheus. | Funktioniert ohne optionalen Observability-Stack. |
| R-OP-16 | Produktionsdaten nie in `dev` oder `staging`. | Kein Anonymisierungswerkzeug; ein Abzug wäre ein Datenschutzvorfall. |
