# SPDX-License-Identifier: Apache-2.0
"""The runner's side of the launch handshake and of a turn under a keeper (09 18.19.4, 18.19.5).

With ``runner.turn_handover`` every process of a run (an attempt) starts through the same
handshake, and every identity is persisted before the CLI runs; `claude_code_runner.runturn`
writes the turn record before each step:

1. `prepare_launch` writes ``<launch>.inputs`` (the attempt's initial input lines) and
   ``<launch>.spec`` (argument list, environment, folder, files, lock descriptors; mode ``0600``,
   removed by the keeper at its start) and creates the named pipe ``<launch>.stdin`` (mode
   ``0600``), removing the named pipe of an earlier try.
2. `start_keeper` starts ``python -m claude_code_runner.keeper`` in its own session with one
   extra descriptor, the read end of the launch pipe whose write end only the runner holds.
3. The runner reads ``<launch>.keeper`` (`wait_for`), checks it, records it with ``go_sent``
   and only then writes the go byte (`KeeperStart.go`).
4. The keeper forks and names the CLI in ``<launch>.cli`` before it releases it.
5. The runner records the CLI's identity; the keeper writes the initial inputs and
   ``<launch>.fed``.

`KeeperGroup` is the group of keeper and CLI as the runner handles it: own (the runner reaps the
keeper) or adopted (identities polled). It writes input lines into the named pipe through
`PipeWriter` (``O_WRONLY | O_NONBLOCK``, ``ENXIO`` means the turn is gone, never a hang; the write
runs off the event loop, counts its bytes and can be aborted at a handover), closes stdin (the
runner's end, then SIGUSR1 to the keeper after its identity check), and takes ``exit_label`` from
``<launch>.exit`` (``unknown`` without it). `settle_launching` decides a record left in phase
``launching`` by the four cases of 18.19.5.

`<launch>` is ``<runner.state_dir>/turns/<run_id>.<attempt>.<try>`` (`LaunchFiles`).

Examples:
    >>> LaunchFiles.of(Path("/s"), "r", 1, 2).stdin.as_posix()
    '/s/turns/r.1.2.stdin'
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import os
import select
import signal
import subprocess  # nosec B404 - a fixed argument list, never a shell
import sys
import threading
import time
from collections.abc import Callable, Mapping, Sequence
from dataclasses import dataclass, field
from enum import Enum
from pathlib import Path
from typing import Any

from claude_code_runner.adoption import read_identity
from claude_code_runner.keeper import NOT_STARTED
from claude_code_runner.statedir import ensure_private_dir, write_private

__all__ = [
    "ATTEMPT_KINDS",
    "AttemptRecord",
    "KeeperGroup",
    "KeeperStart",
    "LaunchFiles",
    "PipeWriter",
    "Settled",
    "identity_matches",
    "prepare_launch",
    "process_identity",
    "read_json",
    "remove_launch_files",
    "settle_launching",
    "start_keeper",
    "wait_for",
]

ATTEMPT_KINDS = ("first", "compaction", "retry", "resume_same_reference", "new_session")
"""The kinds of an attempt (18.19.5)."""

_SUFFIXES = ("stdin", "keeper", "cli", "exit", "fed", "inputs", "spec")
_CHUNK = 65536
_PACKAGE_ROOT = Path(__file__).resolve().parents[1]


@dataclass(frozen=True)
class LaunchFiles:
    """The files of one launch, ``<launch>.<suffix>``.

    Attributes:
        prefix: ``<runner.state_dir>/turns/<run_id>.<attempt>.<try>``.
    """

    prefix: Path

    @classmethod
    def of(cls, state_dir: Path, run_id: str, attempt: int, try_: int) -> LaunchFiles:
        """The files of attempt ``attempt``, try ``try_`` of a run.

        Args:
            state_dir: ``runner.state_dir``.
            run_id: The run.
            attempt: The attempt's number (the first is 1).
            try_: The launch of that attempt (a launch repeated after a crash adds one).

        Returns:
            The files (nothing is created).
        """
        return cls(state_dir / "turns" / f"{run_id}.{attempt}.{try_}")

    def path(self, suffix: str) -> Path:
        """``<launch>.<suffix>``."""
        return Path(f"{self.prefix}.{suffix}")

    @property
    def stdin(self) -> Path:
        """The named pipe."""
        return self.path("stdin")

    @property
    def keeper(self) -> Path:
        """The keeper's identity file."""
        return self.path("keeper")

    @property
    def cli(self) -> Path:
        """The CLI's identity file."""
        return self.path("cli")

    @property
    def exit(self) -> Path:
        """The CLI's wait status, or ``not_started``."""
        return self.path("exit")

    @property
    def fed(self) -> Path:
        """Written by the keeper after the last byte of the initial inputs."""
        return self.path("fed")

    @property
    def inputs(self) -> Path:
        """The attempt's initial input lines."""
        return self.path("inputs")

    @property
    def spec(self) -> Path:
        """The CLI's start parameters for the keeper (removed by it)."""
        return self.path("spec")

    def record(self) -> dict[str, str]:
        """The paths the turn record carries (18.19.5 step 1).

        Returns:
            Each of ``stdin``, ``keeper``, ``cli``, ``exit``, ``fed`` and the ``prefix``.
        """
        out = {s: str(self.path(s)) for s in ("stdin", "keeper", "cli", "exit", "fed")}
        out["prefix"] = str(self.prefix)
        return out

    def remove(self) -> None:
        """Remove every file of this launch (and a temporary file of an atomic write)."""
        for suffix in _SUFFIXES:
            for path in (self.path(suffix), self.path(f"{suffix}.tmp")):
                with contextlib.suppress(FileNotFoundError, IsADirectoryError):
                    path.unlink()


