# tools

Werkzeuge des Repositorys: Gates, Prüfskripte und Generatoren. Nichts davon wird
ausgeliefert. Aufruf immer über `make` (Root-`Makefile`), damit dieselben gepinnten
Versionen laufen wie in CI. Einrichtung einmal je Klon: `make bootstrap`.

| Pfad | Zweck | make-Ziel | Tests |
|---|---|---|---|
| `pyproject.toml`, `uv.lock`, `.python-version` | gepinnte Python-Werkzeugkette (ruff, mypy, bandit, pytest, pre-commit, MkDocs, zizmor, check-jsonschema) | `bootstrap` | |
| `node/` | gepinnte Node-Werkzeuge (mermaid-cli, Redocly CLI, SonarScanner); Install-Skripte von npm-Paketen sind abgeschaltet | `bootstrap` | |
| `bin/fetch_tools.py`, `bin/tools.lock.json` | Release-Binärdateien (actionlint, gitleaks, go-arch-lint, golangci-lint, lychee, osv-scanner, sqlc, trivy) mit sha256-Pin laden | `bootstrap`, `tools-lock` | `bin/tests/` |
| `ci/check_runners.py` | Workflow-Regeln: Runner-Weiche, Trigger, SHA-Pinning, Timeouts, Rechte | `runners-check` | `ci/tests/` |
| `ci/components.py` | Komponenten-Karte: Pfadfilter in CI, Auflösung von Release-Tags | (CI) | `ci/tests/` |
| `ci/license_check.py` | Lizenztext und SPDX-Kopfzeilen | `licenses` | `ci/tests/` |
| `ci/labels.py` | Labels prüfen und auf GitHub anlegen | `labels-check`, `labels-sync` | `ci/tests/` |
| `ci/tracked_tree.py` | getrackte und nicht ignorierte Dateien in einen Ordner kopieren, damit Scanner, die Ordner durchlaufen, nur das Repository sehen | `trivy`, Test `ci/tests/test_gitleaks_rules.py` | `ci/tests/` |
| `ci/migrations_lint.py` | Migrationsregeln der Go-Module (ADR 0007, Kapitel 14): fortlaufende Nummern, Prüfsummen-Datei `migrations.sha256`, `STORED`, `NO TRANSACTION`; `lock` ergänzt nur neue Prüfsummen | `migrations-lint`, `migrations-lock` | `ci/tests/` |
| `ci/codegen_check.py` | Generator ausführen und Drift erzeugten Codes über Schnappschüsse vor und nach dem Lauf melden | `codegen-check` | `ci/tests/` |
| `ci/go_prereqs.py` | Voraussetzungen der Server-Gates: go, Docker-kompatible Laufzeit, Pins von go-arch-lint und sqlc | `codegen-check`, `arch`, `migrations-lint`, `perf`, `test` | `ci/tests/` (Rezepte: `ci/tests/test_make_server_gates.py`) |
| `conftest.py` | Test-Isolation: entfernt Variablen wie `GIT_DIR`, die Git-Hooks setzen, aus der Umgebung jedes Tests, damit Git-Fixtures nie das echte Repository treffen | `test` | `ci/tests/test_git_env_isolation.py` |
| `hooks/text_rules.py` | keine Gedankenstriche (U+2013, U+2014) in Textdateien | pre-commit, `lint` | `hooks/tests/` |
| `hooks/commit_msg.py` | Conventional Commits und DCO-Sign-off | commit-msg, `commitlint` | `hooks/tests/` |
| `denylist/` | White-Label-Scanner mit privater Liste und öffentlichen Regeln ([README](denylist/README.md)) | `denylist`, `denylist-history` | `denylist/tests/` |
| `docs/adr_index.py` | ADR-Index erzeugen und prüfen | `adr-index`, `docs` | `docs/tests/` |
| `docs/config_reference.py` | Abschnitt 4 der Konfigurationsreferenz aus den Kapiteltabellen erzeugen und prüfen, doppelte Namen ablehnen | `config-reference`, `docs` | `docs/tests/` |
| `docs/render_diagrams.py` | jedes Mermaid-Diagramm rendert | `diagrams` | `docs/tests/` |
| `docs/mkdocs_repo_links.py` | MkDocs-Hook: Links auf vorhandene Dateien außerhalb von `docs/` zeigen in der Site auf das Repository | `docs` | `docs/tests/` |
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
