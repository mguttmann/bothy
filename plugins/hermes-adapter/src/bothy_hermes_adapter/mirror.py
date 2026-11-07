# SPDX-License-Identifier: Apache-2.0
r"""The mirror of Hermes sessions outside the platform (09 17.13, design hermes-session-mirror).

A Hermes session that is not started from the server (one another program drives over ``hermes
acp``, a ``hermes`` CLI session, a cron job) runs in a process of the profile where only the
plugin entry is loaded. With ``hermes_adapter.mirror_channel`` and
``hermes_adapter.mirror_platforms`` set, the plugin entry passes the hooks of the listed
platforms here, and the session shows as a post of its own in that forum:

* ``pre_llm_call`` puts the turn's request on the queue; the first request of a session opens
  the post (``thread.create``, title ``<platform>: <first line of the prompt>``), every later one
  goes into the same post. The session ID and the post are kept in
  ``<profile home>/<platform key>/mirror.sqlite3`` (`MirrorMap`), so a restart, a resume and a
  compression (a new session ID whose parent is mapped) stay in the post.
* ``post_tool_call`` counts the turn's tools by name; ``post_llm_call`` puts the reply on the
  queue with one line of tool names and counts; ``on_session_end`` of a turn without a reply
  says that the turn ended without an answer.
* The hooks never wait and never raise: one worker thread with an event loop of its own posts the
  events in order with a client of its own (``User-Agent`` `MIRROR_AGENT`). A full queue, a
  refusal or an unreachable server drops the event and logs a code or an error type, never a
  content; a mirror channel that is no forum, or a refused ``thread.create`` (``403``, ``404``),
  turns the mirror off for the process.

A person's message in a mirror post never reaches the foreign session: the gateway's handler
answers it with `MIRROR_READ_ONLY` once per post (`MirrorMap.first_notice`).

Examples:
    >>> request_text("acp", "Fix the build\nnow", 0)
    '**Request** (acp session)\n\n> Fix the build\n> now'
    >>> post_title("cli", "  Deploy   the site  ")
    'cli: Deploy the site'
"""

from __future__ import annotations

import asyncio
import atexit
import contextlib
import logging
import queue
import sqlite3
import threading
import time
from collections import Counter
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Final
from uuid import UUID

from bothy_sdk.auth import Credentials, FileDpopKey
from bothy_sdk.client import Client
from bothy_sdk.errors import ProblemError
from bothy_sdk.generated.api.channels import get_channel
from bothy_sdk.generated.api.threads import create_thread
from bothy_sdk.generated.models import (
    ChannelKind,
    ChannelView,
    MessageCreate,
    ThreadCreate,
    ThreadCreated,
)
from bothy_sdk.ids import uuid7

from bothy_hermes_adapter import __version__
from bothy_hermes_adapter.config import (
    AdapterSettings,
    ConfigError,
    load_settings,
    load_token,
    platform_extra,
)
from bothy_hermes_adapter.context_format import cut_utf8
from bothy_hermes_adapter.reports import direct_send

__all__ = [
    "EXIT_FLUSH_S",
    "MIRROR_AGENT",
    "MIRROR_DB",
    "MIRROR_READ_ONLY",
    "QUEUE_MAX",
    "TITLE_CHARS",
    "Mirror",
    "MirrorConfig",
    "MirrorMap",
    "mirror_config",
    "mirror_hook",
    "post_title",
    "process_mirror",
    "request_text",
    "reset_process_mirrors",
]

_LOG = logging.getLogger("bothy_hermes_adapter")

