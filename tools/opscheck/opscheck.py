# SPDX-License-Identifier: Apache-2.0
"""Operations configuration checks of the gate `make ops-check` (PLAN M1-04).

Every operations file of the repository is validated by the pinned upstream tool that
reads it in production: the OpenTelemetry Collector base alone, with each overlay and
with every ordered pair of overlays; the Prometheus configuration, its rules and the rule
unit tests; the Alertmanager configuration and its routes; Loki; Tempo; the Sloth SLO
files and the OpenTofu modules. Derived values are compared with their sources: generated
SLO rules with a fresh Sloth run, the SLO table with the Sloth files, the collector
allowlist with the signal catalogue, alert thresholds with the configuration values they
stand for, and the validator versions with the image versions of the Compose stack.

The validators come from tools/bin/tools.lock.json through fetch_tools.py (sha256 per
platform). A validator that cannot be fetched fails its check; nothing is skipped
(docs/spec/23-qualitaet.md R-QA-02). Each validator runs as a short-lived process with
a minimal environment inside a temporary work directory, so neither the caller's shell
nor its working directory can change the result.

Usage:
  opscheck.py [--root <repo root>] [--only <check> ...] [--list]

Prints `<check>: ok` or `<check>: FAIL` with one indented line per finding and exits 1
if any check failed, 0 if all passed, 2 on a usage error, and 143 when SIGTERM stops the
run; after SIGINT it ends by SIGINT, as Python does (a shell shows 130). All selected
checks run, so one run shows every problem.

Example:
  opscheck.py --only collector sloth
"""

from __future__ import annotations

import argparse
import difflib
import itertools
import json
import os
import posixpath
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import threading
from collections.abc import Callable, Iterator, Mapping, Sequence
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import yaml

# The Makefile runs this file as a script, so Python puts only tools/opscheck on sys.path;
# the pin reader lives in tools/bin (pytest finds it through its pythonpath setting).
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "bin"))
# The SLI bounds generator lives in tools/observability (brief M6-11).
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "observability"))

import fetch_tools
import sli_bounds
import tofu_providers

REPO_ROOT = Path(__file__).resolve().parents[2]
"""Default `--root`: the checkout this file belongs to."""

COMMAND_TIMEOUT_S: float = 120
"""Seconds one validator process may run before it is killed and reported."""

OUTPUT_TAIL_LINES = 20
"""Lines of validator output (stdout, then stderr) quoted in a finding."""

COLLECTOR_WORKERS = 4
"""Parallel `otelcol-contrib validate` processes; base, overlays and pairs grow quadratically."""

STOP_SIGNALS = (signal.SIGINT, signal.SIGTERM)
"""Signals that stop a run of `main`: the validators are killed, the work directory removed."""

# ---- files the checks read (relative to the repository root) ------------------------------

LOCK = Path("tools/bin/tools.lock.json")
"""Pins of the release binaries; `tool-versions` reads the validator versions here."""
ENV_EXAMPLE = Path("infra/compose/.env.example")
"""Compose variables with the image versions (`BOTHY_<X>_VERSION`)."""
COMPOSE = Path("infra/compose/compose.yaml")
"""Compose file with the image defaults `${BOTHY_<X>_VERSION:-<version>}`."""
OTEL_DIR = Path("infra/otel")
"""Collector base `collector.yaml` and the overlays under `exporters/`."""
SMOKE_COLLECTOR_OVERLAY = Path("infra/compose/test/otel/file.yaml")
"""Overlay "file" of the Compose smoke test (M1-73): test only, validated with the base alone."""
PROMETHEUS_DIR = Path("infra/observability/prometheus")
"""Prometheus configuration; Compose mounts it at /etc/prometheus/."""
RULES_DIR = PROMETHEUS_DIR / "rules"
"""Rule files: hand-written alerts and the rules Sloth generates."""
RULE_TESTS_DIR = PROMETHEUS_DIR / "tests"
"""`promtool test rules` files; outside the rule glob, so Prometheus never loads them."""
HAND_WRITTEN_ALERTS = RULES_DIR / "bothy-alerts.yaml"
"""The alerts that alerting.md "Weitere Alarme" documents."""
ALERTMANAGER_CONFIG = Path("infra/observability/alertmanager/alertmanager.yaml")
"""Alertmanager routes and receivers."""
LOKI_CONFIG = Path("infra/observability/loki/loki.yaml")
"""Loki configuration."""
TEMPO_CONFIG = Path("infra/observability/tempo/tempo.yaml")
"""Tempo configuration."""
SLO_DIR = Path("infra/observability/slo")
"""Sloth SLO files, the source of the generated rules and of the SLO table."""
TOFU_DIR = Path("infra/tofu")
"""OpenTofu tree; copied as a whole so modules may refer to their siblings."""
TOFU_MODULES = TOFU_DIR / "modules"
"""One directory per OpenTofu module."""
TOFU_FILES = ("*.tf", "*.tf.json", "*.tofu", "*.tofu.json")
"""Configuration files OpenTofu reads in a module directory; one of them makes a module."""
SIGNALS_DOC = Path("docs/observability/signals.md")
"""Signal catalogue; table "Attribute in Spans und Logs" holds the allowlist."""
SLOS_DOC = Path("docs/observability/slos.md")
"""SLO guide; table "SLOs" repeats the Sloth files."""
ALERTING_DOC = Path("docs/observability/alerting.md")
"""Alerting guide; table "Weitere Alarme" names the configuration value of a threshold."""
CONFIG_REFERENCE = Path("docs/operations/configuration.md")
"""Configuration reference with the defaults of the named values."""

# ---- values of the checks -----------------------------------------------------------------

VALIDATOR_IMAGES: Mapping[str, str] = {
    "otelcol-contrib": "BOTHY_OTELCOL_VERSION",
    "promtool": "BOTHY_PROMETHEUS_VERSION",
    "amtool": "BOTHY_ALERTMANAGER_VERSION",
    "loki": "BOTHY_LOKI_VERSION",
    "tempo": "BOTHY_TEMPO_VERSION",
}
"""Pinned validator -> Compose variable of the image it validates for (ADR 0041)."""

COLLECTOR_SECRETS_DIR = "/run/bothy/secrets/"
"""Where the collector reads its secret files in the Compose stack."""

SECRET_PLACEHOLDER = (b"EXAMPLE" * 7)[:48]
"""Content of every placeholder secret; the pseudonym key needs at least 32 bytes."""

COLLECTOR_ENV: Mapping[str, str] = {
    "AZURE_TENANT_ID": "00000000-0000-0000-0000-000000000000",
    "AZURE_CLIENT_ID": "00000000-0000-0000-0000-000000000000",
    "AZURE_MONITOR_TRACES_ENDPOINT": "https://ingest.example.com/v1/traces",
    "AZURE_MONITOR_METRICS_ENDPOINT": "https://ingest.example.com/v1/metrics",
    "AZURE_MONITOR_LOGS_ENDPOINT": "https://ingest.example.com/v1/logs",
    "GRAFANA_OTLP_ENDPOINT": "https://otlp.example.com/otlp",
    "SENTRY_OTLP_TRACES_ENDPOINT": "https://sentry.example.com/api/1/integration/otlp/v1/traces",
    "SENTRY_OTLP_LOGS_ENDPOINT": "https://sentry.example.com/api/1/integration/otlp/v1/logs",
}
"""Variables without a default in the overlays; every other variable has one."""

COLLECTOR_PROVIDERS = frozenset({"env", "file", "yaml"})
"""Config providers a collector file may reference (`${env:...}`, `${file:...}`, `${yaml:...}`).

Every other provider is a finding of `collector`: most of them (`http`, `https`, `s3`,
`secretsmanager`, `googlesecretmanager`) fetch their value from elsewhere, possibly over the
network, and the gate allows no provider beyond the three it needs.
"""

DATADOG_VALIDATE_HOSTNAME = "validate.example.com"
"""Fixed host name of the datadog exporter in the work copy (see `_pin_datadog_hostname`)."""

NO_NETWORK_ADDRESS = "127.0.0.1:9"
"""A closed loopback port: whatever a validator sends there fails at once."""

