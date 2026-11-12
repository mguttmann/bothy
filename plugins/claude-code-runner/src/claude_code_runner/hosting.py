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

`HostedHandler` runs the agent's turns from its definition (26 5): it takes the thread and
session locks under the agent's state folder and a shared slot, fetches the current definition
with the hosted token (``getAgentDefinition`` as agent-self), checks it with the runner's own
copy of the rules (`claude_code_runner.definition`) against the class of the invocation token,
rebuilds the agent and control folders (`claude_code_runner.materialize`), and after the billing
pre-check and ``run.start`` writes the per-turn MCP file and runs the turn in the agent folder
with the command line of 26 5.3. A definition that fails the check, or one with a ``stdio``
server under a token class below ``host``, ends the run ``failed`` with ``tool_class_mismatch``
before ``run.start``; an agent without a definition ends ``adapter_error`` with the cause
``definition_missing`` (26 5.6). The folders are also written once the agent's connection is
ready and on ``agent.definition_changed``.

Hosted turns are not adopted after a crash of the runner (decisions.d/A1-05.md D-A1-05-9): the
next start stops the ones that still run and removes their records (`stop_leftover_turns`,
26 3.2 item 5).

Log lines name agent IDs and problem codes, never a token or content: ``hosted_agent_joined``,
``hosted_agent_left`` (with ``reason`` ``unlisted``, ``revoked`` or ``closed``),
``hosted_token_refreshed``, ``hosted_token_issue_failed`` (with ``code``),
``hosted_definition_refused`` (with the field codes) and ``hosted_definition_written``.

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
from typing import Any, NoReturn, Protocol
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
from bothy_sdk.errors import InvocationTokenInvalid, NotFoundError, ProblemError
from bothy_sdk.generated.api.agent_runtime import issue_hosted_agent_token, list_hosted_agents
from bothy_sdk.generated.api.agents import get_agent_definition
from bothy_sdk.generated.models import (
    AgentPage,
    AgentSession,
    ApiTokenCreated,
    RunContext,
    ToolClass,
)
from bothy_sdk.generated.types import UNSET, Unset
from bothy_sdk.retry import backoff
from bothy_sdk.runs import RunHandle
from bothy_sdk.settings import SdkSettings
from bothy_sdk.sources import Envelope

from claude_code_runner import ADAPTER_NAME, RUNTIME_NAME, __version__
from claude_code_runner.adoption import AdoptedGroup, process_matches
from claude_code_runner.attachments import fetch_attachments
from claude_code_runner.causes import TOOL_CLASS_MISMATCH, cause, definition_refused
from claude_code_runner.channel_skills import resolve_channel_skills
from claude_code_runner.channels import RESYNC_EVENTS, ChannelCache
from claude_code_runner.commandline import (
    ArgumentError,
    HostedArgs,
    budget_argument,
    choose_model,
    choose_session,
    reported_model,
)
from claude_code_runner.config import RunnerConfig, own_credential_files
from claude_code_runner.definition import Definition, Limits, intersect, parse
from claude_code_runner.heartbeat import WAIT_SLOTS_FULL, WAIT_THREAD_BUSY, Probe, runtime_status
from claude_code_runner.locks import LockTimeout, SlotPool, acquire, release
from claude_code_runner.materialize import (
    AgentFolders,
    Materialized,
    MaterializeError,
    definition_resolver,
    materialize,
    remove_turn_mcp,
    write_turn_mcp,
)
from claude_code_runner.reply_upload import remove_run_files
from claude_code_runner.runturn import RunState, RunTurn
from claude_code_runner.statedir import hosted_dir, sweep
from claude_code_runner.stop import stop_group
from claude_code_runner.toolclass import ORDER, lower
from claude_code_runner.transcribe import Transcriber
from claude_code_runner.turn import remove_turn_record

