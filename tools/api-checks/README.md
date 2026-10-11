# tools/api-checks

Vertragsprüfungen der API im Gate (Kapitel [06](../../docs/spec/06-api.md), Abschnitt 15, G2
und G3): Jede Kopie zwischen 04, 05, 06, [`docs/spec/actions.yaml`](../../docs/spec/actions.yaml),
[`openapi.yaml`](../../docs/api/openapi.yaml), [`asyncapi.yaml`](../../docs/api/asyncapi.yaml) und
[`webhooks.md`](../../docs/api/webhooks.md) wird gegen ihre Quelle geprüft. `api_checks.py`
braucht nur Python und PyYAML und liest den Aktionskatalog über
[`tools/catalog`](../catalog/README.md) (`load_catalog`, `generated_blocks`,
`replace_generated_block`).

## Befehle

| Befehl | Wirkung |
|---|---|
| `make api-checks` | Gate: beide Regeln, Exit 1 mit einer Zeile je Befund (`api_checks.py check`; eine Regel allein mit `--rule api-catalogue-sync` oder `--rule api-tables`) |
| `make api-tables` | erzeugte Blöcke in 06 und `webhooks.md` neu schreiben (`api_checks.py generate`); schreibt nichts, solange eine Eingabe unlesbar oder falsch geformt ist, ein Wert keine Tabellenzelle werden kann, ein Marker kaputt, unbekannt oder fehlend ist oder sich eine der zu ändernden Dateien nicht schreiben lässt |
| `uv run --project tools --locked pytest tools/api-checks/tests` | Tests: je Teilprüfung und je Block eine Mutation einer Kopie der echten Eingaben, die rot sein muss; das Repository selbst ist grün |

Ablauf einer Änderung: `openapi.yaml` oder `asyncapi.yaml` ändern, `make api-tables`, den Diff
von 06 und `webhooks.md` mit committen. Auch nach einer Änderung an
[`docs/spec/actions.yaml`](../../docs/spec/actions.yaml) und `make catalog` gehört
`make api-tables` dazu, weil die Spalte "Empfänger und Scope" in `webhooks.md` aus dem Katalog
kommt. Eine Handänderung in einem erzeugten Block scheitert im Gate.

Jeder Befund hat die Form `<Regel>: <Teilprüfung>: <Gegenstand>: <Detail>`. Kein Aufruf endet
in einem Traceback. Wirft eine Teilprüfung, die Erzeugung eines Blocks oder die Formprüfung von
`openapi.yaml` oder `asyncapi.yaml` einen unerwarteten Fehler, wird er zum Befund dieser
Teilprüfung, dieses Blocks oder der Teilprüfung `source` der Datei (bei einem Block `input not
renderable`, sonst `input not checkable`); die anderen melden weiter. Jeden anderen Fehler macht die Kommandozeile
zu einer Zeile `<Regel>: source: input not checkable (...)` (bei `generate`: `input not
renderable`) mit Exit 1. Die Teilprüfung `source` meldet eine Eingabe, die sich nicht lesen
oder laden lässt (ungültiges YAML, ein Wert, den PyYAML nicht bauen kann, etwa ein unquotiertes
Datum, das es nicht gibt, und doppelte Schlüssel, die PyYAML sonst still überschreibt) oder
deren Behälter nicht die Form haben, die die Prüfungen durchlaufen: ein Behälter mit falschem Typ oder mit leerem Wert (`parameters:` ohne
Liste, `responses:` ohne Abbildung, `tags:` ohne Liste) und ein Pfad-Eintrag, der nur über
`$ref` auf `components.pathItems` zeigt (seine Operationen würden sonst keiner Prüfung und keiner
Tabelle begegnen; sie stehen direkt unter `paths`). Bei `api-catalogue-sync` laufen dann die
übrigen Teilprüfungen nicht. Bei `api-tables` hält ein solcher Befund in `actions.yaml`,
`openapi.yaml` oder `asyncapi.yaml` alle Blöcke auf; ist 06 oder `webhooks.md` unlesbar, prüft
die Regel die andere der beiden Dateien weiter. Rekursive YAML-Aliase laufen nicht im Kreis.

## Regel `api-catalogue-sync` (G2)

