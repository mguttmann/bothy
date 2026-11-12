# SPDX-License-Identifier: Apache-2.0
"""The desktop E2E workflow (brief M4-34 Design note 6; docs/decisions.d/AUD-misc-testing-01.md).

`.github/workflows/e2e-desktop.yml` has two jobs with a schedule each: every week it builds the
suites on macOS and Windows, where no Docker engine can start the stack, and every night it runs
the E2E suites headless on a Linux runner against the real server of the Compose stack. That
nightly job is the run in which the Flutter client meets the real server in CI. Never on a pull
request, with read-only permissions, no secrets and every action pinned by SHA.
"""

from __future__ import annotations

import re
from pathlib import Path
from typing import Any

import check_runners as cr

REPO = Path(__file__).resolve().parents[3]
WORKFLOW = REPO / ".github" / "workflows" / "e2e-desktop.yml"
MAKEFILE = REPO / "Makefile"
SUITE = REPO / "apps" / "client" / "tool" / "e2e_suite.sh"
_OTHER_SCHEDULE = re.compile(r"&& github\.event\.schedule != '([^']+)'$")


def load() -> dict[str, Any]:
    """The parsed workflow."""
    doc = cr.load_yaml(WORKFLOW.read_text(encoding="utf-8"))
    assert isinstance(doc, dict)
    return doc


def crons() -> list[str]:
    """The schedules of the workflow."""
    return [entry["cron"] for entry in cr._triggers(load())["schedule"]]


def test_runs_on_its_schedules_and_by_hand_only() -> None:
    assert set(cr._triggers(load())) == {"schedule", "workflow_dispatch"}
    weekly, nightly = crons()
    assert weekly.split()[2:] != ["*", "*", "*"], "the builds run once a week"
    assert nightly.split()[2:] == ["*", "*", "*"], "the suites run every night"


def test_each_job_runs_for_its_own_schedule_and_for_a_manual_run() -> None:
    weekly, nightly = crons()
    jobs = load()["jobs"]
    assert set(jobs) == {"build", "headless"}
    skipped = {}
    for name, job in jobs.items():
        condition = cr._norm(job["if"])
        assert cr.has_public_guard(condition), name
        match = _OTHER_SCHEDULE.search(condition)
        assert match, f"{name} names no schedule it leaves to the other job"
        skipped[name] = match.group(1)
    # A manual run has no schedule, so neither condition holds it back.
    assert skipped == {"build": nightly, "headless": weekly}


def test_read_only_permissions_no_secrets_and_the_runner_policy() -> None:
    doc = load()
    text = WORKFLOW.read_text(encoding="utf-8")
    assert doc["permissions"] == {"contents": "read"}
    assert "secrets." not in text
    for job in doc["jobs"].values():
        assert job.get("permissions", {"contents": "read"}) == {"contents": "read"}
        for step in job["steps"]:
            if step.get("uses"):
                assert cr._PINNED.match(step["uses"]), step["uses"]
    assert cr.check_workflow(WORKFLOW.name, text) == []


def test_the_nightly_job_runs_the_suites_headless_against_the_stack() -> None:
    job = load()["jobs"]["headless"]
    assert job["runs-on"] == "ubuntu-24.04", "the hosted runner with a Docker engine"
    [suite] = [s for s in job["steps"] if "make e2e-desktop" in str(s.get("run", ""))]
    assert suite["run"].strip() == "make e2e-desktop"
    assert suite["env"] == {"E2E_DEVICE": "flutter-tester"}
    steps = list(job["steps"])
    [image] = [s for s in steps if "make image" in str(s.get("run", ""))]
    assert steps.index(image) < steps.index(suite), "the stack starts from the server image"
    [go] = [s for s in steps if "actions/setup-go" in str(s.get("uses", ""))]
    assert go["with"]["go-version-file"] == "services/server/go.mod"
    assert steps.index(go) < steps.index(suite), "the helper of the terminal stack needs Go"
    assert any("subosito/flutter-action" in str(s.get("uses", "")) for s in steps)


def test_the_target_and_the_suite_script_know_the_headless_device() -> None:
    # The job relies on both: the target passes E2E_DEVICE on, the script runs flutter test.
    recipe = MAKEFILE.read_text(encoding="utf-8")
    assert re.search(r"^e2e-desktop:.*\n\t.*e2e_suite\.sh e2e", recipe, re.MULTILINE)
    assert "e2e-desktop-terminal" in recipe
    script = SUITE.read_text(encoding="utf-8")
    assert re.search(
        r'if \[\[ "\$device" = flutter-tester \]\]; then\s+exec flutter test "\$suite"', script
    )
