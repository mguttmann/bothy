---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team E (Apps und Design), Team G (Plattformdienste), Team C (Identität)
informed: Team F (Betrieb)
---

# Native Erweiterungen ohne Flutter-Engine (NSE, Share, Android-Push)

## Context and Problem Statement

Die Apps sind Flutter (ADR 0027). Drei Einstiege laufen aber außerhalb der App-Oberfläche: die
Notification Service Extension (NSE) auf iOS und macOS, die den Text eines Push holt; die Share
Extension auf iOS und macOS, über die eine andere App Inhalte teilt; der Push-Empfang auf Android.
Alle drei haben enge Grenzen: Die NSE hat etwa 30 Sekunden, `onMessageReceived` unter FCM nur
wenige Sekunden, und eine Share Extension kann die eigene App nicht öffnen
([15-client-architektur.md](../spec/15-client-architektur.md), Abschnitte 10 und 12, mit Belegen).
Laufen diese Einstiege mit einer Flutter-Engine oder nativ?

## Decision Drivers

- Zeit- und Speichergrenzen der Erweiterungen und Hintergrunddienste.
- Push trägt nur IDs; den Text holt das Gerät selbst (ADR 0031).
- Teilen darf nicht davon abhängen, dass die Person später die App öffnet.
- Eine Codebasis, nativ nur wo das System es verlangt (15, Abschnitt 1, Grundsatz 4).
- Kein zweiter Prozess darf die Refresh-Familie der App rotieren (Wiederverwendungs-Erkennung,
  [08-identitaet.md](../spec/08-identitaet.md), Abschnitt 7.4).

## Considered Options

- Native Erweiterungen in Swift und Kotlin mit einer gemeinsamen Swift-Bibliothek `ClientKit`;
  keine Flutter-Engine in Erweiterung und Push-Dienst
- Flutter-Engine im Hintergrund (Dart-Handler von `firebase_messaging`, Flutter in der Erweiterung)
- Keine Erweiterungen: Push nur mit Platzhalter, Teilen nur über die App
- Push mit Inhalt, damit nichts nachgeladen werden muss
- Share Extension legt nur ab, die App sendet später

## Decision Outcome

Gewählte Option: "Native Erweiterungen mit `ClientKit`, ohne Flutter-Engine", weil nur sie in den
Grenzen der Systeme sicher fertig werden. Entscheidung R-CL-06 in
[15-client-architektur.md](../spec/15-client-architektur.md).

1. **NSE (iOS, macOS).** Swift ohne Flutter-Engine. Sie holt die Vorschau über
   `GET /api/v1/push/preview` mit dem Vorschau-Token der Push-Registrierung und einem DPoP-Beweis
   des Geräteschlüssels ohne Nutzerbestätigung
   ([13-push-benachrichtigungen.md](../spec/13-push-benachrichtigungen.md), Abschnitte 5.1 und 6.4).
   Sie erneuert nie Tokens. Scheitert der Abruf in `client.push.fetch_timeout`, bleibt der
   Platzhalter. Bei `notify.preview_mode = none` holt sie nichts.
2. **Share Extension (iOS, macOS).** Swift und SwiftUI mit eigenem Blatt für das Ziel. Sie sendet
   selbst über `ClientKit` (Upload, dann Nachricht mit eigener `client_msg_id`) und nutzt eine
   eigene Kind-Sitzung mit eigener Refresh-Familie (08, Abschnitt 7.4). Ohne Netz legt sie den
   Eintrag in `share_inbox`; die App übernimmt ihn mit derselben `client_msg_id`, und der Server
   liefert bei Doppel das Original.
3. **Android-Push.** Ein eigener `FirebaseMessagingService` in Kotlin zeigt sofort eine Mitteilung
   mit Platzhalter und startet einen beschleunigten WorkManager-Auftrag, der die Vorschau holt und
   dieselbe Mitteilung ersetzt. Den FCM-Token gibt er über Pigeon an Dart (`push.register`).
4. **Android-Teilen und Windows** brauchen keine Erweiterung: Das Teilen-Ziel auf Android ist eine
   Activity der App mit Flutter-Blatt, die Windows-App hält im Infobereich die Verbindung
   (15, Abschnitte 10.3 und 12).
5. `ClientKit` (Swift Package in `apps/client`) enthält geteilten Schlüsselbund-Zugriff, DPoP,
   Vorschau-Abruf, Upload und Senden. Das Plugin `device_key` nutzt dieselbe Implementierung
   (15, Abschnitt 9).

