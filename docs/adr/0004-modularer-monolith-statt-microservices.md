---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team Architektur
informed: alle Planungsteams
---

# Modularer Monolith statt Microservices

## Context and Problem Statement

Bothy ist ein selbst gehosteter Team-Chat, in dem Agenten Teilnehmer sind. Die erste Instanz läuft
auf einem einzelnen Rechner, spätere Instanzen in einer Cloud. Jede Schreibanfrage muss ihre
Änderung und ihr Ereignis atomar speichern (B8), jede Zustellung prüft Rechte (B7), und eine Person
soll die Instanz betreiben können. Welche Grundform hat der Server?

## Decision Drivers

* Änderung und Ereignis in einer Transaktion (commit-geordnetes Ereignis-Log, ADR 0009).
* Betrieb durch eine Person: wenige Teile, eine Sicherung, ein Upgrade-Schritt.
* Niedrige Latenz zwischen Schreiben, Rechteprüfung und Zustellung.
* Späteres Wachstum ohne Neuschreiben (mehr Verbindungen, mehr Jobs).
* Klare Grenzen, damit viele Bau-Sitzungen parallel in eigenen Worktrees arbeiten können.

## Considered Options

* Modularer Monolith: ein Binary, strikte Module, Rollen `api`, `worker`, `all`
* Microservices je Domäne mit eigener Datenbank und Nachrichten-Broker
* Monolith ohne geprüfte Modulgrenzen
* Bestehenden Chat-Server erweitern (z. B. einen Matrix-Homeserver, Mattermost oder Zulip)

## Decision Outcome

Chosen option: "Modularer Monolith", because er Änderung und Ereignis in einer
PostgreSQL-Transaktion hält, nur einen Server-Container braucht und über Rollen trotzdem getrennt
skaliert. Die Module sind die Grenzen für parallele Arbeit und für eine spätere Abspaltung
(02-architektur.md, Abschnitt 16).

### Consequences

* Good, because eine Bereitstellung, eine Datenbank und eine Sicherung reichen.
* Good, because Rollen (`api`, `worker`) getrennt skalieren, ohne den Code zu teilen.
* Good, because Module mit Ports und einem Abhängigkeitsgraphen ohne Zyklen später hinter eine
  Prozessgrenze wandern können.
* Bad, because ein Speicherleck oder Absturz alle Module eines Prozesses trifft. Gegenmittel:
  Trennung der Rollen, Recover je Anfrage, Job und Verbindung (02, 14.5).
* Bad, because der globale Zähler `event_seq` ein gemeinsamer Engpass aller Module ist. Gegenmittel:
  Lasttest-Gate auf `perf.event_commit_min_per_s` (02, Abschnitt 17).
* Bad, because Modulgrenzen nur halten, wenn ein Werkzeug sie prüft (ADR 0008).

### Confirmation

* `go-arch-lint check` im lokalen Gate (ADR 0008, 0038) hält die Modulgrenzen.
* Ein Rollentest startet `bothy serve` mit jeder Rolle und prüft: `api` führt keine Jobs aus,
  `worker` öffnet keinen öffentlichen Listener, `all` macht beides.
* Das Release baut genau ein Server-Image (18-release.md).

## Pros and Cons of the Options

### Modularer Monolith

* Good, because Transaktionen über Module hinweg ohne verteilte Transaktion möglich sind.
* Good, because In-Prozess-Fan-out ohne Broker auskommt (R-Stack).
* Neutral, because die Rollen dasselbe Image nutzen; der Betreiber wählt nur einen Parameter.
* Bad, because Disziplin bei Abhängigkeiten nötig ist (Werkzeug statt Konvention).

### Microservices je Domäne

* Good, because Teile unabhängig deploybar und skalierbar sind.
* Bad, because Änderung und Ereignis über Dienste nur mit Outbox und Broker konsistent bleiben;
  das globale, commit-geordnete Log aus B8 wäre ein verteilter Engpass. [abgeleitet]
* Bad, because jede Rechteprüfung über das Netz ginge oder Rechte repliziert werden müssten (B7
  verlangt eine Prüfstelle). [abgeleitet]
* Bad, because der Betrieb auf einem Rechner viele Container, Netzwerk und Sicherungen braucht.

### Monolith ohne geprüfte Modulgrenzen

* Good, because am Anfang am schnellsten.
* Bad, because parallele Bau-Sitzungen ohne Grenzen in dieselben Pakete schreiben und Tabellen
  quer lesen; eine spätere Abspaltung wird teuer. [abgeleitet]

### Bestehenden Chat-Server erweitern

* Good, because Chat-Grundfunktionen schon existieren.
* Bad, because Datenmodell, Rechte und Sync des Fremdsystems gelten. Die Kernobjekte dieses Produkts
  (Lauf und Live-Box, Aufruf-Marke, Invariante I1, ein Bediener je Kanal) wären Erweiterungen
  außerhalb des Kerns, und Rechte müssten an zwei Stellen stimmen. [abgeleitet]
* Bad, because die Apps (Flutter, B5) einen fremden Client-Vertrag sprechen müssten.

## More Information

* Planungsgrundlage B1, B3, B6, B7, B8, B12.
* [02-architektur.md](../spec/02-architektur.md), Abschnitte 3, 4 und 16.
* Verwandt: ADR 0005 (Go), ADR 0008 (Modulregeln), ADR 0009 (Ereignis-Log).
