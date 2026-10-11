# SPDX-License-Identifier: Apache-2.0
"""Tests for the risk ranking check (tools/docs/risk_ranking.py)."""

from __future__ import annotations

from pathlib import Path

import pytest
import risk_ranking

REPO = Path(__file__).resolve().parents[3]

RANKING = (
    "| Rank | ID | Risk | Likelihood | Impact |\n"
    "|---|---|---|---|---|\n"
    "| 1 | RK-01 | a | high | high |\n"
    "| 2 | RK-13 | b | medium | high |\n"
    "| 3 | RK-07 | c | medium | high |\n"
)
REGISTER = (
    "| ID | Risk | Likelihood | Impact | Mitigation |\n"
    "|---|---|---|---|---|\n"
    "| RK-01 | a | high | high | x |\n"
    "| RK-07 | c, `a|b` | medium | high | x \\| y |\n"
    "| RK-13 | b | medium | high | x |\n"
    "| RK-11 | d | high | medium | x |\n"
    "| RK-10 | e | low | low | x |\n"
)
DONE = "| Risk | Done through |\n|---|---|\n| old | D-W-01 |\n"
# RK-10 raised to the rating of rank 3.
LOW_RK10 = "| RK-10 | e | low | low |"
RAISED_RK10 = "| RK-10 | e | medium | high |"
# The list behind the last rank names every unranked risk with the rating of the last rank in the
# bold span that starts a list item; a mention elsewhere (here RK-07) names nothing.
BEHIND = (
    "**Behind rank 3** stand all other risks with the same rating:\n\n"
    "- **RK-11** (d) has less residue than **RK-07**, says criterion 3.\n\n"
)


def _risks(ranking: str = RANKING, register: str = REGISTER, behind: str = BEHIND) -> str:
    return (
        "# Risks\n\n## The five largest risks\n\n"
        + ranking
        + "\n"
        + behind
        + "## Register\n\n### Part\n\n"
        + register
        + "\n## Done\n\n"
        + DONE
    )


def _repo(tmp_path: Path, text: str) -> Path:
    docs = tmp_path / "docs"
    docs.mkdir()
    (docs / "RISKS.md").write_text(text, encoding="utf-8")
    return tmp_path


def test_consistent_ranking_passes(tmp_path: Path) -> None:
    # RK-11 (high, medium) has the same rating as rank 3; it stays outside the ranking and is named
    # in the list behind rank 3.
    assert risk_ranking.check(_repo(tmp_path, _risks())) == []


def test_unranked_risk_with_a_higher_rating_is_reported(tmp_path: Path) -> None:
    # docs/RISKS.md before D-W-78: RK-77 had been raised to high/high, the ranking still held four
    # risks rated medium/high and said RK-01 was the only one rated high/high (finding C4-14).
    register = REGISTER + "| RK-77 | f | high | high | x |\n"
    problems = risk_ranking.check(_repo(tmp_path, _risks(register=register)))
    assert problems == [
        "docs/RISKS.md:26: RK-77 is rated 9 (high x high) but not ranked, "
        "rank 3 RK-07 is rated 6 (medium x high)"
    ]


def test_rating_that_differs_from_the_register_is_reported(tmp_path: Path) -> None:
    ranking = RANKING.replace("| 2 | RK-13 | b | medium |", "| 2 | RK-13 | b | high |")
    problems = risk_ranking.check(_repo(tmp_path, _risks(ranking=ranking)))
    assert problems == [
        "docs/RISKS.md:8: rank 2 RK-13 is rated high x high, the register in line 23 says "
        "medium x high"
    ]


def test_ranking_that_rises_is_reported(tmp_path: Path) -> None:
    ranking = (
        RANKING.split("| 1 |", maxsplit=1)[0]
        + "| 1 | RK-13 | b | medium | high |\n| 2 | RK-01 | a | high | high |\n"
    )
    problems = risk_ranking.check(_repo(tmp_path, _risks(ranking=ranking, behind="")))
    assert problems == [
        "docs/RISKS.md:8: rank 2 RK-01 is rated 9 (high x high), higher than rank 1 RK-13 "
        "with 6 (medium x high)"
    ]


def test_ranked_risk_missing_from_the_register_is_reported(tmp_path: Path) -> None:
    register = REGISTER.replace("| RK-07 | c, `a|b` | medium | high | x \\| y |\n", "")
    problems = risk_ranking.check(_repo(tmp_path, _risks(register=register)))
    assert problems == ["docs/RISKS.md:9: rank 3 RK-07 is not in the register"]


def test_ranks_must_count_from_one_without_gaps(tmp_path: Path) -> None:
    ranking = RANKING.replace("| 3 | RK-07 |", "| 4 | RK-07 |")
    behind = BEHIND.replace("rank 3", "rank 4")
    problems = risk_ranking.check(_repo(tmp_path, _risks(ranking=ranking, behind=behind)))
    assert problems == ["docs/RISKS.md:9: rank 4 follows rank 2, expected rank 3"]


def test_risk_ranked_twice_is_reported(tmp_path: Path) -> None:
    ranking = RANKING + "| 4 | RK-13 | b | medium | high |\n"
    problems = risk_ranking.check(_repo(tmp_path, _risks(ranking=ranking)))
    assert problems == ["docs/RISKS.md:10: rank 4 RK-13 is already rank 2"]


@pytest.mark.parametrize("value", ["very high", "", "High"])
def test_unknown_rating_word_is_reported(tmp_path: Path, value: str) -> None:
    register = REGISTER.replace("| RK-10 | e | low |", f"| RK-10 | e | {value} |")
    problems = risk_ranking.check(_repo(tmp_path, _risks(register=register)))
    assert problems == [
        f"docs/RISKS.md:25: RK-10 has the likelihood {value!r}, allowed are low, medium, high"
    ]


