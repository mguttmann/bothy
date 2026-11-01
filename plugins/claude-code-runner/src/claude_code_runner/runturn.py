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
import time
from collections.abc import Callable, Mapping, Sequence
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
from bothy_sdk.generated.api.agent_runtime import (
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
    RunFinish,
    RunFinishState,
    RunMetrics,
    TranscriptSubmit,
)
from bothy_sdk.generated.types import Unset
from bothy_sdk.long_answers import split_for_limit
from bothy_sdk.outbox import OutboxState
from bothy_sdk.runs import RunHandle, is_run_ended

from claude_code_runner.adoption import read_identity
from claude_code_runner.attachments import followup_file_texts, followup_files_final
from claude_code_runner.background import BackgroundWork
from claude_code_runner.causes import cause
from claude_code_runner.commandline import SessionChoice, build_argv, reported_model
from claude_code_runner.failures import (
    ErrorKind,
    TurnEnd,
    TurnEvidence,
    api_error_text,
    classify_turn_end,
)
from claude_code_runner.followups import (
    FollowupCheck,
    FollowupVerdict,
    PendingFollowups,
    build_followup_line,
    followup_verdict,
    stdin_close_due,
)
from claude_code_runner.ledger import TurnLedger, is_success
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
from claude_code_runner.transcribe import LOCAL_FILE, notice, transcribe_attachments
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
    from claude_code_runner.app import RunnerHandler