NO_NETWORK_PROXY = f"http://{NO_NETWORK_ADDRESS}"
"""Proxy of every validator, so any HTTP request through the proxy fails at once.

No validator needs the network with the setup of the checks. The proxy stops only clients
that honour the proxy variables, such as the registry and HTTP module downloads of
`tofu init`, and turns such a download into a finding instead of a call. A client with a
transport of its own ignores it: the GCP metadata client (`NO_NETWORK_METADATA`), a remote
config provider of the collector (kept out by `COLLECTOR_PROVIDERS`) and the Google Cloud
Storage module source of `tofu` (`gcs::`), which nothing here stops.
"""

NO_NETWORK_METADATA = {"GCE_METADATA_HOST": NO_NETWORK_ADDRESS}
"""Cloud metadata clients that bypass the proxy, pointed at the closed port as well.

The GCP metadata client has its own transport without proxy and probes 169.254.169.254
directly; the datadog exporter runs it while it resolves its host name, even when one is
configured (ruling M1-04/22). `test_no_validator_reaches_the_network` holds this.
"""

NO_PAIR_OVERLAY = "none"
"""The empty overlay: validated alone, left out of the pairs."""

ROUTE_TESTS: Sequence[tuple[str, tuple[str, ...]]] = (
    ("external-oncall,bothy-ops", ("severity=page", "alertname=BothyExample")),
    ("bothy-ops", ("severity=ticket", "alertname=BothyExample")),
    ("external-heartbeat", ("alertname=Watchdog", "severity=heartbeat")),
)
"""Alertmanager routes of docs/observability/alerting.md "Stufen und Wege": receivers, labels."""

_DURATION = re.compile(r"(\d+(?:\.\d+)?)(ms|s|m|h|d)")
_DURATION_UNITS = {"ms": 0.001, "s": 1.0, "m": 60.0, "h": 3600.0, "d": 86400.0}
_HEADING = re.compile(r"^(#{1,6})\s+(.*?)\s*#*\s*$")
_SECTION_NUMBER = re.compile(r"^\d+(?:\.\d+)*[a-z]?\.?\s+")
_TRAILING_PARENTHESIS = re.compile(r"\s*\([^()]*\)$")
_SEPARATOR_CELL = re.compile(r"^:?-+:?$")
_CELL_SPLIT = re.compile(r"(?<!\\)\|")
_BACKTICKED = re.compile(r"`([^`]+)`")
_GREATER = re.compile(r"(?<![<>=!])>(?!=)")
_FILE_NAME = r"[A-Za-z0-9_][A-Za-z0-9_.-]*"
_PROMETHEUS_MOUNT = "/etc/prometheus/"
_ALERTMANAGER_MOUNT = "/etc/alertmanager/"
_PLAIN_NUMBER = re.compile(r"-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?")
_OPERAND_END = re.compile(r"\)|\b(?:and|or|unless)\b")
_FENCE = re.compile(r"^\s*(`{3,}|~{3,})(.*)$")
# A PromQL `le` matcher with its operator and its value in any of the three quotes.
_LE_MATCHER = re.compile(r"(?<!\w)le\s*(=~|!=|!~|=)\s*(?:\"([^\"]+)\"|'([^']+)'|`([^`]+)`)")
# `$$` (an escaped `$`) or the opening `${<provider>:` of a collector reference; group 1 is
# the provider. Openings, not whole references: a reference inside another one counts too.
_CONFMAP_REFERENCE = re.compile(r"\$\$|\$\{([A-Za-z][A-Za-z0-9+.-]*):")
# Cells of slos.md "SLOs": no threshold or no alert, and the column Alarm by (page, ticket).
_NO_VALUE = "keine"
_ALARM_CELLS: Mapping[tuple[bool, bool], str] = {
    (True, True): "Page, Ticket",
    (False, True): "nur Ticket",
    (True, False): "nur Page",
    (False, False): _NO_VALUE,
}


# ---- public interface ---------------------------------------------------------------------


@dataclass(frozen=True)
class CheckResult:
    """Outcome of one check.

    Attributes:
        name: the check name, a key of `CHECKS`.
        findings: one text per problem; empty means the check passed. A finding may span
            several lines (validator output follows its first line, indented).
    """

    name: str
    findings: tuple[str, ...]


@dataclass(frozen=True)
class Context:
    """What a check may use.

    Attributes:
        root: repository root whose files are checked.
        workdir: empty directory of this check inside the run's temporary directory; copies,
            placeholder secrets and validator output go here.
        tool: returns the path of a pinned binary by its name in tools/bin/tools.lock.json
            (default `pinned_tool`); tests inject their own.
    """

    root: Path
    workdir: Path
    tool: Callable[[str], Path]

    def provider_mirror(self) -> Path:
        """The OpenTofu provider mirror for the lock files of `root` (infra/tofu/test/*).

        The archives are fetched and checked by tools/bin/tofu_providers.py into the cache of
        this checkout (.cache/tofu-providers), like the validators of `pinned_tool`.
        """
        locks = sorted((self.root / TOFU_DIR / "test").glob("*/.terraform.lock.hcl"))
        return tofu_providers.ensure_mirror(
            locks, tofu_providers.DEFAULT_CACHE, fetch_tools.platform_key()
        )


Check = Callable[[Context], list[str]]
"""A check: gets its `Context`, returns one finding per problem; an empty list means ok."""


def parse_duration(text: str) -> float:
    """Convert a duration of the configuration reference into seconds.

    Args:
        text: one or more number-unit pairs without spaces; units `ms`, `s`, `m`, `h`, `d`.

    Returns:
        The duration in seconds.

    Raises:
        ValueError: the text is not such a duration.

    Example:
        >>> parse_duration("1h30m")
        5400.0
    """
    value = text.strip()
    if not value or _DURATION.sub("", value):
        raise ValueError(f"not a duration: {text!r} (units ms, s, m, h, d)")
    return sum(float(n) * _DURATION_UNITS[unit] for n, unit in _DURATION.findall(value))


def markdown_table(text: str, heading: str) -> list[dict[str, str]]:
    """Return the first Markdown table after a heading, one dict per row keyed by header cell.

    Args:
        text: Markdown document.
        heading: heading title without its `#` marks, without a leading section number
            (`5.3`, `2.`) and without a trailing parenthesis, e.g. "Weitere Alarme" for
            "### 3.2 Weitere Alarme (`bothy-alerts.yaml`)". Headings and tables inside
            code fences do not count.

    Returns:
        The rows of the table. Cells are stripped; `\\|` inside a cell becomes `|`.

    Raises:
        ValueError: no such heading, no table before the next heading, or a row whose
            number of cells differs from the header.
    """
    lines = list(_unfenced(text))
    for index, line in enumerate(lines):
        if _heading_title(line) != heading:
            continue
        for start in range(index + 1, len(lines)):
            if _heading_title(lines[start]) is not None:
                break
            if lines[start].lstrip().startswith("|"):
                return _parse_table(lines, start)
        raise ValueError(f"no table after heading {heading!r}")
    raise ValueError(f"no heading {heading!r}")


def config_default(reference: Path, name: str) -> str:
    """Return the default of a named configuration value from the configuration reference.

    Args:
        reference: docs/operations/configuration.md (or a copy).
        name: the configuration key, e.g. `backup.max_age`.

    Returns:
        The cell of column `Standard` of the row whose first cell is `` `name` ``, without
        backticks. Every table with the columns `Schlüssel` and `Standard` is searched;
        columns are found by their header, so tables may gain columns.

    Raises:
        ValueError: no such row, or more than one.
    """
    values = [standard for key, standard in _config_rows(reference) if key == name]
    if not values:
        raise ValueError(f"{name}: not found in {reference}")
    if len(values) > 1:
        raise ValueError(f"{name}: {len(values)} rows in {reference}")
    return values[0]


def _config_rows(reference: Path) -> list[tuple[str, str]]:
    """(key, default) of every row of the tables with the columns `Schlüssel` and `Standard`."""
    lines = list(_unfenced(reference.read_text(encoding="utf-8")))
    found: list[tuple[str, str]] = []
    index = 0
    while index < len(lines):
        if not _is_table_start(lines, index):
            index += 1
            continue
        rows = _parse_table(lines, index)
        index = _table_end(lines, index)
        for row in rows:
            if "Schlüssel" in row and "Standard" in row:
                key = row["Schlüssel"]
                if key.startswith("`") and key.endswith("`"):
                    found.append((key.strip("`"), row["Standard"].replace("`", "")))
    return found


