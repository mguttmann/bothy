# Push-Disziplin

Ziel: Fehler fallen vor dem Push auf. `main` enthält nur fertige, geprüfte Features, je
Feature einen Commit, signiert und linear.

## Ablauf

1. **Branch.** Von aktuellem `main` einen Branch `<type>/<thema>` anlegen. Nie auf `main`
   arbeiten.
2. **Arbeiten und committen.** Kleine Commits sind erlaubt, auch `fixup!`-Commits; der
   pre-commit-Hook prüft jede Änderung schnell, der commit-msg-Hook Format und DCO
   (`git commit -s`).
3. **Vor dem Push zusammenfassen.** Auf `main` rebasen, Zwischenstände zu sinnvollen
   Conventional Commits zusammenfassen (`git rebase -i main` im Terminal). Kein Commit
   heißt "fix CI", "typo" oder "wip".
4. **Lokales Review.** `make review`: optional CodeRabbit CLI (nur mit
   `BOTHY_CODERABBIT=1`, siehe unten). Am Ende einer Etappe zusätzlich das Red-Team-Gate
   ([red-team-gate.md](red-team-gate.md)).
5. **Push.** Der pre-push-Hook führt `make check` aus. Rot heißt: beheben, nicht umgehen.
   `git push --no-verify` ist ein Prozessfehler.
6. **Pull Request.** Ein Pull Request je fertigem Feature. Der Titel ist eine
   Conventional-Commit-Kopfzeile, weil er beim Merge die Kopfzeile des Commits auf `main`
   wird; CI prüft ihn (`PR_TITLE` in `make commitlint`).
7. **CI.** `ci.yml` führt dieselben make-Ziele aus. Einziger Pflicht-Check: `ci-ok`.
8. **Merge.** "Squash and merge". GitHub signiert den entstehenden Commit; die
   `Signed-off-by`-Zeilen der Einzel-Commits stehen im Commit-Text. `main` bleibt linear.

## Schutz von `main` (Ruleset)

| Regel | Einstellung | Grund |
|---|---|---|
| Require a pull request before merging | an; erlaubte Merge-Art nur Squash | keine direkten Pushes, ein Commit je Feature |
| Require status checks to pass | `ci-ok` | ein Check fasst alle Jobs zusammen; übersprungene Komponenten-Jobs blockieren nicht |
| Require linear history | an | lesbare Historie, einfache Bisects |
| Require signed commits | an | Herkunft jedes Commits auf `main` |
| Block force pushes | an | Historie bleibt unveränderlich |
| Restrict deletions | an | `main` kann nicht gelöscht werden |

Begründung für Squash statt Rebase-Merge: Beim Rebase-Merge erzeugt GitHub neue Commits,
die es nicht signieren kann; signierte Commits und Rebase-Merge über die Oberfläche
schließen sich damit aus
(https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/about-merge-methods-on-github,
abgerufen 2026-10-10) [belegt]. Den Squash-Commit signiert GitHub
(https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets,
abgerufen 2026-10-10) [belegt]. Rulesets sind für private Repositories ab GitHub Pro
verfügbar [belegt, Recherche-Quelle
https://raw.githubusercontent.com/github/docs/main/data/reusables/gated-features/repo-rules.md,
abgerufen 2026-10-10]. Die Regel "Required reviewers" für Teams gibt es in persönlichen
Repositories nicht; im Ein-Personen-Betrieb ersetzt das Red-Team-Gate das Zweitreview.

## Fehler nach dem Push

Ein roter CI-Lauf nach grünem `make check` bedeutet: lokal und CI sind nicht gleich. Das
ist ein Fehler der Werkzeugkette, kein Pech. Ablauf:

1. Ursache bestimmen und im Pull Request notieren.
2. Die Lücke lokal schließen (Gate, Test, Pin), im selben oder einem eigenen Pull Request.
3. Ein Commit "fix CI" oder "typo" auf `main` ist ein Prozessfehler: kurze Analyse als
   Eintrag in `docs/DECISIONS.md` (Datum, Ursache, neue Regel).

Zulässig sind Befunde nach dem Push nur von Prüfern, die lokal nicht laufen können:
SonarQube Cloud (zum Beispiel Dart-Regeln) und CodeRabbit. Jeder solche Befund wird zu
einer lokalen Regel: Semgrep-Regel, Test, Linter-Einstellung oder Profil-Regel in
`quality/sonar/`.

## CodeRabbit CLI und Datenabfluss

Die CodeRabbit CLI prüft lokale Änderungen über den CodeRabbit-Dienst; die
Dokumentation nennt eine Prüfung der Erreichbarkeit des Backends (`cr doctor`) und keine
rein lokale Ausführung (https://docs.coderabbit.ai/cli/overview, abgerufen 2026-10-10)
[abgeleitet]. Der Diff verlässt also den Rechner. Darum ist sie aus, bis
`BOTHY_CODERABBIT=1` gesetzt ist. Im privaten Zustand entscheidet der Owner, ob privater
Code an den Dienst gehen darf (offene Frage). Im öffentlichen Zustand ist der Code
ohnehin öffentlich; CodeRabbit bewertet dann zusätzlich jeden Pull Request
(`.coderabbit.yaml`). Die Gratisstufe der CLI ist laut Hersteller begrenzt
("Basic static analysis with limited daily usage", gleiche Quelle) [belegt].