| Teilprüfung | Hält |
|---|---|
| `rest-action` | jede Operation unter `paths`: `x-authz-action` ist genau eine bekannte Aktion, keine Systemaktion; `x-rate-limit-class` ist die Klasse dieser Aktion |
| `webhook-events` | jeder Eintrag unter `webhooks`: Schlüssel ist ein Ereignistyp des Katalogs, `x-receive-action` gleich dessen Empfangs-Aktion, `x-authz-action` gleich `webhook_out.deliver`, `x-rate-limit-class` gleich `none`, und das Schema des Körpers hat genau diesen Typ (`type` mit `enum` aus diesem einen Wert) |
| `ws-ops` | die Operationen am Kanal `gateway` sind genau die Ops des Katalogs: eine Nachricht heißt wie die Op, die Operation mit vielen Nachrichten ist `dispatch`; `receive` passt zu einer Richtung auf `_to_server`, `send` zu `server_to_client`; Aktion und Klasse wie im Katalog; jeder Rahmen trägt in `payload.properties.op.enum` genau seine Op (`dispatch` bei den Ereignissen), auch einer mit `contentType: application/octet-stream`, weil das Gateway nur JSON-Rahmen kennt; jede Nachricht unter `messages` einer Operation steht als `$ref` da, wie AsyncAPI 3 es verlangt, eine inline geschriebene (auch mit `$ref: null`) ist ein Befund; `reply.messages` prüft der Check darauf nicht |
| `channel-ops` | die Operationen an `terminalStream`, `hostControl` und `hostData` sind genau `channel_ops` des Katalogs (Kanal per Namen, `terminal_stream` heißt in asyncapi.yaml `terminalStream`); `receive` passt zu `device_to_server`, `host_to_server` oder `both`, `send` zu `server_to_device`, `server_to_host` oder `both`; je Kanal, Op und Richtung genau eine Operation; jeder Rahmen trägt in `payload.properties.op.enum` genau seine Op, ausgenommen nur ein Binärrahmen (`contentType: application/octet-stream`), der kein `op` nennt; jede Nachricht unter `messages` einer Operation steht als `$ref` da; an keinem anderen Kanal steht eine Operation, und jeder Kanalverweis löst auf |
| `events` | die Nachrichten mit `x-event-kind` (unter `components.messages` und direkt in einem Kanal geschriebene) sind genau die Ereignistypen des Katalogs (als Menge, jeder Typ einmal), je Typ mit Art, Empfangs-Aktion und `x-audience: self` genau bei Zielgruppe `self`; `dispatch` listet genau diese Nachrichten, jede einmal; das Schema hinter `payload.properties.d` hat genau den Typ der Nachricht; die Union `Event` in openapi.yaml nennt in `oneOf` und `discriminator.mapping` genau die dauerhaften Typen, jeden einmal und mit seinem Schema; eine Nachricht mit `x-receive-action` oder `x-audience` ohne `x-event-kind` ist ein Befund |
| `close-codes` | `channels.gateway.x-close-codes` nennt dieselben Paare aus Code und Name in derselben Reihenfolge wie die Tabelle in 04 "Schließ-Codes"; ein Terminal-Kanal nutzt einen Code unter 4100 (unterhalb des eigenen Bereichs der Terminal-Kanäle, der bei 4100 beginnt) nur mit dem Namen aus dieser Tabelle und jedes Paar nur einmal; `client_action` wird nicht verglichen (04 ist Deutsch, asyncapi.yaml Englisch) |
| `idempotency` | jede POST- und PATCH-Operation hat genau eins von beiden: einen Header-Parameter `Idempotency-Key` (auf Pfad- oder Operationsebene, direkt oder über `$ref`, Name ohne Rücksicht auf Groß- und Kleinschreibung) oder `x-idempotency-exempt` mit einem bekannten Grund; keine andere Methode, kein Pfad-Eintrag und kein Webhook trägt `x-idempotency-exempt`, auch nicht mit leerem Wert |
| `deprecation` | in beiden Dateien hat jedes `deprecated: true` die Felder `x-deprecated-since` (SemVer), `x-deprecated-at` und `x-sunset` (je `YYYY-MM-DD`), und `x-sunset` liegt mindestens `api.deprecation_min_notice` nach `x-deprecated-at`; keins der drei Felder steht ohne `deprecated: true`. Beispielwerte (`example`, `examples`) sind Daten und zählen nicht; unter `properties` sind die Schlüssel Namen |
| `app-scopes` | das Enum `AppScope` in openapi.yaml nennt genau die Scopes, die der Katalog Aktionen zuordnet (05 Abschnitt 2.6, ohne `none`), jeden einmal |
| `problem-codes` | `x-problem-codes` am Schema `Problem`: Codes eindeutig, `status` dreistellig, Texte nicht leer und ohne deutsche Umlaute oder ß, nach Status sortiert, nur bekannte Schlüssel; jede Antwort in `components.responses` hat `x-status`, jeder Verweis darauf steht unter diesem Status (ab 500 auch unter `default`); jeder Code in Backticks in der Beschreibung einer Antwort steht mit deren Status in `x-problem-codes`; jeder Code ohne `router: true` wird von mindestens einer Antwort genannt |

