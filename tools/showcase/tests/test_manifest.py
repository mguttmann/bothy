# SPDX-License-Identifier: Apache-2.0
"""Tests for the showcase manifest: schema, record, drop-platform and the check."""

from __future__ import annotations

import json
from dataclasses import asdict
from pathlib import Path

import manifest
import pytest
from showcase_fixtures import chunk, mp4, png

COMMIT = "0123456789abcdef0123456789abcdef01234567"
SCRIPT = "tools/showcase/capture_android.sh"


def asset(file: str, **overrides: object) -> manifest.Asset:
    values: dict[str, object] = {
        "file": file,
        "kind": "video" if file.endswith(".mp4") else "image",
        "platform": file.split("/", maxsplit=1)[0],
        "view": "threads",
        "theme": "light",
        "script": SCRIPT,
        "commit": COMMIT,
        "demo_seed": 7,
    }
    values.update(overrides)
    return manifest.Asset(**values)  # type: ignore[arg-type]


@pytest.fixture
def root(tmp_path: Path) -> Path:
    """A repository root with a capture script and an empty showcase folder."""
    (tmp_path / "tools/showcase").mkdir(parents=True)
    (tmp_path / SCRIPT).write_text("#!/bin/sh\n")
    (tmp_path / manifest.ASSET_DIR).mkdir(parents=True)
    return tmp_path


def put(root: Path, file: str, data: bytes | None = None) -> Path:
    path = root / manifest.ASSET_DIR / file
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(png() if data is None else data)
    return path


def captured(root: Path, file: str, data: bytes | None = None, **overrides: object) -> None:
    put(root, file, data)
    manifest.record(root, asset(file, **overrides))


# --- format --------------------------------------------------------------------


def test_record_writes_sorted_stable_json_with_trailing_newline(root: Path) -> None:
    captured(root, "android/threads-light.png")
    captured(root, "android/approval-dark.png", theme="dark", view="approval")
    text = (root / manifest.MANIFEST).read_text()
    assert text.endswith("}\n")
    data = json.loads(text)
    assert data["version"] == 1
    assert [a["file"] for a in data["assets"]] == [
        "android/approval-dark.png",
        "android/threads-light.png",
    ]
    assert list(data["assets"][0]) == sorted(data["assets"][0])
    assert manifest.dump(manifest.load(root / manifest.MANIFEST)) == text


def test_record_replaces_the_entry_of_the_same_file(root: Path) -> None:
    captured(root, "android/threads-light.png")
    manifest.record(root, asset("android/threads-light.png", demo_seed=8))
    assets = manifest.load(root / manifest.MANIFEST)
    assert [(a.file, a.demo_seed) for a in assets] == [("android/threads-light.png", 8)]


def test_record_refuses_an_invalid_entry(root: Path) -> None:
    with pytest.raises(manifest.ManifestError, match=r"commit: expected 40 lower-case hex"):
        manifest.record(root, asset("android/threads-light.png", commit="abc"))
    assert not (root / manifest.MANIFEST).exists()


def test_missing_manifest_loads_as_empty(root: Path) -> None:
    assert manifest.load(root / manifest.MANIFEST) == []


@pytest.mark.parametrize(
    ("data", "message"),
    [
        ([], "manifest: expected an object"),
        ({"version": 2, "assets": []}, "version: expected 1"),
        ({"version": 1}, "missing key 'assets'"),
        ({"version": 1, "assets": [], "x": 1}, "unknown key 'x'"),
        ({"version": 1, "assets": {}}, "assets: expected a list"),
        ({"version": 1, "assets": [1]}, r"assets\[0\]: expected an object"),
    ],
)
def test_load_names_top_level_schema_errors(root: Path, data: object, message: str) -> None:
    (root / manifest.MANIFEST).write_text(json.dumps(data))
    with pytest.raises(manifest.ManifestError, match=message):
        manifest.load(root / manifest.MANIFEST)


