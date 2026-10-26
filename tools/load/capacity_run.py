# SPDX-License-Identifier: Apache-2.0
r"""Runner of the capacity load test: ``make load-capacity`` (brief M7-18).

Measures the capacity table of docs/spec/16-operations.md section 13 on the full stack installed
from the images, by the installation path of the portability test (16 11.1, M7-14), and writes a
report. In this order:

1. the interface of ``make portability`` (``PORTABILITY_TAG``, ``PORTABILITY_INSTALL_SOURCE``,
   ``PORTABILITY_RELEASE_DIR``, ``PORTABILITY_TRIAL_PUBLIC_KEY``) names the release; the thresholds
   come from the tables of the specification (02 section 22, the configuration reference);
2. the images are pulled and their signatures verified, ``install.sh`` installs with
   ``--storage fs`` and the options of ``tests/portability/install-options``, and the overlay
   ``local`` starts Prometheus;
3. the profile (ADR 0063): the Docker runtime's vCPUs and memory against
   ``perf.reference_profile`` give the mode ``match``, ``emulated`` or ``below``; the servers get
   ``GOMAXPROCS`` and ``cpus`` of the profile, PostgreSQL ``cpus`` and ``mem_limit``;
4. the driver ``bothy-load`` (``services/server/test/load``) runs next to the stack in the network
   of the Docker host: it seeds its world while the server runs with raised rate limits, then the
   server restarts with its defaults and the driver runs the ramp (tests/load/README.md);
5. per step, Prometheus gives the counter lock wait, the database backends and pool, the gateway
   connections, the fan-out lag and the events per second; ``docker stats`` gives the memory of
   every container; at the end PostgreSQL gives the size of every table and index;
6. ``LOAD_REPORT_DIR`` (default ``.cache/load``) receives ``capacity-report.json`` and
   ``capacity-report.md``: every row of 16 13 with metric, measured value, lever and conditions,
   the inputs of the disk space formula, and the profile, commit and date.

The installation, its volumes, the driver binary and the temporary directory are removed at the
end, also after a failure (``docker compose ps --all`` and the logs are printed first).

Exit codes: 0 the report was written, 1 a step failed, 2 an invalid interface value or a missing
precondition. Progress lines go to stderr as ``load: <step>``; no secret is ever printed.

Example::

    PORTABILITY_TAG=v0.1.0 PORTABILITY_INSTALL_SOURCE=release-download \\
        PORTABILITY_RELEASE_DIR=/tmp/rel uv run --project tools python tools/load/capacity_run.py
"""

from __future__ import annotations

import datetime as dt
import json
import math
import os
import re
import secrets
import shutil
import subprocess
import sys
import tempfile
import threading
import time
from collections.abc import Mapping, Sequence
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Any
from urllib.parse import urlencode

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "tools" / "portability"))
import portability_run as pr  # noqa: E402 - the installation path of the portability test

ARCHITECTURE = ROOT / "docs" / "spec" / "02-architecture.md"
CONFIGURATION = ROOT / "docs" / "operations" / "configuration.md"
REPORT_DIR = ROOT / ".cache" / "load"
SERVER_MODULE = ROOT / "services" / "server"

# Values of the run (named here, the only place). The rate limits of the seeding phase: the owner
# makes two administrative calls per person, so the seeding raises these classes and the ramp runs
# with the defaults. The export interval of the server's metrics, so a step window holds several
# samples. The interval of the memory samples, the bound of the whole driver run, and the share of
# the profile's memory a runtime must report (the kernel keeps a part, as report.hasMemory).
SEED_LIMIT = 1_000_000
SEED_CLASSES = ("ADMIN", "PUBLIC_AUTH", "WRITE", "READ", "GATEWAY_CONNECT")
METRIC_EXPORT_MS = 5000
STATS_INTERVAL_S = 5
DRIVER_TIMEOUT_S = 4 * 3600
MEMORY_SHARE = 0.9
SERVICES = ("api", "worker", "postgres", "clamav")
# The services of the overlay local the run needs (docs/observability/backends.md).
METRICS_STACK = ("prometheus", "otel-collector")
# The place of the load test values in the tables of chapter 02 section 22.
LOAD_TEST = "Load test"

Runner = pr.Runner


def say(message: str) -> None:
    """Write one progress line to stderr."""
    print(f"load: {message}", file=sys.stderr, flush=True)


# ---- values of the specification ---------------------------------------------------------------


def table_cells(text: str, name: str, place: str | None = None) -> list[str]:
    """The cells of the table row whose first cell is ``name`` in backticks, unquoted.

    With ``place`` only a row whose second cell is that place counts (chapter 02 names the
    performance targets in section 17 too, the values stand in the rows of place ``Load test`` of
    section 22).

    Raises:
        pr.StepError: No row names it.
    """
    for line in text.splitlines():
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        if not line.startswith("|") or cells[0] != f"`{name}`":
            continue
        if place is None or (len(cells) > 1 and cells[1] == place):
            return [c.strip("`") for c in cells]
    raise pr.StepError(f"no table row names {name}")


