# SPDX-License-Identifier: Apache-2.0
"""Hosting of built-in agents in the runner process (docs/spec/26-builtin-agents.md 3, 09 18.18).

With ``runner.hosted_agents`` the runner also serves every active agent whose ``hosted_by`` is its
own agent. `HostedAgents` keeps that set: it lists it with ``listHostedAgents`` once the runner's
own connection is ready (its heartbeat with ``hosts_agents`` is stored then), on
``agent.hosting_changed`` and every ``runner.channels_resync_interval`` (26 3.2 item 1). Each
hosted agent gets a `HostedConnection`: a token from ``issueHostedAgentToken`` kept in memory
only (`HostedToken`, 26 3.3 item 2), an SDK adapter with its own gateway connection, consumer
``default`` of its own principal, inbox and outbox under ``<runner.state_dir>/hosted/<agent_id>/``,
its own channel cache and heartbeat, signed with the runner's DPoP key because the token is bound
to it. Turns of every agent share the runner's `SlotPool` (26 3.2 item 3).

Until the runner runs definitions (26 5), `HostedHandler` accepts a hosted run, takes its thread
lock and a shared slot like a turn would, and ends it ``failed`` with ``adapter_error`` and the
cause ``definition_missing`` without ``run.start`` (26 5.6 item 4).

Log lines name agent IDs and problem codes, never a token: ``hosted_agent_joined``,
``hosted_agent_left`` (with ``reason`` ``unlisted``, ``revoked`` or ``closed``),
``hosted_token_refreshed`` and ``hosted_token_issue_failed`` (with ``code``).

Examples:
    >>> from datetime import UTC, datetime, timedelta
    >>> created = datetime(2026, 11, 6, tzinfo=UTC)
    >>> refresh_at(created, created + timedelta(hours=24)) == created + timedelta(hours=18)
    True
"""

from __future__ import annotations

import asyncio
import contextlib
import dataclasses
import json
import logging
import random
from collections.abc import Awaitable, Callable, Mapping
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Any
from uuid import UUID

import httpx
from bothy_sdk.adapter import (
    ActualBilling,
    Adapter,
    AdapterConfig,
    AgentHandler,
    Invocation,
    RunRefused,
    RuntimeStatus,
    StartOptions,
)
from bothy_sdk.auth import Credentials, DpopKeySource, Secret
from bothy_sdk.client import Client
from bothy_sdk.clock import Clock
from bothy_sdk.errors import ProblemError
from bothy_sdk.generated.api.agent_runtime import issue_hosted_agent_token, list_hosted_agents
from bothy_sdk.generated.models import AgentPage, ApiTokenCreated, ToolClass
from bothy_sdk.generated.types import UNSET, Unset
from bothy_sdk.retry import backoff
from bothy_sdk.runs import RunHandle
from bothy_sdk.settings import SdkSettings
from bothy_sdk.sources import Envelope

from claude_code_runner import ADAPTER_NAME, RUNTIME_NAME, __version__
from claude_code_runner.causes import cause
from claude_code_runner.channels import RESYNC_EVENTS, ChannelCache
from claude_code_runner.heartbeat import WAIT_SLOTS_FULL, WAIT_THREAD_BUSY, Probe, runtime_status
from claude_code_runner.locks import LockTimeout, SlotPool, acquire
from claude_code_runner.runturn import RunState
from claude_code_runner.statedir import hosted_dir
from claude_code_runner.toolclass import lower

__all__ = [
    "HOSTED_CONSUMER",
    "HOSTING_CHANGED",
    "TOKEN_REFRESH_FRACTION",
    "HostedAgents",
    "HostedConnection",
    "HostedHandler",
    "HostedToken",
    "HostingContext",
    "issue_token",
    "refresh_at",
]

_LOG = logging.getLogger("claude_code_runner")

HOSTING_CHANGED = "agent.hosting_changed"
"""The durable event that changes the hosted set (26 9); its payload names one agent."""

HOSTED_CONSUMER = "default"
"""The consumer of every hosted agent: each is a principal of its own (brief A1-04 note 3)."""

TOKEN_REFRESH_FRACTION = 0.25
"""How much of a hosted token's lifetime is left when the runner fetches the next one.

The lifetime is ``agents.hosted_token_ttl`` of the server, read from the token's ``created_at``
and ``expires_at``. A quarter leaves room for many attempts with the SDK's retry backoff when the
server is briefly unreachable, while each token still serves most of its lifetime, so the server
issues few tokens."""

