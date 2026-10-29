# SPDX-License-Identifier: Apache-2.0
"""Run graph mapping of the live box against the recording of the pinned CLI (M2-51)."""

from __future__ import annotations

import json
from datetime import UTC, datetime, timedelta
from pathlib import Path

from claude_code_runner.livebox import WORKFLOW_TASK_TYPE, LiveBox

from .test_livebox import OWN, Patch

RECORDING = Path(__file__).parent.parent / "fixtures" / "recording" / "stream_fields_2.1.288.json"
T0 = datetime(2026, 10, 21, 9, 0, tzinfo=UTC)
INTERVAL = 120.0


class Clock:
    """An injected wall clock that only moves when the test moves it."""

    def __init__(self) -> None:
        self.at = T0

    def __call__(self) -> datetime:
        return self.at


def _box(clock: Clock) -> tuple[LiveBox, Patch]:
    patch = Patch()
    live = LiveBox(
        patch,  # type: ignore[arg-type]
        own_uuid=OWN,
        title_requested=False,
        context_windows={},
        now=clock,
        progress_interval=INTERVAL,
    )
    return live, patch


def test_mapping_uses_only_recorded_fields() -> None:
    recording = json.loads(RECORDING.read_text())
    assert recording["cli_version"] == "2.1.288"
    assert recording["workflow_task_type"] == WORKFLOW_TASK_TYPE
    assert WORKFLOW_TASK_TYPE in recording["task_types"]
    for subtype in ("system/task_started", "system/task_notification"):
        assert "tool_use_id" in recording["messages"][subtype]
    assert "usage" in recording["messages"]["system/task_notification"]
    assert recording["usage"] == ["total_tokens", "tool_uses", "duration_ms"]


def test_tool_progress_patch_interval() -> None:
    clock = Clock()
    live, patch = _box(clock)
    for _ in range(50):
        clock.at += timedelta(seconds=1)
        live.apply({"type": "tool_progress", "tool_use_id": "a"})
    # Within the interval since the start of the turn: counted, never sent.
    assert "/metrics/progress_events" not in patch.values
    clock.at = T0 + timedelta(seconds=INTERVAL)
    live.apply({"type": "tool_progress", "tool_use_id": "a"})
    assert patch.values["/metrics/progress_events"] == 51
    # The patch was a change: the next ones within the interval send nothing.
    for _ in range(10):
        clock.at += timedelta(seconds=10)
        live.apply({"type": "tool_progress", "tool_use_id": "a"})
    assert patch.values["/metrics/progress_events"] == 51
    clock.at += timedelta(seconds=INTERVAL - 100)
    live.apply({"type": "tool_progress", "tool_use_id": "a"})
    assert patch.values["/metrics/progress_events"] == 62


def test_visible_change_restarts_the_interval() -> None:
    clock = Clock()
    live, patch = _box(clock)
    clock.at += timedelta(seconds=INTERVAL - 1)
    live.apply({"type": "system", "subtype": "status", "status": "compacting"})
    clock.at += timedelta(seconds=2)
    live.apply({"type": "tool_progress", "tool_use_id": "a"})
    assert "/metrics/progress_events" not in patch.values


def test_progress_count_survives_adoption() -> None:
    clock = Clock()
    live, _ = _box(clock)
    live.apply({"type": "tool_progress", "tool_use_id": "a"})
    state = live.state()
    again, _ = _box(clock)
    again.restore(state, {})
    assert again.progress_events == 1


def test_workflow_step_restored() -> None:
    clock = Clock()
    live, patch = _box(clock)
    live.apply(
        {
            "type": "system",
            "subtype": "task_started",
            "task_id": "w",
            "task_type": WORKFLOW_TASK_TYPE,
            "description": "Release check",
        }
    )
    (step_id,) = patch.steps
    assert patch.steps[step_id]["kind"] == "workflow"
    assert live.state()["tasks"]["w"] == [str(step_id), "workflow"]
    # An adopting runner takes the workflow step over and ends the step it already has.
    again, again_patch = _box(clock)
    again.restore(live.state(), {str(step_id): "running"})
    again_patch.steps[step_id] = {}
    again.apply(
        {"type": "system", "subtype": "task_notification", "task_id": "w", "status": "failed"}
    )
    assert again_patch.steps[step_id]["status"] == "failed"
    assert len(again_patch.steps) == 1
