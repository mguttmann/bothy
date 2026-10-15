# opscheck

Prüft jede Betriebs-Konfiguration des Repositorys mit dem gepinnten Werkzeug, das sie im
Betrieb liest, und hält abgeleitete Werte gleich ihrer Quelle (PLAN M1-04, Kapitel 17
"Tests, die diese Regeln halten"). Gate: `make ops-check`, Teil von `make gates`.

## Checks

| Check | Prüft |
|---|---|
| `tool-versions` | gepinnte Versionen von `otelcol-contrib`, `promtool`, `amtool`, `loki`, `tempo` gleich `BOTHY_<X>_VERSION` in `infra/compose/.env.example` und den Standardwerten in `infra/compose/compose.yaml` (führendes `v` zählt nicht) |
| `collector` | `otelcol-contrib validate` for the base alone, the base with each overlay and with every ordered pair of two overlays except `none`; before that, a reference to a config provider other than `env`, `file` and `yaml` (`${https:...}`, `${s3:...}`) is a finding and no `validate` runs |
| `collector-allowlist` | `bothy.*`-Schlüssel von `redaction/allowlist` gleich der Tabelle in `docs/observability/signals.md` 5.3; die Allowlist wirkt: `allow_all_keys` ist falsch, jede Trace- und Log-Pipeline mit einem Empfänger führt sie aus, kein Overlay definiert den Prozessor oder eine Pipeline der Basis neu |
| `prometheus-config` | `promtool check config` auf einer Kopie, deren `/etc/prometheus/`-Pfade in die Kopie zeigen, damit die Regeldateien mitgeprüft werden; jeder Eintrag von `rule_files` liegt unter `/etc/prometheus/` (relativ: ab `prometheus.yaml`) und trifft eine Datei, und jede Datei in `rules/` wird geladen |
| `prometheus-rules` | `promtool check rules` für jede Datei in `rules/` |
| `prometheus-rule-tests` | `promtool test rules` für jede `tests/*.test.yaml`; jeder Alarm aus `bothy-alerts.yaml` feuert in mindestens einem Test (nicht leeres `exp_alerts`) |
| `alertmanager` | `amtool check-config` and the routes of `docs/observability/alerting.md` "Stufen und Wege" with `amtool config routes test`, on a copy whose `/etc/alertmanager/` paths point into the copy, so the templates are parsed too; every `templates` entry lies under `/etc/alertmanager/` (relative: from `alertmanager.yaml`) and matches a file |
| `loki` | `loki -verify-config` |
| `tempo` | `tempo -config.verify=true` |
| `sloth` | jede `rules/slo-<n>.yaml` gleich einer frischen `sloth generate` aus `slo/<n>.yaml`, Byte für Byte; keine erzeugte Datei ohne Quelle |
| `slo-docs` | Tabelle "SLOs" in `docs/observability/slos.md` gleich den Sloth-Dateien (Zeilen, Ziel, Schwelle, Alarm) |
| `sli-bounds` | `services/server/internal/stats/domain/slibounds.gen.go` and `packages/bothy_viz/lib/src/slo/slo_catalog.g.dart` equal a fresh run of `tools/observability/sli_bounds.py` ([README](../observability/README.md)) |
| `sli-labels` | every SLO of the Sloth files has exactly one label `bothy_sli_metric` with an `sli.*` value of the `CHECK` of `stats_minutes.metric` (03), every such value belongs to exactly one SLO, and the column "Metrik der eigenen Messung" of `docs/observability/slos.md` equals the labels |
| `alert-thresholds` | Alarme der Tabelle "Weitere Alarme" in `docs/observability/alerting.md` gleich `bothy-alerts.yaml`; Schwellen gleich (`` `<name>` ``) oder echt größer (``über `<name>` ``) als der Standard des Konfigurationswerts in `docs/operations/configuration.md` |
| `tofu` | `tofu init -backend=false` and `tofu validate` for each module under `infra/tofu/modules/` (a directory with `.tf`, `.tf.json`, `.tofu` or `.tofu.json` files), all modules in one copy of `infra/tofu/` |

## Aufruf

```
make ops-check
uv run --project tools --locked python tools/opscheck/opscheck.py --only collector sloth
uv run --project tools --locked python tools/opscheck/opscheck.py --list
```

Ausgabe je Check `<check>: ok` oder `<check>: FAIL` mit den Befunden darunter; Exit-Code 1,
wenn ein Check scheitert, 2 bei falschem Aufruf. Ein Lauf führt alle Checks aus und zeigt alle
Befunde.

## Wie die Validatoren laufen

* Die Werkzeuge kommen aus `tools/bin/tools.lock.json` über `tools/bin/fetch_tools.py` (sha256
  je Plattform, Cache `.cache/tools/`). Fehlt ein Werkzeug und lässt es sich nicht laden, scheitert
  sein Check; ein `SKIP` gibt es nicht (R-QA-02).
