# SPDX-License-Identifier: Apache-2.0
"""Tests for the risk ranking check (tools/docs/risk_ranking.py)."""

from __future__ import annotations

from pathlib import Path

import pytest
import risk_ranking

REPO = Path(__file__).resolve().parents[3]

RANKING = (
    "| Rang | ID | Risiko | Wahrscheinlichkeit | Auswirkung |\n"
    "|---|---|---|---|---|\n"
    "| 1 | RK-01 | a | hoch | hoch |\n"
    "| 2 | RK-13 | b | mittel | hoch |\n"
    "| 3 | RK-07 | c | mittel | hoch |\n"
)
REGISTER = (
    "| ID | Risiko | Wahrscheinlichkeit | Auswirkung | Gegenmaßnahme |\n"
    "|---|---|---|---|---|\n"
    "| RK-01 | a | hoch | hoch | x |\n"
    "| RK-07 | c, `a|b` | mittel | hoch | x \\| y |\n"
    "| RK-13 | b | mittel | hoch | x |\n"
    "| RK-11 | d | hoch | mittel | x |\n"
    "| RK-10 | e | niedrig | niedrig | x |\n"
)
DONE = "| Risiko | Erledigt durch |\n|---|---|\n| alt | D-W-01 |\n"
# RK-10 raised to the rating of rank 3.
LOW_RK10 = "| RK-10 | e | niedrig | niedrig |"
RAISED_RK10 = "| RK-10 | e | mittel | hoch |"
# The list behind the last rank names every unranked risk with the rating of the last rank in the
# bold span that starts a list item; a mention elsewhere (here RK-07) names nothing.
BEHIND = (
    "**Hinter Rang 3** stehen alle übrigen Risiken mit derselben Bewertung:\n\n"
    "- **RK-11** (d) hat weniger Rest als **RK-07**, sagt Kriterium 3.\n\n"
)


def _risks(ranking: str = RANKING, register: str = REGISTER, behind: str = BEHIND) -> str:
    return (
        "# Risiken\n\n## Die fünf größten Risiken\n\n"
        + ranking
        + "\n"
        + behind
        + "## Register\n\n### Teil\n\n"
        + register
        + "\n## Erledigt\n\n"
        + DONE
    )


def _repo(tmp_path: Path, text: str) -> Path:
    docs = tmp_path / "docs"
    docs.mkdir()
    (docs / "RISKS.md").write_text(text, encoding="utf-8")
    return tmp_path


def test_consistent_ranking_passes(tmp_path: Path) -> None:
    # RK-11 (hoch, mittel) has the same rating as rank 3; it stays outside the ranking and is named
    # in the list behind rank 3.
    assert risk_ranking.check(_repo(tmp_path, _risks())) == []


def test_unranked_risk_with_a_higher_rating_is_reported(tmp_path: Path) -> None:
    # docs/RISKS.md before D-W-78: RK-77 had been raised to hoch/hoch, the ranking still held four
    # risks rated mittel/hoch and said RK-01 was the only one rated hoch/hoch (finding C4-14).
    register = REGISTER + "| RK-77 | f | hoch | hoch | x |\n"
    problems = risk_ranking.check(_repo(tmp_path, _risks(register=register)))
    assert problems == [
        "docs/RISKS.md:26: RK-77 is rated 9 (hoch x hoch) but not ranked, "
        "rank 3 RK-07 is rated 6 (mittel x hoch)"
    ]


def test_rating_that_differs_from_the_register_is_reported(tmp_path: Path) -> None:
    ranking = RANKING.replace("| 2 | RK-13 | b | mittel |", "| 2 | RK-13 | b | hoch |")
    problems = risk_ranking.check(_repo(tmp_path, _risks(ranking=ranking)))
    assert problems == [
        "docs/RISKS.md:8: rank 2 RK-13 is rated hoch x hoch, the register in line 23 says "
        "mittel x hoch"
    ]


