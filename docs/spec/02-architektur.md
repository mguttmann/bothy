# 02 Architektur

Status: Entwurf M0. Normativ für den Aufbau von `services/server/`.
Begriffe: [00-glossar.md](00-glossar.md). Tabellen: [03-datenmodell.md](03-datenmodell.md).
Ereignisse und Sync: [04-sync-protokoll.md](04-sync-protokoll.md). Aktionen:
[05-aktionen.md](05-aktionen.md). Entscheidungen: ADR 0004 bis 0009 in [../adr/](../adr/).
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

## 3. Container (C4 Ebene 2)

Diagramm: [c4-2-container.svg](../architecture/c4-2-container.svg)
(Quelle [c4-2-container.mmd](../architecture/c4-2-container.mmd)).

### 3.1 Container

| Container | Technik | Verantwortung | Zustand | Skalierung |
|---|---|---|---|---|
| `bothy serve --role=api` | Go, ein Binary, OCI-Image | REST, Gateway-WebSocket, Log-Tail und Fan-out im Prozess, Agenten-Verbraucher | nur flüchtig im Speicher (Verbindungen, Rechte-Cache, Rate-Limit-Eimer) | waagerecht ab Stufe 2 (Abschnitt 16) |
| `bothy serve --role=worker` | gleiches Binary und Image | River-Jobs, Ereignis-Verbraucher (Push, Webhooks), Dateiverarbeitung, Wartung, Importe | keiner außerhalb der Datenbank | waagerecht ab Stufe 3 |
| PostgreSQL 18 | Datenbank | alle Domänendaten, `event_log`, Audit, River-Tabellen im Schema `jobs.schema` | maßgeblich | senkrecht; Lese-Replikat ab Stufe 4 |
| Objektspeicher | Dateisystem-Treiber (Standard) oder S3-kompatibel | Dateien, Vorschaubilder, Exportpakete | maßgeblich für Bytes | über den Speicherdienst |
| ClamAV `clamd` | eigener Prozess, optional | Malware-Scan per `INSTREAM` | Signaturen | eigener Container |
| OTel Collector | eigener Prozess | nimmt OTLP an, exportiert je Backend | Puffer | eigener Container |
| Apps | Flutter (iOS, Android, macOS, Windows) | Chat, Ausgangsfach, lokale Datenbank, Notification Service Extension | lokaler Cache, nie maßgeblich | je Gerät |
| Web-Konsole | Flutter Web | nur Verwaltung und Statistik (B5) | keiner | statische Dateien |
| Agenten-Adapter | außerhalb des Servers (Referenz: Hermes-Agent-Plugin, Claude-Code-Runner) | Verbraucher mit dauerhaftem Cursor, prüft Aufruf-Marke, meldet Läufe | eigener | beim Betreiber der Laufzeit |

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
| Ops-Listener (Health, optional Metriken) | ja | ja |

Rolle `all` vereint beide in einem Prozess und ist der Standard (`server.role`). Grund: eine
Installation auf einem Rechner braucht genau einen Server-Container. Die Trennung kostet nur eine
Zeile Konfiguration, weil beide Rollen dasselbe Binary und dieselbe Datenbank nutzen.

### 3.3 Befehle des Binarys

| Befehl | Zweck | Kapitel |
|---|---|---|
| `bothy serve --role=api\|worker\|all` | Server starten | dieses Kapitel |
| `bothy migrate up\|status\|validate` | Migrationen des Domänenschemas (goose) und der River-Tabellen (rivermigrate) | ADR 0007 |
| `bothy setup ...` | Einrichtung, erster Owner per Server-Befehl | [08-identitaet.md](08-identitaet.md) |
| `bothy config check\|print` | Konfiguration prüfen, redigiert ausgeben | Abschnitt 13 |

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
7. Drei Module besitzen keine Tabellen: `gateway`, `search`, `stats` (Kern-Vertrag: Ergänzung
   beantragt, Abschnitt 20). Sie sind eigene Module, weil sie eigene Ports, eigene Abhängigkeiten
   und eigene Leistungsziele haben.

### 4.2 Modulliste

| Modul | Verantwortung | Besitzt Tabellen | Kapitel |
|---|---|---|---|
| `instance` | Instanz-Identität, Einstellungen und Branding, Feature-Flags, Server-Signierschlüssel mit JWKS und Rotation, Sequenz-Zähler | `instance`, `instance_settings`, `server_signing_keys`, `feature_flags`, `sequence_counters` | 06, 16, 19 |
| `audit` | Audit-Log mit Hash-Kette, Abfrage, Prüfung, Kürzung mit Anker | `audit_events` | 19, 20 |
| `authz` | `authorize`, Rechte-Cache, sichtbare Ziele, Gruppen, Freigaben, Rechte-Version, I1-Prüfung, "Wer hat Zugriff", "Als Person ansehen" | `groups`, `group_memberships`, `grants` | 07 |
| `billing` | Preistabelle, Kostenbuchungen, Summen je Lauf, Thread, Bereich, Agent, Abgleich | `price_table`, `cost_entries` | 10 |
| `identity` | Prinzipale, Menschen, Owner, Anmelde-Treiber, Identitäten, Passkeys, Geräte und Geräteschlüssel, Sitzungen, Refresh-Familien, API-Tokens, Step-up, Zugriffstokens | `principals`, `users`, `owners`, `identity_providers`, `user_identities`, `passkey_credentials`, `devices`, `device_keys`, `auth_sessions`, `refresh_token_families`, `refresh_tokens`, `api_tokens` | 08 |
| `chat` | Bereiche, Seitenleisten-Zustand, Kanäle mit Verhalten, Skills, Projektbindung, DM-Teilnehmer, Forum-Tags, Threads mit Thread-Sequenz | `spaces`, `user_space_states`, `channels`, `channel_skill_bindings`, `project_bindings`, `dm_participants`, `forum_tags`, `threads`, `thread_tags` | 01, 07, 09 |
| `agents` | Agenten, Backend-Richtlinie (Soll), gemeldeter Ist-Zustand, Verbraucher-Cursor, Befehlsregister, Agenten-Sitzungen | `agents`, `agent_consumer_cursors`, `commands`, `agent_sessions` | 09, 10 |
| `files` | Upload, Objektspeicher-Treiber, Scan, Vorschaubilder, Anhänge, Transkripte | `files`, `file_thumbnails`, `message_attachments` | 12 |
| `messaging` | Nachrichten, Revisionen, Erwähnungen, Reaktionen, Anheften, Vorlagen, Kanal-Export | `messages`, `message_revisions`, `mentions`, `reactions`, `pins`, `message_templates` | 06, 14 |
| `runs` | Auslöse-Entscheidung, Läufe und Lauf-Dokument, Schritte, Nachträge, Komponenten, Interaktionen, Befehlsausführung, Aufruf-Marken | `runs`, `run_steps`, `run_followups`, `message_components`, `interactions` | 09, 10 |
| `sync` | Ereignis-Log (Anhängen, Tail, Nachholen, Schwärzen, Partitionen), Momentaufnahme, Idempotenz-Schlüssel, Gelesen-Stand, Entwürfe, persönliche Einstellungen, Cursor der Worker-Verbraucher | `event_log`, `idempotency_keys`, `read_states`, `drafts`, `user_settings`; beantragt: `event_consumer_cursors` | 04, 15 |
| `notify` | Benachrichtigungsstufen, Ruhezeiten, Push-Registrierungen, Push-Verbraucher und Versand | `notification_preferences`, `quiet_hours`, `push_registrations` | 13 |
| `gateway` | WebSocket-Endpunkt, Gateway-Sitzungen, Hub, Fokus, Tippen, Weitergabe kurzlebiger Ereignisse, Agenten-Verbindungen | keine | 04, 06 |
| `search` | Suchabfrage über Nachrichten, Thread-Titel und Dateien mit Rechtefilter, Rangfolge | keine | 14 |
| `stats` | Statistik für Konsole: Nutzung je Bereich, Läufe, Werkzeugaufrufe, Kosten, Fehlerraten | keine | 01, 17 |
| `apps` | Apps und Manifeste, Installationen, Scopes, ausgehende Webhooks mit Verbraucher und Zustellung, Eingangs-Webhooks | `apps`, `app_installations`, `webhook_subscriptions`, `webhook_deliveries`, `incoming_webhooks` | 11 |
| `importer` | Import-Quellen, Trockenlauf, Import mit Herkunftsschlüssel | `import_sources`, `import_items` | 22 |
| `privacy` | DSGVO-Aufträge, Export und Löschung über die Teilnehmer aller Module | `gdpr_requests` | 20 |
| `status` | Statusseite, Komponenten, Störungen, automatische Prüfungen | `status_components`, `status_incidents`, `status_incident_updates`, `status_incident_components` | 16, 17 |

