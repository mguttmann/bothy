# SPDX-License-Identifier: Apache-2.0
"""The command line of one turn (docs/spec/09-agenten.md 18.2, design note 9).

`build_argv` is a pure function from the run's choices to an exact argument list; no shell ever
sees it. `class_flags` returns exactly the cell of the class in the table "Class | Flags |
Effect" of 18.2, split as a shell would split it (R-AG-21, R-AG-23, D-W-16).

Examples:
    >>> class_flags("host", [])
    ['--permission-mode', 'bypassPermissions']
"""

from __future__ import annotations

import uuid
from collections.abc import Sequence
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from bothy_sdk.generated.types import Unset

from claude_code_runner.config import check_argument

__all__ = [
    "ArgumentError",
    "SessionChoice",
    "build_argv",
    "choose_model",
    "choose_session",
    "class_flags",
    "scoped_tools",
]


class ArgumentError(Exception):
    """A value that must not become a process argument, or an impossible combination.

    Attributes:
        code: ``invalid_argument``, ``session_ref_not_uuid`` or ``unknown_tool_class``.
    """

    def __init__(self, code: str) -> None:
        """Create the error with its code; the value is never part of the message."""
        super().__init__(code)
        self.code = code


@dataclass(frozen=True)
class SessionChoice:
    """The Claude Code session of a turn (18.1, 9.1).

    Attributes:
        ref: The session UUID (``external_session_ref``).
        resume: True for ``--resume`` (the session has started), else ``--session-id``.
        new: True when the payload had no session and the runner made the UUID now.
    """

    ref: str
    resume: bool
    new: bool


def class_flags(tool_class: str, tools: Sequence[str]) -> list[str]:
    """Return the flags of a tool class, exactly the cell of the 18.2 table.

    Args:
        tool_class: ``chat``, ``scoped`` or ``host``.
        tools: The named tools of ``scoped``, one argument each; ignored for the other classes.

    Returns:
        The flags; for ``scoped`` without tools ``--allowedTools`` is left out.

    Raises:
        ArgumentError: ``unknown_tool_class``, or ``invalid_argument`` for a tool value that
            fails `claude_code_runner.config.check_argument`.
    """
    if tool_class == "chat":
        return ["--permission-mode", "dontAsk", "--tools", "", "--disallowedTools", "mcp__*"]
    if tool_class == "scoped":
        if not all(check_argument(t) for t in tools):
            raise ArgumentError("invalid_argument")
        named = ["--allowedTools", *tools] if tools else []
        return ["--permission-mode", "dontAsk", *named]
    if tool_class == "host":
        return ["--permission-mode", "bypassPermissions"]
    raise ArgumentError("unknown_tool_class")


def scoped_tools(configured: Sequence[str], binding_allowed: Sequence[str] | None) -> list[str]:
    """Return the tools of ``scoped``: the binding narrows the runner key, never widens it.

    Args:
        configured: ``runner.scoped_allowed_tools`` in its order.
        binding_allowed: ``runner_config.allowed_tools`` of the binding, ``None`` when absent.

    Returns:
        The configured tools that the binding also allows, in the order of the runner key.
    """
    if binding_allowed is None:
        return list(configured)
    allowed = set(binding_allowed)
    return [t for t in configured if t in allowed]


def choose_model(binding_model: str | None, default_model: str) -> str:
    """Return the model: the binding's ``runner_config.model``, else ``runner.default_model``.

    Args:
        binding_model: The binding's model, if any.
        default_model: ``runner.default_model``.

    Returns:
        The model name.

    Raises:
        ArgumentError: ``invalid_argument`` when the chosen value cannot be an argument.
    """
    model = binding_model if binding_model is not None else default_model
    if not check_argument(model):
        raise ArgumentError("invalid_argument")
    return model


def choose_session(agent_session: Any) -> SessionChoice:
    """Choose ``--session-id`` or ``--resume`` from the payload's ``agent_session``.

    Only ``started`` decides, because ``resume`` and ``bind`` both mean ``--resume`` for this
    runtime (09 9.5 point 2).

    Args:
        agent_session: The generated ``AgentSession`` of the payload, or ``None``/``UNSET``.

    Returns:
        The choice.

    Raises:
        ArgumentError: ``session_ref_not_uuid`` for a reference that is not a UUID.
    """
    if agent_session is None or isinstance(agent_session, Unset):
        return SessionChoice(ref=str(uuid.uuid4()), resume=False, new=True)
    ref = getattr(agent_session, "external_session_ref", None)
    if not isinstance(ref, str):
        return SessionChoice(ref=str(uuid.uuid4()), resume=False, new=True)
    try:
        canonical = str(uuid.UUID(ref))
    except ValueError:
        raise ArgumentError("session_ref_not_uuid") from None
    if canonical != ref.lower():
        raise ArgumentError("session_ref_not_uuid")
    return SessionChoice(ref=ref, resume=bool(agent_session.started is True), new=False)


def build_argv(  # noqa: PLR0913 - every input of the command line is explicit
    *,
    claude_bin: str,
    model: str,
    session: SessionChoice,
    tool_class: str,
    tools: Sequence[str],
    add_dir: Path | None,
    system_prompt_file: Path,
) -> list[str]:
    """Build the argument list of one turn in the order of 18.2.

    Args:
        claude_bin: The absolute path of the CLI found in the cleaned ``PATH``.
        model: The model (`choose_model`).
        session: The session (`choose_session`).
        tool_class: ``runner.tool_class``.
        tools: The tools of ``scoped`` (`scoped_tools`).
        add_dir: The thread's incoming folder, only for ``scoped`` and ``host`` with files.
        system_prompt_file: The file of ``--append-system-prompt-file``.

    Returns:
        The argument list.

    Raises:
        ArgumentError: An invalid model or tool, or ``add_dir`` under ``chat``.
    """
    if not check_argument(model):
        raise ArgumentError("invalid_argument")
    if add_dir is not None and tool_class == "chat":
        raise ArgumentError("invalid_argument")
    session_flag = "--resume" if session.resume else "--session-id"
    argv = [
        claude_bin,
        "-p",
        "--input-format",
        "stream-json",
        "--output-format",
        "stream-json",
        "--verbose",
        "--replay-user-messages",
        "--model",
        model,
        session_flag,
        session.ref,
        *class_flags(tool_class, tools),
        "--permission-prompts",
        "none",
    ]
    if add_dir is not None:
        argv += ["--add-dir", str(add_dir)]
    argv += ["--append-system-prompt-file", str(system_prompt_file)]
    return argv
