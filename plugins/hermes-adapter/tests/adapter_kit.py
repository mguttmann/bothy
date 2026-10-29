# SPDX-License-Identifier: Apache-2.0
"""Test kit of the adapter's unit tests: a fake Hermes runtime and the SDK loop on a FakeServer.

`FakeRuntime` implements `bothy_hermes_adapter.runtime.Runtime` without Hermes: each turn runs
a script that reports to the handler what Hermes would (status sentences, drafts, the answer,
cards, usage) and ends with an outcome. `Rig` starts the FakeServer of the SDK, the handler and
the adapter loop with the Hermes heartbeat, all on the FakeServer's clock.
"""

from __future__ import annotations

import asyncio
import contextlib
import logging
from collections.abc import AsyncIterator, Awaitable, Callable, Mapping
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any
from uuid import UUID

from bothy_sdk.adapter import AdapterConfig
from bothy_sdk.sources import SourceReady
from bothy_sdk.testing._patience import eventually, within_patience
from bothy_sdk.testing.doubles import RecordingSource, Rewrite, TamperingSource
from bothy_sdk.testing.fakeserver import FakeServer

from bothy_hermes_adapter import cards
from bothy_hermes_adapter.config import AdapterSettings
from bothy_hermes_adapter.handler import HermesHandler
from bothy_hermes_adapter.heartbeat import HermesAdapterLoop
from bothy_hermes_adapter.runtime import (
    CardRequest,
    CommandRequest,
    OpenCard,
    ResolvedCredential,
    SessionSwitch,
    Transcription,
    TurnOutcome,
    TurnRequest,
    Usage,
)
from bothy_hermes_adapter.store import CallbackStore

Script = Callable[[TurnRequest, "FakeRuntime"], Awaitable[TurnOutcome]]


async def answer_script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
    """The default turn: one status step, a draft, the answer and the usage."""
    events = rt.events
    events.on_status(turn.thread_id, "Running a tool")
    events.on_status(turn.thread_id, None)
    events.on_draft(turn.thread_id, "Done so")
    events.on_usage(turn.thread_id, Usage("anthropic", "model-small", 11, 7))
    await events.on_answer(turn.thread_id, "Done so far.", str(turn.message_id))
    return TurnOutcome.COMPLETED


