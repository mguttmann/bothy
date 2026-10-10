---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team E (Apps und Design), Team F (Betrieb und Observability)
informed: Team C (Datenschutz)
---

# Client-Telemetrie über einen Server-Endpunkt statt Client-OTel-SDK

## Context and Problem Statement

Der Betreiber soll sehen, ob die Apps schnell starten, ob Nachholen und Senden funktionieren und
ob Push-Texte rechtzeitig erscheinen. Telemetrie geht nur über einen OpenTelemetry Collector
hinaus und enthält nie Inhalte (B11, ADR 0034). Für Dart gibt es kein offizielles
OpenTelemetry-SDK; ein Community-SDK ist gespendet, sein Status offen [Q1, belegt]. Der Collector
läuft im internen Netz der Instanz. Wie kommen Kennzahlen aus den Apps in die Telemetrie?

## Decision Drivers

- Export nur über den Collector, kein direkter Weg zu einem Hersteller (B11, 17 Regel 2).
- Keine Inhalte, keine IDs von Threads, Kanälen oder Personen; Metriken ohne Personen-Merkmale
  ([17-observability.md](../spec/17-observability.md), Regeln 3, 8 und 9).
- Der Collector bleibt intern; kein offener Annahme-Endpunkt im Internet.
- Jede Annahme geht durch `authorize` und ein Rate-Limit (B7).
- Abschaltbar durch die Person (`privacy.client_metrics`).

## Considered Options

- Feste, kleine Liste von Kennzahlen, gebündelt an einen eigenen Server-Endpunkt; der Server prüft
  und gibt sie an den Collector weiter
- OpenTelemetry-SDK in der App, Export per OTLP direkt an einen erreichbaren Collector
- Fremdes Mobil-Analyse- oder RUM-SDK
- Keine Kennzahlen aus den Apps, nur Server-Telemetrie und Absturzberichte

## Decision Outcome

Gewählte Option: "Feste Kennzahlen an einen Server-Endpunkt", weil sie B11 hält, ohne den Collector
zu öffnen, und weil der Server jede Annahme prüfen kann. Normativ in
[15-client-architektur.md](../spec/15-client-architektur.md), Abschnitt 17 (R-CL-10), und
[02-architektur.md](../spec/02-architektur.md), Abschnitt 4.6 (Modul `stats`).

1. Die App misst nur die Kennzahlen aus 15, Abschnitt 17: Kaltstart, Thread öffnen, Senden bis
   lokale Anzeige, Frames über Budget, Nachholen, Wiederverbindungen nach Schließ-Code, Tiefe des
   Ausgangsfachs, Push bis Anzeige. Merkmale sind nur Plattform, App-Version, Dichte und Theme.
2. Sie sendet höchstens alle `client.metrics.flush_interval` an `POST /api/v1/client-metrics`
   (Aktion `client_metrics.submit`, Klasse `rl.write`). Ist `privacy.client_metrics` aus, antwortet
   der Server `403 feature_disabled`.
3. Der Server nimmt nur Namen und Merkmale aus der festen Liste an, verwirft alles andere und hängt
   keinen Prinzipal und kein Gerät an. Er schreibt keine Tabelle; die Werte gehen als eigene
   Messreihen mit festen Grenzen an den Collector (17, R-OB-06).
4. Die Apps senden keine Spans. Sie setzen `traceparent` an jeder Anfrage, damit Server-Traces und
   Absturzberichte dieselbe Trace-ID tragen; das Sampling entscheidet der Server (17, Abschnitt 9).
5. Absturzberichte sind ein getrennter Weg (Sentry nur mit konfiguriertem Ziel, ohne Inhalte;
   15, Abschnitt 16).

### Consequences

- Gut, weil der Collector intern bleibt und Apps nur mit dem Server sprechen, den sie ohnehin
  erreichen.
- Gut, weil Datenschutz an der Quelle (feste Liste) und im Server (Erlaubnisliste) gilt, bevor der
  Collector filtert.
- Gut, weil Annahmen angemeldet, gedrosselt und abschaltbar sind.
- Schlecht, weil es keine Client-Traces gibt; ein langsamer Bildschirm zeigt sich nur in der
  Kennzahl, nicht im Ablauf.
- Schlecht, weil vorab aggregierte Histogramme aus vielen Clients sich nicht direkt als Messungen
  eines Go-SDK-Instruments abbilden lassen; das Bau-Ticket muss belegen, wie sie ohne Verfälschung
  in den Collector gelangen (offen, siehe More Information).
- Schlecht, weil der Server eine weitere Annahme-Schnittstelle mit Schema und Liste pflegt; neue
  Kennzahlen brauchen eine Änderung in App, Server und Signal-Katalog.

### Confirmation

- Bau-Ticket M4-31 in [PLAN.md](../PLAN.md): Kanarien-Test der Client-Pfade; Bericht ohne
  Nachrichtentexte und ohne URLs mit Parametern.
- Test: Ein unbekannter Kennzahl-Name oder ein unbekanntes Merkmal wird verworfen und gezählt, nie
  weitergegeben; mit `privacy.client_metrics` aus antwortet der Server `403 feature_disabled`.
- Allowlist- und Kardinalitäts-Test aus [17-observability.md](../spec/17-observability.md),
  Abschnitt 10: Jede weitergegebene Kennzahl steht im Signal-Katalog, ohne IDs als Merkmal.
- Neuer Check im Gate, gebaut mit M4-31: Kein Dart-Paket hängt von einem OpenTelemetry- oder
  Analyse-SDK ab (Sperrliste der Paketnamen gegen `pubspec.lock`; `sentry_flutter` für
  Absturzberichte ist ausgenommen).

## Pros and Cons of the Options

### Server-Endpunkt mit fester Liste

- Gut, weil Autorisierung, Drosselung und Filter ohne neue Infrastruktur entstehen.
- Schlecht, weil nur Kennzahlen, keine Spans.

### OpenTelemetry-SDK in der App

- Gut, weil Standard-Instrumentierung und Spans aus der App möglich wären.
- Schlecht, weil es kein offizielles Dart-SDK gibt [Q1, belegt] und der Collector für Geräte im
  Internet erreichbar und angemeldet sein müsste.

### Fremdes Analyse-SDK

- Gut, weil fertige Auswertungen.
- Schlecht, weil Daten an Dritte gehen (zusätzlicher Auftragsverarbeiter, 20) und B11 umgangen
  wird.

### Keine Client-Kennzahlen

- Gut, weil nichts zu bauen ist.
- Schlecht, weil Leistungsziele der Apps (15, Abschnitt 19) im Betrieb unsichtbar blieben und
  Push-Probleme erst durch Beschwerden auffielen.

## More Information

- Quelle (abgerufen 2026-10-10): Q1 OpenTelemetry Blog, "Dart and Flutter",
  https://opentelemetry.io/blog/2026/dart-flutter-opentelemetry/ [belegt, wie in 15 und 17
  zitiert].
- Offen für Bau-Ticket M4-31: Weg der vorab aggregierten Histogramme in den Collector (eigene
  Datenpunkte über OTLP oder Messung je Bucket) mit einem Test, der Summen und Grenzen vergleicht.
- Verwandt: ADR 0021 (Audit getrennt von Telemetrie), ADR 0027 (Flutter), ADR 0034 (Collector).
