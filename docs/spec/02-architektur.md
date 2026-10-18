# 02 Architektur

Status: Entwurf M0. Normativ für den Aufbau von `services/server/`.
Begriffe: [00-glossar.md](00-glossar.md). Tabellen: [03-datenmodell.md](03-datenmodell.md).
Ereignisse und Sync: [04-sync-protokoll.md](04-sync-protokoll.md). Aktionen:
[05-aktionen.md](05-aktionen.md). Entscheidungen: ADR 0004 bis 0009, siehe
[ADR-Index](../adr/README.md).
Diagramme: [../architecture/README.md](../architecture/README.md).

Schreibweise: REST-Pfade stehen ohne Versions-Präfix; das Präfix legt [06-api.md](06-api.md) fest.
Grenzen, Fristen und Ziele stehen nur als Name im Text. Standardwerte dieses Kapitels stehen in
Abschnitt 22, alle anderen in dem Kapitel, das den Namen führt.

## 1. Leitplanken

| Leitplanke | Herkunft | Folge für die Architektur |
|---|---|---|
| Eine Instanz läuft zuerst auf einem Rechner, später in einer Cloud. | B1, B12 | Ein Binary mit Rollen. PostgreSQL und Objektspeicher sind der einzige Zustand. Kein Broker, kein Cache-Dienst, kein Service-Mesh. |
| Rechte gelten bei jeder Anfrage und jeder Zustellung. | B7, 05 | Eine Prüfstelle `authorize` im Modul `authz`. Jede Route weist nach, dass sie ihre Aktion geprüft hat (Abschnitt 9.3). |
| Kein Ereignis geht verloren, keine Schreibanfrage wirkt doppelt. | B8, 04 | Commit-geordnetes `event_log`, Cursor, Idempotenz in derselben Transaktion (ADR 0009). |
| Agenten, Plugins und Apps laufen nie im Server-Prozess. | B6, B10 | Die öffentliche API ist die einzige Erweiterungsstelle. Der Server kennt Agenten nur als Prinzipale, Läufe als Objekte. |
| Infrastruktur ist austauschbar. | B3, B4, B9, B11, B12 | Hexagonale Module: Fachlogik kennt nur Ports, Treiber sind Adapter (ADR 0008). |
| Betrieb durch eine Person muss reichen. | B1 | Wenige Teile, ein Konfigurationsweg, Fehlerbilder mit klarer Reaktion (Abschnitt 14). |

## 2. Kontext (C4 Ebene 1)

Diagramm: [c4-1-kontext.svg](../architecture/c4-1-kontext.svg)
(Quelle [c4-1-kontext.mmd](../architecture/c4-1-kontext.mmd)).

| Beteiligter | Art | Schnittstelle zur Instanz | Vertrauen |
|---|---|---|---|
| Mitglied | Mensch | Apps über REST und Gateway-WebSocket | nur eigene Freigaben |
| Owner | Mensch | Apps und Web-Konsole, Verwaltung mit Step-up | alles außer fremde Direktnachrichten (05, R-AZ-02) |
| Betreiber | Mensch | Container, Konfiguration, Server-Befehle `bothy ...` | Host-Zugang; kein Zugang über die API ohne eigene Freigaben |
| Bot-Entwicklerin | Mensch | öffentliche API, SDK, App-Manifest | keines; ihre Apps bekommen Scopes und Freigaben |
| Identitätsanbieter | extern | OIDC (Auth-Code mit PKCE), JWKS des Anbieters | für die Identität, nach Signaturprüfung |
| Reverse-Proxy | extern, beim Betreiber | HTTPS, WebSocket, optional Identitäts-Header | Header nur, wenn der Server ausschließlich auf Loopback lauscht (B9) |
| Push-Dienste | extern | HTTPS vom worker | nicht vertraut; bekommen nur IDs und Platzhalter |
| Agenten (Adapter mit Laufzeit) | extern zum Server | Gateway als Agenten-Prinzipal, REST, Aufruf-Marke | nicht über ihre Freigaben hinaus; prüfen selbst die Marke (I3) |
| KI-Anbieter | extern | nur von der Laufzeit genutzt, nie vom Server | Server speichert keine Zugangsdaten (B10, I5) |
| Integrationen | extern | REST mit App-Token, Eingangs-Webhooks, ausgehende Webhooks | Scopes und Freigaben (B6) |
| Telemetrie-Backends | extern | OTLP über einen Collector | bekommen keine Inhalte (B11) |
| Discord | extern | nur lesend beim Import | Quelle, keine Rückschreibung |
| Transkriptions-Dienst | extern, optional | Port im worker | bekommt Audio nur bei eingeschaltetem Schalter `voice.enabled` |
| Host-Dienst `bothy-host` | extern, auf angebundenen Rechnern | nur ausgehend: WebSocket-Steuerverbindung und je Sitzung eine Datenverbindung ([24-terminal.md](24-terminal.md)) | angemeldet mit eigenem Schlüssel; der Server sieht keinen Terminal-Inhalt (Noise) |

## 3. Container (C4 Ebene 2)

Diagramm: [c4-2-container.svg](../architecture/c4-2-container.svg)
(Quelle [c4-2-container.mmd](../architecture/c4-2-container.mmd)).

### 3.1 Container

| Container | Technik | Verantwortung | Zustand | Skalierung |
|---|---|---|---|---|
| `bothy serve --role=api` | Go, ein Binary, OCI-Image | REST, Gateway-WebSocket, Log-Tail und Fan-out im Prozess, Agenten-Verbraucher | nur flüchtig im Speicher (Verbindungen, Rechte-Cache, Rate-Limit-Eimer) | waagerecht ab Stufe 2 (Abschnitt 16) |
| `bothy serve --role=worker` | gleiches Binary und Image | River-Jobs, Ereignis-Verbraucher (Push, Webhooks), Dateiverarbeitung, Wartung | keiner außerhalb der Datenbank | waagerecht ab Stufe 3 |
| PostgreSQL 18 | Datenbank | alle Domänendaten, `event_log`, Audit, River-Tabellen im Schema `jobs.schema` | maßgeblich | senkrecht; Lese-Replikat ab Stufe 4 |
| Objektspeicher | Dateisystem-Treiber (Standard) oder S3-kompatibel | Dateien, Vorschaubilder, Exportpakete | maßgeblich für Bytes | über den Speicherdienst |
| ClamAV `clamd` | eigener Prozess, optional | Malware-Scan per `INSTREAM` | Signaturen | eigener Container |
| OTel Collector | eigener Prozess | nimmt OTLP an, exportiert je Backend | Puffer | eigener Container |
| Apps | Flutter (iOS, Android, macOS, Windows) | Chat, Ausgangsfach, lokale Datenbank, Notification Service Extension | lokaler Cache, nie maßgeblich | je Gerät |
| Web-Konsole | Flutter Web | Verwaltung, Statistik und Terminal (B5, [ADR 0070](../adr/0070-terminal-in-der-web-konsole.md)); kein Chat | keiner | static files in the server image, served by the api process under `/console/` from `console.assets_dir` ([16-betrieb.md](16-betrieb.md) 14; M6-02) |
| Agenten-Adapter | außerhalb des Servers (Referenz: Hermes-Agent-Plugin, Claude-Code-Runner) | Verbraucher mit dauerhaftem Cursor, prüft Aufruf-Marke, meldet Läufe | eigener | beim Betreiber der Laufzeit |
| Host-Dienst `bothy-host` | eigenes Go-Binary (`services/host`), auf jedem angebundenen Rechner | PTY je Terminal-Sitzung, Noise-Gegenstelle, Profile, Selbstprüfung ([24-terminal.md](24-terminal.md)) | Schlüssel und Rückblick-Puffer auf dem Rechner | je Rechner |

Die Secret-Quelle (Datei, Umgebung, später Key Vault oder Secrets Manager) ist kein eigener Container
im Diagramm. api und worker lesen sie beim Start und bei Rotation (Abschnitt 13).

### 3.2 Rollen eines Prozesses

| Teil | `api` | `worker` |
|---|---|---|
| REST-Endpunkte, Gateway-WebSocket | ja | nein |
| Log-Tail, Hub, Rechte-Cache für Verbindungen | ja | nein |
| Agenten-Verbraucher (Gateway-Verbindungen von Adaptern) | ja | nein |
| Ereignis-Verbraucher `push` und `webhooks` | nein | ja |
| River: Jobs einfügen | ja | ja |
| River: Jobs ausführen, periodische Jobs, Leader-Wahl | nein | ja |
| interner Listener `server.internal_listen_addr` (Health, Eingangs-Webhooks aus dem Stack) | ja | ja |

Rolle `all` vereint beide in einem Prozess und ist der Standard (`server.role`). Grund: eine
Installation auf einem Rechner braucht genau einen Server-Container. Die Trennung kostet nur eine
Zeile Konfiguration, weil beide Rollen dasselbe Binary und dieselbe Datenbank nutzen.

### 3.3 Befehle des Binarys

Ein Binary `bothy` mit Unterbefehlen (Ruling L1). Diese Tabelle ist das Register; andere Kapitel
nennen nur Befehle von hier.

| Befehl | Zweck | Kapitel |
|---|---|---|
| `bothy serve --role=api\|worker\|all` | Server starten | dieses Kapitel |
| `bothy migrate up [--bootstrap]\|status\|validate` | Migrations of the domain schema (goose) and of the River tables (rivermigrate); on the first run creates the `instance` row (through the port of module `instance`), sets `instance.schema_version` and `instance.min_server_version`; creates the first partitions of `event_log` (03). `up --bootstrap` first creates or updates the four database roles and their grants as `migrate.bootstrap_user` (16 "Rollen"); `status` only reads, `validate` needs no database | ADR 0007, [16-betrieb.md](16-betrieb.md) |
| `bothy setup bootstrap-owner\|issue-enrollment-code` | Einrichtung: erster Owner per Server-Befehl (Systemaktion `instance.bootstrap`), Einladungs-Code zur Wiederherstellung | [08-identitaet.md](08-identitaet.md) |
| `bothy config check\|print` | Konfiguration prüfen, redigiert ausgeben; `check --secrets` löst zusätzlich jeden Verweis `*_ref` über den Secret-Provider auf und nennt je Schlüssel nur `ok` oder den Fehlercode | Abschnitt 13 |
| `bothy import login\|logout`, `bothy import discord ...` | login of the import tool as a `cli` device through device pairing; import and read mirror through the import API (`import.read`, `import.apply`) | [22-import-migration.md](22-import-migration.md) |
| `bothy restore finalize` | nach einer Wiederherstellung: neue Sync-Epoche, Cursor auf `head_seq`, Audit | [16-betrieb.md](16-betrieb.md), [03-datenmodell.md](03-datenmodell.md) 6.7 |
| `bothy secrets init\|export` | Geheimnisse anlegen und sichern: `init --target DIR [--source DIR] [--owner DIENST=UID:GID ...] [--adopt-source]` gleicht die Ablage `_store/` mit der Quelle ab und kopiert je Dienst; `export --from DIR --age-recipient SCHLÜSSEL [--age-recipient ...] [--signing-key-dir DIR] [--output DATEI\|-] [--armor]` schreibt das age-verschlüsselte Bündel (mit dem Schlüsselspeicher unter `signing-keys/`); beide lesen nur ihre Flags, ohne Konfigurationsdatei und Datenbank | [16-betrieb.md](16-betrieb.md) 5.3, 5.4 |
| `bothy healthcheck --url URL [--timeout DURATION]` | Gesundheitsprüfung für Container ohne Shell gegen den internen Listener. `--url` takes only `http://` with a loopback host (`127.0.0.0/8`, `::1`, `localhost`), anything else is exit `2`; `--timeout` bounds the whole request (default `5s`, the timeout of the Compose healthcheck); no configuration, secrets or database; exit `0` for `2xx`, else `1` with one line naming the status code or the error class | [16-betrieb.md](16-betrieb.md) |
| `bothy files rescan` | Wartung: Dateien erneut prüfen lassen | [12-dateien.md](12-dateien.md) |
| `bothy telemetry pseudonym <principal_id> [--at TIME] [--config FILE]` | Pseudonym of a principal for the owner's troubleshooting: prints `bothy.principal.pseudo_id` for the period that contains `--at` (RFC 3339, default now) and the start of that period in UTC; reads `telemetry.pseudonym_key_ref` through the secret provider, no database | [../observability/privacy.md](../observability/privacy.md) |
| `bothy dev seed` | Testdaten, nur in `instance.environment = dev` | [../operations/environments-and-flags.md](../operations/environments-and-flags.md) |

Exitcodes aller Befehle: `0` Erfolg bzw. geordnetes Ende; `1` Fehler (Start gescheitert, fataler
Laufzeitfehler, Shutdown über `shutdown.total_timeout`, Prüfung negativ); `2` Aufruf- oder
Konfigurationsfehler. Ab `3` sind Codes je Befehl und stehen in seiner Hilfe: `serve` `3` für ein
erzwungenes Ende durch ein zweites Signal (Abschnitt 15.3); `setup bootstrap-owner` `3`, wenn die
Einrichtung schon abgeschlossen ist; `setup issue-enrollment-code` `4`, wenn sie es noch nicht ist
([08-identitaet.md](08-identitaet.md)). Grund: Orchestrator und Skripte unterscheiden "falsch
aufgerufen" von "zur Laufzeit gescheitert" und erkennen erwartete Zustände ohne Textvergleich.
Ausgaben: Was ein Skript lesen soll (z. B. der Einladungs-Code von `setup`), steht allein auf der
Standardausgabe; Logs gehen auf die Standardfehlerausgabe.

Migrationen laufen nie implizit beim Start von `serve` (R-AR-16). Grund: Bei mehreren Knoten würden
sie gleichzeitig starten, und ein fehlgeschlagener Schritt soll den Betrieb nicht in einer
Neustart-Schleife halten.

## 4. Module des Monolithen

Diagramm: [c4-3-komponenten-server.svg](../architecture/c4-3-komponenten-server.svg)
(Quelle [c4-3-komponenten-server.mmd](../architecture/c4-3-komponenten-server.mmd)).

### 4.1 Regeln

1. Ein Modul ist der Paketbaum `services/server/internal/<modul>/`. Es besitzt Tabellen. Nur
   dieses Modul schreibt und liest sie per SQL (03, Grundsatz 12). Andere Module gehen über seinen
   Port.
2. Der Port ist das Wurzelpaket `internal/<modul>` (Abschnitt 5). Andere Module importieren nur
   dieses Paket, nie `domain`, `app` oder `adapters` eines fremden Moduls.
3. Abhängigkeiten zwischen Modulen bilden einen gerichteten Graphen ohne Zyklen (Tabelle 4.4).
4. Braucht ein tieferes Modul Daten oder Reaktionen eines höheren, gibt es genau zwei Wege:
   - **Quellen-Port:** Das tiefere Modul deklariert eine Schnittstelle in seinem Port; das höhere
     Modul implementiert sie in einem Adapter. Beispiel: `authz` deklariert `StructureSource`,
     `chat` implementiert sie.
   - **In-Tx-Ereignis:** Das tiefere Modul veröffentlicht ein Domänenereignis in der laufenden
     Transaktion; das höhere Modul abonniert es (Abschnitt 8.3). Beispiel: `messaging`
     veröffentlicht `MessagePosted`, `runs` entscheidet daraufhin über einen Lauf.
5. Querschnitts-Ports (`internal/kernel`) darf jedes Modul nutzen. Sie sind klein, stabil und je
   von genau einem Modul implementiert (Tabelle 4.5).
6. Jede Tabelle aus 03 gehört genau einem Modul. Jede Aktion aus 05 wird von genau einem Modul als
   Anwendungsfall ausgeführt. Die Rechte-Logik jeder Aktion liegt trotzdem nur in `authz` (B7).
7. Zwei Module besitzen keine Tabellen: `gateway`, `search` (03, Abschnitt 2; Ruling L3). Sie
   sind eigene Module, weil sie eigene Ports, eigene Abhängigkeiten und eigene Leistungsziele haben.
   `stats` war ebenfalls ohne Tabellen geplant und besitzt seit dem Nachtrag vom 2026-10-10 die
   Minuten-Aggregate der eigenen Messung (`stats_minutes`, Ruling T7,
   [25-visualisierung.md](25-visualisierung.md)).

### 4.2 Modulliste

| Modul | Verantwortung | Besitzt Tabellen | Kapitel |
|---|---|---|---|
| `instance` | Instanz-Identität, Sync-Epoche, Mindest-Server-Version, Einstellungen und Branding, Feature-Flags, Server-Signierschlüssel mit JWKS und Rotation, Sequenz-Zähler | `instance`, `instance_settings`, `server_signing_keys`, `feature_flags`, `sequence_counters` | 06, 16, 19 |
| `audit` | Audit-Log mit Hash-Kette, signierte Prüfpunkte, Schlüssel der IP-Pseudonyme, Abfrage, Prüfung, Kürzung mit Anker | `audit_events`, `audit_checkpoints`, `audit_ip_keys` | 19, 20 |
| `authz` | `authorize`, Rechte-Cache, sichtbare Ziele, Gruppen, Freigaben, Rechte-Version, I1-Prüfung, "Wer hat Zugriff", "Als Person ansehen" | `groups`, `group_memberships`, `grants` | 07 |
| `billing` | Preistabelle, Kostenbuchungen, Summen je Lauf, Thread, Bereich, Agent, Abgleich | `price_table`, `cost_entries` | 10 |
| `identity` | Prinzipale, Menschen, Owner, Anmelde-Treiber, Identitäten, lokale Anmeldedaten, Einladungs-Codes, Anmeldevorgänge, Geräte-Kopplung, Passkeys, Geräte und Geräteschlüssel, Sitzungen, Refresh-Familien, API-Tokens, Step-up, Zugriffstokens | `principals`, `users`, `owners`, `identity_providers`, `user_identities`, `local_credentials`, `enrollment_codes`, `auth_login_flows`, `auth_challenges`, `device_pairings`, `passkey_credentials`, `devices`, `device_keys`, `auth_sessions`, `refresh_token_families`, `refresh_tokens`, `api_tokens` | 08 |
| `chat` | Bereiche, Seitenleisten-Zustand, Kanäle mit Verhalten, Skills, Projektbindung, DM-Teilnehmer, Forum-Tags, Threads mit Thread-Sequenz | `spaces`, `user_space_states`, `channels`, `channel_skill_bindings`, `project_bindings`, `dm_participants`, `forum_tags`, `threads`, `thread_tags` | 01, 07, 09 |
| `agents` | Agenten, Backend-Richtlinie und Werkzeugklasse (Soll), gemeldeter Ist-Zustand, Verbraucher-Cursor mit Fencing, Befehlsregister, Agenten-Sitzungen, Dev-Umgebungen (Etappe) | `agents`, `agent_consumer_cursors`, `commands`, `agent_sessions`, `dev_environments` | 09, 10, 21 |
| `files` | Upload-Sitzungen in Teilen, Objektspeicher-Treiber, Scan, Metadaten-Entfernung, Vorschaubilder, Anhänge, Transkripte, Dateitext | `files`, `file_upload_sessions`, `file_upload_parts`, `file_thumbnails`, `message_attachments`, `file_text_extracts` | 12, 14 |
| `messaging` | Nachrichten, Revisionen, Erwähnungen, Reaktionen, Anheften, Vorlagen, Kanal-Export | `messages`, `message_revisions`, `mentions`, `reactions`, `pins`, `message_templates` | 06, 14 |
| `runs` | Auslöse-Entscheidung, Läufe und Lauf-Dokument, Schritte, Nachträge, Komponenten, Interaktionen, Befehlsausführung, Aufruf-Marken | `runs`, `run_steps`, `run_followups`, `message_components`, `interactions` | 09, 10 |
| `sync` | Ereignis-Log (Anhängen, Tail, Nachholen, Schwärzen, Partitionen), Momentaufnahme, Idempotenz-Schlüssel, Gelesen-Stand, Entwürfe, persönliche Einstellungen, Cursor der Worker-Verbraucher | `event_log`, `event_consumer_cursors`, `idempotency_keys`, `read_states`, `drafts`, `user_settings` | 04, 15 |
| `notify` | Benachrichtigungsstufen, Ruhezeiten, Push-Registrierungen, Push-Verbraucher und Versand | `notification_preferences`, `quiet_hours`, `push_registrations` | 13 |
| `gateway` | WebSocket-Endpunkt, Gateway-Sitzungen, Hub, Fokus, Tippen, Weitergabe kurzlebiger Ereignisse, Agenten-Verbindungen | keine | 04, 06 |
| `search` | Suchabfrage über Nachrichten, Thread-Titel und Dateien mit Rechtefilter, Rangfolge | keine | 14 |
| `stats` | Statistik für Konsole und Apps: Nutzung je Bereich, Läufe, Werkzeugaufrufe, Kosten, Fehlerraten; eigene Messung (Zähler im Speicher, Flush, Aufbewahrung), SLO-Ampel, Live-Rahmen `stats.live`, Timeline | `stats_minutes` | 01, 17, 25 |
| `apps` | Apps und Manifeste, Installationen, Scopes, ausgehende Webhooks mit Verbraucher und Zustellung, Eingangs-Webhooks | `apps`, `app_installations`, `webhook_subscriptions`, `webhook_deliveries`, `incoming_webhooks` | 11 |
| `importer` | Import-Quellen, Stapel mit Herkunftsschlüssel (`import.apply`), Eigentumsliste und Schreibschutz gespiegelter Kanäle; der Trockenlauf liegt im Werkzeug | `import_sources`, `import_items` | 22 |
| `privacy` | DSGVO-Aufträge, Export und Löschung über die Teilnehmer aller Module | `gdpr_requests` | 20 |
| `status` | Statusseite, Komponenten, Störungen, automatische Prüfungen, Sicherungsläufe | `status_components`, `status_incidents`, `status_incident_updates`, `status_incident_components`, `backup_runs` | 16, 17 |
| `terminal` | Rechner und Kopplung, Erklärung der LLM-Anmeldung (I6), Sitzungsbenutzer je Person (T12), Terminal-Sitzungen, Sitzungs-Marken, Steuerverbindungen der Host-Dienste, Relais der Ströme | `hosts`, `host_session_users`, `terminal_sessions` | 24 |