@dataclass(frozen=True)
class Values:
    """The thresholds and the profile of the run, read from the specification.

    Attributes:
        profile: ``perf.reference_profile`` as written.
        cpus: The vCPUs of the profile.
        memory: The memory of the profile in bytes.
        message_create_p99: ``perf.message_create_p99`` as written (a Go duration).
        fanout_p99: ``perf.fanout_local_p99`` as written.
        messages_per_minute: The default of ``ratelimit.rl.message.per_minute``.
    """

    profile: str
    cpus: int
    memory: int
    message_create_p99: str
    fanout_p99: str
    messages_per_minute: int


def parse_profile(profile: str) -> tuple[int, int]:
    """The vCPUs and the memory in bytes of ``perf.reference_profile``.

    Example: ``"4 vCPU, 8 GiB RAM, SSD"`` gives ``(4, 8589934592)``.

    Raises:
        pr.StepError: The text names no vCPU count or no memory in GiB.
    """
    cpu = re.search(r"(\d+)\s*vCPU", profile)
    mem = re.search(r"(\d+)\s*GiB", profile)
    if cpu is None or mem is None:
        raise pr.StepError(f"perf.reference_profile {profile!r} names no vCPU and GiB")
    return int(cpu.group(1)), int(mem.group(1)) << 30


def read_values(architecture: str, configuration: str) -> Values:
    """Read the values of the run from chapter 02 section 22 and the configuration reference."""
    profile = table_cells(architecture, "perf.reference_profile", LOAD_TEST)[2]
    cpus, memory = parse_profile(profile)
    per_minute = table_cells(configuration, "ratelimit.rl.message.per_minute")[4]
    if not per_minute.isdigit():
        raise pr.StepError(f"ratelimit.rl.message.per_minute default {per_minute!r} is no count")
    return Values(
        profile,
        cpus,
        memory,
        table_cells(architecture, "perf.message_create_p99", LOAD_TEST)[2],
        table_cells(architecture, "perf.fanout_local_p99", LOAD_TEST)[2],
        int(per_minute),
    )


def profile_mode(runtime_cpus: int, runtime_memory: int, cpus: int, memory: int) -> str:
    """The mode of ADR 0063 point 3 for the Docker runtime against the profile.

    ``below`` when the runtime has fewer vCPUs or less than ``MEMORY_SHARE`` of the memory,
    ``match`` with exactly the vCPUs, ``emulated`` with more (the limits of the override apply).
    """
    if runtime_cpus < cpus or runtime_memory < memory * MEMORY_SHARE:
        return "below"
    return "match" if runtime_cpus == cpus else "emulated"


# ---- the installation's settings for the run ---------------------------------------------------


def override(seed: bool, cpus: int, memory: int, trusted: str) -> dict[str, Any]:
    """The Compose override of the run (written as JSON, which Compose reads as YAML).

    The servers get ``GOMAXPROCS`` and ``cpus`` of the profile, PostgreSQL ``cpus`` and
    ``mem_limit`` (ADR 0063 point 3). ``api`` trusts the forwarding headers of ``trusted`` (the
    network of the published port), so every simulated person is its own client address, as
    behind the reverse proxy of a deployment (16 4.3). With ``seed`` the rate limits of
    ``SEED_CLASSES`` are raised for the seeding.
    """
    server = {"GOMAXPROCS": str(cpus)}
    api = dict(server, BOTHY_SERVER__TRUSTED_PROXIES=trusted)
    if seed:
        for cls in SEED_CLASSES:
            api[f"BOTHY_RATELIMIT__RL__{cls}__PER_MINUTE"] = str(SEED_LIMIT)
            api[f"BOTHY_RATELIMIT__RL__{cls}__BURST"] = str(SEED_LIMIT)
    return {
        "services": {
            "api": {"environment": api, "cpus": cpus},
            "worker": {"environment": server, "cpus": cpus},
            "postgres": {"cpus": cpus, "mem_limit": memory},
        }
    }


# ---- measurements ------------------------------------------------------------------------------


def prom_queries(seconds: int) -> dict[str, str]:
    """The PromQL of the server metrics of one step window of ``seconds`` (docs signals.md).

    Each is evaluated at the end of the window; the maxima run over a subquery of 5 s.
    """
    w = f"{seconds}s"
    lock = "bothy_db_event_seq_lock_wait_seconds"
    api_pool = '{bothy_pool_state="acquired",bothy_server_role="api"}'
    return {
        "events_per_s": f"sum(increase(bothy_event_appended_total[{w}])) / {seconds}",
        "lock_wait_p99_s": (
            f"histogram_quantile(0.99, sum by (le) (increase({lock}_bucket[{w}])))"
        ),
        "lock_wait_mean_s": (f"sum(increase({lock}_sum[{w}])) / sum(increase({lock}_count[{w}]))"),
        "backends_max": f"max_over_time(sum(postgresql_backends)[{w}:5s])",
        "pool_acquired_max": f"max_over_time(sum(bothy_db_pool_connections{api_pool})[{w}:5s])",
        "gateway_connections_max": (
            f"max_over_time(sum(bothy_gateway_connections_active)[{w}:5s])"
        ),
        "fanout_lag_max_s": f"max_over_time(max(bothy_event_fanout_lag_seconds)[{w}:5s])",
    }


