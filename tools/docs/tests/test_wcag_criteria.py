# SPDX-License-Identifier: Apache-2.0
"""The page of the WCAG 2.2 success criteria is complete and names only tests that exist.

docs/spec/01-product.md section 8 sets WCAG 2.2 level AA as the goal and maps it to rules by
topic. docs/quality/wcag-criteria.md lists every success criterion of the levels A and AA with
its state (`tested`, `partly`, `open`, `not applicable`), the automated evidence and what is
missing (docs/decisions.d/AUD-docs-spec-01.md). This test holds the page:

- its rows are exactly the criteria of the recommendation, each with its name and level
  (https://www.w3.org/TR/WCAG22/, retrieved 2026-11-12: 31 of level A, 24 of level AA; 4.1.1 is
  obsolete and removed there and has no row);
- a row of the state `tested` or `partly` names at least one test, a row of the state `partly`,
  `open` or `not applicable` says what is missing or why;
- every named test exists, by the rules of the accessibility checklists
  (test_accessibility_checklist.py, whose functions this test uses);
- the product chapter links the page.

So a criterion cannot be called tested by a test that was deleted or renamed, and a criterion
cannot be left out.
"""

from __future__ import annotations

import importlib.util
import re
from pathlib import Path
from types import ModuleType
from typing import NamedTuple

import pytest

ROOT = Path(__file__).resolve().parents[3]
PAGE = ROOT / "docs" / "quality" / "wcag-criteria.md"
PRODUCT = ROOT / "docs" / "spec" / "01-product.md"
STATES = ("tested", "partly", "open", "not applicable")
# States that need evidence, and states that need a text in the last column.
NEED_EVIDENCE = ("tested", "partly")
NEED_REASON = ("partly", "open", "not applicable")
# The success criteria of the levels A and AA of WCAG 2.2, in the order of the recommendation.
CRITERIA = (
    ("1.1.1", "Non-text Content", "A"),
    ("1.2.1", "Audio-only and Video-only (Prerecorded)", "A"),
    ("1.2.2", "Captions (Prerecorded)", "A"),
    ("1.2.3", "Audio Description or Media Alternative (Prerecorded)", "A"),
    ("1.2.4", "Captions (Live)", "AA"),
    ("1.2.5", "Audio Description (Prerecorded)", "AA"),
    ("1.3.1", "Info and Relationships", "A"),
    ("1.3.2", "Meaningful Sequence", "A"),
    ("1.3.3", "Sensory Characteristics", "A"),
    ("1.3.4", "Orientation", "AA"),
    ("1.3.5", "Identify Input Purpose", "AA"),
    ("1.4.1", "Use of Color", "A"),
    ("1.4.2", "Audio Control", "A"),
    ("1.4.3", "Contrast (Minimum)", "AA"),
    ("1.4.4", "Resize Text", "AA"),
    ("1.4.5", "Images of Text", "AA"),
    ("1.4.10", "Reflow", "AA"),
    ("1.4.11", "Non-text Contrast", "AA"),
    ("1.4.12", "Text Spacing", "AA"),
    ("1.4.13", "Content on Hover or Focus", "AA"),
    ("2.1.1", "Keyboard", "A"),
    ("2.1.2", "No Keyboard Trap", "A"),
    ("2.1.4", "Character Key Shortcuts", "A"),
    ("2.2.1", "Timing Adjustable", "A"),
    ("2.2.2", "Pause, Stop, Hide", "A"),
    ("2.3.1", "Three Flashes or Below Threshold", "A"),
    ("2.4.1", "Bypass Blocks", "A"),
    ("2.4.2", "Page Titled", "A"),
    ("2.4.3", "Focus Order", "A"),
    ("2.4.4", "Link Purpose (In Context)", "A"),
    ("2.4.5", "Multiple Ways", "AA"),
    ("2.4.6", "Headings and Labels", "AA"),
    ("2.4.7", "Focus Visible", "AA"),
    ("2.4.11", "Focus Not Obscured (Minimum)", "AA"),
    ("2.5.1", "Pointer Gestures", "A"),
    ("2.5.2", "Pointer Cancellation", "A"),
    ("2.5.3", "Label in Name", "A"),
    ("2.5.4", "Motion Actuation", "A"),
    ("2.5.7", "Dragging Movements", "AA"),
    ("2.5.8", "Target Size (Minimum)", "AA"),
    ("3.1.1", "Language of Page", "A"),
    ("3.1.2", "Language of Parts", "AA"),
    ("3.2.1", "On Focus", "A"),
    ("3.2.2", "On Input", "A"),
    ("3.2.3", "Consistent Navigation", "AA"),
    ("3.2.4", "Consistent Identification", "AA"),
    ("3.2.6", "Consistent Help", "A"),
    ("3.3.1", "Error Identification", "A"),
    ("3.3.2", "Labels or Instructions", "A"),
    ("3.3.3", "Error Suggestion", "AA"),
    ("3.3.4", "Error Prevention (Legal, Financial, Data)", "AA"),
    ("3.3.7", "Redundant Entry", "A"),
    ("3.3.8", "Accessible Authentication (Minimum)", "AA"),
    ("4.1.2", "Name, Role, Value", "A"),
    ("4.1.3", "Status Messages", "AA"),
)

