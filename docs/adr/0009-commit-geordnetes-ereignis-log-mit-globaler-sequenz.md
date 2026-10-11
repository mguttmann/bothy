---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team Architektur
informed: alle Planungsteams
---

# Commit-geordnetes Ereignis-Log mit globaler Sequenz

## Context and Problem Statement

Clients und Agenten-Adapter holen nach einer Pause "alles ab Nummer X" nach (B8). Ein Leser darf
dabei nie ein Ereignis überspringen. Eine PostgreSQL-Sequenz (`bigserial`) vergibt Nummern beim
Aufruf, nicht beim Commit, und hinterlässt bei Rollback Lücken; ein Leser kann eine Lücke nicht von
einer noch offenen Transaktion unterscheiden (04, 3.1). Wie entsteht eine Nummer, deren Reihenfolge
der Sichtbarkeit entspricht?

## Decision Drivers

* Kein Ereignis geht beim Nachholen verloren.
* Einfacher Leser: `WHERE seq > cursor ORDER BY seq`.
* Lückenlose Nummern, damit ein Client fehlende Daten erkennt.
* Änderung und Ereignis atomar in einer Transaktion.
* Kein zusätzliches System im Betrieb.

## Considered Options

* Zähler-Zeile mit Zeilensperre in derselben Transaktion (Muster aus 04, 3.2)
* `bigserial` plus Sichtbarkeitsgrenze über Transaktions-IDs (`pg_current_xact_id`, `pg_snapshot_xmin`)
* Logical Decoding (Änderungsdatenstrom aus dem WAL)
* Externer Broker mit Outbox-Tabelle

## Decision Outcome

Chosen option: "Zähler-Zeile mit Zeilensperre", because es lückenlose Nummern in Commit-Reihenfolge
liefert, nur PostgreSQL braucht und denselben Mechanismus für `audit_seq` und `permission_version`
nutzt (03, R-DM-05). Der Zähler-Schritt steht immer zuletzt vor dem Commit; die Unit of Work setzt das
durch (02, Abschnitt 8).

### Consequences

* Good, because ein Leser nach jedem Commit alle kleineren Nummern sieht; Rollback hinterlässt keine
  Lücke (04, 3.2).
* Good, because der Cursor eines Clients eine einzige Zahl ist.
* Bad, because alle Transaktionen mit Ereignissen am Ende serialisiert sind; der Durchsatz ist
  begrenzt. Gegenmittel: Zähler-Schritt zuletzt, keine externen Aufrufe in Transaktionen,
  Lasttest-Gate `perf.event_commit_min_per_s`.
* Bad, because schreibende Transaktionen mit Ereignissen in `READ COMMITTED` laufen müssen.
* Bad, because `NOTIFY` beim Commit zusätzlich eine instanzweite Sperre nimmt [belegt, Q3]; sie fällt
  in denselben serialisierten Abschnitt.

### Confirmation

* Nebenläufigkeitstest: viele parallele Schreiber mit zufälligen Verzögerungen vor dem Commit und
  zufälligen Rollbacks; ein mitlesender Leser sieht nur lückenlos steigende Nummern und verpasst
  keine.
* Lasttest `perf/event_commit` hält `perf.event_commit_min_per_s` (02, Abschnitt 17).
* Die Unit of Work setzt die Isolationsstufe ausdrücklich; ein Test prüft, dass Schreib-Transaktionen
  mit Ereignissen nie in `REPEATABLE READ` oder `SERIALIZABLE` laufen.

## Pros and Cons of the Options

### Zähler-Zeile mit Zeilensperre

* Good, because Sichtbarkeit und Nummernfolge zusammenfallen: PostgreSQL macht eine Transaktion
  sichtbar, bevor es ihre Sperren freigibt (04, 3.2).
* Bad, because ein globaler Serialisierungspunkt entsteht.

### `bigserial` plus Sichtbarkeitsgrenze

* Good, because keine Sperre die Commits serialisiert.
* Good, because PostgreSQL 18 die nötigen Funktionen hat: `pg_current_xact_id()` liefert die
  Transaktions-ID, `pg_snapshot_xmin()` die kleinste noch offene. [belegt, Q1]
* Bad, because Leser nur bis zur ältesten offenen Transaktion der ganzen Datenbank lesen dürfen;
  eine lange Transaktion irgendwo (Wartung, Job) hält jede Zustellung auf. [abgeleitet]
* Bad, because der Cursor ein Paar aus Transaktions-ID und Nummer wird und Nummern Lücken haben; 04
  verlangt lückenlose Nummern. [abgeleitet]

### Logical Decoding

* Good, because Änderungen ohne Eingriff in die Schreibpfade gelesen werden.
* Bad, because Replikations-Slots WAL und Katalogzeilen festhalten, auch ohne verbundenen Leser;
  im Extremfall droht eine Abschaltung wegen Transaktions-ID-Überlauf. [belegt, Q2]
* Bad, because nach einem Absturz Änderungen doppelt kommen können. [belegt, Q2]
* Bad, because Rechte-Filter und Nutzlasten trotzdem eine eigene Ereignis-Tabelle brauchen.
  [abgeleitet]

### Externer Broker mit Outbox-Tabelle

* Good, because Broker für hohen Durchsatz gebaut sind.
* Bad, because die Outbox die Reihenfolge-Frage nicht löst, sondern nur verschiebt; ein zweites
  System kommt in Betrieb und Sicherung dazu. [abgeleitet]

## More Information

Quellen, abgerufen 2026-10-10:

* Q1 https://www.postgresql.org/docs/18/functions-info.html (Transaktions-ID- und Snapshot-Funktionen)
* Q2 https://www.postgresql.org/docs/current/logicaldecoding-explanation.html
* Q3 https://recall.ai/blog/postgres-listen-notify-does-not-scale

Planungsgrundlage B8. [04-sync-protokoll.md](../spec/04-sync-protokoll.md), Abschnitt 3;
[03-datenmodell.md](../spec/03-datenmodell.md), Abschnitt 6; [02-architektur.md](../spec/02-architektur.md),
Abschnitte 8 und 11.
