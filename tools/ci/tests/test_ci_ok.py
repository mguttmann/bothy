# SPDX-License-Identifier: Apache-2.0
"""Tests for ci-ok of ci.yml and build-verify-ok of build-verify.yml (tools/ci/ci_ok.py).

Both are the last job of their run: they need every other job, run with `always()` and decide
from the results of their `needs` and the plan of the run. So the required check always reports,
also after a failed, skipped or cancelled job, and holds no runner while the others run. The
plan job of ci.yml marks ci-ok pending for the head commit at the start of a pull request run,
so an older green ci-ok of the same commit never stands while the run after an edit of title or
body is still going; ci-ok completes that check run with its own conclusion.
"""

from __future__ import annotations

import json
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

# The results of the `needs` of ci-ok in a green run with the secrets.
GREEN = {
    "plan": "success",
    "gates": "success",
    "gates-docs": "success",
    "gates-security": "success",
    "test-tools": "success",
    "denylist": "success",
    "denylist-fork": "skipped",
    "sonar": "success",
    **dict.fromkeys(components.CONDITIONAL_JOBS, "success"),
}
# The results of the `needs` of build-verify-ok in a green run.
GREEN_BUILD = {
    "plan": "success",
    "static": "success",
    "test-tools": "success",
    "docs": "success",
    "security": "success",
    "denylist": "success",
    **dict.fromkeys(components.CONDITIONAL_JOBS, "success"),
}


def _load(path: Path) -> dict[Any, Any]:
    data = yaml.safe_load(path.read_text(encoding="utf-8"))
    assert isinstance(data, dict)
    return data


# --- the evaluation -----------------------------------------------------------------


@pytest.mark.parametrize("green", [GREEN, GREEN_BUILD])
def test_a_green_run_passes(green: dict[str, str]) -> None:
    assert ci_ok.check_needs(green, ALL) == []


@pytest.mark.parametrize("green", [GREEN, GREEN_BUILD])
@pytest.mark.parametrize("job", sorted(components.CONDITIONAL_JOBS))
def test_a_skipped_job_passes_only_when_the_plan_left_it_out(
    green: dict[str, str], job: str
) -> None:
    """A skip never hides a failure: a needed job must succeed, an unneeded one must not fail."""
    skipped = {**green, job: "skipped"}
    assert ci_ok.check_needs(skipped, ALL - {job}) == []
    assert ci_ok.check_needs(skipped, ALL) == [f"{job}: skipped, but the plan needs it"]
    for result in ("failure", "cancelled"):
        ran = {**green, job: result}
        assert ci_ok.check_needs(ran, ALL - {job}) == [f"{job}: {result}"]
        assert ci_ok.check_needs(ran, ALL) == [f"{job}: {result}"]


def test_a_plan_without_conditional_jobs_accepts_their_skips() -> None:
    results = {**GREEN, **dict.fromkeys(ALL, "skipped")}
    assert ci_ok.check_needs(results, frozenset()) == []
    assert ci_ok.check_needs(results, frozenset({"test-server"})) == [
        "test-server: skipped, but the plan needs it"
    ]


@pytest.mark.parametrize(
    ("job", "result"),
    [
        ("plan", "failure"),
        ("plan", "skipped"),
        ("plan", "cancelled"),
        ("gates", "failure"),
        ("gates", "cancelled"),
        ("gates", "skipped"),
        ("gates-docs", "skipped"),
        ("gates-security", "failure"),
        ("test-tools", "skipped"),
        ("sonar", "failure"),
        ("sonar", "cancelled"),
        ("denylist", "failure"),
        ("denylist", "cancelled"),
    ],
)
def test_every_other_job_must_pass(job: str, result: str) -> None:
    problems = ci_ok.check_needs({**GREEN, job: result}, ALL)
    assert f"{job}: {result}" in problems


@pytest.mark.parametrize(
    ("job", "result"),
    [
        ("static", "skipped"),
        ("test-tools", "skipped"),
        ("docs", "failure"),
        ("security", "cancelled"),
        ("denylist", "skipped"),
    ],
)
def test_every_other_build_verify_job_must_pass(job: str, result: str) -> None:
    assert ci_ok.check_needs({**GREEN_BUILD, job: result}, ALL) == [f"{job}: {result}"]


def test_sonar_may_be_skipped() -> None:
    """Without the secrets, on main without SONAR_MAIN_ANALYSIS, and after a failed test job,
    which fails its own rule."""
    assert ci_ok.check_needs({**GREEN, "sonar": "skipped"}, ALL) == []
    failed = {**GREEN, "test-server": "failure", "sonar": "skipped"}
    assert ci_ok.check_needs(failed, ALL) == ["test-server: failure"]


