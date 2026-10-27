# SPDX-License-Identifier: Apache-2.0
"""Tests for the capture of the showcase: the hand-over of demo.sh, filing and preconditions."""

from __future__ import annotations

import base64
import json
import subprocess
from pathlib import Path

import capture
import console_capture
import grafana_capture
import manifest
import media
import pytest
from showcase_fixtures import chunk, mp4, png

REPO = Path(__file__).resolve().parents[3]
COMMIT = "0123456789abcdef0123456789abcdef01234567"


def ready_lines(url: str = "https://localhost:61280", code: str = "en_ABCD-EFGH") -> list[str]:
    return [
        "demo: warm-up and live runs (this takes a few minutes)",
        "demo: the demo is ready:",
        f"demo:   address        {url}",
        "demo:   CA certificate /tmp/bothy-demo.x/ca/ca.pem (trust it in the app or the browser "
        "for this run)",
        f"demo:   sign in as Alex with the enrollment code: {code}",
    ]


def test_parse_ready_reads_the_three_lines() -> None:
    demo = capture.parse_ready(ready_lines(), seed=3)
    assert demo == capture.Demo(
        url="https://localhost:61280",
        ca=Path("/tmp/bothy-demo.x/ca/ca.pem"),
        code="en_ABCD-EFGH",
        seed=3,
    )
    assert demo.port == 61280


@pytest.mark.parametrize("missing", [2, 3, 4])
def test_parse_ready_waits_for_every_line(missing: int) -> None:
    lines = ready_lines()
    del lines[missing]
    assert capture.parse_ready(lines, seed=1) is None


def test_demo_sh_prints_the_lines_the_parser_reads() -> None:
    # The parser reads demo.sh's `say` lines; this holds them equal to the script, so a change
    # of the wording fails here and not in a capture run.
    script = (REPO / capture.DEMO_SH).read_text(encoding="utf-8")
    assert 'say "  address        $public_url"' in script
    assert 'say "  CA certificate $tmp/ca/ca.pem (trust it' in script
    assert 'say "  sign in as Alex with the enrollment code: $code"' in script
    assert "say() { printf 'demo: %s\\n'" in script


def test_android_package_from_the_cache_path() -> None:
    assert capture.android_package("/data/user/0/com.example.chat/code_cache/go") == (
        "com.example.chat"
    )
    with pytest.raises(capture.StepError):
        capture.android_package("/sdcard/go")


def test_split_image_name() -> None:
    assert capture.split_image_name("thread-live-dark") == ("thread-live", "dark")
    with pytest.raises(capture.StepError):
        capture.split_image_name("thread-live")


def test_file_assets_strips_files_and_records_them(tmp_path: Path) -> None:
    old = tmp_path / manifest.ASSET_DIR / "android" / "gone-light.png"
    old.parent.mkdir(parents=True)
    old.write_bytes(png())
    shot = tmp_path / "spaces-light.png"
    shot.write_bytes(png(chunk(b"tEXt", b"Software\0engine")))
    video = tmp_path / "flow.mp4"
    video.write_bytes(mp4(udta=True, created=99))

    filed = capture.file_assets(
        tmp_path,
        "android",
        [(shot, "image", "spaces", "light"), (video, "video", "flow", "dark")],
        COMMIT,
        seed=1,
    )

    assert filed == ["android/spaces-light.png", "android/flow-dark.mp4"]
    assert not old.exists()
    for name in filed:
        assert media.metadata((tmp_path / manifest.ASSET_DIR / name).read_bytes()) == []
    entries = manifest.load(tmp_path / manifest.MANIFEST)
    assert {(a.file, a.kind, a.view, a.theme, a.commit, a.demo_seed) for a in entries} == {
        ("android/spaces-light.png", "image", "spaces", "light", COMMIT, 1),
        ("android/flow-dark.mp4", "video", "flow", "dark", COMMIT, 1),
    }
    assert {a.script for a in entries} == {capture.SCRIPT}


def test_file_assets_refuses_a_file_above_the_limit(tmp_path: Path, monkeypatch) -> None:
    monkeypatch.setattr(capture, "MAX_FILE_BYTES", 10)
    keep = tmp_path / manifest.ASSET_DIR / "android" / "spaces-light.png"
    keep.parent.mkdir(parents=True)
    keep.write_bytes(png())
    shot = tmp_path / "spaces-dark.png"
    shot.write_bytes(png())
    with pytest.raises(capture.StepError, match=r"repo\.max_file_kb"):
        capture.file_assets(tmp_path, "android", [(shot, "image", "spaces", "dark")], COMMIT, 1)
    # Nothing of the platform changed: the old capture stays.
    assert keep.exists()


