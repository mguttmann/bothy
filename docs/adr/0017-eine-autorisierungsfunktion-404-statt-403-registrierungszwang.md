---
status: accepted
date: 2026-10-10
decision-makers: Planungsleitung
---

# Eine Autorisierungsfunktion, 404 statt 403, Registrierungszwang

## Context and Problem Statement

Das Produkt hat viele Eingänge, die Rechte brauchen: REST-Routen, WebSocket-Ops, Zustellung
dauerhafter und kurzlebiger Ereignisse, Push, ausgehende Webhooks, Suche, Zähler, Datei-Abrufe,
Agenten-Aufrufe, Komponenten-Klicks und Befehle. Agenten machen Fehler teuer: Ein Agent, der im
falschen Kanal antwortet oder für die falsche Person handelt, gibt Inhalte weiter oder löst Wirkung
aus. In einem früheren System entstand genau dieser Fehler aus verstreuten Erlaubnislisten an
mehreren Stellen (Studie B, Lehre aus einer früheren Chat-Plattform).

Wie stellen wir sicher, dass jeder Eingang geprüft wird, dass die Prüfung überall gleich entscheidet
und dass Abgelehnte nichts über fremde Bereiche erfahren?

## Decision Drivers

- Eine Wahrheit: dieselbe Frage ergibt überall dieselbe Antwort.
- Vollständigkeit prüfbar: Ein Test muss erkennen, wenn ein Eingang ohne Prüfung entsteht.
- Keine Auskunft über Existenz fremder Ressourcen.
- Fail closed bei Fehlern und unbekannten Fällen.
- Latenz: Prüfung bei jeder Zustellung, also sehr häufig.
- Erklärbarkeit für die Verwaltung ("Wer hat Zugriff?").

## Considered Options

1. Eine Funktion `authorize(principal, action, resource)` im Modul `authz`, geschlossener
   Aktionskatalog, Registrierungszwang für jede Route, Op und Ereignisart.
2. Prüfungen je Handler (Middleware je Route, eigene Abfragen in den Modulen).
3. Externe Policy-Engine als eigener Dienst mit eigener Policy-Sprache oder eigenem
   Beziehungsmodell.
4. Row-Level Security in PostgreSQL als Hauptmechanismus.

## Decision Outcome

Gewählte Option: 1, weil sie als einzige Vollständigkeit testbar macht (jede Registrierung braucht
eine Aktion aus einem geschlossenen Katalog), überall dieselbe Logik benutzt und im Prozess ohne
Netzwerkweg entscheidet.

Festlegungen:

- Jede Route trägt `x-authz-action`, jede Op und jede Ereignisart eine Aktion bzw. Empfangs-Aktion
  ([../spec/05-aktionen.md](../spec/05-aktionen.md)). Der Server startet nicht, wenn eine
  registrierte Route ohne bekannte Aktion ist.
- Kein Leserecht ergibt `404`, gleich ob die Ressource existiert. `403` nur, wenn Lesen erlaubt ist,
  aber Stufe, Zusatzrecht oder Scope fehlen (R-AZ-01).
- Listen, Zähler und Suche filtern in SQL mit der Menge sichtbarer Kanäle aus `authz` und prüfen
  jedes Element zusätzlich mit `authorize` ([../spec/07-rechte.md](../spec/07-rechte.md),
  Abschnitt 7).
- Unbekannte Aktion: `500`, Alarm, nie Erlaubnis.

### Consequences

- Gut, weil ein vergessener Eingang zum Startfehler bzw. Testfehler wird statt zur Lücke.
- Gut, weil Rechte-Änderungen an einer Stelle wirken, auch auf Push, Webhooks und Agenten.
- Gut, weil `404` für alles Unsichtbare keine Bereiche, Kanäle oder IDs verrät.
- Schlecht, weil `authz` auf dem heißen Pfad jeder Zustellung liegt; das verlangt einen Rechte-Cache
  mit Versionszähler und Lasttests.
- Schlecht, weil `404` für Menschen manchmal schwer verständlich ist. Ausgleich: "Wer hat Zugriff?"
  und "Als Person ansehen" für die Verwaltung.
- Schlecht, weil jede neue Funktion zuerst eine Aktion im Katalog braucht; das ist gewollte Reibung.

### Confirmation

- Routen-Test, Op- und Ereignis-Test, Katalog-Test (07, AZ-P1 bis AZ-P3).
- 404-Gleichheitstest für Status, Körper, Kopfzeilen und Antwortzeit (AZ-P4).
- Eigenschaftstest "Zähler und Suche verraten nichts" (AZ-P5) und Lint auf das SQL-Prädikat
  (AZ-P12).
- Lint-Regeln `authz-action-required` und `authz-action-known` auf das OpenAPI-Dokument (05,
  Regel 3).

## Pros and Cons of the Options

### 1. Eine Funktion mit Katalog und Registrierungszwang

- Gut, weil Vollständigkeit maschinell prüfbar ist.
- Gut, weil die Entscheidung im Prozess fällt (keine zusätzliche Latenz über das Netz).
- Neutral, weil die Rechte-Logik eigener Code ist und eigene Tests braucht.
- Schlecht, weil der Katalog gepflegt werden muss.

### 2. Prüfungen je Handler

- Gut, weil schnell zu schreiben.
- Schlecht, weil genau so verstreute Logik entsteht, die zum Fehler im früheren System führte.
- Schlecht, weil Vollständigkeit nicht prüfbar ist.

### 3. Externe Policy-Engine

- Gut, weil Regeln deklarativ und getrennt vom Code sind.
- Schlecht, weil ein zusätzlicher Dienst im Pfad jeder Zustellung liegt (Latenz, Ausfall, Betrieb).
- Schlecht, weil Invarianten wie I1 Transaktionen über Freigaben, Gruppen und Bediener brauchen,
  die in derselben Datenbank-Transaktion geprüft werden müssen.
- Schlecht, weil eine zweite Sprache für Rechte im Projekt entsteht.

### 4. Row-Level Security in PostgreSQL

- Gut, weil die Datenbank selbst filtert, auch bei vergessenen Prädikaten.
- Schlecht, weil Push, Webhooks, WebSocket-Zustellung und Agenten-Aufrufe nicht über SQL laufen
  und trotzdem eine zweite Implementierung bräuchten.
- Schlecht, weil Ablaufzeiten, Schutzregeln und Auftragsbindung in Richtlinien schwer lesbar und
  schwer testbar werden.
- Neutral: als zusätzliche Verteidigungsschicht später möglich, nicht als Wahrheit.

## More Information

- Rechte-Modell und Ablauf: [../spec/07-rechte.md](../spec/07-rechte.md).
- Aktionskatalog: [../spec/05-aktionen.md](../spec/05-aktionen.md).
- Freigabe-Modell: ADR 0018.
- Studie C (Prüfstellen P1 bis P7) ist die fachliche Vorlage; Baseline B7 legt die Funktion fest.
