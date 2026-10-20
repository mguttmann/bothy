# SPDX-License-Identifier: Apache-2.0
"""The sonar job of ci.yml scans the coverage reports of the gates job instead of testing again.

`make gates` runs `make test`, which writes every coverage report SonarQube reads. The gates job
packs those reports into the artifact SONAR_ARTIFACT; the sonar job needs gates, unpacks the
artifact and runs `make sonar-scan`, the scan without its own test run. Locally `make sonar`
still runs `make test` first. ci.yml also runs on pushes to main, so main gets its own analysis.
"""

from __future__ import annotations

import fnmatch
import re
import subprocess
from pathlib import Path
from typing import Any

import yaml

REPO = Path(__file__).resolve().parents[3]
WORKFLOW = REPO / ".github" / "workflows" / "ci.yml"
PROPERTIES = REPO / "sonar-project.properties"
SONAR_ARTIFACT = "sonar-coverage"


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


def _packed_globs() -> list[str]:
    [step] = [s for s in _steps("gates") if "REPORTS" in s.get("env", {})]
    globs = str(step["env"]["REPORTS"]).split()
    assert globs
    return globs


def test_properties_name_coverage_reports() -> None:
    # Guards the parser below: every language Sonar reads coverage for today.
    joined = " ".join(_report_paths())
    for path in ("services/server/coverage.out", "sdk/python/coverage.xml", "lcov.info"):
        assert path in joined


def test_gates_pack_every_coverage_report_sonar_reads() -> None:
    globs = _packed_globs()
    missing = [p for p in _report_paths() if not any(fnmatch.fnmatchcase(p, g) for g in globs)]
    assert missing == []


def test_gates_upload_the_reports_after_make_gates() -> None:
    steps = _steps("gates")
    make = next(i for i, s in enumerate(steps) if s.get("name") == "make gates")
    [pack] = [i for i, s in enumerate(steps) if "REPORTS" in s.get("env", {})]
    [upload] = [i for i, s in enumerate(steps) if _uses(s, "actions/upload-artifact")]
    assert make < pack < upload
    assert steps[upload]["with"]["name"] == SONAR_ARTIFACT
    assert steps[upload]["with"]["if-no-files-found"] == "error"


def test_sonar_needs_gates_and_scans_without_testing() -> None:
    job = _workflow()["jobs"]["sonar"]
    assert set(job["needs"]) == {"plan", "gates"}
    steps = job["steps"]
    [download] = [i for i, s in enumerate(steps) if _uses(s, "actions/download-artifact")]
    assert steps[download]["with"]["name"] == SONAR_ARTIFACT
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
    # ci-ok has no `needs` and waits for the others; sonar now runs after gates, so its
    # timeout must cover plan, gates and sonar in a row.
    jobs = _workflow()["jobs"]
    chain = sum(int(jobs[j]["timeout-minutes"]) for j in ("plan", "gates", "sonar"))
    assert int(jobs["ci-ok"]["timeout-minutes"]) > chain


def test_pack_step_skips_reports_a_partial_run_did_not_write(tmp_path: Path) -> None:
    # A pull request that selects only some components writes only their reports; the step
    # must pack what exists instead of failing on a literal path such as sdk/python/coverage.xml.
    [step] = [s for s in _steps("gates") if "REPORTS" in s.get("env", {})]
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
    done = subprocess.run(
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
