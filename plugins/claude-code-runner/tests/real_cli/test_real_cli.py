# SPDX-License-Identifier: Apache-2.0
"""Tests against the installed Claude Code CLI (09 16.1, 18.9, RK-14; design note 17).

Collected only with ``CLAUDE_CODE_RUNNER_REAL_CLI=1``; a missing CLI then fails, never skips.
Level one is free and needs no credential. Level two (``CLAUDE_CODE_RUNNER_REAL_TURN=1`` in
addition) uses the operator's key or login and costs money: it is the owner's decision.

Inputs: ``CLAUDE_CODE_RUNNER_CLAUDE_BIN`` (default ``claude`` in ``PATH``); level two also
``CLAUDE_CODE_RUNNER_REAL_KEY_FILE`` (a file with the API key of ``organization``).
"""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import tempfile
import uuid
from pathlib import Path

import pytest

from claude_code_runner.authcheck import parse_auth_method
from claude_code_runner.heartbeat import (
    MIN_CLAUDE_CODE_VERSION,
    PINNED_CLAUDE_CODE_VERSION,
    parse_version,
)
from claude_code_runner.usage import parse_model_usage

CONTRACT_FLAGS = (
    "-p",
    "--input-format",
    "--output-format",
    "--verbose",
    "--replay-user-messages",
    "--model",
    "--session-id",
    "--resume",
    "--permission-mode",
    "--tools",
    "--allowedTools",
    "--disallowedTools",
    "--permission-prompts",
    "--add-dir",
    "--append-system-prompt-file",
)
turn = pytest.mark.skipif(
    os.environ.get("CLAUDE_CODE_RUNNER_REAL_TURN") != "1",
    reason="level two needs CLAUDE_CODE_RUNNER_REAL_TURN=1 (real cost, operator's login)",
)


def claude_bin() -> str:
    name = os.environ.get("CLAUDE_CODE_RUNNER_CLAUDE_BIN", "claude")
    found = shutil.which(name)
    assert found, "the Claude Code CLI is not installed (level one fails, never skips)"
    return found


def run(
    args: list[str], env: dict[str, str] | None = None, **kw: object
) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [claude_bin(), *args],
        capture_output=True,
        text=True,
        timeout=120,
        env=env,
        check=False,
        **kw,
    )  # type: ignore[call-overload]


def test_installed_version_equals_pin() -> None:
    version = parse_version(run(["--version"]).stdout)
    assert version is not None
    assert ".".join(map(str, version)) == PINNED_CLAUDE_CODE_VERSION
    assert version >= parse_version(MIN_CLAUDE_CODE_VERSION)  # type: ignore[operator]


def test_help_lists_contract_flags() -> None:
    text = run(["--help"]).stdout
    for flag in CONTRACT_FLAGS:
        # The help of 2.1.259 names the file variants only as "--append-system-prompt[-file]".
        short = flag.removesuffix("-file") + "[-file]"
        assert flag in text or (flag.endswith("-file") and short in text), flag


@turn
def test_auth_status_per_mode() -> None:
    key = Path(os.environ["CLAUDE_CODE_RUNNER_REAL_KEY_FILE"]).read_text().strip()
    with tempfile.TemporaryDirectory() as config_dir:
        org = {
            "HOME": os.environ["HOME"],
            "PATH": os.environ["PATH"],
            "ANTHROPIC_API_KEY": key,
            "CLAUDE_CONFIG_DIR": config_dir,
        }
        assert parse_auth_method(run(["auth", "status"], env=org).stdout) == "api_key"
    personal = {"HOME": os.environ["HOME"], "PATH": os.environ["PATH"]}
    assert parse_auth_method(run(["auth", "status"], env=personal).stdout) in (
        "claude.ai",
        "oauth_token",
    )


@turn
def test_real_turn_streams_mapped_messages() -> None:
    key = Path(os.environ["CLAUDE_CODE_RUNNER_REAL_KEY_FILE"]).read_text().strip()
    session = str(uuid.uuid4())
    with tempfile.TemporaryDirectory() as work, tempfile.TemporaryDirectory() as config_dir:
        env = {
            "HOME": os.environ["HOME"],
            "PATH": os.environ["PATH"],
            "ANTHROPIC_API_KEY": key,
            "CLAUDE_CONFIG_DIR": config_dir,
        }
        usage = []
        for flag in ("--session-id", "--resume"):
            line = json.dumps(
                {
                    "type": "user",
                    "message": {
                        "role": "user",
                        "content": [{"type": "text", "text": "Reply with one word."}],
                    },
                }
            )
            out = run(
                [
                    "-p",
                    "--input-format",
                    "stream-json",
                    "--output-format",
                    "stream-json",
                    "--verbose",
                    "--replay-user-messages",
                    flag,
                    session,
                    "--permission-mode",
                    "dontAsk",
                    "--tools",
                    "",
                    "--disallowedTools",
                    "mcp__*",
                    "--permission-prompts",
                    "none",
                ],
                env=env,
                input=line + "\n",
                cwd=work,
            )
            messages = [json.loads(x) for x in out.stdout.splitlines() if x.startswith("{")]
            kinds = {(m["type"], m.get("subtype")) for m in messages}
            assert ("system", "init") in kinds
            assert any(k[0] == "assistant" for k in kinds)
            result = next(m for m in messages if m["type"] == "result")
            usage.append(parse_model_usage(result))
        model = next(iter(usage[1]))
        assert usage[1][model].input_tokens >= usage[0][model].input_tokens


