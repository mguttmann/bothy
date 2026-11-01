# SPDX-License-Identifier: Apache-2.0
"""The heavy Linux jobs of ci.yml and build-verify.yml free disk space first.

The GitHub-hosted ubuntu runners ship toolchains that no job of this repository uses (.NET,
GHC, Swift, Boost, CodeQL bundles, Docker images). With them, the runner's disk ran full in
`gates` (lint), `test (tools)` and `console-e2e` (the image build) on main: "No space left on
device", no test result. `tools/ci/free_disk.sh` removes them, and the Android SDK as well in a
job that installs no JDK, Android SDK, Flutter or emulator. The checks read the real workflow
and run the real script against a fake root (FIX-main-red-2026-10-31).
"""

from __future__ import annotations

import os
import subprocess
from pathlib import Path
from typing import Any

import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
WORKFLOWS = REPO / ".github" / "workflows"
SCRIPT = REPO / "tools" / "ci" / "free_disk.sh"
STEP_NAME = "Free disk space"
# Per workflow, the Linux jobs that only plan, scan or report: short, no toolchains, no builds.
# build-verify.yml mirrors the jobs of ci.yml (test_build_verify_workflow.py).
LIGHT_JOBS = {
    "ci.yml": {"plan", "denylist", "denylist-fork", "ci-ok", "test-server-coverage", "sonar"},
    "build-verify.yml": {"plan", "denylist", "test-server-coverage", "build-verify-ok"},
}
# Actions after which a job needs the runner's Android SDK.
ANDROID_USERS = ("setup-android", "setup-java", "flutter-action", "android-emulator-runner")
# The folders the script removes, below the root, and the one --keep-android keeps.
REMOVED = (
    "usr/share/dotnet",
    "opt/ghc",
    "usr/local/.ghcup",
    "usr/share/swift",
    "usr/local/share/boost",
    "opt/hostedtoolcache/CodeQL",
)
ANDROID = "usr/local/lib/android"


def _jobs(workflow: str) -> dict[str, Any]:
    return yaml.safe_load((WORKFLOWS / workflow).read_text(encoding="utf-8"))["jobs"]


def _linux(job: dict[str, Any]) -> bool:
    return str(job.get("runs-on", "")).startswith("ubuntu-")


def _action(step: dict[str, Any]) -> str:
    return str(step.get("uses", "")).split("@", 1)[0].rsplit("/", 1)[-1]


@pytest.mark.parametrize("workflow", sorted(LIGHT_JOBS))
def test_every_linux_job_is_light_or_frees_disk_first(workflow: str) -> None:
    wrong = []
    for name, job in _jobs(workflow).items():
        if not _linux(job) or name in LIGHT_JOBS[workflow]:
            continue
        steps = job.get("steps", [])
        if len(steps) < 2 or _action(steps[0]) != "checkout" or steps[1].get("name") != STEP_NAME:
            wrong.append(f"{name}: the step after checkout is not {STEP_NAME!r}")
            continue
        keeps = any(_action(step) in ANDROID_USERS for step in steps)
        want = "sh tools/ci/free_disk.sh" + (" --keep-android" if keeps else "")
        if steps[1].get("run", "").strip() != want:
            wrong.append(f"{name}: run {steps[1].get('run')!r}, want {want!r}")
    assert wrong == [], "\n".join(wrong)


@pytest.mark.parametrize("workflow", sorted(LIGHT_JOBS))
def test_the_light_jobs_exist(workflow: str) -> None:
    assert set(_jobs(workflow)) >= LIGHT_JOBS[workflow]


def _run(tmp_path: Path, *args: str) -> tuple[Path, subprocess.CompletedProcess[str]]:
    root = tmp_path / "root"
    for folder in (*REMOVED, ANDROID, "usr/bin", "home/runner/work"):
        (root / folder).mkdir(parents=True)
        (root / folder / "file").write_text("x", encoding="utf-8")
    log = tmp_path / "docker.log"
    docker = tmp_path / "docker"
    docker.write_text(f'#!/bin/sh\necho "$*" >> {log}\n', encoding="utf-8")
    docker.chmod(0o755)
    env = {
        **os.environ,
        "FREE_DISK_ROOT": str(root),
        "FREE_DISK_SUDO": "",
        "FREE_DISK_DOCKER": str(docker),
    }
    done = subprocess.run(
        ["sh", str(SCRIPT), *args], env=env, capture_output=True, text=True, check=False
    )
    return root, done


def test_the_script_removes_the_unused_toolchains_and_android(tmp_path: Path) -> None:
    root, done = _run(tmp_path)
    assert done.returncode == 0, done.stderr
    for folder in (*REMOVED, ANDROID):
        assert not (root / folder).exists(), folder
    assert (root / "usr/bin/file").exists()
    assert (root / "home/runner/work/file").exists()
    assert (tmp_path / "docker.log").read_text(encoding="utf-8").split("\n")[0] == (
        "image prune --all --force"
    )
    assert "free disk space before" in done.stdout
    assert "free disk space after" in done.stdout


def test_keep_android_keeps_the_sdk(tmp_path: Path) -> None:
    root, done = _run(tmp_path, "--keep-android")
    assert done.returncode == 0, done.stderr
    assert (root / ANDROID / "file").exists()
    for folder in REMOVED:
        assert not (root / folder).exists(), folder


def test_missing_folders_and_no_docker_are_fine(tmp_path: Path) -> None:
    root = tmp_path / "empty"
    root.mkdir()
    env = {
        **os.environ,
        "FREE_DISK_ROOT": str(root),
        "FREE_DISK_SUDO": "",
        "FREE_DISK_DOCKER": str(tmp_path / "no-docker"),
    }
    done = subprocess.run(["sh", str(SCRIPT)], env=env, capture_output=True, text=True, check=False)
    assert done.returncode == 0, done.stderr


def test_an_unknown_option_fails(tmp_path: Path) -> None:
    _, done = _run(tmp_path, "--keep-everything")
    assert done.returncode == 2
    assert "usage" in done.stderr
