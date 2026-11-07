# SPDX-License-Identifier: Apache-2.0
"""The keeper of one launch: the CLI's parent that outlives the runner (09 18.19.4, 18.19.5).

With ``runner.turn_handover`` the runner starts ``python -m claude_code_runner.keeper --launch
<launch> --go-fd <fd> [--compaction]`` with ``start_new_session=True`` instead of the CLI. The
keeper leads its own session and process group, outside the service's group, so a restart of the
runner leaves it and its CLI alive. ``<launch>`` is
``<runner.state_dir>/turns/<run>.<attempt>.<try>`` and names every file of the launch:

* ``<launch>.spec`` (mode ``0600``, written by the runner): the CLI's argument list, environment,
  working folder, stream and stderr files and lock descriptors; read once and removed at once,
  because the environment can hold the API key of ``organization``.
* ``<launch>.stdin``: the named pipe the runner created; the keeper opens it for reading without
  blocking, then for writing (it holds this write end for the turn's life), then switches the read
  end to blocking. The CLI's stdin is an anonymous pipe the keeper relays the named pipe into:
  the pinned CLI does not see the end of input of a named pipe (09 18.19.12), so the keeper closes
  the CLI's stdin once every write end of the named pipe is closed.
* ``<launch>.keeper`` and ``<launch>.cli``: the identity files (``pid`` and ``pgid``), written
  atomically by rename; the CLI is released from its release pipe only after ``<launch>.cli``.
* ``<launch>.inputs``: the attempt's initial input lines; written once into the CLI's stdin before
  any relayed line, without blocking, in a loop that also watches the CLI's exit, and
  ``<launch>.fed`` after the last byte.
* ``<launch>.exit``: the CLI's wait status (``status``, ``code``, ``label``), or the label
  ``not_started`` when no CLI started.

The keeper ignores SIGINT, SIGTERM, SIGHUP and SIGPIPE, so the stop of 18.6 (signals to the one
group) reaches the CLI and its background tasks while the keeper waits and reports; SIGKILL ends
all of them. SIGUSR1 closes the keeper's write end ("close stdin"). It exits with the CLI's exit
code, or 128 plus the signal number, and never outlives the CLI. It holds no lock descriptor after
the fork. This module imports nothing of the runner's asynchronous code.

Examples:
    >>> exit_label(0), exit_label(256), exit_label(9)
    ('0', '1', 'SIGKILL')
"""

from __future__ import annotations

import argparse
import contextlib
import errno
import fcntl
import json
import os
import select
import signal
import sys
from collections.abc import Mapping, Sequence
from pathlib import Path
from typing import Any

__all__ = ["NOT_STARTED", "exit_code", "exit_label", "main", "write_atomic"]

NOT_STARTED = "not_started"
"""The exit label of a launch whose keeper started no CLI."""

_IGNORED = (signal.SIGINT, signal.SIGTERM, signal.SIGHUP, signal.SIGPIPE)
_RESET = (*_IGNORED, signal.SIGUSR1, signal.SIGCHLD)
_CHUNK = 65536
_CHILD_FAILED = 127
"""The exit of a child that got no release or could not exec (no CLI ran)."""


def write_atomic(path: Path, data: Mapping[str, Any]) -> None:
    """Write a small JSON file atomically by rename, mode ``0600``.

    Args:
        path: The target.
        data: The JSON object.
    """
    tmp = path.with_name(path.name + ".tmp")
    fd = os.open(tmp, os.O_WRONLY | os.O_CREAT | os.O_TRUNC | os.O_NOFOLLOW, 0o600)
    try:
        os.write(fd, json.dumps(dict(data), sort_keys=True).encode("utf-8"))
        os.fsync(fd)
    finally:
        os.close(fd)
    tmp.replace(path)


def exit_code(status: int) -> int:
    """The keeper's exit code for a CLI's wait status: its code, or 128 plus the signal.

    Args:
        status: The raw wait status.

    Returns:
        The exit code.
    """
    code = os.waitstatus_to_exitcode(status)
    return 128 - code if code < 0 else code


def exit_label(status: int) -> str:
    """The exit label of 18.9 for a wait status: the code as text or the signal's name.

    Args:
        status: The raw wait status.

    Returns:
        For example ``"0"``, ``"1"`` or ``"SIGKILL"``.
    """
    code = os.waitstatus_to_exitcode(status)
    if code < 0:
        with contextlib.suppress(ValueError):
            return signal.Signals(-code).name
        return str(128 - code)
    return str(code)


