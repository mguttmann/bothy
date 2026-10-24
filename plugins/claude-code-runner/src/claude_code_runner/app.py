# SPDX-License-Identifier: Apache-2.0
"""The runner: a handler of the SDK's run loop plus its start and stop (09 18, notes 3 to 16).

The SDK (`bothy_sdk.adapter.Adapter`) checks every invocation token, stores accepted events in
its durable inbox, acknowledges them, renews tokens, calls ``startRun`` and throttles patches.
`RunnerHandler` answers its questions: the actual tool class is ``runner.tool_class``; the
actual mode at acceptance is the desired one when the runner may run it, else ``unknown``
(note 4). ``prepare`` runs the steps of note 5 in order and ends a run without a start through
`bothy_sdk.adapter.RunRefused`; ``execute`` starts exactly one ``claude -p`` turn after ``200``
on ``startRun`` and ends the run itself. Every path releases the slot and both locks.

Examples:
    >>> RunnerHandler.__mro__[1].__name__
    'AgentHandler'
"""

from __future__ import annotations

import asyncio
import contextlib
import dataclasses
import json
import logging
import os
import sys
from collections.abc import Mapping
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
from bothy_sdk.auth import Credentials, FileDpopKey, Secret
from bothy_sdk.client import Client
from bothy_sdk.clock import Clock
from bothy_sdk.errors import InvocationTokenInvalid, NotFoundError, ProblemError
from bothy_sdk.gateway import CloseReason
from bothy_sdk.generated.api.me import get_me
from bothy_sdk.generated.api.runs import get_run
from bothy_sdk.generated.models import AgentSession, Me, Run, RunContext, ToolClass
from bothy_sdk.generated.types import Unset
from bothy_sdk.invocation import stored_claims
from bothy_sdk.runs import RunHandle
from bothy_sdk.sources import Envelope

from claude_code_runner import ADAPTER_NAME, RUNTIME_NAME, __version__
from claude_code_runner.adoption import (
    AdoptedGroup,
    AdoptionAction,
    adoption_decision,
    process_matches,
)
from claude_code_runner.attachments import fetch_attachments
from claude_code_runner.authcheck import auth_status, decide, evidence
from claude_code_runner.channels import ChannelCache
from claude_code_runner.commandline import (
    ArgumentError,
    SessionChoice,
    choose_model,
    choose_session,
    scoped_tools,
)
from claude_code_runner.config import RunnerConfig
from claude_code_runner.environment import (
    RunEnvironmentError,
    build_environment,
    clean_path,
    find_program,
)
from claude_code_runner.heartbeat import (
    CAPABILITIES,
    Probe,
    billing_modes_supported,
    probe_version,
    runtime_status,
)
from claude_code_runner.locks import LockTimeout, acquire, release
from claude_code_runner.runturn import RunState, RunTurn
from claude_code_runner.secrets import SecretError, resolve
from claude_code_runner.statedir import ensure_private_dir, sweep
from claude_code_runner.stop import stop_group
from claude_code_runner.turn import remove_turn_record
from claude_code_runner.workspace import WorkspaceError, readiness
from claude_code_runner.workspace import resolve as resolve_workspace

__all__ = ["Runner", "RunnerHandler", "StartError", "log", "warn"]

_LOG = logging.getLogger("claude_code_runner")


def log(event: str, **ids: object) -> None:
    """Write one JSON log line: an event code and IDs, never content, tokens or keys.

    Args:
        event: The event code.
        **ids: Run, thread and channel IDs or codes.
    """
    _LOG.info(json.dumps({"event": event, **{k: str(v) for k, v in ids.items()}}))


def warn(event: str, **ids: object) -> None:
    """Like `log`, at level warning.

    Args:
        event: The event code.
        **ids: Run, thread and channel IDs or codes.
    """
    _LOG.warning(json.dumps({"event": event, **{k: str(v) for k, v in ids.items()}}))


class StartError(Exception):
    """The runner cannot start; the message is a code, never a secret."""


