# SPDX-License-Identifier: Apache-2.0
"""Which pull requests get the checks that need repository secrets (D-DEPS-3).

The unit tests cover the decision of tools/ci/pr_mode.py; the workflow tests read the real
ci.yml, so the jobs follow the decision and get the author from the event, never from text.
"""

from __future__ import annotations

from pathlib import Path
from typing import Any

import commit_msg
import pr_mode
import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
CI = REPO / ".github" / "workflows" / "ci.yml"
OWN = "OWNER/bothy"


def _decide(**kw: str) -> pr_mode.Mode:
    args = {
        "event": "pull_request",
        "repo": OWN,
        "head_repo": OWN,
        "author": "alex",
        "actor": "alex",
    }
    args.update(kw)
    return pr_mode.decide(**args)


@pytest.mark.parametrize("event", ["workflow_dispatch", "workflow_call", "push"])
def test_runs_outside_a_pull_request_get_every_check(event: str) -> None:
    mode = _decide(event=event, head_repo="", author="", actor="dependabot[bot]")
    assert mode.private_checks
    assert mode.note == ""


def test_same_repository_pull_request_of_a_person_gets_every_check() -> None:
    mode = _decide()
    assert mode.private_checks
    assert mode.note == ""


def test_fork_pull_request_runs_without_secrets() -> None:
    mode = _decide(head_repo="someone/bothy")
    assert not mode.private_checks
    assert "fork" in mode.note


@pytest.mark.parametrize("bot", ["dependabot[bot]", "renovate[bot]"])
def test_bot_pull_request_started_by_the_bot_runs_like_a_fork(bot: str) -> None:
    mode = _decide(author=bot, actor=bot)
    assert not mode.private_checks
    assert bot in mode.note
    assert "sonar" in mode.note
    assert "generic" in mode.note


def test_pull_request_without_a_head_repository_runs_without_secrets() -> None:
    """A deleted fork reports no head repository; that is never the own repository."""
    assert not _decide(head_repo="").private_checks


def test_fork_pull_request_with_a_bot_author_counts_as_fork() -> None:
    mode = _decide(head_repo="someone/bothy", author="dependabot[bot]", actor="dependabot[bot]")
    assert not mode.private_checks
    assert mode.note.startswith("fork pull request")


def test_bot_pull_request_started_by_a_person_gets_every_check() -> None:
    """A maintainer's edit or push runs with the secrets; a missing one fails closed."""
    assert _decide(author="dependabot[bot]", actor="maintainer").private_checks


def test_bot_actor_on_a_person_s_pull_request_gets_every_check() -> None:
    assert _decide(author="alex", actor="dependabot[bot]").private_checks


@pytest.mark.parametrize(
    "login", ["dependabot", "Dependabot[bot]", "dependabot[bot] ", "github-actions[bot]"]
)
def test_only_the_exact_bot_logins_count(login: str) -> None:
    assert _decide(author=login, actor=login).private_checks


def test_bot_logins_match_the_commit_message_check() -> None:
    assert set(pr_mode.BOT_LOGINS) == set(commit_msg.BOT_AUTHORS)


def test_cli_writes_the_output_and_a_visible_note(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    output = tmp_path / "out"
    summary = tmp_path / "summary"
    rc = pr_mode.main(
        [
            "--event", "pull_request", "--repo", OWN, "--head-repo", OWN,
            "--author", "dependabot[bot]", "--actor", "dependabot[bot]",
            "--github-output", str(output), "--summary", str(summary),
        ]
    )  # fmt: skip
    assert rc == 0
    assert output.read_text(encoding="utf-8") == "private_checks=false\n"
    assert "::notice title=Reduced checks::" in capsys.readouterr().out
    assert "dependabot[bot]" in summary.read_text(encoding="utf-8")


def test_cli_full_checks_write_no_note(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    output = tmp_path / "out"
    summary = tmp_path / "summary"
    rc = pr_mode.main(
        [
            "--event", "pull_request", "--repo", OWN, "--head-repo", OWN,
            "--author", "alex", "--actor", "alex",
            "--github-output", str(output), "--summary", str(summary),
        ]
    )  # fmt: skip
    assert rc == 0
    assert output.read_text(encoding="utf-8") == "private_checks=true\n"
    assert "::notice" not in capsys.readouterr().out
    assert not summary.exists()


# ---- the real workflow -----------------------------------------------------------------


def _jobs() -> dict[str, Any]:
    data = yaml.safe_load(CI.read_text(encoding="utf-8"))
    assert isinstance(data, dict)
    jobs = data["jobs"]
    assert isinstance(jobs, dict)
    return jobs


def test_plan_step_gets_author_and_actor_from_the_event() -> None:
    [step] = [s for s in _jobs()["plan"]["steps"] if s.get("id") == "plan"]
    env = step["env"]
    assert env["AUTHOR"] == "${{ github.event.pull_request.user.login }}"
    assert env["ACTOR"] == "${{ github.actor }}"
    assert "tools/ci/pr_mode.py" in step["run"]
    assert "${{" not in step["run"]
    assert _jobs()["plan"]["outputs"]["private_checks"] == (
        "${{ steps.plan.outputs.private_checks }}"
    )


@pytest.mark.parametrize(
    ("job", "condition"),
    [
        ("denylist", "needs.plan.outputs.private_checks == 'true'"),
        ("sonar", "needs.plan.outputs.private_checks == 'true'"),
        ("denylist-fork", "needs.plan.outputs.private_checks != 'true'"),
    ],
)
def test_secret_jobs_follow_the_decision(job: str, condition: str) -> None:
    if job == "sonar":
        # sonar runs after the test jobs, of which the plan may skip some (always()), and only
        # with the secrets: the decision is one of its conditions.
        assert condition in _jobs()[job]["if"]
        return
    assert _jobs()[job]["if"] == condition


def test_gates_get_the_pull_request_author_from_the_event() -> None:
    [step] = [s for s in _jobs()["gates"]["steps"] if "commitlint" in str(s.get("run", ""))]
    assert step["env"]["PR_AUTHOR"] == "${{ github.event.pull_request.user.login }}"
