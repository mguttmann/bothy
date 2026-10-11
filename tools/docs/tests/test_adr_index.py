# SPDX-License-Identifier: Apache-2.0
"""Tests for the generated ADR index (docs/adr/README.md)."""

from __future__ import annotations

from pathlib import Path

import adr_index
import pytest

REPO = Path(__file__).resolve().parents[3]

ADR = """---
status: {status}
date: 2026-10-10
decision-makers: Owner
---

# {title}

## Context and Problem Statement
"""

README = """# Decisions

Intro text stays.

<!-- adr-index:start -->
old
<!-- adr-index:end -->

Footer stays.
"""


def _adr_dir(tmp_path: Path) -> Path:
    d = tmp_path / "adr"
    d.mkdir()
    (d / "0001-adrs-in-madr-format.md").write_text(
        ADR.format(status="accepted", title="ADRs in MADR format")
    )
    (d / "0040-sonarqube-local.md").write_text(
        ADR.format(status="proposed", title="SonarQube local")
    )
    (d / "0000-template.md").write_text(ADR.format(status="template", title="Template"))
    (d / "README.md").write_text(README)
    return d


def test_collect_reads_number_title_status(tmp_path: Path) -> None:
    entries = adr_index.collect(_adr_dir(tmp_path))
    assert [(e.number, e.title, e.status) for e in entries] == [
        ("0000", "Template", "template"),
        ("0001", "ADRs in MADR format", "accepted"),
        ("0040", "SonarQube local", "proposed"),
    ]


def test_render_replaces_only_marked_block(tmp_path: Path) -> None:
    d = _adr_dir(tmp_path)
    text = adr_index.render((d / "README.md").read_text(), adr_index.collect(d))
    assert text.startswith("# Decisions\n\nIntro text stays.\n")
    assert text.rstrip().endswith("Footer stays.")
    assert "<!-- adr-index:start -->\n| No. | Title | Status | Date |\n|---|---|---|---|\n" in text
    assert "| [0040](0040-sonarqube-local.md) | SonarQube local | proposed | 2026-10-10 |" in text
    assert "old" not in text


def test_check_mode_detects_stale_index(tmp_path: Path) -> None:
    d = _adr_dir(tmp_path)
    assert adr_index.main(["--dir", str(d), "--check"]) == 1
    assert adr_index.main(["--dir", str(d)]) == 0
    assert adr_index.main(["--dir", str(d), "--check"]) == 0


def test_status_from_body_when_front_matter_is_absent(tmp_path: Path) -> None:
    d = _adr_dir(tmp_path)
    (d / "0002-no-front-matter.md").write_text(
        "# No front matter\n\n**Status:** accepted\nDate: 2026-10-09\n"
    )
    (d / "0003-empty.md").write_text("# Empty\n\nText.\n")
    entries = {e.number: e for e in adr_index.collect(d)}
    assert (entries["0002"].status, entries["0002"].date) == ("accepted", "2026-10-09")
    assert (entries["0003"].status, entries["0003"].date) == ("MISSING", "MISSING")


def test_quoted_front_matter_values(tmp_path: Path) -> None:
    d = _adr_dir(tmp_path)
    (d / "0004-quoted.md").write_text('---\nstatus: "accepted"\ndate: 2026-10-10\n---\n\n# Q\n')
    assert {e.number: e.status for e in adr_index.collect(d)}["0004"] == "accepted"


def test_missing_title_is_an_error(tmp_path: Path) -> None:
    d = _adr_dir(tmp_path)
    (d / "0002-broken.md").write_text("no title\n")
    with pytest.raises(adr_index.AdrError, match=r"0002-broken\.md"):
        adr_index.collect(d)


def test_bad_file_name_is_an_error(tmp_path: Path) -> None:
    d = _adr_dir(tmp_path)
    (d / "12-Short.md").write_text(ADR.format(status="accepted", title="x"))
    with pytest.raises(adr_index.AdrError, match=r"12-Short\.md"):
        adr_index.collect(d)


def test_duplicate_number_is_an_error(tmp_path: Path) -> None:
    d = _adr_dir(tmp_path)
    (d / "0001-duplicate.md").write_text(ADR.format(status="accepted", title="x"))
    with pytest.raises(adr_index.AdrError, match="0001"):
        adr_index.collect(d)


def test_repository_index_is_current() -> None:
    assert adr_index.main(["--dir", str(REPO / "docs" / "adr"), "--check"]) == 0
