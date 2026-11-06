# SPDX-License-Identifier: Apache-2.0
"""The seam between the adapter logic and Hermes (docs/spec/09-agents.md 17.2).

Two directions, both without a Hermes import:

* `Runtime` is what the logic asks of Hermes: start a turn, steer it, interrupt it, resolve a
  card's callback, switch or read the session of a thread, run a control command in a thread,
  name the skill commands and build a skill's turn text, tell which credential and which
  toolsets the next turn uses, transcribe audio, put an attachment into Hermes's media
  cache, and read or cancel the background work of a thread's session (09 17.12).
  ``platform.py`` implements it on the pinned Hermes version; unit tests fake it.
* `RuntimeEvents` is what Hermes tells the logic during a turn (status sentences, drafts, the
  answer, edits, cards, usage, session IDs, the start and end of every turn of a thread), and
  whether a tool may run in a thread now. ``platform.py`` forwards the Hermes callbacks to it on
  the adapter's event loop; `bothy_hermes_adapter.handler.HermesHandler` implements it.

Every type here keeps message contents and credential values out of ``repr`` (SEC-LOG-2).

Examples:
    >>> TurnOutcome("cancelled") is TurnOutcome.CANCELLED
    True
    >>> ResolvedCredential("anthropic", "oauth")
    ResolvedCredential(provider='anthropic', key_type='oauth')
"""

from __future__ import annotations

from collections.abc import Mapping, Sequence
from dataclasses import dataclass, field
from enum import StrEnum
from typing import Any, Protocol, runtime_checkable
from uuid import UUID

from bothy_hermes_adapter.context_format import UserSideContext

__all__ = [
    "BackgroundKind",
    "BackgroundTask",
    "CardKind",
    "CardRequest",
    "CommandRequest",
    "FileSent",
    "InboundMedia",
    "MediaKind",
    "OpenCard",
    "ResolvedCredential",
    "Runtime",
    "RuntimeEvents",
    "SessionSwitch",
    "Transcription",
    "TurnOutcome",
    "TurnRequest",
    "Usage",
]


class TurnOutcome(StrEnum):
    """How a Hermes turn ended (``ProcessingOutcome`` of the pinned version)."""

    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"


class SessionSwitch(StrEnum):
    """What `Runtime.switch_session` did (09 9.1, 9.5)."""

    NEW = "new"
    """A new Hermes session now serves the thread."""
    BOUND = "bound"
    """The thread now continues the named Hermes session."""
    MISSING = "missing"
    """There is no thread to switch (no turn of the thread was ever started here)."""


class CardKind(StrEnum):
    """The Hermes prompts that become cards (09 17.3)."""

    CLARIFY = "clarify"
    APPROVAL = "approval"
    SLASH_CONFIRM = "slash_confirm"


class MediaKind(StrEnum):
    """The kind of file Hermes sends with an answer (09 17.3 ``send_document`` and siblings)."""

    DOCUMENT = "document"
    IMAGE = "image"
    VOICE = "voice"
    VIDEO = "video"


class BackgroundKind(StrEnum):
    """What a unit of background work is, as the step kind that shows it (09 5.4, 17.12)."""

    SUBAGENT = "subagent"
    """An asynchronous delegation unit of ``delegate_task`` (one or more subagents)."""
    JOB = "background_job"
    """A background process of the ``terminal`` tool."""


@dataclass(frozen=True)
class BackgroundTask:
    """One unit of a thread's background work, as Hermes' registries report it (09 17.12).

    Attributes:
        task_id: Hermes' ID of the unit (stable while it is known).
        kind: Delegation or process.
        title: The delegation's goal or the process's command (shown as the step title).
        state: ``running``, ``completed``, ``failed`` or ``cancelled``.
        open: Whether the unit still holds its run: it runs, or Hermes has not admitted the turn
            that reports its result yet.
    """

    task_id: str
    kind: BackgroundKind
    title: str = field(repr=False)
    state: str
    open: bool


@dataclass(frozen=True)
class FileSent:
    """What became of a file Hermes sent (``send_document`` and siblings).

    Attributes:
        message_id: The message that carries the file, ``None`` when nothing was sent.
        error: Why nothing was sent (a code, never a path or a content), ``None`` on success.
    """

    message_id: str | None
    error: str | None = None


