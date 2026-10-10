---
status: accepted
date: 2026-10-10
decision-makers: Planungsleitung, Team Agenten-Plattform
---

# SDKs: generierter Kern plus Handschicht (Python, TypeScript)

## Context and Problem Statement

Agenten-Adapter und Apps sprechen die öffentliche API. Der schwierige Teil ist nicht der einzelne
REST-Aufruf, sondern das Protokoll drumherum: Gateway mit Cursor und `ack`, Prüfung der
Aufruf-Marke, Live-Box-Patches mit Drossel und Version, Idempotenz, Wiederholungen, Signaturen
ausgehender Webhooks. Wie liefert das Produkt SDKs, die mit der Spezifikation Schritt halten und
diese Teile einmal richtig lösen?

## Decision Drivers

- OpenAPI 3.1 und AsyncAPI 3.0 sind normativ (B2, ADR 0013); SDKs dürfen nicht davon abweichen.
- Sicherheitskritische Teile (Marke prüfen, Signaturen, Idempotenz) sollen nicht jeder Adapter
  selbst bauen.
- Der Hermes-Adapter braucht Python, Integrationen meist TypeScript (Baseline B2: Python für SDK
  und Hermes-Adapter).
- Wenig Pflege bei neuen Endpunkten.

## Considered Options

1. Generierter Kern aus der Spec (Python: openapi-python-client, TypeScript: openapi-typescript)
   plus eine handgeschriebene Schicht für Gateway, Marke, Läufe, Komponenten, Befehle, Idempotenz,
   Wiederholungen, Webhook-Signaturen und Testdoppel.
2. Vollständig handgeschriebene SDKs.
3. Nur generierte Clients ohne Handschicht.
4. Nur ein SDK (Python); andere Sprachen nutzen die rohe API.

## Decision Outcome

Gewählt: Option 1, weil der Kern ohne Handarbeit mit der Spec wächst und die schwierigen
Protokollteile genau einmal je Sprache gebaut und getestet werden. Die Generatoren folgen der
Leitungsentscheidung R-Clients.

### Consequences

- Gut, weil neue Operationen mit der nächsten Erzeugung im SDK sind.
- Gut, weil Adapter die Marke, das Gateway und die Patch-Disziplin nicht selbst bauen.
- Gut, weil ein FakeServer im SDK Adapter-Tests ohne echten Server erlaubt.
- Schlecht, weil zwei Handschichten parallel gepflegt werden; ein gemeinsamer Satz von
  Vertragsszenarien (docs/spec/09-agenten.md Abschnitt 16) prüft beide gleich.
- Schlecht, weil eine bruchhafte Änderung der Spec die Handschicht zum Kompilieren zwingt; das ist
  gewollt, weil der Fehler im Gate auffällt statt beim Nutzer.

### Confirmation

- Gate: Kern aus `docs/api/openapi.yaml` erzeugen und kompilieren (R-Clients, ADR 0038).
- Gate: Handschicht-Tests beider SDKs laufen dieselben Szenarien gegen den FakeServer und gegen
  einen echten Server.
- Lint: Die Handschicht definiert keine eigenen Typen für Objekte, die der Kern schon erzeugt.
- Kompatibilitätsmatrix in `sdk/README.md` mit Test je Zeile (docs/spec/11-erweiterungen.md
  Abschnitt 12).

## Pros and Cons of the Options

### Option 1: Generierter Kern plus Handschicht

- Gut, weil Spec-Treue und Komfort zusammenkommen.
- Neutral, weil Generatoren eigene Eigenheiten haben (Spec konservativ halten, Baseline R-Stack).
- Schlecht, weil zwei Schichten erklärt werden müssen.

### Option 2: Vollständig von Hand

- Gut, weil die API so ergonomisch wird, wie man will.
- Schlecht, weil jede Spec-Änderung von Hand nachgezogen werden muss und Abweichungen still
  entstehen.

### Option 3: Nur generiert

- Gut, weil kein Pflegeaufwand.
- Schlecht, weil jeder Adapter Gateway, Marke, Idempotenz und Patches selbst baut und dabei
  Sicherheitsfehler machen kann.

### Option 4: Nur Python

- Gut, weil nur eine Handschicht gepflegt wird.
- Schlecht, weil Integrationen im Webhook-Umfeld häufig in TypeScript entstehen und dort die
  Signaturprüfung und das Gateway fehlen würden.

## More Information

- docs/spec/11-erweiterungen.md Abschnitte 11 und 12; Baseline B2, R-Clients.
- Paketnamen und Registrierungen bei PyPI und npm hängen am endgültigen Produktnamen (Frage an den
  Owner).
