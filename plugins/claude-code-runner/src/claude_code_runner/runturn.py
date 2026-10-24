# SPDX-License-Identifier: Apache-2.0
"""One started run: its turn processes, follow-ups, stop and error paths (09 18.2 to 18.9).

`RunTurn.run` starts the turn of a run and follows it to the end. Every process end goes through
`claude_code_runner.failures.classify_turn_end` and `claude_code_runner.recovery.next_action`; a
full context is compacted once and the prompt runs again, a missing session is replaced by a new
one (fail open), an existing session at the first turn is resumed, and every other end finishes
the run once, after the process exited (design notes 5 to 11). Every process passes the pre-check
and the counter-check of M2-34 (I3 per process). Follow-ups are written to stdin while it is
open (`RunTurn.offer_followup`), a stop goes through `RunTurn.request_stop`, and a run that the
server ended stops its turn without a finish. `RunTurn.adopted` follows a turn that outlived an
earlier runner (design note 12).

The turn record ``turns/<run_id>.json`` is written atomically before the action that depends on
it: the reference before its process starts, the offset after the patches of its lines.

Examples:
    >>> RunState.__name__
    'RunState'
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import logging
import time
from collections.abc import Mapping
from dataclasses import dataclass, field
from datetime import UTC, datetime
from pathlib import Path
from typing import TYPE_CHECKING, Any
from uuid import UUID, uuid4

import httpx
from bothy_sdk.adapter import Invocation, RunRefused
from bothy_sdk.context import render_context
from bothy_sdk.errors import (
    ConflictError,
    NotFoundError,
    ProblemError,
    RateLimitedError,
    ServerError,
    UnauthenticatedError,
    ValidationFailed,
    VersionConflict,
)
from bothy_sdk.generated.api.agent_runtime import finish_run, report_agent_session
from bothy_sdk.generated.api.messages import get_message
from bothy_sdk.generated.models import (
    AgentSession,
    AgentSessionReport,
    AgentSessionReportBindFailed,
    BillingEvidence,
    CostReport,
    JsonPatchOperation,
    JsonPatchOperationOp,
    Message,
    RunContext,
    RunFinish,
    RunFinishState,
    RunMetrics,
)
from bothy_sdk.generated.types import Unset
from bothy_sdk.outbox import OutboxState
from bothy_sdk.runs import RunHandle, is_run_ended

from claude_code_runner.adoption import read_identity
from claude_code_runner.commandline import SessionChoice, build_argv
from claude_code_runner.failures import (
    TurnEnd,
    TurnEvidence,
    api_error_text,
    classify_turn_end,
)
from claude_code_runner.followups import (
    FollowupCheck,
    FollowupVerdict,
    PendingFollowups,
    answered_inputs,
    build_followup_line,
    followup_verdict,
    names_input,
    stdin_close_due,
)
from claude_code_runner.livebox import LiveBox
from claude_code_runner.locks import LockTimeout, acquire, release
from claude_code_runner.prompt import context_items, system_prompt, user_line
from claude_code_runner.recovery import (
    RETRY_HINT,
    Cancelled,
    Compact,
    Fail,
    Finish,
    NewSession,
    RecoveryState,
    ResumeSameReference,
    Retry,
    box_note,
    next_action,
)
from claude_code_runner.statedir import ensure_private_dir, write_private
from claude_code_runner.stop import StoppableGroup
from claude_code_runner.streamreader import StreamReader
from claude_code_runner.title import propose_title, split_title
from claude_code_runner.turn import (
    Turn,
    TurnPaths,
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

if TYPE_CHECKING:
    from claude_code_runner.app import RunnerHandler

__all__ = ["COMPACT_IN_PRINT_MODE", "RunState", "RunTurn"]

_LOG = logging.getLogger("claude_code_runner")
_EXPECTED_KEY_SOURCE = {"organization": "ANTHROPIC_API_KEY", "personal": "none"}
# The currency of the runtime's costUSD; a cost baseline in another currency is not subtracted.
_RUNTIME_CURRENCY = "USD"

COMPACT_IN_PRINT_MODE = True
"""``/compact`` is taken as an input line in print mode by the pinned CLI (design note 8 a):
the command ``compact`` of Claude Code 2.1.259 has ``supportsNonInteractive: true`` (file of the
installed pinned CLI) and the Agent SDK page on slash commands sends ``/compact`` as a prompt
(https://code.claude.com/docs/en/agent-sdk/slash-commands, retrieved 2026-10-24)."""


def _bound_baseline(session: AgentSession | None) -> dict[str, Any] | None:
    """The cost baseline of the bound session row with the reference it belongs to (09 18.7)."""
    baseline = getattr(session, "cost_baseline_micros", None)
    ref = getattr(session, "external_session_ref", None)
    if not isinstance(baseline, int) or isinstance(baseline, bool) or baseline <= 0:
        return None
    if not isinstance(ref, str) or not ref:
        return None
    currency = getattr(session, "currency", None)
    return {
        "ref": ref,
        "micros": baseline,
        "currency": currency if isinstance(currency, str) else "",
    }


def _stored_baseline(raw: object) -> dict[str, Any] | None:
    """The cost baseline of a turn record, as `_bound_baseline` wrote it."""
    if not isinstance(raw, Mapping):
        return None
    micros, ref, currency = raw.get("micros"), raw.get("ref"), raw.get("currency")
    if not isinstance(micros, int) or isinstance(micros, bool) or micros <= 0:
        return None
    if not isinstance(ref, str) or not isinstance(currency, str):
        return None
    return {"ref": ref, "micros": micros, "currency": currency}


def _log(event: str, **ids: object) -> None:
    _LOG.info(json.dumps({"event": event, **{k: str(v) for k, v in ids.items()}}))


def _warn(event: str, **ids: object) -> None:
    _LOG.warning(json.dumps({"event": event, **{k: str(v) for k, v in ids.items()}}))


@dataclass
class RunState:
    """What prepare took for one run; `close` gives it back (one object per run).

    Attributes:
        run_id: The run.
        thread_id: Its thread.
        channel_id: Its channel.
        locks: The thread lock and the session lock (descriptors).
        slot: The slot of ``runner.max_parallel_runs``.
        cwd: The resolved project folder.
        session: The Claude Code session of the first process.
        session_mode: ``new``, ``resume`` or ``bind`` of the payload (09 9.1).
        agent_session: The payload's agent session row, if any.
        model: The model.
        tools: The tools of ``scoped``.
        env: The run environment (rebuilt by every pre-check).
        mode: The proven billing mode.
        evidence: The pre-check's evidence.
        context: The run context.
        file_texts: Transcripts of attachments.
        stored_files: How many attachments were stored.
        channel_prompt: ``behavior.prompt``.
        held: Follow-ups that arrived before the turn started, in event order (note 4 c).
        turn: The turn once ``execute`` started it.
        stop_reason: The reason of a stop request that came before the turn object existed.
    """

    run_id: UUID
    thread_id: UUID
    channel_id: UUID
    locks: list[int] = field(default_factory=list)
    slot: asyncio.Semaphore | None = None
    cwd: Path | None = None
    session: SessionChoice | None = None
    session_mode: str = "new"
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
    held: list[Invocation] = field(default_factory=list)
    turn: RunTurn | None = None
    stop_reason: str | None = None

    def close(self) -> None:
        """Release the slot and the locks (locks in reverse order)."""
        for fd in reversed(self.locks):
            release(fd)
        self.locks.clear()
        if self.slot is not None:
            self.slot.release()
            self.slot = None


@dataclass(frozen=True)
class _Attempt:
    """One process of a run: a turn or the compaction."""

    kind: str = "turn"
    resume: bool = False
    hint: bool = False
    new_reference: bool = False
    bind_failed: bool = False


@dataclass
class _Outcome:
    """What one process left behind."""

    evidence: TurnEvidence
    worked: bool
    handled: bool = False
    compacted: bool = False
    """The process wrote ``system/compact_boundary`` (a compaction that reduced the context)."""


class RunTurn:
    """The turn processes of one started run (see the module docstring).

    Args:
        handler: The runner handler (configuration, pre-check, CLI path).
        state: The run's prepared state.
        run: The run handle of the SDK.
    """

    def __init__(self, handler: RunnerHandler, state: RunState, run: RunHandle) -> None:
        """Create the turn of a started run; nothing starts before `run`."""
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
        session = state.agent_session
        started = session is not None and session.started is True
        self.title_request = not started
        self.report_due = not started
        self.session_version = str(session.version) if session is not None else "1"
        self.cost_baseline = _bound_baseline(session)
        self.recovery = RecoveryState(compact_supported=COMPACT_IN_PRINT_MODE)
        self.followups = PendingFollowups()
        self.process: Turn | None = None
        self.phase = "starting"  # starting, open, closed, ended
        self.attempt = _Attempt()
        self.stop_reason: str | None = state.stop_reason
        self.server_ended = False
        self.last_result: dict[str, Any] | None = None
        self.reply_problem: str | None = None
        self.title = ""
        self.replies = 0
        self.prompt_line = ""
        self.own_uuid = ""
        self.write_lock = asyncio.Lock()
        self.live: LiveBox | None = None
        self.offset = 0
        self.answered: set[int] = set()
        # The input lines of the current turn process with their ``uuid``, in write order.
        self.inputs: list[tuple[str, bytes]] = []
        self.answered_uuids: set[str] = set()
        self.inputs_named = False
        """A result of the run named one of its inputs (the CLI names them, D-M2-35-19)."""
        self.retry_inputs: list[tuple[str, bytes]] | None = None
        self.stopping: asyncio.Task[None] | None = None
        self.adopted_group: StoppableGroup | None = None
        self.adopted_record: dict[str, Any] | None = None
        self._identity: dict[str, str] = {}

    # ---- the run ---------------------------------------------------------------------------

    def _paths(self) -> TurnPaths:
        return turn_paths(self.cfg.state_dir, self.session.ref, str(self.handle.run_id))

    async def run(self) -> None:  # noqa: PLR0911, PLR0912, PLR0915 - one branch per action
        """Start the first process, follow every process and end the run once."""
        self.state.turn = self
        paths = self._paths()
        ensure_private_dir(paths.stream.parent)
        context = self.state.context
        if context is None:  # pragma: no cover - prepare always loads it
            raise RuntimeError("no context")
        rendered = render_context(context, file_texts=self.state.file_texts)
        self.prompt_line, self.own_uuid = user_line(context_items(rendered))
        write_private(paths.input, self.prompt_line + "\n")
        usage = UsageStore(self.cfg.state_dir / "sessions" / self.session.ref)
        self.live = LiveBox(
            self.handle.patch,
            own_uuid=self.own_uuid,
            title_requested=self.title_request,
            context_windows=usage.load().context_windows,
            now=lambda: datetime.now(UTC),
        )
        attempt = _Attempt(resume=self.session.resume)
        worked_before = False
        first = True
        try:
            while True:
                if self.stop_reason is not None:
                    await self._finish_cancelled()
                    return
                if not first and not await self._precheck_again():
                    return
                if self._stop_requested():  # a stop during the pre-check starts nothing
                    await self._finish_cancelled()
                    return
                first = False
                self.attempt = attempt
                outcome = await self._run_process(attempt)
                if outcome is None or outcome.handled:
                    return
                if self.server_ended:
                    self._ended_by_server()
                    return
                end = classify_turn_end(outcome.evidence)
                if (
                    attempt.kind == "compaction"
                    and end.outcome == "completed"
                    and not outcome.compacted
                ):
                    # The pinned CLI ends a failed compaction with a success result too.
                    end = TurnEnd("runtime_error", exit_label=end.exit_label)
                worked = worked_before if attempt.kind == "compaction" else outcome.worked
                action = next_action(
                    end,
                    self.recovery,
                    self.state.session_mode,
                    worked=worked,
                    process=attempt.kind,
                    stop_reason=self.stop_reason,
                )
                if isinstance(action, Finish):
                    await self._complete()
                    return
                if isinstance(action, Cancelled):
                    await self._finish_cancelled()
                    return
                if isinstance(action, Fail):
                    if attempt.kind == "compaction":
                        await self._box_note("context_compacted", "failed", None)
                    await self._fail(action.reason, action.errors, end.exit_label)
                    return
                next_attempt = await self._recover(action, outcome)
                if next_attempt is None:
                    return
                if isinstance(action, Compact):
                    worked_before = outcome.worked
                attempt = next_attempt
        except asyncio.CancelledError:
            # A runtime never outlives the end of its run (09 1 point 4).
            if self.process is not None:
                await terminate(
                    self.process, self.cfg.stop_grace, self.cfg.kill_grace, self.cfg.poll_interval
                )
            raise
        finally:
            self.phase = "ended"
            remove_turn_record(paths.record)

    def _stop_requested(self) -> bool:
        """Whether a stop came (it can arrive at every ``await`` of the run)."""
        return self.stop_reason is not None

    async def _recover(self, action: Any, outcome: _Outcome) -> _Attempt | None:
        """The next process of a recovery (design notes 8 and 9)."""
        _log("recovery_started", action=type(action).__name__, **self.ids)
        if self.attempt.kind == "turn":
            # The inputs of the failed turn that no result answered go to the next turn.
            self.retry_inputs = [i for i in self.inputs if i[0] not in self.answered_uuids]
        if isinstance(action, Compact):
            self.recovery = self.recovery.with_(compacted=True)
            return _Attempt(kind="compaction", resume=True)
        if isinstance(action, Retry) and not self.retry_inputs:
            # Every input was answered before the context filled up: nothing to run again.
            await self._box_note("context_compacted", "completed", None)
            await self._complete()
            return None
        if isinstance(action, Retry):
            await self._box_note("context_compacted", "completed", None)
            return _Attempt(resume=True, hint=action.hint)
        if isinstance(action, ResumeSameReference):
            self.recovery = self.recovery.with_(session_resumed=True)
            return _Attempt(resume=True)
        if isinstance(action, NewSession):
            self.recovery = self.recovery.with_(session_replaced=True)
            if not await self._new_reference():
                await self._fail("adapter_error", (), outcome.evidence.exit_label)
                return None
            await self._box_note("session_missing", "completed", {"mode": self.state.session_mode})
            return _Attempt(resume=False, new_reference=True, bind_failed=action.bind_failed)
        raise AssertionError(action)  # pragma: no cover - every action is handled

    async def _new_reference(self) -> bool:
        """A new session UUID; the session lock moves to it, the thread lock stays (9 a)."""
        ref = str(uuid4())
        locks = self.cfg.state_dir / "locks"
        try:
            fd = await acquire(
                locks / f"session-{ref}.lock", self.cfg.session_lock_wait, self.cfg.poll_interval
            )
        except LockTimeout:
            _log("session_busy", **self.ids)
            return False
        if len(self.state.locks) > 1:
            release(self.state.locks.pop())
        self.state.locks.append(fd)
        UsageStore(self.cfg.state_dir / "sessions" / ref).start_without_baseline()
        self.session = SessionChoice(ref=ref, resume=False, new=True)
        self._persist()
        return True

    async def _precheck_again(self) -> bool:
        """The pre-check of M2-34 before every further process (I3 per process)."""
        try:
            await self.h.precheck(self.state, self.state.mode, self.ids)
        except RunRefused as refused:
            await self._fail(refused.reason or "runtime_error", (), "not_started")
            return False
        return True

    # ---- one process -----------------------------------------------------------------------

    def _prompt_file(self, attempt: _Attempt) -> Path:
        paths = self._paths()
        text = system_prompt(
            title_request=self.title_request, channel_prompt=self.state.channel_prompt
        )
        if attempt.hint:
            path = paths.system_prompt.with_name(f"{self.handle.run_id}.retry-prompt.md")
            write_private(path, text.rstrip() + "\n\n## Retry\n\n" + RETRY_HINT + "\n")
            return path
        write_private(paths.system_prompt, text)
        return paths.system_prompt

    async def _run_process(self, attempt: _Attempt) -> _Outcome | None:
        paths = self._paths()
        ensure_private_dir(paths.stream.parent)
        prompt_file = self._prompt_file(attempt)
        add_dir = None
        if self.cfg.tool_class in ("scoped", "host") and self.state.stored_files:
            add_dir = self.cfg.state_dir / "files" / str(self.state.thread_id)
        session = SessionChoice(ref=self.session.ref, resume=attempt.resume, new=self.session.new)
        argv = build_argv(
            claude_bin=self.h.claude_bin or self.cfg.claude_bin,
            model=self.state.model,
            session=session,
            tool_class=self.cfg.tool_class,
            tools=self.state.tools,
            add_dir=add_dir,
            system_prompt_file=prompt_file,
        )
        stream_start = _size(paths.stream)
        stderr_start = _size(paths.stderr)
        cwd = self.state.cwd or self.cfg.projects_root
        try:
            process = Turn.start(
                argv, cwd=cwd, env=self.state.env, paths=paths, pass_fds=self.state.locks
            )
        except OSError:
            _warn("turn_start_failed", **self.ids)
            await self._fail("runtime_error", (), "start_failed")
            return None
        self.process = process
        self.offset = stream_start
        identity = await asyncio.to_thread(read_identity, process.pid, self.cfg.cli_probe_timeout)
        self._identity = (
            {"lstart": identity.lstart, "command": identity.command} if identity else {}
        )
        self._persist()
        _log("turn_started", kind=attempt.kind, **self.ids)
        if attempt.kind == "compaction":
            await process.write_line(user_line(["/compact"])[0])
            process.close_stdin()
            self.phase = "starting"  # follow-ups wait for the retry (note 4 c)
        else:
            await self._write_inputs(process)
        try:
            return await self._follow(process, attempt, stream_start, stderr_start)
        finally:
            self.process = None
            if self.phase != "ended":
                self.phase = "starting"

    async def _write_inputs(self, process: Turn) -> None:
        """The inputs of a turn process, then the held follow-ups (notes 4 c and d, 8 b).

        The first process gets the prompt line. A process of a recovery gets the inputs of the
        failed process that no ``result`` answered, in their order and with their original
        ``uuid``: the prompt line while it was not answered, then the follow-ups after it
        (D-M2-35-19).
        """
        inputs = self.retry_inputs
        if inputs is None:
            inputs = [(self.own_uuid, self.prompt_line.encode("utf-8") + b"\n")]
        async with self.write_lock:
            self.inputs = []
            self.answered_uuids = set()
            for line_uuid, line in inputs:
                await process.write_bytes(line)
                self.inputs.append((line_uuid, line))
            self.phase = "open"
        held, self.state.held = self.state.held, []
        for inv in held:
            await self.offer_followup(inv)

    async def _follow(  # noqa: PLR0912, PLR0915 - one branch per message kind of 18.4
        self, process: Turn, attempt: _Attempt, stream_start: int, stderr_start: int
    ) -> _Outcome:
        reader = StreamReader(
            process.paths.stream,
            self.cfg.poll_interval,
            process.alive,
            offset=stream_start,
            until_exit=True,
        )
        init_seen = worked = compacted = False
        last_result: dict[str, Any] | None = None
        assistant_text = ""
        last_result_at: float | None = None
        saved_at = time.monotonic()
        async for message in reader.items():
            if message is None:
                await self._idle(process, last_result_at)
                if (
                    time.monotonic() - saved_at
                    >= self.handle.patch.settings.run_update_min_interval
                ):
                    saved_at = time.monotonic()
                    await self._save_offset(reader.offset)
                continue
            kind, subtype = message.get("type"), message.get("subtype")
            if kind == "system" and subtype == "init":
                init_seen = True
                if message.get("apiKeySource") != _EXPECTED_KEY_SOURCE.get(self.state.mode):
                    _warn("counter_check_failed", **self.ids)
                    await terminate(
                        process, self.cfg.stop_grace, self.cfg.kill_grace, self.cfg.poll_interval
                    )
                    await self._finish("failed", reason="billing_mode_mismatch")
                    return _Outcome(TurnEvidence(), worked, handled=True)
                await self._report_session(message, attempt)
            elif kind == "assistant":
                error_text = api_error_text(message)
                if error_text is not None:
                    assistant_text = error_text
                # An API error the CLI writes as an assistant message is no work (note 8 b).
                worked = worked or (init_seen and error_text is None)
            elif kind == "system" and subtype == "compact_boundary":
                compacted = True
            elif kind == "user" and message.get("isReplay") is True:
                await self._echo(message.get("uuid"))
            elif kind == "result":
                last_result = message
                self.last_result = message
                last_result_at = time.monotonic()
                if attempt.kind == "turn" and _success(message):
                    order = [u for u, _ in self.inputs]
                    self.answered_uuids = answered_inputs(
                        order, self.answered_uuids, message, named_before=self.inputs_named
                    )
                    self.inputs_named = self.inputs_named or names_input(order, message)
                    await self._answer(message, reader.line_offset)
            if attempt.kind == "turn" and self.live is not None:
                self.live.apply(message)
                if self.live.steps_changed:
                    self._persist()  # the step ids before their patches go out (note 12 b)
            if kind == "result" and not _success(message):
                # A failed turn absorbs nothing more: stdin closes now, and the follow-ups
                # without echo go to the next process of a recovery (notes 4 d and 5).
                await self._close_stdin(process)
            elif kind == "result":
                await self._idle(process, last_result_at)
        await terminate(
            process, self.cfg.stop_grace, self.cfg.kill_grace, self.cfg.poll_interval, self.ids
        )
        if self.stopping is not None:
            await self.stopping
        self.phase = "closed"
        await self._save_offset(reader.offset)
        evidence = TurnEvidence(
            stop_requested=self.stop_reason is not None,
            init_seen=init_seen,
            result=last_result,
            assistant_text=assistant_text,
            stderr=_read_from(process.paths.stderr, stderr_start),
            exit_label=process.exit_label(),
        )
        return _Outcome(evidence, worked, compacted=compacted)

    async def _idle(self, process: StoppableGroup, last_result_at: float | None) -> None:
        """Checks while a turn runs: a run the server ended, and the stdin rule of note 5."""
        if self.handle.patch.ended and not self.server_ended and self.stop_reason is None:
            self.server_ended = True
            _log("run_ended_by_server", **self.ids)
        if (self.server_ended or self.stop_reason is not None) and self.stopping is None:
            # Also a stop that came while this process was being started (note 6 d).
            self._stop_group(process)
            return
        pending = len(self.followups.pending())
        if self.phase == "open" and stdin_close_due(
            time.monotonic(), last_result_at, pending, self.cfg.followup_grace
        ):
            await self._close_stdin(process)

    async def _close_stdin(self, process: StoppableGroup) -> None:
        async with self.write_lock:
            if self.phase == "open":
                self.phase = "closed"
            process.close_stdin()

    def _stop_group(self, group: StoppableGroup) -> None:
        if self.stopping is not None:
            return
        self.phase = "closed"
        self.stopping = asyncio.ensure_future(self._stop(group))

    async def _stop(self, group: StoppableGroup) -> None:
        from claude_code_runner.stop import stop_group  # noqa: PLC0415 - keeps the import list flat

        await stop_group(
            group, self.cfg.stop_grace, self.cfg.kill_grace, self.cfg.poll_interval, self.ids
        )

    # ---- stop ------------------------------------------------------------------------------

    async def request_stop(self, reason: str) -> None:
        """A stop of the run (note 6): SIGINT and escalation now, the finish after the exit.

        A second request changes nothing; after a request no restart, compaction or follow-up
        write starts.

        Args:
            reason: The request's reason, unchanged.
        """
        if self.stop_reason is not None:
            return
        self.stop_reason = reason
        self._persist()
        group = self.process or self.adopted_group
        if group is not None:
            self._stop_group(group)

    def _ended_by_server(self) -> None:
        # The server ended the run (409 conflict of an ended run, or 401): no finish (6 e). The
        # SDK finishes a run whose handler returns unless it is marked finished.
        self.handle.finished = True
        _log("run_finished", state="ended_by_server", **self.ids)

    # ---- follow-ups ------------------------------------------------------------------------

    async def offer_followup(self, inv: Invocation) -> None:  # noqa: PLR0911 - one exit per check
        """Write a follow-up now, hold it, or leave it (design notes 2 to 4).

        Args:
            inv: The ``agent.followup`` invocation (its claims were verified at acceptance).
        """
        message_id = getattr(inv.payload, "message_id", None)
        if not isinstance(message_id, UUID):
            return
        ids = {**self.ids, "message_id": message_id}
        async with self.write_lock:
            if self.stop_reason is not None or self.phase in ("closed", "ended"):
                _log("followup_not_deliverable", **ids)
                return
            if self.phase == "starting":
                if message_id not in self.followups and all(
                    getattr(h.payload, "message_id", None) != message_id for h in self.state.held
                ):
                    self.state.held.append(inv)
                _log("followup_held", **ids)
                return
            if message_id in self.followups:
                return
            message = await self._read_message(message_id, ids)
            if message is None:
                return
            process = self.process
            check = FollowupCheck(
                claims=inv.claims,
                now=self.handle.client.clock.now().timestamp(),
                leeway=self.cfg.sdk.invocation_token_leeway,
                turn_started=process is not None and self.phase == "open",
                run_id=self.handle.run_id,
                thread_id=self.state.thread_id,
                billing_mode=self.state.mode,
                tool_class=self.cfg.tool_class,
                message_id=message_id,
                author_id=message.author.id,
                message_thread_id=message.thread_id,
                written=set(self.followups.by_message),
            )
            verdict, code = followup_verdict(check)
            if verdict is FollowupVerdict.SKIP:
                _warn(code, **ids)
                return
            if verdict is not FollowupVerdict.WRITE or process is None:
                return
            author = message.author.to_dict()
            line_uuid = str(uuid4())
            line = build_followup_line(author, message.content_md, line_uuid)
            self.followups.add(message_id, line_uuid, line)
            if not await process.write_bytes(line):
                del self.followups.by_message[message_id]
                _log("followup_not_deliverable", **ids)
                return
            self.inputs.append((line_uuid, line))
            self._persist()
        _log("followup_written", **ids)
        await self._followup_state(message_id, "delivered")
        if self.followups.delivered(message_id):
            await self._followup_state(message_id, "absorbed")

    async def _read_message(self, message_id: UUID, ids: Mapping[str, Any]) -> Message | None:
        try:
            message = await self.handle.client.call_as(
                Message, get_message.asyncio_detailed, message_id=message_id
            )
        except NotFoundError:
            message = None
        except UnauthenticatedError:
            self.server_ended = True
            return None
        except (ProblemError, httpx.TransportError) as err:
            _log("followup_read_failed", code=getattr(err, "code", "transport_error"), **ids)
            return None
        if (
            message is None
            or not isinstance(message.deleted_at, Unset | type(None))
            or not message.content_md.strip()
        ):
            _log("followup_no_text", **ids)
            return None
        return message

    async def _echo(self, line_uuid: object) -> None:
        item = self.followups.echo(line_uuid)
        if item is not None:
            await self._followup_state(item.message_id, "absorbed")
        self._persist()

    async def _followup_state(self, message_id: UUID, state: str) -> None:
        patch = self.handle.patch
        try:
            if state == "delivered":
                patch.followup_delivered(message_id)
            else:
                patch.followup_absorbed(message_id)
            await patch.flush()
        except ValueError:
            return  # already further: forward only
        except ValidationFailed:
            _log("followup_patch_refused", message_id=message_id, **self.ids)
        except ProblemError as err:
            if is_run_ended(err):
                self.server_ended = True
            _log("followup_patch_failed", code=err.code, message_id=message_id, **self.ids)
        except httpx.TransportError:
            _log("followup_patch_failed", code="transport_error", **self.ids)

    # ---- answers, notes, session report ----------------------------------------------------

    async def _answer(self, result: Mapping[str, Any], offset: int) -> None:
        """One answer per ``result`` without error (note 5); a line read again is skipped."""
        if offset in self.answered or self._stop_requested():
            return  # no answer after a stop (note 6 b)
        text = result.get("result") if isinstance(result.get("result"), str) else ""
        title, reply = split_title(str(text), self.title_request and self.replies == 0)
        self.replies += 1
        if title and not self.title:
            self.title = title
        if reply.strip():
            problem = await self._deliver(reply)
            if problem is not None and self.reply_problem is None:
                _log("reply_refused", code=problem, **self.ids)
                self.reply_problem = problem
        self.answered.add(offset)
        self._persist()

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

    async def _box_note(self, code: str, status: str, detail: Mapping[str, Any] | None) -> None:
        """One ``add /steps/<id>`` of a box note, after the pending patches (note 11)."""
        value = box_note(code, status, detail, datetime.now(UTC))
        op = JsonPatchOperation(op=JsonPatchOperationOp.ADD, path=f"/steps/{value['id']}")
        op.value = value
        patch = self.handle.patch
        try:
            with contextlib.suppress(ProblemError):
                await patch.flush()
            await patch.send_ops([op])
        except ProblemError as err:
            if is_run_ended(err):
                self.server_ended = True
            _log("box_note_failed", code=err.code, note=code, **self.ids)
        except httpx.TransportError:
            _log("box_note_failed", code="transport_error", note=code, **self.ids)

    async def _report_session(  # noqa: PLR0911 - one exit per answer
        self, init: Mapping[str, Any], attempt: _Attempt
    ) -> None:
        """``agent_session.report`` with ``started = true`` (18.4), or the new reference (9 a)."""
        if init.get("session_id") != self.session.ref:
            return
        if not attempt.new_reference and not self.report_due:
            return
        self.report_due = False
        body = AgentSessionReport(
            external_session_ref=self.session.ref, model=self.state.model, started=True
        )
        if attempt.bind_failed:
            body.bind_failed = AgentSessionReportBindFailed.SESSION_MISSING
        version = self.session_version
        for _ in range(2):
            try:
                row = await self.handle.client.call_as(
                    AgentSession,
                    report_agent_session.asyncio_detailed,
                    thread_id=self.state.thread_id,
                    body=body,
                    if_match=f'"{version}"',
                )
                self.session_version = str(row.version)
                return
            except VersionConflict as err:
                current = err.current or {}
                if not attempt.new_reference and current.get("external_session_ref") != (
                    self.session.ref
                ):
                    _log("session_report_failed", code=err.code, **self.ids)
                    return
                version = str(current.get("version"))
            except UnauthenticatedError:
                self.server_ended = True
                return
            except ProblemError as err:
                _log("session_report_failed", code=err.code, **self.ids)
                return
            except httpx.TransportError:
                _log("session_report_failed", code="transport_error", **self.ids)
                return
        _log("session_report_failed", code="version_conflict", **self.ids)

    # ---- the end ---------------------------------------------------------------------------

    async def _complete(self) -> None:
        reports, metrics = self._usage()
        if self.reply_problem is not None:
            await self._finish("failed", reason="adapter_error", metrics=metrics, costs=reports)
            return
        if self.title_request and self.title:
            outcome = await propose_title(self.handle.client, self.state.thread_id, self.title)
            _log("title_proposed", outcome=outcome, **self.ids)
        await self._finish("completed", metrics=metrics, costs=reports)

    async def _fail(self, reason: str, errors: tuple[str, ...], exit_label: str) -> None:
        """``failed`` with ``reason``; for ``runtime_error`` the box note with the texts first."""
        if reason == "runtime_error":
            detail = {"errors": list(errors), "exit": exit_label}
            await self._box_note("runtime_error", "failed", detail)
        reports, metrics = self._usage()
        await self._finish("failed", reason=reason, metrics=metrics, costs=reports)

    async def _finish_cancelled(self) -> None:
        """``cancelled`` with the request's reason; no answer (note 6 b)."""
        reports, metrics = self._usage()
        await self._finish(
            "cancelled", reason=self.stop_reason or "user_cancelled", metrics=metrics, costs=reports
        )

    def _usage(self) -> tuple[list[CostReport], RunMetrics | None]:
        """Cost reports and metrics from the run's last ``result`` (none without one)."""
        result = self.last_result
        if result is None:
            return [], None
        store = UsageStore(self.cfg.state_dir / "sessions" / self.session.ref)
        reports = store.pending_for(str(self.handle.run_id))
        if reports is None:
            stored = store.load()
            current = parse_model_usage(result)
            reports = cost_reports(difference(stored.values, current))
            remaining = stored.baseline_remaining
            if remaining is None and not stored.values:
                remaining = self._start_baseline()
            if remaining is not None:
                if counters_reset(stored.values, current):
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
        live = self.live
        if live is not None:
            metrics.subagents = live.subagents
            metrics.background_jobs = live.background_jobs
            metrics.api_retries = live.api_retries
            if live.context_pct is not None:
                metrics.context_pct = live.context_pct
        return reports, metrics

    def _start_baseline(self) -> int | None:
        """The baseline of a session whose values the runner never stored (09 18.7, RK-75).

        ``cost_baseline_micros`` of the bound session row when it is above 0 and the run still
        works on that row's reference: a session that replaced it (session_missing) starts
        without one. A baseline in another currency than the runtime's US dollars is ignored
        with a warning (no conversion). ``None`` when the session has none.
        """
        baseline = self.cost_baseline
        if baseline is None or baseline["ref"] != self.session.ref:
            return None
        if baseline["currency"] != _RUNTIME_CURRENCY:
            _warn("cost_baseline_currency_ignored", **self.ids)
            return 0
        return int(baseline["micros"])

    async def _finish(self, state: str, **kwargs: Any) -> None:
        """``finishRun`` once; a lost answer is sent again with the same body (D-M2-34-4)."""
        if kwargs.get("metrics") is None:
            kwargs.pop("metrics", None)
        try:
            await self.handle.finish(state, **kwargs)
        except ConflictError as err:
            # invalid_state_transition: the server ended the run already; never retried.
            _log("finish_refused", code=err.code, **self.ids)
        except (ServerError, RateLimitedError, httpx.TransportError):
            await self._finish_again(state, kwargs)
        except ProblemError as err:
            _log("finish_refused", code=err.code, **self.ids)
        _log("run_finished", state=state, reason=kwargs.get("reason", ""), **self.ids)

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
            except (ServerError, RateLimitedError, httpx.TransportError):
                continue
            except ProblemError as err:
                _log("finish_refused", code=err.code, **self.ids)
                return
        _warn("finish_not_stored", **self.ids)

    # ---- the turn record -------------------------------------------------------------------

    def _persist(self) -> None:
        """Write the turn record atomically (mode ``0600``, never a token or a text)."""
        if self.phase == "ended":
            return
        process = self.process
        paths = self._paths()
        record: dict[str, Any] = {
            "run_id": str(self.handle.run_id),
            "thread_id": str(self.state.thread_id),
            "channel_id": str(self.state.channel_id),
            "session_ref": self.session.ref,
            "session_mode": self.state.session_mode,
            "kind": self.attempt.kind,
            "started_at": time.time(),
            "stream": str(paths.stream),
            "stderr": str(paths.stderr),
            "system_prompt": str(paths.system_prompt),
            "input": str(paths.input),
            "offset": self.offset,
            "answered": sorted(self.answered),
            "followups": self.followups.to_record(),
            "recovery": {
                "compacted": self.recovery.compacted,
                "session_replaced": self.recovery.session_replaced,
                "session_resumed": self.recovery.session_resumed,
            },
            "stop": self.stop_reason,
            "title_request": self.title_request,
            "replies": self.replies,
            "title": self.title,
            "session_version": self.session_version,
            "cost_baseline": self.cost_baseline,
            "steps": self.live.state() if self.live is not None else None,
        }
        if process is not None:
            record.update(pid=process.pid, pgid=process.pgid, identity=self._identity)
        elif self.adopted_record is not None:
            for key in ("pid", "pgid", "identity", "started_at"):
                record[key] = self.adopted_record.get(key)
        write_turn_record(paths.record, record)

    async def _save_offset(self, offset: int) -> None:
        """Store the offset after the patches of the lines before it went out.

        A patch that failed keeps the old offset: a runner that adopts the turn reads those
        lines again, so their steps and ends reach the server (design note 12 d).
        """
        try:
            await self.handle.patch.flush()
        except (ProblemError, httpx.TransportError):
            self._persist()
            return
        self.offset = offset
        self._persist()

    # ---- adoption (design note 12) ---------------------------------------------------------

    async def adopted(  # noqa: PLR0912, PLR0915 - one branch per message kind and end
        self, record: dict[str, Any], group: StoppableGroup, known_steps: Mapping[str, str]
    ) -> None:
        """Follow a turn that outlived the previous runner to its end and finish it once.

        No process is started and no follow-up is written (its stdin ended with the old runner);
        the end is classified without recovery (09 1 point 5, R-AG-17).

        Args:
            record: The turn record.
            group: The adopted process group (identity checked before every signal).
            known_steps: The status of each step of the server's run, by step id.
        """
        self.adopted_record = record
        self.adopted_group = group
        # Every record this runner writes keeps the adopted process's kind (a compaction stays one).
        self.attempt = _Attempt(kind=self.record_kind())
        self.phase = "closed"
        self.offset = int(record.get("offset") or 0)
        self.answered = {int(x) for x in record.get("answered", [])}
        self.replies = int(record.get("replies") or 0)
        self.title = str(record.get("title") or "")
        self.title_request = bool(record.get("title_request"))
        self.cost_baseline = _stored_baseline(record.get("cost_baseline"))
        if record.get("stop"):
            self.stop_reason = str(record["stop"])
        self.live = LiveBox(
            self.handle.patch,
            own_uuid="",
            title_requested=False,
            context_windows={},
            now=lambda: datetime.now(UTC),
        )
        self.live.restore(record.get("steps"), known_steps)
        done = False
        try:
            reader = StreamReader(
                Path(str(record["stream"])),
                self.cfg.poll_interval,
                group.alive,  # the leader: a leftover job of its group never holds the reader
                offset=self.offset,
                until_exit=True,
            )
            init_seen = False
            assistant_text = ""
            last: dict[str, Any] | None = None
            saved_at = time.monotonic()
            async for message in reader.items():
                if message is None:
                    await self._idle(group, None)
                    if (
                        time.monotonic() - saved_at
                        >= self.handle.patch.settings.run_update_min_interval
                    ):
                        # Like an own turn: the offset moves only after its patches went out.
                        saved_at = time.monotonic()
                        await self._save_offset(reader.offset)
                    continue
                kind = message.get("type")
                if kind == "system" and message.get("subtype") == "init":
                    init_seen = True
                elif kind == "assistant":
                    assistant_text = api_error_text(message) or assistant_text
                elif kind == "result":
                    last = message
                    self.last_result = message
                    if record.get("kind", "turn") == "turn" and _success(message):
                        await self._answer(message, reader.line_offset)
                if self.record_kind() == "turn":  # a compaction is never mapped (note 8 b)
                    self.live.apply(message)
                    if self.live.steps_changed:
                        self._persist()
            if self.stopping is not None:
                await self.stopping
            elif _group_runs(group):
                # Like an own turn: nothing of the group outlives the run (only a group whose
                # identity was verified is signalled, AdoptedGroup.may_signal).
                await self._stop(group)
            if self.server_ended:
                done = True
                return
            end = classify_turn_end(
                TurnEvidence(
                    stop_requested=self.stop_reason is not None,
                    init_seen=init_seen,
                    result=last,
                    assistant_text=assistant_text,
                    exit_label="unknown",
                )
            )
            await self._end_adopted(end.outcome, end.errors)
            done = True
        finally:
            # An orderly stop of this runner keeps the record for the next start (D-M2-35-20).
            if done:
                remove_turn_record(self.cfg.state_dir / "turns" / f"{self.handle.run_id}.json")

    async def _end_adopted(self, outcome: str, errors: tuple[str, ...]) -> None:
        if outcome == "stopped":
            await self._finish_cancelled()
        elif outcome == "completed" and self.record_kind() == "turn":
            await self._complete()
        elif outcome == "not_authenticated":
            await self._fail("runtime_not_authenticated", (), "unknown")
        else:
            await self._fail("runtime_error", errors, "unknown")

    def record_kind(self) -> str:
        """The kind of the adopted process (``turn`` or ``compaction``)."""
        return str((self.adopted_record or {}).get("kind", "turn"))


def _group_runs(group: StoppableGroup) -> bool:
    from claude_code_runner.stop import group_alive  # noqa: PLC0415 - keeps the import list flat

    return group_alive(group)


def _success(result: Mapping[str, Any]) -> bool:
    return result.get("subtype") == "success" and result.get("is_error") is not True


def _size(path: Path) -> int:
    try:
        return path.stat().st_size
    except FileNotFoundError:
        return 0


def _read_from(path: Path, offset: int) -> str:
    """The part of a file after ``offset`` (only matched against the table, never kept)."""
    try:
        with path.open("rb") as fh:
            fh.seek(offset)
            return fh.read().decode("utf-8", "replace")
    except FileNotFoundError:
        return ""


def _n(value: object) -> int:
    return value if isinstance(value, int) else 0
