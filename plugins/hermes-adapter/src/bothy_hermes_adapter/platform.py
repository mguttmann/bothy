# SPDX-License-Identifier: Apache-2.0
"""The Hermes platform plugin: the only runtime module that imports Hermes (09 17.2, 17.3).

`ServerPlatform` subclasses Hermes's ``BasePlatformAdapter``. It implements
`bothy_hermes_adapter.runtime.Runtime` for the adapter logic and forwards every Hermes callback
of 17.3 to `bothy_hermes_adapter.handler.HermesHandler` on the adapter's event loop:

* ``connect`` loads the configuration from ``PlatformConfig.extra``, the token file and the DPoP
  key, reads the agent's ID with ``getMe`` (a principal of kind ``agent``, else no connect), and
  starts the SDK loop with the Hermes heartbeat (`bothy_hermes_adapter.heartbeat`); a ``4015``
  close (another process took the consumer over) reports the platform disconnected without a
  reconnect. ``disconnect`` stops the loop.
* A turn is ``handle_message`` with a ``MessageEvent`` whose source is built with
  ``build_source`` and the thread ID (one Hermes session per thread, 17.4) and whose ephemeral
  ``channel_prompt`` is the context's system part; it ends with ``on_processing_complete``.
* Steering, stop and session switches are the commands and busy inputs the M2-32 record proved:
  a follow-up as busy input (``display.busy_input_mode``), ``/stop``, ``/new`` with its
  confirmation answered through ``tools.slash_confirm.resolve``, and ``/resume <id> --all`` from
  the adapter's control sender.
* Session IDs and usage come from the plugin hooks ``pre_llm_call``, ``on_session_reset`` and
  ``post_api_request``; the thread of a hook or of ``set_status_text`` is the session's thread
  variable of the turn (``gateway.session_context``).

Settings come only from ``PlatformConfig.extra`` (never from the environment, 17.2); the state
lives in the directory named like the platform key below the profile home (mode ``0700``).
"""

from __future__ import annotations

import asyncio
import contextlib
import logging
import uuid
from collections.abc import Mapping
from pathlib import Path
from typing import Any, Final

from bothy_sdk.adapter import AdapterConfig
from bothy_sdk.auth import Credentials, FileDpopKey
from bothy_sdk.client import Client
from bothy_sdk.errors import ProblemError
from bothy_sdk.generated.api.me import get_me
from bothy_sdk.generated.models import Me
from gateway.config import Platform, PlatformConfig
from gateway.platforms.base import BasePlatformAdapter, ExecApprovalPrompt, SendResult
from gateway.platforms.event import MessageEvent, MessageType, ProcessingOutcome
from gateway.session_context import get_session_env
from hermes_cli import __version__ as hermes_version
from hermes_constants import get_hermes_home
from tools import slash_confirm
from tools.approval import resolve_gateway_approval
from tools.clarify_gateway import resolve_gateway_clarify

from bothy_hermes_adapter import __version__, cards, hermes_config
from bothy_hermes_adapter.config import (
    DEFAULT_MAX_MESSAGE_BYTES,
    AdapterSettings,
    ConfigError,
    load_settings,
    load_token,
)
from bothy_hermes_adapter.handler import HermesHandler
from bothy_hermes_adapter.heartbeat import ADAPTER_NAME, RUNTIME_NAME, HermesAdapterLoop
from bothy_hermes_adapter.installer import CONTROL_SENDER
from bothy_hermes_adapter.runtime import (
    CardKind,
    CardRequest,
    OpenCard,
    ResolvedCredential,
    SessionSwitch,
    TurnOutcome,
    TurnRequest,
    Usage,
)
from bothy_hermes_adapter.store import CallbackStore

__all__ = ["CONTROL_SENDER", "ServerPlatform", "make_register"]

_LOG = logging.getLogger("bothy_hermes_adapter")

_THREAD_VAR: Final = "HERMES_SESSION_THREAD_ID"
_OUTCOMES: Final = {
    ProcessingOutcome.SUCCESS: TurnOutcome.COMPLETED,
    ProcessingOutcome.FAILURE: TurnOutcome.FAILED,
    ProcessingOutcome.CANCELLED: TurnOutcome.CANCELLED,
}
_NOT_BUILT: Final = "not supported by this server adapter yet"
# How often the adapter looks at the gateway's startup restore before it connects; a poll of a
# flag, not a deadline (the wait ends when Hermes is ready).
_READY_POLL_S: Final = 0.1
_PLATFORMS: dict[str, ServerPlatform] = {}


