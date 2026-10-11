# SPDX-License-Identifier: Apache-2.0
"""Every build brief quotes the owner's fixed build rules 1 to 5 byte for byte.

The canonical text stands once in docs/briefs/README.md between the markers
BEGIN/END FIXED BUILD RULES; a brief that rewords a rule fails here (owner build rules,
docs/briefs/README.md point 6). The block is a literal English translation of the owner's
rules (D-W-54).
"""

from __future__ import annotations

import re
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]
BRIEFS = REPO / "docs" / "briefs"
_BLOCK = re.compile(
    r"<!-- BEGIN FIXED BUILD RULES -->\n```text\n(?P<rules>.*?)```\n<!-- END FIXED BUILD RULES -->",
    re.DOTALL,
)


def canonical_rules() -> str:
    match = _BLOCK.search((BRIEFS / "README.md").read_text(encoding="utf-8"))
    assert match, "docs/briefs/README.md lacks the fixed build rules block"
    return match.group("rules")


def test_canonical_block_has_five_rules() -> None:
    rules = canonical_rules()
    assert [line[:2] for line in rules.splitlines() if line[:1].isdigit()] == [
        "1.",
        "2.",
        "3.",
        "4.",
        "5.",
    ]


@pytest.mark.parametrize("brief", sorted(BRIEFS.glob("M*.md")), ids=lambda p: p.name)
def test_brief_quotes_the_rules_verbatim(brief: Path) -> None:
    text = brief.read_text(encoding="utf-8")
    assert text.count(canonical_rules()) == 1, f"{brief.name}: fixed build rules differ"


def test_a_reworded_rule_is_detected() -> None:
    reworded = canonical_rules().replace("MkDocs", "the docs site", 1)
    assert canonical_rules() not in reworded


def test_block_translates_the_owner_wording() -> None:
    """The owner's rules name MkDocs and the stages; no paraphrase, no German word (K2-13)."""
    rules = canonical_rules()
    assert "Python API docs in MkDocs" in rules
    assert "stages are only the order" in rules
    for word in ("etappe", "Etappe", "docs site"):
        assert word not in rules