_REVOKED = 4012
"""The close code of a revoked token (04 6): the agent waits for the next list."""


def _log(event: str, **ids: object) -> None:
    _LOG.info(json.dumps({"event": event, **{k: str(v) for k, v in ids.items()}}))


def _warn(event: str, **ids: object) -> None:
    _LOG.warning(json.dumps({"event": event, **{k: str(v) for k, v in ids.items()}}))


def refresh_at(created_at: datetime, expires_at: datetime) -> datetime:
    """Return when the runner fetches the next token: `TOKEN_REFRESH_FRACTION` before expiry.

    Args:
        created_at: The token's ``created_at``.
        expires_at: The token's ``expires_at``.

    Returns:
        The point in time, between both.
    """
    return expires_at - (expires_at - created_at) * TOKEN_REFRESH_FRACTION


@dataclass(frozen=True)
class HostedToken:
    """A hosted agent's API token, in memory only (26 3.3 item 2).

    Never written to a file, a log line, a turn record or an environment; `Secret` refuses to
    show or pickle its value.

    Attributes:
        secret: The token.
        expires_at: Its ``expires_at``; ``None`` when the server named none.
        refresh_at: When the next one is fetched (`refresh_at`); ``None`` without an expiry.
    """

    secret: Secret = field(repr=False)
    expires_at: datetime | None
    refresh_at: datetime | None


async def issue_token(client: Client, agent_id: UUID) -> HostedToken:
    """Fetch a token for a hosted agent with ``issueHostedAgentToken`` (26 3.3 item 1).

    The call goes out with the runner's own token and DPoP key, outside any run binding; the
    server revokes the runner's older tokens of the agent in the same step.

    Args:
        client: The runner's own SDK client.
        agent_id: The hosted agent.

    Returns:
        The token.

    Raises:
        ProblemError: The server refused (``404`` when the agent is not hosted by this runner,
            or this runner's heartbeat does not report ``hosts_agents`` yet).
        httpx.TransportError: The server was not reachable.
    """
    with client.unbound():
        created = await client.call_as(
            ApiTokenCreated, issue_hosted_agent_token.asyncio_detailed, agent_id=agent_id
        )
    meta = created.token
    expires = None if isinstance(meta.expires_at, Unset) else meta.expires_at
    return HostedToken(
        Secret(created.secret),
        expires_at=expires,
        refresh_at=None if expires is None else refresh_at(meta.created_at, expires),
    )


@dataclass(frozen=True)
class HostingContext:
    """What every hosted connection takes from the runner.

    Attributes:
        client: The runner's own SDK client (lists and token issues).
        dpop: The runner's DPoP key; it signs the proofs of every hosted connection.
        instance_id: ``runner.instance_id``.
        server_url: The origin of a hosted agent's server: ``runner.server_url`` (tests point
            each hosted agent to a server of its own).
        state_dir: ``runner.state_dir``.
        projects_root: ``runner.projects_root`` (the channel caches compute no folder there).
        ceiling: ``runner.hosted_max_tool_class``.
        allow_personal: ``runner.allow_personal``.
        modes: The runner's ``billing_modes_supported``.
        probe: The runner's last version probe (one CLI serves every agent).
        capabilities: The runner's capabilities without ``hosts_agents`` (26 3.2 item 2).
        slots: The runner's shared slot pool.
        session_lock_wait: ``runner.session_lock_wait``.
        poll_interval: ``runner.poll_interval``.
        resync_interval: ``runner.channels_resync_interval``.
        settings: The SDK settings.
        clock: The SDK's time source.
        http_transport: An ``httpx`` transport for the SDK clients (tests).
    """

    client: Client
    dpop: DpopKeySource
    instance_id: UUID
    server_url: Callable[[UUID], str]
    state_dir: Path
    projects_root: Path
    ceiling: str
    allow_personal: bool
    modes: tuple[str, ...] | None
    probe: Callable[[], Probe]
    capabilities: Mapping[str, Any]
    slots: SlotPool
    session_lock_wait: float
    poll_interval: float
    resync_interval: float
    settings: SdkSettings
    clock: Clock
    http_transport: httpx.AsyncBaseTransport | None = None