Technische Tabellen außerhalb der Module: River legt eigene Tabellen mit Präfix `river_` an
(u. a. `river_migration`, `river_job`, `river_leader`, `river_queue`) [Q6, belegt]. Sie liegen im
Schema `jobs.schema` und gehören `internal/platform/jobs` (Kern-Vertrag: Ergänzung beantragt). Die
Versionstabelle von goose liegt im selben Schema wie die Domänentabellen.

### 4.3 Ports, Ereignisse und Quellen je Modul

"Port" nennt die eingehenden Schnittstellen im Wurzelpaket (Auszug, Namen sind Vorschläge für den
Bau). "Gibt aus" sind In-Tx-Ereignisse (Go-Typen, nie serialisiert). "Quellen" sind Schnittstellen
tieferer Module, die das Modul implementiert.

| Modul | Port (Auszug) | Gibt aus (In-Tx) | Abonniert (In-Tx) | Implementiert Quellen |
|---|---|---|---|---|
| `instance` | `Settings`, `Flags`, `Keys` (Signer je Zweck, JWKS, Rotation), `Counters` (nur für die Phasen-Hooks von `authz`, `sync`, `audit`) | | | |
| `audit` | `Log` (Abfrage, Prüfung) | | | kernel `auditing.Recorder` |
| `authz` | `Admin` (Gruppen, Freigaben, Erklärung, Ansicht als Person); deklariert `PrincipalSource`, `StructureSource`, `AgentPolicySource` | kernel `authorize.AccessChanged` | | kernel `authorize.Authorizer` |
| `billing` | `Costs` (Buchen, Korrektur, Summen), `Prices` | | | |
| `identity` | `Auth` (Anmeldung, Refresh, Logout, Step-up), `Directory`, `Devices`, `Tokens`, `Owners`; deklariert `AvatarFiles` | `DeviceRevoked` | | kernel `actor.Authenticator`, `authz.PrincipalSource`, kernel `personaldata.Participant` |
| `chat` | `Structure` (Bereiche, Kanäle), `Threads` (Anlegen, `AppendToThread`, Schließen, Öffnen, Verschieben, Info), `AgentConfig` | `ThreadMoved`, `ChannelArchived`, `SpaceArchived` | | `authz.StructureSource`, kernel `personaldata.Participant` |
| `agents` | `Agents` (Verwaltung, Richtlinie), `Presence`, `Consumers` (Cursor, Bestätigung, Übernahme), `Commands` (Register), `Sessions` | `AgentDisabled` | | `authz.AgentPolicySource`, kernel `personaldata.Participant` |
| `files` | `Uploads`, `Files` (Lesen, Öffnen, Vorschau, Löschen), `Attachments`, `Transcripts`; deklariert `AttachmentLocator` | | | `identity.AvatarFiles`, kernel `personaldata.Participant` |
| `messaging` | `Messages` (Anlegen, Bearbeiten, Löschen, Verlauf, `SetTriggerState`, Anker), `Reactions`, `Pins`, `Templates`, `Mentions`, `Export`; deklariert `ComponentReader` | `MessagePosted`, `MessageDeleted` | `chat.ThreadMoved` | `files.AttachmentLocator`, kernel `personaldata.Participant` |
| `runs` | `Runs` (Lesen, Start, Patch, Ende, Abbruch, Übersicht), `Delivery` (Zustellvorbereitung mit I2 und Marke), `Interactions`, `CommandExec` | | `messaging.MessagePosted`, `messaging.MessageDeleted`, `agents.AgentDisabled`, kernel `authorize.AccessChanged` | `messaging.ComponentReader`, kernel `personaldata.Participant` |
| `sync` | `Tail`, `CatchUp`, `Snapshot`, `Consumers` (Worker-Cursor), `ReadStates`, `Drafts`, `Settings` | | `messaging.MessagePosted` (Entwurf leeren, Erwähnungszähler), `chat.ThreadMoved` | kernel `eventlog.Appender`, kernel `idempotency.Store`, kernel `personaldata.Participant` |
| `notify` | `Preferences`, `QuietHours`, `PushRegistrations` | | `identity.DeviceRevoked`, `chat.ChannelArchived`, `chat.SpaceArchived` | kernel `personaldata.Participant` |
| `gateway` | keiner für andere Module | | | kernel `eventlog.EphemeralPublisher` |
| `search` | `Search` | | | |
| `stats` | `Stats` | | | |
| `apps` | `Apps`, `Webhooks`, `IncomingWebhooks` | | `chat.ChannelArchived` | |
| `importer` | `Imports` | | | |
| `privacy` | `Requests` | | | sammelt alle kernel `personaldata.Participant` |
| `status` | `Status` | | `agents.AgentDisabled` | |

### 4.4 Erlaubte Abhängigkeiten

"Darf nutzen" heißt: Code dieses Moduls darf den Port des genannten Moduls importieren. Alles
andere ist verboten und schlägt im Gate fehl (Abschnitt 7).

| Modul | Darf nutzen | Grund je Kante |
|---|---|---|
| `instance` | nichts | Fundament |
| `billing` | nichts | Kostenzeilen tragen Kopien von Thread, Kanal, Bereich und Agent (03); keine Rückfrage nötig |
| `audit` | `instance` | Zähler `audit_seq` |
| `authz` | `instance` | Zähler `permission_version`; alle anderen Daten über Quellen-Ports |
| `identity` | `instance`, `authz` | Signierschlüssel für Zugriffstokens; implementiert `PrincipalSource` |
| `chat` | `identity`, `authz` | Bediener ist aktiver Agent-Prinzipal, DM-Teilnehmer sind Menschen; implementiert `StructureSource` |
| `agents` | `identity`, `authz`, `chat` | Agent-Prinzipal anlegen, Zugangsdaten-Inhaber ist Mensch; implementiert `AgentPolicySource`; Thread für Sitzungen, Geltung von Befehlen |
| `files` | `identity`, `chat` | implementiert `AvatarFiles`; Upload-Kanal prüfen |
| `messaging` | `identity`, `chat`, `files` | Erwähnungen auflösen; Thread-Sequenz und Wiederöffnen; Anhänge binden und lesen |
| `status` | `agents` | Agenten-Präsenz als Status-Komponente |
| `runs` | `instance`, `chat`, `agents`, `messaging`, `billing` | Marke signieren; Bediener, Erwähnungsregel, Projektbindung; Richtlinie und Sitzung; Anker und `trigger_state`; Kosten buchen |
| `search` | `chat`, `messaging`, `files` | Thread-Titel, Nachrichten, Dateinamen und Transkripte |
| `importer` | `identity`, `chat`, `messaging`, `files` | legt Menschen, Struktur, Nachrichten und Dateien an |
| `privacy` | `identity`, `files` | betroffene Person; Exportpaket speichern |
| `sync` | `instance`, `chat`, `agents`, `messaging`, `runs` | Zähler `event_seq`; Momentaufnahme aus Struktur, Präsenz, laufenden Läufen; Abo auf Nachrichten |
| `stats` | `chat`, `agents`, `messaging`, `runs`, `billing` | liest Kennzahlen über Ports |
| `notify` | `identity`, `chat`, `messaging`, `sync` | Geräte und Zeitzone; Ziel-Pfad für Einstellungen; Erwähnungen; Gelesen-Stand |
| `apps` | `instance`, `identity`, `chat`, `agents`, `messaging`, `sync` | Webhook-Signierschlüssel; App-Prinzipal und Tokens; Ziel der Eingangs-Webhooks; Befehle von Apps; Nachrichten aus Webhooks; Verbraucher für ausgehende Webhooks |
| `gateway` | `chat`, `agents`, `runs`, `sync` | Thread zu Kanal für Fokus und Tippen; Verbraucher-Cursor und Präsenz; Lauf-Patches und Zustellvorbereitung; Tail, Nachholen, Momentaufnahme |

Ebenen (von unten): `instance`, `billing` · `audit`, `authz` · `identity` · `chat` · `agents`,
`files` · `messaging`, `status` · `runs`, `search`, `importer`, `privacy` · `sync`, `stats` ·
`notify`, `apps`, `gateway`. Jede Kante zeigt nach unten.