Technische Tabellen außerhalb der Module ([03-datenmodell.md](03-datenmodell.md), Abschnitt
4.13): River legt eigene Tabellen mit Präfix `river_` an (u. a. `river_migration`, `river_job`,
`river_leader`, `river_queue`) [Q6, belegt]. Sie liegen im Schema `jobs.schema` und gehören
`internal/platform/jobs`. Die Versionstabelle von goose liegt im selben Schema wie die
Domänentabellen.

### 4.3 Ports, Ereignisse und Quellen je Modul

"Port" nennt die eingehenden Schnittstellen im Wurzelpaket (Auszug, Namen sind Vorschläge für den
Bau). "Gibt aus" sind In-Tx-Ereignisse (Go-Typen, nie serialisiert). "Quellen" sind Schnittstellen
tieferer Module, die das Modul implementiert.

| Modul | Port (Auszug) | Gibt aus (In-Tx) | Abonniert (In-Tx) | Implementiert Quellen |
|---|---|---|---|---|
| `instance` | `Settings`, `Flags`, `Keys` (Signer je Zweck, `Signers` and `PublicKeys` of a purpose for the outgoing webhooks, JWKS, Rotation), `Counters` (nur für die Phasen-Hooks von `authz`, `sync`, `audit`), `Versions` (Zählerstände ohne Sperre lesen, z. B. `permission_version` für den Rechte-Cache, `event_seq` für Catch-up); deklariert `LoginOptionsSource` (Anmelde-Treiber und Passwortweg für die Discovery), `FlagUserSource` (Menschen eines Rollouts), `FlagGroupSource` (gültige Gruppen eines Menschen für die Auswertung) | | | |
| `audit` | `Log` (Abfrage, Prüfung) | | | kernel `auditing.Recorder` |
| `authz` | `Admin` (Gruppen, Freigaben, Abgleich der Claim-Gruppen, Erklärung, Vorschau, Ansicht als Person, Widerruf aller Freigaben eines Subjekts in der Rechte-Änderung des Aufrufers), `AppScopes` (Scope-Tabelle für die Ereignis-Abos einer App); deklariert `PrincipalSource`, `CredentialSource`, `StructureSource`, `AgentPolicySource`, `HostSource` | kernel `authorize.AccessChanged` (mit der auslösenden Aktion; `runs` bricht bei `user.disable` mit Grund `user_disabled` ab, sonst mit `access_revoked`) | | kernel `authorize.Authorizer`, `instance.FlagGroupSource` |
| `billing` | `Costs` (Buchen, Korrektur, Summen), `Prices` | | | |
| `identity` | `Auth` (Anmeldung, Refresh, Logout, Kind-Sitzungen, Step-up), `Directory`, `Devices`, `Tokens`, `Owners`, `PrincipalWrites` (Prinzipale von Agenten und Apps); deklariert `AvatarFiles`, `AppInstallations` (aktive Installation einer App) | `DeviceRevoked`, `SessionEnded` | | kernel `actor.Authenticator`, `authz.PrincipalSource`, `authz.CredentialSource` (Sitzung, Gerät, Token für die Prüfung `check_active`, 07 Abschnitt 4.2), `instance.LoginOptionsSource`, `instance.FlagUserSource`, kernel `personaldata.Participant` |
| `chat` | `Structure` (Bereiche, Kanäle, Schreibschutz), `Threads` (Anlegen für `messaging`, `AppendToThread`, Nachrichtenzahl bei Löschung, Schließen, Öffnen, Verschieben, Info), `AgentConfig` | `ThreadMoved`, `ChannelArchived`, `ChannelReadOnlySet`, `SpaceArchived` | | `authz.StructureSource`, kernel `personaldata.Participant` |
| `agents` | `Agents` (Verwaltung, Richtlinie, Werkzeugklasse), `Presence`, `Consumers` (Cursor, Bestätigung, Übernahme mit Fencing), `Commands` (Register), `Sessions`, `DevEnvironments` (Etappe); declares `ConsumerFeed` (the stream of the REST consumer operations, M2-07) | `AgentDisabled` | | `authz.AgentPolicySource` (Richtlinie, Werkzeugklasse, Bereitschaft der Dev-Umgebung), kernel `personaldata.Participant` |
| `files` | `Uploads` (Sitzungen, Teile), `Files` (Lesen, Öffnen, Vorschau, Abruf-Tickets, Löschen), `Attachments`, `Transcripts`, `TextExtracts`; deklariert `AttachmentLocator` | `FileFinalized` (Scan und gegebenenfalls Transkript endgültig) | | `identity.AvatarFiles`, kernel `personaldata.Participant` |
| `messaging` | `Messages` (Anlegen, Thread mit erster Nachricht anlegen, Bearbeiten, Löschen, Verlauf, `SetTriggerState`, Anker), `Reactions`, `Pins`, `Templates`, `Mentions` (Liste, ungelesene je Thread), `Export`; deklariert `ComponentReader` | `MessagePosted`, `MessageDeleted` | `chat.ThreadMoved` | `files.AttachmentLocator`, kernel `personaldata.Participant` |
| `runs` | `Runs` (Lesen, Kontext, Start, Wiederaufnahme, Patch, Ende, Abbruch, Übersicht), `Delivery` (Zustellvorbereitung mit I2, Schutzregeln und Marke), `Interactions`, `CommandExec` (inklusive Autovervollständigung) | | `messaging.MessagePosted`, `messaging.MessageDeleted`, `files.FileFinalized` (zurückgehaltenen Aufruf schreiben), `agents.AgentDisabled`, `chat.ChannelReadOnlySet`, kernel `authorize.AccessChanged` | `messaging.ComponentReader`, kernel `personaldata.Participant` |
| `sync` | `Tail`, `CatchUp`, `Snapshot`, `Consumers` (Worker-Cursor, with `ConsumerLossHandler` for a retention loss), `LogReader` (one event by `seq` and the smallest retained `seq` for delivery jobs), `ReadStates`, `Drafts`, `Settings` | | `messaging.MessagePosted` (Entwurf leeren), `chat.ThreadMoved` (Kopien von `channel_id`) | kernel `eventlog.Appender`, kernel `eventlog.Redactor`, kernel `idempotency.Store`, kernel `personaldata.Participant` |
| `notify` | `Preferences`, `QuietHours`, `PushRegistrations` | | `identity.DeviceRevoked`, `identity.SessionEnded` (Registrierungen der Sitzung deaktivieren), `chat.ChannelArchived`, `chat.SpaceArchived` | kernel `personaldata.Participant` |
| `gateway` | keiner für andere Module | | | kernel `eventlog.EphemeralPublisher`; Implements sources: `agents.ConsumerFeed` (M2-07) |
| `search` | `Search` | | | |
| `stats` | `Stats` (Abfragen, Timeline, Ampel, Backend-Ansicht), `Live` (Basis-Rahmen je Intervall, Filter je Empfänger) | | | kernel `measure.Recorder` |
| `apps` | `Apps`, `Webhooks` (`apps.Webhooks`: subscriptions, deliveries, redelivery, `ResumeInstallation`), `IncomingWebhooks` | | `chat.ChannelArchived` | `identity.AppInstallations` |
| `importer` | `Imports` (Quellen, Stapel über `import.apply`) | | | |
| `privacy` | `Requests` | | | sammelt alle kernel `personaldata.Participant` |
| `status` | `Status`, `BackupRuns` (für `bothy-backupd`) | | `agents.AgentDisabled` | |
| `terminal` | `Hosts` (Verwaltung, Kopplung, Erklärung), `Sessions` (Öffnen, Anhängen, Schließen, Liste), `Relay` (Strom- und Host-Verbindungen, nur im Adapter `wsrelay`) | | kernel `authorize.AccessChanged`, `identity.DeviceRevoked`, `identity.SessionEnded` (Sitzungen beenden) | `authz.HostSource` (Zustand, Inhaber, LLM-Erklärung, Server-Host des Rechners) |

### 4.4 Erlaubte Abhängigkeiten

"Darf nutzen" heißt: Code dieses Moduls darf den Port des genannten Moduls importieren. Alles
andere ist verboten und schlägt im Gate fehl (Abschnitt 7).

| Modul | Darf nutzen | Grund je Kante |
|---|---|---|
| `instance` | nichts | Fundament |
| `billing` | nichts | Kostenzeilen tragen Kopien von Thread, Kanal, Bereich und Agent (03); keine Rückfrage nötig |
| `audit` | `instance` | Zähler `audit_seq` |
| `authz` | `instance` | Zähler `permission_version`; alle anderen Daten über Quellen-Ports |
| `identity` | `instance`, `authz` | Signierschlüssel für Zugriffstokens; implementiert `PrincipalSource` und `CredentialSource` |
| `chat` | `identity`, `authz` | Bediener ist aktiver Agent-Prinzipal, DM-Teilnehmer sind Menschen; implementiert `StructureSource` |
| `agents` | `identity`, `authz`, `chat` | Agent-Prinzipal anlegen, Zugangsdaten-Inhaber ist Mensch; implementiert `AgentPolicySource`; Thread für Sitzungen, Geltung von Befehlen |
| `files` | `identity`, `chat` | implementiert `AvatarFiles`; Upload-Kanal prüfen |
| `messaging` | `identity`, `chat`, `files` | Erwähnungen auflösen; Thread-Sequenz und Wiederöffnen; Anhänge binden und lesen |
| `status` | `agents` | Agenten-Präsenz als Status-Komponente |
| `runs` | `instance`, `chat`, `agents`, `messaging`, `billing` | Marke signieren; Bediener, Erwähnungsregel, Projektbindung; Richtlinie und Sitzung; Anker und `trigger_state`; Kosten buchen |
| `search` | `chat`, `messaging`, `files` | Thread-Titel, Nachrichten, Dateinamen und Transkripte |
| `importer` | `identity`, `chat`, `agents`, `messaging`, `files` | legt Menschen, Struktur, Nachrichten, frühere Sitzungs-Referenzen und Dateien an; setzt den Schreibschutz der gespiegelten Kanäle |
| `privacy` | `identity`, `files` | betroffene Person; Exportpaket speichern |
| `sync` | `instance`, `identity`, `chat`, `agents`, `messaging`, `runs` | Zähler `event_seq`; Prinzipal der Momentaufnahme (Handle, Anzeigename); Momentaufnahme aus Struktur, Präsenz, laufenden Läufen; Abo auf Nachrichten |
| `stats` | `chat`, `agents`, `messaging`, `runs`, `billing` | liest Kennzahlen über Ports |
| `notify` | `identity`, `chat`, `messaging`, `sync` | Geräte und Zeitzone; Ziel-Pfad für Einstellungen; Erwähnungen; Gelesen-Stand |
| `apps` | `instance`, `identity`, `authz`, `chat`, `agents`, `messaging`, `sync` | Webhook-Signierschlüssel; App-Prinzipal und Tokens; Freigaben der App bei der Deinstallation widerrufen; Scope-Tabelle für Ereignis-Abos; Ziel der Eingangs-Webhooks; Befehle von Apps; Nachrichten aus Webhooks; Verbraucher für ausgehende Webhooks |
| `gateway` | `chat`, `agents`, `runs`, `sync`, `stats` | Thread zu Kanal für Fokus und Tippen; Verbraucher-Cursor und Präsenz; Lauf-Patches und Zustellvorbereitung; Tail, Nachholen, Momentaufnahme; Live-Rahmen für `stats.subscribe` |
| `terminal` | `instance`, `identity`, `authz`, `agents` | Signer für Sitzungs-Marken (Zweck `terminal`) und `instance_id` der Antwort von `pairHost`; Menschen, Geräte, Owner, Verbrauch des Step-up, Kopplungs-Codes (Format und HMAC über `identity`, M2-41); Freigaben auf Rechner und I6; Bereitschaft der Dev-Umgebung (Etappe) |

Ebenen (von unten): `instance`, `billing` · `audit`, `authz` · `identity` · `chat` · `agents`,
`files` · `messaging`, `status`, `terminal` · `runs`, `search`, `importer`, `privacy` · `sync`,
`stats` · `notify`, `apps`, `gateway`. Jede Kante zeigt nach unten.

Zusätzlich gilt für alle Module: `kernel` ja, `platform` nur in `adapters`, `api` (generierter Code)
nur in `adapters`, River nur in `platform/jobs`.

Quellen-Ports, deren Implementierung ein späteres Ticket baut, verdrahtet `internal/wire` bis
dahin mit einer leeren Quelle, die nichts kennt; `authorize` entscheidet dann fail closed (keine
Struktur, kein aktives Gerät). Das Ticket der Implementierung ersetzt die leere Quelle (PLAN,
Tickets der Module `identity`, `chat`, `agents`).

### 4.5 Querschnitts-Ports (`internal/kernel`)

| Paket | Zweck | Implementiert von | Genutzt von |
|---|---|---|---|
| `kernel/ids` | UUIDv7 erzeugen und prüfen (03, R-DM-01) | kernel selbst | allen |
| `kernel/clock` | Zeitquelle, in Tests ersetzbar | kernel selbst | allen |
| `kernel/errs` | Fehlerarten mit stabilem Code (Abschnitt 14.1) | kernel selbst | allen |
| `kernel/textcheck` | Prüfung von Anzeige-Texten nach SEC-IN-4 (ungültiges UTF-8, Steuerzeichen, Bidi-Steuerzeichen); keine Längenprüfung | kernel selbst | `domain`-Pakete aller Module mit Anzeige-Texten |
| `kernel/reqmeta` | `correlation_id` (Trace-ID der Anfrage, als UUID geschrieben, keine UUIDv7) und Client-IP der Anfrage im Kontext; nur Standardbibliothek | `platform/httpserver` setzt die Werte | `audit`, `sync`, `platform/*` |
| `kernel/jose` | Kompakte JWS lesen, bauen und prüfen (`ES256`, `EdDSA`, `RS256`, `PS256`), JWK lesen und Fingerabdruck nach RFC 7638; nur Standardbibliothek ([ADR 0067](../adr/0067-jose-profil-mit-der-standardbibliothek.md)) | kernel selbst | `identity` (DPoP, Zugriffstokens, Step-up, ID-Tokens, Client-Assertion), `instance` (JWKS, Prüfung von Prüfpunkten), `audit`, später `runs` |
| `kernel/designkeys` | erlaubte Schlüssel des Design-Systems als Daten: Kategorie-Farben, Akzente, Bereichs-Icons; erzeugt aus `design/build/tokens.resolved.json` (PLAN M1-46) | Generator | `instance` (Branding), `chat` (Bereiche, Tags) |
| `kernel/searchlang` | Validated language configuration of full-text search: `search.languages` and `search.locale_map`, the configuration of a locale's first language subtag, the query languages and every configuration the server uses ([14-suche.md](14-suche.md) 3.1, M3-13); standard library only | kernel selbst | `identity` (language of new messages and threads), `wire` (checks, start check), later `search` (M3-14) and `files` (M3-15) |
| `kernel/actor` | Prinzipal einer Anfrage (ID, Art, Gerät, Sitzung, Step-up-Zeit, Scopes); `Authenticator` mit `Revalidate` (geprüften Prinzipal ohne neuen Beweis erneut prüfen: Sitzung, Gerät, Token, Status; schließt Gateway-Verbindungen nach Widerruf von Tokens und Agenten, die kein `session.revoked` haben) | `identity` | `platform/httpserver`, `gateway` |
| `kernel/authorize` | Aktionen (generiert aus `docs/spec/actions.yaml`, [ADR 0049](../adr/0049-maschinenlesbarer-aktionskatalog-als-einzige-quelle.md)), Ressourcen-Deskriptor (mit optionalem zweitem Ziel, 07 Abschnitt 4.2), `Decision`, `Authorizer` (`Authorize`, `Scope` für Listen, `Observe` für Ereignisse des Logs), Filtermodus `Filter(ctx)` für Elemente von Listen, Suche und Momentaufnahme (keine Ablehnung im Audit, 07 Abschnitt 4.2), Zustellmodus `Delivery(ctx)` des Gateways (Filtermodus plus Rechte-Stand aus `Observe` statt einer Leseabfrage je Ereignis), In-Tx-Ereignis `AccessChanged`. `Registry` collects the event types, WebSocket ops and terminal channel ops a binary emits and accepts; modules fill it through `Register` in their port, and the start checks it against the closed catalogue of `authz` (M1-25) | `authz` | allen |
| `kernel/txn` | Unit of Work, Phasen vor dem Commit, In-Tx-Ereignisbus (Abschnitt 8) | `platform/pg`; Phasen-Hooks von `authz`, `sync`, `audit` | allen `app`-Paketen |
| `kernel/eventlog` | `Appender` für dauerhafte Ereignisse, `Redactor` zum Schwärzen der Nutzlasten einer gelöschten Nachricht (03, Abschnitt 6.5), `EphemeralPublisher` für kurzlebige | `sync` bzw. `gateway` | allen; `Redactor` nur `messaging` (und `files` mit M3) |
| `kernel/auditing` | `Recorder`: in der Transaktion oder getrennt (für Ablehnungen nach Rollback) | `audit` | allen |
| `kernel/idempotency` | `Store` für `Idempotency-Key` im Unit of Work | `sync` | `platform/httpserver` |
| `kernel/personaldata` | `Participant`: Export und Löschung je Modul für einen Menschen | Module mit personenbezogenen Tabellen | `privacy` |
| `kernel/measure` | `Recorder.Record(metric, key, value)` der eigenen Messung; nur Zähler im Speicher, nie blockierend ([25-visualisierung.md](25-visualisierung.md), Abschnitt 3) | `stats` | `messaging`, `runs`, `billing`, `agents`, `notify`, `gateway`, `platform/httpserver` |

