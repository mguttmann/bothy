# opscheck

Checks every operations configuration of the repository with the pinned tool that reads it in
operation, and keeps derived values equal to their source (PLAN M1-04, chapter 17
"Tests that hold these rules"). Gate: `make ops-check`, part of `make gates`.

## Checks

| Check | Checks |
|---|---|
| `tool-versions` | pinned versions of `otelcol-contrib`, `promtool`, `amtool`, `loki`, `tempo` equal to `BOTHY_<X>_VERSION` in `infra/compose/.env.example` and the defaults in `infra/compose/compose.yaml` (a leading `v` does not count) |
| `collector` | `otelcol-contrib validate` for the base alone, the base with each overlay and with every ordered pair of two overlays except `none`; before that, a reference to a config provider other than `env`, `file` and `yaml` (`${https:...}`, `${s3:...}`) is a finding and no `validate` runs |
| `collector-allowlist` | `bothy.*` keys of `redaction/allowlist` equal to the table in `docs/observability/signals.md` 5.3; the allowlist works: `allow_all_keys` is false, every trace and log pipeline with a receiver runs it, and no overlay redefines the processor or a pipeline of the base |
| `prometheus-config` | `promtool check config` on a copy whose `/etc/prometheus/` paths point into the copy, so that the rule files are checked too; every entry of `rule_files` lies under `/etc/prometheus/` (relative: from `prometheus.yaml`) and hits a file, and every file in `rules/` is loaded |
| `prometheus-rules` | `promtool check rules` for every file in `rules/` |
| `prometheus-rule-tests` | `promtool test rules` for every `tests/*.test.yaml`; every alert from `bothy-alerts.yaml` fires in at least one test (non-empty `exp_alerts`) |
| `alertmanager` | `amtool check-config` and the routes of `docs/observability/alerting.md` "Levels and routes" with `amtool config routes test`, on a copy whose `/etc/alertmanager/` paths point into the copy, so the templates are parsed too; every `templates` entry lies under `/etc/alertmanager/` (relative: from `alertmanager.yaml`) and matches a file |
| `loki` | `loki -verify-config` |
| `tempo` | `tempo -config.verify=true` |
| `sloth` | every `rules/slo-<n>.yaml` equal to a fresh `sloth generate` from `slo/<n>.yaml`, byte for byte; no generated file without a source |
| `slo-docs` | table "SLOs" in `docs/observability/slos.md` equal to the Sloth files (rows, Target, Threshold, Alert) |
| `sli-bounds` | `services/server/internal/stats/domain/slibounds.gen.go` and `packages/bothy_viz/lib/src/slo/slo_catalog.g.dart` equal a fresh run of `tools/observability/sli_bounds.py` ([README](../observability/README.md)) |
| `sli-labels` | every SLO of the Sloth files has exactly one label `bothy_sli_metric` with an `sli.*` value of the `CHECK` of `stats_minutes.metric` (03), every such value belongs to exactly one SLO, and the column "Metric of the own measurement" of `docs/observability/slos.md` equals the labels |
| `alert-thresholds` | alerts of the table "Further alerts" in `docs/observability/alerting.md` equal to `bothy-alerts.yaml`; thresholds equal to (`` `<name>` ``) or strictly greater than (``above `<name>` ``) the default of the configuration value in `docs/operations/configuration.md` |
| `tofu` | `tofu init -backend=false` and `tofu validate` for each module under `infra/tofu/modules/` (a directory with `.tf`, `.tf.json`, `.tofu` or `.tofu.json` files), all modules in one copy of `infra/tofu/` |

## Usage

```
make ops-check
uv run --project tools --locked python tools/opscheck/opscheck.py --only collector sloth
uv run --project tools --locked python tools/opscheck/opscheck.py --list
```

Output per check `<check>: ok` or `<check>: FAIL` with the findings below; exit code 1 if a check
fails, 2 for a wrong call. A run executes all checks and shows all findings.

## How the validators run

* The tools come from `tools/bin/tools.lock.json` through `tools/bin/fetch_tools.py` (sha256 per
  platform, cache `.cache/tools/`). If a tool is missing and cannot be downloaded, its check
  fails; there is no `SKIP` (R-QA-02).
