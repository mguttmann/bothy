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
| Terminal öffnen | Step-up, `terminal.open` mit SR10, I6 und SR12, Sitzungs-Marke, Noise `NK` über das Relais, PTY erst nach `start`. | [seq-8-terminal-oeffnen.mmd](seq-8-terminal-oeffnen.mmd) | [seq-8-terminal-oeffnen.svg](seq-8-terminal-oeffnen.svg) |

### Datenmodell (ERD, aus 03-datenmodell.md, nach Modulen geteilt)

| Diagramm | Inhalt | Quelle | Bild |
|---|---|---|---|
| Instanz und Identität | Module `instance` und `identity`. | [erd-1-instanz-identitaet.mmd](erd-1-instanz-identitaet.mmd) | [erd-1-instanz-identitaet.svg](erd-1-instanz-identitaet.svg) |
| Rechte und Struktur | Module `authz` und `chat`. | [erd-2-rechte-struktur.mmd](erd-2-rechte-struktur.mmd) | [erd-2-rechte-struktur.svg](erd-2-rechte-struktur.svg) |
| Agenten und Apps | Module `agents` und `apps`. | [erd-3-agenten-apps.mmd](erd-3-agenten-apps.mmd) | [erd-3-agenten-apps.svg](erd-3-agenten-apps.svg) |
| Nachrichten und Dateien | Module `messaging` und `files`. | [erd-4-nachrichten-dateien.mmd](erd-4-nachrichten-dateien.mmd) | [erd-4-nachrichten-dateien.svg](erd-4-nachrichten-dateien.svg) |
| Läufe und Kosten | Module `runs` und `billing`. | [erd-5-laeufe-kosten.mmd](erd-5-laeufe-kosten.mmd) | [erd-5-laeufe-kosten.svg](erd-5-laeufe-kosten.svg) |
| Sync | Modul `sync`: Ereignis-Log, Idempotenz, Gelesen-Stand, Entwürfe, Einstellungen. | [erd-6-sync.mmd](erd-6-sync.mmd) | [erd-6-sync.svg](erd-6-sync.svg) |
| Terminal | Modul `terminal`: Rechner und Terminal-Sitzungen. | [erd-7-terminal.mmd](erd-7-terminal.mmd) | [erd-7-terminal.svg](erd-7-terminal.svg) |
| Audit und Datenschutz | Module `audit` und `privacy`. | [erd-8-audit-datenschutz.mmd](erd-8-audit-datenschutz.mmd) | [erd-8-audit-datenschutz.svg](erd-8-audit-datenschutz.svg) |
| Import, Status, Statistik | Module `importer`, `status` und `stats`. | [erd-9-import-status-statistik.mmd](erd-9-import-status-statistik.mmd) | [erd-9-import-status-statistik.svg](erd-9-import-status-statistik.svg) |
| Benachrichtigung | Modul `notify`. | [erd-10-benachrichtigung.mmd](erd-10-benachrichtigung.mmd) | [erd-10-benachrichtigung.svg](erd-10-benachrichtigung.svg) |

## Konventionen

- **C4 als Flowchart.** Kontext, Container und Komponenten nutzen die C4-Notation (Person,
  System, Container, extern) als Mermaid-Flowchart. Grund: Der C4-Modus von Mermaid ordnet die Kästen
  in festem Raster an, und bei mehr als einer Handvoll Beziehungen überlappen die Beschriftungen.
  Container- und Komponenten-Diagramm nutzen das ELK-Layout (`config.layout: elk`).
- **Pfade ohne Präfix.** REST-Pfade stehen ohne Versions-Präfix; das Präfix legt
  [../spec/06-api.md](../spec/06-api.md) fest. Wo es klarer ist, steht die Aktion statt des Pfads
  (z. B. `run.finish`).
- **ERDs.** Die Dateien `erd-*.mmd` sind die einzigen ERDs des Projekts; 03 verweist nur auf sie.
  Jede Tabelle aus 03, Abschnitt 4 (außer den technischen Tabellen aus 4.13) steht genau einmal mit
  Spalten (Schlüssel, Fremdschlüssel, Zustandsspalten), in dem Diagramm ihres Moduls. Tabellen
  anderer Module erscheinen dort nur mit Schlüssel. Jede Fremdschlüssel-Beziehung aus 03 ist
  gezeichnet, so oft wie es Fremdschlüssel zwischen den beiden Tabellen gibt; ausgenommen sind reine
  Protokoll-Spalten (Name endet auf `_by`, Ziel `principals(id)`). Vollständige Spalten, Typen und
  Indexe stehen nur in 03. ERDs mit vielen Kind-Tabellen nutzen `direction LR`, damit sie nicht in
  die Breite wachsen.
- **Lesbarkeit.** Kein Diagramm ist breiter oder höher als das Seitenverhältnis `MAX_ASPECT` in
  `tools/docs/render_diagrams.py`. Wird eines zu breit, wird es nach Modulen geteilt oder gedreht.
- **Farben und Schrift aus dem Design-System.** Alle Diagramme rendern mit
  `design/build/mermaid.config.json` (Mermaid-Thema `base`), das `tools/design-tokens/tokens.py`
  aus den Tokens des Themes light erzeugt. Knotenklassen (`classDef`) nutzen nur Token-Farben:
  Akzent (Fichte) für eigene Container und Schritte, See für Datenspeicher, Moos für Rand-Module,
  Ginster für Prüfungen, Granit für Fremdsysteme. Die Doku-Site färbt ihre eingebetteten
  Mermaid-Blöcke über `docs/design/site-theme.css` (ebenfalls erzeugt) passend zum Farbschema.
- **Modulkanten.** Ein Pfeil im Komponenten-Diagramm heißt "darf den Port aufrufen". Kanten zu
  `internal/kernel` sind nicht gezeichnet, weil jedes Modul sie hat.

## Rendern und prüfen

Neu rendern (alle `.svg` oder nur die genannten), danach laufen alle Prüfungen:

```sh
uv run --project tools --locked --quiet python tools/docs/render_diagrams.py --write [docs/architecture/<name>.mmd ...]
```

Prüfen: `make diagrams`. Das Gate prüft, dass jedes Diagramm rendert, jedes `.mmd` ein `.svg` mit
dem Design-Thema hat, das Seitenverhältnis `MAX_ASPECT` nicht überschreitet, `.mmd`-Dateien nur
Token-Farben nennen und die ERDs zu 03 passen (`tools/docs/erd_check.py`: jede Tabelle genau
einmal vollständig, jede Fremdschlüssel-Beziehung gezeichnet). Für Chromium ohne Sandbox braucht
mermaid-cli eine Puppeteer-Konfiguration außerhalb des Repos (`MMDC_PUPPETEER_CONFIG`). Den Abgleich
der Modulkanten beschreibt [../spec/02-architektur.md](../spec/02-architektur.md), Abschnitt 7.3.
