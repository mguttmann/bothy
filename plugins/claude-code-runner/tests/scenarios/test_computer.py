# SPDX-License-Identifier: Apache-2.0
"""Computer use in the runner (docs/spec/27-computer-use.md 10, CU-T21, CU-T22).

The test plays ``bothy-computer-mcp`` itself: while Fake-Claude holds the turn open, it reads
the per-turn MCP file the runner wrote, writes requests into the request folder as C1 does and
reads the answers. The FakeServer gets the two operations of 27 12.2 (`computer_server`); it
checks the agent token and binds the invocation token to the run like every other operation.
The transcript layout a turn leaves is the one the probe of 27 16 recorded for the pinned CLI
(``<config>/projects/<folder>/<session>.jsonl`` and ``<config>/projects/<folder>/<session>/``).
"""

from __future__ import annotations

import asyncio
import json
import re
import stat
import uuid
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.testing import fakeserver
from bothy_sdk.testing._fake_base import FakeProblem
from bothy_sdk.testing._patience import PATIENCE
from bothy_sdk.testing.fakeserver import FakeServer

from claude_code_runner.causes import cause
from claude_code_runner.computer import COMPUTER_PROMPT, ComputerPaths, mark_left

from .conftest import BACKGROUND, LogLines, World
from .test_bg_sessions import launches

pytestmark = pytest.mark.anyio

HOST = "0192d3a0-0000-7000-8000-0000000000d1"
DESKTOP = {
    "host_id": HOST,
    "name": "Desk",
    "platform": "linux",
    "online": True,
    "locked": False,
    "busy": False,
}
GRANT_TOKEN = "attach-token-of-the-session"
CREDENTIAL_WORDS = ("Invocation", "agent_token", "Authorization", "DPoP")


@pytest.fixture
def world_timings() -> dict[str, float]:
    # Tokens and cancellations never expire on the server's timers inside a scenario.
    return {"agent.cancel_grace": 100 * PATIENCE, "agent.invocation_token_ttl": 100 * PATIENCE}


@pytest.fixture
def computer_server(monkeypatch: pytest.MonkeyPatch) -> dict[str, Any]:
    """The two operations of 27 12.2 on the FakeServer; returns what the test may change."""
    state: dict[str, Any] = {"open": None, "seen": []}
    routes = (
        ("GET", re.compile(r"^/computer/desktops$"), "listComputerDesktops"),
        ("POST", re.compile(r"^/computer/sessions$"), "openComputerSession"),
    )
    monkeypatch.setattr(fakeserver, "_COMPILED", (*fakeserver._COMPILED, *routes))
    original = FakeServer._serve

    async def serve(
        self: FakeServer, operation_id: str, params: dict[str, str], request: Any, body: Any
    ) -> tuple[int, Any, dict[str, str]]:
        if operation_id == "listComputerDesktops":
            state["seen"].append((operation_id, dict(request.headers)))
            return 200, {"items": [{**DESKTOP, "later": 1}]}, {}
        if operation_id == "openComputerSession":
            state["seen"].append((operation_id, dict(request.headers)))
            if state["open"] is not None:
                raise FakeProblem("conflict", reason=state["open"])
            grant = {
                "session_id": str(uuid.uuid4()),
                "token": GRANT_TOKEN,
                "attach_path": "/api/v1/computer/sessions/x/attach",
                "noise_public_key": "a" * 43,
                "key_fingerprint": "f",
                "key_generation": 1,
                "expires_at": "2026-11-26T12:00:00Z",
            }
            return 201, grant, {}
        return await original(self, operation_id, params, request, body)

    monkeypatch.setattr(FakeServer, "_serve", serve)
    return state


def _session(world: World, run_id: UUID) -> str:
    """The session reference ``startRun`` of the run carried."""
    for request in world.requests("startRun"):
        if str(run_id) in request.path:
            return str(request.json["external_session_ref"])
    raise AssertionError("no startRun")


