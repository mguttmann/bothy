# SPDX-License-Identifier: Apache-2.0
"""Resumed sessions book only their own turns (09 18.7, RUNNER-RESUME-COST).

The pinned CLI counts ``modelUsage`` for the whole session on ``--resume``: it restores the totals
of the transcript's last ``cost-state`` line. A session the runner never saw (a takeover from an
import, or one started outside the runner) or one continued outside it since its last run must
not book those old totals again. The runner reads the same line before the first resumed turn and
books the difference. The values come from the pinned CLI's recording in
``tests/fixtures/recording/resumed_session_2.1.288``.
"""

from __future__ import annotations

import json
import uuid
from pathlib import Path
from typing import Any

import pytest
from bothy_sdk.ids import uuid7

from .conftest import World

pytestmark = pytest.mark.anyio

RECORDED = (
    Path(__file__).resolve().parents[1] / "fixtures" / "recording" / "resumed_session_2.1.288"
)
# The invented reference of the recording (the runner needs UUIDs).
REF = "0192d3a0-0000-4000-8000-0000000000b2"
MODEL = "claude-sonnet-4-5"


def _recorded_result() -> dict[str, Any]:
    lines = (RECORDED / "stream_resumed.jsonl").read_text().splitlines()
    return next(m for m in map(json.loads, lines) if m.get("type") == "result")


def _transcript(world: World) -> Path:
    """Where the CLI of an ``organization`` turn keeps the session's transcript."""
    folder = world.state / "claude-config" / "organization" / "projects" / "-work"
    folder.mkdir(mode=0o700, parents=True, exist_ok=True)
    for path in (folder, *list(folder.parents)[:3]):
        path.chmod(0o700)  # the runner refuses a state folder others can read
    return folder / f"{REF}.jsonl"


def _cost_state(models: dict[str, tuple[int, float]]) -> str:
    usage = {
        name: {
            "inputTokens": tokens,
            "outputTokens": 40,
            "cacheReadInputTokens": 500,
            "cacheCreationInputTokens": 300,
            "costUSD": cost,
        }
        for name, (tokens, cost) in models.items()
    }
    return json.dumps({"type": "cost-state", "sessionId": REF, "modelUsage": usage}) + "\n"


def _result(models: dict[str, tuple[int, float]]) -> dict[str, Any]:
    recorded = _recorded_result()["modelUsage"][MODEL]
    return {
        "modelUsage": {
            name: {**recorded, "inputTokens": tokens, "costUSD": cost}
            for name, (tokens, cost) in models.items()
        }
    }


def _thread(world: World, baseline: int = 0) -> uuid.UUID:
    thread = uuid7()
    world.fake.world.thread_meta[thread] = world.fake.world._thread(
        thread, world.forum_a, world.fake.world.alex
    )
    row = world.fake.world.session_for(thread, REF, started=True)
    if baseline:
        row["cost_baseline_micros"] = baseline
        row["currency"] = "USD"
    return thread


async def _run(
    world: World, thread: uuid.UUID, override: dict[str, Any], mode: str = "bind"
) -> dict[str, Any]:
    world.scenario(fixture="resume", result_override=override)
    run = await world.fake.trigger("Go on", thread_id=thread, session_mode=mode)
    assert await world.ended(run.run_id) == "completed"
    return [r.json for r in world.requests("finishRun") if str(run.run_id) in r.path][-1]


def _amounts(finish: dict[str, Any]) -> dict[str, int]:
    return {c["model"]: c["amount_micros"] for c in finish["costs"]}


async def test_taken_over_session_books_only_the_resumed_turn(world: World) -> None:
    # Recorded: turn 1 outside the runner cost 0.005475 USD; the resumed turn's result says
    # 0.01095 USD and 2400 input tokens for the whole session.
    await world.start()
    (_transcript(world)).write_text((RECORDED / "transcript.jsonl").read_text())
    thread = _thread(world)
    recorded = _recorded_result()
    finish = await _run(world, thread, {"modelUsage": recorded["modelUsage"]})
    [turn] = world.turns()
    assert turn["env"]["CLAUDE_CONFIG_DIR"] == str(_transcript(world).parents[2])
    assert _amounts(finish) == {MODEL: 5475}
    [cost] = finish["costs"]
    assert (cost["tokens_in"], cost["tokens_out"]) == (1200, 40)
    assert (cost["tokens_cache_read"], cost["tokens_cache_write"]) == (500, 300)
    assert finish["metrics"]["tokens_in"] == recorded["usage"]["input_tokens"]


async def test_transcript_totals_win_over_the_import_baseline(world: World) -> None:
    # The import's baseline describes the same old amount; it is never subtracted on top.
    await world.start()
    _transcript(world).write_text(_cost_state({MODEL: (1200, 0.005475)}))
    thread = _thread(world, baseline=15000)
    finish = await _run(world, thread, _result({MODEL: (2400, 0.01095)}))
    assert _amounts(finish) == {MODEL: 5475}


async def test_use_outside_the_runner_between_runs_is_not_booked(world: World) -> None:
    await world.start()
    transcript = _transcript(world)
    transcript.write_text(_cost_state({MODEL: (1200, 0.005)}))
    thread = _thread(world)
    first = await _run(world, thread, _result({MODEL: (2400, 0.010)}))
    assert _amounts(first) == {MODEL: 5000}
    # The CLI wrote the run's totals; then someone continued the session in a terminal.
    transcript.write_text(_cost_state({MODEL: (2400, 0.010)}) + _cost_state({MODEL: (9000, 3.0)}))
    second = await _run(world, thread, _result({MODEL: (10200, 3.004)}), mode="resume")
    assert _amounts(second) == {MODEL: 4000}
    assert second["costs"][0]["tokens_in"] == 1200


async def test_never_negative_when_the_runtime_counts_less(world: World) -> None:
    await world.start()
    _transcript(world).write_text(_cost_state({MODEL: (9000, 3.0)}))
    thread = _thread(world)
    finish = await _run(world, thread, _result({MODEL: (1200, 0.005)}))
    assert _amounts(finish) == {MODEL: 5000}
    assert all(c["amount_micros"] >= 0 and c["tokens_in"] >= 0 for c in finish["costs"])


async def test_without_a_cost_state_the_import_baseline_applies(world: World) -> None:
    await world.start()
    _transcript(world).write_text(
        (RECORDED / "transcript.jsonl").read_text().splitlines()[0] + "\n"
    )
    thread = _thread(world, baseline=15000)
    finish = await _run(world, thread, _result({MODEL: (5000, 0.0200)}))
    assert _amounts(finish) == {MODEL: 20000 - 15000}
