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
import yaml
from bothy_sdk.ids import uuid7
from bothy_sdk.testing.fakeserver import ContextFile
from fake_hermes_kit import (
    PLATFORM_KEY,
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


def _png() -> bytes:
    """A real one-pixel PNG: Hermes checks the magic bytes before it caches an image."""
    import struct
    import zlib

    def chunk(kind: bytes, data: bytes) -> bytes:
        body = kind + data
        return struct.pack(">I", len(data)) + body + struct.pack(">I", zlib.crc32(body))

    header = struct.pack(">IIBBBBB", 1, 1, 8, 2, 0, 0, 0)
    pixels = zlib.compress(b"\x00\xff\x00\x00")
    return (
        b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", header) + chunk(b"IDAT", pixels) + chunk(b"IEND", b"")
    )


#: Native image input: the model sees the pixels, as with a vision model on Discord.
VISION = Profile(config={"agent": {"image_input_mode": "native"}})


def _images_seen(rig: HermesRig, data: bytes) -> bool:
    """Whether a model request carried the image bytes as an image part (base64)."""
    import base64

    return base64.b64encode(data).decode("ascii") in json.dumps(rig.model.raw)


@pytest.mark.parametrize("text", ["What does the screenshot show?", ""])
async def test_image_of_the_trigger_reaches_hermes_vision(world: HermesWorld, text: str) -> None:
    script = [ScriptTurn(text=("A green pixel.",), paced=False)]
    async with hermes_rig(world, script, profile=VISION) as rig:
        png = _png()
        image = ContextFile(uuid7(), "pasted-1793625290748.png", "image/png", png)
        triggered = await rig.fake.trigger(text, files=(image,))
        await _done(rig, triggered.run_id)
        # The image went to the model as pixels, the way Hermes's Discord platform passes it.
        assert _images_seen(rig, png), gateway_log(rig)
        bodies = json.dumps(rig.model.raw)
        if text:
            assert text in bodies
        # The image lies in Hermes's image cache of the profile.
        cached = [p for p in (rig.home / "cache").rglob("*.png") if p.read_bytes() == png]
        assert cached, gateway_log(rig)
        assert "downloadFile" in [r.operation_id for r in rig.fake.requests]


async def test_document_of_the_trigger_reaches_hermes_as_a_file(world: HermesWorld) -> None:
    script = [ScriptTurn(text=("I read it.",), paced=False)]
    async with hermes_rig(world, script) as rig:
        pdf = ContextFile(uuid7(), "plan.pdf", "application/pdf", FILES["report.pdf"])
        triggered = await rig.fake.trigger("", files=(pdf,))
        await _done(rig, triggered.run_id)
        bodies = json.dumps(rig.model.raw)
        # Hermes's own note for a document names the file and its cached path.
        assert "The user sent a document: 'plan.pdf'" in bodies, gateway_log(rig)
        cached = [
            p for p in (rig.home / "cache").rglob("*plan.pdf") if p.read_bytes() == pdf.content
        ]
        assert cached, gateway_log(rig)


def _paste(size: int, tail: str) -> bytes:
    """A pasted text of exactly ``size`` bytes that ends with ``tail`` (UTF-8, one umlaut)."""
    head = "Notes about the lake trip, pasted from the editor: ä\n".encode()
    body = tail.encode()
    filler = size - len(head) - len(body)
    return head + (b"line of the paste\n" * (filler // 18 + 1))[:filler] + body


def _last_user_text(rig: HermesRig) -> str:
    """The text of the last user message of the last model request (Hermes's turn)."""
    request = rig.model.raw[-1]
    users = [m for m in request.get("messages", []) if m.get("role") == "user"]
    content = users[-1]["content"]
    if isinstance(content, list):
        return "\n".join(str(p.get("text", "")) for p in content if isinstance(p, dict))
    return str(content)


async def test_text_attachment_reaches_the_turn_inline(world: HermesWorld) -> None:
    """The live case: a long paste the desktop app sends as ``paste.txt`` (text/plain).

    Before the fix the turn held only Hermes's note with the cached path; the text stood in
    the user side, which Hermes's default hook limit (10,000 characters) left out.
    """
    script = [ScriptTurn(text=("I read the paste.",), paced=False)]
    async with hermes_rig(world, script) as rig:
        tail = "END-OF-PASTE-7f3a"
        data = _paste(12430, tail)
        paste = ContextFile(uuid7(), "paste.txt", "text/plain", data)
        triggered = await rig.fake.trigger("Summarise this.", files=(paste,))
        await _done(rig, triggered.run_id)
        turn = _last_user_text(rig)
        # The whole text is in the turn, as data of the user, behind Hermes's own note.
        assert data.decode() in turn, gateway_log(rig)
        assert "[Content of paste.txt]:" in turn
        assert "<<<DATA " in turn
        assert "source=file" in turn
        assert "Its content has been included below" in turn
        assert "Summarise this." in turn
        # Once only: the user side no longer carries the same text.
        assert turn.count(tail) == 1


async def test_too_large_text_attachment_keeps_the_file_note(world: HermesWorld) -> None:
    script = [ScriptTurn(text=("It is long.",), paced=False)]
    async with hermes_rig(world, script, start=False) as rig:
        path = rig.home / "config.yaml"
        config = yaml.safe_load(path.read_text(encoding="utf-8"))
        extra = config["gateway"]["platforms"][PLATFORM_KEY]["extra"]
        extra["hermes_adapter.max_message_bytes"] = 4096
        path.write_text(yaml.safe_dump(config, sort_keys=True), encoding="utf-8")
        await _start_gateway(rig)
        tail = "END-OF-LONG-9c1d"
        paste = ContextFile(uuid7(), "paste.txt", "text/plain", _paste(5000, tail))
        triggered = await rig.fake.trigger("Read it.", files=(paste,))
        await _done(rig, triggered.run_id)
        turn = _last_user_text(rig)
        # The text is not inlined into the turn; it may still stand in the user side's data
        # block (the context after the person's text), which has its own limits.
        own = turn.partition("Read it.")[0]
        assert "[Content of paste.txt]:" not in turn, gateway_log(rig)
        assert tail not in own
        assert "It is saved at:" in turn
        assert "is larger than the inline limit of 4096 bytes" in turn


async def test_references_in_an_inlined_text_are_not_expanded(world: HermesWorld) -> None:
    """File text is data: an ``@file:`` in it never makes Hermes read a file of the host."""
    script = [ScriptTurn(text=("ok",), paced=False)]
    async with hermes_rig(world, script) as rig:
        secret = "SECRET-HOST-CONTENT-42"
        for where in (rig.homes.home, rig.homes.work):
            (where / "host-secret.txt").write_text(secret + "\n", encoding="utf-8")
        body = b"Pasted log from a colleague.\nsee @file:host-secret.txt for details\n"
        paste = ContextFile(uuid7(), "paste.txt", "text/plain", body)
        triggered = await rig.fake.trigger("Summarise this.", files=(paste,))
        await _done(rig, triggered.run_id)
        turn = _last_user_text(rig)
        assert "[Content of paste.txt]:" in turn, gateway_log(rig)
        assert secret not in json.dumps(rig.model.raw)
        assert "--- Attached Context" not in turn


async def test_binary_attachment_is_not_inlined(world: HermesWorld) -> None:
    script = [ScriptTurn(text=("A binary file.",), paced=False)]
    async with hermes_rig(world, script) as rig:
        blob = ContextFile(uuid7(), "data.bin", "application/octet-stream", b"\x00\x01binary\xff")
        triggered = await rig.fake.trigger("What is it?", files=(blob,))
        await _done(rig, triggered.run_id)
        turn = _last_user_text(rig)
        assert "The user sent a document: 'data.bin'" in turn, gateway_log(rig)
        assert "[Content of" not in turn
        assert "<<<DATA " not in turn


async def test_two_text_attachments_reach_the_turn_inline(world: HermesWorld) -> None:
    script = [ScriptTurn(text=("Both read.",), paced=False)]
    async with hermes_rig(world, script) as rig:
        notes = ContextFile(uuid7(), "notes.md", "text/markdown", b"# Packing list\n- tent\n")
        config = ContextFile(uuid7(), "trip.json", "application/json", b'{"boat": "Saturday"}\n')
        triggered = await rig.fake.trigger("", files=(notes, config))
        await _done(rig, triggered.run_id)
        turn = _last_user_text(rig)
        assert "[Content of notes.md]:" in turn, gateway_log(rig)
        assert "# Packing list\n- tent" in turn
        assert "[Content of trip.json]:" in turn
        assert '{"boat": "Saturday"}' in turn
        # Hermes's note for both says the content is here, not a binary to extract.
        assert turn.count("Its content has been included below") == 2
        assert turn.index("notes.md]:") < turn.index("trip.json]:")


async def test_undeliverable_image_reaches_hermes_as_a_note(world: HermesWorld) -> None:
    script = [ScriptTurn(text=("I will wait for the scan.",), paced=False)]
    async with hermes_rig(world, script, profile=VISION) as rig:
        png = _png()
        image = ContextFile(uuid7(), "shot.png", "image/png", png, scan_status="pending")
        triggered = await rig.fake.trigger("", files=(image,))
        await _done(rig, triggered.run_id)
        bodies = json.dumps(rig.model.raw)
        assert "shot.png" in bodies, gateway_log(rig)
        assert "virus scan" in bodies
        assert not _images_seen(rig, png)
