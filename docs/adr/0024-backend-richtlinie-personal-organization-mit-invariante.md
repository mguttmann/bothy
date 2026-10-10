---
status: accepted
date: 2026-10-10
decision-makers: Planungsleitung, Team Agenten-Plattform
consulted: Owner (offene Frage zur Owner-Ausnahme, siehe More Information)
---

# Backend-Richtlinie personal/organization mit Invariante

## Context and Problem Statement

Ein Agent kann auf dem persönlichen Abo einer Person laufen oder auf einem Organisations-API-Key.
Die Anthropic-Bedingungen sehen persönliche Abos für die gewöhnliche, eigene Nutzung vor und
verbieten Drittentwicklern, Anfragen über Free-, Pro- oder Max-Zugangsdaten im Namen ihrer Nutzer
zu leiten oder Claude.ai-Zugangsdaten zu sammeln, zu speichern oder zu vermitteln
(https://code.claude.com/docs/en/legal-and-compliance, abgerufen 2026-10-10, [belegt]). Wie stellt
das Produkt sicher, dass ein persönliches Abo nie fremde Aufträge ausführt, ohne selbst
Zugangsdaten zu kennen?

## Decision Drivers

- Kein Pfad, auf dem eine andere Person einen persönlichen Agenten auslöst, auch nicht über
  Gruppen, Vererbung, Owner-Rechte oder Bedienerwechsel.
- Der Server kennt keine Zugangsdaten (B10, I5).
- Prüfung dort, wo sich Rechte ändern, dort, wo aufgerufen wird, und dort, wo die Laufzeit startet.
- Umschalten muss bewusst, begründet und nachvollziehbar sein.
- Fehler sichtbar machen, nicht still verhindern.

## Considered Options

1. Feld `billing_mode` je Agent mit drei Prüfungen: Invariante I1 transaktional bei jeder
   Rechte-Änderung, I2 vor jedem Aufruf, I3 im Adapter mit signierter Marke; Umschalten I4 mit
   Step-up und Grund.
2. Nur Prüfung beim Aufruf (I2), ohne Invariante auf Rechte-Änderungen.
3. Nur Organisations-Keys, kein persönlicher Modus.
4. Wahl des Backends je Auslösung durch den Auslöser.

## Decision Outcome

Gewählt: Option 1 (Baseline Abschnitt 5). Sie verhindert den Fehlerfall an drei unabhängigen
Stellen und hält das Rechte-Modell ehrlich: Niemand bekommt eine Stufe `trigger` angezeigt, die er
bei einem persönlichen Agenten nie nutzen könnte.

### Consequences

- Gut, weil Fehlkonfiguration (Freigabe, Gruppe, zweiter Owner) beim Speichern scheitert, mit Code
  und Hinweis, statt später als abgelehnter Aufruf.
- Gut, weil der Adapter mit dem Claim `billing_mode` seine Laufzeit gegen das Soll prüft.
- Gut, weil der Notschalter "alle auf Organisation" jederzeit geht.
- Schlecht, weil jeder Owner überall `trigger` hat: `personal` geht nur, solange der Inhaber der
  einzige Owner ist (docs/spec/03-datenmodell.md Abschnitt 6.4).
- Schlecht, weil die I1-Prüfung bei Rechte-Änderungen zusätzliche Abfragen braucht; sie läuft nur
  für Agenten mit `personal` und unter der Sperre der Rechte-Version.

### Confirmation

- Tests BR-01 bis BR-08 in docs/spec/10-backend-richtlinie.md (je Aktion aus 05 Regel 1.6 eine
  verletzende Änderung; Wettlauf; Entzug zwischen Annahme und Zustellung; Abweichung Soll und Ist).
- Test 5 und 6 aus docs/spec/05-aktionen.md Abschnitt 9.
- Ein Leck-Test prüft, dass keine Antwort und kein Protokoll Schlüsselwerte enthält (BR-13).

## Pros and Cons of the Options

### Option 1: Invariante plus Aufruf- und Adapter-Prüfung

- Gut, weil drei unabhängige Stellen denselben Fehler abfangen.
- Gut, weil Ablehnungen im Audit-Log stehen.
- Neutral, weil die Konsole einen geführten Ablauf "erst umstellen, dann freigeben" braucht.
- Schlecht, weil mehrere Owner den persönlichen Modus ausschließen.

### Option 2: Nur Prüfung beim Aufruf

- Gut, weil einfacher zu bauen.
- Schlecht, weil Menschen Rechte sehen, die bei jedem Versuch scheitern; der Fehler zeigt sich
  erst beim Nutzer, nicht beim Admin.
- Schlecht, weil eine Lücke in der Aufruf-Prüfung allein zum Verstoß führt.

### Option 3: Nur Organisations-Keys

- Gut, weil kein Abo-Risiko besteht.
- Schlecht, weil eine Person ihr eigenes Abo nicht für ihre eigenen Agenten nutzen könnte,
  obwohl die Bedingungen die eigene Nutzung vorsehen; die Anforderung aus Baseline Abschnitt 5
  (Modus `personal` für die eigene Nutzung) wäre verfehlt.

### Option 4: Wahl je Auslösung

- Gut, weil flexibel.
- Schlecht, weil die Laufzeit (z. B. ein Hermes-Profil) ihre Anmeldung nicht je Turn sicher wechseln
  kann und jede Auslösung eine Entscheidung mit Kostenfolge vom Nutzer verlangen würde.
- Schlecht, weil der Zusammenhang "wer auslöst, zahlt auf wessen Abo" für Dritte offen bliebe.

## More Information

- Baseline Abschnitt 5 (I1 bis I5); docs/spec/10-backend-richtlinie.md.
- Offene Frage an den Owner: Soll ein zweiter Owner möglich sein, ohne persönliche Agenten
  umzustellen? Möglich wäre eine Owner-Ausnahme: Owner außer dem Inhaber zählen nicht zur I1-Menge
  und werden durch I2 abgewiesen. Empfehlung bis zur Entscheidung: strenge Fassung behalten. Eine
  Änderung wäre eine neue ADR, die diese ersetzt.
