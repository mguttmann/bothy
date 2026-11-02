# SPDX-License-Identifier: Apache-2.0
"""The Android device tests run as instrumented tests, without a host connection to the app's VM
service (FIX-android36-dds-start).

On the API 36 emulator of the CI, `flutter test integration_test/phone_suite_test.dart -d
<serial>` failed to start in main runs 40209199826, 40318886842, 41308492022 and 41443318002
(the last one in all three attempts of FAPS/1): every time the device log shows `adbd:
host-18: connection terminated: write failed` 0.3 to 1 s after `The Dart VM service is
listening`, while flutter talked to the VM service through `adb forward` and the Dart
Development Service, and flutter then reported "Service connection disposed", "Failed to start
Dart Development Service" or "Connecting to the VM Service timed out". The app starts of the
same job that run as instrumented tests (Gradle `connectedDebugAndroidTest` and `patrol test`,
four per job) never failed in the same main runs. So an Android entry of a
`platform-tests.json`, and every script it runs, starts no `flutter test`, `flutter drive` or
`flutter run` on the device: the phone suite runs through `PhoneSuiteTest` (the integration
test runner of the `integration_test` plugin) in a build whose entry point is the suite.

The checks read the real files: every tracked `platform-tests.json`, the scripts its Android
entries run, the client's test class and its scripts.
"""

from __future__ import annotations

import json
import re
import shlex
import subprocess
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]
CLIENT = REPO / "apps" / "client"
# A `flutter` command that runs the app on a device with a connection from the host to its VM
# service.
_ATTACHED = re.compile(r"\bflutter\s+(?:test|drive|run)\b")
PHONE_SUITE_CLASS = "com.example.client.PhoneSuiteTest"
PHONE_SUITE_TEST = (
    CLIENT / "android/app/src/androidTest/kotlin/com/example/client/PhoneSuiteTest.kt"
)


def _tracked(pattern: str) -> list[Path]:
    out = subprocess.run(
        ["git", "ls-files", "-z", "--", pattern], cwd=REPO, check=True, capture_output=True
    ).stdout
    return [REPO / name for name in out.decode().split("\0") if name]


def _code_lines(script: Path) -> list[tuple[int, str]]:
    """The lines of a shell script without comments, joined across trailing backslashes."""
    lines: list[tuple[int, str]] = []
    pending = ""
    start = 0
    for number, line in enumerate(script.read_text(encoding="utf-8").splitlines(), 1):
        if not pending and line.lstrip().startswith("#"):
            continue
        if not pending:
            start = number
        if line.endswith("\\"):
            pending += line[:-1] + " "
            continue
        lines.append((start, pending + line))
        pending = ""
    return lines


def android_commands() -> list[tuple[str, list[str]]]:
    """(member folder, argv) of every Android entry of a tracked `platform-tests.json`."""
    found: list[tuple[str, list[str]]] = []
    for path in _tracked("*platform-tests.json"):
        tests = json.loads(path.read_text(encoding="utf-8"))
        for argv in tests.get("android", []):
            found.append((str(path.parent.relative_to(REPO)), argv))
    return found


def attached_starts() -> list[str]:
    """Every place where an Android entry or a script it runs starts flutter attached."""
    wrong: list[str] = []
    for member, argv in android_commands():
        if _ATTACHED.search(" ".join(argv)):
            wrong.append(f"{member}/platform-tests.json: {' '.join(argv)}")
        if argv[0] == "sh":
            script = REPO / member / argv[1]
            for number, line in _code_lines(script):
                if _ATTACHED.search(line):
                    wrong.append(f"{script.relative_to(REPO)}:{number}: {line.strip()}")
    return wrong


def test_the_scan_finds_the_client_android_scripts() -> None:
    scripts = {argv[1] for member, argv in android_commands() if member == "apps/client"}
    assert "tool/run_android_tests.sh" in scripts


def test_the_scan_sees_an_attached_start(tmp_path: Path) -> None:
    script = tmp_path / "run.sh"
    script.write_text(
        "# flutter test in a comment is fine\nflutter test \\\n  integration_test/a_test.dart\n",
        encoding="utf-8",
    )
    assert [line for _, line in _code_lines(script) if _ATTACHED.search(line)] == [
        "flutter test    integration_test/a_test.dart"
    ]


def test_no_android_device_test_attaches_flutter_to_the_app() -> None:
    wrong = attached_starts()
    assert wrong == [], (
        "Android device tests run as instrumented tests (FIX-android36-dds-start):\n"
        + "\n".join(wrong)
    )


def test_the_phone_suite_runs_as_an_instrumented_test() -> None:
    script = CLIENT / "tool" / "run_android_tests.sh"
    runs = [line for _, line in _code_lines(script) if "connectedDebugAndroidTest" in line]
    phone = [line for line in runs if f"class={PHONE_SUITE_CLASS}" in line]
    assert len(phone) == 1, runs
    argv = shlex.split(phone[0])
    targets = [arg for arg in argv if arg.startswith("-Ptarget=")]
    assert len(targets) == 1
    assert targets[0].endswith("integration_test/phone_suite_test.dart")
    source = PHONE_SUITE_TEST.read_text(encoding="utf-8")
    assert "@RunWith(FlutterTestRunner::class)" in source
    # FlutterTestRunner launches the activity of the first public `@Rule` ActivityTestRule field.
    assert re.search(r"@Rule\s+@JvmField\s+val\s+\w+\s*=\s*ActivityTestRule\(", source)


def test_every_other_instrumented_run_leaves_the_phone_suite_out() -> None:
    # PhoneSuiteTest waits for the results of the Dart tests; in a build of another entry point
    # none come, so a run that does not name its classes or package would never end.
    wrong: list[str] = []
    for script in _tracked("apps/client/*.sh"):
        for number, line in _code_lines(script):
            if "connectedDebugAndroidTest" not in line or f"class={PHONE_SUITE_CLASS}" in line:
                continue
            argv = shlex.split(line, comments=True)
            chosen = any(
                re.match(r"-Pandroid\.testInstrumentationRunnerArguments\.(class|package)=", a)
                for a in argv
            )
            left_out = any(
                a.startswith("-Pandroid.testInstrumentationRunnerArguments.notClass=")
                and PHONE_SUITE_CLASS in a.split("=", 1)[1].split(",")
                for a in argv
            )
            if not (chosen or left_out):
                wrong.append(f"{script.relative_to(REPO)}:{number}: {line.strip()}")
    assert wrong == [], "name the classes or leave PhoneSuiteTest out:\n" + "\n".join(wrong)


def test_the_phone_suite_creates_its_binding_before_its_groups() -> None:
    # The binding reports the results to PhoneSuiteTest in a tearDownAll of the place where it is
    # first created; inside the first group it reported after that group, and the runner passed
    # with one of 14 tests (local run on an API 36 emulator).
    source = (CLIENT / "integration_test" / "phone_suite_test.dart").read_text(encoding="utf-8")
    body = source.split("void main() {", 1)[1]
    binding = body.find("IntegrationTestWidgetsFlutterBinding.ensureInitialized();")
    assert binding != -1
    assert binding < body.find("group(")
