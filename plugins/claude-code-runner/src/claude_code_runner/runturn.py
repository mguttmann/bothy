# SPDX-License-Identifier: Apache-2.0
"""One started run: its turn processes, follow-ups, stop and error paths (09 18.2 to 18.9).

`RunTurn.run` starts the turn of a run and follows it to the end. Every process end goes through
`claude_code_runner.failures.classify_turn_end` and `claude_code_runner.recovery.next_action`; a
full context is compacted once and the prompt runs again, a missing session is replaced by a new
one (fail open), an existing session at the first turn is resumed, and every other end finishes
the run once, after the process exited (design notes 5 to 11). Every process passes the pre-check
and the counter-check of M2-34 (I3 per process). Follow-ups are written to stdin while it is
open (`RunTurn.offer_followup`); stdin stays open while the process has background work
(`claude_code_runner.background`), so a follow-up then becomes a turn of its own and the answer
of a background task's notification is posted. A stop goes through `RunTurn.request_stop`, and a
run that the server ended stops its turn without a finish. `RunTurn.adopted` follows a turn that
outlived an earlier runner (design note 12).

A run of a hosted agent (docs/spec/26-builtin-agents.md 5) carries the parts of its command line
in `RunState` (``hosted``, ``max_budget_usd``); a run stopped by the cost cap ends
``runtime_error`` with the cause ``cost_cap_reached`` (26 5.8). Channel skill folders of a
``host`` turn and the live box note of skipped ones come the same way (26 5.9).

The turn record ``turns/<run_id>.json`` is written atomically before the action that depends on
it: the reference before its process starts, a reply's ``client_msg_id`` before the reply goes
out. Own and adopted turns read their stream through one loop (`RunTurn._read`), and the
recovery invariants (offset, answers, inputs, kind) hold in `claude_code_runner.ledger`
(brief design note 12 f, D-M2-35-28).

Examples:
    >>> RunState.__name__
    'RunState'
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import logging
import os
import signal
import time
from collections.abc import Callable, Mapping, Sequence
from dataclasses import dataclass, field
from datetime import UTC, datetime
from pathlib import Path
from typing import TYPE_CHECKING, Any, Protocol
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
from bothy_sdk.generated.api.agent_runtime import (
    check_run_launch,
    finish_run,
    report_agent_session,
    submit_transcript,
)
from bothy_sdk.generated.api.files import get_file
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
    RunContextAttachment,
    RunFinish,
    RunFinishState,
    RunMetrics,
    RunWakeCheckResult,
    TranscriptSubmit,
)
from bothy_sdk.generated.types import Unset
from bothy_sdk.long_answers import split_for_limit
from bothy_sdk.outbox import OutboxState
from bothy_sdk.runs import RunHandle, is_run_ended

from claude_code_runner.adoption import process_matches, read_identity
from claude_code_runner.attachments import (
    describe_followup_files,
    fetch_attachments,
    fetch_followup_files,
    followup_files_final,
)
from claude_code_runner.causes import (
    ADAPTER_STOPPED,
    RUN_ENDED_IDLE,
    adapter_stopped_cause,
    cause,
    turn_end_cause,
)
from claude_code_runner.commandline import HostedArgs, SessionChoice, build_argv, reported_model
from claude_code_runner.failures import (
    ErrorKind,
    TurnEnd,
    TurnEvidence,
    api_error_text,
    classify_turn_end,
    cost_cap_reached,
)
from claude_code_runner.followups import (
    FollowupCheck,
    FollowupVerdict,
    PendingFollowups,
    build_followup_line,
    followup_verdict,
)
from claude_code_runner.launch import (
    AttemptRecord,
    KeeperGroup,
    KeeperStart,
    LaunchFiles,
    Settled,
    prepare_launch,
    process_identity,
    read_json,
    remove_launch_files,
    settle_launching,
    start_keeper,
    wait_for,
)
from claude_code_runner.ledger import TurnLedger, is_success
from claude_code_runner.livebox import LiveBox
from claude_code_runner.livecost import LiveCost, load_prices
from claude_code_runner.locks import LockTimeout, SlotPool, acquire, release
from claude_code_runner.processstate import Decision, ProcessState, replayed_lines
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
    attempt_kind,
    box_note,
    next_action,
)
from claude_code_runner.reply_attachments import split_attach_lines
from claude_code_runner.reply_upload import (
    LIMIT_READ_FLOOR_BYTES,
    ReplyContext,
    RunEnded,
    read_content_limit,
    remove_run_files,
    send_answer_with_attachments,
)
from claude_code_runner.statedir import ensure_private_dir, write_private
from claude_code_runner.stop import StoppableGroup
from claude_code_runner.streamreader import StreamReader
from claude_code_runner.title import propose_title, split_title
from claude_code_runner.transcribe import (
    LOCAL_FILE,
    Untranscribed,
    notice,
    transcribe_attachments,
)
from claude_code_runner.turn import (
    Turn,
    TurnPaths,
    remove_turn_record,
    terminate,
    turn_paths,
    write_turn_record,
)
from claude_code_runner.usage import (
    ModelUsage,
    UsageStore,
    claude_config_dir,
    context_windows,
    cost_reports,
    counters_reset,
    difference,
    find_transcript,
    parse_model_usage,
    subtract_baseline,
    transcript_totals,
)

if TYPE_CHECKING:
    from claude_code_runner.config import RunnerConfig
    from claude_code_runner.transcribe import Transcriber

__all__ = ["COMPACT_IN_PRINT_MODE", "RunState", "RunTurn", "TurnRuntime"]

_LOG = logging.getLogger("claude_code_runner")
_EXPECTED_KEY_SOURCE = {"organization": "ANTHROPIC_API_KEY", "personal": "none"}
# The currency of the runtime's costUSD; a cost baseline in another currency is not subtracted.
_RUNTIME_CURRENCY = "USD"

COMPACT_IN_PRINT_MODE = True
"""``/compact`` is taken as an input line in print mode by the pinned CLI (design note 8 a):
the command ``compact`` of Claude Code 2.1.259 has ``supportsNonInteractive: true`` (file of the
installed pinned CLI; again in 2.1.288, read 2026-10-29) and the Agent SDK page on slash
commands sends ``/compact`` as a prompt
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


class TurnRuntime(Protocol):
    """What a turn takes from the runner: the runner's handler, or a hosted agent's view of it.

    Attributes:
        config: The configuration; for a hosted agent its state folder and loaded class.
        claude_bin: The CLI found in the cleaned ``PATH``.
        transcriber: The transcriber of 09 18.14.
    """

    @property
    def config(self) -> RunnerConfig:
        """The configuration of the turn."""
        ...

    @property
    def claude_bin(self) -> str | None:
        """The CLI found in the cleaned ``PATH``."""
        ...

    @property
    def transcriber(self) -> Transcriber:
        """The transcriber of 09 18.14."""
        ...

    async def precheck(self, state: RunState, desired: str, ids: Mapping[str, Any]) -> None:
        """The pre-check of 18.3 before a process (raises `RunRefused`)."""
        ...

    async def runtime_too_old(self, required: str | None) -> None:
        """The API refused the model to the CLI as too old (18.9)."""
        ...


@dataclass
class RunState:
    """What prepare took for one run; `close` gives it back (one object per run).

    Attributes:
        run_id: The run.
        thread_id: Its thread.
        channel_id: Its channel.
        locks: The thread lock and the session lock (descriptors).
        slot: The pool of ``runner.max_parallel_runs`` while the run holds one of its slots.
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
        hosted: The parts of a hosted turn's command line (26 5.3); ``None`` for the runner's
            own agent.
        plugin_dirs: Channel skill folders of a ``host`` turn (26 5.9).
        max_budget_usd: ``--max-budget-usd`` of a hosted definition (26 5.8), if any.
        cost_cap: The same cap as a number, to recognise a run it stopped.
        notice: The live box status line once per run (skipped channel skills, 26 5.9).
    """

    run_id: UUID
    thread_id: UUID
    channel_id: UUID
    locks: list[int] = field(default_factory=list)
    slot: SlotPool | None = None
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
    hosted: HostedArgs | None = None
    plugin_dirs: tuple[Path, ...] = ()
    max_budget_usd: str | None = None
    cost_cap: float | None = None
    notice: str | None = None

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
    """One process of a run: a turn or the compaction.

    ``label`` is the attempt's kind as its ``launching`` record names it (09 18.19.5).
    """

    kind: str = "turn"
    resume: bool = False
    hint: bool = False
    new_reference: bool = False
    bind_failed: bool = False
    label: str = "first"


def _attempt_of(record: AttemptRecord) -> _Attempt:
    """The process an attempt record describes (to launch it again or to classify its end)."""
    return _Attempt(
        kind="compaction" if record.kind == "compaction" else "turn",
        resume=record.resume,
        hint=record.hint,
        new_reference=record.new_reference,
        bind_failed=record.bind_failed,
        label=record.kind,
    )


_KEPT_PHASES = ("transcribing", "launching", "process", "compaction_retry")
"""The phases of a record the next runner continues (09 18.19.6)."""


@dataclass
class _Outcome:
    """What one process left behind."""

    evidence: TurnEvidence
    worked: bool
    handled: bool = False
    compacted: bool = False
    """The process wrote ``system/compact_boundary`` (a compaction that reduced the context)."""
    lost_tasks: int = 0
    """Background tasks still running when the process ended after a failed turn."""


@dataclass
class _Prepared:
    """The files of a follow-up, ready for its line."""

    files: list[str]
    """The texts for the ``Attachments:`` list."""
    transcripts: list[tuple[str, str]]
    """The name and transcript of each transcribed file."""
    missing: list[Untranscribed]
    """The audio files without a transcript; the thread gets a note once the line is written."""


@dataclass
class _FollowupOrder:
    """The order of the follow-ups of a run (FIX-followup-transcribe)."""

    tail: asyncio.Event | None = None
    """Set once the follow-up offered last is written or left; the next one waits for it."""
    preparing: set[UUID] = field(default_factory=set)
    """Follow-ups whose files are fetched and transcribed now (a redelivery is left)."""
    offered: int = 0
    """Follow-ups offered and not yet written or left; stdin stays open for them (issue 574)."""
    prepared: dict[UUID, _Prepared] = field(default_factory=dict)
    """Prepared files of follow-ups held between the processes of a recovery, so the next
    process takes them without a second transcription (issue 574)."""
    tasks: set[asyncio.Task[None]] = field(default_factory=set)
    """The offers of held follow-ups, which run beside the read of the turn (issue 574)."""


