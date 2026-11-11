# SPDX-License-Identifier: Apache-2.0
"""Which packages of a Gradle lock file the advisory and license gates judge.

A Gradle lock file lists every package a build resolves with the configurations that resolve
it. tools/ci/gradle_locks.py reads these configurations: a package counts when it ships (the
runtime classpath of a release build, `coreLibraryDesugaring`), not when only the build, lint or
a test resolves it. The tests use the file format Gradle writes and, for the tracked lock files,
the files themselves (docs/decisions.d/AUD-root-01.md).
"""

from __future__ import annotations

import subprocess
from pathlib import Path

import gradle_locks
import pytest

REPO = Path(__file__).resolve().parents[3]

LOCK = """\
# This is a Gradle generated file for dependency locking.
# Manual edits can break the build and are not advised.
# This file is expected to be part of source control.
com.example:shipped:1.0=debugRuntimeClasspath,releaseCompileClasspath,releaseRuntimeClasspath
com.example:desugar:2.0=coreLibraryDesugaring
com.example:debug-only:3.0=debugRuntimeClasspath,profileRuntimeClasspath
com.example:test-runner:4.0=_internal-unified-test-platform-core
com.example:unit-test:5.0=releaseUnitTestRuntimeClasspath,debugUnitTestRuntimeClasspath
com.example:lint:6.0=releaseLintChecksClasspath
com.example:two:7.0=releaseRuntimeClasspath
com.example:two:7.1=debugAndroidTestRuntimeClasspath
empty=androidApis,releaseAnnotationProcessorClasspath
"""


