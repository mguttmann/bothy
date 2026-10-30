# SPDX-License-Identifier: Apache-2.0
"""The adapter's own durable state: resolved callbacks and open cards (09 4.3, 4.9, 15).

Inbox, ``jti`` store and outbox are the SDK's files in the state directory; this module keeps
only what the Hermes side adds, in one SQLite file through the standard library:

* the callback IDs already resolved, so a redelivered interaction resolves nothing (AG-15);
* the open card per run (its kind, Hermes ID, Hermes session key, card message and component
  IDs);
* the Hermes session ID of each thread as Hermes last reported it (and its parent in a
  compression chain), so a new process still knows which threads hold a session (09 9.1);
* a digest of each answer the adapter took over for a run (thread and text, SHA-256), so a
  reply Hermes' delivery ledger sends again after a failure or a restart is recognised and not
  posted a second time (09 17.3, HERMES-E2E-REAL-SERVER).

No content is stored: no question, no answer text (only its digest), no label (BR-13, SEC-LOG-4
applied to the adapter); the texts of an open card live only in the process. Every write is
committed before the Hermes callback or the send it guards (``synchronous = FULL``). The file is
created with mode ``0600`` in a directory the SDK requires to be closed to group and others.

Examples:
    >>> import asyncio, tempfile, pathlib
    >>> async def demo() -> bool:
    ...     store = CallbackStore(pathlib.Path(tempfile.mkdtemp()) / "hermes.sqlite3")
    ...     first = await store.claim("cl:a:0")
    ...     second = await store.claim("cl:a:0")
    ...     await store.close()
    ...     return first and not second
    >>> asyncio.run(demo())
    True
"""

from __future__ import annotations

import asyncio
import hashlib
import json
import os
import sqlite3
import time
from pathlib import Path
from typing import Any
from uuid import UUID

__all__ = ["ANSWER_RETENTION_SECONDS", "CallbackStore", "CardRecord", "answer_digest"]

ANSWER_RETENTION_SECONDS = 8 * 24 * 60 * 60
"""How long an answer digest is kept: longer than Hermes keeps a delivery obligation."""

_SCHEMA = (
    "CREATE TABLE IF NOT EXISTS resolved (callback_id TEXT PRIMARY KEY)",
    "CREATE TABLE IF NOT EXISTS open_cards (run_id TEXT PRIMARY KEY, record TEXT NOT NULL)",
    "CREATE TABLE IF NOT EXISTS sessions (thread_id TEXT PRIMARY KEY, session_id TEXT NOT NULL,"
    " parent_id TEXT)",
    "CREATE TABLE IF NOT EXISTS answers (digest TEXT PRIMARY KEY, at REAL NOT NULL)",
)


def answer_digest(thread_id: UUID, text: str) -> str:
    """The digest of an answer in a thread (never the text itself).

    Examples:
        >>> len(answer_digest(UUID(int=1), "Done."))
        64
    """
    return hashlib.sha256(f"{thread_id}\n{text}".encode()).hexdigest()


CardRecord = dict[str, Any]
"""The stored form of an open card: identifiers only."""