@dataclass(frozen=True)
class Transcription:
    """Hermes's transcription of one audio attachment (09 13, 17.6).

    Attributes:
        transcript: The text, ``None`` when Hermes could not transcribe the audio.
        note: The line Hermes puts into the person's text when there is no transcript (the
            same note it writes on its other platforms); empty with a transcript.
    """

    transcript: str | None = field(repr=False)
    note: str = field(default="", repr=False)


@dataclass(frozen=True)
class InboundMedia:
    """An attachment of the triggering message in Hermes's media cache (09 17.6).

    Attributes:
        path: The cached file, as Hermes's own platforms put it into ``media_urls``.
        media_type: Its MIME type (``media_types``); ``image/*`` reaches Hermes's vision.
    """

    path: str = field(repr=False)
    media_type: str


@dataclass(frozen=True)
class TurnRequest:
    """One Hermes turn for a run.

    Attributes:
        run_id: The run.
        thread_id: The thread (Hermes's ``thread_id`` of the source, 17.4).
        channel_id: The channel (Hermes's ``chat_id``).
        user_id: The ``sub`` of the run's token (the triggering person).
        user_name: The person's display name.
        message_id: The triggering message.
        user_text: The person's text plus the transcripts of the voice attachments.
        user_side: The format rules, channel instructions and run context for the user side of
            the turn (09 4.6, 4.11, 17.6).
        media: The attachments of the triggering message in Hermes's media cache.
        new_session: Whether the turn starts a new Hermes session of the thread (the channel's
            bound skills are loaded then, never again in the same session).
        context_taken: Whether an earlier turn of the run took the run context already (a
            follow-up's own turn while the run holds background work, 09 17.12): the user side
            then carries the format rules and channel instructions only.
    """

    run_id: UUID
    thread_id: UUID
    channel_id: UUID
    user_id: UUID
    user_name: str = field(repr=False)
    message_id: UUID
    user_text: str = field(repr=False)
    user_side: UserSideContext = field(repr=False)
    media: tuple[InboundMedia, ...] = ()
    new_session: bool = False
    context_taken: bool = False


@dataclass(frozen=True)
class CommandRequest:
    """A control command of a person, to run in a thread's Hermes session (09 10.4, 17.3).

    Attributes:
        thread_id: The thread.
        channel_id: The channel.
        user_id: The invoking person.
        user_name: The person's display name.
        text: The Hermes command line (``/<name>``).
    """

    thread_id: UUID
    channel_id: UUID
    user_id: UUID
    user_name: str = field(repr=False)
    text: str


@dataclass(frozen=True)
class ResolvedCredential:
    """The kind of credential Hermes resolves for the next turn; never its value (17.8).

    Attributes:
        provider: The provider name, ``None`` when unknown.
        key_type: ``api_key``, ``oauth`` or ``unknown``.
    """

    provider: str | None
    key_type: str


@dataclass(frozen=True)
class Usage:
    """Token counts of one model request of a turn, as Hermes's usage report states them.

    Attributes:
        provider: The provider.
        model: The model.
        tokens_in: Input tokens.
        tokens_out: Output tokens.
        tokens_cache_read: Cache reads.
        tokens_cache_write: Cache writes.
        currency: The currency Hermes states, ``None`` when it states none.
        amount_micros: Hermes's estimate in millionths of the currency, ``None`` without one.
    """

    provider: str
    model: str
    tokens_in: int = 0
    tokens_out: int = 0
    tokens_cache_read: int = 0
    tokens_cache_write: int = 0
    currency: str | None = None
    amount_micros: int | None = None


@dataclass(frozen=True)
class CardRequest:
    """A Hermes prompt that waits for a person (09 11, 17.3).

    Attributes:
        kind: Clarify, approval or slash confirmation.
        hermes_id: Hermes's ID of the prompt (``clarify_id``, ``confirm_id``) or, for an
            approval, an ID the adapter assigns.
        question: The question or the prompt text.
        choices: The answers of a clarify prompt (empty for an open question).
        session_key: Hermes's session key the resolver needs (approval, slash confirmation).
        title: The title of a slash confirmation.
    """

    kind: CardKind
    hermes_id: str
    question: str = field(repr=False)
    choices: Sequence[str] = field(default=(), repr=False)
    session_key: str = ""
    title: str = field(default="", repr=False)


