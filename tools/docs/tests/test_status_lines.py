# SPDX-License-Identifier: Apache-2.0
"""Status lines use the fixed vocabulary of docs/SPEC.md and agree with the roadmap.

A document says in its first lines what it is: `Status: <state>[, <state>]. ...`. The states are
the words of docs/SPEC.md section 8.1: `normative`, `built` and `planned`, plus the two kinds of
page that are neither (`design` for a dated design note, `template` for a form an operator
fills). `built` and `planned` name the milestones or tickets they speak about, and the table of
docs/roadmap.md decides whether the claim holds: a milestone named as built is `done`, `built` or
`in progress` there, a milestone named as planned is not `done` or `built`. So a page cannot say
"draft M0, no product code yet" while the roadmap lists the milestones M1 to M8 as built
(docs/decisions.d/AUD-docs-spec-01.md).

Checked are the pages under docs/ except the historical records (briefs, decision logs, ADRs),
which keep the words of their time, and the READMEs of the component roots that describe the
state of a whole folder. The READMEs of single modules and packages are not checked.

The test lives with the tools tests because they run on every change, also on one that touches
only a document.
"""

from __future__ import annotations

import re
import subprocess
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]
SPEC = REPO / "docs" / "SPEC.md"
ROADMAP = REPO / "docs" / "roadmap.md"
# The states of docs/SPEC.md section 8.1 and the two kinds of page that carry none of them.
KINDS = ("design", "template")
VOCABULARY = ("normative", "built", "planned", *KINDS)
# The states the roadmap is asked about.
BUILT, PLANNED = "built", "planned"
# Roadmap states that allow "built" and that forbid "planned".
ALLOWS_BUILT = ("done", "built", "in progress")
FORBIDS_PLANNED = ("done", "built")
# Historical records: their text stays as written.
HISTORY = re.compile(
    r"^docs/briefs/|^docs/decisions\.d/|^docs/DECISIONS[^/]*\.md$|^docs/adr/|^docs/api/dart/"
)
# READMEs outside docs/ that describe the state of a whole folder.
FOLDER_READMES = ("services/server/README.md", "apps/README.md", "packages/README.md")
# How many lines from the top a status line may stand.
TOP = 15

_STATUS = re.compile(r"^Status:[ \t]*(?P<body>.*?)(?=\n[ \t]*\n|\Z)", re.MULTILINE | re.DOTALL)
_REF = re.compile(r"\b(?P<milestone>M\d|V\d|A\d|AR)(?:-\d+[a-z]?)?\b")
_RANGE = re.compile(r"\b(?P<letter>[MV])(?P<low>\d) to (?P=letter)(?P<high>\d)\b")
_ROW = re.compile(r"^\| *(?P<id>M\d|V\d|A\d|AR) [^|]*\|.*\| *(?P<state>[^|]+?) *\|$", re.MULTILINE)


def status_of(text: str) -> str | None:
    """Return the status paragraph of a document without the word `Status:`, on one line.

    Example: `status_of("# T\\n\\nStatus: built (M7).\\nGuide.\\n\\nText")` returns
    `"built (M7). Guide."`; a document without a status line in its first lines returns None.
    """
    top = "\n".join(text.splitlines()[:TOP])
    found = _STATUS.search(top)
    if found is None:
        return None
    start = found.start()
    full = _STATUS.match(text, start)
    assert full is not None
    return " ".join(full.group("body").split())


def _split(text: str, separators: str) -> list[str]:
    """Split `text` at the separators that stand outside parentheses and code spans."""
    parts, depth, code, current = [], 0, False, ""
    for char in text:
        if char == "`":
            code = not code
        elif not code and char in "([":
            depth += 1
        elif not code and char in ")]":
            depth = max(0, depth - 1)
        if not code and depth == 0 and char in separators:
            parts.append(current)
            current = ""
        else:
            current += char
    parts.append(current)
    return parts