class HostedHandler(AgentHandler):
    """The runtime side of one hosted agent for the SDK's run loop.

    Args:
        agent_id: The hosted agent.
        tool_class: The agent's loaded class now (the lower of its class and the ceiling).
        context: The runner's parts.
        on_agent_updated: Called on ``agent.updated`` of the agent's own connection, so the
            set is listed again with the agent's new class (26 3.1 item 3).
    """

    def __init__(
        self,
        agent_id: UUID,
        tool_class: Callable[[], str],
        context: HostingContext,
        on_agent_updated: Callable[[], Awaitable[None]],
    ) -> None:
        """Create the handler; `HostedConnection` sets the adapter and the channel cache."""
        self.agent_id = agent_id
        self._tool_class = tool_class
        self._ctx = context
        self._on_agent_updated = on_agent_updated
        self.adapter: Adapter | None = None
        self.channels: ChannelCache | None = None
        self._runs: dict[UUID, RunState] = {}
        self._waiting: dict[UUID, str] = {}
        self._beats: set[asyncio.Task[None]] = set()
        self._locks = hosted_dir(context.state_dir, agent_id) / "locks"

    def actual_tool_class(self) -> ToolClass:
        """The loaded class: the lower of the agent's class and ``runner.hosted_max_tool_class``."""
        return ToolClass(self._tool_class())

    def actual_billing(self, inv: Invocation) -> ActualBilling:
        """The desired mode when the runner may run it (as for its own agent), else ``unknown``.

        Args:
            inv: The invocation.

        Returns:
            ``organization``, ``personal`` with ``runner.allow_personal``, else ``unknown``.
        """
        desired = inv.claims.billing_mode
        if desired == "organization" or (desired == "personal" and self._ctx.allow_personal):
            return ActualBilling(desired)
        return ActualBilling("unknown")

    def runtime_status(self) -> RuntimeStatus:
        """The runner's runtime state, the shared slot totals and this agent's waiting runs.

        No project readiness, folders or folder moves: a built-in agent serves no bound forum
        (R-AG-39).
        """
        slots = self._ctx.slots
        status = runtime_status(
            self._ctx.probe(),
            (),
            self._ctx.modes,
            slots=(slots.used, slots.total),
            waiting=dict(self._waiting),
        )
        return dataclasses.replace(status, project_readiness=None)

    async def on_ready(self) -> None:
        """Send the agent's heartbeat as soon as its connection is ready; load its channels.

        A failed load is logged and tried again by the next event or the timer.
        """
        if self.adapter is not None:
            await self.adapter.send_heartbeat()
        await self.resync_channels("ready")

    async def on_event(self, envelope: Envelope) -> None:
        """Reload the agent's channels on their events; list the set again on ``agent.updated``.

        Args:
            envelope: The event.
        """
        cache = self.channels
        if envelope.type == "agent.updated":
            await self._on_agent_updated()
        elif envelope.type in RESYNC_EVENTS and cache is not None:
            await self.resync_channels(envelope.type)
        elif envelope.type == "agent.config_changed" and cache is not None:
            channel_id = getattr(getattr(envelope.event, "payload", None), "channel_id", None)
            if isinstance(channel_id, UUID):
                await cache.reload(channel_id)

    async def resync_channels(self, reason: str) -> None:
        """Load the agent's served channels again; a failure keeps the cache and is logged.

        Args:
            reason: The event type or ``timer``, for the log line.
        """
        cache = self.channels
        if cache is None:
            return
        try:
            await cache.resync()
        except (ProblemError, httpx.TransportError) as err:
            _warn(
                "hosted_channels_resync_failed",
                agent_id=self.agent_id,
                cause=reason,
                code=getattr(err, "code", "transport"),
            )

    def _wait(self, run_id: UUID, reason: str | None) -> None:
        """Note a wait reason of a run in prepare and report it at once (09 5.2.1)."""
        if self._waiting.get(run_id) == reason:
            return
        if reason is None:
            self._waiting.pop(run_id, None)
        else:
            self._waiting[run_id] = reason
            _log("run_waiting", run_id=run_id, agent_id=self.agent_id, reason=reason)
        if self.adapter is None:
            return
        task = asyncio.get_running_loop().create_task(self.adapter.send_heartbeat())
        self._beats.add(task)
        task.add_done_callback(self._beats.discard)

    async def prepare(self, inv: Invocation, run: RunHandle) -> StartOptions:
        """Take the thread lock and a shared slot, then refuse: no definition runs yet.

        The lock and the slot are what a hosted turn holds (26 3.2 item 3), so the agent's run
        waits with ``thread_busy`` or ``slots_full`` like any turn; the definition check of
        26 5.6 item 4 then ends it without ``run.start``.

        Args:
            inv: The invocation.
            run: The queued run.

        Raises:
            RunRefused: Always: ``adapter_error`` with the cause ``definition_missing``, or
                ``session_busy`` when the thread stayed locked for ``runner.session_lock_wait``.
        """
        payload = inv.payload
        state = RunState(run.run_id, payload.thread_id, payload.channel_id)
        self._runs[run.run_id] = state
        ids = {"run_id": run.run_id, "agent_id": self.agent_id, "thread_id": state.thread_id}
        try:
            try:
                state.locks.append(
                    await acquire(
                        self._locks / f"thread-{state.thread_id}.lock",
                        self._ctx.session_lock_wait,
                        self._ctx.poll_interval,
                        on_busy=lambda: self._wait(run.run_id, WAIT_THREAD_BUSY),
                    )
                )
            except LockTimeout:
                _log("session_busy", **ids)
                raise RunRefused("adapter_error", summary=cause("session_busy")) from None
            slots = self._ctx.slots
            if slots.full:
                self._wait(run.run_id, WAIT_SLOTS_FULL)
            await slots.acquire()
            state.slot = slots
        finally:
            self._wait(run.run_id, None)
        _log("definition_missing", **ids)
        raise RunRefused("adapter_error", summary=cause("definition_missing"))

    async def on_not_started(self, inv: Invocation) -> None:
        """Give back the slot and the lock of a run that ended in prepare.

        Args:
            inv: The invocation.
        """
        run_id = inv.claims.run_id
        state = self._runs.pop(run_id, None) if run_id else None
        if state is not None:
            state.close()

    async def execute(self, inv: Invocation, run: RunHandle) -> None:
        """Not reached (prepare always refuses); ends a started run like prepare would.

        Args:
            inv: The invocation.
            run: The started run.
        """
        del inv
        await run.finish("failed", reason="adapter_error", summary=cause("definition_missing"))

    async def close(self) -> None:
        """Cancel the heartbeats sent for wait reasons."""
        for beat in list(self._beats):
            beat.cancel()
            with contextlib.suppress(asyncio.CancelledError, Exception):
                await beat