def test_file_assets_refuses_another_format(tmp_path: Path) -> None:
    shot = tmp_path / "spaces-dark.png"
    shot.write_bytes(b"\xff\xd8\xff\xe0 not a png")
    with pytest.raises(capture.StepError):
        capture.file_assets(tmp_path, "android", [(shot, "image", "spaces", "dark")], COMMIT, 1)


def test_showcase_define_is_the_json_of_the_test(tmp_path: Path) -> None:
    ca = tmp_path / "ca.pem"
    ca.write_text("-----BEGIN CERTIFICATE-----\n", encoding="utf-8")
    demo = capture.Demo("https://localhost:5000", ca, "en_X", 1)
    data = json.loads(
        base64.b64decode(capture.showcase_define(demo, "inst", "en_Y", "android", video=True))
    )
    assert data == {
        "server": "https://localhost:5000",
        "instance_id": "inst",
        "ca_pem": "-----BEGIN CERTIFICATE-----\n",
        "code": "en_Y",
        "platform": "android",
        "upstream_host": capture.EMULATOR_HOST_ALIAS,
        "themes": ["light", "dark"],
        "capture_width": capture.CAPTURE_WIDTH["android"],
        "video": True,
    }
    macos = json.loads(base64.b64decode(capture.showcase_define(demo, "i", "c", "macos", False)))
    assert macos["upstream_host"] is None


def test_codes_hand_out_the_demo_code_once(monkeypatch) -> None:
    demo = capture.Demo("https://localhost:1", Path("ca"), "en_FIRST", 1)
    monkeypatch.setattr(capture, "fresh_code", lambda d: "en_FRESH")
    codes = capture.Codes(demo)
    assert [codes.next(), codes.next(), codes.next()] == ["en_FIRST", "en_FRESH", "en_FRESH"]


def git(repo: Path, *args: str) -> None:
    subprocess.run(["git", "-C", str(repo), *args], check=True, capture_output=True)


def test_preflight_refuses_unknown_platforms_and_a_dirty_tree(tmp_path: Path) -> None:
    git(tmp_path, "init", "-q")
    with pytest.raises(capture.PreconditionError, match="PLATFORM"):
        capture.preflight(tmp_path, ["vax"], allow_dirty=True)
    with pytest.raises(capture.PreconditionError, match="PLATFORM"):
        capture.preflight(tmp_path, [], allow_dirty=True)
    (tmp_path / "x.txt").write_text("x", encoding="utf-8")
    with pytest.raises(capture.PreconditionError, match="commit them first"):
        capture.preflight(tmp_path, ["console"], allow_dirty=False)
    assert capture.dirty_paths(tmp_path) == ["x.txt"]
    shot = tmp_path / manifest.ASSET_DIR / "android" / "a-light.png"
    shot.parent.mkdir(parents=True)
    shot.write_bytes(png())
    # Changes inside the showcase folder are what a capture writes.
    assert capture.dirty_paths(tmp_path) == ["x.txt"]


def test_main_exit_code_2_for_a_precondition(capsys) -> None:
    assert capture.main(["--platform", "vax", "--allow-dirty"]) == 2
    assert "PLATFORM" in capsys.readouterr().err


def test_totp_matches_the_rfc_vector() -> None:
    # RFC 6238 appendix B, SHA-1, T = 59 s: 94287082 (the last six digits).
    assert console_capture.totp(b"12345678901234567890", 1) == "287082"


def test_parse_login() -> None:
    secret = base64.b32encode(b"12345678901234567890").decode().rstrip("=")
    login = console_capture.parse_login(
        json.dumps({"login_name": "alex", "password": "p", "totp_secret": secret, "last_step": 7})
    )
    assert (login.name, login.password, login.secret, login.last_step) == (
        "alex",
        "p",
        b"12345678901234567890",
        7,
    )
    with pytest.raises(capture.StepError):
        console_capture.parse_login("{}")


def test_grafana_images_come_from_compose_yaml() -> None:
    text = (REPO / grafana_capture.COMPOSE).read_text(encoding="utf-8")
    for marker in ("grafana/grafana", "prom/prometheus", "otel/opentelemetry-collector-contrib"):
        image = grafana_capture.compose_image(REPO, marker)
        name, version = image.split(":", 1)
        assert name == marker
        assert f":-{version}}}" in text
    with pytest.raises(capture.StepError):
        grafana_capture.compose_image(REPO, "no/such-image")
