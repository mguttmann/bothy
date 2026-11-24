# SPDX-License-Identifier: Apache-2.0
"""An owner's session command typed as a message (09 17.3, FIX-hermes-session-commands).

Hermes pauses a thread's session when a turn was lost in a restart and tells the person to send
``/discard`` or ``/reset``. A person's text reaches Hermes with ``allow_gateway_control`` off
(17.4), so such a text was never a command. `owner_session_command` names the one case in which
the adapter sends it as a command instead of a turn, from its control sender: a run of
``trigger_kind = message`` whose triggering message is exactly one of `SESSION_COMMANDS`, written
by the run's person (``origin = human``, author of kind ``user`` equal to the token's ``sub``),
without attachments, on an agent whose token carries ``tool_class = host``. For that class the
server lets only an active owner trigger the agent (07 SR2, checked at acceptance and before every
delivery; the server sets no ready dev environment yet), so the signed token proves the owner.
Every other run keeps the text as plain text: a team input (a member's reply or an owner's copy),
a follow-up, an agent's message, a command run and a run of an agent of another tool class.

Examples:
    >>> command_line("  /Discard ")
    '/discard'
    >>> command_line("/discard now") is None
    True
"""

from __future__ import annotations

import re
from typing import Any, Final

__all__ = [
    "CONFIRMED",
    "NOT_ANSWERED",
    "SESSION_COMMANDS",
    "command_line",
    "confirmed",
    "owner_session_command",
]

SESSION_COMMANDS: Final = frozenset({"discard", "new", "reset", "stop"})
"""Hermes' gateway commands an owner may type as a message (``reset``: Hermes' alias of ``new``)."""

CONFIRMED: Final = frozenset({"new", "reset"})
"""The commands Hermes confirms with ``send_slash_confirm``; the adapter answers "once"."""

NOT_ANSWERED: Final = (
    "Hermes saved this message but did not answer it: its conversation in this thread is paused "
    "or waiting. If Hermes said that a previous turn did not finish when it restarted, an owner "
    "can start a fresh conversation with /new, or, on an agent with host tools, send /discard "
    "to drop that turn and continue."
)
"""The run's answer when Hermes ran no turn and sent nothing for the run's message (09 17.6)."""

_LINE: Final = re.compile(r"/([a-z]+)", re.IGNORECASE)


def command_line(text: str) -> str | None:
    """The Hermes command line of a text that is exactly a session command, else ``None``."""
    match = _LINE.fullmatch(text.strip())
    if match is None or match.group(1).lower() not in SESSION_COMMANDS:
        return None
    return f"/{match.group(1).lower()}"


def _value(raw: Any) -> str:
    return str(getattr(raw, "value", raw) or "")


def owner_session_command(claims: Any, trigger: Any) -> str | None:
    """The command line to send for a run, or ``None`` when its text stays a turn.

    Args:
        claims: The verified claims of the run's ``agent.invocation`` token.
        trigger: The run context's ``trigger_message``.

    Returns:
        ``/discard``, ``/new``, ``/reset`` or ``/stop`` only for an owner's own message.
    """
    if getattr(claims, "trigger_kind", None) != "message":
        return None
    if getattr(claims, "tool_class", None) != "host":
        return None
    if trigger is None or _value(getattr(trigger, "origin", None)) != "human":
        return None
    author = getattr(trigger, "author", None)
    if _value(getattr(author, "kind", None)) != "user":
        return None
    if getattr(author, "id", None) != getattr(claims, "sub", None):
        return None
    if getattr(trigger, "attachments", None):
        return None
    text = getattr(trigger, "content_md", None)
    return command_line(text) if isinstance(text, str) else None


def confirmed(line: str) -> bool:
    """Whether Hermes asks to confirm the command line (the adapter confirms it at once)."""
    return line.lstrip("/") in CONFIRMED