def prom_value(body: str) -> float | None:
    """The single value of a Prometheus instant query answer, None without one or for NaN."""
    data = json.loads(body)
    if data.get("status") != "success":
        return None
    result = data.get("data", {}).get("result", [])
    if len(result) != 1:
        return None
    value = float(result[0]["value"][1])
    return None if math.isnan(value) or math.isinf(value) else value


_UNITS = {"B": 1, "KiB": 1 << 10, "MiB": 1 << 20, "GiB": 1 << 30, "kB": 1000, "MB": 10**6}


def parse_mem(text: str) -> int:
    """Bytes of the first value of a ``docker stats`` MemUsage cell (``"81.2MiB / 1GiB"``).

    Raises:
        ValueError: The cell holds no value with a unit.
    """
    m = re.match(r"\s*([0-9.]+)\s*([A-Za-z]+)", text)
    if m is None or m.group(2) not in _UNITS:
        raise ValueError(f"no memory value in {text!r}")
    return int(float(m.group(1)) * _UNITS[m.group(2)])


@dataclass
class MemorySampler:
    """Samples the memory and CPU of the services of the installation with ``docker stats``.

    Attributes:
        containers: Container name to service.
        samples: (UNIX time, service, bytes, CPU percent of one core) in order.
    """

    containers: Mapping[str, str]
    run: Runner = pr._run
    samples: list[tuple[float, str, int, float]] = field(default_factory=list)
    _stop: threading.Event = field(default_factory=threading.Event)
    _thread: threading.Thread | None = None

    def sample(self) -> None:
        """Take one sample of every container."""
        res = self.run(
            ["docker", "stats", "--no-stream", "--format",
             "{{.Name}}\t{{.MemUsage}}\t{{.CPUPerc}}", *self.containers],
            capture_output=True,
        )  # fmt: skip
        now = time.time()
        for line in res.stdout.splitlines():
            name, mem, cpu = ([*line.split("\t"), "", ""])[:3]
            if name in self.containers:
                try:
                    cpu_pct = float(cpu.strip().rstrip("%"))
                    self.samples.append((now, self.containers[name], parse_mem(mem), cpu_pct))
                except ValueError:
                    continue

    def start(self) -> None:
        """Sample every ``STATS_INTERVAL_S`` until stop."""

        def loop() -> None:
            while not self._stop.wait(STATS_INTERVAL_S):
                self.sample()

        self._thread = threading.Thread(target=loop, daemon=True)
        self._thread.start()

    def stop(self) -> None:
        """End the sampling."""
        self._stop.set()
        if self._thread is not None:
            self._thread.join()

    def peak(self, service: str, start: float, end: float) -> int | None:
        """The largest memory sample of service in [start, end], None without one."""
        vals = [b for t, s, b, _ in self.samples if s == service and start <= t <= end]
        return max(vals) if vals else None

    def cpu_peak(self, service: str, start: float, end: float) -> float | None:
        """The largest CPU sample of service in [start, end] in percent of one core."""
        vals = [c for t, s, _, c in self.samples if s == service and start <= t <= end]
        return max(vals) if vals else None


SIZES_SQL = """
SELECT c.relname, pg_total_relation_size(c.oid), pg_indexes_size(c.oid)
FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind IN ('r', 'p') AND n.nspname = 'public' AND NOT c.relispartition
ORDER BY 2 DESC
"""

# Partitions of event_log count with their parent; pg_total_relation_size of a partitioned table
# is 0, so the parent's row sums its partitions.
EVENT_LOG_SQL = """
SELECT coalesce(sum(pg_total_relation_size(i.inhrelid)), 0),
       coalesce(sum(pg_indexes_size(i.inhrelid)), 0)
FROM pg_inherits i WHERE i.inhparent = 'event_log'::regclass
"""

MEANS_SQL = """
SELECT (SELECT count(*) FROM messages),
       (SELECT coalesce(avg(pg_column_size(m.*)), 0) FROM messages m),
       (SELECT count(*) FROM event_log),
       (SELECT coalesce(avg(pg_column_size(e.payload)), 0) FROM event_log e),
       pg_database_size(current_database()),
       current_setting('fsync'), current_setting('synchronous_commit'),
       current_setting('server_version')
"""