MIRROR_AGENT: Final = "hermes-adapter-mirror"
"""The adapter name in the ``User-Agent`` of the mirror's client (06 2)."""
MIRROR_DB: Final = "mirror.sqlite3"
"""The store of the mirror below the adapter's state directory ``<profile home>/<platform key>``."""
QUEUE_MAX: Final = 256
"""The most events waiting for the worker; a hook drops the event beyond it."""
EXIT_FLUSH_S: Final = 5.0
"""How long the exit of the process waits for the worker to post what is queued."""
TITLE_CHARS: Final = 80
"""The longest title of a mirror post."""
TOOL_NAMES_MAX: Final = 20
"""The most tool names of the tool line; more are counted as ``+N more``."""
MIRROR_READ_ONLY: Final = (
    "This chat mirrors a Hermes session that runs outside this app. It is read-only: messages "
    "here do not reach that session, and the agent does not answer them here. Start a new chat "
    "with the agent to talk to it."
)
"""The notice a person gets once per mirror post (09 17.13)."""

_REQUEST_HEAD: Final = "**Request** ({platform} session)"
_NO_TEXT_ANSWER: Final = "The turn ended without a text answer."
_NO_ANSWER: Final = "The turn ended without an answer ({reason})."
_CUT_NOTE: Final = "\n\n(cut)"
_TOOL_NAME_CHARS: Final = 64
_OFF_STATUSES: Final = frozenset({403, 404})
_GONE_STATUSES: Final = frozenset({404, 410})
_DROP_LOG_EVERY: Final = 100
_SCHEMA: Final = """
CREATE TABLE IF NOT EXISTS mirror_threads (
    session_id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    platform TEXT NOT NULL,
    created_at REAL NOT NULL
);
CREATE INDEX IF NOT EXISTS mirror_threads_thread ON mirror_threads (thread_id);
CREATE TABLE IF NOT EXISTS mirror_notices (
    thread_id TEXT PRIMARY KEY,
    noticed_at REAL NOT NULL
);
"""


@dataclass(frozen=True)
class MirrorConfig:
    """The mirror of a profile.

    Attributes:
        channel: The forum the posts go to (``hermes_adapter.mirror_channel``).
        platforms: The foreign platform keys to mirror (``hermes_adapter.mirror_platforms``).
    """

    channel: UUID
    platforms: frozenset[str]


def mirror_config(settings: AdapterSettings) -> MirrorConfig | None:
    """The mirror of the settings, or ``None`` when it is off (the default).

    Args:
        settings: The adapter settings.

    Returns:
        The mirror configuration.
    """
    if settings.mirror_channel is None or not settings.mirror_platforms:
        return None
    return MirrorConfig(settings.mirror_channel, settings.mirror_platforms)


class MirrorMap:
    """Session ID to mirror post, and the posts that showed the read-only notice.

    One SQLite file shared by the processes of the profile (the gateway reads it, the processes
    of the foreign sessions write it); every call opens its own connection with a busy timeout.

    Args:
        path: ``<profile home>/<platform key>/mirror.sqlite3``.
    """

    def __init__(self, path: Path) -> None:
        """Remember the path; the file is created at the first write."""
        self.path = path

    def _connect(self) -> sqlite3.Connection:
        self.path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        fresh = not self.path.exists()
        conn = sqlite3.connect(self.path, timeout=5.0, isolation_level=None)
        if fresh:
            self.path.chmod(0o600)
        conn.executescript(_SCHEMA)
        return conn

    def thread_for(self, session_id: str) -> UUID | None:
        """The post of a session, or ``None``."""
        if not self.path.exists():
            return None
        with contextlib.closing(self._connect()) as conn:
            row = conn.execute(
                "SELECT thread_id FROM mirror_threads WHERE session_id = ?", (session_id,)
            ).fetchone()
        return UUID(row[0]) if row else None

    def bind(self, session_id: str, thread_id: UUID, platform: str) -> None:
        """Map a session to a post (a later bind of the same session replaces it)."""
        with contextlib.closing(self._connect()) as conn:
            conn.execute(
                "INSERT OR REPLACE INTO mirror_threads VALUES (?, ?, ?, ?)",
                (session_id, str(thread_id), platform, time.time()),
            )

    def forget(self, thread_id: UUID) -> None:
        """Forget a post that is gone; its sessions open a new one at their next turn."""
        with contextlib.closing(self._connect()) as conn:
            conn.execute("DELETE FROM mirror_threads WHERE thread_id = ?", (str(thread_id),))

    def is_mirror(self, thread_id: UUID) -> bool:
        """Whether a thread is a mirror post."""
        if not self.path.exists():
            return False
        with contextlib.closing(self._connect()) as conn:
            row = conn.execute(
                "SELECT 1 FROM mirror_threads WHERE thread_id = ? LIMIT 1", (str(thread_id),)
            ).fetchone()
        return row is not None

    def first_notice(self, thread_id: UUID) -> bool:
        """Mark the read-only notice of a post; ``True`` only the first time."""
        with contextlib.closing(self._connect()) as conn:
            cursor = conn.execute(
                "INSERT OR IGNORE INTO mirror_notices VALUES (?, ?)", (str(thread_id), time.time())
            )
            return cursor.rowcount == 1


