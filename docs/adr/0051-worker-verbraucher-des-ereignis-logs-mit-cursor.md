---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team A (Architektur), Team G (Plattformdienste), Team B (API)
informed: Team F (Betrieb)
---

# Worker-Verbraucher des Ereignis-Logs mit Cursor statt Job-Einfügen im Schreibpfad

## Context and Problem Statement

Push und ausgehende Webhooks sind Folgen dauerhafter Ereignisse, die nach außen gehen. Sie müssen
nach einem Commit genau einmal angestoßen werden, auch wenn ein Prozess abstürzt oder mehrere
`worker` laufen. Das Ereignis-Log ist commit-geordnet und lückenlos (ADR 0009). Wo entstehen die
Jobs für diese Folgen: in der schreibenden Transaktion jedes Erzeugers oder in einem Verbraucher,
der das Log liest?

## Decision Drivers

- Jede Folge entsteht genau einmal, auch bei Absturz und mit mehreren `worker`.
- Erzeuger kennen ihre Verbraucher nicht; ein neuer Verbraucher ändert keinen Erzeuger.
- Die serialisierte Commit-Phase am Zähler `event_seq` bleibt kurz
  ([02-architektur.md](../spec/02-architektur.md), Abschnitt 11.7).
- Keine externen Aufrufe in Transaktionen (R-AR-07).
- Kein zusätzlicher Dienst im Betrieb (B3, ADR 0009).
- Rechte gelten zur Zustellzeit (B7).

## Considered Options

- Verbraucher je Name mit dauerhaftem Cursor in `event_consumer_cursors`; eine Runde sperrt die
  Zeile, liest ab Cursor, legt Jobs an und rückt den Cursor in derselben Transaktion vor
- Jobs in der schreibenden Transaktion jedes Erzeugers einfügen (River `InsertTx`)
- Verarbeitung direkt aus `NOTIFY` im Speicher
- Eigener Cursor je Ziel (je Webhook-Abo, je Push-Registrierung), ohne zentralen Verbraucher
- Logical Decoding oder externer Broker

## Decision Outcome

Gewählte Option: "Verbraucher mit Cursor in `event_consumer_cursors`", weil Cursor und Jobs in
einer Transaktion committen und das Log ohnehin der Ausgangskorb ist. Ablauf nach
[02-architektur.md](../spec/02-architektur.md), Abschnitt 11.5, Tabelle in
[03-datenmodell.md](../spec/03-datenmodell.md), Abschnitt 4.10 (R-DM-27), Protokoll in
[04-sync-protokoll.md](../spec/04-sync-protokoll.md), Abschnitt 7.6:

1. Je Verbraucher-Name (`push`, `webhooks`) gibt es eine Zeile mit `last_seq`. Ein neuer
   Verbraucher ist eine additive Migration der erlaubten Namen.
2. Eine Runde im `worker`: Transaktion öffnen, Zeile mit `FOR UPDATE SKIP LOCKED` sperren (ist sie
   gesperrt, endet die Runde sofort), bis zu `consumer.batch_size` Ereignisse ab `last_seq` in
   `seq`-Reihenfolge lesen, je Ereignis Empfänger bestimmen und River-Jobs mit `InsertTx` anlegen,
   `last_seq` vorrücken, committen.
3. Geweckt wird über dasselbe `NOTIFY event_log` wie der Log-Tail, zusätzlich alle
   `consumer.poll_interval`.
4. Jeder Job prüft vor dem Versand erneut `push.deliver` bzw. `webhook_out.deliver` mit den
   aktuellen Rechten ([05-aktionen.md](../spec/05-aktionen.md), R-AZ-11).
5. Ausgehende Webhooks haben zusätzlich den eindeutigen Schlüssel `(subscription_id, event_seq)` in
   `webhook_deliveries` und je Abo `last_delivered_seq` für Reihenfolge und Lückenerkennung
   ([11-erweiterungen.md](../spec/11-erweiterungen.md), Abschnitte 6 und 7).
6. Nach einer Wiederherstellung setzt `bothy restore finalize` jeden Cursor auf den neuen Kopf
   (ADR 0059).

Der Rahmen entsteht mit dem ersten Verbraucher (Webhooks, Bau-Ticket M2-01), nicht vorher
([PLAN.md](../PLAN.md), P-13).

### Consequences

- Gut, weil der Schreibpfad keine Folge kennt; Push, Webhooks und spätere Verbraucher ändern keinen
  Erzeuger.
- Gut, weil Jobs genau einmal entstehen: Cursor und Jobs committen zusammen, und ein zweiter
  `worker` überspringt die gesperrte Zeile [Q1, belegt].
- Gut, weil die Transaktion des Erzeugers nicht länger wird; die Arbeit für Folgen liegt außerhalb
  der serialisierten Commit-Phase.
- Gut, weil Rechte zur Zustellzeit gelten: Ein Entzug zwischen Commit und Versand unterdrückt die
  Zustellung.
