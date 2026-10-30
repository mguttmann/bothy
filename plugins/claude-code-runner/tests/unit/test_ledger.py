# SPDX-License-Identifier: Apache-2.0
"""The turn ledger: the recovery invariants of brief design note 12 f, one rule each."""

from __future__ import annotations

import dataclasses
from typing import Any

import pytest

from claude_code_runner.ledger import TurnLedger


def echo(line_uuid: str) -> dict[str, Any]:
    return {"type": "user", "isReplay": True, "uuid": line_uuid}


def result(*named: str, error: bool = False) -> dict[str, Any]:
    message: dict[str, Any] = {"type": "result", "subtype": "success", "is_error": error}
    if named:
        message["user_message_uuids"] = list(named)
        message["user_message_uuid"] = named[-1]
    return message


API_ERROR = {
    "type": "assistant",
    "is_api_error_message": True,
    "parent_tool_use_id": None,
    "message": {"content": [{"type": "text", "text": "Prompt is too long"}]},
}


def feed(ledger: TurnLedger, *messages: dict[str, Any], start: int = 0) -> int:
    """Feed messages as lines of 10 bytes each from ``start``; returns the offset after them."""
    offset = start
    for message in messages:
        ledger.line(message, offset, offset + 10)
        offset += 10
    return offset


def ledger_with(*inputs: str) -> TurnLedger:
    ledger = TurnLedger.begin("turn", 0)
    for line_uuid in inputs:
        ledger.written(line_uuid)
    return ledger


# ---- rule 3: every input by its own id ---------------------------------------------------------


def test_an_input_is_answered_when_a_success_result_names_it() -> None:
    ledger = ledger_with("p", "f1", "f2")
    feed(ledger, result("p", "f1"))
    assert ledger.unanswered() == ["f2"]


def test_a_folded_input_is_answered_by_the_result_after_its_echo_that_names_nothing() -> None:
    # 2.1.259 folds a queued line into a notification turn: echo, then a result without names.
    ledger = ledger_with("p", "f1", "f2")
    feed(ledger, echo("p"), result("p"), echo("f1"), result())
    assert ledger.unanswered() == ["f2"]


def test_a_queued_input_is_never_answered_by_the_result_of_another_turn() -> None:
    # A notification turn ends while f1 is still queued (no echo yet): f1 stays open.
    ledger = ledger_with("p", "f1")
    feed(ledger, echo("p"), result("p"), result())
    assert ledger.unanswered() == ["f1"]


def test_an_error_result_answers_nothing_and_there_is_no_positional_rule() -> None:
    ledger = ledger_with("p", "f1")
    feed(ledger, echo("p"), echo("f1"), API_ERROR, result("p", "f1", error=True))
    assert ledger.unanswered() == ["p", "f1"]
    other = ledger_with("p", "f1")
    feed(other, result())  # names nothing and no echo came: nothing is answered
    assert other.unanswered() == ["p", "f1"]


def test_a_taken_input_without_its_result_spoils_the_last_success() -> None:
    # The CLI took f1 after the prompt's result and died in f1's turn: no result for f1.
    ledger = ledger_with("p", "f1")
    feed(ledger, echo("p"), result("p"), echo("f1"))
    assert ledger.end_result() is None
    feed(ledger, result("f1"), start=30)
    assert ledger.end_result() == ledger.last_result
    failed = ledger_with("p")
    feed(failed, echo("p"), result("p", error=True))
    assert failed.end_result() == failed.last_result  # an error result stays the evidence


# ---- rule 1: the offset ------------------------------------------------------------------------


def test_the_offset_never_passes_the_last_result_while_the_run_is_open() -> None:
    ledger = ledger_with("p")
    end = feed(ledger, echo("p"), result("p"), {"type": "system", "subtype": "status"})
    assert ledger.checkpoint(end) == 10  # the result's line, not the end of the file
    later = feed(ledger, echo("f"), result(), start=end)
    assert ledger.checkpoint(later) == 40  # the newer result is the end evidence now


def test_the_offset_holds_at_the_api_error_before_an_error_result() -> None:
    ledger = ledger_with("p")
    end = feed(ledger, echo("p"), API_ERROR)
    assert ledger.checkpoint(end) == 10
    end = feed(ledger, result("p", error=True), start=end)
    assert ledger.checkpoint(end) == 10
    assert ledger.api_error == "Prompt is too long"


def test_the_offset_never_moves_back_and_moves_freely_before_a_result() -> None:
    ledger = ledger_with("p")
    assert ledger.checkpoint(feed(ledger, echo("p"))) == 10
    assert ledger.checkpoint(5) == 10


# ---- rule 2: answers ---------------------------------------------------------------------------


def test_an_answer_is_planned_once_and_survives_the_record() -> None:
    ledger = ledger_with("p")
    planned = ledger.plan_answer(20, may_title=True)
    assert planned.first
    assert not planned.sent
    assert ledger.plan_answer(20, may_title=True) == planned
    restored = TurnLedger.from_record(ledger.to_record())
    assert restored.planned(20) == planned
    restored.answer_sent(20)
    again = TurnLedger.from_record(restored.to_record())
    assert again.planned(20) == dataclasses.replace(planned, sent=True)
    assert again.plan_answer(40, may_title=True).first is False


# ---- rule 4: kind ------------------------------------------------------------------------------


@pytest.mark.parametrize("kind", ["turn", "compaction"])
def test_the_kind_survives_the_record(kind: str) -> None:
    ledger = TurnLedger.begin(kind, 7)
    assert TurnLedger.from_record(ledger.to_record()).kind == kind


def test_an_unknown_kind_reads_as_a_compaction_and_bad_entries_are_skipped() -> None:
    record = {"kind": "other", "offset": -1, "answers": [{"line": "x"}], "inputs": [3]}
    ledger = TurnLedger.from_record(record)
    assert (ledger.kind, ledger.offset, ledger.answers, ledger.unanswered()) == (
        "compaction",
        0,
        {},
        [],
    )
    with pytest.raises(ValueError, match="other"):
        TurnLedger.begin("other", 0)


def test_finishing_keeps_its_summary() -> None:
    """RUNNER-NO-GIT: an adopted end repeats the summary that names the cause."""
    from claude_code_runner.ledger import TurnLedger

    record = {"finishing": {"state": "failed", "reason": "adapter_error", "summary": "Why"}}
    assert TurnLedger.from_record(record).finishing == {
        "state": "failed",
        "reason": "adapter_error",
        "summary": "Why",
    }
    old = {"finishing": {"state": "failed", "reason": "adapter_error"}}
    assert TurnLedger.from_record(old).finishing == {
        "state": "failed",
        "reason": "adapter_error",
        "summary": None,
    }
