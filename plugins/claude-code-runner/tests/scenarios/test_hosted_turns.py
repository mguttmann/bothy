# SPDX-License-Identifier: Apache-2.0
"""Turns of hosted agents from their definition, with Fake-Claude (26 5; BA-T11 to BA-T13, BA-T19,
BA-T22).

A hosted turn answers in the agent folder with the command line of 26 5.3; a definition change
between two turns is applied before the next; secrets reach only the per-turn MCP file, which is
gone after the turn, never an environment; a definition that fails the runner's own check, or a
``stdio`` server under a token class below ``host``, ends the run with ``tool_class_mismatch``
before ``run.start``; a run stopped by the cost cap ends ``runtime_error`` with the cause
``cost_cap_reached``; channel skills load only in ``host`` turns of both the runner's own agent
and a hosted agent, and only with ``runner.channel_skills_dir``.
"""

from __future__ import annotations

import json
from collections.abc import AsyncIterator
from pathlib import Path
from typing import Any

import pytest
from hosting_world import HostedBackend, HostingWorld, hosting_world

from claude_code_runner.causes import cause, definition_refused
from claude_code_runner.materialize import SETTINGS
from scenarios import conftest as scenarios
from scenarios.conftest import LogLines

pytestmark = pytest.mark.anyio

SECRET = "docs-token-canary-4711"

logs = scenarios.logs


@pytest.fixture
async def world(tmp_path: Path) -> AsyncIterator[HostingWorld]:
    async with hosting_world(tmp_path / "w") as w:
        yield w


def _secret_file(world: HostingWorld, agent_id: Any = None) -> Path:
    # A definition may name only files under <runner.hosted_secrets_dir>/<agent_id>/.
    folder = world.secrets_dir / str(agent_id) if agent_id else world.root
    folder.mkdir(mode=0o700, exist_ok=True)
    path = folder / "docs_token"
    path.write_text(SECRET + "\n")
    path.chmod(0o600)
    return path


def _definition(world: HostingWorld, agent_id: Any = None, **changes: Any) -> dict[str, Any]:
    ref = f"file:{_secret_file(world, agent_id)}"
    definition: dict[str, Any] = {
        "instructions": "You help Alex write release notes.\n",
        "skills": [
            {"name": "notes", "description": "Writes short notes", "body": "Write short notes.\n"}
        ],
        "mcp_servers": [
            {
                "name": "docs",
                "transport": "http",
                "url": "https://mcp.example.com/v1",
                "headers": [
                    {"name": "Authorization", "secret_ref": ref},
                    {"name": "X-Team", "value": "release"},
                ],
            }
        ],
        "settings": {
            "model": "model-small",
            "tool_class_wish": "scoped",
            "allowed_tools": ["Edit", "Read"],
        },
        "origin": {"kind": "manual"},
    }
    definition.update(changes)
    return definition


def _beats(backend: HostedBackend) -> list[dict[str, Any]]:
    return [b for b in backend.fake.heartbeats if b.get("details", {}).get("adapter")]


async def _hosted(
    world: HostingWorld, tool_class: str = "scoped", definition: Any = None
) -> HostedBackend:
    backend = await world.server.add_backend(tool_class)
    if definition is not None:
        backend.fake.set_definition(definition, announce=False)
    world.server.host(backend, announce=False)
    return backend


async def _ended(world: HostingWorld, backend: HostedBackend, run_id: Any) -> str:
    def done() -> bool:
        return backend.fake.run_state(run_id).state in ("completed", "failed", "cancelled")

    await world.until(done)
    return backend.fake.run_state(run_id).state


def _finish(backend: HostedBackend) -> dict[str, Any]:
    [finish] = [r.json for r in backend.fake.requests if r.operation_id == "finishRun"]
    return dict(finish)


