# SPDX-License-Identifier: Apache-2.0
"""Upload sessions, instance limits and attaching for the FakeServer (M3-03, M3-07, M3-12).

``getInstance`` answers with configurable ``InstanceLimits``. ``createUpload``,
``uploadPart``, ``getUpload``, ``completeUpload`` and ``abortUpload`` follow M3-03 in mode
``direct``: a small ``part_size_bytes`` per test, length and ``Content-Digest`` of each part
checked, the same part with the same digest answers again without new work and with another
digest ``409 conflict`` with ``reason`` ``part_conflict``, a completed session cannot be
aborted (``409 invalid_state_transition``). The received bytes are kept per file, so a test
compares them with the files it announced. ``createMessage`` attaches only completed own
uploads of the message's channel that are not attached yet (``409 conflict`` with ``reason``
``file_not_attachable`` otherwise, M3-07). An upload is never scanned (``scan_status``
``pending``), so a voice message with one answers ``409 file_not_ready`` (12 8.1).
"""

from __future__ import annotations

import base64
import hashlib
from collections.abc import Callable, Sequence
from dataclasses import dataclass, field
from datetime import datetime, timedelta
from typing import Any
from uuid import UUID

from bothy_sdk.ids import uuid7
from bothy_sdk.testing._fake_base import FakeProblem, field_error

__all__ = ["DEFAULT_LIMITS", "FakeUploads", "Upload"]

DEFAULT_LIMITS: dict[str, int] = {
    "message_max_content_bytes": 262144,
    "message_max_attachments": 10,
    "file_max_upload_bytes": 104857600,
    "thread_title_max_chars": 200,
    "voice_max_duration_ms": 900000,
}
"""``InstanceLimits`` of the FakeServer (the examples of docs/api/openapi.yaml)."""

_DEFAULT_PART_SIZE = 5 * 1024 * 1024
_SESSION_TTL = timedelta(hours=24)
_SHA256_HEX = 64


