---
status: accepted
date: 2026-10-10
decision-makers: Planungsleitung, Team Agenten-Plattform
---

# Lauf (Run) als eigenes Objekt für die Live-Box

## Context and Problem Statement

Während ein Agent arbeitet, soll der Mensch eine Live-Box sehen: Zustand, Dauer, Kontext in
Prozent, Kosten, aktuelles Werkzeug, letzter Text, Plan, Unteragenten, Workflows,
Hintergrundjobs, API-Wiederholungen, Stillstand. Am Ende klappt sie zu einer Zeile zusammen. Wie
wird dieser Zustand modelliert, gespeichert und an Geräte verteilt, ohne Ereignis-Log und Geräte
mit hoher Änderungsrate zu überlasten?

## Decision Drivers

- Hohe Änderungsrate während eines Laufs, aber wenige Zustandswechsel.
- Geräte nach einer Pause brauchen den richtigen Zustand, nicht jede Zwischenstufe.
- Zustände steuern Rechte, Kosten, Statistik und "Läuft gerade"; sie müssen dauerhaft und
  geordnet sein.
- Kein Doppelstart, keine verlorene Abbruch-Meldung.
- Keine Revisionsflut an Nachrichten.

## Considered Options

1. Lauf als eigenes Objekt (`runs`, `run_steps`, `run_followups`) mit Zustandsmaschine;
   Zustandswechsel als dauerhafte Ereignisse, Inhalte als kurzlebige JSON-Patches an fokussierte
   Threads; Anker-Nachricht `kind = run` im Verlauf.
2. Live-Box als laufend bearbeitete Nachricht (wie die Fortschritts-Blase in Discord).
3. Streaming als Nachrichten-Entwürfe (`message.draft`-Ereignisse) ohne eigenes Objekt.
4. Jeder Schritt als dauerhaftes Ereignis (Event-Sourcing des Laufs im Ereignis-Log).

## Decision Outcome

Gewählt: Option 1, weil sie dauerhafte Zustände und flüchtige Inhalte trennt (R-SY-05), Patches
über stabile Pfade erlaubt (R-SY-06) und den Lauf zum Träger von Kosten, Backend-Richtlinie,
Sitzung und Marke macht.

### Consequences

- Gut, weil das Ereignis-Log nur Zustandswechsel trägt; Patches gehen nur an Geräte, die den
  Thread zeigen.
- Gut, weil ein Gerät nach einer Pause das ganze Dokument per REST lädt und danach Patches
  anwendet.
- Gut, weil Zustandswechsel als Compare-and-swap Doppelstarts und späte Patches ausschließen.
- Schlecht, weil Live-Box-Inhalte, die nur kurzlebig verteilt wurden, bei einem Server-Neustart
  zwischen zwei Speichervorgängen verloren gehen können; der nächste Patch oder Abruf heilt das.
- Schlecht, weil Adapter eine Patch-Disziplin brauchen (Drossel, `base_version`); das SDK liefert
  sie (ADR 0026).

### Confirmation

- Tests zur Zustandsmaschine: jeder nicht erlaubte Übergang wird abgelehnt (docs/spec/03-datenmodell.md
  `runs`; docs/spec/09-agenten.md Abschnitt 5.1).
- Test AG-12: Patch auf verbotenen Pfad ergibt `400 validation_failed` mit `errors[].code`
  `patch_path_forbidden` (ADR 0015; corrected by M2-10, D-M2-10-15), alte `base_version` ergibt
  `409` mit Dokument.
- Lasttest: Anzahl dauerhafter Ereignisse je Lauf bleibt unabhängig von der Patch-Rate.
- Test, dass Live-Box-Anker keine Revisionen erzeugen (03 `message_revisions`).
- M2-05 names the tests of the state machine and the run object: `TestCanTransition_AllPairs`,
  `TestReasonTable`, `TestNormalizeAdapterReason` and `TestTriggerStateMapping`
  (`services/server/internal/runs/domain`); `TestTransition_EveryDisallowedPairRejected`,
  `TestTransition_RaceOfTwoStartsOneWins`, `TestTransition_TriggerStatePerEndState`,
  `TestTransition_UnknownAdapterReasonBecomesAdapterError`, `TestOpen_AtMostOneActivePerThread`
  and `TestAcceptance_ThreadMessageOpensRun` (`services/server/test/integration/runs`).

## Pros and Cons of the Options

### Option 1: Eigenes Objekt mit Patches

- Gut, weil Zustand, Inhalte und Kosten ein Objekt mit einer Version haben.
- Gut, weil der Client nur den neuesten Stand zeichnet.
- Neutral, weil ein eigenes Dokument-Schema (`run_document.v1`) gepflegt wird.
- Schlecht, weil zwei Wege (dauerhaft und kurzlebig) verstanden werden müssen.

### Option 2: Bearbeitete Nachricht

- Gut, weil kein neues Konzept nötig ist und Plattformen wie Discord es so lösen.
- Schlecht, weil jede Bearbeitung eine Revision und ein dauerhaftes Ereignis wäre.
- Schlecht, weil Zustand, Kosten und Schritte nur Text wären und nicht auswertbar.

### Option 3: Nachrichten-Entwürfe

- Gut, weil Textstreaming einfach ist.
- Schlecht, weil Schritte, Unteragenten, Plan und Kennzahlen keinen Platz haben.
- Schlecht, weil der Zustand des Laufs (Rückfrage offen, abgebrochen) nicht modelliert ist.

### Option 4: Jeder Schritt dauerhaft

- Gut, weil der Verlauf eines Laufs lückenlos nachholbar wäre.
- Schlecht, weil die Patch-Rate das Ereignis-Log und alle Geräte belastet, auch Geräte, die den
  Thread gar nicht zeigen.
- Schlecht, weil die Aufbewahrung des Logs dann vom Agenten-Verkehr bestimmt würde.

## More Information

- docs/spec/04-sync-protokoll.md Abschnitt 11; docs/spec/09-agenten.md Abschnitte 5 und 6.
- `runs.plan`, `metrics.last_api_retry` und `run.start` für den Übergang aus `waiting_input`
  stehen in docs/spec/03-datenmodell.md (`runs`) und docs/spec/04-sync-protokoll.md Abschnitt 11.
