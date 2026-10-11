# Spezifikation: Einstieg

Status: Entwurf M0, noch kein Produktcode. Dieses Dokument ist der Einstieg in
die Spezifikation von Bothy (Arbeitsname). Es sagt, was das Produkt ist, welche Regeln nie brechen
dürfen und wo was steht. Es wiederholt die Kapitel nicht; die Regeln selbst stehen in den Kapiteln
unter [spec/](spec/00-glossar.md).

## 1. Zweck und Leser

Leser sind Bau-Agenten und Menschen, die Bothy bauen, prüfen, betreiben oder erweitern. Nach diesem
Dokument weiß ein Leser, welches Kapitel für seine Frage maßgeblich ist und in welcher Reihenfolge
er liest (Abschnitt 11).

Die Spezifikation ist ein Einstieg mit Kapiteln statt einer einzigen großen Datei, weil so jedes
Thema genau eine Quelle hat, die Doku-Site jedes Kapitel als eigene Seite führt und ein Review genau
das geänderte Kapitel prüft.

## 2. Produkt in einem Absatz

Bothy ist eine **Kommandozentrale für KI-Agenten in Chat-Form**: Chat, Live-Ansicht, Freigaben,
Terminal, Kosten und Statistik in einer selbst gehosteten, quelloffenen App (Wortlaut des Owners,
[01](spec/01-produkt.md) "Vision", R-PR-13). Aufbau und Bedienung folgen Discord. Agenten wie
Hermes Agent, Claude Code und weitere sind vollwertige Teilnehmer und schließen über die öffentliche
API an. Jeder Thread kann eine Agenten-Sitzung sein, mit Live-Box, Rückfragen und Freigabe-Knöpfen.
Ein Terminal öffnet Shells und Sitzungen eines Coding-Agenten auf angebundenen Rechnern. Native
Ansichten zeigen, was die Agenten tun, worauf sie laufen und was es kostet. Jede Person sieht und
steuert nur, was ihr freigegeben ist. Eine Instanz gehört genau einer Organisation (B13).

## 3. Ziele

| Ziel | Woran es sich zeigt | Kapitel |
|---|---|---|
| Mit Agenten arbeiten wie mit Menschen im Chat | Threads als Agenten-Sitzungen, Live-Box, Nachträge, Komponenten, Slash-Befehle | [09](spec/09-agenten.md), [01](spec/01-produkt.md) |
| Rechte gelten überall | eine Prüfstelle für jede Anfrage, Zustellung, Suche, jeden Push und Datei-Abruf | [05](spec/05-aktionen.md), [07](spec/07-rechte.md) |
| Nichts geht verloren, nichts doppelt | commit-geordnetes Ereignis-Log, Cursor, idempotente Schreibanfragen, Offline-Ausgangsfach | [04](spec/04-sync-protokoll.md), [15](spec/15-client-architektur.md) |
| Kosten und Abrechnungsart sind sichtbar und erzwungen | Backend-Richtlinie je Agent mit Invarianten I1 bis I6 | [10](spec/10-backend-richtlinie.md) |
| Terminal als Kernfunktion mit eingebauter Sicherheit | eigenes Recht je Rechner, Step-up je Öffnung, Ende-zu-Ende-Strom, I6 | [24](spec/24-terminal.md) |
| Statistik, die man versteht | eigene Messung, Live-Topologie, TV-Modus, Grafana-Panel-Plugin in derselben Designsprache | [25](spec/25-visualisierung.md) |
| Eigene Instanz auf eigener Hardware oder in der eigenen Cloud | ein Binary, PostgreSQL, Objektspeicher, Compose-Installation, gleiche Images überall | [02](spec/02-architektur.md), [16](spec/16-betrieb.md) |
| Offen erweiterbar | öffentliche, versionierte API; SDKs; Apps und Webhooks außerhalb des Servers | [06](spec/06-api.md), [11](spec/11-erweiterungen.md) |
| Schaufenster-Anspruch | wer README oder Doku-Site zum ersten Mal sieht, versteht in einer Minute, was Bothy kann | [01](spec/01-produkt.md) "Vision", [PLAN](PLAN.md) M7 |

## 4. Nicht-Ziele