_ROW = re.compile(
    r"^\| (?P<id>\d\.\d\.\d+) (?P<name>[^|]+?) \| (?P<level>A{1,2}) \| (?P<state>[^|]*?) \|"
    r"(?P<evidence>[^|]*)\|(?P<reason>[^|]*)\|$",
    re.MULTILINE,
)


class Row(NamedTuple):
    """A row of the page: a success criterion with its state."""

    id: str
    name: str
    level: str
    state: str
    evidence: str
    reason: str


def _checklist_rules() -> ModuleType:
    """Return the module of the checklist test, whose parsers this test shares."""
    path = Path(__file__).with_name("test_accessibility_checklist.py")
    spec = importlib.util.spec_from_file_location("accessibility_checklist_rules", path)
    assert spec is not None
    assert spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


RULES = _checklist_rules()


def rows(page: str) -> list[Row]:
    """Return the rows of the criteria tables of the page."""
    return [
        Row(
            m.group("id"),
            m.group("name").strip(),
            m.group("level"),
            m.group("state").strip(),
            m.group("evidence").strip(),
            m.group("reason").strip(),
        )
        for m in _ROW.finditer(page)
    ]


def row_problems(row: Row) -> list[str]:
    """Return what is wrong with the state, the evidence and the reason of a row."""
    if row.state not in STATES:
        return [f"{row.id}: the state {row.state!r} is none of {', '.join(STATES)}"]
    found = []
    named = RULES.evidence(row.evidence) + RULES.native_evidence(row.evidence)
    if row.state in NEED_EVIDENCE and not any(names for _, names in named):
        found.append(f"{row.id}: the state {row.state!r} names no test")
    if row.state not in NEED_EVIDENCE and row.evidence:
        found.append(f"{row.id}: the state {row.state!r} has evidence; it is `partly` or `tested`")
    if row.state in NEED_REASON and not row.reason:
        found.append(f"{row.id}: the state {row.state!r} says nothing in the last column")
    return found


def missing_tests(text: str) -> list[str]:
    """Return the tests a text names that do not exist, by the rules of the checklists."""
    suffixes = RULES.harness_suffixes()
    problems = []
    for path, names in RULES.evidence(text):
        files = RULES._candidates(path)
        if not files:
            problems.append(f"{path}: no such test file")
            continue
        patterns = [RULES.templates(f.read_text(encoding="utf-8")) for f in files]
        missing = min(
            ([n for n in names if not RULES.is_test_of(n, p, suffixes)] for p in patterns),
            key=len,
        )
        problems.extend(f"{path}: no test {name!r}" for name in missing)
    for path, names in RULES.native_evidence(text):
        file = ROOT / path
        if not file.is_file():
            problems.append(f"{path}: no such test file")
            continue
        methods = RULES.native_methods(file.read_text(encoding="utf-8"))
        problems += [f"{path}: no test {n!r}" for n in names if n not in methods]
    return problems


