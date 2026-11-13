# SPDX-License-Identifier: Apache-2.0
"""An account error does not end a run that holds background work (09 18.5, 18.9).

FIX-RUNNER-LIMIT-KEEPS-RUN. A turn of Alex started a background job and answered; the process
stays for the job. Then the account of Claude Code does not work for a moment (a used-up usage
window, a login that is gone, subscription access switched off, an overloaded API): the turn
the CLI starts for a task's notification, or the turn of a follow-up, ends with an API error.
Before, the runner closed stdin at that error result and stopped the process
``runner.stop_grace`` later, so every background task ended and nothing went on until a person
wrote. Now the runner keeps the process, shows why the run waits, and writes a retry line after
a short pause; only when the account stays unusable for ``runner.account_retry_max_wait`` does
the run end as before, with the reason in its summary.

The members of the API errors are those the pinned CLI writes (``tests/fake_claude/fixtures``).
The pauses are a few hundred milliseconds here; Fake-Claude waits on gates only.
"""

from __future__ import annotations

import json
from collections.abc import Callable
from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.testing._patience import PATIENCE

from claude_code_runner.app import Runner
from claude_code_runner.causes import adapter_stopped_cause

from .conftest import LogLines, World

pytestmark = pytest.mark.anyio

NEVER = f"{int(100 * PATIENCE)}s"
"""A pause or bound no scenario reaches."""
FAST = {
    "followup_grace": "200ms",
    "account_retry_first": "300ms",
    "account_retry_second": "600ms",
    "account_retry_interval": "600ms",
    "account_retry_max_wait": NEVER,
}
"""Short pauses and no bound: the retries decide the scenario."""
WAITING = "Waiting for the Claude Code account: "
LIMIT = "the usage limit of the account is reached and resets at 2026-11-13 02:10 UTC"
"""The fixture ``usage_limit`` in the runner's words: the class and the reset time in UTC."""
ENDED = ("completed", "failed", "cancelled")


@pytest.fixture
def world_timings() -> dict[str, float]:
    return {"agent.cancel_grace": 100 * PATIENCE, "agent.invocation_token_ttl": 100 * PATIENCE}


def texts(world: World) -> list[str]:
    return [m["content_md"] for m in world.fake.messages]


def status_line(world: World, run_id: UUID) -> str:
    return str(world.fake.run(run_id).get("status_line", ""))


def retries(world: World) -> list[dict[str, Any]]:
    """The retry lines Fake-Claude read on stdin, each with the time it was read."""
    found = []
    for call in world.calls("stdin"):
        try:
            text = json.loads(call["line"])["message"]["content"][0]["text"]
        except (ValueError, KeyError, IndexError, TypeError):
            continue
        if str(text).startswith("Automatic retry by the runner"):
            found.append({**call, "text": text})
    return found


def events(logs: LogLines) -> list[dict[str, Any]]:
    """The runner's own log lines as objects."""
    out = []
    for record in logs.records:
        if record.name != "claude_code_runner":
            continue
        try:
            item = json.loads(record.getMessage())
        except ValueError:
            continue
        if isinstance(item, dict):
            out.append(item)
    return out


def named(logs: LogLines, event: str) -> list[dict[str, Any]]:
    return [e for e in events(logs) if e.get("event") == event]


async def reached(world: World, run_id: UUID, predicate: Callable[[], Any], what: str) -> None:
    """Wait for ``predicate`` while the run goes on; a run that ends first fails the test."""

    def check() -> bool:
        if predicate():
            return True
        view = world.fake.run_state(run_id)
        if view.state in ENDED:
            summary = world.fake.run(run_id).get("summary")
            raise AssertionError(
                f"the run ended {view.state} ({view.state_reason}; {summary}) before {what}"
            )
        return False

    await world.until(check)


async def held_run(
    world: World, config: dict[str, Any] | None = None, **scenario: Any
) -> tuple[Runner, UUID, Any]:
    """A run whose first turn started a background job and answered: its process is held."""
    job = world.gate("background_gate")
    world.scenario(fixture="plain", background_gate=str(job), **scenario)
    runner = await world.start(world.config(**(FAST if config is None else config)))
    run = await world.fake.trigger("Run the suite", channel_id=world.forum_a)
    await world.until(lambda: texts(world) == ["Done."])
    return runner, run.run_id, job


