# Qualität und Repository

Wie jede Änderung geprüft wird, bevor sie `main` erreicht, und wie das Repository
veröffentlicht wird. Die verbindlichen Regeln stehen in
Kapitel [23 Qualität](../spec/23-qualitaet.md); diese Seiten sind die Anleitungen dazu.

| Seite | Inhalt |
|---|---|
| [gates.md](gates.md) | Jedes Gate mit Werkzeug, Schwelle und Lauf-Ort; Hooks; Komponenten-Erkennung |
| [toolchain.md](toolchain.md) | Jedes Werkzeug mit Version, Zweck, Lizenz und Grund; Update-Regeln |
| [push-discipline.md](push-discipline.md) | Vom Branch zum Merge: Commits, Push, Pull Request, Prozessfehler |
| [red-team-gate.md](red-team-gate.md) | Unabhängiges Review vor jedem Push (Nachweis `.review/<sha>.md`) und je Etappe, Urteil JA oder NEIN |
| [sonarqube-local.md](sonarqube-local.md) | Lokaler SonarQube, Token, Profil und Gate, Abgleich mit der Cloud |
| [public-switch.md](public-switch.md) | Checkliste der Veröffentlichung: Push, sofort öffentlich, SonarQube Cloud und CodeRabbit, Branch-Schutz, danach jedes Ticket als Pull Request |
| [licenses.md](licenses.md) | Erlaubte Lizenzen, Lizenz-Gate, SBOM und Prüfung von Releases |

Kurzfassung für den Alltag:

```sh
make bootstrap   # einmal je Klon
make check       # vor jedem Push (der pre-push-Hook tut es automatisch)
make help        # alle Ziele
```