class ServerPlatform(BasePlatformAdapter):
    """The platform adapter of one Hermes profile (one agent principal).

    Args:
        config: The platform section Hermes hands to the plugin.
        platform_key: `hermes_adapter.platform_key`, fixed by the plugin entry.
    """

    splits_long_messages = False
    supports_status_text = True

    def __init__(self, config: PlatformConfig, platform_key: str) -> None:
        """Validate the configuration; nothing connects yet."""
        super().__init__(config, Platform(platform_key))
        self.platform_key = platform_key
        self.settings: AdapterSettings | None = None
        self.config_error: str | None = None
        try:
            extra = dict(getattr(config, "extra", None) or {})
            self.settings = load_settings(extra)
        except ConfigError as exc:
            self.config_error = str(exc)
        if self.settings is not None:
            self.MAX_MESSAGE_LENGTH = self.settings.max_message_bytes
        self._loop: asyncio.AbstractEventLoop | None = None
        self._serve_task: asyncio.Task[Any] | None = None
        self._sdk: HermesAdapterLoop | None = None
        self.handler: HermesHandler | None = None
        self._turns: dict[str, asyncio.Future[TurnOutcome]] = {}
        self._sources: dict[uuid.UUID, tuple[str, str, str]] = {}
        self._sessions: dict[uuid.UUID, str] = {}
        self._parents: dict[uuid.UUID, str] = {}
        self._control: dict[uuid.UUID, int] = {}
        self._confirms: set[asyncio.Task[Any]] = set()
        self._store: CallbackStore | None = None

    # ---- lifecycle ---------------------------------------------------------------------------

    @property
    def enforces_own_access_policy(self) -> bool:
        """The adapter enforces its own access policy (09 17.7 layer 2)."""
        return True

    async def connect(self, *, is_reconnect: bool = False) -> bool:
        """Connect as the agent and start the SDK loop (09 17.3).

        Args:
            is_reconnect: Whether Hermes reconnects.

        Returns:
            Whether the platform is connected.
        """
        if self.settings is None:
            _LOG.error("configuration refused: %s", self.config_error)
            return False
        settings = self.settings
        self._loop = asyncio.get_running_loop()
        try:
            credentials = Credentials(
                load_token(settings.token_file),
                FileDpopKey.load(settings.dpop_key_file) if settings.dpop_key_file else None,
            )
            agent_id = await self._agent_id(settings, credentials)
        except ConfigError as exc:
            _LOG.error("configuration refused: %s", exc)
            return False
        except (ProblemError, OSError, ValueError) as exc:
            _LOG.error("connect failed: %s", type(exc).__name__)
            return False
        if agent_id is None:
            _LOG.error("the API token does not belong to an agent; not connected")
            return False
        state_dir = _state_dir(settings.platform_key)
        busy = hermes_config.busy_input_mode()
        store = CallbackStore(state_dir / "hermes.sqlite3")
        self._store = store
        for thread, (session_id, parent) in (await store.sessions()).items():
            self._sessions[thread] = session_id
            if parent:
                self._parents[thread] = parent
        self.handler = HermesHandler(self, settings, store, busy_mode=busy)
        config = AdapterConfig(
            base_url=settings.server_url,
            instance_id=settings.instance_id,
            agent_id=agent_id,
            credentials=credentials,
            consumer=settings.consumer,
            state_dir=state_dir,
            allowlist=True,
            adapter_name=ADAPTER_NAME,
            adapter_version=__version__,
            runtime_name=RUNTIME_NAME,
            runtime_version=str(hermes_version),
            capabilities=self.handler.capabilities(),
            settings=settings.sdk,
        )
        self._sdk = HermesAdapterLoop(config, self.handler)
        self._serve_task = asyncio.ensure_future(self._serve(store))
        _PLATFORMS[self.platform_key] = self
        self._mark_connected()
        return True

    async def _agent_id(
        self, settings: AdapterSettings, credentials: Credentials
    ) -> uuid.UUID | None:
        async with Client(settings.server_url, credentials, settings=settings.sdk) as client:
            me = await client.call_as(Me, get_me.asyncio_detailed)
        kind = str(me.principal.kind)
        return me.principal.id if kind == "agent" else None

    async def _hermes_ready(self) -> None:
        """Wait until the gateway finished its startup restore (17.10 record, harness note).

        While Hermes restores sessions after a start it queues every inbound message and reports
        the queued event as processed; a run started then would end without a turn. The adapter
        therefore connects to the server only after the restore, so nothing is delivered before.
        """
        while True:
            runner = getattr(self, "gateway_runner", None)
            if runner is not None and not getattr(runner, "_startup_restore_in_progress", False):
                return
            await asyncio.sleep(_READY_POLL_S)

    async def _serve(self, store: CallbackStore) -> None:
        loop = self._sdk
        if loop is None:
            return
        try:
            await self._hermes_ready()
            reason = await loop.run()
            if reason.code == 4015:
                _LOG.warning("another process took the consumer over; disconnected")
            else:
                _LOG.info("adapter loop stopped code=%s", reason.code)
        except Exception as exc:
            _LOG.error("adapter loop failed: %s", type(exc).__name__)
        finally:
            await store.close()
            self._mark_disconnected()

    async def disconnect(self) -> None:
        """Stop the SDK loop (close ``1000``); the server ends open runs ``interrupted``."""
        if self._sdk is not None:
            await self._sdk.stop()
        if self._serve_task is not None:
            with contextlib.suppress(asyncio.CancelledError, Exception):
                await self._serve_task
        _PLATFORMS.pop(self.platform_key, None)
        self._mark_disconnected()

    # ---- Runtime -----------------------------------------------------------------------------

    def _event(
        self,
        thread_id: uuid.UUID,
        text: str,
        *,
        user: tuple[str, str] | None = None,
        message_id: str | None = None,
        control: bool = True,
    ) -> MessageEvent:
        """A message of a thread for Hermes.

        ``control`` is True only for the adapter's own commands (``/stop``, ``/new``,
        ``/resume``): a person's text never runs a Hermes command or answers a Hermes prompt by
        itself (09 10.4, 11.5), so it reaches Hermes with ``allow_gateway_control`` off.
        """
        channel, user_id, user_name = self._sources[thread_id]
        if user is not None:
            user_id, user_name = user
        mid = message_id or f"ctl-{uuid.uuid4().hex}"
        source = self.build_source(
            chat_id=channel,
            chat_type="thread",
            thread_id=str(thread_id),
            user_id=user_id,
            user_name=user_name,
            message_id=mid,
        )
        return MessageEvent(
            text=text,
            message_type=MessageType.TEXT,
            source=source,
            message_id=mid,
            allow_gateway_control=control,
        )

    async def _dispatch(self, event: MessageEvent) -> TurnOutcome:
        if self._loop is None:
            return TurnOutcome.FAILED
        future: asyncio.Future[TurnOutcome] = self._loop.create_future()
        self._turns[str(event.message_id)] = future
        try:
            await self.handle_message(event)
            if not getattr(event, "_gateway_accepted", True) and not future.done():
                # Hermes dropped the event (busy lane, policy); no completion will come.
                return TurnOutcome.FAILED
            return await future
        finally:
            self._turns.pop(str(event.message_id), None)

    def _remember_source(self, turn: TurnRequest) -> None:
        self._sources[turn.thread_id] = (str(turn.channel_id), str(turn.user_id), turn.user_name)

    async def start_turn(self, turn: TurnRequest) -> TurnOutcome:
        """Start a Hermes turn for a run and wait for ``on_processing_complete`` (17.6)."""
        self._remember_source(turn)
        event = self._event(
            turn.thread_id, turn.user_text, message_id=str(turn.message_id), control=False
        )
        event.channel_prompt = turn.system_part
        event.user_id = str(turn.user_id)
        event.user_name = turn.user_name
        return await self._dispatch(event)

    async def steer(self, thread_id: uuid.UUID, text: str, message_id: uuid.UUID) -> None:
        """Pass a follow-up into the running turn as busy input (17.5)."""
        if thread_id not in self._sources:
            return
        await self.handle_message(
            self._event(thread_id, text, message_id=str(message_id), control=False)
        )

    async def interrupt(self, thread_id: uuid.UUID) -> None:
        """``/stop`` through ``handle_message`` (17.10 ``interrupt_outcome``)."""
        if thread_id not in self._sources:
            return
        await self._control_command(thread_id, "/stop", wait=False)

    async def _control_command(
        self,
        thread_id: uuid.UUID,
        text: str,
        *,
        user: tuple[str, str] | None = None,
        wait: bool = True,
    ) -> TurnOutcome:
        self._control[thread_id] = self._control.get(thread_id, 0) + 1
        try:
            # Every Hermes command goes out from the adapter's control sender, the slash-command
            # admin the installer lists (09 17.4); the server checked the action before.
            event = self._event(thread_id, text, user=user or (CONTROL_SENDER, "adapter"))
            if not wait:
                await self.handle_message(event)
                return TurnOutcome.COMPLETED
            outcome = await self._dispatch(event)
            while self._confirms:
                await asyncio.gather(*list(self._confirms), return_exceptions=True)
            return outcome
        finally:
            self._control[thread_id] -= 1
            if self._control[thread_id] <= 0:
                del self._control[thread_id]

    async def resolve_callback(
        self, card: OpenCard, custom_id: str, values: Mapping[str, Any], operator: uuid.UUID
    ) -> str | None:
        """Resolve a card's Hermes callback with Hermes's own resolvers (17.3)."""
        label = cards.chosen_label(card, custom_id, values)
        if label is None:
            return None
        request = card.request
        if request.kind is CardKind.CLARIFY:
            return label if resolve_gateway_clarify(request.hermes_id, label) else None
        if request.kind is CardKind.APPROVAL:
            count = resolve_gateway_approval(request.session_key, label)
            return label if count else None
        await slash_confirm.resolve(request.session_key, request.hermes_id, label)
        return label

    async def answer_card(self, card: OpenCard, text: str) -> str | None:
        """A typed answer to an open card: the clarify gets the text, other prompts are refused."""
        request = card.request
        if request.kind is CardKind.CLARIFY:
            return text if resolve_gateway_clarify(request.hermes_id, text) else None
        if request.kind is CardKind.APPROVAL:
            return "deny" if resolve_gateway_approval(request.session_key, "deny") else None
        await slash_confirm.resolve(request.session_key, request.hermes_id, "cancel")
        return "cancel"

    async def switch_session(
        self, turn: TurnRequest, mode: str, external_session_ref: str | None
    ) -> SessionSwitch:
        """``/new`` (with its confirmation) and, for ``bind``, ``/resume <id> --all`` (17.4)."""
        self._remember_source(turn)
        thread_id = turn.thread_id
        if self._sessions.get(thread_id) is not None:
            await self._control_command(thread_id, "/new")
        if mode != "bind" or not external_session_ref:
            return SessionSwitch.NEW
        await self._control_command(thread_id, f"/resume {external_session_ref} --all")
        return SessionSwitch.BOUND

    def current_session(self, thread_id: uuid.UUID) -> str | None:
        """The session ID Hermes last reported for the thread (hooks)."""
        return self._sessions.get(thread_id)

    def session_parent(self, thread_id: uuid.UUID) -> str | None:
        """The parent session of the thread's current session (a compression chain)."""
        return self._parents.get(thread_id)

    def session_title(self, thread_id: uuid.UUID) -> str | None:
        """The title of the thread's session from Hermes's session store, if it has one."""
        session_id = self._sessions.get(thread_id)
        if session_id is None:
            return None
        try:
            from hermes_state import SessionDB  # noqa: PLC0415 - only after a turn

            title = SessionDB().get_session_title(session_id)
        except Exception as exc:  # no title is a valid answer
            _LOG.info("session title not readable: %s", type(exc).__name__)
            return None
        return str(title) if title else None

    def resolved_credential(self) -> ResolvedCredential:
        """The credential kind of the next turn (17.8), never its value."""
        return hermes_config.resolved_credential()

    def loaded_toolsets(self) -> frozenset[str]:
        """The toolsets Hermes enables for this platform."""
        return hermes_config.enabled_toolsets(self.platform_key)

    # ---- Hermes callbacks (17.3) --------------------------------------------------------------

    def _thread(self, chat_id: Any, metadata: Mapping[str, Any] | None = None) -> uuid.UUID | None:
        """The thread of a callback: metadata, the turn's session variable, or the only turn."""
        raw = (metadata or {}).get("thread_id") or get_session_env(_THREAD_VAR, "")
        if raw:
            with contextlib.suppress(ValueError):
                return uuid.UUID(str(raw))
        candidates = [t for t, src in self._sources.items() if src[0] == str(chat_id)]
        return candidates[0] if len(candidates) == 1 else None

    def _post(self, fn: Any, *args: Any) -> None:
        """Run a handler callback on the adapter's loop (Hermes calls some from its threads)."""
        loop = self._loop
        if loop is None:
            return
        try:
            running = asyncio.get_running_loop()
        except RuntimeError:
            running = None
        if running is loop:
            fn(*args)
        else:
            loop.call_soon_threadsafe(fn, *args)

    async def send(
        self,
        chat_id: str,
        content: str,
        reply_to: str | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> SendResult:
        """The answer: one message of the run (or a failure without an active run)."""
        thread = self._thread(chat_id, metadata)
        if thread is not None and thread in self._control:
            return SendResult(success=True, message_id=f"ctl-{uuid.uuid4().hex}")
        if thread is None or self.handler is None:
            return SendResult(success=False, error="no active run in this thread")
        message_id = await self.handler.on_answer(thread, content, reply_to)
        if message_id is None:
            return SendResult(success=False, error="no active run in this thread")
        return SendResult(success=True, message_id=message_id)

    async def edit_message(
        self, chat_id: str, message_id: str, content: str, *, finalize: bool = False
    ) -> SendResult:
        """An edit of an own answer: buffered until ``finalize`` or the end of the run."""
        thread = self._thread(chat_id)
        if thread is not None and thread in self._control:
            return SendResult(success=True, message_id=message_id)
        ok = (
            thread is not None
            and self.handler is not None
            and await self.handler.on_edit(thread, message_id, content, finalize)
        )
        if ok:
            return SendResult(success=True, message_id=message_id)
        return SendResult(success=False, error="not an answer of an active run")

    def supports_draft_streaming(
        self,
        chat_type: str | None = None,
        metadata: dict[str, Any] | None = None,
        chat_id: str | None = None,
    ) -> bool:
        """Drafts fill ``/last_text_excerpt``."""
        return True

    async def send_draft(
        self, chat_id: str, draft_id: int, content: str, metadata: dict[str, Any] | None = None
    ) -> SendResult:
        """A cumulative draft of the answer."""
        thread = self._thread(chat_id, metadata)
        if thread is not None and self.handler is not None:
            self.handler.on_draft(thread, content)
        return SendResult(success=True)

    def set_status_text(self, chat_id: str, text: str | None) -> None:
        """A status sentence; Hermes may call it from the agent's thread."""
        thread = self._thread(chat_id)
        if thread is not None and self.handler is not None:
            self._post(self.handler.on_status, thread, text)

    def format_tool_event(
        self, event: Any, *, mode: str = "all", preview_max_len: int = 40
    ) -> str | None:
        """A tool event becomes a run step; ``None`` keeps Hermes from sending a bubble."""
        thread = self._thread(getattr(event, "chat_id", ""))
        preview = str(getattr(event, "preview", "") or getattr(event, "tool_name", "") or "tool")
        call_id = getattr(event, "tool_call_id", None) or getattr(event, "call_id", None)
        done = bool(getattr(event, "completed", False) or getattr(event, "done", False))
        if thread is not None and self.handler is not None:
            self._post(
                self.handler.on_tool_event,
                thread,
                preview[:preview_max_len] if preview_max_len else preview,
                str(call_id) if call_id else None,
                done,
            )
        return None

    async def send_typing(self, chat_id: str, metadata: dict[str, Any] | None = None) -> None:
        """Typing is not shown (09 4.7)."""

    async def stop_typing(self, chat_id: str) -> None:
        """Typing is not shown (09 4.7)."""

    async def on_processing_start(self, event: MessageEvent) -> None:
        """Nothing: the run started with ``startRun`` already."""

    async def on_processing_complete(self, event: MessageEvent, outcome: Any) -> None:
        """The end of a turn or command: resolves the waiting `start_turn`."""
        future = self._turns.get(str(event.message_id))
        if future is not None and not future.done():
            future.set_result(_OUTCOMES.get(outcome, TurnOutcome.FAILED))

    async def get_chat_info(self, chat_id: str) -> dict[str, Any]:
        """A thread of the channel."""
        return {"name": str(chat_id), "type": "thread", "chat_id": str(chat_id)}

    async def send_image(
        self,
        chat_id: str,
        image_url: str,
        caption: str | None = None,
        reply_to: str | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> SendResult:
        """Not built yet (Design note 21)."""
        return SendResult(success=False, error=_NOT_BUILT)

    async def send_document(
        self,
        chat_id: str,
        file_path: str,
        caption: str | None = None,
        file_name: str | None = None,
        reply_to: str | None = None,
        metadata: dict[str, Any] | None = None,
        **kwargs: Any,
    ) -> SendResult:
        """Not built yet (Design note 21)."""
        return SendResult(success=False, error=_NOT_BUILT)

    async def send_voice(
        self,
        chat_id: str,
        audio_path: str,
        caption: str | None = None,
        reply_to: str | None = None,
        metadata: dict[str, Any] | None = None,
        **kwargs: Any,
    ) -> SendResult:
        """Not built yet (Design note 21)."""
        return SendResult(success=False, error=_NOT_BUILT)

    async def send_video(
        self,
        chat_id: str,
        video_path: str,
        caption: str | None = None,
        reply_to: str | None = None,
        metadata: dict[str, Any] | None = None,
        **kwargs: Any,
    ) -> SendResult:
        """Not built yet (Design note 21)."""
        return SendResult(success=False, error=_NOT_BUILT)

    async def _card(
        self, chat_id: str, metadata: Mapping[str, Any] | None, request: CardRequest
    ) -> SendResult:
        thread = self._thread(chat_id, metadata)
        if thread is None or self.handler is None:
            return SendResult(success=False, error="no active run in this thread")
        shown = await self.handler.on_card(thread, request)
        if not shown:
            return SendResult(success=False, error="card not shown")
        return SendResult(success=True, message_id=f"card-{request.hermes_id}")

    async def send_clarify(
        self,
        chat_id: str,
        question: str,
        choices: list[Any] | None,
        clarify_id: str,
        session_key: str,
        metadata: dict[str, Any] | None = None,
    ) -> SendResult:
        """A clarify question as a card (``cl:<id>:<idx>``)."""
        request = CardRequest(
            CardKind.CLARIFY,
            str(clarify_id),
            str(question),
            tuple(str(c) for c in choices or ()),
            str(session_key),
        )
        return await self._card(chat_id, metadata, request)

    async def _send_exec_approval_prompt(self, prompt: ExecApprovalPrompt) -> SendResult:
        """An approval as the card of 09 11.5 (``appr:<id>:<choice>``)."""
        request = CardRequest(
            CardKind.APPROVAL,
            uuid.uuid4().hex,
            str(prompt.text),
            tuple(str(c) for c in prompt.choices),
            str(prompt.session_key),
        )
        return await self._card(prompt.chat_id, prompt.metadata, request)

    async def send_slash_confirm(
        self,
        chat_id: str,
        title: str,
        message: str,
        session_key: str,
        confirm_id: str,
        metadata: dict[str, Any] | None = None,
    ) -> SendResult:
        """A slash confirmation as a card; the adapter's own ``/new`` is confirmed at once."""
        thread = self._thread(chat_id, metadata)
        if thread is not None and thread in self._control:
            # The server already checked the reset (17.3 "Hermes commands"): answer "once".
            task = asyncio.ensure_future(
                slash_confirm.resolve(str(session_key), str(confirm_id), "once")
            )
            self._confirms.add(task)
            task.add_done_callback(self._confirms.discard)
            return SendResult(success=True, message_id=f"ctl-{confirm_id}")
        request = CardRequest(
            CardKind.SLASH_CONFIRM,
            str(confirm_id),
            str(message),
            (),
            str(session_key),
            str(title),
        )
        return await self._card(chat_id, metadata, request)

    # ---- hooks -------------------------------------------------------------------------------

    def on_hook(self, name: str, kwargs: Mapping[str, Any]) -> None:
        """A plugin hook of this platform (registered in `make_register`)."""
        thread = self._thread("", None)
        if thread is None:
            return
        if name == "pre_llm_call":
            session_id = kwargs.get("session_id")
            if session_id:
                self._sessions[thread] = str(session_id)
                parent = kwargs.get("parent_session_id")
                if parent:
                    self._parents[thread] = str(parent)
                self._post(self._persist_session, thread)
        elif name == "on_session_reset":
            new_id = kwargs.get("new_session_id") or kwargs.get("session_id")
            if new_id:
                self._sessions[thread] = str(new_id)
                self._parents.pop(thread, None)
                self._post(self._persist_session, thread)
        elif name == "post_api_request" and self.handler is not None:
            count = kwargs.get("assistant_tool_call_count")
            calls = count if isinstance(count, int) and not isinstance(count, bool) else None
            self._post(self.handler.on_model_reply, thread, calls)
            usage = _usage(kwargs)
            if usage is not None:
                self._post(self.handler.on_usage, thread, usage)

    def _persist_session(self, thread: uuid.UUID) -> None:
        """Keep the thread's session across restarts (09 9.1): a new process must know it."""
        store, session_id = self._store, self._sessions.get(thread)
        if store is None or session_id is None:
            return
        task = asyncio.ensure_future(
            store.put_session(thread, session_id, self._parents.get(thread))
        )
        self._confirms.add(task)
        task.add_done_callback(self._confirms.discard)


def _usage(kwargs: Mapping[str, Any]) -> Usage | None:
    summary = kwargs.get("usage")
    if not isinstance(summary, Mapping):
        return None

    def number(*names: str) -> int:
        for name in names:
            value = summary.get(name)
            if isinstance(value, int | float) and not isinstance(value, bool):
                return int(value)
        return 0

    provider = str(kwargs.get("provider") or "unknown")
    model = str(kwargs.get("response_model") or kwargs.get("model") or "unknown")
    return Usage(
        provider=provider,
        model=model,
        tokens_in=number("input_tokens", "prompt_tokens"),
        tokens_out=number("output_tokens", "completion_tokens"),
        tokens_cache_read=number("cache_read_tokens", "cache_read_input_tokens"),
        tokens_cache_write=number("cache_write_tokens", "cache_creation_input_tokens"),
    )


def _state_dir(platform_key: str) -> Path:
    """``<profile home>/<platform key>`` with mode ``0700`` (the SDK's ``state_dir``)."""
    path = Path(get_hermes_home()) / platform_key
    path.mkdir(mode=0o700, parents=True, exist_ok=True)
    path.chmod(0o700)
    return path


def make_register(platform_key: str) -> Any:
    """Build the plugin's ``register(ctx)`` for a platform key (the installer's entry calls it).

    Args:
        platform_key: `hermes_adapter.platform_key`.

    Returns:
        ``register(ctx)``: registers the platform and the hooks of this module.
    """

    def factory(config: PlatformConfig) -> ServerPlatform:
        return ServerPlatform(config, platform_key)

    def hook(name: str) -> Any:
        def callback(**kwargs: Any) -> None:
            if str(kwargs.get("platform") or "") != platform_key:
                return
            platform = _PLATFORMS.get(platform_key)
            if platform is not None:
                platform.on_hook(name, kwargs)

        return callback

    def register(ctx: Any) -> None:
        ctx.register_platform(
            name=platform_key,
            label="Chat server adapter",
            adapter_factory=factory,
            check_fn=lambda: True,
            allowed_users_env=f"{platform_key.upper()}_ALLOWED_USERS",
            allow_all_env=f"{platform_key.upper()}_ALLOW_ALL_USERS",
            max_message_length=DEFAULT_MAX_MESSAGE_BYTES,
        )
        for name in ("pre_llm_call", "on_session_reset", "post_api_request"):
            ctx.register_hook(name, hook(name))

    return register
