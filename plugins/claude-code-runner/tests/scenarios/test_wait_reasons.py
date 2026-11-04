# SPDX-License-Identifier: Apache-2.0
"""Wait reasons and slot use in the heartbeat (09 4.8, 5.2.1, 18.2; F-WARTESCHLANGE-GRUND)."""

from __future__ import annotations

from typing import Any

import pytest

from .conftest import World

pytestmark = pytest.mark.anyio


def _waiting(beat: dict[str, Any]) -> list[dict[str, Any]]:
    return list(beat.get("waiting_runs") or [])


async def test_third_run_waits_for_a_slot(world: World) -> None:
    """With ``max_parallel_runs=2`` the third run is reported as ``slots_full`` with 2 of 2."""
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate))
    await world.start(world.config(max_parallel_runs=2))
    runs = [await world.fake.trigger(f"Run {i}", channel_id=world.forum_a) for i in range(3)]
    ids = {str(r.run_id) for r in runs}
    await world.until(lambda: len(world.calls("gate_wait")) == 2)
    await world.until(lambda: any(_waiting(b) for b in world.fake.heartbeats))
    beat = next(b for b in world.fake.heartbeats if _waiting(b))
    assert beat["run_slots"] == {"used": 2, "total": 2}
    [waiting] = _waiting(beat)
    assert waiting["reason"] == "slots_full"
    assert waiting["run_id"] in ids
    waiting_id = next(r.run_id for r in runs if str(r.run_id) == waiting["run_id"])
    assert world.fake.run_state(waiting_id).state == "queued"
    world.release(gate)
    for run in runs:
        assert await world.ended(run.run_id) == "completed"
    assert world.fake.run_state(waiting_id).state == "completed"
    # Once the run took its slot the runner reported it no longer waiting, at once; the send is
    # scheduled, so it may land after the run ended.
    seen = world.fake.heartbeats.index(beat)
    await world.until(lambda: any(not _waiting(b) for b in world.fake.heartbeats[seen + 1 :]))
    await world.until(lambda: not _waiting(world.fake.heartbeats[-1]))
    last = world.fake.heartbeats[-1]
    assert last["run_slots"]["total"] == 2
    assert not _waiting(last)


async def test_second_run_of_a_thread_waits_for_the_lock(world: World) -> None:
    """A run that waits for the thread lock of a running turn is reported as ``thread_busy``."""
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate))
    await world.start()
    first = await world.fake.trigger("One", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    second = await world.fake.trigger("Two", thread_id=first.thread_id)
    want = {"run_id": str(second.run_id), "reason": "thread_busy"}
    await world.until(lambda: any(want in _waiting(b) for b in world.fake.heartbeats))
    beat = next(b for b in world.fake.heartbeats if want in _waiting(b))
    assert beat["run_slots"] == {"used": 1, "total": 4}
    world.release(gate)
    assert await world.ended(first.run_id) == "completed"
    assert await world.ended(second.run_id) == "completed"
    await world.until(lambda: not _waiting(world.fake.heartbeats[-1]))
