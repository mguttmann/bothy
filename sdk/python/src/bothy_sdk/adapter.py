# SPDX-License-Identifier: Apache-2.0
"""The acceptance and run loop of an agent adapter (docs/spec/09-agents.md 4.3, 4.5, 4.9, 15).

`Adapter` connects as a consumer (gateway or REST polling), checks every invocation token
itself, stores an accepted event with its token in the durable inbox and only then acknowledges
it; a run starts later from the inbox, at most once and with the newest token, renewing an
expired one (R-AG-24, R-AG-25). The runtime (the handler's ``execute``) starts only after ``200``
on ``startRun`` (R-AG-07); nothing is retried after the runtime died (R-AG-17). Every request of
a run's handler is bound to the run's token (07 SR9).

Refusals (docs/DECISIONS.md D-M2-29-8): a token that fails the check on an event that would
start or resume a run ends the run with ``finishRun`` (``failed`` and ``invalid_invocation_token``,
``billing_mode_mismatch`` or ``tool_class_mismatch``); on any other event it is logged and
skipped and the run is not ended, because a forged event must not end a running run. An unknown
key leaves the event unsettled (the server delivers again with a current key); a replayed ``jti``
under a new event ID is ignored and not settled. The handler never sees a refused event.

Allow list of 17.7 (docs/DECISIONS.md D-M2-33-3): when ``getAgentTriggerers`` answers ``404``
the server has no such operation, so the event is refused like a failed point 9 with
``adapter_error`` (a start event ends its run) and settled; a network error, ``429`` or ``5xx``
leaves the event unsettled without an inbox entry, so the redelivery tries again.

Examples:
    >>> AgentHandler.prepare.__qualname__
    'AgentHandler.prepare'
"""

from __future__ import annotations

import asyncio
import contextlib
import logging
from collections.abc import Coroutine, Mapping, Sequence
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any
from uuid import UUID

import httpx

from bothy_sdk.auth import Credentials, Secret
from bothy_sdk.client import Client
from bothy_sdk.clock import Clock, SystemClock
from bothy_sdk.commands import (
    AutocompleteRequest,
    CommandSpec,
    guarded_autocomplete,
    reconcile_commands,
)
from bothy_sdk.errors import (
    AgentInvocationDenied,
    AlreadyStarted,
    BillingModeMismatch,
    InvocationTokenInvalid,
    ProblemError,
    ToolClassMismatch,
)
from bothy_sdk.gateway import CloseReason, Gateway, GatewayEvent, gateway_url
from bothy_sdk.generated.api.agent_runtime import (
    finish_run,
    get_agent_triggerers,
    renew_invocation_token,
    resume_run,
    send_heartbeat,
    start_run,
)
from bothy_sdk.generated.api.runs import get_run
from bothy_sdk.generated.models import (
    AgentTriggerers,
    BillingEvidence,
    BillingMode,
    CommandChoice,
    Heartbeat,
    InvocationTokenGrant,
    InvocationTokenRenew,
    ProjectReadiness,
    Run,
    RunFinish,
    RunFinishState,
    RunStart,
    ToolClass,
)
from bothy_sdk.heartbeat import build_heartbeat
from bothy_sdk.inbox import EntryState, InboxEntry, InboxStore, MemoryInboxStore, SqliteInboxStore
from bothy_sdk.invocation import (
    TOKEN_EVENT_TYPES,
    Actual,
    InvocationClaims,
    InvocationVerifier,
    TokenVerdict,
    compare_actual,
    expectation_for,
    stored_claims,
)
from bothy_sdk.outbox import Outbox, OutboxStore, SqliteOutboxStore
from bothy_sdk.runs import RunHandle
from bothy_sdk.settings import SdkSettings
from bothy_sdk.sources import (
    CursorAdvance,
    Envelope,
    EventSource,
    GatewaySource,
    PollingSource,
    SourceReady,
)
from bothy_sdk.tolerance import UnknownEvent

__all__ = [
    "ActualBilling",
    "Adapter",
    "AdapterConfig",
    "AgentHandler",
    "AllowlistUnavailable",
    "Invocation",
    "RunRefused",
    "RuntimeFailure",
    "RuntimeStatus",
    "StartOptions",
]

_LOG = logging.getLogger("bothy_sdk")

_START_EVENT = "agent.invocation"
_FOLLOWUP = "agent.followup"
_INTERACTION = "interaction.created"
_COMMAND = "command.invoked"
_AUTOCOMPLETE = "command.autocomplete_requested"
_CANCEL = "run.cancel_requested"
_STATE_CHANGED = "run.state_changed"
_ACCESS_CHANGED = "access.changed"
_REFUSAL_REASON = {
    TokenVerdict.INVALID: "invalid_invocation_token",
    TokenVerdict.BILLING_MODE_MISMATCH: "billing_mode_mismatch",
    TokenVerdict.TOOL_CLASS_MISMATCH: "tool_class_mismatch",
}


@dataclass(frozen=True)
class AdapterConfig:
    """Configuration of an agent adapter.

    Attributes:
        base_url: The API base, for example ``https://example.com/api/v1``.
        instance_id: The configured instance (``iss`` of every token, 09 4.5 point 4).
        agent_id: The agent's principal ID (``aud``).
        credentials: The agent's API token and optional DPoP key.
        consumer: The consumer name.
        transport: ``gateway`` or ``polling``.
        state_dir: The directory of ``inbox.sqlite3`` and ``outbox.sqlite3``; ``None`` keeps
            both in memory (tests only, not durable).
        allowlist: True to keep the allow list of 17.7 (``getAgentTriggerers``).
        adapter_name: The adapter's name (heartbeat, ``User-Agent``).
        adapter_version: The adapter's version.
        runtime_name: The runtime's name (heartbeat).
        runtime_version: The runtime's version, if known.
        capabilities: The heartbeat ``capabilities`` (09 4.8).
        commands: Declared commands; ``None`` leaves commands alone.
        settings: The SDK settings.
    """

    base_url: str
    instance_id: UUID
    agent_id: UUID
    credentials: Credentials
    consumer: str
    transport: str = "gateway"
    state_dir: Path | None = None
    allowlist: bool = False
    adapter_name: str = "bothy-adapter"
    adapter_version: str = "0.0.0"
    runtime_name: str = "runtime"
    runtime_version: str | None = None
    capabilities: Mapping[str, Any] = field(default_factory=dict)
    commands: Sequence[CommandSpec] | None = None
    settings: SdkSettings = field(default_factory=SdkSettings)


