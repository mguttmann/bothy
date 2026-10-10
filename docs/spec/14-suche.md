# 14 Suche

Status: Entwurf M0. Normativ für die Volltextsuche im Server, die Dateitext-Extraktion und die
Offline-Suche im Client. Begriffe: [00-glossar.md](00-glossar.md). Tabellen und Suchvektoren:
[03-datenmodell.md](03-datenmodell.md) Abschnitt 5; dieses Kapitel legt dort nach Abschnitt 5
"Sprachwahl, Gewichtung, Ranking, Hervorhebung" fest. Rechte: [07-rechte.md](07-rechte.md) und
[05-aktionen.md](05-aktionen.md) (`search.query`). Dateien: [12-dateien.md](12-dateien.md).
Entscheidung: ADR 0032 (Volltextsuche in PostgreSQL).

Alle Grenzen und Ziele sind benannte Konfigurationswerte mit Standard in Abschnitt 15. Externe
Fakten verweisen auf die Quellenliste (Abschnitt 16). Die SQL-Beispiele wurden auf einer
Wegwerf-Instanz von PostgreSQL 16 ausgeführt. Ziel ist PostgreSQL 18 [Q1, belegt]; die
genutzten Funktionen beschreibt dessen Dokumentation gleich [Q2, Q3, belegt]. Ein Lauf gegen
18 steht als Bau-Test aus.

## 1. Ziele und Grenzen

1. **Rechte vor allem.** Treffer, Ausschnitte und Zähler kommen nur aus Kanälen, die der
   Prinzipal lesen darf. Die Abfrage filtert in SQL (P2), nicht danach.
2. **Deutsch und Englisch gut, alles andere brauchbar.** Wortstämme je Sprache plus eine
   unveränderte Wortform (`simple`) für Namen, Kennungen und fremde Sprachen.
3. **Ein Datenbankpfad.** PostgreSQL-Volltext mit GIN-Index, kein zweiter Datenspeicher
   (ADR 0032). Ein Port erlaubt später eine externe Suchmaschine.
4. **Begrenzte Kosten.** Jede Suche hat eine Obergrenze an Kandidaten und ein Zeitlimit.

Nicht durchsuchbar: Lauf-Schritte und Live-Box-Inhalte (Werkzeugausgaben enthalten oft
Geheimnisse und Lärm), Kanal-Prompts (nur mit `manage` lesbar, R-AZ-09), gelöschte Nachrichten
(leerer Inhalt, leerer Vektor), nicht auslieferbare Dateien ([12-dateien.md](12-dateien.md)
Abschnitt 2), Entwürfe.

## 2. Was durchsuchbar ist

| Ergebnis-Typ | Quelle | Spalte | Gewicht | Bedingung |
|---|---|---|---|---|
| `messages` | Nachrichtentext, Wortstämme | `messages.search_vector` | B | `deleted_at IS NULL` |
| `messages` | Nachrichtentext, unveränderte Wörter | `messages.search_vector` | D | wie oben |
| `threads` | Thread-Titel, Stämme und Wörter | `threads.title_search` | A | Thread sichtbar |
| `files` | Dateiname | `files.search_vector` | A | Datei auslieferbar und an einer sichtbaren, nicht gelöschten Nachricht |
| `files` | Transkript | `files.search_vector` | B | wie oben |
| `files` | Dateitext (Abschnitt 5), Stämme | `file_text_extracts.search_vector` | C | wie oben |
| `files` | Dateitext, unveränderte Wörter | `file_text_extracts.search_vector` | D | wie oben |

Gewichte sind die vier Klassen von PostgreSQL. `ts_rank` gewichtet sie standardmäßig mit
`{0.1, 0.2, 0.4, 1.0}` für D, C, B, A [Q2, belegt]. Wichtig ist das vor allem bei Dateien: Ein
Treffer im Namen zählt mehr als einer im Transkript und mehr als einer im Dokumenttext.
Stammformen (B) zählen mehr als nur gleiche Wortformen (D), weil ein Stamm-Treffer in der
richtigen Sprache der stärkere Hinweis ist; die `simple`-Form fängt den Rest.

## 3. Sprachen und Suchvektoren

### 3.1 Sprache je Objekt

Die Spalte `messages.search_language` (`regconfig`) gibt es schon. Sie wird beim Anlegen
gesetzt und danach nicht mehr geändert, außer durch eine Neuindizierung (Abschnitt 8):

- Mensch als Autor: `search.locale_map[users.locale]` (erste Sprachkennung, z. B. `de`).
- Agent, App, Import, System: `search.locale_map[instance_settings['locale.default']]`.
- Kein Eintrag in der Abbildung: `simple`.

