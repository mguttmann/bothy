# SPDX-License-Identifier: Apache-2.0
"""The adapter's inbox: accepted events, their newest tokens and the ``jti`` store.

An event with an invocation token is acknowledged only after it passed the check and is stored
here (docs/spec/09-agents.md 4.3, docs/spec/04-sync-protocol.md 15.2); the start of its run
follows from the inbox. Per event the store keeps the envelope without ``invocation_token``,
the newest token with its ``iat`` and ``jti``, a state and a delivery count; besides that the
``jti`` values of passed tokens until ``exp`` plus ``adapter.invocation_token_leeway``
(09 4.5 point 7).

States move ``accepted`` (stored, not started), ``started`` (``run.start`` answered ``200``),
``waiting`` (``awaitRunInput``) and ``closed`` (ended, refused or handed over to the server).

A redelivery of a stored event (same ``id``) only counts; while the entry is ``accepted`` its
token replaces the stored one when the new ``iat`` is not older, and an equal ``iat`` takes the
later arrival (04 15.2 point 4, docs/DECISIONS.md D-M2-29-9).

`SqliteInboxStore` is durable: its file is the adapter's acceptance and holds tokens that expire
after ``agent.invocation_token_ttl``; it is created with mode ``0600`` and never logged.
`MemoryInboxStore` behaves alike for tests.

Examples:
    >>> import asyncio
    >>> from uuid import UUID
    >>> async def demo() -> int:
    ...     store = MemoryInboxStore()
    ...     event_id = UUID("01890a5d-ac96-774b-bcce-b302099a8057")
    ...     await store.add(InboxEntry(
    ...         event_id=event_id, seq=1, type="agent.invocation", run_id=None,
    ...         envelope={"id": str(event_id)}, token=None, token_iat=None, token_jti=None,
    ...         state=EntryState.ACCEPTED, deliveries=1))
    ...     entry = await store.redelivered(event_id, None, None, None)
    ...     return entry.deliveries
    >>> asyncio.run(demo())
    2
"""

from __future__ import annotations

import asyncio
import copy
import dataclasses
import json
import os
import sqlite3
from collections.abc import Mapping
from dataclasses import dataclass, field
from datetime import datetime
from enum import StrEnum
from typing import Any, Protocol, runtime_checkable
from uuid import UUID

from bothy_sdk._store import Db
from bothy_sdk.auth import Secret
from bothy_sdk.invocation import ReplayStore

__all__ = [
    "EntryState",
    "InboxEntry",
    "InboxStore",
    "MemoryInboxStore",
    "SqliteInboxStore",
]

_TOKEN_MEMBER = "invocation_token"  # noqa: S105 - a member name, not a secret  # nosec B105


class EntryState(StrEnum):
    """The state of an inbox entry."""

    ACCEPTED = "accepted"
    """Stored after the check; the run has not started."""
    STARTED = "started"
    """``run.start`` (or ``run.resume``) answered ``200``."""
    WAITING = "waiting"
    """The run waits for input (``awaitRunInput``)."""
    CLOSED = "closed"
    """Ended, refused or left to the server; nothing more happens with it."""


@dataclass(frozen=True)
class InboxEntry:
    """One stored event; ``repr`` shows neither the token nor the envelope.

    Attributes:
        event_id: The envelope ``id``.
        seq: The envelope ``seq``; ``None`` for an ephemeral event.
        type: The event type, for example ``agent.invocation``.
        run_id: The run the event belongs to, when it has one.
        envelope: The envelope as received, without ``invocation_token``.
        token: The newest invocation token of the event.
        token_iat: ``iat`` of ``token``.
        token_jti: ``jti`` of ``token``.
        state: The entry's state.
        deliveries: How often the event was delivered.
    """

    event_id: UUID
    seq: int | None
    type: str
    run_id: UUID | None
    envelope: Mapping[str, Any] = field(repr=False)
    token: Secret | None = field(repr=False)
    token_iat: int | None
    token_jti: str | None
    state: EntryState
    deliveries: int