`kernel/ids` prüft nur IDs nach R-DM-01. `correlation_id` ist keine UUIDv7 und wird nie mit
`kernel/ids` geprüft; sie kommt nur aus `kernel/reqmeta` bzw. dem Span-Kontext.

`kernel` hängt von nichts im Projekt ab. Gründe: Diese Ports braucht jedes Modul; als Modul-Kanten
würden sie Zyklen erzeugen (jedes Modul ruft `authorize`, `authz` braucht Ereignisse und Audit).

### 4.6 Aktionen je Modul

Jede Aktion aus [05-aktionen.md](05-aktionen.md) gehört genau einem Modul. Ein Test vergleicht diese
Tabelle mit dem Katalog (Abschnitt 7.3).

| Modul | Aktionen |
|---|---|
| `instance` | `instance.discovery.read`, `instance.read`, `instance.settings.read`, `instance.settings.update`, `instance.branding.update`, `feature_flag.read`, `feature_flag.update`, `signing_key.rotate`, `signing_key.transition` |
| `audit` | `audit.read`, `audit.verify` |
| `authz` | `access.explain`, `access.preview`, `access.view_as`, `group.read`, `group.create`, `group.update`, `group.delete`, `group.member.add`, `group.member.remove`, `group.member.expire`, `group.member.sync`, `grant.read`, `grant.create`, `grant.revoke`, `grant.expire` |
| `identity` | `instance.bootstrap`, `auth.login.begin`, `auth.login.complete`, `auth.token.refresh`, `auth.logout`, `auth.step_up`, `identity_provider.read`, `identity_provider.manage`, `owner.add`, `owner.remove`, `owner.recover`, `self.read`, `user.read`, `user.create`, `user.update`, `user.disable`, `user.enable`, `self.profile.update`, `identity.read`, `identity.link`, `identity.unlink`, `local_credential.set`, `passkey.register`, `passkey.revoke`, `device.list`, `device.approve`, `device.revoke`, `device.expire`, `session.list`, `session.revoke`, `session.extension.create`, `token.list`, `token.create`, `token.revoke`, `access.view_as.end` |
| `chat` | `space.read`, `space.create`, `space.update`, `space.reorder`, `space.archive`, `space.operator.set`, `sidebar.update`, `channel.read`, `channel.create`, `channel.update`, `channel.reorder`, `channel.behavior.read`, `channel.behavior.update`, `channel.operator.set`, `channel.project.bind`, `channel.read_only.set`, `channel.archive`, `forum_tag.manage`, `dm.create`, `dm.leave`, `thread.read`, `thread.title.update`, `thread.close`, `thread.reopen`, `thread.move`, `thread.tags.set`, `agent.config.read` |
| `agents` | `agent.read`, `agent.create`, `agent.update`, `agent.disable`, `agent.enable`, `agent.billing_mode.set`, `agent.billing_mode.set_all`, `agent.credential_holder.set`, `agent.tool_class.set`, `agent.heartbeat`, `agent.events.consume`, `agent.triggerers.read`, `agent_session.read`, `agent_session.reset`, `agent_session.bind`, `agent_session.report`, `command.read`, `command.register`, `command.remove`, `dev_environment.read`, `dev_environment.manage`, `dev_environment.report` |
| `files` | `file.upload`, `file.read`, `file.delete`, `file.transcribe`, `file.transcript.submit` |
| `messaging` | `thread.create`, `message.read`, `message.create`, `message.update`, `message.delete`, `message.revisions.read`, `reaction.add`, `reaction.remove`, `pin.create`, `pin.delete`, `mention.list`, `template.read`, `template.manage`, `export.channel` |
| `runs` | `agent.invoke`, `run.read`, `run.overview.read`, `run.context.read`, `run.start`, `run.update`, `run.finish`, `run.cancel`, `run.followup.create`, `component.update`, `interaction.create`, `interaction.respond`, `command.autocomplete`, `command.autocomplete.respond`, `command.invoke` |
| `billing` | `cost.read.thread`, `cost.read.aggregate`, `price.read`, `price.manage`, `cost.reconcile`, `cost.reconcile.submit` |
| `sync` | `read_state.update`, `read_state.mark_unread`, `draft.read`, `draft.write`, `settings.read`, `settings.update`, `sync.snapshot.read`, `events.catch_up` |
| `notify` | `notification_pref.update`, `quiet_hours.update`, `push.register`, `push.unregister`, `push.deliver` |
| `gateway` | `gateway.connect`, `gateway.focus`, `typing.start` |
| `search` | `search.query` |
| `stats` | `stats.read`, `client_metrics.submit` |
| `apps` | `app.read`, `app.install`, `app.scopes.update`, `app.suspend`, `app.resume`, `app.uninstall`, `webhook_out.read`, `webhook_out.manage`, `webhook_out.redeliver`, `webhook_out.deliver`, `webhook_out.transition`, `webhook_in.read`, `webhook_in.create`, `webhook_in.revoke`, `webhook_in.post` |
| `importer` | `import.read`, `import.manage`, `import.apply` |
| `privacy` | `gdpr.request.create`, `gdpr.request.read`, `gdpr.request.decide`, `gdpr.export.download` |
| `status` | `status.public.read`, `status.read`, `status.manage` |
| `terminal` | `host.read`, `host.manage`, `host.pair`, `host.connect`, `terminal.open`, `terminal.attach`, `terminal.stream`, `terminal.read_own`, `terminal.close`, `terminal.terminate` |

Begründungen für Grenzfälle:

- `agent.invoke` gehört zu `runs`, weil die Folge immer ein Lauf, ein Nachtrag oder ein
  Befehlsaufruf ist. Die Regeln aus 05 Abschnitt 5 wertet trotzdem `authz` aus.
- `agent.config.read` gehört zu `chat`, weil Prompt, Skills und Projektbindung dort liegen.
- `thread.create` gehört zu `messaging`: Ein neuer Thread (Forum-Beitrag oder Thread aus einer
  Nachricht) entsteht mit seiner ersten Nachricht, und `chat` darf `messaging` nicht nutzen.
  `chat` legt die Zeile über den Port `Threads` an und prüft `thread.create` nicht noch einmal;
  nur Tags mit `restricted` prüft `chat` dabei mit `thread.tags.set`, weil nur `chat` die Tags
  kennt (05, Zeile `thread.create`). Den Hauptverlauf eines neuen Kanals legt `chat` selbst als
  Teil von `channel.create` an (R-DM-11).
- `instance.bootstrap` und `owner.recover` gehören zu `identity`, weil sie Menschen, Owner,
  Identitäten und Einladungs-Codes schreiben; `instance` setzt nur `setup_completed_at` über
  seinen Port.
- `command.invoke` und `command.autocomplete` gehören zu `runs` (Zustellung an den Agenten mit
  `agent.invoke`), das Register (`command.register`, `command.remove`, `command.read`) zu `agents`.
- `export.channel` gehört zu `messaging`; das Paket legt `files` ab.
- `client_metrics.submit` gehört zu `stats`: Der Endpunkt nimmt Kennzahlen ohne Inhalte an und gibt
  sie an die Telemetrie weiter ([ADR 0057](../adr/0057-client-telemetrie-ueber-einen-server-endpunkt.md)); er schreibt keine Tabelle.
- Die Dev-Umgebungs-Aktionen gehören zu `agents`, weil die Umgebung an einen Runner-Agenten
  gebunden ist (Etappe).

### 4.7 Ereignistypen und WebSocket-Ops je Modul

Ein Ereignistyp hat genau ein erzeugendes Modul. Die Empfangs-Aktion steht in 05 Abschnitt 6.

| Modul | Dauerhafte Ereignisse | Kurzlebige Ereignisse | WebSocket-Ops (Client an Server) |
|---|---|---|---|
| `instance` | `instance.updated` | | |
| `audit` | | | |
| `authz` | `grant.changed`, `group.changed`, `access.changed`, `access.expiring` | | |
| `identity` | `user.updated`, `session.revoked` | | |
| `chat` | `thread.created`, `thread.updated`, `thread.closed`, `thread.reopened`, `thread.moved`, `thread.moved_out`, `space.created`, `space.updated`, `space.archived`, `channel.created`, `channel.updated`, `channel.settings_changed`, `channel.archived`, `sidebar.updated`, `agent.config_changed` | | |
| `agents` | `agent_session.changed`, `command.changed`, `agent.updated` | `agent.presence_changed` | `ack` |
| `files` | `file.updated` | | |
| `messaging` | `message.created`, `message.updated`, `message.deleted`, `reaction.added`, `reaction.removed`, `pin.added`, `pin.removed` | | |
| `runs` | `run.state_changed`, `agent.invocation`, `agent.followup`, `run.cancel_requested`, `interaction.created`, `command.invoked` | `run.updated`, `command.autocomplete_requested` | `run.update`, `command.autocomplete_result` |
| `billing` | | | |
| `sync` | `read_state.updated`, `draft.updated`, `draft.deleted`, `settings.updated` | | |
| `notify` | `notification_pref.updated`, `quiet_hours.updated` | `notification.created` | |
| `apps` | | | |
| `status` | `status.updated` | | |
| `privacy` | `gdpr.request_updated` | | |
| `importer` | `import.progress` | | |
| `gateway` | | `typing.started` | `identify`, `resume`, `heartbeat`, `reauth`, `focus`, `typing.start`, `typing.stop` |
| `search` | | | |
| `stats` | | `stats.live` | `stats.subscribe`, `stats.unsubscribe` (über den Hub des Gateways) |
| `terminal` | `host.updated`, `terminal_session.changed` | `host.presence_changed` | keine im Gateway; eigene Kanäle mit eigenen Ops (05, Abschnitt 7) |

`message.updated` für `trigger_state` erzeugt `messaging`, auch wenn `runs` den Wechsel auslöst
(über `Messages.SetTriggerState`). Grund: ein Ereignistyp, ein Erzeuger, eine Nutzlast-Definition.

## 5. Schichten im Modul

| Schicht | Paket | Enthält | Darf importieren | Darf nicht |
|---|---|---|---|---|
| Port | `internal/<modul>` | Eingehende Schnittstellen, Befehls- und Ergebnis-Typen (DTOs), In-Tx-Ereignisse, deklarierte Quellen-Ports | `kernel` | `domain`, `app`, `adapters`, fremde Module, Fremdbibliotheken |
| Domain | `internal/<modul>/domain` | Entitäten, Wertobjekte, Zustandsmaschinen (z. B. Lauf-Übergänge), Invarianten, reine Funktionen | Pakete aus `kernel-core` (`kernel/ids`, `kernel/clock`, `kernel/errs`, `kernel/textcheck`, `kernel/reqmeta`, `kernel/jose`, `kernel/designkeys`, `kernel/searchlang`) | Ein- und Ausgabe, SQL, HTTP, Kontext mit Transaktion, Fremdbibliotheken |
| Anwendung | `internal/<modul>/app` | Anwendungsfälle: `authorize`, Unit of Work, Domain aufrufen, Ereignisse anhängen; ausgehende Ports (Repository, Uhr, Fremddienst) als Schnittstellen | eigener Port, eigene Domain, `kernel`, Ports erlaubter Module | `adapters`, `platform`, `api`, Fremdbibliotheken |
| Adapter | `internal/<modul>/adapters/<art>` | `postgres` (sqlc-Code und Repository), `httpapi` (Teil des generierten Server-Interfaces), weitere wie `objectstore`, `clamav`, `apns` | alles des eigenen Moduls, `kernel`, `platform`, `api`, Ports erlaubter Module, zugelassene Fremdbibliotheken | `app` oder `domain` fremder Module |

Regeln mit Grund:

1. Der Port enthält eigene DTOs statt Domänentypen. Grund: Die Domain bleibt änderbar, ohne andere
   Module zu brechen, und ein Modul kann später hinter eine Prozessgrenze wandern (Abschnitt 16).
2. Die Anwendungsschicht kennt keine Fremdbibliothek. Grund: Anwendungsfälle sind mit Fakes testbar
   ohne Datenbank; Treiberwechsel betrifft nur Adapter.
3. HTTP-Handler sind Adapter. Jedes Modul implementiert den Teil des generierten
   `StrictServerInterface`, der zu seinen Operationen gehört. `internal/wire` bettet alle
   Modul-Handler in eine Struktur ein; der Compiler prüft, dass das Interface vollständig ist und
   keine Operation doppelt belegt ist (doppelte Methoden sind bei Einbettung mehrdeutig und erfüllen
   das Interface nicht).
4. Fremdbibliotheken sind je Adapter-Art zugelassen (Liste in `modules.yaml`, Abschnitt 7.1).
   Beispiel: nur `gateway/adapters` darf `github.com/coder/websocket` importieren.
5. Metriken: Ein Anwendungsfall zeichnet Kennzahlen über einen ausgehenden Port auf
   (`internal/<modul>/app/metrics.go`, mit einer Umsetzung ohne Wirkung für Unit-Tests); die
   Umsetzung mit OpenTelemetry liegt in `internal/<modul>/adapters/otelmetrics`. Plattform-Pakete
   zeichnen direkt über `platform/otel` auf. Grund: Regel 2; Namen und Attribute kommen nur aus
   dem Signal-Katalog ([signals.md](../observability/signals.md)).

## 6. Code-Layout

```text
services/server/
  go.mod                      Modulpfad github.com/OWNER/bothy/services/server bis V1 (PLAN P-05)
  cmd/bothy/                  main: Unterbefehle serve, migrate, setup, config
  api/
    rest/                     generiert aus docs/api/openapi.yaml (oapi-codegen: std-http-server, strict-server)
    ws/                       Rahmen- und Nutzlast-Typen aus docs/api/asyncapi.yaml (Werkzeug: 06-api.md)
    routes/                   generiert: operationId -> Aktion, Rate-Limit-Klasse; Op -> Aktion
  architecture/
    modules.yaml              Modulregister: Tabellen, Aktionen, Abhängigkeiten, Fremdbibliotheken
  migrations/                 goose-SQL, eingebettet ins Binary (ADR 0007)
  internal/
    kernel/                   Querschnitts-Ports (Abschnitt 4.5), ohne Abhängigkeiten im Projekt
    platform/                 Technik ohne Fachlogik
      config/ secrets/ pg/ httpserver/ ratelimit/ jobs/ otel/ health/ lifecycle/ egress/ buildinfo/
      jsoncanon/
    wire/                     Verdrahtung je Rolle, Registrierung der In-Tx-Abonnenten
    <modul>/                  Port (Wurzelpaket)
      domain/
      app/
      adapters/
        postgres/             queries/*.sql, sqlcgen/ (generiert), Repository
        httpapi/              Handler für die Operationen des Moduls
        ...                   weitere Adapter des Moduls
      README.md               Zweck, Ports, Abhängigkeiten, Tabellen, Kapitel
  test/                       Testunterstützung über Modulgrenzen: harness/ (echter Server, M1-71),
                              integration/, leak/, contract/, canary/, smoke/
  perf/                       Lasttests (Build-Tag perf) und internal/report (Schwellen aus Abschnitt 22)
  tools/archgen/              Generator: architecture/modules.yaml -> .go-arch-lint.yml
  tools/flagcheck/            Gate: Registrierung der Feature-Flags gegen VERSION (ADR 0064)
  sqlc.yaml                   ein Eintrag je Modul, Schema aus migrations/
  .go-arch-lint.yml           generiert aus architecture/modules.yaml
services/host/                Host-Dienst bothy-host (eigenes Go-Modul, eigene Version; 24-terminal.md)
  cmd/bothy-host/             main: commands install, session-user, restart, pair, doctor, run, pty-helper (M2-42)
  internal/                   noise, frame, fingerprint (M2-38); buildinfo, clock (+ clocktest), config, state,
                              hostos (+ hostostest: the fake operating system), doctor (HD1 to HD11), localchan
                              (local channel, admin endpoint), helper (pty-helper), install (plans, units/),
                              startup (run, the Control port), pairing (M2-42); control, pty, profiles follow
  test/platform/              platform suite (build tag platform), only on disposable machines (M2-42)
  testdata/noise/             Testvektoren, Aufzeichnungen Go und Dart, Fingerabdruck-Vektoren (24, 5.2, 14)
  testdata/fakeclaude/        stand-in for claude auth status in the tests of HD6 (M2-42)
  .go-arch-lint.yml           von Hand gepflegt, eine Komponente je Paket (kein Modulregister)
```

Testhilfen, die Tests mehrerer Pakete nutzen, liegen im Baum der Komponente, deren Typen sie bauen,
damit das Architektur-Gate sie einer Komponente zuordnet: `internal/platform/<p>/<p>test` (z. B.
`pgtest`, `httpservertest`) und `internal/<modul>/adapters/<modul>test` (z. B. `identitytest` mit
Geräteschlüsseln, DPoP-Beweisen und echter Anmeldung). Nur `_test.go`-Dateien importieren sie; ein
Test (`go list -deps ./cmd/bothy`) hält sie aus dem Binary. `test/**` und `perf/**` bilden die
Komponente `testsupport` (7.1): Sie darf alles nutzen, keine Produktions-Komponente darf sie nutzen,
und die Abdeckung je Go-Modul (`quality.coverage.go_min_percent`, 23) zählt sie nicht mit, weil sie
selbst Testcode ist; ebenso wenig erzeugte Dateien (`*.gen.go`). Fachliche Teilbereiche eines Moduls
(OIDC, Flags, Konsole) sind Dateien oder Unterpakete von `domain`, `app` und `adapters`, nie
eigene Pakete daneben; ein Paket ohne Komponente scheitert im Gate (7.1).

Stellen mit dem Produktnamen (Baseline Abschnitt 1, ADR 0044; damit ein Wechsel billig bleibt),
nach Art:

- Server: Go-Modulpfad in `go.mod`; Binary `bothy` (`cmd/bothy`); Präfix der Umgebungsvariablen
  `BOTHY_`; Namen der Images (`bothy-server`, `bothy-postgres`, `bothy-backupd`) und Pfade im
  Container (`/run/bothy`, `/var/lib/bothy`); Rollen der Datenbank (`bothy_*`).