Threads bekommen dieselbe Regel über den Ersteller (Spalte `threads.search_language`, 03).
Dateitext bekommt die Sprache der Instanz.

`search.languages` nennt alle Konfigurationen, mit denen Anfragen gebaut werden. Der Server
startet nicht, wenn ein Wert aus `search.locale_map` dort fehlt; sonst fänden Anfragen die
Stämme dieser Sprache nie. PostgreSQL bringt Konfigurationen für viele Sprachen mit, darunter
`german`, `english` und `simple` (geprüft mit `\dF` auf PostgreSQL 16).

### 3.2 Ausdrücke

Die Spalten sind gespeicherte, generierte Spalten mit GIN-Index wie in
[03-datenmodell.md](03-datenmodell.md) Abschnitt 5; die Ausdrücke dort entsprechen diesem
Abschnitt (mit Gewicht und `simple`-Anteil, dazu die Tabelle `file_text_extracts`):

```sql
-- messages
search_vector tsvector GENERATED ALWAYS AS (
  setweight(to_tsvector(search_language, content_md), 'B') ||
  setweight(to_tsvector('simple'::regconfig, content_md), 'D')
) STORED

-- threads (neue Spalte search_language regconfig NOT NULL DEFAULT 'simple')
title_search tsvector GENERATED ALWAYS AS (
  setweight(to_tsvector(search_language, title), 'A') ||
  setweight(to_tsvector('simple'::regconfig, title), 'A')
) STORED

-- files: unverändert aus 03 (Name A, Transkript B, beide simple)

-- file_text_extracts (neu, Abschnitt 5)
search_vector tsvector GENERATED ALWAYS AS (
  setweight(to_tsvector(search_language, coalesce(text_content, '')), 'C') ||
  setweight(to_tsvector('simple'::regconfig, coalesce(text_content, '')), 'D')
) STORED
```

PostgreSQL 18 legt generierte Spalten ohne Schlüsselwort als virtuelle Spalten an [Q3, belegt].
Jede Migration muss `STORED` darum ausdrücklich schreiben; ein Prüfschritt im Migrations-Lint
lehnt `GENERATED ALWAYS AS` ohne `STORED` ab.

Ein Vektor statt zweier: Stämme und unveränderte Wörter liegen im selben `tsvector` mit eigenem
Gewicht. Eine GIN-Abfrage reicht, kein ODER über zwei Indexe.

### 3.3 Grenzen der Sprachverarbeitung

- Snowball zerlegt keine deutschen Komposita [Q4, belegt]. "Rechnung" findet "Rechnungen", aber
  nicht "Rechnungsadresse" (auf PostgreSQL 16 so beobachtet). Hunspell-Wörterbücher könnten das
  teilweise, werden aber nicht mitgeliefert, und PostgreSQL unterstützt Hunspell-Komposita nur
  in Grundzügen [Q4, belegt]. Ein Test hält diese Grenze fest, damit sie sichtbar bleibt.
- Der deutsche Stemmer faltet Umlaute (`verfügbar` wird `verfugbar`); `simple` behält sie. Beide
  Formen stehen im Vektor.
- Wörter, die der Parser trennt (z. B. `ERR_CONN_RESET` in `err`, `conn`, `reset`), werden über
  die Teile gefunden, als Phrase genauer.

## 4. Anfrage

### 4.1 Eingabe

- Text `q` in der Syntax von `websearch_to_tsquery`: Wörter (UND), `"Phrase"`, `or`, `-Wort`.
  Diese Funktion wirft nie einen Syntaxfehler [Q2, belegt]; jede Eingabe ist sicher.
- Länge höchstens `search.query_max_chars`, sonst `400 validation_failed` mit `errors[]` auf
  `q` und Code `too_long` ([06-api.md](06-api.md), Abschnitt 5).
- Bleibt nach der Verarbeitung kein Begriff übrig (nur Satzzeichen), antwortet der Server
  `400 validation_failed` mit `errors[]` auf `q` und Code `empty`.
- Filter kommen als eigene Parameter, nicht als Text in `q` (4.4). Die App darf eine
  Schreibweise wie `von:Alex` anbieten und in Parameter übersetzen; der Server deutet `q` nicht.

### 4.2 Anfrage-Aufbau

Die Anfrage ist ein ODER über alle Sprachen aus `search.languages` plus `simple`. Leere
Teilanfragen (z. B. nur Stoppwörter in einer Sprache) fallen weg:

