---
status: accepted
date: 2026-10-10
decision-makers: Planungsleitung, Team G Plattformdienste
---

# Volltextsuche in PostgreSQL

## Context and Problem Statement

Menschen und Agenten suchen in Nachrichten, Thread-Titeln, Dateinamen, Transkripten und
Dokumenttexten. Jeder Treffer muss durch die Rechte gefiltert sein (B7, P2); Zähler und
Ausschnitte dürfen nichts aus fremden Kanälen verraten. Die Daten liegen in PostgreSQL (B3). Wo
und wie wird gesucht?

## Decision Drivers

- Rechtefilter in der Abfrage, aus einer Wahrheit (`authz`).
- Kein zweiter Datenspeicher, der synchron gehalten, gesichert und gelöscht werden muss.
- Löschen (auch nach DSGVO) wirkt sofort auf die Suche.
- Deutsch und Englisch mit Wortstämmen; Kennungen und Namen exakt.
- Betrieb auf einem einzelnen Rechner; spätere Skalierung möglich.

## Considered Options

- PostgreSQL-Volltext (`tsvector`, GIN, `websearch_to_tsquery`) hinter einem Port
- Externe Suchmaschine (z. B. Meilisearch, OpenSearch)
- Trigramm-Suche (`pg_trgm`) bzw. `ILIKE`
- Nur Suche im Client

## Decision Outcome

Gewählte Option: "PostgreSQL-Volltext hinter einem Port", weil sie ohne weiteren Dienst
auskommt, in derselben Transaktion konsistent bleibt und den Rechtefilter als einfache
Bedingung in SQL trägt. Generierte Spalten mit Stämmen der Sprache und unveränderten Wörtern,
eine Anfrage als ODER über die Instanz-Sprachen, Kandidaten-Grenze vor dem Ranking,
Hervorhebung mit `ts_headline`. Details: [14-suche.md](../spec/14-suche.md).

### Consequences

- Gut, weil Suchvektoren ohne Trigger und Job immer zum Inhalt passen; eine gelöschte Nachricht
  hat sofort einen leeren Vektor.
- Gut, weil Sicherung, Wiederherstellung und Umzug die Suche ohne Neuaufbau mitnehmen.
- Gut, weil `websearch_to_tsquery` rohe Eingaben ohne Syntaxfehler annimmt [Q1, belegt].
- Schlecht, weil deutsche Komposita nicht zerlegt werden [Q2, belegt]; "Rechnung" findet
  "Rechnungsadresse" nicht.
- Schlecht, weil keine Fehlertoleranz für Tippfehler besteht.
- Schlecht, weil Ranking bei vielen Treffern teuer ist [Q1, belegt]; eine Kandidaten-Grenze
  begrenzt die Kosten, macht sehr alte Treffer bei häufigen Wörtern aber nur über Filter
  erreichbar.
- Schlecht, weil eine Änderung der Vektor-Ausdrücke die Tabelle neu schreibt und ein
  Wartungsfenster braucht.

### Confirmation

- Sprachtests (Deutsch, Englisch, Kennungen) und ein Test, der die Komposita-Grenze festhält.
- Rechtetests: gleiche Anfrage zweier Prinzipale mit verschiedenen Freigaben; keine Treffer aus
  fremden Kanälen und DMs.
- Lasttest im Release-Workflow: p95 unter `search.p95_target` bei `search.benchmark_messages`.
- Migrations-Lint: generierte Suchspalten nur mit `STORED`.

## Pros and Cons of the Options

### PostgreSQL-Volltext hinter einem Port

- Gut, weil ein Datenpfad, eine Wahrheit, transaktional.
- Gut, weil der Port später einen externen Adapter erlaubt.
- Neutral, weil Sprachen über eingebaute Konfigurationen kommen (`german`, `english`,
  `simple` u. a.).
- Schlecht, weil Komposita und Tippfehler nicht abgedeckt sind.

### Externe Suchmaschine

- Gut, weil Tippfehler-Toleranz, Facetten und bessere Relevanz eingebaut sind.
- Schlecht, weil ein weiterer Dienst betrieben, gesichert und überwacht werden muss.
- Schlecht, weil ein Index außerhalb der Datenbank synchron gehalten werden muss; Löschungen
  und Rechte-Änderungen kommen verzögert an.
- Schlecht, weil die Versuchung entsteht, Rechte im Index abzubilden (zweite Rechte-Logik).

### Trigramm-Suche bzw. `ILIKE`

- Gut, weil Teilwörter und Ähnlichkeit gefunden werden.
- Schlecht, weil es keine Wortstämme und kein Ranking nach Relevanz gibt und Trigramm-Indexe auf
  langen Texten groß werden.
- Bleibt als Zwischenschritt für Titel und Dateinamen möglich.

### Nur Suche im Client

- Gut, weil kein Server-Aufwand entsteht.
- Schlecht, weil nur gecachte Nachrichten gefunden werden; ältere Inhalte fehlen.
- Bleibt als Ergänzung für den Offline-Fall.

## More Information

- Quellen (abgerufen 2026-10-10): Q1 https://www.postgresql.org/docs/18/textsearch-controls.html
  [belegt]; Q2 https://www.postgresql.org/docs/current/textsearch-dictionaries.html [belegt].
- Die Abfragen aus 14-suche wurden auf einer Wegwerf-Instanz von PostgreSQL 16 ausgeführt.
- Kriterien für eine externe Suchmaschine: [14-suche.md](../spec/14-suche.md) Abschnitt 9.
