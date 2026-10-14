# Wiederverwendung: Lehren aus drei Vorgängern

Status: Entwurf M0. Dieses Dokument hält fest, was Bothy aus drei Vorgängerprojekten übernimmt:
Konzepte, Testmuster und höchstens drei kleine Code-Stücke zum Neuschreiben. Es übernimmt keinen
Code durch Kopieren. Die Vorgänger, ihre Dateien und Belege stehen nur im privaten Ops-Repo; hier
heißen sie neutral V1, V2 und V3.

## 1. Regeln

1. Übernommen wird ein Konzept, wenn es ein Problem löst, das auch Bothy hat. Löst ein Konzept nur
   eine Eigenheit der damaligen Plattform, wird es nicht übernommen (Abschnitt 6).
2. Jede Lehre nennt: Problem, Lehre, wo Bothy sie umsetzt (Kapitel oder ADR) und den Test, der sie
   hält. Ein Test, der hier steht, ist ein Bau-Auftrag; er gilt erst als vorhanden, wenn er im Repo
   läuft.
3. Code-Stücke werden neu geschrieben, nicht kopiert (Abschnitt 5, Lizenzhinweis).

## 2. Die Vorgänger

| Kürzel | Was es war | Was davon zählt |
|---|---|---|
| V1 | Plattform-Adapter für einen fremden Chat-Server: Plugin für das Agenten-Framework Hermes Agent, dazu Struktur-Spiegel, History-Import und Live-Abgleich aus Discord | Zustellung und Doppel, Nachholen, Kanal-Verhalten, Import-Werkzeug, Ruhe-Regel, CDN-Abruf, Testgerüst |
| V2 | Dev-Foren-Runner: je Forum-Beitrag eine Claude-Code-Sitzung im Projektordner, mit Live-Box und Nachträgen | Sitzungsvertrag, Prozessvertrag, Nachträge, Steuerbefehle, Fehlerpfade, Kosten |
| V3 | Chat- und Import-Plugin: neue Chats aus einem Übergabekanal, Auto-Titel, Import alter Threads mit Bindung an frühere Sitzungen | Auto-Titel, Sitzungsbindung wie `/resume`, Import-Regeln, Temp-Wächter |

Alle drei sind Plugins für Hermes Agent (github.com/NousResearch/hermes-agent). V2 und V3 arbeiten
in Discord, V1 liest aus Discord. Getestet wurden sie vor allem gegen Fake-Server.

## 3. Lehren

### L01 Identität an einem Namen (V1)

- Problem: Der fremde Chat-Server kannte Gespräche nur über ihren Namen. Umbenennen und
  Verschieben brachen die Zuordnung zur Agenten-Sitzung; ein eigenes Register flickte das, und
  Umbenennungen bei gestopptem Adapter gingen verloren.
- Lehre: Gespräch und Sitzung hängen an einer festen ID, nie an einem Namen.
- Umsetzung: `threads.id` bleibt beim Verschieben gleich; `agent_sessions` je Thread
  ([03-datenmodell.md](spec/03-datenmodell.md)); Sitzungsschlüssel des Hermes-Adapters aus der
  Thread-ID ([09-agenten.md](spec/09-agenten.md)).
- Test: Thread mit aktiver Sitzung umbenennen und verschieben; dieselbe `agent_sessions`-Zeile
  bleibt aktuell, der nächste Lauf setzt dieselbe externe Sitzung fort.

### L02 Unsichere Zustellung erzeugt Doppel (V1, zweimal)

- Problem: Bei Zeitüberschreitung, `500`, `502` oder `504` hatte der Server manchmal schon
  gespeichert. Raten führte zu Doppeln oder Verlust. Dieselbe Fehlerklasse kam zweimal; die
  Lösung war eine Nachfrage per Inhaltsvergleich.
- Lehre: Jede Schreibanfrage trägt einen vom Client erzeugten Schlüssel, den der Server eindeutig
  erzwingt; eine Wiederholung liefert das Original.
- Umsetzung: `client_msg_id`, `client_interaction_id`, `Idempotency-Key`
  ([04-sync-protokoll.md](spec/04-sync-protokoll.md), Abschnitt 5); ADR 0015.
