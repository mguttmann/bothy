# SPDX-License-Identifier: Apache-2.0
"""Hosted turns and channel skills as background sessions of the pinned CLI.

FIX-RUNNER-CLI-PIN-326.

Collected only with ``CLAUDE_CODE_RUNNER_REAL_CLI=1`` (``tests/conftest.py``); free: no login and
no model call, the same world as ``test_background.py``. Each launch is the runner's own
argument list (`claude_code_runner.bgsession.background_argv` with the parts of a hosted turn
or with channel skill folders) and its own settings file:

1. A hosted ``scoped`` turn (``--restricted``) starts, offers the skill of its plugin, starts
   the server of its MCP file and keeps the hosted settings, but binds no inbox socket: no
   follow-up and no retry line could reach it, so the runner ends such a run with the cause
   ``background_restricted`` instead.
2. A hosted ``host`` turn (``--setting-sources project,local``) and a turn of the runner's own
   agent with a channel skill folder offer the skills of their plugin folders. Only the own one
   keeps ``bypassPermissions`` and takes the runner's line from its inbox socket: without the
   user settings the hosted session runs in mode ``default`` and holds the line, so the runner
   ends a hosted ``host`` run with the cause ``background_hosted_bypass`` instead.
"""

from __future__ import annotations

import asyncio
import json
import sys
from collections.abc import Mapping, Sequence
from pathlib import Path
from typing import Any

from real_cli.stand_in import DUMMY_KEY
from real_cli.test_background import world

from claude_code_runner.bgsession import DENIED_TOOLS
from claude_code_runner.commandline import HostedArgs
from claude_code_runner.materialize import SETTINGS

MCP_SERVER = (
    "import pathlib, sys\npathlib.Path(sys.argv[1]).write_text('started')\nsys.stdin.read()\n"
)
"""A stand-in MCP server: it only marks that the CLI started it."""


def _plugin(root: Path, skill: str) -> Path:
    folder = root / f"plugin-{skill}"
    (folder / ".claude-plugin").mkdir(parents=True)
    (folder / ".claude-plugin" / "plugin.json").write_text(json.dumps({"name": f"p-{skill}"}))
    (folder / "skills" / skill).mkdir(parents=True)
    (folder / "skills" / skill / "SKILL.md").write_text(
        f"---\nname: {skill}\ndescription: The {skill} probe skill\n---\nAnswer {skill}.\n"
    )
    return folder


def _hosted(root: Path, skill: str, mcp_marker: Path | None) -> HostedArgs:
    settings = root / "hosted-settings.json"
    # ``--restricted`` ignores the user settings with the world's key helper; a real session
    # signs in with the stored login of its configuration folder, which is no setting.
    settings.write_text(json.dumps({**SETTINGS, "apiKeyHelper": f"/bin/echo {DUMMY_KEY}"}))
    mcp = None
    if mcp_marker is not None:
        server = root / "server.py"
        server.write_text(MCP_SERVER)
        mcp = root / "mcp.json"
        command = {"command": sys.executable, "args": ["-I", str(server), str(mcp_marker)]}
        mcp.write_text(json.dumps({"mcpServers": {"probe": command}}))
    return HostedArgs(
        settings=settings, plugin=_plugin(root, skill), mcp_config=mcp, has_skills=True
    )


def _modes(records: Sequence[Mapping[str, Any]]) -> set[str]:
    """The permission modes a session's transcript records."""
    return {str(r.get("permissionMode")) for r in records if r.get("type") == "permission-mode"}


def _sent(bodies: Sequence[Mapping[str, Any]], word: str) -> list[Mapping[str, Any]]:
    return [b for b in bodies if word in json.dumps(b)]


def test_a_hosted_scoped_turn_runs_as_a_background_session() -> None:
    async def scenario() -> None:
        async with world() as (place, script):
            marker = place.root / "mcp-started"
            hosted = _hosted(place.root, "osprey", marker)
            out = await place.launch(
                "hosted", "HOSTED first", tool_class="scoped", tools=["Read"], hosted=hosted
            )
            assert out.refusal is None, place.output("hosted")
            assert out.short_id is not None, place.output("hosted")
            entry = await place.until_entry(out.short_id)
            await place.until_turns(entry.session_id, 1)
            [first] = _sent(script.bodies, "HOSTED first")[:1]
            dumped = json.dumps(first)
            assert "osprey" in dumped  # the skill of the plugin folder is offered
            names = [str(t.get("name")) for t in first.get("tools") or []]
            assert "Skill" in names
            assert "Bash" in names  # SCOPED_BASELINE, read-only by dontAsk
            for tool in DENIED_TOOLS:
                assert tool not in names
            assert marker.read_text() == "started"  # the MCP file was honoured
            settings = json.loads((place.root / "hosted.settings.json").read_text())
            assert settings["disableAllHooks"] is True
            assert set(settings) == {*SETTINGS, "apiKeyHelper", "worktree", "env"}
            # --restricted binds no inbox socket: the runner could write no line.
            assert not place.socket("hosted").exists()

    asyncio.run(scenario())


def test_host_turns_load_their_plugin_folders_in_background() -> None:
    async def scenario() -> None:
        async with world() as (place, script):
            place.accept_bypass()
            hosted = _hosted(place.root, "heron", None)
            channel = _plugin(place.root, "egret")
            out = await place.launch(
                "hostedhost",
                "HOSTHOST first",
                tool_class="host",
                hosted=hosted,
                plugin_dirs=[channel],
            )
            assert out.short_id is not None, place.output("hostedhost")
            own = await place.launch(
                "own", "OWN first", tool_class="host", plugin_dirs=[_plugin(place.root, "ibis")]
            )
            assert own.short_id is not None, place.output("own")
            for short_id in (out.short_id, own.short_id):
                entry = await place.until_entry(short_id)
                await place.until_turns(entry.session_id, 1)
            hosted_body = json.dumps(_sent(script.bodies, "HOSTHOST first")[0])
            assert "heron" in hosted_body
            assert "egret" in hosted_body
            assert "ibis" in json.dumps(_sent(script.bodies, "OWN first")[0])
            # Without the user settings (``--setting-sources project,local``, 26 5.3) the
            # hosted session runs in mode ``default``, not ``bypassPermissions``, and holds the
            # runner's line; the own host session with a plugin folder keeps the bypass and
            # takes it. So a hosted turn never runs in background mode
            # (D-FIX-RUNNER-CLI-PIN-326-10).
            hosted_entry = await place.until_entry(out.short_id)
            own_entry = await place.until_entry(own.short_id)
            assert _modes(place.records(hosted_entry.session_id)) == {"default"}
            assert _modes(place.records(own_entry.session_id)) == {"bypassPermissions"}
            await place.send("own", "OWN second")
            await place.until_turns(own_entry.session_id, 2)
            assert _sent(script.bodies, "OWN second")

    asyncio.run(scenario())
