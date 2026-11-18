# SPDX-License-Identifier: Apache-2.0
"""When background mode is unavailable, the run fails with its cause (09 18.9).

Never print mode (ruling D-FIX-RUNNER-CLI-PIN-326-4). Every cause of the table
"Background mode is unavailable": the run ends ``failed`` with ``runtime_error`` and the cause
in its summary, no ``claude -p`` process starts, the log has ``background_unavailable``, and a
cause of the host shows as ``background_unavailable`` in the heartbeat. Also the scope of the
mode: every run in the listed channels, channel skills included, and nothing without the key.
"""

from __future__ import annotations

import json
import os
from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.testing._patience import PATIENCE

from claude_code_runner.causes import cause

from .conftest import BACKGROUND, LogLines, World
from .test_bg_sessions import events, flag, launches, nothing_left, players, start

pytestmark = pytest.mark.anyio

COMMANDS = ("bg_launch", "bg_agents", "bg_stop", "bg_rm", "bg_daemon", "unknown")
"""Every call of a command of background sessions Fake-Claude records."""


@pytest.fixture
def world_timings() -> dict[str, float]:
    return {"agent.cancel_grace": 100 * PATIENCE, "agent.invocation_token_ttl": 100 * PATIENCE}


def status_lines(world: World, run_id: UUID) -> list[str]:
    """Every status line the run showed, in order."""
    return [
        op["value"]
        for r in world.requests("patchRun")
        if str(run_id) in r.path
        for op in r.json.get("ops", [])
        if op["path"] == "/status_line" and op["op"] != "remove"
    ]


def reasons(world: World) -> list[str]:
    return list(world.fake.heartbeats[-1]["details"].get("degraded_reasons", []))


async def fell_back(world: World, logs: LogLines, code: str, *, host: bool) -> UUID:
    """One run that cannot use background mode for ``code``: it fails and says why."""
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run_state(run.run_id).state_reason == "runtime_error"
    assert world.fake.run(run.run_id)["summary"] == cause(code)
    assert world.turns() == []  # never print mode
    assert players(world) == []  # and no session took the prompt
    assert status_lines(world, run.run_id) == []
    assert [e["code"] for e in events(logs, "background_unavailable")] == [code]
    await world.until(lambda: ("background_unavailable" in reasons(world)) == host)
    if host:
        assert world.fake.heartbeats[-1]["state"] == "degraded"
    return run.run_id


async def test_another_version_than_the_pin_fails_with_its_cause(
    bg_world: World, logs: LogLines
) -> None:
    world = bg_world
    world.scenario(fixture="plain", version="2.1.300")
    await start(world)
    await fell_back(world, logs, "background_version_untested", host=True)
    assert launches(world) == []
    assert world.calls("bg_daemon") == []


async def test_a_supervisor_another_program_started_fails_with_its_cause(
    bg_world: World, logs: LogLines
) -> None:
    world = bg_world
    # A supervisor of the pinned version the runner did not start (here: the test's process).
    world.scenario(fixture="plain", bg_supervisor={"pid": os.getpid(), "version": "2.1.326"})
    await start(world)
    await fell_back(world, logs, "background_supervisor_foreign", host=True)
    assert launches(world) == []