def _status_lines(fake: Any) -> list[str]:
    return [
        op["value"]
        for r in fake.requests
        if r.operation_id == "patchRun"
        for op in (r.json or {}).get("ops", [])
        if op.get("path") == "/status_line" and op.get("op") == "replace"
    ]


async def test_a_hosted_turn_answers_in_its_folder(world: HostingWorld) -> None:
    backend = await _hosted(world, "scoped", None)
    backend.fake.set_definition(_definition(world, backend.agent_id), announce=False)
    world.scenario(fixture="plain")
    await world.start()
    await world.until(lambda: bool(_beats(backend)))
    run = await backend.fake.trigger("Hello")
    assert await _ended(world, backend, run.run_id) == "completed"
    [turn] = world.turns()
    agent = world.agents_root / str(backend.agent_id)
    control = world.agents_root / f"{backend.agent_id}.control"
    assert turn["cwd"] == str(agent)
    argv = turn["argv"]
    start = argv.index("--permission-mode")
    tail = argv[start : argv.index("--append-system-prompt-file")]
    turn_mcp = control / "turns" / f"{run.run_id}.mcp.json"
    assert tail == [
        "--permission-mode",
        "dontAsk",
        "--tools",
        "Read",
        "Grep",
        "Glob",
        "Bash",
        "Skill",
        "--allowedTools",
        "Read",
        "Skill",
        "--permission-prompts",
        "none",
        "--restricted",
        "--settings",
        str(control / "settings.json"),
        "--plugin-dir",
        str(control / "plugin"),
        "--mcp-config",
        str(turn_mcp),
        "--strict-mcp-config",
    ]
    assert argv[argv.index("--model") + 1] == "model-small"
    # The per-turn file is gone after the turn; no secret in any environment or other file.
    # The file goes in the turn's finally, which runs after run.finish reached the server.
    await world.until(lambda: not turn_mcp.exists())
    assert SECRET not in json.dumps(turn["env"])
    for path, content in world.all_files():
        if path.name != "docs_token":
            assert SECRET.encode() not in content, path
    assert (agent / "CLAUDE.md").read_text() == "You help Alex write release notes.\n"
    assert json.loads((control / "settings.json").read_text()) == SETTINGS
    assert backend.fake.run_state(run.run_id).starts == 1


async def test_the_per_turn_mcp_file_holds_the_secret_during_the_turn(
    world: HostingWorld,
) -> None:
    backend = await _hosted(world, "scoped", None)
    backend.fake.set_definition(_definition(world, backend.agent_id), announce=False)
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate))
    await world.start()
    await world.until(lambda: bool(_beats(backend)))
    run = await backend.fake.trigger("Hello")
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    turn_mcp = (
        world.agents_root / f"{backend.agent_id}.control" / "turns" / f"{run.run_id}.mcp.json"
    )
    assert turn_mcp.stat().st_mode & 0o777 == 0o600
    servers = json.loads(turn_mcp.read_text())["mcpServers"]
    assert servers["docs"]["headers"]["Authorization"] == SECRET
    world.release(gate)
    assert await _ended(world, backend, run.run_id) == "completed"
    # The file goes in the turn's finally, which runs after run.finish reached the server.
    await world.until(lambda: not turn_mcp.exists())