Funktionen, die Bothy bewusst nie anbietet. Die vollständige Liste mit Grund je Zeile steht in
[01](spec/01-produkt.md) "Funktionsliste" (Etappe "nie").

- Sprach- und Video-Kanäle, Bildschirm teilen.
- Mehrere Server in einer Instanz, Server-Leiste, öffentliche Einladungslinks (B13).
- Rollen-Hierarchie wie Discord; Freigaben mit Stufen ersetzen sie ([ADR 0018](adr/0018-freigabe-modell-stufen-zusatzrechte-vererbung-kein-verbot.md)).
- Threads in Threads, Umfragen, Sticker, GIF-Suche, @everyone und @here.
- Präsenz und Lesebestätigungen von Menschen (R-PR-02, R-PR-03).
- E-Mail-Benachrichtigungen und Link-Vorschau vom Server.
- Ende-zu-Ende-Verschlüsselung von Chat-Inhalten, weil Agenten mitlesen müssen.
- Chat im Browser; das Web-Ziel ist nur die Konsole ([ADR 0028](adr/0028-web-ziel-nur-fuer-admin-und-statistik-konsole.md), [ADR 0070](adr/0070-terminal-in-der-web-konsole.md)).
- Marktplatz für Bots fremder Anbieter.
- Aufzeichnung von Terminal-Inhalten und Terminal für Agenten oder Apps.

## 5. Anti-Ziele

Eigenschaften, die Bothy auch nebenbei nie bekommen darf. Jede Änderung, die eine davon erlaubt,
ist ein Fehler, auch wenn sie eine Funktion bringt.

| Anti-Ziel | Was es verhindert |
|---|---|
| Ein Weg am Rechte-Modell vorbei, auch über das Terminal, einen Adapter oder eine App | B7, [07](spec/07-rechte.md) "Schutzregeln, die keine Freigabe aufhebt", [24](spec/24-terminal.md) "Zweck und Grenzen" |
| Der Server speichert, sammelt oder vermittelt Zugangsdaten eines KI-Abos | B10, I5, [10](spec/10-backend-richtlinie.md) "Begründung aus den Anthropic-Bedingungen" |
| Ein Weg, das persönliche Abo eines anderen Menschen zu nutzen | I1 bis I3, I6 |
| Agenten-, Plugin- oder App-Code im Server-Prozess | B6, [ADR 0022](adr/0022-agenten-und-plugins-ausserhalb-des-server-prozesses.md) |
| Ein bestimmtes Agenten-Framework als Kern; Hermes Agent ist ein Adapter unter mehreren | [09](spec/09-agenten.md) "Grundsätze" |
| Nachrichteninhalte in Telemetrie; Terminal-Inhalte im Server, in Telemetrie, Audit, Suche, Push oder im Kontext eines Agenten | B11, [17](spec/17-observability.md), [24](spec/24-terminal.md) |
| In-App-Statistik, die vom Telemetrie-Stack abhängt | [17](spec/17-observability.md) "In-App-Statistik", [25](spec/25-visualisierung.md) "Eigene Messung" |
| Fremde Webseiten (iframes) in den nativen Apps | [ADR 0069](adr/0069-visualisierung-auf-zwei-ebenen-ohne-iframes-in-apps.md) |
| Instanzspezifisches im Produkt-Repo | White Label (Abschnitt 6.4) |
| Abgespeckter Komfort für fremde Betreiber (Einrichtung, SDK, Doku, DE und EN, Barrierefreiheit) | Vorgabe des Owners zum Schaufenster-Anspruch |

## 6. Harte Invarianten

Diese Regeln gelten in jeder Etappe. Prüfort und Test jeder Regel nennt das verlinkte Kapitel. Eine Änderung, die eine Invariante schwächt, braucht eine neue ADR.

### 6.1 Autorisierung

- Eine Prüfstelle: `authorize(principal, action, resource) -> Decision` im Modul `authz`. Jede
  Route, jede WebSocket-Op, jede Zustellung, jeder Push, jede Suche, jeder Datei-Abruf, jeder
  Agenten-Aufruf und jede Terminal-Öffnung geht hindurch (B7, [05](spec/05-aktionen.md)
  "Verbindliche Regeln").
