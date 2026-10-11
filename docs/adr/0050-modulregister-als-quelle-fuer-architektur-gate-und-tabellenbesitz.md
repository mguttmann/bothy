---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team A (Architektur)
informed: alle Planungsteams
---

# Modulregister als Quelle für Architektur-Gate und Tabellenbesitz

## Context and Problem Statement

Der Server ist ein modularer Monolith mit hexagonalen Modulen (ADR 0004, ADR 0008). Jede Tabelle
gehört genau einem Modul, nur dieses Modul liest und schreibt sie per SQL
([03-datenmodell.md](../spec/03-datenmodell.md), Grundsatz 12), und jede Aktion gehört genau einem
Modul ([02-architektur.md](../spec/02-architektur.md), Abschnitt 4.1, Regel 6). ADR 0008 hat
go-arch-lint als Werkzeug für Paket-Abhängigkeiten gewählt. Offen ist: Welche Datei hält die
Fakten über Module, welche Prüfungen entstehen daraus, und wie wird Tabellenbesitz durchgesetzt,
den go-arch-lint nicht sieht?

## Decision Drivers

- Eine Quelle für Modulgrenzen: Tabellen, Aktionen, erlaubte Abhängigkeiten, Fremdbibliotheken.
- Tabellenbesitz wird geprüft, nicht nur beschrieben (Grundsatz 12, B7: eine Wahrheit für Rechte).
- Neue Pakete, Tabellen und Aktionen fallen auf, wenn sie keinem Modul zugeordnet sind.
- Die Tabellen in 02 (Abschnitte 4.2, 4.4, 4.6) dürfen nicht vom Code abweichen.
- Viele Bau-Sitzungen arbeiten parallel; Verstöße müssen im lokalen Gate auffallen (ADR 0038).
- Ein Unit of Work umfasst mehrere Module in einer Transaktion (02, Abschnitt 8).

## Considered Options

- Register `services/server/architecture/modules.yaml` als einzige Quelle; Konfiguration von
  go-arch-lint und alle weiteren Prüfungen werden daraus erzeugt
- Handgeschriebene go-arch-lint-Konfiguration plus getrennte Listen in der Doku
- Tabellenbesitz in PostgreSQL erzwingen: ein Schema und eine Datenbankrolle je Modul
- Deklarationen im Go-Code je Modul, von einem Werkzeug gelesen

## Decision Outcome

Gewählte Option: "Register `modules.yaml` als einzige Quelle", weil es alle Modul-Fakten an
einer lesbaren Stelle hält und daraus jede Prüfung im lokalen Gate entsteht. Format und Beispiel:
[02-architektur.md](../spec/02-architektur.md), Abschnitt 7.1.

1. Je Modul nennt das Register `tables`, `actions`, `events`, `ephemeral_events`, `ws_ops`,
   `depends_on` und `adapter_vendors`; dazu die Kernel-Pakete (`kernel.core`, `kernel.ports`), je
   Plattform-Paket einen Eintrag mit `depends_on` und `vendors`, die Einträge `wire`, `cmd` und
   `api` mit ihren Vendor-Gruppen und die Liste `vendors` mit den Importpfaden je
   Fremdbibliothek. Technische Tabellen (River im Schema `jobs.schema`, Versionstabelle von goose; 03
   Abschnitt 4.13) gehören keinem Modul und stehen nicht unter `modules`; eine Abfrage eines
   Moduls, die sie nennt, ist ein Fund der Prüfung des Tabellenbesitzes.
2. Daraus entstehen:
    - `.go-arch-lint.yml` mit vier Komponenten je Modul (Port, `domain`, `app`, `adapters`, ADR
      0008), einer Komponente je Plattform-Paket und den Komponenten `wire`, `cmd`, `api`; nur so
      lässt sich eine Fremdbibliothek auf genau ein Plattform-Paket begrenzen (02 Abschnitt 7.2);
    - die Prüfung des Tabellenbesitzes: Jede SQL-Datei unter
      `internal/<modul>/adapters/postgres/queries/` nennt nur Tabellen dieses Moduls;
    - die Vollständigkeit: Jede Tabelle der Migrationen und aus 03 steht genau einmal im Register,
      jede Aktion des Katalogs genau einmal (Mengenvergleich mit `docs/spec/actions.yaml`, ADR 0049);
    - die Routen-Prüfung: Der Handler einer Operation liegt im Modul, dem ihre Aktion gehört;
    - die Richtung der In-Tx-Abonnements: Ein Modul abonniert nur Ereignisse von Modulen, von denen
      es abhängt (Golden-Datei in `internal/wire`, 02 Abschnitt 8.3);
    - der Doku-Abgleich: Die Tabellen 4.2, 4.4, 4.6 und 4.7 in 02 entsprechen dem Register.
