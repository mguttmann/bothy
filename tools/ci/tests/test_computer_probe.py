# SPDX-License-Identifier: Apache-2.0
"""Tests of tools/ci/computer_probe.py without a helper, a window or a screen (brief CU-02).

The probe itself runs only in the jobs of host-platform.yml; these tests hold its channel codec to
the vectors of docs/spec/27-computer-use.md 9.4, its PNG reader, the coordinate mapping and the
result it reports in both modes.
"""

from __future__ import annotations

import json
import struct
import zlib
from pathlib import Path
from typing import Any

import computer_probe as cp
import pytest

VECTORS = cp.REPO / "services" / "host" / "testdata" / "computer" / "channel-vectors.json"


def _vectors() -> list[dict[str, Any]]:
    found = json.loads(VECTORS.read_text(encoding="utf-8"))["vectors"]
    assert isinstance(found, list)
    return found


@pytest.mark.parametrize("vector", _vectors(), ids=lambda v: v["name"])
def test_the_codec_reads_every_vector(vector: dict[str, Any]) -> None:
    kind, content, rest = cp.decode(bytes.fromhex(vector["frame_hex"]))
    assert rest == b""
    assert kind == vector["type"]
    if kind == cp.JSON_FRAME:
        assert cp.decode_object(kind, content) == vector["object"]
        again = cp.decode(cp.encode_json(vector["object"]))
        assert cp.decode_object(again[0], again[1]) == vector["object"]
    else:
        assert content == bytes.fromhex(vector["bytes_hex"])


def test_frame_limits() -> None:
    with pytest.raises(cp.ProbeError, match="exceeds"):
        cp.encode(cp.IMAGE_FRAME, b"x" * 10, limit=10)
    assert cp.decode(cp.encode(cp.IMAGE_FRAME, b"x" * 9, limit=10), limit=10)[1] == b"x" * 9
    with pytest.raises(cp.ProbeError, match="outside"):
        cp.decode(struct.pack(">I", 11) + b"\x02" + b"x" * 10, limit=10)
    with pytest.raises(cp.ProbeError, match="outside"):
        cp.decode(struct.pack(">I", 0))
    with pytest.raises(cp.ProbeError, match="incomplete"):
        cp.decode(struct.pack(">I", 5) + b"\x01{}")
    with pytest.raises(cp.ProbeError, match="object"):
        cp.decode_object(cp.JSON_FRAME, b"[1]")


def _png(width: int, height: int, rgb: list[tuple[int, int, int]], filters: list[int]) -> bytes:
    """A PNG whose rows use the given filter types (raw rows, filtered here)."""
    raw = b""
    prev = bytes(width * 3)
    for y in range(height):
        line = bytes(c for x in range(width) for c in rgb[y * width + x])
        kind = filters[y]
        out = bytearray()
        for i, v in enumerate(line):
            left = line[i - 3] if i >= 3 else 0
            up = prev[i]
            corner = prev[i - 3] if i >= 3 else 0
            pred = [0, left, up, (left + up) // 2, cp._paeth(left, up, corner)][kind]
            out.append((v - pred) & 0xFF)
        raw += bytes([kind]) + bytes(out)
        prev = line

    def chunk(name: bytes, body: bytes) -> bytes:
        return (
            struct.pack(">I", len(body)) + name + body + struct.pack(">I", zlib.crc32(name + body))
        )

    header = struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0)
    return (
        b"\x89PNG\r\n\x1a\n"
        + chunk(b"IHDR", header)
        + chunk(b"IDAT", zlib.compress(raw))
        + chunk(b"IEND", b"")
    )


def test_decode_png_with_every_filter() -> None:
    pixels = [
        ((x * 40) % 256, (y * 70) % 256, (x * y * 13) % 256) for y in range(5) for x in range(4)
    ]
    image = cp.decode_png(_png(4, 5, pixels, [0, 1, 2, 3, 4]))
    assert (image.width, image.height) == (4, 5)
    assert [image.pixel(x, y) for y in range(5) for x in range(4)] == pixels
    with pytest.raises(cp.ProbeError, match="not a PNG"):
        cp.decode_png(b"\xff\xd8\xff\xd9")


def test_colors_and_the_indicator_count() -> None:
    pixels = [cp.FIXTURE_RGB] * 5 + [cp.INDICATOR_RGB]
    image = cp.decode_png(_png(3, 2, pixels, [0, 0]))
    assert cp.close_to(image.pixel(0, 0), cp.FIXTURE_RGB)
    assert not cp.close_to(image.pixel(2, 1), cp.FIXTURE_RGB)
    assert cp.count_color(image, cp.INDICATOR_RGB) == 1