def states(status: str) -> list[tuple[str, str]]:
    """Return the states a status opens with, each with the text that belongs to it.

    The states are the comma-separated items of the first sentence, as long as each starts with
    a word of the vocabulary; a kind of page (`design`, `template`) ends the list, because the
    words after it tell its story. Example: `states("normative, built (M1 to M8). Terms: x")`
    returns `[("normative", ""), ("built", "(M1 to M8)")]`, and `states("draft M0. Guide")`
    returns `[]`.
    """
    head = _split(status, ".;:")[0]
    found = []
    for item in _split(head, ","):
        word, _, rest = item.strip().partition(" ")
        if word not in VOCABULARY:
            break
        found.append((word, rest.strip()))
        if word in KINDS:
            break
    return found


def milestones(text: str) -> list[str]:
    """Return the milestones a text names, a ticket by its milestone and a range in full.

    Example: `milestones("(M1 to M3, M7-27, A1-06)")` returns `["A1", "M1", "M2", "M3", "M7"]`.
    """
    found = {m.group("milestone") for m in _REF.finditer(text)}
    for span in _RANGE.finditer(text):
        low, high = int(span.group("low")), int(span.group("high"))
        found |= {f"{span.group('letter')}{n}" for n in range(low, high + 1)}
    return sorted(found)


def roadmap_states(text: str) -> dict[str, str]:
    """Return the state of every milestone of the roadmap table, by its ID.

    Example: the row `| M1 Foundation | Toolchain | built |` gives `{"M1": "built"}`.
    """
    return {m.group("id"): m.group("state") for m in _ROW.finditer(text)}


def problems(status: str, roadmap: dict[str, str]) -> list[str]:
    """Return what is wrong with a status, one sentence per finding."""
    found = states(status)
    if not found:
        return [f"starts with none of {', '.join(VOCABULARY)}: {status[:60]!r}"]
    out = []
    for word, rest in found:
        if word not in (BUILT, PLANNED):
            continue
        named = milestones(rest)
        if not named:
            out.append(f"{word} names no milestone or ticket")
        for milestone in named:
            state = roadmap.get(milestone)
            if state is None:
                out.append(f"{word} names {milestone}, which the roadmap does not list")
            elif word == BUILT and not state.startswith(ALLOWS_BUILT):
                out.append(f"built names {milestone}, which the roadmap lists as {state!r}")
            elif word == PLANNED and state.startswith(FORBIDS_PLANNED):
                out.append(f"planned names {milestone}, which the roadmap lists as {state!r}")
    return out


def documents() -> list[str]:
    """Return the tracked documents whose status line is checked, as repository paths."""
    out = subprocess.run(
        ["git", "ls-files", "-z", "--", "docs/*.md"],
        cwd=REPO,
        capture_output=True,
        check=True,
        text=True,
    ).stdout
    pages = [p for p in out.split("\0") if p and not HISTORY.search(p)]
    return sorted(pages) + list(FOLDER_READMES)


def chapters() -> list[str]:
    """Return the chapters of the specification and its entry point, which need a status."""
    return [p for p in documents() if re.match(r"docs/spec/\d\d-[^/]+\.md$", p)] + [
        "docs/SPEC.md",
        "docs/PLAN.md",
        *FOLDER_READMES,
    ]


def test_the_status_paragraph_is_read() -> None:
    assert status_of("# T\n\nStatus: built (M7).\nGuide.\n\nText") == "built (M7). Guide."
    assert status_of("# T\n\nText without one.\n") is None
    late = "# T\n" + "\n" * TOP + "Status: built (M7).\n"
    assert status_of(late) is None


def test_the_leading_states_are_read() -> None:
    assert states("normative, built (M1 to M8). Terms: x") == [
        ("normative", ""),
        ("built", "(M1 to M8)"),
    ]
    assert states("built (M1-41; groups from the claim M1-42, section 7). Normative are") == [
        ("built", "(M1-41; groups from the claim M1-42, section 7)")
    ]
    assert states("built in M7-17. Rules: x") == [("built", "in M7-17")]
    assert states("design of 2026-11-06, revised after reviews: their") == [
        ("design", "of 2026-11-06")
    ]
    assert states("design of 2026-11-01 after a request, built by the ticket FEAT-x. Text") == [
        ("design", "of 2026-11-01 after a request")
    ]
    assert states("normative for `a.b`, planned (V2). Later") == [
        ("normative", "for `a.b`"),
        ("planned", "(V2)"),
    ]
    assert states("draft M0, no product code yet. This document") == []
    assert states("skeleton (M1-05) with configuration (M1-07)") == []


