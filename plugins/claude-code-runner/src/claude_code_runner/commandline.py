# SPDX-License-Identifier: Apache-2.0
"""The command line of one turn (docs/spec/09-agents.md 18.2, design note 9).

`build_argv` is a pure function from the run's choices to an exact argument list; no shell ever
sees it. `class_flags` returns exactly the cell of the class in the table "Class | Flags |
Effect" of 18.2, split as a shell would split it (R-AG-21, R-AG-23, D-W-16).

A turn of a hosted agent (docs/spec/26-builtin-agents.md 5.3) passes `HostedArgs`: its class
flags are `hosted_class_flags`, and the hosted additions stand directly after
``--permission-prompts none``, before ``--add-dir``. Channel skill folders (26 5.9) are one more
``--plugin-dir`` each, only in a ``host`` turn. Without either the list is that of 18.2.

Examples:
    >>> class_flags("host", [])
    ['--permission-mode', 'bypassPermissions']
"""

from __future__ import annotations

import re
import uuid
from collections.abc import Sequence
from dataclasses import dataclass
from decimal import Decimal
from pathlib import Path
from typing import Any

from bothy_sdk.generated.types import Unset

from claude_code_runner.config import check_argument

__all__ = [
    "SCOPED_BASELINE",
    "ArgumentError",
    "HostedArgs",
    "SessionChoice",
    "budget_argument",
    "build_argv",
    "choose_model",
    "choose_session",
    "class_flags",
    "hosted_class_flags",
    "reported_model",
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


SCOPED_BASELINE = ("Read", "Grep", "Glob", "Bash")
"""What 09 18.2 grants every ``scoped`` turn already (``Bash`` limited to read-only commands by
``dontAsk``): the start of ``--tools`` of a hosted ``scoped`` turn (26 5.3)."""


@dataclass(frozen=True)
class HostedArgs:
    """The parts of a hosted turn's command line (26 5.3).

    Attributes:
        settings: ``<control>/settings.json`` of ``--settings``.
        plugin: ``<control>/plugin`` of ``--plugin-dir``; ``None`` when there are no skills.
        mcp_config: The per-turn MCP file of ``--mcp-config``; ``None`` without a server.
        has_skills: The definition has skills, so ``Skill`` is a tool of the turn.
    """

    settings: Path
    plugin: Path | None
    mcp_config: Path | None
    has_skills: bool


def hosted_class_flags(tool_class: str, tools: Sequence[str], *, has_skills: bool) -> list[str]:
    """Return the tool flags of a hosted turn, exactly the column "Tool flags" of 26 5.3.

    ``dontAsk`` refuses a tool that is not allowed in advance, so ``Skill`` is named in
    ``--allowedTools`` wherever ``--tools`` carries it.

    Args:
        tool_class: ``chat``, ``scoped`` or ``host``.
        tools: The intersected tools of ``scoped`` (26 4.4 item 2), one argument each.
        has_skills: The definition has skills.

    Returns:
        The flags; ``host`` keeps those of 09 18.2.

    Raises:
        ArgumentError: ``unknown_tool_class`` or ``invalid_argument`` (see `class_flags`).
    """
    skill = ["Skill"] if has_skills else []
    if tool_class == "chat":
        named = ["--allowedTools", "Skill"] if has_skills else []
        return [
            "--permission-mode",
            "dontAsk",
            "--tools",
            "Skill" if has_skills else "",
            *named,
            "--disallowedTools",
            "mcp__*",
        ]
    if tool_class == "scoped":
        if not all(check_argument(t) for t in tools):
            raise ArgumentError("invalid_argument")
        available = [*SCOPED_BASELINE, *skill]
        available += [t for t in tools if t not in available]
        allowed = [*tools, *skill]
        named = ["--allowedTools", *allowed] if allowed else []
        return ["--permission-mode", "dontAsk", "--tools", *available, *named]
    return class_flags(tool_class, tools)


def _hosted_additions(
    tool_class: str, hosted: HostedArgs, plugin_dirs: Sequence[Path]
) -> list[str]:
    """The column "Added directly after ``--permission-prompts none``" of 26 5.3."""
    if tool_class == "host":
        added = ["--setting-sources", "project,local", "--settings", str(hosted.settings)]
    else:
        added = ["--restricted", "--settings", str(hosted.settings)]
    if hosted.plugin is not None:
        added += ["--plugin-dir", str(hosted.plugin)]
    for folder in plugin_dirs:
        added += ["--plugin-dir", str(folder)]
    if hosted.mcp_config is not None:
        if tool_class == "chat":  # 26 4.4 item 3: never an MCP server under chat
            raise ArgumentError("invalid_argument")
        added += ["--mcp-config", str(hosted.mcp_config)]
    # --mcp-config takes several values, so a flag always follows it; --restricted alone would
    # still start the host user's MCP servers (help text of the pinned CLI).
    return [*added, "--strict-mcp-config"]


def budget_argument(value: float) -> str:
    """The value of ``--max-budget-usd``: the cost cap as a plain decimal number (26 5.8).

    Args:
        value: ``settings.max_cost_usd_per_run``.

    Returns:
        The decimal without an exponent.

    Examples:
        >>> budget_argument(2.5), budget_argument(1e-07)
        ('2.5', '0.0000001')
    """
    return format(Decimal(repr(value)), "f")


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


_MODEL_NAME = re.compile(r"^[A-Za-z0-9._:/@-]+$")
"""The pattern of a model name the server takes (09 2.2 ``default_model``, ``run.start``)."""
_CLI_SUFFIX = re.compile(r"\[[^\[\]]*\]$")
"""A trailing selector of the CLI notation, such as the context window in ``<model>[1m]``."""


def reported_model(model: str) -> str | None:
    """Return the model name the runner reports for a CLI model value (D-RUNNER-CLI-PIN-3).

    The CLI takes ``<model>[<selector>]`` (for example a context window, ``[1m]``) and sends
    the model without it; the server's model name (09 2.2) has no ``[``. The suffix is cut; a
    value that still is no model name is left out of ``startRun`` and ``agent_session.report``,
    so the run goes on without it.

    Args:
        model: The value of `choose_model` (passed to ``--model`` unchanged).

    Returns:
        The model name, or ``None``.

    Examples:
        >>> reported_model("claude-opus-5-5[1m]")
        'claude-opus-5-5'
    """
    name = _CLI_SUFFIX.sub("", model)
    return name if _MODEL_NAME.fullmatch(name) else None


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
    hosted: HostedArgs | None = None,
    plugin_dirs: Sequence[Path] = (),
    max_budget_usd: str | None = None,
) -> list[str]:
    """Build the argument list of one turn in the order of 18.2 (and 26 5.3 for a hosted one).

    Args:
        claude_bin: The absolute path of the CLI found in the cleaned ``PATH``.
        model: The model (`choose_model`).
        session: The session (`choose_session`).
        tool_class: ``runner.tool_class``, or the loaded class of a hosted agent.
        tools: The tools of ``scoped`` (`scoped_tools`).
        add_dir: The thread's incoming folder, only for ``scoped`` and ``host`` with files.
        system_prompt_file: The file of ``--append-system-prompt-file``.
        hosted: The parts of a hosted turn (26 5.3); ``None`` for the runner's own agent.
        plugin_dirs: Channel skill folders (26 5.9), only for ``host``.
        max_budget_usd: The cost cap of a hosted definition (`budget_argument`), if any.

    Returns:
        The argument list.

    Raises:
        ArgumentError: An invalid model or tool, ``add_dir`` or an MCP file under ``chat``, or
            a channel skill folder below ``host``.
    """
    if not check_argument(model):
        raise ArgumentError("invalid_argument")
    if add_dir is not None and tool_class == "chat":
        raise ArgumentError("invalid_argument")
    if plugin_dirs and tool_class != "host":  # 26 5.9: channel skills only in host turns
        raise ArgumentError("invalid_argument")
    if hosted is None:
        flags = class_flags(tool_class, tools)
        added = [a for folder in plugin_dirs for a in ("--plugin-dir", str(folder))]
    else:
        flags = hosted_class_flags(tool_class, tools, has_skills=hosted.has_skills)
        added = _hosted_additions(tool_class, hosted, plugin_dirs)
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
        *flags,
        "--permission-prompts",
        "none",
        *added,
    ]
    if add_dir is not None:
        argv += ["--add-dir", str(add_dir)]
    if max_budget_usd is not None:
        argv += ["--max-budget-usd", max_budget_usd]
    argv += ["--append-system-prompt-file", str(system_prompt_file)]
    return argv
