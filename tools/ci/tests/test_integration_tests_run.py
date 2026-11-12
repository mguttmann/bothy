# SPDX-License-Identifier: Apache-2.0
"""Every integration test of the app runs in a platform job (D-M4-18-19, AUD-CM01/2).

`make apps-platform` runs exactly the commands of `apps/client/platform-tests.json`
(tools/ci/apps_platform.py), so a test file under `apps/client/integration_test/` that no command
reaches runs in no job. A command reaches a test file in one of three ways: it is `flutter test`
of that file on a device, it is `flutter test` of a suite that imports the file and calls its
`main`, or it is a script that names the file or such a suite.

A desktop test runs on macOS and on Windows. A test in a phone folder runs on a phone. ONLY names
the files that run elsewhere, each with its reason. The check reads the real files.
"""

from __future__ import annotations

import json
import re
import subprocess
from collections.abc import Callable, Mapping, Sequence
from pathlib import Path, PurePosixPath

import pytest

REPO = Path(__file__).resolve().parents[3]
CLIENT = REPO / "apps" / "client"
TESTS = "integration_test/"
DESKTOP = ("macos", "windows")
PHONES = ("ios", "android")
# The folders of the phone tests: each of their files runs on at least one phone.
PHONE_FOLDERS = ("phone/", "share/")
# The files that run on other systems than both desktops: the systems and the reason.
ONLY: dict[str, tuple[tuple[str, ...], str]] = {
    "composer_ax_insert_test.dart": (("macos",), "it drives the accessibility API of macOS"),
    "legacy_keychain_test.dart": (("macos",), "it reads the keychain of macOS"),
    "device_keys_windows_test.dart": (("windows",), "it uses the key store of Windows"),
    "phone_suite_test.dart": (PHONES, "it is the suite of the phone tests"),
}

_IMPORT = re.compile(r"^import '([^']+)' as (\w+);", re.MULTILINE)

Manifest = Mapping[str, Sequence[Sequence[str]]]
Reader = Callable[[str], str]


def suite_runs(suite: str, text: str, test: str) -> bool:
    """Whether the suite file `suite` with the source `text` imports `test` and calls its main.

    Both names are relative to `integration_test/`.
    """
    folder = PurePosixPath(suite).parent
    for target, alias in _IMPORT.findall(text):
        if (folder / target).as_posix() == test:
            return re.search(rf"\b{alias}\.main\b", text) is not None
    return False


def script_names(text: str, test: str) -> bool:
    """Whether the shell script `text` names the test file, by its path or by folder and stem."""
    if TESTS + test in text:
        return True
    path = PurePosixPath(test)
    nested = f"{TESTS}{path.parent.as_posix()}/" in text
    return nested and re.search(rf"\b{path.stem}\b", text) is not None


def runs(test: str, system: str, manifest: Manifest, tests: Sequence[str], read: Reader) -> bool:
    """Whether a command of `system` in `manifest` reaches `integration_test/<test>`."""
    suites = [t for t in tests if t != test]
    for argv in manifest.get(system, ()):
        if list(argv[:2]) == ["flutter", "test"]:
            # One test file per command (test_flutter_device_tests_one_file.py).
            for file in (a for a in argv[2:] if a.startswith(TESTS) and a.endswith(".dart")):
                target = file.removeprefix(TESTS)
                if target == test or suite_runs(target, read(file), test):
                    return True
        elif argv[0] == "sh" and len(argv) > 1:
            script = read(argv[1])
            if script_names(script, test):
                return True
            if any(
                script_names(script, s) and suite_runs(s, read(TESTS + s), test) for s in suites
            ):
                return True
    return False


def problems(tests: Sequence[str], manifest: Manifest, read: Reader) -> list[str]:
    """What keeps a test of `tests` (names relative to `integration_test/`) out of every job."""
    found = [
        f"ONLY names {name}, which is no tracked test file" for name in ONLY if name not in tests
    ]
    for test in tests:
        if test in ONLY:
            wanted = [(s,) for s in ONLY[test][0]]
        elif test.startswith(PHONE_FOLDERS):
            wanted = [PHONES]
        else:
            wanted = [(s,) for s in DESKTOP]
        for systems in wanted:
            if not any(runs(test, s, manifest, tests, read) for s in systems):
                found.append(
                    f"{TESTS}{test}: no command of {' or '.join(systems)} in platform-tests.json "
                    "runs it (a line of its own, a suite that calls its main, or a script)"
                )
    return found


