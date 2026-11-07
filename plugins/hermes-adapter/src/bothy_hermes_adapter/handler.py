# SPDX-License-Identifier: Apache-2.0
"""The runtime side of the SDK loop for Hermes (09 4.9, 7, 8, 9, 11, 17; Design notes 5 to 14).

`HermesHandler` is the `bothy_sdk.adapter.AgentHandler` of the Hermes adapter. Acceptance,
inbox, ``ack``, allow list, renewal, the start and resume answers, cancel and restart are the
SDK's (M2-29); this module decides what Hermes does:

* `actual_billing` determines the mode from the kind of credential Hermes resolves, at intake and
  again right before every ``startRun`` and ``resumeRun``; `actual_tool_class` reports the class
  of the enabled toolsets.
* `prepare` reads the run context (and the channel configuration when ``settings_version`` is
  newer) and builds the turn's texts; a model command (``trigger_kind = command``) of a Hermes
  skill becomes Hermes' skill invocation with the command's ``input``. It refuses an
  ``interaction`` trigger and every other command, so such a run ends ``failed``
  ``adapter_error`` without a start.
* `execute` handles the session of the thread (09 9.1, 9.5), runs the Hermes turn and ends the
  run: buffered edits first, then the session report, the title and ``finishRun`` with metrics
  and costs. Before the end the run holds the background work its turns started (09 17.12,
  R-AG-44): a step per delegation unit and process until they are done, the turns Hermes starts
  with their results inside the run, a follow-up as a turn of its own; a stop or an early end
  cancels that work. The work includes the external jobs its tool calls registered
  (`bothy_hermes_adapter.jobs`, 17.12 point 8); `sweep_jobs` posts a live job no open run covers
  once, as a message of the agent without a run. `tool_allowed` answers Hermes'
  ``pre_tool_call``: no tool runs in a thread without an open run.
* `on_followup` steers a running turn or answers a waiting card; `on_interaction` resolves a
  card's Hermes callback exactly once and answers the interaction.
* `on_command` runs a control command (``command.invoked``) of
  `bothy_hermes_adapter.hermes_commands` in the thread's Hermes session and posts Hermes' reply
  as the agent's message without a run (09 10.4); `on_report` posts a message Hermes sends
  without an invocation (a cron job's report) through `bothy_hermes_adapter.reports` (09 14).
* The `bothy_hermes_adapter.runtime.RuntimeEvents` methods take what Hermes reports during a
  turn: live box, answer, edits, cards, usage and the files it sends with its answer (each one
  uploaded and sent as a message with that file attached, 09 17.3).
* Before the turn, the audio attachments of the triggering message without a transcript are
  transcribed with Hermes's own speech to text and the transcripts delivered to the server (09
  13, 17.6); every other attachment of it is loaded and put into Hermes's media cache, so the
  turn gets it as Hermes's own platforms pass one (images to its vision, files as paths), and
  one that cannot be passed on (scan pending or blocked, over Hermes's limit, refused) as a
  note in the person's text.

Logs (logger ``bothy_hermes_adapter``) carry run, thread and event IDs, reasons and codes only.

Examples:
    >>> HermesHandler.__mro__[1].__name__
    'AgentHandler'
"""

from __future__ import annotations

import asyncio
import contextlib
import logging
import time
import unicodedata
from collections.abc import Iterator, Mapping
from dataclasses import dataclass, field, replace
from datetime import UTC, datetime
from typing import Any, Final
from uuid import UUID

import httpx
from bothy_sdk.adapter import ActualBilling, AgentHandler, Invocation, StartOptions
from bothy_sdk.client import Client
from bothy_sdk.errors import ProblemError
from bothy_sdk.generated.api.admin import get_instance
from bothy_sdk.generated.api.agent_runtime import (
    get_served_channel_config,
    report_agent_session,
    respond_to_interaction,
    submit_transcript,
)
from bothy_sdk.generated.api.files import download_file, get_file
from bothy_sdk.generated.api.interactions import update_component
from bothy_sdk.generated.api.messages import get_message, update_message
from bothy_sdk.generated.api.threads import get_thread, set_thread_title
from bothy_sdk.generated.models import (
    AgentChannelConfig,
    AgentSessionReport,
    AgentSessionReportBindFailed,
    BillingEvidence,
    ChannelBehavior,
    ComponentUpdate,
    ComponentUpdateState,
    CostReport,
    InstanceInfo,
    InteractionRespond,
    InteractionRespondState,
    InteractionResponse,
    Message,
    MessageUpdate,
    RunContext,
    RunMetrics,
    Thread,
    ThreadTitlePut,
    ToolClass,
    TranscriptSubmit,
)
from bothy_sdk.generated.types import File, Unset
from bothy_sdk.long_answers import send_long_message
from bothy_sdk.runs import RunHandle
from bothy_sdk.sources import Envelope

from bothy_hermes_adapter import cards
from bothy_hermes_adapter.actual_mode import ActualMode, determine
from bothy_hermes_adapter.config import AdapterSettings
from bothy_hermes_adapter.context_format import (
    BuiltContext,
    build_context,
    command_user_side,
    cut_utf8,
)
from bothy_hermes_adapter.hermes_commands import (
    BUILT_IN,
    COMMAND_CLASSES,
    INPUT_OPTION,
    command_text,
    option_values,
)
from bothy_hermes_adapter.jobs import JobError, JobRecord, JobRegistry
from bothy_hermes_adapter.live_box import LiveBox
from bothy_hermes_adapter.media import (
    NO_TEXT,
    FileRefused,
    audio_to_transcribe,
    files_for_hermes,
    problem_reason,
    read_local_file,
    scan_refusal,
    undelivered_note,
    upload,
    with_transcripts,
)
from bothy_hermes_adapter.reports import (
    ReportError,
    direct_send,
    parse_target,
    post_report,
    recovered_reply,
)
from bothy_hermes_adapter.runtime import (
    BackgroundKind,
    BackgroundTask,
    CardKind,
    CardRequest,
    CommandRequest,
    FileSent,
    InboundMedia,
    MediaKind,
    OpenCard,
    Runtime,
    SessionSwitch,
    TurnOutcome,
    TurnRequest,
    Usage,
)
from bothy_hermes_adapter.store import CallbackStore
from bothy_hermes_adapter.tool_class import classify

__all__ = [
    "DEFAULT_CURRENCY",
    "EXTERNAL_JOB_STEP",
    "JOB_WITHOUT_RUN",
    "TOOL_REFUSED",
    "BusyMode",
    "HermesHandler",
    "PrepareRefused",
]

_LOG = logging.getLogger("bothy_hermes_adapter")

DEFAULT_CURRENCY: Final = "USD"
"""Currency of costs when Hermes's usage report states none: the currency of the prices the
pinned version ships (docs/DECISIONS.md D-M2-33-7)."""

_STOP_STATUSES: Final = frozenset({403, 409, 412})

TOOL_REFUSED: Final = (
    "Refused by the chat platform: no run is open in this chat, so no tool may run now and "
    "nothing was done. Answer in text only; the person can ask again."
)
"""The message of a tool call refused outside a run (09 17.12 point 7, R-AG-44); Hermes gives
it to the model as the tool's error."""
_STEP_TITLE_MAX: Final = 200
"""Longest title of a background step (the length of a thread title, ``thread.title_max_chars``)."""
_STEP_NAMES: Final = {BackgroundKind.SUBAGENT: "delegate_task", BackgroundKind.JOB: "terminal"}
_ENDED_STATES: Final = frozenset({"completed", "failed", "cancelled"})
EXTERNAL_JOB_STEP: Final = "external_job"
"""Step name of a registered external job (09 17.12 point 8); its kind is ``background_job``."""
_JOB_TASK: Final = "job:"
JOB_WITHOUT_RUN: Final = (
    "A background job runs in this thread without a run: **{title}** (process {pid}, started "
    "{since}, job {job_id}). Its run has ended, for example at a restart of the agent, or it was "
    "started outside a run, so it has no stop button here. The operator of the agent can stop it "
    "on the host with `python -m bothy_hermes_adapter job stop --profile-home <profile home> "
    "--job-id {job_id}`."
)
"""The message that shows a live registered job no open run covers (09 17.12 point 8)."""


class BusyMode:
    """The values of Hermes's ``busy_input_mode`` (09 17.5)."""

    STEER: Final = "steer"
    QUEUE: Final = "queue"
    INTERRUPT: Final = "interrupt"


class PrepareRefused(Exception):  # noqa: N818 - names the outcome, like the SDK's RuntimeFailure
    """``prepare`` refuses the run; the SDK ends it ``failed`` ``adapter_error`` (Design note 5)."""


@dataclass
class _Turn:
    """What the handler keeps about one run's Hermes turn (process memory only)."""

    run_id: UUID
    thread_id: UUID
    channel_id: UUID
    handle: RunHandle
    built: BuiltContext
    media: tuple[InboundMedia, ...]
    session_mode: str | None
    session_ref: str | None
    session_version: int | None
    mode_used: str
    live: LiveBox | None = None
    answers: dict[str, tuple[UUID, int]] = field(default_factory=dict)
    edits: dict[str, str] = field(default_factory=dict)
    usage: list[Usage] = field(default_factory=list)
    steered: list[UUID] = field(default_factory=list)
    card: OpenCard | None = None
    card_versions: dict[UUID, int] = field(default_factory=dict)
    abort_reason: str | None = None
    final_reply: bool = False
    upload_limit: int | None = None
    anchored: bool = True
    quotes: bool = False
    dropped: bool = False
    interrupted: bool = False
    settled: asyncio.Event = field(default_factory=asyncio.Event)
    ended: bool = False
    # Background work of the run (09 17.12, R-AG-44): the run's start in wall time (the registry
    # filter), whether the first turn ended and the run holds work, the Hermes turns of the run
    # that run now (own: started by the adapter; internal: started by Hermes itself), whether a
    # Hermes turn of the thread passed ``pre_llm_call`` without its ``on_session_end`` yet
    # (``hermes_open``), whether a turn Hermes started itself failed, the steps of the work by
    # Hermes' ID, and the monotonic time of the last sign of life.
    started_wall: float = field(default_factory=time.time)
    held: bool = False
    seen_work: bool = False
    own_turns: int = 1
    internal: bool = False
    hermes_open: bool = False
    failed_turn: bool = False
    failed_result: bool = False
    work_steps: dict[str, tuple[UUID, str]] = field(default_factory=dict)
    work_counts: tuple[int, int] = (0, 0)
    last_busy: float = field(default_factory=time.monotonic)
    last_progress: float = field(default_factory=time.monotonic)
    progress_events: int = 0
    wake: asyncio.Event = field(default_factory=asyncio.Event)
    pending: list[Message] = field(default_factory=list)