@dataclass(frozen=True)
class TableSize:
    """The size of one table with its indexes (bytes) and of its indexes alone."""

    name: str
    total: int
    indexes: int


def parse_sizes(rows: str, event_log: str) -> list[TableSize]:
    """The tables from the ``psql -At -F '|'`` rows of SIZES_SQL and EVENT_LOG_SQL."""
    out = []
    ev_total, _, ev_idx = event_log.strip().partition("|")
    for line in rows.strip().splitlines():
        name, total, idx = line.split("|")
        if name == "event_log":
            total, idx = ev_total, ev_idx
        out.append(TableSize(name, int(total), int(idx)))
    return sorted(out, key=lambda t: -t.total)


@dataclass(frozen=True)
class Means:
    """The inputs of the disk space formula of 16 13 and the durability of the database."""

    messages: int
    message_row_bytes: float
    events: int
    event_payload_bytes: float
    database_bytes: int
    fsync: str
    synchronous_commit: str
    postgres_version: str


def parse_means(row: str) -> Means:
    """The values of MEANS_SQL from one ``psql -At -F '|'`` row."""
    c = row.strip().split("|")
    return Means(int(c[0]), float(c[1]), int(c[2]), float(c[3]), int(c[4]), c[5], c[6], c[7])


# ---- the report --------------------------------------------------------------------------------


@dataclass
class StepMetrics:
    """The server side of one step: the PromQL values, the peak memory and CPU per service."""

    index: int
    prom: dict[str, float | None]
    memory: dict[str, int | None]
    cpu: dict[str, float | None] = field(default_factory=dict)


def capacity(result: Mapping[str, Any]) -> tuple[dict[str, Any] | None, dict[str, Any] | None]:
    """The last step without a missed target and the first with one (None when absent)."""
    last = first = None
    for step in result["steps"]:
        if step.get("missed"):
            first = step
            break
        last = step
    return last, first


def _ms(ns: float | None) -> str:
    return "n/a" if ns is None else f"{ns / 1e6:.1f} ms"


def _num(v: float | None, unit: str = "", digits: int = 1) -> str:
    return "n/a" if v is None else f"{v:.{digits}f}{unit}"


def _mib(v: float | None) -> str:
    return "n/a" if v is None else f"{v / (1 << 20):.0f} MiB"


# The rows of 16 13 in their order: bottleneck, metric, lever.
ROWS = (
    (
        "Serial section at the counter event_seq",
        "bothy.db.event_seq.lock_wait",
        "faster storage for WAL, shorter transactions",
    ),
    (
        "Connections to the database",
        "postgresql.backends, pool metrics",
        "database.max_conns, several api processes only with a pooler",
    ),
    (
        "WebSocket connections",
        "bothy.gateway.connections.active",
        "more api processes; each process reads fan-out itself",
    ),
    ("Fan-out lag", "bothy.event.fanout.lag", "fewer receivers per process"),
    ("ClamAV memory", "Container memory", "separate host for the scan"),
)


def row_values(
    step: Mapping[str, Any] | None, m: StepMetrics | None, clamav_peak: int | None
) -> list[str]:
    """The measured value of every row of ROWS at one step (``n/a`` where nothing was measured)."""
    prom = m.prom if m else {}
    mem = m.memory if m else {}
    fanout = step["fanout"]["p99_ns"] if step else None
    return [
        f"p99 {_num(_s_to_ms(prom.get('lock_wait_p99_s')), ' ms')}, mean "
        f"{_num(_s_to_ms(prom.get('lock_wait_mean_s')), ' ms', 2)} at "
        f"{_num(prom.get('events_per_s'), ' events/s')}",
        f"backends max {_num(prom.get('backends_max'), '', 0)}, pool acquired max "
        f"{_num(prom.get('pool_acquired_max'), '', 0)} of database.max_conns (api)",
        f"{_num(prom.get('gateway_connections_max'), '', 0)} connections, api memory "
        f"{_mib(mem.get('api'))}",
        f"max {_num(_s_to_ms(prom.get('fanout_lag_max_s')), ' ms')}; commit to dispatch p99 "
        f"{_ms(fanout)}",
        f"peak {_mib(clamav_peak)} over the run",
    ]