@dataclass
class _TurnState:
    tools: Counter[str] = field(default_factory=Counter)
    replied: bool = False


@dataclass(frozen=True)
class _Event:
    kind: str  # request, reply or ended
    session: str
    platform: str
    text: str = ""
    parent: str = ""
    skipped: int = 0
    tools: tuple[tuple[str, int], ...] = ()
    reason: str | None = None


def _text(value: Any) -> str:
    return value.strip() if isinstance(value, str) else ""


def _prompt(message: Any) -> tuple[str, int]:
    """The text of a prompt and the number of parts that are no text (images, files)."""
    if isinstance(message, str):
        return message, 0
    parts = [message] if isinstance(message, dict) else message
    if not isinstance(parts, list | tuple):
        return "", 0
    texts: list[str] = []
    skipped = 0
    for part in parts:
        if isinstance(part, str):
            texts.append(part)
        elif isinstance(part, dict) and part.get("type") == "text":
            texts.append(str(part.get("text") or ""))
        else:
            skipped += 1
    return "\n".join(t for t in texts if t), skipped


def request_text(platform: str, text: str, skipped: int) -> str:
    """The request message: a head line and the prompt as a quote.

    Args:
        platform: The foreign platform key.
        text: The prompt's text.
        skipped: Parts of the prompt that are not mirrored.

    Returns:
        The Markdown text.
    """
    lines = text.strip().splitlines() or ["(empty)"]
    quote = "\n".join(f"> {line}".rstrip() for line in lines)
    body = f"{_REQUEST_HEAD.format(platform=platform)}\n\n{quote}"
    if skipped:
        body += f"\n\n({skipped} part{'s' if skipped > 1 else ''} not mirrored)"
    return body


def post_title(platform: str, text: str) -> str:
    """The title of a new mirror post: the platform and the prompt's first line.

    Args:
        platform: The foreign platform key.
        text: The prompt's text.

    Returns:
        At most `TITLE_CHARS` characters, cut at a word.
    """
    line = next((" ".join(ln.split()) for ln in text.splitlines() if ln.strip()), "")
    title = f"{platform}: {line}" if line else f"{platform} session"
    if len(title) <= TITLE_CHARS:
        return title
    cut = title[: TITLE_CHARS - 3]
    if " " in cut[len(platform) + 2 :]:
        cut = cut[: cut.rindex(" ")]
    return cut.rstrip() + "..."


def _tool_line(tools: tuple[tuple[str, int], ...]) -> str:
    if not tools:
        return ""
    shown = [f"{name[:_TOOL_NAME_CHARS]} x{count}" for name, count in tools[:TOOL_NAMES_MAX]]
    more = len(tools) - TOOL_NAMES_MAX
    if more > 0:
        shown.append(f"+{more} more")
    return "Tools: " + ", ".join(shown)