class RunTurn:
    """The turn processes of one started run (see the module docstring).

    Args:
        handler: The runner handler (configuration, pre-check, CLI path).
        state: The run's prepared state.
        run: The run handle of the SDK.
    """

    monotonic: Callable[[], float] = staticmethod(time.monotonic)
    """The clock of the stdin window and of the offset saves (a test sets a fake one)."""

    crash_point: Callable[[str, str], None] | None = None
    """A test-only hook called after each step of the launch handshake (09 18.19.5) with the
    step and the attempt's kind; never set in a normal start (AR-T04)."""

    def __init__(self, handler: TurnRuntime, state: RunState, run: RunHandle) -> None:  # noqa: PLR0915 - one attribute per part of a run
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
        # The order of the follow-ups while one waits for files or is transcribed.
        self.followup_order = _FollowupOrder()
        self.transcribing = 0
        """Transcriptions running now; the status line shows while one runs (issue 574)."""
        self.ps = ProcessState(has_stdin=True)
        """The stream-derived state of the current process (09 18.19.7): its background work
        (18.5, FIX-runner-responsive-turns), the failure mark (FIX-runner-auth-failed-held) and
        the rest the stdin and stop rule reads."""
        self._held_logged = False
        self.process: Turn | KeeperGroup | None = None
        self.phase = "starting"  # starting, open, closed, ended
        self.attempt = _Attempt()
        self.ledger = TurnLedger.begin("turn", 0)
        """The recovery bookkeeping of the current process (design note 12 f)."""
        self.stop_reason: str | None = state.stop_reason
        self.stopped_tasks = 0
        """The background tasks the process held when a stop came; a stop ends them (issue 553)."""
        self.server_ended = False
        self.reply_problem: str | None = None
        self.title = ""
        self.prompt_line = ""
        self.own_uuid = ""
        self.write_lock = asyncio.Lock()
        self.live: LiveBox | None = None
        # The input lines of the run by their ``uuid`` (kept in memory to write them again).
        self.lines: dict[str, bytes] = {}
        self.retry_inputs: list[tuple[str, bytes]] | None = None
        self._worked = False
        """The current process emitted an ``assistant`` message that was no API error."""
        self._replayed_worked = False
        """The replay of an adopted process found such a message before the handover point."""
        self.stopping: asyncio.Task[None] | None = None
        self.adopted_group: StoppableGroup | None = None
        self.adopted_record: dict[str, Any] | None = None
        self._identity: dict[str, str] = {}
        self.upload: asyncio.Task[Any] | None = None
        """The upload of an answer's attachment set while it runs (a stop cancels it)."""
        self.rebased = False
        """The run's first process started; only it takes the session totals (09 18.7)."""
        self.live_cost = LiveCost(load_prices())
        """The run's ``/cost`` while it works (FIX-cost-display)."""
        self._cost_sent = 0
        # ---- runner.turn_handover (09 18.19.3 to 18.19.7) ----
        self.handover = bool(getattr(handler, "turn_handover", False))
        """Every process starts under a keeper through the launch handshake; the record carries
        the run's phase. Off for the runner without the key and for hosted agents."""
        self.accepted = True
        """This runner accepted the run; an adopter calls ``run.launch.check`` before a launch."""
        self.launch_checked = False
        self.record_phase = "transcribing"
        self.launch: AttemptRecord | None = None
        """The current attempt as its record carries it."""
        self.keeper_start: KeeperStart | None = None
        self.attempt_no = 0
        self.outage = 0
        """The run's highest outage number reached before the current attempt (18.19.10)."""
        self.input_write: dict[str, Any] | None = None
        """The input line being written (``uuid``, ``bytes`` once a handover cut it)."""
        self.autonomous: set[str] = set()
        """Input lines written as autonomous inputs (a wake input, 5.8.6)."""
        self.handed_over = False
        self.retry_hint = False
        """The hint of a recorded retry after compaction (phase ``compaction_retry``)."""
        self.task: asyncio.Task[Any] | None = None
        self._fed: asyncio.Task[None] | None = None

    # ---- the run ---------------------------------------------------------------------------

    def _paths(self) -> TurnPaths:
        return turn_paths(self.cfg.state_dir, self.session.ref, str(self.handle.run_id))

    async def run(self) -> None:
        """Start the first process, follow every process and end the run once."""
        self.state.turn = self
        self.task = asyncio.current_task()
        paths = self._paths()
        ensure_private_dir(paths.stream.parent)
        if self.handover:
            self.record_phase = "transcribing"  # after run.start (18.19.6)
            self._persist()
        await self._open_turn()
        try:
            await self._drive(_Attempt(resume=self.session.resume), precheck=False)
        except asyncio.CancelledError:
            await self._on_cancelled()
            raise
        finally:
            self._end()

    async def _open_turn(self) -> None:
        """Transcripts, the prompt line and the live box before the first process."""
        paths = self._paths()
        context = self.state.context
        if context is None:  # pragma: no cover - prepare always loads it
            raise RuntimeError("no context")
        await self._transcribe(context)
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
            progress_interval=self.cfg.progress_patch_interval,
        )
        if self.state.notice:
            await self._status_line(self.state.notice)

    async def _drive(  # noqa: PLR0911, PLR0912 - one branch per action
        self,
        attempt: _Attempt,
        *,
        precheck: bool,
        outcome: _Outcome | None = None,
        repeat: AttemptRecord | None = None,
    ) -> None:
        """Run processes until the run ends: classify each end, then recover or finish.

        Args:
            attempt: The process to start (or whose end ``outcome`` is).
            precheck: Run the pre-check of M2-34 before the first process too (I3 per process);
                ``prepare`` ran it for the first process of a run this runner accepted.
            outcome: The end of an adopted process to classify first (the adoption rule of
                09 18.19.6); nothing is launched for it.
            repeat: The recorded attempt to launch again with the next try (18.19.5).
        """
        worked_before = self.launch.worked_before if self.launch is not None else False
        while True:
            if outcome is None:
                if self.stop_reason is not None:
                    await self._finish_cancelled()
                    return
                if precheck and not await self._precheck_again():
                    return
                if self._stop_requested():  # a stop during the pre-check starts nothing
                    await self._finish_cancelled()
                    return
                if not (self.accepted or self.launch_checked) and not await self._launch_check():
                    return  # R-AG-48: an adopter launches only on allow
                self.launch_checked = False
                precheck = True
                self.attempt = attempt
                outcome = await self._run_process(
                    attempt, repeat=repeat, worked_before=worked_before
                )
                repeat = None
                if outcome is None or outcome.handled:
                    return
            self._crash("attempt_ended")
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
            if end.outcome == ErrorKind.VERSION_UNSUPPORTED.value:
                await self.h.runtime_too_old(end.required_version)
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
                summary = None
                if attempt.kind == "turn":
                    summary = turn_end_cause(action.reason, outcome.lost_tasks)
                if action.reason == "runtime_error" and cost_cap_reached(
                    outcome.evidence.result, self.state.cost_cap
                ):
                    summary = cause("cost_cap_reached")
                await self._fail(action.reason, action.errors, end.exit_label, summary=summary)
                return
            next_attempt = await self._recover(action, outcome)
            if next_attempt is None:
                return
            if isinstance(action, Compact):
                worked_before = outcome.worked
            attempt = next_attempt
            outcome = None

    async def _on_cancelled(self) -> None:
        """The SDK cancels the run: a stop as in 18.6, or nothing at a handover (18.19.3)."""
        if self.handed_over or self._handing_over():
            return  # the turn runs on and its record stays for the next runner
        # The runner stops (its SDK cancels the run): the turn never outlives it (18.6, D-AR-1).
        # `_run_process` keeps the handle while a process of the group lives.
        await self._stop_on_cancel()

    def _handing_over(self) -> bool:
        return bool(getattr(self.h, "handing_over", False))

    def _end(self) -> None:
        """The run left this runner: remove its record and files once no process is left."""
        self.phase = "ended"
        if self._fed is not None:
            self._fed.cancel()
        if self.handed_over or self._handing_over():
            return  # the record and the files of a handed-over run stay (18.19.3)
        if self.process is None:
            # Only once no process of the turn is left: a record of a live turn stays for the
            # next start, which adopts or stops it (18.9).
            remove_turn_record(self._paths().record)
            remove_run_files(self.cfg.state_dir, self.handle.run_id)
            if self.handover:
                remove_launch_files(self.cfg.state_dir, str(self.handle.run_id))

    def _crash(self, point: str) -> None:
        """The test-only crash hook after a step of the handshake (`crash_point`)."""
        hook = type(self).crash_point
        if hook is not None and self.handover:
            hook(point, self.launch.kind if self.launch is not None else "")

    async def _stop_on_cancel(self) -> None:
        """Stop the live process of a cancelled run as in 18.6, then finish it ``cancelled``.

        The reason is the stop that was requested, else ``adapter_stopped`` with its cause for
        the thread. A run the server ended already is not finished (D-M2-35-6). A second
        cancellation during the stop leaves the handle set, so the record stays.
        """
        process = self.process
        if process is None:
            return
        if self.stop_reason is None:
            self.stop_reason = ADAPTER_STOPPED
            self._persist()
        self._count_stopped_tasks()
        await terminate(
            process, self.cfg.stop_grace, self.cfg.kill_grace, self.cfg.poll_interval, self.ids
        )
        self.process = None
        if self.server_ended or self.handle.patch.ended or self.handle.finished:
            return
        await self._finish_cancelled()

    def _stop_requested(self) -> bool:
        """Whether a stop came (it can arrive at every ``await`` of the run)."""
        return self.stop_reason is not None

    async def _transcribe(self, context: RunContext) -> None:
        """Transcripts of the trigger's audio attachments before the first input (09 18.14)."""
        if not any(text.startswith(LOCAL_FILE) for text in self.state.file_texts.values()):
            return  # no stored attachment, nothing to transcribe
        result = await self._transcribe_items(
            context.trigger_message.attachments, self.state.file_texts, self._stop_requested
        )
        if result is not None:
            await self._note_untranscribed(result[1])

    def _followups_moot(self) -> bool:
        """Whether no follow-up can be written any more (issue 574).

        A stop came, the server ended the run, or the run ended.
        """
        return (
            self._stop_requested()
            or self.server_ended
            or self.handle.patch.ended
            or self.phase == "ended"
        )

    async def _transcribe_items(
        self,
        attachments: Sequence[RunContextAttachment],
        file_texts: dict[UUID, str],
        cancel: Callable[[], bool],
    ) -> tuple[list[RunContextAttachment], list[Untranscribed]] | None:
        """Transcribe the audio attachments of the trigger or of a follow-up (09 18.14).

        A transcript is set on its attachment and delivered to the server (09 13). The status
        line shows while any transcription of the run runs. The transcriber ends at once when
        ``cancel`` says so.

        Args:
            attachments: The attachments.
            file_texts: Their file texts (changed in place).
            cancel: Polled every ``runner.poll_interval`` while the transcriber runs.

        Returns:
            The attachments that got a transcript and those that did not; ``None`` when it was
            cancelled.
        """
        started: list[bool] = []

        async def on_start() -> None:
            started.append(True)
            self.transcribing += 1
            if self.transcribing == 1:
                await self._status_line("Transcribing a voice message")

        task = asyncio.ensure_future(
            transcribe_attachments(attachments, file_texts, self.h.transcriber, on_start)
        )
        try:
            while not task.done():
                await asyncio.wait({task}, timeout=self.cfg.poll_interval)
                if cancel() and not task.done():
                    task.cancel()
                    with contextlib.suppress(asyncio.CancelledError):
                        await task
                    return None
        finally:
            if started:
                self.transcribing -= 1
        done, missing = task.result()
        if started and self.transcribing == 0:
            await self._status_line(None)
        for item in done:
            await self._submit_transcript(item.file_id, str(item.transcript))
        return done, missing

    async def _note_untranscribed(self, missing: Sequence[Untranscribed]) -> None:
        """The note in the thread about audio Claude Code gets only as a file (09 18.14)."""
        if not missing or self._stop_requested():
            return
        problem = await self._deliver(notice(missing), uuid4())
        if problem is not None:
            _log("transcript_notice_refused", code=problem, **self.ids)

    async def _submit_transcript(self, file_id: UUID, text: str) -> None:
        """``submitTranscript`` with the transcript ``ETag`` of ``getFile``; a refusal is logged.

        The server keeps it only for a voice message in mode ``agent`` (09 13); any other answer
        leaves the turn as it is.
        """
        client = self.handle.client
        try:
            current = await client.call_detailed(get_file.asyncio_detailed, file_id=file_id)
            await client.call(
                submit_transcript.asyncio_detailed,
                file_id=file_id,
                body=TranscriptSubmit(text=text),
                if_match=current.headers.get("ETag", ""),
            )
        except ProblemError as err:
            _log("transcript_not_submitted", code=err.code, file_id=file_id, **self.ids)
        except httpx.TransportError:
            _log("transcript_not_submitted", code="transport_error", file_id=file_id, **self.ids)

    async def _recover(self, action: Any, outcome: _Outcome) -> _Attempt | None:
        """The next process of a recovery (design notes 8 and 9)."""
        _log("recovery_started", action=type(action).__name__, **self.ids)
        if self.ledger.kind == "turn":
            # The inputs of the failed turn that no result answered go to the next turn (12 f 3).
            self.retry_inputs = [(u, self.lines[u]) for u in self.ledger.unanswered()]
        label = attempt_kind(action)
        if isinstance(action, Compact):
            self.recovery = self.recovery.with_(compacted=True)
            return _Attempt(kind="compaction", resume=True, label=label)
        if isinstance(action, Retry) and not self.retry_inputs:
            # Every input was answered before the context filled up: nothing to run again.
            await self._box_note("context_compacted", "completed", None)
            await self._complete()
            return None
        if isinstance(action, Retry):
            await self._box_note("context_compacted", "completed", None)
            if self.handover:
                # The recorded retry starts through the handshake; the decision of `_recover`
                # is not taken again by a next runner (18.19.6).
                self.record_phase = "compaction_retry"
                self.retry_hint = action.hint
                self._persist()
                self._crash("compaction_retry")
            return _Attempt(resume=True, hint=action.hint, label=label)
        if isinstance(action, ResumeSameReference):
            self.recovery = self.recovery.with_(session_resumed=True)
            return _Attempt(resume=True, label=label)
        if isinstance(action, NewSession):
            self.recovery = self.recovery.with_(session_replaced=True)
            if not await self._new_reference():
                await self._fail(
                    "adapter_error", (), outcome.evidence.exit_label, summary=cause("session_busy")
                )
                return None
            await self._box_note("session_missing", "completed", {"mode": self.state.session_mode})
            return _Attempt(
                resume=False, new_reference=True, bind_failed=action.bind_failed, label=label
            )
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
            await self._fail(
                refused.reason or "runtime_error", (), "not_started", summary=refused.summary
            )
            return False
        return True

    # ---- one process -----------------------------------------------------------------------

    def _prompt_file(self, attempt: _Attempt) -> Path:
        paths = self._paths()
        if not self.accepted and paths.system_prompt.is_file():
            # An adopter keeps the run's own prompt as the accepting runner wrote it.
            text = paths.system_prompt.read_text()
        else:
            text = system_prompt(
                title_request=self.title_request, channel_prompt=self.state.channel_prompt
            )
        if attempt.hint:
            path = paths.system_prompt.with_name(f"{self.handle.run_id}.retry-prompt.md")
            write_private(path, text.rstrip() + "\n\n## Retry\n\n" + RETRY_HINT + "\n")
            return path
        write_private(paths.system_prompt, text)
        return paths.system_prompt

    def _argv(self, attempt: _Attempt, prompt_file: Path) -> list[str]:
        add_dir = None
        if self.cfg.tool_class in ("scoped", "host") and self.state.stored_files:
            add_dir = self.cfg.state_dir / "files" / str(self.state.thread_id)
        session = SessionChoice(ref=self.session.ref, resume=attempt.resume, new=self.session.new)
        return build_argv(
            claude_bin=self.h.claude_bin or self.cfg.claude_bin,
            model=self.state.model,
            session=session,
            tool_class=self.cfg.tool_class,
            tools=self.state.tools,
            add_dir=add_dir,
            system_prompt_file=prompt_file,
            hosted=self.state.hosted,
            plugin_dirs=self.state.plugin_dirs,
            max_budget_usd=self.state.max_budget_usd,
        )

    async def _rebase_once(self, attempt: _Attempt) -> None:
        if not self.rebased:
            # Only the run's first process: a later one resumes the run's own session, whose
            # stored values already hold what the run saw (D-RUNNER-RESUME-COST-2).
            self.rebased = True
            if attempt.resume:
                await asyncio.to_thread(self._rebase_usage)

    async def _run_process(
        self,
        attempt: _Attempt,
        *,
        repeat: AttemptRecord | None = None,
        worked_before: bool = False,
    ) -> _Outcome | None:
        if self.handover:
            return await self._launch_process(attempt, repeat=repeat, worked_before=worked_before)
        paths = self._paths()
        ensure_private_dir(paths.stream.parent)
        prompt_file = self._prompt_file(attempt)
        argv = self._argv(attempt, prompt_file)
        stream_start = _size(paths.stream)
        stderr_start = _size(paths.stderr)
        cwd = self.state.cwd or self.cfg.projects_root
        await self._rebase_once(attempt)
        try:
            process = Turn.start(
                argv, cwd=cwd, env=self.state.env, paths=paths, pass_fds=self.state.locks
            )
        except OSError:
            _warn("turn_start_failed", **self.ids)
            await self._fail("runtime_error", (), "start_failed")
            return None
        self.process = process
        self._reset_process_state()
        # The kind is fixed here (12 f 4); the answers of earlier processes stay known (12 f 2).
        self.ledger = TurnLedger.begin(attempt.kind, stream_start, self.ledger)
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
        return await self._follow(process, attempt, stream_start, stderr_start)

    async def _follow(
        self,
        process: Turn | KeeperGroup,
        attempt: _Attempt,
        read_from: int,
        stderr_start: int,
        *,
        adopted: bool = False,
    ) -> _Outcome | None:
        """Read one process to its end and say how it ended (own or adopted alike)."""
        paths = self._paths()
        try:
            reader = StreamReader(
                paths.stream,
                self.cfg.poll_interval,
                process.alive,
                offset=read_from,
                until_exit=True,
            )
            outcome = await self._read(reader, process, attempt, adopted=adopted)
            if outcome is not None:
                return outcome
            await terminate(
                process, self.cfg.stop_grace, self.cfg.kill_grace, self.cfg.poll_interval, self.ids
            )
            if self.stopping is not None:
                await self.stopping
            self.phase = "closed"
            await self._checkpoint(reader.offset)
            ledger = self.ledger
            evidence = TurnEvidence(
                stop_requested=self.stop_reason is not None,
                init_seen=ledger.init_seen,
                result=ledger.end_result(),
                assistant_text=ledger.api_error,
                stderr=_read_from(paths.stderr, stderr_start),
                exit_label=process.exit_label(),
            )
            # The run's highest outage number reached so far goes into the next attempt.
            self.outage = max(self.outage, self.ps.outage)
            return _Outcome(
                evidence, self._worked, compacted=ledger.compacted, lost_tasks=self._lost_work()
            )
        finally:
            if self._fed is not None:
                self._fed.cancel()
                self._fed = None
            if not _group_runs(process):
                # The handle stays while a process of the group lives: a cancellation of the
                # run stops the group with it (D-AR-1).
                self.process = None
            if self.phase != "ended":
                self.phase = "starting"

    # ---- the launch handshake (09 18.19.5) ---------------------------------------------------

    def _initial_inputs(self, attempt: _Attempt) -> list[tuple[str, bytes]]:
        """The attempt's initial inputs, written by its keeper once (18.19.5 step 1)."""
        if attempt.kind == "compaction":
            line, line_uuid = user_line(["/compact"])
            return [(line_uuid, line.encode("utf-8") + b"\n")]
        inputs = self.retry_inputs
        if inputs is None:
            inputs = [(self.own_uuid, self.prompt_line.encode("utf-8") + b"\n")]
        return list(inputs)

    async def _launch_process(  # noqa: PLR0915 - the steps of the handshake in order
        self,
        attempt: _Attempt,
        *,
        repeat: AttemptRecord | None,
        worked_before: bool,
    ) -> _Outcome | None:
        """Start one attempt through the launch handshake and follow it (18.19.4, 18.19.5).

        Each step is persisted before the next starts: the record in ``launching`` before any
        file, the keeper's identity with ``go_sent`` before the go byte, the CLI's identity in
        phase ``process`` once ``<launch>.cli`` names it. The keeper writes the initial inputs;
        follow-ups wait for ``<launch>.fed``.
        """
        paths = self._paths()
        ensure_private_dir(paths.stream.parent)
        if not self.state.locks and not await self._take_locks():
            return None  # an adopter takes the locks before it launches
        prompt_file = self._prompt_file(attempt)
        argv = self._argv(attempt, prompt_file)
        stream_start, stderr_start = _size(paths.stream), _size(paths.stderr)
        await self._rebase_once(attempt)
        earlier: LaunchFiles | None = None
        if repeat is not None:
            number, try_, label, earlier = repeat.number, repeat.try_ + 1, repeat.kind, repeat.files
            inputs = [(u, self.lines.get(u, b"")) for u in repeat.inputs]
        else:
            number, try_, label = self.attempt_no + 1, 1, attempt.label
            inputs = self._initial_inputs(attempt)
        # The kind is fixed here (12 f 4); the answers of earlier processes stay known (12 f 2).
        self.ledger = TurnLedger.begin(attempt.kind, stream_start, self.ledger)
        for line_uuid, line in inputs:
            self.lines[line_uuid] = line
            if attempt.kind == "turn":
                self.ledger.written(line_uuid)
        self._store_lines(inputs)
        self.attempt_no = number
        files = LaunchFiles.of(self.cfg.state_dir, str(self.handle.run_id), number, try_)
        launch = AttemptRecord(
            number=number,
            kind=label,
            try_=try_,
            files=files,
            resume=attempt.resume,
            hint=attempt.hint,
            new_reference=attempt.new_reference,
            bind_failed=attempt.bind_failed,
            session_ref=self.session.ref,
            inputs=[u for u, _ in inputs],
            outage=self.outage,
            worked_before=worked_before,
            stderr_start=stderr_start,
        )
        self.launch = launch
        self.process = None
        self._identity = {}
        self.record_phase = "launching"
        self._persist()  # step 1: the record before any file of the launch
        self._crash("launching")
        timeout, poll = self.cfg.cli_probe_timeout, self.cfg.poll_interval
        spec = {
            "argv": argv,
            "env": self.state.env,
            "cwd": str(self.state.cwd or self.cfg.projects_root),
            "stdout": str(paths.stream),
            "stderr": str(paths.stderr),
            "lock_fds": list(self.state.locks),
        }
        try:
            prepare_launch(files, b"".join(line for _, line in inputs), spec, earlier=earlier)
            self._crash("prepared")
            start = start_keeper(
                files, compaction=attempt.kind == "compaction", lock_fds=self.state.locks
            )
        except OSError:
            files.spec.unlink(missing_ok=True)
            _warn("turn_start_failed", **self.ids)
            await self._fail("runtime_error", (), "start_failed")
            return None
        self.keeper_start = start
        self._crash("keeper_started")

        def keeper_runs() -> bool:
            return start.process.poll() is None

        keeper = await wait_for(files.keeper, keeper_runs, timeout, poll)
        if keeper is None or self.handed_over:
            start.close()  # no go byte: the keeper starts no CLI and exits by itself
            if not self.handed_over:
                await self._start_failed(start)
            return None
        ident = await asyncio.to_thread(process_identity, int(keeper["pid"]), timeout)
        launch.keeper = {**keeper, **ident}
        launch.go_sent = True
        self._persist()  # step 3: the keeper's identity and go_sent before the go byte
        self._crash("go_recorded")
        start.go()
        self._crash("go_sent")
        cli = await wait_for(files.cli, keeper_runs, timeout, poll)
        if cli is None:
            await self._start_failed(start)
            return None
        identity = await asyncio.to_thread(process_identity, int(cli["pid"]), timeout)
        launch.cli = {**cli, **identity}
        group = KeeperGroup(
            files,
            keeper=launch.keeper,
            cli=launch.cli,
            session_ref=self.session.ref,
            timeout=timeout,
            process=start.process,
        )
        self.process = group
        self._identity = identity
        self.record_phase = "process"
        self._reset_process_state(outage=launch.outage)
        self._persist()  # step 5: the CLI's identity in phase process
        self._crash("cli_recorded")
        _log("turn_started", kind=attempt.kind, **self.ids)
        self.phase = "starting"  # follow-ups wait for <launch>.fed (step 6)
        if attempt.kind != "compaction":
            self._fed = asyncio.ensure_future(self._await_fed(group))
        return await self._follow(group, attempt, stream_start, stderr_start)

    async def _start_failed(self, start: KeeperStart) -> None:
        """The keeper ended, or named no CLI in time: no CLI runs for this launch."""
        if start.process.poll() is None:  # NOSONAR - poll() does not block; wait() runs in a thread
            with contextlib.suppress(ProcessLookupError, PermissionError):
                os.killpg(start.process.pid, signal.SIGKILL)  # our own keeper's group
        await asyncio.to_thread(start.process.wait)
        _warn("turn_start_failed", **self.ids)
        await self._fail("runtime_error", (), "start_failed")

    async def _await_fed(self, group: KeeperGroup) -> None:
        """Step 6 of 18.19.5: follow-ups and stdin wait for ``<launch>.fed``.

        A keeper that ends without it may have cut an input in the middle of a line: nothing
        more is written into that pipe, stdin is closed and ``inputs_written`` stays false.
        """
        while not group.files.fed.exists():
            if not group.keeper_alive() and not group.files.fed.exists():
                await self._close_stdin(group)
                return
            await asyncio.sleep(self.cfg.poll_interval)
        async with self.write_lock:
            if self.launch is not None:
                self.launch.inputs_written = True
            self._persist()
            if self.phase == "starting" and self.stop_reason is None:
                self.phase = "open"
        self._offer_held()

    async def _take_locks(self) -> bool:
        """The thread and session locks of an adopter before it launches (09 18.2)."""
        locks = self.cfg.state_dir / "locks"
        taken: list[int] = []
        try:
            for name in (f"thread-{self.state.thread_id}", f"session-{self.session.ref}"):
                taken.append(
                    await acquire(
                        locks / f"{name}.lock", self.cfg.session_lock_wait, self.cfg.poll_interval
                    )
                )
        except LockTimeout:
            for fd in taken:
                release(fd)
            _log("session_busy", **self.ids)
            await self._fail("adapter_error", (), "not_started", summary=cause("session_busy"))
            return False
        self.state.locks.extend(taken)
        return True

    def _store_lines(self, inputs: Sequence[tuple[str, bytes]]) -> None:
        """Keep input lines in the run's private lines file (a next runner writes them again)."""
        if not self.handover or not inputs:
            return
        path = self.cfg.state_dir / "turns" / f"{self.handle.run_id}.lines"
        ensure_private_dir(path.parent)
        fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_APPEND | os.O_NOFOLLOW, 0o600)
        try:
            os.write(fd, b"".join(line for _, line in inputs))
        finally:
            os.close(fd)

    def _load_lines(self) -> dict[str, bytes]:
        path = self.cfg.state_dir / "turns" / f"{self.handle.run_id}.lines"
        out: dict[str, bytes] = {}
        try:
            data = path.read_bytes()
        except OSError:
            return out
        for raw in data.split(b"\n"):
            with contextlib.suppress(ValueError, TypeError, AttributeError):
                line_uuid = json.loads(raw).get("uuid")
                if isinstance(line_uuid, str):
                    out[line_uuid] = raw + b"\n"
        return out

    @property
    def background(self) -> Any:
        """The background work of the current process (`ProcessState.background`)."""
        return self.ps.background

    def _reset_process_state(self, outage: int = 0) -> None:
        """The per-process state of a new turn process (`ProcessState`, 09 18.19.7)."""
        self.ps = ProcessState(has_stdin=True, outage=outage)
        self._held_logged = False

    def _lost_work(self) -> int:
        """The background tasks a process ended with after a failed turn; their steps end."""
        if self.ps.failed_at is None:
            return 0
        if self.live is not None and self.ledger.kind == "turn":
            self.live.end_open_tasks()  # its background work ended with the process
        return self.ps.lost_tasks

    async def _write_inputs(self, process: Turn) -> None:
        """The inputs of a turn process, then the held follow-ups (notes 4 c and d, 8 b).

        The first process gets the prompt line. A process of a recovery gets the inputs of the
        failed process that no ``result`` answered, in their order and with their original
        ``uuid``: the prompt line while it was not answered, then the follow-ups after it
        (design note 12 f 3).
        """
        inputs = self.retry_inputs
        if inputs is None:
            inputs = [(self.own_uuid, self.prompt_line.encode("utf-8") + b"\n")]
        async with self.write_lock:
            # Every input is in the record before the CLI can have one (design note 12 f 3).
            for line_uuid, line in inputs:
                self.lines[line_uuid] = line
                self.ledger.written(line_uuid)
            self._persist()
            for _, line in inputs:
                await process.write_bytes(line)
            self.phase = "open"
        self._offer_held()

    def _offer_held(self) -> None:
        """Offer the held follow-ups beside the read of the turn, in their order (issue 574).

        Their places in the order are taken here, before any other follow-up can come, and the
        files are fetched and transcribed while the turn's output is read.
        """
        held, self.state.held = self.state.held, []
        order = self.followup_order
        for inv in held:
            slot = self._claim(inv)
            if slot is None:
                continue
            task = asyncio.ensure_future(self._offer(inv, *slot))
            order.tasks.add(task)
            task.add_done_callback(self._offer_done)

    def _offer_done(self, task: asyncio.Task[None]) -> None:
        self.followup_order.tasks.discard(task)
        if not task.cancelled() and task.exception() is not None:
            _warn("followup_offer_failed", error=type(task.exception()).__name__, **self.ids)

    async def _read(  # noqa: PLR0912 - one branch per message kind of 18.4
        self,
        reader: StreamReader,
        group: StoppableGroup,
        attempt: _Attempt | None,
        *,
        adopted: bool = False,
    ) -> _Outcome | None:
        """Read one process's stream to its end: the one loop of an own and an adopted turn.

        ``attempt`` is ``None`` for an adopted turn: it reports no session, writes nothing to
        stdin and runs no counter-check (its runner did). Every line goes through the ledger
        first (design note 12 f), then its effects follow; the stored offset moves only in
        `_checkpoint`.

        Returns:
            An outcome when the read ended the run itself (the counter-check), else ``None``.
        """
        own = attempt is not None
        self._worked = adopted and self._replayed_worked
        saved_at = self.monotonic()
        interval = self.handle.patch.settings.run_update_min_interval
        async for message in reader.items():
            if message is None:
                await self._idle(group)
                if self.monotonic() - saved_at >= interval:
                    saved_at = self.monotonic()
                    await self._checkpoint(reader.offset)
                continue
            self.ledger.line(message, reader.line_offset, reader.offset)
            self.ps.apply(message, self.monotonic())
            kind, subtype = message.get("type"), message.get("subtype")
            if kind == "system" and subtype == "init" and attempt is not None:
                if message.get("apiKeySource") != _EXPECTED_KEY_SOURCE.get(self.state.mode):
                    _warn("counter_check_failed", **self.ids)
                    if self.process is not None:
                        await terminate(
                            self.process,
                            self.cfg.stop_grace,
                            self.cfg.kill_grace,
                            self.cfg.poll_interval,
                        )
                    await self._finish("failed", reason="billing_mode_mismatch")
                    return _Outcome(TurnEvidence(), self._worked, handled=True)
                await self._report_session(message, attempt)
            elif kind == "assistant":
                # An API error the CLI writes as an assistant message is no work (note 8 b).
                no_error = api_error_text(message) is None
                self._worked = self._worked or (self.ledger.init_seen and no_error)
                if self.live_cost.assistant(message):
                    self._report_cost()
            elif kind == "user" and message.get("isReplay") is True:
                await self._echo(message.get("uuid"), adopted=adopted or not own)
            elif kind == "result":
                self._settle_cost()
                if self.ledger.kind == "turn" and is_success(message):
                    await self._answer(message, reader.line_offset)
            if self.ledger.kind == "turn" and self.live is not None:
                self.live.apply(message)  # a compaction is never mapped (note 8 b)
                if self.live.steps_changed:
                    self._persist()  # the step ids before their patches go out (note 12 b)
            if self.ps.has_stdin and kind == "result":
                # A failed turn absorbs nothing more: stdin closes now, and the follow-ups
                # without echo go to the next process of a recovery (notes 4 d and 5); after a
                # result without error the stdin rule of note 5 runs.
                await self._idle(group)
        return None

    async def _idle(self, process: StoppableGroup) -> None:
        """Checks while a turn runs: a run the server ended, and the stdin rule of note 5.

        The stdin and stop rule reads only `ProcessState` (09 18.19.7), the pending follow-ups,
        the run's stop state and the configuration.
        """
        if self.handle.patch.ended and not self.server_ended and self.stop_reason is None:
            self.server_ended = True
            _log("run_ended_by_server", **self.ids)
        if (self.server_ended or self.stop_reason is not None) and self.stopping is None:
            # Also a stop that came while this process was being started (note 6 d).
            self._stop_group(process)
            return
        decision = self.ps.decide(
            self.monotonic(),
            stdin_open=self.phase == "open",
            pending=len(self.followups.pending()),
            followup_grace=self.cfg.followup_grace,
            stop_grace=self.cfg.stop_grace,
            preparing=self.followup_order.offered,
        )
        if decision is Decision.STOP and self.stopping is None:
            # The CLI keeps a process with background work after its stdin ended; a failed turn
            # must not hold the run open for that work (FIX-runner-auth-failed-held).
            lost = self.ps.lost_at_stop()
            _log("turn_stopped_after_error", background_tasks=lost, **self.ids)
            self._stop_group(process)
        elif decision is Decision.HOLD:
            # Background work outlives the turn: the CLI stops it when its input ends, so stdin
            # stays open, and every follow-up becomes a turn of its own (09 18.5).
            if self.background.running and self.live is not None:
                self.live.keepalive()
            if self.ps.last_result_at is not None and not self._held_logged:
                self._held_logged = True
                _log("stdin_held_for_background", **self.ids)
        elif decision is Decision.CLOSE:
            # Also the close at an error result; a queued input (Decision.WAIT) keeps stdin
            # open for the turn still to come (FIX-runner-hold-background).
            await self._close_stdin(process)

    async def _close_stdin(self, process: StoppableGroup) -> None:
        async with self.write_lock:
            if self.phase == "open":
                self.phase = "closed"
            self._record_stdin_closed()
            process.close_stdin()

    def _record_stdin_closed(self) -> None:
        """Every place that closes stdin records ``stdin_closed`` first (09 18.19.7)."""
        if self.handover and self.launch is not None and not self.launch.stdin_closed:
            self.launch.stdin_closed = True
            self._persist()

    def _stop_group(self, group: StoppableGroup) -> None:
        if self.stopping is not None:
            return
        self.phase = "closed"
        self._record_stdin_closed()  # the stop closes stdin too
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
        if self.upload is not None:
            self.upload.cancel()  # the request in flight of an answer's set (M3-12 note 10 d)
        group = self.process or self.adopted_group
        if group is not None:
            self._count_stopped_tasks()
            self._stop_group(group)

    def _count_stopped_tasks(self) -> None:
        """Keep how many background tasks the process holds as the stop comes (issue 553).

        Read before the signal: the CLI reports its task list empty as the stop ends the tasks.
        """
        self.stopped_tasks = max(self.stopped_tasks, len(self.background.running))

    def _ended_by_server(self) -> None:
        # The server ended the run (409 conflict of an ended run, or 401): no finish (6 e). The
        # SDK finishes a run whose handler returns unless it is marked finished.
        self.handle.finished = True
        _log("run_finished", state="ended_by_server", **self.ids)

    # ---- follow-ups ------------------------------------------------------------------------

    async def offer_followup(self, inv: Invocation) -> None:
        """Write a follow-up now, hold it, or leave it (design notes 2 to 4), in their order.

        Each follow-up is written only after the one offered before it is written or left, so
        a follow-up that waits for its files or is transcribed keeps its place. Its token is
        judged at the time of the offer (09 18.5), and a follow-up whose check fails is neither
        stored nor transcribed.

        Args:
            inv: The ``agent.followup`` invocation (its claims were verified at acceptance).
        """
        slot = self._claim(inv)
        if slot is not None:
            await self._offer(inv, *slot)

    def _claim(
        self, inv: Invocation
    ) -> tuple[UUID, float, asyncio.Event | None, asyncio.Event] | None:
        """The place of a follow-up in the order, taken without a wait.

        Returns:
            The message, the time of the offer, the event of the follow-up before it and its
            own event; ``None`` for an invocation without a message.
        """
        message_id = getattr(inv.payload, "message_id", None)
        if not isinstance(message_id, UUID):
            return None
        offered_at = self.handle.client.clock.now().timestamp()
        order = self.followup_order
        before, mine = order.tail, asyncio.Event()
        order.tail = mine
        order.offered += 1
        return message_id, offered_at, before, mine

    async def _offer(
        self,
        inv: Invocation,
        message_id: UUID,
        offered_at: float,
        before: asyncio.Event | None,
        mine: asyncio.Event,
    ) -> None:
        """Prepare and write one follow-up at its place (see `offer_followup`)."""
        ids = {**self.ids, "message_id": message_id}
        order = self.followup_order
        try:
            if before is not None and self.phase != "open":
                await before.wait()  # offered before the turn opened: prepared in its order
                before = None
            read: tuple[Message | None] | None = None
            prepared = order.prepared.pop(message_id, None)  # held in a recovery: prepared
            if prepared is None and self.phase == "open" and message_id not in self.followups:
                if message_id in order.preparing:
                    # A redelivery of a follow-up that is being prepared: it is left, and keeps
                    # its place until the follow-up before it is written or left.
                    if before is not None:
                        await before.wait()
                    return
                read, prepared = await self._prepare_followup(inv, ids, offered_at)
            if before is not None:
                await before.wait()
            await self._write_followup(inv, ids, offered_at, read, prepared)
        finally:
            order.offered -= 1
            mine.set()

    async def _write_followup(  # noqa: PLR0911, PLR0912 - one exit per check
        self,
        inv: Invocation,
        ids: Mapping[str, Any],
        offered_at: float,
        read: tuple[Message | None] | None,
        prepared: _Prepared | None,
    ) -> None:
        """Hold, leave or write one follow-up under the write lock (design notes 2 to 4)."""
        message_id: UUID = ids["message_id"]
        async with self.write_lock:
            if self._followups_moot() or self.phase == "closed":
                _log("followup_not_deliverable", **ids)
                return
            if self.phase == "starting":
                if message_id not in self.followups and all(
                    getattr(h.payload, "message_id", None) != message_id for h in self.state.held
                ):
                    self.state.held.append(inv)
                    if prepared is not None:
                        self.followup_order.prepared[message_id] = prepared
                _log("followup_held", **ids)
                return
            if message_id in self.followups:
                return
            message = read[0] if read is not None else await self._read_message(message_id, ids)
            if message is None:
                return
            process = self.process
            verdict, code = self._followup_verdict(inv, message_id, message, offered_at)
            if verdict is FollowupVerdict.SKIP:
                _warn(code, **ids)
                return
            if verdict is not FollowupVerdict.WRITE or process is None:
                return
            author = message.author.to_dict()
            line_uuid = str(uuid4())
            if prepared is None:  # the turn opened while it waited: prepared here
                prepared = await self._followup_files(message)
            if prepared is None:  # the run ended during the transcription
                _log("followup_not_deliverable", **ids)
                return
            line = build_followup_line(
                author, message.content_md, line_uuid, prepared.files, prepared.transcripts
            )
            # The record knows the line before the CLI can: a runner that adopts the turn maps
            # its echo to the follow-up and patches it absorbed (design note 12 f 3).
            self.followups.add(message_id, line_uuid, line)
            self.lines[line_uuid] = line
            self.ledger.written(line_uuid)
            if self.handover:
                # Recorded before it is written, marked written after its last byte (18.19.3).
                self._store_lines([(line_uuid, line)])
                self.input_write = {"uuid": line_uuid, "bytes": None}
            self._persist()
            written = await process.write_bytes(line)
            if self.handed_over:
                return  # the handover recorded the line with its byte count
            if self.handover:
                self.input_write = None
                if written:
                    self._persist()
            if not written:
                del self.followups.by_message[message_id]
                del self.lines[line_uuid]
                self.ledger.unwritten(line_uuid)
                self._persist()
                _log("followup_not_deliverable", **ids)
                return
        _log("followup_written", **ids)
        await self._followup_state(message_id, "delivered")
        if self.followups.delivered(message_id):
            await self._followup_state(message_id, "absorbed")
        # Only a follow-up that reached the turn gets the note; a deferred one is transcribed
        # again in its own run (issue 574).
        await self._note_untranscribed(prepared.missing)

    def _followup_verdict(
        self, inv: Invocation, message_id: UUID, message: Message, offered_at: float
    ) -> tuple[FollowupVerdict, str]:
        """The checks of a follow-up (design note 2 b and c), its token judged at the offer."""
        check = FollowupCheck(
            claims=inv.claims,
            now=offered_at,
            leeway=self.cfg.sdk.invocation_token_leeway,
            turn_started=self.process is not None and self.phase == "open",
            run_id=self.handle.run_id,
            thread_id=self.state.thread_id,
            billing_mode=self.state.mode,
            tool_class=self.cfg.tool_class,
            message_id=message_id,
            author_id=message.author.id,
            message_thread_id=message.thread_id,
            written=set(self.followups.by_message),
        )
        return followup_verdict(check)

    async def _prepare_followup(
        self, inv: Invocation, ids: Mapping[str, Any], offered_at: float
    ) -> tuple[tuple[Message | None], _Prepared | None]:
        """Read a follow-up once its files are final, then store and transcribe its files.

        Outside the lock, so other follow-ups and the stop go on while a file is checked or
        transcribed. The files are touched only when the checks of the follow-up pass; the
        write checks again and logs a failed check.

        Returns:
            The message of the last read and the prepared files (``None`` when the follow-up
            has none, cannot be written, fails its checks, or a stop or the end of the run
            came).
        """
        message_id: UUID = ids["message_id"]
        preparing = self.followup_order.preparing
        preparing.add(message_id)
        try:
            message = await self._await_followup_files(message_id, ids)
            if message is None or not message.attachments or self._stop_requested():
                return (message,), None
            verdict, _ = self._followup_verdict(inv, message_id, message, offered_at)
            if verdict is not FollowupVerdict.WRITE:
                return (message,), None
            return (message,), await self._followup_files(message)
        finally:
            preparing.discard(message_id)

    async def _followup_files(self, message: Message) -> _Prepared | None:
        """The attachment texts of a follow-up and the transcripts of its audio files.

        The files are stored like those of the trigger, and an audio file is transcribed like
        an audio attachment of the trigger (09 18.5, 18.14; FIX-followup-transcribe).

        Args:
            message: The follow-up message.

        Returns:
            The prepared files; ``None`` when a stop or the end of the run (server or runner)
            cancelled the transcription (issue 574).
        """
        if not message.attachments:
            return _Prepared([], [], [])
        items, texts = await fetch_followup_files(
            self.handle.client,
            message.attachments,
            self.cfg.state_dir / "files" / str(self.state.thread_id),
        )
        result = await self._transcribe_items(items, texts, self._followups_moot)
        if result is None:
            return None
        done, missing = result
        transcripts = [(item.original_name, str(item.transcript)) for item in done]
        return _Prepared(describe_followup_files(items, texts), transcripts, missing)

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
            or (not message.content_md.strip() and not message.attachments)
        ):
            _log("followup_no_text", **ids)
            return None
        return message

    async def _await_followup_files(
        self, message_id: UUID, ids: Mapping[str, Any]
    ) -> Message | None:
        """Wait until the attachments of a follow-up are final (FIX-scan-state-stale).

        The server delivers a follow-up when the message is stored, also while its files are
        still checked (12 10 rule 1). The runner reads the message again every
        ``runner.poll_interval`` until every file is final, a stop or the end of the turn comes,
        or ``runner.followup_file_wait`` has passed; the follow-up is then offered as usual and
        names a file that is still not ready as such.

        Returns:
            The message of the last read (``None`` when it cannot be written), so the follow-up
            is not read once more.
        """
        deadline = self.monotonic() + self.cfg.followup_file_wait
        while True:
            message = await self._read_message(message_id, ids)
            if message is None or followup_files_final(message.attachments):
                return message
            if self._stop_requested() or self.phase != "open":
                return message
            if self.monotonic() >= deadline:
                _log("followup_files_not_ready", **ids)
                return message
            _log("followup_waits_for_files", **ids)
            await asyncio.sleep(self.cfg.poll_interval)

    async def _echo(self, line_uuid: object, *, adopted: bool = False) -> None:
        """The echo of an input line: its follow-up is ``absorbed`` (note 4 b).

        An adopted turn does not know whether the patches of the previous runner went out, so
        it patches ``absorbed`` on every echo it reads (the server takes it from ``queued`` and
        from ``delivered``, and a repeat changes nothing); a follow-up the turn took must never
        stay ``delivered``, or the server would defer it into a run of its own and it would be
        answered twice.
        """
        if adopted:
            item = next((f for f in self.followups if f.uuid == line_uuid), None)
            if item is not None:
                item.echoed = True
                await self._followup_state(item.message_id, "absorbed")
            self._persist()
            return
        echoed = self.followups.echo(line_uuid)
        if echoed is not None:
            await self._followup_state(echoed.message_id, "absorbed")
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

    async def _answer(self, result: Mapping[str, Any], line: int) -> None:
        """One answer per ``result`` without error (note 5), at most once (12 f 2).

        The reply's ``client_msg_id`` is stored in the turn record before the reply goes out;
        a line read again (after a crash) sends the same ``client_msg_id``, which the server
        answers with the original, or nothing once the reply was stored as sent.
        """
        if self._stop_requested():
            return  # no answer after a stop (note 6 b)
        planned = self.ledger.planned(line)
        if planned is not None and planned.sent:
            return
        if planned is None:
            planned = self.ledger.plan_answer(line, may_title=self.title_request)
            self._persist()
        text = result.get("result") if isinstance(result.get("result"), str) else ""
        title, reply = split_title(str(text), planned.first)
        if title and not self.title:
            self.title = title
        body, paths = split_attach_lines(reply)
        problem: str | None = None
        # M4-47: an answer is never split; above message_max_content_bytes its rest becomes
        # the last attachment answer.txt of the same message (D-M4-47-6).
        try:
            body, overflow, limit = await self._cut_to_limit(body if paths else reply)
        except RunEnded:
            self.server_ended = True
            _log("run_ended_by_server", **self.ids)
            return
        if paths or overflow is not None:
            # Answer attachments (M3-12 note 9): an answer with ATTACH lines is sent also with
            # an empty text, with its files or with the notice.
            sent, problem = await self._answer_with_attachments(
                body, paths, planned.client_msg_id, line, overflow=overflow, limit=limit
            )
            if not sent:
                return  # a stop, or the run ended on the server: nothing was sent
        elif reply.strip():
            problem = await self._deliver(reply, planned.client_msg_id)
        if problem is not None and self.reply_problem is None:
            _log("reply_refused", code=problem, **self.ids)
            self.reply_problem = problem
        self.ledger.answer_sent(line)
        self._persist()

    async def _cut_to_limit(self, text: str) -> tuple[str, str | None, int | None]:
        """The head, the rest and the limit of an answer above ``message_max_content_bytes``.

        A text up to `LIMIT_READ_FLOOR_BYTES` is not checked; a longer one reads the limit
        (``getInstance``, M4-47). Without a limit the text goes out as it is.
        """
        if len(text.encode("utf-8")) <= LIMIT_READ_FLOOR_BYTES:
            return text, None, None
        limit = await read_content_limit(self.handle.client, self.ids)
        if limit is None:
            return text, None, None
        head, rest = split_for_limit(text, limit)
        return head, rest, limit

    async def _answer_with_attachments(  # noqa: PLR0913 - the answer, its rest and limit
        self,
        text: str,
        paths: list[str],
        client_msg_id: UUID,
        line: int,
        *,
        overflow: str | None = None,
        limit: int | None = None,
    ) -> tuple[bool, str | None]:
        """Send an answer with ``ATTACH:`` lines (M3-12).

        Returns:
            Whether the answer was handled (False after a stop or when the server ended the
            run: nothing was sent), and ``None`` or the problem code of a refused answer.
        """
        answer = sorted(self.ledger.answers).index(line) + 1
        ctx = ReplyContext(
            client=self.handle.client,
            config=self.cfg,
            run_id=self.handle.run_id,
            thread_id=self.state.thread_id,
            channel_id=self.state.channel_id,
            session_ref=self.session.ref,
            cwd=self.state.cwd,
            deliver=self._deliver_detailed,
            set_status=self._status_line,
            stopped=self._stop_requested,
            track=self._track_upload,
            ids=self.ids,
        )
        try:
            sent = await send_answer_with_attachments(
                ctx, text, paths, answer, client_msg_id, overflow=overflow, limit=limit
            )
        except RunEnded:
            self.server_ended = True
            _log("run_ended_by_server", **self.ids)
            return False, None
        return not sent.stopped, sent.problem

    def _track_upload(self, task: asyncio.Task[Any] | None) -> None:
        self.upload = task
        if task is not None and self.stop_reason is not None:
            task.cancel()

    async def _status_line(self, text: str | None) -> None:
        """``replace`` or ``remove`` of ``/status_line`` (09 5.3), sent at once."""
        patch = self.handle.patch
        if text is None:
            patch.clear_status_line()
        else:
            patch.set_status_line(text)
        try:
            await patch.flush()
        except ProblemError as err:
            if is_run_ended(err):
                self.server_ended = True
            _log("status_line_failed", code=err.code, **self.ids)
        except httpx.TransportError:
            _log("status_line_failed", code="transport_error", **self.ids)

    async def _deliver_detailed(
        self, reply: str, client_msg_id: UUID, attachment_file_ids: Sequence[UUID]
    ) -> tuple[str | None, ProblemError | None]:
        """Like `_deliver`, with ``attachment_file_ids`` and the refusal of the server.

        The same ``client_msg_id`` is sent again while the entry is pending (the outbox answers
        a stored entry as it is), so a refusal that comes on a later attempt is seen too.
        """
        kwargs: dict[str, Any] = {"client_msg_id": client_msg_id}
        if attachment_file_ids:
            kwargs["attachment_file_ids"] = list(attachment_file_ids)
        settings = self.cfg.sdk
        for attempt in range(settings.retry_max_attempts + 1):
            if attempt:
                await self.handle.client.clock.sleep(
                    min(
                        settings.retry_backoff_cap, settings.retry_backoff_base * 2 ** (attempt - 1)
                    )
                )
            entry = await self.handle.send(reply, **kwargs)
            if entry.state is OutboxState.SENT:
                return None, None
            if entry.state is OutboxState.FAILED:
                return entry.problem_code or "unknown", entry.problem
        return "reply_not_delivered", None

    async def _deliver(self, reply: str, client_msg_id: UUID) -> str | None:
        """Send the reply; ``None`` once it is stored on the server, else the problem code.

        A reply still pending after the SDK's own retries (a transient outage) is sent again
        with the same ``client_msg_id`` up to ``sdk.retry_max_attempts`` times, so a run never
        ends ``completed`` with its answer lost.
        """
        entry = await self.handle.send(reply, client_msg_id=client_msg_id)
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
        body = AgentSessionReport(external_session_ref=self.session.ref, started=True)
        model = reported_model(self.state.model)
        if model is not None:
            body.model = model
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
        if self.reply_problem is not None:
            await self._finish("failed", reason="adapter_error", summary=cause("reply_refused"))
            return
        if self.title_request and self.title:
            outcome = await propose_title(self.handle.client, self.state.thread_id, self.title)
            _log("title_proposed", outcome=outcome, **self.ids)
        # The process ended, so nothing waits on the thread any more: the collapsed run line
        # says so at once, also after a promise to go on later (issue 553).
        await self._finish("completed", summary=cause(RUN_ENDED_IDLE))

    async def _fail(
        self,
        reason: str,
        errors: tuple[str, ...],
        exit_label: str,
        *,
        summary: str | None = None,
    ) -> None:
        """``failed`` with ``reason``; for ``runtime_error`` the box note with the texts first.

        ``summary`` names the cause for the thread (RUNNER-NO-GIT).
        """
        if reason == "runtime_error":
            detail = {"errors": list(errors), "exit": exit_label}
            await self._box_note("runtime_error", "failed", detail)
        await self._finish("failed", reason=reason, summary=summary)

    async def _finish_cancelled(self) -> None:
        """``cancelled`` with the request's reason; no answer (note 6 b).

        An orderly stop of the runner (``adapter_stopped``, D-AR-1) names its cause for the
        thread in the summary, with the background tasks the stop ended (issue 553).
        """
        reason = self.stop_reason or "user_cancelled"
        summary = adapter_stopped_cause(self.stopped_tasks) if reason == ADAPTER_STOPPED else None
        await self._finish("cancelled", reason=reason, summary=summary)

    def _report_cost(self) -> None:
        """Patch ``/cost`` with the run's amount so far when it changed (FIX-cost-display)."""
        amount = self.live_cost.amount
        if amount != self._cost_sent:
            self._cost_sent = amount
            self.handle.patch.set_cost(amount, _RUNTIME_CURRENCY)

    def _settle_cost(self) -> None:
        """At a ``result``: the exact amount the run would book now, then report it."""
        result = self.ledger.cost_result
        if result is None:
            return
        store = UsageStore(self.cfg.state_dir / "sessions" / self.session.ref)
        reports, _, _ = self._run_reports(store, result, warn=False)
        self.live_cost.settle(sum(_n(r.amount_micros) for r in reports))
        self._report_cost()

    def _run_reports(
        self, store: UsageStore, result: dict[str, Any], *, warn: bool = True
    ) -> tuple[list[CostReport], dict[str, ModelUsage], int | None]:
        """The run's reports from ``result`` against the stored session values (09 18.7).

        ``warn`` logs an ignored baseline; the live preview leaves that to the finish.

        Returns:
            The reports, the session values of ``result`` and the remaining baseline.
        """
        stored = store.load()
        current = parse_model_usage(result)
        reports = cost_reports(difference(stored.values, current))
        remaining = stored.baseline_remaining
        if remaining is None and not stored.values:
            remaining = self._start_baseline(warn=warn)
        if remaining is not None:
            if counters_reset(stored.values, current):
                # The runtime counts again: the old amounts are no longer in its sums.
                remaining = 0
            reports, remaining = subtract_baseline(reports, remaining)
        return reports, current, remaining

    def _usage(self) -> tuple[list[CostReport], RunMetrics | None]:
        """Cost reports and metrics from the run's last ``result`` (none without one, 12 f 2)."""
        result = self.ledger.cost_result
        if result is None:
            return [], None
        store = UsageStore(self.cfg.state_dir / "sessions" / self.session.ref)
        reports = store.pending_for(str(self.handle.run_id))
        if reports is None:
            reports, current, remaining = self._run_reports(store, result)
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

    def _rebase_usage(self) -> None:
        """Store the totals the resume restores, before the run's first process if it resumes.

        The pinned CLI restores the session's sums from the last ``cost-state`` line of its
        transcript and counts on from there; with them as the stored values the run books only
        its own turns, also for a session the runner never saw (09 18.7, RUNNER-RESUME-COST).
        Without such a line the stored values and the cost baseline stay as they are.
        """
        config_dir = claude_config_dir(self.state.env)
        path = find_transcript(config_dir, self.session.ref) if config_dir else None
        totals = transcript_totals(path) if path is not None else None
        if totals is None:
            return
        UsageStore(self.cfg.state_dir / "sessions" / self.session.ref).rebase(totals)
        _log("session_totals_taken", models=len(totals), **self.ids)

    def _start_baseline(self, *, warn: bool = True) -> int | None:
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
            if warn:
                _warn("cost_baseline_currency_ignored", **self.ids)
            return 0
        return int(baseline["micros"])

    async def _finish(
        self, state: str, *, reason: str | None = None, summary: str | None = None
    ) -> None:
        """``finishRun`` once; a lost answer is sent again with the same body (D-M2-34-4).

        The decided end goes into the turn record first, then the costs into the session's
        usage file, then the call (design note 12 f 2): a runner that adopts a record with a
        decided end repeats exactly this, with the same costs (`UsageStore.pending_for`).
        """
        self.ledger.finishing = {"state": state, "reason": reason, "summary": summary}
        self._persist()
        reports, metrics = self._usage()
        kwargs: dict[str, Any] = {"costs": reports}
        if reason is not None:
            kwargs["reason"] = reason
        if summary is not None:
            kwargs["summary"] = summary
        if metrics is not None:
            kwargs["metrics"] = metrics
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
        if kwargs.get("summary") is not None:
            body.summary = kwargs["summary"]
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
            **self.ledger.to_record(),  # kind, offset, answers, inputs (design note 12 f)
            "started_at": time.time(),
            "stream": str(paths.stream),
            "stderr": str(paths.stderr),
            "system_prompt": str(paths.system_prompt),
            "input": str(paths.input),
            "followups": self.followups.to_record(),
            "recovery": {
                "compacted": self.recovery.compacted,
                "session_replaced": self.recovery.session_replaced,
                "session_resumed": self.recovery.session_resumed,
            },
            "stop": self.stop_reason,
            "title_request": self.title_request,
            "title": self.title,
            "session_version": self.session_version,
            "cost_baseline": self.cost_baseline,
            "steps": self.live.state() if self.live is not None else None,
            "cwd": str(self.state.cwd) if self.state.cwd is not None else None,
        }
        if self.handover:
            record.update(self._handover_part())
        if process is not None:
            record.update(pid=process.pid, pgid=process.pgid, identity=self._identity)
        elif self.adopted_record is not None and not self.handover:
            for key in ("pid", "pgid", "identity", "started_at"):
                record[key] = self.adopted_record.get(key)
        write_turn_record(paths.record, record)

    def _handover_part(self) -> dict[str, Any]:
        """The members of the record with ``runner.turn_handover`` (18.19.5, 18.19.6).

        The phase, the run's parts a next runner needs to build a command line again (never a
        text, a token or the environment), the current attempt with its launch files and
        flags, the run's highest outage number, the inputs of a retry and a line in flight.
        """
        state = self.state
        part: dict[str, Any] = {
            "phase": self.record_phase,
            "run": {
                "model": state.model,
                "tools": list(state.tools),
                "mode": state.mode,
                "plugin_dirs": [str(d) for d in state.plugin_dirs],
                "stored_files": state.stored_files,
                "own_uuid": self.own_uuid,
            },
            "outage": self.outage,
            "autonomous": sorted(self.autonomous),
        }
        if self.launch is not None:
            part.update(self.launch.to_record())
        if self.retry_inputs is not None:
            part["retry_inputs"] = [u for u, _ in self.retry_inputs]
        if self.record_phase == "compaction_retry":
            part["retry"] = {"hint": self.retry_hint}
        if self.input_write is not None:
            part["input_write"] = dict(self.input_write)
        return part

    async def _checkpoint(self, read_to: int) -> None:
        """Store the offset after the patches of the lines before it went out (12 f 1).

        A patch that failed keeps the old offset: the next runner reads those lines again, so
        their steps and ends reach the server. The ledger never lets the offset pass the run's
        end evidence while the run is not finished.
        """
        try:
            await self.handle.patch.flush()
        except (ProblemError, httpx.TransportError):
            self._persist()
            return
        self.ledger.checkpoint(read_to)
        self._persist()

    # ---- handover and adoption with full rights (09 18.19.3 to 18.19.7) ---------------------

    async def offer_input(self, text: str, *, autonomous: bool = False) -> bool:
        """Write one further input line into an open stdin, like a follow-up (18.19.7).

        The path of a wake input and of a nudge: the line is recorded with its ``uuid`` before
        it is written and marked written after its last byte (18.19.3). The checks of a wake
        input (5.8.3) are the caller's.

        Args:
            text: The text of the user line.
            autonomous: The line is an autonomous input (counted by `ProcessState`, 5.8.6).

        Returns:
            True once every byte reached the pipe; False when stdin is not open.
        """
        async with self.write_lock:
            process = self.process
            if process is None or self.phase != "open" or self.stop_reason is not None:
                return False
            line, line_uuid = user_line([text])
            data = line.encode("utf-8") + b"\n"
            self.lines[line_uuid] = data
            self.ledger.written(line_uuid)
            if autonomous:
                self.autonomous.add(line_uuid)
                self.ps.flag_input(line_uuid, autonomous=True)
            if self.handover:
                self._store_lines([(line_uuid, data)])
                self.input_write = {"uuid": line_uuid, "bytes": None}
            self._persist()
            written = await process.write_bytes(data)
            if self.handed_over:
                return False
            self.input_write = None
            if not written:
                self.ledger.unwritten(line_uuid)
            self._persist()
        return written

    def takes_followups(self) -> bool:
        """Whether this adopted run takes follow-ups into its turn (18.19.7).

        A turn whose stdin was closed and a compaction are adopted with reading rights only:
        their follow-ups stay ``queued`` and the server defers them (09 7).
        """
        if not self.handover or self.phase in ("closed", "ended"):
            return False
        launch = self.launch
        return launch is None or not (launch.stdin_closed or launch.kind == "compaction")

    async def hand_over(self) -> bool:
        """Leave the run to the next runner at SIGTERM (18.19.3 item 2).

        An input line in flight is finished within ``runner.kill_grace`` or recorded with the
        count of bytes written (``partial_input``); a launch whose go byte is not recorded gets
        its launch pipe closed without a go byte; the record stays.

        Returns:
            True when the record is left for the next runner (phase ``transcribing`` on).
        """
        self.handed_over = True
        process = self.process
        if isinstance(process, KeeperGroup):
            await self._settle_input_write(process)
        if self.keeper_start is not None and (self.launch is None or not self.launch.go_sent):
            self.keeper_start.close()  # before step 3: the keeper starts no CLI
        if self.phase == "ended" or self.record_phase not in _KEPT_PHASES:
            return False
        self._persist()
        return True

    async def _settle_input_write(self, process: KeeperGroup) -> None:
        """Finish or record the input line in flight at a handover (18.19.3).

        A line whose last byte reached the pipe, within ``runner.kill_grace`` or before the
        handover, is no longer recorded as started; a cut line keeps the count of its bytes
        written (``partial_input``), counted from the start of the line, so the adopter writes
        the rest first. The writer's own task may not resume before the record is saved, so
        the count comes from the pipe writer here.
        """
        writer = process.writer
        if writer.in_flight:
            loop = asyncio.get_running_loop()
            deadline = loop.time() + self.cfg.kill_grace
            while writer.in_flight and loop.time() < deadline:
                await asyncio.sleep(self.cfg.poll_interval)
        pending = self.input_write
        if pending is None:
            return
        cut = writer.in_flight
        if cut:
            await writer.abort()
        count = pending.get("bytes")
        done = count if isinstance(count, int) and not isinstance(count, bool) else 0
        done += writer.written
        line = self.lines.get(str(pending.get("uuid")))
        if line is not None and done >= len(line):
            self.input_write = None
            return
        self.input_write = {"uuid": pending.get("uuid"), "bytes": done}
        _log("partial_input", bytes=done, **self.ids)

    async def take_over(self, record: dict[str, Any], known_steps: Mapping[str, str]) -> None:
        """Continue a run that another runner handed over or left by a crash (18.19.5 to 7).

        By phase: ``transcribing`` starts over after the launch check; ``compaction_retry``
        starts the recorded retry through the handshake; ``launching`` follows the cases of
        18.19.5 (relaunch the same attempt with the next try, adopt a live CLI, or take the end
        of one that ran); ``process`` adopts a live CLI with full rights or takes its end. The
        end of an adopted process is handled like an own one (the adoption rule, R-AG-47), and
        every launch is preceded by ``run.launch.check`` (R-AG-48).

        Args:
            record: The turn record.
            known_steps: The status of each step of the server's run, by step id.
        """
        self.state.turn = self
        self.task = asyncio.current_task()
        self.accepted = False
        self.adopted_record = record
        phase = str(record.get("phase") or "")
        self._restore(record, known_steps, fresh=phase == "transcribing")
        try:
            if self.ledger.finishing is not None:
                # The previous runner decided the end and may have stored its costs: repeat it.
                decided = self.ledger.finishing
                await self._finish(
                    str(decided["state"]), reason=decided["reason"], summary=decided.get("summary")
                )
                return
            await self._continue_phase(phase, record)
        except asyncio.CancelledError:
            await self._on_cancelled()
            raise
        finally:
            self._end()

    async def _restart_transcribing(self) -> None:
        """No CLI ran for the run: started over after the launch check (18.19.6).

        A stop that came first ends it cancelled; the launch check would return without a finish.
        """
        if self._stop_requested():
            await self._finish_cancelled()
            return
        if not await self._launch_check() or not await self._precheck_again():
            return
        self.launch_checked = True  # the check of this start counts for its first launch
        await self._reload_context()
        await self._open_turn()
        await self._drive(_Attempt(resume=self.session.resume), precheck=False)

    async def _continue_phase(self, phase: str, record: Mapping[str, Any]) -> None:
        if phase == "transcribing":
            await self._restart_transcribing()
            return
        if phase == "compaction_retry":
            retry = _Attempt(resume=True, hint=self.retry_hint, label="retry")
            await self._drive(retry, precheck=True)
            return
        launch = AttemptRecord.from_record(record)
        if launch is None:
            await self._fail("runtime_error", (), "unknown")
            return
        self.launch = launch
        attempt = _attempt_of(launch)
        self.attempt = attempt
        self.session = SessionChoice(
            ref=launch.session_ref or self.session.ref,
            resume=launch.resume,
            new=launch.new_reference,
        )
        if phase == "launching":
            settled = await settle_launching(
                launch.files,
                go_sent=launch.go_sent,
                keeper=launch.keeper,
                session_ref=self.session.ref,
                kill_grace=self.cfg.kill_grace,
                poll=self.cfg.poll_interval,
                timeout=self.cfg.cli_probe_timeout,
            )
            _log("launch_settled", case=settled.value, **self.ids)
            if settled is Settled.RELAUNCH:
                await self._drive(attempt, precheck=True, repeat=launch)
                return
        outcome = await self._adopt_process(launch, attempt, record)
        if outcome is None or outcome.handled:
            return
        await self._drive(attempt, precheck=True, outcome=outcome)

    def _restore(
        self, record: Mapping[str, Any], known_steps: Mapping[str, str], *, fresh: bool
    ) -> None:
        """The run's state from a record with a named pipe (a takeover)."""
        self.ledger = TurnLedger.from_record(record)
        self.title = str(record.get("title") or "")
        self.title_request = bool(record.get("title_request"))
        self.cost_baseline = _stored_baseline(record.get("cost_baseline"))
        self.followups = PendingFollowups.from_record(record.get("followups"))
        version = record.get("session_version")
        if isinstance(version, str) and version:
            self.session_version = version
        if record.get("stop") and self.stop_reason is None:
            self.stop_reason = str(record["stop"])
        self.recovery = RecoveryState.from_record(
            record.get("recovery"), compact_supported=COMPACT_IN_PRINT_MODE
        )
        launch = AttemptRecord.from_record(record)
        self.attempt_no = launch.number if launch is not None else 0
        outage = record.get("outage")
        self.outage = outage if isinstance(outage, int) and outage >= 0 else 0
        self.lines = self._load_lines()
        retry = record.get("retry_inputs")
        if isinstance(retry, list):
            self.retry_inputs = [(u, self.lines[u]) for u in retry if u in self.lines]
        hint = record.get("retry")
        self.retry_hint = isinstance(hint, Mapping) and hint.get("hint") is True
        autonomous = record.get("autonomous")
        self.autonomous = {u for u in autonomous if isinstance(u, str)} if autonomous else set()
        run = record.get("run")
        if isinstance(run, Mapping) and isinstance(run.get("own_uuid"), str):
            self.own_uuid = str(run["own_uuid"])
        self.report_due = False  # the accepting runner reported the session
        if fresh:
            return  # started over: the live box and the costs begin as for a new run
        self.rebased = True  # the run's first process ran already (D-RUNNER-RESUME-COST-2)
        self.live = LiveBox(
            self.handle.patch,
            own_uuid=self.own_uuid,
            title_requested=False,
            context_windows={},
            now=lambda: datetime.now(UTC),
            progress_interval=self.cfg.progress_patch_interval,
        )
        self.live.restore(record.get("steps"), known_steps)

    async def _reload_context(self) -> None:
        """The context and the attachments of a run started over from ``transcribing``."""
        self.state.context = await self.handle.load_context()
        files_dir = self.cfg.state_dir / "files" / str(self.state.thread_id)
        self.state.file_texts, self.state.stored_files = await fetch_attachments(
            self.handle.client, self.state.context.trigger_message.attachments, files_dir
        )

    async def _adopt_process(
        self, launch: AttemptRecord, attempt: _Attempt, record: Mapping[str, Any]
    ) -> _Outcome | None:
        """Adopt the recorded CLI with full rights, or take the end of one that ran (18.19.7).

        The lines from ``process_start`` to the record's ``offset`` are replayed through
        `ProcessState.apply` with the adoption time (they post, patch and plan nothing), then
        the stream is read on from ``offset`` as an own turn's. Stdin is taken only for a
        process that is no compaction and whose stdin was not closed.
        """
        files = launch.files
        cli = dict(launch.cli or read_json(files.cli) or {})
        keeper = dict(launch.keeper or read_json(files.keeper) or {})
        keeper.setdefault("pgid", keeper.get("pid"))
        cli.setdefault("pgid", keeper.get("pid"))
        identity = record.get("identity")
        if isinstance(identity, Mapping) and identity.get("lstart") and not cli.get("lstart"):
            cli["lstart"] = identity["lstart"]
        group = KeeperGroup(
            files,
            keeper=keeper,
            cli=cli,
            session_ref=self.session.ref,
            timeout=self.cfg.cli_probe_timeout,
            verified=process_matches(record, self.cfg.cli_probe_timeout),
        )
        self.process = group
        self.adopted_group = group
        self._identity = {k: str(cli[k]) for k in ("lstart", "command") if cli.get(k)}
        self.record_phase = "process"
        self.ps = ProcessState(has_stdin=True, outage=launch.outage, autonomous=self.autonomous)
        self._held_logged = False
        lines = await replayed_lines(
            self._paths().stream, self.ledger.process_start, self.ledger.offset
        )
        now = self.monotonic()
        self._replayed_worked = self._replay_ledger(lines)
        replayed = self.ps.replay([m for m, _, _ in lines], now)
        _log("turn_replayed", lines=replayed, **self.ids)
        self.phase = "closed"
        rights = not launch.stdin_closed and launch.kind != "compaction"
        if rights and group.alive():
            rights = await self._finish_partial_input(group, record)
        if rights and group.alive():
            if launch.inputs_written:
                self.phase = "open"
            else:
                self.phase = "starting"  # step 6 of 18.19.5: wait for <launch>.fed
                self._fed = asyncio.ensure_future(self._await_fed(group))
        self._persist()  # every adopter writes process_start again
        return await self._follow(
            group, attempt, self.ledger.offset, launch.stderr_start, adopted=True
        )

    def _replay_ledger(self, lines: Sequence[tuple[dict[str, Any], int, int]]) -> bool:
        """Classify the replayed lines in the ledger (no posting); whether the process worked.

        Worked as in `_read`: an ``assistant`` message after ``init`` that is no API error, so a
        retry after the handover still warns that the earlier attempt may have changed files.
        """
        worked = False
        for message, line_start, line_end in lines:
            self.ledger.line(message, line_start, line_end)
            if message.get("type") == "assistant" and self.ledger.init_seen:
                worked = worked or api_error_text(message) is None
        return worked

    async def _finish_partial_input(self, group: KeeperGroup, record: Mapping[str, Any]) -> bool:
        """The rest of a line a handover cut, before any other input (18.19.3).

        Returns:
            True when stdin stays open; False after a crash in the middle of a write (a line
            marked started without a byte count): then stdin is closed and nothing is written.
        """
        pending = record.get("input_write")
        if not isinstance(pending, Mapping):
            return True
        line_uuid, count = pending.get("uuid"), pending.get("bytes")
        line = self.lines.get(str(line_uuid))
        if isinstance(count, int) and not isinstance(count, bool) and line is not None:
            self.input_write = {"uuid": line_uuid, "bytes": count}
            if await group.write_bytes(line[count:]):
                self.input_write = None
                self._persist()
                return True
            return False
        self.input_write = None
        await self._close_stdin(group)
        return False

    async def _launch_check(self) -> bool:
        """``run.launch.check`` before a launch of a run this runner did not accept (R-AG-48).

        On allow the runner compares its own tool class and billing mode with the answer. A
        refusal launches nothing and finishes the run ``failed`` with the reason (log code
        ``launch_check_refused``); a refusal because the server already ended the run finishes
        nothing. When the server cannot be reached, the check is tried again with backoff and
        the run stays in ``active_runs``.

        Returns:
            True on allow.
        """
        client = self.handle.client
        settings = self.cfg.sdk
        tries = 0
        while True:
            if self._stop_requested() or self.handed_over or self._handing_over():
                return False
            try:
                result = await client.call_as(
                    RunWakeCheckResult, check_run_launch.asyncio_detailed, run_id=self.handle.run_id
                )
            except (ServerError, RateLimitedError, httpx.TransportError):
                _log("launch_check_unreachable", **self.ids)
                await client.clock.sleep(
                    min(settings.retry_backoff_cap, settings.retry_backoff_base * 2**tries)
                )
                tries += 1
                continue
            except ProblemError as err:
                reason = err.reason or err.code
                _warn("launch_check_refused", reason=reason, **self.ids)
                if reason in ("run_not_running", "run_inactive"):
                    self.server_ended = True
                    self._ended_by_server()
                    return False
                await self._fail(reason, (), "not_started")
                return False
            mismatch = None
            if str(getattr(result.tool_class, "value", result.tool_class)) != self.cfg.tool_class:
                mismatch = "tool_class_mismatch"
            elif str(getattr(result.billing_mode, "value", result.billing_mode)) != self.state.mode:
                mismatch = "billing_mode_mismatch"
            if mismatch is not None:
                _warn("launch_check_refused", reason=mismatch, **self.ids)
                await self._fail(mismatch, (), "not_started")
                return False
            _log("launch_check_allowed", **self.ids)
            return True

    # ---- adoption (design note 12) ---------------------------------------------------------

    async def adopted(
        self, record: dict[str, Any], group: StoppableGroup, known_steps: Mapping[str, str]
    ) -> None:
        """Follow a turn that outlived the previous runner to its end and finish it once.

        No process is started and no follow-up is written (its stdin ended with the old runner);
        the end is classified without recovery (09 1 point 5, R-AG-17). The stream is read by
        the same loop as an own turn's (`_read`), with the ledger of the record (design note
        12 f: its offset, answers, inputs and kind).

        Args:
            record: The turn record.
            group: The adopted process group (identity checked before every signal).
            known_steps: The status of each step of the server's run, by step id.
        """
        self.adopted_record = record
        self.adopted_group = group
        self.ledger = TurnLedger.from_record(record)
        self.ps = ProcessState(has_stdin=False)  # reading rights only (18.9)
        self.phase = "closed"
        self.title = str(record.get("title") or "")
        self.title_request = bool(record.get("title_request"))
        self.cost_baseline = _stored_baseline(record.get("cost_baseline"))
        self.followups = PendingFollowups.from_record(record.get("followups"))
        if record.get("stop"):
            self.stop_reason = str(record["stop"])
        self.live = LiveBox(
            self.handle.patch,
            own_uuid="",
            title_requested=False,
            context_windows={},
            now=lambda: datetime.now(UTC),
            progress_interval=self.cfg.progress_patch_interval,
        )
        self.live.restore(record.get("steps"), known_steps)
        done = False
        try:
            if self.ledger.finishing is not None:
                # The previous runner decided the end and may have stored its costs: repeat it.
                decided = self.ledger.finishing
                await self._finish(
                    str(decided["state"]),
                    reason=decided["reason"],
                    summary=decided.get("summary"),
                )
                done = True
                return
            reader = StreamReader(
                Path(str(record["stream"])),
                self.cfg.poll_interval,
                group.alive,  # the leader: a leftover job of its group never holds the reader
                offset=self.ledger.offset,
                until_exit=True,
            )
            await self._read(reader, group, None)
            if self.stopping is not None:
                await self.stopping
            elif _group_runs(group):
                # Like an own turn: nothing of the group outlives the run (only a group whose
                # identity was verified is signalled, AdoptedGroup.may_signal).
                await self._stop(group)
            await self._checkpoint(reader.offset)
            if self.server_ended:
                done = True
                return
            end = classify_turn_end(
                TurnEvidence(
                    stop_requested=self.stop_reason is not None,
                    init_seen=self.ledger.init_seen,
                    result=self.ledger.end_result(),
                    assistant_text=self.ledger.api_error,
                    exit_label="unknown",
                )
            )
            if end.outcome == ErrorKind.VERSION_UNSUPPORTED.value:
                await self.h.runtime_too_old(end.required_version)
            await self._end_adopted(end.outcome, end.errors)
            done = True
        finally:
            # An orderly stop of this runner keeps the record for the next start (D-M2-35-20).
            if done:
                self.phase = "ended"
                remove_turn_record(self.cfg.state_dir / "turns" / f"{self.handle.run_id}.json")
                remove_run_files(self.cfg.state_dir, self.handle.run_id)

    async def _end_adopted(self, outcome: str, errors: tuple[str, ...]) -> None:
        if outcome == "stopped":
            await self._finish_cancelled()
        elif outcome == "completed" and self.ledger.kind == "turn":
            await self._complete()
        elif outcome == "not_authenticated":
            await self._fail("runtime_not_authenticated", (), "unknown")
        else:
            # Also an adopted compaction: it is never posted and never completes the run.
            await self._fail("runtime_error", errors, "unknown")


def _group_runs(group: StoppableGroup) -> bool:
    from claude_code_runner.stop import group_alive  # noqa: PLC0415 - keeps the import list flat

    return group_alive(group)


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