- Host-Dienst ([24-terminal.md](24-terminal.md)): Go-Modulpfad unter `services/host`; Binary,
  Dienst- und Benutzername `bothy-host`; Zustandsordner (`/var/lib/bothy-host`,
  `%ProgramData%\bothy-host`); Protokoll-Kennungen `bothy-terminal/1` (Noise-Prolog) und
  `bothy-host-fp/1` (Fingerabdruck); Release-Komponente `host`. Since M2-42: program folders
  `/opt/bothy-host/bin/`, `/Library/Application Support/bothy-host/bin/` and
  `%ProgramFiles%\bothy-host\`; configuration folders `/etc/bothy-host/`,
  `/Library/Application Support/bothy-host/` and `%ProgramFiles%\bothy-host\`; systemd units
  `bothy-host.service`, `bothy-host-helper-<name>.socket` and `.service`; launchd labels
  `bothy-host` and `bothy-host.helper.<name>`; Windows services `bothy-host` and
  `bothy-host-helper-<name>` and the virtual account `NT SERVICE\bothy-host`; channel endpoints
  `/run/bothy-host/`, `/Library/Application Support/bothy-host/run/` and the pipes
  `bothy-host-admin` and `bothy-host-helper-<name>`.
- Pakete der Apps: Dart-Pakete mit Präfix `bothy_` (`bothy_viz`, `bothy_noise`).
- SDKs: Python distribution `bothy-sdk`, import package `bothy_sdk`, npm package `bothy-sdk` (not
  published until OQ-17 is decided).
- Grafana-Plugin ([25-visualisierung.md](25-visualisierung.md), Abschnitt 8): Ordner und
  Release-Komponente `grafana-bothy-live`, Plugin-ID `bothy-live-panel`, Dashboard `bothy-live`.
- Querschnitt: Präfix der Telemetrie-, Alarm- und SLO-Namen (`bothy.`, `bothy_`, `Bothy`); Präfix
  der API-Tokens und Kopplungs-Codes `bothy_` mit der gitleaks-Regel
  ([ADR 0061](../adr/0061-token-format-mit-festem-praefix-und-pruefsumme.md)); Standardwert der
  Einstellung `branding.product_name`.
- Werkzeuge und Betrieb: Kennungen mit dem Präfix `bothy-` (Regeln von Semgrep, Workflows,
  Dashboards, SLOs, Compose-Dienste, temporäre Namen); der Name allein als Wert der
  Standardkonfiguration (Datenbank- und Dienstname, Komponenten); Anzeigenamen in den Werkzeugen
  (NOTICE, Doku-Site, Projekt und Quality Gate in SonarQube, Titel von gitleaks, Dashboards und
  Ordner in Grafana, Design-Vorschau). Fest und ohne Namenswechsel: die Schlüssel der Advisory
  Locks `bothyjob` und `bothyboo` (ein neuer Wert ließe zwei Server-Versionen gleichzeitig
  migrieren).

Diese Liste ist die einzige; ein Test (PLAN M1-46, `tools/ci/tests/test_product_name_places.py`)
erlaubt den Namen in Code und Konfiguration (ohne `docs/`, Markdown und aus der Doku erzeugten
Code) nur in diesen Formen: Modulpfad, ganzes Pfadsegment, Präfix (`bothy-`, `bothy_`, `bothy.`,
`BOTHY_`), Alarmname `Bothy<Name>` (nur die Namen der Alarmregeln und Beispiele des
Alarm-Werkzeugs), Binary vor einem Unterbefehl (in `cmd/bothy` auch als Befehlswort und in
Bezeichnern), ganzer Wert außerhalb des Client-Codes (`apps/`, `packages/`) und Anzeigename in den
genannten Werkzeugdateien. Fließtext in Kommentaren und Texte für Menschen nennen den Namen nicht. Ein Wechsel des Namens ändert die Protokoll-Kennungen nicht still: Sie
sind Teil des Terminal-Protokolls und wechseln nur mit einer neuen Protokollversion.

## 7. Automatische Prüfung der Regeln

### 7.1 Modulregister als Quelle

`services/server/architecture/modules.yaml` ist die einzige Quelle für Modulgrenzen ([ADR 0050](../adr/0050-modulregister-als-quelle-fuer-architektur-gate-und-tabellenbesitz.md)).
Daraus entstehen die go-arch-lint-Konfiguration, die Prüfungen aus 7.3 und der Abgleich mit den
Tabellen 4.2, 4.4, 4.6 und 4.7 dieses Kapitels. Format (normativ; spätere Tickets ergänzen
Einträge, nie Felder):

```yaml
# SPDX-License-Identifier: Apache-2.0
version: 1
exclude: [tools]                  # Verzeichnisse, die go-arch-lint nicht prüft (Werkzeuge)
kernel:
  core: [ids, clock, errs, textcheck, reqmeta, jose, designkeys, searchlang]   # von jedem Paket nutzbar
  ports: [actor, authorize, txn, eventlog, auditing, idempotency, personaldata, measure]
platform:                         # je Plattform-Paket eine Komponente
  jobs: {depends_on: [pg], vendors: [river]}
  pg: {vendors: [pgx]}
wire: {vendors: []}
cmd: {vendors: []}
api: {vendors: []}               # mit dem ersten generierten Code (M1-17)
testsupport: {paths: [test, perf]}   # Testcode über Modulgrenzen; darf alles, niemand darf ihn nutzen
modules:
  chat:
    tables: [spaces, user_space_states, channels, channel_skill_bindings, project_bindings,
             dm_participants, forum_tags, threads, thread_tags]
    actions: [space.read, space.create, thread.move, agent.config.read]   # vollständig im Register
    events: [thread.created, channel.created]                              # vollständig im Register
    ephemeral_events: []
    ws_ops: []
    depends_on: [identity, authz]
    adapter_vendors: [pgx, otel]
  # identity, authz und die übrigen Module: Einträge wie chat, hier ausgelassen