class RunnerHandler(AgentHandler):
    """The runtime side of the runner for the SDK's run loop.

    Args:
        config: The runner configuration.
        environ: The runner's own environment (the base of every run environment).
    """

    def __init__(self, config: RunnerConfig, environ: Mapping[str, str]) -> None:
        """Create the handler; `Runner` wires the adapter and the channel cache."""
        self.config = config
        self.environ = dict(environ)
        self.path_entries = clean_path(self.environ.get("PATH", ""))
        self.claude_bin = find_program(config.claude_bin, self.path_entries)
        self.adapter: Adapter | None = None
        self.channels: ChannelCache | None = None
        self.probe = Probe()
        self._probing: asyncio.Task[Probe] | None = None
        self._runs: dict[UUID, RunState] = {}
        self.adopted: dict[UUID, RunTurn] = {}
        self._adopting: set[asyncio.Task[None]] = set()
        self._slots = asyncio.Semaphore(config.max_parallel_runs)
        self._modes = billing_modes_supported(
            has_key_ref=config.anthropic_api_key_ref is not None,
            allow_personal=config.allow_personal,
        )

    # ---- answers to the SDK ---------------------------------------------------------------

    def actual_tool_class(self) -> ToolClass:
        """The configured class: ``reported_tool_class`` and ``tool_class_used`` (AG-30)."""
        return ToolClass(self.config.tool_class)

    def actual_billing(self, inv: Invocation) -> ActualBilling:
        """The mode the runner can run: after prepare the proven one, before it the desired one.

        Args:
            inv: The invocation.

        Returns:
            ``organization`` (always provable before the start), ``personal`` only with
            ``runner.allow_personal``, the mode of the pre-check after prepare, else
            ``unknown``.
        """
        state = self._runs.get(inv.claims.run_id) if inv.claims.run_id else None
        if state is not None and state.evidence is not None:
            return ActualBilling(state.mode, state.evidence)
        desired = inv.claims.billing_mode
        if desired == "organization" or (desired == "personal" and self.config.allow_personal):
            return ActualBilling(desired)
        return ActualBilling("unknown")

    def runtime_status(self) -> RuntimeStatus:
        """The run-time heartbeat members; a new version probe starts in the background."""
        self._probe_soon()
        cache = self.channels
        status = runtime_status(self.probe, cache.readiness() if cache else [], self._modes)
        return dataclasses.replace(status, active_runs=sorted(self.adopted, key=str))

    async def on_ready(self) -> None:
        """Send the first heartbeat as soon as the connection is ready (note 3)."""
        if self.adapter is not None:
            await self.adapter.send_heartbeat()

    async def on_event(self, envelope: Envelope) -> None:
        """``agent.config_changed`` reloads the forum and reports a changed readiness at once.

        Args:
            envelope: The event.
        """
        if envelope.type == "run.cancel_requested":
            await self._cancel_adopted(envelope)
            return
        if envelope.type != "agent.config_changed" or self.channels is None:
            return
        channel_id = getattr(getattr(envelope.event, "payload", None), "channel_id", None)
        if not isinstance(channel_id, UUID):
            return
        if await self.channels.reload(channel_id) and self.adapter is not None:
            await self.adapter.send_heartbeat()

    async def on_followup(self, inv: Invocation, run: RunHandle) -> None:
        """A follow-up of a run of this runner: held until its turn starts, or offered (18.5).

        Args:
            inv: The ``agent.followup`` invocation.
            run: The run.
        """
        state = self._runs.get(run.run_id)
        message_id = getattr(inv.payload, "message_id", "")
        if state is None:
            log("followup_not_deliverable", run_id=run.run_id, message_id=message_id)
            return
        if state.turn is None:
            if all(getattr(h.payload, "message_id", None) != message_id for h in state.held):
                state.held.append(inv)
            log("followup_held", run_id=run.run_id, message_id=message_id)
            return
        await state.turn.offer_followup(inv)

    async def on_cancel(self, run: RunHandle, reason: str) -> None:
        """Stop the turn of a started run; it finishes ``cancelled`` once its group is gone.

        Args:
            run: The run.
            reason: The request's reason, unchanged.
        """
        state = self._runs.get(run.run_id)
        if state is None:
            log("cancel_not_deliverable", run_id=run.run_id)
            return
        if state.turn is None:
            state.stop_reason = state.stop_reason or reason
            return
        await state.turn.request_stop(reason)

    async def on_interaction(self, inv: Invocation, run: RunHandle) -> None:
        """The runner builds no components: log by type only."""
        log("event_ignored", type=inv.type, run_id=run.run_id)

    async def on_command(self, inv: Invocation, run: RunHandle | None) -> None:
        """The runner registers no commands: log by type only."""
        log("event_ignored", type=inv.type)

    # ---- adoption after a restart (design note 12) ----------------------------------------

    async def adopt_turns(self, adapter: Adapter) -> None:
        """Take over or end the turns of the turn records an earlier runner left behind.

        Args:
            adapter: The built adapter (its client, inbox and outbox).
        """
        turns = self.config.state_dir / "turns"
        if not turns.is_dir():
            return
        for path in sorted(turns.glob("*.json")):
            try:
                record = json.loads(path.read_text())
                run_id, thread_id = UUID(record["run_id"]), UUID(record["thread_id"])
            except (OSError, ValueError, KeyError, TypeError):
                warn("turn_record_unreadable", record=path.stem)
                continue
            await self._adopt(adapter, path, record, run_id, thread_id)

    async def _adopt(
        self, adapter: Adapter, path: Path, record: dict[str, Any], run_id: UUID, thread_id: UUID
    ) -> None:
        ids = {"run_id": run_id, "thread_id": thread_id}
        timeout = self.config.cli_probe_timeout
        alive = process_matches(record, timeout)
        held: list[int] = []
        if not alive:
            # A gone turn holds no lock any more: both locks with one attempt (D-M2-35-11).
            locks = self.config.state_dir / "locks"
            try:
                for name in (f"thread-{thread_id}", f"session-{record.get('session_ref')}"):
                    held.append(await acquire(locks / f"{name}.lock", 0, self.config.poll_interval))
            except LockTimeout:
                for fd in held:
                    release(fd)
                log("adoption_lock_busy", **ids)
                return
        try:
            await self._adopt_record(adapter, path, record, run_id, thread_id, alive, ids)
        finally:
            for fd in reversed(held):
                release(fd)

    async def _adopt_record(  # noqa: PLR0913, PLR0917 - the parts of one record
        self,
        adapter: Adapter,
        path: Path,
        record: dict[str, Any],
        run_id: UUID,
        thread_id: UUID,
        alive: bool,
        ids: Mapping[str, Any],
    ) -> None:
        token = await _newest_token(adapter, run_id)
        try:
            with adapter.client.unbound():
                run = await adapter.client.call_as(Run, get_run.asyncio_detailed, run_id=run_id)
        except (ProblemError, httpx.TransportError) as err:
            log("adoption_run_unreadable", code=getattr(err, "code", "transport_error"), **ids)
            run = None
        run_state = str(run.state.value) if run is not None else "unknown"
        in_log = _result_after(Path(str(record.get("stream", ""))), int(record.get("offset") or 0))
        action = adoption_decision(alive=alive, run_state=run_state, result_in_log=in_log)
        if action in (AdoptionAction.ADOPT, AdoptionAction.FINISH_FROM_LOG) and (
            token is None or run is None
        ):
            action = AdoptionAction.STOP if alive else AdoptionAction.LEAVE
        log("turn_adopted", action=action.value, **ids)
        group = AdoptedGroup(record, self.config.cli_probe_timeout, verified=alive)
        if action is AdoptionAction.LEAVE:
            remove_turn_record(path)
            return
        if action is AdoptionAction.STOP:
            await stop_group(
                group,
                self.config.stop_grace,
                self.config.kill_grace,
                self.config.poll_interval,
                ids,
            )
            remove_turn_record(path)
            return
        if token is None or run is None:  # pragma: no cover - handled above
            return
        handle = RunHandle(
            adapter.client, run_id, thread_id, outbox=adapter.outbox, settings=self.config.sdk
        )
        handle.started(run, stored_claims(token).jti)
        state = RunState(run_id, thread_id, run.channel_id)
        state.session = SessionChoice(str(record.get("session_ref")), resume=True, new=False)
        state.session_mode = str(record.get("session_mode") or "resume")
        turn = RunTurn(self, state, handle)
        known = {
            key: str(step.status.value) for key, step in run.steps.additional_properties.items()
        }
        if action is AdoptionAction.FINISH_FROM_LOG:
            with adapter.client.bind_run(run_id, token):
                await turn.adopted(record, group, known)
            return
        self.adopted[run_id] = turn
        task = asyncio.ensure_future(
            self._follow_adopted(adapter, turn, record, group, token, known)
        )
        self._adopting.add(task)
        task.add_done_callback(self._adopting.discard)

    async def _follow_adopted(  # noqa: PLR0913, PLR0917 - the parts of one adopted turn
        self,
        adapter: Adapter,
        turn: RunTurn,
        record: dict[str, Any],
        group: AdoptedGroup,
        token: Secret,
        known: Mapping[str, str],
    ) -> None:
        run_id = turn.handle.run_id
        try:
            with adapter.client.bind_run(run_id, token):
                await turn.adopted(record, group, known)
        finally:
            self.adopted.pop(run_id, None)
            adapter.client.release_run(run_id)

    async def _cancel_adopted(self, envelope: Envelope) -> None:
        payload = getattr(envelope.event, "payload", None)
        run_id = getattr(payload, "run_id", None)
        turn = self.adopted.get(run_id) if isinstance(run_id, UUID) else None
        if turn is None:
            return
        raw = getattr(payload, "reason", None)
        await turn.request_stop(str(getattr(raw, "value", raw) or "user_cancelled"))

    # ---- version probe --------------------------------------------------------------------

    def _probe_env(self) -> dict[str, str]:
        try:
            return build_environment(
                self.environ,
                mode="personal",
                api_key=None,
                config_dir=None,
                forward_ssh_agent=False,
            )
        except RunEnvironmentError:
            return {"PATH": os.pathsep.join(self.path_entries)}

    async def refresh_probe(self) -> Probe:
        """Probe ``claude --version`` now and keep the outcome.

        Returns:
            The probe.
        """
        self.probe = await probe_version(
            self.claude_bin, self._probe_env(), self.config.state_dir, self.config.cli_probe_timeout
        )
        if self.probe.reasons:
            warn("runtime_degraded", reasons=",".join(self.probe.reasons))
        return self.probe

    async def close(self) -> None:
        """Cancel a running version probe and the follow of adopted turns (they keep running)."""
        for follow in list(self._adopting):
            follow.cancel()
            with contextlib.suppress(asyncio.CancelledError, Exception):
                await follow
        task = self._probing
        if task is not None and not task.done():
            task.cancel()
            with contextlib.suppress(asyncio.CancelledError):
                await task

    def _probe_soon(self) -> None:
        if self._probing is not None and not self._probing.done():
            return
        with contextlib.suppress(RuntimeError):
            self._probing = asyncio.get_running_loop().create_task(self.refresh_probe())

    # ---- prepare --------------------------------------------------------------------------

    async def prepare(self, inv: Invocation, run: RunHandle) -> StartOptions:
        """Steps a to i of design note 5; a non-start raises `RunRefused` with its reason.

        Args:
            inv: The invocation (payload ``AgentInvocationPayload``).
            run: The queued run.

        Returns:
            ``model``, ``external_session_ref`` and ``billing_evidence`` of ``startRun``.

        Raises:
            RunRefused: The run must end without a start.
        """
        payload = inv.payload
        state = RunState(run.run_id, payload.thread_id, payload.channel_id)
        self._runs[run.run_id] = state
        ids = {"run_id": run.run_id, "thread_id": state.thread_id, "channel_id": state.channel_id}
        if str(payload.trigger_kind) != "message":
            log("trigger_kind_unsupported", **ids)
            raise RunRefused("adapter_error")
        binding = await self._binding(payload)
        if binding is None:
            log("project_not_bound", **ids)
            raise RunRefused("adapter_error")
        if self.probe.blocking:
            log("runtime_unavailable", **ids)
            raise RunRefused("runtime_error")
        try:
            state.cwd = resolve_workspace(self.config.projects_root, binding.workspace_ref)
        except WorkspaceError as err:
            log(err.code, **ids)
            raise RunRefused("adapter_error") from None
        ready, code = readiness(self.config.projects_root, binding.workspace_ref)
        if not ready:
            log(code or "workspace_missing", **ids)
            raise RunRefused("adapter_error")
        session = payload.agent_session
        state.agent_session = session if isinstance(session, AgentSession) else None
        mode = getattr(payload.session_mode, "value", None)
        started = state.agent_session is not None and state.agent_session.started is True
        state.session_mode = str(mode) if mode else ("resume" if started else "new")
        runner_config = binding.runner_config
        try:
            state.session = choose_session(state.agent_session)
            model = runner_config.model if isinstance(runner_config.model, str) else None
            state.model = choose_model(model, self.config.default_model)
            allowed = runner_config.allowed_tools
            binding_tools = list(allowed) if not isinstance(allowed, Unset) else None
            state.tools = scoped_tools(self.config.scoped_allowed_tools, binding_tools)
        except ArgumentError as err:
            log(err.code, **ids)
            raise RunRefused("adapter_error") from None
        config = self.channels.get(state.channel_id) if self.channels else None
        state.channel_prompt = config.behavior.prompt if config is not None else ""
        await self._take_locks(state, ids)
        state.context = await self._context(run, ids)
        files_dir = self.config.state_dir / "files" / str(state.thread_id)
        state.file_texts, state.stored_files = await fetch_attachments(
            run.client, state.context.trigger_message.attachments, files_dir
        )
        await self.precheck(state, inv.claims.billing_mode, ids)
        log("run_prepared", **ids)
        return StartOptions(
            model=state.model,
            external_session_ref=state.session.ref,
            billing_evidence=state.evidence,
        )

    async def _binding(self, payload: Any) -> Any:
        cache = self.channels
        if cache is None:
            return None
        cached = cache.settings_version(payload.channel_id)
        wanted = payload.settings_version
        if cached is None or (isinstance(wanted, int) and wanted > cached):
            await cache.reload(payload.channel_id)
        return cache.binding(payload.channel_id)

    async def _take_locks(self, state: RunState, ids: Mapping[str, Any]) -> None:
        locks = self.config.state_dir / "locks"
        try:
            for name in (f"thread-{state.thread_id}", f"session-{state.session.ref}"):  # type: ignore[union-attr]
                state.locks.append(
                    await acquire(
                        locks / f"{name}.lock",
                        self.config.session_lock_wait,
                        self.config.poll_interval,
                    )
                )
        except LockTimeout:
            log("session_busy", **ids)
            raise RunRefused("adapter_error") from None
        await self._slots.acquire()
        state.slot = self._slots

    async def _context(self, run: RunHandle, ids: Mapping[str, Any]) -> RunContext:
        try:
            return await run.load_context()
        except NotFoundError:
            log("run_inactive", **ids)
            raise RunRefused(None) from None
        except InvocationTokenInvalid as err:
            if err.reason == "run_inactive":
                log("run_inactive", **ids)
                raise RunRefused(None) from None
            raise

    async def precheck(self, state: RunState, desired: str, ids: Mapping[str, Any]) -> None:
        """The environment and the pre-check ``claude auth status`` of 18.3 (also per restart).

        Args:
            state: The run; its ``env``, ``mode`` and ``evidence`` are set.
            desired: The token's ``billing_mode``.
            ids: IDs for log lines.

        Raises:
            RunRefused: ``runtime_not_authenticated``, ``billing_mode_mismatch``,
                ``adapter_error`` or ``runtime_error``.
        """
        api_key: Secret | None = None
        config_dir: Path | None = None
        if desired == "organization":
            ref = self.config.anthropic_api_key_ref
            try:
                if ref is None:
                    raise SecretError("missing")
                api_key = resolve(ref)
            except SecretError as err:
                # No key: never a CLI call, never a fallback to a subscription (BR-12).
                log("api_key_unavailable", code=err.code, **ids)
                raise RunRefused("runtime_not_authenticated") from None
            config_dir = ensure_private_dir(
                self.config.state_dir / "claude-config" / "organization"
            )
        elif not (desired == "personal" and self.config.allow_personal):
            log("billing_mode_not_allowed", **ids)
            raise RunRefused("billing_mode_mismatch")
        try:
            state.env = build_environment(
                self.environ,
                mode=desired,
                api_key=api_key,
                config_dir=config_dir,
                forward_ssh_agent=self.config.forward_ssh_agent,
            )
        except RunEnvironmentError as err:
            log(err.code, **ids)
            raise RunRefused("adapter_error") from None
        if self.claude_bin is None:
            log("runtime_unreachable", **ids)
            raise RunRefused("runtime_error")
        cwd = state.cwd or self.config.projects_root
        method = await auth_status(self.claude_bin, state.env, cwd, self.config.cli_probe_timeout)
        outcome = decide(desired, method)
        if outcome != "start":
            log("precheck_refused", reason=outcome, **ids)
            raise RunRefused(outcome)
        state.mode = desired
        state.evidence = evidence(method)

    async def on_not_started(self, inv: Invocation) -> None:
        """Give back slot and locks of a run that did not reach ``execute``.

        Args:
            inv: The invocation.
        """
        run_id = inv.claims.run_id
        state = self._runs.pop(run_id, None) if run_id else None
        if state is not None:
            state.close()

    # ---- execute --------------------------------------------------------------------------

    async def execute(self, inv: Invocation, run: RunHandle) -> None:
        """Run one turn and end the run itself (notes 9 to 14).

        Args:
            inv: The invocation.
            run: The started run.
        """
        state = self._runs[run.run_id]
        try:
            await RunTurn(self, state, run).run()
        finally:
            self._runs.pop(run.run_id, None)
            state.close()
            with contextlib.suppress(OSError):
                sweep(self.config.state_dir, self.config.stream_log_retention)