Zusätzlich gilt für alle Module: `kernel` ja, `platform` nur in `adapters`, `api` (generierter Code)
nur in `adapters`, River nur in `platform/jobs`.

### 4.5 Querschnitts-Ports (`internal/kernel`)

| Paket | Zweck | Implementiert von | Genutzt von |
|---|---|---|---|
| `kernel/ids` | UUIDv7 erzeugen und prüfen (03, R-DM-01) | kernel selbst | allen |
| `kernel/clock` | Zeitquelle, in Tests ersetzbar | kernel selbst | allen |
| `kernel/errs` | Fehlerarten mit stabilem Code (Abschnitt 14.1) | kernel selbst | allen |
| `kernel/actor` | Prinzipal einer Anfrage (ID, Art, Gerät, Sitzung, Step-up-Zeit, Scopes); `Authenticator` | `identity` | `platform/httpserver`, `gateway` |
| `kernel/authorize` | Aktionen (generiert aus 05), Ressourcen-Deskriptor, `Decision`, `Authorizer` (`Authorize`, `Scope` für Listen, `Observe` für `access.changed`), In-Tx-Ereignis `AccessChanged` | `authz` | allen |
| `kernel/txn` | Unit of Work, Phasen vor dem Commit, In-Tx-Ereignisbus (Abschnitt 8) | `platform/pg`; Phasen-Hooks von `authz`, `sync`, `audit` | allen `app`-Paketen |
| `kernel/eventlog` | `Appender` für dauerhafte Ereignisse, `EphemeralPublisher` für kurzlebige | `sync` bzw. `gateway` | allen |
| `kernel/auditing` | `Recorder`: in der Transaktion oder getrennt (für Ablehnungen nach Rollback) | `audit` | allen |
| `kernel/idempotency` | `Store` für `Idempotency-Key` im Unit of Work | `sync` | `platform/httpserver` |
| `kernel/personaldata` | `Participant`: Export und Löschung je Modul für einen Menschen | Module mit personenbezogenen Tabellen | `privacy` |

`kernel` hängt von nichts im Projekt ab. Gründe: Diese Ports braucht jedes Modul; als Modul-Kanten
würden sie Zyklen erzeugen (jedes Modul ruft `authorize`, `authz` braucht Ereignisse und Audit).

### 4.6 Aktionen je Modul

Jede Aktion aus [05-aktionen.md](05-aktionen.md) gehört genau einem Modul. Ein Test vergleicht diese
Tabelle mit dem Katalog (Abschnitt 7.3).

| Modul | Aktionen |
|---|---|
| `instance` | `instance.discovery.read`, `instance.read`, `instance.settings.update`, `instance.branding.update`, `feature_flag.read`, `feature_flag.update`, `signing_key.rotate` |
| `audit` | `audit.read`, `audit.verify` |
| `authz` | `access.explain`, `access.view_as`, `group.read`, `group.create`, `group.update`, `group.delete`, `group.member.add`, `group.member.remove`, `grant.read`, `grant.create`, `grant.revoke` |
| `identity` | `auth.login.begin`, `auth.login.complete`, `auth.token.refresh`, `auth.logout`, `auth.step_up`, `identity_provider.read`, `identity_provider.manage`, `owner.add`, `owner.remove`, `user.read`, `user.create`, `user.update`, `user.disable`, `user.enable`, `self.profile.update`, `identity.link`, `identity.unlink`, `passkey.register`, `passkey.revoke`, `device.list`, `device.approve`, `device.revoke`, `session.list`, `session.revoke`, `token.list`, `token.create`, `token.revoke` |
| `chat` | `space.read`, `space.create`, `space.update`, `space.reorder`, `space.archive`, `space.operator.set`, `sidebar.update`, `channel.read`, `channel.create`, `channel.update`, `channel.reorder`, `channel.behavior.read`, `channel.behavior.update`, `channel.operator.set`, `channel.project.bind`, `channel.archive`, `forum_tag.manage`, `dm.create`, `dm.leave`, `thread.read`, `thread.create`, `thread.title.update`, `thread.close`, `thread.reopen`, `thread.move`, `thread.tags.set`, `agent.config.read` |
| `agents` | `agent.read`, `agent.create`, `agent.update`, `agent.disable`, `agent.billing_mode.set`, `agent.billing_mode.set_all`, `agent.credential_holder.set`, `agent.heartbeat`, `agent.events.consume`, `agent_session.read`, `agent_session.reset`, `agent_session.bind`, `agent_session.report`, `command.read`, `command.register`, `command.remove` |
| `files` | `file.upload`, `file.read`, `file.delete`, `file.transcribe` |
| `messaging` | `message.read`, `message.create`, `message.update`, `message.delete`, `message.revisions.read`, `reaction.add`, `reaction.remove`, `pin.create`, `pin.delete`, `mention.list`, `template.read`, `template.manage`, `export.channel` |
| `runs` | `agent.invoke`, `run.read`, `run.overview.read`, `run.start`, `run.update`, `run.finish`, `run.cancel`, `run.followup.create`, `component.update`, `interaction.create`, `interaction.respond`, `command.autocomplete`, `command.invoke` |
| `billing` | `cost.read.thread`, `cost.read.aggregate`, `price.read`, `price.manage`, `cost.reconcile` |
| `sync` | `read_state.update`, `read_state.mark_unread`, `draft.read`, `draft.write`, `settings.read`, `settings.update`, `sync.snapshot.read`, `events.catch_up` |
| `notify` | `notification_pref.update`, `quiet_hours.update`, `push.register`, `push.unregister`, `push.deliver` |
| `gateway` | `gateway.connect`, `gateway.focus`, `typing.start` |
| `search` | `search.query` |
| `stats` | `stats.read` |
| `apps` | `app.read`, `app.install`, `app.scopes.update`, `app.uninstall`, `webhook_out.read`, `webhook_out.manage`, `webhook_out.redeliver`, `webhook_out.deliver`, `webhook_in.create`, `webhook_in.revoke`, `webhook_in.post` |
| `importer` | `import.read`, `import.manage` |
| `privacy` | `gdpr.request.create`, `gdpr.request.read`, `gdpr.request.decide`, `gdpr.export.download` |
| `status` | `status.public.read`, `status.read`, `status.manage` |

Begründungen für Grenzfälle:

- `agent.invoke` gehört zu `runs`, weil die Folge immer ein Lauf, ein Nachtrag oder ein
  Befehlsaufruf ist. Die Regeln aus 05 Abschnitt 5 wertet trotzdem `authz` aus.
- `agent.config.read` gehört zu `chat`, weil Prompt, Skills und Projektbindung dort liegen.
- `command.invoke` und `command.autocomplete` gehören zu `runs` (Zustellung an den Agenten mit
  `agent.invoke`), das Register (`command.register`, `command.remove`, `command.read`) zu `agents`.
- `export.channel` gehört zu `messaging`; das Paket legt `files` ab.

### 4.7 Ereignistypen und WebSocket-Ops je Modul

Ein Ereignistyp hat genau ein erzeugendes Modul. Die Empfangs-Aktion steht in 05 Abschnitt 6.

| Modul | Dauerhafte Ereignisse | Kurzlebige Ereignisse | WebSocket-Ops (Client an Server) |
|---|---|---|---|
| `instance` | `instance.updated` | | |
| `authz` | `grant.changed`, `group.changed`, `access.changed` | | |
| `identity` | `user.updated`, `session.revoked` | | |
| `chat` | `thread.created`, `thread.updated`, `thread.closed`, `thread.reopened`, `thread.moved`, `thread.moved_out`, `space.created`, `space.updated`, `space.archived`, `channel.created`, `channel.updated`, `channel.settings_changed`, `channel.archived`, `sidebar.updated`, `agent.config_changed` | | |
| `agents` | `agent_session.changed`, `command.changed`, `agent.updated` | `agent.presence_changed` | `ack` |
| `files` | `file.updated` | | |
| `messaging` | `message.created`, `message.updated`, `message.deleted`, `reaction.added`, `reaction.removed`, `pin.added`, `pin.removed` | | |
| `runs` | `run.state_changed`, `agent.invocation`, `agent.followup`, `run.cancel_requested`, `interaction.created`, `command.invoked` | `run.updated` | `run.update` |
| `sync` | `read_state.updated`, `draft.updated`, `draft.deleted`, `settings.updated` | | |
| `notify` | `notification_pref.updated`, `quiet_hours.updated` | | |
| `status` | `status.updated` | | |
| `privacy` | `gdpr.request_updated` | | |
| `importer` | `import.progress` | | |
| `gateway` | | `typing.started` | `identify`, `resume`, `heartbeat`, `reauth`, `focus`, `typing.start`, `typing.stop` |

