# SPDX-License-Identifier: Apache-2.0
"""`Store` in memory for unit tests (brief M2-50 Design note 9).

The same SQL as `jira_app.store.sqlite_store.SqliteStore` on an in-memory SQLite database,
without files and without the lock: one implementation of the rules, so the contract test that
runs against both forms checks the files and the lock of the SQLite form and the same
behaviour of each.

Examples:
    >>> import asyncio
    >>> async def demo() -> int:
    ...     store = MemoryStore(now=lambda: 0.0, window=60.0)
    ...     await store.open()
    ...     job = await store.next_job()
    ...     await store.close()
    ...     return 0 if job is None else job.id
    >>> asyncio.run(demo())
    0
"""

from __future__ import annotations

import sqlite3

from jira_app.store.sqlite_store import SCHEMA, SqlStore

__all__ = ["MemoryStore"]


class MemoryStore(SqlStore):
    """The store in an in-memory database; lost when closed."""

    def _connect(self) -> sqlite3.Connection:
        db = sqlite3.connect(":memory:", check_same_thread=False)
        with db:
            db.executescript(SCHEMA)
        return db