```sql
CREATE FUNCTION search_tsquery(langs regconfig[], q text) RETURNS tsquery
LANGUAGE sql STABLE AS $$
  SELECT string_agg('(' || tq::text || ')', ' | ')::tsquery
  FROM (
    SELECT websearch_to_tsquery(l, q) AS tq
    FROM unnest(array_append(langs, 'simple'::regconfig)) AS l
  ) s
  WHERE numnode(tq) > 0
$$;
```

Ergebnis NULL bedeutet "kein Begriff" (4.1). Mit `'{german,english}'` wird aus `Rechnungen`
`'rechnung' | 'rechnungen' | 'rechnungen'`; die GIN-Abfrage nutzt den Index (Plan geprüft).
Die Funktion entsteht mit einer Plattform-Migration.

Bekannte Unschärfe: Verneinung (`-wort`) wirkt je Teilanfrage. Hat ein Text das Wort nur in
einer anderen Form, kann die `simple`-Teilanfrage ihn trotzdem treffen. Die Verneinung ist also
"bestmöglich" und so dokumentiert.

### 4.3 Rechtefilter

1. `authz` liefert die Menge der sichtbaren Kanäle des Prinzipals für `message.read` aus dem
   Rechte-Cache (R-DM-19, [07-rechte.md](07-rechte.md)). Sie enthält DM-Kanäle nur für
   Teilnehmer, auch beim Owner (R-AZ-02).
2. Jede Abfrage enthält `channel_id = ANY($visible)` mit der Liste, die `Authorizer.Scope` je
   Anfrage bildet ([07-rechte.md](07-rechte.md), Abschnitt 7.4; [03-datenmodell.md](03-datenmodell.md)
   Abschnitt 5). Threads filtern über `threads.channel_id`, Dateien über den Kanal der
   Nachricht, an der sie hängen.
3. Die Einträge der ausgelieferten Seite laufen zusätzlich durch `authorize(..., message.read |
   thread.read | file.read, ...)`. Ein abgelehnter Eintrag fällt still weg. Das fängt Rechte,
   die sich zwischen dem Laden der Kanalmenge und der Abfrage geändert haben.
4. Es gibt keine Gesamtzahl und keine Zähler je Kanal. Die Antwort sagt nur, ob es weitere
   Seiten gibt und ob die Kandidaten-Grenze erreicht wurde.
5. Agenten suchen mit ihren Freigaben. Apps brauchen dafür den Scope `messages:read`
   ([05-aktionen.md](05-aktionen.md) 2.6).

Restrisiko: Die Dauer einer Suche hängt auch von Treffern in fremden Kanälen ab, weil der Index
erst danach nach Kanal filtert. Ein Angreifer könnte über viele gemessene Anfragen auf die
Häufigkeit eines Wortes schließen. Kandidaten-Grenze, Zeitlimit und Rate-Limit `rl.search`
begrenzen das; der Punkt steht im Bedrohungsmodell ([19-sicherheit.md](19-sicherheit.md)).

### 4.4 Filter

| Parameter | Wirkung | Typen |
|---|---|---|
| `space_id` (mehrfach) | nur Kanäle dieser Bereiche; Schnittmenge mit den sichtbaren | alle |
| `channel_id` (mehrfach) | nur diese Kanäle; Schnittmenge mit den sichtbaren | alle |
| `author_id` | `messages.author_principal_id` bzw. `files.uploaded_by` | `messages`, `files` |
| `agent_id` | Thread mit `threads.operator_agent_id` = Agent (Gespräche mit diesem Agenten, auch die Nachrichten der Menschen) | alle |
| `after`, `before` | `created_at` im halboffenen Intervall | alle |
| `has_file` | mindestens ein Anhang | `messages` |
| `thread_state` | `open` oder `closed` | `threads`, `messages` |
| `tag_id` (mehrfach) | Forum-Tags des Threads | `threads`, `messages` |

Ein unbekannter oder unsichtbarer Kanal oder Bereich im Filter ergibt keine Treffer, keinen
Fehler (sonst verriete der Fehler die Existenz).

### 4.5 Ranking und Sortierung

`sort = relevance` (Standard) oder `newest`.

1. Kandidaten: die neuesten `search.max_candidates` Treffer nach Filter und Rechten. Ranking
   muss den Vektor jedes Treffers lesen und ist bei vielen Treffern teuer [Q2, belegt]; die
   Grenze macht die Kosten unabhängig von der Treffermenge. Wird sie erreicht, meldet die
   Antwort `candidates_capped: true`, und die App bittet um engere Suche.
2. `relevance`: Wert = `ts_rank(vector, query, search.rank_normalization)` geteilt durch
   `1 + Alter / search.recency_scale`. Normalisierung 1 teilt durch `1 + log(Länge)` [Q2,
   belegt], damit lange Nachrichten nicht allein durch Länge gewinnen. Bei Gleichstand gewinnt
   die höhere ID (neuer).
