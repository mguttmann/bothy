# SPDX-License-Identifier: Apache-2.0
"""An orderly stop of the runner never orphans a turn (D-AR-1, test 1 of the design note).

The runner runs as a process of its own (``python -m claude_code_runner run``) and gets
SIGTERM while a turn of its own runs. Without ``runner.turn_handover`` the turn is stopped as in
09 18.6 (SIGINT, SIGTERM, SIGKILL to its group, each escalation logged with
``stop_escalated``), its run ends ``cancelled`` with the reason ``adapter_stopped`` and a summary
the thread shows, and the turn record goes only after no process of the group is left. The
FakeServer clock follows the real time, because the runner process uses the system clock.
"""

from __future__ import annotations

import asyncio
import dataclasses
import json
import os
import signal
import subprocess
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.testing._patience import PATIENCE

from claude_code_runner.app import Runner

from .conftest import LogLines, World

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


def group_gone(pgid: int) -> bool:
    try:
        os.killpg(pgid, 0)
    except ProcessLookupError:
        return True
    except PermissionError:
        return True  # macOS answers EPERM for a group whose only member is a zombie
    return False


def pid_gone(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return True
    return False


def finishes(world: World, run_id: UUID) -> list[dict[str, Any]]:
    return [r.json for r in world.requests("finishRun") if str(run_id) in r.path]


def log_lines(world: World) -> list[dict[str, Any]]:
    lines = []
    for raw in (world.root / "runner-0.log").read_text(errors="replace").splitlines():
        try:
            item = json.loads(raw)
        except ValueError:
            continue
        if isinstance(item, dict):
            lines.append(item)
    return lines


async def live_turn(world: World, **scenario: Any) -> tuple[subprocess.Popen[bytes], UUID, int]:
    """A runner process with a turn of its own paused on its gate; returns runner, run, PID."""
    gate = world.gate("turn_gate")
    world.scenario(fixture="happy", turn_gate=str(gate), **scenario)
    runner = world.spawn()
    # The consumer's cursor starts at its first connection: trigger only after it is ready.
    await world.until(
        lambda: any(h.get("details", {}).get("adapter") for h in world.fake.heartbeats)
    )
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    path = record_path(world, run.run_id)
    await world.until(lambda: path.exists() and "pid" in json.loads(path.read_text()))
    return runner, run.run_id, world.turns()[0]["pid"]


async def sigterm_and_watch(
    world: World, runner: subprocess.Popen[bytes], run_id: UUID, pgid: int
) -> int:
    """SIGTERM to the runner; until it exits, the record never goes while the group lives."""
    path = record_path(world, run_id)
    runner.send_signal(signal.SIGTERM)
    loop = asyncio.get_running_loop()
    deadline = loop.time() + PATIENCE
    while runner.poll() is None:
        assert loop.time() < deadline, "the runner did not exit after SIGTERM"
        # Order matters: the record is read first, so a gone record with a live group means
        # the runner removed it before the group was gone.
        if not path.exists():
            assert group_gone(pgid), "the turn record went while the turn's group was alive"
        await asyncio.sleep(0.005)
    return runner.wait(PATIENCE)


async def test_sigterm_stops_own_turn_and_finishes_it_cancelled(world: World) -> None:
    # A turn that ignores SIGINT, with a background job of its group that ignores it too: the
    # stop must escalate to SIGTERM, and no process of the group may outlive the runner.
    runner, run_id, pid = await live_turn(
        world, ignore_sigint=True, spawn_child=True, child_ignores_sigint=True
    )
    child = int(world.calls("child")[0]["child_pid"])

    code = await sigterm_and_watch(world, runner, run_id, pid)

    assert code == 0
    assert group_gone(pid)
    assert pid_gone(child)
    assert not record_path(world, run_id).exists()
    [finish] = finishes(world, run_id)
    assert (finish["state"], finish["reason"]) == ("cancelled", "adapter_stopped")
    assert finish["summary"].endswith("(`adapter_stopped`)")
    assert world.fake.run_state(run_id).state == "cancelled"
    assert world.fake.run_state(run_id).state_reason == "adapter_stopped"
    assert world.fake.messages == []  # a stop posts no answer
    escalated = [e for e in log_lines(world) if e.get("event") == "stop_escalated"]
    assert escalated
    assert {e.get("run_id") for e in escalated} == {str(run_id)}
    assert len(world.turns()) == 1


async def test_sigterm_turn_that_answers_sigint_still_ends_cancelled(world: World) -> None:
    # SIGINT ends this turn with an error result at once: no escalation is needed, and the run
    # still ends cancelled with the reason, never as a failed or completed turn.
    runner, run_id, pid = await live_turn(world)

    code = await sigterm_and_watch(world, runner, run_id, pid)

    assert code == 0
    assert group_gone(pid)
    assert not record_path(world, run_id).exists()
    [finish] = finishes(world, run_id)
    assert (finish["state"], finish["reason"]) == ("cancelled", "adapter_stopped")
    assert finish["costs"]  # from the result SIGINT produced
    assert world.fake.messages == []
    assert [c.get("signal") for c in world.calls("turn_end")] == ["SIGINT"]


async def test_next_runner_after_an_orderly_stop_starts_nothing_for_the_run(
    world: World,
) -> None:
    # Nothing of the stopped turn is left for the next runner: no record, no adoption, no
    # second finish, no second process.
    runner, run_id, pid = await live_turn(world, ignore_sigint=True)
    await sigterm_and_watch(world, runner, run_id, pid)
    seen = len(world.fake.heartbeats)
    await world.start()
    await world.until(lambda: len(world.fake.heartbeats) > seen)
    assert all(
        str(run_id) not in h.get("details", {}).get("active_runs", [])
        for h in world.fake.heartbeats[seen:]
    )
    assert len(finishes(world, run_id)) == 1
    assert len(world.turns()) == 1
    assert len(world.requests("startRun")) == 1


async def test_sdk_cancel_of_a_live_turn_stops_its_group_before_the_record_goes(
    world: World, logs: LogLines
) -> None:
    # The safety net of `RunTurn.run`: a turn that the stop of the turns did not reach (one that
    # started after it, or outlived its wait) is stopped when the SDK cancels its run. The
    # adapter is stopped directly here, so only that path runs.
    gate = world.gate("turn_gate")
    world.scenario(fixture="happy", turn_gate=str(gate), ignore_sigint=True, spawn_child=True)
    runner = await world.start()
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    path = record_path(world, run.run_id)
    await world.until(lambda: path.exists() and "pid" in json.loads(path.read_text()))
    pid = world.turns()[0]["pid"]
    child = int(world.calls("child")[0]["child_pid"])
    assert runner.adapter is not None

    await runner.adapter.stop()
    loop = asyncio.get_running_loop()
    deadline = loop.time() + PATIENCE
    while path.exists():
        assert loop.time() < deadline, "the turn record stayed"
        await asyncio.sleep(0.005)
    # Seen right after the record went: no process of the group may be left by then.
    assert group_gone(pid), "the turn record went while the turn's group was alive"

    assert group_gone(pid)
    assert pid_gone(child)
    [finish] = finishes(world, run.run_id)
    assert (finish["state"], finish["reason"]) == ("cancelled", "adapter_stopped")
    assert finish["summary"].endswith("(`adapter_stopped`)")
    escalated = [r.getMessage() for r in logs.records if "stop_escalated" in r.getMessage()]
    assert escalated
    assert all(str(run.run_id) in line for line in escalated)


async def in_process_turn(world: World, **scenario: Any) -> tuple[Runner, UUID, int]:
    """An in-process runner with a turn of its own paused on its gate; returns runner, run, PID."""
    gate = world.gate("turn_gate")
    world.scenario(fixture="happy", turn_gate=str(gate), **scenario)
    runner = await world.start()
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    path = record_path(world, run.run_id)
    await world.until(lambda: path.exists() and "pid" in json.loads(path.read_text()))
    return runner, run.run_id, world.turns()[0]["pid"]


async def test_runner_stop_waits_until_the_own_turn_ended(world: World, logs: LogLines) -> None:
    # `Runner.stop` stops the own turn and returns only once its run ended: by then the group
    # is gone, the record is gone and the run is finished cancelled.
    runner, run_id, pid = await in_process_turn(world, ignore_sigint=True)

    assert await runner.handler.stop_turns()

    assert group_gone(pid)
    assert not record_path(world, run_id).exists()
    [finish] = finishes(world, run_id)
    assert (finish["state"], finish["reason"]) == ("cancelled", "adapter_stopped")
    assert "stop_turns_timeout" not in logs.text
    await world.stop(runner)
    assert len(finishes(world, run_id)) == 1


async def test_runner_stop_past_its_wait_leaves_the_turn_to_the_cancel(
    world: World, logs: LogLines
) -> None:
    # A turn that outlives the wait of the stop is logged with stop_turns_timeout and still
    # stopped by the cancel of its run: no process of the group is left once its record went.
    runner, run_id, pid = await in_process_turn(world, ignore_sigint=True)
    runner.handler.config = dataclasses.replace(
        runner.handler.config, stop_grace=0.0, kill_grace=0.0, cli_probe_timeout=0.0
    )

    await world.stop(runner)

    assert "stop_turns_timeout" in logs.text
    await world.until(lambda: not record_path(world, run_id).exists())
    assert group_gone(pid)
    [finish] = finishes(world, run_id)
    assert (finish["state"], finish["reason"]) == ("cancelled", "adapter_stopped")