* Jeder Check bekommt einen eigenen Ordner in einem Temp-Ordner (`tempfile.mkdtemp`), der am Ende
  gelöscht wird. Jeder Prozess läuft dort mit genau `PATH`, `HOME` und `TMPDIR` in diesem Ordner
  (`PATH` zeigt auf einen leeren Ordner), den Proxy-Variablen `HTTP_PROXY` und `HTTPS_PROXY`
  (auch klein geschrieben) und `GCE_METADATA_HOST` auf einem geschlossenen Port von `127.0.0.1`
  und den Variablen, die der Check nennt; die Umgebung der aufrufenden Shell erreicht ihn nie.
  The proxy stops only clients that honour these variables, such as the module downloads of
  `tofu init` from a registry or over HTTP: such a request fails at once and becomes a finding.
  A client with a transport of its own ignores the proxy; for the known ones see the next
  sentence, the provider rule of `collector` and the `gcs::` note of `tofu` below.
  `GCE_METADATA_HOST` braucht es, weil die GCP-Metadaten-Bibliothek den Proxy nicht nutzt; der
  Datadog-Exporter fragt sie bei der Hostnamen-Erkennung auch mit festem Hostnamen ab. Unter
  macOS hält `test_no_validator_reaches_the_network` das: Alle Checks laufen dort in einer
  Sandbox, die jeden Validator bei einer Verbindung über Loopback hinaus beendet. Nach
  `COMMAND_TIMEOUT_S` wird ein Prozess beendet und gemeldet.
  When SIGINT or SIGTERM stops a run, the running validators are killed by their process id
  and the work directory is removed as well; the exit status is 143 after SIGTERM, and after
  SIGINT the script ends by SIGINT, as Python does (a shell shows 130).
* `collector`: The work copy of `infra/otel/` reads its secrets from placeholder files
  (`SECRET_PLACEHOLDER`, contains `EXAMPLE`, at least 32 bytes for the pseudonym key) instead of
  `/run/bothy/secrets/`. The check sets only the variables of `COLLECTOR_ENV` in `opscheck.py`,
  each to an `example.com` address or a zero UUID: the variables the overlays use without a
  default (Azure Monitor, Grafana, Sentry). A new overlay with an `${env:NAME}` reference without
  a default (instead of `${env:NAME:-<default>}`) needs `NAME` in `COLLECTOR_ENV`; otherwise it
  expands to an empty value, and for an endpoint `validate` fails (with the `otlphttp` exporter:
  "at least one endpoint must be specified").
  In der Kopie von
  `exporters/datadog.yaml` setzt der Check `exporters.datadog.hostname: validate.example.com`.
  Ohne Hostnamen wartet `validate` auf die Hostnamen-Erkennung, die Metadaten-Endpunkte von Clouds
  abfragt (mit 0.162.0 am 2026-10-10 rund 25 Sekunden gemessen). Die Abfrage der GCP-Metadaten,
  die der Exporter auch mit Hostnamen startet, hält `GCE_METADATA_HOST` auf (siehe oben). Das
  Overlay im Repository bleibt unverändert, der Betrieb behält die automatische Erkennung.
  Before any `validate`, a reference to a config provider other than `env`, `file` and `yaml`
  (`${https:...}`, `${s3:...}`, `${googlesecretmanager:...}`) in a collector file is a finding,
  also inside another reference (`${env:NAME:-${https:...}}`): the collector resolves such a
  reference while it validates, and most of these providers fetch over the network with clients
  that may bypass the proxy.
* `tofu`: Die CLI-Konfiguration erlaubt als einzige Provider-Quelle einen leeren lokalen Mirror,
  und Modulquellen aus Registry oder HTTP scheitern am geschlossenen Proxy (Git-Quellen am leeren
  `PATH`). Ein Modul mit Providern oder entfernten Modulen scheitert darum, bis es offline
  initialisiert (siehe `docs/quality/gates.md`, "Was die Gates nicht abdecken").
  A Google Cloud Storage module source (`gcs::`) uses a client of its own that the proxy does
  not stop: in the red team's run on 2026-10-11 it waited until `COMMAND_TIMEOUT_S` and was then
  reported. An `s3::` source failed at the closed proxy in a review run on 2026-10-11.

## Einen Check ergänzen

1. Funktion `check_<name>(ctx: Context) -> list[str]` in `opscheck.py` mit Docstring; jede
   Zeichenkette der Liste ist ein Befund. Ein Validator läuft nur über `run_command`, ein Pfad
   relativ zur Wurzel steht als Konstante oben im Modul.
2. Eintrag in `CHECKS` (Reihenfolge wie die Tabelle oben) und eine Zeile in dieser Tabelle.
3. A new tool first needs its entry in `tools/bin/tools.lock.json`, written by hand: keys
   `version`, `repo`, `tag`, `license`, `binary` and `assets` with `asset`, `member` and a
   placeholder `sha256` of 64 zeros per platform (the lock file must load). `make tools-lock`
   only re-pins an existing entry; for an unknown name it stops with `unknown tool: <name>`
   (exit 2). Then `make tools-lock TOOL=<name> VERSION=<version>` writes the checksums of the
   GitHub release. The tool also gets its row in `docs/quality/toolchain.md` and a test that
   runs `ops-check` with the real tool.
4. Tests in `tests/test_opscheck.py`: Die Fixture `tree` kopiert die gelesenen Teilbäume nach
   `tmp_path`; ein Test bricht genau eine Datei und erwartet den Befund des Checks. Liest der
   Check einen neuen Teilbaum, kommt er in `SUBTREES`.
5. Eine neue Collector-Konfiguration (etwa ein Test-Overlay) liest `check_collector` nur, wenn
   sie unter `infra/otel/exporters/` liegt oder der Check ihren Ort ausdrücklich nennt.
