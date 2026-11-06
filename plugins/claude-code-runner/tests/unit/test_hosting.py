# SPDX-License-Identifier: Apache-2.0
"""The hosted set, its tokens and their refresh (docs/spec/26-builtin-agents.md 3.2, 3.3).

The set follows ``listHostedAgents`` at start, ``agent.hosting_changed`` and the timer of
``runner.channels_resync_interval``; a hosted token lives in memory only and is fetched again
before its ``expires_at``; a connection closed with ``4012`` drops the agent until the next list
names it again. The world is `hosting_world.HostingWorld`.
"""

from __future__ import annotations

import json
import logging
from collections.abc import AsyncIterator, Iterator
from datetime import UTC, datetime, timedelta
from pathlib import Path
from uuid import UUID

import pytest
from bothy_sdk.auth import Secret
from bothy_sdk.testing.faults import Fault
from hosting_world import HostedBackend, HostingWorld, hosting_world
from scenarios.conftest import LogLines

from claude_code_runner.hosting import (
    TOKEN_REFRESH_FRACTION,
    HostedToken,
    refresh_at,
)

pytestmark = pytest.mark.anyio


@pytest.fixture
async def world(tmp_path: Path) -> AsyncIterator[HostingWorld]:
    async with hosting_world(tmp_path / "w") as w:
        yield w


@pytest.fixture
def logs() -> Iterator[LogLines]:
    root = logging.getLogger()
    handler = LogLines()
    level = root.level
    root.addHandler(handler)
    root.setLevel(logging.DEBUG)
    websockets = logging.getLogger("websockets")
    ws_level = websockets.level
    websockets.setLevel(logging.WARNING)
    try:
        yield handler
    finally:
        root.removeHandler(handler)
        root.setLevel(level)
        websockets.setLevel(ws_level)


def _events(logs: LogLines, event: str) -> list[dict[str, str]]:
    out = []
    for record in logs.records:
        try:
            line = json.loads(record.getMessage())
        except ValueError:
            continue
        if isinstance(line, dict) and line.get("event") == event:
            out.append(line)
    return out


def _connected(backend: HostedBackend) -> bool:
    """True once the hosted agent's own connection sent a heartbeat to its server."""
    return any(b.get("details", {}).get("adapter") for b in backend.fake.heartbeats)


def test_refresh_point_is_a_fraction_of_the_lifetime_before_the_expiry() -> None:
    created = datetime(2026, 11, 6, 10, 0, tzinfo=UTC)
    expires = created + timedelta(hours=24)
    point = refresh_at(created, expires)
    assert point == expires - timedelta(hours=24) * TOKEN_REFRESH_FRACTION
    assert created < point < expires


def test_a_hosted_token_never_shows_its_value() -> None:
    token = HostedToken(
        Secret("bothy_agt_canary_value"),
        expires_at=datetime(2026, 11, 7, tzinfo=UTC),
        refresh_at=datetime(2026, 11, 6, 18, tzinfo=UTC),
    )
    for text in (repr(token), str(token), f"{token}"):
        assert "canary" not in text


async def test_off_reads_no_list_and_opens_no_connection(world: HostingWorld) -> None:
    # 26 3.2 item 4: without runner.hosted_agents nothing of the hosting runs.
    backend = await world.server.add_backend()
    world.server.host(backend)
    await world.start(world.config(hosted_agents=False))
    await world.fake.pass_time(5)
    beat = world.fake.heartbeats[-1]
    assert "hosts_agents" not in beat["details"]["capabilities"]
    assert world.server.list_calls == 0
    assert world.server.issue_calls == []
    assert backend.fake.heartbeats == []
    assert not (world.state / "hosted").exists()


async def test_the_set_follows_the_list_at_start(world: HostingWorld, logs: LogLines) -> None:
    backend = await world.server.add_backend()
    world.server.host(backend, announce=False)
    await world.start()
    await world.until(lambda: _connected(backend))
    assert world.server.issue_calls == [backend.agent_id]
    joined = _events(logs, "hosted_agent_joined")
    assert [line["agent_id"] for line in joined] == [str(backend.agent_id)]


async def test_the_set_follows_the_event(world: HostingWorld, logs: LogLines) -> None:
    first = await world.server.add_backend()
    world.server.host(first, announce=False)
    await world.start()
    await world.until(lambda: _connected(first))
    second = await world.server.add_backend()
    world.server.host(second)  # agent.hosting_changed
    await world.until(lambda: _connected(second))
    world.server.unhost(first)  # its host changed: the runner stops serving it
    await world.until(lambda: bool(_events(logs, "hosted_agent_left")))
    [left] = _events(logs, "hosted_agent_left")
    assert left["agent_id"] == str(first.agent_id)
    await world.until(lambda: not first.fake.hub.connections)
    assert second.fake.hub.connections


