# SPDX-License-Identifier: Apache-2.0
"""The Android phone tests of the client run without a system keyboard (FIX-main-red-2026-10-31).

`integration_test/phone/dictation_test.dart` drives the composer through the text input probe,
an `InputConnection` of its own, the way a dictation keyboard does. The emulator's Gboard was
bound to the same field at the same time and rewrote the composing state after each insert
(logcat: "Using roll back input context operation: [AbortComposing]"), so the test failed at
random on API 26 ("the composition ended", a field that did not keep "See ", a field that was
not empty after a send).
Two input methods editing one field at once never happens to a person: the dictation keyboard
is the input method. `apps/client/tool/run_android_tests.sh` disables every enabled input method
of the device before the phone tests and enables them again afterwards with the default one
selected (API 26 has no `ime reset`), also when the tests fail.
Disabling an input method alone does not hold on API 26: Android enables its default input
methods again on its own (a package event does so), and Gboard was bound in the CI run of #494
after the rebase onto #499. The script therefore also disables the packages of the input methods
(`pm disable-user`), which no package event undoes, and enables them again afterwards; a package
the shell may not disable, or one that was disabled before, is left as it is.

The test runs a copy of the real script with fake `adb`, `flutter` and `gradlew` that log their
calls.
"""

from __future__ import annotations

import os
import shutil
import subprocess
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]
SCRIPT = REPO / "apps" / "client" / "tool" / "run_android_tests.sh"
IMES = (
    "com.google.android.inputmethod.latin/com.android.inputmethod.latin.LatinIME",
    "com.google.android.googlequicksearchbox/com.google.android.voicesearch.ime.VoiceInputMethodService",
)
# Every input method of the device (`ime list -a`): the enabled ones, one the shell may not
# disable (as on the API 26 image) and one whose package was disabled before the run.
ALL_PACKAGES = (
    "com.google.android.inputmethod.latin",
    "com.google.android.googlequicksearchbox",
    "com.example.android.softkeyboard",
    "com.google.android.inputmethod.pinyin",
    "jp.co.omronsoft.openwnn",
)
DISABLED_BY_US = ALL_PACKAGES[:3]


def _fake(path: Path, body: str) -> None:
    path.write_text("#!/bin/sh\n" + body, encoding="utf-8")
    path.chmod(0o755)


def _run(tmp_path: Path, flutter_code: int) -> tuple[list[str], subprocess.CompletedProcess[str]]:
    client = tmp_path / "apps" / "client"
    (client / "tool").mkdir(parents=True)
    shutil.copy(SCRIPT, client / "tool" / "run_android_tests.sh")
    (client / "android" / "app").mkdir(parents=True)
    # A build configuration with its own Firebase file: the script ends after the first Gradle run.
    (client / "android" / "app" / "google-services.json").write_text("{}", encoding="utf-8")
    log = tmp_path / "calls.log"
    _fake(client / "android" / "gradlew", f'echo "gradlew $*" >> {log}\n')
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    listing = "\\n".join(IMES)
    listing_all = "\\n".join(
        f"{package}/.Ime:\\n  mId={package}/.Ime mSettingsActivityName=null"
        for package in ALL_PACKAGES
    )
    _fake(
        bin_dir / "adb",
        f'echo "adb $*" >> {log}\n'
        'case "$*" in\n'
        '  devices) printf "List of devices attached\\nemulator-5554\\tdevice\\n" ;;\n'
        f'  *"ime list -s"*) printf "{listing}\\n" ;;\n'
        f'  *"settings get secure default_input_method"*) echo "{IMES[0]}" ;;\n'
        f'  *"ime list -a"*) printf "{listing_all}\\n" ;;\n'
        '  *"pm list packages -d"*) printf "package:jp.co.omronsoft.openwnn\\n" ;;\n'
        '  *"pm disable-user --user 0 com.google.android.inputmethod.pinyin"*)\n'
        '    echo "Error: java.lang.SecurityException: Shell cannot change component state" ;;\n'
        '  *"pm disable-user --user 0 "*) echo "Package ${8} new state: disabled-user" ;;\n'
        "esac\n",
    )
    _fake(bin_dir / "flutter", f'echo "flutter $*" >> {log}\nexit {flutter_code}\n')
    env = {**os.environ, "PATH": f"{bin_dir}{os.pathsep}{os.environ['PATH']}"}
    done = subprocess.run(
        ["sh", str(client / "tool" / "run_android_tests.sh")],
        env=env,
        capture_output=True,
        text=True,
        check=False,
    )
    calls = log.read_text(encoding="utf-8").splitlines() if log.exists() else []
    return calls, done


