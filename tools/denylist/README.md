# Deny-List-Scanner

Findet instanzspezifische Begriffe, bevor sie ein öffentliches Repository erreichen
(ADR 0042). Aufruf über `make denylist` (Teil von `make check`) oder
`make denylist-history` (ganze Historie, vor dem Öffentlich-Schalten).

## Liste einrichten (Maintainer)

Die private Liste liegt außerhalb dieses Repositorys. Eine der beiden Möglichkeiten:

```sh
export BOTHY_DENYLIST_FILE=/pfad/zur/denylist.txt
# oder dauerhaft für diesen Klon (die Datei ist git-ignoriert):
echo /pfad/zur/denylist.txt > .denylist-path
```

Ein relativer Pfad in `.denylist-path` gilt relativ zum Repository. Ohne Liste schlägt
der Scan fehl. Erlaubte Commit-Identitäten stehen in `allow-authors.txt` neben der Liste
(oder `BOTHY_DENYLIST_ALLOW_AUTHORS`).

## Format

```text
# Kommentar
ein-wörtlicher-begriff
re:\bregulärer[- ]ausdruck\b
@exclude-path: pfad/mit/*glob
```

Eine Regel je Zeile, Groß- und Kleinschreibung egal. `re:` leitet einen regulären Ausdruck
nach Python-Syntax ein. Kurze Begriffe besser als Ausdruck mit Wortgrenzen `\b`, sonst
treffen sie Teile anderer Wörter. `allow-authors.txt`: eine Identität je Zeile als
`Name <email>` oder nur `<email>`.

## Was geprüft wird

| Ort | Standard | `--history` |
|---|---|---|
| Inhalt und Name jeder getrackten Datei (auch Binärdateien; bei PNG alles außer den Pixeldaten (`IDAT`), weil komprimierte Pixel zufällige Treffer liefern; `zTXt`, `iTXt` und `iCCP` zusätzlich entpackt; ein beschädigtes PNG wird roh geprüft) | ja | ja, plus jedes frühere Objekt und jeder frühere Pfad |
| Commit-Nachrichten | Commits in `--range` (Standard: noch nicht auf `origin/main`, ohne Remote alle) | alle Commits aller Refs |
| Autor, Committer und die Identitätszeilen `Signed-off-by` und `Co-authored-by` | wie Nachrichten, nur gegen die private Liste | ja |
| Branch-, Remote- und Tag-Namen, Tag-Nachrichten | ja | ja |
| Zusätzliche Namen (`--name`, `BOTHY_DENYLIST_NAMES`) | ja | ja |

Die öffentlichen Regeln in `generic.rules` laufen immer, prüfen aber keine
Commit-Identitäten: Beitragende zeichnen mit ihrer eigenen Adresse ab (DCO). Mit
`--allow-missing` (nur Fork-Pull-Requests in CI) laufen nur sie. Andere Zeilen wie
`Reviewed-by` sind normaler Nachrichtentext und bekommen alle Regeln.

Die erlaubten Identitäten aus `allow-authors.txt` gelten nur als Autor und Committer und in
einer `Signed-off-by`-Zeile, die genau den Autor oder Committer desselben Commits wiederholt
(`git commit -s`). In `Co-authored-by`, in einer fremden Abzeichnung und im Text meldet die
private Liste sie wie jede andere Fundstelle (Anforderung 22, D-W-06, D-W-37).

## Ausgabe

```text
docs/x.md:2:7: content matches private rule line 7
path ************-notes.md: path matches private rule line 5
```

Nur Fundort und Zeile der Regel, nie der Treffer; Treffer in Pfaden sind maskiert.
`--reveal` zeigt den Treffer (nur lokal, nie in CI). Rückgabewerte: 0 sauber, 1 Funde,
2 Konfigurationsfehler (fehlende Liste, kaputter Ausdruck).

## Beheben

* Treffer im aktuellen Stand: Begriff ersetzen (Beispiel-Domains example.com, Personen
  Alex, Sam, Robin).
* Treffer in Commit-Nachrichten oder -Identitäten noch nicht gepushter Commits:
  `git commit --amend` beziehungsweise Rebase mit neuer Nachricht oder Identität.
* Treffer in alter Historie: Umschreiben nach docs/quality/public-switch.md.
