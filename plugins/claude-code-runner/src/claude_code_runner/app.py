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
import json
import logging
import os
import sys
import time
from collections.abc import Mapping
from dataclasses import dataclass, field
from datetime import UTC, datetime
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
from bothy_sdk.context import render_context
from bothy_sdk.errors import (
    ConflictError,
    InvocationTokenInvalid,
    NotFoundError,
    ProblemError,
    RateLimitedError,
    ServerError,
    VersionConflict,
)
from bothy_sdk.gateway import CloseReason
from bothy_sdk.generated.api.agent_runtime import finish_run, report_agent_session
from bothy_sdk.generated.api.me import get_me
from bothy_sdk.generated.models import (
    AgentSession,
    AgentSessionReport,
    BillingEvidence,
    CostReport,
    Me,
    RunContext,
    RunFinish,
    RunFinishState,
    RunMetrics,
    ToolClass,
)
from bothy_sdk.generated.types import Unset
from bothy_sdk.outbox import OutboxState
from bothy_sdk.runs import RunHandle
from bothy_sdk.sources import Envelope

from claude_code_runner import ADAPTER_NAME, RUNTIME_NAME, __version__
from claude_code_runner.attachments import fetch_attachments
from claude_code_runner.authcheck import auth_status, decide, evidence
from claude_code_runner.channels import ChannelCache
from claude_code_runner.commandline import (
    ArgumentError,
    SessionChoice,
    build_argv,
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
from claude_code_runner.livebox import LiveBox
from claude_code_runner.locks import LockTimeout, acquire, release
from claude_code_runner.prompt import context_items, system_prompt, user_line
from claude_code_runner.secrets import SecretError, resolve
from claude_code_runner.statedir import ensure_private_dir, sweep, write_private
from claude_code_runner.streamreader import StreamReader
from claude_code_runner.title import propose_title, split_title
from claude_code_runner.turn import (
    Turn,
    remove_turn_record,
    terminate,
    turn_paths,
    write_turn_record,
)
from claude_code_runner.usage import (
    UsageStore,
    context_windows,
    cost_reports,
    counters_reset,
    difference,
    parse_model_usage,
    subtract_baseline,
)
from claude_code_runner.workspace import WorkspaceError, readiness
from claude_code_runner.workspace import resolve as resolve_workspace

__all__ = ["Runner", "RunnerHandler", "StartError", "log", "warn"]

_LOG = logging.getLogger("claude_code_runner")
_EXPECTED_KEY_SOURCE = {"organization": "ANTHROPIC_API_KEY", "personal": "none"}
# The currency of the runtime's costUSD; a cost baseline in another currency is not subtracted.
_RUNTIME_CURRENCY = "USD"


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


@dataclass
class _Run:
    """What prepare took for one run; `close` gives it back (one object per run)."""

    run_id: UUID
    thread_id: UUID
    channel_id: UUID
    locks: list[int] = field(default_factory=list)
    slot: asyncio.Semaphore | None = None
    cwd: Path | None = None
    session: SessionChoice | None = None
    agent_session: AgentSession | None = None
    model: str = ""
    tools: list[str] = field(default_factory=list)
    env: dict[str, str] = field(default_factory=dict)
    mode: str = "unknown"
    evidence: BillingEvidence | None = None
    context: RunContext | None = None
    file_texts: dict[UUID, str] = field(default_factory=dict)
    stored_files: int = 0
    channel_prompt: str = ""

    def close(self) -> None:
        """Release the slot and the locks (locks in reverse order)."""
        for fd in reversed(self.locks):
            release(fd)
        self.locks.clear()
        if self.slot is not None:
            self.slot.release()
            self.slot = None


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
        self._runs: dict[UUID, _Run] = {}
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
        return runtime_status(self.probe, cache.readiness() if cache else [], self._modes)

    async def on_ready(self) -> None:
        """Send the first heartbeat as soon as the connection is ready (note 3)."""
        if self.adapter is not None:
            await self.adapter.send_heartbeat()

    async def on_event(self, envelope: Envelope) -> None:
        """``agent.config_changed`` reloads the forum and reports a changed readiness at once.

        Args:
            envelope: The event.
        """
        if envelope.type != "agent.config_changed" or self.channels is None:
            return
        channel_id = getattr(getattr(envelope.event, "payload", None), "channel_id", None)
        if not isinstance(channel_id, UUID):
            return
        if await self.channels.reload(channel_id) and self.adapter is not None:
            await self.adapter.send_heartbeat()

    async def on_followup(self, inv: Invocation, run: RunHandle) -> None:
        """No follow-ups before M2-35 (no ``followups`` capability): log by type only."""
        log("event_ignored", type=inv.type, run_id=run.run_id)

    async def on_interaction(self, inv: Invocation, run: RunHandle) -> None:
        """The runner builds no components: log by type only."""
        log("event_ignored", type=inv.type, run_id=run.run_id)

    async def on_command(self, inv: Invocation, run: RunHandle | None) -> None:
        """The runner registers no commands: log by type only."""
        log("event_ignored", type=inv.type)

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
        """Cancel a running version probe and wait for it (its process is killed)."""
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
        state = _Run(run.run_id, payload.thread_id, payload.channel_id)
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
        await self._precheck(state, inv.claims.billing_mode, ids)
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

    async def _take_locks(self, state: _Run, ids: Mapping[str, Any]) -> None:
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

    async def _precheck(self, state: _Run, desired: str, ids: Mapping[str, Any]) -> None:
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
            await _Turn(self, state, run).run()
        finally:
            self._runs.pop(run.run_id, None)
            state.close()
            with contextlib.suppress(OSError):
                sweep(self.config.state_dir, self.config.stream_log_retention)


class _Turn:
    """One turn of a started run."""

    def __init__(self, handler: RunnerHandler, state: _Run, run: RunHandle) -> None:
        self.h = handler
        self.cfg = handler.config
        self.state = state
        self.handle = run
        self.session: SessionChoice = state.session  # type: ignore[assignment]
        self.ids = {
            "run_id": run.run_id,
            "thread_id": state.thread_id,
            "channel_id": state.channel_id,
        }
        started = state.agent_session is not None and state.agent_session.started is True
        self.title_request = not started
        self.report_due = not started
        self.turn: Turn | None = None

    async def run(self) -> None:
        """Start the turn, follow its stream and end the run."""
        paths = turn_paths(self.cfg.state_dir, self.session.ref, str(self.handle.run_id))
        ensure_private_dir(paths.stream.parent)
        write_private(
            paths.system_prompt,
            system_prompt(
                title_request=self.title_request, channel_prompt=self.state.channel_prompt
            ),
        )
        context = self.state.context
        if context is None:  # pragma: no cover - prepare always loads it
            raise RuntimeError("no context")
        rendered = render_context(context, file_texts=self.state.file_texts)
        line, own_uuid = user_line(context_items(rendered))
        write_private(paths.input, line + "\n")
        add_dir = None
        if self.cfg.tool_class in ("scoped", "host") and self.state.stored_files:
            add_dir = self.cfg.state_dir / "files" / str(self.state.thread_id)
        argv = build_argv(
            claude_bin=self.h.claude_bin or self.cfg.claude_bin,
            model=self.state.model,
            session=self.session,
            tool_class=self.cfg.tool_class,
            tools=self.state.tools,
            add_dir=add_dir,
            system_prompt_file=paths.system_prompt,
        )
        cwd = self.state.cwd or self.cfg.projects_root
        try:
            self.turn = Turn.start(
                argv, cwd=cwd, env=self.state.env, paths=paths, pass_fds=self.state.locks
            )
        except OSError:
            warn("turn_start_failed", **self.ids)
            await self._fail_runtime("start_failed")
            return
        write_turn_record(
            paths.record,
            {
                "run_id": str(self.handle.run_id),
                "thread_id": str(self.state.thread_id),
                "channel_id": str(self.state.channel_id),
                "session_ref": self.session.ref,
                "pid": self.turn.pid,
                "pgid": self.turn.pgid,
                "started_at": time.time(),
                "stream": str(paths.stream),
                "stderr": str(paths.stderr),
                "system_prompt": str(paths.system_prompt),
                "input": str(paths.input),
            },
        )
        log("turn_started", **self.ids)
        try:
            await self.turn.write_line(line)
            await self._follow(own_uuid)
        except asyncio.CancelledError:
            # A runtime never outlives the end of its run (09 1 point 4).
            await terminate(
                self.turn, self.cfg.stop_grace, self.cfg.kill_grace, self.cfg.poll_interval
            )
            raise
        finally:
            remove_turn_record(paths.record)

    async def _follow(self, own_uuid: str) -> None:
        turn = self.turn
        if turn is None:  # pragma: no cover - run() starts it first
            return
        usage = UsageStore(self.cfg.state_dir / "sessions" / self.session.ref)
        live = LiveBox(
            self.handle.patch,
            own_uuid=own_uuid,
            title_requested=self.title_request,
            context_windows=usage.load().context_windows,
            now=lambda: datetime.now(UTC),
        )
        reader = StreamReader(turn.paths.stream, self.cfg.poll_interval, turn.alive)
        result: dict[str, Any] | None = None
        async for message in reader.messages():
            if message.get("type") == "system" and message.get("subtype") == "init":
                if message.get("apiKeySource") != _EXPECTED_KEY_SOURCE.get(self.state.mode):
                    warn("counter_check_failed", **self.ids)
                    await terminate(
                        turn, self.cfg.stop_grace, self.cfg.kill_grace, self.cfg.poll_interval
                    )
                    await self._finish("failed", reason="billing_mode_mismatch")
                    return
                await self._report_session(message)
            live.apply(message)
            if message.get("type") == "result":
                result = message
        if result is not None:
            turn.close_stdin()
            await turn.wait(self.cfg.stop_grace, self.cfg.poll_interval)
        # Always: the group may hold children the leader left behind (a background job).
        await terminate(turn, self.cfg.stop_grace, self.cfg.kill_grace, self.cfg.poll_interval)
        if result is None or result.get("is_error") is True or result.get("subtype") != "success":
            await self._fail_runtime(turn.exit_label(), result)
            return
        await self._complete(result, live)

    async def _report_session(self, init: Mapping[str, Any]) -> None:
        if not self.report_due or init.get("session_id") != self.session.ref:
            return
        self.report_due = False
        session = self.state.agent_session
        version = str(session.version) if session is not None else "1"
        body = AgentSessionReport(
            external_session_ref=self.session.ref, model=self.state.model, started=True
        )
        for _ in range(2):
            try:
                await self.handle.client.call_as(
                    AgentSession,
                    report_agent_session.asyncio_detailed,
                    thread_id=self.state.thread_id,
                    body=body,
                    if_match=f'"{version}"',
                )
                return
            except VersionConflict as err:
                current = err.current or {}
                if current.get("external_session_ref") != self.session.ref:
                    log("session_report_failed", code=err.code, **self.ids)
                    return
                version = str(current.get("version"))
            except ProblemError as err:
                log("session_report_failed", code=err.code, **self.ids)
                return
            except httpx.TransportError:
                log("session_report_failed", code="transport_error", **self.ids)
                return
        log("session_report_failed", code="version_conflict", **self.ids)

    async def _fail_runtime(self, exit_label: str, result: Mapping[str, Any] | None = None) -> None:
        patch = self.handle.patch
        step = patch.add_step(
            "status", "runtime_error", title="", status="failed", detail={"exit": exit_label}
        )
        patch.update_step(step, finished_at=datetime.now(UTC))
        if result is None:
            await self._finish("failed", reason="runtime_error")
            return
        # A failed turn may still have spent tokens: they are this run's, never the next one's.
        reports, metrics = self._usage(result, None)
        await self._finish("failed", reason="runtime_error", metrics=metrics, costs=reports)

    def _usage(
        self, result: Mapping[str, Any], live: LiveBox | None
    ) -> tuple[list[CostReport], RunMetrics]:
        """The cost reports (stored as pending before the finish) and the metrics of the run."""
        store = UsageStore(self.cfg.state_dir / "sessions" / self.session.ref)
        reports = store.pending_for(str(self.handle.run_id))
        if reports is None:
            state = store.load()
            current = parse_model_usage(result)
            reports = cost_reports(difference(state.values, current))
            remaining = state.baseline_remaining
            if remaining is None and not state.values:
                remaining = self._start_baseline()
            if remaining is not None:
                if counters_reset(state.values, current):
                    # The runtime counts again: the old amounts are no longer in its sums.
                    remaining = 0
                reports, remaining = subtract_baseline(reports, remaining)
            store.record(
                str(self.handle.run_id), current, reports, context_windows(result), remaining
            )
        metrics = RunMetrics(
            tokens_in=sum(_n(r.tokens_in) for r in reports),
            tokens_out=sum(_n(r.tokens_out) for r in reports),
            tokens_cache_read=sum(_n(r.tokens_cache_read) for r in reports),
            tokens_cache_write=sum(_n(r.tokens_cache_write) for r in reports),
        )
        if live is not None:
            metrics.subagents = live.subagents
            metrics.background_jobs = live.background_jobs
            metrics.api_retries = live.api_retries
            if live.context_pct is not None:
                metrics.context_pct = live.context_pct
        return reports, metrics

    def _start_baseline(self) -> int | None:
        """The baseline of a session whose values the runner never stored (09 18.7, RK-75).

        ``cost_baseline_micros`` of the bound session when it is above 0; a baseline in another
        currency than the runtime's US dollars is ignored with a warning (no conversion).
        ``None`` when the session has none.
        """
        session = self.state.agent_session
        baseline = getattr(session, "cost_baseline_micros", None)
        if not isinstance(baseline, int) or isinstance(baseline, bool) or baseline <= 0:
            return None
        if getattr(session, "currency", None) != _RUNTIME_CURRENCY:
            warn("cost_baseline_currency_ignored", **self.ids)
            return 0
        return baseline

    async def _deliver(self, reply: str) -> str | None:
        """Send the reply; ``None`` once it is stored on the server, else the problem code.

        A reply still pending after the SDK's own retries (a transient outage) is sent again
        with the same ``client_msg_id`` up to ``sdk.retry_max_attempts`` times, so a run never
        ends ``completed`` with its answer lost.
        """
        entry = await self.handle.send(reply)
        outbox = self.handle.outbox
        settings = self.cfg.sdk
        for attempt in range(settings.retry_max_attempts):
            if entry.state is OutboxState.SENT:
                return None
            if entry.state is OutboxState.FAILED:
                return entry.problem_code or "unknown"
            await self.handle.client.clock.sleep(
                min(settings.retry_backoff_cap, settings.retry_backoff_base * 2**attempt)
            )
            await outbox.flush()
            pending = {e.entry_id for e in await outbox.store.pending()}
            if entry.entry_id not in pending:
                failed = {e.entry_id: e for e in await outbox.failed()}
                gone = failed.get(entry.entry_id)
                return None if gone is None else (gone.problem_code or "unknown")
        return "reply_not_delivered"

    async def _complete(self, result: Mapping[str, Any], live: LiveBox) -> None:
        text = result.get("result") if isinstance(result.get("result"), str) else ""
        title, reply = split_title(str(text), self.title_request)
        if reply.strip():
            problem = await self._deliver(reply)
            if problem is not None:
                log("reply_refused", code=problem, **self.ids)
                reports, metrics = self._usage(result, live)
                await self._finish("failed", reason="adapter_error", metrics=metrics, costs=reports)
                return
        if self.title_request and title:
            outcome = await propose_title(self.handle.client, self.state.thread_id, title)
            log("title_proposed", outcome=outcome, **self.ids)
        reports, metrics = self._usage(result, live)
        await self._finish("completed", metrics=metrics, costs=reports)

    async def _finish(self, state: str, **kwargs: Any) -> None:
        """``finishRun`` through the run handle; a lost answer is sent again with the same body.

        A repeated final state answers ``200`` (M2-09 note 6) and the costs come from the
        pending record, so a retry never reports a delta twice. Only the call is repeated,
        never the turn (R-AG-17).
        """
        try:
            await self.handle.finish(state, **kwargs)
        except ConflictError as err:
            # invalid_state_transition: the server ended the run already; never retried.
            log("finish_refused", code=err.code, **self.ids)
        except (ServerError, RateLimitedError, httpx.TransportError):
            await self._finish_again(state, kwargs)
        log("run_finished", state=state, reason=kwargs.get("reason", ""), **self.ids)

    async def _finish_again(self, state: str, kwargs: Mapping[str, Any]) -> None:
        body = RunFinish(state=RunFinishState(state))
        if kwargs.get("reason") is not None:
            body.reason = kwargs["reason"]
        if kwargs.get("metrics") is not None:
            body.metrics = kwargs["metrics"]
        if kwargs.get("costs"):
            body.costs = list(kwargs["costs"])
        settings = self.cfg.sdk
        client = self.handle.client
        for attempt in range(max(1, settings.retry_max_attempts - 1)):
            await client.clock.sleep(
                min(settings.retry_backoff_cap, settings.retry_backoff_base * 2**attempt)
            )
            try:
                await client.call(finish_run.asyncio_detailed, run_id=self.handle.run_id, body=body)
                return
            except ConflictError as err:
                log("finish_refused", code=err.code, **self.ids)
                return
            except (ServerError, RateLimitedError, httpx.TransportError):
                continue
            except ProblemError as err:
                log("finish_refused", code=err.code, **self.ids)
                return
        warn("finish_not_stored", **self.ids)


def _n(value: object) -> int:
    return value if isinstance(value, int) else 0


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
        turns = self.config.state_dir / "turns"
        if turns.is_dir():
            for record in sorted(turns.glob("*.json")):
                # M2-35 takes these over; their turns still hold the locks of their sessions.
                warn("orphan_turn_found", run_id=record.stem)
        sweep(self.config.state_dir, self.config.stream_log_retention)

    async def run(self) -> CloseReason:
        """Build the adapter and run it until it stops.

        Returns:
            The SDK's close reason.
        """
        adapter = self.adapter or await self.build()
        try:
            return await adapter.run()
        finally:
            await self.handler.close()

    async def stop(self) -> None:
        """Stop the adapter; running turns are cancelled by the SDK's shutdown."""
        if self.adapter is not None:
            await self.adapter.stop()
