# SPDX-License-Identifier: Apache-2.0
"""Runs as Claude Code background sessions (09 18.2 "Background mode", FEAT-RUNNER-BG-SESSIONS).

With ``runner.launch_mode`` ``background`` a ``personal`` run of the runner's own agent starts
one background session (``claude --bg``) in the project folder, follows its transcript and the
session list, steers it through its inbox socket, and stops and removes it at the end of the
run. Fake-Claude plays the session (``tests/fake_claude/fake_bg.py``): it writes the transcript
records of the pinned CLI, among them those that hold account identifiers, and answers the
control commands from a state file in the fake ``HOME``.

Each scenario here has its print-mode twin in the scenario file of its topic (sessions,
follow-ups, voice, answer attachments, stop, background work).
"""

from __future__ import annotations

import asyncio
import json
import os
import sys
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.testing._patience import PATIENCE

from claude_code_runner.app import Runner
from claude_code_runner.bgpaths import bg_files
from claude_code_runner.bgtranscript import READ_FIELDS
from claude_code_runner.causes import ADAPTER_STOPPED, RUN_ENDED_IDLE, cause
from claude_code_runner.heartbeat import PINNED_CLAUDE_CODE_VERSION

from .conftest import API_KEY, BACKGROUND, LogLines, World, _group_alive, live_files

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "fake_claude"))
import fake_bg

pytestmark = pytest.mark.anyio


@pytest.fixture
def world_timings() -> dict[str, float]:
    # Tokens and cancellations never expire on the server's timers inside a scenario.
    return {"agent.cancel_grace": 100 * PATIENCE, "agent.invocation_token_ttl": 100 * PATIENCE}


def launches(world: World) -> list[dict[str, Any]]:
    return world.calls("bg_launch")


def players(world: World) -> list[dict[str, Any]]:
    return world.calls("bg_player")


def texts(world: World) -> list[str]:
    return [m["content_md"] for m in world.fake.messages]


def finishes(world: World, run_id: UUID) -> list[dict[str, Any]]:
    return [r.json for r in world.requests("finishRun") if str(run_id) in r.path]


def flag(argv: list[str], name: str) -> str:
    return str(argv[argv.index(name) + 1])


def transcript(world: World, player: dict[str, Any]) -> list[dict[str, Any]]:
    """The records Fake-Claude wrote for one session, in order."""
    folder = "".join(c if c.isalnum() else "-" for c in player["cwd"])
    path = world.home / ".claude" / "projects" / folder / f"{player['session_id']}.jsonl"
    return [json.loads(x) for x in path.read_text().splitlines()]


def events(logs: LogLines, name: str) -> list[dict[str, Any]]:
    """The runner's own log lines with the event ``name``, as objects."""
    found = []
    for record in logs.records:
        if record.name != "claude_code_runner":
            continue
        try:
            item = json.loads(record.getMessage())
        except ValueError:
            continue
        if isinstance(item, dict) and item.get("event") == name:
            found.append(item)
    return found


def followup_states(world: World, message_id: UUID) -> list[str]:
    return [
        op["value"]
        for r in world.requests("patchRun")
        for op in r.json.get("ops", [])
        if op["path"] == f"/followups/{message_id}/state"
    ]


async def start(world: World, **runner: Any) -> Runner:
    return await world.start(world.config(**{**BACKGROUND, **runner}))


