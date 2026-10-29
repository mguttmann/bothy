# SPDX-License-Identifier: Apache-2.0
"""Files Hermes sends with an answer and the audio it transcribes (09 13, 17.3, 17.6).

Outgoing (``send_document``, ``send_image_file``, ``send_voice``, ``send_video``):
`read_local_file` opens the file Hermes names once, without following a symbolic link, checks
that it is a regular file within the server's ``file_max_upload_bytes`` and hashes it;
`upload` sends it through the upload session of the public file API in mode ``direct``
(``createUpload``, ``uploadPart``, ``completeUpload``, each under the run's binding, 07 SR9) and
aborts the session when a step fails. The server scans the file after the upload (12); the
message that carries it is the caller's.

Incoming: `audio_to_transcribe` names the audio attachments of the triggering message that
carry no transcript and that the server serves (scan state ``clean`` or ``skipped``);
`with_transcripts` puts Hermes's transcripts on them, so the context renders them in the
person's text with the trust of the message (ADR 0054), as a transcript of the server.

Nothing here logs or returns a path, a file name or a content (SEC-LOG-2); refusals carry codes.
"""

from __future__ import annotations

import asyncio
import contextlib
import hashlib
import logging
import mimetypes
import os
import stat
from collections.abc import Mapping, Sequence
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Final
from uuid import UUID

import httpx
from bothy_sdk.client import Client
from bothy_sdk.errors import ProblemError
from bothy_sdk.generated.api.files import abort_upload, complete_upload, create_upload
from bothy_sdk.generated.models import (
    File,
    RunContext,
    RunContextAttachment,
    UploadComplete,
    UploadCreate,
    UploadPartResult,
    UploadSession,
)
from bothy_sdk.generated.types import Unset
from bothy_sdk.ids import uuid7
from bothy_sdk.uploads import upload_part

__all__ = [
    "FileRefused",
    "LocalFile",
    "audio_cache_ext",
    "audio_to_transcribe",
    "read_local_file",
    "upload",
    "with_transcripts",
]

_LOG = logging.getLogger("bothy_hermes_adapter")

_READ_CHUNK: Final = 1024 * 1024
_SERVED: Final = frozenset({"clean", "skipped"})
#: Audio suffixes a voice attachment keeps in Hermes's audio cache; Hermes's speech to text
#: picks its handling from the suffix, and the person names the file.
_AUDIO_SUFFIXES: Final = frozenset(
    {".aac", ".amr", ".flac", ".m4a", ".mp3", ".mp4", ".oga", ".ogg", ".opus", ".wav", ".webm"}
)
_DEFAULT_AUDIO_SUFFIX: Final = ".ogg"


def audio_cache_ext(name: str, mime_type: str) -> str:
    """The suffix of an audio attachment in Hermes's audio cache (09 17.6).

    The name's suffix when it is a known audio format, else the MIME type's when that is one,
    else ``.ogg``; a suffix such as ``.45`` of ``Voice 10.45`` never reaches Hermes.
    """
    suffix = Path(name).suffix.lower()
    if suffix in _AUDIO_SUFFIXES:
        return suffix
    guessed = (mimetypes.guess_extension(mime_type.partition(";")[0].strip()) or "").lower()
    return guessed if guessed in _AUDIO_SUFFIXES else _DEFAULT_AUDIO_SUFFIX


class FileRefused(Exception):  # noqa: N818 - names the outcome, like PrepareRefused
    """A file was not sent; ``code`` says why (never a path or a content).

    Args:
        code: ``not_a_file``, ``file_empty``, ``file_too_large``, ``file_changed``,
            ``upload_mode_unsupported``, ``transport_error`` or the server's problem code.
    """

    def __init__(self, code: str) -> None:
        """Keep the code."""
        super().__init__(code)
        self.code = code


@dataclass(frozen=True)
class LocalFile:
    """One file to send, opened and hashed once.

    Attributes:
        fd: The open file descriptor; the upload reads only through it.
        name: The upload name (the last path component, or Hermes's ``file_name``).
        size: Its byte count.
        sha256: Its lowercase hex SHA-256.
    """

    fd: int
    name: str = field(repr=False)
    size: int
    sha256: str

    def close(self) -> None:
        """Close the descriptor."""
        with contextlib.suppress(OSError):
            os.close(self.fd)


def _hash(fd: int, size: int) -> str:
    digest = hashlib.sha256()
    offset = 0
    while offset < size:
        chunk = os.pread(fd, min(_READ_CHUNK, size - offset), offset)
        if not chunk:
            raise FileRefused("file_changed")
        digest.update(chunk)
        offset += len(chunk)
    return digest.hexdigest()