@dataclass(frozen=True)
class OpenCard:
    """A card that waits for its answer, as the store keeps it.

    Attributes:
        run_id: The run.
        request: The Hermes prompt.
        message_id: The card message.
        component_ids: The card's component IDs by ``custom_id``.
    """

    run_id: UUID
    request: CardRequest
    message_id: UUID | None = None
    component_ids: Mapping[str, UUID] = field(default_factory=dict)


@runtime_checkable
class Runtime(Protocol):
    """What the adapter logic asks of Hermes; implemented by ``platform.py``."""

    async def start_turn(self, turn: TurnRequest) -> TurnOutcome:
        """Start a Hermes turn and wait until Hermes reports its end.

        Args:
            turn: The turn.

        Returns:
            How the turn ended.
        """
        ...

    async def steer(self, thread_id: UUID, text: str, message_id: UUID) -> None:
        """Pass a follow-up text into the running turn of a thread (busy input, 17.5).

        Args:
            thread_id: The thread.
            text: The follow-up text.
            message_id: The follow-up message.
        """
        ...

    async def interrupt(self, thread_id: UUID) -> None:
        """Interrupt the running turn of a thread (09 8, 17.3 ``/stop``).

        Args:
            thread_id: The thread.
        """
        ...

    async def resolve_callback(
        self, card: OpenCard, custom_id: str, values: Mapping[str, Any], operator: UUID
    ) -> str | None:
        """Resolve the Hermes callback of a card's component exactly once.

        Args:
            card: The open card.
            custom_id: The chosen component.
            values: The submitted values.
            operator: The person who chose.

        Returns:
            The chosen label, or ``None`` when Hermes had nothing waiting.
        """
        ...

    async def answer_card(self, card: OpenCard, text: str) -> str | None:
        """Answer an open card with a typed follow-up (09 11.6).

        A clarify is resolved with the text; an approval is denied and a slash confirmation
        cancelled, because a text is no approval.

        Args:
            card: The open card.
            text: The follow-up text.

        Returns:
            The answer given, or ``None`` when Hermes had nothing waiting.
        """
        ...

    async def switch_session(
        self, turn: TurnRequest, mode: str, external_session_ref: str | None
    ) -> SessionSwitch:
        """Give the turn's thread a new session (``new``) or bind it to a named one (``bind``).

        Args:
            turn: The turn about to start (its thread, channel and person).
            mode: ``new`` or ``bind``.
            external_session_ref: The session to bind (``bind`` only).

        Returns:
            What happened.
        """
        ...

    def current_session(self, thread_id: UUID) -> str | None:
        """Return Hermes's session ID of the thread as last reported by Hermes.

        Args:
            thread_id: The thread.

        Returns:
            The session ID, or ``None`` before Hermes reported one.
        """
        ...

    def session_parent(self, thread_id: UUID) -> str | None:
        """Return the parent of the thread's current session (a compression chain), if any.

        Args:
            thread_id: The thread.

        Returns:
            The parent session ID, or ``None``.
        """
        ...

    def session_title(self, thread_id: UUID) -> str | None:
        """Return the title Hermes gave the thread's session, if any.

        Args:
            thread_id: The thread.

        Returns:
            The title, or ``None``.
        """
        ...

    async def run_command(self, command: CommandRequest) -> list[str]:
        """Run a control command in the thread's Hermes session and collect Hermes' reply.

        Args:
            command: The command.

        Returns:
            The texts Hermes answered with, in order (empty when it answered nothing).
        """
        ...

    def skill_commands(self) -> Mapping[str, str]:
        """Return Hermes' skill commands for this platform: name (without ``/``) to description.

        Returns:
            The skill commands.
        """
        ...

    def skill_text(self, name: str, text: str) -> str | None:
        """Build Hermes' own turn text that invokes a skill with a person's text.

        Args:
            name: The skill command (without ``/``).
            text: What the skill should work on (may be empty).

        Returns:
            The turn text, or ``None`` when Hermes has no such skill now.
        """
        ...

    def resolved_credential(self) -> ResolvedCredential:
        """Return the kind of credential Hermes resolves for the next turn (17.8).

        Returns:
            Provider and key type, never the value.
        """
        ...

    def loaded_toolsets(self) -> frozenset[str]:
        """Return the toolsets Hermes enables for this platform.

        Returns:
            The toolset names.
        """
        ...

    def voice_input(self) -> bool:
        """Return whether Hermes can transcribe audio now (speech to text on, a provider found).

        Returns:
            Whether the adapter may report ``voice_input`` (09 2.2, 13).
        """
        ...

    async def transcribe(self, data: bytes, name: str, mime_type: str) -> Transcription:
        """Transcribe one audio attachment with Hermes's own speech to text (09 13, 17.6).

        Args:
            data: The audio bytes.
            name: The attachment's original name (for the file extension).
            mime_type: The attachment's MIME type.

        Returns:
            The transcript, or Hermes's note when there is none.
        """
        ...

    def media_limit(self) -> int:
        """Return Hermes's limit for an incoming attachment in bytes (``0``: none).

        Returns:
            ``gateway.max_inbound_media_bytes`` of the profile, as its other platforms read it.
        """
        ...

    async def cache_media(self, data: bytes, name: str, mime_type: str) -> InboundMedia | None:
        """Put one attachment into Hermes's media cache, as its other platforms do (09 17.6).

        Args:
            data: The file's bytes.
            name: The attachment's original name.
            mime_type: The attachment's MIME type.

        Returns:
            The cached file, or ``None`` when Hermes refuses the bytes (an image that is none).
        """
        ...

    async def background_work(self, thread_id: UUID, since: float) -> Sequence[BackgroundTask]:
        """Read the background work of the thread's Hermes session from Hermes' registries.

        Args:
            thread_id: The thread.
            since: Wall time in seconds since the epoch: only work that started at or after it
                counts (the start of the run, 09 17.12).

        Returns:
            Every such unit Hermes still knows, open or ended.
        """
        ...

    async def cancel_background(self, thread_id: UUID) -> None:
        """Interrupt the thread session's delegations and kill its running processes.

        Args:
            thread_id: The thread.
        """
        ...

    def turn_running(self, thread_id: UUID) -> bool | None:
        """Whether a Hermes turn of the thread's session runs now (Hermes' busy state).

        It ends on every way a turn ends (completed, failed, interrupted, raised), also where
        Hermes calls no ``on_session_end`` (09 17.12 point 3).

        Args:
            thread_id: The thread.

        Returns:
            True while a turn runs, False when none does, ``None`` when Hermes cannot tell.
        """
        ...


