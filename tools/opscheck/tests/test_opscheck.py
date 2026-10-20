# SPDX-License-Identifier: Apache-2.0
"""Tests for tools/opscheck/opscheck.py.

The checks run the real pinned validators (fetched on first use through
tools/bin/fetch_tools.py; no other network). Negative cases copy the subtrees a check
reads into a temporary repository root, apply one mutation and run that single check.
"""

from __future__ import annotations

import ast
import contextlib
import copy
import importlib.util
import os
import re
import shutil
import signal
import stat
import subprocess
import sys
import threading
import time
from collections.abc import Callable
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
from types import ModuleType
from typing import Any

import pytest
import yaml


def _load_opscheck() -> ModuleType:
    """Load tools/opscheck/opscheck.py from its file.

    pytest's importlib mode names this test module `opscheck.tests.test_opscheck` and
    registers the directory tools/opscheck as its parent package `opscheck`, which hides the
    tool module of the same name from a plain `import opscheck`.
    """
    path = Path(__file__).resolve().parents[1] / "opscheck.py"
    spec = importlib.util.spec_from_file_location("opscheck_tool", path)
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module  # dataclasses look their module up while it executes
    spec.loader.exec_module(module)
    return module


opscheck = _load_opscheck()

REPO = Path(__file__).resolve().parents[3]
SUBTREES = (
    "infra",
    "docs/observability",
    "docs/operations/configuration.md",
    "tools/bin/tools.lock.json",
    "docs/spec/03-datenmodell.md",
    "services/server/internal/stats/domain/slibounds.gen.go",
    "packages/bothy_viz/lib/src/slo/slo_catalog.g.dart",
)
ALL_CHECKS = [
    "tool-versions",
    "collector",
    "collector-allowlist",
    "prometheus-config",
    "prometheus-rules",
    "prometheus-rule-tests",
    "alertmanager",
    "loki",
    "tempo",
    "sloth",
    "slo-docs",
    "sli-bounds",
    "sli-labels",
    "alert-thresholds",
    "tofu",
]


@pytest.fixture
def tree(tmp_path: Path) -> Path:
    """A repository root with copies of every file the checks read."""
    root = tmp_path / "repo"
    for rel in SUBTREES:
        src, dst = REPO / rel, root / rel
        if src.is_dir():
            shutil.copytree(src, dst)
        else:
            dst.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(src, dst)
    return root


def edit(path: Path, old: str, new: str, count: int = 1) -> None:
    """Replace `old` by `new` in a copied file; the test fails if `old` is not there."""
    text = path.read_text(encoding="utf-8")
    assert text.count(old) >= count, f"{old!r} not in {path}"
    path.write_text(text.replace(old, new, count), encoding="utf-8")


def findings(root: Path, name: str) -> tuple[str, ...]:
    """Run one check against `root` and return its findings."""
    [result] = opscheck.run_checks(root, only=[name])
    assert result.name == name
    return result.findings


def fake_tool(directory: Path, name: str, script: str) -> Path:
    """An executable shell script standing in for a pinned validator."""
    directory.mkdir(parents=True, exist_ok=True)
    path = directory / name
    path.write_text("#!/bin/sh\n" + script + "\n", encoding="utf-8")
    path.chmod(path.stat().st_mode | stat.S_IXUSR)
    return path


# ---- helpers ------------------------------------------------------------------------------


@pytest.mark.parametrize(
    ("text", "seconds"),
    [
        ("500ms", 0.5),
        ("5s", 5.0),
        ("90s", 90.0),
        ("1.5m", 90.0),
        ("26h", 93600.0),
        ("8d", 691200.0),
        ("1h30m", 5400.0),
        (" 60s ", 60.0),
    ],
)
def test_parse_duration(text: str, seconds: float) -> None:
    assert opscheck.parse_duration(text) == seconds


@pytest.mark.parametrize("text", ["", "5", "5w", "s", "1h 30m", "-5s", "five seconds"])
def test_parse_duration_rejects_other_text(text: str) -> None:
    with pytest.raises(ValueError, match="duration"):
        opscheck.parse_duration(text)


MARKDOWN = """\
# Title

## 1. Intro

| a | b |
|---|---|
| not | this |

```
## 2. Fenced heading is not a heading
| x | y |
```

### 3.2 Weitere Alarme (`bothy-alerts.yaml`)

Some text first.

| Alarm | Stufe | Extra \\| Spalte |
|---|:--:|---|
| `One` | page | a \\| b |
| `Two`, `Three` | ticket | |

## 4. Empty

Only text.

## 5. Next
"""


def test_markdown_table() -> None:
    rows = opscheck.markdown_table(MARKDOWN, "Weitere Alarme")
    assert rows == [
        {"Alarm": "`One`", "Stufe": "page", "Extra | Spalte": "a | b"},
        {"Alarm": "`Two`, `Three`", "Stufe": "ticket", "Extra | Spalte": ""},
    ]
    assert opscheck.markdown_table(MARKDOWN, "Intro") == [{"a": "not", "b": "this"}]


@pytest.mark.parametrize(
    ("heading", "message"),
    [
        ("Missing", "no heading"),
        ("Fenced heading is not a heading", "no heading"),
        ("Empty", "no table"),
        ("Next", "no table"),
    ],
)
def test_markdown_table_errors(heading: str, message: str) -> None:
    with pytest.raises(ValueError, match=message):
        opscheck.markdown_table(MARKDOWN, heading)


def test_markdown_table_rejects_ragged_rows() -> None:
    text = "## T\n\n| a | b |\n|---|---|\n| 1 | 2 | 3 |\n"
    with pytest.raises(ValueError, match="cells"):
        opscheck.markdown_table(text, "T")


REFERENCE = """\
# Konfigurationsreferenz

### 04 Kapitel

| Schlüssel | Umgebung | Ort | Standard | Bedeutung |
|---|---|---|---|---|
| `agent.presence_timeout` | `BOTHY_AGENT__PRESENCE_TIMEOUT` | Server | `90s` | text |
| `twice.value` | `A` | Server | `1s` | first |

### 16 Kapitel

| Schlüssel | Standard | Neue Spalte | Bedeutung |
|---|---|---|---|
| `backup.max_age` | `26h` | x | text |
| `twice.value` | `2s` | y | second |

## 9. Compose-Variablen

| Variable | Standard | Bedeutung |
|---|---|---|
| `other.value` | `5s` | not a key table |
"""


def test_config_default(tmp_path: Path) -> None:
    ref = tmp_path / "configuration.md"
    ref.write_text(REFERENCE, encoding="utf-8")
    assert opscheck.config_default(ref, "agent.presence_timeout") == "90s"
    assert opscheck.config_default(ref, "backup.max_age") == "26h"


def test_config_default_missing_or_duplicate(tmp_path: Path) -> None:
    ref = tmp_path / "configuration.md"
    ref.write_text(REFERENCE, encoding="utf-8")
    with pytest.raises(ValueError, match=r"backup\.unknown.*not found"):
        opscheck.config_default(ref, "backup.unknown")
    with pytest.raises(ValueError, match=r"twice\.value.*2 rows"):
        opscheck.config_default(ref, "twice.value")
    # Only tables with the columns `Schlüssel` and `Standard` count.
    with pytest.raises(ValueError, match="not found"):
        opscheck.config_default(ref, "other.value")


def test_config_default_reads_the_repository_reference() -> None:
    ref = REPO / "docs" / "operations" / "configuration.md"
    assert opscheck.config_default(ref, "backup.max_age") == "26h"


# ---- command runner -----------------------------------------------------------------------


def _context(tmp_path: Path) -> opscheck.Context:
    work = tmp_path / "work"
    work.mkdir()
    return opscheck.Context(root=tmp_path, workdir=work, tool=lambda name: Path("/nonexistent"))


