---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team Architektur
informed: alle Planungsteams
---

# PostgreSQL ab Tag 1 statt SQLite

## Context and Problem Statement

Die erste Instanz läuft auf einem Rechner; dort wäre eine eingebettete Datenbank am einfachsten.
Das Produkt soll aber auch in Clouds laufen, braucht Volltextsuche mit Rechtefilter, ein
commit-geordnetes Ereignis-Log mit Zeilensperren (ADR 0009), Benachrichtigungen an andere Prozesse
und Hintergrundjobs. Welche Datenbank nutzt der Server, und eine oder zwei?

## Decision Drivers

* Ein Datenbankpfad im Code und in den Tests.
* Gleichzeitige Schreiber (Nachrichten, Lauf-Patches, Jobs, Heartbeats).
* Volltextsuche, Partitionierung, Zeilensperren und `LISTEN/NOTIFY`.
* Portabilität: verwaltete Dienste in jeder Cloud.
* Kleine Installation auf einem Rechner muss trotzdem einfach bleiben.

## Considered Options

* Nur PostgreSQL
* Nur SQLite
* Beide hinter einer Repository-Schicht

## Decision Outcome

Chosen option: "Nur PostgreSQL", because das Ereignis-Log, die Rechtefilter und die Suche Funktionen
brauchen, die nur PostgreSQL in einer Form bietet, und weil zwei Datenbankpfade jede Abfrage und
jeden Test verdoppeln. Zielversion ist PostgreSQL 18; PostgreSQL 19 erst nach GA und eigenem Test.

### Consequences

* Good, because ein SQL-Dialekt, ein Migrationspfad, eine Testumgebung.
* Good, because `LISTEN/NOTIFY` als Wecksignal ohne Broker dient (02, Abschnitt 11).
* Good, because jede Cloud verwaltetes PostgreSQL anbietet.
* Bad, because auch die kleinste Installation einen PostgreSQL-Container braucht. Gegenmittel:
  Compose als Ein-Befehl-Installation (ADR 0012); PostgreSQL ist im Compose-Netz nicht nach außen
  veröffentlicht, der Port ist Konfiguration.
* Bad, because Sicherung und Upgrade der Datenbank Aufgaben des Betreibers sind (16-betrieb.md).

### Confirmation

* Integrationstests laufen immer gegen ein echtes PostgreSQL 18 (testcontainers-go), nie gegen
  einen Ersatz.
* Die Liste zugelassener Fremdbibliotheken (`modules.yaml`, ADR 0008) enthält keinen SQLite-Treiber.
* Die Compose-Datei des Produkts veröffentlicht keinen PostgreSQL-Port auf dem Host (16-betrieb.md).

## Pros and Cons of the Options

### Nur PostgreSQL

* Good, because PostgreSQL 18 aktuell ist und bis 2030-11-14 gepflegt wird. [belegt, Q1]
* Good, because `NOTIFY` erst beim Commit zustellt und der Commit-Reihenfolge folgt. [belegt, Q2]
* Good, because `websearch_to_tsquery` rohe Eingaben ohne Syntaxfehler annimmt. [belegt, Q3]
* Bad, because der Betrieb einen Datenbankdienst mehr braucht als eine Datei.

### Nur SQLite

* Good, because keine eigene Datenbank-Installation nötig ist.
* Bad, because SQLite beliebig viele Leser, aber zu jedem Zeitpunkt nur einen Schreiber erlaubt.
  [belegt, Q4]
* Bad, because für gemeinsam genutzte Datenbanken über das Netz die Doku eine Client-Server-Datenbank
  empfiehlt; mehrere Server-Knoten wären damit ausgeschlossen. [belegt, Q4]
* Bad, because ein zweiter Prozess (worker) kein `LISTEN/NOTIFY` hätte. [abgeleitet]

### Beide hinter einer Repository-Schicht

* Good, because kleine Installationen ohne PostgreSQL laufen könnten.
* Neutral, because River beide Datenbanken unterstützt. [belegt, Q5]
* Bad, because jede Abfrage, jede Migration und jeder Test doppelt existieren und das Ereignis-Log
  zwei Sperr-Modelle bräuchte. [abgeleitet]

## More Information

Quellen, abgerufen 2026-10-10:

* Q1 https://www.postgresql.org/support/versioning/
* Q2 https://www.postgresql.org/docs/current/sql-notify.html
* Q3 https://www.postgresql.org/docs/current/textsearch-controls.html
* Q4 https://sqlite.org/whentouse.html
* Q5 https://github.com/riverqueue/river

Planungsgrundlage B3. [02-architektur.md](../spec/02-architektur.md), Abschnitte 8 und 11.
Verwandt: ADR 0007, ADR 0009.
