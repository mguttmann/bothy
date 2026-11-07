# SPDX-License-Identifier: Apache-2.0
"""Tests for the commit range of denylist-main.yml (push unchanged, manual run limited)."""

from __future__ import annotations

from pathlib import Path

import denylist_scan as ds
import main_range as mr
import pytest
from git_fixtures import commit, git, init_repo

TESTDATA = Path(__file__).resolve().parents[1] / "testdata"
EXAMPLE_LIST = TESTDATA / "example-denylist.txt"
GENERIC = Path(__file__).resolve().parents[1] / "generic.rules"
ZERO = "0" * 40
A = "a" * 40
B = "b" * 40


@pytest.fixture(autouse=True)
def _isolated_environment(monkeypatch: pytest.MonkeyPatch) -> None:
    for name in (
        "BOTHY_DENYLIST_FILE",
        "BOTHY_DENYLIST_ALLOW_AUTHORS",
        "BOTHY_DENYLIST_NAMES",
        "BOTHY_DENYLIST_SUMMARY",
        "PR_TITLE",
        "PR_BODY",
        "PR_BODY_FILE",
        "EVENT_NAME",
        "BEFORE",
        "AFTER",
        "INPUT_RANGE",
    ):
        monkeypatch.delenv(name, raising=False)


def exists_all(_sha: str) -> bool:
    return True


def exists_none(_sha: str) -> bool:
    return False


# --- push: unchanged behaviour ------------------------------------------------


def test_push_scans_before_to_after() -> None:
    assert mr.scan_range("push", A, B, "", exists_all) == f"{A}..{B}"


@pytest.mark.parametrize("before", ["", ZERO])
def test_push_without_before_scans_all_reachable(before: str) -> None:
    assert mr.scan_range("push", before, B, "", exists_all) == B


def test_push_with_unknown_before_scans_all_reachable() -> None:
    assert mr.scan_range("push", A, B, "", exists_none) == B


def test_push_ignores_an_input_range() -> None:
    assert mr.scan_range("push", A, B, f"{B}..{A}", exists_all) == f"{A}..{B}"


# --- workflow_dispatch: the tree of main plus an optional range ---------------


def test_dispatch_without_input_scans_no_commits() -> None:
    assert mr.scan_range("workflow_dispatch", "", B, "", exists_all) == f"{B}..{B}"


def test_dispatch_with_blank_input_scans_no_commits() -> None:
    assert mr.scan_range("workflow_dispatch", "", B, "  \n", exists_all) == f"{B}..{B}"


def test_dispatch_with_input_scans_that_range() -> None:
    assert mr.scan_range("workflow_dispatch", "", B, " abc1234..def5678 ", exists_all) == (
        "abc1234..def5678"
    )


@pytest.mark.parametrize(
    "requested",
    [
        "--all",
        "HEAD~3..HEAD",
        "main",
        "abc1234...def5678",
        "abc1234..",
        "..def5678",
        "abc1234..def5678 --all",
        "abc1234..def5678;true",
        "abc12..def5678",
        "ABC1234..DEF5678",
        "a" * 41 + "..def5678",
    ],
)
def test_dispatch_rejects_anything_but_two_commit_ids(requested: str) -> None:
    with pytest.raises(mr.RangeError):
        mr.scan_range("workflow_dispatch", "", B, requested, exists_all)


def test_dispatch_rejects_unknown_commits() -> None:
    with pytest.raises(mr.RangeError, match="unknown commit"):
        mr.scan_range("workflow_dispatch", "", B, "abc1234..def5678", exists_none)


def test_other_events_are_rejected() -> None:
    with pytest.raises(mr.RangeError, match="event"):
        mr.scan_range("pull_request", A, B, "", exists_all)


def test_missing_after_is_rejected() -> None:
    with pytest.raises(mr.RangeError):
        mr.scan_range("workflow_dispatch", "", "", "", exists_all)


# --- command line against a real repository -----------------------------------