class CallbackStore:
    """Resolved callback IDs and open cards in one SQLite file.

    Args:
        path: The file, normally ``<state dir>/hermes.sqlite3``.
    """

    def __init__(self, path: Path) -> None:
        """Open (and create) the file."""
        self.path = path
        if not path.exists():
            fd = os.open(path, os.O_CREAT | os.O_WRONLY, 0o600)
            os.close(fd)
        self._conn = sqlite3.connect(path, check_same_thread=False, isolation_level=None)
        self._conn.execute("PRAGMA synchronous = FULL")
        for statement in _SCHEMA:
            self._conn.execute(statement)
        self._lock = asyncio.Lock()

    async def _run(self, fn: Any) -> Any:
        async with self._lock:
            return await asyncio.to_thread(fn, self._conn)

    async def claim(self, callback_id: str) -> bool:
        """Record a callback as resolved; only the first claim wins.

        Args:
            callback_id: The component's ``custom_id``.

        Returns:
            True for the first claim, False when it was resolved before.
        """

        def fn(conn: sqlite3.Connection) -> bool:
            cursor = conn.execute(
                "INSERT OR IGNORE INTO resolved (callback_id) VALUES (?)", (callback_id,)
            )
            return cursor.rowcount == 1

        return bool(await self._run(fn))

    async def resolved(self, callback_id: str) -> bool:
        """Tell whether a callback was resolved.

        Args:
            callback_id: The ``custom_id``.

        Returns:
            True when claimed before.
        """

        def fn(conn: sqlite3.Connection) -> bool:
            row = conn.execute(
                "SELECT 1 FROM resolved WHERE callback_id = ?", (callback_id,)
            ).fetchone()
            return row is not None

        return bool(await self._run(fn))

    async def put_card(self, run_id: UUID, record: CardRecord) -> None:
        """Store the open card of a run (identifiers only).

        Args:
            run_id: The run.
            record: Kind, Hermes ID, session key, message and component IDs.
        """
        text = json.dumps(record, sort_keys=True)

        def fn(conn: sqlite3.Connection) -> None:
            conn.execute(
                "INSERT OR REPLACE INTO open_cards (run_id, record) VALUES (?, ?)",
                (str(run_id), text),
            )

        await self._run(fn)

    async def card(self, run_id: UUID) -> CardRecord | None:
        """The open card of a run.

        Args:
            run_id: The run.

        Returns:
            The record, or ``None``.
        """

        def fn(conn: sqlite3.Connection) -> str | None:
            row = conn.execute(
                "SELECT record FROM open_cards WHERE run_id = ?", (str(run_id),)
            ).fetchone()
            return str(row[0]) if row is not None else None

        text = await self._run(fn)
        loaded: CardRecord | None = json.loads(text) if text is not None else None
        return loaded

    async def drop_card(self, run_id: UUID) -> None:
        """Forget the open card of a run.

        Args:
            run_id: The run.
        """

        def fn(conn: sqlite3.Connection) -> None:
            conn.execute("DELETE FROM open_cards WHERE run_id = ?", (str(run_id),))

        await self._run(fn)

    async def put_session(self, thread_id: UUID, session_id: str, parent_id: str | None) -> None:
        """Record the Hermes session of a thread.

        Args:
            thread_id: The thread.
            session_id: Hermes' session ID.
            parent_id: The parent session in a compression chain, if any.
        """

        def fn(conn: sqlite3.Connection) -> None:
            conn.execute(
                "INSERT OR REPLACE INTO sessions (thread_id, session_id, parent_id)"
                " VALUES (?, ?, ?)",
                (str(thread_id), session_id, parent_id),
            )

        await self._run(fn)

    async def sessions(self) -> dict[UUID, tuple[str, str | None]]:
        """Every recorded thread session.

        Returns:
            Thread to (session ID, parent ID).
        """

        def fn(conn: sqlite3.Connection) -> list[tuple[str, str, str | None]]:
            rows = conn.execute("SELECT thread_id, session_id, parent_id FROM sessions").fetchall()
            return [(str(a), str(b), None if c is None else str(c)) for a, b, c in rows]

        rows = await self._run(fn)
        return {UUID(t): (sid, parent) for t, sid, parent in rows}

    async def put_answer(self, thread_id: UUID, text: str, now: float | None = None) -> None:
        """Record that the adapter took ``text`` over as an answer in the thread.

        Digests older than `ANSWER_RETENTION_SECONDS` are dropped in the same write.

        Args:
            thread_id: The run's thread.
            text: The answer as Hermes sent it.
            now: The wall time in seconds (tests).
        """
        at = time.time() if now is None else now
        digest = answer_digest(thread_id, text)

        def fn(conn: sqlite3.Connection) -> None:
            conn.execute("INSERT OR REPLACE INTO answers (digest, at) VALUES (?, ?)", (digest, at))
            conn.execute("DELETE FROM answers WHERE at < ?", (at - ANSWER_RETENTION_SECONDS,))

        await self._run(fn)

    async def answered(self, thread_id: UUID, text: str) -> bool:
        """Tell whether ``text`` was taken over as an answer in the thread.

        Args:
            thread_id: The thread.
            text: The text without Hermes' recovery notice.

        Returns:
            True when recorded by `put_answer`.
        """
        digest = answer_digest(thread_id, text)

        def fn(conn: sqlite3.Connection) -> bool:
            row = conn.execute("SELECT 1 FROM answers WHERE digest = ?", (digest,)).fetchone()
            return row is not None

        return bool(await self._run(fn))

    async def close(self) -> None:
        """Close the file."""
        async with self._lock:
            self._conn.close()
