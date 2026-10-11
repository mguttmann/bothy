---
status: accepted
date: 2026-10-11
decision-makers: Leitung der Planung
consulted: Team A (Architektur), Team B (API), Team C (Rechte)
informed: alle Planungsteams
---

# Maschinenlesbarer Aktionskatalog als einzige Quelle für Doku, Lint und Code

## Context and Problem Statement

Der Aktionskatalog in [05-aktionen.md](../spec/05-aktionen.md) ist geschlossen und normativ: Jede
Route, jede Op, jedes Ereignis und jede Zustellung nennt eine Aktion daraus (B7, Regeln 1.1 bis
1.5). Heute steht er als Markdown-Tabelle. Dieselben Listen stehen noch einmal in
`docs/api/redocly.yaml` (Lint-Regel `authz-action-known`), in den Tabellen von
[02-architektur.md](../spec/02-architektur.md) (Abschnitte 4.6 und 4.7), in `x-receive-action` der
AsyncAPI und künftig in den Go-Konstanten von `kernel/authorize` und der Routen-Tabelle. Jede
Kopie kann abweichen; der Start-Check des Servers und der Routen-Test brauchen eine Liste, die ein
Programm lesen kann. Wo steht der Katalog, und was entsteht daraus?

## Decision Drivers

- Eine Quelle je Tatsache; Kopien entstehen nur durch Erzeugen und werden im Gate verglichen.
- Spec-first (ADR 0013): Die Spezifikation führt, der Code folgt.
- Der Katalog bleibt für Menschen in 05 lesbar; Review sieht jede Änderung.
- Start-Check, Routen-Test und Lint brauchen dieselben Daten (05, Regeln 1.3 bis 1.5).
- Werkzeuge im Repo sind Python, der Server ist Go; beide müssen die Quelle lesen.

## Considered Options

- YAML-Datei `docs/spec/actions.yaml` als einzige Quelle; Tabellen in 05, Listen in
  `redocly.yaml`, Go-Konstanten und Routen-Tabelle werden daraus erzeugt
- Markdown-Tabellen in 05 bleiben die Quelle und werden von Werkzeugen gelesen
- Go-Code als Quelle (Deklarationen in `kernel/authorize`), Doku daraus erzeugt
- OpenAPI- und AsyncAPI-Erweiterungen als Quelle
- Mehrere gepflegte Listen mit Abgleich-Skript (heutiger Stand, Gate G2 in 06)

## Decision Outcome

Gewählte Option: "`docs/spec/actions.yaml` als einzige Quelle", weil Menschen sie lesen,
Python und Go sie ohne Parser-Tricks laden und jede Kopie mechanisch entsteht.

1. Inhalt der Datei: alle Aktionen mit den Spalten aus 05 (ID, Beschreibung, Ressource, Recht,
   Owner, Step-up, Audit, Rechte, Rate-Limit-Klasse), die App-Scopes aus 05, Abschnitt 2.6, die
   Rate-Limit-Klassen mit Schlüssel und Zweck, die dauerhaften und kurzlebigen Ereignistypen mit
   ihrer Empfangs-Aktion (05, Abschnitt 6) und die WebSocket-Ops mit ihrer Aktion (Abschnitt 7).
   Weitere Merkmale kommen nur über eine ADR dazu (zum Beispiel die Wirkung für den Wartungsmodus,
   ADR 0065) oder über den Kern-Vertrag (je Ereignistyp, ob der Umschlag `resource.version`
   trägt, [04-sync-protokoll.md](../spec/04-sync-protokoll.md), "Zustellgarantie").
2. Erzeugt werden: die Tabellen der Abschnitte 3, 4, 6 und 7 in 05 zwischen Markierungen, die
   Aktionslisten in `docs/api/redocly.yaml` (ohne Systemaktionen, weil keine Route sie nennen
   darf), die Konstanten und Merkmale in `kernel/authorize` und die Zuordnung `operationId` zu
   Aktion in `api/routes` ([02-architektur.md](../spec/02-architektur.md), Abschnitte 4.5 und 6).
   Von Hand bleiben die Tabelle "Scopes für Apps" (05, Abschnitt 2.6; ihre Zeilen enthalten
   Prosa), die Liste "Wirkung" (05, Abschnitt 2.7; eine geschlossene Regel in Worten) und die
   Konfigurationswerte von 05 (Quelle der Konfigurationsreferenz); der Check vergleicht sie mit
   der Datei. Zellen der Empfangs-Aktion, die mehrere Fälle in Worten nennen
   (z. B. "self (Agent) und `agent.events.consume`"), behalten ihren Text; der maschinelle Wert
   steht daneben und gleicht `x-receive-action` in asyncapi.yaml.
