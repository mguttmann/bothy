# SPDX-License-Identifier: Apache-2.0
"""Run graph data from the stream (M2-51; 09 5.6, 18.4; D-RV-3 to D-RV-5).

The Fake-Claude fixture ``graph`` is built from the field names of the pinned CLI
(``tests/fixtures/recording``): a subagent started by an ``Agent`` tool call with a tool inside,
a background command and a workflow started by tool calls, and a task without ``tool_use_id``.
"""

from __future__ import annotations

from typing import Any

import pytest

from .conftest import World

pytestmark = pytest.mark.anyio


async def _graph(world: World) -> dict[str, dict[str, Any]]:
    """Run one turn of the fixture ``graph`` and return the steps of the run by id."""
    world.scenario(fixture="graph")
    await world.start()
    run = await world.fake.trigger("Review and check", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    steps: dict[str, dict[str, Any]] = world.fake.run(run.run_id)["steps"]
    return steps


def _one(steps: dict[str, dict[str, Any]], **match: Any) -> dict[str, Any]:
    found = [s for s in steps.values() if all(s.get(k) == v for k, v in match.items())]
    assert len(found) == 1, (match, list(steps.values()))
    return found[0]


async def test_task_parent_from_tool_use_id(world: World) -> None:
    steps = await _graph(world)
    agent_call = _one(steps, kind="tool_call", name="Agent")
    subagent = _one(steps, kind="subagent")
    read = _one(steps, kind="tool_call", name="Read")
    bash_call = _one(steps, kind="tool_call", name="Bash")
    bash_job = _one(steps, kind="background_job", title="npm test")
    assert "parent_step_id" not in agent_call
    assert subagent["parent_step_id"] == agent_call["id"]
    # A tool inside the subagent keeps its parent from parent_tool_use_id (the Agent call).
    assert read["parent_step_id"] == agent_call["id"]
    assert bash_job["parent_step_id"] == bash_call["id"]
    assert subagent["status"] == "completed"


async def test_task_without_tool_use_id_at_root(world: World) -> None:
    steps = await _graph(world)
    loose = _one(steps, kind="background_job", title="Watch the logs")
    assert "parent_step_id" not in loose
    assert loose["status"] == "cancelled"


async def test_workflow_task_kind(world: World) -> None:
    steps = await _graph(world)
    flow = _one(steps, kind="workflow")
    assert flow["name"] == "local_workflow"
    assert flow["parent_step_id"] == _one(steps, kind="tool_call", name="Workflow")["id"]
    assert flow["status"] == "completed"


async def test_task_usage_in_detail(world: World) -> None:
    steps = await _graph(world)
    subagent = _one(steps, kind="subagent")
    assert subagent["detail"] == {
        "type": "local_agent",
        "usage": {"tokens": 1200, "tool_uses": 1, "duration_ms": 2400},
    }
    flow = _one(steps, kind="workflow")
    assert flow["detail"] == {"usage": {"tokens": 3000, "tool_uses": 4, "duration_ms": 9000}}
    # Without a reported usage the detail stays as it was; no step ever carries a cost.
    assert "usage" not in _one(steps, kind="background_job", title="npm test").get("detail", {})
    for step in steps.values():
        assert "cost_micros" not in step.get("detail", {}).get("usage", {})
