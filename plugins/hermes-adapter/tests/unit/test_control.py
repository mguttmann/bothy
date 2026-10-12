# SPDX-License-Identifier: Apache-2.0
"""The control channel between the harness and the probe plugin."""

from __future__ import annotations

import http.client
import json
import threading
import time
from collections.abc import Iterator
from typing import Any

import pytest
from hermes_harness.control import ControlServer


@pytest.fixture
def control() -> Iterator[ControlServer]:
    with ControlServer() as server:
        yield server


def _request(control: ControlServer, method: str, path: str, body: Any = None) -> tuple[int, Any]:
    conn = http.client.HTTPConnection("127.0.0.1", control.port, timeout=10)
    try:
        payload = json.dumps(body) if body is not None else None
        conn.request(method, path, payload, {"Content-Type": "application/json"})
        response = conn.getresponse()
        raw = response.read()
        return response.status, json.loads(raw) if raw else None
    finally:
        conn.close()


def test_calls_are_kept_in_arrival_order(control: ControlServer) -> None:
    for i in range(5):
        _request(control, "POST", "/call", {"method": f"m{i}", "args": {"i": i}})
    calls = control.calls()
    assert [c["method"] for c in calls] == ["m0", "m1", "m2", "m3", "m4"]
    assert [c["seq"] for c in calls] == [1, 2, 3, 4, 5]
    assert calls[2]["args"] == {"i": 2}
    assert control.mark() == 5


def test_commands_are_served_once_in_order(control: ControlServer) -> None:
    first = control.push("inject", text="one")
    second = control.push("describe")
    assert _request(control, "GET", "/command")[1] == {
        "id": first,
        "name": "inject",
        "args": {"text": "one"},
    }
    assert _request(control, "GET", "/command")[1]["id"] == second
    assert _request(control, "GET", "/command") == (204, None)
    _request(control, "POST", "/result", {"id": second, "result": {"ok": True}})
    assert control.result(second, deadline_s=1) == {"ok": True}


def test_result_fails_after_the_named_deadline(control: ControlServer) -> None:
    command = control.push("describe")
    with pytest.raises(TimeoutError, match="describe"):
        control.result(command, deadline_s=0.2)


def test_wait_for_returns_the_first_matching_call_after_a_mark(control: ControlServer) -> None:
    _request(control, "POST", "/call", {"method": "send", "args": {"content": "early"}})
    mark = control.mark()

    def later() -> None:
        time.sleep(0.2)
        _request(control, "POST", "/call", {"method": "send", "args": {"content": "skip"}})
        _request(control, "POST", "/call", {"method": "send", "args": {"content": "late"}})

    worker = threading.Thread(target=later)
    worker.start()
    call = control.wait_for(
        "send", lambda args: args.get("content") == "late", after=mark, deadline_s=10
    )
    worker.join()
    assert call["args"]["content"] == "late"
    assert control.wait_for("send", deadline_s=1)["args"]["content"] == "early"


def test_wait_for_fails_after_the_named_deadline(control: ControlServer) -> None:
    with pytest.raises(TimeoutError, match="connect"):
        control.wait_for("connect", deadline_s=0.2)


def test_malformed_posts_are_rejected(control: ControlServer) -> None:
    assert _request(control, "POST", "/call", ["not", "an", "object"])[0] == 400
    assert _request(control, "POST", "/result", {"id": "unknown", "result": 1})[0] == 404
    assert _request(control, "GET", "/elsewhere")[0] == 404
    assert control.calls() == []
