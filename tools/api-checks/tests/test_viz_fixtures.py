# SPDX-License-Identifier: Apache-2.0
"""Tests of the check `viz-fixtures` (docs/spec/25-visualisierung.md 7.2, VZ-T11, brief M6-11).

Every negative case copies the real fixtures, their README and openapi.yaml into `tmp_path`,
changes one thing and expects the named finding; the repository itself must stay clean.
"""

from __future__ import annotations

import json
import shutil
from pathlib import Path
from typing import Any

import api_checks
import pytest
import viz_fixtures

REPO = Path(__file__).resolve().parents[3]


@pytest.fixture
def tree(tmp_path: Path) -> Path:
    """A repository root with copies of the fixtures and the OpenAPI document."""
    root = tmp_path / "repo"
    shutil.copytree(REPO / viz_fixtures.FIXTURES_DIR, root / viz_fixtures.FIXTURES_DIR)
    (root / viz_fixtures.OPENAPI_PATH).parent.mkdir(parents=True)
    shutil.copy2(REPO / viz_fixtures.OPENAPI_PATH, root / viz_fixtures.OPENAPI_PATH)
    return root


def fixture(root: Path, name: str) -> Path:
    return root / viz_fixtures.FIXTURES_DIR / name


def rewrite(path: Path, change: Any) -> None:
    """Load a JSON fixture, apply `change` to it and write it back."""
    data = json.loads(path.read_text(encoding="utf-8"))
    change(data)
    path.write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")


def test_real_fixtures_are_valid() -> None:
    assert viz_fixtures.check(REPO) == []


def test_real_fixtures_cover_every_listed_schema() -> None:
    listed = viz_fixtures.listed_fixtures(REPO)
    assert {schema for schema, _ in listed.values()} == {
        "StatsLiveFrame",
        "StatsTimeline",
        "StatsSloReport",
        "StatsBackendReport",
        "StatsSeries",
        "CostSeries",
    }


def test_load_frame_has_the_default_of_stats_live_max_runs() -> None:
    frame = json.loads(fixture(REPO, "StatsLiveFrame.load.json").read_text(encoding="utf-8"))
    assert len(frame["runs"]) == 200  # stats.live_max_runs, docs/spec/25 section 15
    truncated = json.loads(
        fixture(REPO, "StatsLiveFrame.truncated.json").read_text(encoding="utf-8")
    )
    assert truncated["runs_truncated"] is True


def test_valid_fixture_passes(tree: Path) -> None:
    assert viz_fixtures.check(tree) == []


def test_missing_required_member_fails(tree: Path) -> None:
    rewrite(fixture(tree, "StatsLiveFrame.normal.json"), lambda d: d.pop("runs_truncated"))
    [finding] = viz_fixtures.check(tree)
    assert finding.startswith("viz-fixtures: valid: StatsLiveFrame.normal.json: ")
    assert "'runs_truncated' is a required property" in finding


def test_member_of_the_wrong_type_fails(tree: Path) -> None:
    def change(data: dict[str, Any]) -> None:
        data["runs"][0]["approval_pending"] = "no"

    rewrite(fixture(tree, "StatsLiveFrame.normal.json"), change)
    [finding] = viz_fixtures.check(tree)
    assert finding.startswith("viz-fixtures: valid: StatsLiveFrame.normal.json: /runs/0/")
    assert "is not of type 'boolean'" in finding


def test_unknown_enum_value_fails(tree: Path) -> None:
    def change(data: dict[str, Any]) -> None:
        data["items"][0]["state"] = "blue"

    rewrite(fixture(tree, "StatsSloReport.normal.json"), change)
    [finding] = viz_fixtures.check(tree)
    assert "StatsSloReport.normal.json: /items/0/state" in finding


def test_invalid_json_fails(tree: Path) -> None:
    fixture(tree, "CostSeries.empty.json").write_text("{", encoding="utf-8")
    [finding] = viz_fixtures.check(tree)
    assert finding.startswith("viz-fixtures: valid: CostSeries.empty.json: not JSON")


def test_unknown_schema_name_fails(tree: Path) -> None:
    shutil.copy2(fixture(tree, "CostSeries.empty.json"), fixture(tree, "CostSerie.empty.json"))
    assert viz_fixtures.check(tree) == [
        "viz-fixtures: schema: CostSerie.empty.json: no schema CostSerie in docs/api/openapi.yaml",
        "viz-fixtures: readme: CostSerie.empty.json: not listed in"
        " packages/bothy_viz/fixtures/README.md",
    ]


def test_listed_schema_without_fixture_fails(tree: Path) -> None:
    for name in ("CostSeries.normal.json", "CostSeries.empty.json"):
        fixture(tree, name).unlink()
    found = viz_fixtures.check(tree)
    assert "viz-fixtures: coverage: CostSeries: no fixture file" in found
    assert (
        "viz-fixtures: readme: CostSeries.normal.json: listed in"
        " packages/bothy_viz/fixtures/README.md but missing"
    ) in found


def test_readme_without_the_table_fails(tree: Path) -> None:
    (tree / viz_fixtures.README_PATH).write_text("# Fixtures\n", encoding="utf-8")
    [finding] = viz_fixtures.check(tree)
    assert "no table" in finding


def test_schema_change_makes_the_real_fixture_fail(tree: Path) -> None:
    """VZ-T11: a schema change without an adjusted fixture fails the gate."""
    path = tree / viz_fixtures.OPENAPI_PATH
    text = path.read_text(encoding="utf-8")
    old = "    StatsBackendReport:\n      type: object\n      required: [items]\n"
    assert old in text
    path.write_text(text.replace(old, old.replace("[items]", "[items, generated_at]")), "utf-8")
    found = viz_fixtures.check(tree)
    assert found
    assert all("'generated_at' is a required property" in f for f in found)
    assert {f.split(": ")[2] for f in found} == {
        "StatsBackendReport.empty.json",
        "StatsBackendReport.no_cost_permission.json",
        "StatsBackendReport.normal.json",
        "StatsBackendReport.rate_limits.json",
    }


def test_rule_runs_through_the_cli(tree: Path, capsys: pytest.CaptureFixture[str]) -> None:
    assert api_checks.main(["check", "--root", str(REPO), "--rule", "viz-fixtures"]) == 0
    rewrite(fixture(tree, "StatsTimeline.empty.json"), lambda d: d.pop("from"))
    assert api_checks.main(["check", "--root", str(tree), "--rule", "viz-fixtures"]) == 1
    assert "viz-fixtures: valid: StatsTimeline.empty.json" in capsys.readouterr().err
