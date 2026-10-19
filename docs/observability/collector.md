# OpenTelemetry Collector

Status: Entwurf M0. Erklärt [infra/otel/collector.yaml](../../infra/otel/collector.yaml) und die
Overlays unter [infra/otel/exporters/](../../infra/otel/exporters/). Entscheidung: ADR 0034.

![Telemetrie-Pipeline](diagrams/telemetry-pipeline.svg)

## 1. Version und Distribution

`otel/opentelemetry-collector-contrib`, Version gepinnt in `BOTHY_OTELCOL_VERSION`. Geprüft wurde
die Konfiguration mit `otelcol-contrib` 0.162.0 (`otelcol-contrib validate`, Basis allein, mit
jedem Overlay und mit drei Kombinationen aus zwei Overlays). In dieser Version heißen die
Komponenten `otlp_grpc`, `otlp_http`, `cumulative_to_delta`, `azure_monitor` (Ausgabe von
`otelcol-contrib components`) [belegt].

## 2. Aufbau der Basis

| Teil | Komponente | Zweck |
|---|---|---|
| Empfang | `otlp` (gRPC `:4317`, HTTP `:4318`) | Server, `bothy-backupd`, Adapter |
| Empfang | `postgresql` | Kennzahlen der Datenbank mit Rolle `bothy_monitor`; Deadlocks zusätzlich eingeschaltet |
| Schutz | `memory_limiter` | lehnt bei Speicherdruck ab, statt abzustürzen; der Server verwirft dann und zählt |
| Kennzeichnung | `resource/instance` | setzt `deployment.environment.name`, falls die Quelle es nicht setzt |
| Datenschutz | `transform/scrub` | löscht `url.query`, `url.full`, Client-Adressen, Inhalts-Attribute der GenAI-Konventionen, `exception.message` |
| Datenschutz | `redaction/allowlist` | Spans und Logs: nur Schlüssel aus [signals.md](signals.md) 5.3 und den Konventionen; HMAC für verdächtige Werte |
| Datenschutz | `transform/scrub_metrics` | Metriken: Personen-Attribute fremder Quellen löschen |
| Sampling | `tail_sampling` | behält alle Traces mit Fehler, alle langsamer als `BOTHY_TRACE_SLOW_THRESHOLD_MS`, vom Rest `BOTHY_TRACE_SAMPLE_PERCENT` |
| Bündeln | `batch` | weniger Exporte |
| Verteilung | `forward/traces`, `forward/metrics`, `forward/logs` | Ende der Eingangs-Pipelines; Overlays hängen sich an |
| Standardziel | `nop` | ohne Overlay werden Daten verworfen |
| Eigenmetriken | Prometheus-Leser auf `:8888` | Prometheus scrapt sie (Alarm `BothyTelemetryExportFailing`) |

Metriken laufen nicht durch die Allowlist: Bothy-eigene Attribute sind im Code begrenzt
([signals.md](signals.md) 2), und der Prozessor ist für Metriken nur alpha. Für Metriken fremder
Quellen löscht `transform/scrub_metrics` Personen-Attribute (`user.*`, `enduser.*`,
`session.id`, `organization.id`, `client.address`) an Resource und Datenpunkt.

## 3. Overlays

Ein Overlay ist eine zweite `--config`-Datei. Der Collector führt Maps zusammen und ersetzt Listen.
Darum fügt jedes Overlay eigene, benannte Pipelines hinzu (z. B. `traces/newrelic`) und ändert
keine Liste der Basis. Ausnahme: Ein Overlay mit Erweiterung muss `service.extensions` vollständig
nennen; das trifft nur `azure-monitor`. Zwei Overlays mit Erweiterungen gleichzeitig gehen darum
nicht; alle anderen Paare gehen.

| Overlay | Ziel | Signale | Besonderheit |
|---|---|---|---|
| `none` | keins | keine | Standard |
| `local` | Prometheus, Loki, Tempo im Compose-Profil `observability` | alle | Metriken kumulativ an den OTLP-Empfänger von Prometheus |
| `newrelic` | New Relic OTLP | alle | Delta-Temporalität, Attribute gekürzt |
| `grafana` | Grafana Cloud OTLP-Gateway | alle | ganze `Authorization`-Kopfzeile aus einer Datei |
| `datadog` | Datadog | alle | Datadog-Connector für APM-Statistiken |
| `azure-monitor` | Azure Monitor (native OTLP-Aufnahme) | alle | Entra-Anmeldung, Delta-Temporalität |
| `sentry` | Sentry OTLP | Traces, Logs | keine Metriken |

