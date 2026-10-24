# SPDX-License-Identifier: Apache-2.0
"""Stop of a run (09 8 point 3, 18.6; design note 6, AG-14).

SIGINT to the turn's process group and stdin closed, SIGTERM after ``runner.stop_grace``,
SIGKILL after ``runner.kill_grace``; ``finishRun`` ``cancelled`` with the request's reason only
once no process of the group is left, without an answer.
"""

from __future__ import annotations

import os
from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.testing._patience import PATIENCE
from bothy_sdk.testing.faults import Fault

from .conftest import LogLines, World

pytestmark = pytest.mark.anyio


@pytest.fixture
def world_timings() -> dict[str, float]:
    # The server never ends a run with cancel_timeout before the runner reports it.
    return {"agent.cancel_grace": 100 * PATIENCE, "agent.invocation_token_ttl": 100 * PATIENCE}


def finishes(world: World, run_id: UUID) -> list[dict[str, Any]]:
    return [r.json for r in world.requests("finishRun") if str(run_id) in r.path]


def gone(pid: int) -> bool:
    try:
        os.killpg(pid, 0)
    except ProcessLookupError:
        return True
    except PermissionError:
        return True  # macOS answers EPERM for a group whose only member is a zombie
    return False


async def gated(world: World, **scenario: Any) -> tuple[UUID, int]:
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate), **scenario)
    await world.start()
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    return run.run_id, world.turns()[0]["pid"]


def escalations(logs: LogLines) -> list[str]:
    return [r.getMessage() for r in logs.records if "stop_escalated" in r.getMessage()]


async def test_stop_sigint_ends_turn_and_reports_reason(world: World, logs: LogLines) -> None:
    run_id, pid = await gated(world, spawn_child=True, echo="never")
    message_id = await world.fake.deliver_followup(run_id, "Wait", author="sam")
    await world.until(lambda: world.fake.followup_state(run_id, message_id) == "delivered")
    await world.fake.cancel(run_id, "user_cancelled")
    assert await world.ended(run_id) == "cancelled"
    child = world.calls("child")[0]["child_pid"]
    assert gone(pid)
    with pytest.raises(ProcessLookupError):
        os.kill(child, 0)
    [finish] = finishes(world, run_id)
    assert (finish["state"], finish["reason"]) == ("cancelled", "user_cancelled")
    assert finish["costs"]  # from the result SIGINT produced
    assert world.fake.messages == []
    assert len(world.turns()) == 1
    assert [c.get("signal") for c in world.calls("turn_end")] == ["SIGINT"]
    assert world.fake.run_state(run_id).state_reason == "user_cancelled"
    del logs


async def test_stop_escalates_to_sigterm(world: World, logs: LogLines) -> None:
    run_id, pid = await gated(world, ignore_sigint=True)
    await world.fake.cancel(run_id)
    assert await world.ended(run_id) == "cancelled"
    assert gone(pid)
    [finish] = finishes(world, run_id)
    assert "costs" not in finish
    assert ['"signal": "SIGTERM"' in m for m in escalations(logs)] == [True]
    messages = [r.getMessage() for r in logs.records]
    escalated = next(i for i, m in enumerate(messages) if "stop_escalated" in m)
    finished = next(i for i, m in enumerate(messages) if '"run_finished"' in m)
    assert escalated < finished


async def test_stop_escalates_to_sigkill(world: World, logs: LogLines) -> None:
    run_id, pid = await gated(world, ignore_sigint=True, ignore_sigterm=True)
    await world.fake.cancel(run_id)
    assert await world.ended(run_id) == "cancelled"
    assert gone(pid)
    signals = ["SIGTERM" if '"SIGTERM"' in m else "SIGKILL" for m in escalations(logs)]
    assert signals == ["SIGTERM", "SIGKILL"]
    assert finishes(world, run_id)[0]["state"] == "cancelled"


@pytest.mark.parametrize(
    "reason", ["access_revoked", "agent_disabled", "user_disabled", "channel_read_only"]
)
async def test_stop_reason_passes_through(world: World, reason: str) -> None:
    run_id, _ = await gated(world)
    await world.fake.cancel(run_id, reason)
    assert await world.ended(run_id) == "cancelled"
    assert [f["reason"] for f in finishes(world, run_id)] == [reason]


async def test_stop_before_start_never_starts(world: World) -> None:
    gate = world.gate("auth_gate")
    world.scenario(fixture="plain", auth_gate=str(gate))
    runner = await world.start()
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    await world.fake.cancel(run.run_id, "access_revoked")
    await world.until(lambda: len(finishes(world, run.run_id)) == 1)
    world.release(gate)
    await world.until(lambda: not runner.handler._runs)
    assert finishes(world, run.run_id) == [{"state": "cancelled", "reason": "access_revoked"}]
    assert world.requests("startRun") == []
    assert world.turns() == []


async def test_stop_during_compaction_starts_no_retry(world: World) -> None:
    gate = world.gate("compact_gate")
    world.scenario(
        fixture="happy",
        compact_gate=str(gate),
        per_process={"0": {"api_error": "context_full", "api_error_after": 1}},
    )
    await world.start()
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("compact")) == 1 and world.calls("gate_wait"))
    await world.fake.cancel(run.run_id)
    assert await world.ended(run.run_id) == "cancelled"
    assert len(world.turns()) == 2  # the turn and the compaction, no retry


async def test_stop_request_twice_changes_nothing(world: World, logs: LogLines) -> None:
    run_id, _ = await gated(world)
    await world.fake.cancel(run_id)
    await world.fake.cancel(run_id)
    assert await world.ended(run_id) == "cancelled"
    assert len(finishes(world, run_id)) == 1
    assert [c.get("signal") for c in world.calls("turn_end")] == ["SIGINT"]
    assert escalations(logs) == []


@pytest.mark.parametrize("answer", ["conflict", "unauthenticated"])
async def test_run_ended_by_server_stops_turn_without_finish(world: World, answer: str) -> None:
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate))
    reason = "run_not_active" if answer == "conflict" else None
    world.fake.inject("patchRun", Fault.problem(answer, reason), times=1000)
    runner = await world.start()
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    pid = world.turns()[0]["pid"]
    await world.until(lambda: gone(pid))
    await world.until(lambda: not runner.handler._runs)
    assert finishes(world, run.run_id) == []
    assert [c.get("signal") for c in world.calls("turn_end")] == ["SIGINT"]


async def test_stop_during_restart_precheck_starts_nothing(world: World) -> None:
    turn_gate, auth_gate = world.gate("turn_gate"), world.gate("auth_gate")
    first = {"turn_gate": str(turn_gate), "api_error": "context_full", "api_error_after": 1}
    world.scenario(fixture="happy", per_process={"0": first})
    runner = await world.start()
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    # The pre-check before the compaction process waits on its gate (I3 per process).
    world.scenario(fixture="happy", per_process={"0": first}, auth_gate=str(auth_gate))
    world.release(turn_gate)
    await world.until(lambda: len(world.calls("gate_wait")) == 2)
    await world.fake.cancel(run.run_id, "user_cancelled")
    state = runner.handler._runs[run.run_id]
    await world.until(lambda: state.turn is not None and state.turn.stop_reason is not None)
    world.release(auth_gate)
    assert await world.ended(run.run_id) == "cancelled"
    assert len(world.turns()) == 1  # no compaction process
    assert world.calls("compact") == []
    assert [f["reason"] for f in finishes(world, run.run_id)] == ["user_cancelled"]