def _binary(world: World) -> Path:
    path = world.root / "bothy-computer-mcp"
    path.write_bytes(b"#!/bin/sh\nexit 0\n")
    path.chmod(0o755)
    return path


def _config(world: World, **runner: Any) -> Any:
    values = {
        "tool_class": "host",
        "computer_use": True,
        "computer_mcp_bin": str(_binary(world)),
        "computer_request_timeout": "12s",
        **runner,
    }
    return world.config(**values)


def _nonce(mcp: Path) -> str:
    return str(
        json.loads(mcp.read_text())["mcpServers"]["bothy_computer"]["env"]["BOTHY_COMPUTER_NONCE"]
    )


def _mcp_files(world: World) -> list[Path]:
    turns = world.state / "turns"
    return sorted(turns.glob("*.computer.mcp.json")) if turns.is_dir() else []


async def _ask(folder: Path, body: dict[str, Any]) -> dict[str, Any]:
    """Write one request the way C1 does and wait for its answer (27 10.2 items 2 and 4)."""
    request_id = str(uuid.uuid4())
    tmp = folder / f"{request_id}.request.json.tmp"
    tmp.write_text(json.dumps(body))
    tmp.rename(folder / f"{request_id}.request.json")
    answer = folder / f"{request_id}.answer.json"
    for _ in range(int(PATIENCE / 0.02)):
        if answer.exists():
            value = dict(json.loads(answer.read_text()))
            (folder / f"{request_id}.request.json").unlink()
            answer.unlink()
            return value
        await asyncio.sleep(0.02)
    raise AssertionError("no answer of the runner")


def _plant_layout(config_dir: Path, session: str, cwd: str) -> list[Path]:
    """What the pinned CLI writes for a turn whose MCP tool returned an image (27 16)."""
    folder = config_dir / "projects" / re.sub(r"[^A-Za-z0-9]", "-", cwd)
    (folder / session / "tool-results").mkdir(parents=True, exist_ok=True)
    transcript = folder / f"{session}.jsonl"
    with transcript.open("a") as out:
        out.write('{"type": "user", "image": "iVBORw0KGgo"}\n')
    image = folder / session / "tool-results" / "mcp-bothy_computer-blob-1-abc.png"
    image.write_bytes(b"\x89PNG screenshot")
    return [transcript, image]


async def _until_file(world: World) -> Path:
    await world.until(lambda: _mcp_files(world))
    [path] = _mcp_files(world)
    return path


