# SPDX-License-Identifier: Apache-2.0
"""The log of ``run``: one JSON object per line with time and level, no request lines of httpx."""

from __future__ import annotations

import io
import json
import logging
from collections.abc import Iterator

import pytest

from claude_code_runner.logformat import configure


@pytest.fixture
def stream() -> Iterator[io.StringIO]:
    root = logging.getLogger()
    saved = (root.handlers[:], root.level)
    levels = {n: logging.getLogger(n).level for n in ("httpx", "httpcore", "websockets")}
    out = io.StringIO()
    configure(out)
    yield out
    root.handlers[:] = saved[0]
    root.setLevel(saved[1])
    for name, level in levels.items():
        logging.getLogger(name).setLevel(level)


def _lines(out: io.StringIO) -> list[dict[str, object]]:
    return [json.loads(line) for line in out.getvalue().splitlines()]


def test_every_line_is_json_with_time_and_level(stream: io.StringIO) -> None:
    logging.getLogger("claude_code_runner").info(json.dumps({"event": "turn_started", "run": "r1"}))
    logging.getLogger("bothy_sdk").warning("plain text of a library")
    first, second = _lines(stream)
    assert first["event"] == "turn_started"
    assert first["run"] == "r1"
    assert first["level"] == "info"
    assert first["logger"] == "claude_code_runner"
    assert str(first["ts"]).endswith("Z")
    assert second["level"] == "warning"
    assert second["logger"] == "bothy_sdk"
    assert second["message"] == "plain text of a library"
    assert "event" not in second


def test_requests_of_the_http_client_are_not_logged(stream: io.StringIO) -> None:
    logging.getLogger("httpx").info('HTTP Request: GET https://chat.example.com/api/v1/x "200 OK"')
    logging.getLogger("httpcore.connection").info("connect_tcp.started")
    logging.getLogger("httpx").warning("a warning of the client stays")
    lines = _lines(stream)
    assert [line["message"] for line in lines] == ["a warning of the client stays"]


def test_an_exception_stays_on_one_line(stream: io.StringIO) -> None:
    try:
        raise ValueError("boom")
    except ValueError:
        logging.getLogger("claude_code_runner").exception(json.dumps({"event": "stop_failed"}))
    (line,) = _lines(stream)
    assert line["event"] == "stop_failed"
    assert line["level"] == "error"
    assert "ValueError" in str(line["traceback"])
