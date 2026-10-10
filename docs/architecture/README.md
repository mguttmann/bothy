# Architektur-Diagramme

Alle Diagramme sind Mermaid-Quellen (`.mmd`) mit dem gerenderten SVG daneben. Die Quelle ist
maßgeblich; das SVG wird bei jeder Änderung neu erzeugt. Text und Regeln dazu stehen in
[../spec/02-architektur.md](../spec/02-architektur.md). Namen von Tabellen, Ereignissen und Aktionen
stammen aus dem Kern-Vertrag ([00](../spec/00-glossar.md), [03](../spec/03-datenmodell.md),
[04](../spec/04-sync-protokoll.md), [05](../spec/05-aktionen.md)).

## Index

### C4

| Diagramm | Inhalt | Quelle | Bild |
|---|---|---|---|
| Kontext | Eine Instanz mit Menschen und Fremdsystemen, je Beziehung das Protokoll. | [c4-1-kontext.mmd](c4-1-kontext.mmd) | [c4-1-kontext.svg](c4-1-kontext.svg) |
| Container | Rollen `api` und `worker` des Binarys, PostgreSQL, Objektspeicher, clamd, Collector, Apps, Adapter, Host-Dienst. | [c4-2-container.mmd](c4-2-container.mmd) | [c4-2-container.svg](c4-2-container.svg) |
| Komponenten des Servers | Module des Monolithen mit allen erlaubten Abhängigkeiten nach 02-architektur.md, 4.4. | [c4-3-komponenten-server.mmd](c4-3-komponenten-server.mmd) | [c4-3-komponenten-server.svg](c4-3-komponenten-server.svg) |

### Abläufe im Server

| Diagramm | Inhalt | Quelle | Bild |
|---|---|---|---|
| Request-Pfad | Middleware-Kette einer REST-Anfrage bis zur Nachweis-Prüfung von `authorize`. | [fluss-request-pfad.mmd](fluss-request-pfad.mmd) | [fluss-request-pfad.svg](fluss-request-pfad.svg) |
| Ereignis-Pfad | Unit of Work mit Phasen, `event_log`, Log-Tail, Hub, Agenten- und Worker-Verbraucher. | [fluss-ereignis-pfad.mmd](fluss-ereignis-pfad.mmd) | [fluss-ereignis-pfad.svg](fluss-ereignis-pfad.svg) |

### Sequenzen

| Diagramm | Inhalt | Quelle | Bild |
|---|---|---|---|
| Nachricht senden | Senden mit `client_msg_id`, Zustellung an verbundene Clients, Push an ein Gerät ohne Verbindung. | [seq-1-nachricht-senden.mmd](seq-1-nachricht-senden.mmd) | [seq-1-nachricht-senden.svg](seq-1-nachricht-senden.svg) |
| Agenten-Lauf | Auslösung, I2 und Aufruf-Marke bei Zustellung, Lauf-Start per Compare-and-swap, Live-Box-Patches, Nachtrag, Abschluss. | [seq-2-agenten-lauf.mmd](seq-2-agenten-lauf.mmd) | [seq-2-agenten-lauf.svg](seq-2-agenten-lauf.svg) |
| Wiederverbinden | `resume` oder `identify` auf einem anderen Knoten, Nachholen mit Puffer, `resync_required`. | [seq-3-wiederverbinden.mmd](seq-3-wiederverbinden.mmd) | [seq-3-wiederverbinden.svg](seq-3-wiederverbinden.svg) |
| Freigabe entziehen | Entzug bei offener Verbindung über zwei Knoten, Abbruch des laufenden Laufs. | [seq-4-freigabe-entziehen.mmd](seq-4-freigabe-entziehen.mmd) | [seq-4-freigabe-entziehen.svg](seq-4-freigabe-entziehen.svg) |
| Backend-Modus umschalten | Umschalten auf `personal` mit Step-up, I1-Prüfung, Soll und Ist am Adapter. | [seq-5-backend-modus.mmd](seq-5-backend-modus.mmd) | [seq-5-backend-modus.svg](seq-5-backend-modus.svg) |
| Push mit Extension | Push ohne Inhalt, Notification Service Extension holt die Nachricht über die API. | [seq-6-push-nse.mmd](seq-6-push-nse.mmd) | [seq-6-push-nse.svg](seq-6-push-nse.svg) |
| Datei-Upload | Upload-Sitzung, Upload direkt oder über presigned URL, Scan mit clamd, Vorschaubild. | [seq-7-datei-upload.mmd](seq-7-datei-upload.mmd) | [seq-7-datei-upload.svg](seq-7-datei-upload.svg) |
| Terminal öffnen | Step-up, `terminal.open` mit SR10 und I6, Sitzungs-Marke, Noise `NK` über das Relais, PTY erst nach `start`. | [seq-8-terminal-oeffnen.mmd](seq-8-terminal-oeffnen.mmd) | [seq-8-terminal-oeffnen.svg](seq-8-terminal-oeffnen.svg) |

