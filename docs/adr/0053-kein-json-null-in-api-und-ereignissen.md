---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team B (API), Team E (Apps), Team D (Agenten-Plattform)
informed: alle Planungsteams
---

# Kein JSON-null in API und Ereignissen

## Context and Problem Statement

Viele Felder sind optional: eine Spalte mit NULL, ein Ereignis ohne Akteur, ein Lauf ohne Sitzung.
JSON kennt dafür zwei Formen: das Feld fehlt, oder es steht mit `null` da. Alle Clients werden aus
einer Spezifikation erzeugt (ADR 0013): Go-Server, TypeScript, Python und Dart. Der
Dart-Generator `dart-dio` unterstützt den Null-Typ nicht [Q1, belegt]. Wie drücken API, Gateway
und Webhooks "kein Wert" aus, und wie entfernt ein Client einen Wert?

## Decision Drivers

- Alle vier Generatoren erzeugen aus derselben Spezifikation lauffähigen Code (Gate G4 in
  [06-api.md](../spec/06-api.md)).
- Eine Form für "kein Wert" in Antworten, Anfragen und Ereignissen.
- Keine dreiwertige Logik in Änderungen (fehlt, `null`, Wert), die jeder Client richtig deuten muss.
- Konservative Schemas: kein Typ-Array, kein `anyOf` (06, Abschnitt 14).
- Unbekanntes und Unerwartetes wird toleriert, nicht zum Absturz (06, Abschnitt 2).

## Considered Options

- Kein JSON-`null` auf der Leitung; ein fehlendes Feld heißt "kein Wert"; Entfernen über eigene
  Felder oder Modi
- `null` als ausdrücklicher Wert mit OpenAPI-3.1-Typ `[T, "null"]` und Änderungen nach JSON Merge
  Patch (`null` heißt löschen)
- Platzhalter-Werte für "kein Wert" (leere Zeichenkette, 0, Epochenzeit)
- Hüllen-Objekte für optionale Felder (`{"value": ...}`)

## Decision Outcome

Gewählte Option: "Kein JSON-`null`; fehlend heißt kein Wert", weil sie mit allen Generatoren
funktioniert und Änderungen eindeutig macht. Normativ in
[04-sync-protokoll.md](../spec/04-sync-protokoll.md), Abschnitt 2.2 (R-SY-16),
[03-datenmodell.md](../spec/03-datenmodell.md), Grundsatz 13, und
[06-api.md](../spec/06-api.md), Abschnitt 2 (R-API-03).

1. Der Server sendet nie JSON-`null`, weder in REST-Antworten noch in Gateway-Rahmen, Umschlägen,
   Nutzlasten oder Webhooks. Eine Spalte mit NULL erscheint als fehlendes Feld.
2. Pflichtfelder sind immer da, auch mit Nullwerten wie `0`, `false` oder `""`. Nur optionale Felder
   dürfen fehlen.
3. Werte werden über eigene Felder oder Modi entfernt, nie durch `null`: zum Beispiel entfernt
   `use_global_order: true` die eigene Reihenfolge in der Seitenleiste, und `OperatorSet.mode =
   none` setzt "kein Bediener". In Änderungen heißt ein fehlendes Feld "unverändert".
4. Anfragen mit `null` scheitern an der Schema-Prüfung vor dem Handler (SEC-IN-1 in
   [19-sicherheit.md](../spec/19-sicherheit.md)), weil kein Schema den Null-Typ erlaubt.
   Adapter und Apps lassen ein Feld weg, statt `null` zu senden.
5. Clients behandeln ein empfangenes `null` wie ein fehlendes Feld (Toleranz, 06 Abschnitt 2).

### Consequences

- Gut, weil die Typen in Dart, TypeScript, Python und Go ohne Sonderfälle entstehen.
- Gut, weil eine Änderung genau zwei Fälle kennt (Feld da: setzen; Feld fehlt: lassen) und
  Entfernen sichtbar und auditierbar ist.
- Schlecht, weil jedes entfernbare Feld einen eigenen Modus oder ein eigenes Feld braucht; die API
  wird an diesen Stellen etwas breiter.