`api.deprecation_min_notice` steht in
[`docs/operations/configuration.md`](../../docs/operations/configuration.md), Abschnitt
"Benannte Werte aller Kapitel": in jeder Tabelle dort, deren Kopf die Spalten `Schlüssel` und
`Standard` hat, über den Spaltennamen gesucht. Fehlt die Zeile oder steht sie mehrfach da, fehlt
der Abschnitt oder steht er mehrfach da, ist das ein Befund, nie ein Standardwert. Eine Dauer
ist eine Zahl mit höchstens sechs Ziffern und der Einheit `ms`, `s`, `m`, `h` oder `d`, wie in
Abschnitt 2 der Konfigurationsreferenz.

## Regel `api-tables` (G3)

Das Werkzeug besitzt genau diese Blöcke. Ein anderer Blockname, ein fehlender Block und ein
kaputter Marker in den beiden Dateien sind Befunde der Teilprüfung `markers`; ein veralteter
Block ist ein Befund mit seinem Namen als Teilprüfung. Als Marker zählt, was
`action_catalog.generated_blocks` liest (`<!-- BEGIN GENERATED: <name> -->`). Zeilen haben die
Form `| a | b |`, eine leere Zelle wird zu zwei Leerzeichen.

| Datei | Block | Quelle |
|---|---|---|
| 06 | `problems` | `x-problem-codes` in Listenreihenfolge |
| 06 | `idempotency_exempt` | Operationen mit `x-idempotency-exempt`, nach Grund in der Reihenfolge der Tabelle `EXEMPTION_REASONS`, dann in Dateireihenfolge; Grund als deutscher Text |
| 06 | `endpoints` | je Tag in der Reihenfolge von `tags` mit Operationen: Überschrift `####` und Tabelle, Operationen in Dateireihenfolge (Pfade und Methoden wie geschrieben) |
| 06 | `ws_ops` | Operationen am Kanal `gateway` in Dateireihenfolge; die Operation mit vielen Nachrichten als feste Zeile `dispatch` |
| 06 | `events` | Nachrichten in der Reihenfolge von `dispatch` |
| `webhooks.md` | `webhook_events` | Einträge unter `webhooks` in Dateireihenfolge; Empfänger und Scope aus dem Katalog |

Ein Wert, der keine Tabellenzelle werden kann (fehlt, ist kein Text, enthält `|` oder einen
Zeilenumbruch, in einem Code-Span einen Backtick), eine Operation ohne Tag oder mit einem Tag,
das `tags` nicht nennt, fehlende `tags`, eine AsyncAPI-Operation, deren Kanalverweis nicht
auflöst, und eine Operation am Gateway, deren Nachricht nicht auflöst oder inline steht, sind
Befunde des Blocks; sonst fiele die Zeile still aus der Tabelle. Als Zeilenumbruch zählt jede
Stelle, an der `str.splitlines` trennt (auch U+2028 und U+2029), wie in der Zellregel von
`tools/catalog`: Die Marker liest `tools/catalog` ebenso zeilenweise, und ein solches Zeichen
machte den Rest der Zeile sonst zu einer eigenen Zeile, im schlimmsten Fall zu einem Marker.

