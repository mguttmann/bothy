# SPDX-License-Identifier: Apache-2.0
"""Consumers of the FakeServer: the event log, cursors, REST windows and gateway runners.

Private. One holder per consumer (newest wins, ``4015`` for a replaced gateway connection), a
cursor created at the head by the first takeover, cumulative acknowledgements, and redelivery
after ``agent.ack_timeout`` through background tasks on the shared FakeClock
(docs/spec/04-sync-protocol.md 15.1 and 15.2, docs/spec/09-agents.md 4.3).
"""

from __future__ import annotations

import asyncio
import contextlib
from collections.abc import Awaitable, Callable, Mapping
from dataclasses import dataclass, field
from typing import Any

from bothy_sdk.testing._fake_base import FakeProblem, field_error
from bothy_sdk.testing._fake_events import ConsumerCursor, Hook, LogEntry, Window, fill
from bothy_sdk.testing.doubles import FakeClock

Send = Callable[[dict[str, Any]], Awaitable[None]]
Close = Callable[[int, str], Awaitable[None]]


@dataclass(eq=False)
class Connection:
    """One identified gateway connection of a consumer."""

    consumer: str
    session_id: str
    send: Send
    close: Close
    window: Window
    ready_head: int
    live: bool = False
    last_frame: int = 0
    wake: asyncio.Event = field(default_factory=asyncio.Event)
    armed: asyncio.Event = field(default_factory=asyncio.Event)
    tasks: list[asyncio.Task[None]] = field(default_factory=list)
    closed: bool = False


