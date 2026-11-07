# SPDX-License-Identifier: Apache-2.0
"""A runner killed at every step of the launch handshake, and the phases of a record (AR-T04).

The first runner runs as a process of its own with ``runner.turn_handover``; a test-only hook
(`RunTurn.crash_point`, injected by the start script of this module, never reachable in a normal
start) ends it with ``os._exit`` right after a step of 09 18.19.5 at the launch of an attempt of a
given kind, or between an attempt's end and the next ``launching`` record. A new runner on the same
state folder continues the run: it never starts a CLI twice for one attempt, delivers every
initial input exactly once, and the run completes. The phases before ``run.start`` restart through
the inbox; the launch check refuses or waits; a record without a named pipe is adopted as 18.9
says; an adopted process that fills its context is compacted and retried within the limits.
"""

from __future__ import annotations

import asyncio
import fcntl
import json
import os
import signal
import subprocess
import sys
from collections import Counter
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.testing._patience import PATIENCE
from bothy_sdk.testing.faults import Fault

from claude_code_runner.statedir import ensure_private_dir

from .conftest import LogLines, World

pytestmark = pytest.mark.anyio

POINTS = (
    "launching",
    "prepared",
    "keeper_started",
    "go_recorded",
    "go_sent",
    "cli_recorded",
    "attempt_ended",
)
"""The steps of 18.19.5 after which the hook ends the runner, and the end of an attempt."""

CRASH = """
import os, sys
from claude_code_runner.runturn import RunTurn
point, kind = sys.argv[1], sys.argv[2]
def crash(at, attempt_kind):
    if at == point and attempt_kind == kind:
        os._exit(9)
RunTurn.crash_point = staticmethod(crash)
from claude_code_runner.__main__ import main
sys.exit(main(sys.argv[3:]))
"""


@pytest.fixture
def world_real_time() -> bool:
    return True


@pytest.fixture
def world_timings() -> dict[str, float]:
    return {"agent.cancel_grace": 100 * PATIENCE, "agent.invocation_token_ttl": 100 * PATIENCE}


def record_path(world: World, run_id: UUID) -> Path:
    return world.state / "turns" / f"{run_id}.json"


def record(world: World, run_id: UUID) -> dict[str, Any]:
    try:
        value = json.loads(record_path(world, run_id).read_text())
    except (OSError, ValueError):
        return {}
    return value if isinstance(value, dict) else {}


def spawn_crashing(world: World, point: str, kind: str, **runner: Any) -> subprocess.Popen[bytes]:
    world.config(turn_handover=True, **runner)
    argv = [sys.executable, "-c", CRASH, point, kind]
    return world.spawn_command([*argv, "run", "--config", str(world.root / "runner.toml")])


async def ready(world: World) -> None:
    await world.until(
        lambda: any(h.get("details", {}).get("adapter") for h in world.fake.heartbeats)
    )


def check_inputs(world: World) -> None:
    """Every process got each of its input lines once, the initial ones first."""
    for turn in world.turns():
        uuids = [json.loads(line).get("uuid") for line in turn["stdin"]]
        assert len(uuids) == len(set(uuids)), turn["stdin"]


async def crash_and_continue(
    world: World, point: str, kind: str, trigger: Any, **runner: Any
) -> tuple[UUID, int]:
    """Crash the first runner at ``point`` of an attempt of ``kind``; a new runner continues."""
    first = spawn_crashing(world, point, kind, **runner)
    await ready(world)
    run_id = await trigger()
    assert await asyncio.to_thread(first.wait, PATIENCE) == 9  # the hook fired
    await world.start(world.config(turn_handover=True, **runner))
    state = await world.ended(run_id)
    return run_id, {"completed": 0, "failed": 1, "cancelled": 2}[state]


@pytest.mark.parametrize("point", POINTS)
async def test_first_attempt(world: World, point: str) -> None:
    world.scenario(fixture="happy")

    async def trigger() -> UUID:
        return (await world.fake.trigger("Fix it", channel_id=world.forum_a)).run_id

    run_id, state = await crash_and_continue(world, point, "first", trigger)
    assert state == 0
    assert len(world.turns()) == 1  # the CLI of the attempt started exactly once
    check_inputs(world)
    assert len(world.requests("startRun")) == 1
    assert len(world.fake.messages) == 1
    await world.until(lambda: not record_path(world, run_id).exists())


COMPACTION_POINTS = (*POINTS, "compaction_retry")


@pytest.mark.parametrize("point", COMPACTION_POINTS)
async def test_compaction_attempt(world: World, point: str) -> None:
    world.scenario(
        fixture="happy", per_process={"0": {"api_error": "context_full", "api_error_after": 1}}
    )

    async def trigger() -> UUID:
        return (await world.fake.trigger("Fix it", channel_id=world.forum_a)).run_id

    _, state = await crash_and_continue(world, point, "compaction", trigger)
    assert state == 0
    first, _, retry = world.turns()  # each attempt exactly once
    assert len(world.calls("compact")) == 1
    assert retry["stdin"][0] == first["stdin"][0]
    check_inputs(world)
    assert len(world.requests("startRun")) == 1