`message.updated` für `trigger_state` erzeugt `messaging`, auch wenn `runs` den Wechsel auslöst
(über `Messages.SetTriggerState`). Grund: ein Ereignistyp, ein Erzeuger, eine Nutzlast-Definition.

## 5. Schichten im Modul

| Schicht | Paket | Enthält | Darf importieren | Darf nicht |
|---|---|---|---|---|
| Port | `internal/<modul>` | Eingehende Schnittstellen, Befehls- und Ergebnis-Typen (DTOs), In-Tx-Ereignisse, deklarierte Quellen-Ports | `kernel` | `domain`, `app`, `adapters`, fremde Module, Fremdbibliotheken |
| Domain | `internal/<modul>/domain` | Entitäten, Wertobjekte, Zustandsmaschinen (z. B. Lauf-Übergänge), Invarianten, reine Funktionen | `kernel/ids`, `kernel/clock`, `kernel/errs` | Ein- und Ausgabe, SQL, HTTP, Kontext mit Transaktion, Fremdbibliotheken |
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

## 6. Code-Layout

```text
services/server/
  go.mod                      Modulpfad siehe Abschnitt 21 (offen)
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
      config/ secrets/ pg/ httpserver/ ratelimit/ jobs/ otel/ health/ lifecycle/
    wire/                     Verdrahtung je Rolle, Registrierung der In-Tx-Abonnenten
    <modul>/                  Port (Wurzelpaket)
      domain/
      app/
      adapters/
        postgres/             queries/*.sql, sqlcgen/ (generiert), Repository
        httpapi/              Handler für die Operationen des Moduls
        ...                   weitere Adapter des Moduls
      README.md               Zweck, Ports, Abhängigkeiten, Tabellen, Kapitel
  sqlc.yaml                   ein Eintrag je Modul, Schema aus migrations/
  .go-arch-lint.yml           generiert aus architecture/modules.yaml
```

Stellen mit dem Produktnamen (Baseline Abschnitt 1, damit ein Wechsel billig bleibt): Go-Modulpfad in
`go.mod`, Binary-Name `bothy` (`cmd/bothy`), Präfix der Umgebungsvariablen `BOTHY_`, Image-Name.
Eine Liste dieser Stellen führt [18-release.md](18-release.md); weitere Stellen sind ein Fehler.

## 7. Automatische Prüfung der Regeln

### 7.1 Modulregister als Quelle

`services/server/architecture/modules.yaml` ist die einzige Quelle für Modulgrenzen. Daraus entstehen
die go-arch-lint-Konfiguration, die Prüfungen aus 7.3 und der Abgleich mit den Tabellen 4.2, 4.4 und
4.6 dieses Kapitels. Format (Auszug):

```yaml
version: 1
modules:
  chat:
    tables: [spaces, user_space_states, channels, channel_skill_bindings, project_bindings,
             dm_participants, forum_tags, threads, thread_tags]
    actions: [space.read, space.create, thread.move, agent.config.read]   # vollständig im Register
    depends_on: [identity, authz]
    adapter_vendors: [pgx, otel]
  runs:
    tables: [runs, run_steps, run_followups, message_components, interactions]
    actions: [agent.invoke, run.start, run.update, run.finish]            # vollständig im Register
    depends_on: [instance, chat, agents, messaging, billing]
    adapter_vendors: [pgx, otel, jose]
vendors:
  pgx: [github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/**]
  websocket: [github.com/coder/websocket, github.com/coder/websocket/**]
```

### 7.2 go-arch-lint

Werkzeug: go-arch-lint (MIT), aktuell v1.19.0 vom 2026-09-07 [Q8, belegt]. Ein kleiner Generator
erzeugt aus `modules.yaml` je Modul vier Komponenten (`<m>`, `<m>-domain`, `<m>-app`,
`<m>-adapters`) und die Regeln aus Abschnitt 5. Erzeugter Auszug für `chat`:

```yaml
version: 3
workdir: .
allow:
  depOnAnyVendor: false
excludeFiles: ["^.*_test\\.go$"]
components:
  kernel-core:  { in: [internal/kernel/ids, internal/kernel/errs, internal/kernel/clock] }
  kernel-ports: { in: [internal/kernel/actor, internal/kernel/authorize, internal/kernel/txn,
                       internal/kernel/eventlog, internal/kernel/auditing,
                       internal/kernel/idempotency, internal/kernel/personaldata] }
  platform:     { in: internal/platform/** }
  api:          { in: api/** }
  chat:          { in: internal/chat }
  chat-domain:   { in: [internal/chat/domain, internal/chat/domain/**] }
  chat-app:      { in: [internal/chat/app, internal/chat/app/**] }
  chat-adapters: { in: internal/chat/adapters/** }
commonComponents: [kernel-core]
deps:
  chat:          { mayDependOn: [kernel-ports] }
  chat-app:      { mayDependOn: [chat, chat-domain, kernel-ports, identity, authz] }
  chat-adapters: { mayDependOn: [chat, chat-app, chat-domain, kernel-ports, platform, api,
                                 identity, authz],
                   canUse: [v-pgx, v-otel] }
```

Eine Komponente `<m>-domain` hat keinen Eintrag unter `deps`; sie darf damit nur `kernel-core`.
Ein leeres `mayDependOn` lehnt go-arch-lint ab; darum fehlt der Eintrag statt leer zu sein.

Geprüft am 2026-10-10 an einem Gerüst mit allen Modulen aus Abschnitt 4.2 (Go 1.27.2, go-arch-lint
v1.19.0): sauberes Gerüst ergibt "No warnings found", Exitcode 0. Vier eingebaute Verstöße wurden
alle gemeldet, Exitcode 1: `chat/domain` importiert pgx; `chat/app` importiert den Port von
`messaging`; `messaging/app` importiert `runs/app`; ein neues Paket `chat/util` ohne Komponente
("not attached to any component"). Der letzte Punkt ist gewollt: Neue Pakete müssen ins Register.

### 7.3 Weitere Prüfungen im Gate

| Prüfung | Was sie hält | Umsetzung (Ticket) |
|---|---|---|
| Tabellenbesitz | SQL eines Moduls nennt nur eigene Tabellen (Grundsatz 12 aus 03) | Durchlauf über `adapters/postgres/queries/*.sql`, Tabellennamen gegen `modules.yaml` |
| Vollständigkeit | jede Tabelle aus 03 und jede Aktion aus 05 genau einmal im Register | Abgleich Register mit Kern-Vertrag |
| Routen und Ops | jede Operation hat eine bekannte Aktion; ihr Handler liegt im Modul, dem die Aktion gehört | Start-Prüfung des Servers und Test über die generierte Routen-Tabelle |
| In-Tx-Abonnements | jedes Abonnement ist eine erlaubte Richtung (höheres Modul abonniert tieferes) | Golden-Datei der Registrierungen in `internal/wire` |
| Doku-Abgleich | Tabellen 4.2, 4.4, 4.6 dieses Kapitels stimmen mit dem Register überein | Vergleich im Doku-Gate |
| Erzeugte Dateien aktuell | `.go-arch-lint.yml`, sqlc-Code, API-Code entsprechen ihrer Quelle | Neu erzeugen und `git diff --exit-code` |

## 8. Transaktionen und Unit of Work

### 8.1 Ablauf

Jeder Schreib-Anwendungsfall läuft in genau einer Datenbank-Transaktion, `READ COMMITTED` (03,
Grundsatz 11). `kernel/txn` legt sie in den Kontext. Ruft ein Anwendungsfall einen Port eines anderen
Moduls, schreibt dieses Modul in derselben Transaktion (ambiente Transaktion). Nur `platform/pg`
öffnet und beendet Transaktionen.

Phasen (Diagramm [fluss-ereignis-pfad.svg](../architecture/fluss-ereignis-pfad.svg)):

1. **Beginn.** Bei Rechte-Änderungen (Option `RightsChange` mit betroffenen Prinzipalen) ruft
   `platform/pg` zuerst den Phasen-Hook von `authz`. Er sperrt `sequence_counters('permission_version')`
   über `instance.Counters`, erhöht ihn (03, 6.3) und hält die sichtbaren Ziele der Betroffenen vor
   der Änderung fest.
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