def pinned_tool(name: str) -> Path:
    """Path of the pinned binary `name`, fetched and sha256-checked on first use.

    The validators belong to the tool chain of this checkout (tools/bin/tools.lock.json,
    cache .cache/tools), independent of `--root`.

    Raises:
        fetch_tools.LockError, fetch_tools.FetchError: not pinned for this platform or the
            download failed; the check that asked fails with that message.
    """
    lock = fetch_tools.load_lock(fetch_tools.DEFAULT_LOCK)
    if name not in lock["tools"]:
        raise fetch_tools.LockError(f"{name} is not pinned in {LOCK}")
    plat = fetch_tools.platform_key()
    if plat not in lock["tools"][name]["assets"]:
        raise fetch_tools.LockError(f"{name} has no pin for {plat} in {LOCK}")
    return fetch_tools.ensure(lock, name, plat, fetch_tools.DEFAULT_CACHE)


def run_command(
    ctx: Context,
    args: Sequence[object],
    *,
    env: Mapping[str, str] | None = None,
    cwd: Path | None = None,
    label: str | None = None,
) -> list[str]:
    """Run one validator process and turn a failure into a finding.

    The process gets only `PATH`, `HOME` and `TMPDIR`, each pointing into `ctx.workdir`,
    the proxy variables set to `NO_NETWORK_PROXY`, the variables of `NO_NETWORK_METADATA`,
    plus `env`; it never inherits the caller's environment. It runs in `cwd` (default
    `ctx.workdir`) and is killed after `COMMAND_TIMEOUT_S` seconds, or at once when one of
    `STOP_SIGNALS` stops the run (see `main`).

    Args:
        ctx: context of the calling check.
        args: the command; the first element is the binary; each element passes through str().
        env: additional variables the check names.
        cwd: working directory of the process.
        label: first words of a finding; default is the command itself.

    Returns:
        No finding on exit status 0; else one finding naming the exit status (and the
        command, if a label is given) followed by the last `OUTPUT_TAIL_LINES` lines of
        its output, or a finding for a timeout or a binary that cannot be started.
    """
    command = _display(ctx, args)
    head = label or command
    try:
        proc = _RUNNING.start(
            [str(a) for a in args],
            env={**_base_env(ctx.workdir), **(env or {})},
            cwd=cwd or ctx.workdir,
        )
    except OSError as exc:
        return [f"{head}: cannot run {command}: {exc.strerror or exc}"]
    # Leaving the block closes the pipes and waits for the process, also after a stop.
    with proc:
        try:
            stdout, stderr = proc.communicate(timeout=COMMAND_TIMEOUT_S)
        except subprocess.TimeoutExpired:
            # Kill and wait (when the block is left) without reading on: a child of the
            # validator may still hold the output open.
            proc.kill()
            return [f"{head}: timed out after {COMMAND_TIMEOUT_S} s"]
        except BaseException:  # a stop or KeyboardInterrupt: never leave the validator running
            proc.kill()
            raise
        finally:
            _RUNNING.finish(proc)
    if proc.returncode == 0:
        return []
    status = f"exit status {proc.returncode}" + (f" ({command})" if label else "")
    tail = (stdout.splitlines() + stderr.splitlines())[-OUTPUT_TAIL_LINES:]
    return [f"{head}: {status}" + "".join(f"\n    {line}" for line in tail)]


def run_checks(
    root: Path,
    only: Sequence[str] | None = None,
    *,
    tool: Callable[[str], Path] | None = None,
    on_result: Callable[[CheckResult], None] | None = None,
) -> list[CheckResult]:
    """Run the selected checks in the order of `CHECKS`, each in its own work directory.

    Args:
        root: repository root to check.
        only: names of the checks to run; default all.
        tool: binary resolver for the checks (default `pinned_tool`).
        on_result: called with each result as soon as its check ends (progress output).

    Returns:
        One result per selected check. A check that raises is reported as a finding
        `check aborted: <error>`, and the remaining checks still run.

    Raises:
        ValueError: `only` names an unknown check.
    """
    names = _select(only)
    resolver = tool or pinned_tool
    work = Path(tempfile.mkdtemp(prefix="opscheck-"))
    results = []
    try:
        for name in names:
            workdir = work / name
            workdir.mkdir()
            ctx = Context(root=root.resolve(), workdir=workdir, tool=resolver)
            try:
                found = CHECKS[name](ctx)
            except Exception as exc:  # one broken check must not hide the others
                found = [f"check aborted: {type(exc).__name__}: {exc}"]
            result = CheckResult(name, tuple(found))
            results.append(result)
            if on_result is not None:
                on_result(result)
    finally:
        try:
            shutil.rmtree(work, ignore_errors=True)
        except _Stopped:  # the stop arrived during the removal; a second one never raises
            shutil.rmtree(work, ignore_errors=True)
            raise
    return results


# ---- checks: derived values ---------------------------------------------------------------


def check_tool_versions(ctx: Context) -> list[str]:
    """The pinned validators have the versions of the images they validate for.

    Compares each pin of `VALIDATOR_IMAGES` with its variable in infra/compose/.env.example
    and with every default `${BOTHY_<X>_VERSION:-<version>}` in infra/compose/compose.yaml;
    a leading `v` of the image tag is ignored, and the env file is read as Compose reads it
    (quotes and a trailing ` #` comment do not count).
    """
    lock = json.loads((ctx.root / LOCK).read_text(encoding="utf-8"))["tools"]
    env_text = (ctx.root / ENV_EXAMPLE).read_text(encoding="utf-8")
    compose_text = (ctx.root / COMPOSE).read_text(encoding="utf-8")
    found = []
    for tool, variable in VALIDATOR_IMAGES.items():
        pinned = lock.get(tool, {}).get("version")
        if pinned is None:
            found.append(f"{tool} is not pinned in {LOCK}")
            continue
        env_values = [
            _env_value(v) for v in re.findall(rf"^{variable}=(.*)$", env_text, re.MULTILINE)
        ]
        if len(env_values) != 1:
            found.append(
                f"{variable}: expected exactly one line in {ENV_EXAMPLE}, found {len(env_values)}"
            )
        elif env_values[0].removeprefix("v") != pinned:
            found.append(
                f"{variable} in {ENV_EXAMPLE} is {env_values[0]}, the pinned {tool} is {pinned}"
            )
        defaults = re.findall(rf"\$\{{{variable}:-([^}}]*)\}}", compose_text)
        if not defaults:
            found.append(f"{variable}: no default ${{{variable}:-<version>}} in {COMPOSE}")
        for default in sorted(set(defaults)):
            if default.removeprefix("v") != pinned:
                found.append(
                    f"default of {variable} in {COMPOSE} is {default}, the pinned {tool} is "
                    f"{pinned}"
                )
    return found


def check_collector_allowlist(ctx: Context) -> list[str]:
    """The allowlist is in force, and its `bothy.*` keys equal the catalogue of signals.md 5.3.

    In force means: `allow_all_keys` is false; every traces or logs pipeline that receives
    from a receiver (not a connector) runs `redaction/allowlist`, in the base and in every
    overlay; no overlay redefines the processor or a pipeline of the base (an overlay
    replaces lists, so a redefined pipeline could drop the processor).
    """
    base_path = OTEL_DIR / "collector.yaml"
    config = yaml.safe_load((ctx.root / base_path).read_text(encoding="utf-8"))
    processor = config["processors"]["redaction/allowlist"]
    found = _allowlist_structure(ctx.root, config)
    if processor.get("allow_all_keys", False) is not False:
        found.insert(
            0, f"processors.redaction/allowlist.allow_all_keys of {base_path} must be false"
        )
    keys = processor["allowed_keys"]
    allowed = {str(k) for k in keys if str(k).startswith("bothy.")}
    rows = markdown_table(
        (ctx.root / SIGNALS_DOC).read_text(encoding="utf-8"), "Attribute in Spans und Logs"
    )
    catalogue = {name for row in rows for name in _BACKTICKED.findall(row["Attribut"])}
    where = f"processors.redaction/allowlist.allowed_keys of {base_path}"
    found += [f"{k}: in {SIGNALS_DOC} 5.3 but not in {where}" for k in sorted(catalogue - allowed)]
    found += [f"{k}: in {where} but not in {SIGNALS_DOC} 5.3" for k in sorted(allowed - catalogue)]
    return found