def _user(text: str) -> str:
    doc = {
        "type": "user",
        "uuid": str(uuid.uuid4()),
        "message": {"role": "user", "content": [{"type": "text", "text": text}]},
    }
    return json.dumps(doc) + "\n"


@turn
def test_real_background_work_keeps_the_session_reachable() -> None:
    """What 09 18.5 "Background work" relies on (FIX-runner-responsive-turns).

    A turn that starts a background command ends with a ``result`` while the command runs; a
    line written then gets a ``result`` of its own; the end of the command brings a turn the
    CLI starts by itself (a ``result`` that names no input); the end of stdin ends the process.
    """
    key = Path(os.environ["CLAUDE_CODE_RUNNER_REAL_KEY_FILE"]).read_text().strip()
    with tempfile.TemporaryDirectory() as work, tempfile.TemporaryDirectory() as config_dir:
        env = {
            "HOME": os.environ["HOME"],
            "PATH": os.environ["PATH"],
            "ANTHROPIC_API_KEY": key,
            "CLAUDE_CONFIG_DIR": config_dir,
        }
        argv = [
            claude_bin(),
            "-p",
            "--input-format",
            "stream-json",
            "--output-format",
            "stream-json",
            "--verbose",
            "--replay-user-messages",
            "--session-id",
            str(uuid.uuid4()),
            "--permission-mode",
            "bypassPermissions",
            "--permission-prompts",
            "none",
        ]
        process = subprocess.Popen(
            argv,
            cwd=work,
            env=env,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            text=True,
        )
        assert process.stdin is not None
        assert process.stdout is not None
        process.stdin.write(
            _user(
                "Run `sleep 40; echo finished` with the Bash tool and run_in_background true, "
                "then end your turn at once with the word started."
            )
        )
        process.stdin.flush()
        results: list[dict[str, object]] = []
        tasks_seen = False
        try:
            for raw in process.stdout:
                if not raw.startswith("{"):
                    continue
                message = json.loads(raw)
                if message.get("subtype") == "background_tasks_changed" and message.get("tasks"):
                    tasks_seen = True
                if message.get("type") != "result":
                    continue
                results.append(message)
                if len(results) == 1:
                    process.stdin.write(_user("Reply with one word: status."))
                    process.stdin.flush()
                if len(results) == 3:  # the first turn, the status turn, the notification
                    process.stdin.close()
            assert process.wait(timeout=120) == 0
        finally:
            if process.poll() is None:
                process.kill()
        assert tasks_seen
        assert len(results) == 3
        assert results[1].get("user_message_uuids") or results[1].get("user_message_uuid")
        assert not results[2].get("user_message_uuids")
        assert not results[2].get("user_message_uuid")


@turn
def test_real_line_written_during_a_turn_is_taken_at_the_next_tool_boundary() -> None:
    """When a follow-up written during a turn reaches the model (FIX-runner-reachable-2).

    The runner writes a follow-up at once, also while a turn runs. The pinned CLI takes such a
    line into the running turn after the tool call that runs when it arrives (its echo comes
    before the next tool call), not only when the turn ends; the turn's reply covers it.
    """
    key = Path(os.environ["CLAUDE_CODE_RUNNER_REAL_KEY_FILE"]).read_text().strip()
    with tempfile.TemporaryDirectory() as work, tempfile.TemporaryDirectory() as config_dir:
        env = {
            "HOME": os.environ["HOME"],
            "PATH": os.environ["PATH"],
            "ANTHROPIC_API_KEY": key,
            "CLAUDE_CONFIG_DIR": config_dir,
        }
        argv = [
            claude_bin(),
            "-p",
            "--input-format",
            "stream-json",
            "--output-format",
            "stream-json",
            "--verbose",
            "--replay-user-messages",
            "--session-id",
            str(uuid.uuid4()),
            "--permission-mode",
            "bypassPermissions",
            "--permission-prompts",
            "none",
            "--tools",
            "Bash",
        ]
        process = subprocess.Popen(
            argv,
            cwd=work,
            env=env,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            text=True,
        )
        assert process.stdin is not None
        assert process.stdout is not None
        process.stdin.write(
            _user(
                "Run `sleep 15; echo one` and then `sleep 15; echo two`, each as its own "
                "foreground Bash call. Then reply with the word done."
            )
        )
        process.stdin.flush()
        followup = _user("Reply also with the word status.")
        followup_id = json.loads(followup)["uuid"]
        events: list[str] = []
        try:
            for raw in process.stdout:
                if not raw.startswith("{"):
                    continue
                message = json.loads(raw)
                kind = message.get("type")
                if kind == "assistant":
                    for block in message["message"]["content"]:
                        if block.get("type") == "tool_use":
                            events.append("tool_use")
                            if events.count("tool_use") == 1:
                                process.stdin.write(followup)
                                process.stdin.flush()
                elif (
                    kind == "user"
                    and message.get("isReplay")
                    and message.get("uuid") == followup_id
                ):
                    events.append("followup_echo")
                elif kind == "result":
                    events.append("result")
                    process.stdin.close()
                    break
            process.wait(timeout=120)
        finally:
            if process.poll() is None:
                process.kill()
        assert "followup_echo" in events
        echo = events.index("followup_echo")
        assert events.index("result") > echo  # taken into the running turn
        assert "tool_use" in events[echo:]  # before the second tool call, not at the turn's end
