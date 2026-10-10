# Self-hosted Runner (nur privater Zustand)

Im privaten Zustand laufen alle Jobs mit der Runner-Weiche auf einem self-hosted Runner
mit dem Label `bothy-ci` (ADR 0039). Er gehört zum Betrieb einer Instanz, nicht zum
Produkt: Wo er läuft, steht im privaten Ops-Repository. Vor dem Öffentlich-Schalten wird
er entfernt (public-switch.md); GitHub rät von self-hosted Runnern in öffentlichen
Repositories ab, weil jeder per Pull Request Code darauf ausführen kann
(https://docs.github.com/en/actions/reference/security/secure-use, abgerufen 2026-10-10)
[belegt].

## Grundsätze

* **Eigener Benutzer.** Ein eigenes Betriebssystem-Konto nur für den Runner, ohne
  Administratorrechte, ohne Zugriff auf Dateien anderer Benutzer, ohne SSH-Schlüssel,
  Cloud-Zugangsdaten oder Browser-Profile.
* **Eigenes Arbeitsverzeichnis** im Home dieses Benutzers; nichts außerhalb.
* **Nur nötige Secrets.** Der Runner bekommt Secrets nur aus GitHub zur Laufzeit eines
  Jobs (`SONAR_TOKEN`, die Deny-Liste der Umgebung `denylist`, Release-Secrets der
  Release-Umgebungen). Auf der Maschine liegen keine.
* **Aufräumen je Job.** Der Runner ist nicht ephemer (ein ephemerer Runner bräuchte ein
  dauerhaft hinterlegtes Token zum Neu-Registrieren, also ein Secret auf der Maschine).
  Stattdessen räumen Skripte vor und nach jedem Job auf (siehe unten).
* **Container-Laufzeit** im Kontext des Runner-Benutzers, getrennt von anderen
  Container-Instanzen der Maschine. Nötig für Image-Builds im Release; der lokale
  SonarQube kann auf derselben Maschine laufen und ist über `127.0.0.1` erreichbar.

## Einrichten

1. Betriebssystem-Benutzer anlegen (zum Beispiel `bothy-ci`), anmelden als dieser Benutzer.
2. In GitHub: Repository, Settings, Actions, Runners, "New self-hosted runner". Paket für
   Betriebssystem und Architektur laden; die angezeigte Prüfsumme vergleichen.
3. Registrieren mit dem Label `bothy-ci` und einem neutralen Namen:
   `./config.sh --url https://github.com/<owner>/<repo> --token <registrierungs-token>
   --labels bothy-ci --name <neutraler-name> --work _work --unattended`.
   Auf einem Mac zusätzlich das Label `macOS` (für die macOS-Weiche in `release.yml`).
4. Aufräum-Skripte außerhalb des Runner-Verzeichnisses ablegen und in der Datei `.env`
   des Runner-Verzeichnisses eintragen:
   `ACTIONS_RUNNER_HOOK_JOB_STARTED=<pfad>/job-started.sh` und
   `ACTIONS_RUNNER_HOOK_JOB_COMPLETED=<pfad>/job-completed.sh`
   (https://docs.github.com/en/enterprise-server@3.20/actions/how-tos/manage-runners/self-hosted-runners/run-scripts,
   abgerufen 2026-10-10) [belegt]. Die Skripte leeren das Arbeitsverzeichnis `_work`
   bis auf die Tool-Caches, das temporäre Verzeichnis des Benutzers und beenden
   übrig gebliebene Container des Jobs.
5. Als Dienst des Runner-Benutzers starten (`./svc.sh install`, `./svc.sh start`).
6. Repository-Einstellungen, Actions, General: "Fork pull request workflows" für private
   Repositories aus; Standard-Rechte des Workflow-Tokens "Read repository contents".
7. Probelauf: einen Pull Request öffnen und prüfen, dass alle Jobs von `ci.yml` auf diesem
   Runner liefen (Job-Protokoll, Abschnitt "Set up job").

## Betrieb

* Der Runner aktualisiert sich selbst; das Betriebssystem bekommt Sicherheits-Updates.
* Alle Jobs teilen sich den Runner und laufen nacheinander; für ein Repository mit einer
  Person reicht das.
* Ein Job, der hängt, endet an seinem `timeout-minutes`.

## Entfernen vor dem Öffentlich-Schalten

1. Repository, Settings, Actions, Runners: Runner wählen, "Remove", Entfernungs-Token
   kopieren.
2. Auf der Maschine als Runner-Benutzer: `./svc.sh stop`, `./svc.sh uninstall`,
   `./config.sh remove --token <entfernungs-token>`.
3. Runner-Verzeichnis und Betriebssystem-Benutzer löschen.
4. In GitHub prüfen: keine Runner mehr gelistet. Ab dann wählen alle Jobs über die Weiche
   GitHub-gehostete Runner; ein Job mit nur `self-hosted` würde nicht starten.
