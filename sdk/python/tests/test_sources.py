# SPDX-License-Identifier: Apache-2.0
"""Event sources: envelopes, the ack window, cursor frames, polling waits and the heartbeat.

docs/spec/09-agents.md 4.2 and docs/DECISIONS.md D-M2-29-6 (Design note 7 of the brief): the
acknowledged ``seq`` is the highest one up to which every delivered event is settled; a cursor
counts as delivered and settled; the polling source polls again at once after a non-empty page
and waits ``sdk.poll_interval`` after an empty one; it sends ``sendHeartbeat`` every
``HeartbeatAck.interval_ms``.
"""

from __future__ import annotations

import asyncio
import json
from collections.abc import AsyncIterator, Callable
from pathlib import Path
from typing import Any
from uuid import UUID

import httpx
import pytest

import bothy_sdk.testing
from bothy_sdk.auth import Credentials, Secret
from bothy_sdk.client import Client
from bothy_sdk.gateway import CloseReason, GatewayEvent
from bothy_sdk.generated.models import (
    AgentInvocationEvent,
    CommandAutocompleteRequestedEvent,
    Heartbeat,
)
from bothy_sdk.settings import SdkSettings
from bothy_sdk.sources import (
    AckWindow,
    CursorAdvance,
    Delivery,
    Envelope,
    GatewaySource,
    PollingSource,
    SourceReady,
    envelope_from,
)
from bothy_sdk.testing import FakeClock, fingerprint
from bothy_sdk.testing._patience import eventually

pytestmark = pytest.mark.anyio

BASE = "https://example.com/api/v1"
CONSUMER = "alpha"
POLL_PATH = f"/api/v1/agents/me/consumers/{CONSUMER}/events"
ACK_PATH = f"/api/v1/agents/me/consumers/{CONSUMER}/ack"
HEARTBEAT_PATH = "/api/v1/agents/me/heartbeat"
SYNC_EPOCH = "01900000-0000-7000-8000-0000000000e1"
# A step that floats add exactly, so "just before" plus EPS lands on the deadline itself.
EPS = 0.25
FIXTURES = Path(bothy_sdk.testing.__file__).parent / "fixtures"
# A placeholder token: the tests compare fingerprints, the value carries no meaning.
TOKEN = "example-invocation-token-for-tests"  # noqa: S105 - not a credential


def _envelope(seq: int | None, **extra: Any) -> dict[str, Any]:
    """The recorded ``agent.invocation`` envelope with its own ``id`` and ``seq``."""
    data: dict[str, Any] = json.loads((FIXTURES / "agent.invocation.json").read_text())
    data["id"] = f"01900000-0000-7000-8000-{seq or 0:012d}"
    if seq is None:
        data.pop("seq", None)
    else:
        data["seq"] = seq
    data.update(extra)
    return data


def _autocomplete() -> dict[str, Any]:
    data: dict[str, Any] = json.loads(
        (FIXTURES / "command.autocomplete_requested.json").read_text()
    )
    return data


async def _turns(n: int = 200) -> None:
    """Let the other tasks run (HTTP through MockTransport takes a few loop turns)."""
    for _ in range(n):
        await asyncio.sleep(0)


async def _until(condition: Callable[[], bool]) -> None:
    await eventually(condition, "the condition")


# envelope_from ---------------------------------------------------------------------------------


def test_envelope_takes_the_token_out_of_raw() -> None:
    env = envelope_from(_envelope(5, invocation_token=TOKEN, redelivered=True))
    assert "invocation_token" not in env.raw
    assert env.token is not None
    assert fingerprint(env.token) == fingerprint(TOKEN)
    assert TOKEN not in repr(env)
    assert isinstance(env.event, AgentInvocationEvent)
    assert env.type == "agent.invocation"
    assert env.seq == 5
    assert env.id == UUID("01900000-0000-7000-8000-000000000005")
    assert env.redelivered is True
    assert env.ephemeral is False


def test_envelope_empty_token_counts_as_missing() -> None:
    env = envelope_from(_envelope(6, invocation_token=""))
    assert env.token is None
    assert "invocation_token" not in env.raw
    assert envelope_from(_envelope(7)).token is None
    assert envelope_from(_envelope(8)).redelivered is False