## Was der Check nicht abdeckt

- Text außerhalb der erzeugten Blöcke von 06 und `webhooks.md` (Prosa, eine von Hand
  geschriebene zweite Tabelle) vergleicht der Check nicht; das bleibt Review.
- Ob die Texte der Verträge Englisch sind, prüft der Check nur an den drei Texten jedes
  Eintrags in `x-problem-codes` (`title`, `description`, `client_action`), und nur darauf, dass
  sie nicht leer sind und keine deutschen Umlaute oder ß enthalten. Andere Texte, etwa
  `summary` und `description` der Operationen oder `client_action` der Schließ-Codes in
  asyncapi.yaml, prüft er nicht auf die Sprache.
- Ob ein Grund in `x-idempotency-exempt` inhaltlich passt; der Check kennt nur die erlaubten
  Codes.
- Einen Schema-Verweis, den der Check verfolgt (`payload.properties.d` einer
  Ereignis-Nachricht, das Schema des Körpers eines Webhooks, `discriminator.mapping` der Union
  `Event`), löst er immer in `docs/api/openapi.yaml` auf: einen lokalen Verweis (`#/...`, auch
  in asyncapi.yaml) ebenso wie einen Verweis auf jede Datei namens `openapi.yaml`, gleich in
  welchem Verzeichnis oder unter welcher URL. Ein Verweis auf eine Datei anderen Namens ist ein
  Befund. Ob ein Verweis wirklich `docs/api/openapi.yaml` meint, prüft der Check nicht; dass er
  auflöst, prüft der Lint (G1). Andere externe `$ref` löst der Check nicht auf; für die Spalten
  der Tabellen zählt nur das letzte Segment eines Verweises.
- Schließ-Codes ab 4100 auf den Terminal-Kanälen vergleicht der Check mit nichts; ihre Quelle ist
  Kapitel 24, das nicht zu den Eingaben gehört.
- Ein Kommentar, der einem Marker nur ähnelt (etwa `<!--BEGIN GENERATED: x-->` ohne
  Leerzeichen), ist für den Check wie für `tools/catalog` ein gewöhnlicher Kommentar; einen
  eigenen Block verdeckt er nicht, weil dessen Marker dann fehlen oder nicht passen.
- Operationen unter `callbacks` durchläuft der Check nicht; der Vertrag hat keine.
- Eine Katalogzeile mit Richtung `both` gilt schon mit einer Operation in einer der beiden
  Richtungen als abgedeckt. Der Katalog kann die Richtung nicht je Op ausdrücken (die
  `both`-Zeile von `host_data` enthält `hello` und `identify`, die nur in eine Richtung laufen);
  fehlt etwa `terminal.send.frame`, sieht das der Check nicht.
- Weitere Kopien von Katalogwerten in den Schemas (über `AppScope`, die Union `Event` und die
  `type`- und `op`-Enums hinaus) kennt der Check nicht; eine neue Kopie braucht ihre Teilprüfung.
- `api.deprecation_min_notice` wird nur im Abschnitt "Benannte Werte aller Kapitel" gesucht; eine
  zweite Zeile in den Abschnitten 6 oder 7 der Konfigurationsreferenz (andere Quellen) sieht der
  Check nicht.
- `make api-tables` schreibt jede Datei, die es ändert, erst in eine temporäre Datei daneben und
  ersetzt die Ziele erst, wenn alle vollständig sind. Eine schreibgeschützte Datei, eine volle
  Platte oder ein Größenlimit hält den Lauf auf, und keine Datei ist geändert. Nur wenn das
  Umbenennen selbst scheitert, kann die erste Datei schon neu sein; die Meldung nennt sie
  (`rewrote only ...`), und der nächste Lauf von `make api-checks` meldet die andere als
  veraltet. Ist eine der Dateien ein symbolischer Link, ersetzt `make api-tables` den Link durch
  eine gewöhnliche Datei.
- Kein Server liest die Felder `x-problem-codes`, `x-status`, `x-idempotency-exempt` und
  `x-deprecated-at` vor M1-17 und M1-18; die Listen in `docs/api/redocly.yaml` hält
  `catalog-check` (M1-02).
