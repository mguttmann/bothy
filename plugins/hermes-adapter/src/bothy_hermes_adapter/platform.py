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
* Files Hermes sends with an answer (``send_document``, ``send_image_file``, ``send_voice``,
  ``send_video``) pass Hermes's own media delivery policy again and go to the handler, which
  uploads each one and sends it attached; an image Hermes names by a web address is sent as the
  address (the adapter downloads nothing from the web). Voice input is transcribed with Hermes's
  own speech to text (``_enrich_message_with_transcription`` of the gateway, the function its
  other platforms use).
* A control command (09 10.4) is ``handle_message`` with ``/<name>`` from the control sender;
  Hermes' reply (``send`` with the command's message as ``reply_to``) is collected for the
  handler instead of being sent. A skill command is a turn whose text is Hermes' own skill
  invocation message. At connect the adapter declares its commands
  (`bothy_hermes_adapter.hermes_commands`), the SDK reconciles them.
* A ``send`` outside a turn of its thread (a cron job of the gateway's scheduler, a notice of a
  background task, a send into another channel) is a report without a run (09 14):
  `bothy_hermes_adapter.handler.HermesHandler.on_report`.
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
from gateway.platforms._shared import platform_gate_env
from gateway.platforms.base import (
    BasePlatformAdapter,
    ExecApprovalPrompt,
    SendResult,
    cache_audio_from_bytes_async,
)
from gateway.platforms.event import MessageEvent, MessageType, ProcessingOutcome
from gateway.session_context import get_session_env
from hermes_cli import __version__ as hermes_version
from hermes_constants import get_hermes_home
from tools import slash_confirm
from tools.approval import resolve_gateway_approval
from tools.clarify_gateway import resolve_gateway_clarify
from tools.transcription_tools import _get_provider, _load_stt_config, is_stt_enabled

from bothy_hermes_adapter import __version__, cards, hermes_config
from bothy_hermes_adapter.config import (
    AdapterSettings,
    ConfigError,
    load_settings,
    load_token,
)
from bothy_hermes_adapter.defaults import NO_SPLIT_LENGTH
from bothy_hermes_adapter.handler import HermesHandler
from bothy_hermes_adapter.heartbeat import ADAPTER_NAME, RUNTIME_NAME, HermesAdapterLoop
from bothy_hermes_adapter.hermes_commands import declared_commands
from bothy_hermes_adapter.installer import CONTROL_SENDER
from bothy_hermes_adapter.media import audio_cache_ext
from bothy_hermes_adapter.reports import ReportError
from bothy_hermes_adapter.runtime import (
    CardKind,
    CardRequest,
    CommandRequest,
    MediaKind,
    OpenCard,
    ResolvedCredential,
    SessionSwitch,
    Transcription,
    TurnOutcome,
    TurnRequest,
    Usage,
)
from bothy_hermes_adapter.store import CallbackStore

__all__ = ["CONTROL_SENDER", "ServerPlatform", "dispatch_hook"]

_LOG = logging.getLogger("bothy_hermes_adapter")

