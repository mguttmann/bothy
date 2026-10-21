# SPDX-License-Identifier: Apache-2.0
"""Resolution of workspace_ref under runner.projects_root and readiness (09 18.1)."""

from __future__ import annotations

from pathlib import Path

import pytest

from claude_code_runner.workspace import WorkspaceError, readiness, resolve


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