- Registrierungszwang: Jede Operation in `openapi.yaml` trägt genau eine Aktion aus dem
  Aktionskatalog; Lint, Start-Prüfung und ein Zähl-Test brechen bei einer Route ohne Aktion
  ([ADR 0017](adr/0017-eine-autorisierungsfunktion-404-statt-403-registrierungszwang.md)).
- Kein Sichtrecht ergibt `404`, nicht `403`. Listen, Zähler und Suche filtern in der Abfrage.
- Fail closed: Jeder Fehler in `authz` ergibt "nein". Rechte gelten im Moment der Zustellung; ein
  Entzug wirkt sofort und bricht laufende Läufe und Terminal-Sitzungen ab (L23).
- Schutzregeln SR1 bis SR12 hebt keine Freigabe auf, darunter: nur Menschen lösen Agenten aus,
  Verwaltung nur mit Step-up, Direktnachrichten nur für Teilnehmer, auch nicht für Owner
  ([07](spec/07-rechte.md) "Schutzregeln, die keine Freigabe aufhebt").

### 6.2 Backend-Richtlinie und Terminal (I1 bis I6)

Wortlaut und Prüfstellen: [10](spec/10-backend-richtlinie.md) "Invarianten". Kurzfassung:

| Nr | Regel |
|---|---|
| I1 | Ein Agent mit `billing_mode = personal` hat genau einen Auslöser: seinen `credential_holder`. Der Server lehnt jede Änderung an Freigaben, Gruppen, Owner-Liste, Bediener oder Richtlinie ab, die das verletzen würde. Keine Owner-Ausnahme (L22). |
| I2 | Unmittelbar vor jedem Agenten-Aufruf prüft der Dispatch-Pfad I1 noch einmal; sonst kein Aufruf, Audit-Eintrag. |
| I3 | Jeder Aufruf trägt eine signierte, kurzlebige Aufruf-Marke (EdDSA mit JWKS, L24). Der Adapter startet keinen Lauf, wenn Marke oder Ist-Modus nicht passen. |
| I4 | Umschalten der Richtlinie ist eine Admin-Aktion mit Step-up, Grund und Audit; auf `personal` nur, wenn I1 danach gilt. |
| I5 | Der Server kennt keine Zugangsdaten. API-Keys liegen beim Adapter, das persönliche Abo bleibt in der Laufzeit der Person. |
| I6 | Auf einem Rechner mit `llm_credentials = personal` öffnet nur der Inhaber der Anmeldung ein Terminal, auch kein anderer Owner ([24](spec/24-terminal.md) "LLM-Anmeldung und Invariante I6"). |

Dazu kommt Ruling T12: ein Sitzungsbenutzer je Person und Rechner, nie geteilt; ohne Zuordnung kein
Terminal ([07](spec/07-rechte.md) SR12). Ruling T13 ergänzt: Kein Ort auf dem Rechner verbindet
diese Konten wieder (Arbeitsordner und `CLAUDE_CONFIG_DIR` je Sitzungsbenutzer, Prüfung HD11 für
jeden konfigurierten Pfad). Eine Abo-Anmeldung erreicht so nie die Sitzung eines anderen Menschen;
die Grenzen nennt [24](spec/24-terminal.md) Abschnitt 2.2.

### 6.3 Sync-Garantien

Normativ in [04](spec/04-sync-protokoll.md):

- Ein Client, der wiederkommt, bekommt alles, was er sehen darf, oder die klare Anweisung, neu zu
  laden. Dauerhafte Ereignisse kommen mindestens einmal, der Client verwirft Doppel über den Cursor.
- Die globale Sequenz ist commit-geordnet: Nummernfolge gleich Commit-Folge
  ([ADR 0009](adr/0009-commit-geordnetes-ereignis-log-mit-globaler-sequenz.md)).
- Die Reihenfolge in einem Thread ist für alle Empfänger gleich. Zeitstempel ordnen nie etwas.
- Jede Schreibanfrage trägt eine Client-ID und ist wiederholbar; eine Wiederholung liefert das
  Original.
- Ein Agenten-Adapter ist ein normaler Client mit dauerhaftem Cursor: Er verpasst nichts und startet
  nichts doppelt.

### 6.4 White Label