@dataclass(frozen=True)
class ActualBilling:
    """The billing mode the runtime actually uses for a run (I3, 10 7).

    Attributes:
        mode: ``organization``, ``personal`` or ``unknown`` (``unknown`` never matches).
        evidence: The evidence sent with ``startRun``, if any.
    """

    mode: str
    evidence: BillingEvidence | None = None


@dataclass(frozen=True)
class StartOptions:
    """What ``prepare`` adds to ``startRun``.

    Attributes:
        model: The model the runtime will use.
        external_session_ref: The runtime's session reference, if it exists before the run.
        billing_evidence: Evidence of the billing mode (overrides `ActualBilling.evidence`).
    """

    model: str | None = None
    external_session_ref: str | None = None
    billing_evidence: BillingEvidence | None = None


@dataclass(frozen=True)
class Invocation:
    """One accepted event as a handler sees it; never carries the token.

    Attributes:
        event_id: The envelope ``id``.
        seq: The envelope ``seq`` (``None`` for an ephemeral event).
        type: The event type.
        payload: The payload decoded with the generated core.
        claims: The verified claims of the event's token.
    """

    event_id: UUID
    seq: int | None
    type: str
    payload: Any
    claims: InvocationClaims


class AllowlistUnavailable(Exception):  # noqa: N818 - names the state, like RuntimeFailure
    """``getAgentTriggerers`` answered ``404``: the server offers no allow list (17.7)."""


class RuntimeFailure(Exception):  # noqa: N818 - the name of the brief
    """Raised by a handler when the runtime failed.

    The run ends ``failed`` with ``runtime_error``.
    """


class RunRefused(Exception):  # noqa: N818 - a refusal, not an error of the SDK
    """Raised by ``prepare`` to end a run without a start (09 4.9).

    With a reason the run ends ``failed`` with exactly that reason (``finishRun``), for example
    ``runtime_not_authenticated`` or ``billing_mode_mismatch``; without one the run is no longer
    active on the server (``404`` or ``run_inactive`` while loading its context) and only the
    inbox entry is closed, without ``finishRun``. Either way the runtime never starts and the
    entry is never started again.

    Args:
        reason: The reason code of ``finishRun``, or ``None`` for no ``finishRun``.

    Examples:
        >>> RunRefused("runtime_not_authenticated").reason
        'runtime_not_authenticated'
    """

    def __init__(self, reason: str | None) -> None:
        """Create the refusal; see the class docstring."""
        super().__init__(reason or "run no longer active")
        self.reason = reason


@dataclass(frozen=True)
class RuntimeStatus:
    """Heartbeat members a handler knows only at run time (09 4.8, M2-34).

    The adapter asks `AgentHandler.runtime_status` before every heartbeat and merges the
    answer with its own members: the degraded reasons of both count, and the heartbeat is
    ``degraded`` when either says so.

    Attributes:
        state: ``online`` or ``degraded``.
        degraded_reasons: Codes of ``details.degraded_reasons``.
        runtime_version: The runtime's version, replacing ``AdapterConfig.runtime_version``.
        billing_modes_supported: Modes the adapter can prove; ``None`` leaves the member out.
        project_readiness: Runners only: one entry per bound forum; ``None`` leaves it out.
        active_runs: Runs the handler runs outside the adapter's own run loop, for example
            turns a runner adopted after a restart (M2-35); added to ``active_runs``.
    """

    state: str = "online"
    degraded_reasons: Sequence[str] = ()
    runtime_version: str | None = None
    billing_modes_supported: Sequence[str] | None = None
    project_readiness: Sequence[ProjectReadiness] | None = None
    active_runs: Sequence[UUID] = ()


class AgentHandler:
    """The runtime side of an adapter; subclass it and override `prepare` and `execute`.

    Every method that belongs to a run runs inside the run's SR9 binding: each request of the
    adapter's client carries the run's newest invocation token.
    """

    def actual_billing(self, inv: Invocation) -> ActualBilling:
        """Return the billing mode the runtime uses for this run (``unknown`` when unsure).

        Args:
            inv: The invocation.

        Returns:
            The actual mode; the default ``unknown`` refuses every run (fail closed).
        """
        return ActualBilling("unknown")

    def actual_tool_class(self) -> ToolClass:
        """Return the tool class the runtime actually loaded (07 2.11).

        Returns:
            The tool class; the default is ``chat``.
        """
        return ToolClass.CHAT

    async def prepare(self, inv: Invocation, run: RunHandle) -> StartOptions:
        """Get ready for a run before it starts (load context, pick a model); no runtime yet.

        Args:
            inv: The invocation.
            run: The run (not started: no patches yet).

        Returns:
            Options of ``startRun``.
        """
        raise NotImplementedError

    async def execute(self, inv: Invocation, run: RunHandle) -> None:
        """Run the runtime; called only after ``200`` on ``startRun``.

        Returning ends the run ``completed`` unless the handler finished it or waits for input;
        `RuntimeFailure` ends it ``failed`` with ``runtime_error``, any other exception with
        ``adapter_error``.

        Args:
            inv: The invocation.
            run: The started run.
        """
        raise NotImplementedError

    async def on_not_started(self, inv: Invocation) -> None:
        """Free what ``prepare`` took when the run does not reach ``execute`` here.

        Called once after every ``prepare`` that did not lead to ``execute``: ``prepare`` raised
        (also `RunRefused` or a cancellation), or the start was refused, superseded or failed.
        A later start attempt of the same run calls ``prepare`` again.

        Args:
            inv: The invocation.
        """

    def runtime_status(self) -> RuntimeStatus | None:
        """Return the heartbeat members that change at run time, or ``None`` for none.

        Called before every heartbeat; it must not block (no I/O).

        Returns:
            The status; the default ``None`` keeps the members of `AdapterConfig`.
        """
        return None

    async def on_ready(self) -> None:
        """Called after every ready source (gateway ``ready`` or the first poll).

        A handler that wants its state reported at once calls `Adapter.send_heartbeat` here.
        """

    async def on_followup(self, inv: Invocation, run: RunHandle) -> None:
        """Handle a follow-up message of a running, resumed or not yet started run.

        A run this process holds but has not started yet (``run.token_jti`` is ``None``, for
        example while ``prepare`` waits for a lock) gets its follow-ups here too, so a handler can
        keep them and hand them to the runtime once it starts (D-M2-35-3).

        Args:
            inv: The ``agent.followup`` invocation.
            run: The run.
        """

    async def on_cancel(self, run: RunHandle, reason: str) -> None:
        """Stop the runtime of a started run after ``run.cancel_requested`` and finish it.

        Optional (11 11.2 module ``runs``, "cancel callback"). A handler that overrides it keeps
        its ``execute`` running: the adapter calls this hook instead of cancelling the task, and
        sends no ``finishRun`` of its own; the handler stops its runtime and calls
        ``run.finish("cancelled", reason=reason)`` once nothing of it is left (09 8 point 3).
        Without an override the adapter cancels ``execute`` and finishes the run itself.

        Args:
            run: The started run.
            reason: The request's reason exactly as received (also a value the SDK keeps raw).
        """
        raise NotImplementedError

    async def on_interaction(self, inv: Invocation, run: RunHandle) -> None:
        """Handle an interaction with a component of a running or resumed run.

        Args:
            inv: The ``interaction.created`` invocation.
            run: The run.
        """

    async def on_command(self, inv: Invocation, run: RunHandle | None) -> None:
        """Handle a command invocation (``command.invoked``).

        Args:
            inv: The invocation.
            run: The adapter's run the command names, if it is running here.
        """

    async def on_autocomplete(self, request: AutocompleteRequest) -> Sequence[CommandChoice]:
        """Return choices for an autocomplete request before it expires.

        Never call a model here (R-AG-14): typing must cost nothing and never hang.

        Args:
            request: The request.

        Returns:
            At most ``command.autocomplete_max_choices`` choices.
        """
        return ()

    async def on_event(self, envelope: Envelope) -> None:
        """Handle an event without an invocation token (for example ``agent.config_changed``).

        Args:
            envelope: The event.
        """


