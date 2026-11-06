# SPDX-License-Identifier: Apache-2.0
"""Attachments of the triggering message as local files (09 4.6, 18.10, design note 10).

Only attachments with ``scan_status`` ``clean`` or ``skipped`` are fetched with
``downloadFile`` into ``<runner.state_dir>/files/<thread_id>/<file_id>`` (mode ``0600``). The
context helper gets, as the text of each such file, only ``local file: <path>`` (or
``unavailable: <problem code>``), so the path stands inside the file's data block and no file
content enters an input of the turn. The run continues in every case.

Examples:
    >>> FETCHED
    frozenset({'clean', 'skipped'})
"""

from __future__ import annotations

import json
import logging
import re
from collections.abc import Mapping, Sequence
from pathlib import Path
from uuid import UUID

import httpx
from bothy_sdk.client import Client
from bothy_sdk.errors import ProblemError
from bothy_sdk.generated.api.files import download_file
from bothy_sdk.generated.models import Attachment, RunContextAttachment

from claude_code_runner.statedir import ensure_private_dir, write_private

__all__ = [
    "FETCHED",
    "describe_followup_files",
    "fetch_attachments",
    "fetch_followup_files",
    "followup_files_final",
]

_LOG = logging.getLogger("claude_code_runner")

FETCHED = frozenset({"clean", "skipped"})
"""Scan states whose files are downloaded."""


async def fetch_attachments(
    client: Client, attachments: Sequence[RunContextAttachment], files_dir: Path
) -> tuple[dict[UUID, str], int]:
    """Download the attachments of the triggering message.

    Args:
        client: The SDK client, bound to the run by the caller (07 SR9).
        attachments: ``trigger_message.attachments`` of the run context.
        files_dir: ``<runner.state_dir>/files/<thread_id>``.

    Returns:
        The text per ``file_id`` for `bothy_sdk.context.render_context` and the number of
        files stored.
    """
    texts: dict[UUID, str] = {}
    stored = 0
    for item in attachments:
        if item.scan_status not in FETCHED:
            texts[item.file_id] = f"unavailable: {_code(item.scan_status)}"
            continue
        try:
            response = await client.call_detailed(
                download_file.asyncio_detailed, file_id=item.file_id
            )
        except ProblemError as exc:
            texts[item.file_id] = f"unavailable: {exc.code}"
            _LOG.info(json.dumps({"event": "attachment_unavailable", "code": exc.code}))
            continue
        except httpx.TransportError:
            texts[item.file_id] = "unavailable: transport_error"
            continue
        ensure_private_dir(files_dir)
        path = files_dir / str(item.file_id)
        write_private(path, response.content)
        texts[item.file_id] = f"local file: {path}"
        stored += 1
    return texts, stored


def _code(status: str) -> str:
    """A scan status that is not fetched, as the problem code the server would answer."""
    return "file_blocked" if status == "blocked" else "file_not_ready"


_BLOCKED = frozenset({"infected", "error"})
"""Scan states of a file the server never serves."""

_CONTROL = re.compile(r"[\x00-\x1f\x7f]+")


def _value(member: object) -> str:
    """The wire value of a generated enum member (or a plain string)."""
    return str(getattr(member, "value", member))


def _final(item: Attachment) -> bool:
    """Whether the server's check of the attachment has ended (12 2): servable, or never."""
    upload, scan = _value(item.file.upload_state), _value(item.file.scan_status)
    if upload == "failed" or scan in _BLOCKED:
        return True
    return upload == "complete" and scan in FETCHED


def followup_files_final(attachments: Sequence[Attachment]) -> bool:
    """Whether every attachment of a follow-up is final, so its line can be written.

    A follow-up is delivered when the message is stored, also while a file is still checked
    (12 10 rule 1); an image stays ``pending`` after a clean scan until its cleaned original is
    stored (12 3.4), and only then can it be downloaded (FIX-scan-state-stale).

    Args:
        attachments: ``attachments`` of the message read with ``getMessage``.

    Returns:
        True when no attachment waits for the server.
    """
    return all(_final(item) for item in attachments)


async def fetch_followup_files(
    client: Client, attachments: Sequence[Attachment], files_dir: Path
) -> tuple[list[RunContextAttachment], dict[UUID, str]]:
    """Store the servable attachments of a follow-up and give the text of every attachment.

    A servable file goes to ``files_dir/<file_id>`` like an attachment of the trigger and its
    text is ``local file: <path>``; a file that is blocked or failed is ``unavailable:
    file_blocked``, one still not ready ``unavailable: file_not_ready``, one that could not be
    downloaded ``unavailable: <problem code>``. Each attachment is also given in the shape of
    the run context, so the transcriber takes it like an attachment of the trigger (09 18.14,
    FIX-followup-transcribe): without a transcript, since a message carries none.

    Args:
        client: The SDK client, bound to the run by the caller (07 SR9).
        attachments: ``attachments`` of the follow-up message.
        files_dir: ``<runner.state_dir>/files/<thread_id>``.

    Returns:
        The attachments in position order and the text per ``file_id``.
    """
    items: list[RunContextAttachment] = []
    texts: dict[UUID, str] = {}
    for item in attachments:
        f = item.file
        upload, scan = _value(f.upload_state), _value(f.scan_status)
        entry = RunContextAttachment(
            file_id=f.id,
            original_name=f.original_name,
            mime_type=f.mime_type,
            size_bytes=f.size_bytes,
            scan_status=scan,
        )
        items.append(entry)
        if upload == "failed" or scan in _BLOCKED:
            texts[f.id] = "unavailable: file_blocked"
        elif not _final(item):
            texts[f.id] = "unavailable: file_not_ready"
        else:
            fetched, _ = await fetch_attachments(client, [entry], files_dir)
            texts.update(fetched)
    return items, texts


def describe_followup_files(
    items: Sequence[RunContextAttachment], texts: Mapping[UUID, str]
) -> list[str]:
    r"""One text ``<name> (<type>): <text>`` per attachment of a follow-up, in position order.

    Control characters of the name become spaces, so a name never breaks the line of the list.

    Args:
        items: The attachments of `fetch_followup_files`.
        texts: Their texts, with the transcriber's ``not transcribed: <code>`` line if any.

    Returns:
        The texts for `claude_code_runner.followups.build_followup_line`.

    Examples:
        >>> from uuid import UUID
        >>> item = RunContextAttachment(UUID(int=1), "a\nb.png", "image/png", 1, "clean")
        >>> describe_followup_files([item], {UUID(int=1): "unavailable: file_blocked"})
        ['a b.png (image/png): unavailable: file_blocked']
    """
    return [
        f"{_CONTROL.sub(' ', item.original_name)} ({item.mime_type}): {texts[item.file_id]}"
        for item in items
    ]