class RuntimeEvents(Protocol):
    """What Hermes reports during a turn; ``platform.py`` calls it on the adapter's loop."""

    def on_status(self, thread_id: UUID, text: str | None) -> None:
        """A status sentence of the running turn, ``None`` when it is cleared."""
        ...

    def on_tool_event(self, thread_id: UUID, preview: str, call_id: str | None, done: bool) -> None:
        """A tool start or end from ``format_tool_event``."""
        ...

    def on_draft(self, thread_id: UUID, text: str) -> None:
        """The draft text so far."""
        ...

    async def on_answer(self, thread_id: UUID, text: str, reply_to: str | None) -> str | None:
        """An answer message; returns the ID Hermes may edit later, ``None`` on failure."""
        ...

    async def on_edit(self, thread_id: UUID, message_id: str, text: str, finalize: bool) -> bool:
        """An edit of an own answer; returns whether it was taken."""
        ...

    async def on_card(self, thread_id: UUID, request: CardRequest) -> bool:
        """A Hermes prompt to show as a card; returns whether it was shown."""
        ...

    async def on_file(
        self, thread_id: UUID, path: str, kind: MediaKind, caption: str | None, name: str | None
    ) -> FileSent:
        """A file Hermes sends with its answer: upload it and send it as an attachment."""
        ...

    def on_usage(self, thread_id: UUID, usage: Usage) -> None:
        """The token counts of one model request of the running turn."""
        ...

    def on_model_reply(self, thread_id: UUID, tool_calls: int | None) -> None:
        """One model reply of the running turn with its number of tool calls (``None``: unknown)."""
        ...
