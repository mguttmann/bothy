# SPDX-License-Identifier: Apache-2.0
"""Probe and native test of a computer helper on a hosted CI runner (brief CU-02).

Runs one helper (`services/host/desktop/macos` or `windows`, a development build) against a fake
host service on its own channel endpoint and a fixture window (`computer_fixture.py`), as the jobs
`computer probe` and `computer native` of `.github/workflows/host-platform.yml` need it:

1. the fixture window opens with a known color and an entry field and reports its geometry;
2. the enablement is written as on: the state file of the helper (D-CU-02-4) gets
   `enabled: true`, because the switch needs a click at the machine and a hosted runner has no
   person (this file is what the switch writes; the helper reads it at start only);
3. the helper starts with `BOTHY_COMPUTER_HELPER_CHANNEL` (D-CU-02-5, development builds only),
   connects and sends `hello`;
4. `show`, `capture` (PNG): the image shows the fixture's color at its center and no pixel of the
   indicator color (D-CU-02-9); `do left_click` on the entry, `do type`, `do key Return` and an
   injected `do key Escape`, each seen by the fixture, the Escape without a `stop` of the helper
   (CU-T14); `position`; `hide`.

The channel codec follows docs/spec/27-computer-use.md 9.4 and reads the vectors of
`services/host/testdata/computer/channel-vectors.json` (tests). Images stay in memory.

Usage:
  computer_probe.py --helper EXE --mode probe|native --out RESULT.json [--github-output FILE]

In `probe` mode the result is recorded and the exit code is 0 whatever the helper could do; the
outputs `capture` and `input` (true or false) go to `--github-output`. In `native` mode every
failed check makes the exit code 1. Exit code 2 is a usage error.
"""

from __future__ import annotations

import argparse
import contextlib
import importlib
import json
import os
import queue
import socket
import struct
import subprocess
import sys
import tempfile
import threading
import time
import uuid
import zlib
from collections.abc import Callable, Iterator
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

REPO = Path(__file__).resolve().parents[2]
FIXTURE = Path(__file__).resolve().parent / "computer_fixture.py"
# Frame types of the channel (27 9.4).
JSON_FRAME, IMAGE_FRAME = 1, 2
HEADER = struct.Struct(">I")
# The default of host_service.computer_frame_max_bytes (27 section 18); the fake host refuses
# longer frames like the host service.
FRAME_MAX = 64 * 1024 * 1024
# Seconds the probe waits for each step of a slow hosted runner.
CONNECT_WAIT = 60.0
ANSWER_WAIT = 30.0
FIXTURE_WAIT = 60.0
EVENT_WAIT = 10.0
SETTLE = 0.5
# The fixture's fill color and the indicator's border color (D-CU-02-9), as RGB.
FIXTURE_RGB = (0x2E, 0x8B, 0x57)
INDICATOR_RGB = (0xFF, 0x6A, 0x00)
COLOR_TOLERANCE = 24
TYPED = "Alex at example.com"


class ProbeError(Exception):
    """A step of the probe could not run (no connection, no answer, a malformed frame)."""


# ---- channel codec --------------------------------------------------------------------------


def encode(kind: int, content: bytes, limit: int = FRAME_MAX) -> bytes:
    """Return one channel frame: 4-byte big-endian length of type and content, type, content.

    Raises:
        ProbeError: the frame would be longer than `limit`.
    """
    if len(content) + 1 > limit:
        raise ProbeError(f"frame of {len(content) + 1} bytes exceeds {limit}")
    return HEADER.pack(len(content) + 1) + bytes([kind]) + content


def encode_json(obj: dict[str, Any]) -> bytes:
    """Return the JSON frame of `obj` (UTF-8, compact)."""
    return encode(JSON_FRAME, json.dumps(obj, separators=(",", ":")).encode("utf-8"))