def check_slo_docs(ctx: Context) -> list[str]:
    """The table "SLOs" of slos.md equals the Sloth files: one row per SLO, Ziel, Schwelle, Alarm.

    Ziel is the objective with one decimal and ` %` (more decimals only where one decimal
    would round it, ruling M1-04/16); Schwelle is `keine` without an `le` matcher in the
    error query, else `<le> s` (the value in any PromQL quotes; a regex or negated `le`
    matcher is a finding); Alarm is `Page, Ticket`, `nur Ticket` with
    `page_alert.disable: true`, `nur Page` with `ticket_alert.disable: true` and `keine`
    with both.
    """
    expected: dict[str, dict[str, str]] = {}
    found: list[str] = []
    for source in sorted((ctx.root / SLO_DIR).glob("*.yaml")):
        spec = yaml.safe_load(source.read_text(encoding="utf-8"))
        for slo in spec["slos"]:
            key = f"{spec['service']}/{slo['name']}"
            columns, problems = _slo_columns(key, slo)
            found += problems
            expected[key] = columns
    rows = markdown_table((ctx.root / SLOS_DOC).read_text(encoding="utf-8"), "SLOs")
    table = f'{SLOS_DOC} "SLOs"'
    by_key: dict[str, list[dict[str, str]]] = {}
    for row in rows:
        first = next(iter(row.values()), "")
        match = re.fullmatch(r"`([^`]+)`", first)
        if match is None or match.group(1) not in expected:
            found.append(f"{first}: row in {table} without an SLO in {SLO_DIR}/")
            continue
        by_key.setdefault(match.group(1), []).append(row)
    for key, columns in expected.items():
        matches = by_key.get(key, [])
        if len(matches) != 1:
            found.append(f"{key}: {len(matches) or 'no'} row{'s' if matches else ''} in {table}")
            continue
        for column, want in columns.items():
            have = matches[0].get(column)
            if have != want:
                found.append(f"{key}: {column} is {have}, the Sloth file says {want}")
    return sorted(found, key=lambda f: f.startswith("`"))


def check_sli_bounds(ctx: Context) -> list[str]:
    """The generated Go and Dart SLO files equal a fresh run of `sli_bounds.py` (VZ-T03).

    The generator compares in its own temporary folder and quotes a diff per differing file;
    a broken label rule is reported too, because then there is no fresh run to compare with.
    """
    return sli_bounds.check(ctx.root)


SLI_METRIC_COLUMNS = ("Metrik der eigenen Messung", "Metric of the own measurement")
"""Header of the column of slos.md "SLOs" that repeats the label `bothy_sli_metric`."""


def check_sli_labels(ctx: Context) -> list[str]:
    """Label rules of 25 3.3, and the metric column of slos.md "SLOs" equals the labels.

    Every SLO carries exactly one `bothy_sli_metric`, its value is an `sli.*` value of the
    CHECK of `stats_minutes.metric`, and every such value belongs to exactly one SLO
    (`sli_bounds.label_findings`). The table row of each SLO names the same metric.
    """
    found = sli_bounds.label_findings(ctx.root)
    labels = {slo.slo: slo.metric for slo in sli_bounds.read_slos(ctx.root)}
    rows = markdown_table((ctx.root / SLOS_DOC).read_text(encoding="utf-8"), "SLOs")
    column = next((c for c in SLI_METRIC_COLUMNS if rows and c in rows[0]), None)
    if column is None:
        names = " or ".join(SLI_METRIC_COLUMNS)
        return [*found, f'{SLOS_DOC} "SLOs" has no column {names}']
    for row in rows:
        match = re.fullmatch(r"`([^`]+)`", next(iter(row.values()), ""))
        if match is None or match.group(1) not in labels:
            continue  # slo-docs reports rows without an SLO
        metric = labels[match.group(1)]
        want = f"`{metric}`" if metric is not None else "no label"
        if row[column] != want:
            found.append(f"{match.group(1)}: {column} is {row[column]}, the Sloth file says {want}")
    return found


def check_alert_thresholds(ctx: Context) -> list[str]:
    """Alerts of alerting.md "Weitere Alarme" equal bothy-alerts.yaml; thresholds follow config.

    A row whose column "Konfigurationswert" is exactly `` `<name>` `` needs the number of the
    final `> <number>` comparison of the alert expression (seconds) to equal the default of
    `<name>` in the configuration reference; ``über `<name>` `` needs it strictly greater. A
    cell that names a configuration value in any other form is a finding.
    """
    alerts: dict[str, str] = {}
    found: list[str] = []
    for name, expr in _alert_rules(ctx.root / HAND_WRITTEN_ALERTS):
        if name in alerts:
            found.append(f"{name}: defined twice in {HAND_WRITTEN_ALERTS}")
        alerts[name] = expr
    rows = markdown_table((ctx.root / ALERTING_DOC).read_text(encoding="utf-8"), "Weitere Alarme")
    table = f'{ALERTING_DOC} "Weitere Alarme"'
    documented = [row["Alarm"].strip("` ") for row in rows]
    for name in sorted(set(documented) - set(alerts)):
        found.append(f"{name}: in {table} but not in {HAND_WRITTEN_ALERTS}")
    for name in sorted(set(alerts) - set(documented)):
        found.append(f"{name}: in {HAND_WRITTEN_ALERTS} but not in {table}")
    for name in sorted({n for n in documented if documented.count(n) > 1}):
        found.append(f"{name}: {documented.count(name)} rows in {table}")
    reference = ctx.root / CONFIG_REFERENCE
    known = {key for key, _ in _config_rows(reference)}
    for row, name in zip(rows, documented, strict=True):
        # A no-break space after "über" still means the strictly-greater form.
        cell = row["Konfigurationswert"].replace("\u00a0", " ").strip()
        equal = re.fullmatch(r"`([^`]+)`", cell)
        over = re.fullmatch(r"über `([^`]+)`", cell)
        match = equal or over
        if match is None:
            # A cell that names a configuration value in any other form would silently lose
            # the coupling of threshold and value; only the two checked forms may name one.
            for key in sorted(
                k for k in known if re.search(rf"(?<![\w.]){re.escape(k)}(?![\w.])", cell)
            ):
                found.append(
                    f"{name}: the cell Konfigurationswert names {key} but is neither "
                    f"`` `{key}` `` nor ``über `{key}` ``"
                )
            continue
        if name not in alerts:
            continue
        config_name = match.group(1)
        operand = _final_greater_operand(alerts[name])
        if operand is None:
            found.append(f"{name}: the expression has no final `> <number>` comparison")
            continue
        if not _PLAIN_NUMBER.fullmatch(operand):
            found.append(f"{name}: the final `>` comparison has no plain number on its right side")
            continue
        threshold = float(operand)
        try:
            raw = config_default(reference, config_name)
            limit = parse_duration(raw)
        except ValueError as exc:
            found.append(f"{name}: {exc}")
            continue
        shown = f"{config_name} ({raw} = {_seconds(limit)} s)"
        where = f"threshold {_seconds(threshold)} s in {HAND_WRITTEN_ALERTS.name}"
        if equal is not None and threshold != limit:
            found.append(f"{name}: {where} must equal {shown}")
        if over is not None and not threshold > limit:
            found.append(f"{name}: {where} must be greater than {shown}")
    return found


# ---- checks: validators -------------------------------------------------------------------


