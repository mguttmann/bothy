# SPDX-License-Identifier: Apache-2.0
"""Handover at a restart and adoption with full rights (09 18.19.3 to 18.19.7; AR-T02, AR-T03).

The first runner runs as a process of its own (``python -m claude_code_runner run``) with
``runner.turn_handover`` in its own process group, as under a service manager. Its turn runs under
a keeper with a named pipe as stdin. SIGTERM hands the turn over (exit 75, no heartbeat after it),
then SIGKILL goes to the runner's whole group; keeper and CLI live on in their own session. A new
runner adopts the turn: it replays the lines before the stored offset into `ProcessState`, takes
follow-ups and further inputs while the background job runs, applies the stdin rule and the stop
after an error, and handles the end of the adopted process like the end of its own. The FakeServer
clock follows the real time, because the first runner uses the system clock.
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import os
import signal
import subprocess
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.testing._patience import PATIENCE

from .conftest import LogLines, World

pytestmark = pytest.mark.anyio

HANDOVER_EXIT = 75


@pytest.fixture
def world_real_time() -> bool:
    return True


@pytest.fixture
def world_timings() -> dict[str, float]:
    # The server never ends a run on its own timers inside a scenario.
    return {"agent.cancel_grace": 100 * PATIENCE, "agent.invocation_token_ttl": 100 * PATIENCE}


def record_path(world: World, run_id: UUID) -> Path:
    return world.state / "turns" / f"{run_id}.json"


def record(world: World, run_id: UUID) -> dict[str, Any]:
    try:
        value = json.loads(record_path(world, run_id).read_text())
    except (OSError, ValueError):
        return {}
    return value if isinstance(value, dict) else {}


def alive(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    return True


def group_gone(pgid: int) -> bool:
    try:
        os.killpg(pgid, 0)
    except (ProcessLookupError, PermissionError):
        return True
    return False


def texts(world: World) -> list[str]:
    return [m["content_md"] for m in world.fake.messages]


def finishes(world: World, run_id: UUID) -> list[dict[str, Any]]:
    return [r.json for r in world.requests("finishRun") if str(run_id) in r.path]


def runner_log(world: World, index: int = 0) -> list[dict[str, Any]]:
    out = []
    for raw in (world.root / f"runner-{index}.log").read_text(errors="replace").splitlines():
        try:
            item = json.loads(raw)
        except ValueError:
            continue
        if isinstance(item, dict):
            out.append(item)
    return out


def stream_line_offset(world: World, run_id: UUID, predicate: Any) -> int:
    """The offset after the first stream line of the run that matches ``predicate``."""
    session = record(world, run_id).get("session_ref")
    path = world.state / "sessions" / str(session) / f"{run_id}.stream.jsonl"
    offset = 0
    for raw in path.read_bytes().split(b"\n"):
        offset += len(raw) + 1
        try:
            if predicate(json.loads(raw)):
                return offset
        except ValueError:
            continue
    return -1


async def ready(world: World) -> None:
    await world.until(
        lambda: any(h.get("details", {}).get("adapter") for h in world.fake.heartbeats)
    )


def is_task_list(message: dict[str, Any]) -> bool:
    return message.get("subtype") == "background_tasks_changed" and bool(message.get("tasks"))


async def job_turn(world: World, **runner: Any) -> tuple[subprocess.Popen[bytes], UUID, Path]:
    """A runner process with the key whose turn runs a background job past its checkpoint."""
    job = world.gate("background_gate")
    scenario = {"fixture": "plain", "background_gate": str(job), "background_survives_eof": True}
    scenario.update(runner.pop("scenario", {}))
    world.scenario(**scenario)
    first = world.spawn(turn_handover=True, **runner)
    await ready(world)
    run = await world.fake.trigger("Run the suite", channel_id=world.forum_a)
    await world.until(lambda: texts(world) == ["Done."])
    # The job's task list lies before the stored offset (AR-T02).
    await world.until(
        lambda: (
            0
            < stream_line_offset(world, run.run_id, is_task_list)
            <= int(record(world, run.run_id).get("offset") or 0)
        )
    )
    return first, run.run_id, job


async def hand_over(world: World, runner: subprocess.Popen[bytes]) -> int:
    """SIGTERM, then SIGKILL to the runner's whole group (as a service manager would)."""
    runner.send_signal(signal.SIGTERM)
    code = await asyncio.to_thread(runner.wait, PATIENCE)
    with contextlib.suppress(ProcessLookupError, PermissionError):
        os.killpg(runner.pid, signal.SIGKILL)
    return code