- Das Produkt-Repo enthält nichts, was nur für eine bestimmte Instanz gilt: keine Firmen, Personen,
  Hostnamen, Domains außer `example.com` und `example.org`, keine internen Systemnamen. Was nur
  eine Instanz betrifft, steht im privaten Repo ihres Betreibers
  ([ADR 0002](adr/0002-zwei-repos-oeffentliches-produkt-privates-ops-repo.md)).
- Der angezeigte Name ist Konfiguration (`branding.product_name`); der Arbeitsname steht im Code nur
  an gelisteten Stellen ([ADR 0044](adr/0044-arbeitsname-und-branding-als-konfiguration.md)).
- Ein Deny-List-Scanner prüft Inhalte, Dateinamen, Commits, Branches und Tags und läuft in
  `make check` ([23](spec/23-qualitaet.md) "White Label",
  [ADR 0042](adr/0042-deny-list-scanner-mit-liste-aus-dem-ops-repo.md)).

### 6.5 Datenschutz

Normativ in [20](spec/20-datenschutz.md):

- Keine Nachrichteninhalte, Prompts, Dateinamen, Suchbegriffe oder Geheimnisse in Telemetrie (B11).
  Das Audit-Log ist getrennt von Telemetrie ([ADR 0021](adr/0021-audit-log-hash-verkettet-und-getrennt-von-telemetrie.md)).
- Push trägt nur IDs und einen neutralen Platzhalter; den Inhalt holt die App selbst
  ([ADR 0031](adr/0031-push-ohne-inhalt-inhalt-holt-die-app-selbst.md)).
- Jede Tabelle hat eine Aufbewahrungsklasse mit benannter Frist; Löschung, Auskunft und Export je
  Person sind eingebaute Abläufe ([20](spec/20-datenschutz.md) "Aufbewahrung und Löschen",
  "Auskunft, Export, Berichtigung").
- Bei JPEG und PNG entfernt der Server Metadaten wie Standort und Gerät schon beim Upload ([ADR 0047](adr/0047-bild-metadaten-verlustfrei-entfernen.md)).
- Terminal-Inhalte laufen Ende-zu-Ende verschlüsselt zwischen Gerät und Rechner; gespeichert werden
  nur Metadaten der Sitzung ([24](spec/24-terminal.md) "Audit, Ereignisse, Datenschutz").

## 7. Systemüberblick

![C4 Ebene 2: Container einer Bothy-Instanz mit Apps, Web-Konsole, Agenten-Adaptern, Host-Dienst, dem Binary bothy in den Rollen api und worker, PostgreSQL, Objektspeicher, ClamAV und OTel Collector](architecture/c4-2-container.svg)

Ein Binary `bothy` läuft in den Rollen `api` und `worker`. PostgreSQL und ein Objektspeicher sind
der einzige Zustand; es gibt keinen Broker und keinen Cache-Dienst. Apps, Web-Konsole,
Agenten-Adapter, Integrationen und der Host-Dienst `bothy-host` sprechen nur die öffentliche API.
Agenten-Laufzeiten und der Host-Dienst laufen außerhalb des Servers. Telemetrie geht nur über einen
OTel Collector. Einzelheiten: [02](spec/02-architektur.md) "Container (C4 Ebene 2)" und alle
Diagramme im [Architektur-Index](architecture/README.md).

## 8. Kapitel

Spalte "Normativ für": wofür das Kapitel die maßgebliche Quelle ist. Bei Widerspruch zwischen
Kapiteln gilt der Kern-Vertrag (00, 03, 04, 05), weil alle anderen Kapitel auf ihm aufbauen; für
die Form der API gelten die YAML-Dateien (Abschnitt 9). Ein Widerspruch ist ein Fehler und wird in
der Änderung behoben, die ihn findet.

