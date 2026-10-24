# SPDX-License-Identifier: Apache-2.0
"""What follows the end of a turn process, and the box notes of a run (09 18.9, notes 8 to 11).

`next_action` is a pure function from a classified process end (`TurnEnd`), the recovery state
of the run and the run's session mode to the next step: finish, fail with a reason, report a
stop, compact, retry the same prompt, start a new session or resume the same reference. Each
recovery kind happens at most once per run, and no end other than a full context, a missing
session or an existing session starts a process (09 1 point 5, R-AG-17).

A box note is a run step of kind ``status`` whose ``name`` is a code clients translate
(``context_compacted``, ``session_missing``, ``runtime_error``); `box_note` builds its value.

Examples:
    >>> from claude_code_runner.failures import TurnEnd
    >>> next_action(TurnEnd("context_full"), RecoveryState(), "new", worked=True)
    Compact()
"""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass, replace
from datetime import datetime
from typing import Any

from bothy_sdk.ids import uuid7

from claude_code_runner.failures import TurnEnd

__all__ = [
    "BOX_NOTE_CODES",
    "RETRY_HINT",
    "Action",
    "Cancelled",
    "Compact",
    "Fail",
    "Finish",
    "NewSession",
    "RecoveryState",
    "ResumeSameReference",
    "Retry",
    "box_note",
    "next_action",
]

RETRY_HINT = (
    "The previous attempt at this request stopped because the conversation reached the context "
    "limit and was compacted. It may already have changed files or run commands. Check the "
    "current state before you repeat any action."
)
"""Appended to the runner prompt of a retry after compaction when the failed turn had worked."""

BOX_NOTE_CODES = frozenset({"context_compacted", "session_missing", "runtime_error"})
"""The ``name`` codes of the runner's box notes (09 18.9)."""


@dataclass(frozen=True)
class RecoveryState:
    """What a run has used of its recoveries (each at most once).

    Attributes:
        compacted: A compaction ran (successful or not).
        session_replaced: A missing session was replaced by a new one.
        session_resumed: An existing session was resumed after ``--session-id`` failed.
        compact_supported: The pinned CLI takes ``/compact`` in print mode (design note 8 a).
    """

    compacted: bool = False
    session_replaced: bool = False
    session_resumed: bool = False
    compact_supported: bool = True

    def with_(self, **changes: bool) -> RecoveryState:
        """Return a copy with ``changes``.

        Args:
            **changes: Fields to set.

        Returns:
            The new state.
        """
        return replace(self, **changes)


@dataclass(frozen=True)
class Finish:
    """The run completed: answer and finish ``completed``."""


@dataclass(frozen=True)
class Fail:
    """The run ends ``failed``.

    Attributes:
        reason: ``runtime_error``, ``runtime_not_authenticated`` or ``billing_mode_mismatch``.
        errors: For ``runtime_error``: the error texts for the box note.
    """

    reason: str
    errors: tuple[str, ...] = ()


@dataclass(frozen=True)
class Cancelled:
    """The run ends ``cancelled`` with the reason of the stop request.

    Attributes:
        reason: The request's reason, unchanged.
    """

    reason: str


@dataclass(frozen=True)
class Compact:
    """Start the compaction process (``--resume <ref>``, the single line ``/compact``)."""


@dataclass(frozen=True)
class Retry:
    """Start the same first line again with ``--resume <ref>`` after a compaction.

    Attributes:
        hint: Append `RETRY_HINT` to the runner prompt (the failed turn had worked).
    """

    hint: bool


@dataclass(frozen=True)
class NewSession:
    """Replace a missing session by a new reference (``--session-id <new>``), fail open.

    Attributes:
        bind_failed: Report ``bind_failed: session_missing`` (mode ``bind`` only).
    """

    bind_failed: bool


@dataclass(frozen=True)
class ResumeSameReference:
    """Continue the existing session with ``--resume <ref>`` (mode ``new``)."""


Action = Finish | Fail | Cancelled | Compact | Retry | NewSession | ResumeSameReference
"""Every step `next_action` can return."""


def next_action(  # noqa: PLR0911 - one answer per row of the table in the module docstring
    end: TurnEnd,
    state: RecoveryState,
    session_mode: str,
    *,
    worked: bool,
    process: str = "turn",
    stop_reason: str | None = None,
    precheck: str | None = None,
) -> Action:
    """Decide what follows a process end (design notes 8 to 10).

    Args:
        end: The classified end of the process.
        state: The recoveries the run has used before this process ended.
        session_mode: ``new``, ``resume`` or ``bind`` of the invocation payload.
        worked: The turn that failed with a full context emitted an ``assistant`` message after
            ``system/init`` (for a compaction process: the turn before it).
        process: ``turn`` or ``compaction`` (the process that ended).
        stop_reason: The reason of the stop request, if one came.
        precheck: The outcome of the pre-check before a restart when it was not ``start``
            (``runtime_not_authenticated`` or ``billing_mode_mismatch``).

    Returns:
        The next step.
    """
    if end.outcome == "stopped" or stop_reason is not None:
        return Cancelled(stop_reason or "user_cancelled")
    if precheck is not None:
        return Fail(precheck)
    if process == "compaction":
        if end.outcome == "completed":
            return Retry(hint=worked)
        return Fail("runtime_error", end.errors)
    if end.outcome == "completed":
        return Finish()
    if end.outcome == "not_authenticated":
        return Fail("runtime_not_authenticated")
    if end.outcome == "context_full":
        if state.compacted or not state.compact_supported:
            return Fail("runtime_error", end.errors)
        return Compact()
    if end.outcome == "session_missing" and session_mode in ("resume", "bind"):
        if state.session_replaced:
            return Fail("runtime_error", end.errors)
        return NewSession(bind_failed=session_mode == "bind")
    if end.outcome == "session_exists" and session_mode == "new":
        if state.session_resumed:
            return Fail("runtime_error", end.errors)
        return ResumeSameReference()
    return Fail("runtime_error", end.errors)


def box_note(
    code: str, status: str, detail: Mapping[str, Any] | None, now: datetime
) -> dict[str, Any]:
    """Build the value of one box note step (design note 11).

    Args:
        code: One of `BOX_NOTE_CODES`.
        status: ``completed`` or ``failed``.
        detail: The step detail (``{"mode": ...}`` or ``{"errors": [...], "exit": ...}``).
        now: The wall time of ``finished_at``.

    Returns:
        The step value for ``add /steps/<id>`` with ``id`` a fresh UUIDv7, kind ``status``, the
        code as ``name``, an empty ``title`` and ``finished_at`` set.

    Raises:
        ValueError: An unknown code or status.
    """
    if code not in BOX_NOTE_CODES or status not in ("completed", "failed"):
        raise ValueError("unknown box note")
    value: dict[str, Any] = {
        "id": str(uuid7()),
        "kind": "status",
        "name": code,
        "title": "",
        "status": status,
        "finished_at": now.isoformat().replace("+00:00", "Z"),
    }
    if detail is not None:
        value["detail"] = dict(detail)
    return value
