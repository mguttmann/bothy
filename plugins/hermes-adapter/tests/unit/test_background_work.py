# SPDX-License-Identifier: Apache-2.0
"""Background work of a Hermes run (09 17.12, R-AG-44; FIX-hermes-hidden-background).

A turn that starts asynchronous delegations or background processes leaves its run running with a
step per unit until the work is done; the turn Hermes starts by itself for the result runs inside
that run; stop cancels the work; and no tool may run in a thread without an open run. Hermes is
the fake runtime of ``adapter_kit``; its registries are `FakeRuntime.background`.
"""

from __future__ import annotations

import asyncio
import time
from pathlib import Path
from typing import Any
from uuid import UUID, uuid4

import pytest
from adapter_kit import FakeRuntime, Rig, rig

from bothy_hermes_adapter.runtime import (
    BackgroundKind,
    BackgroundTask,
    CardKind,
    CardRequest,
    TurnOutcome,
    TurnRequest,
    Usage,
)

pytestmark = pytest.mark.anyio

FAST: dict[str, Any] = {
    "background_poll_interval": 0.02,
    "background_settle": 0.3,
    "background_progress_interval": 0.2,
}
TEAM = BackgroundTask("deleg_1", BackgroundKind.SUBAGENT, "Count the boats", "running", True)
JOB = BackgroundTask("proc_1", BackgroundKind.JOB, "sleep 600", "running", True)


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def _steps(r: Rig, run_id: UUID, kind: str) -> list[dict[str, Any]]:
    doc = r.fake.run(run_id)
    return [s for s in doc["steps"].values() if s["kind"] == kind]


def _metric_patches(r: Rig, run_id: UUID, name: str) -> list[Any]:
    """The values the adapter patched into ``/metrics/<name>`` of the run, in order."""
    values = []
    for request in r.requests("patchRun"):
        if request.path.endswith(str(run_id)):
            values += [
                op["value"] for op in request.json["ops"] if op["path"] == f"/metrics/{name}"
            ]
    return values


def _starting(*tasks: BackgroundTask, answer: str = "Started a team.") -> Any:
    """A first turn that starts background work and answers at once."""

    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        if len(rt.turns) == 1:
            rt.background[turn.thread_id] = list(tasks)
            rt.events.on_usage(turn.thread_id, Usage("anthropic", "model-small", 11, 7))
            await rt.events.on_answer(turn.thread_id, answer, str(turn.message_id))
            return TurnOutcome.COMPLETED
        await rt.events.on_answer(turn.thread_id, f"About: {turn.user_text}", None)
        return TurnOutcome.COMPLETED

    return script


async def test_async_delegation_keeps_the_run_running_with_a_step(tmp_path: Path) -> None:
    rt = FakeRuntime(script=_starting(TEAM))
    async with rig(tmp_path, runtime=rt, **FAST) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: _steps(r, triggered.run_id, "subagent"), "the subagent step")
        await r.until(lambda: r.fake.messages, "the first answer")
        for _ in range(10):
            await r.fake.pass_time(0.05)
            await asyncio.sleep(0.02)
        assert r.fake.run_state(triggered.run_id).state == "running"
        [step] = _steps(r, triggered.run_id, "subagent")
        assert (step["title"], step["status"]) == ("Count the boats", "running")
        assert r.fake.run(triggered.run_id)["metrics"].get("subagents") == 1
        assert r.handler.tool_allowed(triggered.thread_id) is True
        rt.finish_background(triggered.thread_id, "deleg_1")
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
    assert r.fake.run_state(triggered.run_id).state == "completed"
    [step] = _steps(r, triggered.run_id, "subagent")
    assert step["status"] == "completed"
    assert step.get("finished_at")
    assert _metric_patches(r, triggered.run_id, "subagents") == [1, 0]
    assert [m["run_id"] for m in r.fake.messages] == [str(triggered.run_id)]
    since = set(rt.background_since)
    assert len(since) == 1
    assert abs(since.pop() - time.time()) < 60


