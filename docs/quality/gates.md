# Gates

`make check` führt alle Gates in fester Reihenfolge aus und bricht beim ersten Fehler ab.
CI führt dieselben Ziele aus: Job `gates` ruft `make gates`, Job `denylist` ruft
`make denylist`, Job `sonar` ruft `make sonar`; `check` ist genau `gates denylist sonar`
(Kapitel 23, R-QA-01). Schwellen stehen als Namen; ihre Standardwerte stehen nur in
Kapitel 23, Abschnitt 11.

![Ablauf vom Commit bis zum Release](pipeline.svg)

Quelle des Diagramms: `docs/quality/pipeline.mmd`.

## Liste

| make-Ziel | Werkzeug | Prüft | Schwelle | Heute aktiv |
|---|---|---|---|---|
| `fmt-check` | ruff format, gofmt, dart format, Skript `format:check` der TypeScript-Projekte | Format ohne Änderungen | | Werkzeuge |
| `codegen-check` | `tools/ci/codegen_check.py` mit sqlc und `go generate` (go-arch-lint und sqlc aus den Pins vorn im `PATH`) | erzeugter Code (sqlc-Code, `.go-arch-lint.yml`, API-Code) entspricht einer frischen Erzeugung; es zählen nur Änderungen durch den Generator, nicht Handänderungen im Arbeitsbaum | | ab erstem Go-Modul |
| `lint` | pre-commit-Hooks über alle Dateien (Text-Regeln, Leerzeichen, YAML/TOML/JSON, Merge-Marker, private Schlüssel, Dateigröße), ruff, bandit, check-jsonschema (Issue-Formulare, Renovate, CodeRabbit), actionlint, zizmor, go vet und golangci-lint, flutter analyze und custom_lint, Skript `lint` | Stil, Fehlerquellen, Workflow-Sicherheit | `repo.max_file_kb` | ja |
| `typecheck` | mypy (strict), go build, tsc | Typen | | Werkzeuge |
| `arch` | go-arch-lint mit `<modul>/.go-arch-lint.yml`, erzeugt aus `architecture/modules.yaml` | Modulgrenzen und erlaubte Fremdbibliotheken der Go-Module (Kapitel 02 "go-arch-lint"); keine symbolischen Links unterhalb eines Moduls, weil go-arch-lint einem verlinkten Verzeichnis nicht folgt und es nach dem Pfad des Links zuordnet, der Go-Compiler den Code dahinter aber baut | | ab erstem Go-Modul |
| `register-check` | `tools/doccheck/register_check.py` | Modulregister `services/server/architecture/modules.yaml` gleich dem Kern-Vertrag (Tabellen und ihre Module aus Kapitel 03, Aktionen, Ereignistypen und Ops aus `docs/spec/actions.yaml`) und gleich den Stellen, die es wiederholen: Tabellen "Modulliste", "Erlaubte Abhängigkeiten", "Querschnitts-Ports", "Aktionen je Modul" und "Ereignistypen und WebSocket-Ops je Modul" in Kapitel 02, Komponenten-Diagramm `c4-3-komponenten-server.mmd`, Modul-READMEs; eine Zeile je Abweichung mit Datei und Element (ADR 0050, Regeln C1 bis C12 im Docstring) | | ja |
| `migrations-lint` | `tools/ci/migrations_lint.py` | fortlaufende, lückenlose fünfstellige Nummern ohne Zeitstempel-Versionen, Prüfsummen gemergter Migrationen unverändert (`migrations.sha256`), `STORED` bei `GENERATED ALWAYS AS`, `-- +goose NO TRANSACTION` bei `CONCURRENTLY` (ADR 0007, Kapitel 14) | | ab erster Migration |
| `test` | pytest, go test -race, flutter test, Skript `test` | Tests mit Abdeckungsdaten | | Werkzeuge |
| `coverage` | coverage, go tool cover, LCOV-Auswertung | Mindestabdeckung; Go ohne die Pakete unter `test/` und `perf/` (Testcode, Kapitel 23 "Komponenten-Vertrag") | `quality.coverage.tools_min_percent`, `quality.coverage.go_min_percent`, `quality.coverage.dart_min_percent` | Werkzeuge |
| `commitlint` | `tools/hooks/commit_msg.py` | Conventional Commits und DCO der Commits, die noch nicht auf `origin/main` sind | `commit.header_max_length` | ab erstem Remote |
| `docs` | `tools/docs/adr_index.py`, `tools/docs/config_reference.py`, MkDocs `--strict` mit dem Hook `tools/docs/mkdocs_repo_links.py` | ADR-Index und Konfigurationsreferenz aktuell, kein Konfigurationsname doppelt (Ruling L4), Site baut ohne Warnung; Links auf vorhandene Dateien außerhalb von `docs/` zeigen in der Site auf das Repository, ein fehlendes Ziel bleibt ein Fehler | | ja |
| `linkcheck` | lychee `--offline --include-fragments` | lokale Links und Anker aller getrackten Markdown-Dateien | | ja |
| `diagrams` | `tools/docs/render_diagrams.py` mit mermaid-cli | jedes Mermaid-Diagramm rendert, jede `.mmd` hat ihre `.svg` | | ja |
| `catalog-check` | `tools/catalog/action_catalog.py` | `docs/spec/actions.yaml` gültig (Schema, eindeutige IDs, Rate-Limit-Klassen, Empfangs-Aktionen, Ops ohne Systemaktion), gleich mit den von Hand gepflegten Teilen von Kapitel 05 (Scopes für Apps, Wirkung, Konfigurationswerte der Klassen); die erzeugten Tabellen in 05 und Listen in `docs/api/redocly.yaml` sind aktuell ([ADR 0049](../adr/0049-maschinenlesbarer-aktionskatalog-als-einzige-quelle.md)) | | ja |
| `api-lint` | Redocly CLI mit `docs/api/redocly.yaml` | OpenAPI und AsyncAPI, Pflicht von `x-authz-action` | | sobald `docs/api/openapi.yaml` existiert |
| `api-checks` | `tools/api-checks/api_checks.py` | Kopien der API-Verträge gleich ihren Quellen (Kapitel 06, G2 und G3): `x-authz-action`, `x-rate-limit-class`, Ops, Ereignisse, Empfangs-Aktionen, Scopes (`AppScope`) und die Union `Event` in `openapi.yaml` und `asyncapi.yaml` gleich `docs/spec/actions.yaml`, Schließ-Codes gleich 04, `Idempotency-Key` oder `x-idempotency-exempt` mit bekanntem Grund an jedem POST und PATCH, Deprecation-Daten gegen `api.deprecation_min_notice`, `x-problem-codes` und `x-status` vollständig; die erzeugten Tabellen in 06 und `docs/api/webhooks.md` sind aktuell ([README](../../tools/api-checks/README.md)) | | ja |
| `api-clients` | Ziel `generate-check` der Client-Pakete | generierte Clients sind aktuell und kompilieren | | ab erstem Client-Paket |
| `tokens` | Ziel `check` in `tools/design-tokens/` | Tests des Generators; Design-Tokens bauen, Kontraste und Palettenwächter bestehen, erzeugte Dateien aktuell, eingebettete Dateien passen zu `vendor/MANIFEST.json` | | ja |
| `runners-check` | `tools/ci/check_runners.py` | Runner-Weiche, Trigger, SHA-Pinning, Timeouts, Rechte | | ja |
| `labels-check` | `tools/ci/labels.py` | Label-Datei gültig, Vorlagen und GOVERNANCE.md nutzen nur definierte Labels | | ja |
| `gitleaks` | gitleaks mit `.gitleaks.toml` | Secrets in jedem von `HEAD` erreichbaren Commit; vor dem Öffentlich-Schalten in allen Refs | | ja |
| `semgrep` | Semgrep mit `.semgrep/` | Projektregeln und ihre Regel-Tests | | ja |
| `trivy` | Trivy `config` auf einer Kopie der getrackten und nicht ignorierten Dateien (`tools/ci/tracked_tree.py`) | Fehlkonfiguration in Dockerfiles, Kubernetes, Helm, Terraform | `HIGH`, `CRITICAL` | sobald solche Dateien existieren |
| `osv` | OSV-Scanner, govulncheck | bekannte Schwachstellen aller Lock-Dateien; erreichbare Go-Schwachstellen | | ja |
| `licenses` | `tools/ci/license_check.py`, OSV-Scanner `--licenses`, go-licenses | Lizenztext, SPDX-Kopfzeilen, Lizenzen ausgelieferter Abhängigkeiten; eine Liste ohne Lizenz ist ein Fehler (leer würden `--licenses=` und `--allowed_licenses=` die Lizenzen nicht mehr einschränken) | `quality/licenses/allowed.txt`, `license.spdx_header_lines` | ja (Abhängigkeiten ab erster Komponente) |
| `denylist` | `tools/denylist/denylist_scan.py` | White Label in Inhalten, Namen, Commits, Identitäten, Refs | | ja, braucht die private Liste |
| `sonar` | SonarScanner (npm) gegen den lokalen Server oder SonarQube Cloud | Gate "Bothy way" | `quality.sonar.*` | ab erstem Produkt-Code |