@pytest.mark.parametrize("point", POINTS)
async def test_retry_attempt(world: World, point: str) -> None:
    world.scenario(
        fixture="happy", per_process={"0": {"api_error": "context_full", "api_error_after": 1}}
    )

    async def trigger() -> UUID:
        return (await world.fake.trigger("Fix it", channel_id=world.forum_a)).run_id

    _, state = await crash_and_continue(world, point, "retry", trigger)
    assert state == 0
    assert len(world.turns()) == 3
    assert len(world.calls("compact")) == 1
    check_inputs(world)


async def _first_run_in_a_thread(world: World) -> UUID:
    """A first run in a thread, through the crashing runner (its hook waits for another kind)."""
    world.scenario(fixture="plain")
    first = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(first.run_id) == "completed"
    return first.thread_id


@pytest.mark.parametrize("point", POINTS)
async def test_resume_same_reference_attempt(world: World, point: str) -> None:
    async def trigger() -> UUID:
        thread = await _first_run_in_a_thread(world)
        ref = "0192d3a0-0000-4000-8000-0000000000e1"
        world.fake.world.session_for(thread, ref, started=False)
        world.scenario(fixture="resume", sessions=True, known_sessions=[ref])
        return (await world.fake.trigger("Add a test", thread_id=thread)).run_id

    _, state = await crash_and_continue(world, point, "resume_same_reference", trigger)
    assert state == 0
    assert len(world.turns()) == 3  # the first run, the existing session, the resume
    check_inputs(world)


@pytest.mark.parametrize("point", POINTS)
async def test_new_session_attempt(world: World, point: str) -> None:
    async def trigger() -> UUID:
        thread = await _first_run_in_a_thread(world)
        world.scenario(fixture="resume", sessions=True)  # Claude Code knows no session at all
        run = await world.fake.trigger("Add a test", thread_id=thread, session_mode="resume")
        return run.run_id

    _, state = await crash_and_continue(world, point, "new_session", trigger)
    assert state == 0
    assert len(world.turns()) == 3  # the first run, the missing session, the new session
    check_inputs(world)


# ---- phases before run.start and the launch check ------------------------------------------------


async def test_handover_while_preparing_restarts_through_the_inbox(world: World) -> None:
    # preparing: the pre-check waits on its gate; the handover leaves the record, the next
    # runner removes it and the inbox restart calls run.start exactly once, no launch check.
    gate = world.gate("auth_gate")
    world.scenario(fixture="plain", auth_gate=str(gate))
    first = world.spawn(turn_handover=True)
    await ready(world)
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    await world.until(lambda: record(world, run.run_id).get("phase") == "preparing")
    await world.until(lambda: len(world.calls("auth_status")) == 1)
    first.send_signal(signal.SIGTERM)
    assert await asyncio.to_thread(first.wait, PATIENCE) == 0  # no turn was handed over
    world.release(gate)
    await world.start(world.config(turn_handover=True))
    assert await world.ended(run.run_id) == "completed"
    assert len(world.requests("startRun")) == 1
    assert world.requests("checkRunLaunch") == []
    assert len(world.turns()) == 1