3. `newest`: nach `created_at` absteigend, dann ID.

### 4.6 Hervorhebung

- Nur für die ausgelieferte Seite, weil `ts_headline` das Original statt des Vektors liest und
  langsam sein kann [Q2, belegt].
- Eingabe gekürzt auf `search.headline_input_max_chars`.
- Optionen: `MaxFragments = search.headline_max_fragments`, `MaxWords =
  search.headline_max_words`, `MinWords = search.headline_min_words`, `ShortWord=3` [Q2, belegt].
- Markierungen: `StartSel` und `StopSel` sind je Anfrage zufällige Hex-Zeichenketten (128 Bit).
  Der Server zerlegt das Ergebnis daran in Segmente `[{text, match}]`. Ein Text kann die
  Markierung nicht vorab enthalten; HTML im Inhalt bleibt Text. Die Standard-Markierungen `<b>`
  wären unsicher, weil `ts_headline` keine sichere HTML-Ausgabe garantiert [Q2, belegt].

### 4.7 Seiten

Cursor nach (Wert, ID) bei `relevance` und (`created_at`, ID) bei `newest`, undurchsichtig
kodiert, mit einem Hash aus `q`, Filtern und Sortierung. Passt der Hash nicht zur Anfrage:
`400 cursor_invalid` (06, Abschnitt 5). Seitengröße `search.page_size_default`, höchstens
`search.page_size_max`.

### 4.8 Beispiel: Nachrichten

Geprüft auf PostgreSQL 16. `$4` ist die sichtbare Kanalmenge, `$5` die Kandidaten-Grenze.

```sql
WITH p AS (
  SELECT search_tsquery($1::regconfig[], $2) AS tsq
),
cand AS (
  SELECT m.id, m.created_at, m.search_language, m.content_md,
         ts_rank(m.search_vector, p.tsq, $3) AS rank
  FROM messages m, p
  WHERE m.search_vector @@ p.tsq
    AND m.channel_id = ANY ($4::uuid[])
    AND m.deleted_at IS NULL
  ORDER BY m.created_at DESC
  LIMIT $5
),
page AS (
  SELECT c.*, c.rank / (1 + extract(epoch FROM now() - c.created_at) / $6) AS score
  FROM cand c
  ORDER BY score DESC, id DESC
  LIMIT $7
)
SELECT pg.id, pg.score,
       ts_headline(pg.search_language, left(pg.content_md, $8), p.tsq,
                   'StartSel=' || $9 || ',StopSel=' || $10 ||
                   ',MaxFragments=2,MaxWords=30,MinWords=10,ShortWord=3') AS snippet
FROM page pg, p
ORDER BY pg.score DESC, pg.id DESC;
```

`$6` ist `search.recency_scale` in Sekunden; die Optionswerte im Beispiel sind die Standards aus
Abschnitt 15 und kommen im Code aus der Konfiguration. Filter aus 4.4 und der Cursor werden als
weitere Bedingungen in `cand` bzw. `page` ergänzt.

### 4.9 Präfix-Suche für den Schnellwechsel

Der Schnellwechsel (Cmd/Ctrl+K, [01-produkt.md](01-produkt.md)) sucht Bereiche und Kanäle in der
lokalen Momentaufnahme des Clients und Thread-Titel beim Server mit Präfix:

```sql
SELECT string_agg(quote_literal(lexeme) || ':*', ' & ')::tsquery
FROM unnest(to_tsvector('simple', $1))
WHERE length(lexeme) >= $2;   -- search.prefix_min_chars
```

Aus `rechnungs okt` wird `'okt':* & 'rechnungs':*`; das findet "Rechnungslauf Oktober" (geprüft).
Wörter unter `search.prefix_min_chars` fallen weg; bleibt nichts, ist das Ergebnis NULL und die
Antwort leer.
Präfix-Anfragen sind in `to_tsquery` mit `:*` vorgesehen [Q2, belegt]. Gleicher Rechtefilter wie
4.3, Sortierung nach `last_activity_at`.

## 5. Dateitext-Extraktion

### 5.1 Ablauf

1. Ist eine Datei auslieferbar ([12-dateien.md](12-dateien.md) Abschnitt 2) und ihr Typ
   unterstützt, legt der Worker eine Zeile `file_text_extracts` (`state = 'pending'`) an und
   reiht den Job `files.extract_text` in die Warteschlange `files`
   ([02-architektur.md](02-architektur.md), Abschnitt 12.3).