Release-Workflow zusätzlich: `trivy-image` (Schwachstellen im gebauten Image), SBOM
(Syft über `anchore/sbom-action`), Build-Provenance und SBOM-Attestation (öffentlich),
`portability` (ganzer Stack auf frischer Maschine).

## Überspringen und Fehlschlagen

* Ein Gate schreibt `SKIP <name> <Grund>` und gilt als bestanden, wenn die Komponente
  keinen Code hat. Der Makefile entscheidet das aus `git ls-files`: ein Go-Modul ist ein
  getracktes `go.mod` unter `services/`, ein Dart-Paket ein `pubspec.yaml` unter `apps/`
  oder `packages/`, ein Python-Projekt ein `pyproject.toml` unter `sdk/python/` oder
  `plugins/`, ein TypeScript-Projekt ein `package.json` unter `sdk/typescript/` oder
  `plugins/`. Neue Dateien zählen erst nach `git add`.
* Ein fehlendes Werkzeug, eine fehlende Liste, ein fehlendes Token oder ein fehlender
  Teil des Komponenten-Vertrags (Kapitel 23, Abschnitt 3) ist ein Fehler.
* Beitragende aus Forks schalten ausdrücklich ab, was sie nicht haben:
  `DENYLIST_FLAGS=--allow-missing` (nur öffentliche Regeln) und `SONAR_SKIP=1`. Beides
  steht sichtbar in der Ausgabe; CI behandelt Fork-Pull-Requests genauso (R-QA-02).