class HermesHandler(AgentHandler):
    """The Hermes side of the adapter.

    Args:
        runtime: The Hermes runtime (``platform.py``, or a fake in tests).
        settings: The adapter configuration.
        store: Resolved callbacks and open cards.
        busy_mode: Hermes's configured ``busy_input_mode`` (read at connect).
        now: The wall time (for step end times).
        jobs: The registry of external jobs (09 17.12 point 8), ``None`` without one.
    """

    def __init__(
        self,
        runtime: Runtime,
        settings: AdapterSettings,
        store: CallbackStore,
        *,
        busy_mode: str,
        now: Any = None,
        jobs: JobRegistry | None = None,
    ) -> None:
        """Create the handler; the tool class is read from Hermes at once."""
        self.runtime = runtime
        self._jobs = jobs
        self.settings = settings
        self.store = store
        self.busy_mode = busy_mode
        self._now = now or _utcnow
        self._turns: dict[UUID, _Turn] = {}
        self._by_run: dict[UUID, _Turn] = {}
        self._prepared: dict[UUID, BuiltContext] = {}
        self._anchors: dict[UUID, bool] = {}
        self._inbound: dict[UUID, tuple[RunContext, ChannelBehavior | None]] = {}
        self._channels: dict[UUID, tuple[int, ChannelBehavior]] = {}
        # Runs that stop their work now, by thread: the run's start in wall time. Such a run
        # still covers its jobs until they are stopped (09 17.12 point 8).
        self._stopping: dict[UUID, float] = {}
        # Jobs this process posted without a run; holds the "posted once" rule when the mark
        # on disk cannot be written (09 17.12 point 8).
        self._posted_jobs: set[str] = set()
        self._tool_class = self._read_tool_class()
        self._voice_input = self._read_voice_input()
        self._tasks: set[asyncio.Task[Any]] = set()
        self._client: Client | None = None

    def attach(self, client: Client) -> None:
        """Take the adapter loop's client for the agent's own writes without a run.

        Args:
            client: The loop's client.
        """
        self._client = client

    # ---- mode, class, heartbeat ------------------------------------------------------------

    def _read_tool_class(self) -> str:
        try:
            return classify(frozenset(self.runtime.loaded_toolsets()))
        except Exception as exc:  # an unreadable toolset list is the highest class
            _LOG.warning("toolsets not readable: %s", type(exc).__name__)
            return "host"

    def _read_voice_input(self) -> bool:
        try:
            return bool(self.runtime.voice_input())
        except Exception as exc:  # an unreadable speech to text setup transcribes nothing
            _LOG.warning("speech to text not readable: %s", type(exc).__name__)
            return False

    def actual_mode(self) -> ActualMode:
        """Determine the actual billing mode now (09 17.8).

        Returns:
            The mode with its evidence.
        """
        return determine(self.runtime.resolved_credential)

    def actual_billing(self, inv: Invocation) -> ActualBilling:
        """The SDK's question at intake, before ``startRun`` and before ``resumeRun``.

        Args:
            inv: The invocation.

        Returns:
            The mode (``unknown`` never matches) and its evidence.
        """
        actual = self.actual_mode()
        return ActualBilling(actual.sdk_mode, BillingEvidence.from_dict(actual.evidence))

    def actual_tool_class(self) -> ToolClass:
        """The class of the toolsets Hermes enables (read at connect and every heartbeat).

        Returns:
            The tool class.
        """
        return ToolClass(self._tool_class)

    def capabilities(self) -> dict[str, Any]:
        """``details.capabilities`` of the heartbeat (09 2.2, 17.5).

        Returns:
            ``components``, ``title`` and ``files_out``, ``followups`` unless
            ``busy_input_mode`` is ``interrupt``, and ``voice_input`` while Hermes's speech to
            text is on and finds a provider.
        """
        caps: dict[str, Any] = {"components": True, "title": True, "files_out": True}
        if self.busy_mode in (BusyMode.STEER, BusyMode.QUEUE):
            caps["followups"] = self.busy_mode
        if self._voice_input:
            caps["voice_input"] = True
        return caps

    def heartbeat_report(self) -> tuple[ActualMode, str, dict[str, Any]]:
        """Determine mode and class again for a heartbeat; a mode change aborts running turns.

        Returns:
            The actual mode, the tool class and the capabilities.
        """
        actual = self.actual_mode()
        self._tool_class = self._read_tool_class()
        self._voice_input = self._read_voice_input()
        for turn in list(self._by_run.values()):
            if not turn.ended and actual.sdk_mode != turn.mode_used and turn.abort_reason is None:
                _LOG.warning("run %s: actual mode changed during the turn", turn.run_id)
                turn.abort_reason = "billing_mode_mismatch"
                self._spawn(self._interrupt(turn))
        return actual, self._tool_class, self.capabilities()

    # ---- run start ---------------------------------------------------------------------------

    async def prepare(self, inv: Invocation, run: RunHandle) -> StartOptions:
        """Read the context and build the turn's texts; no Hermes turn yet (09 4.9).

        Args:
            inv: The ``agent.invocation``.
            run: The run (not started).

        Returns:
            Start options with the billing evidence.

        Raises:
            PrepareRefused: An ``interaction`` trigger, or a command that is no Hermes skill
                command now, which this adapter does not serve.
        """
        payload = inv.payload
        kind = str(getattr(payload, "trigger_kind", "message"))
        skill_text = None
        if kind == "command":
            skill_text = self._skill_text(payload)
            if skill_text is None:
                _LOG.warning("run %s refused: not a skill command", run.run_id)
                raise PrepareRefused(kind)
        elif kind != "message":
            _LOG.warning("run %s refused: trigger kind %s", run.run_id, kind)
            raise PrepareRefused(kind)
        channel = await self._channel(payload, run) if _operator_run(payload) else None
        if skill_text is not None:
            # A command run has no run context: 09 4.6 defines it for a triggering message, and
            # the server answers getRunContext of any other trigger kind with 500. The turn is
            # Hermes' own skill invocation with the format rules and the channel instructions.
            self._prepared[run.run_id] = BuiltContext(command_user_side(channel), skill_text)
            actual = self.actual_mode()
            return StartOptions(billing_evidence=BillingEvidence.from_dict(actual.evidence))
        ctx = await run.load_context()
        self._anchors[run.run_id] = await self._trigger_in_thread(payload, run)
        self._prepared[run.run_id] = await build_context(
            ctx,
            channel=channel,
            fetch=self._fetcher(run),
            attachment_limit=self.settings.attachment_text_max_bytes,
        )
        if audio_to_transcribe(ctx) or files_for_hermes(ctx):
            # Transcribed and loaded after the start, before the turn (09 13): the run is
            # running meanwhile.
            self._inbound[run.run_id] = (ctx, channel)
        actual = self.actual_mode()
        return StartOptions(billing_evidence=BillingEvidence.from_dict(actual.evidence))

    async def _trigger_in_thread(self, payload: Any, run: RunHandle) -> bool:
        """Whether the triggering message lies in the run's thread (09 17.3, answer anchor).

        A reply names a message of its own thread (03 ``messages``, ``reply_to_message_id``;
        ``400 validation_failed`` ``not_in_thread`` otherwise). With ``auto_thread`` the trigger
        stays in the main thread while the run lives in the thread opened on it (09 3.4 point
        5), so the answer there quotes nothing. When the message cannot be read the answer
        quotes nothing either: an answer without a quote is always accepted.
        """
        try:
            message = await run.client.call_as(
                Message, get_message.asyncio_detailed, message_id=payload.message_id
            )
        except ProblemError as exc:
            _LOG.info("run %s: trigger not read code=%s", run.run_id, exc.code)
            return False
        except httpx.TransportError:
            _LOG.info("run %s: trigger not read code=transport_error", run.run_id)
            return False
        return bool(message.thread_id == run.thread_id)

    def _skill_text(self, payload: Any) -> str | None:
        """The turn text of a model command: Hermes' skill invocation, or ``None`` (09 17.3)."""
        name = _text_or_none(getattr(payload, "command_name", None))
        if name is None or name in BUILT_IN or name in COMMAND_CLASSES:
            return None
        values = option_values(getattr(payload, "options", None))
        try:
            return self.runtime.skill_text(name, values.get(INPUT_OPTION, ""))
        except Exception as exc:  # an unreadable skill is no skill
            _LOG.warning("skill not readable: %s", type(exc).__name__)
            return None

    async def _channel(self, payload: Any, run: RunHandle) -> ChannelBehavior | None:
        channel_id = payload.channel_id
        version = payload.settings_version
        cached = self._channels.get(channel_id)
        if cached is not None and (isinstance(version, Unset) or version <= cached[0]):
            return cached[1]
        config = await run.client.call_as(
            AgentChannelConfig, get_served_channel_config.asyncio_detailed, channel_id=channel_id
        )
        behavior = config.behavior
        self._channels[channel_id] = (int(behavior.settings_version), behavior)
        return behavior

    def _fetcher(self, run: RunHandle) -> Any:
        async def fetch(file_id: UUID, limit: int) -> bytes:
            result = await run.client.call(
                download_file.asyncio_detailed, file_id=file_id, range_=f"bytes=0-{limit - 1}"
            )
            if not isinstance(result, File):
                return b""
            return bytes(result.payload.read())

        return fetch

    async def execute(self, inv: Invocation, run: RunHandle) -> None:
        """Run the Hermes turn and end the run (called only after ``200`` on ``startRun``).

        Args:
            inv: The invocation.
            run: The started run.
        """
        payload = inv.payload
        built = self._prepared.pop(run.run_id, None)
        if built is None:  # prepare ran in a process that no longer exists
            raise PrepareRefused("context")
        session = getattr(payload, "agent_session", None)
        inbound = self._inbound.pop(run.run_id, None)
        media: tuple[InboundMedia, ...] = ()
        if inbound is not None:
            ctx, channel = inbound
            if audio_to_transcribe(ctx):
                built = await self._transcribed(run, ctx, channel) or built
            media, notes = await self._media(run, ctx)
            built = _with_media(built, media, notes)
        turn = _Turn(
            run_id=run.run_id,
            thread_id=run.thread_id,
            channel_id=payload.channel_id,
            handle=run,
            built=built,
            media=media,
            session_mode=_text_or_none(getattr(payload, "session_mode", None)),
            session_ref=_text_or_none(getattr(session, "external_session_ref", None)),
            session_version=_int_or_none(getattr(session, "version", None)),
            mode_used=inv.claims.billing_mode,
            live=LiveBox(run.patch, self._now),
            anchored=str(getattr(payload, "trigger_kind", "message")) == "message",
            quotes=self._anchors.pop(run.run_id, False),
        )
        self._turns[run.thread_id] = turn
        self._by_run[run.run_id] = turn
        try:
            await self._run_turn(inv, turn)
        except asyncio.CancelledError:
            await self._abandon(turn)
            raise
        finally:
            self._forget(turn)

    async def _transcribed(
        self, run: RunHandle, ctx: RunContext, channel: ChannelBehavior | None
    ) -> BuiltContext | None:
        """The turn's texts with Hermes's transcripts of the trigger's audio (09 13, 17.6).

        Each audio attachment without a transcript is loaded whole and transcribed with Hermes's
        own speech to text; a transcript stands in the person's text with the trust of the
        message and is delivered with ``submitTranscript``; without one Hermes's note (the same
        as on its other platforms) goes before the person's text. A file the server refuses
        (``409 file_not_ready``, ``409 file_blocked``) is left as the context lists it.

        Returns:
            The new texts, or ``None`` when nothing changed.
        """
        transcripts: dict[UUID, str] = {}
        notes: list[str] = []
        for item in audio_to_transcribe(ctx):
            data = await self._download(run, item.file_id)
            if data is None:
                continue
            try:
                result = await self.runtime.transcribe(
                    data, str(item.original_name), str(item.mime_type)
                )
            except Exception as exc:  # the turn goes on without the transcript
                _LOG.warning("run %s: transcription failed: %s", run.run_id, type(exc).__name__)
                continue
            if result.transcript is not None:
                transcripts[item.file_id] = result.transcript
                await self._submit_transcript(run, item.file_id, result.transcript)
            elif result.note:
                notes.append(result.note)
        if not transcripts and not notes:
            return None
        built = await build_context(
            with_transcripts(ctx, transcripts),
            channel=channel,
            fetch=self._fetcher(run),
            attachment_limit=self.settings.attachment_text_max_bytes,
        )
        if not notes:
            return built
        prefix = "\n\n".join(notes)
        text = f"{prefix}\n\n{built.user_text}" if built.user_text.strip() else prefix
        return BuiltContext(user_side=built.user_side, user_text=text)

    async def _media(
        self, run: RunHandle, ctx: RunContext
    ) -> tuple[tuple[InboundMedia, ...], list[str]]:
        """The trigger's attachments in Hermes's media cache, and a note for each one that is not.

        An attachment the server does not serve, one over Hermes's limit, one the server refuses
        to hand out and one Hermes refuses to cache become a note; nothing is loaded for the
        first two. Logs carry codes only (SEC-LOG-2).
        """
        try:
            limit = int(self.runtime.media_limit())
        except Exception as exc:  # an unreadable limit is none; the server's upload limit holds
            _LOG.warning("media limit not readable: %s", type(exc).__name__)
            limit = 0
        media: list[InboundMedia] = []
        notes: list[str] = []
        for item in files_for_hermes(ctx):
            reason = scan_refusal(item)
            if reason is None and limit > 0 and int(item.size_bytes) > limit:
                reason = "too_large"
            if reason is None:
                cached, reason = await self._cache(run, item)
                if cached is not None:
                    media.append(cached)
                    continue
            _LOG.info("run %s: attachment not passed on code=%s", run.run_id, reason)
            notes.append(undelivered_note(item, reason or "unavailable"))
        return tuple(media), notes

    async def _cache(self, run: RunHandle, item: Any) -> tuple[InboundMedia | None, str | None]:
        """Load one attachment whole and put it into Hermes's media cache."""
        try:
            result = await run.client.call(download_file.asyncio_detailed, file_id=item.file_id)
        except ProblemError as exc:
            return None, problem_reason(exc.code)
        except httpx.TransportError:
            return None, "unavailable"
        if not isinstance(result, File):
            return None, "unavailable"
        try:
            cached = await self.runtime.cache_media(
                bytes(result.payload.read()), str(item.original_name), str(item.mime_type)
            )
        except Exception as exc:  # Hermes refused the bytes (its size cap, a path it rejects)
            _LOG.info("run %s: attachment not cached: %s", run.run_id, type(exc).__name__)
            return None, "unreadable"
        return (cached, None) if cached is not None else (None, "unreadable")

    async def _download(self, run: RunHandle, file_id: UUID) -> bytes | None:
        try:
            result = await run.client.call(download_file.asyncio_detailed, file_id=file_id)
        except ProblemError as exc:
            _LOG.info("run %s: audio not loaded code=%s", run.run_id, exc.code)
            return None
        except httpx.TransportError:
            _LOG.info("run %s: audio not loaded code=transport_error", run.run_id)
            return None
        if not isinstance(result, File):
            return None
        return bytes(result.payload.read())

    async def _submit_transcript(self, run: RunHandle, file_id: UUID, text: str) -> None:
        """``submitTranscript`` with the transcript ``ETag`` of ``getFile``; a refusal is logged.

        The server keeps it only for a voice message in mode ``agent`` (09 13); any other answer
        leaves the turn as it is.
        """
        client = run.client
        try:
            current = await client.call_detailed(get_file.asyncio_detailed, file_id=file_id)
            await client.call(
                submit_transcript.asyncio_detailed,
                file_id=file_id,
                body=TranscriptSubmit(text=text),
                if_match=current.headers.get("ETag", ""),
            )
        except ProblemError as exc:
            _LOG.info("run %s: transcript not submitted code=%s", run.run_id, exc.code)
        except httpx.TransportError:
            _LOG.info("run %s: transcript not submitted code=transport_error", run.run_id)

    async def _run_turn(self, inv: Invocation, turn: _Turn) -> None:
        payload = inv.payload
        request = TurnRequest(
            run_id=turn.run_id,
            thread_id=turn.thread_id,
            channel_id=payload.channel_id,
            user_id=inv.claims.sub,
            user_name=str(payload.triggered_by.display_name),
            # A command run has no triggering message; Hermes needs an ID for the turn's event.
            message_id=payload.message_id if turn.anchored else turn.run_id,
            user_text=turn.built.user_text,
            user_side=turn.built.user_side,
            media=turn.media,
            new_session=self._starts_session(turn),
        )
        switched = await self._before_turn(turn, request)
        if switched is SessionSwitch.NEW and not request.new_session:
            # A refused bind falls back to a new session (9.5): it loads the bound skills too.
            request = replace(request, new_session=True)
        outcome = await self.runtime.start_turn(request)
        await self._end_turn(turn, outcome, switched)

    def _starts_session(self, turn: _Turn) -> bool:
        """Whether the turn runs on a new Hermes session of the thread (`_before_turn`).

        ``new`` always does (the adapter resets a current session first); without a current
        session and without a session to bind, Hermes opens a new one as well. A ``resume`` or a
        ``bind`` continues a session.
        """
        if turn.session_mode == "new":
            return True
        return self.runtime.current_session(turn.thread_id) is None and turn.session_ref is None

    async def _before_turn(self, turn: _Turn, request: TurnRequest) -> SessionSwitch | None:
        """Bring the thread's Hermes session in line with ``session_mode`` (09 9.1, 9.5)."""
        mode = turn.session_mode
        current = self.runtime.current_session(turn.thread_id)
        if mode is None or mode == "new":
            if current is None:
                return None
            return await self.runtime.switch_session(request, "new", None)
        if mode == "resume" and (turn.session_ref is None or current == turn.session_ref):
            return None
        if turn.session_ref is None:
            return None
        return await self.runtime.switch_session(request, "bind", turn.session_ref)

    async def _end_turn(
        self, turn: _Turn, outcome: TurnOutcome, switched: SessionSwitch | None
    ) -> None:
        handle = turn.handle
        if turn.dropped or handle.finished:
            return
        turn.own_turns -= 1
        self._own_turn_ended(turn)
        if turn.live is not None:
            turn.live.end(failed=outcome is not TurnOutcome.COMPLETED)
        await self._flush_edits(turn)
        await self._report_session(turn, switched)
        if outcome is TurnOutcome.COMPLETED and turn.abort_reason is None:
            for message_id in turn.steered:
                with contextlib.suppress(ValueError):
                    handle.patch.followup_absorbed(message_id)
            # The run covers the work its turn left running (09 17.12, R-AG-44).
            await self._hold(turn)
            if turn.failed_turn or turn.failed_result:
                outcome = TurnOutcome.FAILED
            if _gone(turn):
                return  # cancelled or ended by the server while it held the work
        cancel = turn.abort_reason is not None or outcome is not TurnOutcome.COMPLETED
        if cancel:
            self._stopping[turn.thread_id] = turn.started_wall
        try:
            turn.ended = True
            if turn.live is not None:
                turn.live.end(failed=outcome is not TurnOutcome.COMPLETED)
            await self._flush_edits(turn)
            if cancel:
                # A run never ends while work it covers goes on unseen (R-AG-44).
                await self._cancel_work(turn)
        finally:
            self._stopped(turn)
        metrics, costs = self._usage(turn)
        if turn.abort_reason is not None:
            await handle.finish("failed", reason=turn.abort_reason, metrics=metrics, costs=costs)
            return
        if outcome is TurnOutcome.COMPLETED:
            if turn.session_mode == "new":
                await self._set_title(turn)
            await handle.finish("completed", metrics=metrics, costs=costs)
            return
        await handle.finish("failed", reason="runtime_error", metrics=metrics, costs=costs)

    # ---- background work of the run (09 17.12, R-AG-44) -------------------------------------

    async def _hold(self, turn: _Turn) -> None:
        """Keep the run open while the work its turns started runs, and for the turns it causes.

        Hermes' registries are read every ``background_poll_interval``; each unit is a step. The
        run may end when no unit is open, no Hermes turn of the thread runs, and, after the run
        saw background work, ``background_settle`` passed without a sign of a turn: the gateway
        admits the turn that reports a result a moment after the unit ended. A turn Hermes started
        itself that failed ends the run ``failed`` once the settle time passed after it (its
        error text arrives after the turn).
        """
        settings = self.settings
        turn.held = True
        turn.settled.set()
        turn.last_progress = time.monotonic()
        while not (turn.dropped or turn.abort_reason is not None or turn.failed_turn):
            open_work = await self._poll_work(turn)
            self._watch_session(turn)
            idle = turn.own_turns == 0 and not turn.internal
            if turn.seen_work and idle and turn.pending and not turn.failed_result:
                # A follow-up the first turn did not take, or one written while the run holds
                # work: a turn of its own, one after the other (17.12 point 5); never while a
                # turn Hermes started itself runs, and not after one failed (the follow-on run
                # takes it).
                self._start_followup(turn, turn.pending.pop(0))
            now = time.monotonic()
            if turn.failed_result and idle:
                if now - turn.last_busy >= settings.background_settle:
                    return
            elif open_work or turn.own_turns > 0 or turn.internal:
                turn.last_busy = now
            elif not turn.seen_work or now - turn.last_busy >= settings.background_settle:
                return
            if open_work and now - turn.last_progress >= settings.background_progress_interval:
                # Progress against the stall notice of 5.5 while work runs.
                turn.last_progress = now
                turn.progress_events += 1
                if not turn.dropped:
                    with self._bound(turn):
                        turn.handle.patch.set_metric("progress_events", turn.progress_events)
            turn.wake.clear()
            with contextlib.suppress(TimeoutError):
                await asyncio.wait_for(turn.wake.wait(), settings.background_poll_interval)

    def _watch_session(self, turn: _Turn) -> None:
        """Follow the turns of the thread's Hermes session by its busy state (17.12 point 3).

        Hermes calls ``on_session_end`` only when a turn's agent loop finalizes; a turn that ends
        with an error (a refused model request, exhausted retries, an interrupt, an exception)
        returns without it, while the session's busy state ends on every path. A busy session
        while no turn of the adapter runs is a turn Hermes started itself, also one whose
        ``pre_llm_call`` came before the hold or during a turn of the adapter; when the session
        is free again, that turn ended, failed if its ``on_session_end`` did not come.
        """
        if turn.own_turns > 0 or not (turn.seen_work or turn.internal):
            return
        running = self.runtime.turn_running(turn.thread_id)
        if running is None:
            return  # Hermes cannot tell: the hooks alone decide, as before
        if running:
            turn.internal = True
            return
        if turn.internal:
            self._internal_ended(turn, failed=turn.hermes_open)

    def _own_turn_ended(self, turn: _Turn) -> None:
        """A turn of the adapter returned: its ``pre_llm_call`` no longer counts as open.

        Only while the session is free: a busy session already runs the next turn.
        """
        if self.runtime.turn_running(turn.thread_id) is False:
            turn.hermes_open = False

    def _internal_ended(self, turn: _Turn, *, failed: bool) -> None:
        """A turn Hermes started itself ended; a failed one makes the run end ``failed``."""
        turn.internal = False
        turn.hermes_open = False
        if failed:
            turn.failed_result = True
            _LOG.info("run %s: a turn Hermes started itself failed", turn.run_id)
        if turn.live is not None and not turn.ended:
            with self._bound(turn):
                turn.live.end(failed=failed)
        self._touch(turn)

    async def _poll_work(self, turn: _Turn) -> bool:
        """Read the thread's background work and show it; returns whether a unit is open."""
        try:
            tasks = list(await self.runtime.background_work(turn.thread_id, turn.started_wall))
        except Exception as exc:  # unreadable registries: nothing to hold (logged)
            _LOG.warning(
                "run %s: background work not readable: %s", turn.run_id, type(exc).__name__
            )
            return False
        job_tasks = await self._job_tasks(turn)
        if job_tasks is not None:
            tasks += job_tasks
        if turn.dropped:
            return False
        if tasks:
            turn.seen_work = True
        with self._bound(turn):
            for task in tasks:
                self._show_task(turn, task)
            if job_tasks is not None:
                self._end_vanished_jobs(turn, {t.task_id for t in job_tasks})
            self._count_work(turn, tasks)
        return any(task.open for task in tasks)

    async def _job_tasks(self, turn: _Turn) -> list[BackgroundTask] | None:
        """The run's registered external jobs (17.12 point 8); ``None`` when not readable."""
        if self._jobs is None:
            return []
        try:
            return await asyncio.to_thread(
                self._read_jobs, self._jobs, turn.thread_id, turn.started_wall
            )
        except (JobError, OSError) as exc:
            _LOG.warning("run %s: registered jobs not readable: %s", turn.run_id, exc)
            return None

    @staticmethod
    def _read_jobs(jobs: JobRegistry, thread_id: UUID, since: float) -> list[BackgroundTask]:
        """The thread's jobs registered since the run started; live ones are open.

        A job whose process cannot be checked counts as open: it is never hidden.
        """
        found = []
        for record in jobs.records():
            if record.thread_id != thread_id or record.started_at < since:
                continue
            try:
                alive = jobs.alive(record)
            except JobError:
                alive = True
            found.append(
                BackgroundTask(
                    f"{_JOB_TASK}{record.job_id}",
                    BackgroundKind.JOB,
                    record.title,
                    "running" if alive else "completed",
                    alive,
                    name=EXTERNAL_JOB_STEP,
                )
            )
        return found

    def _end_vanished_jobs(self, turn: _Turn, seen: set[str]) -> None:
        """A job whose record was removed (deregistered at its end) completed its step."""
        for task_id, (step, shown) in turn.work_steps.items():
            if task_id.startswith(_JOB_TASK) and shown == "running" and task_id not in seen:
                turn.handle.patch.update_step(step, status="completed", finished_at=self._now())
                turn.work_steps[task_id] = (step, "completed")
                _LOG.info("run %s: background %s completed", turn.run_id, BackgroundKind.JOB)

    def _show_task(self, turn: _Turn, task: BackgroundTask) -> None:
        """Add or update the step of one unit (``subagent`` or ``background_job``)."""
        status = task.state if task.state in _ENDED_STATES else "running"
        known = turn.work_steps.get(task.task_id)
        patch = turn.handle.patch
        if known is None:
            step = patch.add_step(
                str(task.kind),
                task.name or _STEP_NAMES[task.kind],
                title=_one_line(task.title, _STEP_TITLE_MAX),
                status="running",
            )
            turn.work_steps[task.task_id] = (step, "running")
            _LOG.info("run %s: background %s started", turn.run_id, task.kind)
            known = (step, "running")
        step, shown = known
        if status != shown:
            patch.update_step(step, status=status, finished_at=self._now())
            turn.work_steps[task.task_id] = (step, status)
            _LOG.info("run %s: background %s %s", turn.run_id, task.kind, status)

    def _count_work(self, turn: _Turn, tasks: Any) -> None:
        """``/metrics/subagents`` and ``/metrics/background_jobs``: the open units (09 5.4)."""
        open_tasks = [t for t in tasks if t.open]
        counts = (
            sum(1 for t in open_tasks if t.kind is BackgroundKind.SUBAGENT),
            sum(1 for t in open_tasks if t.kind is BackgroundKind.JOB),
        )
        if counts == turn.work_counts:
            return
        turn.work_counts = counts
        turn.handle.patch.set_metric("subagents", counts[0])
        turn.handle.patch.set_metric("background_jobs", counts[1])

    async def _cancel_work(self, turn: _Turn) -> None:
        """Stop the work of the run: delegations interrupted, processes killed (R-AG-44).

        The run covers its registered jobs until they are stopped, so the job watch never takes
        a job being stopped for one without a run.
        """
        self._stopping[turn.thread_id] = turn.started_wall
        try:
            await self._stop_work(turn)
        finally:
            self._stopped(turn)

    def _stopped(self, turn: _Turn) -> None:
        """The run no longer stops its work; a job still alive now is one without a run."""
        if self._stopping.get(turn.thread_id) == turn.started_wall:
            del self._stopping[turn.thread_id]

    async def _stop_work(self, turn: _Turn) -> None:
        try:
            await self.runtime.cancel_background(turn.thread_id)
        except Exception as exc:  # the run still ends; the log names the failure
            _LOG.warning(
                "run %s: background work not cancelled: %s", turn.run_id, type(exc).__name__
            )
        if self._jobs is not None:
            try:
                await asyncio.to_thread(
                    self._stop_jobs, self._jobs, turn.thread_id, turn.started_wall
                )
            except (JobError, OSError) as exc:  # the run still ends; the log names the failure
                _LOG.warning("run %s: registered jobs not stopped: %s", turn.run_id, exc)
        if turn.dropped or turn.handle.finished:
            return
        with self._bound(turn):
            for task_id, (step, shown) in list(turn.work_steps.items()):
                if shown == "running":
                    turn.handle.patch.update_step(step, status="cancelled", finished_at=self._now())
                    turn.work_steps[task_id] = (step, "cancelled")
            if turn.work_counts != (0, 0):
                turn.work_counts = (0, 0)
                turn.handle.patch.set_metric("subagents", 0)
                turn.handle.patch.set_metric("background_jobs", 0)

    @staticmethod
    def _stop_jobs(jobs: JobRegistry, thread_id: UUID, since: float) -> None:
        """Stop the run's live registered jobs, each after its PID and start time matched."""
        for record in jobs.records():
            if record.thread_id == thread_id and record.started_at >= since:
                jobs.stop(record)

    # ---- registered jobs without an open run (09 17.12 point 8) ------------------------------

    def start_job_watch(self) -> asyncio.Task[None]:
        """Start `sweep_jobs` every ``background_poll_interval``; the caller cancels the task."""
        return asyncio.ensure_future(self._watch_jobs())

    async def _watch_jobs(self) -> None:
        while True:
            try:
                await self.sweep_jobs()
            except Exception as exc:  # the watch goes on; the log names the failure
                _LOG.warning("registered jobs not swept: %s", type(exc).__name__)
            await asyncio.sleep(self.settings.background_poll_interval)

    async def sweep_jobs(self) -> None:
        """Remove the records of ended jobs; post each live job no open run covers, once.

        A run covers a job of its thread that started after the run started. Any other live job
        (its run ended at a restart, or it was registered outside a run) is not adopted by a
        later run: it is posted once into its thread as a message of the agent without a run,
        and the mark that it was posted outlives a restart.
        """
        jobs = self._jobs
        if jobs is None:
            return
        try:
            live = await asyncio.to_thread(jobs.sweep)
        except (JobError, OSError) as exc:
            _LOG.warning("registered jobs not readable: %s", exc)
            return
        self._posted_jobs &= {record.job_id for record in live}
        for record in live:
            if self._covers(record):
                continue
            if record.job_id in self._posted_jobs:
                continue
            try:
                if await asyncio.to_thread(jobs.reported, record.job_id):
                    continue
                if await self._post_job(record):
                    self._posted_jobs.add(record.job_id)
                    await asyncio.to_thread(jobs.mark_reported, record.job_id)
            except (JobError, OSError) as exc:
                _LOG.warning("job %s: notice not recorded: %s", record.job_id, exc)

    def _covers(self, record: JobRecord) -> bool:
        """Whether an open run of the job's thread started before the job (17.12 point 8)."""
        stopping = self._stopping.get(record.thread_id)
        if stopping is not None and record.started_at >= stopping:
            return True  # its run stops it now
        turn = self._turns.get(record.thread_id)
        return (
            turn is not None
            and self.tool_allowed(record.thread_id)
            and record.started_at >= turn.started_wall
        )

    async def _post_job(self, record: JobRecord) -> bool:
        """Post a live job without a run; returns whether it is done (posted or refused)."""
        if self._client is None:
            return False
        since = datetime.fromtimestamp(record.started_at, UTC).strftime("%Y-%m-%d %H:%M UTC")
        text = JOB_WITHOUT_RUN.format(
            title=_one_line(record.title, _STEP_TITLE_MAX),
            pid=record.pid,
            since=since,
            job_id=record.job_id,
        )
        try:
            await self._post_own(record.thread_id, text)
        except ReportError:
            # Refused by the server (a thread the agent may not write): not asked again.
            _LOG.info("job %s: notice refused", record.job_id)
            return True
        except (httpx.HTTPError, OSError) as exc:  # a transient failure: the next sweep again
            _LOG.info("job %s: notice not posted: %s", record.job_id, type(exc).__name__)
            return False
        _LOG.info("job %s: posted without a run", record.job_id)
        return True

    def _touch(self, turn: _Turn) -> None:
        """A sign of a turn of the run: the settle time starts again, the hold looks at once."""
        turn.last_busy = time.monotonic()
        turn.wake.set()

    def _start_followup(self, turn: _Turn, message: Message) -> None:
        """Start a follow-up's own turn; the run counts it as running at once."""
        turn.own_turns += 1
        self._spawn(self._followup_turn(turn, message))

    async def _followup_turn(self, turn: _Turn, message: Message) -> None:
        """A follow-up while the run holds background work: a turn of its own in the run.

        The person's text is the turn's text, the user side carries the format rules and the
        channel instructions (the run context rode the run's first turn); the follow-up is
        ``delivered`` at the start and ``absorbed`` when the turn completed (09 17.12).
        """
        message_id = message.id
        run = turn.handle
        turn.final_reply = False
        turn.steered.append(message_id)
        self._touch(turn)
        author = message.author
        request = TurnRequest(
            run_id=turn.run_id,
            thread_id=turn.thread_id,
            channel_id=turn.channel_id,
            user_id=author.id,
            user_name=str(getattr(author, "display_name", "") or ""),
            message_id=message_id,
            user_text=_content(message),
            user_side=turn.built.user_side,
            context_taken=True,
        )
        outcome = TurnOutcome.FAILED
        try:
            with contextlib.suppress(ValueError):
                run.patch.followup_delivered(message_id)
            outcome = await self.runtime.start_turn(request)
        finally:
            turn.own_turns -= 1
            self._own_turn_ended(turn)
            self._touch(turn)
        if turn.dropped or turn.handle.finished:
            return
        if turn.live is not None:
            turn.live.end(failed=outcome is not TurnOutcome.COMPLETED)
        await self._flush_edits(turn)
        if outcome is TurnOutcome.COMPLETED:
            with contextlib.suppress(ValueError):
                run.patch.followup_absorbed(message_id)
        else:
            turn.failed_turn = True

    def _usage(self, turn: _Turn) -> tuple[RunMetrics | None, list[CostReport]]:
        if not turn.usage:
            return None, []
        per_model: dict[tuple[str, str, str], dict[str, int]] = {}
        estimates: dict[tuple[str, str, str], int | None] = {}
        for item in turn.usage:
            key = (item.provider, item.model, item.currency or DEFAULT_CURRENCY)
            sums = per_model.setdefault(key, {"in": 0, "out": 0, "read": 0, "write": 0})
            sums["in"] += item.tokens_in
            sums["out"] += item.tokens_out
            sums["read"] += item.tokens_cache_read
            sums["write"] += item.tokens_cache_write
            if item.amount_micros is None:
                estimates[key] = None
            elif key not in estimates or estimates[key] is not None:
                estimates[key] = (estimates.get(key) or 0) + item.amount_micros
        costs = []
        for (provider, model, currency), sums in per_model.items():
            cost = CostReport(
                provider=provider,
                model=model,
                currency=currency,
                tokens_in=sums["in"],
                tokens_out=sums["out"],
                tokens_cache_read=sums["read"],
                tokens_cache_write=sums["write"],
            )
            estimate = estimates.get((provider, model, currency))
            if estimate is not None:
                cost.amount_micros = estimate
            costs.append(cost)
        metrics = RunMetrics(
            tokens_in=sum(u.tokens_in for u in turn.usage),
            tokens_out=sum(u.tokens_out for u in turn.usage),
            tokens_cache_read=sum(u.tokens_cache_read for u in turn.usage),
            tokens_cache_write=sum(u.tokens_cache_write for u in turn.usage),
        )
        return metrics, costs

    async def _report_session(self, turn: _Turn, switched: SessionSwitch | None) -> None:
        """``reportAgentSession`` for operator runs (09 9.1, 9.5; R-AG-04)."""
        if turn.session_mode is None:
            return  # a guest run has no session
        current = self.runtime.current_session(turn.thread_id)
        if current is None:
            return
        body = AgentSessionReport(external_session_ref=current)
        mode = turn.session_mode
        if mode == "new":
            body.started = True
        elif mode == "bind":
            # Hermes took the session over when the turn ran on it or on the living end of its
            # compression chain; otherwise it kept the new session of the switch (fail open).
            taken = turn.session_ref in (current, self.runtime.session_parent(turn.thread_id))
            if not taken or switched is SessionSwitch.MISSING:
                body.bind_failed = AgentSessionReportBindFailed.SESSION_MISSING
                body.started = True
        elif current == turn.session_ref:
            return  # resume without a change: nothing to report
        if_match = f'"{turn.session_version}"' if turn.session_version is not None else None
        try:
            if if_match is None:
                await turn.handle.client.call(
                    report_agent_session.asyncio_detailed, thread_id=turn.thread_id, body=body
                )
            else:
                await turn.handle.client.call(
                    report_agent_session.asyncio_detailed,
                    thread_id=turn.thread_id,
                    body=body,
                    if_match=if_match,
                )
        except ProblemError as exc:
            _LOG.info("run %s: session report refused code=%s", turn.run_id, exc.code)

    async def _set_title(self, turn: _Turn) -> None:
        """The title after the first completed run of a new session (09 12 points 3 to 5).

        It is an automatic title (``auto``): the server only lets it fill a thread without a
        meaningful title, so a status title the agent set with its thread action, or one a
        person set, stays when the owner answers in a new session.
        """
        title = self.runtime.session_title(turn.thread_id)
        if not title or not title.strip():
            return
        client = turn.handle.client
        try:
            thread = await client.call_as(
                Thread, get_thread.asyncio_detailed, thread_id=turn.thread_id
            )
            if thread.title_source != "auto":
                return
            await client.call(
                set_thread_title.asyncio_detailed,
                thread_id=turn.thread_id,
                body=ThreadTitlePut(title=title.strip(), auto=True),
                if_match=f'"{thread.version}"',
            )
        except ProblemError as exc:
            if exc.status not in _STOP_STATUSES:
                _LOG.info("run %s: title not set code=%s", turn.run_id, exc.code)
            else:
                _LOG.info("run %s: title refused code=%s", turn.run_id, exc.code)

    async def _abandon(self, turn: _Turn) -> None:
        """A cancelled run: interrupt the turn, cancel its work, drop later callbacks (09 8)."""
        self._stopping[turn.thread_id] = turn.started_wall
        try:
            turn.dropped = True
            if turn.live is not None:
                turn.live.drop()
            await self._interrupt(turn)
            await self._cancel_work(turn)
        finally:
            self._stopped(turn)

    async def _interrupt(self, turn: _Turn) -> None:
        if turn.interrupted:
            return
        turn.interrupted = True
        if turn.live is not None:
            turn.live.drop()
        try:
            await self.runtime.interrupt(turn.thread_id)
        except Exception as exc:  # the run still ends through the SDK
            _LOG.warning("run %s: interrupt failed: %s", turn.run_id, type(exc).__name__)

    def _forget(self, turn: _Turn) -> None:
        turn.ended = True
        turn.settled.set()
        if self._turns.get(turn.thread_id) is turn:
            del self._turns[turn.thread_id]
        self._by_run.pop(turn.run_id, None)
        self._prepared.pop(turn.run_id, None)
        self._anchors.pop(turn.run_id, None)
        self._inbound.pop(turn.run_id, None)
        self._spawn(self.store.drop_card(turn.run_id))

    def _spawn(self, coro: Any) -> None:
        task = asyncio.ensure_future(coro)
        self._tasks.add(task)
        task.add_done_callback(self._tasks.discard)
        task.add_done_callback(_log_task_failure)

    # ---- follow-ups and interactions ---------------------------------------------------------

    async def on_followup(self, inv: Invocation, run: RunHandle) -> None:
        """A follow-up of a running or waiting run (09 7, 11.6, 17.5; Design note 11).

        Args:
            inv: The ``agent.followup``.
            run: The run.
        """
        turn = self._by_run.get(run.run_id)
        if turn is None or turn.dropped:
            return
        message_id = inv.payload.message_id
        if turn.card is not None:
            # The answer to the open card, in both modes: the SDK resumed the run already.
            card = turn.card
            await self._disable_card(turn)
            text = await self._followup_text(run, message_id)
            if text is not None and await self.store.claim(_claim_key(turn.run_id, card)):
                if card.request.kind is not CardKind.CLARIFY:
                    # Not an approval: the text reaches the still blocked turn first, then the
                    # prompt is refused, so the turn takes the text before it goes on.
                    await self._into_turn(turn, text, message_id)
                await self.runtime.answer_card(card, text)
            await self._until_settled(turn)
            return
        message = await self._followup_message(run, message_id)
        if message is None or _has_media(message):
            # Hermes steers text only and queues a message with media as a turn of its own; left
            # undelivered, the server's follow-on run takes it as its triggering message, whose
            # files reach Hermes as media (09 7; D-FIX-HIMG-3).
            return
        if turn.held:
            # The run holds background work: the follow-up is a turn of its own (09 17.12).
            turn.pending.append(message)
            self._touch(turn)
            return
        text = _content(message)
        if await self._into_turn(turn, text, message_id):
            with contextlib.suppress(ValueError):
                run.patch.followup_delivered(message_id)
            turn.steered.append(message_id)
        else:
            # Not taken by this turn: a turn of its own if the run comes to hold background
            # work; otherwise it stays queued for the follow-on run (09 7).
            turn.pending.append(message)

    async def _into_turn(self, turn: _Turn, text: str, message_id: UUID) -> bool:
        """Pass a person's text into the running turn: the one place that may (09 7, 17.5).

        Only at ``steer`` and only while the turn runs and its last model reply called a tool: at
        ``queue``, after a reply without tool calls (the answer) and after the end Hermes would
        start a new turn with the text, which no run covers. Hermes reports the reply before it
        drains its last steer; a message before a tool call (sent the same way as the answer)
        leaves the turn open.

        Returns:
            Whether the text was passed.
        """
        if self.busy_mode != BusyMode.STEER or turn.ended or turn.dropped or turn.final_reply:
            # After the answer Hermes may have drained its last steer already; the server's
            # follow-on run takes the follow-up instead (09 7).
            return False
        await self.runtime.steer(turn.thread_id, text, message_id)
        return True

    async def _followup_text(self, run: RunHandle, message_id: UUID) -> str | None:
        """The text of a follow-up: its message read with the run's token (``message.read``).

        The run context holds the messages before the triggering message only (09 4.6
        ``thread_context``), never a follow-up, which comes after it; transcripts of follow-ups
        are not made (17.6). A message the server refuses passes nothing.
        """
        message = await self._followup_message(run, message_id)
        return None if message is None else _content(message)

    async def _followup_message(self, run: RunHandle, message_id: UUID) -> Message | None:
        """A follow-up message read with the run's token, or ``None`` when the server refuses."""
        try:
            message = await run.client.call_as(
                Message, get_message.asyncio_detailed, message_id=message_id
            )
        except ProblemError as exc:
            _LOG.info("run %s: follow-up not read code=%s", run.run_id, exc.code)
            return None
        except httpx.TransportError:
            _LOG.info("run %s: follow-up not read code=transport_error", run.run_id)
            return None
        return message

    async def _disable_card(self, turn: _Turn) -> None:
        card = turn.card
        turn.card = None
        if card is None:
            return
        await self.store.drop_card(turn.run_id)
        versions, turn.card_versions = turn.card_versions, {}
        for component_id, version in versions.items():
            try:
                await turn.handle.client.call(
                    update_component.asyncio_detailed,
                    component_id=component_id,
                    body=ComponentUpdate(state=ComponentUpdateState.DISABLED),
                    if_match=f'"{version}"',
                )
            except ProblemError as exc:  # the server disabled it with the follow-up already
                _LOG.info("run %s: component not disabled code=%s", turn.run_id, exc.code)

    async def on_interaction(self, inv: Invocation, run: RunHandle) -> None:
        """An interaction with a card of the run (09 11.4, 17.3; Design note 12).

        Args:
            inv: The ``interaction.created``.
            run: The run (resumed by the SDK with the interaction's token).
        """
        interaction = inv.payload.interaction
        turn = self._by_run.get(run.run_id)
        custom_id = str(interaction.custom_id) if isinstance(interaction.custom_id, str) else ""
        card = turn.card if turn is not None else None
        label = cards.chosen_label(card, custom_id, _values(interaction)) if card else None
        if turn is None or card is None or label is None:
            await self._respond(run, interaction.id, InteractionRespondState.FAILED, None)
            if turn is not None and card is not None and not turn.dropped:
                await run.await_input(cards.status_line(card.request))
            return
        if not await self.store.claim(_claim_key(turn.run_id, card)):
            # This card of this run was answered before; nothing resolves twice, and the run
            # keeps waiting for its card instead of ending without an answer.
            await self._respond(run, interaction.id, InteractionRespondState.FAILED, None)
            if not turn.dropped:
                await run.await_input(cards.status_line(card.request))
            return
        turn.card = None
        await self.store.drop_card(turn.run_id)
        resolved = await self.runtime.resolve_callback(
            card, custom_id, _values(interaction), interaction.principal_id
        )
        if resolved is None:
            _LOG.info("run %s: nothing waited for the callback", run.run_id)
        await self._respond(run, interaction.id, InteractionRespondState.ACKNOWLEDGED, label)
        await self._until_settled(turn)

    async def _respond(
        self,
        run: RunHandle,
        interaction_id: UUID,
        state: InteractionRespondState,
        label: str | None,
    ) -> None:
        body = InteractionRespond(state=state)
        if label:
            body.response = InteractionResponse(text=label[: cards.MAX_RESPONSE_CHARS])
        try:
            await run.client.call(
                respond_to_interaction.asyncio_detailed, interaction_id=interaction_id, body=body
            )
        except ProblemError as exc:
            _LOG.info("run %s: interaction answer refused code=%s", run.run_id, exc.code)

    async def _until_settled(self, turn: _Turn) -> None:
        """Wait until the turn ended or waits again, so the SDK does not end the run early.

        While the run holds background work the run's ``execute`` keeps it open, so nothing waits.
        """
        turn.settled.clear()
        if turn.ended or turn.held or turn.card is not None:
            return
        try:
            await turn.settled.wait()
        except asyncio.CancelledError:
            await self._abandon(turn)
            raise

    # ---- commands and reports without a run -------------------------------------------------

    async def on_command(self, inv: Invocation, run: RunHandle | None) -> None:
        """A control command (``command.invoked``): run it in Hermes, post the reply (09 10.4).

        Only the control commands of `bothy_hermes_adapter.hermes_commands` run; Hermes gets the
        bare command line from the adapter's control sender, never an option value. Each text
        Hermes answers becomes a message of the agent in the thread, without a run.

        Args:
            inv: The ``command.invoked`` invocation (its token was checked by the SDK).
            run: The adapter's run the command names, if any (not used: no run is changed).
        """
        del run
        payload = inv.payload
        name = str(getattr(payload, "command_name", ""))
        thread_id = getattr(payload, "thread_id", None)
        known = COMMAND_CLASSES.get(name, ("", ""))[0] == "control"
        if not getattr(payload, "control", False) or not known or not isinstance(thread_id, UUID):
            _LOG.info("command %s not run: no control command of this adapter", payload.command_id)
            return
        person = getattr(payload, "invoked_by", None)
        request = CommandRequest(
            thread_id=thread_id,
            channel_id=payload.channel_id,
            user_id=inv.claims.sub,
            user_name=str(getattr(person, "display_name", "") or ""),
            text=command_text(name, option_values(getattr(payload, "options", None))),
        )
        try:
            texts = await self.runtime.run_command(request)
        except Exception as exc:  # Hermes failed the command; the person sees no reply
            _LOG.warning("command %s failed in Hermes: %s", payload.command_id, type(exc).__name__)
            return
        for text in texts:
            if not text.strip():
                continue
            try:
                await self._post_own(thread_id, text)
            except Exception as exc:  # a refusal or an unreachable server: the reply is lost
                _LOG.info(
                    "command %s: reply not posted: %s", payload.command_id, type(exc).__name__
                )

    async def _post_own(self, thread_id: UUID, text: str) -> UUID | None:
        """A message of the agent without a run (09 14): one ``createMessage``.

        Not through the outbox: a refusal such as ``403`` must reach Hermes as a failure at
        once, while the outbox keeps such an entry pending and sends it again at every
        reconnect. The client repeats the same ``client_msg_id`` on a transient failure, so a
        retry never posts twice.
        """
        client = self._client
        if client is None:
            raise ReportError("the adapter is not connected")
        body, was_cut = cut_utf8(text.encode("utf-8"), self.settings.max_message_bytes)
        if was_cut:
            _LOG.info("thread %s: message cut to the message limit", thread_id)
        try:
            with client.unbound():
                return await direct_send(client, thread_id, body)
        except ProblemError as exc:
            _LOG.info("thread %s: message refused code=%s", thread_id, exc.code)
            raise ReportError(f"refused by the server: {exc.code}") from None

    async def on_report(self, chat_id: str, thread_id: str | None, text: str) -> UUID | None:
        """A message Hermes sends without an invocation: a report of the agent (09 14, 17.9).

        Args:
            chat_id: Hermes' ``chat_id`` (the channel).
            thread_id: Hermes' ``thread_id`` (a fixed thread), if any.
            text: The report.

        Returns:
            The message or the new forum post.

        Raises:
            ReportError: Nothing was posted; the reason names no content.
        """
        target = parse_target(chat_id, thread_id)
        client = self._client
        if client is None:
            raise ReportError("the adapter is not connected")
        return await post_report(
            client,
            target,
            text,
            max_bytes=self.settings.max_message_bytes,
            send=self._post_own,
        )

    async def on_event(self, envelope: Envelope) -> None:
        """``agent.config_changed``: forget the cached channel configuration.

        Args:
            envelope: The event.
        """
        if envelope.type == "agent.config_changed":
            self._channels.clear()

    # ---- what Hermes reports during a turn (RuntimeEvents) -----------------------------------

    def _active(self, thread_id: UUID) -> _Turn | None:
        turn = self._turns.get(thread_id)
        if turn is None or turn.dropped or turn.ended or turn.live is None:
            return None
        if turn.abort_reason is not None:
            return None  # the run ends with the abort; nothing of the turn is sent any more
        return turn

    @contextlib.contextmanager
    def _bound(self, turn: _Turn) -> Iterator[None]:
        with turn.handle.bound():
            yield

    def on_status(self, thread_id: UUID, text: str | None) -> None:
        """A status sentence (``set_status_text``); opens or closes a step."""
        turn = self._active(thread_id)
        if turn is not None and turn.live is not None:
            if text is not None and turn.held and turn.own_turns == 0:
                turn.internal = True  # a turn Hermes started itself is at work (17.12)
            self._touch(turn)
            with self._bound(turn):
                turn.live.status(text)

    def on_tool_event(self, thread_id: UUID, preview: str, call_id: str | None, done: bool) -> None:
        """A tool event (``format_tool_event``); opens or closes a step."""
        turn = self._active(thread_id)
        if turn is not None and turn.live is not None:
            with self._bound(turn):
                turn.live.tool_event(preview, call_id, done)

    def on_draft(self, thread_id: UUID, text: str) -> None:
        """A draft (``send_draft``); replaces ``/last_text_excerpt``."""
        turn = self._active(thread_id)
        if turn is not None and turn.live is not None:
            with self._bound(turn):
                turn.live.draft(text)

    async def on_answer(self, thread_id: UUID, text: str, reply_to: str | None) -> str | None:
        """The answer (``send``): one message of the run with a stored ``client_msg_id``.

        Returns:
            The ID Hermes may edit later, or ``None`` when no run of this adapter is active in the
            thread (nothing is sent).
        """
        turn = self._active(thread_id)
        if turn is None or turn.live is None:
            return None
        # Recorded before the send: whatever the outbox does with it, a later copy from Hermes'
        # delivery ledger is the same answer (`recovered_duplicate`).
        await self.store.put_answer(turn.thread_id, text)
        with self._bound(turn):
            turn.live.answer()
            # One message, never several (M4-47): above max_message_bytes the head stays the
            # text and the rest goes as one attachment (D-M4-47-5).
            entry = await send_long_message(
                turn.handle.client,
                turn.handle.outbox,
                turn.thread_id,
                turn.channel_id,
                text,
                max_content_bytes=self.settings.max_message_bytes,
                run_id=turn.run_id,
                reply_to_message_id=_quote(turn, reply_to),
            )
        self._touch(turn)
        if entry.message_id is None:
            return None
        key = str(entry.message_id)
        turn.answers[key] = (entry.message_id, int(entry.message_version or 1))
        return key

    async def recovered_duplicate(self, thread_id: UUID | None, content: str) -> bool:
        """Whether ``content`` is a reply Hermes sends again that this adapter took over already.

        Hermes' delivery ledger sends a final reply again, behind a notice that it may be a
        duplicate, when the first send failed or the gateway restarted during it. An answer the
        adapter took over for a run is the outbox's to deliver, also after a restart, and a
        refused one ended its run with the error; posting the copy as a report without a run
        would show the text twice or detached from its run (09 17.3).

        Args:
            thread_id: The thread Hermes names (its ``metadata`` ``thread_id``), if any.
            content: The text of the send.

        Returns:
            True when the send carries the notice and its reply was an answer in the thread.
        """
        reply = recovered_reply(content)
        if reply is None or thread_id is None:
            return False
        return await self.store.answered(thread_id, reply)

    async def on_edit(self, thread_id: UUID, message_id: str, text: str, finalize: bool) -> bool:
        """An edit of an own answer: buffered; one ``updateMessage`` at ``finalize``.

        Returns:
            Whether the message is an own answer of the active run.
        """
        turn = self._active(thread_id)
        if turn is None or message_id not in turn.answers:
            return False
        turn.edits[message_id] = text
        if finalize:
            with self._bound(turn):
                await self._flush_edits(turn, only=message_id)
        return True

    async def _flush_edits(self, turn: _Turn, only: str | None = None) -> None:
        for key in [only] if only is not None else list(turn.edits):
            text = turn.edits.pop(key, None) if key is not None else None
            if text is None or key is None:
                continue
            message_id, version = turn.answers[key]
            try:
                result = await turn.handle.client.call(
                    update_message.asyncio_detailed,
                    message_id=message_id,
                    body=MessageUpdate(content_md=self._cut(turn, text)),
                    if_match=f'"{version}"',
                )
            except ProblemError as exc:
                _LOG.info("run %s: answer edit refused code=%s", turn.run_id, exc.code)
                continue
            new_version = getattr(result, "version", None)
            turn.answers[key] = (message_id, int(new_version) if new_version else version + 1)

    def _cut(self, turn: _Turn, text: str) -> str:
        cut, was_cut = cut_utf8(text.encode("utf-8"), self.settings.max_message_bytes)
        if was_cut:
            _LOG.info("run %s: answer cut to the message limit", turn.run_id)
        return cut

    async def on_file(
        self, thread_id: UUID, path: str, kind: MediaKind, caption: str | None, name: str | None
    ) -> FileSent:
        """A file Hermes sends with its answer: upload it and send it attached (09 17.3).

        The file must be a regular file (a symbolic link is not followed) within the server's
        ``file_max_upload_bytes``; it goes through an upload session of the run's channel and
        then as one message ``kind = text`` of the run with the caption as its text, a voice
        too: the server refuses ``kind = voice`` until its scan has made the file final (12
        8.1, ``409 file_not_ready``), and the scan runs only after ``completeUpload`` (12 3.4).
        Nothing is sent when a step fails; Hermes then tells the person itself.

        Returns:
            The message, or the code of why nothing was sent.
        """
        del kind  # every file goes as kind = text, a voice too (see above)
        turn = self._active(thread_id)
        if turn is None:
            return FileSent(None, "no_active_run")
        with self._bound(turn):
            limit = await self._upload_limit(turn)
            if limit is None:
                return FileSent(None, "limits_unavailable")
            try:
                local = await asyncio.to_thread(read_local_file, path, name, limit)
            except FileRefused as exc:
                _LOG.info("run %s: file not sent code=%s", turn.run_id, exc.code)
                return FileSent(None, exc.code)
            try:
                file_id = await upload(turn.handle.client, turn.channel_id, local)
            except FileRefused as exc:
                _LOG.info("run %s: file not sent code=%s", turn.run_id, exc.code)
                return FileSent(None, exc.code)
            finally:
                local.close()
            if turn.live is not None:
                turn.live.answer()
            entry = await turn.handle.send(
                self._cut(turn, caption or ""),
                kind="text",
                attachment_file_ids=[file_id],
            )
        if entry.message_id is None:
            code = entry.problem_code or "not_sent"
            _LOG.info("run %s: file not sent code=%s", turn.run_id, code)
            return FileSent(None, code)
        return FileSent(str(entry.message_id))

    async def _upload_limit(self, turn: _Turn) -> int | None:
        """``InstanceLimits.file_max_upload_bytes``, read once per run."""
        if turn.upload_limit is None:
            try:
                info = await turn.handle.client.call_as(InstanceInfo, get_instance.asyncio_detailed)
            except ProblemError as exc:
                _LOG.info("run %s: limits not read code=%s", turn.run_id, exc.code)
                return None
            except httpx.TransportError:
                _LOG.info("run %s: limits not read code=transport_error", turn.run_id)
                return None
            turn.upload_limit = int(info.limits.file_max_upload_bytes)
        return turn.upload_limit

    async def on_card(self, thread_id: UUID, request: CardRequest) -> bool:
        """A Hermes prompt: a card of the run, then ``awaitRunInput`` (09 11; Design note 12).

        Returns:
            Whether the card was shown.
        """
        turn = self._active(thread_id)
        if turn is None:
            return False
        try:
            components = cards.build_components(request, turn.handle.client.settings)
        except ValueError:
            _LOG.warning("run %s: card refused by the component rules", turn.run_id)
            turn.abort_reason = "adapter_error"
            self._spawn(self._interrupt(turn))
            return False
        with self._bound(turn):
            if turn.live is not None:
                turn.live.answer()
            entry = await turn.handle.send(
                cards.card_text(request), kind="card", components=components
            )
            if entry.created is None:
                _LOG.warning("run %s: card not stored code=%s", turn.run_id, entry.problem_code)
                turn.abort_reason = "adapter_error"
                self._spawn(self._interrupt(turn))
                return False
            ids = {
                str(c.custom_id): c.id
                for c in entry.created.components
                if isinstance(c.custom_id, str)
            }
            versions = {str(c.custom_id): int(c.version) for c in entry.created.components}
            card = OpenCard(turn.run_id, request, entry.created.id, ids)
            await self.store.put_card(
                turn.run_id,
                {
                    "kind": str(request.kind),
                    "hermes_id": request.hermes_id,
                    "session_key": request.session_key,
                    "message_id": str(entry.created.id),
                    "components": {k: [str(v), versions.get(k, 1)] for k, v in ids.items()},
                },
            )
            turn.card = card
            turn.card_versions = {ids[k]: versions.get(k, 1) for k in ids}
            await turn.handle.await_input(cards.status_line(request))
        turn.settled.set()
        return True

    def on_usage(self, thread_id: UUID, usage: Usage) -> None:
        """Token counts of one model request of the running turn."""
        turn = self._turns.get(thread_id)
        if turn is not None and not turn.dropped:
            turn.usage.append(usage)

    def on_model_reply(self, thread_id: UUID, tool_calls: int | None) -> None:
        """A model reply of the running turn: without tool calls (or unknown) it is the last one.

        Hermes sends text before a tool call the same way as its answer, so only the reply's tool
        calls tell whether the turn goes on and still takes a person's text (09 17.5).
        """
        turn = self._turns.get(thread_id)
        if turn is not None and not turn.dropped:
            turn.final_reply = not tool_calls

    def on_turn_started(self, thread_id: UUID) -> None:
        """Hermes started a turn of the thread (``pre_llm_call``).

        While no turn of the adapter runs (the run holds background work, or its first turn just
        returned), it is a turn Hermes started itself (the result of a delegation or a process): a
        turn of the run (17.12). Without an open run the turn has no run; its tool calls are
        refused (R-AG-44).
        """
        turn = self._active(thread_id)
        if turn is None:
            _LOG.info("thread %s: a Hermes turn without a run", thread_id)
            return
        turn.hermes_open = True
        if turn.own_turns == 0:
            turn.internal = True
        self._touch(turn)

    def on_turn_ended(self, thread_id: UUID, *, failed: bool = False) -> None:
        """A turn of the thread ended its agent loop (``on_session_end``).

        A turn Hermes started itself ends even while a turn of the adapter is counted: the run
        must not wait for it any more (17.12). One Hermes reports ``failed`` (or interrupted)
        makes the run end ``failed``.

        Args:
            thread_id: The thread of the turn.
            failed: Whether Hermes reports the turn failed or interrupted.
        """
        turn = self._turns.get(thread_id)
        if turn is None or turn.dropped:
            return
        turn.hermes_open = False
        if not turn.internal and (turn.own_turns > 0 or not turn.held):
            return  # the end of a turn of the adapter: its outcome tells how it ended
        self._internal_ended(turn, failed=failed)

    def tool_allowed(self, thread_id: UUID | None) -> bool:
        """Whether a tool may run now in the thread: only while a run of it is open (R-AG-44).

        Hermes calls it from its agent threads (``pre_tool_call``), so it only reads.

        Args:
            thread_id: The thread of the tool call, ``None`` when it cannot be told.

        Returns:
            True while a run of the adapter in the thread is open and not ending.
        """
        if thread_id is None:
            return False
        turn = self._turns.get(thread_id)
        return (
            turn is not None
            and not turn.dropped
            and not turn.ended
            and turn.abort_reason is None
            and not turn.handle.finished
        )

    # ---- for tests and the platform ----------------------------------------------------------

    def active_run(self, thread_id: UUID) -> UUID | None:
        """The run whose turn is active in a thread, if any.

        Args:
            thread_id: The thread.

        Returns:
            The run, or ``None``.
        """
        turn = self._active(thread_id)
        return turn.run_id if turn is not None else None


