# SPDX-License-Identifier: Apache-2.0
"""Tests for the ci-ok job (tools/ci/ci_ok.py).

ci-ok starts with the run, without `needs`, so that its check run is pending from the first
second of every run: an edit of the pull request text can never leave an older green ci-ok
as the latest result while the new run is still going.
"""

from __future__ import annotations

from pathlib import Path
from typing import Any

import ci_ok
import components
import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
CI = REPO / ".github" / "workflows" / "ci.yml"
RELEASE = REPO / ".github" / "workflows" / "release.yml"
BUILD_VERIFY = REPO / ".github" / "workflows" / "build-verify.yml"
# Every conditional job needed: a push to main, or a change of a shared input.
ALL = frozenset(components.CONDITIONAL_JOBS)

GREEN = {
    "plan": "success",
    "gates": "success",
    "gates (docs and contracts)": "success",
    "gates (security and supply chain)": "success",
    "test (tools)": "success",
    "test (server)": "success",
    "test (apps)": "success",
    "test (SDKs and plugins)": "success",
    "denylist (private list)": "success",
    "denylist (generic rules, fork)": "skipped",
    "sonar": "success",
    "postgres-image (amd64)": "success",
    "postgres-image (arm64)": "success",
    "apps (macOS)": "success",
    "apps (Windows)": "success",
    "apps (Android API 26)": "success",
    "apps (Android API 36)": "success",
}


def jobs(results: dict[str, str | None], prefix: str = "", own: str = "runner-9") -> list[Any]:
    out: list[dict[str, Any]] = [
        {
            "name": f"{prefix}{name}",
            "status": "completed" if conclusion else "in_progress",
            "conclusion": conclusion,
            "runner_name": f"runner-{i}",
        }
        for i, (name, conclusion) in enumerate(results.items())
    ]
    out.append(
        {"name": f"{prefix}ci-ok", "status": "in_progress", "conclusion": None, "runner_name": own}
    )
    return out


def test_all_green_passes() -> None:
    assert ci_ok.evaluate(jobs(GREEN), "runner-9", ALL)[0] == "pass"


RULE_OF_JOB = {rule.job: rule.names[0] for rule in ci_ok.RULES if rule.job}


@pytest.mark.parametrize("job", sorted(components.CONDITIONAL_JOBS))
def test_a_skipped_job_passes_only_when_the_plan_left_it_out(job: str) -> None:
    """A skip never hides a failure: a needed job must succeed, an unneeded one must not fail."""
    name = RULE_OF_JOB[job]
    skipped = jobs({**GREEN, name: "skipped"})
    assert ci_ok.evaluate(skipped, "runner-9", ALL)[0] == "fail"
    assert ci_ok.evaluate(skipped, "runner-9", ALL - {job})[0] == "pass"
    for conclusion in ("failure", "cancelled", "timed_out"):
        ran = jobs({**GREEN, name: conclusion})
        assert ci_ok.evaluate(ran, "runner-9", ALL - {job})[0] == "fail"
        assert ci_ok.evaluate(ran, "runner-9", ALL)[0] == "fail"


def test_every_conditional_job_has_a_rule() -> None:
    assert set(RULE_OF_JOB) == ALL


def test_a_plan_without_conditional_jobs_accepts_their_skips() -> None:
    results = {**GREEN, **{RULE_OF_JOB[job]: "skipped" for job in ALL}}
    assert ci_ok.evaluate(jobs(results), "runner-9", frozenset())[0] == "pass"
    assert ci_ok.evaluate(jobs(results), "runner-9", frozenset({"test-server"}))[0] == "fail"


@pytest.mark.parametrize(
    ("value", "expected"),
    [
        ("none", frozenset()),
        ("test-server test-apps", frozenset({"test-server", "test-apps"})),
        (" test-apps ", frozenset({"test-apps"})),
    ],
)
def test_needed_jobs_are_read_from_the_plan(value: str, expected: frozenset[str]) -> None:
    assert ci_ok.parse_jobs(value) == expected


@pytest.mark.parametrize("value", ["", "  ", "test-server nope", "none test-server"])
def test_a_missing_or_unknown_plan_fails_closed(value: str) -> None:
    with pytest.raises(ValueError, match="CI_JOBS"):
        ci_ok.parse_jobs(value)


