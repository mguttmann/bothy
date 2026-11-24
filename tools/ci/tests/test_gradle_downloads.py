# SPDX-License-Identifier: Apache-2.0
"""Gradle's downloads in CI: a cache per job and retries with back-off (AUD-PKG-P10-repo-ops-host).

Maven Central refused downloads with HTTP 429 in `test (apps)` and `mobile trial (Android)`.
Every CI job that runs Gradle restores one cache of the downloaded modules and the wrapper
distribution, keyed on the job and on the Gradle build, lock and wrapper files and the Dart lock
(the Flutter plugins add Android dependencies). The key names the job, so each job saves what it
resolves itself: a key shared by all of them would be saved by the first job to finish, which
resolves only part of the dependencies. A release job restores no cache (cache poisoning).

Every Gradle build of the repository raises Gradle's own retries of a repository request in its
`gradle.properties`. The property names are internal to Gradle; they were read from the bytecode
of Gradle 9.3.1 (`ErrorHandlingModuleComponentRepository`, which retries on 408, 429 and 5xx), so
the wrapper version is held here and an upgrade must check them again (docs/quality/gates.md,
"Gradle downloads").

A cold cache still downloads everything, and a job that fails saves no cache, so every cached
Gradle job also installs the init script `tools/ci/gradle/maven-central-mirror.init.gradle`
before its first Gradle run: every build of the build tree (the Flutter Gradle plugin is an
included build) gets Maven Central from Google's public mirror of it. The release build does not.
"""

from __future__ import annotations

import subprocess
from pathlib import Path
from typing import Any

import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
WORKFLOWS = REPO / ".github" / "workflows"

# The jobs that run Gradle: `make test` of the component apps (the JVM tests of
# packages/*/android), the Android trial and the emulator jobs (they build the app).
GRADLE_JOBS = {
    "ci.yml": ("test-apps", "mobile-trial-android", "apps-android-26", "apps-android-36"),
    "build-verify.yml": ("test-apps", "mobile-trial-android", "apps-android-26", "apps-android-36"),
    "e2e-mobile.yml": ("android",),
}
# Jobs with a JDK that run no Gradle: the shared toolchain block installs it everywhere.
NO_GRADLE_JOBS = {
    "ci.yml": (
        "gates",
        "gates-docs",
        "gates-security",
        "test-tools",
        "test-server",
        "test-server-contract",
        "test-sdk-plugins",
    ),
    "build-verify.yml": (
        "static",
        "test-tools",
        "test-server",
        "test-server-contract",
        "test-sdk-plugins",
        "docs",
        "security",
    ),
}
# Jobs that run Gradle without a cache: a release job restores nothing.
UNCACHED_GRADLE_JOBS = {"release.yml": ("build-android",)}

GRADLE_PATHS = ["~/.gradle/caches/modules-2", "~/.gradle/wrapper/dists"]
GRADLE_KEY = (
    "gradle-${{ runner.os }}-${{ runner.arch }}-${{ github.job }}-${{ hashFiles("
    "'apps/*/android/**/*.gradle.kts', 'apps/*/android/**/*.lockfile', "
    "'apps/*/android/gradle/wrapper/gradle-wrapper.properties', "
    "'packages/*/android/**/*.gradle.kts', 'packages/*/android/**/*.lockfile', "
    "'packages/*/android/gradle/wrapper/gradle-wrapper.properties', 'pubspec.lock') }}"
)
RETRY_PROPERTIES = {
    "systemProp.org.gradle.internal.repository.max.tentatives": "6",
    "systemProp.org.gradle.internal.repository.initial.backoff": "2000",
}
VERIFIED_GRADLE = "gradle-9.3.1-bin.zip"
MIRROR_SCRIPT = "tools/ci/gradle/maven-central-mirror.init.gradle"
MIRROR_URL = "https://maven-central.storage-download.googleapis.com/maven2/"


def _jobs(workflow: str) -> dict[str, Any]:
    data = yaml.safe_load((WORKFLOWS / workflow).read_text(encoding="utf-8"))
    jobs = data["jobs"]
    assert isinstance(jobs, dict)
    return jobs


def _uses(step: dict[str, Any], action: str) -> bool:
    return str(step.get("uses", "")).startswith(action + "@")


def _gradle_caches(steps: list[dict[str, Any]]) -> list[int]:
    return [
        i
        for i, step in enumerate(steps)
        if _uses(step, "actions/cache") and ".gradle" in str(step.get("with", {}).get("path", ""))
    ]