def remove_launch_files(state_dir: Path, run_id: str) -> None:
    """Remove every launch file and the input lines of an ended run (not its record).

    Args:
        state_dir: ``runner.state_dir``.
        run_id: The run.
    """
    turns = state_dir / "turns"
    for path in turns.glob(f"{run_id}.*") if turns.is_dir() else []:
        if path.name != f"{run_id}.json":
            with contextlib.suppress(FileNotFoundError, IsADirectoryError):
                path.unlink()


def read_json(path: Path) -> dict[str, Any] | None:
    """A small JSON object file, or ``None`` when it is missing or not an object."""
    try:
        value = json.loads(path.read_text())
    except (OSError, ValueError):
        return None
    return value if isinstance(value, dict) else None


def prepare_launch(
    files: LaunchFiles,
    inputs: bytes,
    spec: Mapping[str, Any],
    *,
    earlier: LaunchFiles | None = None,
) -> None:
    """Step 1 of 18.19.5 after the record: the inputs, the spec and the named pipe.

    Args:
        files: The files of this launch.
        inputs: The initial input lines (complete lines).
        spec: ``argv``, ``env``, ``cwd``, ``stdout``, ``stderr`` and ``lock_fds`` of the CLI.
        earlier: The files of the earlier try of the same attempt: its named pipe and every
            other file are removed (a spec left by a crash may hold a key).
    """
    ensure_private_dir(files.prefix.parent)
    if earlier is not None:
        earlier.remove()
    write_private(files.inputs, inputs.decode("utf-8"), atomic=True)
    write_private(files.spec, json.dumps(dict(spec)), atomic=True)
    with contextlib.suppress(FileNotFoundError):
        files.stdin.unlink()
    os.mkfifo(files.stdin, 0o600)


@dataclass
class KeeperStart:
    """A started keeper and the write end of its launch pipe (only the runner holds it).

    Attributes:
        process: The keeper (an own child: the runner reaps it).
        go_w: The write end of the launch pipe, ``-1`` once closed.
    """

    process: subprocess.Popen[bytes]
    go_w: int

    def go(self) -> None:
        """Write the go byte and close the launch pipe (step 3, after the record)."""
        if self.go_w < 0:
            return
        try:
            os.write(self.go_w, b"g")
        finally:
            self.close()

    def close(self) -> None:
        """Close the launch pipe without a go byte: the keeper starts no CLI."""
        if self.go_w >= 0:
            with contextlib.suppress(OSError):
                os.close(self.go_w)
            self.go_w = -1


