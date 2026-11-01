# SPDX-License-Identifier: Apache-2.0
"""Background work keeps the session reachable (09 18.5, FIX-runner-responsive-turns).

Alex triggers a run whose turn starts a background job and ends with a short answer. While the
job runs, the turn process stays: a follow-up of Alex becomes a turn of its own at once and gets
its own answer, the job's end wakes the session and its answer is posted, and the run stays
``running`` until the background work is done.
"""

from __future__ import annotations

from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.testing._patience import PATIENCE

from claude_code_runner.app import Runner

from .conftest import World

pytestmark = pytest.mark.anyio


@pytest.fixture
def world_timings() -> dict[str, float]:
    return {"agent.cancel_grace": 100 * PATIENCE, "agent.invocation_token_ttl": 100 * PATIENCE}


def texts(world: World) -> list[str]:
    return [m["content_md"] for m in world.fake.messages]


async def background_run(world: World, **runner: Any) -> tuple[Runner, UUID, Any]:
    job = world.gate("background_gate")
    world.scenario(fixture="plain", background_gate=str(job))
    started = await world.start(world.config(**runner))
    run = await world.fake.trigger("Run the suite", channel_id=world.forum_a)
    await world.until(lambda: texts(world) == ["Done."])  # the first turn answered
    return started, run.run_id, job


async def test_followup_while_background_runs_gets_its_own_answer(world: World) -> None:
    _, run_id, job = await background_run(world, followup_grace="200ms")
    message_id = await world.fake.deliver_followup(run_id, "What is the state?", author="alex")
    # The turn process is still there: the follow-up becomes a turn and is answered at once,
    # while the background job keeps running.
    await world.until(lambda: len(texts(world)) == 2)
    assert texts(world)[1] == "Status: the background job is still running."
    assert world.fake.followup_state(run_id, message_id) == "absorbed"
    assert world.state_of(run_id) == "running"
    assert world.calls("background_killed") == []
    world.release(job)
    assert await world.ended(run_id) == "completed"
    assert texts(world)[2] == "The suite passed."
    assert world.calls("background_killed") == []
    assert len(world.turns()) == 1  # one process for the whole conversation


async def test_background_end_wakes_the_session_and_posts_its_answer(world: World) -> None:
    _, run_id, job = await background_run(world, followup_grace="200ms")
    assert world.state_of(run_id) == "running"  # working while the background job runs
    world.release(job)
    assert await world.ended(run_id) == "completed"
    assert texts(world) == ["Done.", "The suite passed."]
    assert world.calls("background_done")
    assert world.calls("background_killed") == []


async def test_stop_during_background_work_ends_everything(world: World) -> None:
    _, run_id, _job = await background_run(world, followup_grace="200ms")
    await world.fake.cancel(run_id)
    assert await world.ended(run_id) == "cancelled"
    assert texts(world) == ["Done."]
    await world.until(lambda: world.calls("turn_end"))
