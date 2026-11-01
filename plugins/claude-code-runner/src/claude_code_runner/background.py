# SPDX-License-Identifier: Apache-2.0
"""The background work of one turn process and when it holds stdin open (09 18.5).

A turn may start work that outlives it: Bash with ``run_in_background``, a background subagent
or a workflow. The pinned CLI reports the running set in every ``system/background_tasks_changed``
(an empty list once the last one ended) and keeps that work only while its stdin is open: when
its input ends it stops the work and exits. After the last task ended it starts a turn by itself
(``system/init``, then a ``result`` that names no input) for the task's notification, and a new
input line written between turns starts a turn of its own at once.

`BackgroundWork` follows those messages; `BackgroundWork.holds` tells the runner to keep stdin
open: while a task runs, while a turn of a process that had background work runs, and for a
quiet window after its last line, so the notification turn that follows the end of the last task
is not lost.

Examples:
    >>> work = BackgroundWork()
    >>> work.apply({"type": "system", "subtype": "background_tasks_changed",
    ...             "tasks": [{"task_id": "b1"}]}, 0.0)
    >>> work.running, work.holds(3600.0, 30.0)
    (['b1'], True)
"""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

__all__ = ["BackgroundWork"]


class BackgroundWork:
    """The running background tasks of one turn process, as the stream reports them.

    Attributes:
        running: The IDs of the running tasks in the order of the last report.
        seen: The process had background work at some time.
        turn_open: A turn of the process has started (``system/init``) and has no ``result`` yet.
        last_line_at: The monotonic time of the last stream message.
    """

    def __init__(self) -> None:
        """Start without background work."""
        self.running: list[str] = []
        self.seen = False
        self.turn_open = False
        self.last_line_at: float | None = None

    def apply(self, message: Mapping[str, Any], now: float) -> None:
        """Take one stream message.

        Args:
            message: A decoded message of the stream reader.
            now: The monotonic time it was read.
        """
        self.last_line_at = now
        kind, subtype = message.get("type"), message.get("subtype")
        if kind == "system" and subtype == "init":
            self.turn_open = True
        elif kind == "result":
            self.turn_open = False
        elif kind == "system" and subtype == "background_tasks_changed":
            tasks = message.get("tasks")
            if not isinstance(tasks, list):
                return
            # Ambient tasks and tasks without a transcript (the CLI's own) are no work of
            # the session.
            self.running = [
                str(t["task_id"])
                for t in tasks
                if isinstance(t, Mapping)
                and isinstance(t.get("task_id"), str)
                and t.get("ambient") is not True
                and t.get("skip_transcript") is not True
            ]
            self.seen = self.seen or bool(self.running)

    def holds(self, now: float, quiet: float) -> bool:
        """Whether stdin of the process must stay open now.

        Args:
            now: The monotonic time.
            quiet: The quiet window after the last line (``runner.followup_grace``).

        Returns:
            True while a task runs; for a process that had background work also while a turn
            runs and until ``quiet`` seconds passed without a stream line. False for a process
            that never had background work (the stdin rule of 18.5 applies unchanged).
        """
        if self.running:
            return True
        if not self.seen:
            return False
        if self.turn_open:
            return True
        return self.last_line_at is not None and now - self.last_line_at < quiet
