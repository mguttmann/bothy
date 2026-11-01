# SPDX-License-Identifier: Apache-2.0
"""The cost of a running run grows while its turn works (FIX-cost-display; 09 4.10, 18.7).

Before, ``/cost`` stayed at zero until ``run.finish``: every card of a run that worked for half
an hour showed "0.00 USD". The fixture ``live_cost`` has two API calls of one turn whose
``message.usage`` the runner prices with the start prices; the ``result`` carries the CLI's
amount, which the run books and settles on.
"""

from __future__ import annotations

from typing import Any

import pytest

from .conftest import World

pytestmark = pytest.mark.anyio

# Opus 5.5 start prices: input 4, cache write 5, cache read 0.2, output 20 USD per million tokens.
FIRST = (8 * 4_000_000 + 500 * 20_000_000 + 100_000 * 200_000) // 1_000_000
SECOND = (2 * 4_000_000 + 1_000 * 20_000_000 + 100_500 * 200_000 + 2_000 * 5_000_000) // 1_000_000


async def test_running_run_reports_growing_cost(world: World) -> None:
    gate = world.gate("turn_gate")
    world.scenario(fixture="live_cost", turn_gate=str(gate), gate_after_lines=1)
    await world.start()
    run = await world.fake.trigger("Read the module", channel_id=world.forum_a)

    def cost() -> int:
        return int(world.fake.run(run.run_id)["cost"]["amount_micros"])

    await world.until(lambda: cost() == FIRST)
    assert world.fake.run(run.run_id)["state"] == "running"
    world.release(gate)
    await world.until(lambda: cost() in (FIRST + SECOND, 278094))
    assert await world.ended(run.run_id) == "completed"
    # The finish books the CLI's amount; the run's cost is that amount.
    finishes = [r for r in world.requests("finishRun") if str(run.run_id) in r.path]
    (report,) = finishes[-1].json["costs"]
    assert report["amount_micros"] == 278094
    assert world.fake.run(run.run_id)["cost"]["is_estimate"] is True
    amounts = [
        op["value"]["amount_micros"]
        for r in world.requests("patchRun")
        for op in _ops(r.json)
        if op.get("path") == "/cost"
    ]
    assert amounts == sorted(amounts), amounts
    assert amounts[0] == FIRST


def _ops(body: Any) -> list[dict[str, Any]]:
    items = body if isinstance(body, list) else (body or {}).get("ops", [])
    return [op for op in items if isinstance(op, dict)]
