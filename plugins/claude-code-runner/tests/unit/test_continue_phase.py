# SPDX-License-Identifier: Apache-2.0
"""A run taken over by the next runner (docs/spec/09-agents.md 18.19.6).

No CLI ran for a run taken over in phase ``transcribing``, so the adopter starts it over after
the launch check. A stop that came first (``stop --turns`` or a cancel kept in the record)
finishes it ``cancelled`` instead: the launch check would return at once and leave the run
``running`` on the server. The replay of an adopted process keeps whether it worked.
"""

from __future__ import annotations

from typing import Any

import pytest

from claude_code_runner.ledger import TurnLedger
from claude_code_runner.runturn import RunTurn

pytestmark = pytest.mark.anyio


def _turn(stop_reason: str | None, calls: list[str]) -> RunTurn:
    turn = RunTurn.__new__(RunTurn)
    turn.stop_reason = stop_reason

    async def finish_cancelled() -> None:
        calls.append("finish_cancelled")

    async def launch_check() -> bool:
        calls.append("launch_check")
        return False

    turn._finish_cancelled = finish_cancelled  # type: ignore[method-assign]
    turn._launch_check = launch_check  # type: ignore[method-assign]
    return turn


@pytest.mark.parametrize("reason", ["adapter_stopped", "user_cancelled"])
async def test_a_stopped_transcribing_run_is_finished_cancelled(reason: str) -> None:
    calls: list[str] = []
    await _turn(reason, calls)._continue_phase("transcribing", {})
    assert calls == ["finish_cancelled"]


async def test_without_a_stop_the_transcribing_run_waits_for_the_launch_check() -> None:
    calls: list[str] = []
    await _turn(None, calls)._continue_phase("transcribing", {})
    assert calls == ["launch_check"]


def _replaying_turn() -> RunTurn:
    turn = RunTurn.__new__(RunTurn)
    turn.ledger = TurnLedger.begin("turn", 0)
    return turn


_INIT = {"type": "system", "subtype": "init"}
_ANSWER = {"type": "assistant", "message": {"content": [{"type": "text", "text": "Done"}]}}
_API_ERROR = {**_ANSWER, "is_api_error_message": True}


@pytest.mark.parametrize(
    ("messages", "worked"),
    [
        ([_INIT, _ANSWER], True),
        ([_INIT, _API_ERROR], False),
        ([_ANSWER, _INIT], False),
        ([_INIT], False),
    ],
)
def test_the_replay_keeps_whether_the_process_worked(
    messages: list[dict[str, Any]], worked: bool
) -> None:
    # Lines before the handover point: a retry after the handover still knows the earlier
    # attempt may have changed files (the evidence _read takes over for an adopted process).
    lines = [(m, i * 10, i * 10 + 10) for i, m in enumerate(messages)]
    assert _replaying_turn()._replay_ledger(lines) is worked