def build_report(
    result: Mapping[str, Any],
    metrics: Sequence[StepMetrics],
    tables: Sequence[TableSize],
    means: Means,
    clamav_peak: int | None,
    conditions: Mapping[str, Any],
) -> dict[str, Any]:
    """The report: conditions, steps with their server metrics, the rows of 16 13, the disk inputs.

    The rows follow the table of 16 13 in its order; each names the metric, the lever and the
    values measured at the last step that held every target and at the first step that missed one
    (02 section 17: those two steps are the result of the ramp).
    """
    last, first = capacity(result)

    def at(step: Mapping[str, Any] | None) -> StepMetrics | None:
        return next((x for x in metrics if step is not None and x.index == step["index"]), None)

    held = row_values(last, at(last), clamav_peak) if last else None
    missed = row_values(first, at(first), clamav_peak) if first else None
    rows = [
        {
            "bottleneck": b,
            "metric": metric,
            "lever": lever,
            "held": held[i] if held else None,
            "missed": missed[i] if missed else None,
        }
        for i, (b, metric, lever) in enumerate(ROWS)
    ]
    index_bytes = sum(t.indexes for t in tables)
    return {
        "conditions": dict(conditions),
        "throughput": {
            "last_held_step": last["index"] if last else None,
            "last_held_rate": last["rate"] if last else None,
            "sustained_messages_per_s": last["sends"]["goodput_per_s"] if last else None,
            "first_missed_step": first["index"] if first else None,
            "first_missed_rate": first["rate"] if first else None,
            "first_missed": first["missed"] if first else None,
            "stopped": result.get("stopped"),
        },
        "rows": rows,
        "disk": {
            "messages": means.messages,
            "message_row_bytes": round(means.message_row_bytes, 1),
            "message_bytes_with_indexes": round(_per(_size(tables, "messages"), means.messages), 1),
            "events": means.events,
            "event_payload_bytes": round(means.event_payload_bytes, 1),
            "event_bytes_with_indexes": round(_per(_size(tables, "event_log"), means.events), 1),
            "database_bytes": means.database_bytes,
            "index_share": round(index_bytes / means.database_bytes, 3)
            if means.database_bytes
            else None,
        },
        "tables": [asdict(t) for t in tables[:12]],
        "steps": [
            {
                "index": s["index"],
                "rate": s["rate"],
                "connections": s["connections"],
                "goodput_per_s": s["sends"]["goodput_per_s"],
                "create_p99_ms": s["sends"]["p99_ns"] / 1e6,
                "fanout_p99_ms": s["fanout"]["p99_ns"] / 1e6,
                "missed": s.get("missed") or [],
                "server": next((asdict(x) for x in metrics if x.index == s["index"]), None),
            }
            for s in result["steps"]
        ],
    }


def _s_to_ms(v: float | None) -> float | None:
    return None if v is None else v * 1000


def _size(tables: Sequence[TableSize], name: str) -> int:
    return next((t.total for t in tables if t.name == name), 0)


def _per(total: int, count: int) -> float:
    return total / count if count else 0.0


def render(report: Mapping[str, Any]) -> str:
    """The report as Markdown: conditions, the rows of 16 13, the disk inputs and the steps."""
    c = report["conditions"]
    t = report["throughput"]
    d = report["disk"]
    lines = [
        "# Capacity load test (16 13)",
        "",
        f"Commit {c['commit']}, {c['date']}, release {c['release']}; profile "
        f"`{c['profile']}` in mode {c['mode']} (runtime {c['runtime_cpus']} vCPU, "
        f"{c['runtime_memory'] / (1 << 30):.1f} GiB; servers and PostgreSQL limited to "
        f"{c['cpus']} vCPU, PostgreSQL to {c['memory'] / (1 << 30):.0f} GiB); layout "
        f"{c['layout']}; PostgreSQL {c['postgres_version']}, fsync {c['fsync']}, "
        f"synchronous_commit {c['synchronous_commit']}; seed {c['seed']}.",
        "",
        f"Sustained throughput (goodput within perf.message_create_p99 at the last step that "
        f"held): {_num(t['sustained_messages_per_s'], ' messages/s')} at step "
        f"{t['last_held_step']}; ramp: {t['stopped']}.",
    ]
    if t["first_missed"]:
        lines.append(f"First missed step {t['first_missed_step']}: {'; '.join(t['first_missed'])}.")
    held = f"Last held step ({_num(t['last_held_rate'], ' messages/s', 0)})"
    missed = f"First missed step ({_num(t['first_missed_rate'], ' messages/s', 0)})"
    lines += ["", f"| Bottleneck | Metric | {held} | {missed} | Lever |", "|---|---|---|---|---|"]
    lines += [
        f"| {r['bottleneck']} | `{r['metric']}` | {r['held'] or '-'} | {r['missed'] or '-'} | "
        f"{r['lever']} |"
        for r in report["rows"]
    ]
    lines += [
        "",
        "Disk space formula inputs: "
        f"{d['messages']} messages, {d['message_row_bytes']} bytes per row, "
        f"{d['message_bytes_with_indexes']} bytes per message with indexes; {d['events']} events, "
        f"payload mean {d['event_payload_bytes']} bytes, {d['event_bytes_with_indexes']} bytes per "
        f"event with indexes; index share {d['index_share']} of {d['database_bytes']} bytes.",
        "",
        "| Step | Rate | Connections | Goodput/s | create p99 ms | fan-out p99 ms | Events/s "
        "| Lock wait p99 ms | Pool acquired max | Fan-out lag max ms | api memory | CPU peak % "
        "(api, worker, postgres, driver) | Missed |",
        "|---|---|---|---|---|---|---|---|---|---|---|---|---|",
    ]
    for s in report["steps"]:
        prom = (s["server"] or {}).get("prom", {})
        mem = (s["server"] or {}).get("memory", {})
        cpu = (s["server"] or {}).get("cpu", {})
        cpus = ", ".join(_num(cpu.get(x), "", 0) for x in ("api", "worker", "postgres", "driver"))
        lines.append(
            f"| {s['index']} | {s['rate']:g} | {s['connections']} | {s['goodput_per_s']:.1f} | "
            f"{s['create_p99_ms']:.1f} | {s['fanout_p99_ms']:.1f} | "
            f"{_num(prom.get('events_per_s'))} | {_num(_s_to_ms(prom.get('lock_wait_p99_s')))} | "
            f"{_num(prom.get('pool_acquired_max'), '', 0)} | "
            f"{_num(_s_to_ms(prom.get('fanout_lag_max_s')))} | {_mib(mem.get('api'))} | "
            f"{cpus} | {'; '.join(s['missed']) or '-'} |"
        )
    return "\n".join(lines) + "\n"