@pytest.mark.parametrize(
    ("override", "message"),
    [
        ({"kind": "gif"}, r"assets\[0\]\.kind: expected 'image' or 'video'"),
        ({"theme": "blue"}, r"assets\[0\]\.theme: expected 'light' or 'dark'"),
        ({"platform": "Android"}, r"assets\[0\]\.platform: expected \[a-z0-9-\]"),
        ({"view": ""}, r"assets\[0\]\.view: expected a non-empty string"),
        ({"view": 3}, r"assets\[0\]\.view: expected a non-empty string"),
        ({"script": "/abs/run.sh"}, r"assets\[0\]\.script: expected a relative repository path"),
        ({"script": "../run.sh"}, r"assets\[0\]\.script: expected a relative repository path"),
        ({"file": "a\\b.png"}, r"assets\[0\]\.file: expected a relative path"),
        ({"commit": "A" * 40}, r"assets\[0\]\.commit: expected 40 lower-case hex"),
        ({"demo_seed": "7"}, r"assets\[0\]\.demo_seed: expected an integer"),
        ({"demo_seed": True}, r"assets\[0\]\.demo_seed: expected an integer"),
    ],
)
def test_load_names_entry_schema_errors(
    root: Path, override: dict[str, object], message: str
) -> None:
    entry = {**asdict(asset("android/threads-light.png")), **override}
    (root / manifest.MANIFEST).write_text(json.dumps({"version": 1, "assets": [entry]}))
    with pytest.raises(manifest.ManifestError, match=message):
        manifest.load(root / manifest.MANIFEST)


def test_load_names_missing_unknown_and_duplicate_entry_keys(root: Path) -> None:
    entry = asdict(asset("android/threads-light.png"))
    broken = {k: v for k, v in entry.items() if k != "view"} | {"title": "x", "file": "ios/a.png"}
    data = {"version": 1, "assets": [broken, entry, entry]}
    (root / manifest.MANIFEST).write_text(json.dumps(data))
    with pytest.raises(manifest.ManifestError) as caught:
        manifest.load(root / manifest.MANIFEST)
    assert caught.value.problems == [
        "assets[0]: missing key 'view'",
        "assets[0]: unknown key 'title'",
        "assets[2].file: 'android/threads-light.png' is listed twice",
    ]


def test_load_refuses_invalid_json(root: Path) -> None:
    (root / manifest.MANIFEST).write_text("{")
    with pytest.raises(manifest.ManifestError, match="not valid JSON"):
        manifest.load(root / manifest.MANIFEST)


# --- drop-platform ---------------------------------------------------------------


def test_drop_platform_removes_its_entries_and_files(root: Path) -> None:
    captured(root, "android/threads-light.png")
    captured(root, "android/flow.mp4", mp4())
    captured(root, "ios/threads-light.png")
    put(root, "android/stale.png")
    put(root, "android/old/nested.png")
    removed = manifest.drop_platform(root, "android")
    assert removed == [
        "android/flow.mp4",
        "android/old/nested.png",
        "android/stale.png",
        "android/threads-light.png",
    ]
    assert not (root / manifest.ASSET_DIR / "android").exists()
    assert [a.file for a in manifest.load(root / manifest.MANIFEST)] == ["ios/threads-light.png"]
    assert manifest.check(root) == []


def test_drop_platform_without_entries_or_files_changes_nothing(root: Path) -> None:
    captured(root, "ios/threads-light.png")
    before = (root / manifest.MANIFEST).read_text()
    assert manifest.drop_platform(root, "android") == []
    assert (root / manifest.MANIFEST).read_text() == before


def test_drop_platform_refuses_a_bad_platform_name(root: Path) -> None:
    with pytest.raises(manifest.ManifestError, match="platform"):
        manifest.drop_platform(root, "../docs")


# --- check ---------------------------------------------------------------------


def test_check_passes_a_clean_showcase(root: Path) -> None:
    captured(root, "android/threads-light.png")
    captured(root, "android/threads-dark.png", theme="dark")
    captured(root, "console/flow.mp4", mp4(), view="flow")
    (root / manifest.ASSET_DIR / "README.md").write_text("# Showcase\n")
    assert manifest.check(root) == []


