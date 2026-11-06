# SPDX-License-Identifier: Apache-2.0
"""The runner's copy of the definition rules (docs/spec/26-builtin-agents.md 4.2, 4.3).

The shared corpus ``tests/agent-definitions`` at the repository root is the conformance set of
every writer (A1-02): each fixture must give exactly its expected findings, as with the server's
Go validator.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import pytest

from claude_code_runner.definition import (
    Limits,
    has_frontmatter,
    has_import_outside_folder,
    has_inline_shell,
    validate,
)

CORPUS = Path(__file__).resolve().parents[4] / "tests" / "agent-definitions"


def _limits() -> Limits:
    raw = json.loads((CORPUS / "limits.json").read_text(encoding="utf-8"))
    return Limits.from_keys(raw)


def _fixtures() -> list[Path]:
    return sorted((CORPUS / "valid").glob("*.json")) + sorted((CORPUS / "invalid").glob("*.json"))


def test_the_corpus_is_there() -> None:
    assert len(list((CORPUS / "valid").glob("*.json"))) > 0
    assert len(list((CORPUS / "invalid").glob("*.json"))) > 0


@pytest.mark.parametrize("path", _fixtures(), ids=lambda p: f"{p.parent.name}/{p.name}")
def test_corpus(path: Path) -> None:
    fixture: dict[str, Any] = json.loads(path.read_text(encoding="utf-8"))
    raw = json.dumps(fixture["definition"], ensure_ascii=False)
    got = sorted(f"{f.pointer} {f.code}" for f in validate(raw, _limits(), fixture["tool_class"]))
    want = sorted(f"{e['pointer']} {e['code']}" for e in fixture["expect"])
    assert got == want


def test_the_limits_need_every_key() -> None:
    raw = json.loads((CORPUS / "limits.json").read_text(encoding="utf-8"))
    del raw["agents.definition.max_skills"]
    with pytest.raises(KeyError):
        Limits.from_keys(raw)


@pytest.mark.parametrize(
    ("text", "found"),
    [
        ("---\nhooks: {}\n---", True),
        ("\ufeff---  \r\nx", True),
        ("text\n---", False),
        ("----", False),
    ],
)
def test_frontmatter(text: str, found: bool) -> None:
    assert has_frontmatter(text) is found


@pytest.mark.parametrize(
    ("text", "found"),
    [
        ("!`id`", True),
        ("run\u00a0!`id`", True),
        ("a!`id`", False),
        ("```!\nid\n```", True),
        ("```\n!x\n```", False),
        ("!``", False),
    ],
)
def test_inline_shell(text: str, found: bool) -> None:
    assert has_inline_shell(text) is found


@pytest.mark.parametrize(
    ("text", "found"),
    [
        ("@/etc/passwd", True),
        ("see @~/x", True),
        ("see @a/../../b", True),
        ("see @x\\ /../../etc/passwd", True),
        ("see @C:/x", True),
        ("see @notes.md", False),
        ("mail alex@example.com", False),
        ("see @a/b#../x", False),
    ],
)
def test_imports(text: str, found: bool) -> None:
    assert has_import_outside_folder(text) is found


def test_not_one_json_value() -> None:
    for raw in ("", "{} {}", "[", "nul"):
        assert [(f.pointer, f.code) for f in validate(raw, _limits(), "chat")] == [
            ("", "invalid_type")
        ]