- Schlecht, weil ein Verbraucher-Name immer nur von einem `worker` zugleich abgearbeitet wird; der
  Durchsatz je Verbraucher hängt an der Rundengröße. Reicht er nicht, braucht es eine neue
  Entscheidung (zum Beispiel Teilung nach Bereich).
- Schlecht, weil Folgen eine Verzögerung bis zur nächsten Runde haben; `perf.push_handoff_p95`
  hält sie fest.
- Schlecht, weil ein Verbraucher, der länger als `sync.event_retention` steht, Ereignisse verliert,
  sobald ihre Partition abgehängt ist. Für Webhook-Abos erkennt `last_delivered_seq` die Lücke und
  schaltet das Abo mit Hinweis ab (11, Abschnitt 6). For the consumer itself the gauges
  `bothy.event.consumer.lag` and `bothy.event.consumer.pending_age` and the alarm
  `BothyEventConsumerLag` at `consumer.pending_alert_after`, strictly below
  `sync.event_retention`, report the stall long before (M2-01, see More Information).

### Confirmation

- Test aus Bau-Ticket M2-01: zwei `worker` parallel, jedes Ereignis erzeugt genau einmal Jobs; ein
  Absturz mitten in einer Runde (Abbruch vor dem Commit) erzeugt keine Doppel; `NOTIFY` und Poll
  wecken beide. Built as `TestConsumersTwoWorkersExactlyOnce`,
  `TestConsumerCrashMidRoundNoDuplicates` (terminated backend, handler error, panic, cancelled
  context), `TestConsumerWakesOnNotify`, `TestConsumerWakesOnPoll`,
  `TestConsumerListenLossLosesNothing`, `TestConsumerRoundJobsAndCursorCommitTogether` and
  `TestConsumerRoundLockedEndsAtOnce` in `services/server/internal/sync/adapters/postgres`; the
  lag gauges by `TestConsumerLagGauges`, the alarm by the promtool cases of
  `BothyEventConsumerLag`.
- Test: Ein Rechte-Entzug zwischen Runde und Job verhindert den Versand.
- Lasttest `perf/push_handoff` mit `perf.push_handoff_p95` (Bau-Ticket M3-24).
- Wiederherstellungstest: Nach `bothy restore finalize` steht jeder Cursor auf `head_seq`
  (ADR 0059).

## Pros and Cons of the Options

### Verbraucher mit Cursor

- Gut, weil nur PostgreSQL und River nötig sind, die ohnehin laufen.
- Schlecht, weil eine weitere Zeile pro Verbraucher und ein Rundenrahmen gebaut werden müssen.

### Jobs in der Transaktion des Erzeugers

- Gut, weil River das transaktionale Einfügen anbietet [Q2, belegt] und kein Cursor nötig ist.
- Schlecht, weil jeder Erzeuger jede Folge kennen muss; ein neuer Verbraucher ändert viele
  Schreibpfade, und eine vergessene Stelle verliert Zustellungen still.
- Schlecht, weil die Empfänger-Bestimmung (Benachrichtigungs-Einstellungen, Abos) in der
  schreibenden Transaktion liefe und sie verlängert.

### Verarbeitung aus `NOTIFY`

- Gut, weil die Verzögerung minimal ist.
- Schlecht, weil eine Benachrichtigung ohne zuhörende Verbindung verloren geht; mindestens einmal
  ist so nicht erreichbar. `NOTIFY` bleibt Wecksignal ohne Inhalt (R-Stack).

### Cursor je Ziel

- Gut, weil jedes Ziel seine eigene Reihenfolge hält; Webhook-Abos nutzen das zusätzlich.
- Schlecht, weil jedes Ziel das ganze Log lesen und filtern müsste; bei vielen Push-Registrierungen
  vervielfacht sich die Leselast.

### Logical Decoding oder Broker

- Schlecht aus den Gründen in ADR 0009: Replikations-Slots halten WAL fest, ein zweites System
  kommt in Betrieb und Sicherung.

## More Information

- Quellen (abgerufen 2026-10-10): Q1 PostgreSQL `SELECT ... FOR UPDATE SKIP LOCKED`,
  https://www.postgresql.org/docs/18/sql-select.html [belegt]; Q2 River, transaktionales Einfügen,
  https://riverqueue.com/docs [belegt].
- Settled by M2-01: the lag per consumer (`head_seq` minus `last_seq`) is the gauge
  `bothy.event.consumer.lag`, and because `sync.event_retention` is a time window the age of the
  oldest unprocessed event is the gauge `bothy.event.consumer.pending_age`
  ([signals.md](../observability/signals.md) 4.2), both exported by every role. The alarm
  `BothyEventConsumerLag` ([alerting.md](../observability/alerting.md) 3.2, severity `page`)
  fires at `consumer.pending_alert_after`, which the start check holds strictly below
  `sync.event_retention` ([DECISIONS.md](../DECISIONS.md) D-M2-01-5 and D-M2-01-6).
- Verwandt: ADR 0009 (Ereignis-Log), ADR 0016 (Webhooks), ADR 0031 (Push), ADR 0052 (Umschlag).
