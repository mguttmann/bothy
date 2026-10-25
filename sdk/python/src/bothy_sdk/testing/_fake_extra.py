# SPDX-License-Identifier: Apache-2.0
"""Context messages, files, message updates and byte ranges of the FakeServer (private).

What an agent adapter of a runtime with sessions needs beyond the world's threads, files and
sessions (docs/spec/09-agents.md 4.6, 9.1, 12, 17): context messages with every trust mark and
their attachments for ``getRunContext``, ``updateMessage`` with ``If-Match`` as compare-and-swap,
``downloadFile`` with ``206`` for one byte range, and ``getFile`` and ``submitTranscript`` with the
derived transcript ``ETag`` of docs/spec/03-data-model.md R-DM-48 (docs/api/openapi.yaml).
"""

from __future__ import annotations

import base64
import hashlib
import re
from collections.abc import Mapping, Sequence
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


def file_record(
    content: bytes, *, name: str, mime: str, status: str, transcript: str | None
) -> dict[str, Any]:
    """The stored form of a file of the world.

    A file created with a transcript holds it as a finished transcript of the service, the
    state a voice message reaches in mode ``service`` (docs/spec/12-files.md 8.2).

    Args:
        content: The bytes ``downloadFile`` serves.
        name: ``original_name``.
        mime: ``mime_type``.
        status: ``clean``, ``skipped``, ``pending`` or ``blocked``.
        transcript: The transcript text, if any.

    Returns:
        The record the world keeps under the file ID.
    """
    record: dict[str, Any] = {
        "content": content,
        "name": name,
        "mime": mime,
        "status": status,
        "transcript": transcript,
        "transcript_status": "none" if transcript is None else "done",
    }
    if transcript is not None:
        record["transcript_source"] = "service"
    return record


def transcript_etag(record: Mapping[str, Any]) -> str:
    """The derived strong ``ETag`` of a file's transcript (03 R-DM-48, 06 8 point 1).

    The quoted first 22 characters of the base64url SHA-256 over ``transcript_status``,
    ``transcript_source``, ``transcript_language`` and ``transcript_text``, each followed by one
    zero byte, so a change of any of them changes the value.

    Args:
        record: A record of `file_record`.

    Returns:
        The ``ETag`` text with its quotes.
    """
    digest = hashlib.sha256()
    for name in ("transcript_status", "transcript_source", "transcript_language", "transcript"):
        digest.update(str(record.get(name) or "").encode() + b"\x00")
    return '"' + base64.urlsafe_b64encode(digest.digest()).decode()[:22] + '"'


def file_doc(
    file_id: UUID, record: Mapping[str, Any], *, uploader: UUID, at: str
) -> dict[str, Any]:
    """``getFile``: the ``File`` of a stored file, never the transcript text.

    Args:
        file_id: The file.
        record: Its record of `file_record`.
        uploader: ``uploaded_by``.
        at: ``created_at``.

    Returns:
        The document.
    """
    scan = {"blocked": "infected"}.get(record["status"], record["status"])
    doc: dict[str, Any] = {
        "id": str(file_id),
        "size_bytes": len(record["content"]),
        "mime_type": record["mime"],
        "original_name": record["name"],
        "uploaded_by": str(uploader),
        "purpose": "attachment",
        "upload_state": "pending" if scan == "pending" else "complete",
        "scan_status": scan,
        "transcript_status": record["transcript_status"],
        "thumbnails": [],
        "created_at": at,
    }
    for name in ("transcript_language", "transcript_source"):
        if record.get(name):
            doc[name] = record[name]
    return doc


def submit_transcript(
    file_id: UUID, record: dict[str, Any], body: Any, if_match: str | None
) -> dict[str, Any]:
    """``submitTranscript``: store the agent's transcript under the derived ``If-Match``.

    Args:
        file_id: The file.
        record: Its record of `file_record`; changed in place.
        body: The ``TranscriptSubmit``.
        if_match: The ``If-Match`` header.

    Returns:
        The ``Transcript`` view of the stored transcript.

    Raises:
        FakeProblem: ``428`` without ``If-Match`` or with ``*``, ``412`` with ``current`` for
            another value, ``400`` for a body outside the schema.
    """
    if if_match is None or if_match.strip() == "*":
        raise FakeProblem("precondition_required")
    if if_match.strip() != transcript_etag(record):
        raise FakeProblem("version_conflict", current=transcript_view(file_id, record))
    if not isinstance(body, dict) or set(body) - {"text", "language"}:
        raise field_error("", "unknown_member")
    if not isinstance(body.get("text"), str):
        raise field_error("/text", "required")
    record["transcript"] = body["text"]
    record["transcript_status"] = "done"
    record["transcript_source"] = "agent"
    record.pop("transcript_language", None)
    if body.get("language"):
        record["transcript_language"] = str(body["language"])
    return transcript_view(file_id, record)


def transcript_view(file_id: UUID, record: Mapping[str, Any]) -> dict[str, Any]:
    """The ``Transcript`` of a file: ``text``, ``language`` and ``source`` only when done."""
    out: dict[str, Any] = {"file_id": str(file_id), "status": record["transcript_status"]}
    if record["transcript_status"] == "done":
        out["text"] = record["transcript"] or ""
        for name, member in (("transcript_language", "language"), ("transcript_source", "source")):
            if record.get(name):
                out[member] = record[name]
    return out


def _check_if_match(if_match: str | None, version: int) -> None:
    if if_match is None:
        raise FakeProblem("precondition_required")
    if if_match.strip() not in (f'"{version}"', f'W/"{version}"'):
        raise FakeProblem("version_conflict")