def test_fork_path_passes() -> None:
    fork = {**GREEN, "denylist": "skipped", "denylist-fork": "success", "sonar": "skipped"}
    assert ci_ok.check_needs(fork, ALL) == []


def test_exactly_one_deny_list_job_must_pass() -> None:
    both_skipped = {**GREEN, "denylist": "skipped"}
    assert ci_ok.check_needs(both_skipped, ALL) == [
        "denylist: skipped",
        "denylist-fork: skipped",
    ]
    fork_failed = {**GREEN, "denylist": "skipped", "denylist-fork": "failure"}
    assert ci_ok.check_needs(fork_failed, ALL) == [
        "denylist: skipped",
        "denylist-fork: failure",
    ]
    private_failed = {**GREEN, "denylist": "failure"}
    assert ci_ok.check_needs(private_failed, ALL) == [
        "denylist: failure",
        "denylist-fork: skipped",
    ]


@pytest.mark.parametrize("job", ["plan", *sorted(components.CONDITIONAL_JOBS)])
def test_a_missing_plan_or_conditional_job_fails(job: str) -> None:
    results = {k: v for k, v in GREEN.items() if k != job}
    assert ci_ok.check_needs(results, ALL - {job}) == [f"{job}: missing"]


def test_an_unknown_result_fails() -> None:
    assert ci_ok.check_needs({**GREEN, "gates": "neutral"}, ALL) == ["gates: neutral"]


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


def _needs(results: dict[str, str]) -> str:
    return json.dumps({k: {"result": v, "outputs": {}} for k, v in results.items()})


def test_cli(monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]) -> None:
    monkeypatch.setenv("GITHUB_JOB", "ci-ok")
    monkeypatch.setenv("RESULTS", _needs(GREEN))
    monkeypatch.setenv("CI_JOBS", " ".join(sorted(ALL)))
    assert ci_ok.main(["needs"]) == 0
    assert "ci-ok: every job the plan needs passed" in capsys.readouterr().out
    monkeypatch.setenv("RESULTS", _needs({**GREEN, "test-apps": "skipped"}))
    assert ci_ok.main(["needs"]) == 1
    assert "ci-ok: test-apps: skipped, but the plan needs it" in capsys.readouterr().out


def test_cli_names_the_job(
    monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    monkeypatch.setenv("GITHUB_JOB", "build-verify-ok")
    monkeypatch.setenv("RESULTS", _needs({**GREEN_BUILD, "docs": "failure"}))
    monkeypatch.setenv("CI_JOBS", "none")
    assert ci_ok.main(["needs"]) == 1
    assert "build-verify-ok: docs: failure" in capsys.readouterr().out


@pytest.mark.parametrize("value", ["", "nope"])
def test_cli_fails_without_a_plan(
    monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str], value: str
) -> None:
    """A failed plan job leaves its output empty; the check fails and names the plan."""
    monkeypatch.setenv("GITHUB_JOB", "ci-ok")
    monkeypatch.setenv("RESULTS", _needs({**GREEN, "plan": "failure"}))
    monkeypatch.setenv("CI_JOBS", value)
    assert ci_ok.main(["needs"]) == 1
    out = capsys.readouterr().out
    assert "ci-ok: CI_JOBS" in out
    assert "ci-ok: plan: failure" in out


def test_cli_rejects_other_arguments() -> None:
    with pytest.raises(SystemExit):
        ci_ok.main([])


# --- the real workflows -----------------------------------------------------------------


def _ok_job(path: Path) -> tuple[str, dict[str, Any], dict[str, Any]]:
    jobs = _load(path)["jobs"]
    name = "ci-ok" if path == CI else "build-verify-ok"
    return name, jobs[name], jobs


@pytest.mark.parametrize("path", [CI, BUILD_VERIFY])
def test_the_result_job_is_last_and_always_reports(path: Path) -> None:
    """It needs every other job, so it cannot start before they end, and runs with always(), so
    it reports after a failure, a skip or a cancellation too: the check never stays missing."""
    name, job, jobs = _ok_job(path)
    assert job["name"] == name
    assert job["if"] == "always()"
    assert isinstance(job["needs"], list)
    assert set(job["needs"]) == set(jobs) - {name}
    assert len(job["needs"]) == len(set(job["needs"]))
    assert int(job["timeout-minutes"]) <= 10


@pytest.mark.parametrize("path", [CI, BUILD_VERIFY])
def test_the_result_job_checks_the_needs_against_the_plan(path: Path) -> None:
    _, job, _ = _ok_job(path)
    [check] = [s for s in job["steps"] if "ci_ok.py needs" in str(s.get("run", ""))]
    assert check["env"]["RESULTS"] == "${{ toJSON(needs) }}"
    assert check["env"]["CI_JOBS"] == "${{ needs.plan.outputs.jobs }}"
    assert str(check["run"]).rstrip().endswith("tools/ci/ci_ok.py needs")


