# SPDX-License-Identifier: Apache-2.0
"""Transcripts of voice messages and audio attachments (docs/spec/09-agents.md 13 and 18.14).

An audio attachment (``mime_type`` ``audio/*``) of the triggering message that has no transcript
of the server and was stored as a local file is given to the local transcriber
``runner.transcribe_cmd``: its argument list plus the file path, no shell, stdin closed, stderr
discarded, the allow-list environment of 18.2 without any mode part, its own process group and
the time limit ``runner.transcribe_timeout``. Its stdout (UTF-8, trimmed, at most
`TRANSCRIPT_MAX_BYTES`) becomes the attachment's ``transcript``, which the context helper puts
into the trigger part with the trust of the message, exactly like a transcript of the server.
Without a transcriber, or when it fails, the file entry says so and the caller tells the person
in the thread (`notice`). Neither the text nor the path is logged.

Examples:
    >>> is_audio("audio/ogg"), is_audio("text/plain")
    (True, False)
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import logging
import os
import signal
from collections.abc import Callable, Mapping, Sequence
from dataclasses import dataclass
from pathlib import Path
from uuid import UUID

from bothy_sdk.generated.models import RunContextAttachment
from bothy_sdk.generated.types import Unset

__all__ = [
    "LOCAL_FILE",
    "NOT_CONFIGURED",
    "TRANSCRIPT_MAX_BYTES",
    "Transcriber",
    "Untranscribed",
    "is_audio",
    "notice",
    "transcribe_attachments",
]

_LOG = logging.getLogger("claude_code_runner")

TRANSCRIPT_MAX_BYTES = 32768
"""Longest transcript in UTF-8 bytes, cut at a character boundary; equal to the default of
``agent.context_message_max_bytes``, the size the server gives one context message."""

LOCAL_FILE = "local file: "
"""Prefix of the file text of a stored attachment (`claude_code_runner.attachments`)."""

NOT_CONFIGURED = "transcriber_not_configured"
"""Problem code when ``runner.transcribe_cmd`` is empty."""


def is_audio(mime_type: str) -> bool:
    """Whether a MIME type is audio (a voice message or an audio attachment).

    Args:
        mime_type: The attachment's ``mime_type``.

    Returns:
        True for ``audio/*``.
    """
    return mime_type.strip().lower().startswith("audio/")


@dataclass(frozen=True)
class Untranscribed:
    """An audio attachment that got no transcript.

    Attributes:
        file_id: The file.
        name: Its ``original_name``.
        problem: ``transcriber_not_configured``, ``transcriber_unreachable``,
            ``transcriber_failed``, ``transcriber_timeout`` or ``transcript_empty``.
    """

    file_id: UUID
    name: str
    problem: str


@dataclass(frozen=True)
class Transcriber:
    """The configured transcriber.

    Attributes:
        argv: ``runner.transcribe_cmd`` with the program resolved in the cleaned ``PATH``
            (empty: off; a program that was not found keeps its name and fails at the start).
        env: The allow-list environment.
        timeout: ``runner.transcribe_timeout`` in seconds.
    """

    argv: tuple[str, ...]
    env: Mapping[str, str]
    timeout: float

    async def run(self, path: Path) -> tuple[str | None, str | None]:
        """Transcribe one file.

        Args:
            path: The local audio file.

        Returns:
            The text and ``None``, or ``None`` and the problem code.
        """
        if not self.argv:
            return None, NOT_CONFIGURED
        try:
            proc = await asyncio.create_subprocess_exec(
                *self.argv,
                str(path),
                env=dict(self.env),
                stdin=asyncio.subprocess.DEVNULL,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.DEVNULL,
                start_new_session=True,
            )
        except OSError:
            return None, "transcriber_unreachable"
        try:
            out, _ = await asyncio.wait_for(proc.communicate(), self.timeout)
        except TimeoutError:
            _kill_group(proc.pid)
            await proc.wait()
            return None, "transcriber_timeout"
        except asyncio.CancelledError:
            _kill_group(proc.pid)
            await proc.wait()
            raise
        if proc.returncode != 0:
            _LOG.warning(
                json.dumps(
                    {"event": "transcriber_failed", "exit": proc.returncode}, separators=(",", ":")
                )
            )
            return None, "transcriber_failed"
        text = _cut(out.decode("utf-8", errors="replace").replace("\x00", "").strip())
        if not text:
            return None, "transcript_empty"
        return text, None


def _cut(text: str) -> str:
    """``text`` cut to `TRANSCRIPT_MAX_BYTES` UTF-8 bytes at a character boundary."""
    data = text.encode("utf-8")
    if len(data) <= TRANSCRIPT_MAX_BYTES:
        return text
    return data[:TRANSCRIPT_MAX_BYTES].decode("utf-8", errors="ignore").rstrip()


def _kill_group(pid: int) -> None:
    with contextlib.suppress(ProcessLookupError, PermissionError):
        os.killpg(pid, signal.SIGKILL)


async def transcribe_attachments(
    attachments: Sequence[RunContextAttachment],
    file_texts: dict[UUID, str],
    transcriber: Transcriber,
    on_start: Callable[[], object] | None = None,
) -> tuple[list[RunContextAttachment], list[Untranscribed]]:
    """Transcribe the audio attachments of the triggering message that need it.

    An attachment counts when it is audio, carries no transcript of the server and its file
    text says it was stored locally. A transcript is set on the attachment in place; an
    attachment without one gets ``not transcribed: <code>`` added to its file text.

    Args:
        attachments: ``trigger_message.attachments`` of the run context.
        file_texts: The file texts of `claude_code_runner.attachments.fetch_attachments`;
            changed in place.
        transcriber: The configured transcriber.
        on_start: Called once before the first transcription (the status line).

    Returns:
        The attachments that got a transcript and those that did not.
    """
    done: list[RunContextAttachment] = []
    missing: list[Untranscribed] = []
    started = False
    for item in attachments:
        text = file_texts.get(item.file_id, "")
        if not (is_audio(item.mime_type) and isinstance(item.transcript, Unset)):
            continue
        if not text.startswith(LOCAL_FILE):
            continue
        if transcriber.argv and not started and on_start is not None:
            started = True
            result = on_start()
            if asyncio.iscoroutine(result):
                await result
        transcript, problem = await transcriber.run(Path(text.removeprefix(LOCAL_FILE)))
        if transcript is not None:
            item.transcript = transcript
            done.append(item)
            _LOG.info(json.dumps({"event": "attachment_transcribed", "file_id": str(item.file_id)}))
            continue
        code = problem or "transcriber_failed"
        file_texts[item.file_id] = f"{text}\nnot transcribed: {code}"
        missing.append(Untranscribed(item.file_id, item.original_name, code))
        _LOG.info(
            json.dumps(
                {"event": "attachment_not_transcribed", "code": code, "file_id": str(item.file_id)}
            )
        )
    return done, missing


def notice(missing: Sequence[Untranscribed]) -> str:
    """The note to the person in the thread about audio Claude Code got only as a file.

    Args:
        missing: The attachments without a transcript (not empty).

    Returns:
        One short message in Markdown.
    """
    if all(m.problem == NOT_CONFIGURED for m in missing):
        return (
            "Voice message not transcribed: this runner has no transcriber configured "
            "(`runner.transcribe_cmd`), so Claude Code received only the audio file. "
            "Please send the request as text."
        )
    codes = ", ".join(sorted({f"`{m.problem}`" for m in missing}))
    return (
        f"Voice message not transcribed ({codes}), so Claude Code received only the audio "
        "file. Please send the request as text."
    )