def check_collector(ctx: Context) -> list[str]:
    """`otelcol-contrib validate` for the base, base plus each overlay and every ordered pair.

    The work copy of infra/otel/ reads its secrets from placeholder files (content
    `SECRET_PLACEHOLDER`) instead of /run/bothy/secrets/, gets the variables of
    `COLLECTOR_ENV` and a fixed datadog host name. Pairs leave out the empty overlay and
    run in both orders, because overlays replace lists, so the merged result depends on the
    order of `BOTHY_OTEL_EXPORTER` and `BOTHY_OTEL_EXPORTER_2`.

    The overlay "file" of the Compose smoke test (`SMOKE_COLLECTOR_OVERLAY`) runs with the base
    alone; it never meets an exporter overlay in an installation.

    A reference to a config provider other than `COLLECTOR_PROVIDERS` (`${https:...}`,
    `${s3:...}`, ...), also one nested in another reference, is a finding, and then no
    `validate` runs: the collector resolves such a reference while it validates, and its
    client may bypass the closed proxy.
    """
    remote = _remote_references(ctx.root)
    if remote:
        return remote
    otel = ctx.workdir / "otel"
    shutil.copytree(ctx.root / OTEL_DIR, otel)
    secrets = ctx.workdir / "secrets"
    secrets.mkdir()
    for path in sorted(otel.rglob("*.yaml")):
        text = path.read_text(encoding="utf-8")
        if COLLECTOR_SECRETS_DIR in text:
            path.write_text(text.replace(COLLECTOR_SECRETS_DIR, f"{secrets}/"), encoding="utf-8")
    _pin_datadog_hostname(otel / "exporters" / "datadog.yaml")
    reference = re.compile(rf"\$\{{file:{re.escape(str(secrets))}/({_FILE_NAME})\}}")
    for path in sorted(otel.rglob("*.yaml")):
        for secret in reference.findall(path.read_text(encoding="utf-8")):
            (secrets / secret).write_bytes(SECRET_PLACEHOLDER)

    base = otel / "collector.yaml"
    overlays = sorted((otel / "exporters").glob("*.yaml"))
    if not overlays:
        return [f"no overlay in {OTEL_DIR / 'exporters'}/"]
    smoke = ctx.root / SMOKE_COLLECTOR_OVERLAY
    if not smoke.is_file():
        return [f"{SMOKE_COLLECTOR_OVERLAY} is missing"]
    smoke_copy = ctx.workdir / "otel-smoke" / smoke.name
    smoke_copy.parent.mkdir()
    shutil.copy2(smoke, smoke_copy)
    paired = [o for o in overlays if o.stem != NO_PAIR_OVERLAY]
    runs: list[tuple[str, list[Path]]] = [("base", [base])]
    runs += [(f"base + {o.stem}", [base, o]) for o in overlays]
    runs.append(("base + smoke file", [base, smoke_copy]))
    runs += [
        (f"base + {a.stem} + {b.stem}", [base, a, b]) for a, b in itertools.permutations(paired, 2)
    ]
    binary = ctx.tool("otelcol-contrib")

    def validate(run: tuple[str, list[Path]]) -> list[str]:
        label, files = run
        args = [binary, "validate", *(f"--config={f}" for f in files)]
        return run_command(ctx, args, env=COLLECTOR_ENV, label=label)

    with ThreadPoolExecutor(max_workers=COLLECTOR_WORKERS) as pool:
        return [f for found in pool.map(validate, runs) for f in found]


def check_prometheus_config(ctx: Context) -> list[str]:
    """`promtool check config` on a work copy whose /etc/prometheus/ paths point into it.

    With the prefix rewritten, promtool also loads and checks the rule files the
    configuration names. promtool accepts a `rule_files` glob that matches nothing, and
    Prometheus would then load no rule at all; so every entry must lie under
    /etc/prometheus/ (where Compose mounts infra/observability/prometheus/; a relative entry
    counts from the directory of prometheus.yaml) and match a file, and every file in rules/
    must be loaded.
    """
    work = ctx.workdir / "prometheus"
    shutil.copytree(ctx.root / PROMETHEUS_DIR, work)
    config = work / "prometheus.yaml"
    config.write_text(
        config.read_text(encoding="utf-8").replace(_PROMETHEUS_MOUNT, f"{work}/"),
        encoding="utf-8",
    )
    found = run_command(ctx, [ctx.tool("promtool"), "check", "config", config])
    spec = yaml.safe_load(config.read_text(encoding="utf-8")) or {}
    loaded: set[Path] = set()
    for entry in spec.get("rule_files") or []:
        matches, problem = _mounted_files(
            work, _PROMETHEUS_MOUNT, PROMETHEUS_DIR, "rule_files", entry
        )
        found += [problem] if problem else []
        loaded |= {m.resolve() for m in matches}
    for rule_file in sorted((work / "rules").iterdir()):
        if _visible_file(rule_file) and rule_file.resolve() not in loaded:
            found.append(
                f"{RULES_DIR / rule_file.name} is not loaded by rule_files of "
                f"{PROMETHEUS_DIR / 'prometheus.yaml'}"
            )
    return found


def check_prometheus_rules(ctx: Context) -> list[str]:
    """`promtool check rules` on every file in infra/observability/prometheus/rules/."""
    files = sorted(p for p in (ctx.root / RULES_DIR).iterdir() if _visible_file(p))
    if not files:
        return [f"no rule file in {RULES_DIR}/"]
    return run_command(ctx, [ctx.tool("promtool"), "check", "rules", *files])


def check_prometheus_rule_tests(ctx: Context) -> list[str]:
    """`promtool test rules` on every tests/*.test.yaml; every hand-written alert is tested.

    An alert counts as tested when some test case names it as `alertname` and expects it to
    fire (a non-empty `exp_alerts`); a quiet case alone does not prove the alert works.
    """
    pattern = RULE_TESTS_DIR / "*.test.yaml"
    tests = sorted((ctx.root / RULE_TESTS_DIR).glob("*.test.yaml"))
    if not tests:
        return [f"no unit test file {pattern}"]
    found = run_command(ctx, [ctx.tool("promtool"), "test", "rules", *tests])
    named: set[str] = set()
    firing: set[str] = set()
    for path in tests:
        spec = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
        for case in spec.get("tests") or []:
            for alert_test in case.get("alert_rule_test") or []:
                named.add(str(alert_test.get("alertname")))
                if alert_test.get("exp_alerts"):
                    firing.add(str(alert_test.get("alertname")))
    for name, _ in _alert_rules(ctx.root / HAND_WRITTEN_ALERTS):
        if name not in named:
            found.append(f"{name}: no promtool unit test names it as alertname in {pattern}")
        elif name not in firing:
            found.append(f"{name}: no promtool unit test expects it to fire in {pattern}")
    return found


def check_alertmanager(ctx: Context) -> list[str]:
    """`amtool check-config` and the routes of alerting.md "Stufen und Wege" (`ROUTE_TESTS`).

    Both run on a work copy of infra/observability/alertmanager/ whose /etc/alertmanager/
    paths point into it, so amtool also parses the templates the configuration names.
    amtool accepts a `templates` pattern that matches nothing, and Alertmanager would then
    load no template; so every entry must lie under /etc/alertmanager/ (where Compose mounts
    the directory; a relative entry counts from the directory of alertmanager.yaml) and
    match a file.
    """
    work = ctx.workdir / "alertmanager"
    shutil.copytree(ctx.root / ALERTMANAGER_CONFIG.parent, work)
    config = work / ALERTMANAGER_CONFIG.name
    config.write_text(
        config.read_text(encoding="utf-8").replace(_ALERTMANAGER_MOUNT, f"{work}/"),
        encoding="utf-8",
    )
    amtool = ctx.tool("amtool")
    found = run_command(ctx, [amtool, "check-config", config], label="amtool check-config")
    try:
        spec = yaml.safe_load(config.read_text(encoding="utf-8"))
    except yaml.YAMLError:  # amtool check-config has reported it with its line
        spec = None
    for entry in (spec.get("templates") if isinstance(spec, dict) else None) or []:
        _, problem = _mounted_files(
            work, _ALERTMANAGER_MOUNT, ALERTMANAGER_CONFIG.parent, "templates", entry
        )
        found += [problem] if problem else []
    for receivers, labels in ROUTE_TESTS:
        args = [
            amtool,
            "config",
            "routes",
            "test",
            f"--config.file={config}",
            f"--verify.receivers={receivers}",
            *labels,
        ]
        found += run_command(ctx, args, label=f"route {' '.join(labels)} -> {receivers}")
    return found