class Hub:
    """Log, cursors and holders.

    Args:
        clock: The shared FakeClock.
        timings: The server keys the FakeServer plays.
        hook: The delivery hook of the run rules.
    """

    def __init__(self, clock: FakeClock, timings: Mapping[str, float], hook: Hook) -> None:
        self.clock = clock
        self.timings = timings
        self.hook = hook
        self.log: list[LogEntry] = []
        self.cursors: dict[str, ConsumerCursor] = {}
        self.rest: dict[str, Window] = {}
        self.connections: dict[str, Connection] = {}
        # The current holder per consumer: a gateway session ID or the REST window.
        self.holders: dict[str, str] = {}

    @property
    def head(self) -> int:
        """The highest ``seq`` in the log."""
        return self.log[-1].seq if self.log else 0

    def append(self, kind: str, envelope: dict[str, Any]) -> LogEntry:
        """Append a durable event and wake every runner."""
        entry = LogEntry(seq=self.head + 1, type=kind, envelope={**envelope, "seq": self.head + 1})
        payload = envelope.get("payload", {})
        entry.run_id = payload.get("run_id")
        self.log.append(entry)
        for conn in list(self.connections.values()):
            conn.wake.set()
        return entry

    def _max_inflight(self) -> int:
        return int(self.timings["agent.max_inflight_events"])

    def _ack_timeout(self) -> float:
        return self.timings["agent.ack_timeout"]

    # REST ---------------------------------------------------------------------------------

    async def poll(self, consumer: str, limit: int | None) -> dict[str, Any]:
        """``pollAgentEvents``: take the consumer over and deliver from the window."""
        conn = self.connections.pop(consumer, None)
        if conn is not None:
            await self._drop(conn, 4015, "consumer_replaced")
        self.holders[consumer] = "rest"
        cursor = self.cursors.get(consumer)
        created = cursor is None
        if cursor is None:
            cursor = self.cursors[consumer] = ConsumerCursor(self.head)
        window = self.rest.get(consumer)
        if window is None or created:
            window = self.rest[consumer] = Window(acked=cursor.acked)
        else:
            window.ack(cursor.acked)
            window.expire(self.clock.monotonic(), self._ack_timeout())
        if created:
            return {"events": [], "next_cursor": self.head, "head_seq": self.head}
        earlier = min(window.inflight) if window.inflight else None
        cap = self._max_inflight()
        events = fill(
            window,
            self.log,
            self.hook,
            now=self.clock.monotonic(),
            max_inflight=cap,
            limit=min(limit or cap, cap),
        )
        next_cursor = window.read if earlier is None else min(window.read, earlier - 1)
        return {"events": events, "next_cursor": next_cursor, "head_seq": self.head}

    def ack(self, consumer: str, seq: int) -> dict[str, Any]:
        """``ackAgentEvents``: store at once and monotonically, without a takeover."""
        if seq < 0 or seq > self.head:
            raise field_error("/seq", "invalid", "the sequence number is below 0 or above the head")
        cursor = self.cursors.get(consumer)
        if cursor is None:
            raise FakeProblem("not_found")
        cursor.acked = max(cursor.acked, seq)
        if consumer in self.rest:
            self.rest[consumer].ack(cursor.acked)
        conn = self.connections.get(consumer)
        if conn is not None:
            conn.window.ack(cursor.acked)
            conn.wake.set()
        return self.state(consumer)

    def state(self, consumer: str) -> dict[str, Any]:
        """``getConsumerState``.

        Raises:
            FakeProblem: ``404`` for a consumer that never connected.
        """
        cursor = self.cursors.get(consumer)
        if cursor is None:
            raise FakeProblem("not_found")
        return {"consumer": consumer, "acked_seq": cursor.acked, "head_seq": self.head}

    # Gateway ------------------------------------------------------------------------------

    async def attach(self, consumer: str, session_id: str, send: Send, close: Close) -> Connection:
        """Take the consumer over for a gateway connection after ``ready`` or ``resumed``."""
        cursor = self.cursors.setdefault(consumer, ConsumerCursor(self.head))
        conn = Connection(
            consumer=consumer,
            session_id=session_id,
            send=send,
            close=close,
            window=Window(acked=cursor.acked),
            ready_head=self.head,
        )
        old = self.connections.get(consumer)
        self.connections[consumer] = conn
        self.holders[consumer] = session_id
        self.rest.pop(consumer, None)
        if old is not None:
            await self._drop(old, 4015, "consumer_replaced")
        conn.tasks = [
            asyncio.create_task(self._run(conn)),
            asyncio.create_task(self._timer(conn)),
        ]
        return conn

    def gateway_ack(self, conn: Connection, seq: int) -> bool:
        """The op ``ack``; False when the ``seq`` is above what may be acknowledged."""
        if seq < 0 or seq > max(self.head, conn.window.read):
            return False
        cursor = self.cursors[conn.consumer]
        cursor.acked = max(cursor.acked, seq)
        conn.window.ack(cursor.acked)
        conn.wake.set()
        return True

    async def detach(self, conn: Connection) -> None:
        """Stop a connection's tasks (the connection ended)."""
        conn.closed = True
        if self.connections.get(conn.consumer) is conn:
            del self.connections[conn.consumer]
        current = asyncio.current_task()
        for task in conn.tasks:
            if task is not current:
                task.cancel()
                with contextlib.suppress(asyncio.CancelledError, Exception):
                    await task

    async def _drop(self, conn: Connection, code: int, reason: str) -> None:
        await self.detach(conn)
        with contextlib.suppress(Exception):
            await conn.close(code, reason)

    async def close_all(self, code: int, reason: str) -> None:
        """Close every gateway connection with ``code``."""
        for conn in list(self.connections.values()):
            await self._drop(conn, code, reason)

    def restart(self, consumer: str | None, acked_seq: int | None) -> None:
        """Forget in-memory windows (the process restarted), optionally set one cursor."""
        self.rest.clear()
        if consumer is not None and acked_seq is not None:
            self.cursors[consumer] = ConsumerCursor(acked_seq)

    async def ephemeral(self, envelope: dict[str, Any]) -> bool:
        """Send an ephemeral event to one live connection; False when none is live."""
        for conn in self.connections.values():
            if conn.live and not conn.closed:
                await conn.send({"op": "dispatch", "d": envelope})
                return True
        return False

    async def _run(self, conn: Connection) -> None:
        # The delivery runner: backlog, cursor and catchup.complete, then live dispatches.
        while not conn.closed:
            # Clear before reading the log, so an append during the sends below is not lost.
            conn.wake.clear()
            events = fill(
                conn.window,
                self.log,
                self.hook,
                now=self.clock.monotonic(),
                max_inflight=self._max_inflight(),
            )
            if events:
                conn.armed.set()
            for envelope in events:
                await conn.send({"op": "dispatch", "d": envelope})
                conn.last_frame = envelope["seq"]
            if not conn.live and conn.window.read >= conn.ready_head:
                if conn.window.read > conn.last_frame:
                    await conn.send({"op": "cursor", "d": {"seq": conn.window.read}})
                    conn.last_frame = conn.window.read
                head = max(conn.ready_head, conn.window.read)
                await conn.send({"op": "catchup.complete", "d": {"head_seq": head}})
                conn.live = True
            if not events:
                await conn.wake.wait()

    async def _timer(self, conn: Connection) -> None:
        # Redelivery after agent.ack_timeout, on the FakeClock.
        while not conn.closed:
            due = conn.window.deadline(self._ack_timeout())
            if due is None:
                conn.armed.clear()
                await conn.armed.wait()
                continue
            await self.clock.sleep(due - self.clock.monotonic())
            if conn.window.expire(self.clock.monotonic(), self._ack_timeout()):
                conn.wake.set()
