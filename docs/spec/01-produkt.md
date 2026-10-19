# 01 Produkt

Status: Entwurf M0. Begriffe: [00-glossar.md](00-glossar.md). Daten: [03-datenmodell.md](03-datenmodell.md).
Sync und Offline: [04-sync-protokoll.md](04-sync-protokoll.md). Rechte und Aktionen:
[05-aktionen.md](05-aktionen.md), [07-rechte.md](07-rechte.md). Agenten: [09-agenten.md](09-agenten.md),
[10-backend-richtlinie.md](10-backend-richtlinie.md). Push: [13-push-benachrichtigungen.md](13-push-benachrichtigungen.md).
Apps technisch: [15-client-architektur.md](15-client-architektur.md). Terminal:
[24-terminal.md](24-terminal.md). Visualisierung: [25-visualisierung.md](25-visualisierung.md).
Gestaltung: [docs/design/](../design/README.md).

Dieses Kapitel beschreibt, was Menschen sehen und bedienen. Es legt keine Rechte fest; jede
Bedienung nennt die Aktion aus dem Aktionskatalog, die der Server prüft. Die App zeigt nur an, was
der Server erlaubt, und enthält keine eigene Rechte-Logik.

## 1. Vision

Bothy ist eine **Kommandozentrale für KI-Agenten in Chat-Form**: Chat, Live-Ansicht, Freigaben,
Terminal, Kosten und Statistik in einer selbst gehosteten, quelloffenen App. Agenten sind Hermes
Agent, Claude Code und weitere, die über die öffentliche API anschließen.

1. **Chat wie Discord.** Aufbau und Bedienung wie Discord; KI-Agenten sind vollwertige Teilnehmer.
2. **Live-Ansicht.** Jeder Thread ist eine Agenten-Sitzung: Man sieht den Agenten arbeiten
   (Live-Box mit Werkzeug- und Statuszeilen), beantwortet seine Rückfragen und gibt Aktionen per
   Knopf frei.
3. **Freigaben.** Jede Person sieht und steuert nur, was ihr freigegeben ist; das prüft der Server
   bei jedem Zugriff ([07-rechte.md](07-rechte.md)).
4. **Terminal.** Eine echte Shell oder eine interaktive Claude-Code-Sitzung auf einem angebundenen
   Rechner, in Desktop-App, Handy und Web-Konsole. Sicherheit ist eingebaut, nicht abschaltbar:
   eigenes Recht je Rechner, Bestätigung bei jedem Öffnen, Ende-zu-Ende-verschlüsselter Strom,
   Audit, Leerlauf-Abbruch ([24-terminal.md](24-terminal.md)).
5. **Kosten.** Jeder Agent zeigt, worauf er läuft (persönliches Abo oder Organisations-Schlüssel),
   und im Organisationsmodus, was ein Lauf, ein Thread, ein Bereich kostet
   ([10-backend-richtlinie.md](10-backend-richtlinie.md)).
6. **Statistik.** Eigene, native Visualisierungen in Apps und Konsole: Live-Topologie der Läufe,
   Backend-Ansicht, Zustellung live, Executive-Seite, abspielbare Timeline, TV-Modus; für den
   Betrieb dieselben Ansichten als Grafana-Panel-Plugin ([25-visualisierung.md](25-visualisierung.md)).
7. **Eigene Instanz.** Eine Instanz gehört einer Organisation und läuft auf eigener Hardware oder in
   der eigenen Cloud. Hermes Agent ist ein Agenten-Framework unter mehreren, nicht der Kern.
8. **Schaufenster-Anspruch.** Was man sieht (Apps, Konsole, Grafana-Plugin, README, Doku-Site,
   Release-Seiten), soll ein Mensch beim ersten Ansehen in einer Minute verstehen. Bilder und Video
   entstehen aus einer Demo-Instanz mit erfundenen Daten (PLAN, Etappe M7).

## 2. Personas

Neutral beschrieben; die Namen sind Beispiele. Rollen sind Vorlagen für Freigaben, keine eigene
Rechte-Art (Baseline Abschnitt 3).

| Persona | Ziel | Typische Aufgaben | Geräte | Freigaben (Beispiel) |
|---|---|---|---|---|
| **Owner** (Alex) | Mit Agenten arbeiten und die Instanz inhaltlich führen | Chats mit Agenten in allen Bereichen, Freigaben vergeben, Agenten und Backend-Richtlinie einstellen, Kosten im Blick, Audit lesen; unterwegs vom iPhone ein Terminal auf dem eigenen Rechner öffnen; Live-Topologie und Executive-Seite | Desktop (Mac oder Windows), iPhone | Owner: alles außer fremden Direktnachrichten; Terminal auf jedem Rechner, außer I6 sperrt ([24-terminal.md](24-terminal.md), Abschnitt 7) |
| **Vertretung** (Sam) | Freigegebene Bereiche im Alltag bedienen | Agenten im Bereich Support auslösen, Rückfragen beantworten, Entwürfe prüfen; Freigaben von Aktionen nur, wenn das Zusatzrecht erteilt ist | iPhone, Android, Laptop | `trigger` auf Bereich Support, Zusatz `upload`; kein `approve_actions`; kein Terminal |
| **Entwicklerin** (Robin) | Mit einem Runner-Agenten an Code arbeiten | Beiträge in Dev-Foren, Werkzeugausgaben lesen, Läufe stoppen, Kosten und Kontext beobachten, Sitzungen fortsetzen; im Terminal auf dem eigenen Rechner `claude` interaktiv nutzen | Desktop mit Tastatur | `trigger` auf einem Dev-Forum mit Projektbindung; Freigabe `terminal` nur auf dem Rechner, dessen Inhaberin sie ist (SR10); Abschottung siehe [21-dev-isolation.md](21-dev-isolation.md) |
| **Betreiberin einer Instanz** | Die Instanz installieren und am Laufen halten | Compose-Installation, Anmelde-Anbieter, Sicherung, Updates, Statusseite, Beobachtung; Rechner koppeln und LLM-Anmeldung erklären; Grafana mit dem Panel-Plugin; TV-Modus im Büro | Web-Konsole, Kommandozeile, Grafana | Owner oder eigene Person mit Owner-Rolle; nicht zu verwechseln mit dem Bediener-Agenten (`operator`) |
| **Bot-Entwickler** | Einen Agenten oder eine App anbinden | Adapter oder App mit SDK bauen, Manifest schreiben, Token beim Owner anfordern, Webhooks testen | eigene Umgebung, Web-Konsole des Owners | App- oder Agenten-Prinzipal mit Scopes; höchstens `write` (R-DM-08) |

## 3. Funktionsliste

Spalte "Etappe": **Kern** = Inhalt einer Bau-Etappe (Reihenfolge legt der Plan fest), **später** =
gewollt, aber nach den Kern-Etappen, **nie** = bewusst ausgeschlossen, mit Grund. Spalte "Wo":
D = Desktop (macOS, Windows), M = Mobil (iOS, Android), W = Web-Konsole. Wo eine Funktion des
Nachtrags vom 2026-10-10 in mehreren Etappen entsteht, nennt die Bemerkung die Bau-Etappen aus dem
[Plan](../PLAN.md).

### 3.1 Struktur und Gespräche

| Funktion | Etappe | Wo | Aktion(en) | Bemerkung |
|---|---|---|---|---|
| Bereiche einklappbar, persönliche Reihenfolge, geräteübergreifend | Kern | D, M | `sidebar.update` | globale Reihenfolge nur Verwaltung (`space.reorder`) |
| Text-Kanäle mit Hauptverlauf und Threads aus Nachrichten | Kern | D, M | `thread.create`, `message.create` | |
| Forum-Kanäle mit dichter Beitragsliste, Tags, Erledigt-Bereich | Kern | D, M | `thread.create`, `thread.tags.set` | Liste statt Karten (Studie B) |
| Dev-Forum mit Projektbindung und Runner-Agent | Kern | D, M | `channel.project.bind` | Auslösung nur bei `project_bindings.ready` |
| Knopf "Neuer Beitrag" bzw. "Neuer Chat" | Kern | D, M | `thread.create` | ersetzt den Umweg über einen Hilfskanal |
| Feste Thread-ID, Auto-Titel mit Quelle, Titel bearbeiten | Kern | D, M | `thread.title.update` | manueller Titel wird nie automatisch überschrieben |
| Erledigt setzen, neue Nachricht öffnet wieder | Kern | D, M | `thread.close`, `thread.reopen` | Abschnitt 6.1 |
| Thread verschieben (nur Kanäle mit gleichem Bediener) | Kern | D | `thread.move` | Dialog zeigt nur gültige Ziele |
| Direktnachrichten zwischen Menschen | später | D, M | `dm.create` | Schalter `dm.enabled`; ohne Agent |
| Threads in Threads | nie | | | eine Ebene genügt; tiefere Bäume zerfasern Agenten-Sitzungen |
| Sprach- und Video-Kanäle, Bildschirm teilen | nie | | | nicht Kern des Produkts; großer Betriebsaufwand (Baseline 4) |
| Mehrere Server, Server-Leiste, öffentliche Einladungslinks | nie | | | eine Instanz ist eine Organisation (B13); Fremde kommen nur über Anlage durch den Owner |
| Rollen-Hierarchie wie Discord | nie | | | Freigaben mit Stufen und Zusatzrechten ersetzen Rollen (ADR 0018) |

### 3.2 Agenten

| Funktion | Etappe | Wo | Aktion(en) | Bemerkung |
|---|---|---|---|---|
| Bediener je Kanal sichtbar im Kanal-Kopf | Kern | D, M | `channel.read` | ohne Bediener steht dort "Kein Agent antwortet hier" |
| Live-Box laufend, angeheftet, mit Stopp | Kern | D, M | `run.read`, `run.cancel` | Abschnitt 6.2 |
| Live-Box eingeklappt nach Ende, aufklappbar | Kern | D, M | `run.read` | |
| Auslöser-Status als Icons an der Nachricht | Kern | D, M | | ersetzt Status-Emoji (Abschnitt 6.3) |
| Nachträge während eines Laufs | Kern | D, M | `run.followup.create` | |
| Knöpfe, Auswahl, Formulare an Nachrichten | Kern | D, M | `interaction.create` | einmalig, nur für Berechtigte |
| Slash-Befehle mit Vorschlägen und Optionen | Kern | D, M | `command.invoke`, `command.autocomplete` | Steuerbefehle (`/stop`) gehen nie an das Modell |
| Eine Agenten-Sitzung je Thread, frühere Sitzungen, neu beginnen, fortsetzen | Kern | D, M | `agent_session.reset`, `agent_session.bind` | |
| Abzeichen Backend-Richtlinie in Thread-Kopf und Agenten-Karte | Kern | D, M, W | `agent.read` | Persönlich oder Organisation |
| Kosten je Lauf und Thread (Organisation: Schätzung bis Abgleich) | Kern | D, M | `cost.read.thread` | bei Persönlich nur Information |
| Übersicht "Läuft gerade" | Kern | D, M | `run.overview.read` | |
| Tippen-Anzeige (Menschen); Agent "tippt" über die Live-Box | Kern | D, M | `typing.start` | |
| Präsenz von Agenten (online, gestört, offline) | Kern | D, M | `agent.read` | Menschen haben keine Präsenz |
| Präsenz von Menschen ("online") | nie | | | verrät Arbeitszeiten; Datenschutz und Mitbestimmung (Studie C, R5) |

### 3.3 Nachrichten

| Funktion | Etappe | Wo | Aktion(en) | Bemerkung |
|---|---|---|---|---|
| Markdown, Code-Blöcke, lange Antworten ohne Teilen, eingeklappt mit "Mehr anzeigen" | Kern | D, M | `message.create` | Grenze `message.max_content_bytes` |
| Bearbeiten und Löschen eigener Nachrichten, Fassungen ansehen | Kern | D, M | `message.update`, `message.delete`, `message.revisions.read` | keine Bearbeitungsfrist |
| Antworten mit Zitat | Kern | D, M | `message.create` | |
| Reaktionen mit freiem Unicode-Zeichen | Kern | D, M | `reaction.add` | Status ist keine Reaktion |
| Erwähnungen von Personen und Agenten | Kern | D, M | `message.create` | Vorschläge nur aus Sichtbarem |
| @everyone, @here, Rollen-Erwähnung | nie | | | Lärm; Benachrichtigung je Kanal reicht |
| Anheften (Moderation) | Kern | D, M | `pin.create` | braucht `manage` |
| Vorlagen (persönlich und je Kanal) | Kern | D, M | `template.read`, `template.manage` | |
| Entwürfe geräteübergreifend | Kern | D, M | `draft.write` | Hinweis bei überschriebener neuerer Fassung |
| Lesebestätigungen anderer Personen | nie | | | Leistungskontrolle; Gelesen-Stand bleibt persönlich (Ruling R-PR-03) |
| Umfragen, Widgets, To-do-Listen im Text | nie | | | Agenten liefern Formulare über Komponenten |
| Sticker, GIF-Suche, Profil-Deko, Banner | nie | | | Ablenkung, Lizenzfragen (Baseline 4) |
| Link-Vorschau vom Server | nie | | | Server lädt keine fremden URLs (SSRF) |
| Link-Vorschau im Client | später | D, M | | nur nach Opt-in je Person, weil die App dann fremde Server kontaktiert |

