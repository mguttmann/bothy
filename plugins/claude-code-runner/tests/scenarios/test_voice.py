# SPDX-License-Identifier: Apache-2.0
"""Voice messages and audio attachments with and without a transcriber (09 13, 18.14)."""

from __future__ import annotations

import json
import os
import sys
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest

from .conftest import LogLines, World

pytestmark = pytest.mark.anyio


def _fake_transcriber(world: World) -> str:
    path = world.root / "transcribe"
    path.write_text(
        f"#!{sys.executable}\nimport pathlib, sys\n"
        "data = pathlib.Path(sys.argv[-1]).read_bytes()\n"
        "print('VOICE-CANARY please run the tests', data.decode())\n"
    )
    path.chmod(0o755)
    return str(path)


def _capabilities(world: World) -> dict[str, Any]:
    beat = world.fake.heartbeats[-1]
    caps: dict[str, Any] = beat["details"]["capabilities"]
    return caps


def _trigger_item(world: World) -> str:
    (turn,) = world.turns()
    line = json.loads(turn["stdin"][0])
    text: str = line["message"]["content"][1]["text"]
    return text


def _system_item(world: World) -> str:
    (turn,) = world.turns()
    line = json.loads(turn["stdin"][0])
    text: str = line["message"]["content"][0]["text"]
    return text


async def test_transcript_is_the_content_of_the_voice_message(world: World) -> None:
    fake = world.fake
    voice = fake.add_file(b"AUDIO-BYTES", name="memo.ogg", mime="audio/ogg")
    await world.start(world.config(transcribe_cmd=[_fake_transcriber(world)]))
    assert _capabilities(world)["voice_input"] is True
    run = await fake.trigger("", channel_id=world.forum_a, attachments=[voice])
    assert await world.ended(run.run_id) == "completed"
    trigger = _trigger_item(world)
    assert 'Transcript of attachment "memo.ogg":' in trigger
    assert "VOICE-CANARY please run the tests AUDIO-BYTES" in trigger
    assert "not transcribed" not in _system_item(world)
    # The transcript is delivered to the server (09 13, mode agent), and only the answer is a
    # message: no note about a missing transcriber.
    (submit,) = world.requests("submitTranscript")
    assert submit.json["text"] == "VOICE-CANARY please run the tests AUDIO-BYTES"
    assert all("transcri" not in m["content_md"] for m in fake.messages)


async def test_without_a_transcriber_the_thread_gets_a_note(world: World) -> None:
    fake = world.fake
    voice = fake.add_file(b"AUDIO-BYTES", name="memo.ogg", mime="audio/ogg")
    await world.start()
    assert _capabilities(world)["voice_input"] is False
    run = await fake.trigger("", channel_id=world.forum_a, attachments=[voice])
    assert await world.ended(run.run_id) == "completed"
    notes = [m for m in fake.messages if "runner.transcribe_cmd" in m["content_md"]]
    assert len(notes) == 1
    assert notes[0]["run_id"] == str(run.run_id)
    assert "not transcribed: transcriber_not_configured" in _system_item(world)
    assert "Transcript of attachment" not in _trigger_item(world)
    assert world.requests("submitTranscript") == []


async def test_a_server_transcript_is_used_as_it_is(world: World) -> None:
    fake = world.fake
    voice = fake.add_file(
        b"AUDIO-BYTES", name="memo.ogg", mime="audio/ogg", transcript="SERVER-TRANSCRIPT"
    )
    await world.start(world.config(transcribe_cmd=[_fake_transcriber(world)]))
    run = await fake.trigger("", channel_id=world.forum_a, attachments=[voice])
    assert await world.ended(run.run_id) == "completed"
    trigger = _trigger_item(world)
    assert "SERVER-TRANSCRIPT" in trigger
    assert "VOICE-CANARY" not in trigger
    assert world.requests("submitTranscript") == []


# ---- follow-ups (FIX-followup-transcribe) ----------------------------------------------------


