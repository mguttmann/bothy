# SPDX-License-Identifier: Apache-2.0
"""The egress recorder refuses every proxied request and records only the destination."""

from __future__ import annotations

import http.client
import socket
from collections.abc import Iterator

import pytest
from hermes_harness.egress import EgressRecorder


@pytest.fixture
def recorder() -> Iterator[EgressRecorder]:
    with EgressRecorder() as rec:
        yield rec


def _request(rec: EgressRecorder, method: str, target: str) -> int:
    conn = http.client.HTTPConnection("127.0.0.1", rec.port, timeout=10)
    try:
        conn.request(method, target, headers={"Host": target.rsplit("/", maxsplit=1)[-1]})
        return conn.getresponse().status
    finally:
        conn.close()


def test_connect_and_absolute_uri_are_refused_and_recorded(
    recorder: EgressRecorder, monkeypatch: pytest.MonkeyPatch
) -> None:
    opened: list[object] = []
    real = socket.create_connection

    def spy(address: object, *args: object, **kwargs: object) -> socket.socket:
        opened.append(address)
        return real(address, *args, **kwargs)  # type: ignore[arg-type]

    monkeypatch.setattr(socket, "create_connection", spy)
    recorder.phase = "turn"
    assert _request(recorder, "CONNECT", "example.com:443") == 403
    assert _request(recorder, "GET", "http://example.com/x") == 403
    assert _request(recorder, "POST", "http://example.com:8443/v1/messages?k=v") == 403
    # The only connections are the test's own ones to the recorder.
    assert set(opened) == {("127.0.0.1", recorder.port)}
    assert recorder.seen() == [
        ("example.com:443", "turn"),
        ("example.com:80", "turn"),
        ("example.com:8443", "turn"),
    ]


def test_origin_form_requests_are_refused_without_a_destination(recorder: EgressRecorder) -> None:
    assert _request(recorder, "GET", "/local") == 403
    assert recorder.seen() == [("<origin-form>", "start")]


def test_recorder_binds_loopback_port_zero_and_names_its_url(recorder: EgressRecorder) -> None:
    assert recorder.port > 0
    assert recorder.url == f"http://127.0.0.1:{recorder.port}"
    assert "example" not in repr(recorder)
