# SPDX-License-Identifier: Apache-2.0
"""Platform tests of the apps and packages (brief M4-01, Design note 6; `make apps-platform`).

The pure gates run on Linux. Native code and desktop integration tests need macOS and Windows,
and Android code needs a device or emulator, so a member that has them lists them in
`platform-tests.json` next to its `pubspec.yaml`:

    {"macos": [["flutter", "test", "integration_test", "-d", "macos"]], "windows": [...],
     "android": [["sh", "android/run_instrumented_tests.sh"]]}

Each command is an argument list without a shell. On a host this script runs the commands for
its own system in the member folder, in order, and stops at the first failure; for the other
desktop system it prints a SKIP line. The system `android` is no host (brief M5-01, Design note
5): its commands run when `adb devices` lists exactly one device or emulator, on any host; without
one they print `SKIP ... android: no device`, and with two or more the run fails, because the
target would be a guess. `--system android` runs only the Android commands. The system `ios`
(the iOS simulator) is no host either: its commands need a macOS host with Xcode and run with
`--system ios`, or on a Mac without `--system` after the `macos` ones; `--system macos` skips them
(PL/44). The CI jobs `apps-macos` (`--system macos`), `apps-ios` (`--system ios`, both macos-15),
`apps-windows` (windows-2025) and `apps-android` (ubuntu-24.04 with an emulator) call it; it is not
part of `make gates`, because it needs a desktop session or a device.

A command whose output reports a failed native assets build (a build hook of a package such as
`sqlite3` or `sodium`, which download their libraries) failed before any test ran: the script
prints the logs of the hook runs and runs that command once more (PL/41). A command whose output
reports a test file that failed to load (`Failed to load "<file>": ...`, for example when the Dart
Development Service did not start on the device) failed before any test of that file ran and also
runs once more (PL/44); every `flutter test` on a device runs one test file, so no test of the
command ran. A command that printed nothing for `SILENCE_SECONDS` after its output began is
stopped with its process tree; when its last line was the build or install of the app, it stalled
before its test file started (flutter waits without a limit for the VM service of an app on the
iOS simulator) and runs once more (PL/44). Before each attempt of an Android command the script
restarts the adb server and waits up to ANDROID_READY_SECONDS until the one device is listed in
state `device` and reports `sys.boot_completed` 1; otherwise that attempt fails with the reason
and the command does not run (PL/51); the device log is cleared then, and a failed Android
attempt prints its last ANDROID_LOG_LINES lines. A failed test never runs again.

Usage: apps_platform.py [--root <repository>] [--system macos|windows|android|ios]
Exit 0 when every command that was to run passed (or nothing was to run), 1 when one failed or
the Android target is ambiguous, 2 when a `platform-tests.json` is invalid.
"""

from __future__ import annotations

import argparse
import contextlib
import json
import os
import queue
import re
import shutil
import signal
import subprocess
import sys
import threading
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any

SYSTEMS = ("macos", "windows", "android", "ios")
# Systems that are a host; `android` runs on any host with one attached device, `ios` on macOS.
HOSTS = ("macos", "windows")
ANDROID = "android"
IOS = "ios"
_PLATFORMS = {"darwin": "macos", "win32": "windows"}


class PlatformTestsError(Exception):
    """A `platform-tests.json` does not have the expected form."""


def host_system(platform: str = sys.platform) -> str | None:
    """The system name of `platform` (`sys.platform`), or None for a host without app tests."""
    return _PLATFORMS.get(platform)


def load(path: Path) -> dict[str, list[list[str]]]:
    """Read and validate one `platform-tests.json`."""
    data = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(data, dict):
        raise PlatformTestsError("must be an object keyed by system")
    for system, commands in data.items():
        if system not in SYSTEMS:
            raise PlatformTestsError(f"unknown system {system!r} (known: {', '.join(SYSTEMS)})")
        if not isinstance(commands, list):
            raise PlatformTestsError(f"{system}: must be a list of commands")
        for command in commands:
            if (
                not isinstance(command, list)
                or not command
                or not all(isinstance(a, str) for a in command)
            ):
                raise PlatformTestsError(
                    f"{system}: every command is a non-empty list of strings, not {command!r}"
                )
    return data


def android_devices() -> int:
    """The number of devices and emulators `adb devices` lists as ready (state `device`).

    0 when `adb` is not on PATH or fails: a host without the Android tools has no device.
    """
    adb = shutil.which("adb")
    if adb is None:
        return 0
    result = subprocess.run([adb, "devices"], capture_output=True, text=True, check=False)
    if result.returncode != 0:
        return 0
    lines = result.stdout.splitlines()[1:]
    return sum(1 for line in lines if line.split("\t")[1:2] == ["device"])


