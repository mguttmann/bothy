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
target would be a guess. `--system android` runs only the Android commands. The CI jobs
`apps-macos` (macos-15), `apps-windows` (windows-2025) and `apps-android` (ubuntu-24.04 with an
emulator) call it; it is not part of `make gates`, because it needs a desktop session or a device.

A command whose output reports a failed native assets build (a build hook of a package such as
`sqlite3` or `sodium`, which download their libraries) failed before any test ran: the script
prints the logs of the hook runs and runs that command once more (PL/41). A command whose output
reports a test file that failed to load (`Failed to load "<file>": ...`, for example when the Dart
Development Service did not start on the device) failed before any test of that file ran and also
runs once more (PL/44); every `flutter test` on a device runs one test file, so no test of the
command ran. A failed test never runs again.

Usage: apps_platform.py [--root <repository>] [--system macos|windows|android]
Exit 0 when every command that was to run passed (or nothing was to run), 1 when one failed or
the Android target is ambiguous, 2 when a `platform-tests.json` is invalid.
"""

from __future__ import annotations

import argparse
import json
import shutil
import subprocess
import sys
from pathlib import Path

SYSTEMS = ("macos", "windows", "android")
# Systems that are a host; `android` runs on any host with one attached device.
HOSTS = ("macos", "windows")
ANDROID = "android"
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


# The line flutter prints when a build hook of a package (native assets: sqlite3, sodium) failed.
# The build runs before the test file loads, so no test ran and one more build is safe.
NATIVE_ASSETS_FAILED = b"Building native assets failed"
# The line flutter prints when a test file did not load, so none of its tests ran (PL/44).
LOAD_FAILED = b'Failed to load "'
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


def _execute(argv: list[str], folder: Path) -> tuple[int, bool, bool]:
    """Run argv in folder, echo its output, and report the native assets and load failures."""
    native_failed = load_failed = False
    with subprocess.Popen(
        argv, cwd=folder, stdout=subprocess.PIPE, stderr=subprocess.STDOUT
    ) as proc:
        for line in proc.stdout or ():
            _emit(line)
            native_failed = native_failed or NATIVE_ASSETS_FAILED in line
            load_failed = load_failed or LOAD_FAILED in line
    return proc.returncode, native_failed, load_failed


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
        code, native_failed, load_failed = _execute([program, *command[1:]], folder)
        if code != 0 and native_failed:
            _print_hook_logs(root)
            print(
                f"apps-platform {name} {system}: the native assets build failed before any test"
                " ran; building once more",
                file=sys.stderr,
            )
            code, _, _ = _execute([program, *command[1:]], folder)
        elif code != 0 and load_failed:
            print(
                f"apps-platform {name} {system}: a test file failed to load before any test ran;"
                " running once more",
                file=sys.stderr,
            )
            code, _, _ = _execute([program, *command[1:]], folder)
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
    system = args.system or host_system() or sys.platform
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
        if ANDROID not in tests:
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