### 3.4 Dateien und Sprache

| Funktion | Etappe | Wo | Aktion(en) | Bemerkung |
|---|---|---|---|---|
| Dateien und Bilder mit Vorschau, rein und raus | Kern | D, M | `file.upload`, `file.read` | Auslieferung erst nach Scan |
| iPhone: Foto, Kamera, Datei | Kern | M | `file.upload` | |
| Sprachnachricht aufnehmen, abspielen, Transkript | Kern | D, M | `file.upload`, `file.transcribe` | Schalter `voice.enabled`; AAC (15-client-architektur.md) |
| Teilen aus anderen Apps in ein gewähltes Ziel | Kern | M, macOS | `message.create`, `file.upload` | Windows siehe nächste Zeile |
| Teilen aus anderen Apps auf Windows | später | D | | braucht MSIX-Identität; nach der ersten Windows-Auslieferung |
| Export eines Kanals | Kern | D, W | `export.channel` | Zusatzrecht `export`, Step-up |

### 3.5 Benachrichtigung, Suche, Navigation

| Funktion | Etappe | Wo | Aktion(en) | Bemerkung |
|---|---|---|---|---|
| Ungelesen-Zähler, Erwähnungszähler | Kern | D, M | | aus `read_states` und `mentions` |
| Ansicht "Erwähnungen" | Kern | D, M | `mention.list` | |
| Benachrichtigung je Bereich, Kanal, Thread (alle, Erwähnungen, stumm), Ruhezeiten | Kern | D, M | `notification_pref.update`, `quiet_hours.update` | Abschnitt 10 |
| Push ohne Inhalt; Text holt die App | Kern | M, macOS | `push.register` | Windows: Infobereich mit eigener Verbindung |
| Push über Windows-Dienst (WNS) | später | D | | ohne laufende App kein Hinweis; Baseline R-Push |
| E-Mail-Benachrichtigungen | nie | | | Inhalte gehören nicht in Postfächer (Baseline 4) |
| Volltextsuche mit Rechtefilter | Kern | D, M | `search.query` | Treffer nur aus Sichtbarem |
| Schnellwechsel Cmd/Strg+K | Kern | D | | auf Mobil als Suchfeld oben |
| Tastenkürzel | Kern | D | | Abschnitt 7 |
| Mehrere Fenster, Thread als eigenes Fenster | später | D | | Mehrfenster in Flutter noch experimentell (15-client-architektur.md) |
| iPad-Layout mit Spalten | später | M | | bis dahin skaliertes iPhone-Layout |

### 3.6 Offline, Sync, Geräte

| Funktion | Etappe | Wo | Aktion(en) | Bemerkung |
|---|---|---|---|---|
| Offline lesen (zuletzt geöffnete Threads) | Kern | D, M | | Cache nach [04](04-sync-protokoll.md) Abschnitt 13 |
| Senden offline mit Warteschlange, Zustände sichtbar | Kern | D, M | | Abschnitt 6.6 |
| Gelesen-Stand, Entwürfe, Einstellungen, Seitenleiste synchron | Kern | D, M | | |
| Gerät freigeben, sperren; Sitzungen beenden | Kern | D, M, W | `device.approve`, `device.revoke`, `session.revoke` | neue Geräte warten |
| Ende-zu-Ende-Verschlüsselung von Chat-Inhalten | nie | | | Agenten müssen lesen (Baseline 4); der Terminal-Strom ist dagegen Ende-zu-Ende verschlüsselt (3.8) |
| Chat im Browser | nie | | | Web nur für Konsole (ADR 0028) |

### 3.7 Verwaltung, Betrieb, Integration

| Funktion | Etappe | Wo | Aktion(en) | Bemerkung |
|---|---|---|---|---|
| Personen anlegen, sperren; Gruppen | Kern | D, W | `user.create`, `user.disable`, `group.*` | kein Selbst-Registrieren |
| Freigabe-Assistent mit Klartext-Vorschau und I1-Prüfung | Kern | D, W | `grant.create` | Step-up |
| "Wer hat Zugriff?" mit Herkunft | Kern | D, W | `access.explain` | |
| "Als Person ansehen" (nur lesen) | Kern | D, W | `access.view_as` | protokolliert |
| Agenten: Backend-Schalter, Kosten, Tokens, Befehle | Kern | D, W | `agent.billing_mode.set`, `cost.read.aggregate`, `token.create` | |
| Audit filtern, Kette prüfen | Kern | D, W | `audit.read`, `audit.verify` | |
| Statistik | Kern | D, W | `stats.read` | |
| Live-Topologie der Agenten-Läufe, Backend-Ansicht, Zustellung und Sync live, abspielbare Tages-Timeline | Kern | D, M, W | `stats.read`, `cost.read.aggregate` | eigene Messung, keine Werte je Person; [25-visualisierung.md](25-visualisierung.md); mobil Screen M10; Bau: Visualisierungs-Spur nach den frühen M4-Tickets gegen Fixtures, Server und Apps M6 |
| Executive-Seite mit SLO-Ampel "eigene Messung" | Kern | D, M, W | `stats.read`, `cost.read.aggregate` | Ampel nur Owner; Bau M6 |
| TV-Modus (Vollbild, dunkel, rotierende Ansichten) | Kern | D, W | `stats.read` | Kosten standardmäßig verborgen; Bau M6; Screen D13 |
| Grafana-Panel-Plugin "Bothy Live" für den Betrieb | Kern | Grafana | keine (Rechte von Grafana) | Bau M7; unsigniert lokal bis V1, Signatur später |
| Grafana-Dashboards in Apps einbetten | nie | | | nur optional in der Web-Konsole, Standard aus ([ADR 0069](../adr/0069-visualisierung-auf-zwei-ebenen-ohne-iframes-in-apps.md)) |
| Statusseite | Kern | D, M, W | `status.read` | öffentlich nur mit `status.public_enabled` |
| Import aus Discord | Kern | W | `import.manage` | [22-import-migration.md](22-import-migration.md) |
| Apps installieren, Webhooks | Kern | W | `app.install`, `webhook_out.manage`, `webhook_in.create` | |
| Notfall-Aktionen auf dem Telefon: Gerät sperren, Freigabe widerrufen, Person sperren, Lauf stoppen | Kern | M | `device.revoke`, `grant.revoke`, `user.disable`, `run.cancel` | große Abläufe (Freigabe anlegen, Audit, Statistik) nur Desktop und Web |
| Marktplatz für Bots | nie | | | Apps installiert der Owner bewusst; kein Katalog fremder Anbieter |

### 3.8 Terminal

Normativ: [24-terminal.md](24-terminal.md). Bau: Server und Host-Dienst M2, Desktop M4, Mobil M5,
Web-Konsole M6.

| Funktion | Etappe | Wo | Aktion(en) | Bemerkung |
|---|---|---|---|---|
| Terminal öffnen: Login-Shell auf einem angebundenen Rechner (macOS, Linux, Windows) | Kern | D, M, W | `terminal.open` | frischer Step-up je Öffnung, in der Konsole nur Passkey ([ADR 0070](../adr/0070-terminal-in-der-web-konsole.md)); Screens D11, M8 |
| Claude-Code-Sitzung im Terminal (Profil `claude`) | Kern | D, M, W | `terminal.open` | interaktives `claude` im Arbeitsordner aus einer Erlaubnisliste, getrennt vom Runner; nur auf Rechnern mit erklärter LLM-Anmeldung (I6) |
| Sitzungsliste je Rechner, Trennen und Wiederanhängen, auch von einem anderen Gerät | Kern | D, M, W | `terminal.read_own`, `terminal.attach` | die Sitzung läuft `terminal.detach_grace` weiter; anderes Gerät mit neuem Step-up |
| Sitzung beenden | Kern | D, M, W | `terminal.close` | Owner beenden jede Sitzung |
| Sondertasten-Leiste (Esc, Tab, Ctrl, Alt, Pfeile, `\|`, `~`, `/`, `-`) und Hardware-Tastatur | Kern | M | | Bau M5; Screen M8 |
| Fingerabdruck des Rechners und Schlüssel-Pinning je Gerät mit harter Warnung bei Wechsel | Kern | D, M, W | | [24-terminal.md](24-terminal.md), Abschnitt 5.5 |
| Rechner anlegen, koppeln, Fingerabdruck bestätigen, LLM-Anmeldung erklären, sperren, zurückziehen | Kern | D, W | `host.read`, `host.manage` | nur Owner, Step-up, Audit; Screen A8 |
| Freigabe `terminal` je Rechner | Kern | D, W | `grant.create` | nur Owner; für Nicht-Owner wirkt sie nur auf dem Rechner, dessen Inhaber sie sind (SR10); die Vorschau sagt es |
| Thread eines Runners im Terminal übernehmen | später | D | | erst wenn Runner und Host-Dienst in einer Dev-Umgebung als derselbe Benutzer laufen ([24-terminal.md](24-terminal.md), Abschnitt 11.3) |
| Dateiübertragung und Zwischenablage über den Terminal-Kanal | später | D, M, W | | in dieser Fassung gesperrt (OSC 52, [24-terminal.md](24-terminal.md), Abschnitt 11.2) |
| Aufzeichnung von Terminal-Inhalten | nie | | | nur Metadaten; ob Nachweispflichten eine Aufzeichnung verlangen, ist eine Owner-Frage (Voreinstellung: nein) |
| Terminal für Agenten oder Apps | nie | | | ein Agent, den Prompt-Injection steuert, darf keinen Rechner erreichen (T-TM-09) |

## 4. Informationsarchitektur

```text
Instanz (Branding, Statusseite)
+-- Bereiche (Seitenleiste, einklappbar, Kategorie-Farbe, Icon, Standard-Bediener)
|   +-- Text-Kanal  -> Hauptverlauf  -> Threads aus Nachrichten
|   +-- Forum       -> Beitragsliste -> je Beitrag ein Thread (Tags, Erledigt-Bereich)
+-- Direktnachrichten (später)
+-- Querschnitt: Läuft gerade, Erwähnungen, Suche, Schnellwechsel, Einstellungen
+-- Rechner (Terminal): je Rechner Profile und eigene Sitzungen
+-- Live und Statistik: Live-Topologie, Executive-Seite, Timeline, TV-Modus
+-- Verwaltung (Desktop und Web-Konsole; Notfall-Aktionen mobil)
```

Ein Thread ist überall dasselbe Objekt mit fester ID: im Forum ein Beitrag, im Text-Kanal ein
Thread aus einer Nachricht. Seine Agenten-Sitzung, Läufe und Kosten hängen an ihm.

## 5. Screens

Zeichen in den Wireframes: `v`/`>` ausgeklappt/eingeklappt, `#` Text-Kanal, `=` Forum, `*`
ungelesen oder Lauf aktiv, `(3)` Zähler, `[ ]` Knopf oder Feld, `[A]` Agent (abgerundetes Quadrat),
`(AL)` Mensch (rund), `(o)` läuft, `[!]` wartet auf dich, `(@)` Erwähnung, `(z)` Warteschlange,
`(v)` erledigt, `(x)` fehlgeschlagen, `(-)` abgebrochen. Auslöser-Status an Nachrichten in
geschweiften Klammern: `{ok}` erledigt, `{..}` in Arbeit, `{z}` Warteschlange, `{x}` fehlgeschlagen,
`{->}` Nachtrag zugestellt. Gerenderte Gestaltung: [Design-System](../design/README.md) mit den
Bildern aus `docs/design/previews/`.

### 5.1 Desktop (macOS, Windows)

**D1 Hauptansicht Forum.** Vier Spalten: Bereiche und Kanäle (Markenfläche), Beitragsliste (nur bei
Foren), Verlauf mit angehefteter Live-Box, rechte Sitzungs-Spalte (einklappbar, statt
Mitgliederliste). Keine Server-Leiste links (B13). Spaltengrenzen ziehbar; Breiten werden als
Einstellung `layout.column_widths` gespeichert. Unter `ui.desktop_min_width_four_columns` klappt
die Sitzungs-Spalte ein, unter `ui.desktop_min_width_three_columns` die Beitragsliste
(Navigationsstapel wie auf dem Telefon).