### Datenmodell (ERD, aus 03-datenmodell.md, nach Modulen geteilt)

| Diagramm | Inhalt | Quelle | Bild |
|---|---|---|---|
| Instanz und Identität | Module `instance` und `identity`. | [erd-1-instanz-identitaet.mmd](erd-1-instanz-identitaet.mmd) | [erd-1-instanz-identitaet.svg](erd-1-instanz-identitaet.svg) |
| Rechte und Struktur | Module `authz` und `chat`. | [erd-2-rechte-struktur.mmd](erd-2-rechte-struktur.mmd) | [erd-2-rechte-struktur.svg](erd-2-rechte-struktur.svg) |
| Agenten und Apps | Module `agents` und `apps`. | [erd-3-agenten-apps.mmd](erd-3-agenten-apps.mmd) | [erd-3-agenten-apps.svg](erd-3-agenten-apps.svg) |
| Nachrichten und Dateien | Module `messaging` und `files`. | [erd-4-nachrichten-dateien.mmd](erd-4-nachrichten-dateien.mmd) | [erd-4-nachrichten-dateien.svg](erd-4-nachrichten-dateien.svg) |
| Läufe und Kosten | Module `runs` und `billing`. | [erd-5-laeufe-kosten.mmd](erd-5-laeufe-kosten.mmd) | [erd-5-laeufe-kosten.svg](erd-5-laeufe-kosten.svg) |
| Sync und Betrieb | Module `sync`, `notify`, `importer`, `audit`, `privacy`, `status`. | [erd-6-sync-betrieb.mmd](erd-6-sync-betrieb.mmd) | [erd-6-sync-betrieb.svg](erd-6-sync-betrieb.svg) |
| Terminal | Modul `terminal`: Rechner und Terminal-Sitzungen. | [erd-7-terminal.mmd](erd-7-terminal.mmd) | [erd-7-terminal.svg](erd-7-terminal.svg) |

## Konventionen

- **C4 als Flowchart.** Kontext, Container und Komponenten nutzen die C4-Notation (Person,
  System, Container, extern) als Mermaid-Flowchart. Grund: Der C4-Modus von Mermaid ordnet die Kästen
  in festem Raster an, und bei mehr als einer Handvoll Beziehungen überlappen die Beschriftungen.
  Container- und Komponenten-Diagramm nutzen das ELK-Layout (`config.layout: elk`).
- **Pfade ohne Präfix.** REST-Pfade stehen ohne Versions-Präfix; das Präfix legt
  [../spec/06-api.md](../spec/06-api.md) fest. Wo es klarer ist, steht die Aktion statt des Pfads
  (z. B. `run.finish`).
- **ERDs.** Jede Tabelle aus 03 steht genau einmal mit Spalten (Schlüssel, Fremdschlüssel,
  Zustandsspalten), in dem Diagramm ihres Moduls. Tabellen anderer Module erscheinen dort nur mit
  Schlüssel. Jede Fremdschlüssel-Beziehung aus 03 ist gezeichnet, außer reinen Protokoll-Spalten
  (`created_by`, `updated_by`, `granted_by`, `revoked_by` und ähnliche, alle auf `principals(id)`).
  Vollständige Spalten, Typen und Indexe stehen nur in 03.
- **Modulkanten.** Ein Pfeil im Komponenten-Diagramm heißt "darf den Port aufrufen". Kanten zu
  `internal/kernel` sind nicht gezeichnet, weil jedes Modul sie hat.

## Rendern und prüfen

Mit mermaid-cli (headless Chromium):

```sh
npx -y @mermaid-js/mermaid-cli@11 -i docs/architecture/<name>.mmd -o docs/architecture/<name>.svg
```

Für Chromium ohne Sandbox braucht mermaid-cli eine Puppeteer-Konfiguration (`-p <datei>`), die
außerhalb des Repos liegt. Ein Diagramm, das nicht rendert, ist nicht fertig. Den Abgleich der
Diagramme mit dem Kern-Vertrag (jede Tabelle, jede Fremdschlüssel-Beziehung, jede Modulkante) soll
das Doku-Gate automatisch prüfen; siehe [../spec/02-architektur.md](../spec/02-architektur.md),
Abschnitt 7.3.
