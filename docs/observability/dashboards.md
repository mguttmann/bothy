# Dashboards

Status: Entwurf M0. Sechs Grafana-Dashboards als Dateien unter
[infra/observability/grafana/dashboards/](../../infra/observability/grafana/dashboards/), geladen per
Datei-Provisioning (https://grafana.com/docs/grafana/latest/administration/provisioning/,
abgerufen 2026-10-10) [belegt].

## 1. Übersicht

| Datei | UID | Titel | Inhalt |
|---|---|---|---|
| `server-overview.json` | `bothy-server-overview` | Bothy / Server overview | REST-Erfolg und -Latenz, Prozesse je Rolle, Gateway-Verbindungen, Nachrichten nach Herkunft, Autorisierung, Rate-Limits, Fehler-Logs |
| `delivery-sync.json` | `bothy-delivery-sync` | Bothy / Delivery and sync | Zustell-Latenz, Fan-out-Verzug, Ereignisse je Typ, Warten am Sequenz-Zähler, Verbinden, Nachholen, `resync_required`, Schließ-Codes |
| `agent-runs.json` | `bothy-agent-runs` | Bothy / Agent runs | aktive Läufe, Start-Latenz, Turn-Dauer, Fehlergründe, Schritte, Tokens, Kosten je Währung (Schätzung), Verbraucher-Rückstand, Heartbeat, Ablehnungen (I2), Modus-Abweichungen (I3) |
| `push.json` | `bothy-push` | Bothy / Push | Annahme beim Anbieter, Abruf-Quote, Sendungen nach Ergebnis, Sendedauer, Auslassungen |
| `database.json` | `bothy-database` | Bothy / Database and backup | Verbindungen, Größe, Deadlocks, Commits und Rollbacks, Abfragedauer, Pool, größte Tabellen, Alter der Sicherungen, WAL-Archiv, Wiederherstellungstest, freier Platz |
| `slo-overview.json` | `bothy-slo-overview` | Bothy / SLO overview | Ziele, Erfüllung über 30 Tage, verbleibendes Fehlerbudget, Burn-Rate, aktive Alarme |

Alle Dashboards haben die Variable `env` (aus `target_info`, Label
`deployment_environment_name`); `agent-runs` zusätzlich `agent`. Datenquellen mit festen UIDs
`prometheus`, `loki`, `tempo`, `alertmanager` aus
[datasources.yaml](../../infra/observability/grafana/provisioning/datasources/datasources.yaml).

## 2. Geprüft

Am 2026-10-10 lokal [belegt]:

1. Alle PromQL-Ausdrücke der Dashboards (nach Ersetzen der Grafana-Variablen) mit
   `promtool check rules` aus Prometheus v3.15.0: gültig.
2. Grafana 13.2.3 mit dem Provisioning dieses Repositorys gestartet: vier Datenquellen und sechs
   Dashboards geladen, `provisioned = true`, keine Fehler im Log.
3. Metrik-Namen nach der Übersetzung von Prometheus (siehe [signals.md](signals.md) 1).

Nicht geprüft: Darstellung mit echten Daten. Das leistet der Portabilitätstest, sobald der Server
Telemetrie liefert.

## 3. Pflege

1. Dashboards sind Code. Der Provider setzt `allowUiUpdates: false` und `disableDeletion: true`.
2. Ändern: in einer lokalen Grafana bearbeiten, JSON exportieren ("Export for sharing externally"
   aus), Datei ersetzen, Formatierung mit `jq` normalisieren, committen.
3. Jede neue Abfrage nutzt nur Metriken aus [signals.md](signals.md); der Namens-Test
   ([17-observability.md](../spec/17-observability.md) 10) prüft es.
4. Grafonnet wird nicht verwendet: Die Bibliothek nennt sich selbst experimentell und nicht für
   den Produktionseinsatz gedacht (https://github.com/grafana/grafonnet, abgerufen 2026-10-10)
   [belegt].
5. Laufzeit-Texte (Titel, Beschreibungen) sind Englisch (R-OB-09).

## 4. Andere Backends

Die Dashboards nutzen PromQL und LogQL. Bei New Relic, Datadog, Azure Monitor oder Sentry gelten
deren eigene Ansichten; diese Datei liefert die Liste der Fragen, die jedes Backend beantworten
muss (Abschnitt 1, Spalte Inhalt). Ein Ticket je Backend kann sie dort nachbauen; das Produkt
liefert nur die Grafana-Fassung.
