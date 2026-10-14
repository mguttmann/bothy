# SPDX-License-Identifier: Apache-2.0
"""Tests of tools/ci/apps_platform.py, the platform tests of the apps (brief M4-01, note 6)."""

from __future__ import annotations

import json
import os
import sys
from pathlib import Path

import apps_platform
import pytest

FAKE = """#!{python}
import json, os, sys
with open(os.environ["FAKE_LOG"], "a", encoding="utf-8") as fh:
    fh.write(json.dumps({{"args": sys.argv[1:], "cwd": os.getcwd()}}) + "\\n")
sys.exit(int(os.environ.get("FAKE_EXIT_ON_" + sys.argv[-1].upper(), "0")))
"""


@pytest.fixture
def fake_flutter(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    flutter = bin_dir / "flutter"
    flutter.write_text(FAKE.format(python=sys.executable), encoding="utf-8")
    flutter.chmod(0o755)
    log = tmp_path / "calls.jsonl"
    monkeypatch.setenv("PATH", f"{bin_dir}{os.pathsep}{os.environ['PATH']}")
    monkeypatch.setenv("FAKE_LOG", str(log))
    return log


def _calls(log: Path) -> list[dict[str, object]]:
    if not log.exists():
        return []
    return [json.loads(line) for line in log.read_text(encoding="utf-8").splitlines()]


def _member(root: Path, rel: str, tests: object) -> Path:
    member = root / rel
    member.mkdir(parents=True)
    (member / "pubspec.yaml").write_text("name: x\n", encoding="utf-8")
    (member / "platform-tests.json").write_text(json.dumps(tests), encoding="utf-8")
    return member


def test_runs_commands_for_host_system(
    tmp_path: Path, fake_flutter: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    root = tmp_path / "repo"
    member = _member(
        root,
        "packages/device_key",
        {
            "macos": [
                ["flutter", "test", "integration_test", "-d", "macos"],
                ["flutter", "test", "test", "first"],
            ],
            "windows": [["flutter", "test", "-d", "windows"]],
        },
    )
    assert apps_platform.main(["--root", str(root), "--system", "macos"]) == 0
    assert _calls(fake_flutter) == [
        {"args": ["test", "integration_test", "-d", "macos"], "cwd": str(member.resolve())},
        {"args": ["test", "test", "first"], "cwd": str(member.resolve())},
    ]
    out = capsys.readouterr().out
    assert "apps-platform packages/device_key macos: 2 commands ok" in out
    assert "SKIP apps-platform packages/device_key windows: other host" in out


def test_prints_skip_for_other_system(
    tmp_path: Path, fake_flutter: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    root = tmp_path / "repo"
    _member(root, "apps/client", {"windows": [["flutter", "test", "-d", "windows"]]})
    assert apps_platform.main(["--root", str(root), "--system", "macos"]) == 0
    assert _calls(fake_flutter) == []
    assert capsys.readouterr().out == "SKIP apps-platform apps/client windows: other host\n"


def test_stops_at_first_failure(
    tmp_path: Path,
    fake_flutter: Path,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    root = tmp_path / "repo"
    _member(
        root,
        "apps/client",
        {"windows": [["flutter", "test", "first"], ["flutter", "test", "second"]]},
    )
    _member(root, "packages/device_key", {"windows": [["flutter", "test", "third"]]})
    monkeypatch.setenv("FAKE_EXIT_ON_FIRST", "3")
    assert apps_platform.main(["--root", str(root), "--system", "windows"]) == 1
    assert [c["args"] for c in _calls(fake_flutter)] == [["test", "first"]]
    assert (
        "apps-platform apps/client windows: flutter test first failed with exit code 3"
        in capsys.readouterr().err
    )


def test_no_member_prints_skip(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    (tmp_path / "packages" / "domain").mkdir(parents=True)
    assert apps_platform.main(["--root", str(tmp_path), "--system", "macos"]) == 0
    assert capsys.readouterr().out == "SKIP apps-platform: no member with platform tests\n"


@pytest.mark.parametrize(
    "tests",
    [
        {"macos": ["flutter test -d macos"]},
        {"macos": "flutter test"},
        {"macos": [[]]},
        {"linux": [["flutter", "test"]]},
        ["flutter", "test"],
    ],
)
def test_rejects_string_command(
    tmp_path: Path, fake_flutter: Path, capsys: pytest.CaptureFixture[str], tests: object
) -> None:
    root = tmp_path / "repo"
    _member(root, "apps/client", tests)
    assert apps_platform.main(["--root", str(root), "--system", "macos"]) == 2
    assert _calls(fake_flutter) == []
    assert "apps/client/platform-tests.json" in capsys.readouterr().err


@pytest.mark.parametrize(
    ("platform", "system"), [("darwin", "macos"), ("win32", "windows"), ("linux", None)]
)
def test_host_system(platform: str, system: str | None) -> None:
    assert apps_platform.host_system(platform) == system


def test_unknown_host_runs_nothing(
    tmp_path: Path, fake_flutter: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    root = tmp_path / "repo"
    _member(root, "apps/client", {"macos": [["flutter", "test"]]})
    assert apps_platform.main(["--root", str(root), "--system", "linux"]) == 0
    assert _calls(fake_flutter) == []
    assert capsys.readouterr().out == "SKIP apps-platform apps/client macos: other host\n"
