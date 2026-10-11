---
status: accepted
date: 2026-10-10
decision-makers: Planungsleitung, Team G Plattformdienste
---

# Push ohne Inhalt, Inhalt holt die App selbst

## Context and Problem Statement

Mobile Apps auf iOS und Android bekommen im Hintergrund nur über Apple Push Notification service
(APNs) und Firebase Cloud Messaging (FCM) Nachrichten. Eine dauerhafte eigene Verbindung im
Hintergrund ist auf iOS nicht möglich. Push-Anbieter sind Dritte. Wie zeigt das Gerät einen
lesbaren Hinweis, ohne dass Apple oder Google Nachrichteninhalte sehen und ohne dass ein Push
Rechte umgeht, die sich nach dem Versand geändert haben?

## Decision Drivers

- Kein Nachrichtentext, kein Name bei Apple oder Google.
- Rechte zur Anzeigezeit, nicht nur zum Versandzeitpunkt (B7, Sync-Grundregel).
- Funktioniert mit den Grenzen der Plattformen (Nutzlast, Laufzeit der Erweiterung).
- Möglichst wenig Schlüsselverwaltung zusätzlich zur Gerätebindung.
- Verhalten, wenn das Gerät den eigenen Server gerade nicht erreicht.

## Considered Options

- Push nur mit IDs und neutralem Platzhalter; die App (iOS: Notification Service Extension)
  holt den Inhalt über die API
- Push mit Klartext-Inhalt
- Push mit Ende-zu-Ende-verschlüsseltem Inhalt für das Gerät
- Kein Push, nur eigene Verbindung

## Decision Outcome

Gewählte Option: "Push nur mit IDs und neutralem Platzhalter", weil sie keinen Inhalt preisgibt
und die Rechte beim Abholen erneut prüft. Die Nutzlast enthält Art, Hinweis-ID, Instanz-,
Thread- und Nachrichten-ID; Titel und Text sind Schlüssel in den Sprachdateien der App. Die
iOS-Erweiterung bzw. die Android-App holt eine Vorschau mit einem eigenen, gerätegebundenen
Vorschau-Token. Details: [13-push-benachrichtigungen.md](../spec/13-push-benachrichtigungen.md).

### Consequences

- Gut, weil Apple und Google nur Zeitpunkte, Tokens und zufällige IDs sehen.
- Gut, weil ein Entzug nach dem Versand wirkt: Der Abruf antwortet `404`, das Gerät zeigt einen
  neutralen Text.
- Gut, weil die Nutzlast klein und fest aufgebaut ist und die Apple-Grenze weit unterschreitet.
- Gut, weil der Abruf zugleich als Empfangsbestätigung für die Zustell-Messung dient.
- Schlecht, weil der Inhalt nur erscheint, wenn das Gerät den Server erreicht. Sonst bleibt der
  Platzhalter stehen. Instanzen in privaten Netzen merken das, wenn das VPN des Geräts aus ist.
- Schlecht, weil ein eigenes Vorschau-Token und ein eigener Endpunkt nötig sind.
- Schlecht, weil ohne Apples Filter-Entitlement eine Mitteilung zu einer gelöschten Nachricht
  nicht unterdrückt, sondern nur neutral angezeigt werden kann.

### Confirmation

- Ein Test serialisiert APNs- und FCM-Nutzlasten und prüft gegen eine Erlaubnisliste von
  Schlüsseln und Werttypen (nur IDs, Zahlen, feste Schlüssel).
- Ein Test entzieht Rechte zwischen Ereignis und Abruf und erwartet `404` vom
  Vorschau-Endpunkt.
- Die Bestätigungsquote je Plattform ist ein Signal im Dashboard (Abrufe je Push).

## Pros and Cons of the Options

### Push nur mit IDs und neutralem Platzhalter

- Gut, weil kein Inhalt beim Anbieter liegt.
- Gut, weil `authorize` beim Abruf mit aktuellen Rechten läuft.
- Neutral, weil die Erweiterung auf iOS nur bei sichtbarem Alert mit `mutable-content` läuft
  und etwa 30 Sekunden hat [Q1, belegt]; das reicht für einen API-Abruf.
- Schlecht, weil der Inhalt Erreichbarkeit des Servers braucht.

### Push mit Klartext-Inhalt

- Gut, weil am einfachsten und ohne Erreichbarkeit des Servers.
- Schlecht, weil FCM nur transportverschlüsselt ist, nicht Ende-zu-Ende [Q2, belegt]; Inhalt und
  Namen lägen beim Anbieter.
- Schlecht, weil Rechte nur zum Versandzeitpunkt gelten.

### Push mit Ende-zu-Ende-verschlüsseltem Inhalt

- Gut, weil der Inhalt auch ohne Erreichbarkeit des Servers erscheint.
- Schlecht, weil ein weiterer Schlüssel je Gerät für Verschlüsselung nötig wird (den
  DPoP-Schlüssel dafür mitzunutzen hieße einen Schlüssel für zwei Zwecke).
- Schlecht, weil Rechte nur zum Versandzeitpunkt gelten und ein entzogener Inhalt trotzdem auf
  dem Gerät erscheint.
- Schlecht, weil lange Texte an der Größengrenze der Nutzlast gekürzt werden müssen.
- Kann später ergänzt werden, wenn die Bestätigungsquote zeigt, dass Platzhalter zu oft stehen
  bleiben.

### Kein Push, nur eigene Verbindung

- Gut, weil kein Dritter beteiligt ist.
- Schlecht, weil iOS Apps kurz nach dem Wechsel in den Hintergrund anhält und WebSocket-Aufgaben
  in Hintergrund-Sitzungen nicht unterstützt [Q3, belegt]; mobile Hinweise kämen nie an.
- Für Windows ist das der Weg der ersten Etappen (App im Infobereich), weil WNS
  Paket-Identität und eine Entra-Registrierung braucht.

## More Information

- Quellen (abgerufen 2026-10-10): Q1
  https://developer.apple.com/documentation/usernotifications/modifying-content-in-newly-delivered-notifications
  [belegt]; Q2 https://firebase.google.com/docs/cloud-messaging/customize-messages/set-message-type
  [belegt]; Q3 https://developer.apple.com/forums/thread/716118 [belegt].
- Baseline Ruling R-Push; [13-push-benachrichtigungen.md](../spec/13-push-benachrichtigungen.md).