async def test_sigterm_hands_the_turn_over_and_the_next_runner_adopts_it(
    world: World, logs: LogLines
) -> None:
    first, run_id, job = await job_turn(world, followup_grace="300ms")
    [turn] = world.turns()
    cli, keeper = turn["pid"], turn["pgid"]
    assert keeper not in (cli, first.pid)
    beats = len(world.fake.heartbeats)

    assert await hand_over(world, first) == HANDOVER_EXIT

    assert len(world.fake.heartbeats) == beats  # no heartbeat after the handover began
    handed = [e for e in runner_log(world) if e.get("event") == "turn_handed_over"]
    assert [e.get("run_id") for e in handed] == [str(run_id)]
    assert alive(cli)  # outside the runner's group, untouched by SIGKILL
    assert alive(keeper)
    assert world.calls("stdin_eof") == []
    assert world.state_of(run_id) == "running"
    start = record(world, run_id)["process_start"]

    seen = len(world.fake.heartbeats)
    runner = await world.start(world.config(turn_handover=True, followup_grace="300ms"))
    await world.until(lambda: len(world.fake.heartbeats) > seen)
    first_beat = world.fake.heartbeats[seen]
    assert str(run_id) in first_beat["details"]["active_runs"]
    assert first_beat["details"]["capabilities"]["turn_handover"] is True

    # Full rights: a follow-up while the job runs is written, echoed and answered, and its
    # result leaves stdin open (the replayed task list holds it).
    message_id = await world.fake.deliver_followup(run_id, "What is the state?", author="alex")
    await world.until(lambda: len(texts(world)) == 2)
    assert texts(world)[1] == "Status: the background job is still running."
    await world.until(lambda: world.fake.followup_state(run_id, message_id) == "absorbed")
    # A further input written like a follow-up (the path of a wake input, 18.19.7).
    turn_object = runner.handler.adopted[run_id]
    assert await turn_object.offer_input("Wake up and check the suite.", autonomous=True)
    await world.until(lambda: len(texts(world)) == 3)
    await asyncio.sleep(1.0)  # longer than runner.followup_grace: the job still holds stdin
    assert world.calls("stdin_eof") == []
    assert world.calls("background_killed") == []
    assert record(world, run_id)["process_start"] == start

    world.release(job)
    assert await world.ended(run_id) == "completed"
    assert texts(world) == [
        "Done.",
        "Status: the background job is still running.",
        "Status: the background job is still running.",
        "The suite passed.",
    ]  # nothing replayed is posted again
    assert len(world.turns()) == 1
    assert len(world.requests("startRun")) == 1
    assert len(finishes(world, run_id)) == 1
    adopted = [r.getMessage() for r in logs.records if "turn_adopted" in r.getMessage()]
    assert adopted
    await world.until(lambda: not record_path(world, run_id).exists())
    await world.until(lambda: group_gone(keeper))


async def test_two_restarts_in_a_row_keep_process_start(world: World) -> None:
    # Variant (a): the adopter hands the turn over again; process_start never moves.
    first, run_id, job = await job_turn(world, followup_grace="300ms")
    start = record(world, run_id)["process_start"]
    assert await hand_over(world, first) == HANDOVER_EXIT
    second = world.spawn(turn_handover=True, followup_grace="300ms")
    await world.until(lambda: "turn_adopted" in _log_text(world, 1))
    assert await hand_over(world, second) == HANDOVER_EXIT
    assert record(world, run_id)["process_start"] == start
    await world.start(world.config(turn_handover=True, followup_grace="300ms"))
    world.release(job)
    assert await world.ended(run_id) == "completed"
    assert texts(world) == ["Done.", "The suite passed."]
    assert len(world.turns()) == 1


def _log_text(world: World, index: int) -> str:
    path = world.root / f"runner-{index}.log"
    return path.read_text(errors="replace") if path.exists() else ""


async def test_restart_inside_the_quiet_window_closes_stdin_after_the_grace(world: World) -> None:
    # Variant (b): the job ended and its notification turn was answered; the restart comes
    # inside the quiet window with no later input: stdin closes runner.followup_grace after
    # the adoption, and the run completes.
    first, run_id, job = await job_turn(world, followup_grace="3s")
    world.release(job)
    await world.until(lambda: texts(world) == ["Done.", "The suite passed."])
    assert await hand_over(world, first) == HANDOVER_EXIT
    loop = asyncio.get_running_loop()
    adopted_at = loop.time()
    await world.start(world.config(turn_handover=True, followup_grace="3s"))
    assert await world.ended(run_id) == "completed"
    assert loop.time() - adopted_at >= 3.0  # held for the whole grace after the adoption
    assert texts(world) == ["Done.", "The suite passed."]
    assert len(world.calls("stdin_eof")) == 1