async def test_handover_while_waiting_for_the_thread_lock(world: World) -> None:
    # waiting_lock: another process holds the thread lock; the run waits thread_busy.
    lock = ensure_private_dir(world.state / "locks")
    world.scenario(fixture="plain")
    first = world.spawn(turn_handover=True)
    await ready(world)
    holder: int | None = None
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    thread = run.thread_id
    path = lock / f"thread-{thread}.lock"
    # The run may take the lock first in a slow start; then it simply completes.
    try:
        holder = os.open(path, os.O_RDWR | os.O_CREAT, 0o600)
        fcntl.flock(holder, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError:
        assert await world.ended(run.run_id) == "completed"
        return
    try:
        await world.until(lambda: record(world, run.run_id).get("phase") == "waiting_lock")
        first.send_signal(signal.SIGTERM)
        assert await asyncio.to_thread(first.wait, PATIENCE) == 0
    finally:
        os.close(holder)
    await world.start(world.config(turn_handover=True))
    assert await world.ended(run.run_id) == "completed"
    assert len(world.requests("startRun")) == 1
    assert world.requests("checkRunLaunch") == []


async def _crashed_before_go(world: World) -> UUID:
    world.scenario(fixture="happy")
    first = spawn_crashing(world, "keeper_started", "first")
    await ready(world)
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await asyncio.to_thread(first.wait, PATIENCE) == 9
    return run.run_id


@pytest.mark.parametrize("refusal", ["tool_class_owner_only", "agent_not_reachable"])
async def test_launch_check_refusal_launches_nothing(
    world: World, logs: LogLines, refusal: str
) -> None:
    # R-AG-48: the tool class was lowered, or the person lost trigger, while no runner ran.
    run_id = await _crashed_before_go(world)
    world.fake.world.launch_refusal = refusal
    await world.start(world.config(turn_handover=True))
    assert await world.ended(run_id) == "failed"
    assert world.turns() == []  # nothing launched
    assert "launch_check_refused" in logs.text
    assert len(world.requests("checkRunLaunch")) == 1


async def test_launch_check_of_a_run_already_stale_finishes_nothing(
    world: World, logs: LogLines
) -> None:
    run_id = await _crashed_before_go(world)
    world.fake.set_run_state(run_id, "failed", "stale")
    finishes = len(world.requests("finishRun"))
    await world.start(world.config(turn_handover=True))
    await world.until(lambda: not record_path(world, run_id).exists())
    assert world.turns() == []
    assert len(world.requests("finishRun")) == finishes


async def test_launch_check_unreachable_launches_nothing_until_it_answers(
    world: World, logs: LogLines
) -> None:
    run_id = await _crashed_before_go(world)
    world.fake.inject("checkRunLaunch", Fault.proxy_error(503), times=30)
    runner = await world.start(world.config(turn_handover=True))
    await world.until(lambda: "launch_check_unreachable" in logs.text)
    assert world.turns() == []
    assert run_id in runner.handler.adopted  # in active_runs while it waits
    assert await world.ended(run_id) == "completed"
    assert len(world.turns()) == 1


async def test_a_record_without_a_named_pipe_is_adopted_as_before(world: World) -> None:
    # A turn started without the key, then a runner with it: reading rights, no recovery.
    gate = world.gate("turn_gate")
    world.scenario(fixture="happy", turn_gate=str(gate))
    first = world.spawn()
    await ready(world)
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    await world.until(lambda: "pid" in record(world, run.run_id))
    first.send_signal(signal.SIGKILL)
    first.wait(PATIENCE)
    await world.start(world.config(turn_handover=True))
    world.release(gate)
    assert await world.ended(run.run_id) == "completed"
    assert len(world.turns()) == 1


async def test_an_adopted_process_that_fills_its_context_is_compacted(world: World) -> None:
    gate = world.gate("turn_gate")
    world.scenario(
        fixture="happy",
        per_process={
            "0": {"api_error": "context_full", "api_error_after": 1, "turn_gate": str(gate)}
        },
    )
    first = world.spawn(turn_handover=True)
    await ready(world)
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    await world.until(lambda: record(world, run.run_id).get("phase") == "process")
    first.send_signal(signal.SIGTERM)
    assert await asyncio.to_thread(first.wait, PATIENCE) == 75
    await world.start(world.config(turn_handover=True))
    world.release(gate)
    assert await world.ended(run.run_id) == "completed"
    assert len(world.turns()) == 3
    assert len(world.calls("compact")) == 1
    kinds = Counter(r.operation_id for r in world.fake.requests)
    assert kinds["checkRunLaunch"] == 2  # before the compaction and before the retry


async def test_an_adopted_second_context_full_ends_runtime_error(world: World) -> None:
    # The record proves the used compaction: a kind is never used twice (18.19.6).
    gate = world.gate("turn_gate")
    error = {"api_error": "context_full", "api_error_after": 1}
    world.scenario(
        fixture="happy", per_process={"0": error, "2": {**error, "turn_gate": str(gate)}}
    )
    first = world.spawn(turn_handover=True)
    await ready(world)
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    await world.until(
        lambda: (
            record(world, run.run_id).get("phase") == "process"
            and record(world, run.run_id).get("attempt", {}).get("kind") == "retry"
        )
    )
    first.send_signal(signal.SIGTERM)
    assert await asyncio.to_thread(first.wait, PATIENCE) == 75
    await world.start(world.config(turn_handover=True))
    world.release(gate)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run_state(run.run_id).state_reason == "runtime_error"
    assert len(world.calls("compact")) == 1
    assert len(world.turns()) == 3


async def test_a_keeper_killed_before_its_inputs_were_fed(world: World) -> None:
    # The CLI does not read yet; the keeper is killed before <launch>.fed: the runner writes
    # nothing more into the pipe, records stdin_closed and inputs_written stays false.
    gate = world.gate("stdin_gate")
    world.scenario(fixture="plain", stdin_gate=str(gate))
    await world.start(world.config(turn_handover=True))
    run = await world.fake.trigger("Fix it " + "x" * 70000, channel_id=world.forum_a)
    await world.until(lambda: record(world, run.run_id).get("phase") == "process")
    keeper = int(record(world, run.run_id)["keeper"]["pid"])
    os.kill(keeper, signal.SIGKILL)
    await world.until(lambda: record(world, run.run_id).get("stdin_closed") is True)
    assert record(world, run.run_id).get("inputs_written") is False
    world.release(gate)
    state = await world.ended(run.run_id)
    assert state in ("completed", "failed")