def test_missing_ranking_table_is_reported(tmp_path: Path) -> None:
    problems = risk_ranking.check(_repo(tmp_path, _risks(ranking="No table.\n")))
    assert problems == ["docs/RISKS.md: no ranking table (header 'Rank | ID | ...')"]


def test_second_ranking_table_is_reported(tmp_path: Path) -> None:
    text = _risks() + "\n" + RANKING
    problems = risk_ranking.check(_repo(tmp_path, text))
    assert problems == ["docs/RISKS.md:33: a second ranking table, the first starts in line 5"]


def test_tables_in_code_blocks_are_ignored(tmp_path: Path) -> None:
    text = _risks() + "\n```text\n" + RANKING + "| RK-77 | f | high | high | x |\n```\n"
    assert risk_ranking.check(_repo(tmp_path, text)) == []


def test_unnamed_risk_with_the_rating_of_the_last_rank_is_reported(tmp_path: Path) -> None:
    # A risk raised to the rating of the last rank must join the list behind it; the list after
    # D-W-78 named its risks by hand and no check held it (finding RT-C4R-03).
    register = REGISTER.replace(LOW_RK10, RAISED_RK10)
    problems = risk_ranking.check(_repo(tmp_path, _risks(register=register)))
    assert problems == [
        "docs/RISKS.md:25: RK-10 is rated 6 (medium x high) like rank 3 RK-07 but not named "
        "in the list behind rank 3"
    ]


def test_risks_with_the_rating_of_the_last_rank_need_the_list(tmp_path: Path) -> None:
    problems = risk_ranking.check(_repo(tmp_path, _risks(behind="")))
    assert problems == [
        "docs/RISKS.md:20: RK-11 is rated 6 (high x medium) like rank 3 RK-07 but not named "
        "in the list behind rank 3"
    ]


def test_list_behind_the_last_rank_may_be_missing_without_ties(tmp_path: Path) -> None:
    register = REGISTER.replace("| RK-11 | d | high | medium |", "| RK-11 | d | medium | medium |")
    assert risk_ranking.check(_repo(tmp_path, _risks(register=register, behind=""))) == []


def test_named_risk_with_another_rating_is_reported(tmp_path: Path) -> None:
    behind = BEHIND.replace("- **RK-11**", "- **RK-11 and RK-10**")
    problems = risk_ranking.check(_repo(tmp_path, _risks(behind=behind)))
    assert problems == [
        "docs/RISKS.md:13: RK-10 is named behind rank 3 but rated 1 (low x low), "
        "rank 3 RK-07 is rated 6 (medium x high)"
    ]


def test_named_ranked_risk_is_reported(tmp_path: Path) -> None:
    behind = BEHIND + "- **RK-13** is already ranked above.\n\n"
    problems = risk_ranking.check(_repo(tmp_path, _risks(behind=behind)))
    assert problems == ["docs/RISKS.md:15: RK-13 is named behind rank 3 but is rank 2"]


def test_named_unknown_risk_is_reported(tmp_path: Path) -> None:
    behind = BEHIND.replace("- **RK-11**", "- **RK-11, RK-99**")
    problems = risk_ranking.check(_repo(tmp_path, _risks(behind=behind)))
    assert problems == ["docs/RISKS.md:13: RK-99 is named behind rank 3 but not in the register"]


def test_risk_named_twice_is_reported(tmp_path: Path) -> None:
    behind = BEHIND + "- **RK-11** once more.\n\n"
    problems = risk_ranking.check(_repo(tmp_path, _risks(behind=behind)))
    assert problems == ["docs/RISKS.md:15: RK-11 is named twice behind rank 3, first in line 13"]


def test_list_behind_another_rank_is_reported(tmp_path: Path) -> None:
    behind = BEHIND.replace("rank 3", "rank 5")
    problems = risk_ranking.check(_repo(tmp_path, _risks(behind=behind)))
    assert problems == [
        "docs/RISKS.md:11: the list '**Behind rank 5**' names rank 5, the last rank is 3"
    ]


def test_second_list_behind_the_last_rank_is_reported(tmp_path: Path) -> None:
    text = _risks() + "\n" + BEHIND
    problems = risk_ranking.check(_repo(tmp_path, text))
    assert problems == [
        "docs/RISKS.md:33: a second list '**Behind rank 3**', the first starts in line 11"
    ]


def test_list_ends_at_the_next_heading(tmp_path: Path) -> None:
    # RK-10 in a list item after the next heading belongs to another section, not to the list.
    register = REGISTER.replace(LOW_RK10, RAISED_RK10)
    text = _risks(register=register) + "\n- **RK-10** stands in another section.\n"
    problems = risk_ranking.check(_repo(tmp_path, text))
    assert problems == [
        "docs/RISKS.md:25: RK-10 is rated 6 (medium x high) like rank 3 RK-07 but not named "
        "in the list behind rank 3"
    ]


def test_main_exit_codes(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    assert risk_ranking.main(["--root", str(_repo(tmp_path, _risks()))]) == 0
    bad = tmp_path / "bad"
    bad.mkdir()
    register = REGISTER + "| RK-77 | f | high | high | x |\n"
    assert risk_ranking.main(["--root", str(_repo(bad, _risks(register=register)))]) == 1
    assert "RK-77" in capsys.readouterr().err


def test_repository_ranking_follows_the_register() -> None:
    assert risk_ranking.check(REPO) == []