@runtime_checkable
class InboxStore(ReplayStore, Protocol):
    """Storage of the inbox and of the ``jti`` store; every write is durable when it returns."""

    async def get(self, event_id: UUID) -> InboxEntry | None:
        """Return the entry of ``event_id``, or ``None`` when it is not stored.

        Args:
            event_id: The envelope ``id``.
        """
        ...

    async def add(self, entry: InboxEntry) -> None:
        """Store a new entry.

        Args:
            entry: The entry; its envelope must not carry ``invocation_token``.

        Raises:
            ValueError: The event is already stored, or the envelope carries a token.
        """
        ...

    async def redelivered(
        self, event_id: UUID, token: Secret | None, iat: int | None, jti: str | None
    ) -> InboxEntry:
        """Count a redelivery and keep the newer token while the entry is ``accepted``.

        The token replaces the stored one when the entry is ``accepted`` and ``iat`` is not
        older than the stored ``iat`` (an equal ``iat`` takes this later arrival).

        Args:
            event_id: The envelope ``id``.
            token: The token of the redelivery, if any.
            iat: Its ``iat``.
            jti: Its ``jti``.

        Returns:
            The entry after the update.

        Raises:
            KeyError: The event is not stored.
        """
        ...

    async def replace_token(
        self, event_id: UUID, token: Secret, iat: int, jti: str, *, replaces: Secret | None = None
    ) -> InboxEntry:
        """Replace the token (after a renewal grant).

        Args:
            event_id: The envelope ``id``.
            token: The granted token.
            iat: Its ``iat``.
            jti: Its ``jti``.
            replaces: The renewed token; when given, the token is replaced only while the stored
                token is still this one (a redelivery during the renewal stored a newer token,
                which stays).

        Returns:
            The entry after the call.

        Raises:
            KeyError: The event is not stored.
        """
        ...

    async def set_state(self, event_id: UUID, state: EntryState) -> None:
        """Set the state of an entry.

        Args:
            event_id: The envelope ``id``.
            state: The new state.

        Raises:
            KeyError: The event is not stored.
        """
        ...

    async def for_run(self, run_id: UUID) -> list[InboxEntry]:
        """Return the entries of a run in ``seq`` order (ephemeral entries last).

        Args:
            run_id: The run.
        """
        ...

    async def in_states(self, *states: EntryState) -> list[InboxEntry]:
        """Return the entries in one of ``states`` in ``seq`` order (ephemeral entries last).

        Args:
            *states: The states asked for.
        """
        ...

    async def prune(self, now: datetime) -> None:
        """Drop the ``jti`` rows that expired at ``now``; inbox entries stay.

        Args:
            now: The current wall time.
        """
        ...

    async def close(self) -> None:
        """Release the store; closing twice is harmless."""
        ...


def _check_new(entry: InboxEntry) -> None:
    if _TOKEN_MEMBER in entry.envelope:
        raise ValueError(f"the envelope must be stored without {_TOKEN_MEMBER}")


def _after_redelivery(
    entry: InboxEntry, token: Secret | None, iat: int | None, jti: str | None
) -> InboxEntry:
    newer = (
        entry.state is EntryState.ACCEPTED
        and token is not None
        and iat is not None
        and jti is not None
        and (entry.token_iat is None or iat >= entry.token_iat)
    )
    if newer:
        return dataclasses.replace(
            entry, token=token, token_iat=iat, token_jti=jti, deliveries=entry.deliveries + 1
        )
    return dataclasses.replace(entry, deliveries=entry.deliveries + 1)


def _after_grant(
    entry: InboxEntry, token: Secret, iat: int, jti: str, replaces: Secret | None
) -> InboxEntry:
    if replaces is not None and entry.token is not None and entry.token != replaces:
        return entry  # a redelivery stored a newer token while the renewal was on its way
    return dataclasses.replace(entry, token=token, token_iat=iat, token_jti=jti)


def _order(entry: InboxEntry) -> tuple[bool, int]:
    return (entry.seq is None, entry.seq if entry.seq is not None else 0)


