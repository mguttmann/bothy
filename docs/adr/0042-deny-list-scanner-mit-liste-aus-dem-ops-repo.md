---
status: accepted
date: 2026-10-10
decision-makers: Maintainer
---

# Deny-List-Scanner mit Liste aus dem Ops-Repo

## Context and Problem Statement

Das Produkt-Repository darf nichts Instanzspezifisches enthalten (ADR 0002): keine Firmen-
und Personennamen, Domains, Hostnamen, internen Kennungen. Welche Begriffe verboten sind,
ist selbst Instanzwissen und darf nicht im Produkt stehen. Lecks entstehen nicht nur in
Dateien, sondern auch in Dateinamen, Commit-Nachrichten, Commit-Identitäten, Branch- und
Tag-Namen und in alten Commits.

## Decision Drivers

* Die Liste liegt außerhalb des Produkt-Repositorys.
* Fehlt die Liste, gilt der Scan als fehlgeschlagen, nicht als bestanden.
* Alle Leck-Orte werden geprüft, auf Wunsch die ganze Historie.
* CI-Logs im öffentlichen Repository verraten die Liste nicht.
* Beiträge aus Forks ohne Listen-Zugriff werden trotzdem geprüft.

## Considered Options

* Eigener Scanner (Python, Standardbibliothek) mit privater Liste und öffentlichen
  generischen Regeln
* gitleaks mit eigener, privater Konfiguration
* Nur Review durch Menschen

## Decision Outcome

Gewählt: "Eigener Scanner", weil nur er alle Leck-Orte abdeckt, ohne Liste fehlschlägt
und Treffer maskiert.

* Liste: `BOTHY_DENYLIST_FILE` oder die git-ignorierte Datei `.denylist-path` mit dem Pfad;
  kein fest verdrahteter Standardpfad. Format: eine Regel je Zeile, `#` Kommentar, `re:`
  für reguläre Ausdrücke, sonst wörtlich, Groß- und Kleinschreibung egal.
* Erlaubte Commit-Identitäten (`allow-authors.txt` neben der Liste) werden nicht gemeldet,
  damit die öffentliche Identität der Maintainer ihre Commits nicht blockiert.
* Geprüft: Inhalt und Name jeder getrackten Datei (auch Binärdateien), Commit-Nachrichten
  und Identitäten, Branch- und Tag-Namen, Tag-Nachrichten; `--history` zusätzlich jedes
  Objekt auf jedem Ref.
* `--allow-missing` nur für Fork-Pull-Requests: dann laufen nur die öffentlichen Regeln
  aus `tools/denylist/generic.rules` (Home-Pfade, Tailnet-Namen, E-Mail-Adressen außerhalb
  der Beispiel-Domains, private IPv4-Bereiche, Discord-IDs). Die volle Liste läuft beim
  Maintainer lokal und im Merge-Gate.
* In CI kommt die Liste aus einem Secret der geschützten Umgebung `denylist`; der
  PR-Branch-Name wird nur über eine Umgebungsvariable übergeben, nie über eine
  Kommandozeile.

### Consequences

* Gut, weil ein Leck an jedem dieser Orte vor dem Push auffällt.
* Gut, weil Ausgaben nur Fundort und Regelzeile zeigen; Pfade mit Treffern werden maskiert.
* Schlecht, weil ein eigenes Werkzeug gepflegt wird. Es ist klein und vollständig getestet.
* Schlecht, weil eine erlaubte Identität selbst nichts Verbotenes enthalten darf; das
  prüft der Scanner nicht, die Liste der erlaubten Identitäten wird im Ops-Repository
  gepflegt und reviewt.

### Confirmation

`tools/denylist/tests/test_denylist_scan.py` prüft jeden Leck-Ort, das Fehlschlagen ohne
Liste, `--allow-missing`, erlaubte Identitäten, Maskierung und die generischen Regeln
gegen das Repository selbst. Gate `denylist` in `make check` und als eigener CI-Job;
`make denylist-history` in der Öffentlich-Schalten-Checkliste.

## Pros and Cons of the Options

### Eigener Scanner

* Gut, weil Dateinamen, Identitäten, Refs und Historie abgedeckt sind.
* Gut, weil fail closed und maskierte Ausgabe eingebaut sind.
* Schlecht, weil eigener Code.

### gitleaks mit privater Konfiguration

* Gut, weil ein bewährter Scanner mit Historien-Modus.
* Schlecht, weil Branch- und Tag-Namen und Commit-Identitäten nicht geprüft werden und eine
  fehlende Konfiguration auf die Standardregeln zurückfällt statt fehlzuschlagen.
* Neutral, weil gitleaks für echte Secrets trotzdem läuft (Gate `gitleaks`).

### Nur Review

* Gut, weil ohne Werkzeug.
* Schlecht, weil Menschen Namen in Commit-Metadaten und alten Commits übersehen.

## More Information

Anleitung: tools/denylist/README.md. Öffentlich-Schalten: docs/quality/public-switch.md.
