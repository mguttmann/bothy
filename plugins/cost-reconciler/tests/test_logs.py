# SPDX-License-Identifier: Apache-2.0
"""JSON lines on stderr with a fixed set of events and fields."""

from __future__ import annotations

import io
import json
from datetime import UTC, datetime

import pytest

from cost_reconciler.logs import EVENTS, EventLog


def test_line_shape() -> None:
    stream = io.StringIO()
    log = EventLog(stream, now=lambda: datetime(2026, 10, 21, 5, 30, 1, 123456, tzinfo=UTC))
    log.emit("submitted", bucket_start="2026-10-20T00:00:00.000Z", entry_count=3, reconciled=3)
    log.emit("pass_failed", level="error", stage="submit", status=500)
    first, second = (json.loads(line) for line in stream.getvalue().splitlines())
    assert first == {
        "ts": "2026-10-21T05:30:01.123Z",
        "level": "info",
        "event": "submitted",
        "bucket_start": "2026-10-20T00:00:00.000Z",
        "entry_count": 3,
        "reconciled": 3,
    }
    assert second["level"] == "error"


def test_events_are_the_closed_list() -> None:
    assert set(EVENTS) == {
        "pass_started",
        "pass_finished",
        "pass_failed",
        "not_assigned",
        "currency_mismatch",
        "submitted",
        "unmatched_runs",
        "config_error",
    }


def test_unknown_event_or_field_is_a_programming_error() -> None:
    log = EventLog(io.StringIO())
    with pytest.raises(ValueError, match="event"):
        log.emit("other")
    with pytest.raises(ValueError, match="field"):
        log.emit("submitted", body="x")
    with pytest.raises(ValueError, match="level"):
        log.emit("submitted", level="debug")


def test_default_clock_is_utc() -> None:
    stream = io.StringIO()
    EventLog(stream).emit("pass_started")
    assert json.loads(stream.getvalue())["ts"].endswith("Z")