Belege und Grenzen je Backend: [backends.md](backends.md).

Compose wählt bis zu zwei Overlays mit `BOTHY_OTEL_EXPORTER` und `BOTHY_OTEL_EXPORTER_2`.

The test overlay `file` (`infra/compose/test/otel/file.yaml`) exists only for the Compose smoke
test (M1-73): it writes the three signals as OTLP JSON lines into a volume of the test overlay,
shortens `tail_sampling.decision_wait` and `batch.timeout` to one second, and is never part of an
installation or of the release bundle. `tools/opscheck` validates it with the base alone.

## 4. Geheimnisse

Alle Schlüssel liest der Collector mit `${file:/run/bothy/secrets/<name>}` aus seinem
Unterverzeichnis des Volumes `secrets`. Umgebungsvariablen tragen nur Endpunkte und IDs.

## 5. Grenzen und Betrieb

1. Tail-Sampling braucht alle Spans eines Traces im selben Collector. Mit mehreren Collectors
   (Cloud, mehrere Hosts) braucht es davor einen Lastverteiler nach Trace-ID (`load_balancing`
   exporter) [abgeleitet].
2. APM-Statistiken bei Datadog beruhen auf der Stichprobe, weil das Sampling vor dem Overlay
   läuft.
3. Fällt der Collector aus, verwirft der Server nach vollem Puffer; der Nachrichtenpfad bleibt
   unberührt. Der Alarm `BothyServerTelemetryMissing` meldet das.
4. Adapter auf demselben Host senden an `127.0.0.1:BOTHY_OTLP_PUBLISH_PORT`; Adapter auf anderen
   Hosts betreiben einen eigenen Collector mit denselben Dateien (Basis plus Overlay).

## 6. Prüfen

`make ops-check` prüft die Konfiguration im Gate
([tools/opscheck](../../tools/opscheck/README.md)) mit `otelcol-contrib` in der Version des Images
(Check `tool-versions` hält beide gleich):

1. Check `collector`: `otelcol-contrib validate` für die Basis allein, die Basis mit jedem Overlay
   und die Basis mit jedem geordneten Paar zweier Overlays außer `none`. Beide Reihenfolgen zählen,
   weil Compose `BOTHY_OTEL_EXPORTER` vor `BOTHY_OTEL_EXPORTER_2` setzt und Listen beim
   Zusammenführen ersetzt werden.
2. Check `collector-allowlist`: Die `bothy.*`-Schlüssel von `redaction/allowlist` sind gleich der
   Tabelle in [signals.md](signals.md) 5.3, und die Allowlist wirkt: `allow_all_keys` ist falsch,
   jede Trace- und Log-Pipeline mit einem Empfänger führt sie aus, und kein Overlay definiert den
   Prozessor oder eine Pipeline der Basis neu.

Before any `validate`, a reference to a config provider other than `env`, `file` and `yaml`
(`${https:...}`, `${s3:...}`), also one inside another reference, is a finding: the collector
resolves it while it validates, and most of these providers fetch over the network with clients
that may bypass the closed proxy of the validators. The check runs on a work copy: instead of
`/run/bothy/secrets` it reads placeholder files (`telemetry_pseudonym_key` needs at least 32
bytes), and the `datadog` exporter gets a fixed host name, so `validate` does not wait for host
name detection. It sets only the variables of `COLLECTOR_ENV` in `tools/opscheck/opscheck.py`,
the ones the overlays use without a default (Azure Monitor, Grafana and Sentry endpoints point at
`example.com`, the Azure IDs are zero UUIDs). A new overlay that reads a variable without a
default (`${env:NAME}` instead of `${env:NAME:-<default>}`) needs `NAME` in `COLLECTOR_ENV`;
otherwise the value is empty and, for an endpoint, `validate` fails (with the `otlphttp`
exporter: "at least one endpoint must be specified").
Die Metadaten-Abfrage für GCP, die der Exporter trotzdem
startet, zeigt über `GCE_METADATA_HOST` auf einen geschlossenen Port von `127.0.0.1`, wie der
Proxy aller Validatoren ([tools/opscheck](../../tools/opscheck/README.md)). Nur den Collector
prüfen:

```
uv run --project tools --locked python tools/opscheck/opscheck.py --only collector collector-allowlist
```
