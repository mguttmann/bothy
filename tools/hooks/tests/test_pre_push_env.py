# SPDX-License-Identifier: Apache-2.0
"""The pre-push hook runs make without the variables git exports to hooks."""

from __future__ import annotations

import os
import shlex
import subprocess
from pathlib import Path

import push_plan
import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
HOOK_VARIABLES = ("GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_PREFIX")


def _make_check_entry() -> list[str]:
    config = yaml.safe_load((REPO / ".pre-commit-config.yaml").read_text(encoding="utf-8"))
    for repo in config["repos"]:
        for hook in repo.get("hooks", []):
            if hook.get("id") == "make-check":
                return shlex.split(hook["entry"])
    raise AssertionError("no hook make-check in .pre-commit-config.yaml")


def test_the_hook_runs_the_push_plan() -> None:
    assert _make_check_entry()[-3:] == ["python", "tools/hooks/push_plan.py", "run"]


def test_make_runs_without_the_git_variables_of_a_hook(monkeypatch: pytest.MonkeyPatch) -> None:
    """Every tool under `make check` sees the repository as CI does, not the hook's GIT_DIR.

    Flutter and dart read their own SDK version with git; under a hook they read this repository
    instead and refuse the pinned SDK (docs/DECISIONS.md D-M2-38-22, D-M4-05-6).
    """
    assert set(push_plan.HOOK_VARIABLES) == set(HOOK_VARIABLES)
    for name in HOOK_VARIABLES:
        monkeypatch.setenv(name, "/nonexistent")
    seen: dict[str, str] = {}

    def fake_run(cmd: list[str], **kwargs: object) -> subprocess.CompletedProcess[str]:
        env = kwargs["env"]
        assert isinstance(env, dict)
        seen.update(env)
        return subprocess.CompletedProcess(cmd, 0)

    monkeypatch.setattr(
        push_plan, "push_plan", lambda repo: (push_plan.Plan(full=True, reason="test"), "", "")
    )
    monkeypatch.setattr(push_plan.subprocess, "run", fake_run)
    assert push_plan.main(["run", "--repo", str(REPO)]) == 0
    assert seen
    assert not set(seen) & set(HOOK_VARIABLES)


def test_the_hook_prefix_runs_the_planner() -> None:
    """The entry of the hook starts this planner with the pinned tools (a dry run)."""
    entry = _make_check_entry()
    env = dict(os.environ, PUSH_FULL="1")
    out = subprocess.run(  # nosec B603 - the hook's own entry with a fixed argument
        [*entry, "--dry-run"], cwd=REPO, env=env, capture_output=True, text=True, check=True
    ).stdout
    assert "push plan: PUSH_FULL=1: make check" in out
