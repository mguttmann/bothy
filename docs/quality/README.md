# Qualität und Repository

Wie jede Änderung geprüft wird, bevor sie `main` erreicht, und wie das Repository vom
privaten in den öffentlichen Zustand wechselt. Die verbindlichen Regeln stehen in
Kapitel [23 Qualität](../spec/23-qualitaet.md); diese Seiten sind die Anleitungen dazu.

| Seite | Inhalt |
|---|---|
| [gates.md](gates.md) | Jedes Gate mit Werkzeug, Schwelle und Lauf-Ort; Hooks; Komponenten-Erkennung |
| [toolchain.md](toolchain.md) | Jedes Werkzeug mit Version, Zweck, Lizenz und Grund; Update-Regeln |
| [push-discipline.md](push-discipline.md) | Vom Branch zum Merge: Commits, Push, Pull Request, Prozessfehler |
| [red-team-gate.md](red-team-gate.md) | Unabhängiges Review je Etappe mit Urteil JA oder NEIN |
| [sonarqube-local.md](sonarqube-local.md) | Lokaler SonarQube, Token, Profil und Gate, Abgleich mit der Cloud |
| [ci-budget.md](ci-budget.md) | Kosten von GitHub Actions, Regeln für ein Budget nahe null, Messung |
| [self-hosted-runner.md](self-hosted-runner.md) | Einrichtung des Runners für den privaten Zustand und seine Entfernung |
| [public-switch.md](public-switch.md) | Checkliste vor dem Öffentlich-Schalten |
| [licenses.md](licenses.md) | Erlaubte Lizenzen, Lizenz-Gate, SBOM und Prüfung von Releases |

Kurzfassung für den Alltag:

```sh
make bootstrap   # einmal je Klon
make check       # vor jedem Push (der pre-push-Hook tut es automatisch)
make help        # alle Ziele
```