__all__ = [
    "DEFINITION_CHANGED",
    "HOSTED_CONSUMER",
    "HOSTING_CHANGED",
    "TOKEN_REFRESH_FRACTION",
    "HostedAgents",
    "HostedConnection",
    "HostedHandler",
    "HostedRuntime",
    "HostedToken",
    "HostingContext",
    "RunnerParts",
    "issue_token",
    "refresh_at",
    "stop_leftover_turns",
]

_LOG = logging.getLogger("claude_code_runner")

HOSTING_CHANGED = "agent.hosting_changed"
"""The durable event that changes the hosted set (26 9); its payload names one agent."""

DEFINITION_CHANGED = "agent.definition_changed"
"""The durable event of a new definition version (26 4.1 item 2): the folders are rewritten."""

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

_EXPIRED = 4011
"""The close code of an expired token (04 6): the agent fetches the next one at once."""


def _log(event: str, **ids: object) -> None:
    _LOG.info(json.dumps({"event": event, **{k: str(v) for k, v in ids.items()}}))


def _refuse(code: str, reason: str, ids: Mapping[str, Any]) -> NoReturn:
    """Log ``code`` and end the run without a start, naming the cause in the thread."""
    _log(code, **ids)
    raise RunRefused(reason, summary=cause(code))


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


async def stop_leftover_turns(
    state_dir: Path, *, stop_grace: float, kill_grace: float, poll: float, timeout: float
) -> None:
    """Stop the hosted turns an earlier runner left behind and remove their records.

    Hosted turns are not adopted (26 3.2 item 5). A runner that crashed leaves the turn of a
    hosted run running in its own process group with the record
    ``hosted/<agent_id>/turns/<run_id>.json``; nothing follows that turn or can stop it, and it
    holds the thread's lock. So at its start, before any connection of a hosted agent, the
    runner reads these records: a process that still matches its record is stopped as in
    09 18.6 without a finish, and the record goes once no process of its group is left. A
    process that does not match counts as gone (09 18.9): its record goes when the thread and
    session locks are free, which a turn that lives on holds through its inherited
    descriptors; with a busy lock the record stays for the next start (``adoption_lock_busy``).
    The server ends such a run as 09 5.5 says (``interrupted``, or ``stale`` while nothing
    serves the agent), as it does for a turn that died.

    Args:
        state_dir: ``runner.state_dir``.
        stop_grace: ``runner.stop_grace``.
        kill_grace: ``runner.kill_grace``.
        poll: ``runner.poll_interval``.
        timeout: ``runner.cli_probe_timeout``, the bound of the identity check.
    """
    hosted = state_dir / "hosted"
    for path in sorted(hosted.glob("*/turns/*.json")) if hosted.is_dir() else []:
        agent_dir = path.parent.parent
        try:
            record = json.loads(path.read_text())
            ids = {
                "agent_id": UUID(agent_dir.name),
                "run_id": UUID(record["run_id"]),
                "thread_id": UUID(record["thread_id"]),
            }
        except (OSError, ValueError, KeyError, TypeError):
            _warn("turn_record_unreadable", record=path.stem)
            continue
        alive = process_matches(record, timeout)
        held: list[int] = []
        if not alive:
            names = (f"thread-{ids['thread_id']}", f"session-{record.get('session_ref')}")
            try:
                for name in names:
                    held.append(await acquire(agent_dir / "locks" / f"{name}.lock", 0, poll))
            except LockTimeout:
                for fd in held:
                    release(fd)
                _log("adoption_lock_busy", **ids)
                continue
        try:
            _log("turn_adopted", action="stop" if alive else "leave", **ids)
            if alive:
                group = AdoptedGroup(record, timeout, verified=True)
                await stop_group(group, stop_grace, kill_grace, poll, ids)
            remove_turn_record(path)
        finally:
            for fd in reversed(held):
                release(fd)


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


