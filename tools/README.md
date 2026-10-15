# tools

Werkzeuge des Repositorys: Gates, Prüfskripte und Generatoren. Nichts davon wird
ausgeliefert. Aufruf immer über `make` (Root-`Makefile`), damit dieselben gepinnten
Versionen laufen wie in CI. Einrichtung einmal je Klon: `make bootstrap`.

| Pfad | Zweck | make-Ziel | Tests |
|---|---|---|---|
| `pyproject.toml`, `uv.lock`, `.python-version` | gepinnte Python-Werkzeugkette (ruff, mypy, bandit, pytest, pre-commit, MkDocs, zizmor, check-jsonschema) | `bootstrap` | |
| `node/` | gepinnte Node-Werkzeuge (mermaid-cli, Redocly CLI, SonarScanner); Install-Skripte von npm-Paketen sind abgeschaltet | `bootstrap` | |
| `bin/fetch_tools.py`, `bin/tools.lock.json` | Release-Binärdateien (actionlint, amtool, gitleaks, go-arch-lint, golangci-lint, loki, lychee, osv-scanner, otelcol-contrib, promtool, sloth, sqlc, tempo, tofu, trivy) mit sha256-Pin laden; Asset als Binärdatei, tar.gz oder Zip | `bootstrap`, `tools-lock` | `bin/tests/` |
| `ci/check_runners.py` | Workflow-Regeln: Runner-Weiche, Trigger, SHA-Pinning, Timeouts, Rechte | `runners-check` | `ci/tests/` |
| `ci/tests/test_codeql_workflow.py` | CodeQL-Wächter: die Matrix von `.github/workflows/codeql.yml` deckt jede Sprache ab, die CodeQL unterstützt und die die Workflows, `tools/pyproject.toml` oder die Komponenten-Manifeste zeigen (dieselben Manifeste wie die Komponenten-Erkennung des Makefiles); Go mit manuellem Build je Modul; Workflow, Job und Schritte nur mit Optionen aus geschlossenen Listen; Schritt "Probelauf" in `docs/quality/public-switch.md` nennt jeden Check | `test` | |
| `ci/components.py` | Komponenten-Karte: Pfadfilter in CI, Auflösung von Release-Tags | (CI) | `ci/tests/` |
| `ci/license_check.py` | Lizenztext und SPDX-Kopfzeilen | `licenses` | `ci/tests/` |
| `ci/labels.py` | Labels prüfen und auf GitHub anlegen | `labels-check`, `labels-sync` | `ci/tests/` |
| `ci/tracked_tree.py` | getrackte und nicht ignorierte Dateien in einen Ordner kopieren, damit Scanner, die Ordner durchlaufen, nur das Repository sehen | `trivy`, Test `ci/tests/test_gitleaks_rules.py` | `ci/tests/` |
| `ci/migrations_lint.py` | Migrationsregeln der Go-Module (ADR 0007, Kapitel 14): fortlaufende Nummern, Prüfsummen-Datei `migrations.sha256`, `STORED`, `NO TRANSACTION`; `lock` ergänzt nur neue Prüfsummen | `migrations-lint`, `migrations-lock` | `ci/tests/` |
| `ci/codegen_check.py` | Generator ausführen und Drift erzeugten Codes über Schnappschüsse vor und nach dem Lauf melden | `codegen-check` | `ci/tests/` |
| `ci/go_prereqs.py` | Voraussetzungen der Server-Gates: go, Docker-kompatible Laufzeit, Pins von go-arch-lint und sqlc | `codegen-check`, `arch`, `migrations-lint`, `perf`, `test` | `ci/tests/` (Rezepte: `ci/tests/test_make_server_gates.py`) |
| `conftest.py` | Test-Isolation: entfernt Variablen wie `GIT_DIR`, die Git-Hooks setzen, aus der Umgebung jedes Tests, damit Git-Fixtures nie das echte Repository treffen | `test` | `ci/tests/test_git_env_isolation.py` |
| `opscheck/` | Betriebs-Konfiguration (Collector, Prometheus, Alertmanager, Loki, Tempo, Sloth, OpenTofu) mit den gepinnten Validatoren prüfen; abgeleitete Werte gleich ihrer Quelle ([README](opscheck/README.md)) | `ops-check` | `opscheck/tests/` |
| `hooks/text_rules.py` | keine Gedankenstriche (U+2013, U+2014) in Textdateien | pre-commit, `lint` | `hooks/tests/` |
| `hooks/commit_msg.py` | Conventional Commits und DCO-Sign-off | commit-msg, `commitlint` | `hooks/tests/` |
| `configdoc/configdoc.py`, `configdoc/planned.yaml`, `configdoc/retired.yaml` | Abschnitt 4 der Konfigurationsreferenz aus dem Schema des Servers (`services/server/architecture/config-schema.json`) und den Kapiteltabellen erzeugen und prüfen (Regeln D1 bis D10: doppelte Namen, Schema gegen Kapitel, Etappen geplanter Schlüssel, abgelöste Namen aus W1, Compose-Frist, Umgebungsnamen, Aktualität); die Tests halten die Erkennung von Code gegen den CommonMark-Parser `markdown-it-py` | `config-reference`, `config-doc-check` | `configdoc/tests/` |
| `configdoc/app_config.py` | Key table of `AppConfig` in `packages/domain` from section 4 of the configuration reference: every value of place `App` and the server keys of D-W-115 and D-W-116, kind derived from the default (docs/DECISIONS.md D-M4-08-5) | `app-config`, `codegen-check` | `configdoc/tests/test_app_config.py` |
| `denylist/` | White-Label-Scanner mit privater Liste und öffentlichen Regeln ([README](denylist/README.md)) | `denylist`, `denylist-history` | `denylist/tests/` |
| `docs/adr_index.py` | ADR-Index erzeugen und prüfen | `adr-index`, `docs` | `docs/tests/` |
| `docs/render_diagrams.py` | jedes Mermaid-Diagramm rendert | `diagrams` | `docs/tests/` |
| `docs/mkdocs_repo_links.py` | MkDocs-Hook: Links auf vorhandene Dateien außerhalb von `docs/` zeigen in der Site auf das Repository | `docs` | `docs/tests/` |
| `catalog/` | Aktionskatalog `docs/spec/actions.yaml` laden und prüfen, Tabellen von Kapitel 05 und Listen in `docs/api/redocly.yaml` daraus erzeugen ([README](catalog/README.md)); `event_catalog_dart.py` writes the event catalogue of `packages/domain` | `catalog`, `catalog-check`, `event-catalog` | `catalog/tests/` |
| `api-checks/` | Kopien der API-Verträge gegen ihre Quellen prüfen (Kapitel 06, G2), die erzeugten Tabellen von 06 und `docs/api/webhooks.md` erzeugen und gegen eine frische Erzeugung prüfen (G3) ([README](api-checks/README.md)) | `api-checks` (G2 und G3), `api-tables` | `api-checks/tests/` |
| `doccheck/register_check.py` | Modulregister `services/server/architecture/modules.yaml` mit Kapitel 03, `docs/spec/actions.yaml` (Lader aus `catalog/`), den Modultabellen von Kapitel 02, dem Komponenten-Diagramm und den Modul-READMEs abgleichen, eine Zeile je Abweichung (ADR 0050) | `register-check` | `doccheck/tests/` (rendern mit Markdown und dem gepinnten mermaid-cli, `make bootstrap`) |
| `docs/tests/test_config_values.py`, `docs/tests/test_toolchain_doc.py` | Zahlen- und Versionswächter für die Doku | `test` | |
| `sonarqube/` | lokaler SonarQube und Abgleich von Profil und Gate ([README](sonarqube/README.md)) | `sonar-up`, `sonar-config`, `sonar-config-check` | `sonarqube/tests/` |
| `design-tokens/` | Design-Tokens in CSS, Dart, Kontrasttabelle und Vorschau übersetzen (Kapitel 15, Design-System) | `tokens` (`design-tokens/Makefile`, Ziel `check`) | `design-tokens/tests/` (unittest, über `make tokens`) |

Regeln für neue Werkzeuge: Python mit Standardbibliothek oder PyYAML (ein MkDocs-Hook
zusätzlich mit MkDocs und Markdown aus derselben gepinnten Umgebung), eine Testdatei je
Werkzeug, `SPDX-License-Identifier: Apache-2.0` in jeder Datei, ein make-Ziel und eine Zeile
in dieser Tabelle. Ein eigenes Prüfskript bekommt zusätzlich seine Zeile im Gate in
docs/quality/gates.md; ein neues gepinntes Werkzeug eines Dritten (Binärdatei, Python- oder
Node-Paket) bekommt seine Zeile in docs/quality/toolchain.md, weil deren Test die Versionen gegen
die Pins hält.
