# SPDX-License-Identifier: Apache-2.0
"""Every build brief has the shape the launcher and the build sessions rely on.

docs/briefs/README.md names the parts of a brief (goal, chapters to read, fixed build rules,
scope and boundaries, tests first, acceptance criteria, report format) and its index lists the
title and file of each ticket. This test holds every brief of the index to that shape: the title
line equals the index row, the status line names the milestone of the ticket, the eleven
sections stand in their order, the report uses the launcher placeholders, and every brief written
before M1 closed (milestones M2 and later, D-W-100 and D-W-114) carries the limit for names that
differ from merged code. Dependencies, waves and make targets are checked by brief_deps.py.
"""

from __future__ import annotations

import re
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]
BRIEFS = REPO / "docs" / "briefs"
SECTIONS = [
    "Goal",
    "Read first",
    "Fixed requirements that apply",
    "Scope",
    "Out of scope",
    "Boundaries",
    "Design notes",
    "Tests (write them first)",
    "Acceptance criteria",
    "Definition of done",
    "Report",
]
_ROW = re.compile(r"^\| \d+ \| (M\d-\d\d) \| .+? \| `([^`]+)` \|", re.MULTILINE)
_STATUS = re.compile(r"^Status: ready \| Depends on: [^|]+ \| Milestone: (M\d)$")


def index_rows() -> list[tuple[str, str]]:
    """Ticket and brief file of every row of the brief index."""
    return _ROW.findall((BRIEFS / "README.md").read_text(encoding="utf-8"))


def problems(tid: str, text: str) -> list[str]:
    """Return one message per deviation of a brief from the shape."""
    found: list[str] = []
    lines = text.splitlines()
    if not lines or not lines[0].startswith(f"# {tid}: ") or not lines[0][len(tid) + 4 :].strip():
        found.append(f"title line does not start with '# {tid}: ' and a title")
    status = _STATUS.match(lines[2]) if len(lines) > 3 else None
    if status is None or lines[1] != "" or lines[3] != "":
        found.append("line 3 is not the status line between blank lines")
    elif status.group(1) != tid[:2]:
        found.append(f"status line names milestone {status.group(1)}, ticket is {tid[:2]}")
    headings = [line[3:] for line in lines if line.startswith("## ")]
    if headings != SECTIONS:
        found.append(f"sections {headings} differ from {SECTIONS}")
    report = text.split("\n## Report\n", 1)[-1]
    for needle in ("`{{REPORT_PATH}}`", "`{{DONE_PATH}}`", "`Verdict: DONE`", "`Verdict: BLOCKED"):
        if needle not in report:
            found.append(f"report section lacks {needle}")
    if tid[:2] not in ("M0", "M1") and "D-W-100" not in text.split("\n## Boundaries\n", 1)[-1]:
        found.append("boundaries lack the limit of D-W-100 for names that differ from merged code")
    return found


@pytest.mark.parametrize(("tid", "name"), index_rows(), ids=str)
def test_brief_has_the_shape(tid: str, name: str) -> None:
    """Each brief of the index has title, status line, sections and report as described."""
    text = (BRIEFS / name).read_text(encoding="utf-8")
    assert problems(tid, text) == []


def test_a_missing_section_is_found() -> None:
    """The check is not vacuous: a brief without its "Out of scope" section fails."""
    tid, name = index_rows()[0]
    text = (BRIEFS / name).read_text(encoding="utf-8").replace("\n## Out of scope\n", "\n")
    assert any("sections" in p for p in problems(tid, text))


def test_a_wrong_milestone_is_found() -> None:
    """A status line that names another milestone than the ticket ID fails."""
    tid, name = index_rows()[0]
    text = (BRIEFS / name).read_text(encoding="utf-8")
    wrong = re.sub(r"Milestone: M\d", "Milestone: M9", text, count=1)
    assert any("milestone M9" in p for p in problems(tid, wrong))


def test_index_lists_every_brief_file() -> None:
    """No brief file exists outside the index (a forgotten row would escape every check)."""
    listed = {name for _, name in index_rows()}
    on_disk = {p.name for p in BRIEFS.glob("M*.md")}
    assert on_disk == listed
