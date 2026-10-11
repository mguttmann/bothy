# SPDX-License-Identifier: Apache-2.0
"""Tests started from a git hook must never reach the repository of that hook.

git exports GIT_DIR to hooks (an absolute path in a linked worktree), and the pre-push hook
runs `make check`, so pytest inherits it. A fixture that runs `git init`, `git add` and
`git commit` in a temporary directory would then rewrite the real repository: `core.bare`
set, a commit that deletes every file. tools/conftest.py removes the variables that point
git at another repository for every test; this test runs a git fixture under such an
environment and checks that the outer repository stays untouched.
"""

from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path

TOOLS = Path(__file__).resolve().parents[2]


def _git(repo: Path, *args: str) -> str:
    return subprocess.run(
        ["git", "-C", str(repo), *args], check=True, capture_output=True, text=True, timeout=60
    ).stdout


def test_fixture_git_calls_stay_in_their_temporary_repository(tmp_path: Path) -> None:
    outer = tmp_path / "outer"
    outer.mkdir()
    _git(outer, "init", "-q", "-b", "main")
    (outer / "real.txt").write_text("real\n")
    _git(outer, "add", ".")
    identity = ["-c", "user.name=Alex", "-c", "user.email=alex@example.com"]
    unsigned = ["-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"]
    _git(outer, *identity, *unsigned, "commit", "-qm", "r")
    config = (outer / ".git" / "config").read_text()
    head = _git(outer, "rev-parse", "HEAD")

    hook_env = {**os.environ, "GIT_DIR": str(outer / ".git")}
    fixture_test = "ci/tests/test_codegen_check.py::test_new_file_fails"
    run = subprocess.run(
        [sys.executable, "-m", "pytest", "-p", "no:cacheprovider", "-q", fixture_test],
        cwd=TOOLS,
        env=hook_env,
        capture_output=True,
        text=True,
        check=False,
        timeout=120,
    )

    assert run.returncode == 0, run.stdout + run.stderr
    assert (outer / ".git" / "config").read_text() == config
    assert _git(outer, "rev-parse", "HEAD") == head
    assert _git(outer, "status", "--porcelain") == ""