def test_fork_path_passes() -> None:
    fork = {
        **GREEN,
        "denylist (private list)": "skipped",
        "denylist (generic rules, fork)": "success",
        "sonar": "skipped",
    }
    assert ci_ok.evaluate(jobs(fork), "runner-9", ALL)[0] == "pass"


@pytest.mark.parametrize(
    ("name", "conclusion"),
    [
        ("plan", "failure"),
        ("gates", "failure"),
        ("gates", "cancelled"),
        ("gates", "skipped"),
        ("sonar", "failure"),
    ],
)
def test_a_failed_job_fails(name: str, conclusion: str) -> None:
    assert ci_ok.evaluate(jobs({**GREEN, name: conclusion}), "runner-9", ALL)[0] == "fail"


def test_failed_private_deny_list_fails_once_the_fork_job_is_skipped() -> None:
    results: dict[str, str | None] = {**GREEN, "denylist (private list)": "failure"}
    assert ci_ok.evaluate(jobs(results), "runner-9", ALL)[0] == "fail"


def test_both_deny_list_jobs_skipped_fails() -> None:
    results = {**GREEN, "denylist (private list)": "skipped"}
    assert ci_ok.evaluate(jobs(results), "runner-9", ALL)[0] == "fail"


def test_running_or_missing_jobs_wait() -> None:
    assert ci_ok.evaluate(jobs({**GREEN, "gates": None}), "runner-9", ALL)[0] == "wait"
    missing = {k: v for k, v in GREEN.items() if k != "sonar"}
    assert ci_ok.evaluate(jobs(missing), "runner-9", ALL)[0] == "wait"


def test_a_failure_decides_before_other_jobs_finish() -> None:
    results = {**GREEN, "plan": "failure", "gates": None}
    assert ci_ok.evaluate(jobs(results), "runner-9", ALL)[0] == "fail"


def test_called_workflow_uses_the_prefix_of_the_own_job() -> None:
    listed = jobs(GREEN, prefix="gates / ")
    # A job of the calling workflow with a colliding name must not count.
    listed.append(
        {"name": "gates", "status": "completed", "conclusion": "failure", "runner_name": "runner-x"}
    )
    assert ci_ok.evaluate(listed, "runner-9", ALL)[0] == "pass"


def test_own_job_not_listed_yet_waits() -> None:
    listed = [j for j in jobs(GREEN) if j["name"] != "ci-ok"]
    assert ci_ok.evaluate(listed, "runner-9", ALL)[0] == "wait"


def test_single_running_ci_ok_is_the_own_job_without_a_runner_match() -> None:
    assert ci_ok.evaluate(jobs(GREEN), "runner-unknown", ALL)[0] == "pass"


def test_run_polls_until_decided() -> None:
    pages = [jobs({**GREEN, "gates": None}), jobs(GREEN)]
    slept: list[float] = []
    rc = ci_ok.run(lambda: pages.pop(0), "runner-9", ALL, sleep=slept.append, interval=1.0)
    assert rc == 0
    assert slept == [1.0]


def test_run_retries_api_errors() -> None:
    calls: list[int] = []

    def fetch() -> list[Any]:
        calls.append(1)
        if len(calls) == 1:
            raise OSError("temporary")
        return jobs({**GREEN, "gates": "failure"})

    assert ci_ok.run(fetch, "runner-9", ALL, sleep=lambda _s: None, interval=1.0) == 1


# --- the real workflow ------------------------------------------------------------


def _load(path: Path) -> dict[Any, Any]:
    data = yaml.safe_load(path.read_text(encoding="utf-8"))
    assert isinstance(data, dict)
    return data


@pytest.mark.parametrize(
    "job",
    [
        "gates",
        "gates (docs and contracts)",
        "gates (security and supply chain)",
        "test (tools)",
        "test (server)",
        "test (apps)",
        "test (SDKs and plugins)",
    ],
)
def test_the_parallel_gate_jobs(job: str) -> None:
    """Every part of `make gates` the plan needs must pass; a skip of a needed one fails."""
    assert ci_ok.evaluate(jobs({**GREEN, job: "failure"}), "runner-9", ALL)[0] == "fail"
    assert ci_ok.evaluate(jobs({**GREEN, job: "skipped"}), "runner-9", ALL)[0] == "fail"