class MemoryInboxStore:
    """An in-memory `InboxStore` for tests; it behaves like `SqliteInboxStore` but is lost on exit.

    Entries are kept in arrival order, so entries without ``seq`` keep their arrival order.
    """

    def __init__(self) -> None:
        """Create an empty store."""
        self._entries: dict[UUID, InboxEntry] = {}
        self._jti: dict[str, datetime] = {}
        self._lock = asyncio.Lock()

    async def jti_seen(self, jti: str, now: datetime) -> bool:
        """Return whether ``jti`` is remembered beyond ``now``.

        Args:
            jti: The token ID.
            now: The current wall time.
        """
        until = self._jti.get(jti)
        return until is not None and now < until

    async def remember_jti(self, jti: str, until: datetime) -> None:
        """Remember ``jti`` until ``until`` (an earlier end never shortens a later one).

        Args:
            jti: The token ID.
            until: When the token cannot pass anymore (``exp`` plus the leeway).
        """
        old = self._jti.get(jti)
        self._jti[jti] = until if old is None or until > old else old

    async def get(self, event_id: UUID) -> InboxEntry | None:
        """Return the entry of ``event_id``, or ``None``.

        Args:
            event_id: The envelope ``id``.
        """
        return self._entries.get(event_id)

    async def add(self, entry: InboxEntry) -> None:
        """Store a new entry.

        Args:
            entry: The entry.

        Raises:
            ValueError: The event is already stored, or the envelope carries a token.
        """
        _check_new(entry)
        async with self._lock:
            if entry.event_id in self._entries:
                raise ValueError("event already stored")
            self._entries[entry.event_id] = dataclasses.replace(
                entry, envelope=copy.deepcopy(dict(entry.envelope))
            )

    async def redelivered(
        self, event_id: UUID, token: Secret | None, iat: int | None, jti: str | None
    ) -> InboxEntry:
        """Count a redelivery; see `InboxStore.redelivered`.

        Args:
            event_id: The envelope ``id``.
            token: The token of the redelivery, if any.
            iat: Its ``iat``.
            jti: Its ``jti``.

        Returns:
            The entry after the update.

        Raises:
            KeyError: The event is not stored.
        """
        async with self._lock:
            updated = _after_redelivery(self._entries[event_id], token, iat, jti)
            self._entries[event_id] = updated
            return updated

    async def replace_token(
        self, event_id: UUID, token: Secret, iat: int, jti: str, *, replaces: Secret | None = None
    ) -> InboxEntry:
        """Replace the token; see `InboxStore.replace_token`.

        Args:
            event_id: The envelope ``id``.
            token: The granted token.
            iat: Its ``iat``.
            jti: Its ``jti``.
            replaces: The renewed token, if the replacement depends on it.

        Returns:
            The entry after the call.

        Raises:
            KeyError: The event is not stored.
        """
        async with self._lock:
            updated = _after_grant(self._entries[event_id], token, iat, jti, replaces)
            self._entries[event_id] = updated
            return updated

    async def set_state(self, event_id: UUID, state: EntryState) -> None:
        """Set the state of an entry.

        Args:
            event_id: The envelope ``id``.
            state: The new state.

        Raises:
            KeyError: The event is not stored.
        """
        async with self._lock:
            self._entries[event_id] = dataclasses.replace(self._entries[event_id], state=state)

    async def for_run(self, run_id: UUID) -> list[InboxEntry]:
        """Return the entries of a run in ``seq`` order.

        Args:
            run_id: The run.
        """
        return sorted((e for e in self._entries.values() if e.run_id == run_id), key=_order)

    async def in_states(self, *states: EntryState) -> list[InboxEntry]:
        """Return the entries in one of ``states`` in ``seq`` order.

        Args:
            *states: The states asked for.
        """
        return sorted((e for e in self._entries.values() if e.state in states), key=_order)

    async def prune(self, now: datetime) -> None:
        """Drop the expired ``jti`` rows.

        Args:
            now: The current wall time.
        """
        self._jti = {jti: until for jti, until in self._jti.items() if now < until}

    async def close(self) -> None:
        """Nothing to release."""


_SCHEMA = (
    """
    CREATE TABLE IF NOT EXISTS inbox (
        arrival INTEGER PRIMARY KEY AUTOINCREMENT,
        event_id TEXT NOT NULL UNIQUE,
        seq INTEGER,
        type TEXT NOT NULL,
        run_id TEXT,
        envelope TEXT NOT NULL,
        token TEXT,
        token_iat INTEGER,
        token_jti TEXT,
        state TEXT NOT NULL,
        deliveries INTEGER NOT NULL
    )
    """,
    "CREATE INDEX IF NOT EXISTS inbox_run ON inbox (run_id)",
    "CREATE INDEX IF NOT EXISTS inbox_state ON inbox (state)",
    "CREATE TABLE IF NOT EXISTS jti (jti TEXT PRIMARY KEY, until REAL NOT NULL)",
)
_COLUMNS = "event_id, seq, type, run_id, envelope, token, token_iat, token_jti, state, deliveries"
# Entries with a seq first in seq order, then those without one in arrival order.
_ORDER = "ORDER BY seq IS NULL, seq, arrival"


