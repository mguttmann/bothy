# tools/catalog

Der Aktionskatalog [`docs/spec/actions.yaml`](../../docs/spec/actions.yaml) ist die einzige
Quelle für Aktionen, Rate-Limit-Klassen, Ereignistypen mit Empfangs-Aktion und WebSocket-Ops
([ADR 0049](../../docs/adr/0049-maschinenlesbarer-aktionskatalog-als-einzige-quelle.md)).
`action_catalog.py` lädt und prüft ihn und erzeugt daraus die Tabellen der Abschnitte 3, 4, 6
und 7 von [Kapitel 05](../../docs/spec/05-aktionen.md) und die Listen `authz_actions`,
`rate_limit_classes` und `receive_actions` in
[`docs/api/redocly.yaml`](../../docs/api/redocly.yaml).

## Befehle

| Befehl | Wirkung |
|---|---|
| `make catalog` | erzeugte Blöcke neu schreiben (`action_catalog.py generate`); schreibt nichts, solange `make catalog-check` einen anderen Befund meldet als einen veralteten Blockinhalt, und sonst Kapitel 05 und `redocly.yaml` zusammen oder keine von beiden; scheitert das Schreiben selbst, endet die Ausgabe mit `cannot write the generated blocks` statt mit `catalogue or blocks invalid` |
| `make catalog-check` | Gate: alle Regeln unten (`action_catalog.py check`); das Skript endet bei einem Befund mit Exit 1 und schreibt eine Zeile je Befund nach stderr, `make` endet dann mit Exit 2 |
| `uv run --project tools --locked pytest tools/catalog/tests` | Tests, auch der Ende-zu-Ende-Test mit der gepinnten Redocly CLI (`make bootstrap`) |

Ablauf einer Änderung: `actions.yaml` ändern, `make catalog`, den Diff von 05 und
`redocly.yaml` mit committen. Eine Handänderung in einem erzeugten Block scheitert im Gate.

Ein neuer Abschnitt der Aktionen braucht:

1. in `actions.yaml` seinen Schlüssel in `sections` (an der Stelle der Tabelle in 05) und
   seine Aktionen;
2. in 05 seine Überschrift der Ebene 3 im Abschnitt "Aktionen" mit dem leeren Block
   `actions.<schlüssel>` darunter (nur die beiden Marker; `make catalog` füllt ihn);
3. denselben Titel in `ACTION_SECTION_TITLES` in `action_catalog.py`;
4. den Schlüssel in `SECTIONS` in `tests/test_action_catalog.py`; dort halten
   `test_repository_catalog_loads` und der Zell-für-Zell-Vergleich die Abschnitte fest.

Ein neuer von Hand gepflegter Abschnitt der Ebene 1 oder 2 mit einer Tabelle braucht seinen
Titel in `HAND_WRITTEN_SECTIONS` in `action_catalog.py`; sonst ist jede Tabellenzeile
außerhalb der erzeugten Blöcke ein Befund.

## Schema

Oberste Ebene, genau diese Schlüssel in dieser Reihenfolge: `version` (1),
`rate_limit_classes`, `sections`, `actions`, `events`, `ws_ops`, `channel_ops`. Unbekannte
Schlüssel sind überall ein Befund. Die Felder je Eintrag und ihre Bedeutung stehen in den
Docstrings der Datenklassen `RateLimitClass`, `Action`, `Event`, `WsOp` und `ChannelOp` in
`action_catalog.py`; kurz:

| Liste | Pflichtfelder | Weitere Felder |
|---|---|---|
| `rate_limit_classes` | `id` (`rl.<name>`), `key`, `purpose` | |
| `actions` | `id`, `section`, `description`, `resource`, `right`, `owner` (`true`, `false`, `as_target`), `step_up` (`true`, `false`, `conditional`, `fresh`), `audit` (`always`, `denials`, `never`, `conditional`), `rights_change`, `rl_class` (Klasse oder `none`), `effect` (`read`, `write`) | `audit_text` (genau bei `conditional`), `delivery_check`, `app_scope` |
| `events` | `type`, `kind` (`durable`, `ephemeral`), `resource`, `resource_version` | `receive_action` (Aktion oder `self`), `audience` (`self`), `receivers` (`principal` als Standard, `agent`, `agent_or_app`), `receive_text`, `note` |
| `ws_ops` | `op`, `direction` | `action`, `also_actions`, `action_text`, `row_group`, `per_event_receive_action` |
| `channel_ops` | `channel`, `ops`, `direction` | `channel_text` (genau in der ersten Zeile eines Kanals), `action`, `action_text` |

