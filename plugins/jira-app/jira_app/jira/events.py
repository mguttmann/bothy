# SPDX-License-Identifier: Apache-2.0
"""The Jira events the app posts (brief M2-50 Design note 6; a closed list).

After a valid signature the body must be a JSON object with a string member ``webhookEvent``.
Supported: ``jira:issue_created``, ``jira:issue_updated`` and ``comment_created``; every other
value is ignored with reason ``unsupported``. The project is ``issue.fields.project.key``,
compared exactly with the configured keys (``project_not_configured``). ``jira:issue_updated``
without changelog items is ignored with ``no_change``, because only field changes are news.

The parser keeps only the fields used; a missing or malformed ``issue.id``, ``issue.key``,
project key or, for comments, ``comment.id`` is ``invalid_payload``. Unknown members are
ignored. The job stores the parsed `JiraEvent`, never the raw payload.

Examples:
    >>> parsed = parse_event({"webhookEvent": "jira:issue_deleted"}, {"SUP"})
    >>> parsed.reason
    'unsupported'
"""

from __future__ import annotations

import re
from collections.abc import Collection, Mapping
from dataclasses import asdict, dataclass
from typing import Any

from jira_app.jira.adf import read_text

__all__ = [
    "ISSUE_KEY",
    "SUPPORTED_EVENTS",
    "Change",
    "Ignored",
    "Invalid",
    "JiraEvent",
    "parse_event",
]

SUPPORTED_EVENTS = ("jira:issue_created", "jira:issue_updated", "comment_created")
"""The events of docs/spec/11-extensions.md 10.2 the app posts."""

ISSUE_KEY = re.compile(r"^[A-Z][A-Z0-9_]*-[1-9][0-9]*$")
"""An issue key; only such a key is ever linked (Design note 11)."""

_DIGITS = re.compile(r"^[0-9]+$")
_SOMEONE = "someone"


@dataclass(frozen=True)
class Change:
    """One item of ``changelog.items``.

    Attributes:
        field: The changed field.
        from_text: ``fromString`` (empty when absent).
        to_text: ``toString`` (empty when absent).
    """

    field: str
    from_text: str
    to_text: str


@dataclass(frozen=True)
class JiraEvent:
    """A supported event, reduced to the fields the app uses.

    Attributes:
        event: ``webhookEvent``.
        issue_id: ``issue.id`` (decimal digits), the key of the thread map.
        issue_key: ``issue.key`` at this event.
        project: ``issue.fields.project.key``.
        summary: ``issue.fields.summary`` (empty when absent).
        actor: The display name of the actor, or "someone".
        text: The description (``jira:issue_created``) or the comment (``comment_created``).
        changes: The changelog items (``jira:issue_updated``).
        comment_id: ``comment.id`` (``comment_created``), else empty.
    """

    event: str
    issue_id: str
    issue_key: str
    project: str
    summary: str
    actor: str
    text: str = ""
    changes: tuple[Change, ...] = ()
    comment_id: str = ""

    def to_json(self) -> dict[str, Any]:
        """Return the event as a JSON object for the job store.

        Returns:
            The members as JSON values.
        """
        data = asdict(self)
        data["changes"] = [asdict(c) for c in self.changes]
        return data

    @classmethod
    def from_json(cls, data: Mapping[str, Any]) -> JiraEvent:
        """Rebuild an event stored with `to_json`.

        Args:
            data: The stored object.

        Returns:
            The event.
        """
        changes = tuple(Change(**c) for c in data.get("changes", ()))
        return cls(**{**data, "changes": changes})


@dataclass(frozen=True)
class Ignored:
    """A valid delivery the app does not post.

    Attributes:
        reason: ``unsupported``, ``project_not_configured`` or ``no_change``.
        event: ``webhookEvent`` when it is a string, else empty.
    """

    reason: str
    event: str = ""


@dataclass(frozen=True)
class Invalid:
    """A supported event that misses a field the app needs (``invalid_payload``).

    Attributes:
        event: ``webhookEvent``.
    """

    event: str


def _obj(value: object) -> Mapping[str, Any]:
    return value if isinstance(value, Mapping) else {}


def _str(value: object) -> str:
    return value if isinstance(value, str) else ""


def _id(value: object) -> str:
    # Jira writes IDs as strings; a number is taken as its decimal form.
    if isinstance(value, int) and not isinstance(value, bool) and value >= 0:
        return str(value)
    text = _str(value)
    return text if _DIGITS.fullmatch(text) else ""


def _changes(payload: Mapping[str, Any]) -> tuple[Change, ...]:
    items = _obj(payload.get("changelog")).get("items")
    if not isinstance(items, list):
        return ()
    out: list[Change] = []
    for item in items:
        entry = _obj(item)
        field = _str(entry.get("field"))
        if field:
            out.append(Change(field, _str(entry.get("fromString")), _str(entry.get("toString"))))
    return tuple(out)


def parse_event(payload: object, projects: Collection[str]) -> JiraEvent | Ignored | Invalid:
    """Classify a verified Jira delivery.

    Args:
        payload: The decoded JSON body.
        projects: The configured project keys.

    Returns:
        The parsed event, the reason it is ignored, or `Invalid`.
    """
    data = _obj(payload)
    event = data.get("webhookEvent")
    if not isinstance(event, str) or event not in SUPPORTED_EVENTS:
        return Ignored("unsupported", _str(event))
    issue = _obj(data.get("issue"))
    fields = _obj(issue.get("fields"))
    issue_id = _id(issue.get("id"))
    issue_key = _str(issue.get("key"))
    project = _str(_obj(fields.get("project")).get("key"))
    if not issue_id or ISSUE_KEY.fullmatch(issue_key) is None or not project:
        return Invalid(event)
    if project not in projects:
        return Ignored("project_not_configured", event)
    summary = _str(fields.get("summary"))
    if event == "comment_created":
        comment = _obj(data.get("comment"))
        comment_id = _id(comment.get("id"))
        if not comment_id:
            return Invalid(event)
        actor = _str(_obj(comment.get("author")).get("displayName")) or _SOMEONE
        return JiraEvent(
            event,
            issue_id,
            issue_key,
            project,
            summary,
            actor,
            text=read_text(comment.get("body")),
            comment_id=comment_id,
        )
    actor = _str(_obj(data.get("user")).get("displayName")) or _SOMEONE
    if event == "jira:issue_updated":
        changes = _changes(data)
        if not changes:
            return Ignored("no_change", event)
        return JiraEvent(event, issue_id, issue_key, project, summary, actor, changes=changes)
    return JiraEvent(
        event,
        issue_id,
        issue_key,
        project,
        summary,
        actor,
        text=read_text(fields.get("description")),
    )
