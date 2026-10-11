# Checkliste: Öffentlich schalten

Das Produkt-Repository wird erst nach dem Go des Owners öffentlich. Diese Liste wird
vollständig abgearbeitet, jeder Punkt mit Beleg (Befehlsausgabe oder Bildschirmfoto im
Ops-Repository). Reihenfolge einhalten: erst prüfen und bereinigen, dann umschalten, dann
die öffentlichen Dienste anbinden.

## A. Vor dem Umschalten

1. **Red-Team-Gate** über den ganzen Stand mit Urteil `JA` (red-team-gate.md).
2. **Commit-Identität.** Die öffentliche Identität der Maintainer steht fest (Name und
   E-Mail, zum Beispiel die No-Reply-Adresse von GitHub) und ist in `allow-authors.txt` der
   privaten Liste eingetragen. Ältere Commits mit anderer Identität werden vorher
   umgeschrieben (git-filter-repo mit Mailmap) oder das Repository startet mit neuer
   Historie. Beim Umschreiben ersetzt `--replace-text` auch die alten Beispielwerte,
   die `.gitleaks.toml` als Ausnahme für die Historie führt; danach wird diese Ausnahme
   gelöscht, und Schritt 3 zeigt, dass die Werte fort sind. Jede bisherige interne
   Identität wird aus `allow-authors.txt` entfernt, bevor Schritt 3 läuft; nur dann belegt
   ein Scan ohne Fund, dass das Umschreiben vollständig war.
3. **Historien-Scan.** `make denylist-history` und
   `make gitleaks GITLEAKS_LOG_OPTS=--all` sind grün. Jeder Treffer wird entfernt; Treffer
   in alten Commits erzwingen ein Umschreiben der Historie, danach beide Scans erneut.
4. **Secrets rotieren**, die je im Repository oder in CI-Logs standen.
5. **Self-hosted Runner entfernen** (self-hosted-runner.md, Abschnitt "Entfernen") und
   prüfen, dass keiner mehr gelistet ist.
6. **Platzhalter ersetzen:** `@OWNER` in `.github/CODEOWNERS`, `OWNER` in
   `.github/ISSUE_TEMPLATE/config.yml`, `mkdocs.yml`, `docs/index.md` und
   `CONTRIBUTING.md`, die Adressen in `SECURITY.md` und `CODE_OF_CONDUCT.md`
   (`grep -rn OWNER` und `grep -rn example.com` als Gegenprobe).
7. **Lizenz-Gate** grün, `NOTICE` vollständig, ScanCode-Lauf über den ganzen Baum ohne
   unerwartete Lizenz (licenses.md).

## B. Umschalten

8. Repository, Settings, General, Danger Zone: Sichtbarkeit "Public".
9. Repository-Variable `BOTHY_VISIBILITY` auf `public` setzen (geplante Läufe,
   ADR 0039).

## C. Direkt danach

10. **Secret Scanning und Push Protection** einschalten (Settings, Advanced Security);
    für öffentliche Repositories kostenlos
    (https://docs.github.com/en/code-security/secret-scanning/introduction/about-push-protection,
    abgerufen 2026-10-10) [belegt für Secret Scanning; Push Protection auf
    Repository-Ebene laut Changelog 2023 kostenlos, nicht erneut geprüft].
11. **Private vulnerability reporting** einschalten (Settings, Advanced Security);
    Voraussetzung für den Meldeweg in SECURITY.md.
12. **Rulesets** für `main` nach push-discipline.md (Pull Request, nur Squash, Pflicht-Check
    `ci-ok`, lineare Historie, signierte Commits, kein Force-Push, kein Löschen); zusätzlich
    "Require actions to be pinned to a full-length commit SHA" in den Actions-Einstellungen.
13. **Environments** prüfen: `denylist` (Secret `BOTHY_DENYLIST`, optional
    `BOTHY_DENYLIST_ALLOW_AUTHORS`), `release`, `release-apple`, `release-windows` mit
    Required Reviewers und Tag-Regel `*/v*` und `v*`; `github-pages` für die Doku.
14. **SonarQube Cloud**: Organisation mit OSS-Plan, Projekt anlegen, automatische Analyse
    aus (R-QA-10), `make sonar-config` und `make sonar-config-check` gegen die Cloud
    (sonarqube-local.md, Abschnitt 6). Repository-Variablen `SONAR_HOST_URL`,
    `SONAR_ORGANIZATION`, `SONAR_PROJECT_KEY`, Secret `SONAR_TOKEN`.
15. **CodeRabbit** als GitHub-App für das Repository installieren; `.coderabbit.yaml` gilt.
16. **Renovate** aktivieren (ADR 0041) und das Dependency Dashboard prüfen.
17. **Pages** auf "GitHub Actions" als Quelle stellen; `docs.yml` manuell starten.
18. **Probelauf:** zuerst `codeql.yml` auf `main` manuell starten, damit Code Scanning eine
    Analyse des Basis-Branchs hat. Dann ein Pull Request aus einem Fork (generische
    Deny-Regeln, kein Secret) und ein Pull Request aus dem Repository (volle Liste, SonarQube
    Cloud, CodeQL). Prüfen, dass alle Jobs auf GitHub-gehosteten Runnern liefen und CodeQL
    und Scorecard starten. Nach dem ersten Pull Request aus dem Repository sind die Jobs
    `analyze (actions)`, `analyze (go)` und `analyze (python)` grün, und Code Scanning zeigt
    Ergebnisse der Kategorie `/language:go`.
19. **DCO** prüfen: ein Pull Request ohne Sign-off muss an `ci-ok` scheitern.
20. **Release-Signaturen umstellen** ([ADR 0062](../adr/0062-release-signaturen-solange-das-repo-privat-ist.md)):
    `release.yml` auf cosign keyless und Attestations stellen (ADR 0036), den privaten
    cosign-Schlüssel und sein Passwort aus der Umgebung `release` löschen; der öffentliche
    Schlüssel bleibt im Repository, damit ältere Releases prüfbar bleiben.

## Zurück auf privat

Wird das Repository wieder privat, zuerst `BOTHY_VISIBILITY` entfernen, dann einen Runner
nach self-hosted-runner.md einrichten. Die Weiche wählt ihn ab dem nächsten Lauf.
