# SPDX-License-Identifier: Apache-2.0
"""Tests of the SLI bounds generator (docs/spec/25-visualisierung.md 3.3, VZ-T03, brief M6-11).

Every negative case copies the real inputs into `tmp_path`, changes one thing and expects the
named finding; the repository itself must give no difference.
"""

from __future__ import annotations

import shutil
from pathlib import Path

import pytest
import sli_bounds

REPO = Path(__file__).resolve().parents[3]
SIX = [
    "sli.api_availability",
    "sli.delivery_latency",
    "sli.push_acceptance",
    "sli.reconnect",
    "sli.run_start",
    "sli.turn_duration",
]
API_LABEL = "      bothy_sli_metric: sli.api_availability\n"
"""The label line of the one SLO of api.yaml."""


@pytest.fixture
def tree(tmp_path: Path) -> Path:
    """A repository root with copies of the Sloth files, chapter 03 and both outputs."""
    root = tmp_path / "repo"
    shutil.copytree(REPO / sli_bounds.SLO_DIR, root / sli_bounds.SLO_DIR)
    [model] = sorted((REPO / "docs" / "spec").glob(sli_bounds.DATA_MODEL_GLOB))
    (root / "docs" / "spec").mkdir(parents=True)
    shutil.copy2(model, root / "docs" / "spec" / model.name)
    for out in (sli_bounds.GO_OUT, sli_bounds.DART_OUT):
        (root / out).parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(REPO / out, root / out)
    return root


def edit(path: Path, old: str, new: str) -> None:
    """Replace `old` by `new` once; the test fails if `old` is not there."""
    text = path.read_text(encoding="utf-8")
    assert old in text, f"{old!r} not in {path}"
    path.write_text(text.replace(old, new, 1), encoding="utf-8")


def test_parser_finds_the_six_sli_metrics_of_the_check() -> None:
    assert sli_bounds.read_sli_metrics(REPO) == SIX


def test_parser_fails_without_the_stats_minutes_table(tmp_path: Path) -> None:
    (tmp_path / "docs" / "spec").mkdir(parents=True)
    (tmp_path / "docs" / "spec" / "03-data-model.md").write_text("# 03\n", encoding="utf-8")
    with pytest.raises(sli_bounds.SliBoundsError, match="stats_minutes"):
        sli_bounds.read_sli_metrics(tmp_path)


def test_real_files_give_no_difference() -> None:
    assert sli_bounds.check(REPO) == []


def test_real_slos_carry_the_labels_of_slos_md() -> None:
    by_metric = {s.metric: s.slo for s in sli_bounds.read_slos(REPO)}
    assert by_metric == {
        "sli.api_availability": "bothy-api/requests-availability",
        "sli.delivery_latency": "bothy-sync/delivery-latency",
        "sli.reconnect": "bothy-sync/reconnect",
        "sli.push_acceptance": "bothy-push/provider-acceptance",
        "sli.run_start": "bothy-agents/run-start",
        "sli.turn_duration": "bothy-agents/turn-duration",
    }


def test_missing_label_fails(tree: Path) -> None:
    edit(tree / sli_bounds.SLO_DIR / "api.yaml", API_LABEL, "")
    assert sli_bounds.label_findings(tree) == [
        "infra/observability/slo/api.yaml: bothy-api/requests-availability:"
        " no label bothy_sli_metric",
        "sli.api_availability: no SLO carries it",
    ]


def test_unknown_metric_fails(tree: Path) -> None:
    edit(
        tree / sli_bounds.SLO_DIR / "push.yaml",
        "bothy_sli_metric: sli.push_acceptance",
        "bothy_sli_metric: sli.push_delivery",
    )
    assert sli_bounds.label_findings(tree) == [
        "infra/observability/slo/push.yaml: bothy-push/provider-acceptance: label"
        " bothy_sli_metric is sli.push_delivery, not an sli.* metric of stats_minutes.metric",
        "sli.push_acceptance: no SLO carries it",
    ]


def test_two_slos_with_the_same_metric_fail(tree: Path) -> None:
    edit(
        tree / sli_bounds.SLO_DIR / "sync.yaml",
        "bothy_sli_metric: sli.reconnect",
        "bothy_sli_metric: sli.delivery_latency",
    )
    assert sli_bounds.label_findings(tree) == [
        "sli.delivery_latency: label of 2 SLOs: bothy-sync/delivery-latency, bothy-sync/reconnect",
        "sli.reconnect: no SLO carries it",
    ]


def test_sli_metric_without_slo_fails(tree: Path) -> None:
    (tree / sli_bounds.SLO_DIR / "push.yaml").unlink()
    assert sli_bounds.label_findings(tree) == ["sli.push_acceptance: no SLO carries it"]


def test_duplicate_label_key_fails(tree: Path) -> None:
    edit(
        tree / sli_bounds.SLO_DIR / "api.yaml",
        API_LABEL,
        API_LABEL + "      bothy_sli_metric: sli.reconnect\n",
    )
    duplicate, missing = sli_bounds.label_findings(tree)
    assert "requests-availability: duplicate key bothy_sli_metric" in duplicate
    assert missing == "sli.api_availability: no SLO carries it"