__all__ = ["COMPACT_IN_PRINT_MODE", "RunState", "RunTurn"]

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

    monotonic: Callable[[], float] = staticmethod(time.monotonic)
    """The clock of the stdin window and of the offset saves (a test sets a fake one)."""

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
        self.background = BackgroundWork()
        """The background work of the current process (09 18.5, FIX-runner-responsive-turns)."""
        self._held_logged = False
        self.process: Turn | None = None
        self.phase = "starting"  # starting, open, closed, ended
        self.attempt = _Attempt()
        self.ledger = TurnLedger.begin("turn", 0)
        """The recovery bookkeeping of the current process (design note 12 f)."""
        self.stop_reason: str | None = state.stop_reason
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
        self.stopping: asyncio.Task[None] | None = None
        self.adopted_group: StoppableGroup | None = None
        self.adopted_record: dict[str, Any] | None = None
        self._identity: dict[str, str] = {}
        self.upload: asyncio.Task[Any] | None = None
        """The upload of an answer's attachment set while it runs (a stop cancels it)."""
        self.rebased = False
        """The run's first process started; only it takes the session totals (09 18.7)."""

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
            remove_run_files(self.cfg.state_dir, self.handle.run_id)

    def _stop_requested(self) -> bool:
        """Whether a stop came (it can arrive at every ``await`` of the run)."""
        return self.stop_reason is not None

    async def _transcribe(self, context: RunContext) -> None:
        """Transcripts of the trigger's audio attachments before the first input (09 18.14).

        A transcript is set on its attachment and delivered to the server (09 13); an audio file
        without one gets a note in the thread. A stop ends the transcriber at once.
        """
        if not any(text.startswith(LOCAL_FILE) for text in self.state.file_texts.values()):
            return  # no stored attachment, nothing to transcribe
        status: list[bool] = []

        async def on_start() -> None:
            status.append(True)
            await self._status_line("Transcribing a voice message")

        task = asyncio.ensure_future(
            transcribe_attachments(
                context.trigger_message.attachments,
                self.state.file_texts,
                self.h.transcriber,
                on_start,
            )
        )
        while not task.done():
            await asyncio.wait({task}, timeout=self.cfg.poll_interval)
            if self._stop_requested() and not task.done():
                task.cancel()
                with contextlib.suppress(asyncio.CancelledError):
                    await task
                return
        done, missing = task.result()
        if status:
            await self._status_line(None)
        for item in done:
            await self._submit_transcript(item.file_id, str(item.transcript))
        if missing and not self._stop_requested():
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
                await self._fail(
                    "adapter_error", (), outcome.evidence.exit_label, summary=cause("session_busy")
                )
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
            await self._fail(
                refused.reason or "runtime_error", (), "not_started", summary=refused.summary
            )
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
        if not self.rebased:
            # Only the run's first process: a later one resumes the run's own session, whose
            # stored values already hold what the run saw (D-RUNNER-RESUME-COST-2).
            self.rebased = True
            if attempt.resume:
                await asyncio.to_thread(self._rebase_usage)
        try:
            process = Turn.start(
                argv, cwd=cwd, env=self.state.env, paths=paths, pass_fds=self.state.locks
            )
        except OSError:
            _warn("turn_start_failed", **self.ids)
            await self._fail("runtime_error", (), "start_failed")
            return None
        self.process = process
        self.background = BackgroundWork()
        self._held_logged = False
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
        try:
            reader = StreamReader(
                process.paths.stream,
                self.cfg.poll_interval,
                process.alive,
                offset=stream_start,
                until_exit=True,
            )
            outcome = await self._read(reader, process, attempt)
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
                stderr=_read_from(process.paths.stderr, stderr_start),
                exit_label=process.exit_label(),
            )
            return _Outcome(evidence, self._worked, compacted=ledger.compacted)
        finally:
            self.process = None
            if self.phase != "ended":
                self.phase = "starting"

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
        held, self.state.held = self.state.held, []
        for inv in held:
            await self.offer_followup(inv)

    async def _read(  # noqa: PLR0912 - one branch per message kind of 18.4
        self, reader: StreamReader, group: StoppableGroup, attempt: _Attempt | None
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
        self._worked = False
        last_result_at: float | None = None
        saved_at = self.monotonic()
        interval = self.handle.patch.settings.run_update_min_interval
        async for message in reader.items():
            if message is None:
                await self._idle(group, last_result_at)
                if self.monotonic() - saved_at >= interval:
                    saved_at = self.monotonic()
                    await self._checkpoint(reader.offset)
                continue
            self.ledger.line(message, reader.line_offset, reader.offset)
            self.background.apply(message, self.monotonic())
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
            elif kind == "user" and message.get("isReplay") is True:
                await self._echo(message.get("uuid"), adopted=not own)
            elif kind == "result":
                last_result_at = self.monotonic()
                if self.ledger.kind == "turn" and is_success(message):
                    await self._answer(message, reader.line_offset)
            if self.ledger.kind == "turn" and self.live is not None:
                self.live.apply(message)  # a compaction is never mapped (note 8 b)
                if self.live.steps_changed:
                    self._persist()  # the step ids before their patches go out (note 12 b)
            if own and kind == "result" and not is_success(message):
                # A failed turn absorbs nothing more: stdin closes now, and the follow-ups
                # without echo go to the next process of a recovery (notes 4 d and 5).
                await self._close_stdin(group)
            elif own and kind == "result":
                await self._idle(group, last_result_at)
        return None

    async def _idle(self, process: StoppableGroup, last_result_at: float | None) -> None:
        """Checks while a turn runs: a run the server ended, and the stdin rule of note 5."""
        if self.handle.patch.ended and not self.server_ended and self.stop_reason is None:
            self.server_ended = True
            _log("run_ended_by_server", **self.ids)
        if (self.server_ended or self.stop_reason is not None) and self.stopping is None:
            # Also a stop that came while this process was being started (note 6 d).
            self._stop_group(process)
            return
        if self.phase == "open" and self.background.holds(
            self.monotonic(), self.cfg.followup_grace
        ):
            # Background work outlives the turn: the CLI stops it when its input ends, so stdin
            # stays open, and every follow-up becomes a turn of its own (09 18.5).
            if self.background.running and self.live is not None:
                self.live.keepalive()
            if last_result_at is not None and not self._held_logged:
                self._held_logged = True
                _log("stdin_held_for_background", **self.ids)
            return
        pending = len(self.followups.pending())
        if self.phase == "open" and stdin_close_due(
            self.monotonic(), last_result_at, pending, self.cfg.followup_grace
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
        if self.upload is not None:
            self.upload.cancel()  # the request in flight of an answer's set (M3-12 note 10 d)
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
        read: tuple[Message | None] | None = None
        if self.phase == "open" and message_id not in self.followups:
            # Outside the lock, so other follow-ups and the stop go on while a file is checked.
            read = (await self._await_followup_files(message_id, ids),)
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
            message = read[0] if read is not None else await self._read_message(message_id, ids)
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
            files = await followup_file_texts(
                self.handle.client,
                message.attachments,
                self.cfg.state_dir / "files" / str(self.state.thread_id),
            )
            line = build_followup_line(author, message.content_md, line_uuid, files)
            # The record knows the line before the CLI can: a runner that adopts the turn maps
            # its echo to the follow-up and patches it absorbed (design note 12 f 3).
            self.followups.add(message_id, line_uuid, line)
            self.lines[line_uuid] = line
            self.ledger.written(line_uuid)
            self._persist()
            if not await process.write_bytes(line):
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
        await self._finish("completed")

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
        """``cancelled`` with the request's reason; no answer (note 6 b)."""
        await self._finish("cancelled", reason=self.stop_reason or "user_cancelled")

    def _usage(self) -> tuple[list[CostReport], RunMetrics | None]:
        """Cost reports and metrics from the run's last ``result`` (none without one, 12 f 2)."""
        result = self.ledger.cost_result
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
        if process is not None:
            record.update(pid=process.pid, pgid=process.pgid, identity=self._identity)
        elif self.adopted_record is not None:
            for key in ("pid", "pgid", "identity", "started_at"):
                record[key] = self.adopted_record.get(key)
        write_turn_record(paths.record, record)

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
