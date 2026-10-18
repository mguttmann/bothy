# SPDX-License-Identifier: Apache-2.0
"""Tests for tools/signalcheck/signalcheck.py (gate `make signal-check`, M1-68).

The fixture repository under fixtures/repo passes every rule; each failing case copies it to
tmp_path, applies one mutation and expects a finding of its rule. Rule 6 runs the pinned
promtool (fetched on first use through tools/bin/fetch_tools.py).
"""

from __future__ import annotations

import importlib.util
import shutil
import sys
from pathlib import Path
from types import ModuleType

import pytest


def _load() -> ModuleType:
    """Load the tool module from its file.

    pytest's importlib mode registers tools/signalcheck as the package `signalcheck`, which
    hides the tool module of the same name from a plain import.
    """
    path = Path(__file__).resolve().parents[1] / "signalcheck.py"
    spec = importlib.util.spec_from_file_location("signalcheck_tool", path)
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module  # dataclasses look their module up while it executes
    spec.loader.exec_module(module)
    return module


sc = _load()

REPO = Path(__file__).resolve().parents[3]
FIXTURE = Path(__file__).resolve().parent / "fixtures" / "repo"
SIGNALS = "docs/observability/signals.md"
DASHBOARD = "infra/observability/grafana/dashboards/overview.json"
SLO = "infra/observability/slo/fixture.yaml"


@pytest.fixture(scope="module")
def promtool() -> Path:
    """The pinned promtool."""
    return Path(sc.promtool_path())


@pytest.fixture
def repo(tmp_path: Path) -> Path:
    """A copy of the fixture repository."""
    root = tmp_path / "repo"
    shutil.copytree(FIXTURE, root)
    return root


def mutate(root: Path, rel: str, old: str, new: str) -> None:
    """Replace old by new in the file rel of root; old must occur."""
    path = root / rel
    text = path.read_text(encoding="utf-8")
    assert old in text, f"{old!r} not in {rel}"
    path.write_text(text.replace(old, new), encoding="utf-8")


def rules_of(findings: list[object]) -> set[int]:
    """The rule numbers of findings."""
    return {f.rule for f in findings}  # type: ignore[attr-defined]


def test_real_repository_passes(promtool: Path) -> None:
    findings, summary = sc.run(REPO, promtool)
    assert [str(f) for f in findings] == []
    assert summary.startswith("signal-check: ok")


def test_fixture_passes_recording_rules_window_variables_by_lists_and_buckets(
    repo: Path, promtool: Path
) -> None:
    """Recording rules, {{.window}}, Grafana variables, by (...) lists and histogram_quantile over
    _bucket pass; the Loki panel and the templating query are not PromQL targets."""
    findings, _ = sc.run(repo, promtool)
    assert [str(f) for f in findings] == []


def test_dashboard_syntax_error_fails(repo: Path, promtool: Path) -> None:
    mutate(
        repo,
        DASHBOARD,
        "max(bothy_event_fanout_lag_seconds) * 0",
        "max(bothy_event_fanout_lag_seconds * 0",
    )
    findings, _ = sc.run(repo, promtool)
    assert any(f.rule == 6 and "Processes" in f.location for f in findings), findings


def test_unknown_metric_in_dashboard_fails(repo: Path) -> None:
    mutate(repo, DASHBOARD, "bothy_gateway_close_total{", "bothy_gateway_closes_total{")
    findings, _ = sc.run(repo, None)
    assert any(f.rule == 2 and "bothy_gateway_closes_total" in f.message for f in findings), (
        findings
    )


def test_unknown_label_on_catalogue_metric_fails(repo: Path) -> None:
    mutate(
        repo,
        DASHBOARD,
        "bothy_gateway_close_total{instance=~",
        'bothy_gateway_close_total{bothy_reason=\\"x\\",instance=~',
    )
    findings, _ = sc.run(repo, None)
    assert any(f.rule == 3 and "bothy_reason" in f.message for f in findings), findings


def test_sloth_le_not_a_bound_fails(repo: Path) -> None:
    mutate(repo, SLO, 'le="0.5"', 'le="0.3"')
    findings, _ = sc.run(repo, None)
    assert any(f.rule == 4 and f.file == SLO for f in findings), findings


def test_le_bounds_compare_numerically(repo: Path) -> None:
    mutate(repo, SLO, 'le="0.5"', 'le="0.50"')
    mutate(repo, DASHBOARD, 'le=\\"0.5\\"', 'le=\\"+Inf\\"')
    findings, _ = sc.run(repo, None)
    assert [str(f) for f in findings] == []


def test_prometheus_column_breaking_translation_fails(repo: Path) -> None:
    mutate(
        repo,
        SIGNALS,
        "| api | `bothy_event_fanout_lag_seconds` |",
        "| api | `bothy_event_fanout_lag` |",
    )
    findings, _ = sc.run(repo, None)
    assert any(f.rule == 1 and "bothy_event_fanout_lag_seconds" in f.message for f in findings), (
        findings
    )


