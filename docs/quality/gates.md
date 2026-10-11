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
| `lint` | pre-commit-Hooks über alle Dateien (Text-Regeln, Stripe webhook secret format `whsec_` of GitHub secret scanning, Leerzeichen, YAML/TOML/JSON, Merge-Marker, private Schlüssel, Dateigröße), ruff (mit Docstring-Pflicht `D`, Abschnitt "Doku im Code" in Kapitel 23), bandit, check-jsonschema (Issue-Formulare, Renovate, CodeRabbit), actionlint, zizmor, go vet und golangci-lint, flutter analyze und custom_lint, Skript `lint` | Stil, Fehlerquellen, Workflow-Sicherheit | `repo.max_file_kb` | ja |
| `typecheck` | mypy (strict), go build, tsc | Typen | | Werkzeuge |
| `arch` | go-arch-lint mit `<modul>/.go-arch-lint.yml`, erzeugt aus `architecture/modules.yaml` | Modulgrenzen und erlaubte Fremdbibliotheken der Go-Module (Kapitel 02 "go-arch-lint") | | ab erstem Go-Modul |
| `migrations-lint` | `tools/ci/migrations_lint.py` | fortlaufende, lückenlose fünfstellige Nummern ohne Zeitstempel-Versionen, Prüfsummen gemergter Migrationen unverändert (`migrations.sha256`), `STORED` bei `GENERATED ALWAYS AS`, `-- +goose NO TRANSACTION` bei `CONCURRENTLY` (ADR 0007, Kapitel 14) | | ab erster Migration |
| `test` | pytest, go test -race, flutter test, Skript `test` | Tests mit Abdeckungsdaten | | Werkzeuge |
| `coverage` | coverage, go tool cover, LCOV-Auswertung | Mindestabdeckung; Go ohne die Pakete unter `test/` und `perf/` (Testcode, Kapitel 23 "Komponenten-Vertrag") | `quality.coverage.tools_min_percent`, `quality.coverage.go_min_percent`, `quality.coverage.dart_min_percent` | Werkzeuge |
| `commitlint` | `tools/hooks/commit_msg.py` | Conventional Commits und DCO der Commits, die noch nicht auf `origin/main` sind | `commit.header_max_length` | ab erstem Remote |
| `docs` | `tools/docs/adr_index.py`, `tools/docs/config_reference.py`, `tools/docs/problem_code_refs.py`, `tools/docs/brief_deps.py`, `tools/docs/register_ids.py`, `tools/docs/risk_ranking.py`, MkDocs `--strict` mit dem Hook `tools/docs/mkdocs_repo_links.py` | ADR-Index und Konfigurationsreferenz aktuell, kein Konfigurationsname doppelt (Ruling L4), jedes Paar aus Status und Problem-Code in Markdown und API-Dateien steht so in `x-problem-codes` (06 Abschnitt 5 Punkt 7), Index, Wellen und Graph der Briefs stimmen mit dem Plan überein und kein Brief ruft ein `make`-Ziel, das erst ein Ticket außerhalb seiner Abhängigkeiten anlegt, jede ID, die eine Tabelle in `docs/**/*.md` in der ersten Spalte definiert (Rulings, Tests, Schutzregeln, Bedrohungen, Entscheidungen, Risiken, offene Fragen, Tickets), steht im ganzen Repository genau einmal so (D-W-60), jede Tabellenzeile hat so viele Zellen wie ihr Kopf (D-W-71), Übersicht und Abschnitte der offenen Fragen nennen dieselben IDs, die Rangliste der größten Risiken nennt die Bewertung des Registers und lässt kein höher bewertetes Risiko aus (D-W-78), und die Liste hinter ihrem letzten Rang nennt genau die übrigen Risiken mit dessen Bewertung (D-W-79), Site baut ohne Warnung; Links auf vorhandene Dateien außerhalb von `docs/` zeigen in der Site auf das Repository, ein fehlendes Ziel bleibt ein Fehler | | ja |
| `linkcheck` | lychee `--offline --include-fragments`; `tools/docs/placeholder_links.py` | lokale Links und Anker aller getrackten Markdown-Dateien; kein Link außerhalb von Code auf das Platzhalter-Repository `OWNER` (P-05), weil er nach dem Veröffentlichen ins Leere führt | | ja |
| `diagrams` | `tools/docs/render_diagrams.py` mit mermaid-cli, `tools/docs/erd_check.py` | jedes Mermaid-Diagramm rendert; jede `.mmd` hat ihre `.svg`, gerendert mit dem Design-Thema (`design/build/mermaid.config.json`), gestempelt mit dem SHA-256 von `.mmd` und Thema (sonst veraltet, D-W-56) und nicht breiter oder höher als `MAX_ASPECT`; `.mmd` nennen nur Token-Farben; die ERDs zeigen jede Tabelle und jeden Fremdschlüssel aus 03 | | ja |
| `api-lint` | Redocly CLI mit `docs/api/redocly.yaml` | OpenAPI und AsyncAPI, Pflicht von `x-authz-action` | | sobald `docs/api/openapi.yaml` existiert |
| `api-clients` | Ziel `generate-check` der Client-Pakete | generierte Clients sind aktuell und kompilieren | | ab erstem Client-Paket |
| `tokens` | Ziel `check` in `tools/design-tokens/` | Tests des Generators; Design-Tokens bauen, Kontraste und Palettenwächter bestehen, erzeugte Dateien aktuell, eingebettete Dateien passen zu `vendor/MANIFEST.json` | | ja |
| `runners-check` | `tools/ci/check_runners.py` | Nur feste GitHub-gehostete Runner-Labels (ADR 0039), Trigger, SHA-Pinning, Timeouts, Rechte; in Jobs mit Umgebung `release*` Ausgaben von Jobs ohne diese Umgebung nur im `if:` des Jobs oder eines Schritts (Erlaubnisliste D-W-61, D-W-72), Artefakte nur per `artifact-ids` (D-W-52); Matrix-Werte in `runs-on` je Kombination geprüft, keine Laufzeit-Matrix hinter `runs-on` (D-W-53); Release-Workflows ruft nur ein Job von `release.yml` ohne Matrix auf (D-W-73); `push` nur auf Tags, außer `denylist-main.yml` auf `main` | | ja |
| `labels-check` | `tools/ci/labels.py` | Label-Datei gültig, Vorlagen und GOVERNANCE.md nutzen nur definierte Labels | | ja |
| `gitleaks` | gitleaks mit `.gitleaks.toml` | Secrets in jedem von `HEAD` erreichbaren Commit; vor dem Öffentlich-Schalten in allen Refs; the EXAMPLE marker exempts only this product's own token formats and generic key-like values | | ja |
| `semgrep` | Semgrep mit `.semgrep/` | Projektregeln und ihre Regel-Tests | | ja |
| `trivy` | Trivy `config` auf einer Kopie der getrackten und nicht ignorierten Dateien (`tools/ci/tracked_tree.py`) | Fehlkonfiguration in Dockerfiles, Kubernetes, Helm, Terraform | `HIGH`, `CRITICAL` | sobald solche Dateien existieren |
| `osv` | OSV-Scanner, govulncheck | bekannte Schwachstellen aller Lock-Dateien; erreichbare Go-Schwachstellen | | ja |
| `licenses` | `tools/ci/license_check.py`, OSV-Scanner `--licenses`, go-licenses | Lizenztext, SPDX-Kopfzeilen, Lizenzen ausgelieferter Abhängigkeiten | `quality/licenses/allowed.txt`, `license.spdx_header_lines` | ja (Abhängigkeiten ab erster Komponente) |
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
  `DENYLIST_FLAGS=--allow-missing` (nur öffentliche Regeln), `SONAR_SKIP=1` und
  `REVIEW_SKIP=1` (kein eigenes unabhängiges Review). Alles steht sichtbar in der Ausgabe;
  CI behandelt Fork-Pull-Requests genauso (R-QA-02). Was der Fork nicht prüfen konnte,
  holt der Maintainer vor dem Merge nach; die private Liste prüft nach dem Merge zusätzlich
  `denylist-main.yml` jeden Push auf `main`.
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
| pre-commit | bei jedem Commit, auf den geänderten Dateien | Text-Regeln (keine Gedankenstriche), Stripe webhook secret format `whsec_` (`tools/hooks/secret_formats.py`), Datei-Hygiene, ruff für `tools/`, gitleaks auf den gestagten Änderungen |
| commit-msg | bei jedem Commit | Conventional Commits und DCO-Sign-off |
| pre-push | vor jedem Push | `tools/hooks/review_gate.py` (Bericht `.review/<sha>.md` des gepushten Commits endet mit `Urteil: JA`, [red-team-gate.md](red-team-gate.md)), dann `make check` |

