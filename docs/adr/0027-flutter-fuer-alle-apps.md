---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung, Team Apps und Design
consulted: Team Architektur, Team Plattformdienste
informed: alle Teams
---

# Flutter für alle Apps

## Context and Problem Statement

Das Produkt braucht Apps für iOS, Android, macOS und Windows sowie eine Web-Konsole für
Verwaltung und Statistik (Baseline B5). Ein kleines Team baut und pflegt sie; Funktionen, Dichte
und Gestaltung sollen auf allen Geräten gleich sein. Gleichzeitig verlangen Systemfunktionen
nativen Code: Notification Service Extension (iOS, macOS), Share Extensions, Push-Dienst auf
Android, Infobereich und MSIX auf Windows, Schlüssel im sicheren Speicher. Welche Technik trägt
alle Oberflächen?

## Decision Drivers

- vier Zielplattformen plus Web-Konsole mit einer Codebasis für Oberfläche und Logik
- eigenes, unverwechselbares Design-System statt Plattform-Standardoptik
- dichte Listen und lange, live aktualisierte Inhalte (Live-Box) ohne Ruckeln
- nativer Code dort, wo das System es verlangt, mit typisierten Kanälen
- Testbarkeit (Widget-, Golden-, Integrationstests) ohne Gerätefarm für jeden Lauf
- Lizenz und Lieferkette passend zu Apache-2.0

## Considered Options

1. Flutter für alle Oberflächen, nativer Code nur für Erweiterungen und Systemdienste
2. Je Plattform nativ (SwiftUI, Jetpack Compose, WinUI 3) plus eigene Web-Konsole
3. React Native (mit react-native-windows und react-native-macos)
4. Kotlin Multiplatform mit Compose Multiplatform
5. Web-Technik im Desktop-Rahmen (Tauri oder Electron) plus Mobil separat

## Decision Outcome

Chosen option: "Flutter für alle Oberflächen", weil es als einzige Option iOS, Android, macOS,
Windows und Web aus einer Codebasis mit eigenem Rendering bedient (unterstützte Plattformen laut
https://docs.flutter.dev/reference/supported-platforms, abgerufen 2026-10-10 [belegt]), das eigene
Design-System ohne Kampf gegen Plattform-Komponenten erlaubt und für Systemfunktionen typisierte
Kanäle (Pigeon) und Erweiterungen in Swift, Kotlin und C++ zulässt. Die Vorstudie kam zum gleichen
Ergebnis; Android und die Web-Konsole stärken es.

### Consequences

- Good, because Oberfläche, Zustand, Sync und Design-System einmal gebaut und getestet werden.
- Good, because Golden-Tests die Gestaltung über alle Plattformen festhalten.
- Good, because reine Dart-Pakete (Domäne, Sync, Anmeldung) ohne Gerät testbar sind.
- Bad, because Erweiterungen (NSE, Share Extension, Android-Push-Dienst) trotzdem nativ entstehen
  und eine kleine Swift- bzw. Kotlin-Bibliothek brauchen (`ClientKit`).
- Bad, because Mehrfenster auf Desktop in Flutter noch experimentell ist (Thread als eigenes
  Fenster ist darum "später").
- Bad, because die Barrierefreiheit auf macOS vor Auslieferung gesondert geprüft werden muss.
- Neutral, because der Windows-Build Windows mit Visual Studio braucht und der macOS- und iOS-Build
  einen Mac mit Xcode; die Runner-Strategie regelt das (ADR 0039).

### Confirmation

- Das Gate baut `apps/client` für iOS, Android, macOS und Windows und `apps/console` für Web.
- Ein Architekturtest prüft, dass Plattform-Code nur in `packages/device_key` und
  `apps/client/{ios,macos,android,windows}` liegt und reine Pakete kein `package:flutter`
  importieren (15-client-architektur.md, Abschnitt 3.1).

## Pros and Cons of the Options

### Flutter

- Good, because eine Codebasis für alle Ziele inklusive Web.
- Good, because eigenes Rendering das Design-System pixelgleich hält.
- Good, because reife Pakete für die Kernaufgaben (drift, Riverpod, Sentry, record) existieren.
- Bad, because einige Desktop-Funktionen (Mehrfenster) jung sind.
- Bad, because Erweiterungen nativ bleiben.

### Je Plattform nativ

- Good, because beste Systemintegration und Barrierefreiheit je Plattform.
- Bad, because drei bis vier Oberflächen-Codebasen; das widerspricht der Teamgröße.
- Bad, because das Design-System je Plattform neu entsteht und auseinanderläuft.

### React Native

- Good, because große Gemeinschaft, Web-Nähe.
- Bad, because Windows und macOS hängen an getrennten Projekten außerhalb des Kerns.
- Bad, because Plattform-Komponenten erschweren ein eigenes, dichtes Design.

### Kotlin Multiplatform mit Compose Multiplatform

- Good, because Logik geteilt, Compose auf Android nativ.
- Bad, because iOS- und Web-Ziele von Compose Multiplatform sind jünger; Windows nur über die
  Desktop-JVM.
- Bad, because zweite Sprache neben Go und Python ohne Mehrwert für das Web-Ziel.

### Web-Technik im Desktop-Rahmen

- Good, because eine Web-Codebasis für Desktop und Konsole.
- Bad, because Mobil bräuchte eine zweite Technik.
- Bad, because Push, Share Extension und sicherer Speicher auf Mobil ohnehin nativ wären.

## More Information

- Plattformdetails, Pakete und Mindestversionen: [15-client-architektur.md](../spec/15-client-architektur.md).
- Flutter 3.47 ist die aktuelle stabile Version (https://docs.flutter.dev/release/release-notes,
  abgerufen 2026-10-10 [belegt]).
- Verwandt: ADR 0028 (Web nur Konsole), ADR 0029 (Zustand und lokale DB), ADR 0030 (Design-Tokens).
