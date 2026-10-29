# SPDX-License-Identifier: Apache-2.0
"""Resolution of workspace_ref under runner.projects_root and readiness (09 18.1)."""

from __future__ import annotations

from pathlib import Path

import pytest

from claude_code_runner.workspace import WorkspaceError, project_folders, readiness, resolve


@pytest.fixture
def root(tmp_path: Path) -> Path:
    r = tmp_path / "projects"
    (r / "website" / ".git").mkdir(parents=True)
    (r / "team" / "website" / ".git").mkdir(parents=True)
    return r


def test_resolves_names(root: Path) -> None:
    assert resolve(root, "website") == (root / "website").resolve()
    assert resolve(root, "team/website") == (root / "team" / "website").resolve()


@pytest.mark.parametrize("ref", ["/etc", "~x", "-x", "a\\b", "C:x", "team/../x", "", "."])
def test_refused(root: Path, ref: str) -> None:
    with pytest.raises(WorkspaceError) as err:
        resolve(root, ref)
    assert err.value.code == "outside_root"


def test_symlink_outside_refused(root: Path, tmp_path: Path) -> None:
    outside = tmp_path / "outside"
    (outside / ".git").mkdir(parents=True)
    (root / "link").symlink_to(outside)
    with pytest.raises(WorkspaceError) as err:
        resolve(root, "link")
    assert err.value.code == "outside_root"


def test_symlink_to_root_refused(root: Path) -> None:
    (root / "self").symlink_to(root)
    with pytest.raises(WorkspaceError):
        resolve(root, "self")


def test_readiness_codes(root: Path) -> None:
    assert readiness(root, "website") == (True, None)
    assert readiness(root, "docs") == (False, "workspace_missing")
    (root / "plain").mkdir()
    assert readiness(root, "plain") == (False, "not_a_git_worktree")
    (root / "file").write_text("x")
    assert readiness(root, "file") == (False, "workspace_missing")
    (root / "linked").mkdir()
    (root / "linked" / ".git").write_text("gitdir: /x\n")
    assert readiness(root, "linked") == (True, None)
    ro = root / "ro"
    (ro / ".git").mkdir(parents=True)
    ro.chmod(0o555)
    try:
        assert readiness(root, "ro") == (False, "not_writable")
    finally:
        ro.chmod(0o755)
    assert readiness(root, "../x") == (False, "outside_root")


def _git_folder(root: Path, name: str) -> None:
    (root / name / ".git").mkdir(parents=True)


def test_project_folders_lists_git_work_trees(root: Path, tmp_path: Path) -> None:
    """09 18.13 item 1: one-segment names of folders with ``.git``, in code point order."""
    _git_folder(root, "Zeta")
    _git_folder(root, "api.v2_x")
    _git_folder(root, ".hidden")
    _git_folder(root, ".provision-0192d3a0")
    _git_folder(root, "my project")
    (root / "plain").mkdir()
    (root / "notes.txt").write_text("x")
    (root / "linked").mkdir()
    (root / "linked" / ".git").write_text("gitdir: /x\n")
    (root / "inside").symlink_to(root / "website")
    outside = tmp_path / "outside"
    (outside / ".git").mkdir(parents=True)
    (root / "away").symlink_to(outside)
    # "team" holds a work tree one level down, but is none itself.
    assert project_folders(root, 500) == ["Zeta", "api.v2_x", "inside", "linked", "website"]


def test_project_folders_cap_logs_the_count(root: Path, caplog: pytest.LogCaptureFixture) -> None:
    """09 18.13 item 2: at most ``runner.project_folders_max`` names, the first ones."""
    for name in ("b", "a", "c"):
        _git_folder(root, name)
    with caplog.at_level("INFO", logger="claude_code_runner"):
        assert project_folders(root, 2) == ["a", "b"]
    assert any(
        '"event": "project_folders_truncated"' in r.getMessage() and '"count": 4' in r.getMessage()
        for r in caplog.records
    )


def test_project_folders_unreadable_root(tmp_path: Path, caplog: pytest.LogCaptureFixture) -> None:
    """A root the runner cannot read gives the empty list and a log code, never an error."""
    with caplog.at_level("INFO", logger="claude_code_runner"):
        assert project_folders(tmp_path / "missing", 500) == []
    assert any('"event": "project_folders_unreadable"' in r.getMessage() for r in caplog.records)


def test_project_folders_long_name_left_out(root: Path) -> None:
    _git_folder(root, "x" * 255)
    assert project_folders(root, 500) == ["website", "x" * 255]
