---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team Architektur
informed: alle Planungsteams
---

# Typisierter Datenbankzugriff (sqlc) und Migrationswerkzeug

## Context and Problem Statement

Das Datenmodell (03-datenmodell.md) nutzt Trigger, Partitionierung, generierte Spalten,
Ausschluss-Constraints, Teilindexe und getrennte Datenbankrollen. Viele Bau-Sitzungen schreiben
parallel Abfragen und Migrationen in eigenen Worktrees. Wie greift der Server typisiert auf
PostgreSQL zu, und mit welchem Werkzeug laufen Migrationen?

## Decision Drivers

* SQL bleibt sichtbar und prüfbar (Rechtefilter, Sperrreihenfolge, Zähler-Muster aus 04).
* Typfehler fallen beim Bauen auf, nicht zur Laufzeit.
* Migrationen in reinem SQL mit allen PostgreSQL-Funktionen, ohne proprietäre Teile.
* Parallele Branches dürfen sich bei Migrationsnummern nicht blockieren.
* Mehrere api-Knoten müssen während eines Upgrades mit altem und neuem Schema laufen.

## Considered Options

* sqlc mit pgx v5 und goose
* sqlc mit pgx v5 und golang-migrate
* sqlc mit pgx v5 und Atlas
* ORM mit eigener Migration (z. B. ent oder GORM)

## Decision Outcome

Chosen option: "sqlc mit pgx v5 und goose", because sqlc typisierten Go-Code aus reinem SQL erzeugt
und goose reine SQL-Migrationen mit Einbettung ins Binary und einem dokumentierten Weg für parallele
Entwicklung bietet.

Regeln:

1. pgx v5 mit Pool; sqlc mit `sql_package: pgx/v5`; ein `sql`-Eintrag je Modul in
   `services/server/sqlc.yaml`, Abfragen unter `internal/<modul>/adapters/postgres/queries/`.
2. sqlc liest das Schema aus `migrations/`; es versteht goose-Dateien und ignoriert Down-Teile.
   [belegt, Q3]
3. Migrationen sind eingebettet und laufen nur über `bothy migrate` (02, R-AR-16). Anweisungen, die
   keine Transaktion vertragen (z. B. `CREATE INDEX CONCURRENTLY`), stehen in eigenen Dateien mit
   `-- +goose NO TRANSACTION`. [belegt, Q4]
4. Versionierung mit fortlaufenden Nummern überall, auch auf Branches: Ein Branch nimmt die
   nächste freie Nummer des Ausgangsbranchs (`NNNNN_<name>.sql`). Hat ein anderes Ticket die
   Nummer inzwischen belegt, nummeriert der später gemergte Branch seine eigenen neuen Migrationen
   vor seinem letzten `make check` um. Das Gate `migrations-lint` lehnt Zeitstempel-Versionen in
   jedem Lauf ab. Produktion nutzt nie `-allow-missing`. Grund: Der lokale Merge folgt erst auf ein
   grünes `make check` des Branchs; das Gate muss darum schon auf dem Branch den Stand des
   Ausgangsbranchs halten. Das hybride Verfahren von goose (Zeitstempel auf Branches, `goose fix`
   beim Merge) [belegt, Q4] passt dazu nicht, weil ein Branch mit Zeitstempel nie grün wäre.
5. Eine Migration auf dem Ausgangsbranch ist unveränderlich: Die Prüfsummen-Datei
   `<modul>/migrations.sha256` (Format von `shasum -a 256`, außerhalb von `migrations/`) schreibt
   nur `make migrations-lock`, das nie eine bestehende Zeile ändert; das Gate vergleicht.
6. Erweitern und Zusammenziehen: Eine Release-Version darf nur additive Änderungen voraussetzen;
   Entfernen folgt frühestens eine Release-Version später. Der Server startet mit jeder Schema-Version
   ab der kleinsten, die er braucht (02, 15.1).
7. Down-Migrationen nur für die Entwicklung; Rücknahme in Produktion ist eine neue Migration.
8. River-Tabellen migriert `rivermigrate` im selben Befehl, im eigenen Schema `jobs.schema`.

