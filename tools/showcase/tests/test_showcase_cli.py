# SPDX-License-Identifier: Apache-2.0
"""Tests for the showcase command line: strip, record, drop-platform and check."""

from __future__ import annotations

import json
from pathlib import Path

import manifest
import pytest
import showcase_cli
from showcase_fixtures import chunk, mp4, png

COMMIT = "0123456789abcdef0123456789abcdef01234567"
SCRIPT = "tools/showcase/capture_console.sh"


@pytest.fixture
def root(tmp_path: Path) -> Path:
    (tmp_path / "tools/showcase").mkdir(parents=True)
    (tmp_path / SCRIPT).write_text("#!/bin/sh\n")
    (tmp_path / manifest.ASSET_DIR / "console").mkdir(parents=True)
    return tmp_path


def record_args(root: Path, file: str, *extra: str) -> list[str]:
    return [
        "--root",
        str(root),
        "record",
        "--file",
        file,
        "--kind",
        "image",
        "--platform",
        "console",
        "--view",
        "threads",
        "--theme",
        "dark",
        "--script",
        SCRIPT,
        "--commit",
        COMMIT,
        "--seed",
        "42",
        *extra,
    ]


def test_strip_rewrites_files_in_place_by_content(tmp_path: Path) -> None:
    image = tmp_path / "a.png"
    image.write_bytes(png(chunk(b"tEXt", b"Author\x00Alex")))
    video = tmp_path / "b.mp4"
    video.write_bytes(mp4(udta=True, created=9))
    assert showcase_cli.main(["strip", str(image), str(video)]) == 0
    assert image.read_bytes() == png()
    assert showcase_cli.media.metadata(video.read_bytes()) == []


def test_strip_fails_on_an_unsupported_file(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    photo = tmp_path / "a.jpg"
    photo.write_bytes(b"\xff\xd8\xff\xe0JFIF")
    assert showcase_cli.main(["strip", str(photo)]) == 1
    assert f"showcase: {photo}: neither PNG nor MP4" in capsys.readouterr().err


def test_strip_fails_on_a_missing_file(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    assert showcase_cli.main(["strip", str(tmp_path / "gone.png")]) == 1
    assert "gone.png" in capsys.readouterr().err


def test_record_then_check(root: Path, capsys: pytest.CaptureFixture[str]) -> None:
    (root / manifest.ASSET_DIR / "console/threads-dark.png").write_bytes(png())
    assert showcase_cli.main(record_args(root, "console/threads-dark.png")) == 0
    entry = json.loads((root / manifest.MANIFEST).read_text())["assets"][0]
    assert entry == {
        "commit": COMMIT,
        "demo_seed": 42,
        "file": "console/threads-dark.png",
        "kind": "image",
        "platform": "console",
        "script": SCRIPT,
        "theme": "dark",
        "view": "threads",
    }
    assert showcase_cli.main(["--root", str(root), "check"]) == 0
    assert "showcase: clean (1 file)" in capsys.readouterr().out


def test_record_refuses_invalid_values_as_usage_error(
    root: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    args = record_args(root, "console/threads-dark.png")
    args[args.index("--commit") + 1] = "HEAD"
    assert showcase_cli.main(args) == 2
    assert "commit: expected 40 lower-case hex" in capsys.readouterr().err


def test_check_prints_one_line_per_finding(root: Path, capsys: pytest.CaptureFixture[str]) -> None:
    (root / manifest.ASSET_DIR / "console/extra.png").write_bytes(png(chunk(b"tIME", bytes(7))))
    assert showcase_cli.main(["--root", str(root), "check"]) == 1
    out = capsys.readouterr().out.splitlines()
    assert out == [
        "showcase: console/extra.png: metadata: tIME chunk",
        "showcase: console/extra.png: not in the manifest",
        "showcase: 2 finding(s); run `showcase_cli.py strip` and `showcase_cli.py record` "
        "(tools/showcase/README.md)",
    ]


def test_drop_platform(root: Path, capsys: pytest.CaptureFixture[str]) -> None:
    (root / manifest.ASSET_DIR / "console/threads-dark.png").write_bytes(png())
    assert showcase_cli.main(record_args(root, "console/threads-dark.png")) == 0
    assert showcase_cli.main(["--root", str(root), "drop-platform", "console"]) == 0
    assert "removed console/threads-dark.png" in capsys.readouterr().out
    assert manifest.load(root / manifest.MANIFEST) == []
    assert showcase_cli.main(["--root", str(root), "drop-platform", "../x"]) == 2


def test_usage_errors_exit_2(capsys: pytest.CaptureFixture[str]) -> None:
    with pytest.raises(SystemExit) as caught:
        showcase_cli.main([])
    assert caught.value.code == 2
    with pytest.raises(SystemExit) as caught:
        showcase_cli.main(["record", "--file", "x.png"])
    assert caught.value.code == 2
    capsys.readouterr()


def test_root_defaults_to_the_repository() -> None:
    assert (showcase_cli.REPO_ROOT / "tools/showcase/showcase_cli.py").is_file()
