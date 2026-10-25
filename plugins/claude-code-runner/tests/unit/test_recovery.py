# SPDX-License-Identifier: Apache-2.0
"""What follows a process end, and the shape of a box note (09 18.9, notes 8 to 11)."""

from __future__ import annotations

from datetime import UTC, datetime
from uuid import UUID

import pytest

from claude_code_runner.failures import TurnEnd
from claude_code_runner.recovery import (
    Action,
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

FRESH = RecoveryState()
ERR = TurnEnd("runtime_error", ("boom",), "1")

CASES: list[tuple[str, TurnEnd, RecoveryState, str, dict[str, object], Action]] = [
    ("first context full", TurnEnd("context_full"), FRESH, "new", {}, Compact()),
    (
        "compaction ok, worked",
        TurnEnd("completed"),
        RecoveryState(compacted=True),
        "new",
        {"process": "compaction", "worked": True},
        Retry(hint=True),
    ),
    (
        "compaction ok, no work",
        TurnEnd("completed"),
        RecoveryState(compacted=True),
        "new",
        {"process": "compaction", "worked": False},
        Retry(hint=False),
    ),
    (
        "compaction failed",
        ERR,
        RecoveryState(compacted=True),
        "new",
        {"process": "compaction"},
        Fail("runtime_error", ("boom",)),
    ),
    (
        "second context full",
        TurnEnd("context_full"),
        RecoveryState(compacted=True),
        "new",
        {},
        Fail("runtime_error"),
    ),
    (
        "no /compact in print mode",
        TurnEnd("context_full"),
        RecoveryState(compact_supported=False),
        "new",
        {},
        Fail("runtime_error"),
    ),
    ("missing, resume", TurnEnd("session_missing"), FRESH, "resume", {}, NewSession(False)),
    ("missing, bind", TurnEnd("session_missing"), FRESH, "bind", {}, NewSession(True)),
    (
        "missing twice",
        TurnEnd("session_missing"),
        RecoveryState(session_replaced=True),
        "resume",
        {},
        Fail("runtime_error"),
    ),
    ("exists, new", TurnEnd("session_exists"), FRESH, "new", {}, ResumeSameReference()),
    (
        "exists twice",
        TurnEnd("session_exists"),
        RecoveryState(session_resumed=True),
        "new",
        {},
        Fail("runtime_error"),
    ),
    (
        "not authenticated",
        TurnEnd("not_authenticated"),
        FRESH,
        "new",
        {},
        Fail("runtime_not_authenticated"),
    ),
    ("runtime error", ERR, FRESH, "new", {}, Fail("runtime_error", ("boom",))),
    ("completed", TurnEnd("completed"), FRESH, "new", {}, Finish()),
    (
        "stopped",
        TurnEnd("stopped"),
        FRESH,
        "new",
        {"stop_reason": "access_revoked"},
        Cancelled("access_revoked"),
    ),
    (
        "pre-check mismatch before a restart",
        TurnEnd("session_missing"),
        FRESH,
        "resume",
        {"precheck": "billing_mode_mismatch"},
        Fail("billing_mode_mismatch"),
    ),
]


@pytest.mark.parametrize(
    ("name", "end", "state", "mode", "extra", "expected"), CASES, ids=[c[0] for c in CASES]
)
def test_next_action(
    name: str,
    end: TurnEnd,
    state: RecoveryState,
    mode: str,
    extra: dict[str, object],
    expected: Action,
) -> None:
    kwargs: dict[str, object] = {"worked": False, **extra}
    assert next_action(end, state, mode, **kwargs) == expected, name  # type: ignore[arg-type]


def test_box_note_shape() -> None:
    now = datetime(2026, 10, 24, 12, 0, tzinfo=UTC)
    note = box_note("session_missing", "completed", {"mode": "bind"}, now)
    assert UUID(note["id"]).version == 7
    assert (note["kind"], note["name"], note["title"], note["status"]) == (
        "status",
        "session_missing",
        "",
        "completed",
    )
    assert note["finished_at"] == "2026-10-24T12:00:00Z"
    assert note["detail"] == {"mode": "bind"}
    with pytest.raises(ValueError, match="unknown box note"):
        box_note("anything", "completed", None, now)