def test_envelope_autocomplete_decoded_as_ephemeral_event() -> None:
    env = envelope_from({**_autocomplete(), "invocation_token": TOKEN})
    assert isinstance(env.event, CommandAutocompleteRequestedEvent)
    assert env.seq is None
    assert env.ephemeral is True
    assert env.token is not None


@pytest.mark.parametrize("missing", ["id", "type"])
def test_envelope_without_id_or_type_raises(missing: str) -> None:
    data = _envelope(9)
    del data[missing]
    with pytest.raises(ValueError, match="id or type"):
        envelope_from(data)


# Ack window ------------------------------------------------------------------------------------


class _Responder:
    """A MockTransport handler of the polling operations that records every request."""

    def __init__(self, clock: FakeClock, pages: list[dict[str, Any]]) -> None:
        self.clock = clock
        self.pages = pages
        self.polls: list[float] = []
        self.acks: list[int] = []
        self.heartbeats: list[float] = []
        self.intervals_ms: list[int] = [3000]

    def __call__(self, request: httpx.Request) -> httpx.Response:
        path = request.url.path
        if request.method == "GET" and path == POLL_PATH:
            self.polls.append(self.clock.monotonic())
            page = self.pages.pop(0) if self.pages else {"events": [], "next_cursor": 0}
            return httpx.Response(200, json={"head_seq": 0, "sync_epoch": SYNC_EPOCH, **page})
        if request.method == "POST" and path == ACK_PATH:
            self.acks.append(int(json.loads(request.content)["seq"]))
            body = {"consumer": CONSUMER, "acked_seq": self.acks[-1], "head_seq": self.acks[-1]}
            return httpx.Response(200, json=body)
        if request.method == "POST" and path == HEARTBEAT_PATH:
            self.heartbeats.append(self.clock.monotonic())
            # Each answer takes the next interval; the last one repeats.
            many = len(self.intervals_ms) > 1
            interval = self.intervals_ms.pop(0) if many else self.intervals_ms[0]
            return httpx.Response(
                200,
                json={
                    "presence": {
                        "state": "online",
                        "last_heartbeat_at": "2026-01-01T00:00:00Z",
                        "billing_mode_matches": True,
                    },
                    "interval_ms": interval,
                },
            )
        return httpx.Response(404, json={"status": 404, "code": "not_found", "title": "x"})


def _client(clock: FakeClock, responder: _Responder, settings: SdkSettings) -> Client:
    return Client(
        BASE,
        Credentials(Secret("example-api-token")),
        settings=settings,
        clock=clock,
        transport=httpx.MockTransport(responder),
    )


class _Running:
    """Iterates a source in a task and collects its deliveries."""

    def __init__(self, source: PollingSource | GatewaySource) -> None:
        self.source = source
        self.deliveries: list[Delivery] = []
        self.task = asyncio.ensure_future(self._consume())

    async def _consume(self) -> None:
        async for delivery in self.source:
            self.deliveries.append(delivery)

    def envelopes(self) -> list[Envelope]:
        return [d for d in self.deliveries if isinstance(d, Envelope)]

    async def stop(self, clock: FakeClock) -> None:
        await self.source.close()
        await clock.advance(0)
        await _turns()
        if not self.task.done():
            self.task.cancel()
        await asyncio.gather(self.task, return_exceptions=True)


def test_ack_window_never_skips_unsettled_seq() -> None:
    window = AckWindow()
    for seq in (5, 6, 7):
        window.delivered(seq)
    assert window.settle(6) is None
    assert window.settle(7) is None
    assert window.acked == 0
    assert window.settle(5) == 7
    assert window.acked == 7
    # Settling again or settling an acknowledged seq moves nothing.
    assert window.settle(7) is None
    assert window.settle(4) is None


async def test_ack_never_skips_unsettled_seq() -> None:
    clock = FakeClock()
    page = {"events": [_envelope(s, invocation_token=TOKEN) for s in (5, 6, 7)], "next_cursor": 7}
    responder = _Responder(clock, [page])
    settings = SdkSettings(poll_interval=1000.0)
    async with _client(clock, responder, settings) as client:
        running = _Running(PollingSource(client, CONSUMER, settings=settings, clock=clock))
        await _until(lambda: len(running.envelopes()) == 3)
        assert [e.seq for e in running.envelopes()] == [5, 6, 7]
        await running.source.settle(6)
        await running.source.settle(7)
        assert responder.acks == []
        await running.source.settle(5)
        assert responder.acks == [7]
        await running.stop(clock)


