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
from collections.abc import Sequence
from pathlib import Path
from uuid import UUID

import httpx
from bothy_sdk.client import Client
from bothy_sdk.errors import ProblemError
from bothy_sdk.generated.api.files import download_file
from bothy_sdk.generated.models import RunContextAttachment

from claude_code_runner.statedir import ensure_private_dir, write_private

__all__ = ["FETCHED", "fetch_attachments"]

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