class _Keeper:
    """The state of one keeper process (see the module docstring)."""

    def __init__(self, launch: Path, go_fd: int, compaction: bool) -> None:
        self.launch = launch
        self.go_fd = go_fd
        self.compaction = compaction
        self.read_end = -1
        self.write_end = -1
        self.usr1 = False
        self.status: int | None = None
        self.child = 0
        self.cli_in = -1
        """The write end of the CLI's stdin pipe; the keeper relays the named pipe into it."""
        self.pending = b""

    def path(self, suffix: str) -> Path:
        return Path(f"{self.launch}.{suffix}")

    # ---- signals ---------------------------------------------------------------------------

    def signals(self) -> int:
        """Ignore the stop signals; SIGCHLD and SIGUSR1 wake the loops through a pipe."""
        for sig in _IGNORED:
            signal.signal(sig, signal.SIG_IGN)
        wake_r, wake_w = os.pipe()
        os.set_blocking(wake_w, False)
        os.set_blocking(wake_r, False)
        signal.set_wakeup_fd(wake_w)
        signal.signal(signal.SIGCHLD, lambda *_: None)
        signal.signal(signal.SIGUSR1, self._on_usr1)
        return wake_r

    def _on_usr1(self, *_: object) -> None:
        self.usr1 = True

    # ---- steps -----------------------------------------------------------------------------

    def not_started(self) -> int:
        write_atomic(self.path("exit"), {"label": NOT_STARTED})
        return 0

    def open_pipe(self) -> bool:
        fifo = self.path("stdin")
        try:
            self.read_end = os.open(fifo, os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW)
            self.write_end = os.open(fifo, os.O_WRONLY | os.O_NOFOLLOW)
            os.set_blocking(self.read_end, True)
        except OSError:
            return False
        return True

    def spec(self) -> dict[str, Any] | None:
        path = self.path("spec")
        try:
            raw = path.read_bytes()
        except OSError:
            return None
        finally:
            with contextlib.suppress(OSError):
                path.unlink()  # the environment may hold a key: never left on disk
        try:
            spec = json.loads(raw)
        except ValueError:
            return None
        return spec if isinstance(spec, dict) else None

    def wait_for_go(self) -> bool:
        while True:
            try:
                data = os.read(self.go_fd, 1)
            except InterruptedError:  # pragma: no cover - PEP 475 retries
                continue
            break
        os.close(self.go_fd)
        return bool(data)

    def fork(self, spec: Mapping[str, Any]) -> None:
        locks = [int(fd) for fd in spec.get("lock_fds") or []]
        release_r, release_w = os.pipe()
        cli_r, cli_w = os.pipe()  # the CLI's stdin: an anonymous pipe the keeper fills
        child = os.fork()
        if child == 0:  # pragma: no cover - the child execs or exits; seen through its CLI
            os.close(release_w)
            os.close(cli_w)
            self._child(spec, release_r, cli_r, locks)
        os.close(release_r)
        os.close(cli_r)
        self.cli_in = cli_w
        os.set_blocking(self.cli_in, False)
        os.set_blocking(self.read_end, False)
        self.child = child
        # The keeper keeps no lock descriptor after the fork (18.19.4 item 2).
        for fd in locks:
            with contextlib.suppress(OSError):
                os.close(fd)
        write_atomic(self.path("cli"), {"pid": child, "pgid": os.getpgid(0)})
        os.write(release_w, b"r")
        os.close(release_w)

    def _child(  # pragma: no cover - runs in the forked child only
        self, spec: Mapping[str, Any], release_r: int, stdin: int, locks: Sequence[int]
    ) -> None:
        try:
            signal.set_wakeup_fd(-1)
            for sig in _RESET:
                signal.signal(sig, signal.SIG_DFL)
            if not os.read(release_r, 1):
                os._exit(_CHILD_FAILED)  # no release: no exec
            os.dup2(stdin, 0)
            flags = os.O_WRONLY | os.O_CREAT | os.O_APPEND | os.O_NOFOLLOW
            os.dup2(os.open(str(spec["stdout"]), flags, 0o600), 1)
            os.dup2(os.open(str(spec["stderr"]), flags, 0o600), 2)
            for fd in locks:
                os.set_inheritable(fd, True)
            os.chdir(str(spec["cwd"]))
            argv = [str(a) for a in spec["argv"]]
            env = {str(k): str(v) for k, v in dict(spec["env"]).items()}
            os.execvpe(argv[0], argv, env)  # noqa: S606  # nosec B606 - argument list, no shell
        except BaseException:
            os._exit(_CHILD_FAILED)

    def reap(self) -> bool:
        """Whether the CLI has exited (its status kept)."""
        if self.status is not None:
            return True
        try:
            pid, status = os.waitpid(self.child, os.WNOHANG)
        except ChildProcessError:  # pragma: no cover - only the keeper reaps its child
            return True
        if pid == 0:
            return False
        self.status = status
        return True

    def close_write_end(self) -> None:
        if self.write_end >= 0:
            with contextlib.suppress(OSError):
                os.close(self.write_end)
            self.write_end = -1

    def feed(self, wake_r: int) -> None:
        """Write the initial inputs without blocking; ``<launch>.fed`` after the last byte."""
        try:
            data = self.path("inputs").read_bytes()
        except OSError:
            data = b""
        view = memoryview(data)
        while view:
            if self.reap() or self.cli_in < 0:
                return  # the CLI ended first: no <launch>.fed
            try:
                view = view[os.write(self.cli_in, view[:_CHUNK]) :]
                continue
            except BlockingIOError:
                pass
            except OSError as err:
                if err.errno == errno.EPIPE:
                    self.close_cli_in()
                    return
                raise
            select.select([wake_r], [self.cli_in], [], 1.0)
            _drain(wake_r)
            if self.usr1:
                self.close_write_end()
        write_atomic(self.path("fed"), {"bytes": len(data)})
        if self.compaction:
            self.close_write_end()  # the compaction's stdin closes after /compact

    def close_cli_in(self) -> None:
        if self.cli_in >= 0:
            with contextlib.suppress(OSError):
                os.close(self.cli_in)
            self.cli_in = -1

    def _relay(self) -> None:
        """Move what the named pipe holds into the CLI's stdin; its end closes the CLI's."""
        if self.read_end >= 0 and not self.pending:
            try:
                data = os.read(self.read_end, _CHUNK)
            except BlockingIOError:
                data = None
            if data == b"":
                # Every write end of the named pipe is closed (the keeper's after SIGUSR1 or a
                # compaction, and the runner's): the CLI's input ends.
                os.close(self.read_end)
                self.read_end = -1
                self.close_cli_in()
            elif data:
                self.pending = data
        while self.pending and self.cli_in >= 0:
            try:
                self.pending = self.pending[os.write(self.cli_in, self.pending) :]
            except BlockingIOError:
                return
            except OSError:
                self.pending = b""  # EPIPE: the CLI is gone
                self.close_cli_in()

    def wait(self, wake_r: int) -> int:
        while not self.reap():
            if self.usr1:
                self.close_write_end()
            self._relay()
            readers = [wake_r] + (
                [self.read_end] if self.read_end >= 0 and not self.pending else []
            )
            writers = [self.cli_in] if self.pending and self.cli_in >= 0 else []
            select.select(readers, writers, [], 1.0)
            _drain(wake_r)
        status = self.status if self.status is not None else 0
        write_atomic(
            self.path("exit"),
            {"status": status, "code": exit_code(status), "label": exit_label(status)},
        )
        return exit_code(status)