def start_keeper(files: LaunchFiles, *, compaction: bool, lock_fds: Sequence[int]) -> KeeperStart:
    """Step 2 of 18.19.5: the keeper in its own session with the launch pipe's read end.

    Args:
        files: The files of the launch (prepared).
        compaction: The attempt is a compaction (the keeper closes stdin after its inputs).
        lock_fds: The lock descriptors the CLI inherits (the keeper closes its copies).

    Returns:
        The keeper and the launch pipe's write end.

    Raises:
        OSError: The keeper could not be started.
    """
    go_r, go_w = os.pipe()
    argv = [sys.executable, "-m", "claude_code_runner.keeper", "--launch", str(files.prefix)]
    argv += ["--go-fd", str(go_r)]
    if compaction:
        argv.append("--compaction")
    for fd in lock_fds:
        os.set_inheritable(fd, True)
    try:
        process = subprocess.Popen(  # noqa: S603  # nosec B603 - fixed argument list, no shell
            argv,
            # The keeper needs no secret: the CLI's environment travels in the spec file.
            env={"PATH": "/usr/bin:/bin", "PYTHONPATH": str(_PACKAGE_ROOT), "LC_ALL": "C"},
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            start_new_session=True,
            pass_fds=(go_r, *lock_fds),
            close_fds=True,
        )
    except OSError:
        os.close(go_w)
        raise
    finally:
        os.close(go_r)
    return KeeperStart(process, go_w)


async def wait_for(
    path: Path, alive: Callable[[], bool], timeout: float, poll: float
) -> dict[str, Any] | None:
    """Wait for an identity file while its writer lives (at most ``timeout``).

    Args:
        path: ``<launch>.keeper`` or ``<launch>.cli``.
        alive: Whether the writer (the keeper) still runs.
        timeout: The bound of the wait.
        poll: ``runner.poll_interval``.

    Returns:
        The file's object, or ``None`` when the writer ended or the time ran out without it.
    """
    loop = asyncio.get_running_loop()
    deadline = loop.time() + timeout
    while True:
        found = read_json(path)
        if found is not None:
            return found
        if not alive() or loop.time() >= deadline:
            return read_json(path)
        await asyncio.sleep(poll)


def process_identity(pid: int, timeout: float) -> dict[str, str]:
    """The start time and command line of a process (``ps``), for the record.

    Args:
        pid: The process.
        timeout: ``runner.cli_probe_timeout``.

    Returns:
        ``lstart`` and ``command``, empty when the process is gone.
    """
    found = read_identity(pid, timeout)
    return {"lstart": found.lstart, "command": found.command} if found else {}


def identity_matches(ident: Mapping[str, Any] | None, marker: str, timeout: float) -> bool:
    """Whether a recorded process is still the one the runner started (``process_matches``).

    Args:
        ident: ``pid``, ``pgid`` and, when known, ``lstart`` of the record or identity file.
        marker: A text its command line contains (the session reference for the CLI, the
            launch path for the keeper).
        timeout: ``runner.cli_probe_timeout`` for ``ps``.

    Returns:
        True only when the group, the start time (when recorded) and the marker all match;
        any doubt counts as gone.
    """
    if not isinstance(ident, Mapping):
        return False
    pid, pgid = ident.get("pid"), ident.get("pgid")
    if not isinstance(pid, int) or not isinstance(pgid, int) or pid <= 1 or pgid <= 1:
        return False
    try:
        if os.getpgid(pid) != pgid:
            return False
    except (ProcessLookupError, PermissionError):
        return False
    current = read_identity(pid, timeout)
    if current is None or marker not in current.command:
        return False
    lstart = ident.get("lstart")
    return not lstart or current.lstart == lstart


def _group_exists(pgid: int) -> bool:
    try:
        os.killpg(pgid, 0)
    except (ProcessLookupError, PermissionError):
        return False
    return True


