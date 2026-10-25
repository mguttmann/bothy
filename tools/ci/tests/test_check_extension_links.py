# SPDX-License-Identifier: Apache-2.0
"""Tests for tools/ci/check_extension_links.py (ADR 0056 "Confirmation", brief M5-04 note 7)."""

from __future__ import annotations

import plistlib
from pathlib import Path

import check_extension_links as cel

CLEAN = """NotificationService:
\t/System/Library/Frameworks/UserNotifications.framework/UserNotifications (compatibility 1.0.0)
\t/System/Library/Frameworks/Foundation.framework/Foundation (compatibility version 300.0.0)
\t/usr/lib/libSystem.B.dylib (compatibility version 1.0.0, current version 1351.0.0)
\t/usr/lib/swift/libswiftCore.dylib (compatibility version 1.0.0, current version 6.0.0)
"""

WITH_FLUTTER = CLEAN + "\t@rpath/Flutter.framework/Flutter (compatibility version 1.0.0)\n"
WITH_MACOS = (
    CLEAN
    + "\t@rpath/FlutterMacOS.framework/Versions/A/FlutterMacOS (compatibility version 1.0.0)\n"
)
WITH_APP = CLEAN + "\t@rpath/App.framework/App (compatibility version 0.0.0)\n"
SIMILAR = CLEAN + "\t@rpath/MyApp.framework/MyApp (compatibility version 0.0.0)\n"


def _app(
    root: Path, name: str, *, macos: bool, extension: str | None = "NotificationService"
) -> Path:
    app = root / f"{name}.app"
    plugins = app / "Contents" / "PlugIns" if macos else app / "PlugIns"
    plugins.mkdir(parents=True)
    if extension:
        appex = plugins / f"{extension}.appex"
        info_dir = appex / "Contents" if macos else appex
        info_dir.mkdir(parents=True)
        with (info_dir / "Info.plist").open("wb") as handle:
            plistlib.dump({"CFBundleExecutable": extension}, handle)
    return app


def test_forbidden_links_finds_each_flutter_framework() -> None:
    assert cel.forbidden_links(CLEAN) == []
    assert cel.forbidden_links(SIMILAR) == []
    assert len(cel.forbidden_links(WITH_FLUTTER)) == 1
    assert len(cel.forbidden_links(WITH_MACOS)) == 1
    assert len(cel.forbidden_links(WITH_APP)) == 1


def test_check_passes_clean_extensions_of_both_layouts(tmp_path: Path) -> None:
    ios = _app(tmp_path, "Runner", macos=False)
    mac = _app(tmp_path, "ExampleChat", macos=True)
    seen: list[Path] = []

    def otool(binary: Path) -> str:
        seen.append(binary)
        return CLEAN

    assert cel.check([ios, mac], ["NotificationService"], otool) == []
    assert seen == [
        ios / "PlugIns/NotificationService.appex/NotificationService",
        mac / "Contents/PlugIns/NotificationService.appex/Contents/MacOS/NotificationService",
    ]


def test_check_fails_on_flutter_link_and_missing_extension(tmp_path: Path) -> None:
    ios = _app(tmp_path, "Runner", macos=False)
    problems = cel.check([ios], ["NotificationService"], lambda _: WITH_FLUTTER)
    assert len(problems) == 1
    assert "Flutter.framework" in problems[0]
    empty = _app(tmp_path / "e", "Runner", macos=False, extension=None)
    assert cel.check([empty], ["NotificationService"], lambda _: CLEAN) == [
        "Runner.app: extension NotificationService.appex is missing"
    ]
    assert cel.check([], ["NotificationService"], lambda _: CLEAN) == ["no app bundle found"]


def test_main_reports_unmatched_glob(tmp_path: Path, monkeypatch) -> None:
    monkeypatch.setattr(cel.shutil, "which", lambda _: "/usr/bin/otool")
    monkeypatch.setattr(cel, "xcode_available", lambda: True)
    assert cel.main(["--app", str(tmp_path / "missing/*.app")]) == 1


def test_main_skips_without_otool(monkeypatch, capsys) -> None:
    monkeypatch.setattr(cel.shutil, "which", lambda _: None)
    assert cel.main(["--app", "x.app"]) == 0
    assert "SKIP" in capsys.readouterr().out


def test_main_skips_without_xcode(monkeypatch, capsys) -> None:
    monkeypatch.setattr(cel.shutil, "which", lambda _: "/usr/bin/otool")
    monkeypatch.setattr(cel, "xcode_available", lambda: False)
    assert cel.main(["--app", "x.app"]) == 0
    assert "SKIP" in capsys.readouterr().out
