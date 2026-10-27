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
from bothy_sdk.testing.faults import Fault

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


def _write_exchange(directory: Path, fake: FakeServer, *entries: DemoEntry) -> None:
    doc: dict[str, Any] = {
        "instance_id": str(fake.instance_id),
        "values": VALUES,
        "agents": [
            {
                "handle": e.handle,
                "role": e.role,
                "provider": e.provider,
                "model": e.model,
                "currency": e.currency,
                "agent_id": e.agent_id,
                "token": e.token,
                "dpop_private_jwk": dict(e.dpop_private_jwk),
            }
            for e in entries
        ],
    }
    (directory / "agents.json").write_text(json.dumps(doc), encoding="utf-8")


async def test_serve_reads_the_exchange(fake: FakeServer, tmp_path: Path) -> None:
    _write_exchange(tmp_path, fake, _entry(fake, "worker"))
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


async def test_transient_errors_do_not_stop_the_agents(fake: FakeServer, tmp_path: Path) -> None:
    # A short outage of the server: receive and heartbeat answer 503, 429 or no answer at all.
    # One attempt per request, so every fault reaches the demo agent, not the SDK's own retries.
    _write_exchange(tmp_path, fake, _entry(fake, "worker"))
    stop = asyncio.Event()
    task = asyncio.ensure_future(
        serve(
            tmp_path,
            fake.base_url,
            stop,
            settings={"sdk.retry_max_attempts": "1"},
            sleep=_fast,
            clock=fake.clock.now,
        )
    )
    await eventually(lambda: task.done() or "default" in fake.hub.cursors, "the connected agent")
    fake.inject("pollAgentEvents", Fault.problem("unavailable"), times=3)
    fake.inject("pollAgentEvents", Fault.rate_limited(1))
    fake.inject("pollAgentEvents", Fault.disconnect())
    fake.inject("sendHeartbeat", Fault.problem("unavailable"), times=2)
    trig = await fake.trigger("Compare the last two load tests")
    await eventually(
        lambda: task.done() or fake.run(trig.run_id)["state"] == "completed", "the finished run"
    )
    assert not task.done(), f"a transient error ended the demo agents: {task!r}"
    assert not fake._faults["pollAgentEvents"], "a fault of receive was never sent"
    stop.set()
    await within_patience(task)


@pytest.mark.parametrize("status", [500, 502, 503, 504])
async def test_proxy_errors_of_receive_do_not_stop_the_agents(
    fake: FakeServer, tmp_path: Path, status: int
) -> None:
    # The API restarts behind its TLS proxy: receive and heartbeat get a 5xx whose body is no
    # problem, which the SDK raises as ServerError. The agent waits it out and goes on.
    _write_exchange(tmp_path, fake, _entry(fake, "worker"))
    stop = asyncio.Event()
    task = asyncio.ensure_future(
        serve(
            tmp_path,
            fake.base_url,
            stop,
            settings={"sdk.retry_max_attempts": "1"},
            sleep=_fast,
            clock=fake.clock.now,
        )
    )
    await eventually(lambda: task.done() or "default" in fake.hub.cursors, "the connected agent")
    fake.inject("pollAgentEvents", Fault.proxy_error(status), times=3)
    fake.inject("sendHeartbeat", Fault.proxy_error(status), times=2)
    trig = await fake.trigger("Compare the last two load tests")
    await eventually(
        lambda: task.done() or fake.run(trig.run_id)["state"] == "completed", "the finished run"
    )
    assert not task.done(), f"a {status} of the proxy ended the demo agents: {task!r}"
    assert not fake._faults["pollAgentEvents"], "a fault of receive was never sent"
    stop.set()
    await within_patience(task)


async def test_a_refused_receive_without_a_problem_body_stops_the_agent(
    fake: FakeServer,
) -> None:
    # Only a 4xx other than 429 is permanent, also when its body is no problem.
    agent, stop = _agent(fake, "worker"), asyncio.Event()
    task = await _serving(agent, stop)
    fake.inject("pollAgentEvents", Fault.proxy_error(403))
    await eventually(task.done, "the stopped agent")
    with pytest.raises(DemoError, match="receive answered 403"):
        task.result()
    await agent.close()


async def test_one_failing_agent_does_not_end_the_others(fake: FakeServer, tmp_path: Path) -> None:
    # The first agent's token is refused (401, not transient): only that agent stops.
    refused = DemoEntry(
        handle="copy-helper",
        role="worker",
        provider="example-provider",
        model="model-small",
        currency="USD",
        agent_id=str(fake.other_agent_id),
        token=str(UUID(int=7)),
        dpop_private_jwk=private_jwk(fake),
    )
    _write_exchange(tmp_path, fake, refused, _entry(fake, "worker"))
    stop = asyncio.Event()
    task = asyncio.ensure_future(
        serve(tmp_path, fake.base_url, stop, sleep=_fast, clock=fake.clock.now)
    )
    await eventually(lambda: task.done() or "default" in fake.hub.cursors, "the connected agent")
    trig = await fake.trigger("Compare the last two load tests")
    await eventually(
        lambda: task.done() or fake.run(trig.run_id)["state"] == "completed", "the finished run"
    )
    assert not task.done(), f"one failing agent ended the others: {task!r}"
    stop.set()
    await within_patience(task)


async def test_an_agent_that_stops_ends_its_scripts(fake: FakeServer) -> None:
    # A refused receive (403, not transient) ends serve, and with it the watcher it started.
    agent, stop = _agent(fake, "watcher"), asyncio.Event()
    task = await _serving(agent, stop)
    trig = await fake.trigger("Watch the nightly build")
    await eventually(lambda: len(fake.run(trig.run_id)["steps"]) >= 2, "two build stages")
    fake.inject("pollAgentEvents", Fault.problem("forbidden"))
    await eventually(task.done, "the stopped agent")
    with pytest.raises(DemoError, match="receive answered 403"):
        task.result()
    assert not agent._tasks, "the watcher outlived its agent"
    await agent.close()