def test_ranking_that_rises_is_reported(tmp_path: Path) -> None:
    ranking = (
        RANKING.split("| 1 |", maxsplit=1)[0]
        + "| 1 | RK-13 | b | mittel | hoch |\n| 2 | RK-01 | a | hoch | hoch |\n"
    )
    problems = risk_ranking.check(_repo(tmp_path, _risks(ranking=ranking, behind="")))
    assert problems == [
        "docs/RISKS.md:8: rank 2 RK-01 is rated 9 (hoch x hoch), higher than rank 1 RK-13 "
        "with 6 (mittel x hoch)"
    ]


def test_ranked_risk_missing_from_the_register_is_reported(tmp_path: Path) -> None:
    register = REGISTER.replace("| RK-07 | c, `a|b` | mittel | hoch | x \\| y |\n", "")
    problems = risk_ranking.check(_repo(tmp_path, _risks(register=register)))
    assert problems == ["docs/RISKS.md:9: rank 3 RK-07 is not in the register"]


def test_ranks_must_count_from_one_without_gaps(tmp_path: Path) -> None:
    ranking = RANKING.replace("| 3 | RK-07 |", "| 4 | RK-07 |")
    behind = BEHIND.replace("Rang 3", "Rang 4")
    problems = risk_ranking.check(_repo(tmp_path, _risks(ranking=ranking, behind=behind)))
    assert problems == ["docs/RISKS.md:9: rank 4 follows rank 2, expected rank 3"]


def test_risk_ranked_twice_is_reported(tmp_path: Path) -> None:
    ranking = RANKING + "| 4 | RK-13 | b | mittel | hoch |\n"
    problems = risk_ranking.check(_repo(tmp_path, _risks(ranking=ranking)))
    assert problems == ["docs/RISKS.md:10: rank 4 RK-13 is already rank 2"]


@pytest.mark.parametrize("value", ["sehr hoch", "", "Hoch"])
def test_unknown_rating_word_is_reported(tmp_path: Path, value: str) -> None:
    register = REGISTER.replace("| RK-10 | e | niedrig |", f"| RK-10 | e | {value} |")
    problems = risk_ranking.check(_repo(tmp_path, _risks(register=register)))
    assert problems == [
        f"docs/RISKS.md:25: RK-10 has the likelihood {value!r}, allowed are niedrig, mittel, hoch"
    ]


def test_missing_ranking_table_is_reported(tmp_path: Path) -> None:
    problems = risk_ranking.check(_repo(tmp_path, _risks(ranking="Keine Tabelle.\n")))
    assert problems == ["docs/RISKS.md: no ranking table (header 'Rang | ID | ...')"]


def test_second_ranking_table_is_reported(tmp_path: Path) -> None:
    text = _risks() + "\n" + RANKING
    problems = risk_ranking.check(_repo(tmp_path, text))
    assert problems == ["docs/RISKS.md:33: a second ranking table, the first starts in line 5"]


def test_tables_in_code_blocks_are_ignored(tmp_path: Path) -> None:
    text = _risks() + "\n```text\n" + RANKING + "| RK-77 | f | hoch | hoch | x |\n```\n"
    assert risk_ranking.check(_repo(tmp_path, text)) == []


def test_unnamed_risk_with_the_rating_of_the_last_rank_is_reported(tmp_path: Path) -> None:
    # A risk raised to the rating of the last rank must join the list behind it; the list after
    # D-W-78 named its risks by hand and no check held it (finding RT-C4R-03).
    register = REGISTER.replace(LOW_RK10, RAISED_RK10)
    problems = risk_ranking.check(_repo(tmp_path, _risks(register=register)))
    assert problems == [
        "docs/RISKS.md:25: RK-10 is rated 6 (mittel x hoch) like rank 3 RK-07 but not named "
        "in the list behind rank 3"
    ]