def check_loki(ctx: Context) -> list[str]:
    """`loki -verify-config` with the defaults of the `${VAR:-default}` references expanded."""
    config = ctx.root / LOKI_CONFIG
    args = [ctx.tool("loki"), f"-config.file={config}", "-config.expand-env=true", "-verify-config"]
    return run_command(ctx, args)


def check_tempo(ctx: Context) -> list[str]:
    """`tempo -config.verify=true` (Tempo needs the `=true` form) with expanded defaults."""
    config = ctx.root / TEMPO_CONFIG
    args = [
        ctx.tool("tempo"),
        f"-config.file={config}",
        "-config.expand-env=true",
        "-config.verify=true",
    ]
    return run_command(ctx, args)


def check_sloth(ctx: Context) -> list[str]:
    """Each rules/slo-<n>.yaml equals a fresh `sloth generate` of slo/<n>.yaml, byte for byte.

    Every generated file needs its source, and every source its generated file.
    """
    sources = sorted((ctx.root / SLO_DIR).glob("*.yaml"))
    if not sources:
        return [f"no Sloth file in {SLO_DIR}/"]
    sloth = ctx.tool("sloth")
    found: list[str] = []
    for source in sources:
        rel_source = SLO_DIR / source.name
        committed = RULES_DIR / f"slo-{source.stem}.yaml"
        fresh = ctx.workdir / f"slo-{source.stem}.yaml"
        args = [sloth, "generate", "--no-log", "-i", source, "-o", fresh]
        problems = run_command(ctx, args, label=f"sloth generate {rel_source}")
        if problems:
            found += problems
        elif not (ctx.root / committed).is_file():
            found.append(f"{committed} is missing; generate it from {rel_source}")
        elif fresh.read_bytes() != (ctx.root / committed).read_bytes():
            found.append(
                f"{committed} differs from a fresh `sloth generate` of {rel_source}"
                + _diff_tail((ctx.root / committed).read_text(encoding="utf-8"), fresh)
            )
    for generated in sorted((ctx.root / RULES_DIR).glob("slo-*.yaml")):
        source_name = generated.name.removeprefix("slo-")
        if not (ctx.root / SLO_DIR / source_name).is_file():
            found.append(
                f"{RULES_DIR / generated.name} has no Sloth source {SLO_DIR / source_name}"
            )
    return found


def check_tofu(ctx: Context) -> list[str]:
    """`tofu init -backend=false` and `tofu validate` for each module of infra/tofu/modules/.

    A module is a directory with a configuration file OpenTofu reads (`TOFU_FILES`).
    The modules run in one work copy of infra/tofu/, so a module may use a sibling as
    module source. Each runs with a CLI configuration of its own, so no mirror,
    credential or plugin cache of the caller is used. Its only provider source is the local
    mirror of `tofu_providers.py` (tools/bin), filled before any validator runs with the
    archives the lock files of infra/tofu/test/ pin, each checked against its `zh:` hash: an
    empty configuration would let `tofu init` download providers from the registry, a network
    call the validator must not make. A module with a provider the lock files do not pin
    therefore fails here (docs/DECISIONS.md D-M7-16-2). The unpacked providers are shared
    through a plugin cache in the work directory.
    """
    modules = sorted(
        d
        for d in (ctx.root / TOFU_MODULES).iterdir()
        if d.is_dir() and any(any(d.glob(pattern)) for pattern in TOFU_FILES)
    )
    if not modules:
        *others, last = (pattern.removeprefix("*") for pattern in TOFU_FILES)
        kinds = f"{', '.join(others)} or {last}"
        return [f"no OpenTofu module (a directory with {kinds} files) under {TOFU_MODULES}/"]
    try:
        mirror = ctx.provider_mirror()
    except (tofu_providers.ProviderError, OSError, ValueError, KeyError) as exc:
        return [f"provider mirror: {exc}"]
    plugins = ctx.workdir / "plugin-cache"
    plugins.mkdir()
    cli_config = ctx.workdir / "offline.tfrc"
    cli_config.write_text(tofu_providers.cli_config(mirror, plugins), encoding="utf-8")
    env = {"TF_IN_AUTOMATION": "1", "TF_CLI_CONFIG_FILE": str(cli_config)}
    tofu = ctx.tool("tofu")
    found: list[str] = []
    tree = ctx.workdir / "tofu"
    shutil.copytree(ctx.root / TOFU_DIR, tree, ignore=shutil.ignore_patterns(".terraform"))
    for module in modules:
        copy = tree / "modules" / module.name
        init = [tofu, "init", "-backend=false", "-input=false", "-no-color"]
        problems = run_command(ctx, init, env=env, cwd=copy, label=f"{module.name}: tofu init")
        if not problems:
            validate = [tofu, "validate", "-no-color"]
            label = f"{module.name}: tofu validate"
            problems = run_command(ctx, validate, env=env, cwd=copy, label=label)
        found += problems
    return found


CHECKS: Mapping[str, Check] = {
    "tool-versions": check_tool_versions,
    "collector": check_collector,
    "collector-allowlist": check_collector_allowlist,
    "prometheus-config": check_prometheus_config,
    "prometheus-rules": check_prometheus_rules,
    "prometheus-rule-tests": check_prometheus_rule_tests,
    "alertmanager": check_alertmanager,
    "loki": check_loki,
    "tempo": check_tempo,
    "sloth": check_sloth,
    "slo-docs": check_slo_docs,
    "sli-bounds": check_sli_bounds,
    "sli-labels": check_sli_labels,
    "alert-thresholds": check_alert_thresholds,
    "tofu": check_tofu,
}
"""Every check by name, in the order of the table in PLAN M1-04 "Design notes"."""


# ---- CLI ----------------------------------------------------------------------------------


def main(argv: Sequence[str] | None = None) -> int:
    """Command line entry point; see the module docstring for options and exit codes.

    For the run, each of `STOP_SIGNALS` stops it the same way: the running validators are
    killed by their own process id, the work directory is removed, and the exit status is
    128 plus the signal number (130 for SIGINT, 143 for SIGTERM). The previous handlers
    are restored afterwards.
    """
    parser = argparse.ArgumentParser(description=(__doc__ or "").splitlines()[0])
    parser.add_argument("--root", type=Path, default=REPO_ROOT, help="repository root to check")
    parser.add_argument(
        "--only", nargs="+", action="extend", metavar="CHECK", help="run only these checks"
    )
    parser.add_argument("--list", action="store_true", help="print the check names and exit")
    args = parser.parse_args(argv)
    if args.list:
        print("\n".join(CHECKS))
        return 0
    previous = {signum: signal.signal(signum, _on_stop_signal) for signum in STOP_SIGNALS}
    try:
        results = run_checks(args.root, args.only, on_result=_print_result)
        return 1 if any(r.findings for r in results) else 0
    except ValueError as exc:
        print(f"opscheck: {exc}", file=sys.stderr)
        return 2
    except _Stopped as stop:
        print(f"opscheck: stopped by {signal.Signals(stop.signum).name}", file=sys.stderr)
        return 128 + stop.signum
    finally:
        for signum, handler in previous.items():
            # None: the handler was not set from Python; the default is the best match.
            signal.signal(signum, signal.SIG_DFL if handler is None else handler)
        _RUNNING.reset()


# ---- internals ----------------------------------------------------------------------------


class _Stopped(BaseException):
    """A stop signal arrived; like KeyboardInterrupt, no `except Exception` catches it."""

    def __init__(self, signum: int) -> None:
        super().__init__(signum)
        self.signum = signum


