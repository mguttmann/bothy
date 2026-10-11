# opscheck

Prüft jede Betriebs-Konfiguration des Repositorys mit dem gepinnten Werkzeug, das sie im
Betrieb liest, und hält abgeleitete Werte gleich ihrer Quelle (PLAN M1-04, Kapitel 17
"Tests, die diese Regeln halten"). Gate: `make ops-check`, Teil von `make gates`.

## Checks

| Check | Prüft |
|---|---|
| `tool-versions` | gepinnte Versionen von `otelcol-contrib`, `promtool`, `amtool`, `loki`, `tempo` gleich `BOTHY_<X>_VERSION` in `infra/compose/.env.example` und den Standardwerten in `infra/compose/compose.yaml` (führendes `v` zählt nicht) |
| `collector` | `otelcol-contrib validate` für die Basis allein, die Basis mit jedem Overlay und mit jedem geordneten Paar zweier Overlays außer `none` |
| `collector-allowlist` | `bothy.*`-Schlüssel von `redaction/allowlist` gleich der Tabelle in `docs/observability/signals.md` 5.3; die Allowlist wirkt: `allow_all_keys` ist falsch, jede Trace- und Log-Pipeline mit einem Empfänger führt sie aus, kein Overlay definiert den Prozessor oder eine Pipeline der Basis neu |
| `prometheus-config` | `promtool check config` auf einer Kopie, deren `/etc/prometheus/`-Pfade in die Kopie zeigen, damit die Regeldateien mitgeprüft werden; jeder Eintrag von `rule_files` liegt unter `/etc/prometheus/` (relativ: ab `prometheus.yaml`) und trifft eine Datei, und jede Datei in `rules/` wird geladen |
| `prometheus-rules` | `promtool check rules` für jede Datei in `rules/` |
| `prometheus-rule-tests` | `promtool test rules` für jede `tests/*.test.yaml`; jeder Alarm aus `bothy-alerts.yaml` feuert in mindestens einem Test (nicht leeres `exp_alerts`) |
| `alertmanager` | `amtool check-config` und die Wege aus `docs/observability/alerting.md` "Stufen und Wege" mit `amtool config routes test` |
| `loki` | `loki -verify-config` |
| `tempo` | `tempo -config.verify=true` |
| `sloth` | jede `rules/slo-<n>.yaml` gleich einer frischen `sloth generate` aus `slo/<n>.yaml`, Byte für Byte; keine erzeugte Datei ohne Quelle |
| `slo-docs` | Tabelle "SLOs" in `docs/observability/slos.md` gleich den Sloth-Dateien (Zeilen, Ziel, Schwelle, Alarm) |
| `alert-thresholds` | Alarme der Tabelle "Weitere Alarme" in `docs/observability/alerting.md` gleich `bothy-alerts.yaml`; Schwellen gleich (`` `<name>` ``) oder echt größer (``über `<name>` ``) als der Standard des Konfigurationswerts in `docs/operations/configuration.md` |
| `tofu` | `tofu init -backend=false` und `tofu validate` je Modul unter `infra/tofu/modules/`, alle Module in einer Kopie von `infra/tofu/` |

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
  und den Variablen, die der Check nennt; die Umgebung der aufrufenden Shell erreicht ihn nie. Ein
  unerwarteter Download scheitert so sofort und wird ein Befund, statt das Netz zu erreichen.
  `GCE_METADATA_HOST` braucht es, weil die GCP-Metadaten-Bibliothek den Proxy nicht nutzt; der
  Datadog-Exporter fragt sie bei der Hostnamen-Erkennung auch mit festem Hostnamen ab. Unter
  macOS hält `test_no_validator_reaches_the_network` das: Alle Checks laufen dort in einer
  Sandbox, die jeden Validator bei einer Verbindung über Loopback hinaus beendet. Nach
  `COMMAND_TIMEOUT_S` wird ein Prozess beendet und gemeldet.
* `collector`: Die Arbeitskopie von `infra/otel/` liest ihre Geheimnisse aus Platzhalterdateien
  (`SECRET_PLACEHOLDER`, enthält `EXAMPLE`, mindestens 32 Byte für den Pseudonym-Schlüssel) statt
  aus `/run/bothy/secrets/`; Endpunkte ohne Standardwert zeigen auf `example.com`. In der Kopie von
  `exporters/datadog.yaml` setzt der Check `exporters.datadog.hostname: validate.example.com`.
  Ohne Hostnamen wartet `validate` auf die Hostnamen-Erkennung, die Metadaten-Endpunkte von Clouds
  abfragt (mit 0.162.0 am 2026-10-10 rund 25 Sekunden gemessen). Die Abfrage der GCP-Metadaten,
  die der Exporter auch mit Hostnamen startet, hält `GCE_METADATA_HOST` auf (siehe oben). Das
  Overlay im Repository bleibt unverändert, der Betrieb behält die automatische Erkennung.
* `tofu`: Die CLI-Konfiguration erlaubt als einzige Provider-Quelle einen leeren lokalen Mirror,
  und Modulquellen aus Registry oder HTTP scheitern am geschlossenen Proxy (Git-Quellen am leeren
  `PATH`). Ein Modul mit Providern oder entfernten Modulen scheitert darum, bis es offline
  initialisiert (siehe `docs/quality/gates.md`, "Was die Gates nicht abdecken").

## Einen Check ergänzen

1. Funktion `check_<name>(ctx: Context) -> list[str]` in `opscheck.py` mit Docstring; jede
   Zeichenkette der Liste ist ein Befund. Ein Validator läuft nur über `run_command`, ein Pfad
   relativ zur Wurzel steht als Konstante oben im Modul.
2. Eintrag in `CHECKS` (Reihenfolge wie die Tabelle oben) und eine Zeile in dieser Tabelle.
3. Ein neues Werkzeug bekommt seinen Pin (`make tools-lock`), seine Zeile in
   `docs/quality/toolchain.md` und einen Test, der `ops-check` mit dem echten Werkzeug laufen
   lässt.
4. Tests in `tests/test_opscheck.py`: Die Fixture `tree` kopiert die gelesenen Teilbäume nach
   `tmp_path`; ein Test bricht genau eine Datei und erwartet den Befund des Checks. Liest der
   Check einen neuen Teilbaum, kommt er in `SUBTREES`.
5. Eine neue Collector-Konfiguration (etwa ein Test-Overlay) liest `check_collector` nur, wenn
   sie unter `infra/otel/exporters/` liegt oder der Check ihren Ort ausdrücklich nennt.
