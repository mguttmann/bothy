# SPDX-License-Identifier: Apache-2.0
"""The demo agent of ``bothy setup demo`` against the FakeServer (brief M7-27, 16 16.4)."""

from __future__ import annotations

import asyncio
import json
from collections.abc import AsyncIterator
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest
from _fake_agent_driver import private_jwk

from bothy_sdk.testing._patience import eventually, within_patience
from bothy_sdk.testing.demo_agent import (
    DemoAgent,
    DemoEntry,
    DemoError,
    DemoValues,
    load_exchange,
    serve,
)
from bothy_sdk.testing.fakeserver import FakeServer

pytestmark = pytest.mark.anyio

VALUES = {
    "worker_steps": 2,
    "step_interval_ms": 1,
    "watcher_interval_ms": 1,
    "heartbeat_interval_ms": 1,
    "tokens_in": 4200,
    "tokens_out": 900,
}


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


@pytest.fixture
async def fake() -> AsyncIterator[FakeServer]:
    async with FakeServer(costs=True) as server:
        yield server


async def _fast(_seconds: float) -> None:
    """The waits of the tests: yield to the event loop only."""
    await asyncio.sleep(0)


def _entry(fake: FakeServer, role: str) -> DemoEntry:
    return DemoEntry(
        handle="triage-bot",
        role=role,
        provider="example-provider",
        model="model-small",
        currency="USD",
        agent_id=str(fake.agent_id),
        token=fake.credentials.api_token.reveal(),
        dpop_private_jwk=private_jwk(fake),
    )


def _agent(fake: FakeServer, role: str) -> DemoAgent:
    return DemoAgent(
        _entry(fake, role),
        DemoValues.from_mapping(VALUES),
        instance_id=str(fake.instance_id),
        base_url=fake.base_url,
        settings={"sdk.retry_backoff_base": "10ms", "sdk.retry_backoff_cap": "50ms"},
        sleep=_fast,
        clock=fake.clock.now,
    )


async def _serving(agent: DemoAgent, stop: asyncio.Event) -> asyncio.Task[None]:
    await agent.connect()
    return asyncio.ensure_future(agent.serve(stop))


async def _stop(agent: DemoAgent, stop: asyncio.Event, task: asyncio.Task[None]) -> None:
    stop.set()
    await within_patience(task)
    await agent.close()


async def test_worker_finishes_with_costs(fake: FakeServer) -> None:
    agent, stop = _agent(fake, "worker"), asyncio.Event()
    task = await _serving(agent, stop)
    trig = await fake.trigger("Please summarize the open requests")
    await eventually(lambda: fake.run(trig.run_id)["state"] == "completed", "the finished run")
    await _stop(agent, stop, task)
    run = fake.run(trig.run_id)
    assert len(run["steps"]) == VALUES["worker_steps"]
    assert all(s["status"] == "completed" for s in run["steps"].values())
    booked = fake.world.runs[trig.run_id].costs
    assert booked == [
        {
            "provider": "example-provider",
            "model": "model-small",
            "tokens_in": 4200,
            "tokens_out": 900,
            "currency": "USD",
        }
    ]
    replies = [m for m in fake.messages if str(m.get("run_id")) == str(trig.run_id)]
    assert replies, "the worker posted no reply"


async def test_reviewer_waits_for_the_click(fake: FakeServer) -> None:
    agent, stop = _agent(fake, "reviewer"), asyncio.Event()
    task = await _serving(agent, stop)
    trig = await fake.trigger("Prepare the release notes")
    await eventually(lambda: fake.run(trig.run_id)["state"] == "waiting_input", "the waiting run")
    assert await fake.click(trig.run_id, "approval.once") is not None
    await eventually(lambda: fake.run(trig.run_id)["state"] == "completed", "the approved run")
    await _stop(agent, stop, task)
    assert fake.world.runs[trig.run_id].costs


async def test_watcher_keeps_the_run_open(fake: FakeServer) -> None:
    agent, stop = _agent(fake, "watcher"), asyncio.Event()
    task = await _serving(agent, stop)
    trig = await fake.trigger("Watch the nightly build")
    await eventually(lambda: len(fake.run(trig.run_id)["steps"]) >= 2, "two build stages")
    await _stop(agent, stop, task)
    run = fake.run(trig.run_id)
    assert run["state"] == "running"
    assert run["status_line"].startswith("Watching build stage")


async def test_serve_reads_the_exchange(fake: FakeServer, tmp_path: Path) -> None:
    entry = _entry(fake, "worker")
    doc: dict[str, Any] = {
        "instance_id": str(fake.instance_id),
        "values": VALUES,
        "agents": [
            {
                "handle": entry.handle,
                "role": entry.role,
                "provider": entry.provider,
                "model": entry.model,
                "currency": entry.currency,
                "agent_id": entry.agent_id,
                "token": entry.token,
                "dpop_private_jwk": dict(entry.dpop_private_jwk),
            }
        ],
    }
    (tmp_path / "agents.json").write_text(json.dumps(doc), encoding="utf-8")
    stop = asyncio.Event()
    task = asyncio.ensure_future(
        serve(tmp_path, fake.base_url, stop, sleep=_fast, clock=fake.clock.now)
    )
    # The consumer of the agent exists from its first poll on; a trigger before it is not its own.
    await eventually(lambda: "default" in fake.hub.cursors, "the connected agent")
    trig = await fake.trigger("Compare the last two load tests")
    await eventually(lambda: fake.run(trig.run_id)["state"] == "completed", "the finished run")
    stop.set()
    await within_patience(task)


def test_load_exchange_and_repr(tmp_path: Path) -> None:
    doc = {
        "instance_id": str(UUID(int=1)),
        "values": VALUES,
        "agents": [
            {
                "handle": "copy-helper",
                "role": "worker",
                "provider": "example-provider",
                "model": "model-small",
                "currency": "USD",
                "agent_id": str(UUID(int=2)),
                "token": "example-token-value",
                "dpop_private_jwk": {"kty": "EC"},
            }
        ],
    }
    (tmp_path / "agents.json").write_text(json.dumps(doc), encoding="utf-8")
    instance, values, entries = load_exchange(tmp_path)
    assert instance == str(UUID(int=1))
    assert values.step_interval == 0.001
    assert values.heartbeat_interval == 0.001
    assert [e.handle for e in entries] == ["copy-helper"]
    assert "example-token-value" not in repr(entries[0])
    doc["agents"][0]["role"] = "boss"
    (tmp_path / "agents.json").write_text(json.dumps(doc), encoding="utf-8")
    with pytest.raises(DemoError, match="unknown role"):
        load_exchange(tmp_path)


async def test_failed_op_names_op_not_token(fake: FakeServer) -> None:
    agent = _agent(fake, "worker")
    await agent.connect()
    with pytest.raises(DemoError) as err:
        await agent.op("finish", run=str(UUID(int=9)))
    assert "finish" in str(err.value)
    assert fake.credentials.api_token.reveal() not in str(err.value)
    await agent.close()