- Schlecht, weil Go-Code aufpassen muss: Mit `omitempty` verschwinden auch Nullwerte von
  Pflichtfeldern. Darum tragen nur optionale Felder `omitempty` (als Zeiger oder mit `omitzero`),
  Pflichtfelder nie; die Antwort-Validierung (G8) findet fehlende Pflichtfelder.
- Neutral, weil die Datenbank NULL weiter nutzt; die Grenze liegt in der Kodierung nach außen.

### Confirmation

- Lint-Regel `rule/no-type-arrays-or-null` in `docs/api/redocly.yaml` (Gate G1): kein Schema mit
  Null-Typ oder Typ-Array.
- Eigenschaftstest im Bau-Ticket M1-18: `TestNoNull_PropertyRandomValues` und `FuzzSanitizeJSON`
  in `services/server/internal/platform/httpserver` (der Antwort-Schreiber entfernt `null`-Mitglieder
  und lehnt `null`-Elemente und Ganzzahlen über 2^53 - 1 mit `500` ab; `TestNoNull_NullInArrayGives500`,
  `TestInt53_AboveLimitGives500`, `TestNoNull_CleanBodyByteIdentical`), dazu der Harness
  `httpservertest`, der jeden Handler-Test mit `null` in der Antwort scheitern lässt; ein Test im
  Bau-Ticket M1-16: kein Umschlag enthält JSON-`null`.
- Gate G7 (Schemathesis) mit eigener Prüfung "kein `null`" und Gate G8 (Antwort-Validierung in
  Integrationstests) in [06-api.md](../spec/06-api.md), Abschnitt 15.
- Toleranz-Test G5: Die Handschicht jedes SDK überspringt ein empfangenes `null` ohne Fehler.
- Test: Eine Anfrage mit `null` in einem optionalen Feld ergibt `400` mit `validation_failed`.

## Pros and Cons of the Options

### Kein `null`, fehlend heißt kein Wert

- Gut, weil alle Generatoren es abbilden und Go mit `omitempty` oder `omitzero` es direkt kann.
- Schlecht, weil "Wert entfernen" nicht generisch ist, sondern je Feld entworfen wird.

### `null` mit JSON Merge Patch

- Gut, weil Merge Patch eine bekannte Regel hat: `null` entfernt einen Wert [Q2, belegt].
- Schlecht, weil `dart-dio` den Null-Typ im Schema nicht unterstützt [Q1, belegt]; die Option
  `useOptional` unterscheidet zwar fehlend, `null` und Wert [Q1, belegt], hilft aber nur im
  Dart-Client. Typ-Arrays sind in den Generatoren unterschiedlich weit unterstützt (oapi-codegen
  erst seit v2.8.0, 06 Quelle Q1).
- Schlecht, weil jeder Client drei Zustände (fehlt, `null`, Wert) richtig unterscheiden muss; ein
  Client, der `null` und "fehlt" gleich behandelt, löscht oder behält ungewollt.

### Platzhalter-Werte

- Gut, weil jedes Feld immer da ist.
- Schlecht, weil `0`, `false` und `""` oft echte Werte sind (Zähler, Schalter); Platzhalter
  verwechseln "kein Wert" mit einem gültigen Wert.

### Hüllen-Objekte

- Gut, weil eindeutig.
- Schlecht, weil jedes optionale Feld ein Objekt wird; Schemas, Beispiele und generierter Code
  werden deutlich größer, ohne dass ein Generator es verlangt.

## More Information

- Quellen (abgerufen 2026-10-10): Q1 openapi-generator `dart-dio`,
  https://openapi-generator.tech/docs/generators/dart-dio/ [belegt]; Q2 RFC 7396 JSON Merge Patch,
  Abschnitt 1, https://www.rfc-editor.org/rfc/rfc7396 [belegt].
- `omitzero` gibt es in `encoding/json` seit Go 1.24 (https://go.dev/doc/go1.24, abgerufen
  2026-10-10) [belegt].
- Verwandt: ADR 0013 (Spec-first), ADR 0015 (Fehlerformat), ADR 0026 (SDKs), ADR 0052 (Umschlag).
