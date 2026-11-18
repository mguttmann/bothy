# SPDX-License-Identifier: Apache-2.0
"""The hook of `claude_code_runner.sendguard` in the pinned CLI (FIX-RUNNER-SENDMESSAGE-OWN).

Collected only with ``CLAUDE_CODE_RUNNER_REAL_CLI=1`` (``tests/conftest.py``); free: no login and
no model call. The CLI runs in print mode against the local stand-in (``real_cli/stand_in.py``)
with a ``HOME`` and a ``CLAUDE_CONFIG_DIR`` of its own, so it sees no session of the developer,
and no background session or supervisor is started. A second print session with the name
``bothy-peer`` in the same configuration folder is the other Claude Code session of the host
user.

The turn has the class flags and the settings file the runner gives a background session with
``own_messages`` (`background_class_flags`, `settings_document`): it launches an agent with
the ``Agent`` tool in the background, then sends ``SendMessage`` to that agent by its
``agentId``, by its name, to the other session by its name, to a ``uds:`` address and to
``main``. Only the message to the ``agentId`` and the one to ``main`` (which the CLI answers
itself) pass the hook; the other session receives nothing. Without the hook the same turn
reaches the other session by its name (the threat the hook closes, run on 2026-11-18).
"""

from __future__ import annotations

import json
import re
import subprocess
import tempfile
import time
from collections.abc import Mapping
from pathlib import Path
from typing import Any

from real_cli.stand_in import cli_env, private_ancestors, stand_in, text
from real_cli.test_real_cli import claude_bin

from claude_code_runner.bgsession import background_class_flags, settings_document

MODEL = "claude-sonnet-4-5"
PEER = "bothy-peer"
UDS = "uds:/tmp/no-such-session.sock"


class Turn:
    """The stand-in's answers: the turn's steps, the agent's answer, the peer's answer."""

    def __init__(self) -> None:
        self.agent: str | None = None
        self.results: list[str] = []
        self.peer_got = 0

    def targets(self) -> list[str]:
        return [self.agent or "no-agent", "helper", PEER, UDS, "main"]

    def __call__(self, body: Mapping[str, Any]) -> list[dict[str, Any]]:
        messages = body.get("messages") or []
        joined = json.dumps(messages)
        first = json.dumps(messages[0]) if messages else ""
        if "PEERSTART" in first:
            if "cross-session-message" in joined:
                self.peer_got += 1
            return [text("peer ok")]
        if "SUBTASK" in first:
            return [text("sub ok")]
        results = [
            block
            for message in messages
            if message.get("role") == "user" and isinstance(message.get("content"), list)
            for block in message["content"]
            if block.get("type") == "tool_result"
        ]
        step = len(results)
        if step:
            self.results.append(json.dumps(results[-1].get("content")))
        if step == 2:
            found = re.search(r"agentId: (a[0-9A-Za-z_-]+)", self.results[-1])
            self.agent = found.group(1) if found else None
        call = f"toolu_{step:02d}"
        if step == 0:
            query = {"query": "select:SendMessage", "max_results": 1}
            return [{"type": "tool_use", "id": call, "name": "ToolSearch", "input": query}]
        if step == 1:
            agent = {
                "description": "helper work",
                "prompt": "SUBTASK do it",
                "subagent_type": "general-purpose",
                "run_in_background": True,
                "name": "helper",
            }
            return [{"type": "tool_use", "id": call, "name": "Agent", "input": agent}]
        targets = self.targets()
        if step - 2 < len(targets):
            send = {"to": targets[step - 2], "message": "extra info", "summary": "extra"}
            return [{"type": "tool_use", "id": call, "name": "SendMessage", "input": send}]
        return [text("done")]


def _run(*, guarded: bool) -> Turn:
    """Run the turn beside the peer; without ``guarded`` it has no hook and no deny rule."""
    turn = Turn()
    root = Path(tempfile.mkdtemp(prefix="ccr-sg-"))
    work = root / "work"
    work.mkdir()
    (root / "peer").mkdir()
    assert not private_ancestors(work), private_ancestors(work)
    with stand_in(turn) as (url, _handler):
        env = cli_env(root, url)
        flags = background_class_flags("host", [], own_messages=True)
        if guarded:
            document = settings_document({"TMPDIR": str(root)}, own_messages=True)
            (root / "run.settings.json").write_text(json.dumps(document), "utf-8")
            flags += ["--settings", str(root / "run.settings.json")]
        peer = subprocess.Popen(
            [
                claude_bin(),
                "-p",
                *("--input-format", "stream-json", "--output-format", "stream-json"),
                *("--verbose", "-n", PEER, "--model", MODEL),
                *("--permission-mode", "bypassPermissions"),
            ],
            cwd=root / "peer",
            env=env,
            stdin=subprocess.PIPE,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        try:
            assert peer.stdin is not None
            start = {"type": "user", "message": {"role": "user", "content": [text("PEERSTART")]}}
            peer.stdin.write((json.dumps(start) + "\n").encode())
            peer.stdin.flush()
            time.sleep(6)  # the peer registers itself as a live session of the folder
            done = subprocess.run(
                [claude_bin(), "-p", "MAIN start", "--model", MODEL, *flags],
                cwd=work,
                env=env,
                stdin=subprocess.DEVNULL,
                capture_output=True,
                text=True,
                timeout=110,
                check=False,
            )
            assert done.returncode == 0, done.stderr[-2000:]
            time.sleep(3)  # a delivered message reaches the peer's model
        finally:
            peer.kill()
            peer.wait(timeout=30)
            if peer.stdin is not None:
                peer.stdin.close()
    return turn


def test_the_hook_lets_messages_reach_the_session_s_own_agents_only() -> None:
    turn = _run(guarded=True)
    assert turn.agent is not None, turn.results
    own, by_name, peer, uds, main = turn.results[2:7]
    assert '\\"success\\":true' in own, own
    assert "Resuming agent" in own, own
    for refused in (by_name, peer, uds):
        assert "PreToolUse:SendMessage hook error" in refused, refused
        assert "not an agent of this session" in refused, refused
    assert "main conversation" in main, main
    assert turn.peer_got == 0


def test_without_the_hook_the_name_of_another_session_reaches_it() -> None:
    # What the hook closes: SendMessage without it delivers to another session by its name.
    turn = _run(guarded=False)
    assert turn.agent is not None, turn.results
    assert "another Claude session on this machine" in turn.results[4], turn.results[4]
    assert turn.peer_got == 1