# ---- the reproduction: an account error while background work is alive -------------------------


async def test_a_usage_limit_at_a_task_notification_keeps_the_work_and_tries_again(
    world: World, logs: LogLines
) -> None:
    # The reported failure: a task's notification wakes the main loop, its request meets a
    # used-up usage window, and the runner logged turn_stopped_after_error and stopped the
    # process with its tasks.
    failure = world.gate("background_error_gate")
    _, run_id, job = await held_run(
        world, background_error_gate=str(failure), background_api_error="usage_limit"
    )
    world.release(failure)
    await reached(
        world, run_id, lambda: status_line(world, run_id).startswith(WAITING), "it showed the wait"
    )
    # A person sees why the run waits: the class and the reset time, never a name of the account.
    assert status_line(world, run_id) == (
        f"{WAITING}{LIMIT}. The session and its background work are kept; next try in 1 second."
    )
    assert world.state_of(run_id) == "running"
    await reached(world, run_id, lambda: len(retries(world)) == 1, "the first retry")
    # The retry is a turn of its own in the same process; its answer is posted.
    await reached(world, run_id, lambda: len(texts(world)) == 2, "the answer of the retry")
    assert texts(world)[1] == "Status: the background job is still running."
    await reached(world, run_id, lambda: status_line(world, run_id) == "", "the wait ended")
    assert world.state_of(run_id) == "running"
    assert world.calls("stdin_eof") == []  # stdin never closed
    assert world.calls("background_killed") == []  # the task lives
    # The pause is the configured one, counted from the error; the reset time is no pause.
    [failed] = world.calls("background_turn_failed")
    [retry] = retries(world)
    assert retry["t"] - failed["t"] >= 0.3
    world.release(job)
    assert await world.ended(run_id) == "completed"
    assert texts(world)[2] == "The suite passed."
    assert len(world.turns()) == 1  # one process for the whole run
    assert world.calls("background_killed") == []
    [started] = named(logs, "account_wait_started")
    assert (started["error"], started["background_tasks"]) == ("usage_limit", "1")
    assert [e["attempt"] for e in named(logs, "account_retry")] == ["1"]
    assert len(named(logs, "account_wait_ended")) == 1
    assert named(logs, "turn_stopped_after_error") == []
    assert named(logs, "stop_escalated") == []


@pytest.mark.parametrize(
    ("fixture", "words"),
    [
        ("usage_limit", LIMIT),
        ("not_authenticated", "Claude Code is not logged in"),
        (
            "subscription_disabled",
            "the organization of the account has switched off subscription access for Claude Code",
        ),
        ("overloaded", "the API is overloaded"),
    ],
)
async def test_every_account_error_keeps_the_process_and_is_tried_again(
    world: World, fixture: str, words: str
) -> None:
    failure = world.gate("background_error_gate")
    _, run_id, job = await held_run(
        world, background_error_gate=str(failure), background_api_error=fixture
    )
    world.release(failure)
    await reached(
        world, run_id, lambda: status_line(world, run_id).startswith(WAITING), "it showed the wait"
    )
    assert status_line(world, run_id).startswith(f"{WAITING}{words}. ")
    await reached(world, run_id, lambda: len(texts(world)) == 2, "the answer of the retry")
    assert len(retries(world)) == 1
    assert world.calls("background_killed") == []
    world.release(job)
    assert await world.ended(run_id) == "completed"
    assert texts(world) == [
        "Done.",
        "Status: the background job is still running.",
        "The suite passed.",
    ]
    assert len(world.turns()) == 1


async def test_the_second_retry_comes_after_the_first_one_failed(world: World) -> None:
    failure = world.gate("background_error_gate")
    account = world.gate("account_gate")
    _, run_id, job = await held_run(
        world,
        background_error_gate=str(failure),
        background_api_error="usage_limit",
        account_gate=str(account),  # the account does not work until the gate opens
    )
    world.release(failure)
    await reached(world, run_id, lambda: world.calls("account_refused"), "the first retry failed")
    world.release(account)  # the account works again
    await reached(world, run_id, lambda: len(texts(world)) == 2, "the answer of the second retry")
    first, second = retries(world)
    [failed] = world.calls("background_turn_failed")
    [refused] = world.calls("account_refused")
    assert first["t"] - failed["t"] >= 0.3  # runner.account_retry_first
    assert second["t"] - refused["t"] >= 0.6  # runner.account_retry_second
    assert world.state_of(run_id) == "running"
    assert world.calls("stdin_eof") == []
    assert world.calls("background_killed") == []
    world.release(job)
    assert await world.ended(run_id) == "completed"
    assert len(world.turns()) == 1