`platform/pg` kennt keine Fachmodule. `kernel/txn` definiert die Phasen; `internal/wire` registriert
die Hooks von `authz` (Phase 1 und 3.1), `sync` (3.2) und `audit` (3.3); `platform/pg` ruft sie in
dieser Reihenfolge.

Ein Veto in Phase 2 oder 3.1 (Domänenfehler, I1-Verletzung) rollt alles zurück. Ablehnungen, die ins
Audit müssen, schreibt `auditing.Recorder` danach in einer eigenen Transaktion (03, 6.4).

### 8.2 Sperrreihenfolge

Die Reihenfolge aus 03 Abschnitt 6.3 setzt die Unit of Work durch, nicht der einzelne
Anwendungsfall: Zähler sperren nur diese Phasen-Hooks über `instance.Counters`; sonst ruft kein
Modul `Counters` auf (Prüfung über die Golden-Datei der Registrierungen, 7.3). Mehrere Zeilen derselben Tabelle sperrt ein Modul in
aufsteigender `id`. Schlägt eine Transaktion mit Deadlock (`40P01`) oder Sperr-Zeitüberschreitung
(`55P03`) fehl, wiederholt `platform/pg` den ganzen Unit of Work höchstens `db.tx_retry_max` Mal.
Das ist sicher, weil der Unit of Work vollständig zurückgerollt ist und keine externen Aufrufe enthält.

### 8.3 In-Tx-Ereignisse

- Typen liegen im Port des veröffentlichenden Moduls, Querschnitts-Ereignisse in `kernel`
  (`authorize.AccessChanged`).
- Zustellung synchron, in der Reihenfolge der Registrierung, in derselben Transaktion. Ein Abonnent
  darf schreiben, dauerhafte Ereignisse anhängen und mit einem Fehler den Commit verhindern.
- Registrierung nur in `internal/wire`, als eine Liste. Eine Golden-Datei hält sie fest; jede Änderung
  ist im Diff sichtbar. Jeder Abonnent bekommt einen eigenen OTel-Span.
- Grund für synchron statt asynchron: Die Folgen (Lauf anlegen, Entwurf leeren, Lauf abbrechen bei
  Entzug) müssen atomar mit der Ursache sein. Was nicht atomar sein muss, läuft über das
  `event_log` (Abschnitt 11).

### 8.4 Idempotenz im Unit of Work

- Nachrichten und Interaktionen: Paar aus Autor und `client_msg_id` bzw. `client_interaction_id`
  nach 04 Abschnitt 5.1.
- Alle anderen schreibenden Operationen mit `Idempotency-Key`: Der `httpapi`-Adapter öffnet den Unit of
  Work, legt die Zeile in `idempotency_keys` mit `in_progress` an, ruft den Anwendungsfall (der der
  Transaktion beitritt), speichert Status und Antwortkörper und committet alles zusammen. Eine
  gleichzeitige Wiederholung wartet am Primärschlüssel, bis die erste Transaktion endet, und liest
  dann die gespeicherte Antwort. Endet die Wartezeit mit `db.lock_timeout`, antwortet der Server
  `409 request_in_progress` (04, 5.2).
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

| Schritt | Ort | Aufgabe |
|---|---|---|
| 1 | `platform/httpserver` | Panic abfangen, Request-ID und `correlation_id` setzen |
| 2 | `platform/otel` | Span über otelhttp, Zugriffslog ohne Inhalte und ohne Tokens |
| 3 | `platform/httpserver` | Grenzen je Routenklasse: `http.max_json_body_bytes` bzw. `file.max_upload_bytes`, Kontext-Frist `http.request_timeout` bzw. `http.transfer_timeout` |
| 4 | `kernel/actor` (von `identity`) | Zugriffstoken mit DPoP oder API-Token prüfen, Sitzung, Gerät, Prinzipal-Status; ohne Token: Prinzipal `anonymous` |
| 5 | `api/routes` | Aktion und Rate-Limit-Klasse der Operation nachschlagen (nach dem Routing über `Request.Pattern` bzw. die `operationId` der Strict-Middleware [Q10, Q11, belegt]) |
| 6 | `platform/ratelimit` | Token-Bucket der Klasse, `429` mit `Retry-After` |
| 7 | `api/rest` (generiert) | Anfrage dekodieren und gegen das Schema prüfen |
| 8 | `<modul>/adapters/httpapi` | DTO bauen, Anwendungsfall aufrufen, Antwort abbilden |
| 9 | `<modul>/app` | `authorize`, Unit of Work, Domain, Repository |
| 10 | `platform/httpserver` | Fehler als `application/problem+json` (Abschnitt 14.1) |
| 11 | Strict-Middleware | Nachweis-Prüfung (9.3) |

### 9.2 404 statt 403

Ein Anwendungsfall lädt die Ressource, bevor er `authorize` ruft, und gibt bei `not_found` denselben
Fehler zurück wie bei fehlender Ressource. Gleiche Antwort, gleicher Körper, gleiche Codepfade bis
zum Fehler (05, Test 4).

### 9.3 Nachweis der Prüfung

`kernel/authorize` protokolliert im Anfrage-Kontext jede Prüfung (Aktion, Entscheidung). Nach dem
Handler prüft die Strict-Middleware: Bei Erfolg (`2xx`, `3xx`) muss mindestens eine Prüfung mit der
Aktion der Route stattgefunden haben; Listen zählen über `Scope`. Fehlt sie, wird die Antwort durch
`500` ersetzt, ein Alarm-Zähler steigt, Tests schlagen fehl. Grund: B7 verlangt, dass jede Route
prüft. Die Lint-Regel in der Spec hält nur die Deklaration; dieser Nachweis hält die Ausführung.

## 10. Gateway

Das Gateway ist Teil der Rolle `api`. Je Verbindung laufen eine Lese- und eine Schreib-Goroutine. Der
Schreiber liest aus einer begrenzten Warteschlange (`sync.outbound_queue_max`) und schreibt mit Frist
`gateway.write_timeout`. Läuft die Warteschlange über, schließt der Server mit `4014` (04, 7.2).

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
  `LISTEN`-Verbindung verloren, verbindet er mit Backoff bis `db.listen_reconnect_backoff_cap` neu
  und liest ab `local_head`. Weil `seq` lückenlos ist, geht dabei nichts verloren.
- Er gibt Ereignisse in `seq`-Reihenfolge an den Rechte-Cache und dann an den Hub, in einer
  Goroutine. Grund: Reihenfolge ist nur dann einfach zu garantieren, wenn ein Leser sie erzeugt.

### 11.3 Rechte-Cache

Der Cache liegt in `authz`, je Prozess, je Prinzipal, mit Schlüssel `permission_version`. Der
Log-Tail meldet jedes `access.changed` über `Authorizer.Observe`, bevor der Hub spätere Ereignisse
verteilt (R-SY-02). Einträge tragen eine Gültigkeit bis zum frühesten Ablauf einer beteiligten
Freigabe oder Mitgliedschaft und werden danach neu berechnet. Strukturänderungen ohne neue
Rechte-Version (`channel.created`, `channel.archived`, `space.archived`, `thread.moved`) muss der
Cache ebenfalls in Log-Reihenfolge beachten (Kern-Vertrag: Ergänzung beantragt, Abschnitt 20).

### 11.4 Hub und Zustellung

- Der Hub hält Indizes: Kanal zu Verbindungen, Prinzipal zu Verbindungen, Thread zu fokussierten
  Verbindungen.
- Für jedes Ereignis bestimmt er die Kandidaten über die Indizes und ruft je Kandidat
  `authorize(empfänger, receive_action, ressource)` aus dem Cache. Erst dann legt er den Rahmen in
  die Warteschlange der Verbindung. Der Hub blockiert nie auf einer Verbindung.
- Verbindungen im Nachholen puffern Live-Ereignisse und senden sie nach `catchup.complete` (04,
  7.2).

### 11.5 Worker-Verbraucher (Push, Webhooks)

Folgen nach außen entstehen im worker aus dem Log, nicht im Schreibpfad (R-AR-08):

1. Je Verbraucher-Name (`push`, `webhooks`) gibt es eine Cursor-Zeile in `event_consumer_cursors`
   (Kern-Vertrag: Ergänzung beantragt).
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

### 11.6 Kurzlebige Ereignisse

