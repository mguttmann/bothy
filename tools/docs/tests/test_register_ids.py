# SPDX-License-Identifier: Apache-2.0
"""Tests for the register ID check (tools/docs/register_ids.py)."""

from __future__ import annotations

from pathlib import Path

import pytest
import register_ids

REPO = Path(__file__).resolve().parents[3]
# docs/spec/09-agents.md at efca7f7 (translated excerpt): R-AG-22 defined twice in one table
# (finding K3-05).
OLD_AGENTS = Path(__file__).resolve().parent / "data" / "09-agents-efca7f7.md.txt"

DECISIONS = (
    "# Log\n\n| ID | Decision |\n|---|---|\n"
    "| D-B1 | a |\n| D-W-01 | b |\n| D-K-A1 | c |\n| D-W-02 | d |\n\n"
    "```text\n| D-W-01 | inside a code block, ignored |\n```\n"
)
RISKS = (
    "# Risks\n\n| Rank | ID | Risk |\n|---|---|---|\n| 1 | RK-01 | x |\n\n"
    "| ID | Risk |\n|---|---|\n| RK-01 | a |\n| RK-02 | b |\n"
)
QUESTIONS = (
    "# Questions\n\n| ID | Question |\n|---|---|\n| OQ-01 | a |\n| OQ-02 | b |\n\n"
    "### OQ-01: A\n\nText.\n\n### OQ-02: B\n\nText.\n"
)


def _repo(
    tmp_path: Path, decisions: str = DECISIONS, risks: str = RISKS, questions: str = QUESTIONS
) -> Path:
    docs = tmp_path / "docs"
    docs.mkdir()
    (docs / "DECISIONS.md").write_text(decisions, encoding="utf-8")
    (docs / "RISKS.md").write_text(risks, encoding="utf-8")
    (docs / "OPEN-QUESTIONS.md").write_text(questions, encoding="utf-8")
    return tmp_path


def test_clean_registers_pass(tmp_path: Path) -> None:
    assert register_ids.check(_repo(tmp_path)) == []


def test_duplicate_decision_id_is_reported_with_both_lines(tmp_path: Path) -> None:
    root = _repo(tmp_path, decisions=DECISIONS + "\n| ID | E |\n|---|---|\n| D-W-02 | again |\n")
    problems = register_ids.check(root)
    assert len(problems) == 1
    assert "docs/DECISIONS.md:" in problems[0]
    assert "D-W-02" in problems[0]
    assert "line 8" in problems[0]


def test_duplicate_risk_id_is_reported(tmp_path: Path) -> None:
    root = _repo(tmp_path, risks=RISKS + "| RK-02 | c |\n")
    problems = register_ids.check(root)
    assert len(problems) == 1
    assert "RK-02" in problems[0]


def test_reference_in_a_later_column_is_not_a_definition(tmp_path: Path) -> None:
    # The ranking table names RK-01 in its second column; that is a reference, not a duplicate.
    assert register_ids.check(_repo(tmp_path)) == []


def test_duplicate_question_heading_is_reported(tmp_path: Path) -> None:
    root = _repo(tmp_path, questions=QUESTIONS + "\n### OQ-02: B again\n")
    problems = register_ids.check(root)
    assert any("OQ-02" in p and "heading" in p for p in problems)


def test_question_overview_and_headings_must_match(tmp_path: Path) -> None:
    root = _repo(tmp_path, questions=QUESTIONS + "\n### OQ-03: C\n\nText.\n")
    problems = register_ids.check(root)
    assert len(problems) == 1
    assert "OQ-03" in problems[0]
    assert "overview" in problems[0]


