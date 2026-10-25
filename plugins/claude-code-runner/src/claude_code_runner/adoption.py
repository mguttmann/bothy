# SPDX-License-Identifier: Apache-2.0
"""Turns that outlived a runner, and their identity (09 18.2 "Detachment", 18.9; note 12).

A turn's stdout and stderr are files and its process group is its own, so a crash of the runner
leaves the turn running; stdin was a pipe of the old runner, so the CLI sees the end of its input
and finishes its current work. A new runner reads the turn records before the first event of the
inbox and before the first heartbeat and decides per record (`adoption_decision`): adopt a live
turn of a ``running`` run, stop a live turn of an ended run without a finish, finish a gone turn
from the ``result`` in its stream file, or leave a gone turn without ``result`` to the server
(it ends the run ``interrupted``, 09 5.5). It never starts a process or ``run.start`` for such a
run (R-AG-17).

Before every signal the stored ``pid`` and ``pgid`` must still be the process the runner started:
``os.getpgid(pid)`` equals the stored group, and ``ps -ww -o lstart= -o command= -p <pid>``
gives the stored start time and a command that contains the session reference
(`process_matches`); otherwise the process counts as gone (D-M2-35-11).

Examples:
    >>> adoption_decision(alive=True, run_state="running", result_in_log=False).value
    'adopt'
"""

from __future__ import annotations

import os
import shutil
import subprocess  # nosec B404 - a fixed argument list, never a shell
from collections.abc import Mapping
from dataclasses import dataclass
from enum import Enum
from typing import Any

__all__ = [
    "AdoptedGroup",
    "AdoptionAction",
    "ProcessIdentity",
    "adoption_decision",
    "process_matches",
    "read_identity",
]


class AdoptionAction(Enum):
    """What a new runner does with a turn record (design note 12 d)."""

    ADOPT = "adopt"
    """Live turn, run ``running``: follow it to its end and finish once."""
    STOP = "stop"
    """Live turn, run ended: stop the group (note 6 a), no finish."""
    FINISH_FROM_LOG = "finish_from_log"
    """Turn gone, its ``result`` is in the stream file: finish by it."""
    LEAVE = "leave"
    """Turn gone without ``result``: no finish, not in ``active_runs``."""


@dataclass(frozen=True)
class ProcessIdentity:
    """What identifies a started process beyond its PID (design note 12 c).

    Attributes:
        lstart: The start time as ``ps -o lstart=`` prints it.
        command: The command line as ``ps -ww -o command=`` prints it.
    """

    lstart: str
    command: str


def read_identity(pid: int, timeout: float) -> ProcessIdentity | None:
    """Read the start time and command line of a process with ``ps``.

    Args:
        pid: The process.
        timeout: ``runner.cli_probe_timeout``, the bound of the local call.

    Returns:
        The identity, or ``None`` when the process does not exist or ``ps`` is unavailable.
    """
    ps = shutil.which("ps", path="/bin:/usr/bin")
    if ps is None:
        return None
    try:
        out = subprocess.run(  # noqa: S603  # nosec B603 - fixed argument list, no shell
            [ps, "-ww", "-o", "lstart=", "-o", "command=", "-p", str(pid)],
            capture_output=True,
            text=True,
            timeout=timeout,
            check=False,
            env={"LC_ALL": "C", "PATH": "/bin:/usr/bin"},
        )
    except (OSError, subprocess.TimeoutExpired):
        return None
    line = out.stdout.strip()
    if out.returncode != 0 or not line:
        return None
    parts = line.split(None, 5)
    if len(parts) < 6:  # five fields of lstart plus the command
        return None
    return ProcessIdentity(" ".join(parts[:5]), parts[5])


def process_matches(record: Mapping[str, Any], timeout: float) -> bool:
    """Whether the process of a turn record is still the one the runner started.

    Args:
        record: The turn record (``pid``, ``pgid``, ``identity``, ``session_ref``).
        timeout: ``runner.cli_probe_timeout`` for ``ps``.

    Returns:
        True only when the group, the start time and the session reference in the command
        line all match; any doubt counts as gone.
    """
    pid, pgid = record.get("pid"), record.get("pgid")
    identity = record.get("identity")
    ref = record.get("session_ref")
    if not isinstance(pid, int) or not isinstance(pgid, int) or pid <= 1:
        return False
    if not isinstance(identity, Mapping) or not isinstance(ref, str) or not ref:
        return False
    try:
        if os.getpgid(pid) != pgid:
            return False
    except (ProcessLookupError, PermissionError):
        return False
    current = read_identity(pid, timeout)
    if current is None:
        return False
    return current.lstart == identity.get("lstart") and ref in current.command


def adoption_decision(*, alive: bool, run_state: str, result_in_log: bool) -> AdoptionAction:
    """Decide what happens to one turn record (design note 12 d).

    Args:
        alive: The process matches its identity and runs.
        run_state: The run's state from ``getRun``.
        result_in_log: The stream file has a ``result`` after the stored offset.

    Returns:
        The action.
    """
    if alive:
        return AdoptionAction.ADOPT if run_state == "running" else AdoptionAction.STOP
    if result_in_log and run_state == "running":
        return AdoptionAction.FINISH_FROM_LOG
    return AdoptionAction.LEAVE


class AdoptedGroup:
    """The process group of a turn record, for `claude_code_runner.stop.stop_group`.

    The runner did not start this process in its own lifetime, so it signals it only after
    `process_matches` (design note 12 c), and has no stdin to close.

    Args:
        record: The turn record.
        timeout: ``runner.cli_probe_timeout`` for ``ps``.
    """

    def __init__(
        self, record: Mapping[str, Any], timeout: float, *, verified: bool = False
    ) -> None:
        """Wrap a record; see the class docstring.

        Args:
            record: The turn record.
            timeout: ``runner.cli_probe_timeout`` for ``ps``.
            verified: `process_matches` held for the record at the start of this runner.
        """
        self.record = dict(record)
        self.timeout = timeout
        self.pid = int(record.get("pid") or 0)
        self.pgid = int(record.get("pgid") or 0)
        self.verified = verified

    def close_stdin(self) -> None:
        """Nothing to close: stdin ended with the previous runner."""

    def alive(self) -> bool:
        """Whether the leader still runs in its group (no ``ps`` call)."""
        if self.pid <= 1:
            return False
        try:
            return os.getpgid(self.pid) == self.pgid
        except (ProcessLookupError, PermissionError):
            return False

    def may_signal(self) -> bool:
        """The identity check before a signal (design note 12 c).

        While the leader lives, `process_matches` decides. Once a group whose leader matched has
        lost its leader, the group is still the one the runner started for as long as any of
        its processes lives, because a process group ID is not given to a new process while the
        group exists; so its remaining processes (a background job of a tool call) can be
        stopped. A group that never matched is never signalled.

        Returns:
            True when a signal to the group reaches only processes the runner started.
        """
        if self.alive() and process_matches(self.record, self.timeout):
            self.verified = True
            return True
        if not self.verified or self.alive():
            return False
        try:
            os.killpg(self.pgid, 0)
        except (ProcessLookupError, PermissionError):
            return False
        return True