async def test_error_result_before_the_checkpoint_stops_after_the_grace(
    world: World, logs: LogLines
) -> None:
    # Variant (c): an error result while the job runs, before the checkpoint: the adopter stops
    # the group runner.stop_grace after the adoption and logs the lost task.
    error_gate = world.gate("background_error_gate")
    first, run_id, _ = await job_turn(
        world,
        followup_grace="300ms",
        stop_grace="3s",
        scenario={
            "background_error_gate": str(error_gate),
            "background_api_error": "not_authenticated",
        },
    )
    world.release(error_gate)
    await world.until(lambda: record(world, run_id).get("stdin_closed") is True)
    assert await hand_over(world, first) == HANDOVER_EXIT
    [turn] = world.turns()
    loop = asyncio.get_running_loop()
    adopted_at = loop.time()
    await world.start(world.config(turn_handover=True, stop_grace="3s", followup_grace="300ms"))
    await world.until(lambda: group_gone(turn["pgid"]))
    assert loop.time() - adopted_at >= 3.0
    stopped = [r.getMessage() for r in logs.records if "turn_stopped_after_error" in r.getMessage()]
    assert stopped
    assert '"background_tasks": "1"' in stopped[0]
    assert await world.ended(run_id) == "failed"


async def test_a_turn_with_stdin_closed_is_adopted_with_reading_rights_only(
    world: World,
) -> None:
    # Variant (d): stdin was closed before the handover: follow-ups stay queued.
    error_gate = world.gate("background_error_gate")
    first, run_id, _ = await job_turn(
        world,
        followup_grace="300ms",
        stop_grace="30s",
        scenario={
            "background_error_gate": str(error_gate),
            "background_api_error": "not_authenticated",
        },
    )
    world.release(error_gate)
    await world.until(lambda: record(world, run_id).get("stdin_closed") is True)
    assert await hand_over(world, first) == HANDOVER_EXIT
    runner = await world.start(world.config(turn_handover=True, stop_grace="30s"))
    await world.until(lambda: run_id in runner.handler.adopted)
    message_id = await world.fake.deliver_followup(run_id, "Anything?", author="alex")
    await asyncio.sleep(1.0)
    assert world.fake.followup_state(run_id, message_id) == "queued"
    assert all("Anything?" not in json.dumps(c) for c in world.calls("stdin"))
    assert runner.handler.adopted_run(run_id) is None
    await world.fake.cancel(run_id, "user_cancelled")  # ends the turn before the teardown
    assert await world.ended(run_id) == "cancelled"


# ---- AR-T03 -------------------------------------------------------------------------------------


async def test_runner_killed_the_turn_is_adopted_with_full_rights(world: World) -> None:
    first, run_id, job = await job_turn(world, followup_grace="300ms")
    os.killpg(first.pid, signal.SIGKILL)  # a crash, not a handover
    first.wait(PATIENCE)
    await world.start(world.config(turn_handover=True, followup_grace="300ms"))
    message_id = await world.fake.deliver_followup(run_id, "Still there?", author="alex")
    await world.until(lambda: world.fake.followup_state(run_id, message_id) == "absorbed")
    world.release(job)
    assert await world.ended(run_id) == "completed"
    assert len(world.turns()) == 1


async def test_cli_exit_code_comes_from_the_keeper_in_the_starting_runner(world: World) -> None:
    # The keeper exits with the CLI's code; <launch>.exit carries the wait status, and
    # exit_label and the classification equal today's (the box note names the code).
    world.scenario(fixture="error_result", exit_code=2)
    await world.start(world.config(turn_handover=True))
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "failed"
    note = _runtime_error_note(world, run.run_id)
    assert note["detail"]["exit"] == "2"


def _runtime_error_note(world: World, run_id: UUID) -> dict[str, Any]:
    steps = world.fake.run(run_id)["steps"].values()
    [note] = [s for s in steps if s.get("name") == "runtime_error"]
    return dict(note)


async def test_cli_exit_code_comes_from_the_exit_file_in_an_adopting_runner(world: World) -> None:
    gate = world.gate("turn_gate")
    world.scenario(fixture="error_result", exit_code=2, turn_gate=str(gate))
    first = world.spawn(turn_handover=True)
    await ready(world)
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    await world.until(lambda: record(world, run.run_id).get("phase") == "process")
    assert await hand_over(world, first) == HANDOVER_EXIT
    await world.start(world.config(turn_handover=True))
    world.release(gate)
    assert await world.ended(run.run_id) == "failed"
    assert _runtime_error_note(world, run.run_id)["detail"]["exit"] == "2"