_THREAD_VAR: Final = "HERMES_SESSION_THREAD_ID"
_OUTCOMES: Final = {
    ProcessingOutcome.SUCCESS: TurnOutcome.COMPLETED,
    ProcessingOutcome.FAILURE: TurnOutcome.FAILED,
    ProcessingOutcome.CANCELLED: TurnOutcome.CANCELLED,
}
_NO_RUN: Final = "no active run in this thread"
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

    # M4-47: Hermes hands every answer whole to `send` (no split, no cut of cron output); the
    # handler keeps it one message and attaches the rest above hermes_adapter.max_message_bytes.
    splits_long_messages = True
    MAX_MESSAGE_LENGTH = NO_SPLIT_LENGTH
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
        # Hermes's own allow list of the platform (09 17.7 layer 3, the name entry.py registers).
        self._allow_list_variable = f"{platform_key.upper()}_ALLOWED_USERS"
        self._loop: asyncio.AbstractEventLoop | None = None
        self._serve_task: asyncio.Task[Any] | None = None
        self._sdk: HermesAdapterLoop | None = None
        self.handler: HermesHandler | None = None
        self._turns: dict[str, asyncio.Future[TurnOutcome]] = {}
        self._sources: dict[uuid.UUID, tuple[str, str, str]] = {}
        self._sessions: dict[uuid.UUID, str] = {}
        self._parents: dict[uuid.UUID, str] = {}
        # Session ID to the thread whose turn ran on it in this process (09 17.4): Hermes v0.22
        # keeps such a session live on that thread's route until the gateway restarts.
        self._ran_here: dict[str, uuid.UUID] = {}
        self._control: dict[uuid.UUID, int] = {}
        self._confirms: set[asyncio.Task[Any]] = set()
        self._store: CallbackStore | None = None
        # Replies of a running control command by the command's message ID, and the IDs given
        # back to Hermes for them (an edit replaces the collected text).
        self._replies: dict[str, list[str]] = {}
        self._reply_ids: dict[str, tuple[list[str], int]] = {}
        # Message IDs this process handed to Hermes, so a ``send`` that replies to one of them is
        # known after Hermes cleared its session variable (09 14 versus 17.3): a turn's message to
        # the run it started or steered, an adapter command's message to nothing.
        self._turn_messages: dict[str, uuid.UUID | None] = {}
        self._control_messages: dict[str, None] = {}

    # ---- lifecycle ---------------------------------------------------------------------------

    @property
    def enforces_own_access_policy(self) -> bool:
        """The adapter enforces its own access policy (09 17.7 layer 2)."""
        return True

    @property
    def authorization_is_upstream(self) -> bool:
        """The server authorized the person and the adapter checked its token (09 17.7).

        Hermes v0.22 checks every message again at admission and trusts an own access policy
        there only through this declaration. It holds only while Hermes has no allow list of
        its own for the platform: with the list of ``personal`` (layer 3) Hermes keeps judging
        each person by it. The list is read on every call with the reader Hermes's admission
        check uses, so it covers the process environment and a list written without a restart.
        """
        return not platform_gate_env(self._allow_list_variable)

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
            commands=declared_commands(self._skills_or_none()),
        )
        self._sdk = HermesAdapterLoop(config, self.handler)
        self._serve_task = asyncio.ensure_future(self._serve(store))
        _PLATFORMS[self.platform_key] = self
        self._mark_connected()
        return True

    def _skills_or_none(self) -> Mapping[str, str]:
        """The skill commands to declare; an unreadable skill list declares none (logged)."""
        try:
            return self.skill_commands()
        except Exception as exc:
            _LOG.warning("skill commands not readable: %s", type(exc).__name__)
            return {}

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
        if control:
            _remember(self._control_messages, mid, None)
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
        _remember(self._turn_messages, str(turn.message_id), turn.run_id)
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
        run = self.handler.active_run(thread_id) if self.handler is not None else None
        _remember(self._turn_messages, str(message_id), run)
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
            await self._command_settled(event)
            while self._confirms:
                await asyncio.gather(*list(self._confirms), return_exceptions=True)
            return outcome
        finally:
            self._control[thread_id] -= 1
            if self._control[thread_id] <= 0:
                del self._control[thread_id]

    async def _command_settled(self, event: MessageEvent) -> None:
        """Wait until Hermes has finished processing a command, not only reported it.

        Hermes v0.22 calls `on_processing_complete` of a command before its processing task
        ends; a turn sent in that gap on the same session key runs but never completes
        (measured after ``/resume``, 09 17.4).
        """
        task = self._session_tasks.get(self._event_session_key(event))
        if task is not None and not task.done() and task is not asyncio.current_task():
            await asyncio.wait({task})

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
        holder = self._ran_here.get(external_session_ref)
        if holder is not None and holder != thread_id:
            # Hermes does not move it off the other thread's live route, and the turn of this
            # thread stalls on it (an admission conflict): fail open with the new session (9.5).
            _LOG.info("bind refused: the session is live in another thread until a restart")
            return SessionSwitch.NEW
        await self._control_command(thread_id, f"/resume {external_session_ref} --all")
        return SessionSwitch.BOUND

    async def run_command(self, command: CommandRequest) -> list[str]:
        """A control command in the thread's session; returns Hermes' reply texts (09 10.4).

        It goes out from the control sender like ``/new`` (17.4). Hermes answers a command for
        an idle session through its normal processing (the adapter waits for its end) and one
        for a busy session inline, before ``handle_message`` returns; either way the reply comes
        through ``send`` with the command's message as ``reply_to`` and is collected here.
        """
        thread_id = command.thread_id
        self._sources.setdefault(
            thread_id, (str(command.channel_id), str(command.user_id), command.user_name)
        )
        message_id = f"ctl-{uuid.uuid4().hex}"
        replies: list[str] = []
        self._replies[message_id] = replies
        try:
            event = self._event(
                thread_id, command.text, user=(CONTROL_SENDER, "adapter"), message_id=message_id
            )
            await self._dispatch(event)
            if getattr(event, "_gateway_accepted", True):
                await self._command_settled(event)
        finally:
            self._replies.pop(message_id, None)
            for key in [k for k in self._reply_ids if k.startswith(f"{message_id}-")]:
                del self._reply_ids[key]
        return list(replies)

    def skill_commands(self) -> Mapping[str, str]:
        """Hermes' skill commands for this platform, without the skills it disables here."""
        from agent.skill_commands import get_skill_commands  # noqa: PLC0415 - Hermes' skill scan
        from agent.skill_utils import get_disabled_skill_names  # noqa: PLC0415

        disabled = get_disabled_skill_names(platform=self.platform_key)
        found: dict[str, str] = {}
        for key, info in get_skill_commands().items():
            if str(info.get("name", "")) in disabled:
                continue
            found[str(key).lstrip("/")] = str(info.get("description") or "")
        return found

    def skill_text(self, name: str, text: str) -> str | None:
        """Hermes' own turn text for a skill command, as its gateway builds it for ``/<skill>``."""
        from agent.skill_commands import build_skill_invocation_message  # noqa: PLC0415

        if name not in self.skill_commands():
            return None
        return build_skill_invocation_message(f"/{name}", text)

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

    def voice_input(self) -> bool:
        """Whether Hermes's speech to text is on and finds a provider (09 2.2, 13)."""
        config = _load_stt_config()
        return is_stt_enabled(config) and _get_provider(config) != "none"

    async def transcribe(self, data: bytes, name: str, mime_type: str) -> Transcription:
        """Transcribe audio as Hermes does on its other platforms (09 13, 17.6).

        The audio goes into Hermes's audio cache of the profile and through the gateway's own
        ``_enrich_message_with_transcription``: the configured provider with the local fallback,
        and without a transcript the note Hermes writes on every platform.
        """
        runner = getattr(self, "gateway_runner", None)
        if runner is None:
            return Transcription(None)
        ext = audio_cache_ext(name, mime_type)
        path = await cache_audio_from_bytes_async(data, ext)
        text, transcripts = await runner._enrich_message_with_transcription("", [path])
        if transcripts:
            return Transcription(str(transcripts[0]))
        return Transcription(None, str(text or ""))

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
        """The answer of a turn, the reply to a control command, or a report without a run.

        * ``reply_to`` names a running control command: the text is collected as its reply.
        * The thread is in one of the adapter's own commands (``/new``, ``/stop``): swallowed.
        * ``reply_to`` names a message this process handed to Hermes: a late command reply
          fails, and a turn's answer becomes a message of the run it belongs to, or fails when
          that run is no longer active. Hermes sends the final answer after it cleared its
          session variable, so a cancelled or aborted turn's answer never becomes a report.
        * The send belongs to a turn of its thread (`_turn_send`): one message of the run, or a
          failure when no run of the thread is active any more (a late answer is never posted).
        * Anything else is a report of the agent (09 14): a cron job, a background notice, a
          send into another channel or thread.
        """
        if reply_to is not None and reply_to in self._replies:
            replies = self._replies[reply_to]
            replies.append(content)
            key = f"{reply_to}-{len(replies)}"
            self._reply_ids[key] = (replies, len(replies) - 1)
            return SendResult(success=True, message_id=key)
        meta_thread = _uuid_of((metadata or {}).get("thread_id"))
        env_thread = _uuid_of(get_session_env(_THREAD_VAR, ""))
        thread = meta_thread or env_thread
        if thread is not None and thread in self._control:
            return SendResult(success=True, message_id=f"ctl-{uuid.uuid4().hex}")
        if self.handler is None:
            return SendResult(success=False, error="the adapter is not connected")
        if reply_to is not None and reply_to in self._control_messages:
            return SendResult(success=False, error="the command has ended")
        if reply_to is not None and reply_to in self._turn_messages:
            run = self._turn_messages[reply_to]
            if thread is None or run is None or self.handler.active_run(thread) != run:
                return SendResult(success=False, error="no active run in this thread")
            message_id = await self.handler.on_answer(thread, content, reply_to)
            if message_id is None:
                return SendResult(success=False, error="no active run in this thread")
            return SendResult(success=True, message_id=message_id)
        if thread is not None and self._turn_send(str(chat_id), thread, env_thread):
            message_id = await self.handler.on_answer(thread, content, reply_to)
            if message_id is None:
                return SendResult(success=False, error=_NO_RUN)
            return SendResult(success=True, message_id=message_id)
        raw_thread = (metadata or {}).get("thread_id")
        try:
            posted = await self.handler.on_report(
                str(chat_id), str(raw_thread) if raw_thread else None, content
            )
        except ReportError as exc:
            return SendResult(success=False, error=str(exc))
        except Exception as exc:  # the server was not reachable after the client's retries
            _LOG.warning("report not sent: %s", type(exc).__name__)
            return SendResult(success=False, error="the server was not reachable")
        return SendResult(success=True, message_id=str(posted) if posted else None)

    def _turn_send(self, chat_id: str, thread: uuid.UUID, env_thread: uuid.UUID | None) -> bool:
        """Whether a send into ``thread`` belongs to a turn of that thread (09 14 versus 17.3).

        Only a thread a turn of this process ran in, in the same channel, qualifies. Inside a
        turn (Hermes' session variable names a thread) only that turn's own thread does; outside
        any turn only a thread whose run is active right now.
        """
        source = self._sources.get(thread)
        if source is None or source[0] != chat_id:
            return False
        if env_thread is not None:
            return env_thread == thread
        return self.handler is not None and self.handler.active_run(thread) is not None

    async def edit_message(
        self, chat_id: str, message_id: str, content: str, *, finalize: bool = False
    ) -> SendResult:
        """An edit of an own answer: buffered until ``finalize`` or the end of the run."""
        collected = self._reply_ids.get(message_id)
        if collected is not None:
            replies, index = collected
            replies[index] = content
            return SendResult(success=True, message_id=message_id)
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
        """An image by web address: sent as the address (the adapter downloads nothing)."""
        text = f"{caption}\n{image_url}" if caption else image_url
        return await self.send(chat_id, text, reply_to=reply_to, metadata=metadata)

    async def _send_file(
        self,
        chat_id: str,
        path: str,
        kind: MediaKind,
        caption: str | None,
        file_name: str | None,
        metadata: Mapping[str, Any] | None,
    ) -> SendResult:
        """A local file of the answer: Hermes's delivery policy, then upload and send (17.3)."""
        thread = self._thread(chat_id, metadata)
        if thread is None or self.handler is None:
            return SendResult(success=False, error=_NO_RUN)
        # Hermes checks a MEDIA path before it calls here; other callers do not, so the policy
        # (a regular file outside the credential and system paths) applies again.
        safe = self.validate_media_delivery_path(str(path))
        if safe is None:
            return SendResult(success=False, error="refused by the media delivery policy")
        sent = await self.handler.on_file(thread, safe, kind, caption, file_name)
        if sent.message_id is None:
            return SendResult(success=False, error=f"file not sent: {sent.error}")
        return SendResult(success=True, message_id=sent.message_id)

    async def send_image_file(
        self,
        chat_id: str,
        image_path: str,
        caption: str | None = None,
        reply_to: str | None = None,
        metadata: dict[str, Any] | None = None,
        **kwargs: Any,
    ) -> SendResult:
        """A local image as an attachment of the answer."""
        return await self._send_file(chat_id, image_path, MediaKind.IMAGE, caption, None, metadata)

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
        """A file as an attachment of the answer."""
        return await self._send_file(
            chat_id, file_path, MediaKind.DOCUMENT, caption, file_name, metadata
        )

    async def send_voice(
        self,
        chat_id: str,
        audio_path: str,
        caption: str | None = None,
        reply_to: str | None = None,
        metadata: dict[str, Any] | None = None,
        **kwargs: Any,
    ) -> SendResult:
        """Audio as an attachment of the answer (a text message, 09 17.3)."""
        return await self._send_file(chat_id, audio_path, MediaKind.VOICE, caption, None, metadata)

    async def send_video(
        self,
        chat_id: str,
        video_path: str,
        caption: str | None = None,
        reply_to: str | None = None,
        metadata: dict[str, Any] | None = None,
        **kwargs: Any,
    ) -> SendResult:
        """A video as an attachment of the answer."""
        return await self._send_file(chat_id, video_path, MediaKind.VIDEO, caption, None, metadata)

    async def _card(
        self, chat_id: str, metadata: Mapping[str, Any] | None, request: CardRequest
    ) -> SendResult:
        thread = self._thread(chat_id, metadata)
        if thread is None or self.handler is None:
            return SendResult(success=False, error=_NO_RUN)
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
        """A plugin hook of this platform (registered by `bothy_hermes_adapter.entry`)."""
        thread = self._thread("", None)
        if thread is None:
            return
        if name == "pre_llm_call":
            session_id = kwargs.get("session_id")
            if session_id:
                self._sessions[thread] = str(session_id)
                self._ran_here[str(session_id)] = thread
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


_KNOWN_MESSAGES = 4096


def _remember(known: dict[str, Any], key: str, value: Any) -> None:
    """Keep ``key`` in an insertion-ordered map of at most `_KNOWN_MESSAGES` entries."""
    known.pop(key, None)
    known[key] = value
    while len(known) > _KNOWN_MESSAGES:
        del known[next(iter(known))]


def _uuid_of(raw: Any) -> uuid.UUID | None:
    if not raw:
        return None
    try:
        return uuid.UUID(str(raw))
    except ValueError:
        return None


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


def dispatch_hook(platform_key: str, name: str, kwargs: Mapping[str, Any]) -> None:
    """Pass a plugin hook to the platform of the key, if it runs (`bothy_hermes_adapter.entry`).

    Args:
        platform_key: `hermes_adapter.platform_key`.
        name: The hook's name.
        kwargs: The hook's arguments.
    """
    platform = _PLATFORMS.get(platform_key)
    if platform is not None:
        platform.on_hook(name, kwargs)