* `COMPONENTS=<liste>` beschränkt die Komponenten-Gates; CI setzt es aus den geänderten
  Pfaden. Lokal ist der Standard `all`.
* Server-Voraussetzungen: Sobald ein Go-Modul ausgewählt ist, prüfen alle Server-Gates
  (`codegen-check`, `arch`, `migrations-lint`, `perf`) und der Go-Teil von `test` zuerst
  `go` auf dem `PATH`, eine erreichbare Docker-kompatible Laufzeit (`docker info`) und die
  Pins von go-arch-lint und sqlc (`tools/ci/go_prereqs.py`, lädt fehlende Pins und prüft
  ihre sha256). Fehlt etwas, scheitert das Gate mit einer Zeile je fehlendem Teil; ein
  `SKIP` gibt es dafür nicht (R-QA-02, PLAN P-09). Docker ist für jedes Server-Gate
  Pflicht, nicht nur für die Tests, damit ein Rechner, der die Integrationstests nicht
  ausführen kann, nie einen halb grünen Server-Stand meldet.
* `migrations-lint` überspringt ein Go-Modul mit einer `SKIP`-Zeile, solange es weder
  `migrations/` noch `migrations.sha256` hat (keine Migration, nichts zu prüfen). Liegt nur
  noch die Prüfsummen-Datei da, scheitert es.

## Leistung (`make perf`)

