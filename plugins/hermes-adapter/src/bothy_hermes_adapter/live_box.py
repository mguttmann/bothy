# SPDX-License-Identifier: Apache-2.0
"""The live box of a running turn: steps, status line and draft (09 4.7, 5.3, 17.3; D-L7).

`LiveBox` turns what Hermes reports during a turn into patches of the run document through the
SDK's throttled `bothy_sdk.runs.RunPatcher` (``agent.run_update_min_interval``, ``base_version``
kept and rebuilt after a conflict). The pinned version reports tool steps only through
``set_status_text`` (09 17.10 ``progress_hooks``): a status sentence when a tool starts and
``None`` when it ends. So a status sentence opens a step of kind ``tool_call`` whose title is the
sentence verbatim (never parsed, D-L7) and sets ``/status_line``; the step completes at ``None``,
at the next status sentence, at the next draft, at the answer or at the end of the turn, whichever
comes first. A later version that calls ``format_tool_event`` opens and closes steps the same way,
paired by Hermes's call ID or first in, first out, with ``detail`` holding only the preview Hermes
would have shown (docs/DECISIONS.md D-M2-33-7). Drafts replace ``/last_text_excerpt``.

After `drop` (a cancelled run) nothing is patched any more (09 8).

Examples:
    >>> LiveBox.STEP_KIND
    'tool_call'
"""

from __future__ import annotations

from collections import deque
from collections.abc import Callable
from datetime import datetime
from typing import ClassVar
from uuid import UUID

from bothy_sdk.runs import RunPatcher

__all__ = ["LiveBox"]


class LiveBox:
    """Live-box state of one run.

    Args:
        patcher: The run's patcher (the run has started).
        now: The wall time for ``finished_at``.
    """

    STEP_KIND: ClassVar[str] = "tool_call"
    STEP_NAME: ClassVar[str] = "tool"

    def __init__(self, patcher: RunPatcher, now: Callable[[], datetime]) -> None:
        """Create the state; nothing is patched yet."""
        self._patch = patcher
        self._now = now
        self._status_step: UUID | None = None
        self._by_call: dict[str, UUID] = {}
        self._fifo: deque[UUID] = deque()
        self.dropped = False
        self.steps_added = 0

    def drop(self) -> None:
        """Stop patching: the run was cancelled or has ended."""
        self.dropped = True

    def _close(self, step_id: UUID, status: str = "completed") -> None:
        self._patch.update_step(step_id, status=status, finished_at=self._now())

    def _close_status_step(self) -> None:
        if self._status_step is not None:
            self._close(self._status_step)
            self._status_step = None

    def status(self, text: str | None) -> None:
        """Hermes set or cleared the status sentence.

        Args:
            text: The sentence, or ``None`` when Hermes cleared it.
        """
        if self.dropped:
            return
        self._close_status_step()
        if text is None or not text.strip():
            self._patch.clear_status_line()
            return
        self._status_step = self._patch.add_step(
            self.STEP_KIND, self.STEP_NAME, title=text, status="running"
        )
        self.steps_added += 1
        self._patch.set_status_line(text)

    def tool_event(self, preview: str, call_id: str | None, done: bool) -> None:
        """A tool start or end from ``format_tool_event``.

        Args:
            preview: The text Hermes would have shown (the only content of ``detail``).
            call_id: Hermes's call ID, if the event has one.
            done: Whether the tool ended.
        """
        if self.dropped:
            return
        if not done:
            step = self._patch.add_step(
                self.STEP_KIND,
                self.STEP_NAME,
                title=preview,
                status="running",
                detail={"preview": preview},
            )
            self.steps_added += 1
            if call_id:
                self._by_call[call_id] = step
            else:
                self._fifo.append(step)
            return
        found = self._by_call.pop(call_id, None) if call_id else None
        if found is None and self._fifo:
            found = self._fifo.popleft()
        if found is not None:
            self._close(found)

    def draft(self, text: str) -> None:
        """Hermes streamed the answer so far.

        Args:
            text: The cumulative draft.
        """
        if self.dropped:
            return
        self._close_status_step()
        self._patch.set_last_text_excerpt(text)

    def answer(self) -> None:
        """The answer arrived: the running status step ends."""
        if not self.dropped:
            self._close_status_step()

    def end(self, failed: bool = False) -> None:
        """The turn ended: every open step ends.

        Args:
            failed: Whether the turn failed (open steps end ``failed``).
        """
        if self.dropped:
            return
        status = "failed" if failed else "completed"
        if self._status_step is not None:
            self._close(self._status_step, status)
            self._status_step = None
        for step in [*self._by_call.values(), *self._fifo]:
            self._close(step, status)
        self._by_call.clear()
        self._fifo.clear()