# ---- the run -----------------------------------------------------------------------------------


def go_arch(docker_arch: str) -> str:
    """The GOARCH of the Docker runtime's architecture (``aarch64`` is ``arm64``)."""
    return {"aarch64": "arm64", "arm64": "arm64", "x86_64": "amd64", "amd64": "amd64"}.get(
        docker_arch, docker_arch
    )


def runtime(run: Runner) -> tuple[int, int, str]:
    """The vCPUs, the memory in bytes and the GOARCH of the Docker runtime."""
    res = run(
        ["docker", "info", "--format", "{{.NCPU}} {{.MemTotal}} {{.Architecture}}"],
        capture_output=True,
    )
    cpus, mem, arch = res.stdout.split()
    return int(cpus), int(mem), go_arch(arch)


def build_driver(run: Runner, target: Path, goarch: str) -> None:
    """Build ``bothy-load`` for Linux on the runtime's architecture into target."""
    env = {**os.environ, "GOOS": "linux", "GOARCH": goarch, "CGO_ENABLED": "0"}
    res = run(
        ["go", "build", "-trimpath", "-o", str(target), "./test/load/cmd/bothy-load"],
        cwd=SERVER_MODULE, env=env, capture_output=True,
    )  # fmt: skip
    if res.returncode != 0:
        raise pr.StepError(f"building bothy-load failed: {res.stderr.strip()[-800:]}")


def sql(inst: pr.Installation, query: str) -> str:
    """Run query as the superuser over the socket of the PostgreSQL container."""
    res = inst.compose(
        "exec", "-T", "postgres", "psql", "-U", "postgres", "-d", "bothy", "-At", "-F", "|",
        "-v", "ON_ERROR_STOP=1", "-c", query, capture_output=True,
    )  # fmt: skip
    return str(res.stdout)


def prometheus(inst: pr.Installation, query: str, at: float) -> float | None:
    """Evaluate query at the UNIX time at inside the Prometheus container."""
    url = "http://127.0.0.1:9090/api/v1/query?" + urlencode({"query": query, "time": f"{at:.3f}"})
    res = inst.compose("exec", "-T", "prometheus", "wget", "-qO-", url, capture_output=True)
    return prom_value(res.stdout)


def _ts(text: str) -> float:
    return dt.datetime.fromisoformat(text.replace("Z", "+00:00")).timestamp()


def step_metrics(
    inst: pr.Installation, result: Mapping[str, Any], sampler: MemorySampler
) -> list[StepMetrics]:
    """The server metrics of every step window."""
    out = []
    for s in result["steps"]:
        start, end = _ts(s["window_from"]), _ts(s["window_to"])
        seconds = max(int(end - start), 1)
        prom = {k: prometheus(inst, q, end) for k, q in prom_queries(seconds).items()}
        svcs = ("api", "worker", "postgres", "driver")
        memory = {svc: sampler.peak(svc, start, end) for svc in svcs}
        cpu = {svc: sampler.cpu_peak(svc, start, end) for svc in svcs}
        out.append(StepMetrics(s["index"], prom, memory, cpu))
    return out


def apply(inst: pr.Installation, path: Path, doc: Mapping[str, Any]) -> None:
    """Write the override and bring the servers and PostgreSQL to it."""
    path.write_text(json.dumps(doc, indent=2) + "\n", encoding="utf-8")
    inst.compose(
        "up", "--detach", "--wait", "--wait-timeout", "600", "postgres", "api", "worker",
        capture_output=True,
    )  # fmt: skip


def containers(inst: pr.Installation) -> dict[str, str]:
    """Container name to service of the services of SERVICES."""
    res = inst.compose("ps", "--format", "json", capture_output=True)
    return {
        row["Name"]: row["Service"]
        for row in pr._json_rows(res.stdout)
        if row.get("Service") in SERVICES
    }


