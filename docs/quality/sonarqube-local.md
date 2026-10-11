# SonarQube lokal

Der lokale SonarQube Community Build bewertet Produkt-Code vor dem Push mit demselben
Profil und Gate wie SonarQube Cloud (ADR 0040). Er läuft aus
`tools/sonarqube/compose.yaml`: Web-Oberfläche nur auf `127.0.0.1`, PostgreSQL nur in
einem internen Netz ohne Host-Port.

## Voraussetzungen

* Eine Docker-kompatible Laufzeit mit Compose.
* Arbeitsspeicher für die Container-VM nach den Herstellerangaben: der Community Build
  nennt 4 GB für bis zu eine Million Zeilen
  (https://docs.sonarsource.com/sonarqube-community-build/server-installation/server-host-requirements/,
  abgerufen 2026-10-10) [belegt].
* Der Port `quality.sonar.local_port` (Umgebungsvariable `SONARQUBE_PORT`) ist auf dem
  Rechner frei. Ist er belegt, `SONARQUBE_PORT` setzen und `SONAR_HOST_URL` passend wählen.
  Die Beispiele unten nutzen den Standard-Port.

## 1. Starten

```sh
make sonar-up
```

Beim ersten Mal legt das Ziel `tools/sonarqube/.env` mit einem zufälligen
Datenbank-Passwort an (Dateirechte nur für den Besitzer, git-ignoriert), startet beide
Container und wartet bis `api/system/status` den Status `UP` meldet, höchstens
`quality.sonar.startup_timeout_s`. Die Daten liegen in benannten Volumes; `make sonar-down`
stoppt, ohne sie zu löschen.

Die eingebaute Suchmaschine verlangt sonst Kernel-Einstellungen des Hosts
(`vm.max_map_count`); für den lokalen Einzelbetrieb schaltet die Compose-Datei diese
Startprüfung ab, wie es die Schnellstart-Anleitung des Herstellers tut
(https://docs.sonarsource.com/sonarqube-community-build/try-out-sonarqube/, abgerufen
2026-10-10) [belegt].

## 2. Admin-Passwort ändern

`http://127.0.0.1:9000` öffnen, mit `admin`/`admin` anmelden (Herstellervorgabe, gleiche
Quelle) und das Passwort sofort ändern. Das neue Passwort gehört in den Passwort-Manager,
nie in eine Datei im Repository.

## 3. Tokens erzeugen, ohne sie anzuzeigen

Zwei Tokens mit getrennten Rechten:

| Token | Typ | Wofür |
|---|---|---|
| Konfiguration | `USER_TOKEN` des Admins, mit Ablaufdatum | einmalig `make sonar-config`, danach widerrufen |
| Analyse | `GLOBAL_ANALYSIS_TOKEN` | `make sonar` und `make check` |

Die Typen nennt die Web-API (`api/user_tokens/generate`, Parameter `type`) [belegt am
öffentlichen Server der Herstellerin, https://next.sonarqube.com/sonarqube/api/webservices/list,
abgerufen 2026-10-10]. Das Token geht direkt in eine Datei außerhalb des Repositorys und
erscheint nie im Terminal:

```sh
read -rs -p "SonarQube admin password: " SQ_ADMIN; echo
umask 077; mkdir -p ~/.config/bothy
curl -fsS -u "admin:${SQ_ADMIN}" -X POST "http://127.0.0.1:9000/api/user_tokens/generate" \
  --data-urlencode "name=bothy-analysis" --data-urlencode "type=GLOBAL_ANALYSIS_TOKEN" \
  | python3 -c 'import json, sys; print(json.load(sys.stdin)["token"])' > ~/.config/bothy/sonar-token
unset SQ_ADMIN
```

Für die Konfiguration dasselbe mit `type=USER_TOKEN`, einem `expirationDate` und einer
eigenen Datei. In der Shell, die `make` aufruft:

```sh
export SONAR_TOKEN="$(cat ~/.config/bothy/sonar-token)"
```

Statt einer Datei geht auch der Schlüsselbund des Betriebssystems.

## 4. Profil und Gate einspielen

```sh
SONAR_TOKEN="$(cat ~/.config/bothy/sonar-config-token)" make sonar-config
SONAR_TOKEN="$(cat ~/.config/bothy/sonar-token)" make sonar-config-check
```

`sonar-config` legt das Gate "Bothy way" an (oder repariert es) und macht es zum
Standard, legt je Sprache das Profil "Bothy way" an, setzt das Eltern-Profil, spielt die
Zusatzregeln aus `quality/sonar/quality-profile-<sprache>.xml` ein und macht das Profil
zum Standard. Sprachen, die der Community Build nicht kennt (Dart, Swift), werden
übersprungen und genannt. Danach das Konfigurations-Token widerrufen.

Fehlt das Eltern-Profil (eingebaute Profilnamen können sich mit einer neuen
SonarQube-Version ändern), bricht `sonar-config` ab. Dann die eingebauten Profile prüfen,
`quality/sonar/profiles.json` per Pull Request anpassen und erneut einspielen.

## 5. Analysieren

```sh
make sonar          # oder make check
```

Der Scanner (`@sonar/scan`, ohne lokale Java-Installation) liest
`sonar-project.properties` und wartet auf das Gate-Ergebnis; ein rotes Gate lässt
`make sonar` scheitern. Solange es keinen Produkt-Code gibt, überspringt das Ziel.

## 6. Abgleich mit SonarQube Cloud

Im öffentlichen Zustand (public-switch.md) gegen die Cloud:

```sh
SONAR_HOST_URL=https://sonarcloud.io SONAR_ORGANIZATION=<organisation> \
  SONAR_TOKEN="<Token aus dem Passwort-Manager>" make sonar-config        # einmalig
SONAR_HOST_URL=https://sonarcloud.io SONAR_ORGANIZATION=<organisation> \
  SONAR_TOKEN="<Token>" make sonar-config-check
```

Beide Prüfungen (lokal und Cloud) bestehen gegen dieselben Dateien, also gelten dieselben
Regeln. Verglichen wird der Zusatz über dem Eltern-Profil und jede Gate-Bedingung. Das
eingebaute Eltern-Profil selbst kann sich zwischen Community Build (monatliche Releases)
und Cloud (laufend) unterscheiden; das ist die bekannte Restabweichung. Wann prüfen:
nach jedem Update des lokalen Images, nach jeder Änderung unter `quality/sonar/` und
monatlich.

## Eine Regel hinzufügen

1. Regel-Schlüssel und Begründung im Pull Request nennen (welcher Fehler, welche
   Fundstelle).
2. Regel in `quality/sonar/quality-profile-<sprache>.xml` unter `<rules>` eintragen
   (Format der Backup-Datei von SonarQube: `repositoryKey`, `key`, optional `priority`
   und `parameters`).
3. `make sonar-config` lokal, nach dem Merge gegen die Cloud; `sonar-config-check` an
   beiden Orten.