def decode(buf: bytes, limit: int = FRAME_MAX) -> tuple[int, bytes, bytes]:
    """Split the first frame off `buf`; return type, content and the rest.

    Raises:
        ProbeError: `buf` holds no complete frame, an empty one or one longer than `limit`.
    """
    if len(buf) < HEADER.size:
        raise ProbeError("incomplete frame header")
    (length,) = HEADER.unpack_from(buf)
    if length == 0 or length > limit:
        raise ProbeError(f"frame length {length} outside 1 to {limit}")
    end = HEADER.size + length
    if len(buf) < end:
        raise ProbeError("incomplete frame")
    return buf[HEADER.size], buf[HEADER.size + 1 : end], buf[end:]


def decode_object(kind: int, content: bytes) -> dict[str, Any]:
    """Return the JSON object of a frame of type 1.

    Raises:
        ProbeError: another type or no JSON object.
    """
    if kind != JSON_FRAME:
        raise ProbeError(f"expected a JSON frame, got type {kind}")
    obj = json.loads(content.decode("utf-8"))
    if not isinstance(obj, dict):
        raise ProbeError("a JSON frame must carry an object")
    return obj


# ---- PNG ------------------------------------------------------------------------------------


@dataclass
class Image:
    """A decoded 8-bit RGB or RGBA image: `rows[y][x * channels + c]`."""

    width: int
    height: int
    channels: int
    rows: list[bytes]

    def pixel(self, x: int, y: int) -> tuple[int, int, int]:
        """Return the RGB value at (x, y)."""
        i = x * self.channels
        row = self.rows[y]
        return row[i], row[i + 1], row[i + 2]


def _paeth(a: int, b: int, c: int) -> int:
    p = a + b - c
    pa, pb, pc = abs(p - a), abs(p - b), abs(p - c)
    if pa <= pb and pa <= pc:
        return a
    return b if pb <= pc else c


