---
status: proposed
date: 2026-10-10
decision-makers: Owner (Konten und Kosten), Leitung der Planung, Team F (Betrieb)
consulted: Team E (Apps und Design), Team G (Plattformdienste, Push)
informed: alle Teams
---

# App-Verteilung und Auto-Update-Kanäle

## Context and Problem Statement

Die Flutter-Apps laufen auf iOS, Android, macOS und Windows (B5). Jede Plattform hat eigene Wege
für Signatur, Verteilung und Updates. Push-Schlüssel und Firebase-Projekt gehören zur
Instanz-Konfiguration (R-Push), und die Apps sind White Label. Wer baut und signiert die Apps, und
über welche Kanäle kommen Updates?

Status "proposed": Die technischen Wege stehen; Konten und Kosten (Apple Developer Program,
Windows-Signatur, Play-Konto) und der Android-Weg sind Entscheidungen des Owners.

## Decision Drivers

* Updates ohne Handarbeit der Menschen, mit Kanälen stable und beta
* Signatur, die Gatekeeper, SmartScreen und Play akzeptieren
* Push braucht dieselbe App-Identität wie die Signatur (Bundle-ID, Team, Firebase-Projekt)
* White Label: keine zentrale Infrastruktur des Projekts im Betrieb fremder Instanzen
* geringe laufende Kosten für eine kleine Organisation

## Considered Options

* Herausgeber-Modell: jede Organisation baut und signiert selbst mit `distribution.yaml`;
  macOS Developer ID mit Notarisierung und Sparkle 2; Windows MSIX mit `.appinstaller`; iOS
  TestFlight; Android Play Internal Testing
* Zentrale Builds des Projekts mit eigener Push-Weiterleitung für alle Instanzen
* macOS über TestFlight bzw. Mac App Store statt Developer ID
* Windows über Velopack oder WinSparkle statt MSIX

## Decision Outcome

Chosen option (vorgeschlagen): "Herausgeber-Modell mit Sparkle, MSIX und TestFlight", weil es ohne
zentrale Push-Weiterleitung auskommt und auf jeder Plattform den Weg mit Update-Kanälen nutzt,
den die Baseline nennt (R-Desktop-Verteilung).

1. Herausgeber-Modell und `distribution.yaml` ([18-release.md](../spec/18-release.md) 8).
2. macOS: Developer ID, Notarisierung, Sparkle 2 mit EdDSA, Kanal `beta` im Appcast, gestufte
   Verteilung (https://sparkle-project.org/documentation/publishing/, abgerufen 2026-10-10).
3. Windows: MSIX mit `stable.appinstaller` und `beta.appinstaller`; Signatur über Azure Artifact
   Signing (EU-Organisation möglich) oder OV-Zertifikat mit HSM; SmartScreen-Warnungen am Anfang
   einplanen (https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation,
   abgerufen 2026-10-10). MSIX gibt der App Package Identity für lokale Benachrichtigungen.
4. iOS: TestFlight-Gruppen beta und stable; App Store später.
5. Android: Play Internal Testing (Empfehlung); Firebase App Distribution als Alternative.

### Consequences

* Gut, weil jede Instanz Push mit eigenen Schlüsseln betreibt und niemand Inhalte oder Tokens
  einer fremden Instanz sieht.
* Gut, weil jede Plattform einen Kanal beta mit demselben Build für stable hat.
* Schlecht, weil jede Organisation eigene Konten bei Apple, Microsoft bzw. einer CA und Google
  braucht.
* Schlecht, weil eine App-Build-Kette mit macOS- und Windows-Runnern nötig ist.
* Neutral, weil eine zentrale Distribution später als zusätzlicher Herausgeber möglich bleibt.

### Confirmation

* Release-Kette prüft je Plattform: Signatur gültig (`codesign --verify`, `spctl --assess`,
  `signtool verify`), Notarisierung gestapelt, Appcast-Eintrag mit EdDSA-Signatur.
* Test der Update-Kette je Plattform mit einem Test-Herausgeber (beta-Build ersetzt älteren Build).

## Pros and Cons of the Options

### Herausgeber-Modell, Sparkle, MSIX, TestFlight

* Gut, weil Push-Identität und Signatur in einer Hand liegen.
* Gut, weil Sparkle und MSIX Kanäle und signierte Updates mitbringen.
* Schlecht, weil der Aufwand je Organisation steigt.

### Zentrale Builds mit Push-Weiterleitung

* Gut, weil Organisationen keine Konten brauchen.
* Schlecht, weil das Projekt einen Dienst betreiben müsste, der Push-Daten aller Instanzen sieht.
* Schlecht, weil das gegen White Label und Single-Tenant (B13) arbeitet.

### macOS über TestFlight oder Mac App Store

* Gut, weil Apple die Verteilung übernimmt.
* Schlecht, weil TestFlight für macOS eine App Sandbox verlangt [vermutet] und damit eine zweite
  Ausgabe entstünde.

### Windows über Velopack oder WinSparkle

* Gut, weil Velopack Deltas und mehrere Plattformen kann.
* Schlecht, weil Velopack kein Dart-SDK hat (https://github.com/velopack/velopack) und das
  Flutter-Paket `auto_updater` unter Windows WinSparkle mit DSA nutzt und seit rund zwei Jahren
  keine neue Version hat (https://pub.dev/packages/auto_updater); beide abgerufen 2026-10-10
  [belegt].
* Schlecht, weil ohne MSIX keine Package Identity für Benachrichtigungen vorhanden ist.

## More Information

* Owner-Entscheidungen: Apple Developer Program; Weg der Windows-Signatur (Artifact Signing oder
  OV mit HSM); Android-Weg; Zeitpunkt App Store.
* Push-Einzelheiten: Kapitel 13 (Team G).
