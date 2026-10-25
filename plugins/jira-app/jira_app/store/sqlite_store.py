# SPDX-License-Identifier: Apache-2.0
"""`Store` on SQLite (brief M2-50 Design note 9).

The file ``state.sqlite3`` in ``jira_app.state_dir`` with ``PRAGMA journal_mode = WAL`` and
``PRAGMA synchronous = FULL``, one connection used through `asyncio.to_thread` under one
`asyncio.Lock`. Files get mode ``0600`` through the umask ``0o077`` the process sets at start;
the store sets the mode of its files explicitly as well. The lock file ``lock`` in the state
directory is held with ``fcntl.flock(LOCK_EX | LOCK_NB)`` for the life of the store; a second
process fails its start with `StoreLockedError` naming the directory. The kernel releases the
lock when the process ends, so no stale-lock rule is needed.

`SqlStore` holds the SQL shared with the in-memory form `jira_app.store.memory_store.MemoryStore`.

Examples:
    >>> import asyncio, tempfile
    >>> async def demo(path: str) -> bool:
    ...     store = SqliteStore(path, now=lambda: 0.0, window=60.0)
    ...     await store.open()
    ...     await store.record_created("issue", "10001")
    ...     found = await store.is_created("issue", "10001")
    ...     await store.close()
    ...     return found
    >>> with tempfile.TemporaryDirectory() as d:
    ...     asyncio.run(demo(d))
    True
"""

from __future__ import annotations

import asyncio
import contextlib
import fcntl
import json
import os
import sqlite3
from collections.abc import Callable
from pathlib import Path
from typing import Any, TypeVar

from jira_app.store.base import (
    AdmitResult,
    IssueDraft,
    JobRecord,
    MappedIssue,
    NewJob,
    SeenKey,
    StoreError,
    StoreLockedError,
    ThreadRef,
)

__all__ = ["DATABASE_FILE", "LOCK_FILE", "SCHEMA", "SqlStore", "SqliteStore"]

DATABASE_FILE = "state.sqlite3"
"""The database in the state directory."""

LOCK_FILE = "lock"
"""The lock file in the state directory."""

_T = TypeVar("_T")

SCHEMA = """
CREATE TABLE IF NOT EXISTS threads (
  issue_id TEXT PRIMARY KEY,
  issue_key TEXT NOT NULL,
  thread_id TEXT NOT NULL UNIQUE,
  channel_id TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS drafts (
  draft_id TEXT PRIMARY KEY,
  thread_id TEXT NOT NULL,
  channel_id TEXT NOT NULL,
  card_message_id TEXT NOT NULL,
  title TEXT NOT NULL,
  projects TEXT NOT NULL,
  created_at REAL NOT NULL
);
CREATE INDEX IF NOT EXISTS drafts_card ON drafts (card_message_id);
CREATE TABLE IF NOT EXISTS created (
  kind TEXT NOT NULL,
  object_id TEXT NOT NULL,
  created_at REAL NOT NULL,
  PRIMARY KEY (kind, object_id)
);
CREATE TABLE IF NOT EXISTS seen (
  kind TEXT NOT NULL,
  value TEXT NOT NULL,
  status INTEGER NOT NULL,
  seen_at REAL NOT NULL,
  PRIMARY KEY (kind, value)
);
CREATE TABLE IF NOT EXISTS jobs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  kind TEXT NOT NULL,
  data TEXT NOT NULL,
  progress TEXT NOT NULL,
  created_at REAL NOT NULL
);
"""


_DRAFT_COLUMNS = (
    "SELECT draft_id, thread_id, channel_id, card_message_id, title, projects, created_at "
    "FROM drafts WHERE "
)
_DRAFT_BY = {
    "draft_id": _DRAFT_COLUMNS + "draft_id = ? AND created_at >= ?",
    "card_message_id": _DRAFT_COLUMNS + "card_message_id = ? AND created_at >= ?",
}