| Ereignis | Erzeugt in | Ein Knoten | Mehrere Knoten (`cluster.mode = multi`) |
|---|---|---|---|
| `typing.started` | `gateway` (Op `typing.start`) | direkt an den Hub | zusätzlich `NOTIFY ephemeral` mit Thread-ID und Prinzipal-ID |
| `run.updated` | `runs` (Patch gespeichert, Transaktion ohne `event_log`) | Hub sammelt je `sync.run_patch_coalesce_interval` | `NOTIFY ephemeral` mit Lauf-ID und Version; der andere Knoten lädt das Lauf-Dokument und sendet `run.updated` mit `snapshot` |
| `agent.presence_changed` | `agents` (Heartbeat) | direkt an den Hub | `NOTIFY ephemeral` mit Agent-ID und Zustand |

Grund: R-Stack erlaubt `LISTEN/NOTIFY` nur als Wecksignal ohne Inhalt. Lauf-Inhalte gehen darum
nicht über `NOTIFY`; der Empfängerknoten liest sie aus der Datenbank. Der Kanal `ephemeral` ist nur
im Modus `multi` aktiv, weil jedes `NOTIFY` beim Commit eine instanzweite Sperre nimmt [Q2, belegt]
und ein Knoten ihn nicht braucht (Kern-Vertrag: Ergänzung beantragt).

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
  Speicherort; Löschung und Datenschutz bleiben bei den Fachtabellen.

### 12.2 Warteschlangen

| Warteschlange | Inhalt | Parallelität |
|---|---|---|
| `default` | kurze Fachjobs | `jobs.queue.default.max_workers` |
| `files` | Scan, Vorschaubilder, Transkription (CPU, Speicher) | `jobs.queue.files.max_workers` |
| `delivery` | Push, Webhooks (Netz) | `jobs.queue.delivery.max_workers` |
| `maintenance` | Aufräumen, Partitionen, Prüfungen | `jobs.queue.maintenance.max_workers` |
| `import` | Importläufe | `jobs.queue.import.max_workers` |
| `privacy` | Export und Löschung | `jobs.queue.privacy.max_workers` |

Grund für getrennte Warteschlangen: Ein großer Import oder viele Vorschaubilder dürfen Push und
Webhooks nicht verzögern.

### 12.3 Job-Katalog

| Job | Modul | Auslöser | Warteschlange | Fachregel in |
|---|---|---|---|---|
| `files.scan` | `files` | `InsertTx` beim Abschluss eines Uploads | `files` | 12 |
| `files.thumbnail` | `files` | nach `clean` | `files` | 12 |
| `files.transcribe` | `files` | `file.transcribe` bei Server-Transkription | `files` | 09, 12 |
| `files.cleanup_pending` | `files` | periodisch | `maintenance` | 12 |
| `files.purge_objects` | `files` | periodisch, gelöschte Dateien | `maintenance` | 12, 20 |
| `notify.push_send` | `notify` | Verbraucher `push` | `delivery` | 13 |
| `apps.webhook_deliver` | `apps` | Verbraucher `webhooks`, `webhook_out.redeliver` | `delivery` | 11 |
| `runs.cancel_timeout` | `runs` | periodisch, `agent.cancel_grace` | `maintenance` | 09 |
| `runs.stall_check` | `runs` | periodisch, `run.stall_after` | `maintenance` | 09 |
| `runs.component_expiry` | `runs` | periodisch, `component.default_ttl` | `maintenance` | 09 |
| `authz.expiry_reminder` | `authz` | periodisch, Erinnerung vor Ablauf | `maintenance` | 07 |
| `authz.expiry_sweep` | `authz` | periodisch, `access.changed` nach Ablauf (beantragt) | `maintenance` | 07 |
| `identity.session_cleanup` | `identity` | periodisch | `maintenance` | 08 |
| `instance.key_rotation` | `instance` | periodisch, Zustände `next` bis `retired` | `maintenance` | 19 |
| `sync.partition_maintenance` | `sync` | periodisch, Partitionen anlegen und nach `sync.event_retention` abhängen | `maintenance` | 04 |
| `sync.idempotency_cleanup` | `sync` | periodisch | `maintenance` | 04 |
| `audit.chain_verify` | `audit` | periodisch | `maintenance` | 19 |
| `audit.retention_trim` | `audit` | periodisch, eigene Wartungsrolle der Datenbank | `maintenance` | 20 |
| `privacy.export`, `privacy.erase` | `privacy` | Freigabe eines Auftrags | `privacy` | 20 |
| `importer.run` | `importer` | `import.manage` | `import` | 22 |
| `billing.reconcile` | `billing` | `cost.reconcile` oder periodisch, falls eingeschaltet | `default` | 10 |
| `status.probe` | `status` | periodisch | `maintenance` | 16 |
| `retention.<klasse>` | besitzendes Modul | periodisch je Aufbewahrungsklasse | `maintenance` | 20 |

### 12.4 Regeln für Jobs

1. Jeder Job ist wiederholbar ohne doppelte Wirkung (Prüfung des Zustands vor der Wirkung, eindeutige
   Schlüssel wie `(subscription_id, event_seq)`).
2. Jeder Job reicht seinen Kontext weiter und gibt bei Abbruch `ctx.Err()` zurück; ein `nil` gilt in
   River als Erfolg [Q4, belegt].
3. Externe Aufrufe haben eine Frist (`outbound.default_timeout` oder einen eigenen Namen des
   Fachkapitels).
4. Wiederholung mit Backoff übernimmt River; Fachkapitel mit eigener Regel (Webhooks:
   `webhook.retry_backoff_base`, `webhook.retry_backoff_cap`) setzen eine eigene Wiederholungs-Regel.

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
2. **Quellen, Vorrang von niedrig nach hoch:** eingebaute Standardwerte (aus der
   Konfigurationsreferenz erzeugt) < Datei (`--config` oder `BOTHY_CONFIG_FILE`, YAML, verschachtelt
   wie die Namen) < Umgebung (`BOTHY_` plus Name in Großbuchstaben, `.` wird `__`, Beispiel
   `sync.event_retention` wird `BOTHY_SYNC__EVENT_RETENTION`) < Kommandozeile (nur `--role` und
   `--config`). Die genaue Abbildung legt ADR 0011 fest; diese ist der Vorschlag.
3. **Geheimnisse** nur als Verweis in Schlüsseln mit Endung `_ref` (z. B. `db.url_ref`). Der
   Secret-Provider löst sie beim Start auf. Werte erscheinen nie in Logs, Fehlertexten oder
   `bothy config print` (dort `<redacted>`).
4. **Prüfung beim Start, fail fast:** unbekannte Schlüssel brechen ab (Tippfehler sind sonst
   still); Typen und Bereiche; Querbedingungen. Beispiele: Identitäts-Header-Treiber nur mit
   Loopback-`http.listen_addr`; `http.max_json_body_bytes` größer als `message.max_content_bytes`
   plus Umschlag; `http.ops_listen_addr` ungleich `http.listen_addr`.
5. **Typisiert je Modul.** Jedes Modul hat eine eigene `Config`-Struktur; `internal/wire` füllt sie
   aus `platform/config`. Module lesen nie Umgebungsvariablen. Grund: Testbarkeit und eine Stelle
   für Prüfungen.
6. **Kein Nachladen im Betrieb** außer Geheimnis-Rotation (Signierschlüssel, Webhook-Geheimnisse),
   die der Secret-Provider meldet. Grund: Vorhersagbares Verhalten; halb übernommene Grenzen sind
   schwerer zu diagnostizieren als ein Neustart.
7. `bothy config check` führt Schritte 2 bis 4 aus, ohne zu starten. `bothy config print` zeigt die
   wirksamen Werte mit Quelle je Schlüssel.

## 14. Fehler- und Timeout-Strategie

### 14.1 Fehlerarten

`kernel/errs` kennt diese Arten. Jeder Fehler trägt einen stabilen `code` (snake_case). Die
`type`-URIs und den Körper nach RFC 9457 legt [06-api.md](06-api.md) fest (ADR 0015). Das Gateway
nutzt dieselben Codes in der Op `error`.

| Art | HTTP | Beispiele für `code` |
|---|---|---|
| `bad_request` | `400` | `malformed_json`, `invalid_cursor` |
| `unauthenticated` | `401` | `token_expired`, `dpop_invalid` |
| `step_up_required` | `401` | `step_up_required` |
| `forbidden` | `403` | `insufficient_level`, `capability_missing` |
| `not_found` | `404` | `not_found` (auch ohne Leserecht) |
| `conflict` | `409` | `version_conflict`, `already_started`, `request_in_progress` |
| `invariant_violation` | `409` | `i1_violation`, `last_owner` |
| `gone` | `410` | `message_deleted`, `resync_required` |
| `too_large` | `413` | `payload_too_large` |
| `unprocessable` | `422` | `validation_failed`, `idempotency_key_reuse` |
| `rate_limited` | `429` | `rate_limited` mit `Retry-After` |
| `unavailable` | `503` | `dependency_unavailable`, `db_pool_exhausted`, `shutting_down`, `deadline_exceeded` mit `Retry-After` |
| `internal` | `500` | `internal`, `unknown_action`, `authz_not_checked` |