async def test_a_host_turn_serves_computer_use_and_removes_the_screenshots(
    world: World, computer_server: dict[str, Any], logs: LogLines
) -> None:
    """CU-T21 and CU-T22 in print mode."""
    world.fake.world.tool_class = "host"
    gate = world.gate("turn_gate")
    world.scenario(fixture="happy", turn_gate=str(gate))
    await world.start(_config(world))
    run = await world.fake.trigger("Open the desktop", channel_id=world.forum_a)
    mcp = await _until_file(world)
    await world.until(world.turns)
    [turn] = world.turns()
    paths = ComputerPaths(world.state, run.run_id)
    assert mcp == paths.turn_file
    assert stat.S_IMODE(mcp.stat().st_mode) == 0o600
    assert stat.S_IMODE(paths.request_dir.stat().st_mode) == 0o700
    document = json.loads(mcp.read_text())
    server = document["mcpServers"]["bothy_computer"]
    env = server["env"]
    assert env["BOTHY_COMPUTER_DIR"] == str(paths.request_dir)
    assert env["BOTHY_COMPUTER_TIMEOUT_MS"] == "12000"
    assert env["BOTHY_COMPUTER_SERVER"] == world.fake.origin
    assert env["BOTHY_COMPUTER_PINS"] == str(world.state / "computer" / "pins.json")
    # The command line of 27 10.1: the file directly after --permission-prompts none.
    argv = turn["argv"]
    at = argv.index("--permission-prompts")
    assert argv[at : at + 4] == ["--permission-prompts", "none", "--mcp-config", str(mcp)]
    assert "--strict-mcp-config" not in argv
    assert COMPUTER_PROMPT.strip() in turn["system_prompt"]
    # No credential and no variable of C1 in the turn's environment (CU-T21).
    assert not [k for k in turn["env"] if k.startswith("BOTHY_COMPUTER")]
    token = world.token_file.read_text().strip()
    assert token not in json.dumps(turn["env"])
    assert token not in mcp.read_text()
    for word in CREDENTIAL_WORDS:
        assert word not in mcp.read_text()

    nonce = env["BOTHY_COMPUTER_NONCE"]
    folder = paths.request_dir
    listed = await _ask(folder, {"v": 1, "nonce": nonce, "op": "list"})
    assert listed == {"v": 1, "ok": True, "result": {"items": [DESKTOP]}}
    refused = await _ask(folder, {"v": 1, "nonce": "guessed", "op": "list"})
    assert refused == {"v": 1, "ok": False, "status": 0, "code": "request_refused", "reason": None}
    opened = await _ask(
        folder,
        {
            "v": 1,
            "nonce": nonce,
            "op": "open",
            "host_id": HOST,
            "client_ephemeral_key_hash": "hash",
            "pin_changed": False,
        },
    )
    assert opened["ok"] is True
    assert opened["result"]["token"] == GRANT_TOKEN
    # Both calls carried the agent token and the run's invocation token.
    seen = [op for op, _ in computer_server["seen"]]
    assert seen == ["listComputerDesktops", "openComputerSession"]
    for _, headers in computer_server["seen"]:
        assert headers.get("Authorization")
        assert headers.get("Invocation-Token")

    session = _session(world, run.run_id)
    config_dir = world.state / "claude-config" / "organization"
    planted = _plant_layout(config_dir, session, turn["cwd"])
    other = config_dir / "projects" / "-other" / "0192d3a0-0000-4000-8000-00000000eeee.jsonl"
    other.parent.mkdir(parents=True)
    other.write_text("{}\n")
    world.release(gate)
    assert await world.ended(run.run_id) == "completed"
    await world.until(lambda: not paths.turn_file.exists() and not paths.request_dir.exists())
    for path in planted:
        assert not path.exists(), path
    assert not (planted[0].parent / session).exists()
    assert other.exists()
    assert "computer_transcript_removed" in logs.text


async def test_a_run_without_an_open_keeps_its_session_files(
    world: World, computer_server: dict[str, Any]
) -> None:
    """Only a run that opened a session ends its Claude Code session (27 10.5)."""
    world.fake.world.tool_class = "host"
    gate = world.gate("turn_gate")
    world.scenario(fixture="happy", turn_gate=str(gate))
    await world.start(_config(world))
    run = await world.fake.trigger("Which desktops are there?", channel_id=world.forum_a)
    mcp = await _until_file(world)
    await world.until(world.turns)
    nonce = _nonce(mcp)
    paths = ComputerPaths(world.state, run.run_id)
    computer_server["open"] = "computer_busy"
    refused = await _ask(
        paths.request_dir,
        {
            "v": 1,
            "nonce": nonce,
            "op": "open",
            "host_id": HOST,
            "client_ephemeral_key_hash": "hash",
            "pin_changed": True,
        },
    )
    assert refused == {
        "v": 1,
        "ok": False,
        "status": 409,
        "code": "conflict",
        "reason": "computer_busy",
    }
    session = _session(world, run.run_id)
    planted = _plant_layout(
        world.state / "claude-config" / "organization", session, world.turns()[0]["cwd"]
    )
    world.release(gate)
    assert await world.ended(run.run_id) == "completed"
    await world.until(lambda: not paths.turn_file.exists())
    assert all(path.exists() for path in planted)