async def test_background_process_is_a_background_job_step(tmp_path: Path) -> None:
    rt = FakeRuntime(script=_starting(JOB, answer="Started the job."))
    async with rig(tmp_path, runtime=rt, **FAST) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: _steps(r, triggered.run_id, "background_job"), "the job step")
        assert r.fake.run_state(triggered.run_id).state == "running"
        assert r.fake.run(triggered.run_id)["metrics"].get("background_jobs") == 1
        rt.finish_background(triggered.thread_id, "proc_1", state="failed")
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
    [step] = _steps(r, triggered.run_id, "background_job")
    assert (step["title"], step["status"]) == ("sleep 600", "failed")
    assert r.fake.run_state(triggered.run_id).state == "completed"


async def test_completion_turn_runs_inside_the_run(tmp_path: Path) -> None:
    rt = FakeRuntime(script=_starting(TEAM))
    async with rig(tmp_path, runtime=rt, **FAST) as r:
        triggered = await r.fake.trigger()
        thread = triggered.thread_id
        await r.until(lambda: _steps(r, triggered.run_id, "subagent"), "the subagent step")
        await r.until(lambda: r.fake.messages, "the first answer")
        rt.finish_background(thread, "deleg_1")
        # Hermes' own completion turn: it reaches the adapter only through hooks and callbacks.
        r.handler.on_turn_started(thread)
        assert r.handler.tool_allowed(thread) is True
        r.handler.on_status(thread, "Reading the team's result")
        r.handler.on_status(thread, None)
        r.handler.on_usage(thread, Usage("anthropic", "model-small", 5, 3))
        sent = await r.handler.on_answer(thread, "The team counted seven boats.", None)
        assert sent is not None
        r.handler.on_turn_ended(thread)
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
    assert r.fake.run_state(triggered.run_id).state == "completed"
    assert [(m["content_md"], m["run_id"]) for m in r.fake.messages] == [
        ("Started a team.", str(triggered.run_id)),
        ("The team counted seven boats.", str(triggered.run_id)),
    ]
    tools = _steps(r, triggered.run_id, "tool_call")
    assert [s["title"] for s in tools] == ["Reading the team's result"]
    assert all(s["status"] == "completed" for s in tools)
    finish = r.requests("finishRun")[-1].json
    assert finish["state"] == "completed"
    assert finish["metrics"]["tokens_in"] == 16


async def test_run_waits_for_a_completion_turn_that_runs(tmp_path: Path) -> None:
    rt = FakeRuntime(script=_starting(TEAM))
    async with rig(tmp_path, runtime=rt, **FAST) as r:
        triggered = await r.fake.trigger()
        thread = triggered.thread_id
        await r.until(lambda: _steps(r, triggered.run_id, "subagent"), "the subagent step")
        rt.finish_background(thread, "deleg_1")
        r.handler.on_turn_started(thread)
        # Longer than the settle time: the turn still runs, so the run stays open.
        await asyncio.sleep(0.8)
        await r.fake.pass_time(0.1)
        assert r.fake.run_state(triggered.run_id).state == "running"
        r.handler.on_turn_ended(thread)
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
    assert r.fake.run_state(triggered.run_id).state == "completed"


async def test_progress_counts_while_work_runs(tmp_path: Path) -> None:
    rt = FakeRuntime(script=_starting(TEAM))
    async with rig(tmp_path, runtime=rt, **FAST) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: _steps(r, triggered.run_id, "subagent"), "the subagent step")

        def progressed() -> bool:
            return int(r.fake.run(triggered.run_id)["metrics"].get("progress_events", 0)) >= 2

        await r.until(progressed, "two progress events")
        rt.finish_background(triggered.thread_id, "deleg_1")
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")


async def test_stop_cancels_the_background_work(tmp_path: Path) -> None:
    rt = FakeRuntime(script=_starting(TEAM, JOB))
    async with rig(tmp_path, runtime=rt, **FAST) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: _steps(r, triggered.run_id, "background_job"), "the job step")
        await r.fake.cancel(triggered.run_id)
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
        await r.until(lambda: rt.cancelled_background, "the cancel of the work")
        assert r.handler.tool_allowed(triggered.thread_id) is False
    view = r.fake.run_state(triggered.run_id)
    assert (view.state, view.state_reason) == ("cancelled", "user_cancelled")
    assert rt.cancelled_background == [triggered.thread_id]
    assert rt.interrupted == [triggered.thread_id]
    assert not [t for t in rt.background[triggered.thread_id] if t.open]


