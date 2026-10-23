# SPDX-License-Identifier: Apache-2.0
"""Context messages, files, message updates and byte ranges of the FakeServer (private).

What an agent adapter of a runtime with sessions needs beyond the world's threads, files and
sessions (docs/spec/09-agents.md 4.6, 9.1, 12, 17): context messages with every trust mark and
their attachments for ``getRunContext``, ``updateMessage`` with ``If-Match`` as compare-and-swap,
and ``downloadFile`` with ``206`` for one byte range (docs/api/openapi.yaml).
"""

from __future__ import annotations

import re
from collections.abc import Sequence
from dataclasses import dataclass, field
from typing import Any
from uuid import UUID

from bothy_sdk.testing._fake_base import FakeProblem, field_error

_RANGE = re.compile(r"^bytes=(\d+)-(\d*)$")
_TRUST = frozenset({"trigger", "authorized", "self", "unverified", "system"})
_ROLES = frozenset({"user", "assistant", "other"})


@dataclass(frozen=True)
class ContextFile:
    """A file attached to a context message.

    Attributes:
        file_id: The file.
        name: ``original_name``.
        mime_type: The MIME type.
        content: The bytes ``downloadFile`` serves.
        scan_status: ``clean``, ``skipped``, ``pending`` or ``blocked``; ``pending`` answers
            ``409 file_not_ready`` and ``blocked`` ``409 file_blocked``.
        transcript: The transcript of a voice message, if any.
    """

    file_id: UUID
    name: str
    mime_type: str
    content: bytes = field(repr=False)
    scan_status: str = "clean"
    transcript: str | None = field(default=None, repr=False)

    def attachment(self) -> dict[str, Any]:
        """The ``RunContextAttachment`` of the file."""
        out: dict[str, Any] = {
            "file_id": str(self.file_id),
            "original_name": self.name,
            "mime_type": self.mime_type,
            "size_bytes": len(self.content),
            "scan_status": self.scan_status,
        }
        if self.transcript is not None:
            out["transcript"] = self.transcript
        return out


@dataclass(frozen=True)
class ContextMessage:
    """A message that stands in a thread before the triggering message.

    Attributes:
        author: The author's handle: ``alex``, ``sam``, ``writer``, ``robin``, ``lotse`` or
            ``texter``; ``system`` for a system line.
        text: The content.
        trust: The trust mark the context gives it.
        role: ``user``, ``assistant`` or ``other``.
        origin: ``human``, ``agent``, ``import`` or ``system``.
        files: Its attachments.
        already_delivered: Whether the session already holds it (09 4.6).
    """

    author: str
    text: str = field(repr=False)
    trust: str = "authorized"
    role: str = "user"
    origin: str = "human"
    files: Sequence[ContextFile] = ()
    already_delivered: bool = False

    def __post_init__(self) -> None:
        """Refuse marks and roles outside the schema."""
        if self.trust not in _TRUST or self.role not in _ROLES:
            raise ValueError("unknown trust mark or role")


class Extras:
    """Data of the operations of this module beyond the world's threads, files and sessions.

    Args:
        agent_id: Lotse's principal ID (only its own messages may be updated).
    """

    def __init__(self, agent_id: UUID) -> None:
        """Create empty tables."""
        self.agent_id = agent_id
        self.message_meta: dict[UUID, dict[str, Any]] = {}
        self.titles_set: list[dict[str, Any]] = []
        self.message_updates: list[dict[str, Any]] = []
        self.reports: dict[UUID, list[dict[str, Any]]] = {}

    def update_message(
        self, message: dict[str, Any], body: Any, if_match: str | None
    ) -> dict[str, Any]:
        """``updateMessage`` on an own message."""
        if message["author"]["id"] != str(self.agent_id):
            raise FakeProblem("forbidden")
        _check_if_match(if_match, message["version"])
        if not isinstance(body, dict) or set(body) - {"content_md", "mention_ids"}:
            raise field_error("", "unknown_member")
        if "content_md" in body:
            message["content_md"] = str(body["content_md"])
        message["version"] += 1
        self.message_updates.append({"message_id": message["id"], "body": dict(body)})
        return message


def byte_range(
    content: bytes, mime: str, range_header: str | None
) -> tuple[int, bytes, dict[str, str]]:
    """``downloadFile``'s answer: the whole file, or exactly one byte range (``206``)."""
    headers = {"Content-Type": mime, "Accept-Ranges": "bytes"}
    if range_header is None:
        return 200, content, headers
    match = _RANGE.match(range_header.strip())
    if match is None or not content:
        raise FakeProblem("range_not_satisfiable", status=416)
    start = int(match.group(1))
    end = int(match.group(2)) if match.group(2) else len(content) - 1
    end = min(end, len(content) - 1)
    if start > end:
        raise FakeProblem("range_not_satisfiable", status=416)
    headers["Content-Range"] = f"bytes {start}-{end}/{len(content)}"
    return 206, content[start : end + 1], headers


def _check_if_match(if_match: str | None, version: int) -> None:
    if if_match is None:
        raise FakeProblem("precondition_required")
    if if_match.strip() not in (f'"{version}"', f'W/"{version}"'):
        raise FakeProblem("version_conflict")
