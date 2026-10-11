# Checkliste: Veröffentlichen

Das Produkt-Repository ist ab dem ersten Push öffentlich
([ADR 0039](../adr/0039-nur-github-gehostete-runner-oeffentlich-ab-dem-ersten-push.md)); jeder
Workflow läuft auf GitHub-gehosteten Runnern. Diese Liste wird vollständig und in dieser
Reihenfolge abgearbeitet, jeder Punkt mit Beleg (Befehlsausgabe oder Bildschirmfoto im
Ops-Repository): erst lokal prüfen, dann pushen und sofort öffentlich schalten, dann SonarQube
Cloud und CodeRabbit anbinden, dann `main` schützen. Danach kommt jedes Ticket als eigener Pull
Request.

## A. Vor dem Push

1. **Red-Team-Gate** über den ganzen Stand mit Urteil `JA` (red-team-gate.md).
2. **Ein Wurzel-Commit mit der gewählten Identität.** Veröffentlicht wird ein einzelner Commit
   ohne Eltern mit dem geprüften Stand; die Planungshistorie bleibt privat (D-W-93). Ein Umschreiben der
   Historie mit `git filter-repo` (PLAN M0-08, D-W-48) ist nur nötig, wenn doch eine Historie
   veröffentlicht werden soll. Die Commit-Identität hat der Owner gewählt; sie ist in
   `allow-authors.txt` der privaten Liste eingetragen und dort nur in den Feldern Autor und
   Committer erlaubt (und in einer `Signed-off-by`-Zeile, die genau diese wiederholt, D-W-37).
   Jede Identität in `allow-authors.txt`, die nicht die gewählte ist, wird entfernt, bevor
   Schritt 3 läuft; nur dann belegt ein Scan ohne Fund, dass keine andere Identität im Commit
   steht.
3. **Scans** in einem frischen Klon nur dieses Commits, mit der privaten Liste: `make denylist`,
   `make denylist-history` und `make gitleaks GITLEAKS_LOG_OPTS=--all` sind grün. Jeder Treffer
   wird im Stand entfernt, danach wird der Commit neu gebaut und beide Scans laufen erneut.
4. **Secrets rotieren**, die je im Repository oder in CI-Logs standen.
5. **Lizenz-Gate** grün, `NOTICE` vollständig, ScanCode-Lauf über den ganzen Baum ohne
   unerwartete Lizenz (licenses.md).
6. **Umgebung `denylist`** im noch leeren Repository anlegen: Secrets `BOTHY_DENYLIST` und
   `BOTHY_DENYLIST_ALLOW_AUTHORS`, erreichbar für Pull Requests aus dem Repository und für
   `main`. `denylist-main.yml` läuft schon beim Push (Schritt 7); fehlt die Erlaubnisliste der
   Autoren, kann sein Log zeigen, dass die Autor-Identität eine Regel der privaten Liste trifft,
   und dieses Log ist danach öffentlich.

## B. Push und sofort öffentlich

7. **Push** des Commits als `main` in das leere Repository. Es darf noch kein `main` geben:
   Einen Commit ohne Eltern nimmt ein bestehendes `main` nur per Force-Push an.
8. **Sofort öffentlich:** Repository, Settings, General, Danger Zone: Sichtbarkeit "Public". Im
   selben Zug die Repository-Variable `BOTHY_VISIBILITY` auf `public` setzen (geplante Läufe von
   CodeQL, Scorecard und Pages, ADR 0039 Regel 1).
9. **Private vulnerability reporting** einschalten (Settings, Advanced Security), im selben
   Zug wie Schritt 8: `SECURITY.md` und `CODE_OF_CONDUCT.md` nennen nur diesen Meldeweg, er
   muss ab dem Umschalten funktionieren. Gegenprobe: ein neues Issue beginnen; die Auswahl
   verlinkt `SECURITY.md` und `SUPPORT.md` aus dem Repository [vermutet, offline nicht
   geprüft]. Fehlt einer der beiden Links, bekommt `.github/ISSUE_TEMPLATE/config.yml` per
   Pull Request einen Eintrag unter `contact_links` mit `name`, `about` und der absoluten URL der
   Datei; der Eintrag muss `make denylist` bestehen, und der Kopfkommentar der Datei wird
   angepasst.
10. **Secret Scanning und Push Protection** einschalten (Settings, Advanced Security);
    für öffentliche Repositories kostenlos
    (https://docs.github.com/en/code-security/secret-scanning/introduction/about-push-protection,
    abgerufen 2026-10-10) [belegt für Secret Scanning; Push Protection auf
    Repository-Ebene laut Changelog 2023 kostenlos, nicht erneut geprüft].