async def test_failed_turn_cancels_the_background_work(tmp_path: Path) -> None:
    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        rt.background[turn.thread_id] = [TEAM]
        return TurnOutcome.FAILED

    rt = FakeRuntime(script=script)
    async with rig(tmp_path, runtime=rt, **FAST) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
    view = r.fake.run_state(triggered.run_id)
    assert (view.state, view.state_reason) == ("failed", "runtime_error")
    assert rt.cancelled_background == [triggered.thread_id]


async def test_no_tool_use_outside_a_run(tmp_path: Path) -> None:
    seen: list[bool] = []

    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        seen.append(rt.events.tool_allowed(turn.thread_id))
        await rt.events.on_answer(turn.thread_id, "Done.", str(turn.message_id))
        return TurnOutcome.COMPLETED

    rt = FakeRuntime(script=script)
    async with rig(tmp_path, runtime=rt, **FAST) as r:
        # A thread the adapter never ran a turn in, and an unknown thread: refused.
        assert r.handler.tool_allowed(uuid4()) is False
        assert r.handler.tool_allowed(None) is False
        triggered = await r.fake.trigger()
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
        # The run ended: a turn Hermes starts by itself now has no run, so no tool may run.
        r.handler.on_turn_started(triggered.thread_id)
        assert r.handler.tool_allowed(triggered.thread_id) is False
        r.handler.on_turn_ended(triggered.thread_id)
    assert seen == [True]


async def test_run_without_background_work_finishes_at_once(tmp_path: Path) -> None:
    rt = FakeRuntime()
    async with rig(tmp_path, runtime=rt, **{**FAST, "background_settle": 600.0}) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
    assert r.fake.run_state(triggered.run_id).state == "completed"
    assert not _steps(r, triggered.run_id, "subagent")


async def test_followup_while_held_is_a_turn_of_its_own(tmp_path: Path) -> None:
    rt = FakeRuntime(script=_starting(TEAM))
    async with rig(tmp_path, runtime=rt, **FAST) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: _steps(r, triggered.run_id, "subagent"), "the subagent step")
        await r.until(lambda: r.fake.messages, "the first answer")
        followup = await r.fake.deliver_followup(triggered.run_id, "How far is the team?")
        await r.until(lambda: len(r.fake.messages) == 2, "the answer to the follow-up")
        await r.until(
            lambda: r.fake.followup_state(triggered.run_id, followup) == "absorbed",
            "the absorbed follow-up",
        )
        assert r.fake.run_state(triggered.run_id).state == "running"
        rt.finish_background(triggered.thread_id, "deleg_1")
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
    second = rt.turns[1]
    assert (second.user_text, second.message_id) == ("How far is the team?", followup)
    assert second.run_id == triggered.run_id
    assert [(m["content_md"], m["run_id"]) for m in r.fake.messages] == [
        ("Started a team.", str(triggered.run_id)),
        ("About: How far is the team?", str(triggered.run_id)),
    ]
    assert r.fake.run_state(triggered.run_id).state == "completed"


async def test_voice_followup_while_held_gets_no_turn_of_its_own(tmp_path: Path) -> None:
    """A voice message has no text: no empty turn, and it stays for the follow-on run.

    Follow-ups are not transcribed (09 17.6); the follow-on run takes the message as its
    triggering message and transcribes it there (AUD-HA1/3).
    """
    rt = FakeRuntime(script=_starting(TEAM))
    async with rig(tmp_path, runtime=rt, **FAST) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: _steps(r, triggered.run_id, "subagent"), "the subagent step")
        await r.until(lambda: r.fake.messages, "the first answer")
        audio = r.fake.add_file(b"\x00\x00\x00\x1cftypM4A ", name="note.m4a", mime="audio/mp4")
        voice = await r.fake.deliver_followup(triggered.run_id, "", files=[audio])
        typed = await r.fake.deliver_followup(triggered.run_id, "How far is the team?")
        await r.until(
            lambda: r.fake.followup_state(triggered.run_id, typed) == "absorbed",
            "the absorbed text follow-up",
        )
        rt.finish_background(triggered.thread_id, "deleg_1")
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
        assert r.fake.followup_state(triggered.run_id, voice) == "queued"
    assert [t.user_text for t in rt.turns[1:]] == ["How far is the team?"]


