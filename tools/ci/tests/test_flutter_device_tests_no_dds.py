# SPDX-License-Identifier: Apache-2.0
"""Every `flutter test` on a device runs without the Dart Development Service (PL/44).

With DDS, flutter connects DDS to the VM service of the app in one attempt right after the app
started; on a slow device (the Android emulator of main run 40209199826) the forwarded VM service
was not ready yet and the file failed with "Failed to start Dart Development Service" before any
test ran. With `--no-dds` flutter connects to the VM service directly, with its own retries.
The check reads the real files: every `platform-tests.json` and every tracked shell script.
"""

from __future__ import annotations

import json
import re
import subprocess
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]
# A shell line that runs `flutter test` on a named device (`-d <device>`).
_SHELL_DEVICE_TEST = re.compile(r"\bflutter\s+test\b.*\s-d\s")


def _tracked(*patterns: str) -> list[Path]:
    out = subprocess.run(
        ["git", "ls-files", "-z", "--", *patterns],
        cwd=REPO,
        check=True,
        capture_output=True,
    ).stdout
    return [REPO / name for name in out.decode().split("\0") if name]


def device_test_commands() -> list[tuple[str, str]]:
    """(place, command) of every `flutter test` on a device in the platform tests and scripts."""
    found: list[tuple[str, str]] = []
    for path in _tracked("*platform-tests.json"):
        tests = json.loads(path.read_text(encoding="utf-8"))
        for commands in tests.values():
            for argv in commands:
                if argv[:2] == ["flutter", "test"] and "-d" in argv:
                    found.append((str(path.relative_to(REPO)), " ".join(argv)))
    for path in _tracked("*.sh"):
        for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
            code = line.split("#", 1)[0]
            if _SHELL_DEVICE_TEST.search(code):
                found.append((f"{path.relative_to(REPO)}:{number}", code.strip()))
    return found


def test_the_scan_finds_the_device_tests() -> None:
    places = {place.split(":", 1)[0] for place, _ in device_test_commands()}
    assert {
        "apps/client/platform-tests.json",
        "apps/client/tool/run_android_tests.sh",
        "apps/client/tool/run_ios_integration_tests.sh",
    } <= places


def test_every_flutter_test_on_a_device_runs_without_dds() -> None:
    missing = [
        f"{place}: {command}"
        for place, command in device_test_commands()
        if "--no-dds" not in command.split()
    ]
    assert missing == [], "add --no-dds (PL/44):\n" + "\n".join(missing)
