# SPDX-License-Identifier: Apache-2.0
"""Background work of the pinned Hermes through the adapter (09 17.12, R-AG-44).

The pinned version runs every top-level ``delegate_task`` in the background and starts a turn of
its own with the result, past the platform's ``handle_message``. These scenarios hold that the
run covers that work: it stays running with a ``subagent`` step, the result turn runs inside it,
stop interrupts the delegation, and a turn without a run can call no tool.
"""

from __future__ import annotations

import asyncio
from typing import Any

import pytest
from fake_hermes_kit import BoundTurn, HermesRig, HermesWorld, Profile, gateway_log, hermes_rig
from hermes_harness.fake_model import ScriptTurn

from bothy_hermes_adapter.handler import TOOL_REFUSED

pytestmark = pytest.mark.anyio

GOAL = "Count the boats on the lake."
DELEGATE = {"name": "delegate_task", "arguments": {"tasks": [{"goal": GOAL, "context": "none"}]}}
TODO = {
    "name": "todo_list",
    "arguments": {"todos": [{"content": "Report", "id": "1", "status": "pending"}]},
}
STARTED = "Started a team."
RESULT = "The team counted seven boats."
PROFILE = Profile(toolsets=("todo", "delegation", "terminal"))


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def _steps(rig: HermesRig, run_id: Any, kind: str) -> list[dict[str, Any]]:
    return [s for s in rig.fake.run(run_id)["steps"].values() if s["kind"] == kind]


def _hermes_log(rig: HermesRig) -> str:
    logs = rig.home / "logs"
    if not logs.exists():
        return ""
    return "\n".join(
        p.read_text(encoding="utf-8", errors="replace") for p in sorted(logs.glob("*.log"))
    )


async def test_delegation_keeps_the_run_and_its_result_turn_runs_inside(
    world: HermesWorld,
) -> None:
    script = [
        BoundTurn(tool_calls=(DELEGATE,), requires_tool="delegate_task"),
        BoundTurn(text=(STARTED,), paced=True, requires_tool="delegate_task"),
        BoundTurn(text=("Seven boats.",), hold="child", without_tool="delegate_task"),
        BoundTurn(tool_calls=(TODO,), requires_tool="delegate_task"),
        BoundTurn(text=(RESULT,), paced=True, requires_tool="delegate_task"),
    ]
    async with hermes_rig(world, script, profile=PROFILE, tool_class="host") as rig:
        triggered = await rig.fake.trigger("Count the boats with a team.", session_mode="new")
        try:
            await rig.until(lambda: rig.fake.messages, "the first answer")
            await asyncio.to_thread(rig.model.wait_held, "child")
            await rig.until(lambda: _steps(rig, triggered.run_id, "subagent"), "the subagent step")
        except AssertionError:
            raise AssertionError(gateway_log(rig)) from None
        # The turn has ended, the subagent works: the run stays open with its step.
        assert rig.fake.run_state(triggered.run_id).state == "running", gateway_log(rig)
        [step] = _steps(rig, triggered.run_id, "subagent")
        assert (step["title"], step["status"]) == (GOAL, "running")
        rig.model.release("child")
        try:
            await rig.until(lambda: rig.finished(triggered.run_id), "the end of the run")
        except AssertionError:
            raise AssertionError(gateway_log(rig)) from None
        view = rig.fake.run_state(triggered.run_id)
        assert view.state == "completed", gateway_log(rig)
        # The result turn ran inside the run: its answer is a message of the run.
        assert [(m["content_md"], m["run_id"]) for m in rig.fake.messages] == [
            (STARTED, str(triggered.run_id)),
            (RESULT, str(triggered.run_id)),
        ], gateway_log(rig)
        [step] = _steps(rig, triggered.run_id, "subagent")
        assert step["status"] == "completed"
        # Its tool call ran (the run was open) and is a step of the run.
        assert TOOL_REFUSED not in str(rig.model_requests())
        assert len(_steps(rig, triggered.run_id, "tool_call")) >= 2


