---
status: accepted
date: 2026-10-11
decision-makers: Maintainer
---

# Nur GitHub-gehostete Runner, öffentlich ab dem ersten Push

## Context and Problem Statement

Beiträge sollen der Reihe nach als Pull Requests in ein öffentliches Repository fließen, geprüft
von CI, SonarQube Cloud und CodeRabbit. Für öffentliche Repositories gilt: "Use of the standard
GitHub-hosted runners is free and unlimited on public repositories"
(https://docs.github.com/en/actions/reference/runners/github-hosted-runners, abgerufen
2026-10-11) [belegt]. Ein Runner, den die Maintainer selbst betreiben, würde in einem
öffentlichen Repository Code aus fremden Pull Requests auf ihren Rechnern ausführen; GitHub rät
davon für öffentliche Repositories ab
(https://docs.github.com/en/actions/reference/security/secure-use, abgerufen 2026-10-10)
[belegt]. Auf welchen Runnern laufen die Workflows, und ab wann ist das Repository öffentlich?

## Decision Drivers

* Kein Pull Request führt Code auf einem Rechner der Maintainer aus.
* Ein Satz Workflow-Dateien ohne Weiche, gültig vom ersten Push an.
* CI auf den Standard-Runnern von GitHub, die für öffentliche Repositories frei sind.
* Prüfbar durch ein Werkzeug, nicht nur durch Review.

## Considered Options

* Nur GitHub-gehostete Runner, Repository öffentlich ab dem ersten Push
* Selbst betriebene Runner, solange das Repository privat ist, danach GitHub-gehostete Runner
  (Weiche in `runs-on`)
* Immer selbst betriebene Runner

## Decision Outcome

Gewählt: "Nur GitHub-gehostete Runner, Repository öffentlich ab dem ersten Push", weil kein
Runner der Maintainer existiert, den fremder Code erreichen kann, und die Workflows keine Weiche
brauchen. Es gibt keine selbst betriebenen Runner. Jeder Job nennt ein festes Label eines
Standard-Runners: `ubuntu-24.04`, für den Portabilitätstest zusätzlich `ubuntu-24.04-arm`, für
die App-Builds `macos-15` und `windows-2025`. Das Repository wird direkt nach dem ersten Push
öffentlich geschaltet ([public-switch.md](../quality/public-switch.md)).

Ergänzende Regeln:

1. **Öffentlich-Jobs.** CodeQL, Scorecard und Pages laufen nur, solange das Repository
   öffentlich ist (Job-Bedingung `github.event.repository.visibility == 'public'`): Code
   Scanning und Scorecard setzen ein öffentliches Repository voraus, und Pages machte auch aus
   einem privaten Repository eine öffentliche Website. Geplante Läufe tragen kein
   Repository-Objekt in der Nutzlast [abgeleitet aus der Ereignis-Referenz, die für `schedule`
   keine Nutzlast nennt,
   https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows,
   abgerufen 2026-10-10]; für sie gilt zusätzlich `vars.BOTHY_VISIBILITY == 'public'`, gesetzt
   beim Öffentlich-Schalten. Fehlt die Variable, laufen geplante Jobs nicht.
2. **Release-Workflows.** `release.yml` startet nur per Tag-Push, `portability.yml` nur als
   Aufruf aus `release.yml` nach `publish` (`workflow_call`) oder per Hand mit einem Produkt-Tag
   (`workflow_dispatch`, D-W-59). Einen Release-Workflow ruft nur `release.yml` auf, aus einem
   Job und ohne Matrix (D-W-73): Der Portabilitätstest prüft das Release, das derselbe Lauf
   veröffentlicht hat, und zwar einmal.
3. **Feste Labels.** Kein Label `*-latest` (es wechselt das Image ohne Änderung im Repository),
   keine Larger Runner (sie gehören nicht zu den Standard-Runnern), keine Liste, keine Gruppe und
   kein Ausdruck außer `${{ matrix.<key> }}` über eine statische Matrix aus festen Labels.
4. **Release-Jobs.** Signier-Secrets liegen nur in den Umgebungen `release`, `release-apple` und
   `release-windows`. Ein Job mit einer dieser Umgebungen nutzt Ausgaben anderer Jobs nur im
   `if:` des Jobs oder eines Schritts (D-W-72), löst den Tag selbst neu auf und vergleicht, und
   holt Artefakte nur per ID (D-W-52). Jeder Standard-Runner außer den Ein-Kern-Runnern ist
   eine neue VM ("each GitHub-hosted runner is a new virtual machine (VM) hosted by GitHub",
   gleiche Quelle wie oben, abgerufen 2026-10-11) [belegt]; ein Job übernimmt also keinen
   Zustand eines anderen Jobs außer über Caches, Artefakte und Ausgaben.
5. **Scan nach dem Merge.** Einzige Ausnahme von "kein `push` auf Branches" ist
   `denylist-main.yml`: ein `push` auf genau `main`, der die gepushten Commits mit der
   privaten Deny-Liste prüft (D-W-34).

### Consequences

* Gut, weil kein Pull Request Code auf einem Rechner der Maintainer ausführt.
* Gut, weil die Workflows einfach sind: ein festes Label je Job, keine Weiche, keine Einrichtung
  und keine Entfernung von Runnern.
* Gut, weil CI, CodeQL, Scorecard, SonarQube Cloud und CodeRabbit vom ersten Pull Request an
  laufen.
* Schlecht, weil alles im Repository vom ersten Push an öffentlich ist; der Stand muss vorher
  vollständig geprüft sein ([public-switch.md](../quality/public-switch.md), Teil A).
* Schlecht, weil vor dem ersten Push keine CI läuft; bis dahin prüft `make check` lokal
  (ADR 0038).
* Schlecht, weil eine neue Image-Version von GitHub eine Änderung der Workflows braucht; dafür
  ändert sich kein Lauf ohne Änderung im Repository.

### Confirmation

`tools/ci/check_runners.py` (Gate `runners-check`, Tests in `tools/ci/tests/`) lehnt jeden Job
ab, dessen Runner kein festes GitHub-gehostetes Label ist (Regel `runner-not-github-hosted`),
dazu geplante Workflows ohne Öffentlich-Bedingung (`schedule-without-public-guard`),
Release-Workflows mit anderen als ihren eigenen Triggern (`release-trigger`), Aufrufe eines
Release-Workflows außerhalb eines Jobs von `release.yml` ohne Matrix (`reusable-release-caller`,
D-W-73), `push` auf Branches außer `main` in `denylist-main.yml` (`push-on-branch`), in Jobs mit
Umgebung `release*` Ausgaben anderer Jobs außerhalb des `if:` (Erlaubnisliste D-W-61, D-W-72,
Regel `release-untrusted-output`) und Artefakte nicht per ID (`release-artifact-by-id`).

## Pros and Cons of the Options

### Nur GitHub-gehostete Runner, öffentlich ab dem ersten Push

* Gut, weil einfach und sicher.
* Gut, weil die Standard-Runner in öffentlichen Repositories frei sind (siehe oben).
* Schlecht, weil vor der Veröffentlichung keine CI läuft.

### Selbst betriebene Runner im privaten Zustand, danach GitHub-gehostete Runner

* Gut, weil CI schon vor der Veröffentlichung liefe.
* Schlecht, weil eine Weiche in jedem Job steht, Runner eingerichtet, getrennt und vor dem
  Öffentlich-Schalten entfernt werden müssen und ein vergessener Schritt fremden Code auf einen
  Rechner der Maintainer bringt.

### Immer selbst betriebene Runner

* Schlecht, weil im öffentlichen Repository unsicher (siehe oben).

## More Information

Ablauf der Veröffentlichung: [public-switch.md](../quality/public-switch.md). Lokale Gates:
[ADR 0038](0038-lokale-gates-zuerst-ein-befehl-identisch-zu-ci.md).
