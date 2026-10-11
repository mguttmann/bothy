# SPDX-License-Identifier: Apache-2.0
"""Tests for the forbidden-character check (en dash U+2013, em dash U+2014)."""

from __future__ import annotations

from pathlib import Path

import pytest
import text_rules

EN_DASH = chr(0x2013)
EM_DASH = chr(0x2014)


def test_clean_file_has_no_findings(tmp_path: Path) -> None:
    f = tmp_path / "ok.md"
    f.write_text("Plain hyphen - fine.\n", encoding="utf-8")
    assert text_rules.scan_file(f) == []


@pytest.mark.parametrize("char", [EN_DASH, EM_DASH])
def test_dash_is_reported_with_line_and_column(tmp_path: Path, char: str) -> None:
    f = tmp_path / "bad.md"
    f.write_text(f"first\nsecond {char} line\n", encoding="utf-8")
    findings = text_rules.scan_file(f)
    assert [(x.line, x.column, x.codepoint) for x in findings] == [(2, 8, f"U+{ord(char):04X}")]


def test_binary_file_is_skipped(tmp_path: Path) -> None:
    f = tmp_path / "img.png"
    f.write_bytes(b"\x89PNG\r\n\x1a\n\x00\x00" + EM_DASH.encode("utf-8"))
    assert text_rules.scan_file(f) == []


def test_non_utf8_text_is_reported_not_crashing(tmp_path: Path) -> None:
    f = tmp_path / "latin1.txt"
    f.write_bytes(b"caf\xe9\n")
    findings = text_rules.scan_file(f)
    assert findings
    assert findings[0].codepoint == "invalid-utf-8"


def test_main_returns_one_and_prints_location(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    good = tmp_path / "good.txt"
    good.write_text("ok\n", encoding="utf-8")
    bad = tmp_path / "bad.txt"
    bad.write_text(f"x{EN_DASH}y\n", encoding="utf-8")
    rc = text_rules.main([str(good), str(bad)])
    out = capsys.readouterr().out
    assert rc == 1
    assert f"{bad}:1:2: U+2013" in out


def test_main_returns_zero_when_clean(tmp_path: Path) -> None:
    good = tmp_path / "good.txt"
    good.write_text("ok\n", encoding="utf-8")
    assert text_rules.main([str(good)]) == 0


def test_repository_is_clean() -> None:
    """The gate itself: every tracked text file of this repository passes."""
    assert text_rules.main(["--all"]) == 0
