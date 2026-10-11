---
status: accepted
date: 2026-10-10
decision-makers: Maintainer
---

# Lokale Gates zuerst: ein Befehl identisch zu CI

## Context and Problem Statement

Jeder Fehler soll vor dem Push auffallen: "fix CI"-Commits auf `main` gelten als
Prozessfehler. Wie organisieren wir die Prüfungen so, dass lokal und in CI dasselbe
passiert?

## Decision Drivers

* Lokal grün heißt CI grün; keine zweite Liste von Prüfungen, die auseinanderläuft.
* Schnelle Rückmeldung beim Commit, vollständige Prüfung vor dem Push.
* Reproduzierbar: gleiche Werkzeugversionen auf jedem Rechner und in CI.

## Considered Options

* `make check` als einziger Gate-Befehl, Hooks und CI rufen dieselben make-Ziele auf
* CI zuerst: GitHub-gehostete Jobs bei jedem Push, lokal nur Teilprüfungen
* Nur lokal, ohne CI
* pre-commit als einziges Gate-System, CI ruft `pre-commit run --all-files`

## Decision Outcome

Gewählt: "`make check` als einziger Gate-Befehl", weil die Gleichheit von lokal und CI
dann durch Konstruktion entsteht: `check` besteht aus `gates`, `denylist` und `sonar`,
und die drei CI-Jobs rufen genau diese Ziele auf. pre-commit liefert die schnellen
Datei-Prüfungen und ruft vor dem Push `make check`; `make lint` führt die pre-commit-Hooks
über alle Dateien aus, damit es keine Prüfung nur im Hook gibt.

### Consequences

* Gut, weil ein Entwickler genau einen Befehl kennt und CI nichts Neues findet.
* Gut, weil Werkzeuge gepinnt sind (uv-Lock, npm-Lock, sha256-Lock für Binärdateien).
* Schlecht, weil `make check` vor jedem Push Minuten dauert. Das ist gewollt: Pushes sind
  selten (ein Pull Request je fertigem Feature).
* Schlecht, weil Komponenten ohne Code heute überspringen. Das ist sichtbar (`SKIP`-Zeile)
  und endet automatisch mit dem ersten Code.

### Confirmation

* `Makefile`: `check: gates denylist sonar`; `.github/workflows/ci.yml` ruft
  `make gates`, `make denylist`, `make sonar` auf.
* `.pre-commit-config.yaml`: pre-push-Hook `make check`.
* `tools/ci/check_runners.py` (Gate `runners-check`) verbietet `push`-Trigger auf
  Branches, verlangt `timeout-minutes` und feste GitHub-gehostete Runner (ADR 0039).

## Pros and Cons of the Options

### `make check` als einziger Gate-Befehl

* Gut, weil eine Definition für lokal und CI.
* Gut, weil make überall vorhanden ist und Komponenten-Erkennung im Makefile liegt.
* Neutral, weil Windows-Beitragende WSL brauchen.

### CI zuerst

* Gut, weil nichts lokal installiert werden muss.
* Schlecht, weil Fehler erst nach dem Push auffallen.

### Nur lokal, ohne CI

* Gut, weil einfach.
* Schlecht, weil Beiträge von außen und Releases ungeprüft blieben und kein
  Pflicht-Check `main` schützt.

### pre-commit als einziges System

* Gut, weil ein verbreitetes Werkzeug.
* Schlecht, weil Tests, Doku-Build, SonarQube und Komponenten-Logik schlecht in
  Datei-Hooks passen.

## More Information

Gate-Liste: docs/quality/gates.md. Push-Regeln: docs/quality/push-discipline.md.