def _tracked_locks() -> list[str]:
    out = subprocess.run(
        ["git", "ls-files", "--", "*.lockfile"],
        cwd=REPO,
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    return sorted(out.split())


def _write(tmp_path: Path, name: str, text: str) -> Path:
    path = tmp_path / name
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")
    return path


def test_a_lock_file_names_its_packages_with_their_configurations() -> None:
    parsed = gradle_locks.parse(LOCK)
    assert parsed.packages[("com.example:shipped", "1.0")] == {
        "debugRuntimeClasspath",
        "releaseCompileClasspath",
        "releaseRuntimeClasspath",
    }
    assert ("com.example:two", "7.0") in parsed.packages
    assert ("com.example:two", "7.1") in parsed.packages
    assert parsed.empty == {"androidApis", "releaseAnnotationProcessorClasspath"}
    assert len(parsed.packages) == 8


@pytest.mark.parametrize(
    ("configuration", "ships"),
    [
        ("releaseRuntimeClasspath", True),
        ("coreLibraryDesugaring", True),
        # A module without build variants and a build with product flavors.
        ("runtimeClasspath", True),
        ("storeReleaseRuntimeClasspath", True),
        ("debugRuntimeClasspath", False),
        ("profileRuntimeClasspath", False),
        ("releaseCompileClasspath", False),
        ("releaseUnitTestRuntimeClasspath", False),
        ("debugAndroidTestRuntimeClasspath", False),
        ("testRuntimeClasspath", False),
        ("releaseLintChecksClasspath", False),
        ("_internal-unified-test-platform-core", False),
        ("classpath", False),
        ("kotlinCompilerClasspath", False),
    ],
)
def test_only_the_release_runtime_and_the_desugaring_library_ship(
    configuration: str, ships: bool
) -> None:
    assert gradle_locks.ships(configuration) is ships


def test_the_shipped_packages_of_a_lock_file(tmp_path: Path) -> None:
    lock = _write(tmp_path, "android/gradle.lockfile", LOCK)
    assert gradle_locks.shipped(lock) == {
        ("com.example:shipped", "1.0"),
        ("com.example:desugar", "2.0"),
        ("com.example:two", "7.0"),
    }


def test_a_build_script_lock_file_ships_nothing(tmp_path: Path) -> None:
    lock = _write(
        tmp_path,
        "android/buildscript-gradle.lockfile",
        "com.example:plugin:1.0=classpath\nempty=\n",
    )
    assert gradle_locks.shipped(lock) == set()


def test_a_lock_file_without_a_configuration_that_ships_cannot_be_judged(tmp_path: Path) -> None:
    """Another variant layout would make the gates read nothing of the file, so it fails."""
    lock = _write(
        tmp_path,
        "android/gradle.lockfile",
        "com.example:lib:1.0=stagingRuntimeClasspath,debugRuntimeClasspath\nempty=androidApis\n",
    )
    with pytest.raises(gradle_locks.LockError, match="names no configuration that ships"):
        gradle_locks.shipped(lock)


def test_a_release_runtime_without_packages_is_known_from_the_empty_line(tmp_path: Path) -> None:
    lock = _write(
        tmp_path,
        "android/gradle.lockfile",
        "com.example:lint:1.0=releaseLintChecksClasspath\nempty=releaseRuntimeClasspath\n",
    )
    assert gradle_locks.shipped(lock) == set()


def test_a_line_that_is_no_lock_entry_fails(tmp_path: Path) -> None:
    lock = _write(tmp_path, "android/gradle.lockfile", "com.example:lib=releaseRuntimeClasspath\n")
    with pytest.raises(gradle_locks.LockError, match="no lock entry"):
        gradle_locks.shipped(lock)


@pytest.mark.parametrize(
    ("path", "argument"),
    [
        ("apps/a/android/gradle.lockfile", "apps/a/android/gradle.lockfile"),
        ("p/android/buildscript-gradle.lockfile", "p/android/buildscript-gradle.lockfile"),
        # OSV-Scanner picks the parser by the file name; a variant names its format.
        (
            "apps/a/android/gradle-firebase.lockfile",
            "gradle.lockfile:apps/a/android/gradle-firebase.lockfile",
        ),
        ("a/uv.lock", "a/uv.lock"),
        ("services/server/go.mod", "services/server/go.mod"),
    ],
)
def test_the_scanner_argument_of_a_lock_file(path: str, argument: str) -> None:
    assert gradle_locks.scan_arg(path) == argument


def test_gradle_lock_files_are_known_by_their_name() -> None:
    assert gradle_locks.is_gradle_lock("apps/a/android/gradle-firebase.lockfile")
    assert gradle_locks.is_gradle_lock(Path("/x/buildscript-gradle.lockfile"))
    assert not gradle_locks.is_gradle_lock("pubspec.lock")
    assert not gradle_locks.is_gradle_lock("a/package-lock.json")


def _report(lock: Path, *names: tuple[str, str]) -> dict:
    packages = [
        {"package": {"name": name, "version": version, "ecosystem": "Maven"}}
        for name, version in names
    ]
    return {"results": [{"source": {"path": str(lock), "type": "lockfile"}, "packages": packages}]}


def test_a_report_keeps_only_the_shipped_packages_of_a_gradle_lock_file(tmp_path: Path) -> None:
    lock = _write(tmp_path, "android/gradle.lockfile", LOCK)
    other = _write(tmp_path, "a/uv.lock", "")
    report = _report(lock, ("com.example:shipped", "1.0"), ("com.example:test-runner", "4.0"))
    report["results"] += _report(other, ("requests", "1.0"))["results"]
    kept, dropped = gradle_locks.shipped_only(report)
    names = [[p["package"]["name"] for p in r["packages"]] for r in kept["results"]]
    assert names == [["com.example:shipped"], ["requests"]]
    assert [(src, p["package"]["name"]) for src, p in dropped] == [
        (str(lock), "com.example:test-runner")
    ]
    # The report of the scanner is not changed.
    assert len(report["results"][0]["packages"]) == 2


def test_the_repository_has_gradle_lock_files() -> None:
    assert len(_tracked_locks()) >= 2


@pytest.mark.parametrize("lock", _tracked_locks())
def test_every_tracked_gradle_lock_file_can_be_judged(lock: str) -> None:
    """Each tracked lock file parses, and each but the build script ones names what ships."""
    shipped = gradle_locks.shipped(REPO / lock)
    if Path(lock).name == gradle_locks.BUILDSCRIPT_LOCK:
        assert shipped == set()
    else:
        assert shipped, f"{lock} ships no package"
