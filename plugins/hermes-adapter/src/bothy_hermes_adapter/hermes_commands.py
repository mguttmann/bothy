# SPDX-License-Identifier: Apache-2.0
"""Hermes commands as the agent's commands on the server (09 10, 17.3 row "Hermes commands").

Hermes offers slash commands on its other platforms (``hermes_cli.commands.COMMAND_REGISTRY``)
and one command per skill. The adapter maps them onto the server's command contract:

* The Hermes commands that the server already has as built-in commands (09 10.1) stay the
  server's: ``/stop`` cancels the run, ``/new`` and ``/resume`` change the session record,
  ``/title`` sets the thread title, ``/status`` opens the run details. The adapter carries them
  out through its own session and stop paths (17.3, 17.4); it never registers them.
* `COMMAND_CLASSES` names every gateway command of the pinned Hermes version with its class and
  reason: ``control`` commands are registered as control commands (``control = true``) and sent
  to Hermes in the thread from the adapter's control sender; Hermes' reply becomes a message of
  the agent in the thread. Every other command is ``refused``: it calls the model outside a run,
  changes the thread's session outside the server's session record, bypasses the server's
  follow-up, approval or card rules, reaches the host, or acts on the whole Hermes profile
  rather than one thread. ``test_command_table_covers_pinned_commands`` fails when a pin update
  brings a command the table does not name; commands of other Hermes plugins are never offered
  (their effect is unknown, fail closed).
* Every skill command Hermes offers for this platform becomes a model command (``control =
  false``) with one optional text option `INPUT_OPTION`: the server opens a run, and the turn
  is Hermes' own skill invocation message for that skill and text (`skill_specs`).

Examples:
    >>> [spec.name for spec in control_specs()]
    ['context', 'version']
    >>> command_text("context", {})
    '/context'
    >>> sorted(BUILT_IN)
    ['close', 'new', 'resume', 'status', 'stop', 'title']
"""

from __future__ import annotations

import logging
import re
import unicodedata
from collections.abc import Iterable, Mapping, Sequence
from typing import Final

from bothy_sdk.commands import CommandSpec
from bothy_sdk.generated.models import CommandOption, CommandOptionType

__all__ = [
    "BUILT_IN",
    "COMMAND_CLASSES",
    "CONTROL_DESCRIPTIONS",
    "INPUT_OPTION",
    "command_text",
    "control_specs",
    "declared_commands",
    "option_values",
    "skill_specs",
]

_LOG = logging.getLogger("bothy_hermes_adapter")

BUILT_IN: Final = frozenset({"stop", "status", "new", "resume", "title", "close"})
"""The server's built-in commands (09 10.1); ``registerCommand`` refuses these names."""

INPUT_OPTION: Final = "input"
"""The one option of a skill command: the text Hermes passes to the skill."""

DESCRIPTION_MAX: Final = 100
"""``command.description_max_chars`` (09 21); the SDK checks the same limit before a request."""

MAX_COMMANDS: Final = 100
"""``command.max_per_principal`` (09 21): the declared list never exceeds it."""

_NAME: Final = re.compile(r"[a-z0-9][a-z0-9_-]*")

_SERVER = "builtin: the server's own command (09 10.1); the adapter carries it out"
_MODEL = "refused: calls the model outside a run"
_SESSION = "refused: changes the thread's session outside the server's session record (09 9)"
_FOLLOWUP = "refused: a follow-up goes through the server (09 7)"
_APPROVAL = "refused: approvals go through cards (09 11.5)"
_HOST = "refused: reads or writes the host (files, git, logs, network)"
_PROFILE = "refused: acts on the whole Hermes profile, not on this thread"
_LISTING = "refused: lists Hermes commands this platform does not offer; the client lists them"
_PLATFORM = "refused: a feature of another platform"
_CONTROL = "control: reads the thread's Hermes session or Hermes itself, no model call"

COMMAND_CLASSES: Final[Mapping[str, tuple[str, str]]] = {
    "agents": ("refused", _PROFILE),
    "approvals": ("refused", _APPROVAL),
    "approve": ("refused", _APPROVAL),
    "autoprune": ("refused", _PROFILE),
    "bg": ("refused", _MODEL),
    "blueprint": ("refused", _PROFILE),
    "branch": ("refused", _SESSION),
    "btw": ("refused", _MODEL),
    "bundles": ("refused", _PROFILE),
    "busy": ("refused", _FOLLOWUP),
    "codex-runtime": ("refused", _PROFILE),
    "commands": ("refused", _LISTING),
    "compress": ("refused", _MODEL),
    "context": ("control", _CONTROL),
    "curator": ("refused", _PROFILE),
    "debug": ("refused", _HOST),
    "deny": ("refused", _APPROVAL),
    "diff": ("refused", _HOST),
    "discard": ("refused", _SESSION),
    "egress": ("refused", _HOST),
    "fast": ("refused", _PROFILE),
    "footer": ("refused", _PROFILE),
    "goal": ("refused", _MODEL),
    "heartbeat": ("refused", _MODEL),
    "help": ("refused", _LISTING),
    "init": ("refused", _MODEL),
    "insights": ("refused", _PROFILE),
    "kanban": ("refused", _PROFILE),
    "learn": ("refused", _MODEL),
    "login": ("refused", _PROFILE),
    "loop": ("refused", _MODEL),
    "memory": ("refused", _PROFILE),
    "moa": ("refused", _MODEL),
    "model": ("refused", _PROFILE),
    "new": ("builtin", _SERVER),
    "pause": ("refused", _PROFILE),
    "personality": ("refused", _PROFILE),
    "plan": ("refused", _MODEL),
    "platform": ("refused", _PROFILE),
    "profile": ("refused", _HOST),
    "queue": ("refused", _FOLLOWUP),
    "reasoning": ("refused", _PROFILE),
    "refine": ("refused", _MODEL),
    "reload-mcp": ("refused", _PROFILE),
    "reload-skills": ("refused", _PROFILE),
    "restart": ("refused", _PROFILE),
    "resume": ("builtin", _SERVER),
    "retry": ("refused", _MODEL),
    "review": ("refused", _MODEL),
    "rollback": ("refused", _HOST),
    "save": ("refused", _HOST),
    "sessions": ("refused", _SESSION),
    "sethome": ("refused", _PROFILE),
    "skills": ("refused", _PROFILE),
    "start": ("refused", _PLATFORM),
    "status": ("builtin", _SERVER),
    "steer": ("refused", _FOLLOWUP),
    "stop": ("builtin", _SERVER),
    "subgoal": ("refused", _MODEL),
    "suggestions": ("refused", _PROFILE),
    "title": ("builtin", _SERVER),
    "topic": ("refused", _PLATFORM),
    "topup": ("refused", _PROFILE),
    "undo": ("refused", _MODEL),
    "update": ("refused", _PROFILE),
    "usage": ("refused", _PROFILE),
    "verbose": ("refused", _PROFILE),
    "version": ("control", _CONTROL),
    "voice": ("refused", _PROFILE),
    "whoami": ("refused", _PROFILE),
    "yolo": ("refused", _APPROVAL),
}
"""Every gateway command of the pinned Hermes version: class and reason (fail closed)."""