async def test_a_followup_that_meets_the_limit_is_answered_by_the_retry(world: World) -> None:
    account = world.gate("account_gate")
    _, run_id, job = await held_run(
        world, background_api_error="usage_limit", account_gate=str(account)
    )
    message_id = await world.fake.deliver_followup(run_id, "What is the state?", author="alex")
    await reached(
        world, run_id, lambda: status_line(world, run_id).startswith(WAITING), "it showed the wait"
    )
    assert world.fake.followup_state(run_id, message_id) == "absorbed"  # the turn took it
    assert texts(world) == ["Done."]
    world.release(account)
    await reached(world, run_id, lambda: len(texts(world)) == 2, "the answer of the retry")
    assert len(retries(world)) >= 1
    assert world.state_of(run_id) == "running"
    assert world.calls("background_killed") == []
    world.release(job)
    assert await world.ended(run_id) == "completed"
    assert world.fake.followup_state(run_id, message_id) == "absorbed"
    assert len(world.turns()) == 1


async def test_the_notification_of_the_last_task_is_tried_again(world: World) -> None:
    # The task list is empty when the last task's notification turn runs. Its failure ended
    # the run too, and the result of the finished work was never reported.
    account = world.gate("account_gate")
    _, run_id, job = await held_run(
        world, background_api_error="usage_limit", account_gate=str(account)
    )
    world.release(job)  # the job ends; its notification turn meets the limit
    await reached(
        world, run_id, lambda: status_line(world, run_id).startswith(WAITING), "it showed the wait"
    )
    assert world.calls("stdin_eof") == []
    world.release(account)
    assert await world.ended(run_id) == "completed"
    assert texts(world) == ["Done.", "Status: the background job is finished."]
    assert len(retries(world)) >= 1
    assert len(world.turns()) == 1


# ---- the bound, the stop and the ends that stay as they were -----------------------------------


async def test_the_bound_ends_the_run_with_the_reason(world: World, logs: LogLines) -> None:
    failure = world.gate("background_error_gate")
    account = world.gate("account_gate")  # never opened: the account stays unusable
    bounded = {**FAST, "account_retry_second": "300ms", "account_retry_interval": "300ms"}
    bounded["account_retry_max_wait"] = "1500ms"
    _, run_id, _job = await held_run(
        world,
        bounded,
        background_error_gate=str(failure),
        background_api_error="usage_limit",
        account_gate=str(account),
        background_survives_eof=True,
    )
    world.release(failure)
    assert await world.ended(run_id) == "failed"
    tried = len(retries(world))
    assert tried >= 1
    assert len(world.calls("account_refused")) == tried
    # From the bound on the error is an ordinary one: stdin closes, the process is stopped
    # runner.stop_grace later, and the lost work is named.
    assert world.calls("eof_with_background")
    assert [c.get("signal") for c in world.calls("background_killed")] == ["SIGINT"]
    view = world.fake.run_state(run_id)
    assert view.state_reason == "runtime_error"
    summary = world.fake.run(run_id)["summary"]
    assert summary.startswith(f"Claude Code could not use its account on the runner host: {LIMIT}.")
    assert f"tried the turn again {tried} time" in summary
    assert "runner.account_retry_max_wait" in summary
    assert "1 background task was still running and was stopped; its result is lost." in summary
    assert "next message starts a new run that continues the same session" in summary
    assert summary.endswith("(`runtime_error`)")
    assert not status_line(world, run_id).startswith(WAITING)
    steps = world.fake.run(run_id)["steps"].values()
    assert [s["status"] for s in steps if s["kind"] == "background_job"] == ["cancelled"]
    [gave_up] = named(logs, "account_wait_gave_up")
    assert (gave_up["error"], gave_up["retries"]) == ("usage_limit", str(tried))
    [stopped] = named(logs, "turn_stopped_after_error")
    assert stopped["background_tasks"] == "1"