```text
+----------------------------+--------------------------------+---------------------------------------+------------------------+
| [^] Bothy                  | = server  [+ Neuer Beitrag]    | Gateway: resume nach Netzwechsel      | Sitzung            [x] |
| [Springen zu ...  Cmd+K]   | [Offen][Wartet auf mich]       | [Organisation] 1,84 USD Schätzung     |                        |
| (o) Läuft gerade      (2)  | [Tags v][Letzte Aktivität v]   |      [Erledigt] [Anheften] [Spalte]   | [A] dev-runner         |
|                            |                                | +- (o) Läuft  4:12  62 %  0,62 USD -+ |     online             |
| v [H] Support              | (o) Gateway: resume na.. 4min  | | Prüfe, ob resume den Fokus ...    | | Backend [Organisation] |
|     # allgemein            |     dev-runner  [sync]         | | [ok Read] internal/sync/gw.go     | | Modell     model-large |
|   | = anfragen     * (3)   | [!] Migration 0012 prüf. 12:41 | | [.. Bash] go test ./internal/...  | | Sitzung   0193f2a7-..  |
|     # meldungen  (stumm)   |     [Wartet auf dich] [db]     | | > Der Fokus wird beim Fortsetz..  | | Projekt   server, main |
| v [<>] Engineering         | (@) Review: Freigabe-As. 11:58 | | [Stopp]  Nachrichten = Nachtrag   | |                        |
|   >= server          *     |     Sam erwähnt dich [rechte]  | +-----------------------------------+ | Kontext  [######----]  |
|     = apps                 | (z) Suche: Treffer im T. 11:20 | [A] dev-runner AGENT 10:14            | Kosten Thread 1,84 USD |
|     # releases             | (v) Kosten je Bereich ..  2h   |   Ursache: ... [Code-Block]           |         (Schätzung)    |
| > [M] Marketing        *   | (x) Upload-Grenze ...  gestern |   [v Lief 12 min, 3 Werkzeuge  >]     | Läufe               5  |
| > [R] Research             |     fehlgeschlagen: Projekt .. | [A] dev-runner AGENT 10:16            |                        |
|                            | (-) Alter Entwurf: Export  Mo  |   Freigabe nötig: go test ...         | Frühere Sitzungen (2)  |
|                            |                                |   [Einmal erlauben][Sitzung][Immer]   | Angeheftet (2)         |
|                            |                                | |(SA) Sam: @Alex kannst du ...   (!)  | Dateien (3)            |
|                            |                                | (AL) Alex 10:22 {..} @dev-runner ...  |                        |
|                            |                                | ... Sam schreibt gerade               |                        |
| (AL) Alex         [Einst.] | > Erledigt (14)                | [+] Nachtrag an dev-runner  [Mic][>]  |                        |
+----------------------------+--------------------------------+---------------------------------------+------------------------+
```

**D2 Text-Kanal mit geöffnetem Thread.** Ohne Beitragsliste. Ein Thread öffnet rechts neben dem
Hauptverlauf; die Sitzungs-Spalte ist dann eingeklappt. Der Kopf nennt den Bediener.

```text
+--------------------------+------------------------------------------------+------------------------------------------+
| v [H] Support            | # allgemein   Bediener: support-agent  [Thema] | #allgemein > Zusammenfassung Anfrage     |
|   ># allgemein           |                                                | [Organisation]  [Erledigt]  [...]        |
|     = anfragen    * (3)  | (SA) Sam 09:12                                 |                                          |
|     # meldungen          |   Kunde fragt nach dem Exportformat.           | [v] Lief 2 min, 1 Werkzeug, 0,04 USD >   |
|                          |   [Thread: Exportformat, 4 Antworten >]        | [A] support-agent AGENT 09:31            |
|                          | (AL) Alex 09:30 {ok}                           |   Die Anfrage betrifft ... (gekürzt)     |
|                          |   @support-agent fasse die Anfrage zusammen    |   [Mehr anzeigen]                        |
|                          |   [Thread: Zusammenfassung Anfrage 4711 >]     | (AL) Alex 09:33 {->}                     |
|                          | (RO) Robin 09:41                               |   Bitte auch die Frist nennen.           |
|                          |   Danke, übernehme ich.                        |                                          |
|                          |                                                |                                          |
|                          | [+] Nachricht an #allgemein          [Mic][>]  | [+] Nachtrag an support-agent  [Mic][>]  |
+--------------------------+------------------------------------------------+------------------------------------------+
```

**D3 Thread-Ansicht: Live-Box in vier Zuständen.** Laufend (angeheftet), eingeklappt nach dem
Ende, wartet auf Antwort, fehlgeschlagen. Spezifikation: [components.md](../design/components.md)
Abschnitt 2.

```text
+--------------------------------------------------------------------------------+
| (o) Läuft  4:12   Kontext 62 %   48,1k Token   0,62 USD   [Organisation]   [v] |
| Prüfe, ob resume den Fokus nach dem Netzwechsel wiederherstellt.               |
|   [ok Read ] internal/sync/gateway.go                              0,4 s       |
|   [ok Grep ] resume_window                                          0,2 s      |
|   [.. Unteragent] Testfälle für 4006 schreiben                                 |
|       [.. Bash] go test ./internal/sync/...                     läuft 38 s     |
|   2 frühere Schritte [anzeigen]                                                |
| > Der Fokus wird beim Fortsetzen aus der Gateway-Sitzung übernommen; offen     |
| > ist, ob abgelaufene Threads still entfallen.                                 |
| [Stopp]   Neue Nachrichten in diesem Thread gehen als Nachtrag an den Lauf.    |
+--------------------------------------------------------------------------------+
```

```text
+--------------------------------------------------------------------------------+
| (v) Lief 28 min   3 Werkzeuge   2 Unteragenten   1,20 USD Schätzung       [>]  |
+--------------------------------------------------------------------------------+
```

```text
+--------------------------------------------------------------------------------+
| [!] Wartet auf dich  2:03  0,18 USD                              [Persönlich]  |
| Welche Datenbank soll der Test nutzen?                                         |
|   Datenbank [Lokaler Container  v]   Port [55432   ]   [Antworten]             |
+--------------------------------------------------------------------------------+
```

```text
+--------------------------------------------------------------------------------+
| (x) Fehlgeschlagen   Backend passt nicht: Ist organization, Soll personal      |
| [Erneut versuchen]  [Details]                                                  |
+--------------------------------------------------------------------------------+
```

**D4 Komponenten an Nachrichten.** Freigabe-Knöpfe (einmal, Sitzung, immer, ablehnen), Auswahl,
Formular; nach Benutzung eine Zeile mit Person und Wahl.

```text
+--------------------------------------------------------------------------------+
| [A] dev-runner AGENT 10:16                                                     |
|   +- [Schild] Freigabe nötig -------------------------------------------+      |
|   | go test möchte das Netzwerk für die Testdatenbank nutzen.            |     |
|   | [Einmal erlauben] [Für diese Sitzung] [Immer] [Ablehnen]             |     |
|   | [Schloss] Nur Alex oder ein Owner kann das freigeben.                |     |
|   +---------------------------------------------------------------------+      |
|   +- Modell wählen -----------------+   +- Rückfrage --------------------+     |
|   | [model-large            v]      |   | Kunde       [Text          ]   |     |
|   |  > model-large                  |   | Priorität   [normal v]         |     |
|   |    model-fast                   |   | [Absenden]                     |     |
|   +---------------------------------+   +--------------------------------+     |
|   Nach Benutzung:  {ok} Alex hat 'Einmal erlauben' gewählt, 10:24              |
+--------------------------------------------------------------------------------+
```

**D5 Slash-Autovervollständigung.** Öffnet bei "/" am Zeilenanfang. Befehle des Bedieners zuerst,
gleichnamige Befehle anderer Agenten mit Handle. Optionen mit Vorschlägen vom Agenten.

```text
+--------------------------------------------------------------------------------+
| +- Befehle von dev-runner (Bediener dieses Kanals) -----------------------+    |
| | > /status   Zustand des laufenden Laufs                  Steuerbefehl   |    |
| |   /stop     Lauf abbrechen, geht nie an das Modell       Steuerbefehl   |    |
| |   /model    Modell für diese Sitzung wählen              name           |    |
| |   /stats    Kennzahlen des Kanals                        research-bot   |    |
| +-------------------------------------------------------------------------+    |
| [+] /st|                                                          [Mic][>]     |
|                                                                                |
| Nach Auswahl:  /model [name: model-l|]   Vorschläge vom Agenten:               |
|                > model-large   model-fast                                      |
+--------------------------------------------------------------------------------+
```

**D6 Schnellwechsel (Cmd+K bzw. Strg+K).** Springt zu Kanal, Thread, Person, Agent oder Befehl.
Treffer nur aus Sichtbarem; zuletzt besuchte Ziele zuerst. Höchstens
`ui.quick_switch_max_results` Treffer.

```text
+------------------------------------------------------------------------+
| [Lupe] serv|                                                    [esc]  |
+------------------------------------------------------------------------+
| > = server                                            Engineering      |
|   [Thread] Gateway: resume nach Netzwechsel                server      |
|   [A] dev-runner                                            Agent      |
|   (SA) Sam                                                 Person      |
+------------------------------------------------------------------------+
| Tab: Filter  Kanäle | Threads | Personen | Agenten | Befehle           |
+------------------------------------------------------------------------+
```

**D7 Suche.** Volltext über Nachrichten, Thread-Titel, Dateinamen und Transkripte; Filter links;
Treffer mit Hervorhebung und Pfad. Die Trefferzahl zählt nur Sichtbares.

```text
+--------------------------+------------------------------------------------------------------------+
| Filter                   | [Lupe] resume_window                                    [Suchen]       |
+--------------------------+------------------------------------------------------------------------+
| Bereich  [alle v]        | 14 Treffer, nur aus Kanälen, die du sehen darfst                       |
| Kanal    [alle v]        |                                                                        |
| Von      [alle v]        | = server > Gateway: resume nach Netzwechsel                            |
| Art  [x] Nachrichten     |   (A) dev-runner 10:14  ... Fenster **resume_window** wird ab          |
|      [x] Threads         |   dem letzten Heartbeat gemessen ...                                   |
|      [x] Dateien         | # releases > Version 0.4                                               |
|      [ ] Transkripte     |   (RO) Robin gestern  ... neues **resume_window** ...                  |
| Zeit [beliebig v]        | [Datei] notizen-sync.md  ... **resume_window** ...                     |
| Status [offen+erledigt]  | [mehr laden]                                                           |
+--------------------------+------------------------------------------------------------------------+
```

**D8 Einstellungen.** Darstellung, Benachrichtigungen, Sprache, Tastatur, Geräte und Sitzungen,
Datenschutz, Über (Version, Lizenzen).

```text
+--------------------------+------------------------------------------------------------------------+
| Einstellungen            | Darstellung                                                            |
+--------------------------+------------------------------------------------------------------------+
| > Darstellung            | Theme        ( ) System  ( ) Hell  (x) Dunkel                          |
|   Benachrichtigungen     | Dichte       (x) Normal  ( ) Kompakt                                   |
|   Sprache                | Bewegung     (x) System  ( ) Voll  ( ) Reduziert                       |
|   Tastatur               | Enter sendet [x]   (Shift+Enter: neue Zeile)                           |
|   Geräte und Sitzungen   | Spaltenbreiten [Zurücksetzen]                                          |
|   Datenschutz            |                                                                        |
|   Über                   | Sprache      [Deutsch v]  Zeitzone [Europe/Berlin v]                   |
|                          | Geräte: iPhone (aktiv), Laptop (dieses), [Gerät sperren]               |
|                          | Datenschutz: [x] Absturzberichte  [x] Leistungsdaten  [Export]         |
+--------------------------+------------------------------------------------------------------------+
```

**D9 Benachrichtigungs-Einstellungen.** Standard, Ruhezeiten, Inhalt in Mitteilungen, Stufe je
Bereich, Kanal und Thread mit sichtbarem Erbe; befristetes Stummschalten.

```text
+------------------------------------------+--------------------------------------------------------+
| Benachrichtigungen                       |                                                        |
+------------------------------------------+--------------------------------------------------------+
| Standard für neue Kanäle                 | ( ) Alle  (x) Erwähnungen  ( ) Stumm                   |
| Läufe, die ich ausgelöst habe            | [x] Melden, wenn fertig oder Rückfrage                 |
| Inhalt in Mitteilungen zeigen            | [x] Text laden (sonst nur 'Neue Nachricht')            |
| Ruhezeiten                               | Mo-Fr 19:00-07:00, Sa-So ganztags  [Bearbeiten]        |
+------------------------------------------+--------------------------------------------------------+
| Je Bereich, Kanal und Thread             | Stufe                                                  |
| v Support                                | [Alle v]                                               |
|     # allgemein                          | [erbt: Alle]                                           |
|     = anfragen                           | [Erwähnungen v]                                        |
| v Engineering                            | [Erwähnungen v]                                        |
|     = server > Gateway: resume ...       | [Alle v]  bis [heute 18:00 v]                          |
| > Marketing                              | [Stumm v]                                              |
+------------------------------------------+--------------------------------------------------------+
```

**D10 Läuft gerade.** Alle aktiven Läufe, die die Person sehen darf; Stopp nur, wo `run.cancel`
erlaubt ist (Auslöser, `manage`, Owner).

