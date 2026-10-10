# M1-04: Operations configuration checks in the gate

Status: ready | Depends on: M1-01 | Etappe: M1

## Goal

After this ticket `make ops-check` is part of `gates` and validates every operations
configuration file of the repository with the pinned upstream tool that reads it: the
OpenTelemetry Collector base and every overlay alone and in pairs, Prometheus configuration,
rules and rule unit tests, Alertmanager configuration and routing, Loki, Tempo, the Sloth SLO
files and OpenTofu modules. It also proves that derived values equal their sources: generated SLO
rules equal a fresh Sloth run, the SLO table equals the Sloth files, the collector allowlist equals
the signal catalogue, alert thresholds equal the configuration values they stand for, and the
validator versions equal the image versions of the Compose stack. A broken observability file
then fails before the push instead of at the next deployment.

## Read first

- `docs/PLAN.md`: section "Arbeitsweise je Ticket"; section "M1 Fundament" (Block A, row M1-04).
- `docs/spec/17-observability.md`: "Collector und Backends", "SLOs und Alarme", "Tests, die diese
  Regeln halten" (items 2, 5, 6), R-OB-01, R-OB-09.
- `docs/observability/collector.md` (all; section "Prüfen"), `docs/observability/signals.md`
  section "Attribute in Spans und Logs" (5.3), `docs/observability/slos.md` ("SLOs", "Erzeugen und
  prüfen"), `docs/observability/alerting.md` ("Stufen und Wege", "Weitere Alarme", "Prüfen").
- Files under test: `infra/otel/collector.yaml`, `infra/otel/exporters/*.yaml`,
  `infra/observability/prometheus/prometheus.yaml`, `infra/observability/prometheus/rules/*.yaml`,
  `infra/observability/alertmanager/alertmanager.yaml`, `infra/observability/loki/loki.yaml`,
  `infra/observability/tempo/tempo.yaml`, `infra/observability/slo/*.yaml`,
  `infra/tofu/modules/*/variables.tf`, `infra/compose/compose.yaml` and
  `infra/compose/.env.example` (image versions only).
- `docs/operations/configuration.md` section "Benannte Werte aller Kapitel" (rows
  `status.fanout_lag_degraded`, `agent.presence_timeout`, `backup.max_age`,
  `backup.pg.archive_timeout`, `backup.restore_test.max_age`).
- ADRs: `docs/adr/0034-opentelemetry-ueber-collector-lokaler-standard-stack.md`,
  `docs/adr/0035-slos-mit-fehlerbudgets-und-burn-rate-alarmen.md`,
  `docs/adr/0012-oci-images-compose-opentofu.md`,
  `docs/adr/0038-lokale-gates-zuerst-ein-befehl-identisch-zu-ci.md`.
- `docs/spec/23-qualitaet.md` ("Grundsätze", "Gates"), `docs/quality/gates.md`,
  `docs/quality/toolchain.md`, `tools/README.md`, `tools/bin/fetch_tools.py` with
  `tools/bin/tests/test_fetch_tools.py`, `tools/bin/tools.lock.json`, `tools/pyproject.toml`,
  `renovate.json`, `Makefile` (including the server gates from M1-01), `CHANGELOG.md`,
  `docs/DECISIONS.md`.

This ticket touches no API operation, no table of chapter 03 and no action id.

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or
   doc comment (purpose, parameters, return value, errors, example where useful); comments explain
   why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision,
   a CHANGELOG entry per release, docs site kept current. A change without matching documentation
   is not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the
   merge.
6. Test first: every behaviour gets a test that is red before the change (show the red output in
   the report), then green.
7. No en dash (U+2013) and no em dash (U+2014) in any file or commit message. Check every changed
   file with `grep -nP "[\x{2013}\x{2014}]" <file>` (must print nothing).
8. White label: no company, person or host names, no real domains (only example.com and
   example.org), no real IP addresses, no absolute paths into a home directory, no internal
   system names, in any file including tests and fixtures. Placeholder secrets contain the word
   `EXAMPLE`.
9. Thresholds are named configuration values; the threshold check reads them from
   `docs/operations/configuration.md` and never hard-codes a value.
10. Fail closed: a missing tool is an error, never a skip (23 "Grundsätze" item 3, R-QA-02); every
    tool pinned with a checksum (23 "Grundsätze" item 4).
11. Validation commands named by 17 "Tests, die diese Regeln halten" items 5 and 6 and by the
    observability guides: `otelcol-contrib validate` for base, each overlay and pairs (collector.md
    "Overlays": two overlays combine), `promtool check rules`, `amtool check-config`, Sloth
    regeneration without diff, SLO table equal to the Sloth files, collector allowlist equal to the
    signal catalogue (signals.md 5.3), alert thresholds equal to configuration values
    (alerting.md "Weitere Alarme").
12. Alert texts and other runtime texts in ops files are English (R-OB-09, L5).

## Scope

### 1. Tool pins (`tools/bin/tools.lock.json`)

Seven entries, key order `version`, `repo`, `tag`, `license`, `binary`, `assets` (the Renovate
regex expects it). Platform keys `darwin-amd64`, `darwin-arm64`, `linux-amd64`, `linux-arm64`;
`<p>` below is the platform key, `<o>_<a>` its underscore form (`darwin_arm64`).

| name | version | repo | license | binary | asset | member |
|---|---|---|---|---|---|---|
| `otelcol-contrib` | `0.162.0` | `open-telemetry/opentelemetry-collector-releases` | `Apache-2.0` | `otelcol-contrib` | `otelcol-contrib_{version}_<o>_<a>.tar.gz` | `otelcol-contrib` |
| `promtool` | `3.15.0` | `prometheus/prometheus` | `Apache-2.0` | `promtool` | `prometheus-{version}.<p>.tar.gz` | `prometheus-{version}.<p>/promtool` |
| `amtool` | `0.34.1` | `prometheus/alertmanager` | `Apache-2.0` | `amtool` | `alertmanager-{version}.<p>.tar.gz` | `alertmanager-{version}.<p>/amtool` |
| `loki` | `3.7.8` | `grafana/loki` | `AGPL-3.0-only` | `loki` | `loki-<p>.zip` | `loki-<p>` |
| `tempo` | `3.1.0` | `grafana/tempo` | `AGPL-3.0-only` | `tempo` | `tempo_{version}_<o>_<a>.tar.gz` | `tempo` |
| `sloth` | `0.16.0` | `slok/sloth` | `Apache-2.0` | `sloth` | `sloth-<p>` (raw binary) | empty string |
| `tofu` | `1.13.1` | `opentofu/opentofu` | `MPL-2.0` | `tofu` | `tofu_{version}_<o>_<a>.tar.gz` | `tofu` |

Tag is `v{version}` for all seven. sha256 values (GitHub release asset digests, read on
2026-10-10), in the order darwin-amd64, darwin-arm64, linux-amd64, linux-arm64:

- otelcol-contrib: `91a6e7a0f5e1981986c897db4535de5ba8d7fc45d4205809aecb9ebebf68c71c`,
  `d5e11974d2e4adac3cc001a25137aad52f6e77ec3034ba7735cac7c219a5f97a`,
  `fcc063749f730f8c21fe29f2d340ff174f5f1c5885bd3156fb6c985a3036fcc3`,
  `ecf6a917e2b53beb1703b19a84bc4e112a2fcaba549639cf6b0ba58446bfd7d2`
- promtool: `2d79e744c2d7e505db936fbc898e05abc74fcb6e437c25befd26e9c9f00aa58b`,
  `920df4d17e78b3b0175af144eb318b0c74d1cf7b1d1251b326966f0e81977260`,
  `2a542df32eac02ee17b9d844fb2aa1de00dafa5476579ba8a3ba862e9d572ea0`,
  `f1f90ec08e849d494ca66c611470afc50192f0355f1a61c33f2cbde02d067823`
- amtool: `9f3a3806c81a2ef3546fea9bae3dd39abcf2206edd112c9433133a9355875fce`,
  `a3941879f340ef12a4cc1a479c82bd8520611265ea8d1ee25c88bd5f944d8293`,
  `265b9d1e55ef0d5306a436018af6d2b686c2ce051f03d968f7464ecb1372a7e8`,
  `d98d6cbaf52151c7e76e24355fec88b11cebcb9875d4cdd8b76ddce7a7e5535c`
- loki: `313d03f8f2d88bcfd79e11dec52655997805f8c94d12ac26efe555b478a09674`,
  `95d830437482aba989a7d7a65be3c618a67a6e46f6e614544cc4b9ce09f499a1`,
  `62aea42c9cba52cd1642b3666ab37019a0ce4c24ab50b07e85dccc8d812f7d61`,
  `a874c10ffefe4d66c914b8aac8897159451d3026469598341aa03c76084697de`
- tempo: `4459d55672712d88529d9e4e2975e57f66356b57261bc4f3b35e78ee88725c79`,
  `0511d3d7fb959312ae6e3b269f469942e9fc4839e53f01c07cc65844f312eb0c`,
  `9604ab46348270a42954801bba91cd65335e16568ef886bdf71b988fdcb7ac4e`,
  `e4092dcf2b93e141be77e300b539f6c5cceefa9803417d7d635c6797d9bc19ad`
- sloth: `c62623ea730818ef611c82fd1318d9741cfde4717a6179ebd9e3649a3b8b6017`,
  `d9ee5967fd412f6592a0c5aec35f7f9f1fc4a2726eb25414a11815ca78185aa9`,
  `1cecaa50853abbb3669a928612b523df8174d258ea6f2a3c8f048e5fc5af90b2`,
  `7b570c465df3cc997e744dfcfcea83c3559bba6b12f4d6165994925419cbb41c`
- tofu: `a73720443ba38712d7d96dc1e857add02c15a790919c653ad07492e9952f8c27`,
  `be78f659f04ef06a9dbd9b3934d46af95d787a3aa38396d459dea395261816a9`,
  `378ada19d4bc70c43732004e8159be771b23b9a5afdf059e5f8a2b3fa2c70a69`,
  `9c1ef375aa1852db0b2888aa921b640c71f8140d4682aa4fec99378a64fa7dc3`

Verify with `make tools-lock TOOL=<name> VERSION=<version>` per tool (must leave the file
unchanged) and `make bootstrap-ci` (downloads and verifies all pins). The binaries are large; they
land in `.cache/tools/` (git-ignored), which CI caches by the hash of `tools.lock.json`.

### 2. Zip archives in `tools/bin/fetch_tools.py`

Loki publishes only `.zip` assets. Extend `_extract` so that an archive whose bytes start with the
zip signature `PK\x03\x04` is read with `zipfile` (same rules as for tar: the member must exist and
be a regular file; no path outside the archive), everything else as today. Tests in
`tools/bin/tests/test_fetch_tools.py`: zip member extracted, missing zip member fails, zip member
that is a directory fails, tar behaviour unchanged. Update the module docstring.

### 3. `tools/opscheck/opscheck.py` (new; Python 3.12, standard library plus PyYAML)

CLI: `opscheck.py [--root <repo root>] [--only <check> ...] [--list]`. Runs every check (or the
named ones), prints `<check>: ok` or `<check>: FAIL` followed by its finding lines, and exits 1 if
any check failed, else 0. It never stops at the first failing check, so one run shows everything.

Public interface with docstrings:

- `@dataclass(frozen=True) class CheckResult: name: str; findings: tuple[str, ...]`
- `@dataclass(frozen=True) class Context: root: Path; workdir: Path; tool: Callable[[str], Path]`
  (`tool(name)` returns the pinned binary via `fetch_tools.ensure`; tests may inject it).
- `CHECKS: Mapping[str, Callable[[Context], list[str]]]` in the order of the table in "Design
  notes".
- `def run_checks(root: Path, only: Sequence[str] | None = None, *, tool=None) -> list[CheckResult]`
  (creates and removes its own `tempfile.mkdtemp` work directory).
- Helpers used by several checks and tested on their own: `parse_duration(text: str) -> float`
  (units `ms`, `s`, `m`, `h`, `d`; seconds), `markdown_table(text: str, heading: str) ->
  list[dict[str, str]]` (first table after the heading, keyed by header cell),
  `config_default(reference: Path, name: str) -> str` (row of the table with the columns
  `Schlüssel` and `Standard` whose first cell is `` `name` ``; backticks stripped; missing or
  duplicate row raises; columns found by header name because M1-08 may add columns).

Every subprocess runs with `check=False`, `capture_output=True`, a timeout constant, and an
explicit minimal environment (`PATH`, `HOME` and `TMPDIR` pointing into the work directory, plus
the variables a check names below). It never inherits the caller's environment, so a developer's
shell cannot change the result. A non-zero exit becomes a finding with the command and the last
lines of its output.

### 4. Prometheus rule unit tests

`infra/observability/prometheus/tests/bothy-alerts.test.yaml` (new, `promtool test rules`
format, `rule_files: [../rules/bothy-alerts.yaml]`, `evaluation_interval: 1m`). For every alert
in `bothy-alerts.yaml` at least one case where it fires (after its `for`, with the expected
labels and the rendered `summary`) and, for every alert with a threshold, one case at the
threshold where it does not fire. The directory lies outside the `rules/*.yaml` glob of
`prometheus.yaml`, so Prometheus never loads it as rules.

### 5. Wiring and documentation

- `Makefile`: `ops-check` (`## Operations configuration valid with the pinned upstream tools;
  derived values equal their sources`) running `$(UVRUN) python tools/opscheck/opscheck.py`, in
  `gates` after `api-lint` (or after `api-checks` if M1-03 is merged); bandit directory list of
  `lint` gains `tools/opscheck`; `.PHONY`. No SKIP: the files exist.
- `tools/pyproject.toml`: add `opscheck` to `testpaths`, `pythonpath`, coverage `source`, mypy
  `files`.
- `renovate.json`: a package rule for `custom.regex` in `tools/bin/tools.lock.json` with
  `matchDepNames` `otelcol-contrib`, `promtool`, `amtool`, `loki`, `tempo` and `enabled: false`,
  description: these validators follow the image versions in `infra/compose/`; bump image and pin
  together; check `tool-versions` holds them equal. Sloth and tofu stay with the "release
  binaries" group. `make lint` validates the file against the Renovate schema.
- `docs/quality/gates.md`: row `ops-check` (German) and, under "Was die Gates nicht abdecken",
  that a future OpenTofu module with providers must make `tofu init` work offline (the check
  fails otherwise). `docs/spec/23-qualitaet.md` "Gates": new group row `| Betrieb | \`ops-check\` |
  Collector, Prometheus, Alertmanager, Loki, Tempo, Sloth, OpenTofu; abgeleitete Werte gleich ihrer
  Quelle |`.
- `docs/quality/toolchain.md`: seven rows in "Gate-Werkzeuge" with version, purpose, license and
  reason (for Loki and Tempo note that they are executed for validation only and not distributed;
  AGPL-3.0 applies to distribution and network use of modified versions, neither happens here).
  `tools/docs/tests/test_toolchain_doc.py` must stay green.
- `tools/README.md` (row `opscheck/`, binaries list in row `bin/`), `tools/opscheck/README.md`
  (German, short: checks, command, how to add a check).
- `docs/observability/collector.md` "Prüfen", `docs/observability/slos.md` (header sentence and
  "Erzeugen und prüfen"), `docs/observability/alerting.md` (sentence in "Weitere Alarme" and
  section "Prüfen"), `docs/observability/signals.md` 5.3 (sentence "ein Check hält beide gleich"),
  `docs/spec/17-observability.md` "Tests, die diese Regeln halten" items 5 and 6: replace
  "Bau-Ticket ..." and "empfohlen" wording by `make ops-check` and the check name.
- `CHANGELOG.md` `[Unreleased]`, `docs/DECISIONS.md` (rulings below).

## Out of scope

- Metric names used in dashboards, rules and SLOs against the signal catalogue, cardinality, bucket
  boundaries: M1-68 (`signal-check`). PromQL validity of the Grafana dashboard queries: not covered
  by any M1 ticket (reported as a gap).
- Grafana provisioning and the running Compose stack: portability test (M1-73, M7).
- Canary test of telemetry content: M1-69.
- OpenTofu resources and providers: later module tickets.
- `docker compose config` validation of the Compose files: not part of this ticket.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-04-ops-config-checks`.
- Conventional Commits with DCO sign-off (`git commit -s`), for example `build(opscheck): ...`,
  `test(observability): ...`.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except `docs/DECISIONS.md` and `CHANGELOG.md` as
  named. If an existing ops file fails a check, fix the file in this ticket and name the fix in the
  report; never weaken the check.
- Follow the host rules supplied by the launcher. Never start the Compose stack or a long-running
  service; the validators run as short-lived processes only. Stop only processes you started, by
  PID. Temporary directories only via `tempfile.mkdtemp`, `mktemp -d` or pytest `tmp_path`,
  removed at the end.
- Never read or print secrets; every secret file the checks create holds a placeholder with
  `EXAMPLE`. The launcher provides the private deny list; do not set
  `DENYLIST_FLAGS=--allow-missing` or `SONAR_SKIP=1`.
- No network calls except package downloads for pinned tools and the `make tools-lock` runs above.
  The validators make no network calls with the setup below (the datadog host name rule in
  "Design notes" exists for exactly this reason).
- Do not lower a threshold, disable a gate or delete a test.

## Design notes

### Checks

| Check | What it runs or compares |
|---|---|
| `tool-versions` | the pinned versions of `otelcol-contrib`, `promtool`, `amtool`, `loki`, `tempo` equal `BOTHY_OTELCOL_VERSION`, `BOTHY_PROMETHEUS_VERSION`, `BOTHY_ALERTMANAGER_VERSION`, `BOTHY_LOKI_VERSION`, `BOTHY_TEMPO_VERSION` in `infra/compose/.env.example` and the defaults `${BOTHY_..._VERSION:-...}` in `infra/compose/compose.yaml` (leading `v` ignored) |
| `collector` | `otelcol-contrib validate --config=<base>` alone; base plus each overlay; base plus every ordered pair of two different overlays except `none` (Compose sets `BOTHY_OTEL_EXPORTER` then `BOTHY_OTEL_EXPORTER_2`, and list merging depends on order) |
| `collector-allowlist` | the `bothy.*` keys of `processors.redaction/allowlist.allowed_keys` in `infra/otel/collector.yaml` equal the attributes of the table in signals.md "Attribute in Spans und Logs" (a cell may list several backticked names) |
| `prometheus-config` | `promtool check config <work>/prometheus/prometheus.yaml` on a copy whose `/etc/prometheus/` prefix points into the work copy, so the rule files load too |
| `prometheus-rules` | `promtool check rules` on every file in `rules/` |
| `prometheus-rule-tests` | `promtool test rules` on every `tests/*.test.yaml`; at least one test file exists; every alert of `rules/bothy-alerts.yaml` appears as `alertname` in some test |
| `alertmanager` | `amtool check-config <file>`; routing per alerting.md "Stufen und Wege": `amtool config routes test --config.file=<file> --verify.receivers=external-oncall,bothy-ops severity=page alertname=BothyExample`, `--verify.receivers=bothy-ops severity=ticket alertname=BothyExample`, `--verify.receivers=external-heartbeat alertname=Watchdog severity=heartbeat` (each label a separate argument) |
| `loki` | `loki -config.file=<file> -config.expand-env=true -verify-config` |
| `tempo` | `tempo -config.file=<file> -config.expand-env=true -config.verify=true` (Tempo needs the `=true` form) |
| `sloth` | for each `infra/observability/slo/<n>.yaml`: `sloth generate --no-log -i <file> -o <work>/slo-<n>.yaml` equals `rules/slo-<n>.yaml` byte for byte; every `rules/slo-*.yaml` has a source |
| `slo-docs` | table in slos.md "SLOs" equals the Sloth files, see below |
| `alert-thresholds` | alerting.md "Weitere Alarme" against `rules/bothy-alerts.yaml` and the configuration reference, see below |
| `tofu` | for each directory under `infra/tofu/modules/` with a `.tf` file: copy to the work directory, `tofu init -backend=false -input=false -no-color`, then `tofu validate -no-color`, with `TF_IN_AUTOMATION=1` and `TF_CLI_CONFIG_FILE` pointing to an empty file |

Collector environment: copy `infra/otel/` into the work directory and replace every
`/run/bothy/secrets/` by `<work>/secrets/`; create one file per secret name found in the copies,
content `EXAMPLE` repeated to 48 bytes (the pseudonym key needs at least 32 bytes). Set exactly
these variables (all others in the files have defaults):

```
AZURE_TENANT_ID=00000000-0000-0000-0000-000000000000
AZURE_CLIENT_ID=00000000-0000-0000-0000-000000000000
AZURE_MONITOR_TRACES_ENDPOINT=https://ingest.example.com/v1/traces
AZURE_MONITOR_METRICS_ENDPOINT=https://ingest.example.com/v1/metrics
AZURE_MONITOR_LOGS_ENDPOINT=https://ingest.example.com/v1/logs
GRAFANA_OTLP_ENDPOINT=https://otlp.example.com/otlp
SENTRY_OTLP_TRACES_ENDPOINT=https://sentry.example.com/api/1/integration/otlp/v1/traces
SENTRY_OTLP_LOGS_ENDPOINT=https://sentry.example.com/api/1/integration/otlp/v1/logs
```

In the work copy of `exporters/datadog.yaml` only, set `exporters.datadog.hostname:
validate.example.com` (edit the parsed YAML, write it back). Without it, `validate` with the
datadog overlay waits about 25 seconds for host name detection, which probes cloud metadata
endpoints; that is a network call the gate must not make. Name this adjustment in a code comment
and in `tools/opscheck/README.md`.

Verified on 2026-10-10 with the pinned versions: base, every overlay and all 30 ordered pairs of
the six real overlays pass with this setup; without `GRAFANA_OTLP_ENDPOINT` the grafana overlay
fails, which is why the map is explicit. A single `validate` takes well under a second once the
datadog host name is set; still run the collector validations through a bounded thread pool so
the check stays fast as overlays grow. `amtool check-config` does not read `url_file` targets;
`loki` and `tempo` verify without creating their data directories.

`slo-docs`: for each SLO of each Sloth file there is exactly one row whose first cell is
`` `<service>/<name>` ``, and no extra rows. Column "Ziel" equals the objective formatted with one
decimal and ` %` (`99.9 %`, `95.0 %`). Column "Schwelle" is `keine` if the `error_query` has no
`le="..."` selector, else `<le> s` (`0.5 s`, `600 s`). Column "Alarm" is `Page, Ticket` unless
`page_alert.disable: true`, then `nur Ticket`.

`alert-thresholds`: the alert names of the table in alerting.md "Weitere Alarme" equal the alerts
in `bothy-alerts.yaml`. For each row whose column "Konfigurationswert" is exactly `` `<name>` ``
(must equal) or ``über `<name>` `` (must be strictly greater), read the number of the final
`> <number>` comparison of the alert `expr` (seconds) and compare it with
`parse_duration(config_default(reference, name))`. Other cells are not checked. Today this covers
`BothyEventFanoutLag` (`status.fanout_lag_degraded`), `BothyAgentOffline` (over
`agent.presence_timeout`), `BothyBackupStale` (`backup.max_age`), `BothyWalArchiveStale` (over
`backup.pg.archive_timeout`) and `BothyRestoreTestStale` (`backup.restore_test.max_age`).

### Rulings for `docs/DECISIONS.md`

1. The validators run as pinned release binaries through `fetch_tools.py`, not as container
   images. Reason: same mechanism as every other gate tool, no Docker needed for a docs-only
   change, checksum per platform.
2. `fetch_tools.py` reads zip archives. Reason: Loki publishes only zip assets.
3. Collector pairs are validated in both orders. Reason: overlays merge lists by replacement, so
   the result depends on the order of `BOTHY_OTEL_EXPORTER` and `BOTHY_OTEL_EXPORTER_2`.
4. Validator pins of components that also run as images follow the image versions; Renovate is
   off for them and `tool-versions` holds them equal. Reason: a validator newer or older than the
   running component accepts or rejects other keys.
5. Alertmanager routing is part of the gate. Reason: alerting.md documents the routes as checked
   by hand; a wrong route silences pages.
6. The collector check sets a fixed datadog host name in its work copy. Reason: otherwise
   `validate` probes cloud metadata endpoints (network) and stalls; the overlay itself stays
   unchanged so production keeps automatic host detection.

## Tests (write them first)

`tools/opscheck/tests/test_opscheck.py`, with the real pinned tools (fetched on first use; no
other network). Negative cases copy the needed subtrees (`infra/`, `docs/observability/`,
`docs/operations/configuration.md`, `tools/bin/tools.lock.json`) into `tmp_path`, apply one
mutation and run the single check through `run_checks(tmp_root, only=[name])`:

- `test_repository_passes_all_checks` (real repository).
- `tool-versions`: `test_env_example_version_mismatch_fails`.
- `collector`: `test_unknown_processor_in_base_fails`, `test_unknown_exporter_in_overlay_fails`,
  `test_pair_failure_is_detected`: add two overlay files to the copied `exporters/`, one with
  `processors: {batch: {send_batch_size: 10000}}`, one with
  `processors: {batch: {send_batch_max_size: 5000}}`. Each passes alone with the base; the pair
  fails with "send_batch_max_size must be greater or equal to send_batch_size" (verified with
  0.162.0 on 2026-10-10).
- `collector-allowlist`: `test_allowlist_missing_catalogue_key_fails`,
  `test_allowlist_extra_key_fails`.
- `prometheus-config`: `test_invalid_scrape_config_fails`.
- `prometheus-rules`: `test_invalid_promql_fails`.
- `prometheus-rule-tests`: `test_changed_threshold_breaks_unit_test`,
  `test_alert_without_unit_test_fails`.
- `alertmanager`: `test_route_to_undefined_receiver_fails`, `test_wrong_page_route_fails`.
- `loki`: `test_unknown_loki_key_fails`. `tempo`: `test_unknown_tempo_key_fails`.
- `sloth`: `test_edited_generated_rules_fail`, `test_rules_without_source_fail`.
- `slo-docs`: `test_objective_mismatch_fails`, `test_threshold_mismatch_fails`,
  `test_missing_slo_row_fails`.
- `alert-thresholds`: `test_equal_threshold_mismatch_fails` (`BothyBackupStale`),
  `test_over_threshold_not_greater_fails` (`BothyAgentOffline` at or below
  `agent.presence_timeout`), `test_unknown_config_name_fails`, `test_alert_set_mismatch_fails`.
- `tofu`: `test_invalid_hcl_fails`.
- Helpers: `test_parse_duration`, `test_markdown_table`, `test_config_default_missing_or_duplicate`.
- `test_cli_reports_all_failures` (two broken checks, both reported, exit 1), `test_list`.

`tools/bin/tests/test_fetch_tools.py`: the zip cases of Scope item 2.

Commands that must pass:

```
uv run --project tools --locked pytest tools/opscheck/tests tools/bin/tests
make ops-check
make check
```

## Acceptance criteria

- [ ] `make ops-check` is green on the repository and part of `gates`.
- [ ] `pytest tools/opscheck/tests`: for each check one deliberately wrong fixture fails:
      `otelcol-contrib validate` for base, an overlay and a pair, `promtool check config`,
      `promtool check rules`, `promtool test rules`, `amtool check-config` and routing,
      `loki -verify-config`, `tempo -config.verify`, Sloth regeneration, SLO table, collector
      allowlist, alert thresholds, `tofu validate`, tool versions.
- [ ] Every hand-written alert has a `promtool` unit test.
- [ ] The validators are pinned with sha256 in `tools/bin/tools.lock.json`; `make tools-lock`
      leaves the file unchanged; `fetch_tools.py` handles zip with tests.
- [ ] Docs updated: `docs/quality/gates.md`, `docs/quality/toolchain.md`, chapter 17 and 23,
      the four observability guides, `tools/README.md`, `tools/opscheck/README.md`, `renovate.json`,
      `CHANGELOG.md`, `docs/DECISIONS.md`.
- [ ] `make check` green; no file of the diff contains U+2013 or U+2014.

## Definition of done

Code, tests and docs as above: docstrings on every public symbol of `opscheck.py` and on the
changed `fetch_tools.py` functions; README of the tool; no ADR needed (tooling within ADR 0034,
0035 and 0038; rulings in `docs/DECISIONS.md`); CHANGELOG entry under `[Unreleased]`;
`make check` green. Then `make review` and an independent code review over the diff, then the red
team gate (`docs/quality/red-team-gate.md`) over the diff with verdict recorded. Fix every finding
with a test and re-check the delta. Merge locally into the base branch only if the review is
clean, the red team says `JA` and `make check` is green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red runs), the
runtime of `make ops-check`, every existing ops file you had to fix and why, what is not proven
(for example: validators accept a file that the running component would still reject at runtime;
dashboards are not validated), and follow-ups that could not be done in scope, each with its
reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<merged sha>` or `exit=1 <reason>`.