### Consequences

* Good, because SQL im Review lesbar bleibt und Rechtefilter sichtbar sind.
* Good, because Schemaänderungen ohne Codegenerierung Bau-Fehler ergeben.
* Good, because jeder Branch schon vor dem Merge genau den Zustand hat, den der Ausgangsbranch
  danach hat, und das Gate das in jedem Lauf prüft.
* Bad, because parallele Branches um dieselbe Nummer konkurrieren; der später gemergte Branch
  nummeriert um. Das Gate fängt doppelte Nummern und unveränderte Prüfsummen ab.
* Bad, because dynamische Abfragen (Filter mit vielen optionalen Teilen) in sqlc umständlich sind;
  dafür sind mehrere feste Abfragen oder nullbare Parameter vorgesehen.

### Confirmation

* Gate: `sqlc generate` und `git diff --exit-code` (erzeugter Code aktuell).
* Gate `migrations-lint` (`tools/ci/migrations_lint.py`, Bau-Ticket M1-01): nur fortlaufende,
  lückenlose fünfstellige Nummern, keine Zeitstempel-Versionen, Prüfsummen gemergter Migrationen
  unverändert; dazu `goose validate`.
* Integrationstest: alle Migrationen auf leerem PostgreSQL 18.
* Kompatibilitätstest: Der Server der Vorversion läuft gegen das Schema der neuen Version
  (Erweitern und Zusammenziehen).
* Tabellenbesitz-Prüfung aus 02, Abschnitt 7.3: Abfragen eines Moduls nennen nur eigene Tabellen.

## Pros and Cons of the Options

### sqlc mit pgx v5 und goose

* Good, because sqlc (MIT) v1.31.1 `pgx/v5` als Paket unterstützt. [belegt, Q1, Q2]
* Good, because goose (MIT) v3.28.0 reines SQL, eingebettete Migrationen und Go-Migrationen kann.
  [belegt, Q4]
* Good, because goose ein hybrides Verfahren mit `fix` für Teams beschreibt [belegt, Q4]; dieses
  Projekt nutzt es nicht (Regel 4), die fortlaufenden Nummern reichen.
* Bad, because goose kein Schema-Diff erzeugt; Migrationen schreibt man von Hand.

### sqlc mit pgx v5 und golang-migrate

* Good, because golang-migrate (MIT) v4.20.1 reines SQL nutzt und verbreitet ist. [belegt, Q5]
* Bad, because ein dokumentierter Weg für Zeitstempel in Branches und fortlaufende Nummern auf
  `main` in den gelesenen Quellen fehlt. [abgeleitet]

### sqlc mit pgx v5 und Atlas

* Good, because Atlas Schema-Diffs erzeugt.
* Bad, because der Community Edition u. a. Trigger, Funktionen, RLS, Partitionierung und
  `migrate lint` fehlen und die Standard-Binaries unter einer proprietären Lizenz laufen.
  [belegt, Q6]

### ORM mit eigener Migration

* Good, because einfache CRUD-Abfragen weniger Code brauchen.
* Bad, because das Zähler-Muster aus 04, Abschnitt 3.2, die Sperrreihenfolge und die Rechtefilter
  exaktes SQL brauchen; ein ORM verdeckt genau diese Stellen. [abgeleitet]
* Bad, because PostgreSQL-Funktionen wie Ausschluss-Constraints und Partitionierung in
  ORM-Migrationen meist als rohes SQL enden. [vermutet]

## More Information

Quellen, abgerufen 2026-10-10:

* Q1 https://docs.sqlc.dev/en/latest/reference/config.html
* Q2 https://github.com/jackc/pgx (v5.11.0)
* Q3 https://docs.sqlc.dev/en/latest/howto/ddl.html
* Q4 https://github.com/pressly/goose (README, Abschnitte "Hybrid Versioning" und "NO TRANSACTION")
* Q5 https://github.com/golang-migrate/migrate
* Q6 https://atlasgo.io/community-edition

Planungsgrundlage B2, R-Stack. [02-architektur.md](../spec/02-architektur.md), Abschnitte 6, 7.3,
8 und 15.1.