| Kapitel | Inhalt | Normativ für |
|---|---|---|
| [00 Glossar](spec/00-glossar.md) | Alle Begriffe mit deutschem Namen, englischem Code-Namen und Definition. Teil des Kern-Vertrags. | Begriffe und Code-Namen in API, Datenbank und Code |
| [01 Produkt](spec/01-produkt.md) | Vision, Personas, Funktionsliste mit Etappe, Screens als ASCII-Wireframes, Interaktionen, Tastenkürzel, Barrierefreiheit, Sprachen. Jede Bedienung nennt ihre Aktion; Rechte legt das Kapitel nicht fest. | Funktionsumfang und Bedienung |
| [02 Architektur](spec/02-architektur.md) | Modularer Monolith mit hexagonalen Modulen, Container, Request- und Ereignis-Pfad, Jobs, Fehler- und Timeout-Strategie, Skalierungspfad. Dazu die automatische Prüfung der Modulgrenzen. | Aufbau von `services/server/` |
| [03 Datenmodell](spec/03-datenmodell.md) | Alle Tabellen in PostgreSQL mit Spalten, Checks, Indizes, Aufbewahrungsklasse und ERD. Transaktionsregeln für die Invarianten. Teil des Kern-Vertrags. | Migrationen, sqlc-Abfragen, Repository-Ports |
| [04 Sync-Protokoll](spec/04-sync-protokoll.md) | Ereignisse, globale und Thread-Sequenz, Gateway, Nachholen, Sichtbarkeit bei Rechte-Änderungen, Offline und Live-Box-Streaming. Teil des Kern-Vertrags. | Gateway, Nachholschnittstelle, Clients, Adapter |
| [05 Aktionen](spec/05-aktionen.md) | Der geschlossene Aktionskatalog mit Stufe, Zusatzrecht und Rate-Limit-Klasse je Aktion. Dazu Agenten-Aufruf, Ereignistypen und WebSocket-Ops. Teil des Kern-Vertrags. | jede Prüfung durch `authorize` |
| [06 API](spec/06-api.md) | Konventionen der öffentlichen API: Authentifizierung je Prinzipal, Fehler nach RFC 9457, Paginierung, Idempotenz, Versionierung. Liste der Endpunkte und Vertragstests. | Konventionen der API (Formen: Abschnitt 9) |
| [07 Rechte](spec/07-rechte.md) | Freigabe-Modell mit Stufen, Zusatzrechten und Vererbung, die Funktion `authorize` und die Schutzregeln SR1 bis SR12. Dazu Rechte-Cache, Zustellfilter und Anforderungen an die Admin-Oberfläche. | Modul `authz` |
| [08 Identität](spec/08-identitaet.md) | Identitätsanbieter (OIDC, Proxy-Header, lokale Konten), Geräte mit Schlüssel und DPoP, Step-up, Token-Modell. Dazu Sperren und API-Tokens. | Modul `identity`, Anmeldung |
| [09 Agenten](spec/09-agenten.md) | Agenten als Prinzipale, Bediener, Adapter-Vertrag, Lauf und Live-Box, Nachträge, Slash-Befehle, Komponenten. Dazu die Referenz-Adapter für Hermes Agent und den Claude-Code-Runner. | Module `agents` und `runs`, jeder Adapter |
| [10 Backend-Richtlinie](spec/10-backend-richtlinie.md) | Abrechnungsart `personal` oder `organization` je Agent mit den Invarianten I1 bis I6 und ihren Prüfstellen. Dazu Umschalten, Kosten und Anzeige. | Module `agents`, `authz`, `billing` |
| [11 Erweiterungen](spec/11-erweiterungen.md) | Apps mit Manifest und Scopes, ausgehende und eingehende Webhooks, SDK-Design. Dazu ein Leitfaden, einen eigenen Agenten anzubinden. | Modul `apps`, SDKs |
| [12 Dateien](spec/12-dateien.md) | Upload in Teilen, Objektspeicher-Port, Malware-Scan, Bilder ohne Metadaten, Sprachnachrichten. Abruf nur über den Server mit kurzlebigen Tickets. | Modul `files` |
| [13 Push und Benachrichtigungen](spec/13-push-benachrichtigungen.md) | Arten von Hinweisen, Einstellungen je Kanal und Thread, Ruhezeiten, Push ohne Inhalt je Plattform. Dazu Zustell-Messung. | Modul `notify`, Push-Adapter |
| [14 Suche](spec/14-suche.md) | Volltextsuche in PostgreSQL mit Rechtefilter in SQL, Sprachen, Ranking, Dateitext und Offline-Suche im Client. Dazu Leistungsziele. | Suche im Server und im Client |
| [15 Client-Architektur](spec/15-client-architektur.md) | Flutter-Workspace, lokale Datenbank als Quelle der Oberfläche, Ausgangsfach, Sync-Engine, Geräteschlüssel, Hintergrund und Push. Dazu Auto-Update, Teststrategie und die Terminal-Ansicht. | `apps/` und `packages/` |
| [16 Betrieb](spec/16-betrieb.md) | Bausteine und Deployment-Ziele, Netz, Konfiguration und Geheimnisse, Sicherung, Wiederherstellung, Upgrade, Umzug. Dazu Statusseite und Kapazität. | Deployment und Betrieb |
| [17 Observability](spec/17-observability.md) | Regeln für Traces, Metriken und Logs, Datenschutz der Telemetrie, Collector, SLOs, Alarme, In-App-Statistik. Einzelheiten unter [observability/](observability/signals.md). | Telemetrie und Abgrenzung zum Audit |
| [18 Release](spec/18-release.md) | Komponenten mit eigener Version, Changelogs, Kanäle, Signatur, SBOM, Provenance, App-Verteilung. Dazu Kompatibilität zwischen App und Server und Rollback. | Versionierung und Releases |
| [19 Sicherheit](spec/19-sicherheit.md) | Kontrollen mit ID und Test: Transport, Kopfzeilen, CSP, Eingaben, Uploads, SSRF-Verbot, Geheimnisse, Agenten, Terminal, Visualisierung. Die Analyse steht im [Bedrohungsmodell](security/threat-model.md). | Sicherheitskontrollen |
| [20 Datenschutz](spec/20-datenschutz.md) | Rollen nach DSGVO, Datenkategorien je Tabelle, Aufbewahrung und Löschen, Auskunft und Export, LLM-Anbieter, Hinweispflichten. Keine Rechtsberatung, sondern Mechanismen und Vorlagen. | Aufbewahrung, Löschung, Export |
| [21 Dev-Abschottung](spec/21-dev-isolation.md) | Warum ein Runner als ein OS-Benutzer für weitere Personen unsicher ist und welche Abschottung vor der Freischaltung gelten muss. Spätere Etappe. | Schutzregel SR3, Datensatz `dev_environments` |
| [22 Import und Migration](spec/22-import-migration.md) | Import aus Discord nur lesend, mit Trockenlauf, Herkunftsschlüssel, Prüfbericht und Lese-Spiegel. Dazu Umschalten und Rückweg je Kanal. | Import-Werkzeug und Import-Schnittstelle |
| [23 Qualität](spec/23-qualitaet.md) | Gates hinter `make check`, identisch lokal und in CI, Komponenten-Vertrag, Push-Disziplin, Doku im Code, White Label, SonarQube, Lieferkette. Dazu das unabhängige Review je Push und das Red-Team-Gate je Etappe. | Prüfungen jeder Änderung |
| [24 Terminal](spec/24-terminal.md) | Terminal auf angebundenen Rechnern über den Host-Dienst `bothy-host` mit Noise-verschlüsseltem Strom. Dazu Kopplung, Sitzungen, Rechte, Invariante I6, Mehrknoten-Betrieb und Tests. | Modul `terminal`, Host-Dienst, Terminal-Ansichten |
| [25 Visualisierung](spec/25-visualisierung.md) | Eigene Messung des Servers, Live-Strom `stats.live`, native Ansichten im Paket `bothy_viz` und das Grafana-Panel-Plugin "Bothy Live". Dazu die optionale Einbettung in die Web-Konsole. | Statistik-Ansichten und Plugin |

