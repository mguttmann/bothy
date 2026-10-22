# SPDX-License-Identifier: Apache-2.0
"""build-verify.yml runs every gate of the required CI for a push to build/**, without secrets.

docs/quality/push-discipline.md, "Build branches": build teams push to `build/<ticket>` and read
the result of the job build-verify-ok; the pre-push hook runs only the fast gates for such a push
(tools/hooks/build_push.py), so this workflow must cover every target of `make gates`.
"""

from __future__ import annotations

import re
from pathlib import Path
from typing import Any

import check_runners
import pytest

REPO = Path(__file__).resolve().parents[3]
WORKFLOW = REPO / ".github" / "workflows" / "build-verify.yml"
GO_IF = "hashFiles('services/*/go.mod') != ''"
FLUTTER_IF = "hashFiles('apps/*/pubspec.yaml', 'packages/*/pubspec.yaml') != ''"
ANDROID_IF = "hashFiles('packages/*/android/gradlew') != ''"
# The jobs of ci.yml outside `make gates` that ci-ok requires; sonar and the private deny list
# need secrets and stay in ci.yml.
PLATFORM_JOBS = (
    "postgres-image-amd64",
    "postgres-image-arm64",
    "console-e2e",
    "apps-macos",
    "apps-windows",
    "apps-android-26",
    "apps-android-36",
)
_MAKE = re.compile(r"(?:^|[;&|]\s*|\n\s*)make\s+([^\n]*)")


def _workflow() -> dict[str, Any]:
    data = check_runners.load_yaml(WORKFLOW.read_text(encoding="utf-8"))
    assert isinstance(data, dict)
    return data


def _jobs() -> dict[str, Any]:
    jobs = _workflow()["jobs"]
    assert isinstance(jobs, dict)
    return jobs


def _make_targets(run: str) -> list[str]:
    targets: list[str] = []
    for args in _MAKE.findall(run):
        targets += [a for a in args.split() if re.fullmatch(r"[a-z][a-z0-9-]*", a)]
    return targets


def _gates_prerequisites() -> set[str]:
    text = (REPO / "Makefile").read_text(encoding="utf-8").replace("\\\n", " ")
    [line] = [ln for ln in text.splitlines() if ln.startswith("gates: ") and "##" not in ln]
    return set(line.removeprefix("gates:").split())


def _job_targets() -> dict[str, list[str]]:
    out: dict[str, list[str]] = {}
    for job_id, job in _jobs().items():
        for step in job.get("steps", []):
            targets = [t for t in _make_targets(str(step.get("run", ""))) if t != "bootstrap-ci"]
            if targets:
                out.setdefault(job_id, []).extend(targets)
    return out


def test_runs_only_on_a_push_to_a_build_branch() -> None:
    assert _workflow()["on"] == {"push": {"branches": ["build/**"]}}


def test_uses_no_secret_and_no_environment() -> None:
    text = WORKFLOW.read_text(encoding="utf-8")
    assert "secrets." not in text
    assert all("environment" not in job for job in _jobs().values())
    assert _workflow()["permissions"] == {"contents": "read"}


def test_a_newer_push_cancels_the_older_run_of_the_branch() -> None:
    concurrency = _workflow()["concurrency"]
    assert "github.ref" in concurrency["group"]
    assert concurrency["cancel-in-progress"] is True


def test_every_target_of_make_gates_runs() -> None:
    covered = {t for targets in _job_targets().values() for t in targets}
    assert _gates_prerequisites() - covered == set()


def test_no_gate_runs_twice_except_the_tests_split_by_component() -> None:
    seen: dict[str, list[str]] = {}
    gates = _gates_prerequisites()
    for job_id, targets in _job_targets().items():
        for target in (t for t in targets if t in gates):
            seen.setdefault(target, []).append(job_id)
    twice = {t: jobs for t, jobs in seen.items() if len(jobs) > 1}
    assert set(twice) <= {"test", "coverage"}, twice


def test_the_generic_deny_rules_scan_the_push() -> None:
    [run] = [s["run"] for s in _jobs()["denylist"]["steps"] if "run" in s]
    assert run == "make denylist DENYLIST_FLAGS=--allow-missing"


@pytest.mark.parametrize("job", PLATFORM_JOBS)
def test_the_platform_jobs_of_ci_run_too(job: str) -> None:
    ci = check_runners.load_yaml(
        (REPO / ".github" / "workflows" / "ci.yml").read_text(encoding="utf-8")
    )
    mine = _jobs()[job]
    theirs = ci["jobs"][job]
    assert mine["runs-on"] == theirs["runs-on"]
    assert [s.get("uses") for s in mine["steps"]] == [s.get("uses") for s in theirs["steps"]]
    assert [s.get("run") for s in mine["steps"]] == [s.get("run") for s in theirs["steps"]]
    assert [s.get("with", {}).get("script") for s in mine["steps"]] == [
        s.get("with", {}).get("script") for s in theirs["steps"]
    ]


@pytest.mark.parametrize("job", ["static", "test-tools", "test-server", "test-apps", "docs"])
def test_gate_jobs_install_every_toolchain_with_caches(job: str) -> None:
    steps = _jobs()[job]["steps"]

    def step(action: str) -> dict[str, Any]:
        [found] = [s for s in steps if str(s.get("uses", "")).startswith(action + "@")]
        return found

    assert step("actions/checkout")["with"]["fetch-depth"] == 0
    assert step("astral-sh/setup-uv")["with"]["enable-cache"] is True
    assert step("actions/setup-node")["with"]["cache"] == "npm"
    go = step("actions/setup-go")
    assert go["if"] == GO_IF
    assert go["with"]["cache-dependency-path"] == "services/*/go.sum"
    flutter = step("subosito/flutter-action")
    assert flutter["if"] == FLUTTER_IF
    assert flutter["with"]["cache"] is True
    assert step("actions/setup-java")["if"] == ANDROID_IF
    assert step("android-actions/setup-android")["if"] == ANDROID_IF
    assert ".cache/tools" in step("actions/cache")["with"]["path"]
    runs = [str(s.get("run", "")) for s in steps]
    assert runs.index("make bootstrap-ci") < max(i for i, r in enumerate(runs) if "make " in r)


def test_build_verify_ok_sums_up_every_job() -> None:
    jobs = _jobs()
    ok = jobs["build-verify-ok"]
    assert ok["name"] == "build-verify-ok"
    assert ok["if"] == "always()"
    assert set(ok["needs"]) == set(jobs) - {"build-verify-ok"}
    [run] = [s["run"] for s in ok["steps"] if "run" in s]
    # tools/ci/ci_ok.py needs: the plan and every needed job passed (tools/ci/tests/test_ci_ok.py).
    assert run.rstrip().endswith("tools/ci/ci_ok.py needs")


def test_the_plan_measures_from_the_build_base() -> None:
    """The plan step takes its base from tools/ci/build_base.py, with the pushed branch."""
    plan = _jobs()["plan"]
    [step] = [s for s in plan["steps"] if s.get("id") == "jobs"]
    assert step["env"]["BRANCH"] == "${{ github.ref_name }}"
    run = str(step["run"])
    assert "tools/ci/build_base.py" in run
    assert '--branch "$BRANCH"' in run
    assert "git merge-base" not in run
    assert 'ci-plan --base "$base" --head "$HEAD_SHA"' in run
    assert "ci-plan --components all" in run
    # Every branch and its history: the base ref is a remote branch of the checkout.
    uses = [s for s in plan["steps"] if str(s.get("uses", "")).startswith("actions/checkout@")]
    assert [s["with"]["fetch-depth"] for s in uses] == [0]
