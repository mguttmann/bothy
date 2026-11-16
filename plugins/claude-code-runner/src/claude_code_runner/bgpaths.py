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

Examples:
    >>> from pathlib import Path
    >>> socket_path(Path("/state"), "0192d3a0-0000-7000-8000-000000000001").parent.as_posix()
    '/state/bg'
"""

from __future__ import annotations

import contextlib
import hashlib
import os
import sys
from dataclasses import dataclass
from pathlib import Path
from uuid import UUID

__all__ = [
    "SOCKET_PATH_MAX",
    "BgFiles",
    "bg_files",
    "remove_bg_files",
    "socket_path",
    "socket_path_fits",
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
