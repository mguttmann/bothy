# SPDX-License-Identifier: Apache-2.0
"""Platform tests of the apps and packages (brief M4-01, Design note 6; `make apps-platform`).

The pure gates run on Linux. Native code and desktop integration tests need macOS and Windows,
so a member that has them lists them in `platform-tests.json` next to its `pubspec.yaml`:

    {"macos": [["flutter", "test", "integration_test", "-d", "macos"]], "windows": [...]}

Each command is an argument list without a shell. On a host this script runs the commands for
its own system in the member folder, in order, and stops at the first failure; for the other
system it prints a SKIP line. The CI jobs `apps-macos` (macos-15) and `apps-windows`
(windows-2025) call it; it is not part of `make gates`, because it needs a desktop session.

Usage: apps_platform.py [--root <repository>] [--system macos|windows]
Exit 0 when every command of the host passed (or nothing was to run), 1 when one failed, 2 when
a `platform-tests.json` is invalid.
"""

from __future__ import annotations

import argparse
import json
import shutil
import subprocess
import sys
from pathlib import Path

SYSTEMS = ("macos", "windows")
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


def _members(root: Path) -> list[Path]:
    files = [*root.glob("apps/*/platform-tests.json"), *root.glob("packages/*/platform-tests.json")]
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
    for name, folder, tests in plans:
        for other in (s for s in SYSTEMS if s in tests and s != system):
            print(f"SKIP apps-platform {name} {other}: other host")
        commands = tests.get(system, [])
        for command in commands:
            # Without a shell, Windows does not resolve `flutter` to `flutter.bat`; which does.
            program = shutil.which(command[0])
            if program is None:
                print(
                    f"apps-platform {name} {system}: {command[0]} is not on PATH", file=sys.stderr
                )
                return 1
            result = subprocess.run([program, *command[1:]], cwd=folder, check=False)
            if result.returncode != 0:
                print(
                    f"apps-platform {name} {system}: {' '.join(command)} failed with exit code"
                    f" {result.returncode}",
                    file=sys.stderr,
                )
                return 1
        if commands:
            print(f"apps-platform {name} {system}: {len(commands)} commands ok")
    return 0


if __name__ == "__main__":
    sys.exit(main())
