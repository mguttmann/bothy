# Red-Team-Gate

Ein unabhängiger Prüfer sucht aktiv nach Wegen, die Regeln des Produkts zu brechen. Sein
Urteil ist `JA` (darf gemergt, gepusht oder veröffentlicht werden) oder `NEIN` mit Grund.

## Wann

| Anlass | Umfang | Nachweis |
|---|---|---|
| Vor jedem Push und jedem lokalen Merge eines Tickets (Anforderung 23, PLAN 3.9) | Diff des Branches gegen `main` | `.review/<sha>.md` des gepushten oder gemergten Commits; der pre-push-Hook prüft ihn (`tools/hooks/review_gate.py`) |
| Am Ende jeder Etappe | ganzer Stand gegen den Stand der letzten Etappe | Bericht im Ops-Repository, verlinkt im Abschluss der Etappe |
| Vor dem Öffentlich-Schalten | ganzer Stand | Bericht im Ops-Repository (public-switch.md, Schritt 1) |

Das Review je Push ist kurz und auf den Diff beschränkt; das Gesamt-Review findet, was
erst im Zusammenspiel mehrerer Tickets sichtbar wird.

## Wer prüft

* Unabhängig: nicht die Person oder Sitzung, die den Stand gebaut hat. Im
  Ein-Personen-Betrieb ist das eine frische Sitzung eines KI-Prüfers ohne Kontext aus der
  Bau-Sitzung, oder eine zweite Person.
* Nur lesend: der Prüfer ändert nichts am Arbeitsbaum, pusht nicht und hat keine Secrets.
  Er bekommt den Diff, die Spezifikation und das Bedrohungsmodell.

## Was geprüft wird

1. **Rechte.** Jede neue Route, jedes Ereignis, jede Zustellung geht durch `authorize`
   mit einer Aktion aus Kapitel 05; kein Sichtrecht ergibt 404. Gibt es einen Weg, Daten
   eines fremden Bereichs zu sehen (Zähler, Suche, Vorschläge, Push, Webhooks)?
2. **Backend-Richtlinie.** Kann eine andere Person als der `credential_holder` einen
   Agenten mit `billing_mode = personal` auslösen (I1 bis I3)?
3. **Agenten.** Kann Inhalt aus einem Bereich einen Agenten in einem anderen Bereich
   steuern? Kann ein Agent mit mächtigen Werkzeugen von einer Person mit wenig Rechten
   ausgelöst werden?
4. **Lieferkette und CI.** Neue Actions gepinnt, keine neuen Trigger mit fremdem Code,
   keine Secrets in Jobs, die sie nicht brauchen; neue Abhängigkeiten mit erlaubter
   Lizenz.
5. **White Label und Datenschutz.** Nichts Instanzspezifisches, keine Inhalte in
   Telemetrie, keine personenbezogenen Daten in Logs.
6. **Gates.** Wurde ein Gate abgeschwächt, eine Schwelle gesenkt, ein Test gelöscht oder
   übersprungen?

## Ablauf

1. Bau-Sitzung meldet "fertig" mit grünem `make check`.
2. Prüfer erhält den Diff (`git diff <basis>...HEAD`), Kapitel 07, 10, 19 und diese Seite.
3. Prüfer schreibt einen Bericht: Befunde mit Datei und Zeile, Schweregrad, und als letzte
   Zeile `Urteil: JA` oder `Urteil: NEIN <Grund>`. Beim Review je Push liegt er unter
   `.review/<sha>.md` (`<sha>` aus `git rev-parse HEAD`).
4. Bei `NEIN`: beheben, Tests für jeden Befund, dann prüft der Prüfer nur das Delta
   erneut. Kommt dasselbe `NEIN` erneut, entscheidet der Owner, nicht eine weitere Runde.
5. Bei `JA`: Merge beziehungsweise Abschluss der Etappe. Der Bericht je Push steht im Text des
   Pull Requests (Abschnitt "Review report" der Vorlage); eine git-ignorierte Datei lässt sich
   nicht verlinken.
6. Das Bedrohungsmodell (`docs/security/threat-model.md`) wird mit neuen Bedrohungen und
   Gegenmaßnahmen nachgezogen, bevor die Etappe als fertig gilt.

## Was das Gate nicht ersetzt

Es ersetzt keine Penetrationstests (Kapitel 19) und keine Gates; es prüft, was Werkzeuge
nicht prüfen können: Absicht, Zusammenspiel und fehlende Prüfungen.