def _gone(turn: _Turn) -> bool:
    """Whether the run was cancelled or has ended (read again after an ``await``)."""
    return turn.dropped or turn.handle.finished


def _one_line(text: str, limit: int) -> str:
    """A step title: one line without control or direction characters, at most ``limit``."""
    cleaned = "".join(
        " " if unicodedata.category(c) in ("Cc", "Cf", "Zl", "Zp") else c for c in text
    )
    words = " ".join(cleaned.split())
    return words if len(words) <= limit else words[: limit - 3].rstrip() + "..."


def _quote(turn: _Turn, reply_to: str | None) -> UUID | None:
    """The message an answer quotes: only one that lies in the run's thread (09 17.3).

    Hermes names the message of the turn's event: the triggering message, which may lie in the
    main thread while the run lives in its auto thread (`_Turn.quotes` is then false), or a
    follow-up passed into the turn, which always lies in the run's thread. A turn without a
    triggering message (a command run) quotes nothing.
    """
    target = _uuid_or_none(reply_to)
    if target is None or not turn.anchored:
        return None
    if target in turn.steered:
        return target
    return target if turn.quotes else None


def _claim_key(run_id: UUID, card: OpenCard) -> str:
    """The once-only key of a card: its run and Hermes' ID (a new process may reuse the ID)."""
    return f"{run_id}:{card.request.kind}:{card.request.hermes_id}"