3. Die Zuordnung Aktion zu Modul steht nicht hier, sondern im Modulregister (ADR 0050). Ein Check
   verlangt, dass beide Dateien genau dieselbe Menge an Aktionen nennen.
4. Neue Aktionen entstehen zuerst in `actions.yaml`, dann in den Verträgen (06, Lint).
5. Bis das Bau-Ticket M1-02 die Datei liefert, gilt 05 als Quelle und das Abgleich-Gate G2 aus
   [06-api.md](../spec/06-api.md), Abschnitt 15, hält die Kopien gleich (05, Regel 1.3).

### Consequences

- Gut, weil eine neue Aktion an einer Stelle entsteht und Lint, Server-Start-Check, Routen-Test
  und Doku sie gleichzeitig kennen.
- Gut, weil der Katalog geschlossen bleibt: Eine Aktion, die nicht in der Datei steht, gibt es im
  Code nicht (05, Regel 1.5).
- Schlecht, weil ein Generator und ein Check gepflegt werden müssen und Änderungen an den
  Tabellen in 05 nur über die YAML-Datei gehen; eine Handänderung in 05 schlägt im Gate fehl.
- Schlecht, weil die Beschreibung einer Aktion in der YAML-Datei steht und Reviewer zwei Stellen
  sehen (Quelle und erzeugte Tabelle); der Diff der erzeugten Tabelle zeigt die Wirkung.
- Neutral, weil 05 normativ bleibt: Die erzeugte Tabelle ist der Text, die YAML-Datei seine Quelle.

### Confirmation

- `pytest tools/catalog/tests`: doppelte Aktions-ID, unbekannte Rate-Limit-Klasse, Ereignistyp
  ohne Empfangs-Aktion, Op ohne Aktion und ein abweichender Tabellenblock in 05 lassen den Check
  scheitern; ein Test belegt, dass die Tabellen in 05 vor und nach dem Umbau dieselben Zeilen haben.
- `make catalog-check` im Ziel `gates` (ADR 0038): liest `docs/spec/actions.yaml`, Kapitel 05,
  `docs/api/redocly.yaml` und, falls vorhanden, `docs/api/.redocly.lint-ignore.yaml`; erneutes
  Erzeugen ergäbe keinen Diff in 05 und `redocly.yaml`. Die erzeugten Konstanten in
  `kernel/authorize` (M1-25) und die Routentabelle in `api/routes` (M1-17) hält
  `make codegen-check`.
- `make api-lint`: Regeln `authz-action-required` und `authz-action-known` mit der erzeugten Liste.
- Server-Start-Check und Routen-Test aus 05, Abschnitt 9 (Tests 1 und 2), gegen die erzeugten
  Konstanten.
- Mengenvergleich mit dem Modulregister (ADR 0050).
- Bau-Ticket M1-02 in [PLAN.md](../PLAN.md).

## Pros and Cons of the Options

### YAML-Datei als Quelle

- Gut, weil strukturiert, versioniert und von allen Werkzeugen lesbar.
- Schlecht, weil ein weiteres Format neben Markdown und OpenAPI.

### Markdown-Tabellen als Quelle

- Gut, weil kein zweites Format entsteht.
- Schlecht, weil Tabellen mit Fließtext, Verweisen und Code-Spans beim Lesen per Programm brechen,
  sobald jemand die Formatierung ändert; Merkmale wie Scopes stehen in anderen Abschnitten.

### Go-Code als Quelle

- Gut, weil der Compiler die Liste prüft.
- Schlecht, weil die Doku dann aus dem Code folgt und nicht umgekehrt (gegen ADR 0013), und weil
  Doku-Werkzeuge eine Go-Umgebung bräuchten.

### OpenAPI- und AsyncAPI-Erweiterungen als Quelle

- Gut, weil die Verträge die Aktionen ohnehin nennen.
- Schlecht, weil Systemaktionen, interne Zustellprüfungen (`push.deliver`, `webhook_out.deliver`)
  und Aktionen ohne Route dort keinen Ort haben (05, Regel 1.10 und R-AZ-11); die Merkmale (Recht,
  Step-up, Audit) wären über viele Operationen verstreut.

### Mehrere Listen mit Abgleich

- Gut, weil sofort ohne Generator möglich; so ist der Stand bis M1-02.
- Schlecht, weil jede Änderung an mehreren Stellen von Hand geschieht und der Abgleich nur meldet,
  nicht verhindert.

## More Information

- Beschluss im Kern-Vertrag: 05, Regel 1.3; [02-architektur.md](../spec/02-architektur.md),
  Abschnitt 20 (Zeile "Maschinenlesbarer Aktionskatalog").
- Verwandt: ADR 0013 (Spec-first), ADR 0017 (eine Autorisierungsfunktion), ADR 0050
  (Modulregister), ADR 0065 (Wartungsmodus).