```text
+----------------------------------------------------------------------------------------------------+
| Läuft gerade (3)                                       [Filter: Bereich v] [Agent v] [nur meine]   |
+----------------------------------------------------------------------------------------------------+
| (o) dev-runner     server > Gateway: resume nach Netzwechsel   4:12  62 %  0,62 USD  [Stopp]       |
| [!] dev-runner     server > Migration 0012 prüfen        wartet auf dich  2:03  [Öffnen]           |
| (z) research-bot   Research > Wochenbericht                Warteschlange          [Öffnen]         |
+----------------------------------------------------------------------------------------------------+
| Organisation: Summe laufender Läufe 0,80 USD (Schätzung). Persönliche Läufe: nur Information.      |
+----------------------------------------------------------------------------------------------------+
```

**D11 Terminal.** Seitenleiste mit der Gruppe "Rechner" unter den Bereichen; rechts die
Terminal-Ansicht mit einem Reiter je eigener Sitzung des Rechners. Der Kopf zeigt Rechner,
Verbindung, Abzeichen der LLM-Anmeldung, den Anfang des Fingerabdrucks und die Restzeit bis
`terminal.max_session_duration`. "Trennen" lässt die Sitzung laufen (wiederanhängbar), "Beenden"
schließt sie. Öffnen verlangt die Bestätigung mit Passkey bzw. Step-up-Geräteschlüssel; der Dialog
nennt Rechner und Profil. Alle Tasten gehen an die Shell, außer den Kürzeln aus Abschnitt 7.

```text
+----------------------------+-------------------------------------------------------------------------------+
| [^] Bothy                  | Build-Rechner  [online]  [Organisation]  Fp 7KQ2 9XTA ..  Rest 11:42  [i]     |
| [Springen zu ...  Cmd+K]   | [shell 1 x] [claude: server x] [+ Neue Sitzung v]       [Trennen] [Beenden]   |
| (o) Läuft gerade      (2)  +-------------------------------------------------------------------------------+
|                            | ~/src/server $ go test ./internal/sync/...                                    |
| > [H] Support              | ok      internal/sync           4.212s                                        |
| v [<>] Engineering         | ok      internal/sync/gateway   1.874s                                        |
|     = server         *     | ~/src/server $ git status --short                                             |
|     # releases             |  M internal/sync/gateway.go                                                   |
|                            | ~/src/server $ _                                                              |
| v Rechner                  |                                                                               |
|   (o) Build-Rechner  (2)   |                                                                               |
|   ( ) Linux-VM             |                                                                               |
|   (-) Test-Windows         |                                                                               |
|       gesperrt             |                                                                               |
|                            +-------------------------------------------------------------------------------+
| (AL) Alex         [Einst.] | Ende-zu-Ende verschlüsselt | Profil shell | Leerlauf-Abbruch aktiv            |
+----------------------------+-------------------------------------------------------------------------------+
```

Bestätigung beim Öffnen und Warnung bei geändertem Schlüssel (24, Abschnitt 5.5):

```text
+------------------------------------------------------------+  +------------------------------------------------------------+
| Terminal öffnen                                            |  | [!] Schlüssel des Rechners hat sich geändert               |
+------------------------------------------------------------+  +------------------------------------------------------------+
| Rechner   Build-Rechner                                    |  | Gespeichert:  7KQ2 9XTA M4PB 1C8D ...                      |
| Profil    (x) shell   ( ) claude [Arbeitsordner: server v] |  | Jetzt:        Q0ZJ 55RW 2HNE K7TS ...                      |
| Anmeldung [Organisation]  Kosten trägt die Organisation    |  | Ohne Neukopplung durch einen Owner ist das ein Hinweis auf |
|                                                            |  | einen Angriff. Erst verbinden, wenn du den neuen           |
| Jede Öffnung wird protokolliert.                           |  | Fingerabdruck am Rechner geprüft hast.                     |
| [Abbrechen]              [Bestätigen mit Passkey / Gerät]  |  | [Nicht verbinden]           [Neuen Schlüssel annehmen]     |
+------------------------------------------------------------+  +------------------------------------------------------------+
```

**D12 Live-Topologie.** Bahnen je Bereich, Lauf-Karten mit Zustand, Werkzeug, Abzeichen und Kosten;
Klick öffnet den Thread. Gleiche Daten als Tabelle über "Tabelle". Normativ:
[25-visualisierung.md](25-visualisierung.md), Abschnitt 6.1.

```text
+----------------------------+-------------------------------------------------------------------------------+
| [^] Bothy                  | Live  [Topologie] [Backend] [Zustellung] [Executive] [Timeline]               |
| (o) Läuft gerade      (3)  | Bereich [alle v]                    12:04:31  live   [Tabelle] [TV-Modus]     |
| v Live und Statistik       +-------------------------------------------------------------------------------+
|   > Live                   | Engineering | = server   +- dev-runner ------------------+                    |
|     Statistik              |             |            | (o) running  4:12             |                    |
|                            |             |            | [.. Bash] go test             |                    |
| > [H] Support              |             |            | [Org.] 0,62 USD Schätzung     |                    |
| v [<>] Engineering         |             |            +-------------------------------+                    |
|     = server         *     |             | # releases  (keine aktiven Läufe)                               |
| v Rechner                  | Support     | = anfragen +- support-agent ---------------+                    |
|   (o) Build-Rechner  (2)   |             |            | [!] waiting_input             |                    |
|                            |             |            | Freigabe wartet  [Persönlich] |                    |
|                            |             |            +-------------------------------+                    |
|                            +-------------------------------------------------------------------------------+
|                            | Letzte Minute: Nachrichten 14 | Läufe 3 | Werkzeugaufrufe 41 | Fehler 0       |
+----------------------------+-------------------------------------------------------------------------------+
```

**D13 TV-Modus.** Vollbild, dunkles Theme, größere Schrift, keine Bedienelemente; rotiert alle
`viz.tv_rotation_interval` durch die gewählten Ansichten. Kosten nur mit `viz.tv_show_costs`.
Tastatur oder Klick beendet ihn. Desktop und Web-Konsole, nicht mobil.

```text
+--------------------------------------------------------------------------------------------------------+
|  BOTHY LIVE                        Live-Topologie  (1 von 3)                         12:04   [=  =  =] |
+--------------------------------------------------------------------------------------------------------+
|                                                                                                        |
|   ENGINEERING        server     [ (o) dev-runner    running   Bash      Organisation ]                 |
|                      releases   [ (v) research-bot  completed 2 min                  ]                 |
|                                                                                                        |
|   SUPPORT            anfragen   [ [!] support-agent waiting_input   Freigabe wartet  ]                 |
|                                                                                                        |
|   RESEARCH           (keine aktiven Läufe)                                                             |
|                                                                                                        |
+--------------------------------------------------------------------------------------------------------+
|   Läufe aktiv 3      Werkzeugaufrufe/min 41      Zustellung im Ziel [ok]      eigene Messung           |
+--------------------------------------------------------------------------------------------------------+
```

### 5.2 Verwaltung (Desktop und Web-Konsole)

Jede ändernde Verwaltungsaktion mit Step-up zeigt vor dem Absenden die Bestätigung mit Passkey
bzw. Geräteschlüssel ([08-identitaet.md](08-identitaet.md)).

**A1 Personen.** Liste, Anlegen (kein Selbst-Registrieren, keine Einladungslinks), Warteschlange
neuer Geräte mit Herkunft der Anmeldung.

```text
+------------------------+----------------------------------------------------------------------------+
| Verwaltung             | Personen                                  [+ Person anlegen]               |
+------------------------+----------------------------------------------------------------------------+
| > Personen             | [Lupe] Suchen        Filter: [aktiv v] [Gruppe v]                          |
|   Gruppen              | (AL) Alex     Owner          2 Geräte   zuletzt heute                      |
|   Freigaben            | (SA) Sam      Support (trigger) 1 Gerät   zuletzt heute                    |
|   Geräte (1 wartet)    | (RO) Robin    Engineering (trigger) 3 Geräte gestern                       |
|   Agenten              | (NE) Neu      wartet auf Anmeldung                                         |
|   Apps                 | ---                                                                        |
|   Audit                | Wartende Geräte: Sam, 'Pixel', angemeldet 08:12 per OIDC                   |
|   Statistik            |   [Freigeben] [Ablehnen]          Schlüssel: Hardware, attestiert          |
|   Instanz              |                                                                            |
+------------------------+----------------------------------------------------------------------------+
```

**A2 Freigabe anlegen mit Klartext-Vorschau.** Drei Schritte (Wer, Wo, Stufe und Zusatzrechte).
Die Vorschau formuliert die Wirkung in Sätzen, nennt neu sichtbare Kanäle und prüft I1 vor dem
Absenden; eine Verletzung wird erklärt, nicht nur abgelehnt ("zuerst auf Organisation umstellen").

```text
+----------------------------------------------------------------------------------------------+
| Freigabe anlegen                                                         Schritt 3 von 3     |
+----------------------------------------------------------------------------------------------+
| Wer      (SA) Sam                         [Person v]                                         |
| Wo       Bereich Support  (gilt für alle Kanäle darin)                                       |
| Stufe    ( ) lesen  ( ) schreiben  (x) Agenten auslösen                                      |
| Zusatz   [ ] Aktionen freigeben  [x] Dateien hochladen  [ ] Exportieren  [ ] Verwalten       |
| Bis      [x] 31.12.2026          Grund [Vertretung Support Q4                   ]            |
+----------------------------------------------------------------------------------------------+
| Vorschau in Klartext:                                                                        |
|   Sam darf ab sofort alle Kanäle im Bereich Support lesen, dort schreiben und den            |
|   Agenten support-agent auslösen. Sam darf Dateien hochladen. Sam darf keine                 |
|   Aktionen freigeben (Freigabe-Knöpfe bleiben gesperrt). Gilt bis 31.12.2026.                |
|   Neu sichtbar für Sam: 3 Kanäle, 214 Threads.                                               |
| [!] Prüfung: support-agent läuft im Modus Organisation. Kosten trägt die Organisation.       |
| [x] Abgelehnt wäre: dev-runner (Persönlich, nur Alex darf auslösen). Zuerst umstellen.       |
+----------------------------------------------------------------------------------------------+
| [Zurück]                                         [Bestätigen mit Passkey / Gerät]            |
+----------------------------------------------------------------------------------------------+
```

**A3 Wer hat Zugriff?** Je Bereich oder Kanal: Personen, Gruppen, Agenten mit wirksamer Stufe und
Herkunft (direkt, Gruppe, geerbt, Owner).

```text
+--------------------------------+----------------------------------------------------------------+
| Wer hat Zugriff?               | Bereich Support > Kanal anfragen                               |
+--------------------------------+----------------------------------------------------------------+
| Person oder Gruppe             | Stufe, Zusatz und Herkunft                                     |
+--------------------------------+----------------------------------------------------------------+
| (AL) Alex                      | auslösen, alle Zusatzrechte    Owner                           |
| (SA) Sam                       | auslösen, Dateien    direkt auf Bereich Support, bis 31.12.    |
| [G] Support-Team (4)           | schreiben             Gruppe, geerbt von Bereich Support       |
| (RO) Robin                     | lesen                 über Gruppe Support-Team und direkt      |
| [A] support-agent              | schreiben (Bediener)  direkt auf Bereich Support               |
+--------------------------------+----------------------------------------------------------------+
| [Als Person ansehen v]         | [Freigabe anlegen]  [Widerrufen]  [Verlauf]                    |
+--------------------------------+----------------------------------------------------------------+
```

**A4 Als Person ansehen.** Nur lesen; deutlicher Rahmen; jede Nutzung im Audit-Log.

```text
+----------------------------------------------------------------------------------------------+
| [Auge] Ansicht als Sam (nur lesen). Nichts wird gesendet. Protokolliert.       [Beenden]     |
+----------------------------------------------------------------------------------------------+
| Seitenleiste wie Sam sie sieht:   v Support (# allgemein, = anfragen, # meldungen)           |
|                                   > Research (# lesekreis)                                   |
| Nicht sichtbar für Sam: Engineering, Marketing                                               |
| Klick auf einen Kanal zeigt Inhalte wie für Sam; Eingabe, Knöpfe und Reaktionen sind aus.    |
+----------------------------------------------------------------------------------------------+
```

**A5 Agenten mit Backend-Schalter und Kosten.** Ist-Modus aus dem Heartbeat neben dem Soll;
Umschalten mit Grund und Step-up; Erklärung, warum Persönlich gerade nicht geht (I1).

```text
+--------------------------------+----------------------------------------------------------------+
| Agenten                        | support-agent                                 [online]         |
+--------------------------------+----------------------------------------------------------------+
| > support-agent  [Org.]        | Adapter      hermes        Ist-Modus: Organisation             |
|   dev-runner     [Pers.]       | Bedient      Bereich Support (Standard), # meldungen           |
|   research-bot   [Org.]        | Backend      (x) Organisation  ( ) Persönlich                  |
|                                |              Umschalten braucht Grund und Bestätigung.         |
| [Alle umstellen ...]           |              Persönlich geht nur, wenn genau eine Person       |
|                                |              auslösen darf (I1). Heute: 5 Personen.            |
|                                | Kosten 30 Tage  41,20 USD (Schätzung)  [nach Bereich v]        |
|                                |   Support 38,10   Marketing 3,10      [Abgleichen]             |
|                                | Tokens [Token erzeugen]   Befehle (6)   Präsenz-Verlauf        |
+--------------------------------+----------------------------------------------------------------+
```