@dataclass
class _RunState:
    """What the adapter keeps in memory about one run of this process."""

    run_id: UUID
    event_id: UUID
    handle: RunHandle
    invocation: Invocation
    task: asyncio.Task[None] | None = None
    renewing: asyncio.Lock = field(default_factory=asyncio.Lock)
    waiting_for_token: bool = False
    cancel_reason: str | None = None
    resuming: asyncio.Lock = field(default_factory=asyncio.Lock)


class Adapter:
    """Connects an `AgentHandler` to the server.

    Args:
        config: The adapter configuration.
        handler: The runtime side.
        clock: The time source (tests pass a ``FakeClock``).
        source: An event source; default from ``config.transport``.
        inbox: The inbox; default ``inbox.sqlite3`` in ``config.state_dir`` (memory without one).
        outbox_store: The outbox store; default ``outbox.sqlite3`` in ``config.state_dir``.
        http_transport: An ``httpx`` transport for the REST client (tests).
    """

    def __init__(  # noqa: PLR0913 - the binding signature of Design note 2 plus a test seam
        self,
        config: AdapterConfig,
        handler: AgentHandler,
        *,
        clock: Clock | None = None,
        source: EventSource | None = None,
        inbox: InboxStore | None = None,
        outbox_store: OutboxStore | None = None,
        http_transport: httpx.AsyncBaseTransport | None = None,
    ) -> None:
        """Create the adapter; nothing connects before `run`."""
        self.config = config
        self.handler = handler
        self.clock = clock or SystemClock()
        self.settings = config.settings
        self.client = Client(
            config.base_url,
            config.credentials,
            settings=self.settings,
            clock=self.clock,
            user_agent=f"{config.adapter_name}/{config.adapter_version}",
            transport=http_transport,
        )
        state = config.state_dir
        self.inbox: InboxStore = inbox or (
            SqliteInboxStore(state / "inbox.sqlite3") if state is not None else MemoryInboxStore()
        )
        store = outbox_store or (
            SqliteOutboxStore(state / "outbox.sqlite3") if state is not None else None
        )
        self.outbox = Outbox(self.client, store, token_for=self._token_for_run)
        self._source = source
        self.source: EventSource | None = source
        self.verifier: InvocationVerifier | None = None
        self._runs: dict[UUID, _RunState] = {}
        # The newest token per run of this process, kept after the end, so a late outbox entry of
        # a run goes out bound to it and the server answers run_inactive (07 SR9).
        self._run_tokens: dict[UUID, Secret] = {}
        self._tasks: set[asyncio.Task[Any]] = set()
        self._stopping = False
        self._degraded: list[str] = []

    # ---- lifecycle -----------------------------------------------------------------------

    async def run(self) -> CloseReason:
        """Run until the source stops or `stop` is called.

        Returns:
            Why the adapter stopped (for example ``4015`` when another process took the
            consumer over, or ``1000`` after `stop`).

        Raises:
            ProblemError: The discovery or the first JWKS fetch failed.
        """
        discovery = await self.client.discover()
        if discovery.instance_id != self.config.instance_id:
            _LOG.error("the server is another instance than configured; stopping")
            await self._close()
            return CloseReason(1000, "instance_mismatch")
        self.verifier = InvocationVerifier(
            instance_id=self.config.instance_id,
            agent_id=self.config.agent_id,
            jwks=self._fetcher(discovery.jwks_uri),
            replay=self.inbox,
            settings=self.settings,
            clock=self.clock,
            allowlist=self._load_triggerers if self.config.allowlist else None,
        )
        await self.verifier.refresh()
        if self.source is None:
            self.source = self._default_source(discovery.gateway.url)
        await self._recover()
        await self.outbox.flush()
        try:
            async for delivery in self.source:
                if isinstance(delivery, SourceReady):
                    self._spawn(self._on_ready())
                elif isinstance(delivery, CursorAdvance):
                    await self.source.advance(delivery.seq)
                else:
                    await self._on_envelope(delivery)
                if self._stopping:
                    break
        finally:
            await self._close()
        return self.source.close_reason or CloseReason(1000, "stopped")

    async def stop(self) -> None:
        """Stop the source; `run` returns after the current delivery."""
        self._stopping = True
        if self.source is not None:
            await self.source.close()

    async def make_source(self) -> EventSource:
        """Build the source ``config.transport`` names, for a test to wrap before `run`.

        Example: ``adapter.source = TamperingSource(await adapter.make_source(), rewrite)``.

        Returns:
            The gateway or polling source of this adapter.
        """
        discovery = await self.client.discover()
        return self._default_source(discovery.gateway.url)

    async def _token_for_run(self, run_id: UUID) -> Secret | None:
        """The newest token of a run for its outbox entries: this process's, else the inbox's."""
        token = self._run_tokens.get(run_id)
        if token is not None:
            return token
        entries = [e for e in await self.inbox.for_run(run_id) if e.token is not None]
        entries.sort(key=lambda e: e.token_iat or 0)
        return entries[-1].token if entries else None

    def _default_source(self, gateway_path: str) -> EventSource:
        if self.config.transport == "polling":
            return PollingSource(
                self.client,
                self.config.consumer,
                settings=self.settings,
                clock=self.clock,
                heartbeat=self.heartbeat,
            )
        gateway = Gateway(
            gateway_url(self.client.base_url, gateway_path),
            self.config.credentials,
            consumer=self.config.consumer,
            agent_id=self.config.agent_id,
            settings=self.settings,
            clock=self.clock,
            heartbeat=self.heartbeat,
            user_agent=f"{self.config.adapter_name}/{self.config.adapter_version}",
            nonce_sink=self.client.auth.remember_nonce,
        )
        source = GatewaySource(gateway)
        source.on_error = self._on_gateway_error
        return source

    def _on_gateway_error(self, frame: GatewayEvent) -> None:
        ref = frame.data.get("ref")
        if not isinstance(ref, str) or ":" not in ref:
            return
        try:
            run_id = UUID(ref.split(":", 1)[0])
        except ValueError:
            return
        state = self._runs.get(run_id)
        if state is not None and state.handle.started_patcher is not None:
            self._spawn(state.handle.started_patcher.on_gateway_error(frame.model))

    async def _close(self) -> None:
        for task in list(self._tasks):
            task.cancel()
        for task in list(self._tasks):
            with contextlib.suppress(asyncio.CancelledError, Exception):
                await task
        await self.inbox.close()
        await self.outbox.store.close()
        await self.client.aclose()

    def _spawn(self, coro: Coroutine[Any, Any, Any]) -> asyncio.Task[Any]:
        task: asyncio.Task[Any] = asyncio.ensure_future(coro)
        self._tasks.add(task)
        task.add_done_callback(self._tasks.discard)
        task.add_done_callback(_log_failure)
        return task

    def _fetcher(self, jwks_uri: str) -> Any:
        url = self.client.origin_url(jwks_uri)

        async def fetch() -> tuple[Mapping[str, Any], float | None]:
            response = await self.client.http.get(url, headers={"Accept": "application/json"})
            if response.status_code != httpx.codes.OK:
                raise ProblemError(response.status_code, "jwks_unavailable")
            return response.json(), _max_age(response.headers.get("Cache-Control"))

        return fetch

    async def _load_triggerers(self) -> AgentTriggerers:
        try:
            return await self.client.call_as(AgentTriggerers, get_agent_triggerers.asyncio_detailed)
        except ProblemError as exc:
            if exc.status == httpx.codes.NOT_FOUND:
                raise AllowlistUnavailable from None
            raise

    async def _on_ready(self) -> None:
        await self.outbox.flush()
        try:
            await self.handler.on_ready()
        except Exception:  # a handler failure must not stop the connection
            _LOG.exception("on_ready failed")
        if self.config.commands is not None:
            try:
                await reconcile_commands(
                    self.client, self.config.agent_id, self.config.commands, self.settings
                )
            except ProblemError as exc:
                _LOG.warning("command reconciliation failed code=%s", exc.code)

    def heartbeat(self) -> Heartbeat:
        """Return the agent heartbeat of this process (09 4.8), ``active_runs`` included.

        The members of `AgentHandler.runtime_status` replace or extend those of the
        configuration.

        Returns:
            The heartbeat (generated model).
        """
        status = self.handler.runtime_status()
        reasons = list(self._degraded)
        if status is not None:
            reasons += [r for r in status.degraded_reasons if r not in reasons]
        active = [str(s.run_id) for s in self._runs.values() if s.handle.token_jti is not None]
        if status is not None:
            active += [str(r) for r in status.active_runs if str(r) not in active]
        details: dict[str, Any] = {
            "active_runs": active,
            "adapter": {
                "name": self.config.adapter_name,
                "version": self.config.adapter_version,
            },
            "runtime": {"name": self.config.runtime_name},
            "protocol_version": 1,
        }
        if self.config.capabilities:
            details["capabilities"] = dict(self.config.capabilities)
        if reasons:
            details["degraded_reasons"] = reasons
        degraded = bool(reasons) or (status is not None and status.state == "degraded")
        runtime_version = self.config.runtime_version
        if status is not None and status.runtime_version is not None:
            runtime_version = status.runtime_version
        return build_heartbeat(
            "degraded" if degraded else "online",
            runtime_version=runtime_version,
            billing_modes_supported=None if status is None else status.billing_modes_supported,
            reported_tool_class=self.handler.actual_tool_class().value,
            details=details,
            project_readiness=None if status is None else status.project_readiness,
        )

    async def send_heartbeat(self) -> None:
        """Send the current heartbeat at once with ``sendHeartbeat``, outside any run binding.

        For a state change the next regular heartbeat would report too late (for example a
        project that became ready). A failure is logged; the regular heartbeat follows anyway.
        """
        try:
            with self.client.unbound():
                await self.client.call(send_heartbeat.asyncio_detailed, body=self.heartbeat())
        except (ProblemError, httpx.TransportError) as exc:
            _LOG.info("immediate heartbeat failed: %s", type(exc).__name__)

    # ---- restart -------------------------------------------------------------------------

    async def _recover(self) -> None:
        """Adapter restart (09 4.9): accepted runs start again, started ones are closed."""
        for entry in await self.inbox.in_states(EntryState.STARTED, EntryState.WAITING):
            # The runtime of this run died with the previous process; the server ends the run
            # interrupted and nothing is retried (R-AG-17).
            await self.inbox.set_state(entry.event_id, EntryState.CLOSED)
        for entry in await self.inbox.in_states(EntryState.ACCEPTED):
            if entry.type == _START_EVENT and entry.token is not None and entry.run_id is not None:
                state = self._state_for(entry, stored_claims(entry.token))
                state.task = self._spawn(self._start(state))
            else:
                await self.inbox.set_state(entry.event_id, EntryState.CLOSED)

    # ---- deliveries ----------------------------------------------------------------------

    async def _settle(self, env: Envelope) -> None:
        if env.seq is not None and self.source is not None:
            await self.source.settle(env.seq)

    async def _on_envelope(self, env: Envelope) -> None:
        if env.type in TOKEN_EVENT_TYPES:
            if env.type == _AUTOCOMPLETE:
                self._spawn(self._autocomplete(env))
                return
            await self._accept(env)
            return
        if env.type == _CANCEL:
            await self._on_cancel(env)
        elif env.type == _STATE_CHANGED:
            await self._on_state_changed(env)
        elif env.type == _ACCESS_CHANGED and self.verifier is not None:
            self.verifier.access_changed()
        if not isinstance(env.event, UnknownEvent):
            try:
                await self.handler.on_event(env)
            except Exception:  # a handler failure must not block the acknowledgement
                _LOG.exception("on_event failed for event %s", env.id)
        await self._settle(env)

    def _invocation(self, env: Envelope | InboxEntry, claims: InvocationClaims) -> Invocation:
        if isinstance(env, Envelope):
            event_id, seq, kind, payload = env.id, env.seq, env.type, env.event
        else:
            from bothy_sdk.tolerance import decode_event  # noqa: PLC0415 - avoid a cycle at import

            event_id, seq, kind = env.event_id, env.seq, env.type
            payload = decode_event(dict(env.envelope))
        return Invocation(event_id, seq, kind, getattr(payload, "payload", payload), claims)

    def _actual(self, env: Envelope) -> Any:
        def resolve(claims: InvocationClaims) -> Actual:
            inv = self._invocation(env, claims)
            billing = self.handler.actual_billing(inv)
            return Actual(billing.mode, self.handler.actual_tool_class().value)

        return resolve

    async def _check(self, env: Envelope) -> tuple[TokenVerdict, InvocationClaims | None]:
        if env.token is None or self.verifier is None:
            return TokenVerdict.INVALID, None
        try:
            expect = expectation_for(env.type, env.event)
        except (ValueError, TypeError, AttributeError):
            return TokenVerdict.INVALID, None
        result = await self.verifier.check(env.token, expect, self._actual(env))
        return result.verdict, result.claims

    def _run_id_of(self, env: Envelope, claims: InvocationClaims | None) -> UUID | None:
        payload = getattr(env.event, "payload", None)
        for holder in (payload, getattr(payload, "interaction", None)):
            run_id = getattr(holder, "run_id", None)
            if isinstance(run_id, UUID):
                return run_id
        return claims.run_id if claims is not None else None

    async def _accept(self, env: Envelope) -> None:
        stored = await self.inbox.get(env.id)
        if stored is not None:
            await self._redelivery(env, stored)
            return
        checked = await self._checked(env)
        if checked is None:
            return
        verdict, claims = checked
        run_id = self._run_id_of(env, claims)
        if verdict is TokenVerdict.UNKNOWN_KEY:
            _LOG.info("event %s: unknown key; not settled", env.id)
            return
        if verdict is TokenVerdict.REPLAY:
            _LOG.warning("event %s: replayed token ignored", env.id)
            return
        if verdict in _REFUSAL_REASON:
            await self._refuse(env, _REFUSAL_REASON[verdict], run_id)
            return
        if claims is None or env.token is None:  # VALID and EXPIRED always carry claims
            return
        entry = InboxEntry(
            event_id=env.id,
            seq=env.seq,
            type=env.type,
            run_id=run_id,
            envelope=dict(env.raw),
            token=env.token,
            token_iat=claims.iat,
            token_jti=claims.jti,
            state=EntryState.ACCEPTED,
            deliveries=1,
        )
        await self.inbox.add(entry)
        await self._settle(env)
        await self._dispatch(entry, env, claims)

    async def _checked(self, env: Envelope) -> tuple[TokenVerdict, InvocationClaims | None] | None:
        """The token check of a new event; ``None`` when the event is handled already."""
        try:
            return await self._check(env)
        except AllowlistUnavailable:
            await self._refuse(env, "adapter_error", self._run_id_of(env, None))
        except Exception:  # a failed JWKS or allow-list fetch: neither accept nor refuse
            _LOG.warning("token check of event %s failed; not settled", env.id)
        return None

    async def _refuse(self, env: Envelope, reason: str, run_id: UUID | None) -> None:
        state = self._runs.get(run_id) if run_id is not None else None
        starts_run = env.type == _START_EVENT or (
            env.type in (_FOLLOWUP, _INTERACTION) and state is not None and state.handle.waiting
        )
        if not starts_run or run_id is None:
            _LOG.warning("event %s refused (%s); run not ended", env.id, reason)
            await self._settle(env)
            return
        try:
            await self.client.call(
                finish_run.asyncio_detailed,
                run_id=run_id,
                body=RunFinish(state=RunFinishState.FAILED, reason=reason),
            )
        except ProblemError as exc:
            if exc.status >= httpx.codes.INTERNAL_SERVER_ERROR or exc.status in (401, 429):
                _LOG.warning("refusal of run %s not stored (%s); not settled", run_id, exc.code)
                return
            _LOG.info("refusal of run %s answered %s", run_id, exc.code)
        except httpx.TransportError:
            _LOG.warning("refusal of run %s not sent; not settled", run_id)
            return
        await self.inbox.add(
            InboxEntry(
                event_id=env.id,
                seq=env.seq,
                type=env.type,
                run_id=run_id,
                envelope=dict(env.raw),
                token=None,
                token_iat=None,
                token_jti=None,
                state=EntryState.CLOSED,
                deliveries=1,
            )
        )
        await self._settle(env)
        _LOG.warning("run %s refused: %s", run_id, reason)

    async def _redelivery(self, env: Envelope, stored: InboxEntry) -> None:
        token, iat, jti = None, None, None
        if env.token is not None and stored.state is EntryState.ACCEPTED:
            try:
                verdict, claims = await self._check(env)
            except Exception:
                verdict, claims = TokenVerdict.INVALID, None
            if verdict in (TokenVerdict.VALID, TokenVerdict.EXPIRED) and claims is not None:
                token, iat, jti = env.token, claims.iat, claims.jti
        updated = await self.inbox.redelivered(env.id, token, iat, jti)
        await self._settle(env)
        state = self._runs.get(updated.run_id) if updated.run_id is not None else None
        if (
            state is not None
            and state.waiting_for_token
            and updated.state is EntryState.ACCEPTED
            and updated.token_jti != state.invocation.claims.jti
        ):
            state.waiting_for_token = False
            state.task = self._spawn(self._start(state))

    def _state_for(self, entry: InboxEntry, claims: InvocationClaims) -> _RunState:
        if entry.run_id is None:
            raise ValueError("a start event carries a run")
        thread_id = claims.thread_id
        event_id = entry.event_id

        async def waiting() -> None:
            await self.inbox.set_state(event_id, EntryState.WAITING)

        handle = RunHandle(
            self.client,
            entry.run_id,
            thread_id,
            outbox=self.outbox,
            settings=self.settings,
            on_waiting=waiting,
        )
        state = _RunState(entry.run_id, event_id, handle, self._invocation(entry, claims))
        self._runs[entry.run_id] = state
        return state

    async def _dispatch(self, entry: InboxEntry, env: Envelope, claims: InvocationClaims) -> None:
        inv = self._invocation(env, claims)
        if env.type == _START_EVENT:
            fresh = self._state_for(entry, claims)
            fresh.task = self._spawn(self._start(fresh))
            return
        run_id = entry.run_id
        state = self._runs.get(run_id) if run_id is not None else None
        if env.type in (_FOLLOWUP, _INTERACTION):
            if state is None:
                _LOG.info("event %s for a run this process does not hold", env.id)
                await self.inbox.set_state(entry.event_id, EntryState.CLOSED)
                return
            self._spawn(self._continue(state, entry, inv))
            return
        if env.type == _COMMAND:
            self._spawn(self._command(entry, inv, state))

    # ---- runs ----------------------------------------------------------------------------

    async def _newest(self, state: _RunState) -> InboxEntry:
        entry = await self.inbox.get(state.event_id)
        if entry is None:
            raise KeyError(str(state.event_id))
        return entry

    async def _renew(self, state: _RunState, token: Secret, event_id: UUID | None = None) -> Secret:
        """``renewInvocationToken`` once per run at a time; the grant becomes the newest token."""
        target = event_id or state.event_id
        async with state.renewing:
            current = await self.inbox.get(target)
            if current is not None and current.token is not None and current.token != token:
                return current.token  # another renewal or a redelivery won the race
            grant = await self.client.call_as(
                InvocationTokenGrant,
                renew_invocation_token.asyncio_detailed,
                run_id=state.run_id,
                body=InvocationTokenRenew(invocation_token=token.reveal()),
            )
            renewed = Secret(grant.invocation_token)
            claims = stored_claims(renewed)
            await self.inbox.replace_token(target, renewed, claims.iat, claims.jti)
            self._run_tokens[state.run_id] = renewed
            return renewed

    async def _renew_or_close(
        self, state: _RunState, token: Secret, event_id: UUID | None = None
    ) -> Secret | None:
        """Renew, reacting to a refusal as the ``reason`` table of 09 4.5 and 4.5.1 says.

        Returns the renewed token, or ``None`` when the run must not start here (the entry is
        handled: superseded waits, the rest is closed).
        """
        try:
            return await self._renew(state, token, event_id)
        except InvocationTokenInvalid as exc:
            if exc.reason == "superseded" and event_id is None:
                await self._superseded(state, token)
            elif exc.reason == "superseded":
                pass  # a newer delivery for the waiting run is on its way
            else:
                await self._finish_refused(state, "invalid_invocation_token")
            return None
        except (AlreadyStarted, AgentInvocationDenied) as exc:
            _LOG.info("renewal for run %s refused: %s", state.run_id, exc.code)
        except (ProblemError, httpx.TransportError) as exc:
            _LOG.warning("renewal for run %s failed: %s", state.run_id, type(exc).__name__)
        if event_id is None:
            await self.inbox.set_state(state.event_id, EntryState.CLOSED)
            self._runs.pop(state.run_id, None)
        return None

    def _expired(self, claims: InvocationClaims) -> bool:
        now = self.clock.now().timestamp()
        return now >= claims.exp + self.settings.invocation_token_leeway

    async def _prepare(self, state: _RunState) -> StartOptions | None:
        entry = await self._newest(state)
        if entry.token is None:
            return None
        try:
            with self.client.bind_run(state.run_id, entry.token):
                return await self.handler.prepare(state.invocation, state.handle)
        except asyncio.CancelledError:
            # A cancel request closes the entry; a stop of the adapter keeps it accepted, so the
            # next process starts the run again from the inbox (09 4.9).
            if state.cancel_reason is not None:
                await self.inbox.set_state(state.event_id, EntryState.CLOSED)
            raise
        except RunRefused as refusal:
            if refusal.reason is not None:
                with self.client.bind_run(state.run_id, entry.token):
                    await self._finish_refused(state, refusal.reason)
            else:
                await self.inbox.set_state(state.event_id, EntryState.CLOSED)
                self._runs.pop(state.run_id, None)
            _LOG.info("run %s refused in prepare: %s", state.run_id, refusal.reason or "inactive")
            return None
        except Exception:
            _LOG.exception("prepare failed for run %s", state.run_id)
            await self._finish_refused(state, "adapter_error")
            return None

    async def _start(self, state: _RunState) -> None:
        """Start a run from the inbox: prepare, compare again, renew if expired, start.

        `AgentHandler.on_not_started` follows every attempt that does not reach ``execute``.
        """
        executing = False
        try:
            executing = await self._start_attempt(state)
        finally:
            if not executing:
                with contextlib.suppress(Exception):
                    await self.handler.on_not_started(state.invocation)

    async def _start_attempt(self, state: _RunState) -> bool:
        """One start attempt; True once ``execute`` was entered (it has ended when this returns)."""
        options = await self._prepare(state)
        if options is None:
            return False
        entry = await self._newest(state)
        if entry.state is not EntryState.ACCEPTED or entry.token is None:
            return False
        claims = stored_claims(entry.token)
        inv = state.invocation
        state.invocation = inv = Invocation(inv.event_id, inv.seq, inv.type, inv.payload, claims)
        billing = self.handler.actual_billing(inv)
        tool_class = self.handler.actual_tool_class()
        mismatch = compare_actual(claims, Actual(billing.mode, tool_class.value))
        if mismatch is not TokenVerdict.VALID:
            await self._finish_refused(state, _REFUSAL_REASON[mismatch])
            return False
        token = entry.token
        if self._expired(claims):
            renewed = await self._renew_or_close(state, token)
            if renewed is None:
                return False
            token = renewed
        body = RunStart(
            invocation_token=token.reveal(),
            billing_mode_used=BillingMode(billing.mode),
            tool_class_used=tool_class,
        )
        if options.model is not None:
            body.model = options.model
        if options.external_session_ref is not None:
            body.external_session_ref = options.external_session_ref
        evidence = options.billing_evidence or billing.evidence
        if evidence is not None:
            body.billing_evidence = evidence
        started = await self._start_call(state, token, body)
        if started is None:
            return False
        run, token = started
        await self.inbox.set_state(state.event_id, EntryState.STARTED)
        state.handle.started(run, stored_claims(token).jti)
        self.client.set_run_token(state.run_id, token)
        self._run_tokens[state.run_id] = token
        await self._execute(state, token, self.handler.execute, inv)
        return True

    async def _start_call(
        self, state: _RunState, token: Secret, body: RunStart
    ) -> tuple[Run, Secret] | None:
        """``startRun`` with the reactions of the ``reason`` table of 09 4.5."""
        for attempt in range(2):
            body.invocation_token = token.reveal()
            try:
                run = await self.client.call_as(
                    Run, start_run.asyncio_detailed, run_id=state.run_id, body=body
                )
            except InvocationTokenInvalid as exc:
                if exc.reason == "expired" and attempt == 0:
                    renewed = await self._renew_or_close(state, token)
                    if renewed is None:
                        return None
                    token = renewed
                    continue
                if exc.reason == "superseded":
                    await self._superseded(state, token)
                else:
                    await self._finish_refused(state, "invalid_invocation_token")
                return None
            except (AlreadyStarted, BillingModeMismatch, ToolClassMismatch, AgentInvocationDenied):
                # The server already ended or holds the run: close the entry, start nothing.
                await self.inbox.set_state(state.event_id, EntryState.CLOSED)
                self._runs.pop(state.run_id, None)
                return None
            return run, token
        return None

    async def _execute(self, state: _RunState, token: Secret, work: Any, inv: Invocation) -> None:
        """Run handler work of a started run and end the run as 09 4.9 says."""
        handle = state.handle
        try:
            with self.client.bind_run(state.run_id, token):
                await work(inv, handle)
                if not handle.finished and not handle.waiting:
                    await handle.finish("completed")
        except asyncio.CancelledError:
            if state.cancel_reason is not None and not handle.finished:
                with self.client.bind_run(state.run_id, token), contextlib.suppress(ProblemError):
                    await handle.finish("cancelled", reason=state.cancel_reason)
            raise
        except RuntimeFailure:
            with self.client.bind_run(state.run_id, token), contextlib.suppress(ProblemError):
                await handle.finish("failed", reason="runtime_error")
        except Exception:
            _LOG.exception("handler failed for run %s", state.run_id)
            with self.client.bind_run(state.run_id, token), contextlib.suppress(ProblemError):
                await handle.finish("failed", reason="adapter_error")
        finally:
            if handle.finished or handle.cancelled.is_set():
                with contextlib.suppress(Exception):
                    await self.inbox.set_state(state.event_id, EntryState.CLOSED)
                self._runs.pop(state.run_id, None)
                self.client.release_run(state.run_id)

    async def _finish_refused(self, state: _RunState, reason: str) -> None:
        with contextlib.suppress(ProblemError, httpx.TransportError):
            await self.client.call(
                finish_run.asyncio_detailed,
                run_id=state.run_id,
                body=RunFinish(state=RunFinishState.FAILED, reason=reason),
            )
        await self.inbox.set_state(state.event_id, EntryState.CLOSED)
        self._runs.pop(state.run_id, None)

    async def _superseded(self, state: _RunState, used: Secret) -> None:
        """``superseded``: start nothing; close when the run left ``queued``, else wait."""
        try:
            run = await self.client.call_as(Run, get_run.asyncio_detailed, run_id=state.run_id)
        except (ProblemError, httpx.TransportError):
            run = None
        if run is not None and run.state.value != "queued":
            await self.inbox.set_state(state.event_id, EntryState.CLOSED)
            self._runs.pop(state.run_id, None)
            return
        state.waiting_for_token = True
        # A redelivery may have stored a newer token while startRun was on its way.
        entry = await self.inbox.get(state.event_id)
        # A redelivery may also have restarted the run during the read: start at most once.
        if (
            state.waiting_for_token
            and entry is not None
            and entry.token is not None
            and entry.token != used
        ):
            state.waiting_for_token = False
            state.task = self._spawn(self._start(state))

    async def _continue(self, state: _RunState, entry: InboxEntry, inv: Invocation) -> None:
        """A follow-up or interaction: resume a waiting run, or hand it to a running one.

        Resumes are serialised per run: of two answers to one question, the first resumes the
        run and the second reaches the handler as a follow-up of the running run.
        """
        if entry.token is None:  # accepted entries always carry their token
            return
        work = self.handler.on_followup if entry.type == _FOLLOWUP else self.handler.on_interaction
        async with state.resuming:
            token = await self._resume(state, entry, inv) if state.handle.waiting else None
            running_here = state.handle.token_jti is not None and not state.handle.waiting
        await self.inbox.set_state(entry.event_id, EntryState.CLOSED)
        if token is not None:
            state.task = asyncio.current_task()
            await self._execute(state, token, work, inv)
            return
        handle = state.handle
        not_started = handle.token_jti is None and not handle.waiting
        if entry.type == _FOLLOWUP and not_started and state.cancel_reason is None:
            # Held by the handler until the run starts (D-M2-35-3); bound like the run itself.
            start = await self.inbox.get(state.event_id)
            token = start.token if start is not None and start.token is not None else entry.token
            with self.client.bind_run(state.run_id, token):
                await work(inv, handle)
            return
        if running_here and not state.handle.finished:
            with self.client.bind_run(
                state.run_id, self._run_tokens.get(state.run_id, entry.token)
            ):
                await work(inv, state.handle)

    async def _resume(self, state: _RunState, entry: InboxEntry, inv: Invocation) -> Secret | None:
        """``resumeRun`` with the event's token; ``None`` when the run did not resume here.

        An expired token is renewed once; a superseded one is replaced once by the token of a
        newer answer of the same run.
        """
        token: Secret | None = entry.token
        for attempt in range(2):
            if token is None:
                return None
            body = RunStart(
                invocation_token=token.reveal(),
                billing_mode_used=BillingMode(self.handler.actual_billing(inv).mode),
                tool_class_used=self.handler.actual_tool_class(),
            )
            try:
                run = await self.client.call_as(
                    Run, resume_run.asyncio_detailed, run_id=state.run_id, body=body
                )
            except InvocationTokenInvalid as exc:
                _LOG.info("resume of run %s refused: %s", state.run_id, exc.reason)
                if attempt == 0 and exc.reason == "expired":
                    token = await self._renew_or_close(state, token, entry.event_id)
                    continue
                if attempt == 0 and exc.reason == "superseded":
                    token = await self._newer_answer(state, entry)
                    continue
                if exc.reason == "invalid":
                    await self._finish_refused(state, "invalid_invocation_token")
                return None
            except (AlreadyStarted, BillingModeMismatch, ToolClassMismatch, AgentInvocationDenied):
                return None
            except (ProblemError, httpx.TransportError) as exc:
                _LOG.warning("resume of run %s failed: %s", state.run_id, type(exc).__name__)
                return None
            await self.inbox.set_state(state.event_id, EntryState.STARTED)
            state.handle.started(run, stored_claims(token).jti)
            self.client.set_run_token(state.run_id, token)
            self._run_tokens[state.run_id] = token
            return token
        return None

    async def _newer_answer(self, state: _RunState, entry: InboxEntry) -> Secret | None:
        """The token of a later accepted follow-up or interaction of the same run, if any."""
        newer = [
            e
            for e in await self.inbox.for_run(state.run_id)
            if e.event_id != entry.event_id
            and e.type in (_FOLLOWUP, _INTERACTION)
            and e.token is not None
            and (e.token_iat or 0) >= (entry.token_iat or 0)
        ]
        newer.sort(key=lambda e: e.token_iat or 0)
        return newer[-1].token if newer else None

    async def _command(self, entry: InboxEntry, inv: Invocation, state: _RunState | None) -> None:
        if entry.token is None:  # accepted entries always carry their token
            return
        handle = state.handle if state is not None and state.handle.token_jti else None
        try:
            if handle is not None:
                with self.client.bind_run(handle.run_id, entry.token):
                    await self.handler.on_command(inv, handle)
            else:
                await self.handler.on_command(inv, None)
        finally:
            await self.inbox.set_state(entry.event_id, EntryState.CLOSED)

    async def _on_cancel(self, env: Envelope) -> None:
        payload = getattr(env.event, "payload", None)
        run_id = getattr(payload, "run_id", None)
        state = self._runs.get(run_id) if isinstance(run_id, UUID) else None
        if state is None:
            return
        if state.cancel_reason is not None:
            return  # a second request for the same run changes nothing
        raw = getattr(payload, "reason", None)
        reason = getattr(raw, "value", raw) or "user_cancelled"
        state.cancel_reason = str(reason)
        state.handle.cancelled.set()
        if state.handle.token_jti is None:
            # Not started: never start it, and finish it cancelled with the request's reason
            # (09 8 point 3; a run the server already cancelled answers 200, D-M2-35-12).
            if state.task is not None:
                state.task.cancel()
            await self._finish_cancelled_unstarted(state)
            await self.inbox.set_state(state.event_id, EntryState.CLOSED)
            self._runs.pop(state.run_id, None)
            return
        task_running = state.task is not None and not state.task.done()
        if task_running and self._handler_cancels():
            self._spawn(self._cancel_through_handler(state, state.cancel_reason))
            return
        if task_running and state.task is not None:
            state.task.cancel()
        elif not state.handle.finished:
            entry = await self._newest(state)
            if entry.token is not None:
                with (
                    self.client.bind_run(state.run_id, entry.token),
                    contextlib.suppress(ProblemError),
                ):
                    await state.handle.finish("cancelled", reason=state.cancel_reason)
            await self.inbox.set_state(state.event_id, EntryState.CLOSED)
            self._runs.pop(state.run_id, None)

    def _handler_cancels(self) -> bool:
        """Whether the handler implements `AgentHandler.on_cancel`."""
        return type(self.handler).on_cancel is not AgentHandler.on_cancel

    async def _cancel_through_handler(self, state: _RunState, reason: str) -> None:
        """Let the handler stop its runtime and finish the run (D-M2-35-12)."""
        token = self._run_tokens.get(state.run_id)
        try:
            if token is None:
                await self.handler.on_cancel(state.handle, reason)
                return
            with self.client.bind_run(state.run_id, token):
                await self.handler.on_cancel(state.handle, reason)
        except Exception:  # a handler failure must not stop the loop
            _LOG.exception("on_cancel failed for run %s", state.run_id)

    async def _finish_cancelled_unstarted(self, state: _RunState) -> None:
        try:
            entry = await self._newest(state)
        except KeyError:
            return
        if entry.token is None or state.handle.finished:
            return
        with (
            self.client.bind_run(state.run_id, entry.token),
            contextlib.suppress(ProblemError, httpx.TransportError),
        ):
            await state.handle.finish("cancelled", reason=state.cancel_reason)

    async def _on_state_changed(self, env: Envelope) -> None:
        summary = getattr(getattr(env.event, "payload", None), "run", None)
        run_id = getattr(summary, "id", None)
        state = self._runs.get(run_id) if isinstance(run_id, UUID) else None
        new_state = getattr(getattr(summary, "state", None), "value", None)
        if state is not None and state.waiting_for_token and new_state not in (None, "queued"):
            await self.inbox.set_state(state.event_id, EntryState.CLOSED)
            self._runs.pop(state.run_id, None)

    async def _autocomplete(self, env: Envelope) -> None:
        try:
            verdict, _ = await self._check(env)
        except Exception:
            _LOG.warning("autocomplete %s: token check failed", env.id)
            return
        if verdict not in (TokenVerdict.VALID,):
            _LOG.warning("autocomplete %s refused (%s)", env.id, verdict.value)
            return
        p = getattr(env.event, "payload", None)
        if p is None or self.source is None:
            return
        options = getattr(p, "options", None)
        request = AutocompleteRequest(
            request_id=p.request_id,
            command_id=p.command_id,
            option=p.option,
            value=p.value,
            options=[o.to_dict() for o in options] if isinstance(options, list) else [],
            channel_id=p.channel_id,
            thread_id=p.thread_id if isinstance(p.thread_id, UUID) else None,
            user_id=p.user_id,
            expires_at=p.expires_at,
        )
        await guarded_autocomplete(
            self.handler.on_autocomplete,
            request,
            self.source.answer_autocomplete,
            clock=self.clock,
            settings=self.settings,
        )


def _max_age(cache_control: str | None) -> float | None:
    for directive in (cache_control or "").split(","):
        name, _, value = directive.strip().partition("=")
        if name.lower() == "max-age":
            try:
                return max(0.0, float(value.strip('"')))
            except ValueError:
                return None
    return None


def _log_failure(task: asyncio.Task[Any]) -> None:
    if task.cancelled():
        return
    exc = task.exception()
    if exc is not None:
        _LOG.error("adapter task failed: %s", type(exc).__name__)
