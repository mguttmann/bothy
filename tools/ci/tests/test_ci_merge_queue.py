# SPDX-License-Identifier: Apache-2.0
"""ci.yml in two stages: fast pull request checks, the full suite in the merge queue.

docs/quality/gates.md, "How a pull request lands" (docs/decisions.d/CI-SPLIT-MERGE-QUEUE.md): with
the repository variable BOTHY_MERGE_QUEUE a pull request runs the plan of the stage `pr`; the
merge queue (`merge_group`) runs every job of a push to main; a push to main still runs everything;
without the variable a pull request keeps the full plan of its diff. ci-ok is the one required
check of both events. These tests read the real workflow and run the real script of its plan step
with a stand-in for `uv` that records the call.
"""

from __future__ import annotations

import os
import stat
import subprocess
from pathlib import Path
from typing import Any

import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
CI = REPO / ".github" / "workflows" / "ci.yml"


def _workflow() -> dict[Any, Any]:
    data = yaml.safe_load(CI.read_text(encoding="utf-8"))
    assert isinstance(data, dict)
    return data


def _triggers() -> dict[str, Any]:
    data = _workflow()
    on = data.get("on", data.get(True))
    assert isinstance(on, dict)
    return on


def _step(job: str, step_id: str) -> dict[str, Any]:
    for step in _workflow()["jobs"][job]["steps"]:
        if step.get("id") == step_id:
            return step
    raise AssertionError(f"{job} has no step {step_id}")


def test_the_merge_queue_runs_the_workflow() -> None:
    assert _triggers()["merge_group"] == {"types": ["checks_requested"]}
    # A push to main still runs the full suite after each merge (the watcher reads it).
    assert _triggers()["push"] == {"branches": ["main"]}
    assert "pull_request" in _triggers()


def test_ci_ok_is_the_check_of_both_events() -> None:
    job = _workflow()["jobs"]["ci-ok"]
    assert job["name"] == "ci-ok"
    assert job["if"] == "always()"
    # The pending marker exists only for pull requests; in the queue the job's own check run is
    # the only ci-ok of the merge group commit.
    assert "github.event_name == 'pull_request'" in _step("plan", "mark")["if"]


def _plan(tmp_path: Path, **env: str) -> list[str]:
    """Run the script of the plan step `jobs` and return the arguments it passed to uv."""
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    record = tmp_path / "uv-args"
    uv = bin_dir / "uv"
    uv.write_text(f'#!/bin/sh\nprintf "%s\\n" "$@" > "{record}"\necho components=all\n')
    uv.chmod(uv.stat().st_mode | stat.S_IEXEC)
    output = tmp_path / "github-output"
    step = _step("plan", "jobs")
    full_env = {
        "PATH": f"{bin_dir}{os.pathsep}{os.environ['PATH']}",
        "GITHUB_OUTPUT": str(output),
        "PUBLIC": "true",
        "BASE_SHA": "",
        "HEAD_SHA": "",
        "QUEUE_BASE_SHA": "",
        "QUEUE_HEAD_SHA": "",
        "MERGE_QUEUE": "",
        "INPUT_COMPONENTS": "",
        **env,
    }
    subprocess.run(["bash", "-e", "-c", step["run"]], env=full_env, check=True, cwd=REPO)
    return record.read_text(encoding="utf-8").split()


def test_the_plan_step_reads_the_queue_fields_of_the_event() -> None:
    env = _step("plan", "jobs")["env"]
    assert env["QUEUE_BASE_SHA"] == "${{ github.event.merge_group.base_sha }}"
    assert env["QUEUE_HEAD_SHA"] == "${{ github.event.merge_group.head_sha }}"
    assert env["MERGE_QUEUE"] == "${{ vars.BOTHY_MERGE_QUEUE }}"


def test_a_pull_request_runs_the_fast_plan_with_a_merge_queue(tmp_path: Path) -> None:
    args = _plan(tmp_path, EVENT="pull_request", BASE_SHA="b1", HEAD_SHA="h1", MERGE_QUEUE="true")
    assert args[args.index("--stage") + 1] == "pr"
    assert args[args.index("--base") + 1] == "b1"
    assert args[args.index("--head") + 1] == "h1"


def test_a_pull_request_keeps_the_full_plan_without_a_merge_queue(tmp_path: Path) -> None:
    args = _plan(tmp_path, EVENT="pull_request", BASE_SHA="b1", HEAD_SHA="h1")
    assert args[args.index("--stage") + 1] == "full"


def test_the_merge_queue_runs_the_full_suite(tmp_path: Path) -> None:
    args = _plan(
        tmp_path, EVENT="merge_group", QUEUE_BASE_SHA="b2", QUEUE_HEAD_SHA="h2", MERGE_QUEUE="true"
    )
    assert args[args.index("--stage") + 1] == "queue"
    assert args[args.index("--base") + 1] == "b2"
    assert args[args.index("--head") + 1] == "h2"


@pytest.mark.parametrize("event", ["push", "workflow_dispatch", "workflow_call"])
def test_other_events_run_the_components(tmp_path: Path, event: str) -> None:
    args = _plan(tmp_path, EVENT=event, MERGE_QUEUE="true")
    assert args[args.index("--components") + 1] == "all"
    assert "--stage" not in args


def test_the_queue_compares_with_its_base_like_a_pull_request() -> None:
    # A failed Go test of a changed package is not rerun into a pass, and an advisory that main
    # has too does not stop the queue (the daily scan and the push to main report it).
    env = _workflow()["env"]
    assert "github.event.merge_group.base_sha" in env["FLAKE_BASE"]
    osv = _step_env("gates-security", "OSV_BASE")
    assert "github.event.merge_group.base_sha" in osv
    assert "github.event.pull_request.base.sha" in osv


def _step_env(job: str, key: str) -> str:
    for step in _workflow()["jobs"][job]["steps"]:
        if key in step.get("env", {}):
            return str(step["env"][key])
    raise AssertionError(f"{job} sets no {key}")


def test_sonar_does_not_scan_the_queue_branches() -> None:
    # The pull request's sonar is the quality gate of its new code; a scan of each temporary
    # gh-readonly-queue branch would only add branch analyses. ci_ok.MAY_SKIP allows the skip.
    assert "github.event_name != 'merge_group'" in _workflow()["jobs"]["sonar"]["if"]
