# SPDX-License-Identifier: Apache-2.0
"""Files from Hermes as answer attachments and voice input to Hermes (09 13, 17.3, 17.6).

Hermes is the fake runtime of ``adapter_kit``; the server is the SDK's FakeServer with its upload
sessions in mode ``direct``.
"""

from __future__ import annotations

from pathlib import Path
from typing import Any
from uuid import UUID

import pytest
from adapter_kit import FakeRuntime, Rig, captured_logs, rig
from bothy_sdk.ids import uuid7
from bothy_sdk.testing.fakeserver import ContextFile
from bothy_sdk.testing.faults import Fault

from bothy_hermes_adapter.runtime import FileSent, MediaKind, TurnOutcome, TurnRequest

pytestmark = pytest.mark.anyio

REPORT = b"%PDF-1.4 a report about the lake\n" * 300


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def _ops(r: Rig) -> list[str]:
    return [q.operation_id for q in r.fake.requests]


def _sending(
    files: list[tuple[str, MediaKind, str | None, str | None]], sent: list[FileSent]
) -> Any:
    """A turn that answers and then sends the given files, as Hermes does after its text."""

    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        await rt.events.on_answer(turn.thread_id, "Here it is.", None)
        for path, kind, caption, name in files:
            sent.append(await rt.events.on_file(turn.thread_id, path, kind, caption, name))
        return TurnOutcome.COMPLETED

    return script


async def _run(r: Rig) -> UUID:
    triggered = await r.fake.trigger("Send me the report.")
    await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
    return triggered.run_id


async def test_file_is_uploaded_in_parts_and_sent_as_an_attachment(tmp_path: Path) -> None:
    path = tmp_path / "report.pdf"
    path.write_bytes(REPORT)
    sent: list[FileSent] = []
    runtime = FakeRuntime(
        script=_sending([(str(path), MediaKind.DOCUMENT, "The report", None)], sent)
    )
    async with rig(tmp_path, runtime=runtime) as r:
        r.fake.uploads.part_size = 4096
        run_id = await _run(r)
        assert r.fake.run_state(run_id).state == "completed"
        assert sent[0].error is None
        assert sent[0].message_id
        texts = [m for m in r.fake.messages if not m.get("attachment_file_ids")]
        files = [m for m in r.fake.messages if m.get("attachment_file_ids")]
        assert [m["content_md"] for m in texts] == ["Here it is."]
        assert len(files) == 1
        message = files[0]
        assert message["content_md"] == "The report"
        assert message["kind"] == "text"
        assert message["run_id"] == str(run_id)
        (file_id,) = message["attachment_file_ids"]
        upload = r.fake.uploads.sessions[UUID(file_id)]
        assert upload.name == "report.pdf"
        assert upload.content() == REPORT
        assert upload.attached is not None
        assert str(upload.attached) == sent[0].message_id
        assert len(r.requests("uploadPart")) == upload.part_count > 1
        assert r.requests("createUpload")[0].json["declared_mime_type"] == "application/pdf"
        ops = _ops(r)
        assert (
            ops.index("createUpload")
            < ops.index("completeUpload")
            < len(ops) - 1 - ops[::-1].index("createMessage")
            < ops.index("finishRun")
        )
        # Every upload request went out bound to the run (07 SR9).
        for op in ("createUpload", "uploadPart", "completeUpload"):
            assert all("invocation-token" in q.headers for q in r.requests(op))


async def test_voice_goes_as_an_attachment_of_a_text_message(tmp_path: Path) -> None:
    # The server refuses kind = voice until its scan has made the file final (12 8.1, 409
    # file_not_ready); the FakeServer never scans, so a voice kind here would fail.
    mp3 = tmp_path / "reply.mp3"
    mp3.write_bytes(b"ID3 fake mp3 audio")
    ogg = tmp_path / "reply.ogg"
    ogg.write_bytes(b"OggS fake opus audio")
    sent: list[FileSent] = []
    files = [(str(mp3), MediaKind.VOICE, None, None), (str(ogg), MediaKind.VOICE, None, None)]
    async with rig(tmp_path, runtime=FakeRuntime(script=_sending(files, sent))) as r:
        await _run(r)
        assert [s.error for s in sent] == [None, None]
        kinds = {
            r.fake.uploads.sessions[UUID(m["attachment_file_ids"][0])].name: m["kind"]
            for m in r.fake.messages
            if m.get("attachment_file_ids")
        }
        assert kinds == {"reply.mp3": "text", "reply.ogg": "text"}