@pytest.mark.parametrize("tool_class", ["chat", "scoped"])
async def test_never_below_host(world: World, tool_class: str) -> None:
    """CU-T21: with the key a turn below ``host`` has no file, no flag and no section."""
    world.fake.world.tool_class = tool_class
    world.scenario(fixture="happy")
    await world.start(_config(world, tool_class=tool_class))
    run = await world.fake.trigger("Hello", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    [turn] = world.turns()
    assert "--mcp-config" not in turn["argv"]
    assert "## Computer use" not in turn["system_prompt"]
    assert not (world.state / "computer-requests").exists()
    assert _mcp_files(world) == []


async def test_without_the_key_nothing_changes(world: World) -> None:
    world.fake.world.tool_class = "host"
    world.scenario(fixture="happy")
    await world.start(world.config(tool_class="host"))
    run = await world.fake.trigger("Hello", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    [turn] = world.turns()
    assert "--mcp-config" not in turn["argv"]
    assert "## Computer use" not in turn["system_prompt"]
    assert not (world.state / "computer-requests").exists()
    assert not (world.state / "computer").exists()


async def test_a_left_transcript_is_named_in_the_next_run(world: World) -> None:
    """A failed deletion is named in the thread's next run and tried again (27 10.5)."""
    world.fake.world.tool_class = "host"
    world.scenario(fixture="happy")
    await world.start(_config(world))
    first = await world.fake.trigger("Hello", channel_id=world.forum_a)
    assert await world.ended(first.run_id) == "completed"
    config_dir = world.root / "elsewhere"
    session = "0192d3a0-0000-4000-8000-00000000abcd"
    planted = _plant_layout(config_dir, session, "/srv/projects/website")
    mark_left(world.state, first.thread_id, config_dir, session)
    run = await world.fake.trigger("Again", channel_id=world.forum_a, thread_id=first.thread_id)
    assert await world.ended(run.run_id) == "completed"
    patches = json.dumps([r.json for r in world.requests("patchRun") if str(run.run_id) in r.path])
    assert json.dumps(cause("computer_transcript_left"))[1:-1] in patches
    assert not any(path.exists() for path in planted)
    assert not (world.state / "computer" / "left" / f"{first.thread_id}.json").exists()


async def test_background_session_gets_the_file_and_is_removed(
    bg_world: World, computer_server: dict[str, Any], logs: LogLines
) -> None:
    """CU-T21 and CU-T22 in background mode: the flag after the class flags; after a run that
    opened a session the session is stopped and removed and its files are deleted."""
    world = bg_world
    world.fake.world.tool_class = "host"
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate))
    await world.start(_config(world, **BACKGROUND))
    run = await world.fake.trigger("Open the desktop", channel_id=world.forum_a)
    mcp = await _until_file(world)
    await world.until(lambda: launches(world))
    [launch] = launches(world)
    argv = launch["argv"]
    at = argv.index("--mcp-config")
    assert argv[at + 1] == str(mcp)
    assert argv[at - 1] == "SendFile"  # directly after the class flags and their deny list
    assert "--strict-mcp-config" not in argv
    paths = ComputerPaths(world.state, run.run_id)
    nonce = _nonce(mcp)
    opened = await _ask(
        paths.request_dir,
        {
            "v": 1,
            "nonce": nonce,
            "op": "open",
            "host_id": HOST,
            "client_ephemeral_key_hash": "hash",
            "pin_changed": False,
        },
    )
    assert opened["ok"] is True
    world.release(gate)
    assert await world.ended(run.run_id) == "completed"
    await world.until(lambda: not paths.turn_file.exists())
    assert world.calls("bg_rm"), "the session's entry is removed"
    leftovers = list((world.home / ".claude" / "projects").rglob("*.jsonl"))
    assert leftovers == [], leftovers
    assert "computer_transcript_removed" in logs.text