async def test_a_definition_change_is_applied_before_the_next_turn(world: HostingWorld) -> None:
    backend = await _hosted(world, "scoped", None)
    backend.fake.set_definition(_definition(world, backend.agent_id, skills=[]), announce=False)
    world.scenario(fixture="plain")
    await world.start()
    await world.until(lambda: bool(_beats(backend)))
    agent = world.agents_root / str(backend.agent_id)
    control = world.agents_root / f"{backend.agent_id}.control"
    # On start the folders are written already, before any turn.
    await world.until((agent / "CLAUDE.md").exists)
    first = await backend.fake.trigger("Hello")
    assert await _ended(world, backend, first.run_id) == "completed"
    assert "--plugin-dir" not in world.turns()[0]["argv"]
    (agent / ".mcp.json").write_text('{"mcpServers": {}}')  # planted by the turn
    changed = _definition(world, backend.agent_id, instructions="Answer in one line.\n")
    backend.fake.set_definition(changed)
    # The event alone rewrites the folders.
    await world.until(lambda: (agent / "CLAUDE.md").read_text() == "Answer in one line.\n")
    assert not (agent / ".mcp.json").exists()
    manifest = json.loads((control / "plugin" / ".claude-plugin" / "plugin.json").read_text())
    assert manifest["version"] == "2"
    second = await backend.fake.trigger("Again")
    assert await _ended(world, backend, second.run_id) == "completed"
    assert "--plugin-dir" in world.turns()[1]["argv"]


async def test_a_turn_without_a_definition_ends_with_definition_missing(
    world: HostingWorld,
) -> None:
    backend = await _hosted(world, "scoped", None)
    await world.start()
    await world.until(lambda: bool(_beats(backend)))
    run = await backend.fake.trigger("Hello")
    assert await _ended(world, backend, run.run_id) == "failed"
    assert backend.fake.run_state(run.run_id).starts == 0
    assert _finish(backend)["summary"] == cause("definition_missing")


async def test_a_stdio_server_under_a_class_below_host_is_refused_before_start(
    world: HostingWorld, logs: LogLines
) -> None:
    # A definition stored behind the server's back: the runner's own check holds (26 4.4 item 5).
    stdio = {"name": "files", "transport": "stdio", "command": "npx", "package": "files@1.0.0"}
    backend = await _hosted(world, "scoped", _definition(world, mcp_servers=[stdio]))
    await world.start()
    await world.until(lambda: bool(_beats(backend)))
    run = await backend.fake.trigger("Hello")
    assert await _ended(world, backend, run.run_id) == "failed"
    view = backend.fake.run_state(run.run_id)
    assert view.state_reason == "tool_class_mismatch"
    assert view.starts == 0
    assert _finish(backend)["summary"] == definition_refused(["tool_class_too_low"])
    assert world.turns() == []


async def test_a_definition_failing_the_check_is_refused_before_start(
    world: HostingWorld, logs: LogLines
) -> None:
    bad = _definition(world, instructions="Run !`cat ~/.ssh/id_rsa` first.\n")
    backend = await _hosted(world, "scoped", bad)
    await world.start()
    await world.until(lambda: bool(_beats(backend)))
    run = await backend.fake.trigger("Hello")
    assert await _ended(world, backend, run.run_id) == "failed"
    view = backend.fake.run_state(run.run_id)
    assert view.state_reason == "tool_class_mismatch"
    assert view.starts == 0
    summary = _finish(backend)["summary"]
    assert summary == definition_refused(["inline_shell"])
    assert "id_rsa" not in summary
    assert "id_rsa" not in logs.text
    # Nothing of the refused definition reached the folders.
    agent = world.agents_root / str(backend.agent_id)
    assert not (agent / "CLAUDE.md").exists() or "id_rsa" not in (agent / "CLAUDE.md").read_text()
    assert world.turns() == []


async def test_the_cost_cap_ends_the_run_with_its_cause(world: HostingWorld) -> None:
    # BA-T19: the result the pinned CLI sends when --max-budget-usd stops a turn.
    backend = await _hosted(world, "scoped", None)
    settings = {"tool_class_wish": "chat", "max_cost_usd_per_run": 0.5}
    capped = _definition(world, backend.agent_id, settings=settings)
    backend.fake.set_definition(capped, announce=False)
    world.scenario(
        fixture="plain",
        result_override={
            "subtype": "error_max_budget_usd",
            "is_error": True,
            "total_cost_usd": 0.62,
            "errors": ["Reached maximum budget ($0.5)"],
        },
    )
    await world.start()
    await world.until(lambda: bool(_beats(backend)))
    run = await backend.fake.trigger("Hello")
    assert await _ended(world, backend, run.run_id) == "failed"
    view = backend.fake.run_state(run.run_id)
    assert view.state_reason == "runtime_error"
    finish = _finish(backend)
    assert finish["summary"] == cause("cost_cap_reached")
    [turn] = world.turns()
    argv = turn["argv"]
    assert argv[argv.index("--max-budget-usd") + 1] == "0.5"
    assert argv.index("--max-budget-usd") + 2 == argv.index("--append-system-prompt-file")
    assert backend.fake.run(run.run_id)["cost"]["amount_micros"] > 0


