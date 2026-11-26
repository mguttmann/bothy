# SPDX-License-Identifier: Apache-2.0
"""The computer helper jobs of host-platform.yml (brief CU-02, docs/spec/27-computer-use.md 16).

A probe job per platform builds and tests the helper, installs the package with the optional
helper component and records whether the hosted runner allows capture and input; it never fails
on that. The native job of a platform runs the same steps as a test, only after its probe passed.
The platform suite stays manual and scheduled; a pull request that changes the helpers runs only
the helper jobs.
"""

from __future__ import annotations

from pathlib import Path
from typing import Any

import check_runners as cr
import pytest

REPO = Path(__file__).resolve().parents[3]
WORKFLOW = REPO / ".github" / "workflows" / "host-platform.yml"
PLATFORMS = (("helper-macos", "macos-15"), ("helper-windows", "windows-2025"))


def _doc() -> dict[str, Any]:
    doc = cr.load_yaml(WORKFLOW.read_text(encoding="utf-8"))
    assert isinstance(doc, dict)
    return doc


def _trigger(doc: dict[str, Any]) -> dict[str, Any]:
    # YAML 1.1 reads the key `on` as true.
    on = doc.get("on", doc.get(True))
    assert isinstance(on, dict)
    return on


def _runs(job: dict[str, Any]) -> list[str]:
    return [str(s.get("run", "")) for s in job["steps"]]


def test_the_policy_check_passes() -> None:
    assert cr.check_workflow(str(WORKFLOW), WORKFLOW.read_text(encoding="utf-8")) == []


def test_a_pull_request_runs_only_for_the_helper_paths_and_not_the_suite() -> None:
    doc = _doc()
    paths = _trigger(doc)["pull_request"]["paths"]
    for want in ("services/host/desktop/**", "infra/host/**", "tools/ci/computer_probe.py"):
        assert want in paths
    assert "github.event_name != 'pull_request'" in doc["jobs"]["platform"]["if"]


@pytest.mark.parametrize(("name", "runner"), PLATFORMS)
def test_the_probe_job_records_and_does_not_fail(name: str, runner: str) -> None:
    job = _doc()["jobs"][name]
    assert job["runs-on"] == runner
    assert cr.has_public_guard(job["if"])
    assert job["outputs"] == {
        "capture": "${{ steps.probe.outputs.capture }}",
        "input": "${{ steps.probe.outputs.input }}",
    }
    (probe,) = [s for s in job["steps"] if s.get("id") == "probe"]
    run = str(probe["run"])
    assert "tools/ci/computer_probe.py --mode probe" in run
    assert '--github-output "$GITHUB_OUTPUT"' in run
    assert "continue-on-error" not in probe
    runs = "\n".join(_runs(job))
    # Unit tests and the package with the optional helper run whatever the probe says.
    assert ("swift test" if "macos" in name else "ctest") in runs
    notice = [s for s in job["steps"] if "::notice" in str(s.get("run", ""))]
    assert len(notice) == 1
    assert "steps.probe.outputs.capture != 'true'" in notice[0]["if"]


@pytest.mark.parametrize(("name", "runner"), PLATFORMS)
def test_the_native_job_runs_only_after_its_probe_passed(name: str, runner: str) -> None:
    job = _doc()["jobs"][f"{name}-native"]
    assert job["runs-on"] == runner
    assert job["needs"] == name
    cond = str(job["if"])
    assert cr.has_public_guard(cond)
    assert f"needs.{name}.outputs.capture == 'true'" in cond
    assert f"needs.{name}.outputs.input == 'true'" in cond
    assert any("tools/ci/computer_probe.py --mode native" in r for r in _runs(job))


def test_the_package_steps_install_with_and_without_the_component() -> None:
    jobs = _doc()["jobs"]
    mac = "\n".join(_runs(jobs["helper-macos"]))
    assert "helper-package-macos" in mac
    assert mac.count("sudo installer -pkg") == 2
    assert "-applyChoiceChangesXML" in mac
    win = "\n".join(_runs(jobs["helper-windows"]))
    for step in ("helper-cert-windows", "helper-sign-windows", "helper-package-windows"):
        assert step in win
    assert "ADDLOCAL=ComputerHelper" in win
    assert "-DHELPER_UIACCESS=OFF" in win
    assert "-DHELPER_UIACCESS=ON" in win