Interne Details (SQL-Fehler, Stack) stehen nur im Log mit `correlation_id`, nie in der Antwort.

### 14.2 Fristen

| Ort | Frist | Grund |
|---|---|---|
| Kopfzeilen lesen | `http.read_header_timeout` | Schutz gegen langsame Clients |
| normale Anfrage (Kontext) | `http.request_timeout` | begrenzt Wartezeit und Ressourcen |
| Upload und Download (über `ResponseController`) | `http.transfer_timeout` | große Dateien |
| Leerlauf Keep-Alive | `http.idle_timeout` | Verbindungen freigeben |
| Verbindung aus dem Pool holen | `db.acquire_timeout` | Überlast wird `503`, nicht Warteschlange |
| SQL-Anweisung | `db.statement_timeout` (je Transaktion per `SET LOCAL`) | kein Abfrage-Ausreißer blockiert |
| Sperre warten | `db.lock_timeout` | Zähler-Sperren nie unbegrenzt |
| offene Transaktion ohne Arbeit | `db.idle_in_tx_timeout` | schützt die serialisierte Commit-Phase |
| WebSocket schreiben | `gateway.write_timeout` | langsame Leser blockieren nicht |
| Aufrufe nach außen | `outbound.default_timeout` oder Name des Fachkapitels | jeder Netzaufruf hat eine Frist |

Der Kontext eines abgebrochenen Clients bricht die Transaktion ab. Hat sie schon committet, deckt die
Idempotenz die Wiederholung des Clients.

### 14.3 Wiederholungen

- Im Anfragepfad: nur der ganze Unit of Work bei `40P01` und `55P03` (Abschnitt 8.2). Keine
  Wiederholung externer Aufrufe im Anfragepfad (es gibt dort keine).
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
mit `4000` geschlossen. Ein Panic im Log-Tail oder Hub beendet den Prozess mit Fehlercode, weil ein
Prozess ohne Fan-out nicht bereit wäre, aber noch Verbindungen hielte. Der Orchestrator startet neu,
Clients nehmen nach 04 wieder auf.

## 15. Start und Graceful Shutdown

### 15.1 Start

1. Konfiguration laden und prüfen (Abschnitt 13), Telemetrie starten.
2. Geheimnisse auflösen.
3. Datenbank erreichen, höchstens `startup.dependency_wait` lang mit Backoff.
4. Schema-Version prüfen: Das Binary kennt die kleinste nötige Version. Ist die Datenbank älter,
   bricht der Start mit Hinweis auf `bothy migrate up` ab. Neuere Versionen sind erlaubt, weil
   Migrationen nach Erweitern und Zusammenziehen gebaut sind (ADR 0007).
5. Instanz laden; ohne `setup_completed_at` startet nur der Einrichtungsweg (08).
6. Routen registrieren und Routen-Metadaten prüfen: Jede Operation hat eine bekannte Aktion, sonst
   Abbruch (05, Regel 3).
7. Rolle `api`: Log-Tail starten, `local_head` auf den aktuellen Kopf, Listener öffnen.
   Rolle `worker`: River und Verbraucher starten.
8. Bereit melden.

### 15.2 Health

Auf dem Ops-Listener `http.ops_listen_addr`, nicht in der öffentlichen API (R-AR-13):

- `live`: Prozess antwortet, Log-Tail und Hub laufen (api) bzw. River-Client läuft (worker).
- `ready`: Start abgeschlossen, Datenbank erreichbar, kein Shutdown begonnen.

Grund für den eigenen Listener: Health braucht keine Anmeldung und keine Aktion, darf aber nicht
öffentlich erreichbar sein.

### 15.3 Shutdown

Signale `SIGTERM` und `SIGINT` starten ihn, ein zweites Signal beendet sofort. Gesamtfrist
`shutdown.total_timeout`; der Orchestrator muss länger warten (Compose `stop_grace_period`,
[16-betrieb.md](16-betrieb.md)).

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
2. Verbraucher beenden ihre Runde (Commit oder Rollback) und starten keine neue.
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
  schließt mit `4015` (Fencing, Spalte beantragt). Ein Signal über `NOTIFY ephemeral` beschleunigt
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

Ziele sind benannte Werte (Abschnitt 22). Jedes Ziel hält ein Test im Gate; bis es ihn gibt, ist
das Ziel "ungeprüft" (Tickets).

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
| [10-backend-richtlinie.md](10-backend-richtlinie.md) | I1 bis I5, Umschalten, Kosten | `agents`, `authz` (I1), `runs` (I2, I3), `billing` |
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

## 19. Entscheidungen dieses Kapitels

| Nr | Entscheidung | Grund |
|---|---|---|
| R-AR-01 | Ein Binary `bothy` mit Unterbefehlen; Rollen über `--role`. | Baseline nennt den Binary-Namen `bothy`; B1 beschreibt die Server-Rolle. Ein Image für alle Rollen. |
| R-AR-02 | Module `gateway`, `search`, `stats` ohne eigene Tabellen. | Eigene Ports, Abhängigkeiten und Leistungsziele; sonst würden sie `sync`, `messaging` oder `runs` aufblähen. |
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
| R-AR-13 | Health und optionale Metriken auf eigenem Ops-Listener außerhalb der öffentlichen API. | Ohne Anmeldung nutzbar für Orchestrator, aber nicht öffentlich; keine Aktion im Katalog nötig. |
| R-AR-14 | Nachweis je Anfrage, dass `authorize` mit der Aktion der Route lief. | Hält B7 zur Laufzeit, nicht nur in der Spec. |
| R-AR-15 | Trennung Konfiguration (Betreiber, Neustart) und Einstellungen (Owner, Datenbank, sofort); unbekannte Schlüssel brechen den Start ab. | Klare Zuständigkeit; Tippfehler fallen sofort auf. |
| R-AR-16 | Migrationen nur über `bothy migrate`, nie implizit beim Start; Server prüft die Schema-Version. | Kein gleichzeitiges Migrieren mehrerer Knoten, keine Neustart-Schleife. |
| R-AR-17 | Shutdown schließt WebSockets mit `4013` und sichert Agenten-Cursor vorher. | `http.Server.Shutdown` deckt übernommene Verbindungen nicht ab; Clients nehmen geordnet wieder auf. |
| R-AR-18 | Im Anfragepfad nur Wiederholung des ganzen Unit of Work bei Deadlock und Sperr-Zeitüberschreitung. | Andere Wiederholungen gehören in Jobs oder Clients. |
| R-AR-19 | Architektur-Gate mit go-arch-lint, Konfiguration aus `modules.yaml` erzeugt. | Eine Quelle für Modulgrenzen; Werkzeug prüft Pakete und Fremdbibliotheken. |
| R-AR-20 | Leistungsziele als benannte Werte, gehalten durch Lasttests im Gate. | Keine Zahl ohne Test. |

## 20. Ergänzungen am Kern-Vertrag (beantragt)

Diese Punkte plant dieses Kapitel so, wie es richtig wäre. Der Kern-Vertrag ist dafür noch nicht
geändert.

1. **Tabelle `event_consumer_cursors`** (Modul `sync`): `name text` PK, `last_seq bigint`,
   `updated_at timestamptz`; Aufbewahrung `config`. Für die Worker-Verbraucher `push` und `webhooks`
   (Abschnitt 11.5).
2. **River-Tabellen** (`river_*`) als technische Tabellen im Schema `jobs.schema` in 03 nennen;
   Eigentümer `platform/jobs`, Aufbewahrung `transient`.
3. **Fencing der Agenten-Verbraucher:** Spalte `agent_consumer_cursors.holder_session_id uuid` für
   die Exklusivität über Knoten (Abschnitt 16).
4. **Nachholen bei neuen Kanälen:** 04 Abschnitt 7.2 filtert nur nach `$visible_channels`. Kanäle,
   die nach dem Aufbau des Caches in einem sichtbaren Bereich entstehen, fehlen dann. Vorschlag:
   zusätzlich `space_id = ANY($fully_visible_spaces)` und ein Merker für instanzweite Sicht; der
   Rechte-Cache folgt auch `channel.created`, `channel.archived`, `space.archived`, `thread.moved` in
   Log-Reihenfolge, weil diese `permission_version` nicht erhöhen.