def commit(run: Runner) -> str:
    """The commit of this checkout (short)."""
    res = run(["git", "-C", str(ROOT), "rev-parse", "--short=12", "HEAD"], capture_output=True)
    return res.stdout.strip() or "unknown"


@dataclass
class Stack:
    """The installation of the run with what the driver needs.

    Attributes:
        inst: The installation.
        out: The folder of the driver (mounted at /work): the binary, setup.out, the result.
        override: The Compose override file of the run.
        rates: ``LOAD_RATES``, the rates of a shorter ramp by hand; empty runs the ramp of
            params.go (the measurement).
        trusted: ``server.trusted_proxies`` of the run.
        driver: The ``docker run`` of the driver.
    """

    inst: pr.Installation
    out: Path
    override: Path
    trusted: str = ""
    driver: list[str] = field(default_factory=list)
    rates: str = ""


def metrics_overlay(inst: pr.Installation) -> None:
    """Switch the overlay local on for the metrics only: the collector and Prometheus.

    As ``enable_local_overlay`` of the portability test, but without Loki and Tempo: under load
    their logs and traces would fill the disk of the Docker host, and the run reads metrics only.
    The collector keeps retrying their exporters; that never blocks the server. The traces keep
    the installation's sampling, as an operator runs it.
    """
    profiles = pr.env_value(inst.compose_dir / ".env", "COMPOSE_PROFILES")
    inst.profiles = f"{profiles},observability"
    inst.env["BOTHY_OTEL_EXPORTER"] = "local"
    inst.compose(
        "up", "--detach", "--wait", "--wait-timeout", "600", *METRICS_STACK, capture_output=True
    )


def install_stack(ctx: pr.Context, stack: Stack, values: Values, goarch: str) -> None:
    """Build the driver, verify the images, install, start the overlay and the seeding settings."""
    inst, run = stack.inst, ctx.run
    build_driver(run, stack.out / "bothy-load", goarch)
    say(f"pull and verify the images of {ctx.release.registry}")
    verified = pr.verify_images(ctx.release, ctx.key, ctx.iface.repository, run)
    code = pr.install(ctx, inst)
    pr.write_secret(stack.out / "setup.out", code + "\n")
    metrics_overlay(inst)
    subnet = run(
        ["docker", "network", "inspect", f"{inst.project}_default", "--format",
         "{{range .IPAM.Config}}{{.Subnet}} {{end}}"],
        capture_output=True,
    ).stdout.split()  # fmt: skip
    stack.trusted = ",".join(["127.0.0.1/32", "::1/128", *subnet])
    inst.env["COMPOSE_FILE"] = f"compose.yaml:{stack.override}"
    apply(inst, stack.override, override(True, values.cpus, values.memory, stack.trusted))
    port = inst.published_port()
    # The restart before the ramp must keep the port the driver dials.
    inst.env["BOTHY_HTTP_PUBLISH_PORT"] = str(port)
    server = next(v for v in verified if "/bothy-server:" in v.ref)
    stack.driver = [
        "docker", "run", "--rm", "--name", f"{inst.project}-driver", "--network", "host",
        "--user", f"{os.getuid()}:{os.getgid()}",
        "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true",
        "--ulimit", "nofile=65536:65536", "-v", f"{stack.out}:/work",
        "--entrypoint", "/work/bothy-load", server.ref, "ramp",
        "--base-url", f"http://127.0.0.1:{port}", "--public-url", pr.PUBLIC_URL,
        "--dir", "/work", "--message-create-p99", values.message_create_p99,
        "--fanout-p99", values.fanout_p99, "--messages-per-minute", str(values.messages_per_minute),
    ]  # fmt: skip
    if stack.rates:
        stack.driver += ["--rates", stack.rates]


def drive(stack: Stack, values: Values, log: Path) -> tuple[dict[str, Any], MemorySampler]:
    """Run the driver: seeding, the restart with the default limits, the ramp; return its result."""
    inst = stack.inst
    say("seeding the world with raised rate limits, then the ramp (tests/load/README.md)")
    sampler = MemorySampler(containers(inst), inst.run)
    with pr.background(stack.driver, log) as proc:
        pr.wait_for(stack.out / "seeded", proc, DRIVER_TIMEOUT_S)
        apply(inst, stack.override, override(False, values.cpus, values.memory, stack.trusted))
        # The driver shares the Docker host: its CPU shows whether it, not the stack, was the limit.
        sampler.containers = {**containers(inst), f"{inst.project}-driver": "driver"}
        say("the server runs with its default rate limits: the ramp starts")
        (stack.out / "measure").write_text("measure\n", encoding="utf-8")
        sampler.start()
        try:
            rc = proc.wait(timeout=DRIVER_TIMEOUT_S)
        finally:
            sampler.stop()
    sys.stderr.write(log.read_text(encoding="utf-8"))
    if rc != 0:
        raise pr.StepError(f"bothy-load ended with exit code {rc}")
    result: dict[str, Any] = json.loads((stack.out / "result.json").read_text(encoding="utf-8"))
    return result, sampler