def _pid_alive(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    return True


async def nothing_left(world: World) -> None:
    """No session process, no entry and no file of a background turn is left.

    The runner removes its record and files right after ``finishRun``, so each is waited for.
    """
    await world.until(lambda: not any(_group_alive(p["pgid"]) for p in players(world)))

    def entries() -> dict[str, Any]:
        path = world.home / ".claude" / "fake-bg" / "state.json"
        sessions: dict[str, Any] = json.loads(path.read_text())["sessions"]
        return sessions

    def files() -> list[str]:
        folder = world.state / "bg"
        return [p.name for p in folder.iterdir() if p.name != "supervisor.json"]

    await world.until(lambda: entries() == {})
    await world.until(lambda: files() == [])
    await world.until(lambda: list((world.state / "turns").glob("*.json")) == [])


# ---- a new post and its second message --------------------------------------------------------


async def test_a_new_post_runs_as_one_background_session(bg_world: World) -> None:
    world = bg_world
    await start(world)
    run = await world.fake.trigger("Fix the login form", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "completed"
    assert world.turns() == []  # no print turn
    [launch] = launches(world)
    [player] = players(world)
    files = bg_files(world.state, run.run_id)
    # The launch: one command in the project folder with exactly this argument list (18.2).
    assert launch["cwd"] == str((world.projects / "website").resolve())
    assert launch["argv"] == [
        "--bg",
        "--name",
        f"bothy-{run.run_id}",
        "--model",
        "model-large",
        "--permission-mode",
        "dontAsk",
        "--allowedTools",
        "Read",
        "Bash(git status)",
        "--disallowedTools",
        "AskUserQuestion",
        "SendMessage",
        "ListAgents",
        "SendFile",
        "--append-system-prompt-file",
        str(files.system_prompt),
        "--messaging-socket-path",
        str(files.socket),
        "--settings",
        str(files.settings),
    ]
    tmpdir = str(world.state / "tmp" / str(run.run_id))
    # No crossSessionInbound while the host user's settings have none: what the session
    # takes from other sessions is not widened.
    assert launch["settings"] == {
        "worktree": {"bgIsolation": "none"},
        "env": {
            "CLAUDE_PRODUCT_RUN": "1",
            "DISABLE_AUTOUPDATER": "1",
            "TMPDIR": tmpdir,
            "SSH_AUTH_SOCK": "",
        },
    }
    # The prompt is the stdin of the command: the context, a blank line, the trigger part.
    assert launch["prompt"].endswith("Fix the login form")
    assert "\n\n" in launch["prompt"]
    assert not launch["prompt"].startswith(("/", "!"))
    # The launch environment is the fixed one of the runner: the allow list of 18.2 plus
    # DISABLE_AUTOUPDATER, never a key and never a value of the run. The launch starts the
    # supervisor, which later sessions inherit from; the run's own temporary folder reaches
    # the session through the settings file alone.
    assert launch["env"]["DISABLE_AUTOUPDATER"] == "1"
    assert launch["env"]["CLAUDE_PRODUCT_RUN"] == "1"
    assert "ANTHROPIC_API_KEY" not in launch["env"]
    assert API_KEY not in json.dumps(launch)
    assert tmpdir not in json.dumps(launch["env"])
    assert "SSH_AUTH_SOCK" not in launch["env"]
    assert player["env"]["TMPDIR"] == tmpdir
    assert player["env"]["SSH_AUTH_SOCK"] == ""
    # A follow-up reaches a background session as a relayed message: the prompt says so.
    assert "runs as a background session" in launch["system_prompt"]
    assert "TITLE:" in launch["system_prompt"]
    # The run: the session's answer, the live box, the costs and the session the CLI made.
    assert texts(world) == ["The form validates the email twice; I removed the second check."]
    doc = world.fake.run(run.run_id)
    assert doc["model"] == "model-large"
    assert doc["last_text_excerpt"] == texts(world)[0]
    steps = [(s["kind"], s["name"], s["title"], s["status"]) for s in doc["steps"].values()]
    assert steps == [("tool_call", "Bash", "git status", "completed")]
    [finish] = finishes(world, run.run_id)
    assert finish["summary"] == cause(RUN_ENDED_IDLE)
    assert finish["costs"] == [
        {
            "provider": "anthropic",
            "model": "model-large",
            "currency": "USD",
            "tokens_in": 2700,
            "tokens_out": 100,
            "tokens_cache_read": 600,
            "tokens_cache_write": 500,
            "amount_micros": 12500,
        }
    ]
    assert doc["cost"]["amount_micros"] == 12500
    [started] = world.requests("startRun")
    [report] = world.requests("reportAgentSession")
    assert report.json["external_session_ref"] == player["session_id"]
    assert report.json["started"] is True
    assert started.json["external_session_ref"] != player["session_id"]  # the runner's own UUID
    assert world.fake.world.thread_meta[run.thread_id]["title"] == "Fix the login form"
    # The release: stop, then remove; nothing of the session is left.
    assert [c["id"] for c in world.calls("bg_stop")] == [player["short"]]
    assert [c["id"] for c in world.calls("bg_rm")] == [player["short"]]
    assert world.calls("bg_player_end")[0]["reason"] == "stop"
    await nothing_left(world)


async def test_the_next_message_continues_the_same_session(bg_world: World) -> None:
    world = bg_world
    await start(world)
    first = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(first.run_id) == "completed"
    world.scenario(fixture="resume")
    second = await world.fake.trigger("Add a test", thread_id=first.thread_id)
    assert await world.ended(second.run_id) == "completed"
    one, two = launches(world)
    session = players(world)[0]["session_id"]
    assert "--resume" not in one["argv"]
    assert two["argv"][:3] == ["--resume", session, "--bg"]
    # The same session under the same id: the entry was removed, so no copy was started.
    assert [p["session_id"] for p in players(world)] == [session, session]
    assert len(world.requests("reportAgentSession")) == 1
    assert "TITLE:" not in two["system_prompt"]
    assert texts(world)[-1] == "Added a test for the form."
    # The costs of the second run are the difference of the session's totals.
    [finish] = finishes(world, second.run_id)
    assert finish["costs"] == [
        {
            "provider": "anthropic",
            "model": "model-large",
            "currency": "USD",
            "tokens_in": 3000,
            "tokens_out": 30,
            "tokens_cache_read": 2000,
            "tokens_cache_write": 100,
            "amount_micros": 7500,
        }
    ]
    await nothing_left(world)


# ---- two runs at once ---------------------------------------------------------------------------


async def test_two_runs_that_start_together_both_run_as_background_sessions(
    bg_world: World, logs: LogLines
) -> None:
    world = bg_world
    world.scenario(fixture="plain")
    await start(world)
    # No supervisor runs yet: the first launch starts it, and the second run must not take it
    # for one of another program, whatever the order of the two runs' commands is.
    runs = await asyncio.gather(
        world.fake.trigger("Fix it", channel_id=world.forum_a),
        world.fake.trigger("Fix that", channel_id=world.forum_a),
    )
    assert [await world.ended(r.run_id) for r in runs] == ["completed", "completed"]
    assert [world.fake.run_state(r.run_id).state_reason for r in runs] == [None, None]
    assert world.turns() == []  # no fallback: neither prompt ran in print mode
    assert len(launches(world)) == len(players(world)) == 2
    assert events(logs, "background_fallback") == []
    assert events(logs, "background_launch_in_doubt") == []
    assert not any(
        "background_unavailable" in h.get("details", {}).get("degraded_reasons", [])
        for h in world.fake.heartbeats
    )
    # One launch at a time: the second run reads the supervisor only after the first run's
    # launch started it and the runner recorded it (check, launch, record; check, launch).
    commands = sorted(world.calls("bg_daemon") + launches(world), key=lambda c: c["t"])
    order = [c["kind"] for c in commands]
    assert order[:5] == ["bg_daemon", "bg_launch", "bg_daemon", "bg_daemon", "bg_launch"]
    await nothing_left(world)


# ---- follow-ups ---------------------------------------------------------------------------------


async def test_a_followup_during_a_tool_call_is_absorbed_in_the_same_turn(bg_world: World) -> None:
    world = bg_world
    gate = world.gate("turn_gate")
    world.scenario(fixture="happy", turn_gate=str(gate), gate_after_lines=1)
    await start(world)
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)  # the tool call runs
    message_id = await world.fake.deliver_followup(run.run_id, "Also the footer", author="sam")
    await world.until(lambda: world.fake.followup_state(run.run_id, message_id) == "delivered")
    [line] = world.calls("bg_socket")
    assert line["text"] == "Follow-up from Sam (@sam):\n\nAlso the footer"
    UUID(line["uuid"])
    assert world.state_of(run.run_id) == "running"
    world.release(gate)
    assert await world.ended(run.run_id) == "completed"
    assert followup_states(world, message_id) == ["delivered", "absorbed"]
    # One turn, one answer, and the answer was produced after the model received the line.
    assert texts(world) == ["The form validates the email twice; I removed the second check."]
    records = transcript(world, players(world)[0])
    taken = next(
        i
        for i, r in enumerate(records)
        if r.get("reason") == "absorbed_mid_turn" and r.get("commandUuid") == line["uuid"]
    )
    answer = max(i for i, r in enumerate(records) if r["type"] == "assistant")
    assert taken < answer
    assert sum(1 for r in records if r.get("subtype") == "turn_duration") == 1
    # The attachment that names whose input the turn took has the members of the pinned CLI.
    assert QUEUED_ATTACHMENT in members_seen(world)
    await nothing_left(world)


async def test_a_followup_after_the_last_tool_call_gets_a_turn_of_its_own(bg_world: World) -> None:
    world = bg_world
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate))
    await start(world)
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    message_id = await world.fake.deliver_followup(run.run_id, "One more", author="sam")
    await world.until(lambda: world.fake.followup_state(run.run_id, message_id) == "delivered")
    world.release(gate)
    assert await world.ended(run.run_id) == "completed"
    assert world.fake.followup_state(run.run_id, message_id) == "absorbed"
    assert texts(world) == ["Done.", "Follow-up 1 handled."]
    [finish] = finishes(world, run.run_id)
    assert [c["tokens_in"] for c in finish["costs"]] == [1100]  # the totals after both turns
    assert len(players(world)) == 1


