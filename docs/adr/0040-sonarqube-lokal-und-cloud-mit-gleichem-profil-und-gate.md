---
status: accepted
date: 2026-10-10
decision-makers: Maintainer
---

# SonarQube lokal und Cloud mit gleichem Profil und Gate

## Context and Problem Statement

SonarQube soll vor dem Push lokal laufen (ADR 0038) und im öffentlichen Zustand Pull
Requests in SonarQube Cloud bewerten. Beide Seiten müssen dieselben Regeln und dasselbe
Gate anwenden, sonst findet die Cloud nach dem Push, was lokal durchging. Zwei Lücken
sind bekannt: Der Community Build analysiert kein Dart
(https://docs.sonarsource.com/sonarqube-community-build/analyzing-source-code/languages/overview/,
abgerufen 2026-10-10) [belegt], und der kostenlose Cloud-Plan für private Repositories
erlaubt keine eigenen Profile und Gates; der OSS-Plan für öffentliche Repositories
erlaubt sie
(https://docs.sonarsource.com/sonarqube-cloud/administering-sonarcloud/managing-subscription/subscription-plans.md,
abgerufen 2026-10-10) [belegt].

## Decision Drivers

* Gleiche Regeln und gleiches Gate lokal und in der Cloud, maschinell geprüft.
* Kein Abfluss von privatem Code an einen Dienst vor dem Öffentlich-Schalten.
* Dart-Abdeckung trotz fehlendem Analyzer im Community Build.
* Profil-Änderungen reviewbar im Repository.

## Considered Options

* Beide: lokaler Community Build und SonarQube Cloud, Profil und Gate als Dateien im
  Repository, eingespielt und verglichen durch ein Werkzeug
* Nur SonarQube Cloud
* Nur lokaler Community Build
* Kein SonarQube; nur sprachspezifische Linter

## Decision Outcome

Gewählt: "Beide, mit Dateien im Repository", weil nur so lokal und in der Cloud
nachweisbar dasselbe gilt.

* Profil "Bothy way" je Sprache erbt vom eingebauten Profil (`quality/sonar/profiles.json`,
  Eltern-Profil "Sonar way") und aktiviert zusätzlich nur die Regeln aus
  `quality/sonar/quality-profile-<sprache>.xml`. Zu Beginn sind diese Listen leer:
  zusätzliche Regeln kommen einzeln mit Grund im Pull Request.
* Gate "Bothy way" (`quality/sonar/quality-gate.json`) mit den vier Bedingungen des
  eingebauten Gates (keine neuen Befunde, alle neuen Hotspots geprüft, Abdeckung und
  Duplikation im neuen Code). Es nutzt nur Metriken, die in beiden Modi von SonarQube
  existieren
  (https://docs.sonarsource.com/sonarqube-community-build/user-guide/code-metrics/metrics-definition,
  abgerufen 2026-10-10) [belegt].
* `tools/sonarqube/quality_sync.py apply` spielt beides ein, `check` vergleicht. Gleichheit
  lokal und Cloud folgt daraus, dass beide gegen dieselben Dateien bestehen.
* Dart: lokal `flutter analyze` und `custom_lint` als eigenes Gate, in der Cloud zusätzlich
  SonarQube.
* Im privaten Zustand nur der lokale Community Build; SonarQube Cloud wird beim
  Öffentlich-Schalten angebunden (docs/quality/public-switch.md).

### Consequences

* Gut, weil eine Regeländerung ein reviewbarer Diff ist und beide Seiten prüfbar folgen.
* Gut, weil kein privater Code an die Cloud geht.
* Schlecht, weil der Community Build monatlich erscheint und die Cloud laufend
  aktualisiert wird; das eingebaute Eltern-Profil kann sich unterscheiden. Verglichen wird
  darum der Zusatz über dem Eltern-Profil, nicht die volle Regelmenge.
* Schlecht, weil eingebaute Profilnamen wechseln können: der öffentliche SonarQube-Server
  der Herstellerin führt für Go "Sonar way core", "Sonar way extended" und "Sonar way
  comprehensive" (Web-API `api/qualityprofiles/search?language=go` auf
  https://next.sonarqube.com/sonarqube, abgerufen 2026-10-10) [belegt]. Fehlt das
  Eltern-Profil, bricht `apply` mit einer Meldung ab, statt still ein anderes zu wählen.

### Confirmation

* `tools/sonarqube/tests/test_quality_sync.py` prüft Einspielen, Idempotenz, Reparatur
  abweichender Bedingungen und das Erkennen zusätzlicher Regeln gegen einen Test-Server.
* `make sonar-config-check` gegen den lokalen Server und gegen SonarQube Cloud ist Teil
  der Öffentlich-Schalten-Checkliste und jedes SonarQube-Updates.
* `tools/docs/tests/test_config_values.py` hält die Schwellen des Gates gleich den
  benannten Werten in Kapitel 23.

## Pros and Cons of the Options

### Beide, mit Dateien im Repository

* Gut, weil prüfbare Gleichheit und lokale Prüfung vor dem Push.
* Schlecht, weil ein lokaler Server mit Datenbank betrieben wird (Docker-kompatible
  Laufzeit, Arbeitsspeicher).

### Nur SonarQube Cloud

* Gut, weil kein lokaler Server.
* Schlecht, weil Befunde erst nach dem Push kommen und privater Code das Haus verlässt.
* Schlecht, weil eigene Profile und Gates im privaten Zustand nicht möglich sind.

### Nur lokaler Community Build

* Gut, weil alles lokal bleibt.
* Schlecht, weil keine Dart-Analyse und keine Bewertung von Fork-Beiträgen.

### Kein SonarQube

* Gut, weil weniger Werkzeuge.
* Schlecht, weil Duplikation, Abdeckung neuen Codes und Security Hotspots dann kein
  gemeinsames Gate haben.

## More Information

Schritt-für-Schritt: docs/quality/sonarqube-local.md.
