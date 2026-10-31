# SPDX-License-Identifier: Apache-2.0
"""CI validates every tracked Gradle wrapper JAR (SEC-ALERTS-2026-10-31).

The repository tracks `gradle-wrapper.jar` files, because `make test` and the release builds call
`./gradlew` directly. A wrapper JAR is a binary that runs on every build, so the job
`gates-security` of .github/workflows/ci.yml checks each one against the checksums Gradle
publishes, with `gradle/actions/wrapper-validation`. The job runs in every pull request and on
every push to main; OpenSSF Scorecard (Binary-Artifacts) accepts a wrapper JAR only when a
workflow with this action has a successful run on the head of main.
"""

from __future__ import annotations

import subprocess
from pathlib import Path
from typing import Any

import yaml

REPO = Path(__file__).resolve().parents[3]
WORKFLOW = REPO / ".github" / "workflows" / "ci.yml"
ACTION = "gradle/actions/wrapper-validation@"


def _tracked_wrappers() -> list[str]:
    out = subprocess.run(
        ["git", "-C", str(REPO), "ls-files", "--", "*gradle-wrapper.jar"],
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    return [line for line in out.splitlines() if line]


def _workflow() -> dict[str, Any]:
    data = yaml.safe_load(WORKFLOW.read_text(encoding="utf-8"))
    assert isinstance(data, dict)
    return data


def test_ci_runs_on_pull_requests_and_pushes_to_main() -> None:
    # PyYAML reads the key `on` as True.
    on = _workflow()[True]
    assert on["push"]["branches"] == ["main"]
    assert on["pull_request"]["branches"] == ["main"]


def test_gates_security_validates_every_tracked_wrapper() -> None:
    wrappers = _tracked_wrappers()
    assert wrappers, "no gradle-wrapper.jar tracked: drop the validation step and this test"
    job = _workflow()["jobs"]["gates-security"]
    assert "if" not in job, "the job must run on every event, or a wrapper goes unchecked"
    steps = job["steps"]
    checkout = next(
        i for i, s in enumerate(steps) if str(s.get("uses", "")).startswith("actions/checkout@")
    )
    found = [i for i, s in enumerate(steps) if str(s.get("uses", "")).startswith(ACTION)]
    assert len(found) == 1, f"gates-security has {len(found)} steps with {ACTION}"
    step = steps[found[0]]
    assert checkout < found[0]
    assert "if" not in step
    # The action fails when it finds fewer JARs than this, so a wrapper cannot drop out unseen.
    assert int(step["with"]["min-wrapper-count"]) == len(wrappers)
    assert step["with"].get("allow-checksums", "") == ""
    assert step["with"].get("allow-snapshots", "false") in ("false", False)
