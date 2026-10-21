# SPDX-License-Identifier: Apache-2.0
"""Tailing the stream file (design note 12)."""

from __future__ import annotations

import asyncio
from pathlib import Path

import pytest

from claude_code_runner.streamreader import StreamReader


async def collect(reader: StreamReader) -> list[dict[str, object]]:
    return [m async for m in reader.messages()]


@pytest.mark.anyio
async def test_partial_invalid_unknown_and_end_after_result(tmp_path: Path) -> None:
    path = tmp_path / "s.jsonl"
    path.write_text("")
    alive = True
    reader = StreamReader(path, 0.01, lambda: alive)
    task = asyncio.ensure_future(collect(reader))
    with path.open("a") as fh:
        fh.write('{"type":"system","subtype":"init"}\n{"type":"assis')
        fh.flush()
        await asyncio.sleep(0.05)
        fh.write('tant"}\nnot json\n[1, 2]\n{"type":"weird"}\n\n')
        fh.write('{"type":"result","subtype":"success"}\n{"type":"after"}\n')
    messages = await asyncio.wait_for(task, 5)
    assert [m["type"] for m in messages] == ["system", "assistant", "result"]


@pytest.mark.anyio
async def test_end_after_exit(tmp_path: Path) -> None:
    path = tmp_path / "s.jsonl"
    path.write_text('{"type":"assistant"}\n{"type":"user"')
    reader = StreamReader(path, 0.01, lambda: False)
    messages = await asyncio.wait_for(collect(reader), 5)
    assert [m["type"] for m in messages] == ["assistant"]


@pytest.mark.anyio
async def test_missing_file_after_exit(tmp_path: Path) -> None:
    reader = StreamReader(tmp_path / "none.jsonl", 0.01, lambda: False)
    assert await asyncio.wait_for(collect(reader), 5) == []