def test_generator_refuses_to_write_with_label_findings(tree: Path) -> None:
    edit(tree / sli_bounds.SLO_DIR / "api.yaml", API_LABEL, "")
    with pytest.raises(sli_bounds.SliBoundsError) as err:
        sli_bounds.render(tree)
    assert "no label bothy_sli_metric" in str(err.value)


@pytest.mark.parametrize(
    ("query", "seconds"),
    [
        ('sum(rate(x_bucket{le="0.5"}[5m]))', 0.5),
        ("sum(rate(x_bucket{le='600'}[5m]))", 600.0),
        ("sum(rate(x_bucket{le=`7.5`}[5m]))", 7.5),
        ("sum(rate(x_count[5m]))", None),
    ],
)
def test_threshold_parses_from_le(query: str, seconds: float | None) -> None:
    assert sli_bounds.threshold_seconds(query, "") == seconds


def test_threshold_from_total_query_when_error_query_has_none() -> None:
    assert sli_bounds.threshold_seconds("sum(rate(e[5m]))", 'sum(rate(x{le="2.5"}[5m]))') == 2.5


def test_period_default_and_override(tree: Path) -> None:
    assert {s.period_days for s in sli_bounds.read_slos(tree)} == {
        sli_bounds.SLOTH_DEFAULT_PERIOD_DAYS
    }
    edit(
        tree / sli_bounds.SLO_DIR / "api.yaml",
        'service: "bothy-api"\n',
        'service: "bothy-api"\nslo_period: 28d\n',
    )
    periods = {s.slo: s.period_days for s in sli_bounds.read_slos(tree)}
    assert periods["bothy-api/requests-availability"] == 28
    assert periods["bothy-sync/reconnect"] == 30


def test_bad_period_fails(tree: Path) -> None:
    edit(
        tree / sli_bounds.SLO_DIR / "api.yaml",
        'service: "bothy-api"\n',
        'service: "bothy-api"\nslo_period: 4w\n',
    )
    with pytest.raises(sli_bounds.SliBoundsError, match="slo_period"):
        sli_bounds.read_slos(tree)


def test_outputs_are_deterministic_and_sorted(tree: Path) -> None:
    first = sli_bounds.render(tree)
    second = sli_bounds.render(tree)
    assert first == second
    go = first[sli_bounds.GO_OUT]
    dart = first[sli_bounds.DART_OUT]
    assert [go.index(f'"{m}"') for m in SIX] == sorted(go.index(f'"{m}"') for m in SIX)
    assert [dart.index(f"'{m}'") for m in SIX] == sorted(dart.index(f"'{m}'") for m in SIX)
    assert "DO NOT EDIT" in go.splitlines()[0]
    assert "DO NOT EDIT" in dart.splitlines()[0]
    assert "tools/observability/sli_bounds.py" in go.splitlines()[0]
    assert "Threshold: 500 * time.Millisecond" in go
    assert "Threshold: 0," in go
    assert "Objective: 99.9," in go
    assert "le=" not in dart
    assert "Threshold" not in dart


def test_check_reports_a_hand_edit_with_a_diff(tree: Path) -> None:
    edit(tree / sli_bounds.GO_OUT, "Objective: 99.9,", "Objective: 99.0,")
    [finding] = sli_bounds.check(tree)
    assert finding.startswith(f"{sli_bounds.GO_OUT}: differs from a fresh run")
    assert "-\t\tObjective: 99.0," in finding
    assert "+\t\tObjective: 99.9," in finding


def test_check_reports_a_missing_output(tree: Path) -> None:
    (tree / sli_bounds.DART_OUT).unlink()
    assert sli_bounds.check(tree) == [f"{sli_bounds.DART_OUT}: missing; run sli_bounds.py --write"]


def test_check_follows_a_changed_objective(tree: Path) -> None:
    edit(tree / sli_bounds.SLO_DIR / "api.yaml", "objective: 99.9", "objective: 99.95")
    [finding] = sli_bounds.check(tree)
    assert finding.startswith(f"{sli_bounds.GO_OUT}: differs")


def test_write_then_check_is_clean(tree: Path) -> None:
    edit(tree / sli_bounds.SLO_DIR / "api.yaml", "objective: 99.9", "objective: 99.95")
    assert sli_bounds.main(["--root", str(tree), "--write"]) == 0
    assert sli_bounds.main(["--root", str(tree), "--check"]) == 0
    assert "Objective: 99.95," in (tree / sli_bounds.GO_OUT).read_text(encoding="utf-8")


def test_cli_check_prints_findings(tree: Path, capsys: pytest.CaptureFixture[str]) -> None:
    edit(tree / sli_bounds.SLO_DIR / "api.yaml", API_LABEL, "")
    assert sli_bounds.main(["--root", str(tree), "--check"]) == 1
    assert "no label bothy_sli_metric" in capsys.readouterr().err
    assert sli_bounds.main(["--root", str(tree), "--write"]) == 1


def test_cli_needs_a_mode() -> None:
    with pytest.raises(SystemExit):
        sli_bounds.main([])