Texte, die in eine Tabellenzelle von 05 kommen, stehen wörtlich in der Datei (Markdown, eine
Zeile, keine Leerzeichen am Rand), und jede bleibt auf der Site genau eine Zelle. Die Prüfung
liest sie so, wie die Erweiterung `tables` von Python-Markdown eine Tabellenzeile teilt: `\\`
ist ein maskierter Backslash, `\|` ein maskiertes `|`, ein Backslash vor Backticks maskiert
einen davon; jeder Backtick-Lauf, den die Zelle öffnet, schließt in ihr ein späterer Lauf
gleicher Länge, und jedes `|` steht zwischen einem solchen Paar oder hinter einer ungeraden Zahl
von Backslashes. Außerhalb ihrer Code-Spans (gelesen wie die Zeilen von 05 außerhalb der
Blöcke) hält eine Zelle kein HTML (kein `<` vor Buchstabe, `!`, `?` oder `/`) und kein
Klammerpaar `{...}`: rohes HTML kann die Tabelle verschlucken oder die Zelle früher enden
lassen, und eine Attributliste nimmt Text aus der Zelle oder versteckt sie. Sonst nennt eine
Befundzeile Eintrag, Feld und Spalte. Ein Test vergleicht die
Regel mit dem Renderer, ein anderer rendert jede erzeugte Tabelle und vergleicht sie Zelle für
Zelle mit dem Katalog. `*_text`-Felder ersetzen die Zelle, wenn sie mehr sagt als die Aktion;
der maschinelle Wert steht daneben.

## Regeln

Jeder Befund hat die Form `<Datei>: <Regel>: <Detail>`.