async def test_run_of_an_answered_card_holds_its_background_work(tmp_path: Path) -> None:
    """A card answered by a click, then a delegation: the run stays open (AUD-HA1/1).

    The SDK ends a resumed run as soon as the handler of the click returns, so that handler
    waits for the run's own end, also while the run holds background work (09 17.12).
    """
    question = CardRequest(CardKind.CLARIFY, "c1", "Which lake?", ("North", "South"))

    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        assert await rt.events.on_card(turn.thread_id, question)
        lake = await rt.answers.get()
        rt.background[turn.thread_id] = [TEAM]
        rt.events.on_usage(turn.thread_id, Usage("anthropic", "model-small", 11, 7))
        await rt.events.on_answer(turn.thread_id, f"Started a team at {lake}.", None)
        return TurnOutcome.COMPLETED

    rt = FakeRuntime(script=script)
    async with rig(tmp_path, runtime=rt, **FAST) as r:
        r.fake.enable_costs()
        triggered = await r.fake.trigger()
        await r.until(lambda: r.fake.run_state(triggered.run_id).state == "waiting_input")
        await r.fake.click(triggered.run_id, "cl:c1:0")
        await r.until(lambda: _steps(r, triggered.run_id, "subagent"), "the subagent step")
        for _ in range(10):
            await r.fake.pass_time(0.05)
            await asyncio.sleep(0.02)
        assert r.fake.run_state(triggered.run_id).state == "running"
        assert r.requests("finishRun") == []
        assert r.handler.tool_allowed(triggered.thread_id) is True
        rt.finish_background(triggered.thread_id, "deleg_1")
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
    assert r.fake.run_state(triggered.run_id).state == "completed"
    [finish] = [q.json for q in r.requests("finishRun")]
    assert finish["metrics"]["tokens_in"] == 11
    assert [c["tokens_out"] for c in finish["costs"]] == [7]
    assert [s["status"] for s in _steps(r, triggered.run_id, "subagent")] == ["completed"]


class _SlowStop(FakeRuntime):
    """Stops the work as the platform may: in a worker thread, longer than a poll of the hold."""

    async def cancel_background(self, thread_id: UUID) -> None:
        await asyncio.sleep(0.2)
        await super().cancel_background(thread_id)


@pytest.mark.parametrize("held", [True, False])
async def test_stop_of_a_run_resumed_by_a_card_answer_ends_it_cancelled(
    tmp_path: Path, held: bool
) -> None:
    """The stop of a run that a card answer resumed: ``cancelled``, its work stopped.

    Since the resume the SDK's task of the run is the handler of the answer, so the stop reaches
    that handler. ``execute`` must not return before that handler ended the run, or the SDK
    would end the run ``completed`` (AUD-HA1/6): while the run holds work, and while its turn
    still runs.
    """
    question = CardRequest(CardKind.CLARIFY, "c3", "Which lake?", ("North", "South"))

    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        assert await rt.events.on_card(turn.thread_id, question)
        await rt.answers.get()
        if held:
            rt.background[turn.thread_id] = [TEAM]
            await rt.events.on_answer(turn.thread_id, "Started a team.", None)
            return TurnOutcome.COMPLETED
        await rt.events.on_answer(turn.thread_id, "Working on it.", None)
        await rt.stop_event(turn.thread_id).wait()
        return TurnOutcome.CANCELLED

    rt = _SlowStop(script=script)
    async with rig(tmp_path, runtime=rt, **FAST) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: r.fake.run_state(triggered.run_id).state == "waiting_input")
        await r.fake.click(triggered.run_id, "cl:c3:0")
        await r.until(lambda: len(r.fake.messages) == 2, "the answer after the card")
        if held:
            await r.until(lambda: _steps(r, triggered.run_id, "subagent"), "the subagent step")
        await r.fake.cancel(triggered.run_id)
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
        assert r.handler.tool_allowed(triggered.thread_id) is False
    view = r.fake.run_state(triggered.run_id)
    assert (view.state, view.state_reason) == ("cancelled", "user_cancelled")
    assert [q.json["state"] for q in r.requests("finishRun")] == ["cancelled"]
    assert rt.cancelled_background == [triggered.thread_id]
    assert rt.interrupted == [triggered.thread_id]


