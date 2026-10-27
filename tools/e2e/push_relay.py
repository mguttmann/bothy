# SPDX-License-Identifier: Apache-2.0
"""Push relay of the mobile E2E run (brief M5-11 Design note 2; docs/decisions.d/M5-11.md).

The server of the Compose stack sends its pushes to the fake providers of the service
``fakepush`` (infra/compose/test/compose.e2e-mobile.yaml), which capture every send. This relay
runs on the host next to one device while its suite runs: it polls the capture endpoint of
fakepush on the loopback and hands every new capture of its provider to the device the way the
real provider would deliver it, so the rest of the path is the real one (the Notification Service
Extension or the Android preview worker fetches the text from the real server):

- iOS (``apns``): ``xcrun simctl push <udid> <bundle> <file>`` with the captured APNs body plus
  ``Simulator Target Bundle``;
- Android (``fcm``): an explicit broadcast through ``adb shell am broadcast`` to the debug-only
  receiver ``E2ePushReceiver`` with the captured ``data`` as base64 of its JSON object.

It prints one line per delivery with the provider and sequence number, never a payload or a
token. It runs until SIGTERM or SIGINT; ``mobile_e2e.py`` starts and stops it by PID.

Usage:
    push_relay.py --capture-url http://127.0.0.1:PORT --platform ios --udid UDID
    push_relay.py --capture-url http://127.0.0.1:PORT --platform android --serial SERIAL
"""

from __future__ import annotations

import argparse
import base64
import json
import os
import signal
import subprocess
import sys
import tempfile
import time
import urllib.parse
import urllib.request
from collections.abc import Callable, Sequence
from dataclasses import dataclass
from pathlib import Path
from typing import Any

# The bundle ID and package name of the neutral branding (apps/client/pubspec.yaml patrol).
APP_ID = "com.example.chat"
# The debug-only receiver of the Android E2E build (apps/client/android, brief M5-11).
RECEIVER = "com.example.client.push.E2ePushReceiver"
# The provider each platform receives from.
PROVIDER = {"ios": "apns", "android": "fcm"}
# The time between two polls of the capture endpoint. It is a poll interval, not a bound: the
# suites wait for the notification with their own patience.
POLL_INTERVAL_S = 0.25

Runner = Callable[[Sequence[str]], int]


def _run(cmd: Sequence[str]) -> int:
    """Runs cmd without a shell and returns its exit code; output goes to this process's."""
    return subprocess.run(list(cmd), check=False).returncode


def fetch(capture_url: str, after: int) -> list[dict[str, Any]]:
    """Returns the captures with a sequence number above after, in increasing order.

    Raises:
        OSError: the endpoint is not reachable (urllib.error.URLError is one).
        ValueError: the answer is not the JSON object of the capture API.
    """
    query = urllib.parse.urlencode({"after": after})
    url = f"{capture_url.rstrip('/')}/captured?{query}"
    if not url.startswith(("http://127.0.0.1:", "http://localhost:")):
        raise ValueError("the capture endpoint must be on the loopback")
    # Loopback http only, checked above.
    with urllib.request.urlopen(url, timeout=10) as resp:  # noqa: S310  # nosec B310
        body = json.loads(resp.read().decode("utf-8"))
    items = body.get("items") if isinstance(body, dict) else None
    if not isinstance(items, list):
        raise ValueError("the capture endpoint did not answer with an items list")
    return sorted((i for i in items if isinstance(i, dict)), key=lambda i: int(i.get("seq", 0)))


def _alerts(item: dict[str, Any]) -> bool:
    """Whether the captured APNs body asks for an alert (a hint does, a badge sync does not)."""
    payload = item.get("payload")
    aps = payload.get("aps") if isinstance(payload, dict) else None
    return not isinstance(aps, dict) or "alert" in aps


def ios_command(udid: str, path: str, bundle: str = APP_ID) -> list[str]:
    """The simctl call that delivers the APNs body in path to the app bundle on simulator udid."""
    return ["xcrun", "simctl", "push", udid, bundle, path]


def ios_payload(item: dict[str, Any], bundle: str = APP_ID) -> dict[str, Any]:
    """The file content of simctl push: the captured APNs body plus the target bundle.

    Raises:
        ValueError: the capture has no APNs body.
    """
    payload = item.get("payload")
    if not isinstance(payload, dict):
        raise ValueError("an apns capture without a payload object")
    return {**payload, "Simulator Target Bundle": bundle}


