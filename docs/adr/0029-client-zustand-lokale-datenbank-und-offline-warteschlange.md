---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung, Team Apps und Design
consulted: Team Architektur, Team API
informed: alle Teams
---

# Client-Zustand, lokale Datenbank und Offline-Warteschlange

## Context and Problem Statement

Die Apps müssen offline lesen und schreiben, nach Pausen lückenlos nachholen, Doppel vermeiden und
Live-Box-Patches flüssig zeichnen ([04-sync-protokoll.md](../spec/04-sync-protokoll.md)). Rechte
gelten zur Zustellzeit; ein Entzug muss lokale Daten sofort entfernen. Wie halten die Apps ihren
Zustand, wo speichern sie, und wie warten Schreibungen auf das Netz?

## Decision Drivers

- ein Weg für online, offline und Nachholen
- reaktive Oberfläche, die nur neu zeichnet, was sich ändert
- transaktionale Anwendung von Ereignis-Seiten und Cursor
- verschlüsselte lokale Daten, Migrationen mit Tests
- Testbarkeit ohne Gerät (Fake-Uhr, Fake-Transport)
- Pflege der Pakete und Lizenzen

## Considered Options

Zustand:

1. Riverpod 3 mit Code-Erzeugung
2. BLoC (`flutter_bloc`)
3. Provider mit `ChangeNotifier`

Lokale Datenbank:

1. drift auf SQLite mit SQLite3MultipleCiphers
2. Isar
3. Hive bzw. Key-Value-Speicher
4. sembast (Dokumente auf Dateibasis)

Ausgangsfach:

1. Tabelle in derselben Datenbank, je Thread FIFO
2. Warteschlange im Speicher mit Dateisicherung

## Decision Outcome

Chosen option: "Riverpod 3, drift mit SQLite3MultipleCiphers, Ausgangsfach als Tabelle", weil
drift relationale Abfragen mit reaktiven Strömen, Transaktionen, erzeugten Migrationstests und
Verschlüsselung verbindet und Riverpod diese Ströme ohne Widget-Kontext, testbar und mit
automatischer Freigabe an die Oberfläche bringt. Das Ausgangsfach in derselben Datenbank lässt
Eintrag, Echo und Cursor in einer Transaktion abgleichen.

Belege (abgerufen 2026-10-10): `flutter_riverpod` 3.4.3, MIT (https://pub.dev/packages/flutter_riverpod)
[belegt]; `drift` 2.35.2 für Android, iOS, macOS, Windows, Linux, Web
(https://pub.dev/packages/drift) [belegt]; Migrations-Workflow `make-migrations` mit erzeugten Tests
(https://drift.simonbinder.eu/migrations/) [belegt]; `sqlite3` bündelt SQLite über Build-Hooks und
kann `source: sqlite3mc` (https://github.com/simolus3/sqlite3.dart/blob/main/sqlite3/doc/hook.md)
[belegt].

### Consequences

- Good, because die Oberfläche nur aus der Datenbank liest und alle Netzwege dort zusammenlaufen.
- Good, because Ereignis-Seite, Cursor und Bestätigung des Ausgangsfachs atomar sind.
- Good, because Entzug eine Transaktion ist, die Zeilen, Suchindex und Datei-Cache-Verweise löscht.
- Good, because Migrationen pro Schema-Version erzeugte Tests haben.
- Bad, because Code-Erzeugung (drift, Riverpod) Bauzeit kostet.
- Bad, because SQLite3MultipleCiphers eigene Lizenzbedingungen hat; das Lizenz-Gate muss es kennen.
- Neutral, because die Web-Konsole keine lokale Datenbank nutzt (ADR 0028).

### Confirmation

- Widget-Tests prüfen, dass Screens nur über Provider lesen (kein direkter API-Aufruf aus Widgets;
  Lint-Regel im Gate).
- Unit-Tests der `sync_engine`: Doppel, Lücken, `resync_required`, Entzug, Echo des Ausgangsfachs.
- drift-Migrationstests plus Test "Ausgangsfach und Entwürfe überleben jede Migration".
- Integrationstests mit Fake-Server (15-client-architektur.md, Abschnitt 20).

## Pros and Cons of the Options

### Riverpod 3

- Good, because Abhängigkeiten ohne `BuildContext`, `family` je ID, `autoDispose` passend zum Fokus.
- Good, because Overrides in Tests.
- Bad, because Code-Erzeugung und eine eigene Denkweise.

### BLoC

- Good, because klare Ereignis-Zustand-Trennung, verbreitet.
- Bad, because viel Boilerplate für viele kleine, ID-gebundene Ströme (je Thread, je Lauf).

### Provider mit ChangeNotifier

- Good, because einfach.
- Bad, because grob granulare Neuzeichnung und schwache Testbarkeit großer Zustände.

### drift auf SQLite

- Good, because relationale Abfragen (Ungelesen-Zähler, Beitragsliste nach Aktivität), Ströme,
  Transaktionen, FTS5 für Offline-Suche, Verschlüsselung.
- Bad, because SQL-Schema muss gepflegt werden (ein Spiegel des Server-Schemas).

### Isar

- Good, because schnell, einfache Objekte.
- Bad, because die letzte stabile Version auf pub.dev ist 3.1.0+1 vom 2023-04-25 (pub.dev-API, abgerufen
  2026-10-10 [belegt]); keine Relationen im SQL-Sinn.

### Hive bzw. Key-Value

- Good, because sehr einfach; die Fortführung `hive_ce` wird gepflegt (pub.dev-API, abgerufen
  2026-10-10 [belegt]).
- Bad, because keine Abfragen für Zähler und Listen, keine Transaktionen über mehrere Objekte.

### sembast

- Good, because reines Dart, läuft überall.
- Bad, because ganze Datei im Speicher; ungeeignet für große Verläufe.

### Ausgangsfach als Tabelle

- Good, because atomar mit Echo und Cursor; übersteht Abstürze.

### Ausgangsfach im Speicher

- Bad, because Abstürze verlieren Einträge oder brauchen eine zweite, nicht transaktionale Sicherung.

## More Information

- Schema-Spiegel, Verschlüsselung, Aufräumen: [15-client-architektur.md](../spec/15-client-architektur.md), Abschnitt 6.
- Sync-Engine und Zustände: ebd., Abschnitt 7.
- Normative Regeln für Ausgangsfach und Abgleich: [04-sync-protokoll.md](../spec/04-sync-protokoll.md), Abschnitte 9, 13, 14.
