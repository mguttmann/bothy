# SPDX-License-Identifier: Apache-2.0
"""Row "Crash of the runner" of 09 18.9 and its variants (design note 12, AG-10 runner side).

The first runner runs as a process of its own and is killed with SIGKILL; its turn keeps
running in its own process group and session. A second runner on the same ``runner.state_dir``
takes the turn over or ends it, and never starts a process or ``run.start`` for its run. The
FakeServer clock follows the real time here, because the first runner uses the system clock.
"""

from __future__ import annotations

import json
import os
import signal
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.testing._patience import PATIENCE

from .conftest import LogLines, World
from .test_baseline import _amounts, _bind, _run, _usage

pytestmark = pytest.mark.anyio


@pytest.fixture
def world_real_time() -> bool:
    return True


@pytest.fixture
def world_timings() -> dict[str, float]:
    # The server never ends a run on its own timers inside a scenario.
    return {"agent.cancel_grace": 100 * PATIENCE, "agent.invocation_token_ttl": 100 * PATIENCE}


def record_path(world: World, run_id: UUID) -> Path:
    return world.state / "turns" / f"{run_id}.json"


def alive(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    return True


def finishes(world: World, run_id: UUID) -> list[dict[str, Any]]:
    return [r.json for r in world.requests("finishRun") if str(run_id) in r.path]


async def crashed_turn(world: World, **scenario: Any) -> tuple[UUID, int, Path]:
    """A turn paused on its gate whose runner was killed; returns run, turn PID and gate."""
    gate = world.gate("turn_gate")
    world.scenario(fixture="happy", turn_gate=str(gate), **scenario)
    first = world.spawn()
    # The consumer's cursor starts at its first connection: trigger only after it is ready.
    await world.until(
        lambda: any(h.get("details", {}).get("adapter") for h in world.fake.heartbeats)
    )
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    await world.until(lambda: record_path(world, run.run_id).exists())
    await world.until(lambda: "pid" in json.loads(record_path(world, run.run_id).read_text()))
    pid = world.turns()[0]["pid"]
    first.send_signal(signal.SIGKILL)
    first.wait(PATIENCE)
    assert alive(pid)
    return run.run_id, pid, gate


def beats_after(world: World, count: int) -> list[dict[str, Any]]:
    return world.fake.heartbeats[count:]


async def test_runner_crash_turn_survives_and_is_adopted(world: World) -> None:
    run_id, pid, gate = await crashed_turn(world)
    seen = len(world.fake.heartbeats)
    await world.start()
    await world.until(
        lambda: any(
            str(run_id) in h.get("details", {}).get("active_runs", [])
            for h in beats_after(world, seen)
        )
    )
    assert world.turns()[0]["pid"] == pid
    assert alive(pid)
    world.release(gate)
    assert await world.ended(run_id) == "completed"
    assert len(world.turns()) == 1
    assert len(world.requests("startRun")) == 1
    assert len(finishes(world, run_id)) == 1
    assert [m["content_md"] for m in world.fake.messages] == [
        "The form validates the email twice; I removed the second check."
    ]
    steps = world.fake.run(run_id)["steps"].values()
    assert any(s["kind"] == "tool_call" for s in steps)
    await world.until(lambda: not record_path(world, run_id).exists())


async def test_crash_run_ended_meanwhile_stops_turn(world: World) -> None:
    run_id, pid, _ = await crashed_turn(world)
    world.fake.set_run_state(run_id, "failed", "stale")
    await world.start()
    await world.until(lambda: not alive(pid))
    await world.until(lambda: not record_path(world, run_id).exists())
    assert finishes(world, run_id) == []
    ends = [c for c in world.calls("turn_end") if c["pid"] == pid]
    assert [e.get("signal") for e in ends] == ["SIGINT"]
    assert len(world.turns()) == 1


async def test_crash_turn_finished_while_down_finishes_from_log(world: World) -> None:
    run_id, pid, gate = await crashed_turn(world)
    world.release(gate)
    await world.until(lambda: not alive(pid))
    await world.start()
    assert await world.ended(run_id) == "completed"
    assert len(finishes(world, run_id)) == 1
    assert len(world.fake.messages) == 1
    assert len(world.turns()) == 1
    assert len(world.requests("startRun")) == 1


async def test_crash_turn_died_without_result_left_to_server(world: World) -> None:
    run_id, pid, gate = await crashed_turn(world, no_result=True, exit_without_eof=True)
    world.release(gate)
    await world.until(lambda: not alive(pid))
    seen = len(world.fake.heartbeats)
    await world.start()
    await world.until(lambda: not record_path(world, run_id).exists())
    await world.until(lambda: len(beats_after(world, seen)) >= 1)
    assert all(
        str(run_id) not in h.get("details", {}).get("active_runs", [])
        for h in beats_after(world, seen)
    )
    assert finishes(world, run_id) == []
    assert world.fake.run_state(run_id).state == "running"  # the server ends it interrupted
    assert len(world.turns()) == 1


async def test_crash_identity_mismatch_never_signals(world: World, logs: LogLines) -> None:
    run_id, pid, gate = await crashed_turn(world)
    path = record_path(world, run_id)
    record = json.loads(path.read_text())
    record["identity"]["lstart"] = "Thu Jan  1 00:00:00 1970"
    path.write_text(json.dumps(record))
    world.fake.set_run_state(run_id, "failed", "stale")  # a live match would be stopped
    await world.start()
    await world.until(lambda: "adoption_lock_busy" in logs.text)
    assert alive(pid)
    assert not [c for c in world.calls("turn_end") if c["pid"] == pid]
    assert finishes(world, run_id) == []
    assert path.exists()  # kept for the next start
    world.release(gate)  # the turn ends on its own; the scenario leaves no process behind
    await world.until(lambda: not alive(pid))
    ends = [c for c in world.calls("turn_end") if c["pid"] == pid]
    assert [e.get("signal") for e in ends] == [None]


async def test_crash_orderly_stop_keeps_adopted_record(world: World) -> None:
    run_id, pid, gate = await crashed_turn(world)
    second = await world.start()
    await world.until(lambda: run_id in second.handler.adopted)
    await world.stop(second)  # a deploy: the adopted turn keeps running, its record stays
    assert alive(pid)
    assert record_path(world, run_id).exists()
    world.release(gate)
    await world.until(lambda: not alive(pid))
    await world.start()
    assert await world.ended(run_id) == "completed"
    assert len(finishes(world, run_id)) == 1
    assert len(world.turns()) == 1


def child_of(world: World) -> int:
    return int(world.calls("child")[0]["child_pid"])


async def test_crash_adopted_turn_leaves_no_job_of_its_group(world: World) -> None:
    run_id, pid, gate = await crashed_turn(world, spawn_child=True, child_ignores_sigint=True)
    second = await world.start()
    await world.until(lambda: run_id in second.handler.adopted)
    world.release(gate)  # the leader answers and exits; its background job lives on
    assert await world.ended(run_id) == "completed"
    await world.until(lambda: not alive(child_of(world)))
    assert not alive(pid)


async def test_crash_finish_from_log_does_not_wait_for_leftover_jobs(world: World) -> None:
    run_id, pid, gate = await crashed_turn(world, spawn_child=True, child_ignores_sigint=True)
    world.release(gate)
    await world.until(lambda: not alive(pid))
    child = child_of(world)
    await world.start()
    assert await world.ended(run_id) == "completed"
    # The group never matched its record after the restart: the runner sends it no signal.
    assert alive(child)
    os.kill(child, signal.SIGKILL)  # this test's own cleanup of the leftover job
    await world.until(lambda: not alive(child))


async def _crashed_bound_turn(world: World) -> tuple[UUID, int, Path, UUID]:
    """The first run after an import binding, paused on its gate, its runner killed."""
    thread = _bind(world, 15000)
    gate = world.gate("turn_gate")
    world.scenario(
        fixture="resume", turn_gate=str(gate), result_override=_usage({"model-large": (5000, 0.02)})
    )
    first = world.spawn()
    await world.until(
        lambda: any(h.get("details", {}).get("adapter") for h in world.fake.heartbeats)
    )
    run = await world.fake.trigger("Go on", thread_id=thread, session_mode="bind")
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    await world.until(lambda: record_path(world, run.run_id).exists())
    await world.until(lambda: "pid" in json.loads(record_path(world, run.run_id).read_text()))
    pid = world.turns()[0]["pid"]
    first.send_signal(signal.SIGKILL)
    first.wait(PATIENCE)
    return run.run_id, pid, gate, thread


@pytest.mark.parametrize("live", [True, False])
async def test_adopted_first_run_after_an_import_keeps_the_baseline(
    world: World, live: bool
) -> None:
    # The baseline travels in the turn record: an adopted run and the next one book together
    # only the part above it (09 18.7, RK-75).
    run_id, pid, gate, thread = await _crashed_bound_turn(world)
    if live:
        second = await world.start()
        await world.until(lambda: run_id in second.handler.adopted)
        world.release(gate)
    else:
        world.release(gate)
        await world.until(lambda: not alive(pid))
        await world.start()
    assert await world.ended(run_id) == "completed"
    adopted = _amounts(finishes(world, run_id)[-1]["costs"])
    assert adopted == {"model-large": 5000}
    following = _amounts(await _run(world, thread, {"model-large": (6000, 0.026)}, "resume"))
    assert following == {"model-large": 6000}
