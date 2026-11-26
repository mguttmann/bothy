# SPDX-License-Identifier: Apache-2.0
"""The backend drives the fixture app in Xvfb through the channel (CU-03 acceptance, CU-T24 base).

The test plays the host service: it opens the endpoint, admits the backend, and sends capture,
do, position, release and hide like internal/computer does. It needs Xvfb, python-xlib, Pillow
and Tk; without them it is skipped with the missing part named (the Compose image of CU-06 has
them; a maintainer machine without an X server has not).
"""

from __future__ import annotations

import importlib.util
import io
import json
import os
import pathlib
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time
import unittest
from typing import Any

HERE = pathlib.Path(__file__).resolve().parents[1]
sys.path.insert(0, str(HERE))

import backend

DISPLAY = ":97"
WAIT = 15.0


def missing() -> str:
    """Return the first missing dependency, or ""."""
    if sys.platform != "linux":
        return "not Linux"
    if shutil.which("Xvfb") is None:
        return "Xvfb"
    for mod in ("Xlib", "PIL", "tkinter"):
        if importlib.util.find_spec(mod) is None:
            return mod
    return ""


@unittest.skipIf(missing() != "", f"needs {missing()}")
class XvfbTest(unittest.TestCase):
    """End to end through the channel against the fixture app."""

    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        tmp = pathlib.Path(self.tmp.name)
        self.events = tmp / "events.jsonl"
        env = dict(os.environ, DISPLAY=DISPLAY)
        self.procs: list[subprocess.Popen[bytes]] = []
        self.procs.append(
            subprocess.Popen(
                ["Xvfb", DISPLAY, "-screen", "0", "1024x768x24", "-nolisten", "tcp"]
            )
        )
        self._wait(
            lambda: pathlib.Path(f"/tmp/.X11-unix/X{DISPLAY[1:]}").exists(), "Xvfb"
        )
        self.procs.append(
            subprocess.Popen(
                [sys.executable, str(HERE / "fixture_app.py"), str(self.events)],
                env=env,
            )
        )
        self._wait(lambda: self._events_with("ready"), "fixture app")
        endpoint = str(tmp / "computer.sock")
        self.listener = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.listener.bind(endpoint)
        self.listener.listen(1)
        self.backend = subprocess.Popen(
            [
                sys.executable,
                str(HERE / "backend.py"),
                "--endpoint",
                endpoint,
                "--generation",
                "3",
            ],
            env=env,
        )
        self.procs.append(self.backend)
        self.listener.settimeout(WAIT)
        self.conn, _ = self.listener.accept()
        self.conn.settimeout(WAIT)
        self.reader = self.conn.makefile("rb")
        self.seq = 0

    def tearDown(self) -> None:
        self.conn.close()
        self.listener.close()
        for p in reversed(self.procs):
            p.terminate()
            try:
                p.wait(5)
            except subprocess.TimeoutExpired:
                p.kill()
                p.wait()
        self.tmp.cleanup()

    def _wait(self, cond: Any, what: str) -> None:
        deadline = time.monotonic() + WAIT
        while time.monotonic() < deadline:
            if cond():
                return
            time.sleep(0.05)
        self.fail(f"{what} did not come up; last events {self._events()[-5:]}")

    def _events(self) -> list[dict[str, str]]:
        if not self.events.exists():
            return []
        return [
            json.loads(line)
            for line in self.events.read_text(encoding="utf-8").splitlines()
            if line
        ]

    def _events_with(self, event: str) -> bool:
        return any(e["event"] == event for e in self._events())

    def recv(self) -> dict[str, Any]:
        kind, body = backend.read_frame(self.reader)
        self.assertEqual(kind, backend.KIND_JSON)
        return json.loads(body)

    def send(self, obj: dict[str, Any]) -> None:
        self.conn.sendall(backend.encode_msg(obj))

    def request(self, obj: dict[str, Any]) -> tuple[dict[str, Any], bytes]:
        self.seq += 1
        obj["seq"] = self.seq
        self.send(obj)
        while True:
            m = self.recv()
            if m["op"] == "done":
                break
        self.assertEqual(m["seq"], self.seq)
        img = b""
        if "image" in m:
            kind, img = backend.read_frame(self.reader)
            self.assertEqual(kind, backend.KIND_IMAGE)
            self.assertEqual(len(img), m["image"]["bytes"])
        return m, img

    def click(self, x: int, y: int) -> dict[str, Any]:
        m, _ = self.request({"op": "do", "action": "left_click", "x": x, "y": y})
        return m

    def test_drive_fixture_app(self) -> None:
        from PIL import Image

        hello = self.recv()
        self.assertEqual(hello["op"], "hello")
        self.assertTrue(hello["enabled"])
        self.assertEqual(hello["generation"], 3)
        self.send(
            {
                "op": "show",
                "session_id": "0192f3a4-5b6c-7d8e-9f01-23456789abcd",
                "agent_name": "Builder",
            }
        )

        # Pixel check: the red square of the fixture app in a PNG of the whole screen.
        m, img = self.request(
            {"op": "capture", "max_edge": 1024, "format": "png", "quality": 100}
        )
        self.assertEqual(m["code"], "ok")
        self.assertEqual(
            m["screen"], {"x": 0, "y": 0, "width": 1024, "height": 768, "scale": 1}
        )
        pic = Image.open(io.BytesIO(img)).convert("RGB")
        self.assertEqual(pic.size, (1024, 768))
        self.assertEqual(pic.getpixel((330, 230)), (255, 0, 0))
        # A JPEG scaled to the long edge, and a zoom of a region.
        m, img = self.request(
            {"op": "capture", "max_edge": 512, "format": "jpeg", "quality": 80}
        )
        self.assertEqual((m["image"]["width"], m["image"]["height"]), (512, 384))
        self.assertEqual(Image.open(io.BytesIO(img)).format, "JPEG")
        m, img = self.request(
            {
                "op": "capture",
                "max_edge": 1280,
                "format": "png",
                "quality": 100,
                "region": {"x": 300, "y": 200, "width": 60, "height": 60},
            }
        )
        self.assertEqual(
            Image.open(io.BytesIO(img)).convert("RGB").getpixel((30, 30)), (255, 0, 0)
        )

        # Click Alpha, then Beta.
        self.assertEqual(self.click(80, 40)["code"], "ok")
        self.assertEqual(self.click(220, 40)["code"], "ok")
        self._wait(
            lambda: (
                [e.get("button") for e in self._events() if e["event"] == "click"]
                == ["Alpha", "Beta"]
            ),
            "clicks",
        )

        # Type Unicode text into the field, whatever the layout.
        self.click(200, 115)
        m, _ = self.request({"op": "do", "action": "type", "text": "naïve café"})
        self.assertEqual(m["code"], "ok")
        self._wait(
            lambda: any(e.get("value") == "naïve café" for e in self._events()),
            "typed text",
        )
        m, _ = self.request(
            {"op": "do", "action": "key", "keys": "BackSpace", "repeat": 2}
        )
        self._wait(
            lambda: any(e.get("value") == "naïve ca" for e in self._events()),
            "key with repeat",
        )
        self.assertEqual(m["code"], "ok")

        # The pointer position after a move.
        self.request({"op": "do", "action": "mouse_move", "x": 123, "y": 45})
        m, _ = self.request({"op": "position"})
        self.assertEqual(m["cursor"], {"x": 123, "y": 45})

        # A simulated lock: state locked, input refused with screen_locked, unlock resumes.
        self.backend.send_signal(signal.SIGUSR1)
        self.assertEqual(self.recv(), {"op": "state", "locked": True})
        before = len(self._events())
        self.assertEqual(self.click(80, 40)["code"], "screen_locked")
        time.sleep(0.3)
        self.assertEqual(len(self._events()), before)
        self.backend.send_signal(signal.SIGUSR2)
        self.assertEqual(self.recv(), {"op": "state", "locked": False})
        self.assertEqual(self.click(80, 40)["code"], "ok")

        # The stop button of the indicator, then release and hide.
        self.backend.send_signal(signal.SIGHUP)
        self.assertEqual(
            self.recv(),
            {
                "op": "stop",
                "session_id": "0192f3a4-5b6c-7d8e-9f01-23456789abcd",
                "detail": "indicator_stop",
            },
        )
        self.send({"op": "release"})
        self.send({"op": "hide", "session_id": "0192f3a4-5b6c-7d8e-9f01-23456789abcd"})
        # Disable switches the enablement off.
        self.send({"op": "disable", "generation": 3})
        self.assertEqual(self.recv(), {"op": "state", "enabled": False})


if __name__ == "__main__":
    unittest.main()