**A6 Audit.** Filterbar nach Aktion, Person, Ergebnis, Ressource, Zeitraum; Prüfung der Hash-Kette.
Export eines Zeitraums als NDJSON des Servers mit Hashes (offline prüfbar) und als CSV, die das
Gerät aus demselben Strom umwandelt.

```text
+----------------------------------------------------------------------------------------------------+
| Audit   Filter: [Aktion v] [Person v] [Ergebnis: alle v] [Zeitraum v]      [Kette prüfen]          |
+----------------------------------------------------------------------------------------------------+
| Zeit    Wer       Aktion                    Ergebnis  Ressource         Grund                      |
| 10:24   Alex      interaction.create        erlaubt   Karte in server   approve_actions            |
| 10:20   Sam       interaction.create        abgelehnt Karte in server   fehlendes Zusatzrecht      |
| 09:58   Alex      grant.create              erlaubt   Bereich Support   Vertretung Q4              |
| 09:57   Alex      auth.step_up              erlaubt   Sitzung Laptop                               |
| 08:12   Sam       agent.invoke              abgelehnt Lauf 0193..       Persönlich: nicht Inhaber  |
+----------------------------------------------------------------------------------------------------+
| Kette geprüft bis Nr. 18 402: unverändert.  [Exportieren] [Als CSV exportieren]  (Owner, Step-up)  |
+----------------------------------------------------------------------------------------------------+
```

**A7 Statistik.** Kennzahl-Kacheln, Läufe je Tag, Kosten je Agent und Bereich, Fehlgründe. Werte
aus dem Server; die Konsole zeigt dieselbe Ansicht in voller Breite.

```text
+----------------------------------------------------------------------------------------------------+
| Statistik   Zeitraum [30 Tage v]   Bereich [alle v]                       [Export]                 |
+----------------------------------------------------------------------------------------------------+
| + Läufe ----------+ + Kosten (Org.) ---+ + Mittlere Dauer -+ + Wartet auf Menschen -+              |
| | 1 284           | | 41,20 USD        | | 3 min 10 s       | | 12 offen             |             |
| | +8 % zur Vorper.| | Schätzung        | | p95 18 min       | | älteste 2 Tage       |             |
| +-----------------+ +------------------+ +------------------+ +----------------------+             |
| Läufe je Tag (gestapelt nach Ergebnis)       Kosten je Agent                                       |
|   ::::::::::::::::::::::::::::::::::::      support-agent  ##########  38,10                       |
|   erledigt / fehlgeschlagen / abgebrochen   research-bot   #           3,10                        |
| Fehlgründe: Projekt nicht bereit 9, Backend passt nicht 2, Zeitlimit 1                             |
+----------------------------------------------------------------------------------------------------+
```

**A8 Rechner.** Nur Owner. Rechner anlegen mit Name, optionalem Inhaber und Grund; einmaliger
Kopplungs-Code mit Befehl für den Rechner; Bestätigung durch Eintippen der ersten
`terminal.fingerprint_confirm_chars` Zeichen des Fingerabdrucks, den `bothy-host pair` dort zeigt;
Pflichtfragen zur LLM-Anmeldung (I6) und zum Server-Host; Prüfergebnisse von `bothy-host doctor`;
Sperren, Neukoppeln, Zurückziehen. Jede Änderung mit Step-up und Grund.

```text
+------------------------+----------------------------------------------------------------------------+
| Verwaltung             | Rechner > Build-Rechner                         [aktiv]   [Sperren v]      |
+------------------------+----------------------------------------------------------------------------+
|   Personen             | Inhaber       (RO) Robin                                   [Ändern]        |
|   Gruppen              | Plattform     macOS, Host-Dienst 1.2.0          verbunden seit 07:58       |
|   Freigaben            | Fingerabdruck 7KQ2 9XTA M4PB 1C8D ...   Schlüssel-Generation 2             |
|   Geräte               | LLM-Anmeldung (x) Organisation  ( ) Persönlich von [Person v]  ( ) Keine   |
|   Agenten              |               Persönlich: nur diese Person darf hier ein Terminal öffnen   |
| > Rechner (3)          |               (I6), auch andere Owner nicht.                               |
|   Apps                 | Server-Host   ( ) ja  (x) nein   Meldung des Host-Dienstes: nein           |
|   Audit                | Prüfungen     HD1 ok  HD2 ok  HD3 ok  HD4 ok  HD5 ok  HD6 ok  HD7 Warnung  |
|   Statistik            | Profile       shell, claude (Arbeitsordner: server, apps)                  |
|   Instanz              | Freigaben     (RO) Robin  terminal   wirkt (Inhaberin)                     |
|                        |               (SA) Sam    terminal   wirkt nicht: nicht Inhaber (SR10)     |
|                        | [Neu koppeln]  [Sitzungen (2)]  [Zurückziehen]                             |
+------------------------+----------------------------------------------------------------------------+
```

Kopplung eines neuen Rechners:

```text
+----------------------------------------------------------------------------------------------+
| Rechner koppeln                                                          Schritt 2 von 3     |
+----------------------------------------------------------------------------------------------+
| Auf dem Rechner als Benutzer des Host-Dienstes ausführen (Code einmalig, läuft bald ab):     |
|   bothy-host pair --server https://chat.example.com --code bothy_hpc_EXAMPLE...              |
| [Kopieren]                                                       Gültig noch 14:12           |
+----------------------------------------------------------------------------------------------+
| Schritt 3: Fingerabdruck vom Rechner abtippen (erste Zeichen) [7KQ2 9XTA]                    |
|            LLM-Anmeldung auf diesem Rechner  ( ) Keine  ( ) Persönlich  ( ) Organisation     |
|            Läuft der Server auf diesem Rechner?  ( ) ja  ( ) nein                            |
+----------------------------------------------------------------------------------------------+
| [Zurück]                                         [Bestätigen mit Passkey / Gerät]            |
+----------------------------------------------------------------------------------------------+
```

### 5.3 iPhone

Navigationsstapel: Bereiche -> Kanal oder Beitragsliste -> Verlauf. Zurück per Wischen vom Rand.
Eingabe unten mit Abstand zu den sicheren Bereichen. Trefferflächen `density.target-min` (Kontext
`touch`). Push öffnet direkt den Thread an der Nachricht.

**M1 Bereiche.**

```text
+--------------------------------------+
| [^] Bothy                    [Lupe]  |
| (o) Läuft gerade              (2)    |
| v [H] Support                        |
|      # allgemein                     |
|    | = anfragen           * (3)      |
|      # meldungen                     |
| v [<>] Engineering                   |
|      = server               *        |
| > [M] Marketing             *        |
|                                      |
+--------------------------------------+
| [Bereiche] [Erwähnt] [Läuft] [Ich]   |
+--------------------------------------+
```

**M2 Beitragsliste.**

```text
+--------------------------------------+
| < Support      = anfragen       [+]  |
| [Offen][Wartet auf mich][Tags]       |
+--------------------------------------+
| (o) Gateway: resume nach Netz..      |
|     dev-runner [sync]      4 min     |
| [!] Migration 0012 prüfen            |
|     [Wartet auf dich]      12:41     |
| (@) Review: Freigabe-Assistent       |
| (v) Kosten je Bereich       2 h      |
|                                      |
| > Erledigt (14)                      |
+--------------------------------------+
```

**M3 Verlauf mit kompakter Live-Box.** Stopp immer erreichbar rechts oben in der Live-Box-Zeile;
Antippen öffnet die volle Live-Box als Blatt. Freigabe-Knöpfe untereinander in voller Breite.

```text
+--------------------------------------+
| < server   Gateway: resume ...  ...  |
+--------------------------------------+
| (o) Läuft 4:12 62 %        [STOPP]   |
|     go test ./internal/sync/...      |
+--------------------------------------+
| [A] dev-runner 10:16                 |
|  Freigabe nötig: go test ...         |
|  [      Einmal erlauben       ]      |
|  [     Für diese Sitzung      ]      |
|  [         Ablehnen           ]      |
| (AL) Alex 10:22 {..}                 |
|  Bitte auch abgelaufene Threads.     |
+--------------------------------------+
| [+] Nachtrag an dev-runner   [Mic]   |
+--------------------------------------+
```

**M4 Komponenten groß.** Wie M3: Knöpfe in voller Breite, Formulare als Blatt mit Tastatur; Auswahl
als System-Auswahlliste.

**M5 Sprachnachricht aufnehmen.** Mikrofon gedrückt halten; Wischen nach links verwirft, nach oben
sperrt die Aufnahme; Loslassen sendet. Höchstdauer `voice.max_duration` ([12-dateien.md](12-dateien.md), von `instance.read` geliefert); vor Ablauf ein
Hinweis.

```text
+--------------------------------------+
| < server   Gateway: resume ...       |
|                                      |
|                                      |
+--------------------------------------+
|   ( * )  0:17   Aufnahme läuft       |
|   ||||||||||| ||||| ||||||           |
|                                      |
|   <- zum Verwerfen wischen           |
|   ^  zum Sperren nach oben           |
|   [Loslassen = senden]               |
+--------------------------------------+
| [Mic gedrückt halten]                |
+--------------------------------------+
```

**M6 Teilen aus anderen Apps (Ziel wählen).** Eigenes Blatt der Share Extension: Vorschau,
Kommentar, Ziel aus zuletzt genutzten Threads und Kanälen mit Schreibrecht, Suche nach weiteren.
Senden geht direkt; ohne Netz landet der Eintrag in der Warteschlange der App.

```text
+--------------------------------------+
| Abbrechen   Teilen nach    [Senden]  |
+--------------------------------------+
| [Bild] screenshot.png  1,2 MB        |
| Kommentar [Siehe Fehler oben     ]   |
+--------------------------------------+
| Ziel                                 |
| > = server > Gateway: resume ..      |
|   = anfragen (neuer Beitrag)         |
|   # allgemein                        |
|   [Lupe] Andere suchen ...           |
+--------------------------------------+
| Nur Ziele mit Schreibrecht.          |
+--------------------------------------+
```

**M7 Offline.** Banner oben; wartende Nachrichten mit "wird gesendet"; dauerhafte Fehler mit
Grund und Knöpfen; nichts wird still verworfen.

```text
+--------------------------------------+
| < server   Gateway: resume ...       |
| [Wolke] Offline. 1 Nachricht wartet  |
+--------------------------------------+
| (AL) Alex 10:22 {..}                 |
|  Bitte auch abgelaufene Threads.     |
| (AL) Alex                            |
|  Und danach zusammenfassen.          |
|  {z} wird gesendet                   |
| (AL) Alex                            |
|  Anhang zu groß. {x} fehlgeschl.     |
|  [Erneut] [Bearbeiten] [Verwerfen]   |
+--------------------------------------+
| [+] Nachricht (wird später gesendet) |
+--------------------------------------+
```

**M8 Terminal mit Sondertasten-Leiste.** Vollbild-Ansicht einer Sitzung. Über der System-Tastatur
liegt die Sondertasten-Leiste: Esc, Tab, Ctrl, Alt, Pfeile, `|`, `~`, `/`, `-`. Ctrl und Alt
wirken einmal für die nächste Taste; zweimal tippen hält sie bis zum nächsten Tippen (Anzeige
gefüllt). Mit Hardware-Tastatur blendet die App die Leiste aus. Wischen mit zwei Fingern blättert
im Rückblick; langes Drücken markiert Text zum Kopieren. Geht die App in den Hintergrund, trennt sie
die Sitzung (sie läuft auf dem Rechner weiter) und verdeckt die Ansicht im App-Umschalter; zurück
in der App hängt sie innerhalb von `terminal.quick_reattach_window` ohne neue Bestätigung wieder an,
danach mit Face ID bzw. Fingerabdruck.

```text
+--------------------------------------+
| < Rechner   Build-Rechner   [Org.]   |
| shell 1  v   Rest 11:42   [Trennen]  |
+--------------------------------------+
| ~/src/server $ git status            |
| On branch main                       |
| nothing to commit, working tree clean|
| ~/src/server $ tail -f run.log       |
| 12:04:31 sync: resume ok             |
| 12:04:32 sync: catch-up 14 events    |
| _                                    |
+--------------------------------------+
| [Esc][Tab][Ctrl][Alt] [<][^][v][>]   |
| [ | ] [ ~ ] [ / ] [ - ]     [Tastat.]|
+--------------------------------------+
|  q  w  e  r  t  z  u  i  o  p        |
|   a  s  d  f  g  h  j  k  l          |
|  [^]  y  x  c  v  b  n  m  [<-]      |
|  [123]   [  Leerzeichen  ]  [Enter]  |
+--------------------------------------+
```

