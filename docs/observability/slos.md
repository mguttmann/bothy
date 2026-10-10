# Service Level Objectives

Status: Entwurf M0. Entscheidung: ADR 0035. Quelle aller Zahlen sind die Sloth-Dateien unter
[infra/observability/slo/](../../infra/observability/slo/); die Tabelle unten gibt sie wieder.
Ein Check vergleicht Tabelle und Dateien (Bau-Ticket "SLO-Docs-Check").

## 1. Grundsätze

1. Ein SLO misst, was Menschen spüren: Antworten der API, Ankunft von Nachrichten, Wiederverbinden,
   Push, Start und Dauer von Agenten-Läufen.
2. Ein SLI ist ein Verhältnis guter zu allen Ereignissen aus Histogrammen und Zählern
   ([signals.md](signals.md)). Schwellen sind Bucket-Grenzen.
3. Zeitraum: 30 Tage, rollierend (Standard von Sloth).
4. Fehlerbudget = 1 minus Ziel. Ein verbrauchtes Budget heißt: Zuverlässigkeit vor neuen Funktionen,
   bis das Budget sich erholt (Regel für die Etappenplanung).
5. Alle Ziele sind Startwerte. Nach 30 Tagen Messung im Betrieb prüft der Owner sie; eine Änderung
   ist eine Änderung der Sloth-Datei mit Begründung im Commit.

## 2. SLOs

| SLO (`service/name`) | SLI: gut | SLI: alle | Schwelle | Ziel | Alarm | Metrik der eigenen Messung |
|---|---|---|---|---|---|---|
| `bothy-api/requests-availability` | REST-Anfragen ohne 5xx | REST-Anfragen ohne `/api/v1/gateway` | keine | 99.9 % | Page, Ticket | `sli.api_availability` |
| `bothy-sync/delivery-latency` | Live-Zustellungen dauerhafter Ereignisse bis Schwelle | alle Live-Zustellungen dauerhafter Ereignisse | 0.5 s | 99.5 % | Page, Ticket | `sli.delivery_latency` |
| `bothy-sync/reconnect` | Verbindungen mit `ok` bis Schwelle | Verbindungen ohne `client_error` | 2.5 s | 99.0 % | Page, Ticket | `sli.reconnect` |
| `bothy-push/provider-acceptance` | Sendungen `accepted` | Sendungen ohne `rejected_token` | keine | 99.5 % | Page, Ticket | `sli.push_acceptance` |
| `bothy-agents/run-start` | Läufe `started` bis Schwelle | Läufe ohne `cancelled` | 7.5 s | 99.0 % | Page, Ticket | `sli.run_start` |
| `bothy-agents/turn-duration` | abgeschlossene Läufe bis Schwelle | abgeschlossene Läufe | 600 s | 95.0 % | nur Ticket | `sli.turn_duration` |

Die Spalte "Metrik der eigenen Messung" gibt das Label `bothy_sli_metric` des SLO in der
Sloth-Datei wieder; es ordnet das SLO seiner Metrik in `stats_minutes` zu
([25-visualisierung.md](../spec/25-visualisierung.md), Abschnitt 3.3). Die Labels setzt das
Bau-Ticket PLAN M6-11; bis dahin ist diese Spalte die Vorgabe dafür.

Begründungen:

1. Verfügbarkeit ohne Gateway-Upgrade: Die Dauer eines Upgrades ist die Verbindungsdauer, kein
   Antwortverhalten. Das Gateway misst `reconnect`.
2. Zustellung nur live: Nachholen nach einer Pause misst `bothy.gateway.catchup.duration`; dessen
   Dauer hängt am Rückstand des Clients, nicht am Server.
3. Push misst die Annahme beim Anbieter. Ob ein Gerät die Meldung zeigt, sieht der Server nicht;
   die Abruf-Quote (`bothy.push.fetch`) ist ein Hinweis, kein SLI.
4. `run-start` schließt Adapter und Netz ein: Ist ein Adapter offline, verbrennt das Budget. Das
   ist gewollt; der Betreiber betreibt auch den Adapter.
5. `turn-duration` hängt an Modell und Aufgabe. Ein Page-Alarm wäre meist nicht vom Betreiber
   behebbar; darum nur Ticket.

## 3. Burn-Rate-Alarme

Sloth erzeugt Mehrfenster-Alarme (https://github.com/slok/sloth, abgerufen 2026-10-10) [belegt],
nach dem Verfahren des Google SRE Workbook (https://sre.google/workbook/alerting-on-slos/,
abgerufen 2026-10-10) [belegt]. In den erzeugten Regeln (Sloth v0.16.0) stehen:

| Stufe | Kurzes Fenster | Langes Fenster | Burn-Rate |
|---|---|---|---|
| page | 5m | 1h | 14.4 |
| page | 30m | 6h | 6 |
| ticket | 2h | 1d | 3 |
| ticket | 6h | 3d | 1 |

Ein Alarm feuert nur, wenn beide Fenster über der Schwelle liegen.

## 4. Erzeugen und prüfen

```
sloth generate -i infra/observability/slo/api.yaml -o infra/observability/prometheus/rules/slo-api.yaml
promtool check rules infra/observability/prometheus/rules/*.yaml
```

Die erzeugten Dateien sind eingecheckt und tragen den Kopf "DO NOT EDIT". Ein Check erzeugt sie
neu und vergleicht (kein Unterschied erlaubt). Am 2026-10-10 lokal mit Sloth v0.16.0 erzeugt und
mit `promtool` aus Prometheus v3.15.0 geprüft: gültig [belegt].

## 5. Sichtbarkeit

Das Dashboard `bothy-slo-overview` zeigt Ziel, Erfüllung, Restbudget und Burn-Rate aus den
Recording-Regeln `slo:objective:ratio`, `slo:sli_error:ratio_rate30d`,
`slo:period_error_budget_remaining:ratio` und `slo:current_burn_rate:ratio`.

## 6. Eigene Messung in den Apps

Die SLO-Ampel der Apps und der Konsole rechnet nicht mit Prometheus, sondern mit der eigenen
Messung des Servers (`stats_minutes`, [25-visualisierung.md](../spec/25-visualisierung.md),
Abschnitt 3 und 6.4). Ziele, Schwellen und Zeitraum erzeugt ein Generator aus denselben
Sloth-Dateien; ein Check hält sie gleich. Die Ampel trägt das Etikett "eigene Messung" und
alarmiert nicht; Alarme bleiben bei den Burn-Rate-Regeln aus Abschnitt 3.