## 9. Wo die API normativ steht

| Was | Datei |
|---|---|
| REST (OpenAPI 3.1): Pfade, Felder, Typen, Aktion und Rate-Limit-Klasse je Operation | [api/openapi.yaml](api/openapi.yaml) |
| Echtzeit-Gateway (AsyncAPI 3.0): Kanäle, Ops, Ereignis-Umschlag | [api/asyncapi.yaml](api/asyncapi.yaml) |
| Webhooks nach Standard Webhooks | [api/webhooks.md](api/webhooks.md) |
| Konventionen und Rangfolge bei Widerspruch | [06](spec/06-api.md) |

Spec first: Server-Stubs und Clients werden aus den YAML-Dateien erzeugt, nie umgekehrt
([ADR 0013](adr/0013-spec-first-openapi-31-und-asyncapi-30-normativ.md)). Ein Gate prüft, dass der
erzeugte Code zur Spezifikation passt.

## 10. Weitere Dokumente

| Dokument | Zweck |
|---|---|
| [PLAN](PLAN.md) | Etappen, Exit-Kriterien, Bau-Tickets und ihre Abhängigkeiten |
| [Bau-Briefs](briefs/README.md) | ein Auftrag je Ticket der laufenden Etappe, für Bau-Sitzungen |
| [DECISIONS](DECISIONS.md) | Ruling-Log: jede Entscheidung unterhalb einer ADR mit Grund und Ort |
| [OPEN-QUESTIONS](OPEN-QUESTIONS.md) | Fragen, die nur der Owner beantworten kann, mit Voreinstellung |
| [RISKS](RISKS.md) | Produktrisiken mit Gegenmaßnahme und Etappe der Prüfung |
| [ADR-Index](adr/README.md) | Architekturentscheidungen im Format MADR 4 |
| [Roadmap](roadmap.md) | öffentliche Übersicht der Etappen |
| [Design](design/README.md) | Design-System: Farben, Typografie, Komponenten, Vorschaubilder |
| [Qualität](quality/README.md) | Gates, Werkzeuge, Push-Disziplin, Öffentlich-Schalten |
| [Betrieb](operations/configuration.md) | Konfigurationsreferenz und Anleitungen für Betreiber |
| [Lehren der Vorgänger](REUSE.md) | was Bothy aus früheren Projekten übernimmt und was nicht |