def test_screen_points_map_to_image_pixels() -> None:
    image = cp.Image(width=1280, height=831, channels=3, rows=[])
    screen = {"x": -1512, "y": 0, "width": 1512, "height": 982, "scale": 2}
    assert cp.to_image(screen, image, -1512, 0) == (0, 0)
    assert cp.to_image(screen, image, -756, 491) == (640, 415)
    assert cp.to_image(screen, image, 10_000, 10_000) == (1279, 830)


class _Pipe(cp.Endpoint):
    """An endpoint that reads prepared frames and records what the probe writes."""

    def __init__(self, frames: list[bytes]) -> None:
        super().__init__()
        self._data = b"".join(frames)
        self.written: list[dict[str, Any]] = []

    def read(self, n: int) -> bytes:
        chunk, self._data = self._data[:n], self._data[n:]
        return chunk

    def write(self, data: bytes) -> None:
        kind, content, _ = cp.decode(data)
        self.written.append(cp.decode_object(kind, content))


def test_a_session_collects_stops_and_reads_the_image_after_its_done() -> None:
    done = {"op": "done", "seq": 1, "code": "ok", "image": {"format": "png", "bytes": 3}}
    pipe = _Pipe(
        [
            cp.encode_json({"op": "state", "locked": False}),
            cp.encode_json({"op": "stop", "session_id": "s", "detail": "local_escape"}),
            cp.encode_json(done),
            cp.encode(cp.IMAGE_FRAME, b"abc"),
        ]
    )
    s = cp.Session(pipe)
    obj, data = s.request("capture", max_edge=8, format="png", quality=100)
    assert pipe.written == [
        {"op": "capture", "seq": 1, "max_edge": 8, "format": "png", "quality": 100}
    ]
    assert obj == done
    assert data == b"abc"
    assert s.stops == [{"op": "stop", "session_id": "s", "detail": "local_escape"}]


def test_a_session_refuses_a_missing_image_and_a_closed_channel() -> None:
    done = {"op": "done", "seq": 1, "code": "ok", "image": {"format": "png", "bytes": 4}}
    s = cp.Session(_Pipe([cp.encode_json(done), cp.encode(cp.IMAGE_FRAME, b"abc")]))
    with pytest.raises(cp.ProbeError, match="image frame"):
        s.request("capture")
    with pytest.raises(cp.ProbeError, match="closed"):
        cp.Session(_Pipe([])).request("position")


@pytest.mark.parametrize(("mode", "code"), [("probe", 0), ("native", 1)])
def test_main_records_a_failed_run(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, mode: str, code: int
) -> None:
    def fail(helper: Path, result: cp.Result, work: Path) -> None:
        result.check("connect", True)
        result.check("capture", False, "os_refused")

    monkeypatch.setattr(cp, "run_checks", fail)
    out, gh = tmp_path / "r.json", tmp_path / "gh"
    argv = ["--helper", "h", "--mode", mode, "--out", str(out), "--github-output", str(gh)]
    assert cp.main(argv) == code
    report = json.loads(out.read_text(encoding="utf-8"))
    assert report["capture"] is False
    assert report["input"] is False
    assert report["checks"]["capture"] == {"ok": False, "detail": "os_refused"}
    assert gh.read_text(encoding="utf-8") == "capture=false\ninput=false\n"


def test_main_reports_a_passing_run(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    def passing(helper: Path, result: cp.Result, work: Path) -> None:
        for name in (*cp.CAPTURE_CHECKS, *cp.INPUT_CHECKS):
            result.check(name, True)

    monkeypatch.setattr(cp, "run_checks", passing)
    out, gh = tmp_path / "r.json", tmp_path / "gh"
    argv = ["--helper", "h", "--mode", "native", "--out", str(out), "--github-output", str(gh)]
    assert cp.main(argv) == 0
    assert gh.read_text(encoding="utf-8") == "capture=true\ninput=true\n"


def test_the_state_file_is_the_one_of_the_helper(monkeypatch: pytest.MonkeyPatch) -> None:
    path = cp.state_file()
    assert path.name == "state.json"
    assert path.parent.name == "Bothy Computer Helper"


def test_the_helper_starts_with_the_endpoint() -> None:
    app = Path("/x/Bothy Computer Helper.app")
    assert cp.helper_command(app, "/tmp/c.sock") == [
        "open",
        "-n",
        "-W",
        "--env",
        "BOTHY_COMPUTER_HELPER_CHANNEL=/tmp/c.sock",
        str(app),
    ]
    assert cp.helper_command(Path("h.exe"), r"\\.\pipe\x") == ["h.exe"]
    assert cp.helper_env("e")["BOTHY_COMPUTER_HELPER_CHANNEL"] == "e"
