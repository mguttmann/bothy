# SPDX-License-Identifier: Apache-2.0
"""Gateway: frames, heartbeat, close codes, reconnect and tolerance against a scripted server."""

from __future__ import annotations

import asyncio
import contextlib
import json
import random
from collections.abc import AsyncIterator, Awaitable, Callable
from typing import Any
from uuid import UUID

import pytest
from conftest import schema
from websockets.asyncio.server import ServerConnection, serve

from bothy_sdk.auth import Credentials, Secret
from bothy_sdk.gateway import IDENTIFY_ON, RESUME_ON, STOP_ON, Gateway, GatewayEvent
from bothy_sdk.settings import SdkSettings
from bothy_sdk.testing._patience import eventually, within_patience
from bothy_sdk.testing.doubles import FakeClock

AGENT = UUID("0192f4a0-7b1c-7000-8a00-0000000000a2")
SESSION = "0192f4a0-7b1c-7000-8a00-0000000000b1"
EPOCH = "0192f4a0-7b1c-7000-8a00-0000000000c1"
TOKEN = "gateway-api-token-1"  # noqa: S105 - a test value
FAST = SdkSettings(reconnect_backoff_base=0.01, reconnect_backoff_cap=0.02)


def seeded(seed: int) -> random.Random:
    return random.Random(seed)  # noqa: S311 - deterministic jitter for the test


def hello(interval_ms: int = 600000) -> str:
    return json.dumps(
        {
            "op": "hello",
            "d": {
                "heartbeat_interval_ms": interval_ms,
                "resume_window_ms": 300000,
                "protocol_versions": [1],
                "dpop_nonce": "nonce-1",
                "server_time": "2026-01-01T00:00:00Z",
                "sync_epoch": EPOCH,
            },
        }
    )


def ready(principal: UUID = AGENT) -> str:
    who = {"id": str(principal), "kind": "agent", "handle": "lotse", "display_name": "Lotse"}
    snapshot = {
        "head_seq": 10,
        "permission_version": 1,
        "sync_epoch": EPOCH,
        "principal": who,
        "is_owner": False,
        "spaces": [],
        "channels": [],
        "sidebar": [],
        "unread_threads": [],
        "active_runs": [],
        "agents": [],
        "settings": [],
        "drafts": [],
        "features": [],
    }
    return json.dumps(
        {
            "op": "ready",
            "d": {
                "gateway_session_id": SESSION,
                "principal": who,
                "permission_version": 1,
                "head_seq": 10,
                "snapshot": snapshot,
                "sync_epoch": EPOCH,
            },
        }
    )


Script = Callable[[ServerConnection, int], Awaitable[None]]


class Server:
    """A scripted gateway: each connection runs `script(ws, n)`; frames it got are recorded."""

    def __init__(self, script: Script) -> None:
        self.script = script
        self.connections = 0
        self.frames: list[dict[str, Any]] = []
        self.url = ""

    async def handler(self, ws: ServerConnection) -> None:
        self.connections += 1
        with contextlib.suppress(Exception):
            await self.script(ws, self.connections)

    async def recv(self, ws: ServerConnection) -> dict[str, Any]:
        frame = json.loads(await ws.recv())
        self.frames.append(frame)
        return frame


@contextlib.asynccontextmanager
async def running(script: Script) -> AsyncIterator[Server]:
    server = Server(script)
    async with serve(server.handler, "127.0.0.1", 0) as ws_server:
        port = next(iter(ws_server.sockets)).getsockname()[1]
        server.url = f"ws://127.0.0.1:{port}/api/v1/gateway"
        yield server


def gateway(
    server: Server,
    *,
    clock: Any = None,
    rng: random.Random | None = None,
    settings: SdkSettings = FAST,
) -> Gateway:
    return Gateway(
        server.url,
        Credentials(Secret(TOKEN)),
        consumer="alpha",
        agent_id=AGENT,
        settings=settings,
        clock=clock,
        rng=rng or seeded(1),
    )


async def collect(gw: Gateway, until: Callable[[list[GatewayEvent]], bool]) -> list[GatewayEvent]:
    seen: list[GatewayEvent] = []

    async def run() -> None:
        async for frame in gw.frames():
            seen.append(frame)
            if until(seen):
                await gw.close()

    await within_patience(run())
    return seen


def _matches_schema(frame: dict[str, Any], name: str) -> None:
    s = schema(name)
    assert set(s.get("required", [])) <= set(frame)
    assert set(frame) <= set(s["properties"]), f"{name}: {set(frame) - set(s['properties'])}"


@pytest.mark.anyio
async def test_identify_and_resume_frames_match_schemas() -> None:
    async def script(ws: ServerConnection, n: int) -> None:
        await ws.send(hello())
        await server.recv(ws)
        await ws.send(ready())
        if n == 1:
            await ws.close(1001, "going away")
            return
        await ws.wait_closed()

    async with running(script) as server:
        gw = gateway(server)
        await collect(gw, lambda seen: server.connections >= 2 and len(server.frames) >= 2)
    identify, resume = server.frames[0], server.frames[1]
    assert identify["op"] == "identify"
    assert resume["op"] == "resume"
    _matches_schema(identify["d"], "GatewayIdentify")
    _matches_schema(resume["d"], "GatewayResume")
    assert identify["d"]["consumer"] == "alpha"
    assert identify["d"]["protocol_version"] == 1
    assert resume["d"]["gateway_session_id"] == SESSION
    assert resume["d"]["cursor"] == 0
    assert identify["d"]["api_token"] == TOKEN  # the token goes only into the frame