2. Der Job ruft den Port `TextExtractor` (`Extract(ctx, reader, mime) -> text, truncated`).
3. Ergebnis: `state = 'done'`, Text höchstens `search.extract.max_text_bytes` (`truncated`
   markiert Kürzung), `extractor` mit Name und Version, `extracted_at`. Fehler: `failed` mit
   Grund ohne Inhalt. Nicht unterstützter Typ: `unsupported`.
4. Steigt die Version eines Extraktors, holt ein Wartungsjob ältere Zeilen nach.

### 5.2 Typen und Werkzeuge

| Typ | Adapter | Lizenz | Hinweis |
|---|---|---|---|
| `text/plain`, `text/markdown`, `text/csv`, `application/json`, Quelltext | `builtin` | eigener Code | UTF-8 prüfen, ungültige Bytes ersetzen |
| `text/html` | `builtin` mit `golang.org/x/net/html` | BSD-3-Clause [Q7, belegt] | nur Textknoten; Skripte und Stile fallen weg |
| DOCX, XLSX, PPTX | `builtin` mit `archive/zip` und `encoding/xml` | Go-Standardbibliothek | Text aus Dokument, gemeinsamen Zeichenketten und Folien |
| ODT, ODS, ODP | `builtin` | Go-Standardbibliothek | `content.xml` |
| PDF, ältere Office-Formate, RTF, EPUB und weitere | `tika` (optional) | Apache-2.0 [Q8, abgeleitet] | Apache Tika Server als eigener Container |

`builtin` ist reines Go und immer an. `tika` ist aus (`search.extract.tika_url` leer), weil es
eine Java-Laufzeit mit eigenem Speicherbedarf ist. Ohne `tika` sind PDFs nur über Namen und
Nachricht auffindbar. Grund für Tika statt einer reinen Go-Bibliothek für PDF: Text aus PDFs
robust zu lesen ist aufwendig, und ein eigener Prozess trennt Fehler im Parser vom Server wie
beim Malware-Scan (ADR 0033).

Tika-Aufruf: `PUT /tika` mit den Rohbytes und `Accept: text/plain` [Q5, belegt]; eingebettete
Dateien überspringen mit `X-Tika-Skip-Embedded: true` [Q5, belegt]. Die Server-Doku beschreibt
für 2.x: Parsen in einem Kindprozess, der nach Speichermangel oder Zeitüberschreitung neu
startet; in dieser Zeit antwortet der Server `503` [Q5, belegt]. Der Job wiederholt dann. 2.x
ist seit 2025 am Ende der Pflege, aktuell sind 3.x (gepflegt) und 4.1.0 [Q8, belegt]; ob das
Verhalten in der gepinnten Version gleich ist, prüft der Integrationstest des Adapters. Der
Container hat keinen Netzzugang nach außen und ist nur im internen Netz erreichbar.

### 5.3 Grenzen und Schutz

- Nur Dateien bis `search.extract.max_input_bytes`.
- Archive (Office, ODF): höchstens `search.extract.max_uncompressed_bytes` entpackt und
  `search.extract.max_archive_entries` Einträge; sonst Abbruch mit `failed` (Schutz vor
  Zip-Bomben).
- Zeitlimit je Datei `search.extract.timeout`.
- XML mit `encoding/xml`; das Paket lädt keine externen Entitäten nach [abgeleitet aus dem Fehlen
  einer solchen Funktion; durch einen Test mit XXE-Datei zu belegen].
- Der Text ist Inhalt der Klasse `content`; er wird mit der Datei gelöscht (ON DELETE CASCADE).

## 6. Offline-Suche im Client

- Der Client durchsucht seinen lokalen Cache ([04-sync-protokoll.md](04-sync-protokoll.md) 13.1)
  mit SQLite FTS5. drift bringt eine SQLite-Version mit FTS5 mit; FTS5-Tabellen müssen in
  `.drift`-Dateien stehen, Abfragen können `bm25`, `highlight` und `snippet` nutzen [Q6, belegt].
- Index: Nachrichten, Thread-Titel und Transkripte, die im Cache liegen. Kein Dateitext.
- Rechte: Der Cache enthält nur, was der Client sehen durfte. Bei `access.changed` mit `removed`
  löscht der Client auch den lokalen Suchindex dieser Ziele ([04-sync-protokoll.md](04-sync-protokoll.md)
  8.3). Bei Sitzungs- oder Geräte-Widerruf alles.
- Anzeige: Offline zeigt die App lokale Treffer mit dem Hinweis "nur auf diesem Gerät
  gespeicherte Nachrichten". Online gelten die Server-Ergebnisse; lokale Treffer ergänzen nur,
  solange die Server-Antwort aussteht.
