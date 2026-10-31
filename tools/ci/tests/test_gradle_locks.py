# SPDX-License-Identifier: Apache-2.0
"""Every Gradle build of the repository locks its resolved dependencies (SonarQube rule S8569).

A Gradle build is a directory with a tracked `settings.gradle.kts`. Its root keeps the lock file
`gradle.lockfile` next to the settings, and one of its build or settings scripts turns dependency
locking on for all configurations, so a build resolves exactly the locked versions. The app
build locks in `app/build.gradle.kts` (docs/decisions.d/M5-03.md D-M5-03-8); the standalone build
of the plugin `device_key` locks in its `settings.gradle.kts`, which an app never reads
(docs/decisions.d/SONAR-MAIN-GATE.md).
"""

from __future__ import annotations

import subprocess
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[3]


def _gradle_roots() -> list[Path]:
    out = subprocess.run(
        ["git", "ls-files", "--", "*settings.gradle.kts", "*settings.gradle"],
        cwd=REPO,
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    return sorted((REPO / line).parent for line in out.splitlines() if line)


def test_the_repository_has_gradle_builds() -> None:
    roots = {root.relative_to(REPO).as_posix() for root in _gradle_roots()}
    assert {"apps/client/android", "packages/device_key/android"} <= roots


@pytest.mark.parametrize("root", _gradle_roots(), ids=lambda p: p.relative_to(REPO).as_posix())
def test_gradle_build_keeps_a_lock_file(root: Path) -> None:
    lock = root / "gradle.lockfile"
    assert lock.is_file(), f"{lock.relative_to(REPO)} is missing (SonarQube rule S8569)"
    lines = lock.read_text(encoding="utf-8").splitlines()
    entries = [line for line in lines if line and not line.startswith("#")]
    assert entries, f"{lock.relative_to(REPO)} locks no dependency"


@pytest.mark.parametrize("root", _gradle_roots(), ids=lambda p: p.relative_to(REPO).as_posix())
def test_gradle_build_turns_locking_on(root: Path) -> None:
    scripts = [
        p for p in root.rglob("*.gradle.kts") if "build" not in p.relative_to(root).parts[:-1]
    ]
    assert any("lockAllConfigurations()" in p.read_text(encoding="utf-8") for p in scripts), (
        f"no script under {root.relative_to(REPO)} calls lockAllConfigurations()"
    )