async def test_stop_interrupts_the_delegation_and_no_tool_runs_without_a_run(
    world: HermesWorld,
) -> None:
    script = [
        BoundTurn(tool_calls=(DELEGATE,), requires_tool="delegate_task"),
        BoundTurn(text=(STARTED,), paced=True, requires_tool="delegate_task"),
        BoundTurn(text=("Partial count.",), hold="child", without_tool="delegate_task"),
        BoundTurn(tool_calls=(TODO,), requires_tool="delegate_task"),
        BoundTurn(text=("Nothing more.",), paced=True, requires_tool="delegate_task"),
    ]
    texts = [TOOL_REFUSED]
    async with hermes_rig(world, script, profile=PROFILE, tool_class="host", texts=texts) as rig:
        triggered = await rig.fake.trigger("Count the boats with a team.", session_mode="new")
        try:
            await asyncio.to_thread(rig.model.wait_held, "child")
            await rig.until(lambda: _steps(rig, triggered.run_id, "subagent"), "the subagent step")
            await rig.fake.cancel(triggered.run_id)
            await rig.until(lambda: rig.finished(triggered.run_id), "the end of the run")
        except AssertionError:
            raise AssertionError(gateway_log(rig)) from None
        view = rig.fake.run_state(triggered.run_id)
        assert (view.state, view.state_reason) == ("cancelled", "user_cancelled")
        await rig.until(
            lambda: "Interrupted 1 async delegation" in _hermes_log(rig),
            "the interrupt of the delegation",
        )
        rig.model.release("child")
        # Hermes reports the interrupted delegation in a turn of its own, without a run: its tool
        # call is refused, and its text is a report without a run.
        try:
            await rig.until(
                lambda: any(m["content_md"] == "Nothing more." for m in rig.fake.messages),
                "the answer of the turn without a run",
            )
        except AssertionError:
            raise AssertionError(gateway_log(rig) + _hermes_log(rig)[-6000:]) from None
        refused = [
            r for r in rig.model_requests() if TOOL_REFUSED in (r.get("scenario_texts") or [])
        ]
        assert refused, gateway_log(rig)
        late = next(m for m in rig.fake.messages if m["content_md"] == "Nothing more.")
        assert late.get("run_id") is None


async def test_background_process_is_a_job_of_the_run(world: HermesWorld) -> None:
    sleep = {"name": "terminal", "arguments": {"command": "sleep 3", "background": True}}
    script = [
        ScriptTurn(tool_calls=(sleep,)),
        ScriptTurn(text=("The job runs.",), paced=True),
    ]
    async with hermes_rig(world, script, profile=PROFILE, tool_class="host") as rig:
        triggered = await rig.fake.trigger("Start the job.", session_mode="new")
        try:
            await rig.until(lambda: _steps(rig, triggered.run_id, "background_job"), "the job step")
            assert rig.fake.run_state(triggered.run_id).state == "running"
            await rig.until(lambda: rig.finished(triggered.run_id), "the end of the run")
        except AssertionError:
            raise AssertionError(gateway_log(rig)) from None
        assert rig.fake.run_state(triggered.run_id).state == "completed", gateway_log(rig)
        [step] = _steps(rig, triggered.run_id, "background_job")
        assert (step["title"], step["status"]) == ("sleep 3", "completed")
        assert [m["run_id"] for m in rig.fake.messages] == [str(triggered.run_id)]


async def test_a_detached_job_that_registers_is_a_job_of_the_run(world: HermesWorld) -> None:
    """09 17.12 point 8: a job a tool call detaches registers itself and holds the run.

    The ``terminal`` call starts a supervisor in a new session (its launcher exits at once, so
    Hermes' registry never sees it) and registers it with the helper, taking the thread and the
    profile home from the shell Hermes gives the tool.
    """
    launch = (
        "import subprocess, sys; p = subprocess.Popen([sys.executable, '-c', "
        "'import time; time.sleep(4)'], start_new_session=True, stdin=subprocess.DEVNULL, "
        "stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL); print(p.pid)"
    )
    python = world.python
    command = (
        f'SUP=$({python} -c "{launch}") && {python} -m bothy_hermes_adapter job register '
        '--profile-home "$HERMES_HOME" --thread "$HERMES_SESSION_THREAD_ID" --pid "$SUP" '
        '--title "Detached sleep"'
    )
    start = {"name": "terminal", "arguments": {"command": command}}
    script = [
        ScriptTurn(tool_calls=(start,)),
        ScriptTurn(text=("The job runs.",), paced=True),
    ]
    # No approval card for the shell command: the scenario is about the job, not the approval.
    profile = Profile(toolsets=PROFILE.toolsets, config={"approvals": {"mode": "off"}})
    async with hermes_rig(world, script, profile=profile, tool_class="host") as rig:
        triggered = await rig.fake.trigger("Start the long job.", session_mode="new")
        try:
            await rig.until(lambda: _steps(rig, triggered.run_id, "background_job"), "the job step")
            assert rig.fake.run_state(triggered.run_id).state == "running"
            [step] = _steps(rig, triggered.run_id, "background_job")
            assert (step["name"], step["title"]) == ("external_job", "Detached sleep")
            await rig.until(lambda: rig.finished(triggered.run_id), "the end of the run")
        except AssertionError:
            raise AssertionError(gateway_log(rig) + _hermes_log(rig)[-6000:]) from None
        assert rig.fake.run_state(triggered.run_id).state == "completed", gateway_log(rig)
        [step] = _steps(rig, triggered.run_id, "background_job")
        assert step["status"] == "completed"
        assert [m["run_id"] for m in rig.fake.messages] == [str(triggered.run_id)]