- Grenzen: Ältere, nie geladene Threads fehlen; Wortstämme anders als im Server (Tokenizer-Wahl
  in [15-client-architektur.md](15-client-architektur.md)). Der lokale Index liegt in der
  verschlüsselten Datenbank des Clients.

## 7. Leistungsziele und Lasttest

| Ziel | Wert |
|---|---|
| p95 der Server-Dauer von `GET /api/v1/search` (Typ `messages`, ohne Netz) | `search.p95_target` |
| bei Korpusgröße | `search.benchmark_messages` |
| Präfix-Suche p95 | `search.prefix_p95_target` |

Lasttest: synthetischer Korpus mit deutschen und englischen Texten, Wortfrequenz nach einer
Zipf-Verteilung, mehrere Bereiche und Kanäle, ein Prinzipal sieht einen Teil der Kanäle.
Anfragen mit ein bis drei Wörtern, Phrasen und Verneinung. Messung auf dem Referenzprofil aus
[16-betrieb.md](16-betrieb.md) (Kapazität). Der Test läuft im Release-Workflow und von Hand,
nicht bei jedem PR (Laufzeit). Verfehlt er das Ziel, ist das ein Release-Blocker.

Jede Suchabfrage läuft mit `SET LOCAL statement_timeout` = `search.statement_timeout`. Bei
Zeitüberschreitung: `503 search_timeout` mit Hinweis, die Suche einzugrenzen.

## 8. Indexpflege

- Generierte Spalten halten Vektoren ohne Trigger und Job aktuell
  ([03-datenmodell.md](03-datenmodell.md) Abschnitt 5).
- GIN-Indexe mit Standard-Einstellungen (schnelles Einfügen über eine Warteliste, die Autovacuum
  einarbeitet). Indexgröße und Aufblähung beobachtet ein Dashboard
  ([17-observability.md](17-observability.md)); bei Bedarf `REINDEX CONCURRENTLY`.
- **Ausdruck ändern** (neue Gewichte, neue Sprachen im Ausdruck) schreibt die Tabelle neu. Auf
  PostgreSQL 16 ändert `ADD COLUMN ... GENERATED ... STORED` den Dateiknoten der Tabelle
  (geprüft); für 18 gilt dasselbe [abgeleitet]. Das braucht ein Wartungsfenster; der
  Upgrade-Leitfaden ([16-betrieb.md](16-betrieb.md)) nennt die gemessene Dauer aus dem Lasttest.
- **Sprache neu zuordnen** (z. B. nach Änderung von `search.locale_map`): ein Job setzt
  `search_language` in kleinen Stapeln neu. Das Aktualisieren der Basisspalte berechnet den
  Vektor neu, Zeile für Zeile, ohne Tabellensperre.

## 9. Grenze und Ausblick

Eine externe Suchmaschine kommt erst, wenn einer dieser Punkte eintritt:

1. Der Lasttest verfehlt `search.p95_target` auf realer Datenmenge, nachdem Abfrage und Indexe
   optimiert sind.
2. Fehlertoleranz (Tippfehler) oder Komposita-Zerlegung im Deutschen wird verlangt (3.3).
3. Typ-übergreifendes Ranking (Nachrichten, Threads, Dateien in einer Liste) wird verlangt.

Zwischenschritt vor einer externen Maschine: `pg_trgm` für Titel und Dateinamen (Teilwort- und
Ähnlichkeitssuche), eine Erweiterung, die PostgreSQL mitliefert.

Der Port `SearchIndex` bleibt dabei gleich (Abschnitt 10). Ein externer Adapter liest den
Ereignis-Log wie ein Verbraucher, speichert je Dokument die `channel_id` und bekommt bei jeder
Anfrage die sichtbare Kanalmenge aus `authz` als Filter. Rechte-Logik im Suchindex selbst gibt es
nie (P2, B7). Löschungen kommen über `message.deleted` und das Schwärzen
([03-datenmodell.md](03-datenmodell.md) 6.5).

## 10. Modul und Port

Die Suche liest Tabellen dreier Module (`messaging`, `chat`, `files`). Nach
[03-datenmodell.md](03-datenmodell.md) 1.12 liest ein Modul fremde Tabellen nur über deren Port.
Darum:

- `messaging`, `chat` und `files` bieten je eine Lese-Operation an: `SearchMessages`,
  `SearchThreadTitles`, `SearchFiles`. Sie bekommen Anfrage, Filter, sichtbare Kanäle und
  Grenzen und liefern Treffer mit Wert und Segmenten.
- Ein Koordinator `search` ohne eigene Tabellen besitzt den Port `SearchIndex`, holt die
  sichtbaren Kanäle von `authz` und ruft die passende Operation. Ein späterer externer Adapter
  ersetzt nur diesen Koordinator-Adapter.