async def test_keeper_killed_the_cli_sees_end_of_input_and_the_run_ends(world: World) -> None:
    # A SIGKILL to the keeper alone closes its write end; without exit file the label is
    # unknown and the classification uses the stream alone.
    job = world.gate("background_gate")
    world.scenario(fixture="plain", background_gate=str(job))
    await world.start(world.config(turn_handover=True, followup_grace="300ms"))
    run = await world.fake.trigger("Run the suite", channel_id=world.forum_a)
    await world.until(lambda: texts(world) == ["Done."])
    [turn] = world.turns()
    os.kill(turn["pgid"], signal.SIGKILL)
    await world.until(lambda: world.calls("background_killed") != [])
    state = await world.ended(run.run_id)
    assert state in ("completed", "failed")
    await world.until(lambda: group_gone(turn["pgid"]))


async def test_stop_of_a_run_leaves_no_process_of_the_group(world: World) -> None:
    # 18.6 with the key: the stop signals the one group of keeper and CLI.
    gate = world.gate("turn_gate")
    world.scenario(fixture="happy", turn_gate=str(gate), ignore_sigint=True, spawn_child=True)
    runner = await world.start(world.config(turn_handover=True))
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    [turn] = world.turns()
    await world.fake.cancel(run.run_id, "user_cancelled")
    assert await world.ended(run.run_id) == "cancelled"
    await world.until(lambda: group_gone(turn["pgid"]))
    assert not alive(int(world.calls("child")[0]["child_pid"]))
    assert run.run_id not in runner.handler.adopted


async def test_stop_turns_ends_every_turn_and_exits_zero(world: World) -> None:
    # claude-code-runner stop --turns: every turn stopped as in 18.6, its run cancelled with
    # adapter_stopped, exit 0; then nothing is left to adopt.
    first, run_id, _ = await job_turn(world, followup_grace="300ms")
    [turn] = world.turns()
    # Off the event loop: the FakeServer answers both processes meanwhile.
    command = await asyncio.to_thread(
        subprocess.run,
        [*world.cli_command(), "stop", "--turns", "--config", str(world.root / "runner.toml")],
        env=world.cli_env(),
        capture_output=True,
        timeout=60,
        check=False,
    )
    assert command.returncode == 0, command.stderr
    assert first.wait(PATIENCE) == 0
    assert group_gone(turn["pgid"])
    [finish] = finishes(world, run_id)
    assert (finish["state"], finish["reason"]) == ("cancelled", "adapter_stopped")
    assert not record_path(world, run_id).exists()


async def test_stop_turns_without_a_running_runner_stops_the_handed_over_turn(
    world: World,
) -> None:
    # The service was unloaded after a handover: stop --turns ends the turn itself.
    first, run_id, _ = await job_turn(world, followup_grace="300ms")
    [turn] = world.turns()
    assert await hand_over(world, first) == HANDOVER_EXIT
    command = await asyncio.to_thread(
        subprocess.run,
        [*world.cli_command(), "stop", "--turns", "--config", str(world.root / "runner.toml")],
        env=world.cli_env(),
        capture_output=True,
        timeout=PATIENCE,
        check=False,
    )
    assert command.returncode == 0, command.stderr
    assert group_gone(turn["pgid"])
    [finish] = finishes(world, run_id)
    assert (finish["state"], finish["reason"]) == ("cancelled", "adapter_stopped")
    assert not record_path(world, run_id).exists()


async def test_key_switched_off_with_a_handed_over_turn_refuses_the_start_until_stop_turns(
    world: World,
) -> None:
    # Issue 567 item 3: runner.turn_handover goes off only after stop --turns or while no turn
    # runs. A runner started without the key while a record written with it is left refuses to
    # start (exit 1, start_failed turn_handover_records_left) and touches nothing; stop --turns
    # with the key off still stops the handed-over turn through its keeper and finishes its run.
    first, run_id, _ = await job_turn(world, followup_grace="300ms")
    [turn] = world.turns()
    assert await hand_over(world, first) == HANDOVER_EXIT
    starts = len(world.requests("startRun"))

    second = world.spawn()  # the key off
    assert await asyncio.to_thread(second.wait, PATIENCE) == 1
    failed = [e for e in runner_log(world, 1) if e.get("event") == "start_failed"]
    assert [e.get("code") for e in failed] == ["turn_handover_records_left"]
    assert alive(turn["pid"])  # the turn is untouched
    assert record(world, run_id).get("phase") == "process"
    assert world.state_of(run_id) == "running"
    assert len(world.requests("startRun")) == starts

    command = await asyncio.to_thread(
        subprocess.run,
        [*world.cli_command(), "stop", "--turns", "--config", str(world.root / "runner.toml")],
        env=world.cli_env(),
        capture_output=True,
        timeout=PATIENCE,
        check=False,
    )
    assert command.returncode == 0, command.stderr
    assert group_gone(turn["pgid"])
    [finish] = finishes(world, run_id)
    assert (finish["state"], finish["reason"]) == ("cancelled", "adapter_stopped")
    assert not record_path(world, run_id).exists()

    # Nothing is left: a runner without the key starts.
    await world.start(world.config())
