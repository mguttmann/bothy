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

A record is a file an earlier process wrote, so nothing in it is taken as given
(docs/decisions.d/FIX-RUNNER-HOSTING-REVIEW.md): `read_turn_record` returns a record only when it
is a regular file with a JSON object whose ``run_id`` is the UUID the file is named after and
whose ``thread_id`` and ``session_ref`` are UUIDs, so no name of a lock and no path is built from
free text; only the record of a run before ``run.start`` carries no ``session_ref``
(`PHASES_BEFORE_START`). `process_id` takes a ``pid`` or ``pgid`` only as an integer above 1 that
fits the system's type, so a value outside it counts as no process instead of an error of the
start.

Examples:
    >>> adoption_decision(alive=True, run_state="running", result_in_log=False).value
    'adopt'
"""

from __future__ import annotations

import itertools
import json
import os
import shutil
import stat
import subprocess  # nosec B404 - a fixed argument list, never a shell
from collections.abc import Mapping
from dataclasses import dataclass
from enum import Enum
from pathlib import Path
from typing import Any
from uuid import UUID

__all__ = [
    "MAX_PROCESS_ID",
    "PHASES_BEFORE_START",
    "AdoptedGroup",
    "AdoptionAction",
    "ProcessIdentity",
    "adoption_decision",
    "matching_command",
    "names_session",
    "process_id",
    "process_matches",
    "read_identity",
    "read_turn_record",
]

MAX_PROCESS_ID = 2**31 - 1
"""The largest process or group ID a record may name: the system's type is a signed 32-bit
integer on Linux and macOS, and ``os.getpgid`` raises ``OverflowError`` above it."""

PHASES_BEFORE_START = ("preparing", "waiting_lock")
"""The phases of a record that lie before ``run.start`` (09 18.19.6). With
``runner.turn_handover`` the runner writes such a record when it accepts a run, with the run, its
thread and channel and the phase; the session is stored from the phase ``transcribing`` on. A
next runner removes the record and the inbox restart prepares the run again."""

_SESSION_FLAGS = frozenset({"--session-id", "--resume"})
"""The flags of the CLI whose argument is the session reference (09 18.2)."""


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


def process_id(value: object) -> int | None:
    """A process or group ID as a record may name it.

    Args:
        value: ``pid`` or ``pgid`` of a record.

    Returns:
        The ID, or ``None`` for anything but an integer above 1 that fits the system's type
        (`MAX_PROCESS_ID`): no number, a truth value, 0 and 1 (every process and ``init``), a
        negative number (a group, to ``kill``) and a number the system cannot hold.

    Examples:
        >>> process_id(4711), process_id(1), process_id(True), process_id(2**31), process_id("7")
        (4711, None, None, None, None)
    """
    if isinstance(value, bool) or not isinstance(value, int):
        return None
    return value if 1 < value <= MAX_PROCESS_ID else None


def _is_uuid(value: object, *, any_case: bool = False) -> bool:
    """Whether a member of a record is a UUID in its canonical text, as the runner writes it."""
    if not isinstance(value, str):
        return False
    try:
        return str(UUID(value)) == (value.lower() if any_case else value)
    except ValueError:
        return False


def _session_checked(record: Mapping[str, Any]) -> bool:
    """Whether the session of a record is a UUID, or the record is one that carries none."""
    if "session_ref" not in record:
        # A tuple, so a phase of any JSON type is compared and never hashed.
        return record.get("phase") in PHASES_BEFORE_START
    return _is_uuid(record["session_ref"], any_case=True)


def read_turn_record(path: Path) -> dict[str, Any] | None:
    """Read a turn record and check what the runner builds names and paths from.

    The file is opened without following a link and without waiting, so a link, a named pipe
    or a folder in the place of a record never holds up or misleads the start.

    Args:
        path: ``turns/<run_id>.json``.

    Returns:
        The record, or ``None`` when it is unreadable: no regular file, no JSON object, a
        ``run_id`` that is not the UUID the file is named after, or a ``thread_id`` or
        ``session_ref`` that is no UUID in its canonical text (a reference in upper case is one,
        as `claude_code_runner.commandline.choose_session` takes it). A record without the
        member ``session_ref`` is returned only with a phase before ``run.start``
        (`PHASES_BEFORE_START`), as the runner writes it: a next runner removes it by its
        phase, and no lock and no path is built from it.
    """
    try:
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    except OSError:
        return None
    try:
        if not stat.S_ISREG(os.fstat(fd).st_mode):
            return None
        with os.fdopen(fd, "rb", closefd=False) as file:
            record = json.loads(file.read())
    except (OSError, ValueError, RecursionError):
        return None
    finally:
        os.close(fd)
    if not isinstance(record, dict):
        return None
    run_id = record.get("run_id")
    if run_id != path.stem or not _is_uuid(run_id) or not _is_uuid(record.get("thread_id")):
        return None
    return record if _session_checked(record) else None


def matching_command(record: Mapping[str, Any], timeout: float) -> str | None:
    """The command line of the process of a turn record while it is still that process.

    Args:
        record: The turn record (``pid``, ``pgid``, ``identity``, ``session_ref``).
        timeout: ``runner.cli_probe_timeout`` for ``ps``.

    Returns:
        The command line as ``ps`` prints it now, only when the group, the start time and the
        session reference in the command line all match; else ``None`` (any doubt counts as
        gone).
    """
    pid, pgid = process_id(record.get("pid")), process_id(record.get("pgid"))
    identity = record.get("identity")
    ref = record.get("session_ref")
    if pid is None or pgid is None:
        return None
    if not isinstance(identity, Mapping) or not isinstance(ref, str) or not ref:
        return None
    try:
        if os.getpgid(pid) != pgid:
            return None
    except (ProcessLookupError, PermissionError):
        return None
    current = read_identity(pid, timeout)
    if current is None or current.lstart != identity.get("lstart") or ref not in current.command:
        return None
    return current.command


def process_matches(record: Mapping[str, Any], timeout: float) -> bool:
    """Whether the process of a turn record is still the one the runner started.

    Args:
        record: The turn record (``pid``, ``pgid``, ``identity``, ``session_ref``).
        timeout: ``runner.cli_probe_timeout`` for ``ps``.

    Returns:
        True only when the group, the start time and the session reference in the command
        line all match (`matching_command`); any doubt counts as gone.
    """
    return matching_command(record, timeout) is not None


def names_session(command: str, ref: str) -> bool:
    """Whether a command line names ``ref`` as the session of the CLI.

    Args:
        command: A command line as ``ps`` prints it (the arguments joined by spaces).
        ref: The session reference, a UUID.

    Returns:
        True when ``ref`` is the whole argument right after ``--session-id`` or ``--resume``;
        a reference that is only a part of an argument, or stands elsewhere, does not count.

    Examples:
        >>> names_session("claude -p --resume 0192d3a0-0000-4000-8000-0000000000dd --model m",
        ...               "0192d3a0-0000-4000-8000-0000000000dd")
        True
        >>> names_session("tail -f 0192d3a0-0000-4000-8000-0000000000dd.jsonl",
        ...               "0192d3a0-0000-4000-8000-0000000000dd")
        False
    """
    words = command.split()
    return any(flag in _SESSION_FLAGS and value == ref for flag, value in itertools.pairwise(words))


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
        self.pid = process_id(record.get("pid")) or 0
        self.pgid = process_id(record.get("pgid")) or 0
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

    def matches(self) -> bool:
        """Whether the leader is still the process of the record (`process_matches`).

        A turn that is bound to more than its record checks that here as well, so the same
        check runs before every signal (`claude_code_runner.hosting`).
        """
        return process_matches(self.record, self.timeout)

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
        if self.alive() and self.matches():
            self.verified = True
            return True
        if not self.verified or self.alive():
            return False
        try:
            os.killpg(self.pgid, 0)
        except (ProcessLookupError, PermissionError):
            return False
        return True
