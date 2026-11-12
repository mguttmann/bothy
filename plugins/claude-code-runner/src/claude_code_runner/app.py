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
from collections.abc import Callable, Mapping
from pathlib import Path
from typing import Any, NoReturn
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
from bothy_sdk.agent_interview import InterviewReply, InterviewRequest
from bothy_sdk.auth import Credentials, FileDpopKey, Secret
from bothy_sdk.client import Client
from bothy_sdk.clock import Clock, SystemClock
from bothy_sdk.errors import InvocationTokenInvalid, NotFoundError, ProblemError
from bothy_sdk.gateway import CloseReason
from bothy_sdk.generated.api.me import get_me
from bothy_sdk.generated.api.runs import get_run
from bothy_sdk.generated.models import (
    AgentSession,
    Heartbeat,
    HeartbeatAck,
    Me,
    Run,
    RunContext,
    ToolClass,
)
from bothy_sdk.generated.types import Unset
from bothy_sdk.invocation import stored_claims
from bothy_sdk.runs import RunHandle
from bothy_sdk.search_assist import AssistQuestion, AssistReply
from bothy_sdk.sources import Envelope

from claude_code_runner import ADAPTER_NAME, RUNTIME_NAME, __version__
from claude_code_runner.adoption import (
    PHASES_BEFORE_START,
    AdoptedGroup,
    AdoptionAction,
    adoption_decision,
    process_matches,
    read_turn_record,
)
from claude_code_runner.attachments import fetch_attachments
from claude_code_runner.authcheck import auth_status, decide, evidence
from claude_code_runner.builder import AgentBuilder
from claude_code_runner.causes import ADAPTER_STOPPED, cause
from claude_code_runner.channel_skills import resolve_channel_skills
from claude_code_runner.channels import RESYNC_EVENTS, ChannelCache
from claude_code_runner.commandline import (
    ArgumentError,
    SessionChoice,
    choose_model,
    choose_session,
    reported_model,
    scoped_tools,
)
from claude_code_runner.config import RunnerConfig
from claude_code_runner.environment import (
    RunEnvironmentError,
    build_environment,
    clean_path,
    find_program,
    kept_environment,
)
from claude_code_runner.handover import (
    hand_over,
    handover_records,
    remove_pid_file,
    stop_turns_requested,
    write_pid_file,
)
from claude_code_runner.heartbeat import (
    MIN_CLAUDE_CODE_VERSION,
    WAIT_SLOTS_FULL,
    WAIT_THREAD_BUSY,
    Probe,
    billing_modes_supported,
    capabilities,
    parse_version,
    probe_version,
    raised_minimum,
    runtime_status,
)
from claude_code_runner.hosting import HostedAgents, HostingContext, stop_leftover_turns
from claude_code_runner.launch import KeeperGroup, LaunchFiles, read_json, remove_launch_files
from claude_code_runner.locks import LockTimeout, SlotPool, acquire, release
from claude_code_runner.materialize import sweep_turn_files
from claude_code_runner.provision import Provisioner
from claude_code_runner.reply_upload import remove_run_files, run_temp_dir
from claude_code_runner.runturn import RunState, RunTurn
from claude_code_runner.searchassist import SearchAssistant
from claude_code_runner.secrets import SecretError, resolve
from claude_code_runner.statedir import ensure_private_dir, sweep
from claude_code_runner.stop import stop_group
from claude_code_runner.transcribe import Transcriber
from claude_code_runner.trash import FolderTrash, GitRunner, TrashPlaces
from claude_code_runner.turn import remove_turn_record, write_turn_record
from claude_code_runner.workspace import (
    WorkspaceError,
    default_readiness,
    project_folders,
    readiness,
    resolve_default,
)
from claude_code_runner.workspace import resolve as resolve_workspace

__all__ = ["RESYNC_EVENTS", "Runner", "RunnerHandler", "StartError", "log", "warn"]

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


def _failed(event: str, what: str, run_id: UUID, err: Exception) -> None:
    """Log an unexpected error of a task the runner itself started for a run.

    The line of level error names only the kind of the error: its message may hold a path or
    a value, and the traceback ends with that message. Both go to a line of level debug, which
    the console script does not write (it logs from level info on).

    Args:
        event: The event code.
        what: What failed, the start of the debug line.
        run_id: The run.
        err: The error.
    """
    line = {"event": event, "run_id": str(run_id), "error": type(err).__name__}
    _LOG.error(json.dumps(line))
    _LOG.debug("%s %s failed", what, run_id, exc_info=err)