def _transcriber(world: World) -> str:
    path = world.root / "transcribe"
    path.write_text(
        f"#!{sys.executable}\nimport pathlib, sys\n"
        "data = pathlib.Path(sys.argv[-1]).read_bytes()\n"
        "print('VOICE-CANARY please run the tests', data.decode())\n"
    )
    path.chmod(0o755)
    return str(path)


async def test_followups_with_an_attachment_and_a_voice_message(bg_world: World) -> None:
    world = bg_world
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate))
    await start(world, transcribe_cmd=[_transcriber(world)])
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    thread = world.fake.world.runs[run.run_id].thread_id
    notes = world.fake.add_file(b"NOTES", name="notes.txt", mime="text/plain")
    await world.fake.deliver_followup(run.run_id, "See the notes", author="sam", files=(notes,))
    await world.until(lambda: len(world.calls("bg_socket")) == 1)
    voice = world.fake.add_file(b"AUDIO-BYTES", name="memo.ogg", mime="audio/ogg")
    await world.fake.deliver_followup(run.run_id, "", author="sam", files=(voice,))
    await world.until(lambda: len(world.calls("bg_socket")) == 2)
    first, second = (c["text"] for c in world.calls("bg_socket"))
    folder = world.state / "files" / str(thread)
    assert first == (
        "Follow-up from Sam (@sam):\n\nSee the notes\n\n"
        f"Attachments:\n- notes.txt (text/plain): local file: {folder / str(notes)}"
    )
    assert second == (
        "Follow-up from Sam (@sam):\n\n"
        'Transcript of attachment "memo.ogg":\n'
        "VOICE-CANARY please run the tests AUDIO-BYTES\n\n"
        f"Attachments:\n- memo.ogg (audio/ogg): local file: {folder / str(voice)}"
    )
    world.release(gate)
    assert await world.ended(run.run_id) == "completed"
    assert texts(world) == ["Done.", "Follow-up 1 handled.", "Follow-up 2 handled."]


