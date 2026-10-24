# SPDX-License-Identifier: Apache-2.0
"""No secret, stderr text or follow-up content in the runner's output (BR-13, SEC-LOG-4).

The paths of M2-35 (context full, missing session, error result, a follow-up) run with the
canary API key, a canary follow-up text and a canary on Fake-Claude's stderr. The key appears in
no request, log line or file the runner writes; the follow-up text only on stdin and in the
stream files (never in a log line or a patch); the stderr text in no patch, finish body or log.
"""

from __future__ import annotations

import json
from pathlib import Path

import pytest
from bothy_sdk.testing._patience import PATIENCE

from .conftest import API_KEY, LogLines, World

pytestmark = pytest.mark.anyio

FOLLOWUP = "FOLLOWUP-CANARY-5c1e"
STDERR = "STDERR-CANARY-8d2b"


@pytest.fixture
def world_timings() -> dict[str, float]:
    return {"agent.cancel_grace": 100 * PATIENCE, "agent.invocation_token_ttl": 100 * PATIENCE}


def _written_by_runner(state: Path) -> list[Path]:
    # Fake-Claude writes its own session store under the configuration folder; everything
    # else under runner.state_dir is the runner's.
    return [p for p in state.rglob("*") if p.is_file() and "fake-sessions" not in p.parts]


async def test_no_secret_or_content_in_runner_output(world: World, logs: LogLines) -> None:
    gate = world.gate("turn_gate")
    world.scenario(
        fixture="happy",
        stderr_canary=STDERR,
        per_process={
            "0": {"turn_gate": str(gate), "api_error": "context_full", "api_error_after": 1}
        },
    )
    await world.start()
    first = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    message_id = await world.fake.deliver_followup(first.run_id, FOLLOWUP, author="sam")
    await world.until(lambda: world.fake.followup_state(first.run_id, message_id) != "queued")
    world.release(gate)
    assert await world.ended(first.run_id) == "completed"

    world.scenario(fixture="resume", sessions=True, stderr_canary=STDERR)  # session missing
    second = await world.fake.trigger("Add a test", thread_id=first.thread_id)
    assert await world.ended(second.run_id) == "completed"

    world.scenario(fixture="error_texts", stderr_canary=STDERR)
    third = await world.fake.trigger("Once more", channel_id=world.forum_a)
    assert await world.ended(third.run_id) == "failed"

    assert any(FOLLOWUP in line for turn in world.turns() for line in turn["stdin"])
    requests = json.dumps(
        [
            [r.json, {k: v for k, v in r.headers.items() if k != "authorization"}]
            for r in world.fake.requests
        ]
    )
    assert API_KEY not in requests
    assert STDERR not in requests
    patches = json.dumps([r.json for r in world.requests("patchRun")])
    assert FOLLOWUP not in patches
    text = logs.text
    assert logs.records
    for canary in (API_KEY, FOLLOWUP, STDERR):
        assert canary not in text, canary
    for path in _written_by_runner(world.state):
        try:
            data = path.read_bytes()
        except FileNotFoundError:  # a SQLite journal that was just removed
            continue
        assert API_KEY.encode() not in data, path.name
