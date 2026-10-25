# SPDX-License-Identifier: Apache-2.0
"""Private files and directories of ``runner.state_dir`` (design notes 11 and 16).

Every directory the runner creates there has mode ``0700`` and every file ``0600``: the state
holds transcripts, prompts, downloaded attachments and the inbox with its invocation tokens.
Files older than ``runner.stream_log_retention`` under ``sessions/``, ``files/``, ``outgoing/``
(staged answer attachments) and ``tmp/`` (temporary directories of runs) are swept.

Examples:
    >>> import tempfile, pathlib
    >>> d = ensure_private_dir(pathlib.Path(tempfile.mkdtemp()) / "x")
    >>> oct(d.stat().st_mode & 0o777)
    '0o700'
"""

from __future__ import annotations

import contextlib
import os
import shutil
import stat
import tempfile
import time
from collections.abc import Collection
from pathlib import Path

__all__ = ["StateDirError", "ensure_private_dir", "sweep", "write_private"]

_OPEN_BITS = stat.S_IRWXG | stat.S_IRWXO
_SWEPT = ("sessions", "files")
# One directory per run (the run's TMPDIR and its staged answer attachments, M3-12): swept as a
# whole by its newest entry, never while the run is active, so a turn's unpacked old file stays.
_SWEPT_RUNS = ("outgoing", "tmp")
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


def sweep(
    root: Path, retention: float, now: float | None = None, active: Collection[str] = ()
) -> int:
    """Delete what is older than ``retention`` under the swept directories of ``root``.

    Under ``sessions/`` and ``files/`` every regular file older than ``retention`` goes and
    directories left empty are removed; the usage of a session (``usage.json``) is kept. Under
    ``outgoing/`` and ``tmp/`` each run's directory goes as a whole once its newest entry is
    older than ``retention``, never while its run is active (``active``, or a turn record in
    ``turns/``). Symbolic links are never followed.

    Args:
        root: ``runner.state_dir``.
        retention: ``runner.stream_log_retention`` in seconds.
        now: The current time (tests).
        active: The IDs of the runs this runner works on now.

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
    return deleted + _sweep_runs(root, limit, set(active) | _recorded_runs(root))


def _sweep_runs(root: Path, limit: float, busy: set[str]) -> int:
    """Remove the run directories under ``outgoing/`` and ``tmp/`` whose newest entry is old."""
    deleted = 0
    for name in _SWEPT_RUNS:
        base = root / name
        if not base.is_dir() or base.is_symlink():
            continue
        for entry in sorted(base.iterdir()):
            if entry.name in busy or _newest(entry) >= limit:
                continue
            deleted += _count_files(entry)
            if entry.is_dir() and not entry.is_symlink():
                shutil.rmtree(entry, ignore_errors=True)
            else:
                with contextlib.suppress(OSError):
                    entry.unlink()
    return deleted


def _recorded_runs(root: Path) -> set[str]:
    """The runs with a turn record (own or left for adoption)."""
    turns = root / "turns"
    return {p.stem for p in turns.glob("*.json")} if turns.is_dir() else set()


def _newest(entry: Path) -> float:
    """The newest modification time of ``entry`` and everything below it (links not followed)."""
    newest = _mtime(entry)
    if entry.is_dir() and not entry.is_symlink():
        for dirpath, dirnames, filenames in os.walk(entry):
            for item in (*dirnames, *filenames):
                newest = max(newest, _mtime(Path(dirpath) / item))
    return newest


def _count_files(entry: Path) -> int:
    if not entry.is_dir() or entry.is_symlink():
        return 1
    return sum(len(files) for _, _, files in os.walk(entry))


def _mtime(path: Path) -> float:
    """The modification time of ``path``; now when it is gone (then nothing is removed)."""
    try:
        return path.lstat().st_mtime
    except OSError:
        return time.time()
