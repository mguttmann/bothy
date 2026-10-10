# Telemetrie-Backends

Status: Entwurf M0. Jedes Backend ist ein Overlay des Collectors ([collector.md](collector.md));
ein Wechsel ist Konfiguration, kein Code. Alle Quellen am 2026-10-10 abgerufen.

## 1. Übersicht

| Backend | Overlay | Signale | Anmeldung | Temporalität | Status der Komponente | Offene Punkte |
|---|---|---|---|---|---|---|
| Grafana-Stack lokal | `local` | alle | keine (internes Netz) | kumulativ | `otlp_http`, `otlp_grpc` (Kern) | keine |
| Grafana Cloud | `grafana` | alle | Basic-Auth-Kopfzeile | kumulativ | `otlp_http` | genaues Format der Kopfzeile und gRPC nicht belegt |
| New Relic | `newrelic` | alle | Kopfzeile `api-key` | Delta | `otlp_http` | keine |
| Datadog | `datadog` | alle | API-Key | intern umgerechnet | Exporter beta | Statistiken nur aus der Stichprobe |
| Azure Monitor | `azure-monitor` | alle | Microsoft Entra (Service Principal oder Managed Identity) | Delta | `azure_auth` beta | GA oder Preview; exponentielle Histogramme |
| Sentry | `sentry` | Traces, Logs | Kopfzeile `x-sentry-auth` | keine Metriken | OTLP "open beta" | Status der OTLP-Aufnahme |

## 2. Grafana-Stack lokal

Compose-Profil `observability` mit Prometheus, Loki, Tempo, Grafana und Alertmanager.

| Teil | Annahme | Beleg | Konfiguration |
|---|---|---|---|
| Prometheus | OTLP-Empfänger mit `--web.enable-otlp-receiver`, Pfad `/api/v1/otlp/v1/metrics`; Delta nur experimentell | https://prometheus.io/docs/guides/opentelemetry/ [belegt] | [prometheus.yaml](../../infra/observability/prometheus/prometheus.yaml), mit `promtool check config` geprüft |
| Loki | `otlphttp`-Exporter an `http://<loki>/otlp`; `allow_structured_metadata: true` | https://grafana.com/docs/loki/latest/send-data/otel/ [belegt] | [loki.yaml](../../infra/observability/loki/loki.yaml), mit `loki -verify-config` (3.7.8) geprüft |
| Tempo | OTLP gRPC und HTTP; Tempo 3 ohne Block `compactor:` | Beispiel und Manifest im Tempo-Repository, Tag v3.1.0 [belegt] | [tempo.yaml](../../infra/observability/tempo/tempo.yaml), mit `tempo -config.verify=true` (3.1.0) geprüft |
| Grafana | Datei-Provisioning | https://grafana.com/docs/grafana/latest/administration/provisioning/ [belegt] | Provisioning mit Grafana 13.2.3 geladen |

Das Admin-Passwort setzt Compose über die Grafana-Auflösung `$__file{...}`. Die Variante
`GF_SECURITY_ADMIN_PASSWORD__FILE` wirkte beim lokalen Test mit dem Grafana-Binary 13.2.3 nicht
(Anmeldung mit `admin:admin` ging weiter) [belegt]; ob das Docker-Image sie über sein Startskript
auflöst, ist nicht geprüft [vermutet]. `$__file{}` wirkt in beiden Fällen.

Der lokale Stack ist für eine Instanz gedacht. Er hat keine Hochverfügbarkeit und keine
Mandantentrennung.

## 3. Grafana Cloud

