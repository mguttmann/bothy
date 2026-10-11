---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team Architektur
informed: alle Planungsteams
---

# Hexagonale Module und geprüfte Abhängigkeitsregeln

## Context and Problem Statement

Der Server ist ein modularer Monolith (ADR 0004). Viele Bau-Sitzungen arbeiten parallel. Ohne
durchgesetzte Grenzen liest ein Modul fremde Tabellen, Domänencode importiert Datenbanktreiber, und
Module hängen im Kreis voneinander ab. Wie sind Module aufgebaut, und wie werden die Regeln
automatisch gehalten?

## Decision Drivers

* Fachlogik ohne Infrastruktur testbar (B1: hexagonal).
* Treiber austauschbar (Objektspeicher, Identitätsanbieter, Push, Datenbank-Details).
* Ein Abhängigkeitsgraph ohne Zyklen, damit Module abspaltbar bleiben.
* Verstöße fallen im lokalen Gate mit Datei und Zeile auf, nicht im Review.
* Eine Quelle für Modulgrenzen, die Code, Doku und Prüfungen speist.

## Considered Options

* go-arch-lint mit einer aus `modules.yaml` erzeugten Konfiguration
* depguard (in golangci-lint) mit Regeln je Dateimuster
* Nur verschachtelte Go-`internal`-Verzeichnisse
* Eigener Test mit `golang.org/x/tools/go/packages`

## Decision Outcome

Chosen option: "go-arch-lint mit einer aus `modules.yaml` erzeugten Konfiguration", because es
Komponenten, erlaubte Abhängigkeiten und zugelassene Fremdbibliotheken je Komponente direkt
ausdrückt und in einem Probelauf alle vier Arten von Verstößen gemeldet hat.

Aufbau je Modul (02-architektur.md, Abschnitt 5): Port im Wurzelpaket, `domain`, `app`,
`adapters/<art>`. `services/server/architecture/modules.yaml` nennt je Modul Tabellen, Aktionen,
erlaubte Abhängigkeiten und Fremdbibliotheken; ein Generator erzeugt `.go-arch-lint.yml` daraus.

### Consequences

* Good, because Verstöße mit Datei und Zeile im Gate erscheinen.
* Good, because neue Pakete ohne Eintrag im Register auffallen ("not attached to any component").
* Good, because dieselbe Quelle die Prüfungen für Tabellenbesitz, Aktionen und Doku speist
  (02, 7.3).
* Bad, because die erzeugte Konfiguration groß ist; sie wird nie von Hand bearbeitet.
* Bad, because ein weiteres Werkzeug gepflegt werden muss. Fallback: Der Generator kann
  depguard-Regeln aus demselben Register erzeugen.

### Confirmation

* `go-arch-lint check` im lokalen Gate (ADR 0038); Exitcode ungleich 0 bricht ab.
* Gate prüft, dass `.go-arch-lint.yml` dem Generator-Ergebnis aus `modules.yaml` entspricht.
* Selbsttest des Gates: Ein Gerüst mit eingebauten Verstößen (Fremdbibliothek in `domain`, Port
  eines nicht erlaubten Moduls, `app` eines fremden Moduls, Paket ohne Komponente) muss fehlschlagen.
* Probelauf am 2026-10-10 (Go 1.27.2, go-arch-lint v1.19.0) an einem Gerüst mit allen Modulen:
  sauber "No warnings found"; mit den vier Verstößen vier Meldungen, Exitcode 1.

## Pros and Cons of the Options

### go-arch-lint mit erzeugter Konfiguration

* Good, because die Konfiguration (Version 3) Komponenten über Pfadmuster, `mayDependOn`,
  `canUse` für Fremdbibliotheken und `commonComponents` kennt. [belegt, Q1]
* Good, because das Projekt 2026 regelmäßig veröffentlicht (v1.17.0 am 2026-08-05, v1.18.0 am
  2026-08-21, v1.19.0 am 2026-09-07) und MIT-lizenziert ist. [belegt, Q2]
* Bad, because ein leeres `mayDependOn` ein Fehler ist; der Generator lässt solche Einträge weg
  (im Probelauf festgestellt).

### depguard mit Regeln je Dateimuster

* Good, because depguard Teil von golangci-lint ist und Regeln mit Dateimustern, Erlaubt- und
  Verbots-Listen sowie Listenmodi (`strict`, `lax`) kennt. [belegt, Q3]
* Bad, because ein Abhängigkeitsgraph zwischen Modulen als Listen je Dateimuster geschrieben werden
  muss; Pakete ohne Regel fallen nicht auf. [abgeleitet]

### Nur verschachtelte Go-`internal`-Verzeichnisse

* Good, because der Compiler die Regel durchsetzt.
* Bad, because `internal` nur "privat für den Elternbaum" ausdrückt, keinen erlaubten Graphen
  zwischen Modulen und keine Regeln für Fremdbibliotheken. [abgeleitet]

### Eigener Test mit go/packages

* Good, because volle Kontrolle.
* Bad, because ein eigenes Werkzeug gebaut und gepflegt werden muss, das go-arch-lint schon ist.

## More Information

Quellen, abgerufen 2026-10-10:

* Q1 https://github.com/fe3dback/go-arch-lint/blob/master/docs/syntax/README.md
* Q2 https://github.com/fe3dback/go-arch-lint/releases und
  https://proxy.golang.org/github.com/fe3dback/go-arch-lint/@latest
* Q3 https://pkg.go.dev/github.com/OpenPeeDeeP/depguard/v2

Planungsgrundlage B1. [02-architektur.md](../spec/02-architektur.md), Abschnitte 4, 5 und 7.
