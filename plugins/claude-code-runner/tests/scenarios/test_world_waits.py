"""The scenario world's waits: a wait that runs out names what the runner and Fake-Claude did.

A scenario that hangs in CI cannot be run again with a debugger, so the error of the wait is the
only evidence: it carries the tail of every runner process log, the Fake-Claude records and the
state of every runner process (docs/quality/gates.md, section "Flakes").
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

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


async def test_a_runner_process_ends_through_the_world_wait(world: World) -> None:
    process = world.spawn_command([sys.executable, "-c", "raise SystemExit(3)"])
    assert await world.exited(process) == 3


def test_no_scenario_waits_on_a_process_beside_the_world_wait() -> None:
    """``Popen.wait`` blocks the event loop of the FakeServer and fails without evidence; every
    scenario waits for a runner process with ``World.exited`` instead (the teardown of the
    world in conftest.py is the one exception)."""
    blocking = re.compile(r"\.wait\(PATIENCE\)|to_thread\(\w+\.wait\b")
    found = [
        f"{path.name}:{number}"
        for path in sorted(Path(__file__).parent.glob("test_*.py"))
        for number, line in enumerate(path.read_text().splitlines(), 1)
        if blocking.search(line) and "blocking.search" not in line
    ]
    assert found == []
