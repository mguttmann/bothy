# SPDX-License-Identifier: Apache-2.0
"""Tests for info_plist_strings: the iOS permission texts from the ARB files (brief M5-07)."""

from __future__ import annotations

import json
from pathlib import Path

import info_plist_strings as ips
import pytest

PLIST = """<plist version="1.0"><dict>
<key>NSCameraUsageDescription</key>
<string>{camera}</string>
<key>NSMicrophoneUsageDescription</key>
<string>{mic}</string>
<key>NSPhotoLibraryUsageDescription</key>
<string>{photos}</string>
</dict></plist>
"""


def _repo(tmp_path: Path, en: dict[str, str], de: dict[str, str], plist: str) -> Path:
    arb = tmp_path / ips.ARB_DIR
    arb.mkdir(parents=True)
    (arb / "app_en.arb").write_text(json.dumps(en), encoding="utf-8")
    (arb / "app_de.arb").write_text(json.dumps(de), encoding="utf-8")
    runner = tmp_path / ips.RUNNER
    runner.mkdir(parents=True)
    (runner / "Info.plist").write_text(plist, encoding="utf-8")
    return tmp_path


EN = {
    "permissionCameraUsage": 'The camera takes the "photos".',
    "permissionMicrophoneUsage": "The microphone records.",
    "permissionPhotoLibraryUsage": "Photos you choose.",
}
DE = {
    "permissionCameraUsage": "Die Kamera.",
    "permissionMicrophoneUsage": "Das Mikrofon.",
    "permissionPhotoLibraryUsage": "Die Fotos.",
}


def test_write_then_check_passes_and_escapes_quotes(tmp_path: Path) -> None:
    root = _repo(
        tmp_path,
        EN,
        DE,
        PLIST.format(
            camera=EN["permissionCameraUsage"],
            mic=EN["permissionMicrophoneUsage"],
            photos=EN["permissionPhotoLibraryUsage"],
        ),
    )
    assert ips.main(["--root", str(root)]) == 0
    assert ips.main(["--root", str(root), "--check"]) == 0
    en = (root / ips.RUNNER / "en.lproj" / "InfoPlist.strings").read_text(encoding="utf-8")
    assert '"NSCameraUsageDescription" = "The camera takes the \\"photos\\".";' in en
    de = (root / ips.RUNNER / "de.lproj" / "InfoPlist.strings").read_text(encoding="utf-8")
    assert '"NSMicrophoneUsageDescription" = "Das Mikrofon.";' in de


def test_a_changed_message_makes_the_files_stale(tmp_path: Path) -> None:
    root = _repo(
        tmp_path,
        EN,
        DE,
        PLIST.format(
            camera=EN["permissionCameraUsage"],
            mic=EN["permissionMicrophoneUsage"],
            photos=EN["permissionPhotoLibraryUsage"],
        ),
    )
    assert ips.main(["--root", str(root)]) == 0
    changed = dict(DE, permissionCameraUsage="Die Kamera nimmt auf.")
    (root / ips.ARB_DIR / "app_de.arb").write_text(json.dumps(changed), encoding="utf-8")
    assert ips.main(["--root", str(root), "--check"]) == 1


def test_info_plist_must_hold_the_english_text(tmp_path: Path) -> None:
    root = _repo(
        tmp_path,
        EN,
        DE,
        PLIST.format(
            camera="Old text",
            mic=EN["permissionMicrophoneUsage"],
            photos=EN["permissionPhotoLibraryUsage"],
        ),
    )
    ips.write(root)
    problems = ips.check(root)
    assert len(problems) == 1
    assert "NSCameraUsageDescription" in problems[0]


@pytest.mark.parametrize("text", ["", "Uses {thing}"])
def test_missing_or_placeholder_messages_fail(tmp_path: Path, text: str) -> None:
    en = dict(EN, permissionMicrophoneUsage=text)
    root = _repo(
        tmp_path,
        en,
        DE,
        PLIST.format(
            camera=EN["permissionCameraUsage"], mic=text, photos=EN["permissionPhotoLibraryUsage"]
        ),
    )
    assert ips.main(["--root", str(root), "--check"]) == 1