class Mirror:
    """The session mirror of one process.

    Args:
        settings: The adapter settings of the profile (server, token, message limit).
        config: The mirror's forum and platforms.
        mapping: The store of session to post.
        queue_max: The most queued events.
        autostart: Start the worker at once (tests start it later with `start`).
    """

    def __init__(
        self,
        settings: AdapterSettings,
        config: MirrorConfig,
        mapping: MirrorMap,
        *,
        queue_max: int = QUEUE_MAX,
        autostart: bool = True,
    ) -> None:
        """Create the mirror and, unless told otherwise, start its worker."""
        self.settings = settings
        self.config = config
        self.mapping = mapping
        self.dropped = 0
        self.disabled = False
        self._queue: queue.Queue[_Event | None] = queue.Queue(maxsize=queue_max)
        self._lock = threading.Lock()
        self._turns: dict[str, _TurnState] = {}
        self._thread: threading.Thread | None = None
        self._forum_checked = False
        self._closed = False
        if autostart:
            self.start()

    # ---- the hook side (Hermes' agent threads) ----------------------------------------------

    def hook(self, name: str, kwargs: Any) -> None:
        """Take one plugin hook; never raises, never waits.

        Args:
            name: The hook.
            kwargs: Its arguments.
        """
        try:
            self._hook(name, kwargs)
        except Exception as exc:  # the foreign session must never fail because of the mirror
            _LOG.warning("mirror: hook %s failed: %s", name, type(exc).__name__)

    def _hook(self, name: str, kwargs: Any) -> None:
        if self.disabled or self._closed:
            return
        session = _text(kwargs.get("session_id"))
        if not session:
            return
        if name == "post_tool_call":
            tool = _text(kwargs.get("tool_name"))
            with self._lock:
                state = self._turns.get(session)
                if state is not None and tool:
                    state.tools[tool] += 1
            return
        platform = _text(kwargs.get("platform")).lower()
        if platform not in self.config.platforms:
            return
        if name == "pre_llm_call":
            text, skipped = _prompt(kwargs.get("user_message"))
            with self._lock:
                self._turns[session] = _TurnState()
            parent = _text(kwargs.get("parent_session_id"))
            self._put(_Event("request", session, platform, text, parent, skipped))
        elif name == "post_llm_call":
            with self._lock:
                state = self._turns.get(session) or _TurnState()
                state.replied = True
                tools = tuple(state.tools.items())
                state.tools.clear()
            answer = kwargs.get("assistant_response")
            text = answer if isinstance(answer, str) else ""
            self._put(_Event("reply", session, platform, text, tools=tools))
        elif name == "on_session_end":
            with self._lock:
                state = self._turns.pop(session, None)
            if state is None or state.replied:
                return
            reason = None
            if kwargs.get("interrupted"):
                reason = "interrupted"
            elif kwargs.get("failed"):
                reason = "failed"
            self._put(
                _Event("ended", session, platform, tools=tuple(state.tools.items()), reason=reason)
            )

    def _put(self, event: _Event) -> None:
        try:
            self._queue.put_nowait(event)
        except queue.Full:
            self.dropped += 1
            if self.dropped % _DROP_LOG_EVERY == 1:
                _LOG.warning("mirror: queue full, events dropped=%s", self.dropped)

    # ---- the worker -----------------------------------------------------------------------

    def start(self) -> None:
        """Start the worker thread (once)."""
        if self._thread is not None:
            return
        self._thread = threading.Thread(target=self._run, name="bothy-mirror", daemon=True)
        self._thread.start()

    def idle(self) -> bool:
        """Whether the worker posted everything queued so far."""
        return self._queue.unfinished_tasks == 0

    def close(self, timeout: float = EXIT_FLUSH_S) -> None:
        """Let the worker post what is queued for at most ``timeout`` seconds, then stop it.

        Args:
            timeout: Seconds.
        """
        if self._closed:
            return
        self._closed = True
        thread = self._thread
        if thread is None:
            return
        deadline = time.monotonic() + timeout
        with contextlib.suppress(queue.Full):
            self._queue.put(None, timeout=max(0.0, deadline - time.monotonic()))
        thread.join(max(0.0, deadline - time.monotonic()))
        left = self._queue.qsize()
        if thread.is_alive() and left:
            _LOG.warning("mirror: events not posted at exit=%s", left)

    def _run(self) -> None:
        loop = asyncio.new_event_loop()
        try:
            loop.run_until_complete(self._serve())
        except Exception as exc:  # the worker ends; the hooks keep dropping into a full queue
            _LOG.error("mirror: worker ended: %s", type(exc).__name__)
        finally:
            loop.close()

    async def _serve(self) -> None:
        client: Client | None = None
        try:
            while True:
                event = self._queue.get()
                try:
                    if event is None:
                        return
                    if self.disabled:
                        self.dropped += 1
                        continue
                    if client is None:
                        client = self._client()
                        if client is None:
                            self.dropped += 1
                            continue
                    await self._handle(client, event)
                finally:
                    self._queue.task_done()
        finally:
            if client is not None:
                with contextlib.suppress(Exception):
                    await client.aclose()

    def _client(self) -> Client | None:
        settings = self.settings
        try:
            credentials = Credentials(
                load_token(settings.token_file),
                FileDpopKey.load(settings.dpop_key_file) if settings.dpop_key_file else None,
            )
        except Exception as exc:  # a token or key that does not load: the mirror is off
            self._turn_off(f"configuration {type(exc).__name__}")
            return None
        return Client(
            settings.server_url,
            credentials,
            settings=settings.sdk,
            user_agent=f"{MIRROR_AGENT}/{__version__}",
        )

    def _turn_off(self, why: str) -> None:
        if not self.disabled:
            self.disabled = True
            _LOG.error("mirror: turned off for this process: %s", why)

    def _drop(self, why: str) -> None:
        self.dropped += 1
        _LOG.info("mirror: event dropped: %s", why)

    async def _handle(self, client: Client, event: _Event) -> None:
        thread = self.mapping.thread_for(event.session)
        if thread is None and event.kind == "request" and event.parent:
            thread = self.mapping.thread_for(event.parent)
            if thread is not None:
                self.mapping.bind(event.session, thread, event.platform)
        if event.kind == "request":
            body = self._fit(request_text(event.platform, event.text, event.skipped))
        elif event.kind == "reply":
            body = self._fit(self._reply_text(event))
        else:
            reason = _NO_ANSWER.format(reason=event.reason) if event.reason else _NO_TEXT_ANSWER
            body = self._fit("\n\n".join(p for p in (reason, _tool_line(event.tools)) if p))
        if thread is None and event.kind != "request":
            self._drop("no post for the session")
            return
        try:
            with client.unbound():
                if thread is None:
                    await self._open_post(client, event, body)
                else:
                    await direct_send(client, thread, body)
        except ProblemError as exc:
            if thread is None and exc.status in _OFF_STATUSES:
                self._turn_off(f"code={exc.code}")
            elif thread is not None and exc.status in _GONE_STATUSES:
                self.mapping.forget(thread)
            self._drop(f"code={exc.code}")
        except Exception as exc:  # a transport failure after the client's retries
            self._drop(type(exc).__name__)

    async def _open_post(self, client: Client, event: _Event, body: str) -> None:
        channel = self.config.channel
        if not self._forum_checked:
            view = await client.call_as(
                ChannelView, get_channel.asyncio_detailed, channel_id=channel
            )
            if view.channel.kind is not ChannelKind.FORUM:
                self._turn_off("the mirror channel is no forum")
                raise _NotPosted
            self._forum_checked = True
        request = ThreadCreate(
            title=post_title(event.platform, event.text),
            first_message=MessageCreate(client_msg_id=uuid7(), content_md=body),
        )
        with client.idempotency_key(str(uuid7())):
            created = await client.call_as(
                ThreadCreated, create_thread.asyncio_detailed, channel_id=channel, body=request
            )
        self.mapping.bind(event.session, created.thread.id, event.platform)

    def _reply_text(self, event: _Event) -> str:
        text = event.text.strip() or _NO_TEXT_ANSWER
        line = _tool_line(event.tools)
        return f"{text}\n\n{line}" if line else text

    def _fit(self, text: str) -> str:
        """Cut to ``hermes_adapter.max_message_bytes`` with a closing note (09 14)."""
        limit = self.settings.max_message_bytes
        data = text.encode("utf-8")
        if len(data) <= limit:
            return text
        head, _ = cut_utf8(data, max(0, limit - len(_CUT_NOTE.encode("utf-8"))))
        return head + _CUT_NOTE