### Consequences

- Gut, weil keine Engine im Hintergrund startet; die knappe Zeit gehört dem Netzabruf.
- Gut, weil geteilte Inhalte sofort gesendet werden und ohne Netz nicht verloren gehen.
- Gut, weil die NSE keine Refresh-Familie berührt und die Share Extension eine eigene hat; eine
  gleichzeitige Erneuerung löst keine Diebstahl-Erkennung aus.
- Schlecht, weil Logik doppelt entsteht: DPoP-Beweise, Vorschau-Abruf, Upload und Senden gibt es in
  Dart, Swift und für den Abruf auch in Kotlin. Gegenmittel: kleiner Umfang der nativen Teile und
  Vertragstests gegen dieselben Beispiel-Antworten wie der Dart-Client.
- Schlecht, weil Swift und Kotlin weitere Werkzeugketten im Repo und in der Release-Kette sind
  (Xcode-Builds brauchen macOS).
- Schlecht, weil die Speichergrenze der Erweiterungen nicht belegt ist; große Dateien streamt
  `ClientKit` darum von der Platte, und Gerätetests messen den Rest.
- Neutral, weil eine NSE einen nur im privaten Netz erreichbaren Server nur mit aktivem VPN des
  Geräts erreicht; dann bleibt der Platzhalter (15, Abschnitt 10.1).

### Confirmation

- Bau-Ticket M5-04 in [PLAN.md](../PLAN.md): XCTest von `ClientKit`; Gerätetest "Text erscheint bei
  erreichbarem Server, sonst Platzhalter"; bei `notify.preview_mode = none` wird nichts geholt.
- Test ID-T16 aus [08-identitaet.md](../spec/08-identitaet.md), Abschnitt 15, mit der echten Share
  Extension: gleichzeitige Erneuerung von App und Erweiterung löst keine Wiederverwendungs-Erkennung
  aus.
- Bau-Ticket M5-05: instrumentierter Android-Test mit nachgebildeter FCM-Nutzlast; Platzhalter ohne
  Netz.
- Bau-Ticket M5-06: Gerätetest mit `patrol`, ein Bild aus einer anderen App landet im gewählten
  Thread.
- Build-Prüfung in M5-04: Die Ziele der Erweiterungen binden kein Flutter-Framework.

## Pros and Cons of the Options

### Native Erweiterungen mit `ClientKit`

- Gut, weil sie die Systemgrenzen einhalten und ohne Umweg über Dart arbeiten.
- Schlecht, weil zwei weitere Sprachen gepflegt werden.

### Flutter-Engine im Hintergrund

- Gut, weil die Logik aus den Dart-Paketen wiederverwendet würde.
- Schlecht, weil der Start einer Engine einen großen Teil der knappen Zeit kostet und zwei Dienste
  (Flutter-Plugin und eigener Dienst) dieselbe Nachricht empfangen würden (15, Abschnitt 10.2).

### Keine Erweiterungen

- Gut, weil nichts Natives nötig ist.
- Schlecht, weil Mitteilungen nur einen neutralen Platzhalter zeigen und Teilen aus anderen Apps
  fehlt; beides verlangt die Funktionsliste ([01-produkt.md](../spec/01-produkt.md)).

### Push mit Inhalt

- Gut, weil nichts nachgeladen werden muss.
- Schlecht, weil Inhalte über Apple und Google liefen; das verbietet ADR 0031 (R-Push).

### Share Extension legt nur ab

- Gut, weil die Erweiterung klein bleibt.
- Schlecht, weil das Senden davon abhinge, dass die Person die App später öffnet; die Erweiterung
  kann die App nicht selbst öffnen (15, Abschnitt 12). Bleibt als Rückfall ohne Netz.

## More Information

- Belege zu Zeitgrenzen, `NSExtensionContext.open` und FCM stehen in
  [15-client-architektur.md](../spec/15-client-architektur.md), Abschnitte 10 und 12, und in
  [13-push-benachrichtigungen.md](../spec/13-push-benachrichtigungen.md), Abschnitt 18.
- Verwandt: ADR 0020 (Gerätebindung, zwei Geräteschlüssel nach Ruling L10), ADR 0027 (Flutter),
  ADR 0029 (Client-Zustand und Ausgangsfach), ADR 0031 (Push ohne Inhalt).
