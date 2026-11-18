# SPDX-License-Identifier: Apache-2.0
r"""The hook that lets a background session message its own subagents only (09 18.2).

A background session of the runner may send ``SendMessage`` to an agent it launched with the
``Agent`` tool, so a running agent gets more information while it works. It must not reach
another Claude Code session of the host user, nor a workflow agent of its own (a message
there starts a second copy of that agent in the same folder). The pinned CLI has no setting
for that: the tool reaches the sessions of this machine, of Remote Control and of the cloud
by name or by a ``uds:`` or ``bridge:`` address. This file is the ``PreToolUse`` hook of the
run's settings file (`claude_code_runner.bgsession.settings_document`) that decides each call.

A call passes only when ``to`` is the ``agentId`` of an agent of this session, or ``main``,
and ``message`` is plain text without ``notify_when_idle``. The pinned CLI resolves a
recipient of the form of an ``agentId`` among the agents of the session and never among
other sessions (read in the file of the pinned CLI and run, D-FIX-RUNNER-SENDMESSAGE-OWN-1).
An agent of the session is one with a file ``subagents/agent-<id>.jsonl`` or
``subagents/agent-<id>.meta.json`` in the session's transcript folder; an id that also has a
file below ``subagents/workflows`` is a workflow agent and refused. A name is refused: the
CLI looks a name up among other sessions when no agent of the session has it.

Every other call, and every error, ends with exit status 2 and the reason on standard error,
which the CLI takes as a refusal of the call. The file uses the standard library only and is
run by its path (``python -I <file>``), so no import of the package can fail before the
decision; the command of the hook adds ``|| exit 2`` for an interpreter that does not start.

Examples:
    >>> decide({"tool_name": "SendMessage", "tool_input": {"to": "bothy-x", "message": "m"}})
    'the recipient is not an agent of this session'
"""

from __future__ import annotations

import json
import re
import sys
from collections.abc import Mapping
from pathlib import Path
from typing import Any

__all__ = ["AGENT_ID", "MAIN", "decide", "main"]

TOOL = "SendMessage"
MAIN = "main"
"""The name of the main conversation, for a subagent of the session (resolved first by the
pinned CLI, before any other session)."""

AGENT_ID = re.compile(r"a(?:[A-Za-z0-9_-]{1,63}-)?[0-9a-f]{16}")
r"""The form of an ``agentId`` of the pinned CLI (2.1.326: ``^a(?:[\w-]{1,63}-)?[0-9a-f]{16}$``)."""

_SESSION_ID = re.compile(r"[A-Za-z0-9._-]+")


def _own_agent(payload: Mapping[str, Any], agent_id: str) -> bool:
    """Whether ``agent_id`` is an agent of the hook's session that is no workflow agent."""
    transcript = payload.get("transcript_path")
    session_id = payload.get("session_id")
    if not isinstance(transcript, str) or not transcript:
        return False
    if not isinstance(session_id, str) or not _SESSION_ID.fullmatch(session_id):
        return False
    if session_id in {".", ".."}:
        return False
    subagents = Path(transcript).parent / session_id / "subagents"
    own = any(
        (subagents / f"agent-{agent_id}{suffix}").is_file() for suffix in (".jsonl", ".meta.json")
    )
    workflows = subagents / "workflows"
    in_workflow = workflows.is_dir() and any(
        (run / f"agent-{agent_id}.jsonl").exists() for run in workflows.iterdir()
    )
    return own and not in_workflow


def decide(payload: Mapping[str, Any]) -> str | None:
    """Return why a call is refused, or ``None`` for a call that passes.

    Args:
        payload: The hook input of the CLI (``tool_name``, ``tool_input``, ``session_id``,
            ``transcript_path``).

    Returns:
        The reason of the refusal, or ``None``.
    """
    if payload.get("tool_name") != TOOL:
        return None
    tool_input = payload.get("tool_input")
    if not isinstance(tool_input, Mapping):
        return "the call has no input"
    if not isinstance(tool_input.get("message"), str) or tool_input.get("notify_when_idle"):
        return "only a plain text message to an agent of this session is allowed"
    to = tool_input.get("to")
    own = to == MAIN or (
        isinstance(to, str) and AGENT_ID.fullmatch(to) is not None and _own_agent(payload, to)
    )
    return None if own else "the recipient is not an agent of this session"


def main() -> int:
    """Read the hook input from standard input and decide; exit status 2 refuses the call."""
    try:
        payload = json.loads(sys.stdin.read())
        reason = decide(payload) if isinstance(payload, Mapping) else "the hook input is no object"
    except Exception:  # every error refuses the call
        reason = "the hook could not read its input"
    if reason is None:
        return 0
    sys.stderr.write(
        f"Not sent: {reason}. A session of the Bothy runner sends SendMessage only to an agent"
        " it launched with the Agent tool (to: the agentId of the spawn result), never to"
        " another session or a workflow agent, and never by name.\n"
    )
    return 2


if __name__ == "__main__":
    sys.exit(main())
