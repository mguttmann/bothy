# SPDX-License-Identifier: Apache-2.0
r"""Reports without a trigger: Hermes cron jobs and other proactive posts (09 14, 17.9).

Hermes sends a message without an invocation in two ways, and both end here:

* in the gateway process, through the platform's ``send`` (a cron job of the gateway's
  scheduler, a notice of a background task): `bothy_hermes_adapter.handler.HermesHandler`
  posts with the adapter loop's client;
* in another process (``hermes cron run``, ``hermes send``), through the plugin's
  ``standalone_sender_fn``: `standalone_send` opens a client of its own with the profile's
  platform configuration and posts once.

The target is Hermes' ``chat_id`` and optional ``thread_id`` (``deliver=<key>`` takes the
platform's home channel, ``<key>:<channel>`` a channel, ``<key>:<channel>:<thread>`` a fixed
thread). Without a thread the post follows the channel's kind (`post_report`): a forum gets a
new post per report (``createThread`` with the title from the report's first line and the report
as its first message), a text channel gets the report in its main thread. Every write is the
agent's own (no run, no invocation token) and the server checks it with the agent's rights:
``write`` on the channel and ``rl.message`` (09 14). A refusal comes back as a failure result
with the problem code; nothing is retried by hand, and no log line holds a content.

Examples:
    >>> report_title("## Nightly build\\nAll green")
    'Nightly build'
    >>> parse_target("unassigned", None)
    Traceback (most recent call last):
    ...
    bothy_hermes_adapter.reports.ReportError: no report channel assigned
"""

from __future__ import annotations

import logging
from collections.abc import Awaitable, Callable, Mapping
from dataclasses import dataclass
from typing import Any, Final
from uuid import UUID

from bothy_sdk.auth import Credentials, FileDpopKey
from bothy_sdk.client import Client
from bothy_sdk.errors import ProblemError
from bothy_sdk.generated.api.channels import get_channel
from bothy_sdk.generated.api.messages import create_message
from bothy_sdk.generated.api.threads import create_thread
from bothy_sdk.generated.models import (
    ChannelKind,
    ChannelView,
    Message,
    MessageCreate,
    ThreadCreate,
    ThreadCreated,
)
from bothy_sdk.generated.types import Unset
from bothy_sdk.ids import uuid7

from bothy_hermes_adapter import __version__
from bothy_hermes_adapter.config import ConfigError, load_settings, load_token
from bothy_hermes_adapter.context_format import cut_utf8

__all__ = [
    "STANDALONE_AGENT",
    "TITLE_MAX",
    "ReportError",
    "ReportTarget",
    "SendInThread",
    "direct_send",
    "parse_target",
    "post_report",
    "recovered_reply",
    "report_title",
    "standalone_send",
]

_LOG = logging.getLogger("bothy_hermes_adapter")

TITLE_MAX: Final = 200
"""``thread.title_max_chars`` (03): the longest title of a new forum post."""

STANDALONE_AGENT: Final = "hermes-adapter-standalone"
"""The adapter name in the ``User-Agent`` of the standalone sender (06 2)."""

UNASSIGNED: Final = "unassigned"
"""The installer's placeholder home channel (no report channel assigned)."""

SendInThread = Callable[[UUID, str], Awaitable[UUID | None]]
"""Posts a text into a thread as the agent; returns the message ID once it is stored."""


class ReportError(Exception):
    """A report that was not posted; the text is the reason Hermes shows (never a content)."""


@dataclass(frozen=True)
class ReportTarget:
    """Where a report goes.

    Attributes:
        channel_id: The channel (Hermes' ``chat_id``).
        thread_id: A fixed thread, or ``None`` to follow the channel's kind.
    """

    channel_id: UUID
    thread_id: UUID | None = None


RECOVERED_PREFIX = "\u267b\ufe0f Recovered reply"
"""The start of every notice of Hermes' delivery ledger before a reply it sends again."""
_RECOVERED_END = ":\n\n"


def recovered_reply(content: str) -> str | None:
    r"""The reply behind a notice of Hermes' delivery ledger, or ``None`` for any other text.

    Hermes records each final reply before the send and, when the send failed or the gateway
    restarted during it, sends it again with a notice that it may be a duplicate (its
    ``gateway.delivery_ledger``: restart, reconnect and rate limit notices). The notice is one
    line ending with a colon and an empty line.

    Examples:
        >>> recovered_reply("\u267b\ufe0f Recovered reply (restart), may be a duplicate:\n\nDone.")
        'Done.'
        >>> recovered_reply("Done.") is None
        True
    """
    if not content.startswith(RECOVERED_PREFIX):
        return None
    notice, end, reply = content.partition(_RECOVERED_END)
    if not end or "\n" in notice:
        return None
    return reply


def parse_target(chat_id: str, thread_id: str | None) -> ReportTarget:
    """Read Hermes' ``chat_id`` and ``thread_id`` as a report target.

    Args:
        chat_id: The channel ID, or the installer's placeholder.
        thread_id: A thread ID, or ``None``/empty.

    Returns:
        The target.

    Raises:
        ReportError: The placeholder (no report channel assigned) or an ID that is no UUID.
    """
    raw = str(chat_id or "").strip()
    if not raw or raw == UNASSIGNED:
        raise ReportError("no report channel assigned")
    try:
        channel = UUID(raw)
        thread = UUID(str(thread_id).strip()) if thread_id else None
    except ValueError:
        raise ReportError("the target is not a channel or thread ID of this server") from None
    return ReportTarget(channel, thread)


