# SPDX-License-Identifier: Apache-2.0
"""Tests for tracked_tree: scanners see tracked and committable files, never ignored ones."""

from __future__ import annotations

from pathlib import Path

import pytest
import tracked_tree as tt
from git_fixtures import commit, init_repo


@pytest.fixture
def repo(tmp_path: Path) -> Path:
    root = init_repo(tmp_path / "repo")
    (root / ".gitignore").write_text("cache/\n.claude/worktrees/\n")
    (root / "tracked.txt").write_text("committed\n")
    (root / "deleted.txt").write_text("gone\n")
    (root / "sub").mkdir()
    (root / "sub" / "nested.tf").write_text("resource {}\n")
    commit(root, "x")
    (root / "tracked.txt").write_text("changed in the working tree\n")
    (root / "deleted.txt").unlink()
    (root / "new.txt").write_text("not yet added\n")
    (root / "cache").mkdir()
    (root / "cache" / "blob.bin").write_text("ignored\n")
    (root / ".claude" / "worktrees" / "other").mkdir(parents=True)
    (root / ".claude" / "worktrees" / "other" / "copy.tf").write_text("resource {}\n")
    return root


def test_committable_lists_tracked_and_new_files_only(repo: Path) -> None:
    assert tt.committable(repo) == [".gitignore", "new.txt", "sub/nested.tf", "tracked.txt"]


def test_snapshot_copies_the_working_tree_state(repo: Path, tmp_path: Path) -> None:
    dest = tmp_path / "out"
    assert tt.snapshot(repo, dest) == 4
    assert (dest / "tracked.txt").read_text() == "changed in the working tree\n"
    assert (dest / "sub" / "nested.tf").is_file()
    assert not (dest / "cache").exists()
    assert not (dest / ".claude").exists()
    assert not (dest / "deleted.txt").exists()


def test_main_refuses_a_non_empty_destination(
    repo: Path, tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    dest = tmp_path / "out"
    dest.mkdir()
    (dest / "stale.txt").write_text("x\n")
    assert tt.main(["--repo", str(repo), str(dest)]) == 2
    assert "is not empty" in capsys.readouterr().err


def test_main_reports_the_count(
    repo: Path, tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    assert tt.main(["--repo", str(repo), str(tmp_path / "out")]) == 0
    assert "4 file(s) copied" in capsys.readouterr().out