@pytest.mark.parametrize("answer", ["click", "text"])
async def test_card_of_a_followup_turn_keeps_the_held_run_open(tmp_path: Path, answer: str) -> None:
    """A card a turn shows while the run holds work: its answer does not end the run.

    A typed answer the turn took is ``absorbed`` at the end of the run (AUD-HA1/2).
    """
    question = CardRequest(CardKind.CLARIFY, "c2", "Which lake?", ("North", "South"))

    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        if len(rt.turns) == 1:
            rt.background[turn.thread_id] = [TEAM]
            await rt.events.on_answer(turn.thread_id, "Started a team.", str(turn.message_id))
            return TurnOutcome.COMPLETED
        assert await rt.events.on_card(turn.thread_id, question)
        lake = await rt.answers.get()
        await rt.events.on_answer(turn.thread_id, f"The team counts at {lake}.", None)
        return TurnOutcome.COMPLETED

    rt = FakeRuntime(script=script)
    async with rig(tmp_path, runtime=rt, **FAST) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: _steps(r, triggered.run_id, "subagent"), "the subagent step")
        await r.fake.deliver_followup(triggered.run_id, "Which lake is it?")
        await r.until(lambda: r.fake.run_state(triggered.run_id).state == "waiting_input")
        typed = None
        if answer == "click":
            await r.fake.click(triggered.run_id, "cl:c2:1")
        else:
            typed = await r.fake.deliver_followup(triggered.run_id, "South")
        await r.until(lambda: len(r.fake.messages) == 3, "the answer after the card")
        for _ in range(10):
            await r.fake.pass_time(0.05)
            await asyncio.sleep(0.02)
        assert r.fake.run_state(triggered.run_id).state == "running"
        rt.finish_background(triggered.thread_id, "deleg_1")
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
        if typed is not None:
            assert r.fake.followup_state(triggered.run_id, typed) == "absorbed"
    assert r.fake.run_state(triggered.run_id).state == "completed"
    assert len(r.requests("finishRun")) == 1
    assert r.fake.messages[-1]["content_md"] == "The team counts at South."


async def test_followup_after_the_answer_gets_a_turn_while_the_run_holds(tmp_path: Path) -> None:
    gate = asyncio.Event()

    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        if len(rt.turns) == 1:
            rt.background[turn.thread_id] = [TEAM]
            await rt.events.on_answer(turn.thread_id, "Started a team.", str(turn.message_id))
            rt.events.on_model_reply(turn.thread_id, 0)  # the answer: no further steering
            await gate.wait()
            return TurnOutcome.COMPLETED
        await rt.events.on_answer(turn.thread_id, f"About: {turn.user_text}", None)
        return TurnOutcome.COMPLETED

    rt = FakeRuntime(script=script)
    async with rig(tmp_path, runtime=rt, **FAST) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: r.fake.messages, "the first answer")
        followup = await r.fake.deliver_followup(triggered.run_id, "Also count the ducks.")
        await r.fake.pass_time(0.2)
        assert rt.steered == []  # after the answer nothing is steered into the turn (17.5)
        gate.set()
        await r.until(lambda: len(rt.turns) == 2, "the follow-up's own turn")
        await r.until(
            lambda: r.fake.followup_state(triggered.run_id, followup) == "absorbed",
            "the absorbed follow-up",
        )
        rt.finish_background(triggered.thread_id, "deleg_1")
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
    assert rt.turns[1].user_text == "Also count the ducks."
    assert [m["content_md"] for m in r.fake.messages] == [
        "Started a team.",
        "About: Also count the ducks.",
    ]


async def test_followup_without_background_work_stays_for_the_follow_on_run(
    tmp_path: Path,
) -> None:
    gate = asyncio.Event()

    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        await rt.events.on_answer(turn.thread_id, "Done.", str(turn.message_id))
        rt.events.on_model_reply(turn.thread_id, 0)
        await gate.wait()
        return TurnOutcome.COMPLETED

    rt = FakeRuntime(script=script)
    async with rig(tmp_path, runtime=rt, **FAST) as r:
        triggered = await r.fake.trigger()
        await r.until(lambda: r.fake.messages, "the answer")
        followup = await r.fake.deliver_followup(triggered.run_id, "And then?")
        gate.set()
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
        assert r.fake.followup_state(triggered.run_id, followup) == "queued"
    assert len(rt.turns) == 1