def decode_png(data: bytes) -> Image:
    """Decode a non-interlaced 8-bit RGB or RGBA PNG (what the helpers write).

    Raises:
        ProbeError: not a PNG of that kind.
    """
    if data[:8] != b"\x89PNG\r\n\x1a\n":
        raise ProbeError("not a PNG")
    pos, idat, header = 8, b"", b""
    while pos < len(data):
        (size,) = struct.unpack_from(">I", data, pos)
        kind = data[pos + 4 : pos + 8]
        body = data[pos + 8 : pos + 8 + size]
        if kind == b"IHDR":
            header = body
        elif kind == b"IDAT":
            idat += body
        pos += 12 + size
    width, height, depth, color, _, _, interlace = struct.unpack(">IIBBBBB", header)
    channels = {2: 3, 6: 4}.get(color)
    if depth != 8 or channels is None or interlace != 0:
        raise ProbeError(f"unsupported PNG (depth {depth}, color type {color})")
    raw = zlib.decompress(idat)
    stride = width * channels
    rows: list[bytes] = []
    prev = bytearray(stride)
    for y in range(height):
        start = y * (stride + 1)
        kind = raw[start]
        line = bytearray(raw[start + 1 : start + 1 + stride])
        for i in range(stride):
            left = line[i - channels] if i >= channels else 0
            up = prev[i]
            corner = prev[i - channels] if i >= channels else 0
            if kind == 1:
                line[i] = (line[i] + left) & 0xFF
            elif kind == 2:
                line[i] = (line[i] + up) & 0xFF
            elif kind == 3:
                line[i] = (line[i] + (left + up) // 2) & 0xFF
            elif kind == 4:
                line[i] = (line[i] + _paeth(left, up, corner)) & 0xFF
        rows.append(bytes(line))
        prev = line
    return Image(width, height, channels, rows)


def close_to(rgb: tuple[int, int, int], want: tuple[int, int, int]) -> bool:
    """Whether `rgb` is within COLOR_TOLERANCE of `want` in every channel."""
    return all(abs(a - b) <= COLOR_TOLERANCE for a, b in zip(rgb, want, strict=True))


def count_color(image: Image, want: tuple[int, int, int]) -> int:
    """Count the pixels of `image` close to `want`."""
    found = 0
    for y in range(image.height):
        for x in range(image.width):
            if close_to(image.pixel(x, y), want):
                found += 1
    return found


def to_image(screen: dict[str, Any], image: Image, sx: float, sy: float) -> tuple[int, int]:
    """Map a point of the screen space of `done.screen` to a pixel of the captured image."""
    x = (sx - screen["x"]) * image.width / screen["width"]
    y = (sy - screen["y"]) * image.height / screen["height"]
    return min(max(int(x), 0), image.width - 1), min(max(int(y), 0), image.height - 1)


# ---- fake host service ----------------------------------------------------------------------


class Endpoint:
    """The fake host's end of the channel: a Unix socket or a Windows named pipe."""

    def __init__(self) -> None:
        """Start without a connection; subclasses set `name`."""
        self.name = ""
        self._buf = b""

    def accept(self, wait: float) -> None:
        """Wait up to `wait` seconds for the helper to connect."""
        raise NotImplementedError

    def read(self, n: int) -> bytes:
        """Read up to `n` bytes; b"" at the end of the connection."""
        raise NotImplementedError

    def write(self, data: bytes) -> None:
        """Write all of `data`."""
        raise NotImplementedError

    def close(self) -> None:
        """Close the endpoint."""
        raise NotImplementedError

    def frame(self) -> tuple[int, bytes]:
        """Read the next frame; return its type and content.

        Raises:
            ProbeError: the connection ended or the frame is malformed.
        """
        while True:
            with contextlib.suppress(ProbeError):
                kind, content, self._buf = decode(self._buf)
                return kind, content
            if len(self._buf) >= HEADER.size and HEADER.unpack_from(self._buf)[0] > FRAME_MAX:
                raise ProbeError("frame longer than the limit")
            chunk = self.read(1 << 16)
            if not chunk:
                raise ProbeError("the helper closed the channel")
            self._buf += chunk


class UnixEndpoint(Endpoint):
    """A Unix socket in a short temporary folder (macOS limits socket paths)."""

    def __init__(self) -> None:
        """Bind and listen; `name` is the socket path for the helper."""
        super().__init__()
        # mkdtemp makes a private folder; /tmp keeps the socket path short.
        self._dir = tempfile.mkdtemp(prefix="cu-", dir="/tmp")  # nosec B108
        self.name = str(Path(self._dir) / "c.sock")
        self._server = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self._server.bind(self.name)
        self._server.listen(1)
        self._conn: socket.socket | None = None

    def accept(self, wait: float) -> None:  # noqa: D102
        self._server.settimeout(wait)
        try:
            self._conn, _ = self._server.accept()
        except TimeoutError as err:
            raise ProbeError("the helper did not connect") from err
        self._conn.settimeout(ANSWER_WAIT)

    def read(self, n: int) -> bytes:  # noqa: D102
        if self._conn is None:
            raise ProbeError("not connected")
        try:
            return self._conn.recv(n)
        except TimeoutError as err:
            raise ProbeError("no answer from the helper") from err

    def write(self, data: bytes) -> None:  # noqa: D102
        if self._conn is None:
            raise ProbeError("not connected")
        self._conn.sendall(data)

    def close(self) -> None:  # noqa: D102
        if self._conn is not None:
            self._conn.close()
        self._server.close()
        with contextlib.suppress(OSError):
            Path(self.name).unlink()
        with contextlib.suppress(OSError):
            Path(self._dir).rmdir()


def pipe_mode(winapi: Any) -> int:
    """The mode of a byte pipe that waits (PIPE_TYPE_BYTE | PIPE_READMODE_BYTE | PIPE_WAIT).

    Python's _winapi does not export the two byte names; all three Win32 values are 0.
    """
    return (
        getattr(winapi, "PIPE_TYPE_BYTE", 0)
        | getattr(winapi, "PIPE_READMODE_BYTE", 0)
        | getattr(winapi, "PIPE_WAIT", 0)
    )


class PipeEndpoint(Endpoint):
    """A Windows named pipe in byte mode; blocking calls run in a thread to keep the waits."""

    def __init__(self) -> None:
        """Create the pipe; `name` is the pipe name for the helper."""
        super().__init__()
        # Windows only; typed as Any because the stubs gate the module by platform.
        self._w: Any = importlib.import_module("_winapi")
        _winapi = self._w
        self.name = rf"\\.\pipe\bothy-ci-{uuid.uuid4().hex}"
        self._handle = _winapi.CreateNamedPipe(
            self.name,
            _winapi.PIPE_ACCESS_DUPLEX,
            pipe_mode(_winapi),
            1,
            1 << 16,
            1 << 16,
            0,
            _winapi.NULL,
        )

    def _timed(self, call: Callable[[], Any], wait: float, what: str) -> Any:
        box: queue.Queue[tuple[bool, Any]] = queue.Queue()

        def run() -> None:
            try:
                box.put((True, call()))
            except OSError as err:
                box.put((False, err))

        threading.Thread(target=run, daemon=True).start()
        try:
            good, value = box.get(timeout=wait)
        except queue.Empty as err:
            raise ProbeError(what) from err
        if not good:
            raise ProbeError(f"{what}: {value}")
        return value

    def accept(self, wait: float) -> None:  # noqa: D102
        self._timed(
            lambda: self._w.ConnectNamedPipe(self._handle, False),
            wait,
            "the helper did not connect",
        )

    def read(self, n: int) -> bytes:  # noqa: D102
        try:
            data, _ = self._timed(
                lambda: self._w.ReadFile(self._handle, n, False), ANSWER_WAIT, "no answer"
            )
        except ProbeError as err:
            if "109" in str(err):  # ERROR_BROKEN_PIPE: the helper closed its end
                return b""
            raise
        return bytes(data)

    def write(self, data: bytes) -> None:  # noqa: D102
        self._w.WriteFile(self._handle, data, False)

    def close(self) -> None:  # noqa: D102
        with contextlib.suppress(OSError):
            self._w.CloseHandle(self._handle)


def new_endpoint() -> Endpoint:
    """Return the endpoint kind of this platform."""
    return PipeEndpoint() if sys.platform == "win32" else UnixEndpoint()


# ---- the run --------------------------------------------------------------------------------


@dataclass
class Result:
    """The checks of one run, by name: `ok` and a detail without screen content."""

    platform: str
    checks: dict[str, dict[str, Any]] = field(default_factory=dict)
    hello: dict[str, Any] = field(default_factory=dict)

    def check(self, name: str, ok: bool, detail: str = "") -> bool:
        """Record one check; return `ok`."""
        self.checks[name] = {"ok": ok, "detail": detail}
        return ok

    def passed(self, *names: str) -> bool:
        """Whether every named check is recorded and passed."""
        return all(self.checks.get(n, {}).get("ok") is True for n in names)


CAPTURE_CHECKS = ("connect", "show", "capture", "fixture_in_image", "indicator_not_in_image")
INPUT_CHECKS = ("click", "type", "key", "injected_escape_passes")


def state_file() -> Path:
    """The state file of the helper on this platform (D-CU-02-4)."""
    if sys.platform == "win32":
        base = Path(os.environ["LOCALAPPDATA"])
    else:
        base = Path.home() / "Library" / "Application Support"
    return base / "bothy-computer-helper" / "state.json"


def enable_helper() -> None:
    """Write the enablement as on, as the helper's switch would after a click (see above)."""
    path = state_file()
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    path.write_text(json.dumps({"v": 1, "enabled": True, "generation": 1}), encoding="utf-8")
    path.chmod(0o600)


class Fixture:
    """The fixture window process and its event log."""

    def __init__(self, events: Path) -> None:
        """Start the fixture and wait for its geometry line."""
        self.events = events
        self.proc = subprocess.Popen(
            [sys.executable, str(FIXTURE), "--events", str(events)],
            stdout=subprocess.PIPE,
            text=True,
        )
        out = self.proc.stdout
        if out is None:
            raise ProbeError("the fixture has no output")
        box: queue.Queue[str] = queue.Queue()
        threading.Thread(target=lambda: box.put(out.readline()), daemon=True).start()
        try:
            line = box.get(timeout=FIXTURE_WAIT)
        except queue.Empty as err:
            raise ProbeError("the fixture window did not open") from err
        self.geometry: dict[str, Any] = json.loads(line)

    def seen(self, test: Callable[[dict[str, Any]], bool]) -> dict[str, Any] | None:
        """Wait up to EVENT_WAIT for an event that passes `test`; return it or None."""
        deadline = time.monotonic() + EVENT_WAIT
        while time.monotonic() < deadline:
            if self.events.exists():
                for line in self.events.read_text(encoding="utf-8").splitlines():
                    event = json.loads(line)
                    if test(event):
                        return dict(event)
            time.sleep(SETTLE)
        return None

    def stop(self) -> None:
        """End the fixture process."""
        self.proc.terminate()
        with contextlib.suppress(subprocess.TimeoutExpired):
            self.proc.wait(timeout=EVENT_WAIT)


class Session:
    """The fake host's side of one helper connection."""

    def __init__(self, endpoint: Endpoint) -> None:
        """Wrap the connected `endpoint`."""
        self.ep = endpoint
        self.seq = 0
        self.stops: list[dict[str, Any]] = []

    def send(self, obj: dict[str, Any]) -> None:
        """Send one JSON frame."""
        self.ep.write(encode_json(obj))

    def answer(self, seq: int) -> tuple[dict[str, Any], bytes | None]:
        """Read frames until the `done` of `seq`; return it and its image bytes, if any."""
        while True:
            kind, content = self.ep.frame()
            obj = decode_object(kind, content)
            if obj.get("op") == "stop":
                self.stops.append(obj)
                continue
            if obj.get("op") != "done" or obj.get("seq") != seq:
                continue
            if "image" not in obj:
                return obj, None
            kind, data = self.ep.frame()
            if kind != IMAGE_FRAME or len(data) != obj["image"]["bytes"]:
                raise ProbeError("the image frame does not follow its done")
            return obj, data

    def request(self, op: str, **members: Any) -> tuple[dict[str, Any], bytes | None]:
        """Send `op` with the next seq and wait for its `done`."""
        self.seq += 1
        self.send({"op": op, "seq": self.seq, **members})
        return self.answer(self.seq)


def helper_command(helper: Path, channel: str) -> list[str]:
    """Return the command that starts `helper` with the fake host's endpoint.

    A macOS app bundle starts through LaunchServices (`open`), so the helper is its own
    responsible process for the screen recording and accessibility checks of the system, as it
    is under its LaunchAgent; a terminal or the runner would otherwise be asked instead.
    """
    if helper.suffix == ".app":
        return [
            "open",
            "-n",
            "-W",
            "--env",
            f"BOTHY_COMPUTER_HELPER_CHANNEL={channel}",
            str(helper),
        ]
    return [str(helper)]


def helper_env(channel: str) -> dict[str, str]:
    """Return the environment of the helper: this one plus the endpoint (D-CU-02-5)."""
    return {**os.environ, "BOTHY_COMPUTER_HELPER_CHANNEL": channel}


def run_checks(helper: Path, result: Result, work: Path) -> None:
    """Run the steps of the module docstring; record each check in `result`."""
    fixture = Fixture(work / "events.jsonl")
    endpoint = new_endpoint()
    proc: subprocess.Popen[bytes] | None = None
    try:
        enable_helper()
        proc = subprocess.Popen(
            helper_command(helper, endpoint.name), env=helper_env(endpoint.name)
        )
        endpoint.accept(CONNECT_WAIT)
        result.check("connect", True)
        s = Session(endpoint)
        hello = decode_object(*endpoint.frame())
        result.hello = {k: v for k, v in hello.items() if k != "os_account"}
        result.check("hello", hello.get("op") == "hello", json.dumps(result.hello))
        session_id = str(uuid.uuid4())
        s.send({"op": "show", "session_id": session_id, "agent_name": "Alex"})
        result.check("show", True)
        time.sleep(SETTLE * 2)
        capture_checks(s, fixture, result)
        input_checks(s, fixture, result)
        s.send({"op": "hide", "session_id": session_id})
    except ProbeError as err:
        result.check("error", False, str(err))
    finally:
        if proc is not None:
            proc.terminate()
            with contextlib.suppress(subprocess.TimeoutExpired):
                proc.wait(timeout=EVENT_WAIT)
        if helper.suffix == ".app":
            subprocess.run(["pkill", "-f", str(helper / "Contents" / "MacOS")], check=False)
        endpoint.close()
        fixture.stop()


def capture_checks(s: Session, fixture: Fixture, result: Result) -> None:
    """Capture the screen as PNG and look for the fixture and the indicator."""
    done, data = s.request("capture", max_edge=1280, format="png", quality=100)
    if not result.check("capture", done.get("code") == "ok" and data is not None, done["code"]):
        return
    image = decode_png(data or b"")
    g = fixture.geometry
    cx, cy = to_image(done["screen"], image, g["x"] + g["width"] / 2, g["y"] + g["height"] * 0.75)
    rgb = image.pixel(cx, cy)
    result.check("fixture_in_image", close_to(rgb, FIXTURE_RGB), f"pixel {rgb} at {cx},{cy}")
    found = count_color(image, INDICATOR_RGB)
    result.check("indicator_not_in_image", found == 0, f"{found} pixels of the indicator color")
    result.checks["capture"]["detail"] = f"{image.width}x{image.height}, screen {done['screen']}"


def input_checks(s: Session, fixture: Fixture, result: Result) -> None:
    """Click the entry, type, press Return and an injected Escape; the fixture must see them."""
    ex, ey = fixture.geometry["entry"]["x"], fixture.geometry["entry"]["y"]
    done, _ = s.request("do", action="left_click", x=ex, y=ey)
    click = fixture.seen(lambda e: e.get("kind") == "click")
    result.check("click", done["code"] == "ok" and click is not None, done["code"])
    done, _ = s.request("do", action="type", text=TYPED)
    typed = fixture.seen(lambda e: e.get("kind") == "text" and e.get("value") == TYPED)
    result.check("type", done["code"] == "ok" and typed is not None, done["code"])
    done, _ = s.request("do", action="key", keys="Return", repeat=1)
    key = fixture.seen(lambda e: e.get("kind") == "key" and e.get("keysym") == "Return")
    result.check("key", done["code"] == "ok" and key is not None, done["code"])
    done, _ = s.request("do", action="key", keys="Escape", repeat=1)
    esc = fixture.seen(lambda e: e.get("kind") == "key" and e.get("keysym") == "Escape")
    done2, _ = s.request("position")
    result.check(
        "injected_escape_passes",
        done["code"] == "ok" and esc is not None and not s.stops,
        f"stops {len(s.stops)}",
    )
    result.check("position", done2["code"] == "ok" and "cursor" in done2, done2["code"])


@contextlib.contextmanager
def workdir() -> Iterator[Path]:
    """A temporary folder for the event log."""
    with tempfile.TemporaryDirectory(prefix="computer-probe-") as tmp:
        yield Path(tmp)


def main(argv: list[str] | None = None) -> int:
    """Run the probe or the native test; return the exit code of the module docstring."""
    parser = argparse.ArgumentParser(description="Probe a computer helper on a hosted runner.")
    parser.add_argument("--helper", type=Path, required=True)
    parser.add_argument("--mode", choices=("probe", "native"), required=True)
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--github-output", type=Path)
    args = parser.parse_args(argv)
    result = Result(platform=sys.platform)
    with workdir() as work:
        try:
            run_checks(args.helper, result, work)
        except (OSError, ProbeError) as err:
            result.check("error", False, str(err))
    capture, typed = result.passed(*CAPTURE_CHECKS), result.passed(*INPUT_CHECKS)
    report = {
        "platform": result.platform,
        "capture": capture,
        "input": typed,
        "hello": result.hello,
        "checks": result.checks,
        "lock": "not attempted: a hosted runner cannot lock its session and keep the job alive",
    }
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(report, indent=2))
    if args.github_output:
        with args.github_output.open("a", encoding="utf-8") as fh:
            fh.write(f"capture={str(capture).lower()}\ninput={str(typed).lower()}\n")
    if args.mode == "native" and not all(c["ok"] for c in result.checks.values()):
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