@pytest.fixture
def history(tmp_path: Path) -> tuple[Path, str, str]:
    """A repository whose first commit message has a term; the tree and later commits are clean."""
    repo = init_repo(tmp_path / "repo")
    (repo / "README.md").write_text("# Demo\n")
    commit(repo, "docs: start for acme-widgets")
    old = git(repo, "rev-parse", "HEAD").strip()
    (repo / "a.md").write_text("clean\n")
    commit(repo, "docs: a")
    head = git(repo, "rev-parse", "HEAD").strip()
    return repo, old, head


def cli(
    monkeypatch: pytest.MonkeyPatch,
    repo: Path,
    capsys: pytest.CaptureFixture[str],
    event: str,
    after: str,
    before: str = "",
    requested: str = "",
) -> tuple[int, str, str]:
    monkeypatch.chdir(repo)
    monkeypatch.setenv("EVENT_NAME", event)
    monkeypatch.setenv("BEFORE", before)
    monkeypatch.setenv("AFTER", after)
    monkeypatch.setenv("INPUT_RANGE", requested)
    rc = mr.main([])
    out = capsys.readouterr()
    return rc, out.out.strip(), out.err


def scan_commits(repo: Path, rev_range: str) -> list[ds.Finding]:
    args = ["--repo", str(repo), "--generic", str(GENERIC), "--list", str(EXAMPLE_LIST)]
    return ds.scan(ds.parse_args([*args, "--range", rev_range])).findings


def test_manual_run_is_not_red_on_an_old_commit(
    history: tuple[Path, str, str],
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    repo, _old, head = history
    rc, rev_range, _ = cli(monkeypatch, repo, capsys, "workflow_dispatch", head)
    assert rc == 0
    assert scan_commits(repo, rev_range) == []
    # the push of the first commit (no before) still scans all reachable commits
    rc, rev_range, _ = cli(monkeypatch, repo, capsys, "push", head, before=ZERO)
    assert rc == 0
    assert [f.kind for f in scan_commits(repo, rev_range)] == ["commit-message"]


def test_manual_run_still_sees_the_tree_of_main(
    history: tuple[Path, str, str],
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    repo, _old, _head = history
    (repo / "b.md").write_text("globex.test\n")
    commit(repo, "docs: b")
    head = git(repo, "rev-parse", "HEAD").strip()
    rc, rev_range, _ = cli(monkeypatch, repo, capsys, "workflow_dispatch", head)
    assert rc == 0
    assert [f.location for f in scan_commits(repo, rev_range)] == ["b.md:1:1"]


def test_manual_run_with_a_range_scans_those_commits(
    history: tuple[Path, str, str],
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    repo, old, head = history
    rc, rev_range, _ = cli(
        monkeypatch, repo, capsys, "workflow_dispatch", head, requested=f"{old[:12]}..{head}"
    )
    assert rc == 0
    assert rev_range == f"{old[:12]}..{head}"
    assert scan_commits(repo, rev_range) == []
    git(repo, "commit", "-q", "--allow-empty", "-m", "docs: about project falcon")
    tip = git(repo, "rev-parse", "HEAD").strip()
    rc, rev_range, _ = cli(
        monkeypatch, repo, capsys, "workflow_dispatch", tip, requested=f"{head}..{tip}"
    )
    assert rc == 0
    assert [f.kind for f in scan_commits(repo, rev_range)] == ["commit-message"]


def test_cli_rejects_an_unknown_commit(
    history: tuple[Path, str, str],
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    repo, _old, head = history
    rc, out, err = cli(
        monkeypatch, repo, capsys, "workflow_dispatch", head, requested=f"{'1' * 40}..{head}"
    )
    assert rc == 2
    assert out == ""
    assert "unknown commit" in err


def test_cli_push_with_unknown_before_falls_back(
    history: tuple[Path, str, str],
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    repo, _old, head = history
    rc, out, _ = cli(monkeypatch, repo, capsys, "push", head, before="1" * 40)
    assert (rc, out) == (0, head)
