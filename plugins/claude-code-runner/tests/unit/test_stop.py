# SPDX-License-Identifier: Apache-2.0
"""The escalation schedule of a stop (09 18.6, design note 6)."""

from __future__ import annotations

import signal

import pytest

from claude_code_runner import stop
from claude_code_runner.stop import escalation_due, stop_group

STOP, KILL = 10.0, 5.0


def test_escalation_due() -> None:
    assert escalation_due(0.0, 0.0, True, STOP, KILL) is None
    assert escalation_due(STOP - 0.01, 0.0, True, STOP, KILL) is None
    assert escalation_due(STOP, 0.0, True, STOP, KILL) is signal.SIGTERM
    assert escalation_due(STOP + KILL - 0.01, 0.0, True, STOP, KILL) is signal.SIGTERM
    assert escalation_due(STOP + KILL, 0.0, True, STOP, KILL) is signal.SIGKILL
    for at in (0.0, STOP, STOP + KILL, STOP + KILL + 100):
        assert escalation_due(at, 0.0, False, STOP, KILL) is None  # an exit ends the schedule
    assert escalation_due(100.0 + STOP, 100.0, True, STOP, KILL) is signal.SIGTERM


@pytest.mark.anyio
@pytest.mark.parametrize("pgid", [0, 1, -7])
async def test_a_group_id_below_two_is_never_signalled(
    monkeypatch: pytest.MonkeyPatch, pgid: int
) -> None:
    # A signal to group 0 goes to the group of the caller, the runner itself, and one to a
    # negative number to another target again. No turn has such a group, so whatever a group
    # object says about its identity, nothing is sent (FIX-RUNNER-HOSTING-REVIEW).
    sent: list[tuple[int, int]] = []

    def killpg(group: int, sig: int) -> None:
        if sig != 0:
            sent.append((group, sig))
        elif sent:  # signal 0 only asks whether the group exists: until it was signalled
            raise ProcessLookupError

    class Group:
        def __init__(self) -> None:
            self.pgid = pgid

        def close_stdin(self) -> None:
            return None

        def alive(self) -> bool:
            return False

        def may_signal(self) -> bool:
            return True

    monkeypatch.setattr(stop.os, "killpg", killpg)
    await stop_group(Group(), 0.01, 0.01, 0.01)
    assert sent == []
