# SPDX-License-Identifier: Apache-2.0
"""ci.yml runs `make gates` in parallel jobs and still runs exactly its targets.

docs/quality/gates.md, "CI jobs": the static gates, the docs and contract gates, the security and
supply-chain gates and the tests run in their own jobs at the same time. Together they run every
target of `make gates` once (the tests split by component), for the components of the plan, and
ci-ok requires every one of them (tools/ci/ci_ok.py).
"""

from __future__ import annotations

import re
from pathlib import Path
from typing import Any

import ci_ok
import components as component_map
import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
WORKFLOW = REPO / ".github" / "workflows" / "ci.yml"
GATE_JOBS = (
    "gates",
    "gates-docs",
    "gates-security",
    "test-tools",
    "test-server",
    "test-apps",
    "test-sdk-plugins",
)
# Component test jobs and the component each one tests; the plan may skip them.
COMPONENT_TESTS = {
    "test-server": {"server"},
    "test-apps": {"apps"},
    "test-sdk-plugins": {"sdk-python", "sdk-typescript", "plugins"},
}
GO_IF = "hashFiles('services/*/go.mod') != ''"
FLUTTER_IF = "hashFiles('apps/*/pubspec.yaml', 'packages/*/pubspec.yaml') != ''"
ANDROID_IF = "hashFiles('packages/*/android/gradlew') != ''"
_MAKE = re.compile(r"(?:^|[;&|]\s*|\n\s*)make\s+([^\n]*)")


def _jobs() -> dict[str, Any]:
    data = yaml.safe_load(WORKFLOW.read_text(encoding="utf-8"))
    jobs = data["jobs"]
    assert isinstance(jobs, dict)
    return jobs


def _make_args(job: str) -> list[list[str]]:
    out = []
    for step in _jobs()[job]["steps"]:
        for args in _MAKE.findall(str(step.get("run", ""))):
            if args.strip() != "bootstrap-ci":
                out.append(args.split())
    return out


def _targets(job: str) -> list[str]:
    return [a for args in _make_args(job) for a in args if re.fullmatch(r"[a-z][a-z0-9-]*", a)]


def _gates_prerequisites() -> set[str]:
    text = (REPO / "Makefile").read_text(encoding="utf-8").replace("\\\n", " ")
    [line] = [ln for ln in text.splitlines() if ln.startswith("gates: ") and "##" not in ln]
    return set(line.removeprefix("gates:").split())


def test_the_parallel_jobs_run_every_target_of_make_gates() -> None:
    covered = {t for job in GATE_JOBS for t in _targets(job)}
    assert covered == _gates_prerequisites()


def test_no_gate_runs_twice_except_the_tests_split_by_component() -> None:
    seen: dict[str, list[str]] = {}
    for job in GATE_JOBS:
        for target in _targets(job):
            seen.setdefault(target, []).append(job)
    twice = {t: jobs for t, jobs in seen.items() if len(jobs) > 1}
    assert set(twice) == {"test", "coverage"}, twice
    assert set(twice["test"]) == {"test-tools", *COMPONENT_TESTS}


def test_no_job_runs_make_gates_whole() -> None:
    for job, spec in _jobs().items():
        for step in spec.get("steps", []):
            assert not re.search(r"\bmake gates\b", str(step.get("run", ""))), job


def test_the_tools_tests_run_with_no_component_and_every_area() -> None:
    [args] = _make_args("test-tools")
    assert args[:2] == ["test", "coverage"]
    assert 'COMPONENTS="$COMPONENTS"' in args
    assert not any(a.startswith("AREAS=") for a in args)
    [step] = [s for s in _jobs()["test-tools"]["steps"] if "make test" in str(s.get("run", ""))]
    assert step["env"]["COMPONENTS"] == "none"


@pytest.mark.parametrize("job", sorted(COMPONENT_TESTS))
def test_component_tests_leave_the_tools_suite_to_its_job(job: str) -> None:
    [args] = _make_args(job)
    assert args[:2] == ["test", "coverage"]
    assert "AREAS=none" in args


@pytest.mark.parametrize(("job", "components"), sorted(COMPONENT_TESTS.items()))
def test_component_tests_run_when_the_plan_selects_their_component(
    job: str, components: set[str]
) -> None:
    # The plan lists the job for these components (tools/ci/components.py COMPONENT_TEST_JOBS).
    assert _jobs()[job]["if"] == f"contains(format(' {{0}} ', needs.plan.outputs.jobs), ' {job} ')"
    assert set(component_map.COMPONENT_TEST_JOBS[job]) == components


@pytest.mark.parametrize("job", ["gates", "gates-docs", "gates-security"])
def test_repository_gates_follow_the_plan(job: str) -> None:
    assert "if" not in _jobs()[job]
    [step] = [
        s
        for s in _jobs()[job]["steps"]
        if "make " in str(s.get("run", "")) and "bootstrap" not in str(s["run"])
    ]
    assert step["env"]["COMPONENTS"] == "${{ needs.plan.outputs.components }}"


@pytest.mark.parametrize("job", GATE_JOBS)
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
    assert runs.index("make bootstrap-ci") < max(
        i for i, r in enumerate(runs) if "make " in r and "bootstrap" not in r
    )


def test_ci_ok_requires_every_gate_job() -> None:
    required = {n for rule in ci_ok.RULES for n in rule.names}
    for job in GATE_JOBS:
        assert _jobs()[job]["name"] in required, job