@dataclass
class PipeWriter:
    """The runner's write end of a turn's named pipe (18.19.3, 18.19.4).

    Attributes:
        fifo: The named pipe.
        written: Bytes of the current (or last) line written so far.
    """

    fifo: Path
    written: int = 0
    _fd: int = -1
    _abort: threading.Event = field(default_factory=threading.Event)
    _done: threading.Event = field(default_factory=threading.Event)
    _lock: threading.Lock = field(default_factory=threading.Lock)

    def __post_init__(self) -> None:
        """No write is in flight at the start."""
        self._done.set()

    @property
    def in_flight(self) -> bool:
        """A write has started and not ended."""
        return not self._done.is_set()

    def _open(self) -> bool:
        if self._fd >= 0:
            return True
        try:
            self._fd = os.open(self.fifo, os.O_WRONLY | os.O_NONBLOCK | os.O_NOFOLLOW)
        except OSError:
            return False  # ENXIO: no reader, the turn is gone; or the pipe was removed
        return True

    def _write(self, data: bytes) -> bool:
        with self._lock:
            if self._abort.is_set() or not self._open():
                return False
            view = memoryview(data)
            while view:
                if self._abort.is_set():
                    return False
                try:
                    count = os.write(self._fd, view[:_CHUNK])
                except BlockingIOError:
                    select.select([], [self._fd], [], 0.05)
                    continue
                except OSError:
                    return False  # EPIPE: the CLI is gone
                self.written += count
                view = view[count:]
            return True

    async def write(self, data: bytes) -> bool:
        """Write complete lines off the event loop; the bytes written count in `written`.

        Args:
            data: One or more complete lines.

        Returns:
            True when every byte reached the pipe; False when the turn is gone, stdin is
            closed or the write was aborted (`abort`).
        """
        self.written = 0
        self._done.clear()
        try:
            return await asyncio.to_thread(self._write, data)
        finally:
            self._done.set()

    async def abort(self) -> int:
        """Stop a write in flight after its current chunk and close the write end.

        Returns:
            The bytes of the line written before it stopped.
        """
        self._abort.set()
        await asyncio.to_thread(self._done.wait)
        self.close()
        return self.written

    def close(self) -> None:
        """Close the runner's write end (nothing is written afterwards)."""
        self._abort.set()
        with self._lock:
            if self._fd >= 0:
                with contextlib.suppress(OSError):
                    os.close(self._fd)
                self._fd = -1


