# 23 Qualität, Repository und Lieferkette

Status: Entwurf M0. Dieses Kapitel legt fest, welche Prüfungen jede Änderung besteht, wie
sie lokal und in CI laufen und wie das Repository vom privaten in den öffentlichen Zustand
wechselt. Die Anleitungen dazu stehen unter [docs/quality/](../quality/README.md).
Entscheidungen: ADR [0038](../adr/0038-lokale-gates-zuerst-ein-befehl-identisch-zu-ci.md),
[0039](../adr/0039-runner-strategie-privat-oeffentlich-und-release-ausnahmen.md),
[0040](../adr/0040-sonarqube-lokal-und-cloud-mit-gleichem-profil-und-gate.md),
[0041](../adr/0041-abhaengigkeits-updates-mit-renovate.md),
[0042](../adr/0042-deny-list-scanner-mit-liste-aus-dem-ops-repo.md),
[0045](../adr/0045-dokumentations-site-generator.md).

## 1. Grundsätze

1. **Ein Befehl.** `make check` führt jedes Gate aus. CI führt dieselben make-Ziele aus,
   aufgeteilt auf drei Jobs (`gates`, `denylist`, `sonar`). Was lokal grün ist, ist in CI
   grün; einzige gewollte Ausnahme ist die Pfadauswahl der Komponenten (Abschnitt 4).
2. **Lokal zuerst.** Fehler werden vor dem Push gefunden. Der pre-push-Hook ruft
   `make check` auf. CI ist die Bestätigung, nicht der Ort der Fehlersuche.
3. **Fail closed.** Fehlt ein Werkzeug, eine Liste oder ein Token, schlägt das Gate fehl.
   Ein Gate darf nur dann mit einer `SKIP`-Zeile überspringen, wenn die geprüfte
   Komponente noch keinen Code hat. Diese Entscheidung trifft der Makefile aus den
   getrackten Dateien, nicht der Entwickler. Ausnahme sind nur die ausdrücklichen Schalter
   für Beiträge aus Forks (R-QA-02).
4. **Gepinnte Werkzeuge.** Jedes Werkzeug hat eine feste Version mit Prüfsumme
   (`tools/uv.lock`, `tools/node/package-lock.json`, `tools/bin/tools.lock.json`,
   Image-Digests, Action-SHAs). Updates kommen als Pull Request von Renovate.
5. **Zahlen nur als Namen.** Schwellen und Fristen dieses Kapitels sind
   Konfigurationswerte (Abschnitt 11). Ein Test (`tools/docs/tests/test_config_values.py`)
   hält die Tabelle, die Werkzeug-Konfiguration und die Community-Dateien gleich.

## 2. Gates

Die vollständige Liste mit Werkzeug, Schwelle und Lauf-Ort steht in
[gates.md](../quality/gates.md). Übersicht:

| Gruppe | make-Ziele | Prüft |
|---|---|---|
| Code | `fmt-check`, `codegen-check`, `lint`, `typecheck`, `arch`, `register-check`, `migrations-lint`, `test`, `coverage` | Format, erzeugter Code ohne Drift, Linter, Typen, Modulgrenzen der Go-Module, Modulregister gleich Kern-Vertrag und Doku, Migrationsregeln, Tests mit Abdeckung je Komponente |
| Commits | `commitlint` | Conventional Commits und DCO-Sign-off der neuen Commits |
| Doku | `docs`, `linkcheck`, `diagrams`, `catalog-check`, `api-lint`, `api-clients`, `tokens` | Site-Build strikt, Links und Anker, Diagramme, Aktionskatalog und seine erzeugten Kopien, API-Verträge, generierte Clients, Design-Tokens |
| CI-Regeln | `runners-check`, `labels-check` | Runner-Weiche, Trigger, SHA-Pinning, Timeouts, Rechte; Labels |
| Sicherheit | `gitleaks`, `denylist`, `semgrep`, `trivy`, `osv` | Secrets, White Label, Projektregeln, Fehlkonfiguration, bekannte Schwachstellen |
| Recht | `licenses` | Lizenztext, SPDX-Kopfzeilen, Lizenzen der Abhängigkeiten |
| Analyse | `sonar` | SonarQube mit gemeinsamem Profil und Gate |

Am Release kommen hinzu: SBOM, Provenance, Signatur, Image-Scan (`trivy-image`) und der
Portabilitätstest (`portability`). Am Abschluss jeder Etappe und im Release läuft `make perf`
mit den Leistungszielen aus 02 ([ADR 0063](../adr/0063-leistungs-gate-fuer-das-ereignis-log.md),
PLAN P-07). Siehe [toolchain.md](../quality/toolchain.md).

## 3. Komponenten-Vertrag

