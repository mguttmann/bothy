# SPDX-License-Identifier: Apache-2.0
"""Test doubles of the hand layer: a fake clock, fingerprints and recording helpers.

`FakeClock` drives every time decision of the SDK and of the FakeServer without sleeping;
`fingerprint` lets a test compare tokens without ever printing one (SEC-LOG-2).

Examples:
    >>> import asyncio
    >>> clock = FakeClock()
    >>> async def demo() -> float:
    ...     task = asyncio.create_task(clock.sleep(5))
    ...     await clock.advance(5)
    ...     await task
    ...     return clock.monotonic()
    >>> asyncio.run(demo())
    5.0
"""

from __future__ import annotations

import asyncio
import dataclasses
import hashlib
import heapq
import itertools
from collections.abc import AsyncIterator, Awaitable, Callable, Sequence
from datetime import UTC, datetime, timedelta
from typing import Any
from uuid import UUID

from bothy_sdk.adapter import ActualBilling, AgentHandler, Invocation, StartOptions
from bothy_sdk.auth import Secret
from bothy_sdk.commands import AutocompleteRequest
from bothy_sdk.gateway import CloseReason
from bothy_sdk.generated.models import CommandChoice, ToolClass
from bothy_sdk.runs import RunHandle
from bothy_sdk.sources import Delivery, Envelope, EventSource

__all__ = [
    "KEEP",
    "REMOVE",
    "FakeClock",
    "RecordingHandler",
    "RecordingSource",
    "TamperingSource",
    "fingerprint",
]

_SETTLE_TURNS = 20


def fingerprint(token: str | object) -> str:
    """Return a short, stable fingerprint of a token for assertions and logs.

    Args:
        token: A token string or a ``bothy_sdk.auth.Secret``.

    Returns:
        The first 16 hex characters of the SHA-256 of the token.
    """
    reveal = getattr(token, "reveal", None)
    text = reveal() if callable(reveal) else str(token)
    return hashlib.sha256(str(text).encode("utf-8")).hexdigest()[:16]


class FakeClock:
    """A clock that moves only when a test calls `advance`.

    ``now`` starts at ``start`` (UTC) and ``monotonic`` at zero; `sleep` waits until `advance`
    passes its deadline, and `advance` wakes sleepers in deadline order, letting each woken task
    run before the time moves on.

    Args:
        start: The wall time at monotonic zero.
    """

    def __init__(self, start: datetime | None = None) -> None:
        """Create the clock; see the class docstring."""
        self._start = start or datetime(2026, 1, 1, tzinfo=UTC)
        self._offset = 0.0
        self._order = itertools.count()
        self._sleepers: list[tuple[float, int, asyncio.Future[None]]] = []

    def now(self) -> datetime:
        """Return the fake UTC wall time."""
        return self._start + timedelta(seconds=self._offset)

    def monotonic(self) -> float:
        """Return the seconds advanced since the start."""
        return self._offset

    async def sleep(self, seconds: float) -> None:
        """Wait until the clock has advanced by ``seconds``.

        Args:
            seconds: The wait; zero or negative yields once to the loop.
        """
        if seconds <= 0:
            await asyncio.sleep(0)
            return
        fut: asyncio.Future[None] = asyncio.get_running_loop().create_future()
        heapq.heappush(self._sleepers, (self._offset + seconds, next(self._order), fut))
        await fut

    @property
    def sleepers(self) -> int:
        """The number of tasks currently waiting in `sleep`."""
        return sum(1 for _, _, f in self._sleepers if not f.done())

    async def advance(self, seconds: float) -> None:
        """Move the time forward by ``seconds``, waking sleepers in deadline order.

        Args:
            seconds: How far to move; must not be negative.

        Raises:
            ValueError: ``seconds`` is negative.
        """
        if seconds < 0:
            raise ValueError("time only moves forward")
        target = self._offset + seconds
        while self._sleepers and self._sleepers[0][0] <= target:
            deadline, _, fut = heapq.heappop(self._sleepers)
            self._offset = max(self._offset, deadline)
            if not fut.done():
                fut.set_result(None)
            await _settle()
        self._offset = target
        await _settle()


async def _settle() -> None:
    # Let woken tasks run their synchronous parts before the time moves on.
    for _ in range(_SETTLE_TURNS):
        await asyncio.sleep(0)


class _Marker:
    def __init__(self, name: str) -> None:
        self._name = name

    def __repr__(self) -> str:
        return self._name


KEEP = _Marker("KEEP")
"""Rewrite result of `TamperingSource`: keep the delivered token."""
REMOVE = _Marker("REMOVE")
"""Rewrite result of `TamperingSource`: remove the delivered token."""

Rewrite = Callable[[Envelope], Awaitable[Secret | _Marker]]


class _Wrapper:
    """Forwards everything but iteration to the wrapped source."""

    def __init__(self, inner: EventSource) -> None:
        self.inner = inner

    @property
    def close_reason(self) -> CloseReason | None:
        return self.inner.close_reason

    async def settle(self, seq: int) -> None:
        await self.inner.settle(seq)

    async def advance(self, seq: int) -> None:
        await self.inner.advance(seq)

    async def ack(self, seq: int) -> None:
        await self.inner.ack(seq)

    async def answer_autocomplete(self, request_id: UUID, choices: Sequence[CommandChoice]) -> None:
        await self.inner.answer_autocomplete(request_id, choices)

    async def close(self) -> None:
        await self.inner.close()