def test_required_names_are_the_other_jobs_of_ci_yml() -> None:
    names = {job.get("name", key) for key, job in _load(CI)["jobs"].items() if key != "ci-ok"}
    assert {n for rule in ci_ok.RULES for n in rule.names} == names


def test_ci_ok_starts_with_the_run_and_may_read_the_jobs() -> None:
    job = _load(CI)["jobs"]["ci-ok"]
    assert job["name"] == "ci-ok"
    assert "needs" not in job
    assert "if" not in job
    assert job["permissions"]["actions"] == "read"


def test_release_grants_the_called_ci_ok_read_access_to_the_jobs() -> None:
    assert _load(RELEASE)["jobs"]["gates"]["permissions"]["actions"] == "read"


def test_a_job_that_only_ends_with_the_name_is_not_the_own_job() -> None:
    listed = jobs(GREEN)
    listed.append(
        {"name": "not-ci-ok", "status": "in_progress", "conclusion": None, "runner_name": "r"}
    )
    assert ci_ok.evaluate(listed, "runner-unknown", ALL)[0] == "pass"


def test_run_retries_any_reading_error_and_names_the_status(
    capsys: pytest.CaptureFixture[str],
) -> None:
    import io
    import urllib.error
    from email.message import Message

    denied = urllib.error.HTTPError(
        "https://api.example.com", 403, "forbidden", Message(), io.BytesIO()
    )
    errors: list[Exception] = [denied, TypeError("bad payload")]

    def fetch() -> list[Any]:
        if errors:
            raise errors.pop(0)
        return jobs(GREEN)

    assert ci_ok.run(fetch, "runner-9", ALL, sleep=lambda _s: None, interval=1.0) == 0
    denied.close()
    assert "HTTP 403" in capsys.readouterr().out


def test_duplicate_job_names_fail_closed() -> None:
    listed = jobs(GREEN)
    listed.insert(
        0,
        {
            "name": "gates",
            "status": "completed",
            "conclusion": "failure",
            "runner_name": "runner-y",
        },
    )
    assert ci_ok.evaluate(listed, "runner-9", ALL)[0] == "fail"


# --- the plan of ci.yml and build-verify.yml -------------------------------------------------


def _condition(job: str) -> str:
    return f"contains(format(' {{0}} ', needs.plan.outputs.jobs), ' {job} ')"


@pytest.mark.parametrize("path", [CI, BUILD_VERIFY])
def test_each_conditional_job_runs_exactly_when_the_plan_lists_it(path: Path) -> None:
    """The only reason a conditional job is skipped is the plan; other jobs run always."""
    workflow = _load(path)["jobs"]
    for job in components.CONDITIONAL_JOBS:
        assert workflow[job]["if"] == _condition(job), job
        assert workflow[job]["needs"] == "plan", job
    others = {job for job, spec in workflow.items() if "if" in spec} - ALL
    expected = {"denylist", "denylist-fork", "sonar"} if path == CI else {"build-verify-ok"}
    assert others == expected


def test_rules_name_the_conditional_jobs_of_ci_yml() -> None:
    workflow = _load(CI)["jobs"]
    assert {job: workflow[job]["name"] for job in ALL} == RULE_OF_JOB


def _plan_step(job: dict[str, Any]) -> dict[str, Any]:
    [step] = [s for s in job["steps"] if s.get("id") == "jobs"]
    return step


def test_ci_ok_plans_the_jobs_with_the_step_of_the_plan_job() -> None:
    """ci-ok has no `needs`, so it plans again with the same step on the same commit."""
    workflow = _load(CI)["jobs"]
    plan, ok = workflow["plan"], workflow["ci-ok"]
    assert _plan_step(ok) == _plan_step(plan)
    assert "components.py ci-plan" in _plan_step(plan)["run"]
    assert plan["outputs"]["jobs"] == "${{ steps.jobs.outputs.jobs }}"
    assert plan["outputs"]["components"] == "${{ steps.jobs.outputs.components }}"
    [check] = [s for s in ok["steps"] if "ci_ok.py" in str(s.get("run", ""))]
    assert check["env"]["CI_JOBS"] == "${{ steps.jobs.outputs.jobs }}"
    # The diff needs the history of both commits.
    [checkout] = [s for s in ok["steps"] if str(s.get("uses", "")).startswith("actions/checkout@")]
    assert checkout["with"]["fetch-depth"] == 0


