# SPDX-License-Identifier: Apache-2.0
"""The table of recognised CLI texts and the classification of a process end (09 18.9)."""

from __future__ import annotations

import dataclasses
import json
from pathlib import Path

import pytest

from claude_code_runner.failures import (
    ERROR_TEXTS,
    ErrorKind,
    TurnEnd,
    TurnEvidence,
    api_error_text,
    classify_turn_end,
)

FIXTURES = Path(__file__).resolve().parents[1] / "fake_claude" / "fixtures"
# Lines the real CLI wrote, unchanged (not replayed by the fake).
CAPTURED = Path(__file__).resolve().parents[1] / "real_cli" / "captured"
CANARY = "stderr-canary-7f3a"


def fragment(kind: ErrorKind) -> str:
    return next(e.fragment for e in ERROR_TEXTS if e.kind is kind)


def test_error_text_table_is_complete() -> None:
    assert {e.kind for e in ERROR_TEXTS} == set(ErrorKind)
    fragments = [e.fragment for e in ERROR_TEXTS]
    assert all(f.strip() for f in fragments)
    assert len(set(fragments)) == len(fragments)
    for entry in ERROR_TEXTS:
        assert entry.channel in ("result", "assistant", "stderr")
        assert entry.source.strip()


ERROR = {"type": "result", "subtype": "error_during_execution", "is_error": True}
OK = {"type": "result", "subtype": "success", "is_error": False, "result": "Done."}

CASES = [
    ("stop with an error result", TurnEvidence(stop_requested=True, result=ERROR), "stopped"),
    ("success", TurnEvidence(init_seen=True, result=OK), "completed"),
    (
        "success flagged as error",
        TurnEvidence(init_seen=True, result={**OK, "is_error": True}),
        "runtime_error",
    ),
    (
        "context full",
        TurnEvidence(
            init_seen=True,
            result=ERROR,
            assistant_text=fragment(ErrorKind.CONTEXT_FULL),
        ),
        "context_full",
    ),
    (
        "session missing before init",
        TurnEvidence(stderr=f"{fragment(ErrorKind.SESSION_MISSING)}: x", exit_label="1"),
        "session_missing",
    ),
    (
        "session missing after init",
        TurnEvidence(init_seen=True, stderr=fragment(ErrorKind.SESSION_MISSING)),
        "runtime_error",
    ),
    (
        "session exists before init",
        TurnEvidence(stderr=f"Error: Session ID x {fragment(ErrorKind.SESSION_EXISTS)}."),
        "session_exists",
    ),
    (
        "not authenticated in the assistant text",
        TurnEvidence(init_seen=True, result=ERROR, assistant_text="Not logged in · Run"),
        "not_authenticated",
    ),
]


@pytest.mark.parametrize(("name", "evidence", "outcome"), CASES, ids=[c[0] for c in CASES])
def test_classify_turn_end(name: str, evidence: TurnEvidence, outcome: str) -> None:
    assert classify_turn_end(evidence).outcome == outcome, name


def test_classify_unknown_texts_and_signals() -> None:
    texts = {**ERROR, "errors": ["upstream refused", "second text"]}
    end = classify_turn_end(TurnEvidence(init_seen=True, result=texts, exit_label="1"))
    assert end == TurnEnd("runtime_error", ("upstream refused", "second text"), "1")
    killed = classify_turn_end(TurnEvidence(init_seen=True, exit_label="SIGKILL"))
    assert killed == TurnEnd("runtime_error", (), "SIGKILL")
    both = TurnEvidence(
        stderr=fragment(ErrorKind.SESSION_EXISTS),
        assistant_text=fragment(ErrorKind.CONTEXT_FULL),
    )
    assert classify_turn_end(both).outcome == "context_full"  # first in table order


def test_stderr_text_never_in_turn_end() -> None:
    for evidence in (
        TurnEvidence(stderr=CANARY, exit_label="1"),
        TurnEvidence(init_seen=True, result=ERROR, stderr=CANARY),
        TurnEvidence(stderr=f"{fragment(ErrorKind.SESSION_MISSING)} {CANARY}"),
    ):
        end = classify_turn_end(evidence)
        assert CANARY not in json.dumps(dataclasses.asdict(end))


def test_fixture_texts_are_in_table() -> None:
    fragments = {e.fragment for e in ERROR_TEXTS}
    found = 0
    for path in sorted(FIXTURES.glob("*.jsonl")) + sorted(FIXTURES.glob("*.txt")):
        meta = path.read_text().splitlines()[0]
        if '"error_text"' not in meta:
            continue
        found += 1
        text = json.loads(meta)["error_text"]
        assert any(f in text for f in fragments), path.name
    assert found >= len(ErrorKind)


def test_only_the_cli_api_error_is_the_assistant_channel() -> None:
    quote = {
        "type": "assistant",
        "message": {"content": [{"type": "text", "text": "The log said: Prompt is too long"}]},
    }
    error = {**quote, "is_api_error_message": True}
    assert api_error_text(quote) is None
    assert api_error_text({**quote, "isApiErrorMessage": True}) is None  # the in-memory name
    assert api_error_text({"type": "user", "is_api_error_message": True}) is None
    assert api_error_text({**error, "parent_tool_use_id": "toolu_1"}) is None  # a subagent's
    assert api_error_text(error) == "The log said: Prompt is too long"


def test_captured_cli_api_errors_classify() -> None:
    """Lines the pinned CLI 2.1.259 wrote against a local API answering 400 and 401."""
    lines = [
        json.loads(line)
        for line in (CAPTURED / "api_errors_2.1.259.jsonl").read_text("utf-8").splitlines()
    ]
    errors = [m for m in lines if m["type"] == "assistant"]
    results = [m for m in lines if m["type"] == "result"]
    kinds = [
        classify_turn_end(
            TurnEvidence(
                init_seen=True,
                result=result,
                assistant_text=api_error_text(error) or "",
                exit_label="1",
            )
        ).outcome
        for error, result in zip(errors, results, strict=True)
    ]
    assert kinds == ["context_full", "not_authenticated"]
