---
status: accepted
date: 2026-10-10
decision-makers: Planungsleitung
---

# Freigabe-Modell: Stufen, Zusatzrechte, Vererbung, kein Verbot

## Context and Problem Statement

Menschen, Gruppen, Agenten und Apps brauchen Zugriff auf Bereiche und Kanäle. Agenten verlangen
eine eigene Stufe: Lesen und Schreiben ist etwas anderes als einen Agenten auslösen, der Werkzeuge
benutzt und Kosten erzeugt. Die Verwaltung soll in einem kleinen Team ohne Fachwissen richtig
gelingen, und jede Entscheidung soll in einem Satz erklärbar sein ("Sam darf hier schreiben, weil
...").

Welches Modell bildet Zugriff ab, so dass es sicher, erklärbar und für Invarianten wie I1 prüfbar
ist?

## Decision Drivers

- Erklärbarkeit in Klartext für "Wer hat Zugriff?" und die Vorschau.
- Prüfbare Invarianten (I1: Trigger-Menge eines persönlichen Agenten).
- Wenige Konzepte; die Lehre aus einer früheren Chat-Plattform: zu viele Funktionen.
- Zeitlich begrenzte Freigaben, geprüft bei jedem Aufruf.
- Delegation an Bereichsverantwortliche ohne Rechte-Ausweitung.

## Considered Options

1. Aufsteigende Stufen `read` < `write` < `trigger`, Zusatzrechte (`upload`, `approve_actions`,
   `export`, `manage`), Vererbung Instanz -> Bereich -> Kanal, höchste gültige Stufe gilt, keine
   Verbote.
2. Rollen mit Erlauben- und Verbieten-Overrides je Kanal (Muster großer Chat-Plattformen).
3. Beziehungsbasiertes Modell mit Tupeln (Zanzibar-Muster).
4. Flache Zugriffsliste je Kanal ohne Vererbung.

## Decision Outcome

Gewählte Option: 1, weil sie die wenigsten Konzepte hat, jede Entscheidung als Maximum über eine
kurze Liste gültiger Freigaben erklärbar macht und die Trigger-Menge für I1 direkt berechenbar ist.

Festlegungen (Details in [../spec/07-rechte.md](../spec/07-rechte.md)):

- Subjekte: Mensch, Gruppe (nur Menschen), Agent und App (höchstens `write`, als Zusatzrecht nur
  `upload`).
- Gültigkeit je Aufruf: nicht widerrufen, nicht abgelaufen, Subjekt aktiv, Mitgliedschaft gültig.
- Freigaben sind unveränderlich; Änderung ist Widerruf plus neue Freigabe (R-DM-09).
- Delegation mit `manage` nur bis zur eigenen Stufe und zu eigenen Zusatzrechten, Widerruf im selben
  Rahmen; Freigaben an Agenten und Apps nur durch den Owner.
- Schutzregeln (SR1 bis SR12 in 07) stehen über Freigaben.

### Consequences

- Gut, weil jede Entscheidung erklärbar ist: höchste Stufe, Herkunft, Ablauf.
- Gut, weil es keine Reihenfolge-Fehler zwischen Erlauben und Verbieten gibt.
- Gut, weil I1 als Mengenrechnung über Freigaben, Gruppen, Owner und Bediener prüfbar ist.
- Schlecht, weil "alles außer X" nicht ausdrückbar ist; man gibt Freigaben auf die einzelnen
  Kanäle statt auf den Bereich.
- Schlecht, weil eine Freigabe auf einen Bereich auch künftige Kanäle des Bereichs öffnet; die
  Vorschau beim Anlegen eines Kanals zeigt, wer ihn erbt.

### Confirmation

- Testmatrix Prinzipal x Ziel x Aktion inklusive abgelaufen, widerrufen, überlappend, Gruppe,
  Agent im fremden Bereich (07, Abschnitt 11.1).
- Differenztest Cache gegen langsame Referenz in zufälligen Welten (07, AZ-P8).
- I1-Test je Änderungsart (AZ-P7).
- Check in `grants`: Agenten und Apps nie `trigger`, Zusatzrechte nur `upload` (03 `grants`,
  R-DM-20).

## Pros and Cons of the Options

### 1. Stufen, Zusatzrechte, Vererbung, kein Verbot

- Gut, weil klein und erklärbar.
- Gut, weil Studie C genau dieses Modell vorschlägt und es mit Agenten-Stufe `trigger` endet.
- Neutral, weil Zusatzrechte eine zweite Achse sind; sie bleibt geschlossen (vier Werte).
- Schlecht, weil Ausnahmen nach unten nicht gehen.

### 2. Rollen mit Erlauben- und Verbieten-Overrides

- Gut, weil Ausnahmen nach unten möglich sind.
- Schlecht, weil die Wirkung von der Reihenfolge der Overrides abhängt und schwer zu erklären ist.
- Schlecht, weil Rollen-Hierarchien bewusst nicht Teil des Produkts sind (Baseline Abschnitt 4).

### 3. Beziehungsbasiertes Modell

- Gut, weil sehr ausdrucksstark (geteilte Dokumente, verschachtelte Gruppen).
- Schlecht, weil die Ausdrucksstärke hier nicht gebraucht wird und Erklärbarkeit und I1-Prüfung
  schwerer macht.
- Schlecht, weil es meist einen eigenen Dienst oder eine eigene Bibliothek mit eigenem Modell
  verlangt.

### 4. Flache Liste je Kanal

- Gut, weil am einfachsten.
- Schlecht, weil jeder neue Kanal alle Freigaben neu braucht; Bereiche als Einheit fehlen.

## More Information

### Teilentscheidung: Filterung von Listen und Suche

Optionen:

1. Rechte-Cache in `authz` je Prinzipal, gültig für eine Rechte-Version und bis zum nächsten
   Ablaufzeitpunkt; Abfragen bekommen die Liste sichtbarer Kanäle als Parameter.
2. Materialisierte Sicht wirksamer Rechte in der Datenbank (Tabelle oder Materialized View), mit
   Join in jeder Abfrage.

Gewählt: 1 (gleich R-DM-19 im Kern-Vertrag), weil der Cache aus derselben Funktion entsteht wie
`authorize` und nicht abweichen kann; Ablauf durch Zeit ohne Schreibvorgang korrekt bleibt (der
Cache endet bei `valid_until`); Gruppenänderungen keine Massen-Schreibvorgänge auslösen. Option 2
wäre eine zweite Wahrheit, bräuchte trotzdem eine Zeitprüfung in jeder Abfrage und liefert bei einem
Pflegefehler still falsche Ergebnisse. Grenze von Option 1: sehr lange Listen sichtbarer Kanäle;
Abhilfe ist eine temporäre Tabelle aus demselben Snapshot in derselben Transaktion (07, 7.4).

Weitere Quellen: Baseline Abschnitt 3 (Freigabe), Studie C (Tabelle `freigabe`, keine Verbots-Zeilen,
Ablauf je Aufruf geprüft).
