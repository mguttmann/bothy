# SPDX-License-Identifier: Apache-2.0
"""Every `flutter test` on a device runs exactly one test file (PL/44).

`tools/ci/apps_platform.py` runs a command once more when a test file failed to load: with one
file per command that means no test of the command ran, so no failed test runs again. The check
reads the real files: every tracked `platform-tests.json` and every tracked shell script.
"""

from __future__ import annotations

import json
import re
import shlex
import subprocess
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]
# A shell line that runs `flutter test` on a named device (`-d <device>`).
_SHELL_DEVICE_TEST = re.compile(r"\bflutter\s+test\b.*\s-d\s")
# Options of `flutter test` that take a value as the next argument.
_VALUE_OPTIONS = {"-d", "--device-id", "--dart-define", "--plain-name", "--name", "-n"}


def _tracked(pattern: str) -> list[Path]:
    out = subprocess.run(
        ["git", "ls-files", "-z", "--", pattern],
        cwd=REPO,
        check=True,
        capture_output=True,
    ).stdout
    return [REPO / name for name in out.decode().split("\0") if name]


def device_test_commands() -> list[tuple[str, list[str]]]:
    """(place, argv) of every `flutter test` on a device in the platform tests and scripts."""
    found: list[tuple[str, list[str]]] = []
    for path in _tracked("*platform-tests.json"):
        tests = json.loads(path.read_text(encoding="utf-8"))
        for commands in tests.values():
            for argv in commands:
                if argv[:2] == ["flutter", "test"] and "-d" in argv:
                    found.append((str(path.relative_to(REPO)), argv))
    for path in _tracked("*.sh"):
        for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
            if _SHELL_DEVICE_TEST.search(line):
                argv = shlex.split(line, comments=True)
                start = argv.index("flutter")
                found.append((f"{path.relative_to(REPO)}:{number}", argv[start:]))
    return found


def positional(argv: list[str]) -> list[str]:
    """The positional arguments of `flutter test`: the test files or folders it runs."""
    files: list[str] = []
    skip = False
    for arg in argv[2:]:
        if skip:
            skip = False
        elif arg in _VALUE_OPTIONS:
            skip = True
        elif not arg.startswith("-"):
            files.append(arg)
    return files


def test_the_scan_finds_the_device_tests() -> None:
    places = {place.split(":", 1)[0] for place, _ in device_test_commands()}
    assert {
        "apps/client/platform-tests.json",
        "apps/client/tool/run_android_tests.sh",
        "apps/client/tool/run_ios_integration_tests.sh",
    } <= places


def test_positional_arguments_are_the_test_files() -> None:
    argv = ["flutter", "test", "a_test.dart", "-d", "macos", "--no-dds", "b"]
    assert positional(argv) == ["a_test.dart", "b"]


def test_every_flutter_test_on_a_device_runs_one_test_file() -> None:
    wrong = [
        f"{place}: {' '.join(argv)}"
        for place, argv in device_test_commands()
        if len(files := positional(argv)) != 1 or not files[0].endswith(".dart")
    ]
    assert wrong == [], "run one test file per command (PL/44):\n" + "\n".join(wrong)
