---
status: accepted
date: 2026-10-10
decision-makers: Maintainer
---

# ADRs im MADR-Format, DECISIONS.md als Ruling-Log

## Context and Problem Statement

Das Produkt entsteht in Etappen, oft durch unbeaufsichtigte Bau-Sitzungen. Wer später
etwas ändert, muss wissen, warum es so ist, und welche Prüfung die Entscheidung hält.
Es gibt zwei Arten von Entscheidungen: tragende (Architektur, Verträge, Werkzeuge,
Lizenz, Prozess) und kleine Festlegungen im Alltag. Beide brauchen einen festen Ort.

## Decision Drivers

* Entscheidungen müssen mit Grund und Alternativen auffindbar sein, auch für Agenten.
* Jede Entscheidung braucht eine Prüfung, die sie im Repository hält.
* Wenig Aufwand für kleine Festlegungen, sonst werden sie nicht notiert.
* Reviewbar im Pull Request wie Code.

## Considered Options

* MADR 4 für tragende Entscheidungen plus `docs/DECISIONS.md` als Ruling-Log
* Nur ein fortlaufendes Entscheidungs-Log
* ADRs im Format nach Nygard (Kontext, Entscheidung, Folgen)
* Entscheidungen in Issues oder einem Wiki

## Decision Outcome

Gewählt: "MADR 4 plus Ruling-Log", weil MADR Optionen, Treiber und eine Confirmation
erzwingt und das Log kleine Festlegungen billig hält.

Regeln: Dateiname `docs/adr/NNNN-titel-in-kebab-case.md`, Kopfdaten `status` und `date`,
Überschriften englisch wie MADR, Text deutsch, mindestens zwei echte Optionen, eine
Confirmation. Eine ADR beginnt als `proposed` im Pull Request und wird beim Merge
`accepted` oder `rejected`. Eine überholte ADR wird nicht gelöscht, sondern
`superseded by ADR-NNNN`. Kleine Festlegungen stehen mit Datum und Grund in
`docs/DECISIONS.md`; Kapitel der Spezifikation führen ihre eigenen Rulings
(zum Beispiel R-QA-01 in Kapitel 23).

### Consequences

* Gut, weil jede tragende Entscheidung ihre Alternativen und ihre Prüfung zeigt.
* Gut, weil der Index (`docs/adr/README.md`) generiert wird und nicht veraltet.
* Schlecht, weil eine ADR mehr Text als ein Log-Eintrag ist; dafür bleibt das Log für
  Kleines.

### Confirmation

`make docs` ruft `tools/docs/adr_index.py --check` auf: es schlägt fehl bei falschem
Dateinamen, doppelter Nummer, fehlendem Titel oder veraltetem Index. Fehlende Kopfdaten
erscheinen als `MISSING` in der Index-Tabelle und fallen im Review auf. Die Vorlage ist
[ADR 0000](0000-vorlage-madr-4.md).

## Pros and Cons of the Options

### MADR 4 plus Ruling-Log

* Gut, weil MADR die Felder Decision Drivers und Confirmation vorgibt.
* Gut, weil Markdown im Repository reviewbar und versioniert ist.
* Neutral, weil zwei Orte gepflegt werden; die Abgrenzung ist "tragend oder nicht".

### Nur ein Entscheidungs-Log

* Gut, weil minimaler Aufwand.
* Schlecht, weil Optionen und Prüfung meist fehlen.

### ADRs nach Nygard

* Gut, weil kurz.
* Schlecht, weil keine Pflicht zu Optionen und Confirmation.

### Issues oder Wiki

* Gut, weil Diskussion und Ergebnis an einem Ort liegen.
* Schlecht, weil nicht versioniert mit dem Code und nicht im Pull Request prüfbar.

## More Information

MADR: https://adr.github.io/madr/ (abgerufen 2026-10-10) [belegt]. Ablauf für
Beitragende: `CONTRIBUTING.md`, Abschnitt "Architecture decisions".