# How long a device may take after a restart of the adb server until it is listed in state
# `device` and reports sys.boot_completed 1 (PL/51); the emulator of the CI job has booted
# before the script starts, so a healthy one is ready within seconds.
ANDROID_READY_SECONDS = 5 * 60
# The pause between two questions to adb while it waits, and the limit of one adb call.
ANDROID_READY_POLL_SECONDS = 2.0
ADB_CALL_SECONDS = 60
# The clock of android_ready; the tests replace it.
_monotonic = time.monotonic
_sleep = time.sleep


def _adb(adb: str, *args: str) -> subprocess.CompletedProcess[str] | None:
    """Run one adb command; None when it did not end within ADB_CALL_SECONDS."""
    try:
        return subprocess.run(
            [adb, *args], capture_output=True, text=True, check=False, timeout=ADB_CALL_SECONDS
        )
    except subprocess.TimeoutExpired:
        return None


def _android_state(adb: str) -> str | None:
    """None when exactly one device is in state `device` and has booted, else what is missing."""
    listed = _adb(adb, "devices")
    if listed is None or listed.returncode != 0:
        return "adb devices failed"
    entries = [line.split("\t") for line in listed.stdout.splitlines()[1:] if "\t" in line]
    ready = [entry[0] for entry in entries if entry[1:2] == ["device"]]
    if len(ready) != 1:
        others = ", ".join(" ".join(entry) for entry in entries) or "none listed"
        return f"no device in state device: {others}" if not ready else "more than one device"
    boot = _adb(adb, "-s", ready[0], "shell", "getprop", "sys.boot_completed")
    if boot is None or boot.returncode != 0 or boot.stdout.strip() != "1":
        return f"{ready[0]}: sys.boot_completed is not 1"
    return None


def android_ready() -> str | None:
    """Restart the adb server and wait until its one device is ready (PL/51).

    A device that the adb server lost (main run 40318886842: the VM service of the test app
    disappeared, then `adb devices` listed nothing in state `device`) comes back only through a
    new server. Returns None when the device is listed in state `device` and reports
    sys.boot_completed 1 within ANDROID_READY_SECONDS, else the reason it is not ready.
    """
    adb = shutil.which("adb")
    if adb is None:
        return "adb is not on PATH"
    _adb(adb, "kill-server")
    started = _adb(adb, "start-server")
    if started is None or started.returncode != 0:
        code = "a timeout" if started is None else f"exit code {started.returncode}"
        return f"adb start-server failed with {code}"
    deadline = _monotonic() + ANDROID_READY_SECONDS
    while True:
        state = _android_state(adb)
        if state is None:
            # The attempt starts with an empty device log, so a failed one prints its own.
            _adb(adb, "logcat", "-b", "all", "-c")
            return None
        if _monotonic() >= deadline:
            return state
        _sleep(ANDROID_READY_POLL_SECONDS)


# The lines of the device log (logcat, priority info and above of the main, system and crash
# buffers) printed after a failed Android attempt: the job log shows nothing of the device
# otherwise, so a test app that crashed or was killed looked like a lost VM service.
ANDROID_LOG_LINES = 400


def print_android_log(label: str) -> None:
    """Print the end of the device log after a failed Android attempt (or why it is missing)."""
    adb = shutil.which("adb")
    dump = None
    if adb is not None:
        argv = [adb, "logcat", "-d", "-b", "main,system,crash", "-t", str(ANDROID_LOG_LINES), "*:I"]
        # Bytes, not text: the log of an app may hold anything, and _emit passes it on unchanged.
        with contextlib.suppress(subprocess.TimeoutExpired):
            dump = subprocess.run(argv, capture_output=True, check=False, timeout=ADB_CALL_SECONDS)
    if dump is None or dump.returncode != 0:
        print(f"apps-platform {label}: no device log (adb logcat failed)", file=sys.stderr)
        return
    header = f"apps-platform {label}: device log of the failed attempt (adb logcat):\n"
    _emit(header.encode() + dump.stdout)