class HostedConnection:
    """One hosted agent: its token, its connection and their renewal.

    `start` runs it until `stop`, until its token is revoked (``4012``) or until the server
    ends the connection for good; the next list of `HostedAgents` starts a new one when the
    agent is still hosted.

    Args:
        agent_id: The hosted agent.
        tool_class: The agent's own class from the list.
        context: The runner's parts.
        on_agent_updated: See `HostedHandler`.
    """

    def __init__(
        self,
        agent_id: UUID,
        tool_class: str,
        context: HostingContext,
        on_agent_updated: Callable[[], Awaitable[None]],
    ) -> None:
        """Create the connection; nothing connects before `start`."""
        self.agent_id = agent_id
        self.tool_class = tool_class
        self._ctx = context
        self.handler = HostedHandler(agent_id, self.loaded_class, context, on_agent_updated)
        self._token: HostedToken | None = None
        self._adapter: Adapter | None = None
        self._task: asyncio.Task[None] | None = None
        self._stopping = False
        self._issuing = False
        self._rng = random.Random()  # noqa: S311 - jitter of a retry wait, not a secret  # nosec B311

    def loaded_class(self) -> str:
        """The class the agent runs with: the lower of its own and the runner's ceiling."""
        return lower(self.tool_class, self._ctx.ceiling)

    @property
    def stopping(self) -> bool:
        """True once `stop` was called."""
        return self._stopping

    @property
    def done(self) -> bool:
        """True when the connection ended by itself (revoked or closed for good)."""
        return self._task is not None and self._task.done()

    def start(self) -> None:
        """Start serving the agent in the background."""
        self._task = asyncio.get_running_loop().create_task(self._serve())

    async def set_tool_class(self, tool_class: str) -> None:
        """Take the agent's class from a new list; report a change at once.

        Args:
            tool_class: The agent's class.
        """
        if tool_class == self.tool_class:
            return
        self.tool_class = tool_class
        if self._adapter is not None:
            await self._adapter.send_heartbeat()

    async def resync_channels(self) -> None:
        """Load the agent's served channels again (the timer of 09 18.1)."""
        await self.handler.resync_channels("timer")

    async def stop(self) -> None:
        """Close the connection and wait until the agent is no longer served."""
        self._stopping = True
        adapter = self._adapter
        if adapter is not None:
            await adapter.stop()
        task = self._task
        if task is not None and not task.done():
            try:
                await asyncio.wait_for(asyncio.shield(task), self._ctx.settings.http_timeout)
            except TimeoutError:
                task.cancel()
                with contextlib.suppress(asyncio.CancelledError, Exception):
                    await task
            except Exception:
                _LOG.debug("hosted connection ended with an error")

    async def _serve(self) -> None:
        attempt = 0
        try:
            while not self._stopping:
                token = await self._valid_token()
                if token is None:
                    return
                adapter = self._build(token)
                closed = await self._run(adapter, token)
                if self.stopping:
                    return
                if closed is None:  # a failure before or around the connection: try again
                    await self._ctx.clock.sleep(backoff(attempt, self._ctx.settings, self._rng))
                    attempt += 1
                    continue
                attempt = 0
                if self._token is token:  # not a renewal: the connection ended for good
                    self._token = None
                    reason = "revoked" if closed.code == _REVOKED else "closed"
                    _warn("hosted_agent_left", agent_id=self.agent_id, reason=reason)
                    return
        finally:
            await self.handler.close()

    async def _run(self, adapter: Adapter, token: HostedToken) -> Any:
        """Run the agent's connection until it ends; its channels load once it is ready.

        Returns:
            The SDK's close reason, or ``None`` after a failure (the caller retries).
        """
        self.handler.channels = ChannelCache(adapter.client, self._ctx.projects_root, None)
        self.handler.adapter = adapter
        self._adapter = adapter
        renewal = asyncio.get_running_loop().create_task(self._renew(adapter, token))
        try:
            return await adapter.run()
        except (ProblemError, httpx.TransportError) as err:
            _warn(
                "hosted_connection_failed",
                agent_id=self.agent_id,
                code=getattr(err, "code", "transport"),
            )
            # A failed discovery leaves the adapter's stores and client open; the next attempt
            # builds a new adapter.
            for closing in (adapter.inbox.close, adapter.outbox.store.close, adapter.client.aclose):
                with contextlib.suppress(Exception):
                    await closing()
            return None
        finally:
            self._adapter = None
            if self._issuing:
                # The server closes the old token's connection while it issues the next one
                # (26 3.3 item 1); the issue decides whether this was a renewal.
                with contextlib.suppress(asyncio.CancelledError, Exception):
                    await renewal
            renewal.cancel()
            with contextlib.suppress(asyncio.CancelledError, Exception):
                await renewal

    async def _valid_token(self) -> HostedToken | None:
        """The current token, or a new one when there is none or it is due for renewal."""
        token = self._token
        if token is not None and (
            token.refresh_at is None or self._ctx.clock.now() < token.refresh_at
        ):
            return token
        return await self._issue()

    async def _issue(self) -> HostedToken | None:
        """Fetch a token, retrying with the SDK's backoff until it works or the agent stops."""
        attempt = 0
        while not self._stopping:
            try:
                token = await issue_token(self._ctx.client, self.agent_id)
            except (ProblemError, httpx.TransportError) as err:
                _warn(
                    "hosted_token_issue_failed",
                    agent_id=self.agent_id,
                    code=getattr(err, "code", "transport"),
                )
                await self._ctx.clock.sleep(backoff(attempt, self._ctx.settings, self._rng))
                attempt += 1
                continue
            self._token = token
            _log("hosted_token_refreshed", agent_id=self.agent_id)
            return token
        return None

    async def _renew(self, adapter: Adapter, token: HostedToken) -> None:
        """At the token's renewal point fetch the next one, then reconnect with it."""
        if token.refresh_at is None:
            return
        clock = self._ctx.clock
        while clock.now() < token.refresh_at:
            await clock.sleep((token.refresh_at - clock.now()).total_seconds())
        self._issuing = True
        try:
            new = await self._issue()
        finally:
            self._issuing = False
        if new is not None:
            await adapter.stop()

    def _build(self, token: HostedToken) -> Adapter:
        ctx = self._ctx
        config = AdapterConfig(
            base_url=ctx.server_url(self.agent_id) + "/api/v1",
            instance_id=ctx.instance_id,
            agent_id=self.agent_id,
            credentials=Credentials(token.secret, ctx.dpop),
            consumer=HOSTED_CONSUMER,
            state_dir=hosted_dir(ctx.state_dir, self.agent_id),
            adapter_name=ADAPTER_NAME,
            adapter_version=__version__,
            runtime_name=RUNTIME_NAME,
            capabilities=ctx.capabilities,
            settings=ctx.settings,
        )
        return Adapter(config, self.handler, clock=ctx.clock, http_transport=ctx.http_transport)