async def test_file_name_of_hermes_names_the_upload(tmp_path: Path) -> None:
    path = tmp_path / "tmp1234.bin"
    path.write_bytes(REPORT)
    sent: list[FileSent] = []
    files = [(str(path), MediaKind.DOCUMENT, None, "../../trip plan.pdf")]
    async with rig(tmp_path, runtime=FakeRuntime(script=_sending(files, sent))) as r:
        await _run(r)
        assert sent[0].error is None
        assert r.requests("createUpload")[0].json["name"] == "trip plan.pdf"


@pytest.mark.parametrize(
    ("make", "code"),
    [
        ("missing", "not_a_file"),
        ("link", "not_a_file"),
        ("directory", "not_a_file"),
        ("empty", "file_empty"),
        ("large", "file_too_large"),
    ],
)
async def test_unsendable_file_is_refused_before_any_upload(
    tmp_path: Path, make: str, code: str
) -> None:
    target = tmp_path / "secret.txt"
    target.write_bytes(b"0123456789abcdef")
    path = tmp_path / "file"
    if make == "link":
        path.symlink_to(target)
    elif make == "directory":
        path.mkdir()
    elif make == "empty":
        path.write_bytes(b"")
    elif make == "large":
        path.write_bytes(b"x" * 64)
    sent: list[FileSent] = []
    files = [(str(path), MediaKind.DOCUMENT, "caption", None)]
    async with rig(tmp_path, runtime=FakeRuntime(script=_sending(files, sent))) as r:
        r.fake.uploads.limits["file_max_upload_bytes"] = 32
        with captured_logs() as logs:
            run_id = await _run(r)
        assert sent == [FileSent(None, code)]
        assert "createUpload" not in _ops(r)
        assert [m["content_md"] for m in r.fake.messages] == ["Here it is."]
        assert r.fake.run_state(run_id).state == "completed"
        assert str(tmp_path) not in logs.text()


async def test_failed_upload_is_aborted_and_nothing_is_sent(tmp_path: Path) -> None:
    path = tmp_path / "report.pdf"
    path.write_bytes(REPORT)
    sent: list[FileSent] = []
    files = [(str(path), MediaKind.DOCUMENT, None, None)]
    async with rig(tmp_path, runtime=FakeRuntime(script=_sending(files, sent))) as r:
        r.fake.inject("completeUpload", Fault.problem("conflict", "invalid_state_transition"))
        run_id = await _run(r)
        assert sent == [FileSent(None, "conflict")]
        assert len(r.requests("abortUpload")) == 1
        assert [m["content_md"] for m in r.fake.messages] == ["Here it is."]
        assert r.fake.run_state(run_id).state == "completed"


async def test_attachment_refused_by_the_server_sends_nothing(tmp_path: Path) -> None:
    path = tmp_path / "report.pdf"
    path.write_bytes(REPORT)
    sent: list[FileSent] = []

    async def script(turn: TurnRequest, rt: FakeRuntime) -> TurnOutcome:
        await rt.events.on_answer(turn.thread_id, "Here it is.", None)
        # The owner switched supports_files_out off: the server refuses the attachment (2.2).
        r_fake.inject("createMessage", Fault.problem("feature_disabled"))
        sent.append(
            await rt.events.on_file(turn.thread_id, str(path), MediaKind.DOCUMENT, None, None)
        )
        return TurnOutcome.COMPLETED

    async with rig(tmp_path, runtime=FakeRuntime(script=script)) as r:
        r_fake = r.fake
        run_id = await _run(r)
        assert sent == [FileSent(None, "feature_disabled")]
        assert [m["content_md"] for m in r.fake.messages] == ["Here it is."]
        assert r.fake.run_state(run_id).state == "completed"


async def test_file_without_an_active_run_is_refused(tmp_path: Path) -> None:
    path = tmp_path / "report.pdf"
    path.write_bytes(REPORT)
    async with rig(tmp_path) as r:
        await _run(r)
        before = len(r.fake.requests)
        sent = await r.handler.on_file(uuid7(), str(path), MediaKind.DOCUMENT, None, None)
        assert sent.message_id is None
        assert sent.error
        assert len(r.fake.requests) == before


