# SPDX-License-Identifier: Apache-2.0
"""The escalation schedule of a stop (09 18.6, design note 6)."""

from __future__ import annotations

import signal

from claude_code_runner.stop import escalation_due

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