## Einzelne Gates

Jedes Gate läuft auch allein, zum Beispiel `make lint` oder `make diagrams`. Für die
Diagramme kann `MMDC_PUPPETEER_CONFIG` auf eine Puppeteer-Konfiguration zeigen. Nennt sie
ein eigenes Chromium (`executablePath`), lädt `make bootstrap` keinen Browser; sonst lädt
es den zu Puppeteer passenden. CI nutzt `tools/node/puppeteer-ci.json` (Chromium ohne
Sandbox, weil Runner Benutzer-Namensräume einschränken können; gerendert werden nur die
eigenen Diagramme).

Weitere Schalter: `COMMIT_RANGE` und `DENYLIST_RANGE` (Commit-Bereich statt
`origin/main..HEAD`), `GITLEAKS_LOG_OPTS` (Standard `HEAD`, alle Refs mit `--all`), `GITLEAKS_FLAGS` (zum
Beispiel ein JSON-Bericht), `DENYLIST_FLAGS` (zum Beispiel `--reveal` lokal).

## Was die Gates nicht abdecken

* Links ins Internet: `make linkcheck-external` bei Bedarf (R-QA-09).
* Ob der Inhalt einer `.svg` dem entspricht, was mermaid-cli heute aus ihrer `.mmd` rendern
  würde: Die Ausgabe hängt von der Browser-Version ab, ein Byte-Vergleich wäre auf jedem
  Rechner anders. `make diagrams` prüft stattdessen den Stempel, den `render_diagrams.py
  --write` aus `.mmd` und Thema schreibt (D-W-56); eine von Hand bearbeitete `.svg` mit altem
  Stempel fällt nicht auf.
