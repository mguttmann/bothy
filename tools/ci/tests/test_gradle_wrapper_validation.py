# SPDX-License-Identifier: Apache-2.0
"""CI validates every tracked Gradle wrapper JAR (SEC-ALERTS-2026-10-31).

The repository tracks `gradle-wrapper.jar` files, because `make test` and the release builds call
`./gradlew` directly. A wrapper JAR is a binary that runs on every build, so the job
`gates-security` of .github/workflows/ci.yml checks each one against the checksums Gradle
publishes, with `gradle/actions/wrapper-validation`. The job runs in every pull request and on
every push to main; OpenSSF Scorecard (Binary-Artifacts) accepts a wrapper JAR only when a
workflow with this action has a successful run on the head of main.

The step is a gate of the merge, not of the run (AUD-CG1/3): the jobs of the same run that call
`./gradlew` do not wait for it, and a pull request runs its own build scripts in them anyway.
The job `security` of build-verify.yml carries the same step, and the one job that runs a
wrapper with a signing key, in release.yml, waits for the gate stage with the step.
"""

from __future__ import annotations

import subprocess
from pathlib import Path
from typing import Any

import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
WORKFLOWS = REPO / ".github" / "workflows"
WORKFLOW = WORKFLOWS / "ci.yml"
ACTION = "gradle/actions/wrapper-validation@"
# The job with the supply chain gates of each workflow that runs every gate.
SECURITY_JOBS = {"ci.yml": "gates-security", "build-verify.yml": "security"}


def _tracked_wrappers() -> list[str]:
    out = subprocess.run(
        ["git", "-C", str(REPO), "ls-files", "--", "*gradle-wrapper.jar"],
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    return [line for line in out.splitlines() if line]


def _workflow(name: str = "ci.yml") -> dict[str, Any]:
    data = yaml.safe_load((WORKFLOWS / name).read_text(encoding="utf-8"))
    assert isinstance(data, dict)
    return data


def _uses(step: dict[str, Any], action: str) -> bool:
    return str(step.get("uses", "")).startswith(action)


def test_ci_runs_on_pull_requests_and_pushes_to_main() -> None:
    # PyYAML reads the key `on` as True.
    on = _workflow()[True]
    assert on["push"]["branches"] == ["main"]
    assert on["pull_request"]["branches"] == ["main"]


@pytest.mark.parametrize("name", sorted(SECURITY_JOBS))
def test_the_security_job_validates_every_tracked_wrapper(name: str) -> None:
    wrappers = _tracked_wrappers()
    assert wrappers, "no gradle-wrapper.jar tracked: drop the validation step and this test"
    job = _workflow(name)["jobs"][SECURITY_JOBS[name]]
    assert "if" not in job, "the job must run on every event, or a wrapper goes unchecked"
    steps = job["steps"]
    checkout = next(i for i, s in enumerate(steps) if _uses(s, "actions/checkout@"))
    found = [i for i, s in enumerate(steps) if _uses(s, ACTION)]
    assert len(found) == 1, f"{SECURITY_JOBS[name]} has {len(found)} steps with {ACTION}"
    step = steps[found[0]]
    assert checkout < found[0]
    assert "if" not in step
    # The action fails when it finds fewer JARs than this, so a wrapper cannot drop out unseen.
    assert int(step["with"]["min-wrapper-count"]) == len(wrappers)
    assert step["with"].get("allow-checksums", "") == ""
    assert step["with"].get("allow-snapshots", "false") in ("false", False)


def test_both_workflows_pin_the_same_validation_step() -> None:
    steps = []
    for name, job in SECURITY_JOBS.items():
        steps += [s for s in _workflow(name)["jobs"][job]["steps"] if _uses(s, ACTION)]
    assert len(steps) == len(SECURITY_JOBS)
    assert all(step == steps[0] for step in steps)


def test_a_wrapper_runs_with_a_signing_key_only_after_the_validated_gate_stage() -> None:
    """release.yml: the gate stage is ci.yml, and every job with a JDK waits for it."""
    jobs = _workflow("release.yml")["jobs"]
    assert str(jobs["gates"]["uses"]).startswith("./.github/workflows/ci.yml")
    builders = {
        name: job
        for name, job in jobs.items()
        if any(_uses(step, "actions/setup-java@") for step in job.get("steps", []))
    }
    assert builders, "no release job installs a JDK: drop this test"
    for name, job in builders.items():
        assert "gates" in job["needs"], name
