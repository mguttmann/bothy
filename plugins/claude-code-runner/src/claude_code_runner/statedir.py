# SPDX-License-Identifier: Apache-2.0
"""Private files and directories of ``runner.state_dir`` (design notes 11 and 16).

Every directory the runner creates there has mode ``0700`` and every file ``0600``: the state
holds transcripts, prompts, downloaded attachments and the inbox with its invocation tokens.
Files older than ``runner.stream_log_retention`` under ``sessions/`` and ``files/`` are swept.

Examples:
    >>> import tempfile, pathlib
    >>> d = ensure_private_dir(pathlib.Path(tempfile.mkdtemp()) / "x")
    >>> oct(d.stat().st_mode & 0o777)
    '0o700'
"""

from __future__ import annotations

import contextlib
import os
import stat
import tempfile
import time
from pathlib import Path

__all__ = ["StateDirError", "ensure_private_dir", "sweep", "write_private"]

_OPEN_BITS = stat.S_IRWXG | stat.S_IRWXO
_SWEPT = ("sessions", "files")
# The cumulative usage of a session is state, not a recording: without it a resumed session
# would report its whole cost again (09 18.7).
_KEPT = frozenset({"usage.json"})


class StateDirError(Exception):
    """A directory of the state is not private.

    Attributes:
        code: ``not_private`` or ``not_a_directory``.
    """

    def __init__(self, code: str) -> None:
        """Create the error with its code; no path is part of the message."""
        super().__init__(code)
        self.code = code


def ensure_private_dir(path: Path) -> Path:
    """Create ``path`` and missing parents with mode ``0700``; refuse an open existing one.

    Args:
        path: The directory.

    Returns:
        ``path``.

    Raises:
        StateDirError: ``path`` exists and is not a directory, or group or others have a
            permission on it.
    """
    missing: list[Path] = []
    probe = path
    while not probe.exists():
        missing.append(probe)
        probe = probe.parent
    for directory in reversed(missing):
        with contextlib.suppress(FileExistsError):
            directory.mkdir(mode=0o700)
        directory.chmod(0o700)
    info = path.lstat()
    if not stat.S_ISDIR(info.st_mode):
        raise StateDirError("not_a_directory")
    if info.st_mode & _OPEN_BITS:
        raise StateDirError("not_private")
    return path


def write_private(path: Path, data: str | bytes, *, atomic: bool = False) -> None:
    """Write a file of mode ``0600`` (created that way, never widened afterwards).

    Args:
        path: The file; its directory must exist.
        data: The content; text is written as UTF-8.
        atomic: Write a temporary file in the same directory and rename it over ``path``, so a
            reader sees the old or the new content, never a part.
    """
    raw = data.encode("utf-8") if isinstance(data, str) else data
    if atomic:
        fd, tmp = tempfile.mkstemp(dir=path.parent, prefix=f".{path.name}.")
        try:
            with os.fdopen(fd, "wb") as fh:
                fh.write(raw)
                fh.flush()
                os.fsync(fh.fileno())
            Path(tmp).replace(path)
        except BaseException:
            with contextlib.suppress(FileNotFoundError):
                Path(tmp).unlink()
            raise
        return
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, "wb") as fh:
        fh.write(raw)


def sweep(root: Path, retention: float, now: float | None = None) -> int:
    """Delete files older than ``retention`` under ``sessions/`` and ``files/`` of ``root``.

    Directories left empty are removed too; symbolic links are never followed; the usage of a
    session (``usage.json``) is kept.

    Args:
        root: ``runner.state_dir``.
        retention: ``runner.stream_log_retention`` in seconds.
        now: The current time (tests).

    Returns:
        The number of files deleted.
    """
    limit = (time.time() if now is None else now) - retention
    deleted = 0
    for name in _SWEPT:
        base = root / name
        if not base.is_dir() or base.is_symlink():
            continue
        for dirpath, dirnames, filenames in os.walk(base, topdown=False):
            here = Path(dirpath)
            for filename in filenames:
                if filename in _KEPT:
                    continue
                item = here / filename
                with contextlib.suppress(FileNotFoundError):
                    info = item.lstat()
                    if stat.S_ISREG(info.st_mode) and info.st_mtime < limit:
                        item.unlink()
                        deleted += 1
            for dirname in dirnames:
                with contextlib.suppress(OSError):
                    (here / dirname).rmdir()  # only when empty
    return deleted