@pytest.mark.anyio
async def test_first_heartbeat_jittered() -> None:
    clock = FakeClock()
    got = asyncio.Event()

    async def script(ws: ServerConnection, n: int) -> None:
        await ws.send(hello(interval_ms=10000))
        await server.recv(ws)
        await ws.send(ready())
        frame = await server.recv(ws)
        if frame["op"] == "heartbeat":
            got.set()
        await ws.wait_closed()

    expected = seeded(5).random() * 10.0
    async with running(script) as server:
        gw = gateway(server, clock=clock, rng=seeded(5))
        task = asyncio.ensure_future(collect(gw, lambda s: False))
        # The heartbeat loop sleeps on the clock once the connection is ready.
        await eventually(lambda: clock.sleepers == 1, "the first heartbeat wait")
        await clock.advance(max(0.0, expected - 0.01))
        await asyncio.sleep(0.05)
        assert not got.is_set()
        await clock.advance(0.02)
        await within_patience(got.wait())
        await gw.close()
        await task


@pytest.mark.anyio
async def test_missing_heartbeat_ack_reconnects() -> None:
    clock = FakeClock()

    async def script(ws: ServerConnection, n: int) -> None:
        await ws.send(hello(interval_ms=1000))
        await server.recv(ws)
        await ws.send(ready())
        while True:
            await server.recv(ws)  # heartbeats, never acknowledged

    async with running(script) as server:
        gw = gateway(
            server,
            clock=clock,
            settings=SdkSettings(reconnect_backoff_base=0.01, reconnect_backoff_cap=0.01),
        )
        task = asyncio.ensure_future(collect(gw, lambda s: False))
        await eventually(
            lambda: server.connections >= 2, "the reconnect", step=lambda: clock.advance(0.5)
        )
        assert server.connections >= 2
        assert any(f["op"] == "heartbeat" for f in server.frames)
        await gw.close()
        await task


@pytest.mark.parametrize(
    "code",
    sorted(STOP_ON - {1000})
    + sorted(RESUME_ON)
    + sorted(IDENTIFY_ON)
    + [4001, 4002, 4003, 4005, 4999],
)
@pytest.mark.anyio
async def test_close_code_reactions(code: int) -> None:
    async def script(ws: ServerConnection, n: int) -> None:
        await ws.send(hello())
        await server.recv(ws)
        await ws.send(ready())
        if n == 1:
            await ws.close(code, "reason")
            return
        await ws.wait_closed()

    async with running(script) as server:
        gw = gateway(server)
        await collect(gw, lambda seen: server.connections >= 2 and len(server.frames) >= 2)
    if code in STOP_ON:
        assert server.connections == 1
        assert gw.close_reason is not None
        assert gw.close_reason.code == code
        assert not gw.close_reason.reconnects
        return
    assert server.connections >= 2
    second = server.frames[1]["op"]
    assert second == ("identify" if code in IDENTIFY_ON else "resume")


@pytest.mark.anyio
async def test_rate_limited_close_waits_retry_after() -> None:
    clock = FakeClock()

    async def script(ws: ServerConnection, n: int) -> None:
        await ws.send(hello())
        await server.recv(ws)
        if n == 1:
            await ws.close(4008, "retry_after=30")
            return
        await ws.send(ready())
        await ws.wait_closed()

    async with running(script) as server:
        gw = gateway(server, clock=clock)
        task = asyncio.ensure_future(collect(gw, lambda s: False))
        # The heartbeat of the first connection sleeps on the same clock, so wait for the sleep
        # whose deadline is the retry_after of the close (30), not for any sleeper.
        await eventually(lambda: 30.0 in clock.deadlines, "the wait of retry_after")
        await clock.advance(29)
        await asyncio.sleep(0.1)
        assert server.connections == 1
        await clock.advance(1.5)
        await eventually(lambda: server.connections == 2, "the second connection")
        await gw.close()
        await task


def test_reconnect_full_jitter() -> None:
    gw = Gateway(
        "ws://127.0.0.1:1/g",
        Credentials(Secret(TOKEN)),
        consumer="alpha",
        agent_id=AGENT,
        settings=SdkSettings(reconnect_backoff_base=1.0, reconnect_backoff_cap=8.0),
        rng=seeded(9),
    )
    for attempt in range(10):
        waits = [gw._backoff(attempt) for _ in range(50)]
        assert all(0 <= w <= min(8.0, 2**attempt) for w in waits)
        assert max(waits) > min(8.0, 2**attempt) / 4  # spread over the whole range


@pytest.mark.anyio
async def test_principal_mismatch_stops() -> None:
    async def script(ws: ServerConnection, n: int) -> None:
        await ws.send(hello())
        await server.recv(ws)
        await ws.send(ready(principal=UUID("0192f4a0-7b1c-7000-8a00-0000000000ff")))
        await ws.wait_closed()

    async with running(script) as server:
        gw = gateway(server)
        seen = await collect(gw, lambda s: False)
    assert server.connections == 1
    assert not any(f.op == "ready" for f in seen)
    assert gw.close_reason is not None
    assert gw.close_reason.reason == "principal_mismatch"


@pytest.mark.anyio
async def test_unknown_frame_tolerated() -> None:
    async def script(ws: ServerConnection, n: int) -> None:
        await ws.send(hello())
        await server.recv(ws)
        await ws.send(ready())
        await ws.send("not json")
        await ws.send(json.dumps({"op": "brand_new_op", "d": {"x": 1}}))
        await ws.send(
            json.dumps({"op": "dispatch", "d": {"id": SESSION, "type": "x.y", "seq": 11}})
        )
        await ws.wait_closed()

    async with running(script) as server:
        gw = gateway(server)
        seen = await collect(gw, lambda s: any(f.op == "dispatch" for f in s))
    assert [f.op for f in seen] == ["ready", "dispatch"]