def test_every_job_of_ci_yml_without_a_fixed_rule_must_succeed() -> None:
    """Only the conditional jobs, sonar and the deny-list pair may end skipped."""
    jobs = set(_load(CI)["jobs"]) - {"ci-ok"}
    lenient = ALL | ci_ok.MAY_SKIP | {j for group in ci_ok.ONE_OF for j in group}
    assert lenient <= jobs
    assert {"sonar"} == ci_ok.MAY_SKIP
    assert (frozenset({"denylist", "denylist-fork"}),) == ci_ok.ONE_OF
    for job in jobs - lenient:
        assert ci_ok.check_needs({**GREEN, job: "skipped"}, ALL) == [f"{job}: skipped"], job


def _plan_step(job: dict[str, Any], step_id: str) -> dict[str, Any]:
    found: list[dict[str, Any]] = [s for s in job["steps"] if s.get("id") == step_id]
    [step] = found
    return step


MARK_IF = (
    "github.event_name == 'pull_request'"
    " && github.event.pull_request.head.repo.full_name == github.repository"
    " && github.actor != 'dependabot[bot]' && github.actor != 'renovate[bot]'"
)


def test_the_plan_job_marks_ci_ok_pending_first() -> None:
    """Before anything else the plan job creates a pending check run `ci-ok` for the head commit
    of a pull request; the latest ci-ok of that commit is then pending until ci-ok ends."""
    plan = _load(CI)["jobs"]["plan"]
    assert plan["permissions"] == {"checks": "write", "contents": "read"}
    first = plan["steps"][0]
    assert first["id"] == "mark"
    assert first["if"] == MARK_IF
    assert first["env"]["HEAD_SHA"] == "${{ github.event.pull_request.head.sha }}"
    assert first["env"]["GH_TOKEN"] == "${{ github.token }}"
    run = str(first["run"])
    assert "-X POST" in run
    assert "check-runs" in run
    assert "name=ci-ok" in run
    assert "status=in_progress" in run
    assert plan["outputs"]["marker"] == "${{ steps.mark.outputs.marker }}"


def test_ci_ok_completes_the_pending_check_run_with_its_conclusion() -> None:
    job = _load(CI)["jobs"]["ci-ok"]
    assert job["permissions"] == {"checks": "write", "contents": "read"}
    steps = job["steps"]
    check = _plan_step(job, "check")
    done = steps[-1]
    assert steps.index(check) < len(steps) - 1
    assert done["if"] == "always() && needs.plan.outputs.marker != ''"
    assert done["env"]["MARKER"] == "${{ needs.plan.outputs.marker }}"
    conclusion = "${{ steps.check.outcome == 'success' && 'success' || 'failure' }}"
    assert done["env"]["CONCLUSION"] == conclusion
    run = str(done["run"])
    assert "-X PATCH" in run
    assert "status=completed" in run
    assert 'conclusion="$CONCLUSION"' in run


def test_release_grants_the_called_workflow_its_check_permission() -> None:
    assert _load(RELEASE)["jobs"]["gates"]["permissions"] == {
        "checks": "write",
        "contents": "read",
    }


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
    expected = (
        {"denylist", "denylist-fork", "sonar", "ci-ok"} if path == CI else {"build-verify-ok"}
    )
    assert others == expected


def test_the_plan_writes_the_jobs_output() -> None:
    plan = _load(CI)["jobs"]["plan"]
    assert "components.py ci-plan" in _plan_step(plan, "jobs")["run"]
    assert plan["outputs"]["jobs"] == "${{ steps.jobs.outputs.jobs }}"
    assert plan["outputs"]["components"] == "${{ steps.jobs.outputs.components }}"


def test_build_verify_plans_with_the_same_command() -> None:
    plan = _load(BUILD_VERIFY)["jobs"]["plan"]
    run = _plan_step(plan, "jobs")["run"]
    assert "components.py ci-plan" in run
    assert plan["outputs"]["jobs"] == "${{ steps.jobs.outputs.jobs }}"
    assert (
        _plan_step(plan, "jobs")["env"]["PUBLIC"]
        == _plan_step(_load(CI)["jobs"]["plan"], "jobs")["env"]["PUBLIC"]
    )


def test_superseded_pull_request_runs_are_cancelled() -> None:
    concurrency = _load(CI)["concurrency"]
    assert "github.event.pull_request.number" in concurrency["group"]
    assert concurrency["cancel-in-progress"] == "${{ github.event_name != 'push' }}"