async def test_followup_during_the_result_turn_waits_and_the_run_ends(tmp_path: Path) -> None:
    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        if len(rt.turns) == 1:
            rt.background[turn.thread_id] = [TEAM]
            await rt.events.on_answer(turn.thread_id, "Started a team.", str(turn.message_id))
            return TurnOutcome.COMPLETED
        await rt.events.on_answer(turn.thread_id, f"About: {turn.user_text}", None)
        rt.events.on_turn_ended(turn.thread_id)  # Hermes' on_session_end fires inside the turn
        return TurnOutcome.COMPLETED

    rt = FakeRuntime(script=script)
    async with rig(tmp_path, runtime=rt, **FAST) as r:
        triggered = await r.fake.trigger()
        thread = triggered.thread_id
        await r.until(lambda: _steps(r, triggered.run_id, "subagent"), "the subagent step")
        await r.until(lambda: r.fake.messages, "the first answer")
        rt.finish_background(thread, "deleg_1")
        r.handler.on_turn_started(thread)  # Hermes' result turn runs
        followup = await r.fake.deliver_followup(triggered.run_id, "Is it done?")
        for _ in range(10):
            await r.fake.pass_time(0.05)
            await asyncio.sleep(0.02)
        # One after the other (17.12 point 5): no turn of the adapter while the result turn runs.
        assert len(rt.turns) == 1
        await r.handler.on_answer(thread, "The team counted seven boats.", None)
        r.handler.on_turn_ended(thread)
        await r.until(lambda: len(rt.turns) == 2, "the follow-up's own turn")
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
        assert r.fake.followup_state(triggered.run_id, followup) == "absorbed"
    assert r.fake.run_state(triggered.run_id).state == "completed"
    assert [m["content_md"] for m in r.fake.messages] == [
        "Started a team.",
        "The team counted seven boats.",
        "About: Is it done?",
    ]


# ---- the end of a result turn without on_session_end (FIX-hermes-result-turn-end) -----------

FAILED_ANSWER = "Anthropic rejected the request and retrying won't help: HTTP 400"


async def _held_with_finished_work(r: Rig, rt: FakeRuntime) -> tuple[UUID, UUID]:
    """A run that started a team whose unit already finished; returns the run and its thread."""
    triggered = await r.fake.trigger()
    thread = triggered.thread_id
    await r.until(lambda: _steps(r, triggered.run_id, "subagent"), "the subagent step")
    await r.until(lambda: r.fake.messages, "the first answer")
    rt.finish_background(thread, "deleg_1")
    return triggered.run_id, thread


async def _pass(r: Rig, seconds: float) -> None:
    for _ in range(int(seconds / 0.05)):
        await r.fake.pass_time(0.05)
        await asyncio.sleep(0.05)


async def test_failed_result_turn_without_session_end_fails_the_run(tmp_path: Path) -> None:
    # Hermes v0.22.5 returns from a failed turn (a model error, exhausted retries, an interrupt,
    # an exception) without finalize_turn, so on_session_end never comes; only its session's busy
    # state tells that the turn ended.
    rt = FakeRuntime(script=_starting(TEAM), busy=set())
    async with rig(tmp_path, runtime=rt, **FAST) as r:
        run_id, thread = await _held_with_finished_work(r, rt)
        rt.busy.add(thread)
        r.handler.on_turn_started(thread)
        followup = await r.fake.deliver_followup(run_id, "Is it done?")
        await _pass(r, 0.5)
        assert r.fake.run_state(run_id).state == "running"
        rt.busy.discard(thread)
        # Hermes delivers the failed turn's text after the session is free again.
        await asyncio.sleep(0.05)
        await r.handler.on_answer(thread, FAILED_ANSWER, None)
        await r.until(lambda: r.finished(run_id), "the end of the run")
        # The follow-up gets its turn in the follow-on run, not in the failed run (09 7).
        assert r.fake.followup_state(run_id, followup) == "queued"
    view = r.fake.run_state(run_id)
    assert (view.state, view.state_reason) == ("failed", "runtime_error")
    assert len(rt.turns) == 1
    assert [(m["content_md"], m["run_id"]) for m in r.fake.messages] == [
        ("Started a team.", str(run_id)),
        (FAILED_ANSWER, str(run_id)),
    ]


