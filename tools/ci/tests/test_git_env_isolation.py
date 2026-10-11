# SPDX-License-Identifier: Apache-2.0
"""Tests must never reach the repository of a calling git hook or the identity of the machine.

git exports GIT_DIR to hooks (an absolute path in a linked worktree), and the pre-push hook
runs `make check`, so pytest inherits it. A fixture that runs `git init`, `git add` and
`git commit` in a temporary directory would then rewrite the real repository: `core.bare`
set, a commit that deletes every file. tools/conftest.py removes the variables that point
git at another repository for every test; the first test runs a git fixture under such an
environment and checks that the outer repository stays untouched.

A fresh CI runner has no git identity, and a fixture that relied on the identity of a
workstation failed there (`git tag -a`, exit 128). tools/conftest.py runs the whole test run
without one; the next tests check that no identity of the machine reaches a test or a
session-scoped fixture and that the repositories of testing/git_fixtures.py commit and tag
with their own.
"""

from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path

import pytest
from git_fixtures import commit, git, init_repo

TOOLS = Path(__file__).resolve().parents[2]


def test_fixture_git_calls_stay_in_their_temporary_repository(tmp_path: Path) -> None:
    outer = init_repo(tmp_path / "outer")
    (outer / "real.txt").write_text("real\n")
    commit(outer, "r")
    config = (outer / ".git" / "config").read_text()
    head = git(outer, "rev-parse", "HEAD")

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
    assert git(outer, "rev-parse", "HEAD") == head
    assert git(outer, "status", "--porcelain") == ""


def test_tests_run_without_a_git_identity(tmp_path: Path) -> None:
    """A fresh CI runner has no git identity, so no test may have one either."""
    subprocess.run(["git", "init", "-q", "-b", "main", str(tmp_path)], check=True, timeout=60)
    for ident in ("GIT_AUTHOR_IDENT", "GIT_COMMITTER_IDENT"):
        run = subprocess.run(
            ["git", "-C", str(tmp_path), "var", ident],
            capture_output=True,
            text=True,
            check=False,
            timeout=60,
        )
        assert run.returncode != 0, f"{ident} is known to the tests: {run.stdout}"


@pytest.fixture(scope="session")
def session_fixture_identity(
    tmp_path_factory: pytest.TempPathFactory,
) -> subprocess.CompletedProcess[str]:
    """`git var` in a fixture that pytest sets up before every function-scoped one."""
    repo = tmp_path_factory.mktemp("session-repo")
    subprocess.run(["git", "init", "-q", str(repo)], check=True, timeout=60)
    return subprocess.run(
        ["git", "-C", str(repo), "var", "GIT_AUTHOR_IDENT"],
        capture_output=True,
        text=True,
        check=False,
        timeout=60,
    )


def test_wider_scoped_fixtures_run_without_a_git_identity(
    session_fixture_identity: subprocess.CompletedProcess[str],
) -> None:
    """A module or session fixture that commits without an identity fails locally too."""
    run = session_fixture_identity
    assert run.returncode != 0, f"a session fixture knows the identity {run.stdout}"


def test_no_identity_of_the_machine_reaches_a_test(tmp_path: Path) -> None:
    """Every place a workstation or a calling git keeps an identity is offered to a test run."""
    sam = "[user]\n\tname = Sam Example\n\temail = sam@example.com\n"
    home = tmp_path / "home"
    (home / ".config" / "git").mkdir(parents=True)
    (home / ".gitconfig").write_text(sam)
    (home / ".config" / "git" / "config").write_text(sam)
    (tmp_path / "system.gitconfig").write_text(sam)
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    env.update(
        HOME=str(home),
        XDG_CONFIG_HOME=str(home / ".config"),
        GIT_CONFIG_SYSTEM=str(tmp_path / "system.gitconfig"),
        GIT_CONFIG_PARAMETERS="'user.name'='Sam Example' 'user.email'='sam@example.com'",
        GIT_CONFIG_COUNT="2",
        GIT_CONFIG_KEY_0="user.name",
        GIT_CONFIG_VALUE_0="Sam Example",
        GIT_CONFIG_KEY_1="user.email",
        GIT_CONFIG_VALUE_1="sam@example.com",
        GIT_AUTHOR_NAME="Sam Example",
        GIT_AUTHOR_EMAIL="sam@example.com",
        GIT_COMMITTER_NAME="Sam Example",
        GIT_COMMITTER_EMAIL="sam@example.com",
        EMAIL="sam@example.com",
    )
    probes = [
        "ci/tests/test_git_env_isolation.py::test_tests_run_without_a_git_identity",
        "ci/tests/test_git_env_isolation.py::test_wider_scoped_fixtures_run_without_a_git_identity",
    ]
    run = subprocess.run(
        [sys.executable, "-m", "pytest", "-p", "no:cacheprovider", "-q", *probes],
        cwd=TOOLS,
        env=env,
        capture_output=True,
        text=True,
        check=False,
        timeout=120,
    )
    assert run.returncode == 0, run.stdout + run.stderr


def test_fixture_repositories_commit_and_tag_with_their_own_identity(tmp_path: Path) -> None:
    repo = init_repo(tmp_path / "repo")
    commit(repo, "docs: start")
    git(repo, "tag", "-a", "v0.0.1", "-m", "release")
    alex = "Alex Example <alex@example.com>"
    assert git(repo, "log", "-1", "--format=%an <%ae>|%cn <%ce>") == f"{alex}|{alex}\n"
    assert f"\ntagger {alex} " in git(repo, "cat-file", "tag", "v0.0.1")


def test_commit_author_replaces_the_fixture_identity(tmp_path: Path) -> None:
    repo = init_repo(tmp_path / "repo")
    commit(repo, "docs: start", author="Sam Example <sam@example.com>")
    sam = "Sam Example <sam@example.com>"
    assert git(repo, "log", "-1", "--format=%an <%ae>|%cn <%ce>") == f"{sam}|{sam}\n"
