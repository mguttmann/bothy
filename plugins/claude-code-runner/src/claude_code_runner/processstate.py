# SPDX-License-Identifier: Apache-2.0
"""The per-process state that stream lines produce, and its replay (09 18.19.7).

Every value that the stdin rule of 18.5 and the stop after an error read and that stream lines
produce lives in one `ProcessState` per turn process, and `ProcessState.apply` is its only
writer: the background work (`claude_code_runner.background.BackgroundWork`), the time of the
last ``result`` that answered an input, the failure mark with the count of running tasks, the
task list with each task's description, the usage mark with the outage number and the usage
error mark (18.19.10), the nudge mark (18.19.11), and the idle flag and the count of autonomous
inputs (5.8.6). `ProcessState.decide` is the stdin and stop rule; it reads only the object and
what the runner passes in (whether stdin is open, the pending follow-ups, the configuration).

The live read and the replay call the same ``apply``: a runner that adopts a turn with a named
pipe first replays the lines from the ledger's ``process_start`` to the record's ``offset`` with
the adoption time (`replayed_lines`, `ProcessState.replay`), then reads on. Replayed lines get
the adoption time, so the quiet window, the window of ``stdin_close_due`` and the stop grace after
an error start again at the adoption: stdin is held longer, never shorter, and a failed turn is
stopped at most ``runner.stop_grace`` later than by the old runner. The replay posts nothing,
patches nothing, plans no answer and moves no offset; it only changes this object.

The usage values are carried through record and replay here; the stream reader learns the frame
``rate_limit_event`` with AR-06, which acts on the mark. The nudge mark and the autonomous count
are fed by the input flags of the record, which AR-05 and AR-07 write.

Examples:
    >>> state = ProcessState(has_stdin=True)
    >>> state.apply({"type": "result", "subtype": "success"}, 10.0)
    >>> state.decide(10.0, stdin_open=True, pending=0, followup_grace=30.0, stop_grace=10.0)
    <Decision.CLOSE: 'close'>
"""

from __future__ import annotations

from collections.abc import Collection, Iterable, Mapping
from dataclasses import dataclass
from enum import Enum
from pathlib import Path
from typing import Any

from claude_code_runner.background import BackgroundWork
from claude_code_runner.followups import stdin_close_due
from claude_code_runner.streamreader import StreamReader

__all__ = ["Decision", "ProcessState", "UsageMark", "replayed_lines"]

_ALLOWED = frozenset({"allowed", "allowed_warning"})
_WAKE_COMMAND = "bothy-wake"
"""A tool call of this command alone does not make an autonomous turn count as work (5.8.6)."""


class Decision(Enum):
    """What the stdin and stop rule asks for now (`ProcessState.decide`)."""

    NONE = "none"
    """Nothing to do."""
    HOLD = "hold"
    """Background work holds stdin open (09 18.5)."""
    WAIT = "wait"
    """An input is queued and not started: the result just read answered none."""
    CLOSE = "close"
    """Close stdin now: after a ``result`` with nothing pending, or after an error ``result``."""
    STOP = "stop"
    """Stop the group: the process outlived an error ``result`` by ``runner.stop_grace``."""


@dataclass(frozen=True)
class UsageMark:
    """The usage mark of a rejected frame (09 18.19.10).

    Attributes:
        resets_at: ``resetsAt`` of the frame (seconds since the epoch), if any.
        rate_limit_type: ``rateLimitType`` of the frame, if any.
    """

    resets_at: int | None
    rate_limit_type: str | None