class _NotPosted(Exception):  # noqa: N818 - an outcome, already logged
    """The event was not posted; the reason is logged already."""


# ---- the mirror of this process (the plugin entry's way in) -----------------------------------

# One mirror per profile home and platform key: a Hermes gateway that serves several profiles
# runs their sessions in one process, so a mirror read once per process would send one profile's
# sessions to another profile's forum (09 17.13 point 2).
_PROCESS: dict[tuple[str, str], Mirror | None] = {}
_PROCESS_LOCK = threading.Lock()


def _process_home() -> Path:
    """The Hermes home of the profile whose session calls the hook (a context-local value)."""
    from bothy_hermes_adapter import hermes_config  # noqa: PLC0415 - imports Hermes, at the hook

    return hermes_config.current_home()


def _load_process_mirror(home: Path, platform_key: str) -> Mirror | None:
    """The mirror of a profile from its platform section, or ``None`` when off.

    Args:
        home: The profile's Hermes home; Hermes points its config reader at it for the session.
        platform_key: `hermes_adapter.platform_key`.

    Returns:
        The mirror.
    """
    from bothy_hermes_adapter import hermes_config  # noqa: PLC0415 - imports Hermes, at the hook

    extra = platform_extra(hermes_config.read_config(), platform_key)
    if not any(k.startswith("hermes_adapter.mirror_") for k in extra):
        return None
    settings = load_settings(extra)
    config = mirror_config(settings)
    if config is None:
        return None
    state = home / settings.platform_key
    state.mkdir(mode=0o700, parents=True, exist_ok=True)
    found = Mirror(settings, config, MirrorMap(state / MIRROR_DB))
    atexit.register(found.close, EXIT_FLUSH_S)
    return found


