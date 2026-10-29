# SPDX-License-Identifier: Apache-2.0
"""Files and voice through the pinned Hermes (09 13, 17.3, 17.6; HERMES-MEDIA).

The model's answer names files with Hermes's ``MEDIA:`` tags; Hermes sends them through the
adapter's ``send_document``, ``send_voice``, ``send_video`` and ``send_image_file`` after the
text, and the adapter uploads each one to the FakeServer and sends it attached. A voice message
of the triggering person reaches Hermes's own speech to text, here a command provider of the
profile that prints the audio file, so the transcript proves the bytes went through.
"""

from __future__ import annotations

import json
from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.ids import uuid7
from bothy_sdk.testing.fakeserver import ContextFile
from fake_hermes_kit import (
    HermesRig,
    HermesWorld,
    Profile,
    _start_gateway,
    gateway_log,
    hermes_rig,
)
from hermes_harness.fake_model import ScriptTurn

pytestmark = pytest.mark.anyio

FILES = {
    "report.pdf": b"%PDF-1.4 the plan of the lake trip\n",
    "reply.mp3": b"ID3 a spoken answer",
    "clip.mp4": b"\x00\x00\x00\x18ftypmp42 a short video",
    "picture.png": b"\x89PNG\r\n\x1a\n a picture of the lake",
}
SPOKEN = "Please book the boat for Saturday."
#: A speech to text command of the profile: the "transcript" is the audio file's content.
STT = {
    "provider": "fakestt",
    "providers": {"fakestt": {"type": "command", "command": "cat {input_path}"}},
}


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


async def _done(rig: HermesRig, run_id: Any) -> None:
    try:
        await rig.until(lambda: rig.finished(run_id), "the end of the run")
    except AssertionError:
        raise AssertionError(gateway_log(rig)) from None
    assert rig.fake.run_state(run_id).state == "completed", gateway_log(rig)


def _beats(rig: HermesRig) -> list[dict[str, Any]]:
    return [f["d"]["agent"] for f in rig.fake.frames if f.get("op") == "heartbeat"]


def _sent_files(rig: HermesRig) -> dict[str, tuple[str, bytes]]:
    """Upload name to the message kind and the uploaded bytes, for every attached file."""
    out = {}
    for message in rig.fake.messages:
        for file_id in message.get("attachment_file_ids") or []:
            upload = rig.fake.uploads.sessions[UUID(file_id)]
            out[upload.name] = (str(message["kind"]), upload.content())
    return out


async def test_files_of_the_answer_arrive_as_attachments(world: HermesWorld) -> None:
    async with hermes_rig(world, [], start=False) as rig:
        out = rig.home / "out"
        out.mkdir()
        for name, data in FILES.items():
            (out / name).write_bytes(data)
        tags = " ".join(f"MEDIA:{out / name}" for name in FILES)
        secret = f"MEDIA:{rig.home / '.env'}"
        # The paths exist only now; the model's script is read at its first request.
        rig.model._script.append(
            ScriptTurn(text=(f"Here is everything for the trip. {tags} {secret}",), paced=False)
        )
        await _start_gateway(rig)
        assert _beats(rig)[-1]["details"]["capabilities"]["files_out"] is True
        triggered = await rig.fake.trigger("Send me the plan.")
        await _done(rig, triggered.run_id)
        sent = _sent_files(rig)
        assert sent == {
            "report.pdf": ("text", FILES["report.pdf"]),
            "reply.mp3": ("text", FILES["reply.mp3"]),
            "clip.mp4": ("text", FILES["clip.mp4"]),
            "picture.png": ("text", FILES["picture.png"]),
        }, gateway_log(rig)
        # Hermes's delivery policy keeps the profile's secrets out; nothing of .env went up.
        env = (rig.home / ".env").read_bytes()
        assert all(u.content() != env for u in rig.fake.uploads.sessions.values())
        texts = [m["content_md"] for m in rig.fake.messages if not m.get("attachment_file_ids")]
        assert texts
        # Hermes removes the tags of the delivered files from the text (a refused tag stays).
        assert not [t for t in texts for name in FILES if str(out / name) in t]
        assert all(m["run_id"] == str(triggered.run_id) for m in rig.fake.messages)
        ops = [r.operation_id for r in rig.fake.requests]
        last_message = len(ops) - 1 - ops[::-1].index("createMessage")
        assert last_message < ops.index("finishRun")


async def test_voice_message_reaches_hermes_speech_to_text(world: HermesWorld) -> None:
    script = [ScriptTurn(text=("The boat is booked.",), paced=False)]
    profile = Profile(config={"stt": STT})
    async with hermes_rig(world, script, texts=[SPOKEN], profile=profile) as rig:
        assert _beats(rig)[-1]["details"]["capabilities"]["voice_input"] is True
        voice = ContextFile(uuid7(), "voice.m4a", "audio/mp4", SPOKEN.encode())
        triggered = await rig.fake.trigger("", files=(voice,))
        await _done(rig, triggered.run_id)
        # The transcript reached the model as the person's words (09 17.6) ...
        asked = [r for r in rig.model_requests(scripted=True) if SPOKEN in r["scenario_texts"]]
        assert asked, gateway_log(rig)
        # ... and the server holds it as the agent's transcript (09 13).
        submitted = rig.requests("submitTranscript")
        assert [q.json["text"] for q in submitted] == [SPOKEN]
        assert rig.fake.world.files[voice.file_id]["transcript_source"] == "agent"
        # The audio lies in Hermes's audio cache of the profile, as on its other platforms.
        cached = [
            p for p in (rig.home / "cache").rglob("*.m4a") if p.read_bytes() == SPOKEN.encode()
        ]
        assert cached


async def test_voice_without_speech_to_text_reaches_hermes_as_its_note(
    world: HermesWorld,
) -> None:
    script = [ScriptTurn(text=("I could not listen to it.",), paced=False)]
    profile = Profile(config={"stt": {"enabled": False}})
    async with hermes_rig(world, script, profile=profile) as rig:
        assert "voice_input" not in _beats(rig)[-1]["details"]["capabilities"]
        voice = ContextFile(uuid7(), "voice.m4a", "audio/mp4", b"unheard audio")
        triggered = await rig.fake.trigger("Listen.", files=(voice,))
        await _done(rig, triggered.run_id)
        assert "submitTranscript" not in [r.operation_id for r in rig.fake.requests]
        bodies = json.dumps(rig.model.raw)
        assert "The user sent a voice message" in bodies, gateway_log(rig)
        assert "Listen." in bodies