`make perf` führt die Lasttests jedes Go-Moduls aus: `go test -tags perf -count=1` über
`<modul>/perf/...`, mit dem Zeitlimit `quality.perf.timeout` je Modul. Es läuft am
Abschluss jeder Etappe und im Release, nie in `gates` und nie im pre-push-Hook (PLAN P-07):
Die Leistungsziele aus Kapitel 02 "Leistungsziele" gelten nur auf `perf.reference_profile`,
auf beliebiger Hardware wären die Messwerte nicht vergleichbar (ADR 0063). Darum steht vor
jedem Modul der Hinweis auf `perf.reference_profile` in der Ausgabe.

Lasttests tragen das Build-Tag `perf`, damit `make test` sie nicht ausführt. Ihre Pakete
liegen unter `<modul>/perf/` und zählen nicht zur Abdeckung. Ohne Go-Modul meldet
`make perf` `SKIP`; ein Go-Modul ohne `perf/` ist ein Fehler. Wie die Server-Gates braucht
`make perf` go, eine Docker-kompatible Laufzeit und die Pins.

## Migrationen

Ablauf für die Migrationen eines Go-Moduls (ADR 0007, Gate `migrations-lint`):

1. Die nächste freie Nummer des Ausgangsbranchs nehmen: `migrations/NNNNN_<slug>.sql`
   (fünf Ziffern, Slug aus Kleinbuchstaben, Ziffern und einzelnen Unterstrichen) mit genau
   einer Zeile `-- +goose Up`.
2. `make migrations-lock` aufrufen. Es trägt die Prüfsumme jeder neuen Datei in
   `<modul>/migrations.sha256` ein und ändert oder entfernt nie eine bestehende Zeile.
3. Eine Migration, die im Ausgangsbranch gemergt ist, wird nie geändert oder gelöscht;
   eine Korrektur ist eine neue Migration.
4. Haben zwei Branches dieselbe Nummer gewählt, nummeriert der später gemergte Branch seine
   Datei um, entfernt seine eigene Zeile aus `migrations.sha256` und ruft
   `make migrations-lock` erneut auf, vor seinem letzten `make check`.
5. Ein Entwurf darf eine Zeitstempel-Version tragen. Das Gate lehnt sie in jedem Lauf ab,
   auch auf Feature-Branches, weil der lokale Merge erst nach einem grünen `make check`
   folgt.

Jede Spalte `GENERATED ALWAYS AS (...)` schreibt `STORED`, weil PostgreSQL 18 sonst eine
virtuelle Spalte anlegt (Kapitel 14 "Ausdrücke"). Eine Anweisung mit `CONCURRENTLY` (außer
`REFRESH MATERIALIZED VIEW CONCURRENTLY`) steht in einer eigenen Datei mit der Zeile
`-- +goose NO TRANSACTION` (ADR 0007, Regel 3). Kommentare, Zeichenketten und Bezeichner in
Anführungszeichen zählen für beide Regeln nicht. Die Prüfsummen-Datei hat das Format von
`shasum -a 256` (zwei Leerzeichen, nach Dateiname sortiert, Zeilenende am Schluss) und liegt
neben `migrations/`, damit weder das Einbettungsmuster `*.sql` noch sqlc sie liest.

## Hooks

`make bootstrap` installiert drei Hooks aus `.pre-commit-config.yaml`:

| Hook | Läuft | Inhalt |
|---|---|---|
| pre-commit | bei jedem Commit, auf den geänderten Dateien | Text-Regeln (keine Gedankenstriche), Datei-Hygiene, ruff für `tools/`, gitleaks auf den gestagten Änderungen |
| commit-msg | bei jedem Commit | Conventional Commits und DCO-Sign-off |
| pre-push | vor jedem Push | `make check` |

## Einzelne Gates

