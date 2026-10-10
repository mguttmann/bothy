---
status: accepted
date: 2026-10-10
decision-makers: Maintainer
---

# Runner-Strategie privat/öffentlich und Release-Ausnahmen

## Context and Problem Statement

Im privaten Zustand sollen CI-Jobs auf einem self-hosted Runner laufen, weil gehostete
Minuten kosten. Im öffentlichen Zustand dürfen dort keine self-hosted Runner laufen:
jeder kann per Pull Request Code darauf ausführen. GitHub schreibt dazu "self-hosted
runners should almost never be used for public repositories"
(https://docs.github.com/en/actions/reference/security/secure-use, abgerufen 2026-10-10)
[belegt]. Standard-Runner sind in öffentlichen Repositories kostenlos
(https://docs.github.com/en/billing/concepts/product-billing/github-actions, abgerufen
2026-10-10) [belegt]. Wie wählt ein und derselbe Satz Workflows den richtigen Runner?

## Decision Drivers

* Kein self-hosted Runner, sobald das Repository öffentlich ist, auch wenn jemand einen
  Schritt im Runbook vergisst.
* Gehostete Minuten im privaten Zustand nahe null.
* Ein Satz Workflow-Dateien, keine Parallelpflege.
* Prüfbar durch ein Werkzeug, nicht nur durch Review.

## Considered Options

* Weiche im Ausdruck: `runs-on` wertet `github.event.repository.private` aus
* Weiche über eine Repository-Variable
* Zwei Sätze Workflows, beim Öffentlich-Schalten getauscht
* Immer GitHub-gehostet
* Immer self-hosted

## Decision Outcome

Gewählt: "Weiche im Ausdruck", weil sie aus der Ereignis-Nutzlast des konkreten Laufs
entscheidet und deshalb im Moment des Öffentlich-Schaltens umschaltet:

```yaml
runs-on: ${{ github.event.repository.private && fromJSON('["self-hosted","bothy-ci"]') || 'ubuntu-24.04' }}
```

Ergänzende Regeln:

1. **Öffentlich-Jobs.** CodeQL, Scorecard und Pages laufen nur im öffentlichen Zustand
   (Job-Bedingung `github.event.repository.visibility == 'public'`). Geplante Läufe tragen
   kein Repository-Objekt in der Nutzlast [abgeleitet aus der Ereignis-Referenz, die für
   `schedule` keine Nutzlast nennt,
   https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows,
   abgerufen 2026-10-10]; für sie gilt zusätzlich `vars.BOTHY_VISIBILITY == 'public'`.
   Fehlt die Variable, laufen geplante Jobs nicht: kostenfrei und sicher.
2. **Release-Klasse.** `release.yml` und `portability.yml` dürfen GitHub-gehostete Runner
   ohne Weiche nutzen, aber nur bei Release-Tags (Portabilität zusätzlich manuell): für
   Windows fehlt eigene Hardware, und der Portabilitätstest braucht eine frische
   Linux-Maschine. macOS-Jobs nutzen eine Weiche mit dem Label `macOS`, weil der
   self-hosted Runner ein Mac sein kann.
3. **Runbook.** Vor dem Öffentlich-Schalten wird der self-hosted Runner entfernt
   (docs/quality/public-switch.md). Die Weiche ist die zweite Sicherung, nicht die einzige.

### Consequences

* Gut, weil ein vergessener Runbook-Schritt nicht zu einem öffentlich erreichbaren
  self-hosted Runner führt.
* Gut, weil gehostete Minuten im privaten Zustand nur bei Releases anfallen.
* Schlecht, weil der Ausdruck in jedem Job steht; das Gate prüft jede Kopie.
* Schlecht, weil sich alle Jobs im privaten Zustand einen Runner teilen und nacheinander
  laufen. Für ein Ein-Personen-Repository ist das ausreichend.

### Confirmation

`tools/ci/check_runners.py` (Gate `runners-check`, Tests in `tools/ci/tests/`) lehnt
jeden Job ab, der einen Runner anders wählt: gehostete Labels nur mit Öffentlich-Bedingung
oder in der Release-Klasse, `self-hosted` nur über die Weiche, geplante Workflows nur mit
Öffentlich-Bedingung, Release-Workflows nur mit Tag-Triggern.

## Pros and Cons of the Options

### Weiche im Ausdruck

* Gut, weil sie den tatsächlichen Zustand des Repositorys je Lauf auswertet.
* Neutral, weil der Ausdruck lang ist.

### Repository-Variable

* Gut, weil kurz und zentral.
* Schlecht, weil eine vergessene Variable einen self-hosted Runner im öffentlichen
  Repository lässt.

### Zwei Sätze Workflows

* Gut, weil jede Datei einfach ist.
* Schlecht, weil doppelte Pflege und ein Tausch genau im riskantesten Moment.

### Immer GitHub-gehostet

* Gut, weil einfach und sicher.
* Schlecht, weil das Minutenbudget im privaten Zustand schon erschöpft ist.

### Immer self-hosted

* Gut, weil kostenlos.
* Schlecht, weil im öffentlichen Repository unsicher.

## More Information

Kosten und Messung: docs/quality/ci-budget.md. Einrichtung des Runners:
docs/quality/self-hosted-runner.md.