Der Root-Makefile entdeckt Komponenten an ihren Manifesten: Go-Module unter `services/`,
Dart-Pakete unter `apps/` und `packages/`, Python-Projekte unter `sdk/python/` und
`plugins/`, TypeScript-Projekte unter `sdk/typescript/` und `plugins/`. Für die Gates
gilt je Art:

| Art | Erwartet von der Komponente |
|---|---|
| Go-Modul | `go.mod` und `go.sum`; Tests mit `go test`; golangci-lint-Konfiguration im Modul (Abhängigkeitsregeln: Kapitel 02); `.go-arch-lint.yml`, erzeugt aus `architecture/modules.yaml` (Gate `arch`); Migrationen unter `migrations/` mit der Prüfsummen-Datei `migrations.sha256` daneben (Gate `migrations-lint`, sobald es Migrationen gibt); Lasttests unter `perf/` mit dem Build-Tag `perf` (`make perf`); die Abdeckung zählt `test/**` und `perf/**` nicht mit, weil sie selbst Testcode sind (02, Abschnitt 6) |
| Dart-Paket | `pubspec.yaml`, Tests unter `test/`; `custom_lint` als dev-Abhängigkeit, wenn eigene Regeln gelten |
| Python-Projekt | `pyproject.toml` mit uv-Lock; ruff, mypy und pytest in der dev-Gruppe; Abdeckungsschwelle in der eigenen pytest-Konfiguration |
| TypeScript-Projekt | `package.json` mit den Skripten `format`, `format:check`, `lint`, `typecheck`, `test` (Abdeckungsschwelle im Test-Runner) |
| Release | Makefile im Komponenten-Ordner mit dem Ziel `release-build` (Argumente `VERSION`, `OUT`); Apps zusätzlich `release-apple` und `release-windows` |
| Generierter Client | Makefile mit dem Ziel `generate-check` (aus `docs/api/` erzeugen, kompilieren, Abweichung vom eingecheckten Stand meldet Fehler) |
| Design-Tokens | `tools/design-tokens/Makefile` mit dem Ziel `check` |
| Portabilität | ausführbares `infra/compose/smoke.sh` |

Fehlt ein Teil, obwohl die Komponente Code hat, schlägt das Gate fehl. Der Teil "Release" gilt
ab der Release-Kette der Komponente, die das Ziel `release-build` und seinen Check mitbringt
(Server: [PLAN.md](../PLAN.md), Ticket M7-10); vorher gibt es kein Artefakt, das er bauen müsste.

## 4. CI

- Pull Requests auf `main` lösen `ci.yml` aus. Der Job `plan` bestimmt die betroffenen
  Komponenten aus den geänderten Pfaden (`tools/ci/components.py`). Gates aller anderen
  Komponenten überspringen; die repository-weiten Gates laufen immer. Ein Pfad außerhalb
  bekannter Bereiche wählt alle Komponenten.
- Einziger Pflicht-Check ist `ci-ok`; er fasst alle Jobs zusammen.
- Kein `push`-Trigger auf Branches, kein `pull_request_target`, `permissions: contents: read`
  als Standard, `timeout-minutes` an jedem Job, Actions per voller Commit-SHA.
  `tools/ci/check_runners.py` setzt das durch (Gate `runners-check`), actionlint und
  zizmor prüfen zusätzlich.
- Runner: im privaten Zustand der self-hosted Runner, im öffentlichen Zustand
  GitHub-gehostete Runner, gewählt über `github.event.repository.private`
  (ADR 0039, [ci-budget.md](../quality/ci-budget.md),
  [self-hosted-runner.md](../quality/self-hosted-runner.md)).

## 5. Push-Disziplin

Ein Pull Request je fertigem Feature, Commits vorher lokal zusammengefasst, `main`
geschützt mit linearer Historie, Pflicht-Check `ci-ok` und signierten Commits. Ein
Folge-Commit "fix CI" oder "typo" auf `main` gilt als Prozessfehler und bekommt eine
Analyse. Fehler nach dem Push sind nur zulässig, wenn ein externer Prüfer (SonarQube
Cloud, CodeRabbit) etwas findet, das lokal nicht sichtbar war; daraus wird eine lokale
Regel. Details: [push-discipline.md](../quality/push-discipline.md).

## 6. White Label

Das Produkt-Repository enthält nichts Instanzspezifisches. Der Deny-List-Scanner
(`tools/denylist/`) prüft getrackte Inhalte, Dateinamen, Commit-Nachrichten,
Commit-Identitäten, Branch- und Tag-Namen; `--history` prüft jedes je gespeicherte
Objekt. Die private Liste liegt außerhalb des Repositorys; ohne sie schlägt der Scan fehl.
Öffentliche, generische Regeln (`tools/denylist/generic.rules`) laufen immer, auch für
Fork-Beiträge ohne Zugriff auf die Liste. Die Ausgabe nennt nur Fundort und Regelzeile,
nie den Treffer. ADR 0042.