- Test: Testmuster 4.1 mit "gespeichert, dann 502"; die Wiederholung ergibt genau eine Nachricht
  und `Idempotent-Replay: true`.

### L03 Nachholen nach Ausfall, dreimal gebaut (V1, V2, V3)

- Problem: Jeder Vorgänger baute ein eigenes Nachholen mit eigenen Grenzen (Seiten, Alter, "nie
  vor der Aktivierung"). V1 brauchte zusätzlich ein Lücken-Modell, zurückgehaltene
  Live-Ereignisse und Hinweise auf verspätete Nachrichten.
- Lehre: Ein Ereignis-Log mit commit-geordneter Sequenz und ein dauerhafter Cursor je
  Verbraucher. "Nie vor der Aktivierung" ist der Startpunkt des Cursors.
- Umsetzung: [04-sync-protokoll.md](spec/04-sync-protokoll.md), Abschnitte 3, 7 und 15.1; ADR 0009.
- Test: Absturz eines Adapters mitten im Nachholen, Neustart: kein Ereignis doppelt verarbeitet
  (Dedupe über Ereignis-ID), keines fehlt; Live-Ereignisse erst nach `catchup.complete`.

### L04 Antwort im falschen Kanal (V1, Konzept-Studie)

- Problem: Freigaben standen verstreut in mehreren Listen je Plattform (erlaubte, ignorierte,
  freie Kanäle). Ein Agent antwortete in einem Kanal, der ihm nicht gehörte.
- Lehre: Genau ein Bediener je Kanal; ohne Bediener antwortet niemand; eine Prüfstelle.
- Umsetzung: Bediener in `channels` ([03-datenmodell.md](spec/03-datenmodell.md)), `agent.invoke`
  ([05-aktionen.md](spec/05-aktionen.md), Abschnitt 5); ADR 0017.
- Test: In einem Kanal ohne Bediener erzeugt keine Nachricht einen Lauf; der Routen-Test findet
  keine Route ohne Aktion.

### L05 Schleifen im Doppelbetrieb (V1)

- Problem: Kopien und Agenten-Antworten sahen für den Adapter aus wie Eingaben. Der
  Schleifenschutz hing an Kopfzeilen-Mustern im Text.
- Lehre: Herkunft ist ein Datenfeld; nur Menschen lösen aus.
- Umsetzung: `messages.origin`; `agent.invoke` Regel 1.
- Test: Nachrichten mit `origin` `agent`, `app`, `import` und `system` in einem Kanal mit
  `free_response` erzeugen keinen Lauf.

### L06 Kanal-Verhalten als lose Schlüssel (V1)

- Problem: Verhalten je Kanal kam aus untypisierten Zusatzschlüsseln der Framework-Konfiguration.
  Ein Tippfehler blieb still ungelesen und war nur live zu bemerken.
- Lehre: Kanal-Verhalten als typisierte Felder mit Schema; Unbekanntes wird abgelehnt.
- Umsetzung: `require_mention`, `thread_require_mention`, `free_response`, `auto_thread`,
  `history_backfill_limit`, `prompt`, `channel_skill_bindings` in
  [03-datenmodell.md](spec/03-datenmodell.md); striktes Schema in [06-api.md](spec/06-api.md).
- Test: `channel.behavior.update` mit unbekanntem Feld ergibt `400`.

### L07 Plattform-Grenzen formen die Fortschrittsanzeige (V1, V2)

- Problem: Bearbeitungsfrist für Bot-Nachrichten, Längengrenze und Rate-Limits beim Bearbeiten
  zwangen die Live-Box zu Verdichtung, Drosselung, Teilung und Löschen vor der Antwort.
- Lehre: Der Lauf ist ein eigenes Objekt mit Patches; keine Frist, kein Teilen.
- Umsetzung: `runs`, `run_steps`, `run.updated` ([04-sync-protokoll.md](spec/04-sync-protokoll.md),
  Abschnitt 11); ADR 0025.
- Test: Lauf mit vielen Patches über lange Zeit: jeder Patch wird angenommen, das Dokument ist am
  Ende vollständig; eine Antwort bis `message.max_content_bytes` bleibt eine Nachricht.

### L08 "Erledigt" als Wort, Tag oder Präfix (V1, V2, V3)

- Problem: Erledigt war je Vorgänger ein Wort, ein Tag, ein Archiv-Merkmal oder ein
  Namenspräfix. Jede Bot-Nachricht holte einen archivierten Thread zurück.
- Lehre: `open` und `closed` als Feld; eine neue Nachricht öffnet wieder; die Sitzung bleibt.
- Umsetzung: `threads.state`; `thread.close`, `thread.reopen`.
- Test: Nachricht in einem erledigten Thread: Zustand `open`, dieselbe aktuelle Sitzung.

### L09 Fremde Konfiguration per Textmuster geändert (V1, V3)

- Problem: Ein Installer änderte die YAML-Konfiguration des Agenten-Frameworks per Textpatch.
  Das Framework schreibt Listen in Blockform; dieselbe Fehlerklasse trat an drei Stellen auf;
  `[]` und `null` bedeuten dort Verschiedenes.
- Lehre: Fremde Konfiguration nie per Text raten. Vorher und nachher mit dem echten Parser des
  Frameworks lesen und vergleichen; bei unklarem Zustand abbrechen.
- Umsetzung: Installationshilfe des Hermes-Referenz-Adapters ([09-agenten.md](spec/09-agenten.md),
  [11-erweiterungen.md](spec/11-erweiterungen.md)).
- Test: Installer gegen Fixtures in Block- und Inline-Form; danach liest der Parser des Frameworks
  genau die alte Konfiguration plus den neuen Abschnitt.

### L10 Geheimnisse rutschen durch eine Sperrliste (V2, zweimal)

- Problem: Der Runner gab dem Kindprozess seine Umgebung ohne gesperrte Schlüssel. Zweimal kamen
  Geheimnisse des Agenten-Frameworks trotzdem durch.
- Lehre: Die Umgebung von Kindprozessen ist eine Erlaubnisliste.
- Umsetzung: Dev-Runner-Adapter ([09-agenten.md](spec/09-agenten.md), Baseline-Ruling zum
  Claude-Code-Runner); [21-dev-isolation.md](spec/21-dev-isolation.md).
- Test: Eine Kanarienvariable in der Umgebung des Adapters; der gestartete Fake-Prozess meldet
  seine Umgebung; sie enthält nur Schlüssel der Erlaubnisliste, die Kanarie fehlt.

### L11 Abgebrochene Turns automatisch wiederholen (V2)

- Problem: Ein Neustart des Turns nach einem Absturz hätte Nebenwirkungen doppelt ausgeführt.
- Lehre: Nie automatisch wiederholen; als unterbrochen melden; der Mensch entscheidet.
- Umsetzung: Lauf `failed` mit `state_reason = stale` ([03-datenmodell.md](spec/03-datenmodell.md));
  [09-agenten.md](spec/09-agenten.md).
- Test: Adapter-Neustart während eines Laufs: Lauf `failed` mit `stale`; ein zweiter Start mit
  derselben Aufruf-Marke ergibt `409 already_started`.

### L12 Nachträge während der Arbeit (V2, Busy-Modus des Frameworks)

- Problem: Nachrichten während eines Turns gingen je nach Weg verloren, unterbrachen den Turn
  oder wurden ein eigener Turn. Sichtbar war das nur über Reaktionen.
- Lehre: Ein Nachtrag hat einen eigenen Zustand (wartet, übergeben, aufgenommen, verschoben). Den
  Modus des Frameworks immer ausdrücklich setzen, nie den Standard erben.
- Umsetzung: `run_followups`, `trigger_state = followup_delivered`
  ([04-sync-protokoll.md](spec/04-sync-protokoll.md), Abschnitt 15.4); Hermes-Adapter mit
  `busy_input_mode` `steer` oder `queue` (Baseline-Ruling zum Hermes-Adapter).
- Test: Nachtrag während eines Laufs durchläuft die Zustände in Reihenfolge; nicht aufgenommene
  Nachträge am Laufende werden ein neuer Lauf.

### L13 Steuerbefehle als Prompt (V2)

- Problem: `stop` und `status` dürfen nie an das Modell gehen und nicht hinter einem Turn warten.
- Lehre: Steuerbefehle sind ein eigener Typ.
- Umsetzung: `commands.control`; [04-sync-protokoll.md](spec/04-sync-protokoll.md), Abschnitt 15.4.
- Test: `stop` während eines Laufs: sofort `command.invoked`, kein Nachtrag, Lauf `cancelled`.

### L14 Sitzungs-ID erst nach dem Start bekannt (V2)

- Problem: Ohne vorab vergebene ID ließ sich ein Lauf nach einem frühen Absturz keiner Sitzung
  zuordnen.
- Lehre: Die Sitzungs-ID wird vor dem ersten Lauf vergeben und gespeichert; der erste Lauf legt
  an, jeder weitere setzt fort.
- Umsetzung: `agent_sessions.external_session_ref` und `started`; Dev-Runner mit vorab vergebener
  Sitzungs-ID (Baseline-Ruling zum Claude-Code-Runner).
- Test: Absturz vor der ersten Antwort; der nächste Lauf setzt dieselbe ID fort und legt keine
  zweite an.

### L15 Kosten sind kumulativ je Sitzung (V2)

- Problem: Die Laufzeit meldet Kosten erst am Turn-Ende und als Summe der ganzen Sitzung.
  Aufaddieren zählt doppelt.
- Lehre: Delta selbst rechnen und als Schätzung kennzeichnen; übernommene Sitzungen brauchen einen
  Sockel.
- Umsetzung: `cost_entries.is_estimate`; [10-backend-richtlinie.md](spec/10-backend-richtlinie.md);
  [22-import-migration.md](spec/22-import-migration.md), Abschnitt 9.
- Test: Zwei Turns derselben Sitzung mit kumulativen Werten: Summe der Einträge gleich letzter Wert
  minus Sockel.

### L16 Alte Sitzung nach einem Import fortsetzen (V3)

- Problem: Importierte Threads sollten ihren alten Verlauf behalten. Umstellen ohne Nachricht ging
  nicht, doppeltes Umstellen hätte Verlauf verschoben, und eine Störung durfte den Menschen nicht
  stumm schalten.
- Lehre: Referenz beim Import speichern, beim ersten Schreiben binden wie `/resume`, genau einmal
  mit Compare-and-swap, fail open mit genau einem Hinweis. Fehlt die alte Sitzung, beginnt eine
  neue.
- Umsetzung: [22-import-migration.md](spec/22-import-migration.md), Abschnitt 9.
- Test: Zwei gleichzeitige erste Auslösungen binden genau einmal; bei fehlender alter Sitzung
  entsteht eine neue, genau eine Systemnachricht, und die Nachricht des Menschen erreicht den
  Agenten.

### L17 Werkzeug schreibt bei vergessenem Schalter (V1, V3)

- Problem: V3 verlangte einen von zwei Schaltern, V1 machte den Trockenlauf zum Standard. Beide
  wollten verhindern, dass ein versehentlicher Aufruf anlegt.
- Lehre: Trockenlauf ist der Standard; widersprüchliche Optionen und eine ungültige Auswahl
  brechen vor jedem Aufruf ab.
- Umsetzung: [22-import-migration.md](spec/22-import-migration.md), Abschnitt 10; ADR 0043.
- Test: Testmuster 4.7; eine unbekannte ID in `--only-thread` bricht vor jedem Schreiben ab.

### L18 Idempotenz über das Ziel, nicht über eine lokale Datei (V1, V3)

- Problem: Ein lokaler Zustand schützt nicht, wenn er verloren geht oder ein Lauf zwischen
  Anlegen und Vermerk abbricht. V1 fragte nach unsicheren Sendungen per Inhaltsvergleich nach.
- Lehre: Der Herkunftsschlüssel ist im Zielsystem eindeutig; `pending` vor dem Anlegen, `posted`
  in derselben Transaktion wie das Ziel.
- Umsetzung: `import_items`; [22-import-migration.md](spec/22-import-migration.md), Abschnitt 7;
  ADR 0043.
- Test: Abbruch zwischen Stapel und Cursor, Neustart auf einem anderen Rechner ohne lokale Datei:
  jedes Objekt genau einmal.

### L19 Archiviert nur bei ausdrücklichem Merkmal (V3)

- Problem: Ein zwischen zwei Abrufen archivierter Thread stand in beiden Listen; fehlende
  Metadaten waren mehrdeutig.
- Lehre: Zweifel heißt offen. Ein falsch offener Thread schadet nicht, ein falsch geschlossener
  versteckt Arbeit.
- Umsetzung: [22-import-migration.md](spec/22-import-migration.md), Abschnitt 5.1.1.
- Test: Thread in aktiver und archivierter Liste, Thread ohne Metadaten: beide `open`.

### L20 Import ohne Benachrichtigung (V3)

- Problem: Ein Import alter Threads hätte eine Flut von Erwähnungen und Benachrichtigungen
  ausgelöst.
- Lehre: Importiertes ist still, ohne Erwähnungs-Zeilen, und gilt als gelesen.
- Umsetzung: [22-import-migration.md](spec/22-import-migration.md), Abschnitt 6.4.
- Test: Nach einem Import gibt es keinen Push, keine Zeile in `mentions`, und `read_states` steht
  am Ende jedes importierten Threads.

### L21 Halbfertige Agenten-Ausgaben im Parallelbetrieb (V1)

- Problem: Agenten bearbeiten ihre Antwort, während sie arbeiten. Eine zu früh kopierte Nachricht
  bleibt halbfertig, weil spätere Bearbeitungen nicht nachgeführt werden.
- Lehre: Ruhe-Regel: nicht kopieren, solange eine Arbeits-Markierung da ist oder die letzte
  Änderung zu jung ist; je Thread streng der Reihe nach, Halt an der ersten unruhigen Nachricht.
- Umsetzung: [22-import-migration.md](spec/22-import-migration.md), Abschnitt 12.3; Code-Stück 5.3.
- Test: Tabelle aus Nachrichtenfolgen mit Markierung, Bearbeitungszeit und Typ; erwartet ist der
  kopierbare Anfang.

### L22 Ablaufende CDN-Links und Lecks in Fehlertexten (V1)

- Problem: Anhang-URLs sind signiert und laufen ab; Fehlertexte und Fehlerkörper konnten die
  signierte URL oder das Token enthalten.
- Lehre: Nur erlaubte Hosts, nur `GET`, kein Token an den CDN, Größe vor dem Lesen begrenzen,
  Fehlertexte nur mit Schema, Host und Pfad, bei Ablauf einmal neu lesen, sonst Sprunglink.
- Umsetzung: [22-import-migration.md](spec/22-import-migration.md), Abschnitt 6.5; Code-Stück 5.2.
- Test: Testmuster 4.3 gegen Fake-CDN, das Query und Token in Fehlerkörpern spiegelt.

### L23 Auto-Titel überschreibt echte Titel (V3)

- Problem: Der Agent sollte Gesprächen einen Titel geben, aber nie einen Titel überschreiben, den
  ein Mensch gewählt hat. Kurze echte Namen ("KI", "M365") sahen aus wie Platzhalter.
- Lehre: Titelquelle als Feld; automatische Titel nur bei `auto`; eine Platzhalter-Regel mit
  geschützten kurzen Namen entscheidet, ob ein menschlicher Titel als `auto` startet.
- Umsetzung: `threads.title_source`, `thread.title.update` ([05-aktionen.md](spec/05-aktionen.md));
  Code-Stück 5.1.
- Test: Tabellen-Test mit Platzhaltern und geschützten kurzen Namen.

### L24 Neuer Chat über einen Umweg-Kanal (V3)

- Problem: Die Plattform hatte keinen Knopf "Neuer Chat". Ein Plugin legte aus einer Nachricht in
  einem Übergabekanal einen Beitrag an und brauchte eine eigene Zustandsmaschine gegen Doppel und
  Waisen.
- Lehre: Ein neuer Thread ist eine Server-Funktion; kein Umweg.
- Umsetzung: `thread.create`; [01-produkt.md](spec/01-produkt.md).
- Test: `thread.create` mit derselben `client_msg_id` der ersten Nachricht legt bei Wiederholung
  keinen zweiten Thread an.

### L25 Nur gegen Fakes bewiesen (V1, V2, V3)

- Problem: Der Eingang eines Menschen auf der fremden Plattform war live nicht prüfbar; mehrere
  Verhalten blieben "nur gegen Fakes geprüft".
- Lehre: Auf dem eigenen Server laufen End-to-End-Tests mit Testpersonen und Test-Agenten; die
  Kopplung an das Agenten-Framework wird gegen dessen echten Loader geprüft (Testmuster 4.8).
- Umsetzung: [23-qualitaet.md](spec/23-qualitaet.md).
- Test: E2E-Suite legt eine Testperson und einen Test-Agenten an, sendet, prüft Lauf, Live-Box und
  Antwort.

### L26 Blockierende Verbindung beim Stoppen (V1)

- Problem: Ein blockierender Abruf hielt den Prozess beim Herunterfahren länger, als der
  Dienst-Manager erlaubte.
- Lehre: Jede Verbindung eines Adapters ist abbrechbar; Stoppen endet in einer festen Frist.
- Umsetzung: Adapter-Vertrag ([09-agenten.md](spec/09-agenten.md)); SDKs (ADR 0026).
- Test: Stopp-Signal bei offener Verbindung; der Prozess endet innerhalb der konfigurierten Frist.

## 4. Testmuster

### 4.1 Fake-Server mit Fehlerinjektion (V1, V3)

- Aufbau: Ein HTTP-Fake im Testprozess, nur auf `127.0.0.1`, Port vom System vergeben, mit
  Anfrage-Log und gezielten Fehlern: Status zurückgeben; speichern, dann mit Status antworten oder
  die Verbindung kappen ("gespeichert, dann 502"); erst nach der Antwort speichern ("spät
  gespeichert"); Rate-Limit bis zu einem Zeitpunkt; Verzögerung.
- Beweist: Wiederholungen nach unsicherer Zustellung erzeugen kein Doppel und keinen Verlust.
- Einsatz: Fake-Bothy für SDKs, Adapter und Import-Werkzeug; Fake-Discord und Fake-CDN für den
  Import. In Go mit `httptest`, in Python mit der Standardbibliothek.

### 4.2 Szenario-Register mit genau einer RESULT-Zeile (V1)

- Aufbau: Szenarien sind registrierte Funktionen. Ein Test startet ein Szenario als Kindprozess
  mit sauberer Umgebung; das Szenario druckt genau eine Zeile `RESULT <json>`. Der Test verlangt
  Exit-Code `0` und genau eine solche Zeile und prüft das JSON. Arbeitsregel: jede
  Verhaltensänderung bekommt ein Szenario, das vorher rot war.
- Beweist: Verhalten im echten Laufzeit-Kontext (fremdes Interpreter-venv, echte Loader) ohne
  Testcode in diesem Kontext.
- Einsatz: Tests des Hermes-Referenz-Adapters und des Dev-Runners.

### 4.3 Geheimnis-Leck-Test (V1, V3)

- Aufbau: Der Fake spiegelt Token, Schlüssel und signierte Query in Fehlerkörpern und
  Fehlermeldungen. Der Test fängt alle Log-Zeilen auf Debug-Stufe, Standardausgabe, Fehlerausgabe
  und Berichte und sucht den Wert und abgeleitete Formen (Basic-Auth-Kodierung, Präfixe).
- Beweist: Kein Geheimnis erscheint in Log, Ausgabe oder Fehlertext, auch im Fehlerpfad.
- Einsatz: Server-Logs, SDKs, Adapter, Import-Werkzeug.

### 4.4 Temp-Ordner-Wächter (V3, V1)

- Aufbau: Ein Test startet alle Testmodule in einem Kindprozess mit frischem, leerem `TMPDIR` und
  verlangt, dass der Ordner danach leer ist. Die Modulliste liest er aus dem Verzeichnis, damit neue
  Module ohne Pflege dabei sind. Ein absichtlich scheiterndes Modul (ein Fehlschlag, Fehler in
  `setUp` und im Test) beweist das Aufräumen im Fehlerpfad.
- Beweist: Tests hinterlassen keine Reste, auch wenn sie scheitern.
- Einsatz: Python-Suiten (SDK, Adapter, Werkzeuge); für Go-Tests derselbe Wächter über `make check`
  mit eigenem `TMPDIR`.

### 4.5 Doku-Zahlen-Wächter (V1, V2)

- Aufbau: Zahlen in README und Entscheidungen stammen aus benannten Konstanten; ein Test vergleicht
  sie und findet nackte Zahlen außerhalb von Code-Spans, Daten und Versionen; Backticks müssen
  paarweise stehen, sonst liest die Prüfung Code als Prosa.
- Beweist: Doku und Code nennen dieselben Grenzen.
- Einsatz: Bothy führt Grenzen nur als Konfigurationswerte mit einer Standard-Tabelle je Kapitel
  und einer Konfigurationsreferenz; die Prüfung ([23-qualitaet.md](spec/23-qualitaet.md)) verlangt,
  dass jeder genannte Schlüssel in der Referenz steht.

### 4.6 Gedankenstrich-Wächter (V1, V2, V3)

- Aufbau: Alle getrackten Textdateien werden auf die Zeichen U+2013 und U+2014 geprüft.
- Beweist: Die Hausregel hält ohne Handarbeit.
- Einsatz: pre-commit und `make check` ([23-qualitaet.md](spec/23-qualitaet.md)).

### 4.7 Trockenlauf-nur-GET-Beweis (V1, V3)

- Aufbau: Die Fakes zählen alle Methoden; erwartet ist genau die Menge `{GET}`. Kein
  Registrierungs- oder Schreib-Endpunkt erscheint im Log. Ein vorher fehlender Zustandsordner wird
  nicht angelegt; ein vorhandener bleibt leer.
- Beweist: Der Trockenlauf schreibt nirgends.
- Einsatz: `bothy import discord` ([22-import-migration.md](spec/22-import-migration.md),
  Abschnitt 16) und jedes andere Werkzeug mit Trockenlauf.

### 4.8 Test gegen das echte Laden des Hermes-Plugins (V1, V3)

- Aufbau: Kindprozess im Python-venv von Hermes Agent mit Wegwerf-`HOME`, Wegwerf-`HERMES_HOME`
  und eigenem `TMPDIR`; das Plugin liegt als Verweis unter `HERMES_HOME/plugins`; die
  Konfiguration schaltet es über den Manifest-Namen ein; Hermes findet es über seine echte
  Plugin-Suche. Vor und nach der Klasse vergleicht der Test `git status --porcelain --ignored` und
  `git diff HEAD` des Hermes-Checkouts.
- Beweist: Das Plugin lädt im echten Framework, und die Tests verändern das Framework nicht.
- Einsatz: Hermes-Referenz-Adapter, gegen die in CI gepinnte Hermes-Version
  ([09-agenten.md](spec/09-agenten.md), [23-qualitaet.md](spec/23-qualitaet.md)).

## 5. Code, der sich zu portieren lohnt

Lizenzhinweis: Die drei Stücke sind eigener Code des Owners aus privaten Vorgänger-Repos ohne
Fremdlizenz; sie nutzen nur die Python-Standardbibliothek. Übernahme heißt Neuschreiben in Go unter
der Lizenz des Produkts (Apache-2.0, DCO, ADR 0003), mit neu geschriebenen Tests nach den alten
Testfällen. Der alte Quelltext wird nicht kopiert. Wo die Stücke liegen, steht nur im Ops-Repo.

### 5.1 Auto-Titel-Platzhalterregel (V3)

- Ziel: Go, Modul `chat`; genutzt beim Anlegen eines Threads durch einen Menschen und beim Import
  (Titelquelle `auto` oder `manual`).
- Regel: leer ist Platzhalter; nur Satzzeichen bis zu einer kleinen festen Länge ist Platzhalter,
  Emoji und Symbole nicht; nach Unicode-NFKC und Groß-/Kleinschreibung-Faltung ist ein Wort aus der
  Platzhalter-Liste ein Platzhalter; einige davon auch mit angehängter Zahl ("Test 2"). Kurze echte
  Namen bleiben echt ("KI", "M365", "C#", "PC 2").
- Anpassung: Die Wortlisten werden Daten je Sprache (Deutsch und Englisch), nicht Code; die feste
  Länge wird eine benannte Konstante.

### 5.2 CDN-Abruf mit Host-Liste (V1)

- Ziel: Go, Import-Werkzeug.
- Regel: nur `GET`; ein Transport je erlaubtem Host; nur `https` (V1 ließ in einer Prüfung noch
  `http` zu, das wird beim Port behoben); keine Weiterleitungen; kein Proxy aus der Umgebung; kein
  Token an den CDN; Lese-Grenze Maximum plus ein Byte und Prüfung von `content-length`; `429` mit
  Wartegrenze; Fehlertexte nur mit Schema, Host und Pfad; der Fehlerkörper des CDN wird nie
  gezeigt.
- Anpassung: Neu lesen der Nachricht bei abgelaufener Signatur
  ([22-import-migration.md](spec/22-import-migration.md), Abschnitt 6.5).

### 5.3 Ruhe-Regel (V1)

- Ziel: Go, Lese-Spiegel des Import-Werkzeugs.
- Regel: Eine Nachricht ist beschäftigt, wenn sie oder ihre Ausgangsnachricht eine
  Arbeits-Markierung trägt; unruhig, wenn ihre letzte Änderung jünger als die Ruhezeit ist;
  kopierbar ist der Anfang bis vor die erste beschäftigte oder unruhige Nachricht; nicht kopierte
  Typen halten nicht an; ein Rückblick auf frühere Nachrichten fängt späte Markierungen.
- Anpassung: Markierungen, Ruhezeit und Rückblick sind Konfigurationswerte
  ([22-import-migration.md](spec/22-import-migration.md), Abschnitt 19).

## 6. Ausdrücklich nicht übernommen

| Konzept | Grund |
|---|---|
| Register für Umbenennen und Verschieben von Gesprächen | Bothy hat feste Thread-IDs (L01). |
| Bearbeitungsfenster mit logischen Nachrichten-IDs | Keine Bearbeitungsfrist für Agenten; die Live-Box ist ein Lauf (L07). |
| Lücken-Modell, Zurückhalten und Hinweise auf verspätete Nachrichten | Commit-geordnete Sequenz und Cursor (L03). |
| Namensfaltung nach Art des fremden Servers | Keine Identität über Namen (L01). |
| Nachfrage nach unsicheren Sendungen per Inhaltsvergleich | Idempotenz-Schlüssel und Herkunftsschlüssel (L02, L18). |
| Kopfzeile "Name und Zeit" in kopierten Nachrichten | Bothy speichert Original-Autor und Original-Zeit; Herkunft ist `origin`. |
| Teilen langer Nachrichten und Verdichten der Live-Box | Große Nachrichtengrenze; Lauf-Dokument statt Textblase. |
| Status-Reaktionen als Zeichen | `trigger_state` mit Icons; Status ist keine Reaktion. |
| Datei-Eingang mit Ordnern für wartend, in Arbeit und gesehen | Ereignis-Log, Verbraucher-Cursor und `ack`. |
| Abfangen von Plattform-Slash-Befehlen im Plugin | Befehle sind Server-Objekte (`commands`). |
| Übergabekanal für neue Chats | `thread.create` (L24). |
| Eigene Discord-REST-Schicht für Antworten | Bothy schreibt Discord nie; Antworten laufen über die eigene API. |
| Kopie der Konvertierung aus einem Spiegel-Werkzeug | Der Import wandelt Discord-Formatierung selbst um (22, Abschnitt 6.3). |

## 7. Third-party data

| Data | Where | Source and licence |
|---|---|---|
| Truncated SHA-256 values of common passwords (`common_passwords.txt`, M1-39) | `services/server/internal/identity/adapters/passwordpolicy` | SecLists `Passwords/Common-Credentials/10k-most-common.txt` at commit `3a5f62f0e18a1b8a36b25399aff0077f3706e5af`, MIT licence, Copyright (c) 2018 Daniel Miessler (https://github.com/danielmiessler/SecLists). The repository holds only the first 16 hex characters of the SHA-256 of each normalised entry, no word list; source, checksum and generation command are in the adapter README. |