class ProcessState:
    """The stream-derived state of one turn process; see the module docstring.

    Args:
        has_stdin: The process was started with stdin for the runner: an own turn, or an
            adopted one whose record has a named pipe. Only then does an error ``result`` set
            the failure mark (the ``own`` gate of 18.5 becomes "the turn has stdin").
        outage: The highest outage number of the run reached before this attempt (from its
            ``launching`` record, 18.19.5).
        nudges: The ``uuid`` of every input line flagged as the nudge (18.19.11).
        autonomous: The ``uuid`` of every autonomous input line (a wake input, 5.8.6).

    Attributes:
        background: The background work of the process.
        last_result_at: When the last ``result`` that answered an input was read; ``None``
            before one, and again once a queued input started (``command_lifecycle``).
        failed_at: When the first error ``result`` was read while the usage mark was clear.
        lost_tasks: The running tasks at that error ``result`` (again at the stop).
        tasks: Each task's description by task id, from ``system/background_tasks_changed``.
        usage: The usage mark, or ``None`` while clear.
        outage: The outage number reached so far.
        usage_error: An error ``result`` was read while the usage mark was set.
        nudged: The nudge's echo was read in the current waiting phase.
        idle: The current input's turn made no tool call other than ``bothy-wake``.
        autonomous_inputs: Autonomous inputs taken in a row (reset by a person's input).
        last_line_at: The read time of the last line (the quiet window's clock).
    """

    def __init__(
        self,
        *,
        has_stdin: bool,
        outage: int = 0,
        nudges: Collection[str] = (),
        autonomous: Collection[str] = (),
    ) -> None:
        """Start the state of a process; see the class docstring."""
        self.has_stdin = has_stdin
        self.background = BackgroundWork()
        self.last_result_at: float | None = None
        self.failed_at: float | None = None
        self.lost_tasks = 0
        self.tasks: dict[str, str] = {}
        self.usage: UsageMark | None = None
        self.outage = max(0, outage)
        self.usage_error = False
        self.nudged = False
        self.idle = False
        self.autonomous_inputs = 0
        self._nudges = set(nudges)
        self._autonomous = set(autonomous)

    # ---- the one writer --------------------------------------------------------------------

    def apply(self, message: Mapping[str, Any], now: float) -> None:
        """Take one stream message read (or replayed) at ``now``.

        Args:
            message: A decoded stream message.
            now: The monotonic read time (the adoption time for a replayed line).
        """
        self.background.apply(message, now)
        kind, subtype = message.get("type"), message.get("subtype")
        if kind == "command_lifecycle" and message.get("state") == "started":
            # An input starts a turn of its own: an earlier result does not end it, so the
            # stdin rule waits for the result of this turn (FIX-runner-hold-background).
            self.last_result_at = None
        elif kind == "result":
            if not self.background.input_waiting:
                # A result before a queued input started answers no input: it must not start
                # the stdin rule for the turn of that input (FIX-runner-hold-background).
                self.last_result_at = now
            if not _success(message):
                self._error_result(now)
        elif kind == "system" and subtype == "background_tasks_changed":
            self._task_list(message.get("tasks"))
        elif kind == "rate_limit_event":
            self._usage_frame(message.get("rate_limit_info"))
        elif kind == "user" and message.get("isReplay") is True:
            self._input_taken(message.get("uuid"))
        elif kind == "assistant":
            self._tool_calls(message)

    def _error_result(self, now: float) -> None:
        if not self.has_stdin:
            return
        if self.usage is not None:
            # A usage error never sets the failure mark, also not after the mark clears.
            self.usage_error = True
            return
        if self.failed_at is None:
            self.failed_at = now
            self.lost_tasks = len(self.background.running)

    def _task_list(self, tasks: object) -> None:
        if not isinstance(tasks, list):
            return
        for task in tasks:
            if isinstance(task, Mapping) and isinstance(task.get("task_id"), str):
                description = task.get("description")
                self.tasks[task["task_id"]] = description if isinstance(description, str) else ""

    def _usage_frame(self, info: object) -> None:
        if not isinstance(info, Mapping):
            return
        status = info.get("status")
        if status == "rejected" and info.get("overageStatus") not in _ALLOWED:
            resets = info.get("resetsAt")
            limit = info.get("rateLimitType")
            if self.usage is None:
                self.outage += 1  # an outage starts at a frame that sets a clear mark
            self.usage = UsageMark(
                resets if isinstance(resets, int) and not isinstance(resets, bool) else None,
                limit if isinstance(limit, str) else None,
            )
        elif status != "rejected":
            # allowed, allowed_warning and an unknown status (treated as allowed) clear it.
            self.usage = None

    def _input_taken(self, line_uuid: object) -> None:
        if not isinstance(line_uuid, str):
            return
        self.idle = True
        if line_uuid in self._nudges:
            self.nudged = True
            return
        self.nudged = False  # a new waiting phase starts with any other input
        if line_uuid in self._autonomous:
            self.autonomous_inputs += 1
        else:
            self.autonomous_inputs = 0

    def _tool_calls(self, message: Mapping[str, Any]) -> None:
        body = message.get("message")
        content = body.get("content") if isinstance(body, Mapping) else None
        for block in content if isinstance(content, list) else []:
            if isinstance(block, Mapping) and block.get("type") == "tool_use":
                command = block.get("input", {})
                text = command.get("command") if isinstance(command, Mapping) else None
                if not (isinstance(text, str) and text.strip().startswith(_WAKE_COMMAND)):
                    self.idle = False

    def flag_input(self, line_uuid: str, *, nudge: bool = False, autonomous: bool = False) -> None:
        """Flag an input line before it is written, as the record does (18.19.7).

        Its echo then restores the nudge mark or counts as an autonomous input.

        Args:
            line_uuid: The line's ``uuid``.
            nudge: The line is the nudge of 18.19.11.
            autonomous: The line is an autonomous input (5.8.6).
        """
        if nudge:
            self._nudges.add(line_uuid)
        if autonomous:
            self._autonomous.add(line_uuid)

    # ---- the replay ------------------------------------------------------------------------

    def replay(self, lines: Iterable[Mapping[str, Any]], now: float) -> int:
        """Apply the lines before the stored offset, each with the adoption time.

        Args:
            lines: The decoded lines from ``process_start`` to the record's ``offset``.
            now: The adoption time (monotonic).

        Returns:
            The number of lines replayed.
        """
        count = 0
        for message in lines:
            self.apply(message, now)
            count += 1
        return count

    # ---- the rule --------------------------------------------------------------------------

    @property
    def input_waiting(self) -> bool:
        """An input line is queued and not started (`BackgroundWork.input_waiting`)."""
        return self.background.input_waiting

    @property
    def last_line_at(self) -> float | None:
        """The read time of the last line."""
        return self.background.last_line_at

    def holds(self, now: float, quiet: float) -> bool:
        """`BackgroundWork.holds` of the process."""
        return self.background.holds(now, quiet)

    def decide(
        self,
        now: float,
        *,
        stdin_open: bool,
        pending: int,
        followup_grace: float,
        stop_grace: float,
    ) -> Decision:
        """The stdin and stop rule now (09 18.5; the order of the runner's checks).

        Args:
            now: The monotonic time.
            stdin_open: The runner holds stdin open (phase ``open``).
            pending: Written inputs without echo.
            followup_grace: ``runner.followup_grace``.
            stop_grace: ``runner.stop_grace``.

        Returns:
            The decision; the caller acts on it (it never changes this object).
        """
        if self.failed_at is not None and stdin_open:
            # A failed turn absorbs nothing more: stdin closes at the error result.
            return Decision.CLOSE
        if self.failed_at is not None and now - self.failed_at >= stop_grace:
            return Decision.STOP
        if stdin_open and self.background.holds(now, followup_grace):
            return Decision.HOLD
        if self.background.input_waiting:
            return Decision.WAIT
        if stdin_open and stdin_close_due(now, self.last_result_at, pending, followup_grace):
            return Decision.CLOSE
        return Decision.NONE

    def lost_at_stop(self) -> int:
        """The running tasks at the stop after an error; kept as the lost count.

        Returns:
            The count of tasks the stop ends.
        """
        self.lost_tasks = len(self.background.running)
        return self.lost_tasks


def _success(result: Mapping[str, Any]) -> bool:
    return result.get("subtype") == "success" and result.get("is_error") is not True


async def replayed_lines(path: Path, start: int, end: int) -> list[tuple[dict[str, Any], int, int]]:
    """The decoded lines of a stream file whose line starts lie in ``[start, end)``.

    The lines are decoded exactly as the live read decodes them (`StreamReader`): a line that
    is not a JSON object or of a type the reader does not know is skipped.

    Args:
        path: The stream file.
        start: ``process_start`` of the ledger.
        end: The record's ``offset``.

    Returns:
        Each message with the offsets where its line starts and ends.
    """
    if end <= start:
        return []
    reader = StreamReader(path, 0.0, lambda: False, offset=start, until_exit=True)
    out: list[tuple[dict[str, Any], int, int]] = []
    async for message in reader.items():
        if message is None or reader.line_offset >= end:
            break
        out.append((message, reader.line_offset, reader.offset))
    return out
