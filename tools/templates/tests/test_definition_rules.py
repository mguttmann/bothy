# SPDX-License-Identifier: Apache-2.0
"""The Python rules of 26 4.3 give exactly the findings of the shared corpus.

tests/agent-definitions is the conformance set of every writer (docs/decisions.d/A1-02.md
D-A1-02-8): the Go validator of the server runs it in corpus_test.go, and this test runs the same
files through tools/templates/definition_rules.py, so the template gate and the server refuse the
same definitions.
"""

from __future__ import annotations

import json
from pathlib import Path

import definition_rules as rules
import pytest

REPO = Path(__file__).resolve().parents[3]
CORPUS = REPO / "tests" / "agent-definitions"
FIXTURES = sorted((CORPUS / "valid").glob("*.json")) + sorted((CORPUS / "invalid").glob("*.json"))


def _limits() -> rules.Limits:
    return rules.Limits.from_keys(json.loads((CORPUS / "limits.json").read_text(encoding="utf-8")))


@pytest.mark.parametrize("path", FIXTURES, ids=lambda p: f"{p.parent.name}/{p.name}")
def test_corpus_fixture_gives_exactly_its_findings(path: Path) -> None:
    fixture = json.loads(path.read_text(encoding="utf-8"))
    assert set(fixture) == {"tool_class", "definition", "expect"}
    got = rules.validate(fixture["definition"], _limits(), fixture["tool_class"])
    want = sorted(f"{e['pointer']} {e['code']}" for e in fixture["expect"])
    assert sorted(f"{f.pointer} {f.code}" for f in got) == want


def test_the_corpus_has_valid_and_invalid_fixtures() -> None:
    kinds = {p.parent.name for p in FIXTURES}
    assert kinds == {"valid", "invalid"}


def test_limits_need_every_key_and_no_other() -> None:
    keys = json.loads((CORPUS / "limits.json").read_text(encoding="utf-8"))
    with pytest.raises(ValueError, match=r"agents\.mcp_stdio_runners"):
        rules.Limits.from_keys({k: v for k, v in keys.items() if k != "agents.mcp_stdio_runners"})
    with pytest.raises(ValueError, match="unknown"):
        rules.Limits.from_keys({**keys, "agents.unknown": 1})


def test_validate_text_reads_one_json_value() -> None:
    assert [(f.pointer, f.code) for f in rules.validate_text("{} []", _limits(), "chat")] == [
        ("", "invalid_type")
    ]
    assert [(f.pointer, f.code) for f in rules.validate_text("NaN", _limits(), "chat")] == [
        ("", "invalid_type")
    ]


@pytest.mark.parametrize(
    ("text", "outside"),
    [
        ("@/etc/x", True),
        ("see @~/x", True),
        ("@a/../b", True),
        ("@C:/x", True),
        ('@"/x"', True),
        ("@a\\ /../../b", True),
        ("@docs/a.md#..", False),
        ("mail alex@example.com", False),
        ("@./notes.md", False),
    ],
)
def test_imports_outside_the_folder(text: str, outside: bool) -> None:
    assert rules.has_import_outside_folder(text) is outside


@pytest.mark.parametrize(
    ("text", "shell"),
    [
        ("!`date`", True),
        ("run\u3000!`date`", True),
        ("x!`date`", False),
        ("```!\nls\n```", True),
        ("!``", False),
    ],
)
def test_inline_shell(text: str, shell: bool) -> None:
    assert rules.has_inline_shell(text) is shell


@pytest.mark.parametrize(
    ("url", "ok"),
    [
        ("https://mcp.example.com/v1?x=1", True),
        ("HTTPS://mcp.example.com:8443", True),
        ("https://mcp.example.com:x", False),
        ("https:mcp.example.com", False),
        ("https://a b.example.com", False),
        ("https://mcp.example.com/%zz", False),
        ("https://[2001:db8::1]/", True),
        ("https://mcp.example.com/a\u0007", False),
        ("https://ex_ample.com", True),
        ("https://a.example.com?q=%zz", True),
        ("https://:443", False),
        ("https://h%41.example.com", False),
    ],
)
def test_urls(url: str, ok: bool) -> None:
    assert rules.valid_url(url) is ok


def test_cost_outside_a_float_is_invalid() -> None:
    lim = _limits()
    base = {
        "instructions": "",
        "skills": [],
        "mcp_servers": [],
        "settings": {"tool_class_wish": "chat", "max_cost_usd_per_run": 10**400},
        "origin": {"kind": "manual"},
    }
    got = rules.validate(base, lim, "chat")
    assert [(f.pointer, f.code) for f in got] == [("/settings/max_cost_usd_per_run", "invalid")]
    base["settings"] = {"tool_class_wish": "chat", "max_cost_usd_per_run": True}
    got = rules.validate(base, lim, "chat")
    assert [(f.pointer, f.code) for f in got] == [
        ("/settings/max_cost_usd_per_run", "invalid_type")
    ]


def test_a_lone_surrogate_reads_as_a_replacement_character() -> None:
    lim = _limits()
    raw = (
        '{"instructions": "a\\ud800b", "skills": [], "mcp_servers": [],'
        ' "settings": {"tool_class_wish": "chat"}, "origin": {"kind": "manual"}}'
    )
    assert rules.validate_text(raw, lim, "chat") == []