11. **Deny-Scan des Pushs:** `denylist-main.yml` ist für den Push aus Schritt 7 grün gelaufen.
    Ist er nicht grün, Ursache klären und ihn per `workflow_dispatch` erneut starten.

## C. SonarQube Cloud und CodeRabbit

12. **SonarQube Cloud**: Organisation des Kontos, dem das Repository gehört (Name im
    Ops-Repository), mit OSS-Plan, Projekt anlegen, automatische Analyse aus (R-QA-10),
    `make sonar-config` und `make sonar-config-check` gegen die Cloud (sonarqube-local.md,
    Abschnitt 6). Repository-Variablen `SONAR_HOST_URL` (`https://sonarcloud.io`),
    `SONAR_ORGANIZATION`, `SONAR_PROJECT_KEY`, Secret `SONAR_TOKEN`. Ohne diese Werte scheitert
    der Job `sonar` jedes Pull Requests aus dem Repository.
13. **CodeRabbit** als GitHub-App für das Repository installieren; `.coderabbit.yaml` gilt.

## D. Branch-Schutz

14. **Rulesets** für `main` nach push-discipline.md (Pull Request, nur Squash, Pflicht-Check
    `ci-ok`, lineare Historie, signierte Commits, kein Force-Push, kein Löschen); zusätzlich
    "Require actions to be pinned to a full-length commit SHA" in den Actions-Einstellungen.
15. **Environments** `release`, `release-apple` und `release-windows` mit Required Reviewers und
    Tag-Regel `*/v*` und `v*`; `github-pages` für die Doku.

## E. Danach: jedes Ticket als eigener Pull Request

Jedes Ticket kommt der Reihe nach als eigener Pull Request nach push-discipline.md auf `main`.

16. **Renovate** aktivieren (ADR 0041) und das Dependency Dashboard prüfen.
17. **Pages** auf "GitHub Actions" als Quelle stellen; `docs.yml` manuell starten.
18. **Probelauf** mit dem ersten Pull Request: Alle Jobs liefen auf GitHub-gehosteten Runnern,
    `ci-ok` ist grün, SonarQube Cloud, CodeQL und CodeRabbit melden sich, Scorecard startet.
    Dazu ein Pull Request aus einem Fork (generische Deny-Regeln, kein Secret).
19. **DCO** prüfen: ein Pull Request ohne Sign-off muss an `ci-ok` scheitern.
20. **Release-Signaturen** vor dem ersten Release-Tag (Bau-Ticket M7-10): Die Release-Kette
    signiert keyless und mit Attestations (ADR 0036). Das Schlüsselpaar aus
    [ADR 0062](../adr/0062-release-signaturen-solange-das-repo-privat-ist.md) gilt nur für
    Releases aus einem privaten Repository und entsteht nicht.
21. **Portabilitätstest auf dem öffentlichen Weg** mit dem ersten Produkt-Release: Zuerst die
    Pakete `bothy-server` und `bothy-postgres` in der Container-Registry öffentlich stellen
    (Paket-Einstellungen, Danger Zone, "Change visibility"). Ein neues Paket ist privat, und ein
    mit dem Repository verknüpftes Paket erbt "the access permissions (but not the visibility)"
    des Repositorys
    (https://docs.github.com/en/packages/learn-github-packages/configuring-a-packages-access-control-and-visibility,
    abgerufen 2026-10-11) [belegt]. Der Schritt lässt sich nicht zurücknehmen ("Once you make a
    package public, you cannot make it private again", gleiche Quelle) [belegt]. Dann
    `portability.yml` per `workflow_dispatch` mit dem letzten Produkt-Tag starten. Er checkt diesen
    Tag aus (Test-Code passend zum Release, [16-betrieb.md](../spec/16-betrieb.md) 11.1), holt
    `install.sh` ohne Anmeldung mit dem dokumentierten `curl`-Befehl (Quelle `url`,
    [16-betrieb.md](../spec/16-betrieb.md) 3.1) und zieht die Images, die das Bündel pinnt
    ([18-release.md](../spec/18-release.md) 2.1), ebenfalls ohne Anmeldung; ein noch privates
    Paket lässt ihn scheitern. Er muss grün sein.
22. **Platzhalter `OWNER` ersetzen** (PLAN V1-03): `@OWNER` in `.github/CODEOWNERS`, `OWNER` in
    `mkdocs.yml`, `docs/index.md`, `README.md` und `CONTRIBUTING.md` (`grep -rn OWNER` als
    Gegenprobe). `make denylist` muss danach grün sein; trifft die private Liste den neuen Pfad,
    bekommt sie im Ops-Repository eine enge Ausnahme für genau diesen Pfad.