def refuse(
    code: str, reason: str, ids: Mapping[str, Any], *, default_workspace: bool = False
) -> NoReturn:
    """Log ``code`` and end the run without a start, naming the cause in the thread.

    Args:
        code: The log code of the cause.
        reason: The reason of ``finishRun``.
        ids: IDs for the log line.
        default_workspace: The cause is about ``runner.default_workspace``.

    Raises:
        RunRefused: Always, with ``reason`` and the text of `cause` as its summary.
    """
    log(code, **ids)
    raise RunRefused(reason, summary=cause(code, default_workspace=default_workspace))


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
        git_bin = find_program(config.git_bin, self.path_entries)
        self.provisioner = Provisioner(
            projects_root=config.projects_root,
            state_dir=config.state_dir,
            git_bin=git_bin,
            environ=self.environ,
            timeout=config.provision_timeout,
            retry_after=config.provision_retry_after,
            on_done=self.on_provisioned,
        )
        self.transcriber = _transcriber(config, self.environ, self.path_entries)
        self.search_assistant = SearchAssistant(
            claude_bin=self.claude_bin,
            env=self._assist_env,
            state_dir=config.state_dir,
            model=config.search_assist_model,
            timeout=config.search_assist_timeout,
            now=self._now,
        )
        self.builder = AgentBuilder(
            claude_bin=self.claude_bin,
            env=self._assist_env,
            state_dir=config.state_dir,
            model=config.agent_builder_model,
            timeout=config.agent_builder_timeout,
            now=self._now,
        )
        self.trash = FolderTrash(
            projects_root=config.projects_root,
            state_dir=config.state_dir,
            git=GitRunner(git_bin=git_bin, environ=self.environ, timeout=config.cli_probe_timeout),
            places=TrashPlaces.of(self.environ),
            interval=config.trash_check_interval,
            bound=self._workspace_refs,
            on_done=self.on_provisioned,
        )
        self._trashing: asyncio.Task[None] | None = None
        self.probe = Probe()
        self.minimum = MIN_CLAUDE_CODE_VERSION
        """The lowest supported CLI version; raised when the API refuses the model (18.9)."""
        self._probing: asyncio.Task[Probe] | None = None
        self._runs: dict[UUID, RunState] = {}
        self.adopted: dict[UUID, RunTurn] = {}
        self._adopting: set[asyncio.Task[None]] = set()
        self._resync_lock = asyncio.Lock()
        self._resyncing: asyncio.Task[None] | None = None
        self.slots = SlotPool(config.max_parallel_runs)
        """The slots of every turn of the process, shared with hosted agents (26 3.2 item 3)."""
        self.hosting: HostedAgents | None = None
        """The hosted agents with ``runner.hosted_agents``; ``None`` without (26 3.2 item 4)."""
        self._waiting: dict[UUID, str] = {}
        """The runs that wait in prepare, with their wait reason (09 5.2.1)."""
        self._beats: set[asyncio.Task[None]] = set()
        self._modes = billing_modes_supported(
            has_key_ref=config.anthropic_api_key_ref is not None,
            allow_personal=config.allow_personal,
        )
        self.turn_handover = config.turn_handover
        """``runner.turn_handover``: turns run under a keeper and are handed over (18.19)."""
        self.handing_over = False
        """SIGTERM began a handover: live turns are left running with their records (18.19.3)."""
        self.stopping_all = False
        """``stop --turns``: every adopted turn is stopped and its run finished ``cancelled``."""

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
        folders = project_folders(self.config.projects_root, self.config.project_folders_max)
        readiness = cache.readiness() if cache else []
        self.trash.annotate(readiness)
        status = runtime_status(
            self.probe,
            readiness,
            self._modes,
            folders,
            self.trash.results(),
            slots=(self.slots.used, self.slots.total),
            waiting=dict(self._waiting),
        )
        return dataclasses.replace(status, active_runs=sorted(self.adopted, key=str))

    def _wait(self, run_id: UUID, reason: str | None) -> None:
        """Note that a run in prepare waits for ``reason`` (``None``: no longer) and report it.

        A change sends a heartbeat at once (09 5.2.1), so the person sees why the run waits;
        the task is kept until it ends and cancelled by `close`.
        """
        if self._waiting.get(run_id) == reason:
            return
        if reason is None:
            self._waiting.pop(run_id, None)
        else:
            self._waiting[run_id] = reason
            log("run_waiting", run_id=run_id, reason=reason)
        if self.adapter is None:
            return
        task = asyncio.get_running_loop().create_task(self.adapter.send_heartbeat())
        self._beats.add(task)
        task.add_done_callback(self._beats.discard)

    def _workspace_refs(self) -> dict[UUID, str]:
        return self.channels.workspace_refs() if self.channels is not None else {}

    async def on_heartbeat_ack(self, sent: Heartbeat, ack: HeartbeatAck) -> None:
        """An accepted heartbeat: its folder move results are sent; take new requests (18.15).

        Args:
            sent: The heartbeat the server accepted.
            ack: Its answer with ``project_trash``.
        """
        results = sent.project_trash_results
        if not isinstance(results, Unset):
            self.trash.reported(r.channel_id for r in results)
        requests = ack.project_trash
        if not isinstance(requests, Unset) and requests:
            self.trash.requested(requests, stop=self.stop_channel)

    async def stop_channel(self, channel_id: UUID) -> bool:
        """Stop every turn of a channel and wait until none runs (18.15, the stop path of 18.6).

        Args:
            channel_id: The deleted forum.

        Returns:
            True once no run of the channel is left; false when one still runs after
            ``runner.stop_grace`` plus ``runner.kill_grace`` plus ``runner.cli_probe_timeout``.
        """
        reason = "channel_read_only"  # a deleted forum is archived (D-DEL-2)
        for state in list(self._runs.values()):
            if state.channel_id != channel_id:
                continue
            if state.turn is None:
                state.stop_reason = state.stop_reason or reason
            else:
                await state.turn.request_stop(reason)
        for turn in list(self.adopted.values()):
            if turn.state.channel_id == channel_id:
                await turn.request_stop(reason)
        loop = asyncio.get_running_loop()
        deadline = (
            loop.time()
            + self.config.stop_grace
            + self.config.kill_grace
            + self.config.cli_probe_timeout
        )
        while self._channel_busy(channel_id):
            if loop.time() >= deadline:
                return False
            await asyncio.sleep(self.config.poll_interval)
        return True

    async def stop_turns(self, *, adopted: bool = False) -> bool:
        """An orderly stop of the runner: stop every own turn and wait until its run ended.

        Each turn of this process with a live process is stopped as in 18.6 with the reason
        ``adapter_stopped`` and its run finishes ``cancelled`` once no process of its group is
        left (D-AR-1). A run between processes (an answer being posted, a recovery being
        prepared) is left to the SDK's cancellation, which stops a process it started meanwhile
        (`RunTurn.run`). Adopted turns keep running and keep their records for the next start
        (18.9), unless ``adopted`` (``stop --turns``, 18.19.3); a run that has no turn yet is
        started again from the inbox by the next process (09 4.9).

        Args:
            adopted: Stop the adopted turns too and wait for their runs.

        Returns:
            True once none of these runs is left; false when one still runs after
            ``runner.stop_grace`` plus ``runner.kill_grace`` plus ``runner.cli_probe_timeout``.
        """
        stopping = [
            (s.run_id, s.turn)
            for s in self._runs.values()
            if s.turn is not None and s.turn.process is not None
        ]
        if adopted:
            stopping += list(self.adopted.items())
        for _, turn in stopping:
            await turn.request_stop(ADAPTER_STOPPED)
        loop = asyncio.get_running_loop()
        deadline = (
            loop.time()
            + self.config.stop_grace
            + self.config.kill_grace
            + self.config.cli_probe_timeout
        )
        while any(run_id in self._runs or run_id in self.adopted for run_id, _ in stopping):
            if loop.time() >= deadline:
                return False
            await asyncio.sleep(self.config.poll_interval)
        return True

    def _channel_busy(self, channel_id: UUID) -> bool:
        return any(s.channel_id == channel_id for s in self._runs.values()) or any(
            t.state.channel_id == channel_id for t in self.adopted.values()
        )

    async def trash_periodically(self) -> None:
        """Check the bound folders and send a heartbeat every ``runner.trash_check_interval``.

        The gateway's ``heartbeat_ack`` carries no ``project_trash``; this ``sendHeartbeat``
        is how the runner learns of a folder move (09 18.15). The first check goes out with the
        adapter's own first heartbeat: an extra heartbeat before the adapter listens would
        report the runner present before it can take an event.
        """
        await self.trash.refresh()
        while True:
            await asyncio.sleep(self.config.trash_check_interval)
            await self.trash.refresh()
            if self.adapter is not None:
                await self.adapter.send_heartbeat()

    def start_trash(self) -> None:
        """Start the timer of `trash_periodically` (once); `close` cancels it."""
        if self._trashing is None or self._trashing.done():
            self._trashing = asyncio.get_running_loop().create_task(self.trash_periodically())

    async def on_provisioned(self) -> None:
        """A folder creation of 09 18.12 or a folder move of 18.15 ended: report it at once."""
        if self.adapter is not None:
            await self.adapter.send_heartbeat()

    async def on_ready(self) -> None:
        """Send the first heartbeat as soon as the connection is ready (note 3).

        With ``runner.hosted_agents`` the hosted set is listed after it: the server issues a
        hosted token only to a runner whose stored heartbeat reports ``hosts_agents``
        (26 3.3 item 1), and the heartbeat is stored when it is answered.
        """
        if self.adapter is not None:
            await self.adapter.send_heartbeat()
        if self.hosting is not None:
            await self.hosting.start()

    async def on_event(self, envelope: Envelope) -> None:
        """``agent.config_changed`` reloads the forum, an operator change every served channel.

        A changed readiness is reported at once (09 18.1).

        Args:
            envelope: The event.
        """
        if envelope.type == "run.cancel_requested":
            await self._cancel_adopted(envelope)
            return
        if self.hosting is not None and await self.hosting.on_event(envelope):
            return
        if envelope.type in RESYNC_EVENTS:
            await self.resync_channels(envelope.type)
            return
        if envelope.type != "agent.config_changed" or self.channels is None:
            return
        channel_id = getattr(getattr(envelope.event, "payload", None), "channel_id", None)
        if not isinstance(channel_id, UUID):
            return
        if await self.channels.reload(channel_id) and self.adapter is not None:
            await self.adapter.send_heartbeat()

    async def resync_channels(self, cause: str) -> None:
        """Load the served channels again and send a heartbeat when a readiness changed.

        A failure keeps the cache and is logged (``channels_resync_failed``); the next event or
        the timer tries again.

        Args:
            cause: The event type or ``timer``, for the log line.
        """
        cache = self.channels
        if cache is None:
            return
        async with self._resync_lock:
            try:
                changed = await cache.resync()
            except (ProblemError, httpx.TransportError) as err:
                warn("channels_resync_failed", cause=cause, code=getattr(err, "code", "transport"))
                return
        log("channels_resynced", cause=cause, changed=changed)
        if changed and self.adapter is not None:
            await self.adapter.send_heartbeat()

    async def resync_periodically(self) -> None:
        """The fallback of `resync_channels` every ``runner.channels_resync_interval``."""
        while True:
            await asyncio.sleep(self.config.channels_resync_interval)
            await self.resync_channels("timer")

    def start_resync(self) -> None:
        """Start the timer of `resync_periodically` (once); `close` cancels it."""
        if self._resyncing is None or self._resyncing.done():
            self._resyncing = asyncio.get_running_loop().create_task(self.resync_periodically())

    async def on_followup(self, inv: Invocation, run: RunHandle) -> None:
        """A follow-up of a run of this runner: held until its turn starts, or offered (18.5).

        Args:
            inv: The ``agent.followup`` invocation.
            run: The run.
        """
        state = self._runs.get(run.run_id)
        message_id = getattr(inv.payload, "message_id", "")
        adopted = self.adopted.get(run.run_id)
        if state is None and adopted is not None and adopted.takes_followups():
            await adopted.offer_followup(inv)  # adoption with full rights (18.19.7)
            return
        if state is None:
            log("followup_not_deliverable", run_id=run.run_id, message_id=message_id)
            return
        if state.turn is None:
            if all(getattr(h.payload, "message_id", None) != message_id for h in state.held):
                state.held.append(inv)
            log("followup_held", run_id=run.run_id, message_id=message_id)
            return
        await state.turn.offer_followup(inv)

    def adopted_run(self, run_id: UUID) -> RunHandle | None:
        """The handle of a turn adopted with full rights, for its follow-ups (18.19.7).

        Args:
            run_id: The run of a follow-up.

        Returns:
            The handle while the adopted turn takes follow-ups, else ``None`` (the follow-up
            stays ``queued`` and the server defers it).
        """
        turn = self.adopted.get(run_id)
        if turn is None or not turn.takes_followups():
            return None
        return turn.handle

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

        The turns of hosted agents are never taken over: one that still runs is stopped and its
        record removed (`claude_code_runner.hosting.stop_leftover_turns`).

        Args:
            adapter: The built adapter (its client, inbox and outbox).
        """
        turns = self.config.state_dir / "turns"
        for path in sorted(turns.glob("*.json")) if turns.is_dir() else []:
            # Checked before anything is built from it: the run of its file name, and UUIDs
            # for the thread and the session, whose names the locks carry. Only the record of
            # a run before ``run.start`` carries no session: it is removed by its phase, and
            # without the key the start is refused while one is left (`Runner.run`).
            record = read_turn_record(path)
            if record is None:
                warn("turn_record_unreadable", record=path.stem)
                continue
            run_id, thread_id = UUID(record["run_id"]), UUID(record["thread_id"])
            await self._adopt(adapter, path, record, run_id, thread_id)
        await stop_leftover_turns(
            self.config.state_dir,
            stop_grace=self.config.stop_grace,
            kill_grace=self.config.kill_grace,
            poll=self.config.poll_interval,
            timeout=self.config.cli_probe_timeout,
        )

    async def _adopt(
        self, adapter: Adapter, path: Path, record: dict[str, Any], run_id: UUID, thread_id: UUID
    ) -> None:
        ids = {"run_id": run_id, "thread_id": thread_id}
        if self.turn_handover and "phase" in record:
            await self._adopt_handover(adapter, path, record, run_id, thread_id)
            return
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
        in_log = end_in_record_or_log(record)
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
        if record.get("cwd"):
            state.cwd = Path(str(record["cwd"]))  # the root of answer attachments (M3-12)
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
        if self.stopping_all:
            turn.stop_reason = ADAPTER_STOPPED  # stop --turns: stopped as in 18.6 at once
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
        except Exception as err:
            if adapter.client.http.is_closed:
                # No failure of the turn: the adapter closed under its follow, because the
                # runner ends. The turn runs on and the next start adopts it again (18.9).
                _LOG.debug(
                    "the follow of the adopted run %s ended with the runner", run_id, exc_info=True
                )
            else:
                # The turn is stopped and its run ended (`RunTurn.adopted`); this task is the
                # follow's only caller, so the error is logged here.
                _failed("adopted_follow_failed", "the follow of the adopted run", run_id, err)
        finally:
            self.adopted.pop(run_id, None)
            adapter.client.release_run(run_id)

    async def _adopt_handover(
        self, adapter: Adapter, path: Path, record: dict[str, Any], run_id: UUID, thread_id: UUID
    ) -> None:
        """A record written with ``runner.turn_handover``, by its phase (09 18.19.6).

        ``preparing`` and ``waiting_lock``: the record is removed; the inbox restart of 4.9
        prepares the run again. Every other phase is continued by `RunTurn.take_over` while the
        server shows the run ``running``; a live turn of a run the server already ended is
        stopped as in 18.6 without a finish.
        """
        ids = {"run_id": run_id, "thread_id": thread_id}
        phase = str(record.get("phase"))
        if phase in PHASES_BEFORE_START:
            remove_turn_record(path)
            log("turn_adopted", action="restart_from_inbox", phase=phase, **ids)
            return
        token = await _newest_token(adapter, run_id)
        try:
            with adapter.client.unbound():
                run = await adapter.client.call_as(Run, get_run.asyncio_detailed, run_id=run_id)
        except (ProblemError, httpx.TransportError) as err:
            log("adoption_run_unreadable", code=getattr(err, "code", "transport_error"), **ids)
            run = None
        if token is None or run is None or str(run.state.value) != "running":
            await self._end_unowned(path, record, ids)
            return
        log("turn_adopted", action="take_over", phase=phase, **ids)
        handle = RunHandle(
            adapter.client, run_id, thread_id, outbox=adapter.outbox, settings=self.config.sdk
        )
        handle.started(run, stored_claims(token).jti)
        state = _state_of(record, run_id, thread_id, run.channel_id)
        cached = self.channels.get(run.channel_id) if self.channels is not None else None
        state.channel_prompt = cached.behavior.prompt if cached is not None else ""
        turn = RunTurn(self, state, handle)
        if self.stopping_all:
            turn.stop_reason = ADAPTER_STOPPED  # stop --turns: stopped as in 18.6
        known = {
            key: str(step.status.value) for key, step in run.steps.additional_properties.items()
        }
        self.adopted[run_id] = turn
        task = asyncio.ensure_future(self._take_over(adapter, turn, record, token, known))
        self._adopting.add(task)
        task.add_done_callback(self._adopting.discard)

    async def _end_unowned(
        self, path: Path, record: Mapping[str, Any], ids: Mapping[str, Any]
    ) -> None:
        """A record whose run the server ended (or cannot show): stop a live turn, no finish."""
        prefix = (record.get("launch") or {}).get("prefix")
        keeper = record.get("keeper")
        if isinstance(prefix, str) and isinstance(keeper, Mapping):
            group = KeeperGroup(
                LaunchFiles(Path(prefix)),
                keeper=keeper,
                cli=record.get("cli") or read_json(LaunchFiles(Path(prefix)).cli) or {},
                session_ref=str(record.get("session_ref")),
                timeout=self.config.cli_probe_timeout,
            )
            if group.may_signal():
                log("turn_adopted", action="stop", **ids)
                await stop_group(
                    group,
                    self.config.stop_grace,
                    self.config.kill_grace,
                    self.config.poll_interval,
                    ids,
                )
        else:
            log("turn_adopted", action="leave", **ids)
        remove_turn_record(path)
        remove_launch_files(self.config.state_dir, str(ids["run_id"]))

    async def _take_over(
        self,
        adapter: Adapter,
        turn: RunTurn,
        record: dict[str, Any],
        token: Secret,
        known: Mapping[str, str],
    ) -> None:
        run_id = turn.handle.run_id
        try:
            with adapter.client.bind_run(run_id, token):
                await turn.take_over(record, known)
        except Exception as err:
            # As in `_follow_adopted`: `RunTurn.take_over` stopped the turn and ended the run.
            _failed("turn_takeover_failed", "the takeover of the run", run_id, err)
        finally:
            self.adopted.pop(run_id, None)
            turn.state.close()  # the locks an adopter took before a launch
            adapter.client.release_run(run_id)

    async def _cancel_adopted(self, envelope: Envelope) -> None:
        payload = getattr(envelope.event, "payload", None)
        run_id = getattr(payload, "run_id", None)
        turn = self.adopted.get(run_id) if isinstance(run_id, UUID) else None
        if turn is None:
            return
        raw = getattr(payload, "reason", None)
        await turn.request_stop(str(getattr(raw, "value", raw) or "user_cancelled"))

    # ---- search assistant (09 18.17) --------------------------------------------------------

    async def on_search_assist(self, request: AssistQuestion) -> AssistReply | None:
        """Answer a question of the search assistant with one call without tools.

        Args:
            request: The question with its candidates.

        Returns:
            The reply, or ``None`` when the call failed (the server then shows full text).
        """
        return await self.search_assistant.answer(request)

    # ---- builder of the interview (26 6.2) --------------------------------------------------

    async def on_agent_interview(self, request: InterviewRequest) -> InterviewReply | None:
        """Answer the interview of "New agent" with one call without tools.

        Only with ``runner.agent_builder`` on; the server sends the request only to the agent of
        its ``agents.builder_agent_id``. The call runs with the billing rule of the search
        assistant (09 18.17 item 1).

        Args:
            request: The interview so far with the catalogue and the limits.

        Returns:
            Questions or a draft that passed the runner's own check, or ``None`` (the server then
            falls back).
        """
        if not self.config.agent_builder:
            return None
        return await self.builder.answer(request)

    def _now(self) -> Any:
        return (self.adapter.clock if self.adapter is not None else SystemClock()).now()

    def _assist_env(self) -> dict[str, str]:
        """The environment of a search assistant call (09 18.17) and of a builder call (26 6.2).

        ``organization`` with the API key when one is configured, else ``personal`` when allowed,
        never a fallback between them.

        Raises:
            RunEnvironmentError: ``no_billing_mode``, or the errors of `build_environment`.
            SecretError: The API key cannot be read.
        """
        ref = self.config.anthropic_api_key_ref
        if ref is not None:
            return build_environment(
                self.environ,
                mode="organization",
                api_key=resolve(ref),
                config_dir=ensure_private_dir(
                    self.config.state_dir / "claude-config" / "organization"
                ),
                forward_ssh_agent=False,
            )
        if not self.config.allow_personal:
            raise RunEnvironmentError("no_billing_mode")
        return build_environment(
            self.environ, mode="personal", api_key=None, config_dir=None, forward_ssh_agent=False
        )

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
        probe = await probe_version(
            self.claude_bin,
            self._probe_env(),
            self.config.state_dir,
            self.config.cli_probe_timeout,
            minimum=self.minimum,
        )
        self.probe = self._held(probe)  # a minimum raised while the probe ran still holds
        if self.probe.reasons:
            warn("runtime_degraded", reasons=",".join(self.probe.reasons))
        return self.probe

    async def close(self) -> None:
        """Stop the hosted agents; cancel the probe, follows, timers, folder creations and moves.

        Adopted turns keep running; what a cancelled folder creation made is removed (09 18.12).
        A follow that fails on the closed adapter before it is cancelled here ends without
        stopping its turn (`RunTurn._on_error`).
        """
        if self.hosting is not None:
            await self.hosting.close()
        await self.provisioner.close()
        await self.trash.close()
        for timer in (self._resyncing, self._trashing):
            if timer is not None and not timer.done():
                timer.cancel()
                with contextlib.suppress(asyncio.CancelledError):
                    await timer
        for follow in list(self._adopting):
            follow.cancel()
            with contextlib.suppress(asyncio.CancelledError, Exception):
                await follow
        for beat in list(self._beats):
            beat.cancel()
            with contextlib.suppress(asyncio.CancelledError, Exception):
                await beat
        task = self._probing
        if task is not None and not task.done():
            task.cancel()
            with contextlib.suppress(asyncio.CancelledError):
                await task

    def _held(self, probe: Probe) -> Probe:
        """``probe`` with ``runtime_version_unsupported`` when its version is below `minimum`."""
        parsed = parse_version(probe.version or "")
        minimum = parse_version(self.minimum)
        if probe.blocking or parsed is None or minimum is None or parsed >= minimum:
            return probe
        return Probe(probe.version, ("runtime_version_unsupported",))

    async def runtime_too_old(self, required: str | None) -> None:
        """The API refused a turn's model to the CLI as too old (09 18.9, D-RUNNER-CLI-PIN-2).

        The minimum rises to the version the API named (without one: past the probed version),
        so the runner reports ``degraded`` with ``runtime_version_unsupported`` and prepare
        refuses runs until a probe finds a CLI at that version; the heartbeat goes out at once.

        Args:
            required: The version the API named, if any.
        """
        self.minimum = raised_minimum(self.minimum, current=self.probe.version, required=required)
        self.probe = self._held(self.probe)
        warn(
            "runtime_version_unsupported",
            version=self.probe.version or "unknown",
            minimum=self.minimum,
        )
        if self.adapter is not None:
            await self.adapter.send_heartbeat()

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
        self._phase(state, "preparing")  # with the key: written when the run is accepted
        if str(payload.trigger_kind) != "message":
            refuse("trigger_kind_unsupported", "adapter_error", ids)
        binding = await self._binding(payload)
        default = self.config.default_workspace
        if binding is None and not self._serves(payload.channel_id):
            # A guest run (09 3.6) in a channel another agent operates: never the default
            # folder, whether that channel is bound or not (D-RUNNER-NO-GIT-3).
            refuse("channel_not_served", "adapter_error", ids)
        if binding is None and default is None:
            refuse("project_not_bound", "adapter_error", ids)
        if self.probe.blocking:
            refuse("runtime_unavailable", "runtime_error", ids)
        state.cwd = self._working_folder(binding, ids)
        session = payload.agent_session
        state.agent_session = session if isinstance(session, AgentSession) else None
        mode = getattr(payload.session_mode, "value", None)
        started = state.agent_session is not None and state.agent_session.started is True
        state.session_mode = str(mode) if mode else ("resume" if started else "new")
        # A forum without a binding has no runner settings: the runner's own defaults apply.
        runner_config = binding.runner_config if binding is not None else None
        model_value = getattr(runner_config, "model", None)
        allowed = getattr(runner_config, "allowed_tools", None)
        try:
            state.session = choose_session(state.agent_session)
            model = model_value if isinstance(model_value, str) else None
            state.model = choose_model(model, self.config.default_model)
            binding_tools = list(allowed) if isinstance(allowed, list | tuple) else None
            state.tools = scoped_tools(self.config.scoped_allowed_tools, binding_tools)
        except ArgumentError as err:
            refuse(err.code, "adapter_error", ids)
        config = self.channels.get(state.channel_id) if self.channels else None
        state.channel_prompt = config.behavior.prompt if config is not None else ""
        if self.config.channel_skills_dir is not None and config is not None:
            # 26 5.9: read only with the key; loaded only by a host turn, named otherwise.
            found = resolve_channel_skills(
                list(config.behavior.skills),
                self.config.channel_skills_dir,
                self.config.tool_class,
                **ids,
            )
            state.plugin_dirs, state.notice = found.plugin_dirs, found.notice()
        await self._take_locks(state, ids)
        state.context = await self._context(run, ids)
        files_dir = self.config.state_dir / "files" / str(state.thread_id)
        state.file_texts, state.stored_files = await fetch_attachments(
            run.client, state.context.trigger_message.attachments, files_dir
        )
        await self.precheck(state, inv.claims.billing_mode, ids)
        log("run_prepared", **ids)
        return StartOptions(
            model=reported_model(state.model),
            external_session_ref=state.session.ref,
            billing_evidence=state.evidence,
        )

    def _working_folder(self, binding: Any, ids: Mapping[str, Any]) -> Path:
        """The folder of the turn: the binding's, else ``runner.default_workspace``.

        Git is optional (RUNNER-NO-GIT). A refusal names its cause for the thread.
        """
        root = self.config.projects_root
        if binding is not None:
            ref = binding.workspace_ref
            try:
                path = resolve_workspace(root, ref)
            except WorkspaceError as err:
                refuse(err.code, "adapter_error", ids)
            ready, code = readiness(root, ref)
            if not ready:
                refuse(code or "workspace_missing", "adapter_error", ids)
            return path
        default = str(self.config.default_workspace)
        try:
            path = resolve_default(root, default)
        except WorkspaceError as err:
            refuse(err.code, "adapter_error", ids, default_workspace=True)
        ready, code = default_readiness(root, default)
        if not ready:
            refuse(code or "workspace_missing", "adapter_error", ids, default_workspace=True)
        log("default_workspace_used", **ids)
        return path

    def _serves(self, channel_id: UUID) -> bool:
        """True when the channel is in the served cache, i.e. the runner operates it."""
        return self.channels is not None and self.channels.get(channel_id) is not None

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
        """Take the thread lock, the session lock and a slot; report each wait (09 5.2.1)."""
        locks = self.config.state_dir / "locks"
        run_id = state.run_id
        try:
            try:
                for name in (f"thread-{state.thread_id}", f"session-{state.session.ref}"):  # type: ignore[union-attr]
                    state.locks.append(
                        await acquire(
                            locks / f"{name}.lock",
                            self.config.session_lock_wait,
                            self.config.poll_interval,
                            on_busy=lambda: self._lock_wait(state),
                        )
                    )
            except LockTimeout:
                refuse("session_busy", "adapter_error", ids)
            self._phase(state, "preparing")
            if self.slots.full:
                self._wait(run_id, WAIT_SLOTS_FULL)
            await self.slots.acquire()
            state.slot = self.slots
        finally:
            self._wait(run_id, None)

    def _lock_wait(self, state: RunState) -> None:
        """A ``thread_busy`` or ``session_busy`` wait: reported, and phase ``waiting_lock``."""
        self._wait(state.run_id, WAIT_THREAD_BUSY)
        self._phase(state, "waiting_lock")

    def _phase(self, state: RunState, phase: str) -> None:
        """With ``runner.turn_handover`` the record carries the phase before ``run.start``.

        ``preparing`` and ``waiting_lock`` lie before ``run.start`` (4.9): a next runner removes
        such a record and the inbox restart prepares the run again (09 18.19.6).
        """
        if not self.turn_handover:
            return
        write_turn_record(
            self.config.state_dir / "turns" / f"{state.run_id}.json",
            {
                "run_id": str(state.run_id),
                "thread_id": str(state.thread_id),
                "channel_id": str(state.channel_id),
                "phase": phase,
            },
        )

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

    async def precheck(
        self,
        state: RunState,
        desired: str,
        ids: Mapping[str, Any],
        *,
        config: RunnerConfig | None = None,
    ) -> None:
        """The environment and the pre-check ``claude auth status`` of 18.3 (also per restart).

        Args:
            state: The run; its ``env``, ``mode`` and ``evidence`` are set.
            desired: The token's ``billing_mode``.
            ids: IDs for log lines.
            config: The configuration of the turn: a hosted agent's has its own state folder
                (its CLI configuration and the run's scratch place live there); the runner's
                own when omitted.

        Raises:
            RunRefused: ``runtime_not_authenticated``, ``billing_mode_mismatch``,
                ``adapter_error`` or ``runtime_error``.
        """
        cfg = config or self.config
        api_key: Secret | None = None
        config_dir: Path | None = None
        if desired == "organization":
            ref = cfg.anthropic_api_key_ref
            try:
                if ref is None:
                    raise SecretError("missing")
                api_key = resolve(ref)
            except SecretError as err:
                # No key: never a CLI call, never a fallback to a subscription (BR-12).
                log("api_key_unavailable", code=err.code, **ids)
                raise RunRefused(
                    "runtime_not_authenticated", summary=cause("api_key_unavailable")
                ) from None
            config_dir = ensure_private_dir(cfg.state_dir / "claude-config" / "organization")
        elif not (desired == "personal" and cfg.allow_personal):
            refuse("billing_mode_not_allowed", "billing_mode_mismatch", ids)
        try:
            state.env = build_environment(
                self.environ,
                mode=desired,
                api_key=api_key,
                config_dir=config_dir,
                forward_ssh_agent=cfg.forward_ssh_agent,
                # The run's own scratch place, a root of answer attachments (M3-12 note 3 c).
                tmpdir=ensure_private_dir(run_temp_dir(cfg.state_dir, state.run_id)),
            )
        except RunEnvironmentError as err:
            refuse(err.code, "adapter_error", ids)
        if self.claude_bin is None:
            refuse("runtime_unreachable", "runtime_error", ids)
        cwd = state.cwd or cfg.projects_root
        method = await auth_status(self.claude_bin, state.env, cwd, cfg.cli_probe_timeout)
        outcome = decide(desired, method)
        if outcome != "start":
            log("precheck_refused", reason=outcome, **ids)
            raise RunRefused(outcome, summary=cause(outcome))
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
            remove_run_files(self.config.state_dir, state.run_id)
            if self.turn_handover and not self.handing_over:
                # A handover leaves the record of a run in prepare to the next runner, which
                # removes it and starts the run again from the inbox (18.19.6).
                remove_turn_record(self.config.state_dir / "turns" / f"{state.run_id}.json")

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
                active = {str(r) for r in (*self._runs, *self.adopted)}
                sweep(self.config.state_dir, self.config.stream_log_retention, active=active)


class Runner:
    """Start and stop of the runner process (note 3).

    Args:
        config: The configuration.
        environ: The process environment.
        http_transport: An ``httpx`` transport for the SDK client (tests).
        clock: The SDK's time source (tests pass the FakeServer's clock).
        hosted_url: The origin of a hosted agent's server (tests, where each hosted agent is
            the agent of a FakeServer of its own); ``runner.server_url`` when omitted.
    """

    def __init__(
        self,
        config: RunnerConfig,
        environ: Mapping[str, str] | None = None,
        *,
        http_transport: httpx.AsyncBaseTransport | None = None,
        clock: Clock | None = None,
        hosted_url: Callable[[UUID], str] | None = None,
    ) -> None:
        """Create the runner; nothing connects before `run`."""
        self.clock = clock
        self.config = config
        self.handler = RunnerHandler(config, os.environ if environ is None else environ)
        self.http_transport = http_transport
        self.hosted_url = hosted_url
        self.adapter: Adapter | None = None
        self.handed_over = 0
        """The turns the last SIGTERM handed over (exit 75 when above zero, 18.19.3)."""

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
            capabilities=capabilities(
                voice_input=bool(self.config.transcribe_cmd),
                hosts_agents=(
                    self.config.hosted_max_tool_class if self.config.hosted_agents else None
                ),
                turn_handover=self.config.turn_handover,
            ),
            settings=self.config.sdk,
        )
        self.adapter = Adapter(
            adapter_config, self.handler, clock=self.clock, http_transport=self.http_transport
        )
        self.handler.adapter = self.adapter
        cache = ChannelCache(
            self.adapter.client, self.config.projects_root, self.handler.provisioner
        )
        try:
            await cache.load()
        except (ProblemError, httpx.TransportError) as err:
            raise StartError("channels_unavailable") from err
        self.handler.channels = cache
        if self.config.hosted_agents:
            self.handler.hosting = self._hosting(self.adapter, credentials)
        self._startup_files()
        return self.adapter

    def _hosting(self, adapter: Adapter, credentials: Credentials) -> HostedAgents:
        """The hosted set of ``runner.hosted_agents`` (26 3.2), not started yet."""
        config = self.config
        if credentials.dpop is None:  # pragma: no cover - load_config requires the key
            raise StartError("hosted_agents_need_dpop_key")
        handler = self.handler
        root = config.hosted_agents_root
        if root is None:  # pragma: no cover - load_config requires the root
            raise StartError("hosted_agents_need_a_root")
        removed = sweep_turn_files(root)
        if removed:
            warn("hosted_turn_files_swept", count=removed)
        context = HostingContext(
            client=adapter.client,
            dpop=credentials.dpop,
            instance_id=config.instance_id,
            server_url=self.hosted_url or (lambda _agent: config.server_url),
            state_dir=config.state_dir,
            projects_root=config.projects_root,
            ceiling=config.hosted_max_tool_class,
            allow_personal=config.allow_personal,
            modes=billing_modes_supported(
                has_key_ref=config.anthropic_api_key_ref is not None,
                allow_personal=config.allow_personal,
            ),
            probe=lambda: handler.probe,
            capabilities=capabilities(voice_input=bool(config.transcribe_cmd)),
            slots=handler.slots,
            session_lock_wait=config.session_lock_wait,
            poll_interval=config.poll_interval,
            resync_interval=config.channels_resync_interval,
            settings=config.sdk,
            clock=adapter.clock,
            http_transport=self.http_transport,
            runner=handler,
            root=root,
        )
        return HostedAgents(context)

    def _startup_files(self) -> None:
        sweep(self.config.state_dir, self.config.stream_log_retention)

    async def run(self) -> CloseReason:
        """Build the adapter and run it until it stops.

        Returns:
            The SDK's close reason.

        Raises:
            StartError: ``turn_handover_records_left`` when ``runner.turn_handover`` is off and a
                turn record written with it is left (issue 567 item 3).
        """
        if not self.config.turn_handover and handover_records(self.config.state_dir):
            # A turn handed over with the key cannot be continued without it; switch the key
            # off only after `stop --turns` or while no turn runs (issue 567 item 3, 16 10.1).
            raise StartError("turn_handover_records_left")
        adapter = self.adapter or await self.build()
        if self.config.turn_handover:
            write_pid_file(self.config.state_dir)  # for `stop --turns`
        try:
            # Before the first event of the inbox and the first heartbeat (design note 12 d).
            await self.handler.adopt_turns(adapter)
            self.handler.start_resync()
            self.handler.start_trash()
            return await adapter.run()
        finally:
            await self.handler.close()
            if self.config.turn_handover:
                remove_pid_file(self.config.state_dir)

    async def stop(self) -> None:
        """Stop the own turns as in 18.6 (D-AR-1), then the adapter; or hand them over.

        Without ``runner.turn_handover`` a turn is never left behind: each own turn is stopped
        and its run finished ``cancelled`` with ``adapter_stopped`` before the SDK shuts down; a
        turn that starts in between, or one that outlives the wait, is stopped by its run's
        cancellation (`RunTurn.run`). Adopted turns keep running with their records (18.9).

        With the key, SIGTERM hands every live turn over (18.19.3, `hand_over`), unless
        ``claude-code-runner stop --turns`` asked for the stop of every turn: then own and
        adopted turns are stopped as above.
        """
        if self.config.turn_handover and not stop_turns_requested(self.config.state_dir):
            self.handed_over = await hand_over(self.handler, self.adapter)
            return
        if not await self.handler.stop_turns(adopted=self.config.turn_handover):
            warn("stop_turns_timeout")
        if self.adapter is not None:
            await self.adapter.stop()


def _transcriber(
    config: RunnerConfig, environ: Mapping[str, str], path_entries: list[str]
) -> Transcriber:
    """The transcriber of 09 18.14.

    The program is looked up in the cleaned ``PATH`` (a name that is not found stays and fails
    as ``transcriber_unreachable``); the environment is the allow list without a mode part.
    """
    argv = config.transcribe_cmd
    if argv:
        argv = (find_program(argv[0], path_entries) or argv[0], *argv[1:])
    return Transcriber(argv=argv, env=kept_environment(environ), timeout=config.transcribe_timeout)


async def _newest_token(adapter: Adapter, run_id: UUID) -> Secret | None:
    """The newest invocation token of a run from the durable inbox (the previous process's)."""
    entries = [e for e in await adapter.inbox.for_run(run_id) if e.token is not None]
    entries.sort(key=lambda e: e.token_iat or 0)
    return entries[-1].token if entries else None


def _state_of(
    record: Mapping[str, Any], run_id: UUID, thread_id: UUID, channel_id: UUID
) -> RunState:
    """The prepared state of a run from its record with ``runner.turn_handover`` (18.19.6).

    The record carries what a next runner needs to build the command line again (model,
    tools, billing mode, plugin folders, whether files were stored), never the environment;
    the pre-check before a launch builds that again (18.9).
    """
    state = RunState(run_id, thread_id, channel_id)
    if record.get("cwd"):
        state.cwd = Path(str(record["cwd"]))
    state.session = SessionChoice(str(record.get("session_ref")), resume=True, new=False)
    state.session_mode = str(record.get("session_mode") or "resume")
    run = record.get("run")
    run = run if isinstance(run, Mapping) else {}
    state.model = str(run.get("model") or "")
    tools = run.get("tools")
    state.tools = [str(t) for t in tools] if isinstance(tools, list) else []
    state.mode = str(run.get("mode") or "unknown")
    dirs = run.get("plugin_dirs")
    state.plugin_dirs = tuple(Path(str(d)) for d in dirs) if isinstance(dirs, list) else ()
    stored = run.get("stored_files")
    state.stored_files = stored if isinstance(stored, int) and not isinstance(stored, bool) else 0
    return state


def end_in_record_or_log(record: Mapping[str, Any]) -> bool:
    """Whether a turn record carries the end of its run (``result_in_log`` of the decision).

    A decided end in the record (design note 12 f 2) counts like a ``result`` in the stream
    file after the stored offset.

    Args:
        record: The turn record.

    Returns:
        True when the run can be finished from the record or the log.
    """
    if record.get("finishing"):
        return True
    return _result_after(Path(str(record.get("stream", ""))), int(record.get("offset") or 0))


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
