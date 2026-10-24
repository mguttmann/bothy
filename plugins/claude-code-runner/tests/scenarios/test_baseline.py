# SPDX-License-Identifier: Apache-2.0
"""The cost baseline of a bound session (09 18.7, 4.10; RK-75; M8-06 design note 4).

A session bound from an import (or a rebind) carries ``cost_baseline_micros``: the amount the
runtime's cumulative session values already hold. The first runs after the binding book only the
part above it, so the old session's amount is never booked again.
"""

from __future__ import annotations

import logging
import uuid
from typing import Any

import pytest
from bothy_sdk.ids import uuid7

from .conftest import World

pytestmark = pytest.mark.anyio

# Invented session references (the runner needs UUIDs); none may appear in a log line.
REF_A = "0192d3a0-0000-4000-8000-0000000000a1"


def _usage(models: dict[str, tuple[int, float]]) -> dict[str, Any]:
    """A ``result_override`` with cumulative ``modelUsage``: tokens in and cost per model."""
    return {
        "modelUsage": {
            name: {
                "inputTokens": tokens,
                "outputTokens": 1,
                "cacheReadInputTokens": 0,
                "cacheCreationInputTokens": 0,
                "costUSD": cost,
                "contextWindow": 200000,
            }
            for name, (tokens, cost) in models.items()
        }
    }


def _bind(world: World, baseline: int, currency: str = "USD") -> uuid.UUID:
    """A thread whose current session is the bound import row with the baseline."""
    thread = uuid7()
    world.fake.world.thread_meta[thread] = world.fake.world._thread(
        thread, world.forum_a, world.fake.world.alex
    )
    row = world.fake.world.session_for(thread, REF_A, started=True)
    row["cost_baseline_micros"] = baseline
    row["currency"] = currency
    return thread


async def _run(
    world: World, thread: uuid.UUID, models: dict[str, tuple[int, float]], mode: str
) -> list[dict[str, Any]]:
    """One run in the thread whose result reports ``models``; returns its ``finishRun`` costs."""
    world.scenario(fixture="resume", result_override=_usage(models))
    run = await world.fake.trigger("Go on", thread_id=thread, session_mode=mode)
    assert await world.ended(run.run_id) == "completed"
    finish = [r for r in world.requests("finishRun") if str(run.run_id) in r.path][-1].json
    return list(finish["costs"])


def _amounts(costs: list[dict[str, Any]]) -> dict[str, int]:
    return {c["model"]: c["amount_micros"] for c in costs}


async def test_bound_import_session_books_above_baseline(world: World) -> None:
    await world.start()
    thread = _bind(world, 15000)
    first = await _run(world, thread, {"model-large": (5000, 0.0200)}, "bind")
    assert _amounts(first) == {"model-large": 20000 - 15000}
    second = await _run(world, thread, {"model-large": (6000, 0.0260)}, "resume")
    assert _amounts(second) == {"model-large": 6000}


async def test_baseline_two_models(world: World) -> None:
    await world.start()
    thread = _bind(world, 10000)
    costs = await _run(
        world, thread, {"model-large": (4000, 0.0090), "model-small": (900, 0.0030)}, "bind"
    )
    amounts = _amounts(costs)
    assert sum(amounts.values()) == 9000 + 3000 - 10000
    assert all(v >= 0 for v in amounts.values())
    assert amounts == {"model-large": 1500, "model-small": 500}


async def test_baseline_larger_than_first_run(world: World) -> None:
    await world.start()
    thread = _bind(world, 30000)
    first = await _run(world, thread, {"model-large": (5000, 0.0200)}, "bind")
    assert _amounts(first) == {"model-large": 0}
    assert first[0]["tokens_in"] == 5000  # tokens have no baseline (09 18.7)
    second = await _run(world, thread, {"model-large": (6000, 0.0350)}, "resume")
    assert _amounts(second) == {"model-large": 15000 - 10000}
    third = await _run(world, thread, {"model-large": (7000, 0.0400)}, "resume")
    assert _amounts(third) == {"model-large": 5000}


async def test_counter_reset_clears_baseline(world: World) -> None:
    await world.start()
    thread = _bind(world, 30000)
    first = await _run(world, thread, {"model-large": (5000, 0.0200)}, "bind")
    assert _amounts(first) == {"model-large": 0}
    # The runtime started counting again: the old amounts are no longer in the sums.
    second = await _run(world, thread, {"model-large": (100, 0.0010)}, "resume")
    assert _amounts(second) == {"model-large": 1000}
    third = await _run(world, thread, {"model-large": (200, 0.0030)}, "resume")
    assert _amounts(third) == {"model-large": 2000}


async def test_baseline_other_currency_ignored(
    world: World, caplog: pytest.LogCaptureFixture
) -> None:
    caplog.set_level(logging.DEBUG)
    await world.start()
    thread = _bind(world, 15000, currency="EUR")
    first = await _run(world, thread, {"model-large": (5000, 0.0200)}, "bind")
    assert _amounts(first) == {"model-large": 20000}
    warnings = [r for r in caplog.records if "cost_baseline_currency_ignored" in r.getMessage()]
    assert len(warnings) == 1
    assert warnings[0].levelno == logging.WARNING
    runner_lines = [
        r.getMessage() for r in caplog.records if r.name.startswith("claude_code_runner")
    ]
    assert runner_lines
    assert all(REF_A not in line for line in runner_lines)


async def test_replaced_session_books_the_full_amount(world: World) -> None:
    # Claude Code no longer knows the bound reference: the new session never held the old
    # amount, so the import's baseline does not apply to it (09 18.7, 18.9 session_missing).
    await world.start()
    thread = _bind(world, 15000)
    world.scenario(
        fixture="resume", sessions=True, result_override=_usage({"model-large": (5000, 0.0200)})
    )
    run = await world.fake.trigger("Go on", thread_id=thread, session_mode="bind")
    assert await world.ended(run.run_id) == "completed"
    finish = [r.json for r in world.requests("finishRun") if str(run.run_id) in r.path][-1]
    assert _amounts(finish["costs"]) == {"model-large": 20000}


async def test_replaced_session_without_a_result_never_takes_the_baseline(world: World) -> None:
    # The session that replaced the bound one on a resume ended before its first result; the
    # server rewrites the row's reference and keeps the import's baseline, but the runner
    # created that session itself, so the next run books its full amount (09 18.7, 18.9).
    await world.start()
    thread = _bind(world, 15000)
    world.scenario(
        fixture="resume",
        sessions=True,
        per_process={"1": {"no_result": True, "exit_code": 3, "exit_without_eof": True}},
    )
    run = await world.fake.trigger("Go on", thread_id=thread, session_mode="resume")
    assert await world.ended(run.run_id) == "failed"
    [row] = [r for r in world.fake.sessions(thread) if r["is_current"]]
    assert row["external_session_ref"] != REF_A
    following = _amounts(await _run(world, thread, {"model-large": (6000, 0.0060)}, "resume"))
    assert following == {"model-large": 6000}