@dataclass
class FakeRuntime:
    """A Hermes stand-in; every call is recorded."""

    credential: ResolvedCredential | Exception = field(
        default_factory=lambda: ResolvedCredential("anthropic", "api_key")
    )
    toolsets: frozenset[str] = frozenset({"todo"})
    script: Script = answer_script
    turns: list[TurnRequest] = field(default_factory=list)
    steered: list[tuple[UUID, str, UUID]] = field(default_factory=list)
    interrupted: list[UUID] = field(default_factory=list)
    resolved: list[tuple[str, str]] = field(default_factory=list)
    answered: list[str] = field(default_factory=list)
    switches: list[tuple[UUID, str, str | None]] = field(default_factory=list)
    sessions: dict[UUID, str] = field(default_factory=dict)
    parents: dict[UUID, str] = field(default_factory=dict)
    known: set[str] = field(default_factory=set)
    titles: dict[UUID, str] = field(default_factory=dict)
    answers: asyncio.Queue[str] = field(default_factory=asyncio.Queue)
    stopped: dict[UUID, asyncio.Event] = field(default_factory=dict)
    started: asyncio.Event = field(default_factory=asyncio.Event)
    late: Any = None
    stt: bool = True
    transcripts: Mapping[bytes, str] = field(default_factory=dict)
    heard: list[tuple[bytes, str, str]] = field(default_factory=list)
    skills: dict[str, str] = field(default_factory=dict)
    commands: list[CommandRequest] = field(default_factory=list)
    replies: list[str] = field(default_factory=lambda: ["Hermes Agent v0.0.0"])
    command_error: Exception | None = None
    _events: HermesHandler | None = None
    _scripts: set[asyncio.Task[TurnOutcome]] = field(default_factory=set)

    @property
    def events(self) -> HermesHandler:
        assert self._events is not None
        return self._events

    def stop_event(self, thread_id: UUID) -> asyncio.Event:
        return self.stopped.setdefault(thread_id, asyncio.Event())

    async def start_turn(self, turn: TurnRequest) -> TurnOutcome:
        self.turns.append(turn)
        self.started.set()
        if turn.thread_id not in self.sessions:
            self.sessions[turn.thread_id] = f"sess-{len(self.sessions) + 1}"
        # Hermes runs the turn on its own: a cancelled caller does not stop the script.
        task = asyncio.ensure_future(self.script(turn, self))
        self._scripts.add(task)
        return await asyncio.shield(task)

    async def steer(self, thread_id: UUID, text: str, message_id: UUID) -> None:
        self.steered.append((thread_id, text, message_id))

    async def interrupt(self, thread_id: UUID) -> None:
        self.interrupted.append(thread_id)
        self.stop_event(thread_id).set()

    async def resolve_callback(
        self, card: OpenCard, custom_id: str, values: Mapping[str, Any], operator: UUID
    ) -> str | None:
        label = cards.chosen_label(card, custom_id, values)
        self.resolved.append((custom_id, str(label)))
        if label is not None:
            self.answers.put_nowait(label)
        return label

    async def answer_card(self, card: OpenCard, text: str) -> str | None:
        answer = (
            text
            if card.request.kind.value == "clarify"
            else ("deny" if card.request.kind.value == "approval" else "cancel")
        )
        self.answered.append(answer)
        self.answers.put_nowait(answer)
        return answer

    async def switch_session(
        self, turn: TurnRequest, mode: str, external_session_ref: str | None
    ) -> SessionSwitch:
        thread_id = turn.thread_id
        self.switches.append((thread_id, mode, external_session_ref))
        if mode == "bind" and external_session_ref in self.known:
            self.sessions[thread_id] = str(external_session_ref)
            return SessionSwitch.BOUND
        self.sessions[thread_id] = f"sess-new-{len(self.switches)}"
        return SessionSwitch.NEW if mode == "new" else SessionSwitch.BOUND

    async def run_command(self, command: CommandRequest) -> list[str]:
        self.commands.append(command)
        if self.command_error is not None:
            raise self.command_error
        return list(self.replies)

    def skill_commands(self) -> Mapping[str, str]:
        return dict(self.skills)

    def skill_text(self, name: str, text: str) -> str | None:
        if name not in self.skills:
            return None
        return f"[skill {name}] {text}".rstrip()

    def current_session(self, thread_id: UUID) -> str | None:
        return self.sessions.get(thread_id)

    def session_parent(self, thread_id: UUID) -> str | None:
        return self.parents.get(thread_id)

    def session_title(self, thread_id: UUID) -> str | None:
        return self.titles.get(thread_id)

    def resolved_credential(self) -> ResolvedCredential:
        if isinstance(self.credential, Exception):
            raise self.credential
        return self.credential

    def loaded_toolsets(self) -> frozenset[str]:
        return self.toolsets

    def voice_input(self) -> bool:
        return self.stt

    async def transcribe(self, data: bytes, name: str, mime_type: str) -> Transcription:
        self.heard.append((data, name, mime_type))
        text = self.transcripts.get(data)
        if text is None:
            return Transcription(None, "[voice message could not be transcribed automatically]")
        return Transcription(text)


@dataclass
class Rig:
    """The FakeServer, the fake runtime, the handler and the adapter loop."""

    fake: FakeServer
    runtime: FakeRuntime
    handler: HermesHandler
    loop: HermesAdapterLoop
    source: RecordingSource
    store: CallbackStore

    async def until(self, predicate: Callable[[], Any], what: str = "the condition") -> None:
        await eventually(predicate, what, step=lambda: self.fake.pass_time(0.05))

    def requests(self, operation_id: str) -> list[Any]:
        return [r for r in self.fake.requests if r.operation_id == operation_id]

    def finished(self, run_id: UUID) -> bool:
        return self.fake.run_state(run_id).state in ("completed", "failed", "cancelled")


def settings_for(fake: FakeServer, root: Path, **over: Any) -> AdapterSettings:
    token = root / "token"
    if not token.exists():
        token.write_text(fake.credentials.api_token.reveal(), encoding="utf-8")
        token.chmod(0o600)
    values: dict[str, Any] = {
        "platform_key": "bothy",
        "consumer": over.pop("consumer", "unit"),
        "server_url": fake.base_url,
        "instance_id": fake.instance_id,
        "token_file": token,
        "sdk": fake.sdk_settings,
    }
    values.update(over)
    return AdapterSettings(**values)


