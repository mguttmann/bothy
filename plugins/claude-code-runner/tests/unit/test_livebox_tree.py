# SPDX-License-Identifier: Apache-2.0
"""The agent tree of a turn from a recorded stream (FEAT-topology-agent-tree).

``agent_tree_<pinned version>.jsonl`` is one turn of the pinned CLI against the local stand-in
for the API (``tests/real_cli/record.py``; no login, no model call): a workflow with three agents,
a background and a foreground shell task, the foreground commands of the workflow agents, and an
Agent tool subagent with its own tool calls. Every answer is written in the recording program;
the lines are as the CLI wrote them, with ``system/init`` reduced to what the runner reads and
the paths of the recording folder replaced.
"""

from __future__ import annotations

import json
from datetime import UTC, datetime
from pathlib import Path
from typing import Any
from uuid import UUID

from claude_code_runner.heartbeat import PINNED_CLAUDE_CODE_VERSION
from claude_code_runner.livebox import WORKFLOW_TASK_TYPE, LiveBox

from .test_livebox import OWN, Patch

RECORDING = (
    Path(__file__).parent.parent
    / "fixtures"
    / "recording"
    / f"agent_tree_{PINNED_CLAUDE_CODE_VERSION}.jsonl"
)
NOW = datetime(2026, 11, 1, 9, 0, tzinfo=UTC)


def _box() -> tuple[LiveBox, Patch]:
    patch = Patch()
    live = LiveBox(
        patch,  # type: ignore[arg-type]
        own_uuid=OWN,
        title_requested=False,
        context_windows={},
        now=lambda: NOW,
    )
    return live, patch


def _replay(live: LiveBox, messages: list[dict[str, Any]]) -> None:
    for message in messages:
        live.apply(message)


def _recording() -> list[dict[str, Any]]:
    return [json.loads(line) for line in RECORDING.read_text().splitlines() if line.strip()]


def _of(patch: Patch, **match: Any) -> list[tuple[UUID, dict[str, Any]]]:
    return [(k, s) for k, s in patch.steps.items() if all(s.get(m) == v for m, v in match.items())]


def _progress(task: str, *agents: dict[str, Any]) -> dict[str, Any]:
    return {
        "type": "system",
        "subtype": "task_progress",
        "task_id": task,
        "tool_use_id": "toolu_w",
        "workflow_progress": [
            {"type": "workflow_phase", "index": 1, "title": "Build"},
            *agents,
        ],
    }


def _agent(index: int, label: str, state: str, **more: Any) -> dict[str, Any]:
    return {
        "type": "workflow_agent",
        "index": index,
        "label": label,
        "phaseIndex": 1,
        "phaseTitle": "Build",
        "agentId": f"agent{index}",
        "state": state,
        "promptPreview": "secret prompt text",
        "resultPreview": "secret result text",
        **more,
    }


def _workflow_started(live: LiveBox) -> None:
    _replay(
        live,
        [
            {
                "type": "assistant",
                "message": {
                    "content": [
                        {"type": "tool_use", "id": "toolu_w", "name": "Workflow", "input": {}}
                    ]
                },
            },
            {
                "type": "system",
                "subtype": "task_started",
                "task_id": "w1",
                "tool_use_id": "toolu_w",
                "task_type": WORKFLOW_TASK_TYPE,
                "description": "Build and review",
            },
        ],
    )


def test_recording_workflow_agents_are_children_of_the_workflow() -> None:
    live, patch = _box()
    _replay(live, _recording())
    ((flow_id, flow),) = _of(patch, kind="workflow")
    agents = _of(patch, name="workflow_agent")
    assert [s["title"] for _, s in agents] == ["writer", "checker", "review"]
    for _, step in agents:
        assert step["kind"] == "subagent"
        assert step["parent_step_id"] == flow_id
        assert step["status"] == "completed"
        assert step["detail"]["type"] == "workflow_agent"
        assert step["detail"]["usage"]["tokens"] > 0
    # The workflow ended completed; its agents ended with it.
    assert flow["status"] == "completed"


def test_recording_has_no_root_steps_for_foreground_commands_of_workflow_agents() -> None:
    live, patch = _box()
    _replay(live, _recording())
    recording = _recording()
    owned = {
        m["task_id"]
        for m in recording
        if m.get("subtype") == "task_started"
        and m.get("owned_by_subagent") is True
        and m.get("is_backgrounded") is not True
    }
    assert owned, "the recording carries foreground commands of subagents"
    root_jobs = [s for _, s in _of(patch, kind="background_job") if s.get("parent_step_id") is None]
    owned_titles = {
        m["description"]
        for m in recording
        if m.get("subtype") == "task_started" and m.get("task_id") in owned
    }
    assert not [s for s in root_jobs if s["title"] in owned_titles]


def test_recording_subagent_and_background_jobs_keep_their_steps() -> None:
    live, patch = _box()
    _replay(live, _recording())
    subagents = [
        m
        for m in _recording()
        if m.get("subtype") == "task_started" and m.get("task_type") == "local_agent"
    ]
    assert subagents
    assert len(_of(patch, kind="subagent", name="local_agent")) == len(subagents)
    backgrounded = [
        m
        for m in _recording()
        if m.get("subtype") == "task_started"
        and m.get("task_type") == "local_bash"
        and m.get("owned_by_subagent") is not True
    ]
    titles = {s["title"] for _, s in _of(patch, kind="background_job")}
    assert {m["description"] for m in backgrounded} <= titles


