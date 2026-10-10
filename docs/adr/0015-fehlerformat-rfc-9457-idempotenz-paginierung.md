---
status: accepted
date: 2026-10-10
decision-makers: Planungsleitung, Team B (API)
informed: alle Teams
---

# Fehlerformat RFC 9457, Idempotenz, Paginierung

## Context and Problem Statement

Clients arbeiten offline mit einem Ausgangsfach und wiederholen Anfragen (04-sync-protokoll.md,
Abschnitt 13). Sie müssen Fehler maschinell unterscheiden (erneut versuchen, Step-up, verwerfen,
neu laden) und lange Listen seitenweise laden, ohne dass Rechtefilter Mengen verraten. Welches
Fehlerformat, welcher Idempotenz-Mechanismus und welche Paginierung gelten für alle Operationen?

## Decision Drivers

* Ein Format für REST, Gateway-Fehler und Werkzeuge; Standard statt Eigenbau.
* Jede schreibende Anfrage ist wiederholbar ohne Doppel (04, Ziel 2).
* Wiederholung nach `401 step_up_required` oder `429` muss mit derselben ID klappen.
* Listen verraten nichts über unsichtbare Objekte (R-AZ-01, P2).
* Generatoren aller vier Sprachen müssen Fehler und Seiten typisieren können.

## Considered Options

* Fehler: RFC 9457 Problem Details mit eigenem `type` je Code.
* Fehler: eigenes Format `{"error": {"code", "message"}}`.
* Idempotenz: Client-ID im Körper für Nachrichten und Interaktionen, `Idempotency-Key` für alle
  übrigen POST und PATCH.
* Idempotenz: nur `Idempotency-Key` für alles.
* Paginierung: undurchsichtiger Cursor ohne Gesamtzahl.
* Paginierung: Offset und Limit mit Gesamtzahl.

## Decision Outcome

Chosen options: "RFC 9457", "Client-ID plus Idempotency-Key", "Cursor ohne Gesamtzahl", because
sie Standards folgen, die Regeln aus 04 ohne Ausnahme abbilden und keine Mengen verraten.

Festlegungen (docs/spec/06-api.md, Abschnitte 5 bis 8):

* `application/problem+json` mit `type` als relativem URI `/api/v1/problems/{code}`, `code`,
  `correlation_id`; Erweiterungen `errors[]`, `current`, `reason`, `hint`. Jede Instanz liefert
  die Beschreibung eines Codes unter diesem Pfad.
* `400` für Schema- und Regelverstöße, `422` nur für `idempotency_key_reuse`, `409` für
  Zustandskonflikte, `412` für gescheitertes `If-Match`.
* Nachrichten: `client_msg_id`; Interaktionen: `client_interaction_id`; übrige POST und PATCH:
  `Idempotency-Key` (UUIDv7). Gespeichert werden nur endgültige Antworten, nicht `401`, `429`
  und `5xx`.
* Cursor sind an Filter und Sortierung gebunden, Keyset-basiert, ohne Gesamtzahl. Thread-Verlauf
  über `thread_seq`, Ereignisse über `seq`.

### Consequences

* Good, because HTTP-Bibliotheken und Proxys Problem Details kennen und Clients nach `code`
  verzweigen.
* Good, because das Ausgangsfach jede Anfrage blind wiederholen kann.
* Good, because Keyset-Seiten unter Last stabil und billig sind.
* Bad, because relative `type`-URIs von der Empfehlung "absolute URIs" in RFC 9457 abweichen; eine
  spätere Umstellung wäre ein Bruch.
* Bad, because ohne Gesamtzahl zeigt die Oberfläche keine "Seite 3 von 12"; das ist gewollt.
* Bad, because zwei Idempotenz-Wege existieren; Grund ist der dauerhafte Unique-Index für
  Nachrichten (04, 5.1), der kein Ablaufdatum hat.

### Confirmation

Lint (G1): jede Operation dokumentiert 4xx-Antworten aus `components.responses` mit
`application/problem+json`; Beispiele werden gegen Schemas geprüft. Schemathesis (G7) prüft Status,
Medientyp und Schema jeder Antwort und eigene Regeln: Problem-Format bei jedem Fehler,
`Correlation-Id` in jeder Antwort, gleiche `404` für fremde und fehlende IDs. Integrationstests
prüfen Wiederholungen mit derselben ID nach Zeitüberschreitung, Step-up und `429`.

## Pros and Cons of the Options

### RFC 9457

* Good, because Standard mit Medientyp und Erweiterungsregeln (unbekannte Felder ignorieren).
* Neutral, because `title` ist Englisch; Clients übersetzen über `code`.

### Eigenes Fehlerformat

* Good, because kürzer.
* Bad, because jedes Werkzeug muss es neu lernen; kein Medientyp zur Unterscheidung.

### Client-ID plus Idempotency-Key

* Good, because Nachrichten bleiben über Tage und Neuinstallationen eindeutig (04, 5.1).
* Bad, because zwei Mechanismen in der Doku.

### Nur Idempotency-Key

* Good, because ein Mechanismus.
* Bad, because ein Schlüssel mit Ablauf (`sync.idempotency_key_ttl`) schützt tagealte
  Ausgangsfach-Einträge nicht vor Doppeln.

### Cursor ohne Gesamtzahl

* Good, because stabil bei neuen Zeilen und ohne zweite Abfrage.
* Bad, because kein Sprung auf eine beliebige Seite.

### Offset und Limit mit Gesamtzahl

* Good, because vertraut.
* Bad, because Verschiebungen bei neuen Zeilen; Gesamtzahlen über gefilterte Mengen sind teuer und
  verraten Größen.

## More Information

* RFC 9457: https://www.rfc-editor.org/rfc/rfc9457.html (abgerufen 2026-10-10) [belegt].
* draft-ietf-httpapi-idempotency-key-header, Entwurf -07 abgelaufen:
  https://datatracker.ietf.org/doc/draft-ietf-httpapi-idempotency-key-header/ (abgerufen
  2026-10-10) [belegt].
* RFC 9110, Abschnitt 13.1.1 (If-Match, 412): https://www.rfc-editor.org/rfc/rfc9110.html
  (abgerufen 2026-10-10) [belegt].