class _Validators:
    """The validator processes running now, so a stop signal can kill them by their PID.

    Starting and registering a process and recording a stop happen under one lock, so no
    validator starts unseen after a stop. The lock is reentrant because the signal handler
    runs in the main thread, possibly while that thread holds the lock in `start`.
    """

    def __init__(self) -> None:
        self._lock = threading.RLock()
        self._running: set[subprocess.Popen[str]] = set()
        self._stopped_by: int | None = None
        self._main_starting = False

    def start(self, args: list[str], *, env: Mapping[str, str], cwd: Path) -> subprocess.Popen[str]:
        """Start and register one validator; raises `_Stopped` after a stop signal."""
        with self._lock:
            self._raise_if_stopped()
            self._main_starting = threading.current_thread() is threading.main_thread()
            try:
                proc = subprocess.Popen(
                    args,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    text=True,
                    errors="replace",
                    env=env,
                    cwd=cwd,
                )
                self._running.add(proc)
            finally:
                self._main_starting = False
            if self._stopped_by is not None:  # the stop arrived while this process started
                self._running.discard(proc)
                with proc:  # closes the pipes and waits for the killed process
                    proc.kill()
                self._raise_if_stopped()
        return proc

    def finish(self, proc: subprocess.Popen[str]) -> None:
        """Forget a process that has ended."""
        with self._lock:
            self._running.discard(proc)

    def kill_all(self) -> None:
        """Kill every registered process; its `run_command` then reports exit status -9."""
        with self._lock:
            for proc in self._running:
                proc.kill()

    def stop(self, signum: int) -> None:
        """Record a stop, kill every process and raise `_Stopped` in the main thread.

        If the main thread is just starting a process, `start` raises once that process is
        registered and killed. A further signal during the stop only kills again.
        """
        with self._lock:
            first = self._stopped_by is None
            if first:
                self._stopped_by = signum
            self.kill_all()
            if not first or self._main_starting:
                return
        raise _Stopped(signum)

    def reset(self) -> None:
        """Forget a stop after `main` returned, so a later run in this process may start."""
        with self._lock:
            self._stopped_by = None

    def _raise_if_stopped(self) -> None:
        if self._stopped_by is not None:
            raise _Stopped(self._stopped_by)


_RUNNING = _Validators()
"""Every validator `run_command` started and that has not ended yet."""


def _on_stop_signal(signum: int, _frame: object) -> None:
    _RUNNING.stop(signum)


def _print_result(result: CheckResult) -> None:
    print(f"{result.name}: {'FAIL' if result.findings else 'ok'}", flush=True)
    for finding in result.findings:
        for line in finding.splitlines():
            print(f"  {line}", flush=True)


def _select(only: Sequence[str] | None) -> list[str]:
    if only is None:
        return list(CHECKS)
    unknown = [n for n in only if n not in CHECKS]
    if unknown:
        raise ValueError(f"unknown check: {', '.join(unknown)} (see --list)")
    return [n for n in CHECKS if n in only]


def _base_env(workdir: Path) -> dict[str, str]:
    """Environment of every validator: `PATH`, `HOME`, `TMPDIR`, a closed proxy and metadata host.

    `PATH`, `HOME` and `TMPDIR` are directories inside the work directory. `PATH` is empty:
    no validator may start another program found through the caller's search path. The
    proxy variables point at `NO_NETWORK_PROXY`, the variables of `NO_NETWORK_METADATA` at
    the same closed port.
    """
    env = {}
    for variable, directory in (("PATH", "bin"), ("HOME", "home"), ("TMPDIR", "tmp")):
        path = workdir / directory
        path.mkdir(exist_ok=True)
        env[variable] = str(path)
    for variable in ("HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy"):
        env[variable] = NO_NETWORK_PROXY
    env.update(NO_NETWORK_METADATA)
    return env


def _display(ctx: Context, args: Sequence[object]) -> str:
    """The command as text: binary by name, work and repository paths shortened."""
    shown = [Path(str(args[0])).name] + [str(a) for a in args[1:]]
    text = " ".join(shown)
    return text.replace(str(ctx.workdir), "<work>").replace(str(ctx.root), "<root>")


def _unfenced(text: str) -> Iterator[str]:
    """Lines of a Markdown text; lines inside code fences are replaced by empty lines.

    As in CommonMark, a fence of backticks or tildes ends only with a line of the same
    character, at least as long and with nothing after it. A fence may be indented, as it is
    inside a list item.
    """
    opened = ""
    for line in text.splitlines():
        match = _FENCE.match(line)
        if not opened:
            if match is not None and not (match.group(1)[0] == "`" and "`" in match.group(2)):
                opened = match.group(1)
                yield ""
                continue
            yield line
            continue
        closing = match is not None and not match.group(2).strip()
        if closing and match is not None:
            fence = match.group(1)
            if fence[0] == opened[0] and len(fence) >= len(opened):
                opened = ""
        yield ""


def _heading_title(line: str) -> str | None:
    """Title of a Markdown heading without section number and trailing parenthesis."""
    match = _HEADING.match(line)
    if match is None:
        return None
    title = _SECTION_NUMBER.sub("", match.group(2))
    return _TRAILING_PARENTHESIS.sub("", title)


def _cells(line: str) -> list[str]:
    inner = line.strip()
    inner = inner.removeprefix("|")
    if inner.endswith("|") and not inner.endswith("\\|"):
        inner = inner[:-1]
    return [cell.strip().replace("\\|", "|") for cell in _CELL_SPLIT.split(inner)]


def _is_table_start(lines: list[str], index: int) -> bool:
    if index + 1 >= len(lines) or not lines[index].lstrip().startswith("|"):
        return False
    separator = _cells(lines[index + 1])
    return bool(separator) and all(_SEPARATOR_CELL.match(c) for c in separator)


def _table_end(lines: list[str], start: int) -> int:
    end = start
    while end < len(lines) and lines[end].lstrip().startswith("|"):
        end += 1
    return end


def _parse_table(lines: list[str], start: int) -> list[dict[str, str]]:
    if not _is_table_start(lines, start):
        raise ValueError(f"line {start + 1}: a table needs a header and a separator row")
    header = _cells(lines[start])
    rows = []
    for number in range(start + 2, _table_end(lines, start)):
        cells = _cells(lines[number])
        if len(cells) != len(header):
            raise ValueError(
                f"line {number + 1}: {len(cells)} cells, the header has {len(header)} cells"
            )
        rows.append(dict(zip(header, cells, strict=True)))
    return rows


def _slo_columns(key: str, slo: Mapping[str, Any]) -> tuple[dict[str, str], list[str]]:
    """Expected cells Ziel, Schwelle and Alarm of one Sloth SLO, and problems reading it.

    Schwelle is left out when the `le` matchers of the error query name no single bound, so
    the problem is reported once instead of also as a wrong cell.
    """
    problems: list[str] = []
    query = ((slo.get("sli") or {}).get("events") or {}).get("error_query")
    if query is None:
        problems.append(f"{key}: no sli.events.error_query; slo-docs reads event SLIs only")
    bounds: set[str] = set()
    for matcher in _LE_MATCHER.finditer(str(query or "")):
        operator, *values = matcher.groups()
        if operator != "=":
            problems.append(
                f"{key}: le matcher {matcher.group(0)} in the error query is no bound; "
                'slo-docs reads le="<bound>" only'
            )
            continue
        bounds.update(v for v in values if v is not None)
    if len(bounds) > 1:
        problems.append(f"{key}: several le bounds {', '.join(sorted(bounds))} in the error query")
    alerting = slo.get("alerting") or {}
    page, ticket = (
        (alerting.get(kind) or {}).get("disable") is not True
        for kind in ("page_alert", "ticket_alert")
    )
    columns = {
        "Ziel": f"{_objective(float(slo['objective']))} %",
        "Schwelle": f"{min(bounds)} s" if bounds else _NO_VALUE,
        "Alarm": _ALARM_CELLS[page, ticket],
    }
    if problems and query is not None:
        del columns["Schwelle"]
    return columns, problems


def _promql_code(expr: str) -> str:
    """The expression without comments and with empty string literals.

    A `#` outside a string starts a comment up to the end of the line; a `>` inside a label
    value or a comment is no comparison.
    """
    code: list[str] = []
    quote = ""
    index = 0
    while index < len(expr):
        char = expr[index]
        if quote:
            if char == "\\" and quote != "`":
                index += 1
            elif char == quote:
                code.append(char)
                quote = ""
        elif char in "\"'`":
            code.append(char)
            quote = char
        elif char == "#":
            end = expr.find("\n", index)
            index = len(expr) if end < 0 else end
            continue
        else:
            code.append(char)
        index += 1
    return "".join(code)