async def test_the_set_follows_the_timer(world: HostingWorld) -> None:
    # 26 3.2 item 1: runner.channels_resync_interval is the fallback for a missed event.
    await world.start(world.config(channels_resync_interval="2s"))
    calls = world.server.list_calls
    backend = await world.server.add_backend()
    world.server.host(backend, announce=False)
    await world.until(lambda: world.server.list_calls > calls)
    await world.until(lambda: _connected(backend))


async def test_the_token_is_refreshed_before_its_expiry(
    world: HostingWorld, logs: LogLines
) -> None:
    world.server.ttl = 100.0
    backend = await world.server.add_backend()
    world.server.host(backend, announce=False)
    await world.start()
    await world.until(lambda: _connected(backend))
    assert len(backend.tokens) == 1
    # Before the refresh point nothing is fetched; after it a new token is, before the expiry.
    await world.fake.pass_time(100 * (1 - TOKEN_REFRESH_FRACTION) - 10)
    assert len(backend.tokens) == 1
    await world.fake.pass_time(15)
    await world.until(lambda: len(backend.tokens) == 2)
    count = len(backend.fake.heartbeats)
    await world.until(lambda: len(backend.fake.heartbeats) > count)  # connected with the new one
    refreshed = _events(logs, "hosted_token_refreshed")
    assert [line["agent_id"] for line in refreshed] == [str(backend.agent_id)] * 2
    assert not _events(logs, "hosted_agent_left")


async def test_4012_drops_the_agent_until_the_next_list(
    world: HostingWorld, logs: LogLines
) -> None:
    backend = await world.server.add_backend()
    world.server.host(backend, announce=False)
    await world.start()
    await world.until(lambda: _connected(backend))
    await world.server.revoke(backend)
    await world.until(lambda: bool(_events(logs, "hosted_agent_left")))
    [left] = _events(logs, "hosted_agent_left")
    assert left == {
        "event": "hosted_agent_left",
        "agent_id": str(backend.agent_id),
        "reason": "revoked",
    }
    await world.fake.pass_time(30)
    assert world.server.issue_calls == [backend.agent_id]  # no new token on its own
    world.server.announce(backend.agent_id)  # the next list still names it
    await world.until(lambda: len(world.server.issue_calls) == 2)
    count = len(backend.fake.heartbeats)
    await world.until(lambda: len(backend.fake.heartbeats) > count)


async def test_a_failed_issue_is_logged_with_its_code_and_retried(
    world: HostingWorld, logs: LogLines
) -> None:
    backend = await world.server.add_backend()
    world.server.issue_problem[backend.agent_id] = "conflict"
    world.server.host(backend, announce=False)
    await world.start()
    await world.until(lambda: len(world.server.issue_calls) >= 2)
    failed = _events(logs, "hosted_token_issue_failed")
    assert failed[0] == {
        "event": "hosted_token_issue_failed",
        "agent_id": str(backend.agent_id),
        "code": "conflict",
    }
    del world.server.issue_problem[backend.agent_id]
    await world.until(lambda: _connected(backend))


async def test_no_token_in_any_file_or_log_line(world: HostingWorld, logs: LogLines) -> None:
    # BA-T09, 26 3.3 item 2: a hosted token is never in a file under the runner's folders, a
    # log line or a turn's environment, also after a refresh and runs of the hosted agents.
    world.server.ttl = 100.0
    backends = [await world.server.add_backend() for _ in range(2)]
    for backend in backends:
        world.server.host(backend, announce=False)
    await world.start()
    for backend in backends:
        await world.until(lambda b=backend: _connected(b))
        run = await backend.fake.trigger("Hello")
        await world.until(lambda b=backend, r=run: b.fake.run_state(r.run_id).state == "failed")
    run = await world.fake.trigger("Hello", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    await world.fake.pass_time(90)
    await world.until(lambda: all(len(b.tokens) == 2 for b in backends))
    secrets = world.issued_secrets()
    assert len(secrets) == 4
    for path, content in world.all_files():
        for secret in secrets:
            assert secret.encode() not in content, path
    for secret in secrets:
        assert secret not in logs.text
    for turn in world.turns():
        assert not any(secret in value for value in turn["env"].values() for secret in secrets)
    hosted = {p.name for p in (world.state / "hosted").iterdir()}
    assert hosted == {str(b.agent_id) for b in backends}
    for name in hosted:
        UUID(name)
        assert (world.state / "hosted" / name).stat().st_mode & 0o777 == 0o700


async def test_a_failed_connection_is_logged_and_tried_again(
    world: HostingWorld, logs: LogLines
) -> None:
    # A refused discovery of the hosted agent's server ends the attempt; the runner logs the
    # code, keeps the token and connects again with the SDK's backoff.
    backend = await world.server.add_backend()
    backend.fake.inject("getDiscovery", Fault.problem("forbidden"), times=2)
    world.server.host(backend, announce=False)
    await world.start()
    await world.until(lambda: _connected(backend))
    failed = _events(logs, "hosted_connection_failed")
    assert [line["code"] for line in failed] == ["forbidden", "forbidden"]
    assert len(backend.tokens) == 1