class HostedAgents:
    """The set of hosted agents of one runner (26 3.2 item 1).

    Args:
        context: The runner's parts.
    """

    def __init__(self, context: HostingContext) -> None:
        """Create the empty set; `start` lists it."""
        self._ctx = context
        self._connections: dict[UUID, HostedConnection] = {}
        self._lock = asyncio.Lock()
        self._timer: asyncio.Task[None] | None = None
        self._syncs: set[asyncio.Task[None]] = set()

    @property
    def agent_ids(self) -> list[UUID]:
        """The agents served now."""
        return sorted(self._connections, key=str)

    async def start(self) -> None:
        """List the set and start the timer (once; later calls list again)."""
        if self._timer is None:
            self._timer = asyncio.get_running_loop().create_task(self._periodically())
        await self.sync("start")

    async def on_event(self, envelope: Envelope) -> bool:
        """List the set again on ``agent.hosting_changed``.

        Args:
            envelope: An event of the runner's own connection.

        Returns:
            True when the event was this one.
        """
        if envelope.type != HOSTING_CHANGED:
            return False
        await self.sync(envelope.type)
        return True

    async def sync_soon(self) -> None:
        """List the set again in the background (``agent.updated`` of a hosted connection).

        In the background because the listing may stop the very connection that delivered the
        event.
        """
        task = asyncio.get_running_loop().create_task(self.sync("agent.updated"))
        self._syncs.add(task)
        task.add_done_callback(self._syncs.discard)

    async def sync(self, reason: str) -> None:
        """List the hosted agents and start, keep or stop their connections.

        A failed list keeps the set and is logged (``hosted_list_failed``); the next event or
        the timer tries again.

        Args:
            reason: ``start``, the event type or ``timer``, for the log line.
        """
        async with self._lock:
            try:
                listed = await self._list()
            except (ProblemError, httpx.TransportError) as err:
                _warn("hosted_list_failed", cause=reason, code=getattr(err, "code", "transport"))
                return
            for agent_id, connection in list(self._connections.items()):
                if agent_id not in listed:
                    del self._connections[agent_id]
                    await connection.stop()
                    _log("hosted_agent_left", agent_id=agent_id, reason="unlisted")
                elif connection.done:
                    del self._connections[agent_id]  # dropped after 4012, listed again
            for agent_id, tool_class in listed.items():
                known = self._connections.get(agent_id)
                if known is not None:
                    await known.set_tool_class(tool_class)
                    continue
                joined = HostedConnection(agent_id, tool_class, self._ctx, self.sync_soon)
                self._connections[agent_id] = joined
                joined.start()
                _log("hosted_agent_joined", agent_id=agent_id)
            _log("hosted_agents_listed", cause=reason, count=len(listed))

    async def _list(self) -> dict[UUID, str]:
        """``listHostedAgents`` over all pages: the agents with their own tool class."""
        client = self._ctx.client
        found: dict[UUID, str] = {}
        cursor: str | Unset = UNSET
        while True:
            with client.unbound():
                page = await client.call_as(
                    AgentPage, list_hosted_agents.asyncio_detailed, cursor=cursor
                )
            for agent in page.items:
                found[agent.id] = str(agent.tool_class.value)
            if isinstance(page.next_cursor, Unset) or not page.next_cursor:
                return found
            cursor = page.next_cursor

    async def _periodically(self) -> None:
        """List the set and reload every agent's channels each ``channels_resync_interval``."""
        while True:
            await asyncio.sleep(self._ctx.resync_interval)
            await self.sync("timer")
            for connection in list(self._connections.values()):
                await connection.resync_channels()

    async def close(self) -> None:
        """Stop the timer and every hosted connection."""
        tasks = [t for t in (self._timer, *self._syncs) if t is not None]
        for task in tasks:
            task.cancel()
            with contextlib.suppress(asyncio.CancelledError, Exception):
                await task
        async with self._lock:
            for connection in list(self._connections.values()):
                await connection.stop()
            self._connections.clear()
