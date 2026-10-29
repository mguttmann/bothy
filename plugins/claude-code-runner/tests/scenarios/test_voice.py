# SPDX-License-Identifier: Apache-2.0
"""Voice messages and audio attachments with and without a transcriber (09 13, 18.14)."""

from __future__ import annotations

import json
import sys
from typing import Any

import pytest

from .conftest import World

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