PAGE_SAMPLE = (
    "| Criterion | Level | State | Evidence | Open or reason |\n|---|---|---|---|---|\n"
    '| 1.1.1 Non-text Content | A | tested | Icons: `a/b_test.dart`: "one"; "two" | |\n'
    "| 1.2.4 Captions (Live) | AA | not applicable | | No live audio. |\n"
    '| 2.5.7 Dragging Movements | AA | partly | `c/d_test.dart`: "three" | A border. |\n'
)


def test_rows_are_read() -> None:
    assert rows(PAGE_SAMPLE) == [
        Row("1.1.1", "Non-text Content", "A", "tested", 'Icons: `a/b_test.dart`: "one"; "two"', ""),
        Row("1.2.4", "Captions (Live)", "AA", "not applicable", "", "No live audio."),
        Row("2.5.7", "Dragging Movements", "AA", "partly", '`c/d_test.dart`: "three"', "A border."),
    ]


def test_the_list_is_the_one_of_the_recommendation() -> None:
    assert len(CRITERIA) == len({c[0] for c in CRITERIA})
    assert [c[2] for c in CRITERIA].count("A") == 31
    assert [c[2] for c in CRITERIA].count("AA") == 24
    assert "4.1.1" not in {c[0] for c in CRITERIA}


def test_a_row_needs_its_evidence_and_its_reason() -> None:
    tested, absent, partly = rows(PAGE_SAMPLE)
    assert row_problems(tested) == []
    assert row_problems(absent) == []
    assert row_problems(partly) == []
    assert "names no test" in row_problems(tested._replace(evidence="Icons have names."))[0]
    assert "names no test" in row_problems(tested._replace(evidence="`a/b_test.dart`"))[0]
    assert "has evidence" in row_problems(absent._replace(evidence='`a/b_test.dart`: "one"'))[0]
    assert "says nothing" in row_problems(absent._replace(reason=""))[0]
    assert "says nothing" in row_problems(partly._replace(reason=""))[0]
    assert "is none of" in row_problems(tested._replace(state="met"))[0]


def test_a_missing_test_is_reported() -> None:
    real = '`tools/docs/tests/data/no_such_test.dart`: "x"'
    assert missing_tests(real) == ["tools/docs/tests/data/no_such_test.dart: no such test file"]
    named = (
        "`packages/design_system/test/ds_icon_test.dart`: "
        '"a decorative icon is excluded from semantics"; "a test that was deleted"'
    )
    assert missing_tests(named) == [
        "packages/design_system/test/ds_icon_test.dart: no test 'a test that was deleted'"
    ]
    native = '`apps/client/ios/RunnerUITests/AccessibilityAuditTests.swift`: "testNoSuchScreen"'
    assert missing_tests(native) == [
        "apps/client/ios/RunnerUITests/AccessibilityAuditTests.swift: no test 'testNoSuchScreen'"
    ]


def test_the_page_lists_exactly_the_criteria_of_the_recommendation() -> None:
    found = rows(PAGE.read_text(encoding="utf-8"))
    assert [(r.id, r.name, r.level) for r in found] == list(CRITERIA)


def test_every_row_has_its_evidence_and_its_reason() -> None:
    problems = [p for row in rows(PAGE.read_text(encoding="utf-8")) for p in row_problems(row)]
    assert problems == [], "\n".join(problems)


def test_every_named_test_exists() -> None:
    problems = missing_tests(PAGE.read_text(encoding="utf-8"))
    assert problems == [], "\n".join(problems)


def test_the_states_are_explained_and_the_product_chapter_links_the_page() -> None:
    page = PAGE.read_text(encoding="utf-8")
    missing = [state for state in STATES if f"| `{state}` |" not in page]
    assert missing == [], f"the page explains no {missing}"
    section = PRODUCT.read_text(encoding="utf-8").split("\n## 8. Accessibility", 1)[1]
    assert "quality/wcag-criteria.md" in section.split("\n## 9. ", 1)[0]


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(pytest.main([__file__]))