def read_local_file(path: str, name: str | None, max_bytes: int) -> LocalFile:
    """Open, check and hash the file Hermes names (09 17.3).

    Args:
        path: The path Hermes's delivery policy accepted.
        name: Hermes's ``file_name``, if any; else the last path component.
        max_bytes: ``InstanceLimits.file_max_upload_bytes``.

    Returns:
        The open file; the caller closes it.

    Raises:
        FileRefused: Not a regular file, empty, larger than ``max_bytes`` or changed while
            read.
    """
    flags = os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC
    try:
        fd = os.open(path, flags)
    except OSError:
        raise FileRefused("not_a_file") from None
    try:
        info = os.fstat(fd)
        if not stat.S_ISREG(info.st_mode):
            raise FileRefused("not_a_file")
        if info.st_size <= 0:
            raise FileRefused("file_empty")
        if info.st_size > max_bytes:
            raise FileRefused("file_too_large")
        sha256 = _hash(fd, info.st_size)
    except BaseException:
        os.close(fd)
        raise
    upload_name = Path(name).name if name and Path(name).name else Path(path).name
    return LocalFile(fd, upload_name, info.st_size, sha256)


def _parts(size: int, part_size: int) -> list[tuple[int, int, int]]:
    count = max(1, -(-size // part_size))
    return [
        (n, (n - 1) * part_size, min(part_size, size - (n - 1) * part_size))
        for n in range(1, count + 1)
    ]


async def upload(client: Client, channel_id: UUID, local: LocalFile) -> UUID:
    """Upload one file for a channel in mode ``direct`` (12 3.2); abort it on any failure.

    Args:
        client: The client; the caller is inside the run's binding (07 SR9).
        channel_id: The channel of the run's thread.
        local: The open file.

    Returns:
        The completed file.

    Raises:
        FileRefused: The server refused a step, the network failed, the mode is not
            ``direct`` or the file changed during the upload.
    """
    body = UploadCreate(name=local.name, size_bytes=local.size, sha256=local.sha256)
    declared, _ = mimetypes.guess_type(local.name)
    if declared:
        body.declared_mime_type = declared
    file_id: UUID | None = None
    try:
        with client.idempotency_key(str(uuid7())):
            session = await client.call_as(
                UploadSession, create_upload.asyncio_detailed, channel_id=channel_id, body=body
            )
        file_id = session.file_id
        if str(session.mode.value) != "direct":
            raise FileRefused("upload_mode_unsupported")
        plans: Mapping[int, Any] = {p.part_number: p for p in session.parts}
        etags: list[UploadPartResult] = []
        for number, offset, length in _parts(local.size, int(session.part_size_bytes)):
            plan = plans.get(number)
            if plan is None:
                raise FileRefused("upload_mode_unsupported")
            data = await asyncio.to_thread(os.pread, local.fd, length, offset)
            if len(data) != length:
                raise FileRefused("file_changed")
            headers = {h.name: h.value for h in plan.headers}
            done = await upload_part(client, file_id, number, data, headers)
            etags.append(UploadPartResult(part_number=number, etag=done.etag))
        with client.idempotency_key(str(uuid7())):
            await client.call_as(
                File,
                complete_upload.asyncio_detailed,
                file_id=file_id,
                body=UploadComplete(parts=etags),
            )
        return file_id
    except ProblemError as exc:
        await _abort(client, file_id)
        raise FileRefused(exc.code) from None
    except httpx.TransportError:
        await _abort(client, file_id)
        raise FileRefused("transport_error") from None
    except FileRefused:
        await _abort(client, file_id)
        raise


async def _abort(client: Client, file_id: UUID | None) -> None:
    """``abortUpload`` of an open session (best effort, codes only)."""
    if file_id is None:
        return
    try:
        await client.call(abort_upload.asyncio_detailed, file_id=file_id)
    except ProblemError as exc:
        _LOG.info("upload not aborted code=%s", exc.code)
    except httpx.TransportError:
        _LOG.info("upload not aborted code=transport_error")


def audio_to_transcribe(ctx: RunContext) -> list[RunContextAttachment]:
    """The audio attachments of the triggering message that need Hermes's transcript (09 13).

    An attachment counts when its type is ``audio/*``, it carries no transcript (a transcript of
    the server, mode ``service``, is used as it is) and the server serves it (scan state
    ``clean`` or ``skipped``). Attachments of context messages and of follow-ups do not count.

    Args:
        ctx: The run context.

    Returns:
        The attachments in their order.
    """
    out = []
    for item in ctx.trigger_message.attachments:
        mime = str(item.mime_type).split(";", 1)[0].strip().lower()
        has_transcript = isinstance(item.transcript, str) and not isinstance(item.transcript, Unset)
        if mime.startswith("audio/") and not has_transcript and str(item.scan_status) in _SERVED:
            out.append(item)
    return out


def with_transcripts(ctx: RunContext, transcripts: Mapping[UUID, str]) -> RunContext:
    """The context with Hermes's transcripts on the triggering message's attachments.

    Args:
        ctx: The run context.
        transcripts: Transcript per file.

    Returns:
        A new context.
    """
    if not transcripts:
        return ctx
    data = ctx.to_dict()
    attachments: Sequence[dict[str, Any]] = data["trigger_message"].get("attachments", [])
    for item in attachments:
        text = transcripts.get(UUID(str(item["file_id"])))
        if text is not None:
            item["transcript"] = text
    return RunContext.from_dict(data)