def test_check_passes_an_empty_showcase_without_manifest(root: Path) -> None:
    assert manifest.check(root) == []


def test_check_finds_a_missing_image(root: Path) -> None:
    captured(root, "android/threads-light.png")
    (root / manifest.ASSET_DIR / "android/threads-light.png").unlink()
    assert manifest.check(root) == [
        "showcase: android/threads-light.png: named in the manifest but missing"
    ]


def test_check_finds_an_extra_image(root: Path) -> None:
    captured(root, "android/threads-light.png")
    put(root, "android/extra.png")
    assert manifest.check(root) == ["showcase: android/extra.png: not in the manifest"]


@pytest.mark.parametrize(
    "file",
    ["android/Threads.png", "android/threads_light.png", "android/threads.PNG", "threads.png"],
)
def test_check_finds_a_bad_name(root: Path, file: str) -> None:
    put(root, file)
    manifest.record(root, asset(file, platform="android"))
    findings = manifest.check(root)
    assert any("file name" in f for f in findings), findings


def test_check_finds_a_wrong_folder(root: Path) -> None:
    captured(root, "ios/threads-light.png", platform="android")
    assert manifest.check(root) == [
        "showcase: ios/threads-light.png: folder 'ios' is not the platform 'android'"
    ]


def test_check_finds_a_nested_folder(root: Path) -> None:
    put(root, "android/tablet/threads-light.png")
    manifest.record(root, asset("android/tablet/threads-light.png"))
    assert manifest.check(root) == [
        "showcase: android/tablet/threads-light.png: file name must be "
        "<platform>/<[a-z0-9-]+>.png or .mp4"
    ]


def test_check_finds_metadata(root: Path) -> None:
    captured(root, "android/text.png", png(chunk(b"tEXt", b"Author\x00Alex")))
    captured(root, "android/exif.png", png(chunk(b"eXIf", b"MM\x00*")))
    captured(root, "android/flow.mp4", mp4(udta=True))
    assert manifest.check(root) == [
        "showcase: android/exif.png: metadata: eXIf chunk",
        "showcase: android/flow.mp4: metadata: moov/udta box",
        "showcase: android/text.png: metadata: tEXt chunk",
    ]


def test_check_finds_a_wrong_format(root: Path) -> None:
    captured(root, "android/photo.jpg", b"\xff\xd8\xff\xe0JFIF")
    captured(root, "android/fake.png", mp4(), kind="image")
    captured(root, "android/clip.mp4", png(), kind="video")
    captured(root, "android/still.png", png(), kind="video")
    assert manifest.check(root) == [
        "showcase: android/clip.mp4: content is PNG, extension is .mp4",
        "showcase: android/fake.png: content is MP4, extension is .png",
        "showcase: android/photo.jpg: file name must be <platform>/<[a-z0-9-]+>.png or .mp4",
        "showcase: android/photo.jpg: neither PNG nor MP4",
        "showcase: android/still.png: kind 'video' does not match .png",
    ]


def test_check_finds_a_damaged_file(root: Path) -> None:
    captured(root, "android/threads-light.png", png()[:-2])
    iend = len(png()) - 12
    assert manifest.check(root) == [
        f"showcase: android/threads-light.png: PNG truncated: chunk at byte {iend} runs past "
        "the end"
    ]


def test_check_finds_a_missing_capture_script(root: Path) -> None:
    captured(root, "android/threads-light.png", script="tools/showcase/gone.sh")
    assert manifest.check(root) == [
        "showcase: android/threads-light.png: capture script tools/showcase/gone.sh is missing"
    ]


def test_check_reports_schema_errors_as_findings(root: Path) -> None:
    put(root, "android/threads-light.png")
    (root / manifest.MANIFEST).write_text(json.dumps({"version": 3, "assets": []}))
    assert manifest.check(root) == ["showcase: manifest.json: version: expected 1, got 3"]