| Regel | Prüft |
|---|---|
| `schema` | Pflichtfelder, Typen und erlaubte Werte, keine unbekannten oder doppelten Schlüssel, `version` gleich 1; Zellen nach dem Absatz oben; `right` nennt `system` nur als genau `system` |
| `unique` | Klassen-IDs, Aktions-IDs, Ereignistypen und Op-Namen sind eindeutig |
| `section` | jede Aktion in einem gelisteten Abschnitt, jeder Abschnitt mit Aktion, Aktionen in der Reihenfolge der Abschnitte |
| `rate-limit-class` | Klasse bekannt; `none` genau bei Systemaktionen und internen Zustellprüfungen; jede Klasse benutzt |
| `rate-limit-config` | je Klasse genau eine Zeile `ratelimit.<klasse>.per_minute` / `.burst` in den Konfigurationswerten von 05, keine fremde Klasse; der Abschnitt hält genau eine Tabelle mit der Kopfzeile `Name \| Standard \| Bedeutung` und einer Trennzeile mit ebenso vielen Spalten |
| `event` | Empfangs-Aktion vorhanden, bekannt, keine Systemaktion; `self` nur mit `audience: self`; `receivers` nur mit `audience: self`; `receive_text` nennt die Aktion; Zelle beginnt genau bei `audience: self` mit `self`; dauerhafte vor kurzlebigen Typen |
| `op` | Aktion vorhanden, bekannt, keine Systemaktion (auch `also_actions`); Gruppen zusammenhängend und gleich in Richtung, Aktion, `action_text`, `also_actions` und `per_event_receive_action`; `action_text` nennt die Aktionen; `also_actions` und `per_event_receive_action` nur mit `action_text`; `per_event_receive_action` nur bei `dispatch` |
| `channel-op` | wie `op`; Zeilen eines Kanals zusammenhängend; Op-Name je Kanal und Richtung eindeutig |
| `app-scope` | `app_scope` gleich der Tabelle "Scopes für Apps" (05, 2.6); die Tabelle nennt nur bekannte Aktionen, jede höchstens einmal; der Abschnitt hält genau eine Tabelle mit der Kopfzeile `Aktionen \| Scope` und einer Trennzeile mit ebenso vielen Spalten |
| `effect` | `effect` gleich der geschlossenen Liste "Wirkung" (05, 2.7); jeder Listenpunkt des Abschnitts beginnt mit `- ` in der ersten Spalte, auch kein eingerückter oder verschachtelter Punkt, und hält keine Überschrift (Punkte nach einer Leerzeile gehören wie auf der Site zur selben Liste). Für diese drei Regeln trägt keine andere Überschrift einen Titel mit denselben Buchstaben wie der gelesene Abschnitt (Ziffern zählen nicht, weil eine Nummer verschieden geschrieben werden kann, und zugleich gelten `1` als `l` und `0` als `o`; `I` und `l` gelten als gleich), und der Abschnitt endet wie bei Markdown an jeder Zeile, die mit höchstens so vielen `#` beginnt |
| `generated-block` | Marker vollständig; Blocknamen in 05 und `redocly.yaml` genau die erwarteten, in 05 in der Reihenfolge des Katalogs (in `redocly.yaml` ist die Reihenfolge der Regeln frei, sie ändert den Lint nicht); in 05 jede Tabelle in ihrem Abschnitt der Ebene 2 (`BLOCK_SECTIONS`; eine Blockart ohne Eintrag ist ein Befund), jede Aktionstabelle unter der Überschrift ihres Katalog-Abschnitts (`ACTION_SECTION_TITLES`), und zwischen jeder erzeugten Tabelle und der letzten Überschrift der Ebene 2 oder 3 über ihr steht keine Überschrift der Ebene 4 oder tiefer; jeder Block gleich der Erzeugung. Außerhalb der Blöcke steht keine Überschrift ohne sichtbaren Titel (mindestens ein Zeichen aus `[0-9A-Za-zÄÖÜäöüß]`), und jeder Titel besteht nur aus diesen Zeichen, Leerzeichen, Backticks und `"(),-.:/_`; so kann keine andere Schrift, keine Entity und kein unsichtbares Zeichen eine Überschrift wie eine andere aussehen lassen. Überschriften liest die Prüfung nach einer Näherung der Blockregeln von Python-Markdown: Tabs auf vier Spalten, eine Leerzeile nur aus Leerzeichen; ATX nur mit `#` in der ersten Spalte, auch mit leerem Titel; Setext nur mit Unterstreichung in der ersten Spalte unter einer Zeile, die einen Block beginnt (am Anfang, nach einer Leerzeile, einer Überschrift, einer Trennlinie, einem Marker, einer Link-Referenz oder eingerücktem Code). 05 enthält kein Zeichen, das die Prüfung und Markdown verschieden lesen: keinen Zeilentrenner nur für `str.splitlines` (U+000B, U+000C, U+001C bis U+001E, U+0085, U+2028, U+2029) und kein STX oder ETX (U+0002, U+0003), die Markdown entfernt. Kapitel 05 beginnt mit seiner `# `-Überschrift (keine Seiten-Metadaten, kein Front-Matter). In 05 steht außerhalb der Blöcke kein HTML (kein `<` vor Buchstabe, `!`, `?` oder `/` außerhalb eines Code-Spans; Code-Spans und Escapes findet die Prüfung je Zeile wie die Doku-Site, also wie `find_code_spans` aus pymdownx.inlinehilite in der Fassung aus `tools/uv.lock`), keine Überschrift ohne Leerzeichen nach `#`, kein Klammerpaar `{...}` außerhalb eines Code-Spans (also keine Attributliste), keine Admonition (`!!!`), kein einklappbarer Block (`???`) und keine Fußnote am Zeilenanfang (eingerückte Formen fängt der Render-Test), und in keinem Abschnitt eine Zeile mit `\|` oder `<table`, auch nicht in einem Codeblock, außer in den von Hand gepflegten Abschnitten "Begriffe der Tabellen", "Agenten-Aufruf `agent.invoke`", "Entscheidungen dieses Kapitels" und "Konfigurationswerte dieses Kapitels"; jeder dieser Titel steht genau einmal da. `redocly.yaml` bleibt in der Positivliste: oben nur `extends` (genau `[recommended]`), `apis` und `rules`, je API nur `root` (relativ, nach Auflösung gegen `docs/api` eindeutig) und `rules`, kein Schlüssel mit `$` (Redocly löst `$ref` auf), keine doppelten Schlüssel. Die Lint-Regeln des Katalogs behalten ihre erzeugte Form (nur `subject` mit Typ und Eigenschaft, `assertions` mit genau `enum` gleich der erzeugten Liste, `message`), und keine Regelmenge unter `apis` nennt sie. `docs/api/.redocly.lint-ignore.yaml` nennt keine Katalog-Regel |

`load_catalog` prüft die Regeln, die nur die YAML-Datei brauchen, `check` ergänzt den Abgleich
mit Kapitel 05 und `redocly.yaml`. Die Befunde kommen in Stufen, und eine Stufe läuft nur, wenn
die Stufen davor nichts fanden: Datei lesen und parsen (ein Befund in einer Zeile, auch für einen
Wert, den Python nicht bauen kann, etwa ein ungültiges Datum, oder für zu tiefe
Verschachtelung), dann doppelte Schlüssel und rekursive Aliase, dann das Schema, dann die
Beziehungsregeln; erst ein gültiger Katalog wird mit 05 und `redocly.yaml` verglichen, und deren
Regeln melden dann alle zusammen. Nach einer Korrektur können also Befunde der nächsten Stufe
erscheinen. Aliase liest die Prüfung je Knoten einmal, auch tief verschachtelte.