def test_main_exit_codes(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    assert register_ids.main(["--root", str(_repo(tmp_path))]) == 0
    bad = tmp_path / "bad"
    bad.mkdir()
    assert register_ids.main(["--root", str(_repo(bad, risks=RISKS + "| RK-01 | dup |\n"))]) == 1
    assert "RK-01" in capsys.readouterr().err


def _chapter(root: Path, rel: str, text: str) -> None:
    path = root / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def test_old_agents_chapter_with_r_ag_22_twice_is_reported(tmp_path: Path) -> None:
    root = _repo(tmp_path)
    _chapter(root, "docs/spec/09-agents.md", OLD_AGENTS.read_text(encoding="utf-8"))
    problems = register_ids.check(root)
    assert len(problems) == 1
    assert problems[0].startswith("docs/spec/09-agents.md:")
    assert "R-AG-22" in problems[0]


@pytest.mark.parametrize(
    "ident", ["R-QA-12", "T-SC-05", "SEC-TERM-01", "ID-T01", "AZ-M01", "M1-23", "D-W-01", "OQ-02"]
)
def test_chapter_id_defined_in_two_files_is_reported(tmp_path: Path, ident: str) -> None:
    root = _repo(tmp_path)
    table = f"| ID | Text |\n|---|---|\n| {ident} | a |\n"
    _chapter(root, "docs/spec/23-quality.md", table)
    _chapter(root, "docs/security/threat-model.md", table)
    problems = register_ids.check(root)
    assert any(ident in p and "already defined" in p for p in problems), problems


def test_duplicate_word_decision_id_is_reported(tmp_path: Path) -> None:
    """Decision IDs without a number (D-R-STACK) count as IDs too (P4)."""
    root = _repo(tmp_path, decisions=DECISIONS + "| D-R-STACK | a |\n| D-R-STACK | b |\n")
    problems = register_ids.check(root)
    assert len(problems) == 1
    assert "D-R-STACK" in problems[0]


def test_short_and_non_id_first_cells_are_not_checked(tmp_path: Path) -> None:
    root = _repo(tmp_path)
    table = "| No. | Text |\n|---|---|\n| M1 | a |\n| D1 | b |\n| `MIT-0` | c |\n| 1 | d |\n"
    _chapter(root, "docs/operations/upgrade.md", table)
    _chapter(root, "docs/compliance/ropa-template.md", table)
    assert register_ids.check(root) == []


def test_chapter_id_in_a_later_column_or_code_block_is_a_reference(tmp_path: Path) -> None:
    root = _repo(tmp_path)
    _chapter(root, "docs/spec/23-quality.md", "| ID | Text |\n|---|---|\n| R-QA-12 | a |\n")
    _chapter(
        root,
        "docs/security/threat-model.md",
        "| Rank | ID |\n|---|---|\n| 1 | R-QA-12 |\n\n```text\n| R-QA-12 | x |\n```\n",
    )
    assert register_ids.check(root) == []


def test_repository_registers_have_unique_ids() -> None:
    assert register_ids.check(REPO) == []


# docs/security/threat-model.md at 4a9ced3, section 5.16: two rows of the 7-column register 6.7
# (T-TM-13, T-TM-14) sat in the 5-column STRIDE table; the renderer dropped the extra cells
# (finding R5-01).
MISPLACED_ROWS = (
    "| Part | STRIDE | Threat | Mitigation | Register |\n"
    "|---|---|---|---|---|\n"
    "| Host service | T | Shell replaces the helper | HD10 | T-TM-12 |\n"
    "| T-TM-13 | **Shared location.** | medium | high | T13 | TM-T29, TM-T24 | low |\n"
    "| T-TM-14 | **Standstill.** | medium | medium | Instance operator | TM-T21 | medium |\n"
    "| Host service | E | Long runner | Process group | T-TM-11 |\n"
)


def test_row_with_more_cells_than_its_header_is_reported(tmp_path: Path) -> None:
    root = _repo(tmp_path)
    _chapter(root, "docs/security/threat-model.md", "# Model\n\n" + MISPLACED_ROWS)
    problems = register_ids.check(root)
    assert problems == [
        "docs/security/threat-model.md:6: table row has 7 cells, its header in line 3 has 5",
        "docs/security/threat-model.md:7: table row has 7 cells, its header in line 3 has 5",
    ]


def test_row_with_fewer_cells_than_its_header_is_reported(tmp_path: Path) -> None:
    root = _repo(tmp_path)
    _chapter(root, "docs/spec/23-quality.md", "| A | B | C |\n|---|---|---|\n| 1 | 2 |\n")
    assert register_ids.check(root) == [
        "docs/spec/23-quality.md:3: table row has 2 cells, its header in line 1 has 3"
    ]


def test_escaped_pipes_and_pipes_in_code_spans_do_not_split_cells(tmp_path: Path) -> None:
    root = _repo(tmp_path)
    table = (
        "| Name | Value |\n|:---|---:|\n| `a|b` | x \\| y |\n| ``c`|`d`` | z |\n"
        "| `open | tick |\n| back\\\\ | slash |\n| `\\|` and `|` | escaped in code |\n\n"
        "Text with | a pipe is no table.\n"
    )
    _chapter(root, "docs/spec/24-terminal.md", table)
    assert register_ids.check(root) == []


def test_a_table_ends_at_the_first_line_without_a_pipe(tmp_path: Path) -> None:
    root = _repo(tmp_path)
    tables = "| A | B |\n|---|---|\n| 1 | 2 |\n\n| A | B | C |\n|---|---|---|\n| 1 | 2 | 3 |\n"
    _chapter(root, "docs/spec/16-operations.md", tables)
    assert register_ids.check(root) == []
