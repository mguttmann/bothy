# SPDX-License-Identifier: Apache-2.0
"""Outbox for replies and other writes that must never go out twice or get lost.

A reply gets its ``client_msg_id`` (UUIDv7) stored before the first send, and every retry sends
the same ID, so the server answers a repeat with the original (docs/spec/04-sync-protocol.md
13.2, 06 7). Other writes (`Outbox.call`) get an ``Idempotency-Key`` stored the same way. Entries
leave in order per thread (FIFO), threads in parallel. Transient failures use the retry policy of
the client; after the last attempt the entry stays pending and `Outbox.flush` (the adapter calls
it at its start and after every ``ready``) sends it again with the same ID. ``400``, ``404``,
``409``, ``410``, ``413`` and ``422`` mark it failed with its problem code: it stays visible in
`Outbox.failed` and `on_failed` is called; nothing is dropped silently (04 13.2 point 5).

The store is exchangeable: `SqliteOutboxStore` (``outbox.sqlite3`` in the adapter's state
directory, mode ``0600``) or `MemoryOutboxStore`.

Examples:
    >>> import asyncio
    >>> store = MemoryOutboxStore()
    >>> asyncio.run(store.pending()) == []
    True
"""

from __future__ import annotations

import asyncio
import contextlib
import importlib
import json
import logging
import os
import sqlite3
from collections.abc import Awaitable, Callable, Mapping, Sequence
from dataclasses import dataclass, replace
from enum import StrEnum
from typing import Any, Protocol
from uuid import UUID

import httpx

from bothy_sdk._store import Db
from bothy_sdk.auth import Secret
from bothy_sdk.client import Client
from bothy_sdk.errors import InvocationTokenInvalid, ProblemError, RequestInProgress
from bothy_sdk.generated import models
from bothy_sdk.generated.api.messages import create_message
from bothy_sdk.generated.models import ComponentCreate, Message, MessageCreate, MessageCreateKind
from bothy_sdk.ids import uuid7

__all__ = [
    "PERMANENT_STATUSES",
    "MemoryOutboxStore",
    "Outbox",
    "OutboxEntry",
    "OutboxState",
    "OutboxStore",
    "SqliteOutboxStore",
]

_LOG = logging.getLogger("bothy_sdk")

PERMANENT_STATUSES = frozenset({400, 404, 409, 410, 413, 422})
"""Statuses that mark an entry failed instead of retrying it later."""

_MESSAGE = "message"
_CALL = "call"


class OutboxState(StrEnum):
    """State of an outbox entry."""

    PENDING = "pending"
    SENT = "sent"
    FAILED = "failed"


@dataclass(frozen=True)
class OutboxEntry:
    """One stored write.

    Attributes:
        entry_id: The order of the entry (ascending per store).
        thread_id: The thread whose order it keeps (FIFO).
        kind: ``message`` (a reply) or ``call`` (another write).
        key: The ``client_msg_id`` of a reply or the ``Idempotency-Key`` of a call.
        payload: The stored request (``MessageCreate`` as a dict, or the operation and its
            arguments).
        state: Pending, sent or failed.
        problem_code: The problem code of a failed entry.
        run_id: The run whose binding the write goes out under (07 SR9): the run passed by the
            caller or, when none was passed, the run the client was bound to when the entry was
            stored; ``None`` for a write made outside any run.
        created: The message a reply created, on the entry `send_message` returns after a
            successful send (its ``id``, ``version`` and ``components`` serve a later
            ``updateMessage`` or ``updateComponent``); not stored, ``None`` otherwise.
    """

    entry_id: int
    thread_id: UUID
    kind: str
    key: str
    payload: Mapping[str, Any]
    state: OutboxState = OutboxState.PENDING
    problem_code: str | None = None
    run_id: UUID | None = None
    created: Message | None = None

    @property
    def message_id(self) -> UUID | None:
        """The ID of the created message, if this entry carries one."""
        return self.created.id if self.created is not None else None

    @property
    def message_version(self) -> int | None:
        """The version of the created message, if this entry carries one."""
        return self.created.version if self.created is not None else None


class OutboxStore(Protocol):
    """Durable storage of outbox entries."""

    async def add(self, entry: OutboxEntry) -> OutboxEntry:
        """Store a new entry durably and return it with its ``entry_id``."""
        ...

    async def pending(self) -> list[OutboxEntry]:
        """Return every pending entry in ``entry_id`` order."""
        ...

    async def mark(self, entry_id: int, state: OutboxState, problem_code: str | None) -> None:
        """Change the state of an entry."""
        ...

    async def failed(self) -> list[OutboxEntry]:
        """Return every failed entry in ``entry_id`` order."""
        ...

    async def by_key(self, key: str) -> OutboxEntry | None:
        """Return the entry with ``key`` in any state, or ``None``."""
        ...

    async def close(self) -> None:
        """Release the storage."""
        ...