class KeeperGroup:
    """The one process group of a keeper and its CLI, as the runner handles it (18.19.4).

    Args:
        files: The files of the launch.
        keeper: The keeper's identity (``pid``, ``pgid``, ``lstart``, ``command``).
        cli: The CLI's identity (``pid``, ``pgid``, ``lstart``, ``command``).
        session_ref: The session reference (it stands in the CLI's command line).
        timeout: ``runner.cli_probe_timeout`` for ``ps``.
        process: The keeper as an own child (the runner that started it reaps it); ``None``
            for an adopted turn, whose end is learnt by polling the identities.
        verified: The identities matched when this runner took the group over.
    """

    def __init__(  # noqa: PLR0913 - the parts of one launch
        self,
        files: LaunchFiles,
        *,
        keeper: Mapping[str, Any],
        cli: Mapping[str, Any],
        session_ref: str,
        timeout: float,
        process: subprocess.Popen[bytes] | None = None,
        verified: bool = False,
    ) -> None:
        """Wrap a launch; see the class docstring."""
        self.files = files
        self.keeper = dict(keeper)
        self.cli = dict(cli)
        self.session_ref = session_ref
        self.timeout = timeout
        self.process = process
        self.pid = int(cli.get("pid") or 0)
        self.pgid = int(keeper.get("pid") or 0)
        self.verified = verified or process is not None
        self.writer = PipeWriter(files.stdin)
        self._cli_alive = True
        self._checked_at = 0.0

    # ---- liveness and identity ---------------------------------------------------------------

    def keeper_alive(self) -> bool:
        """Whether the keeper still runs (an own child, or its identity still matches)."""
        if self.process is not None:
            return self.process.poll() is None
        return identity_matches(self.keeper, str(self.files.prefix), self.timeout)

    def alive(self) -> bool:
        """Whether the turn lives: the own keeper runs, or the recorded CLI still matches.

        A matching keeper without a matching CLI is a keeper about to exit (18.19.4); a CLI
        whose keeper was killed still counts.
        """
        if self.process is not None and self.process.poll() is None:
            return True
        if not self._cli_alive:
            return False
        now = time.monotonic()
        if now - self._checked_at < 0.2:  # ``ps`` at most a few times per second
            return True
        self._checked_at = now
        self._cli_alive = identity_matches(self.cli, self.session_ref, self.timeout)
        return self._cli_alive

    def may_signal(self) -> bool:
        """The identity check before a signal to the group (D-M2-35-11).

        Returns:
            True while the own keeper is not reaped or a recorded process still matches, and
            afterwards while a process of a group that matched still lives (a process group ID
            is not given to a new process while the group exists).
        """
        if self.process is not None and self.process.poll() is None:
            return True
        if identity_matches(self.cli, self.session_ref, self.timeout) or identity_matches(
            self.keeper, str(self.files.prefix), self.timeout
        ):
            self.verified = True
            return True
        return self.verified and _group_exists(self.pgid)

    # ---- stdin -------------------------------------------------------------------------------

    async def write_bytes(self, data: bytes) -> bool:
        """Write complete lines into the named pipe (see `PipeWriter.write`)."""
        return await self.writer.write(data)

    def close_stdin(self) -> None:
        """Close stdin: the runner's own write end, then SIGUSR1 to the keeper (18.19.4 item 5).

        The signal goes only to a keeper whose identity still matches.
        """
        self.writer.close()
        if self.keeper_alive():
            with contextlib.suppress(ProcessLookupError, PermissionError):
                os.kill(self.pgid, signal.SIGUSR1)

    # ---- the end -----------------------------------------------------------------------------

    def poll(self) -> int | None:
        """The CLI's exit code once known (from the keeper or ``<launch>.exit``)."""
        if self.process is not None:
            return self.process.poll()
        if self.alive():
            return None
        done = read_json(self.files.exit)
        code = done.get("code") if done else None
        return code if isinstance(code, int) else -1

    async def wait(self, timeout: float, poll: float = 0.02) -> bool:
        """Wait until the turn and its keeper ended (the keeper writes ``<launch>.exit`` last).

        Args:
            timeout: Seconds to wait at most.
            poll: Interval of the checks.

        Returns:
            True when the CLI has exited (and the keeper with it, or the time for it ran out).
        """
        deadline = time.monotonic() + timeout
        while self.alive() or self.keeper_alive():
            if time.monotonic() >= deadline:
                return not self.alive()
            await asyncio.sleep(poll)
        self.writer.close()  # the pipe of an ended process is never written again
        return True

    def exit_label(self) -> str:
        """``exit_label`` of 18.9 from ``<launch>.exit``; ``unknown`` without it (keeper killed).

        Returns:
            For example ``"0"`` or ``"SIGKILL"``; ``"running"`` while the turn runs.
        """
        done = read_json(self.files.exit)
        if done is not None and isinstance(done.get("label"), str):
            return str(done["label"])
        return "running" if self.alive() else "unknown"


