# SPDX-License-Identifier: Apache-2.0
"""Hosted agents in background mode, never in print mode (FIX-RUNNER-CLI-PIN-326).

A hosted ``host`` turn runs as a background session with the parts of 26 5.3 as launch flags
and its settings in the launch's one settings file; the runner holds the cost cap itself. A
hosted turn below ``host`` (``--restricted`` binds no inbox socket) and an ``organization`` turn
end ``failed`` with their cause, and no ``claude -p`` process starts.
"""

from __future__ import annotations

from collections.abc import AsyncIterator
from pathlib import Path
from typing import Any

import pytest
from hosting_world import HostedBackend, HostingWorld, hosting_world

from claude_code_runner.causes import cause
from claude_code_runner.materialize import SETTINGS
from scenarios import conftest as scenarios
from scenarios.test_bg_sessions import flag, launches, players
from scenarios.test_hosted_turns import _beats, _definition, _ended, _finish, _hosted

pytestmark = pytest.mark.anyio

short_dir = scenarios.short_dir

BACKGROUND = {**scenarios.BACKGROUND, "hosted_max_tool_class": "host"}


@pytest.fixture
async def world(short_dir: Path) -> AsyncIterator[HostingWorld]:
    async with hosting_world(short_dir / "w") as w:
        (w.home / ".claude" / "fake-login").write_text("claude.ai")  # the host user's login
        yield w


async def _start(
    world: HostingWorld, tool_class: str, mode: str = "personal", **settings: Any
) -> HostedBackend:
    backend = await _hosted(world, tool_class, None)
    wish = {"tool_class_wish": tool_class, "model": "claude-fable-5-1", **settings}
    definition = _definition(world, backend.agent_id, settings=wish)
    if tool_class == "host":
        definition["mcp_servers"] = []  # an http server with a secret needs no change here
    backend.fake.set_definition(definition, announce=False)
    backend.fake.world.billing_mode = mode
    world.fake.world.billing_mode = mode
    world.scenario(fixture="plain")
    await world.start(world.config(**BACKGROUND))
    await world.until(lambda: bool(_beats(backend)))
    return backend


async def test_a_hosted_host_turn_runs_as_a_background_session(world: HostingWorld) -> None:
    backend = await _start(world, "host")
    run = await backend.fake.trigger("Hello")
    assert await _ended(world, backend, run.run_id) == "completed"
    assert world.turns() == []  # never claude -p
    [launch] = launches(world)
    argv = launch["argv"]
    control = world.agents_root / f"{backend.agent_id}.control"
    assert "--bg" in argv
    assert flag(argv, "--setting-sources") == "project,local"
    assert flag(argv, "--plugin-dir") == str(control / "plugin")
    assert "--strict-mcp-config" in argv
    assert "--max-budget-usd" not in argv
    # One settings file: the hosted settings with the two members of a launch.
    settings = launch["settings"]
    assert {k: settings[k] for k in SETTINGS} == SETTINGS
    assert settings["worktree"] == {"bgIsolation": "none"}
    # The launch files lie under the runner's own state folder.
    assert Path(flag(argv, "--settings")).parent == world.state / "bg"
    assert len(players(world)) == 1


async def test_the_runner_holds_the_cost_cap_of_a_background_session(world: HostingWorld) -> None:
    backend = await _start(world, "host", max_cost_usd_per_run=0.0001)
    run = await backend.fake.trigger("Hello")
    assert await _ended(world, backend, run.run_id) == "failed"
    assert backend.fake.run_state(run.run_id).state_reason == "runtime_error"
    assert _finish(backend)["summary"] == cause("cost_cap_reached")
    assert world.turns() == []


@pytest.mark.parametrize(
    ("tool_class", "mode", "code"),
    [
        ("scoped", "personal", "background_restricted"),
        ("chat", "personal", "background_restricted"),
        ("host", "organization", "background_api_key"),
    ],
)
async def test_a_hosted_turn_that_cannot_run_in_background_fails_with_its_cause(
    world: HostingWorld, tool_class: str, mode: str, code: str
) -> None:
    backend = await _start(world, tool_class, mode)
    run = await backend.fake.trigger("Hello")
    assert await _ended(world, backend, run.run_id) == "failed"
    assert backend.fake.run_state(run.run_id).state_reason == "runtime_error"
    assert _finish(backend)["summary"] == cause(code)
    assert world.turns() == []
    assert launches(world) == []
