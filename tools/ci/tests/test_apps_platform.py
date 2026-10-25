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
log = os.environ["FAKE_LOG"]
with open(log, "a", encoding="utf-8") as fh:
    fh.write(json.dumps({{"args": sys.argv[1:], "cwd": os.getcwd()}}) + "\\n")
with open(log, encoding="utf-8") as fh:
    calls = len(fh.read().splitlines())
if calls <= int(os.environ.get("FAKE_NATIVE_FAILURES", "0")):
    print("Target build_hooks failed : error : Building native assets failed. See the logs.")
    sys.exit(1)
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


def test_finds_a_nested_harness_member(
    tmp_path: Path, fake_flutter: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    # The harness of bothy_viz is a workspace member one level below its package
    # (docs/DECISIONS.md D-M6-11-6); its performance test runs on macOS (brief M6-12).
    root = tmp_path / "repo"
    member = _member(root, "packages/bothy_viz/harness", {"macos": [["flutter", "drive", "perf"]]})
    assert apps_platform.main(["--root", str(root), "--system", "macos"]) == 0
    assert _calls(fake_flutter) == [{"args": ["drive", "perf"], "cwd": str(member.resolve())}]
    assert "apps-platform packages/bothy_viz/harness macos: 1 commands ok" in (
        capsys.readouterr().out
    )


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


def test_program_is_resolved_on_path(
    tmp_path: Path, fake_flutter: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """The program comes from PATH through shutil.which, which also finds `flutter.bat` on
    Windows, where a bare name without a shell would not resolve."""
    root = tmp_path / "repo"
    _member(root, "apps/client", {"macos": [["flutter", "test"]]})
    seen: list[str] = []
    real_which = apps_platform.shutil.which

    def which(name: str) -> str | None:
        seen.append(name)
        return real_which(name)

    monkeypatch.setattr(apps_platform.shutil, "which", which)
    assert apps_platform.main(["--root", str(root), "--system", "macos"]) == 0
    assert seen == ["flutter"]
    assert len(_calls(fake_flutter)) == 1


def test_missing_program_fails(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    root = tmp_path / "repo"
    _member(root, "apps/client", {"macos": [["no-such-program-m401", "test"]]})
    assert apps_platform.main(["--root", str(root), "--system", "macos"]) == 1
    assert (
        "apps-platform apps/client macos: no-such-program-m401 is not on PATH"
        in capsys.readouterr().err
    )


ADB = """#!{python}
import os, sys
if sys.argv[1:] != ["devices"]:
    sys.exit(9)
sys.stdout.write("List of devices attached\\n")
for serial in filter(None, os.environ.get("FAKE_ADB_DEVICES", "").split(",")):
    name, _, state = serial.partition(":")
    sys.stdout.write(f"{{name}}\\t{{state or 'device'}}\\n")
sys.stdout.write("\\n")
"""


@pytest.fixture
def fake_adb(fake_flutter: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    """An `adb` next to the fake flutter; FAKE_ADB_DEVICES lists `serial[:state]` entries."""
    bin_dir = Path(os.environ["PATH"].split(os.pathsep)[0])
    adb = bin_dir / "adb"
    adb.write_text(ADB.format(python=sys.executable), encoding="utf-8")
    adb.chmod(0o755)
    monkeypatch.setenv("FAKE_ADB_DEVICES", "")
    return fake_flutter


def _android_member(root: Path) -> Path:
    return _member(
        root,
        "packages/device_key",
        {
            "macos": [["flutter", "test", "mac"]],
            "android": [["flutter", "test", "android"]],
        },
    )


def test_android_runs_with_one_device(
    tmp_path: Path,
    fake_adb: Path,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    root = tmp_path / "repo"
    member = _android_member(root)
    # A device in another state (offline, unauthorized) is not a target.
    monkeypatch.setenv("FAKE_ADB_DEVICES", "emulator-5554,R58M1234:offline")
    assert apps_platform.main(["--root", str(root), "--system", "macos"]) == 0
    assert _calls(fake_adb) == [
        {"args": ["test", "mac"], "cwd": str(member.resolve())},
        {"args": ["test", "android"], "cwd": str(member.resolve())},
    ]
    out = capsys.readouterr().out
    assert "apps-platform packages/device_key android: 1 commands ok" in out


def test_android_skips_without_device(
    tmp_path: Path, fake_adb: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    root = tmp_path / "repo"
    _android_member(root)
    assert apps_platform.main(["--root", str(root), "--system", "android"]) == 0
    assert _calls(fake_adb) == []
    out = capsys.readouterr().out
    assert "SKIP apps-platform packages/device_key android: no device" in out
    assert "SKIP apps-platform packages/device_key macos: other host" in out


def test_android_skips_without_adb(
    tmp_path: Path,
    fake_flutter: Path,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    root = tmp_path / "repo"
    _android_member(root)
    monkeypatch.setattr(apps_platform.shutil, "which", lambda name: None if name == "adb" else name)
    assert apps_platform.main(["--root", str(root), "--system", "linux"]) == 0
    assert "SKIP apps-platform packages/device_key android: no device" in capsys.readouterr().out


def test_android_fails_with_two_devices(
    tmp_path: Path,
    fake_adb: Path,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    root = tmp_path / "repo"
    _android_member(root)
    monkeypatch.setenv("FAKE_ADB_DEVICES", "emulator-5554,emulator-5556")
    assert apps_platform.main(["--root", str(root), "--system", "android"]) == 1
    assert _calls(fake_adb) == []
    err = capsys.readouterr().err
    assert "apps-platform packages/device_key android: 2 devices attached" in err


def test_explicit_system_argument(
    tmp_path: Path,
    fake_adb: Path,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """`--system android` runs only the Android commands, on any host (CI job apps-android)."""
    root = tmp_path / "repo"
    member = _android_member(root)
    monkeypatch.setenv("FAKE_ADB_DEVICES", "emulator-5554")
    monkeypatch.setattr(apps_platform.sys, "platform", "darwin")
    assert apps_platform.main(["--root", str(root), "--system", "android"]) == 0
    assert _calls(fake_adb) == [{"args": ["test", "android"], "cwd": str(member.resolve())}]
    out = capsys.readouterr().out
    assert "SKIP apps-platform packages/device_key macos: other host" in out


def test_android_is_a_known_system(tmp_path: Path) -> None:
    path = tmp_path / "platform-tests.json"
    path.write_text(json.dumps({"android": [["sh", "android/run.sh"]]}), encoding="utf-8")
    assert apps_platform.load(path) == {"android": [["sh", "android/run.sh"]]}


def test_native_assets_failure_builds_once_more(
    tmp_path: Path,
    fake_flutter: Path,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    # Main run 40169660725 (apps (Windows)): a build hook failed before the first test file
    # loaded; no test ran, so the command builds once more and its hook logs are printed.
    root = tmp_path / "repo"
    _member(root, "apps/client", {"windows": [["flutter", "test", "first"]]})
    hook = root / ".dart_tool/hooks_runner/sqlite3/abc/stderr.txt"
    hook.parent.mkdir(parents=True)
    hook.write_text("download failed: connection reset\n", encoding="utf-8")
    monkeypatch.setenv("FAKE_NATIVE_FAILURES", "1")
    assert apps_platform.main(["--root", str(root), "--system", "windows"]) == 0
    assert [c["args"] for c in _calls(fake_flutter)] == [["test", "first"], ["test", "first"]]
    captured = capsys.readouterr()
    assert "the native assets build failed before any test ran; building once more" in (
        captured.err
    )
    assert "hook log .dart_tool/hooks_runner/sqlite3/abc/stderr.txt" in captured.out
    assert "download failed: connection reset" in captured.out
    assert "apps-platform apps/client windows: 1 commands ok" in captured.out


def test_native_assets_failure_twice_fails(
    tmp_path: Path,
    fake_flutter: Path,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    root = tmp_path / "repo"
    _member(root, "apps/client", {"windows": [["flutter", "test", "first"], ["flutter", "x"]]})
    monkeypatch.setenv("FAKE_NATIVE_FAILURES", "2")
    assert apps_platform.main(["--root", str(root), "--system", "windows"]) == 1
    assert [c["args"] for c in _calls(fake_flutter)] == [["test", "first"], ["test", "first"]]
    assert "flutter test first failed with exit code 1" in capsys.readouterr().err


def test_a_failing_test_is_not_repeated(
    tmp_path: Path,
    fake_flutter: Path,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    # Only a build that failed in its native assets runs again, never a failed test.
    root = tmp_path / "repo"
    _member(root, "apps/client", {"windows": [["flutter", "test", "first"]]})
    monkeypatch.setenv("FAKE_EXIT_ON_FIRST", "1")
    assert apps_platform.main(["--root", str(root), "--system", "windows"]) == 1
    assert [c["args"] for c in _calls(fake_flutter)] == [["test", "first"]]
    assert "building once more" not in capsys.readouterr().err
