---
status: template
date: 2026-10-10
decision-makers: Maintainer
---

# Vorlage (MADR 4)

Diese Datei ist die Vorlage für neue Architekturentscheidungen. Kopie anlegen als
`docs/adr/NNNN-titel-in-kebab-case.md` mit der nächsten freien Nummer, Text auf Deutsch,
Überschriften wie unten (MADR 4, https://adr.github.io/madr/, abgerufen 2026-10-10).
Danach `make adr-index`. Ablauf: [ADR 0001](0001-adrs-im-madr-format-decisions-als-ruling-log.md).

Kopfdaten: `status` ist `proposed`, `accepted`, `rejected`, `deprecated` oder
`superseded by ADR-NNNN`; `date` ist der Tag der letzten Änderung; `decision-makers`
nennt Rollen, keine Personennamen.

## Context and Problem Statement

Welches Problem lösen wir, in zwei bis vier Sätzen? Was zwingt jetzt zu einer
Entscheidung? Verweise auf Kapitel und Belege (URL, abgerufen am, Marker
[belegt]/[abgeleitet]/[vermutet]).

## Decision Drivers

* Treiber 1, zum Beispiel eine Anforderung oder Invariante
* Treiber 2

## Considered Options

* Option 1
* Option 2 (mindestens zwei echte Optionen)

## Decision Outcome

Gewählt: "Option 1", weil sie Treiber 1 und 2 als einzige erfüllt.

### Consequences

* Gut, weil ...
* Schlecht, weil ... (und wie wir damit umgehen)

### Confirmation

Welcher Test, welches Gate oder welche Prüfung hält die Entscheidung im Repository
wahr? Ohne Confirmation ist eine ADR nicht fertig.

## Pros and Cons of the Options

### Option 1

* Gut, weil ...
* Neutral, weil ...
* Schlecht, weil ...

### Option 2

* Gut, weil ...
* Schlecht, weil ...

## More Information

Folgefragen, Zeitpunkt der Überprüfung, verwandte ADRs.
