# SPDX-License-Identifier: Apache-2.0
"""Tests for the brief dependency check (tools/docs/brief_deps.py)."""

from __future__ import annotations

from pathlib import Path

import brief_deps
import pytest

REPO = Path(__file__).resolve().parents[3]

INDEX_ROWS = {
    "M1-01": (1, "none (M0 closed)"),
    "M1-02": (2, "M1-01"),
    "M1-03": (3, "M1-02"),
    "M1-04": (1, "none (M0 closed)"),
}

BRIEF = "# T\n\nStatus: ready | Depends on: {deps} | Etappe: M1\n\n{body}\n"


def _readme(rows: dict[str, tuple[int, str]], waves: str, edges: str) -> str:
    index = "\n".join(f"| {w} | {t} | Title | `{t}-x.md` | {d} |" for t, (w, d) in rows.items())
    return (
        "# Build briefs\n\n## Waves\n\nText.\n\n"
        f"{waves}\n"
        "## Index\n\n| Wave | ID | Title | Brief file | Depends on |\n|---|---|---|---|---|\n"
        f"{index}\n\n"
        "## Make targets created by tickets\n\n| Target | Created by |\n|---|---|\n"
        "| `gen-check` | M1-02 |\n| `other-check` | M1-04 |\n\n"
        "## Dependency graph\n\n```mermaid\nflowchart TB\n"
        f"{edges}```\n"
    )


WAVES = (
    "| Wave | Tickets that may run in parallel |\n|---|---|\n"
    "| 1 | M1-01, M1-04 |\n| 2 | M1-02 |\n| 3 | M1-03 |\n"
)
EDGES = "  M1_01 --> M1_02\n  M1_02 --> M1_03\n"


def _repo(
    tmp_path: Path,
    rows: dict[str, tuple[int, str]] | None = None,
    waves: str = WAVES,
    edges: str = EDGES,
    bodies: dict[str, str] | None = None,
    plan_deps: dict[str, str] | None = None,
) -> Path:
    rows = INDEX_ROWS if rows is None else rows
    bodies = bodies or {}
    plan_deps = plan_deps or {}
    (tmp_path / "docs/briefs").mkdir(parents=True)
    (tmp_path / "docs/briefs/README.md").write_text(_readme(rows, waves, edges), encoding="utf-8")
    plan = ["| ID | Titel | Abhängig von |", "|---|---|---|"]
    for tid, (_, deps) in rows.items():
        plan_cell = plan_deps.get(tid, deps.replace("none (M0 closed)", "keine"))
        plan.append(f"| {tid} | Titel | {plan_cell} |")
        (tmp_path / f"docs/briefs/{tid}-x.md").write_text(
            BRIEF.format(deps=deps, body=bodies.get(tid, "Run `make check`.")), encoding="utf-8"
        )
    (tmp_path / "docs/PLAN.md").write_text("\n".join(plan) + "\n", encoding="utf-8")
    (tmp_path / "Makefile").write_text(
        "X := 1\ncheck: ## all\n\t@true\ndocs:\n\t@true\n", encoding="utf-8"
    )
    return tmp_path


def test_repository_is_consistent() -> None:
    assert brief_deps.check(REPO) == []


def test_clean_fixture_passes(tmp_path: Path) -> None:
    assert brief_deps.check(_repo(tmp_path)) == []


def test_target_from_dependency_passes(tmp_path: Path) -> None:
    root = _repo(tmp_path, bodies={"M1-03": "```sh\nmake gen-check && make docs\n```"})
    assert brief_deps.check(root) == []


def test_target_outside_closure_fails(tmp_path: Path) -> None:
    root = _repo(tmp_path, bodies={"M1-03": "Run `make other-check`."})
    assert brief_deps.check(root) == [
        "M1-03-x.md: make other-check: created by M1-04, not a dependency of M1-03"
    ]


def test_unknown_target_fails(tmp_path: Path) -> None:
    root = _repo(tmp_path, bodies={"M1-01": "Run `make nowhere`."})
    assert brief_deps.check(root) == ["M1-01-x.md: make nowhere: no Makefile target and no creator"]


def test_prose_is_ignored(tmp_path: Path) -> None:
    prose = "Write the test, then make it green; make other-check."
    root = _repo(tmp_path, bodies={"M1-01": prose})
    assert brief_deps.check(root) == []


def test_variable_assignment_is_no_target() -> None:
    assert brief_deps.makefile_targets("X := 1\nY ?= 2\na-b: c\n") == {"a-b"}