def _gated_transcriber(world: World, gate: Path) -> str:
    """A transcriber that records its PID and waits for ``gate`` before it answers."""
    path = world.root / "transcribe-gated"
    path.write_text(
        f"#!{sys.executable}\nimport os, pathlib, sys, time\n"
        f"pathlib.Path({str(world.root / 'transcriber.pid')!r}).write_text(str(os.getpid()))\n"
        f"while not pathlib.Path({str(gate)!r}).exists():\n"
        "    time.sleep(0.02)\n"
        "print('LATE-VOICE')\n"
    )
    path.chmod(0o755)
    return str(path)


def _failing_transcriber(world: World) -> str:
    path = world.root / "transcribe-fails"
    path.write_text(f"#!{sys.executable}\nimport sys\nsys.exit(3)\n")
    path.chmod(0o755)
    return str(path)


def _followup_text(world: World, index: int = 1) -> str:
    text: str = json.loads(world.turns()[0]["stdin"][index])["message"]["content"][0]["text"]
    return text


async def _open_turn(world: World, **config: Any) -> tuple[UUID, Path]:
    gate = world.gate("turn_gate")
    world.scenario(fixture="plain", turn_gate=str(gate))
    await world.start(world.config(**config))
    run = await world.fake.trigger("Fix the form", channel_id=world.forum_a)
    await world.until(lambda: len(world.calls("gate_wait")) == 1)
    return run.run_id, gate


async def test_a_voice_followup_reaches_the_turn_with_its_transcript(world: World) -> None:
    run_id, gate = await _open_turn(world, transcribe_cmd=[_fake_transcriber(world)])
    voice = world.fake.add_file(b"AUDIO-BYTES", name="memo.ogg", mime="audio/ogg")
    message_id = await world.fake.deliver_followup(run_id, "", author="sam", files=(voice,))
    await world.until(lambda: len(world.turns()[0]["stdin"]) == 2)
    stored = world.state / "files" / str(world.fake.world.runs[run_id].thread_id) / str(voice)
    assert _followup_text(world) == (
        "Follow-up from Sam (@sam):\n\n"
        'Transcript of attachment "memo.ogg":\n'
        "VOICE-CANARY please run the tests AUDIO-BYTES\n\n"
        "Attachments:\n"
        f"- memo.ogg (audio/ogg): local file: {stored}"
    )
    (submit,) = world.requests("submitTranscript")
    assert submit.json["text"] == "VOICE-CANARY please run the tests AUDIO-BYTES"
    assert str(voice) in submit.path
    await world.until(lambda: world.fake.followup_state(run_id, message_id) == "delivered")
    world.release(gate)
    assert await world.ended(run_id) == "completed"
    assert all("transcri" not in m["content_md"] for m in world.fake.messages)


async def test_a_failed_transcription_of_a_followup_notes_it_and_delivers_the_file(
    world: World,
) -> None:
    run_id, gate = await _open_turn(world, transcribe_cmd=[_failing_transcriber(world)])
    voice = world.fake.add_file(b"AUDIO-BYTES", name="memo.ogg", mime="audio/ogg")
    message_id = await world.fake.deliver_followup(
        run_id, "Listen to this", author="sam", files=(voice,)
    )
    await world.until(lambda: len(world.turns()[0]["stdin"]) == 2)
    stored = world.state / "files" / str(world.fake.world.runs[run_id].thread_id) / str(voice)
    assert _followup_text(world) == (
        "Follow-up from Sam (@sam):\n\nListen to this\n\n"
        "Attachments:\n"
        f"- memo.ogg (audio/ogg): local file: {stored}\n"
        "  not transcribed: transcriber_failed"
    )
    await world.until(lambda: world.fake.followup_state(run_id, message_id) == "delivered")
    notes = [m for m in world.fake.messages if "Voice message not transcribed" in m["content_md"]]
    assert len(notes) == 1
    assert "`transcriber_failed`" in notes[0]["content_md"]
    assert notes[0]["run_id"] == str(run_id)
    assert world.requests("submitTranscript") == []
    world.release(gate)
    assert await world.ended(run_id) == "completed"


