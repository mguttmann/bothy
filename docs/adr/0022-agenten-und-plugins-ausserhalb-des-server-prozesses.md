---
status: accepted
date: 2026-10-10
decision-makers: Planungsleitung, Team Agenten-Plattform
---

# Agenten und Plugins außerhalb des Server-Prozesses

## Context and Problem Statement

Das Produkt soll KI-Agenten verschiedener Herkunft (Hermes Agent, Claude Code, eigene Agenten)
und Integrationen (Code-Hosting, Tickets, Monitoring) einbinden. Agenten-Laufzeiten sind groß,
in fremden Sprachen geschrieben, starten Werkzeuge mit Dateisystem- und Netzzugriff und halten
eigene Zugangsdaten. Wo laufen sie, und wie sprechen sie mit dem Server?

## Decision Drivers

- Sicherheit: Ein Fehler oder Einbruch in einer Erweiterung darf den Server, seine Datenbank und
  seine Schlüssel nicht erreichen (Bedrohungsmodell, Baseline B6).
- Zugangsdaten: Der Server darf keine Claude.ai-Zugangsdaten oder Abo-Tokens speichern (B10, I5).
- Rechte: Jede Aktion einer Erweiterung muss durch `authorize` gehen (B7).
- Freie Sprache und freier Ort für Erweiterungen (Python für Hermes, Node.js für Integrationen).
- Die öffentliche API soll vom ersten Tag an von echten Klienten benutzt werden.
- Betrieb: Ein Neustart des Servers darf keinen Agenten-Turn töten, und umgekehrt.

## Considered Options

1. Eigene Prozesse, die nur die öffentliche API nutzen (REST, Gateway-WebSocket, signierte
   Webhooks), als Prinzipale mit Token, Freigaben und Scopes.
2. Plugins im Server-Prozess (Go-Plugins oder eingebettete WebAssembly-Laufzeit mit
   Host-Funktionen).
3. Beistellprozesse mit einem privaten RPC-Protokoll (z. B. gRPC) neben der öffentlichen API.

## Decision Outcome

Gewählt: Option 1, weil sie als einzige alle Treiber erfüllt: harte Prozessgrenze, keine
Zugangsdaten im Server, jede Aktion über `authorize`, freie Sprachwahl, und die öffentliche API
wird durch die Referenz-Adapter selbst geprüft.

### Consequences

- Gut, weil eine kompromittierte Erweiterung nur so viel kann wie ihr Token mit Freigaben und
  Scopes.
- Gut, weil Hermes, der Claude-Code-Runner und fremde Agenten denselben Vertrag nutzen
  (docs/spec/09-agenten.md); der Hermes-Adapter ist ein Klient unter mehreren.
- Gut, weil Server und Laufzeiten getrennt neu starten.
- Schlecht, weil jeder Aufruf über das Netz geht (Latenz, Wiederholungen, Idempotenz nötig). Das
  Protokoll trägt deshalb Cursor, `ack`, Client-IDs und Compare-and-swap (04).
- Schlecht, weil Live-Box-Patches eine Drossel und ein Sammelfenster brauchen.

### Confirmation

- Architektur-Prüfung im Gate: Das Server-Modul importiert weder das Go-Paket `plugin` noch eine
  WebAssembly-Laufzeit; die Abhängigkeitsregel (ADR 0008) schlägt sonst fehl.
- Die Vertragstests (docs/spec/09-agenten.md Abschnitt 16) laufen mit Fake-Agent, Fake-Hermes und
  Fake-Claude ausschließlich gegen die öffentliche API des laufenden Servers.
- Ein Test zählt alle Routen und Gateway-Ops und verlangt je eine Aktion aus dem Katalog (05).

## Pros and Cons of the Options

### Option 1: Eigene Prozesse über die öffentliche API

- Gut, weil die Prozessgrenze auch Speicher, Dateisystem und Netz trennt.
- Gut, weil Erweiterungen in jeder Sprache möglich sind.
- Neutral, weil der Betreiber mehr Prozesse betreibt (Compose-Dienste oder eigene Rechner).
- Schlecht, weil Netzfehler und Doppelzustellungen behandelt werden müssen.

### Option 2: Plugins im Server-Prozess

- Gut, weil Aufrufe ohne Netz und ohne Serialisierung laufen.
- Schlecht, weil Go-Plugins an Compiler- und Abhängigkeitsversionen gebunden sind und keinen
  Speicherschutz bieten.
- Schlecht, weil eine WebAssembly-Sandbox Agenten-Laufzeiten mit Werkzeugen, Unterprozessen und
  großen Python-Ökosystemen nicht tragen kann; jede Fähigkeit bräuchte eine Host-Funktion.
- Schlecht, weil Zugangsdaten der Laufzeit dann im Server-Prozess lägen (B10).

### Option 3: Beistellprozesse mit privatem RPC

- Gut, weil die Prozessgrenze bleibt.
- Schlecht, weil ein zweites Protokoll neben der öffentlichen API entsteht, das eigene
  Autorisierung, Versionierung und Tests braucht.
- Schlecht, weil die öffentliche API dann nicht von den wichtigsten Klienten benutzt wird und
  Lücken unbemerkt bleiben.

## More Information

- Baseline B6, B7, B10; Spezifikation docs/spec/09-agenten.md, docs/spec/11-erweiterungen.md.
- Die Mehrbenutzer-Abschottung von Dev-Laufzeiten ist eine eigene Entscheidung (ADR 0046).
