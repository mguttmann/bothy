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
    # The repository rules of a path instruction (dashes, glossary) are not nits.
    assert "path-specific instruction" in text


def test_markdown_asks_for_sources_only_in_spec_security_and_adr() -> None:
    text = _instructions("**/*.md")
    for scope in ("docs/spec/**", "docs/security/**", "docs/adr/**"):
        assert scope in text
    assert "dashes" in text


def _config() -> dict[str, Any]:
    config = yaml.safe_load((REPO / ".coderabbit.yaml").read_text(encoding="utf-8"))
    assert isinstance(config, dict)
    return config


def test_learnings_are_stored_for_this_repository() -> None:
    # With opt_out the knowledge base drops every learning a reply teaches CodeRabbit.
    kb = _config()["knowledge_base"]
    assert kb["opt_out"] is False
    assert kb["learnings"]["scope"] == "local"
    # No private knowledge source reaches the reviews of this public repository.
    for source in ("jira", "linear", "mcp"):
        assert kb[source]["usage"] == "disabled", source


def test_sql_rule_covers_data_access_only() -> None:
    text = _instructions("services/**/*.go")
    assert "sqlc" in text
    for exempt in ("LISTEN/NOTIFY", "advisory locks"):
        assert exempt in text


def test_go_sum_is_excluded_on_purpose() -> None:
    assert "!**/go.sum" in _reviews()["path_filters"]
    assert "go.sum" in _instructions("**")


def test_default_theme_is_dark() -> None:
    assert "default theme is dark" in _instructions("**")
    tokens = (REPO / "docs" / "design" / "tokens.md").read_text(encoding="utf-8")
    assert "## Modifier `theme` (default: `dark`)" in tokens


def test_git_variables_are_cleared_by_the_pre_push_hook() -> None:
    assert "GIT_DIR" in _instructions("**")
    hooks = yaml.safe_load((REPO / ".pre-commit-config.yaml").read_text(encoding="utf-8"))
    entries = [
        h["entry"] for r in hooks["repos"] for h in r["hooks"] if "pre-push" in h.get("stages", [])
    ]
    assert any("env -u GIT_DIR " in e and e.endswith(" make check") for e in entries)


def test_test_names_and_reserved_addresses_are_fine_in_tests() -> None:
    text = _instructions("**")
    for word in ("test names", "loopback", "192.0.2.0/24", "2001:db8::/32"):
        assert word in text