# The line flutter prints when a build hook of a package (native assets: sqlite3, sodium) failed.
# The build runs before the test file loads, so no test ran and one more build is safe.
NATIVE_ASSETS_FAILED = b"Building native assets failed"
# The line flutter prints when a test file did not load, so none of its tests ran (PL/44).
LOAD_FAILED = b'Failed to load "'
# A command that prints nothing for this long after its output began is stopped (PL/44). The
# longest silence of a passing command is the Xcode build of the iOS app with its Swift package
# fetch (more than 7 minutes in main run 40213460068).
SILENCE_SECONDS = 15 * 60
# The last lines flutter prints before the app starts the test file on a device: the end of the
# iOS build, the end of a desktop or Android build, the Android install. A command that stalls
# right after one of them stalled before its test file started (PL/44).
APP_STARTING = re.compile(rb"^(?:Xcode build done\.|\xe2\x9c\x93 Built |Installing )")
# Where the hooks runner keeps the output of each hook run of the workspace.
HOOK_LOGS = ".dart_tool/hooks_runner"
# The lines of each hook log that are printed.
HOOK_LOG_LINES = 40


def _emit(data: bytes) -> None:
    """Write data unchanged to stdout: no decoding and no re-encoding.

    On the Windows runner Python's stdout is a pipe in the ANSI code page, which cannot encode the
    marks flutter prints (for example the check mark of "Built"), so text would crash the script.
    """
    sys.stdout.flush()
    sys.stdout.buffer.write(data)
    sys.stdout.buffer.flush()


@dataclass
class Outcome:
    """What one run of a command showed."""

    code: int
    native_failed: bool = False
    load_failed: bool = False
    stalled: bool = False
    # The last line printed before the end or the stall.
    last_line: bytes = b""


def _stop_tree(proc: subprocess.Popen[bytes]) -> None:
    """Kill the command and every process it started (it runs in its own process group)."""
    if sys.platform == "win32":
        subprocess.run(
            ["taskkill", "/F", "/T", "/PID", str(proc.pid)], capture_output=True, check=False
        )
    else:
        # The group may have ended between the last look and the kill.
        with contextlib.suppress(ProcessLookupError, PermissionError):
            os.killpg(proc.pid, signal.SIGKILL)


def _execute(argv: list[str], folder: Path) -> Outcome:
    """Run argv in folder, echo its output, and report what it showed.

    Once the command printed its first line, a silence of SILENCE_SECONDS stops it with its
    process tree. A command whose own process ended while a process it started still holds the
    output open is done, not stalled.
    """
    group: dict[str, Any]
    if sys.platform == "win32":
        group = {"creationflags": subprocess.CREATE_NEW_PROCESS_GROUP}
    else:
        group = {"start_new_session": True}
    proc = subprocess.Popen(
        argv, cwd=folder, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, **group
    )
    lines: queue.Queue[bytes | None] = queue.Queue()

    def pump() -> None:
        for line in proc.stdout or ():
            lines.put(line)
        lines.put(None)

    reader = threading.Thread(target=pump, daemon=True)
    reader.start()
    outcome = Outcome(code=0)
    last: float | None = None
    while True:
        try:
            line = lines.get(timeout=SILENCE_SECONDS if last is None else 1.0)
        except queue.Empty:
            if proc.poll() is not None:
                break
            if last is not None and time.monotonic() - last >= SILENCE_SECONDS:
                outcome.stalled = True
                _stop_tree(proc)
                break
            continue
        if line is None:
            break
        last = time.monotonic()
        _emit(line)
        outcome.native_failed = outcome.native_failed or NATIVE_ASSETS_FAILED in line
        outcome.load_failed = outcome.load_failed or LOAD_FAILED in line
        outcome.last_line = line
    outcome.code = proc.wait()
    # A process the command started and that left its group may still hold the output open;
    # then the reader stays behind and the pipe is left to the end of this script.
    reader.join(timeout=5)
    if not reader.is_alive() and proc.stdout is not None:
        proc.stdout.close()
    if outcome.stalled and outcome.code == 0:
        outcome.code = 1
    return outcome


def _attempt(name: str, system: str, command: list[str], program: str, folder: Path) -> Outcome:
    """Run command once with its resolved program and say when it was stopped as stalled.

    On Android the device is made ready first (android_ready); a device that does not become
    ready fails the attempt with the reason, before the command runs.
    """
    label = f"{name} {system}"
    if system == ANDROID:
        reason = android_ready()
        if reason is not None:
            print(
                f"apps-platform {label}: the device did not become ready within"
                f" {ANDROID_READY_SECONDS:g} s after an adb server restart ({reason})",
                file=sys.stderr,
            )
            return Outcome(code=1)
    outcome = _execute([program, *command[1:]], folder)
    if system == ANDROID and outcome.code != 0:
        print_android_log(label)
    if outcome.stalled:
        print(
            f"apps-platform {label}: {' '.join(command)} printed nothing for"
            f" {SILENCE_SECONDS:g} s; stopped",
            file=sys.stderr,
        )
    return outcome