def test_build_verify_plans_with_the_same_command() -> None:
    plan = _load(BUILD_VERIFY)["jobs"]["plan"]
    run = _plan_step(plan)["run"]
    assert "components.py ci-plan" in run
    assert plan["outputs"]["jobs"] == "${{ steps.jobs.outputs.jobs }}"
    assert (
        _plan_step(plan)["env"]["PUBLIC"] == _plan_step(_load(CI)["jobs"]["plan"])["env"]["PUBLIC"]
    )


def test_build_verify_ok_checks_skips_against_the_plan() -> None:
    ok = _load(BUILD_VERIFY)["jobs"]["build-verify-ok"]
    [check] = [s for s in ok["steps"] if "ci_ok.py needs" in str(s.get("run", ""))]
    assert check["env"]["RESULTS"] == "${{ toJSON(needs) }}"
    assert check["env"]["CI_JOBS"] == "${{ needs.plan.outputs.jobs }}"


def test_superseded_pull_request_runs_are_cancelled() -> None:
    concurrency = _load(CI)["concurrency"]
    assert "github.event.pull_request.number" in concurrency["group"]
    assert concurrency["cancel-in-progress"] == "${{ github.event_name != 'push' }}"


GREEN_NEEDS = {
    "plan": "success",
    "static": "success",
    "test-tools": "success",
    "denylist": "success",
    **dict.fromkeys(components.CONDITIONAL_JOBS, "success"),
}


def test_needs_check_passes_a_green_run() -> None:
    assert ci_ok.check_needs(GREEN_NEEDS, ALL) == []


@pytest.mark.parametrize("job", sorted(components.CONDITIONAL_JOBS))
def test_needs_check_accepts_a_skip_only_when_the_plan_left_the_job_out(job: str) -> None:
    skipped = {**GREEN_NEEDS, job: "skipped"}
    assert ci_ok.check_needs(skipped, ALL - {job}) == []
    assert ci_ok.check_needs(skipped, ALL) == [f"{job}: skipped, but the plan needs it"]
    failed = {**GREEN_NEEDS, job: "failure"}
    assert ci_ok.check_needs(failed, ALL - {job}) == [f"{job}: failure"]


@pytest.mark.parametrize(
    ("job", "result", "problem"),
    [
        ("plan", "failure", "plan: failure"),
        ("plan", "skipped", "plan: skipped"),
        ("static", "skipped", "static: skipped"),
        ("test-tools", "skipped", "test-tools: skipped"),
        ("denylist", "cancelled", "denylist: cancelled"),
    ],
)
def test_needs_check_requires_every_other_job(job: str, result: str, problem: str) -> None:
    assert ci_ok.check_needs({**GREEN_NEEDS, job: result}, ALL) == [problem]


def test_needs_check_requires_the_plan() -> None:
    results = {k: v for k, v in GREEN_NEEDS.items() if k != "plan"}
    assert ci_ok.check_needs(results, ALL) == ["plan: missing"]


def test_needs_cli(monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]) -> None:
    import json

    results = {k: {"result": v, "outputs": {}} for k, v in GREEN_NEEDS.items()}
    monkeypatch.setenv("RESULTS", json.dumps(results))
    monkeypatch.setenv("CI_JOBS", " ".join(sorted(ALL)))
    assert ci_ok.main(["needs"]) == 0
    results["test-apps"]["result"] = "skipped"
    monkeypatch.setenv("RESULTS", json.dumps(results))
    assert ci_ok.main(["needs"]) == 1
    assert "test-apps: skipped, but the plan needs it" in capsys.readouterr().out
    monkeypatch.setenv("CI_JOBS", "")
    with pytest.raises(ValueError, match="CI_JOBS is empty"):
        ci_ok.main(["needs"])
