---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung, Team F (Betrieb)
consulted: Team A (Architektur), Team D (Agenten-Plattform)
informed: alle Teams
---

# SLOs mit Fehlerbudgets und Burn-Rate-Alarmen

## Context and Problem Statement

Betreiber sollen merken, wenn Menschen den Dienst als gestört erleben, ohne von Einzelspitzen
geweckt zu werden. Alarme auf feste Schwellen (z. B. "5xx über X") sind entweder zu laut oder zu
spät. Wie werden Ziele, Messung und Alarme festgelegt und als Code gepflegt?

## Decision Drivers

* Alarme spiegeln, was Menschen spüren (Zustellung, Wiederverbinden, Push, Läufe)
* wenige, verlässliche Page-Alarme; Rest als Ticket
* Ziele und Regeln als Dateien im Repository, prüfbar im Gate
* läuft mit dem lokalen Prometheus ohne Kubernetes

## Considered Options

* SLO-Spezifikationen für Sloth, erzeugte Prometheus-Regeln eingecheckt
* Pyrra (Operator oder Dateisystem-Modus mit eigener Oberfläche)
* Handgeschriebene Prometheus-Regeln für Mehrfenster-Burn-Rates
* Schwellen-Alarme ohne SLO

## Decision Outcome

Chosen option: "SLO-Spezifikationen für Sloth", weil Sloth aus einfachen Dateien Recording-Regeln,
Metadaten und Mehrfenster-Alarme (Page und Ticket) erzeugt, ohne laufenden Dienst
(https://github.com/slok/sloth, abgerufen 2026-10-10).

1. Sechs SLOs in `infra/observability/slo/` ([slos.md](../observability/slos.md)); Zeitraum 30 Tage.
2. Burn-Raten nach dem Google SRE Workbook (Page 14,4 und 6, Ticket 3 und 1;
   https://sre.google/workbook/alerting-on-slos/, abgerufen 2026-10-10).
3. Erzeugte Regeln liegen in `infra/observability/prometheus/rules/slo-*.yaml` und werden nie von
   Hand geändert.
4. `turn-duration` nur als Ticket, weil Modell und Aufgabe die Dauer bestimmen.
5. Schwellen sind Bucket-Grenzen der Histogramme.

### Consequences

* Gut, weil eine Zahländerung eine Zeile in einer Datei ist und der Generator den Rest macht.
* Gut, weil Ticket-Alarme langsame Verschlechterung zeigen, bevor sie Page-Alarme werden.
* Schlecht, weil Sloth ein weiteres Werkzeug in der Kette ist (Version gepinnt).
* Schlecht, weil die Ziele Startwerte sind; vor 30 Tagen Messung sind sie geraten.
* Neutral, weil andere Backends die SLOs nicht automatisch übernehmen; die Dateien beschreiben sie
  so, dass ein Backend-Ticket sie nachbauen kann.

### Confirmation

* Check im Gate: Regeln neu erzeugen und mit den eingecheckten vergleichen.
* `promtool check rules` im Gate (am 2026-10-10 lokal grün mit Sloth v0.16.0 und Prometheus v3.15.0).
* SLO-Docs-Check: Tabelle in [slos.md](../observability/slos.md) gleich den Dateien; Schwellen
  sind Bucket-Grenzen aus [signals.md](../observability/signals.md).

## Pros and Cons of the Options

### Sloth

* Gut, weil Dateien im Format `prometheus/v1` einfach zu lesen sind.
* Gut, weil Page- und Ticket-Alarme mit Mehrfenster-Logik fertig erzeugt werden.
* Schlecht, weil eine eigene Oberfläche fehlt (Dashboard aus den Recording-Regeln nötig).

### Pyrra

* Gut, weil eine Oberfläche für Fehlerbudget und Burn-Rate dabei ist.
* Schlecht, weil ein weiterer laufender Dienst nötig ist.
* Neutral, weil die erzeugten Alarme gleichwertig sind.

### Handgeschriebene Regeln

* Gut, weil kein Werkzeug nötig ist.
* Schlecht, weil die Mehrfenster-Logik je SLO fehleranfällig von Hand gepflegt würde.

### Schwellen-Alarme ohne SLO

* Gut, weil sofort verständlich.
* Schlecht, weil sie Spitzen und schleichende Verschlechterung schlecht trennen.
* Schlecht, weil kein Fehlerbudget für die Planung entsteht.

## More Information

* Alarme und Wege: [alerting.md](../observability/alerting.md).