## 11. Lesewege

| Rolle | Reihenfolge |
|---|---|
| Bau-Agent | eigener Brief unter [briefs/](briefs/README.md), dann die dort genannten Kapitel; immer [00](spec/00-glossar.md), [05](spec/05-aktionen.md) und [23](spec/23-qualitaet.md); bei einer Lücke im Kern-Vertrag anhalten statt abweichen ([PLAN](PLAN.md) "Grundsätze") |
| Reviewer | dieses Dokument (Invarianten), [DECISIONS](DECISIONS.md), die geänderten Kapitel und ADRs, [19](spec/19-sicherheit.md), [Bedrohungsmodell](security/threat-model.md), [Red-Team-Gate](quality/red-team-gate.md) |
| Betreiber | [16](spec/16-betrieb.md), [Installation mit Compose](operations/install-compose.md), [Konfiguration](operations/configuration.md), [08](spec/08-identitaet.md), [17](spec/17-observability.md), [20](spec/20-datenschutz.md), [18](spec/18-release.md); für das Terminal [24](spec/24-terminal.md) |
| Bot-Entwickler | [11](spec/11-erweiterungen.md) "Leitfaden: eigenen Agenten anbinden in zehn Schritten", [09](spec/09-agenten.md), [06](spec/06-api.md), [api/openapi.yaml](api/openapi.yaml), [api/asyncapi.yaml](api/asyncapi.yaml), [Webhooks](api/webhooks.md) |

## 12. Glossar

Alle Begriffe stehen nur in [00 Glossar](spec/00-glossar.md). Dokumente verwenden die dortigen
deutschen Begriffe in Prosa und die Code-Namen in Bezeichnern. Ein neuer Begriff kommt zuerst ins
Glossar.

## 13. Änderungsregel

1. Die Spezifikation ändert sich nur per Pull Request. Jede Änderung einer Regel trägt eine neue
   oder geänderte ADR (tragende Entscheidung) oder einen Eintrag in [DECISIONS](DECISIONS.md)
   (kleinere Festlegung, [ADR 0001](adr/0001-adrs-im-madr-format-decisions-als-ruling-log.md)).
2. Spezifikation zuerst: Ändert Code Verhalten, API, Ereignis, Recht oder Konfiguration, ändert
   dieselbe Änderung zuerst das Kapitel oder die Datei unter [api/](api/README.md).
3. Grenzen, Fristen und Schwellen stehen nur als benannte Konfigurationswerte in der Tabelle
   "Konfigurationswerte dieses Kapitels"; die [Konfigurationsreferenz](operations/configuration.md)
   wird daraus erzeugt (L4).
4. IDs sind stabil: Rulings, Tests, Schutzregeln, Invarianten und ADR-Nummern werden nie neu
   vergeben. Ersetztes bleibt mit Verweis auf den Nachfolger stehen.
5. Ein Widerspruch zwischen zwei Dokumenten wird in der Änderung behoben, die ihn findet.
