# SPDX-License-Identifier: Apache-2.0
"""The sonar job of ci.yml scans the coverage reports of the test jobs instead of testing again.

The test jobs of the components (TEST_JOBS) run `make test`, which writes every coverage report
SonarQube reads. Each packs its reports into the artifact `sonar-coverage-<part>`; the sonar job
needs them, unpacks every artifact and runs `make sonar-scan`, the scan without its own test run.
Locally `make sonar` still runs `make test` first. ci.yml also runs on pushes to main, so main
gets its own analysis.
"""

from __future__ import annotations

import fnmatch
import re
import subprocess
from pathlib import Path
from typing import Any

import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
WORKFLOW = REPO / ".github" / "workflows" / "ci.yml"
PROPERTIES = REPO / "sonar-project.properties"
SONAR_ARTIFACT = "sonar-coverage"
# The jobs that run the component tests (the tools tests write no report SonarQube reads).
TEST_JOBS = {"test-server": "server", "test-apps": "apps", "test-sdk-plugins": "sdk-plugins"}


def _workflow() -> dict[Any, Any]:
    data = yaml.safe_load(WORKFLOW.read_text(encoding="utf-8"))
    assert isinstance(data, dict)
    return data


def _steps(job: str) -> list[dict[str, Any]]:
    steps = _workflow()["jobs"][job]["steps"]
    assert isinstance(steps, list)
    return steps


def _uses(step: dict[str, Any], action: str) -> bool:
    return str(step.get("uses", "")).startswith(action + "@")


def _report_paths() -> list[str]:
    paths: list[str] = []
    for line in PROPERTIES.read_text(encoding="utf-8").splitlines():
        if re.match(r"sonar\.[a-z]+\.(coverage|lcov)\.reportPaths=", line):
            paths += line.split("=", 1)[1].split(",")
    return paths


def _packed_globs(job: str) -> list[str]:
    [step] = [s for s in _steps(job) if "REPORTS" in s.get("env", {})]
    globs = str(step["env"]["REPORTS"]).split()
    assert globs
    return globs


def test_properties_name_coverage_reports() -> None:
    # Guards the parser below: every language Sonar reads coverage for today.
    joined = " ".join(_report_paths())
    for path in ("services/server/coverage.out", "sdk/python/coverage.xml", "lcov.info"):
        assert path in joined


@pytest.mark.parametrize("job", sorted(TEST_JOBS))
def test_test_jobs_pack_every_coverage_report_sonar_reads(job: str) -> None:
    globs = _packed_globs(job)
    missing = [p for p in _report_paths() if not any(fnmatch.fnmatchcase(p, g) for g in globs)]
    assert missing == []


@pytest.mark.parametrize(("job", "part"), sorted(TEST_JOBS.items()))
def test_test_jobs_upload_the_reports_after_make_test(job: str, part: str) -> None:
    steps = _steps(job)
    make = next(i for i, s in enumerate(steps) if "make test coverage" in str(s.get("run", "")))
    [pack] = [i for i, s in enumerate(steps) if "REPORTS" in s.get("env", {})]
    [upload] = [i for i, s in enumerate(steps) if _uses(s, "actions/upload-artifact")]
    assert make < pack < upload
    assert steps[upload]["with"]["name"] == f"{SONAR_ARTIFACT}-{part}"
    assert steps[upload]["with"]["if-no-files-found"] == "error"


def test_sonar_needs_the_test_jobs_and_scans_without_testing() -> None:
    job = _workflow()["jobs"]["sonar"]
    assert set(job["needs"]) == {"plan", *TEST_JOBS}
    # A skipped test job (component not planned) must not skip the scan, a failed one must.
    condition = " ".join(str(job["if"]).split())
    assert condition.startswith("always() && needs.plan.result == 'success'")
    assert "!contains(needs.*.result, 'failure')" in condition
    assert "!contains(needs.*.result, 'cancelled')" in condition
    steps = job["steps"]
    [download] = [i for i, s in enumerate(steps) if _uses(s, "actions/download-artifact")]
    assert steps[download]["with"]["pattern"] == f"{SONAR_ARTIFACT}-*"
    runs = [str(s.get("run", "")) for s in steps]
    [scan] = [i for i, r in enumerate(runs) if "make sonar-scan" in r]
    assert download < scan
    assert not any(re.search(r"make (sonar|test|gates)\b(?!-)", r) for r in runs)
    # The JDK and the Android SDK only served the JVM tests of `make test`.
    for action in ("actions/setup-java", "android-actions/setup-android"):
        assert not any(_uses(s, action) for s in steps), action


def test_ci_runs_on_pushes_to_main() -> None:
    # PyYAML reads the key `on` as True.
    triggers = _workflow()[True]
    assert triggers["push"] == {"branches": ["main"]}


def _rule(target: str) -> str:
    text = (REPO / "Makefile").read_text(encoding="utf-8")
    [line] = [x for x in text.splitlines() if x.startswith(f"{target}:")]
    return line.split("##", 1)[0]


def test_make_sonar_tests_first_and_sonar_scan_does_not() -> None:
    assert _rule("sonar").split(":", 1)[1].split() == ["test"]
    assert _rule("sonar-scan").split(":", 1)[1].split() == []


def test_ci_ok_outlasts_the_longest_chain() -> None:
    # ci-ok has no `needs` and waits for the others; sonar runs after the test jobs, so its
    # timeout must cover plan, the longest test job and sonar in a row.
    jobs = _workflow()["jobs"]
    longest = max(int(jobs[j]["timeout-minutes"]) for j in TEST_JOBS)
    chain = int(jobs["plan"]["timeout-minutes"]) + longest + int(jobs["sonar"]["timeout-minutes"])
    assert int(jobs["ci-ok"]["timeout-minutes"]) > chain


@pytest.mark.parametrize("job", sorted(TEST_JOBS))
def test_pack_step_skips_reports_the_job_did_not_write(tmp_path: Path, job: str) -> None:
    """A test job writes only its components' reports; it packs what exists, a literal path of
    another component (sdk/python/coverage.xml) must not fail the step."""
    [step] = [s for s in _steps(job) if "REPORTS" in s.get("env", {})]
    work = tmp_path / "repo"
    (work / "services" / "server").mkdir(parents=True)
    (work / "services" / "server" / "coverage.out").write_text("mode: set\n", encoding="utf-8")
    temp = tmp_path / "runner"
    temp.mkdir()
    env = {
        "PATH": "/usr/bin:/bin",
        "REPORTS": str(step["env"]["REPORTS"]),
        "RUNNER_TEMP": str(temp),
    }
    done = subprocess.run(  # nosec B603 B607 - the workflow's own step with a fixed environment
        ["bash", "-e", "-c", str(step["run"])],
        cwd=work,
        env=env,
        capture_output=True,
        text=True,
        check=False,
    )
    assert done.returncode == 0, done.stderr
    listed = (temp / SONAR_ARTIFACT / "reports.txt").read_text(encoding="utf-8").split()
    assert listed == ["services/server/coverage.out"]


def test_a_push_to_main_is_never_cancelled_by_the_next_merge() -> None:
    # Merges come faster than one full run; with cancel-in-progress every main run was
    # cancelled by the next merge, and SonarQube never got an analysis of main. A newer push
    # waits instead (GitHub keeps at most one pending run per group).
    concurrency = _workflow()["concurrency"]
    assert concurrency["cancel-in-progress"] == "${{ github.event_name != 'push' }}"
