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
| `lint` | pre-commit-Hooks über alle Dateien (Text-Regeln, Leerzeichen, YAML/TOML/JSON, Merge-Marker, private Schlüssel, Dateigröße), ruff, bandit, check-jsonschema (Issue-Formulare, Renovate, CodeRabbit), actionlint, zizmor, go vet und golangci-lint, flutter analyze und custom_lint, Skript `lint` | Stil, Fehlerquellen, Workflow-Sicherheit | `repo.max_file_kb` | ja |
| `typecheck` | mypy (strict), go build, tsc | Typen | | Werkzeuge |
| `test` | pytest, go test -race, flutter test, Skript `test` | Tests mit Abdeckungsdaten | | Werkzeuge |
| `coverage` | coverage, go tool cover, LCOV-Auswertung | Mindestabdeckung | `quality.coverage.tools_min_percent`, `quality.coverage.go_min_percent`, `quality.coverage.dart_min_percent` | Werkzeuge |
| `commitlint` | `tools/hooks/commit_msg.py` | Conventional Commits und DCO der Commits, die noch nicht auf `origin/main` sind | `commit.header_max_length` | ab erstem Remote |
| `docs` | `tools/docs/adr_index.py`, MkDocs `--strict` | ADR-Index aktuell, Site baut ohne Warnung | | ja |
| `linkcheck` | lychee `--offline --include-fragments` | lokale Links und Anker aller getrackten Markdown-Dateien | | ja |
| `diagrams` | `tools/docs/render_diagrams.py` mit mermaid-cli | jedes Mermaid-Diagramm rendert, jede `.mmd` hat ihre `.svg` | | ja |
| `api-lint` | Redocly CLI mit `docs/api/redocly.yaml` | OpenAPI und AsyncAPI, Pflicht von `x-authz-action` | | sobald `docs/api/openapi.yaml` existiert |
| `api-clients` | Ziel `generate-check` der Client-Pakete | generierte Clients sind aktuell und kompilieren | | ab erstem Client-Paket |
| `tokens` | Ziel `check` in `tools/design-tokens/` | Design-Tokens bauen, Kontraste bestehen | | sobald `tools/design-tokens/` existiert |
| `runners-check` | `tools/ci/check_runners.py` | Runner-Weiche, Trigger, SHA-Pinning, Timeouts, Rechte | | ja |
| `labels-check` | `tools/ci/labels.py` | Label-Datei gültig, Vorlagen und GOVERNANCE.md nutzen nur definierte Labels | | ja |
| `gitleaks` | gitleaks mit `.gitleaks.toml` | Secrets in jedem von `HEAD` erreichbaren Commit; vor dem Öffentlich-Schalten in allen Refs | | ja |
| `semgrep` | Semgrep mit `.semgrep/` | Projektregeln und ihre Regel-Tests | | ja |
| `trivy` | Trivy `config` | Fehlkonfiguration in Dockerfiles, Kubernetes, Helm, Terraform | `HIGH`, `CRITICAL` | sobald solche Dateien existieren |
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
  `DENYLIST_FLAGS=--allow-missing` (nur öffentliche Regeln) und `SONAR_SKIP=1`. Beides
  steht sichtbar in der Ausgabe; CI behandelt Fork-Pull-Requests genauso (R-QA-02).
* `COMPONENTS=<liste>` beschränkt die Komponenten-Gates; CI setzt es aus den geänderten
  Pfaden. Lokal ist der Standard `all`.

## Hooks

`make bootstrap` installiert drei Hooks aus `.pre-commit-config.yaml`:

| Hook | Läuft | Inhalt |
|---|---|---|
| pre-commit | bei jedem Commit, auf den geänderten Dateien | Text-Regeln (keine Gedankenstriche), Datei-Hygiene, ruff für `tools/`, gitleaks auf den gestagten Änderungen |
| commit-msg | bei jedem Commit | Conventional Commits und DCO-Sign-off |
| pre-push | vor jedem Push | `make check` |

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
* Shell-Code in Workflows prüft actionlint ohne shellcheck, weil shellcheck nicht gepinnt
  verfügbar ist; Workflows rufen darum nur kurze Befehle und make-Ziele auf.
* Die Gleichheit von lokalem SonarQube und Cloud prüft `make sonar-config-check`, nicht
  jeder Lauf (sonarqube-local.md).
