# SPDX-License-Identifier: Apache-2.0
"""The control protocol of the Fake-Agent (brief M2-31 Design note 5 a)."""

from __future__ import annotations

import asyncio
import json
from datetime import UTC, datetime

import pytest

from bothy_sdk.testing.control import (
    ControlError,
    ErrorKind,
    Reply,
    encode_reply,
    format_now,
    parse_request,
)
from bothy_sdk.testing.fake_agent import _serve

pytestmark = pytest.mark.anyio


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def test_request_and_reply_round_trip() -> None:
    now = datetime(2030, 1, 2, 3, 4, 5, 678000, tzinfo=UTC)
    line = json.dumps(
        {"id": 7, "op": "receive", "args": {"type": "agent.invocation"}, "now": format_now(now)}
    )
    req = parse_request(line)
    assert (req.id, req.op, req.args, req.now) == (7, "receive", {"type": "agent.invocation"}, now)
    assert json.loads(encode_reply(Reply.success(7, {"none": True}))) == {
        "id": 7,
        "ok": True,
        "result": {"none": True},
    }
    failure = json.loads(encode_reply(Reply.failure(7, ErrorKind.TRANSPORT, "closed")))
    assert failure == {"id": 7, "ok": False, "error": {"kind": "transport", "detail": "closed"}}


@pytest.mark.parametrize(
    "line",
    [
        "not json",
        "[1]",
        '{"op": "connect"}',
        '{"id": 0, "op": "connect"}',
        '{"id": true, "op": "connect"}',
        '{"id": 1, "op": ""}',
        '{"id": 1, "op": "connect", "args": []}',
        '{"id": 1, "op": "connect", "now": "yesterday"}',
    ],
)
def test_malformed_requests_are_usage_errors(line: str) -> None:
    with pytest.raises(ControlError) as caught:
        parse_request(line)
    assert caught.value.kind is ErrorKind.USAGE


async def _run_lines(lines: list[str]) -> list[dict[str, object]]:
    reader = asyncio.StreamReader()
    for line in lines:
        reader.feed_data((line + "\n").encode())
    reader.feed_eof()
    out: list[str] = []
    await _serve(reader, out.append)
    return [json.loads(x) for x in out]


async def test_errors_keep_the_process_serving() -> None:
    replies = await _run_lines(
        [
            "garbage",
            json.dumps({"id": 1, "op": "connect"}),
            json.dumps({"id": 2, "op": "dance"}),
            json.dumps({"id": 3, "op": "hello", "args": {}}),
            json.dumps({"id": 4, "op": "receive"}),
        ]
    )
    assert [r["id"] for r in replies] == [0, 1, 2, 3, 4]
    assert all(r["ok"] is False for r in replies)
    assert [r["error"]["kind"] for r in replies] == ["usage"] * 5  # type: ignore[index]