@dataclass
class Upload:
    """One upload session and its file.

    Attributes:
        file_id: The file.
        channel_id: ``scope_channel_id``.
        uploader: ``uploaded_by``.
        name: ``original_name``.
        size: ``size_bytes``.
        sha256: The declared lowercase hex SHA-256.
        part_size: ``part_size_bytes``.
        parts: Confirmed parts: number to (digest, bytes, etag).
        state: ``open``, ``complete`` or ``aborted``.
        attached: The message the file is attached to, if any.
        created_at: The creation time.
    """

    file_id: UUID
    channel_id: UUID
    uploader: UUID
    name: str
    size: int
    sha256: str
    part_size: int
    created_at: str
    expires_at: str
    parts: dict[int, tuple[str, bytes, str]] = field(default_factory=dict)
    state: str = "open"
    attached: UUID | None = None

    @property
    def part_count(self) -> int:
        """How many parts the session has."""
        return max(1, -(-self.size // self.part_size))

    def content(self) -> bytes:
        """The received bytes in part order."""
        return b"".join(self.parts[n][1] for n in sorted(self.parts))

    def file_doc(self) -> dict[str, Any]:
        """The ``File`` of the upload."""
        doc: dict[str, Any] = {
            "id": str(self.file_id),
            "size_bytes": self.size,
            "sha256": self.sha256,
            "mime_type": "application/octet-stream",
            "original_name": self.name,
            "uploaded_by": str(self.uploader),
            "purpose": "attachment",
            "upload_state": "complete" if self.state == "complete" else "pending",
            "scan_status": "pending",
            "transcript_status": "none",
            "thumbnails": [],
            "created_at": self.created_at,
        }
        if self.attached is None:
            doc["scope_channel_id"] = str(self.channel_id)
        return doc


class FakeUploads:
    """The upload part of the FakeServer world.

    Args:
        now: The FakeServer clock.
        uploader: The agent whose token the FakeServer serves (``uploaded_by``).
    """

    def __init__(self, now: Callable[[], datetime], uploader: UUID) -> None:
        """Create an empty store with the default limits."""
        self.now = now
        self.uploader = uploader
        self.limits: dict[str, int] = dict(DEFAULT_LIMITS)
        self.part_size = _DEFAULT_PART_SIZE
        self.sessions: dict[UUID, Upload] = {}

    def instance(self, instance_id: UUID) -> dict[str, Any]:
        """``getInstance``: an ``InstanceInfo`` with the configured limits."""
        return {
            "instance_id": str(instance_id),
            "branding": {"product_name": "Team Chat", "version": 1},
            "locale_default": "en",
            "spec_version": "1.0.0",
            "features": [],
            "limits": dict(self.limits),
            # The app lock policy of the server defaults (M5-08), required in ``InstanceInfo``.
            "app_lock": {"required": False, "after_ms": 300_000},
        }

    def _get(self, file_id: str) -> Upload:
        try:
            upload = self.sessions.get(UUID(file_id))
        except ValueError:
            upload = None
        if upload is None or upload.uploader != self.uploader:
            raise FakeProblem("not_found")
        return upload

    def create(self, channel_id: str, body: Any) -> dict[str, Any]:
        """``createUpload`` (mode ``direct``).

        Raises:
            FakeProblem: A body outside ``UploadCreate`` or a size above the limit.
        """
        if not isinstance(body, dict):
            raise field_error("", "required")
        unknown = set(body) - {"name", "size_bytes", "sha256", "declared_mime_type"}
        if unknown:
            raise field_error(f"/{sorted(unknown)[0]}", "unknown_member")
        for name in ("name", "size_bytes", "sha256"):
            if name not in body:
                raise field_error(f"/{name}", "required")
        size, sha = body["size_bytes"], body["sha256"]
        if not isinstance(size, int) or isinstance(size, bool) or size < 1:
            raise field_error("/size_bytes", "too_small")
        if (
            not isinstance(sha, str)
            or len(sha) != _SHA256_HEX
            or any(c not in "0123456789abcdef" for c in sha)
        ):
            raise field_error("/sha256", "pattern_mismatch")
        if size > self.limits["file_max_upload_bytes"]:
            raise FakeProblem("payload_too_large")
        now = self.now()
        upload = Upload(
            file_id=uuid7(),
            channel_id=UUID(channel_id),
            uploader=self.uploader,
            name=str(body["name"]),
            size=size,
            sha256=sha,
            part_size=self.part_size,
            created_at=now.isoformat(),
            expires_at=(now + _SESSION_TTL).isoformat(),
        )
        self.sessions[upload.file_id] = upload
        return self._session(upload)

    @staticmethod
    def _session(upload: Upload) -> dict[str, Any]:
        base = f"/api/v1/uploads/{upload.file_id}/parts/"
        return {
            "file_id": str(upload.file_id),
            "mode": "direct",
            "part_size_bytes": upload.part_size,
            "parts": [
                {
                    "part_number": n,
                    "url": f"{base}{n}",
                    "headers": [{"name": "Content-Type", "value": "application/octet-stream"}],
                }
                for n in range(1, upload.part_count + 1)
            ],
            "expires_at": upload.expires_at,
        }

    def put_part(
        self, file_id: str, part_number: str, data: bytes, digest: str | None
    ) -> dict[str, Any]:
        """``uploadPart``: length and digest checked, idempotent per digest.

        Raises:
            FakeProblem: ``404``, ``410``, ``409`` or ``400`` as in M3-03.
        """
        upload = self._get(file_id)
        if upload.state == "aborted":
            raise FakeProblem("gone")
        if upload.state == "complete":
            raise FakeProblem("invalid_state_transition")
        try:
            number = int(part_number)
        except ValueError:
            raise field_error("#path/part_number", "invalid_type") from None
        if not 1 <= number <= upload.part_count:
            raise field_error("#path/part_number", "out_of_range")
        start = (number - 1) * upload.part_size
        expected = min(upload.part_size, upload.size - start)
        wanted = "sha-256=:" + base64.b64encode(hashlib.sha256(data).digest()).decode() + ":"
        if len(data) != expected or digest != wanted:
            raise field_error("", "part_digest_mismatch")
        stored = upload.parts.get(number)
        if stored is not None:
            if stored[0] != digest:
                raise FakeProblem("conflict", reason="part_conflict")
            return {"part_number": number, "etag": stored[2]}
        etag = f'"{hashlib.md5(data, usedforsecurity=False).hexdigest()}"'
        upload.parts[number] = (digest, data, etag)
        return {"part_number": number, "etag": etag}

    def state(self, file_id: str) -> dict[str, Any]:
        """``getUpload``.

        Raises:
            FakeProblem: ``404`` or ``410 gone`` for an aborted session.
        """
        upload = self._get(file_id)
        if upload.state == "aborted":
            raise FakeProblem("gone")
        if upload.state == "complete":
            label = "complete"
        else:
            label = "uploading" if upload.parts else "pending"
        return {
            "file_id": str(upload.file_id),
            "upload_state": label,
            "part_size_bytes": upload.part_size,
            "completed_parts": [
                {"part_number": n, "etag": upload.parts[n][2]} for n in sorted(upload.parts)
            ],
            "expires_at": upload.expires_at,
        }

    def complete(self, file_id: str, body: Any) -> dict[str, Any]:
        """``completeUpload``: every part confirmed with its etag (the scan checks the hash later).

        Raises:
            FakeProblem: ``404``, ``410``, ``409`` or ``400`` as in M3-03.
        """
        upload = self._get(file_id)
        if upload.state == "aborted":
            raise FakeProblem("gone")
        if upload.state == "complete":
            return upload.file_doc()  # a completed session answers the file again (12 3.4)
        if not isinstance(body, dict) or not isinstance(body.get("parts"), list):
            raise field_error("/parts", "required")
        given = {}
        for item in body["parts"]:
            if not isinstance(item, dict) or not isinstance(item.get("part_number"), int):
                raise field_error("/parts", "invalid_type")
            number = item["part_number"]
            if number in given or not 1 <= number <= upload.part_count:
                raise field_error("/parts", "invalid_value")
            given[number] = item.get("etag")
        if set(given) != set(range(1, upload.part_count + 1)) or set(given) - set(upload.parts):
            raise FakeProblem("conflict", reason="upload_incomplete")
        if any(upload.parts[n][2] != etag for n, etag in given.items()):
            raise FakeProblem("conflict", reason="part_conflict")
        upload.state = "complete"
        return upload.file_doc()

    def abort(self, file_id: str) -> None:
        """``abortUpload``: an open session ends; a completed one cannot be aborted.

        Raises:
            FakeProblem: ``404`` or ``409 invalid_state_transition``.
        """
        upload = self._get(file_id)
        if upload.state == "complete":
            raise FakeProblem("invalid_state_transition")
        upload.state = "aborted"
        upload.parts.clear()

    def check_attachable(self, ids: Any, channel_id: UUID) -> list[Upload]:
        """The uploads of ``attachment_file_ids`` of a new message, or the problem (M3-07).

        Raises:
            FakeProblem: ``400`` for a malformed or too long list, ``409 conflict`` with
                ``reason`` ``file_not_attachable`` for a file that is not attachable.
        """
        if not isinstance(ids, list):
            raise field_error("/attachment_file_ids", "invalid_type")
        if len(ids) > self.limits["message_max_attachments"]:
            raise field_error("/attachment_file_ids", "limit_reached")
        out: list[Upload] = []
        for index, raw in enumerate(ids):
            try:
                upload = self.sessions.get(UUID(str(raw)))
            except ValueError:
                upload = None
            if (
                upload is None
                or upload.uploader != self.uploader
                or upload.channel_id != channel_id
                or upload.state != "complete"
                or upload.attached is not None
                or upload in out
            ):
                raise FakeProblem(
                    "conflict",
                    reason="file_not_attachable",
                    detail=f"/attachment_file_ids/{index}",
                )
            out.append(upload)
        return out

    @staticmethod
    def attach(uploads: Sequence[Upload], message: dict[str, Any]) -> None:
        """Attach checked uploads to a stored message in their order."""
        for position, upload in enumerate(uploads):
            upload.attached = UUID(message["id"])
            message["attachments"].append({"position": position, "file": upload.file_doc()})
