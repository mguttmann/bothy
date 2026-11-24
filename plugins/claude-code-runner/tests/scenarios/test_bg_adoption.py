# SPDX-License-Identifier: Apache-2.0
"""A background turn that outlived its runner (09 18.9 "Background mode").

The first runner runs as a process of its own and is killed with SIGKILL while its background
session works; the session belongs to the CLI's supervisor and runs on. A second runner on the
same ``runner.state_dir`` finds it by the turn record and the session list and adopts it with
every right: the answer is posted once, a follow-up after the adoption is absorbed, and a stop
stops it. The FakeServer clock follows the real time, because the first runner uses the system
clock.
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

from .conftest import BACKGROUND, LogLines, World
from .test_bg_sessions import events, finishes, followup_states, nothing_left, players, texts

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


def record(world: World, run_id: UUID) -> dict[str, Any]:
    try:
        loaded: dict[str, Any] = json.loads(record_path(world, run_id).read_text())
    except (OSError, ValueError):
        return {}
    return loaded


def alive(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    return True


def active(world: World, run_id: UUID, seen: int) -> bool:
    return any(
        str(run_id) in h.get("details", {}).get("active_runs", [])
        for h in world.fake.heartbeats[seen:]
    )


async def restart(world: World) -> None:
    """Start the next runner in this process and wait until it serves.

    Its first heartbeat goes out after the adoption of the turns the killed runner left, so
    a scenario that ends right after the adoption never stops a runner that has not started.
    """
    seen = len(world.fake.heartbeats)
    await world.start(world.config(**BACKGROUND))
    await world.until(lambda: len(world.fake.heartbeats) > seen)


async def crashed(world: World, **scenario: Any) -> tuple[UUID, Path]:
    """A background turn paused in a tool call whose runner was killed; the run and its gate."""
    gate = world.gate("turn_gate")
    world.scenario(fixture="happy", turn_gate=str(gate), gate_after_lines=1, **scenario)
    first = world.spawn(**BACKGROUND)
    # The consumer's cursor starts at its first connection: trigger only after it is ready.
    await world.until(
        lambda: any(h.get("details", {}).get("adapter") for h in world.fake.heartbeats)
    )
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    # The runner follows the session: its record names the session and its process.
    await world.until(lambda: "short_id" in record(world, run.run_id).get("bg", {}))
    await world.until(lambda: record(world, run.run_id).get("steps", {}).get("tools"))
    first.send_signal(signal.SIGKILL)
    await world.exited(first)
    assert alive(players(world)[0]["pid"])  # the session belongs to the supervisor
    return run.run_id, gate


async def test_a_killed_runner_s_background_turn_is_adopted_with_every_right(
    bg_world: World, logs: LogLines
) -> None:
    world = bg_world
    run_id, gate = await crashed(world)
    seen = len(world.fake.heartbeats)
    await restart(world)
    await world.until(lambda: active(world, run_id, seen))
    [adopted] = [e for e in events(logs, "turn_adopted") if e.get("launch_mode") == "background"]
    assert adopted["action"] == "adopt"
    # A follow-up after the adoption reaches the session through its inbox socket.
    message_id = await world.fake.deliver_followup(run_id, "Also the footer", author="sam")
    await world.until(lambda: world.fake.followup_state(run_id, message_id) == "delivered")
    world.release(gate)
    assert await world.ended(run_id) == "completed"
    assert followup_states(world, message_id) == ["delivered", "absorbed"]
    # The answer is posted exactly once, by the session the first runner launched.
    assert texts(world) == ["The form validates the email twice; I removed the second check."]
    assert len(players(world)) == 1
    assert len(world.calls("bg_launch")) == 1
    assert len(world.requests("startRun")) == 1
    assert len(finishes(world, run_id)) == 1
    assert [c["tokens_in"] for c in finishes(world, run_id)[0]["costs"]] == [2700]
    steps = world.fake.run(run_id)["steps"].values()
    assert [(s["kind"], s["status"]) for s in steps] == [("tool_call", "completed")]
    await world.until(lambda: not record_path(world, run_id).exists())
    await nothing_left(world)


async def test_a_second_kill_of_the_adopter_is_adopted_again(bg_world: World) -> None:
    world = bg_world
    run_id, gate = await crashed(world)
    seen = len(world.fake.heartbeats)
    second = world.spawn(**BACKGROUND)
    await world.until(lambda: active(world, run_id, seen))
    second.send_signal(signal.SIGKILL)
    await world.exited(second)
    assert alive(players(world)[0]["pid"])
    await restart(world)
    world.release(gate)
    assert await world.ended(run_id) == "completed"
    assert texts(world) == ["The form validates the email twice; I removed the second check."]
    assert len(finishes(world, run_id)) == 1
    assert len(world.calls("bg_launch")) == 1
    await nothing_left(world)


async def test_a_stop_reaches_an_adopted_background_turn(bg_world: World) -> None:
    world = bg_world
    run_id, _gate = await crashed(world)
    seen = len(world.fake.heartbeats)
    await restart(world)
    await world.until(lambda: active(world, run_id, seen))
    await world.fake.cancel(run_id, "user_cancelled")
    assert await world.ended(run_id) == "cancelled"
    assert texts(world) == []
    await nothing_left(world)


async def test_a_turn_that_ended_while_the_runner_was_down_is_answered_once(
    bg_world: World,
) -> None:
    world = bg_world
    run_id, gate = await crashed(world)
    world.release(gate)  # the turn ends; the session stays idle, nobody releases it
    await world.until(lambda: any(p.get("status") == "idle" for p in _entries(world)))
    await restart(world)
    assert await world.ended(run_id) == "completed"
    assert texts(world) == ["The form validates the email twice; I removed the second check."]
    assert len(finishes(world, run_id)) == 1
    await nothing_left(world)


def _entries(world: World) -> list[dict[str, Any]]:
    path = world.home / ".claude" / "fake-bg" / "state.json"
    try:
        return list(json.loads(path.read_text())["sessions"].values())
    except (OSError, ValueError, KeyError):
        return []


async def test_a_session_without_a_process_is_finished_from_its_transcript(
    bg_world: World, logs: LogLines
) -> None:
    world = bg_world
    run_id, gate = await crashed(world)
    world.release(gate)
    await world.until(lambda: any(p.get("status") == "idle" for p in _entries(world)))
    pid = players(world)[0]["pid"]
    os.kill(pid, signal.SIGKILL)  # the session process is gone too; its transcript has the end
    await world.until(lambda: not alive(pid))
    await restart(world)
    assert await world.ended(run_id) == "completed"
    assert [e["action"] for e in events(logs, "turn_adopted")] == ["finish_from_log"]
    assert texts(world) == ["The form validates the email twice; I removed the second check."]
    assert len(finishes(world, run_id)) == 1
    await nothing_left(world)


async def test_a_session_that_died_in_its_turn_is_left_to_the_server(
    bg_world: World, logs: LogLines
) -> None:
    world = bg_world
    run_id, _gate = await crashed(world)
    pid = players(world)[0]["pid"]
    os.kill(pid, signal.SIGKILL)  # no process, and no end of the turn in the transcript
    await world.until(lambda: not alive(pid))
    seen = len(world.fake.heartbeats)
    await restart(world)
    await world.until(lambda: not record_path(world, run_id).exists())
    assert [e["action"] for e in events(logs, "turn_adopted")] == ["leave"]
    await world.until(lambda: len(world.fake.heartbeats) > seen)
    assert not active(world, run_id, seen)
    assert finishes(world, run_id) == []
    assert world.fake.run_state(run_id).state == "running"  # the server ends it interrupted
    await nothing_left(world)


async def test_a_run_the_server_ended_meanwhile_has_its_session_stopped(
    bg_world: World, logs: LogLines
) -> None:
    world = bg_world
    run_id, _gate = await crashed(world)
    world.fake.set_run_state(run_id, "failed", "stale")
    await restart(world)
    await world.until(lambda: not record_path(world, run_id).exists())
    assert [e["action"] for e in events(logs, "turn_adopted")] == ["stop"]
    assert finishes(world, run_id) == []
    await nothing_left(world)


async def test_a_runner_killed_before_it_learnt_the_session_id_finds_it_by_name(
    bg_world: World,
) -> None:
    world = bg_world
    hold = world.gate("bg_pid_gate")  # the entry has no process yet: the runner still waits
    world.scenario(fixture="plain", bg_pid_gate=str(hold))
    first = world.spawn(**BACKGROUND)
    await world.until(
        lambda: any(h.get("details", {}).get("adapter") for h in world.fake.heartbeats)
    )
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    assert "short_id" not in record(world, run.run_id)["bg"]  # only the name is recorded
    first.send_signal(signal.SIGKILL)
    await world.exited(first)
    world.release(hold)
    await world.until(lambda: len(players(world)) == 1)
    await restart(world)
    assert await world.ended(run.run_id) == "completed"
    assert texts(world) == ["Done."]
    # The adopter reports the id the CLI made, which the first runner never learnt.
    reports = [r.json["external_session_ref"] for r in world.requests("reportAgentSession")]
    assert reports == [players(world)[0]["session_id"]]
    assert len(world.calls("bg_launch")) == 1
    await nothing_left(world)
