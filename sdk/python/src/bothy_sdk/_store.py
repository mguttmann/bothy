# SPDX-License-Identifier: Apache-2.0
"""Private SQLite helper of the durable stores (inbox and outbox).

`open_db` creates the database file with mode ``0600`` before SQLite opens it, so the file
never exists with wider permissions, and sets the pragmas every store needs: a rollback journal
(SQLite creates the journal with the permissions of the database file) and
``synchronous = FULL`` (a committed transaction survives a power loss). `Db` keeps one
connection behind a lock and runs every call in a worker thread through `asyncio.to_thread`, so
the event loop never blocks on the disk.

Examples:
    >>> import asyncio, tempfile, pathlib
    >>> async def demo(path: pathlib.Path) -> int:
    ...     db = Db(path)
    ...     await db.run(lambda c: c.execute("CREATE TABLE t (v INTEGER)"))
    ...     await db.run(lambda c: c.execute("INSERT INTO t VALUES (1)"))
    ...     n = await db.run(lambda c: c.execute("SELECT count(*) FROM t").fetchone()[0])
    ...     await db.close()
    ...     return n
    >>> with tempfile.TemporaryDirectory() as d:
    ...     asyncio.run(demo(pathlib.Path(d) / "example.sqlite3"))
    1
"""

from __future__ import annotations

import asyncio
import os
import sqlite3
import stat
import threading
from collections.abc import Callable
from pathlib import Path
from typing import TypeVar

__all__ = ["Db", "open_db"]

T = TypeVar("T")

_PRIVATE = 0o600


def open_db(path: str | os.PathLike[str]) -> sqlite3.Connection:
    """Open (and create when absent) a SQLite database readable by its owner only.

    The file is created with ``O_CREAT | O_NOFOLLOW`` and mode ``0600`` before SQLite sees it;
    an existing file must be a regular file of mode ``0600`` or tighter. The connection runs
    in autocommit mode (`Db.run` opens the transactions) with ``journal_mode = DELETE``,
    ``synchronous = FULL`` and foreign keys on.

    Args:
        path: The database file.

    Returns:
        The connection; usable from any thread, so the caller must serialize its use.

    Raises:
        PermissionError: The file is accessible by group or others (POSIX).
        OSError: The path is a symbolic link or not a regular file, or cannot be created.
    """
    p = Path(path)
    flags = os.O_RDWR | os.O_CREAT | getattr(os, "O_NOFOLLOW", 0) | getattr(os, "O_CLOEXEC", 0)
    fd = os.open(p, flags, _PRIVATE)
    try:
        info = os.fstat(fd)
    finally:
        os.close(fd)
    if not stat.S_ISREG(info.st_mode):
        raise OSError(f"{p} is not a regular file")
    if os.name == "posix" and info.st_mode & (stat.S_IRWXG | stat.S_IRWXO):
        raise PermissionError(f"{p} must not be accessible by group or others (mode 0600)")
    conn = sqlite3.connect(p, isolation_level=None, check_same_thread=False)
    try:
        conn.execute("PRAGMA journal_mode = DELETE")
        conn.execute("PRAGMA synchronous = FULL")
        conn.execute("PRAGMA foreign_keys = ON")
    except BaseException:
        conn.close()
        raise
    return conn


class Db:
    """One SQLite connection behind a lock; every call runs in a worker thread.

    Args:
        path: The database file, opened with `open_db`.
    """

    def __init__(self, path: str | os.PathLike[str]) -> None:
        """Open the database; see the class docstring."""
        self._conn: sqlite3.Connection | None = open_db(path)
        self._lock = threading.Lock()

    async def run(self, fn: Callable[[sqlite3.Connection], T]) -> T:
        """Run ``fn(conn)`` inside one immediate transaction in a worker thread.

        The transaction commits when ``fn`` returns and rolls back when it raises; the commit
        is durable when this returns (``synchronous = FULL``).

        Args:
            fn: The work; it must not keep the connection.

        Returns:
            What ``fn`` returned.

        Raises:
            RuntimeError: The database is closed.
        """
        return await asyncio.to_thread(self._call, fn)

    def _call(self, fn: Callable[[sqlite3.Connection], T]) -> T:
        with self._lock:
            conn = self._conn
            if conn is None:
                raise RuntimeError("the database is closed")
            conn.execute("BEGIN IMMEDIATE")
            try:
                result = fn(conn)
            except BaseException:
                conn.execute("ROLLBACK")
                raise
            conn.execute("COMMIT")
            return result

    async def close(self) -> None:
        """Close the connection; later calls of `run` raise. Closing twice is harmless."""
        await asyncio.to_thread(self._close)

    def _close(self) -> None:
        with self._lock:
            if self._conn is not None:
                self._conn.close()
                self._conn = None
