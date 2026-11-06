# SPDX-License-Identifier: Apache-2.0
"""Byte-equal fixtures of the runner without the hosting keys (BA-T20, hosting part).

docs/spec/26-builtin-agents.md 3.2 item 4 and 11 item 3: without ``runner.hosted_agents`` the
runner's command line per tool class, its heartbeat bodies and its served channels equal the
fixtures recorded from the version before stage A1 (``tests/fixtures/baseline``, its README says
how). The served forum carries channel skills of both forms (``claude-code:<name>`` and a
reference of another adapter), which the runner without a key ignores.

Values that differ between two runs of the same scenario are replaced by stable names before the
comparison: the scenario's temporary folder by ``<root>``, the forums, agent and instance by
their names, every other UUID by ``<id-N>`` in the order of its first appearance, and the
runner's own release version by ``<runner-version>``. Everything else is compared byte for byte.
"""

from __future__ import annotations

import json
import os
import re
from pathlib import Path
from typing import Any

import pytest
from scenarios import conftest as scenarios
from scenarios.conftest import World

from claude_code_runner import __version__
from claude_code_runner.app import Runner

pytestmark = pytest.mark.anyio

# The fixtures of the scenario world, reused as they are.
world = scenarios.world
world_timings = scenarios.world_timings
world_real_time = scenarios.world_real_time

FIXTURES = Path(__file__).resolve().parents[1] / "fixtures" / "baseline"
RECORD = os.environ.get("CLAUDE_CODE_RUNNER_RECORD_BASELINE") == "1"
"""Write the fixtures instead of comparing (only with a ruling, see the fixtures' README)."""

SKILLS = ["claude-code:review", "other-adapter:notes"]
"""Channel skills of both forms on the served forum (BA-T20)."""

_UUID = re.compile(r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}")


def _names(world: World) -> dict[str, str]:
    return {
        str(world.forum_a): "<forum-a>",
        str(world.forum_b): "<forum-b>",
        str(world.werkzeuge): "<werkzeuge>",
        str(world.fake.world.channel_id): "<allgemein>",
        str(world.fake.agent_id): "<agent>",
        str(world.fake.instance_id): "<instance>",
    }


def normalized(value: Any, world: World) -> str:
    """``value`` as indented JSON with the varying parts replaced (see the module docstring)."""
    text = json.dumps(value, indent=2) + "\n"
    text = text.replace(str(world.root), "<root>")
    text = text.replace(f'"version": "{__version__}"', '"version": "<runner-version>"')
    names = _names(world)
    seen: dict[str, str] = {}

    def name(match: re.Match[str]) -> str:
        found = match.group(0)
        if found in names:
            return names[found]
        return seen.setdefault(found, f"<id-{len(seen) + 1}>")

    return _UUID.sub(name, text)


def check(name: str, text: str) -> None:
    """Compare ``text`` with the fixture ``name`` byte for byte (or record it)."""
    path = FIXTURES / name
    if RECORD:
        path.write_bytes(text.encode("utf-8"))
        return
    assert path.read_bytes() == text.encode("utf-8"), name


def _with_skills(world: World) -> None:
    world.fake.world.channels[world.forum_a]["behavior"]["skills"] = list(SKILLS)


async def _beat(world: World, runner: Runner) -> dict[str, Any]:
    """The heartbeat body the runner sends now, once the first folder check has run.

    The first heartbeat on the wire may go out before or after the runner's first check of the
    bound folders (09 18.15), so the body is taken from the runner itself at a fixed point.
    """
    adapter = runner.adapter
    assert adapter is not None

    def body() -> dict[str, Any]:
        return adapter.heartbeat().to_dict()

    await world.until(lambda: all("trash_check" in r for r in body()["project_readiness"]))
    return body()


@pytest.mark.parametrize("tool_class", ["chat", "scoped", "host"])
async def test_command_line_per_tool_class(world: World, tool_class: str) -> None:
    _with_skills(world)
    world.fake.world.tool_class = tool_class
    world.scenario(fixture="plain")
    await world.start(world.config(tool_class=tool_class))
    run = await world.fake.trigger("Hello", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    [turn] = world.turns()
    check(f"command_line_{tool_class}.json", normalized(turn["argv"], world))


async def test_heartbeat_at_start(world: World) -> None:
    _with_skills(world)
    runner = await world.start()
    check("heartbeat_start.json", normalized(await _beat(world, runner), world))


async def test_heartbeat_with_a_run(world: World) -> None:
    _with_skills(world)
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate))
    runner = await world.start()
    run = await world.fake.trigger("Hello", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    beat = await _beat(world, runner)
    world.release(gate)
    assert await world.ended(run.run_id) == "completed"
    check("heartbeat_with_run.json", normalized(beat, world))


async def test_heartbeat_degraded(world: World) -> None:
    _with_skills(world)
    world.scenario(version="2.0.1")
    runner = await world.start()
    beat = await _beat(world, runner)
    assert beat["state"] == "degraded"
    check("heartbeat_degraded.json", normalized(beat, world))


async def test_served_channels(world: World) -> None:
    _with_skills(world)
    runner = await world.start()
    cache = runner.handler.channels
    assert cache is not None
    served = [
        cache.get(channel_id).to_dict()  # type: ignore[union-attr]
        for channel_id in cache._configs  # the cache as the runner holds it
    ]
    check("served_channels.json", normalized(served, world))
