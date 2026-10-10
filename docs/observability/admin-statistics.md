# Statistik in der Admin-Konsole

Status: Entwurf M0. Regeln: [17-observability.md](../spec/17-observability.md) 8. Darstellung in
der Konsole: Kapitel 01 (Team E); Visualisierungen, eigene Messung und Live-Strom:
[25-visualisierung.md](../spec/25-visualisierung.md). Rechte: `stats.read` und `cost.read.aggregate`
([05-aktionen.md](../spec/05-aktionen.md) 4.2, 4.12).

## 1. Grundsatz

Die Statistik liest eigene Tabellen in PostgreSQL, nie ein Telemetrie-Backend. Dazu gehört seit
dem Nachtrag vom 2026-10-10 auch `stats_minutes`, die Minuten-Aggregate der eigenen Messung
(Abschnitt 7). Grund: Der
Observability-Stack ist optional, gesampelt und hat kurze Fristen; die Statistik muss in jeder
Installation gleich und vollständig sein und denselben Rechten folgen wie die Daten.

## 2. Kennzahlen

| Bereich | Kennzahl | Quelle | Gruppierung |
|---|---|---|---|
| Nutzung | Nachrichten je Tag nach Herkunft (`human`, `agent`, `app`, `import`) | `messages` (ohne gelöschte) | Bereich, Kanal, Tag |
| Nutzung | neue Threads, erledigte Threads | `threads` | Bereich, Kanal, Tag |
| Nutzung | aktive Menschen (mindestens eine Nachricht) | `messages.author_principal_id` | Bereich, Woche; nur ab `privacy.stats_min_group_size` |
| Nutzung | Speicher der Dateien | `files.size_bytes` über `scope_channel_id` | Bereich |
| Läufe | Läufe nach Endzustand, Dauer (Median, p95) | `runs` | Agent, Bereich, Tag |
| Läufe | Fehlergründe | `runs.state_reason` bei `failed` | Agent, Tag |
| Werkzeuge | Werkzeugaufrufe nach Name, Anteil `failed` | `run_steps` mit `kind = 'tool_call'` | Agent, Werkzeug, Tag |
| Kosten | gültige Summe, Anteil Schätzung, nach Modell | `cost_entries` ohne Nachfolger | Bereich, Agent, Modell, Tag, Währung |
| Fehlerraten | Läufe `failed` je Läufe | `runs` | Agent, Tag |
| Fehlerraten | Webhook-Zustellungen `failed` und `dead` | `webhook_deliveries` (solange in der Frist) | App, Tag |
| Fehlerraten | Scans `error` und `infected` | `files.scan_status` | Tag |
| Fehlerraten | Push-Registrierungen deaktiviert | `push_registrations.disabled_reason` | Anbieter |

Kosten im Modus `personal` sind nur Information, nie Rechnung
([03-datenmodell.md](../spec/03-datenmodell.md) 4.9); die Konsole kennzeichnet beide Modi getrennt
und Schätzungen als "Schätzung".

Grenzen der Quellen: `run_steps` fasst ab `run.max_steps_persisted` ältere Schritte zusammen;
sehr lange Läufe zählen darum zu wenige Werkzeugaufrufe. `webhook_deliveries` liegt in der
Klasse `transient` und reicht nur so weit zurück wie deren Frist. Gelöschte Nachrichten zählen
nicht.

## 3. Datenschutz

1. Keine Kennzahl je Mensch. Weder Nachrichten noch Läufe noch Kosten werden nach Person
   gruppiert. Auslöser eines Laufs (`triggered_by`) erscheinen nicht in der Statistik.
2. Zähler über Menschen (z. B. aktive Menschen je Bereich) unter `privacy.stats_min_group_size` zeigt die
   Konsole als "weniger als `privacy.stats_min_group_size`". Grund: kleine Gruppen machen Einzelne
   erkennbar.
3. Die Kostenansicht je Thread ist keine Statistik, sondern Teil des Threads (`cost.read.thread`).
4. Export der Statistik als CSV enthält nur dieselben Aggregate.

## 4. Rechte

| Sicht | Aktion | Wer |
|---|---|---|
| ganze Instanz | `stats.read` auf `instance` | Owner |
| ein Bereich | `stats.read` auf `space` | `manage` auf dem Bereich |
| Kosten gesamt | `cost.read.aggregate` auf `instance` | Owner |
| Kosten eines Bereichs | `cost.read.aggregate` auf `space` | `manage` auf dem Bereich |

Jede Abfrage filtert auf die Kanäle, die `authz` für den Prinzipal liefert; ein Bereichsverwalter
sieht keine Zahlen aus Kanälen, die er nicht lesen darf.

## 5. Abfragen und Leistung

1. Abfragen laufen über die vorhandenen Indexe (`runs (agent_id, finished_at)`,
   `cost_entries (space_id, occurred_at)`, `messages (channel_id, created_at)`).
2. Ergebnisse gelten `stats.cache_ttl`; der Zeitraum ist höchstens `stats.max_range`.
3. Tagesaggregate: Wird eine Abfrage zu teuer (Lasttest), schreibt ein nächtlicher Job Zeilen in
   eine Tabelle `stats_daily` (Bereich, Kanal, Agent, Tag, Kennzahl, Wert) und die Konsole liest
   von dort. Die Tabelle ist noch nicht im Kern-Vertrag; sie kommt als additive Migration erst,
   wenn der Lasttest es verlangt (Integrations-Entscheid vom 2026-10-10).
4. Die Zeitzone der Tagesgrenze ist die der Instanz (`locale.default` legt sie nicht fest; Ticket
   für eine Instanz-Einstellung `stats.time_zone` bei Team E).

## 6. Bezug zur Telemetrie

Für Betriebsfragen (Latenz, 5xx, Verbindungen) verlinkt die Konsole auf `telemetry.dashboard_url`,
wenn gesetzt. Die Statistik selbst rechnet nie mit Telemetrie-Werten.

## 7. Eigene Messung und Visualisierungen

1. Werte ohne Fachtabelle (Zustell-Latenz, Wiederverbinden, Annahme beim Push-Anbieter,
   Verfügbarkeit der API) und die Raten der Live-Ansichten zählt der Server selbst und schreibt sie
   je Minute nach `stats_minutes` ([25-visualisierung.md](../spec/25-visualisierung.md), Abschnitt 3).
   Die Tabelle hat keine Spalte für Menschen; Werkzeugnamen stehen nicht darin.
2. Neue Abfragen: `getStatsLive` (Live-Rahmen), `getStatsTimeline` (Minuten und Läufe eines
   Zeitraums), `getStatsSlo` (Ampel "eigene Messung", nur Owner), `getStatsBackends`
   (Backend-Ansicht je Agent). Alle mit `stats.read` und demselben Rechtefilter wie Abschnitt 4;
   Kostenfelder nur mit `cost.read.aggregate`. Dazu das Gateway-Abo `stats.subscribe` mit dem
   kurzlebigen Ereignis `stats.live`.
3. Zeilen ohne Kanal (SLI, Gateway-Werte, Wechsel der Backend-Richtlinie) sieht nur, wer
   `stats.read` auf der Instanz hat.
4. Die Mindestgruppe aus Abschnitt 3 betrifft nur Zähler über Menschen; Live-Rahmen, Timeline und
   `stats_minutes` enthalten keine. Aktive Menschen zeigt weiter nur `getUsageStats`.
5. Lange Zeiträume, Exporte und die Tabellen aus Abschnitt 2 rechnen weiter aus den Fachtabellen;
   `stats_minutes` lebt nur `retention.stats_minutes` lang.