3. Ein neues Paket ohne Komponente bricht go-arch-lint ab ("not attached to any component"); das
   ist gewollt (02, Abschnitt 7.2).

### Consequences

- Gut, weil eine Änderung der Modulgrenzen genau eine Datei ändert und alle Prüfungen folgen.
- Gut, weil Tabellenbesitz vor dem Merge auffällt, mit Datei und Tabelle, statt im Review.
- Gut, weil die Vollständigkeit gegen die Migrationen geprüft wird, also gegen das echte Schema.
- Schlecht, weil die Prüfung des Tabellenbesitzes SQL-Dateien liest und kein PostgreSQL ist: Eine
  Tabelle, die nur über eine Sicht oder eine Funktion erreicht wird, sieht sie nicht. Gegenmittel:
  Sichten und Funktionen stehen im Register wie Tabellen.
- Schlecht, weil Generator und Prüfungen gepflegt werden müssen; Rückfall für go-arch-lint ist die
  Erzeugung von depguard-Regeln aus demselben Register (ADR 0008).
- Neutral, weil sqlc pro Modul arbeitet (`sqlc.yaml`, ein Eintrag je Modul) und das Register die
  Liste der Module dafür liefern kann.

### Confirmation

- `make arch` (go-arch-lint, Exitcode 0) und `make codegen-check` (erzeugte `.go-arch-lint.yml`
  gleich dem Generator-Ergebnis) im Ziel `gates`.
- `go test ./tools/archgen/...` mit Fixture-Modulen: Domain importiert einen Treiber, App importiert
  einen fremden Port, App importiert fremde App, Paket ohne Komponente; alle vier scheitern
  erwartet ([02-architektur.md](../spec/02-architektur.md), Abschnitt 7.2). Die Form mit
  Plattform-Komponenten belegt M1-05 mit denselben vier Verstößen.
- Fixture für den Tabellenbesitz: Eine Abfrage in Modul A nennt eine Tabelle von Modul B; der
  Check scheitert mit Datei und Tabelle.
- Vollständigkeits-Check gegen Migrationen, 03 und `actions.yaml`; Doku-Abgleich mit 02.
- Bau-Ticket M1-05 in [PLAN.md](../PLAN.md).

## Pros and Cons of the Options

### Register als einzige Quelle

- Gut, weil Menschen und Werkzeuge (Go-Generator, Python-Doku-Checks) dieselbe Datei lesen.
- Schlecht, weil die erzeugte Konfiguration groß ist; sie wird nie von Hand bearbeitet.

### Handgeschriebene Konfiguration und getrennte Listen

- Gut, weil kein Generator nötig ist.
- Schlecht, weil drei Listen (Werkzeug, 02, 03) auseinanderlaufen und Tabellenbesitz gar nicht
  geprüft wird.

### Schema und Rolle je Modul in PostgreSQL

- Gut, weil die Datenbank einen fremden Zugriff zur Laufzeit ablehnt.
- Schlecht, weil ein Unit of Work mehrere Module in einer Transaktion auf einer Verbindung ausführt
  (02, Abschnitt 8.1); Rollen je Modul bräuchten `SET ROLE` vor jeder Anweisung oder getrennte
  Verbindungen und damit getrennte Transaktionen [abgeleitet].
- Schlecht, weil die Phasen-Hooks (Rechte-Version, Ereignis-Log, Audit) in jeder Transaktion
  fremde Tabellen schreiben; sie bräuchten Ausnahmen, die die Regel wieder aufweichen.

### Deklarationen im Go-Code

- Gut, weil sie neben dem Code stehen.
- Schlecht, weil die Fakten über viele Dateien verteilt sind und Doku-Werkzeuge Go-Quelltext
  auswerten müssten.

## More Information

- [02-architektur.md](../spec/02-architektur.md), Abschnitte 4, 6 und 7; Entscheidung R-AR-19.
- Verwandt: ADR 0004 (modularer Monolith), ADR 0007 (sqlc und Migrationen), ADR 0008 (hexagonale
  Module, go-arch-lint), ADR 0049 (Aktionskatalog).