class RunnerParts(Protocol):
    """What a hosted turn takes from the runner's own handler (`claude_code_runner.app`)."""

    @property
    def config(self) -> RunnerConfig:
        """The runner's configuration."""
        ...

    @property
    def claude_bin(self) -> str | None:
        """The CLI found in the cleaned ``PATH``."""
        ...

    @property
    def transcriber(self) -> Transcriber:
        """The transcriber of 09 18.14."""
        ...

    async def precheck(
        self,
        state: RunState,
        desired: str,
        ids: Mapping[str, Any],
        *,
        config: RunnerConfig | None = None,
    ) -> None:
        """The pre-check of 09 18.3 with the configuration of the turn."""
        ...

    async def runtime_too_old(self, required: str | None) -> None:
        """The API refused the model to the CLI as too old (09 18.9)."""
        ...


class HostedRuntime:
    """The runner's turn parts as one hosted agent sees them (`claude_code_runner.runturn`).

    The configuration is the runner's with the agent's state folder
    (``<runner.state_dir>/hosted/<agent_id>/``: turn records, sessions, streams, the CLI
    configuration and the scratch place of its runs) and the agent's loaded tool class.

    Args:
        runner: The runner's own handler.
        config: The runner's configuration for this agent.
    """

    def __init__(self, runner: RunnerParts, config: RunnerConfig) -> None:
        """Wrap the runner's parts."""
        self._runner = runner
        self._config = config

    @property
    def config(self) -> RunnerConfig:
        """The agent's view of the configuration."""
        return self._config

    @property
    def claude_bin(self) -> str | None:
        """The runner's CLI."""
        return self._runner.claude_bin

    @property
    def transcriber(self) -> Transcriber:
        """The runner's transcriber."""
        return self._runner.transcriber

    async def precheck(self, state: RunState, desired: str, ids: Mapping[str, Any]) -> None:
        """The runner's pre-check with the agent's configuration.

        Args:
            state: The run.
            desired: The token's ``billing_mode``.
            ids: IDs for log lines.
        """
        await self._runner.precheck(state, desired, ids, config=self._config)

    async def runtime_too_old(self, required: str | None) -> None:
        """Raise the runner's minimum CLI version (one CLI serves every agent).

        Args:
            required: The version the API named, if any.
        """
        await self._runner.runtime_too_old(required)


@dataclass(frozen=True)
class _Loaded:
    """A checked definition with its version, as fetched for one turn or one rebuild."""

    definition: Definition
    version: int