@dataclass
class AttemptRecord:
    """One attempt of a run as its ``launching`` record carries it (18.19.5 step 1, 18.19.6).

    Attributes:
        number: The attempt's number (the first is 1, each recovery process adds one).
        kind: One of `ATTEMPT_KINDS`.
        try_: The launch of this attempt (a launch repeated after a crash adds one).
        files: The files of this try.
        resume: ``--resume`` instead of ``--session-id``.
        hint: The retry hint of a retry after compaction.
        new_reference: The attempt runs on a new session reference.
        bind_failed: Report ``bind_failed: session_missing`` with the new reference.
        session_ref: The session reference of the command line.
        inputs: The ``uuid`` of each initial input, in order.
        outage: The run's highest outage number reached before this attempt (18.19.10).
        worked_before: For a compaction: the failed turn before it had worked (its retry hint).
        stderr_start: Where the attempt's lines begin in the stderr file.
        go_sent: The go byte is recorded as sent (step 3).
        inputs_written: ``<launch>.fed`` was seen (step 6).
        stdin_closed: Stdin was closed (recorded before every close, 18.19.7).
        keeper: The keeper's identity once read (``pid``, ``pgid``, ``lstart``, ``command``).
        cli: The CLI's identity once read.
    """

    number: int
    kind: str
    try_: int
    files: LaunchFiles
    resume: bool = False
    hint: bool = False
    new_reference: bool = False
    bind_failed: bool = False
    session_ref: str = ""
    inputs: list[str] = field(default_factory=list)
    outage: int = 0
    worked_before: bool = False
    stderr_start: int = 0
    go_sent: bool = False
    inputs_written: bool = False
    stdin_closed: bool = False
    keeper: dict[str, Any] | None = None
    cli: dict[str, Any] | None = None

    def to_record(self) -> dict[str, Any]:
        """The members of the turn record (no text, no token).

        Returns:
            ``attempt``, ``launch``, ``keeper``, ``outage``, ``go_sent``, ``inputs_written``
            and ``stdin_closed``.
        """
        return {
            "attempt": {
                "number": self.number,
                "kind": self.kind,
                "try": self.try_,
                "resume": self.resume,
                "hint": self.hint,
                "new_reference": self.new_reference,
                "bind_failed": self.bind_failed,
                "session_ref": self.session_ref,
                "inputs": list(self.inputs),
                "worked_before": self.worked_before,
                "stderr_start": self.stderr_start,
            },
            "launch": self.files.record(),
            "keeper": self.keeper,
            "cli": self.cli,
            "outage": self.outage,
            "go_sent": self.go_sent,
            "inputs_written": self.inputs_written,
            "stdin_closed": self.stdin_closed,
        }

    @classmethod
    def from_record(cls, record: Mapping[str, Any]) -> AttemptRecord | None:
        """The attempt of a turn record; ``None`` for a record without a named pipe.

        Args:
            record: The turn record.

        Returns:
            The attempt, or ``None`` when the record has no ``launch`` (written without
            ``runner.turn_handover``) or is malformed.
        """
        launch, attempt = record.get("launch"), record.get("attempt")
        if not isinstance(launch, Mapping) or not isinstance(attempt, Mapping):
            return None
        prefix, kind = launch.get("prefix"), attempt.get("kind")
        number, try_ = attempt.get("number"), attempt.get("try")
        if not isinstance(prefix, str) or kind not in ATTEMPT_KINDS:
            return None
        if not isinstance(number, int) or not isinstance(try_, int):
            return None
        inputs = attempt.get("inputs")
        keeper, cli = record.get("keeper"), record.get("cli")
        outage = record.get("outage")
        start = attempt.get("stderr_start")
        return cls(
            number=number,
            kind=str(kind),
            try_=try_,
            files=LaunchFiles(Path(prefix)),
            resume=attempt.get("resume") is True,
            hint=attempt.get("hint") is True,
            new_reference=attempt.get("new_reference") is True,
            bind_failed=attempt.get("bind_failed") is True,
            session_ref=str(attempt.get("session_ref") or ""),
            inputs=[u for u in inputs if isinstance(u, str)] if isinstance(inputs, list) else [],
            outage=outage if isinstance(outage, int) and outage >= 0 else 0,
            worked_before=attempt.get("worked_before") is True,
            stderr_start=start if isinstance(start, int) and start >= 0 else 0,
            go_sent=record.get("go_sent") is True,
            inputs_written=record.get("inputs_written") is True,
            stdin_closed=record.get("stdin_closed") is True,
            keeper=dict(keeper) if isinstance(keeper, Mapping) else None,
            cli=dict(cli) if isinstance(cli, Mapping) else None,
        )