async def test_an_answer_attachment_is_sent_from_the_run_s_temporary_folder(
    bg_world: World,
) -> None:
    world = bg_world
    world.fake.uploads.limits["message_max_attachments"] = 3
    gate = world.gate("turn_gate")
    answer = "Here it is.\nATTACH: {{tmpdir}}/summary.txt"
    world.scenario(fixture="plain", turn_gate=str(gate), result_override={"result": answer})
    await start(world)
    run = await world.fake.trigger("Send me the summary", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    # TMPDIR of the run reaches the session through the env block of --settings.
    tmpdir = Path(players(world)[0]["env"]["TMPDIR"])
    assert tmpdir == world.state / "tmp" / str(run.run_id)
    (tmpdir / "summary.txt").write_bytes(b"SUMMARY-BYTES")
    world.release(gate)
    assert await world.ended(run.run_id) == "completed"
    [message] = world.fake.messages
    assert message["content_md"] == "Here it is."
    assert len(message["attachment_file_ids"]) == 1
    assert "ATTACH" not in world.fake.run(run.run_id)["last_text_excerpt"]


# ---- stop ---------------------------------------------------------------------------------------


async def test_a_stop_during_a_tool_call_ends_the_run_without_an_answer(
    bg_world: World, logs: LogLines
) -> None:
    world = bg_world
    gate = world.gate("turn_gate")
    world.scenario(fixture="happy", turn_gate=str(gate), gate_after_lines=1)
    await start(world)
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    await world.fake.cancel(run.run_id, "user_cancelled")
    assert await world.ended(run.run_id) == "cancelled"
    assert world.fake.run_state(run.run_id).state_reason == "user_cancelled"
    assert texts(world) == []
    assert world.calls("bg_player_end")[0]["reason"] == "stop"
    assert events(logs, "stop_escalated") == []
    [finish] = finishes(world, run.run_id)
    assert [c["tokens_in"] for c in finish["costs"]] == [50]  # the totals the stop wrote
    await nothing_left(world)


async def test_a_stop_escalates_to_the_process_group_of_the_session(
    bg_world: World, logs: LogLines
) -> None:
    world = bg_world
    gate = world.gate("turn_gate")
    world.scenario(
        fixture="happy",
        turn_gate=str(gate),
        gate_after_lines=1,
        bg_ignore_stop=True,
        ignore_sigterm=True,
    )
    await start(world, stop_grace="200ms", kill_grace="200ms")
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    await world.fake.cancel(run.run_id, "user_cancelled")
    assert await world.ended(run.run_id) == "cancelled"
    assert [e["signal"] for e in events(logs, "stop_escalated")] == ["SIGTERM", "SIGKILL"]
    # A process killed hard writes no totals: the run books what its messages reported.
    [finish] = finishes(world, run.run_id)
    assert [(c["tokens_in"], c["tokens_out"]) for c in finish["costs"]] == [(1200, 40)]
    assert len(events(logs, "costs_priced_without_totals")) == 1
    await world.until(lambda: not _group_alive(players(world)[0]["pgid"]))


async def test_an_orderly_stop_of_the_runner_stops_the_session(bg_world: World) -> None:
    world = bg_world
    gate = world.gate("turn_gate")
    world.scenario(fixture="happy", turn_gate=str(gate), gate_after_lines=1)
    runner = await start(world)
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    await world.stop(runner)
    assert world.state_of(run.run_id) == "cancelled"
    assert world.fake.run_state(run.run_id).state_reason == ADAPTER_STOPPED
    await nothing_left(world)


# ---- background work ----------------------------------------------------------------------------


async def test_a_background_command_a_followup_and_the_notification(bg_world: World) -> None:
    world = bg_world
    job = world.gate("background_gate")
    world.scenario(fixture="plain", background_gate=str(job))
    await start(world, followup_grace="200ms")
    run = await world.fake.trigger("Run the suite", channel_id=world.forum_a)
    await world.until(lambda: texts(world) == ["Done."])  # the turn ended
    # The session list says busy without a turn: the run stays running and shows the work.
    await world.until(
        lambda: world.fake.run(run.run_id).get("metrics", {}).get("background_jobs") == 1
    )
    assert world.state_of(run.run_id) == "running"
    message_id = await world.fake.deliver_followup(run.run_id, "What is the state?", author="alex")
    await world.until(lambda: len(texts(world)) == 2)
    assert texts(world)[1] == "Status: the background job is still running."
    assert world.fake.followup_state(run.run_id, message_id) == "absorbed"
    assert world.state_of(run.run_id) == "running"
    world.release(job)
    assert await world.ended(run.run_id) == "completed"
    assert texts(world)[2] == "The suite passed."
    assert world.fake.run(run.run_id)["metrics"]["background_jobs"] == 0
    assert len(players(world)) == 1  # one process for the whole conversation
    assert [c["reason"] for c in world.calls("bg_player_end")] == ["stop"]
    await nothing_left(world)


async def test_a_stop_during_background_work_names_it_without_a_count(bg_world: World) -> None:
    world = bg_world
    job = world.gate("background_gate")
    world.scenario(fixture="plain", background_gate=str(job))
    runner = await start(world)
    run = await world.fake.trigger("Run the suite", channel_id=world.forum_a)
    await world.until(
        lambda: world.fake.run(run.run_id).get("metrics", {}).get("background_jobs") == 1
    )
    await world.stop(runner)
    summary = world.fake.run(run.run_id)["summary"]
    assert "Background work was still running and was stopped" in summary
    assert summary.endswith(f"(`{ADAPTER_STOPPED}`)")


# ---- what a run cannot go on with ---------------------------------------------------------------


async def test_a_session_that_waits_for_a_person_is_stopped(bg_world: World) -> None:
    world = bg_world
    world.scenario(fixture="plain", bg_waiting="input needed")
    await start(world)
    run = await world.fake.trigger("Ask me", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run_state(run.run_id).state_reason == "runtime_error"
    [note] = [s for s in world.fake.run(run.run_id)["steps"].values() if s["kind"] == "status"]
    assert note["detail"]["errors"] == [
        "The background session waited for a question and was stopped."
    ]
    assert world.fake.run(run.run_id)["summary"] == cause("background_session_waiting")
    assert texts(world) == []
    await nothing_left(world)


async def test_a_copy_at_resume_is_stopped_and_fails_the_run(bg_world: World) -> None:
    world = bg_world
    await start(world)
    first = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(first.run_id) == "completed"
    world.scenario(fixture="resume", bg_copy=True)
    second = await world.fake.trigger("Add a test", thread_id=first.thread_id)
    assert await world.ended(second.run_id) == "failed"
    assert world.fake.run_state(second.run_id).state_reason == "runtime_error"
    assert world.fake.run(second.run_id)["summary"] == cause("background_session_copied")
    assert len(texts(world)) == 1  # the copy answered nothing
    assert world.turns() == []  # and the prompt never ran in print mode either
    await nothing_left(world)


async def test_a_process_the_runner_did_not_launch_is_stopped(bg_world: World) -> None:
    world = bg_world
    gate = world.gate("bg_replace_gate")
    job = world.gate("background_gate")
    world.scenario(fixture="plain", background_gate=str(job), bg_replace_gate=str(gate))
    await start(world, stop_grace="200ms", kill_grace="200ms")
    run = await world.fake.trigger("Run the suite", channel_id=world.forum_a)
    await world.until(lambda: texts(world) == ["Done."])
    world.release(gate)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run(run.run_id)["summary"] == cause("background_process_replaced")
    other = world.calls("bg_replaced")[0]["other"]
    await world.until(lambda: not _alive(other))
    await world.until(lambda: not _group_alive(players(world)[0]["pgid"]))


def _alive(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    return True


async def test_a_request_without_the_stored_login_ends_the_run(bg_world: World) -> None:
    # The counter-check of 18.3: a personal turn uses the stored login.
    world = bg_world
    world.scenario(fixture="plain", bg_no_login=True)
    await start(world)
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run_state(run.run_id).state_reason == "billing_mode_mismatch"
    assert texts(world) == []
    await nothing_left(world)


# ---- what never leaves a transcript -------------------------------------------------------------


async def test_no_account_identifier_leaves_the_transcript(bg_world: World, logs: LogLines) -> None:
    world = bg_world
    gate = world.gate("turn_gate")
    world.scenario(fixture="happy", turn_gate=str(gate), gate_after_lines=1)
    await start(world)
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    canary = fake_bg.ACCOUNT_CANARY.encode()
    records = transcript(world, players(world)[0])
    assert any(fake_bg.ACCOUNT_CANARY in json.dumps(r) for r in records)  # it is in there
    held = live_files(p for p in world.state.rglob("*") if p.is_file())
    assert held
    assert all(canary not in data for _, data in held)
    world.release(gate)
    assert await world.ended(run.run_id) == "completed"
    left = live_files(p for p in world.state.rglob("*") if p.is_file())
    assert all(canary not in data for _, data in left)
    assert fake_bg.ACCOUNT_CANARY not in logs.text
    assert fake_bg.ACCOUNT_CANARY not in json.dumps([r.json for r in world.fake.requests])


# ---- account errors (09 18.9, the scenarios of FIX-RUNNER-LIMIT-KEEPS-RUN) ----------------------

NEVER = f"{int(100 * PATIENCE)}s"
FAST = {
    "followup_grace": "200ms",
    "account_retry_first": "300ms",
    "account_retry_second": "300ms",
    "account_retry_interval": "300ms",
    "account_retry_max_wait": NEVER,
}
WAITING = "Waiting for the Claude Code account: "
LIMIT = "the usage limit of the account is reached and resets at 2026-11-13 02:10 UTC"


def status_line(world: World, run_id: UUID) -> str:
    return str(world.fake.run(run_id).get("status_line", ""))


def retries(world: World) -> list[dict[str, Any]]:
    """The retry lines the session read from its inbox socket."""
    return [
        c for c in world.calls("bg_socket") if c["text"].startswith("Automatic retry by the runner")
    ]


async def held_run(world: World, config: dict[str, Any], **scenario: Any) -> tuple[UUID, Any]:
    """A run whose first turn started a background command and answered: its session is kept."""
    job = world.gate("background_gate")
    world.scenario(fixture="plain", background_gate=str(job), **scenario)
    await start(world, **config)
    run = await world.fake.trigger("Run the suite", channel_id=world.forum_a)
    await world.until(lambda: texts(world) == ["Done."])
    return run.run_id, job


async def test_an_account_error_keeps_the_session_and_the_retry_is_answered(
    bg_world: World, logs: LogLines
) -> None:
    world = bg_world
    failure = world.gate("background_error_gate")
    run_id, job = await held_run(
        world, FAST, background_error_gate=str(failure), background_api_error="usage_limit"
    )
    world.release(failure)  # the notification turn meets a used-up usage window
    await world.until(lambda: status_line(world, run_id).startswith(WAITING))
    assert status_line(world, run_id) == (
        f"{WAITING}{LIMIT}. The session and its background work are kept; next try in 1 second."
    )
    assert world.state_of(run_id) == "running"
    # The retry line goes to the inbox socket; the turn it starts is answered and posted.
    await world.until(lambda: len(retries(world)) == 1)
    await world.until(lambda: len(texts(world)) == 2)
    assert texts(world)[1] == "Status: the background job is still running."
    await world.until(lambda: status_line(world, run_id) == "")
    assert world.calls("bg_stop") == []  # the session and its work were kept all along
    world.release(job)
    assert await world.ended(run_id) == "completed"
    assert texts(world)[2] == "The suite passed."
    assert len(players(world)) == 1
    [started] = events(logs, "account_wait_started")
    assert started["error"] == "usage_limit"
    assert [e["attempt"] for e in events(logs, "account_retry")] == ["1"]
    assert events(logs, "turn_stopped_after_error") == []


async def test_the_bound_of_the_account_wait_ends_the_run_with_the_reason(
    bg_world: World, logs: LogLines
) -> None:
    world = bg_world
    failure = world.gate("background_error_gate")
    account = world.gate("account_gate")  # never opened: the account stays unusable
    run_id, _job = await held_run(
        world,
        {**FAST, "account_retry_max_wait": "1500ms"},
        background_error_gate=str(failure),
        background_api_error="usage_limit",
        account_gate=str(account),
    )
    world.release(failure)
    assert await world.ended(run_id) == "failed"
    tried = len(retries(world))
    assert tried >= 1
    assert world.fake.run_state(run_id).state_reason == "runtime_error"
    summary = world.fake.run(run_id)["summary"]
    assert summary.startswith(f"Claude Code could not use its account on the runner host: {LIMIT}.")
    assert f"tried the turn again {tried} time" in summary
    assert "Background work was still running and was stopped; its results are lost." in summary
    assert summary.endswith("(`runtime_error`)")
    [gave_up] = events(logs, "account_wait_gave_up")
    assert (gave_up["error"], gave_up["retries"]) == ("usage_limit", str(tried))
    await nothing_left(world)


async def test_a_stop_during_the_account_wait_ends_the_run_at_once(bg_world: World) -> None:
    world = bg_world
    failure = world.gate("background_error_gate")
    run_id, _job = await held_run(
        world,
        {**FAST, "account_retry_first": NEVER},
        background_error_gate=str(failure),
        background_api_error="usage_limit",
    )
    world.release(failure)
    await world.until(lambda: status_line(world, run_id).startswith(WAITING))
    await world.fake.cancel(run_id)
    assert await world.ended(run_id) == "cancelled"
    assert world.fake.run_state(run_id).state_reason == "user_cancelled"
    assert status_line(world, run_id) == ""
    assert retries(world) == []
    assert texts(world) == ["Done."]
    await nothing_left(world)


async def test_an_account_error_without_background_work_ends_the_run_at_once(
    bg_world: World,
) -> None:
    world = bg_world
    world.scenario(fixture="plain", api_error="usage_limit")
    await start(world, **FAST)
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run_state(run.run_id).state_reason == "runtime_error"
    summary = world.fake.run(run.run_id)["summary"]
    assert "This session had no background work to keep" in summary
    assert retries(world) == []
    await nothing_left(world)


async def test_a_lost_login_ends_the_run_not_authenticated(bg_world: World) -> None:
    world = bg_world
    world.scenario(fixture="plain", api_error="not_authenticated")
    await start(world)
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run_state(run.run_id).state_reason == "runtime_not_authenticated"
    await nothing_left(world)


# ---- recoveries across the two modes (09 18.9) --------------------------------------------------


async def test_a_full_context_fails_the_run_without_print_mode(bg_world: World) -> None:
    # The runner compacts only in print mode, which background mode never uses (owner order
    # 2026-11-17, FIX-RUNNER-CLI-PIN-326): the run fails with the cause in the thread.
    world = bg_world
    world.scenario(fixture="plain", per_launch={"0": {"api_error": "context_full"}})
    await start(world)
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(run.run_id) == "failed"
    assert world.fake.run(run.run_id)["summary"] == cause("background_compaction")
    assert world.turns() == []  # no compaction with claude -p
    assert len(launches(world)) == 1
    await nothing_left(world)


async def test_a_session_without_a_transcript_is_replaced_by_a_new_one(bg_world: World) -> None:
    world = bg_world
    await start(world)
    first = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    assert await world.ended(first.run_id) == "completed"
    session = players(world)[0]["session_id"]
    for path in (world.home / ".claude" / "projects").rglob(f"{session}.jsonl"):
        path.unlink()
    second = await world.fake.trigger("Add a test", thread_id=first.thread_id)
    assert await world.ended(second.run_id) == "completed"
    _, two = launches(world)
    assert "--resume" not in two["argv"]  # a new session, as after "session missing" in print
    replaced = players(world)[1]["session_id"]
    assert replaced != session
    reports = [r.json["external_session_ref"] for r in world.requests("reportAgentSession")]
    assert reports == [session, replaced]
    notes = [s["name"] for s in world.fake.run(second.run_id)["steps"].values()]
    assert "session_missing" in notes


# ---- a stop before the session runs -------------------------------------------------------------


async def test_a_stop_of_the_runner_during_the_launch_leaves_no_session(bg_world: World) -> None:
    world = bg_world
    gate = world.gate("bg_pid_gate")  # the entry has no process yet: the runner still waits
    world.scenario(fixture="plain", bg_pid_gate=str(gate))
    runner = await start(world)
    run = await world.fake.trigger("Fix it", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    await world.stop(runner)
    # The launch may have handed the prompt over: the run ends, it is not started again.
    assert world.state_of(run.run_id) == "cancelled"
    assert world.fake.run_state(run.run_id).state_reason == ADAPTER_STOPPED
    assert world.turns() == []
    await nothing_left(world)
    # The process that waited to become the session ends with its entry.
    [waiting] = world.calls("gate_wait")
    await world.until(lambda: not _pid_alive(waiting["pid"]))


async def test_fake_claude_writes_only_members_the_pinned_cli_writes(bg_world: World) -> None:
    # Fake-Claude constructs its transcript records; the recorded member table of the pinned
    # CLI (tests/fixtures/recording/background_<version>) holds every member of a record type
    # the runner reads, so the constructed records cannot drift from the real ones unseen.
    world = bg_world
    failure = world.gate("background_error_gate")
    run_id, job = await held_run(
        world, FAST, background_error_gate=str(failure), background_api_error="usage_limit"
    )
    world.release(failure)
    await world.until(lambda: len(texts(world)) == 2)  # the retry was answered
    world.release(job)
    assert await world.ended(run_id) == "completed"
    # The attachment of a line taken during a turn is held by the scenario of that line.
    assert members_seen(world) == set(READ_FIELDS) - {QUEUED_ATTACHMENT}


QUEUED_ATTACHMENT = "attachment/queued_command"
NOTIFICATION_ORIGIN = "origin/task-notification"


def members_seen(world: World) -> set[str]:
    """Hold what Fake-Claude wrote against the recorded member table; the record types seen."""
    folder = Path(__file__).resolve().parents[1] / "fixtures" / "recording"
    recording = folder / f"background_{PINNED_CLAUDE_CODE_VERSION}"
    table = json.loads((recording / "transcript_fields.json").read_text())["records"]
    seen = set()
    for record in transcript(world, players(world)[0]):
        name = str(record["type"])
        if name == "system":
            name = f"system/{record['subtype']}"
        if name not in READ_FIELDS:
            continue
        seen.add(name)
        # Read in the CLI's file, not recorded: the stand-in sends no usage window.
        assert set(record) - {"apiErrorParams"} <= set(table[name]), name
        assert set(READ_FIELDS[name]) - {
            "apiErrorParams",
            "commandUuid",
            "reason",
            "error",
            "isApiErrorMessage",
            "origin",
        } <= set(record), name
        inner = record.get("attachment")
        origin = record.get("origin") if name == "user" else None
        if name == "attachment" and inner.get("type") == "queued_command":
            seen.add(QUEUED_ATTACHMENT)
            assert set(inner) <= set(table[QUEUED_ATTACHMENT])
            assert set(READ_FIELDS[QUEUED_ATTACHMENT]) <= set(inner)
            origin = inner.get("origin")
        if isinstance(origin, dict) and origin.get("kind") == "task-notification":
            # The notification of the session's own work, with the producer the runner reads
            # (``subkind`` marks a delivery of another sender: read in the CLI's file).
            seen.add(NOTIFICATION_ORIGIN)
            assert set(origin) <= set(table[NOTIFICATION_ORIGIN])
            assert set(READ_FIELDS[NOTIFICATION_ORIGIN]) - {"subkind"} <= set(origin)
    return seen
