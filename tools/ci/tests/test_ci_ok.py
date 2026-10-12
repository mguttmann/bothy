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
import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
CI = REPO / ".github" / "workflows" / "ci.yml"
RELEASE = REPO / ".github" / "workflows" / "release.yml"

GREEN = {
    "plan": "success",
    "gates": "success",
    "denylist (private list)": "success",
    "denylist (generic rules, fork)": "skipped",
    "sonar": "success",
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
    assert ci_ok.evaluate(jobs(GREEN), "runner-9")[0] == "pass"


def test_fork_path_passes() -> None:
    fork = {
        **GREEN,
        "denylist (private list)": "skipped",
        "denylist (generic rules, fork)": "success",
        "sonar": "skipped",
    }
    assert ci_ok.evaluate(jobs(fork), "runner-9")[0] == "pass"


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
    assert ci_ok.evaluate(jobs({**GREEN, name: conclusion}), "runner-9")[0] == "fail"


def test_failed_private_deny_list_fails_once_the_fork_job_is_skipped() -> None:
    results: dict[str, str | None] = {**GREEN, "denylist (private list)": "failure"}
    assert ci_ok.evaluate(jobs(results), "runner-9")[0] == "fail"


def test_both_deny_list_jobs_skipped_fails() -> None:
    results = {**GREEN, "denylist (private list)": "skipped"}
    assert ci_ok.evaluate(jobs(results), "runner-9")[0] == "fail"


def test_running_or_missing_jobs_wait() -> None:
    assert ci_ok.evaluate(jobs({**GREEN, "gates": None}), "runner-9")[0] == "wait"
    missing = {k: v for k, v in GREEN.items() if k != "sonar"}
    assert ci_ok.evaluate(jobs(missing), "runner-9")[0] == "wait"


def test_a_failure_decides_before_other_jobs_finish() -> None:
    results = {**GREEN, "plan": "failure", "gates": None}
    assert ci_ok.evaluate(jobs(results), "runner-9")[0] == "fail"


def test_called_workflow_uses_the_prefix_of_the_own_job() -> None:
    listed = jobs(GREEN, prefix="gates / ")
    # A job of the calling workflow with a colliding name must not count.
    listed.append(
        {"name": "gates", "status": "completed", "conclusion": "failure", "runner_name": "runner-x"}
    )
    assert ci_ok.evaluate(listed, "runner-9")[0] == "pass"


def test_own_job_not_listed_yet_waits() -> None:
    listed = [j for j in jobs(GREEN) if j["name"] != "ci-ok"]
    assert ci_ok.evaluate(listed, "runner-9")[0] == "wait"


def test_single_running_ci_ok_is_the_own_job_without_a_runner_match() -> None:
    assert ci_ok.evaluate(jobs(GREEN), "runner-unknown")[0] == "pass"


def test_run_polls_until_decided() -> None:
    pages = [jobs({**GREEN, "gates": None}), jobs(GREEN)]
    slept: list[float] = []
    rc = ci_ok.run(lambda: pages.pop(0), "runner-9", sleep=slept.append, interval=1.0)
    assert rc == 0
    assert slept == [1.0]


def test_run_retries_api_errors() -> None:
    calls: list[int] = []

    def fetch() -> list[Any]:
        calls.append(1)
        if len(calls) == 1:
            raise OSError("temporary")
        return jobs({**GREEN, "gates": "failure"})

    assert ci_ok.run(fetch, "runner-9", sleep=lambda _s: None, interval=1.0) == 1


# --- the real workflow ------------------------------------------------------------


def _load(path: Path) -> dict[Any, Any]:
    data = yaml.safe_load(path.read_text(encoding="utf-8"))
    assert isinstance(data, dict)
    return data


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
    assert ci_ok.evaluate(listed, "runner-unknown")[0] == "pass"


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

    assert ci_ok.run(fetch, "runner-9", sleep=lambda _s: None, interval=1.0) == 0
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
    assert ci_ok.evaluate(listed, "runner-9")[0] == "fail"
