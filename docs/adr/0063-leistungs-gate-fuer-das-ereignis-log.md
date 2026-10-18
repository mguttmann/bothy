---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team A (Architektur), Team H (Qualität und Repo)
informed: Team F (Betrieb)
---

# Leistungs-Gate für das Ereignis-Log

## Context and Problem Statement

Das Ereignis-Log vergibt Nummern über eine Zähler-Zeile mit Zeilensperre (ADR 0009). Alle
Transaktionen mit dauerhaften Ereignissen serialisieren sich damit am Ende ihres Laufs; das `NOTIFY`
beim Commit verlängert diesen Abschnitt ([04-sync-protokoll.md](../spec/04-sync-protokoll.md),
Abschnitt 3.2; [02-architektur.md](../spec/02-architektur.md), Abschnitt 11.7). Das ist die bewusste
Grenze der Architektur: Reicht der Durchsatz nicht, braucht es eine neue Entscheidung (02, Abschnitt
16). Wie wird der Durchsatz so geprüft, dass ein Rückschritt auffällt, bevor er Nutzer trifft, und
dass die Zahl etwas bedeutet?

## Decision Drivers

- Keine Zahl ohne Test: Das Ziel ist ein benannter Wert mit einem Test, der ihn hält (R-AR-20).
- Messwerte müssen wiederholbar sein; Leistungsmessungen auf beliebiger Hardware sind es nicht.
- Das lokale Gate bleibt schnell und für Beitragende ohne Spezial-Hardware nutzbar (ADR 0038).
- Die Messung prüft das echte Muster mit echter Haltbarkeit, nicht eine schnellere Abkürzung.
- Ein Verfehlen löst eine Architekturentscheidung aus, keine stille Schwellenänderung.

## Considered Options

- Absolutes Ziel `perf.event_commit_min_per_s` auf `perf.reference_profile`, gemessen von
  `perf/event_commit`, geprüft mit `make perf` am Etappenabschluss, im Release und in Tickets, die
  den Schreibpfad berühren
- Dieselbe Messung in jedem `make check` und in CI
- Relativer Benchmark: Vergleich mit dem Stand vor der Änderung auf derselben Maschine
- Nur Überwachung im Betrieb (`bothy.db.event_seq.lock_wait` mit Alarm)
- Kein Gate; Kapazität geschätzt

## Decision Outcome

Gewählte Option: "Absolutes Ziel auf dem Referenzprofil mit `make perf`", weil nur eine absolute
Zahl aussagt, ob die Grenze der Architektur reicht, und nur ein festes Profil sie wiederholbar
macht. Grundlage: [02-architektur.md](../spec/02-architektur.md), Abschnitt 17, und
[PLAN.md](../PLAN.md), P-07.

1. `perf/event_commit` lässt viele parallele Schreiber realistische Transaktionen ausführen
   (Domänenänderung, mehrere Ereignisse, Zähler-Schritt zuletzt, `pg_notify`) und misst dauerhafte
   Ereignisse je Sekunde sowie die Wartezeit an der Zähler-Sperre.
2. Gemessen wird nur auf `perf.reference_profile` mit den Haltbarkeits-Einstellungen des Betriebs
   (`fsync` und `synchronous_commit` an). Grund: Ohne Haltbarkeit wäre der Commit kürzer und die
   Zahl wertlos.
3. `make perf` schreibt je Ziel Messwert, Schwelle und gemessene Umgebung und scheitert bei
   Unterschreitung. Die Umgebung bekommt einen Modus aus der vCPU-Zahl und dem Speicher von
   `perf.reference_profile`: `match` (genau so viele logische CPUs, mindestens so viel Speicher),
   `emulated` (größerer Rechner, begrenzt auf das Profil: `GOMAXPROCS` gleich der vCPU-Zahl,
   PostgreSQL-Container mit `--cpus` und `--memory` aus dem Profil) oder `below` (kleinerer
   Rechner). Nur `match` und `emulated` gelten als Beleg; ein Lauf mit `below` oder ohne die
   Haltbarkeits-Einstellungen aus Punkt 2 endet nicht grün, zeigt die Messung aber an. Grund für
   `emulated`: Ein eigener Rechner genau im Profil wäre eine Kostenfrage; Begrenzung auf das Profil
   misst dieselbe Parallelität, die eine CPU-Zahl im Profil meint.
4. `make perf` läuft am Abschluss jeder Etappe, im Release und in jedem Ticket, das ein
   Leistungsziel berührt ([PLAN.md](../PLAN.md), Abschnitt 3, Regel 6); nicht im Pre-Push.
5. Kein Ticket senkt die Schwelle, außer sein Brief nennt es ausdrücklich (PLAN, Abschnitt 3,
   Regel 6). Wird sie verfehlt, entscheidet eine neue ADR über das Muster (zum Beispiel Sequenzen
   je Bereich, 02 Abschnitt 16).

### Consequences

- Gut, weil die Kapazitätsgrenze des Designs gemessen statt geschätzt ist.
- Gut, weil das lokale Gate schnell bleibt und Beitragende kein Referenzsystem brauchen.
- Gut, weil die Messung auch beantwortet, ob River zusätzliche `NOTIFY`-Last erzeugt und `PollOnly`
  nötig ist (02, Abschnitt 21).
- Schlecht, weil ein Rückschritt zwischen zwei Etappen erst am Etappenende auffällt, wenn das Ticket
  das Leistungsziel nicht als berührt erkannt hat.