- Gemeinsame Teile (Anfrage-Aufbau, Segmentierung, Cursor) liegen in einer internen Bibliothek
  ohne Zustand.
- `file_text_extracts` gehört `files`, wie das Transkript.

Die Modulliste in [02-architektur.md](02-architektur.md) muss `search` als Koordinator führen;
das ist im Rückgabebericht an Team A gemeldet.

## 11. Endpunkte

Verbindlich ist `docs/api/openapi.yaml` (`search`, `searchThreadTitles`); diese Tabelle gibt den
Stand der Spec wieder.

| Methode und Pfad | Aktion | RL-Klasse | Zweck |
|---|---|---|---|
| `GET /api/v1/search` | `search.query` | `rl.search` | Parameter `q`, `type` (`messages`, `threads`, `files`; Standard `messages`), Filter aus 4.4, `sort`, `cursor`, `limit` |
| `GET /api/v1/search/thread-titles` | `search.query` | `rl.search` | Präfix-Suche für den Schnellwechsel (4.9) |

Antwort je Treffer: Objekt-Zusammenfassung (Nachricht, Thread oder Datei), Thread-Titel, Kanal-
und Bereichsname, `segments`, bei `relevance` der Wert. Dazu `next_cursor` und
`candidates_capped`.

## 12. Tests

| Test | Hält |
|---|---|
| "Rechnungen" findet "Rechnung" (Deutsch), "invoice run" findet "invoices are running" (Englisch), `err_conn_reset` findet die Kennung | Abschnitt 3 |
| "Rechnung" findet "Rechnungsadresse" nicht | dokumentierte Grenze 3.3 |
| Zufällige Eingaben (Fuzzing) | nie ein Fehler außer `400 validation_failed` auf `q` (`empty` bzw. `too_long`) |
| Zwei Prinzipale, gleiche Anfrage, verschiedene Freigaben | Treffer nur aus eigenen sichtbaren Kanälen; keine Zähler |
| Owner sucht | keine Treffer aus fremden DMs |
| Entzug, dann Suche | keine Treffer aus dem entzogenen Kanal; Seitenprüfung fängt einen Wettlauf ab (Test mit Entzug zwischen Kanalmenge und Abfrage) |
| Filter auf unsichtbaren Kanal | leeres Ergebnis, gleiche Antwort wie für einen unbekannten Kanal |
| Gelöschte Nachricht, Datei `pending` oder `infected`, nie angehängte Datei | kein Treffer |
| Inhalt mit `<b>` und Text, der wie eine Markierung aussieht | Segmente korrekt, kein HTML |
| Cursor mit geänderter Anfrage | `400 cursor_invalid` |
| Migrations-Lint | `GENERATED ALWAYS AS` ohne `STORED` schlägt fehl |
| Start mit Sprache in `search.locale_map`, die in `search.languages` fehlt | Startfehler |
| Office-Datei mit Zip-Bombe, XML mit externer Entität | `failed`, kein Nachladen |
| Lasttest | `search.p95_target` bei `search.benchmark_messages` |
| Client: `access.changed` mit `removed` | lokaler FTS-Index enthält keine Zeile dieser Ziele mehr |

## 13. Entscheidungen dieses Kapitels

| Nr | Entscheidung | Grund |
|---|---|---|
| R-SU-01 | Ein `tsvector` je Objekt mit Sprach-Stämmen und `simple`-Wörtern, unterschiedlich gewichtet. | Ein Index, ein Prädikat; Namen, Kennungen und fremde Sprachen bleiben auffindbar. |
| R-SU-02 | Sprache je Nachricht aus der Sprache des Autors, gespeichert; keine automatische Spracherkennung. | Kurze Chat-Nachrichten erkennt man schlecht; die `simple`-Form fängt Fehlzuordnungen ab. |
| R-SU-03 | Anfrage als ODER über alle Instanz-Sprachen plus `simple`. | Eine Anfrage trifft Texte jeder konfigurierten Sprache. |
| R-SU-04 | Kandidaten-Grenze vor dem Ranking, neueste zuerst. | Kosten unabhängig von der Treffermenge; im Chat zählt Aktuelles. |
| R-SU-05 | Ein Ergebnis-Typ je Anfrage, keine Gesamtzahl. | Kein typ-übergreifendes Ranking nötig; Zähler sind teuer und ein Leck-Risiko. |
| R-SU-06 | Hervorhebung mit zufälligen Markierungen und Segmenten statt HTML. | Keine HTML-Injektion, eindeutige Zerlegung. |
| R-SU-07 | Dateitext: reines Go für Text- und Office-Formate, Tika optional als eigener Dienst. | Kein JVM-Zwang; Parser-Risiko außerhalb des Servers. |
| R-SU-08 | Lauf-Schritte sind nicht durchsuchbar. | Werkzeugausgaben enthalten Geheimnisse und Lärm. |
| R-SU-09 | Suche als Koordinator mit Lese-Ports der besitzenden Module. | Hält die Modulregel aus 03; ein externer Adapter ersetzt nur den Koordinator. |

