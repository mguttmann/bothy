# SPDX-License-Identifier: Apache-2.0
"""The pre-push hook runs `make check` without the variables git exports to hooks."""

from __future__ import annotations

import os
import shlex
import subprocess
from pathlib import Path

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


def test_make_check_runs_without_the_git_variables_of_a_hook() -> None:
    """Every tool under `make check` sees the repository as CI does, not the hook's GIT_DIR.

    Flutter and dart read their own SDK version with git; under a hook they read this repository
    instead and refuse the pinned SDK (docs/DECISIONS.md D-M2-38-22, D-M4-05-6).
    """
    entry = _make_check_entry()
    assert entry[-2:] == ["make", "check"]
    env = dict(os.environ, **dict.fromkeys(HOOK_VARIABLES, "/nonexistent"))
    printed = subprocess.run(  # nosec B603 - the hook's own prefix with a fixed command
        [*entry[:-2], "env"], cwd=REPO, env=env, capture_output=True, text=True, check=True
    ).stdout
    names = {line.split("=", 1)[0] for line in printed.splitlines()}
    assert not names & set(HOOK_VARIABLES)
