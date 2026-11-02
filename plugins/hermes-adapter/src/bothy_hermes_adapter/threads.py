# SPDX-License-Identifier: Apache-2.0
"""Thread actions of the agent: set a thread's title, mark it done, reopen it (09 14).

A support desk flow keeps one ticket per forum post: the agent opens the post with a report
(`bothy_hermes_adapter.reports`), shows the ticket's status in the title, marks the post done
when the ticket is closed (the state the app's Done button sets, ``thread.close``) and reopens
it when an answer needs work again (``thread.reopen``). Closing never blocks writing: a new
message of a person or of the agent in a done post reopens it on the server (01 6.1), and a
person's message there starts a run like in any open post.

Every call is the agent's own, without a run (no invocation token), like a report; the server
checks it with the agent's rights (``write`` on the thread, which the operator of a channel has
by the operator rule, 07 2.9). The title follows 09 section 12: the operator agent may replace a
title whose source is ``auto`` (its own post's title stays ``auto``), a title a person set gives
``title_manual``.

Two ways in, both with the profile's platform section (``PlatformConfig.extra``):

* `thread_action` for a process with Hermes' platform configuration or its ``extra`` mapping (a
  script of the profile, for example a support desk backend);
* ``python -m bothy_hermes_adapter thread close|reopen|title`` (`bothy_hermes_adapter.__main__`)
  for a skill or a shell, which prints one JSON line.

Examples:
    >>> sorted(ACTIONS)
    ['close', 'reopen', 'title']
"""

from __future__ import annotations

import logging
from collections.abc import Mapping
from typing import Any, Final
from uuid import UUID

from bothy_sdk.auth import Credentials, FileDpopKey
from bothy_sdk.client import Client
from bothy_sdk.errors import ProblemError
from bothy_sdk.generated.api.threads import (
    close_thread,
    get_thread,
    reopen_thread,
    set_thread_title,
)
from bothy_sdk.generated.models import Thread, ThreadTitlePut
from bothy_sdk.ids import uuid7

from bothy_hermes_adapter import __version__
from bothy_hermes_adapter.config import ConfigError, load_settings, load_token
from bothy_hermes_adapter.reports import TITLE_MAX

__all__ = [
    "ACTIONS",
    "THREAD_AGENT",
    "close",
    "reopen",
    "set_title",
    "thread_action",
]

_LOG = logging.getLogger("bothy_hermes_adapter")

ACTIONS: Final = frozenset({"close", "reopen", "title"})
"""The thread actions of `thread_action`."""

THREAD_AGENT: Final = "hermes-adapter-thread"
"""The adapter name in the ``User-Agent`` of `thread_action` (06 2)."""


async def set_title(client: Client, thread_id: UUID, title: str) -> Thread:
    """Set the thread's title with ``If-Match`` of a fresh ``getThread``.

    Args:
        client: The agent's client.
        thread_id: The thread.
        title: The new title (cut to `bothy_hermes_adapter.reports.TITLE_MAX` characters).

    Returns:
        The thread after the change.

    Raises:
        bothy_sdk.errors.ProblemError: The server's refusal, for example ``title_manual`` when a
            person set the title, or ``version_conflict`` when it changed in between.
    """
    with client.unbound():
        current = await client.call_as(Thread, get_thread.asyncio_detailed, thread_id=thread_id)
        return await client.call_as(
            Thread,
            set_thread_title.asyncio_detailed,
            thread_id=thread_id,
            body=ThreadTitlePut(title=title.strip()[:TITLE_MAX]),
            if_match=f'"{current.version}"',
        )


async def close(client: Client, thread_id: UUID) -> Thread:
    """Mark the thread done (``thread.close``); a done thread stays as it is.

    Args:
        client: The agent's client.
        thread_id: The thread.

    Returns:
        The thread.
    """
    with client.unbound(), client.idempotency_key(str(uuid7())):
        return await client.call_as(Thread, close_thread.asyncio_detailed, thread_id=thread_id)


async def reopen(client: Client, thread_id: UUID) -> Thread:
    """Reopen a done thread (``thread.reopen``); an open thread stays as it is.

    Args:
        client: The agent's client.
        thread_id: The thread.

    Returns:
        The thread.
    """
    with client.unbound(), client.idempotency_key(str(uuid7())):
        return await client.call_as(Thread, reopen_thread.asyncio_detailed, thread_id=thread_id)


def _result(thread: Thread) -> dict[str, Any]:
    return {
        "ok": True,
        "thread_id": str(thread.id),
        "state": str(thread.state.value),
        "title": thread.title,
        "title_source": str(thread.title_source.value),
    }


async def thread_action(
    config: Any, action: str, thread_id: str, title: str | None = None
) -> dict[str, Any]:
    """Run one thread action as the agent of a profile's platform section.

    The settings, the token file and the DPoP key come from the platform section (Hermes'
    ``PlatformConfig`` or its ``extra`` mapping), never from the environment (17.2).

    Args:
        config: ``PlatformConfig`` of the platform, or its ``extra`` mapping.
        action: ``close``, ``reopen`` or ``title``.
        thread_id: The thread.
        title: The new title (``title`` only).

    Returns:
        ``{"ok": True, "thread_id", "state", "title", "title_source"}``, or ``{"ok": False,
        "error": <reason>}`` with ``"code"`` and ``"status"`` of a refusal of the server. No
        content and no value of the configuration is ever part of an error.
    """
    if action not in ACTIONS:
        return {"ok": False, "error": f"unknown action: {action}"}
    if action == "title" and not (title or "").strip():
        return {"ok": False, "error": "the title is empty"}
    try:
        thread = UUID(str(thread_id).strip())
    except ValueError:
        return {"ok": False, "error": "the thread is not a thread ID of this server"}
    extra = config if isinstance(config, Mapping) else getattr(config, "extra", None)
    try:
        settings = load_settings(dict(extra or {}))
        credentials = Credentials(
            load_token(settings.token_file),
            FileDpopKey.load(settings.dpop_key_file) if settings.dpop_key_file else None,
        )
    except (ConfigError, OSError, ValueError) as exc:
        _LOG.error("thread action not run: configuration %s", type(exc).__name__)
        return {"ok": False, "error": "the platform configuration is not usable"}
    try:
        async with Client(
            settings.server_url,
            credentials,
            settings=settings.sdk,
            user_agent=f"{THREAD_AGENT}/{__version__}",
        ) as client:
            if action == "close":
                done = await close(client, thread)
            elif action == "reopen":
                done = await reopen(client, thread)
            else:
                done = await set_title(client, thread, str(title))
    except ProblemError as exc:
        _LOG.info("thread action %s refused code=%s", action, exc.code)
        return {
            "ok": False,
            "error": f"refused by the server: {exc.code}",
            "code": exc.code,
            "status": exc.status,
        }
    except Exception as exc:  # a transport failure after the client's retries
        _LOG.warning("thread action %s not run: %s", action, type(exc).__name__)
        return {"ok": False, "error": f"the server was not reachable ({type(exc).__name__})"}
    return _result(done)