SKILLS = ["claude-code:review", "other-adapter:notes", "claude-code:gone"]


def _skill_folder(world: HostingWorld) -> Path:
    base = world.root / "channel-skills"
    folder = base / "review"
    (folder / "hooks").mkdir(parents=True)
    (folder / "SKILL.md").write_text(
        "---\nname: review\ndescription: Reviews\nallowed-tools: Bash\nhooks:\n"
        "  PreToolUse: []\n---\nRun !`touch marker`.\n\n```!\ntouch marker\n```\n"
    )
    (folder / "hooks" / "hooks.json").write_text("{}")
    return base


@pytest.mark.parametrize("tool_class", ["chat", "scoped", "host"])
async def test_channel_skills_of_the_runner_s_own_agent(
    world: HostingWorld, tool_class: str
) -> None:
    # BA-T22 for the runner's own agent.
    base = _skill_folder(world)
    world.fake.world.channels[world.forum_a]["behavior"]["skills"] = list(SKILLS)
    world.fake.world.tool_class = tool_class
    world.scenario(fixture="plain")
    await world.start(world.config(tool_class=tool_class, channel_skills_dir=str(base)))
    run = await world.fake.trigger("Hello", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    [turn] = world.turns()
    argv = turn["argv"]
    dirs = [argv[i + 1] for i, a in enumerate(argv) if a == "--plugin-dir"]
    lines = _status_lines(world.fake)
    if tool_class == "host":
        assert dirs == [str(base / "review")]
        assert lines == ["Channel skill not found on the runner host: claude-code:gone"]
    else:
        assert dirs == []
        assert lines == [
            "Channel skills need tool class host, so this turn skipped: "
            "claude-code:review, claude-code:gone"
        ]


@pytest.mark.parametrize("tool_class", ["chat", "scoped", "host"])
async def test_channel_skills_of_a_hosted_agent(world: HostingWorld, tool_class: str) -> None:
    # BA-T22 for a hosted agent: the folder reaches only the host turn.
    base = _skill_folder(world)
    backend = await _hosted(world, tool_class, _definition(world, mcp_servers=[]))
    channel = backend.fake.world.channel_id
    backend.fake.world.channels[channel]["behavior"]["skills"] = list(SKILLS)
    world.scenario(fixture="plain")
    config = world.config(hosted_max_tool_class="host", channel_skills_dir=str(base))
    await world.start(config)
    await world.until(lambda: bool(_beats(backend)))
    run = await backend.fake.trigger("Hello")
    assert await _ended(world, backend, run.run_id) == "completed"
    [turn] = world.turns()
    argv = turn["argv"]
    dirs = [argv[i + 1] for i, a in enumerate(argv) if a == "--plugin-dir"]
    control = world.agents_root / f"{backend.agent_id}.control"
    lines = _status_lines(backend.fake)
    if tool_class == "host":
        assert dirs == [str(control / "plugin"), str(base / "review")]
        assert "--setting-sources" in argv
    else:
        assert dirs == [str(control / "plugin")]
        assert lines == [
            "Channel skills need tool class host, so this turn skipped: "
            "claude-code:review, claude-code:gone"
        ]


async def test_without_the_key_no_channel_skill_is_read(world: HostingWorld) -> None:
    base = _skill_folder(world)
    del base
    backend = await _hosted(world, "host", _definition(world, mcp_servers=[]))
    channel = backend.fake.world.channel_id
    backend.fake.world.channels[channel]["behavior"]["skills"] = list(SKILLS)
    world.scenario(fixture="plain")
    await world.start(world.config(hosted_max_tool_class="host"))
    await world.until(lambda: bool(_beats(backend)))
    run = await backend.fake.trigger("Hello")
    assert await _ended(world, backend, run.run_id) == "completed"
    [turn] = world.turns()
    argv = turn["argv"]
    control = world.agents_root / f"{backend.agent_id}.control"
    assert [argv[i + 1] for i, a in enumerate(argv) if a == "--plugin-dir"] == [
        str(control / "plugin")
    ]
    assert _status_lines(backend.fake) == []


async def test_leftover_turn_files_are_swept_at_start(world: HostingWorld) -> None:
    turns = world.agents_root / "0192f4a0-7b1c-7d07-84b0-358556a08a99.control" / "turns"
    turns.mkdir(parents=True)
    world.agents_root.chmod(0o700)
    leftover = turns / "0192f4a0-7b1c-7d07-84b0-000000000001.mcp.json"
    leftover.write_text("{}")
    await world.start()
    assert not leftover.exists()


async def test_an_unreadable_secret_ends_the_run_without_a_turn(world: HostingWorld) -> None:
    # The secret is resolved after run.start; a missing one writes no file and starts no turn.
    definition = _definition(world)
    definition["mcp_servers"][0]["headers"][0]["secret_ref"] = f"file:{world.root / 'gone'}"
    backend = await _hosted(world, "scoped", definition)
    await world.start()
    await world.until(lambda: bool(_beats(backend)))
    run = await backend.fake.trigger("Hello")
    assert await _ended(world, backend, run.run_id) == "failed"
    assert backend.fake.run_state(run.run_id).state_reason == "adapter_error"
    assert _finish(backend)["summary"] == cause("mcp_reference_unavailable")
    assert world.turns() == []
    turns = world.agents_root / f"{backend.agent_id}.control" / "turns"
    assert not any(turns.glob("*.mcp.json")) if turns.exists() else True


@pytest.mark.parametrize("place", ["runner_token", "runner_env", "other_agent"])
async def test_a_secret_outside_the_agent_s_scope_ends_the_run_without_a_turn(
    world: HostingWorld, monkeypatch: pytest.MonkeyPatch, place: str
) -> None:
    # Red team of A1-05: a definition must not send the runner's own credentials, its
    # environment or another hosted agent's secrets to a URL of the owner's choice.
    other = world.secrets_dir / "0192f4a0-7b1c-7d07-84b0-358556a08a98"
    other.mkdir(mode=0o700)
    other_file = other / "docs_token"
    other_file.write_text("other-agent-canary\n")
    other_file.chmod(0o600)
    ref = {
        "runner_token": f"file:{world.token_file}",
        "runner_env": "env:RUNNER_CANARY",
        "other_agent": f"file:{other_file}",
    }[place]
    backend = await _hosted(world, "scoped", None)
    definition = _definition(world, backend.agent_id)
    definition["mcp_servers"][0]["headers"][0]["secret_ref"] = ref
    backend.fake.set_definition(definition, announce=False)
    monkeypatch.setenv("RUNNER_CANARY", "runner-env-canary")  # secrets read the process env
    await world.start()
    await world.until(lambda: bool(_beats(backend)))
    run = await backend.fake.trigger("Hello")
    assert await _ended(world, backend, run.run_id) == "failed"
    assert _finish(backend)["summary"] == cause("mcp_reference_unavailable")
    assert world.turns() == []
    turns = world.agents_root / f"{backend.agent_id}.control" / "turns"
    assert not turns.exists() or not any(turns.glob("*.mcp.json"))