class TamperingSource(_Wrapper):
    """Rewrites or removes the ``invocation_token`` of chosen deliveries (AG-08, AZ-P6).

    Args:
        inner: The real source.
        rewrite: Called for every envelope with a token-bearing type; returns a new token,
            `KEEP` or `REMOVE`. It may wait (for example until the scenario forged the token).
    """

    def __init__(self, inner: EventSource, rewrite: Rewrite) -> None:
        """Wrap ``inner``."""
        super().__init__(inner)
        self._rewrite = rewrite

    async def __aiter__(self) -> AsyncIterator[Delivery]:
        """Yield the inner deliveries with rewritten tokens."""
        async for delivery in self.inner:
            out = delivery
            if isinstance(delivery, Envelope) and delivery.token is not None:
                result = await self._rewrite(delivery)
                if result is REMOVE:
                    out = dataclasses.replace(delivery, token=None)
                elif isinstance(result, Secret):
                    out = dataclasses.replace(delivery, token=result)
            yield out


class RecordingSource(_Wrapper):
    """Records every delivery (tokens stay in memory as ``Secret``, never printed).

    Args:
        inner: The real source.
    """

    def __init__(self, inner: EventSource) -> None:
        """Wrap ``inner``."""
        super().__init__(inner)
        self.deliveries: list[Delivery] = []

    async def __aiter__(self) -> AsyncIterator[Delivery]:
        """Yield and record the inner deliveries."""
        async for delivery in self.inner:
            self.deliveries.append(delivery)
            yield delivery

    def envelopes(self, event_type: str | None = None) -> list[Envelope]:
        """Return the recorded envelopes, optionally of one type.

        Args:
            event_type: The event type to keep.

        Returns:
            The envelopes in delivery order.
        """
        return [
            d
            for d in self.deliveries
            if isinstance(d, Envelope) and (event_type is None or d.type == event_type)
        ]


class RecordingHandler(AgentHandler):
    """An `AgentHandler` that records its calls and can block in ``prepare`` or ``execute``.

    Args:
        billing: The actual billing mode; ``None`` follows the token's claim.
        tool_class: The actual tool class.
        block_prepare: When set, ``prepare`` waits for this event.
        block_execute: When set, ``execute`` waits for this event.
    """

    def __init__(
        self,
        *,
        billing: str | None = None,
        tool_class: ToolClass = ToolClass.CHAT,
        block_prepare: asyncio.Event | None = None,
        block_execute: asyncio.Event | None = None,
    ) -> None:
        """Create the handler; see the class docstring."""
        self.billing = billing
        self.tool_class = tool_class
        self.block_prepare = block_prepare
        self.block_execute = block_execute
        self.calls: list[tuple[str, UUID | None]] = []
        self.handles: dict[UUID, RunHandle] = {}
        self.invocations: list[Invocation] = []
        self.events: list[Envelope] = []
        self.autocomplete_choices: Sequence[CommandChoice] = ()
        self.autocomplete_delay: float = 0.0
        self.clock: Any = None
        self.changed = asyncio.Event()

    def _record(self, name: str, run_id: UUID | None) -> None:
        self.calls.append((name, run_id))
        self.changed.set()

    def count(self, name: str, run_id: UUID | None = None) -> int:
        """Return how often ``name`` was called (for ``run_id`` when given).

        Args:
            name: ``prepare``, ``execute``, ``on_followup`` ...
            run_id: The run to count for.

        Returns:
            The count.
        """
        return sum(1 for n, r in self.calls if n == name and (run_id is None or r == run_id))

    def actual_billing(self, inv: Invocation) -> ActualBilling:
        """Return ``billing`` or the claim's mode."""
        return ActualBilling(self.billing or inv.claims.billing_mode)

    def actual_tool_class(self) -> ToolClass:
        """Return ``tool_class``."""
        return self.tool_class

    async def prepare(self, inv: Invocation, run: RunHandle) -> StartOptions:
        """Record, wait for ``block_prepare`` and return no options."""
        self._record("prepare", run.run_id)
        self.invocations.append(inv)
        self.handles[run.run_id] = run
        if self.block_prepare is not None:
            await self.block_prepare.wait()
        return StartOptions(model="model-test")

    async def execute(self, inv: Invocation, run: RunHandle) -> None:
        """Record and wait for ``block_execute``."""
        self._record("execute", run.run_id)
        self.handles[run.run_id] = run
        if self.block_execute is not None:
            await self.block_execute.wait()

    async def on_followup(self, inv: Invocation, run: RunHandle) -> None:
        """Record."""
        self._record("on_followup", run.run_id)

    async def on_interaction(self, inv: Invocation, run: RunHandle) -> None:
        """Record."""
        self._record("on_interaction", run.run_id)

    async def on_command(self, inv: Invocation, run: RunHandle | None) -> None:
        """Record."""
        self._record("on_command", run.run_id if run is not None else None)

    async def on_autocomplete(self, request: AutocompleteRequest) -> Sequence[CommandChoice]:
        """Record and return ``autocomplete_choices`` after ``autocomplete_delay``."""
        self._record("on_autocomplete", None)
        if self.autocomplete_delay and self.clock is not None:
            await self.clock.sleep(self.autocomplete_delay)
        return self.autocomplete_choices

    async def on_event(self, envelope: Envelope) -> None:
        """Record."""
        self.events.append(envelope)
        self._record("on_event", None)