def test_run_command_uses_only_the_minimal_environment(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A developer's shell variables never reach a validator."""
    monkeypatch.setenv("OPSCHECK_LEAK_PROBE", "leaked")
    ctx = _context(tmp_path)
    probe = fake_tool(tmp_path / "bin", "probe", '/usr/bin/env > "$HOME/env.txt"')

    assert opscheck.run_command(ctx, [probe], env={"EXTRA": "1"}) == []

    lines = (ctx.workdir / "home" / "env.txt").read_text(encoding="utf-8").splitlines()
    env = dict(line.split("=", 1) for line in lines)
    # /bin/sh sets these itself; they are not inherited from the caller.
    assert Path(env.pop("PWD")).resolve() == ctx.workdir.resolve()
    env.pop("SHLVL", None)
    env.pop("_", None)
    closed = opscheck.NO_NETWORK_PROXY
    assert env == {
        "PATH": str(ctx.workdir / "bin"),
        "HOME": str(ctx.workdir / "home"),
        "TMPDIR": str(ctx.workdir / "tmp"),
        "HTTP_PROXY": closed,
        "HTTPS_PROXY": closed,
        "http_proxy": closed,
        "https_proxy": closed,
        "GCE_METADATA_HOST": closed.removeprefix("http://"),
        "EXTRA": "1",
    }
    assert closed.startswith("http://127.0.0.1:")


def test_run_command_reports_exit_code_and_output_tail(tmp_path: Path) -> None:
    ctx = _context(tmp_path)
    lines = "; ".join(f"echo line{i}" for i in range(30))
    tool = fake_tool(tmp_path / "bin", "failing", f"{lines}; echo oops >&2; exit 3")

    [finding] = opscheck.run_command(ctx, [tool, ctx.workdir / "file.yaml"], label="demo")

    first, *tail = finding.splitlines()
    assert first == "demo: exit status 3 (failing <work>/file.yaml)"
    assert len(tail) == opscheck.OUTPUT_TAIL_LINES
    assert tail[-1] == "    oops"
    assert "line0" not in finding


def test_run_command_without_label_names_the_command(tmp_path: Path) -> None:
    ctx = _context(tmp_path)
    tool = fake_tool(tmp_path / "bin", "quiet-fail", "exit 1")
    assert opscheck.run_command(ctx, [tool, "check"]) == ["quiet-fail check: exit status 1"]


def test_run_command_times_out(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(opscheck, "COMMAND_TIMEOUT_S", 0.5)
    ctx = _context(tmp_path)
    tool = fake_tool(tmp_path / "bin", "hangs", "exec /bin/sleep 30")
    [finding] = opscheck.run_command(ctx, [tool])
    assert finding == "hangs: timed out after 0.5 s"


def test_run_command_times_out_although_a_child_holds_the_output(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """After the timeout the validator is killed and waited for; its output is not read on."""
    monkeypatch.setattr(opscheck, "COMMAND_TIMEOUT_S", 0.5)
    ctx = _context(tmp_path)
    tool = fake_tool(tmp_path / "bin", "forks", "/bin/sleep 10 &\nexec /bin/sleep 30")
    started = time.monotonic()
    [finding] = opscheck.run_command(ctx, [tool])
    assert finding == "forks: timed out after 0.5 s"
    assert time.monotonic() - started < 5


def test_run_command_reports_a_binary_that_cannot_run(tmp_path: Path) -> None:
    ctx = _context(tmp_path)
    [finding] = opscheck.run_command(ctx, [tmp_path / "missing-tool"], label="demo")
    assert finding.startswith("demo: cannot run missing-tool:")


# ---- runner and CLI -----------------------------------------------------------------------


def test_checks_are_registered_in_table_order() -> None:
    assert list(opscheck.CHECKS) == ALL_CHECKS


def test_list(capsys: pytest.CaptureFixture[str]) -> None:
    assert opscheck.main(["--list"]) == 0
    assert capsys.readouterr().out.splitlines() == ALL_CHECKS


def test_unknown_check_is_a_usage_error(capsys: pytest.CaptureFixture[str]) -> None:
    assert opscheck.main(["--only", "nope"]) == 2
    assert "unknown check: nope" in capsys.readouterr().err
    with pytest.raises(ValueError, match="unknown check"):
        opscheck.run_checks(REPO, only=["nope"])


def test_missing_tool_is_a_finding_not_a_skip(tree: Path) -> None:
    """Fail closed: a validator that cannot be fetched fails its check (R-QA-02)."""

    def unavailable(name: str) -> Path:
        raise opscheck.fetch_tools.FetchError(f"{name}: download failed")

    [result] = opscheck.run_checks(tree, only=["loki"], tool=unavailable)
    assert result.findings == ("check aborted: FetchError: loki: download failed",)


def test_crashing_check_does_not_stop_the_others(tree: Path) -> None:
    (tree / "infra" / "observability" / "slo" / "api.yaml").write_text("slos: [", encoding="utf-8")
    results = opscheck.run_checks(tree, only=["slo-docs", "tool-versions"])
    assert [r.name for r in results] == ["tool-versions", "slo-docs"]
    assert results[0].findings == ()
    assert results[1].findings[0].startswith("check aborted: ")


def test_run_checks_removes_its_work_directory(tree: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    created: list[Path] = []
    real_mkdtemp = opscheck.tempfile.mkdtemp

    def spy(*args: object, **kwargs: object) -> str:
        path = real_mkdtemp(*args, **kwargs)  # type: ignore[call-overload]
        created.append(Path(path))
        return str(path)

    monkeypatch.setattr(opscheck.tempfile, "mkdtemp", spy)
    opscheck.run_checks(tree, only=["tool-versions"])
    assert len(created) == 1
    assert not created[0].exists()


def test_cli_reports_all_failures(tree: Path, capsys: pytest.CaptureFixture[str]) -> None:
    """One run shows every failing check, not only the first one."""
    edit(tree / "docs" / "observability" / "slos.md", "| 99.9 % |", "| 99.5 % |")
    edit(
        tree / "infra" / "observability" / "prometheus" / "rules" / "bothy-alerts.yaml",
        "> 93600",
        "> 90000",
    )
    rc = opscheck.main(["--root", str(tree), "--only", "alert-thresholds", "slo-docs"])
    out = capsys.readouterr().out
    assert rc == 1
    assert "slo-docs: FAIL\n" in out
    assert "alert-thresholds: FAIL\n" in out
    assert "  bothy-api/requests-availability: Ziel is 99.5 %, the Sloth file says 99.9 %" in out
    assert out.index("slo-docs: FAIL") < out.index("alert-thresholds: FAIL")


def test_cli_ok_lines(tree: Path, capsys: pytest.CaptureFixture[str]) -> None:
    assert opscheck.main(["--root", str(tree), "--only", "tool-versions", "slo-docs"]) == 0
    assert capsys.readouterr().out == "tool-versions: ok\nslo-docs: ok\n"


# ---- whole repository ---------------------------------------------------------------------


def test_repository_passes_all_checks() -> None:
    results = opscheck.run_checks(REPO)
    assert [r.name for r in results] == ALL_CHECKS
    assert {r.name: r.findings for r in results if r.findings} == {}


# ---- tool-versions ------------------------------------------------------------------------

ENV_EXAMPLE = Path("infra/compose/.env.example")
COMPOSE = Path("infra/compose/compose.yaml")


def test_env_example_version_mismatch_fails(tree: Path) -> None:
    edit(tree / ENV_EXAMPLE, "BOTHY_LOKI_VERSION=3.7.8", "BOTHY_LOKI_VERSION=3.7.9")
    assert findings(tree, "tool-versions") == (
        "BOTHY_LOKI_VERSION in infra/compose/.env.example is 3.7.9, the pinned loki is 3.7.8",
    )


def test_compose_default_mismatch_fails(tree: Path) -> None:
    edit(
        tree / COMPOSE,
        "${BOTHY_PROMETHEUS_VERSION:-v3.15.0}",
        "${BOTHY_PROMETHEUS_VERSION:-v3.16.0}",
    )
    assert findings(tree, "tool-versions") == (
        "default of BOTHY_PROMETHEUS_VERSION in infra/compose/compose.yaml is v3.16.0, "
        "the pinned promtool is 3.15.0",
    )


def test_missing_version_variable_fails(tree: Path) -> None:
    edit(tree / ENV_EXAMPLE, "BOTHY_TEMPO_VERSION=3.1.0\n", "")
    assert findings(tree, "tool-versions") == (
        "BOTHY_TEMPO_VERSION: expected exactly one line in infra/compose/.env.example, found 0",
    )


def test_missing_compose_default_fails(tree: Path) -> None:
    edit(tree / COMPOSE, "${BOTHY_TEMPO_VERSION:-3.1.0}", "${BOTHY_TEMPO_VERSION}")
    assert findings(tree, "tool-versions") == (
        "BOTHY_TEMPO_VERSION: no default ${BOTHY_TEMPO_VERSION:-<version>} in "
        "infra/compose/compose.yaml",
    )


# ---- collector ----------------------------------------------------------------------------

OTEL = Path("infra/otel")


def test_unknown_processor_in_base_fails(tree: Path) -> None:
    edit(
        tree / OTEL / "collector.yaml",
        "processors:\n  memory_limiter:",
        "processors:\n  bogus: {}\n  memory_limiter:",
    )
    found = findings(tree, "collector")
    [base] = [f for f in found if f.startswith("base: ")]
    assert 'unknown type: "bogus"' in base


def test_unknown_exporter_in_overlay_fails(tree: Path) -> None:
    edit(tree / OTEL / "exporters" / "local.yaml", "exporters:\n", "exporters:\n  bogus: {}\n")
    found = findings(tree, "collector")
    assert not [f for f in found if f.startswith("base: ")]
    [single] = [f for f in found if f.startswith("base + local: ")]
    assert 'unknown type: "bogus"' in single
    # Every pair with the broken overlay fails too, in both orders.
    exporters = tree / OTEL / "exporters"
    partners = [p for p in exporters.glob("*.yaml") if p.stem not in ("none", "local")]
    assert len([f for f in found if " local" in f.splitlines()[0]]) == 1 + 2 * len(partners)


SMOKE_OVERLAY = Path("infra/compose/test/otel/file.yaml")


def test_smoke_overlay_is_validated_with_the_base(tree: Path) -> None:
    """The overlay "file" of the Compose smoke test (M1-73) runs with the base alone."""
    edit(tree / SMOKE_OVERLAY, "exporters:\n", "exporters:\n  bogus: {}\n")
    [single] = findings(tree, "collector")
    assert single.startswith("base + smoke file: ")
    assert 'unknown type: "bogus"' in single


def test_smoke_overlay_must_exist(tree: Path) -> None:
    (tree / SMOKE_OVERLAY).unlink()
    assert findings(tree, "collector") == (f"{SMOKE_OVERLAY} is missing",)


def test_smoke_overlay_must_not_redefine_a_base_pipeline(tree: Path) -> None:
    edit(
        tree / SMOKE_OVERLAY,
        "    traces/smoke:\n",
        "    traces/in:\n      receivers: [otlp]\n      exporters: [nop]\n    traces/smoke:\n",
    )
    found = findings(tree, "collector-allowlist")
    assert f"{SMOKE_OVERLAY} redefines the base pipeline traces/in" in found


def test_pair_failure_is_detected(tree: Path) -> None:
    """Each overlay passes alone with the base; only the pair breaks (lists merge by order)."""
    exporters = tree / OTEL / "exporters"
    (exporters / "zz-big.yaml").write_text("processors: {batch: {send_batch_size: 10000}}\n")
    (exporters / "zz-cap.yaml").write_text("processors: {batch: {send_batch_max_size: 5000}}\n")
    found = findings(tree, "collector")
    assert sorted(f.splitlines()[0].split(":")[0] for f in found) == [
        "base + zz-big + zz-cap",
        "base + zz-cap + zz-big",
    ]
    assert all(
        "send_batch_max_size must be greater or equal to send_batch_size" in f for f in found
    )


def test_none_overlay_is_validated_alone_but_not_in_pairs(tree: Path) -> None:
    edit(tree / OTEL / "exporters" / "none.yaml", "{}", "processors: {bogus: {}}")
    found = findings(tree, "collector")
    assert [f.splitlines()[0].split(":")[0] for f in found] == ["base + none"]


def test_collector_secret_outside_the_secrets_directory_fails(tree: Path) -> None:
    edit(
        tree / OTEL / "exporters" / "sentry.yaml",
        "${file:/run/bothy/secrets/sentry_key}",
        "${file:/run/other/sentry_key}",
    )
    found = findings(tree, "collector")
    assert found
    assert all("sentry" in f.splitlines()[0] for f in found)


def test_datadog_overlay_in_the_repository_is_unchanged(tree: Path) -> None:
    """The host name is set only in the work copy; production keeps host detection."""
    before = (tree / OTEL / "exporters" / "datadog.yaml").read_bytes()
    assert findings(tree, "collector") == ()
    assert (tree / OTEL / "exporters" / "datadog.yaml").read_bytes() == before
    assert b"hostname" not in before


# ---- collector-allowlist ------------------------------------------------------------------

SIGNALS = Path("docs/observability/signals.md")


def test_allowlist_missing_catalogue_key_fails(tree: Path) -> None:
    edit(tree / OTEL / "collector.yaml", "      - bothy.step.kind\n", "")
    assert findings(tree, "collector-allowlist") == (
        "bothy.step.kind: in docs/observability/signals.md 5.3 but not in "
        "processors.redaction/allowlist.allowed_keys of infra/otel/collector.yaml",
    )


def test_allowlist_extra_key_fails(tree: Path) -> None:
    edit(
        tree / OTEL / "collector.yaml",
        "      - bothy.thread.id\n",
        "      - bothy.thread.id\n      - bothy.secret.thing\n",
    )
    assert findings(tree, "collector-allowlist") == (
        "bothy.secret.thing: in processors.redaction/allowlist.allowed_keys of "
        "infra/otel/collector.yaml but not in docs/observability/signals.md 5.3",
    )


def test_allowlist_reads_every_name_of_a_cell(tree: Path) -> None:
    edit(tree / SIGNALS, "`bothy.run.id`, `bothy.file.id`", "`bothy.run.id`")
    assert findings(tree, "collector-allowlist") == (
        "bothy.file.id: in processors.redaction/allowlist.allowed_keys of "
        "infra/otel/collector.yaml but not in docs/observability/signals.md 5.3",
    )


# ---- prometheus ---------------------------------------------------------------------------

PROM = Path("infra/observability/prometheus")


def test_invalid_scrape_config_fails(tree: Path) -> None:
    edit(
        tree / PROM / "prometheus.yaml",
        "  - job_name: tempo\n",
        "  - job_name: tempo\n    scrape_interval: soon\n",
    )
    [finding] = findings(tree, "prometheus-config")
    assert "soon" in finding


def test_prometheus_config_loads_the_rule_files(tree: Path) -> None:
    """The /etc/prometheus/ prefix points into the work copy, so broken rules fail here too."""
    edit(tree / PROM / "rules" / "bothy-alerts.yaml", "expr: vector(1)", "expr: vector(")
    [finding] = findings(tree, "prometheus-config")
    assert "bothy-alerts.yaml" in finding


def test_invalid_promql_fails(tree: Path) -> None:
    edit(tree / PROM / "rules" / "bothy-alerts.yaml", "expr: vector(1)", "expr: sum(rate(")
    [finding] = findings(tree, "prometheus-rules")
    output = "\n".join(finding.splitlines()[1:])
    assert '"Watchdog": could not parse expression' in output
    assert "bothy-alerts.yaml" in output


def test_changed_threshold_breaks_unit_test(tree: Path) -> None:
    edit(
        tree / PROM / "rules" / "bothy-alerts.yaml",
        'bothy_agent_heartbeat_age_seconds{job="bothy/bothy-server"}) > 300',
        'bothy_agent_heartbeat_age_seconds{job="bothy/bothy-server"}) > 301',
    )
    [finding] = findings(tree, "prometheus-rule-tests")
    assert "BothyAgentOffline" in finding


def test_alert_without_unit_test_fails(tree: Path) -> None:
    edit(
        tree / PROM / "rules" / "bothy-alerts.yaml",
        "  - name: bothy-data\n    rules:\n",
        "  - name: bothy-data\n    rules:\n"
        "      - alert: BothyUntested\n        expr: vector(0) > 1\n",
    )
    assert findings(tree, "prometheus-rule-tests") == (
        "BothyUntested: no promtool unit test names it as alertname in "
        "infra/observability/prometheus/tests/*.test.yaml",
    )


def test_rule_tests_need_a_test_file(tree: Path) -> None:
    for path in (tree / PROM / "tests").glob("*.test.yaml"):
        path.unlink()
    assert findings(tree, "prometheus-rule-tests")[0] == (
        "no unit test file infra/observability/prometheus/tests/*.test.yaml"
    )


# ---- alertmanager -------------------------------------------------------------------------

ALERTMANAGER = Path("infra/observability/alertmanager/alertmanager.yaml")


def test_route_to_undefined_receiver_fails(tree: Path) -> None:
    edit(tree / ALERTMANAGER, "      receiver: external-heartbeat\n", "      receiver: nobody\n")
    found = findings(tree, "alertmanager")
    assert any(f.startswith("amtool check-config") and "nobody" in f for f in found)


def test_wrong_page_route_fails(tree: Path) -> None:
    edit(tree / ALERTMANAGER, "      continue: true\n", "")
    [finding] = findings(tree, "alertmanager")
    assert finding.startswith(
        "route severity=page alertname=BothyExample -> external-oncall,bothy-ops: exit status 1"
    )
    assert "Expected receivers did not match" in finding


# ---- loki, tempo --------------------------------------------------------------------------


def test_unknown_loki_key_fails(tree: Path) -> None:
    edit(
        tree / "infra/observability/loki/loki.yaml",
        "auth_enabled: false\n",
        "auth_enabled: false\nbogus_key: 1\n",
    )
    [finding] = findings(tree, "loki")
    assert "bogus_key" in finding


def test_unknown_tempo_key_fails(tree: Path) -> None:
    edit(
        tree / "infra/observability/tempo/tempo.yaml",
        "stream_over_http_enabled: true\n",
        "stream_over_http_enabled: true\nbogus_key: 1\n",
    )
    [finding] = findings(tree, "tempo")
    assert "bogus_key" in finding


# ---- sloth --------------------------------------------------------------------------------

SLO = Path("infra/observability/slo")


def test_edited_generated_rules_fail(tree: Path) -> None:
    edit(tree / PROM / "rules" / "slo-api.yaml", "sloth_window: 5m", "sloth_window: 6m")
    [finding] = findings(tree, "sloth")
    first, *diff = finding.splitlines()
    assert first == (
        "infra/observability/prometheus/rules/slo-api.yaml differs from a fresh "
        "`sloth generate` of infra/observability/slo/api.yaml"
    )
    assert "    -      sloth_window: 6m" in diff
    assert "    +      sloth_window: 5m" in diff


def test_rules_without_source_fail(tree: Path) -> None:
    shutil.copy2(tree / PROM / "rules" / "slo-api.yaml", tree / PROM / "rules" / "slo-extra.yaml")
    assert findings(tree, "sloth") == (
        "infra/observability/prometheus/rules/slo-extra.yaml has no Sloth source "
        "infra/observability/slo/extra.yaml",
    )


def test_source_without_generated_rules_fails(tree: Path) -> None:
    (tree / PROM / "rules" / "slo-push.yaml").unlink()
    assert findings(tree, "sloth") == (
        "infra/observability/prometheus/rules/slo-push.yaml is missing; generate it from "
        "infra/observability/slo/push.yaml",
    )


def test_invalid_sloth_file_fails(tree: Path) -> None:
    edit(tree / SLO / "push.yaml", "objective: 99.5", "objective: 101")
    [finding] = findings(tree, "sloth")
    assert finding.startswith("sloth generate infra/observability/slo/push.yaml: exit status")


# ---- slo-docs -----------------------------------------------------------------------------

SLOS_MD = Path("docs/observability/slos.md")


def test_objective_mismatch_fails(tree: Path) -> None:
    edit(tree / SLO / "sync.yaml", "objective: 99.0", "objective: 98.0")
    assert findings(tree, "slo-docs") == (
        "bothy-sync/reconnect: Ziel is 99.0 %, the Sloth file says 98.0 %",
    )


def test_threshold_mismatch_fails(tree: Path) -> None:
    edit(tree / SLOS_MD, "| 0.5 s |", "| 1 s |")
    assert findings(tree, "slo-docs") == (
        "bothy-sync/delivery-latency: Schwelle is 1 s, the Sloth file says 0.5 s",
    )


def test_missing_slo_row_fails(tree: Path) -> None:
    text = (tree / SLOS_MD).read_text(encoding="utf-8")
    row = next(line for line in text.splitlines() if line.startswith("| `bothy-push/"))
    edit(tree / SLOS_MD, row + "\n", "")
    assert findings(tree, "slo-docs") == (
        'bothy-push/provider-acceptance: no row in docs/observability/slos.md "SLOs"',
    )


def test_extra_and_duplicate_slo_rows_fail(tree: Path) -> None:
    text = (tree / SLOS_MD).read_text(encoding="utf-8")
    row = next(line for line in text.splitlines() if line.startswith("| `bothy-push/"))
    extra = row.replace("bothy-push/provider-acceptance", "bothy-push/gone")
    edit(tree / SLOS_MD, row + "\n", row + "\n" + row + "\n" + extra + "\n")
    assert findings(tree, "slo-docs") == (
        'bothy-push/provider-acceptance: 2 rows in docs/observability/slos.md "SLOs"',
        '`bothy-push/gone`: row in docs/observability/slos.md "SLOs" without an SLO in '
        "infra/observability/slo/",
    )


def test_alarm_column_follows_page_alert_disable(tree: Path) -> None:
    edit(
        tree / SLO / "agents.yaml",
        "      page_alert:\n        disable: true\n",
        "      page_alert:\n        labels:\n          severity: page\n",
    )
    assert findings(tree, "slo-docs") == (
        "bothy-agents/turn-duration: Alarm is nur Ticket, the Sloth file says Page, Ticket",
    )


# ---- alert-thresholds ---------------------------------------------------------------------

ALERTS = PROM / "rules" / "bothy-alerts.yaml"
ALERTING_MD = Path("docs/observability/alerting.md")


def test_equal_threshold_mismatch_fails(tree: Path) -> None:
    edit(tree / ALERTS, "> 93600", "> 90000")
    assert findings(tree, "alert-thresholds") == (
        "BothyBackupStale: threshold 90000 s in bothy-alerts.yaml must equal backup.max_age "
        "(26h = 93600 s)",
    )


def test_over_threshold_not_greater_fails(tree: Path) -> None:
    edit(
        tree / ALERTS,
        'bothy_agent_heartbeat_age_seconds{job="bothy/bothy-server"}) > 300',
        'bothy_agent_heartbeat_age_seconds{job="bothy/bothy-server"}) > 90',
    )
    assert findings(tree, "alert-thresholds") == (
        "BothyAgentOffline: threshold 90 s in bothy-alerts.yaml must be greater than "
        "agent.presence_timeout (90s = 90 s)",
    )


def test_unknown_config_name_fails(tree: Path) -> None:
    edit(tree / ALERTING_MD, "| `backup.max_age` |", "| `backup.max_agex` |")
    [finding] = findings(tree, "alert-thresholds")
    assert finding.startswith("BothyBackupStale: backup.max_agex: not found in ")


def test_alert_set_mismatch_fails(tree: Path) -> None:
    text = (tree / ALERTING_MD).read_text(encoding="utf-8")
    row = next(line for line in text.splitlines() if line.startswith("| `BothyVolumeFull` |"))
    edit(tree / ALERTING_MD, row + "\n", row.replace("BothyVolumeFull", "BothyDiskFull") + "\n")
    assert findings(tree, "alert-thresholds") == (
        'BothyDiskFull: in docs/observability/alerting.md "Weitere Alarme" but not in '
        "infra/observability/prometheus/rules/bothy-alerts.yaml",
        "BothyVolumeFull: in infra/observability/prometheus/rules/bothy-alerts.yaml but not in "
        'docs/observability/alerting.md "Weitere Alarme"',
    )


def test_threshold_without_comparison_fails(tree: Path) -> None:
    edit(
        tree / ALERTS,
        "time() - max(bothy_backup_wal_last_archived_timestamp_seconds) > 900",
        "time() - max(bothy_backup_wal_last_archived_timestamp_seconds) < 900",
    )
    assert findings(tree, "alert-thresholds") == (
        "BothyWalArchiveStale: the expression has no final `> <number>` comparison",
    )


# ---- tofu ---------------------------------------------------------------------------------


def test_invalid_hcl_fails(tree: Path) -> None:
    (tree / "infra/tofu/modules/hetzner/broken.tf").write_text('variable "x" {\n', encoding="utf-8")
    [finding] = findings(tree, "tofu")
    assert finding.startswith("hetzner: tofu init: exit status 1")
    # The finding names the HCL error, not only that init failed for some reason.
    assert "Error: Unclosed configuration block" in finding
    assert "on broken.tf line 1" in finding


def test_invalid_reference_fails_in_validate(tree: Path) -> None:
    (tree / "infra/tofu/modules/aws/outputs.tf").write_text(
        'output "x" {\n  value = var.does_not_exist\n}\n', encoding="utf-8"
    )
    [finding] = findings(tree, "tofu")
    assert finding.startswith("aws: tofu validate: exit status 1")
    assert "does_not_exist" in finding


def test_tofu_needs_a_module(tree: Path) -> None:
    for module in (tree / "infra/tofu/modules").iterdir():
        for tf in module.glob("*.tf"):
            tf.unlink()
    assert findings(tree, "tofu") == (
        "no OpenTofu module (a directory with .tf, .tf.json, .tofu or .tofu.json files) under "
        "infra/tofu/modules/",
    )


# ---- edge cases ---------------------------------------------------------------------------


def test_pinned_tool_rejects_an_unpinned_name() -> None:
    with pytest.raises(opscheck.fetch_tools.LockError, match="nope is not pinned"):
        opscheck.pinned_tool("nope")


def test_unpinned_validator_fails_tool_versions(tree: Path) -> None:
    lock = tree / "tools/bin/tools.lock.json"
    edit(lock, '"amtool": {', '"amtool-renamed": {')
    assert findings(tree, "tool-versions") == ("amtool is not pinned in tools/bin/tools.lock.json",)


def test_alert_defined_twice_and_documented_twice_fail(tree: Path) -> None:
    edit(
        tree / ALERTS,
        "  - name: bothy-data\n    rules:\n",
        "  - name: bothy-data\n    rules:\n"
        "      - alert: BothyVolumeFull\n        expr: vector(1)\n",
    )
    text = (tree / ALERTING_MD).read_text(encoding="utf-8")
    row = next(line for line in text.splitlines() if line.startswith("| `BothyAuditChainBroken` |"))
    edit(tree / ALERTING_MD, row + "\n", row + "\n" + row + "\n")
    assert findings(tree, "alert-thresholds") == (
        "BothyVolumeFull: defined twice in infra/observability/prometheus/rules/bothy-alerts.yaml",
        'BothyAuditChainBroken: 2 rows in docs/observability/alerting.md "Weitere Alarme"',
    )


def test_rules_directory_without_rule_file_fails(tree: Path) -> None:
    for path in (tree / PROM / "rules").iterdir():
        path.unlink()
    (tree / PROM / "rules" / ".DS_Store").write_bytes(b"")
    assert findings(tree, "prometheus-rules") == (
        "no rule file in infra/observability/prometheus/rules/",
    )


def test_slo_docs_reads_event_slis_with_one_bound_only(tree: Path) -> None:
    edit(
        tree / SLO / "api.yaml",
        "        error_query: sum(rate(",
        "        error_query_moved: sum(rate(",
    )
    edit(tree / SLO / "sync.yaml", 'le="2.5"', 'le="2.5",le="5"')
    assert findings(tree, "slo-docs") == (
        "bothy-api/requests-availability: no sli.events.error_query; slo-docs reads event SLIs "
        "only",
        "bothy-sync/reconnect: several le bounds 2.5, 5 in the error query",
    )


def test_markdown_table_needs_a_separator_row() -> None:
    with pytest.raises(ValueError, match="header and a separator"):
        opscheck.markdown_table("## T\n\n| just a line |\n\n", "T")


def test_script_runs_standalone_as_in_the_makefile() -> None:
    """`make ops-check` runs the file as a script; it must find fetch_tools on its own."""
    script = REPO / "tools" / "opscheck" / "opscheck.py"
    proc = subprocess.run(
        [sys.executable, str(script), "--list"], capture_output=True, text=True, check=False
    )
    assert proc.returncode == 0, proc.stderr
    assert proc.stdout.splitlines() == ALL_CHECKS


def test_tofu_cli_configuration_allows_no_registry_access(tree: Path, tmp_path: Path) -> None:
    """The gate makes no network call: providers may come only from an empty local mirror."""
    script = (
        '/usr/bin/grep -q "filesystem_mirror" "$TF_CLI_CONFIG_FILE" '
        '|| { echo "registry access allowed" >&2; exit 1; }'
    )
    fake = fake_tool(tmp_path / "fake-bin", "tofu", script)
    [result] = opscheck.run_checks(tree, only=["tofu"], tool=lambda name: fake)
    assert result.findings == ()


def test_module_with_provider_fails_offline(tree: Path) -> None:
    (tree / "infra/tofu/modules/aws/versions.tf").write_text(
        "terraform {\n  required_providers {\n"
        '    random = {\n      source  = "hashicorp/random"\n      version = "3.6.0"\n    }\n'
        "  }\n}\n",
        encoding="utf-8",
    )
    [finding] = findings(tree, "tofu")
    assert finding.startswith("aws: tofu init: exit status 1")
    assert "hashicorp/random" in finding


# ---- findings of the closing review -------------------------------------------------------


def _visible_rule_files(tree: Path) -> list[str]:
    return sorted(p.name for p in (tree / PROM / "rules").iterdir() if not p.name.startswith("."))


def test_rule_files_glob_that_matches_nothing_fails(tree: Path) -> None:
    """promtool accepts a glob without matches; Prometheus would then load no rule at all."""
    edit(
        tree / PROM / "prometheus.yaml",
        "/etc/prometheus/rules/*.yaml",
        "/etc/prometheus/rulez/*.yaml",
    )
    found = findings(tree, "prometheus-config")
    assert found[0] == "rule_files entry /etc/prometheus/rulez/*.yaml matches no file"
    assert found[1:] == tuple(
        f"infra/observability/prometheus/rules/{name} is not loaded by rule_files of "
        "infra/observability/prometheus/prometheus.yaml"
        for name in _visible_rule_files(tree)
    )


def test_rule_files_entry_outside_the_mounted_directory_fails(tree: Path) -> None:
    edit(
        tree / PROM / "prometheus.yaml",
        "  - /etc/prometheus/rules/*.yaml\n",
        "  - /etc/prometheus/rules/*.yaml\n  - /opt/rules/*.yaml\n",
    )
    assert findings(tree, "prometheus-config") == (
        "rule_files entry /opt/rules/*.yaml lies outside /etc/prometheus/, where Compose mounts "
        "infra/observability/prometheus/; the gate cannot check it",
    )


def test_rule_file_that_prometheus_does_not_load_fails(tree: Path) -> None:
    shutil.copy2(tree / ALERTS, tree / PROM / "rules" / "extra.yml")
    assert findings(tree, "prometheus-config") == (
        "infra/observability/prometheus/rules/extra.yml is not loaded by rule_files of "
        "infra/observability/prometheus/prometheus.yaml",
    )


COLLECTOR_PROBE = r"""
for a in "$@"; do
  case "$a" in
    --config=*)
      f="${a#--config=}"
      case "$f" in
        *datadog.yaml)
          /usr/bin/grep -q "hostname: validate.example.com" "$f" \
            || { echo "datadog host name not pinned in $f" >&2; exit 1; } ;;
      esac
      for s in $(/usr/bin/sed -n 's/.*[$][{]file:\([^}]*\)[}].*/\1/p' "$f"); do
        /usr/bin/grep -q EXAMPLE "$s" || { echo "no placeholder secret $s" >&2; exit 1; }
        /usr/bin/wc -c < "$s" | /usr/bin/awk '{ exit !($1 >= 32) }' \
          || { echo "secret $s shorter than 32 bytes" >&2; exit 1; }
      done ;;
  esac
done
[ -n "$GRAFANA_OTLP_ENDPOINT" ] || { echo "GRAFANA_OTLP_ENDPOINT missing" >&2; exit 1; }
exit 0
"""


def test_collector_gets_placeholder_secrets_and_the_pinned_datadog_host_name(
    tree: Path, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Without the pinned host name `validate` probes cloud metadata endpoints (network)."""
    fake = fake_tool(tmp_path / "fake-bin", "otelcol-contrib", COLLECTOR_PROBE)
    [result] = opscheck.run_checks(tree, only=["collector"], tool=lambda name: fake)
    assert result.findings == ()
    # The probe notices a missing pin, so the test above would fail without it.
    monkeypatch.setattr(opscheck, "_pin_datadog_hostname", lambda path: None)
    [result] = opscheck.run_checks(tree, only=["collector"], tool=lambda name: fake)
    assert result.findings
    assert all("datadog host name not pinned" in f for f in result.findings)
    assert all("datadog" in f.splitlines()[0] for f in result.findings)


def test_broken_provider_lock_is_a_finding(tree: Path) -> None:
    """The mirror is filled from the lock files of the test roots; a broken one stops the check."""
    (tree / "infra/tofu/test/cloud/.terraform.lock.hcl").write_text(
        "# no provider\n", encoding="utf-8"
    )
    [finding] = findings(tree, "tofu")
    assert finding.startswith("provider mirror: ")
    assert "no provider" in finding


def test_placeholder_secret_is_long_enough_for_the_pseudonym_key() -> None:
    assert len(opscheck.SECRET_PLACEHOLDER) >= 32
    assert b"EXAMPLE" in opscheck.SECRET_PLACEHOLDER


def test_module_from_a_registry_fails_without_network(tree: Path) -> None:
    """Module sources are fetched over HTTP too; the closed proxy stops them."""
    (tree / "infra/tofu/modules/aws/main.tf").write_text(
        'module "remote" {\n  source  = "registry.example.com/team/name/provider"\n'
        '  version = "1.0.0"\n}\n',
        encoding="utf-8",
    )
    [finding] = findings(tree, "tofu")
    assert finding.startswith("aws: tofu init: exit status 1")
    assert "127.0.0.1" in finding


def test_module_with_a_sibling_module_source(tree: Path) -> None:
    """All modules sit in one work copy, so `../<sibling>` resolves whatever the names."""
    modules = tree / "infra/tofu/modules"
    (modules / "zz-common").mkdir()
    (modules / "zz-common" / "variables.tf").write_text(
        'variable "label" {\n  type    = string\n  default = "bothy"\n}\n', encoding="utf-8"
    )
    (modules / "aws" / "common.tf").write_text(
        'module "common" {\n  source = "../zz-common"\n}\n', encoding="utf-8"
    )
    assert findings(tree, "tofu") == ()


@pytest.mark.parametrize(
    ("old", "new"),
    [
        ("> 93600", "> 93600 * 2"),
        ("> 93600", "> 26h"),
        ("> 93600", "> bool 93600"),
    ],
)
def test_threshold_that_is_not_a_plain_number_fails(tree: Path, old: str, new: str) -> None:
    edit(tree / ALERTS, old, new)
    assert findings(tree, "alert-thresholds") == (
        "BothyBackupStale: the final `>` comparison has no plain number on its right side",
    )


def test_threshold_is_the_final_comparison(tree: Path) -> None:
    rule = "expr: time() - max by (bothy_backup_kind) (bothy_backup_last_success_timestamp_seconds)"
    edit(tree / ALERTS, rule, "expr: vector(1) > 0 and " + rule.removeprefix("expr: "))
    assert findings(tree, "alert-thresholds") == ()
    edit(tree / ALERTS, "> 93600", "> 93601")
    assert findings(tree, "alert-thresholds") == (
        "BothyBackupStale: threshold 93601 s in bothy-alerts.yaml must equal backup.max_age "
        "(26h = 93600 s)",
    )


def test_threshold_ignores_promql_comments_and_strings(tree: Path) -> None:
    metric = "bothy_backup_last_success_timestamp_seconds"
    before = f"        expr: time() - max by (bothy_backup_kind) ({metric}) > 93600\n"
    after = (
        "        expr: |\n"
        f'          time() - max by (bothy_backup_kind) ({metric}{{note="a \\" > 1"}}) > 93600'
        " # was > 1\n"
    )
    edit(tree / ALERTS, before, after)
    assert findings(tree, "alert-thresholds") == ()


def test_objective_with_two_decimals(tree: Path) -> None:
    edit(tree / SLO / "sync.yaml", "objective: 99.5", "objective: 99.95")
    edit(tree / SLOS_MD, "| 0.5 s | 99.5 % |", "| 0.5 s | 99.95 % |")
    assert findings(tree, "slo-docs") == ()
    edit(tree / SLOS_MD, "| 0.5 s | 99.95 % |", "| 0.5 s | 100.0 % |")
    assert findings(tree, "slo-docs") == (
        "bothy-sync/delivery-latency: Ziel is 100.0 %, the Sloth file says 99.95 %",
    )


FENCES = """\
## A

~~~
```
## Fake
| x | y |
|---|---|
~~~

| a | b |
|---|---|
| 1 | 2 |

````md
## B
```
| f | g |
|---|---|
````

## B
| c | d |
|---|---|
| 3 | 4 |
"""


def test_markdown_table_follows_fence_type_and_length() -> None:
    assert opscheck.markdown_table(FENCES, "A") == [{"a": "1", "b": "2"}]
    assert opscheck.markdown_table(FENCES, "B") == [{"c": "3", "d": "4"}]
    with pytest.raises(ValueError, match="no heading"):
        opscheck.markdown_table(FENCES, "Fake")


def test_env_example_values_are_read_as_compose_reads_them(tree: Path) -> None:
    edit(tree / ENV_EXAMPLE, "BOTHY_LOKI_VERSION=3.7.8", 'BOTHY_LOKI_VERSION="3.7.8"')
    edit(tree / ENV_EXAMPLE, "BOTHY_TEMPO_VERSION=3.1.0", "BOTHY_TEMPO_VERSION=3.1.0  # pinned")
    edit(
        tree / ENV_EXAMPLE,
        "BOTHY_ALERTMANAGER_VERSION=v0.34.1",
        "BOTHY_ALERTMANAGER_VERSION='v0.34.1'",
    )
    assert findings(tree, "tool-versions") == ()
    edit(tree / ENV_EXAMPLE, 'BOTHY_LOKI_VERSION="3.7.8"', 'BOTHY_LOKI_VERSION="3.7.9"')
    assert findings(tree, "tool-versions") == (
        "BOTHY_LOKI_VERSION in infra/compose/.env.example is 3.7.9, the pinned loki is 3.7.8",
    )


def test_collector_needs_an_overlay(tree: Path) -> None:
    for overlay in (tree / OTEL / "exporters").glob("*.yaml"):
        overlay.unlink()
    assert findings(tree, "collector") == ("no overlay in infra/otel/exporters/",)


def test_sloth_needs_a_source(tree: Path) -> None:
    for path in [*(tree / SLO).glob("*.yaml"), *(tree / PROM / "rules").glob("slo-*.yaml")]:
        path.unlink()
    assert findings(tree, "sloth") == ("no Sloth file in infra/observability/slo/",)


def test_wrong_ticket_route_fails(tree: Path) -> None:
    edit(
        tree / ALERTMANAGER,
        "    - matchers: ['severity = \"ticket\"']\n      receiver: bothy-ops\n",
        "    - matchers: ['severity = \"ticket\"']\n      receiver: external-oncall\n",
    )
    [finding] = findings(tree, "alertmanager")
    assert finding.startswith(
        "route severity=ticket alertname=BothyExample -> bothy-ops: exit status 1"
    )


def test_wrong_watchdog_route_fails(tree: Path) -> None:
    edit(tree / ALERTMANAGER, "      receiver: external-heartbeat\n", "      receiver: bothy-ops\n")
    [finding] = findings(tree, "alertmanager")
    assert finding.startswith(
        "route alertname=Watchdog severity=heartbeat -> external-heartbeat: exit status 1"
    )


def test_only_can_be_repeated(tree: Path, capsys: pytest.CaptureFixture[str]) -> None:
    args = ["--root", str(tree), "--only", "tool-versions", "--only", "slo-docs"]
    assert opscheck.main(args) == 0
    assert capsys.readouterr().out == "tool-versions: ok\nslo-docs: ok\n"


def test_every_public_symbol_is_documented() -> None:
    """Functions, classes and module-level names of opscheck.py carry a docstring."""
    module = ast.parse(Path(opscheck.__file__).read_text(encoding="utf-8"))
    missing = []
    body = module.body
    for index, node in enumerate(body):
        if isinstance(node, (ast.FunctionDef, ast.ClassDef)):
            if not node.name.startswith("_") and ast.get_docstring(node) is None:
                missing.append(node.name)
            continue
        targets = []
        if isinstance(node, ast.Assign):
            targets = [t.id for t in node.targets if isinstance(t, ast.Name)]
        elif isinstance(node, ast.AnnAssign) and isinstance(node.target, ast.Name):
            targets = [node.target.id]
        documented = (
            index + 1 < len(body)
            and isinstance(body[index + 1], ast.Expr)
            and isinstance(body[index + 1].value, ast.Constant)
            and isinstance(body[index + 1].value.value, str)
        )
        missing += [name for name in targets if not name.startswith("_") and not documented]
    assert missing == []


# ---- findings of re-review round 1 --------------------------------------------------------


def test_env_example_value_with_quotes_and_comment(tree: Path) -> None:
    edit(tree / ENV_EXAMPLE, "BOTHY_LOKI_VERSION=3.7.8", 'BOTHY_LOKI_VERSION="3.7.8" # pinned')
    assert findings(tree, "tool-versions") == ()


def test_relative_rule_files_entry_resolves_against_the_configuration(tree: Path) -> None:
    """Prometheus reads a relative entry relative to prometheus.yaml, as the check does."""
    edit(
        tree / PROM / "prometheus.yaml", "  - /etc/prometheus/rules/*.yaml\n", "  - rules/*.yaml\n"
    )
    assert findings(tree, "prometheus-config") == ()


INDENTED_FENCE = """\
## A

1. Item with an example:

       ```
       | x | y |
       |---|---|
       ```

| a | b |
|---|---|
| 1 | 2 |
"""


def test_markdown_table_skips_indented_fences() -> None:
    assert opscheck.markdown_table(INDENTED_FENCE, "A") == [{"a": "1", "b": "2"}]


# ---- findings of the red team -------------------------------------------------------------

SANDBOX_EXEC = Path("/usr/bin/sandbox-exec")
NO_BREAK_SPACE = chr(0xA0)
NO_NETWORK_PROFILE = """\
(version 1)
(allow default)
(deny network-outbound (remote ip "*:*") (with send-signal SIGKILL))
(allow network-outbound (remote ip "localhost:*"))
"""


@pytest.mark.skipif(
    not SANDBOX_EXEC.exists(),
    reason="needs the macOS sandbox to see a blocked connection; Linux has no equivalent",
)
def test_no_validator_reaches_the_network(tmp_path: Path) -> None:
    """Every validator runs in a sandbox that kills it on any connection beyond loopback.

    A proxy does not stop a client with its own transport (the GCP metadata probe of the
    datadog exporter); this test sees every attempt, because the killed validator becomes
    a finding.
    """
    profile = tmp_path / "no-network.sb"
    profile.write_text(NO_NETWORK_PROFILE, encoding="utf-8")

    def sandboxed(name: str) -> Path:
        real = opscheck.pinned_tool(name)
        script = f'exec {SANDBOX_EXEC} -f "{profile}" "{real}" "$@"'
        return fake_tool(tmp_path / "sandboxed", name, script)

    results = opscheck.run_checks(REPO, tool=sandboxed)
    assert [r.name for r in results] == ALL_CHECKS
    assert {r.name: r.findings for r in results if r.findings} == {}


def test_validators_get_no_cloud_metadata_host(tmp_path: Path) -> None:
    """The GCP metadata client ignores the proxy; its host points at the closed port too."""
    ctx = _context(tmp_path)
    probe = fake_tool(tmp_path / "bin", "probe", '/usr/bin/env > "$HOME/env.txt"')
    assert opscheck.run_command(ctx, [probe]) == []
    env = (ctx.workdir / "home" / "env.txt").read_text(encoding="utf-8").splitlines()
    assert f"GCE_METADATA_HOST={opscheck.NO_NETWORK_PROXY.removeprefix('http://')}" in env


@pytest.mark.parametrize(
    "cell",
    [
        "`backup.max_age` (26 h)",
        "backup.max_age",
        "über" + NO_BREAK_SPACE + "`backup.max_age`",
        "gleich `backup.max_age`",
    ],
)
def test_threshold_cell_with_a_configuration_value_in_another_form_fails(
    tree: Path, cell: str
) -> None:
    """A cell that names a configuration value must use one of the two checked forms."""
    edit(tree / ALERTING_MD, "| `backup.max_age` |", f"| {cell} |")
    if cell.startswith("über"):
        # A no-break space after "über" is still the strictly-greater form.
        edit(tree / ALERTS, "> 93600", "> 93601")
        assert findings(tree, "alert-thresholds") == ()
        return
    assert findings(tree, "alert-thresholds") == (
        "BothyBackupStale: the cell Konfigurationswert names backup.max_age but is neither "
        "`` `backup.max_age` `` nor ``über `backup.max_age` ``",
    )


def test_alert_with_only_a_quiet_case_is_untested(tree: Path) -> None:
    tests = tree / PROM / "tests" / "bothy-alerts.test.yaml"
    text = tests.read_text(encoding="utf-8")
    start = text.index("  - name: BothyVolumeFull fires just below 10 percent free")
    end = text.index("  - name: BothyVolumeFull stays quiet at 10 percent free")
    tests.write_text(text[:start] + text[end:], encoding="utf-8")
    assert findings(tree, "prometheus-rule-tests") == (
        "BothyVolumeFull: no promtool unit test expects it to fire in "
        "infra/observability/prometheus/tests/*.test.yaml",
    )


def test_allowlist_must_not_allow_all_keys(tree: Path) -> None:
    edit(
        tree / OTEL / "collector.yaml", "    allow_all_keys: false\n", "    allow_all_keys: true\n"
    )
    assert findings(tree, "collector-allowlist") == (
        "processors.redaction/allowlist.allow_all_keys of infra/otel/collector.yaml must be false",
    )


def test_allowlist_must_guard_every_trace_and_log_input(tree: Path) -> None:
    edit(
        tree / OTEL / "collector.yaml",
        "transform/scrub, redaction/allowlist, tail_sampling, batch]",
        "transform/scrub, tail_sampling, batch]",
    )
    assert findings(tree, "collector-allowlist") == (
        "pipeline traces/in of infra/otel/collector.yaml receives from otlp but does not run "
        "redaction/allowlist",
    )


def test_overlay_must_not_redefine_the_allowlist(tree: Path) -> None:
    (tree / OTEL / "exporters" / "zz-open.yaml").write_text(
        "processors:\n  redaction/allowlist:\n    allowed_keys: [enduser.id, bothy.action]\n",
        encoding="utf-8",
    )
    assert findings(tree, "collector-allowlist") == (
        "infra/otel/exporters/zz-open.yaml redefines processors.redaction/allowlist",
    )


def test_overlay_must_not_redefine_a_base_pipeline(tree: Path) -> None:
    (tree / OTEL / "exporters" / "zz-bypass.yaml").write_text(
        "service:\n  pipelines:\n    traces/in:\n      receivers: [otlp]\n"
        "      exporters: [forward/traces]\n",
        encoding="utf-8",
    )
    assert findings(tree, "collector-allowlist") == (
        "infra/otel/exporters/zz-bypass.yaml redefines the base pipeline traces/in",
        "pipeline traces/in of infra/otel/exporters/zz-bypass.yaml receives from otlp but does "
        "not run redaction/allowlist",
    )


# ---- findings of the red team on d73c418 --------------------------------------------------


def test_alarm_column_follows_ticket_alert_disable(tree: Path) -> None:
    """Without its ticket alert an SLO has only the page alert; the row must say so."""
    edit(
        tree / SLO / "push.yaml",
        "      ticket_alert:\n        labels:\n          severity: ticket\n",
        "      ticket_alert:\n        disable: true\n",
    )
    assert findings(tree, "slo-docs") == (
        "bothy-push/provider-acceptance: Alarm is Page, Ticket, the Sloth file says nur Page",
    )
    row = "| 99.5 % | Page, Ticket | `sli.push_acceptance` |"
    edit(tree / SLOS_MD, row, row.replace("Page, Ticket", "nur Page"))
    assert findings(tree, "slo-docs") == ()


def test_alarm_column_without_any_alert(tree: Path) -> None:
    edit(
        tree / SLO / "agents.yaml",
        "        disable: true\n      ticket_alert:\n        labels:\n          severity: ticket\n",
        "        disable: true\n      ticket_alert:\n        disable: true\n",
    )
    assert findings(tree, "slo-docs") == (
        "bothy-agents/turn-duration: Alarm is nur Ticket, the Sloth file says keine",
    )
    edit(tree / SLOS_MD, "| 95.0 % | nur Ticket |", "| 95.0 % | keine |")
    assert findings(tree, "slo-docs") == ()


@pytest.mark.parametrize("matcher", ["le='0.5'", "le=`0.5`", 'le = "0.5"'])
def test_slo_docs_reads_every_quoting_of_the_le_bound(tree: Path, matcher: str) -> None:
    """PromQL quotes a label value with ", ' or `; every form gives the same Schwelle."""
    edit(tree / SLO / "sync.yaml", 'le="0.5"', matcher)
    assert findings(tree, "slo-docs") == ()
    edit(tree / SLOS_MD, "| 0.5 s |", "| keine |")
    assert findings(tree, "slo-docs") == (
        "bothy-sync/delivery-latency: Schwelle is keine, the Sloth file says 0.5 s",
    )


def test_slo_docs_reads_an_empty_le_as_no_bound(tree: Path) -> None:
    edit(tree / SLO / "sync.yaml", 'le="0.5"', 'le=""')
    assert findings(tree, "slo-docs") == (
        "bothy-sync/delivery-latency: Schwelle is 0.5 s, the Sloth file says keine",
    )


@pytest.mark.parametrize("matcher", ['le=~"0.5"', "le != '0.5'"])
def test_slo_docs_rejects_an_le_matcher_that_is_no_bound(tree: Path, matcher: str) -> None:
    """A regex or negated `le` matcher names no bound; the row could not be checked."""
    edit(tree / SLO / "sync.yaml", 'le="0.5"', matcher)
    assert findings(tree, "slo-docs") == (
        f"bothy-sync/delivery-latency: le matcher {matcher} in the error query is no bound; "
        'slo-docs reads le="<bound>" only',
    )


@pytest.mark.parametrize("name", ["main.tofu", "main.tf.json", "main.tofu.json"])
def test_module_of_other_configuration_files_is_checked(tree: Path, name: str) -> None:
    """OpenTofu also reads .tofu and JSON files; a module made only of them is checked too."""
    module = tree / "infra/tofu/modules/scaleway"
    module.mkdir()
    (module / name).write_text("this is not hcl {\n", encoding="utf-8")
    [finding] = findings(tree, "tofu")
    assert finding.startswith("scaleway: tofu init: exit status 1")


VALID_TEMPLATE = '{{ define "bothy.title" }}{{ .Status }}{{ end }}\n'
BROKEN_TEMPLATE = '{{ define "bothy.title" }}{{ .Status }\n'


def _alertmanager_template(tree: Path, entry: str, text: str) -> Path:
    """Add a template file and a `templates` entry to the copied Alertmanager configuration."""
    templates = (tree / ALERTMANAGER).parent / "templates"
    templates.mkdir()
    (templates / "bothy.tmpl").write_text(text, encoding="utf-8")
    edit(tree / ALERTMANAGER, "\nroute:\n", f"\ntemplates:\n  - {entry}\n\nroute:\n")
    return templates / "bothy.tmpl"


@pytest.mark.parametrize("entry", ["/etc/alertmanager/templates/*.tmpl", "templates/*.tmpl"])
def test_alertmanager_templates_are_checked_where_compose_mounts_them(
    tree: Path, entry: str
) -> None:
    """Compose mounts the directory at /etc/alertmanager/; the check reads the templates there."""
    template = _alertmanager_template(tree, entry, VALID_TEMPLATE)
    assert findings(tree, "alertmanager") == ()
    template.write_text(BROKEN_TEMPLATE, encoding="utf-8")
    [finding] = findings(tree, "alertmanager")
    assert finding.startswith("amtool check-config: exit status 1")
    assert 'bothy.tmpl:1: unexpected "}" in operand' in finding


@pytest.mark.parametrize(
    ("entry", "message"),
    [
        (
            "/etc/alertmanager/tmpl/*.tmpl",
            "templates entry /etc/alertmanager/tmpl/*.tmpl matches no file",
        ),
        (
            "/opt/templates/*.tmpl",
            "templates entry /opt/templates/*.tmpl lies outside /etc/alertmanager/, where "
            "Compose mounts infra/observability/alertmanager/; the gate cannot check it",
        ),
    ],
)
def test_alertmanager_template_entry_must_match_a_mounted_file(
    tree: Path, entry: str, message: str
) -> None:
    """amtool accepts a pattern without a match; Alertmanager would then load no template."""
    _alertmanager_template(tree, entry, BROKEN_TEMPLATE)
    assert findings(tree, "alertmanager") == (message,)


def test_alertmanager_yaml_error_is_reported_by_amtool(tree: Path) -> None:
    """A file that is no YAML keeps amtool's finding and the route findings."""
    edit(tree / ALERTMANAGER, "  group_wait: 30s\n", "  group_wait: 30s\n bad: [\n")
    found = findings(tree, "alertmanager")
    assert found[0].startswith("amtool check-config: exit status 1")
    assert [f.split(": exit status")[0] for f in found[1:]] == [
        "route severity=page alertname=BothyExample -> external-oncall,bothy-ops",
        "route severity=ticket alertname=BothyExample -> bothy-ops",
        "route alertname=Watchdog severity=heartbeat -> external-heartbeat",
    ]


SENTRY_KEY_REFERENCE = "${file:/run/bothy/secrets/sentry_key}"


def _provider_finding(scheme: str) -> str:
    return (
        f"infra/otel/exporters/sentry.yaml: ${{{scheme}:...}} uses the config provider {scheme}, "
        "which is not allowed; collector files may use only env, file and yaml references "
        "(most other providers fetch their value over the network)"
    )


@pytest.mark.parametrize(
    ("reference", "scheme"),
    [
        ("${https://config.example.com/sentry_key}", "https"),
        ("${http://config.example.com/sentry_key}", "http"),
        ("${s3://bucket.s3.example.com/sentry_key}", "s3"),
        (
            "${googlesecretmanager:projects/example/secrets/sentry/versions/1}",
            "googlesecretmanager",
        ),
        # The collector resolves the inner reference first, so a default may not hide it.
        ("${env:SENTRY_KEY:-${https://config.example.com/sentry_key}}", "https"),
        ("$$${https://config.example.com/sentry_key}", "https"),  # `$$` then a reference
    ],
)
def test_remote_reference_in_a_collector_file_fails_before_validate(
    tree: Path, tmp_path: Path, reference: str, scheme: str
) -> None:
    """A provider other than env, file or yaml may reach the network; validate never runs."""
    marker = tmp_path / "validated"
    fake = fake_tool(tmp_path / "fake-bin", "otelcol-contrib", f'echo ran > "{marker}"')
    edit(tree / OTEL / "exporters" / "sentry.yaml", SENTRY_KEY_REFERENCE, reference)
    [result] = opscheck.run_checks(tree, only=["collector"], tool=lambda name: fake)
    assert result.findings == (_provider_finding(scheme),)
    assert not marker.exists()


def test_remote_reference_in_a_collector_file_that_is_no_yaml(tree: Path, tmp_path: Path) -> None:
    """A file PyYAML cannot read is searched as text; the collector may still read it."""
    marker = tmp_path / "validated"
    fake = fake_tool(tmp_path / "fake-bin", "otelcol-contrib", f'echo ran > "{marker}"')
    edit(
        tree / OTEL / "exporters" / "sentry.yaml",
        SENTRY_KEY_REFERENCE,
        '${https://config.example.com/sentry_key}" [unclosed',  # text after the closing quote
    )
    [result] = opscheck.run_checks(tree, only=["collector"], tool=lambda name: fake)
    assert result.findings == (_provider_finding("https"),)
    assert not marker.exists()


def test_escaped_reference_in_a_collector_file_is_text(tree: Path, tmp_path: Path) -> None:
    """`$${...}` is the escaped form: the collector reads it as text and expands nothing."""
    fake = fake_tool(tmp_path / "fake-bin", "otelcol-contrib", "exit 0")
    edit(
        tree / OTEL / "exporters" / "sentry.yaml",
        SENTRY_KEY_REFERENCE,
        f"{SENTRY_KEY_REFERENCE} $${{https://config.example.com/text}}",
    )
    [result] = opscheck.run_checks(tree, only=["collector"], tool=lambda name: fake)
    assert result.findings == ()


SIGNAL_DRIVER = """\
import importlib.util
import sys
from pathlib import Path

spec = importlib.util.spec_from_file_location("opscheck_tool", {script!r})
module = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = module
spec.loader.exec_module(module)
module.pinned_tool = lambda name: Path({fake!r})
sys.argv = ["opscheck.py", "--root", {root!r}, "--only", {check!r}]
module._cli()  # the entry of `python opscheck.py`
"""


def _wait_for(condition: Callable[[], bool], seconds: float = 60) -> None:
    deadline = time.monotonic() + seconds
    while not condition():
        assert time.monotonic() < deadline, "condition not reached in time"
        time.sleep(0.05)


def _alive(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    return True


@pytest.mark.parametrize("signum", [signal.SIGTERM, signal.SIGINT])
@pytest.mark.parametrize(
    ("check", "validators"), [("loki", 1), ("collector", opscheck.COLLECTOR_WORKERS)]
)
def test_signal_stops_the_validators_and_removes_the_work_directory(
    tree: Path, tmp_path: Path, signum: int, check: str, validators: int
) -> None:
    """SIGTERM stops a run like SIGINT: no validator keeps running, no work directory stays.

    The fake validators sleep for 30 s, so one that survives the stop is still alive here
    (and ends on its own after that).
    """
    pids = tmp_path / "pids"
    pids.mkdir()
    temp = tmp_path / "temp"
    temp.mkdir()
    fake = fake_tool(
        tmp_path / "fake-bin", "validator", f'echo $$ > "{pids}/$$"\nexec /bin/sleep 30'
    )
    driver = tmp_path / "driver.py"
    driver.write_text(
        SIGNAL_DRIVER.format(
            script=str(Path(opscheck.__file__)), fake=str(fake), root=str(tree), check=check
        ),
        encoding="utf-8",
    )
    proc = subprocess.Popen(
        [sys.executable, str(driver)],
        env={**os.environ, "TMPDIR": str(temp)},
        stdout=subprocess.DEVNULL,
        stderr=subprocess.PIPE,
        text=True,
    )
    try:
        _wait_for(lambda: len(list(pids.iterdir())) >= validators or proc.poll() is not None)
        assert proc.poll() is None, "opscheck ended before its validators started"
        proc.send_signal(signum)
        with contextlib.suppress(subprocess.TimeoutExpired):
            proc.wait(timeout=20)
    finally:
        stopped = proc.poll() is not None
        if not stopped:
            proc.kill()
        stderr = proc.communicate()[1]
    started = sorted(int(path.name) for path in pids.iterdir())
    assert stopped, "opscheck did not stop within 20 s"
    assert [pid for pid in started if _alive(pid)] == []
    assert sorted(temp.glob("opscheck-*")) == []
    # After SIGINT the script ends by SIGINT, as Python does, so a calling shell stops too.
    assert proc.returncode == (-signum if signum == signal.SIGINT else 128 + signum)
    assert stderr == f"opscheck: stopped by {signal.Signals(signum).name}\n"


def test_main_returns_the_exit_status_of_a_stop_signal(
    monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    """The handlers of SIGINT and SIGTERM are installed for the run and restored after it."""
    before = {s: signal.getsignal(s) for s in (signal.SIGINT, signal.SIGTERM)}
    seen = {}

    def stopped(*args: object, **kwargs: object) -> list[object]:
        seen.update({s: signal.getsignal(s) for s in before})
        if seen[signal.SIGTERM] != before[signal.SIGTERM]:  # never kill pytest itself
            signal.raise_signal(signal.SIGTERM)
        return []

    monkeypatch.setattr(opscheck, "run_checks", stopped)
    assert opscheck.main(["--only", "loki"]) == 128 + signal.SIGTERM
    assert capsys.readouterr().err == "opscheck: stopped by SIGTERM\n"
    assert all(seen[s] not in (before[s], signal.SIG_DFL) for s in before)
    assert {s: signal.getsignal(s) for s in before} == before


def test_stopped_check_kills_the_validators_of_other_workers(tmp_path: Path) -> None:
    """A stop in the main thread kills the validators the collector's workers started."""
    ctx = _context(tmp_path)
    pids = tmp_path / "pids"
    pids.mkdir()
    sleeper = fake_tool(tmp_path / "bin", "sleeper", f'echo $$ > "{pids}/$$"\nexec /bin/sleep 30')
    with ThreadPoolExecutor(max_workers=2) as pool:
        running = [pool.submit(opscheck.run_command, ctx, [sleeper]) for _ in range(2)]
        _wait_for(lambda: len(list(pids.iterdir())) == 2)
        opscheck._RUNNING.kill_all()
        results = [future.result(timeout=20) for future in running]
    assert results == [["sleeper: exit status -9"]] * 2
    assert [p for p in pids.iterdir() if _alive(int(p.name))] == []


def test_stopped_run_command_reaps_its_validator(tmp_path: Path) -> None:
    """A stop in the main thread kills the validator and waits for it, so no zombie stays."""
    ctx = _context(tmp_path)
    pids = tmp_path / "pids"
    pids.mkdir()
    sleeper = fake_tool(tmp_path / "bin", "sleeper", f'echo $$ > "{pids}/$$"\nexec /bin/sleep 30')
    previous = signal.signal(signal.SIGUSR1, opscheck._on_stop_signal)

    def stop_when_started() -> None:
        _wait_for(lambda: any(pids.iterdir()))
        signal.pthread_kill(threading.main_thread().ident or 0, signal.SIGUSR1)

    stopper = threading.Thread(target=stop_when_started)
    stopper.start()
    try:
        with pytest.raises(opscheck._Stopped):
            opscheck.run_command(ctx, [sleeper])
    finally:
        stopper.join()
        signal.signal(signal.SIGUSR1, previous)
        opscheck._RUNNING.reset()
    [pid] = [int(path.name) for path in pids.iterdir()]
    with pytest.raises(ChildProcessError):  # already reaped by run_command
        os.waitpid(pid, os.WNOHANG)


def test_stop_while_the_main_thread_starts_a_validator(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A stop during the start raises once the new process is registered, killed and reaped."""
    validators = opscheck._Validators()
    started: list[subprocess.Popen[str]] = []
    real_popen = subprocess.Popen

    def popen_then_stop(*args: Any, **kwargs: Any) -> subprocess.Popen[str]:
        proc = real_popen(*args, **kwargs)
        started.append(proc)
        validators.stop(signal.SIGTERM)  # the handler while the main thread is in `start`
        return proc

    monkeypatch.setattr(opscheck.subprocess, "Popen", popen_then_stop)
    with pytest.raises(opscheck._Stopped):
        validators.start(["/bin/sleep", "30"], env={}, cwd=tmp_path)
    [proc] = started
    assert proc.returncode == -signal.SIGKILL
    assert validators._running == set()


def test_stop_during_the_removal_still_removes_the_work_directory(
    tree: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    created: list[Path] = []
    real_mkdtemp = opscheck.tempfile.mkdtemp
    real_rmtree = shutil.rmtree

    def spy(*args: Any, **kwargs: Any) -> str:
        path = real_mkdtemp(*args, **kwargs)
        created.append(Path(path))
        return str(path)

    def interrupted(path: Path, ignore_errors: bool = False) -> None:
        if not interrupted.stopped:  # type: ignore[attr-defined]
            interrupted.stopped = True  # type: ignore[attr-defined]
            raise opscheck._Stopped(signal.SIGTERM)  # the stop arrives during the removal
        real_rmtree(path, ignore_errors=ignore_errors)

    interrupted.stopped = False  # type: ignore[attr-defined]
    monkeypatch.setattr(opscheck.tempfile, "mkdtemp", spy)
    monkeypatch.setattr(opscheck.shutil, "rmtree", interrupted)
    with pytest.raises(opscheck._Stopped):
        opscheck.run_checks(tree, only=["tool-versions"])
    [work] = created
    assert not work.exists()


def test_a_second_stop_signal_only_kills_again() -> None:
    validators = opscheck._Validators()
    with pytest.raises(opscheck._Stopped):
        validators.stop(signal.SIGTERM)
    validators.stop(signal.SIGINT)  # no second exception while the first one unwinds
    with pytest.raises(opscheck._Stopped) as stopped:
        validators.start(["/bin/true"], env={}, cwd=Path.cwd())
    assert stopped.value.signum == signal.SIGTERM


def test_main_restores_the_default_for_a_handler_set_outside_python(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """`signal.signal` returns None for such a handler; main then restores SIG_DFL, not its own."""
    before = {s: signal.getsignal(s) for s in opscheck.STOP_SIGNALS}
    real_signal = signal.signal

    def outside_python(signum: int, handler: Any) -> Any:
        previous = real_signal(signum, handler)
        return None if handler is opscheck._on_stop_signal else previous

    monkeypatch.setattr(opscheck.signal, "signal", outside_python)
    monkeypatch.setattr(opscheck, "run_checks", lambda *args, **kwargs: [])
    try:
        assert opscheck.main(["--only", "loki"]) == 0
        after = {s: signal.getsignal(s) for s in opscheck.STOP_SIGNALS}
    finally:
        for signum, handler in before.items():
            real_signal(signum, handler)
    assert after == dict.fromkeys(opscheck.STOP_SIGNALS, signal.SIG_DFL)


def _for_mutants(rules: Any) -> list[tuple[str, int, int, str | None]]:
    """(alert, group, rule, new `for`) for each alert: one minute longer, shorter or added."""
    mutants: list[tuple[str, int, int, str | None]] = []
    for group_index, group in enumerate(rules["groups"]):
        for rule_index, rule in enumerate(group["rules"]):
            if "alert" not in rule:
                continue
            if "for" not in rule:
                changes: list[str | None] = ["1m"]
            else:
                seconds = opscheck.parse_duration(rule["for"])
                changes = [f"{seconds + 60:.0f}s", f"{seconds - 60:.0f}s" if seconds > 60 else None]
            mutants += [(rule["alert"], group_index, rule_index, c) for c in changes]
    return mutants


UNCHANGED = "unchanged"


def test_alert_unit_tests_hold_every_for_duration(tmp_path: Path) -> None:
    """A `for` one minute longer, one minute shorter or newly added breaks a unit test.

    Alerts without `for` fire at the first evaluation where their condition holds, so an
    added `for` must break a case too (red team on d73c418).
    """
    promtool = opscheck.pinned_tool("promtool")
    rules = yaml.safe_load((REPO / ALERTS).read_text(encoding="utf-8"))
    mutants = _for_mutants(rules)

    def survives(mutant: tuple[str, int, int, str | None]) -> bool:
        alert, group_index, rule_index, change = mutant
        root = tmp_path / f"{alert}-{change}"
        shutil.copytree(REPO / PROM / "tests", root / "tests")
        spec = copy.deepcopy(rules)
        rule = spec["groups"][group_index]["rules"][rule_index]
        if change is None:
            del rule["for"]
        elif change != UNCHANGED:
            rule["for"] = change
        (root / "rules").mkdir()
        (root / "rules" / "bothy-alerts.yaml").write_text(
            yaml.safe_dump(spec, sort_keys=False), encoding="utf-8"
        )
        tests = sorted((root / "tests").glob("*.test.yaml"))
        # Only the cases of this alert; their names start with it.
        run = f"--run=^{re.escape(alert)} "
        proc = subprocess.run(
            [promtool, "test", "rules", run, *tests], capture_output=True, check=False, timeout=120
        )
        return proc.returncode == 0

    # Control: unmutated, every alert's filtered run passes, so a failing mutant run means a
    # broken case and not a broken setup.
    controls = sorted({(alert, g, r, UNCHANGED) for alert, g, r, _ in mutants})
    with ThreadPoolExecutor(max_workers=4) as pool:
        assert all(pool.map(survives, controls))
        survived = [
            f"{alert}: for {change or 'removed'}"
            for (alert, _, _, change), alive in zip(
                mutants, pool.map(survives, mutants), strict=True
            )
            if alive
        ]
    assert {m[0] for m in mutants} == {name for name, _ in opscheck._alert_rules(REPO / ALERTS)}
    assert survived == []


def test_over_threshold_below_the_value_fails(tree: Path) -> None:
    """``über `<name>` `` needs a threshold strictly greater; one below the value fails too."""
    edit(
        tree / ALERTS,
        'bothy_agent_heartbeat_age_seconds{job="bothy/bothy-server"}) > 300',
        'bothy_agent_heartbeat_age_seconds{job="bothy/bothy-server"}) > 60',
    )
    assert findings(tree, "alert-thresholds") == (
        "BothyAgentOffline: threshold 60 s in bothy-alerts.yaml must be greater than "
        "agent.presence_timeout (90s = 90 s)",
    )


@pytest.mark.parametrize(
    ("old", "new"),
    [("sloth_window: 5m", "sloth_window:  5m"), ("sloth_window: 5m\n", "sloth_window: 5m \n")],
)
def test_sloth_compares_byte_for_byte(tree: Path, old: str, new: str) -> None:
    """A change of white space only is a difference too: the files must be equal byte for byte."""
    edit(tree / PROM / "rules" / "slo-api.yaml", old, new)
    [finding] = findings(tree, "sloth")
    assert finding.startswith(
        "infra/observability/prometheus/rules/slo-api.yaml differs from a fresh "
        "`sloth generate` of infra/observability/slo/api.yaml"
    )


def test_markdown_table_returns_the_first_table_after_the_heading() -> None:
    text = "## T\n\n| a |\n|---|\n| first |\n\nText.\n\n| b |\n|---|\n| second |\n\n## U\n"
    assert opscheck.markdown_table(text, "T") == [{"a": "first"}]


# ---- sli-bounds and sli-labels (VZ-T03) ---------------------------------------------------

GO_BOUNDS = Path("services/server/internal/stats/domain/slibounds.gen.go")
DART_CATALOG = Path("packages/bothy_viz/lib/src/slo/slo_catalog.g.dart")


def test_sli_bounds_reports_a_hand_edited_output(tree: Path) -> None:
    edit(tree / GO_BOUNDS, "Objective: 95,", "Objective: 96,")
    [finding] = findings(tree, "sli-bounds")
    assert finding.startswith(f"{GO_BOUNDS}: differs from a fresh run")


def test_sli_bounds_follows_a_changed_threshold(tree: Path) -> None:
    edit(tree / SLO / "sync.yaml", 'le="0.5"', 'le="1"')
    [finding] = findings(tree, "sli-bounds")
    assert finding.startswith(f"{GO_BOUNDS}: differs from a fresh run")
    assert "+\t\tThreshold: 1000 * time.Millisecond," in finding


def test_sli_bounds_reports_a_missing_catalog(tree: Path) -> None:
    (tree / DART_CATALOG).unlink()
    assert findings(tree, "sli-bounds") == (f"{DART_CATALOG}: missing; run sli_bounds.py --write",)


def test_sli_labels_reports_a_missing_label(tree: Path) -> None:
    edit(tree / SLO / "push.yaml", "      bothy_sli_metric: sli.push_acceptance\n", "")
    assert findings(tree, "sli-labels") == (
        "infra/observability/slo/push.yaml: bothy-push/provider-acceptance: no label"
        " bothy_sli_metric",
        "sli.push_acceptance: no SLO carries it",
        "bothy-push/provider-acceptance: Metrik der eigenen Messung is `sli.push_acceptance`,"
        " the Sloth file says no label",
    )


def test_sli_labels_reports_a_table_that_differs_from_the_label(tree: Path) -> None:
    edit(tree / SLOS_MD, "| `sli.reconnect` |", "| `sli.run_start` |")
    assert findings(tree, "sli-labels") == (
        "bothy-sync/reconnect: Metrik der eigenen Messung is `sli.run_start`, the Sloth file"
        " says `sli.reconnect`",
    )


def test_sli_labels_needs_the_metric_column(tree: Path) -> None:
    edit(tree / SLOS_MD, "| Metrik der eigenen Messung |", "| Metrik |")
    [finding] = findings(tree, "sli-labels")
    assert "has no column" in finding