def _final_greater_operand(expr: str) -> str | None:
    """Right side of the last `>` comparison, up to `)`, `and`, `or`, `unless` or the end."""
    code = _promql_code(expr)
    comparisons = list(_GREATER.finditer(code))
    if not comparisons:
        return None
    rest = code[comparisons[-1].end() :]
    end = _OPERAND_END.search(rest)
    return (rest[: end.start()] if end else rest).strip()


def _objective(value: float) -> str:
    """One decimal (`99.9`, `95.0`), more where one would round the objective (`99.95`)."""
    text = f"{value:.1f}"
    return text if float(text) == value else repr(value)


def _env_value(raw: str) -> str:
    """A value of an env file as Compose reads it: quotes removed, a ` #` comment dropped.

    A quoted value ends at its closing quote; what follows (a comment) does not count.
    """
    value = raw.strip()
    if value[:1] in ('"', "'"):
        end = value.find(value[0], 1)
        if end > 0:
            return value[1:end]
    return re.split(r"\s+#", value, maxsplit=1)[0].strip()


def _allowlist_structure(root: Path, base: Mapping[str, Any]) -> list[str]:
    """Findings where telemetry could reach an exporter without `redaction/allowlist`."""
    files: list[tuple[Path, Mapping[str, Any]]] = [(OTEL_DIR / "collector.yaml", base)]
    overlays = [
        OTEL_DIR / "exporters" / o.name for o in (root / OTEL_DIR / "exporters").glob("*.yaml")
    ]
    if (root / SMOKE_COLLECTOR_OVERLAY).is_file():
        overlays.append(SMOKE_COLLECTOR_OVERLAY)
    for rel in sorted(overlays):
        spec = yaml.safe_load((root / rel).read_text(encoding="utf-8")) or {}
        files.append((rel, spec))
    receivers = set()
    for _, spec in files:
        receivers |= set((spec.get("receivers") or {}).keys())
    base_pipelines = set(((base.get("service") or {}).get("pipelines") or {}).keys())
    found: list[str] = []
    for rel, spec in files:
        pipelines = (spec.get("service") or {}).get("pipelines") or {}
        if spec is not base:
            if "redaction/allowlist" in (spec.get("processors") or {}):
                found.append(f"{rel} redefines processors.redaction/allowlist")
            for name in sorted(set(pipelines) & base_pipelines):
                found.append(f"{rel} redefines the base pipeline {name}")
        for name, pipeline in sorted(pipelines.items()):
            if name.split("/")[0] not in ("traces", "logs"):
                continue
            sources = [r for r in (pipeline or {}).get("receivers") or [] if r in receivers]
            if sources and "redaction/allowlist" not in ((pipeline or {}).get("processors") or []):
                found.append(
                    f"pipeline {name} of {rel} receives from {', '.join(sources)} but does not "
                    "run redaction/allowlist"
                )
    return found


def _alert_rules(path: Path) -> Iterator[tuple[str, str]]:
    """(alert name, expression) of every alerting rule of a Prometheus rule file."""
    spec = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
    for group in spec.get("groups") or []:
        for rule in group.get("rules") or []:
            if "alert" in rule:
                yield str(rule["alert"]), str(rule.get("expr", ""))


def _remote_references(root: Path) -> list[str]:
    """Findings for references to a config provider outside `COLLECTOR_PROVIDERS`.

    In a file PyYAML reads, only its strings count (keys and values), not comments; a file
    it cannot read is searched as text, because the collector may still read it. `$$` is
    the escaped `$`, so `$${https:...}` is text. One finding per file and provider.
    """
    *others, last = sorted(COLLECTOR_PROVIDERS)
    allowed = f"{', '.join(others)} and {last}"
    found: list[str] = []
    paths = sorted((root / OTEL_DIR).rglob("*.yaml"))
    if (root / SMOKE_COLLECTOR_OVERLAY).is_file():
        paths.append(root / SMOKE_COLLECTOR_OVERLAY)
    for path in paths:
        text = path.read_text(encoding="utf-8")
        try:
            texts = list(_yaml_strings(yaml.safe_load(text)))
        except yaml.YAMLError:
            texts = [text]
        schemes = {m.group(1) for s in texts for m in _CONFMAP_REFERENCE.finditer(s)}
        found += [
            f"{path.relative_to(root)}: ${{{scheme}:...}} uses the config provider {scheme}, "
            f"which is not allowed; collector files may use only {allowed} references "
            "(most other providers fetch their value over the network)"
            for scheme in sorted(schemes - COLLECTOR_PROVIDERS - {None})
        ]
    return found


def _yaml_strings(node: object) -> Iterator[str]:
    """Every string of a parsed YAML document, keys and values, depth first."""
    if isinstance(node, str):
        yield node
    elif isinstance(node, dict):
        for key, value in node.items():
            yield from _yaml_strings(key)
            yield from _yaml_strings(value)
    elif isinstance(node, list):
        for item in node:
            yield from _yaml_strings(item)


def _pin_datadog_hostname(path: Path) -> None:
    """Set `exporters.datadog.hostname` in the work copy of the datadog overlay.

    Without a host name, `validate` with this overlay waits about 25 seconds for host name
    detection, which probes cloud metadata endpoints: a network call the gate must not
    make. Only the work copy changes; the overlay in the repository keeps automatic host
    detection for production.
    """
    if not path.is_file():
        return
    spec = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
    exporter = (spec.get("exporters") or {}).get("datadog")
    if isinstance(exporter, dict):
        exporter["hostname"] = DATADOG_VALIDATE_HOSTNAME
        path.write_text(yaml.safe_dump(spec, sort_keys=False), encoding="utf-8")


def _diff_tail(committed: str, fresh: Path) -> str:
    """Up to `OUTPUT_TAIL_LINES` lines of a unified diff, indented for a finding."""
    diff = difflib.unified_diff(
        committed.splitlines(),
        fresh.read_text(encoding="utf-8").splitlines(),
        "committed",
        "generated",
        lineterm="",
    )
    return "".join(f"\n    {line}" for line in itertools.islice(diff, OUTPUT_TAIL_LINES))


def _seconds(value: float) -> str:
    """Seconds as written in a rule: no exponent, no `.0` for whole numbers."""
    return f"{value:.0f}" if value.is_integer() else f"{value:f}".rstrip("0")


def _mounted_files(
    work: Path, mount: str, source: Path, key: str, entry: object
) -> tuple[list[Path], str | None]:
    """Files that one path entry of a configuration matches in the work copy of its directory.

    Args:
        work: work copy of the directory Compose mounts at `mount`; the configuration file
            lies in it, and its `mount` prefixes already point at `work`.
        mount: the container path of the directory, with a trailing slash.
        source: the directory in the repository, for the finding.
        key: the configuration key of the entry (`rule_files`, `templates`), for the finding.
        entry: the entry as written; a relative one counts from the configuration's directory.

    Returns:
        The matching regular files and a finding, or `None` if the entry lies under the
        mount and matches at least one file.
    """
    shown = str(entry).replace(f"{work}/", mount)
    pattern = posixpath.normpath(str(entry) if posixpath.isabs(str(entry)) else f"{work}/{entry}")
    if not pattern.startswith(f"{work}/"):
        problem = (
            f"{key} entry {shown} lies outside {mount}, where Compose mounts {source}/; "
            "the gate cannot check it"
        )
        return [], problem
    matches = [m for m in work.glob(pattern.removeprefix(f"{work}/")) if m.is_file()]
    return matches, None if matches else f"{key} entry {shown} matches no file"


def _visible_file(path: Path) -> bool:
    """Regular file whose name does not start with a dot (editor and system files)."""
    return path.is_file() and not path.name.startswith(".")


def _cli() -> None:
    """Entry of `python opscheck.py`: run `main` and exit with its status.

    After a stop by SIGINT the script ends by SIGINT itself, as Python does after an
    uncaught KeyboardInterrupt, so a calling shell stops as well instead of going on.
    """
    status = main()
    if status == 128 + signal.SIGINT:
        signal.signal(signal.SIGINT, signal.SIG_DFL)
        os.kill(os.getpid(), signal.SIGINT)
    sys.exit(status)


if __name__ == "__main__":
    _cli()
