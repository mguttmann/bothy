# SPDX-License-Identifier: Apache-2.0
"""One runner process hosting two built-in agents next to its own (BA-T09, 26 3.2, 5.6 item 4).

Two hosted agents and the runner's own agent: separate connections, cursors and state folders;
the slots of ``runner.max_parallel_runs`` are shared, so a hosted agent's run waits with
``slots_full`` while turns of another agent hold every slot; every run of a hosted agent ends
``failed`` with ``adapter_error`` and the cause ``definition_missing`` (until the turns of A1-05);
the runner's heartbeat carries ``hosts_agents`` with its ceiling, the hosted heartbeats do not.
"""

from __future__ import annotations

from collections.abc import AsyncIterator
from pathlib import Path
from typing import Any

import pytest
from hosting_world import HostedBackend, HostingWorld, hosting_world

from claude_code_runner.causes import cause

pytestmark = pytest.mark.anyio


@pytest.fixture
async def world(tmp_path: Path) -> AsyncIterator[HostingWorld]:
    async with hosting_world(tmp_path / "w") as w:
        yield w


def _beats(backend: HostedBackend) -> list[dict[str, Any]]:
    return [b for b in backend.fake.heartbeats if b.get("details", {}).get("adapter")]


async def _hosted(world: HostingWorld, *classes: str) -> list[HostedBackend]:
    backends = []
    for tool_class in classes:
        backend = await world.server.add_backend(tool_class)
        world.server.host(backend, announce=False)
        backends.append(backend)
    return backends


async def test_two_hosted_agents_have_their_own_connection_cursor_and_state(
    world: HostingWorld,
) -> None:
    a, b = await _hosted(world, "scoped", "scoped")
    await world.start()
    for backend in (a, b):
        await world.until(lambda x=backend: bool(_beats(x)))
    run_a = await a.fake.trigger("Hello")
    run_b1 = await b.fake.trigger("Hello")
    run_b2 = await b.fake.trigger("Again")
    for backend, run in ((a, run_a), (b, run_b1), (b, run_b2)):
        await world.until(lambda x=backend, r=run: x.fake.run_state(r.run_id).state == "failed")
    # Each agent acknowledged its own events on its own consumer of its own principal.
    for backend in (a, b):
        await world.until(
            lambda x=backend: (
                x.fake.consumer_state("default")[0] == x.fake.consumer_state("default")[1]
            )
        )
    assert world.fake.hub.cursors.keys() == {"default"}  # the runner's own, on its server
    for backend in (a, b):
        folder = world.state / "hosted" / str(backend.agent_id)
        assert folder.stat().st_mode & 0o777 == 0o700
        assert (folder / "inbox.sqlite3").exists()
    assert not (world.state / "hosted" / str(world.fake.agent_id)).exists()
    assert (world.state / "inbox.sqlite3").exists()  # the runner's own state stays where it was


async def test_hosted_runs_end_with_definition_missing(world: HostingWorld) -> None:
    [a] = await _hosted(world, "scoped")
    await world.start()
    await world.until(lambda: bool(_beats(a)))
    run = await a.fake.trigger("Hello")
    await world.until(lambda: a.fake.run_state(run.run_id).state == "failed")
    view = a.fake.run_state(run.run_id)
    assert view.state_reason == "adapter_error"
    assert view.starts == 0  # no run.start, no turn
    [finish] = [r.json for r in a.fake.requests if r.operation_id == "finishRun"]
    assert finish["summary"] == cause("definition_missing")
    assert world.turns() == []


async def test_slots_are_shared_with_the_runner_s_own_turns(world: HostingWorld) -> None:
    a, b = await _hosted(world, "scoped", "chat")
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate))
    await world.start(world.config(max_parallel_runs=2))
    for backend in (a, b):
        await world.until(lambda x=backend: bool(_beats(x)))
    own = [await world.fake.trigger(f"Run {i}", channel_id=world.forum_a) for i in range(2)]
    await world.until(lambda: len(world.calls("gate_wait")) == 2)
    # Two turns of the runner's own agent hold both slots: a run of each hosted agent waits.
    run_a = await a.fake.trigger("Hello")
    run_b = await b.fake.trigger("Hello")
    for backend, run in ((a, run_a), (b, run_b)):
        want = {"run_id": str(run.run_id), "reason": "slots_full"}
        await world.until(
            lambda x=backend, w=want: any(w in (h.get("waiting_runs") or []) for h in _beats(x))
        )
        beat = next(h for h in _beats(backend) if want in (h.get("waiting_runs") or []))
        assert beat["run_slots"] == {"used": 2, "total": 2}
        assert beat["waiting_runs"] == [want]  # only the agent's own waiting runs
        assert backend.fake.run_state(run.run_id).state == "queued"
    world.release(gate)
    for run in own:
        assert await world.ended(run.run_id) == "completed"
    for backend, run in ((a, run_a), (b, run_b)):
        await world.until(lambda x=backend, r=run: x.fake.run_state(r.run_id).state == "failed")
        assert backend.fake.run_state(run.run_id).state_reason == "adapter_error"


async def test_hosts_agents_only_in_the_runner_s_heartbeat(world: HostingWorld) -> None:
    host_class, chat_class = await _hosted(world, "host", "chat")
    await world.start(world.config(hosted_max_tool_class="scoped"))
    for backend in (host_class, chat_class):
        await world.until(lambda x=backend: bool(_beats(x)))
    runner_beat = world.fake.heartbeats[-1]
    assert runner_beat["details"]["capabilities"]["hosts_agents"] == "scoped"
    assert runner_beat["reported_tool_class"] == "scoped"
    for backend, loaded in ((host_class, "scoped"), (chat_class, "chat")):
        beat = _beats(backend)[-1]
        caps = beat["details"]["capabilities"]
        assert "hosts_agents" not in caps
        assert caps == {
            k: v for k, v in runner_beat["details"]["capabilities"].items() if k != "hosts_agents"
        }
        # The lower of the agent's class and the ceiling (26 3.1 item 3).
        assert beat["reported_tool_class"] == loaded
        assert beat["billing_modes_supported"] == runner_beat["billing_modes_supported"]
        assert "project_readiness" not in beat
        assert "project_folders" not in beat


async def test_a_class_change_is_reported_after_the_next_list(world: HostingWorld) -> None:
    [a] = await _hosted(world, "chat")
    await world.start(world.config(hosted_max_tool_class="host", channels_resync_interval="2s"))
    await world.until(lambda: bool(_beats(a)))
    assert _beats(a)[-1]["reported_tool_class"] == "chat"
    world.server.set_tool_class(a, "host")
    await world.until(lambda: _beats(a)[-1]["reported_tool_class"] == "host")
