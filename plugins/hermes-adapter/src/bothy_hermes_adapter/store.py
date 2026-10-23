# SPDX-License-Identifier: Apache-2.0
"""The adapter's own durable state: resolved callbacks and open cards (09 4.3, 4.9, 15).

Inbox, ``jti`` store and outbox are the SDK's files in the state directory; this module keeps
only what the Hermes side adds, in one SQLite file through the standard library:

* the callback IDs already resolved, so a redelivered interaction resolves nothing (AG-15);
* the open card per run (its kind, Hermes ID, Hermes session key, card message and component
  IDs).

No content is stored: no question, no answer text, no label (BR-13, SEC-LOG-4 applied to the
adapter); the texts of an open card live only in the process. Every write is committed before the
Hermes callback or the send it guards (``synchronous = FULL``). The file is created with mode
``0600`` in a directory the SDK requires to be closed to group and others.

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
import json
import os
import sqlite3
from pathlib import Path
from typing import Any
from uuid import UUID

__all__ = ["CallbackStore", "CardRecord"]

_SCHEMA = (
    "CREATE TABLE IF NOT EXISTS resolved (callback_id TEXT PRIMARY KEY)",
    "CREATE TABLE IF NOT EXISTS open_cards (run_id TEXT PRIMARY KEY, record TEXT NOT NULL)",
)

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

    async def close(self) -> None:
        """Close the file."""
        async with self._lock:
            self._conn.close()
