# SPDX-License-Identifier: Apache-2.0
"""Tests of tools/ci/lcov_filter.py: generated Dart files leave the coverage sum (brief M4-01)."""

from __future__ import annotations

from pathlib import Path

import lcov_filter
import pytest

HAND = "SF:lib/src/hand.dart\nDA:1,1\nDA:2,0\nLF:2\nLH:1\nend_of_record\n"


def _record(path: str, found: int, hit: int) -> str:
    return f"SF:{path}\nLF:{found}\nLH:{hit}\nend_of_record\n"


@pytest.mark.parametrize(
    "path",
    [
        "lib/src/model.g.dart",
        "lib/src/model.freezed.dart",
        "lib/src/db.drift.dart",
        "lib/src/generated/api.dart",
        "lib/src/generated/deep/model.dart",
        "build/x.dart",
        "/abs/member/lib/src/generated/api.dart",
        "/abs/member/lib/x.g.dart",
    ],
)
def test_generated_records_are_dropped(path: str) -> None:
    assert lcov_filter.is_generated(path)
    text = HAND + _record(path, 10, 0)
    assert lcov_filter.filter_lcov(text) == HAND


@pytest.mark.parametrize(
    "path",
    ["lib/src/hand.dart", "lib/src/generator.dart", "lib/generated.dart", "lib/src/build/x.dart"],
)
def test_hand_written_records_are_kept(path: str) -> None:
    assert not lcov_filter.is_generated(path)


def test_totals_are_recomputed_without_generated_records() -> None:
    text = HAND + _record("lib/src/x.g.dart", 10, 0) + _record("lib/b.dart", 8, 8)
    assert lcov_filter.totals(lcov_filter.filter_lcov(text)) == (10, 9)
    assert lcov_filter.totals(text) == (20, 9)


def test_main_prints_totals_and_filtered_records(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    lcov = tmp_path / "lcov.info"
    lcov.write_text(HAND + _record("lib/x.g.dart", 4, 0), encoding="utf-8")
    assert lcov_filter.main(["--totals", str(lcov)]) == 0
    assert capsys.readouterr().out == "2 1\n"
    assert lcov_filter.main([str(lcov)]) == 0
    assert capsys.readouterr().out == HAND


def test_empty_report_has_no_lines(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    lcov = tmp_path / "lcov.info"
    lcov.write_text("", encoding="utf-8")
    assert lcov_filter.main(["--totals", str(lcov)]) == 0
    assert capsys.readouterr().out == "0 0\n"


def test_missing_report_fails(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    assert lcov_filter.main(["--totals", str(tmp_path / "none.info")]) == 2
    assert "lcov_filter: cannot read" in capsys.readouterr().err
