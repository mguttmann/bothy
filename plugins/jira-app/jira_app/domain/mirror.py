# SPDX-License-Identifier: Apache-2.0
"""Mirroring and loop protection (brief M2-50 Design note 13; 11 10.1 rows "Scopes", "Loops").

`mirror_decision` decides at receipt of ``message.created``, in this order, each refusal with
its reason: the author is the app itself (``own_message``, the loop protection of 10.1);
``origin`` is not ``human`` (``not_human``: agent, app, import and system messages are never
mirrored, so two mirroring apps cannot feed each other and no agent output leaves the instance
without a person); ``kind`` is not ``text`` (``not_text``); ``deleted_at`` is set
(``deleted``); the thread is not mapped (``thread_not_mapped``).

`echo_of` names the Jira object an event reports, so the app can skip its own issues and
comments (``own_jira_object``): the chat already shows the original.

Examples:
    >>> mirror_decision({"author": {"id": "a"}, "origin": "human", "kind": "text",
    ...     "thread_id": "t"}, "a", lambda thread: True)
    'own_message'
"""

from __future__ import annotations

from collections.abc import Callable, Mapping
from typing import Any

from jira_app.jira.events import JiraEvent

__all__ = ["echo_of", "mirror_decision"]


def mirror_decision(
    message: Mapping[str, Any], own_principal_id: str, mapped: Callable[[str], bool]
) -> str | None:
    """Decide whether a delivered message is mirrored to Jira.

    Args:
        message: The ``payload.message`` of ``message.created``.
        own_principal_id: The app's principal ID from ``getMe``.
        mapped: Whether a thread belongs to an issue.

    Returns:
        ``None`` to mirror, else the reason of Design note 13.
    """
    author = message.get("author")
    author_id = author.get("id") if isinstance(author, Mapping) else None
    if isinstance(author_id, str) and author_id.lower() == own_principal_id.lower():
        return "own_message"
    if message.get("origin") != "human":
        return "not_human"
    if message.get("kind") != "text":
        return "not_text"
    if message.get("deleted_at") is not None:
        return "deleted"
    thread = message.get("thread_id")
    if not isinstance(thread, str) or not mapped(thread.lower()):
        return "thread_not_mapped"
    return None


def echo_of(event: JiraEvent) -> tuple[str, str] | None:
    """Return the Jira object an event announces, when the app may have created it.

    Args:
        event: A parsed event.

    Returns:
        ``("issue", <id>)`` for ``jira:issue_created``, ``("comment", <id>)`` for
        ``comment_created``, ``None`` for ``jira:issue_updated`` (always posted).
    """
    if event.event == "jira:issue_created":
        return ("issue", event.issue_id)
    if event.event == "comment_created":
        return ("comment", event.comment_id)
    return None
