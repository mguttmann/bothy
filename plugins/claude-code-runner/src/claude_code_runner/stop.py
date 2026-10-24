# SPDX-License-Identifier: Apache-2.0
"""Stopping the process group of a turn (09 8 point 3, 18.6; design note 6).

SIGINT to the group ends the model turn cleanly with a ``result``; stdin is closed right after,
so the process ends after that ``result`` instead of waiting for input. If a process of the
group is still alive ``runner.stop_grace`` after SIGINT, the group gets SIGTERM; if one is still
alive ``runner.kill_grace`` later, SIGKILL. `escalation_due` is the pure schedule; `stop_group`
applies it and returns only when no process of the group is left (``os.killpg(pgid, 0)``
raises ``ProcessLookupError``). Every SIGTERM and SIGKILL is logged with ``stop_escalated``.

Examples:
    >>> escalation_due(10.0, 0.0, True, 5.0, 3.0).name
    'SIGKILL'
    >>> escalation_due(1.0, 0.0, True, 5.0, 3.0) is None
    True
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import logging
import os
import signal
import time
from collections.abc import Callable
from typing import Protocol

__all__ = ["StoppableGroup", "escalation_due", "group_alive", "stop_group"]

_LOG = logging.getLogger("claude_code_runner")


class StoppableGroup(Protocol):
    """A process group the runner started (its own child or an adopted turn)."""

    pgid: int

    def close_stdin(self) -> None:
        """Close the runner's end of stdin (no-op for an adopted turn)."""

    def alive(self) -> bool:
        """Whether the group's leader still runs (reaps an own child)."""

    def may_signal(self) -> bool:
        """Whether the identity of the group still matches what the runner started."""


def escalation_due(
    now: float, sigint_at: float, alive: bool, stop_grace: float, kill_grace: float
) -> signal.Signals | None:
    """The signal the schedule of 18.6 calls for now.

    Args:
        now: Monotonic seconds.
        sigint_at: When SIGINT was sent.
        alive: A process of the group is still alive.
        stop_grace: ``runner.stop_grace``.
        kill_grace: ``runner.kill_grace``.

    Returns:
        ``SIGKILL`` from ``stop_grace + kill_grace`` after SIGINT on, ``SIGTERM`` from
        ``stop_grace`` on, else ``None``; always ``None`` once the group is gone.
    """
    if not alive:
        return None
    elapsed = now - sigint_at
    if elapsed >= stop_grace + kill_grace:
        return signal.SIGKILL
    if elapsed >= stop_grace:
        return signal.SIGTERM
    return None


def group_alive(group: StoppableGroup) -> bool:
    """Whether the leader or any other process of the group still runs.

    Args:
        group: The group.

    Returns:
        False once ``os.killpg(pgid, 0)`` raises ``ProcessLookupError`` and the leader ended,
        and for a group whose identity no longer matches (it counts as gone, note 12 c).
    """
    if not group.may_signal():
        return False
    if group.alive():
        return True
    try:
        os.killpg(group.pgid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return False  # the group ID was reused by a process of another user: ours is gone
    return True


def _signal(group: StoppableGroup, sig: signal.Signals) -> None:
    if not group.may_signal():
        return
    with contextlib.suppress(ProcessLookupError, PermissionError):
        os.killpg(group.pgid, sig)


async def stop_group(
    group: StoppableGroup,
    stop_grace: float,
    kill_grace: float,
    poll: float,
    *,
    clock: Callable[[], float] = time.monotonic,
    **ids: object,
) -> None:
    """SIGINT, close stdin, escalate, and return once no process of the group is left.

    Args:
        group: The group of the turn.
        stop_grace: ``runner.stop_grace``.
        kill_grace: ``runner.kill_grace``.
        poll: ``runner.poll_interval``.
        clock: Monotonic seconds (tests).
        **ids: IDs for the log line of an escalation.
    """
    if not group_alive(group):
        group.close_stdin()
        return
    _signal(group, signal.SIGINT)
    group.close_stdin()
    sigint_at = clock()
    sent: set[signal.Signals] = set()
    while True:
        alive = group_alive(group)
        if not alive:
            return
        due = escalation_due(clock(), sigint_at, alive, stop_grace, kill_grace)
        if due is not None and due not in sent:
            sent.add(due)
            if due is signal.SIGKILL:
                sent.add(signal.SIGTERM)
            _LOG.warning(
                json.dumps(
                    {
                        "event": "stop_escalated",
                        "signal": due.name,
                        **{k: str(v) for k, v in ids.items()},
                    }
                )
            )
            _signal(group, due)
        await asyncio.sleep(poll)
