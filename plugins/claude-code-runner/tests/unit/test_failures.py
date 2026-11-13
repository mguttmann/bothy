# SPDX-License-Identifier: Apache-2.0
"""The table of recognised CLI texts and the classification of a process end (09 18.9)."""

from __future__ import annotations

import dataclasses
import json
from pathlib import Path

import pytest

from claude_code_runner.account import ACCOUNT_ERRORS
from claude_code_runner.failures import (
    ERROR_TEXTS,
    ErrorKind,
    TurnEnd,
    TurnEvidence,
    api_error_text,
    classify_turn_end,
)
from claude_code_runner.heartbeat import PINNED_CLAUDE_CODE_VERSION

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
    by_member = 0
    for path in sorted(FIXTURES.glob("*.jsonl")) + sorted(FIXTURES.glob("*.txt")):
        meta = path.read_text().splitlines()[0]
        if '"error_text"' not in meta:
            continue
        fixture = json.loads(meta)
        kind = (fixture.get("members") or {}).get("error")
        if any(f in fixture["error_text"] for f in fragments):
            found += 1
        else:
            # An error the runner knows by the member ``error`` of the CLI's message, never
            # by its text: an account error (09 18.9), or the server error the scenarios use
            # as an error that is none.
            by_member += 1
            assert kind in {*ACCOUNT_ERRORS, "server_error"}, path.name
    assert found >= len(ErrorKind)
    assert by_member >= len(set(ACCOUNT_ERRORS.values())) - 1  # the login text is in the table


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
    """Lines the pinned CLI wrote against a local API answering 400, 401 and 400."""
    captured = CAPTURED / f"api_errors_{PINNED_CLAUDE_CODE_VERSION}.jsonl"
    lines = [json.loads(line) for line in captured.read_text("utf-8").splitlines()]
    errors = [m for m in lines if m["type"] == "assistant"]
    results = [m for m in lines if m["type"] == "result"]
    assert [m["api_error_status"] for m in results] == [400, 401, 400]
    too_old = api_error_text(errors[2]) or ""
    assert f"Claude Code {PINNED_CLAUDE_CODE_VERSION} does not support this model" in too_old
    # The scenario of the outdated CLI plays the captured text.
    fixture = json.loads((FIXTURES / "cli_too_old.jsonl").read_text("utf-8").splitlines()[0])
    assert fixture["error_text"] == too_old
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
    assert kinds == ["context_full", "not_authenticated", "runtime_version_unsupported"]


LIVE_TOO_OLD = (
    "API Error: 400 Claude Code 2.1.259 does not support this model; "
    "version 2.1.280 or newer is required"
)
POLICY_TOO_OLD = (
    "API Error: 400 Claude Code 2.1.321 is older than the minimum version required by your "
    "organization's policy"
)
# The third text of the too-old pattern, which the pinned CLI knows since 2.1.321.
ORGANIZATION_TOO_OLD = (
    "API Error: 400 Claude Code 2.1.330 or later is required for your organization"
)


def test_cli_too_old_for_the_model_is_actionable() -> None:
    """The API's answer to an outdated CLI names the version; the run says what to do."""
    result = {"type": "result", "subtype": "success", "is_error": True, "result": LIVE_TOO_OLD}
    end = classify_turn_end(
        TurnEvidence(init_seen=True, result=result, assistant_text=LIVE_TOO_OLD, exit_label="1")
    )
    assert end.outcome == "runtime_version_unsupported"
    assert end.required_version == "2.1.280"
    assert end.errors == (
        "Claude Code on the runner host is too old for the model: version 2.1.280 or newer "
        "is required. Update Claude Code (the runner reports runtime_version_unsupported "
        "until it is updated).",
    )


def test_cli_below_a_policy_floor_is_actionable() -> None:
    end = classify_turn_end(TurnEvidence(init_seen=True, assistant_text=POLICY_TOO_OLD))
    assert end.outcome == "runtime_version_unsupported"
    assert end.required_version is None
    assert end.errors == (
        "Claude Code on the runner host is older than the minimum version the organization's "
        "policy requires. Update Claude Code (the runner reports runtime_version_unsupported "
        "until it is updated).",
    )


def test_cli_below_an_organization_minimum_is_actionable() -> None:
    """The organization's minimum names its version; the run says what to do."""
    result = {"type": "result", "subtype": "success", "is_error": True}
    end = classify_turn_end(
        TurnEvidence(init_seen=True, result=result, assistant_text=ORGANIZATION_TOO_OLD)
    )
    assert end.outcome == "runtime_version_unsupported"
    assert end.required_version == "2.1.330"
    assert end.errors == (
        "Claude Code on the runner host is too old for the organization: version 2.1.330 or "
        "later is required. Update Claude Code (the runner reports runtime_version_unsupported "
        "until it is updated).",
    )
    unreadable = classify_turn_end(
        TurnEvidence(
            init_seen=True,
            assistant_text="Claude Code next or later is required for your organization",
        )
    )
    assert unreadable.outcome == "runtime_version_unsupported"
    assert unreadable.required_version is None
    # Only the API error channel counts: the model's own words never mark the CLI outdated.
    quoted = {"type": "result", "subtype": "error_during_execution", "result": ORGANIZATION_TOO_OLD}
    assert classify_turn_end(TurnEvidence(init_seen=True, result=quoted)).outcome == "runtime_error"


def test_quoted_too_old_text_is_not_the_cli_answer() -> None:
    """The model's own words never mark the CLI outdated: only the API error channel counts."""
    result = {"type": "result", "subtype": "error_during_execution", "result": LIVE_TOO_OLD}
    end = classify_turn_end(TurnEvidence(init_seen=True, result=result, stderr=LIVE_TOO_OLD))
    assert end.outcome == "runtime_error"
    assert end.required_version is None


# ---- A1-05: the cost cap of a hosted definition (26 5.8) --------------------------------------


def test_cost_cap_is_recognised_by_the_budget_result_or_the_cost() -> None:
    from claude_code_runner.failures import BUDGET_SUBTYPE, cost_cap_reached

    assert cost_cap_reached({"subtype": BUDGET_SUBTYPE}, 0.5)
    assert cost_cap_reached({"subtype": "error_during_execution", "total_cost_usd": 0.5}, 0.5)
    assert not cost_cap_reached({"subtype": "error_during_execution", "total_cost_usd": 0.4}, 0.5)
    assert not cost_cap_reached({"subtype": BUDGET_SUBTYPE}, None)
    assert not cost_cap_reached(None, 0.5)
    assert not cost_cap_reached({"total_cost_usd": True}, 0.5)
