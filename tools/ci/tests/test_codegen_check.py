# SPDX-License-Identifier: Apache-2.0
"""Tests for codegen_check: only changes made by the generator count as drift."""

from __future__ import annotations

import hashlib
import sys
from pathlib import Path

import codegen_check as cc
import pytest
from git_fixtures import commit, init_repo


@pytest.fixture
def repo(tmp_path: Path) -> Path:
    root = init_repo(tmp_path / "repo")
    (root / ".gitignore").write_text("build/\n")
    (root / "gen").mkdir()
    (root / "gen" / "a.go").write_text("package gen\n")
    (root / "keep.txt").write_text("kept\n")
    commit(root, "x")
    return root


def run(directory: Path, code: str) -> int:
    """Run codegen_check with a Python one-liner as the generator."""
    return cc.main(["--dir", str(directory), "--", sys.executable, "-c", code])


def test_no_change_passes(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    assert run(repo, "pass") == 0
    assert "stale" not in capsys.readouterr().out


def test_modified_file_fails_and_is_listed(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    assert run(repo, "open('gen/a.go', 'w').write('package gen // new\\n')") == 1
    out = capsys.readouterr().out
    assert "changed: gen/a.go" in out
    assert f"codegen-check: generated code in {repo} was stale; commit the regenerated files" in out


def test_new_file_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    assert run(repo, "open('gen/b.go', 'w').write('package gen\\n')") == 1
    assert "added: gen/b.go" in capsys.readouterr().out


def test_deleted_file_fails(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    assert run(repo, "import os; os.remove('keep.txt')") == 1
    assert "removed: keep.txt" in capsys.readouterr().out


def test_preexisting_dirty_file_is_not_drift(repo: Path) -> None:
    (repo / "keep.txt").write_text("hand edit, not committed\n")
    (repo / "notes.txt").write_text("untracked\n")
    (repo / "gen" / "a.go").unlink()
    assert run(repo, "pass") == 0
    assert run(repo, "open('keep.txt', 'w').write('hand edit, not committed\\n')") == 0


def test_ignored_file_is_not_counted(repo: Path) -> None:
    assert run(repo, "import os; os.makedirs('build'); open('build/out.o', 'w').write('x')") == 0


def test_changes_outside_the_directory_are_not_counted(repo: Path) -> None:
    assert run(repo / "gen", "open('../keep.txt', 'w').write('changed\\n')") == 0


def test_command_failure_propagates_exit_code(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    assert run(repo, "import sys; open('gen/a.go', 'w').write('half'); sys.exit(3)") == 3
    captured = capsys.readouterr()
    assert "failed with exit code 3" in captured.err
    assert "stale" not in captured.out


def test_signal_exit_maps_to_128_plus(repo: Path) -> None:
    assert run(repo, "import os, signal; os.kill(os.getpid(), signal.SIGTERM)") == 143


def test_missing_command_is_127(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    assert cc.main(["--dir", str(repo), "--", "no-such-generator-on-path"]) == 127
    assert "no-such-generator-on-path" in capsys.readouterr().err


def test_usage_errors(repo: Path, tmp_path: Path) -> None:
    assert cc.main(["--dir", str(repo)]) == 2
    assert cc.main(["--dir", str(repo), "--"]) == 2
    assert cc.main(["--", sys.executable, "-c", "pass"]) == 2
    outside = tmp_path / "plain"
    outside.mkdir()
    assert cc.main(["--dir", str(outside), "--", sys.executable, "-c", "pass"]) == 2
    assert cc.main(["--dir", str(tmp_path / "absent"), "--", sys.executable, "-c", "pass"]) == 2


def test_snapshot_hashes_paths_relative_to_the_directory(repo: Path) -> None:
    (repo / "gen" / "new.go").write_text("x\n")
    assert cc.snapshot(repo / "gen") == {
        "a.go": hashlib.sha256(b"package gen\n").hexdigest(),
        "new.go": hashlib.sha256(b"x\n").hexdigest(),
    }


def test_compare_lists_sorted_lines() -> None:
    before = {"b.go": "1", "c.go": "2", "a.go": "3"}
    after = {"b.go": "9", "a.go": "3", "d.go": "4"}
    assert cc.compare(before, after) == ["added: d.go", "changed: b.go", "removed: c.go"]


def test_retargeted_symlink_is_drift(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    (repo / "same_a.txt").write_text("same\n")
    (repo / "same_b.txt").write_text("same\n")
    (repo / "link").symlink_to("same_a.txt")
    assert run(repo, "import os; os.remove('link'); os.symlink('same_b.txt', 'link')") == 1
    assert "changed: link" in capsys.readouterr().out


def test_command_that_is_not_executable_is_126(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    script = repo / "gen.sh"
    script.write_text("#!/bin/sh\n")
    script.chmod(0o644)
    assert cc.main(["--dir", str(repo), "--", str(script)]) == 126
    assert "permission denied" in capsys.readouterr().err


def test_hook_environment_does_not_change_what_git_sees(
    repo: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """git exports GIT_DIR to hooks; with it, git would take the module directory for the top
    of the working tree, skip the root .gitignore and count ignored output as drift."""
    monkeypatch.setenv("GIT_DIR", str(repo / ".git"))
    code = "import os; os.makedirs('build'); open('build/cache.bin', 'w').write('x')"
    assert run(repo / "gen", code) == 0
    assert cc.snapshot(repo / "gen") == {"a.go": hashlib.sha256(b"package gen\n").hexdigest()}
