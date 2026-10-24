# SPDX-License-Identifier: Apache-2.0
r"""The title proposal of a new session (09 12 point 5, 18.4, AG-19).

The runner prompt asks for a first line ``TITLE: <title>`` in the first reply of a new or
unstarted session. The runner removes it from the reply and, after the first completed run,
proposes it once: ``getThread``, and when ``title_source`` is ``auto``, ``setThreadTitle`` with
``If-Match``. ``409 title_manual``, ``412 version_conflict`` and every other failure are logged
by code and never retried; the run still completes.

Examples:
    >>> split_title("TITLE: Fix the login form\\nDone.", True)
    ('Fix the login form', 'Done.')
"""

from __future__ import annotations

import json
import logging
from uuid import UUID

import httpx
from bothy_sdk.client import Client
from bothy_sdk.errors import ProblemError
from bothy_sdk.generated.api.threads import get_thread, set_thread_title
from bothy_sdk.generated.models import Thread, ThreadTitlePut

__all__ = ["propose_title", "split_title"]

_LOG = logging.getLogger("claude_code_runner")
_PREFIX = "TITLE:"


def split_title(text: str, requested: bool) -> tuple[str | None, str]:
    """Split a leading ``TITLE:`` line from a text.

    Leading empty lines are skipped; only the first non-empty line can be the title line, and
    only when the request was given.

    Args:
        text: The reply or excerpt.
        requested: Whether the system prompt asked for the line.

    Returns:
        ``(title or None, rest)``; the rest has its leading line breaks removed when a line was
        taken.
    """
    if not requested:
        return None, text
    stripped = text.lstrip("\r\n")
    first, _, rest = stripped.partition("\n")
    if not first.startswith(_PREFIX):
        return None, text
    title = first[len(_PREFIX) :].strip()
    return (title or None), rest.lstrip("\r\n")


async def propose_title(client: Client, thread_id: UUID, title: str) -> str:
    """Propose ``title`` for the thread once (inside the run binding of the caller).

    Args:
        client: The SDK client.
        thread_id: The run's thread.
        title: The title of the ``TITLE:`` line.

    Returns:
        ``set``, ``not_auto`` or the problem code of the failure.
    """
    try:
        thread = await client.call_as(Thread, get_thread.asyncio_detailed, thread_id=thread_id)
        if str(thread.title_source) != "auto":
            return "not_auto"
        await client.call_as(
            Thread,
            set_thread_title.asyncio_detailed,
            thread_id=thread_id,
            body=ThreadTitlePut(title=title),
            if_match=f'"{thread.version}"',
        )
    except ProblemError as exc:
        _LOG.info(
            json.dumps({"event": "title_not_set", "thread_id": str(thread_id), "code": exc.code})
        )
        return exc.code
    except httpx.TransportError:
        _LOG.info(
            json.dumps({"event": "title_not_set", "thread_id": str(thread_id), "code": "transport"})
        )
        return "transport"
    return "set"