Jedes Gate läuft auch allein, zum Beispiel `make lint` oder `make diagrams`. Für die
Diagramme kann `MMDC_PUPPETEER_CONFIG` auf eine Puppeteer-Konfiguration zeigen, die auch die
Diagrammtests von `make test` nutzen; ein relativer Pfad gilt ab dem Repository-Stamm, auch
wenn ein Test aus `tools/` rendert. Nennt sie ein eigenes Chromium (`executablePath`), lädt
`make bootstrap` keinen Browser; sonst lädt es den zu Puppeteer passenden. CI setzt `tools/node/puppeteer-ci.json` (Chromium ohne Sandbox, weil Runner
Benutzer-Namensräume einschränken können; gerendert werden nur die eigenen Diagramme) in jedem
Schritt, der `make diagrams` oder `make test` erreicht, also für `make gates` und `make sonar`;
ein Test in `tools/doccheck` hält das.

Weitere Schalter: `COMMIT_RANGE` und `DENYLIST_RANGE` (Commit-Bereich statt
`origin/main..HEAD`), `GITLEAKS_LOG_OPTS` (Standard `HEAD`, alle Refs mit `--all`), `GITLEAKS_FLAGS` (zum
Beispiel ein JSON-Bericht), `DENYLIST_FLAGS` (zum Beispiel `--reveal` lokal).

## Was die Gates nicht abdecken

* Links ins Internet: `make linkcheck-external` bei Bedarf (R-QA-09).
* Ob eine `.svg` zum Stand ihrer `.mmd` passt: `make diagrams` prüft nur, dass die Quelle
  rendert und die `.svg` daneben liegt. Die Ausgabe von mermaid-cli hängt von der
  Browser-Version ab, ein Byte-Vergleich wäre auf jedem Rechner anders. Wer eine `.mmd`
  ändert, rendert die `.svg` im selben Commit neu.
* Shell-Code in Workflows prüft actionlint ohne shellcheck, weil shellcheck nicht gepinnt
  verfügbar ist; Workflows rufen darum nur kurze Befehle und make-Ziele auf.
* Die Gleichheit von lokalem SonarQube und Cloud prüft `make sonar-config-check`, nicht
  jeder Lauf (sonarqube-local.md).
* Ob eine Migration auf PostgreSQL läuft, prüft `migrations-lint` nicht; das zeigt der
  Integrationstest mit allen Migrationen auf leerem PostgreSQL 18 (ADR 0007,
  "Confirmation"). `goose validate` prüft die Dateien ohne Datenbank; es kommt mit
  `bothy migrate validate` (PLAN M1-12) und ist noch kein Teil von `migrations-lint`.
* `register-check` vergleicht nur Namen: die Maschinenspalten der Tabellen in 02, Knoten und
  Kanten des Diagramms, Listen der READMEs. Prosa ("Verantwortung", "Grund je Kante", die
  Tabelle "Ports, Ereignisse und Quellen je Modul", Zweck, Port-Texte und weitere Absätze der
  READMEs) bleibt Review. Einen Namen außerhalb eines Code-Spans meldet es nur, wenn eine
  Quelle ihn kennt; ein unbekanntes Wort (etwa ein Tippfehler) bleibt Review. Seine statische
  Lesart nähert Python-Markdown an (Setext-Überschriften liest es nicht); dass sie auf den
  echten Dateien der gerenderten Site gleicht, hält ein Test in `make test` (DECISIONS M1-06/11).
  Die READMEs rendert GitHub; was nur GitHub anders zeigt als Python-Markdown, sieht der Test
  nicht. Das Diagramm liest es über eine geschlossene Zeilengrammatik; dass sie Mermaid auf
  dem echten Diagramm gleicht, hält ein Test mit dem gepinnten mermaid-cli (braucht
  `make bootstrap`).
  Tabellenbesitz in SQL-Dateien und der Abgleich mit den Tabellen der Migrationen (ADR 0050)
  fehlen, solange es keine Migrationen und Abfragen gibt; sie kommen mit dem ersten Schema
  (PLAN M1-12).
* `migrations-lint` vergleicht gemergte Migrationen nur mit `migrations.sha256` im
  Arbeitsbaum, nicht mit dem Stand des Ausgangsbranchs. Wer eine Migration und ihre Zeile in
  der Prüfsummen-Datei zugleich von Hand ändert, die neueste Migration samt ihrer Zeile
  entfernt oder `migrations/` samt `migrations.sha256` löscht, besteht das Gate; das muss
  das Review des Diffs sehen.