## 14. Abgleich mit dem Kern-Vertrag

Die beim Entwurf beantragten Ergänzungen sind entschieden (Integration vom 2026-10-10):

| Nr | Ergänzung | Entscheidung, Ort |
|---|---|---|
| 1 | neue Ausdrücke für `messages.search_vector` und `threads.title_search` | übernommen, 03 Abschnitt 5 |
| 2 | Spalte `threads.search_language` | übernommen, 03 `threads` |
| 3 | Tabelle `file_text_extracts` | übernommen, 03 Abschnitt 4.7 |
| 4 | Scope `messages:read` für `search.query` | übernommen, 05 Abschnitt 2.6 |
| 5 | Modul `search` als Koordinator ohne Tabellen | übernommen, 03 Abschnitt 2 |

## 15. Konfigurationswerte dieses Kapitels

| Name | Standard | Bedeutung |
|---|---|---|
| `search.languages` | `["german","english"]` | Konfigurationen für den Anfrage-Aufbau (plus immer `simple`) |
| `search.locale_map` | `{"de":"german","en":"english"}` | Sprachkennung zu Textsuche-Konfiguration |
| `search.query_max_chars` | `256` | Länge der Eingabe |
| `search.max_candidates` | `2000` | Kandidaten vor dem Ranking |
| `search.rank_normalization` | `1` | Normalisierung für `ts_rank` |
| `search.recency_scale` | `30d` | Alter, bei dem der Wert auf die Hälfte fällt |
| `search.page_size_default` | `20` | Treffer je Seite |
| `search.page_size_max` | `50` | höchstens Treffer je Seite |
| `search.headline_input_max_chars` | `20000` | Länge des Textes für `ts_headline` |
| `search.headline_max_fragments` | `2` | Ausschnitte je Treffer |
| `search.headline_max_words` | `30` | Wörter je Ausschnitt, höchstens |
| `search.headline_min_words` | `10` | Wörter je Ausschnitt, mindestens |
| `search.prefix_min_chars` | `2` | kürzeste Wortlänge in der Präfix-Suche |
| `search.statement_timeout` | `3s` | Zeitlimit einer Suchabfrage |
| `search.p95_target` | `500ms` | Ziel p95 im Lasttest |
| `search.prefix_p95_target` | `100ms` | Ziel p95 der Präfix-Suche |
| `search.benchmark_messages` | `2000000` | Korpusgröße im Lasttest |
| `search.extract.max_input_bytes` | `52428800` | größte Datei für die Extraktion |
| `search.extract.max_text_bytes` | `1048576` | größter gespeicherter Text |
| `search.extract.max_uncompressed_bytes` | `209715200` | größte entpackte Menge bei Archiven |
| `search.extract.max_archive_entries` | `10000` | Einträge je Archiv |
| `search.extract.timeout` | `60s` | Zeitlimit je Extraktion |
| `search.extract.tika_url` | leer | Tika-Server; leer = aus |

## 16. Quellen

Alle abgerufen am 2026-10-10.

- Q1 PostgreSQL 18, Versionen und Unterstützung: https://www.postgresql.org/support/versioning/ [belegt]
- Q2 PostgreSQL 18, Controlling Text Search (`websearch_to_tsquery`, `ts_rank`, `ts_headline`, Präfix): https://www.postgresql.org/docs/18/textsearch-controls.html [belegt]
- Q3 PostgreSQL 18, Generated Columns: https://www.postgresql.org/docs/18/ddl-generated-columns.html [belegt]
- Q4 PostgreSQL, Dictionaries (Snowball, Ispell/Hunspell): https://www.postgresql.org/docs/current/textsearch-dictionaries.html [belegt]
- Q5 Apache Tika Server: https://cwiki.apache.org/confluence/display/TIKA/TikaServer [belegt]
- Q6 drift, Supported sqlite extensions: https://drift.simonbinder.eu/sql_api/extensions/ [belegt]
- Q7 `golang.org/x/net/html`: https://pkg.go.dev/golang.org/x/net/html [belegt]
- Q8 Apache Tika, Startseite und Release-Hinweise: https://tika.apache.org/ [belegt für Versionen; Lizenz nur über den Verweis auf die ASF-Lizenzseite, darum abgeleitet]