def test_histogram_without_bounds_row_fails(repo: Path) -> None:
    mutate(repo, SIGNALS, "| durations (`http.server.request.duration`) |", "| durations |")
    findings, _ = sc.run(repo, None)
    assert any(f.rule == 1 and "0 rows in 4.7" in f.message for f in findings), findings


def test_bounds_row_naming_no_histogram_fails(repo: Path) -> None:
    mutate(
        repo,
        SIGNALS,
        "| durations (`http.server.request.duration`) |",
        "| durations (`http.server.request.duration`, `bothy.gateway.close`) |",
    )
    findings, _ = sc.run(repo, None)
    assert any(f.rule == 1 and "no histogram" in f.message for f in findings), findings


@pytest.mark.parametrize(
    "key", ["bothy.thread.id", "bothy.principal.pseudo_id", "user.name", "client.address"]
)
def test_person_attribute_in_catalogue_fails(repo: Path, key: str) -> None:
    mutate(repo, SIGNALS, "| `bothy.close.code` |", f"| `bothy.close.code`, `{key}` |")
    findings, _ = sc.run(repo, None)
    assert any(f.rule == 1 and key in f.message for f in findings), findings


def test_undefined_recording_rule_fails(repo: Path) -> None:
    mutate(repo, DASHBOARD, "max(slo:sli_error:ratio_rate5m)", "max(slo:sli_error:ratio_rate1h)")
    findings, _ = sc.run(repo, None)
    assert any(f.rule == 2 and "slo:sli_error:ratio_rate1h" in f.message for f in findings), (
        findings
    )


def test_main_exit_codes(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    assert sc.main(["--root", str(repo)]) == 0
    assert capsys.readouterr().out.startswith("signal-check: ok")
    mutate(repo, SLO, 'le="0.5"', 'le="0.3"')
    assert sc.main(["--root", str(repo)]) == 1
    out = capsys.readouterr().out.strip().splitlines()
    assert len(out) == 1
    assert out[0].startswith(f"{SLO}: slo 'latency' error_query: rule 4:")


def test_selectors_lexing() -> None:
    expr = (
        'sum without (instance) (rate(a_total{x="a{b}",y=~"$env"}[$__rate_interval] offset 5m))'
        " / on (job) group_left (z) b_count > bool 0.5 and topk(3, c) unless nan"
    )
    got = [(s.name, s.matchers) for s in sc.selectors(expr)]
    assert got == [("a_total", [("x", "=", "a{b}"), ("y", "=~", "5m")]), ("b_count", []), ("c", [])]


def test_unknown_promtool_fails_main(
    monkeypatch: pytest.MonkeyPatch, repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    def broken() -> Path:
        raise sc.fetch_tools.LockError("promtool is not pinned")

    monkeypatch.setattr(sc, "promtool_path", broken)
    assert sc.main(["--root", str(repo)]) == 1
    assert "promtool unavailable" in capsys.readouterr().err


PANEL_QUERIES = "plugins/grafana-bothy-live/queries.yaml"


def test_panel_queries_are_read(repo: Path) -> None:
    """Every expr of the panel's queries.yaml is an expression of the check (M7-25)."""
    exprs = sc.panel_query_expressions(repo)
    assert [e.location for e in exprs] == [
        "view 'delivery' query 'fanout_lag'",
        "view 'delivery' query 'request_p95'",
        "view 'executive' query 'error_ratio'",
    ]
    assert all(e.file == PANEL_QUERIES for e in exprs)


def test_unknown_metric_in_panel_queries_fails(repo: Path) -> None:
    mutate(
        repo, PANEL_QUERIES, "max(bothy_event_fanout_lag_seconds{", "max(bothy_fanout_lag_seconds{"
    )
    findings, _ = sc.run(repo, None)
    assert any(
        f.rule == 2 and f.file == PANEL_QUERIES and "bothy_fanout_lag_seconds" in f.message
        for f in findings
    ), findings


def test_id_label_in_panel_queries_fails(repo: Path) -> None:
    """A label outside the metric's attributes (here an ID) is refused, as in dashboards."""
    mutate(
        repo,
        PANEL_QUERIES,
        'deployment_environment_name=~"$env"',
        'deployment_environment_name=~"$env",bothy_run_id!=""',
    )
    findings, _ = sc.run(repo, None)
    assert any(f.rule == 3 and "bothy_run_id" in f.message for f in findings), findings


def test_panel_query_syntax_error_fails(repo: Path, promtool: Path) -> None:
    mutate(repo, PANEL_QUERIES, "max(slo:sli_error:ratio_rate5m)", "max(slo:sli_error:ratio_rate5m")
    findings, _ = sc.run(repo, promtool)
    assert any(f.rule == 6 and f.file == PANEL_QUERIES for f in findings), findings


def test_without_panel_queries_nothing_is_read(repo: Path) -> None:
    (repo / PANEL_QUERIES).unlink()
    assert sc.panel_query_expressions(repo) == []
