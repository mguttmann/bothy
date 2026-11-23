"""The scenario world's waits: a wait that runs out names what the runner and Fake-Claude did.

A scenario that hangs in CI cannot be run again with a debugger, so the error of the wait is the
only evidence: it carries the tail of every runner process log, the Fake-Claude records and the
state of every runner process (docs/quality/gates.md, section "Flakes").
"""

from __future__ import annotations

import sys

import pytest

from .conftest import World

pytestmark = pytest.mark.anyio


async def test_a_wait_that_runs_out_names_the_runner_logs_and_fake_claude(world: World) -> None:
    process = world.spawn_command(
        [sys.executable, "-c", "import sys; print('runner said hello', file=sys.stderr)"]
    )
    await world.until(lambda: process.poll() is not None)
    with pytest.raises(AssertionError) as caught:
        await world.until(lambda: False, timeout=0.05)
    text = str(caught.value)
    assert text.startswith("condition not reached")
    assert "runner said hello" in text
    assert "runner-0.log" in text
    assert f"process {process.pid}: exit 0" in text
    assert "Fake-Claude records" in text