async def test_capabilities_report_files_out_and_voice_input(tmp_path: Path) -> None:
    async with rig(tmp_path, runtime=FakeRuntime(stt=True)) as r:
        caps = r.handler.capabilities()
        assert caps["files_out"] is True
        assert caps["voice_input"] is True
        r.runtime.stt = False
        _, _, caps = r.handler.heartbeat_report()
        assert caps["files_out"] is True
        assert "voice_input" not in caps


# ---- voice input -------------------------------------------------------------------------------


def _voice(content: bytes, **over: Any) -> ContextFile:
    values: dict[str, Any] = {"name": "voice.m4a", "mime_type": "audio/mp4"}
    values.update(over)
    return ContextFile(uuid7(), content=content, **values)


async def test_voice_message_is_transcribed_by_hermes_and_submitted(tmp_path: Path) -> None:
    runtime = FakeRuntime(transcripts={b"audio-1": "Please book the boat for Saturday."})
    async with rig(tmp_path, runtime=runtime) as r:
        voice = _voice(b"audio-1")
        triggered = await r.fake.trigger("", files=(voice,))
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
        assert r.fake.run_state(triggered.run_id).state == "completed"
        assert runtime.heard == [(b"audio-1", "voice.m4a", "audio/mp4")]
        user_text = runtime.turns[0].user_text
        assert "Please book the boat for Saturday." in user_text
        assert "Please book the boat" not in runtime.turns[0].system_part.split("Triggering")[0]
        submitted = r.requests("submitTranscript")
        assert [q.json["text"] for q in submitted] == ["Please book the boat for Saturday."]
        assert submitted[0].headers.get("if-match")
        record = r.fake.world.files[voice.file_id]
        assert record["transcript_source"] == "agent"
        ops = _ops(r)
        assert ops.index("startRun") < ops.index("submitTranscript") < ops.index("finishRun")


async def test_voice_without_a_transcript_reaches_hermes_with_its_note(tmp_path: Path) -> None:
    runtime = FakeRuntime()
    async with rig(tmp_path, runtime=runtime) as r:
        triggered = await r.fake.trigger("Listen.", files=(_voice(b"noise"),))
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
        assert r.fake.run_state(triggered.run_id).state == "completed"
        assert "could not be transcribed" in runtime.turns[0].user_text
        assert "Listen." in runtime.turns[0].user_text
        assert "submitTranscript" not in _ops(r)


async def test_refused_transcript_leaves_the_turn_as_it_is(tmp_path: Path) -> None:
    runtime = FakeRuntime(transcripts={b"audio-2": "Two tickets."})
    async with rig(tmp_path, runtime=runtime) as r:
        r.fake.inject("submitTranscript", Fault.problem("conflict", "transcript_not_agent"))
        with captured_logs() as logs:
            triggered = await r.fake.trigger("", files=(_voice(b"audio-2"),))
            await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
        assert r.fake.run_state(triggered.run_id).state == "completed"
        assert "Two tickets." in runtime.turns[0].user_text
        assert "Two tickets." not in logs.text()


@pytest.mark.parametrize(
    "voice",
    [
        _voice(b"served", transcript="The server's transcript."),
        _voice(b"unscanned", scan_status="pending"),
        _voice(b"blocked", scan_status="blocked"),
        _voice(b"not audio", name="notes.txt", mime_type="text/plain"),
    ],
)
async def test_only_served_audio_without_a_transcript_is_transcribed(
    tmp_path: Path, voice: ContextFile
) -> None:
    runtime = FakeRuntime(transcripts={b"served": "Hermes's transcript."})
    async with rig(tmp_path, runtime=runtime) as r:
        triggered = await r.fake.trigger("Hello.", files=(voice,))
        await r.until(lambda: r.finished(triggered.run_id), "the end of the run")
        assert r.fake.run_state(triggered.run_id).state == "completed"
        assert runtime.heard == []
        assert "submitTranscript" not in _ops(r)
        if voice.transcript:
            assert voice.transcript in runtime.turns[0].user_text
