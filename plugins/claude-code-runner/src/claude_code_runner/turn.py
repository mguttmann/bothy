# SPDX-License-Identifier: Apache-2.0
"""One turn process, its files and the termination primitive (09 18.2, 18.6, note 11).

A turn is started with ``subprocess.Popen`` (an argument list, never a shell) in its own
process group; stream and stderr go to files in ``runner.state_dir/sessions/<ref>/``, not to
pipes, so a turn survives a restart of the runner and the files are the transcript.
`terminate` sends SIGINT to the group, then SIGTERM after ``runner.stop_grace`` and SIGKILL
after ``runner.kill_grace``; M2-35 reuses it for the stop.

Examples:
    >>> turn_paths(__import__("pathlib").Path("/state"), "s", "r").record.as_posix()
    '/state/turns/r.json'
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import os
import signal
import subprocess  # nosec B404 - argument list, never a shell, program from the cleaned PATH
import time
from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from pathlib import Path
from typing import IO, Any

from claude_code_runner.statedir import ensure_private_dir, write_private

__all__ = [
    "Turn",
    "TurnPaths",
    "remove_turn_record",
    "terminate",
    "turn_paths",
    "write_turn_record",
]


@dataclass(frozen=True)
class TurnPaths:
    """The files of one turn.

    Attributes:
        stream: ``sessions/<ref>/<run>.stream.jsonl`` (stdout of the turn).
        stderr: ``sessions/<ref>/<run>.stderr.log``.
        system_prompt: ``sessions/<ref>/<run>.system-prompt.md``.
        input: ``sessions/<ref>/<run>.input.jsonl`` (the first stdin line, for M2-35).
        record: ``turns/<run>.json`` (the turn record, never a token).
    """

    stream: Path
    stderr: Path
    system_prompt: Path
    input: Path
    record: Path


def turn_paths(state_dir: Path, session_ref: str, run_id: str) -> TurnPaths:
    """Return the file paths of a turn.

    Args:
        state_dir: ``runner.state_dir``.
        session_ref: The session UUID.
        run_id: The run.

    Returns:
        The paths (nothing is created).
    """
    session = state_dir / "sessions" / session_ref
    return TurnPaths(
        stream=session / f"{run_id}.stream.jsonl",
        stderr=session / f"{run_id}.stderr.log",
        system_prompt=session / f"{run_id}.system-prompt.md",
        input=session / f"{run_id}.input.jsonl",
        record=state_dir / "turns" / f"{run_id}.json",
    )


def _private_append(path: Path) -> IO[bytes]:
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_APPEND | os.O_NOFOLLOW, 0o600)
    return os.fdopen(fd, "ab")


class Turn:
    """A started ``claude -p`` process.

    Attributes:
        process: The ``Popen`` object.
        paths: The files of the turn.
        pid: The process ID.
        pgid: The process group (equal to ``pid``: an own session).
    """

    def __init__(self, process: subprocess.Popen[bytes], paths: TurnPaths) -> None:
        """Wrap a started process; use `Turn.start`."""
        self.process = process
        self.paths = paths
        self.pid = process.pid
        self.pgid = process.pid

    @classmethod
    def start(
        cls,
        argv: Sequence[str],
        *,
        cwd: Path,
        env: Mapping[str, str],
        paths: TurnPaths,
        pass_fds: Sequence[int],
    ) -> Turn:
        """Start the process of a turn.

        Args:
            argv: The argument list (`claude_code_runner.commandline.build_argv`).
            cwd: The resolved project directory.
            env: The run environment.
            paths: The files of the turn.
            pass_fds: The lock descriptors the turn inherits.

        Returns:
            The turn.

        Raises:
            OSError: The program could not be started.
        """
        ensure_private_dir(paths.stream.parent)
        for fd in pass_fds:
            os.set_inheritable(fd, True)
        with _private_append(paths.stream) as out, _private_append(paths.stderr) as err:
            process = subprocess.Popen(  # noqa: S603  # nosec B603 - argument list, no shell, cleaned PATH
                list(argv),
                cwd=cwd,
                env=dict(env),
                stdin=subprocess.PIPE,
                stdout=out,
                stderr=err,
                start_new_session=True,
                pass_fds=tuple(pass_fds),
                close_fds=True,
            )
        return cls(process, paths)

    def write_line(self, line: str) -> None:
        """Write one line to stdin of the turn.

        Args:
            line: The JSON line without its line break.
        """
        stdin = self.process.stdin
        if stdin is None:
            return
        with contextlib.suppress(BrokenPipeError, ValueError):
            stdin.write(line.encode("utf-8") + b"\n")
            stdin.flush()

    def close_stdin(self) -> None:
        """Close stdin; the turn ends after its ``result``."""
        stdin = self.process.stdin
        if stdin is not None:
            with contextlib.suppress(BrokenPipeError, ValueError, OSError):
                stdin.close()

    def poll(self) -> int | None:
        """Return the exit code, or ``None`` while the process runs."""
        return self.process.poll()

    def alive(self) -> bool:
        """Tell whether the process still runs."""
        return self.process.poll() is None

    async def wait(self, timeout: float, poll: float = 0.02) -> bool:
        """Wait for the exit.

        Args:
            timeout: Seconds to wait at most.
            poll: Interval of the checks.

        Returns:
            True when the process has exited.
        """
        deadline = time.monotonic() + timeout
        while self.process.poll() is None:
            if time.monotonic() >= deadline:
                return False
            await asyncio.sleep(poll)
        self.close_stdin()  # the pipe of an ended process is never written again
        return True

    def exit_label(self) -> str:
        """The exit code as text, or the name of the signal that ended the process.

        Returns:
            For example ``"0"``, ``"1"`` or ``"SIGKILL"``; ``"running"`` while it runs.
        """
        code = self.process.poll()
        if code is None:
            return "running"
        if code < 0:
            with contextlib.suppress(ValueError):
                return signal.Signals(-code).name
        return str(code)


def _signal_group(turn: Turn, sig: signal.Signals) -> None:
    with contextlib.suppress(ProcessLookupError, PermissionError):
        os.killpg(turn.pgid, sig)


async def terminate(turn: Turn, stop_grace: float, kill_grace: float, poll: float) -> None:
    """End a turn and every process of its group (18.6).

    Args:
        turn: The turn.
        stop_grace: ``runner.stop_grace``, from SIGINT to SIGTERM.
        kill_grace: ``runner.kill_grace``, from SIGTERM to SIGKILL.
        poll: ``runner.poll_interval``.
    """
    for sig, grace in ((signal.SIGINT, stop_grace), (signal.SIGTERM, kill_grace)):
        if not turn.alive():
            break
        _signal_group(turn, sig)
        if await turn.wait(grace, poll):
            break
    _signal_group(turn, signal.SIGKILL)  # also children that outlived the leader
    await turn.wait(max(kill_grace, poll), poll)


def write_turn_record(path: Path, record: Mapping[str, Any]) -> None:
    """Write the turn record M2-35 reads after a restart (atomic, mode ``0600``).

    Args:
        path: ``turns/<run>.json``.
        record: Run, thread, channel, session reference, pid, group, start time and paths.
    """
    ensure_private_dir(path.parent)
    write_private(path, json.dumps(dict(record), sort_keys=True), atomic=True)


def remove_turn_record(path: Path) -> None:
    """Remove the turn record once the run has ended.

    Args:
        path: ``turns/<run>.json``.
    """
    with contextlib.suppress(FileNotFoundError):
        path.unlink()