def android_command(serial: str, item: dict[str, Any], package: str = APP_ID) -> list[str]:
    """The adb call that hands the captured FCM data of item to the debug-only receiver.

    Raises:
        ValueError: the capture has no data object of strings.
    """
    data = item.get("data")
    if not isinstance(data, dict) or not all(
        isinstance(k, str) and isinstance(v, str) for k, v in data.items()
    ):
        raise ValueError("an fcm capture without a data object of strings")
    encoded = base64.b64encode(json.dumps(data, separators=(",", ":")).encode("utf-8")).decode(
        "ascii"
    )
    return [
        "adb", "-s", serial, "shell", "am", "broadcast",
        "-a", f"{package}.E2E_PUSH", "-n", f"{package}/{RECEIVER}",
        "--es", "data_b64", encoded,
    ]  # fmt: skip


@dataclass
class Relay:
    """Delivers the captures of one provider to one device.

    Attributes:
        platform: ``ios`` or ``android``.
        device: the simulator UDID or the adb serial.
        run: runs a command and returns its exit code (a seam for the tests).
        after: the sequence number of the last capture seen.
        delivered: how many captures were delivered.
        failed: how many deliveries failed.
    """

    platform: str
    device: str
    run: Runner = _run
    after: int = 0
    delivered: int = 0
    failed: int = 0

    def handle(self, items: list[dict[str, Any]]) -> None:
        """Delivers every capture of this platform's provider in items and advances ``after``."""
        for item in items:
            seq = int(item.get("seq", 0))
            if seq <= self.after:
                continue
            self.after = seq
            provider = item.get("provider")
            if provider != PROVIDER[self.platform]:
                continue
            if self.platform == "ios" and not _alerts(item):
                # A badge sync has no alert; the simulator refuses to deliver such a push.
                print(f"relay: skipped {provider} #{seq} (no alert)", flush=True)
                continue
            try:
                rc = self._deliver(item)
            except ValueError as err:
                print(f"relay: {provider} #{seq} not delivered: {err}", flush=True)
                self.failed += 1
                continue
            if rc == 0:
                self.delivered += 1
                print(f"relay: delivered {provider} #{seq}", flush=True)
            else:
                self.failed += 1
                print(f"relay: {provider} #{seq} delivery exited with {rc}", flush=True)

    def _deliver(self, item: dict[str, Any]) -> int:
        if self.platform == "android":
            return self.run(android_command(self.device, item))
        payload = ios_payload(item)
        fd, path = tempfile.mkstemp(prefix="push-", suffix=".json")
        try:
            with os.fdopen(fd, "w", encoding="utf-8") as f:
                json.dump(payload, f)
            return self.run(ios_command(self.device, path))
        finally:
            Path(path).unlink()


class _StopError(Exception):
    """Raised by the signal handler to end the loop."""


def _stop(_signum: int, _frame: object) -> None:
    raise _StopError


def main(argv: Sequence[str] | None = None) -> int:
    """Polls and delivers until SIGTERM or SIGINT; returns 1 if a delivery failed, else 0."""
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--capture-url", required=True)
    parser.add_argument("--platform", choices=sorted(PROVIDER), required=True)
    parser.add_argument("--udid", default="")
    parser.add_argument("--serial", default="")
    args = parser.parse_args(argv)
    device = args.udid if args.platform == "ios" else args.serial
    if not device:
        parser.error("--udid is required for ios, --serial for android")
    relay = Relay(args.platform, device)
    signal.signal(signal.SIGTERM, _stop)
    signal.signal(signal.SIGINT, _stop)
    print(f"relay: {args.platform} ready", flush=True)
    try:
        while True:
            try:
                relay.handle(fetch(args.capture_url, relay.after))
            except (OSError, ValueError) as err:
                # fakepush may restart with the stack; the next poll tries again.
                print(f"relay: capture endpoint not readable ({type(err).__name__})", flush=True)
            time.sleep(POLL_INTERVAL_S)
    except _StopError:
        pass
    print(
        f"relay: {args.platform} stopped, {relay.delivered} delivered, {relay.failed} failed",
        flush=True,
    )
    return 1 if relay.failed else 0


if __name__ == "__main__":
    sys.exit(main())
