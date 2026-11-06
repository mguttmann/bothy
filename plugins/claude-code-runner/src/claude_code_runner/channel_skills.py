# SPDX-License-Identifier: Apache-2.0
"""Channel skills behind ``runner.channel_skills_dir`` (docs/spec/26-builtin-agents.md 5.9).

The skills of ``channel_skill_bindings`` reach the runner as ``behavior.skills`` of a served
channel (``agent.config.read``). Only with ``runner.channel_skills_dir`` set does the runner read
them, and only a turn of class ``host`` loads one (R-BA-13): a reference
``claude-code:<name>`` with ``<name>`` matching ``^[a-z0-9-]{1,64}$`` resolves to the folder
``<runner.channel_skills_dir>/<name>/`` with a ``SKILL.md``, handed to the turn as one more
``--plugin-dir``. Every other reference belongs to another adapter and is skipped with a debug
log line only (a ``claude-code:`` reference whose name fails the pattern too). Below ``host``
each ``claude-code:`` reference is skipped and logged, because a channel skill folder is the
host operator's content that no definition check covers (T-BA-03); a missing folder is logged.
`ChannelSkills.notice` names both for the live box, once per run.

Examples:
    >>> resolve_channel_skills(["claude-code:review"], None, "host").plugin_dirs
    ()
"""

from __future__ import annotations

import json
import logging
import re
from collections.abc import Sequence
from dataclasses import dataclass
from pathlib import Path

__all__ = ["PREFIX", "ChannelSkills", "resolve_channel_skills"]

_LOG = logging.getLogger("claude_code_runner")

PREFIX = "claude-code:"
"""The adapter part of a skill reference this runner understands (26 5.9)."""

_NAME = re.compile(r"[a-z0-9-]{1,64}")


@dataclass(frozen=True)
class ChannelSkills:
    """The channel skills of one turn.

    Attributes:
        plugin_dirs: The folders to load, one ``--plugin-dir`` each (``host`` only).
        skipped: ``claude-code:`` references a turn below ``host`` did not load.
        missing: ``claude-code:`` references of a ``host`` turn without a folder.
    """

    plugin_dirs: tuple[Path, ...] = ()
    skipped: tuple[str, ...] = ()
    missing: tuple[str, ...] = ()

    def notice(self) -> str | None:
        """The status line of the live box that names what was not loaded, or ``None``.

        Returns:
            One line with the skipped and the missing references.
        """
        parts: list[str] = []
        if self.skipped:
            parts.append(
                "Channel skills need tool class host, so this turn skipped: "
                + ", ".join(self.skipped)
            )
        if self.missing:
            parts.append("Channel skill not found on the runner host: " + ", ".join(self.missing))
        return ". ".join(parts) or None


def _log(level: int, event: str, **ids: object) -> None:
    _LOG.log(level, json.dumps({"event": event, **{k: str(v) for k, v in ids.items()}}))


def resolve_channel_skills(
    refs: Sequence[str], skills_dir: Path | None, tool_class: str, **ids: object
) -> ChannelSkills:
    """Resolve a channel's skill references for one turn (26 5.9).

    Args:
        refs: ``behavior.skills`` of the channel, in their order.
        skills_dir: ``runner.channel_skills_dir``; ``None`` reads nothing.
        tool_class: The class whose flags the turn's command line carries.
        **ids: Run, agent and channel IDs for the log lines.

    Returns:
        The folders to load and the references to name.
    """
    if skills_dir is None:
        return ChannelSkills()
    dirs: list[Path] = []
    skipped: list[str] = []
    missing: list[str] = []
    for ref in refs:
        name = ref[len(PREFIX) :] if ref.startswith(PREFIX) else None
        if name is None or _NAME.fullmatch(name) is None:
            # Another adapter's reference, or one this runner cannot name: never content in a log.
            _log(logging.DEBUG, "channel_skill_foreign", **ids)
            continue
        if tool_class != "host":
            if ref not in skipped:
                skipped.append(ref)
                _log(logging.INFO, "channel_skill_skipped", skill=name, reason="needs_host", **ids)
            continue
        folder = skills_dir / name
        if not (folder / "SKILL.md").is_file():
            if ref not in missing:
                missing.append(ref)
                _log(logging.WARNING, "channel_skill_missing", skill=name, **ids)
            continue
        if folder not in dirs:
            dirs.append(folder)
    return ChannelSkills(tuple(dirs), tuple(skipped), tuple(missing))
