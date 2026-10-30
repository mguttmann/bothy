# SPDX-License-Identifier: Apache-2.0
"""Refusals in prepare, locks and parallel turns (design notes 5, 6 and 11)."""

from __future__ import annotations

import asyncio
from typing import Any

import pytest

from .conftest import World

pytestmark = pytest.mark.anyio


async def test_unbound_channel_and_other_trigger_kinds_refused(world: World) -> None:
    fake = world.fake
    await world.start()
    unbound = await fake.trigger("Hello", channel_id=world.werkzeuge)
    assert await world.ended(unbound.run_id) == "failed"
    assert fake.run_state(unbound.run_id).state_reason == "adapter_error"
    interaction = await fake.trigger(
        "Approve", channel_id=world.forum_a, trigger_kind="interaction"
    )
    assert await world.ended(interaction.run_id) == "failed"
    assert fake.run_state(interaction.run_id).state_reason == "adapter_error"
    # The SDK has no counterpart for trigger_kind command yet and refuses its token (M2-29).
    command = await fake.trigger("Run", channel_id=world.forum_a, trigger_kind="command")
    assert await world.ended(command.run_id) == "failed"
    assert fake.run_state(command.run_id).state_reason == "invalid_invocation_token"
    missing = await fake.trigger("Hello", channel_id=world.forum_b)
    assert await world.ended(missing.run_id) == "failed"
    assert fake.run_state(missing.run_id).state_reason == "adapter_error"
    assert world.calls("auth_status") == []
    assert world.turns() == []
    assert world.requests("startRun") == []


def _spans(world: World) -> list[tuple[float, float]]:
    ends = {c["pid"]: c["t"] for c in world.calls("turn_end")}
    return [(t["t"], ends[t["pid"]]) for t in world.turns()]


async def test_session_lock_serializes_one_thread(world: World) -> None:
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate))
    await world.start()
    first = await world.fake.trigger("One", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    second = await world.fake.trigger("Two", thread_id=first.thread_id)
    await asyncio.sleep(0.5)  # the second run waits for the session lock meanwhile
    assert len(world.turns()) == 1
    world.release(gate)
    assert await world.ended(first.run_id) == "completed"
    assert await world.ended(second.run_id) == "completed"
    (_a_start, a_end), (b_start, _) = sorted(_spans(world))
    assert b_start >= a_end


async def test_session_lock_wait_exceeded(world: World) -> None:
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate))
    await world.start(world.config(session_lock_wait="500ms"))
    first = await world.fake.trigger("One", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    second = await world.fake.trigger("Two", thread_id=first.thread_id)
    assert await world.ended(second.run_id) == "failed"
    assert world.fake.run_state(second.run_id).state_reason == "adapter_error"
    (finish,) = [r.json for r in world.requests("finishRun") if str(second.run_id) in r.path]
    assert "`session_busy`" in finish["summary"]
    world.release(gate)
    assert await world.ended(first.run_id) == "completed"
    assert len(world.turns()) == 1


async def test_max_parallel_runs(world: World) -> None:
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate))
    await world.start(world.config(max_parallel_runs=2))
    runs = [await world.fake.trigger(f"Run {i}", channel_id=world.forum_a) for i in range(3)]
    await world.until(lambda: len(world.calls("gate_wait")) == 2)
    await asyncio.sleep(0.5)  # the third run waits for a slot meanwhile
    assert len(world.turns()) == 2
    world.release(gate)
    for run in runs:
        assert await world.ended(run.run_id) == "completed"
    spans = _spans(world)
    assert len(spans) == 3
    events: list[tuple[float, int]] = []
    for start, end in spans:
        events += [(start, 1), (end, -1)]
    running, peak = 0, 0
    for _, step in sorted(events, key=lambda e: (e[0], e[1])):
        running += step
        peak = max(peak, running)
    assert peak == 2


_ = Any