CONTROL_DESCRIPTIONS: Final[Mapping[str, str]] = {
    "context": "Show the context window of this thread's Hermes session",
    "version": "Show the Hermes Agent version",
}
"""The descriptions of the control commands (English; Hermes' own are localized)."""


def control_specs() -> list[CommandSpec]:
    """The control commands of `COMMAND_CLASSES`, sorted by name.

    Returns:
        One instance-wide control command per ``control`` row.
    """
    names = sorted(n for n, (kind, _) in COMMAND_CLASSES.items() if kind == "control")
    return [CommandSpec(n, CONTROL_DESCRIPTIONS[n], control=True) for n in names]


def _description(raw: str, name: str) -> str:
    """A description the server accepts: one line, no control or bidi characters, cut."""
    kept = "".join(
        " " if unicodedata.category(ch) in ("Cc", "Cf", "Zl", "Zp") else ch for ch in raw
    )
    text = " ".join(kept.split())
    if not text:
        text = f"Run the {name} skill"
    if len(text) > DESCRIPTION_MAX:
        text = text[: DESCRIPTION_MAX - 3].rstrip() + "..."
    return text


def skill_specs(skills: Mapping[str, str], taken: Iterable[str] = ()) -> list[CommandSpec]:
    """Model commands for Hermes' skill commands, sorted by name.

    A skill whose name is no valid command name, a built-in name or a name in ``taken`` is left
    out (logged with its count only, never the name).

    Args:
        skills: Skill command names (without ``/``) to their descriptions.
        taken: Names already declared (the control commands).

    Returns:
        One instance-wide model command per skill with the option `INPUT_OPTION`.
    """
    blocked = set(BUILT_IN) | set(taken) | set(COMMAND_CLASSES)
    option = CommandOption(
        name=INPUT_OPTION,
        description="What the skill should work on",
        type_=CommandOptionType.STRING,
        required=False,
    )
    specs = []
    skipped = 0
    for name in sorted(skills):
        if not _NAME.fullmatch(name) or name in blocked:
            skipped += 1
            continue
        specs.append(CommandSpec(name, _description(skills[name], name), options=(option,)))
    if skipped:
        _LOG.info("skill commands not offered: %d (name not usable on the server)", skipped)
    return specs


def declared_commands(skills: Mapping[str, str]) -> list[CommandSpec]:
    """The adapter's complete command list: control commands, then skill commands.

    Args:
        skills: Hermes' skill commands for this platform (name to description).

    Returns:
        At most `MAX_COMMANDS` commands; skills beyond the limit are left out (logged).
    """
    control = control_specs()
    declared = control + skill_specs(skills, (s.name for s in control))
    if len(declared) > MAX_COMMANDS:
        _LOG.warning("commands beyond the limit not offered: %d", len(declared) - MAX_COMMANDS)
        declared = declared[:MAX_COMMANDS]
    return declared


def option_values(options: Sequence[object] | object) -> dict[str, str]:
    """The option values of a ``command.invoked`` or ``agent.invocation`` payload by name.

    Args:
        options: The payload's ``options`` (a list of ``CommandOptionValue``, or unset).

    Returns:
        Name to value; an unset or malformed list gives an empty mapping.
    """
    if not isinstance(options, list | tuple):
        return {}
    values = {}
    for item in options:
        name, value = getattr(item, "name", None), getattr(item, "value", None)
        if isinstance(name, str) and isinstance(value, str):
            values[name] = value
    return values


def command_text(name: str, options: Mapping[str, str]) -> str:
    """The Hermes command line of a control command: ``/<name>``, never an option value.

    The control commands take no options; a value the server passed anyway is not forwarded,
    so no person's text ever becomes part of a Hermes command line.

    Args:
        name: A control command of `COMMAND_CLASSES`.
        options: The option values (ignored).

    Returns:
        The command line.
    """
    del options
    return f"/{name}"