def _row(row: tuple[Any, ...]) -> InboxEntry:
    event_id, seq, type_, run_id, envelope, token, iat, jti, state, deliveries = row
    return InboxEntry(
        event_id=UUID(event_id),
        seq=seq,
        type=type_,
        run_id=UUID(run_id) if run_id is not None else None,
        envelope=json.loads(envelope),
        token=Secret(token) if token else None,
        token_iat=iat,
        token_jti=jti,
        state=EntryState(state),
        deliveries=deliveries,
    )


def _select(conn: sqlite3.Connection, event_id: UUID) -> InboxEntry | None:
    row = conn.execute(
        f"SELECT {_COLUMNS} FROM inbox WHERE event_id = ?",  # noqa: S608 - constant columns  # nosec B608
        (str(event_id),),
    ).fetchone()
    return _row(row) if row is not None else None


def _require(conn: sqlite3.Connection, event_id: UUID) -> InboxEntry:
    found = _select(conn, event_id)
    if found is None:
        # The message names the event ID only, never a token.
        raise KeyError(str(event_id))
    return found


class SqliteInboxStore:
    """The durable `InboxStore`: one SQLite file of mode ``0600`` (``synchronous = FULL``).

    The token column holds the token text, because the file is the adapter's durable acceptance
    (docs/DECISIONS.md D-M2-29-9); it is never logged. Every call runs in a worker thread.

    Args:
        path: The database file, usually ``inbox.sqlite3`` in ``AdapterConfig.state_dir``.

    Raises:
        PermissionError: An existing file is accessible by group or others.
    """

    def __init__(self, path: str | os.PathLike[str]) -> None:
        """Open or create the store; see the class docstring."""
        self._db = Db(path)
        self._ready = False

    async def _run(self, fn: Any) -> Any:
        if not self._ready:
            await self._db.run(_create)
            self._ready = True
        return await self._db.run(fn)

    async def jti_seen(self, jti: str, now: datetime) -> bool:
        """Return whether ``jti`` is remembered beyond ``now``.

        Args:
            jti: The token ID.
            now: The current wall time.
        """

        def fn(conn: sqlite3.Connection) -> bool:
            row = conn.execute(
                "SELECT 1 FROM jti WHERE jti = ? AND until > ?", (jti, now.timestamp())
            ).fetchone()
            return row is not None

        seen: bool = await self._run(fn)
        return seen

    async def remember_jti(self, jti: str, until: datetime) -> None:
        """Remember ``jti`` until ``until`` (an earlier end never shortens a later one).

        Args:
            jti: The token ID.
            until: When the token cannot pass anymore (``exp`` plus the leeway).
        """

        def fn(conn: sqlite3.Connection) -> None:
            conn.execute(
                "INSERT INTO jti (jti, until) VALUES (?, ?) "
                "ON CONFLICT (jti) DO UPDATE SET until = max(until, excluded.until)",
                (jti, until.timestamp()),
            )

        await self._run(fn)

    async def get(self, event_id: UUID) -> InboxEntry | None:
        """Return the entry of ``event_id``, or ``None``.

        Args:
            event_id: The envelope ``id``.
        """
        found: InboxEntry | None = await self._run(lambda c: _select(c, event_id))
        return found

    async def add(self, entry: InboxEntry) -> None:
        """Store a new entry; durable when this returns.

        Args:
            entry: The entry.

        Raises:
            ValueError: The event is already stored, or the envelope carries a token.
        """
        _check_new(entry)
        envelope = json.dumps(dict(entry.envelope), separators=(",", ":"))

        def fn(conn: sqlite3.Connection) -> None:
            try:
                conn.execute(
                    f"INSERT INTO inbox ({_COLUMNS}) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",  # noqa: S608 - constant columns  # nosec B608
                    (
                        str(entry.event_id),
                        entry.seq,
                        entry.type,
                        str(entry.run_id) if entry.run_id is not None else None,
                        envelope,
                        entry.token.reveal() if entry.token is not None else None,
                        entry.token_iat,
                        entry.token_jti,
                        entry.state.value,
                        entry.deliveries,
                    ),
                )
            except sqlite3.IntegrityError:
                raise ValueError("event already stored") from None

        await self._run(fn)

    async def redelivered(
        self, event_id: UUID, token: Secret | None, iat: int | None, jti: str | None
    ) -> InboxEntry:
        """Count a redelivery; see `InboxStore.redelivered`.

        Args:
            event_id: The envelope ``id``.
            token: The token of the redelivery, if any.
            iat: Its ``iat``.
            jti: Its ``jti``.

        Returns:
            The entry after the update.

        Raises:
            KeyError: The event is not stored.
        """

        def fn(conn: sqlite3.Connection) -> InboxEntry:
            updated = _after_redelivery(_require(conn, event_id), token, iat, jti)
            _write_token_and_count(conn, updated)
            return updated

        result: InboxEntry = await self._run(fn)
        return result

    async def replace_token(
        self, event_id: UUID, token: Secret, iat: int, jti: str, *, replaces: Secret | None = None
    ) -> InboxEntry:
        """Replace the token; durable when this returns; see `InboxStore.replace_token`.

        Args:
            event_id: The envelope ``id``.
            token: The granted token.
            iat: Its ``iat``.
            jti: Its ``jti``.
            replaces: The renewed token, if the replacement depends on it.

        Returns:
            The entry after the call.

        Raises:
            KeyError: The event is not stored.
        """

        def fn(conn: sqlite3.Connection) -> InboxEntry:
            current = _require(conn, event_id)
            updated = _after_grant(current, token, iat, jti, replaces)
            if updated is not current:
                _write_token_and_count(conn, updated)
            return updated

        result: InboxEntry = await self._run(fn)
        return result

    async def set_state(self, event_id: UUID, state: EntryState) -> None:
        """Set the state of an entry; durable when this returns.

        Args:
            event_id: The envelope ``id``.
            state: The new state.

        Raises:
            KeyError: The event is not stored.
        """

        def fn(conn: sqlite3.Connection) -> None:
            cur = conn.execute(
                "UPDATE inbox SET state = ? WHERE event_id = ?", (state.value, str(event_id))
            )
            if cur.rowcount == 0:
                raise KeyError(str(event_id))

        await self._run(fn)

    async def for_run(self, run_id: UUID) -> list[InboxEntry]:
        """Return the entries of a run in ``seq`` order (ephemeral entries last).

        Args:
            run_id: The run.
        """

        def fn(conn: sqlite3.Connection) -> list[InboxEntry]:
            rows = conn.execute(
                f"SELECT {_COLUMNS} FROM inbox WHERE run_id = ? {_ORDER}",  # noqa: S608 - constant columns  # nosec B608
                (str(run_id),),
            ).fetchall()
            return [_row(r) for r in rows]

        found: list[InboxEntry] = await self._run(fn)
        return found

    async def in_states(self, *states: EntryState) -> list[InboxEntry]:
        """Return the entries in one of ``states`` in ``seq`` order (ephemeral entries last).

        Args:
            *states: The states asked for.
        """
        if not states:
            return []
        marks = ", ".join("?" for _ in states)

        def fn(conn: sqlite3.Connection) -> list[InboxEntry]:
            rows = conn.execute(
                f"SELECT {_COLUMNS} FROM inbox WHERE state IN ({marks}) {_ORDER}",  # noqa: S608 - placeholders only  # nosec B608
                [s.value for s in states],
            ).fetchall()
            return [_row(r) for r in rows]

        found: list[InboxEntry] = await self._run(fn)
        return found

    async def prune(self, now: datetime) -> None:
        """Drop the ``jti`` rows that expired at ``now``; inbox entries stay.

        Args:
            now: The current wall time.
        """
        await self._run(lambda c: c.execute("DELETE FROM jti WHERE until <= ?", (now.timestamp(),)))

    async def close(self) -> None:
        """Close the database; closing twice is harmless."""
        await self._db.close()


def _create(conn: sqlite3.Connection) -> None:
    for statement in _SCHEMA:
        conn.execute(statement)


def _write_token_and_count(conn: sqlite3.Connection, entry: InboxEntry) -> None:
    conn.execute(
        "UPDATE inbox SET token = ?, token_iat = ?, token_jti = ?, deliveries = ? "
        "WHERE event_id = ?",
        (
            entry.token.reveal() if entry.token is not None else None,
            entry.token_iat,
            entry.token_jti,
            entry.deliveries,
            str(entry.event_id),
        ),
    )
