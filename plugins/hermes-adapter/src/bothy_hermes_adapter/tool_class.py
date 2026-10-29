# SPDX-License-Identifier: Apache-2.0
"""The actual tool class from the toolsets Hermes enables (09 2.4, 17.11; 07 2.11).

The class is the highest class of any enabled toolset (docs/DECISIONS.md D-M2-33-5): no toolset
gives ``chat``; a toolset in `TOOLSET_CLASSES` gives its class; any other name, also an unknown
one or the name of an MCP server, gives ``host``. Fail closed: a too high class only blocks starts
whose token carries a lower ``tool_class`` (09 2.4), a too low one would hide host access.

`TOOLSET_CLASSES` names every toolset of the pinned Hermes version with its class and reason;
``test_tool_class_table_covers_pinned_toolsets`` fails when a pin update brings a toolset the
table does not name. Only toolsets without any effect outside the conversation are ``chat``. A
``scoped`` toolset would need named tools with their own minimal credentials whose effect
outside the thread asks for an approval card; the pinned version has none, so the table names no
``scoped`` row and every toolset with an outside effect is ``host``.

Examples:
    >>> classify(frozenset())
    'chat'
    >>> classify(frozenset({"clarify", "todo"}))
    'chat'
    >>> classify(frozenset({"todo", "an-mcp-server"}))
    'host'
"""

from __future__ import annotations

from collections.abc import Mapping
from typing import Final

__all__ = ["RANK", "TOOLSET_CLASSES", "classify"]

RANK: Final = {"chat": 0, "scoped": 1, "host": 2}
"""Order of the tool classes (07 2.11)."""

_CONVERSATION = "chat: acts only inside the conversation"
_HOST = "host: reaches the host, the network or state outside the thread"
_COMPOSITE = "host: composite of host toolsets"

TOOLSET_CLASSES: Final[Mapping[str, tuple[str, str]]] = {
    "bot_room": ("host", "host: group chat turns of other bots"),
    "browser": ("host", _HOST),
    "catalog": ("host", "host: installs plugins and skills of the profile"),
    "clarify": ("chat", _CONVERSATION),
    "code_execution": ("host", _HOST),
    "coding": ("host", _COMPOSITE),
    "computer_use": ("host", _HOST),
    "connections": ("host", _HOST),
    "context_engine": ("host", "host: tools of a context engine the adapter cannot see"),
    "cronjob": ("host", "host: schedules jobs outside the thread"),
    "debugging": ("host", _COMPOSITE),
    "delegation": ("host", "host: subagents with their own tools"),
    "desktop_ui": ("host", _HOST),
    "discord": ("host", _HOST),
    "discord_admin": ("host", _HOST),
    "feishu_doc": ("host", _HOST),
    "feishu_drive": ("host", _HOST),
    "file": ("host", _HOST),
    "hermes-acp": ("host", _COMPOSITE),
    "hermes-api-server": ("host", _COMPOSITE),
    "hermes-bluebubbles": ("host", _COMPOSITE),
    "hermes-cli": ("host", _COMPOSITE),
    "hermes-cron": ("host", _COMPOSITE),
    "hermes-dingtalk": ("host", _COMPOSITE),
    "hermes-discord": ("host", _COMPOSITE),
    "hermes-email": ("host", _COMPOSITE),
    "hermes-feishu": ("host", _COMPOSITE),
    "hermes-gateway": ("host", _COMPOSITE),
    "hermes-matrix": ("host", _COMPOSITE),
    "hermes-mattermost": ("host", _COMPOSITE),
    "hermes-qqbot": ("host", _COMPOSITE),
    "hermes-signal": ("host", _COMPOSITE),
    "hermes-slack": ("host", _COMPOSITE),
    "hermes-sms": ("host", _COMPOSITE),
    "hermes-telegram": ("host", _COMPOSITE),
    "hermes-webhook": ("host", _COMPOSITE),
    "hermes-wecom": ("host", _COMPOSITE),
    "hermes-wecom-callback": ("host", _COMPOSITE),
    "hermes-weixin": ("host", _COMPOSITE),
    "hermes-whatsapp": ("host", _COMPOSITE),
    "hermes-yuanbao": ("host", _COMPOSITE),
    "image_gen": ("host", "host: sends prompts to an outside service"),
    "kanban": ("host", "host: shared boards outside the thread"),
    "memory": ("host", "host: memory shared across threads"),
    "onboarding": ("host", "host: first-run setup of the profile"),
    "project": ("host", _HOST),
    "safe": ("host", "host: composite of web, vision and image generation"),
    "search": ("host", "host: sends queries to an outside service"),
    "session_search": ("host", "host: reads other sessions"),
    "skills": ("host", "host: writes skills shared across threads"),
    "terminal": ("host", _HOST),
    "todo": ("chat", _CONVERSATION),
    "tts": ("host", "host: writes files and calls an outside service"),
    "video": ("host", _HOST),
    "video_gen": ("host", "host: sends prompts to an outside service"),
    "vision": ("host", "host: sends images to an outside service"),
    "web": ("host", "host: sends queries to an outside service"),
    "x_search": ("host", "host: sends queries to an outside service"),
    "yuanbao": ("host", _HOST),
}
"""Every toolset of the pinned Hermes version: name to (class, reason)."""


def classify(toolsets: frozenset[str]) -> str:
    """Return the tool class of a set of enabled toolsets.

    Args:
        toolsets: The toolset names Hermes enables for the platform.

    Returns:
        ``chat``, ``scoped`` or ``host``.
    """
    best = "chat"
    for name in toolsets:
        found = TOOLSET_CLASSES.get(name, ("host", "unknown"))[0]
        if RANK[found] > RANK[best]:
            best = found
    return best