class MemoryOutboxStore:
    """An outbox store in memory (tests; not durable)."""

    def __init__(self) -> None:
        """Create an empty store."""
        self._entries: dict[int, OutboxEntry] = {}
        self._next = 1

    async def add(self, entry: OutboxEntry) -> OutboxEntry:
        """Store ``entry`` with the next ``entry_id``."""
        stored = replace(entry, entry_id=self._next)
        self._entries[self._next] = stored
        self._next += 1
        return stored

    async def pending(self) -> list[OutboxEntry]:
        """Return the pending entries in order."""
        return [e for e in self._entries.values() if e.state is OutboxState.PENDING]

    async def mark(self, entry_id: int, state: OutboxState, problem_code: str | None) -> None:
        """Change the state of an entry."""
        self._entries[entry_id] = replace(
            self._entries[entry_id], state=state, problem_code=problem_code
        )

    async def failed(self) -> list[OutboxEntry]:
        """Return the failed entries in order."""
        return [e for e in self._entries.values() if e.state is OutboxState.FAILED]

    async def by_key(self, key: str) -> OutboxEntry | None:
        """Return the entry with ``key``, or ``None``."""
        return next((e for e in self._entries.values() if e.key == key), None)

    async def close(self) -> None:
        """Nothing to release."""


_SCHEMA = """
CREATE TABLE IF NOT EXISTS outbox (
    entry_id INTEGER PRIMARY KEY AUTOINCREMENT,
    thread_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    key TEXT NOT NULL UNIQUE,
    payload TEXT NOT NULL,
    state TEXT NOT NULL,
    problem_code TEXT,
    run_id TEXT
)
"""


def _row(row: Sequence[Any]) -> OutboxEntry:
    return OutboxEntry(
        entry_id=int(row[0]),
        thread_id=UUID(row[1]),
        kind=str(row[2]),
        key=str(row[3]),
        payload=json.loads(row[4]),
        state=OutboxState(row[5]),
        problem_code=row[6],
        run_id=UUID(row[7]) if row[7] else None,
    )


_COLUMNS = "entry_id, thread_id, kind, key, payload, state, problem_code, run_id"


class SqliteOutboxStore:
    """The durable outbox store: a SQLite file of mode ``0600`` (``synchronous = FULL``).

    Args:
        path: The database file, usually ``<state_dir>/outbox.sqlite3``.
    """

    def __init__(self, path: str | os.PathLike[str]) -> None:
        """Open or create the store."""
        self._db = Db(path)
        self._ready = False

    async def _init(self) -> None:
        if not self._ready:
            await self._db.run(lambda c: c.execute(_SCHEMA))
            self._ready = True

    async def add(self, entry: OutboxEntry) -> OutboxEntry:
        """Store ``entry`` durably and return it with its ``entry_id``."""
        await self._init()

        def insert(c: sqlite3.Connection) -> int:
            cur = c.execute(
                "INSERT INTO outbox (thread_id, kind, key, payload, state, problem_code, run_id)"
                " VALUES (?, ?, ?, ?, ?, ?, ?)",
                (
                    str(entry.thread_id),
                    entry.kind,
                    entry.key,
                    json.dumps(entry.payload, separators=(",", ":")),
                    entry.state.value,
                    entry.problem_code,
                    str(entry.run_id) if entry.run_id else None,
                ),
            )
            return int(cur.lastrowid or 0)

        return replace(entry, entry_id=await self._db.run(insert))

    async def _select(self, state: OutboxState) -> list[OutboxEntry]:
        await self._init()
        rows = await self._db.run(
            lambda c: c.execute(
                f"SELECT {_COLUMNS} FROM outbox WHERE state = ? ORDER BY entry_id",  # noqa: S608 - constant columns  # nosec B608
                (state.value,),
            ).fetchall()
        )
        return [_row(r) for r in rows]

    async def pending(self) -> list[OutboxEntry]:
        """Return the pending entries in order."""
        return await self._select(OutboxState.PENDING)

    async def failed(self) -> list[OutboxEntry]:
        """Return the failed entries in order."""
        return await self._select(OutboxState.FAILED)

    async def by_key(self, key: str) -> OutboxEntry | None:
        """Return the entry with ``key``, or ``None``."""
        await self._init()
        row = await self._db.run(
            lambda c: c.execute(
                f"SELECT {_COLUMNS} FROM outbox WHERE key = ?",  # noqa: S608 - constant columns  # nosec B608
                (key,),
            ).fetchone()
        )
        return _row(row) if row is not None else None

    async def mark(self, entry_id: int, state: OutboxState, problem_code: str | None) -> None:
        """Change the state of an entry."""
        await self._init()
        await self._db.run(
            lambda c: c.execute(
                "UPDATE outbox SET state = ?, problem_code = ? WHERE entry_id = ?",
                (state.value, problem_code, entry_id),
            )
        )

    async def close(self) -> None:
        """Close the database."""
        await self._db.close()


