# SPDX-License-Identifier: Apache-2.0
r"""Long answers stay one message (brief M4-47; docs/spec/01-product.md 6.4 and 6.13).

A reply is never split into several messages. `send_long_message` sends a text within
``message_max_content_bytes`` as it is; a longer text goes out as one message that carries the
head (`split_for_limit`: never inside a character or a fenced code block) and the rest as one
text attachment `OVERFLOW_NAME`, uploaded through an upload session of mode ``direct``
(docs/spec/12-files.md 3.2). When that attachment cannot be made (uploads refused or switched
off, a file over ``file_max_upload_bytes``, a session of another mode), the head goes out with a
short note, still as one message, instead of a refused message.

Every request goes through the given `bothy_sdk.client.Client`, so a caller inside a run's
binding (07 SR9) uploads and sends under it.

Examples:
    >>> split_for_limit("Done.", 100)
    ('Done.', None)
    >>> split_for_limit("one\ntwo\nthree", 9)
    ('one\ntwo\n', 'three')
"""

from __future__ import annotations

import hashlib
import logging
from collections.abc import Sequence
from typing import Any
from uuid import UUID

import httpx

from bothy_sdk.client import Client
from bothy_sdk.errors import ProblemError
from bothy_sdk.generated.api.admin import get_instance
from bothy_sdk.generated.api.files import abort_upload, complete_upload, create_upload
from bothy_sdk.generated.models import (
    File,
    InstanceInfo,
    UploadComplete,
    UploadCreate,
    UploadSession,
)
from bothy_sdk.ids import uuid7
from bothy_sdk.outbox import Outbox, OutboxEntry
from bothy_sdk.uploads import upload_part

__all__ = [
    "OVERFLOW_NAME",
    "UploadNotPossible",
    "overflow_note",
    "send_long_message",
    "split_for_limit",
    "upload_text",
]

_LOG = logging.getLogger("bothy_sdk")

OVERFLOW_NAME = "answer.txt"
"""The name of the attachment that carries the rest of a long answer (D-M4-47-3)."""

_FENCE_CHARS = "`~"
_FENCE_MIN = 3
_FENCE_INDENT_MAX = 3


class UploadNotPossible(Exception):  # noqa: N818 - a state, not an error of the caller
    """The rest of a long answer cannot become an attachment (another upload mode)."""


def overflow_note(rest_bytes: int) -> str:
    """The note after the head when the rest could not be attached.

    Args:
        rest_bytes: The UTF-8 bytes of the rest that was not sent.

    Returns:
        A short English note (the language of agent texts, 09 4.7).
    """
    return f"\n\n(The rest of this answer, {rest_bytes} bytes, could not be attached.)"


def _fence(line: str) -> tuple[str, int, str] | None:
    """The fence character, its run and the rest of a fence line (CommonMark 4.5), or None."""
    stripped = line.lstrip(" ")
    if len(line) - len(stripped) > _FENCE_INDENT_MAX or not stripped:
        return None
    char = stripped[0]
    if char not in _FENCE_CHARS:
        return None
    run = len(stripped) - len(stripped.lstrip(char))
    if run < _FENCE_MIN:
        return None
    return char, run, stripped[run:]


def _open_fence_start(head: str) -> int | None:
    """The offset of the line that opens a fenced block still open at the end of ``head``."""
    open_at: tuple[str, int, int] | None = None
    start = 0
    for line in head.split("\n"):
        fence = _fence(line)
        if open_at is None:
            if fence is not None and not (fence[0] == "`" and "`" in fence[2]):
                open_at = (fence[0], fence[1], start)
        elif (
            fence is not None
            and fence[0] == open_at[0]
            and fence[1] >= open_at[1]
            and not fence[2].strip()
        ):
            open_at = None
        start += len(line) + 1
    return open_at[2] if open_at is not None else None


def split_for_limit(text: str, max_bytes: int) -> tuple[str, str | None]:
    """Cut ``text`` so the head has at most ``max_bytes`` UTF-8 bytes (D-M4-47-3).

    The head is the longest prefix within the limit that never splits a character, shortened to
    its last line break when it has one. When the cut lies inside a fenced code block, the head
    ends before the line that opens the block, so the block goes whole into the rest (01 6.4: a
    code block stays one block); a block that starts the text stays cut, because the head would
    be empty. Head and rest together are always exactly ``text``.

    Args:
        text: The answer.
        max_bytes: ``message_max_content_bytes`` (at least 1).

    Returns:
        The head and the rest, or the text and ``None`` when it fits.
    """
    if len(text.encode("utf-8")) <= max_bytes:
        return text, None
    used = 0
    cut = 0
    for char in text:
        size = len(char.encode("utf-8", errors="replace"))
        if used + size > max_bytes:
            break
        used += size
        cut += 1
    if cut > 0:
        newline = text.rfind("\n", 0, cut)
        if newline >= 0:
            cut = newline + 1
    opened = _open_fence_start(text[:cut])
    if opened is not None and opened > 0:
        cut = opened
    return text[:cut], text[cut:]