def test_no_content_preview_leaves_the_runner() -> None:
    live, patch = _box()
    _workflow_started(live)
    live.apply(_progress("w1", _agent(1, "review", "progress", lastToolName="Bash")))
    sent = json.dumps(list(patch.steps.values()), default=str)
    assert "secret" not in sent


def test_agent_state_tool_and_usage_follow_the_progress() -> None:
    live, patch = _box()
    _workflow_started(live)
    live.apply(_progress("w1", _agent(1, "build", "start")))
    ((step_id, step),) = _of(patch, name="workflow_agent")
    assert step["status"] == "running"
    assert step["title"] == "build"
    assert step["detail"] == {"type": "workflow_agent", "phase": "Build"}
    live.apply(
        _progress(
            "w1",
            _agent(1, "build", "progress", lastToolName="Bash", tokens=10, toolCalls=2),
        )
    )
    assert patch.steps[step_id]["detail"] == {
        "type": "workflow_agent",
        "phase": "Build",
        "last_tool": "Bash",
        "usage": {"tokens": 10, "tool_uses": 2},
    }
    live.apply(
        _progress(
            "w1",
            _agent(1, "build", "done", lastToolName="Bash", tokens=10, toolCalls=2),
            _agent(2, "review", "failed"),
        )
    )
    assert patch.steps[step_id]["status"] == "completed"
    assert patch.steps[step_id]["finished_at"] == NOW
    ((_, review),) = _of(patch, title="review")
    assert review["status"] == "failed"
    assert len(_of(patch, name="workflow_agent")) == 2


def test_agent_listed_before_its_start_keeps_its_step() -> None:
    """Agents that start side by side are listed first without ``agentId``, then with one."""
    live, patch = _box()
    _workflow_started(live)
    waiting = {k: v for k, v in _agent(2, "review", "start").items() if k != "agentId"}
    live.apply(_progress("w1", _agent(1, "build", "start"), waiting))
    ((_, review),) = _of(patch, title="review")
    assert review["status"] == "running"
    live.apply(_progress("w1", _agent(1, "build", "progress"), _agent(2, "review", "start")))
    live.apply(
        _progress(
            "w1",
            _agent(1, "build", "done"),
            _agent(2, "review", "done", lastToolName="Bash", tokens=7, toolCalls=1),
        )
    )
    agents = _of(patch, name="workflow_agent")
    assert [(s["title"], s["status"]) for _, s in agents] == [
        ("build", "completed"),
        ("review", "completed"),
    ]
    assert agents[1][1]["detail"]["usage"] == {"tokens": 7, "tool_uses": 1}


def test_agent_listed_before_its_start_survives_adoption() -> None:
    live, patch = _box()
    _workflow_started(live)
    waiting = {k: v for k, v in _agent(1, "build", "start").items() if k != "agentId"}
    live.apply(_progress("w1", waiting))
    ((step_id, _),) = _of(patch, name="workflow_agent")
    state = live.state()
    known = {str(k): "running" for k in patch.steps}
    again, again_patch = _box()
    again.restore(state, known)
    for k in patch.steps:
        again_patch.steps[k] = dict(patch.steps[k])
    again.apply(_progress("w1", _agent(1, "build", "done")))
    assert [k for k, _ in _of(again_patch, name="workflow_agent")] == [step_id]
    assert again_patch.steps[step_id]["status"] == "completed"


def test_unchanged_progress_sends_nothing() -> None:
    live, patch = _box()
    _workflow_started(live)
    live.apply(_progress("w1", _agent(1, "build", "progress", lastToolName="Bash")))
    live.steps_changed = False
    calls: list[Any] = []
    original = patch.update_step

    def counting(step_id: UUID, **kw: Any) -> None:
        calls.append(kw)
        original(step_id, **kw)

    patch.update_step = counting  # type: ignore[method-assign]
    live.apply(_progress("w1", _agent(1, "build", "progress", lastToolName="Bash")))
    assert calls == []


def test_open_agents_end_with_their_workflow() -> None:
    live, patch = _box()
    _workflow_started(live)
    live.apply(_progress("w1", _agent(1, "build", "progress"), _agent(2, "review", "start")))
    live.apply(
        {"type": "system", "subtype": "task_notification", "task_id": "w1", "status": "stopped"}
    )
    agents = _of(patch, name="workflow_agent")
    assert {s["status"] for _, s in agents} == {"cancelled"}


def test_progress_of_an_unknown_or_non_workflow_task_adds_no_agent() -> None:
    live, patch = _box()
    live.apply(_progress("nobody", _agent(1, "build", "progress")))
    assert not _of(patch, name="workflow_agent")


def test_backgrounded_command_of_a_subagent_keeps_its_step() -> None:
    live, patch = _box()
    live.apply(
        {
            "type": "system",
            "subtype": "task_started",
            "task_id": "b1",
            "tool_use_id": "toolu_unknown",
            "owned_by_subagent": True,
            "is_backgrounded": True,
            "task_type": "local_bash",
            "description": "long test run",
        }
    )
    assert len(_of(patch, kind="background_job")) == 1


def test_agent_steps_survive_adoption() -> None:
    live, patch = _box()
    _workflow_started(live)
    live.apply(_progress("w1", _agent(1, "build", "progress")))
    ((step_id, _),) = _of(patch, name="workflow_agent")
    state = live.state()
    known = {str(k): "running" for k in patch.steps}
    again, again_patch = _box()
    again.restore(state, known)
    for k in patch.steps:
        again_patch.steps[k] = dict(patch.steps[k])
    again.apply(_progress("w1", _agent(1, "build", "done")))
    assert len(_of(again_patch, name="workflow_agent")) == 1
    assert again_patch.steps[step_id]["status"] == "completed"
