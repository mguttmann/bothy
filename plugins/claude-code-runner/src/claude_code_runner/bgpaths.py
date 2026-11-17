# SPDX-License-Identifier: Apache-2.0
"""The files of a background turn under ``runner.state_dir`` (09 18.2 "Background mode").

Everything a background session is started with lies in ``<runner.state_dir>/bg/`` and is named
after its run, never after its session: the CLI makes the session id of a new session itself
(18.1), and the files a live session process was started with (its settings file, its inbox
socket) must not move when the runner learns that id.

* ``<digest>.sock``: the inbox socket the runner names with ``--messaging-socket-path``. The
  system bounds the length of a socket address (`SOCKET_PATH_MAX`), so the name is the first 16
  hexadecimal digits of the SHA-256 of the run id instead of the id itself.
* ``<run_id>.settings.json``: the settings file of the run (``--settings``, mode ``0600``).
* ``<run_id>.system-prompt.md``: the system prompt file of the session
  (``--append-system-prompt-file``).
* ``<run_id>.launch.out`` and ``<run_id>.launch.err``: what the launch command printed.

One file of the mode is named after the session and outlives the run:
``<runner.state_dir>/sessions/<session id>/foreign-input`` marks a session whose conversation
took an input the runner did not write (18.5 "Whose input"). No run continues such a session,
in either launch mode (`mark_foreign_input`, `took_foreign_input`), and the retention sweep
keeps the file.

Examples:
    >>> from pathlib import Path
    >>> socket_path(Path("/state"), "0192d3a0-0000-7000-8000-000000000001").parent.as_posix()
    '/state/bg'
"""

from __future__ import annotations

import contextlib
import hashlib
import json
import os
import sys
from dataclasses import dataclass
from pathlib import Path
from uuid import UUID

from claude_code_runner.statedir import FOREIGN_INPUT_MARK, ensure_private_dir, write_private

__all__ = [
    "SOCKET_PATH_MAX",
    "BgFiles",
    "bg_files",
    "mark_foreign_input",
    "remove_bg_files",
    "socket_path",
    "socket_path_fits",
    "took_foreign_input",
]

_DIR = "bg"
_DIGEST_CHARS = 16

SOCKET_PATH_MAX = 103 if sys.platform == "darwin" else 107
"""The longest socket path in bytes: ``sun_path`` holds 104 bytes on macOS and 108 on Linux,
one of them for the final zero byte."""


def socket_path(state_dir: Path, run_id: UUID | str) -> Path:
    """The inbox socket of a run's background session.

    Args:
        state_dir: ``runner.state_dir``.
        run_id: The run.

    Returns:
        ``<runner.state_dir>/bg/<digest>.sock`` (nothing is created).
    """
    digest = hashlib.sha256(str(run_id).encode("utf-8")).hexdigest()[:_DIGEST_CHARS]
    return state_dir / _DIR / f"{digest}.sock"


def socket_path_fits(state_dir: Path) -> bool:
    """Whether the inbox socket of a run under ``state_dir`` is a valid socket address.

    Args:
        state_dir: ``runner.state_dir``.

    Returns:
        False when the path is longer than `SOCKET_PATH_MAX` bytes.

    Examples:
        >>> from pathlib import Path
        >>> socket_path_fits(Path("/var/lib/runner")), socket_path_fits(Path("/" + "x" * 120))
        (True, False)
    """
    return len(os.fsencode(socket_path(state_dir, ""))) <= SOCKET_PATH_MAX


@dataclass(frozen=True)
class BgFiles:
    """The files of one background turn.

    Attributes:
        socket: The inbox socket.
        settings: The settings file of the run.
        system_prompt: The system prompt file of the session.
        stdout: What the launch command wrote to stdout.
        stderr: What the launch command wrote to stderr.
    """

    socket: Path
    settings: Path
    system_prompt: Path
    stdout: Path
    stderr: Path


def bg_files(state_dir: Path, run_id: UUID | str) -> BgFiles:
    """Return the file paths of a run's background turn.

    Args:
        state_dir: ``runner.state_dir``.
        run_id: The run.

    Returns:
        The paths (nothing is created).
    """
    folder = state_dir / _DIR
    return BgFiles(
        socket=socket_path(state_dir, run_id),
        settings=folder / f"{run_id}.settings.json",
        system_prompt=folder / f"{run_id}.system-prompt.md",
        stdout=folder / f"{run_id}.launch.out",
        stderr=folder / f"{run_id}.launch.err",
    )


def remove_bg_files(state_dir: Path, run_id: UUID | str) -> None:
    """Remove the files of a run's background turn once the run left the runner.

    Args:
        state_dir: ``runner.state_dir``.
        run_id: The run.
    """
    files = bg_files(state_dir, run_id)
    for path in (files.socket, files.settings, files.system_prompt, files.stdout, files.stderr):
        with contextlib.suppress(OSError):
            path.unlink()


def _foreign_input_mark(state_dir: Path, session_ref: str) -> Path:
    return state_dir / "sessions" / session_ref / FOREIGN_INPUT_MARK


def mark_foreign_input(state_dir: Path, session_ref: str, run_id: UUID | str) -> None:
    """Mark a session whose conversation took an input the runner did not write (09 18.5).

    The foreign input stays in the conversation: a process that continues the session gives
    it to the model again with its next request. So the session is never continued, by no
    run and in no launch mode (`took_foreign_input`). The mark names the run that found the
    input and nothing of the input itself.

    Args:
        state_dir: ``runner.state_dir``.
        session_ref: The session.
        run_id: The run whose session took the input.

    Raises:
        OSError: The file could not be written.
        claude_code_runner.statedir.StateDirError: The session's folder is not private.
    """
    path = _foreign_input_mark(state_dir, session_ref)
    ensure_private_dir(path.parent)
    write_private(path, json.dumps({"run_id": str(run_id)}) + "\n", atomic=True)


def took_foreign_input(state_dir: Path, session_ref: str) -> bool:
    """Whether a session carries the mark of `mark_foreign_input`.

    Only the presence of the name counts, whatever the entry is or holds.

    Args:
        state_dir: ``runner.state_dir``.
        session_ref: The session.

    Returns:
        True for a marked session.

    Examples:
        >>> import tempfile
        >>> state = Path(tempfile.mkdtemp())
        >>> took_foreign_input(state, "s")
        False
        >>> mark_foreign_input(state, "s", "run-1")
        >>> took_foreign_input(state, "s"), took_foreign_input(state, "another")
        (True, False)
    """
    return os.path.lexists(_foreign_input_mark(state_dir, session_ref))