async def test_the_own_supervisor_that_ends_right_after_the_check_is_no_foreign_one(
    bg_world: World, logs: LogLines
) -> None:
    world = bg_world
    # The supervisor ends when its last session has ended; here exactly after `daemon status`
    # reported it idle and before the runner read its start time (a race of the full suite).
    world.scenario(fixture="plain", bg_idle_supervisor_ends_after_status=True)
    await start(world)
    first = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(first.run_id) == "completed"
    run = await world.fake.trigger("Fix that", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    assert events(logs, "background_unavailable") == []
    assert world.turns() == []  # both prompts ran as background sessions
    assert len(launches(world)) == len(players(world)) == 2
    assert "background_unavailable" not in reasons(world)
    await nothing_left(world)
    state = json.loads((world.home / ".claude" / "fake-bg" / "state.json").read_text())
    left = (state.get("supervisor") or {}).get("pid")
    if left:  # the second run's supervisor waits for a status call that ends it
        os.kill(left, 15)


async def test_the_own_supervisor_in_another_version_fails_with_its_cause(
    bg_world: World, logs: LogLines
) -> None:
    world = bg_world
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate))
    await start(world)
    held = await world.fake.trigger("Hold the session", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    await world.until(
        lambda: (
            held.run_id
            in [
                UUID(p.stem)
                for p in (world.state / "turns").glob("*.json")
                if "short_id" in p.read_text()
            ]
        )
    )
    recorded = json.loads((world.state / "bg" / "supervisor.json").read_text())
    # The supervisor a launch of the runner started now reports another version.
    forced = {"pid": recorded["pid"], "version": "2.1.327"}
    world.scenario(fixture="plain", bg_supervisor=forced)
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run(run.run_id)["summary"] == cause("background_supervisor_version")
    assert [e["code"] for e in events(logs, "background_unavailable")] == [
        "background_supervisor_version"
    ]
    assert world.turns() == []
    assert len(launches(world)) == 1
    world.release(gate)
    assert await world.ended(held.run_id) == "completed"


async def test_bypass_mode_without_a_recorded_acceptance_fails_with_its_cause(
    bg_world: World, logs: LogLines
) -> None:
    world = bg_world
    world.fake.world.tool_class = "host"
    world.scenario(fixture="plain", bg_refuse="bypass")
    await start(world, tool_class="host")
    await fell_back(world, logs, "background_bypass_not_accepted", host=True)
    [launch] = launches(world)
    assert flag(launch["argv"], "--permission-mode") == "bypassPermissions"
    assert launch["argv"][launch["argv"].index("--disallowedTools") + 1] == "AskUserQuestion"


async def test_a_folder_without_workspace_trust_fails_with_its_cause(
    bg_world: World, logs: LogLines
) -> None:
    world = bg_world
    world.scenario(fixture="plain", bg_refuse="untrusted")
    await start(world)
    await fell_back(world, logs, "background_workspace_untrusted", host=False)
    assert world.fake.heartbeats[-1]["state"] == "online"


async def test_a_files_folder_without_trust_fails_with_its_cause(
    bg_world: World, logs: LogLines
) -> None:
    world = bg_world
    world.scenario(fixture="plain", bg_refuse="add_dir")
    await start(world)
    notes = world.fake.add_file(b"NOTES", name="notes.txt", mime="text/plain")
    run = await world.fake.trigger("Read it", channel_id=world.forum_a, attachments=[notes])
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run(run.run_id)["summary"] == cause("background_files_untrusted")
    assert world.turns() == []  # never print mode
    assert players(world) == []
    [launch] = launches(world)
    assert "--add-dir" in launch["argv"]
    assert [e["code"] for e in events(logs, "background_unavailable")] == [
        "background_files_untrusted"
    ]
    await world.until(lambda: "background_unavailable" in reasons(world))
    # A run without files still runs as a session.
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"


async def test_a_launch_that_fails_fails_with_its_cause(bg_world: World, logs: LogLines) -> None:
    world = bg_world
    world.scenario(fixture="plain", bg_refuse="error")
    await start(world)
    await fell_back(world, logs, "background_launch_failed", host=True)


async def test_a_launch_without_a_process_fails_with_its_cause(
    bg_world: World, logs: LogLines
) -> None:
    world = bg_world
    world.scenario(fixture="plain", bg_no_process=True)
    await start(world, cli_probe_timeout="1s")
    await fell_back(world, logs, "background_launch_failed", host=True)
    # The entry the launch left got no process and took no prompt: it is stopped and removed.
    assert len(world.calls("bg_stop")) == 1
    assert len(world.calls("bg_rm")) == 1
    await nothing_left(world)


async def test_the_next_launch_that_starts_clears_the_heartbeat_reason(
    bg_world: World, logs: LogLines
) -> None:
    world = bg_world
    world.scenario(fixture="plain", bg_refuse="error")
    await start(world)
    await fell_back(world, logs, "background_launch_failed", host=True)
    world.scenario(fixture="plain")
    run = await world.fake.trigger("Again", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    assert len(players(world)) == 1
    assert world.turns() == []
    await world.until(lambda: "background_unavailable" not in reasons(world))
    assert world.fake.heartbeats[-1]["state"] == "online"


def _foreign(session: str, **more: Any) -> dict[str, Any]:
    """An entry of the session list under a name the runner did not give."""
    return {"id": session[:8], "sessionId": session, "name": "my-own-session", **more}


async def test_a_session_another_program_listed_fails_with_its_cause(
    bg_world: World, logs: LogLines
) -> None:
    world = bg_world
    await start(world)
    first = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(first.run_id) == "completed"
    session = players(world)[0]["session_id"]
    entry = _foreign(session, kind="background", state="stopped")
    world.scenario(fixture="resume", bg_foreign_entries=[entry])
    second = await world.fake.trigger("Add a test", thread_id=first.thread_id)
    assert await world.ended(second.run_id) == "failed"
    assert world.fake.run(second.run_id)["summary"] == cause("background_session_foreign")
    assert [e["code"] for e in events(logs, "background_unavailable")] == [
        "background_session_foreign"
    ]
    # No print turn, and the entry of the other program is untouched.
    assert world.turns() == []
    assert len(launches(world)) == 1
    assert [c["id"] for c in world.calls("bg_rm")] == [session[:8]]  # only the first run's own
    assert "background_unavailable" not in reasons(world)


async def test_a_session_another_program_runs_is_busy(bg_world: World) -> None:
    world = bg_world
    await start(world)
    first = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(first.run_id) == "completed"
    session = players(world)[0]["session_id"]
    entry = _foreign(session, kind="interactive", pid=os.getpid(), status="busy")
    world.scenario(fixture="resume", bg_foreign_entries=[entry])
    second = await world.fake.trigger("Add a test", thread_id=first.thread_id)
    assert await world.ended(second.run_id) == "failed"
    assert world.fake.run_state(second.run_id).state_reason == "adapter_error"
    assert world.fake.run(second.run_id)["summary"] == cause("session_busy")
    assert world.turns() == []
    assert len(launches(world)) == 1
    assert len(world.calls("bg_stop")) == 1  # only the release of the first run


async def test_a_launch_in_doubt_ends_the_run_instead_of_running_the_prompt_twice(
    bg_world: World, logs: LogLines
) -> None:
    world = bg_world
    # The command exits 0 without its launch line, but a session started and took the prompt.
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", bg_silent=True, turn_gate=str(gate))
    await start(world)
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run_state(run.run_id).state_reason == "runtime_error"
    assert world.fake.run(run.run_id)["summary"] == cause("background_launch_failed")
    assert world.turns() == []  # never in print mode: the session may have taken the prompt
    assert events(logs, "background_unavailable") == []
    assert len(events(logs, "background_launch_in_doubt")) == 1
    await nothing_left(world)


# ---- the scope of the mode (09 18.2) ------------------------------------------------------------


async def test_only_the_listed_channels_use_background_mode(bg_world: World) -> None:
    world = bg_world
    world.fake.add_forum("Projekt-C", "website")
    await start(world, background_channels=[str(world.forum_b)])
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    assert len(world.turns()) == 1
    assert [c for kind in COMMANDS for c in world.calls(kind)] == []
    assert status_lines(world, run.run_id) == []


async def test_a_listed_channel_uses_background_mode(bg_world: World) -> None:
    world = bg_world
    await start(world, background_channels=[str(world.forum_a)])
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    assert world.turns() == []
    assert len(players(world)) == 1


async def test_an_organization_run_fails_with_its_cause(bg_world: World, logs: LogLines) -> None:
    world = bg_world
    world.fake.world.billing_mode = "organization"
    await start(world)
    await fell_back(world, logs, "background_api_key", host=False)
    assert [c for kind in COMMANDS for c in world.calls(kind)] == []


async def test_a_turn_that_loads_channel_skills_runs_as_a_background_session(
    bg_world: World,
) -> None:
    # The channel skill folder is a --plugin-dir of the launch (26 5.9, FIX-RUNNER-CLI-PIN-326).
    world = bg_world
    base = world.root / "channel-skills"
    (base / "review").mkdir(parents=True)
    (base / "review" / "SKILL.md").write_text("---\nname: review\ndescription: Reviews\n---\n")
    world.fake.world.channels[world.forum_a]["behavior"]["skills"] = ["claude-code:review"]
    world.fake.world.tool_class = "host"
    await start(world, tool_class="host", channel_skills_dir=str(base))
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    assert world.turns() == []
    [launch] = launches(world)
    argv = launch["argv"]
    assert [argv[i + 1] for i, a in enumerate(argv) if a == "--plugin-dir"] == [
        str(base / "review")
    ]
    assert len(players(world)) == 1


async def test_without_the_key_no_command_of_background_sessions_runs(bg_world: World) -> None:
    world = bg_world
    keys = {k: v for k, v in BACKGROUND.items() if k == "allow_personal"}
    runner = await world.start(world.config(**keys))
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    await world.stop(runner)
    assert len(world.turns()) == 1
    assert [c for kind in COMMANDS for c in world.calls(kind)] == []
    assert not (world.state / "bg").exists()
    assert "runs as a background session" not in world.turns()[0]["system_prompt"]
