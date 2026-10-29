# SPDX-License-Identifier: Apache-2.0
"""The local transcriber of audio attachments (09 18.14) with fake transcribers."""

from __future__ import annotations

import sys
from pathlib import Path
from uuid import UUID, uuid4

import pytest
from bothy_sdk.generated.models import RunContextAttachment
from bothy_sdk.generated.types import Unset

from claude_code_runner.transcribe import (
    NOT_CONFIGURED,
    TRANSCRIPT_MAX_BYTES,
    Transcriber,
    Untranscribed,
    notice,
    transcribe_attachments,
)

pytestmark = pytest.mark.anyio


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def script(tmp_path: Path, body: str) -> str:
    path = tmp_path / "transcriber"
    path.write_text(f"#!{sys.executable}\nimport os, sys, time, pathlib\n{body}\n")
    path.chmod(0o755)
    return str(path)


def attachment(mime: str = "audio/ogg", transcript: str | None = None) -> RunContextAttachment:
    item = RunContextAttachment(
        file_id=uuid4(), original_name="memo.ogg", mime_type=mime, size_bytes=3, scan_status="clean"
    )
    if transcript is not None:
        item.transcript = transcript
    return item


def stored(tmp_path: Path, item: RunContextAttachment, content: bytes = b"ogg") -> dict[UUID, str]:
    path = tmp_path / str(item.file_id)
    path.write_bytes(content)
    return {item.file_id: f"local file: {path}"}


def transcriber(argv: tuple[str, ...], timeout: float = 30.0) -> Transcriber:
    return Transcriber(argv=argv, env={"PATH": "/usr/bin:/bin", "SECRET": "x"}, timeout=timeout)


async def test_text_of_the_transcriber_becomes_the_transcript(tmp_path: Path) -> None:
    cmd = script(
        tmp_path,
        "data = pathlib.Path(sys.argv[-1]).read_bytes()\n"
        "env = ('HOME' in os.environ, os.environ.get('SECRET'))\n"
        "print(f'  {sys.argv[1]} heard {data.decode()} env={env}  ')",
    )
    item = attachment()
    texts = stored(tmp_path, item, b"OGG-BYTES")
    before = dict(texts)
    done, missing = await transcribe_attachments([item], texts, transcriber((cmd, "-x")))
    assert done == [item]
    assert missing == []
    assert item.transcript == "-x heard OGG-BYTES env=(False, 'x')"
    assert texts == before  # the file entry stays the local path


async def test_without_a_transcriber_the_file_entry_says_so(tmp_path: Path) -> None:
    item = attachment()
    texts = stored(tmp_path, item)
    calls: list[int] = []
    done, missing = await transcribe_attachments(
        [item], texts, transcriber(()), on_start=lambda: calls.append(1)
    )
    assert done == []
    assert missing == [Untranscribed(item.file_id, "memo.ogg", NOT_CONFIGURED)]
    assert isinstance(item.transcript, Unset)
    assert texts[item.file_id].endswith("\nnot transcribed: transcriber_not_configured")
    assert calls == []
    assert "runner.transcribe_cmd" in notice(missing)


@pytest.mark.parametrize(
    ("body", "code"),
    [
        ("sys.exit(3)", "transcriber_failed"),
        ("print('   ')", "transcript_empty"),
        ("time.sleep(30)", "transcriber_timeout"),
    ],
)
async def test_failures_leave_the_file_and_name_the_code(
    tmp_path: Path, body: str, code: str
) -> None:
    item = attachment()
    texts = stored(tmp_path, item)
    _, missing = await transcribe_attachments(
        [item], texts, transcriber((script(tmp_path, body),), timeout=0.5)
    )
    assert [m.problem for m in missing] == [code]
    assert texts[item.file_id].endswith(f"not transcribed: {code}")
    assert f"`{code}`" in notice(missing)


async def test_missing_program_is_unreachable(tmp_path: Path) -> None:
    item = attachment()
    texts = stored(tmp_path, item)
    _, missing = await transcribe_attachments(
        [item], texts, transcriber((str(tmp_path / "nothing"),))
    )
    assert [m.problem for m in missing] == ["transcriber_unreachable"]


async def test_long_output_is_cut_at_a_character_boundary(tmp_path: Path) -> None:
    cmd = script(tmp_path, f"sys.stdout.write('\\u00e9' * {TRANSCRIPT_MAX_BYTES})")
    item = attachment()
    await transcribe_attachments([item], stored(tmp_path, item), transcriber((cmd,)))
    assert isinstance(item.transcript, str)
    assert len(item.transcript.encode("utf-8")) == TRANSCRIPT_MAX_BYTES
    assert set(item.transcript) == {"\u00e9"}


async def test_only_stored_audio_without_a_server_transcript_is_given(tmp_path: Path) -> None:
    cmd = script(tmp_path, "print('words')")
    text_file = attachment(mime="text/plain")
    served = attachment(transcript="from the server")
    blocked = attachment()
    audio = attachment(mime="Audio/Mpeg")
    texts = {
        **stored(tmp_path, text_file),
        **stored(tmp_path, served),
        blocked.file_id: "unavailable: file_blocked",
        **stored(tmp_path, audio),
    }
    starts: list[int] = []
    done, missing = await transcribe_attachments(
        [text_file, served, blocked, audio], texts, transcriber((cmd,)), lambda: starts.append(1)
    )
    assert done == [audio]
    assert missing == []
    assert audio.transcript == "words"
    assert served.transcript == "from the server"
    assert isinstance(text_file.transcript, Unset)
    assert isinstance(blocked.transcript, Unset)
    assert starts == [1]