def _index(calls: list[str], prefix: str) -> int:
    return next(i for i, call in enumerate(calls) if call.startswith(prefix))


def test_the_phone_tests_run_with_every_input_method_disabled(tmp_path: Path) -> None:
    calls, done = _run(tmp_path, 0)
    assert done.returncode == 0, done.stderr
    phone = _index(calls, "flutter test integration_test/phone_suite_test.dart")
    for ime in IMES:
        assert _index(calls, f"adb -s emulator-5554 shell ime disable {ime}") < phone
    gradle = _index(calls, "gradlew ")
    for ime in IMES:
        assert phone < _index(calls, f"adb -s emulator-5554 shell ime enable {ime}") < gradle
    assert phone < _index(calls, f"adb -s emulator-5554 shell ime set {IMES[0]}") < gradle


def test_a_failed_phone_run_enables_the_input_methods_again(tmp_path: Path) -> None:
    calls, done = _run(tmp_path, 1)
    assert done.returncode != 0
    phone = _index(calls, "flutter test integration_test/phone_suite_test.dart")
    for ime in IMES:
        assert _index(calls, f"adb -s emulator-5554 shell ime enable {ime}") > phone
    assert _index(calls, f"adb -s emulator-5554 shell ime set {IMES[0]}") > phone
    assert not any(call.startswith("gradlew ") for call in calls)


def test_the_input_method_packages_are_disabled_for_the_phone_tests(tmp_path: Path) -> None:
    calls, done = _run(tmp_path, 0)
    assert done.returncode == 0, done.stderr
    phone = _index(calls, "flutter test integration_test/phone_suite_test.dart")
    gradle = _index(calls, "gradlew ")
    for package in ALL_PACKAGES:
        if package == "jp.co.omronsoft.openwnn":
            # Disabled before the run: neither disabled nor enabled by the script.
            assert not any(package in call and " pm " in call for call in calls)
            continue
        assert (
            _index(calls, f"adb -s emulator-5554 shell pm disable-user --user 0 {package}") < phone
        )
    # `ime disable` runs while the packages are still enabled, so it never meets an unknown one.
    last_ime = max(_index(calls, f"adb -s emulator-5554 shell ime disable {ime}") for ime in IMES)
    assert last_ime < _index(calls, "adb -s emulator-5554 shell pm disable-user")
    for package in DISABLED_BY_US:
        assert (
            phone
            < _index(calls, f"adb -s emulator-5554 shell pm enable --user 0 {package}")
            < gradle
        )
    # The shell may not disable it, so it is not enabled afterwards either.
    assert not any(
        call.endswith("pm enable --user 0 com.google.android.inputmethod.pinyin") for call in calls
    )


def test_a_failed_phone_run_enables_the_input_method_packages_again(tmp_path: Path) -> None:
    calls, done = _run(tmp_path, 1)
    assert done.returncode != 0
    phone = _index(calls, "flutter test integration_test/phone_suite_test.dart")
    for package in DISABLED_BY_US:
        enable = _index(calls, f"adb -s emulator-5554 shell pm enable --user 0 {package}")
        assert enable > phone
        # The packages are enabled before their input methods are, which needs them installed.
        assert enable < _index(calls, f"adb -s emulator-5554 shell ime enable {IMES[0]}")
