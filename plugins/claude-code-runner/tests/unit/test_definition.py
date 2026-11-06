# SPDX-License-Identifier: Apache-2.0
"""The runner's own check of a definition against the shared corpus (26 4.2, 4.3, 5.6 item 1).

Every file of ``tests/agent-definitions`` at the repository root gives exactly its expected
findings (pointer and field code, order free) in `claude_code_runner.definition.check`, run with
the limits of the corpus' ``limits.json``, like the server's validator (A1-02).
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import pytest

from claude_code_runner.definition import (
    Finding,
    Limits,
    check,
    has_frontmatter,
    has_import_outside_folder,
    has_inline_shell,
    intersect,
    parse,
    required_class,
    validate,
    validate_value,
)

CORPUS = Path(__file__).resolve().parents[4] / "tests" / "agent-definitions"
FILES = sorted(p for p in CORPUS.rglob("*.json") if p.name != "limits.json")


def corpus_limits() -> Limits:
    """The limits of ``limits.json`` by configuration key."""
    raw = json.loads((CORPUS / "limits.json").read_text(encoding="utf-8"))
    return Limits.from_keys(raw)


def _fixture(path: Path) -> dict[str, Any]:
    data: dict[str, Any] = json.loads(path.read_text(encoding="utf-8"))
    return data


def test_the_corpus_has_valid_and_invalid_files() -> None:
    assert any(p.parent.name == "valid" for p in FILES)
    assert sum(p.parent.name == "invalid" for p in FILES) >= 30


@pytest.mark.parametrize("path", FILES, ids=lambda p: f"{p.parent.name}/{p.stem}")
def test_every_fixture_gives_its_expected_findings(path: Path) -> None:
    fixture = _fixture(path)
    raw = json.dumps(fixture["definition"]).encode("utf-8")
    found = check(raw, corpus_limits(), fixture["tool_class"])
    want = sorted((e["pointer"], e["code"]) for e in fixture["expect"])
    assert sorted((f.pointer, f.code) for f in found) == want


def test_limits_know_every_key_of_the_corpus() -> None:
    raw = json.loads((CORPUS / "limits.json").read_text(encoding="utf-8"))
    with pytest.raises(ValueError, match="unknown"):
        Limits.from_keys({**raw, "agents.definition.other": 1})
    with pytest.raises(ValueError, match="missing"):
        Limits.from_keys({k: v for k, v in raw.items() if k != "agents.mcp_stdio_runners"})


def test_the_runner_limits_skip_only_the_server_s_size_bounds() -> None:
    """The runner cannot read the server's size keys; every other rule holds without them."""
    many = {"name": "x", "description": "d", "body": "b"}
    definition = {
        "instructions": "x" * 70000,
        "skills": [{**many, "name": f"s{i}"} for i in range(40)],
        "mcp_servers": [],
        "settings": {"tool_class_wish": "chat", "max_cost_usd_per_run": 99},
        "origin": {"kind": "manual"},
    }
    raw = json.dumps(definition).encode()
    assert check(raw, Limits.runner(("npx",)), "chat") == []
    assert {f.code for f in check(raw, corpus_limits(), "chat")} == {
        "too_long",
        "too_many",
        "too_large",
    }


@pytest.mark.parametrize(
    ("raw", "code"),
    [
        (b"{", "invalid_type"),
        (b"{} {}", "invalid_type"),
        (b'{"instructions": NaN}', "invalid_type"),
        (b"\xff", "invalid_type"),
    ],
)
def test_text_that_is_not_one_json_value_is_one_root_finding(raw: bytes, code: str) -> None:
    assert check(raw, corpus_limits(), "host") == [Finding("", code)]


def test_a_number_out_of_range_and_a_boolean_cost_are_refused() -> None:
    base = {
        "instructions": "",
        "skills": [],
        "mcp_servers": [],
        "origin": {"kind": "manual"},
    }
    huge = json.dumps({**base, "settings": {"tool_class_wish": "chat"}})
    huge = huge.replace('"chat"}', '"chat", "max_cost_usd_per_run": 1e999}')
    assert check(huge.encode(), Limits.runner(()), "chat") == [
        Finding("/settings/max_cost_usd_per_run", "invalid")
    ]
    flag = {**base, "settings": {"tool_class_wish": "chat", "max_cost_usd_per_run": True}}
    assert check(json.dumps(flag).encode(), Limits.runner(()), "chat") == [
        Finding("/settings/max_cost_usd_per_run", "invalid_type")
    ]


def test_a_lone_surrogate_is_invalid_utf8() -> None:
    raw = b'{"instructions": "a\\ud800", "skills": [], "mcp_servers": [], '
    raw += b'"settings": {"tool_class_wish": "chat"}, "origin": {"kind": "manual"}}'
    assert check(raw, Limits.runner(()), "chat") == [Finding("/instructions", "invalid_utf8")]


def test_parse_returns_the_typed_definition_of_a_passing_fixture() -> None:
    fixture = _fixture(CORPUS / "valid" / "full-host.json")
    raw = json.dumps(fixture["definition"]).encode()
    definition, found = parse(raw, corpus_limits(), "host")
    assert found == []
    assert definition is not None
    assert [s.name for s in definition.skills] == ["notes", "summarize-2"]
    assert [f.name for f in definition.skills[0].files] == ["checklist.md", "style-guide.v2.md"]
    docs, files, search = definition.mcp_servers
    assert docs.transport == "http"
    assert docs.headers[0].secret_ref == "env:DOCS_MCP_TOKEN"
    assert docs.headers[1].value == "release"
    assert files.command == "npx"
    assert files.package == "@example/mcp-files@1.4.2"
    assert files.args == ("--root", "/srv/shared", "--read-only")
    assert search.args == ()
    assert definition.settings.model == "claude-sonnet-4-5"
    assert definition.settings.allowed_tools == (
        "Read",
        "Grep",
        "Bash(git log:*)",
        "mcp__docs__search",
        "mcp__files",
    )
    assert definition.settings.max_cost_usd_per_run == 2.5
    assert required_class(definition) == "host"


def test_parse_gives_no_definition_with_a_finding() -> None:
    fixture = _fixture(CORPUS / "valid" / "full-host.json")
    raw = json.dumps(fixture["definition"]).encode()
    definition, found = parse(raw, corpus_limits(), "scoped")
    assert definition is None
    assert {f.code for f in found} == {"tool_class_too_low"}


def test_required_class_without_a_stdio_server_is_chat() -> None:
    fixture = _fixture(CORPUS / "valid" / "scoped-http.json")
    definition, _ = parse(json.dumps(fixture["definition"]).encode(), corpus_limits(), "scoped")
    assert definition is not None
    assert required_class(definition) == "chat"


def test_intersection_keeps_the_order_of_the_runner_key() -> None:
    configured = ("Read", "Grep", "Bash(git log:*)", "Edit")
    assert intersect(configured, ("Edit", "Read", "WebFetch")) == ["Read", "Edit"]
    assert intersect(configured, None) == list(configured)
    assert intersect(configured, ()) == []


def test_validate_and_validate_value_give_the_findings_of_check() -> None:
    fixture = _fixture(CORPUS / "valid" / "full-host.json")
    text = json.dumps(fixture["definition"])
    assert validate(text, corpus_limits(), "host") == []
    assert validate_value(fixture["definition"], corpus_limits(), "scoped") == check(
        text.encode(), corpus_limits(), "scoped"
    )
    assert validate("{", corpus_limits(), "chat") == [Finding("", "invalid_type")]


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