async def test_stop_during_the_wait_ends_the_run_at_once(world: World, logs: LogLines) -> None:
    failure = world.gate("background_error_gate")
    waiting = {**FAST, "account_retry_first": NEVER}  # no retry comes inside the scenario
    _, run_id, _job = await held_run(
        world, waiting, background_error_gate=str(failure), background_api_error="usage_limit"
    )
    world.release(failure)
    await reached(
        world, run_id, lambda: status_line(world, run_id).startswith(WAITING), "it showed the wait"
    )
    await world.fake.cancel(run_id)
    assert await world.ended(run_id) == "cancelled"
    assert world.fake.run_state(run_id).state_reason == "user_cancelled"
    assert status_line(world, run_id) == ""  # a stopped run does not say that it waits
    assert retries(world) == []
    assert [c.get("signal") for c in world.calls("background_killed")] == ["SIGINT"]
    await world.until(lambda: world.calls("turn_end"))
    assert named(logs, "turn_stopped_after_error") == []
    assert texts(world) == ["Done."]


async def test_a_stop_of_the_runner_during_the_wait_ends_the_turn(world: World) -> None:
    failure = world.gate("background_error_gate")
    waiting = {**FAST, "account_retry_first": NEVER}
    runner, run_id, _job = await held_run(
        world, waiting, background_error_gate=str(failure), background_api_error="usage_limit"
    )
    world.release(failure)
    await reached(
        world, run_id, lambda: status_line(world, run_id).startswith(WAITING), "it showed the wait"
    )
    await runner.stop()
    assert await world.ended(run_id) == "cancelled"
    finish = world.fake.run(run_id)
    assert finish["state_reason"] == "adapter_stopped"
    assert finish["summary"] == adapter_stopped_cause(1)
    assert "status_line" not in finish
    await world.until(lambda: world.calls("turn_end"))


async def test_a_turn_without_background_work_fails_at_once_and_says_why(world: World) -> None:
    world.scenario(fixture="plain", api_error="usage_limit")
    await world.start(world.config(**FAST))
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run_state(run.run_id).state_reason == "runtime_error"
    summary = world.fake.run(run.run_id)["summary"]
    assert summary.startswith(f"Claude Code could not use its account on the runner host: {LIMIT}.")
    assert "no background work to keep, so the run ended at once" in summary
    assert summary.endswith("(`runtime_error`)")
    assert retries(world) == []
    assert len(world.turns()) == 1
    assert texts(world) == []


async def test_an_ordinary_error_while_background_runs_still_ends_the_run(
    world: World, logs: LogLines
) -> None:
    # Not weakened: an error that is no account error ends the run as before, with the stop
    # after runner.stop_grace and the lost work in the summary (FIX-runner-auth-failed-held).
    failure = world.gate("background_error_gate")
    _, run_id, _job = await held_run(
        world,
        background_error_gate=str(failure),
        background_api_error="server_error",
        background_survives_eof=True,
    )
    world.release(failure)
    assert await world.ended(run_id) == "failed"
    assert world.fake.run_state(run_id).state_reason == "runtime_error"
    assert world.fake.run(run_id)["summary"] == (
        "1 background task was still running and was stopped; its result is lost. Your next "
        "message starts a new run that continues the same session (`runtime_error`)"
    )
    assert retries(world) == []
    assert world.calls("eof_with_background")
    assert [c.get("signal") for c in world.calls("background_killed")] == ["SIGINT"]
    assert len(named(logs, "turn_stopped_after_error")) == 1
    assert named(logs, "account_wait_started") == []


async def test_the_retry_is_off_with_a_bound_of_zero(world: World) -> None:
    failure = world.gate("background_error_gate")
    off = {**FAST, "account_retry_max_wait": "0s"}
    _, run_id, _job = await held_run(
        world,
        off,
        background_error_gate=str(failure),
        background_api_error="usage_limit",
        background_survives_eof=True,
    )
    world.release(failure)
    assert await world.ended(run_id) == "failed"
    assert retries(world) == []
    summary = world.fake.run(run_id)["summary"]
    assert summary.startswith(f"Claude Code could not use its account on the runner host: {LIMIT}.")
    assert "1 background task was still running and was stopped" in summary
    assert "tried the turn again" not in summary
    assert "no background work" not in summary