Ein OTLP-Gateway `https://otlp-gateway-<REGION>.grafana.net/otlp` für alle drei Signale;
Anmeldung per Basic-Auth aus der Stack-Kachel
(https://grafana.com/docs/grafana-cloud/send-data/otlp/send-data-otlp/) [belegt]. Das Overlay liest
den ganzen Kopfzeilenwert `Basic <base64(instanz-id:token)>` aus einer Datei, damit es ohne
Erweiterung auskommt. Die Dashboards dieses Repositorys laufen dort, wenn die Datenquelle die UID
`prometheus` bekommt oder die UID beim Import ersetzt wird.

## 4. New Relic

OTLP-Endpunkte US `https://otlp.nr-data.net` und EU `https://otlp.eu01.nr-data.net`, Kopfzeile
`api-key` mit dem License Key, http/protobuf und Delta-Temporalität empfohlen; Grenzen u. a.
String-Attribute bis 4095 Zeichen und Nutzlast unter 1 MB
(https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-otlp/) [belegt]. Einen
eigenen New-Relic-Exporter gibt es im contrib-Repository nicht; der Weg ist OTLP [belegt]. Das
Overlay rechnet mit `cumulative_to_delta` um und kürzt Attribute.

## 5. Datadog

Der Datadog-Exporter (contrib, beta für Traces, Metriken, Logs) berechnet APM-Statistiken nicht
mehr selbst; dafür ist der Datadog-Connector vorgesehen. Gegen "413 Request Entity Too Large" hilft
das Bündeln im `sending_queue` des Exporters
(https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/datadogexporter)
[belegt]. Datadog nennt außerdem den eigenen DDOT-Collector als empfohlenen Weg
(https://docs.datadoghq.com/opentelemetry/setup/) [belegt]; das Produkt bleibt beim
Standard-Collector, damit alle Backends dieselbe Basis mit denselben Datenschutz-Filtern haben.

## 6. Azure Monitor

Native OTLP-Aufnahme über Collector, Azure Monitor Agent oder AKS-Add-on; der Collector braucht
die Erweiterung `azure_auth`, Anmeldung nur über Microsoft Entra mit der Rolle "Monitoring Metrics
Publisher" auf der Data Collection Rule; Endpunkte hängen an DCE und DCR
(https://learn.microsoft.com/en-us/azure/azure-monitor/containers/opentelemetry-protocol-ingestion)
[belegt]. Application Insights erwartet Delta-Temporalität und exponentielle Histogramme [belegt].

Folgen:

1. Das Overlay rechnet Summen in Delta um. Die expliziten Histogramme des Servers bleiben
   explizit; ob Azure sie so darstellt, ist offen [vermutet]. Exponentielle Histogramme würden die
   SLO-Regeln für Prometheus brechen (dort sind Schwellen Bucket-Grenzen). Ticket: Prüfung gegen
   eine echte Azure-Umgebung.
2. Ob die native Aufnahme allgemein verfügbar oder Vorschau ist, widerspricht sich in den Quellen
   [belegt: widersprüchlich]. Der Owner entscheidet vor dem Einsatz.
3. Auf Azure-VMs ist `managed_identity` statt Service Principal der bessere Weg (kein Geheimnis);
   das Overlay zeigt den Service Principal, weil er überall geht.
4. Die Erweiterung `azure_auth` hatte in den Versionen v0.124.0 bis v0.150.0 eine Lücke bei
   eingehender Prüfung; die ausgehende Nutzung war nicht betroffen
   (https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/azureauthextension)
   [belegt]. Das Overlay nutzt sie nur ausgehend, und die gepinnte Version liegt darüber.

## 7. Sentry

Sentry nimmt OTLP-Traces und -Logs an, keine Metriken, Status "open beta"; Endpunkt je Signal und
Kopfzeile `x-sentry-auth: sentry sentry_key=<public-key>` (https://docs.sentry.io/concepts/otlp/)
[belegt]. Der contrib-Exporter `sentry` ist alpha [belegt]; das Overlay nutzt OTLP.

Für App-Abstürze ist Sentry das einzige Ziel, das alle Plattformen der Apps abdeckt
(`sentry_flutter`: Android, iOS, Linux, macOS, Web, Windows;
https://docs.sentry.io/platforms/dart/guides/flutter/) [belegt]. Die Apps senden direkt, nicht
über den Collector ([privacy.md](privacy.md) 7).

## 8. Telemetrie der Agenten-Laufzeiten

Claude Code kann eigene OpenTelemetry-Daten senden (Metriken, Ereignisse, Traces als Beta); Inhalte
sind standardmäßig aus; die Variablen müssen in der Prozessumgebung stehen
(https://code.claude.com/docs/en/monitoring-usage) [belegt]. Ein Adapter darf sie an den Collector
der Instanz richten. Dann gilt: nur Metadaten, `OTEL_LOG_USER_PROMPTS`,
`OTEL_LOG_ASSISTANT_RESPONSES`, `OTEL_LOG_TOOL_DETAILS`, `OTEL_LOG_TOOL_CONTENT` und
`OTEL_LOG_RAW_API_BODIES` bleiben aus. Claude Code setzt standardmäßig Attribute wie `user.email`,
`user.account_uuid`, `organization.id` und `session.id` [belegt, gleiche Quelle]. Spans und Logs
filtert die Allowlist; für Metriken löscht `transform/scrub_metrics` diese Schlüssel an Resource
und Datenpunkt (lokal geprüft am 2026-10-10 mit otelcol-contrib 0.162.0) [belegt]. Die
`claude_code.*`-Namen stehen nicht im Katalog; wer sie auswerten will, ergänzt sie dort zuerst
(Kapitel 09, Team D).