## 7. SonarQube

Ein Profil "Bothy way" je Sprache erbt vom eingebauten Profil und aktiviert nur die
Regeln aus `quality/sonar/quality-profile-<sprache>.xml` zusätzlich. Das Gate "Bothy way"
(`quality/sonar/quality-gate.json`) nutzt nur Metriken, die es in beiden Modi von
SonarQube gibt. `tools/sonarqube/quality_sync.py` spielt beides in den lokalen Community
Build und in SonarQube Cloud ein und prüft die Gleichheit. Dart analysiert nur
SonarQube Cloud; lokal deckt `flutter analyze` mit `custom_lint` Dart ab. ADR 0040,
[sonarqube-local.md](../quality/sonarqube-local.md).

## 8. Lieferkette

- Actions per voller Commit-SHA, Images per Digest, Release-Binärdateien per sha256.
- Renovate schlägt Updates erst nach `deps.min_release_age` vor und gruppiert sie.
  Sicherheits-Updates kommen sofort.
- Trivy bleibt auf der Version, die das Hersteller-Advisory nach dem Vorfall vom
  2026-03-19 als sicher nennt (GHSA-69fq-xp46-6x23,
  https://github.com/aquasecurity/trivy/security/advisories/GHSA-69fq-xp46-6x23,
  abgerufen 2026-10-10) [belegt]. Ein Wechsel folgt der Prüfung in
  [toolchain.md](../quality/toolchain.md).
- Releases: SBOM (SPDX), Prüfsummen; im öffentlichen Zustand zusätzlich signierte
  Build-Provenance über GitHub Artifact Attestations. Images tragen Buildx-Provenance und
  SBOM im Registry-Eintrag. Ein Server-Image wird zuerst nur per Digest gepusht, mit Trivy
  geprüft und erst danach mit seiner Version getaggt.

## 9. Red-Team-Gate

Jede Etappe endet mit einem unabhängigen, nur lesenden Red-Team-Review mit Urteil JA
oder NEIN. Bei NEIN wird behoben und das Delta erneut geprüft; das Bedrohungsmodell
(Kapitel 19) wird nachgezogen. Ablauf: [red-team-gate.md](../quality/red-team-gate.md).

## 10. Rulings

| Nr | Regel | Grund |
|---|---|---|
| R-QA-01 | `make check` = `gates` + `denylist` + `sonar`; CI-Jobs rufen genau diese Ziele auf. | Gleichheit lokal und CI entsteht durch Konstruktion, nicht durch Abgleich zweier Listen. |
| R-QA-02 | Ein Gate überspringt nur, wenn die Komponente keinen Code hat; fehlende Werkzeuge sind Fehler. Einzige Ausnahme sind die ausdrücklichen Schalter für Beiträge aus Forks (`DENYLIST_FLAGS=--allow-missing`, `SONAR_SKIP=1`); CI prüft Fork-Pull-Requests genauso, Maintainer prüfen vor dem Merge voll. | Ein stilles Überspringen wäre ein grünes Gate, das nichts geprüft hat; Beitragende von außen haben weder die private Liste noch einen SonarQube-Server. |
| R-QA-03 | Runner-Wahl über `github.event.repository.private` statt über eine Variable. | Wird das Repository öffentlich, wechseln die Jobs sofort auf GitHub-gehostete Runner, auch wenn jemand eine Variable vergisst. |
| R-QA-04 | Geplante Workflows laufen nur mit der Öffentlich-Bedingung; ihr Ereignis trägt kein Repository-Objekt, darum dort zusätzlich `vars.BOTHY_VISIBILITY`. | Ein geplanter Lauf im privaten Zustand würde sonst GitHub-gehostete Minuten verbrauchen. |
| R-QA-05 | Semgrep läuft nur mit eigenen Projektregeln, ohne Registry-Regelsätze. | Registry-Regeln ändern sich ohne Versionsbezug und haben eine eigene Lizenz; allgemeine Statikanalyse leisten SonarQube, CodeQL, golangci-lint und bandit. |
| R-QA-06 | SonarQube analysiert Produkt-Code; die Repository-Werkzeuge prüfen ruff, mypy (strict), bandit und pytest. | Heute gibt es nur Werkzeug-Code; ein laufender SonarQube-Server wäre sonst Pflicht für jede Planungsänderung. |
| R-QA-07 | DCO-Pflicht für Commits ab dem ersten Push gegen ein `origin/main`; ohne Remote prüft der commit-msg-Hook jeden neuen Commit. | Die Planungs-Commits entstanden vor den Gates; neue Beiträge brauchen den Herkunftsnachweis. |
| R-QA-08 | Der Deny-List-Scan prüft ohne Remote alle Commits von `HEAD`. | Der erste Push ist der gefährlichste; Identitäten alter Commits müssen vorher stimmen. |
| R-QA-09 | Links ins Internet sind kein Gate (`linkcheck-external` manuell). | Fremde Server machen ein Gate sonst nicht reproduzierbar. Lokale Links und Anker prüft `linkcheck` streng. |
| R-QA-10 | Keine `.sonarcloud.properties`; SonarQube Cloud analysiert über CI, die automatische Analyse bleibt aus. | Abdeckung und Pull-Request-Dekoration brauchen die CI-Analyse; beide Wege zugleich stören sich. |
| R-QA-11 | Testserver, die große Welten über die öffentliche API aufbauen (Vertrags-, Leck-, Kanarien-Tests, Vorbereitung der Lasttests), heben `ratelimit.*` über ihre Konfiguration an; gemessene Lasttests und Tests der Rate-Limits laufen mit den Standardwerten. | Aufbau über die API prüft die echten Wege, scheitert aber an Limits für Menschen; Messungen und Limit-Tests müssen die ausgelieferten Werte sehen. |