def _drain(fd: int) -> None:
    with contextlib.suppress(BlockingIOError, InterruptedError):
        while os.read(fd, 512):
            pass


def run(launch: Path, go_fd: int, *, compaction: bool) -> int:
    """Run the keeper of one launch (see the module docstring).

    Args:
        launch: ``<runner.state_dir>/turns/<run>.<attempt>.<try>``.
        go_fd: The read end of the launch pipe; its write end only the runner holds.
        compaction: The attempt is a compaction: the write end closes after its inputs.

    Returns:
        The exit code: the CLI's, 128 plus its signal, or 0 when no CLI started.
    """
    keeper = _Keeper(launch, go_fd, compaction)
    wake_r = keeper.signals()
    spec = keeper.spec()
    if spec is None or not keeper.open_pipe():
        os.close(go_fd)
        return keeper.not_started()
    write_atomic(keeper.path("keeper"), {"pid": os.getpid(), "pgid": os.getpgid(0)})
    if not keeper.wait_for_go():
        return keeper.not_started()
    try:
        keeper.fork(spec)
    except OSError:
        return keeper.not_started()
    if keeper.usr1:
        keeper.close_write_end()
    keeper.feed(wake_r)
    return keeper.wait(wake_r)


def main(argv: Sequence[str] | None = None) -> int:
    """Entry point of ``python -m claude_code_runner.keeper``.

    Args:
        argv: ``--launch <path> --go-fd <fd> [--compaction]``.

    Returns:
        The exit code of `run`.
    """
    parser = argparse.ArgumentParser(prog="claude_code_runner.keeper")
    parser.add_argument("--launch", required=True, type=Path)
    parser.add_argument("--go-fd", required=True, type=int)
    parser.add_argument("--compaction", action="store_true")
    args = parser.parse_args(argv)
    flags = fcntl.fcntl(args.go_fd, fcntl.F_GETFD)
    fcntl.fcntl(args.go_fd, fcntl.F_SETFD, flags | fcntl.FD_CLOEXEC)  # never to the CLI
    return run(args.launch, args.go_fd, compaction=args.compaction)


if __name__ == "__main__":  # pragma: no cover - started as a program
    sys.exit(main())
