# SPDX-License-Identifier: Apache-2.0
"""Tests for the base of the change plan of a build branch (tools/ci/build_base.py).

docs/quality/push-discipline.md, "Build branches": a build team may push `build-base/<ticket>`
right before `build/<ticket>`, pointing at the integration head it built on. build-verify.yml
then plans the jobs from the merge base of the pushed commit with that ref; without the ref it
measures from the merge base with main as before, and without any base every job runs.
"""

from __future__ import annotations

from pathlib import Path

import build_base
import pytest
from git_fixtures import commit, git, init_repo


def _head(repo: Path) -> str:
    return git(repo, "rev-parse", "HEAD").strip()


@pytest.fixture
def repo(tmp_path: Path) -> Path:
    """main, an integration head on top of it, and a ticket branch built on that head.

    The remote refs are written as `actions/checkout` with `fetch-depth: 0` leaves them.
    """
    root = init_repo(tmp_path / "repo")
    (root / "a.txt").write_text("a\n")
    commit(root, "main")
    git(root, "update-ref", "refs/remotes/origin/main", "HEAD")
    for n in range(3):
        (root / f"int{n}.txt").write_text("x\n")
        commit(root, f"integration {n}")
    git(root, "branch", "integration")
    (root / "ticket.txt").write_text("t\n")
    commit(root, "ticket")
    return root


def test_without_a_base_ref_the_plan_measures_from_main(repo: Path) -> None:
    main = git(repo, "rev-parse", "origin/main").strip()
    assert build_base.select(repo, "build/M2-44", _head(repo)) == build_base.Base(
        main, "merge base with origin/main"
    )


def test_a_base_ref_measures_from_the_integration_head(repo: Path) -> None:
    integration = git(repo, "rev-parse", "integration").strip()
    git(repo, "update-ref", "refs/remotes/origin/build-base/M2-44", integration)
    assert build_base.select(repo, "build/M2-44", _head(repo)) == build_base.Base(
        integration, "merge base with origin/build-base/M2-44"
    )


def test_the_base_ref_of_another_ticket_is_ignored(repo: Path) -> None:
    integration = git(repo, "rev-parse", "integration").strip()
    git(repo, "update-ref", "refs/remotes/origin/build-base/M2-45", integration)
    main = git(repo, "rev-parse", "origin/main").strip()
    assert build_base.select(repo, "build/M2-44", _head(repo)).sha == main


def test_a_base_ref_that_is_no_ancestor_counts_from_the_fork_point(repo: Path) -> None:
    """The plan covers every change of the pushed commit since the common history."""
    fork = git(repo, "rev-parse", "HEAD~2").strip()
    git(repo, "checkout", "-q", "-b", "other", fork)
    (repo / "other.txt").write_text("o\n")
    commit(repo, "other")
    git(repo, "update-ref", "refs/remotes/origin/build-base/M2-44", "HEAD")
    git(repo, "checkout", "-q", "-")
    assert build_base.select(repo, "build/M2-44", _head(repo)).sha == fork


def test_a_base_ref_without_common_history_falls_back_to_main(repo: Path, tmp_path: Path) -> None:
    other = init_repo(tmp_path / "other")
    (other / "b.txt").write_text("b\n")
    commit(other, "unrelated")
    git(repo, "fetch", "-q", str(other), "main:refs/remotes/origin/build-base/M2-44")
    main = git(repo, "rev-parse", "origin/main").strip()
    assert build_base.select(repo, "build/M2-44", _head(repo)).sha == main


def test_without_any_base_every_job_runs(repo: Path) -> None:
    git(repo, "update-ref", "-d", "refs/remotes/origin/main")
    assert build_base.select(repo, "build/M2-44", _head(repo)) == build_base.Base(
        "", "no merge base with origin/main"
    )


def test_a_nested_ticket_name(repo: Path) -> None:
    integration = git(repo, "rev-parse", "integration").strip()
    git(repo, "update-ref", "refs/remotes/origin/build-base/m2/wave-3", integration)
    assert build_base.select(repo, "build/m2/wave-3", _head(repo)).sha == integration


@pytest.mark.parametrize("branch", ["main", "build/", "feat/x", "build-base/M2-44"])
def test_only_a_build_branch_has_a_base(repo: Path, branch: str) -> None:
    with pytest.raises(ValueError, match="build/<ticket>"):
        build_base.select(repo, branch, _head(repo))


def test_cli_writes_the_base(
    repo: Path, tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    integration = git(repo, "rev-parse", "integration").strip()
    git(repo, "update-ref", "refs/remotes/origin/build-base/M2-44", integration)
    out = tmp_path / "out"
    args = ["--repo", str(repo), "--branch", "build/M2-44", "--head", _head(repo)]
    assert build_base.main([*args, "--github-output", str(out)]) == 0
    assert out.read_text() == f"base={integration}\n"
    assert "merge base with origin/build-base/M2-44" in capsys.readouterr().out