async def test_cursor_frames_settle() -> None:
    window = AckWindow()
    window.delivered(3)
    # A cursor counts as delivered and settled, but an unsettled lower seq still blocks it.
    assert window.settle(5) is None
    assert window.settle(3) == 5
    # A cursor alone moves the point when nothing below it is open.
    assert window.settle(9) == 9
    # A cursor below an open seq moves the point up to the cursor only.
    window.delivered(12)
    assert window.settle(10) == 10
    assert window.settle(12) == 12

    class _Gateway:
        close_reason = CloseReason(1000, "done")

        def __init__(self) -> None:
            self.acks: list[int] = []

        async def frames(self) -> AsyncIterator[GatewayEvent]:
            yield GatewayEvent("ready", {})
            yield GatewayEvent("dispatch", _envelope(3, invocation_token=TOKEN))
            yield GatewayEvent("cursor", {"seq": 5})

        async def ack(self, seq: int) -> None:
            self.acks.append(seq)

    gateway = _Gateway()
    source = GatewaySource(gateway)  # type: ignore[arg-type]
    deliveries = [d async for d in source]
    assert isinstance(deliveries[0], SourceReady)
    assert isinstance(deliveries[1], Envelope)
    assert deliveries[1].seq == 3
    assert deliveries[2] == CursorAdvance(5)
    assert source.close_reason == CloseReason(1000, "done")
    await source.settle(5)
    assert gateway.acks == []
    await source.settle(3)
    assert gateway.acks == [5]


async def test_page_cursor_does_not_settle_an_unsettled_event() -> None:
    # The page cursor names the last seq read, which is the seq of the page's last event. The
    # adapter settles a CursorAdvance at once; that must not settle the event itself, which
    # stays unsettled while its token check failed (D-M2-29-6, 04 15.2 point 1).
    clock = FakeClock()
    page = {"events": [_envelope(s, invocation_token=TOKEN) for s in (5, 6, 7)], "next_cursor": 7}
    responder = _Responder(clock, [page])
    settings = SdkSettings(poll_interval=1000.0)
    async with _client(clock, responder, settings) as client:
        running = _Running(PollingSource(client, CONSUMER, settings=settings, clock=clock))
        await _until(lambda: CursorAdvance(7) in running.deliveries)
        await running.source.settle(5)
        await running.source.settle(6)
        await running.source.advance(7)  # the CursorAdvance, as the adapter passes it on
        assert max(responder.acks, default=0) <= 6
        await running.stop(clock)


async def test_polling_waits_poll_interval_only_after_empty_page() -> None:
    clock = FakeClock()
    interval = 7.0
    pages = [
        {"events": [_envelope(1, invocation_token=TOKEN)], "next_cursor": 1},
        {"events": [_envelope(2, invocation_token=TOKEN)], "next_cursor": 2},
        {"events": [], "next_cursor": 2},
        {"events": [], "next_cursor": 2},
    ]
    responder = _Responder(clock, pages)
    settings = SdkSettings(poll_interval=interval)
    async with _client(clock, responder, settings) as client:
        running = _Running(PollingSource(client, CONSUMER, settings=settings, clock=clock))
        await _until(lambda: len(responder.polls) == 3 and clock.sleepers == 1)
        # Two non-empty pages: the next poll came at once; the empty third page waits.
        assert responder.polls == [0.0, 0.0, 0.0]
        assert isinstance(running.deliveries[0], SourceReady)
        assert CursorAdvance(2) in running.deliveries
        await clock.advance(interval - EPS)
        await _turns()
        assert len(responder.polls) == 3
        await clock.advance(EPS)
        await _until(lambda: len(responder.polls) == 4 and clock.sleepers == 1)
        assert responder.polls[3] == pytest.approx(interval)
        # The fourth page is empty too: the next poll waits a full interval again.
        await clock.advance(interval - EPS)
        await _turns()
        assert len(responder.polls) == 4
        await clock.advance(EPS)
        await _until(lambda: len(responder.polls) == 5)
        assert responder.polls[4] == pytest.approx(2 * interval)
        await running.stop(clock)
        assert running.task.done()
        assert running.source.close_reason is not None


