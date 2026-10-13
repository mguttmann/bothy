# SPDX-License-Identifier: Apache-2.0
"""CodeRabbit reports real problems only (docs/DECISIONS.md, CRT-1).

Only Blockers and reproduced Majors block a merge; style and wording nits cost review
rounds without changing behavior. These tests read the real `.coderabbit.yaml`.
"""

from __future__ import annotations

from pathlib import Path
from typing import Any

import yaml

REPO = Path(__file__).resolve().parents[3]


def _reviews() -> dict[str, Any]:
    config = yaml.safe_load((REPO / ".coderabbit.yaml").read_text(encoding="utf-8"))
    assert isinstance(config, dict)
    reviews = config["reviews"]
    assert isinstance(reviews, dict)
    return reviews


def _instructions(path: str) -> str:
    [entry] = [e for e in _reviews()["path_instructions"] if e["path"] == path]
    text = entry["instructions"]
    assert isinstance(text, str)
    return text


def test_profile_is_chill() -> None:
    assert _reviews()["profile"] == "chill"


def test_general_instruction_skips_nits() -> None:
    text = _instructions("**")
    for word in ("bugs", "security", "contracts", "CI"):
        assert word in text
    assert "nits" in text


def test_markdown_asks_for_sources_only_in_spec_security_and_adr() -> None:
    text = _instructions("**/*.md")
    for scope in ("docs/spec/**", "docs/security/**", "docs/adr/**"):
        assert scope in text
    assert "dashes" in text