def process_mirror(platform_key: str) -> Mirror | None:
    """The mirror of the calling profile, read once per profile home; ``None`` when off.

    Args:
        platform_key: `hermes_adapter.platform_key`.

    Returns:
        The mirror.
    """
    try:
        home = _process_home()
    except Exception as exc:  # no profile to read: the mirror is off; the session runs on
        _LOG.error("mirror: off: the profile home does not resolve: %s", type(exc).__name__)
        return None
    key = (str(home), platform_key)
    with _PROCESS_LOCK:
        if key not in _PROCESS:
            try:
                _PROCESS[key] = _load_process_mirror(home, platform_key)
            except Exception as exc:  # the mirror is off; the session runs on
                _PROCESS[key] = None
                # A ConfigError names the key only; any other error is named by its type.
                why = str(exc) if isinstance(exc, ConfigError) else type(exc).__name__
                _LOG.error("mirror: off: the configuration does not load: %s", why)
        return _PROCESS[key]


def mirror_hook(platform_key: str, name: str, kwargs: Any) -> None:
    """A plugin hook of a foreign platform (`bothy_hermes_adapter.entry`); never raises.

    Args:
        platform_key: `hermes_adapter.platform_key`.
        name: The hook.
        kwargs: Its arguments.
    """
    found = process_mirror(platform_key)
    if found is not None:
        found.hook(name, kwargs)


def reset_process_mirrors() -> None:
    """Close and forget the mirrors of this process (tests)."""
    with _PROCESS_LOCK:
        found = [m for m in _PROCESS.values() if m is not None]
        _PROCESS.clear()
    for item in found:
        item.close(1.0)