async def test_failed_result_turn_cancels_the_open_work(tmp_path: Path) -> None:
    other = BackgroundTask("proc_1", BackgroundKind.JOB, "sleep 600", "running", True)
    rt = FakeRuntime(script=_starting(TEAM, other), busy=set())
    async with rig(tmp_path, runtime=rt, **FAST) as r:
        run_id, thread = await _held_with_finished_work(r, rt)
        rt.busy.add(thread)
        r.handler.on_turn_started(thread)
        await _pass(r, 0.1)
        rt.busy.discard(thread)
        await r.until(lambda: r.finished(run_id), "the end of the run")
    view = r.fake.run_state(run_id)
    assert (view.state, view.state_reason) == ("failed", "runtime_error")
    assert rt.cancelled_background == [thread]
    [job] = _steps(r, run_id, "background_job")
    assert job["status"] == "cancelled"


async def test_result_turn_hermes_reports_failed_fails_the_run(tmp_path: Path) -> None:
    rt = FakeRuntime(script=_starting(TEAM), busy=set())
    async with rig(tmp_path, runtime=rt, **FAST) as r:
        run_id, thread = await _held_with_finished_work(r, rt)
        rt.busy.add(thread)
        r.handler.on_turn_started(thread)
        r.handler.on_turn_ended(thread, failed=True)
        rt.busy.discard(thread)
        await r.until(lambda: r.finished(run_id), "the end of the run")
    view = r.fake.run_state(run_id)
    assert (view.state, view.state_reason) == ("failed", "runtime_error")


async def test_result_turn_that_started_before_the_hold_keeps_the_run(tmp_path: Path) -> None:
    # The result turn starts while the run's own turn still counts (pre_llm_call before the
    # hold): only the session's busy state shows it.
    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        rt.background[turn.thread_id] = [_ended(TEAM)]
        await rt.events.on_answer(turn.thread_id, "Started a team.", str(turn.message_id))
        assert rt.busy is not None
        rt.busy.add(turn.thread_id)
        rt.events.on_turn_started(turn.thread_id)
        return TurnOutcome.COMPLETED

    rt = FakeRuntime(script=script, busy=set())
    async with rig(tmp_path, runtime=rt, **FAST) as r:
        triggered = await r.fake.trigger()
        thread = triggered.thread_id
        await r.until(lambda: _steps(r, triggered.run_id, "subagent"), "the subagent step")
        # Longer than the settle time: the result turn still runs, so the run stays open.
        await _pass(r, 0.8)
        assert r.fake.run_state(triggered.run_id).state == "running"
        sent = await r.handler.on_answer(thread, "The team counted seven boats.", None)
        assert sent is not None
        r.handler.on_turn_ended(thread)
        rt.busy.discard(thread)
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
    assert r.fake.run_state(triggered.run_id).state == "completed"
    assert [(m["content_md"], m["run_id"]) for m in r.fake.messages] == [
        ("Started a team.", str(triggered.run_id)),
        ("The team counted seven boats.", str(triggered.run_id)),
    ]


async def test_followup_runs_after_a_result_turn_seen_only_as_busy(tmp_path: Path) -> None:
    rt = FakeRuntime(script=_starting(TEAM), busy=set())
    async with rig(tmp_path, runtime=rt, **FAST) as r:
        run_id, thread = await _held_with_finished_work(r, rt)
        rt.busy.add(thread)  # a result turn whose pre_llm_call the run did not see
        followup = await r.fake.deliver_followup(run_id, "Is it done?")
        await _pass(r, 0.5)
        assert len(rt.turns) == 1  # one after the other (17.12 point 5)
        r.handler.on_turn_ended(thread)
        rt.busy.discard(thread)
        await r.until(lambda: len(rt.turns) == 2, "the follow-up's own turn")
        await r.until(lambda: r.finished(run_id), "the end of the run")
        assert r.fake.followup_state(run_id, followup) == "absorbed"
    assert r.fake.run_state(run_id).state == "completed"


def _ended(task: BackgroundTask) -> BackgroundTask:
    """``task`` as Hermes reports it once the unit ended and its result turn was admitted."""
    return BackgroundTask(task.task_id, task.kind, task.title, "completed", False)