def _tracked(*patterns: str) -> list[str]:
    out = subprocess.run(
        ["git", "-C", str(REPO), "ls-files", "--", *patterns],
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    return [line for line in out.splitlines() if line]


def test_every_job_with_a_jdk_is_classified() -> None:
    """A new job with a JDK must say whether it runs Gradle, so it cannot miss the cache."""
    known = {
        (wf, job)
        for table in (GRADLE_JOBS, NO_GRADLE_JOBS, UNCACHED_GRADLE_JOBS)
        for wf, jobs in table.items()
        for job in jobs
    }
    found = {
        (path.name, job)
        for path in sorted(WORKFLOWS.glob("*.yml"))
        for job, spec in _jobs(path.name).items()
        if any(_uses(s, "actions/setup-java") for s in spec.get("steps", []))
    }
    assert found == known


@pytest.mark.parametrize(
    ("workflow", "job"), [(wf, job) for wf, jobs in GRADLE_JOBS.items() for job in jobs]
)
def test_a_gradle_job_restores_its_own_gradle_cache(workflow: str, job: str) -> None:
    steps = _jobs(workflow)[job]["steps"]
    caches = _gradle_caches(steps)
    assert len(caches) == 1, f"{workflow} {job}: one actions/cache step for Gradle"
    cache = steps[caches[0]]
    assert cache["with"]["path"].split() == GRADLE_PATHS
    assert cache["with"]["key"] == GRADLE_KEY
    [java] = [i for i, s in enumerate(steps) if _uses(s, "actions/setup-java")]
    assert cache.get("if") == steps[java].get("if")
    gradle = next(
        i
        for i, s in enumerate(steps)
        if _uses(s, "reactivecircus/android-emulator-runner")
        or ("make " in str(s.get("run", "")) and "bootstrap" not in str(s["run"]))
    )
    assert java < caches[0] < gradle


@pytest.mark.parametrize(
    ("workflow", "job"),
    [
        (wf, job)
        for table in (NO_GRADLE_JOBS, UNCACHED_GRADLE_JOBS)
        for wf, jobs in table.items()
        for job in jobs
    ],
)
def test_other_jobs_restore_no_gradle_cache(workflow: str, job: str) -> None:
    assert _gradle_caches(_jobs(workflow)[job]["steps"]) == []


def test_the_key_covers_every_gradle_build_file() -> None:
    files = _tracked("*.gradle.kts", "*.lockfile", "*gradle-wrapper.properties")
    assert files
    for name in files:
        assert name.startswith(("apps/", "packages/")), name
        assert "/android/" in name, name


@pytest.mark.parametrize("root", sorted({Path(p).parent for p in _tracked("*settings.gradle.kts")}))
def test_every_gradle_build_retries_repository_requests(root: Path) -> None:
    lines = (REPO / root / "gradle.properties").read_text(encoding="utf-8").splitlines()
    props = dict(line.split("=", 1) for line in lines if "=" in line and not line.startswith("#"))
    for key, value in RETRY_PROPERTIES.items():
        assert props.get(key) == value, f"{root}: {key}"
    wrapper = (REPO / root / "gradle" / "wrapper" / "gradle-wrapper.properties").read_text(
        encoding="utf-8"
    )
    # A Gradle upgrade must check the internal property names again (module docstring).
    assert VERIFIED_GRADLE in wrapper, f"{root}: Gradle changed, check the retry properties"


def _mirror_steps(steps: list[dict[str, Any]]) -> list[int]:
    return [i for i, step in enumerate(steps) if MIRROR_SCRIPT in str(step.get("run", ""))]


@pytest.mark.parametrize(
    ("workflow", "job"), [(wf, job) for wf, jobs in GRADLE_JOBS.items() for job in jobs]
)
def test_a_gradle_job_asks_the_mirror_before_its_first_gradle_run(workflow: str, job: str) -> None:
    steps = _jobs(workflow)[job]["steps"]
    [mirror] = _mirror_steps(steps)
    assert "$HOME/.gradle/init.d/" in steps[mirror]["run"]
    [cache] = _gradle_caches(steps)
    assert steps[mirror].get("if") == steps[cache].get("if")
    gradle = next(
        i
        for i, s in enumerate(steps)
        if _uses(s, "reactivecircus/android-emulator-runner")
        or ("make " in str(s.get("run", "")) and "bootstrap" not in str(s["run"]))
    )
    assert cache < mirror < gradle


@pytest.mark.parametrize(
    ("workflow", "job"),
    [
        (wf, job)
        for table in (NO_GRADLE_JOBS, UNCACHED_GRADLE_JOBS)
        for wf, jobs in table.items()
        for job in jobs
    ],
)
def test_other_jobs_install_no_mirror(workflow: str, job: str) -> None:
    assert _mirror_steps(_jobs(workflow)[job]["steps"]) == []


def test_every_build_of_the_tree_gets_maven_central_from_the_mirror() -> None:
    """The init script points every Maven Central repository of every build at the mirror.

    Text checks only: CI has no Gradle outside the jobs that build. The behaviour was checked
    with Gradle 9.3.1 and Maven Central unreachable (ruling AUD-PKG-P10/B5).
    """
    script = (REPO / MIRROR_SCRIPT).read_text(encoding="utf-8")
    assert f"'{MIRROR_URL}'" in script
    for central in ("https://repo.maven.apache.org/maven2/", "https://repo1.maven.org/maven2/"):
        assert central in script, central
    # Maven Central in the settings, in every buildscript block and project (also those added
    # later, configureEach), the mirror in front of the plugin repositories before the settings
    # script runs, and the Plugin Portal kept for a build that declares no plugin repository.
    for hook in (
        "beforeSettings",
        "beforeProject",
        "configureEach",
        "buildscript.repositories",
        "dependencyResolutionManagement.repositories",
        "addFirst",
        "gradlePluginPortal()",
    ):
        assert hook in script, hook
