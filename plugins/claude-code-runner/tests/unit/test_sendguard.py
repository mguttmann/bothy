# SPDX-License-Identifier: Apache-2.0
"""The hook that lets a background session message its own subagents only (09 18.2).

The form of an ``agentId``, the layout of a session's transcript folder and the fields of the
hook input are those of the pinned CLI 2.1.326 (run against the stand-in on 2026-11-18,
D-FIX-RUNNER-SENDMESSAGE-OWN-1); ``tests/real_cli/test_send_guard.py`` holds the same hook
against that CLI.
"""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path
from typing import Any

import pytest

from claude_code_runner import sendguard
from claude_code_runner.bgsession import send_guard_command
from claude_code_runner.sendguard import decide

SESSION = "8fed0fdb-9064-429f-aeef-39cd6648d564"
OWN = "ae19ed16ca1560db8"
"""An agentId as the pinned CLI gives it in the result of the ``Agent`` tool."""
LABELLED = "aworker-0123456789abcdef"
WORKFLOW = "a0123456789abcdef"


@pytest.fixture
def transcript(tmp_path: Path) -> Path:
    """The transcript of a session with one agent, one labelled agent and a workflow agent."""
    folder = tmp_path / "projects" / "-work"
    subagents = folder / SESSION / "subagents"
    (subagents / "workflows" / "run1").mkdir(parents=True)
    (subagents / f"agent-{OWN}.jsonl").write_text("{}\n", "utf-8")
    (subagents / f"agent-{LABELLED}.meta.json").write_text("{}", "utf-8")
    (subagents / "workflows" / "run1" / f"agent-{WORKFLOW}.jsonl").write_text("{}\n", "utf-8")
    path = folder / f"{SESSION}.jsonl"
    path.write_text("{}\n", "utf-8")
    return path


def payload(transcript: Path, to: object, **more: Any) -> dict[str, Any]:
    tool_input = {"to": to, "message": "extra info", "summary": "extra", **more}
    return {
        "session_id": SESSION,
        "transcript_path": str(transcript),
        "hook_event_name": "PreToolUse",
        "tool_name": "SendMessage",
        "tool_input": tool_input,
    }


def test_a_message_to_an_agent_of_the_session_passes(transcript: Path) -> None:
    assert decide(payload(transcript, OWN)) is None
    # An agent whose transcript has not been written yet is known by its meta file.
    assert decide(payload(transcript, LABELLED)) is None
    # A subagent of the session writes to its main conversation.
    assert decide(payload(transcript, "main")) is None


def test_every_recipient_outside_the_session_is_refused(transcript: Path) -> None:
    # A name: the pinned CLI looks it up among the other sessions of the host user when no
    # agent of the session has it, and a session named like that took the message (run).
    for to in ("helper", "bothy-peer", "bothy-0192d3a0", OWN.upper(), f" {OWN}", f"agent-{OWN}"):
        assert decide(payload(transcript, to)) is not None, to
    # The addresses of a session of this machine and of Remote Control.
    for to in ("uds:/tmp/claude/peer.sock", "bridge:session_01", "local_0192d3a0-0000"):
        assert decide(payload(transcript, to)) is not None, to
    # The form of an agentId without an agent of the session, a path and no text at all.
    for to in ("a1111111111111111", f"../{OWN}", f"{OWN}/x", "", None, 7):
        assert decide(payload(transcript, to)) is not None, to


def test_a_workflow_agent_of_the_session_is_refused(transcript: Path) -> None:
    # A message there starts a second copy of that agent in the same folder.
    assert decide(payload(transcript, WORKFLOW)) is not None
    # Also when a file of the same id stands beside the plain agents.
    subagents = transcript.parent / SESSION / "subagents"
    (subagents / f"agent-{WORKFLOW}.jsonl").write_text("{}\n", "utf-8")
    assert decide(payload(transcript, WORKFLOW)) is not None


def test_only_plain_text_without_an_idle_subscription_passes(transcript: Path) -> None:
    assert decide(payload(transcript, OWN, message={"type": "shutdown_response"})) is not None
    assert decide(payload(transcript, OWN, notify_when_idle=True)) is not None
    without = payload(transcript, OWN)
    del without["tool_input"]["message"]
    assert decide(without) is not None


def test_an_input_without_the_session_s_place_is_refused(transcript: Path) -> None:
    for change in (
        {"transcript_path": ""},
        {"transcript_path": None},
        {"session_id": ".."},
        {"session_id": "a/b"},
        {"session_id": None},
        {"session_id": "other-session"},
        {"tool_input": "to"},
    ):
        assert decide({**payload(transcript, OWN), **change}) is not None, change


def test_another_tool_is_not_decided(transcript: Path) -> None:
    assert decide({**payload(transcript, "bothy-peer"), "tool_name": "Bash"}) is None


def run_guard(stdin: str) -> subprocess.CompletedProcess[str]:
    """Run the file as the hook does: by its path, isolated from the package."""
    return subprocess.run(
        [sys.executable, "-I", sendguard.__file__],
        input=stdin,
        capture_output=True,
        text=True,
        timeout=30,
        check=False,
    )


def test_the_file_refuses_with_exit_status_two_and_a_reason(transcript: Path) -> None:
    passed = run_guard(json.dumps(payload(transcript, OWN)))
    assert (passed.returncode, passed.stdout, passed.stderr) == (0, "", "")
    refused = run_guard(json.dumps(payload(transcript, "bothy-peer")))
    assert refused.returncode == 2
    assert "not an agent of this session" in refused.stderr
    assert "agentId" in refused.stderr
    # Any input the file cannot read refuses the call too.
    for broken in ("", "{not json", "[]", "null"):
        assert run_guard(broken).returncode == 2, broken


def test_the_command_refuses_when_the_interpreter_does_not_start(transcript: Path) -> None:
    # The CLI takes any exit status but 2 of a failed hook as no decision and runs the tool
    # (run with the pinned CLI), so the command ends with ``|| exit 2``.
    command = send_guard_command(sys.executable)
    assert command.endswith(" || exit 2")
    assert f" -I {sendguard.__file__} " in command or f"-I '{sendguard.__file__}'" in command
    stdin = json.dumps(payload(transcript, OWN))
    ok = subprocess.run(["/bin/sh", "-c", command], input=stdin, text=True, check=False)
    assert ok.returncode == 0
    broken = send_guard_command("/nonexistent/python3")
    failed = subprocess.run(
        ["/bin/sh", "-c", broken], input=stdin, capture_output=True, text=True, check=False
    )
    assert failed.returncode == 2
