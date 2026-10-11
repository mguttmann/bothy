# tools

Werkzeuge des Repositorys: Gates, Prüfskripte und Generatoren. Nichts davon wird
ausgeliefert. Aufruf immer über `make` (Root-`Makefile`), damit dieselben gepinnten
Versionen laufen wie in CI. Einrichtung einmal je Klon: `make bootstrap`.

| Pfad | Zweck | make-Ziel | Tests |
|---|---|---|---|
| `pyproject.toml`, `uv.lock`, `.python-version` | gepinnte Python-Werkzeugkette (ruff, mypy, bandit, pytest, pre-commit, MkDocs, zizmor, check-jsonschema) | `bootstrap` | |
| `node/` | gepinnte Node-Werkzeuge (mermaid-cli, Redocly CLI, SonarScanner); Install-Skripte von npm-Paketen sind abgeschaltet | `bootstrap` | |
| `bin/fetch_tools.py`, `bin/tools.lock.json` | Release-Binärdateien (actionlint, gitleaks, go-arch-lint, golangci-lint, lychee, osv-scanner, sqlc, trivy) mit sha256-Pin laden | `bootstrap`, `tools-lock` | `bin/tests/` |
| `ci/check_runners.py` | Workflow-Regeln: GitHub-gehostete Runner, Trigger, SHA-Pinning, Timeouts, Rechte | `runners-check` | `ci/tests/` |
| `ci/components.py` | Komponenten-Karte: Pfadfilter in CI, Auflösung von Release-Tags | (CI) | `ci/tests/` |
| `ci/license_check.py` | Lizenztext und SPDX-Kopfzeilen | `licenses` | `ci/tests/` |
| `ci/labels.py` | Labels prüfen und auf GitHub anlegen | `labels-check`, `labels-sync` | `ci/tests/` |
| `ci/tracked_tree.py` | getrackte und nicht ignorierte Dateien in einen Ordner kopieren, damit Scanner, die Ordner durchlaufen, nur das Repository sehen | `trivy`, Test `ci/tests/test_gitleaks_rules.py` | `ci/tests/` |
| `ci/migrations_lint.py` | Migrationsregeln der Go-Module (ADR 0007, Kapitel 14): fortlaufende Nummern, Prüfsummen-Datei `migrations.sha256`, `STORED`, `NO TRANSACTION`; `lock` ergänzt nur neue Prüfsummen | `migrations-lint`, `migrations-lock` | `ci/tests/` |
| `ci/codegen_check.py` | Generator ausführen und Drift erzeugten Codes über Schnappschüsse vor und nach dem Lauf melden | `codegen-check` | `ci/tests/` |
| `ci/go_prereqs.py` | Voraussetzungen der Server-Gates: go, Docker-kompatible Laufzeit, Pins von go-arch-lint und sqlc | `codegen-check`, `arch`, `migrations-lint`, `perf`, `test` | `ci/tests/` (Rezepte: `ci/tests/test_make_server_gates.py`) |
| `conftest.py` | Test isolation: removes variables such as `GIT_DIR`, which git hooks set, from the environment of every test, so that git fixtures never reach the real repository; runs the whole test run (collection, fixtures of every scope, every test) without a git identity, like a fresh CI runner (no global or system configuration, `user.useConfigOnly`) | `test` | `ci/tests/test_git_env_isolation.py` |
| `testing/git_fixtures.py` | throw-away git repositories for tests with their own identity, no signing and no hooks; the fixtures that commit or tag use it, and one without an identity fails in every test run, as on the runner | `test` | `ci/tests/test_git_env_isolation.py` |
| `hooks/text_rules.py` | keine Gedankenstriche (U+2013, U+2014) in Textdateien | pre-commit, `lint` | `hooks/tests/` |
| `hooks/secret_formats.py` | no value in the Stripe webhook secret format `whsec_` (16 or more base64 characters) that GitHub secret scanning reports; other GitHub patterns are not covered (docs/quality/gates.md); examples stay short of the format, tests assemble their samples at runtime | pre-commit, `lint`, `test` | `hooks/tests/` |
| `hooks/commit_msg.py` | Conventional Commits und DCO-Sign-off | commit-msg, `commitlint` | `hooks/tests/` |
| `denylist/` | White-Label-Scanner mit privater Liste und öffentlichen Regeln ([README](denylist/README.md)) | `denylist`, `denylist-history` | `denylist/tests/` |
| `docs/adr_index.py` | ADR-Index erzeugen und prüfen | `adr-index`, `docs` | `docs/tests/` |
| `docs/config_reference.py` | Abschnitt 4 der Konfigurationsreferenz aus den Kapiteltabellen erzeugen und prüfen, doppelte Namen ablehnen | `config-reference`, `docs` | `docs/tests/` |
| `docs/problem_code_refs.py` | jedes Paar aus Status und Problem-Code in Markdown und API-Dateien steht so in `x-problem-codes` (06 Abschnitt 5 Punkt 7) | `docs` | `docs/tests/` |
| `docs/brief_deps.py` | Brief-Index, Wellen, Graph, Spalte "Abhängig von" des Plans und Kopfzeilen der Briefs gleich halten; kein Brief ruft ein `make`-Ziel eines Tickets außerhalb seiner Abhängigkeiten; `--write` erzeugt Wellen und Graph neu | `docs` | `docs/tests/` |
| `docs/register_ids.py` | jede ID in der ersten Spalte einer Tabelle in `docs/**/*.md` (zum Beispiel `R-AG-22`, `T-SC-05`, `ID-T01`, `D-W-01`, `RK-82`) im ganzen Repository genau einmal definiert (D-W-60); jede Tabellenzeile hat so viele Zellen wie ihr Kopf (D-W-71); Übersicht und Abschnitte der offenen Fragen nennen dieselben IDs | `docs` | `docs/tests/` |
| `docs/risk_ranking.py` | die Tabelle "Die fünf größten Risiken" in `docs/RISKS.md` folgt dem ersten Kriterium: gleiche Wahrscheinlichkeit und Auswirkung wie im Register, Ränge ohne Lücke, Bewertung steigt nach unten nie an, kein Risiko außerhalb der Tabelle höher bewertet als ihr letzter Rang (D-W-78), die Liste "Hinter Rang" nennt genau die Risiken außerhalb der Tabelle mit der Bewertung des letzten Rangs (D-W-79) | `docs` | `docs/tests/` |
| `docs/render_diagrams.py` | jedes Mermaid-Diagramm rendert mit dem Design-Thema, Seitenverhältnis und Token-Farben; `--write` rendert die `.svg` neu | `diagrams` | `docs/tests/` |
| `docs/erd_check.py` | ERDs in `docs/architecture/` gegen 03: jede Tabelle genau einmal vollständig, jeder Fremdschlüssel gezeichnet | `diagrams` | `docs/tests/` |
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