class _Refused(Exception):  # noqa: N818 - a refusal of a definition, not an error of the runner
    """The definition fails the runner's own check; ``codes`` are its field codes."""

    def __init__(self, codes: list[str]) -> None:
        super().__init__("definition refused")
        self.codes = codes


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
        runner: The runner's own handler: configuration, CLI, transcriber and pre-check.
        root: ``runner.hosted_agents_root``.
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
    runner: RunnerParts | None = None
    root: Path | None = None


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
        self._state_dir = hosted_dir(context.state_dir, agent_id)
        self._locks = self._state_dir / "locks"
        self._folders = AgentFolders(context.root, agent_id) if context.root else None
        self._servers: dict[UUID, Materialized] = {}

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

        The agent's folders are written from its current definition then (26 5.2). A failed
        load is logged and tried again by the next event or the timer.
        """
        if self.adapter is not None:
            await self.adapter.send_heartbeat()
        await self.resync_channels("ready")
        await self.refresh_definition("ready")

    async def on_event(self, envelope: Envelope) -> None:
        """Reload the agent's channels on their events; list the set again on ``agent.updated``.

        ``agent.definition_changed`` rewrites the agent's folders (26 5.2).

        Args:
            envelope: The event.
        """
        cache = self.channels
        if envelope.type == DEFINITION_CHANGED:
            await self.refresh_definition(envelope.type)
        elif envelope.type == "agent.updated":
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

    # ---- the definition (26 5.2, 5.6) ----------------------------------------------------

    def _config(self) -> RunnerConfig:
        """The runner's configuration as this agent's turns see it (`HostedRuntime`)."""
        runner = self._ctx.runner
        if runner is None:  # pragma: no cover - the runner always passes its handler
            raise RuntimeError("no runner parts")
        return dataclasses.replace(
            runner.config, state_dir=self._state_dir, tool_class=self._tool_class()
        )

    async def _fetch(self, tool_class: str) -> _Loaded:
        """Fetch the current definition with the hosted token and check it (26 5.6 item 1).

        Args:
            tool_class: The class the check runs for: the invocation token's before a turn.

        Returns:
            The checked definition.

        Raises:
            NotFoundError: The agent has no definition.
            ProblemError: The server refused otherwise.
            httpx.TransportError: The server was not reachable.
            _Refused: The definition fails the check.
        """
        adapter = self.adapter
        if adapter is None:  # pragma: no cover - a run comes only over the connection
            raise RuntimeError("no connection")
        with adapter.client.unbound():
            answer = await adapter.client.call_detailed(
                get_agent_definition.asyncio_detailed, agent_id=self.agent_id
            )
        try:
            body = json.loads(answer.content)
            raw = json.dumps(body["definition"]).encode("utf-8")
            version = int(body["version"])
        except (ValueError, KeyError, TypeError):
            raise _Refused(["invalid_type"]) from None
        config = self._config()
        limits = Limits.runner(config.mcp_stdio_runners)
        definition, found = parse(raw, limits, tool_class)
        if definition is None:
            raise _Refused([f.code for f in found])
        return _Loaded(definition, version)

    def _write(self, loaded: _Loaded) -> Materialized:
        """Rebuild the agent's folders for the loaded class (26 5.6 items 2 and 3)."""
        if self._folders is None:  # pragma: no cover - load_config requires the root
            raise MaterializeError("not_a_folder")
        done = materialize(
            self._folders,
            loaded.definition,
            loaded.version,
            self._tool_class(),
            active_runs=set(self._runs),
        )
        _log("hosted_definition_written", agent_id=self.agent_id, version=loaded.version)
        return done

    async def refresh_definition(self, reason: str) -> None:
        """Write the agent's folders from its current definition (start and event, 26 5.2).

        A missing or refused definition and every failure are logged and leave the folders as
        they are; the next turn fetches and checks again.

        Args:
            reason: ``ready`` or the event type, for the log line.
        """
        if self._folders is None or self._ctx.runner is None:
            return
        try:
            loaded = await self._fetch(self._tool_class())
            self._write(loaded)
        except NotFoundError:
            _log("definition_missing", agent_id=self.agent_id, cause=reason)
        except _Refused as refused:
            _warn(
                "hosted_definition_refused",
                agent_id=self.agent_id,
                cause=reason,
                codes=",".join(sorted(set(refused.codes))),
            )
        except (ProblemError, httpx.TransportError) as err:
            _warn(
                "hosted_definition_unavailable",
                agent_id=self.agent_id,
                cause=reason,
                code=getattr(err, "code", "transport"),
            )
        except (MaterializeError, OSError) as err:
            _warn(
                "hosted_definition_unwritable",
                agent_id=self.agent_id,
                cause=reason,
                code=getattr(err, "code", type(err).__name__),
            )

    # ---- a turn -------------------------------------------------------------------------

    async def prepare(self, inv: Invocation, run: RunHandle) -> StartOptions:
        """The steps of a hosted turn before ``run.start`` (brief A1-05 design note 2).

        Locks and a shared slot (26 3.2 item 3); the current definition, checked against the
        class of the invocation token and written to the agent's folders; the session, model,
        tools and channel skills; the run context, attachments and the billing pre-check.

        Args:
            inv: The invocation.
            run: The queued run.

        Returns:
            ``model``, ``external_session_ref`` and ``billing_evidence`` of ``startRun``.

        Raises:
            RunRefused: The run ends without a start: ``tool_class_mismatch`` for a definition
                that fails the check, ``adapter_error`` with ``definition_missing`` and the
                other causes of the runner, or the reasons of the pre-check.
        """
        payload = inv.payload
        state = RunState(run.run_id, payload.thread_id, payload.channel_id)
        self._runs[run.run_id] = state
        ids = {"run_id": run.run_id, "agent_id": self.agent_id, "thread_id": state.thread_id}
        if str(payload.trigger_kind) != "message":
            _refuse("trigger_kind_unsupported", "adapter_error", ids)
        session = payload.agent_session
        state.agent_session = session if isinstance(session, AgentSession) else None
        mode = getattr(payload.session_mode, "value", None)
        started = state.agent_session is not None and state.agent_session.started is True
        state.session_mode = str(mode) if mode else ("resume" if started else "new")
        try:
            state.session = choose_session(state.agent_session)
        except ArgumentError as err:
            _refuse(err.code, "adapter_error", ids)
        await self._take_locks(state, run.run_id, ids)
        if self._ctx.probe().blocking:
            _refuse("runtime_unavailable", "runtime_error", ids)
        loaded = await self._definition_for(inv, ids)
        try:
            done = self._write(loaded)
        except (MaterializeError, OSError) as err:
            code = getattr(err, "code", type(err).__name__)
            _warn("hosted_definition_unwritable", code=code, **ids)
            raise RunRefused("adapter_error", summary=cause("definition_unwritable")) from None
        self._servers[run.run_id] = done
        self._turn_parts(state, loaded, done, ids)
        state.context = await self._context(run, ids)
        files_dir = self._state_dir / "files" / str(state.thread_id)
        state.file_texts, state.stored_files = await fetch_attachments(
            run.client, state.context.trigger_message.attachments, files_dir
        )
        await self._runtime().precheck(state, inv.claims.billing_mode, ids)
        _log("run_prepared", **ids)
        return StartOptions(
            model=reported_model(state.model),
            external_session_ref=state.session.ref,
            billing_evidence=state.evidence,
        )

    def _runtime(self) -> HostedRuntime:
        runner = self._ctx.runner
        if runner is None:  # pragma: no cover - the runner always passes its handler
            raise RuntimeError("no runner parts")
        return HostedRuntime(runner, self._config())

    async def _take_locks(self, state: RunState, run_id: UUID, ids: Mapping[str, Any]) -> None:
        """The thread lock, the session lock and a shared slot; each wait is reported."""
        try:
            try:
                for name in (f"thread-{state.thread_id}", f"session-{state.session.ref}"):  # type: ignore[union-attr]
                    state.locks.append(
                        await acquire(
                            self._locks / f"{name}.lock",
                            self._ctx.session_lock_wait,
                            self._ctx.poll_interval,
                            on_busy=lambda: self._wait(run_id, WAIT_THREAD_BUSY),
                        )
                    )
            except LockTimeout:
                _refuse("session_busy", "adapter_error", ids)
            slots = self._ctx.slots
            if slots.full:
                self._wait(run_id, WAIT_SLOTS_FULL)
            await slots.acquire()
            state.slot = slots
        finally:
            self._wait(run_id, None)

    async def _definition_for(self, inv: Invocation, ids: Mapping[str, Any]) -> _Loaded:
        """The definition of the turn, checked against the token's class (26 5.6 item 1)."""
        try:
            return await self._fetch(str(inv.claims.tool_class))
        except NotFoundError:
            _refuse("definition_missing", "adapter_error", ids)
        except _Refused as refused:
            codes = list(dict.fromkeys(refused.codes))
            _warn("hosted_definition_refused", codes=",".join(sorted(codes)), **ids)
            raise RunRefused(TOOL_CLASS_MISMATCH, summary=definition_refused(codes)) from None
        except (ProblemError, httpx.TransportError) as err:
            _warn("hosted_definition_unavailable", code=getattr(err, "code", "transport"), **ids)
            raise RunRefused("adapter_error", summary=cause("definition_unavailable")) from None

    def _turn_parts(
        self, state: RunState, loaded: _Loaded, done: Materialized, ids: Mapping[str, Any]
    ) -> None:
        """Model, tools, folder, command line parts and channel skills of the turn (26 5.3)."""
        definition = loaded.definition
        config = self._config()
        folders = self._folders
        if folders is None:  # pragma: no cover - load_config requires the root
            raise RuntimeError("no root")
        settings = definition.settings
        try:
            state.model = choose_model(settings.model, config.default_model)
        except ArgumentError as err:
            _refuse(err.code, "adapter_error", ids)
        state.tools = intersect(config.scoped_allowed_tools, settings.allowed_tools)
        state.cwd = folders.agent
        state.hosted = HostedArgs(
            settings=folders.settings,
            plugin=folders.plugin if done.has_skills else None,
            mcp_config=folders.turn_mcp(state.run_id) if done.servers else None,
            has_skills=done.has_skills,
        )
        cap = settings.max_cost_usd_per_run
        state.cost_cap = cap
        state.max_budget_usd = None if cap is None else budget_argument(cap)
        channel = self.channels.get(state.channel_id) if self.channels else None
        state.channel_prompt = channel.behavior.prompt if channel is not None else ""
        if config.channel_skills_dir is not None and channel is not None:
            found = resolve_channel_skills(
                list(channel.behavior.skills), config.channel_skills_dir, config.tool_class, **ids
            )
            state.plugin_dirs, state.notice = found.plugin_dirs, found.notice()

    async def _context(self, run: RunHandle, ids: Mapping[str, Any]) -> RunContext:
        try:
            return await run.load_context()
        except NotFoundError:
            _log("run_inactive", **ids)
            raise RunRefused(None) from None
        except InvocationTokenInvalid as err:
            if err.reason == "run_inactive":
                _log("run_inactive", **ids)
                raise RunRefused(None) from None
            raise

    async def on_not_started(self, inv: Invocation) -> None:
        """Give back the slot and the locks of a run that ended in prepare.

        Args:
            inv: The invocation.
        """
        run_id = inv.claims.run_id
        state = self._runs.pop(run_id, None) if run_id else None
        if run_id:
            self._servers.pop(run_id, None)
        if state is not None:
            state.close()
            remove_run_files(self._state_dir, state.run_id)

    def _resolver(self) -> Callable[[str], str]:
        """The resolver of this agent's ``secret_ref`` values (26 5.1 item 3)."""
        runner = self._ctx.runner
        if runner is None:
            return definition_resolver(None, self.agent_id, ())
        config = runner.config
        refused = own_credential_files(config)
        return definition_resolver(config.hosted_secrets_dir, self.agent_id, refused)

    async def execute(self, inv: Invocation, run: RunHandle) -> None:
        """Write the per-turn MCP file, run the turn in the agent folder, then delete the file.

        The file is deleted in a ``finally``, also when the turn is stopped or the runner's
        stop cancels it (26 5.2); a crash leaves it to the sweep at the next start.

        Args:
            inv: The invocation.
            run: The started run.
        """
        del inv
        state = self._runs[run.run_id]
        done = self._servers.get(run.run_id)
        folders = self._folders
        try:
            hosted = state.hosted
            if folders is not None and done is not None and hosted and hosted.mcp_config:
                try:
                    write_turn_mcp(folders, run.run_id, done.servers, self._resolver())
                except (MaterializeError, OSError) as err:
                    code = getattr(err, "code", type(err).__name__)
                    _warn("mcp_reference_unavailable", run_id=run.run_id, code=code)
                    await run.finish(
                        "failed", reason="adapter_error", summary=cause("mcp_reference_unavailable")
                    )
                    return
            await RunTurn(self._runtime(), state, run).run()
        finally:
            if folders is not None:
                remove_turn_mcp(folders, run.run_id)
            self._runs.pop(run.run_id, None)
            self._servers.pop(run.run_id, None)
            state.close()
            with contextlib.suppress(OSError):
                active = {str(r) for r in self._runs}
                runner = self._ctx.runner
                retention = runner.config.stream_log_retention if runner else 0.0
                if runner is not None:
                    sweep(self._state_dir, retention, active=active)

    async def on_followup(self, inv: Invocation, run: RunHandle) -> None:
        """A follow-up of a hosted run: held until its turn starts, or offered (09 18.5).

        Args:
            inv: The ``agent.followup`` invocation.
            run: The run.
        """
        state = self._runs.get(run.run_id)
        message_id = getattr(inv.payload, "message_id", "")
        if state is None:
            _log("followup_not_deliverable", run_id=run.run_id, message_id=message_id)
            return
        if state.turn is None:
            if all(getattr(h.payload, "message_id", None) != message_id for h in state.held):
                state.held.append(inv)
            return
        await state.turn.offer_followup(inv)

    async def on_cancel(self, run: RunHandle, reason: str) -> None:
        """Stop the turn of a started hosted run (09 18.6).

        Args:
            run: The run.
            reason: The request's reason, unchanged.
        """
        state = self._runs.get(run.run_id)
        if state is None:
            return
        if state.turn is None:
            state.stop_reason = state.stop_reason or reason
            return
        await state.turn.request_stop(reason)

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
                if self._token is token and closed.code == _EXPIRED:
                    # The token expired before its renewal point: fetch the next one now.
                    self._token = None
                    _warn("hosted_token_expired", agent_id=self.agent_id)
                    continue
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
        self._sync_in_background(envelope.type)
        return True

    async def sync_soon(self) -> None:
        """List the set again in the background (``agent.updated`` of a hosted connection).

        In the background because the listing may stop the very connection that delivered the
        event.
        """
        self._sync_in_background("agent.updated")

    def _sync_in_background(self, reason: str) -> None:
        """Run `sync` as a tracked task, so a slow list never holds up the delivering events."""
        task = asyncio.get_running_loop().create_task(
            self._guarded(lambda: self.sync(reason), reason)
        )
        self._syncs.add(task)
        task.add_done_callback(self._syncs.discard)

    @staticmethod
    async def _guarded(step: Callable[[], Awaitable[None]], reason: str) -> None:
        """Run one step of the set; an unexpected error is logged and the next try still runs."""
        try:
            await step()
        except Exception as err:  # the timer and the next event must survive any one failure
            _warn("hosted_sync_failed", cause=reason, error=type(err).__name__)

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
                tool_class = str(agent.tool_class.value)
                if tool_class not in ORDER:  # a class of a newer server: its order is unknown
                    _warn("hosted_agent_skipped", agent_id=agent.id, code="unknown_tool_class")
                    continue
                found[agent.id] = tool_class
            if isinstance(page.next_cursor, Unset) or not page.next_cursor:
                return found
            cursor = page.next_cursor

    async def _periodically(self) -> None:
        """List the set and reload every agent's channels each ``channels_resync_interval``."""
        while True:
            await asyncio.sleep(self._ctx.resync_interval)
            await self._guarded(lambda: self.sync("timer"), "timer")
            for connection in list(self._connections.values()):
                await self._guarded(connection.resync_channels, "timer")

    async def close(self) -> None:
        """Stop the timer and every hosted connection."""
        tasks = [t for t in (self._timer, *self._syncs) if t is not None]
        for task in tasks:
            task.cancel()
            with contextlib.suppress(asyncio.CancelledError, Exception):
                await task
        async with self._lock:
            # Together: every stop waits until the turns of its agent are stopped (09 18.6),
            # and one after the other these waits would add up over the agents.
            await asyncio.gather(*(c.stop() for c in self._connections.values()))
            self._connections.clear()