class Runner:
    """Start and stop of the runner process (note 3).

    Args:
        config: The configuration.
        environ: The process environment.
        http_transport: An ``httpx`` transport for the SDK client (tests).
        clock: The SDK's time source (tests pass the FakeServer's clock).
    """

    def __init__(
        self,
        config: RunnerConfig,
        environ: Mapping[str, str] | None = None,
        *,
        http_transport: httpx.AsyncBaseTransport | None = None,
        clock: Clock | None = None,
    ) -> None:
        """Create the runner; nothing connects before `run`."""
        self.clock = clock
        self.config = config
        self.handler = RunnerHandler(config, os.environ if environ is None else environ)
        self.http_transport = http_transport
        self.adapter: Adapter | None = None

    def _credentials(self) -> Credentials:
        try:
            token = resolve(self.config.agent_token_ref)
        except SecretError as err:
            raise StartError(f"agent_token_{err.code}") from None
        dpop = None
        ref = self.config.dpop_key_ref
        if ref is not None:
            if ref.scheme != "file":
                raise StartError("dpop_key_must_be_a_file")
            try:
                dpop = FileDpopKey.load(ref.target)
            except (OSError, ValueError):
                raise StartError("dpop_key_unreadable") from None
        return Credentials(token, dpop)

    async def build(self) -> Adapter:
        """Read the identity, load the channels and build the SDK adapter.

        Returns:
            The adapter (not yet running).

        Raises:
            StartError: The platform is not POSIX, the agent token is missing, the principal
                is not an agent, or the server cannot be reached.
        """
        if os.name != "posix" or sys.platform not in ("linux", "darwin"):
            raise StartError("platform_not_supported")
        credentials = self._credentials()
        base_url = self.config.server_url + "/api/v1"
        probe = Client(
            base_url,
            credentials,
            settings=self.config.sdk,
            clock=self.clock,
            transport=self.http_transport,
        )
        try:
            me = await probe.call_as(Me, get_me.asyncio_detailed)
        except (ProblemError, httpx.TransportError) as err:
            raise StartError("identity_unavailable") from err
        finally:
            await probe.aclose()
        if str(me.principal.kind) != "agent":
            raise StartError("principal_not_an_agent")
        await self.handler.refresh_probe()
        adapter_config = AdapterConfig(
            base_url=base_url,
            instance_id=self.config.instance_id,
            agent_id=me.principal.id,
            credentials=credentials,
            consumer=self.config.consumer,
            state_dir=self.config.state_dir,
            adapter_name=ADAPTER_NAME,
            adapter_version=__version__,
            runtime_name=RUNTIME_NAME,
            capabilities=dict(CAPABILITIES),
            settings=self.config.sdk,
        )
        self.adapter = Adapter(
            adapter_config, self.handler, clock=self.clock, http_transport=self.http_transport
        )
        self.handler.adapter = self.adapter
        cache = ChannelCache(self.adapter.client, self.config.projects_root)
        try:
            await cache.load()
        except (ProblemError, httpx.TransportError) as err:
            raise StartError("channels_unavailable") from err
        self.handler.channels = cache
        self._startup_files()
        return self.adapter

    def _startup_files(self) -> None:
        sweep(self.config.state_dir, self.config.stream_log_retention)

    async def run(self) -> CloseReason:
        """Build the adapter and run it until it stops.

        Returns:
            The SDK's close reason.
        """
        adapter = self.adapter or await self.build()
        try:
            # Before the first event of the inbox and the first heartbeat (design note 12 d).
            await self.handler.adopt_turns(adapter)
            return await adapter.run()
        finally:
            await self.handler.close()

    async def stop(self) -> None:
        """Stop the adapter; running turns are cancelled by the SDK's shutdown."""
        if self.adapter is not None:
            await self.adapter.stop()


async def _newest_token(adapter: Adapter, run_id: UUID) -> Secret | None:
    """The newest invocation token of a run from the durable inbox (the previous process's)."""
    entries = [e for e in await adapter.inbox.for_run(run_id) if e.token is not None]
    entries.sort(key=lambda e: e.token_iat or 0)
    return entries[-1].token if entries else None


def _result_after(stream: Path, offset: int) -> bool:
    """Whether the stream file holds a ``result`` line after ``offset``."""
    try:
        with stream.open("rb") as fh:
            fh.seek(offset)
            data = fh.read()
    except OSError:
        return False
    for raw in data.split(b"\n"):
        with contextlib.suppress(ValueError):
            message = json.loads(raw)
            if isinstance(message, dict) and message.get("type") == "result":
                return True
    return False