def _values(interaction: Any) -> Mapping[str, Any]:
    values = getattr(interaction, "values", None)
    if values is None or isinstance(values, Unset):
        return {}
    to_dict = getattr(values, "to_dict", None)
    return dict(to_dict()) if callable(to_dict) else dict(values)


def _operator_run(payload: Any) -> bool:
    return not isinstance(getattr(payload, "session_mode", Unset()), Unset)


def _text_or_none(value: Any) -> str | None:
    if value is None or isinstance(value, Unset):
        return None
    return str(getattr(value, "value", value))


def _int_or_none(value: Any) -> int | None:
    if value is None or isinstance(value, Unset):
        return None
    return int(value)


def _uuid_or_none(text: str | None) -> UUID | None:
    if not text:
        return None
    try:
        return UUID(text)
    except ValueError:
        return None


def _utcnow() -> datetime:
    from datetime import UTC  # noqa: PLC0415 - keeps the module header short

    return datetime.now(UTC)


def _content(message: Message) -> str:
    return message.content_md if isinstance(message.content_md, str) else ""


def _has_media(message: Message) -> bool:
    """Whether a message carries a file Hermes takes as media (every type but ``audio/*``)."""
    return any(not str(a.file.mime_type).startswith("audio/") for a in message.attachments)


def _log_task_failure(task: asyncio.Task[Any]) -> None:
    if task.cancelled():
        return
    exc = task.exception()
    if exc is not None:
        _LOG.warning("handler task failed: %s", type(exc).__name__)


def _with_media(
    built: BuiltContext, media: tuple[InboundMedia, ...], notes: list[str]
) -> BuiltContext:
    """The person's text with the notes of undelivered attachments before it.

    A message whose only content is attachments gets Hermes's own wording for it (`NO_TEXT`),
    so it never reaches Hermes as an empty message.
    """
    text = built.user_text if built.user_text.strip() else ""
    if not text and media:
        text = NO_TEXT
    if not notes and text == built.user_text:
        return built
    joined = "\n\n".join([*notes, text] if text else notes)
    return BuiltContext(user_side=built.user_side, user_text=joined)