async def upload_text(client: Client, channel_id: UUID, name: str, data: bytes) -> UUID:
    """Upload ``data`` as the file ``name`` to ``channel_id`` (12 3.2, mode ``direct``).

    Args:
        client: The SDK client (inside the run's binding when called from a run).
        channel_id: The channel of the message the file will be attached to.
        name: The file name.
        data: The bytes (at least one).

    Returns:
        The ``file_id`` of the completed upload.

    Raises:
        ProblemError: The server refused a step (for example ``413`` above
            ``file_max_upload_bytes``, ``403`` when uploads are off).
        httpx.TransportError: The server could not be reached.
        UploadNotPossible: The server answered with an upload mode other than ``direct``.
    """
    body = UploadCreate(
        name=name,
        size_bytes=len(data),
        sha256=hashlib.sha256(data).hexdigest(),
        declared_mime_type="text/plain",
    )
    with client.idempotency_key(str(uuid7())):
        session = await client.call_as(
            UploadSession, create_upload.asyncio_detailed, channel_id=channel_id, body=body
        )
    try:
        if str(session.mode.value) != "direct":
            raise UploadNotPossible(str(session.mode.value))
        size = int(session.part_size_bytes)
        parts = []
        for part in session.parts:
            start = (part.part_number - 1) * size
            headers = {h.name: h.value for h in part.headers}
            parts.append(
                await upload_part(
                    client, session.file_id, part.part_number, data[start : start + size], headers
                )
            )
        with client.idempotency_key(str(uuid7())):
            done = await client.call_as(
                File,
                complete_upload.asyncio_detailed,
                file_id=session.file_id,
                body=UploadComplete(parts=parts),
            )
    except (ProblemError, httpx.TransportError, UploadNotPossible):
        await _abort(client, session.file_id)
        raise
    return done.id


async def _abort(client: Client, file_id: UUID) -> None:
    """``abortUpload`` of a session that will not complete (best effort).

    The session then holds none of the principal's open upload sessions until it expires.
    """
    try:
        await client.call(abort_upload.asyncio_detailed, file_id=file_id)
    except (ProblemError, httpx.TransportError) as exc:
        _LOG.warning("long answer: upload not aborted code=%s", getattr(exc, "code", "transport"))


async def _limit(client: Client) -> int:
    info = await client.call_as(InstanceInfo, get_instance.asyncio_detailed)
    return int(info.limits.message_max_content_bytes)


async def send_long_message(  # noqa: PLR0913 - the members of a reply and its place
    client: Client,
    outbox: Outbox,
    thread_id: UUID,
    channel_id: UUID,
    content_md: str,
    *,
    max_content_bytes: int | None = None,
    attachment_file_ids: Sequence[UUID] = (),
    **kwargs: Any,
) -> OutboxEntry:
    """Send ``content_md`` as one message, its rest above the limit as one attachment.

    Args:
        client: The SDK client.
        outbox: The outbox the message goes through.
        thread_id: The thread.
        channel_id: Its channel (the upload goes there).
        content_md: The whole answer.
        max_content_bytes: ``message_max_content_bytes``; ``None`` reads it once with
            ``getInstance``.
        attachment_file_ids: Files of the caller; the rest of a long text comes after them.
        **kwargs: Further arguments of `bothy_sdk.outbox.Outbox.send_message` (``run_id``,
            ``reply_to_message_id``, ``client_msg_id``, ...).

    Returns:
        The outbox entry of the one message.
    """
    limit = max_content_bytes if max_content_bytes is not None else await _limit(client)
    files = list(attachment_file_ids)
    if len(content_md.encode("utf-8")) <= limit:
        return await outbox.send_message(thread_id, content_md, attachment_file_ids=files, **kwargs)
    head, rest = split_for_limit(content_md, limit)
    try:
        file_id = await upload_text(client, channel_id, OVERFLOW_NAME, (rest or "").encode("utf-8"))
    except (ProblemError, httpx.TransportError, UploadNotPossible) as exc:
        code = getattr(exc, "code", type(exc).__name__)
        _LOG.warning("long answer: rest not attached code=%s", code)
        # The note names the bytes left out; its length for the whole text bounds it, so the
        # shorter head and the note fit the limit together.
        room = len(overflow_note(len(content_md.encode("utf-8"))).encode("utf-8"))
        head, rest = split_for_limit(content_md, max(1, limit - room))
        note = overflow_note(len((rest or "").encode("utf-8")))
        return await outbox.send_message(
            thread_id, head + note, attachment_file_ids=files, **kwargs
        )
    return await outbox.send_message(
        thread_id, head, attachment_file_ids=[*files, file_id], **kwargs
    )