**M9 Rechner und Sitzungen.** Liste der Rechner, die die Person sehen darf, mit Verbindung,
Abzeichen der LLM-Anmeldung und eigenen Sitzungen; Antippen einer Sitzung hängt an.

```text
+--------------------------------------+
| < Ich        Rechner                 |
+--------------------------------------+
| (o) Build-Rechner        [Org.]      |
|     shell 1     getrennt  vor 3 min  |
|     claude: server   aktiv           |
|     [+ Neue Sitzung]                 |
| ( ) Linux-VM             [Keine]     |
|     offline seit 08:12               |
+--------------------------------------+
| Fingerabdruck und Anmeldung: [i]     |
+--------------------------------------+
```

**M10 Live.** Dieselben Ansichten wie D12 aus `bothy_viz` in Dichte `touch`, je Ansicht ein Reiter:
Topologie, Backend, Zustellung, Executive, Timeline. Die Topologie stapelt die Bahnen untereinander;
Antippen einer Lauf-Karte öffnet den Thread. Jede Ansicht hat die Tabellenansicht. Der Live-Strom
läuft nur im Vordergrund. Kein TV-Modus. Normativ: [25-visualisierung.md](25-visualisierung.md),
Abschnitt 6.

```text
+--------------------------------------+
| < Ich   Live      12:04:31   live    |
| [Topo][Backend][Zustell.][Exec][Zeit]|
+--------------------------------------+
| Engineering                          |
|  = server                            |
|   +- dev-runner -------------------+ |
|   | (o) running 4:12   [.. Bash]   | |
|   | [Org.] 0,62 USD Schätzung      | |
|   +--------------------------------+ |
| Support                              |
|  = anfragen                          |
|   +- support-agent ----------------+ |
|   | [!] waiting_input              | |
|   | Freigabe wartet  [Persönlich]  | |
|   +--------------------------------+ |
+--------------------------------------+
| Letzte Minute: Läufe 3 | Fehler 0    |
| [Tabelle]                            |
+--------------------------------------+
```

### 5.4 Android

Gleiche Screens wie iPhone, mit Android-Bedienung: System-Zurück, obere App-Leiste,
Mitteilungskanäle des Systems, Teilen über das System-Teilen-Blatt in dieselbe Ziel-Auswahl. Im
Terminal (M8) trennt System-Zurück die Sitzung, statt sie zu beenden; die Sondertasten-Leiste ist
dieselbe.

```text
+--------------------------------------+
| [<-] Gateway: resume ...   [:]       |
+--------------------------------------+
| (o) Läuft 4:12 62 %        [STOPP]   |
| ...                                  |
+--------------------------------------+
| System-Zurück: zur Beitragsliste     |
| Mitteilungskanäle in den             |
| Systemeinstellungen: Erwähnungen,    |
| Rückfragen, Alle Nachrichten, Läufe  |
| Teilen: Systemblatt -> App ->        |
|   gleiches Ziel-Blatt wie iOS        |
+--------------------------------------+
```

### 5.5 Web-Konsole (Verwaltung, Statistik und Terminal)

Gleiche Verwaltungs-Screens wie A1 bis A8, die Ansichten aus D12 und D13 und das Terminal aus D11,
Navigation links. Kein Chat (ADR 0028); Links auf Threads öffnen die App per Deep Link. Das Terminal
öffnet nur nach Bestätigung mit einem Passkey mit Nutzerprüfung; andere Step-up-Wege der Konsole
gelten dafür nicht ([ADR 0070](../adr/0070-terminal-in-der-web-konsole.md)).

```text
+--------------------------+------------------------------------------------------------------------+
| [^] Bothy Konsole        | Statistik                     Angemeldet: Alex (Passkey)               |
+--------------------------+------------------------------------------------------------------------+
|   Live                   | (wie Desktop-Statistik, volle Breite)                                  |
| > Statistik              |                                                                        |
|   Rechner und Terminal   | Kein Chat im Browser. Links in Threads öffnen die App:                 |
|   Personen               | [In der App öffnen]  (Deep Link)                                       |
|   Gruppen, Freigaben     |                                                                        |
|   Geräte                 | Terminal: Öffnen nur mit Passkey; gepinnte Rechner-Schlüssel liegen    |
|   Agenten, Kosten        | im Speicher dieses Browsers und werden beim Abmelden gelöscht.         |
|   Apps, Webhooks         |                                                                        |
|   Audit                  |                                                                        |
|   Import                 |                                                                        |
|   Instanz, Branding      |                                                                        |
|   Status                 |                                                                        |
+--------------------------+------------------------------------------------------------------------+
```

### 5.6 Onboarding and sign-in (all apps)

Screens E1 to E7 apply equally to desktop, iPhone and Android; the flows behind them are in
[08-identitaet.md](08-identitaet.md). On the phone they fill the screen; on the desktop they appear
as a card in the center of an empty window with the branding of the instance. The wireframes show
the phone form, E2 also the desktop form. The web console uses only E2 and E7: it has
no device confirmation, but requires a step-up after the login (08, section 7.5).

Rules for all onboarding screens:

- No content of the instance before the login, only name, logo and accent from
  `getDiscovery` (`branding`).
- Every code is set in groups, in a mono font (`font.family.mono`), can be copied and
  is read character by character by the screen reader. Input fields for codes accept pasting and
  ignore case, spaces and hyphens (08, section 3.2).
- Remaining times appear as text, not only as bars; the screen reader announces them at most once
  per `a11y.live_announce_min_interval`.
- Error texts name the cause and the next step; the response to a wrong code is
  always the same (08, section 3.2).
- Keyboard: Enter triggers the main action, Esc goes back one step; the focus order follows the
  reading direction.

**E1 Server address (first start).** One field for the address of the instance. "Connect" calls
`getDiscovery`. Results: on to E2; "No instance at this address" (no
valid response); "Certificate invalid" (no continue, no exception button); "Instance not set up
yet" on `setup_required` with a pointer to the setup wizard on the host
([16-betrieb.md](16-betrieb.md), section 3.1); "App too old" below `min_client_versions` with the
update path ([18-release.md](18-release.md)). The app remembers the address; a second server is
not planned (B13).

```text
+--------------------------------------+
|                                      |
|              [Logo]                  |
|   Connect to your instance           |
|                                      |
|   Server address                     |
|   [https://chat.example.com      ]   |
|                                      |
|   [          Connect            ]    |
|                                      |
|   Instance not set up yet:           |
|   The owner sets it up on the        |
|   host (install.sh). [More]          |
+--------------------------------------+
```