* Shell-Code in Workflows prüft actionlint ohne shellcheck, weil shellcheck nicht gepinnt
  verfügbar ist; Workflows rufen darum nur kurze Befehle und make-Ziele auf.
* Die Gleichheit von lokalem SonarQube und Cloud prüft `make sonar-config-check`, nicht
  jeder Lauf (sonarqube-local.md).
* Ob eine Migration auf PostgreSQL läuft, prüft `migrations-lint` nicht; das zeigt der
  Integrationstest mit allen Migrationen auf leerem PostgreSQL 18 (ADR 0007,
  "Confirmation"). `goose validate` prüft die Dateien ohne Datenbank; es kommt mit
  `bothy migrate validate` (PLAN M1-12) und ist noch kein Teil von `migrations-lint`.
* `migrations-lint` vergleicht gemergte Migrationen nur mit `migrations.sha256` im
  Arbeitsbaum, nicht mit dem Stand des Ausgangsbranchs. Wer eine Migration und ihre Zeile in
  der Prüfsummen-Datei zugleich von Hand ändert, die neueste Migration samt ihrer Zeile
  entfernt oder `migrations/` samt `migrations.sha256` löscht, besteht das Gate; das muss
  das Review des Diffs sehen.
* GitHub secret scanning has its own patterns
  ([supported patterns](https://docs.github.com/en/code-security/secret-scanning/introduction/supported-secret-scanning-patterns),
  retrieved 2026-10-11): it knows provider formats that gitleaks does not, and its
  `stripe_webhook_signing_secret` type also reports `whsec_` values shorter than the
  gitleaks rule `standard-webhooks-secret` accepts. An example in a third-party format
  can therefore pass the gates and still raise an alert on push; only the `whsec_` format has
  its own check with a wider margin (`tools/hooks/secret_formats.py`). Examples therefore
  stay short of every third-party format (CONTRIBUTING.md).
