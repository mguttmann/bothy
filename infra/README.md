# infra

Alles, was eine Installation des Produkts beschreibt, ohne Werte einer konkreten Instanz.
Instanz-Werte (Hostnamen, Konten, Secrets) gehören in das private Repository des
Betreibers (ADR 0002).

| Ordner | Inhalt | Kapitel |
|---|---|---|
| `compose/` | Installation mit Docker Compose in einem Befehl; `smoke.sh` für den Portabilitätstest | [16 Betrieb](../docs/spec/16-betrieb.md) |
| `otel/` | OpenTelemetry Collector mit Exportern je Backend | [17 Observability](../docs/spec/17-observability.md) |
| `observability/` | Dashboards, SLO-Regeln, Alarm-Routen | [17 Observability](../docs/spec/17-observability.md) |
| `tofu/` | OpenTofu-Module je Ziel (später) | [16 Betrieb](../docs/spec/16-betrieb.md) |
| `helm/` | Helm-Chart (optional, später) | [16 Betrieb](../docs/spec/16-betrieb.md) |

Gates: Trivy prüft Dockerfiles, Kubernetes-, Helm- und Terraform-Dateien auf
Fehlkonfiguration, sobald es sie gibt (`make trivy`). Der Portabilitätstest
(`.github/workflows/portability.yml`) startet den ganzen Stack aus den veröffentlichten
Images auf einer frischen Linux-Maschine.