* Every check gets a folder of its own in a temporary folder (`tempfile.mkdtemp`), which is
  deleted at the end. Every process runs there with exactly `PATH`, `HOME` and `TMPDIR` in this
  folder (`PATH` points to an empty folder), the proxy variables `HTTP_PROXY` and `HTTPS_PROXY`
  (also in lower case) and `GCE_METADATA_HOST` on a closed port of `127.0.0.1`, and the
  variables the check names; the environment of the calling shell never reaches it.
  The proxy stops only clients that honour these variables, such as the module downloads of
  `tofu init` from a registry or over HTTP: such a request fails at once and becomes a finding.
  A client with a transport of its own ignores the proxy; for the known ones see the next
  sentence, the provider rule of `collector` and the `gcs::` note of `tofu` below.
  `GCE_METADATA_HOST` is needed because the GCP metadata library does not use the proxy;
  the Datadog exporter queries it during the host name detection even with a fixed host name. On
  macOS, `test_no_validator_reaches_the_network` holds this: all checks run there in a sandbox
  that ends every validator on a connection beyond loopback. After `COMMAND_TIMEOUT_S` a process
  is ended and reported.
  When SIGINT or SIGTERM stops a run, the running validators are killed by their process id
  and the work directory is removed as well; the exit status is 143 after SIGTERM, and after
  SIGINT the script ends by SIGINT, as Python does (a shell shows 130).
* `collector`: the work copy of `infra/otel/` reads its secrets from placeholder files
  (`SECRET_PLACEHOLDER`, contains `EXAMPLE`, at least 32 bytes for the pseudonym key) instead of
  `/run/bothy/secrets/`. The check sets only the variables of `COLLECTOR_ENV` in `opscheck.py`,
  each to an `example.com` address or a zero UUID: the variables the overlays use without a
  default (Azure Monitor, Grafana, Sentry). A new overlay with an `${env:NAME}` reference without
  a default (instead of `${env:NAME:-<default>}`) needs `NAME` in `COLLECTOR_ENV`; otherwise it
  expands to an empty value, and for an endpoint `validate` fails (with the `otlphttp` exporter:
  "at least one endpoint must be specified"). In the copy of
  `exporters/datadog.yaml` the check sets `exporters.datadog.hostname: validate.example.com`.
  Without a host name, `validate` waits for the host name detection, which queries the metadata
  endpoints of clouds (measured on 2026-10-10 with 0.162.0 at about 25 seconds, the default
  `hostname_detection_timeout` of the exporter; source in `docs/DECISIONS.md` M1-04/6).
  `GCE_METADATA_HOST` stops the query of the GCP metadata that the exporter starts even with a
  host name (see above). The overlay in the repository stays unchanged, and operation keeps the
  automatic detection.
  Before any `validate`, a reference to a config provider other than `env`, `file` and `yaml`
  (`${https:...}`, `${s3:...}`, `${googlesecretmanager:...}`) in a collector file is a finding,
  also inside another reference (`${env:NAME:-${https:...}}`): the collector resolves such a
  reference while it validates, and most of these providers fetch over the network with clients
  that may bypass the proxy.
* `tofu`: the CLI configuration allows an empty local mirror as the only provider source, and
  module sources from a registry or HTTP fail at the closed proxy (Git sources at the empty
  `PATH`). A module with providers or remote modules therefore fails until it initializes offline
  (see `docs/quality/gates.md`, "What the gates do not cover").
  A Google Cloud Storage module source (`gcs::`) uses a client of its own that the proxy does
  not stop: in the red team's run on 2026-10-11 it waited until `COMMAND_TIMEOUT_S` and was then
  reported. An `s3::` source failed at the closed proxy in a review run on 2026-10-11.

## Adding a check

1. A function `check_<name>(ctx: Context) -> list[str]` in `opscheck.py` with a docstring; every
   string of the list is a finding. A validator runs only through `run_command`, and a path
   relative to the root stands as a constant at the top of the module.
2. An entry in `CHECKS` (order as in the table above) and a row in this table.
3. A new tool first needs its entry in `tools/bin/tools.lock.json`, written by hand: keys
   `version`, `repo`, `tag`, `license`, `binary` and `assets` with `asset`, `member` and a
   placeholder `sha256` of 64 zeros per platform (the lock file must load). `make tools-lock`
   only re-pins an existing entry; for an unknown name it stops with `unknown tool: <name>`
   (exit 2). Then `make tools-lock TOOL=<name> VERSION=<version>` writes the checksums of the
   GitHub release. The tool also gets its row in `docs/quality/toolchain.md` and a test that
   runs `ops-check` with the real tool.
4. Tests in `tests/test_opscheck.py`: the fixture `tree` copies the read subtrees to `tmp_path`;
   a test breaks exactly one file and expects the finding of the check. If the check reads a new
   subtree, it goes into `SUBTREES`.
5. `check_collector` reads a new collector configuration (such as a test overlay) only if it lies
   under `infra/otel/exporters/` or the check names its place explicitly.