5. **Ablauf ohne `access.changed`:** Ablaufende Freigaben und Mitgliedschaften erzeugen keine
   Transaktion und damit kein `access.changed`. Vorschlag: Job `authz.expiry_sweep` erhöht die
   Rechte-Version und schreibt `access.changed` für Betroffene. Die Durchsetzung bleibt bei jedem
   `authorize` (03, `grants.expires_at`).
6. **Thread verschieben:** 03 sollte festlegen, dass `files.scope_channel_id` nur vor dem Anhängen
   gilt und angehängte Dateien über `message_attachments` und `messages.channel_id` aufgelöst werden
   (05, 2.2). Die Kopien `read_states.channel_id` und `drafts.channel_id` ändern sich beim Verschieben
   mit.
7. **Pfad-Präfix:** 04 nennt `/v1/...` und `wss://<host>/v1/gateway`, der Auftrag an das API-Kapitel
   `/api/v1`. Eine Form muss in 04 stehen.
8. **Maschinenlesbarer Aktionskatalog:** 05 ist eine Markdown-Tabelle. Codegenerierung (Konstanten in
   `kernel/authorize`), die Lint-Regel `authz-action-known` und der Routen-Test brauchen eine
   Datenquelle. Vorschlag: `docs/spec/actions.yaml` als Quelle, Tabelle in 05 daraus erzeugt.
9. **Health ohne Aktion:** Bleibt Health auf dem Ops-Listener (R-AR-13), ist nichts zu tun. Führt die
   öffentliche API Health doch, braucht 05 eine Aktion (z. B. `health.read`, `public`).
10. **Modulliste in 03 Abschnitt 2:** `gateway`, `search`, `stats` ergänzen (ohne Tabellen).
11. **Soll-Änderung an den Agenten:** `agent.updated` hat die Empfangs-Aktion `agent.read`; der Agent
    selbst ist dort nicht genannt. Der Adapter braucht die Soll-Änderung der Backend-Richtlinie
    (Diagramm [seq-5-backend-modus.svg](../architecture/seq-5-backend-modus.svg)). Vorschlag:
    `agent.read` gilt auch für `agent-self`.
12. **Live-Box-Anker eindeutig:** Das ERD in 03 zeigt `runs` zu `messages` als eins zu eins, die
    Tabelle hat keinen eindeutigen Index. Vorschlag: `UNIQUE (run_id) WHERE run_id IS NOT NULL` auf
    `messages`.
13. **Kanal `ephemeral`:** 03 Abschnitt 6.6 und 04 kennen nur `NOTIFY event_log`. Ergänzung für
    `cluster.mode = multi` nach Abschnitt 11.6.

## 21. Offen

- Go-Modulpfad: hängt am Ort des öffentlichen Repos (Abschnitt 6). Bis zur Entscheidung steht in
  allen Plänen der Platzhalter `<go-module>`.
- Werkzeug für Go-Typen aus AsyncAPI 3.0: entscheidet [06-api.md](06-api.md); bis dahin
  handgeschriebene Typen mit Vertragstest gegen die Schemas.
- Ob `PollOnly` für River unter Last nötig ist (eigene NOTIFY-Last von River), zeigt
  `perf/event_commit`. Die Option existiert [Q3, belegt].

## 22. Konfigurationswerte dieses Kapitels

Standardwerte. Leistungsziele (`perf.*`) sind Schwellen für Tests, keine Laufzeit-Konfiguration.

| Name | Standard | Bedeutung |
|---|---|---|
| `server.role` | `all` | Rolle des Prozesses: `api`, `worker`, `all` |
| `cluster.mode` | `single` | `single` oder `multi` (mehrere api-Knoten) |
| `http.listen_addr` | `127.0.0.1:8700` | Adresse der öffentlichen API und des Gateways |
| `http.ops_listen_addr` | `127.0.0.1:8701` | Adresse für Health und optionale Metriken |
| `http.public_base_url` | leer, Pflicht | öffentliche Basis-URL (DPoP `htu`, Rückruf-URLs) |
| `http.read_header_timeout` | `5s` | Frist für Kopfzeilen |
| `http.request_timeout` | `15s` | Kontext-Frist normaler Anfragen |
| `http.transfer_timeout` | `10m` | Frist für Upload und Download |
| `http.idle_timeout` | `120s` | Leerlauf von Keep-Alive-Verbindungen |
| `http.max_header_bytes` | `65536` | größte Kopfzeilen |
| `http.max_json_body_bytes` | `1048576` | größter JSON-Körper |
| `db.url_ref` | leer, Pflicht | Verweis auf die Verbindungs-URL im Secret-Provider |
| `db.max_conns` | `32` | Verbindungen im Pool je Prozess |
| `db.acquire_timeout` | `2s` | Warten auf eine Pool-Verbindung |
| `db.statement_timeout` | `5s` | Frist je SQL-Anweisung |
| `db.lock_timeout` | `2s` | Frist für Sperren |
| `db.idle_in_tx_timeout` | `10s` | offene Transaktion ohne Arbeit |
| `db.tx_retry_max` | `2` | Wiederholungen des Unit of Work bei `40P01` und `55P03` |
| `db.listen_reconnect_backoff_cap` | `10s` | größter Abstand beim Neuaufbau von `LISTEN` |
| `startup.dependency_wait` | `60s` | Warten auf die Datenbank beim Start |
| `sync.log_poll_interval` | `1s` | Poll des Log-Tails ohne `NOTIFY` |
| `sync.tail_batch_size` | `1000` | Ereignisse je Lesevorgang des Log-Tails |
| `gateway.write_timeout` | `10s` | Frist für einen WebSocket-Schreibvorgang |
| `consumer.batch_size` | `500` | Ereignisse je Runde eines Worker-Verbrauchers |
| `consumer.poll_interval` | `1s` | Poll eines Worker-Verbrauchers ohne `NOTIFY` |
| `outbound.default_timeout` | `10s` | Frist für Aufrufe nach außen ohne eigenen Namen |
| `jobs.schema` | `jobs` | PostgreSQL-Schema der River-Tabellen |
| `jobs.queue.default.max_workers` | `8` | Parallelität `default` |
| `jobs.queue.files.max_workers` | `2` | Parallelität `files` |
| `jobs.queue.delivery.max_workers` | `16` | Parallelität `delivery` |
| `jobs.queue.maintenance.max_workers` | `2` | Parallelität `maintenance` |
| `jobs.queue.import.max_workers` | `1` | Parallelität `import` |
| `jobs.queue.privacy.max_workers` | `1` | Parallelität `privacy` |
| `shutdown.drain_delay` | `5s` | Warten nach `ready = false` |
| `shutdown.http_grace` | `20s` | Frist für laufende HTTP-Anfragen |
| `shutdown.jobs_soft_timeout` | `20s` | weicher Stopp von River |
| `shutdown.telemetry_flush_timeout` | `5s` | Telemetrie leeren |
| `shutdown.total_timeout` | `45s` | gesamte Frist bis zum Ende |
| `perf.reference_profile` | `4 vCPU, 8 GiB RAM, SSD, PostgreSQL 18 lokal, Rolle all` | Umgebung der Lasttests |
| `perf.sustained_messages_per_s` | `200` | Dauerlast für `perf.message_create_p99` |
| `perf.message_create_p99` | `150ms` | Ziel |
| `perf.event_commit_min_per_s` | `1000` | Ziel (Mindestwert) |
| `perf.fanout_local_p99` | `200ms` | Ziel |
| `perf.fanout_cross_node_p99` | `500ms` | Ziel |
| `perf.invocation_delivery_p99` | `500ms` | Ziel |
| `perf.run_patch_fanout_p99` | `500ms` | Ziel |
| `perf.catchup_page_p99` | `250ms` | Ziel |
| `perf.authorize_cached_p99` | `100us` | Ziel |
| `perf.ws_connections_per_node` | `5000` | Ziel (Mindestwert) |
| `perf.ws_idle_conn_mem_max` | `64KiB` | Ziel (Höchstwert) |
| `perf.idle_rss_max` | `256MiB` | Ziel (Höchstwert) |
| `perf.startup_ready_max` | `5s` | Ziel (Höchstwert) |
| `perf.push_handoff_p95` | `2s` | Ziel |

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