class Settled(Enum):
    """How the next runner continues a record in phase ``launching`` (18.19.5)."""

    RELAUNCH = "relaunch"
    """No CLI of this try ran: launch the same attempt with the next try."""
    LIVE = "live"
    """A recorded CLI lives: adopt it; nothing is launched again."""
    ENDED = "ended"
    """A CLI ran (or may have run) and ended: its end follows the adoption rule."""


async def _await_gone(alive: Callable[[], bool], timeout: float, poll: float) -> bool:
    loop = asyncio.get_running_loop()
    deadline = loop.time() + timeout
    while alive():
        if loop.time() >= deadline:
            return False
        await asyncio.sleep(poll)
    return True


async def settle_launching(  # noqa: PLR0913 - every input of the cases is explicit
    files: LaunchFiles,
    *,
    go_sent: bool,
    keeper: Mapping[str, Any] | None,
    session_ref: str,
    kill_grace: float,
    poll: float,
    timeout: float,
    after_wait: Callable[[], None] | None = None,
) -> Settled:
    """Decide a record left in phase ``launching`` by the files of its try (18.19.5).

    * ``go_sent = false``: no CLI of this try can run; a keeper named in ``<launch>.keeper`` is
      awaited until it exits (at most ``runner.kill_grace``, then SIGKILL to its group, which
      holds no CLI, after its identity check); then `Settled.RELAUNCH`.
    * ``go_sent = true`` and ``<launch>.cli`` names a live CLI: `Settled.LIVE`.
    * ``go_sent = true``, ``<launch>.cli`` present and the CLI gone: `Settled.ENDED`.
    * ``go_sent = true`` and no ``<launch>.cli``: wait for the file or the keeper's exit (at
      most ``runner.kill_grace``), SIGKILL to the keeper's group, and only after the group is
      gone read ``<launch>.cli`` once more: a file present now counts as a CLI that may have
      run; only a file still absent, or the label ``not_started``, relaunches.

    Args:
        files: The files of the recorded try.
        go_sent: ``go_sent`` of the record.
        keeper: The keeper's identity in the record (``None`` before step 3).
        session_ref: The session reference (for the CLI's identity check).
        kill_grace: ``runner.kill_grace``.
        poll: ``runner.poll_interval``.
        timeout: ``runner.cli_probe_timeout`` for ``ps``.
        after_wait: A test-only hook called after the wait, before the kill.

    Returns:
        The case.
    """
    marker = str(files.prefix)
    named = keeper if keeper is not None else read_json(files.keeper)
    named = dict(named) if named is not None else None
    if named is not None:
        named.setdefault("pgid", named.get("pid"))

    def keeper_alive() -> bool:
        return identity_matches(named, marker, timeout)

    async def kill_keeper() -> None:
        if named is not None and keeper_alive():
            with contextlib.suppress(ProcessLookupError, PermissionError):
                os.killpg(int(named["pgid"]), signal.SIGKILL)
            await _await_gone(lambda: _group_exists(int(named["pgid"])), kill_grace, poll)

    if not go_sent:
        if named is not None:
            await _await_gone(keeper_alive, kill_grace, poll)
            await kill_keeper()
        return Settled.RELAUNCH
    cli = read_json(files.cli)
    if cli is None:
        loop = asyncio.get_running_loop()
        deadline = loop.time() + kill_grace
        while read_json(files.cli) is None and keeper_alive() and loop.time() < deadline:
            await asyncio.sleep(poll)
        if after_wait is not None:
            after_wait()
        await kill_keeper()
        cli = read_json(files.cli)  # this read decides
        done = read_json(files.exit)
        if cli is None or (done is not None and done.get("label") == NOT_STARTED):
            return Settled.RELAUNCH
    ident = {**cli, "pgid": cli.get("pgid") or (named or {}).get("pid")}
    if identity_matches(ident, session_ref, timeout):
        return Settled.LIVE
    return Settled.ENDED