@contextlib.asynccontextmanager
async def rig(
    root: Path,
    *,
    runtime: FakeRuntime | None = None,
    busy_mode: str = "steer",
    fake: FakeServer | None = None,
    billing_mode: str = "organization",
    tool_class: str = "scoped",
    tamper: Rewrite | None = None,
    commands: Any = None,
    **settings: Any,
) -> AsyncIterator[Rig]:
    """Start everything; stop and close on exit.

    ``tamper`` rewrites the tokens of chosen deliveries before the adapter sees them (AG-08).
    """
    own = fake is None
    server = fake or FakeServer(billing_mode=billing_mode, tool_class=tool_class)
    if own:
        await server.start()
    rt = runtime or FakeRuntime()
    state = root / "state"
    state.mkdir(mode=0o700, parents=True, exist_ok=True)
    state.parent.chmod(0o700)
    adapter_settings = settings_for(server, root, **settings)
    store = CallbackStore(state / "hermes.sqlite3")
    handler = HermesHandler(rt, adapter_settings, store, busy_mode=busy_mode)
    rt._events = handler
    config = AdapterConfig(
        base_url=server.base_url,
        instance_id=server.instance_id,
        agent_id=server.agent_id,
        credentials=server.credentials,
        consumer=adapter_settings.consumer,
        state_dir=state,
        allowlist=True,
        adapter_name="hermes-adapter",
        adapter_version="0.0.0",
        runtime_name="hermes-agent",
        runtime_version="0.21.5",
        capabilities=handler.capabilities(),
        settings=server.sdk_settings,
        commands=commands,
    )
    loop = HermesAdapterLoop(config, handler, clock=server.clock)
    inner = await loop.make_source()
    source = RecordingSource(inner if tamper is None else TamperingSource(inner, tamper))
    loop.source = source
    task = asyncio.ensure_future(loop.run())
    the_rig = Rig(server, rt, handler, loop, source, store)
    try:
        await the_rig.until(lambda: any(isinstance(d, SourceReady) for d in source.deliveries))
        yield the_rig
    finally:
        await loop.stop()
        with contextlib.suppress(Exception):
            await within_patience(task)
        with contextlib.suppress(Exception):
            await store.close()
        if own:
            await server.stop()


class Captured(logging.Handler):
    """Collects the records of the adapter's and the SDK's loggers."""

    def __init__(self) -> None:
        super().__init__(logging.DEBUG)
        self.records: list[logging.LogRecord] = []

    def emit(self, record: logging.LogRecord) -> None:
        self.records.append(record)

    def text(self) -> str:
        parts = []
        for record in self.records:
            parts.append(record.getMessage())
            if record.exc_info and record.exc_info[1] is not None:
                parts.append(repr(record.exc_info[1]))
        return "\n".join(parts)


@contextlib.contextmanager
def captured_logs() -> Any:
    handler = Captured()
    loggers = [logging.getLogger("bothy_hermes_adapter"), logging.getLogger("bothy_sdk")]
    old = [(lg, lg.level) for lg in loggers]
    for lg in loggers:
        lg.addHandler(handler)
        lg.setLevel(logging.DEBUG)
    try:
        yield handler
    finally:
        for lg, level in old:
            lg.removeHandler(handler)
            lg.setLevel(level)


def card_script(request: CardRequest) -> Script:
    """A turn that shows one card, waits for its answer and then answers."""

    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        shown = await rt.events.on_card(turn.thread_id, request)
        assert shown
        stop = rt.stop_event(turn.thread_id)
        answer = asyncio.ensure_future(rt.answers.get())
        stopped = asyncio.ensure_future(stop.wait())
        done, pending = await asyncio.wait({answer, stopped}, return_when=asyncio.FIRST_COMPLETED)
        for task in pending:
            task.cancel()
        if stopped in done:
            return TurnOutcome.CANCELLED
        await rt.events.on_answer(turn.thread_id, f"You chose {answer.result()}.", None)
        return TurnOutcome.COMPLETED

    return script