def report_title(text: str) -> str:
    """The title of a new forum post: the first non-empty line without Markdown heading marks.

    Args:
        text: The report.

    Returns:
        At most `TITLE_MAX` characters; ``"Report"`` when the report has no visible line.
    """
    for line in text.splitlines():
        stripped = line.strip().lstrip("#").strip()
        if stripped:
            return stripped[:TITLE_MAX].rstrip()
    return "Report"


async def post_report(
    client: Client,
    target: ReportTarget,
    text: str,
    *,
    max_bytes: int,
    send: SendInThread,
) -> UUID | None:
    """Post a report as the agent, without a run (09 14).

    Args:
        client: The agent's client.
        target: Channel and optional thread.
        text: The report (cut to ``max_bytes`` at a UTF-8 boundary).
        max_bytes: ``hermes_adapter.max_message_bytes``.
        send: Posts into an existing thread.

    Returns:
        The message ID, or the ID of the new forum post.

    Raises:
        ReportError: An empty report, a dm channel, or a refusal of the server (its code).
    """
    body, _ = cut_utf8(text.encode("utf-8"), max_bytes)
    if not body.strip():
        raise ReportError("the report is empty")
    try:
        with client.unbound():
            if target.thread_id is not None:
                return await send(target.thread_id, body)
            view = await client.call_as(
                ChannelView, get_channel.asyncio_detailed, channel_id=target.channel_id
            )
            channel = view.channel
            if channel.kind is ChannelKind.FORUM:
                return await _new_post(client, target.channel_id, body)
            main = channel.main_thread_id
            if channel.kind is not ChannelKind.TEXT or isinstance(main, Unset):
                raise ReportError("reports go to a text channel or a forum")
            return await send(main, body)
    except ProblemError as exc:
        _LOG.info("report refused code=%s", exc.code)
        raise ReportError(f"refused by the server: {exc.code}") from None


async def _new_post(client: Client, channel_id: UUID, body: str) -> UUID:
    """A forum post per report: ``createThread`` with the report as its first message."""
    request = ThreadCreate(
        title=report_title(body),
        first_message=MessageCreate(client_msg_id=uuid7(), content_md=body),
    )
    with client.idempotency_key(str(uuid7())):
        created = await client.call_as(
            ThreadCreated, create_thread.asyncio_detailed, channel_id=channel_id, body=request
        )
    return created.thread.id


async def direct_send(client: Client, thread_id: UUID, text: str) -> UUID | None:
    """One ``createMessage`` with a fresh ``client_msg_id`` (the client repeats it on retries).

    Args:
        client: The agent's client (unbound: the message belongs to no run).
        thread_id: The thread.
        text: The Markdown text.

    Returns:
        The message ID.
    """
    body = MessageCreate(client_msg_id=uuid7(), content_md=text)
    message = await client.call_as(
        Message, create_message.asyncio_detailed, thread_id=thread_id, body=body
    )
    return message.id


async def standalone_send(
    pconfig: Any,
    chat_id: str,
    message: str,
    thread_id: str | None = None,
    media_files: Any = None,
    **_: Any,
) -> dict[str, Any]:
    """Hermes' ``standalone_sender_fn``: one report from a process without the gateway (17.9).

    The settings, the token file and the DPoP key come from the platform section Hermes hands
    over (``PlatformConfig.extra``), never from the environment (17.2).

    Args:
        pconfig: Hermes' ``PlatformConfig`` of the platform.
        chat_id: The channel.
        message: The report.
        thread_id: A fixed thread, if any.
        media_files: Attachments; refused (a report carries text only), nothing is sent.

    Returns:
        ``{"success": True, "message_id": ...}`` or ``{"error": <reason>}``.
    """
    if media_files:
        return {"error": "attachments are not supported by this server adapter yet"}
    try:
        target = parse_target(chat_id, thread_id)
        extra: Mapping[str, Any] = dict(getattr(pconfig, "extra", None) or {})
        settings = load_settings(extra)
        credentials = Credentials(
            load_token(settings.token_file),
            FileDpopKey.load(settings.dpop_key_file) if settings.dpop_key_file else None,
        )
    except ReportError as exc:
        return {"error": str(exc)}
    except (ConfigError, OSError, ValueError) as exc:
        _LOG.error("report not sent: configuration %s", type(exc).__name__)
        return {"error": "the platform configuration is not usable"}
    try:
        async with Client(
            settings.server_url,
            credentials,
            settings=settings.sdk,
            user_agent=f"{STANDALONE_AGENT}/{__version__}",
        ) as client:

            async def send(thread: UUID, text: str) -> UUID | None:
                return await direct_send(client, thread, text)

            posted = await post_report(
                client, target, message, max_bytes=settings.max_message_bytes, send=send
            )
    except ReportError as exc:
        return {"error": str(exc)}
    except Exception as exc:  # a transport failure after the client's retries
        _LOG.warning("report not sent: %s", type(exc).__name__)
        return {"error": f"the server was not reachable ({type(exc).__name__})"}
    return {"success": True, "message_id": str(posted) if posted else None}
