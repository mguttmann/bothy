# SPDX-License-Identifier: Apache-2.0
"""Event sources of an agent adapter: the gateway or REST polling (docs/spec/09-agents.md 4.2).

A source yields deliveries: `Envelope` (one event with its delivery fields), `CursorAdvance`
(the cursor moves without an event) and `SourceReady` (a connection is ready; the adapter flushes
its outbox and reconciles its commands). The adapter settles each delivered ``seq`` with
`EventSource.settle`; the source acknowledges the highest ``seq`` up to which every delivered
event is settled (docs/DECISIONS.md D-M2-29-6). Because ``ack`` is cumulative (04 15.2 point 1),
one unsettled event blocks the acknowledgement of every later one.

Examples:
    >>> window = AckWindow()
    >>> window.delivered(5); window.delivered(6)
    >>> (window.settle(6), window.settle(5))
    (None, 6)
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import logging
import secrets
from collections.abc import AsyncIterator, Callable, Mapping, Sequence
from dataclasses import dataclass
from typing import Any, Protocol, runtime_checkable
from uuid import UUID

import httpx

from bothy_sdk.auth import Secret
from bothy_sdk.client import Client, expect
from bothy_sdk.clock import Clock, SystemClock
from bothy_sdk.errors import (
    ForbiddenError,
    GoneError,
    ProblemError,
    UnauthenticatedError,
    UseDpopNonce,
)
from bothy_sdk.gateway import CloseReason, Gateway, GatewayEvent
from bothy_sdk.generated.api.agent_runtime import (
    ack_agent_events,
    answer_autocomplete,
    poll_agent_events,
    send_heartbeat,
)
from bothy_sdk.generated.models import (
    AutocompleteAnswer,
    CommandAutocompleteRequestedEvent,
    CommandChoice,
    ConsumerAck,
    Heartbeat,
    HeartbeatAck,
)
from bothy_sdk.settings import SdkSettings
from bothy_sdk.tolerance import KnownEvent, UnknownEvent, decode_event

__all__ = [
    "AckWindow",
    "CursorAdvance",
    "Delivery",
    "Envelope",
    "EventSource",
    "GatewaySource",
    "PollingSource",
    "SourceReady",
    "envelope_from",
]

_LOG = logging.getLogger("bothy_sdk")
_AUTOCOMPLETE = "command.autocomplete_requested"
_TOKEN_FIELD = "invocation_token"  # noqa: S105 - a member name, not a secret  # nosec B105


@dataclass(frozen=True)
class Envelope:
    """One delivered event.

    Attributes:
        raw: The envelope as received, without ``invocation_token``.
        event: The event decoded with the generated core (or ``UnknownEvent``).
        id: The envelope ``id``.
        type: The event type, for example ``agent.invocation``.
        seq: The ``seq``; ``None`` for an ephemeral event.
        token: The delivery field ``invocation_token``, if any.
        redelivered: The delivery field ``redelivered``.
    """

    raw: Mapping[str, Any]
    event: KnownEvent | UnknownEvent
    id: UUID
    type: str
    seq: int | None
    token: Secret | None
    redelivered: bool

    @property
    def ephemeral(self) -> bool:
        """True for an event without ``seq`` (no inbox, no acknowledgement)."""
        return self.seq is None


@dataclass(frozen=True)
class CursorAdvance:
    """The cursor moved to ``seq`` without an event (a ``cursor`` frame or a poll cursor)."""

    seq: int


@dataclass(frozen=True)
class SourceReady:
    """A connection is ready (``ready`` or ``resumed`` of the gateway, the first poll).

    Attributes:
        resumed: True after a ``resume``.
        resync: True after ``resync_required`` (inbox entries keep their state).
    """

    resumed: bool = False
    resync: bool = False


Delivery = Envelope | CursorAdvance | SourceReady
"""What a source yields."""


def envelope_from(data: Mapping[str, Any]) -> Envelope:
    """Build an `Envelope` from a received envelope, taking the token out of it.

    Args:
        data: The envelope (``d`` of a dispatch or an item of ``EventPage.events``).

    Returns:
        The envelope.

    Raises:
        ValueError: ``id`` or ``type`` is missing or malformed.
    """
    raw = dict(data)
    token_text = raw.pop(_TOKEN_FIELD, None)
    # An empty token counts as missing; the adapter refuses a token-bearing event without one.
    token = Secret(token_text) if isinstance(token_text, str) and token_text else None
    event_type = raw.get("type")
    if not isinstance(event_type, str) or "id" not in raw:
        raise ValueError("envelope without id or type")
    seq = raw.get("seq")
    if event_type == _AUTOCOMPLETE:
        # The ephemeral event is not part of the durable Event union of the core.
        event: KnownEvent | UnknownEvent = CommandAutocompleteRequestedEvent.from_dict(raw)
    else:
        event = decode_event(raw)
    return Envelope(
        raw=raw,
        event=event,
        id=UUID(str(raw["id"])),
        type=event_type,
        seq=seq if isinstance(seq, int) else None,
        token=token,
        redelivered=bool(raw.get("redelivered", False)),
    )


class AckWindow:
    """Tracks delivered and settled ``seq`` values and computes the acknowledgement point."""

    def __init__(self) -> None:
        """Create an empty window."""
        self._open: set[int] = set()
        self._settled: set[int] = set()
        self._acked = 0

    @property
    def acked(self) -> int:
        """The highest acknowledged ``seq``."""
        return self._acked

    def delivered(self, seq: int) -> None:
        """Note a delivered event.

        Args:
            seq: Its ``seq``.
        """
        if seq > self._acked:
            self._open.add(seq)

    def settle(self, seq: int) -> int | None:
        """Note a settled event or cursor and return a new acknowledgement point, if any.

        Args:
            seq: The settled ``seq`` (a cursor counts as delivered and settled).

        Returns:
            The new highest ``seq`` up to which everything delivered is settled, or ``None``
            when the point did not move.
        """
        if seq <= self._acked:
            return None
        self._open.discard(seq)
        self._settled.add(seq)
        limit = min(self._open) if self._open else None
        candidates = [s for s in self._settled if limit is None or s < limit]
        if not candidates:
            return None
        point = max(candidates)
        self._settled = {s for s in self._settled if s > point}
        self._acked = point
        return point

    def cursor(self, seq: int) -> int | None:
        """Note a cursor (a ``cursor`` frame or a poll cursor) and return a new point, if any.

        A cursor says that nothing undelivered lies at or below ``seq``; it never settles an
        event that was delivered and is still open, so a cursor equal to the ``seq`` of such an
        event changes nothing.

        Args:
            seq: The cursor.

        Returns:
            The new acknowledgement point, or ``None``.
        """
        if seq in self._open:
            return None
        return self.settle(seq)

    def reset(self) -> None:
        """Start over after a (re)connect.

        The server delivers again from its stored cursor, which a server restart may have set
        back (04 15.2 point 4), so the window forgets its own point and acknowledges again.
        """
        self._open.clear()
        self._settled.clear()
        self._acked = 0


@runtime_checkable
class EventSource(Protocol):
    """A transport of events for one consumer: an async iterator of deliveries plus ``ack``."""

    @property
    def close_reason(self) -> CloseReason | None:
        """Why the source stopped, once it has."""
        ...

    def __aiter__(self) -> AsyncIterator[Delivery]:
        """Iterate the deliveries until the source stops."""
        ...

    async def settle(self, seq: int) -> None:
        """Mark ``seq`` settled and acknowledge when the acknowledgement point moves.

        Args:
            seq: The settled ``seq``.
        """
        ...

    async def advance(self, seq: int) -> None:
        """Pass on a cursor; it never settles an open delivered event.

        Args:
            seq: The cursor.
        """
        ...

    async def ack(self, seq: int) -> None:
        """Acknowledge every event up to ``seq``.

        Args:
            seq: The ``seq``.
        """
        ...

    async def answer_autocomplete(self, request_id: UUID, choices: Sequence[CommandChoice]) -> None:
        """Send the choices of an autocomplete request.

        Args:
            request_id: The request.
            choices: The choices.
        """
        ...

    async def close(self) -> None:
        """Stop the source; the iterator ends."""
        ...


class _WindowedSource:
    """The settle and acknowledgement logic shared by both sources."""

    def __init__(self) -> None:
        self.window = AckWindow()
        self.close_reason: CloseReason | None = None
        self._lock = asyncio.Lock()

    async def ack(self, seq: int) -> None:
        raise NotImplementedError  # pragma: no cover

    async def settle(self, seq: int) -> None:
        async with self._lock:
            point = self.window.settle(seq)
            if point is not None:
                await self.ack(point)

    async def advance(self, seq: int) -> None:
        async with self._lock:
            point = self.window.cursor(seq)
            if point is not None:
                await self.ack(point)


class GatewaySource(_WindowedSource):
    """Events over the gateway; acknowledged with the op ``ack``.

    Args:
        gateway: The gateway session.
    """

    def __init__(self, gateway: Gateway) -> None:
        """Wrap ``gateway``."""
        super().__init__()
        self.gateway = gateway
        self._errors: list[GatewayEvent] = []
        self.on_error: Callable[[GatewayEvent], None] | None = None

    async def __aiter__(self) -> AsyncIterator[Delivery]:
        """Yield the deliveries of the gateway until it stops."""
        async for frame in self.gateway.frames():
            if frame.op in ("ready", "resumed"):
                self.window.reset()
                yield SourceReady(resumed=frame.op == "resumed")
            elif frame.op == "resync_required":
                self.window.reset()
                yield SourceReady(resync=True)
            elif frame.op == "dispatch":
                try:
                    envelope = envelope_from(frame.data)
                except (ValueError, TypeError):
                    _LOG.info("gateway: dispatch without id or type skipped")
                    continue
                if envelope.seq is not None:
                    self.window.delivered(envelope.seq)
                yield envelope
            elif frame.op == "cursor":
                yield CursorAdvance(int(frame.data.get("seq", 0)))
            elif frame.op == "error" and self.on_error is not None:
                self.on_error(frame)
        self.close_reason = self.gateway.close_reason

    async def ack(self, seq: int) -> None:
        """Send the op ``ack``.

        Args:
            seq: The highest settled ``seq``.
        """
        await self.gateway.ack(seq)

    async def answer_autocomplete(self, request_id: UUID, choices: Sequence[CommandChoice]) -> None:
        """Send the op ``command.autocomplete_result``.

        Args:
            request_id: The request.
            choices: The choices.
        """
        await self.gateway.send_autocomplete_result(request_id, choices)

    async def close(self) -> None:
        """Close the gateway with ``1000``."""
        await self.gateway.close()


def _page_envelopes(events: object) -> list[Envelope]:
    out = []
    for item in events if isinstance(events, list) else []:
        if not isinstance(item, dict):
            continue
        try:
            out.append(envelope_from(item))
        except (ValueError, TypeError):
            continue
    return out


class PollingSource(_WindowedSource):
    """Events by REST polling (``pollAgentEvents``), acknowledged with ``ackAgentEvents``.

    It polls again at once after a non-empty page and waits ``sdk.poll_interval`` after an
    empty one; it sends the agent heartbeat with ``sendHeartbeat`` every
    ``HeartbeatAck.interval_ms``.

    Args:
        client: The REST client.
        consumer: The consumer name.
        settings: The SDK settings (``sdk.poll_interval``).
        clock: The time source.
        heartbeat: Returns the agent heartbeat to send, or ``None`` for none.
        limit: The page size asked for (the server caps it).
    """

    def __init__(  # noqa: PLR0913 - every input of the poll loop is explicit
        self,
        client: Client,
        consumer: str,
        *,
        settings: SdkSettings | None = None,
        clock: Clock | None = None,
        heartbeat: Callable[[], Heartbeat | None] | None = None,
        limit: int | None = None,
    ) -> None:
        """Create the source; nothing is sent before iterating."""
        super().__init__()
        self.client = client
        self.consumer = consumer
        self.settings = settings or client.settings
        self.clock = clock or client.clock or SystemClock()
        self._heartbeat = heartbeat
        self._limit = limit
        self._stopped = asyncio.Event()
        self._rng = secrets.SystemRandom()

    async def __aiter__(self) -> AsyncIterator[Delivery]:
        """Poll until `close` is called."""
        beats = asyncio.create_task(self._beat()) if self._heartbeat is not None else None
        failures = 0
        try:
            yield SourceReady()
            while not self._stopped.is_set():
                outcome, body = await self._poll()
                if outcome == "stop":
                    return
                if outcome == "resync":
                    self.window.reset()
                    yield SourceReady(resync=True)
                    continue
                if outcome == "retry":
                    await self._wait(self._backoff(failures))
                    failures += 1
                    continue
                failures = 0
                events = body.get("events", [])
                for envelope in _page_envelopes(events):
                    if envelope.seq is not None:
                        self.window.delivered(envelope.seq)
                    yield envelope
                cursor = body.get("next_cursor")
                if isinstance(cursor, int):
                    yield CursorAdvance(cursor)
                if not events:
                    await self._wait(self.settings.poll_interval)
        finally:
            if beats is not None:
                beats.cancel()
                with contextlib.suppress(asyncio.CancelledError):
                    await beats
        if self.close_reason is None:
            self.close_reason = CloseReason(1000, "polling stopped")

    async def _poll(self) -> tuple[str, dict[str, Any]]:
        """One ``pollAgentEvents``: ``page``, ``resync``, ``retry`` or ``stop``."""
        kwargs: dict[str, Any] = {"consumer": self.consumer}
        if self._limit is not None:
            kwargs["limit"] = self._limit
        try:
            response = await self.client.call_detailed(poll_agent_events.asyncio_detailed, **kwargs)
            body = json.loads(response.content)
        except GoneError:
            # 410 resync_required: the server moved the cursor to head_seq.
            return "resync", {}
        except UseDpopNonce:
            # The nonce rotated again after the one retry of the auth flow: transient, the next
            # poll after the backoff carries the newest nonce.
            _LOG.info("poll met a new DPoP nonce; polling again")
            return "retry", {}
        except (UnauthenticatedError, ForbiddenError) as exc:
            # The API token is invalid or revoked: only the owner can fix that.
            _LOG.error("polling stopped: %s", exc.code)
            self.close_reason = CloseReason(4004, exc.code)
            return "stop", {}
        except (ProblemError, httpx.TransportError, ValueError) as exc:
            # A server restart or an outage outlasted the retries: wait with full jitter between
            # sync.reconnect_backoff_base and sync.reconnect_backoff_cap, then poll again.
            _LOG.info("poll failed (%s); polling again", type(exc).__name__)
            return "retry", {}
        return ("page", body) if isinstance(body, dict) else ("retry", {})

    def _backoff(self, attempt: int) -> float:
        cap = min(
            self.settings.reconnect_backoff_cap,
            self.settings.reconnect_backoff_base * (2**attempt),
        )
        return self._rng.uniform(0.0, cap)

    async def _wait(self, seconds: float) -> None:
        sleeper = asyncio.create_task(self.clock.sleep(seconds))
        stopper = asyncio.create_task(self._stopped.wait())
        _, pending = await asyncio.wait({sleeper, stopper}, return_when=asyncio.FIRST_COMPLETED)
        for task in pending:
            task.cancel()

    async def _beat(self) -> None:
        interval = self.settings.poll_interval
        while not self._stopped.is_set():
            beat = self._heartbeat() if self._heartbeat is not None else None
            if beat is not None:
                try:
                    result = await self.client.call(send_heartbeat.asyncio_detailed, body=beat)
                    interval = expect(result, HeartbeatAck).interval_ms / 1000.0
                except Exception:  # a failed heartbeat is retried in the next interval
                    _LOG.info("heartbeat failed; retrying in the next interval")
            await self._wait(interval)

    async def ack(self, seq: int) -> None:
        """Call ``ackAgentEvents``; a failure is logged, the next acknowledgement covers it.

        Args:
            seq: The highest settled ``seq``.
        """
        try:
            await self.client.call(
                ack_agent_events.asyncio_detailed,
                consumer=self.consumer,
                body=ConsumerAck(seq=seq),
            )
        except (ProblemError, httpx.TransportError) as exc:
            # ack is cumulative: a later one covers this seq; without one the server delivers
            # again after agent.ack_timeout and the inbox settles the duplicate.
            _LOG.info("ack %d failed (%s)", seq, type(exc).__name__)

    async def answer_autocomplete(self, request_id: UUID, choices: Sequence[CommandChoice]) -> None:
        """Call ``answerAutocomplete``.

        Args:
            request_id: The request.
            choices: The choices.
        """
        await self.client.call(
            answer_autocomplete.asyncio_detailed,
            request_id=request_id,
            body=AutocompleteAnswer(choices=list(choices)),
        )

    async def close(self) -> None:
        """Stop polling after the current request."""
        self._stopped.set()