class SqlStore:
    """The SQL of the store over one SQLite connection.

    Args:
        now: Wall time in Unix seconds (the app's clock).
        window: ``jira_app.dedupe_window`` in seconds.
    """

    def __init__(self, now: Callable[[], float], window: float) -> None:
        """Create the store; nothing is opened before `open`."""
        self._now = now
        self._window = window
        self._lock = asyncio.Lock()
        self._db: sqlite3.Connection | None = None

    def _connect(self) -> sqlite3.Connection:
        raise NotImplementedError

    def _release(self) -> None:
        """Release what `_connect` took besides the connection."""

    async def open(self) -> None:
        """Open the database and create the tables.

        Raises:
            StoreLockedError: Another process holds the lock.
            StoreError: The database cannot be opened.
        """
        try:
            db = await asyncio.to_thread(self._connect)
        except sqlite3.Error as exc:
            self._release()
            raise StoreError("the state database cannot be opened") from exc
        self._db = db

    async def close(self) -> None:
        """Close the database and release the lock."""
        async with self._lock:
            db, self._db = self._db, None
            if db is not None:
                with contextlib.suppress(sqlite3.Error):
                    await asyncio.to_thread(db.close)
            self._release()

    async def _run(self, fn: Callable[[sqlite3.Connection], _T]) -> _T:
        async with self._lock:
            db = self._db
            if db is None:
                raise StoreError("the store is closed")
            try:
                return await asyncio.to_thread(_transaction, db, fn)
            except sqlite3.Error as exc:
                raise StoreError("the state database failed") from exc

    def _prune(self, db: sqlite3.Connection) -> None:
        cutoff = self._now() - self._window
        db.execute("DELETE FROM seen WHERE seen_at < ?", (cutoff,))
        db.execute("DELETE FROM created WHERE created_at < ?", (cutoff,))
        db.execute("DELETE FROM drafts WHERE created_at < ?", (cutoff,))

    async def admit(
        self, seen: SeenKey, status: int, job: NewJob | None, max_jobs: int
    ) -> AdmitResult:
        """See `jira_app.store.base.Store.admit`."""

        def go(db: sqlite3.Connection) -> AdmitResult:
            self._prune(db)
            row = db.execute(
                "SELECT status FROM seen WHERE kind = ? AND value = ?", (seen.kind, seen.value)
            ).fetchone()
            if row is not None:
                return AdmitResult("duplicate", int(row[0]))
            job_id = 0
            if job is not None:
                (count,) = db.execute("SELECT COUNT(*) FROM jobs").fetchone()
                if count >= max_jobs:
                    return AdmitResult("full", 503)
                cursor = db.execute(
                    "INSERT INTO jobs (kind, data, progress, created_at) VALUES (?, ?, '{}', ?)",
                    (job.kind, json.dumps(job.data), self._now()),
                )
                job_id = int(cursor.lastrowid or 0)
            db.execute(
                "INSERT INTO seen (kind, value, status, seen_at) VALUES (?, ?, ?, ?)",
                (seen.kind, seen.value, status, self._now()),
            )
            return AdmitResult("accepted", status, job_id)

        return await self._run(go)

    async def seen_status(self, seen: SeenKey) -> int | None:
        """See `jira_app.store.base.Store.seen_status`."""

        def go(db: sqlite3.Connection) -> int | None:
            row = db.execute(
                "SELECT status FROM seen WHERE kind = ? AND value = ? AND seen_at >= ?",
                (seen.kind, seen.value, self._now() - self._window),
            ).fetchone()
            return None if row is None else int(row[0])

        return await self._run(go)

    async def next_job(self) -> JobRecord | None:
        """See `jira_app.store.base.Store.next_job`."""

        def go(db: sqlite3.Connection) -> JobRecord | None:
            row = db.execute(
                "SELECT id, kind, data, progress, created_at FROM jobs ORDER BY id LIMIT 1"
            ).fetchone()
            if row is None:
                return None
            return JobRecord(
                int(row[0]), str(row[1]), json.loads(row[2]), json.loads(row[3]), float(row[4])
            )

        return await self._run(go)

    async def count_jobs(self) -> int:
        """Return the number of queued jobs."""

        def go(db: sqlite3.Connection) -> int:
            (count,) = db.execute("SELECT COUNT(*) FROM jobs").fetchone()
            return int(count)

        return await self._run(go)

    async def save_progress(self, job: JobRecord) -> None:
        """See `jira_app.store.base.Store.save_progress`."""
        text = json.dumps(job.progress)
        await self._run(
            lambda db: db.execute("UPDATE jobs SET progress = ? WHERE id = ?", (text, job.id))
        )

    async def remove_job(self, job_id: int) -> None:
        """See `jira_app.store.base.Store.remove_job`."""

        def go(db: sqlite3.Connection) -> None:
            db.execute("DELETE FROM jobs WHERE id = ?", (job_id,))
            self._prune(db)

        await self._run(go)

    async def thread_of_issue(self, issue_id: str) -> ThreadRef | None:
        """See `jira_app.store.base.Store.thread_of_issue`."""

        def go(db: sqlite3.Connection) -> ThreadRef | None:
            row = db.execute(
                "SELECT thread_id, channel_id FROM threads WHERE issue_id = ?", (issue_id,)
            ).fetchone()
            return None if row is None else ThreadRef(str(row[0]), str(row[1]))

        return await self._run(go)

    async def issue_of_thread(self, thread_id: str) -> MappedIssue | None:
        """See `jira_app.store.base.Store.issue_of_thread`."""

        def go(db: sqlite3.Connection) -> MappedIssue | None:
            row = db.execute(
                "SELECT issue_id, issue_key FROM threads WHERE thread_id = ?", (thread_id,)
            ).fetchone()
            return None if row is None else MappedIssue(str(row[0]), str(row[1]))

        return await self._run(go)

    async def map_thread(self, issue_id: str, issue_key: str, ref: ThreadRef) -> None:
        """See `jira_app.store.base.Store.map_thread`."""

        def go(db: sqlite3.Connection) -> None:
            db.execute(
                "DELETE FROM threads WHERE issue_id = ? OR thread_id = ?", (issue_id, ref.thread_id)
            )
            db.execute(
                "INSERT INTO threads (issue_id, issue_key, thread_id, channel_id) "
                "VALUES (?, ?, ?, ?)",
                (issue_id, issue_key, ref.thread_id, ref.channel_id),
            )
            self._prune(db)

        await self._run(go)

    async def set_issue_key(self, issue_id: str, issue_key: str) -> None:
        """See `jira_app.store.base.Store.set_issue_key`."""
        await self._run(
            lambda db: db.execute(
                "UPDATE threads SET issue_key = ? WHERE issue_id = ?", (issue_key, issue_id)
            )
        )

    async def unmap_thread(self, issue_id: str, thread_id: str) -> None:
        """See `jira_app.store.base.Store.unmap_thread`."""
        await self._run(
            lambda db: db.execute(
                "DELETE FROM threads WHERE issue_id = ? AND thread_id = ?", (issue_id, thread_id)
            )
        )

    async def record_created(self, kind: str, object_id: str) -> None:
        """See `jira_app.store.base.Store.record_created`."""

        def go(db: sqlite3.Connection) -> None:
            self._prune(db)
            db.execute(
                "INSERT OR REPLACE INTO created (kind, object_id, created_at) VALUES (?, ?, ?)",
                (kind, object_id, self._now()),
            )

        await self._run(go)

    async def is_created(self, kind: str, object_id: str) -> bool:
        """See `jira_app.store.base.Store.is_created`."""

        def go(db: sqlite3.Connection) -> bool:
            row = db.execute(
                "SELECT 1 FROM created WHERE kind = ? AND object_id = ? AND created_at >= ?",
                (kind, object_id, self._now() - self._window),
            ).fetchone()
            return row is not None

        return await self._run(go)

    async def put_draft(self, draft: IssueDraft) -> None:
        """See `jira_app.store.base.Store.put_draft`."""

        def go(db: sqlite3.Connection) -> None:
            self._prune(db)
            db.execute(
                "INSERT OR REPLACE INTO drafts (draft_id, thread_id, channel_id, card_message_id, "
                "title, projects, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
                (
                    draft.draft_id,
                    draft.thread_id,
                    draft.channel_id,
                    draft.card_message_id,
                    draft.title,
                    json.dumps(list(draft.projects)),
                    draft.created_at,
                ),
            )

        await self._run(go)

    def _draft(self, row: Any) -> IssueDraft:
        return IssueDraft(
            str(row[0]),
            str(row[1]),
            str(row[2]),
            str(row[3]),
            str(row[4]),
            tuple(json.loads(row[5])),
            float(row[6]),
        )

    async def get_draft(self, draft_id: str) -> IssueDraft | None:
        """See `jira_app.store.base.Store.get_draft`."""
        return await self._find_draft("draft_id", draft_id)

    async def draft_by_message(self, message_id: str) -> IssueDraft | None:
        """See `jira_app.store.base.Store.draft_by_message`."""
        return await self._find_draft("card_message_id", message_id)

    async def _find_draft(self, column: str, value: str) -> IssueDraft | None:
        query = _DRAFT_BY[column]

        def go(db: sqlite3.Connection) -> IssueDraft | None:
            row = db.execute(query, (value, self._now() - self._window)).fetchone()
            return None if row is None else self._draft(row)

        return await self._run(go)

    async def delete_draft(self, draft_id: str) -> None:
        """See `jira_app.store.base.Store.delete_draft`."""
        await self._run(lambda db: db.execute("DELETE FROM drafts WHERE draft_id = ?", (draft_id,)))