def test_milestones_are_read_from_tickets_and_ranges() -> None:
    assert milestones("(M1 to M3, M7-27, A1-06)") == ["A1", "M1", "M2", "M3", "M7"]
    assert milestones("(V2)") == ["V2"]
    assert milestones("of 2026-11-06 for FEAT-x") == []


def test_the_roadmap_table_is_read() -> None:
    table = (
        "| Milestone | Content | Status |\n|---|---|---|\n"
        "| M1 Foundation | Toolchain | built |\n"
        "| V1 Publication | Public | in progress: the name is open |\n"
        "| V2 Dev | Isolation ([chapter 21](spec/21.md)) | later |\n"
    )
    assert roadmap_states(table) == {
        "M1": "built",
        "V1": "in progress: the name is open",
        "V2": "later",
    }


def test_a_status_against_the_roadmap() -> None:
    roadmap = {"M0": "done", "M1": "built", "V1": "in progress: x", "V2": "later"}
    assert problems("normative, built (M0, M1). Text", roadmap) == []
    assert problems("normative, planned (V2). Text", roadmap) == []
    assert problems("built (V1-08), planned (V1-12). Text", roadmap) == []
    assert problems("design of 2026-11-06 for M9", roadmap) == []
    assert problems("draft M0, no product code yet. Text", roadmap) != []
    assert problems("M6-17. Rules", roadmap) != []
    assert problems("built. Text", roadmap) == ["built names no milestone or ticket"]
    assert "later" in problems("built (V2). Text", roadmap)[0]
    assert "built" in problems("planned (M1). Text", roadmap)[0]
    assert "does not list" in problems("built (M7). Text", roadmap)[0]


def test_the_history_is_left_as_written() -> None:
    for path in (
        "docs/briefs/M7-03-restore-test.md",
        "docs/decisions.d/M7-02.md",
        "docs/DECISIONS.md",
        "docs/adr/0001-adrs-in-madr-format-decisions-as-ruling-log.md",
    ):
        assert HISTORY.search(path), path
    for path in ("docs/SPEC.md", "docs/spec/16-operations.md", "docs/operations/upgrade.md"):
        assert not HISTORY.search(path), path


def test_the_roadmap_lists_the_milestones() -> None:
    roadmap = roadmap_states(ROADMAP.read_text(encoding="utf-8"))
    assert {"M0", "M1", "M8", "V1", "V2"} <= set(roadmap), sorted(roadmap)


def test_the_specification_explains_every_word() -> None:
    text = SPEC.read_text(encoding="utf-8")
    section = text.split("### 8.1 Status lines", 1)
    assert len(section) == 2, "docs/SPEC.md has no section 8.1 Status lines"
    rows = section[1].split("\n## ", 1)[0]
    missing = [word for word in VOCABULARY if f"| `{word}` |" not in rows]
    assert missing == [], f"docs/SPEC.md 8.1 explains no {missing}"


@pytest.mark.parametrize("path", chapters())
def test_every_chapter_has_a_status_line(path: str) -> None:
    assert status_of((REPO / path).read_text(encoding="utf-8")) is not None, path


def test_every_status_line_uses_the_vocabulary_and_agrees_with_the_roadmap() -> None:
    roadmap = roadmap_states(ROADMAP.read_text(encoding="utf-8"))
    found = []
    for path in documents():
        status = status_of((REPO / path).read_text(encoding="utf-8"))
        if status is not None:
            found += [f"{path}: {p}" for p in problems(status, roadmap)]
    assert found == [], "\n".join(found)
