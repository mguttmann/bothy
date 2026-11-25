# SPDX-License-Identifier: Apache-2.0
"""Which attachments a run stores (FIX-TEAM-CHANNEL-ATTACHMENTS D-TCAT-6)."""

from __future__ import annotations

from pathlib import Path
from typing import Any
from uuid import uuid4

import pytest
from bothy_sdk.generated.models import RunContext, RunContextAttachment
from bothy_sdk.ids import uuid7

from claude_code_runner.attachments import run_attachments


def _file(name: str) -> dict[str, Any]:
    return {
        "file_id": str(uuid7()),
        "original_name": name,
        "mime_type": "image/png",
        "size_bytes": 3,
        "scan_status": "clean",
    }


def _message(seq: int, files: list[dict[str, Any]], **extra: Any) -> dict[str, Any]:
    author = {"id": str(uuid7()), "kind": "user", "handle": "alex", "display_name": "Alex"}
    return {
        "message_id": str(uuid7()),
        "thread_seq": seq,
        "created_at": "2026-11-22T10:00:00Z",
        "author": author,
        "role": "user",
        "origin": "human",
        "kind": "text",
        "channel_id": str(uuid7()),
        "content_md": "x",
        "attachments": files,
        "trust": "authorized",
        **extra,
    }


def _context(*, channel: list[dict[str, Any]], thread: list[dict[str, Any]]) -> RunContext:
    trigger = _message(9, [_file("trigger.png")])
    trigger["trust"] = "trigger"
    return RunContext.from_dict(
        {
            "run_id": str(uuid7()),
            "session": {"mode": "new"},
            "context_floor_seq": 0,
            "trigger_message": trigger,
            "thread_context": thread,
            "channel_context": channel,
        }
    )


def test_a_team_run_takes_the_new_files_of_its_thread_context_only() -> None:
    # The main thread of a text home channel (channel_context) holds no team input; its files stay
    # out, as do files the session already holds.
    ctx = _context(
        channel=[_message(1, [_file("home-history.png")])],
        thread=[
            _message(2, [_file("context-input.png")]),
            _message(3, [_file("seen.png")], already_delivered=True),
        ],
    )
    names = [a.original_name for a in run_attachments(ctx, True)]
    assert names == ["trigger.png", "context-input.png"]
    assert [a.original_name for a in run_attachments(ctx, False)] == ["trigger.png"]


@pytest.mark.anyio
@pytest.mark.parametrize(
    ("status", "text"),
    [
        ("infected", "unavailable: file_blocked"),
        ("error", "unavailable: file_blocked"),
        ("pending", "unavailable: file_not_ready"),
    ],
)
async def test_a_trigger_attachment_the_scan_refused_is_blocked(
    tmp_path: Path, status: str, text: str
) -> None:
    """The server sends the raw scan status: infected and error never become ready."""
    from claude_code_runner.attachments import fetch_attachments

    item = RunContextAttachment(
        file_id=uuid4(),
        original_name="a.txt",
        mime_type="text/plain",
        size_bytes=1,
        scan_status=status,
    )
    texts, stored = await fetch_attachments(None, [item], tmp_path)  # type: ignore[arg-type]
    assert texts == {item.file_id: text}
    assert stored == 0