def test_implied_dependency_fails(tmp_path: Path) -> None:
    rows = dict(INDEX_ROWS, **{"M1-03": (3, "M1-01, M1-02")})
    edges = "  M1_01 --> M1_02\n  M1_01 --> M1_03\n  M1_02 --> M1_03\n"
    problems = brief_deps.check(_repo(tmp_path, rows=rows, edges=edges))
    assert problems == ["docs/briefs/README.md: M1-03: dependency M1-01 is implied by another one"]


def test_wrong_wave_fails(tmp_path: Path) -> None:
    rows = dict(INDEX_ROWS, **{"M1-03": (2, "M1-02")})
    problems = brief_deps.check(_repo(tmp_path, rows=rows))
    assert "docs/briefs/README.md: M1-03: wave 2, expected 3" in problems


def test_waves_table_mismatch_fails(tmp_path: Path) -> None:
    waves = WAVES.replace("| 1 | M1-01, M1-04 |", "| 1 | M1-01 |")
    problems = brief_deps.check(_repo(tmp_path, waves=waves))
    assert problems == ["docs/briefs/README.md: table 'Waves' differs from the index (run --write)"]


def test_graph_mismatch_fails(tmp_path: Path) -> None:
    problems = brief_deps.check(_repo(tmp_path, edges="  M1_01 --> M1_02\n"))
    assert problems == ["docs/briefs/README.md: graph edges differ from the index (run --write)"]


def test_plan_mismatch_fails(tmp_path: Path) -> None:
    problems = brief_deps.check(_repo(tmp_path, plan_deps={"M1-03": "M1-01"}))
    assert problems == ["docs/PLAN.md: M1-03: depends on ['M1-01'], index says ['M1-02']"]


def test_brief_header_mismatch_fails(tmp_path: Path) -> None:
    root = _repo(tmp_path)
    brief = root / "docs/briefs/M1-03-x.md"
    brief.write_text(brief.read_text(encoding="utf-8").replace("M1-02", "M1-01"), encoding="utf-8")
    assert brief_deps.check(root) == ["M1-03-x.md: header 'Depends on' differs from the index"]


def test_missing_brief_and_plan_row_fail(tmp_path: Path) -> None:
    root = _repo(tmp_path)
    (root / "docs/briefs/M1-04-x.md").unlink()
    (root / "docs/PLAN.md").write_text("| M1-01 | T | keine |\n", encoding="utf-8")
    problems = brief_deps.check(root)
    assert "docs/briefs/README.md: M1-04: brief file M1-04-x.md missing" in problems
    assert "docs/PLAN.md: no ticket row for M1-02" in problems


def test_unknown_dependency_and_creator_fail(tmp_path: Path) -> None:
    rows = dict(INDEX_ROWS, **{"M1-03": (3, "M1-09")})
    problems = brief_deps.check(_repo(tmp_path, rows=rows))
    assert problems == ["docs/briefs/README.md: M1-03: unknown dependency M1-09"]
    root = tmp_path / "second"
    root.mkdir()
    _repo(root)
    readme = root / "docs/briefs/README.md"
    readme.write_text(
        readme.read_text(encoding="utf-8").replace("| M1-04 |\n\n", "| M1-07 |\n\n"),
        encoding="utf-8",
    )
    assert brief_deps.check(root) == [
        "docs/briefs/README.md: target other-check: unknown ticket M1-07"
    ]


def test_cycle_fails(tmp_path: Path) -> None:
    rows = dict(INDEX_ROWS, **{"M1-01": (1, "M1-03")})
    problems = brief_deps.check(_repo(tmp_path, rows=rows))
    assert len(problems) == 1
    assert "dependency cycle" in problems[0]


def test_write_regenerates_waves_and_graph(tmp_path: Path) -> None:
    rows = dict(INDEX_ROWS, **{"M1-04": (7, "M1-03")})
    root = _repo(tmp_path, rows=rows, waves=WAVES, edges="  M1_01 --> M1_02\n")
    assert brief_deps.main(["--root", str(root), "--write"]) == 0
    text = (root / "docs/briefs/README.md").read_text(encoding="utf-8")
    assert "| 4 | M1-04 |" in text
    assert "| 4 | M1-04 |\n" in text.split("## Index")[0]
    assert "  M1_03 --> M1_04\n```" in text
    assert brief_deps.check(root) == []


def test_main_reports_problems(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    root = _repo(tmp_path, bodies={"M1-01": "Run `make nowhere`."})
    assert brief_deps.main(["--root", str(root)]) == 1
    assert "brief-deps: M1-01-x.md: make nowhere" in capsys.readouterr().err


def test_missing_section_raises() -> None:
    with pytest.raises(ValueError, match="Index"):
        brief_deps.parse_index("# nothing\n")
