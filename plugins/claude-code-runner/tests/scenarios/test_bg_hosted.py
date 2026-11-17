# SPDX-License-Identifier: Apache-2.0
"""Hosted agents in background mode, never in print mode (FIX-RUNNER-CLI-PIN-326).

No hosted turn can run as a background session of the pinned CLI: below ``host``
``--restricted`` binds no inbox socket, and under ``host`` the session drops
``bypassPermissions`` without the user settings that 26 5.3 leaves out. Such a turn and an
``organization`` turn end ``failed`` with their cause, and no ``claude -p`` process starts.
"""

from __future__ import annotations

from collections.abc import AsyncIterator
from pathlib import Path
from typing import Any

import pytest
from hosting_world import HostedBackend, HostingWorld, hosting_world

from claude_code_runner.causes import cause
from scenarios import conftest as scenarios
from scenarios.test_bg_sessions import launches
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
    wish = {"tool_class_wish": tool_class, "model": "claude-opus-5-5", **settings}
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


@pytest.mark.parametrize(
    ("tool_class", "mode", "code"),
    [
        ("host", "personal", "background_hosted_bypass"),
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