- Schlecht, weil ein Referenzsystem bereitstehen muss; Teile des Profils (zum Beispiel SSD) lassen
  sich nur eingeschränkt prüfen.
- Neutral, weil die Korrektheit des Musters ein eigener Test hält (Nebenläufigkeitstest aus
  ADR 0009); dieses Gate prüft nur den Durchsatz.

### Confirmation

- Bau-Ticket M1-16 in [PLAN.md](../PLAN.md): `make perf` misst `perf.event_commit_min_per_s` auf
  `perf.reference_profile` und scheitert bei Unterschreitung (built: `TestEventCommitThroughput`
  in `services/server/perf/event_commit` on a database from `pgtest.Durable()`; it fails when
  `fsync` or `synchronous_commit` is not `on` and below the threshold, which
  `perf/internal/report` reads from the table of 02, `TestReport_FailsBelowThreshold`).
- Bau-Ticket M1-72: `make perf` schreibt je Ziel Messwert und Schwelle; Chaos-Tests der
  Ereignis-Kette (Verlust der `LISTEN`-Verbindung, Datenbank-Neustart, SIGKILL im Commit)
  hinterlassen keine Lücke.
- Test des Werkzeugs: Die Modi `match`, `emulated` und `below` entstehen aus festen
  Beispiel-Rechnern; ein Lauf im Modus `below` oder mit `synchronous_commit = off` endet nicht grün;
  `results.json` nennt den Modus (Bau-Ticket M1-72).
- Built by M1-72: `make perf` measures every M1 target of 02 section 17 on the profile of
  point 3: `perf.message_create_p99` under `perf.sustained_messages_per_s`
  (`TestPerf_MessageCreateP99`), `perf.fanout_local_p99` (`TestPerf_FanoutLocalP99`),
  `perf.catchup_page_p99` with a fractional and a large visible set (`TestPerf_CatchupPageP99`,
  `TestPerf_CatchupPageP99LargeVisibleSet`), `perf.authorize_cached_p99`
  (`TestPerf_AuthorizeCachedP99` over `BenchmarkAuthorizeCached`), `perf.ws_connections_per_node`
  and `perf.ws_idle_conn_mem_max` (`TestPerf_WSConnectionsPerNode`), `perf.startup_ready_max` and
  `perf.idle_rss_max` (`TestPerf_StartupReadyAndIdleRSS`), and `perf.event_commit_min_per_s`
  (`TestEventCommitThroughput`). One results file `.cache/perf/results.json` holds value,
  threshold, comparison and pass per target, the profile with its mode and the durability settings;
  `perfreport` prints the table and exits non-zero on a miss (`TestExitFailsOnMissedTarget`).
- The modes come from fixture hosts in `TestProfileModes`; the container runtime of the database
  counts as well, so a large host whose Docker VM is below the profile is `below`.
  `TestProfileRejectsNonDurable` shows that a run with `fsync = off` or `synchronous_commit = off`
  does not end green although every target passes.
- Chaos tests of the event path (M1-72, `services/server/test/chaos`): `TestChaos_ListenConnectionLoss`,
  `TestChaos_DatabaseRestartDuringTail` and `TestChaos_SigkillDuringCommit` check under load that
  `seq` stays gapless, the counter equals its maximum, every acknowledged message exists once with
  one `message.created`, no idempotency key stays `in_progress`, the audit chain is valid and every
  receiver gets every event once in order.
- Exit-Kriterium jeder Etappe ([PLAN.md](../PLAN.md), Abschnitt 4, Punkt 3).

## Pros and Cons of the Options

### Absolutes Ziel auf dem Referenzprofil

- Gut, weil die Zahl eine Aussage über Kapazität ist.
- Schlecht, weil sie nur auf dem Profil gilt.

### Messung in jedem `make check`

- Gut, weil Rückschritte sofort auffielen.
- Schlecht, weil Laptops, Runner und Container verschieden schnell sind; das Gate würde zufällig rot
  oder müsste so großzügig sein, dass es nichts mehr prüft.

### Relativer Benchmark

- Gut, weil er auf jeder Maschine Rückschritte gegenüber dem Vorstand findet.
- Schlecht, weil er nicht sagt, ob der Durchsatz reicht; zwei kleine Rückschritte unter der
  Rauschgrenze summieren sich unbemerkt.
- Neutral, weil er als Ergänzung in Tickets am Schreibpfad sinnvoll bleibt.

### Nur Überwachung im Betrieb

- Gut, weil echte Last gemessen wird.
- Schlecht, weil ein Engpass erst bei Nutzern auffällt; die Kennzahl bleibt trotzdem im Dashboard
  ([16-betrieb.md](../spec/16-betrieb.md), Abschnitt 13).

### Kein Gate

- Schlecht, weil die zentrale Grenze der Architektur ungeprüft bliebe.

## More Information

- Verwandt: ADR 0004 (modularer Monolith), ADR 0009 (Ereignis-Log), ADR 0038 (lokale Gates).
- Weitere Leistungsziele des Server-Kerns (`perf.*` in 02, Abschnitt 22) laufen im selben
  `make perf` (Bau-Ticket M1-72); diese ADR begründet die Form an seinem wichtigsten Ziel.
- M1-72 measures every target of M1 in `make perf` with the same rules: thresholds only from the
  table of 02, one tag `perf` for load and chaos tests, packages one after another, durable
  databases only. The modes `match` and `emulated` are evidence; `below` prints the values as not
  proven. `services/server/perf/README.md` names each test, its parameters and how to read the
  table; the milestone-close run on the reference host is an owner operation.