vendors:                          # <gruppe>: Importpfade
  otel: [go.opentelemetry.io/otel, go.opentelemetry.io/otel/**]
  pgx: [github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/**]
  river: [github.com/riverqueue/river, github.com/riverqueue/river/**]
```

Regeln: Module stehen in der Reihenfolge der Ebenen aus 4.4; `tables`, `actions`, `events`,
`ephemeral_events`, `ws_ops` und `depends_on` folgen den Tabellen 4.2, 4.6, 4.7 und 4.4. Der
Generator lehnt unbekannte Felder, doppelte Einträge, unbekannte Module oder Vendor-Gruppen und
Zyklen ab, dazu Schlüssel und Werte, die keine einfachen Zeichenketten sind (getaggt, etwa mit
`!!binary`, oder ohne Anführungszeichen so geschrieben, dass ein YAML-1.1-Leser wie PyYAML sie
nicht als Zeichenkette liest, etwa `no`, `on` oder `1:20`, dazu `y`, `n`, `yes`, `no`, `on` und
`off` in jeder Schreibweise), eine `version` außerhalb des Zehnersystems, Einträge in
`testsupport.paths` unter `internal`, `cmd` oder `api`, Einträge in `exclude`, die als
Zeichenketten-Präfix `internal`, `cmd`, `api` oder ein anderes Verzeichnis des Moduls treffen
(go-arch-lint lässt jeden Pfad aus, der mit dem Eintrag beginnt, `in` also ganz `internal/`;
beides nähme Produktcode aus dem Gate), und Vendor-Muster, die kein Go-Importpfad sind:
Das erste Element ist eine Domain mit Punkt, die übrigen enthalten nur Buchstaben, Ziffern, `-`,
`.`, `_` und `~`, und `**` steht nur am Ende nach mindestens zwei festen Elementen. Grund:
go-arch-lint macht aus jedem Muster einen regulären Ausdruck und maskiert nur `.` und `/`, und
`github.com/**` gäbe fast jede Bibliothek frei. Ein Ticket, das ein Plattform-Paket oder eine
Fremdbibliothek einführt, trägt seinen Eintrag und seine Vendor-Gruppe in derselben Änderung ein;
ein Paket ohne Eintrag scheitert in `make arch` ("not attached to any component").
The optional top-level section `migrations: {used_by: [<platform package>, ...]}` (M1-12) makes
the package `migrations`, the embedded SQL migrations of ADR 0007, a component of its own that
uses nothing; `wire`, `cmd` and the platform packages listed in `used_by` may use it (the server
register lists `pg`, which runs them and builds the test databases from them).

### 7.2 go-arch-lint

Werkzeug: go-arch-lint (MIT), aktuell v1.19.0 vom 2026-09-07 [Q8, belegt]. Der Generator
`tools/archgen` erzeugt aus `modules.yaml` je Modul vier Komponenten (`<m>`, `<m>-domain`,
`<m>-app`, `<m>-adapters`), je Plattform-Paket eine Komponente `platform-<p>`, dazu `wire`, `cmd`,
`api` und `testsupport` (alle Projekt- und Fremdabhängigkeiten erlaubt, in keinem `mayDependOn`
einer anderen Komponente), und die Regeln aus Abschnitt 5. Grund für eine Komponente je Plattform-Paket: Nur so
lässt sich eine Fremdbibliothek auf genau ein Paket begrenzen (River nur in `platform/jobs`,
Abschnitt 12.1). Erzeugter Auszug für `chat`:

```yaml
version: 3
workdir: .
allow:
  depOnAnyVendor: false
  ignoreNotFoundComponents: true
exclude: [tools]
excludeFiles: ["^.*_test\\.go$"]
vendors:
  v-otel:  { in: [go.opentelemetry.io/otel, go.opentelemetry.io/otel/**] }
  v-pgx:   { in: [github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/**] }
  v-river: { in: [github.com/riverqueue/river, github.com/riverqueue/river/**] }
components:
  kernel-core:   { in: [internal/kernel/ids, internal/kernel/clock, internal/kernel/errs,
                        internal/kernel/textcheck, internal/kernel/reqmeta, internal/kernel/jose,
                        internal/kernel/designkeys, internal/kernel/searchlang] }
  kernel-ports:  { in: [internal/kernel/actor, internal/kernel/authorize, internal/kernel/txn,
                        internal/kernel/eventlog, internal/kernel/auditing,
                        internal/kernel/idempotency, internal/kernel/personaldata,
                        internal/kernel/measure] }
  platform-jobs: { in: [internal/platform/jobs, internal/platform/jobs/**] }
  platform-pg:   { in: [internal/platform/pg, internal/platform/pg/**] }
  wire:          { in: [internal/wire, internal/wire/**] }
  cmd:           { in: cmd/** }
  api:           { in: api/** }
  chat:          { in: internal/chat }
  chat-domain:   { in: [internal/chat/domain, internal/chat/domain/**] }
  chat-app:      { in: [internal/chat/app, internal/chat/app/**] }
  chat-adapters: { in: internal/chat/adapters/** }
commonComponents: [kernel-core]
deps:
  kernel-core:   { mayDependOn: [kernel-core] }
  platform-jobs: { mayDependOn: [platform-jobs, kernel-ports, platform-pg], canUse: [v-river] }
  platform-pg:   { mayDependOn: [platform-pg, kernel-ports], canUse: [v-pgx] }
  chat:          { mayDependOn: [chat, kernel-ports] }
  chat-domain:   { mayDependOn: [chat-domain] }
  chat-app:      { mayDependOn: [chat-app, chat, chat-domain, kernel-ports, identity, authz] }
  chat-adapters: { mayDependOn: [chat-adapters, chat, chat-app, chat-domain, kernel-ports,
                                 platform-jobs, platform-pg, api, identity, authz],
                   canUse: [v-pgx, v-otel] }
```

Der Auszug ist ein Teil dessen, was `tools/archgen` aus dem Beispiel in 7.1 erzeugt; der Test
`TestChapter02ExamplesMatchGenerator` hält Beispiel und Auszug gegen den Generator.

Jede Komponente nennt sich selbst zuerst in ihrem `mayDependOn`: go-arch-lint erlaubt einer
Komponente den Import ihrer eigenen Pakete nur dann oder wenn sie unter `commonComponents` steht,
und das Layout aus Abschnitt 6 braucht ihn (Unterpakete von `domain`, `app` und `adapters`,
`adapters/postgres/sqlcgen`, die Testhilfen `platform/<p>/<p>test` und `adapters/<modul>test`,
`api/routes` ruft `api/rest`). `kernel-core` steht unter `commonComponents` und bräuchte den
Eintrag darum nicht; er bleibt der einheitlichen Regel wegen. `wire` darf dazu alle Modul-,
Plattform- und Kernel-Komponenten nutzen, `cmd` nur `wire`, Plattform und `kernel-ports`.
`<m>-domain` darf nur sich selbst und `kernel-core`. `kernel-ports` hat keinen Eintrag unter
`deps`, weil go-arch-lint ein leeres `mayDependOn` ablehnt, und darf damit nur `kernel-core`: Die
Kernel-Ports nutzen einander nicht. Abschnitt 4.5 sagt nur, dass `kernel` von
nichts im Projekt abhängt; die engere Lesart gilt, bis ein Ticket eine Kante zwischen zwei Ports
begründet und diese Regel in derselben Änderung anpasst. `ignoreNotFoundComponents: true` lässt
Komponenten zu, deren Muster noch kein Verzeichnis trifft (`<m>-domain`, `<m>-app` und
`<m>-adapters` vor dem ersten Anwendungsfall, `testsupport` vor `test/` und `perf/`); ohne die
Einstellung bricht go-arch-lint v1.19.0 für jedes solche Muster ab.
Ein Paket ohne Komponente und ein unbekannter Komponentenname in `mayDependOn` scheitern weiterhin
(geprüft am 2026-10-11 in M1-05).

`make arch` lehnt vor go-arch-lint jeden symbolischen Link unterhalb des Moduls ab: go-arch-lint
folgt einem verlinkten Verzeichnis nicht und ordnet es nach dem Pfad des Links zu, der
Go-Compiler baut aber den Code dahinter. Ausgenommen sind nur Sperrdateien von Editoren, also
Links mit Namen `.#*`, deren Ziel nicht existiert; hinter ihnen lässt sich nichts bauen. Weil
go-arch-lint v1.19.0 an jedem toten Link unterhalb einer Komponente mit `**` abbricht, prüft
`make arch` dann eine Kopie des Moduls ohne diese Namen und nennt in Meldungen die Dateien des
Moduls; der Go-Compiler übergeht Dateien mit Punkt am Anfang ohnehin.

Geprüft am 2026-10-10 an einem Gerüst mit allen Modulen aus Abschnitt 4.2 (Go 1.27.2, go-arch-lint
v1.19.0): sauberes Gerüst ergibt "No warnings found", Exitcode 0. Vier eingebaute Verstöße wurden
alle gemeldet, Exitcode 1: `chat/domain` importiert pgx; `chat/app` importiert den Port von
`messaging`; `messaging/app` importiert `runs/app`; ein neues Paket `chat/util` ohne Komponente
("not attached to any component"). Der letzte Punkt ist gewollt: Neue Pakete müssen ins Register.
Die Form mit Plattform-Komponenten belegen seit M1-05 die Tests in `tools/archgen` an einem Gerüst
mit Plattform-Paket und Treiber-Stub: `TestArchLintSkeletonClean` (Exitcode 0) und die vier
Verstöße `TestArchLintDomainImportsDriver`, `TestArchLintAppImportsForeignPort`,
`TestArchLintAppImportsForeignApp` (dort importiert `messaging/app` die App des erlaubten Moduls
`chat`) und `TestArchLintPackageWithoutComponent` (Exitcode 1 mit dem Paketpfad), dazu
`TestArchLintTestSupport` für die Komponente `testsupport`, `TestArchLintChapter02Layout` (das
Layout aus Abschnitt 6 mit mehreren Paketen je Komponente, Exitcode 0) und
`TestArchLintLayersStayClosed` (Domain nutzt eigene Adapter oder App, Port nutzt eigene Domain, App
nutzt ein Unterpaket einer fremden App, ein Kernel-Port nutzt einen anderen: Exitcode 1)
([ADR 0050](../adr/0050-modulregister-als-quelle-fuer-architektur-gate-und-tabellenbesitz.md), "Confirmation").

### 7.3 Weitere Prüfungen im Gate

| Prüfung | Was sie hält | Umsetzung (Ticket) |
|---|---|---|
| Tabellenbesitz | SQL eines Moduls nennt nur eigene Tabellen (Grundsatz 12 aus 03) | Durchlauf über `adapters/postgres/queries/*.sql`, Tabellennamen gegen `modules.yaml` |
| Vollständigkeit | jede Tabelle aus 03 und jede Aktion aus 05 genau einmal im Register | `make register-check` (M1-06): Abgleich des Registers mit 03 und `actions.yaml`, auch Ereignistypen und Ops; der Abgleich mit den Migrationen kommt mit dem ersten Schema |
| Routen und Ops | jede Operation hat eine bekannte Aktion; ihr Handler liegt im Modul, dem die Aktion gehört | Start-Prüfung des Servers und Test über die generierte Routen-Tabelle |
| In-Tx-Abonnements | jedes Abonnement ist eine erlaubte Richtung (höheres Modul abonniert tieferes) | Golden-Datei der Registrierungen in `internal/wire`; Veröffentlicher und Abonnent leitet die Registrierung aus den Go-Paketen ab, nicht aus Angaben |
| Zähler-Zugriff | nur die Phasen-Hooks von `authz`, `sync`, `audit` nutzen `instance.Counters` | Phasen-Wächter in `Counters` (Laufzeit, 8.2) und statischer Test: nur `instance`, `wire` und die Pakete der registrierten Hooks referenzieren `Counters` |
| Doku-Abgleich | Tabellen 4.2, 4.4, 4.6, 4.7 dieses Kapitels stimmen mit dem Register überein | `make register-check` (M1-06): vergleicht die Maschinenspalten, nicht die Prosa; hält dazu 4.5, das Komponenten-Diagramm und die Modul-READMEs |
| Erzeugte Dateien aktuell | `.go-arch-lint.yml`, sqlc-Code, API-Code entsprechen ihrer Quelle | Neu erzeugen und `git diff --exit-code` |

## 8. Transaktionen und Unit of Work

### 8.1 Ablauf

Jeder Schreib-Anwendungsfall läuft in genau einer Datenbank-Transaktion, `READ COMMITTED` (03,
Grundsatz 11). `kernel/txn` legt sie in den Kontext. Ruft ein Anwendungsfall einen Port eines anderen
Moduls, schreibt dieses Modul in derselben Transaktion (ambiente Transaktion). Nur `platform/pg`
öffnet und beendet Transaktionen.

Phasen (Diagramm [fluss-ereignis-pfad.svg](../architecture/fluss-ereignis-pfad.svg)):

1. **Beginn.** Bei Rechte-Änderungen (Option `RightsChange`) ruft `platform/pg` zuerst den
   Phasen-Hook von `authz`. Er sperrt `sequence_counters('permission_version')` über
   `instance.Counters`, erhöht ihn (03, 6.3) und hält die sichtbaren Ziele der Betroffenen vor der
   Änderung fest. `RightsChange` nennt die Betroffenen direkt (Prinzipale, z. B. das Subjekt einer
   Freigabe) oder ein Ziel (Bereich oder Kanal, z. B. beim Archivieren); bei einem Ziel bestimmt
   `authz` die Betroffenen selbst: Subjekte gültiger Freigaben auf dem Ziel und seinen Vorfahren
   (Gruppen aufgelöst), aktive Owner und die Agenten, die das Ziel nach der Bediener-Regel
   bedienen. Erfährt der Anwendungsfall in der Arbeit weitere Betroffene (z. B. den neuen
   Bediener), meldet er sie vor dem Schreiben nach (04, Abschnitt 8.2).
2. **Arbeit.** Domänenzeilen schreiben, In-Tx-Ereignisse veröffentlichen, dauerhafte Ereignisse
   anhängen (im Kontext gepuffert, noch ohne Nummer).
3. **Vor dem Commit, feste Reihenfolge:**
   1. `authz` (Phasen-Hook): nur bei Rechte-Änderung. I1 prüfen (03, 6.4), Unterschied der
      sichtbaren Ziele vorher und nachher berechnen, `access.changed` anhängen, In-Tx-Ereignis
      `AccessChanged` veröffentlichen. Abonnenten (z. B. `runs` für Abbrüche) laufen hier noch in
      der Transaktion.
   2. `sync` (Phasen-Hook): `event_seq` sperren und um die Zahl der Ereignisse erhöhen,
      `event_log` schreiben, `permission_version` lesen (nach der Sperre, damit der Wert zur
      Commit-Reihenfolge passt), `pg_notify('event_log', <höchste seq>)`.
   3. `audit` (Phasen-Hook): `audit_seq` sperren, Hash-Kette fortschreiben.
4. **Commit.** Danach Antwort an den Aufrufer.

`platform/pg` kennt keine Fachmodule. `kernel/txn` definiert die Phasen und das Register
`txn.Registry` (`SetHook`, `Subscribe`, `Seal`); `internal/wire` registriert die Hooks von `authz`
(Phase 1 und 3.1), `sync` (3.2) und `audit` (3.3) und alle Abonnenten an genau einer Stelle und
versiegelt das Register vor dem ersten Unit of Work; `platform/pg` ruft die Hooks in dieser
Reihenfolge. Hooks und Abonnenten sind Funktionen oder Methodenwerte des besitzenden Moduls, nie
Closures in `internal/wire`; das Register leitet ihr Modul aus dem Go-Paket ab und lehnt sonst ab.
Grund: Die Golden-Datei soll den echten Besitzer zeigen. `RightsChange` gilt nur am äußersten
Unit of Work; In-Tx-Ereignisse und dauerhafte Ereignisse entstehen nur in den Phasen "Arbeit" und
3.1.

Ein Veto in Phase 2 oder 3.1 (Domänenfehler, I1-Verletzung) rollt alles zurück. Ablehnungen, die ins
Audit müssen, schreibt `auditing.Recorder` danach in einer eigenen Transaktion (03, 6.4).

### 8.2 Sperrreihenfolge

Die Reihenfolge aus 03 Abschnitt 6.3 setzt die Unit of Work durch, nicht der einzelne
Anwendungsfall: Zähler sperren nur diese Phasen-Hooks über `instance.Counters`; sonst ruft kein
Modul `Counters` auf. `Counters` prüft dazu die Phase zur Laufzeit (`permission_version` nur in
Phase 1, `event_seq` nur in 3.2, `audit_seq` nur in 3.3; sonst Fehlerart `internal`), und ein
statischer Test begrenzt die Pakete, die `Counters` nennen (7.3). Mehrere Zeilen derselben Tabelle
sperrt ein Modul in aufsteigender `id`. Schlägt eine Transaktion mit Deadlock (`40P01`) oder
Sperr-Zeitüberschreitung (`55P03`) fehl, wiederholt `platform/pg` den ganzen Unit of Work höchstens
`database.tx_retry_max` Mal. Das ist sicher, weil der Unit of Work vollständig zurückgerollt ist und
keine externen Aufrufe enthält. Ausnahme: Das Warten am Idempotenz-Schlüssel (8.4) meldet der
Speicher als `request_in_progress`, nicht als wiederholbaren `55P03`; sonst würde die Wiederholung
die Wartezeit vervielfachen.

### 8.3 In-Tx-Ereignisse

- Typen liegen im Port des veröffentlichenden Moduls, Querschnitts-Ereignisse in `kernel`
  (`authorize.AccessChanged`).
- Zustellung synchron, in der Reihenfolge der Registrierung, in derselben Transaktion. Ein Abonnent
  darf schreiben, dauerhafte Ereignisse anhängen und mit einem Fehler den Commit verhindern.
- Registrierung nur in `internal/wire`, als eine Liste. Eine Golden-Datei hält sie fest; jede Änderung
  ist im Diff sichtbar. Jeder Abonnent bekommt einen eigenen OTel-Span `txn.subscribe <Abonnent>`
  mit `bothy.outcome` (`ok` oder `veto`) ([signals.md](../observability/signals.md), Abschnitt 5.2).
- Grund für synchron statt asynchron: Die Folgen (Lauf anlegen, Entwurf leeren, Lauf abbrechen bei
  Entzug) müssen atomar mit der Ursache sein. Was nicht atomar sein muss, läuft über das
  `event_log` (Abschnitt 11).

### 8.4 Idempotenz im Unit of Work

- Nachrichten und Interaktionen: Paar aus Autor und `client_msg_id` bzw. `client_interaction_id`
  nach 04 Abschnitt 5.1.
- Alle anderen schreibenden Operationen mit `Idempotency-Key`: Eine Stufe der HTTP-Kette
  (`platform/httpserver`, Speicher `kernel/idempotency.Store` aus `sync`) öffnet den Unit of Work,
  legt die Zeile in `idempotency_keys` mit `in_progress` an, ruft den Anwendungsfall (der der
  Transaktion beitritt), speichert bei `2xx` Status und Antwortkörper und committet alles zusammen.
  Endgültige Fehlerantworten (06, Abschnitt 7, Punkt 2) haben keine Wirkung: Der Unit of Work rollt
  zurück, und die Antwort wird danach in einer eigenen kurzen Transaktion gespeichert. Eine
  gleichzeitige Wiederholung wartet am Primärschlüssel, bis die erste Transaktion endet, und liest
  dann die gespeicherte Antwort. Endet die Wartezeit mit `database.lock_timeout`, antwortet der
  Server `409 request_in_progress` (04, 5.2) und speichert nichts.
- Ausnahme (M2-41): Eine endgültige Fehlerantwort, die der Handler mit
  `idempotency.KeepEffectOnFinal` markiert, behält ihre Wirkung; die Stufe speichert sie mit
  `Store.Complete` in derselben Transaktion und committet beides zusammen. Markiert wird nur
  `409 fingerprint_mismatch` von `confirmHost`, weil jeder Fehlversuch auch mit Schlüssel zählen
  muss (TM-T19); eine markierte `401` oder `5xx` rollt weiter zurück (docs/DECISIONS.md D-M2-41-7).
- Rights changes behind the stage (M1-31): the stage opens its unit with
  `txn.AllowLateRightsChange()`. The use case declares its rights change in its nested unit as
  everywhere else; that nested unit then runs the rights begin hook (phase 1), its work and the
  rights check (phase 3.1) inside the stage's transaction, and sync and audit run once at the end.
  The stage has written only its key row by then, so `permission_version` is still the first
  domain lock (section "Sperrreihenfolge"); a second rights change in the same unit is refused.
- Grund: Schlüssel und Wirkung entstehen in einer Transaktion. Ein Absturz dazwischen hinterlässt
  keinen Schlüssel ohne Wirkung und keine Wirkung ohne Schlüssel.

### 8.5 Was nie in einer Transaktion passiert

Keine Netzwerk-Aufrufe nach außen (Objektspeicher, Push-Dienste, Webhooks, Identitätsanbieter,
clamd, Transkription) innerhalb einer Datenbank-Transaktion. Folgen nach außen entstehen nach dem
Commit über einen Ereignis-Verbraucher oder einen River-Job, der mit `InsertTx` in derselben
Transaktion eingefügt wurde [Q5, belegt]. Grund: lange Transaktionen halten Sperren, verlängern die
serialisierte Commit-Phase am `event_seq`-Zähler und machen Wiederholungen unsicher.

## 9. Request-Pfad

Diagramm: [fluss-request-pfad.svg](../architecture/fluss-request-pfad.svg).

### 9.1 Kette im api-Knoten

Äußerer Teil, für jede Anfrage auf dem öffentlichen Listener vor dem Routing:

| Schritt | Ort | Aufgabe |
|---|---|---|
| 1 | `platform/httpserver` | Panic abfangen (`500 internal_error`, Stack nur im Log) |
| 2 | `platform/otel` | Server-Span über otelhttp (eingehender `traceparent` nach 17, Abschnitt 3), `correlation_id` aus der Trace-ID, Zugriffslog ohne Inhalte, Query und Tokens |
| 3 | `platform/httpserver` | Basis-Kopfzeilen (SEC-HDR-1, `Correlation-Id`), Client-IP aus vertrauten Proxys in `kernel/reqmeta` |
| 4 | `net/http`-Router | Operation finden; kein Muster: `404 not_found`, falsche Methode: `405 method_not_allowed` mit `Allow` |

Je Route, um den Handler jeder Operation gelegt (Muster und Operation sind bekannt):

| Schritt | Ort | Aufgabe |
|---|---|---|
| 5 | `platform/httpserver` | Operation mit Aktion und Rate-Limit-Klasse aus `api/routes` in den Kontext; Grenzen je Routenklasse: `http.max_json_body_bytes` bzw. `file.max_upload_bytes`, Frist `http.request_timeout` bzw. `http.transfer_timeout`; Antwort-Schreiber mit Nachweis-Prüfung (9.3) |
| 6 | `platform/httpserver` | CORS (SEC-CORS-1, SEC-CORS-2) |
| 7 | `kernel/actor` (von `identity`) | Zugriffstoken mit DPoP oder API-Token prüfen, Sitzung, Gerät, Prinzipal-Status; ohne Anmeldedaten: Prinzipal `anonymous` |
| 8 | `platform/ratelimit` | Token-Bucket der Klasse, `429` mit `Retry-After` |
| 9 | `platform/httpserver` | Wartungsmodus (05, Regel 9) |
| 10 | `platform/httpserver` | `Idempotency-Key` (8.4) |
| 11 | `platform/httpserver` | Medientyp; JSON-Wächter: doppelte Schlüssel, ungültiges UTF-8, Schachtelung über `http.max_json_depth` (SEC-IN-2) |
| 12 | `platform/httpserver` | Pflicht-`If-Match` (`428 precondition_required`) |
| 13 | `api/rest` (generiert) | Anfrage gegen das Schema prüfen (SEC-IN-1) |
| 14 | `platform/httpserver` | Prüfprotokoll für den Nachweis installieren (9.3) |
| 15 | `<modul>/adapters/httpapi`, `<modul>/app` | DTO bauen, Anwendungsfall mit `authorize`, Unit of Work, Domain, Repository; Antwort abbilden |

Jeder Schritt meldet Fehler als `application/problem+json` (Abschnitt 14.1). Grund für die
Reihenfolge: Körpergrenze und Frist hängen von der Routenklasse ab, und die ist erst nach dem
Routing bekannt; die Drosselung läuft vor dem teuren Teil (Dekodierung, Schema, Datenbank), aber
nach der Authentisierung, weil die meisten Klassen je Prinzipal zählen (05, Abschnitt 3).

Gegen Fluten mit ungültigen Anmeldedaten vor der Drosselung: Scheitert in Schritt 7 die Prüfung
vorgelegter Anmeldedaten (`401` außer `token_expired` und `use_dpop_nonce`, die normale Abläufe
sind), verbraucht die Anfrage ein Token aus `rl.public_auth` der Client-IP. Ist dieser Eimer leer,
antwortet Schritt 7 für Anfragen mit Anmeldedaten von dieser Client-IP mit `429`, ohne die
Anmeldedaten zu prüfen (05, Abschnitt 3). Grund: Jede Prüfung kostet eine Signaturprüfung, und ohne
diese Regel wäre sie unbegrenzt.

### 9.2 404 statt 403

Ein Anwendungsfall lädt die Ressource, bevor er `authorize` ruft, und gibt bei `not_found` denselben
Fehler zurück wie bei fehlender Ressource. Gleiche Antwort, gleicher Körper, gleiche Codepfade bis
zum Fehler (05, Test 4).

### 9.3 Nachweis der Prüfung

`kernel/authorize` protokolliert im Anfrage-Kontext jede Prüfung (Aktion, Art, Entscheidung); jede
Implementierung von `Authorizer` schreibt einen Eintrag je `Authorize` und `Scope`. Der
Antwort-Schreiber aus Schritt 5 prüft beim ersten `WriteHeader` bzw. `Write`: Bei Erfolg (Status
unter `400`) muss mindestens eine Prüfung mit der Aktion der Route stattgefunden haben; Listen
zählen über `Scope`. Fehlt sie, verwirft er Kopfzeilen und Körper des Handlers, sendet `500
internal_error` (Ursache `authz_not_checked` nur in Log und Metrik, 14.1), erhöht
`bothy.authz.errors` mit `bothy.reason = authz_not_checked`
([signals.md](../observability/signals.md)), und Tests schlagen fehl.
Antworten ab `400` gehen unverändert durch. Grund für den Ort: Die Strict-Middleware sieht ein
typisiertes Antwortobjekt, nicht den Status. Grund: B7 verlangt, dass jede Route
prüft. Die Lint-Regel in der Spec hält nur die Deklaration; dieser Nachweis hält die Ausführung.

The gateway upgrade `GET /api/v1/gateway` is the one route marked as authorized in-band
(`AuthorizedInBand`, M1-64): it passes only steps 1 to 4, because `identify` and `resume`
authenticate and authorize on the connection (R-SY-01). The same proof applies per op there: the
gateway dispatcher runs every op with its own check log and, after a successful handler, requires a
check of the op's action (05 "WebSocket-Ops und Aktion"); a missing check closes the connection with
`4000` and raises `bothy.authz.errors` with `bothy.reason = authz_not_checked`.

## 10. Gateway

Das Gateway ist Teil der Rolle `api`. Je Verbindung laufen eine Lese- und eine Schreib-Goroutine. Der
Schreiber liest aus einer begrenzten Warteschlange (`sync.outbound_queue_max`) und schreibt mit Frist
`gateway.write_timeout`. Läuft die Warteschlange über oder verpasst ein Schreibvorgang die Frist,
schließt der Server mit `4014` (04, 6.4 und 7.2). Kurzlebige Ereignisse verwirft er bei voller
Warteschlange (04, 7.2 Punkt 6).

Gateway-Sitzungen leben nur im Speicher des Knotens. Ein `resume` auf einem anderen Knoten endet mit
`4006`; der Client macht `identify` mit seinem Cursor und erhält dasselbe Nachholen (R-AR-11,
Diagramm [seq-3-wiederverbinden.svg](../architecture/seq-3-wiederverbinden.svg)). Grund: Ein
geteilter Sitzungsspeicher wäre ein weiterer Zustand; der Unterschied zwischen `resume` und
`identify` ist nur die Größe der Momentaufnahme und der wiederhergestellte Fokus.

Agenten-Verbindungen sind Verbraucher mit dauerhaftem Cursor (`agent_consumer_cursors`, 04 Abschnitt
15). Persönliche Aufruf-Ereignisse (`agent.invocation`, `agent.followup`, `interaction.created`,
`command.invoked`) gibt das Gateway vor dem Senden an `runs.Delivery`. Dort laufen `agent.invoke`
mit aktuellen Rechten, I2 und das Signieren der Aufruf-Marke (04, 15.3). Scheitert die Prüfung,
setzt `runs` den Lauf in einer eigenen Transaktion auf `failed` und das Gateway sendet nichts.
An agent consumer connection reads the log through its own runner, up to the hub position (the
highest `seq` the log tail of the process has passed), and is woken by the hub after each event
(M2-07).

## 11. Ereignis-Pfad

Diagramm: [fluss-ereignis-pfad.svg](../architecture/fluss-ereignis-pfad.svg).

### 11.1 Schreiben

Wie Abschnitt 8.1. `pg_notify` trägt nur die höchste `seq`, nie Inhalte. PostgreSQL stellt die
Benachrichtigung erst nach dem Commit zu [Q1, belegt].

### 11.2 Log-Tail je api-Prozess

- Genau ein Leser je Prozess (`sync.Tail`) mit eigener, nicht gepoolter Verbindung für `LISTEN
  event_log`. Er hält `local_head` und liest `event_log WHERE seq > local_head ORDER BY seq LIMIT
  sync.tail_batch_size`.
- Wecksignal ist `NOTIFY`; zusätzlich liest er alle `sync.log_poll_interval`. Geht die
  `LISTEN`-Verbindung verloren, verbindet er mit Backoff bis `database.listen_reconnect_backoff_cap` neu
  und liest ab `local_head`. Weil `seq` lückenlos ist, geht dabei nichts verloren.
- Er gibt Ereignisse in `seq`-Reihenfolge an den Rechte-Cache und dann an den Hub, in einer
  Goroutine. Grund: Reihenfolge ist nur dann einfach zu garantieren, wenn ein Leser sie erzeugt.

### 11.3 Rechte-Cache

Der Cache liegt in `authz`, je Prozess, je Prinzipal, mit Schlüssel `permission_version`. Der
Log-Tail meldet jedes `access.changed` über `Authorizer.Observe`, bevor der Hub spätere Ereignisse
verteilt (R-SY-02). Einträge tragen eine Gültigkeit bis zum frühesten Ablauf einer beteiligten
Freigabe oder Mitgliedschaft und werden danach neu berechnet. Strukturänderungen
(`channel.created`, `channel.updated` mit Schreibschutz, `channel.archived`, `space.archived`,
`thread.moved`) wendet der Cache ebenfalls in Log-Reihenfolge an; Archivieren bringt zusätzlich
eine neue Rechte-Version (04, Abschnitt 8.1). Das Nachholen filtert zusätzlich nach vollständig sichtbaren Bereichen, damit Ereignisse
neuer Kanäle nicht fehlen (04, Abschnitt 7.2).

### 11.4 Hub und Zustellung

- Der Hub hält Indizes: Kanal zu Verbindungen, Prinzipal zu Verbindungen, Thread zu fokussierten
  Verbindungen.
- Für jedes Ereignis bestimmt er die Kandidaten über die Indizes und ruft je Kandidat
  `authorize(empfänger, receive_action, ressource)` aus dem Cache. Erst dann legt er den Rahmen in
  die Warteschlange der Verbindung. Der Hub blockiert nie auf einer Verbindung.
- Verbindungen im Nachholen puffern Live-Ereignisse und senden sie nach `catchup.complete` (04,
  7.2).
- Agent consumer connections take no durable dispatch from the hub: each reads the log through its
  own runner up to the hub position and is woken by the hub after each event (M2-07).

### 11.5 Worker-Verbraucher (Push, Webhooks)

Folgen nach außen entstehen im worker aus dem Log, nicht im Schreibpfad (R-AR-08):

1. Je Verbraucher-Name (`push`, `webhooks`) gibt es eine Cursor-Zeile in `event_consumer_cursors`
   (03, [ADR 0051](../adr/0051-worker-verbraucher-des-ereignis-logs-mit-cursor.md)).
2. Eine Runde: Transaktion öffnen, Zeile mit `FOR UPDATE SKIP LOCKED` sperren (läuft schon ein
   anderer worker, endet die Runde sofort), bis zu `consumer.batch_size` Ereignisse ab Cursor lesen,
   je Ereignis Empfänger bestimmen und River-Jobs mit `InsertTx` einfügen, Cursor vorrücken,
   committen.
3. Wecksignal ist dasselbe `NOTIFY event_log`; zusätzlich alle `consumer.poll_interval`.
4. Jobs prüfen vor dem Versand erneut `push.deliver` bzw. `webhook_out.deliver` (05, R-AZ-11).

Grund gegen Job-Einfügen im Schreibpfad: Der Schreibpfad müsste jede Folge kennen; ein neuer
Verbraucher würde jeden Erzeuger ändern. Das Log ist schon der Ausgangskorb. Eine Runde erzeugt Jobs
genau einmal, weil Cursor und Jobs in einer Transaktion committen. `webhook_deliveries` hat zusätzlich
den eindeutigen Schlüssel `(subscription_id, event_seq)` (03).

Rules of the frame (M2-01, [DECISIONS.md](../DECISIONS.md) D-M2-01-1 to D-M2-01-9):

5. **Start at the head.** In the roles `worker` and `all` the start inserts, before the first
   round, the row of every registered consumer that has none, with `last_seq = max(seq)` of the
   log. A new consumer therefore never replays retained events, like the first connection of an
   agent consumer (04, 15.1 item 2). No migration inserts rows; names that are not registered have
   no row. When two workers start together, one row wins.
6. **Gaps.** When the events from `last_seq + 1` on are missing because retention detached their
   partition (the first number read equals `min(seq)`), the round processes from `min(seq)` and
   writes the error line `event consumer lost events to retention`; the events cannot come back,
   and a webhook subscription detects its own gap (11, section 6). Any other gap is a bug (04, 3.2
   items 3 and 4): the round rolls back, the cursor never passes the gap, the consumer stalls and
   the alarm reports it.
7. **Failures.** A handler error or panic, a database error and a gap roll back the whole round:
   no job, no cursor move, the next attempt at the next wake or poll. No event is ever skipped.
   The first failed round after a success writes `event consumer round failed`, the next success
   `event consumer recovered`. Consumers are independent; a stalled consumer holds back no other.
8. **No durable events in a round.** A handler that appends an event fails the round: the
   consumer would read its own output, and the round would take the lock of the counter
   `event_seq` (11.7).
9. **One `LISTEN` connection per process** for all consumers, apart from the log tail's
   (application name `bothy-consumer-wake`), reconnected with a backoff from
   `consumer.poll_interval` up to `database.listen_reconnect_backoff_cap`. After a full batch the
   next round starts at once. Every role exports `bothy.event.consumer.lag` and
   `bothy.event.consumer.pending_age` per consumer ([signals.md](../observability/signals.md) 4.2);
   the alarm `BothyEventConsumerLag` fires at `consumer.pending_alert_after`, which is strictly
   below `sync.event_retention` (13, item 4).

### 11.6 Kurzlebige Ereignisse

| Ereignis | Erzeugt in | Ein Knoten | Mehrere Knoten (`cluster.mode = multi`) |
|---|---|---|---|
| `typing.started` | `gateway` (Op `typing.start`) | direkt an den Hub | zusätzlich `NOTIFY ephemeral` mit Thread-ID und Prinzipal-ID |
| `run.updated` | `runs` (Patch gespeichert, Transaktion ohne `event_log`) | Hub sammelt je `sync.run_patch_coalesce_interval` | `NOTIFY ephemeral` mit Lauf-ID und Version; der andere Knoten lädt das Lauf-Dokument und sendet `run.updated` mit `snapshot` |
| `agent.presence_changed` | `agents` (Heartbeat) | direkt an den Hub | `NOTIFY ephemeral` mit Agent-ID und Zustand |
| Ablösung einer Geräte-Verbindung (`4017`, 04 Abschnitt 6.1) | `gateway` (`identify`, `resume`) | direkt an die ältere Verbindung | `NOTIFY ephemeral` mit Geräte-ID und Gateway-Sitzung |

Grund: R-Stack erlaubt `LISTEN/NOTIFY` nur als Wecksignal ohne Inhalt. Lauf-Inhalte gehen darum
nicht über `NOTIFY`; der Empfängerknoten liest sie aus der Datenbank. Der Kanal `ephemeral` ist nur
im Modus `multi` aktiv, weil jedes `NOTIFY` beim Commit eine instanzweite Sperre nimmt [Q2, belegt]
und ein Knoten ihn nicht braucht (03, Abschnitt 6.6). `notification.created` und
`command.autocomplete_requested` gehen denselben Weg mit IDs.

### 11.7 Kosten der Serialisierung

Alle Transaktionen mit dauerhaften Ereignissen serialisieren sich am Ende an `event_seq` (04, 3.2).
Die zusätzliche `NOTIFY`-Sperre beim Commit fällt in denselben Abschnitt und verlängert ihn
[abgeleitet aus Q2 und 04]. Transaktionen ohne Ereignis (Lauf-Patches, Heartbeat-Speicherung)
nehmen weder Zähler noch `NOTIFY`. Den Durchsatz hält das Ziel `perf.event_commit_min_per_s`
(Abschnitt 17).

## 12. Hintergrundjobs (River)

### 12.1 Einsatz

- River (MPL-2.0) nur in `internal/platform/jobs`; Module definieren Job-Argumente und
  Handler gegen eine eigene Schnittstelle von `platform/jobs`. Grund: River ist 0.x; eine
  Breaking Change betrifft dann ein Paket.
- Tabellen im eigenen Schema `jobs.schema`; Migration über `rivermigrate` in `bothy migrate`
  [Q7, belegt].
- Rolle `api` nutzt einen reinen Einfüge-Client (ohne `Start`, ohne Warteschlangen); er nimmt nicht
  an der Leader-Wahl teil [Q5, belegt]. Rolle `worker` führt Jobs, periodische Jobs und Wartung aus.
- Keine Pro-Funktionen (R-Stack). Für Abläufe mit mehreren Schritten verkettet ein Job den nächsten
  per `InsertTx`.
- Job-Argumente enthalten nur IDs, nie Inhalte oder Geheimnisse. Grund: `river_job` ist ein weiterer
  Speicherort; Löschung und Datenschutz bleiben bei den Fachtabellen. Enforced at registration:
  the argument type is a struct whose exported fields are only `kernel/ids.ID`, `[]kernel/ids.ID`
  or `int64` (sequence numbers and versions), so a string, byte slice, map, interface, pointer or
  nested struct fails the start (`jobs.ValidateArgsType`, M1-14).

### 12.2 Warteschlangen

| Warteschlange | Inhalt | Parallelität |
|---|---|---|
| `default` | kurze Fachjobs | `jobs.queue.default.max_workers` |
| `files` | Scan, Vorschaubilder, Transkription (CPU, Speicher) | `jobs.queue.files.max_workers` |
| `delivery` | Push, Webhooks (Netz) | `jobs.queue.delivery.max_workers` |
| `maintenance` | Aufräumen, Partitionen, Prüfungen | `jobs.queue.maintenance.max_workers` |
| `privacy` | Export und Löschung | `jobs.queue.privacy.max_workers` |

Grund für getrennte Warteschlangen: Viele Vorschaubilder oder ein großer DSGVO-Export dürfen Push
und Webhooks nicht verzögern. Importe laufen nicht als Job im Server, sondern im Werkzeug
`bothy import` über die API ([22-import-migration.md](22-import-migration.md), Abschnitt 2).

Im Wartungsmodus (`server.maintenance_mode = read_only`,
[ADR 0065](../adr/0065-wartungsmodus-read-only.md)) arbeitet der `worker` nur die Verbraucher
`push` und `webhooks` ([ADR 0051](../adr/0051-worker-verbraucher-des-ereignis-logs-mit-cursor.md)),
die Warteschlange `delivery` und den Job `runs.cancel_timeout` ab; ihre Transaktionen sind von der
lesenden Unit of Work ausgenommen. Alle anderen Warteschlangen und periodischen Jobs setzen aus und
laufen erst nach dem Neustart ohne Modus weiter, beim Umzug auf dem Ziel ab dem kopierten Stand.
Grund: Jeder andere Job schriebe Zustand, der auf dem Ziel fehlen würde.

### 12.3 Job-Katalog

| Job | Modul | Auslöser | Warteschlange | Fachregel in |
|---|---|---|---|---|
| `files.scan` | `files` | `InsertTx` beim Abschluss eines Uploads | `files` | 12 |
| `files.thumbnail` | `files` | nach `clean` | `files` | 12 |
| `files.extract_text` | `files` | nach `clean`, unterstützter Typ | `files` | 14 |
| `files.transcribe` | `files` | `file.transcribe` bei Server-Transkription | `files` | 09, 12 |
| `files.cleanup_pending` | `files` | periodisch | `maintenance` | 12 |
| `files.purge_objects` | `files` | periodisch, gelöschte Dateien | `maintenance` | 12, 20 |
| `notify.push_send` | `notify` | Verbraucher `push` | `delivery` | 13 |
| `apps.webhook_deliver` | `apps` | one attempt of a subscription (args: the subscription only, M2-26): a round of the consumer `webhooks`, the chain after a finished delivery, a scheduled retry, the lease wake-up after `webhook.delivery_timeout`, an owner resume or re-enable, `webhook_out.redeliver` and `app.resume`; jobs of one subscription serialize on its row lock | `delivery` | 11 |
| `runs.cancel_timeout` | `runs` | periodisch, `agent.cancel_grace` | `maintenance` | 09 |
| `runs.stall_check` | `runs` | periodisch, `run.stall_after` | `maintenance` | 09 |
| `runs.component_expiry` | `runs` | periodisch, `component.default_ttl` | `maintenance` | 09 |
| `runs.invocation_hold_timeout` | `runs` | terminiert, `file.invocation_hold_max` bzw. `voice.transcript_hold_max` | `default` | 04, 12 |
| `authz.expiry_reminder` | `authz` | periodisch, `access.expiring` vor Ablauf | `maintenance` | 07 |
| `authz.expiry_sweep` | `authz` | periodisch, `access.changed` nach Ablauf (Systemaktionen `grant.expire`, `group.member.expire`) | `maintenance` | 07 |
| `identity.auth_cleanup` | `identity` | periodisch alle `auth.cleanup_interval`: abgelaufene Zeilen von `auth_challenges`, `auth_login_flows` und `device_pairings` löschen; wartende Geräte über `auth.device_pending_ttl`, nie abgeholte gekoppelte und ungenutzte Geräte über `auth.device_inactive_revoke_after` widerrufen (Systemaktion `device.expire`); die Entscheidung je Anfrage hängt nie am Job | `maintenance` | 08, 20 |
| `instance.key_rotation` | `instance` | periodisch, Zustände `next` bis `retired` | `maintenance` | 19 |
| `sync.partition_maintenance` | `sync` | periodisch alle `sync.partition_maintenance_interval` und beim Start des worker: `sync.event_log_partitions_ahead` Partitionen vor dem Kopf anlegen, nach `sync.event_retention` abhängen, nie die Partition mit dem Kopf (03 `event_log`); `bothy migrate up` ruft dieselbe Routine | `maintenance` | 03, 04 |
| `sync.idempotency_cleanup` | `sync` | periodisch alle `sync.idempotency_cleanup_interval`, löscht abgelaufene Zeilen in Stapeln von `sync.idempotency_cleanup_batch_size` | `maintenance` | 04 |
| `audit.chain_verify` | `audit` | periodisch alle `audit.checkpoint_interval`: den Abschnitt seit dem letzten Prüfpunkt nachrechnen, erst dann den nächsten Prüfpunkt signieren, speichern und an `audit.checkpoint_sink` ablegen; ein Bruch endet ohne Prüfpunkt und ohne Wiederholung (nie eine ungeprüfte Kette signieren) | `maintenance` | 19 |
| `audit.retention_trim` | `audit` | periodisch, eigene Wartungsrolle der Datenbank | `maintenance` | 20 |
| `privacy.export`, `privacy.erase` | `privacy` | Freigabe eines Auftrags | `privacy` | 20 |
| `billing.reconcile` | `billing` | `cost.reconcile` (von Hand) und `cost.reconcile.submit` (Abgleich-App): übermittelte Einträge anwenden, Summen an Läufen und Sitzungen nachziehen; nie periodisch, weil der Server keine Abrechnung abruft und keinen Admin-Schlüssel hält ([ADR 0055](../adr/0055-kostenabgleich-als-eigene-app-admin-key-nie-im-server.md)) | `default` | 10 |
| `status.probe` | `status` | periodisch | `maintenance` | 16 |
| `retention.<klasse>` | besitzendes Modul | periodisch je Aufbewahrungsklasse | `maintenance` | 20 |
| `stats.retention` | `stats` | periodisch: Tages-Partitionen von `stats_minutes` im Voraus anlegen, Partitionen älter als `retention.stats_minutes` abhängen; ersetzt für die Klasse `stats` den allgemeinen `retention.<klasse>`, weil die Tabelle partitioniert ist | `maintenance` | 25 |
| `terminal.sweep` | `terminal` | periodisch alle `terminal.sweep_interval`: abgelaufene Kopplungs-Codes verwerfen (`pairing_expires_at`); Sitzungen beenden, die `terminal.detach_grace`, `terminal.max_session_duration` oder `terminal.idle_timeout` plus `terminal.idle_grace` überschritten haben (`terminal.terminate`). Rückfall für die Prüfung im Relais, falls ein Knoten ausfiel | `maintenance` | 24 |

### 12.4 Regeln für Jobs

1. Jeder Job ist wiederholbar ohne doppelte Wirkung (Prüfung des Zustands vor der Wirkung, eindeutige
   Schlüssel wie `(subscription_id, event_seq)`).
2. Jeder Job reicht seinen Kontext weiter und gibt bei Abbruch `ctx.Err()` zurück; ein `nil` gilt in
   River als Erfolg [Q4, belegt].
3. Externe Aufrufe haben eine Frist (`egress.timeout` oder einen eigenen Namen des
   Fachkapitels).
4. Wiederholung mit Backoff übernimmt River; Fachkapitel mit eigener Regel (Webhooks:
   `webhook.retry_backoff_base`, `webhook.retry_backoff_cap`) setzen eine eigene Wiederholungs-Regel.
5. Sonst gelten die Standardwerte der gepinnten River-Version für Job-Frist, Abrufintervall,
   Wiederholungs-Backoff und Höchstzahl der Versuche; das README von `platform/jobs` nennt sie mit
   der Version. Ein Job mit anderem Bedarf setzt einen benannten Wert seines Fachkapitels. Grund:
   Ein eigener Name ohne Bedarf wäre eine zweite Quelle für denselben Wert. One River default is
   off: the nightly reindexer, because `bothy_app` owns no index of River and the reindex could
   only fail (16 "Rollen"; docs/DECISIONS.md M1-14).
6. SQL in Jobs läuft im selben Unit of Work wie im Anfragepfad, also mit `database.statement_timeout`
   und `database.lock_timeout`. Lange Arbeit teilt ein Job in Stapel (Regel 1). Migrationen setzen
   nur `migrate.lock_timeout` je Schritt ([upgrade.md](../operations/upgrade.md), Regel M6).

## 13. Konfigurationsladen

Grundlage ist ADR 0011 (12-Factor, Secret-Provider; Team Betrieb). Dieses Kapitel legt fest, wie der
Server lädt.

1. **Zwei Arten von Werten.**
   - *Konfiguration* (Betreiber): Adressen, Grenzen, Fristen, Treiber, Geheimnis-Verweise. Gilt ab
     Start, Änderung braucht Neustart.
   - *Einstellungen* (Owner): `instance_settings` und `feature_flags` in der Datenbank. Änderung über
     die API mit Audit, sofort wirksam, Ereignis `instance.updated`. Grund für die Trennung:
     Infrastruktur gehört dem Betreiber, Fachliches dem Owner, und nur Einstellungen brauchen eine
     Rechteprüfung.
2. **Quellen, Vorrang von niedrig nach hoch:** eingebaute Standardwerte (im Schema,
   `internal/wire/configkeys_*.go`, gleich der Kapiteltabelle des Schlüssels) < Datei (`--config`
   oder `BOTHY_CONFIG_FILE`, YAML, verschachtelt wie die Namen) < Umgebung (`BOTHY_` plus Name in
   Großbuchstaben, `.` wird `__`, Beispiel `sync.event_retention` wird
   `BOTHY_SYNC__EVENT_RETENTION`) < Kommandozeile (nur `--role` und `--config`). Die Abbildung ist
   umkehrbar (ADR 0011, [16-betrieb.md](16-betrieb.md) 5.1).
3. **Geheimnisse** nur als Verweis in Schlüsseln mit Endung `_ref` (z. B. `database.password_ref`). Der
   Secret-Provider löst sie beim Start auf. Werte erscheinen nie in Logs, Fehlertexten oder
   `bothy config print` (dort steht nur der Verweis; nur `bothy config check --secrets` löst auf
   und nennt je Schlüssel `ok` oder den Fehlercode). Der Provider entsteht nur in `internal/wire`
   und `cmd/bothy`; Module bekommen Werte über ihre `Config` oder eine enge Schnittstelle und lesen
   weder Dateien noch Umgebung selbst.
4. **Prüfung beim Start, fail fast:** unbekannte Schlüssel und unbekannte `BOTHY_`-Variablen
   brechen ab (Tippfehler sind sonst still; Ausnahmen in [16-betrieb.md](16-betrieb.md) 5.1);
   Typen und Bereiche; Querbedingungen. Querbedingungen heute: `http.max_json_body_bytes` echt
   größer als `message.max_content_bytes` (der Umschlag einer Nachricht ist klein gegen den
   Abstand der Standardwerte; ein eigener Wert für ihn wäre ohne Messung geraten);
   `server.internal_listen_addr` und `server.listen_addr` teilen keinen Port außer `0`, wenn ihre
   Hosts gleich sind oder einer der Hosts leer, `0.0.0.0` oder `::` ist (auch mit IPv6-Zone; eine
   IPv4-gemappte IPv6-Adresse zählt als ihre IPv4-Adresse). Port `0` darf doppelt vorkommen, weil
   Tests Port 0 binden. `consumer.pending_alert_after` is strictly below `sync.event_retention`
   (cross-check `consumer_alert_below_retention`, M2-01), so the alarm `BothyEventConsumerLag`
   fires while a stalled worker consumer can still catch up. `webhook.retry_backoff_base` is at
   most `webhook.retry_backoff_cap` (cross-check `webhook_backoff_order`, M2-26), and
   `webhook.retry_max_age` is strictly below `sync.event_retention` (cross-check
   `webhook_retry_within_retention`, M2-26), because a retry needs its event in the log. With `file.storage.driver = fs`, `file.fs.root` is an absolute path in
   clean form (no `.` or `..` element, no trailing `/`; cross-check `file_fs_root_absolute`,
   M3-01), because a relative root would depend on the working directory of the process; with
   `s3` the root is not read. With `file.storage.driver = s3` the cross-check `file_s3` (M3-02)
   holds `file.s3.endpoint` (set, an absolute `http` or `https` URL with a host and without user
   info, query, fragment or a path other than `/`), `file.s3.bucket` (set, S3 bucket naming
   rules), `file.s3.region` (set), `file.s3.prefix` (empty or the key grammar of
   [12-dateien.md](12-dateien.md) 4.1), `file.s3.use_path_style` (true when the endpoint host is
   an IP address), `file.s3.checksum_mode` (`when_supported` only with an `https` endpoint) and `egress.allowed_hosts` (an `http` endpoint needs an entry with
   `allow_private` for the request host and port, because the egress client allows `http` only
   there, [19-sicherheit.md](19-sicherheit.md) SEC-NET-2); with `fs` no `file.s3.*` value is
   read. The entries of `search.languages` are unique, unqualified lower-case PostgreSQL
   identifiers and never `simple` (check `search_languages`); every key of `search.locale_map` is
   a first language subtag in lower-case ASCII letters and every value is listed in
   `search.languages` or is `simple` (check `search_locale_map`, the start error of
   [14-suche.md](14-suche.md) 3.1; M3-13). Dazu prüft der Lader eine Regel für einen einzelnen Schlüssel, die kein
   Typ ausdrückt: `secrets.dir` ist ein absoluter Pfad in sauberer Form (kein `.` oder `..`, kein
   doppelter oder abschließender `/`), nicht `/`, ohne NUL-Zeichen und ohne `<`, wie ihn der
   Secret-Provider verlangt; ein falscher Wert ist ein Problem von `secrets.dir` allein, nicht der
   Verweise, die `<secrets.dir>` nutzen. Ebenso sind `server.public_url` und jeder Eintrag von
   `api.cors_allowed_origins` ein Origin (Schema `http` oder `https`, Host, optional Port, kein
   Pfad, keine Query, kein Fragment; M1-18), weil die Kette das DPoP-`htu` aus diesem Origin baut
   und `Origin` genau vergleicht. Die Regel "Identitäts-Header nur mit Loopback-Listener"
   ist keine Konfigurationsprüfung, weil der Treiber eine Einstellung des Owners in der Datenbank
   ist; sie gilt bei der Bereitschaft ([16-betrieb.md](16-betrieb.md) 4.3). Meldungen nennen
   Schlüssel und Quelle, nie den Wert.
5. **Typisiert je Modul.** Jedes Modul und jedes Plattform-Paket hat eine eigene `Config`-Struktur
   im eigenen Paket; `internal/wire` füllt sie aus `platform/config` (Bindung über Struktur-Tags).
   Module lesen nie Umgebungsvariablen. Ein Schlüssel kommt mit dem Ticket ins Schema, das seinen
   ersten Nutzer baut; bis dahin führt ihn die Konfigurationsreferenz als "geplant" mit Etappe.
   Grund: Testbarkeit, eine Stelle für Prüfungen und kein Schlüssel ohne Wirkung.
6. **Kein Nachladen im Betrieb** außer Geheimnis-Rotation (Signierschlüssel, Webhook-Geheimnisse),
   die der Secret-Provider meldet. Grund: Vorhersagbares Verhalten; halb übernommene Grenzen sind
   schwerer zu diagnostizieren als ein Neustart.
7. `bothy config check` führt Schritte 2 bis 4 aus, ohne zu starten, und schreibt je Schlüssel
   `*_ref`, dessen wirksamer Verweis `env:` nutzt, eine Warnung auf die Standardfehlerausgabe; mit
   `--secrets` löst es zusätzlich jeden Verweis `*_ref` über den Secret-Provider auf (Zeile
   `secret <schlüssel>: ok` oder `secret <schlüssel>: <fehlercode>`, Exitcode 1, wenn einer
   scheitert). `bothy config print` zeigt die wirksamen Werte mit Quelle je Schlüssel.

## 14. Fehler- und Timeout-Strategie

### 14.1 Fehlerarten

`kernel/errs` kennt diese Arten. Jeder Fehler trägt einen stabilen öffentlichen `code` aus der
geschlossenen Liste in [06-api.md](06-api.md), Abschnitt 5; die Art folgt aus dem Code, nie aus dem
Aufrufer. `type`-URIs und Körper nach RFC 9457 legt 06 fest (ADR 0015). Das Gateway nutzt dieselben
Codes in der Op `error`.

| Art | HTTP | Codes aus 06 (Beispiele) |
|---|---|---|
| `bad_request` | `400` | `validation_failed`, `malformed_request`, `cursor_invalid` |
| `unauthenticated` | `401` | `unauthenticated`, `token_expired`, `invalid_dpop_proof` |
| `step_up_required` | `401` | `step_up_required` |
| `forbidden` | `403` | `forbidden`, `scope_missing`, `channel_read_only` |
| `not_found` | `404` | `not_found` (auch ohne Leserecht) |
| `method_not_allowed` | `405` | `method_not_allowed` |
| `conflict` | `409` | `conflict`, `already_started`, `request_in_progress` |
| `invariant_violation` | `409` | `i1_violation`, `last_owner` |
| `gone` | `410` | `message_deleted`, `resync_required` |
| `precondition_failed` | `412` | `version_conflict` |
| `too_large` | `413` | `payload_too_large` |
| `unsupported_media_type` | `415` | `unsupported_media_type` |
| `range_not_satisfiable` | `416` | `range_not_satisfiable` |
| `unprocessable` | `422` | `idempotency_key_reuse` |
| `precondition_required` | `428` | `precondition_required` |
| `rate_limited` | `429` | `rate_limited` mit `Retry-After` |
| `internal` | `500` | `internal_error` |
| `unavailable` | `503` | `unavailable`, `maintenance`, immer mit `Retry-After` (`http.unavailable_retry_after`) |

Gründe einer Ablehnung (`level_insufficient`, `capability_missing` usw., 07 Abschnitt 4.1) sind
keine Codes: Sie stehen im Audit und, wo 06 es vorsieht, in der Erweiterung `reason`; der Code ist
`forbidden`. Interne Ursachen sind Bezeichner für Log und Metrik, nie Teil einer Antwort:
`authz_not_checked`, `unknown_action` (beide `500 internal_error`), `db_pool_exhausted`,
`shutting_down`, `deadline_exceeded`, `dependency_unavailable` (alle `503 unavailable`). Grund:
06 hält die geschlossene Menge öffentlicher Codes, und interne Zustände gehören nicht in Antworten.

Interne Details (SQL-Fehler, Stack) stehen nur im Log mit `correlation_id`, nie in der Antwort.

### 14.2 Fristen

| Ort | Frist | Grund |
|---|---|---|
| Kopfzeilen lesen | `http.read_header_timeout` | Schutz gegen langsame Clients |
| normale Anfrage (Kontext) | `http.request_timeout` | begrenzt Wartezeit und Ressourcen |
| Upload und Download (über `ResponseController`) | `http.transfer_timeout` | große Dateien |
| Leerlauf Keep-Alive | `http.idle_timeout` | Verbindungen freigeben |
| Verbindung aus dem Pool holen | `database.acquire_timeout` | Überlast wird `503`, nicht Warteschlange |
| SQL-Anweisung | `database.statement_timeout` (je Transaktion per `SET LOCAL`) | kein Abfrage-Ausreißer blockiert |
| Sperre warten | `database.lock_timeout` | Zähler-Sperren nie unbegrenzt |
| offene Transaktion ohne Arbeit | `database.idle_in_tx_timeout` | schützt die serialisierte Commit-Phase |
| WebSocket schreiben | `gateway.write_timeout` | langsame Leser blockieren nicht |
| Aufrufe nach außen | `egress.timeout` ([19-sicherheit.md](19-sicherheit.md)) oder Name des Fachkapitels | jeder Netzaufruf hat eine Frist |

Der Kontext eines abgebrochenen Clients bricht die Transaktion ab. Hat sie schon committet, deckt die
Idempotenz die Wiederholung des Clients.

### 14.3 Wiederholungen

- Im Anfragepfad: nur der ganze Unit of Work bei `40P01` und `55P03` (Abschnitt 8.2). Nach
  `database.tx_retry_max` Wiederholungen endet er mit Fehlerart `unavailable` (`503 unavailable`
  mit `Retry-After`); der Datenbankfehler steht nur im Log. Keine Wiederholung externer Aufrufe im
  Anfragepfad (es gibt dort keine).
- In Jobs: River mit Backoff; Fachkapitel können eigene Regeln setzen.
- Clients wiederholen nach 04 (Ausgangsfach, Full Jitter).

### 14.4 Ausfall einer Abhängigkeit

| Abhängigkeit | Verhalten | Art |
|---|---|---|
| PostgreSQL | api meldet nicht bereit, Anfragen `503`; Gateway hält Verbindungen, liefert nichts; worker pausiert; Rückkehr ohne Eingriff | fail closed |
| `LISTEN`-Verbindung | Neuaufbau mit Backoff, Poll überbrückt; kein Verlust (lückenlose `seq`) | weich |
| Objektspeicher | Upload und Download `503`; Nachrichten ohne Dateien laufen weiter | teilweise |
| clamd | Scan-Jobs wiederholen; Dateien bleiben `pending` und werden nicht ausgeliefert | fail closed |
| Push-Dienste | Jobs wiederholen; Nachrichtenpfad unberührt | weich |
| Webhook-Ziele | Wiederholung bis zur Grenze, dann Abo `disabled` (03) | weich |
| OTel Collector | Daten verwerfen mit Zähler; nie blockieren (R-Telemetrie) | fail open |
| Identitätsanbieter | neue Anmeldung unmöglich; bestehende Sitzungen laufen weiter | teilweise |
| Secret-Provider | Start bricht ab; Rotation scheitert: alter Wert bleibt, Alarm | fail fast |
| Transkriptions-Dienst | Job wiederholt, danach `transcript_status = failed` | weich |

### 14.5 Panics

Je Anfrage, je Job und je Verbindung abgefangen: Anfrage `500`, Job als Fehler an River, Verbindung
mit `4000` geschlossen. A panic in the handler of a worker consumer becomes the error of its round,
which rolls back (11.5 item 7); a panic in the frame of the consumers itself ends the process. Ein Panic im Log-Tail oder Hub beendet den Prozess mit Fehlercode, weil ein
Prozess ohne Fan-out nicht bereit wäre, aber noch Verbindungen hielte. Der Orchestrator startet neu,
Clients nehmen nach 04 wieder auf.

## 15. Start und Graceful Shutdown

### 15.1 Start

1. Konfiguration laden und prüfen (Abschnitt 13), Telemetrie starten.
2. Geheimnisse auflösen. Der interne Listener öffnet jetzt, damit der Orchestrator `/healthz`
   sieht und `/readyz` mit `startup` scheitert, solange der Start läuft. Every role also checks the
   key store `secrets.signing_key_dir` (owned by the service user, mode `0700`, writable) and ends
   the start otherwise (ADR 0066, M1-32).
3. Datenbank erreichen, höchstens `startup.dependency_wait` lang mit Backoff; danach Abbruch.
4. Schema-Version prüfen: Das Binary kennt die kleinste nötige Version. Ist die Datenbank älter,
   bricht der Start mit Hinweis auf `bothy migrate up` ab (Exitcode `1`). Neuere Versionen sind
   erlaubt, weil Migrationen nach Erweitern und Zusammenziehen gebaut sind (ADR 0007). River's
   migrations in `jobs.schema` must be complete as well; otherwise the start aborts with the same
   hint (checked in step 8 for every role, M1-14).
   Then the text search of the database (M3-13, [14-suche.md](14-suche.md) 3.1): every
   configuration of `search.languages` and `search.locale_map` (plus `simple`) must exist on the
   search path, and the database must fold the case of non-ASCII letters
   ([16-betrieb.md](16-betrieb.md) 7.2); otherwise the start aborts with exit code `1` before any
   public listener, and the message names the key and the entry, never the configured name.
5. Instanz laden (fehlt die Zeile: Abbruch mit Hinweis auf `bothy migrate up`). Ohne
   `setup_completed_at` startet der Server vollständig, beantwortet aber nur Health-Checks und
   Discovery, alles andere mit `503 setup_required` (08, Abschnitt 4); er prüft den Zustand je
   Anfrage, bis er gesetzt ist, und braucht nach der Einrichtung keinen Neustart. `/readyz` hängt
   nicht an der Einrichtung, sonst öffnete ein Orchestrator den Weg zum Einlösen des Codes nie.
   Verlangt
   `instance.min_server_version` eine neuere Version als die eigene, läuft der Prozess weiter,
   wird aber nie bereit: `/readyz` scheitert mit `schema_requires_newer_server`.
6. Signierschlüssel sicherstellen: Jeder Zweck ohne `active`-Schlüssel bekommt einen
   ([19-sicherheit.md](19-sicherheit.md), Abschnitt 10; [ADR 0066](../adr/0066-signierschluessel-im-beschreibbaren-schluesselspeicher.md)).
   Keys whose private file is missing from the key store (a restore without it) are retired
   first, so the purpose gets a fresh active key; this runs after audit, sync, authz and identity
   (steps 6a to 6d of `internal/wire`), whose unit of work records `signing_key.transition`.
7. Routen registrieren und Routen-Metadaten prüfen: Jede Operation hat eine bekannte Aktion, sonst
   Abbruch (05, Regel 3).
8. Rolle `api`: Log-Tail starten, `local_head` auf den aktuellen Kopf, öffentlichen Listener
   öffnen. Rolle `worker`: River und Verbraucher starten.
9. Bereit melden.

Grund für Abbruch bei zu altem Schema, aber "nicht bereit" bei `min_server_version`: Ein Binary
vor einem zu alten Schema findet seine Tabellen nicht; weiterlaufen hätte keinen Nutzen. Ein zu
altes Binary vor einem neueren Schema (Rückweg über mehr als eine Version) könnte laufen, darf es
aber nicht; als nicht bereiter Prozess zeigt es den Grund auf `/readyz` und hält den Orchestrator
nicht in einer Neustart-Schleife (R-AR-16).

### 15.2 Health

Auf dem internen Listener `server.internal_listen_addr`, nicht in der öffentlichen API
(R-AR-13, Ruling L2); Pfade und Fehlercodes legt [16-betrieb.md](16-betrieb.md), Abschnitt 4.2 fest:

- `/healthz` (lebt): Der Prozess antwortet. Fällt der Log-Tail, der Hub (api) oder der
  River-Client (worker) unerwartet aus, beendet sich der Prozess mit Exitcode `1` (Abschnitt
  14.5); darum braucht `/healthz` keine eigene Prüfung dafür. The same holds for the runner of the
  worker consumers (worker, M2-01); a failing round is no such end (11.5 item 7).
- `/readyz` (bereit): Start abgeschlossen (`startup`), Datenbank erreichbar (`database`), Version
  zum Schema passend (`schema_requires_newer_server`), je Zweck ein aktiver Signierschlüssel
  (`signing_keys`), kein Shutdown begonnen (`shutting_down`). Weitere Prüfungen bringen die
  Fachkapitel mit (z. B. `trusted_header_requires_loopback`, 16 Abschnitt 4.3). Antwortformat:
  16, Abschnitt 4.2.

Grund für den eigenen Listener: Health braucht keine Anmeldung und keine Aktion, darf aber nicht
öffentlich erreichbar sein. Darum hat Health keinen Eintrag im Aktionskatalog (05, R-AZ-22).

### 15.3 Shutdown

Signale `SIGTERM` und `SIGINT` starten ihn, ein zweites Signal beendet sofort (Exitcode `3`).
Gesamtfrist `shutdown.total_timeout`; jeder Schritt läuft höchstens mit seinem eigenen Budget und
dem Rest der Gesamtfrist. Beim Verdrahten darf die Summe der Schritt-Budgets
(`shutdown.drain_delay`, `shutdown.http_grace`, `shutdown.jobs_soft_timeout`,
`shutdown.telemetry_flush_timeout`) `shutdown.total_timeout` nicht übersteigen, sonst bricht der
Start ab (Exitcode `2`). Der Orchestrator muss länger warten: Compose setzt `stop_grace_period`
größer als `shutdown.total_timeout` ([infra/compose/compose.yaml](../../infra/compose/compose.yaml),
ein Test hält es).

Rolle `api`:

1. `ready` auf falsch, `shutdown.drain_delay` warten, damit der Proxy keine neuen Anfragen schickt.
2. `http.Server.Shutdown` mit `shutdown.http_grace`: Listener schließen, laufende Anfragen beenden.
   `Shutdown` schließt übernommene Verbindungen wie WebSockets nicht [Q9, belegt]; darum:
3. Gateway: `acked_seq` aller Agenten-Verbraucher sofort speichern; alle Verbindungen mit `4013
   server_restart` schließen (04, 6.4; Clients verbinden mit Zufallsverzögerung neu).
4. Log-Tail und `LISTEN` beenden.
5. Telemetrie leeren, höchstens `shutdown.telemetry_flush_timeout`.
6. Pool schließen.

Rolle `worker`:

1. `ready` auf falsch.
2. Verbraucher beenden ihre Runde (Commit oder Rollback) und starten keine neue. The step has no
   own budget, only the rest of `shutdown.total_timeout`; after it the round is cancelled and
   rolls back.
3. River: weicher Stopp mit `SoftStopTimeout` gleich `shutdown.jobs_soft_timeout`, danach werden
   Job-Kontexte abgebrochen [Q3, Q4, belegt]; abgebrochene Jobs liefern einen Fehler und werden
   wiederholt.
4. Telemetrie leeren, Pool schließen.

Rolle `all`: api-Schritte 1 bis 4, dann worker-Schritte 2 und 3, dann Telemetrie und Pool. Grund:
Zuerst keine neuen Anfragen und Ereignisse annehmen, dann laufende Jobs beenden.

## 16. Skalierungspfad

| Stufe | Aufbau | Auslöser (messbar) | Was sich ändert |
|---|---|---|---|
| 0 | ein Prozess `--role=all`, PostgreSQL und Objektspeicher daneben | Standard | nichts |
| 1 | api und worker getrennt, eine Datenbank | Jobs verletzen `perf.message_create_p99`, oder Speicher der Jobs stört den api-Prozess | nur Rolle je Container |
| 2 | mehrere api-Knoten hinter dem Proxy | ein Knoten erreicht `perf.ws_connections_per_node` oder seine CPU; rollende Updates ohne Ausfall | `cluster.mode = multi`: Kanal `ephemeral`, Fencing der Agenten-Verbraucher, Rate-Limits je Knoten, Migrationen strikt nach Erweitern und Zusammenziehen |
| 3 | mehrere worker | Job-Rückstand wächst dauerhaft (Kennzahl in 17) | nichts im Code: River verteilt, Verbraucher sperren mit `SKIP LOCKED`, periodische Jobs über den River-Leader |
| 4 | Lese-Replikat für Nachholen, Suche, Statistik | CPU der Datenbank | Lesepfade wählen die Verbindung; `head_seq` und Seiten aus derselben Quelle; Schreiben und Aufruf-Zustellung bleiben auf dem Primärsystem |
| 5 | ein Modul abspalten | Kriterien unten | neue Rolle oder eigener Dienst |

Mehrere Knoten im Detail (Stufe 2):

- Ereignis-Log: Jeder Knoten hat seinen Log-Tail; das `NOTIFY` weckt alle. Nichts weiter nötig.
- Gateway-Sitzung: `resume` auf fremdem Knoten wird `identify` (Abschnitt 10).
- Agenten-Verbraucher: Ein neues `identify` auf Knoten B muss die alte Verbindung auf Knoten A beenden
  (`4015`). Knoten B schreibt beim Übernehmen seine Gateway-Sitzungs-ID in die Cursor-Zeile; Knoten A
  stellt beim nächsten Speichern von `acked_seq` fest, dass die Zeile nicht mehr ihm gehört, und
  schließt mit `4015` (Fencing über `agent_consumer_cursors.holder_session_id`, 03). Ein Signal über `NOTIFY ephemeral` beschleunigt
  das. Bis dahin verhindern Ereignis-ID und Lauf-Start per Compare-and-swap doppelte Arbeit (04,
  15.3).
- Rate-Limits: Eimer im Prozess. Bei `n` Knoten gilt bis zu `n`-fach; der Betreiber teilt die Werte
  oder setzt Affinität am Proxy. Grund: kein gemeinsamer Zähler-Dienst.
- Präsenz und Tippen: über `NOTIFY ephemeral` (Abschnitt 11.6).

Ein Modul wird erst abgespalten, wenn alle drei Bedingungen gelten:

1. Es hat einen Ressourcen- oder Ausfallbedarf, den eine eigene Rolle desselben Binarys nicht löst.
2. Es teilt keine Schreib-Transaktion mit anderen Modulen; es spricht nur über Ports und das
   Ereignis-Log.
3. Ein Messwert aus Abschnitt 17 oder 17-observability belegt den Bedarf.

Kandidaten sind die Dateiverarbeitung von `files` (zuerst als eigene Rolle) und `importer`. Keine
Kandidaten sind `chat`, `messaging`, `runs`, `authz`, `sync`, weil sie Transaktionen teilen. Die
Grenze des Musters ist der globale Zähler `event_seq`: Reicht `perf.event_commit_min_per_s` nicht,
braucht es eine neue Entscheidung (z. B. Sequenzen je Bereich) mit eigenem ADR.

## 17. Leistungsziele

Ziele sind benannte Werte (Abschnitt 22). Jedes Ziel hält ein Test in `make perf`; bis es ihn gibt,
ist das Ziel "ungeprüft" (Tickets). Wann und auf welcher Umgebung gemessen wird, regelt [ADR 0063](../adr/0063-leistungs-gate-fuer-das-ereignis-log.md).

| Name | Was gemessen wird | Test |
|---|---|---|
| `perf.reference_profile` | Umgebung, in der die Ziele gelten | alle Lasttests laufen darauf |
| `perf.message_create_p99` | Server-Zeit von `message.create` (ohne Anhang) unter `perf.sustained_messages_per_s` | `perf/load_messages` |
| `perf.event_commit_min_per_s` | dauerhafte Ereignisse je Sekunde mit dem Zähler-Muster | `perf/event_commit` |
| `perf.fanout_local_p99` | Commit bis `dispatch` an verbundenen Client, gleicher Knoten | `perf/gateway_fanout` |
| `perf.fanout_cross_node_p99` | dasselbe über zwei api-Knoten | `perf/gateway_fanout_multi` |
| `perf.invocation_delivery_p99` | Commit der auslösenden Nachricht bis `agent.invocation` beim Fake-Adapter | `perf/invocation` |
| `perf.run_patch_fanout_p99` | `run.update` des Adapters bis `run.updated` beim fokussierten Client | `perf/run_patch` |
| `perf.catchup_page_p99` | eine Nachhol-Seite (`sync.catchup_page_size`) inklusive `authorize` je Ereignis | `perf/catchup` |
| `perf.authorize_cached_p99` | `authorize` aus dem warmen Cache | Benchmark `BenchmarkAuthorizeCached` |
| `perf.ws_connections_per_node` | gleichzeitige Verbindungen eines api-Knotens mit Heartbeats | `perf/ws_soak` |
| `perf.ws_idle_conn_mem_max` | Speicher je ruhender Verbindung | `perf/ws_soak` |
| `perf.idle_rss_max` | Speicher eines Prozesses `all` ohne Last | `perf/startup` |
| `perf.startup_ready_max` | Start bis `ready` ohne Migrationen | `perf/startup` |
| `perf.push_handoff_p95` | Commit bis Übergabe an den Push-Dienst (Fake) | `perf/push_handoff` |

## 18. Module und Kapitel 06 bis 23

| Kapitel | Inhalt | Module |
|---|---|---|
| [06-api.md](06-api.md) | Konventionen, Endpunkte, Ops, Codegenerierung | alle `httpapi`-Adapter, `gateway`, `api/` |
| [07-rechte.md](07-rechte.md) | Freigabe-Modell, `authorize`, Filter, Entzug | `authz`, Rechte-Cache in `gateway` und `sync` |
| [08-identitaet.md](08-identitaet.md) | Anbieter, Geräte, Tokens, Step-up, Einrichtung | `identity` |
| [09-agenten.md](09-agenten.md) | Adapter-Vertrag, Läufe, Komponenten, Befehle, Sitzungen | `agents`, `runs`, Teile von `chat` |
| [10-backend-richtlinie.md](10-backend-richtlinie.md) | I1 bis I6, Umschalten, Kosten | `agents`, `authz` (I1, I6), `runs` (I2, I3), `billing`, `terminal` (I6) |
| [11-erweiterungen.md](11-erweiterungen.md) | Apps, Manifest, Webhooks, SDK | `apps` |
| [12-dateien.md](12-dateien.md) | Upload, Speicher, Scan, Vorschau | `files` |
| [13-push-benachrichtigungen.md](13-push-benachrichtigungen.md) | Benachrichtigung, Push | `notify` |
| [14-suche.md](14-suche.md) | Volltextsuche | `search`, Suchvektoren in `messaging`, `chat`, `files` |
| [15-client-architektur.md](15-client-architektur.md) | Apps, lokale Datenbank, Sync-Engine | Gegenstelle zu `gateway`, `sync` |
| [16-betrieb.md](16-betrieb.md) | Installation, Konfiguration, Sicherung | `platform/config`, `platform/lifecycle`, `status` |
| [17-observability.md](17-observability.md) | Telemetrie, SLOs, Dashboards | `platform/otel`, `stats` (Abgrenzung) |
| [18-release.md](18-release.md) | Versionen, Images, Signaturen | `cmd/bothy`, Build |
| [19-sicherheit.md](19-sicherheit.md) | Bedrohungsmodell, Härtung | `audit`, `instance` (Schlüssel), alle Adapter |
| [20-datenschutz.md](20-datenschutz.md) | DSGVO, Aufbewahrung, Löschung | `privacy`, `kernel/personaldata`, Aufbewahrungsjobs |
| [21-dev-isolation.md](21-dev-isolation.md) | Mehrbenutzer-Dev-Umgebung | `chat` (Projektbindung), `runs` (Prüfung `ready`) |
| [22-import-migration.md](22-import-migration.md) | Discord-Import | `importer` |
| [23-qualitaet.md](23-qualitaet.md) | Gates, Tests, Werkzeuge | Abschnitt 7, Lasttests aus Abschnitt 17 |
| [24-terminal.md](24-terminal.md) | Rechner, Host-Dienst, Noise-Strom, I6, Terminal-Sitzungen | `terminal`; Host-Dienst `services/host` außerhalb des Servers |
| [25-visualisierung.md](25-visualisierung.md) | Eigene Messung, Live-Rahmen, Visualisierungen, Grafana-Plugin | `stats`, `kernel/measure`; Plugin `plugins/grafana-bothy-live` außerhalb des Servers |

## 19. Entscheidungen dieses Kapitels

| Nr | Entscheidung | Grund |
|---|---|---|
| R-AR-01 | Ein Binary `bothy` mit Unterbefehlen; Rollen über `--role`. | Baseline nennt den Binary-Namen `bothy`; B1 beschreibt die Server-Rolle. Ein Image für alle Rollen. |
| R-AR-02 | Module `gateway` und `search` ohne eigene Tabellen; `stats` mit genau einer Tabelle (`stats_minutes`). | Eigene Ports, Abhängigkeiten und Leistungsziele; sonst würden sie `sync`, `messaging` oder `runs` aufblähen. Die eigene Messung braucht einen Speicher, der keinem Fachmodul gehört (Ruling T7). |
| R-AR-03 | Port ist das Wurzelpaket mit eigenen DTOs. | Domain bleibt intern änderbar; spätere Prozessgrenze ohne Umbau der Aufrufer. |
| R-AR-04 | Modul-Abhängigkeiten bilden einen Graphen ohne Zyklen; Gegenrichtung nur über Quellen-Ports oder In-Tx-Ereignisse. | Abspaltbarkeit und Verständlichkeit; Go verbietet ohnehin Import-Zyklen. |
| R-AR-05 | Querschnitts-Ports in `internal/kernel`. | Jedes Modul braucht `authorize`, Ereignisse, Audit, Unit of Work; als Modul-Kanten entstünden Zyklen. |
| R-AR-06 | Ambiente Transaktion im Kontext, Unit of Work mit festen Phasen vor dem Commit. | Setzt die Sperrreihenfolge aus 03 zentral durch; Module können sie nicht falsch machen. |
| R-AR-07 | Keine externen Aufrufe in Transaktionen. | Kurze serialisierte Commit-Phase, sichere Wiederholung. |
| R-AR-08 | Folgen nach außen über Worker-Verbraucher mit Cursor, nicht über Job-Einfügen im Schreibpfad. | Erzeuger kennen Verbraucher nicht; Log ist schon der Ausgangskorb. |
| R-AR-09 | River nur in `platform/jobs`, eigenes Schema, keine Pro-Funktionen. | 0.x-Abhängigkeit an einer Stelle; Domänenschema bleibt frei von Fremdtabellen. |
| R-AR-10 | Kurzlebige Ereignisse über Knoten nur als ID-Signal (`NOTIFY ephemeral`), nur im Modus `multi`. | R-Stack: NOTIFY ohne Inhalt; jedes NOTIFY kostet eine instanzweite Sperre beim Commit. |
| R-AR-11 | Gateway-Sitzungen nur im Prozess; fremder Knoten antwortet auf `resume` mit `4006`. | Kein geteilter Sitzungsspeicher; `identify` mit Cursor liefert dasselbe Nachholen. |
| R-AR-12 | Rate-Limits im Prozess, bei mehreren Knoten je Knoten. | Kein weiterer Dienst; Grenzen sind Schutz, keine Abrechnung. |
| R-AR-13 | Health auf dem internen Listener außerhalb der öffentlichen API; Metriken nur über OTLP, kein Metrik-Endpunkt (ADR 0034). | Ohne Anmeldung nutzbar für Orchestrator, aber nicht öffentlich; keine Aktion im Katalog nötig. |
| R-AR-14 | Nachweis je Anfrage, dass `authorize` mit der Aktion der Route lief. | Hält B7 zur Laufzeit, nicht nur in der Spec. |
| R-AR-15 | Trennung Konfiguration (Betreiber, Neustart) und Einstellungen (Owner, Datenbank, sofort); unbekannte Schlüssel brechen den Start ab. | Klare Zuständigkeit; Tippfehler fallen sofort auf. |
| R-AR-16 | Migrationen nur über `bothy migrate`, nie implizit beim Start; Server prüft die Schema-Version. | Kein gleichzeitiges Migrieren mehrerer Knoten, keine Neustart-Schleife. |
| R-AR-17 | Shutdown schließt WebSockets mit `4013` und sichert Agenten-Cursor vorher. | `http.Server.Shutdown` deckt übernommene Verbindungen nicht ab; Clients nehmen geordnet wieder auf. |
| R-AR-18 | Im Anfragepfad nur Wiederholung des ganzen Unit of Work bei Deadlock und Sperr-Zeitüberschreitung. | Andere Wiederholungen gehören in Jobs oder Clients. |
| R-AR-19 | Architektur-Gate mit go-arch-lint, Konfiguration aus `modules.yaml` erzeugt ([ADR 0050](../adr/0050-modulregister-als-quelle-fuer-architektur-gate-und-tabellenbesitz.md)). | Eine Quelle für Modulgrenzen; Werkzeug prüft Pakete und Fremdbibliotheken. |
| R-AR-20 | Leistungsziele als benannte Werte, gehalten durch Lasttests in `make perf` ([ADR 0063](../adr/0063-leistungs-gate-fuer-das-ereignis-log.md)). | Keine Zahl ohne Test. |

## 20. Abgleich mit dem Kern-Vertrag

Die Punkte, die dieses Kapitel beim Entwurf als Ergänzung beantragt hatte, stehen jetzt im
Kern-Vertrag (Integration vom 2026-10-10):

| Punkt | Wo |
|---|---|
| Tabelle `event_consumer_cursors` für die Worker-Verbraucher | 03, Abschnitt 4.10; [ADR 0051](../adr/0051-worker-verbraucher-des-ereignis-logs-mit-cursor.md) |
| River-Tabellen als technische Tabellen | 03, Abschnitt 4.13 |
| Fencing der Agenten-Verbraucher (`holder_session_id`) | 03 `agent_consumer_cursors`; 04, Abschnitt 15.1 |
| Nachholen mit vollständig sichtbaren Bereichen; Cache folgt Strukturereignissen | 04, Abschnitte 7.2 und 8.1 |
| Ablauf erzeugt `access.changed` über `authz.expiry_sweep` | 04, Abschnitt 8.2; 05 `grant.expire`, `group.member.expire` |
| Thread verschieben: Kopien von `channel_id`, Dateien folgen der Nachricht | 03 `threads`; 05, Abschnitt 2.2 |
| Pfad-Präfix `/api/v1` | 04, Kopf und Abschnitt 6.1 |
| Maschinenlesbarer Aktionskatalog | [`docs/spec/actions.yaml`](actions.yaml) ([ADR 0049](../adr/0049-maschinenlesbarer-aktionskatalog-als-einzige-quelle.md)), geprüft von `make catalog-check` (05, Regel 3) |
| Health ohne Aktion, auf dem internen Listener | 05, R-AZ-22; Abschnitt 15.2 |
| Module `gateway`, `search` ohne Tabellen; `stats` mit `stats_minutes` | 03, Abschnitte 2 und 4.15 |
| `agent.read` für den Agenten selbst | 05, Abschnitt 4.10 |
| Eindeutiger Live-Box-Anker | 03 `messages`, Teilindex auf `run_id` |
| Kanal `ephemeral` im Modus `multi` | 03, Abschnitt 6.6; 04, Abschnitt 12 |

## 21. Offen

- Werkzeug für Go-Typen aus AsyncAPI 3.0: entscheidet [06-api.md](06-api.md); bis dahin
  handgeschriebene Typen mit Vertragstest gegen die Schemas.
- Ob `PollOnly` für River unter Last nötig ist (eigene NOTIFY-Last von River), zeigt
  `perf/event_commit`. Die Option existiert [Q3, belegt].

## 22. Konfigurationswerte dieses Kapitels

Standardwerte. Leistungsziele (`perf.*`, Ort `Lasttest`) sind Schwellen für Tests, keine
Laufzeit-Konfiguration: Sie haben keinen Umgebungsnamen, stehen nicht im Konfigurationsschema des
Servers, und `make perf` liest sie aus dieser Tabelle ([ADR 0063](../adr/0063-leistungs-gate-fuer-das-ereignis-log.md)).
Listener, Rolle, öffentliche URL, vertrauenswürdige Proxys und die Verbindungsdaten der Datenbank
(`server.*`, `database.host` bis `database.max_conns`) führt [16-betrieb.md](16-betrieb.md); die
Grenzen ausgehender Aufrufe (`egress.*`) führt [19-sicherheit.md](19-sicherheit.md).

| Name | Ort | Standard | Bedeutung |
|---|---|---|---|
| `cluster.mode` | Server | `single` | `single` oder `multi` (mehrere api-Knoten) |
| `http.read_header_timeout` | Server | `5s` | Frist für Kopfzeilen |
| `http.request_timeout` | Server | `15s` | Kontext-Frist normaler Anfragen |
| `http.transfer_timeout` | Server | `10m` | Frist für Upload und Download |
| `http.idle_timeout` | Server | `120s` | Leerlauf von Keep-Alive-Verbindungen |
| `http.max_header_bytes` | Server | `65536` | größte Kopfzeilen |
| `http.max_json_body_bytes` | Server | `1048576` | größter JSON-Körper; echt größer als `message.max_content_bytes` (13, Punkt 4) |
| `http.max_json_depth` | Server | `32` | größte Schachtelungstiefe eines JSON-Körpers (SEC-IN-2); ein Test hält, dass jedes Anfrage-Beispiel in `openapi.yaml` darunter bleibt |
| `http.unavailable_retry_after` | Server | `5s` | `Retry-After` jeder Antwort `503 unavailable` (in ganzen Sekunden) |
| `database.acquire_timeout` | Server | `2s` | Warten auf eine Pool-Verbindung |
| `database.statement_timeout` | Server | `5s` | Frist je SQL-Anweisung |
| `database.lock_timeout` | Server | `2s` | Frist für Sperren |
| `database.idle_in_tx_timeout` | Server | `10s` | offene Transaktion ohne Arbeit |
| `database.tx_retry_max` | Server | `2` | Wiederholungen des Unit of Work bei `40P01` und `55P03` |
| `database.listen_reconnect_backoff_cap` | Server | `10s` | größter Abstand beim Neuaufbau von `LISTEN` |
| `startup.dependency_wait` | Server | `60s` | Warten auf die Datenbank beim Start |
| `sync.log_poll_interval` | Server | `1s` | Poll des Log-Tails ohne `NOTIFY` |
| `sync.tail_batch_size` | Server | `1000` | Ereignisse je Lesevorgang des Log-Tails |
| `sync.partition_maintenance_interval` | Server | `10m` | Abstand des Jobs `sync.partition_maintenance` |
| `sync.event_log_partitions_ahead` | Server | `3` | Partitionen von `event_log`, die vor dem Kopf bereitstehen; mit `sync.event_log_partition_size` reichen sie bei `perf.event_commit_min_per_s` länger als zwei Abstände `sync.partition_maintenance_interval` |
| `gateway.write_timeout` | Server | `10s` | Frist für einen WebSocket-Schreibvorgang |
| `consumer.batch_size` | Server | `500` | Ereignisse je Runde eines Worker-Verbrauchers |
| `consumer.poll_interval` | Server | `1s` | Poll eines Worker-Verbrauchers ohne `NOTIFY` |
| `consumer.pending_alert_after` | Server | `15m` | age of the oldest unprocessed event of a worker consumer that raises the alarm `BothyEventConsumerLag`; strictly below `sync.event_retention` (13, item 4) |
| `sync.idempotency_cleanup_interval` | Server | `1h` | Abstand des Jobs `sync.idempotency_cleanup` |
| `sync.idempotency_cleanup_batch_size` | Server | `1000` | Zeilen je Löschstapel von `sync.idempotency_cleanup` |
| `ratelimit.max_tracked_keys` | Server | `100000` | höchstens so viele Rate-Limit-Eimer hält ein Prozess; darüber werden volle, dann älteste Eimer entfernt |
| `ratelimit.sweep_interval` | Server | `1m` | Abstand, in dem volle Eimer entfernt werden (ein voller Eimer entscheidet wie ein fehlender) |
| `jobs.schema` | Server | `jobs` | PostgreSQL-Schema der River-Tabellen |
| `jobs.queue.default.max_workers` | Server | `8` | Parallelität `default` |
| `jobs.queue.files.max_workers` | Server | `2` | Parallelität `files` |
| `jobs.queue.delivery.max_workers` | Server | `16` | Parallelität `delivery` |
| `jobs.queue.maintenance.max_workers` | Server | `2` | Parallelität `maintenance` |
| `jobs.queue.privacy.max_workers` | Server | `1` | Parallelität `privacy` |
| `shutdown.drain_delay` | Server | `5s` | Warten nach `ready = false` |
| `shutdown.http_grace` | Server | `20s` | Frist für laufende HTTP-Anfragen |
| `shutdown.jobs_soft_timeout` | Server | `20s` | weicher Stopp von River |
| `shutdown.telemetry_flush_timeout` | Server | `5s` | Telemetrie leeren |
| `shutdown.total_timeout` | Server | `60s` | gesamte Frist bis zum Ende; mindestens die Summe der Schritt-Budgets (15.3) |
| `perf.reference_profile` | Lasttest | `4 vCPU, 8 GiB RAM, SSD, PostgreSQL 18 lokal mit fsync und synchronous_commit an, Rolle all` | Umgebung der Lasttests; Modi der Messung in ADR 0063 |
| `perf.sustained_messages_per_s` | Lasttest | `200` | Dauerlast für `perf.message_create_p99` |
| `perf.message_create_p99` | Lasttest | `150ms` | Ziel |
| `perf.event_commit_min_per_s` | Lasttest | `1000` | Ziel (Mindestwert) |
| `perf.fanout_local_p99` | Lasttest | `200ms` | Ziel |
| `perf.fanout_cross_node_p99` | Lasttest | `500ms` | Ziel |
| `perf.invocation_delivery_p99` | Lasttest | `500ms` | Ziel |
| `perf.run_patch_fanout_p99` | Lasttest | `500ms` | Ziel |
| `perf.catchup_page_p99` | Lasttest | `250ms` | Ziel |
| `perf.authorize_cached_p99` | Lasttest | `100us` | Ziel |
| `perf.ws_connections_per_node` | Lasttest | `5000` | Ziel (Mindestwert) |
| `perf.ws_idle_conn_mem_max` | Lasttest | `64KiB` | Ziel (Höchstwert) |
| `perf.idle_rss_max` | Lasttest | `256MiB` | Ziel (Höchstwert) |
| `perf.startup_ready_max` | Lasttest | `5s` | Ziel (Höchstwert) |
| `perf.push_handoff_p95` | Lasttest | `2s` | Ziel |

## Quellen

Alle abgerufen am 2026-10-10.

- [Q1] PostgreSQL, NOTIFY: https://www.postgresql.org/docs/current/sql-notify.html. Zustellung erst
  nach dem Commit, Payload-Grenze. [belegt]
- [Q2] Recall.ai, "Postgres LISTEN/NOTIFY does not scale":
  https://recall.ai/blog/postgres-listen-notify-does-not-scale. Eine Transaktion mit NOTIFY nimmt
  beim Commit eine instanzweite Sperre. [belegt]
- [Q3] River, `client.go` (Felder `PollOnly`, `SoftStopTimeout`, `FetchPollInterval`):
  https://github.com/riverqueue/river/blob/master/client.go. [belegt]
- [Q4] River, Graceful Shutdown: https://riverqueue.com/docs/graceful-shutdown. Weicher und harter
  Stopp; Jobs müssen bei Abbruch einen Fehler liefern. [belegt]
- [Q5] River, Insert-only clients und `InsertTx`: https://riverqueue.com/docs/insert-only-clients
  und https://riverqueue.com/docs. [belegt]
- [Q6] River, Migrationen des pgx-Treibers:
  https://github.com/riverqueue/river/tree/master/riverdriver/riverpgxv5/migration/main
  (Hauptzweig). [belegt]
- [Q7] River, `rivermigrate`: https://pkg.go.dev/github.com/riverqueue/river/rivermigrate (v0.49.0).
  [belegt]
- [Q8] go-arch-lint, Syntax: https://github.com/fe3dback/go-arch-lint/blob/master/docs/syntax/README.md;
  Version v1.19.0 vom 2026-09-07 laut https://proxy.golang.org/github.com/fe3dback/go-arch-lint/@latest.
  [belegt]
- [Q9] Go, `http.Server.Shutdown`: https://pkg.go.dev/net/http#Server.Shutdown (gelesen im Quelltext
  von go1.27.2). [belegt]
- [Q10] Go, `http.Request.Pattern`: https://pkg.go.dev/net/http#Request (gelesen im Quelltext von
  go1.27.2). [belegt]
- [Q11] oapi-codegen runtime, `StrictHTTPMiddlewareFunc(f, operationID)`:
  https://pkg.go.dev/github.com/oapi-codegen/runtime/strictmiddleware/nethttp (v1.7.0). [belegt]