async def test_a_stop_cancels_the_transcription_of_a_followup(world: World) -> None:
    hold = world.root / "transcriber.open"
    run_id, _ = await _open_turn(world, transcribe_cmd=[_gated_transcriber(world, hold)])
    voice = world.fake.add_file(b"AUDIO-BYTES", name="memo.ogg", mime="audio/ogg")
    message_id = await world.fake.deliver_followup(run_id, "", author="sam", files=(voice,))
    pid_file = world.root / "transcriber.pid"
    await world.until(pid_file.exists)
    await world.until(lambda: pid_file.read_text().strip() != "")
    pid = int(pid_file.read_text())
    await world.fake.cancel(run_id, "user_cancelled")
    assert await world.ended(run_id) == "cancelled"
    await world.until(lambda: _gone(pid))
    assert len(world.turns()[0]["stdin"]) == 1
    assert world.requests("submitTranscript") == []
    assert world.fake.followup_state(run_id, message_id) == "queued"


async def test_followups_keep_their_order_while_one_is_transcribed(world: World) -> None:
    hold = world.root / "transcriber.open"
    run_id, gate = await _open_turn(world, transcribe_cmd=[_gated_transcriber(world, hold)])
    voice = world.fake.add_file(b"AUDIO-BYTES", name="memo.ogg", mime="audio/ogg")
    await world.fake.deliver_followup(run_id, "First", author="sam", files=(voice,))
    await world.until((world.root / "transcriber.pid").exists)
    await world.fake.deliver_followup(run_id, "Second", author="sam")
    await world.until(lambda: len(world.requests("getMessage")) >= 2)
    assert len(world.turns()[0]["stdin"]) == 1  # the second waits for the first
    hold.touch()
    await world.until(lambda: len(world.turns()[0]["stdin"]) == 3)
    first, second = _followup_text(world, 1), _followup_text(world, 2)
    assert first.startswith("Follow-up from Sam (@sam):\n\nFirst\n\n")
    assert "LATE-VOICE" in first
    assert second == "Follow-up from Sam (@sam):\n\nSecond"
    world.release(gate)
    assert await world.ended(run_id) == "completed"


async def test_a_slow_transcription_does_not_expire_the_followups(
    world: World, logs: LogLines
) -> None:
    # The token is judged when the follow-up is offered: a transcription longer than the
    # token's lifetime loses neither the voice follow-up nor the text follow-up behind it.
    hold = world.root / "transcriber.open"
    run_id, gate = await _open_turn(world, transcribe_cmd=[_gated_transcriber(world, hold)])
    voice = world.fake.add_file(b"AUDIO-BYTES", name="memo.ogg", mime="audio/ogg")
    first = await world.fake.deliver_followup(run_id, "First", author="sam", files=(voice,))
    await world.until((world.root / "transcriber.pid").exists)
    second = await world.fake.deliver_followup(run_id, "Second", author="sam")
    await world.until(lambda: len(world.requests("getMessage")) >= 2)
    await world.fake.clock.advance(600)  # well past the token's lifetime and leeway
    hold.touch()
    await world.until(lambda: len(world.turns()[0]["stdin"]) == 3)
    assert "LATE-VOICE" in _followup_text(world, 1)
    assert _followup_text(world, 2) == "Follow-up from Sam (@sam):\n\nSecond"
    await world.until(lambda: world.fake.followup_state(run_id, first) == "delivered")
    await world.until(lambda: world.fake.followup_state(run_id, second) == "delivered")
    assert "followup_token_expired" not in logs.text
    world.release(gate)
    assert await world.ended(run_id) == "completed"


async def test_an_unauthorised_voice_followup_is_neither_stored_nor_transcribed(
    world: World, logs: LogLines
) -> None:
    run_id, gate = await _open_turn(world, transcribe_cmd=[_fake_transcriber(world)])
    voice = world.fake.add_file(b"AUDIO-BYTES", name="memo.ogg", mime="audio/ogg")
    alex = str(world.fake.world.alex.id)
    await world.fake.deliver_followup(
        run_id, "", author="sam", claims={"sub": alex}, files=(voice,)
    )
    await world.until(lambda: "followup_check_failed" in logs.text)
    world.release(gate)
    assert await world.ended(run_id) == "completed"
    assert len(world.turns()[0]["stdin"]) == 1
    stored = world.state / "files" / str(world.fake.world.runs[run_id].thread_id) / str(voice)
    assert not stored.exists()
    assert world.requests("submitTranscript") == []
    assert all("transcri" not in m["content_md"] for m in world.fake.messages)


def _gone(pid: int) -> bool:
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return True
    return False