## 11. Konfigurationswerte dieses Kapitels

| Name | Standard | Einheit | Bedeutung | Gehalten durch |
|---|---|---|---|---|
| `quality.coverage.tools_min_percent` | `90` | Prozent | Mindestabdeckung der Repository-Werkzeuge | `tools/pyproject.toml` (`fail_under`) |
| `quality.coverage.go_min_percent` | `80` | Prozent | Mindestabdeckung je Go-Modul | `Makefile` (`COVERAGE_GO_MIN`) |
| `quality.coverage.dart_min_percent` | `80` | Prozent | Mindestabdeckung je Dart-Paket | `Makefile` (`COVERAGE_DART_MIN`) |
| `quality.perf.timeout` | `60m` | Dauer | Zeitlimit von `make perf` je Go-Modul | `Makefile` (`PERF_TIMEOUT`) |
| `quality.sonar.new_issues_max` | `0` | Anzahl | Neue Befunde im neuen Code | `quality/sonar/quality-gate.json` |
| `quality.sonar.new_hotspots_reviewed_min_percent` | `100` | Prozent | Geprüfte Security Hotspots im neuen Code | `quality/sonar/quality-gate.json` |
| `quality.sonar.new_coverage_min_percent` | `80` | Prozent | Abdeckung des neuen Codes | `quality/sonar/quality-gate.json` |
| `quality.sonar.new_duplication_max_percent` | `3` | Prozent | Duplizierte Zeilen im neuen Code | `quality/sonar/quality-gate.json` |
| `quality.sonar.local_port` | `9000` | Port | Port des lokalen SonarQube auf `127.0.0.1` | `tools/sonarqube/compose.yaml`, `Makefile` (`SONAR_HOST_URL`) |
| `quality.sonar.startup_timeout_s` | `300` | Sekunden | Wartezeit von `make sonar-up` auf den Status UP | `Makefile` (`SONAR_STARTUP_TIMEOUT_S`) |
| `commit.header_max_length` | `100` | Zeichen | Längste Commit-Kopfzeile | `tools/hooks/commit_msg.py` |
| `repo.max_file_kb` | `1024` | KiB | Größte Datei im Repository | `.pre-commit-config.yaml` |
| `license.spdx_header_lines` | `5` | Zeilen | Bereich, in dem die SPDX-Kopfzeile stehen muss | `tools/ci/license_check.py` |
| `deps.min_release_age` | `7 days` | Dauer | Wartezeit, bevor Renovate eine neue Version vorschlägt | `renovate.json` |
| `security.report_ack_target` | `3` | Werktage | Eingangsbestätigung einer Schwachstellen-Meldung | `SECURITY.md` |
| `security.assessment_target` | `10` | Werktage | Erste Bewertung mit Schweregrad | `SECURITY.md` |
| `security.fix_target_critical` | `30` | Tage | Veröffentlichte Korrektur bei kritischem Schweregrad | `SECURITY.md` |
| `security.fix_target_high` | `60` | Tage | Veröffentlichte Korrektur bei hohem Schweregrad | `SECURITY.md` |
| `security.disclosure_deadline` | `90` | Tage | Späteste Veröffentlichung | `SECURITY.md` |
| `triage.first_response_target` | `14` | Tage | Erste Antwort auf ein neues Issue | `GOVERNANCE.md` |
| `triage.needs_info_close_after` | `30` | Tage | Schließen ohne Antwort bei `status: needs-info` | `GOVERNANCE.md` |