def _print_hook_logs(root: Path) -> None:
    """Print the end of every non-empty log of a hook run, the details flutter only points to."""
    for log in sorted((root / HOOK_LOGS).glob("*/*/std*.txt")):
        lines = log.read_bytes().strip().splitlines()
        if lines:
            header = f"apps-platform hook log {log.relative_to(root).as_posix()}:\n"
            _emit(header.encode() + b"\n".join(lines[-HOOK_LOG_LINES:]) + b"\n")


def _run(name: str, folder: Path, system: str, commands: list[list[str]], root: Path) -> bool:
    for command in commands:
        # Without a shell, Windows does not resolve `flutter` to `flutter.bat`; which does.
        program = shutil.which(command[0])
        if program is None:
            print(f"apps-platform {name} {system}: {command[0]} is not on PATH", file=sys.stderr)
            return False
        outcome = _attempt(name, system, command, program, folder)
        code = outcome.code
        if code != 0 and outcome.native_failed:
            _print_hook_logs(root)
            print(
                f"apps-platform {name} {system}: the native assets build failed before any test"
                " ran; building once more",
                file=sys.stderr,
            )
            code = _attempt(name, system, command, program, folder).code
        elif code != 0 and outcome.load_failed:
            print(
                f"apps-platform {name} {system}: a test file failed to load before any test ran;"
                " running once more",
                file=sys.stderr,
            )
            code = _attempt(name, system, command, program, folder).code
        elif outcome.stalled and APP_STARTING.match(outcome.last_line):
            print(
                f"apps-platform {name} {system}: it stalled while the app started, before its"
                " test file ran; running once more",
                file=sys.stderr,
            )
            code = _attempt(name, system, command, program, folder).code
        if code != 0:
            print(
                f"apps-platform {name} {system}: {' '.join(command)} failed with exit code {code}",
                file=sys.stderr,
            )
            return False
    if commands:
        print(f"apps-platform {name} {system}: {len(commands)} commands ok")
    return True


def _members(root: Path) -> list[Path]:
    # One level under apps/ and packages/, plus the nested harness members (packages/*/harness,
    # docs/DECISIONS.md D-M6-11-6), as the workspace lists them.
    files = [
        *root.glob("apps/*/platform-tests.json"),
        *root.glob("packages/*/platform-tests.json"),
        *root.glob("packages/*/harness/platform-tests.json"),
    ]
    return sorted(f for f in files if (f.parent / "pubspec.yaml").is_file())


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; see the module documentation."""
    parser = argparse.ArgumentParser(description="Run the platform tests of the host system.")
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[2])
    parser.add_argument("--system", choices=[*SYSTEMS, "linux"], default=None)
    args = parser.parse_args(argv)
    host = host_system()
    system = args.system or host or sys.platform
    files = _members(args.root)
    if not files:
        print("SKIP apps-platform: no member with platform tests")
        return 0
    plans: list[tuple[str, Path, dict[str, list[list[str]]]]] = []
    for file in files:
        rel = file.relative_to(args.root).as_posix()
        try:
            plans.append((file.parent.relative_to(args.root).as_posix(), file.parent, load(file)))
        except (PlatformTestsError, json.JSONDecodeError) as exc:
            print(f"apps-platform: {rel}: {exc}", file=sys.stderr)
            return 2
    devices: int | None = None
    for name, folder, tests in plans:
        for other in (s for s in HOSTS if s in tests and s != system):
            print(f"SKIP apps-platform {name} {other}: other host")
        if not _run(
            name, folder, system, tests.get(system, []) if system in HOSTS else [], args.root
        ):
            return 1
        if IOS in tests:
            if system != IOS and (args.system is not None or host != "macos"):
                if system == "macos":
                    print(f"SKIP apps-platform {name} {IOS}: own job (--system ios)")
            elif host != "macos":
                print(f"SKIP apps-platform {name} {IOS}: needs a macOS host")
            elif not _run(name, folder, IOS, tests[IOS], args.root):
                return 1
        if ANDROID not in tests or system == IOS:
            continue
        if devices is None:
            devices = android_devices()
        if devices == 0:
            print(f"SKIP apps-platform {name} {ANDROID}: no device")
        elif devices > 1:
            print(
                f"apps-platform {name} {ANDROID}: {devices} devices attached; attach exactly one",
                file=sys.stderr,
            )
            return 1
        elif not _run(name, folder, ANDROID, tests[ANDROID], args.root):
            return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