def _encode_arg(value: Any) -> Any:
    if isinstance(value, UUID):
        return {"$uuid": str(value)}
    to_dict = getattr(value, "to_dict", None)
    if callable(to_dict):
        return {"$model": type(value).__name__, "data": to_dict()}
    if isinstance(value, str | int | float | bool) or value is None:
        return value
    raise TypeError(f"outbox cannot store an argument of type {type(value).__name__}")


def _decode_arg(value: Any) -> Any:
    if isinstance(value, dict) and "$uuid" in value:
        return UUID(value["$uuid"])
    if isinstance(value, dict) and "$model" in value:
        return getattr(models, value["$model"]).from_dict(value["data"])
    return value


class Outbox:
    """Sends replies and writes in order per thread and never twice.

    Args:
        client: The REST client.
        store: The storage (default `MemoryOutboxStore`).
        on_failed: Called with every entry that failed permanently.
        token_for: Returns the newest invocation token of a run. With it, an entry of a run goes
            out bound to that run (07 SR9) whenever it is sent, also from a later `flush`, and
            an entry of a run without a known token fails instead of going out unbound.
    """

    def __init__(
        self,
        client: Client,
        store: OutboxStore | None = None,
        *,
        on_failed: Callable[[OutboxEntry], Awaitable[None] | None] | None = None,
        token_for: Callable[[UUID], Awaitable[Secret | None]] | None = None,
    ) -> None:
        """Create the outbox; see the class docstring."""
        self.client = client
        self.store: OutboxStore = store or MemoryOutboxStore()
        self.on_failed = on_failed
        self.token_for = token_for
        self._locks: dict[UUID, asyncio.Lock] = {}

    def _lock(self, thread_id: UUID) -> asyncio.Lock:
        return self._locks.setdefault(thread_id, asyncio.Lock())

    async def send_message(  # noqa: PLR0913 - the members of a reply
        self,
        thread_id: UUID,
        content_md: str,
        *,
        run_id: UUID | None = None,
        reply_to_message_id: UUID | None = None,
        kind: str = "text",
        components: Sequence[ComponentCreate] = (),
        client_msg_id: UUID | None = None,
    ) -> OutboxEntry:
        """Store a reply with a new ``client_msg_id`` and send it (``createMessage``).

        A caller that stored a ``client_msg_id`` of its own before (a runner that may die
        between storing and sending, M2-35 D-M2-35-28) passes it: an entry with that key
        already in the store is not added again, it is sent again while pending and returned
        as it is otherwise; the server answers a repeat with the original (04 13.2).

        Args:
            thread_id: The thread.
            content_md: The Markdown text.
            run_id: The run the reply belongs to (inside a run the adapter passes it).
            reply_to_message_id: The message it answers, if any.
            kind: ``text`` or ``card`` (a message with components).
            components: The components of a card.
            client_msg_id: The reply's UUIDv7, when the caller stored one; a new one otherwise.

        Returns:
            The entry after the send attempt (sent, failed or still pending).
        """
        if client_msg_id is not None:
            existing = await self.store.by_key(str(client_msg_id))
            if existing is not None:
                if existing.state is OutboxState.PENDING:
                    return await self._drain(existing.thread_id, until=existing) or existing
                return existing
        body = MessageCreate(
            client_msg_id=client_msg_id or uuid7(),
            kind=MessageCreateKind(kind),
            content_md=content_md,
        )
        if run_id is not None:
            body.run_id = run_id
        if reply_to_message_id is not None:
            body.reply_to_message_id = reply_to_message_id
        if components:
            body.components = list(components)
        entry = OutboxEntry(
            entry_id=0,
            thread_id=thread_id,
            kind=_MESSAGE,
            key=str(body.client_msg_id),
            payload=body.to_dict(),
            # Every write made inside a run goes out under that run's binding (09 4.7, 07 SR9),
            # also one into another thread that is not the run's reply.
            run_id=run_id or self.client.bound_run,
        )
        stored = await self.store.add(entry)
        return await self._drain(thread_id, until=stored) or stored

    async def call(
        self,
        operation: Callable[..., Awaitable[Any]],
        /,
        *,
        thread_id: UUID,
        run_id: UUID | None = None,
        **kwargs: Any,
    ) -> OutboxEntry:
        """Store a write with a new ``Idempotency-Key`` and send it.

        Args:
            operation: A generated ``asyncio_detailed`` function.
            thread_id: The thread whose order the write keeps.
            run_id: The run it belongs to, if any.
            **kwargs: Its arguments (UUIDs, generated models and plain values).

        Returns:
            The entry after the send attempt.
        """
        payload = {
            "operation": f"{operation.__module__}:{operation.__name__}",
            "kwargs": {k: _encode_arg(v) for k, v in kwargs.items()},
        }
        entry = OutboxEntry(
            entry_id=0,
            thread_id=thread_id,
            kind=_CALL,
            key=str(uuid7()),
            payload=payload,
            run_id=run_id or self.client.bound_run,
        )
        stored = await self.store.add(entry)
        return await self._drain(thread_id, until=stored) or stored

    async def flush(self) -> None:
        """Send every pending entry again, in order per thread, threads in parallel."""
        threads = {e.thread_id for e in await self.store.pending()}
        await asyncio.gather(*(self._drain(t) for t in threads))

    async def failed(self) -> list[OutboxEntry]:
        """Return every entry that failed permanently.

        Returns:
            The failed entries in order.
        """
        return await self.store.failed()

    async def _drain(self, thread_id: UUID, until: OutboxEntry | None = None) -> OutboxEntry | None:
        """Send the pending entries of one thread in order; stop at the first transient error.

        Returns the state of ``until`` after the attempt (it stays pending when an earlier
        entry of the thread is still pending).
        """
        result = until
        async with self._lock(thread_id):
            pending = [e for e in await self.store.pending() if e.thread_id == thread_id]
            for entry in pending:
                if until is not None and entry.entry_id > until.entry_id:
                    break
                sent = await self._send(entry)
                if until is not None and entry.entry_id == until.entry_id:
                    result = sent
                if sent.state is OutboxState.PENDING:
                    break
        return result

    async def _binding(self, entry: OutboxEntry) -> contextlib.AbstractContextManager[None] | None:
        """The SR9 binding an entry is sent under; ``None`` when it must not be sent."""
        if entry.run_id is None:
            # Stored outside any run: never under the binding of whatever run is current now.
            return self.client.unbound()
        if self.token_for is None:
            return contextlib.nullcontext()
        token = await self.token_for(entry.run_id)
        if token is None:
            return None
        return self.client.bind_run(entry.run_id, token)

    async def _fail(self, entry: OutboxEntry, code: str) -> OutboxEntry:
        await self.store.mark(entry.entry_id, OutboxState.FAILED, code)
        failed = replace(entry, state=OutboxState.FAILED, problem_code=code)
        _LOG.warning("outbox entry %d failed code=%s", entry.entry_id, code)
        if self.on_failed is not None:
            result = self.on_failed(failed)
            if asyncio.iscoroutine(result):
                await result
        return failed

    async def _send(self, entry: OutboxEntry) -> OutboxEntry:
        binding = await self._binding(entry)
        if binding is None:
            return await self._fail(entry, "run_token_unavailable")
        try:
            with binding:
                result = await self._call(entry)
        except InvocationTokenInvalid as exc:
            # invalid or run_inactive: the token of the run is no longer usable (07 SR9).
            return await self._fail(entry, exc.code)
        except ProblemError as exc:
            # 409 request_in_progress: the original request with this key may still complete,
            # so the entry waits and goes out again with the same key (06 7).
            if exc.status in PERMANENT_STATUSES and not isinstance(exc, RequestInProgress):
                return await self._fail(entry, exc.code)
            _LOG.info("outbox entry %d stays pending code=%s", entry.entry_id, exc.code)
            return entry
        except httpx.TransportError as exc:
            _LOG.info("outbox entry %d stays pending: %s", entry.entry_id, type(exc).__name__)
            return entry
        await self.store.mark(entry.entry_id, OutboxState.SENT, None)
        created = result if isinstance(result, Message) else None
        return replace(entry, state=OutboxState.SENT, created=created)

    async def _call(self, entry: OutboxEntry) -> Any:
        if entry.kind == _MESSAGE:
            return await self.client.call(
                create_message.asyncio_detailed,
                thread_id=entry.thread_id,
                body=MessageCreate.from_dict(dict(entry.payload)),
            )
        module, _, name = str(entry.payload["operation"]).partition(":")
        operation = getattr(importlib.import_module(module), name)
        kwargs = {k: _decode_arg(v) for k, v in dict(entry.payload["kwargs"]).items()}
        with self.client.idempotency_key(entry.key):
            return await self.client.call(operation, **kwargs)