async def test_polling_heartbeat_every_interval_ms() -> None:
    clock = FakeClock()
    responder = _Responder(clock, [])
    responder.intervals_ms = [3000, 5000]
    settings = SdkSettings(poll_interval=1000.0)
    beat = Heartbeat.from_dict({"state": "online"})
    async with _client(clock, responder, settings) as client:
        source = PollingSource(
            client, CONSUMER, settings=settings, clock=clock, heartbeat=lambda: beat
        )
        running = _Running(source)
        await _until(lambda: len(responder.heartbeats) == 1 and clock.sleepers == 2)
        assert responder.heartbeats == [0.0]
        await clock.advance(3 - EPS)
        await _turns()
        assert len(responder.heartbeats) == 1
        await clock.advance(EPS)
        await _until(lambda: len(responder.heartbeats) == 2 and clock.sleepers == 2)
        # The second answer names 5000 ms: the third beat follows that interval.
        await clock.advance(5 - EPS)
        await _turns()
        assert len(responder.heartbeats) == 2
        await clock.advance(EPS)
        await _until(lambda: len(responder.heartbeats) == 3)
        assert responder.heartbeats == pytest.approx([0.0, 3.0, 8.0])
        await running.stop(clock)


async def test_polling_without_heartbeat_sends_none() -> None:
    clock = FakeClock()
    responder = _Responder(clock, [])
    settings = SdkSettings(poll_interval=1.0)
    async with _client(clock, responder, settings) as client:
        running = _Running(PollingSource(client, CONSUMER, settings=settings, clock=clock))
        await _until(lambda: len(responder.polls) == 1)
        await clock.advance(5)
        await _turns()
        assert responder.heartbeats == []
        await running.stop(clock)


class _Failing(_Responder):
    """Answers the first polls with 503 (as during a server restart), then pages."""

    def __init__(
        self,
        clock: FakeClock,
        pages: list[dict[str, Any]],
        failures: int,
        failure: Callable[[], httpx.Response] | None = None,
    ) -> None:
        super().__init__(clock, pages)
        self.failures = failures
        self.failure = failure or (lambda: httpx.Response(503, content=b"<html>restarting</html>"))

    def __call__(self, request: httpx.Request) -> httpx.Response:
        if request.url.path == POLL_PATH and self.failures > 0:
            self.failures -= 1
            self.polls.append(self.clock.monotonic())
            return self.failure()
        return super().__call__(request)


async def test_polling_survives_a_server_restart() -> None:
    clock = FakeClock()
    page = {"events": [_envelope(3, invocation_token=TOKEN)], "next_cursor": 3}
    responder = _Failing(clock, [page], failures=3)
    settings = SdkSettings(poll_interval=1000.0, retry_max_attempts=1)
    async with _client(clock, responder, settings) as client:
        running = _Running(PollingSource(client, CONSUMER, settings=settings, clock=clock))
        await eventually(running.envelopes, "the first page", step=lambda: clock.advance(1.0))
        assert [e.seq for e in running.envelopes()] == [3]
        assert not running.task.done()
        await running.stop(clock)


async def test_polling_survives_a_second_dpop_nonce_challenge() -> None:
    # A nonce rotation between the one retry of the auth flow and the next poll is transient:
    # `use_dpop_nonce` never stops the consumer like a revoked token (4004) would.
    def challenge() -> httpx.Response:
        return httpx.Response(
            401,
            json={
                "type": "about:blank",
                "title": "use_dpop_nonce",
                "status": 401,
                "code": "use_dpop_nonce",
                "correlation_id": "0192f4a0-7b1c-7000-8a00-000000000009",
            },
            headers={"Content-Type": "application/problem+json", "DPoP-Nonce": "n-2"},
        )

    clock = FakeClock()
    page = {"events": [_envelope(3, invocation_token=TOKEN)], "next_cursor": 3}
    responder = _Failing(clock, [page], failures=2, failure=challenge)
    settings = SdkSettings(poll_interval=1000.0, retry_max_attempts=1)
    async with _client(clock, responder, settings) as client:
        source = PollingSource(client, CONSUMER, settings=settings, clock=clock)
        running = _Running(source)
        await eventually(running.envelopes, "the first page", step=lambda: clock.advance(1.0))
        assert [e.seq for e in running.envelopes()] == [3]
        assert source.close_reason is None
        await running.stop(clock)
