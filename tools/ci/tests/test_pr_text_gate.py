# SPDX-License-Identifier: Apache-2.0
"""The pull request title and body are deny-list scanned in CI (they become the commit on main).

These tests read the real workflow: both deny-list jobs get title and body from the event
payload through the environment, never interpolated into a script, and the workflow runs
again when the title or body is edited.
"""

from __future__ import annotations

from pathlib import Path
from typing import Any

import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
CI = REPO / ".github" / "workflows" / "ci.yml"
DENY_JOBS = ("denylist", "denylist-fork")


def _workflow() -> dict[Any, Any]:
    data = yaml.safe_load(CI.read_text(encoding="utf-8"))
    assert isinstance(data, dict)
    return data


def _triggers(data: dict[Any, Any]) -> dict[str, Any]:
    # PyYAML reads the bare key `on` as the boolean True.
    on = data.get("on", data.get(True))
    assert isinstance(on, dict)
    return on


def test_pull_request_runs_again_when_title_or_body_is_edited() -> None:
    types = _triggers(_workflow())["pull_request"].get("types", [])
    assert {"opened", "synchronize", "reopened", "edited"} <= set(types)


@pytest.mark.parametrize("job", DENY_JOBS)
def test_deny_list_jobs_get_title_and_body_through_the_environment(job: str) -> None:
    steps = _workflow()["jobs"][job]["steps"]
    [step] = [s for s in steps if "make denylist" in s.get("run", "")]
    env = step["env"]
    assert env["PR_TITLE"] == "${{ github.event.pull_request.title }}"
    assert env["PR_BODY"] == "${{ github.event.pull_request.body }}"


def test_title_and_body_are_never_interpolated_into_a_script() -> None:
    for name, job in _workflow()["jobs"].items():
        for step in job.get("steps", []):
            run = step.get("run", "")
            assert "github.event.pull_request.title" not in run, name
            assert "github.event.pull_request.body" not in run, name


def test_coderabbit_keeps_its_summary_out_of_the_body() -> None:
    """A bot summary in the body would land in the squash commit and restart CI on every edit."""
    config = yaml.safe_load((REPO / ".coderabbit.yaml").read_text(encoding="utf-8"))
    assert config["reviews"]["high_level_summary_in_walkthrough"] is True