def write_report(report_dir: Path, report: Mapping[str, Any], result: Mapping[str, Any]) -> None:
    """Write capacity-report.json (report and the driver's result) and capacity-report.md."""
    report_dir.mkdir(parents=True, exist_ok=True)
    (report_dir / "capacity-report.json").write_text(
        json.dumps({"report": report, "result": result}, indent=2) + "\n", encoding="utf-8"
    )
    (report_dir / "capacity-report.md").write_text(render(report), encoding="utf-8")


def measure(ctx: pr.Context, values: Values, workdir: Path, report_dir: Path) -> dict[str, Any]:
    """One installation with fs: install, profile, seed, ramp, metrics, report; then clean up."""
    project = f"bothy-load-{secrets.token_hex(4)}"
    env = pr.clean_env(ctx.environ)
    env["COMPOSE_PROJECT_NAME"] = project
    env.update(dict.fromkeys(pr.PORT_VARIABLES, "0"))
    env["BOTHY_OTEL_METRIC_EXPORT_INTERVAL_MS"] = str(METRIC_EXPORT_MS)
    inst = pr.Installation(workdir / "fs", "fs", project, env, ctx.run)
    inst.workdir.mkdir()
    stack = Stack(inst, workdir / "out", workdir / "load-override.json")
    stack.rates = ctx.environ.get("LOAD_RATES", "")
    stack.out.mkdir(mode=0o700)
    try:
        cpus, mem, goarch = runtime(ctx.run)
        mode = profile_mode(cpus, mem, values.cpus, values.memory)
        say(f"runtime {cpus} vCPU, {mem} bytes: mode {mode} of {values.profile!r}")
        install_stack(ctx, stack, values, goarch)
        result, sampler = drive(stack, values, workdir / "driver.log")
        metrics = step_metrics(inst, result, sampler)
        tables = parse_sizes(sql(inst, SIZES_SQL), sql(inst, EVENT_LOG_SQL))
        means = parse_means(sql(inst, MEANS_SQL))
        conditions = {
            "commit": commit(ctx.run),
            "date": dt.datetime.now(dt.UTC).strftime("%Y-%m-%d"),
            "release": ctx.iface.tag,
            "profile": values.profile,
            "mode": mode,
            "runtime_cpus": cpus,
            "runtime_memory": mem,
            "cpus": values.cpus,
            "memory": values.memory,
            "layout": "api and worker as installed (16 3), one api node",
            "postgres_version": means.postgres_version,
            "fsync": means.fsync,
            "synchronous_commit": means.synchronous_commit,
            "seed": result["seed"],
            "params": result["params"],
            "targets": result["targets"],
        }
        clamav = sampler.peak("clamav", 0, time.time())
        report = build_report(result, metrics, tables, means, clamav, conditions)
        write_report(report_dir, report, result)
        return report
    except (pr.StepError, subprocess.SubprocessError, OSError, KeyError, ValueError) as exc:
        say(f"FAILED: {exc}")
        inst.diagnose()
        raise pr.StepError(str(exc)) from exc
    finally:
        left = inst.down()
        if left:
            say(f"cleanup left objects of {project} behind: {left}")


def main(environ: Mapping[str, str] | None = None, run: Runner = pr._run) -> int:
    """Run the load test; see the module documentation."""
    environ = os.environ if environ is None else environ
    try:
        iface = pr.read_interface(environ)
        pr.preflight(run)
        if shutil.which("go") is None:
            raise pr.InterfaceError("go is not on PATH (it builds bothy-load)")
        values = read_values(
            ARCHITECTURE.read_text(encoding="utf-8"), CONFIGURATION.read_text(encoding="utf-8")
        )
    except (pr.InterfaceError, pr.StepError) as exc:
        say(str(exc))
        return 2
    key = iface.trial_key or (pr.REPO_KEY if pr.REPO_KEY.is_file() else None)
    workdir = Path(tempfile.mkdtemp(prefix="bothy-load-"))
    report_dir = Path(environ.get("LOAD_REPORT_DIR", "") or REPORT_DIR)
    try:
        release = pr.fetch_release(iface, workdir)
        ctx = pr.Context(iface, workdir, release, "", key, environ, run)
        report = measure(ctx, values, workdir, report_dir)
    except (pr.StepError, subprocess.SubprocessError, OSError) as exc:
        say(f"FAIL: {exc}")
        return 1
    finally:
        shutil.rmtree(workdir, ignore_errors=True)
    sustained = report["throughput"]["sustained_messages_per_s"]
    say(f"report in {report_dir}: sustained {sustained} messages/s")
    return 0


if __name__ == "__main__":
    sys.exit(main())
