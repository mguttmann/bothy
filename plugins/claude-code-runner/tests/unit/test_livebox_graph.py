# SPDX-License-Identifier: Apache-2.0
"""Run graph mapping of the live box against the recording of the pinned CLI (M2-51)."""

from __future__ import annotations

import json
from datetime import UTC, datetime, timedelta
from pathlib import Path

from claude_code_runner.heartbeat import PINNED_CLAUDE_CODE_VERSION
from claude_code_runner.livebox import WORKFLOW_TASK_TYPE, LiveBox

from .test_livebox import OWN, Patch

RECORDING = (
    Path(__file__).parent.parent
    / "fixtures"
    / "recording"
    / f"stream_fields_{PINNED_CLAUDE_CODE_VERSION}.json"
)
# What the runner reads of each stream message (09 18.4, 18.5, 18.7, 18.9).
READ_MEMBERS = {
    "system/init": ["model", "apiKeySource"],
    "assistant": ["message", "parent_tool_use_id", "is_api_error_message"],
    "user": ["message", "isReplay"],
    "system/task_started": [
        "task_id",
        "tool_use_id",
        "description",
        "task_type",
        "ambient",
        "is_backgrounded",
        "owned_by_subagent",
    ],
    "system/task_progress": ["task_id", "tool_use_id", "usage", "workflow_progress"],
    "system/task_updated": ["task_id"],
    "system/task_notification": ["task_id", "tool_use_id", "status", "usage", "ambient"],
    "system/background_tasks_changed": ["tasks"],
    "system/status": ["status"],
    "system/api_retry": ["attempt", "max_retries", "retry_delay_ms", "error"],
    "command_lifecycle": ["command_uuid", "state"],
    "result/success": [
        "is_error",
        "result",
        "modelUsage",
        "total_cost_usd",
        "user_message_uuid",
        "user_message_uuids",
    ],
}
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
    assert recording["cli_version"] == PINNED_CLAUDE_CODE_VERSION
    assert recording["workflow_task_type"] == WORKFLOW_TASK_TYPE
    assert WORKFLOW_TASK_TYPE in recording["task_types"]
    assert "local_agent" in recording["task_types"]
    for message, members in READ_MEMBERS.items():
        assert set(members) <= set(recording["messages"][message]), message
    assert recording["usage"] == ["total_tokens", "tool_uses", "duration_ms"]
    assert {"task_id", "description", "ambient"} <= set(recording["background_task"])
    assert recording["task_status"] == ["completed", "failed", "stopped"]
    assert {"queued", "started", "completed", "cancelled"} <= set(recording["command_states"])
    assert {
        "agentId",
        "index",
        "label",
        "phaseTitle",
        "state",
        "lastToolName",
        "tokens",
        "toolCalls",
        "durationMs",
    } <= set(recording["workflow_agent"])


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
