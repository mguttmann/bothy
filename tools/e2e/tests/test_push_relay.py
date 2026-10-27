# SPDX-License-Identifier: Apache-2.0
"""The push relay of the mobile E2E run (tools/e2e/push_relay.py, brief M5-11 Design note 2)."""

from __future__ import annotations

import base64
import json
import threading
from collections.abc import Iterator, Sequence
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from typing import Any, ClassVar

import push_relay
import pytest

APNS = {
    "seq": 1,
    "provider": "apns",
    "token": "ab" * 32,
    "topic": "com.example.chat",
    "push_type": "alert",
    "priority": "10",
    "payload": {"aps": {"alert": {"title": "New message"}, "mutable-content": 1}, "v": 1},
}
FCM = {"seq": 2, "provider": "fcm", "token": "e2e-token", "data": {"v": "1", "kind": "hint"}}


class Recorder:
    """Records the commands the relay runs and answers each with rc."""

    def __init__(self, rc: int = 0) -> None:
        self.rc = rc
        self.calls: list[list[str]] = []
        self.files: list[dict[str, Any]] = []

    def __call__(self, cmd: Sequence[str]) -> int:
        self.calls.append(list(cmd))
        if cmd[:3] == ["xcrun", "simctl", "push"]:
            with Path(cmd[-1]).open(encoding="utf-8") as f:
                self.files.append(json.load(f))
        return self.rc


def test_ios_delivers_the_apns_body_with_the_target_bundle() -> None:
    rec = Recorder()
    relay = push_relay.Relay("ios", "UDID-1", run=rec)
    relay.handle([APNS, FCM])
    assert len(rec.calls) == 1
    assert rec.calls[0][:5] == ["xcrun", "simctl", "push", "UDID-1", "com.example.chat"]
    assert rec.files == [{**APNS["payload"], "Simulator Target Bundle": "com.example.chat"}]
    assert (relay.after, relay.delivered, relay.failed) == (2, 1, 0)


def test_android_broadcasts_the_fcm_data_as_base64_json() -> None:
    rec = Recorder()
    relay = push_relay.Relay("android", "emulator-5560", run=rec)
    relay.handle([APNS, FCM])
    assert len(rec.calls) == 1
    cmd = rec.calls[0]
    assert cmd[:6] == ["adb", "-s", "emulator-5560", "shell", "am", "broadcast"]
    assert cmd[cmd.index("-a") + 1] == "com.example.chat.E2E_PUSH"
    assert cmd[cmd.index("-n") + 1] == "com.example.chat/com.example.client.push.E2ePushReceiver"
    assert cmd[cmd.index("--es") + 1] == "data_b64"
    assert json.loads(base64.b64decode(cmd[-1])) == FCM["data"]


def test_ios_skips_a_capture_without_an_alert(capsys: pytest.CaptureFixture[str]) -> None:
    # A badge sync (13 6.1) has no alert; the simulator refuses to deliver it (UNErrorDomain
    # 2002), so it is reported as skipped, not as a failed delivery.
    rec = Recorder()
    relay = push_relay.Relay("ios", "UDID-1", run=rec)
    relay.handle([{**APNS, "seq": 3, "priority": "5", "payload": {"aps": {"badge": 0}}}])
    assert rec.calls == []
    assert (relay.after, relay.delivered, relay.failed) == (3, 0, 0)
    assert "skipped apns #3" in capsys.readouterr().out


def test_a_capture_is_delivered_once() -> None:
    rec = Recorder()
    relay = push_relay.Relay("android", "emulator-5560", run=rec)
    relay.handle([FCM])
    relay.handle([FCM])
    assert len(rec.calls) == 1


def test_a_failed_delivery_is_counted_and_the_relay_goes_on() -> None:
    rec = Recorder(rc=1)
    relay = push_relay.Relay("android", "emulator-5560", run=rec)
    relay.handle([FCM, {**FCM, "seq": 3}])
    assert (relay.delivered, relay.failed, relay.after) == (0, 2, 3)


def test_a_malformed_capture_is_a_failure_not_a_crash(capsys: pytest.CaptureFixture[str]) -> None:
    rec = Recorder()
    relay = push_relay.Relay("android", "emulator-5560", run=rec)
    relay.handle([{"seq": 4, "provider": "fcm", "data": {"v": 1}}])
    relay2 = push_relay.Relay("ios", "UDID-1", run=rec)
    relay2.handle([{"seq": 5, "provider": "apns"}])
    assert rec.calls == []
    assert (relay.failed, relay2.failed) == (1, 1)
    assert "e2e-token" not in capsys.readouterr().out


def test_the_output_names_no_payload_or_token(capsys: pytest.CaptureFixture[str]) -> None:
    relay = push_relay.Relay("ios", "UDID-1", run=Recorder())
    relay.handle([APNS])
    out = capsys.readouterr().out
    assert "delivered apns #1" in out
    assert APNS["token"] not in out
    assert "New message" not in out


class _Capture(BaseHTTPRequestHandler):
    items: ClassVar[list[dict[str, Any]]] = []
    queries: ClassVar[list[str]] = []

    def do_GET(self) -> None:
        type(self).queries.append(self.path)
        after = int(self.path.split("after=")[1]) if "after=" in self.path else 0
        body = json.dumps({"items": [i for i in type(self).items if i["seq"] > after]}).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format: str, *args: object) -> None:
        return


@pytest.fixture
def capture() -> Iterator[str]:
    handler = type("Capture", (_Capture,), {"items": [FCM, APNS], "queries": []})
    srv = HTTPServer(("127.0.0.1", 0), handler)
    t = threading.Thread(target=srv.serve_forever, daemon=True)
    t.start()
    try:
        yield f"http://127.0.0.1:{srv.server_address[1]}"
    finally:
        srv.shutdown()
        srv.server_close()
        t.join()


def test_fetch_reads_the_capture_api_in_order(capture: str) -> None:
    items = push_relay.fetch(capture, 0)
    assert [i["seq"] for i in items] == [1, 2]
    assert [i["seq"] for i in push_relay.fetch(capture, 1)] == [2]


def test_fetch_refuses_a_non_loopback_endpoint() -> None:
    with pytest.raises(ValueError, match="loopback"):
        push_relay.fetch("http://fakepush:8090", 0)


def test_main_requires_the_device_of_the_platform() -> None:
    with pytest.raises(SystemExit):
        push_relay.main(["--capture-url", "http://127.0.0.1:1", "--platform", "ios"])
