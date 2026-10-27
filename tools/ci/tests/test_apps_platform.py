# SPDX-License-Identifier: Apache-2.0
"""Tests of tools/ci/apps_platform.py, the platform tests of the apps (brief M4-01, note 6)."""

from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path

import apps_platform
import pytest

FAKE = """#!{python}
import json, os, sys, time
log = os.environ["FAKE_LOG"]
with open(log, "a", encoding="utf-8") as fh:
    fh.write(json.dumps({{"args": sys.argv[1:], "cwd": os.getcwd()}}) + "\\n")
with open(log, encoding="utf-8") as fh:
    calls = len(fh.read().splitlines())
if os.environ.get("FAKE_MARK"):
    sys.stdout.buffer.write(os.environ["FAKE_MARK"].encode() + b" Built client.exe\\n")
if calls <= int(os.environ.get("FAKE_NATIVE_FAILURES", "0")):
    print("Target build_hooks failed : error : Building native assets failed. See the logs.")
    sys.exit(1)
if calls <= int(os.environ.get("FAKE_HANG_CALLS", "0")):
    print("Xcode build done.                                           392.7s")
    if os.environ.get("FAKE_HANG_AFTER_RESULT"):
        # A failed test as test_core's GitHub reporter prints it, raw (the runner shows ##[group]).
        print("::group::\u274c a test (failed)")
        print("::endgroup::")
    sys.stdout.flush()
    time.sleep(3600)
if calls <= int(os.environ.get("FAKE_LOAD_FAILURES", "0")):
    print('Failed to load "/w/x_test.dart": Failed to start Dart Development Service')
    print("0 tests passed, 1 failed.")
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
log = os.environ.get("FAKE_ADB_LOG")
if log:
    with open(log, "a", encoding="utf-8") as fh:
        fh.write(" ".join(sys.argv[1:]) + "\\n")
if sys.argv[1:] in (["kill-server"], ["start-server"]):
    sys.exit(int(os.environ.get("FAKE_ADB_SERVER_EXIT", "0")))
if sys.argv[1:2] == ["-s"] and sys.argv[3:] == ["shell", "getprop", "sys.boot_completed"]:
    # The device reports a finished boot from the FAKE_ADB_BOOT_AFTER-th question on.
    with open(log, encoding="utf-8") as fh:
        asked = sum(1 for line in fh if line.endswith("sys.boot_completed\\n"))
    booted = asked >= int(os.environ.get("FAKE_ADB_BOOT_AFTER", "1"))
    sys.stdout.write("1\\n" if booted else "0\\n")
    sys.exit(0)
if sys.argv[1:2] == ["logcat"]:
    # -c clears the device log; -d prints it (FAKE_ADB_LOGCAT) and ends.
    if "-d" in sys.argv:
        sys.stdout.write(os.environ.get("FAKE_ADB_LOGCAT", ""))
    sys.exit(0)
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
    monkeypatch.setenv("FAKE_ADB_LOG", str(bin_dir.parent / "adb.log"))
    return fake_flutter


def _adb_calls(fake_adb: Path) -> list[str]:
    log = fake_adb.parent / "adb.log"
    return log.read_text(encoding="utf-8").splitlines() if log.exists() else []


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
    # Only a build that failed in its native assets or a file that failed to load runs again,
    # never a failed test.
    root = tmp_path / "repo"
    _member(root, "apps/client", {"windows": [["flutter", "test", "first"]]})
    monkeypatch.setenv("FAKE_EXIT_ON_FIRST", "1")
    assert apps_platform.main(["--root", str(root), "--system", "windows"]) == 1
    assert [c["args"] for c in _calls(fake_flutter)] == [["test", "first"]]
    assert "building once more" not in capsys.readouterr().err


def test_a_test_file_that_failed_to_load_runs_once_more(
    tmp_path: Path,
    fake_flutter: Path,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    # Main run 40209199826 (apps (Android API 36)): the test file failed to load ("Failed to
    # start Dart Development Service") after the app was installed; no test of it ran, so the
    # command runs once more.
    root = tmp_path / "repo"
    _member(root, "apps/client", {"android": [["flutter", "test", "first"]]})
    monkeypatch.setattr(apps_platform, "android_devices", lambda: 1)
    monkeypatch.setattr(apps_platform, "android_ready", lambda: None)
    monkeypatch.setenv("FAKE_LOAD_FAILURES", "1")
    assert apps_platform.main(["--root", str(root), "--system", "android"]) == 0
    assert [c["args"] for c in _calls(fake_flutter)] == [["test", "first"], ["test", "first"]]
    captured = capsys.readouterr()
    assert "a test file failed to load before any test ran; running once more" in captured.err
    assert "apps-platform apps/client android: 1 commands ok" in captured.out


def test_a_test_file_that_failed_to_load_twice_fails(
    tmp_path: Path,
    fake_flutter: Path,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    root = tmp_path / "repo"
    _member(root, "apps/client", {"windows": [["flutter", "test", "first"], ["flutter", "x"]]})
    monkeypatch.setenv("FAKE_LOAD_FAILURES", "2")
    assert apps_platform.main(["--root", str(root), "--system", "windows"]) == 1
    assert [c["args"] for c in _calls(fake_flutter)] == [["test", "first"], ["test", "first"]]
    assert "flutter test first failed with exit code 1" in capsys.readouterr().err


def test_a_command_that_stalls_while_the_app_starts_runs_once_more(
    tmp_path: Path,
    fake_flutter: Path,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    # Main run 40213460068 (apps (macOS)): after "Xcode build done." flutter waited for the VM
    # service of the app on the iOS simulator and printed nothing until the job timed out. A
    # command silent for SILENCE_SECONDS after its output began is stopped; when its last line
    # was the build of the app it runs once more. The fake never prints again, so the stop is
    # certain.
    root = tmp_path / "repo"
    _member(root, "apps/client", {"ios": [["flutter", "test", "first"]]})
    monkeypatch.setattr(apps_platform, "host_system", lambda: "macos")
    monkeypatch.setattr(apps_platform, "SILENCE_SECONDS", 0.2)
    monkeypatch.setenv("FAKE_HANG_CALLS", "1")
    assert apps_platform.main(["--root", str(root), "--system", "ios"]) == 0
    assert [c["args"] for c in _calls(fake_flutter)] == [["test", "first"], ["test", "first"]]
    captured = capsys.readouterr()
    assert "printed nothing for 0.2 s; stopped" in captured.err
    assert "stalled while the app started, before its test file ran; running once more" in (
        captured.err
    )
    assert "apps-platform apps/client ios: 1 commands ok" in captured.out


def test_a_command_that_stalls_after_a_failed_test_is_not_repeated(
    tmp_path: Path,
    fake_flutter: Path,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    root = tmp_path / "repo"
    _member(root, "apps/client", {"macos": [["flutter", "test", "first"]]})
    monkeypatch.setattr(apps_platform, "SILENCE_SECONDS", 0.2)
    monkeypatch.setenv("FAKE_HANG_CALLS", "1")
    monkeypatch.setenv("FAKE_HANG_AFTER_RESULT", "1")
    assert apps_platform.main(["--root", str(root), "--system", "macos"]) == 1
    assert [c["args"] for c in _calls(fake_flutter)] == [["test", "first"]]
    err = capsys.readouterr().err
    assert "printed nothing for 0.2 s; stopped" in err
    assert "running once more" not in err


def test_a_command_that_stalls_twice_fails(
    tmp_path: Path,
    fake_flutter: Path,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    root = tmp_path / "repo"
    _member(root, "apps/client", {"macos": [["flutter", "test", "first"]]})
    monkeypatch.setattr(apps_platform, "SILENCE_SECONDS", 0.2)
    monkeypatch.setenv("FAKE_HANG_CALLS", "2")
    assert apps_platform.main(["--root", str(root), "--system", "macos"]) == 1
    assert [c["args"] for c in _calls(fake_flutter)] == [["test", "first"], ["test", "first"]]
    assert "flutter test first failed" in capsys.readouterr().err


def test_ios_runs_in_its_own_job(
    tmp_path: Path,
    fake_flutter: Path,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    # The iOS simulator commands run with --system ios (CI job apps-ios); --system macos (CI job
    # apps-macos) runs only the desktop ones, and a Mac without --system runs both (PL/44).
    root = tmp_path / "repo"
    _member(root, "apps/client", {"macos": [["flutter", "mac"]], "ios": [["flutter", "sim"]]})
    monkeypatch.setattr(apps_platform, "host_system", lambda: "macos")
    assert apps_platform.main(["--root", str(root), "--system", "macos"]) == 0
    assert [c["args"] for c in _calls(fake_flutter)] == [["mac"]]
    assert "SKIP apps-platform apps/client ios: own job (--system ios)" in capsys.readouterr().out
    assert apps_platform.main(["--root", str(root), "--system", "ios"]) == 0
    assert [c["args"] for c in _calls(fake_flutter)] == [["mac"], ["sim"]]
    assert apps_platform.main(["--root", str(root)]) == 0
    assert [c["args"] for c in _calls(fake_flutter)] == [["mac"], ["sim"], ["mac"], ["sim"]]


def test_ios_needs_a_mac(
    tmp_path: Path,
    fake_flutter: Path,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    root = tmp_path / "repo"
    _member(root, "apps/client", {"ios": [["flutter", "sim"]]})
    monkeypatch.setattr(apps_platform, "host_system", lambda: "windows")
    assert apps_platform.main(["--root", str(root), "--system", "ios"]) == 0
    assert _calls(fake_flutter) == []
    assert "SKIP apps-platform apps/client ios: needs a macOS host" in capsys.readouterr().out


def test_output_passes_through_a_code_page_stdout(tmp_path: Path, fake_flutter: Path) -> None:
    # On the Windows runner Python's stdout is a cp1252 pipe; flutter prints marks outside it
    # ("✓ Built ..." after every desktop build, "√" in a legacy console) and the hook logs may
    # hold any bytes. The script passes them through instead of crashing.
    root = tmp_path / "repo"
    _member(root, "apps/client", {"windows": [["flutter", "test", "first"]]})
    hook = root / ".dart_tool/hooks_runner/sodium/abc/stdout.txt"
    hook.parent.mkdir(parents=True)
    hook.write_bytes("arrow → check ✓\n".encode())
    env = {
        **os.environ,
        "PYTHONIOENCODING": "cp1252",
        "PYTHONUTF8": "0",
        "FAKE_MARK": "✓ √",
        "FAKE_NATIVE_FAILURES": "1",
    }
    script = Path(apps_platform.__file__)
    run = subprocess.run(
        [sys.executable, str(script), "--root", str(root), "--system", "windows"],
        env=env,
        capture_output=True,
        check=False,
    )
    assert run.returncode == 0, run.stderr.decode(errors="replace")
    assert "✓ √ Built".encode() in run.stdout
    assert "arrow → check ✓".encode() in run.stdout
    assert b"apps-platform apps/client windows: 1 commands ok" in run.stdout


@pytest.mark.parametrize(
    ("line", "starting"),
    [
        # The last lines before the tests of the logs of main runs 40213460068 and 40209199826.
        (b"Xcode build done.                                           392.7s\n", True),
        ("\u2713 Built build/macos/Build/Products/Debug/ExampleChat.app\n".encode(), True),
        (b"Installing build/app/outputs/flutter-apk/app-debug.apk...          827ms\n", True),
        ("::group::\u274c a test (failed)\n".encode(), False),
        (b"::endgroup::\n", False),
        (b"> Task :app:connectedDebugAndroidTest\n", False),
    ],
)
def test_app_starting_lines(line: bytes, starting: bool) -> None:
    assert bool(apps_platform.APP_STARTING.match(line)) is starting


def test_the_silence_limit_is_the_documented_one() -> None:
    # PL/44 and the changelog name 15 minutes; the longest quiet step of a passing command (the
    # Xcode build of the iOS app) took more than 7 in main run 40213460068.
    assert apps_platform.SILENCE_SECONDS == 15 * 60


class FakeClock:
    """The clock of android_ready: sleeping advances it, nothing waits for real time."""

    def __init__(self) -> None:
        self.now = 0.0

    def monotonic(self) -> float:
        return self.now

    def sleep(self, seconds: float) -> None:
        self.now += seconds


@pytest.fixture
def fake_clock(monkeypatch: pytest.MonkeyPatch) -> FakeClock:
    clock = FakeClock()
    monkeypatch.setattr(apps_platform, "_monotonic", clock.monotonic)
    monkeypatch.setattr(apps_platform, "_sleep", clock.sleep)
    return clock


def test_android_restarts_adb_and_waits_for_the_boot_before_each_attempt(
    tmp_path: Path,
    fake_adb: Path,
    fake_clock: FakeClock,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    # Main run 40318886842 (apps (Android API 36)): the adb server lost the emulator during the
    # first attempt (VmServiceDisappearedException), and the second attempt found no device in
    # state `device`. Each attempt first restarts the adb server and waits until the one device
    # is listed and reports a finished boot (PL/51).
    root = tmp_path / "repo"
    _member(root, "apps/client", {"android": [["flutter", "test", "first"]]})
    monkeypatch.setenv("FAKE_ADB_DEVICES", "emulator-5554")
    monkeypatch.setenv("FAKE_ADB_BOOT_AFTER", "3")
    monkeypatch.setenv("FAKE_LOAD_FAILURES", "1")
    assert apps_platform.main(["--root", str(root), "--system", "android"]) == 0
    assert [c["args"] for c in _calls(fake_adb)] == [["test", "first"], ["test", "first"]]
    calls = [c for c in _adb_calls(fake_adb) if c != "devices"]
    ready = ["kill-server", "start-server"]
    boot = "-s emulator-5554 shell getprop sys.boot_completed"
    # First attempt: the boot is reported at the third question; second attempt: at once.
    clear = "logcat -b all -c"
    dump = f"logcat -d -b main,system,crash -t {apps_platform.ANDROID_LOG_LINES} *:I"
    assert calls == [*ready, boot, boot, boot, clear, dump, *ready, boot, clear]
    assert "apps-platform apps/client android: 1 commands ok" in capsys.readouterr().out


def test_android_device_that_never_becomes_ready_fails_clearly(
    tmp_path: Path,
    fake_adb: Path,
    fake_clock: FakeClock,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    root = tmp_path / "repo"
    _member(root, "apps/client", {"android": [["flutter", "test", "first"]]})
    monkeypatch.setenv("FAKE_ADB_DEVICES", "emulator-5554")
    monkeypatch.setenv("FAKE_ADB_BOOT_AFTER", "1000000")
    assert apps_platform.main(["--root", str(root), "--system", "android"]) == 1
    assert _calls(fake_adb) == []
    assert fake_clock.now >= apps_platform.ANDROID_READY_SECONDS
    err = capsys.readouterr().err
    assert (
        "apps-platform apps/client android: the device did not become ready within"
        f" {apps_platform.ANDROID_READY_SECONDS:g} s after an adb server restart"
        " (emulator-5554: sys.boot_completed is not 1)" in err
    )


def test_android_device_lost_after_the_restart_fails_clearly(
    tmp_path: Path,
    fake_adb: Path,
    fake_clock: FakeClock,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    root = tmp_path / "repo"
    _member(root, "apps/client", {"android": [["flutter", "test", "first"]]})
    # main counted one device; after the restart of the adb server it is offline.
    monkeypatch.setattr(apps_platform, "android_devices", lambda: 1)
    monkeypatch.setenv("FAKE_ADB_DEVICES", "emulator-5554:offline")
    assert apps_platform.main(["--root", str(root), "--system", "android"]) == 1
    assert _calls(fake_adb) == []
    assert "(no device in state device: emulator-5554 offline)" in capsys.readouterr().err


def test_android_adb_server_that_does_not_start_fails_clearly(
    tmp_path: Path,
    fake_adb: Path,
    fake_clock: FakeClock,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    root = tmp_path / "repo"
    _member(root, "apps/client", {"android": [["flutter", "test", "first"]]})
    monkeypatch.setenv("FAKE_ADB_DEVICES", "emulator-5554")
    monkeypatch.setenv("FAKE_ADB_SERVER_EXIT", "1")
    assert apps_platform.main(["--root", str(root), "--system", "android"]) == 1
    assert _calls(fake_adb) == []
    assert "(adb start-server failed with exit code 1)" in capsys.readouterr().err


def test_android_prints_the_device_log_of_a_failed_attempt(
    tmp_path: Path,
    fake_adb: Path,
    fake_clock: FakeClock,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    # Main runs 40318886842 and 40445419021 (apps (Android API 36)): the VM service of the test
    # app disappeared right after the install, also after an adb restart, and the job log showed
    # nothing of the device. Each attempt starts with an empty device log, and a failed one prints
    # it, so the next failure names its cause (a crash, a killed process).
    root = tmp_path / "repo"
    _member(root, "apps/client", {"android": [["flutter", "test", "first"]]})
    monkeypatch.setenv("FAKE_ADB_DEVICES", "emulator-5554")
    monkeypatch.setenv("FAKE_LOAD_FAILURES", "1")
    monkeypatch.setenv("FAKE_ADB_LOGCAT", "F libc    : Fatal signal 6 (SIGABRT)\n")
    assert apps_platform.main(["--root", str(root), "--system", "android"]) == 0
    calls = [c for c in _adb_calls(fake_adb) if c != "devices" and "getprop" not in c]
    clear = "logcat -b all -c"
    dump = f"logcat -d -b main,system,crash -t {apps_platform.ANDROID_LOG_LINES} *:I"
    ready = ["kill-server", "start-server"]
    # The failed first attempt prints its log; the passing second one does not.
    assert calls == [*ready, clear, dump, *ready, clear]
    out = capsys.readouterr().out
    assert out.count("F libc    : Fatal signal 6 (SIGABRT)") == 1
    assert "apps-platform apps/client android: device log of the failed attempt" in out