def _transaction(db: sqlite3.Connection, fn: Callable[[sqlite3.Connection], _T]) -> _T:
    with db:
        return fn(db)


class SqliteStore(SqlStore):
    """The store in ``jira_app.state_dir`` with the process lock.

    Args:
        directory: The state directory (checked by `jira_app.config.check_state_dir`).
        now: Wall time in Unix seconds.
        window: ``jira_app.dedupe_window`` in seconds.
    """

    def __init__(self, directory: str | os.PathLike[str], now: Callable[[], float], window: float):
        """Create the store; see the class docstring."""
        super().__init__(now, window)
        self._dir = Path(directory)
        self._lock_fd: int | None = None

    def _take_lock(self) -> None:
        path = self._dir / LOCK_FILE
        fd = os.open(path, os.O_RDWR | os.O_CREAT, 0o600)
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except OSError as exc:
            os.close(fd)
            raise StoreLockedError(
                f"jira_app.state_dir ({self._dir}): another process uses this directory"
            ) from exc
        self._lock_fd = fd

    def _connect(self) -> sqlite3.Connection:
        self._take_lock()
        path = self._dir / DATABASE_FILE
        db = sqlite3.connect(path, check_same_thread=False, isolation_level=None)
        try:
            db.execute("PRAGMA journal_mode = WAL")
            db.execute("PRAGMA synchronous = FULL")
            db.isolation_level = "DEFERRED"
            with db:
                db.executescript(SCHEMA)
        except sqlite3.Error:
            db.close()
            raise
        for name in (DATABASE_FILE, DATABASE_FILE + "-wal", DATABASE_FILE + "-shm"):
            with contextlib.suppress(FileNotFoundError):
                (self._dir / name).chmod(0o600)
        return db

    def _release(self) -> None:
        fd, self._lock_fd = self._lock_fd, None
        if fd is not None:
            os.close(fd)

    async def open(self) -> None:
        """Take the lock and open the database.

        Raises:
            StoreLockedError: Another process holds the lock; the message names the directory.
            StoreError: The database cannot be opened.
        """
        try:
            await super().open()
        except OSError as exc:
            self._release()
            raise StoreError(f"jira_app.state_dir ({self._dir}): cannot be opened") from exc
