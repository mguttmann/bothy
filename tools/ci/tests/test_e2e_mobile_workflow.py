# SPDX-License-Identifier: Apache-2.0
"""The nightly mobile E2E workflow (brief M5-11 Design note 4; docs/decisions.d/M5-11.md).

`.github/workflows/e2e-mobile.yml` runs the Android suite of `make e2e-mobile` on a GitHub-hosted
Linux runner with an emulator and Docker: on a schedule and by hand, never on a pull request (it
is no required check), with read-only permissions, no secrets and every action pinned by SHA.
"""

from __future__ import annotations

from pathlib import Path
from typing import Any

import check_runners as cr

REPO = Path(__file__).resolve().parents[3]
WORKFLOW = REPO / ".github" / "workflows" / "e2e-mobile.yml"


def load() -> dict[str, Any]:
    """The parsed workflow."""
    doc = cr.load_yaml(WORKFLOW.read_text(encoding="utf-8"))
    assert isinstance(doc, dict)
    return doc


def test_runs_nightly_and_by_hand_only() -> None:
    triggers = cr._triggers(load())
    assert set(triggers) == {"schedule", "workflow_dispatch"}
    crons = [entry["cron"] for entry in triggers["schedule"]]
    assert len(crons) == 1
    _minute, _hour, dom, month, dow = crons[0].split()
    assert (dom, month, dow) == ("*", "*", "*"), "every night"


def test_read_only_permissions_and_no_secrets() -> None:
    doc = load()
    assert doc["permissions"] == {"contents": "read"}
    text = WORKFLOW.read_text(encoding="utf-8")
    assert "secrets." not in text
    for job in doc["jobs"].values():
        assert job.get("permissions", {"contents": "read"}) == {"contents": "read"}


def test_passes_the_runner_policy() -> None:
    text = WORKFLOW.read_text(encoding="utf-8")
    assert cr.check_workflow(".github/workflows/e2e-mobile.yml", text) == []


def test_every_action_is_pinned_by_sha() -> None:
    for job in load()["jobs"].values():
        for step in job["steps"]:
            uses = step.get("uses")
            if uses:
                assert cr._PINNED.match(uses), uses


def test_runs_the_android_suite_of_make_e2e_mobile() -> None:
    (job,) = load()["jobs"].values()
    assert job["runs-on"] == "ubuntu-24.04"
    emulator = next(s for s in job["steps"] if "android-emulator-runner" in s.get("uses", ""))
    script = emulator["with"]["script"]
    assert "make e2e-mobile" in script
    assert "MOBILE_E2E_PLATFORMS=android" in script
    assert "MOBILE_E2E_SERIAL=" in script
    names = " ".join(str(s.get("run", "")) for s in job["steps"])
    assert "make image" in names, "the stack needs the server image"
    assert "kvm" in names, "hardware acceleration for the emulator"


def test_the_android_job_runs_the_terminal_suite_too() -> None:
    """Brief M5-13 Scope: the terminal suite runs in the same job, against the terminal stack of
    the Go helper, so the job sets up Go of the server module and leaves the suites unrestricted.
    """
    (job,) = load()["jobs"].values()
    go = [s for s in job["steps"] if "actions/setup-go" in s.get("uses", "")]
    assert len(go) == 1, "the helper of the terminal stack needs Go"
    assert go[0]["with"]["go-version-file"] == "services/server/go.mod"
    emulator = next(s for s in job["steps"] if "android-emulator-runner" in s.get("uses", ""))
    steps = list(job["steps"])
    assert steps.index(go[0]) < steps.index(emulator)
    script = emulator["with"]["script"]
    env = emulator.get("env", {})
    suites = env.get("MOBILE_E2E_SUITES", "")
    assert "MOBILE_E2E_SUITES" not in script
    assert suites == "" or "terminal" in suites.split()