def test_risks_with_the_rating_of_the_last_rank_need_the_list(tmp_path: Path) -> None:
    problems = risk_ranking.check(_repo(tmp_path, _risks(behind="")))
    assert problems == [
        "docs/RISKS.md:20: RK-11 is rated 6 (hoch x mittel) like rank 3 RK-07 but not named "
        "in the list behind rank 3"
    ]


def test_list_behind_the_last_rank_may_be_missing_without_ties(tmp_path: Path) -> None:
    register = REGISTER.replace("| RK-11 | d | hoch | mittel |", "| RK-11 | d | mittel | mittel |")
    assert risk_ranking.check(_repo(tmp_path, _risks(register=register, behind=""))) == []


def test_named_risk_with_another_rating_is_reported(tmp_path: Path) -> None:
    behind = BEHIND.replace("- **RK-11**", "- **RK-11 und RK-10**")
    problems = risk_ranking.check(_repo(tmp_path, _risks(behind=behind)))
    assert problems == [
        "docs/RISKS.md:13: RK-10 is named behind rank 3 but rated 1 (niedrig x niedrig), "
        "rank 3 RK-07 is rated 6 (mittel x hoch)"
    ]


def test_named_ranked_risk_is_reported(tmp_path: Path) -> None:
    behind = BEHIND + "- **RK-13** steht doch schon oben.\n\n"
    problems = risk_ranking.check(_repo(tmp_path, _risks(behind=behind)))
    assert problems == ["docs/RISKS.md:15: RK-13 is named behind rank 3 but is rank 2"]


def test_named_unknown_risk_is_reported(tmp_path: Path) -> None:
    behind = BEHIND.replace("- **RK-11**", "- **RK-11, RK-99**")
    problems = risk_ranking.check(_repo(tmp_path, _risks(behind=behind)))
    assert problems == ["docs/RISKS.md:13: RK-99 is named behind rank 3 but not in the register"]


def test_risk_named_twice_is_reported(tmp_path: Path) -> None:
    behind = BEHIND + "- **RK-11** noch einmal.\n\n"
    problems = risk_ranking.check(_repo(tmp_path, _risks(behind=behind)))
    assert problems == ["docs/RISKS.md:15: RK-11 is named twice behind rank 3, first in line 13"]


def test_list_behind_another_rank_is_reported(tmp_path: Path) -> None:
    behind = BEHIND.replace("Rang 3", "Rang 5")
    problems = risk_ranking.check(_repo(tmp_path, _risks(behind=behind)))
    assert problems == [
        "docs/RISKS.md:11: the list '**Hinter Rang 5**' names rank 5, the last rank is 3"
    ]


def test_second_list_behind_the_last_rank_is_reported(tmp_path: Path) -> None:
    text = _risks() + "\n" + BEHIND
    problems = risk_ranking.check(_repo(tmp_path, text))
    assert problems == [
        "docs/RISKS.md:33: a second list '**Hinter Rang 3**', the first starts in line 11"
    ]


def test_list_ends_at_the_next_heading(tmp_path: Path) -> None:
    # RK-10 in a list item after the next heading belongs to another section, not to the list.
    register = REGISTER.replace(LOW_RK10, RAISED_RK10)
    text = _risks(register=register) + "\n- **RK-10** steht in einem anderen Abschnitt.\n"
    problems = risk_ranking.check(_repo(tmp_path, text))
    assert problems == [
        "docs/RISKS.md:25: RK-10 is rated 6 (mittel x hoch) like rank 3 RK-07 but not named "
        "in the list behind rank 3"
    ]


def test_main_exit_codes(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    assert risk_ranking.main(["--root", str(_repo(tmp_path, _risks()))]) == 0
    bad = tmp_path / "bad"
    bad.mkdir()
    register = REGISTER + "| RK-77 | f | hoch | hoch | x |\n"
    assert risk_ranking.main(["--root", str(_repo(bad, _risks(register=register)))]) == 1
    assert "RK-77" in capsys.readouterr().err


def test_repository_ranking_follows_the_register() -> None:
    assert risk_ranking.check(REPO) == []