def _tracked_tests() -> list[str]:
    out = subprocess.run(
        ["git", "ls-files", "-z", "--", f"{TESTS}*_test.dart"],
        cwd=CLIENT,
        check=True,
        capture_output=True,
    ).stdout
    return sorted(name.removeprefix(TESTS) for name in out.decode().split("\0") if name)


def _read(relative: str) -> str:
    return (CLIENT / relative).read_text(encoding="utf-8")


def test_every_integration_test_of_the_app_runs_in_a_platform_job() -> None:
    tests = _tracked_tests()
    assert "forum_test.dart" in tests, "the scan finds the tests"
    assert "phone/offline_test.dart" in tests, "the scan finds the tests of the folders"
    manifest = json.loads(_read("platform-tests.json"))
    found = problems(tests, manifest, _read)
    assert found == [], "integration tests that no job runs:\n" + "\n".join(found)


SUITE = (
    "import 'a_test.dart' as a;\nimport 'phone/b_test.dart' as b;\nvoid main() {\n  a.main();\n}\n"
)


def _files(extra: dict[str, str] | None = None) -> Reader:
    files = {
        "integration_test/suite_test.dart": SUITE,
        "integration_test/phone_suite_test.dart": "import 'phone/b_test.dart' as b;\nb.main();\n",
        "tool/run_phone.sh": "flutter test integration_test/phone_suite_test.dart -d x\n",
        "tool/run_ax.sh": "flutter test integration_test/composer_ax_insert_test.dart -d macos\n",
        "tool/run_keychain.sh": "flutter test integration_test/legacy_keychain_test.dart -d x\n",
        "tool/run_share.sh": 'for t in c_test; do patrol "integration_test/share/$t.dart"; done\n',
        **(extra or {}),
    }
    return lambda name: files.get(name, "")


def _line(test: str, system: str) -> list[str]:
    return ["flutter", "test", TESTS + test, "-d", system]


def _manifest() -> dict[str, list[list[str]]]:
    return {
        "macos": [
            _line("a_test.dart", "macos"),
            ["sh", "tool/run_ax.sh"],
            ["sh", "tool/run_keychain.sh"],
        ],
        "windows": [
            _line("suite_test.dart", "windows"),
            _line("device_keys_windows_test.dart", "windows"),
        ],
        "ios": [["sh", "tool/run_phone.sh"], ["sh", "tool/run_share.sh", "ios"]],
        "android": [["sh", "tool/run_phone.sh"]],
    }


TREE = ["a_test.dart", "phone/b_test.dart", "share/c_test.dart", "suite_test.dart", *ONLY]


def test_a_line_a_suite_and_a_script_each_run_a_test() -> None:
    manifest = _manifest()
    manifest["macos"].append(_line("suite_test.dart", "macos"))
    assert problems(TREE, manifest, _files()) == []


def test_a_test_without_a_command_on_one_desktop_is_reported() -> None:
    found = problems(TREE, _manifest(), _files())
    assert found == [
        "integration_test/suite_test.dart: no command of macos in platform-tests.json runs it "
        "(a line of its own, a suite that calls its main, or a script)"
    ]


@pytest.mark.parametrize(
    ("suite", "missing"),
    [
        # Imported, but its main is never called.
        (SUITE.replace("  a.main();\n", ""), "integration_test/a_test.dart: no command of windows"),
        # A phone test the phone suite does not import.
        (None, "integration_test/phone/d_test.dart: no command of ios or android"),
    ],
)
def test_a_test_no_suite_calls_is_reported(suite: str | None, missing: str) -> None:
    manifest = _manifest()
    manifest["macos"].append(_line("suite_test.dart", "macos"))
    tree = TREE if suite is not None else [*TREE, "phone/d_test.dart"]
    extra = {"integration_test/suite_test.dart": suite} if suite is not None else None
    found = problems(tree, manifest, _files(extra))
    assert len(found) == 1
    assert found[0].startswith(missing)


def test_an_exception_for_a_file_that_is_gone_is_reported() -> None:
    manifest = _manifest()
    manifest["macos"].append(_line("suite_test.dart", "macos"))
    tree = [t for t in TREE if t != "legacy_keychain_test.dart"]
    assert problems(tree, manifest, _files()) == [
        "ONLY names legacy_keychain_test.dart, which is no tracked test file"
    ]