## Schnittstelle für spätere Tickets

M1-03, M1-06 und M1-25 importieren das Modul. Unter pytest liegt `tools/catalog` im
`pythonpath` von `tools/pyproject.toml`; außerhalb von pytest nimmt der Aufrufer
`tools/catalog` selbst in `sys.path` oder `PYTHONPATH` auf, denn das Modul ist kein installiertes
Paket. Öffentlich sind:
`load_catalog`, `catalog_from_data`, `Catalog` mit `actions_by_id`, `events_by_type`,
`ws_ops_by_name`, `classes_by_id`, `CatalogError`, `CatalogWriteError` (Unterklasse für
einen Schreibfehler von `generate`), `generated_blocks`,
`replace_generated_block`, `yaml_generated_blocks`, `replace_yaml_block`,
`render_chapter_blocks`, `chapter_table_cells`, `redocly_lists`, `receive_cell`, `scope_assignments`,
`read_effect_ids`, `relational_findings`, `chapter_findings`, `check`, `generate`.

## Was der Check nicht abdeckt

- Die statischen Prüfungen der Regel `generated-block` geben schnelle Rückmeldung; gehalten wird
  die Klasse durch Tests gegen die echten Artefakte in `tests/test_action_catalog.py`:
  - Ein Ende-zu-Ende-Test setzt `x-authz-action`, `x-rate-limit-class` oder `x-receive-action`
    an jeder Stelle der Verträge auf einen ausgeschlossenen Wert, führt auf einer Kopie von
    `docs/api` und `Makefile` das echte `make api-lint` aus und verlangt genau so viele Fehler,
    wie er Stellen geändert hat.
  - Ein Test heftet das Rezept von `make api-lint` wörtlich an, und ein direkter Redocly-Lauf
    ohne make verlangt, dass die unveränderten Verträge die echte Konfiguration bestehen.
  - Die Rezepte von `make catalog` und `make catalog-check` sind wörtlich angeheftet, und ein
    Lauf des echten `make catalog-check` auf einer Kopie muss bei einem veralteten Block
    scheitern.
  - Ein Test rendert Kapitel 05 mit der Markdown-Konfiguration aus `mkdocs.yml` und verlangt:
    keine Seiten-Metadaten und kein Front-Matter (gefragt beim Leser von mkdocs);
    dieselbe Folge von Überschriften wie die statische Prüfung; nur Elemente und Attribute einer
    Positivliste (was Markdown für Text, Listen, Links und Tabellen erzeugt, an Links nur die
    Klasse `headerlink`; kein `div`, `span`, `details`, `style`, kein Attribut `hidden`, `style`
    oder `data-*`); kein rohes HTML außer den Markern, auch nicht über `md_in_html`, gefragt beim
    Renderer selbst; außerhalb der von Hand gepflegten Abschnitte genau die erzeugten Tabellen,
    jede im Abschnitt und unter der Überschrift ihres Blocks ohne Überschrift der Ebene 4 oder
    tiefer dazwischen, auch wenn eine Überschrift in
    einem Zitat, einer Liste, einer Definitionsliste oder einem Codeblock steht; eine
    Überschrift ohne sichtbaren Titel, mit einem Zeichen außerhalb der Positivliste oder in
    einem Listenpunkt oder Zitat ist ein Befund. Die Tabelle 2.6 samt Kopfzeile, die Liste 2.7
    und die Konfigurationstabelle zeigt die Site genau so, wie die Prüfung sie liest, und keine
    andere Überschrift, gleich welcher Ebene, zeigt dieselben Buchstaben wie ihre Titel.
    Doppelgänger aus mehreren lateinischen Buchstaben (etwa `rn` für `m`) und Ziffern außer
    `1` und `0` sieht keine Prüfung.
    Ein Codeblock oder eine Admonition in 05 müsste die Positivliste im Review erweitern.
  - Statisch zählt als Tabelle nur eine Folge von Zeilen, die mit `|` beginnen; Tabellen in
    anderer Form (ohne führendes `|`, in einem Zitat) sieht nur der Render-Test. Einen gelesenen
    Abschnitt beendet statisch nur eine Zeile mit `#`, keine Setext-Überschrift. Die statische
    Überschriftensicht ist eine Näherung, die jede Blockregel nur an der Form einer einzelnen
    Zeile erkennt; unter anderem liest sie keine Behälter, nicht das Zusammenwirken mehrerer
    Blockregeln in einem Block (etwa eine Trennlinie und eine Link-Referenz, oder eine Zeile
    nur aus geschütztem Leerzeichen nach eingerücktem Code), keine Blockregel über Zeilen
    hinweg (eine Link-Referenz mit Rest oder mit Ziel auf der Folgezeile), keine ATX-Sonderfälle
    (ein Titel, der auf `\` endet) und kein Markup im Titel (Entities und Escapes sind
    statisch Befunde, weil `&`, `;` und `\` nicht in der Positivliste stehen). Ein
    Test vergleicht sie mit der gerenderten Folge für die Fälle aus den Reviews; die Klasse hält
    der Render-Test.
  - Ein Test vergleicht die statische Sicht auf Code-Spans und Escapes Zeile für Zeile mit dem
    Renderer (alle Zeilen aus bis zu fünf Bausteinen mit `<i>`), ein Test die Escape-Zeichen
    mit denen des Renderers. Spans über mehrere Zeilen und Code-Spans in Tabellenzeilen
    außerhalb der Blöcke (dort paart `tables` zuerst die Backtick-Läufe der ganzen Zeile, teilt
    an den übrigen `|` und liest dann jede Zelle für sich) sieht nur der Render-Test.
  - Ein Test rendert jede erzeugte Tabelle und vergleicht sie Zelle für Zelle mit den Zellen
    des Katalogs (`chapter_table_cells`), und ein Test vergleicht die Zellregel der Regel
    `schema` für alle Zellen aus bis zu fünf Bausteinen mit dem Renderer.
- Nicht abgedeckt: Makefile-Inhalte außerhalb der angehefteten Rezepte, die nur zusammen mit
  anderen Dateien wirken (eingebundene Makefiles, Bedingungen auf andere Dateien, den
  Git-Zustand oder die Umgebung), Abschwächungen, die nur in CI wirken, beim Rendern alles in
  `mkdocs.yml` außer `markdown_extensions` (Hooks, Plugins, `nav`, `theme.custom_dir`,
  `extra_css`, `extra_javascript`), andere Darstellungen als die Doku-Site (zum Beispiel
  GitHub) und absichtliche Änderungen an `tools/catalog` oder seinen Tests. Das prüfen das
  Review und das Red-Team-Gate ([red-team-gate.md](../../docs/quality/red-team-gate.md), Punkt
  "Gates").
- Den Wert von `resource_version` prüft kein Gate: die Regel `schema` verlangt das Feld bei
  jedem Ereignistyp als Wahrheitswert, ob er stimmt (Spalte `version` der Ressource in 03, die
  Ausnahmen aus 04 "Zustellgarantie" Punkt 3, Ruling M1-02/6), bleibt Review. Jedes versendete
  Ereignis prüft später der Rahmen-Konformitätstest aus M1-67 dagegen.
- Den direkten Abgleich der Verträge mit dem Katalog bringt M1-03 (Gate G2). Er fängt auch eine
  Operation, deren `x-authz-action` ein Objekt ist; die Aufzählung von Redocly überspringt es.
- Dekoratoren wendet Redocly beim Lint nicht an (nachgestellt mit `remove-x-internal`); die
  Positivliste lässt sie trotzdem nicht zu.
- Eine Zelle `right`, die `system` mit ähnlich aussehenden Zeichen eines anderen Alphabets
  schreibt, erkennt die Regel `schema` nicht; eine echte Systemaktion fällt dann trotzdem bei
  `rate-limit-class` auf, solange ihre Klasse `none` bleibt und `delivery_check` fehlt. Der Rest
  bleibt Review.
- Abgleich von `openapi.yaml`, `asyncapi.yaml` und `webhooks.md` mit dem Katalog (M1-03).
- Zuordnung der Aktionen zu Modulen; sie steht im Modulregister (ADR 0050, M1-06).
- Go-Konstanten und der geschlossene Katalog im Server (M1-25).
- Ob die wörtlichen Texte (Beschreibung, Recht) inhaltlich stimmen; das bleibt Review.
- `tests/table_import.py` liest nur die eingefrorene Fassung von 05 vor dem Umbau
  (`tests/fixtures/05-aktionen-before.md.txt`) und dient nur dem Beweis, dass die Tabellen
  zeilengleich geblieben sind.
