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

BRIEF = "# T\n\nStatus: ready | Depends on: {deps} | Milestone: M1\n\n{body}\n"


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
    plan = ["| ID | Title | Depends on |", "|---|---|---|"]
    for tid, (_, deps) in rows.items():
        plan_cell = plan_deps.get(tid, deps.replace("none (M0 closed)", "none"))
        plan.append(f"| {tid} | Title | {plan_cell} |")
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


def test_start_check_outside_closure_fails(tmp_path: Path) -> None:
    body = "Start check: M1-04 is merged, else `Verdict: BLOCKED dependency M1-04 missing`."
    root = _repo(tmp_path, bodies={"M1-03": body})
    assert brief_deps.check(root) == [
        "M1-03-x.md: start check needs M1-04, which is not a dependency of M1-03"
    ]


def test_start_check_list_before_verdict_fails(tmp_path: Path) -> None:
    body = (
        "Confirm that the base branch has:\nM1-02 (gen) and M1-04 (other).\n"
        "A missing one ends with `Verdict: BLOCKED dependency <ticket> missing`."
    )
    root = _repo(tmp_path, bodies={"M1-03": body})
    assert brief_deps.check(root) == [
        "M1-03-x.md: start check needs M1-04, which is not a dependency of M1-03"
    ]


def test_start_check_not_merged_wording_fails(tmp_path: Path) -> None:
    body = "If M1-04 is absent, stop with `Verdict: BLOCKED M1-04 not merged`."
    root = _repo(tmp_path, bodies={"M1-03": body})
    assert brief_deps.check(root) == [
        "M1-03-x.md: start check needs M1-04, which is not a dependency of M1-03"
    ]


def test_optional_ticket_in_start_check_is_ignored(tmp_path: Path) -> None:
    body = (
        "If the base lacks it, stop with `Verdict: BLOCKED M1-02 not merged`. "
        "Used only if merged: M1-04 (other check). M1-04 is read where merged."
    )
    assert brief_deps.check(_repo(tmp_path, bodies={"M1-03": body})) == []


def test_start_check_in_closure_passes(tmp_path: Path) -> None:
    body = "M1-01 and M1-02 are merged, else `Verdict: BLOCKED dependency <ticket> missing`."
    assert brief_deps.check(_repo(tmp_path, bodies={"M1-03": body})) == []


def test_mention_outside_start_check_is_ignored(tmp_path: Path) -> None:
    body = (
        "M1-04 builds the other check later; on a spec gap stop with `Verdict: BLOCKED <reason>`."
        "\n\n- M1-04 owns the other check.\n"
        "- M1-01 is merged, else `Verdict: BLOCKED dependency M1-01 missing`.\n"
    )
    assert brief_deps.check(_repo(tmp_path, bodies={"M1-03": body})) == []


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
    (root / "docs/PLAN.md").write_text("| M1-01 | T | none |\n", encoding="utf-8")
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


@pytest.mark.parametrize(
    ("sentence", "named"),
    [
        ("M1-01 is not in the closure: do not rely on it.", ["M1-01"]),
        ("Used only if merged: M1-01 (generator).", ["M1-01"]),
        ("If M1-02 is on the base, add the case.", ["M1-02"]),
        ("M1-02 is not in this ticket's dependency closure.", ["M1-02"]),
        ("Add the test once M1-02 is on the base.", ["M1-02"]),
        ("Use the generator when M1-02 has been merged.", ["M1-02"]),
        ("Use the generator once M1-02 has already been merged.", ["M1-02"]),
        ("Read the registry of M1-02 (if merged).", ["M1-02"]),
        ("Use M1-02 only if merged.", ["M1-02"]),
        ("Not in the closure but used if merged: M1-04 (other), M1-02 (generator).", ["M1-02"]),
        (
            "Not in the closure but used if merged: M1-04 (other, used rarely), M1-02 (gen).",
            ["M1-02"],
        ),
        ("Not in the closure: M1-04 (a), M1-05 (b), and M1-02 (c).", ["M1-02"]),
        ("M1-01, M1-02 and M1-04 are not in the closure.", ["M1-01", "M1-02"]),
        ("Run it when M1-01 and M1-02 are on the base.", ["M1-01", "M1-02"]),
        ("Change it through SetTriggerState (once M1-02 is merged).", ["M1-02"]),
        ("Use the registry (if M1-02 is on the base).", ["M1-02"]),
        ("Add the case (M1-02 is not in this closure).", ["M1-02"]),
        ("Read M1-04 or M1-02 (if merged).", ["M1-02"]),
        ("Depends on M1-04, M1-02 only if merged.", ["M1-02"]),
    ],
)
def test_dependency_called_optional_fails(tmp_path: Path, sentence: str, named: list[str]) -> None:
    """A brief must not treat a ticket of its own closure as missing or optional (D-W-113)."""
    root = _repo(tmp_path, bodies={"M1-03": sentence})
    assert brief_deps.check(root) == [
        f"M1-03-x.md: {t} is a dependency of M1-03, but the brief calls it optional"
        " or outside its closure"
        for t in named
    ]


@pytest.mark.parametrize(
    "sentence",
    [
        "M1-04 is not in the closure: do not rely on it.",
        "Used only if merged: M1-04 (other check).",
        "M1-02 is in the closure; M1-04 is not in the closure.",
        "M1-02 reports a plan gap.",
        "M1-02 provides the generator, but M1-04 is not in the closure.",
        "M1-02 is in the closure through M1-01, and M1-04 is not in the closure.",
        "Not in the closure but used if merged: M1-04 (other), while M1-02 is required.",
        "M1-02 adds the lint and M1-04 is not in the closure.",
        "M1-04 (which builds on M1-02) is not in the closure.",
        "Not in the closure: M1-04, and M1-02 is required.",
        "Not in the closure but used if merged: M1-04 (other), and M1-02 is required.",
        "Not in the closure: M1-04, or else M1-02 helps.",
        "Read M1-02 and M1-04 (if merged).",
        "Depends on M1-02, M1-04 only if merged.",
    ],
)
def test_optional_ticket_outside_closure_passes(tmp_path: Path, sentence: str) -> None:
    assert brief_deps.check(_repo(tmp_path, bodies={"M1-03": sentence})) == []


@pytest.mark.parametrize(
    "body",
    [
        "Not in the closure:\n\nM1-02 provides the generator.",
        "- Not in the closure, used if merged:\n- M1-02 provides the generator.",
    ],
)
def test_optional_list_stops_at_a_block(tmp_path: Path, body: str) -> None:
    """A list after "not in the closure:" never reaches into the next paragraph or list item."""
    assert brief_deps.check(_repo(tmp_path, bodies={"M1-03": body})) == []