**E2 Login.** Name and logo of the instance at the top. Below them, one button per provider from
`auth_providers` with its `name`: OIDC opens the system browser (08, section 14.1); a proxy provider
(`trusted_header`) logs in without input (08, section 2.4). If a `local` driver is active: a field
for the login name, "Log in with passkey" (native apps only with a public domain, 08,
section 2.5) and, if `local_password_enabled`, "Password and one-time code" with a second step for
the TOTP code (autofill for the system's one-time codes on). At the bottom always "I have an
enrollment code" (E3) and "Pair with another device" (E5). If no method exists, it says
"No login is set up for this instance" with a pointer to the owner.

```text
+--------------------------------------+
| < Server        chat.example.com     |
+--------------------------------------+
|              [Logo]                  |
|        Log in to Team Chat           |
|                                      |
|   [   Log in with Company SSO    ]   |
|   -------------- or --------------   |
|   Login name  [alex             ]    |
|   [     Log in with passkey      ]   |
|   [  Password and one-time code  ]   |
|                                      |
|   I have an enrollment code          |
|   Pair with another device           |
+--------------------------------------+
```

Desktop: the same card in the center; the sidebar stays empty until the login succeeds.

```text
+------------------------------------------------------------------------------------+
|                                                                                    |
|                    +----------------------------------------+                      |
|                    |                [Logo]                  |                      |
|                    |        Log in to Team Chat             |                      |
|                    |   [     Log in with Company SSO      ] |                      |
|                    |   ---------------- or ---------------- |                      |
|                    |   Login name [alex                   ] |                      |
|                    |   [       Log in with passkey        ] |                      |
|                    |   [    Password and one-time code    ] |                      |
|                    |   Enrollment code   Pair device        |                      |
|                    |   Server: chat.example.com  [Change]   |                      |
|                    +----------------------------------------+                      |
|                                                                                    |
+------------------------------------------------------------------------------------+
```

**E3 Redeem enrollment code.** For the first device and for recovery (08, section 3.2).
First the code, then the method that the instance offers: "Create passkey", "Set up password and
one-time code" (two steps with a QR code of the one-time code and confirmation, 08, section 2.5) or
"Link with login service" (OIDC or proxy). A redeemed code makes this device
`active` immediately; the app goes straight to the main view.

```text
+--------------------------------------+
| < Login        Enrollment code       |
+--------------------------------------+
|   Code from the owner                |
|   [en_ 7K2M-9QXA-...            ]    |
|                                      |
|   How do you want to log in?         |
|   (x) Create passkey                 |
|   ( ) Password and one-time code     |
|   ( ) Link with Company SSO          |
|                                      |
|   [          Continue           ]    |
|   The code is valid once.            |
+--------------------------------------+
```

**E4 Device waiting for confirmation.** After a login on a new device (08, section
5.2). The confirmation code appears large in the center (`auth.device_verification_code_digits`
digits), below it the instructions: type in the code on an active device under Settings, Devices,
or ask an owner. The remaining time until `auth.device_pending_ttl` appears as text.
"Cancel" logs out and deletes the keys of the device. The app polls the state; after the
confirmation it goes to the main view, after rejection or expiry to E7.

```text
+--------------------------------------+
|     Device waiting for confirmation  |
+--------------------------------------+
|                                      |
|           4 8 1   2 0 7              |
|                                      |
|   Type this code on one of your      |
|   active devices: Settings,          |
|   Devices and sessions, Confirm.     |
|   No active device: ask the owner.   |
|                                      |
|   Valid for 23 more minutes          |
|   [          Cancel             ]    |
+--------------------------------------+
```

**E5 Pair with another device.** The new device shows a pairing code with
`auth.pairing_code_chars` characters and the remaining time until `auth.pairing_code_ttl` (08, section
5.3). On the active device, "Add device" (D8, Devices and sessions) opens a field for the
code and the step-up. After that the new device is `active`.

```text
+--------------------------------------+
| < Login        Pair device           |
+--------------------------------------+
|   On an active device:               |
|   Settings > Devices >               |
|   Add device, then enter:            |
|                                      |
|           HQ7P - 3MXR                |
|                                      |
|   Valid for 4 more minutes           |
|   [          New code           ]    |
+--------------------------------------+
```

**E6 App lock.** After `client.app_lock_after` without use and at start, if the person has turned
it on or `security.app_lock_required` applies (08, section 9). The content is
hidden, also in the app switcher. A button calls the system's biometrics or device PIN; after
a failure the lock stays with "Try again". "Log out" leads to E7 and deletes the
local data. The lock detaches terminal sessions (they keep running on the host, 5.3, M8).

```text
+--------------------------------------+
|                                      |
|              [Logo]                  |
|           Team Chat is locked        |
|                                      |
|   [    Unlock with Face ID      ]    |
|                                      |
|              Log out                 |
+--------------------------------------+
```

**E7 Logged out.** After logout, revocation of the device, expiry of the session
(`auth.session_max_age`), disabling of the person or rejection in E4. The reason appears as text
(`devices.revoke_reason` or the code of the session, without internal names), together with the
sentence "The data of this device has been deleted" (04, section 8.5, R-SY-14) and "Log in again"
(E2). For a disabled person there is no login button, only "Contact the owner".

```text
+--------------------------------------+
|                                      |
|   You are logged out.                |
|   Reason: This device was revoked    |
|   by another device.                 |
|                                      |
|   The data of this device has        |
|   been deleted.                      |
|                                      |
|   [         Log in again        ]    |
+--------------------------------------+
```

## 6. Interaktionsregeln

### 6.1 Erledigt und Wiederöffnen

- "Erledigt" (`thread.close`) im Thread-Kopf, im Kontextmenü der Beitragsliste und per Kürzel.
- Erledigte Beiträge stehen unten im Bereich "Erledigt (n)", standardmäßig eingeklappt; der
  Zustand des Einklappens ist persönlich je Forum.
  The collapse state is kept per forum on each device in the local database and is not
  synchronized, because 11 has no setting key for it (docs/DECISIONS.md D-M4-19-4).
- Eine neue Nachricht in einem erledigten Thread öffnet ihn wieder (`thread.reopen` implizit). Im
  Verlauf erscheint eine System-Zeile "Wieder geöffnet durch Nachricht von Sam".
- Das Wort "erledigt" als Nachricht ist kein Befehl; der Weg ist der Knopf oder ein Befehl, den ein
  Agent registriert.

### 6.2 Live-Box

- Läuft ein Lauf, ist die Live-Box oben im Thread angeheftet; am Ende klappt sie an ihrer Stelle
  im Verlauf zu einer Zeile zusammen ("Lief 28 min · 3 Agenten · 1,20 USD Schätzung").
- Aufklappen lädt das Lauf-Dokument nach; während des Laufs kommen Änderungen nur, solange der
  Thread sichtbar ist (Fokus, [04](04-sync-protokoll.md) Abschnitt 11).
- Stopp verlangt keine Rückfrage; er ist ungefährlich und muss schnell sein.
- Stillstand nach `run.stall_after` zeigt einen Hinweis in der Live-Box, keinen Push.
- Nachträge: Bei aktivem Lauf heißt die Eingabe "Nachtrag an <Agent>". Der Auslöser-Status der
  Nachricht wechselt auf "Nachtrag zugestellt", sobald der Adapter übergeben hat.

### 6.3 Auslöser-Status als Icons

Statt Status-Emoji zeigt die auslösende Nachricht ein Icon mit Beschriftung im Tooltip und im
Screenreader-Label. Icons und Farben: [components.md](../design/components.md) Abschnitt 5.

| `trigger_state` | Bedeutung für die Person |
|---|---|
| `received` | Der Server hat die Nachricht angenommen. |
| `queued` | Ein Lauf ist angelegt und wartet. |
| `working` | Der Agent arbeitet. |
| `done` | Der Lauf ist fertig. |
| `failed` | Auslösung oder Lauf ist gescheitert; der Grund steht dabei. |
| `followup_delivered` | Der Nachtrag ist beim laufenden Agenten angekommen. |

### 6.4 Lange Antworten

Nachrichten über `ui.message_collapse_lines` Zeilen werden eingeklappt mit "Mehr anzeigen";
Code-Blöcke scrollen waagrecht statt umzubrechen. Lange Antworten werden nie auf mehrere
Nachrichten verteilt.

### 6.5 Komponenten

Knöpfe gelten einmal und nur für Berechtigte. Wer nicht berechtigt ist, sieht sie gesperrt mit
Grund ("Nur der Auslöser oder ein Owner"). Nach Benutzung zeigt die Karte Person, Wahl und Zeit.
Abgelaufene Komponenten (`component.default_ttl`) zeigen "Abgelaufen". Doppelte Klicks erzeugen
keine zweite Interaktion.

### 6.6 Offline und Warteschlange

- Banner "Offline" oben im Inhalt; Verbinden läuft im Hintergrund mit Backoff.
- Gesendete Nachrichten erscheinen sofort mit "wird gesendet" am Ende des Threads.
- Dauerhafte Fehler zeigen den Grund mit "Erneut senden" (neue ID), "Bearbeiten", "Verwerfen".
- Verliert die Person den Zugriff, schließen offene Ansichten mit "Kein Zugriff mehr"; wartende
  Nachrichten bleiben als Text zum Kopieren.
- Nach dem Wiederverbinden holt die App erst nach und sendet dann ([04](04-sync-protokoll.md)
  Abschnitt 13.4).

### 6.7 Weitere Regeln

- **Nicht gefunden statt verboten.** Ein Link auf etwas Unsichtbares zeigt "Nicht gefunden oder
  kein Zugriff", gleich ob es existiert (B7).
- **Erwähnungs-Vorschläge** zeigen nur Personen und Agenten, die die Person sehen darf.
- **Titel:** automatisch gesetzte Titel tragen ein kleines Zeichen "automatisch"; Bearbeiten macht
  den Titel manuell.
- **Bediener:** Der Kanal-Kopf nennt den Bediener. Ohne Bediener steht "Kein Agent antwortet hier";
  so antwortet nie ein Agent im falschen Kanal unbemerkt.
- **Kosten:** im Modus Organisation mit "Schätzung", bis abgeglichen; im Modus Persönlich mit
  "nur Information".
- **Zerstörende Aktionen** (Nachricht löschen, Freigabe widerrufen, Gerät sperren) fragen einmal
  nach; Verwaltungsaktionen mit Step-up fragen die Bestätigung des Geräts.
- **Verschieben:** Der Dialog zeigt nur Kanäle mit gleichem Bediener; die Thread-ID bleibt.

## 7. Tastenkürzel

Kürzel gelten auf Desktop. Sie vermeiden Belegungen des Systems (Cmd+H, Cmd+M, Cmd+Q, Alt+F4,
Win-Taste). Es gibt kein Kürzel, das eine Freigabe auslöst.

| Aktion | macOS | Windows |
|---|---|---|
| Schnellwechsel | Cmd+K | Strg+K |
| Suche | Cmd+Shift+F | Strg+Shift+F |
| Im Thread suchen | Cmd+F | Strg+F |
| Neuer Beitrag bzw. neuer Chat im aktuellen Forum | Cmd+N | Strg+N |
| Eingabe fokussieren | Esc dann Tab, oder Cmd+L | Esc dann Tab, oder Strg+L |
| Nächster bzw. vorheriger Kanal | Option+Pfeil runter / hoch | Alt+Pfeil runter / hoch |
| Nächster bzw. vorheriger ungelesener Kanal | Option+Shift+Pfeil runter / hoch | Alt+Shift+Pfeil runter / hoch |
| Spalte wechseln (Seitenleiste, Liste, Verlauf, Sitzung) | F6 / Shift+F6 | F6 / Shift+F6 |
| Thread als gelesen markieren | Esc | Esc |
| Als ungelesen markieren (ab Nachricht) | Option+Klick | Alt+Klick |
| Thread erledigt bzw. wieder öffnen | Cmd+Shift+E | Strg+Shift+E |
| Lauf im aktuellen Thread stoppen | Cmd+. | Strg+. |
| Live-Box auf- und zuklappen | Cmd+Shift+L | Strg+Shift+L |
| Sitzungs-Spalte ein- und ausblenden | Cmd+Shift+I | Strg+Shift+I |
| Letzte eigene Nachricht bearbeiten | Pfeil hoch im leeren Feld | Pfeil hoch im leeren Feld |
| Senden / neue Zeile | Enter / Shift+Enter (umkehrbar) | Enter / Shift+Enter (umkehrbar) |
| Läuft gerade | Cmd+Shift+R | Strg+Shift+R |
| Erwähnungen | Cmd+Shift+M | Strg+Shift+M |
| Einstellungen | Cmd+, | Strg+, |
| Liste der Kürzel | Cmd+/ | Strg+/ |
| Rechner und Terminal öffnen | Cmd+Shift+T | Strg+Shift+T |

**Im Terminal** gehen alle Tasten an die Shell, auch Strg-Kombinationen wie Strg+C, Strg+D und
Strg+K. Kürzel der App wirken dort nur mit dem Modifikator der Plattform: auf macOS mit Cmd, auf
Windows mit Strg+Shift. Grund: Shells und Coding-Agenten belegen Strg und Alt selbst; ein Kürzel der
App darf keine Eingabe verschlucken (R-PR-10).

| Aktion im Terminal | macOS | Windows |
|---|---|---|
| Kopieren / Einfügen | Cmd+C / Cmd+V | Strg+Shift+C / Strg+Shift+V |
| Schnellwechsel | Cmd+K | Strg+Shift+K |
| Neue Sitzung auf diesem Rechner | Cmd+T | Strg+Shift+N |
| Nächste bzw. vorherige Sitzung | Cmd+Shift+] / [ | Strg+Shift+Pfeil rechts / links |
| Sitzung trennen (läuft weiter) | Cmd+Shift+D | Strg+Shift+D |
| Im Rückblick suchen | Cmd+F | Strg+Shift+F |
| Schrift größer / kleiner | Cmd+Plus / Cmd+Minus | Strg+Shift+Plus / Strg+Shift+Minus |

## 8. Barrierefreiheit

Ziel: WCAG 2.2 Stufe AA (https://www.w3.org/TR/WCAG22/, abgerufen 2026-10-10 [belegt]) in Apps und
Konsole, geprüft durch Tests, nicht durch Absicht.

| Thema | Regel | Prüfung |
|---|---|---|
| Kontrast | Jede Text- und Icon-Paarung erreicht AA (Text 4.5:1, UI 3:1) in hell und dunkel | Gate `tokens.py check` ([contrast.md](../design/contrast.md)); Widget-Tests mit `textContrastGuideline` |
| Screenreader | VoiceOver, TalkBack, Narrator; in der Konsole NVDA und VoiceOver. Jede Komponente hat Rolle und Label; Live-Box als Container mit Zusammenfassung; Auslöser-Status als eigenes Label | Widget-Tests mit `labeledTapTargetGuideline`; Semantik-Snapshots je Komponente |
| Live-Inhalte | Nur Zustandswechsel und Rückfragen werden angesagt, höchstens einmal je `a11y.live_announce_min_interval`; Patches nie | Test mit simuliertem Patch-Strom |
| Schriftgröße | Systemskalierung bis 200 Prozent ohne Verlust (SC 1.4.4); Layout bricht in den Stapel um | Golden-Tests mit Textskalierung 1 und 2 |
| Fokus | Reihenfolge je Spalte (Seitenleiste, Liste, Verlauf, Eingabe, Sitzung); F6 wechselt Spalten; Fokusring immer sichtbar | Integrationstest Tastatur-Durchlauf |
| Ziele | Trefferfläche mindestens `density.target-min` (Desktop SC 2.5.8, Touch Plattformwert) | `androidTapTargetGuideline`, `iOSTapTargetGuideline` |
| Bewegung | Systemeinstellung "Bewegung reduzieren" und eigene Einstellung schalten auf `motion.reduced` | Golden-Test im Kontext `reduced` |
| Farbe | Zustände nie nur über Farbe; Icons unterscheiden sich in der Form | Review-Liste in components.md |
| Sprache | Sprache der App im Semantik-Baum gesetzt; Agenten-Text ohne Sprachangabe wird nicht umgeschaltet | |
| Terminal | Die Zeile am Cursor und auf Befehl der sichtbare Bildschirm sind für Screenreader lesbar; laufende Ausgabe wird nicht vorgelesen; Sondertasten-Leiste mit Labels; Schriftgröße folgt der Systemeinstellung und ist im Terminal getrennt einstellbar. Wie weit der Renderer `xterm` Semantik liefert, ist nicht belegt [vermutet: wenig]; das Bau-Ticket prüft es und ergänzt eine eigene Semantik-Schicht | Widget-Test der Semantik; Prüfliste mit VoiceOver, TalkBack, Narrator |
| Visualisierungen | Jede Ansicht hat eine Tabellenansicht mit denselben Werten; Zustände mit Icon und Text ([25-visualisierung.md](25-visualisierung.md), Abschnitt 7.4) | Golden- und Widget-Tests von `bothy_viz` |

Bekanntes Risiko: Die Barrierefreiheit von Flutter auf macOS war laut einer internen Vorstudie im
Umbau [vermutet, nicht selbst geprüft]; sie wird vor der ersten macOS-Auslieferung mit VoiceOver
geprüft (Ticket in 15-client-architektur.md).

## 9. Sprachen

- Deutsch (Standard) und Englisch. Sprache folgt `users.locale`; die App startet in der
  Systemsprache, solange die Person keine gewählt hat.
- Texte in ARB-Dateien in `packages/l10n` (`app_de.arb`, `app_en.arb`), erzeugt mit
  `flutter gen-l10n`; Mehrzahl und Auswahl über ICU-Nachrichten. Fehlende Übersetzung bricht den
  Build.
- Ein Test über die ARB-Dateien lehnt Gedankenstriche (U+2013, U+2014) und Emoji ab.
- Fehler kommen vom Server als Code (RFC 9457, [06-api.md](06-api.md)); die App übersetzt den Code.
  Unbekannte Codes zeigen einen allgemeinen Text plus Code.
- Agenten-Inhalte werden nicht übersetzt.
- Zahlen, Daten, Währungen nach Gebietsschema der App (z. B. "1,84 USD" bzw. "USD 1.84").
- Begriffe sind festgelegt, damit DE und EN dasselbe meinen:

<!-- language-check: off -->

| Deutsch | Englisch | Code |
|---|---|---|
| Bereich | Space | `space` |
| Kanal | Channel | `channel` |
| Forum, Beitrag | Forum, Post | `forum`, `thread` |
| Thread | Thread | `thread` |
| Erledigt | Done | `closed` |
| Live-Box | Live box | `run` |
| Lauf | Run | `run` |
| Nachtrag | Follow-up | `followup` |
| Bediener | Operator | `operator` |
| Freigabe | Access grant | `grant` |
| Agenten auslösen (Stufe) | Trigger agents | `trigger` |
| Aktionen freigeben | Approve actions | `approve_actions` |
| Persönlich, Organisation | Personal, Organization | `personal`, `organization` |
| Schätzung | Estimate | `cost_is_estimate` |
| Rechner | Host | `host` |
| Host-Dienst | Host service | `bothy-host` |
| Kopplung, Kopplungs-Code | Pairing, pairing code | `host.pair` |
| Fingerabdruck | Fingerprint | `key_fingerprint` |
| Terminal-Sitzung | Terminal session | `terminal_session` |
| Trennen, Wiederanhängen | Detach, reattach | `detached`, `terminal.attach` |
| Sondertasten-Leiste | Extra keys bar | |
| LLM-Anmeldung | LLM login | `llm_credentials` |
| Live-Topologie | Live topology | |
| TV-Modus | TV mode | |
| Eigene Messung | Own measurement | `stats_minutes` |

<!-- language-check: on -->

Implementation in `packages/l10n` (M4-06, docs/DECISIONS.md D-M4-06-1 to D-M4-06-7):

- `app_en.arb` is the template of `flutter gen-l10n` and German is the default locale
  (`supportedLocales` lists `de` first). Every message has an `@key` with a `description`.
- Problem codes: each code of `x-problem-codes` in `docs/api/openapi.yaml` has the message
  `problem<Code>` (for example `problemNotFound`); a reason that 07 names selects
  `problem<Code><Reason>` (for example `problemForbiddenCapabilityMissing`). Any other code
  shows `problemFallback` with the code: "Something went wrong ({code})" or "Etwas ist
  schiefgegangen ({code})".
- The table above is the source of `packages/l10n/lib/src/term_table.dart`, generated by
  `packages/l10n/tool/term_table.dart`; a test fails when the generated file is stale and when an
  English message uses a term of the table while the German message of the same key lacks the
  German term (direction English to German).
- The language gate of the repository excludes the German product strings by path:
  `packages/l10n/lib/l10n/app_de.arb` and the generated
  `packages/l10n/lib/src/generated/app_localizations_de.dart` in `tools/docs/language-allow.txt`.

## 10. Benachrichtigungen

### 10.1 Wann ein Hinweis kommt

Normativ für Arten, Empfänger, wirksame Stufe und Ruhezeiten ist
[13-push-benachrichtigungen.md](13-push-benachrichtigungen.md), Abschnitte 2 bis 4. Für die
Oberfläche gilt kurz:

| Ereignis | Stufe `all` | Stufe `mentions` | Stufe `mute` |
|---|---|---|---|
| Neue Nachricht in einem sichtbaren Thread (nicht eigene) | ja | nein | nein |
| Erwähnung meiner Person, Nachricht in meiner Direktnachricht | ja | ja | nein |
| Lauf, den ich ausgelöst habe, wartet auf mich (`waiting_input`) | ja | ja | ja |
| Lauf, den ich ausgelöst habe, ist fertig oder fehlgeschlagen | ja | ja | nein |
| Nachrichten von Agenten ohne Erwähnung | ja | nein | nein |
| Importierte oder stille Nachrichten (`silent`) | nein | nein | nein |

In Ruhezeiten kommen Hinweise leise (kein Ton, kein Wecken); Zähler und Abzeichen laufen weiter.
Laut bleibt nur eine Rückfrage eines eigenen Laufs, und nur wenn die Person
`notify.input_needed_breaks_quiet_hours` eingeschaltet hat. Jeder Hinweis läuft vorher durch
`push.deliver`.

### 10.2 Inhalt

Der Push selbst trägt nur IDs und einen neutralen Platzhalter ("Neue Nachricht"). Die App holt den
Text selbst ([15](15-client-architektur.md), Hintergrundverhalten) und zeigt danach
"<Kanal> · <Thread-Titel>" als Titel und "<Autor>: <Text, gekürzt>" als Text. Klappt das Holen
nicht (kein Netz zum Server, Zeit abgelaufen), bleibt der Platzhalter. Was die Vorschau zeigen
darf, steuert `notify.preview_mode` (`full`, `names`, `none`); bei `none` holt die App nie Text,
dann steht dort nur der Platzhalter mit dem Kanalnamen, den die App lokal kennt.

### 10.3 Plattform

| Plattform | Verhalten |
|---|---|
| iOS, macOS | Gruppierung je Thread; Antippen öffnet den Thread an der Nachricht; Abzeichen = ungelesene Erwähnungen plus Rückfragen |
| Android | Mitteilungskanäle "Erwähnungen", "Rückfragen", "Alle Nachrichten", "Läufe", in den Systemeinstellungen änderbar; Gruppierung je Thread |
| Windows | App im Infobereich hält die Verbindung und zeigt lokale Hinweise; beendete App bekommt keine Hinweise (Baseline R-Push) |
| alle | Ist der Thread gerade offen und die App im Vordergrund, gibt es keinen Hinweis, nur den Verlauf |

Gelesenes auf einem Gerät entfernt Mitteilungen auf den anderen, sobald diese App das nächste Mal
aktiv ist; ein eigener Push nur zum Aufräumen ist später zu prüfen.

## 11. Einstellungen der Person

Synchronisiert über `user_settings` ([04](04-sync-protokoll.md) Abschnitt 14). Diese Tabelle ist
für alle persönlichen Schlüssel außer `notify.*` normativ ([03-datenmodell.md](03-datenmodell.md),
`user_settings`); die Schlüssel `notify.*` stehen in
[13-push-benachrichtigungen.md](13-push-benachrichtigungen.md), Abschnitt 3.3. Der Server lehnt
unbekannte Schlüssel und Werte außerhalb der Liste ab.

| Schlüssel | Werte | Standard | Bedeutung |
|---|---|---|---|
| `appearance.theme` | `system`, `light`, `dark` | `system` | Theme; `system` folgt dem Gerät |
| `appearance.density` | `comfortable`, `compact` | `comfortable` | nur Desktop; Mobil fest `touch` |
| `appearance.reduce_motion` | `system`, `on`, `off` | `system` | Bewegung reduzieren |
| `composer.enter_sends` | `true`, `false` | ohne gespeicherten Wert `true` auf Desktop und `false` auf Mobil; ein gespeicherter Wert gilt auf allen Geräten | Enter sendet |
| `privacy.crash_reports` | `true`, `false` | `true`, wenn die Instanz einen Absturzdienst eingerichtet hat | Absturzberichte senden |
| `privacy.client_metrics` | `true`, `false` | `true` | Leistungsdaten ohne Inhalte senden |
| `layout.column_widths` | Objekt: Namen `^[a-z][a-z0-9_]*$` (Spalte, z. B. `sidebar`, `post_list`, `session`), Werte nicht negative Ganzzahlen in logischen Pixeln | leer | Spaltenbreiten Desktop; der Client deutet die Namen |

## 12. Branding je Instanz

Zwei Ebenen (ADR 0044):

| Ebene | Was | Wo festgelegt | Wirkung |
|---|---|---|---|
| Laufzeit | Produktname, Logo, Akzent | `instance_settings`: `branding.product_name`, `branding.logo_file_id`, `branding.accent_color` (Aktion `instance.branding.update`) | App und Konsole zeigen Name und Logo; Akzent tauscht `color.accent.*` |
| Build | App-Name im System, Bundle- und Paket-IDs, App-Icons, URL-Schema für Deep Links und Anmeldung, Push-Themen (APNs-Topic = Bundle-ID, Firebase-Projekt), Update-Feeds (Sparkle, `.appinstaller`), Signier-Identitäten, Standard-Server-Adresse | Datei `branding.yaml` der Instanz (liegt im Betreiber-Repo, nicht im Produkt), gelesen vom Build | eigene, signierte App je Instanz |

Regeln:

- `branding.accent_color` ist ein Schlüssel aus den Kategorie-Farben (`spruce`, `loch`, `heather`,
  `rowan`, `amber`, `gorse`, `moss`), kein freier Farbwert; nur diese Kombinationen sind im
  Kontrast-Test (03, `instance_settings`; Schema in der Konfigurationsreferenz).
  Every variant of these accents, in light and dark, is in the contrast table
  ([contrast.md](../design/contrast.md)); the design generator fails when one falls below AA.
- Ohne eigene Build-Konfiguration baut das Repo eine neutrale Fassung mit dem Arbeitsnamen und
  Platzhalter-IDs unter `example.com`.
- Das Logo wird als Datei hochgeladen und nur über den Server ausgeliefert; die App fällt auf das
  Zeichen des Produkts zurück, solange es fehlt.
- `branding.product_name` hat höchstens `branding.product_name_max_chars` Zeichen (03) und keine
  Steuer- oder Bidi-Zeichen ([19-sicherheit.md](19-sicherheit.md), SEC-IN-4).

## 13. Entscheidungen dieses Kapitels

| Nr | Entscheidung | Grund |
|---|---|---|
| R-PR-01 | Vier Spalten auf Desktop; Beitragsliste nur bei Foren; rechte Spalte zeigt die Sitzung statt Mitgliedern. | Studie B; Mitgliederlisten sind bei Agenten-Arbeit ohne Nutzen, Sitzungsdaten schon. |
| R-PR-02 | Präsenz nur für Agenten, nie für Menschen. | Datenschutz und Mitbestimmung; für Agenten ist Präsenz Betriebsinformation. |
| R-PR-03 | Keine Lesebestätigungen für andere; Gelesen-Stand bleibt persönlich. | Leistungskontrolle vermeiden; nicht im Baseline-Umfang. |
| R-PR-04 | Kein Tastenkürzel löst eine Freigabe aus. | Fehlbedienung bei Freigaben ist teuer. |
| R-PR-05 | Rückfragen eigener Läufe melden sich auch bei Stufe "stumm" (13, Art `input_needed`). | Ein wartender Agent blockiert die Arbeit der Person, die ihn gestartet hat. |
| R-PR-06 | Ruhezeiten machen Hinweise leise; laut wird nur eine Rückfrage eines eigenen Laufs, wenn die Person es selbst einschaltet (13). | Verlässlichkeit vor Dringlichkeit; die einzige Ausnahme liegt in der Hand der Person. |
| R-PR-07 | Mobil gibt es Notfall-Aktionen der Verwaltung, nicht die ganze Verwaltung. | Kleine Bildschirme für komplexe Rechte sind fehleranfällig; Sperren muss aber unterwegs gehen. |
| R-PR-08 | Akzent je Instanz nur aus der geprüften Palette. | Kontrast AA ist ein Test, keine Hoffnung. |
| R-PR-09 | Link-Vorschau im Client nur später und nur mit Opt-in. | Der Server lädt keine fremden URLs; der Client würde fremde Server kontaktieren. |
| R-PR-10 | Im Terminal gehen alle Tasten an die Shell; Kürzel der App nur mit Cmd (macOS) bzw. Strg+Shift (Windows). | Shells und Coding-Agenten belegen Strg und Alt; ein verschlucktes Strg+C wäre ein Bedienfehler mit Folgen. |
| R-PR-11 | Mobil eine Sondertasten-Leiste mit einmal wirkenden Modifikatoren statt einer eigenen Tastatur. | Die System-Tastatur bleibt (Diktat, Sprachen, Barrierefreiheit); es fehlen nur die Tasten, die eine Shell braucht. |
| R-PR-12 | Die App trennt das Terminal beim Wechsel in den Hintergrund und verdeckt es im App-Umschalter. | Mobile Systeme beenden Verbindungen im Hintergrund ohnehin; das Bild im App-Umschalter würde Terminal-Inhalte zeigen. |
| R-PR-13 | Produktbeschreibung "Kommandozentrale für KI-Agenten in Chat-Form" mit sechs Säulen (Chat, Live-Ansicht, Freigaben, Terminal, Kosten, Statistik). | Vorgabe des Owners (Nachtrag vom 2026-10-10); sie bestimmt Startseite, Doku und Schaufenster. |
| R-PR-14 | Onboarding screens E1 to E7 shared by all apps; before login only the branding of the instance; with an invalid certificate no continue and no exception button. | One flow for all platforms keeps the security rules from 08 in one place; an exception button would let an attacker in the middle defeat DPoP and device binding. |

## 14. Konfigurationswerte dieses Kapitels

| Name | Standard | Bedeutung |
|---|---|---|
| `ui.message_group_window` | `5m` | Nachrichten desselben Autors innerhalb dieser Zeit ohne eigenen Kopf |
| `ui.message_collapse_lines` | `24` | ab so vielen Zeilen wird eine Nachricht eingeklappt |
| `ui.livebox_visible_steps` | `6` | sichtbare Schritte in der laufenden Live-Box |
| `ui.composer_max_lines` | `12` | so weit wächst die Eingabe |
| `ui.autocomplete_debounce` | `150ms` | Wartezeit vor `command.autocomplete` |
| `ui.quick_switch_max_results` | `20` | Treffer im Schnellwechsel |
| `ui.desktop_min_width_four_columns` | `1280px` | darunter klappt die Sitzungs-Spalte ein |
| `ui.desktop_min_width_three_columns` | `960px` | darunter wird die Beitragsliste zum Stapel |
| `a11y.live_announce_min_interval` | `10s` | höchstens so oft sagt die Live-Box einen Wechsel an |
| `client.share.recent_targets` | `8` | zuletzt genutzte Ziele im Teilen-Blatt |
